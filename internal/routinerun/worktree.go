package routinerun

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/acdsl"
	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/gitwt"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/trash"
)

const (
	archiveStampFormat = "20060102-150405"
	commitBodyFileName = ".routine-commit-body"
	failedMarker       = "[failed]"
	landingSuffix      = "-landing"
	ledgerDir          = "sessions"
	leftoverMarker     = "[leftover]"
	mainBranch         = "main"
	salvagedMarker     = "[salvaged]"
	stageFailedMarker  = "[stage-failed: %s]"
	staleArchiveDir    = ".stale-archive"
)

var (
	errOpenBranchLimit = errors.New("groupWorktree.create: Open-branch limit reached")
)

// groupWorktree never pushes: each run commits locally on a fresh branch stacked on the chain tip.
type groupWorktree struct {
	agentwtPath     string
	branchPrefix    string
	maxOpenBranches int
	path            string
	repoRoot        string
	routineName     string
	trash           *trash.Store // the stash every deleted branch's diff lands in
}

// ageOut drops every open change beyond the newest window−1 proposals branches and every open change whose probe onto main conflicts; emptied branches go at the run's end.
func (g *groupWorktree) ageOut(ctx context.Context, window int) {
	branches, err := repos.ListNightBranches(ctx, g.repoRoot, repos.ProposalsSuffix)
	if err != nil {
		log.Printf("groupWorktree.ageOut: %v", err)
		return
	}
	open, err := repos.OpenChanges(ctx, g.agentwtPath, g.repoRoot)
	if err != nil {
		log.Printf("groupWorktree.ageOut: %v", err)
		return
	}

	kept := branches[:min(max(window-1, 0), len(branches))]
	probes := g.probeKept(ctx, open, kept)
	for index := range open {
		change := &open[index]
		reason := dropReason(change, kept, probes)
		if reason == "" {
			continue
		}
		if err := repos.Drop(ctx, g.repoRoot, change.Commit.Sha); err != nil {
			log.Printf("groupWorktree.ageOut: Change %s: %v", change.Commit.ShortSha, err)
			continue
		}
		log.Printf("groupWorktree.ageOut: Change %s: dropped: %s", change.Commit.ShortSha, reason)
	}
}

// archiveBranch writes the branch's diff against main into the trash and journals its sha before a -D; a failed archive keeps the branch.
func (g *groupWorktree) archiveBranch(ctx context.Context, branch, action string) error {
	dest, err := g.trash.StampDir()
	if err != nil {
		return fmt.Errorf("groupWorktree.archiveBranch: %w", err)
	}

	sha, err := gitwt.ArchiveBranch(ctx, mainBranch, branch, filepath.Join(dest, "branches"), g.repoRoot)
	if err != nil {
		return fmt.Errorf("groupWorktree.archiveBranch: %w", err)
	}

	entry := &trash.Entry{
		Action: action,
		Kind:   trash.KindBranchDeleted,
		Path:   g.repoRoot + "#" + branch,
		Reason: "sha " + sha,
		Stash:  g.trash.RelOf(dest) + "/branches",
	}
	if err := g.trash.Note(entry); err != nil {
		return fmt.Errorf("groupWorktree.archiveBranch: %w", err)
	}
	return nil
}

// archiveForeign moves a directory the runner did not create aside, lossless; the user deletes the archive once seen.
func (g *groupWorktree) archiveForeign() error {
	archiveDir := filepath.Join(filepath.Dir(g.path), staleArchiveDir)
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return fmt.Errorf("groupWorktree.archiveForeign: %w", err)
	}
	archive := filepath.Join(archiveDir, time.Now().UTC().Format(archiveStampFormat)+"-"+filepath.Base(g.path))
	if err := os.Rename(g.path, archive); err != nil {
		return fmt.Errorf("groupWorktree.archiveForeign: %w", err)
	}
	log.Printf("groupWorktree.archiveForeign: %s is not a worktree of this repo, moved to %s", g.path, archive)
	return nil
}

func (g *groupWorktree) branchExists(ctx context.Context, name string) bool {
	_, err := git(ctx, g.repoRoot, "rev-parse", "--quiet", "--verify", "refs/heads/"+name)
	return err == nil
}

// branches lists the group's dated branches, newest commit first.
func (g *groupWorktree) branches(ctx context.Context) ([]string, error) {
	output, err := git(
		ctx,
		g.repoRoot,
		"for-each-ref",
		"--sort=-committerdate",
		"--format=%(refname:short)",
		"refs/heads/"+g.branchPrefix+"-*",
	)
	if err != nil {
		return nil, fmt.Errorf("groupWorktree.branches: %w", err)
	}
	return nonEmptyLines(output), nil
}

// branchesWithSuffix lists the group's branches ending in suffix, newest commit first.
func (g *groupWorktree) branchesWithSuffix(ctx context.Context, suffix string) ([]string, error) {
	output, err := git(
		ctx,
		g.repoRoot,
		"for-each-ref",
		"--sort=-committerdate",
		"--format=%(refname:short)",
		"refs/heads/"+g.branchPrefix+"-*"+suffix,
	)
	if err != nil {
		return nil, fmt.Errorf("groupWorktree.branchesWithSuffix: %w", err)
	}
	return nonEmptyLines(output), nil
}

// chainBase skips failure-only branches but never deletes them.
func (g *groupWorktree) chainBase(ctx context.Context) (string, error) {
	branches, err := g.branches(ctx)
	if err != nil {
		return "", err
	}
	for _, branch := range branches {
		if !g.holdsOnlyFailures(ctx, branch) {
			return branch, nil
		}
	}
	return mainBranch, nil
}

func (g *groupWorktree) checkOpenLimit(ctx context.Context) error {
	if g.maxOpenBranches == 0 {
		return nil
	}

	branches, err := g.branches(ctx)
	if err != nil {
		return err
	}
	if len(branches) >= g.maxOpenBranches {
		return errOpenBranchLimit
	}
	return nil
}

// commitIfDirty writes the body back on a refused commit so the next attempt still carries it.
func (g *groupWorktree) commitIfDirty(ctx context.Context, subject, body string) error {
	if _, err := commitChanges(ctx, body, g.path, nil, subject); err != nil {
		g.restoreCommitBody(body)
		return fmt.Errorf("groupWorktree.commitIfDirty: %w", err)
	}
	return nil
}

func (g *groupWorktree) create(ctx context.Context, resolveConflicts func(context.Context) error) (string, error) {
	if err := g.sweep(ctx); err != nil {
		return "", err
	}
	g.pruneMerged(ctx)

	base, err := g.chainBase(ctx)
	if err != nil {
		return "", err
	}
	if err := g.checkOpenLimit(ctx); err != nil {
		return "", err
	}

	branch := g.freshStem(ctx)
	if err := os.MkdirAll(filepath.Dir(g.path), 0o755); err != nil {
		return "", fmt.Errorf("groupWorktree.create: %w", err)
	}
	if _, err := git(ctx, g.repoRoot, "worktree", "add", "--quiet", "-b", branch, g.path, base); err != nil {
		return "", fmt.Errorf("groupWorktree.create: %w", err)
	}
	if base == mainBranch {
		return branch, nil
	}

	if err := g.syncMain(ctx, resolveConflicts); err != nil {
		g.discard(ctx, branch, base)
		return "", fmt.Errorf("groupWorktree.create: %w", err)
	}

	head, err := git(ctx, g.path, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		g.discard(ctx, branch, base)
		return "", fmt.Errorf("groupWorktree.create: %w", err)
	}
	if head != branch {
		g.discard(ctx, branch, base)
		return "", fmt.Errorf("groupWorktree.create: HEAD left on %s instead of %s", head, branch)
	}
	return branch, nil
}

// createNight cuts the proposals branch at main; prepareNight has run.
func (g *groupWorktree) createNight(ctx context.Context) (string, error) {
	branch := g.freshStem(ctx) + repos.ProposalsSuffix
	if err := os.MkdirAll(filepath.Dir(g.path), 0o755); err != nil {
		return "", fmt.Errorf("groupWorktree.createNight: %w", err)
	}
	if _, err := git(ctx, g.repoRoot, "worktree", "add", "--quiet", "-b", branch, g.path, mainBranch); err != nil {
		return "", fmt.Errorf("groupWorktree.createNight: %w", err)
	}

	batches, err := g.branchesWithSuffix(ctx, repos.BatchSuffix)
	if err != nil {
		g.discard(ctx, branch, mainBranch)
		return "", err
	}
	if len(batches) == 0 {
		return branch, nil
	}
	if err := g.seedLedger(ctx, batches[0]); err != nil {
		g.discard(ctx, branch, mainBranch)
		return "", fmt.Errorf("groupWorktree.createNight: %w", err)
	}
	return branch, nil
}

func (g *groupWorktree) deleteBranch(ctx context.Context, branch string) {
	if _, err := git(ctx, g.repoRoot, "branch", "-d", branch); err != nil {
		log.Printf("groupWorktree.deleteBranch: %s: %v", branch, err)
	}
}

// deleteSettledProposals deletes every older proposals branch whose changes are all placed, rejected or dropped.
func (g *groupWorktree) deleteSettledProposals(ctx context.Context, current string) {
	branches, err := g.branchesWithSuffix(ctx, repos.ProposalsSuffix)
	if err != nil {
		log.Printf("groupWorktree.deleteSettledProposals: %v", err)
		return
	}
	open, err := repos.OpenChanges(ctx, g.agentwtPath, g.repoRoot)
	if err != nil {
		log.Printf("groupWorktree.deleteSettledProposals: %v", err)
		return
	}

	older := slices.DeleteFunc(branches, func(branch string) bool { return branch == current })
	for _, branch := range repos.SettledBranches(older, open) {
		if err := g.archiveBranch(ctx, branch, "nightly settled proposals"); err != nil {
			log.Printf("groupWorktree.deleteSettledProposals: %s kept: %v", branch, err)
			continue
		}

		if _, err := git(ctx, g.repoRoot, "branch", "-D", branch); err != nil {
			log.Printf("groupWorktree.deleteSettledProposals: %s kept: %v", branch, err)
		}
	}
}

// discard deletes the ref only while it still names base's commit, so nothing it holds is lost.
func (g *groupWorktree) discard(ctx context.Context, branch, base string) {
	if err := g.removeWorktree(ctx); err != nil {
		log.Printf("groupWorktree.discard: %v", err)
		return
	}

	baseSha, err := git(ctx, g.repoRoot, "rev-parse", "refs/heads/"+base)
	if err != nil {
		log.Printf("groupWorktree.discard: %s kept: %v", branch, err)
		return
	}
	if _, err := git(ctx, g.repoRoot, "update-ref", "-d", "refs/heads/"+branch, baseSha); err != nil {
		log.Printf("groupWorktree.discard: %s kept, it moved past %s: %v", branch, base, err)
	}
}

// probeKept probes the open changes of the kept nights in one agentwt call; a failed probe answers nothing, so every change stays.
func (g *groupWorktree) probeKept(ctx context.Context, open []repos.OpenChange, kept []string) map[string]repos.ProbeEntry {
	shas := make([]string, 0, len(open))
	for _, change := range open {
		if slices.Contains(kept, change.Branch) {
			shas = append(shas, change.Commit.Sha)
		}
	}

	probe, err := repos.Probe(ctx, g.agentwtPath, mainBranch, g.repoRoot, shas)
	if err != nil {
		log.Printf("groupWorktree.probeKept: %v", err)
		return map[string]repos.ProbeEntry{}
	}
	return probe.ByCommit()
}

// forceDeleteBranch deletes a branch whose commits live on elsewhere: picked onto main or never checked out.
func (g *groupWorktree) forceDeleteBranch(ctx context.Context, branch string) {
	if err := g.archiveBranch(ctx, branch, "nightly landing"); err != nil {
		log.Printf("groupWorktree.forceDeleteBranch: %s kept: %v", branch, err)
		return
	}

	if _, err := git(ctx, g.repoRoot, "branch", "-D", branch); err != nil {
		log.Printf("groupWorktree.forceDeleteBranch: %s kept: %v", branch, err)
	}
}

// freshStem is the day stem a single routine's branch and both branches of a night hang off; a taken stem gets a counter.
func (g *groupWorktree) freshStem(ctx context.Context) string {
	day := time.Now().UTC().Format(time.DateOnly)
	stem := g.branchPrefix + "-" + day
	for suffix := 2; g.isStemTaken(ctx, stem); suffix++ {
		stem = fmt.Sprintf("%s-%s-%d", g.branchPrefix, day, suffix)
	}
	return stem
}

func (g *groupWorktree) holdsOnlyFailures(ctx context.Context, branch string) bool {
	output, err := git(ctx, g.repoRoot, "log", "--format=%s", mainBranch+".."+branch)
	if err != nil {
		return false
	}

	subjects := nonEmptyLines(output)
	if len(subjects) == 0 {
		return false
	}
	for _, subject := range subjects {
		isMarked := strings.Contains(subject, failedMarker) || strings.Contains(subject, salvagedMarker)
		if !isMarked {
			return false
		}
	}
	return true
}

func (g *groupWorktree) isMerged(ctx context.Context, branch string) bool {
	_, err := git(ctx, g.repoRoot, "merge-base", "--is-ancestor", branch, mainBranch)
	return err == nil
}

// isRegistered asks git, not the filesystem: a worktree is what the repo lists; git lists the resolved path.
func (g *groupWorktree) isRegistered(ctx context.Context) (bool, error) {
	if !g.pathExists() {
		return false, nil
	}
	resolved, err := filepath.EvalSymlinks(g.path)
	if err != nil {
		return false, fmt.Errorf("groupWorktree.isRegistered: %w", err)
	}

	output, err := git(ctx, g.repoRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("groupWorktree.isRegistered: %w", err)
	}
	return slices.Contains(nonEmptyLines(output), "worktree "+resolved), nil
}

func (g *groupWorktree) isStemTaken(ctx context.Context, stem string) bool {
	isBareTaken := g.branchExists(ctx, stem)
	isNightTaken := g.branchExists(ctx, stem+repos.ProposalsSuffix) || g.branchExists(ctx, stem+repos.BatchSuffix)
	return isBareTaken || isNightTaken
}

func (g *groupWorktree) land(ctx context.Context, isFailed bool) error {
	if err := g.stripProjections(ctx); err != nil {
		return fmt.Errorf("groupWorktree.land: %w", err)
	}

	marker := ""
	if isFailed {
		marker = failedMarker + " "
	}
	subject := fmt.Sprintf("fleet: %sRecorded worker %s output", marker, g.routineName)
	if err := g.commitIfDirty(ctx, subject, g.takeCommitBody()); err != nil {
		return fmt.Errorf("groupWorktree.land: %w", err)
	}
	return nil
}

// landBatchBranch picks every unplaced commit of a batch branch onto main; returns the reason it stays, "" when it is gone.
func (g *groupWorktree) landBatchBranch(ctx context.Context, branch string) string {
	unplaced, err := repos.ListUnplaced(ctx, g.agentwtPath, branch, mainBranch, g.repoRoot)
	if err != nil {
		return err.Error()
	}
	for _, commit := range unplaced.Commits {
		pick, err := repos.Pick(ctx, g.agentwtPath, mainBranch, g.repoRoot, commit.Sha)
		if err != nil {
			return err.Error()
		}
		if pick.Outcome != repos.PickOutcomePicked {
			return pick.Outcome + ": " + pick.Reason
		}
	}
	if err := g.archiveBranch(ctx, branch, "nightly batch landing"); err != nil {
		return err.Error()
	}
	if _, err := git(ctx, g.repoRoot, "branch", "-D", branch); err != nil {
		return err.Error()
	}
	return ""
}

// landOpenBatches retries yesterday's records: a batch branch with nothing unplaced is deleted, one that will not pick stays.
func (g *groupWorktree) landOpenBatches(ctx context.Context) {
	batches, err := g.branchesWithSuffix(ctx, repos.BatchSuffix)
	if err != nil {
		log.Printf("groupWorktree.landOpenBatches: %v", err)
		return
	}
	slices.Reverse(batches)
	for _, branch := range batches {
		if reason := g.landBatchBranch(ctx, branch); reason != "" {
			log.Printf("groupWorktree.landOpenBatches: %s kept: %s", branch, reason)
		}
	}
}

func (g *groupWorktree) pathExists() bool {
	_, err := os.Stat(g.path)
	return err == nil
}

// prepareNight lands yesterday's records and prunes merged branches; the run decides afterwards whether a night is cut.
func (g *groupWorktree) prepareNight(ctx context.Context) error {
	if err := g.sweep(ctx); err != nil {
		return err
	}
	g.pruneMerged(ctx)
	g.landOpenBatches(ctx)
	return nil
}

func (g *groupWorktree) pruneMerged(ctx context.Context) {
	branches, err := g.branches(ctx)
	if err != nil {
		log.Printf("groupWorktree.pruneMerged: %v", err)
		return
	}
	for _, branch := range branches {
		if g.isMerged(ctx, branch) {
			g.deleteBranch(ctx, branch)
		}
	}
}

// publish keeps the worktree and the body on a failed commit, for the next run's salvage.
func (g *groupWorktree) publish(ctx context.Context, exitStatus int, marker string) error {
	if _, err := os.Stat(g.path); err != nil {
		return fmt.Errorf("groupWorktree.publish: Worktree vanished: %s", g.path)
	}

	if err := g.stripProjections(ctx); err != nil {
		return fmt.Errorf("groupWorktree.publish: %w", err)
	}
	body := g.takeCommitBody()
	prefix := ""
	if marker != "" {
		prefix = marker + " "
	}
	subject := fmt.Sprintf(
		"sessions: %sRecorded nightly %s output (%s, exit %d)",
		prefix,
		g.routineName,
		time.Now().UTC().Format(time.DateOnly),
		exitStatus,
	)

	if err := g.commitIfDirty(ctx, subject, body); err != nil {
		return fmt.Errorf("groupWorktree.publish: %w", err)
	}
	return g.removeWorktree(ctx)
}

// publishNight commits leftovers on the proposals branch and the records on a fresh batch branch at main's tip.
func (g *groupWorktree) publishNight(ctx context.Context, exitStatus int, marker, night string) (string, error) {
	if _, err := os.Stat(g.path); err != nil {
		return "", fmt.Errorf("groupWorktree.publishNight: Worktree vanished: %s", g.path)
	}
	if err := g.stripProjections(ctx); err != nil {
		return "", fmt.Errorf("groupWorktree.publishNight: %w", err)
	}

	body := g.takeCommitBody()
	leftoverSubject := fmt.Sprintf(
		"smine: %s Recorded leftover %s worktree state (%s)",
		leftoverMarker,
		g.routineName,
		time.Now().UTC().Format(time.DateOnly),
	)
	leftoverPaths := []string{".", ":!" + ledgerDir}
	if _, err := commitChanges(ctx, body, g.path, leftoverPaths, leftoverSubject); err != nil {
		g.restoreCommitBody(body)
		return "", fmt.Errorf("groupWorktree.publishNight: %w", err)
	}

	batch := strings.TrimSuffix(night, repos.ProposalsSuffix) + repos.BatchSuffix
	if _, err := git(ctx, g.path, "switch", "--quiet", "-c", batch, mainBranch); err != nil {
		return "", fmt.Errorf("groupWorktree.publishNight: %w", err)
	}
	prefix := ""
	if marker != "" {
		prefix = marker + " "
	}
	subject := fmt.Sprintf(
		"sessions: %sRecorded nightly %s output (%s, exit %d)",
		prefix,
		g.routineName,
		time.Now().UTC().Format(time.DateOnly),
		exitStatus,
	)
	isCommitted, err := commitChanges(ctx, "", g.path, []string{ledgerDir}, subject)
	if err != nil {
		return "", fmt.Errorf("groupWorktree.publishNight: %w", err)
	}
	if err := g.removeWorktree(ctx); err != nil {
		return "", err
	}
	if !isCommitted {
		g.deleteBranch(ctx, batch)
		return "", nil
	}
	return batch, nil
}

// A failed regeneration leaves the cell without a context index, never without its run.
func (g *groupWorktree) regenerateContext() {
	if err := contextdocs.Regenerate(filepath.Join(g.path, contextdocs.SourceContextDir)); err != nil {
		log.Printf("groupWorktree.regenerateContext: %v", err)
	}
}

func (g *groupWorktree) release(ctx context.Context) error {
	// An unlocked worktree answers the unlock with an error; only the removal decides.
	_, _ = git(ctx, g.repoRoot, "worktree", "unlock", g.path)
	return g.removeWorktree(ctx)
}

func (g *groupWorktree) removeWorktree(ctx context.Context) error {
	if _, err := git(ctx, g.repoRoot, "worktree", "remove", g.path); err != nil {
		return fmt.Errorf("groupWorktree.removeWorktree: %s kept: %w", g.path, err)
	}
	return nil
}

func (g *groupWorktree) restoreCommitBody(body string) {
	if body == "" {
		return
	}
	if err := os.WriteFile(filepath.Join(g.path, commitBodyFileName), []byte(body+"\n"), 0o644); err != nil {
		log.Printf("groupWorktree.restoreCommitBody: %v", err)
	}
}

func (g *groupWorktree) salvage(ctx context.Context) error {
	if err := g.stripProjections(ctx); err != nil {
		return fmt.Errorf("groupWorktree.salvage: %w", err)
	}

	subject := fmt.Sprintf(
		"sessions: %s Recovered leftover %s worktree state (%s)",
		salvagedMarker,
		g.routineName,
		time.Now().UTC().Format(time.DateOnly),
	)
	if err := g.commitIfDirty(ctx, subject, g.takeCommitBody()); err != nil {
		return fmt.Errorf("groupWorktree.salvage: %w", err)
	}
	return nil
}

// seedLedger never touches the index, so a change commit made during the run cannot carry the records.
func (g *groupWorktree) seedLedger(ctx context.Context, night string) error {
	if _, err := git(ctx, g.repoRoot, "cat-file", "-e", night+":"+ledgerDir); err != nil {
		return nil
	}
	if _, err := git(ctx, g.path, "restore", "--source="+night, "--worktree", "--", ledgerDir); err != nil {
		return fmt.Errorf("groupWorktree.seedLedger: %w", err)
	}
	return nil
}

// stripProjections removes the Read hook's ACDSL blocks, which the pre-commit guard refuses.
func (g *groupWorktree) stripProjections(ctx context.Context) error {
	if _, err := os.Stat(filepath.Join(g.path, "acdsl", "registry.json")); err != nil {
		return nil
	}

	universe, err := acdsl.FileUniverse(ctx, g.path)
	if err != nil {
		return fmt.Errorf("groupWorktree.stripProjections: %w", err)
	}
	if _, err := acdsl.StripAll(g.path, universe); err != nil {
		return fmt.Errorf("groupWorktree.stripProjections: %w", err)
	}
	return nil
}

// sweep never touches a sibling group's worktree; a leftover it cannot clear blocks the run.
func (g *groupWorktree) sweep(ctx context.Context) error {
	isRegistered, err := g.isRegistered(ctx)
	if err != nil {
		return fmt.Errorf("groupWorktree.sweep: %w", err)
	}
	switch {
	case isRegistered:
		if err := g.salvage(ctx); err != nil {
			return fmt.Errorf("groupWorktree.sweep: %w", err)
		}
		if err := g.removeWorktree(ctx); err != nil {
			return fmt.Errorf("groupWorktree.sweep: %w", err)
		}
	case g.pathExists():
		if err := g.archiveForeign(); err != nil {
			return fmt.Errorf("groupWorktree.sweep: %w", err)
		}
	}
	if _, err := git(ctx, g.repoRoot, "worktree", "prune"); err != nil {
		log.Printf("groupWorktree.sweep: %v", err)
	}
	return nil
}

func (g *groupWorktree) syncMain(ctx context.Context, resolveConflicts func(context.Context) error) error {
	if _, err := git(ctx, g.path, "merge", "--no-edit", "--quiet", mainBranch); err == nil {
		return nil
	}
	_, _ = git(ctx, g.path, "merge", "--abort")

	log.Printf("groupWorktree.syncMain: Chain diverged from main with conflicts, running unattended /merge-resolve")
	if err := resolveConflicts(ctx); err != nil {
		log.Printf("groupWorktree.syncMain: %v", err)
	}

	status, err := git(ctx, g.path, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("groupWorktree.syncMain: %w", err)
	}
	if status != "" {
		return errors.New("groupWorktree.syncMain: Merge-resolve left the worktree dirty")
	}

	if _, err := git(ctx, g.path, "merge-tree", "--write-tree", mainBranch, "HEAD"); err != nil {
		return fmt.Errorf("groupWorktree.syncMain: Chain still conflicts with main: %w", err)
	}
	return nil
}

// takeCommitBody removes the body file so it is never staged.
func (g *groupWorktree) takeCommitBody() string {
	path := filepath.Join(g.path, commitBodyFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	_ = os.Remove(path)
	return strings.TrimRight(string(data), "\n")
}

// dropReason is empty for a change that stays: on a kept night and not conflicting with main; a change the probe did not answer stays too.
func dropReason(change *repos.OpenChange, kept []string, probes map[string]repos.ProbeEntry) string {
	if !slices.Contains(kept, change.Branch) {
		return "outside the window of " + strconv.Itoa(len(kept)+1) + " nights"
	}

	probe, ok := probes[change.Commit.Sha]
	if !ok {
		return ""
	}
	switch probe.Outcome {
	case repos.ProbeOutcomeConflict:
		return "no longer applies on main: " + probe.Reason
	case repos.ProbeOutcomeApplies, repos.ProbeOutcomeSkipped:
		return ""
	}
	log.Printf("dropReason: Change %s: Unknown outcome %s", change.Commit.ShortSha, probe.Outcome)
	return ""
}

func exitMarker(exitStatus int) string {
	if exitStatus == exitOk {
		return ""
	}
	return failedMarker
}

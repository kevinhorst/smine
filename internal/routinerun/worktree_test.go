package routinerun

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/acdsl"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/trash"
)

// gitRun runs git in dir for test setup and returns trimmed stdout.
func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	output, err := git(context.Background(), dir, args...)
	require.NoError(t, err)
	return output
}

// newTestRepo is a local repo on main with one commit and a committed
// .gitignore for runner scratch.
func newTestRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	cmd := exec.Command("git", "init", "-q", "-b", "main", repo)
	require.NoError(t, cmd.Run())
	gitRun(t, repo, "config", "user.email", "test@example.com")
	gitRun(t, repo, "config", "user.name", "test")
	require.NoError(t, os.WriteFile(filepath.Join(repo, "README"), []byte("repo\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".gitignore"), []byte(".cell-pids\n"), 0o644))
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-qm", "init")
	return repo
}

func newTestGroup(t *testing.T, repo, group string, maxOpen int) *groupWorktree {
	t.Helper()
	worktree := &groupWorktree{
		agentwtPath:     installFakeAgentwt(t, t.TempDir()),
		branchPrefix:    "claude-routines/" + group,
		maxOpenBranches: maxOpen,
		path:            filepath.Join(t.TempDir(), group),
		repoRoot:        repo,
		routineName:     group,
		trash:           trash.New(t.TempDir(), 0),
	}
	return worktree
}

// journalPaths lists the paths of the store's branch-deleted journal lines written by action, oldest first.
func journalPaths(t *testing.T, store *trash.Store, action string) []string {
	t.Helper()
	journal, err := store.Tail(100)
	require.NoError(t, err)
	paths := make([]string, 0)
	for _, entry := range slices.Backward(journal) {
		isWitness := entry.Kind == trash.KindBranchDeleted
		if isWitness && entry.Action == action {
			paths = append(paths, entry.Path)
		}
	}
	return paths
}

func today() string {
	return time.Now().UTC().Format(time.DateOnly)
}

func noResolve(context.Context) error {
	return errors.New("unexpected conflict")
}

// commitInWorktree creates a file in the group worktree and publishes it.
func commitInWorktree(t *testing.T, worktree *groupWorktree, name string, status int) {
	t.Helper()
	publishInWorktree(t, worktree, name, status, exitMarker(status))
}

// publishInWorktree creates a file in the group worktree and publishes it
// under an explicit subject marker.
func publishInWorktree(t *testing.T, worktree *groupWorktree, name string, status int, marker string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(worktree.path, name), []byte(name+"\n"), 0o644))
	require.NoError(t, worktree.publish(context.Background(), status, marker))
}

func TestGroupWorktree_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("fresh-branch-from-main", func(t *testing.T) {
		repo := newTestRepo(t)
		worktree := newTestGroup(t, repo, "demo", 0)

		branch, err := worktree.create(ctx, noResolve)

		require.NoError(t, err)
		assert.Equal(t, "claude-routines/demo-"+today(), branch)
		assert.Equal(t, gitRun(t, repo, "rev-parse", "main"), gitRun(t, worktree.path, "rev-parse", "HEAD"))
	})

	t.Run("same-day-suffix-and-chain-tip-stacking", func(t *testing.T) {
		repo := newTestRepo(t)
		worktree := newTestGroup(t, repo, "demo", 0)
		first, err := worktree.create(ctx, noResolve)
		require.NoError(t, err)
		commitInWorktree(t, worktree, "one.txt", 0)

		second, err := worktree.create(ctx, noResolve)

		require.NoError(t, err)
		assert.Equal(t, first+"-2", second)
		assert.FileExists(t, filepath.Join(worktree.path, "one.txt"))
	})

	t.Run("prunes-merged", func(t *testing.T) {
		repo := newTestRepo(t)
		worktree := newTestGroup(t, repo, "demo", 0)
		first, err := worktree.create(ctx, noResolve)
		require.NoError(t, err)
		commitInWorktree(t, worktree, "one.txt", 0)
		gitRun(t, repo, "merge", "--quiet", "--no-edit", first)

		second, err := worktree.create(ctx, noResolve)

		require.NoError(t, err)
		assert.Equal(t, first, second, "the merged branch was pruned, so its name is free again")
		assert.Equal(t, gitRun(t, repo, "rev-parse", "main"), gitRun(t, worktree.path, "rev-parse", "HEAD"))
	})

	t.Run("skips-failed-chain-keeps-it", func(t *testing.T) {
		repo := newTestRepo(t)
		worktree := newTestGroup(t, repo, "demo", 0)
		failed, err := worktree.create(ctx, noResolve)
		require.NoError(t, err)
		commitInWorktree(t, worktree, "failed.txt", 1)

		_, err = worktree.create(ctx, noResolve)

		require.NoError(t, err)
		branches, err := worktree.branches(ctx)
		require.NoError(t, err)
		assert.Len(t, branches, 2, "the failed chain stays for review beside the fresh branch")
		assert.True(t, worktree.branchExists(ctx, failed))
		assert.NoFileExists(t, filepath.Join(worktree.path, "failed.txt"))
	})

	t.Run("stacks-on-stage-failed-run", func(t *testing.T) {
		repo := newTestRepo(t)
		worktree := newTestGroup(t, repo, "demo", 0)
		_, err := worktree.create(ctx, noResolve)
		require.NoError(t, err)
		publishInWorktree(t, worktree, "partial.txt", exitSoftware, "[stage-failed: style]")

		_, err = worktree.create(ctx, noResolve)

		require.NoError(t, err)
		assert.FileExists(t, filepath.Join(worktree.path, "partial.txt"), "a finished run with a failed stage is reviewable output")
	})

	t.Run("salvages-leftover-worktree", func(t *testing.T) {
		repo := newTestRepo(t)
		worktree := newTestGroup(t, repo, "demo", 0)
		leftover, err := worktree.create(ctx, noResolve)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(worktree.path, "record.md"), []byte("record\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(worktree.path, commitBodyFileName), []byte("kept body\n"), 0o644))
		gitRun(t, worktree.path, "add", "record.md")

		_, err = worktree.create(ctx, noResolve)

		require.NoError(t, err)
		subject := gitRun(t, repo, "log", "-1", "--format=%s", leftover)
		assert.True(t, strings.HasPrefix(subject, "sessions: [salvaged] Recovered leftover demo worktree state"))
		assert.Equal(t, "kept body", gitRun(t, repo, "log", "-1", "--format=%b", leftover))
		assert.Contains(t, gitRun(t, repo, "show", "--name-only", "--format=", leftover), "record.md")
	})

	t.Run("locked-leftover-blocks-run", func(t *testing.T) {
		repo := newTestRepo(t)
		worktree := newTestGroup(t, repo, "demo", 0)
		_, err := worktree.create(ctx, noResolve)
		require.NoError(t, err)
		gitRun(t, repo, "worktree", "lock", worktree.path)

		_, err = worktree.create(ctx, noResolve)

		require.Error(t, err)
		assert.DirExists(t, worktree.path)
	})

	t.Run("keeps-chain-with-real-commit", func(t *testing.T) {
		repo := newTestRepo(t)
		worktree := newTestGroup(t, repo, "demo", 0)
		_, err := worktree.create(ctx, noResolve)
		require.NoError(t, err)
		t.Setenv("GIT_COMMITTER_DATE", "2026-09-01T10:00:00Z")
		commitInWorktree(t, worktree, "real.txt", 0)
		_, err = worktree.create(ctx, noResolve)
		require.NoError(t, err)
		// Distinct committer dates: the chain tip is the newest by committerdate.
		t.Setenv("GIT_COMMITTER_DATE", "2026-09-02T10:00:00Z")
		commitInWorktree(t, worktree, "failed.txt", 1)

		_, err = worktree.create(ctx, noResolve)

		require.NoError(t, err)
		assert.FileExists(t, filepath.Join(worktree.path, "real.txt"))
		assert.FileExists(t, filepath.Join(worktree.path, "failed.txt"))
	})

	t.Run("open-branch-limit", func(t *testing.T) {
		repo := newTestRepo(t)
		worktree := newTestGroup(t, repo, "demo", 1)
		_, err := worktree.create(ctx, noResolve)
		require.NoError(t, err)
		commitInWorktree(t, worktree, "one.txt", 0)

		_, err = worktree.create(ctx, noResolve)

		assert.ErrorIs(t, err, errOpenBranchLimit)
	})

	t.Run("sibling-group-untouched", func(t *testing.T) {
		repo := newTestRepo(t)
		sibling := newTestGroup(t, repo, "other", 0)
		siblingBranch, err := sibling.create(ctx, noResolve)
		require.NoError(t, err)
		worktree := newTestGroup(t, repo, "demo", 0)

		_, err = worktree.create(ctx, noResolve)

		require.NoError(t, err)
		assert.True(t, sibling.branchExists(ctx, siblingBranch))
		assert.DirExists(t, sibling.path)
	})
}

// nightGroup is the nightly's group name; its prefix is the one repos.OpenChanges scans.
const nightGroup = "smine-nightly"

// nightBranch names one branch of the group's night on day.
func nightBranch(day, suffix string) string {
	return "claude-routines/" + nightGroup + "-" + day + suffix
}

// commitOnBranch commits rel on branch, cutting the branch at main when it is new, and returns the commit.
func commitOnBranch(t *testing.T, repo, branch, rel, message string) string {
	t.Helper()
	if _, err := git(context.Background(), repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); err != nil {
		gitRun(t, repo, "branch", branch, "main")
	}
	gitRun(t, repo, "checkout", "-q", branch)
	path := filepath.Join(repo, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(message+"\n"), 0o644))
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-qm", message)
	sha := gitRun(t, repo, "rev-parse", "HEAD")
	gitRun(t, repo, "checkout", "-q", "main")
	return sha
}

// commitFilesOnBranch commits one one-line file per rel on branch in a single commit, cutting the branch at main when it is new, and returns the commit.
func commitFilesOnBranch(t *testing.T, repo, branch string, files []string, message string) string {
	t.Helper()
	if _, err := git(context.Background(), repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); err != nil {
		gitRun(t, repo, "branch", branch, "main")
	}
	gitRun(t, repo, "checkout", "-q", branch)
	for _, rel := range files {
		path := filepath.Join(repo, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(rel+"\n"), 0o644))
	}
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-qm", message)
	sha := gitRun(t, repo, "rev-parse", "HEAD")
	gitRun(t, repo, "checkout", "-q", "main")
	return sha
}

// droppedShas lists the repo's dropped refs.
func droppedShas(t *testing.T, repo string) []string {
	t.Helper()
	settled, err := repos.SettledChanges(context.Background(), repo)
	require.NoError(t, err)
	shas := make([]string, 0, len(settled))
	for _, change := range settled {
		if change.Outcome == repos.SettledOutcomeDropped {
			shas = append(shas, change.Sha)
		}
	}
	return shas
}

// newFailingProbeGroup is a nightly group whose agentwt lists unplaced commits but fails every probe without a document.
func newFailingProbeGroup(t *testing.T, repo string) *groupWorktree {
	t.Helper()
	worktree := newTestGroup(t, repo, nightGroup, 0)
	script := "#!/bin/sh\nif [ \"$5\" = probe ]; then echo 'agentwt: boom' >&2; exit 1; fi\nexec '" + worktree.agentwtPath + "' \"$@\"\n"
	wrapper := filepath.Join(t.TempDir(), "agentwt-probe-fails")
	require.NoError(t, os.WriteFile(wrapper, []byte(script), 0o755))
	worktree.agentwtPath = wrapper
	return worktree
}

func TestGroupWorktree_AgeOut(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedDropped []string

		window   int
		worktree *groupWorktree
	}

	tests := make([]*testCase, 0)

	// older-night-dropped
	olderRepo := newTestRepo(t)
	oldest := commitOnBranch(t, olderRepo, nightBranch("2026-01-01", repos.ProposalsSuffix), "a.txt", "skills.a: Fixed a\n\nSmine-Change: skills/a")
	commitOnBranch(t, olderRepo, nightBranch("2026-01-02", repos.ProposalsSuffix), "b.txt", "skills.b: Fixed b\n\nSmine-Change: skills/b")
	commitOnBranch(t, olderRepo, nightBranch("2026-01-03", repos.ProposalsSuffix), "c.txt", "skills.c: Fixed c\n\nSmine-Change: skills/c")
	tests = append(tests, &testCase{
		_id:              "older-night-dropped",
		_expectedDropped: []string{oldest},

		window:   3,
		worktree: newTestGroup(t, olderRepo, nightGroup, 0),
	})

	// window-one-keeps-only-tonight
	windowOneRepo := newTestRepo(t)
	first := commitOnBranch(t, windowOneRepo, nightBranch("2026-01-01", repos.ProposalsSuffix), "a.txt", "skills.a: Fixed a\n\nSmine-Change: skills/a")
	second := commitOnBranch(t, windowOneRepo, nightBranch("2026-01-02", repos.ProposalsSuffix), "b.txt", "skills.b: Fixed b\n\nSmine-Change: skills/b")
	tests = append(tests, &testCase{
		_id:              "window-one-keeps-only-tonight",
		_expectedDropped: []string{first, second},

		window:   1,
		worktree: newTestGroup(t, windowOneRepo, nightGroup, 0),
	})

	// stale-change-dropped
	staleRepo := newTestRepo(t)
	stale := commitOnBranch(t, staleRepo, nightBranch("2026-01-01", repos.ProposalsSuffix), "a.txt", "skills.a: Fixed a\n\nSmine-Change: skills/a")
	commitOnBranch(t, staleRepo, nightBranch("2026-01-01", repos.ProposalsSuffix), "b.txt", "skills.b: Fixed b\n\nSmine-Change: skills/b")
	commitFile(t, staleRepo, "a.txt", "main side\n")
	tests = append(tests, &testCase{
		_id:              "stale-change-dropped",
		_expectedDropped: []string{stale},

		window:   2,
		worktree: newTestGroup(t, staleRepo, nightGroup, 0),
	})

	// probe-error-keeps-change
	errorRepo := newTestRepo(t)
	commitOnBranch(t, errorRepo, nightBranch("2026-01-01", repos.ProposalsSuffix), "a.txt", "skills.a: Fixed a\n\nSmine-Change: skills/a")
	commitFile(t, errorRepo, "a.txt", "main side\n")
	tests = append(tests, &testCase{
		_id:              "probe-error-keeps-change",
		_expectedDropped: []string{},

		window:   2,
		worktree: newFailingProbeGroup(t, errorRepo),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			test.worktree.ageOut(context.Background(), test.window)

			assert.ElementsMatch(t, test._expectedDropped, droppedShas(t, test.worktree.repoRoot))
		})
	}
}

func TestGroupWorktree_CreateNight(t *testing.T) {
	ctx := context.Background()

	t.Run("cuts-proposals-at-main", func(t *testing.T) {
		repo := newTestRepo(t)
		worktree := newTestGroup(t, repo, nightGroup, 0)
		require.NoError(t, worktree.prepareNight(ctx))

		branch, err := worktree.createNight(ctx)

		require.NoError(t, err)
		assert.Equal(t, nightBranch(today(), repos.ProposalsSuffix), branch)
		assert.Equal(t, gitRun(t, repo, "rev-parse", "main"), gitRun(t, worktree.path, "rev-parse", "HEAD"))
	})

	t.Run("lands-open-batch-and-deletes-it", func(t *testing.T) {
		repo := newTestRepo(t)
		worktree := newTestGroup(t, repo, nightGroup, 0)
		batch := nightBranch("2026-01-01", repos.BatchSuffix)
		commitOnBranch(t, repo, batch, "sessions/a.md", "sessions: Recorded nightly output")
		require.NoError(t, worktree.prepareNight(ctx))

		_, err := worktree.createNight(ctx)

		require.NoError(t, err)
		assert.False(t, worktree.branchExists(ctx, batch))
		assert.Contains(t, gitRun(t, repo, "ls-tree", "-r", "--name-only", "main"), "sessions/a.md")
		assert.Equal(t, gitRun(t, repo, "rev-parse", "main"), gitRun(t, worktree.path, "rev-parse", "HEAD"))
	})

	t.Run("skipped-batch-kept-and-seeds-ledger", func(t *testing.T) {
		repo := newTestRepo(t)
		worktree := newTestGroup(t, repo, nightGroup, 0)
		batch := nightBranch("2026-01-01", repos.BatchSuffix)
		commitOnBranch(t, repo, batch, "sessions/a.md", "sessions: Recorded nightly output")
		require.NoError(t, os.WriteFile(filepath.Join(repo, "dirty.txt"), []byte("wip\n"), 0o644))
		require.NoError(t, worktree.prepareNight(ctx))

		_, err := worktree.createNight(ctx)

		require.NoError(t, err)
		assert.True(t, worktree.branchExists(ctx, batch))
		assert.NotContains(t, gitRun(t, repo, "ls-tree", "-r", "--name-only", "main"), "sessions/a.md")
		assert.FileExists(t, filepath.Join(worktree.path, "sessions", "a.md"))
		assert.Empty(t, gitRun(t, worktree.path, "diff", "--cached", "--name-only"))
	})

	t.Run("taken-stem-gets-counter", func(t *testing.T) {
		repo := newTestRepo(t)
		worktree := newTestGroup(t, repo, nightGroup, 0)
		commitOnBranch(t, repo, nightBranch(today(), repos.ProposalsSuffix), "leftover.txt", "smine: Leftover without a change trailer")
		require.NoError(t, worktree.prepareNight(ctx))

		branch, err := worktree.createNight(ctx)

		require.NoError(t, err)
		assert.Equal(t, nightBranch(today()+"-2", repos.ProposalsSuffix), branch)
	})

	t.Run("foreign-dir-archived", func(t *testing.T) {
		repo := newTestRepo(t)
		worktree := newTestGroup(t, repo, nightGroup, 0)
		require.NoError(t, os.MkdirAll(filepath.Join(worktree.path, ".idea"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(worktree.path, ".idea", "workspace.xml"), []byte("<project/>\n"), 0o644))
		require.NoError(t, worktree.prepareNight(ctx))

		branch, err := worktree.createNight(ctx)

		require.NoError(t, err)
		assert.Equal(t, nightBranch(today(), repos.ProposalsSuffix), branch)
		assert.Equal(t, gitRun(t, repo, "rev-parse", "main"), gitRun(t, worktree.path, "rev-parse", "HEAD"))
		assert.NoFileExists(t, filepath.Join(worktree.path, ".idea", "workspace.xml"))
		archives, err := filepath.Glob(filepath.Join(filepath.Dir(worktree.path), staleArchiveDir, "*-"+nightGroup, ".idea", "workspace.xml"))
		require.NoError(t, err)
		assert.Len(t, archives, 1)
	})
}

func TestGroupWorktree_PublishNight(t *testing.T) {
	ctx := context.Background()

	t.Run("leftover-on-proposals-records-on-batch", func(t *testing.T) {
		repo := newTestRepo(t)
		worktree := newTestGroup(t, repo, nightGroup, 0)
		night, err := worktree.createNight(ctx)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(worktree.path, "stray.txt"), []byte("stray\n"), 0o644))
		require.NoError(t, os.MkdirAll(filepath.Join(worktree.path, "sessions"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(worktree.path, "sessions", "r.md"), []byte("record\n"), 0o644))

		batch, err := worktree.publishNight(ctx, exitOk, "", night)

		require.NoError(t, err)
		assert.Equal(t, nightBranch(today(), repos.BatchSuffix), batch)
		leftoverSubject := gitRun(t, repo, "log", "-1", "--format=%s", night)
		assert.True(t, strings.HasPrefix(leftoverSubject, "smine: [leftover] Recorded leftover smine-nightly worktree state"), leftoverSubject)
		assert.Equal(t, "stray.txt", gitRun(t, repo, "show", "--name-only", "--format=", night))
		assert.Equal(t, gitRun(t, repo, "rev-parse", "main"), gitRun(t, repo, "rev-parse", batch+"^"))
		assert.Equal(t, "sessions/r.md", gitRun(t, repo, "show", "--name-only", "--format=", batch))
		assert.True(t, strings.HasPrefix(gitRun(t, repo, "log", "-1", "--format=%s", batch), "sessions: Recorded nightly smine-nightly output"))
		assert.NoDirExists(t, worktree.path)
	})

	t.Run("no-records-no-batch", func(t *testing.T) {
		repo := newTestRepo(t)
		worktree := newTestGroup(t, repo, nightGroup, 0)
		night, err := worktree.createNight(ctx)
		require.NoError(t, err)

		batch, err := worktree.publishNight(ctx, exitOk, "", night)

		require.NoError(t, err)
		assert.Empty(t, batch)
		assert.False(t, worktree.branchExists(ctx, nightBranch(today(), repos.BatchSuffix)))
		assert.Equal(t, gitRun(t, repo, "rev-parse", "main"), gitRun(t, repo, "rev-parse", night))
		assert.NoDirExists(t, worktree.path)
	})
}

func TestGroupWorktree_DeleteSettledProposals(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepo(t)
	worktree := newTestGroup(t, repo, nightGroup, 0)
	settled := nightBranch("2026-01-01", repos.ProposalsSuffix)
	rejected := commitOnBranch(t, repo, settled, "a.txt", "skills.a: Fixed a\n\nSmine-Change: skills/a")
	require.NoError(t, repos.Reject(ctx, repo, rejected))
	open := nightBranch("2026-01-02", repos.ProposalsSuffix)
	commitOnBranch(t, repo, open, "b.txt", "skills.b: Fixed b\n\nSmine-Change: skills/b")
	current := nightBranch(today(), repos.ProposalsSuffix)
	gitRun(t, repo, "branch", current, "main")

	settledSha := gitRun(t, repo, "rev-parse", settled)

	worktree.deleteSettledProposals(ctx, current)

	assert.False(t, worktree.branchExists(ctx, settled), "every change rejected: settled")
	assert.True(t, worktree.branchExists(ctx, open), "an open change keeps its branch")
	assert.True(t, worktree.branchExists(ctx, current), "the current night is never deleted")
	stashes, err := worktree.trash.List()
	require.NoError(t, err)
	require.Len(t, stashes, 1)
	diffName := strings.ReplaceAll(settled, "/", "_") + ".diff"
	assert.Equal(t, "branches/"+diffName, stashes[0].Entries[0].Rel)
	journal, err := worktree.trash.Tail(10)
	require.NoError(t, err)
	require.Len(t, journal, 1)
	assert.Equal(t, "nightly settled proposals", journal[0].Action)
	assert.Equal(t, trash.KindBranchDeleted, journal[0].Kind)
	assert.Equal(t, repo+"#"+settled, journal[0].Path)
	assert.Equal(t, "sha "+settledSha, journal[0].Reason)
	assert.Equal(t, stashes[0].Stamp+"/branches", journal[0].Stash)
}

func TestGroupWorktree_SyncMain(t *testing.T) {
	ctx := context.Background()

	t.Run("clean-merge", func(t *testing.T) {
		repo := newTestRepo(t)
		worktree := newTestGroup(t, repo, "demo", 0)
		_, err := worktree.create(ctx, noResolve)
		require.NoError(t, err)
		commitInWorktree(t, worktree, "chain.txt", 0)
		require.NoError(t, os.WriteFile(filepath.Join(repo, "main.txt"), []byte("main\n"), 0o644))
		gitRun(t, repo, "add", "main.txt")
		gitRun(t, repo, "commit", "-qm", "main moved")

		_, err = worktree.create(ctx, noResolve)

		require.NoError(t, err)
		assert.FileExists(t, filepath.Join(worktree.path, "main.txt"))
		assert.FileExists(t, filepath.Join(worktree.path, "chain.txt"))
	})

	t.Run("conflict-unresolved-discards", func(t *testing.T) {
		repo := newTestRepo(t)
		worktree := newTestGroup(t, repo, "demo", 0)
		first, err := worktree.create(ctx, noResolve)
		require.NoError(t, err)
		commitInWorktree(t, worktree, "README", 0)
		require.NoError(t, os.WriteFile(filepath.Join(repo, "README"), []byte("main side\n"), 0o644))
		gitRun(t, repo, "commit", "-qam", "main edits README")
		resolverCalls := 0
		resolve := func(context.Context) error {
			resolverCalls++
			return nil
		}

		_, err = worktree.create(ctx, resolve)

		require.Error(t, err)
		assert.Equal(t, 1, resolverCalls)
		assert.True(t, worktree.branchExists(ctx, first))
		assert.False(t, worktree.branchExists(ctx, first+"-2"))
	})
}

func TestGroupWorktree_Publish(t *testing.T) {
	ctx := context.Background()

	t.Run("no-change-no-commit", func(t *testing.T) {
		repo := newTestRepo(t)
		worktree := newTestGroup(t, repo, "demo", 0)
		branch, err := worktree.create(ctx, noResolve)
		require.NoError(t, err)

		require.NoError(t, worktree.publish(ctx, 0, ""))

		assert.Equal(t, gitRun(t, repo, "rev-parse", "main"), gitRun(t, repo, "rev-parse", branch))
		assert.NoDirExists(t, worktree.path)
	})

	t.Run("commit-body-handoff", func(t *testing.T) {
		repo := newTestRepo(t)
		worktree := newTestGroup(t, repo, "demo", 0)
		branch, err := worktree.create(ctx, noResolve)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(worktree.path, commitBodyFileName), []byte("disposition table\n"), 0o644))

		commitInWorktree(t, worktree, "out.txt", 0)

		assert.Equal(t, "disposition table", gitRun(t, repo, "log", "-1", "--format=%b", branch))
		files := gitRun(t, repo, "show", "--name-only", "--format=", branch)
		assert.NotContains(t, files, commitBodyFileName)
	})

	t.Run("failed-marker-subject", func(t *testing.T) {
		repo := newTestRepo(t)
		worktree := newTestGroup(t, repo, "demo", 0)
		branch, err := worktree.create(ctx, noResolve)
		require.NoError(t, err)

		commitInWorktree(t, worktree, "out.txt", 70)

		subject := gitRun(t, repo, "log", "-1", "--format=%s", branch)
		assert.True(t, strings.HasPrefix(subject, "sessions: [failed] Recorded nightly demo output"))
		assert.Contains(t, subject, "exit 70")
	})

	t.Run("stage-failed-marker-subject", func(t *testing.T) {
		repo := newTestRepo(t)
		worktree := newTestGroup(t, repo, "demo", 0)
		branch, err := worktree.create(ctx, noResolve)
		require.NoError(t, err)

		publishInWorktree(t, worktree, "out.txt", exitSoftware, "[stage-failed: style, apply]")

		subject := gitRun(t, repo, "log", "-1", "--format=%s", branch)
		assert.True(t, strings.HasPrefix(subject, "sessions: [stage-failed: style, apply] Recorded nightly demo output"))
		assert.NotContains(t, subject, failedMarker)
		assert.Contains(t, subject, "exit 70")
	})

	t.Run("strips-projection-before-staging", func(t *testing.T) {
		repo := newTestRepo(t)
		require.NoError(t, os.MkdirAll(filepath.Join(repo, "acdsl"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(repo, "acdsl", "registry.json"), []byte("{}\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main\n"), 0o644))
		gitRun(t, repo, "add", "-A")
		gitRun(t, repo, "commit", "-qm", "registry")
		worktree := newTestGroup(t, repo, "demo", 0)
		branch, err := worktree.create(ctx, noResolve)
		require.NoError(t, err)
		projected := "// " + acdsl.ProjectionMarker + " 1 rule(s) govern this file\n// - [ACDSL" + "-GOLANG-FMT-001] gofmt\n\npackage main\n"
		require.NoError(t, os.WriteFile(filepath.Join(worktree.path, "main.go"), []byte(projected), 0o644))

		commitInWorktree(t, worktree, "out.txt", 0)

		files := gitRun(t, repo, "show", "--name-only", "--format=", branch)
		assert.Contains(t, files, "out.txt")
		assert.NotContains(t, files, "main.go", "the stripped file is unchanged against its base")
		assert.NoDirExists(t, worktree.path)
	})

	t.Run("refused-commit-keeps-worktree-and-body", func(t *testing.T) {
		repo := newTestRepo(t)
		hook := filepath.Join(repo, ".git", "hooks", "pre-commit")
		require.NoError(t, os.WriteFile(hook, []byte("#!/bin/sh\nexit 1\n"), 0o755))
		worktree := newTestGroup(t, repo, "demo", 0)
		_, err := worktree.create(ctx, noResolve)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(worktree.path, commitBodyFileName), []byte("kept body\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(worktree.path, "out.txt"), []byte("out\n"), 0o644))

		err = worktree.publish(ctx, 0, "")

		require.Error(t, err)
		assert.FileExists(t, filepath.Join(worktree.path, "out.txt"))
		body, readErr := os.ReadFile(filepath.Join(worktree.path, commitBodyFileName))
		require.NoError(t, readErr)
		assert.Equal(t, "kept body\n", string(body))
	})

	t.Run("vanished-worktree-errors", func(t *testing.T) {
		repo := newTestRepo(t)
		worktree := newTestGroup(t, repo, "demo", 0)
		_, err := worktree.create(ctx, noResolve)
		require.NoError(t, err)
		require.NoError(t, os.RemoveAll(worktree.path))

		assert.Error(t, worktree.publish(ctx, 0, ""))
	})
}

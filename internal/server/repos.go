package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/gitwt"
	"github.com/kevinhorst/smine/internal/peek"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/server/respond"
	"github.com/kevinhorst/smine/internal/shell"
	"github.com/kevinhorst/smine/internal/trash"
)

const (
	pageRepos          = "repos"
	tmplCommitList     = "_commit_list.html"
	tmplOpResult       = "_op_result.html"
	tmplRepoDetail     = "repo_detail.html"
	tmplReposIndex     = "repos_index.html"
	tmplWorktreeStatus = "_worktree_status.html"

	// registryLockKey serializes registry add/delete; reserved like _jetbrains.
	registryLockKey = "_registry"

	// chooseFolderLockKey keeps a second native dialog from piling onto an open one.
	chooseFolderLockKey = "_choose-folder"

	// jetbrainsLockKey never races a same-named repo's routes: the prune route is literal.
	jetbrainsLockKey = "_jetbrains"

	// peekSessionsTimeout: a cold peek blocks session_list for its ~1 min initial scan; the page must not.
	peekSessionsTimeout = 5 * time.Second

	tmplRepoPath = "_repo_path.html"

	// statusKindAll is a cache scope and a chip, never a binary argument.
	statusKindAll = "all"

	// statusKindDefault is the kind a repo page opens on.
	statusKindDefault = "claude"

	// allWorktreesConfirm is the confirm every all-scope request asks (D7).
	allWorktreesConfirm = "Load all worktrees of %s? This scans every agent branch and worktree at once and can overload this machine."
)

type commitListPage struct {
	Branch  string
	Class   string
	Commits []repos.Commit
	Page    string
	Repo    string
	Title   string
}

type opResult struct {
	Duration string
	Error    string
	Output   string
	Page     string
	Subject  string
}

type stepTiming struct {
	Ms   int64
	Step string
}

// formatTimings renders "status=1840ms checkout=95ms sessions=210ms".
func formatTimings(timings []stepTiming) string {
	parts := make([]string, 0, len(timings))
	for _, t := range timings {
		parts = append(parts, fmt.Sprintf("%s=%dms", t.Step, t.Ms))
	}
	return strings.Join(parts, " ")
}

// kindFilters keeps the loaded kind's chip even when the repo holds none of it.
func kindFilters(active, confirm string, kinds []string) []worktreeFilter {
	values := slices.Clone(kinds)
	isHeld := slices.Contains(values, active)
	if !isHeld && active != statusKindAll {
		values = append(values, active)
		slices.Sort(values)
	}
	all := worktreeFilter{Confirm: confirm, IsActive: active == statusKindAll, Label: statusKindAll, Value: statusKindAll}
	filters := []worktreeFilter{all}
	for _, kind := range values {
		filter := worktreeFilter{IsActive: kind == active, Label: kind, Value: kind}
		filters = append(filters, filter)
	}
	return filters
}

type repoDetailPage struct {
	Kind  string
	Page  string
	Repo  repos.Repo
	Title string
}

// worktreeFilter base chips filter client-side; the detail page's JS owns their active state.
type worktreeFilter struct {
	Confirm  string
	IsActive bool
	Label    string
	Value    string
}

type worktreeStatusPage struct {
	BaseFilters       []worktreeFilter
	Cached            bool
	Checkout          *repos.CheckoutStatus
	CheckoutErr       string
	Confirm           string // the all-scope confirm on Reload and the repo-op refresh; empty for a kind scope
	IsAgentwtOff      bool
	Kind              string
	KindFilters       []worktreeFilter
	PeekSessions      map[string]peek.Session // keyed by session id, for the dashboard link and last-active time
	Repo              repos.Repo
	ScannedAt         time.Time
	SessionsErr       string
	SessionsErrDetail string
	StatusErr         string
	Statuses          []repos.WorktreeStatus
	Timings           []stepTiming
}

type contextBadge struct {
	Class string
	Label string
	Title string
}

func newAcdslBadge(presence repos.ContextPresence) contextBadge {
	if presence.Acdsl {
		return contextBadge{Class: "badge-ok", Label: "on", Title: "acdsl gate slice active (deploy.acdsl / registry present)"}
	}
	return contextBadge{Class: "badge-dim", Label: "off", Title: "no acdsl slice — sync with sync acdsl to activate"}
}

func newContextBadge(presence repos.ContextPresence, coverage repos.ContextCoverage, coverageErr error) contextBadge {
	switch presence.State {
	case repos.ContextSource:
		return contextBadge{Class: "badge-ok", Label: "source", Title: "authors the context system (context/aspects.json)"}
	case repos.ContextDeployed:
		return deployedContextBadge(presence, coverage, coverageErr)
	case repos.ContextNone:
		return contextBadge{Class: "badge-error", Label: "✗", Title: "no context/aspects.json and no context build for this repo"}
	}
	return contextBadge{Class: "badge-error", Label: "✗", Title: "unknown context state " + presence.State}
}

func deployedContextBadge(presence repos.ContextPresence, coverage repos.ContextCoverage, coverageErr error) contextBadge {
	prefix := "patch:"
	if presence.IsWritten {
		prefix = "synced:"
	}
	if coverageErr != nil {
		badge := contextBadge{Class: "badge-action", Label: prefix + presence.ContextDir}
		badge.Title = "coverage check failed: " + coverageErr.Error()
		return badge
	}
	if isContextSynced(presence, coverage, coverageErr) {
		badge := contextBadge{Class: "badge-ok", Label: prefix + presence.ContextDir}
		badge.Title = fmt.Sprintf("all %d entries reaching this repo are in the build", coverage.Expected)
		if !presence.IsWritten {
			badge.Title += " — overlay built, not written to the repo"
		}
		return badge
	}
	shown := coverage.Missing
	if len(shown) > 5 {
		shown = shown[:5]
	}
	badge := contextBadge{Class: "badge-action"}
	badge.Label = fmt.Sprintf("partial %d/%d", coverage.Expected-len(coverage.Missing), coverage.Expected)
	badge.Title = "re-sync — missing: " + strings.Join(shown, ", ")
	if len(coverage.Missing) > len(shown) {
		badge.Title += fmt.Sprintf(" (+%d more)", len(coverage.Missing)-len(shown))
	}
	return badge
}

// isContextSynced is the Repos page's "synced" badge rule, which the Overview Repos tile counts too.
func isContextSynced(presence repos.ContextPresence, coverage repos.ContextCoverage, coverageErr error) bool {
	if presence.State != repos.ContextDeployed {
		return false
	}
	return coverageErr == nil && len(coverage.Missing) == 0
}

type repoRow struct {
	Acdsl     contextBadge
	Context   contextBadge
	Repo      repos.Repo
	Worktrees []repos.WorktreeKindCount
}

type reposIndexPage struct {
	AddPath   folderPick
	Page      string
	RepoNames []string
	Rows      []repoRow
	Title     string
}

// rootCause shows the actionable innermost error; the full chain stays in the tooltip.
func rootCause(err error) error {
	for {
		next := errors.Unwrap(err)
		if next == nil {
			return err
		}
		err = next
	}
}

func (s *Server) findRepo(w http.ResponseWriter, r *http.Request) *repos.Repo {
	repo, ok := s.repoRegistry.Find(r.PathValue("name"))
	if !ok {
		http.NotFound(w, r)
		return nil
	}
	return repo
}

// runRepoOp renders failures inside the result block: the script output is the UI, not an HTTP error.
func (s *Server) runRepoOp(lockKey, trigger string, op func(ctx context.Context) (string, error), w http.ResponseWriter, r *http.Request) {
	if !s.repoLocks.TryAcquire(lockKey) {
		respond.WithConflict("an operation is already running", w)
		return
	}
	defer s.repoLocks.Release(lockKey)

	opStart := time.Now()
	output, err := op(r.Context())
	elapsed := time.Since(opStart)
	log.Printf("timing: op subject=%s ms=%d err=%v", lockKey, elapsed.Milliseconds(), err != nil)
	result := opResult{Duration: elapsed.Round(100 * time.Millisecond).String(), Output: output, Page: pageRepos, Subject: lockKey}
	if err != nil {
		result.Error = err.Error()
	}

	// Fire even on error: partial effects may have changed worktree state.
	w.Header().Set("HX-Trigger", trigger)
	s.renderFragment(w, tmplOpResult, result)
}

// cachedStatuses treats a fingerprint error as changed: degrade to a fresh scan, never to stale rows.
func (s *Server) cachedStatuses(ctx context.Context, kind string, refresh bool, repo *repos.Repo) (statusEntry, bool, error) {
	key := statusKey{Kind: kind, Repo: repo.Name}
	fingerprint, fingerprintErr := repos.Fingerprint(ctx, repo.Path)
	entry, cached := s.statusCache.Get(key)
	hit := cached && !refresh && fingerprintErr == nil && fingerprint == entry.Fingerprint
	if hit {
		return entry, true, nil
	}
	binaryKind := kind
	if kind == statusKindAll {
		binaryKind = ""
	}
	scan, err := repos.Status(ctx, s.agentwt, binaryKind, repo.Path)
	if err != nil {
		return statusEntry{}, false, err
	}
	return s.statusCache.Store(key, fingerprint, scan), false, nil
}

// handleRepoCommits reads the cached document, so a drill-down spawns no subprocess.
func (s *Server) handleRepoCommits(w http.ResponseWriter, r *http.Request) {
	repo := s.findRepo(w, r)
	if repo == nil {
		return
	}

	// class: fixed operation set only (concept allowlist)
	class := r.PathValue("class")
	switch class {
	case "ahead", "behind", "unpicked":
	default:
		http.NotFound(w, r)
		return
	}

	branch := r.PathValue("branch")
	if err := repos.ValidateBranch(r.Context(), branch, repo.Path); err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}
	// The branch's own kind scope: a drill-down never starts an all scan (D4).
	entry, _, err := s.cachedStatuses(r.Context(), repos.AgentBranchKind(branch), false, repo)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	index := slices.IndexFunc(entry.Statuses, func(status repos.WorktreeStatus) bool {
		return status.Branch == branch
	})
	if index < 0 {
		http.NotFound(w, r)
		return
	}

	data := commitListPage{
		Branch:  branch,
		Class:   class,
		Commits: repos.Commits(class, &entry.Statuses[index]),
		Page:    pageRepos,
		Repo:    repo.Name,
		Title:   "Commits — " + branch,
	}
	s.renderFragment(w, tmplCommitList, data)
}

func (s *Server) handleRepoDetail(w http.ResponseWriter, r *http.Request) {
	repo := s.findRepo(w, r)
	if repo == nil {
		return
	}

	// The shell never opens on the all scope: a full scan starts only from a confirmed click.
	kind := r.URL.Query().Get("kind")
	if kind == "" || kind == statusKindAll {
		kind = statusKindDefault
	}
	data := repoDetailPage{
		Kind:  kind,
		Page:  pageRepos,
		Repo:  *repo,
		Title: "Repo — " + repo.Name,
	}
	s.renderFragment(w, tmplRepoDetail, data)
}

// handleRepoWorktreeStatus holds all subprocess and peek cost so the page shell paints instantly.
func (s *Server) handleRepoWorktreeStatus(w http.ResponseWriter, r *http.Request) {
	repo := s.findRepo(w, r)
	if repo == nil {
		return
	}

	kind := r.URL.Query().Get("kind")
	if kind == "" {
		kind = statusKindDefault
	}
	confirm := fmt.Sprintf(allWorktreesConfirm, repo.Name)
	data := worktreeStatusPage{Kind: kind, Repo: *repo}
	if kind == statusKindAll {
		data.Confirm = confirm
	}
	refresh := r.URL.Query().Get("refresh") == "1"
	// agentwt off is a choice: an explanation, never the red scan error (concept D9).
	if !s.records.has(s.machine, components.Agentwt) {
		data.IsAgentwtOff = true
		s.renderFragment(w, tmplWorktreeStatus, data)
		return
	}

	// Each goroutine writes only its own locals before wg.Wait(), so errors stay per-section.
	var wg sync.WaitGroup
	var checkout *repos.CheckoutStatus
	var checkoutErr error
	var checkoutMs int64
	var peekSessions map[string]peek.Session
	var sessionsErr error
	var sessionsMs int64
	wg.Add(2)
	go func() {
		defer wg.Done()
		start := time.Now()
		checkout, checkoutErr = repos.Checkout(r.Context(), repo.Path)
		checkoutMs = time.Since(start).Milliseconds()
	}()
	go func() {
		defer wg.Done()
		start := time.Now()
		peekCtx, cancel := context.WithTimeout(r.Context(), peekSessionsTimeout)
		defer cancel()
		peekSessions, sessionsErr = s.peekClient.SessionsById(peekCtx)
		sessionsMs = time.Since(start).Milliseconds()
	}()

	// A plain load serves the last scan only while the fingerprint still matches.
	start := time.Now()
	entry, cached, err := s.cachedStatuses(r.Context(), kind, refresh, repo)
	statuses := entry.Statuses
	data.Cached = cached
	statusMs := time.Since(start).Milliseconds()
	wg.Wait()
	data.ScannedAt = entry.ScannedAt

	data.Timings = []stepTiming{
		{Ms: statusMs, Step: "status"},
		{Ms: entry.Timings.AnalysisMs, Step: "analysis"},
		{Ms: entry.Timings.WorktreesMs, Step: "worktrees"},
		{Ms: entry.Timings.OrderMs, Step: "order"},
		{Ms: checkoutMs, Step: "checkout"},
		{Ms: sessionsMs, Step: "sessions"},
	}
	if err != nil {
		data.StatusErr = err.Error()
	}
	if checkoutErr != nil {
		data.CheckoutErr = checkoutErr.Error()
	}
	data.Checkout = checkout
	// peek down → degraded session column; the fragment still renders, and peek off shows no banner.
	isPeekDown := sessionsErr != nil && s.records.has(s.machine, components.Peek)
	if isPeekDown {
		data.SessionsErr = fmt.Sprintf("%s at %s", rootCause(sessionsErr), s.peekClient.Endpoint())
		data.SessionsErrDetail = sessionsErr.Error()
	}
	data.PeekSessions = peekSessions

	// "unknown" is a sentinel, not a base.
	values := make(map[string]bool)
	for _, status := range statuses {
		if status.From != "unknown" {
			values[status.From] = true
		}
	}
	data.Statuses = statuses
	data.KindFilters = kindFilters(kind, confirm, entry.Kinds)
	for _, base := range append([]string{""}, slices.Sorted(maps.Keys(values))...) {
		label := base
		if label == "" {
			label = "all"
		}
		chip := worktreeFilter{Label: label, Value: base}
		data.BaseFilters = append(data.BaseFilters, chip)
	}

	log.Printf("timing: repo=%s kind=%s %s rows=%d cached=%t", repo.Name, kind, formatTimings(data.Timings), len(statuses), data.Cached)
	s.renderFragment(w, tmplWorktreeStatus, data)
}

// archiveSelected salvages a worktree before a forced removal and archives its branch before a deletion, into one trash stamp; returns the stash path.
func (s *Server) archiveSelected(ctx context.Context, branch, repoPath string, isForce, isDeleteBranch bool) (string, error) {
	if !isForce && !isDeleteBranch {
		return "", nil
	}

	worktrees, err := gitwt.List(ctx, repoPath)
	if err != nil {
		return "", fmt.Errorf("Server.archiveSelected: %w", err)
	}

	base, err := shell.Run(ctx, repoPath, "git", "merge-base", "HEAD", branch)
	if err != nil {
		return "", fmt.Errorf("Server.archiveSelected: %s: %w", branch, err)
	}

	base = strings.TrimSpace(base)
	index := slices.IndexFunc(worktrees, func(worktree gitwt.Worktree) bool { return worktree.Branch == branch })
	isCheckedOut := index >= 0
	isSalvageDue := isForce && isCheckedOut
	if !isSalvageDue && !isDeleteBranch {
		return "", nil
	}

	dest, err := s.trash.StampDir()
	if err != nil {
		return "", fmt.Errorf("Server.archiveSelected: %w", err)
	}

	if isSalvageDue {
		request := &gitwt.SalvageRequest{
			Base:          base,
			Dest:          filepath.Join(dest, "worktree"),
			IsArchiveOnly: true,
			Repo:          repoPath,
			Worktree:      worktrees[index].Path,
		}
		if _, err := gitwt.Salvage(ctx, request); err != nil {
			return "", fmt.Errorf("Server.archiveSelected: %w", err)
		}
	}

	if isDeleteBranch {
		sha, err := gitwt.ArchiveBranch(ctx, base, branch, filepath.Join(dest, "branches"), repoPath)
		if err != nil {
			return "", fmt.Errorf("Server.archiveSelected: %w", err)
		}

		entry := &trash.Entry{
			Action: "repos remove",
			Kind:   trash.KindBranchDeleted,
			Path:   repoPath + "#" + branch,
			Reason: "sha " + sha,
			Stash:  s.trash.RelOf(dest) + "/branches",
		}
		if err := s.trash.Note(entry); err != nil {
			return "", fmt.Errorf("Server.archiveSelected: %w", err)
		}
	}
	return dest, nil
}

// handleRepoRemoveSelected fires repo-op only on an error or skip: the optimistically hidden rows are otherwise the truth.
func (s *Server) handleRepoRemoveSelected(w http.ResponseWriter, r *http.Request) {
	repo := s.findRepo(w, r)
	if repo == nil {
		return
	}

	if err := r.ParseForm(); err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}
	branches := r.Form["branch"]
	if len(branches) == 0 {
		respond.WithBadRequest("no branches selected", w)
		return
	}

	if !s.repoLocks.TryAcquire(repo.Name) {
		respond.WithConflict("an operation is already running", w)
		return
	}
	defer s.repoLocks.Release(repo.Name)

	force := r.FormValue("force") == "on"
	deleteBranch := r.FormValue("delete-branch") == "on"
	opStart := time.Now()
	var outputs []string
	var opErr error
	skipped := false
	for _, branch := range branches {
		stash, err := s.archiveSelected(r.Context(), branch, repo.Path, force, deleteBranch)
		if err != nil {
			opErr = fmt.Errorf("%s: %w", branch, err)
			break
		}

		if stash != "" {
			outputs = append(outputs, "archived "+branch+" -> "+stash)
		}
		output, err := repos.Remove(r.Context(), branch, force, deleteBranch, repo.Path, s.agentwt)
		if output != nil {
			outputs = append(outputs, output.Text)
			skipped = skipped || output.HasSkips()
		}
		if err != nil {
			opErr = fmt.Errorf("%s: %w", branch, err)
			break
		}
	}

	elapsed := time.Since(opStart)
	log.Printf("timing: op subject=%s ms=%d err=%v", repo.Name, elapsed.Milliseconds(), opErr != nil)
	result := opResult{
		Duration: elapsed.Round(100 * time.Millisecond).String(),
		Output:   strings.Join(outputs, "\n"),
		Page:     pageRepos,
		Subject:  repo.Name,
	}
	if opErr != nil {
		result.Error = opErr.Error()
	}
	if opErr != nil || skipped {
		w.Header().Set("HX-Trigger", "repo-op")
	} else {
		// Clean run: no re-scan fires, so drop the rows from the cache for plain loads to agree.
		s.statusCache.DropBranches(repo.Name, branches)
	}
	s.renderFragment(w, tmplOpResult, result)
}

func (s *Server) handleReposIndex(w http.ResponseWriter, r *http.Request) {
	title := "Repos"
	if !s.presentation.isDeveloperAudience() {
		title = translate(s.presentation.language(), "Projects")
	}
	data := reposIndexPage{Page: pageRepos, Title: title}
	sourceIndexPath := filepath.Join(s.contextDir, repos.ContextFileName)
	for _, repo := range s.repoRegistry.Repos() {
		data.RepoNames = append(data.RepoNames, repo.Name)
		worktrees := repos.WorktreeCounts(r.Context(), repo.Path)
		presence := repos.DetectContext(repo.Path, contextdocs.DeployRecordPath(s.stateDir, repo.Name))
		coverage, coverageErr := repos.DetectContextCoverage(sourceIndexPath, repo.Name, presence)
		row := repoRow{
			Acdsl:     newAcdslBadge(presence),
			Context:   newContextBadge(presence, coverage, coverageErr),
			Repo:      repo,
			Worktrees: worktrees,
		}
		data.Rows = append(data.Rows, row)
	}
	s.renderFragment(w, tmplReposIndex, data)
}

func (s *Server) handleReposAdd(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSpace(r.FormValue("path"))
	if path == "" {
		respond.WithBadRequest("path must not be empty", w)
		return
	}

	// Clean strips the picker's trailing slash so Base is the folder name.
	path = filepath.Clean(path)
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = filepath.Base(path)
	}
	label := strings.TrimSpace(r.FormValue("label"))
	repo := repos.Repo{Label: label, Name: name, Path: path}
	// Validate first: git init must never fire for a repo the registry would reject.
	if err := repo.Validate(); err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}
	// Path — a missing path or a file would register a dead project the roster can never mine
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		respond.WithBadRequest("path is not an existing folder: "+path, w)
		return
	}
	op := func(ctx context.Context) (string, error) {
		// Fail before git init touches the folder; Add re-checks under its own lock and stays the authority.
		if _, isRegistered := s.repoRegistry.Find(repo.Name); isRegistered {
			return "", fmt.Errorf("handleReposAdd: Duplicate repo name %s", repo.Name)
		}
		// A folder without a repo gets a local one (no remote, C19).
		if _, gitErr := os.Stat(filepath.Join(path, ".git")); gitErr != nil {
			if output, err := shell.Run(ctx, path, "git", "init", "-b", "main"); err != nil {
				return "", fmt.Errorf("handleReposAdd: git init failed: %w\n%s", err, output)
			}
		}
		if err := s.repoRegistry.Add(repo); err != nil {
			return "", err
		}
		return fmt.Sprintf("added %s (%s)", repo.Name, repo.Path), nil
	}
	s.runRepoOp(registryLockKey, "repo-op", op, w, r)
}

func (s *Server) handleReposDelete(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	if name == "" {
		respond.WithBadRequest("name must not be empty", w)
		return
	}

	op := func(ctx context.Context) (string, error) {
		if err := s.repoRegistry.Remove(name); err != nil {
			return "", err
		}
		s.statusCache.Delete(name)
		return "removed " + name, nil
	}
	s.runRepoOp(registryLockKey, "repo-op", op, w, r)
}

func (s *Server) handleReposChooseFolder(w http.ResponseWriter, r *http.Request) {
	if !s.repoLocks.TryAcquire(chooseFolderLockKey) {
		respond.WithConflict("a folder dialog is already open", w)
		return
	}
	defer s.repoLocks.Release(chooseFolderLockKey)

	data := folderPick{Value: r.FormValue("path")}
	path, err := contextdocs.ChooseFolder(r.Context(), "Add repo: choose the repository folder")
	switch {
	case errors.Is(err, contextdocs.ErrCanceled):
		// dismissed: keep the typed value, no error
	case err != nil:
		data.Error = err.Error()
	default:
		data.Value = path
	}
	s.renderFragment(w, tmplRepoPath, data)
}

func (s *Server) handleReposPruneJetbrains(w http.ResponseWriter, r *http.Request) {
	dryRun := r.FormValue("dry-run") == "on"
	force := r.FormValue("force") == "on"
	op := func(ctx context.Context) (string, error) {
		return repos.PruneJetbrains(ctx, dryRun, force)
	}
	s.runRepoOp(jetbrainsLockKey, "repo-op", op, w, r)
}

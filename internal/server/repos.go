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

	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/peek"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/server/respond"
	"github.com/kevinhorst/smine/internal/shell"
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

	// chooseFolderLockKey keeps a second native dialog from piling onto an
	// open one.
	chooseFolderLockKey = "_choose-folder"

	tmplRepoPath = "_repo_path.html"
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

// stepTiming is one measured step of the status render, shown in the
// fragment's timing line and logged per request (DR3).
type stepTiming struct {
	Ms   int64
	Step string
}

// formatTimings renders "status=1840ms checkout=95ms sessions=210ms" for the
// per-request timing log line.
func formatTimings(timings []stepTiming) string {
	parts := make([]string, 0, len(timings))
	for _, t := range timings {
		parts = append(parts, fmt.Sprintf("%s=%dms", t.Step, t.Ms))
	}
	return strings.Join(parts, " ")
}

type repoDetailPage struct {
	Page  string
	Repo  repos.Repo
	Title string
}

// worktreeFilter is one chip of the worktree-table filters (kind or base);
// filtering is client-side — the chip carries its group value as a data
// attribute and the detail page's JS owns the active state.
type worktreeFilter struct {
	Label string
	Value string
}

// worktreeStatusPage feeds the #worktree-status fragment — everything the
// status table needs, decoupled from the instant page shell (D3).
type worktreeStatusPage struct {
	BaseFilters       []worktreeFilter
	Cached            bool
	Checkout          *repos.CheckoutStatus
	CheckoutErr       string
	KindFilters       []worktreeFilter
	Repo              repos.Repo
	ScannedAt         time.Time
	Sessions          map[string]peek.Session
	SessionsErr       string
	SessionsErrDetail string
	StatusErr         string
	Statuses          []repos.WorktreeStatus
	Timings           []stepTiming
}

// contextBadge is the index-cell view of the context-system state: one text
// badge, the detail in the tooltip.
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
		return contextBadge{Class: "badge-ok", Label: "source", Title: "authors the context system (context/context.json)"}
	case repos.ContextDeployed:
		return deployedContextBadge(presence, coverage, coverageErr)
	case repos.ContextNone:
		return contextBadge{Class: "badge-error", Label: "✗", Title: "no context.json under context/ or docs/"}
	}
	return contextBadge{Class: "badge-error", Label: "✗", Title: "unknown context state " + presence.State}
}

func deployedContextBadge(presence repos.ContextPresence, coverage repos.ContextCoverage, coverageErr error) contextBadge {
	if coverageErr != nil {
		badge := contextBadge{Class: "badge-action", Label: "deployed:" + presence.ContextDir}
		badge.Title = "coverage check failed: " + coverageErr.Error()
		return badge
	}
	if len(coverage.Missing) == 0 {
		badge := contextBadge{Class: "badge-ok", Label: "synced:" + presence.ContextDir}
		badge.Title = fmt.Sprintf("all %d entries reaching this repo are deployed", coverage.Expected)
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

// rootCause unwraps to the innermost error — the actionable fact of a wrapped
// chain (e.g. "connection refused"); the full chain stays in the tooltip.
// worktreeSessions attributes a session to each worktree row: the newest
// session on the worktree's checked-out branch wins over the newest session
// whose cwd is the directory — a pool-recycled dir otherwise shows its
// previous occupant. The cwd fallback keeps detached-HEAD worktrees covered.
func worktreeSessions(statuses []repos.WorktreeStatus, index *peek.SessionIndex) map[string]peek.Session {
	if index == nil {
		return nil
	}

	sessions := make(map[string]peek.Session)
	for _, status := range statuses {
		if status.Worktree == "" {
			continue
		}
		if session, ok := index.ByBranch[status.Branch]; ok {
			sessions[status.Worktree] = session
			continue
		}
		if session, ok := index.ByCwd[status.Worktree]; ok {
			sessions[status.Worktree] = session
		}
	}
	return sessions
}

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

// runRepoOp wraps every mutation: lock (D20/H1), operation, captured-output
// fragment. Operation failures render inside the result block — the script
// output is the UI, not an HTTP error.
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

	// Every op may have mutated worktree state (also on error — partial
	// effects); the detail page's #worktree-status listens for this event
	// and re-pulls the table so removed worktrees never linger as rows.
	w.Header().Set("HX-Trigger", trigger)
	s.renderFragment(w, tmplOpResult, result)
}

func (s *Server) handleRepoCherryPick(w http.ResponseWriter, r *http.Request) {
	repo := s.findRepo(w, r)
	if repo == nil {
		return
	}

	commit := r.FormValue("commit")
	op := func(ctx context.Context) (string, error) {
		return repos.CherryPick(ctx, commit, repo.Path, s.worktreeScripts)
	}
	s.runRepoOp(repo.Name, "repo-op", op, w, r)
}

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
	commits, err := repos.Commits(r.Context(), branch, class, repo.Path, s.worktreeScripts)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	data := commitListPage{
		Branch:  branch,
		Class:   class,
		Commits: commits,
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

	data := repoDetailPage{
		Page:  pageRepos,
		Repo:  *repo,
		Title: "Repo — " + repo.Name,
	}
	s.renderFragment(w, tmplRepoDetail, data)
}

// handleRepoWorktreeStatus renders the #worktree-status fragment — all
// subprocess and peek cost lives here so the page shell paints instantly and
// load, filter, and repo-op refreshes all swap this one element (D3).
func (s *Server) handleRepoWorktreeStatus(w http.ResponseWriter, r *http.Request) {
	repo := s.findRepo(w, r)
	if repo == nil {
		return
	}

	data := worktreeStatusPage{Repo: *repo}
	refresh := r.URL.Query().Get("refresh") == "1"

	// Checkout and peek run concurrently with the dominant status script —
	// sum becomes max, and a peek timeout no longer stacks on top (D7). Each
	// goroutine writes only its own locals before wg.Wait(); degradation
	// stays per-section: every error lands in its own page field (like D3).
	var wg sync.WaitGroup
	var checkout *repos.CheckoutStatus
	var checkoutErr error
	var checkoutMs int64
	var sessionIndex *peek.SessionIndex
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
		sessionIndex, sessionsErr = s.peekClient.SessionIndex(r.Context())
		sessionsMs = time.Since(start).Milliseconds()
	}()

	// A plain load serves the last scan only while the fingerprint (branch
	// tips + worktree set) still matches it; any structural change — and
	// ?refresh=1 (Reload button, repo-op re-pull) — re-scans, so mutations
	// and new worktrees never linger or hide. A fingerprint error counts as
	// changed: degrade to a fresh scan, never to stale rows.
	var statuses []repos.WorktreeStatus
	var err error
	start := time.Now()
	fingerprint, fingerprintErr := repos.Fingerprint(r.Context(), repo.Path)
	entry, cached := s.statusCache.Get(repo.Name)
	data.Cached = cached && !refresh && fingerprintErr == nil && fingerprint == entry.Fingerprint
	if data.Cached {
		statuses = entry.Statuses
	} else {
		statuses, err = repos.Status(r.Context(), repo.Path, s.worktreeScripts)
		if err == nil {
			entry = s.statusCache.Store(repo.Name, fingerprint, statuses)
		}
	}
	statusMs := time.Since(start).Milliseconds()
	wg.Wait()
	data.ScannedAt = entry.ScannedAt

	data.Timings = []stepTiming{
		{Ms: statusMs, Step: "status"},
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
	// peek down → degraded session column, the fragment still renders (D3);
	// banner label carries the root cause + endpoint, full chain in the tooltip.
	if sessionsErr != nil {
		data.SessionsErr = fmt.Sprintf("%s at %s", rootCause(sessionsErr), s.peekClient.Endpoint())
		data.SessionsErrDetail = sessionsErr.Error()
	}
	data.Sessions = worktreeSessions(statuses, sessionIndex)

	// Chips are the distinct base branches from the full list ("unknown" is a
	// sentinel, not a base). Filtering itself is client-side — every row is
	// rendered with data attributes and the detail page's JS owns visibility.
	values := make(map[string]bool)
	for _, status := range statuses {
		if status.From != "unknown" {
			values[status.From] = true
		}
	}
	data.Statuses = statuses
	// Kind chips separate claude/* session worktrees from claude-routines/*
	// routine lineages.
	for _, kind := range []string{"", "claude", "claude-routines"} {
		label := kind
		if label == "" {
			label = "all"
		}
		chip := worktreeFilter{Label: label, Value: kind}
		data.KindFilters = append(data.KindFilters, chip)
	}
	for _, base := range append([]string{""}, slices.Sorted(maps.Keys(values))...) {
		label := base
		if label == "" {
			label = "all"
		}
		chip := worktreeFilter{Label: label, Value: base}
		data.BaseFilters = append(data.BaseFilters, chip)
	}

	log.Printf("timing: repo=%s %s rows=%d cached=%t", repo.Name, formatTimings(data.Timings), len(statuses), data.Cached)
	s.renderFragment(w, tmplWorktreeStatus, data)
}

func (s *Server) handleRepoMerge(w http.ResponseWriter, r *http.Request) {
	repo := s.findRepo(w, r)
	if repo == nil {
		return
	}

	branch := r.PathValue("branch")
	op := func(ctx context.Context) (string, error) {
		return repos.Merge(ctx, branch, repo.Path, s.worktreeScripts)
	}
	s.runRepoOp(repo.Name, "repo-op", op, w, r)
}

// removalSkipMarkers are the remove script's skip-line prefixes — the script
// exits 0 on skips, so output is the only signal that rows were kept (D6).
var removalSkipMarkers = []string{"skipped: ", "skipped branch: ", "no worktree checked out"}

// removalNeedsRefresh reports whether a remove run left rows the optimistic
// UI hid — any error or skip means the table must re-sync from the server.
func removalNeedsRefresh(output string) bool {
	for line := range strings.Lines(output) {
		for _, marker := range removalSkipMarkers {
			if strings.HasPrefix(line, marker) {
				return true
			}
		}
	}
	return false
}

// handleRepoRemoveSelected removes every selected worktree in one op — the
// checkbox selection posts its branches as form values; the shared repo lock
// and one op-result cover the whole batch, stopping at the first failure.
// The UI hides the selected rows optimistically on submit; a clean run sends
// no refresh trigger (the hidden rows are the truth), while any error or
// script-reported skip fires repo-op so the fragment re-sync restores the
// actual state incl. partial effects (D4/D5/D6).
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
	for _, branch := range branches {
		output, err := repos.Remove(r.Context(), branch, force, deleteBranch, repo.Path, s.worktreeScripts)
		outputs = append(outputs, output)
		if err != nil {
			opErr = fmt.Errorf("%s: %w", branch, err)
			break
		}
	}

	elapsed := time.Since(opStart)
	output := strings.Join(outputs, "\n")
	log.Printf("timing: op subject=%s ms=%d err=%v", repo.Name, elapsed.Milliseconds(), opErr != nil)
	result := opResult{
		Duration: elapsed.Round(100 * time.Millisecond).String(),
		Output:   output,
		Page:     pageRepos,
		Subject:  repo.Name,
	}
	if opErr != nil {
		result.Error = opErr.Error()
	}
	if opErr != nil || removalNeedsRefresh(output) {
		w.Header().Set("HX-Trigger", "repo-op")
	} else {
		// Clean run: the optimistic UI keeps the rows hidden and no re-scan
		// fires — drop them from the cache so plain loads agree.
		s.statusCache.DropBranches(repo.Name, branches)
	}
	s.renderFragment(w, tmplOpResult, result)
}

func (s *Server) handleRepoSync(w http.ResponseWriter, r *http.Request) {
	repo := s.findRepo(w, r)
	if repo == nil {
		return
	}

	branch := r.PathValue("branch")
	op := func(ctx context.Context) (string, error) {
		return repos.Sync(ctx, branch, repo.Path, s.worktreeScripts)
	}
	s.runRepoOp(repo.Name, "repo-op", op, w, r)
}

func (s *Server) handleReposIndex(w http.ResponseWriter, r *http.Request) {
	title := "Repos"
	if !s.presentation.isDeveloperAudience() {
		title = translate(s.presentation.language(), "Projects")
	}
	data := reposIndexPage{Page: pageRepos, Title: title}
	branchPrefixes := []string{"claude/", "claude-routines/"}
	sourceIndexPath := filepath.Join(s.contextDir, repos.ContextFileName)
	for _, repo := range s.repoRegistry.Repos() {
		data.RepoNames = append(data.RepoNames, repo.Name)
		worktrees := repos.WorktreeCounts(r.Context(), repo.Path, branchPrefixes)
		presence := repos.DetectContext(repo.Path)
		coverage, coverageErr := repos.DetectContextCoverage(sourceIndexPath, repo.Name, repo.Path, presence)
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
	// Casual installs always grant: the mining roster derives from
	// additionalDirectories, so an unmined project must not be registrable (C6).
	grantAccess := r.FormValue("additional-dir") == "on" || !s.presentation.isDeveloperAudience()
	label := strings.TrimSpace(r.FormValue("label"))
	repo := repos.Repo{Label: label, Name: name, Path: path}
	// Validate before the op: its git init must never fire for a repo the
	// registry would reject anyway (reserved or malformed name).
	if err := repo.Validate(); err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}
	op := func(ctx context.Context) (string, error) {
		// A project the roster can't mine is a dead registration — an existing
		// folder without a repo gets a local one (no remote, C19); a missing
		// path keeps the registry's own error behavior.
		if info, statErr := os.Stat(path); statErr == nil && info.IsDir() {
			if _, gitErr := os.Stat(filepath.Join(path, ".git")); gitErr != nil {
				if output, err := shell.Run(ctx, path, "git", "init", "-b", "main"); err != nil {
					return "", fmt.Errorf("handleReposAdd: git init failed: %w\n%s", err, output)
				}
			}
		}
		if err := s.repoRegistry.Add(repo); err != nil {
			return "", err
		}
		if !grantAccess {
			return fmt.Sprintf("added %s (%s)", repo.Name, repo.Path), nil
		}
		if err := s.appendClaudeItem("permissions.additionalDirectories", repo.Path); err != nil {
			// Roll back the registry Add so add-with-grant is transactional:
			// otherwise the repo is left registered with no grant and a retry
			// errors "already exists" from Add, stranding it permanently.
			if rollbackErr := s.repoRegistry.Remove(repo.Name); rollbackErr != nil {
				return "", fmt.Errorf("handleReposAdd: additionalDirectories grant failed: %w; rollback of repo add also failed: %v", err, rollbackErr)
			}
			return "", fmt.Errorf("handleReposAdd: additionalDirectories grant failed, repo add rolled back: %w", err)
		}
		return fmt.Sprintf("added %s (%s) + additionalDirectories grant", repo.Name, repo.Path), nil
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

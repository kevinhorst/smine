package server

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/peek"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/server/respond"
)

const (
	pageRepos      = "repos"
	tmplCommitList = "_commit_list.html"
	tmplOpResult   = "_op_result.html"
	tmplRepoDetail = "repo_detail.html"
	tmplReposIndex = "repos_index.html"

	// agentBranchPrefix re-attaches what the URL param strips (D16).
	agentBranchPrefix = "claude/"

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
	Error   string
	Output  string
	Page    string
	Subject string
}

type repoDetailPage struct {
	Checkout     *repos.CheckoutStatus
	CheckoutErr  string
	Merged       string
	MergedValues []string
	Page         string
	Repo         repos.Repo
	Sessions     map[string]peek.Session
	SessionsErr  string
	StatusErr    string
	Statuses     []repos.WorktreeStatus
	Title        string
}

// presenceBadge is the index-cell view of a Presence (D4): one glyph badge,
// the detail in the tooltip.
type presenceBadge struct {
	Class string
	Glyph string
	Title string
}

func newPresenceBadge(presence repos.Presence, doc, dir string) presenceBadge {
	switch {
	case presence.HasRootDoc && presence.HasDir:
		return presenceBadge{Class: "badge-ok", Glyph: "✓", Title: doc + " · " + dir}
	case presence.HasRootDoc:
		return presenceBadge{Class: "badge-action", Glyph: "?", Title: "missing " + dir}
	case presence.HasDir:
		return presenceBadge{Class: "badge-action", Glyph: "?", Title: "missing " + doc}
	default:
		return presenceBadge{Class: "badge-error", Glyph: "✗", Title: "missing " + doc + " · " + dir}
	}
}

type repoRow struct {
	Claude    presenceBadge
	Codex     presenceBadge
	Context   repos.ContextPresence
	Repo      repos.Repo
	Worktrees int
}

type reposIndexPage struct {
	AddPath   folderPick
	Page      string
	RepoNames []string
	Rows      []repoRow
	Title     string
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

	output, err := op(r.Context())
	result := opResult{Output: output, Page: pageRepos, Subject: lockKey}
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

	branch := agentBranchPrefix + r.PathValue("branch")
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
		Merged: r.URL.Query().Get("merged"),
		Page:   pageRepos,
		Repo:   *repo,
		Title:  "Repo — " + repo.Name,
	}
	statuses, err := repos.Status(r.Context(), repo.Path, s.worktreeScripts)
	if err != nil {
		data.StatusErr = err.Error()
	}

	// Checkout down → degraded banner, the page still renders (like D3).
	checkout, err := repos.Checkout(r.Context(), repo.Path)
	if err != nil {
		data.CheckoutErr = err.Error()
	}
	data.Checkout = checkout

	// Filter values come from the unfiltered list so an active filter still
	// offers every other target (mirrors the sessions dimension filter).
	values := make(map[string]bool)
	for _, status := range statuses {
		for _, in := range status.In {
			values[in] = true
		}
		if data.Merged == "" || slices.Contains(status.In, data.Merged) {
			data.Statuses = append(data.Statuses, status)
		}
	}
	data.MergedValues = slices.Sorted(maps.Keys(values))

	// peek down → degraded session column, the page still renders (D3).
	sessions, err := s.peekClient.SessionsByCwd(r.Context())
	if err != nil {
		data.SessionsErr = err.Error()
	}
	data.Sessions = sessions
	s.renderFragment(w, tmplRepoDetail, data)
}

func (s *Server) handleRepoMerge(w http.ResponseWriter, r *http.Request) {
	repo := s.findRepo(w, r)
	if repo == nil {
		return
	}

	branch := agentBranchPrefix + r.PathValue("branch")
	op := func(ctx context.Context) (string, error) {
		return repos.Merge(ctx, branch, repo.Path, s.worktreeScripts)
	}
	s.runRepoOp(repo.Name, "repo-op", op, w, r)
}

func (s *Server) handleRepoRemove(w http.ResponseWriter, r *http.Request) {
	repo := s.findRepo(w, r)
	if repo == nil {
		return
	}

	branch := agentBranchPrefix + r.PathValue("branch")
	force := r.FormValue("force") == "on"
	deleteBranch := r.FormValue("delete-branch") == "on"
	op := func(ctx context.Context) (string, error) {
		return repos.Remove(ctx, branch, force, deleteBranch, repo.Path, s.worktreeScripts)
	}
	s.runRepoOp(repo.Name, "repo-op", op, w, r)
}

// handleRepoRemoveSelected removes every selected worktree in one op — the
// checkbox selection posts its branches as form values; the shared repo lock
// and one op-result cover the whole batch. Stops at the first failure;
// partial effects surface via the repo-op refresh like every other op.
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

	force := r.FormValue("force") == "on"
	deleteBranch := r.FormValue("delete-branch") == "on"
	op := func(ctx context.Context) (string, error) {
		var outputs []string
		for _, branch := range branches {
			output, err := repos.Remove(ctx, agentBranchPrefix+branch, force, deleteBranch, repo.Path, s.worktreeScripts)
			outputs = append(outputs, output)
			if err != nil {
				return strings.Join(outputs, "\n"), fmt.Errorf("%s: %w", branch, err)
			}
		}
		return strings.Join(outputs, "\n"), nil
	}
	s.runRepoOp(repo.Name, "repo-op", op, w, r)
}

func (s *Server) handleRepoSync(w http.ResponseWriter, r *http.Request) {
	repo := s.findRepo(w, r)
	if repo == nil {
		return
	}

	branch := agentBranchPrefix + r.PathValue("branch")
	op := func(ctx context.Context) (string, error) {
		return repos.Sync(ctx, branch, repo.Path, s.worktreeScripts)
	}
	s.runRepoOp(repo.Name, "repo-op", op, w, r)
}

func (s *Server) handleReposIndex(w http.ResponseWriter, r *http.Request) {
	data := reposIndexPage{Page: pageRepos, Title: "Repos"}
	for _, repo := range s.repoRegistry.Repos() {
		data.RepoNames = append(data.RepoNames, repo.Name)
		row := repoRow{
			Claude:    newPresenceBadge(repos.DetectClaude(repo.Path), "CLAUDE.md", ".claude"),
			Codex:     newPresenceBadge(repos.DetectCodex(repo.Path), "AGENTS.md", ".codex"),
			Context:   repos.DetectContext(repo.Path),
			Repo:      repo,
			Worktrees: repos.CountWorktrees(repo.Path),
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
	repo := repos.Repo{Name: filepath.Base(path), Path: path}
	op := func(ctx context.Context) (string, error) {
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

package server

import (
	"context"
	"net/http"

	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/secretscan"
	"github.com/kevinhorst/smine/internal/server/respond"
)

const (
	pageTools = "tools"
	tmplTools = "tools.html"

	// jetbrainsLockKey serializes the global prune via the repo-locks map;
	// registry names are user-authored but the prune route is literal, so the
	// key can never race a repo of the same name's routes (D30).
	jetbrainsLockKey = "_jetbrains"
)

// toolNames mirrors the actions offered on the Tools page (tools.html).
var toolNames = []string{"prune-jetbrains", "secretscan"}

type toolsPage struct {
	Page      string
	RepoNames []string
	Title     string
}

func (s *Server) handleToolsIndex(w http.ResponseWriter, r *http.Request) {
	data := toolsPage{Page: pageTools, Title: "Tools"}
	for _, repo := range s.repoRegistry.Repos() {
		data.RepoNames = append(data.RepoNames, repo.Name)
	}
	s.renderFragment(w, tmplTools, data)
}

func (s *Server) handleToolsPruneJetbrains(w http.ResponseWriter, r *http.Request) {
	dryRun := r.FormValue("dry-run") == "on"
	force := r.FormValue("force") == "on"
	op := func(ctx context.Context) (string, error) {
		return repos.PruneJetbrains(ctx, dryRun, force, s.worktreeScripts)
	}
	s.runRepoOp(jetbrainsLockKey, "repo-op", op, w, r)
}

func (s *Server) handleToolsSecretScan(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.repoRegistry.Find(r.FormValue("name"))
	if !ok {
		respond.WithBadRequest("unknown repo", w)
		return
	}

	history := r.FormValue("history") == "on"
	op := func(ctx context.Context) (string, error) {
		result, err := secretscan.Scan(repo.Path, secretscan.Options{ShouldScanHistory: history})
		if err != nil {
			return "", err
		}
		data, err := secretscan.RenderJson(result)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	s.runRepoOp(repo.Name, "repo-op", op, w, r)
}

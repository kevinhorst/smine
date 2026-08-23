package server

import (
	"context"
	"net/http"

	"github.com/kevinhorst/smine/internal/repos"
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
var toolNames = []string{"prune-jetbrains"}

type toolsPage struct {
	Page  string
	Title string
}

func (s *Server) handleToolsIndex(w http.ResponseWriter, r *http.Request) {
	data := toolsPage{Page: pageTools, Title: "Tools"}
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

package server

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/server/respond"
)

// syncLockKey serializes the engine's writers; the env, model, permission, MCP and row editors write settings.json without it and keep the previous bytes through config.Save.
const syncLockKey = "_sync"

// handleSync runs the ticked targets over the stored record; all is the Profile page's full apply, restart included.
func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		respond.WithBadRequest("invalid form", w)
		return
	}

	// target
	targets := r.PostForm["target"]
	if len(targets) == 0 {
		respond.WithBadRequest("pick at least one sync target", w)
		return
	}
	for _, target := range targets {
		if !install.IsSyncTarget(target) {
			respond.WithBadRequest(fmt.Sprintf("unknown sync target %q", target), w)
			return
		}
	}

	record, err := s.records.load(s.machine)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	if slices.Contains(targets, install.SyncTargetAll) {
		s.applyInstallation(record, w, r)
		return
	}

	op := func(ctx context.Context) (string, error) {
		var report strings.Builder
		env := s.machine
		env.Stdout = &report
		err := install.SyncTargets(ctx, env, record, targets)
		if slices.Contains(targets, install.SyncTargetSkills) {
			s.titles.reload()
		}
		return report.String(), err
	}
	s.runRepoOp(syncLockKey, "repo-op", op, w, r)
}

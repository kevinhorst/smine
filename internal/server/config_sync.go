package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/kevinhorst/smine/internal/codex"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/server/catalog"
	"github.com/kevinhorst/smine/internal/server/respond"
	"github.com/kevinhorst/smine/internal/trash"
)

// syncMode is the direction of a config sync op; force rebuilds the live Claude file from the template alone.
type syncMode int

const (
	syncModePersist syncMode = iota
	syncModeSync
	syncModeForce
)

const tmplForcePreview = "_force_preview.html"

// forcePreviewData is the Force sync preview: what the rebuild removes and the target its confirm posts to.
type forcePreviewData struct {
	Report *install.ForceReport
	Target string
}

func (s *Server) handleConfigApplyOverrides(w http.ResponseWriter, r *http.Request) {
	s.handleConfigSyncOp(w, r, syncModePersist)
}

// handleConfigForce runs only the confirm of the preview fragment, which posts confirmed=on (master B8).
func (s *Server) handleConfigForce(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("confirmed") != "on" {
		respond.WithBadRequest("preview first: the Force sync button shows what goes, its confirm runs the rebuild", w)
		return
	}
	s.handleConfigSyncOp(w, r, syncModeForce)
}

// handleConfigForcePreview renders what a Force sync would remove, with the confirm that runs it (master B8).
func (s *Server) handleConfigForcePreview(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")
	if target != catalog.TargetClaude {
		respond.WithNotFound("force preview exists for the claude target only", w)
		return
	}

	record, err := s.records.load(s.machine)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	report, err := install.ForcePreview(s.machine, record)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	data := forcePreviewData{Report: report, Target: target}
	s.renderFragment(w, tmplForcePreview, data)
}

func (s *Server) handleConfigRevert(w http.ResponseWriter, r *http.Request) {
	s.handleConfigSyncOp(w, r, syncModeSync)
}

// handleConfigSyncOp refuses a result that does not parse as the target's format, writing nothing; Codex force is its whole-file sync.
func (s *Server) handleConfigSyncOp(w http.ResponseWriter, r *http.Request, mode syncMode) {
	target := r.PathValue("target")
	if !catalog.IsTarget(target) {
		respond.WithNotFound("unknown target: "+target, w)
		return
	}
	if target != catalog.TargetCodex {
		s.handleClaudeSync(w, r, mode)
		return
	}

	revert := mode != syncModePersist
	src, dst := s.codexPath, s.codexFragmentPath
	if revert {
		src, dst = s.codexFragmentPath, s.codexPath
	}

	op := func(context.Context) (string, error) {
		before, err := os.ReadFile(dst)
		if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("handleConfigSyncOp: Failed to read %s: %w", dst, err)
		}
		data, err := os.ReadFile(src)
		if err != nil {
			return "", fmt.Errorf("handleConfigSyncOp: Failed to read %s: %w", src, err)
		}
		if revert {
			data = s.markers().Expand(data)
		} else if data, err = s.markers().Collapse(data, before); err != nil {
			return "", fmt.Errorf("handleConfigSyncOp: %w", err)
		}
		if err := validateTarget(target, data); err != nil {
			return "", fmt.Errorf("handleConfigSyncOp: Refusing to write %s: %w", dst, err)
		}
		entry := &trash.Entry{Action: "codex sync", Reason: filepath.Base(dst) + " overwritten"}
		if _, err := s.trash.Keep(dst, entry); err != nil {
			return "", fmt.Errorf("handleConfigSyncOp: %w", err)
		}
		if err := fsx.WriteFileAtomic(data, dst, 0o644); err != nil {
			return "", fmt.Errorf("handleConfigSyncOp: %w", err)
		}

		output := fmt.Sprintf("synced: %s -> %s", src, dst)
		diff, err := fileDiff(before, dst)
		if err != nil {
			return output, fmt.Errorf("handleConfigSyncOp: Failed to diff %s: %w", dst, err)
		}
		if diff == nil {
			output += "\n(no changes)"
		} else {
			output += "\n" + formatDiff(diff)
		}
		return output, nil
	}
	s.runRepoOp(syncLockKey, "config-op", op, w, r)
}

// handleClaudeSync: persist writes the live settings back into base.json and the owning fragments; sync runs the engine's settings step, parked groups kept; force rebuilds the live file from the template.
func (s *Server) handleClaudeSync(w http.ResponseWriter, r *http.Request, mode syncMode) {
	syncSettings := func(ctx context.Context) (string, error) {
		env := s.machine
		env.Stdout = io.Discard

		var changes []string
		var err error
		switch mode {
		case syncModePersist:
			changes, err = install.PersistSettings(env)
		case syncModeSync:
			changes, err = syncClaudeSettings(env, install.ApplySettings)
		case syncModeForce:
			changes, err = syncClaudeSettings(env, install.ForceSettings)
		}
		if len(changes) == 0 {
			return "(no changes)", err
		}
		return strings.Join(changes, "\n"), err
	}
	s.runRepoOp(syncLockKey, "config-op", syncSettings, w, r)
}

// syncClaudeSettings runs one engine settings step over the stored record.
func syncClaudeSettings(env install.Env, step func(install.Env, install.Record) ([]string, error)) ([]string, error) {
	record, _, err := install.Load(env)
	if err != nil {
		return nil, fmt.Errorf("syncClaudeSettings: %w", err)
	}
	return step(env, record)
}

// sectionOverridden degrades a fragment load error to false: the sections must never 500 over the badge.
func (s *Server) sectionOverridden(main *config.Settings, keys ...string) bool {
	fragment, err := config.LoadFragment(s.claudeFragmentPath, s.markers())
	if err != nil {
		return false
	}

	for _, key := range keys {
		if claudeOverridden(main.Doc(), fragment.Doc(), []string{key}) {
			return true
		}
	}
	return false
}

// validateTarget reports whether data parses as the target's config format.
func validateTarget(target string, data []byte) error {
	if target == catalog.TargetCodex {
		_, err := codex.Parse(data)
		return err
	}
	if !json.Valid(data) {
		return errors.New("not valid JSON")
	}
	return nil
}

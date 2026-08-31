package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/server/catalog"
	"github.com/kevinhorst/smine/internal/server/respond"
)

// configSyncLockKey serializes both sync directions via the repo-locks map;
// underscore-prefixed like _jetbrains to stay out of repo-name space.
const configSyncLockKey = "_config-sync"

// syncPaths resolves the live/fragment file pair for a target.
func (s *Server) syncPaths(target string) (live, fragment string) {
	if target == catalog.TargetCodex {
		return s.codexPath, s.codexFragmentPath
	}
	return s.settingsPath, s.claudeFragmentPath
}

func (s *Server) handleConfigApplyOverrides(w http.ResponseWriter, r *http.Request) {
	s.handleConfigSyncOp(w, r, false)
}

func (s *Server) handleConfigRevert(w http.ResponseWriter, r *http.Request) {
	s.handleConfigSyncOp(w, r, true)
}

// handleConfigSyncOp copies live→fragment (apply) or fragment→live (revert).
func (s *Server) handleConfigSyncOp(w http.ResponseWriter, r *http.Request, revert bool) {
	target := r.PathValue("target")
	if !catalog.IsTarget(target) {
		respond.WithNotFound("unknown target: "+target, w)
		return
	}

	live, fragment := s.syncPaths(target)
	src, dst := live, fragment
	if revert {
		src, dst = fragment, live
	}

	expandMarkers := revert && target != catalog.TargetCodex

	op := func(context.Context) (string, error) {
		before, err := os.ReadFile(dst)
		if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("handleConfigSyncOp: Failed to read %s: %w", dst, err)
		}
		output, err := copyFileAtomic(src, dst, expandMarkers)
		if err != nil {
			return output, err
		}
		diff, err := fileDiff(before, dst)
		if err != nil {
			return output, fmt.Errorf("handleConfigSyncOp: Failed to diff %s: %w", dst, err)
		}
		if diff == nil {
			output += "\n(no changes)"
		} else {
			output += "\n" + formatDiff(diff)
		}
		if !revert || target == catalog.TargetCodex {
			return output, nil
		}

		// Revert makes the repo fragment the whole truth: parked disabled
		// state would re-append as duplicates on the next enable toggle.
		if err := s.clearParkedClaudeState(); err != nil {
			return output, err
		}
		return output + "\ncleared parked disabled state", nil
	}
	s.runRepoOp(configSyncLockKey, "config-op", op, w, r)
}

// clearParkedClaudeState empties the disabled-hooks sidecar and the
// settings.disabled.json sidecar.
func (s *Server) clearParkedClaudeState() error {
	if err := s.disabledHooks.Clear(); err != nil {
		return fmt.Errorf("clearParkedClaudeState: %w", err)
	}
	if err := config.Save(config.DisabledPath(s.settingsPath), config.NewSettings()); err != nil {
		return fmt.Errorf("clearParkedClaudeState: %w", err)
	}
	return nil
}

// sectionOverridden reports whether any of the section's top-level keys
// differs between the live settings and the repo fragment. A fragment load
// error degrades to false — the sections must never 500 over the badge.
func (s *Server) sectionOverridden(main *config.Settings, keys ...string) bool {
	fragment, err := config.LoadFragment(s.claudeFragmentPath)
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

// copyFileAtomic replaces dst with src's content via tmp+rename; a missing
// src is an error — the buttons are disabled when the fragment is absent,
// so this only fires on a race.
func copyFileAtomic(src, dst string, expandMarkers bool) (string, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return "", fmt.Errorf("copyFileAtomic: Failed to read %s: %w", src, err)
	}
	if expandMarkers {
		data = config.ExpandInstallMarkers(data)
	}

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".config-sync-*")
	if err != nil {
		return "", fmt.Errorf("copyFileAtomic: Failed to create temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", fmt.Errorf("copyFileAtomic: Failed to write %s: %w", tmp.Name(), err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("copyFileAtomic: Failed to close %s: %w", tmp.Name(), err)
	}
	if err := fsx.ReplaceFile(tmp.Name(), dst); err != nil {
		return "", fmt.Errorf("copyFileAtomic: Failed to rename to %s: %w", dst, err)
	}
	return fmt.Sprintf("synced: %s -> %s", src, dst), nil
}

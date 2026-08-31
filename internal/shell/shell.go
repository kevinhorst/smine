// Package shell runs allowlisted external commands (worktree scripts, git,
// launchctl) with a hard timeout and captured output. Callers never pass
// request input as the command name — only as pre-validated arguments.
package shell

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"time"
)

// Timeout kills any operation after 60s — the status script on a large repo
// is the ceiling (concept limit).
const Timeout = 60 * time.Second

// SyncTimeout bounds the sync scripts — sync_skills spawns hundreds of
// processes under Windows Git Bash; the 60s Run ceiling killed cold installs.
const SyncTimeout = 10 * time.Minute

// Run executes name with args in dir (empty dir = inherit cwd). The combined
// stdout+stderr is returned even when the command fails, so callers can
// render script output alongside the error.
func Run(ctx context.Context, dir, name string, args ...string) (string, error) {
	runCtx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()

	name, args = platformArgv(name, args)
	cmd := exec.CommandContext(runCtx, name, args...)
	cmd.Dir = dir
	HideWindow(cmd)
	// Without WaitDelay a grandchild holding the output pipe keeps
	// CombinedOutput blocked past the kill — the timeout would never fire
	// for scripts that spawn children.
	cmd.WaitDelay = time.Second
	start := time.Now()
	output, err := cmd.CombinedOutput()
	log.Printf("shell: %s dur=%dms err=%v", filepath.Base(name), time.Since(start).Milliseconds(), err != nil)
	if err != nil {
		return string(output), fmt.Errorf("Run: %s in %s: %w", name, dir, err)
	}

	return string(output), nil
}

// RunSync is Run with the sync-script deadline; sync_* scripts and
// ensure_git_repo.sh are the only sanctioned callers.
func RunSync(ctx context.Context, dir, name string, args ...string) (string, error) {
	runCtx, cancel := context.WithTimeout(ctx, SyncTimeout)
	defer cancel()

	name, args = platformArgv(name, args)
	cmd := exec.CommandContext(runCtx, name, args...)
	cmd.Dir = dir
	HideWindow(cmd)
	cmd.WaitDelay = time.Second
	start := time.Now()
	output, err := cmd.CombinedOutput()
	log.Printf("shell: %s dur=%dms err=%v", filepath.Base(name), time.Since(start).Milliseconds(), err != nil)
	if err != nil {
		return string(output), fmt.Errorf("RunSync: %s in %s: %w", name, dir, err)
	}

	return string(output), nil
}

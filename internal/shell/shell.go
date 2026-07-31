// Package shell runs allowlisted external commands (worktree scripts, git,
// launchctl) with a hard timeout and captured output. Callers never pass
// request input as the command name — only as pre-validated arguments.
package shell

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Timeout kills any operation after 60s — the status script on a large repo
// is the ceiling (concept limit).
const Timeout = 60 * time.Second

// Run executes name with args in dir (empty dir = inherit cwd). The combined
// stdout+stderr is returned even when the command fails, so callers can
// render script output alongside the error.
func Run(ctx context.Context, dir, name string, args ...string) (string, error) {
	runCtx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, name, args...)
	cmd.Dir = dir
	// Without WaitDelay a grandchild holding the output pipe keeps
	// CombinedOutput blocked past the kill — the timeout would never fire
	// for scripts that spawn children.
	cmd.WaitDelay = time.Second
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("Run: %s in %s: %w", name, dir, err)
	}

	return string(output), nil
}

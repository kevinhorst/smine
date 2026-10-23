// Package shell runs allowlisted external commands with a hard timeout and captured output.
package shell

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"time"
)

// Timeout's ceiling is the status script on a large repo.
const Timeout = 60 * time.Second

// SyncTimeout exists because the 60s Run ceiling killed cold sync installs under Windows Git Bash.
const SyncTimeout = 10 * time.Minute

func run(ctx context.Context, dir, name string, timeout time.Duration, args ...string) (string, error) {
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	name, args = platformArgv(name, args)
	cmd := exec.CommandContext(runCtx, name, args...)
	cmd.Dir = dir
	HideWindow(cmd)
	// without WaitDelay a grandchild holding the output pipe blocks CombinedOutput past the kill
	cmd.WaitDelay = time.Second
	start := time.Now()
	output, err := cmd.CombinedOutput()
	log.Printf("shell: %s dur=%dms err=%v", filepath.Base(name), time.Since(start).Milliseconds(), err != nil)
	if err != nil {
		return string(output), fmt.Errorf("run: %s in %s: %w", name, dir, err)
	}

	return string(output), nil
}

// Run returns the combined output even when the command fails.
func Run(ctx context.Context, dir, name string, args ...string) (string, error) {
	output, err := run(ctx, dir, name, Timeout, args...)
	if err != nil {
		return output, fmt.Errorf("Run: %w", err)
	}
	return output, nil
}

// RunSync's only sanctioned callers are the sync_* scripts and the install's initial commit.
func RunSync(ctx context.Context, dir, name string, args ...string) (string, error) {
	output, err := run(ctx, dir, name, SyncTimeout, args...)
	if err != nil {
		return output, fmt.Errorf("RunSync: %w", err)
	}
	return output, nil
}

// RunWithin's only sanctioned caller is the ACDSL verifier path, whose registry entries carry their own bounded timeout.
func RunWithin(ctx context.Context, dir, name string, timeout time.Duration, args ...string) (string, error) {
	output, err := run(ctx, dir, name, timeout, args...)
	if err != nil {
		return output, fmt.Errorf("RunWithin: %w", err)
	}
	return output, nil
}

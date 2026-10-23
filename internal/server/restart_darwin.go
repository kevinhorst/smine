//go:build darwin

package server

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// restartHelperTimeout bounds the helper: a grace second, the bootout wait (20 × 250 ms) and the bootstrap.
const restartHelperTimeout = time.Minute

// restartServerAgent runs bin/configserver -restart-agent in its own session so launchd's bootout, which kills only the job's process group, spares it (master C3: the caller restarts).
func restartServerAgent(repoDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), restartHelperTimeout)
	helper := exec.CommandContext(ctx, filepath.Join(repoDir, "bin", "configserver"), "-restart-agent")
	helper.Dir = repoDir
	helper.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := helper.Start(); err != nil {
		cancel()
		return fmt.Errorf("restartServerAgent: %w", err)
	}

	go func() {
		_ = helper.Wait()
		cancel()
	}()
	return nil
}

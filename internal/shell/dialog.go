package shell

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// DialogTimeout bounds interactive dialog children — generous because a
// human is browsing, unlike Run's 60s script ceiling.
const DialogTimeout = 10 * time.Minute

// RunDialog executes a command that must show a window and wait for the
// user: no HideWindow (SW_HIDE would suppress the dialog), no console
// (dialogAttr), DialogTimeout instead of Timeout. Callers are the folder
// pickers only.
func RunDialog(ctx context.Context, name string, args ...string) (string, error) {
	runCtx, cancel := context.WithTimeout(ctx, DialogTimeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, name, args...)
	dialogAttr(cmd)
	cmd.WaitDelay = time.Second
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("RunDialog: %s: %w", name, err)
	}
	return string(output), nil
}

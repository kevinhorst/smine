package shell

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// DialogTimeout is generous because a human is browsing the dialog.
const DialogTimeout = 10 * time.Minute

// RunDialog skips HideWindow because SW_HIDE would suppress the dialog.
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

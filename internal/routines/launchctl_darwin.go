package routines

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/kevinhorst/smine/internal/shell"
)

// notFoundExit is launchctl's exit code for an unknown or unloaded label
// (F14, probed on this machine).
const notFoundExit = 113

func guiTarget(label string) string {
	return fmt.Sprintf("gui/%d/%s", os.Getuid(), label)
}

// IsLoaded probes `launchctl print` and reads only the exit code — the output
// is explicitly not a stable API (D23).
func IsLoaded(ctx context.Context, label string) (bool, error) {
	_, err := shell.Run(ctx, "", "launchctl", "print", guiTarget(label))
	if err == nil {
		return true, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == notFoundExit {
		return false, nil
	}
	return false, fmt.Errorf("IsLoaded: %s: %w", label, err)
}

// RunNow kickstarts the loaded job; completion is not synchronous — the row
// polls results.jsonl for a newer line instead (D25, deferred finding in
// Scope).
func RunNow(ctx context.Context, label string) (string, error) {
	output, err := shell.Run(ctx, "", "launchctl", "kickstart", guiTarget(label))
	if err != nil {
		return output, fmt.Errorf("RunNow: %s: %w", label, err)
	}
	return output, nil
}

// Start enables and loads the routine (launchctl enable + bootstrap). Enable
// clears a persisted Stop override so the routine survives the next login's
// SyncAll
func Start(ctx context.Context, label, plistPath string) (string, error) {
	output, err := shell.Run(ctx, "", "launchctl", "enable", guiTarget(label))
	if err != nil {
		return output, fmt.Errorf("Start: enable %s: %w", label, err)
	}

	output, err = shell.Run(ctx, "", "launchctl", "bootstrap", fmt.Sprintf("gui/%d", os.Getuid()), plistPath)
	if err != nil {
		return output, fmt.Errorf("Start: %s: %w", plistPath, err)
	}

	return output, nil
}

// Stop unloads the routine and persists the off state (launchctl bootout +
// disable) so SyncAll does not revive it at the next server start
func Stop(ctx context.Context, label string) (string, error) {
	output, err := shell.Run(ctx, "", "launchctl", "bootout", guiTarget(label))
	if err != nil {
		return output, fmt.Errorf("Stop: %s: %w", label, err)
	}

	if out, err := shell.Run(ctx, "", "launchctl", "disable", guiTarget(label)); err != nil {
		return out, fmt.Errorf("Stop: disable %s: %w", label, err)
	}

	return output, nil
}

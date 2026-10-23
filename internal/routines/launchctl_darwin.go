package routines

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

// notFoundExit is launchctl's exit code for an unknown or unloaded label, as probed.
const notFoundExit = 113

func guiTarget(label string) string {
	return fmt.Sprintf("gui/%d/%s", os.Getuid(), label)
}

// IsLoaded reads only the exit code: `launchctl print` output is not a stable API.
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

// IsRunning reports whether the loaded job has a live instance.
func IsRunning(ctx context.Context, label string) (bool, error) {
	pid, err := RunPid(ctx, label)
	return pid != 0, err
}

// RunPid degrades unmatched output to 0, the safe direction for the reload guard.
func RunPid(ctx context.Context, label string) (int, error) {
	output, err := shell.Run(ctx, "", "launchctl", "print", guiTarget(label))
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == notFoundExit {
			return 0, nil
		}
		return 0, fmt.Errorf("RunPid: %s: %w", label, err)
	}
	return parseRunPid(output), nil
}

func parseRunPid(output string) int {
	for _, line := range strings.Split(output, "\n") {
		value, found := strings.CutPrefix(strings.TrimSpace(line), "pid = ")
		if !found {
			continue
		}

		pid, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return 0
		}
		return pid
	}
	return 0
}

// RegisterLogonTask and StopTask are Task Scheduler verbs; launchd installs its agents through plists.
func RegisterLogonTask(ctx context.Context, label, xml string) (string, error) {
	return "", errors.New("RegisterLogonTask: Logon tasks exist on windows only")
}

func StopTask(ctx context.Context, label string) (string, error) {
	return "", errors.New("StopTask: Logon tasks exist on windows only")
}

// RunNow is asynchronous; the row polls results.jsonl for a newer line.
func RunNow(ctx context.Context, label string) (string, error) {
	output, err := shell.Run(ctx, "", "launchctl", "kickstart", guiTarget(label))
	if err != nil {
		return output, fmt.Errorf("RunNow: %s: %w", label, err)
	}
	return output, nil
}

// Start enables before bootstrap, clearing a persisted Stop override for the next login's SyncAll.
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

// Stop also disables, so SyncAll does not revive the routine at the next server start.
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

// Unregister has nothing to do under launchd: Stop booted the agent out and the plist moves with the routine dir.
func Unregister(ctx context.Context, label string) (string, error) {
	return "", nil
}

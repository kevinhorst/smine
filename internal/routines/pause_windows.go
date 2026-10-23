//go:build windows

package routines

import (
	"context"
	"fmt"
)

// PauseSupported gates the pause/resume UI; windows suspends the task's tree through ntdll.
const PauseSupported = true

// Pause suspends the whole tree; the cells' stage deadlines keep running, as on darwin.
func Pause(ctx context.Context, label string) (int, error) {
	pid, err := RunPid(ctx, label)
	if err != nil {
		return 0, err
	}
	if pid == 0 {
		return 0, fmt.Errorf("Pause: %s: No active run", label)
	}

	if _, err := SuspendTree(pid); err != nil {
		return 0, fmt.Errorf("Pause: %w", err)
	}
	return pid, nil
}

// Resume treats a run that ended while paused as a no-op; the caller clears the marker either way.
func Resume(ctx context.Context, label string) error {
	pid, err := RunPid(ctx, label)
	if err != nil {
		return err
	}
	if pid == 0 {
		return nil
	}

	if _, err := ResumeTree(pid); err != nil {
		return fmt.Errorf("Resume: %w", err)
	}
	return nil
}

//go:build !darwin && !windows

package routines

import "context"

// KillRun off darwin restarts the job: it cannot signal the scheduler-owned process tree directly.
func KillRun(ctx context.Context, cellPidsPath, label, plistPath string) (int, error) {
	if _, err := Stop(ctx, label); err != nil {
		return 0, err
	}
	if _, err := Start(ctx, label, plistPath); err != nil {
		return 0, err
	}
	return 1, nil
}

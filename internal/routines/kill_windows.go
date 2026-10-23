//go:build windows

package routines

import (
	"context"
	"fmt"
)

// KillRun ends the task's tree, then every registered cell's tree; the count is what TerminateProcess ended.
func KillRun(ctx context.Context, cellPidsPath, label, plistPath string) (int, error) {
	killed := 0
	rootPid, err := RunPid(ctx, label)
	if err != nil {
		return 0, err
	}
	if rootPid > 0 {
		ended, err := TerminateTree(rootPid)
		if err != nil {
			return 0, fmt.Errorf("KillRun: %w", err)
		}
		killed += ended
	}

	for _, pid := range ReadCellPids(cellPidsPath) {
		ended, err := TerminateTree(pid)
		if err != nil {
			continue
		}
		killed += ended
	}
	return killed, nil
}

package routines

import (
	"context"
	"syscall"
)

// KillRun freezes the tree with SIGSTOP while collecting: a TERM'd parent reparents its children mid-walk.
func KillRun(ctx context.Context, cellPidsPath, label, plistPath string) (int, error) {
	killed := 0

	rootPid, err := RunPid(ctx, label)
	if err != nil {
		return 0, err
	}
	if rootPid > 0 {
		pids := collectTree(ctx, rootPid)
		for _, pid := range pids {
			_ = syscall.Kill(pid, syscall.SIGTERM)
		}
		for _, pid := range pids {
			_ = syscall.Kill(pid, syscall.SIGCONT)
		}
		killed += len(pids)
	}

	for _, pid := range ReadCellPids(cellPidsPath) {
		if syscall.Kill(pid, 0) != nil {
			continue
		}
		if syscall.Kill(-pid, syscall.SIGTERM) == nil {
			killed++
		}
	}
	return killed, nil
}

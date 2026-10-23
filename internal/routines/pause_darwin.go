package routines

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"syscall"

	"github.com/kevinhorst/smine/internal/shell"
)

// PauseSupported gates the pause/resume UI; only darwin can signal the launchd-owned process tree.
const PauseSupported = true

// Pause does not stop the stage ceiling: the runner's deadline timer keeps running and its cancel lands on SIGCONT.
func Pause(ctx context.Context, label string) (int, error) {
	pid, err := RunPid(ctx, label)
	if err != nil {
		return 0, err
	}
	if pid == 0 {
		return 0, fmt.Errorf("Pause: %s: No active run", label)
	}

	signalTree(ctx, pid, syscall.SIGSTOP)
	return pid, nil
}

// Resume treats a run that died while paused as a no-op; the caller clears the marker either way.
func Resume(ctx context.Context, label string) error {
	pid, err := RunPid(ctx, label)
	if err != nil {
		return err
	}
	if pid == 0 {
		return nil
	}

	signalTree(ctx, pid, syscall.SIGCONT)
	return nil
}

// childPids reads pgrep's exit 1 ("no matches") as a leaf, not an error.
func childPids(ctx context.Context, pid int) []int {
	output, err := shell.Run(ctx, "", "pgrep", "-P", strconv.Itoa(pid))
	if err != nil {
		return nil
	}

	var children []int
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		child, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil {
			continue
		}
		children = append(children, child)
	}
	return children
}

// collectTree leaves the pids stopped; the caller must SIGCONT them after signaling.
func collectTree(ctx context.Context, rootPid int) []int {
	var collected []int
	queue := []int{rootPid}
	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		if err := syscall.Kill(pid, syscall.SIGSTOP); err != nil {
			continue
		}

		collected = append(collected, pid)
		queue = append(queue, childPids(ctx, pid)...)
	}
	return collected
}

// signalTree signals each node before listing its children, so SIGSTOP freezes the set it walks.
func signalTree(ctx context.Context, rootPid int, signal syscall.Signal) {
	queue := []int{rootPid}
	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		if err := syscall.Kill(pid, signal); err != nil {
			continue
		}

		queue = append(queue, childPids(ctx, pid)...)
	}
}

package routines

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Run files shared by the runner (cmd/routine) and the config server.
const (
	CellPidsFileName       = ".cell-pids"
	ResultsFileName        = "results.jsonl"
	RunCompleteFileName    = "run-complete"
	RunNowPromptFileName   = ".run-now-prompt"
	SteerDeliveredFileName = "steer-delivered"
	SteerPromptFileName    = "steer-prompt"
)

// SteerReceipt names the delivered prompt so reconciliation matches by content, not by file mtimes.
type SteerReceipt struct {
	Cell        string    `json:"cell"`
	DeliveredAt time.Time `json:"delivered_at"`
	Interrupt   bool      `json:"interrupt"`
	Prompt      string    `json:"prompt"`
}

func cacheRoot(override, leaf string) (string, error) {
	if override != "" {
		return filepath.Clean(override), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cacheRoot: %w", err)
	}
	return filepath.Join(home, ".cache", "claude-routine", leaf), nil
}

// ContainerRoot holds slot locks and rendered settings; no container mounts it.
func ContainerRoot(env map[string]string) (string, error) {
	return cacheRoot(env["ROUTINE_CONTAINER_ROOT"], "container")
}

// GroupName is ROUTINE_GROUP, defaulting to the routine's own name.
func GroupName(env map[string]string, routineName string) string {
	if group := env["ROUTINE_GROUP"]; group != "" {
		return group
	}
	return routineName
}

// GroupWorktree also holds the cell-pgid registry.
func GroupWorktree(env map[string]string, routineName string) (string, error) {
	root, err := WorktreeRoot(env)
	if err != nil {
		return "", fmt.Errorf("GroupWorktree: %w", err)
	}
	return filepath.Join(root, GroupName(env, routineName)), nil
}

// ReadCellPids rejects pid 1: signaling group -1 would broadcast to every process.
func ReadCellPids(path string) []int {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var pids []int
	for line := range strings.SplitSeq(string(data), "\n") {
		pid, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil || pid <= 1 {
			continue
		}
		pids = append(pids, pid)
	}
	return pids
}

// SteerDir lives outside the worktree so the steer channel survives worktree teardown.
func SteerDir(env map[string]string, routineName string) (string, error) {
	root, err := cacheRoot(env["ROUTINE_STEER_ROOT"], "steer")
	if err != nil {
		return "", fmt.Errorf("SteerDir: %w", err)
	}
	return filepath.Join(root, GroupName(env, routineName)), nil
}

func WorktreeRoot(env map[string]string) (string, error) {
	return cacheRoot(env["ROUTINE_WT_ROOT"], "worktrees")
}

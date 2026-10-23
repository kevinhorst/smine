package reviewscope

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Snapshot scopes differ only in the base the caller pairs: HEAD for wip, the fork point for all.
const (
	SnapshotScopeAll = "all"
	SnapshotScopeWip = "wip"
)

// snapshotTimeout bounds each git step of a snapshot.
const snapshotTimeout = time.Minute

type SnapshotResult struct {
	Differs  bool   `json:"differs"`
	Snapshot string `json:"snapshot"`
}

// Snapshot commits through a temporary index; no branch, index or working-tree file is touched.
func Snapshot(ctx context.Context, repo, scope string) (*SnapshotResult, error) {
	switch scope {
	case SnapshotScopeAll, SnapshotScopeWip:
	default:
		return nil, fmt.Errorf("Snapshot: Unknown scope %q: want wip or all", scope)
	}

	tempDir, err := os.MkdirTemp("", "railroad-snapshot-")
	if err != nil {
		return nil, fmt.Errorf("Snapshot: %w", err)
	}
	defer os.RemoveAll(tempDir)

	index := filepath.Join(tempDir, "index")
	steps := [][]string{{"read-tree", "HEAD"}, {"add", "-A"}}
	for _, step := range steps {
		if _, err := gitIndexed(ctx, repo, index, step...); err != nil {
			return nil, err
		}
	}

	tree, err := gitIndexed(ctx, repo, index, "write-tree")
	if err != nil {
		return nil, err
	}

	headTree, err := gitLine(ctx, repo, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return nil, err
	}

	commit, err := gitLine(ctx, repo, "commit-tree", tree, "-p", "HEAD", "-m", "railroad-review "+scope+" snapshot")
	if err != nil {
		return nil, err
	}
	return &SnapshotResult{Differs: tree != headTree, Snapshot: commit}, nil
}

// gitIndexed runs one git command in repo against the temporary index.
func gitIndexed(ctx context.Context, repo, index string, args ...string) (string, error) {
	stepCtx, cancel := context.WithTimeout(ctx, snapshotTimeout)
	defer cancel()

	cmd := exec.CommandContext(stepCtx, "git", args...)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+index)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Snapshot: git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

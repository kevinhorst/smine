// Package desktop reads the Claude Desktop app's on-disk stores, written before a session's CLI spawns.
package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const worktreeStoreName = "Claude/git-worktrees.json"

// ErrNoWorktree marks a name the store does not hold.
var ErrNoWorktree = errors.New("desktop: No worktree entry")

// Worktree is the slice of a store entry the hooks read.
type Worktree struct {
	HookBased    bool   `json:"hookBased"`
	LeasedBy     string `json:"leasedBy"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	SourceBranch string `json:"sourceBranch"`
}

// WorktreeStore is the store file's shape.
type WorktreeStore struct {
	Worktrees map[string]*Worktree `json:"worktrees"`
}

// ReadWorktree returns the entry named name from the store at storePath.
func ReadWorktree(storePath string, name string) (*Worktree, error) {
	raw, err := os.ReadFile(storePath)
	if err != nil {
		return nil, fmt.Errorf("desktop.ReadWorktree: Failed to read %s: %w", storePath, err)
	}

	store := &WorktreeStore{}
	if err := json.Unmarshal(raw, store); err != nil {
		return nil, fmt.Errorf("desktop.ReadWorktree: Failed to decode %s: %w", storePath, err)
	}

	worktree, isKnown := store.Worktrees[name]
	if !isKnown || worktree == nil {
		return nil, fmt.Errorf("desktop.ReadWorktree: %s holds no entry %q: %w", storePath, name, ErrNoWorktree)
	}
	return worktree, nil
}

// WorktreeStorePath is the store file under the user config dir.
func WorktreeStorePath(configDir string) string {
	return filepath.Join(configDir, filepath.FromSlash(worktreeStoreName))
}

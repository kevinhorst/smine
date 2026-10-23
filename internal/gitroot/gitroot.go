// Package gitroot resolves a path's working-tree root and main checkout without running git.
package gitroot

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	gitDirPrefix      = "gitdir:"
	gitEntry          = ".git"
	gitWorktreesInfix = string(os.PathSeparator) + ".git" + string(os.PathSeparator) + "worktrees" + string(os.PathSeparator)
)

// A pointer not under <main>/.git/worktrees (a submodule) keeps the holding dir.
func worktreeMain(pointerPath string, holdingDir string) string {
	raw, err := os.ReadFile(pointerPath)
	if err != nil {
		return holdingDir
	}
	gitDir, isPointer := strings.CutPrefix(strings.TrimSpace(string(raw)), gitDirPrefix)
	if !isPointer {
		return holdingDir
	}
	main, _, isWorktree := strings.Cut(strings.TrimSpace(gitDir), gitWorktreesInfix)
	if !isWorktree {
		return holdingDir
	}
	return main
}

// Outside git it returns cwd.
func CheckoutRoot(cwd string) string {
	current := filepath.Clean(cwd)
	for {
		if _, err := os.Stat(filepath.Join(current, gitEntry)); err == nil {
			return current
		}

		parent := filepath.Dir(current)
		if parent == current {
			return cwd
		}
		current = parent
	}
}

// Outside git it returns cwd.
func MainRoot(cwd string) string {
	root := CheckoutRoot(cwd)
	entry := filepath.Join(root, gitEntry)
	info, err := os.Stat(entry)
	switch {
	case err != nil:
		return cwd
	case info.IsDir():
		return root
	}
	return worktreeMain(entry, root)
}

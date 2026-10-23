package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// A routine runner exports the colon-separated checkouts no session of the run writes into.
const ProtectedPathsKey = "ROUTINE_PROTECTED_PATHS"

func ClaudeHomeDir(override string) (string, error) {
	if override != "" {
		return override, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("ClaudeHomeDir: Failed to resolve home: %w", err)
	}
	return filepath.Join(home, ".claude"), nil
}

// A worktree below a protected checkout is not protected; poolSegment marks one; paths compare as text, as the harness sends them.
func ProtectedRoot(path string, protectedPaths string, poolSegment string) string {
	for root := range strings.SplitSeq(protectedPaths, ":") {
		if root == "" || strings.HasPrefix(path, root+poolSegment) {
			continue
		}

		if path == root || strings.HasPrefix(path, root+"/") {
			return root
		}
	}
	return ""
}

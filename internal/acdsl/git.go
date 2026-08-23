package acdsl

import (
	"context"
	"fmt"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

// shellGitGrepCached greps the index (staged content) for a fixed string.
// git grep exits 1 on no matches — callers treat that via isExitOne.
// -I skips binaries: a projection block is text, and a staged compiled
// artifact (e.g. a distributed bin/acdsl, which embeds the marker literal)
// must never trip the leak guard.
func shellGitGrepCached(ctx context.Context, root, literal string) (string, error) {
	// -e guards literals that begin with a dash (the sql comment shape).
	return shell.Run(ctx, root, "git", "grep", "--cached", "-InF", "-e", literal)
}

// GitBranch returns the current branch name — the session proxy in verdict
// records (pool sessions run on claude/<slug> branches).
func GitBranch(ctx context.Context, root string) (string, error) {
	output, err := shell.Run(ctx, root, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("GitBranch: %w", err)
	}
	return strings.TrimSpace(output), nil
}

package acdsl

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

// -I skips binaries: a staged bin/acdsl embeds the marker literal and must not trip the leak guard.
func shellGitGrepCached(ctx context.Context, root, literal string) (string, error) {
	// -e guards literals that begin with a dash (the sql comment shape).
	return shell.Run(ctx, root, "git", "grep", "--cached", "-InF", "-e", literal)
}

func shellGitGrepTree(ctx context.Context, root, literal, rev string, excludes []string) (string, error) {
	args := []string{"grep", "-InF", "-e", literal, rev, "--", "."}
	for _, exclude := range excludes {
		args = append(args, ":(exclude)"+exclude)
	}
	return shell.Run(ctx, root, "git", args...)
}

func shellGitAbbrevRef(ctx context.Context, root, ref string) (string, error) {
	output, err := shell.Run(ctx, root, "git", "rev-parse", "--abbrev-ref", ref)
	if err != nil {
		return "", fmt.Errorf("shellGitAbbrevRef: %s: %w", ref, err)
	}
	return strings.TrimSpace(output), nil
}

// The branch is the session proxy in verdict records: pool sessions run on claude/<slug> branches.
func GitBranch(ctx context.Context, root string) (string, error) {
	output, err := shell.Run(ctx, root, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("GitBranch: %w", err)
	}
	return strings.TrimSpace(output), nil
}

// Covers committed, staged and unstaged changes plus untracked unignored files.
func GitChangedFiles(ctx context.Context, root, base string) ([]string, error) {
	diffed, err := shell.Run(ctx, root, "git", "diff", "--name-only", base)
	if err != nil {
		return nil, fmt.Errorf("GitChangedFiles: %w", err)
	}
	untracked, err := shell.Run(ctx, root, "git", "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, fmt.Errorf("GitChangedFiles: %w", err)
	}
	seen := map[string]bool{}
	var files []string
	for _, path := range strings.Fields(diffed + "\n" + untracked) {
		if seen[path] {
			continue
		}
		seen[path] = true
		files = append(files, path)
	}
	sort.Strings(files)
	return files, nil
}

// GitMergeBase returns merge-base(HEAD, ref).
func GitMergeBase(ctx context.Context, root, ref string) (string, error) {
	output, err := shell.Run(ctx, root, "git", "merge-base", "HEAD", ref)
	if err != nil {
		return "", fmt.Errorf("GitMergeBase: %s: %w", ref, err)
	}
	return strings.TrimSpace(output), nil
}

// GitRevParse resolves ref to a commit sha.
func GitRevParse(ctx context.Context, root, ref string) (string, error) {
	output, err := shell.Run(ctx, root, "git", "rev-parse", "--verify", "--quiet", ref)
	if err != nil {
		return "", fmt.Errorf("GitRevParse: %s: %w", ref, err)
	}
	return strings.TrimSpace(output), nil
}

// Callers pass a validated rev, so an exit failure means the path is absent: ok=false, no error.
func GitShowFile(ctx context.Context, root, rev, path string) (string, bool, error) {
	output, err := shell.Run(ctx, root, "git", "show", rev+":"+path)
	if err == nil {
		return output, true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return "", false, nil
	}
	return "", false, fmt.Errorf("GitShowFile: %s:%s: %w", rev, path, err)
}

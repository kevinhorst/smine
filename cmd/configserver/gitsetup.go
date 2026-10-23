package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/shell"
)

const (
	// hookMarker is the purpose line every version of our pre-commit hook carries; a hook without it is somebody else's.
	hookMarker = "ACDSL projection blocks"
	// hookPerm is the mode git needs to run a hook.
	hookPerm = 0o755
	// initialCommitMessage is the one commit a fresh install dir gets.
	initialCommitMessage = "smine initial install"
	// mergeDriverName is the merge.skillcue.name git shows in conflict messages.
	mergeDriverName = "skill model changelog merge"
	// mergeDriverPlaceholders are git's driver placeholders: base, ours, theirs, path, ours label, theirs label.
	mergeDriverPlaceholders = "%O %A %B %P %X %Y"
)

// ensureRepo makes the install dir a standalone repository: a fresh dir gets init on main and one commit; an existing .git, dir or worktree gitlink, is never touched and no remote is configured.
func ensureRepo(ctx context.Context, env install.Env) error {
	if _, err := os.Lstat(filepath.Join(env.RepoDir, ".git")); err == nil {
		fmt.Printf("skip: git repository exists in %s\n", env.RepoDir)
		return nil
	}

	fmt.Printf("-> Initializing standalone git repository in %s ...\n", env.RepoDir)
	if _, err := shell.Run(ctx, env.RepoDir, "git", "init", "-b", "main"); err != nil {
		// -b needs git >= 2.28; older gits get init + symbolic-ref
		if output, err := shell.Run(ctx, env.RepoDir, "git", "init"); err != nil {
			return fmt.Errorf("ensureRepo: git init: %w\n%s", err, output)
		}
		if output, err := shell.Run(ctx, env.RepoDir, "git", "symbolic-ref", "HEAD", "refs/heads/main"); err != nil {
			return fmt.Errorf("ensureRepo: git symbolic-ref: %w\n%s", err, output)
		}
	}

	commit := []string{"commit", "-m", initialCommitMessage}
	if _, err := shell.Run(ctx, env.RepoDir, "git", "config", "user.email"); err != nil {
		commit = append([]string{"-c", "user.name=smine", "-c", "user.email=smine@localhost"}, commit...)
	}
	if output, err := shell.RunSync(ctx, env.RepoDir, "git", "add", "-A"); err != nil {
		return fmt.Errorf("ensureRepo: git add: %w\n%s", err, output)
	}
	if output, err := shell.RunSync(ctx, env.RepoDir, "git", commit...); err != nil {
		return fmt.Errorf("ensureRepo: git commit: %w\n%s", err, output)
	}
	fmt.Println("   initialized on branch main (no remote — add your own to push)")
	return nil
}

// installHooks copies the pre-commit hook into the common git dir, shared by the main checkout and every worktree; a foreign pre-commit is refused, never chained.
func installHooks(ctx context.Context, env install.Env) error {
	hook, err := os.ReadFile(filepath.Join(env.RepoDir, "cmd", "sync", "hooks", "pre-commit"))
	if err != nil {
		return fmt.Errorf("installHooks: %w", err)
	}

	commonDir, err := shell.Run(ctx, env.RepoDir, "git", "rev-parse", "--git-common-dir")
	if err != nil {
		return fmt.Errorf("installHooks: git rev-parse: %w\n%s", err, commonDir)
	}
	commonDir = strings.TrimSpace(commonDir)
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(env.RepoDir, commonDir)
	}
	target := filepath.Join(commonDir, "hooks", "pre-commit")

	if existing, err := os.ReadFile(target); err == nil && !bytes.Contains(existing, []byte(hookMarker)) {
		return fmt.Errorf("installHooks: a foreign pre-commit hook exists at %s — not overwriting", target)
	}
	if err := os.MkdirAll(filepath.Dir(target), hookPerm); err != nil {
		return fmt.Errorf("installHooks: %w", err)
	}
	if err := fsx.WriteFileAtomic(hook, target, hookPerm); err != nil {
		return fmt.Errorf("installHooks: %w", err)
	}
	fmt.Printf("  pre-commit -> %s\n", target)
	return nil
}

// mergeDriverCommand is the merge.skillcue.driver value: the installed rules binary by absolute path, forward slashes on every OS.
func mergeDriverCommand(env install.Env) string {
	suffix := ""
	if env.GOOS == "windows" {
		suffix = ".exe"
	}
	binary := filepath.Join(config.SmineBinDir(env.GOOS, env.Home), "smine-rules"+suffix)
	return filepath.ToSlash(binary) + " merge-skillcue " + mergeDriverPlaceholders
}

// setupGit is the git half every install shares: the repository itself, the bare-repo guard, the skill model merge driver and the projection pre-commit hook.
func setupGit(ctx context.Context, env install.Env) error {
	if err := ensureRepo(ctx, env); err != nil {
		return fmt.Errorf("setupGit: %w", err)
	}

	settings := [][2]string{
		{"safe.bareRepository", "explicit"},
		{"merge.skillcue.name", mergeDriverName},
		{"merge.skillcue.driver", mergeDriverCommand(env)},
	}
	for _, setting := range settings {
		fmt.Printf("-> Setting git %s ...\n", setting[0])
		if output, err := shell.Run(ctx, env.RepoDir, "git", "config", "--global", setting[0], setting[1]); err != nil {
			return fmt.Errorf("setupGit: git config %s: %w\n%s", setting[0], err, output)
		}
	}

	fmt.Println("-> Installing git hooks ...")
	if err := installHooks(ctx, env); err != nil {
		return fmt.Errorf("setupGit: %w", err)
	}
	return nil
}

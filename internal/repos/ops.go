package repos

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

const (
	cherryPickScript = "cherry_pick_worktree.sh"
	mergeScript      = "merge_worktree.sh"
	pruneScript      = "prune_jetbrains_recent_projects.sh"
)

var commitPattern = regexp.MustCompile(`^[0-9a-f]{4,40}$`)

// CherryPick runs cherry_pick_worktree.sh (§25): clean-checkout check,
// cherry-pick onto the main checkout HEAD, abort + CONFLICT line on conflict —
// all inside the script (D2, D5).
func CherryPick(ctx context.Context, commit, repoPath, scriptsDir string) (string, error) {
	if err := ValidateCommit(ctx, commit, repoPath); err != nil {
		return "", err
	}

	script := filepath.Join(scriptsDir, cherryPickScript)
	output, err := shell.Run(ctx, repoPath, script, commit)
	if err != nil {
		return output, fmt.Errorf("CherryPick: %s: %w", commit, err)
	}
	return output, nil
}

// Merge runs merge_worktree.sh (§25): clean-checkout check, merge of the
// worktree branch into the main checkout HEAD, abort + CONFLICT line on
// conflict — all inside the script (D2, D5).
func Merge(ctx context.Context, branch, repoPath, scriptsDir string) (string, error) {
	if err := ValidateBranch(ctx, branch, repoPath); err != nil {
		return "", err
	}

	script := filepath.Join(scriptsDir, mergeScript)
	output, err := shell.Run(ctx, repoPath, script, branch)
	if err != nil {
		return output, fmt.Errorf("Merge: %s: %w", branch, err)
	}
	return output, nil
}

// PruneJetbrains runs the global JetBrains recent-projects prune (D30). Flags
// are composed server-side from the two form checkboxes — request input never
// reaches the argv.
func PruneJetbrains(ctx context.Context, dryRun, force bool, scriptsDir string) (string, error) {
	var args []string
	if dryRun {
		args = append(args, "--dry-run")
	}
	if force {
		args = append(args, "--force")
	}

	script := filepath.Join(scriptsDir, pruneScript)
	output, err := shell.Run(ctx, "", script, args...)
	if err != nil {
		return output, fmt.Errorf("PruneJetbrains: %w", err)
	}
	return output, nil
}

// Remove runs remove_agent_worktrees.sh targeted at one branch (§23); the
// script's own safety checks decide removed vs. skipped. With deleteBranch
// the branch itself is deleted too, under the same safety rules.
func Remove(ctx context.Context, branch string, force, deleteBranch bool, repoPath, scriptsDir string) (string, error) {
	if err := ValidateBranch(ctx, branch, repoPath); err != nil {
		return "", err
	}

	script := filepath.Join(scriptsDir, removeScript)
	var args []string
	if deleteBranch {
		args = append(args, "--delete-branch")
	}
	if force {
		args = append(args, "--force")
	}
	args = append(args, branch)

	output, err := shell.Run(ctx, repoPath, script, args...)
	if err != nil {
		return output, fmt.Errorf("Remove: %s: %w", branch, err)
	}
	return output, nil
}

// Sync runs sync_worktrees.sh targeted at one branch (§22) — merge the
// feature branch in, skip dirty, abort on conflict, all inside the script.
func Sync(ctx context.Context, branch, repoPath, scriptsDir string) (string, error) {
	if err := ValidateBranch(ctx, branch, repoPath); err != nil {
		return "", err
	}

	script := filepath.Join(scriptsDir, syncScript)
	output, err := shell.Run(ctx, repoPath, script, branch)
	if err != nil {
		return output, fmt.Errorf("Sync: %s: %w", branch, err)
	}
	return output, nil
}

// ValidateBranch guards every operation on a branch: agent namespaces enforced,
// ref existence verified — request input never reaches an argv unverified
// (hot item H2).
func ValidateBranch(ctx context.Context, branch, repoPath string) error {
	// Branch: agent namespaces only (session pool + routine lineages)
	if !strings.HasPrefix(branch, "claude/") && !strings.HasPrefix(branch, "claude-routines/") {
		return fmt.Errorf("ValidateBranch: Not an agent branch: %q", branch)
	}

	// Branch: must exist as a local ref in this repo
	if _, err := shell.Run(ctx, repoPath, "git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err != nil {
		return fmt.Errorf("ValidateBranch: Unknown branch %q: %w", branch, err)
	}

	return nil
}

// ValidateCommit guards cherry-pick input (hot item H2).
func ValidateCommit(ctx context.Context, commit, repoPath string) error {
	// Commit: sha shape before it reaches an argv
	if !commitPattern.MatchString(commit) {
		return fmt.Errorf("ValidateCommit: Invalid commit sha: %q", commit)
	}

	// Commit: must resolve to a commit object in this repo
	if _, err := shell.Run(ctx, repoPath, "git", "rev-parse", "--verify", "--quiet", commit+"^{commit}"); err != nil {
		return fmt.Errorf("ValidateCommit: Unknown commit %q: %w", commit, err)
	}

	return nil
}

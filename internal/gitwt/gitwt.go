// Package gitwt provisions review worktrees and archives every byte before an unforced removal.
package gitwt

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

const (
	branchLinePrefix   = "branch refs/heads/"
	commitsDiffFile    = "commits.diff"
	headLinePrefix     = "HEAD "
	ignoredPrefix      = "!! "
	trackedDiffFile    = "tracked.diff"
	untrackedPrefix    = "?? "
	worktreeLinePrefix = "worktree "
)

// An empty Prefix matches no branch; each branch's diff against Base is saved into Dest first.
type BranchRequest struct {
	Base   string
	Dest   string
	Names  []string
	Prefix string
	Repo   string
}

// BranchResult lists the deleted branches and the refusals.
type BranchResult struct {
	Deleted []string `json:"deleted"`
	Skipped []string `json:"skipped"`
}

type ProvisionRequest struct {
	Branch   string
	Head     string
	Overlay  string
	Prebuilt []string
	Repo     string
	Root     string
	Slug     string
}

// Provisioned is one created worktree.
type Provisioned struct {
	Branch string `json:"branch"`
	Path   string `json:"path"`
}

// The Overlay is reversed first when set; commits since Base are archived when Base is set.
type SalvageRequest struct {
	Base string
	Dest string
	// IsArchiveOnly archives commits, tracked changes and untracked files but neither checks out nor removes: the caller removes with force afterwards.
	IsArchiveOnly bool
	Overlay       string
	Repo          string
	Worktree      string
}

// Reason carries git's refusal of the unforced removal.
type SalvageResult struct {
	CommitsDiff     bool   `json:"commitsDiff"`
	Moved           int    `json:"moved"`
	OverlayReversed bool   `json:"overlayReversed"`
	Path            string `json:"path"`
	Reason          string `json:"reason,omitempty"`
	Removed         bool   `json:"removed"`
	TrackedDiff     bool   `json:"trackedDiff"`
}

// Worktree is one entry of git worktree list --porcelain.
type Worktree struct {
	Branch string `json:"branch"`
	Head   string `json:"head"`
	Path   string `json:"path"`
}

// archiveCommits saves the worktree's commits since Base as commits.diff.
func archiveCommits(ctx context.Context, request *SalvageRequest, result *SalvageResult) error {
	if request.Base == "" {
		return nil
	}

	diff, err := shell.Run(ctx, request.Worktree, "git", "diff", "--binary", request.Base, "HEAD")
	if err != nil {
		return fmt.Errorf("archiveCommits: %w", err)
	}

	written, err := WriteIfNonEmpty(diff, filepath.Join(request.Dest, commitsDiffFile))
	if err != nil {
		return err
	}

	result.CommitsDiff = written
	return nil
}

// Unstages afterwards so new files count as untracked.
func archiveTracked(ctx context.Context, request *SalvageRequest, result *SalvageResult) error {
	diff, err := shell.Run(ctx, request.Worktree, "git", "diff", "--binary", "HEAD")
	if err != nil {
		return fmt.Errorf("archiveTracked: %w", err)
	}

	written, err := WriteIfNonEmpty(diff, filepath.Join(request.Dest, trackedDiffFile))
	if err != nil {
		return err
	}

	result.TrackedDiff = written
	if _, err := shell.Run(ctx, request.Worktree, "git", "reset", "-q"); err != nil {
		return fmt.Errorf("archiveTracked: %w", err)
	}

	return nil
}

func branchTargets(ctx context.Context, request *BranchRequest) ([]string, error) {
	targets := make([]string, 0, len(request.Names))
	if request.Prefix != "" {
		prefixed, err := Branches(ctx, request.Prefix, request.Repo)
		if err != nil {
			return nil, err
		}

		targets = append(targets, prefixed...)
	}

	for _, name := range request.Names {
		if !slices.Contains(targets, name) {
			targets = append(targets, name)
		}
	}
	return targets, nil
}

// Runs before any worktree exists, so a refusal leaves nothing behind.
func checkPrebuilt(request *ProvisionRequest) error {
	for _, relative := range request.Prebuilt {
		if !filepath.IsLocal(relative) {
			return fmt.Errorf("checkPrebuilt: Prebuilt path %s is not inside the repo", relative)
		}

		if _, err := os.Stat(filepath.Join(request.Repo, relative)); err != nil {
			return fmt.Errorf("checkPrebuilt: Prebuilt path %s is missing in %s: %w", relative, request.Repo, err)
		}
	}
	return nil
}

// isGone reports whether nothing exists at path any more.
func isGone(path string) bool {
	_, err := os.Lstat(path)
	return errors.Is(err, os.ErrNotExist)
}

// Salvage later moves a link, never the files behind it.
func linkPrebuilt(path string, request *ProvisionRequest) error {
	repo, err := filepath.Abs(request.Repo)
	if err != nil {
		return fmt.Errorf("linkPrebuilt: %w", err)
	}

	for _, relative := range request.Prebuilt {
		link := filepath.Join(path, relative)
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			return fmt.Errorf("linkPrebuilt: %w", err)
		}

		if err := os.Symlink(filepath.Join(repo, relative), link); err != nil {
			return fmt.Errorf("linkPrebuilt: %w", err)
		}
	}
	return nil
}

// Ignored bytes too, since the unforced removal would delete them; a vanished path is skipped.
func moveUntracked(ctx context.Context, request *SalvageRequest, result *SalvageResult) error {
	status, err := shell.Run(ctx, request.Worktree, "git", "status", "--porcelain", "-z", "--ignored", "--untracked-files=all")
	if err != nil {
		return fmt.Errorf("moveUntracked: %w", err)
	}

	for entry := range strings.SplitSeq(status, "\x00") {
		relative, isUntracked := strings.CutPrefix(entry, untrackedPrefix)
		if !isUntracked {
			relative, isUntracked = strings.CutPrefix(entry, ignoredPrefix)
		}
		if !isUntracked {
			continue
		}

		target := filepath.Join(request.Dest, strings.TrimSuffix(relative, "/"))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("moveUntracked: %w", err)
		}

		source := filepath.Join(request.Worktree, relative)
		err := os.Rename(source, target)
		if err != nil && isGone(source) {
			continue
		}

		if err != nil {
			return fmt.Errorf("moveUntracked: %w", err)
		}

		result.Moved++
	}
	return nil
}

// A matching registered worktree counts as provisioned so a re-spawned worker's first command succeeds.
func reuseWorktree(ctx context.Context, path string, request *ProvisionRequest) (*Provisioned, error) {
	head, err := shell.Run(ctx, request.Repo, "git", "rev-parse", "--verify", request.Head+"^{commit}")
	if err != nil {
		return nil, fmt.Errorf("reuseWorktree: %s: %w", strings.TrimSpace(head), err)
	}

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, fmt.Errorf("reuseWorktree: %w", err)
	}

	worktrees, err := List(ctx, request.Repo)
	if err != nil {
		return nil, err
	}

	for _, worktree := range worktrees {
		isSameCheckout := worktree.Path == resolved && worktree.Branch == request.Branch
		if isSameCheckout && worktree.Head == strings.TrimSpace(head) {
			provisioned := &Provisioned{Branch: request.Branch, Path: path}
			return provisioned, nil
		}
	}
	return nil, fmt.Errorf("Provision: Path %s already exists and is not the worktree of %s at %s", path, request.Branch, request.Head)
}

// The checkout is pristine, so the unforced removal loses no byte.
func rollback(ctx context.Context, branch, path, repo string) error {
	if _, err := shell.Run(ctx, repo, "git", "worktree", "remove", path); err != nil {
		return fmt.Errorf("rollback: %w", err)
	}

	if _, err := shell.Run(ctx, repo, "git", "branch", "-D", branch); err != nil {
		return fmt.Errorf("rollback: %w", err)
	}

	return nil
}

// ArchiveBranch writes branch's diff against base into dest as <branch with / as _>.diff and returns branch's sha; an empty diff writes nothing.
func ArchiveBranch(ctx context.Context, base, branch, dest, repo string) (string, error) {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return "", fmt.Errorf("ArchiveBranch: %w", err)
	}

	sha, err := shell.Run(ctx, repo, "git", "rev-parse", "--verify", branch+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("ArchiveBranch: %s: %w", branch, err)
	}

	diff, err := shell.Run(ctx, repo, "git", "diff", "--binary", base, branch)
	if err != nil {
		return "", fmt.Errorf("ArchiveBranch: %s: %w", branch, err)
	}

	diffPath := filepath.Join(dest, strings.ReplaceAll(branch, "/", "_")+".diff")
	if _, err := WriteIfNonEmpty(diff, diffPath); err != nil {
		return "", err
	}
	return strings.TrimSpace(sha), nil
}

// Branches lists the repo's local branches under prefix, sorted by name.
func Branches(ctx context.Context, prefix, repo string) ([]string, error) {
	listing, err := shell.Run(ctx, repo, "git", "for-each-ref", "--format=%(refname:short)", "refs/heads/"+prefix+"*")
	if err != nil {
		return nil, fmt.Errorf("Branches: %w", err)
	}
	return strings.Fields(listing), nil
}

// List returns the repo's worktrees from git worktree list --porcelain.
func List(ctx context.Context, repo string) ([]Worktree, error) {
	listing, err := shell.Run(ctx, repo, "git", "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("List: %w", err)
	}

	worktrees := make([]Worktree, 0)
	for block := range strings.SplitSeq(strings.TrimSpace(listing), "\n\n") {
		worktree := Worktree{}
		for line := range strings.SplitSeq(block, "\n") {
			if path, isPath := strings.CutPrefix(line, worktreeLinePrefix); isPath {
				worktree.Path = path
			}
			if head, isHead := strings.CutPrefix(line, headLinePrefix); isHead {
				worktree.Head = head
			}
			if branch, isBranch := strings.CutPrefix(line, branchLinePrefix); isBranch {
				worktree.Branch = branch
			}
		}
		if worktree.Path != "" {
			worktrees = append(worktrees, worktree)
		}
	}
	return worktrees, nil
}

// A failed overlay apply removes the worktree again: a worker never reviews without the overlay.
func Provision(ctx context.Context, request *ProvisionRequest) (*Provisioned, error) {
	path := filepath.Join(request.Root, request.Slug)
	if _, err := os.Stat(path); err == nil {
		return reuseWorktree(ctx, path, request)
	}

	if err := checkPrebuilt(request); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(request.Root, 0o755); err != nil {
		return nil, fmt.Errorf("Provision: %w", err)
	}

	output, err := shell.Run(ctx, request.Repo, "git", "worktree", "add", "-b", request.Branch, path, request.Head)
	if err != nil {
		return nil, fmt.Errorf("Provision: %s: %w", strings.TrimSpace(output), err)
	}

	provisioned := &Provisioned{Branch: request.Branch, Path: path}
	if request.Overlay != "" {
		output, err = shell.Run(ctx, path, "git", "apply", request.Overlay)
		if err != nil {
			applyErr := fmt.Errorf("Provision: Overlay apply failed: %s: %w", strings.TrimSpace(output), err)
			rollbackErr := rollback(ctx, request.Branch, path, request.Repo)
			return nil, errors.Join(applyErr, rollbackErr)
		}
	}

	if err := linkPrebuilt(path, request); err != nil {
		return nil, err
	}
	return provisioned, nil
}

// A refused deletion is reported and the next branch is still processed.
func RemoveBranches(ctx context.Context, request *BranchRequest) (*BranchResult, error) {
	targets, err := branchTargets(ctx, request)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(request.Dest, 0o755); err != nil {
		return nil, fmt.Errorf("RemoveBranches: %w", err)
	}

	result := &BranchResult{Deleted: []string{}, Skipped: []string{}}
	for _, branch := range targets {
		if _, err := ArchiveBranch(ctx, request.Base, branch, request.Dest, request.Repo); err != nil {
			return nil, fmt.Errorf("RemoveBranches: %w", err)
		}

		output, err := shell.Run(ctx, request.Repo, "git", "branch", "-D", branch)
		if err != nil {
			result.Skipped = append(result.Skipped, branch+": "+strings.TrimSpace(output))
			continue
		}

		result.Deleted = append(result.Deleted, branch)
	}
	return result, nil
}

// A refused removal is a reported result; errors are harness failures.
func Salvage(ctx context.Context, request *SalvageRequest) (*SalvageResult, error) {
	result := &SalvageResult{OverlayReversed: request.Overlay == "", Path: request.Worktree}
	if err := os.MkdirAll(request.Dest, 0o755); err != nil {
		return nil, fmt.Errorf("Salvage: %w", err)
	}

	if request.Overlay != "" {
		_, err := shell.Run(ctx, request.Worktree, "git", "apply", "-R", request.Overlay)
		result.OverlayReversed = err == nil
	}

	if err := archiveCommits(ctx, request, result); err != nil {
		return nil, err
	}

	if err := archiveTracked(ctx, request, result); err != nil {
		return nil, err
	}

	if err := moveUntracked(ctx, request, result); err != nil {
		return nil, err
	}

	if request.IsArchiveOnly {
		return result, nil
	}

	if _, err := shell.Run(ctx, request.Worktree, "git", "checkout", "-q", "--", "."); err != nil {
		return nil, fmt.Errorf("Salvage: %w", err)
	}

	output, err := shell.Run(ctx, request.Repo, "git", "worktree", "remove", request.Worktree)
	result.Removed = err == nil
	if err != nil {
		result.Reason = strings.TrimSpace(output)
	}

	return result, nil
}

// WriteIfNonEmpty writes content to path unless it is blank.
func WriteIfNonEmpty(content, path string) (bool, error) {
	if strings.TrimSpace(content) == "" {
		return false, nil
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return false, fmt.Errorf("WriteIfNonEmpty: %w", err)
	}

	return true, nil
}

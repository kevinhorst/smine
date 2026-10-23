package skilleval

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/kevinhorst/smine/internal/gitwt"
	"github.com/kevinhorst/smine/internal/shell"
	"github.com/kevinhorst/smine/internal/trash"
)

const (
	armBranchPrefix    = "claude-review/run-"
	salvageDir         = "salvage"
	workerBranchPrefix = "claude-review/rr-"

	// harness isolation worktrees: claude/<name> from the worktree hook, worktree-<name> from the harness
	harnessBranchPrefix    = "worktree-"
	harnessWorktreesDir    = "worktrees"
	harnessWorktreesParent = ".claude"
	hookBranchPrefix       = "claude/"
)

// CleanupReport is what one arm cleanup salvaged and removed.
type CleanupReport struct {
	Branches  *gitwt.BranchResult    `json:"branches"`
	Stash     string                 `json:"stash,omitempty"`
	Worktrees []*gitwt.SalvageResult `json:"worktrees"`
}

// isEveryWorktreeRemoved reports whether git removed every salvaged worktree.
func (r *CleanupReport) isEveryWorktreeRemoved() bool {
	for _, worktree := range r.Worktrees {
		if !worktree.Removed {
			return false
		}
	}
	return true
}

// IsAllRemoved reports whether every worktree and branch went.
func (r *CleanupReport) IsAllRemoved() bool {
	return r.isEveryWorktreeRemoved() && len(r.Branches.Skipped) == 0
}

type armScope struct {
	armWorktree  string
	dest         string
	workerRoot   string
	workflowRuns []string
}

// isWorkflowWorktree relies on the harness naming isolation worktrees <run id>-….
func (s *armScope) isWorkflowWorktree(path string) bool {
	parent := filepath.Dir(path)
	isHarnessDir := filepath.Base(parent) == harnessWorktreesDir && filepath.Base(filepath.Dir(parent)) == harnessWorktreesParent
	if !isHarnessDir {
		return false
	}

	name := filepath.Base(path)
	return slices.ContainsFunc(s.workflowRuns, func(runId string) bool {
		return strings.HasPrefix(name, runId+"-")
	})
}

func (s *armScope) salvageDest(path string) string {
	switch {
	case path == s.armWorktree:
		return filepath.Join(s.dest, "arm-wt")
	case strings.HasPrefix(path, s.workerRoot+string(filepath.Separator)):
		return filepath.Join(s.dest, "wt", filepath.Base(path))
	case s.isWorkflowWorktree(path):
		return filepath.Join(s.dest, "wf", filepath.Base(path))
	}
	return ""
}

func workflowBranch(branch, path string) string {
	name := filepath.Base(path)
	isCreatedForIt := branch == hookBranchPrefix+name || branch == harnessBranchPrefix+name
	if !isCreatedForIt {
		return ""
	}
	return branch
}

func workflowRuns(label, runDir string) ([]string, error) {
	stream, err := parseStreamFile(filepath.Join(runDir, runsDir, label, streamFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}
	return stream.WorkflowRuns, nil
}

// CleanupArm salvages into <worktree>-out/salvage/, never the run record, keeps -out when git refused a removal and moves it to the trash otherwise.
func CleanupArm(ctx context.Context, label, runDir string, store *trash.Store) (*CleanupReport, error) {
	manifest, err := loadManifest(runDir)
	if err != nil {
		return nil, err
	}

	arm, err := manifest.armByLabel(label)
	if err != nil {
		return nil, err
	}

	runs, err := workflowRuns(arm.Label, runDir)
	if err != nil {
		return nil, err
	}

	outDir := arm.Worktree + outDirSuffix
	scope := &armScope{
		armWorktree:  arm.Worktree,
		dest:         filepath.Join(outDir, salvageDir),
		workerRoot:   filepath.Join(outDir, "wt"),
		workflowRuns: runs,
	}
	worktrees, err := gitwt.List(ctx, manifest.Repo)
	if err != nil {
		return nil, err
	}

	// Worker and workflow worktrees first, the arm worktree last — only what git still lists.
	sort.SliceStable(worktrees, func(i, j int) bool { return worktrees[j].Path == arm.Worktree })
	report := &CleanupReport{Worktrees: []*gitwt.SalvageResult{}}
	workflowBranches := make([]string, 0)
	for _, worktree := range worktrees {
		target := scope.salvageDest(worktree.Path)
		if target == "" {
			continue
		}

		request := &gitwt.SalvageRequest{Base: manifest.At, Dest: target, Overlay: manifest.OverlayPatch, Repo: manifest.Repo, Worktree: worktree.Path}
		if scope.isWorkflowWorktree(worktree.Path) {
			// A workflow worktree is cut from the arm's commit: it never held the overlay.
			request.Overlay = ""
			workflowBranches = append(workflowBranches, workflowBranch(worktree.Branch, worktree.Path))
		}

		salvaged, err := gitwt.Salvage(ctx, request)
		if err != nil {
			return nil, err
		}
		report.Worktrees = append(report.Worktrees, salvaged)
	}

	if _, err := shell.Run(ctx, manifest.Repo, "git", "worktree", "prune"); err != nil {
		return nil, fmt.Errorf("CleanupArm: %w", err)
	}

	runId := armRunId(arm.Label, runDir)
	branchDest := filepath.Join(scope.dest, "branches")
	workflowBranches = slices.DeleteFunc(workflowBranches, func(branch string) bool { return branch == "" })
	requests := []*gitwt.BranchRequest{
		{Base: manifest.At, Dest: branchDest, Prefix: armBranchPrefix + runId, Repo: manifest.Repo},
		{Base: manifest.At, Dest: branchDest, Prefix: workerBranchPrefix + runId + "-", Repo: manifest.Repo},
		{Base: manifest.At, Dest: branchDest, Names: workflowBranches, Repo: manifest.Repo},
	}
	report.Branches = &gitwt.BranchResult{Deleted: []string{}, Skipped: []string{}}
	for _, branchRequest := range requests {
		branches, err := gitwt.RemoveBranches(ctx, branchRequest)
		if err != nil {
			return nil, err
		}
		report.Branches.Deleted = append(report.Branches.Deleted, branches.Deleted...)
		report.Branches.Skipped = append(report.Branches.Skipped, branches.Skipped...)
	}

	if !report.isEveryWorktreeRemoved() {
		return report, nil
	}

	entry := &trash.Entry{Action: "eval cleanup " + label, Reason: "arm out dir with its salvage"}
	stash, err := store.Move(outDir, entry)
	if err != nil {
		return nil, fmt.Errorf("CleanupArm: %w", err)
	}

	report.Stash = stash
	return report, nil
}

package reviewscope

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/cmdline"
	"github.com/kevinhorst/smine/internal/gitwt"
	"github.com/kevinhorst/smine/internal/routinerun"
)

const (
	buildTestTimeout = 20 * time.Minute
	decisiveLines    = 30
	shortShaRunes    = 7
	stepWaived       = "skipped: waived by the invocation"
)

type BuildTestRequest struct {
	Branch   string
	Build    string
	Head     string
	IsWaived bool
	Overlay  string
	Repo     string
	Root     string
	Salvage  string
	Test     string
}

type BuildTestResult struct {
	Branches *gitwt.BranchResult  `json:"branches"`
	Build    string               `json:"build"`
	Salvage  *gitwt.SalvageResult `json:"salvage"`
	Tests    string               `json:"tests"`
}

// commandLines splits a flag value into command lines; an empty value names the Environment page.
func commandLines(name, value string) ([][]string, error) {
	if value == "" {
		return nil, fmt.Errorf("commandLines: No %s command — the repo's build and test commands are set on the skill page", name)
	}

	lines, err := cmdline.SplitLines(value)
	if err != nil {
		return nil, fmt.Errorf("commandLines: %s: %w", name, err)
	}
	return lines, nil
}

// lastLines keeps the final count lines of a step's output.
func lastLines(count int, text string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	return strings.Join(lines[max(0, len(lines)-count):], "\n")
}

// runLines runs one command line after the other in tree; the first red step is the result.
func runLines(ctx context.Context, lines [][]string, tree string) string {
	outcome := "ok"
	for _, argv := range lines {
		outcome = runStep(ctx, argv, tree)
		if outcome != "ok" {
			return outcome
		}
	}
	return outcome
}

func runStep(ctx context.Context, argv []string, tree string) string {
	var output bytes.Buffer
	spec := &routinerun.CellSpec{Args: argv, Dir: tree, Stderr: &output, Stdout: &output, Timeout: buildTestTimeout}
	status, err := routinerun.RunCell(ctx, spec)
	if err != nil {
		return "skipped: " + err.Error()
	}

	if status == 0 {
		return "ok"
	}
	return "failed: " + lastLines(decisiveLines, output.String())
}

// RunBuildTest answers a waived request in the same result shape, so a caller reads one form.
func RunBuildTest(ctx context.Context, request *BuildTestRequest) (*BuildTestResult, error) {
	if request.IsWaived {
		waived := &BuildTestResult{Build: stepWaived, Tests: stepWaived}
		return waived, nil
	}

	build, err := commandLines("build", request.Build)
	if err != nil {
		return nil, fmt.Errorf("RunBuildTest: %w", err)
	}

	test, err := commandLines("test", request.Test)
	if err != nil {
		return nil, fmt.Errorf("RunBuildTest: %w", err)
	}

	slug := "buildtest-" + request.Head[:min(shortShaRunes, len(request.Head))]
	provisionRequest := &gitwt.ProvisionRequest{Branch: request.Branch, Head: request.Head, Overlay: request.Overlay, Repo: request.Repo, Root: request.Root, Slug: slug}
	provisioned, err := gitwt.Provision(ctx, provisionRequest)
	if err != nil {
		return nil, err
	}

	result := &BuildTestResult{Build: runLines(ctx, build, provisioned.Path)}
	result.Tests = runLines(ctx, test, provisioned.Path)
	salvageRequest := &gitwt.SalvageRequest{Base: request.Head, Dest: filepath.Join(request.Salvage, slug), Overlay: request.Overlay, Repo: request.Repo, Worktree: provisioned.Path}
	result.Salvage, err = gitwt.Salvage(ctx, salvageRequest)
	if err != nil {
		return result, err
	}

	branchRequest := &gitwt.BranchRequest{Base: request.Head, Dest: filepath.Join(request.Salvage, "branches"), Names: []string{request.Branch}, Repo: request.Repo}
	result.Branches, err = gitwt.RemoveBranches(ctx, branchRequest)
	if err != nil {
		return result, err
	}

	return result, nil
}

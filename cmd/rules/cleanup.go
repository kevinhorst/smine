package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/gitwt"
	"github.com/kevinhorst/smine/internal/reviewscope"
	"github.com/kevinhorst/smine/internal/shell"
)

const (
	branchesSalvageDir  = "branches"
	firstDestSuffix     = 2
	harnessBranchPrefix = "worktree-wf_"
	reviewBranchPrefix  = "claude-review/"
	worktreesDirName    = "worktrees"
	worktreesParentName = ".claude"
)

var runIdPattern = regexp.MustCompile(`^wf_[0-9a-z-]+$`)

// cleanupReport is the cleanup and reset-round subcommands' output.
type cleanupReport struct {
	Branches     []string `json:"branches"`
	Removed      []string `json:"removed"`
	Skipped      []string `json:"skipped"`
	Unattributed []string `json:"unattributed"`
}

type cleanupScope struct {
	Head    string
	Overlay string
	Prefix  string
	Repo    string
	Root    string
	Round   int
	RunIds  []string
	Salvage string
}

// bind registers the flags cleanup and reset-round share; --run repeats.
func (s *cleanupScope) bind(flags *flag.FlagSet) {
	flags.StringVar(&s.Head, "head", "", "review head commit; required")
	flags.StringVar(&s.Overlay, "overlay", "", "review-context overlay patch; absent = none")
	flags.StringVar(&s.Prefix, "prefix", "", "the run's branch prefix, claude-review/<runNs>-; required")
	flags.StringVar(&s.Repo, "repo", "", "target repository; required")
	flags.StringVar(&s.Root, "root", "", "explicit worktree root; absent = harness isolation")
	flags.Func("run", "workflow run id (wf_…) whose harness worktrees are selected too; repeatable", func(runId string) error {
		s.RunIds = append(s.RunIds, runId)
		return nil
	})
	flags.StringVar(&s.Salvage, "salvage", "", "salvage directory; required")
}

func (s *cleanupScope) branchNames(ctx context.Context, runBranches []string) ([]string, error) {
	prefixed, err := gitwt.Branches(ctx, s.Prefix, s.Repo)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(prefixed)+len(runBranches))
	for _, branch := range prefixed {
		if s.isRoundBranch(branch) {
			names = append(names, branch)
		}
	}

	for _, branch := range runBranches {
		if !slices.Contains(names, branch) {
			names = append(names, branch)
		}
	}
	return names, nil
}

func (s *cleanupScope) clean(ctx context.Context) (*cleanupReport, error) {
	worktrees, err := gitwt.List(ctx, s.Repo)
	if err != nil {
		return nil, err
	}

	report := &cleanupReport{Branches: []string{}, Removed: []string{}, Skipped: []string{}, Unattributed: []string{}}
	runBranches := make([]string, 0)
	for _, worktree := range worktrees {
		isRun := s.isRunWorktree(worktree.Path)
		isSelected := isRun || s.isOwnWorktree(&worktree)
		if !isSelected && s.isForeignReview(&worktree) {
			report.Unattributed = append(report.Unattributed, worktree.Path)
		}

		if !isSelected {
			continue
		}

		if branch := s.runBranch(worktree.Branch); isRun && branch != "" {
			runBranches = append(runBranches, branch)
		}
		s.salvage(ctx, worktree.Path, report)
	}

	if _, err := shell.Run(ctx, s.Repo, "git", "worktree", "prune"); err != nil {
		return nil, fmt.Errorf("cleanupScope.clean: %w", err)
	}

	names, err := s.branchNames(ctx, runBranches)
	if err != nil {
		return nil, err
	}

	if len(names) == 0 {
		return report, nil
	}

	dest, err := freeDest(filepath.Join(s.Salvage, branchesSalvageDir))
	if err != nil {
		return nil, err
	}

	branchRequest := &gitwt.BranchRequest{Base: s.Head, Dest: dest, Names: names, Repo: s.Repo}
	branches, err := gitwt.RemoveBranches(ctx, branchRequest)
	if err != nil {
		return nil, err
	}

	report.Branches = branches.Deleted
	report.Skipped = append(report.Skipped, branches.Skipped...)
	return report, nil
}

func (s *cleanupScope) isForeignReview(worktree *gitwt.Worktree) bool {
	isOtherRun := strings.HasPrefix(worktree.Branch, reviewBranchPrefix) && !strings.HasPrefix(worktree.Branch, s.Prefix)
	return isOtherRun && worktree.Head == s.Head
}

func (s *cleanupScope) isOwnWorktree(worktree *gitwt.Worktree) bool {
	isUnderRoot := s.Root != "" && strings.HasPrefix(worktree.Path, s.Root+string(filepath.Separator))
	isRunsWorktree := strings.HasPrefix(worktree.Branch, s.Prefix) || isUnderRoot
	return isRunsWorktree && s.isRoundBranch(worktree.Branch)
}

func (s *cleanupScope) isRoundBranch(branch string) bool {
	return s.Round == 0 || strings.Contains(branch, "-r"+strconv.Itoa(s.Round)+"-")
}

func (s *cleanupScope) isRunWorktree(path string) bool {
	parent := filepath.Dir(path)
	isHarnessDir := filepath.Base(parent) == worktreesDirName && filepath.Base(filepath.Dir(parent)) == worktreesParentName
	if !isHarnessDir {
		return false
	}

	name := filepath.Base(path)
	return slices.ContainsFunc(s.RunIds, func(runId string) bool {
		return strings.HasPrefix(name, runId+"-")
	})
}

func (s *cleanupScope) isValid() bool {
	isMissingRequired := s.Head == "" || s.Prefix == "" || s.Repo == "" || s.Salvage == ""
	if isMissingRequired || !strings.HasPrefix(s.Prefix, reviewBranchPrefix) {
		return false
	}

	return !slices.ContainsFunc(s.RunIds, func(runId string) bool {
		return !runIdPattern.MatchString(runId)
	})
}

func (s *cleanupScope) runBranch(branch string) string {
	isDeletable := strings.HasPrefix(branch, harnessBranchPrefix) || strings.HasPrefix(branch, s.Prefix)
	if !isDeletable {
		return ""
	}
	return branch
}

func (s *cleanupScope) salvage(ctx context.Context, path string, report *cleanupReport) {
	dest, err := freeDest(filepath.Join(s.Salvage, filepath.Base(path)))
	if err != nil {
		report.Skipped = append(report.Skipped, path+": "+err.Error())
		return
	}

	request := &gitwt.SalvageRequest{Base: s.Head, Dest: dest, Overlay: s.Overlay, Repo: s.Repo, Worktree: path}
	salvaged, err := gitwt.Salvage(ctx, request)
	switch {
	case err != nil:
		report.Skipped = append(report.Skipped, path+": "+err.Error())
	case !salvaged.Removed:
		report.Skipped = append(report.Skipped, path+": "+salvaged.Reason)
	default:
		report.Removed = append(report.Removed, path)
	}
}

func checkRoundDir(round int, roundDir, salvage string) error {
	number, err := reviewscope.RoundNumber(roundDir)
	if err != nil {
		return err
	}

	if number != round {
		return fmt.Errorf("checkRoundDir: Round dir %s is not round %d", roundDir, round)
	}

	info, err := os.Stat(roundDir)
	if err != nil {
		return fmt.Errorf("checkRoundDir: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("checkRoundDir: Round dir %s is not a directory", roundDir)
	}

	roundAbs, err := filepath.Abs(roundDir)
	if err != nil {
		return fmt.Errorf("checkRoundDir: %w", err)
	}

	salvageAbs, err := filepath.Abs(salvage)
	if err != nil {
		return fmt.Errorf("checkRoundDir: %w", err)
	}

	relative, err := filepath.Rel(roundAbs, salvageAbs)
	if err != nil {
		return fmt.Errorf("checkRoundDir: %w", err)
	}

	isOutside := relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator))
	if !isOutside {
		return fmt.Errorf("checkRoundDir: Salvage dir %s lies inside round dir %s", salvage, roundDir)
	}
	return nil
}

func freeDest(dest string) (string, error) {
	candidate := dest
	for suffix := firstDestSuffix; ; suffix++ {
		_, err := os.Lstat(candidate)
		if errors.Is(err, fs.ErrNotExist) {
			return candidate, nil
		}

		if err != nil {
			return "", fmt.Errorf("freeDest: %w", err)
		}

		candidate = dest + "-" + strconv.Itoa(suffix)
	}
}

func moveRoundFiles(roundDir, salvage string) error {
	entries, err := os.ReadDir(roundDir)
	if err != nil {
		return fmt.Errorf("moveRoundFiles: %w", err)
	}

	if len(entries) == 0 {
		return nil
	}

	dest, err := freeDest(filepath.Join(salvage, filepath.Base(filepath.Clean(roundDir))))
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("moveRoundFiles: %w", err)
	}

	for _, entry := range entries {
		source := filepath.Join(roundDir, entry.Name())
		target := filepath.Join(dest, entry.Name())
		if err := os.Rename(source, target); err != nil {
			return fmt.Errorf("moveRoundFiles: %w", err)
		}
	}
	return nil
}

func printCleanupReport(report *cleanupReport) int {
	if code := printJson(report); code != exitClean {
		return code
	}

	if len(report.Skipped) > 0 {
		return exitViolations
	}
	return exitClean
}

func runCleanup(args []string) int {
	flags := flag.NewFlagSet("cleanup", flag.ExitOnError)
	scope := &cleanupScope{}
	scope.bind(flags)
	flags.Parse(args)
	if !scope.isValid() {
		fmt.Fprintln(os.Stderr, "usage: rules cleanup --repo <repo> --prefix claude-review/<runNs>- --head <commit> --salvage <dir> [--run <run id>]… [--root <dir>] [--overlay <patch>]")
		return exitError
	}

	report, err := scope.clean(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	return printCleanupReport(report)
}

func runResetRound(args []string) int {
	flags := flag.NewFlagSet("reset-round", flag.ExitOnError)
	scope := &cleanupScope{}
	scope.bind(flags)
	flags.IntVar(&scope.Round, "round", 0, "the round to reset, 1 or more; required")
	roundDir := flags.String("round-dir", "", "the round's artifact directory, emptied into the salvage dir; required")
	flags.Parse(args)
	isMissingRequired := scope.Round < 1 || *roundDir == ""
	if isMissingRequired || !scope.isValid() {
		fmt.Fprintln(os.Stderr, "usage: rules reset-round --repo <repo> --prefix claude-review/<runNs>- --head <commit> --salvage <dir> --round-dir <dir> --round <r> [--run <run id>]… [--root <dir>] [--overlay <patch>]")
		return exitError
	}

	if err := checkRoundDir(scope.Round, *roundDir, scope.Salvage); err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	if err := moveRoundFiles(*roundDir, scope.Salvage); err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	report, err := scope.clean(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	return printCleanupReport(report)
}

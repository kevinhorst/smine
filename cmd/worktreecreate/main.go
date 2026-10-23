// Command worktreecreate is the WorktreeCreate hook: it creates the session worktree on branch <branch_prefix><name> and prints its path.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/desktop"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/shell"
	"github.com/kevinhorst/smine/internal/trash"
)

type Aligner struct {
	section   *hookconfig.Worktreecreate
	storePath string
	trash     *trash.Store
}

// gitRetrying retries a git call that git refused for a held index.lock, waiting attempt times the configured delay.
func (a *Aligner) gitRetrying(ctx context.Context, cwd string, args ...string) error {
	var output string
	var err error
	for attempt := range a.section.ResetAttempts {
		output, err = shell.Run(ctx, cwd, "git", args...)
		if err == nil {
			return nil
		}
		if !strings.Contains(output, indexLockText) {
			break
		}
		time.Sleep(a.section.ResetRetryDelayMs.Duration() * time.Duration(attempt+1))
	}
	return fmt.Errorf("gitRetrying: git %s: %s: %w", args[0], strings.TrimSpace(output), err)
}

func (a *Aligner) recreateBranch(ctx context.Context, cwd string, branch string, startPoint string, base string, creation string) error {
	entry := &trash.Entry{
		Action: "worktreecreate base",
		Kind:   trash.KindBranchRecreated,
		Path:   cwd + "#" + branch,
		Reason: "recreated from " + startPoint + "; previous sha " + creation,
	}
	if err := a.trash.Note(entry); err != nil {
		return fmt.Errorf("recreateBranch: %w", err)
	}

	steps := [][]string{
		{"switch", "--detach"},
		{"branch", "-D", branch},
		{"switch", "--no-track", "-c", branch, startPoint},
	}

	var err error
	for _, step := range steps {
		err = a.gitRetrying(ctx, cwd, step...)
		if err != nil {
			break
		}
	}

	if err == nil {
		head, headErr := revParse(ctx, cwd, "HEAD^{commit}")
		if headErr == nil && head == base {
			return nil
		}
		err = fmt.Errorf("recreateBranch: %s is at %s, not at %s", startPoint, head, base)
	}

	restoreErr := a.gitRetrying(ctx, cwd, "switch", "-C", branch, creation)
	if restoreErr != nil {
		return fmt.Errorf("recreateBranch: %w; HEAD stays detached, the restore failed: %w", err, restoreErr)
	}
	return err
}

func (a *Aligner) Align(ctx context.Context, input *HookInput) (string, error) {
	isStartup := input.Source == sourceStartup && input.AgentId == ""
	if !isStartup {
		return "", nil
	}

	cwd, err := filepath.EvalSymlinks(input.Cwd)
	if err != nil {
		return "", fmt.Errorf("Aligner.Align: Failed to resolve %s: %w", input.Cwd, err)
	}

	name, isPoolDir := worktreeName(cwd, a.section.WorktreesDir)
	if !isPoolDir {
		return "", nil
	}

	branch, err := currentBranch(ctx, cwd)
	if err != nil {
		return "", err
	}
	if branch != a.section.BranchPrefix+name {
		return "", nil
	}

	worktree, err := desktop.ReadWorktree(a.storePath, name)
	if err != nil {
		return "", err
	}

	entryPath, err := filepath.EvalSymlinks(worktree.Path)
	isOwn := err == nil && entryPath == cwd && worktree.HookBased
	if !isOwn {
		return "", fmt.Errorf("Aligner.Align: Store entry %q is not this worktree (path %q, hookBased %t)", name, worktree.Path, worktree.HookBased)
	}

	baseName, base, err := resolveBase(ctx, cwd, a.section.DefaultBranchFallback, worktree.SourceBranch)
	if err != nil {
		return "", err
	}

	head, err := revParse(ctx, cwd, "HEAD^{commit}")
	if err != nil {
		return "", err
	}

	creation, createdFrom, err := creationEntry(ctx, cwd, branch)
	if err != nil {
		return "", err
	}
	if createdFrom == baseName {
		return "", nil
	}
	if head != creation {
		return "", nil
	}

	statusOutput, err := shell.Run(ctx, cwd, "git", "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return "", fmt.Errorf("Aligner.Align: Failed to read the status of %s: %s: %w", cwd, strings.TrimSpace(statusOutput), err)
	}
	shortHead, shortBase := shortSha(head, a.section.ShortShaLength), shortSha(base, a.section.ShortShaLength)
	if strings.TrimSpace(statusOutput) != "" {
		return fmt.Sprintf("worktree-base: %s sits at %s, the session's base is %s (%s); not moved: the tracked tree has changes", branch, shortHead, baseName, shortBase), nil
	}

	if err := a.recreateBranch(ctx, cwd, branch, baseName, base, creation); err != nil {
		return "", fmt.Errorf("Aligner.Align: Failed to move %s to %s: %w", branch, base, err)
	}

	moved := fmt.Sprintf("worktree-base: moved %s from %s to %s (%s), the base picked for this session", branch, shortHead, baseName, shortBase)
	if err := contextdocs.Regenerate(filepath.Join(cwd, contextdocs.SourceContextDir)); err != nil {
		moved = fmt.Sprintf("%s; context file not regenerated: %v", moved, err)
	}
	if err := buildAcdsl(ctx, a.section.AcdslBuildTimeoutMs.Duration(), cwd); err != nil {
		moved = fmt.Sprintf("%s; bin/acdsl not rebuilt: %v", moved, err)
	}
	return moved, nil
}

type HookInput struct {
	AgentId       string `json:"agent_id"`
	Cwd           string `json:"cwd"`
	HookEventName string `json:"hook_event_name"`
	Name          string `json:"name"`
	Source        string `json:"source"`
}

func (i *HookInput) Validate(namePattern *regexp.Regexp) error {
	// Cwd
	if !filepath.IsAbs(i.Cwd) {
		return fmt.Errorf("HookInput.Validate: Invalid field Cwd %q", i.Cwd)
	}

	// Name
	if i.HookEventName == eventSessionStart {
		return nil
	}
	if !namePattern.MatchString(i.Name) {
		return fmt.Errorf("HookInput.Validate: Invalid field Name %q", i.Name)
	}

	return nil
}

// buildAcdsl builds the repo's own acdsl into bin/ when the checkout carries its source; a vendoring repo tracks bin/acdsl instead.
func buildAcdsl(ctx context.Context, timeout time.Duration, worktreePath string) error {
	if _, err := os.Stat(filepath.Join(worktreePath, acdslSourceDir)); err != nil {
		return nil
	}

	binary := filepath.Join("bin", "acdsl")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	output, err := shell.RunWithin(ctx, worktreePath, "go", timeout, "build", "-o", binary, "./"+acdslSourceDir)
	if err != nil {
		return fmt.Errorf("buildAcdsl: %s: %w", strings.TrimSpace(output), err)
	}
	return nil
}

func createWorktree(ctx context.Context, input *HookInput, namePattern *regexp.Regexp, section *hookconfig.Worktreecreate) (string, error) {
	if err := input.Validate(namePattern); err != nil {
		return "", err
	}

	commonDirOutput, err := shell.Run(
		ctx,
		input.Cwd,
		"git",
		"rev-parse",
		"--path-format=absolute",
		"--git-common-dir",
	)
	if err != nil {
		return "", fmt.Errorf("createWorktree: Failed to resolve the common git dir of %s: %s: %w", input.Cwd, strings.TrimSpace(commonDirOutput), err)
	}

	commonDir := strings.TrimSpace(commonDirOutput)
	if filepath.Base(commonDir) != ".git" {
		return "", fmt.Errorf("createWorktree: Common git dir %s is not a checkout's .git", commonDir)
	}

	startPoint, _, err := resolveBase(ctx, input.Cwd, section.DefaultBranchFallback, "")
	if err != nil {
		return "", fmt.Errorf("createWorktree: %w", err)
	}

	mainCheckout := filepath.Dir(commonDir)
	worktreePath := filepath.Join(mainCheckout, section.WorktreesDir, input.Name)
	branch := section.BranchPrefix + input.Name
	addOutput, err := shell.Run(
		ctx,
		mainCheckout,
		"git",
		"worktree",
		"add",
		"--no-track",
		"-b",
		branch,
		worktreePath,
		startPoint,
	)
	if err != nil {
		return "", fmt.Errorf("createWorktree: Failed to add worktree %s on %s: %s: %w", worktreePath, branch, strings.TrimSpace(addOutput), err)
	}

	return worktreePath, nil
}

func creationEntry(ctx context.Context, cwd string, branch string) (string, string, error) {
	output, err := shell.Run(ctx, cwd, "git", "log", "-g", "--format=%H %gs", branch)
	if err != nil {
		return "", "", fmt.Errorf("creationEntry: Failed to read the reflog of %s: %s: %w", branch, strings.TrimSpace(output), err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	commit, subject, _ := strings.Cut(lines[len(lines)-1], " ")
	if commit == "" {
		return "", "", fmt.Errorf("creationEntry: Empty reflog for %s", branch)
	}

	createdFrom, _ := strings.CutPrefix(subject, "branch: Created from ")
	if createdFrom == subject {
		createdFrom = ""
	}
	return commit, createdFrom, nil
}

// currentBranch is the checked-out branch of cwd.
func currentBranch(ctx context.Context, cwd string) (string, error) {
	output, err := shell.Run(ctx, cwd, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("currentBranch: Failed to resolve the branch of %s: %s: %w", cwd, strings.TrimSpace(output), err)
	}
	return strings.TrimSpace(output), nil
}

func defaultBranch(ctx context.Context, cwd string, fallbackBranches []string) (string, error) {
	headOutput, err := shell.Run(ctx, cwd, "git", "symbolic-ref", "--short", "--quiet", "refs/remotes/origin/HEAD")
	if err == nil {
		return strings.TrimPrefix(strings.TrimSpace(headOutput), "origin/"), nil
	}

	for _, candidate := range fallbackBranches {
		for _, prefix := range []string{"refs/heads/", "refs/remotes/origin/"} {
			_, err := shell.Run(ctx, cwd, "git", "rev-parse", "--verify", "--quiet", prefix+candidate+"^{commit}")
			if err == nil {
				return candidate, nil
			}
		}
	}
	return "", nil
}

// loadSection reads the worktreecreate section of the hooks config and compiles its name pattern.
func loadSection(claudeHome string) (*hookconfig.Worktreecreate, *regexp.Regexp, error) {
	home, err := config.ClaudeHomeDir(claudeHome)
	if err != nil {
		return nil, nil, fmt.Errorf("loadSection: %w", err)
	}

	cfg, err := hookconfig.Load(home)
	if err != nil {
		return nil, nil, fmt.Errorf("loadSection: %w", err)
	}

	namePattern, err := regexp.Compile(cfg.Worktreecreate.NamePattern)
	if err != nil {
		return nil, nil, fmt.Errorf("loadSection: %s: worktreecreate.name_pattern: %w", hookconfig.Path(home), err)
	}
	return &cfg.Worktreecreate, namePattern, nil
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stderr, os.Stdin, os.Stdout))
}

func resolveBase(ctx context.Context, cwd string, fallbackBranches []string, sourceBranch string) (string, string, error) {
	baseName := sourceBranch
	if baseName == "" {
		defaultName, err := defaultBranch(ctx, cwd, fallbackBranches)
		if err != nil {
			return "", "", err
		}
		baseName = defaultName
	}
	if baseName == "" {
		return "", "", errors.New("resolveBase: No source branch recorded and no default branch")
	}

	for _, startPoint := range []string{baseName, "origin/" + baseName} {
		ref := "refs/heads/" + startPoint
		if startPoint != baseName {
			ref = "refs/remotes/" + startPoint
		}
		output, err := shell.Run(ctx, cwd, "git", "rev-parse", "--verify", "--quiet", ref+"^{commit}")
		if err == nil {
			return startPoint, strings.TrimSpace(output), nil
		}
	}
	return "", "", fmt.Errorf("resolveBase: Source branch %q names no local or origin branch", baseName)
}

// revParse is the commit a revision names.
func revParse(ctx context.Context, cwd string, revision string) (string, error) {
	output, err := shell.Run(ctx, cwd, "git", "rev-parse", "--verify", revision)
	if err != nil {
		return "", fmt.Errorf("revParse: Failed to resolve %s in %s: %s: %w", revision, cwd, strings.TrimSpace(output), err)
	}
	return strings.TrimSpace(output), nil
}

// -base exits 0 even when the base cannot be applied, so the session still starts.
func run(ctx context.Context, args []string, stderr io.Writer, stdin io.Reader, stdout io.Writer) int {
	flags := flag.NewFlagSet("worktreecreate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	isBase := flags.Bool(flagBase, false, "SessionStart: move a fresh session branch onto the base the Desktop recorded for its worktree")
	claudeHome := flags.String(flagClaudeHome, "", "Claude Code home directory (default ~/.claude)")
	storePath := flags.String(flagStore, "", "Desktop worktree store (default <user config dir>/Claude/git-worktrees.json)")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	input := &HookInput{}
	if err := json.NewDecoder(stdin).Decode(input); err != nil {
		fmt.Fprintf(stderr, "run: Failed to decode hook input: %v\n", err)
		return 1
	}

	section, namePattern, err := loadSection(*claudeHome)
	if err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return 1
	}
	if *isBase {
		return runBase(ctx, *claudeHome, input, namePattern, section, stderr, stdout, *storePath)
	}

	worktreePath, err := createWorktree(ctx, input, namePattern, section)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := contextdocs.Regenerate(filepath.Join(worktreePath, contextdocs.SourceContextDir)); err != nil {
		fmt.Fprintln(stderr, err)
	}
	if err := buildAcdsl(ctx, section.AcdslBuildTimeoutMs.Duration(), worktreePath); err != nil {
		fmt.Fprintln(stderr, err)
	}

	fmt.Fprintln(stdout, worktreePath)
	return 0
}

func runBase(ctx context.Context, claudeHome string, input *HookInput, namePattern *regexp.Regexp, section *hookconfig.Worktreecreate, stderr io.Writer, stdout io.Writer, storePath string) int {
	if err := input.Validate(namePattern); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	stateDir, err := config.StateDir(claudeHome)
	if err != nil {
		fmt.Fprintf(stderr, "runBase: %v\n", err)
		return 1
	}

	if storePath == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			fmt.Fprintf(stderr, "runBase: Failed to resolve user config dir: %v\n", err)
			return 1
		}
		storePath = desktop.WorktreeStorePath(configDir)
	}

	aligner := &Aligner{section: section, storePath: storePath, trash: trash.New(stateDir, 0)}
	line, err := aligner.Align(ctx, input)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 0
	}
	if line != "" {
		fmt.Fprintln(stdout, line)
	}
	return 0
}

// shortSha is the leading length hex digits of a commit, the whole commit when it is shorter.
func shortSha(commit string, length int) string {
	return commit[:min(length, len(commit))]
}

// worktreeName is the <name> of a cwd at <main>/<worktreesDir>/<name>.
func worktreeName(cwd string, worktreesDir string) (string, bool) {
	poolDir := string(filepath.Separator) + filepath.FromSlash(worktreesDir)
	isPoolDir := strings.HasSuffix(filepath.Dir(cwd), poolDir)
	if !isPoolDir {
		return "", false
	}
	return filepath.Base(cwd), true
}

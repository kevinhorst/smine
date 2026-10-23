// Command golangciwrap is an ACDSL verifier: golangci-lint reports no finding on an anchored Go file.
package main

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/acdsl"
	"github.com/kevinhorst/smine/internal/shell"
)

const (
	exitViolations = 1
	exitError      = 2

	lintSubcommand = "run"
	// a second run in this checkout waits for its lock instead of failing
	serialRunnersFlag = "--allow-serial-runners"
)

var contractLineRe = regexp.MustCompile(`^(\S+?):\d+:\d+: `)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

// changedGoFiles is anchored ∩ changed against the merge-base with main; ok is false when no base resolves.
func changedGoFiles(anchored map[string]bool) (map[string]bool, bool) {
	ctx := context.Background()
	base, err := acdsl.GitMergeBase(ctx, ".", "main")
	if err != nil {
		base, err = acdsl.GitMergeBase(ctx, ".", "origin/main")
	}
	if err != nil {
		return nil, false
	}

	changed, err := acdsl.GitChangedFiles(ctx, ".", base)
	if err != nil {
		return nil, false
	}

	scoped := map[string]bool{}
	for _, file := range changed {
		if anchored[file] {
			scoped[file] = true
		}
	}
	return scoped, true
}

func isolateLint() error {
	cacheDir, err := filepath.Abs(filepath.Join("bin", "golangci-cache"))
	if err != nil {
		return fmt.Errorf("isolateLint: %w", err)
	}

	lockDir, err := filepath.Abs(filepath.Join("bin", "golangci-tmp"))
	if err != nil {
		return fmt.Errorf("isolateLint: %w", err)
	}

	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return fmt.Errorf("isolateLint: %w", err)
	}

	if err := os.Setenv("GOLANGCI_LINT_CACHE", cacheDir); err != nil {
		return fmt.Errorf("isolateLint: %w", err)
	}

	if err := os.Setenv("TMPDIR", lockDir); err != nil {
		return fmt.Errorf("isolateLint: %w", err)
	}
	return nil
}

// lintArgs returns nil when nothing needs linting: no anchored Go file changed against the base.
func lintArgs(anchored, changed map[string]bool, hasBase bool) []string {
	isFixtureRun := true
	for file := range anchored {
		if !strings.HasPrefix(file, "acdsl/testdata/") {
			isFixtureRun = false
			break
		}
	}
	if isFixtureRun {
		dirs := map[string]bool{}
		for file := range anchored {
			dirs["./"+path.Dir(file)+"/"] = true
		}
		args := []string{lintSubcommand, serialRunnersFlag, "--no-config", "--enable-only", "mnd"}
		return append(args, slices.Sorted(maps.Keys(dirs))...)
	}
	if !hasBase {
		return []string{lintSubcommand, serialRunnersFlag, "./..."}
	}
	dirs := map[string]bool{}
	for file := range changed {
		if anchored[file] {
			dirs["./"+path.Dir(file)+"/"] = true
		}
	}
	if len(dirs) == 0 {
		return nil
	}
	return append([]string{lintSubcommand, serialRunnersFlag}, slices.Sorted(maps.Keys(dirs))...)
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "golangciwrap: usage: <files-list>")
		return exitError
	}
	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "golangciwrap:", err)
		return exitError
	}
	anchored := map[string]bool{}
	for file := range strings.FieldsSeq(string(listRaw)) {
		anchored[file] = true
	}
	if len(anchored) == 0 {
		return 0
	}
	changed, hasBase := changedGoFiles(anchored)
	lint := lintArgs(anchored, changed, hasBase)
	if lint == nil {
		return 0
	}
	if err := isolateLint(); err != nil {
		fmt.Fprintln(os.Stderr, "golangciwrap:", err)
		return exitError
	}
	output, err := shell.Run(context.Background(), ".", "golangci-lint", lint...)
	if err == nil {
		return 0
	}
	violations := 0
	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		match := contractLineRe.FindStringSubmatch(line)
		if match == nil || !anchored[match[1]] {
			continue
		}
		fmt.Fprintln(out, line)
		violations++
	}
	if violations > 0 {
		return exitViolations
	}
	fmt.Fprintf(os.Stderr, "golangciwrap: %v\n%s\n", err, output)
	return exitError
}

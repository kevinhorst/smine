package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/acdsl"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/planrules"
)

const (
	displayArgCount   = 2
	displayDirName    = "plans"
	displayFileMode   = 0o644
	displaySuffix     = ".md"
	planEmbedPrefix   = "](ui/"
	planFileDepth     = 5
	planSectionPrefix = "## "
	planUiDirName     = "ui"
)

func checkDisplayPath(display string, home string) error {
	absolute, err := filepath.Abs(display)
	if err != nil {
		return fmt.Errorf("checkDisplayPath: %w", err)
	}

	parent, err := filepath.EvalSymlinks(filepath.Dir(absolute))
	if err != nil {
		return fmt.Errorf("checkDisplayPath: %w", err)
	}

	plansDir, err := filepath.EvalSymlinks(filepath.Join(home, displayDirName))
	if err != nil {
		return fmt.Errorf("checkDisplayPath: %w", err)
	}

	isInPlansDir := parent == plansDir && strings.HasSuffix(absolute, displaySuffix)
	if !isInPlansDir {
		return fmt.Errorf("checkDisplayPath: Display file %s is not a markdown file directly in %s", display, plansDir)
	}

	info, err := os.Lstat(absolute)
	isOtherThanFile := err == nil && !info.Mode().IsRegular()
	if isOtherThanFile {
		return fmt.Errorf("checkDisplayPath: Display file %s exists and is not a regular file", display)
	}
	return nil
}

func displayContent(plan []byte, planPath string, rules *planrules.File) string {
	text := string(acdsl.StripProjection(planPath, plan))
	uiDir := filepath.ToSlash(filepath.Join(filepath.Dir(planPath), planUiDirName))
	text = strings.ReplaceAll(text, planEmbedPrefix, "]("+uiDir+"/")
	if rules == nil {
		return text
	}

	lines := strings.Split(text, "\n")
	firstSection := slices.IndexFunc(lines, func(line string) bool {
		return strings.HasPrefix(line, planSectionPrefix)
	})
	if firstSection < 0 {
		firstSection = len(lines)
	}

	shown := slices.Concat(lines[:firstSection], planrules.ViewLines(rules), lines[firstSection:])
	return strings.Join(shown, "\n")
}

func isPlanDirFile(resolved string) bool {
	segments := strings.Split(filepath.ToSlash(resolved), "/")
	tail := strings.Join(segments[max(len(segments)-planFileDepth, 0):], "/")
	return planrules.IsPlanFile(tail)
}

func planSourcePath(planFile string) (string, error) {
	absolute, err := filepath.Abs(planFile)
	if err != nil {
		return "", fmt.Errorf("planSourcePath: %w", err)
	}

	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("planSourcePath: %w", err)
	}

	if !isPlanDirFile(resolved) {
		return "", fmt.Errorf("planSourcePath: %s is not a plan dir's plan file", planFile)
	}
	return resolved, nil
}

func runPlanDisplay(args []string) int {
	if len(args) != displayArgCount {
		fmt.Fprintln(os.Stderr, "usage: rules plan-display <plan file> <display file>")
		return exitError
	}

	home, err := config.ClaudeHomeDir("")
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	if err := writePlanDisplay(args[1], home, args[0]); err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	fmt.Println("plan-display: wrote", args[1])
	return exitClean
}

func writePlanDisplay(display string, home string, planFile string) error {
	planPath, err := planSourcePath(planFile)
	if err != nil {
		return err
	}

	if err := checkDisplayPath(display, home); err != nil {
		return err
	}

	plan, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("writePlanDisplay: %w", err)
	}

	rulesPath := filepath.Join(filepath.Dir(filepath.Dir(planPath)), planrules.FileName)
	rules, err := planrules.LoadFile(rulesPath)
	isUnreadable := err != nil && !errors.Is(err, os.ErrNotExist)
	if isUnreadable {
		return fmt.Errorf("writePlanDisplay: %w", err)
	}

	content := displayContent(plan, planPath, rules)
	if err := os.WriteFile(display, []byte(content), displayFileMode); err != nil {
		return fmt.Errorf("writePlanDisplay: %w", err)
	}
	return nil
}

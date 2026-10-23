// Command skillmodelcheck is an ACDSL verifier: every formal clause of a skill's derived semantic model validates.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skillmodel"
)

const (
	exitClean      = 0
	exitViolations = 1
	exitError      = 2
	skillsRoot     = "skills"
	vocabularyFile = "skills/vocabulary.cue"
)

func checkSkill(dir string, out io.Writer) int {
	file := skillgraph.ModelPath(dir)
	skill, err := skillgraph.Load(dir)
	if err != nil {
		fmt.Fprintf(out, "%s:1: model does not load — clauses unverified (violations: ACDSL-SKILL-005)\n", file)
		return 1
	}

	model, err := skillmodel.Derive(skill)
	if err != nil {
		fmt.Fprintf(out, "%s:1: %v\n", file, err)
		return 1
	}

	found := model.Validate()
	for _, violation := range found {
		fmt.Fprintf(out, "%s:1: %s\n", file, violation)
	}
	return len(found)
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "skillmodelcheck: usage: <files-list>")
		return exitError
	}
	listData, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillmodelcheck:", err)
		return exitError
	}

	dirs, err := skillDirs(strings.Fields(string(listData)))
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillmodelcheck:", err)
		return exitError
	}

	violations := 0
	for _, dir := range dirs {
		violations += checkSkill(dir, out)
	}
	if violations > 0 {
		return exitViolations
	}
	return exitClean
}

func skillDirs(files []string) ([]string, error) {
	var dirs []string
	for _, file := range files {
		switch {
		case filepath.ToSlash(file) == vocabularyFile:
			discovered, err := skillgraph.Discover(skillsRoot)
			if err != nil {
				return nil, fmt.Errorf("skillDirs: %w", err)
			}
			dirs = append(dirs, discovered...)
		case skillgraph.IsModelFile(file):
			dirs = append(dirs, filepath.Dir(file))
		}
	}
	slices.Sort(dirs)
	return slices.Compact(dirs), nil
}

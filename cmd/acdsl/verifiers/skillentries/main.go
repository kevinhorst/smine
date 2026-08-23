// Command skillentries is a registered ACDSL verifier over SKILL.md files.
// It validates the skill entry grammar (internal/skills.ParseEntries):
//
//	headline — **SKILL-<NAME>-<TOPIC>-NNN** `[class]` — statement
//	NAME     — the leaf directory name upper-cased without hyphens
//	class    — one of internal/skills.EntryClasses
//	payload  — a [payload] entry is followed by one fenced block
//	IDs      — unique within the file
//
// Skills without entries pass — entries are opt-in until a skill migrates.
// Contract: args = <files-list path>; one violation per stdout line as
// file:line: message; exit 0 pass, 1 violations, 2 error.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kevinhorst/smine/internal/skills"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "skillentries: usage: <files-list>")
		return 2
	}
	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "skillentries:", err)
		return 2
	}
	violations := 0
	for _, file := range strings.Fields(string(listRaw)) {
		if filepath.Base(file) != "SKILL.md" {
			continue
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintln(os.Stderr, "skillentries:", err)
			return 2
		}
		violations += checkEntries(file, strings.Split(string(raw), "\n"), out)
	}
	if violations > 0 {
		return 1
	}
	return 0
}

func checkEntries(file string, lines []string, out io.Writer) int {
	entries, parseViolations := skills.ParseEntries(lines)
	violations := 0
	for _, violation := range parseViolations {
		fmt.Fprintf(out, "%s:%s\n", file, violation)
		violations++
	}
	wantName := skills.SkillIdName(filepath.Base(filepath.Dir(file)))
	seen := map[string]int{}
	for _, entry := range entries {
		if entry.Skill != wantName {
			fmt.Fprintf(out, "%s:%d: entry %s names skill %s, leaf dir is %s\n", file, entry.Line+1, entry.Id, entry.Skill, wantName)
			violations++
		}
		if previous, isDuplicate := seen[entry.Id]; isDuplicate {
			fmt.Fprintf(out, "%s:%d: duplicate entry id %s (first at line %d)\n", file, entry.Line+1, entry.Id, previous)
			violations++
		}
		seen[entry.Id] = entry.Line + 1
	}
	return violations
}

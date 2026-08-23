// Command rules validates the ACTION/RULE/FACT entry files and generates
// the committed context file (context/context.json).
//
//	rules validate [--deployed <dir>] [--dir <context-dir>]
//	rules generate [-check] [--dir <context-dir>] [--out <file>]
//	rules filter --target <name> [--langs go,sql] [--dir <context-dir>] <file>
//	rules render-skill [--disable id,glob*] [--list-entries] <SKILL.md>
//
// validate parses a context tree (actions/ + rules/ + facts/; with --deployed,
// baseline files are recognized by their synced header) and prints one line
// per contract violation. generate renders context.json — entries, tombstones,
// and the aspect taxonomy; -check compares against the committed file instead
// of writing. filter prints one rules file with non-deployable entries
// removed — the sync boundary: entries whose reach does not cover the target
// and unselected-language entries never ship. render-skill prints a SKILL.md
// with disabled entries stripped, or its parsed entries as JSON — the SKILL.md
// is the only source, there is no stored skill index.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/skills"
)

const (
	exitClean      = 0
	exitViolations = 1
	exitError      = 2
)

// skillEntryView is the --list-entries JSON row: Entry with the line reported
// 1-based like every other surfaced file position, while Entry.Line stays the
// 0-based slice index the renderer needs.
type skillEntryView struct {
	skills.Entry
	Line int `json:"line"`
}

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: rules <validate|generate|filter|render-skill> [flags]")
		return exitError
	}

	switch os.Args[1] {
	case "validate":
		return runValidate(os.Args[2:])
	case "generate":
		return runGenerate(os.Args[2:])
	case "render-skill":
		return runRenderSkill(os.Args[2:])
	case "filter":
		return runFilter(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "rules: unknown command: %s\n", os.Args[1])
		return exitError
	}
}

func runValidate(args []string) int {
	flags := flag.NewFlagSet("validate", flag.ExitOnError)
	deployedDir := flags.String("deployed", "", "deployed context root; baseline files recognized by their synced header")
	sourceDir := flags.String("dir", "context", "source context directory (ignored with --deployed)")
	flags.Parse(args)

	contextDir, detectOrigin := *sourceDir, false
	if *deployedDir != "" {
		contextDir, detectOrigin = *deployedDir, true
	}

	set, err := contextdocs.ParseContext(contextDir, detectOrigin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	aspects, err := contextdocs.LoadAspects(*sourceDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	violations := contextdocs.ValidateRules(set, aspects)
	for _, violation := range violations {
		fmt.Println(violation)
	}
	if len(violations) > 0 {
		fmt.Fprintf(os.Stderr, "rules: %d violation(s) in %s\n", len(violations), contextDir)
		return exitViolations
	}

	fmt.Printf("rules: %d entries, %d tombstones OK in %s\n", len(set.Entries), len(set.Tombstones), contextDir)
	return exitClean
}

// runFilter prints one rules file with non-deployable entries removed —
// the sync boundary: entries whose reach does not cover the target and
// unselected-language entries never ship.
func runFilter(args []string) int {
	flags := flag.NewFlagSet("filter", flag.ExitOnError)
	targetName := flags.String("target", "", "deploy-target repo name (dir basename); required")
	langsCsv := flags.String("langs", "", "comma-separated deployed languages (rules-guide basenames)")
	sourceDir := flags.String("dir", "context", "directory whose context.json carries the aspects array")
	flags.Parse(args)
	if flags.NArg() != 1 || *targetName == "" {
		fmt.Fprintln(os.Stderr, "usage: rules filter --target <name> [--langs a,b] [--dir <context-dir>] <file>")
		return exitError
	}
	content, err := os.ReadFile(flags.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	aspects, err := contextdocs.LoadAspects(*sourceDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	var langs []string
	for _, lang := range strings.Split(*langsCsv, ",") {
		if lang != "" {
			langs = append(langs, lang)
		}
	}
	fmt.Print(contextdocs.FilterRulesFile(string(content), *targetName, aspects, langs))
	return exitClean
}

func runGenerate(args []string) int {
	flags := flag.NewFlagSet("generate", flag.ExitOnError)
	shouldCheck := flags.Bool("check", false, "compare against the committed context file instead of writing")
	deployedDir := flags.String("deployed", "", "deployed context root; baseline files recognized by their synced header")
	sourceDir := flags.String("dir", "context", "source context directory (ignored with --deployed)")
	outFile := flags.String("out", "context/context.json", "context file to write or check")
	flags.Parse(args)

	contextDir, detectOrigin := *sourceDir, false
	if *deployedDir != "" {
		contextDir, detectOrigin = *deployedDir, true
	}

	set, err := contextdocs.ParseContext(contextDir, detectOrigin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	aspects, err := contextdocs.LoadAspects(*sourceDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	if violations := contextdocs.ValidateRules(set, aspects); len(violations) > 0 {
		for _, violation := range violations {
			fmt.Println(violation)
		}
		fmt.Fprintf(os.Stderr, "rules: refusing to generate from %d violation(s)\n", len(violations))
		return exitViolations
	}

	rendered, err := contextdocs.RenderContextJson(set, aspects)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	if *shouldCheck {
		committed, err := os.ReadFile(*outFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "rules:", err)
			return exitError
		}
		if !bytes.Equal(committed, rendered) {
			fmt.Fprintf(os.Stderr, "rules: %s is stale — run: go run ./cmd/rules generate\n", *outFile)
			return exitViolations
		}
		fmt.Printf("rules: %s is current\n", *outFile)
		return exitClean
	}

	if err := os.WriteFile(*outFile, rendered, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	fmt.Printf("rules: wrote %s (%d entries, %d tombstones)\n", *outFile, len(set.Entries), len(set.Tombstones))
	return exitClean
}

// runRenderSkill prints a SKILL.md with the disabled entries stripped, or —
// with --list-entries — its parsed entries as JSON. The SKILL.md is the only
// source; there is no stored index.
func runRenderSkill(args []string) int {
	flags := flag.NewFlagSet("render-skill", flag.ExitOnError)
	disable := flags.String("disable", "", "comma-separated entry ids or trailing-* globs to strip")
	listEntries := flags.Bool("list-entries", false, "print the parsed entries as JSON instead of rendering")
	flags.Parse(args)
	if flags.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: rules render-skill [--disable id,glob*] [--list-entries] <SKILL.md>")
		return exitError
	}
	raw, err := os.ReadFile(flags.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	if *listEntries {
		return listSkillEntries(flags.Arg(0), string(raw))
	}
	var tokens []string
	if *disable != "" {
		tokens = strings.Split(*disable, ",")
	}
	rendered, err := skills.RenderSkill(string(raw), tokens)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	fmt.Print(rendered)
	return exitClean
}

// listSkillEntries prints the parsed entries of one SKILL.md as JSON;
// grammar violations go to stderr and turn the exit into violations.
func listSkillEntries(path, raw string) int {
	entries, violations := skills.ParseEntries(strings.Split(raw, "\n"))
	for _, violation := range violations {
		fmt.Fprintf(os.Stderr, "%s:%s\n", path, violation)
	}
	views := make([]skillEntryView, 0, len(entries))
	for _, entry := range entries {
		views = append(views, skillEntryView{Entry: entry, Line: entry.Line + 1})
	}
	data, err := json.MarshalIndent(views, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	fmt.Println(string(data))
	if len(violations) > 0 {
		return exitViolations
	}
	return exitClean
}

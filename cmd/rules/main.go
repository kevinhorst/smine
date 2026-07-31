// Command rules validates the FACT/NEVER/ALWAYS entry files and generates
// the committed registry (context/rules.json).
//
//	rules validate [--pack <dir>] [--dir <rules-dir>]
//	rules generate [-check] [--dir <rules-dir>] [--out <file>]
//
// validate parses the source rules directory (or a deployed pack's rules/ +
// facts/ with --pack, where baseline files are recognized by their synced
// header) and prints one line per contract violation. generate renders the
// registry JSON; -check compares against the committed file instead of
// writing.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kevinhorst/smine/internal/contextdocs"
)

const (
	exitClean      = 0
	exitError      = 2
	exitViolations = 1
)

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: rules <validate|generate> [flags]")
		return exitError
	}

	switch os.Args[1] {
	case "validate":
		return runValidate(os.Args[2:])
	case "generate":
		return runGenerate(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "rules: unknown command: %s\n", os.Args[1])
		return exitError
	}
}

func runValidate(args []string) int {
	flags := flag.NewFlagSet("validate", flag.ExitOnError)
	packDir := flags.String("pack", "", "deployed pack root; validates <pack>/rules + <pack>/facts with origin detection")
	rulesDir := flags.String("dir", "context/rules", "source rules directory (ignored with --pack)")
	flags.Parse(args)

	var set contextdocs.RuleSet
	var err error
	described := *rulesDir
	if *packDir != "" {
		described = *packDir
		set, err = contextdocs.ParsePack(*packDir)
	} else {
		set, err = contextdocs.ParseRulesDir(*rulesDir, false)
		if err == nil {
			// The source tree is this repo's own pack: its sibling facts dir
			// validates with the baseline rules.
			factsDir := filepath.Join(*rulesDir, "..", "facts")
			if _, statErr := os.Stat(factsDir); statErr == nil {
				var factsSet contextdocs.RuleSet
				factsSet, err = contextdocs.ParseFactsDir(factsDir)
				set.Entries = append(set.Entries, factsSet.Entries...)
				set.Tombstones = append(set.Tombstones, factsSet.Tombstones...)
			}
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	aspects, err := loadAspectsWithFallback(*packDir, *rulesDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	violations := contextdocs.ValidateRules(set, aspects)
	for _, violation := range violations {
		fmt.Println(violation)
	}
	if len(violations) > 0 {
		fmt.Fprintf(os.Stderr, "rules: %d violation(s) in %s\n", len(violations), described)
		return exitViolations
	}

	fmt.Printf("rules: %d entries, %d tombstones OK in %s\n", len(set.Entries), len(set.Tombstones), described)
	return exitClean
}

// loadAspectsWithFallback prefers the pack's own vocabulary and falls back
// to the source rules dir for packs without one (facts-only, legacy).
func loadAspectsWithFallback(packDir, rulesDir string) ([]contextdocs.RuleAspect, error) {
	if packDir != "" {
		aspects, err := contextdocs.LoadAspects(filepath.Join(packDir, "rules"))
		if err == nil {
			return aspects, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	return contextdocs.LoadAspects(rulesDir)
}

func runGenerate(args []string) int {
	flags := flag.NewFlagSet("generate", flag.ExitOnError)
	shouldCheck := flags.Bool("check", false, "compare against the committed registry instead of writing")
	rulesDir := flags.String("dir", "context/rules", "source rules directory")
	outFile := flags.String("out", "context/rules.json", "registry file to write or check")
	flags.Parse(args)

	set, err := contextdocs.ParseRulesDir(*rulesDir, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	aspects, err := contextdocs.LoadAspects(*rulesDir)
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

	rendered, err := contextdocs.RenderRulesJson(set)
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

// Command runbookformat is a registered ACDSL verifier: the lintable subset
// of rules/runbooks.md over Bruno runbook collections. Checks per .bru file:
// request files need a non-empty docs block and an assert block;
// collection.bru needs a non-empty docs block; folder.bru and environment
// files are skipped. No prose heuristics.
//
// Contract: args = <files-list path>; one violation per stdout line as
// file:line: message; exit 0 pass, 1 violations, 2 error. No params.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "runbookformat: usage: <files-list>")
		return 2
	}
	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "runbookformat:", err)
		return 2
	}
	violations := 0
	for _, file := range strings.Fields(string(listRaw)) {
		raw, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintln(os.Stderr, "runbookformat:", err)
			return 2
		}
		violations += checkFile(file, strings.Split(string(raw), "\n"), out)
	}
	if violations > 0 {
		return 1
	}
	return 0
}

func checkFile(file string, lines []string, out io.Writer) int {
	base := filepath.Base(file)
	if base == "folder.bru" || filepath.Base(filepath.Dir(file)) == "environments" {
		return 0
	}
	violations := 0
	if !hasNonEmptyBlock(lines, "docs") {
		fmt.Fprintf(out, "%s:1: non-empty docs block missing (rules/runbooks.md RULE-RUNBOOK-004)\n", file)
		violations++
	}
	if base != "collection.bru" && !hasNonEmptyBlock(lines, "assert") {
		fmt.Fprintf(out, "%s:1: assert block missing (rules/runbooks.md RULE-RUNBOOK-006)\n", file)
		violations++
	}
	return violations
}

// hasNonEmptyBlock reports whether a top-level "name {" block exists and
// contains at least one non-blank line before its closing brace.
func hasNonEmptyBlock(lines []string, name string) bool {
	inBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inBlock {
			if trimmed == name+" {" {
				inBlock = true
			}
			continue
		}
		if trimmed == "}" {
			return false
		}
		if trimmed != "" {
			return true
		}
	}
	return false
}

// Command runbookformat is an ACDSL verifier: a Bruno runbook file meets the lintable subset of rules/runbooks.md.
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

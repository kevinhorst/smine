// Command interpretationformat is an ACDSL verifier: an eval run's interpretation.md stays within the Tests tab bounds.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kevinhorst/smine/internal/evals"
)

// The ACDSL verifier contract: 0 clean, 1 violations found, 2 the verifier could not run.
const (
	exitClean      = 0
	exitViolations = 1
	exitToolError  = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "interpretationformat: usage: <files-list>")
		return exitToolError
	}
	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "interpretationformat:", err)
		return exitToolError
	}
	violations := 0
	for file := range strings.FieldsSeq(string(listRaw)) {
		if !evals.IsInterpretationChecked(file) {
			continue
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintln(os.Stderr, "interpretationformat:", err)
			return exitToolError
		}
		for _, violation := range evals.CheckInterpretation(string(raw)) {
			fmt.Fprintf(out, "%s:%d: %s (rules/skills.md RULE-SKILL-020)\n", file, violation.Line, violation.Message)
			violations++
		}
	}
	if violations > 0 {
		return exitViolations
	}
	return exitClean
}

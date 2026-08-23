// Command rulescheck is a registered ACDSL verifier wrapping cmd/rules —
// mode=validate runs `go run ./cmd/rules validate`, mode=generate-check runs
// `go run ./cmd/rules generate -check`. Both verbs already print one
// violation per stdout line and exit 0/1; the wrapper normalizes tool
// failures. The anchored files-list only decides WHEN the rule fires — the
// wrapped command always checks the whole pack (its own contract).
// Replaces the two raw Makefile audit lines (gofmt-fold precedent).
//
// Contract: args = <files-list path> mode=<validate|generate-check>; exit 0
// pass, 1 violations, 2 error.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "rulescheck: usage: <files-list> mode=<mode>")
		return 2
	}
	mode := ""
	for _, arg := range args[1:] {
		if value, ok := strings.CutPrefix(arg, "mode="); ok {
			mode = value
		}
	}
	var verb []string
	switch mode {
	case "validate":
		verb = []string{"run", "./cmd/rules", "validate"}
	case "generate-check":
		verb = []string{"run", "./cmd/rules", "generate", "-check"}
	default:
		fmt.Fprintf(os.Stderr, "rulescheck: unknown mode %q\n", mode)
		return 2
	}
	output, err := shell.Run(context.Background(), ".", "go", verb...)
	if err == nil {
		return 0
	}
	violations := 0
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line != "" && !strings.HasPrefix(line, "rules:") {
			fmt.Fprintln(out, line)
			violations++
		}
	}
	if violations > 0 {
		return 1
	}
	fmt.Fprintf(os.Stderr, "rulescheck: %v\n%s\n", err, output)
	return 2
}

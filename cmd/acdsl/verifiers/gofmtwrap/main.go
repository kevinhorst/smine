// Command gofmtwrap is a registered ACDSL verifier: it fails (exit 1) when
// any anchored file is not gofmt-formatted. It wraps `gofmt -l`, which exits 0
// even when it finds unformatted files (findings are stdout filenames) — the
// wrapper maps output to the registry contract: one file:line: violation per
// finding, exit 1. Replaces the raw `gofmt -l cmd internal` audit line.
//
// Contract: args = <files-list path>; one violation per stdout line as
// file:line: message; exit 0 pass, 1 violations, 2 error. No params.
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
		fmt.Fprintln(os.Stderr, "gofmtwrap: usage: <files-list>")
		return 2
	}
	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "gofmtwrap:", err)
		return 2
	}
	files := strings.Fields(string(listRaw))
	if len(files) == 0 {
		return 0
	}

	output, err := shell.Run(context.Background(), ".", "gofmt", append([]string{"-l"}, files...)...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gofmtwrap: %v\n%s\n", err, output)
		return 2
	}

	violations := 0
	for _, file := range strings.Fields(strings.TrimSpace(output)) {
		fmt.Fprintf(out, "%s:1: file is not gofmt-formatted — run gofmt -w\n", file)
		violations++
	}
	if violations > 0 {
		return 1
	}
	return 0
}

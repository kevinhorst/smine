// Command rulescheck is an ACDSL verifier: cmd/rules validate passes over the whole rules pack.
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

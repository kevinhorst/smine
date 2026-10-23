// Command gofmtwrap is an ACDSL verifier: every anchored file is gofmt-formatted.
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

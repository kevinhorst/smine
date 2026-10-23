// Command filebaseline is an ACDSL verifier: every listed file is on the committed baseline list.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// exitToolError is the verifier contract's tool-failure exit (0 pass, 1 violations).
const exitToolError = 2

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "filebaseline: usage: <files-list> baseline=<path>")
		return exitToolError
	}
	params := map[string]string{}
	for _, arg := range args[1:] {
		key, value, ok := strings.Cut(arg, "=")
		if !ok {
			fmt.Fprintf(os.Stderr, "filebaseline: bad param %q\n", arg)
			return exitToolError
		}
		params[key] = value
	}
	if params["baseline"] == "" {
		fmt.Fprintln(os.Stderr, "filebaseline: param baseline is required")
		return exitToolError
	}

	baselineRaw, err := os.ReadFile(params["baseline"])
	if err != nil {
		fmt.Fprintln(os.Stderr, "filebaseline:", err)
		return exitToolError
	}
	baseline := map[string]bool{}
	for line := range strings.Lines(string(baselineRaw)) {
		entry := strings.TrimSpace(line)
		if entry == "" || strings.HasPrefix(entry, "#") {
			continue
		}
		baseline[entry] = true
	}

	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "filebaseline:", err)
		return exitToolError
	}

	violations := 0
	for path := range strings.FieldsSeq(string(listRaw)) {
		if baseline[path] {
			continue
		}
		fmt.Fprintf(out, "%s:1: file is not in the baseline %s — new files of this class are forbidden\n", path, params["baseline"])
		violations++
	}

	if violations > 0 {
		return 1
	}
	return 0
}

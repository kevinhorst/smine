// Command forbidphrase is an ACDSL verifier: no file matches the forbidden pattern, and some file holds the required one.
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

// exitToolError is the verifier contract's tool-failure exit (0 pass, 1 violations).
const exitToolError = 2

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "forbidphrase: usage: <files-list> pattern=<RE2> [require=<RE2>]")
		return exitToolError
	}
	params := map[string]string{}
	for _, arg := range args[1:] {
		key, value, ok := strings.Cut(arg, "=")
		if !ok {
			fmt.Fprintf(os.Stderr, "forbidphrase: bad param %q\n", arg)
			return exitToolError
		}
		params[key] = value
	}
	if params["pattern"] == "" {
		fmt.Fprintln(os.Stderr, "forbidphrase: param pattern is required")
		return exitToolError
	}

	pattern, err := regexp.Compile(params["pattern"])
	if err != nil {
		fmt.Fprintln(os.Stderr, "forbidphrase:", err)
		return exitToolError
	}

	var require *regexp.Regexp
	if params["require"] != "" {
		require, err = regexp.Compile(params["require"])
		if err != nil {
			fmt.Fprintln(os.Stderr, "forbidphrase:", err)
			return exitToolError
		}
	}

	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "forbidphrase:", err)
		return exitToolError
	}

	violations := 0
	requireMet := false
	for path := range strings.FieldsSeq(string(listRaw)) {
		content, err := skillgraph.ReadManifest(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "forbidphrase:", err)
			return exitToolError
		}
		for lineIdx, line := range strings.Split(string(content), "\n") {
			if pattern.MatchString(line) {
				fmt.Fprintf(out, "%s:%d: forbidden phrase matches %q\n", skillgraph.ManifestLabel(path), lineIdx+1, params["pattern"])
				violations++
			}
		}
		if require != nil && require.Match(content) {
			requireMet = true
		}
	}
	if require != nil && !requireMet {
		fmt.Fprintf(out, "%s:1: no anchored file contains required phrase %q\n", args[0], params["require"])
		violations++
	}

	if violations > 0 {
		return 1
	}
	return 0
}

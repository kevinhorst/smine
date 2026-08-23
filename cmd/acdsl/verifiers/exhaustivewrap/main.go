// Command exhaustivewrap is a registered ACDSL verifier: it fails (exit 1)
// when a switch over a typed-constant enum misses members (ACTION-IMPL-INTEG-005),
// by running the exhaustive analyzer (go.mod tool dependency) over the
// packages containing the anchored files — analyzers load packages, not loose
// files, so the wrapper derives unique package dirs from the files list.
// Exit-code normalization per the registry contract: contract-shaped output
// lines mean violations (exit 1) regardless of the analyzer's own exit code;
// a failure without contract lines is a tool error (exit 2).
//
// Contract: args = <files-list path>; one violation per stdout line as
// file:line:col: message; exit 0 pass, 1 violations, 2 error. No params.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

var contractLineRe = regexp.MustCompile(`^\S+:\d+:\d+: `)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "exhaustivewrap: usage: <files-list>")
		return 2
	}
	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "exhaustivewrap:", err)
		return 2
	}

	dirs := packageDirs(strings.Fields(string(listRaw)))
	if len(dirs) == 0 {
		return 0
	}
	output, err := shell.Run(context.Background(), ".", "go", append([]string{"tool", "exhaustive"}, dirs...)...)
	if err == nil {
		return 0
	}

	violations := 0
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if contractLineRe.MatchString(line) {
			fmt.Fprintln(out, line)
			violations++
		}
	}
	if violations > 0 {
		return 1
	}
	if strings.Contains(output, `no such tool "exhaustive"`) {
		// Distributed target without the tool dependency: an actionable
		// violation, never tool breakage that aborts the whole gate.
		fmt.Fprintf(out, "%s:1: go tool exhaustive unavailable — add the exhaustive tool dependency to go.mod or override exhaustive-switch in acdsl/registry.local.json\n", dirs[0])
		return 1
	}
	fmt.Fprintf(os.Stderr, "exhaustivewrap: %v\n%s\n", err, output)
	return 2
}

// packageDirs maps file paths to their unique ./-prefixed directories, sorted.
func packageDirs(files []string) []string {
	seen := map[string]bool{}
	for _, file := range files {
		seen["./"+filepath.Dir(file)] = true
	}
	dirs := make([]string, 0, len(seen))
	for dir := range seen {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return dirs
}

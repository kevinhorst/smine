// Command exhaustivewrap is an ACDSL verifier: a switch over a typed-constant enum names every member.
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

	"github.com/kevinhorst/smine/internal/acdsl"
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
	exhaustiveArgs := append([]string{"tool", "exhaustive"}, dirs...)
	output, err := shell.RunWithin(
		context.Background(),
		".",
		"go",
		acdsl.MaxVerifierTimeout,
		exhaustiveArgs...,
	)
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

// Command shellcheckwrap is a registered ACDSL verifier: it fails (exit 1)
// when shellcheck reports findings on an anchored script. Dep-gated (Band D,
// accepted): shellcheck is a brew binary; a missing binary is a tool error
// (exit 2), never a silent pass. -f gcc yields the contract line shape.
// Exit-code normalization per the registry contract: contract-shaped output
// lines mean violations regardless of shellcheck's own exit code.
//
// Contract: args = <files-list path>; one violation per stdout line as
// file:line:col: message; exit 0 pass, 1 violations, 2 error. No params.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

var contractLineRe = regexp.MustCompile(`^\S+:\d+:\d+: `)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "shellcheckwrap: usage: <files-list>")
		return 2
	}
	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "shellcheckwrap:", err)
		return 2
	}
	files := strings.Fields(string(listRaw))
	if len(files) == 0 {
		return 0
	}
	output, err := shell.Run(context.Background(), ".", "shellcheck", append([]string{"-f", "gcc"}, files...)...)
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
	fmt.Fprintf(os.Stderr, "shellcheckwrap: %v\n%s\n", err, output)
	return 2
}

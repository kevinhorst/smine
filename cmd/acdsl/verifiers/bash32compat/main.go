// Command bash32compat is an ACDSL verifier: a shell script uses no bash-4+ construct.
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

type bash4Pattern struct {
	re  *regexp.Regexp
	msg string
}

var bash4Patterns = []bash4Pattern{
	{regexp.MustCompile(`\bdeclare\s+-[a-zA-Z]*A`), "declare -A (associative arrays) needs bash 4"},
	{regexp.MustCompile(`\bmapfile\b`), "mapfile needs bash 4"},
	{regexp.MustCompile(`\breadarray\b`), "readarray needs bash 4"},
	{regexp.MustCompile(`\$\{[^}]*(\^\^|,,)\}`), "case expansion ${var^^}/${var,,} needs bash 4"},
	{regexp.MustCompile(`\$\{[^}]*@[QEPAa]\}`), "parameter transformation ${var@Q} needs bash 4.4"},
	{regexp.MustCompile(`\|&`), "|& pipe shorthand needs bash 4"},
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "bash32compat: usage: <files-list>")
		return 2
	}
	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "bash32compat:", err)
		return 2
	}
	violations := 0
	for _, file := range strings.Fields(string(listRaw)) {
		raw, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintln(os.Stderr, "bash32compat:", err)
			return 2
		}
		for i, line := range strings.Split(string(raw), "\n") {
			for _, pattern := range bash4Patterns {
				if pattern.re.MatchString(line) {
					fmt.Fprintf(out, "%s:%d: %s\n", file, i+1, pattern.msg)
					violations++
				}
			}
		}
	}
	if violations > 0 {
		return 1
	}
	return 0
}

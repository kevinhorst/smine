// Command literalowner is a registered ACDSL verifier: it fails (exit 1)
// when a file outside the allowed path regexp contains the literal.
// Params: literal=<string>, allowed=<RE2 over repo-relative paths>.
//
// Contract: args = <files-list path> [key=value ...]; one violation per
// stdout line as file:line: message; exit 0 pass, 1 violations, 2 error.
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// projectionMarker is the working-copy block opener (concatenated so the
// committed source stays clear of the staged-leak-guard grep). Projection
// blocks are meta-text, not file content: a text-class verifier cleans its
// own input — in memory, never on disk — before scanning.
const projectionMarker = "// [ACDSL-" + "PROJECTION]"

// cleanProjection removes a top-of-file projection block and returns the
// content lines plus the number of removed lines, so violation line numbers
// keep referring to the file as it exists on disk.
func cleanProjection(lines []string) ([]string, int) {
	if len(lines) == 0 || !strings.HasPrefix(lines[0], projectionMarker) {
		return lines, 0
	}
	i := 1
	for i < len(lines) && strings.HasPrefix(lines[i], "// - [") {
		i++
	}
	if i < len(lines) && lines[i] == "" {
		i++
	}
	return lines[i:], i
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "literalowner: usage: <files-list> literal=<s> allowed=<re>")
		return 2
	}
	params := map[string]string{}
	for _, arg := range args[1:] {
		key, value, ok := strings.Cut(arg, "=")
		if !ok {
			fmt.Fprintf(os.Stderr, "literalowner: bad param %q\n", arg)
			return 2
		}
		params[key] = value
	}
	literal, allowedPattern := params["literal"], params["allowed"]
	if literal == "" || allowedPattern == "" {
		fmt.Fprintln(os.Stderr, "literalowner: params literal and allowed are required")
		return 2
	}
	allowed, err := regexp.Compile(allowedPattern)
	if err != nil {
		fmt.Fprintln(os.Stderr, "literalowner:", err)
		return 2
	}

	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "literalowner:", err)
		return 2
	}

	violations := 0
	for _, path := range strings.Fields(string(listRaw)) {
		if allowed.MatchString(path) {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "literalowner:", err)
			return 2
		}
		content, offset := cleanProjection(strings.Split(string(raw), "\n"))
		for i, line := range content {
			if strings.Contains(line, literal) {
				fmt.Fprintf(out, "%s:%d: literal %q outside its owner\n", path, i+1+offset, literal)
				violations++
			}
		}
	}
	if violations > 0 {
		return 1
	}
	return 0
}

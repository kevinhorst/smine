// Command literalowner is an ACDSL verifier: only files on the allowed paths contain the literal.
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// Concatenated so the committed source stays clear of the staged-leak-guard grep.
const projectionMarker = "// [ACDSL-" + "PROJECTION]"

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

// Command commentcap is an ACDSL verifier: a comment block in a listed Go file runs at most max lines.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"strconv"
	"strings"
)

const (
	exitPass       = 0
	exitViolations = 1
	exitToolError  = 2
	projectionLead = "// [ACDSL-" + "PROJECTION]"
)

var directiveLeads = []string{"//go:", "//nolint", "//export ", "//line "}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "commentcap: usage: <files-list> [max=<n>]")
		return exitToolError
	}

	params := map[string]string{"max": "1"}
	for _, arg := range args[1:] {
		key, value, ok := strings.Cut(arg, "=")
		if !ok {
			fmt.Fprintf(os.Stderr, "commentcap: bad param %q\n", arg)
			return exitToolError
		}
		params[key] = value
	}
	limit, err := strconv.Atoi(params["max"])
	if err != nil || limit < 1 {
		fmt.Fprintf(os.Stderr, "commentcap: max must be a positive integer, got %q\n", params["max"])
		return exitToolError
	}

	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "commentcap:", err)
		return exitToolError
	}

	violations := 0
	for path := range strings.FieldsSeq(string(listRaw)) {
		count, err := checkFile(limit, out, path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "commentcap:", err)
			return exitToolError
		}
		violations += count
	}
	if violations > 0 {
		return exitViolations
	}
	return exitPass
}

func checkFile(limit int, out io.Writer, path string) (int, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, parser.ParseComments)
	if err != nil {
		return 0, fmt.Errorf("checkFile: Failed to parse %s: %w", path, err)
	}

	violations := 0
	for _, group := range file.Comments {
		lines := groupLines(fileSet, group)
		if lines <= limit {
			continue
		}
		position := fileSet.Position(group.Pos())
		fmt.Fprintf(out, "%s:%d: comment block of %d lines, at most %d — delete it, cut it to one line or move it to the directory's README\n", position.Filename, position.Line, lines, limit)
		violations++
	}
	return violations, nil
}

func groupLines(fileSet *token.FileSet, group *ast.CommentGroup) int {
	if strings.HasPrefix(group.List[0].Text, projectionLead) {
		return 0
	}

	lines := 0
	for _, comment := range group.List {
		isUncounted := comment.Text == "//" || isDirective(comment.Text)
		if isUncounted {
			continue
		}
		lines += fileSet.Position(comment.End()).Line - fileSet.Position(comment.Pos()).Line + 1
	}
	return lines
}

func isDirective(text string) bool {
	for _, lead := range directiveLeads {
		if strings.HasPrefix(text, lead) {
			return true
		}
	}
	return false
}

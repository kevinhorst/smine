// Command errorfprefix is a registered ACDSL verifier: it fails (exit 1)
// when a fmt.Errorf or errors.New message literal lacks the Component.Method:
// prefix (RULE-GOLANG-ERR-001 shape) — the segment before the first ": " must be a
// single identifier or dotted identifier, so "failed to read config: %w"
// (spaces before the colon) and unprefixed messages are violations.
// Non-literal first arguments are skipped (not statically checkable).
//
// Contract: args = <files-list path>; one violation per stdout line as
// file:line: message; exit 0 pass, 1 violations, 2 error. No params.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var prefixRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*(\.[A-Za-z][A-Za-z0-9_]*)?: `)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "errorfprefix: usage: <files-list>")
		return 2
	}
	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "errorfprefix:", err)
		return 2
	}

	violations := 0
	for _, path := range strings.Fields(string(listRaw)) {
		fileSet := token.NewFileSet()
		file, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			fmt.Fprintln(os.Stderr, "errorfprefix:", err)
			return 2
		}
		fmtAlias := importAlias(file, "fmt")
		errorsAlias := importAlias(file, "errors")
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			isConstructor := (ident.Name == fmtAlias && selector.Sel.Name == "Errorf") ||
				(ident.Name == errorsAlias && selector.Sel.Name == "New")
			if !isConstructor || len(call.Args) == 0 {
				return true
			}
			literal, ok := call.Args[0].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			message, err := strconv.Unquote(literal.Value)
			if err != nil || prefixRe.MatchString(message) {
				return true
			}
			position := fileSet.Position(call.Pos())
			fmt.Fprintf(out, "%s:%d: error message lacks the Component.Method: prefix\n", position.Filename, position.Line)
			violations++
			return true
		})
	}
	if violations > 0 {
		return 1
	}
	return 0
}

// importAlias returns the identifier the file uses for pkgPath, or "" when
// not imported; dot and blank imports are out of scope.
func importAlias(file *ast.File, pkgPath string) string {
	for _, imported := range file.Imports {
		if strings.Trim(imported.Path.Value, `"`) != pkgPath {
			continue
		}
		if imported.Name != nil {
			if imported.Name.Name == "." || imported.Name.Name == "_" {
				return ""
			}
			return imported.Name.Name
		}
		parts := strings.Split(strings.Trim(imported.Path.Value, `"`), "/")
		return parts[len(parts)-1]
	}
	return ""
}

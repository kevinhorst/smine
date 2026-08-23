// Command forbidcall is a registered ACDSL verifier: it fails (exit 1)
// when any file in the files list calls <pkg>.<func>, resolving import
// aliases per file. Params: pkg=<import path>, func=<name>.
//
// Contract: args = <files-list path> [key=value ...]; one violation per
// stdout line as file:line: message; exit 0 pass, 1 violations, 2 error.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "forbidcall: usage: <files-list> pkg=<path> func=<name>")
		return 2
	}
	params := map[string]string{}
	for _, arg := range args[1:] {
		key, value, ok := strings.Cut(arg, "=")
		if !ok {
			fmt.Fprintf(os.Stderr, "forbidcall: bad param %q\n", arg)
			return 2
		}
		params[key] = value
	}
	pkgPath, fn := params["pkg"], params["func"]
	if pkgPath == "" || fn == "" {
		fmt.Fprintln(os.Stderr, "forbidcall: params pkg and func are required")
		return 2
	}

	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "forbidcall:", err)
		return 2
	}

	violations := 0
	for _, path := range strings.Fields(string(listRaw)) {
		fileSet := token.NewFileSet()
		file, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			fmt.Fprintln(os.Stderr, "forbidcall:", err)
			return 2
		}
		alias := importAlias(file, pkgPath)
		if alias == "" {
			continue
		}
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
			if !ok || ident.Name != alias || selector.Sel.Name != fn {
				return true
			}
			position := fileSet.Position(call.Pos())
			fmt.Fprintf(out, "%s:%d: call to %s.%s is forbidden here\n", position.Filename, position.Line, pkgPath, fn)
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
// the file does not import it. Dot and blank imports return "" — a dot
// import cannot be matched syntactically and is out of v0 scope.
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

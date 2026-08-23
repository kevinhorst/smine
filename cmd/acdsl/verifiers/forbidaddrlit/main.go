// Command forbidaddrlit is a registered ACDSL verifier: it fails (exit 1)
// when any file in the files list takes the address of a composite literal
// (&T{...}) — the pre-Go-1.26 pointer-initialization idiom that new(expr)
// replaces. Taking the address of a variable (&v) is out of scope: the rule
// targets the literal idiom, not addressability in general.
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
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "forbidaddrlit: usage: <files-list>")
		return 2
	}
	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "forbidaddrlit:", err)
		return 2
	}

	violations := 0
	for _, path := range strings.Fields(string(listRaw)) {
		fileSet := token.NewFileSet()
		file, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			fmt.Fprintln(os.Stderr, "forbidaddrlit:", err)
			return 2
		}
		ast.Inspect(file, func(node ast.Node) bool {
			unary, ok := node.(*ast.UnaryExpr)
			if !ok || unary.Op != token.AND {
				return true
			}
			if _, isLit := unary.X.(*ast.CompositeLit); !isLit {
				return true
			}
			position := fileSet.Position(unary.Pos())
			fmt.Fprintf(out, "%s:%d: address-of composite literal is forbidden here\n", position.Filename, position.Line)
			violations++
			return true
		})
	}
	if violations > 0 {
		return 1
	}
	return 0
}

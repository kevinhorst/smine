// Command testschema is an ACDSL verifier: a test file runs table cases with t.Run(test._id, ...).
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
		fmt.Fprintln(os.Stderr, "testschema: usage: <files-list>")
		return 2
	}
	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "testschema:", err)
		return 2
	}

	violations := 0
	for _, path := range strings.Fields(string(listRaw)) {
		fileSet := token.NewFileSet()
		file, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			fmt.Fprintln(os.Stderr, "testschema:", err)
			return 2
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
			receiver, ok := selector.X.(*ast.Ident)
			if !ok || receiver.Name != "t" || selector.Sel.Name != "Run" || len(call.Args) == 0 {
				return true
			}
			if nameArg, ok := call.Args[0].(*ast.SelectorExpr); ok && nameArg.Sel.Name == "_id" {
				return true
			}
			position := fileSet.Position(call.Pos())
			fmt.Fprintf(out, "%s:%d: t.Run case name is not the case's _id field (house table-test schema)\n", position.Filename, position.Line)
			violations++
			return true
		})
	}
	if violations > 0 {
		return 1
	}
	return 0
}

// Command funcsig is an ACDSL verifier: a function declaration follows the house signature rule.
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
		fmt.Fprintln(os.Stderr, "funcsig: usage: <files-list>")
		return 2
	}
	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "funcsig:", err)
		return 2
	}

	violations := 0
	for _, path := range strings.Fields(string(listRaw)) {
		fileSet := token.NewFileSet()
		file, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			fmt.Fprintln(os.Stderr, "funcsig:", err)
			return 2
		}
		contextAlias := importAlias(file, "context")
		ast.Inspect(file, func(node ast.Node) bool {
			decl, ok := node.(*ast.FuncDecl)
			if !ok {
				return true
			}
			for _, message := range checkSignature(decl, contextAlias) {
				position := fileSet.Position(decl.Pos())
				fmt.Fprintf(out, "%s:%d: %s\n", position.Filename, position.Line, message)
				violations++
			}
			return true
		})
	}
	if violations > 0 {
		return 1
	}
	return 0
}

// checkSignature returns the signature findings for one declaration.
func checkSignature(decl *ast.FuncDecl, contextAlias string) []string {
	var messages []string
	for fieldIndex, field := range decl.Type.Params.List {
		if !isContextType(field.Type, contextAlias) {
			continue
		}
		if fieldIndex != 0 {
			messages = append(messages, decl.Name.Name+": context.Context must be the first parameter")
		}
		for _, name := range field.Names {
			if name.Name != "ctx" {
				messages = append(messages, decl.Name.Name+": context.Context parameter must be named ctx")
			}
		}
	}
	results := decl.Type.Results
	if results == nil {
		return messages
	}
	if count := results.NumFields(); count > 3 {
		messages = append(messages, fmt.Sprintf("%s: %d return values — at most 3 including error", decl.Name.Name, count))
	}
	for fieldIndex, field := range results.List {
		ident, ok := field.Type.(*ast.Ident)
		if !ok || ident.Name != "error" {
			continue
		}
		if fieldIndex != len(results.List)-1 {
			messages = append(messages, decl.Name.Name+": error must be the last return value")
		}
	}
	return messages
}

func isContextType(expr ast.Expr, contextAlias string) bool {
	if contextAlias == "" {
		return false
	}
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	return ok && ident.Name == contextAlias && selector.Sel.Name == "Context"
}

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

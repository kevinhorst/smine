// Command funcsig is a registered ACDSL verifier: it fails (exit 1) when a
// function declaration violates RULE-GOLANG-FUNC-001 — a context.Context parameter
// must be the first parameter and be named ctx, an error result must be the
// last result, and a signature has at most 3 results. Scope: FuncDecls only;
// function literals and interface method sets are out of v1 scope. Unnamed
// context parameters are not flagged for naming (lenient).
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

// checkSignature returns the RULE-GOLANG-FUNC-001 findings for one declaration.
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

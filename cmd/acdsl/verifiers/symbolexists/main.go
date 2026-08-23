// Command symbolexists checks a task contract's planned symbol: the named
// func/method/type/var/const exists in the anchored files, optionally with
// the planned signature (whitespace-normalized text compare — plan-level
// precision, no type resolution).
//
// Contract: args = <files-list path> symbol=<name|Recv.Method> [signature=<text>];
// one violation per stdout line as file:line: message; exit 0 pass, 1
// violations, 2 error.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"os"
	"strings"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout)) }

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "symbolexists: usage: <files-list> symbol=<name> [signature=<text>]")
		return 2
	}
	params := parseParams(args[1:])
	symbol := params["symbol"]
	if symbol == "" {
		fmt.Fprintln(os.Stderr, "symbolexists: symbol= is required")
		return 2
	}
	files, err := readLines(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "symbolexists:", err)
		return 2
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "symbolexists: empty files list")
		return 2
	}

	recv, name := splitSymbol(symbol)
	for _, path := range files {
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			fmt.Fprintln(os.Stderr, "symbolexists:", err)
			return 2
		}
		decl := findDecl(parsed, recv, name)
		if decl == nil {
			continue
		}
		if want := params["signature"]; want != "" {
			got := renderSignature(fset, decl)
			if got == "" {
				fmt.Fprintf(out, "%s:%d: symbol %q is not a func — signature check needs one\n", path, fset.Position(decl.Pos()).Line, symbol)
				return 1
			}
			if normalize(got) != normalize(want) {
				fmt.Fprintf(out, "%s:%d: symbol %q signature mismatch: got %s, planned %s\n", path, fset.Position(decl.Pos()).Line, symbol, normalize(got), normalize(want))
				return 1
			}
		}
		return 0
	}
	fmt.Fprintf(out, "%s:1: symbol %q not found in anchored files\n", files[0], symbol)
	return 1
}

// findDecl returns the declaration node for the symbol, or nil.
func findDecl(file *ast.File, recv, name string) ast.Decl {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name.Name == name && receiverBase(d) == recv {
				return d
			}
		case *ast.GenDecl:
			if recv != "" {
				continue
			}
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if s.Name.Name == name {
						return d
					}
				case *ast.ValueSpec:
					for _, ident := range s.Names {
						if ident.Name == name {
							return d
						}
					}
				}
			}
		}
	}
	return nil
}

// receiverBase names the receiver's base type ("" for plain functions).
func receiverBase(d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return ""
	}
	expr := d.Recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// renderSignature prints a func declaration without body or doc — the text a
// plan declares. Non-funcs render empty.
func renderSignature(fset *token.FileSet, decl ast.Decl) string {
	fn, ok := decl.(*ast.FuncDecl)
	if !ok {
		return ""
	}
	trimmed := *fn
	trimmed.Body = nil
	trimmed.Doc = nil
	var builder strings.Builder
	if err := printer.Fprint(&builder, fset, &trimmed); err != nil {
		return ""
	}
	return builder.String()
}

// normalize collapses whitespace and the go/printer artifacts a multiline
// declaration leaves behind (space inside parens, trailing comma), so the
// on-disk layout never fails a matching planned signature.
func normalize(signature string) string {
	collapsed := strings.Join(strings.Fields(signature), " ")
	collapsed = strings.ReplaceAll(collapsed, "( ", "(")
	collapsed = strings.ReplaceAll(collapsed, " )", ")")
	collapsed = strings.ReplaceAll(collapsed, ",)", ")")
	return collapsed
}

func splitSymbol(symbol string) (recv, name string) {
	if at := strings.IndexByte(symbol, '.'); at >= 0 {
		return symbol[:at], symbol[at+1:]
	}
	return "", symbol
}

func parseParams(args []string) map[string]string {
	params := map[string]string{}
	for _, arg := range args {
		if key, value, found := strings.Cut(arg, "="); found {
			params[key] = value
		}
	}
	return params
}

func readLines(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, line := range strings.Split(string(raw), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines, nil
}

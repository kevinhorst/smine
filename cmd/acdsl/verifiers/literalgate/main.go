// Command literalgate is an ACDSL verifier: hook packages hold no behavioural literal outside protocol.go.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"regexp"
	"strings"
)

// exitUsage is the verifier's exit code for a usage or input error.
const exitUsage = 2

// trivial are the numeric literals every file may hold.
var trivial = map[string]bool{"0": true, "1": true}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "literalgate: usage: <files-list> [exempt=<re>] [ignored=<n,n>]")
		return exitUsage
	}
	params := map[string]string{}
	for _, arg := range args[1:] {
		key, value, ok := strings.Cut(arg, "=")
		if !ok {
			fmt.Fprintf(os.Stderr, "literalgate: bad param %q\n", arg)
			return exitUsage
		}
		params[key] = value
	}
	exemptPattern := params["exempt"]
	if exemptPattern == "" {
		exemptPattern = `(^|/)protocol\.go$`
	}
	exempt, err := regexp.Compile(exemptPattern)
	if err != nil {
		fmt.Fprintln(os.Stderr, "literalgate:", err)
		return exitUsage
	}
	ignored := map[string]bool{}
	for number := range strings.SplitSeq(params["ignored"], ",") {
		if number != "" {
			ignored[number] = true
		}
	}

	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "literalgate:", err)
		return exitUsage
	}

	violations := 0
	for path := range strings.FieldsSeq(string(listRaw)) {
		if exempt.MatchString(path) {
			continue
		}
		findings, err := check(path, ignored)
		if err != nil {
			fmt.Fprintln(os.Stderr, "literalgate:", err)
			return exitUsage
		}
		for _, finding := range findings {
			fmt.Fprintln(out, finding)
			violations++
		}
	}
	if violations > 0 {
		return 1
	}
	return 0
}

// check parses one file and reports its literals: package-level declarations with a literal value, numeric literals in bodies.
func check(path string, ignored map[string]bool) ([]string, error) {
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("check: %w", err)
	}

	var findings []string
	for _, declaration := range file.Decls {
		switch node := declaration.(type) {
		case *ast.GenDecl:
			if node.Tok != token.CONST && node.Tok != token.VAR {
				continue
			}
			for _, spec := range node.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, expr := range value.Values {
					if isLiteralValue(expr) {
						position := set.Position(expr.Pos())
						findings = append(findings, fmt.Sprintf("%s:%d: package-level literal — a catalog row (internal/components) or protocol.go", path, position.Line))
					}
				}
			}
		case *ast.FuncDecl:
			if node.Body == nil {
				continue
			}
			ast.Inspect(node.Body, func(inner ast.Node) bool {
				literal, ok := inner.(*ast.BasicLit)
				isNumber := ok && (literal.Kind == token.INT || literal.Kind == token.FLOAT)
				if !isNumber || trivial[literal.Value] || ignored[literal.Value] {
					return true
				}
				position := set.Position(literal.Pos())
				findings = append(findings, fmt.Sprintf("%s:%d: numeric literal %s — a catalog row (internal/components)", path, position.Line, literal.Value))
				return true
			})
		}
	}
	return findings, nil
}

// isLiteralValue reports a basic literal, a composite of basic literals, or a unary minus over one.
func isLiteralValue(expr ast.Expr) bool {
	switch node := expr.(type) {
	case *ast.BasicLit:
		return true
	case *ast.UnaryExpr:
		return isLiteralValue(node.X)
	case *ast.CompositeLit:
		for _, element := range node.Elts {
			if value, ok := element.(*ast.KeyValueExpr); ok {
				element = value.Value
			}
			if !isLiteralValue(element) {
				return false
			}
		}
		return len(node.Elts) > 0
	case *ast.BinaryExpr:
		return isLiteralValue(node.X) && isLiteralValue(node.Y)
	}
	return false
}

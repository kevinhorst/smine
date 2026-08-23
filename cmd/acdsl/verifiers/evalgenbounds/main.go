// Command evalgenbounds is a registered ACDSL verifier: it fails (exit 1)
// when a verifier main.go leaves the acdsl/evalgen.json bounds — line
// count over maxLines, or an import outside the policy (Go stdlib, the
// module's internal packages, or a module already in go.mod's require set).
// Uniform over handwritten and agent-generated scripts: generation may never
// add a dependency (Band D stays a user call), and the gate — not prompt
// discipline — enforces it. Missing config is a tool error, never a silent
// pass.
//
// Contract: args = <files-list path>; one violation per stdout line as
// file:line: message; exit 0 pass, 1 violations, 2 error. No params.
package main

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"strings"
)

const configPath = "acdsl/evalgen.json"

type bounds struct {
	MaxLines int `json:"maxLines"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "evalgenbounds: usage: <files-list>")
		return 2
	}
	rawConfig, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "evalgenbounds:", err)
		return 2
	}
	var config bounds
	if err := json.Unmarshal(rawConfig, &config); err != nil || config.MaxLines < 1 {
		fmt.Fprintf(os.Stderr, "evalgenbounds: %s unparseable or maxLines missing\n", configPath)
		return 2
	}
	module, requires, err := goModAllowlist("go.mod")
	if err != nil {
		fmt.Fprintln(os.Stderr, "evalgenbounds:", err)
		return 2
	}
	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "evalgenbounds:", err)
		return 2
	}
	violations := 0
	for _, file := range strings.Fields(string(listRaw)) {
		raw, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintln(os.Stderr, "evalgenbounds:", err)
			return 2
		}
		violations += checkLines(file, string(raw), config.MaxLines, out)
		count, err := checkImports(file, raw, module, requires, out)
		if err != nil {
			fmt.Fprintln(os.Stderr, "evalgenbounds:", err)
			return 2
		}
		violations += count
	}
	if violations > 0 {
		return 1
	}
	return 0
}

func checkLines(file, content string, maxLines int, out io.Writer) int {
	nonEmpty := 0
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) != "" {
			nonEmpty++
		}
	}
	if nonEmpty > maxLines {
		fmt.Fprintf(out, "%s:1: %d non-empty lines exceed the evalgen.json bound of %d\n", file, nonEmpty, maxLines)
		return 1
	}
	return 0
}

// checkImports flags every import outside the policy: stdlib (first path
// segment carries no dot), <module>/internal/..., or a go.mod require
// module. Everything else would be a new dependency.
func checkImports(file string, raw []byte, module string, requires []string, out io.Writer) (int, error) {
	parsed, err := parser.ParseFile(token.NewFileSet(), file, raw, parser.ImportsOnly)
	if err != nil {
		return 0, fmt.Errorf("checkImports: %w", err)
	}
	violations := 0
	for _, spec := range parsed.Imports {
		path := strings.Trim(spec.Path.Value, `"`)
		if allowedImport(path, module, requires) {
			continue
		}
		fmt.Fprintf(out, "%s:1: import %q is outside the evalgen policy (stdlib, %s/internal, go.mod requires)\n", file, path, module)
		violations++
	}
	return violations, nil
}

func allowedImport(path, module string, requires []string) bool {
	first, _, _ := strings.Cut(path, "/")
	if !strings.Contains(first, ".") {
		return true // stdlib
	}
	if strings.HasPrefix(path, module+"/internal/") {
		return true
	}
	for _, required := range requires {
		if path == required || strings.HasPrefix(path, required+"/") {
			return true
		}
	}
	return false
}

// goModAllowlist reads the module path and every require entry (single-line
// and block form) line-wise — deliberately dep-free.
func goModAllowlist(path string) (string, []string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("goModAllowlist: %w", err)
	}
	module := ""
	var requires []string
	inBlock := false
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "module "):
			module = strings.TrimSpace(strings.TrimPrefix(trimmed, "module "))
		case trimmed == "require (":
			inBlock = true
		case inBlock && trimmed == ")":
			inBlock = false
		case inBlock, strings.HasPrefix(trimmed, "require "):
			entry := strings.TrimSpace(strings.TrimPrefix(trimmed, "require "))
			if fields := strings.Fields(entry); len(fields) >= 2 {
				requires = append(requires, fields[0])
			}
		}
	}
	if module == "" {
		return "", nil, fmt.Errorf("goModAllowlist: no module line in %s", path)
	}
	return module, requires, nil
}

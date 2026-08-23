// Command jsonschemacheck is a registered ACDSL verifier: it fails (exit 1)
// when an anchored JSON file does not conform to the schema named by the
// schema= param (repo-relative path, draft-07). One generic entry serves
// every schema-backed JSON rule.
//
// Contract: args = <files-list path> schema=<path>; one violation per stdout
// line as file:line: message; exit 0 pass, 1 violations, 2 error.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "jsonschemacheck: usage: <files-list> schema=<path>")
		return 2
	}
	schemaPath := ""
	for _, arg := range args[1:] {
		if value, ok := strings.CutPrefix(arg, "schema="); ok {
			schemaPath = value
		}
	}
	if schemaPath == "" {
		fmt.Fprintln(os.Stderr, "jsonschemacheck: schema= param required")
		return 2
	}
	compiler := jsonschema.NewCompiler()
	schema, err := compiler.Compile(schemaPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "jsonschemacheck:", err)
		return 2
	}
	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "jsonschemacheck:", err)
		return 2
	}
	violations := 0
	for _, file := range strings.Fields(string(listRaw)) {
		handle, err := os.Open(file)
		if err != nil {
			fmt.Fprintln(os.Stderr, "jsonschemacheck:", err)
			return 2
		}
		instance, err := jsonschema.UnmarshalJSON(handle)
		handle.Close()
		if err != nil {
			fmt.Fprintf(out, "%s:1: not valid JSON: %v\n", file, err)
			violations++
			continue
		}
		if err := schema.Validate(instance); err != nil {
			for _, line := range strings.Split(err.Error(), "\n") {
				if line = strings.TrimSpace(line); line != "" {
					fmt.Fprintf(out, "%s:1: %s\n", file, line)
					violations++
				}
			}
		}
	}
	if violations > 0 {
		return 1
	}
	return 0
}

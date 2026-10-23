// Command jsonschemacheck is an ACDSL verifier: an anchored JSON file conforms to the schema= schema.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kevinhorst/smine/internal/jsonschemacheck"
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
	schema, err := jsonschemacheck.NewSchema(schemaPath)
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
		lines, err := schema.Violations(file)
		if err != nil {
			fmt.Fprintln(os.Stderr, "jsonschemacheck:", err)
			return 2
		}
		for _, line := range lines {
			fmt.Fprintln(out, line)
		}
		violations += len(lines)
	}
	if violations > 0 {
		return 1
	}
	return 0
}

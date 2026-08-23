// Command idgrammar is a registered ACDSL verifier: it fails (exit 1) when
// a doctrine declaration's ID leaves the shared identity grammar —
// KIND-SCOPE[-TOPIC]-NNN, with SCOPE and TOPIC registered in the taxonomy
// (the generated context file's aspects carrying class "scope" / "topic"). Task-lifetime
// entries are exempt: contracts are ephemeral and free-form. The naming
// doctrine as a gate, not review vigilance.
//
// Contract: args = <files-list path> taxonomy=<generated context file path>; one
// violation per stdout line as file:line: message; exit 0 pass, 1
// violations, 2 error.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

var idShapeRe = regexp.MustCompile(`^(ACDSL|RULE|FACT|ACTION)-([A-Z]{2,12})(?:-([A-Z]{2,12}))?-(\d{3})$`)

var markerPrefix = "//" + "acdsl:"

// taxonomyAspect is one aspect entry in the generated context file: a name and
// its class ("scope" / "topic"), the vocabulary loadTaxonomy indexes.
type taxonomyAspect struct {
	Name  string `json:"name"`
	Class string `json:"class"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "idgrammar: usage: <files-list> taxonomy=<path>")
		return 2
	}
	taxonomyPath := ""
	for _, arg := range args[1:] {
		if value, ok := strings.CutPrefix(arg, "taxonomy="); ok {
			taxonomyPath = value
		}
	}
	if taxonomyPath == "" {
		fmt.Fprintln(os.Stderr, "idgrammar: taxonomy= param required")
		return 2
	}
	scopes, topics, err := loadTaxonomy(taxonomyPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "idgrammar:", err)
		return 2
	}
	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "idgrammar:", err)
		return 2
	}
	violations := 0
	for _, file := range strings.Fields(string(listRaw)) {
		raw, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintln(os.Stderr, "idgrammar:", err)
			return 2
		}
		for i, line := range strings.Split(string(raw), "\n") {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, markerPrefix) {
				continue
			}
			if strings.Contains(trimmed, `lifetime="task"`) {
				continue
			}
			fields := strings.Fields(strings.TrimPrefix(trimmed, markerPrefix))
			if len(fields) == 0 {
				continue
			}
			violations += checkId(file, i+1, fields[0], scopes, topics, out)
		}
	}
	if violations > 0 {
		return 1
	}
	return 0
}

func checkId(file string, lineNo int, id string, scopes, topics map[string]bool, out io.Writer) int {
	match := idShapeRe.FindStringSubmatch(id)
	if match == nil {
		fmt.Fprintf(out, "%s:%d: id %q is not KIND-SCOPE[-TOPIC]-NNN (KIND ∈ ACDSL|RULE|FACT|ACTION, 3-digit number)\n", file, lineNo, id)
		return 1
	}
	violations := 0
	if !scopes[match[2]] {
		fmt.Fprintf(out, "%s:%d: id %q scope %q is not a registered class-scope taxonomy entry\n", file, lineNo, id, match[2])
		violations++
	}
	if match[3] != "" && !topics[match[3]] {
		fmt.Fprintf(out, "%s:%d: id %q topic %q is not a registered class-topic taxonomy entry\n", file, lineNo, id, match[3])
		violations++
	}
	return violations
}

// loadTaxonomy reads the vocabulary: the generated context file's aspects carrying class
// "scope" and "topic". Missing file or empty vocabularies are tool errors —
// the gate never silently passes.
func loadTaxonomy(path string) (map[string]bool, map[string]bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("loadTaxonomy: %w", err)
	}
	var parsed struct {
		Aspects []taxonomyAspect `json:"aspects"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, nil, fmt.Errorf("loadTaxonomy: %w", err)
	}
	entries := parsed.Aspects
	scopes, topics := map[string]bool{}, map[string]bool{}
	for _, entry := range entries {
		switch entry.Class {
		case "scope":
			scopes[entry.Name] = true
		case "topic":
			topics[entry.Name] = true
		}
	}
	if len(scopes) == 0 {
		return nil, nil, fmt.Errorf("loadTaxonomy: %s carries no class-scope entries", path)
	}
	return scopes, topics, nil
}

// Command modeltoken is the ACDSL verifier that resolves selected model tokens against availableModels.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/models"
	"github.com/kevinhorst/smine/internal/skillgraph"
)

const (
	exitPass       = 0
	exitViolations = 1
	exitToolError  = 2
)

var (
	idPattern      = regexp.MustCompile(`claude-(?:opus|sonnet|haiku|fable)-[0-9][a-z0-9.-]*(?:\[1m\])?`)
	literalPattern = regexp.MustCompile(`^(?:claude-[a-z0-9.-]+|[a-z][a-z0-9-]*)(?:\[1m\])?`)
	nonLiteral     = regexp.MustCompile(`[.()${}<>A-Z_]`)
)

type params struct {
	isIds       bool
	isSubagent  bool
	keyPattern  *regexp.Regexp
	keys        map[string]bool
	linePattern *regexp.Regexp
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout))
}

func run(args []string, out io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "modeltoken: usage: <files-list> [role=session|subagent] [keys=a,b] [ids=true] [line=<RE2>]")
		return exitToolError
	}

	parsed, err := parseParams(args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "modeltoken:", err)
		return exitToolError
	}

	vocabulary, err := models.LoadTemplates(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "modeltoken:", err)
		return exitToolError
	}

	listRaw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "modeltoken:", err)
		return exitToolError
	}

	violations := 0
	for path := range strings.FieldsSeq(string(listRaw)) {
		count, err := checkFile(path, parsed, vocabulary, out)
		if err != nil {
			fmt.Fprintln(os.Stderr, "modeltoken:", err)
			return exitToolError
		}
		violations += count
	}
	if violations > 0 {
		return exitViolations
	}
	return exitPass
}

func checkFile(path string, parsed *params, vocabulary *models.Vocabulary, out io.Writer) (int, error) {
	content, err := skillgraph.ReadManifest(path)
	if err != nil {
		return 0, fmt.Errorf("checkFile: Failed to read %s: %w", path, err)
	}

	label := skillgraph.ManifestLabel(path)
	violations := 0
	for index, line := range strings.Split(string(content), "\n") {
		isSkipped := parsed.linePattern != nil && !parsed.linePattern.MatchString(line)
		if isSkipped {
			continue
		}
		for _, token := range lineTokens(line, parsed) {
			if err := checkToken(vocabulary, token, parsed.isSubagent); err != nil {
				fmt.Fprintf(out, "%s:%d: %v\n", label, index+1, err)
				violations++
			}
		}
	}

	isDecoded := strings.HasSuffix(path, ".json") && !parsed.isIds
	if !isDecoded {
		return violations, nil
	}
	var document any
	if err := json.Unmarshal(content, &document); err != nil {
		return 0, fmt.Errorf("checkFile: Failed to decode %s: %w", path, err)
	}
	for _, value := range keyedValues(document, parsed.keys, nil) {
		for _, token := range valueTokens(value) {
			if err := checkToken(vocabulary, token, parsed.isSubagent); err != nil {
				fmt.Fprintf(out, "%s:%d: %v\n", label, lineOf(string(content), value), err)
				violations++
			}
		}
	}
	return violations, nil
}

func keyedValues(node any, keys map[string]bool, values []string) []string {
	switch typed := node.(type) {
	case map[string]any:
		name, _ := typed["name"].(string)
		if keys[name] {
			values = append(values, stringsOf(typed["value"])...)
		}
		for _, key := range slices.Sorted(maps.Keys(typed)) {
			if keys[key] {
				values = append(values, stringsOf(typed[key])...)
			}
			values = keyedValues(typed[key], keys, values)
		}
	case []any:
		for _, member := range typed {
			values = keyedValues(member, keys, values)
		}
	}
	return values
}

func lineOf(content string, value string) int {
	before, _, isFound := strings.Cut(content, value)
	if !isFound {
		return 1
	}
	return strings.Count(before, "\n") + 1
}

func stringsOf(node any) []string {
	switch typed := node.(type) {
	case string:
		return []string{typed}
	case []any:
		values := make([]string, 0, len(typed))
		for _, member := range typed {
			if text, isText := member.(string); isText {
				values = append(values, text)
			}
		}
		return values
	}
	return nil
}

func checkToken(vocabulary *models.Vocabulary, token string, isSubagent bool) error {
	if isSubagent {
		return vocabulary.CheckSubagent(token)
	}
	return vocabulary.Check(token)
}

func lineTokens(line string, parsed *params) []string {
	if parsed.isIds {
		return idPattern.FindAllString(line, -1)
	}
	tokens := make([]string, 0)
	for _, match := range parsed.keyPattern.FindAllStringIndex(line, -1) {
		tokens = append(tokens, valueTokens(line[match[1]:])...)
	}
	return tokens
}

func parseParams(args []string) (*params, error) {
	parsed := &params{keys: map[string]bool{}}
	keys := ""
	for _, arg := range args {
		name, value, ok := strings.Cut(arg, "=")
		if !ok {
			return nil, fmt.Errorf("parseParams: Bad param %q", arg)
		}
		switch name {
		case "role":
			parsed.isSubagent = value == "subagent"
		case "keys":
			keys = value
		case "ids":
			parsed.isIds = value == "true"
		case "line":
			pattern, err := regexp.Compile(value)
			if err != nil {
				return nil, fmt.Errorf("parseParams: Invalid line pattern: %w", err)
			}
			parsed.linePattern = pattern
		}
	}
	if parsed.isIds {
		return parsed, nil
	}
	if keys == "" {
		return nil, errors.New("parseParams: keys= or ids=true is required")
	}

	quoted := make([]string, 0)
	for key := range strings.SplitSeq(keys, ",") {
		parsed.keys[strings.TrimSpace(key)] = true
		quoted = append(quoted, regexp.QuoteMeta(strings.TrimSpace(key)))
	}
	parsed.keyPattern = regexp.MustCompile(`(?:^|[\s"'{,(])(?:` + strings.Join(quoted, "|") + `)\s*:\s*`)
	return parsed, nil
}

// valueTokens reads a key's first field plus comma-continued fields, stopping at the next `key:` field.
func valueTokens(value string) []string {
	tokens := make([]string, 0)
	for field := range strings.FieldsSeq(value) {
		if strings.HasSuffix(field, ":") {
			break
		}
		for item := range strings.SplitSeq(field, ",") {
			if token := itemToken(item); token != "" {
				tokens = append(tokens, token)
			}
		}
		if !strings.HasSuffix(field, ",") {
			break
		}
	}
	return tokens
}

// itemToken is one list item's literal model token; "" for a non-literal item.
func itemToken(item string) string {
	item, _, _ = strings.Cut(item, ":")
	item, _, _ = strings.Cut(item, "+")
	item = strings.Trim(item, `"'[]`)
	if _, rest, hasCount := strings.Cut(item, "["); hasCount && !strings.HasPrefix(rest, "1m") {
		item = rest
	}
	if nonLiteral.MatchString(item) {
		return ""
	}
	return literalPattern.FindString(item)
}

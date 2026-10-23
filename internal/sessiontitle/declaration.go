package sessiontitle

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"regexp"
	"slices"
	"unicode/utf8"
)

// FileName is the rendered title declaration beside a SKILL.md.
const FileName = "title.json"

// DocumentSchema is the schema number of a rendered title.json.
const DocumentSchema = 2

// tokenPattern is a declared state: uppercase words joined by hyphens.
var tokenPattern = regexp.MustCompile(`^[A-Z]+(-[A-Z]+)*$`)

type Declaration struct {
	States map[string]*DeclaredState `json:"states"`
}

// Tokens returns the declared states in token order.
func (d *Declaration) Tokens() []State {
	tokens := make([]State, 0, len(d.States))
	for _, token := range slices.Sorted(maps.Keys(d.States)) {
		tokens = append(tokens, State(token))
	}
	return tokens
}

// Violations covers only what the schema cannot express.
func (d *Declaration) Violations(limits *Limits) []string {
	var violations []string
	if len(d.States) > limits.MaxStates {
		violations = append(violations, fmt.Sprintf("title: %d states, at most %d", len(d.States), limits.MaxStates))
	}
	for _, state := range d.Tokens() {
		violations = append(violations, tokenViolations(limits.MaxTokenRunes, state, "title: state "+string(state))...)
	}
	return violations
}

type DeclaredState struct {
	Entry   string `json:"entry"`
	Meaning string `json:"meaning"`
}

type Document struct {
	*Declaration

	Schema  int    `json:"schema"`
	Skill   string `json:"skill"`
	Version string `json:"version"`
}

// SetCommand is the command a gate step runs to put the state into the title at the turn end.
func SetCommand(state State) string {
	return "smine-rules title set " + string(state)
}

// SetOutput is the line SetCommand prints on success; the hook records the state only when it sees it.
func SetOutput(state State) string {
	return "title: set " + string(state)
}

// tokenViolations checks one declared token: grammar, cap, no built-in.
func tokenViolations(maxTokenRunes int, state State, subject string) []string {
	var violations []string
	if !tokenPattern.MatchString(string(state)) {
		violations = append(violations, subject+": token is not uppercase words joined by hyphens")
	}
	if utf8.RuneCountInString(string(state)) > maxTokenRunes {
		violations = append(violations, fmt.Sprintf("%s: token exceeds %d runes", subject, maxTokenRunes))
	}
	if state.IsBuiltin() {
		violations = append(violations, subject+": token is a built-in state")
	}
	return violations
}

// LoadDocument reads one title.json; a missing file is fs.ErrNotExist.
func LoadDocument(path string) (*Document, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("LoadDocument: Failed to read %s: %w", path, err)
	}

	document, err := ParseDocument(content)
	if err != nil {
		return nil, fmt.Errorf("LoadDocument: %s: %w", path, err)
	}
	return document, nil
}

func ParseDocument(content []byte) (*Document, error) {
	document := &Document{}
	if err := json.Unmarshal(content, document); err != nil {
		return nil, fmt.Errorf("ParseDocument: Invalid title declaration: %w", err)
	}
	if document.Schema != DocumentSchema {
		return nil, fmt.Errorf("ParseDocument: Unsupported schema %d", document.Schema)
	}
	if document.Declaration == nil {
		return nil, errors.New("ParseDocument: No states declared")
	}
	return document, nil
}

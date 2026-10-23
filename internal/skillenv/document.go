package skillenv

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// FileName is the rendered repo-variable declaration beside a SKILL.md.
const FileName = "env.json"

// DocumentSchema is the schema number of a rendered env.json.
const DocumentSchema = 1

// Document is a deployed skill's env.json: the repo-scoped variables its rendered body's placeholders read.
type Document struct {
	Schema    int      `json:"schema"`
	Skill     string   `json:"skill"`
	Variables []*Entry `json:"variables"`
	Version   string   `json:"version"`
}

// Entry is one variable the body reads; Interface is the catalog id its global default is keyed by.
type Entry struct {
	Example   string `json:"example"`
	Interface string `json:"interface"`
	Name      string `json:"name"`
	Required  bool   `json:"required"`
	Scope     string `json:"scope"`
	Summary   string `json:"summary"`
}

// Bindings maps each listed variable to the interface id its global default is keyed by.
func Bindings(documents []*Document) map[string]string {
	bindings := map[string]string{}
	for _, document := range documents {
		for _, entry := range document.Variables {
			bindings[entry.Name] = entry.Interface
		}
	}
	return bindings
}

// LoadDocument reads one env.json; a missing file is fs.ErrNotExist.
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

// LoadDocuments reads the env.json of every deployed skill under skillsDir; an unreadable one is skipped and joined into the error.
func LoadDocuments(skillsDir string) ([]*Document, error) {
	paths, err := filepath.Glob(filepath.Join(skillsDir, "*", FileName))
	if err != nil {
		return nil, fmt.Errorf("LoadDocuments: %w", err)
	}

	documents := make([]*Document, 0, len(paths))
	var errs []error
	for _, path := range paths {
		document, err := LoadDocument(path)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		documents = append(documents, document)
	}
	return documents, errors.Join(errs...)
}

func ParseDocument(content []byte) (*Document, error) {
	document := &Document{}
	if err := json.Unmarshal(content, document); err != nil {
		return nil, fmt.Errorf("ParseDocument: Invalid env declaration: %w", err)
	}
	if document.Schema != DocumentSchema {
		return nil, fmt.Errorf("ParseDocument: Unsupported schema %d", document.Schema)
	}
	return document, nil
}

// Phrase is a record summary as a clause: its first rune lowered, a trailing period cut — how the guard and the refusal quote it.
func Phrase(summary string) string {
	phrase := strings.TrimSuffix(summary, ".")
	first, size := utf8.DecodeRuneInString(phrase)
	if size == 0 {
		return phrase
	}
	return string(unicode.ToLower(first)) + phrase[size:]
}

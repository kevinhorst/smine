// Package catalog embeds the key-reference catalog for the config editor.
// catalog.json is generated from the SchemaStore Claude Code settings schema
// and the Codex config-schema JSON (enum values backfilled from the Codex
// config-reference page); the /catalog-refresh skill (backlog) must emit the
// same flat Entry array.
package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
)

//go:embed catalog.json
var catalogJSON []byte

const (
	TargetClaude = "claude"
	TargetCodex  = "codex"
)

const (
	TypeArray  = "array"
	TypeBool   = "bool"
	TypeEnum   = "enum"
	TypeNumber = "number"
	TypeObject = "object"
	TypeString = "string"
	TypeTable  = "table"
)

type Entry struct {
	Category    string   `json:"category"`
	Explanation string   `json:"explanation"`
	Key         string   `json:"key"`
	Source      string   `json:"source"`
	Target      string   `json:"target"` // TargetClaude | TargetCodex
	Type        string   `json:"type"`   // one of the Type* constants
	Values      []string `json:"values,omitempty"`
}

func Load() ([]Entry, error) {
	var entries []Entry
	if err := json.Unmarshal(catalogJSON, &entries); err != nil {
		return nil, fmt.Errorf("Load: Failed to parse embedded catalog: %w", err)
	}
	return entries, nil
}

func IsTarget(target string) bool {
	return target == TargetClaude || target == TargetCodex
}

func IsTextualType(configType string) bool {
	return configType == TypeEnum || configType == TypeString
}

func ForTarget(entries []Entry, target string) []Entry {
	var result []Entry
	for _, e := range entries {
		if e.Target == target {
			result = append(result, e)
		}
	}
	return result
}

// Validate checks a submitted string value against the entry's type.
// Structured types (array/object/table) are validated target-side (D8).
func Validate(e *Entry, value string) error {
	switch e.Type {
	case TypeBool:
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("Validate: Key %s: Not a bool: %s", e.Key, value)
		}
	case TypeNumber:
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return fmt.Errorf("Validate: Key %s: Not a number: %s", e.Key, value)
		}
	case TypeEnum:
		if !slices.Contains(e.Values, value) {
			return fmt.Errorf("Validate: Key %s: Not an allowed value: %s", e.Key, value)
		}
	}
	return nil
}

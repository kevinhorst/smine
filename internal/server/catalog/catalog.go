// Package catalog embeds the key-reference catalog for the config editor.
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
	TypeMap    = "map"
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

// Validate leaves structured types (array, object, table) to target-side validation.
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

// ValidateEntry checks one map entry's value; map keys carry a closed value set.
func ValidateEntry(e *Entry, value string) error {
	if e.Type != TypeMap {
		return fmt.Errorf("ValidateEntry: Key %s: Not a map", e.Key)
	}
	if !slices.Contains(e.Values, value) {
		return fmt.Errorf("ValidateEntry: Key %s: Not an allowed value: %s", e.Key, value)
	}
	return nil
}

// Package jsonschemacheck validates JSON files against the draft their schema declares (2020-12 when absent) as file:line: message lines.
package jsonschemacheck

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Schema is one compiled schema of the draft its $schema names.
type Schema struct {
	compiled *jsonschema.Schema
}

// NewSchema compiles the schema file at path.
func NewSchema(path string) (*Schema, error) {
	compiled, err := jsonschema.NewCompiler().Compile(path)
	if err != nil {
		return nil, fmt.Errorf("jsonschemacheck.NewSchema: %s: %w", path, err)
	}

	schema := &Schema{compiled: compiled}
	return schema, nil
}

// NewSchemaDocument compiles an in-memory schema; name is the URL the compiler files it under.
func NewSchemaDocument(name string, document []byte) (*Schema, error) {
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(document))
	if err != nil {
		return nil, fmt.Errorf("jsonschemacheck.NewSchemaDocument: %s: %w", name, err)
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(name, instance); err != nil {
		return nil, fmt.Errorf("jsonschemacheck.NewSchemaDocument: %s: %w", name, err)
	}

	compiled, err := compiler.Compile(name)
	if err != nil {
		return nil, fmt.Errorf("jsonschemacheck.NewSchemaDocument: %s: %w", name, err)
	}

	schema := &Schema{compiled: compiled}
	return schema, nil
}

// An unreadable file is an error, never a violation.
func (s *Schema) Violations(file string) ([]string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("Schema.Violations: %w", err)
	}
	return s.ViolationsIn(file, data), nil
}

// ViolationsIn validates data already read; file only names the lines.
func (s *Schema) ViolationsIn(file string, data []byte) []string {
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		notJson := []string{fmt.Sprintf("%s:1: not valid JSON: %v", file, err)}
		return notJson
	}

	err = s.compiled.Validate(instance)
	if err == nil {
		return nil
	}

	violations := make([]string, 0)
	for line := range strings.SplitSeq(err.Error(), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			violations = append(violations, file+":1: "+line)
		}
	}
	return violations
}

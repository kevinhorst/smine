package sessions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/kevinhorst/smine/internal/redact"
	"github.com/kevinhorst/smine/internal/register"
)

const RecordFormat = 2

// recordViolations are the format-2 rules the schema cannot express; lengths stay in the schema.
func recordViolations(session *Session) []string {
	if session.Format != RecordFormat {
		return []string{fmt.Sprintf("format: %d required", RecordFormat)}
	}

	var violations []string
	for index, finding := range session.Findings {
		for _, violation := range register.SentenceViolations(finding.Headline) {
			violations = append(violations, fmt.Sprintf("findings[%d].headline: %s", index, violation))
		}
	}
	return violations
}

func redactPermissions(permissions *Permissions) {
	if permissions == nil {
		return
	}
	for index := range permissions.Granted {
		permissions.Granted[index].Command = redact.RedactSecrets(permissions.Granted[index].Command)
	}
	for index := range permissions.Denied {
		permissions.Denied[index].Command = redact.RedactSecrets(permissions.Denied[index].Command)
	}
}

// saveRecord writes the file even on a schema violation, so a redaction never waits on a fix.
func saveRecord(path, schemaPath string, session *Session) ([]string, error) {
	if session.Findings == nil {
		session.Findings = []Finding{}
	}
	redactPermissions(session.Permissions)

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("saveRecord: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return nil, fmt.Errorf("saveRecord: %w", err)
	}

	schemaViolation, err := validateSession(data, schemaPath)
	if err != nil {
		return nil, err
	}
	if schemaViolation != "" {
		return []string{schemaViolation}, nil
	}
	return nil, nil
}

// validateSession asserts formats (date-time, date) instead of treating them as annotations.
func validateSession(data []byte, schemaPath string) (string, error) {
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	schema, err := compiler.Compile(schemaPath)
	if err != nil {
		return "", fmt.Errorf("validateSession: %s: %w", schemaPath, err)
	}

	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("validateSession: %w", err)
	}
	if err := schema.Validate(instance); err != nil {
		return "schema: " + err.Error(), nil
	}
	return "", nil
}

// CheckRecord leaves malformed or unknown-field JSON untouched; a non-nil usedContext replaces the context key.
func CheckRecord(path, schemaPath string, usedContext *ContextRecord) ([]string, error) {
	session, violations, err := ReadRecord(path)
	if err != nil || len(violations) > 0 {
		return violations, err
	}

	if usedContext != nil {
		session.Context = usedContext
	}
	violations, err = saveRecord(path, schemaPath, session)
	if err != nil {
		return nil, err
	}
	return append(violations, recordViolations(session)...), nil
}

// MarkSynthesized is the only writer of the synthesized field.
func MarkSynthesized(date, path, schemaPath string) ([]string, error) {
	session, violations, err := ReadRecord(path)
	if err != nil || len(violations) > 0 {
		return violations, err
	}

	session.Synthesized = date
	return saveRecord(path, schemaPath, session)
}

// ReadRecord reports a decode failure as a violation and an unreadable file as an error.
func ReadRecord(path string) (*Session, []string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("ReadRecord: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	session := &Session{}
	if err := decoder.Decode(session); err != nil {
		return nil, []string{"json: " + err.Error()}, nil
	}
	return session, nil, nil
}

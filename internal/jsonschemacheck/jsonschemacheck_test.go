package jsonschemacheck

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const kindSchema = `{"$schema": "http://json-schema.org/draft-07/schema#", "type": "object", "required": ["kind"], "properties": {"kind": {"enum": ["skills"]}}}`

// provideFile writes content to name in dir and returns its path.
func provideFile(t *testing.T, content string, dir string, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

// violationFiles is the ordered set of file:line locations the violation
// lines name.
func violationFiles(violations []string) []string {
	files := make([]string, 0)
	for _, violation := range violations {
		location, _, _ := strings.Cut(violation, ": ")
		if !slices.Contains(files, location) {
			files = append(files, location)
		}
	}
	return files
}

func TestNewSchema(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool

		path string
	}

	dir := t.TempDir()
	kindSchemaPath := provideFile(t, kindSchema, dir, "schema.json")
	notJsonSchemaPath := provideFile(t, "{not json", dir, "broken.json")
	tests := make([]*testCase, 0)

	// compiles
	tests = append(tests, &testCase{
		_id:         "compiles",
		_shouldPass: true,

		path: kindSchemaPath,
	})

	// missing-schema
	tests = append(tests, &testCase{
		_id:         "missing-schema",
		_shouldPass: false,

		path: filepath.Join(dir, "absent.json"),
	})

	// not-json-schema
	tests = append(tests, &testCase{
		_id:         "not-json-schema",
		_shouldPass: false,

		path: notJsonSchemaPath,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			_, err := NewSchema(test.path)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
		})
	}
}

func TestNewSchemaDocument(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool

		document []byte
	}

	tests := make([]*testCase, 0)

	// compiles
	tests = append(tests, &testCase{
		_id:         "compiles",
		_shouldPass: true,

		document: []byte(kindSchema),
	})

	// not-json-document
	tests = append(tests, &testCase{
		_id:         "not-json-document",
		_shouldPass: false,

		document: []byte("{not json"),
	})

	// invalid-schema
	tests = append(tests, &testCase{
		_id:         "invalid-schema",
		_shouldPass: false,

		document: []byte(`{"type": 7}`),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			_, err := NewSchemaDocument("schema.json", test.document)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
		})
	}
}

func TestSchema_ViolationsIn(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedFiles []string
		_expectedText  string

		data []byte
	}

	schema, err := NewSchemaDocument("schema.json", []byte(kindSchema))
	require.NoError(t, err)

	tests := make([]*testCase, 0)

	// valid
	tests = append(tests, &testCase{
		_id:            "valid",
		_expectedFiles: []string{},
		_expectedText:  "",

		data: []byte(`{"kind": "skills"}`),
	})

	// violation
	tests = append(tests, &testCase{
		_id:            "violation",
		_expectedFiles: []string{"t.json:1"},
		_expectedText:  "/kind",

		data: []byte(`{"kind": "context"}`),
	})

	// not-json
	tests = append(tests, &testCase{
		_id:            "not-json",
		_expectedFiles: []string{"t.json:1"},
		_expectedText:  "not valid JSON",

		data: []byte("{not json"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			violations := schema.ViolationsIn("t.json", test.data)
			assert.Equal(t, test._expectedFiles, violationFiles(violations))
			assert.Contains(t, strings.Join(violations, "\n"), test._expectedText)
		})
	}
}

func TestSchema_Violations(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedFiles []string
		_expectedText  string
		_shouldPass    bool

		file string
	}

	dir := t.TempDir()
	schema, err := NewSchema(provideFile(t, kindSchema, dir, "schema.json"))
	require.NoError(t, err)

	validFile := provideFile(t, `{"kind": "skills"}`, dir, "valid.json")
	enumBreakFile := provideFile(t, `{"kind": "context"}`, dir, "enum.json")
	requiredBreakFile := provideFile(t, `{}`, dir, "required.json")
	notJsonFile := provideFile(t, "{not json", dir, "broken.json")
	tests := make([]*testCase, 0)

	// valid-file
	tests = append(tests, &testCase{
		_id:            "valid-file",
		_expectedFiles: []string{},
		_expectedText:  "",
		_shouldPass:    true,

		file: validFile,
	})

	// enum-break-names-file
	tests = append(tests, &testCase{
		_id:            "enum-break-names-file",
		_expectedFiles: []string{enumBreakFile + ":1"},
		_expectedText:  "/kind",
		_shouldPass:    true,

		file: enumBreakFile,
	})

	// required-break-names-file
	tests = append(tests, &testCase{
		_id:            "required-break-names-file",
		_expectedFiles: []string{requiredBreakFile + ":1"},
		_expectedText:  "kind",
		_shouldPass:    true,

		file: requiredBreakFile,
	})

	// not-json-one-violation
	tests = append(tests, &testCase{
		_id:            "not-json-one-violation",
		_expectedFiles: []string{notJsonFile + ":1"},
		_expectedText:  "not valid JSON",
		_shouldPass:    true,

		file: notJsonFile,
	})

	// missing-file-is-error
	tests = append(tests, &testCase{
		_id:            "missing-file-is-error",
		_expectedFiles: []string{},
		_expectedText:  "",
		_shouldPass:    false,

		file: filepath.Join(dir, "absent.json"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			violations, err := schema.Violations(test.file)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedFiles, violationFiles(violations))
			assert.Contains(t, strings.Join(violations, "\n"), test._expectedText)
		})
	}
}

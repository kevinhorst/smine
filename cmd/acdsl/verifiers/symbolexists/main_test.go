package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFilesList(t *testing.T, files ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "files.txt")
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(files, "\n")+"\n"), 0o644))
	return path
}

func TestRunFuncFound(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{writeFilesList(t, "testdata/sample.go"), "symbol=Load"}, &out)
	assert.Equal(t, 0, code)
	assert.Empty(t, out.String())
}

func TestRunMethodFound(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{writeFilesList(t, "testdata/sample.go"), "symbol=Config.Validate"}, &out)
	assert.Equal(t, 0, code)
}

func TestRunTypeAndVarFound(t *testing.T) {
	var out bytes.Buffer
	assert.Equal(t, 0, run([]string{writeFilesList(t, "testdata/sample.go"), "symbol=Config"}, &out))
	assert.Equal(t, 0, run([]string{writeFilesList(t, "testdata/sample.go"), "symbol=DefaultName"}, &out))
}

func TestRunMissingSymbolIsViolation(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{writeFilesList(t, "testdata/sample.go"), "symbol=Absent"}, &out)
	assert.Equal(t, 1, code)
	assert.Contains(t, out.String(), `testdata/sample.go:1: symbol "Absent" not found`)
}

func TestRunSignatureMatchNormalizesWhitespace(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{
		writeFilesList(t, "testdata/sample.go"),
		"symbol=Load",
		"signature=func Load(ctx context.Context, path string) (*Config, error)",
	}, &out)
	assert.Equal(t, 0, code, out.String())
}

func TestRunSignatureMismatchCitesBothSides(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{
		writeFilesList(t, "testdata/sample.go"),
		"symbol=Load",
		"signature=func Load(path string) (*Config, error)",
	}, &out)
	assert.Equal(t, 1, code)
	assert.Contains(t, out.String(), "signature mismatch")
	assert.Contains(t, out.String(), "planned func Load(path string) (*Config, error)")
}

func TestRunSignatureOnNonFuncIsViolation(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{writeFilesList(t, "testdata/sample.go"), "symbol=Config", "signature=func Config()"}, &out)
	assert.Equal(t, 1, code)
	assert.Contains(t, out.String(), "not a func")
}

func TestRunMissingParamsAreErrors(t *testing.T) {
	var out bytes.Buffer
	assert.Equal(t, 2, run([]string{}, &out))
	assert.Equal(t, 2, run([]string{writeFilesList(t, "testdata/sample.go")}, &out))
}

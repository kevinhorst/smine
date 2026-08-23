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

// Fixture inputs are the committed ACDSL-GOLANG-FMT-001 example sets — the same
// files acdsl fixtures proves the rule against.
const (
	passFixture = "../../../../acdsl/testdata/ACDSL-GOLANG-FMT-001/pass/formatted.go"
	failFixture = "../../../../acdsl/testdata/ACDSL-GOLANG-FMT-001/fail/unformatted.go"
)

func writeFilesList(t *testing.T, files ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "files.txt")
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(files, "\n")+"\n"), 0o644))
	return path
}

func TestRunUnformattedFlagged(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{writeFilesList(t, failFixture)}, &out)
	assert.Equal(t, 1, code)
	assert.Contains(t, out.String(), "unformatted.go:1: file is not gofmt-formatted — run gofmt -w")
}

func TestRunFormattedClean(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{writeFilesList(t, passFixture)}, &out)
	assert.Equal(t, 0, code)
	assert.Empty(t, out.String())
}

func TestRunEmptyListClean(t *testing.T) {
	var out bytes.Buffer
	listPath := filepath.Join(t.TempDir(), "files.txt")
	require.NoError(t, os.WriteFile(listPath, []byte("\n"), 0o644))
	assert.Equal(t, 0, run([]string{listPath}, &out))
}

func TestRunMissingListIsError(t *testing.T) {
	var out bytes.Buffer
	assert.Equal(t, 2, run([]string{}, &out))
}

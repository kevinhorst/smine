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

func TestRunMixedFixture(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{writeFilesList(t, "testdata/mixed.go")}, &out)
	assert.Equal(t, 1, code)
	// Only the two bad() literals are flagged; prefixed and non-literal pass.
	assert.Equal(t, 2, strings.Count(out.String(), "lacks the Component.Method: prefix"))
	assert.Contains(t, out.String(), "testdata/mixed.go:19")
	assert.Contains(t, out.String(), "testdata/mixed.go:22")
}

func TestRunMissingListIsError(t *testing.T) {
	var out bytes.Buffer
	assert.Equal(t, 2, run([]string{}, &out))
}

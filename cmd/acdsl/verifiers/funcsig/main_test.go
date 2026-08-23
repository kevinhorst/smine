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
	// Only the four bad declarations are flagged; func, method, and no-context
	// declarations that follow the rule stay clean.
	assert.Equal(t, 4, strings.Count(out.String(), "\n"))
	assert.Contains(t, out.String(), "testdata/mixed.go:21: ctxSecond: context.Context must be the first parameter")
	assert.Contains(t, out.String(), "testdata/mixed.go:27: ctxMisnamed: context.Context parameter must be named ctx")
	assert.Contains(t, out.String(), "testdata/mixed.go:32: errorNotLast: error must be the last return value")
	assert.Contains(t, out.String(), "testdata/mixed.go:37: fourReturns: 4 return values — at most 3 including error")
}

func TestRunMissingListIsError(t *testing.T) {
	var out bytes.Buffer
	assert.Equal(t, 2, run([]string{}, &out))
}

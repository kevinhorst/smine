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

func TestRunDirectCallFlagged(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{writeFilesList(t, "testdata/direct.go"), "pkg=os/exec", "func=Command"}, &out)
	assert.Equal(t, 1, code)
	assert.Contains(t, out.String(), "testdata/direct.go:6: call to os/exec.Command is forbidden here")
}

func TestRunAliasedImportFlagged(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{writeFilesList(t, "testdata/aliased.go"), "pkg=os/exec", "func=Command"}, &out)
	assert.Equal(t, 1, code)
	assert.Contains(t, out.String(), "testdata/aliased.go:6")
}

func TestRunCommandContextNotFlagged(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{writeFilesList(t, "testdata/contextonly.go"), "pkg=os/exec", "func=Command"}, &out)
	assert.Equal(t, 0, code)
	assert.Empty(t, out.String())
}

func TestRunNoImportClean(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{writeFilesList(t, "testdata/noimport.go"), "pkg=os/exec", "func=Command"}, &out)
	assert.Equal(t, 0, code)
}

func TestRunDotImportCleanDocumentedLimitation(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{writeFilesList(t, "testdata/dotimport.go"), "pkg=os/exec", "func=Command"}, &out)
	assert.Equal(t, 0, code)
}

func TestRunMissingParamsIsError(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{writeFilesList(t, "testdata/direct.go")}, &out)
	assert.Equal(t, 2, code)
}

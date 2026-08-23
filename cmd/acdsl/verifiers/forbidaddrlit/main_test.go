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

func TestRunAddrLitFlagged(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{writeFilesList(t, "testdata/addrlit.go")}, &out)
	assert.Equal(t, 1, code)
	assert.Contains(t, out.String(), "testdata/addrlit.go:8: address-of composite literal is forbidden here")
}

func TestRunNewExprAndAddrOfVarClean(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{writeFilesList(t, "testdata/newexpr.go")}, &out)
	assert.Equal(t, 0, code)
	assert.Empty(t, out.String())
}

func TestRunMissingListIsError(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{}, &out)
	assert.Equal(t, 2, code)
}

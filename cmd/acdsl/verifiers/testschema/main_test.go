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

func TestRunHouseSchemaClean(t *testing.T) {
	var out bytes.Buffer
	assert.Equal(t, 0, run([]string{writeFilesList(t, "testdata/house.go")}, &out))
	assert.Empty(t, out.String())
}

func TestRunCommonStyleFlagged(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{writeFilesList(t, "testdata/common.go")}, &out)
	assert.Equal(t, 1, code)
	// tt.name loop and the ad-hoc literal subtest are both flagged.
	assert.Equal(t, 2, strings.Count(out.String(), "not the case's _id field"))
}

func TestRunMissingListIsError(t *testing.T) {
	var out bytes.Buffer
	assert.Equal(t, 2, run([]string{}, &out))
}

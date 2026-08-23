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

func TestRunMissingListIsError(t *testing.T) {
	var out bytes.Buffer
	assert.Equal(t, 2, run([]string{}, &out))
}

func TestRunUnreadableListIsError(t *testing.T) {
	var out bytes.Buffer
	assert.Equal(t, 2, run([]string{filepath.Join(t.TempDir(), "absent.txt")}, &out))
}

func TestRunEmptyFilesListPasses(t *testing.T) {
	// No anchored files -> no package dirs -> nothing to analyze -> clean pass,
	// the analyzer is never invoked.
	var out bytes.Buffer
	assert.Equal(t, 0, run([]string{writeFilesList(t)}, &out))
	assert.Empty(t, out.String())
}

func TestPackageDirsDedupsSameDir(t *testing.T) {
	dirs := packageDirs([]string{"internal/a/x.go", "internal/a/y.go"})
	assert.Equal(t, []string{"./internal/a"}, dirs)
}

func TestPackageDirsSortedWithPrefix(t *testing.T) {
	dirs := packageDirs([]string{"internal/b/x.go", "cmd/a/main.go"})
	assert.Equal(t, []string{"./cmd/a", "./internal/b"}, dirs)
}

func TestPackageDirsEmpty(t *testing.T) {
	assert.Empty(t, packageDirs(nil))
}

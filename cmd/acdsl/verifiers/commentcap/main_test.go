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

func writeProjectedFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "projected.go")
	content := projectionLead + " 1 rule(s) govern this file — working-copy view, stripped before commit\n" +
		"// - [ACDSL" + "-GOLANG-COMMENT-001] A comment block in non-test Go runs one line at most\n" +
		"\n" +
		"package fixture\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestRunMixedFixture(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{writeFilesList(t, "testdata/mixed.go", writeProjectedFile(t)), "max=1"}, &out)
	assert.Equal(t, 1, code)
	// Only the two over-long blocks are flagged; one-liners, directive lines and the projection block stay clean.
	assert.Equal(t, 2, strings.Count(out.String(), "\n"))
	assert.Contains(t, out.String(), "testdata/mixed.go:6: comment block of 3 lines, at most 1 — delete it, cut it to one line or move it to the directory's README")
	assert.Contains(t, out.String(), "testdata/mixed.go:22: comment block of 2 lines, at most 1 — delete it, cut it to one line or move it to the directory's README")
}

func TestRunMissingListIsError(t *testing.T) {
	var out bytes.Buffer
	assert.Equal(t, 2, run([]string{}, &out))
}

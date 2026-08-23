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

// fixtures creates owner/owned.go and other/outside.go under a temp root and
// returns the root plus a files-list referencing both (paths relative to root
// are not required by the script — it reads what the list says).
func fixtures(t *testing.T, outsideContent string) (string, string) {
	t.Helper()
	root := t.TempDir()
	ownerPath := filepath.Join(root, "owner", "owned.go")
	otherPath := filepath.Join(root, "other", "outside.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(ownerPath), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(otherPath), 0o755))
	require.NoError(t, os.WriteFile(ownerPath, []byte("package owner\n\nconst target = \"state.json\"\n"), 0o644))
	require.NoError(t, os.WriteFile(otherPath, []byte(outsideContent), 0o644))

	listPath := filepath.Join(root, "files.txt")
	require.NoError(t, os.WriteFile(listPath, []byte(strings.Join([]string{ownerPath, otherPath}, "\n")+"\n"), 0o644))
	return root, listPath
}

func TestRunLiteralOutsideOwnerFlaggedWithLine(t *testing.T) {
	_, listPath := fixtures(t, "package other\n\nvar leak = \"state.json\"\n")
	var out bytes.Buffer
	code := run([]string{listPath, "literal=state.json", "allowed=/owner/"}, &out)
	assert.Equal(t, 1, code)
	assert.Contains(t, out.String(), "outside.go:3")
	assert.Contains(t, out.String(), `literal "state.json" outside its owner`)
}

func TestRunLiteralInsideOwnerClean(t *testing.T) {
	_, listPath := fixtures(t, "package other\n\nvar clean = 1\n")
	var out bytes.Buffer
	code := run([]string{listPath, "literal=state.json", "allowed=/owner/"}, &out)
	assert.Equal(t, 0, code)
	assert.Empty(t, out.String())
}

func TestRunMultipleHitsCounted(t *testing.T) {
	_, listPath := fixtures(t, "package other\n\nvar a = \"state.json\"\nvar b = \"state.json\"\n")
	var out bytes.Buffer
	code := run([]string{listPath, "literal=state.json", "allowed=/owner/"}, &out)
	assert.Equal(t, 1, code)
	assert.Equal(t, 2, strings.Count(out.String(), "outside its owner"))
}

func TestRunMissingParamsIsError(t *testing.T) {
	_, listPath := fixtures(t, "package other\n")
	var out bytes.Buffer
	code := run([]string{listPath}, &out)
	assert.Equal(t, 2, code)
}

func TestRunProjectionBlockCleanedBeforeScan(t *testing.T) {
	projected := projectionMarker + " 1 rule(s) govern this file — working-copy view, stripped before commit\n" +
		"// - [X-001] the literal state.json has one owner\n" +
		"\n" +
		"package other\n" +
		"\n" +
		"var leak = \"state.json\"\n"
	_, listPath := fixtures(t, projected)
	var out bytes.Buffer
	code := run([]string{listPath, "literal=state.json", "allowed=/owner/"}, &out)
	assert.Equal(t, 1, code)
	// Only the content hit is flagged, at its on-disk line number (6).
	assert.Equal(t, 1, strings.Count(out.String(), "outside its owner"))
	assert.Contains(t, out.String(), "outside.go:6")
}

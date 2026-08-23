package fsx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplaceFileReplacesExisting(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	tmp := filepath.Join(dir, "target.json.tmp")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o644))
	require.NoError(t, os.WriteFile(tmp, []byte("new"), 0o644))

	require.NoError(t, ReplaceFile(tmp, target))

	content, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "new", string(content))
	assert.NoFileExists(t, tmp)
}

func TestReplaceFileCreatesMissingDestination(t *testing.T) {
	dir := t.TempDir()
	tmp := filepath.Join(dir, "fresh.tmp")
	require.NoError(t, os.WriteFile(tmp, []byte("fresh"), 0o644))

	require.NoError(t, ReplaceFile(tmp, filepath.Join(dir, "fresh.json")))
	assert.FileExists(t, filepath.Join(dir, "fresh.json"))
}

func TestReplaceFileMissingTmpFails(t *testing.T) {
	dir := t.TempDir()
	err := ReplaceFile(filepath.Join(dir, "absent.tmp"), filepath.Join(dir, "target"))
	require.ErrorContains(t, err, "ReplaceFile")
}

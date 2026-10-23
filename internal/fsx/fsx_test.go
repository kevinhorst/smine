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

func TestWriteFileAtomic(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedContent string
		_shouldPass      bool

		path string
	}

	tests := make([]*testCase, 0)

	// pass-creates-file
	tests = append(tests, &testCase{
		_id:              "pass-creates-file",
		_expectedContent: "new",
		_shouldPass:      true,

		path: filepath.Join(t.TempDir(), "target.json"),
	})

	// pass-replaces-existing
	existing := filepath.Join(t.TempDir(), "target.json")
	require.NoError(t, os.WriteFile(existing, []byte("old"), 0o644))
	tests = append(tests, &testCase{
		_id:              "pass-replaces-existing",
		_expectedContent: "new",
		_shouldPass:      true,

		path: existing,
	})

	// pass-creates-parent-dir
	tests = append(tests, &testCase{
		_id:              "pass-creates-parent-dir",
		_expectedContent: "new",
		_shouldPass:      true,

		path: filepath.Join(t.TempDir(), "a", "b", "target.json"),
	})

	// fail-parent-is-file
	parentFile := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(parentFile, nil, 0o644))
	tests = append(tests, &testCase{
		_id:              "fail-parent-is-file",
		_expectedContent: "",
		_shouldPass:      false,

		path: filepath.Join(parentFile, "target.json"),
	})

	// fail-target-is-non-empty-directory
	directoryTarget := filepath.Join(t.TempDir(), "target")
	require.NoError(t, os.MkdirAll(filepath.Join(directoryTarget, "child"), 0o755))
	tests = append(tests, &testCase{
		_id:              "fail-target-is-non-empty-directory",
		_expectedContent: "",
		_shouldPass:      false,

		path: directoryTarget,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := WriteFileAtomic([]byte("new"), test.path, 0o644)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			content, _ := os.ReadFile(test.path)
			assert.Equal(t, test._expectedContent, string(content))
			assert.NoFileExists(t, test.path+".tmp")
		})
	}
}

func TestReplaceFileMissingTmpFails(t *testing.T) {
	dir := t.TempDir()
	err := ReplaceFile(filepath.Join(dir, "absent.tmp"), filepath.Join(dir, "target"))
	require.ErrorContains(t, err, "ReplaceFile")
}

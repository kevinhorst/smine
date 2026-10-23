package contextdocs

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeStub(t *testing.T, dir, name, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755))
}

func TestChooseFolder(t *testing.T) {
	stubPath := func(t *testing.T, body string) {
		t.Helper()
		dir := t.TempDir()
		writeStub(t, dir, "osascript", body)
		t.Setenv("PATH", dir)
	}

	t.Run("returns-trimmed-path", func(t *testing.T) {
		stubPath(t, "#!/bin/sh\necho '/tmp/chosen/'\n")
		path, err := ChooseFolder(context.Background(), "pick a folder")
		require.NoError(t, err)
		assert.Equal(t, "/tmp/chosen/", path)
	})

	t.Run("cancel-maps-to-ErrCanceled", func(t *testing.T) {
		stubPath(t, "#!/bin/sh\necho 'execution error: User canceled. (-128)' >&2\nexit 1\n")
		_, err := ChooseFolder(context.Background(), "pick a folder")
		assert.ErrorIs(t, err, ErrCanceled)
	})

	t.Run("failure-wraps-error", func(t *testing.T) {
		stubPath(t, "#!/bin/sh\necho 'no GUI session' >&2\nexit 1\n")
		_, err := ChooseFolder(context.Background(), "pick a folder")
		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrCanceled)
	})
}

// TestFolderDialogScript pins the Windows picker template: the TopMost owner
// form passed to ShowDialog is what brings the dialog to the foreground when
// the server is a background windowsgui process (addendum A6).
func TestFolderDialogScript(t *testing.T) {
	assert.Contains(t, folderDialogScript, "ShowDialog($owner)")
	assert.Contains(t, folderDialogScript, "TopMost=$true")
}

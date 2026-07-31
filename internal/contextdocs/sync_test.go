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

func TestSync(t *testing.T) {
	scripts := t.TempDir()
	writeStub(t, scripts, "sync_context.sh", "#!/bin/sh\necho \"context-sync $*\"\n")

	t.Run("passes-flags-and-target", func(t *testing.T) {
		opts := SyncOptions{ContextDir: "docs", Langs: []string{"general", "go"}, Symlink: true, Target: "/tmp/target"}
		output, err := Sync(context.Background(), opts, scripts)
		require.NoError(t, err)
		assert.Contains(t, output, "context-sync --context-dir docs --langs general,go --symlink /tmp/target")
	})

	t.Run("no-symlink", func(t *testing.T) {
		opts := SyncOptions{Langs: nil, Symlink: false, Target: "/tmp/target"}
		output, err := Sync(context.Background(), opts, scripts)
		require.NoError(t, err)
		assert.Contains(t, output, "--langs  --no-symlink")
	})

	t.Run("empty-target-errors-without-exec", func(t *testing.T) {
		output, err := Sync(context.Background(), SyncOptions{Target: "  "}, scripts)
		require.Error(t, err)
		assert.Empty(t, output)
		assert.Contains(t, err.Error(), "target path is empty")
	})
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

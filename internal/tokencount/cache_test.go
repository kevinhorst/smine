package tokencount

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/contextrecord"
)

func TestCache(t *testing.T) {
	t.Run("missing-file-empty", func(t *testing.T) {
		loaded := loadCache(filepath.Join(t.TempDir(), cacheFile))

		assert.Empty(t, loaded.entries)
		assert.False(t, loaded.isDirty)
	})

	t.Run("invalid-json-empty", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), cacheFile)
		require.NoError(t, os.WriteFile(path, []byte("{not json"), cacheFileMode))

		loaded := loadCache(path)

		assert.Empty(t, loaded.entries)
	})

	t.Run("put-get", func(t *testing.T) {
		loaded := loadCache(filepath.Join(t.TempDir(), cacheFile))
		loaded.put("key", 42)

		tokens, ok := loaded.get("key")
		_, isMissingFound := loaded.get("other")

		assert.True(t, ok)
		assert.Equal(t, 42, tokens)
		assert.False(t, isMissingFound)
		assert.True(t, loaded.isDirty)
	})

	t.Run("save-0600-and-reload", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nested", cacheFile)
		loaded := loadCache(path)
		loaded.put("key", 7)

		err := loaded.save()
		require.NoError(t, err)

		info, err := os.Stat(path)
		require.NoError(t, err)

		reloaded := loadCache(path)
		tokens, ok := reloaded.get("key")

		if runtime.GOOS != "windows" { // Windows keeps no POSIX mode: Stat reports 0666
			assert.Equal(t, fs.FileMode(cacheFileMode), info.Mode().Perm())
		}
		assert.False(t, loaded.isDirty)
		assert.True(t, ok)
		assert.Equal(t, 7, tokens)
	})

	t.Run("merge-with-disk-entry", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), cacheFile)
		loaded := loadCache(path)
		loaded.put("mine", 1)
		require.NoError(t, os.WriteFile(path, []byte(`{"theirs":2}`), cacheFileMode))

		err := loaded.save()
		require.NoError(t, err)

		reloaded := loadCache(path)
		expectedEntries := map[string]int{"mine": 1, "theirs": 2}

		assert.Equal(t, expectedEntries, reloaded.entries)
		assert.Equal(t, expectedEntries, loaded.entries)
	})

	t.Run("clean-save-no-op-without-file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), cacheFile)
		loaded := loadCache(path)

		err := loaded.save()
		_, statErr := os.Stat(path)

		require.NoError(t, err)
		assert.ErrorIs(t, statErr, fs.ErrNotExist)
	})

	t.Run("clean-save-no-op-mtime-unchanged", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), cacheFile)
		require.NoError(t, os.WriteFile(path, []byte(`{"key":3}`), cacheFileMode))
		before, err := os.Stat(path)
		require.NoError(t, err)

		loaded := loadCache(path)
		err = loaded.save()
		require.NoError(t, err)

		after, err := os.Stat(path)
		require.NoError(t, err)

		assert.Equal(t, before.ModTime(), after.ModTime())
	})

	t.Run("two-caches-save-same-path-both-entries-kept", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), cacheFile)
		first := loadCache(path)
		second := loadCache(path)
		first.put("first", 1)
		second.put("second", 2)

		require.NoError(t, first.save())
		require.NoError(t, second.save())

		reloaded := loadCache(path)
		expectedEntries := map[string]int{"first": 1, "second": 2}

		assert.Equal(t, expectedEntries, reloaded.entries)
	})

	t.Run("no-temp-file-left", func(t *testing.T) {
		dir := t.TempDir()
		loaded := loadCache(filepath.Join(dir, cacheFile))
		loaded.put("key", 1)

		require.NoError(t, loaded.save())

		entries, err := os.ReadDir(dir)
		require.NoError(t, err)

		require.Len(t, entries, 1)
		assert.Equal(t, cacheFile, entries[0].Name())
	})

	t.Run("clean-cache-no-write", func(t *testing.T) {
		dir := t.TempDir()
		loaded := loadCache(filepath.Join(dir, cacheFile))

		require.NoError(t, loaded.save())

		entries, err := os.ReadDir(dir)
		require.NoError(t, err)

		assert.Empty(t, entries)
	})

	t.Run("keys-differ-per-model-and-text", func(t *testing.T) {
		base := cacheKey(contextrecord.CounterAnthropic, "claude-opus-4-1", "text")

		assert.Equal(t, base, cacheKey(contextrecord.CounterAnthropic, "claude-opus-4-1", "text"))
		assert.NotEqual(t, base, cacheKey(contextrecord.CounterAnthropic, "claude-sonnet-4-5", "text"))
		assert.NotEqual(t, base, cacheKey(contextrecord.CounterAnthropic, "claude-opus-4-1", "other"))
		assert.NotEqual(t, base, cacheKey(contextrecord.CounterO200k, "claude-opus-4-1", "text"))
	})
}

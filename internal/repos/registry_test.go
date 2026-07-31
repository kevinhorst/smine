package repos

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeRegistry(t *testing.T, content string) *Registry {
	t.Helper()
	path := filepath.Join(t.TempDir(), "repos.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return NewRegistry(path)
}

func TestReloadMissingFileIsEmpty(t *testing.T) {
	registry := NewRegistry(filepath.Join(t.TempDir(), "repos.json"))
	require.NoError(t, registry.Reload())
	assert.Empty(t, registry.Repos())
}

func TestReloadDuplicateNameFails(t *testing.T) {
	registry := writeRegistry(t, `{"repos": [
		{"name": "a", "path": "/tmp/a"},
		{"name": "a", "path": "/tmp/b"}
	]}`)
	err := registry.Reload()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Duplicate repo name")
}

func TestReloadRelativePathFails(t *testing.T) {
	registry := writeRegistry(t, `{"repos": [{"name": "a", "path": "relative/path"}]}`)
	err := registry.Reload()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Must be absolute")
}

func TestReloadBadNameFails(t *testing.T) {
	registry := writeRegistry(t, `{"repos": [{"name": "a/b", "path": "/tmp/a"}]}`)
	err := registry.Reload()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid field Name")
}

func TestRegistryAdd(t *testing.T) {
	t.Run("appends-and-persists", func(t *testing.T) {
		registry := writeRegistry(t, `{"repos": [{"name": "a", "path": "/tmp/a"}]}`)
		require.NoError(t, registry.Reload())

		require.NoError(t, registry.Add(Repo{Name: "b", Path: "/tmp/b"}))
		assert.Len(t, registry.Repos(), 2)

		require.NoError(t, registry.Reload())
		repo, ok := registry.Find("b")
		require.True(t, ok)
		assert.Equal(t, "/tmp/b", repo.Path)
	})

	t.Run("duplicate-name-fails-file-unchanged", func(t *testing.T) {
		registry := writeRegistry(t, `{"repos": [{"name": "a", "path": "/tmp/a"}]}`)
		require.NoError(t, registry.Reload())

		err := registry.Add(Repo{Name: "a", Path: "/tmp/other"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Duplicate repo name")

		require.NoError(t, registry.Reload())
		assert.Len(t, registry.Repos(), 1)
	})

	t.Run("invalid-name-fails", func(t *testing.T) {
		registry := writeRegistry(t, `{"repos": []}`)
		require.NoError(t, registry.Reload())
		err := registry.Add(Repo{Name: "a/b", Path: "/tmp/a"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid field Name")
	})

	t.Run("relative-path-fails", func(t *testing.T) {
		registry := writeRegistry(t, `{"repos": []}`)
		require.NoError(t, registry.Reload())
		err := registry.Add(Repo{Name: "a", Path: "relative/path"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Must be absolute")
	})

	t.Run("missing-file-created-on-first-add", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "repos.json")
		registry := NewRegistry(path)
		require.NoError(t, registry.Reload())

		require.NoError(t, registry.Add(Repo{Name: "a", Path: "/tmp/a"}))
		require.NoError(t, registry.Reload())
		assert.Len(t, registry.Repos(), 1)
	})
}

func TestRegistryRemove(t *testing.T) {
	t.Run("removes-and-persists", func(t *testing.T) {
		registry := writeRegistry(t, `{"repos": [
			{"name": "a", "path": "/tmp/a"},
			{"name": "b", "path": "/tmp/b"}
		]}`)
		require.NoError(t, registry.Reload())

		require.NoError(t, registry.Remove("a"))
		_, ok := registry.Find("a")
		assert.False(t, ok)

		require.NoError(t, registry.Reload())
		assert.Len(t, registry.Repos(), 1)
	})

	t.Run("unknown-name-fails-file-unchanged", func(t *testing.T) {
		registry := writeRegistry(t, `{"repos": [{"name": "a", "path": "/tmp/a"}]}`)
		require.NoError(t, registry.Reload())

		err := registry.Remove("nope")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Unknown repo name")

		require.NoError(t, registry.Reload())
		assert.Len(t, registry.Repos(), 1)
	})
}

func TestCountWorktrees(t *testing.T) {
	t.Run("real-worktrees-only", func(t *testing.T) {
		repo := t.TempDir()
		pool := filepath.Join(repo, ".claude", "worktrees")
		for _, name := range []string{"wt-a", "wt-b"} {
			require.NoError(t, os.MkdirAll(filepath.Join(pool, name), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(pool, name, ".git"), []byte("gitdir: elsewhere\n"), 0o644))
		}
		// hollow pool dir (no .git) and the pool-guard decoy file
		require.NoError(t, os.MkdirAll(filepath.Join(pool, "hollow"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(pool, ".git"), []byte("gitdir: /decoy\n"), 0o644))

		assert.Equal(t, 2, CountWorktrees(repo))
	})

	t.Run("missing-pool-is-zero", func(t *testing.T) {
		assert.Equal(t, 0, CountWorktrees(t.TempDir()))
	})

	t.Run("files-only-is-zero", func(t *testing.T) {
		repo := t.TempDir()
		pool := filepath.Join(repo, ".claude", "worktrees")
		require.NoError(t, os.MkdirAll(pool, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(pool, "stray"), []byte("x\n"), 0o644))
		assert.Equal(t, 0, CountWorktrees(repo))
	})
}

func TestDetectContext(t *testing.T) {
	t.Run("general-and-languages", func(t *testing.T) {
		repo := t.TempDir()
		for _, name := range []string{"general", "go", "python"} {
			require.NoError(t, os.MkdirAll(filepath.Join(repo, "context", name), 0o755))
		}
		require.NoError(t, os.WriteFile(filepath.Join(repo, "context", "stray.md"), []byte("x\n"), 0o644))

		presence := DetectContext(repo)
		assert.True(t, presence.HasGeneral)
		assert.Equal(t, []string{"go", "python"}, presence.Languages)
	})

	t.Run("missing-context-dir", func(t *testing.T) {
		presence := DetectContext(t.TempDir())
		assert.False(t, presence.HasGeneral)
		assert.Empty(t, presence.Languages)
	})
}

func TestFindAndRepos(t *testing.T) {
	registry := writeRegistry(t, `{"repos": [{"name": "demo", "path": "/tmp/demo"}]}`)
	require.NoError(t, registry.Reload())

	repo, ok := registry.Find("demo")
	require.True(t, ok)
	assert.Equal(t, "/tmp/demo", repo.Path)

	_, ok = registry.Find("nope")
	assert.False(t, ok)
}

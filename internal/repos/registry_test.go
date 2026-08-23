package repos

import (
	"context"
	"os"
	"os/exec"
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

// gitRepoWithAgentWorktrees builds a real git repo with one claude/* and one
// claude-routines/* worktree checked out outside the repo (like routine
// worktrees are).
func gitRepoWithAgentWorktrees(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	run := func(args ...string) {
		gitArgs := append([]string{"-C", repo, "-c", "user.email=t@t", "-c", "user.name=t"}, args...)
		output, err := exec.Command("git", gitArgs...).CombinedOutput()
		require.NoError(t, err, string(output))
	}
	run("init", "-q", "-b", "main")
	run("commit", "-q", "--allow-empty", "-m", "init")
	run("worktree", "add", "-q", "-b", "claude/session-x", filepath.Join(t.TempDir(), "wt-session"))
	run("worktree", "add", "-q", "-b", "claude-routines/nightly-x", filepath.Join(t.TempDir(), "wt-routine"))
	return repo
}

func TestCountWorktrees(t *testing.T) {
	t.Run("counts-by-branch-prefix", func(t *testing.T) {
		repo := gitRepoWithAgentWorktrees(t)
		ctx := context.Background()
		assert.Equal(t, 2, CountWorktrees(ctx, repo, []string{"claude/", "claude-routines/"}))
		assert.Equal(t, 1, CountWorktrees(ctx, repo, []string{"claude/"}))
		assert.Equal(t, 1, CountWorktrees(ctx, repo, []string{"claude-routines/"}))
	})

	t.Run("non-git-dir-is-zero", func(t *testing.T) {
		assert.Equal(t, 0, CountWorktrees(context.Background(), t.TempDir(), []string{"claude/"}))
	})

	t.Run("no-agent-prefix-matches-nothing", func(t *testing.T) {
		repo := gitRepoWithAgentWorktrees(t)
		assert.Equal(t, 0, CountWorktrees(context.Background(), repo, []string{"feature/"}))
	})
}

func TestCountPoolWorktrees(t *testing.T) {
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

		assert.Equal(t, 2, CountPoolWorktrees(repo))
	})

	t.Run("missing-pool-is-zero", func(t *testing.T) {
		assert.Equal(t, 0, CountPoolWorktrees(t.TempDir()))
	})

	t.Run("files-only-is-zero", func(t *testing.T) {
		repo := t.TempDir()
		pool := filepath.Join(repo, ".claude", "worktrees")
		require.NoError(t, os.MkdirAll(pool, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(pool, "stray"), []byte("x\n"), 0o644))
		assert.Equal(t, 0, CountPoolWorktrees(repo))
	})
}

func TestDetectContext(t *testing.T) {
	t.Run("source-repo-with-language-guides", func(t *testing.T) {
		repo := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(repo, "context", "rules"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(repo, "context", "context.json"),
			[]byte(`{"entries": [], "aspects": []}`), 0o644))
		for _, name := range []string{"go.md", "python.md", "plan.md", "commits.md"} {
			require.NoError(t, os.WriteFile(filepath.Join(repo, "context", "rules", name), []byte("# guide\n"), 0o644))
		}

		presence := DetectContext(repo)
		assert.Equal(t, ContextSource, presence.State)
		assert.Equal(t, "context", presence.ContextDir)
		assert.Equal(t, []string{"go", "python"}, presence.Langs)
		assert.False(t, presence.Acdsl)
	})

	t.Run("source-repo-with-acdsl-registry", func(t *testing.T) {
		repo := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(repo, "context"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(repo, "context", "context.json"),
			[]byte(`{"entries": [], "aspects": []}`), 0o644))
		require.NoError(t, os.MkdirAll(filepath.Join(repo, "acdsl"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(repo, "acdsl", "registry.json"), []byte("{}\n"), 0o644))

		presence := DetectContext(repo)
		assert.Equal(t, ContextSource, presence.State)
		assert.True(t, presence.Acdsl)
	})

	t.Run("deployed-target-with-deploy-section", func(t *testing.T) {
		repo := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(repo, "docs"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(repo, "docs", "context.json"),
			[]byte(`{"deploy": {"contextDir": "docs", "langs": ["go"], "acdsl": true}, "entries": []}`), 0o644))

		presence := DetectContext(repo)
		assert.Equal(t, ContextDeployed, presence.State)
		assert.Equal(t, "docs", presence.ContextDir)
		assert.Equal(t, []string{"go"}, presence.Langs)
		assert.True(t, presence.Acdsl)
	})

	t.Run("no-context-json-anywhere", func(t *testing.T) {
		repo := t.TempDir()
		// an old-layout dir without context.json is still none
		require.NoError(t, os.MkdirAll(filepath.Join(repo, "context", "general"), 0o755))
		presence := DetectContext(repo)
		assert.Equal(t, ContextNone, presence.State)
		assert.Empty(t, presence.Langs)
	})
}

func TestDetectContextCoverage(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   ContextCoverage
		_shouldPass bool

		presence   ContextPresence
		repoName   string
		repoPath   string
		sourcePath string
	}

	writeIndex := func(dir, name, content string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
		return path
	}
	sourceIndex := `{
		"aspects": [{"name": "GOLANG", "class": "scope", "lang": "go"}],
		"entries": [
			{"id": "ACTION-NAV-001", "scope": "NAV", "reach": "global"},
			{"id": "RULE-GOLANG-ERR-001", "scope": "GOLANG", "reach": "global"},
			{"id": "FACT-REPO-ARCH-001", "scope": "REPO", "reach": "othertarget"}
		]
	}`

	tests := make([]*testCase, 0)

	// fully-synced
	repo := t.TempDir()
	writeIndex(repo, "docs/context.json", `{"entries": [{"id": "ACTION-NAV-001"}, {"id": "RULE-GOLANG-ERR-001"}]}`)
	tests = append(tests, &testCase{
		_id:         "fully-synced",
		_expected:   ContextCoverage{Expected: 2},
		_shouldPass: true,

		presence:   ContextPresence{ContextDir: "docs", Langs: []string{"go"}, State: ContextDeployed},
		repoName:   "demo",
		repoPath:   repo,
		sourcePath: writeIndex(t.TempDir(), "context.json", sourceIndex),
	})

	// missing-entry
	repo = t.TempDir()
	writeIndex(repo, "docs/context.json", `{"entries": [{"id": "RULE-GOLANG-ERR-001"}]}`)
	tests = append(tests, &testCase{
		_id:         "missing-entry",
		_expected:   ContextCoverage{Expected: 2, Missing: []string{"ACTION-NAV-001"}},
		_shouldPass: true,

		presence:   ContextPresence{ContextDir: "docs", Langs: []string{"go"}, State: ContextDeployed},
		repoName:   "demo",
		repoPath:   repo,
		sourcePath: writeIndex(t.TempDir(), "context.json", sourceIndex),
	})

	// lang-bound-scope-not-synced
	repo = t.TempDir()
	writeIndex(repo, "docs/context.json", `{"entries": [{"id": "ACTION-NAV-001"}]}`)
	tests = append(tests, &testCase{
		_id:         "lang-bound-scope-not-synced",
		_expected:   ContextCoverage{Expected: 1},
		_shouldPass: true,

		presence:   ContextPresence{ContextDir: "docs", State: ContextDeployed},
		repoName:   "demo",
		repoPath:   repo,
		sourcePath: writeIndex(t.TempDir(), "context.json", sourceIndex),
	})

	// reach-covers-other-target
	repo = t.TempDir()
	writeIndex(repo, "docs/context.json", `{"entries": [{"id": "ACTION-NAV-001"}, {"id": "RULE-GOLANG-ERR-001"}, {"id": "FACT-REPO-ARCH-001"}]}`)
	tests = append(tests, &testCase{
		_id:         "reach-covers-other-target",
		_expected:   ContextCoverage{Expected: 3},
		_shouldPass: true,

		presence:   ContextPresence{ContextDir: "docs", Langs: []string{"go"}, State: ContextDeployed},
		repoName:   "othertarget",
		repoPath:   repo,
		sourcePath: writeIndex(t.TempDir(), "context.json", sourceIndex),
	})

	// non-deployed-zero-value
	tests = append(tests, &testCase{
		_id:         "non-deployed-zero-value",
		_expected:   ContextCoverage{},
		_shouldPass: true,

		presence:   ContextPresence{ContextDir: "context", State: ContextSource},
		repoName:   "demo",
		repoPath:   t.TempDir(),
		sourcePath: filepath.Join(t.TempDir(), "context.json"),
	})

	// unreadable-target-index
	tests = append(tests, &testCase{
		_id:         "unreadable-target-index",
		_expected:   ContextCoverage{},
		_shouldPass: false,

		presence:   ContextPresence{ContextDir: "docs", State: ContextDeployed},
		repoName:   "demo",
		repoPath:   t.TempDir(),
		sourcePath: writeIndex(t.TempDir(), "context.json", sourceIndex),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			coverage, err := DetectContextCoverage(test.sourcePath, test.repoName, test.repoPath, test.presence)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, coverage)
		})
	}
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

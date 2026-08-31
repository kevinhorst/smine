package repos

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

	t.Run("reserved-name-fails", func(t *testing.T) {
		registry := writeRegistry(t, `{"repos": []}`)
		require.NoError(t, registry.Reload())
		for _, name := range []string{"default", "archived"} {
			err := registry.Add(Repo{Name: name, Path: "/tmp/a"})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "is reserved")
		}
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

// touchRegistry rewrites the registry file and bumps its mtime past the
// loaded identity — same-second same-size rewrites are invisible to a bare
// write on coarse-mtime filesystems.
func touchRegistry(t *testing.T, registry *Registry, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(registry.path, []byte(content), 0o644))
	future := time.Now().Add(2 * time.Second)
	require.NoError(t, os.Chtimes(registry.path, future, future))
}

func TestRegistryRefreshOnExternalEdit(t *testing.T) {
	t.Run("external-write-visible-without-reload", func(t *testing.T) {
		registry := writeRegistry(t, `{"repos": [{"name": "a", "path": "/tmp/a"}]}`)
		require.NoError(t, registry.Reload())

		touchRegistry(t, registry, `{"repos": [
			{"name": "a", "path": "/tmp/a"},
			{"name": "b", "path": "/tmp/b"}
		]}`)
		assert.Len(t, registry.Repos(), 2)
	})

	t.Run("external-delete-visible-in-find", func(t *testing.T) {
		registry := writeRegistry(t, `{"repos": [{"name": "a", "path": "/tmp/a"}]}`)
		require.NoError(t, registry.Reload())

		touchRegistry(t, registry, `{"repos": []}`)
		_, ok := registry.Find("a")
		assert.False(t, ok)
	})

	t.Run("unchanged-file-serves-loaded-state", func(t *testing.T) {
		registry := writeRegistry(t, `{"repos": [{"name": "a", "path": "/tmp/a"}]}`)
		require.NoError(t, registry.Reload())
		assert.Len(t, registry.Repos(), 1)
		assert.Len(t, registry.Repos(), 1)
	})
}

func TestRegistryAddPreservesExternalEdit(t *testing.T) {
	t.Run("external-add-survives-in-process-add", func(t *testing.T) {
		registry := writeRegistry(t, `{"repos": [{"name": "a", "path": "/tmp/a"}]}`)
		require.NoError(t, registry.Reload())

		touchRegistry(t, registry, `{"repos": [
			{"name": "a", "path": "/tmp/a"},
			{"name": "b", "path": "/tmp/b"}
		]}`)
		require.NoError(t, registry.Add(Repo{Name: "c", Path: "/tmp/c"}))

		require.NoError(t, registry.Reload())
		assert.Len(t, registry.Repos(), 3)
	})

	t.Run("external-remove-stays-removed-after-add", func(t *testing.T) {
		registry := writeRegistry(t, `{"repos": [
			{"name": "a", "path": "/tmp/a"},
			{"name": "b", "path": "/tmp/b"}
		]}`)
		require.NoError(t, registry.Reload())

		touchRegistry(t, registry, `{"repos": [{"name": "a", "path": "/tmp/a"}]}`)
		require.NoError(t, registry.Add(Repo{Name: "c", Path: "/tmp/c"}))

		require.NoError(t, registry.Reload())
		_, ok := registry.Find("b")
		assert.False(t, ok)
		assert.Len(t, registry.Repos(), 2)
	})
}

func TestRegistryRemoveAfterExternalEdit(t *testing.T) {
	registry := writeRegistry(t, `{"repos": []}`)
	require.NoError(t, registry.Reload())

	touchRegistry(t, registry, `{"repos": [{"name": "disk-only", "path": "/tmp/d"}]}`)
	require.NoError(t, registry.Remove("disk-only"))

	require.NoError(t, registry.Reload())
	assert.Empty(t, registry.Repos())
}

func TestRefreshKeepsLastGoodStateOnParseError(t *testing.T) {
	registry := writeRegistry(t, `{"repos": [{"name": "a", "path": "/tmp/a"}]}`)
	require.NoError(t, registry.Reload())

	touchRegistry(t, registry, `{broken`)
	assert.Len(t, registry.Repos(), 1)
}

func TestRegistryDeleteStaysDeletedAcrossInstances(t *testing.T) {
	registry := writeRegistry(t, `{"repos": [
		{"name": "a", "path": "/tmp/a"},
		{"name": "b", "path": "/tmp/b"}
	]}`)
	require.NoError(t, registry.Reload())

	// A second instance (stale state loaded before the delete) writes after
	// the first instance deleted — its write must not resurrect the entry.
	stale := NewRegistry(registry.path)
	require.NoError(t, stale.Reload())

	require.NoError(t, registry.Remove("a"))
	require.NoError(t, stale.Add(Repo{Name: "c", Path: "/tmp/c"}))

	require.NoError(t, registry.Reload())
	_, ok := registry.Find("a")
	assert.False(t, ok)
	assert.Len(t, registry.Repos(), 2)
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

func TestFingerprint(t *testing.T) {
	repo := gitRepoWithAgentWorktrees(t)
	ctx := context.Background()

	first, err := Fingerprint(ctx, repo)
	require.NoError(t, err)
	assert.NotEmpty(t, first)

	// stable while nothing changes
	same, err := Fingerprint(ctx, repo)
	require.NoError(t, err)
	assert.Equal(t, first, same)

	// a new agent branch changes it
	run := func(args ...string) {
		gitArgs := append([]string{"-C", repo}, args...)
		output, err := exec.Command("git", gitArgs...).CombinedOutput()
		require.NoError(t, err, string(output))
	}
	run("branch", "claude/extra")
	afterBranch, err := Fingerprint(ctx, repo)
	require.NoError(t, err)
	assert.NotEqual(t, first, afterBranch)

	// a new worktree changes it
	run("worktree", "add", "-q", "-b", "claude/extra-wt", filepath.Join(t.TempDir(), "wt-extra"))
	afterWorktree, err := Fingerprint(ctx, repo)
	require.NoError(t, err)
	assert.NotEqual(t, afterBranch, afterWorktree)

	// a non-git dir errors
	_, err = Fingerprint(ctx, t.TempDir())
	assert.Error(t, err)
}

func TestWorktreeCounts(t *testing.T) {
	t.Run("buckets-by-branch-prefix", func(t *testing.T) {
		repo := gitRepoWithAgentWorktrees(t)
		ctx := context.Background()
		expected := []WorktreeKindCount{
			{Count: 1, Label: "claude"},
			{Count: 1, Label: "claude-routines"},
		}
		assert.Equal(t, expected, WorktreeCounts(ctx, repo, []string{"claude/", "claude-routines/"}))
	})

	t.Run("zero-count-prefix-omitted", func(t *testing.T) {
		repo := gitRepoWithAgentWorktrees(t)
		expected := []WorktreeKindCount{{Count: 1, Label: "claude"}}
		assert.Equal(t, expected, WorktreeCounts(context.Background(), repo, []string{"claude/", "feature/"}))
	})

	t.Run("non-git-dir-is-nil", func(t *testing.T) {
		assert.Nil(t, WorktreeCounts(context.Background(), t.TempDir(), []string{"claude/"}))
	})

	t.Run("no-agent-prefix-matches-nothing", func(t *testing.T) {
		repo := gitRepoWithAgentWorktrees(t)
		assert.Nil(t, WorktreeCounts(context.Background(), repo, []string{"feature/"}))
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

func TestRepoLabel(t *testing.T) {
	t.Run("roundtrips-through-add-and-reload", func(t *testing.T) {
		registry := writeRegistry(t, `{"repos": []}`)
		require.NoError(t, registry.Reload())

		require.NoError(t, registry.Add(Repo{Label: "Haushalt", Name: "b", Path: "/tmp/b"}))
		require.NoError(t, registry.Reload())
		repo, ok := registry.Find("b")
		require.True(t, ok)
		assert.Equal(t, "Haushalt", repo.Label)
	})

	t.Run("over-length-label-rejected", func(t *testing.T) {
		registry := writeRegistry(t, `{"repos": []}`)
		require.NoError(t, registry.Reload())

		long := strings.Repeat("x", 81)
		err := registry.Add(Repo{Label: long, Name: "b", Path: "/tmp/b"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid field Label")
	})

	t.Run("absent-label-omitted-from-json", func(t *testing.T) {
		registry := writeRegistry(t, `{"repos": []}`)
		require.NoError(t, registry.Reload())
		require.NoError(t, registry.Add(Repo{Name: "b", Path: "/tmp/b"}))

		content, err := os.ReadFile(registry.path)
		require.NoError(t, err)
		assert.NotContains(t, string(content), "label")
	})
}

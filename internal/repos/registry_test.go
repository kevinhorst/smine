package repos

import (
	"bytes"
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/trash"
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
		for _, name := range []string{"unknown", "archived", "always", "global", "none"} {
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

	t.Run("stashes-file-and-journals-row", func(t *testing.T) {
		previous := `{"repos": [{"env": {"SKILL_FDESIGN_BUILD": "make build", "SKILL_FDESIGN_AUDIT": "make audit"}, "name": "a", "path": "/tmp/a"}]}`
		registry := writeRegistry(t, previous)
		require.NoError(t, registry.Reload())

		require.NoError(t, registry.Remove("a"))

		store := trash.New(filepath.Dir(registry.path), 0)
		journal, err := store.Tail(2)
		require.NoError(t, err)
		require.Len(t, journal, 2)
		assert.Equal(t, "repos save", journal[0].Action)
		assert.Equal(t, trash.KindReplaced, journal[0].Kind)
		assert.Equal(t, "repos delete", journal[1].Action)
		assert.Equal(t, trash.KindRemoved, journal[1].Kind)
		assert.Equal(t, registry.path+"#a", journal[1].Path)
		assert.Equal(t, "row removed; env keys SKILL_FDESIGN_AUDIT, SKILL_FDESIGN_BUILD", journal[1].Reason)

		stashed, err := os.ReadFile(filepath.Join(filepath.Dir(registry.path), trash.TrashDir, filepath.FromSlash(journal[0].Stash)))
		require.NoError(t, err)
		assert.Equal(t, previous, string(stashed))
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

func TestRegistryFileRemovedAfterLoad(t *testing.T) {
	registry := writeRegistry(t, `{"repos": [{"name": "a", "path": "/tmp/a"}]}`)
	require.NoError(t, registry.Reload())
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	require.NoError(t, os.Remove(registry.path))

	assert.Empty(t, registry.Repos())
	assert.Contains(t, logged.String(), "removed")
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
	newRepo := func(t *testing.T, branches ...string) string {
		t.Helper()
		repo := t.TempDir()
		run := func(args ...string) {
			gitArgs := append([]string{"-C", repo, "-c", "user.email=t@t", "-c", "user.name=t"}, args...)
			output, err := exec.Command("git", gitArgs...).CombinedOutput()
			require.NoError(t, err, string(output))
		}
		run("init", "-q", "-b", "main")
		run("commit", "-q", "--allow-empty", "-m", "init")
		for _, branch := range branches {
			run("worktree", "add", "-q", "-b", branch, filepath.Join(t.TempDir(), "wt"))
		}
		return repo
	}

	t.Run("buckets-per-kind-sorted-non-agent-ignored", func(t *testing.T) {
		repo := newRepo(t, "claude/session-x", "claude-routines/nightly-x", "claude-review/station-x", "feature/x")
		expected := []WorktreeKindCount{
			{Count: 1, Label: "claude"},
			{Count: 1, Label: "claude-review"},
			{Count: 1, Label: "claude-routines"},
		}
		assert.Equal(t, expected, WorktreeCounts(context.Background(), repo))
	})

	t.Run("detached-codex-pool-counts-as-codex", func(t *testing.T) {
		repo := newRepo(t, "claude/session-x")
		codexPool := filepath.Join(t.TempDir(), ".codex", "worktrees", "ffc8", "repo")
		other := filepath.Join(t.TempDir(), "elsewhere")
		for _, path := range []string{codexPool, other} {
			gitArgs := []string{"-C", repo, "worktree", "add", "-q", "--detach", path, "HEAD"}
			output, err := exec.Command("git", gitArgs...).CombinedOutput()
			require.NoError(t, err, string(output))
		}
		expected := []WorktreeKindCount{
			{Count: 1, Label: "claude"},
			{Count: 1, Label: "codex"},
			{Count: 1, Label: "detached"},
		}
		assert.Equal(t, expected, WorktreeCounts(context.Background(), repo))
	})

	t.Run("non-git-dir-is-nil", func(t *testing.T) {
		assert.Nil(t, WorktreeCounts(context.Background(), t.TempDir()))
	})

	t.Run("no-agent-worktrees-is-nil", func(t *testing.T) {
		repo := newRepo(t, "feature/x")
		assert.Nil(t, WorktreeCounts(context.Background(), repo))
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
		require.NoError(t, contextdocs.SaveAspects(filepath.Join(repo, "context"), nil))
		for _, name := range []string{"go.md", "python.md", "plan.md", "commits.md"} {
			require.NoError(t, os.WriteFile(filepath.Join(repo, "context", "rules", name), []byte("# guide\n"), 0o644))
		}

		presence := DetectContext(repo, "")
		assert.Equal(t, ContextSource, presence.State)
		assert.Equal(t, "context", presence.ContextDir)
		assert.Equal(t, []string{"go", "python"}, presence.Langs)
		assert.False(t, presence.Acdsl)
	})

	t.Run("source-repo-skips-part-guides", func(t *testing.T) {
		repo := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(repo, "context", "rules"), 0o755))
		require.NoError(t, contextdocs.SaveAspects(filepath.Join(repo, "context"), nil))
		for _, name := range []string{"go.md", "go_test.md"} {
			require.NoError(t, os.WriteFile(filepath.Join(repo, "context", "rules", name), []byte("# guide\n"), 0o644))
		}

		presence := DetectContext(repo, "")
		assert.Equal(t, []string{"go"}, presence.Langs)
	})

	t.Run("source-repo-with-acdsl-registry", func(t *testing.T) {
		repo := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(repo, "context"), 0o755))
		require.NoError(t, contextdocs.SaveAspects(filepath.Join(repo, "context"), nil))
		require.NoError(t, os.MkdirAll(filepath.Join(repo, "acdsl"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(repo, "acdsl", "registry.json"), []byte("{}\n"), 0o644))

		presence := DetectContext(repo, "")
		assert.Equal(t, ContextSource, presence.State)
		assert.True(t, presence.Acdsl)
	})

	t.Run("deployed-written-from-record", func(t *testing.T) {
		repo := t.TempDir()
		record := filepath.Join(t.TempDir(), "context", "demo", "context.json")
		require.NoError(t, os.MkdirAll(filepath.Dir(record), 0o755))
		require.NoError(t, os.WriteFile(record,
			[]byte(`{"deploy": {"contextDir": "docs", "langs": ["go"], "acdsl": true, "writeRepo": true}, "entries": []}`), 0o644))
		require.NoError(t, os.MkdirAll(filepath.Join(repo, "docs"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(repo, "docs", "context.json"), []byte(`{"entries": []}`), 0o644))

		presence := DetectContext(repo, record)
		assert.Equal(t, ContextDeployed, presence.State)
		assert.Equal(t, "docs", presence.ContextDir)
		assert.Equal(t, []string{"go"}, presence.Langs)
		assert.True(t, presence.Acdsl)
		assert.True(t, presence.IsWritten)
		assert.Equal(t, filepath.Join(repo, "docs", "context.json"), presence.IndexPath)
	})

	t.Run("deployed-patch-only", func(t *testing.T) {
		repo := t.TempDir()
		record := filepath.Join(t.TempDir(), "context", "demo", "context.json")
		require.NoError(t, os.MkdirAll(filepath.Dir(record), 0o755))
		require.NoError(t, os.WriteFile(record,
			[]byte(`{"deploy": {"contextDir": "docs", "langs": [], "writeRepo": false}, "entries": []}`), 0o644))

		presence := DetectContext(repo, record)
		assert.Equal(t, ContextDeployed, presence.State)
		assert.False(t, presence.IsWritten)
		assert.Equal(t, record, presence.IndexPath)
	})

	t.Run("deployed-index-beside-aspects-is-not-source", func(t *testing.T) {
		repo := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(repo, "context"), 0o755))
		require.NoError(t, contextdocs.SaveAspects(filepath.Join(repo, "context"), nil))
		require.NoError(t, os.WriteFile(filepath.Join(repo, "context", "context.json"),
			[]byte(`{"deploy": {"contextDir": "context"}, "entries": []}`), 0o644))

		presence := DetectContext(repo, filepath.Join(t.TempDir(), "missing.json"))
		assert.Equal(t, ContextNone, presence.State)
	})

	t.Run("record-without-deploy-is-none", func(t *testing.T) {
		record := filepath.Join(t.TempDir(), "context.json")
		require.NoError(t, os.WriteFile(record, []byte(`{"entries": []}`), 0o644))
		presence := DetectContext(t.TempDir(), record)
		assert.Equal(t, ContextNone, presence.State)
	})

	t.Run("no-context-json-anywhere", func(t *testing.T) {
		repo := t.TempDir()
		// an old-layout dir without context.json is still none
		require.NoError(t, os.MkdirAll(filepath.Join(repo, "context", "general"), 0o755))
		presence := DetectContext(repo, "")
		assert.Equal(t, ContextNone, presence.State)
		assert.Empty(t, presence.Langs)
	})

	t.Run("generated-file-without-aspects-source-is-none", func(t *testing.T) {
		repo := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(repo, "context"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(repo, "context", "context.json"),
			[]byte(`{"entries": [], "aspects": []}`), 0o644))
		presence := DetectContext(repo, "")
		assert.Equal(t, ContextNone, presence.State)
	})
}

func TestDetectContextCoverage(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   ContextCoverage
		_shouldPass bool

		presence   ContextPresence
		repoName   string
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
	deployed := writeIndex(t.TempDir(), "docs/context.json", `{"entries": [{"id": "ACTION-NAV-001"}, {"id": "RULE-GOLANG-ERR-001"}]}`)
	tests = append(tests, &testCase{
		_id:         "fully-synced",
		_expected:   ContextCoverage{Expected: 2},
		_shouldPass: true,

		presence:   ContextPresence{IndexPath: deployed, Langs: []string{"go"}, State: ContextDeployed},
		repoName:   "demo",
		sourcePath: writeIndex(t.TempDir(), "context.json", sourceIndex),
	})

	// missing-entry
	deployed = writeIndex(t.TempDir(), "docs/context.json", `{"entries": [{"id": "RULE-GOLANG-ERR-001"}]}`)
	tests = append(tests, &testCase{
		_id:         "missing-entry",
		_expected:   ContextCoverage{Expected: 2, Missing: []string{"ACTION-NAV-001"}},
		_shouldPass: true,

		presence:   ContextPresence{IndexPath: deployed, Langs: []string{"go"}, State: ContextDeployed},
		repoName:   "demo",
		sourcePath: writeIndex(t.TempDir(), "context.json", sourceIndex),
	})

	// lang-bound-scope-not-synced
	deployed = writeIndex(t.TempDir(), "docs/context.json", `{"entries": [{"id": "ACTION-NAV-001"}]}`)
	tests = append(tests, &testCase{
		_id:         "lang-bound-scope-not-synced",
		_expected:   ContextCoverage{Expected: 1},
		_shouldPass: true,

		presence:   ContextPresence{IndexPath: deployed, State: ContextDeployed},
		repoName:   "demo",
		sourcePath: writeIndex(t.TempDir(), "context.json", sourceIndex),
	})

	// reach-covers-other-target
	deployed = writeIndex(t.TempDir(), "docs/context.json", `{"entries": [{"id": "ACTION-NAV-001"}, {"id": "RULE-GOLANG-ERR-001"}, {"id": "FACT-REPO-ARCH-001"}]}`)
	tests = append(tests, &testCase{
		_id:         "reach-covers-other-target",
		_expected:   ContextCoverage{Expected: 3},
		_shouldPass: true,

		presence:   ContextPresence{IndexPath: deployed, Langs: []string{"go"}, State: ContextDeployed},
		repoName:   "othertarget",
		sourcePath: writeIndex(t.TempDir(), "context.json", sourceIndex),
	})

	// non-deployed-zero-value
	tests = append(tests, &testCase{
		_id:         "non-deployed-zero-value",
		_expected:   ContextCoverage{},
		_shouldPass: true,

		presence:   ContextPresence{ContextDir: "context", State: ContextSource},
		repoName:   "demo",
		sourcePath: filepath.Join(t.TempDir(), "context.json"),
	})

	// unreadable-target-index
	tests = append(tests, &testCase{
		_id:         "unreadable-target-index",
		_expected:   ContextCoverage{},
		_shouldPass: false,

		presence:   ContextPresence{IndexPath: filepath.Join(t.TempDir(), "docs", "context.json"), State: ContextDeployed},
		repoName:   "demo",
		sourcePath: writeIndex(t.TempDir(), "context.json", sourceIndex),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			coverage, err := DetectContextCoverage(test.sourcePath, test.repoName, test.presence)
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

func TestRepoValidate_Env(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool

		repo *Repo
	}

	tests := make([]*testCase, 0)

	// pass-env-set
	envSet := map[string]string{
		"SKILL_FIMPLEMENT_BUILD": "make build",
		"SKILL_FIMPLEMENT_TEST":  `go test -run "TestA B" ./...`,
	}
	tests = append(tests, &testCase{
		_id:         "pass-env-set",
		_shouldPass: true,

		repo: &Repo{Env: envSet, Name: "a", Path: "/tmp/a"},
	})

	// pass-no-env
	tests = append(tests, &testCase{
		_id:         "pass-no-env",
		_shouldPass: true,

		repo: &Repo{Env: nil, Name: "a", Path: "/tmp/a"},
	})

	// fail-bad-key
	badKeyEnv := map[string]string{"FIMPLEMENT_BUILD": "make build"}
	tests = append(tests, &testCase{
		_id:         "fail-bad-key",
		_shouldPass: false,

		repo: &Repo{Env: badKeyEnv, Name: "a", Path: "/tmp/a"},
	})

	// fail-empty-value
	emptyValueEnv := map[string]string{"SKILL_FIMPLEMENT_BUILD": " \n\t"}
	tests = append(tests, &testCase{
		_id:         "fail-empty-value",
		_shouldPass: false,

		repo: &Repo{Env: emptyValueEnv, Name: "a", Path: "/tmp/a"},
	})

	// pass-multi-line-value
	multiLineEnv := map[string]string{"SKILL_FIMPLEMENT_TEST": "go test ./...\nmake test-shell"}
	tests = append(tests, &testCase{
		_id:         "pass-multi-line-value",
		_shouldPass: true,

		repo: &Repo{Env: multiLineEnv, Name: "a", Path: "/tmp/a"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := test.repo.Validate()
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
		})
	}
}

func TestRegistryFindByPath(t *testing.T) {
	type testCase struct {
		_id           string
		_expectedName string
		_isFound      bool

		path string
	}

	registry := writeRegistry(t, `{"repos": [
		{"name": "a", "path": "/tmp/a"},
		{"name": "b", "path": "/tmp/b"}
	]}`)
	require.NoError(t, registry.Reload())

	tests := make([]*testCase, 0)

	// match
	tests = append(tests, &testCase{
		_id:           "match",
		_expectedName: "b",
		_isFound:      true,

		path: "/tmp/b",
	})

	// trailing-slash-match
	tests = append(tests, &testCase{
		_id:           "trailing-slash-match",
		_expectedName: "a",
		_isFound:      true,

		path: "/tmp/a/",
	})

	// unknown
	tests = append(tests, &testCase{
		_id:           "unknown",
		_expectedName: "",
		_isFound:      false,

		path: "/tmp/c",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			repo, ok := registry.FindByPath(test.path)
			assert.Equal(t, test._isFound, ok)
			assert.Equal(t, test._expectedName, repoName(repo))
		})
	}
}

func repoName(repo *Repo) string {
	if repo == nil {
		return ""
	}
	return repo.Name
}

func TestRegistryUpdate(t *testing.T) {
	t.Run("env-row-replaced-whole", func(t *testing.T) {
		registry := writeRegistry(t, `{"repos": [
			{"name": "a", "path": "/tmp/a"},
			{"name": "b", "path": "/tmp/b", "env": {"SKILL_FIMPLEMENT_BUILD": "make build", "SKILL_FIMPLEMENT_TEST": "make test"}}
		]}`)
		require.NoError(t, registry.Reload())

		replacedEnv := map[string]string{"SKILL_FIMPLEMENT_TEST": "go test ./..."}
		require.NoError(t, registry.Update(Repo{Env: replacedEnv, Name: "b", Path: "/tmp/b"}))

		require.NoError(t, registry.Reload())
		expected := []Repo{
			{Name: "a", Path: "/tmp/a"},
			{Env: map[string]string{"SKILL_FIMPLEMENT_TEST": "go test ./..."}, Name: "b", Path: "/tmp/b"},
		}
		assert.Equal(t, expected, registry.Repos())
	})

	t.Run("unknown-name-fails-file-unchanged", func(t *testing.T) {
		registry := writeRegistry(t, `{"repos": [{"name": "a", "path": "/tmp/a"}]}`)
		require.NoError(t, registry.Reload())

		err := registry.Update(Repo{Name: "nope", Path: "/tmp/nope"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Unknown repo name")

		require.NoError(t, registry.Reload())
		assert.Equal(t, []Repo{{Name: "a", Path: "/tmp/a"}}, registry.Repos())
	})

	t.Run("external-edit-survives-update", func(t *testing.T) {
		registry := writeRegistry(t, `{"repos": [{"name": "a", "path": "/tmp/a"}]}`)
		require.NoError(t, registry.Reload())

		touchRegistry(t, registry, `{"repos": [
			{"name": "a", "path": "/tmp/a"},
			{"name": "b", "path": "/tmp/b"}
		]}`)
		updatedEnv := map[string]string{"SKILL_FIMPLEMENT_TEST": "go test ./..."}
		require.NoError(t, registry.Update(Repo{Env: updatedEnv, Name: "a", Path: "/tmp/a"}))

		require.NoError(t, registry.Reload())
		expected := []Repo{
			{Env: map[string]string{"SKILL_FIMPLEMENT_TEST": "go test ./..."}, Name: "a", Path: "/tmp/a"},
			{Name: "b", Path: "/tmp/b"},
		}
		assert.Equal(t, expected, registry.Repos())
	})
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

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// reposTestEnv builds a registry with one git-backed repo and echo-stub
// worktree scripts; script semantics live in the bash tests (§25).
func reposTestEnv(t *testing.T) (*Server, string, string) {
	t.Helper()

	repo := t.TempDir()
	gitRun := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))
	}
	gitRun("init", "-q")
	gitRun("config", "user.email", "test@example.com")
	gitRun("config", "user.name", "test")
	require.NoError(t, os.WriteFile(filepath.Join(repo, "base"), []byte("base\n"), 0o644))
	gitRun("add", "base")
	gitRun("commit", "-qm", "base")
	gitRun("branch", "claude/feature")

	scriptsDir := t.TempDir()
	scripts := map[string]string{
		"print_agent_worktrees_status.sh": "#!/bin/sh\necho 'No claude/* branches found.'\n",
		"sync_worktrees.sh":               "#!/bin/sh\necho \"sync $*\"\n",
		"merge_worktree.sh":               "#!/bin/sh\necho \"merge $*\"\n",
		"cherry_pick_worktree.sh":         "#!/bin/sh\necho \"pick $*\"\n",
		"remove_agent_worktrees.sh":       "#!/bin/sh\necho \"remove $*\"\n",
	}
	for name, content := range scripts {
		require.NoError(t, os.WriteFile(filepath.Join(scriptsDir, name), []byte(content), 0o755))
	}

	reposPath := filepath.Join(t.TempDir(), "repos.json")
	registry := `{"repos": [{"name": "demo", "path": "` + repo + `"}]}`
	require.NoError(t, os.WriteFile(reposPath, []byte(registry), 0o644))

	server := newTestServer(t, &Options{
		PeekEndpoint:       "http://127.0.0.1:1/mcp",
		ReposPath:          reposPath,
		WorktreeScriptsDir: scriptsDir,
	})
	return server, repo, reposPath
}

func TestReposIndexRendersRegistryRows(t *testing.T) {
	server, repo, _ := reposTestEnv(t)
	// Claude: CLAUDE.md + .claude (created by the pool below) → full check.
	require.NoError(t, os.WriteFile(filepath.Join(repo, "CLAUDE.md"), []byte("x\n"), 0o644))
	// Context: general + one language dir.
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "context", "general"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "context", "go"), 0o755))
	// One real worktree (own .git file) plus a hollow pool dir.
	pool := filepath.Join(repo, ".claude", "worktrees")
	require.NoError(t, os.MkdirAll(filepath.Join(pool, "wt-a"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pool, "wt-a", ".git"), []byte("gitdir: elsewhere\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(pool, "hollow"), 0o755))

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "demo")
	assert.Contains(t, body, `<span class="badge badge-ok" title="CLAUDE.md · .claude">✓</span>`)
	assert.Contains(t, body, `<span class="badge badge-error" title="missing AGENTS.md · .codex">✗</span>`)
	assert.Contains(t, body, `<span class="badge badge-ok" title="context/general">general</span>`)
	assert.Contains(t, body, "<th>Languages</th>")
	assert.Contains(t, body, `<span class="badge badge-dim">go</span>`)
	assert.Contains(t, body, "<td>1</td>")
	assert.Contains(t, body, `<option value="demo">demo</option>`)
}

func TestReposIndexPartialPresenceIsQuestionMark(t *testing.T) {
	server, repo, _ := reposTestEnv(t)
	require.NoError(t, os.WriteFile(filepath.Join(repo, "CLAUDE.md"), []byte("x\n"), 0o644))

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, `<span class="badge badge-action" title="missing .claude">?</span>`)
	// no context dir → red cross with the empty state in the tooltip
	assert.Contains(t, body, `<span class="badge badge-error" title="no context/general">✗</span>`)
}

func TestRepoDetailUnknownRepoIs404(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/nope", nil))
	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestRepoDetailDegradedPeekStillRenders(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "session column degraded")
}

func TestRepoSyncRendersStubOutput(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/repos/demo/branches/feature/sync", url.Values{}))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "sync claude/feature")
	assert.Contains(t, response.Body.String(), ">Ok</span>")
	assert.Equal(t, "repo-op", response.Header().Get("HX-Trigger"))
}

func TestRepoDetailWorktreeColumnFlagsDirBranchMismatch(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	statusStub := "#!/bin/sh\ncat <<'EOF'\n" +
		"#    BRANCH        FROM         DIRTY  UNTRACKED  AHEAD  BEHIND  UNPICKED  VERDICTS    MERGED  IN            LAST-COMMIT      WORKTREE\n" +
		"1    claude/alpha  main         0      0          0      0       0         -           -       main          2026-07-15 10:00 /repo/.claude/worktrees/alpha\n" +
		"2    claude/beta   origin/main  0      0          1      2       0         resolved:1  -       origin/main*  2026-07-15 10:00 /repo/.claude/worktrees/recycled-dir\n" +
		"EOF\n"
	stubPath := filepath.Join(server.worktreeScripts, "print_agent_worktrees_status.sh")
	require.NoError(t, os.WriteFile(stubPath, []byte(statusStub), 0o755))

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()

	assert.Contains(t, body, "<th>Worktree</th>")
	assert.Contains(t, body, ">Base</th>")
	assert.Contains(t, body, ">Verdicts</th>")
	assert.Contains(t, body, ">Merged into</th>")
	assert.Contains(t, body, "<th>Safe</th>")
	assert.Contains(t, body, `<span title="/repo/.claude/worktrees/alpha">alpha</span>`)
	// Only the pool-recycled row (dir name != branch slug) carries the marker.
	assert.Equal(t, 1, strings.Count(body, "dir≠branch"))
	// The probe-upgraded row renders its verdict summary and the safe-via-probe tooltip.
	assert.Contains(t, body, ">resolved:1</td>")
	assert.Contains(t, body, "safe via applied-probe")
	// Ops widget + per-row radios carry the data the ops JS reads.
	assert.Contains(t, body, `id="worktree-ops" data-repo="demo"`)
	assert.Contains(t, body, ">Remove</button>")
	assert.Contains(t, body, ">Remove Worktree</button>")
	assert.Contains(t, body, `data-branch="claude/beta" data-worktree="/repo/.claude/worktrees/recycled-dir"`)
	// Both fixture rows are safe (alpha exact-contained, beta via probe) → ✓ badge, twice.
	assert.Equal(t, 2, strings.Count(body, `data-safe="1"`))
}

func TestRepoDetailMergedIntoFilter(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	statusStub := "#!/bin/sh\ncat <<'EOF'\n" +
		"#    BRANCH        FROM  DIRTY  UNTRACKED  AHEAD  BEHIND  UNPICKED  VERDICTS  MERGED  IN        LAST-COMMIT      WORKTREE\n" +
		"1    claude/alpha  main  0      0          0      0       0         -         -       main      2026-07-15 10:00 /repo/.claude/worktrees/alpha\n" +
		"2    claude/beta   main  0      0          0      0       0         -         dev     dev,main  2026-07-15 10:00 /repo/.claude/worktrees/beta\n" +
		"EOF\n"
	stubPath := filepath.Join(server.worktreeScripts, "print_agent_worktrees_status.sh")
	require.NoError(t, os.WriteFile(stubPath, []byte(statusStub), 0o755))

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo?merged=dev", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()

	// filter badges offer every target from the unfiltered list; dev is active
	assert.Contains(t, body, ">all</a>")
	assert.Contains(t, body, `?merged=main`)
	assert.Contains(t, body, `class="badge badge-ok"
     href="/repos/demo?merged=dev"`)
	// only the dev row survives the filter
	assert.Contains(t, body, "claude/beta")
	assert.NotContains(t, body, "claude/alpha")
	// the auto-refresh URL keeps the filter
	assert.Contains(t, body, `hx-get="/repos/demo?merged=dev"`)
}

func TestRepoRemoveDeleteBranchCheckbox(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/repos/demo/branches/feature/remove", url.Values{"delete-branch": {"on"}}))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "remove --delete-branch claude/feature")
}

func TestRepoRemoveForceCheckbox(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/repos/demo/branches/feature/remove", url.Values{"force": {"on"}}))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "remove --force claude/feature")
}

func TestRepoRemoveSelected(t *testing.T) {
	t.Run("removes-every-selected-branch", func(t *testing.T) {
		server, _, _ := reposTestEnv(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/demo/remove-selected",
			url.Values{"branch": {"feature", "feature"}, "delete-branch": {"on"}}))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Equal(t, 2, strings.Count(body, "remove --delete-branch claude/feature"))
	})

	t.Run("force-flag-passed-through", func(t *testing.T) {
		server, _, _ := reposTestEnv(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/demo/remove-selected",
			url.Values{"branch": {"feature"}, "force": {"on"}}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "remove --force claude/feature")
	})

	t.Run("no-selection-400", func(t *testing.T) {
		server, _, _ := reposTestEnv(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/demo/remove-selected", url.Values{}))
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})
}

func TestRepoOpConcurrent409(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	require.True(t, server.repoLocks.TryAcquire("demo"))
	defer server.repoLocks.Release("demo")

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/repos/demo/branches/feature/sync", url.Values{}))
	assert.Equal(t, http.StatusConflict, response.Code)
}

func TestRepoCommitsClassAllowlist(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo/branches/feature/commits/evil", nil))
	assert.Equal(t, http.StatusNotFound, response.Code)
}

func readRegistryFile(t *testing.T, reposPath string) map[string][]map[string]string {
	t.Helper()
	data, err := os.ReadFile(reposPath)
	require.NoError(t, err)
	var file map[string][]map[string]string
	require.NoError(t, json.Unmarshal(data, &file))
	return file
}

func TestReposAdd(t *testing.T) {
	t.Run("adds-and-persists", func(t *testing.T) {
		server, _, reposPath := reposTestEnv(t)
		dir := filepath.Join(t.TempDir(), "newrepo")
		require.NoError(t, os.Mkdir(dir, 0o755))

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/add", url.Values{"path": {dir}}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "added newrepo")
		assert.Equal(t, "repo-op", response.Header().Get("HX-Trigger"))

		file := readRegistryFile(t, reposPath)
		require.Len(t, file["repos"], 2)
		assert.Equal(t, "newrepo", file["repos"][1]["name"])
		assert.Equal(t, dir, file["repos"][1]["path"])

		// The repo-op trigger makes #repo-list re-pull the index.
		refresh := httptest.NewRecorder()
		server.Handler().ServeHTTP(refresh, httptest.NewRequest(http.MethodGet, "/repos", nil))
		require.Equal(t, http.StatusOK, refresh.Code)
		assert.Contains(t, refresh.Body.String(), "newrepo")
	})

	t.Run("trailing-slash-uses-folder-basename", func(t *testing.T) {
		server, _, reposPath := reposTestEnv(t)
		dir := filepath.Join(t.TempDir(), "slashed")
		require.NoError(t, os.Mkdir(dir, 0o755))

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/add", url.Values{"path": {dir + "/"}}))
		require.Equal(t, http.StatusOK, response.Code)

		file := readRegistryFile(t, reposPath)
		require.Len(t, file["repos"], 2)
		assert.Equal(t, "slashed", file["repos"][1]["name"])
		assert.Equal(t, dir, file["repos"][1]["path"])
	})

	t.Run("empty-path-is-bad-request", func(t *testing.T) {
		server, _, _ := reposTestEnv(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/add", url.Values{}))
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("duplicate-name-renders-failure", func(t *testing.T) {
		server, repo, reposPath := reposTestEnv(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/add", url.Values{"path": {filepath.Dir(repo) + "/demo"}}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), ">Failed</span>")
		assert.Contains(t, response.Body.String(), "Duplicate repo name")
		assert.Len(t, readRegistryFile(t, reposPath)["repos"], 1)
	})
}

func TestReposDelete(t *testing.T) {
	t.Run("removes-and-persists", func(t *testing.T) {
		server, _, reposPath := reposTestEnv(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/delete", url.Values{"name": {"demo"}}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "removed demo")
		assert.Equal(t, "repo-op", response.Header().Get("HX-Trigger"))
		assert.Empty(t, readRegistryFile(t, reposPath)["repos"])

		refresh := httptest.NewRecorder()
		server.Handler().ServeHTTP(refresh, httptest.NewRequest(http.MethodGet, "/repos", nil))
		require.Equal(t, http.StatusOK, refresh.Code)
		assert.Contains(t, refresh.Body.String(), "No repos registered.")
	})

	t.Run("unknown-name-renders-failure", func(t *testing.T) {
		server, _, reposPath := reposTestEnv(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/delete", url.Values{"name": {"nope"}}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), ">Failed</span>")
		assert.Len(t, readRegistryFile(t, reposPath)["repos"], 1)
	})

	t.Run("empty-name-is-bad-request", func(t *testing.T) {
		server, _, _ := reposTestEnv(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/delete", url.Values{}))
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})
}

func TestReposChooseFolder(t *testing.T) {
	stubPath := func(t *testing.T, body string) {
		t.Helper()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "osascript"), []byte(body), 0o755))
		t.Setenv("PATH", dir)
	}

	t.Run("picked-path-fills-value", func(t *testing.T) {
		server, _, _ := reposTestEnv(t)
		stubPath(t, "#!/bin/sh\necho '/tmp/chosen/'\n")
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/choose-folder", url.Values{"path": {"/old"}}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `value="/tmp/chosen/"`)
	})

	t.Run("cancel-keeps-typed-value", func(t *testing.T) {
		server, _, _ := reposTestEnv(t)
		stubPath(t, "#!/bin/sh\necho 'execution error: User canceled. (-128)' >&2\nexit 1\n")
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/choose-folder", url.Values{"path": {"/typed/path"}}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `value="/typed/path"`)
		assert.NotContains(t, response.Body.String(), "picker failed")
	})
}

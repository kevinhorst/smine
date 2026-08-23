package server

import (
	"encoding/json"
	"errors"
	"fmt"
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
		"print_agent_worktrees_status.sh": "#!/bin/sh\necho 'No agent branches (claude/*, claude-routines/*) found.'\n",
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

	// Source context index for the coverage column: one global entry.
	contextDir := t.TempDir()
	sourceIndex := `{"aspects": [], "entries": [{"id": "ACTION-NAV-001", "kind": "action", "scope": "NAV", "reach": "global"}]}`
	require.NoError(t, os.WriteFile(filepath.Join(contextDir, "context.json"), []byte(sourceIndex), 0o644))

	server := newTestServer(t, &Options{
		ContextDir:         contextDir,
		PeekEndpoint:       "http://127.0.0.1:1/mcp",
		ReposPath:          reposPath,
		WorktreeScriptsDir: scriptsDir,
	})
	return server, repo, reposPath
}

func TestReposIndexRendersRegistryRows(t *testing.T) {
	server, repo, _ := reposTestEnv(t)
	// Context: a fully synced target — docs/context.json carries the deploy
	// section, the acdsl flag and the one entry the source index expects.
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "docs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "docs", "context.json"),
		[]byte(`{"deploy": {"contextDir": "docs", "langs": ["go"], "acdsl": true}, "entries": [{"id": "ACTION-NAV-001"}]}`), 0o644))
	// One real claude/* worktree so the Worktrees column counts 1.
	gitInitWithWorktrees(t, repo, "claude/session-x")

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "demo")
	assert.Contains(t, body, `synced:docs`)
	assert.Contains(t, body, "all 1 entries reaching this repo are deployed")
	assert.Contains(t, body, ">on</span>")
	assert.NotContains(t, body, "<th>Languages</th>")
	assert.NotContains(t, body, "<th>Claude</th>")
	assert.Contains(t, body, "<td>1</td>")
	assert.Contains(t, body, `<option value="demo">demo</option>`)
}

// gitInitWithWorktrees turns dir into a git repo and checks out one worktree
// per branch name outside the repo (routine worktrees live outside too).
func gitInitWithWorktrees(t *testing.T, dir string, branches ...string) {
	t.Helper()
	run := func(args ...string) {
		gitArgs := append([]string{"-C", dir, "-c", "user.email=t@t", "-c", "user.name=t"}, args...)
		output, err := exec.Command("git", gitArgs...).CombinedOutput()
		require.NoError(t, err, string(output))
	}
	run("init", "-q", "-b", "main")
	run("commit", "-q", "--allow-empty", "-m", "init")
	for _, branch := range branches {
		run("worktree", "add", "-q", "-b", branch, filepath.Join(t.TempDir(), "wt"))
	}
}

func TestReposIndexCountsAgentWorktrees(t *testing.T) {
	// copy_trader_flask_server_llm-style long name with one worktree of each
	// kind — the count column sees both, the name wraps at underscores.
	repo := t.TempDir()
	gitInitWithWorktrees(t, repo, "claude/session-x", "claude-routines/nightly-x")
	reposPath := filepath.Join(t.TempDir(), "repos.json")
	registry := `{"repos": [{"name": "copy_trader_flask_server_llm", "path": "` + repo + `"}]}`
	require.NoError(t, os.WriteFile(reposPath, []byte(registry), 0o644))

	server := newTestServer(t, &Options{ReposPath: reposPath})

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "copy_<wbr>trader_<wbr>flask_<wbr>server_<wbr>llm")
	assert.Contains(t, body, "<td>2</td>")
}

func TestWorktreeStatusFragmentKindFilter(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	statusStub := "#!/bin/sh\ncat <<'EOF'\n" +
		"#    BRANCH                    FROM  DIRTY  UNTRACKED  AHEAD  BEHIND  UNPICKED  VERDICTS  MERGED  IN    LAST-COMMIT      WORKTREE\n" +
		"1    claude/alpha              main  0      0          0      0       0         -         -       main  2026-07-15 10:00 /repo/.claude/worktrees/alpha\n" +
		"2    claude-routines/nightly   main  0      0          0      0       0         -         -       main  2026-07-15 10:00 /cache/worktrees/nightly\n" +
		"EOF\n"
	stubPath := filepath.Join(server.worktreeScripts, "print_agent_worktrees_status.sh")
	require.NoError(t, os.WriteFile(stubPath, []byte(statusStub), 0o755))

	// claude: only the session worktree row.
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo/status?kind=claude", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "claude/alpha")
	assert.NotContains(t, body, "claude-routines/nightly")
	// the auto-refresh URL keeps the filter
	assert.Contains(t, body, `hx-get="/repos/demo/status?kind=claude"`)

	// claude-routines: only the routine lineage row.
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo/status?kind=claude-routines", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body = response.Body.String()
	assert.Contains(t, body, "claude-routines/nightly")
	assert.NotContains(t, body, "claude/alpha")

	// kind chips carry an active base filter and vice versa.
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo/status?base=main&kind=claude", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body = response.Body.String()
	assert.Contains(t, body, `?base=main&amp;kind=claude-routines`)
	assert.Contains(t, body, `?kind=claude"`) // base "all" chip keeps the kind
}

func TestReposIndexPartialCoverage(t *testing.T) {
	server, repo, _ := reposTestEnv(t)
	// Deployed, but the expected entry is absent from the target index.
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "docs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "docs", "context.json"),
		[]byte(`{"deploy": {"contextDir": "docs", "langs": []}, "entries": []}`), 0o644))

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "partial 0/1")
	assert.Contains(t, body, "re-sync — missing: ACTION-NAV-001")
	assert.Contains(t, body, ">off</span>")
}

func TestReposIndexNoContextIsCross(t *testing.T) {
	server, _, _ := reposTestEnv(t)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	// no context.json anywhere → red cross, acdsl off
	assert.Contains(t, body, `<span class="badge badge-error" title="no context.json under context/ or docs/">✗</span>`)
	assert.Contains(t, body, ">off</span>")
}

func TestRepoDetailUnknownRepoIs404(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/nope", nil))
	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestRepoDetailShellRendersPlaceholderOnly(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	// The shell never runs the status pipeline — placeholder + lazy fragment
	// fetch only (D3).
	assert.Contains(t, body, "Loading worktree status…")
	assert.Contains(t, body, `hx-get="/repos/demo/status"`)
	assert.Contains(t, body, `hx-trigger="load"`)
	assert.NotContains(t, body, ">Branch</th>")
}

func TestWorktreeStatusFragmentDegradedPeekStillRenders(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo/status", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	// Banner label carries the root cause + endpoint; the full wrapped chain
	// lives only in the title tooltip.
	assert.Contains(t, body, "session column degraded")
	assert.Contains(t, body, "at http://127.0.0.1:1/mcp</span>")
	assert.Contains(t, body, `title="Client.connect:`)
	// The swapped-in fragment refreshes on repo-op only — a `load` trigger
	// would re-fire on every swap and loop (D3).
	assert.Contains(t, body, `hx-trigger="repo-op from:body"`)
	assert.NotContains(t, body, `hx-trigger="load"`)
	// Timing line present (DR3).
	assert.Contains(t, body, "status ")
	assert.Contains(t, body, "ms</div>")
}

func TestRootCause(t *testing.T) {
	inner := errors.New("connection refused")
	wrapped := fmt.Errorf("Initialize failed: %w", fmt.Errorf("transport error: %w", inner))
	assert.Equal(t, inner, rootCause(wrapped))
	assert.Equal(t, inner, rootCause(inner))
}

func TestRepoSyncRendersStubOutput(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/repos/demo/branches/claude%2Ffeature/sync", url.Values{}))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "sync claude/feature")
	assert.Contains(t, response.Body.String(), ">Ok</span>")
	assert.Equal(t, "repo-op", response.Header().Get("HX-Trigger"))
}

func TestWorktreeStatusFragmentFlagsDirBranchMismatch(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	statusStub := "#!/bin/sh\ncat <<'EOF'\n" +
		"#    BRANCH        FROM         DIRTY  UNTRACKED  AHEAD  BEHIND  UNPICKED  VERDICTS    MERGED  IN            LAST-COMMIT      WORKTREE\n" +
		"1    claude/alpha  main         0      0          0      0       0         -           -       main          2026-07-15 10:00 /repo/.claude/worktrees/alpha\n" +
		"2    claude/beta   origin/main  0      0          1      2       0         resolved:1  -       origin/main*  2026-07-15 10:00 /repo/.claude/worktrees/recycled-dir\n" +
		"EOF\n"
	stubPath := filepath.Join(server.worktreeScripts, "print_agent_worktrees_status.sh")
	require.NoError(t, os.WriteFile(stubPath, []byte(statusStub), 0o755))

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo/status", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()

	assert.Contains(t, body, ">Worktree</th>")
	assert.Contains(t, body, ">Base</th>")
	assert.Contains(t, body, ">Verdicts</th>")
	assert.Contains(t, body, ">Merged into</th>")
	// The safe badge renders inside the first cell, next to the checkbox;
	// the standalone Safe column is gone.
	assert.NotContains(t, body, ">Safe</th>")
	assert.Contains(t, body, `<input type="checkbox" class="select-all"`)
	assert.Contains(t, body, `<span title="/repo/.claude/worktrees/alpha">alpha</span>`)
	// Only the pool-recycled row (dir name != branch slug) carries the marker.
	assert.Equal(t, 1, strings.Count(body, "dir≠branch"))
	// The probe-upgraded row renders its verdict summary and the safe-via-probe tooltip.
	assert.Contains(t, body, ">resolved:1</td>")
	assert.Contains(t, body, "safe via probe")
	// Ops widget + per-row radios carry the data the ops JS reads.
	assert.Contains(t, body, `id="worktree-ops" data-repo="demo"`)
	assert.Contains(t, body, ">Remove</button>")
	assert.Contains(t, body, ">Remove Worktree</button>")
	assert.Contains(t, body, `data-branch="claude/beta" data-worktree="/repo/.claude/worktrees/recycled-dir"`)
	// Both fixture rows are safe (alpha exact-contained, beta via probe) → ✓ badge, twice.
	assert.Equal(t, 2, strings.Count(body, `data-safe="1"`))
}

func TestWorktreeStatusFragmentSafeCellRendersReasonsAndResolvedMarker(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	// Row 1: worktree gone, work transferred via manually resolved picks —
	// safe, with the visible "resolved" marker. Row 2: unsafe, and the cell
	// names the blocking conditions instead of a generic ✗.
	statusStub := "#!/bin/sh\ncat <<'EOF'\n" +
		"#    BRANCH          FROM  DIRTY  UNTRACKED  AHEAD  BEHIND  UNPICKED  VERDICTS           MERGED  IN     LAST-COMMIT      WORKTREE\n" +
		"1    claude/resolved main  -      -          2      0       0         picked-resolved:2  -       main*  2026-07-15 10:00 –\n" +
		"2    claude/blocked  main  0      2          1      0       1         -                  -       -      2026-07-15 10:00 /repo/.claude/worktrees/blocked\n" +
		"EOF\n"
	stubPath := filepath.Join(server.worktreeScripts, "print_agent_worktrees_status.sh")
	require.NoError(t, os.WriteFile(stubPath, []byte(statusStub), 0o755))

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo/status", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()

	assert.Contains(t, body, "✓ resolved")
	assert.Contains(t, body, "NOT auto-reconcilable")
	assert.Contains(t, body, `title="unsafe to remove: untracked(2), unpicked(1)"`)
	assert.Equal(t, 1, strings.Count(body, `data-safe="1"`))
}

func TestWorktreeStatusFragmentBaseFilter(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	statusStub := "#!/bin/sh\ncat <<'EOF'\n" +
		"#    BRANCH        FROM     DIRTY  UNTRACKED  AHEAD  BEHIND  UNPICKED  VERDICTS  MERGED  IN        LAST-COMMIT      WORKTREE\n" +
		"1    claude/alpha  main     0      0          0      0       0         -         -       main      2026-07-15 10:00 /repo/.claude/worktrees/alpha\n" +
		"2    claude/beta   dev      0      0          0      0       0         -         dev     dev,main  2026-07-15 10:00 /repo/.claude/worktrees/beta\n" +
		"3    claude/gamma  unknown  0      0          0      0       0         -         -       main      2026-07-15 10:00 /repo/.claude/worktrees/gamma\n" +
		"EOF\n"
	stubPath := filepath.Join(server.worktreeScripts, "print_agent_worktrees_status.sh")
	require.NoError(t, os.WriteFile(stubPath, []byte(statusStub), 0o755))

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo/status?base=dev", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()

	// filter badges offer every base from the unfiltered list; dev is active
	assert.Contains(t, body, ">all</a>")
	assert.Contains(t, body, `?base=main`)
	assert.Contains(t, body, `class="badge badge-ok"
     href="/repos/demo?base=dev"`)
	// the "unknown" sentinel never becomes a chip
	assert.NotContains(t, body, `?base=unknown`)
	// only the dev-based row survives the filter
	assert.Contains(t, body, "claude/beta")
	assert.NotContains(t, body, "claude/alpha")
	assert.NotContains(t, body, "claude/gamma")
	// the auto-refresh URL keeps the filter
	assert.Contains(t, body, `hx-get="/repos/demo/status?base=dev"`)
}

func TestRepoRemoveDeleteBranchCheckbox(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/repos/demo/branches/claude%2Ffeature/remove", url.Values{"delete-branch": {"on"}}))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "remove --delete-branch claude/feature")
}

func TestRepoRemoveForceCheckbox(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/repos/demo/branches/claude%2Ffeature/remove", url.Values{"force": {"on"}}))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "remove --force claude/feature")
}

func TestRepoRemoveSelected(t *testing.T) {
	t.Run("removes-every-selected-branch", func(t *testing.T) {
		server, _, _ := reposTestEnv(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/demo/remove-selected",
			url.Values{"branch": {"claude/feature", "claude/feature"}, "delete-branch": {"on"}}))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Equal(t, 2, strings.Count(body, "remove --delete-branch claude/feature"))
		// Clean run: the optimistically hidden rows are the truth — no
		// repo-op refresh, and the op-result carries its duration (D4).
		assert.Empty(t, response.Header().Get("HX-Trigger"))
		assert.Contains(t, body, `<span class="meta">`)
	})

	t.Run("skip-output-triggers-refresh-despite-exit-0", func(t *testing.T) {
		server, _, _ := reposTestEnv(t)
		stub := "#!/bin/sh\necho \"skipped: /some/worktree (claude/feature): dirty(1)\"\n"
		require.NoError(t, os.WriteFile(
			filepath.Join(server.worktreeScripts, "remove_agent_worktrees.sh"), []byte(stub), 0o755))
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/demo/remove-selected",
			url.Values{"branch": {"claude/feature"}}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, "repo-op", response.Header().Get("HX-Trigger"))
	})

	t.Run("script-error-triggers-refresh", func(t *testing.T) {
		server, _, _ := reposTestEnv(t)
		stub := "#!/bin/sh\necho \"boom\"\nexit 1\n"
		require.NoError(t, os.WriteFile(
			filepath.Join(server.worktreeScripts, "remove_agent_worktrees.sh"), []byte(stub), 0o755))
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/demo/remove-selected",
			url.Values{"branch": {"claude/feature"}}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, "repo-op", response.Header().Get("HX-Trigger"))
		assert.Contains(t, response.Body.String(), ">Failed</span>")
	})

	t.Run("force-flag-passed-through", func(t *testing.T) {
		server, _, _ := reposTestEnv(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/demo/remove-selected",
			url.Values{"branch": {"claude/feature"}, "force": {"on"}}))
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

func TestRemovalNeedsRefresh(t *testing.T) {
	// Real script line shapes from remove_agent_worktrees.sh (D6).
	assert.False(t, removalNeedsRefresh("removed: /repo/.claude/worktrees/x\ndeleted branch: claude/x\n"))
	assert.True(t, removalNeedsRefresh("skipped: /repo/.claude/worktrees/x (claude/x): dirty(1)\n"))
	assert.True(t, removalNeedsRefresh("skipped branch: claude/x: worktree kept (use --force)\n"))
	assert.True(t, removalNeedsRefresh("no worktree checked out for claude/x — nothing removed\n"))
	// Marker mid-line (e.g. inside a path) must not trip the scan.
	assert.False(t, removalNeedsRefresh("removed: /repo/worktrees/skipped: weird/path\n"))
}

func TestRepoOpConcurrent409(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	require.True(t, server.repoLocks.TryAcquire("demo"))
	defer server.repoLocks.Release("demo")

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/repos/demo/branches/claude%2Ffeature/sync", url.Values{}))
	assert.Equal(t, http.StatusConflict, response.Code)

	// The batch-remove handler inlines the lock (D5) — same guard applies.
	remove := httptest.NewRecorder()
	server.Handler().ServeHTTP(remove, formPost("/repos/demo/remove-selected", url.Values{"branch": {"claude/feature"}}))
	assert.Equal(t, http.StatusConflict, remove.Code)
}

func TestRepoCommitsRejectsNonAgentBranch(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo/branches/main/commits/ahead", nil))
	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestRepoCommitsClassAllowlist(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo/branches/claude%2Ffeature/commits/evil", nil))
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

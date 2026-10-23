package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/trash"
)

// agentwtStub is the stub binary the server tests run in place of agentwt:
// `status` counts its runs, records their argv tail and prints document.json
// from its own directory (ignoring --kind); `remove` prints remove.json (exit
// code from remove.exit) when present, else a one-line removed result echoing
// the argv after the global flags.
const agentwtStub = `#!/bin/sh
dir=$(dirname "$0")
cmd="$5"
shift 5
if [ "$cmd" = "status" ]; then
	echo run >> "$dir/runs"
	echo "$*" >> "$dir/argv"
	cat "$dir/document.json"
	exit 0
fi
if [ "$cmd" = "remove" ]; then
	if [ -f "$dir/remove.json" ]; then
		cat "$dir/remove.json"
		if [ -f "$dir/remove.exit" ]; then exit "$(cat "$dir/remove.exit")"; fi
		exit 0
	fi
	printf '{"operation":"remove","lines":[{"subject":"%s","outcome":"removed","text":"remove %s"}]}' "$*" "$*"
	exit 0
fi
echo "agentwt: unknown command $cmd" >&2
exit 1
`

// writeAgentwtStub installs the stub with an empty status document and
// returns the binary path.
func writeAgentwtStub(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	stub := filepath.Join(dir, "agentwt.sh")
	require.NoError(t, os.WriteFile(stub, []byte(agentwtStub), 0o755))
	writeStatusDocumentAt(t, dir)
	return stub
}

// wireRow is one status-document row with every field the view reads, in the
// binary's wire shape, `overrides` replacing top-level keys.
func wireRow(overrides map[string]any) map[string]any {
	row := map[string]any{
		"branch": "claude/alpha",
		"kind":   "claude",
		"head": map[string]any{
			"sha": "0123456789abcdef0123456789abcdef01234567", "short_sha": "01234567",
			"subject": "alpha work", "committed_at": "2026-07-15T10:00:00Z",
		},
		"worktree": map[string]any{
			"path": "/repo/.claude/worktrees/alpha", "main": false, "detached": false,
			"prunable": false, "dirty": 0, "untracked": 0, "untracked_paths": []string{},
		},
		"from":   map[string]any{"name": "main", "resolution": "ref", "origin": "main"},
		"counts": map[string]any{"vs_from": map[string]int{"ahead": 0, "behind": 0}},
		"commits": map[string]any{
			"ahead": []any{}, "behind": []any{}, "behind_truncated": false,
		},
		"unpicked":       0,
		"containment":    map[string]string{"branch": "main", "reason": "ancestor", "rank": "base"},
		"safe_to_remove": true,
		"unsafe_reasons": []string{},
	}
	maps.Copy(row, overrides)
	return row
}

// writeStatusDocumentAt writes a schema-1 status document with the given rows
// next to the stub; `kinds` is the sorted distinct string kinds of the rows,
// the binary's rule.
func writeStatusDocumentAt(t *testing.T, dir string, rows ...map[string]any) {
	t.Helper()
	if rows == nil {
		rows = []map[string]any{}
	}
	kinds := make(map[string]bool)
	for _, row := range rows {
		if kind, isString := row["kind"].(string); isString {
			kinds[kind] = true
		}
	}
	document := map[string]any{
		"kinds":  slices.Sorted(maps.Keys(kinds)),
		"schema": 1,
		"repo":   map[string]any{"path": "/repo", "default_branch": "main"},
		"rows":   rows,
	}
	encoded, err := json.Marshal(document)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "document.json"), encoded, 0o644))
}

// writeStatusDocument replaces the server stub's status document.
func writeStatusDocument(t *testing.T, server *Server, rows ...map[string]any) {
	t.Helper()
	writeStatusDocumentAt(t, filepath.Dir(server.agentwt), rows...)
}

// writeRemoveResult pins the stub's remove output and exit code.
func writeRemoveResult(t *testing.T, server *Server, body string, exitCode int) {
	t.Helper()
	dir := filepath.Dir(server.agentwt)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "remove.json"), []byte(body), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "remove.exit"), []byte(strconv.Itoa(exitCode)), 0o644))
}

// statusRuns counts the stub's `status` invocations.
func statusRuns(server *Server) int {
	content, err := os.ReadFile(filepath.Join(filepath.Dir(server.agentwt), "runs"))
	if err != nil {
		return 0
	}
	return strings.Count(string(content), "run")
}

// statusArgv lists the argv tail after the subcommand of every stub `status`
// run, in run order; an unfiltered run is the empty string.
func statusArgv(server *Server) []string {
	content, err := os.ReadFile(filepath.Join(filepath.Dir(server.agentwt), "argv"))
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
}

// jsonString encodes value as a JSON string literal; a raw Windows path
// between quotes is not valid JSON.
func jsonString(t *testing.T, value string) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)

	return string(encoded)
}

// reposTestEnv builds a registry with one git-backed repo and the agentwt
// stub; the binary's semantics live in cmd/agentwt/tests.
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

	agentwt := writeAgentwtStub(t)

	reposPath := filepath.Join(t.TempDir(), "repos.json")
	registry := `{"repos": [{"name": "demo", "path": ` + jsonString(t, repo) + `}]}`
	require.NoError(t, os.WriteFile(reposPath, []byte(registry), 0o644))

	// Source context index for the coverage column: one global entry.
	contextDir := t.TempDir()
	sourceIndex := `{"aspects": [], "entries": [{"id": "ACTION-NAV-001", "kind": "action", "scope": "NAV", "reach": "global"}]}`
	require.NoError(t, os.WriteFile(filepath.Join(contextDir, "context.json"), []byte(sourceIndex), 0o644))

	server := newTestServer(t, &Options{
		AgentwtPath:  agentwt,
		ContextDir:   contextDir,
		PeekEndpoint: "http://127.0.0.1:1/mcp",
		ReposPath:    reposPath,
	})
	return server, repo, reposPath
}

// runGitIn runs git in dir as the fixture's committer and returns its trimmed output.
func runGitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	gitArgs := append([]string{"-c", "user.email=t@t", "-c", "user.name=t"}, args...)
	command := exec.Command("git", gitArgs...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
	return strings.TrimSpace(string(output))
}

// stashRels lists the stamp-relative paths of a stash's entries.
func stashRels(stash *trash.Stash) []string {
	rels := make([]string, 0, len(stash.Entries))
	for _, entry := range stash.Entries {
		rels = append(rels, entry.Rel)
	}
	return rels
}

// writeDeployRecord saves the demo repo's build record into the server's state dir.
func writeDeployRecord(t *testing.T, server *Server, content string) {
	t.Helper()
	record := contextdocs.DeployRecordPath(server.stateDir, "demo")
	require.NoError(t, os.MkdirAll(filepath.Dir(record), 0o755))
	require.NoError(t, os.WriteFile(record, []byte(content), 0o644))
}

func TestReposIndexRendersRegistryRows(t *testing.T) {
	server, repo, _ := reposTestEnv(t)
	// Context: a fully synced target — the build record carries the deploy
	// section with writeRepo, the repo copy the one entry the source index expects.
	writeDeployRecord(t, server, `{"deploy": {"contextDir": "docs", "langs": ["go"], "acdsl": true, "writeRepo": true}, "entries": [{"id": "ACTION-NAV-001"}]}`)
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "docs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "docs", "context.json"),
		[]byte(`{"deploy": {"contextDir": "docs", "langs": ["go"], "acdsl": true}, "entries": [{"id": "ACTION-NAV-001"}]}`), 0o644))
	// One real claude/* worktree so the Worktrees column counts 1.
	gitInitWithWorktrees(t, repo, "claude/session-x")

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "<h1>Repos</h1>")
	assert.Contains(t, body, "<h2>repos</h2>")
	assert.Contains(t, body, ">demo</a>")
	assert.Contains(t, body, `synced:docs`)
	assert.Contains(t, body, "all 1 entries reaching this repo are in the build")
	assert.Contains(t, body, ">on</span>")
	assert.NotContains(t, body, "<th>Languages</th>")
	assert.NotContains(t, body, "<th>Claude</th>")
	assert.Contains(t, body, "<div>claude:1</div>")
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
	registry := `{"repos": [{"name": "copy_trader_flask_server_llm", "path": ` + jsonString(t, repo) + `}]}`
	require.NoError(t, os.WriteFile(reposPath, []byte(registry), 0o644))

	server := newTestServer(t, &Options{ReposPath: reposPath})

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "copy_<wbr>trader_<wbr>flask_<wbr>server_<wbr>llm")
	assert.Contains(t, body, "<div>claude:1</div><div>claude-routines:1</div>")
}

// elementBefore is the markup of body from the last prefix before the first
// marker through the marker — one rendered element located by its tail.
func elementBefore(body, prefix, marker string) string {
	end := strings.Index(body, marker)
	if end == -1 {
		return ""
	}

	start := strings.LastIndex(body[:end], prefix)
	if start == -1 {
		return ""
	}
	return body[start : end+len(marker)]
}

// getStatusFragment serves one fragment request and returns its body.
func getStatusFragment(t *testing.T, server *Server, target string) string {
	t.Helper()
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
	require.Equal(t, http.StatusOK, response.Code)
	return response.Body.String()
}

// The fragment scans the requested kind scope: no kind is claude, all is the
// unfiltered scan; kind chips load their kind from the server and every
// all-scope request carries the confirm (D5, D7).
func TestWorktreeStatusFragmentKindScope(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	writeStatusDocument(t, server,
		wireRow(nil),
		wireRow(map[string]any{"branch": "claude-routines/nightly", "kind": "claude-routines",
			"worktree": map[string]any{"path": "/cache/worktrees/nightly", "dirty": 0, "untracked": 0}}),
	)
	confirm := `hx-confirm="Load all worktrees of demo? This scans every agent branch and worktree at once and can overload this machine."`

	// no kind → the claude scope, its chip active
	body := getStatusFragment(t, server, "/repos/demo/status")
	assert.Equal(t, []string{"--kind=claude"}, statusArgv(server))
	claudeChip := elementBefore(body, "<a ", `data-kind="claude">claude</a>`)
	routinesChip := elementBefore(body, "<a ", `data-kind="claude-routines">claude-routines</a>`)
	allChip := elementBefore(body, "<a ", `data-kind="all">all</a>`)
	assert.Contains(t, claudeChip, "badge-ok wt-kind")
	assert.Contains(t, routinesChip, "badge-dim wt-kind")
	assert.Contains(t, allChip, "badge-dim wt-kind")
	assert.Contains(t, claudeChip, `hx-get="/repos/demo/status?kind=claude"`)
	assert.Contains(t, routinesChip, `hx-get="/repos/demo/status?kind=claude-routines"`)
	assert.Contains(t, allChip, `hx-get="/repos/demo/status?kind=all"`)
	assert.Contains(t, allChip, confirm)
	assert.NotContains(t, claudeChip, "hx-confirm")
	assert.NotContains(t, routinesChip, "hx-confirm")
	root := elementBefore(body, `<div id="worktree-status"`, `hx-disinherit="*">`)
	reload := elementBefore(body, "<button", ">Reload</button>")
	assert.Contains(t, root, `hx-get="/repos/demo/status?kind=claude&refresh=1"`)
	assert.Contains(t, reload, `hx-get="/repos/demo/status?kind=claude&refresh=1"`)
	assert.NotContains(t, root, "hx-confirm")
	assert.NotContains(t, reload, "hx-confirm")

	// another kind is passed to the binary and its chip is the active one
	body = getStatusFragment(t, server, "/repos/demo/status?kind=claude-routines")
	assert.Equal(t, []string{"--kind=claude", "--kind=claude-routines"}, statusArgv(server))
	assert.Contains(t, elementBefore(body, "<a ", `data-kind="claude-routines">claude-routines</a>`), "badge-ok wt-kind")
	assert.Contains(t, elementBefore(body, "<a ", `data-kind="claude">claude</a>`), "badge-dim wt-kind")

	// all → the unfiltered scan; Reload and the repo-op refresh confirm first
	body = getStatusFragment(t, server, "/repos/demo/status?kind=all")
	assert.Equal(t, []string{"--kind=claude", "--kind=claude-routines", ""}, statusArgv(server))
	assert.Contains(t, elementBefore(body, "<a ", `data-kind="all">all</a>`), "badge-ok wt-kind")
	root = elementBefore(body, `<div id="worktree-status"`, `hx-disinherit="*">`)
	reload = elementBefore(body, "<button", ">Reload</button>")
	assert.Contains(t, root, `hx-get="/repos/demo/status?kind=all&refresh=1"`)
	assert.Contains(t, reload, `hx-get="/repos/demo/status?kind=all&refresh=1"`)
	assert.Contains(t, root, confirm)
	assert.Contains(t, reload, confirm)
}

// A new claude-<kind>/* namespace produces its own chip without any code
// change — chips derive from the document's kinds, "all" first; the loaded
// default kind is added as the active chip when the repo holds none of it,
// and kinds absent from the document produce no chip.
func TestWorktreeStatusFragmentKindChipsDerived(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	writeStatusDocument(t, server,
		wireRow(map[string]any{"branch": "claude-review/station-r1", "kind": "claude-review"}),
	)

	body := getStatusFragment(t, server, "/repos/demo/status")
	assert.Contains(t, body, `<tr data-kind="claude-review" data-base="main">`)
	reviewChip := elementBefore(body, "<a ", `data-kind="claude-review">claude-review</a>`)
	claudeChip := elementBefore(body, "<a ", `data-kind="claude">claude</a>`)
	assert.Contains(t, reviewChip, "badge-dim wt-kind")
	assert.Contains(t, claudeChip, "badge-ok wt-kind")
	assert.NotContains(t, body, `data-kind="claude-routines">`)
	allIdx := strings.Index(body, `data-kind="all">all</a>`)
	claudeIdx := strings.Index(body, `data-kind="claude">claude</a>`)
	reviewIdx := strings.Index(body, `data-kind="claude-review">claude-review</a>`)
	require.NotEqual(t, -1, allIdx)
	assert.Less(t, allIdx, claudeIdx)
	assert.Less(t, claudeIdx, reviewIdx)
}

func TestReposIndexPartialCoverage(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	// Built, not written: the expected entry is absent from the build record.
	writeDeployRecord(t, server, `{"deploy": {"contextDir": "docs", "langs": []}, "entries": []}`)

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
	assert.Contains(t, body, `<span class="badge badge-error" title="no context/aspects.json and no context build for this repo">✗</span>`)
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
	// fetch only (D3), opening on the claude scope.
	assert.Contains(t, body, "Loading worktree status…")
	assert.Contains(t, body, `hx-get="/repos/demo/status?kind=claude"`)
	assert.Contains(t, body, `hx-trigger="load"`)
	assert.NotContains(t, body, ">Branch</th>")
	assert.NotContains(t, body, `name="build"`)
	assert.NotContains(t, body, "/commands")
	assert.Equal(t, 0, statusRuns(server))

	// a named kind is passed through to the fragment request
	routines := httptest.NewRecorder()
	server.Handler().ServeHTTP(routines, httptest.NewRequest(http.MethodGet, "/repos/demo?kind=claude-routines", nil))
	require.Equal(t, http.StatusOK, routines.Code)
	assert.Contains(t, routines.Body.String(), `hx-get="/repos/demo/status?kind=claude-routines"`)

	// the shell never opens on the all scope — a full scan needs a confirmed click (D6)
	all := httptest.NewRecorder()
	server.Handler().ServeHTTP(all, httptest.NewRequest(http.MethodGet, "/repos/demo?kind=all", nil))
	require.Equal(t, http.StatusOK, all.Code)
	assert.Contains(t, all.Body.String(), `hx-get="/repos/demo/status?kind=claude"`)
	assert.NotContains(t, all.Body.String(), `status?kind=all`)
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
	assert.Contains(t, body, `title="Client.Connect:`)
	// The swapped-in fragment refreshes on repo-op only — a `load` trigger
	// would re-fire on every swap and loop (D3).
	assert.Contains(t, body, `hx-trigger="repo-op from:body"`)
	assert.NotContains(t, body, `hx-trigger="load"`)
	// Timing line present (DR3).
	assert.Contains(t, body, "status ")
	assert.Contains(t, body, "ms</div>")
}

// The session cell reads the registry session the binary attached: a live
// Claude session shows the live badge, status and name; a dead one the dead
// badge; a Codex rollout (no pid) neither; a detached row has no checkbox.
func TestWorktreeStatusFragmentSessionAndDetachedCells(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	writeStatusDocument(t, server,
		wireRow(map[string]any{"session": map[string]any{
			"agent": "claude", "id": "af30b387-live", "pid": 185, "alive": true, "status": "busy",
			"entrypoint": "claude-desktop", "name": "CC, FD - Worktree binary", "updated_at": "2026-09-25T00:05:27Z",
		}}),
		wireRow(map[string]any{"branch": "claude/beta", "session": map[string]any{
			"agent": "claude", "id": "dead-id", "pid": 999, "alive": false, "status": "idle",
			"entrypoint": "cli", "name": "CC,FD - Gate states - [READY]", "updated_at": "2026-09-25T00:05:27Z",
		}}),
		wireRow(map[string]any{"branch": nil, "kind": "codex",
			"worktree": map[string]any{"path": "/Users/x/.codex/worktrees/ffc8/repo", "detached": true, "dirty": 0, "untracked": 0},
			"from":     map[string]any{"name": nil, "resolution": "unknown", "origin": nil},
			"counts":   map[string]any{"vs_default": map[string]int{"ahead": 0, "behind": 240}},
			"session": map[string]any{
				"agent": "codex", "id": "01a0b555-codex", "entrypoint": "Codex Desktop", "updated_at": "2026-09-18T16:24:40.134Z",
			}}),
	)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo/status", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()

	assert.Equal(t, 1, strings.Count(body, `<span class="badge badge-ok">live</span>`))
	assert.Contains(t, body, `<span class="meta">busy</span>`)
	assert.Contains(t, body, `<span title="af30b387-live">CC, FD - Worktree binary</span>`)
	assert.Contains(t, body, `>dead</span>`)
	assert.Contains(t, body, `<span title="dead-id">CC,FD - Gate states</span> <span class="badge badge-action">READY</span>`, "session state badge")
	// Codex: no pid → neither badge; label falls back to agent + entrypoint.
	assert.Contains(t, body, `<span title="01a0b555-codex">codex Codex Desktop</span>`)
	// Detached row: no checkbox, tip shown with the pool badge, no commit links.
	assert.Equal(t, 2, strings.Count(body, `<input type="checkbox" name="branch"`))
	assert.Contains(t, body, `(detached 01234567) <span class="badge badge-dim"`)
	assert.Contains(t, body, `<tr data-kind="codex" data-base="unknown">`)
	assert.NotContains(t, body, `/branches//commits/`)
	// Kind chips include the detached pool.
	assert.Contains(t, body, `hx-get="/repos/demo/status?kind=codex"`)
}

func TestRootCause(t *testing.T) {
	inner := errors.New("connection refused")
	wrapped := fmt.Errorf("Initialize failed: %w", fmt.Errorf("transport error: %w", inner))
	assert.Equal(t, inner, rootCause(wrapped))
	assert.Equal(t, inner, rootCause(inner))
}

func TestWorktreeStatusFragmentFlagsDirBranchMismatch(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	writeStatusDocument(t, server,
		wireRow(map[string]any{"order": map[string]any{
			"conflicts_with": []string{}, "train_position": 1, "train": map[string]any{"state": "clean"},
		}}),
		wireRow(map[string]any{
			"branch":      "claude/beta",
			"worktree":    map[string]any{"path": "/repo/.claude/worktrees/recycled-dir", "dirty": 0, "untracked": 0},
			"from":        map[string]any{"name": "main", "resolution": "sha", "origin": "0abee6992be34bc3a2cbac4eb9b4e9ff3fc81e5e"},
			"counts":      map[string]any{"vs_from": map[string]int{"ahead": 1, "behind": 2}},
			"containment": map[string]string{"branch": "release/a", "reason": "all_commits_placed", "rank": "other"},
			"order": map[string]any{
				"conflicts_with": []string{"claude/alpha"}, "train_position": 2,
				"train": map[string]any{"state": "blocked", "blocked_by": []string{"claude/alpha"}, "paths": []string{"shared.txt"}},
			},
		}),
	)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo/status", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()

	assert.Contains(t, body, ">Worktree</th>")
	assert.Contains(t, body, ">Base</th>")
	assert.Contains(t, body, ">Contained in</th>")
	assert.Contains(t, body, ">Merge Order</th>")
	assert.NotContains(t, body, ">Order</th>")
	assert.NotContains(t, body, ">Counts</th>")
	// Column order: Base, Unpicked, Dirty / Untracked, Ahead / Behind, Contained in, Session, Merge Order, Last commit.
	headers := []string{">Base</th>", ">Unpicked</th>", ">Dirty / Untracked</th>", ">Ahead / Behind</th>",
		">Contained in</th>", ">Session</th>", ">Merge Order</th>", ">Last commit</th>"}
	for i := 1; i < len(headers); i++ {
		assert.Less(t, strings.Index(body, headers[i-1]), strings.Index(body, headers[i]), headers[i])
	}
	// beta: ahead 1 / behind 2 as linked GoLand arrows; alpha: dimmed zeros.
	assert.Contains(t, body, `<span class="wt-ahead" title="ahead of main">↗ <a`)
	assert.Contains(t, body, `<span class="wt-behind" title="behind main">↙ <a`)
	assert.Contains(t, body, `<span class="wt-ahead wt-zero" title="ahead of main">↗ 0</span>`)
	// No unpicked commits → no warning badge.
	assert.NotContains(t, body, "st-badge-warn")
	// One containment cell; the Verdicts and Merged into columns are gone.
	assert.NotContains(t, body, ">Verdicts</th>")
	assert.NotContains(t, body, ">Merged into</th>")
	// The safe badge renders inside the first cell, next to the checkbox;
	// the standalone Safe column is gone.
	assert.NotContains(t, body, ">Safe</th>")
	assert.Contains(t, body, `<input type="checkbox" class="select-all"`)
	assert.Contains(t, body, `<span title="/repo/.claude/worktrees/alpha">alpha</span>`)
	// Only the pool-recycled row (dir name != branch slug) carries the marker.
	assert.Equal(t, 1, strings.Count(body, "dir≠branch"))
	// Containment cell: branch with its rank in the tooltip, reason as a badge; base badge for the sha origin.
	assert.Contains(t, body, `<span title="base branch">main</span> <span class="badge badge-dim" title="the tip is in this branch's history">ancestor</span>`)
	assert.Contains(t, body, `<span title="other branch">release/a</span> <span class="badge badge-dim" title="every own commit is on this branch: re-applies as a no-op, was picked (same author, author time and subject, conflict resolution included) or has a patch-id twin">placed</span>`)
	assert.Equal(t, 1, strings.Count(body, `>sha</span>`))
	assert.NotContains(t, body, `>chain</span>`)
	// Order: alpha clean at #1, beta blocked at #2 with the conflicting path in the tooltip.
	assert.Contains(t, body, `>#1</span>`)
	assert.Contains(t, body, `title="conflicts after claude/alpha: shared.txt">#2 ✗</span>`)
	// Ops widget: remove only — no sync/merge forms — and per-row checkboxes carry the data the ops JS reads.
	assert.Contains(t, body, `id="worktree-ops" data-repo="demo"`)
	assert.Contains(t, body, ">Remove</button>")
	assert.Contains(t, body, ">Remove Worktree</button>")
	assert.NotContains(t, body, `data-op="sync"`)
	assert.NotContains(t, body, `data-op="merge"`)
	assert.NotContains(t, body, ">Sync</button>")
	assert.NotContains(t, body, ">Merge</button>")
	assert.Contains(t, body, `data-branch="claude/beta" data-worktree="/repo/.claude/worktrees/recycled-dir"`)
	// Both rows are safe → ✓ badge, twice.
	assert.Equal(t, 2, strings.Count(body, `data-safe="1"`))
}

func TestWorktreeStatusFragmentRendersMergedAndMovedChips(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	// A squash-merged row (merge adds nothing) whose base a rebase moved to develop.
	writeStatusDocument(t, server,
		wireRow(map[string]any{"branch": "claude/squashed",
			"from":        map[string]any{"name": "develop", "resolution": "reanchored", "origin": "rebase (finish): refs/heads/claude/squashed onto 0abee6992be34bc3a2cbac4eb9b4e9ff3fc81e5e"},
			"containment": map[string]string{"branch": "develop", "reason": "merge_adds_nothing", "rank": "base"}}),
	)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo/status", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()

	assert.Contains(t, body, `<span title="base branch">develop</span> <span class="badge badge-dim" title="merging this branch into it changes nothing — squash-merged, or its commits cancel out">merged</span>`)
	assert.Contains(t, body, `<span class="badge badge-dim" title="a rebase, reset or branch -f moved this branch onto this base after creation">moved</span>`)
	assert.NotContains(t, body, `>placed</span>`)
}

func TestWorktreeStatusFragmentSafeCellRendersReasons(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	// Row 1: no worktree, work placed on main — safe with the no-worktree
	// tooltip. Row 2: unsafe, and the cell names the blocking conditions
	// instead of a generic ✗; the chain base carries its badge.
	writeStatusDocument(t, server,
		wireRow(map[string]any{"branch": "claude/placed", "worktree": nil,
			"containment": map[string]string{"branch": "main", "reason": "all_commits_placed", "rank": "default"}}),
		wireRow(map[string]any{"branch": "claude-routines/blocked", "kind": "claude-routines",
			"worktree":       map[string]any{"path": "/repo/.claude/worktrees/blocked", "dirty": 0, "untracked": 2},
			"from":           map[string]any{"name": "main", "resolution": "chain", "origin": "claude-routines/previous"},
			"unpicked":       1,
			"containment":    nil,
			"safe_to_remove": false,
			"unsafe_reasons": []string{"untracked(2)", "unpicked(1)"}}),
	)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo/status", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()

	assert.Contains(t, body, `title="no worktree checked out — nothing dirty to lose; every commit is contained on a non-agent branch">✓</span>`)
	assert.Contains(t, body, `title="unsafe to remove: untracked(2), unpicked(1)"`)
	assert.Equal(t, 1, strings.Count(body, `data-safe="1"`))
	assert.Contains(t, body, `>chain</span>`)
	assert.Contains(t, body, `commits/unpicked`)
	// The blocked row's unpicked count carries the ⚠ badge and the amber highlight, once.
	assert.Equal(t, 1, strings.Count(body, `class="st-badge st-badge-warn"`))
	assert.Contains(t, body, `<span class="wt-unpicked-hot"><a`)
	// The placed row has no worktree → one dash in Dirty / Untracked.
	assert.Contains(t, body, "<td>–</td>")
	// The uncontained row renders the dash, the placed row its default-rank cell.
	assert.Contains(t, body, `<span title="default branch">main</span>`)
	assert.Equal(t, 1, strings.Count(body, `>placed</span>`))
}

func TestWorktreeStatusFragmentBaseFilter(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	writeStatusDocument(t, server,
		wireRow(nil),
		wireRow(map[string]any{"branch": "claude/beta", "from": map[string]any{"name": "dev", "resolution": "ref"},
			"containment": map[string]string{"branch": "dev", "reason": "ancestor", "rank": "base"}}),
		wireRow(map[string]any{"branch": "claude/gamma", "from": map[string]any{"name": nil, "resolution": "unknown"}, "counts": map[string]any{}}),
	)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo/status?base=dev", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()

	// base chips come from the full list; the "unknown" sentinel never
	// becomes a chip; the JS owns the active state
	assert.Contains(t, body, ">all</a>")
	assert.Contains(t, body, `data-group="base" data-value="main"`)
	assert.Contains(t, body, `data-group="base" data-value="dev"`)
	assert.NotContains(t, body, `data-value="unknown"`)
	// every row is rendered, tagged with its base for the client-side filter
	assert.Contains(t, body, `data-base="main">`)
	assert.Contains(t, body, `data-base="dev">`)
	assert.Contains(t, body, `data-base="unknown">`)
	assert.Contains(t, body, "claude/alpha")
	assert.Contains(t, body, "claude/beta")
	assert.Contains(t, body, "claude/gamma")
}

func TestWorktreeStatusFragmentServesCache(t *testing.T) {
	server, repo, _ := reposTestEnv(t)
	// The stub counts its invocations so the cache behavior is observable.
	writeStatusDocument(t, server, wireRow(nil))
	scriptRuns := func() int { return statusRuns(server) }

	// first load scans and renders the reload button + timestamp
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo/status", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, 1, scriptRuns())
	body := response.Body.String()
	assert.Contains(t, body, ">Reload</button>")
	assert.Contains(t, body, "as of ")

	// second plain load serves the cache — no script run, cached marker shown
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo/status", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, 1, scriptRuns())
	assert.Contains(t, response.Body.String(), "claude/alpha")
	assert.Contains(t, response.Body.String(), " · cached")

	// ?refresh=1 always re-scans
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo/status?refresh=1", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, 2, scriptRuns())
	assert.NotContains(t, response.Body.String(), " · cached")

	// a structural change (new agent branch) flips the fingerprint — the next
	// plain load re-scans instead of serving the stale entry
	branchCmd := exec.Command("git", "branch", "claude/fresh")
	branchCmd.Dir = repo
	output, err := branchCmd.CombinedOutput()
	require.NoError(t, err, string(output))
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo/status", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, 3, scriptRuns())
	assert.NotContains(t, response.Body.String(), " · cached")

	// another kind is its own cache scope — its first load scans
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo/status?kind=claude-routines", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, 4, scriptRuns())
	assert.NotContains(t, response.Body.String(), " · cached")

	// the claude scope is still cached afterwards
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo/status", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, 4, scriptRuns())
	assert.Contains(t, response.Body.String(), " · cached")
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

	t.Run("skipped-outcome-triggers-refresh-despite-exit-0", func(t *testing.T) {
		server, _, _ := reposTestEnv(t)
		writeRemoveResult(t, server, `{"operation":"remove","lines":[{"subject":"claude/feature","outcome":"skipped","text":"skipped: /some/worktree (claude/feature): dirty(1)"}]}`, 0)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/demo/remove-selected",
			url.Values{"branch": {"claude/feature"}}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, "repo-op", response.Header().Get("HX-Trigger"))
		assert.Contains(t, response.Body.String(), "skipped: /some/worktree (claude/feature): dirty(1)")
	})

	t.Run("binary-error-triggers-refresh", func(t *testing.T) {
		server, _, _ := reposTestEnv(t)
		writeRemoveResult(t, server, "boom", 1)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/demo/remove-selected",
			url.Values{"branch": {"claude/feature"}}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, "repo-op", response.Header().Get("HX-Trigger"))
		assert.Contains(t, response.Body.String(), ">Failed</span>")
		assert.Contains(t, response.Body.String(), "boom")
	})

	t.Run("force-flag-passed-through", func(t *testing.T) {
		server, _, _ := reposTestEnv(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/demo/remove-selected",
			url.Values{"branch": {"claude/feature"}, "force": {"on"}}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "remove --force claude/feature")
	})

	t.Run("force-salvages-first", func(t *testing.T) {
		server, repo, _ := reposTestEnv(t)
		worktree := filepath.Join(t.TempDir(), "wt")
		runGitIn(t, repo, "worktree", "add", "-q", worktree, "claude/feature")
		require.NoError(t, os.WriteFile(filepath.Join(worktree, "scratch.txt"), []byte("scratch\n"), 0o644))
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/demo/remove-selected",
			url.Values{"branch": {"claude/feature"}, "force": {"on"}}))
		require.Equal(t, http.StatusOK, response.Code)

		stashes, err := server.trash.List()
		require.NoError(t, err)
		require.Len(t, stashes, 1)
		assert.Equal(t, []string{"worktree/scratch.txt"}, stashRels(stashes[0]))
		assert.Contains(t, response.Body.String(), "archived claude/feature -&gt; ")
		assert.Contains(t, response.Body.String(), "remove --force claude/feature")
	})

	t.Run("delete-branch-archives", func(t *testing.T) {
		server, repo, _ := reposTestEnv(t)
		worktree := filepath.Join(t.TempDir(), "wt")
		runGitIn(t, repo, "worktree", "add", "-q", worktree, "claude/feature")
		require.NoError(t, os.WriteFile(filepath.Join(worktree, "change.txt"), []byte("change\n"), 0o644))
		runGitIn(t, worktree, "add", "change.txt")
		runGitIn(t, worktree, "commit", "-qm", "change")
		runGitIn(t, repo, "worktree", "remove", worktree)
		sha := runGitIn(t, repo, "rev-parse", "claude/feature")
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/demo/remove-selected",
			url.Values{"branch": {"claude/feature"}, "delete-branch": {"on"}}))
		require.Equal(t, http.StatusOK, response.Code)

		stashes, err := server.trash.List()
		require.NoError(t, err)
		require.Len(t, stashes, 1)
		assert.Equal(t, []string{"branches/claude_feature.diff"}, stashRels(stashes[0]))
		journal, err := server.trash.Tail(1)
		require.NoError(t, err)
		require.Len(t, journal, 1)
		assert.Equal(t, "repos remove", journal[0].Action)
		assert.Equal(t, trash.KindBranchDeleted, journal[0].Kind)
		assert.Equal(t, repo+"#claude/feature", journal[0].Path)
		assert.Equal(t, "sha "+sha, journal[0].Reason)
		assert.Equal(t, stashes[0].Stamp+"/branches", journal[0].Stash)
	})

	t.Run("plain-remove-archives-nothing", func(t *testing.T) {
		server, _, _ := reposTestEnv(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/demo/remove-selected",
			url.Values{"branch": {"claude/feature"}}))
		require.Equal(t, http.StatusOK, response.Code)

		stashes, err := server.trash.List()
		require.NoError(t, err)
		assert.Empty(t, stashes)
		journal, err := server.trash.Tail(1)
		require.NoError(t, err)
		assert.Empty(t, journal)
		assert.NotContains(t, response.Body.String(), "archived")
	})

	t.Run("no-selection-400", func(t *testing.T) {
		server, _, _ := reposTestEnv(t)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/demo/remove-selected", url.Values{}))
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("clean-removal-drops-cached-rows", func(t *testing.T) {
		server, _, _ := reposTestEnv(t)
		key := statusKey{Kind: "claude", Repo: "demo"}
		scan := &repos.StatusScan{Rows: []repos.WorktreeStatus{
			{Branch: "claude/feature"},
			{Branch: "claude/keep"},
		}}
		server.statusCache.Store(key, "", scan)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/demo/remove-selected",
			url.Values{"branch": {"claude/feature"}}))
		require.Equal(t, http.StatusOK, response.Code)

		entry, ok := server.statusCache.Get(key)
		require.True(t, ok)
		kept := []repos.WorktreeStatus{{Branch: "claude/keep"}}
		assert.Equal(t, kept, entry.Statuses)
	})

	t.Run("skipped-outcome-leaves-cache-untouched", func(t *testing.T) {
		server, _, _ := reposTestEnv(t)
		key := statusKey{Kind: "claude", Repo: "demo"}
		statuses := []repos.WorktreeStatus{{Branch: "claude/feature"}}
		scan := &repos.StatusScan{Rows: statuses}
		server.statusCache.Store(key, "", scan)
		writeRemoveResult(t, server, `{"operation":"remove","lines":[{"subject":"claude/feature","outcome":"skipped","text":"skipped branch: claude/feature: dirty(1)"}]}`, 0)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/demo/remove-selected",
			url.Values{"branch": {"claude/feature"}}))
		require.Equal(t, http.StatusOK, response.Code)

		entry, ok := server.statusCache.Get(key)
		require.True(t, ok)
		assert.Equal(t, statuses, entry.Statuses)
	})
}

func TestRepoOpConcurrent409(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	require.True(t, server.repoLocks.TryAcquire("demo"))
	defer server.repoLocks.Release("demo")

	// The batch-remove handler inlines the lock (D5).
	remove := httptest.NewRecorder()
	server.Handler().ServeHTTP(remove, formPost("/repos/demo/remove-selected", url.Values{"branch": {"claude/feature"}}))
	assert.Equal(t, http.StatusConflict, remove.Code)
}

// The merge, sync and cherry-pick routes are gone (read-only overview, D9).
func TestRepoGitClientRoutesGone(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	for _, path := range []string{
		"/repos/demo/branches/claude%2Ffeature/sync",
		"/repos/demo/branches/claude%2Ffeature/merge",
		"/repos/demo/cherry-pick",
	} {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost(path, url.Values{"commit": {"abc1234"}}))
		assert.Equalf(t, http.StatusNotFound, response.Code, "path=%s", path)
	}
}

func TestRepoCommitsRejectsNonAgentBranch(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo/branches/main/commits/ahead", nil))
	assert.Equal(t, http.StatusBadRequest, response.Code)
}

// The commit drill-down reads the cached document: one status run serves the
// fragment and every class; a branch missing from the document is 404; the
// unpicked list carries no cherry-pick form.
func TestRepoCommitsFromCachedDocument(t *testing.T) {
	server, _, _ := reposTestEnv(t)
	writeStatusDocument(t, server,
		wireRow(map[string]any{"branch": "claude/feature",
			"counts":           map[string]any{"vs_from": map[string]int{"ahead": 1, "behind": 0}},
			"unpicked":         1,
			"unpicked_commits": []string{"abc1234abc1234abc1234abc1234abc1234abc12"},
			"containment":      nil,
			"commits": map[string]any{
				"ahead": []map[string]any{{"sha": "abc1234abc1234abc1234abc1234abc1234abc12", "short_sha": "abc1234a",
					"subject": "the change", "committed_at": "2026-07-15T10:00:00Z", "is_merge": false}},
				"behind": []any{}, "behind_truncated": false,
			}}),
	)

	fragment := httptest.NewRecorder()
	server.Handler().ServeHTTP(fragment, httptest.NewRequest(http.MethodGet, "/repos/demo/status", nil))
	require.Equal(t, http.StatusOK, fragment.Code)

	for _, class := range []string{"ahead", "unpicked"} {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos/demo/branches/claude%2Ffeature/commits/"+class, nil))
		require.Equalf(t, http.StatusOK, response.Code, "class=%s", class)
		assert.Contains(t, response.Body.String(), "abc1234a  2026-07-15 10:00  the change")
		assert.NotContains(t, response.Body.String(), "cherry-pick")
	}
	assert.Equal(t, 1, statusRuns(server))

	// A branch that exists in git but not in the document (empty class) still
	// renders — with no commits; an unknown branch is refused before any lookup.
	behind := httptest.NewRecorder()
	server.Handler().ServeHTTP(behind, httptest.NewRequest(http.MethodGet, "/repos/demo/branches/claude%2Ffeature/commits/behind", nil))
	require.Equal(t, http.StatusOK, behind.Code)
	assert.Contains(t, behind.Body.String(), "No commits.")
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

	t.Run("custom-name-overrides-basename", func(t *testing.T) {
		server, _, reposPath := reposTestEnv(t)
		dir := filepath.Join(t.TempDir(), "clash")
		require.NoError(t, os.Mkdir(dir, 0o755))

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/add", url.Values{"path": {dir}, "name": {"clash-2"}}))
		require.Equal(t, http.StatusOK, response.Code)

		file := readRegistryFile(t, reposPath)
		require.Len(t, file["repos"], 2)
		assert.Equal(t, "clash-2", file["repos"][1]["name"])
	})

	t.Run("reserved-name-is-bad-request-before-git-init", func(t *testing.T) {
		server, _, reposPath := reposTestEnv(t)
		dir := filepath.Join(t.TempDir(), "somerepo")
		require.NoError(t, os.Mkdir(dir, 0o755))

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/add", url.Values{"path": {dir}, "name": {"unknown"}}))
		require.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "is reserved")
		assert.Len(t, readRegistryFile(t, reposPath)["repos"], 1)
		// validation ran before the op — the folder was not git-inited
		assert.NoDirExists(t, filepath.Join(dir, ".git"))
	})

	t.Run("duplicate-name-renders-failure", func(t *testing.T) {
		server, _, reposPath := reposTestEnv(t)
		dir := filepath.Join(t.TempDir(), "demo")
		require.NoError(t, os.Mkdir(dir, 0o755))

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/repos/add", url.Values{"path": {dir}}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), ">Failed</span>")
		assert.Contains(t, response.Body.String(), "Duplicate repo name")
		assert.Len(t, readRegistryFile(t, reposPath)["repos"], 1)
	})
}

func TestReposAddRejectsBeforeGitInit(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedBody   string
		_expectedStatus int

		path      string
		reposPath string
		server    *Server
	}

	tests := make([]*testCase, 0)

	// fail-missing-path
	server, _, reposPath := reposTestEnv(t)
	tests = append(tests, &testCase{
		_id:             "fail-missing-path",
		_expectedBody:   "path is not an existing folder",
		_expectedStatus: http.StatusBadRequest,

		path:      filepath.Join(t.TempDir(), "typo"),
		reposPath: reposPath,
		server:    server,
	})

	// fail-file-path
	server, _, reposPath = reposTestEnv(t)
	filePath := filepath.Join(t.TempDir(), "notes.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("x"), 0o644))
	tests = append(tests, &testCase{
		_id:             "fail-file-path",
		_expectedBody:   "path is not an existing folder",
		_expectedStatus: http.StatusBadRequest,

		path:      filePath,
		reposPath: reposPath,
		server:    server,
	})

	// fail-duplicate-name-plain-folder
	server, _, reposPath = reposTestEnv(t)
	duplicateDir := filepath.Join(t.TempDir(), "demo")
	require.NoError(t, os.Mkdir(duplicateDir, 0o755))
	tests = append(tests, &testCase{
		_id:             "fail-duplicate-name-plain-folder",
		_expectedBody:   "Duplicate repo name",
		_expectedStatus: http.StatusOK,

		path:      duplicateDir,
		reposPath: reposPath,
		server:    server,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			response := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(response, formPost("/repos/add", url.Values{"path": {test.path}}))

			assert.Equal(t, test._expectedStatus, response.Code)
			assert.Contains(t, response.Body.String(), test._expectedBody)
			assert.Len(t, readRegistryFile(t, test.reposPath)["repos"], 1)
			assert.NoDirExists(t, filepath.Join(test.path, ".git"))
		})
	}
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
	if runtime.GOOS != "darwin" {
		t.Skip("osascript stub; ChooseFolder runs powershell off darwin")
	}
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

func reposProfileEnv(t *testing.T, presentationPath string) (*Server, string, string) {
	t.Helper()
	repoDir := t.TempDir()
	reposPath := filepath.Join(t.TempDir(), "repos.json")
	registry := `{"repos": [{"label": "Haushalt", "name": "demo", "path": ` + jsonString(t, repoDir) + `}]}`
	require.NoError(t, os.WriteFile(reposPath, []byte(registry), 0o644))

	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	opts := &Options{
		PresentationPath: presentationPath,
		ReposPath:        reposPath,
		SettingsPath:     settingsPath,
	}
	return newTestServer(t, opts), reposPath, settingsPath
}

func TestReposAddSkipsGitInitForExistingRepo(t *testing.T) {
	server, reposPath, _ := reposProfileEnv(t, "")
	dir := filepath.Join(t.TempDir(), "existing")
	// An empty .git: git init would write HEAD into it (plan D7).
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))
	form := url.Values{"path": {dir}}

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, formPost("/repos/add", form))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "added existing")
	assert.Len(t, readRegistryFile(t, reposPath)["repos"], 2)
	assert.NoFileExists(t, filepath.Join(dir, ".git", "HEAD"))
}

func TestReposCasualProjects(t *testing.T) {
	server, _, _ := reposProfileEnv(t, writeGermanProfile(t))

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/repos", nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	body := recorder.Body.String()
	assert.Contains(t, body, "<h1>Projekte</h1>")
	assert.Contains(t, body, "<h2>Deine Projekte</h2>")
	assert.Contains(t, body, "Haushalt")
	assert.Contains(t, body, `hx-post="/repos/add"`)
	assert.Contains(t, body, `hx-post="/repos/delete"`)
	assert.NotContains(t, body, `name="additional-dir"`)
	assert.NotContains(t, body, "<th>Path</th>")
	assert.NotContains(t, body, "<th>ACDSL</th>")
	assert.NotContains(t, body, `href="/repos/demo"`)
	assert.NotContains(t, body, `id="prune-form"`)
}

func TestReposIndexCarriesPruneForm(t *testing.T) {
	server, _, _ := reposTestEnv(t)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/repos", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "Prune JetBrains recent projects")
	assert.Contains(t, body, `hx-post="/repos/prune-jetbrains"`)
	assert.Less(t, strings.Index(body, `id="prune-form"`), strings.Index(body, `<div id="op-result">`))
}

func TestReposPruneJetbrains(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	server, _, _ := reposTestEnv(t)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, formPost("/repos/prune-jetbrains", url.Values{"dry-run": {"on"}}))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "no JetBrains config dir at")
	assert.Equal(t, "repo-op", response.Header().Get("HX-Trigger"))
}

func TestToolsPageGone(t *testing.T) {
	server, _, _ := reposTestEnv(t)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/tools", nil))
	assert.NotEqual(t, http.StatusOK, response.Code)
	assert.NotContains(t, response.Body.String(), `href="/tools"`)
}

func TestReposAddNeverGrants(t *testing.T) {
	type testCase struct {
		_id                 string
		_expectedAdded      string
		_expectedEncodedDir string

		dir          string
		reposPath    string
		server       *Server
		settingsPath string
	}

	tests := make([]*testCase, 0)

	// casual-profile
	casualServer, casualRepos, casualSettings := reposProfileEnv(t, writeGermanProfile(t))
	casualDir := filepath.Join(t.TempDir(), "projekt")
	require.NoError(t, os.Mkdir(casualDir, 0o755))
	casualEncoded := jsonString(t, casualDir)
	tests = append(tests, &testCase{
		_id:                 "casual-profile",
		_expectedAdded:      "added projekt (" + casualDir + ")",
		_expectedEncodedDir: strings.Trim(casualEncoded, `"`),

		dir:          casualDir,
		reposPath:    casualRepos,
		server:       casualServer,
		settingsPath: casualSettings,
	})

	// developer-profile
	developerServer, developerRepos, developerSettings := reposProfileEnv(t, "")
	developerDir := filepath.Join(t.TempDir(), "devrepo")
	require.NoError(t, os.Mkdir(developerDir, 0o755))
	developerEncoded := jsonString(t, developerDir)
	tests = append(tests, &testCase{
		_id:                 "developer-profile",
		_expectedAdded:      "added devrepo (" + developerDir + ")",
		_expectedEncodedDir: strings.Trim(developerEncoded, `"`),

		dir:          developerDir,
		reposPath:    developerRepos,
		server:       developerServer,
		settingsPath: developerSettings,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(recorder, formPost("/repos/add", url.Values{"additional-dir": {"on"}, "path": {test.dir}}))
			settings, _ := os.ReadFile(test.settingsPath)
			registry, _ := os.ReadFile(test.reposPath)
			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Contains(t, recorder.Body.String(), test._expectedAdded)
			assert.NotContains(t, string(settings), test._expectedEncodedDir)
			assert.Contains(t, string(registry), test._expectedEncodedDir)
		})
	}
}

func TestReposAddWithLabel(t *testing.T) {
	server, reposPath, _ := reposProfileEnv(t, "")
	dir := filepath.Join(t.TempDir(), "labeled")
	require.NoError(t, os.Mkdir(dir, 0o755))

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, formPost("/repos/add", url.Values{"label": {"Vereinsunterlagen"}, "path": {dir}}))
	require.Equal(t, http.StatusOK, recorder.Code)

	content, err := os.ReadFile(reposPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "Vereinsunterlagen")

	index := httptest.NewRecorder()
	server.Handler().ServeHTTP(index, httptest.NewRequest(http.MethodGet, "/repos", nil))
	assert.Contains(t, index.Body.String(), "Vereinsunterlagen")
}

func TestReposAddInitsGit(t *testing.T) {
	server, _, _ := reposProfileEnv(t, "")
	dir := filepath.Join(t.TempDir(), "plainfolder")
	require.NoError(t, os.Mkdir(dir, 0o755))

	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, formPost("/repos/add", url.Values{"path": {dir}}))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "added plainfolder")

	info, err := os.Stat(filepath.Join(dir, ".git"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

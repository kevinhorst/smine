package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kevinhorst/smine/internal/contextrecord"
	"github.com/kevinhorst/smine/internal/peek"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/sessions"
	"github.com/kevinhorst/smine/internal/sessionstate"
	"github.com/kevinhorst/smine/internal/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStubPeek(t *testing.T, sessionItems []map[string]any) *httptest.Server {
	t.Helper()
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Id     any            `json:"id"`
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if request.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
			return
		}

		var result map[string]any
		switch request.Method {
		case "initialize":
			result = map[string]any{
				"protocolVersion": request.Params["protocolVersion"],
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "stub-peek", "version": "1.0.5"},
			}
		case "tools/call":
			result = map[string]any{
				"content":           []any{},
				"isError":           false,
				"structuredContent": map[string]any{"sessions": sessionItems},
			}
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		response := map[string]any{"jsonrpc": "2.0", "id": request.Id, "result": result}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	t.Cleanup(stub.Close)
	return stub
}

// newAgentStubPeek answers session_list per arguments.agent from
// sessionsByAgent, session_get per arguments.id from turnsById,
// session_events per arguments.id from eventsById, and every other tool (or
// an unknown id) with a tool error.
func newAgentStubPeek(t *testing.T, eventsById map[string]map[string]any, sessionsByAgent map[string][]map[string]any, turnsById map[string][]map[string]any) *httptest.Server {
	t.Helper()
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Id     any            `json:"id"`
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if request.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
			return
		}

		var result map[string]any
		switch request.Method {
		case "initialize":
			result = map[string]any{
				"protocolVersion": request.Params["protocolVersion"],
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "stub-peek", "version": "1.2.11"},
			}
		case "tools/call":
			result = stubToolResult(request.Params, eventsById, sessionsByAgent, turnsById)
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		response := map[string]any{"jsonrpc": "2.0", "id": request.Id, "result": result}
		assert.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	t.Cleanup(stub.Close)
	return stub
}

// stubToolResult dispatches one tools/call of newAgentStubPeek on the tool
// name and its agent or id argument.
func stubToolResult(params map[string]any, eventsById map[string]map[string]any, sessionsByAgent map[string][]map[string]any, turnsById map[string][]map[string]any) map[string]any {
	arguments, _ := params["arguments"].(map[string]any)
	id, _ := arguments["id"].(string)
	events, hasEvents := eventsById[id]
	if params["name"] == "session_events" && hasEvents {
		return map[string]any{"content": []any{}, "isError": false, "structuredContent": events}
	}

	switch params["name"] {
	case "session_list":
		agent, _ := arguments["agent"].(string)
		return map[string]any{
			"content":           []any{},
			"isError":           false,
			"structuredContent": map[string]any{"sessions": sessionsByAgent[agent]},
		}
	case "session_get":
		return map[string]any{
			"content":           []any{},
			"isError":           false,
			"structuredContent": map[string]any{"turns": turnsById[id]},
		}
	}
	return map[string]any{"content": []any{}, "isError": true}
}

func TestSessionReportPeekAgents(t *testing.T) {
	lastActive := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	sessionsByAgent := map[string][]map[string]any{
		"claude": {
			{"id": "cl1", "last_active": lastActive, "meta": map[string]any{"cwd": "/nonexistent/repo"}, "title": "Claude session"},
		},
		"codex": {
			{"id": "cx1", "last_active": lastActive, "meta": map[string]any{"cwd": "/repo", "model": "gpt-6-astra"}, "title": "Codex session"},
		},
	}
	instructionsTurn := "<recommended_plugins>\n- github\n</recommended_plugins>\n" +
		"# AGENTS.md instructions for /repo\n\n<INSTRUCTIONS>\nRead AGENTS.md first.\n</INSTRUCTIONS>\n" +
		"<environment_context>\n  <cwd>/repo</cwd>\n</environment_context>"
	turnsById := map[string][]map[string]any{
		"cx1": {
			{"role": "user", "text": instructionsTurn, "meta": map[string]any{}},
			{"role": "assistant", "text": "Reading AGENTS.md.", "meta": map[string]any{"model": "gpt-6-astra"}},
		},
	}
	stub := newAgentStubPeek(t, nil, sessionsByAgent, turnsById)
	server := newTestServer(t, &Options{ClaudeHome: t.TempDir(), PeekEndpoint: stub.URL})

	t.Run("codex-report-shows-codex-tag-and-model", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions-report/cx1", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `st-tag-hook" title="hook: codex">codex`)
		assert.Contains(t, body, "gpt-6-astra")
	})

	t.Run("claude-report-without-transcript-shows-context-error", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions-report/cl1", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "Server.loadRecord")
	})
}

// copyTranscriptFixture copies the contextrecord transcript tree (root, one
// Explore subagent, one Workflow agent) into a Claude home as session id's
// transcript in a project dir of its own.
func copyTranscriptFixture(t *testing.T, claudeHome, id string) string {
	t.Helper()
	source := filepath.Join("..", "contextrecord", "testdata")
	target := filepath.Join(claudeHome, "projects", "-repo-x", id+".jsonl")
	stem := strings.TrimSuffix(target, ".jsonl")
	files := map[string]string{
		"root.jsonl":                                   target,
		"root/subagents/agent-a1.jsonl":                filepath.Join(stem, "subagents", "agent-a1.jsonl"),
		"root/subagents/agent-a1.meta.json":            filepath.Join(stem, "subagents", "agent-a1.meta.json"),
		"root/subagents/workflows/run1/agent-w1.jsonl": filepath.Join(stem, "subagents", "workflows", "run1", "agent-w1.jsonl"),
	}
	for from, to := range files {
		data, err := os.ReadFile(filepath.Join(source, from))
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Dir(to), 0o755))
		require.NoError(t, os.WriteFile(to, data, 0o644))
	}
	return target
}

// totalsLine is the markdown total row's injections and chars cells.
func totalsLine(totals *contextrecord.Totals) string {
	return fmt.Sprintf("| **Total** | | | %d | %s | ", totals.Items, groupThousands(int64(totals.Chars)))
}

func TestLoadSessionReport(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string
		_missing  []string

		path string
	}

	lastActive := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	sessionsByAgent := map[string][]map[string]any{
		"claude": {{"id": "sess-1", "last_active": lastActive, "meta": map[string]any{"cwd": "/repo/x"}, "title": "Fixture session"}},
	}
	eventsById := map[string]map[string]any{
		"sess-1": {"events": []any{map[string]any{"kind": "skill_invoked", "skill": map[string]any{"skill": "fdesign", "source": "slash"}, "timestamp": lastActive}}},
	}
	stub := newAgentStubPeek(t, eventsById, sessionsByAgent, nil)
	claudeHome := t.TempDir()
	transcript := copyTranscriptFixture(t, claudeHome, "sess-1")
	switchRecord := &sessionstate.SessionState{Switches: sessionstate.Switches{History: []sessionstate.SwitchEvent{
		{Action: sessionstate.SwitchDisabled, At: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), Feature: sessionstate.InjectGlobal},
		{Action: sessionstate.SwitchEnabled, At: time.Date(2000, 1, 1, 1, 0, 0, 0, time.UTC), Feature: sessionstate.InjectGlobal},
	}}}
	require.NoError(t, sessionstate.Save(sessionstate.File(sessionstate.StateDir(claudeHome), "sess-1"), switchRecord))
	server := newTestServer(t, &Options{ClaudeHome: claudeHome, ContextDir: filepath.Join(t.TempDir(), "docs"), PeekEndpoint: stub.URL})
	record, err := contextrecord.Extract(transcript, "docs")
	require.NoError(t, err)

	var root, explore *contextrecord.Agent
	for index := range record.Agents {
		agent := &record.Agents[index]
		switch {
		case agent.Kind == contextrecord.AgentRoot:
			root = agent
		case agent.Type == "Explore":
			explore = agent
		}
	}
	require.NotNil(t, root)
	require.NotNil(t, explore)
	tests := make([]*testCase, 0)

	// no-scope
	tests = append(tests, &testCase{
		_id:       "no-scope",
		_expected: []string{totalsLine(&record.Totals), "## Invoked skills\n\n| Skill |"},
		_missing:  []string{"- Agents:"},

		path: "/sessions-report/sess-1/md",
	})

	// root
	tests = append(tests, &testCase{
		_id:       "root",
		_expected: []string{"- Agents: `root`\n", totalsLine(&root.Totals), "## Invoked skills\n\n| Skill | Version | Skill tokens | Source | Started | Ended |\n|---|---|---|---|---|---|\n| /fdesign |"},
		_missing:  nil,

		path: "/sessions-report/sess-1/md?agent=root",
	})

	// root-page-dims-subagents
	tests = append(tests, &testCase{
		_id:       "root-page-dims-subagents",
		_expected: []string{`<tr class="agent-dim" title="a1">`, `<tr title="main">`, `<a class="badge badge-ok" href="/sessions-report/sess-1?agent=root">root</a>`},
		_missing:  []string{`<tr class="agent-dim" title="main">`, "data-agent"},

		path: "/sessions-report/sess-1?agent=root",
	})

	// type
	tests = append(tests, &testCase{
		_id:       "type",
		_expected: []string{"- Agents: `type:Explore`\n", totalsLine(&explore.Totals), "## Invoked skills\n\n—\n"},
		_missing:  []string{"| /fdesign | "},

		path: "/sessions-report/sess-1/md?agent=type:Explore",
	})

	// unknown-id
	tests = append(tests, &testCase{
		_id:       "unknown-id",
		_expected: []string{"- Agents: `id:nope`\n", "| **Total** | | | 0 | 0 | — |", "### Hook injections\n\n—\n", "### Context files read [Read] [read-gate]\n\n—\n"},
		_missing:  []string{"| Hook | Subject |"},

		path: "/sessions-report/sess-1/md?agent=id:nope",
	})

	// scope-plus-filter
	tests = append(tests, &testCase{
		_id: "scope-plus-filter",
		_expected: []string{
			`href="/sessions-report/sess-1/md?layer=rule&amp;agent=type%3AExplore"`,
			`href="/sessions-report/sess-1?layer=rule&amp;agent=root"`,
			`href="/sessions-report/sess-1?layer=rule&amp;agent=id%3Amain"`,
			`href="/sessions-report/sess-1?layer=acdsl&amp;agent=type%3AExplore"`,
		},
		_missing: nil,

		path: "/sessions-report/sess-1?layer=rule&agent=type:Explore",
	})

	// page-shows-session-features
	tests = append(tests, &testCase{
		_id: "page-shows-session-features",
		_expected: []string{
			"<h2>Session features</h2>",
			`<button type="button" class="feature-toggle">inject:global</button>`,
			`<tr class="feature-detail" hidden>`,
			`<code>hb</code>`,
			`global-context</span><span class="warn-mark" title="inject:global was disabled at `,
			`<input type="checkbox" id="unit-switch">`,
			`<div class="table-scroll">`,
		},
		_missing: []string{`href="/switches"`, `data-unit="chars"`},

		path: "/sessions-report/sess-1",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
			require.Equal(t, http.StatusOK, response.Code)
			assertContainsAll(t, response.Body.String(), test._expected, test._missing)
		})
	}
}

// assertContainsAll asserts every expected fragment is in body and every
// missing one is not.
func assertContainsAll(t *testing.T, body string, expected, missing []string) {
	t.Helper()
	for _, fragment := range expected {
		assert.Contains(t, body, fragment)
	}
	for _, fragment := range missing {
		assert.NotContains(t, body, fragment)
	}
}

func TestBatchPagePeekLinks(t *testing.T) {
	sessionsDir := t.TempDir()
	writeWeekFixture(t, sessionsDir, "personal")

	server := newTestServer(t, &Options{
		PeekDashboardURL: "http://127.0.0.1:4243/",
		SessionsDir:      sessionsDir,
	})

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/2026-W29", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, `href="http://127.0.0.1:4243/sessions/abc123"`, "session id links to peek")
	assert.Contains(t, body, `href="http://127.0.0.1:4243/sessions/def456"`, "stale/unknown session id links to peek too")
}

func TestBatchPageNoDashboardNoLinks(t *testing.T) {
	sessionsDir := t.TempDir()
	writeWeekFixture(t, sessionsDir, "personal")
	server := newTestServer(t, &Options{SessionsDir: sessionsDir})

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/2026-W29", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.NotContains(t, response.Body.String(), "open in peek")
}

const fixtureWeek = "2026-W29"

// weekFixtureRecords are three rendered session records of one week; jkl012 is format 2.
var weekFixtureRecords = map[string]string{
	"abc123": `{
  "id": "abc123",
  "agent": "claude",
  "title": "First session",
  "repo": "personal",
  "week": "2026-W29",
  "lastActive": "2026-07-16T10:00:00Z",
  "signal": "yes (high)",
  "findings": [{"dimension": "memory", "summary": "a finding", "quotes": ["a quote"], "snippets": [{"kind": "violation", "lang": "go", "code": "func broken() {}", "source": "transcript"}]}],
  "frustration": [{"quote": "argh", "trigger": "the trigger"}]
}`,
	"def456": `{
  "id": "def456",
  "agent": "claude",
  "title": "Second session",
  "repo": "personal",
  "week": "2026-W29",
  "lastActive": "2026-07-15T10:00:00Z",
  "findings": [{"dimension": "rule", "summary": "another finding", "quotes": []}],
  "positive": [{"quote": "worked great", "trigger": "clean run"}]
}`,
	"jkl012": `{
  "id": "jkl012",
  "agent": "claude",
  "title": "Third session",
  "repo": "personal",
  "week": "2026-W29",
  "lastActive": "2026-07-14T10:00:00Z",
  "format": 2,
  "dataGaps": "one gap",
  "findings": [{"dimension": "rule", "headline": "The guard was weakened to pass a test.", "summary": "a long finding. with two sentences", "quotes": [{"text": "skip the check", "turn": 7}]}]
}`,
}

func writeWeekFixture(t *testing.T, sessionsDir, scope string) {
	t.Helper()
	weekDir := filepath.Join(sessionsDir, scope, fixtureWeek)
	require.NoError(t, os.MkdirAll(weekDir, 0o755))
	for id, content := range weekFixtureRecords {
		require.NoError(t, os.WriteFile(filepath.Join(weekDir, id+".json"), []byte(content), 0o644))
	}
}

// runSessionsGit runs one git command in dir; the archive handlers commit, so their fixture is a checkout.
func runSessionsGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	output, err := shell.Run(t.Context(), dir, "git", args...)
	require.NoError(t, err, output)
}

// commitSessionsCheckout makes the parent of sessionsDir a git checkout and commits everything under it.
func commitSessionsCheckout(t *testing.T, sessionsDir string) {
	t.Helper()
	repo := filepath.Dir(sessionsDir)
	runSessionsGit(t, repo, "init", "-q")
	runSessionsGit(t, repo, "config", "user.name", "t")
	runSessionsGit(t, repo, "config", "user.email", "t@t")
	runSessionsGit(t, repo, "config", "commit.gpgsign", "false")
	runSessionsGit(t, repo, "config", "core.hooksPath", t.TempDir())
	runSessionsGit(t, repo, "add", "-A")
	runSessionsGit(t, repo, "commit", "-q", "-m", "records")
}

func TestWeekPageInvalidWeek(t *testing.T) {
	sessionsDir := t.TempDir()
	writeWeekFixture(t, sessionsDir, "work")
	server := newTestServer(t, &Options{SessionsDir: sessionsDir})

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/work/12", nil))
	assert.Equal(t, http.StatusBadRequest, response.Code)

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/work/"+fixtureWeek, nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `hx-get="/sessions/work/`+fixtureWeek, "htmx URLs key off the directory scope and week")
}

func TestSessionsReloadFailureServesPreviousState(t *testing.T) {
	sessionsDir := filepath.Join(t.TempDir(), "sessions")
	writeWeekFixture(t, sessionsDir, "personal")
	options := &Options{SessionsDir: sessionsDir}
	server := newTestServer(t, options)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal", nil))
	require.Equal(t, http.StatusOK, response.Code)

	// A file where the folder was: every later rescan fails (a missing folder
	// would reload as empty instead).
	require.NoError(t, os.RemoveAll(sessionsDir))
	require.NoError(t, os.WriteFile(sessionsDir, []byte("not a folder"), 0o644))

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "2026-W29 · 3 sessions")

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/2026-W29", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `id="session-abc123"`)
}

func TestSessionListCasualSnippetSummary(t *testing.T) {
	sessionsDir := t.TempDir()
	writeWeekFixture(t, sessionsDir, "personal")
	options := &Options{PresentationPath: writeGermanProfile(t), SessionsDir: sessionsDir}
	server := newTestServer(t, options)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/2026-W29", nil))

	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "<summary>Technisches Detail</summary>")
	assert.NotContains(t, body, "violation · go · from transcript")
	assert.Contains(t, body, "<pre><code>func broken() {}</code></pre>")
}

func TestSessionsPages(t *testing.T) {
	sessionsDir := filepath.Join(t.TempDir(), "sessions")
	writeWeekFixture(t, sessionsDir, "personal")
	writeWeekFixture(t, sessionsDir, "oldscope")
	commitSessionsCheckout(t, sessionsDir)
	server := newTestServer(t, &Options{SessionsDir: sessionsDir})

	t.Run("index-renders-all-repos", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `<option value="" selected>all repos</option>`)
		assert.Contains(t, body, `hx-get="/sessions-context" hx-trigger="load"`)
		assert.Contains(t, body, `<option value="personal">personal</option>`)
		assert.Contains(t, body, `href="/sessions/personal/2026-W29"`)
		assert.Contains(t, body, "personal · 2026-W29 · 3 sessions")
		assert.Contains(t, body, "mined records")
	})

	t.Run("scope-page-lists-weeks", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `href="/sessions/personal/2026-W29">Week 2026-07-13 – 2026-07-19</a>`)
		assert.Contains(t, body, "2026-W29 · 3 sessions")
		assert.NotContains(t, body, "personal · 2026-W29")
		assert.Contains(t, body, `hx-get="/sessions-context?scope=personal"`)
	})

	t.Run("week-page-renders", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/2026-W29", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `id="session-abc123"`)
		assert.Contains(t, body, "memory")
		assert.Contains(t, body, "a quote")
		assert.Contains(t, body, "violation · go · from transcript")
		assert.Contains(t, body, "<pre><code>func broken() {}</code></pre>")
		assert.Contains(t, body, "<html")
		assert.Contains(t, body, `id="batch-body"`)
		assert.Contains(t, body, `class="session-select"`)
		assert.NotContains(t, body, "Show all sessions")
		assert.Contains(t, body, `<span class="meta session-signal">analyzed</span>`)
		assert.NotContains(t, body, `title="yes (high)">analyzed<`)
		assert.Contains(t, body, `<details class="finding-card" id="f-abc123-0">`)
		assert.Contains(t, body, `<span class="headline">a finding</span>`)
		assert.NotContains(t, body, `<span class="badge badge-dim">memory 1</span>`)
		assert.Contains(t, body, `<span class="headline">The guard was weakened to pass a test.</span>`)
		assert.Contains(t, body, `<span class="quote-turn">turn 7</span>skip the check`)
		assert.Contains(t, body, `<details class="data-gaps"><summary>⚠ data gaps</summary>`)
		assert.NotContains(t, body, "data gaps:")
		assert.Contains(t, body, `forEach(d => d.open = true)">Expand all</button>`)
		assert.Contains(t, body, `forEach(d => d.open = false)">Collapse all</button>`)
		assert.NotContains(t, body, `class="select-all"`)
	})

	t.Run("reload-endpoint-removed", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/sessions/reload", url.Values{}))
		// the GET /sessions/{scope} pattern still matches the path, so the
		// removed POST route answers 405 — either way, no reload handler
		require.Equal(t, http.StatusMethodNotAllowed, response.Code)
	})

	t.Run("live-tab-gone", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.NotContains(t, response.Body.String(), `href="/sessions/live"`)
		assert.NotContains(t, response.Body.String(), "live now")
		assert.Contains(t, response.Body.String(), `<option value="personal" selected>personal</option>`)
		assert.NotContains(t, response.Body.String(), `<a href="/sessions/personal"`)

		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/live", nil))
		assert.Equal(t, http.StatusNotFound, response.Code)
	})

	t.Run("report-route-without-peek", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions-report/abc123", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "peek-mcp unreachable")
		assert.Contains(t, response.Body.String(), `href="/sessions"`)
		assert.NotContains(t, response.Body.String(), "Download .md")
	})

	t.Run("report-markdown-without-peek", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions-report/abc123/md", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, "text/markdown; charset=utf-8", response.Header().Get("Content-Type"))
		assert.Equal(t, `attachment; filename="session-abc123-context-report.md"`, response.Header().Get("Content-Disposition"))
		assert.Contains(t, response.Body.String(), "peek-mcp unreachable")
	})

	t.Run("batch-page-without-peek-has-no-badges", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/2026-W29", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.NotContains(t, response.Body.String(), "/sessions-report/")
	})

	t.Run("no-reload-button-rendered", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.NotContains(t, response.Body.String(), "Reload from disk")
		assert.NotContains(t, response.Body.String(), "Archive folder")
		assert.Contains(t, response.Body.String(), "archived records")
		assert.Contains(t, response.Body.String(), `href="/sessions-archive"`)
	})

	t.Run("archive-tab-empty-before-first-archive", func(t *testing.T) {
		require.NoDirExists(t, filepath.Join(sessionsDir, "archived"))

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions-archive", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "No archived folders.")
	})

	t.Run("rescan-per-request-picks-up-new-batch", func(t *testing.T) {
		writeWeekFixture(t, sessionsDir, "latecomer")
		defer os.RemoveAll(filepath.Join(sessionsDir, "latecomer"))

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/latecomer", nil))
		require.Equal(t, http.StatusOK, response.Code)
	})

	t.Run("scope-named-archive-reachable", func(t *testing.T) {
		writeWeekFixture(t, sessionsDir, "archive")
		defer os.RemoveAll(filepath.Join(sessionsDir, "archive"))

		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/archive", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `<option value="archive" selected>archive</option>`)

		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions-archive", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "No archived folders.")
	})

	t.Run("archive-unarchive-delete-flow", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions-archive", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `<select name="scope"`)
		assert.Contains(t, response.Body.String(), `<option value="oldscope">oldscope</option>`)

		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/sessions-archive", url.Values{"scope": {"oldscope"}}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, "/sessions-archive", response.Header().Get("HX-Redirect"))
		assert.DirExists(t, filepath.Join(sessionsDir, "archived", "oldscope"))

		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions-archive", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "oldscope")
		assert.Contains(t, response.Body.String(), "Unarchive")

		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/sessions-archive/oldscope/unarchive", url.Values{}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, "/sessions-archive", response.Header().Get("HX-Redirect"))
		assert.DirExists(t, filepath.Join(sessionsDir, "oldscope"))

		// delete works only on archived folders — a live name errors, and an error never redirects
		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/sessions-archive/oldscope/delete", url.Values{}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "is not archived")
		assert.Empty(t, response.Header().Get("HX-Redirect"))
		assert.DirExists(t, filepath.Join(sessionsDir, "oldscope"))

		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/sessions-archive", url.Values{"scope": {"oldscope"}}))
		require.Equal(t, http.StatusOK, response.Code)
		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/sessions-archive/oldscope/delete", url.Values{}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, "/sessions-archive", response.Header().Get("HX-Redirect"))
		assert.NoDirExists(t, filepath.Join(sessionsDir, "archived", "oldscope"))

		subject, err := shell.Run(t.Context(), sessionsDir, "git", "log", "-1", "--format=%s")
		require.NoError(t, err, subject)
		assert.Equal(t, "sessions: Deleted the archived oldscope records", strings.TrimSpace(subject))
	})

	t.Run("archive-outside-checkout-renders-error-without-redirect", func(t *testing.T) {
		plainDir := t.TempDir()
		writeWeekFixture(t, plainDir, "plainscope")
		plainServer := newTestServer(t, &Options{SessionsDir: plainDir})

		response := httptest.NewRecorder()
		plainServer.Handler().ServeHTTP(response, formPost("/sessions-archive", url.Values{"scope": {"plainscope"}}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "sessions dir is not a git checkout")
		assert.Empty(t, response.Header().Get("HX-Redirect"))
		assert.DirExists(t, filepath.Join(plainDir, "plainscope"))
	})

	t.Run("archive-without-scope-fails", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/sessions-archive", url.Values{}))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "Invalid scope name")
	})

	t.Run("old-archive-route-gone", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, formPost("/sessions/personal/archive", url.Values{}))
		assert.Equal(t, http.StatusMethodNotAllowed, response.Code)
		assert.DirExists(t, filepath.Join(sessionsDir, "personal"))
	})

	t.Run("session-filter-single-id", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/2026-W29?session=abc123", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `id="session-abc123"`)
		assert.NotContains(t, body, `id="session-def456"`)
		assert.Contains(t, body, `hx-get="/sessions/personal/2026-W29" hx-target="#batch-body" hx-swap="outerHTML" hx-push-url="true">Show all sessions</button>`)
		assert.Contains(t, body, "target.classList.add('arrival-flash')")
	})

	t.Run("collapsed-by-default-expanded-when-selected", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/2026-W29", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.NotContains(t, response.Body.String(), `id="session-abc123" class="card card-column session-card" open`)

		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/2026-W29?session=abc123", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `id="session-abc123" class="card card-column session-card" open`)
	})

	t.Run("open-param-expands-only-target-card", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/2026-W29?open=abc123", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `id="session-abc123" class="card card-column session-card" open`)
		assert.NotContains(t, body, `id="session-def456" class="card card-column session-card" open`)
		assert.Contains(t, body, `id="session-def456"`)
	})

	t.Run("session-filter-multi-id", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/2026-W29?session=abc123,def456", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `id="session-abc123"`)
		assert.Contains(t, body, `id="session-def456"`)
	})

	t.Run("dimension-filter-fragment", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/sessions/personal/2026-W29?dimension=memory", nil)
		request.Header.Set("HX-Request", "true")
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `id="batch-body"`)
		assert.Contains(t, body, `id="session-list"`)
		assert.NotContains(t, body, "<html")
	})

	t.Run("unknown-week-404", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/2026-W01", nil))
		assert.Equal(t, http.StatusNotFound, response.Code)
	})

	t.Run("pseudo-dimensions-in-filter-bar", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/2026-W29", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, "?dimension=frustration")
		assert.Contains(t, body, "?dimension=positive")
	})

	t.Run("positive-filter-keeps-only-positive-sessions", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/2026-W29?dimension=positive", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `id="session-def456"`)
		assert.NotContains(t, body, `id="session-abc123"`)
		assert.Contains(t, body, `<span class="badge badge-ok">positive</span>`)
		assert.Contains(t, body, `class="meta positive"`)
		assert.Contains(t, body, "worked great")
	})

	t.Run("frustration-filter-shows-quote-block", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/2026-W29?dimension=frustration", nil))
		require.Equal(t, http.StatusOK, response.Code)
		body := response.Body.String()
		assert.Contains(t, body, `id="session-abc123"`)
		assert.NotContains(t, body, `id="session-def456"`)
		assert.Contains(t, body, `<span class="badge badge-error">frustration</span>`)
		assert.Contains(t, body, "argh")
		// the finding-dimension filter still hides the quote blocks
		response = httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal/2026-W29?dimension=memory", nil))
		require.Equal(t, http.StatusOK, response.Code)
		assert.NotContains(t, response.Body.String(), "argh")
	})
}

func TestSessionsScopeRegisteredUnmined(t *testing.T) {
	sessionsDir := t.TempDir()
	writeWeekFixture(t, sessionsDir, "personal")

	reposPath := filepath.Join(t.TempDir(), "repos.json")
	registry := `{"repos": [{"name": "unmined", "path": ` + jsonString(t, t.TempDir()) + `}]}`
	require.NoError(t, os.WriteFile(reposPath, []byte(registry), 0o644))
	server := newTestServer(t, &Options{ReposPath: reposPath, SessionsDir: sessionsDir})

	// the scope select lists the union of mined and registered names
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/personal", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `<option value="unmined"`)

	// a registered repo without mined batches renders the empty state
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/unmined", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "No mined records yet")

	// an unregistered unknown scope still 404s
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/nosuch", nil))
	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestContextSessionsUnder(t *testing.T) {
	type testCase struct {
		_id          string
		_expectedIds []string

		peekSessions map[string]peek.Session
		root         string
	}

	older := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	tests := make([]*testCase, 0)

	// cwd-equals-root
	tests = append(tests, &testCase{
		_id:          "cwd-equals-root",
		_expectedIds: []string{"a"},

		peekSessions: map[string]peek.Session{"a": {Id: "a", Cwd: filepath.FromSlash("/repo"), Live: true}},
		root:         filepath.FromSlash("/repo/"),
	})

	// cwd-under-root-worktree
	tests = append(tests, &testCase{
		_id:          "cwd-under-root-worktree",
		_expectedIds: []string{"a"},

		peekSessions: map[string]peek.Session{"a": {Id: "a", Cwd: filepath.FromSlash("/repo/.claude/worktrees/x"), Live: true}},
		root:         filepath.FromSlash("/repo"),
	})

	// sibling-prefix-excluded
	tests = append(tests, &testCase{
		_id:          "sibling-prefix-excluded",
		_expectedIds: nil,

		peekSessions: map[string]peek.Session{"a": {Id: "a", Cwd: filepath.FromSlash("/repo2"), Live: true}},
		root:         filepath.FromSlash("/repo"),
	})

	// stale-included
	tests = append(tests, &testCase{
		_id:          "stale-included",
		_expectedIds: []string{"a"},

		peekSessions: map[string]peek.Session{"a": {Id: "a", Cwd: filepath.FromSlash("/repo"), Live: false}},
		root:         filepath.FromSlash("/repo"),
	})

	// newest-first
	tests = append(tests, &testCase{
		_id:          "newest-first",
		_expectedIds: []string{"new", "old"},

		peekSessions: map[string]peek.Session{
			"old": {Id: "old", Cwd: filepath.FromSlash("/repo"), LastActive: older, Live: true},
			"new": {Id: "new", Cwd: filepath.FromSlash("/repo/sub"), LastActive: newer, Live: true},
		},
		root: filepath.FromSlash("/repo"),
	})

	// nil-map
	tests = append(tests, &testCase{
		_id:          "nil-map",
		_expectedIds: nil,

		peekSessions: nil,
		root:         filepath.FromSlash("/repo"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			views := contextSessionsUnder(test.peekSessions, test.root)
			var ids []string
			for _, view := range views {
				ids = append(ids, view.Session.Id)
			}
			assert.Equal(t, test._expectedIds, ids)
		})
	}
}

func TestGroupContextSessionsByWeek(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []contextWeekGroup

		location *time.Location
		views    []contextSessionView
	}

	berlin := time.FixedZone("CEST", 2*60*60)
	sundayLate := contextSessionView{Session: peek.Session{Id: "sun", LastActive: time.Date(2026, 9, 27, 23, 30, 0, 0, time.UTC)}}
	mondayEarly := contextSessionView{Session: peek.Session{Id: "mon", LastActive: time.Date(2026, 9, 28, 0, 30, 0, 0, time.UTC)}}
	saturday := contextSessionView{Session: peek.Session{Id: "sat-a", LastActive: time.Date(2026, 9, 26, 16, 0, 0, 0, time.UTC)}}
	saturdayEarlier := contextSessionView{Session: peek.Session{Id: "sat-b", LastActive: time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)}}
	monday := contextSessionView{Session: peek.Session{Id: "w39-mon", LastActive: time.Date(2026, 9, 21, 9, 0, 0, 0, time.UTC)}}
	lastWeek := contextSessionView{Session: peek.Session{Id: "w38", LastActive: time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)}}
	week39 := sessions.Week{Key: "2026-W39"}
	week38 := sessions.Week{Key: "2026-W38"}
	week40 := sessions.Week{Key: "2026-W40"}
	tests := make([]*testCase, 0)

	// empty
	tests = append(tests, &testCase{
		_id:       "empty",
		_expected: nil,

		location: time.UTC,
		views:    nil,
	})

	// one-week-two-days
	tests = append(tests, &testCase{
		_id: "one-week-two-days",
		_expected: []contextWeekGroup{{
			Count: 3,
			Days: []contextDayGroup{
				{Label: "Sat 2026-09-26", Sessions: []contextSessionView{saturday, saturdayEarlier}},
				{Label: "Mon 2026-09-21", Sessions: []contextSessionView{monday}},
			},
			Key:   "2026-W39",
			Label: week39.Label(),
		}},

		location: time.UTC,
		views:    []contextSessionView{saturday, saturdayEarlier, monday},
	})

	// two-weeks-newest-first
	tests = append(tests, &testCase{
		_id: "two-weeks-newest-first",
		_expected: []contextWeekGroup{
			{Count: 1, Days: []contextDayGroup{{Label: "Sat 2026-09-26", Sessions: []contextSessionView{saturday}}}, Key: "2026-W39", Label: week39.Label()},
			{Count: 1, Days: []contextDayGroup{{Label: "Fri 2026-09-18", Sessions: []contextSessionView{lastWeek}}}, Key: "2026-W38", Label: week38.Label()},
		},

		location: time.UTC,
		views:    []contextSessionView{saturday, lastWeek},
	})

	// sunday-monday-boundary-utc
	tests = append(tests, &testCase{
		_id: "sunday-monday-boundary-utc",
		_expected: []contextWeekGroup{
			{Count: 1, Days: []contextDayGroup{{Label: "Mon 2026-09-28", Sessions: []contextSessionView{mondayEarly}}}, Key: "2026-W40", Label: week40.Label()},
			{Count: 1, Days: []contextDayGroup{{Label: "Sun 2026-09-27", Sessions: []contextSessionView{sundayLate}}}, Key: "2026-W39", Label: week39.Label()},
		},

		location: time.UTC,
		views:    []contextSessionView{mondayEarly, sundayLate},
	})

	// local-day-crosses-into-next-week
	tests = append(tests, &testCase{
		_id: "local-day-crosses-into-next-week",
		_expected: []contextWeekGroup{{
			Count: 2,
			Days:  []contextDayGroup{{Label: "Mon 2026-09-28", Sessions: []contextSessionView{mondayEarly, sundayLate}}},
			Key:   "2026-W40",
			Label: week40.Label(),
		}},

		location: berlin,
		views:    []contextSessionView{mondayEarly, sundayLate},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, groupContextSessionsByWeek(test.location, test.views))
		})
	}
}

func TestContextSessionsAll(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedIds   []string
		_expectedRepos []string

		peekSessions map[string]peek.Session
		registered   []repos.Repo
	}

	older := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	outer := repos.Repo{Name: "outer", Path: filepath.FromSlash("/code")}
	inner := repos.Repo{Name: "inner", Path: filepath.FromSlash("/code/inner")}
	tests := make([]*testCase, 0)

	// deepest-repo-wins
	tests = append(tests, &testCase{
		_id:            "deepest-repo-wins",
		_expectedIds:   []string{"a"},
		_expectedRepos: []string{"inner"},

		peekSessions: map[string]peek.Session{"a": {Id: "a", Cwd: filepath.FromSlash("/code/inner/.claude/worktrees/x")}},
		registered:   []repos.Repo{inner, outer},
	})

	// unregistered-cwd-excluded
	tests = append(tests, &testCase{
		_id:            "unregistered-cwd-excluded",
		_expectedIds:   []string{"a"},
		_expectedRepos: []string{"outer"},

		peekSessions: map[string]peek.Session{
			"a": {Id: "a", Cwd: filepath.FromSlash("/code/x")},
			"b": {Id: "b", Cwd: filepath.FromSlash("/elsewhere")},
		},
		registered: []repos.Repo{outer},
	})

	// newest-first-ties-by-id
	tests = append(tests, &testCase{
		_id:            "newest-first-ties-by-id",
		_expectedIds:   []string{"c", "a", "b"},
		_expectedRepos: []string{"outer", "outer", "outer"},

		peekSessions: map[string]peek.Session{
			"a": {Id: "a", Cwd: filepath.FromSlash("/code"), LastActive: older},
			"b": {Id: "b", Cwd: filepath.FromSlash("/code"), LastActive: older},
			"c": {Id: "c", Cwd: filepath.FromSlash("/code"), LastActive: newer},
		},
		registered: []repos.Repo{outer},
	})

	// nil-map
	tests = append(tests, &testCase{
		_id:            "nil-map",
		_expectedIds:   nil,
		_expectedRepos: nil,

		peekSessions: nil,
		registered:   []repos.Repo{outer},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			views := contextSessionsAll(test.peekSessions, test.registered)
			var ids, names []string
			for _, view := range views {
				ids = append(ids, view.Session.Id)
				names = append(names, view.Repo)
			}
			assert.Equal(t, test._expectedIds, ids)
			assert.Equal(t, test._expectedRepos, names)
		})
	}
}

func TestLinkContextSessions(t *testing.T) {
	sessionsDir := t.TempDir()
	writeWeekFixture(t, sessionsDir, "personal")
	server := newTestServer(t, &Options{SessionsDir: sessionsDir})
	server.reloadSessions()
	views := []contextSessionView{
		{Session: peek.Session{Id: "abc123"}},
		{Session: peek.Session{Id: "unmined"}},
	}

	server.linkContextSessions(views)

	assert.Equal(t, "/sessions/personal/2026-W29?open=abc123#session-abc123", views[0].RecordUrl)
	assert.Empty(t, views[1].RecordUrl)
}

func TestVisibleSessions(t *testing.T) {
	week := &sessions.Week{
		Sessions: []sessions.Session{
			{
				Id: "s1",
				Findings: []sessions.Finding{
					{Dimension: "memory", Summary: "m1"},
					{Dimension: "rule", Summary: "r1"},
				},
				Frustration: []sessions.Frustration{{Quote: "argh", Trigger: "t"}},
			},
			{
				Id:       "s2",
				Findings: []sessions.Finding{{Dimension: "rule", Summary: "r2"}},
				Positive: []sessions.Positive{{Quote: "nice", Trigger: "t"}},
			},
		},
	}

	tests := []struct {
		name         string
		dimension    string
		peekSessions map[string]peek.Session
		sessionIds   []string
		wantIds      []string
		wantLive     map[string]bool
		wantReport   map[string]bool
		wantSelected bool
		wantSummary  map[string][]string
	}{
		{
			name:        "no-filters-all-sessions",
			wantIds:     []string{"s1", "s2"},
			wantSummary: map[string][]string{"s1": {"m1", "r1"}, "s2": {"r2"}},
		},
		{
			name:         "peek-known-session-has-report",
			peekSessions: map[string]peek.Session{"s1": {Id: "s1"}},
			wantIds:      []string{"s1", "s2"},
			wantReport:   map[string]bool{"s1": true},
			wantSummary:  map[string][]string{"s1": {"m1", "r1"}, "s2": {"r2"}},
		},
		{
			name:         "peek-known-live-session",
			peekSessions: map[string]peek.Session{"s2": {Id: "s2", Live: true}},
			wantIds:      []string{"s1", "s2"},
			wantLive:     map[string]bool{"s2": true},
			wantReport:   map[string]bool{"s2": true},
			wantSummary:  map[string][]string{"s1": {"m1", "r1"}, "s2": {"r2"}},
		},
		{
			name:        "dimension-hides-non-matching",
			dimension:   "memory",
			wantIds:     []string{"s1"},
			wantSummary: map[string][]string{"s1": {"m1"}},
		},
		{
			name:         "session-id-filter",
			sessionIds:   []string{"s2"},
			wantIds:      []string{"s2"},
			wantSelected: true,
			wantSummary:  map[string][]string{"s2": {"r2"}},
		},
		{
			name:         "multiple-session-ids",
			sessionIds:   []string{"s1", "s2"},
			wantIds:      []string{"s1", "s2"},
			wantSelected: true,
			wantSummary:  map[string][]string{"s1": {"m1", "r1"}, "s2": {"r2"}},
		},
		{
			name:         "both-combined",
			dimension:    "rule",
			sessionIds:   []string{"s1"},
			wantIds:      []string{"s1"},
			wantSelected: true,
			wantSummary:  map[string][]string{"s1": {"r1"}},
		},
		{
			name:       "combined-no-match",
			dimension:  "memory",
			sessionIds: []string{"s2"},
			wantIds:    nil,
		},
		{
			name:      "frustration-pseudo-dimension",
			dimension: "frustration",
			wantIds:   []string{"s1"},
		},
		{
			name:      "positive-pseudo-dimension",
			dimension: "positive",
			wantIds:   []string{"s2"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			views := visibleSessions(test.dimension, test.peekSessions, test.sessionIds, week)
			var ids []string
			for _, view := range views {
				ids = append(ids, view.Session.Id)
				assert.Equal(t, test.wantSelected, view.Selected)
				assert.Equal(t, test.wantReport[view.Session.Id], view.HasReport)
				assert.Equal(t, test.wantLive[view.Session.Id], view.Live)
				var summaries []string
				for _, row := range view.Rows {
					summaries = append(summaries, row.Summary)
				}
				assert.Equal(t, test.wantSummary[view.Session.Id], summaries)
			}
			assert.Equal(t, test.wantIds, ids)
		})
	}
}

func TestFindingRows(t *testing.T) {
	session := &sessions.Session{
		Id: "s1",
		Findings: []sessions.Finding{
			{Dimension: "memory", Headline: "A memory headline.", Summary: "m1"},
			{Dimension: "rule", Summary: "The rule broke: here is why. More text."},
		},
	}

	rows := findingRows("rule", session)

	require.Len(t, rows, 1)
	assert.Equal(t, "f-s1-1", rows[0].Anchor)
	assert.Equal(t, "The rule broke", rows[0].Headline)
	assert.Equal(t, "The rule broke: here is why. More text.", rows[0].Summary)
}

func TestFirstSentence(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		summary string
	}

	long := strings.Repeat("word ", 30)
	tests := make([]*testCase, 0)

	// period-cut
	tests = append(tests, &testCase{
		_id:       "period-cut",
		_expected: "First sentence",

		summary: "First sentence. Second sentence.",
	})

	// colon-cut
	tests = append(tests, &testCase{
		_id:       "colon-cut",
		_expected: "The miner failed",

		summary: "The miner failed: the probe was denied",
	})

	// no-mark
	tests = append(tests, &testCase{
		_id:       "no-mark",
		_expected: "a finding",

		summary: "a finding",
	})

	// over-cap-ellipsis
	tests = append(tests, &testCase{
		_id:       "over-cap-ellipsis",
		_expected: string([]rune(long)[:120]) + "…",

		summary: long,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, firstSentence(test.summary))
		})
	}
}

func TestSignalLabel(t *testing.T) {
	tests := []struct {
		signal string
		want   string
	}{
		{signal: "yes", want: "analyzed"},
		{signal: "yes (high)", want: "analyzed"},
		{signal: "Yes (clean)", want: "analyzed"},
		{signal: "exemplar", want: "exemplar"},
		{signal: "HIGH VALUE — pattern", want: "HIGH VALUE — pattern"},
		{signal: "", want: ""},
	}

	for _, test := range tests {
		t.Run(test.signal, func(t *testing.T) {
			assert.Equal(t, test.want, signalLabel(test.signal))
		})
	}
}

func TestParseSessionIds(t *testing.T) {
	tests := []struct {
		name  string
		query url.Values
		want  []string
	}{
		{
			name:  "repeated-params",
			query: url.Values{"session": {"a", "b"}},
			want:  []string{"a", "b"},
		},
		{
			name:  "comma-joined",
			query: url.Values{"session": {"a,b,c"}},
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "mixed",
			query: url.Values{"session": {"a,b", "c"}},
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "empty-entries-dropped",
			query: url.Values{"session": {" , a ,", ""}},
			want:  []string{"a"},
		},
		{
			name:  "absent",
			query: url.Values{},
			want:  nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, parseSessionIds(test.query))
		})
	}
}

func TestSessionsPagesSkipPeek(t *testing.T) {
	sessionsDir := t.TempDir()
	writeWeekFixture(t, sessionsDir, "personal")
	hanging := newHangingPeek(t)
	server := newTestServer(t, &Options{PeekEndpoint: hanging.url, SessionsDir: sessionsDir})

	for _, path := range []string{"/sessions", "/sessions/personal"} {
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

		assert.Zero(t, hanging.calls.Load(), path)
		require.Equal(t, http.StatusOK, recorder.Code, path)
		assert.Contains(t, recorder.Body.String(), `hx-get="/sessions-context`, path)
		assert.Contains(t, recorder.Body.String(), "mined records", path)
	}
}

func TestSessionsContextFragment(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedContains string
		_expectedMissing  string

		request *http.Request
		server  *Server
	}

	repoDir := t.TempDir()
	reposPath := filepath.Join(t.TempDir(), "repos.json")
	registry := `{"repos": [{"name": "work", "path": ` + jsonString(t, repoDir) + `}]}`
	require.NoError(t, os.WriteFile(reposPath, []byte(registry), 0o644))
	lastActive := time.Date(2026, 10, 6, 11, 42, 0, 0, time.UTC).Format(time.RFC3339)
	stub := newStubPeek(t, []map[string]any{
		{"id": "live1", "last_active": lastActive, "meta": map[string]any{"cwd": repoDir}, "title": "Live work session"},
	})
	answering := newTestServer(t, &Options{PeekEndpoint: stub.URL, ReposPath: reposPath})
	olderDay := time.Date(2026, 10, 5, 11, 42, 0, 0, time.UTC).Format(time.RFC3339)
	twoDaysStub := newStubPeek(t, []map[string]any{
		{"id": "live1", "last_active": lastActive, "meta": map[string]any{"cwd": repoDir}, "title": "Live work session"},
		{"id": "old1", "last_active": olderDay, "meta": map[string]any{"cwd": repoDir}, "title": "Older work session"},
	})
	twoDays := newTestServer(t, &Options{PeekEndpoint: twoDaysStub.URL, ReposPath: reposPath})
	expired, cancel := context.WithDeadline(context.Background(), time.Now())
	t.Cleanup(cancel)
	tests := make([]*testCase, 0)

	// peek-answers-all-repos
	tests = append(tests, &testCase{
		_id:               "peek-answers-all-repos",
		_expectedContains: "work · live1",
		_expectedMissing:  "not answering",

		request: httptest.NewRequest(http.MethodGet, "/sessions-context", nil),
		server:  answering,
	})

	// peek-answers-one-repo
	tests = append(tests, &testCase{
		_id:               "peek-answers-one-repo",
		_expectedContains: "Live work session",
		_expectedMissing:  "work · live1",

		request: httptest.NewRequest(http.MethodGet, "/sessions-context?scope=work", nil),
		server:  answering,
	})

	// unregistered-scope-lists-nothing
	tests = append(tests, &testCase{
		_id:               "unregistered-scope-lists-nothing",
		_expectedContains: `<div id="sessions-context">`,
		_expectedMissing:  "Live work session",

		request: httptest.NewRequest(http.MethodGet, "/sessions-context?scope=nosuch", nil),
		server:  answering,
	})

	// peek-disabled-renders-nothing
	tests = append(tests, &testCase{
		_id:               "peek-disabled-renders-nothing",
		_expectedContains: `<div id="sessions-context">`,
		_expectedMissing:  "<h2>",

		request: httptest.NewRequest(http.MethodGet, "/sessions-context", nil),
		server:  newTestServer(t, &Options{ReposPath: reposPath}),
	})

	// peek-unreachable-retries
	tests = append(tests, &testCase{
		_id:               "peek-unreachable-retries",
		_expectedContains: `hx-get="/sessions-context?scope=work" hx-trigger="load delay:15s"`,
		_expectedMissing:  "Live work session",

		request: httptest.NewRequest(http.MethodGet, "/sessions-context?scope=work", nil),
		server:  newTestServer(t, &Options{PeekEndpoint: "http://127.0.0.1:1/mcp", ReposPath: reposPath}),
	})

	// peek-out-of-time-degrades
	tests = append(tests, &testCase{
		_id:               "peek-out-of-time-degrades",
		_expectedContains: "peek is not answering yet",
		_expectedMissing:  "Live work session",

		request: httptest.NewRequest(http.MethodGet, "/sessions-context", nil).WithContext(expired),
		server:  answering,
	})

	// newest-day-open
	tests = append(tests, &testCase{
		_id:               "newest-day-open",
		_expectedContains: `<details class="context-day" open>`,
		_expectedMissing:  "not answering",

		request: httptest.NewRequest(http.MethodGet, "/sessions-context", nil),
		server:  twoDays,
	})

	// older-day-collapsed
	tests = append(tests, &testCase{
		_id:               "older-day-collapsed",
		_expectedContains: `<details class="context-day">`,
		_expectedMissing:  "not answering",

		request: httptest.NewRequest(http.MethodGet, "/sessions-context", nil),
		server:  twoDays,
	})

	// row-has-no-context-link
	tests = append(tests, &testCase{
		_id:               "row-has-no-context-link",
		_expectedContains: "Live work session",
		_expectedMissing:  `">context</a>`,

		request: httptest.NewRequest(http.MethodGet, "/sessions-context", nil),
		server:  answering,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			response := httptest.NewRecorder()
			test.server.Handler().ServeHTTP(response, test.request)
			assert.Equal(t, http.StatusOK, response.Code)
			assert.Contains(t, response.Body.String(), test._expectedContains)
			assert.NotContains(t, response.Body.String(), test._expectedMissing)
		})
	}
}

func TestSessionDays(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []sessionDayView

		open  string
		views []sessionView
	}

	monday := sessionView{Session: &sessions.Session{Id: "mon", LastActive: "2026-07-13T10:00:00Z"}}
	wednesday := sessionView{Session: &sessions.Session{Id: "wed", LastActive: "2026-07-15T22:00:00Z"}}
	wednesdayEarly := sessionView{Session: &sessions.Session{Id: "wed-early", LastActive: "2026-07-15T08:00:00Z"}}
	undated := sessionView{Session: &sessions.Session{Id: "undated", LastActive: "soon"}}
	selectedMonday := sessionView{Selected: true, Session: monday.Session}
	tests := make([]*testCase, 0)

	// newest-day-open-others-closed
	tests = append(tests, &testCase{
		_id: "newest-day-open-others-closed",
		_expected: []sessionDayView{
			{IsOpen: true, Key: "2026-07-15", Label: "Wed 2026-07-15", Sessions: []sessionView{wednesday, wednesdayEarly}},
			{IsOpen: false, Key: "2026-07-13", Label: "Mon 2026-07-13", Sessions: []sessionView{monday}},
			{IsOpen: false, Key: unknownDayKey, Label: unknownDayLabel, Sessions: []sessionView{undated}},
		},

		open:  "",
		views: []sessionView{wednesday, wednesdayEarly, monday, undated},
	})

	// opened-session-opens-its-day
	tests = append(tests, &testCase{
		_id: "opened-session-opens-its-day",
		_expected: []sessionDayView{
			{IsOpen: true, Key: "2026-07-15", Label: "Wed 2026-07-15", Sessions: []sessionView{wednesday}},
			{IsOpen: true, Key: "2026-07-13", Label: "Mon 2026-07-13", Sessions: []sessionView{monday}},
		},

		open:  "mon",
		views: []sessionView{wednesday, monday},
	})

	// selected-session-opens-its-day
	tests = append(tests, &testCase{
		_id: "selected-session-opens-its-day",
		_expected: []sessionDayView{
			{IsOpen: true, Key: "2026-07-15", Label: "Wed 2026-07-15", Sessions: []sessionView{wednesday}},
			{IsOpen: true, Key: "2026-07-13", Label: "Mon 2026-07-13", Sessions: []sessionView{selectedMonday}},
		},

		open:  "",
		views: []sessionView{wednesday, selectedMonday},
	})

	// no-sessions-no-days
	tests = append(tests, &testCase{
		_id:       "no-sessions-no-days",
		_expected: []sessionDayView{},

		open:  "",
		views: nil,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, sessionDays(time.UTC, test.open, test.views))
		})
	}
}

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/heartbeat"
)

// fleetFixture is the real heartbeat status document the decoder tests use.
func fleetFixture(t *testing.T) string {
	t.Helper()
	fixture, err := os.ReadFile(filepath.Join("..", "heartbeat", "testdata", "status_document.json"))
	require.NoError(t, err)
	return string(fixture)
}

// fleetFixtureWith returns the fixture after edit changed its decoded form.
func fleetFixtureWith(t *testing.T, edit func(document map[string]any)) string {
	t.Helper()
	var document map[string]any
	require.NoError(t, json.Unmarshal([]byte(fleetFixture(t)), &document))
	edit(document)
	output, err := json.Marshal(document)
	require.NoError(t, err)
	return string(output)
}

// writeHeartbeatStub creates a stub heartbeat binary that records its argv in
// `argv` beside itself, prints output and exits with exitCode.
func writeHeartbeatStub(t *testing.T, exitCode int, output string) string {
	t.Helper()
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "output")
	require.NoError(t, os.WriteFile(outputPath, []byte(output), 0o644))
	script := "#!/bin/sh\n" +
		`printf '%s' "$*" > "` + filepath.ToSlash(filepath.Join(dir, "argv")) + "\"\n" +
		`cat "` + filepath.ToSlash(outputPath) + "\"\n" +
		"exit " + strconv.Itoa(exitCode) + "\n"
	stub := filepath.Join(dir, "heartbeat.sh")
	require.NoError(t, os.WriteFile(stub, []byte(script), 0o755))
	return stub
}

func fleetTestServer(t *testing.T, stub string) (*Server, string) {
	t.Helper()
	claudeHome := t.TempDir()
	return newTestServer(t, &Options{ClaudeHome: claudeHome, HeartbeatPath: stub}), claudeHome
}

func serveFleet(server *Server, request *http.Request) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	return response
}

func TestFleetShellRendersPlaceholderOnly(t *testing.T) {
	server, _ := fleetTestServer(t, writeHeartbeatStub(t, 0, fleetFixture(t)))

	response := serveFleet(server, httptest.NewRequest(http.MethodGet, "/fleet", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "<h1>Fleet</h1>")
	assert.Contains(t, body, `<a href="/fleet" class="active">Fleet</a>`)
	assert.Contains(t, body, `<a href="/fleet" class="active">Fleet</a>`)
	assert.Contains(t, body, `<a href="/runtime" >Runtime</a>`)
	assert.Contains(t, body, `hx-trigger="load"`)
	assert.Contains(t, body, "Loading fleet status")
	assert.Contains(t, body, `hx-get="/fleet/status"`)
	assert.NotContains(t, body, "<th")

	filtered := serveFleet(server, httptest.NewRequest(http.MethodGet, "/fleet?fleet=T1", nil))
	require.Equal(t, http.StatusOK, filtered.Code)
	assert.Contains(t, filtered.Body.String(), `hx-get="/fleet/status?fleet=T1"`)
}

func TestFleetStatusFragment(t *testing.T) {
	stub := writeHeartbeatStub(t, 0, fleetFixture(t))
	server, claudeHome := fleetTestServer(t, stub)

	response := serveFleet(server, httptest.NewRequest(http.MethodGet, "/fleet/status?fleet=T1", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "human-layer queue")
	assert.Contains(t, body, "Nothing waits on a human.")
	assert.Contains(t, body, "fleet T1")
	assert.Contains(t, body, `<span class="field-key">target</span><span title="/Users/devuser/GolandProjects/claude-configs"><code>main</code></span>`)
	assert.Contains(t, body, `<span class="badge badge-action">open</span>`)
	assert.Contains(t, body, "<span>fleet branches only</span>")
	assert.Contains(t, body, "CC,FImpl - P3 orchestrator loop acceptance tests")
	assert.Contains(t, body, `<td title="bd080288-be15-4ddb-b88f-993e49c1ffd9">CC,FImpl - P3 orchestrator loop acceptance tests <span class="badge badge-action">READY</span> <span class="meta"`, "session state badge")
	assert.Contains(t, body, `<td title="bd080288-be15-4ddb-b88f-993e49c1ffd9">CC,FImpl - P3 orchestrator loop acceptance tests <span class="badge badge-action">READY</span></td>`, "member state badge")
	assert.NotContains(t, body, `acceptance tests - [READY] <span class="badge`, "state shown once")
	assert.Contains(t, body, "claude/agent-a3ede1d975161fb7f")
	assert.Contains(t, body, `<span class="badge badge-action">uncommitted</span>`)
	assert.Contains(t, body, "<td>operator</td>")
	assert.Contains(t, body, "<td>worker</td>")
	assert.NotContains(t, body, `class="fl-train"`, "no train step is a fleet branch")
	assert.Contains(t, body, "<td>claude/golden-review-triage-b68d35</td><td>claude/skill-protocol-v2-p2-be0aa4</td>")
	assert.Contains(t, body, `<div title="skills/README.md">skills/README.md</div>`)
	assert.NotContains(t, body, "<th>symbols</th>")
	assert.Contains(t, body, `<span title="PreToolUse">01:17:55Z</span>`, "session last active")
	assert.NotContains(t, body, "hx-post=\"/fleet/T1/members/", "no lettered member, no Remove")
	assert.Contains(t, body, `hx-trigger="every 5s, fleet-op from:body"`)
	assert.Contains(t, body, `hx-get="/fleet/status?fleet=T1"`)
	assert.NotContains(t, body, `hx-trigger="load"`)
	assert.Contains(t, body, "No journal entries.")

	argv, err := os.ReadFile(filepath.Join(filepath.Dir(stub), "argv"))
	require.NoError(t, err)
	assert.Equal(t, "--addr 127.0.0.1:6101 --claude-home "+filepath.ToSlash(claudeHome)+" status --format json --fleet T1", string(argv))
}

func TestFleetStatusWorkerRow(t *testing.T) {
	document := fleetFixtureWith(t, func(document map[string]any) {
		worker := document["members"].([]any)[1].(map[string]any)
		worker["member"] = "B"
		worker["token"] = "T1-MB"
	})
	server, _ := fleetTestServer(t, writeHeartbeatStub(t, 0, document))

	response := serveFleet(server, httptest.NewRequest(http.MethodGet, "/fleet/status?fleet=T1", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, `<td><span class="badge badge-action">running</span></td>
    <td title="claude/agent-a3ede1d975161fb7f"><span class="badge badge-dim">T1-MB</span> agent-a3ede1d975161fb7f`, "running worker named by its branch without claude/")
	assert.Contains(t, body, "<td>worker</td>")
	assert.Contains(t, body, `hx-post="/fleet/T1/members/B/remove"`, "a lettered worker keeps its Remove")
}

func TestFleetStatusMemberTokensAndJournal(t *testing.T) {
	document := fleetFixtureWith(t, func(document map[string]any) {
		worker := document["members"].([]any)[1].(map[string]any)
		worker["member"] = "B"
		worker["token"] = "T1-MB"
		worker["delivered"] = true
		older := map[string]any{"at": "2026-09-28T10:00:00Z", "by": "o", "kind": "spawned", "members": []any{"B"}, "subject": "spawned implement-runner"}
		newer := map[string]any{"at": "2026-09-28T11:00:00Z", "by": "o", "detail": "delivered and merge-ready", "kind": "merged", "members": []any{"B"}, "subject": "merged claude/agent-a3ede1d975161fb7f"}
		fleet := document["fleets"].([]any)[0].(map[string]any)
		fleet["journal"] = []any{older, newer}
		fleet["instance"] = "0199a1b2-c3d4-7e5f-8a6b-7c8d9e0f1a2b"
		fleet["started_at"] = "2026-09-28T09:00:00Z"
		fleet["merge_branch"] = "claude-orch/T1"
		fleet["synced"] = true
		fleet["target_state"] = "integrated"
	})
	server, _ := fleetTestServer(t, writeHeartbeatStub(t, 0, document))

	response := serveFleet(server, httptest.NewRequest(http.MethodGet, "/fleet/status?fleet=T1", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, `<span class="badge badge-dim">T1-MB</span> agent-a3ede1d975161fb7f`)
	assert.Contains(t, body, `<span class="badge badge-ok">delivered</span>`)
	assert.Contains(t, body, "<code>claude-orch/T1</code>")
	assert.Contains(t, body, "<code>0199a1b2</code>", "the live run")
	assert.Contains(t, body, `hx-post="/fleet/T1/members/B/remove"`)
	assert.Contains(t, body, `hx-confirm="Remove T1-MB from fleet T1?"`)
	assert.Contains(t, body, `>synced</span>`)
	assert.Contains(t, body, "ready for main")
	assert.Contains(t, body, "<td>merged claude/agent-a3ede1d975161fb7f</td>")
	assert.Contains(t, body, "<td>delivered and merge-ready</td>")
	assert.NotContains(t, body, "No journal entries.")
	assert.Less(t, strings.Index(body, "merged claude/agent-"), strings.Index(body, "spawned implement-runner"), "newest first")
}

func TestFleetStatusLandings(t *testing.T) {
	server, _ := fleetTestServer(t, writeHeartbeatStub(t, 0, fleetFixture(t)))

	response := serveFleet(server, httptest.NewRequest(http.MethodGet, "/fleet/status", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, ">landings</h2>")
	assert.Contains(t, body, `<th title="the branch a landing moves">target</th>`)
	assert.Contains(t, body, `<td title="/Users/devuser/GolandProjects/claude-configs">`)
	assert.Contains(t, body, "<td>develop</td>\n    <td><span class=\"badge badge-ok\">releasable</span></td>\n    <td>3</td>", "staging row: target, badge and count")
	assert.Contains(t, body, "<td>develop</td>\n    <td><span class=\"badge badge-dim\">absent</span></td>\n    <td>0</td>", "staging row of a repo without the staging branch")
	assert.Contains(t, body, "<td>feature/x</td>\n    <td></td>\n    <td></td>", "non-staging row: target, no badge, no count")
	assert.Equal(t, 2, strings.Count(body, "<td>develop</td>"), "one staging row per repo")
	assert.Contains(t, body, `<span title="83985202-8be5-450e-8d92-586ced8742d5">CC,FD - Permission jailbreak fix continuation - [DONE]</span>`)
	assert.Contains(t, body, `<span title="7c1e9a40-2b3d-4f5e-8a6b-9c0d1e2f3a4b">CC,FImpl - Feature x slice - [DONE]</span>`, "the feature target's own holder")
	assert.Contains(t, body, `<div title="4db43599-30f5-4200-a130-331604e5ebaf">4db43599-30f5-4200-a130-331604e5ebaf `, "a waiter without a title shows its id")
	assert.Contains(t, body, `<span class="meta">free</span>`)
	assert.Less(t, strings.Index(body, ">collisions</h2>"), strings.Index(body, ">landings</h2>"), "after collisions")
	assert.Less(t, strings.Index(body, ">landings</h2>"), strings.Index(body, ">fleet sessions</h2>"), "before sessions")

	diverged := fleetFixtureWith(t, func(document map[string]any) {
		landing := document["landings"].([]any)[1].(map[string]any)
		landing["release"].(map[string]any)["releasable"] = false
	})
	server, _ = fleetTestServer(t, writeHeartbeatStub(t, 0, diverged))
	response = serveFleet(server, httptest.NewRequest(http.MethodGet, "/fleet/status", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `<span class="badge badge-action">diverged</span>`)

	without := fleetFixtureWith(t, func(document map[string]any) {
		delete(document, "landings")
	})
	server, _ = fleetTestServer(t, writeHeartbeatStub(t, 0, without))
	response = serveFleet(server, httptest.NewRequest(http.MethodGet, "/fleet/status", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "No landings.")
}

func TestFleetStatusDegraded(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedBanner string
		_hasTables      bool

		server *Server
	}

	degradedPeek := fleetFixtureWith(t, func(document map[string]any) {
		for _, sensor := range document["sensors"].([]any) {
			row := sensor.(map[string]any)
			if row["name"] == "peek" {
				row["state"] = "degraded"
				row["detail"] = "connection refused"
			}
		}
	})
	failedScan := fleetFixtureWith(t, func(document map[string]any) {
		repo := document["repos"].([]any)[0].(map[string]any)
		repo["scan_error"] = "agentwt: no default branch could be determined"
	})
	serverFor := func(exitCode int, output string) *Server {
		server, _ := fleetTestServer(t, writeHeartbeatStub(t, exitCode, output))
		return server
	}

	tests := make([]*testCase, 0)

	// unreachable-exit-3
	tests = append(tests, &testCase{
		_id:             "unreachable-exit-3",
		_expectedBanner: "fleet state unavailable: Status: Heartbeat unreachable",
		_hasTables:      false,

		server: serverFor(3, "heartbeat: no daemon on 127.0.0.1:6101\n"),
	})

	// foreign-exit-4
	tests = append(tests, &testCase{
		_id:             "foreign-exit-4",
		_expectedBanner: "another Claude home",
		_hasTables:      false,

		server: serverFor(4, "heartbeat: 127.0.0.1:6101 serves /other\n"),
	})

	// degraded-sensor
	tests = append(tests, &testCase{
		_id:             "degraded-sensor",
		_expectedBanner: "peek degraded since",
		_hasTables:      true,

		server: serverFor(0, degradedPeek),
	})

	// scan-error
	tests = append(tests, &testCase{
		_id:             "scan-error",
		_expectedBanner: "repo claude-configs scan failed",
		_hasTables:      true,

		server: serverFor(0, failedScan),
	})

	// invalid-fleet-exit-2
	tests = append(tests, &testCase{
		_id:             "invalid-fleet-exit-2",
		_expectedBanner: "fleet state unavailable",
		_hasTables:      false,

		server: serverFor(2, "error: invalid value 'T 1' for '--fleet'\n"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			response := serveFleet(test.server, httptest.NewRequest(http.MethodGet, "/fleet/status", nil))
			assert.Equal(t, http.StatusOK, response.Code)
			assert.Contains(t, response.Body.String(), test._expectedBanner)
			assert.Equal(t, test._hasTables, strings.Contains(response.Body.String(), ">fleet sessions</h2>"))
		})
	}
}

func TestFleetStatusFleetSessionsOnly(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedText   string
		_unexpectedText string

		server *Server
	}

	serverFor := func(output string) *Server {
		server, _ := fleetTestServer(t, writeHeartbeatStub(t, 0, output))
		return server
	}
	withOutsider := fleetFixtureWith(t, func(document map[string]any) {
		outsider := map[string]any{
			"session_id":    "0000aaaa-0000-4000-8000-000000000001",
			"since_restart": false,
			"state":         "idle",
			"state_since":   "2026-09-28T01:00:00Z",
			"subagents":     []any{},
			"title":         "CC,FD - Unrelated work - [WIP]",
		}
		document["sessions"] = append(document["sessions"].([]any), outsider)
	})
	withoutMembers := fleetFixtureWith(t, func(document map[string]any) {
		document["members"] = []any{}
	})

	tests := make([]*testCase, 0)

	// outsider-hidden
	tests = append(tests, &testCase{
		_id:             "outsider-hidden",
		_expectedText:   `<td title="bd080288-be15-4ddb-b88f-993e49c1ffd9">CC,FImpl - P3 orchestrator loop acceptance tests <span class="badge badge-action">READY</span> <span class="meta"`,
		_unexpectedText: "0000aaaa-0000-4000-8000-000000000001",

		server: serverFor(withOutsider),
	})

	// no-members
	tests = append(tests, &testCase{
		_id:             "no-members",
		_expectedText:   "No fleet sessions.",
		_unexpectedText: "bd080288-be15-4ddb-b88f-993e49c1ffd9",

		server: serverFor(withoutMembers),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			response := serveFleet(test.server, httptest.NewRequest(http.MethodGet, "/fleet/status", nil))
			assert.Equal(t, http.StatusOK, response.Code)
			assert.Contains(t, response.Body.String(), test._expectedText)
			assert.NotContains(t, response.Body.String(), test._unexpectedText)
		})
	}
}

// archiveFixtureInstance is the run id of internal/heartbeat/testdata/archive_fleet.json.
const archiveFixtureInstance = "0199a1b2-c3d4-7e5f-8a6b-7c8d9e0f1a2b"

// serverWithArchive is a fleet test server whose claude home holds the
// archive fixture.
func serverWithArchive(t *testing.T) *Server {
	t.Helper()
	server, claudeHome := fleetTestServer(t, writeHeartbeatStub(t, 0, fleetFixture(t)))
	dir := filepath.Join(claudeHome, "heartbeat", "archive")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	fixture, err := os.ReadFile(filepath.Join("..", "heartbeat", "testdata", "archive_fleet.json"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, archiveFixtureInstance+".json"), fixture, 0o600))
	return server
}

func TestFleetStatusListsArchivedFleets(t *testing.T) {
	server := serverWithArchive(t)

	response := serveFleet(server, httptest.NewRequest(http.MethodGet, "/fleet/status?fleet=T1", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "archived fleets")
	assert.Contains(t, body, `<a href="/fleet/archive/`+archiveFixtureInstance+`">T1</a>`)
	assert.Contains(t, body, "<code>0199a1b2</code>")
	assert.NotContains(t, body, "No archived fleets.")

	other := serveFleet(server, httptest.NewRequest(http.MethodGet, "/fleet/status?fleet=T2", nil))
	assert.Contains(t, other.Body.String(), "No archived fleets.", "filtered by name")

	empty, _ := fleetTestServer(t, writeHeartbeatStub(t, 0, fleetFixture(t)))
	none := serveFleet(empty, httptest.NewRequest(http.MethodGet, "/fleet/status", nil))
	assert.Contains(t, none.Body.String(), "No archived fleets.")
}

func TestFleetArchive(t *testing.T) {
	type testCase struct {
		_id           string
		_expectedCode int
		_expectedBody []string

		path string
	}

	server := serverWithArchive(t)
	tests := make([]*testCase, 0)

	// archived-run
	tests = append(tests, &testCase{
		_id:           "archived-run",
		_expectedCode: http.StatusOK,
		_expectedBody: []string{
			`<span class="badge badge-dim">T1-MA</span>`,
			`<span class="badge badge-dim">T1-O</span>`,
			"<td>claude/w1</td>",
			"unsubscribed by the session",
			"<td>merged claude/w1</td>",
			`<a href="/fleet">← fleet</a>`,
		},

		path: "/fleet/archive/" + archiveFixtureInstance,
	})

	// invalid-id
	tests = append(tests, &testCase{
		_id:           "invalid-id",
		_expectedCode: http.StatusBadRequest,
		_expectedBody: []string{},

		path: "/fleet/archive/not-a-uuid",
	})

	// unknown-id
	tests = append(tests, &testCase{
		_id:           "unknown-id",
		_expectedCode: http.StatusNotFound,
		_expectedBody: []string{},

		path: "/fleet/archive/0199a1b2-0000-7000-8000-00000000ffff",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			response := serveFleet(server, httptest.NewRequest(http.MethodGet, test.path, nil))
			assert.Equal(t, test._expectedCode, response.Code, response.Body.String())
			for _, want := range test._expectedBody {
				assert.Contains(t, response.Body.String(), want)
			}
		})
	}
}

func TestShortPath(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		path string
	}

	sixty := strings.Repeat("a", 55) + "/b.go"
	long := "cmd/toolpolicy/" + strings.Repeat("x", 55) + "/fence_specs.go"
	umlauts := strings.Repeat("ä", 70) + "/ü.go"

	tests := make([]*testCase, 0)

	// sixty-runes-kept
	tests = append(tests, &testCase{
		_id:       "sixty-runes-kept",
		_expected: sixty,

		path: sixty,
	})

	// long-cut-from-left
	tests = append(tests, &testCase{
		_id:       "long-cut-from-left",
		_expected: "…" + long[len(long)-59:],

		path: long,
	})

	// multi-byte-rune-boundary
	tests = append(tests, &testCase{
		_id:       "multi-byte-rune-boundary",
		_expected: "…" + strings.Repeat("ä", 54) + "/ü.go",

		path: umlauts,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, shortPath(test.path))
		})
	}
}

func TestHeadTailPaths(t *testing.T) {
	type testCase struct {
		_id           string
		_expectedHead int
		_expectedTail int

		paths []string
	}

	paths := func(count int) []string {
		list := make([]string, 0, count)
		for i := range count {
			list = append(list, "p/"+strconv.Itoa(i)+".go")
		}
		return list
	}

	tests := make([]*testCase, 0)

	// three
	tests = append(tests, &testCase{
		_id:           "three",
		_expectedHead: 3,
		_expectedTail: 0,

		paths: paths(3),
	})

	// eighty-seven
	tests = append(tests, &testCase{
		_id:           "eighty-seven",
		_expectedHead: 10,
		_expectedTail: 77,

		paths: paths(87),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Len(t, headPaths(test.paths), test._expectedHead)
			assert.Len(t, tailPaths(test.paths), test._expectedTail)
		})
	}
}

func TestFleetProblems(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedSummary []string
		_expectedDetails int

		document *heartbeat.Document
	}

	failed := []heartbeat.Repo{
		{Name: "smine", ScanError: "agentwt: Too many open files (os error 24)", Warnings: []string{"fork unreadable"}},
		{Name: "copy_trader", ScanError: "agentwt: commit-graph"},
	}
	degraded := heartbeat.Sensor{Name: sensorAgentwt, Since: "2026-09-28T19:15:10Z", State: heartbeat.SensorStateDegraded}

	tests := make([]*testCase, 0)

	// agentwt-degraded-two-repos
	tests = append(tests, &testCase{
		_id:              "agentwt-degraded-two-repos",
		_expectedSummary: []string{"agentwt degraded since 19:15:10Z — 2 repo(s) failed to scan, git rows are from the last good scan"},
		_expectedDetails: 3,

		document: &heartbeat.Document{Repos: failed, Sensors: []heartbeat.Sensor{degraded}},
	})

	// agentwt-degraded-repo-filtered-out
	tests = append(tests, &testCase{
		_id:              "agentwt-degraded-repo-filtered-out",
		_expectedSummary: []string{"agentwt degraded since 19:15:10Z"},
		_expectedDetails: 1,

		document: &heartbeat.Document{Sensors: []heartbeat.Sensor{degraded}},
	})

	// failed-repo-without-sensor
	tests = append(tests, &testCase{
		_id:              "failed-repo-without-sensor",
		_expectedSummary: []string{"repo copy_trader scan failed — git rows are from the last good scan"},
		_expectedDetails: 1,

		document: &heartbeat.Document{Repos: failed[1:]},
	})

	// none
	tests = append(tests, &testCase{
		_id:              "none",
		_expectedSummary: []string{},
		_expectedDetails: 0,

		document: &heartbeat.Document{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			problems := fleetProblems(test.document)
			summaries := make([]string, 0, len(problems))
			details := 0
			for _, problem := range problems {
				summaries = append(summaries, problem.Summary)
				details += len(problem.Details)
			}
			assert.Equal(t, test._expectedSummary, summaries)
			assert.Equal(t, test._expectedDetails, details)
		})
	}
}

func TestTrainSteps(t *testing.T) {
	fleet := &heartbeat.Fleet{Id: "T1", MergeBranch: "claude-orch/T1", Repo: "/r", Target: "feature/x"}
	members := []heartbeat.MemberRow{{Branch: "claude/member-b"}, {Branch: ""}}
	document := &heartbeat.Document{Trains: []heartbeat.Train{
		{Repo: "/r", Base: "main", Steps: []heartbeat.TrainStep{{Branch: "claude/member-b", Position: 1}}},
		{Repo: "/r", Base: "feature/x", Steps: []heartbeat.TrainStep{
			{Branch: "claude-routines/nightly", Position: 1},
			{Branch: "claude-orch/T1", Position: 2, State: heartbeat.TrainStepStateClean},
			{Branch: "claude/outsider", Position: 3},
			{Branch: "claude/member-b", Position: 4, State: heartbeat.TrainStepStateBlocked},
		}},
	}}

	steps := trainSteps(document, fleet, members)

	branches := make([]string, 0, len(steps))
	for _, step := range steps {
		branches = append(branches, step.Branch)
	}
	assert.Equal(t, []string{"claude-orch/T1", "claude/member-b"}, branches, "fleet branches of the target train, merge order kept")
	assert.Equal(t, heartbeat.ToneError, steps[1].State.Tone())
}

func TestHandleFleetMemberRemove(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedCode    int
		_expectedTrigger string
		_expectedBody    string

		stub string
	}

	tests := make([]*testCase, 0)

	// cli-success
	tests = append(tests, &testCase{
		_id:              "cli-success",
		_expectedCode:    http.StatusNoContent,
		_expectedTrigger: "fleet-op",
		_expectedBody:    "",

		stub: writeHeartbeatStub(t, 0, "{}"),
	})

	// cli-refusal
	tests = append(tests, &testCase{
		_id:              "cli-refusal",
		_expectedCode:    http.StatusConflict,
		_expectedTrigger: "",
		_expectedBody:    "no live member with that letter",

		stub: writeHeartbeatStub(t, 1, "heartbeat: 409 no live member with that letter\n"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			server, claudeHome := fleetTestServer(t, test.stub)
			response := serveFleet(server, httptest.NewRequest(http.MethodPost, "/fleet/T1/members/B/remove", nil))
			assert.Equal(t, test._expectedCode, response.Code, response.Body.String())
			assert.Equal(t, test._expectedTrigger, response.Header().Get("HX-Trigger"))
			assert.Contains(t, response.Body.String(), test._expectedBody)

			argv, err := os.ReadFile(filepath.Join(filepath.Dir(test.stub), "argv"))
			require.NoError(t, err)
			assert.Equal(t, "--addr 127.0.0.1:6101 --claude-home "+filepath.ToSlash(claudeHome)+" remove --fleet T1 --member B", string(argv))
		})
	}
}

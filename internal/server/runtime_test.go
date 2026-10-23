package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/heartbeat"
)

// runtimeFixture is the runtime-v1 document the decoder tests use.
func runtimeFixture(t *testing.T) string {
	t.Helper()
	fixture, err := os.ReadFile(filepath.Join("..", "heartbeat", "testdata", "runtime_document.json"))
	require.NoError(t, err)
	return string(fixture)
}

func TestRuntimeShellRendersPlaceholderOnly(t *testing.T) {
	server, _ := fleetTestServer(t, writeHeartbeatStub(t, 0, runtimeFixture(t)))

	response := serveFleet(server, httptest.NewRequest(http.MethodGet, "/runtime", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, "<title>Fleet — Runtime</title>")
	// The nav's Fleet entry exists only where the Heartbeat component can run (darwin); the page tabs render everywhere.
	hasHeartbeat := components.Available(components.Heartbeat, runtime.GOOS)
	assert.Equal(t, hasHeartbeat, strings.Contains(body, `<a href="/fleet" class="active">Fleet</a>`))
	assert.Contains(t, body, "<h1>Fleet</h1>")
	assert.Contains(t, body, `<a href="/fleet" >Fleet</a>`)
	assert.Contains(t, body, `<a href="/runtime" class="active">Runtime</a>`)
	assert.Contains(t, body, "Loading runtime status")
	assert.Contains(t, body, `hx-get="/runtime/status"`)
	assert.Contains(t, body, `hx-trigger="load"`)
	assert.NotContains(t, body, "<th")
}

func TestAnchorTrack(t *testing.T) {
	hits := []heartbeat.AnchorHit{
		{Id: "task_check", Seq: 1},
		{Id: "task_check", Seq: 2},
		{Id: "task_check", Seq: 5},
		{Id: "implementer_spawn", Seq: 6},
		{Id: "task_check", Seq: 7},
	}

	expected := []anchorStep{
		{Id: "task_check", Count: 3, LastSeq: 5, Seq: 1},
		{Id: "implementer_spawn", Count: 1, LastSeq: 6, Seq: 6},
		{Id: "task_check", Count: 1, LastSeq: 7, Seq: 7},
	}
	assert.Equal(t, expected, anchorTrack(hits))
	assert.Nil(t, anchorTrack(nil))
}

func TestRuntimeStatusFragment(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedBody   []string
		_unexpectedBody []string

		stub string
	}

	noSubscriber := `{"schema":1,"heartbeat":{"addr":"127.0.0.1:6101"},"subscriptions":[],"runs":[],"findings":[]}`
	testingRun := strings.Replace(runtimeFixture(t), `"title": "CC,QQ - Quick question"`, `"title": "CC,QQ - Quick question - [TESTING]"`, 1)

	tests := make([]*testCase, 0)

	// fixture-document
	tests = append(tests, &testCase{
		_id: "fixture-document",
		_expectedBody: []string{
			"<td>CC,Rt - Runtime watch</td>\n    <td>CC,Rr - Review runtime checker</td>",
			`<span class="badge">act</span>`,
			"d3 · sent · 2026-09-29T09:14:02Z",
			"<td>CC,Rr - Review runtime checker </td>",
			`<span class="badge">open</span>`,
			`<span class="badge badge-error">killed</span></td>`,
			`<span class="anchor-step" title="seq 4">stack</span><span class="anchor-step" title="seq 9">lanes</span>`,
			"<td>value_provenance ×8</td>",
			"<code>red base reported at 09:13:55Z</code>",
			`<code title="0199a1b2-c3d4-7e5f-8a6b-7c8d9e0f1a2c">0199a1b2-c3d4-7e5f-8a6b-7c8d9e0f1a2c</code>`,
			`<span class="badge badge-error">check_failed</span>`,
			`<span class="badge">judgment</span>`,
			"<td>SKILL-RAILROADREVIEW-STATION-009</td>",
			"<td>send · stop</td>",
			"<td>handed_over</td>",
			`hx-trigger="every 5s"`,
		},
		_unexpectedBody: []string{"No runtime watcher", "No runs since", "No findings.", `hx-trigger="load"`},

		stub: writeHeartbeatStub(t, 0, runtimeFixture(t)),
	})

	// run-state-badge
	tests = append(tests, &testCase{
		_id:             "run-state-badge",
		_expectedBody:   []string{`<td>CC,QQ - Quick question <span class="badge badge-action">TESTING</span></td>`},
		_unexpectedBody: []string{"[TESTING]"},

		stub: writeHeartbeatStub(t, 0, testingRun),
	})

	// no-subscriber
	tests = append(tests, &testCase{
		_id:             "no-subscriber",
		_expectedBody:   []string{"No runtime watcher — run /runtime session: &lt;title&gt; in another session.", "No runs since the subscription began.", "No findings."},
		_unexpectedBody: []string{"<th", "runtime state unavailable"},

		stub: writeHeartbeatStub(t, 0, noSubscriber),
	})

	// unreachable-exit-3
	tests = append(tests, &testCase{
		_id:             "unreachable-exit-3",
		_expectedBody:   []string{"runtime state unavailable: Status: Heartbeat unreachable", `hx-get="/runtime/status"`},
		_unexpectedBody: []string{"<h2>runs</h2>"},

		stub: writeHeartbeatStub(t, 3, "heartbeat: no daemon on 127.0.0.1:6101\n"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			server, claudeHome := fleetTestServer(t, test.stub)
			response := serveFleet(server, httptest.NewRequest(http.MethodGet, "/runtime/status", nil))
			body := response.Body.String()
			assert.Equal(t, http.StatusOK, response.Code)
			for _, want := range test._expectedBody {
				assert.Contains(t, body, want)
			}
			for _, unwanted := range test._unexpectedBody {
				assert.NotContains(t, body, unwanted)
			}

			argv, err := os.ReadFile(filepath.Join(filepath.Dir(test.stub), "argv"))
			require.NoError(t, err)
			assert.Equal(t, "--addr 127.0.0.1:6101 --claude-home "+filepath.ToSlash(claudeHome)+" runtime status --format json", string(argv))
		})
	}
}

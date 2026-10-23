package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/contextrecord"
)

func TestCountRuns(t *testing.T) {
	runs := newCountRuns()

	isFirstOwner := runs.begin("sid")
	isSecondOwner := runs.begin("sid")
	isOtherOwner := runs.begin("other")
	isRunningDuring := runs.isRunning("sid")
	runs.end("sid")
	isRunningAfter := runs.isRunning("sid")
	isOwnerAgain := runs.begin("sid")

	assert.True(t, isFirstOwner)
	assert.False(t, isSecondOwner)
	assert.True(t, isOtherOwner)
	assert.True(t, isRunningDuring)
	assert.False(t, isRunningAfter)
	assert.True(t, isOwnerAgain)
	assert.True(t, runs.isRunning("other"))
	assert.False(t, runs.isRunning("unknown"))
}

func TestCountCopy(t *testing.T) {
	queued := contextrecord.NewItem(contextrecord.KindHookContext, "sessiontitle", contextrecord.SourceHookContext, "hello")
	queued.Uncounted = contextrecord.UncountedQueued
	record := &contextrecord.Record{
		Agent: contextrecord.SessionClaude,
		Agents: []contextrecord.Agent{
			{Id: "main", Items: []contextrecord.Item{queued}, Model: "claude-opus-5-5"},
			{Id: "a1", Items: nil},
		},
	}

	clone := countCopy(record)
	clone.Agents[0].Items[0].SetTokens(contextrecord.CounterAnthropic, 9, false)
	clone.Agents[0].Model = "changed"

	assert.Equal(t, contextrecord.SessionClaude, clone.Agent)
	assert.Len(t, clone.Agents, 2)
	assert.Equal(t, "hello", clone.Agents[0].Items[0].Text())
	assert.Equal(t, 9, clone.Agents[0].Items[0].Tokens)
	assert.Equal(t, contextrecord.UncountedQueued, record.Agents[0].Items[0].Uncounted)
	assert.Equal(t, 0, record.Agents[0].Items[0].Tokens)
	assert.Equal(t, "claude-opus-5-5", record.Agents[0].Model)
}

func TestRecordPending(t *testing.T) {
	type testCase struct {
		_id       string
		_expected int

		record *contextrecord.Record
	}

	queued := contextrecord.Item{Uncounted: contextrecord.UncountedQueued}
	rate := contextrecord.Item{Uncounted: contextrecord.UncountedRate}
	skipped := contextrecord.Item{Uncounted: contextrecord.UncountedSize}
	counted := contextrecord.Item{Counter: contextrecord.CounterAnthropic, Tokens: 3}
	tests := make([]*testCase, 0)

	// no-agents
	tests = append(tests, &testCase{
		_id:       "no-agents",
		_expected: 0,

		record: &contextrecord.Record{},
	})

	// pending-over-two-agents
	tests = append(tests, &testCase{
		_id:       "pending-over-two-agents",
		_expected: 3,

		record: &contextrecord.Record{Agents: []contextrecord.Agent{
			{Id: "main", Items: []contextrecord.Item{queued, counted, skipped}},
			{Id: "a1", Items: []contextrecord.Item{rate, queued}},
		}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, recordPending(test.record))
		})
	}
}

func TestServer_StartCounting(t *testing.T) {
	server := newTestServer(t, &Options{})
	queued := contextrecord.NewItem(contextrecord.KindHookContext, "sessiontitle", contextrecord.SourceHookContext, "hello")
	queued.Uncounted = contextrecord.UncountedQueued
	record := &contextrecord.Record{
		Agent:  contextrecord.SessionClaude,
		Agents: []contextrecord.Agent{{Id: "main", Items: []contextrecord.Item{queued}}},
	}

	// A run in flight is not started twice.
	require.True(t, server.countRuns.begin("busy"))
	server.startCounting("busy", time.Time{}, record)
	assert.True(t, server.countRuns.isRunning("busy"))

	// A run releases its session when the count returns; with counting off
	// it returns at once, and the page's record stays untouched.
	server.startCounting("sid", time.Time{}, record)
	assert.Eventually(t, func() bool { return !server.countRuns.isRunning("sid") }, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, contextrecord.UncountedQueued, record.Agents[0].Items[0].Uncounted)
	assert.True(t, server.countRuns.isRunning("busy"))
}

func TestHandleSessionReportCounting(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		path string
	}

	server := newTestServer(t, &Options{})
	require.True(t, server.countRuns.begin("busy"))
	tests := make([]*testCase, 0)

	// run-in-flight
	tests = append(tests, &testCase{
		_id:       "run-in-flight",
		_expected: `{"counting":true}`,

		path: "/sessions-report/busy/counting",
	})

	// no-run
	tests = append(tests, &testCase{
		_id:       "no-run",
		_expected: `{"counting":false}`,

		path: "/sessions-report/idle/counting",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))

			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
			assert.JSONEq(t, test._expected, recorder.Body.String())
		})
	}
}

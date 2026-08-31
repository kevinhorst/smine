package peek

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newStubPeek serves the minimal MCP Streamable-HTTP surface the client
// touches: initialize, the initialized notification, and session_list.
func newStubPeek(t *testing.T, sessions []map[string]any, toolError bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
				"isError":           toolError,
				"structuredContent": map[string]any{"sessions": sessions},
			}
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		response := map[string]any{"jsonrpc": "2.0", "id": request.Id, "result": result}
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
}

func rfc3339(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func TestSessionIndexNewestPerCwdWins(t *testing.T) {
	now := time.Now()
	stub := newStubPeek(t, []map[string]any{
		{"id": "old", "meta": map[string]any{"cwd": "/repo"}, "title": "old", "last_active": rfc3339(now.Add(-time.Hour))},
		{"id": "new", "meta": map[string]any{"cwd": "/repo", "git_branch": "main", "model": "claude-opus-4-8"}, "title": "new", "last_active": rfc3339(now.Add(-time.Minute))},
		{"id": "nometa", "title": "meta-less", "last_active": rfc3339(now)},
		{"id": "emptymeta", "meta": map[string]any{}, "title": "no-cwd", "last_active": rfc3339(now)},
	}, false)
	defer stub.Close()

	index, err := NewClient(stub.URL).SessionIndex(t.Context())
	require.NoError(t, err)
	require.Len(t, index.ByCwd, 1)
	assert.Equal(t, "new", index.ByCwd["/repo"].Id)
}

func TestSessionIndexNewestPerBranchWins(t *testing.T) {
	now := time.Now()
	stub := newStubPeek(t, []map[string]any{
		{"id": "old", "meta": map[string]any{"cwd": "/pool-dir", "git_branch": "claude/feature"}, "title": "old", "last_active": rfc3339(now.Add(-time.Hour))},
		{"id": "new", "meta": map[string]any{"cwd": "/elsewhere", "git_branch": "claude/feature"}, "title": "new", "last_active": rfc3339(now.Add(-time.Minute))},
		{"id": "detached", "meta": map[string]any{"cwd": "/detached", "git_branch": "HEAD"}, "last_active": rfc3339(now)},
		{"id": "branchless", "meta": map[string]any{"cwd": "/branchless"}, "last_active": rfc3339(now)},
	}, false)
	defer stub.Close()

	index, err := NewClient(stub.URL).SessionIndex(t.Context())
	require.NoError(t, err)
	require.Len(t, index.ByBranch, 1, "HEAD and branch-less sessions never index by branch")
	assert.Equal(t, "new", index.ByBranch["claude/feature"].Id)
	assert.Equal(t, "claude/feature", index.ByBranch["claude/feature"].GitBranch)
}

func TestSessionIndexLiveWindowBoundary(t *testing.T) {
	now := time.Now()
	stub := newStubPeek(t, []map[string]any{
		{"id": "in", "meta": map[string]any{"cwd": "/in"}, "last_active": rfc3339(now.Add(-14 * time.Minute))},
		{"id": "out", "meta": map[string]any{"cwd": "/out"}, "last_active": rfc3339(now.Add(-16 * time.Minute))},
	}, false)
	defer stub.Close()

	index, err := NewClient(stub.URL).SessionIndex(t.Context())
	require.NoError(t, err)
	assert.True(t, index.ByCwd["/in"].Live)
	assert.False(t, index.ByCwd["/out"].Live)
}

func TestSessionsById(t *testing.T) {
	now := time.Now()
	stub := newStubPeek(t, []map[string]any{
		{"id": "live", "meta": map[string]any{"cwd": "/repo"}, "title": "live", "last_active": rfc3339(now.Add(-time.Minute))},
		{"id": "stale", "meta": map[string]any{"cwd": "/repo"}, "title": "stale", "last_active": rfc3339(now.Add(-16 * time.Minute))},
		{"id": "nometa", "title": "meta-less", "last_active": rfc3339(now)},
	}, false)
	defer stub.Close()

	sessions, err := NewClient(stub.URL).SessionsById(t.Context())
	require.NoError(t, err)
	require.Len(t, sessions, 3, "every session keyed by id, cwd-less ones included")
	assert.True(t, sessions["live"].Live)
	assert.False(t, sessions["stale"].Live)
	assert.True(t, sessions["nometa"].Live)
}

func TestSessionIndexUnreachableEndpoint(t *testing.T) {
	_, err := NewClient("http://127.0.0.1:1/mcp").SessionIndex(t.Context())
	require.Error(t, err)
}

func TestSessionIndexToolError(t *testing.T) {
	stub := newStubPeek(t, nil, true)
	defer stub.Close()

	_, err := NewClient(stub.URL).SessionIndex(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Tool returned an error")
}

func TestSessionIndexCleansPaths(t *testing.T) {
	now := time.Now()
	stub := newStubPeek(t, []map[string]any{
		{"id": "s", "meta": map[string]any{"cwd": "/repo/sub/.."}, "last_active": rfc3339(now)},
	}, false)
	defer stub.Close()

	index, err := NewClient(stub.URL).SessionIndex(t.Context())
	require.NoError(t, err)
	_, ok := index.ByCwd["/repo"]
	assert.True(t, ok, fmt.Sprintf("keys: %v", index.ByCwd))
}

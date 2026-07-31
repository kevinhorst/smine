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

func TestSessionsByCwdNewestPerCwdWins(t *testing.T) {
	now := time.Now()
	stub := newStubPeek(t, []map[string]any{
		{"id": "old", "meta": map[string]any{"cwd": "/repo"}, "title": "old", "last_active": rfc3339(now.Add(-time.Hour))},
		{"id": "new", "meta": map[string]any{"cwd": "/repo", "git_branch": "main", "model": "claude-opus-4-8"}, "title": "new", "last_active": rfc3339(now.Add(-time.Minute))},
		{"id": "nometa", "title": "meta-less", "last_active": rfc3339(now)},
		{"id": "emptymeta", "meta": map[string]any{}, "title": "no-cwd", "last_active": rfc3339(now)},
	}, false)
	defer stub.Close()

	sessions, err := NewClient(stub.URL).SessionsByCwd(t.Context())
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "new", sessions["/repo"].Id)
}

func TestSessionsByCwdLiveWindowBoundary(t *testing.T) {
	now := time.Now()
	stub := newStubPeek(t, []map[string]any{
		{"id": "in", "meta": map[string]any{"cwd": "/in"}, "last_active": rfc3339(now.Add(-14 * time.Minute))},
		{"id": "out", "meta": map[string]any{"cwd": "/out"}, "last_active": rfc3339(now.Add(-16 * time.Minute))},
	}, false)
	defer stub.Close()

	sessions, err := NewClient(stub.URL).SessionsByCwd(t.Context())
	require.NoError(t, err)
	assert.True(t, sessions["/in"].Live)
	assert.False(t, sessions["/out"].Live)
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

func TestSessionsByCwdUnreachableEndpoint(t *testing.T) {
	_, err := NewClient("http://127.0.0.1:1/mcp").SessionsByCwd(t.Context())
	require.Error(t, err)
}

func TestSessionsByCwdToolError(t *testing.T) {
	stub := newStubPeek(t, nil, true)
	defer stub.Close()

	_, err := NewClient(stub.URL).SessionsByCwd(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Tool returned an error")
}

func TestSessionsByCwdCleansPaths(t *testing.T) {
	now := time.Now()
	stub := newStubPeek(t, []map[string]any{
		{"id": "s", "meta": map[string]any{"cwd": "/repo/sub/.."}, "last_active": rfc3339(now)},
	}, false)
	defer stub.Close()

	sessions, err := NewClient(stub.URL).SessionsByCwd(t.Context())
	require.NoError(t, err)
	_, ok := sessions["/repo"]
	assert.True(t, ok, fmt.Sprintf("keys: %v", sessions))
}

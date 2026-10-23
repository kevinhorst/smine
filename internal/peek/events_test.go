package peek

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newStubPeekStructured serves the same minimal MCP surface as newStubPeek but
// returns an arbitrary structuredContent object, so session_events payloads
// can be shaped per case.
func newStubPeekStructured(t *testing.T, structured map[string]any, toolError bool) *httptest.Server {
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
				"serverInfo":      map[string]any{"name": "stub-peek", "version": "1.2.2"},
			}
		case "tools/call":
			result = map[string]any{
				"content":           []any{},
				"isError":           toolError,
				"structuredContent": structured,
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

// newStubPeekCapturing answers every tools/call with structured and toolError
// and copies the call's arguments into received.
func newStubPeekCapturing(t *testing.T, received, structured map[string]any, toolError bool) *httptest.Server {
	t.Helper()
	return newStubPeekFunc(t, func(arguments map[string]any) (map[string]any, bool) {
		maps.Copy(received, arguments)
		return structured, toolError
	})
}

// provideConn opens a connection to stub; the test closes both.
func provideConn(t *testing.T, stub *httptest.Server) *Conn {
	t.Helper()
	t.Cleanup(stub.Close)

	conn, err := NewClient(stub.URL).Connect(t.Context())
	require.NoError(t, err)

	t.Cleanup(func() { assert.NoError(t, conn.Close()) })
	return conn
}

// provideEventsResult decodes payload the way Conn.SessionEvents does.
func provideEventsResult(t *testing.T, payload map[string]any) *eventsResult {
	t.Helper()

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	parsed := &eventsResult{}
	require.NoError(t, json.Unmarshal(data, parsed))
	return parsed
}

func TestNewSessionEvents(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *SessionEvents

		parsed *eventsResult
	}

	startedAt := time.Date(2026, 9, 1, 18, 38, 38, 0, time.UTC)
	lastActive := time.Date(2026, 9, 1, 18, 42, 21, 0, time.UTC)
	claudeTime := map[string]any{
		"started_at":     "2026-09-01T18:38:38Z",
		"last_active":    "2026-09-01T18:42:21Z",
		"wall_seconds":   222,
		"idle_seconds":   5,
		"active_seconds": 217,
		"telemetry":      map[string]any{"cost_usd": 4.59},
	}
	claudeUsage := map[string]any{
		"input_tokens":                18,
		"output_tokens":               5144,
		"cache_read_input_tokens":     863857,
		"cache_creation_input_tokens": 79275,
	}
	claudeTokens := TokenUsage{
		CacheCreationInputTokens: 79275,
		CacheReadInputTokens:     863857,
		InputTokens:              18,
		OutputTokens:             5144,
	}

	tests := make([]*testCase, 0)

	// claude-usage-four-classes
	tests = append(tests, &testCase{
		_id: "claude-usage-four-classes",
		_expected: &SessionEvents{
			ActiveSeconds:        217,
			CostUsd:              4.59,
			HasPermissions:       true,
			HasTelemetry:         true,
			IdleSeconds:          5,
			InvokedSkills:        []SkillInvocation{{Skill: "fdesign", Source: "slash", Timestamp: startedAt}},
			LastActive:           lastActive,
			PermissionRejections: 2,
			StartedAt:            startedAt,
			TouchedFiles:         []TouchedFile{{Path: "/a.go", Reads: 2}, {Path: "/b.go", Reads: 1, Writes: 3}},
			Usage:                claudeTokens,
			WallSeconds:          222,
		},

		parsed: provideEventsResult(t, map[string]any{
			"permissions": map[string]any{
				"auto_allowed":    30,
				"hook_decided":    32,
				"prompted_once":   0,
				"prompted_always": 39,
				"rejected":        0,
				"requests": []any{
					map[string]any{"command": "git status --porcelain", "decision": "accept", "source": "hook", "timestamp": "2026-09-26T15:32:30.956Z", "tool": "Bash", "tool_use_id": "toolu_01BFaPyoqaGeSzaPDTj1ekup"},
					map[string]any{"decision": "reject", "source": "hook", "timestamp": "2026-09-26T15:32:41.762Z", "tool": "Read", "tool_use_id": "toolu_01BPC6fNka3T3eVQgx5zidxT"},
					map[string]any{"decision": "reject", "source": "hook", "timestamp": "2026-09-26T15:34:09.425Z", "tool": "Bash", "tool_use_id": "toolu_01L4agVGM6R8nBEC7np3ZAUA"},
				},
				"detail": "persisted",
			},
			"time":  claudeTime,
			"usage": claudeUsage,
			"events": []any{
				map[string]any{"kind": "skill_invoked", "skill": map[string]any{"skill": "fdesign", "source": "slash"}, "timestamp": "2026-09-01T18:38:38Z"},
				map[string]any{"kind": "plan_mode_enter", "timestamp": "2026-09-01T18:38:38Z"},
				map[string]any{"kind": "subagent_spawned", "timestamp": "2026-09-01T18:38:57Z"},
			},
			"touched_files": []any{
				map[string]any{"path": "/a.go", "reads": 2},
				map[string]any{"path": "/b.go", "reads": 1, "writes": 3},
			},
		}),
	})

	// codex-cached-input-moves-to-cache-read
	tests = append(tests, &testCase{
		_id: "codex-cached-input-moves-to-cache-read",
		_expected: &SessionEvents{
			ActiveSeconds: -3,
			LastActive:    lastActive,
			StartedAt:     startedAt,
			Usage: TokenUsage{
				CacheCreationInputTokens: 0,
				CacheReadInputTokens:     1000,
				InputTokens:              200,
				OutputTokens:             50,
			},
			WallSeconds: 222,
		},

		parsed: provideEventsResult(t, map[string]any{
			"time": map[string]any{
				"started_at":     "2026-09-01T18:38:38Z",
				"last_active":    "2026-09-01T18:42:21Z",
				"wall_seconds":   222,
				"active_seconds": -3,
			},
			"usage": map[string]any{
				"input_tokens":        1200,
				"cached_input_tokens": 1000,
				"output_tokens":       50,
			},
			"events": []any{},
		}),
	})

	// skills-and-subagents-decoded
	tests = append(tests, &testCase{
		_id: "skills-and-subagents-decoded",
		_expected: &SessionEvents{
			ActiveSeconds: 217,
			CostUsd:       4.59,
			HasTelemetry:  true,
			IdleSeconds:   5,
			LastActive:    lastActive,
			SkillRuns: []SkillRun{
				{
					EndedAt:   lastActive,
					Seconds:   223,
					Skill:     "fimplement",
					StartedAt: startedAt,
					Usage: TokenUsage{
						CacheCreationInputTokens: 4000,
						CacheReadInputTokens:     90000,
						InputTokens:              12,
						OutputTokens:             3000,
					},
				},
			},
			StartedAt: startedAt,
			Subagents: []SubagentUsage{
				{
					Id:      "a1b2c3d4e5f6a7b8c",
					Model:   "claude-opus-5-5",
					Seconds: 61,
					Type:    "Explore",
					Usage: TokenUsage{
						CacheCreationInputTokens: 20000,
						CacheReadInputTokens:     150000,
						InputTokens:              40,
						OutputTokens:             2500,
					},
				},
			},
			Usage:       claudeTokens,
			WallSeconds: 222,
		},

		parsed: provideEventsResult(t, map[string]any{
			"time":   claudeTime,
			"usage":  claudeUsage,
			"events": []any{},
			"skills": []any{
				map[string]any{
					"skill":      "fimplement",
					"started_at": "2026-09-01T18:38:38Z",
					"ended_at":   "2026-09-01T18:42:21Z",
					"seconds":    223,
					"usage": map[string]any{
						"input_tokens":                12,
						"output_tokens":               3000,
						"cache_read_input_tokens":     90000,
						"cache_creation_input_tokens": 4000,
					},
				},
			},
			"subagents": []any{
				map[string]any{
					"agent_id":   "a1b2c3d4e5f6a7b8c",
					"agent_type": "Explore",
					"model":      "claude-opus-5-5",
					"seconds":    61,
					"usage": map[string]any{
						"input_tokens":                40,
						"output_tokens":               2500,
						"cache_read_input_tokens":     150000,
						"cache_creation_input_tokens": 20000,
					},
				},
			},
		}),
	})

	// no-breakdown-fields
	tests = append(tests, &testCase{
		_id: "no-breakdown-fields",
		_expected: &SessionEvents{
			ActiveSeconds: 217,
			CostUsd:       4.59,
			HasTelemetry:  true,
			IdleSeconds:   5,
			LastActive:    lastActive,
			StartedAt:     startedAt,
			Usage:         claudeTokens,
			WallSeconds:   222,
		},

		parsed: provideEventsResult(t, map[string]any{
			"time":   claudeTime,
			"usage":  claudeUsage,
			"events": []any{},
		}),
	})

	// no-telemetry
	tests = append(tests, &testCase{
		_id: "no-telemetry",
		_expected: &SessionEvents{
			StartedAt:   startedAt,
			WallSeconds: 10,
		},

		parsed: provideEventsResult(t, map[string]any{
			"time": map[string]any{
				"started_at":   "2026-09-01T18:38:38Z",
				"wall_seconds": 10,
			},
			"events": []any{},
		}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, newSessionEvents(test.parsed))
		})
	}
}

func TestConn_SessionEvents(t *testing.T) {
	type testCase struct {
		_id                string
		_expected          *SessionEvents
		_expectedArguments map[string]any
		_shouldPass        bool

		conn     *Conn
		received map[string]any
	}

	expectedArguments := map[string]any{"breakdown": true, "id": "id", "json": true}

	tests := make([]*testCase, 0)

	// request-carries-breakdown-true
	breakdownReceived := make(map[string]any)
	breakdownStub := newStubPeekCapturing(
		t,
		breakdownReceived,
		map[string]any{"time": map[string]any{"wall_seconds": 10}},
		false,
	)
	tests = append(tests, &testCase{
		_id:                "request-carries-breakdown-true",
		_expected:          &SessionEvents{WallSeconds: 10},
		_expectedArguments: expectedArguments,
		_shouldPass:        true,

		conn:     provideConn(t, breakdownStub),
		received: breakdownReceived,
	})

	// tool-error
	toolErrorReceived := make(map[string]any)
	toolErrorStub := newStubPeekCapturing(t, toolErrorReceived, map[string]any{}, true)
	tests = append(tests, &testCase{
		_id:                "tool-error",
		_expected:          nil,
		_expectedArguments: expectedArguments,
		_shouldPass:        false,

		conn:     provideConn(t, toolErrorStub),
		received: toolErrorReceived,
	})

	// malformed-content
	malformedReceived := make(map[string]any)
	malformedStub := newStubPeekCapturing(
		t,
		malformedReceived,
		map[string]any{"usage": "not-an-object"},
		false,
	)
	tests = append(tests, &testCase{
		_id:                "malformed-content",
		_expected:          nil,
		_expectedArguments: expectedArguments,
		_shouldPass:        false,

		conn:     provideConn(t, malformedStub),
		received: malformedReceived,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			events, err := test.conn.SessionEvents(t.Context(), "id")
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, events)
			assert.Equal(t, test._expectedArguments, test.received)
		})
	}
}

func TestStdioArgs(t *testing.T) {
	home := filepath.Join("home", "user")
	expected := []string{
		"start",
		"--transport=stdio",
		"--depth=100",
		"--control-port=0",
		"--claude-home=" + filepath.Join(home, ".claude"),
		"--codex-home=" + filepath.Join(home, ".codex"),
		"--watch-window-days=14",
	}

	assert.Equal(t, expected, StdioArgs(home, MiningWindowDays))
}

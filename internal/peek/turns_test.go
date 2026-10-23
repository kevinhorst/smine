package peek

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionTurns(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   []Turn
		_shouldPass bool

		client *Client
	}

	tests := make([]*testCase, 0)

	// full-payload
	fullPayload := map[string]any{
		"turns": []any{
			map[string]any{"role": "user", "text": "# AGENTS.md instructions for /repo", "meta": map[string]any{"session_id": "x"}},
			map[string]any{"role": "assistant", "text": "On it.", "meta": map[string]any{"model": "gpt-6-astra"}},
		},
	}
	fullPayloadStub := newStubPeekStructured(t, fullPayload, false)
	t.Cleanup(fullPayloadStub.Close)
	tests = append(tests, &testCase{
		_id: "full-payload",
		_expected: []Turn{
			{Role: "user", Text: "# AGENTS.md instructions for /repo"},
			{Model: "gpt-6-astra", Role: "assistant", Text: "On it."},
		},
		_shouldPass: true,

		client: NewClient(fullPayloadStub.URL),
	})

	// tool-error
	toolErrorStub := newStubPeekStructured(t, map[string]any{}, true)
	t.Cleanup(toolErrorStub.Close)
	tests = append(tests, &testCase{
		_id:         "tool-error",
		_expected:   nil,
		_shouldPass: false,

		client: NewClient(toolErrorStub.URL),
	})

	// empty-turns
	emptyTurns := map[string]any{"turns": []any{}}
	emptyTurnsStub := newStubPeekStructured(t, emptyTurns, false)
	t.Cleanup(emptyTurnsStub.Close)
	tests = append(tests, &testCase{
		_id:         "empty-turns",
		_expected:   []Turn{},
		_shouldPass: true,

		client: NewClient(emptyTurnsStub.URL),
	})

	// non-json-structured-content
	mistyped := map[string]any{"turns": "not-a-list"}
	mistypedStub := newStubPeekStructured(t, mistyped, false)
	t.Cleanup(mistypedStub.Close)
	tests = append(tests, &testCase{
		_id:         "non-json-structured-content",
		_expected:   nil,
		_shouldPass: false,

		client: NewClient(mistypedStub.URL),
	})

	// unreachable-endpoint
	tests = append(tests, &testCase{
		_id:         "unreachable-endpoint",
		_expected:   nil,
		_shouldPass: false,

		client: NewClient("http://127.0.0.1:1/mcp"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			turns, err := test.client.SessionTurns(t.Context(), "codex", "x")
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, turns)
		})
	}
}

func TestSessionTurnsRequestArguments(t *testing.T) {
	var received map[string]any
	stub := newStubPeekFunc(t, func(arguments map[string]any) (map[string]any, bool) {
		received = arguments
		return map[string]any{"turns": []any{}}, false
	})
	defer stub.Close()

	_, err := NewClient(stub.URL).SessionTurns(t.Context(), "codex", "x")
	require.NoError(t, err)

	expected := map[string]any{
		"agent":  "codex",
		"diff":   false,
		"events": false,
		"id":     "x",
		"json":   true,
		"n":      float64(turnsPageSize),
		"plan":   false,
		"turns":  true,
	}
	assert.Equal(t, expected, received)
}

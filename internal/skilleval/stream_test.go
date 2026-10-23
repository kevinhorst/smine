package skilleval

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const tracePrefixRunes = 32

func initModel(stream *ArmStream) string {
	if stream.Init == nil {
		return ""
	}
	return stream.Init.Model
}

func initServers(stream *ArmStream) []McpServer {
	if stream.Init == nil {
		return nil
	}
	return stream.Init.McpServers
}

// tracePrefixes cuts each trace line to its head so real-capture lines are
// comparable without restating 240-rune snippets.
func tracePrefixes(trace []string) []string {
	prefixes := make([]string, 0, len(trace))
	for _, line := range trace {
		runes := []rune(line)
		if len(runes) > tracePrefixRunes {
			runes = runes[:tracePrefixRunes]
		}
		prefixes = append(prefixes, string(runes))
	}
	return prefixes
}

func TestParseStream_wallSeconds(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *int

		stream string
	}

	sixtyFive := 65
	ninety := 90

	tests := make([]*testCase, 0)

	// one-process-first-to-last
	tests = append(tests, &testCase{
		_id:       "one-process-first-to-last",
		_expected: &ninety,

		stream: `{"type":"assistant","message":{"content":[]},"timestamp":"2026-10-06T02:00:00Z"}` + "\n" +
			`{"type":"user","message":{"content":[]},"timestamp":"2026-10-06T02:01:00Z"}` + "\n" +
			`{"type":"assistant","message":{"content":[]},"timestamp":"2026-10-06T02:01:30Z"}` + "\n",
	})

	// resume-pause-not-counted
	tests = append(tests, &testCase{
		_id:       "resume-pause-not-counted",
		_expected: &sixtyFive,

		stream: `{"type":"assistant","message":{"content":[]},"timestamp":"2026-10-06T02:00:00Z"}` + "\n" +
			`{"type":"assistant","message":{"content":[]},"timestamp":"2026-10-06T02:00:40Z"}` + "\n" +
			`{"type":"skilleval","subtype":"resume","session_id":"s-1","at":"2026-10-06T09:20:00+02:00"}` + "\n" +
			`{"type":"assistant","message":{"content":[]},"timestamp":"2026-10-06T07:20:05Z"}` + "\n" +
			`{"type":"assistant","message":{"content":[]},"timestamp":"2026-10-06T07:20:30Z"}` + "\n",
	})

	// out-of-order-records
	tests = append(tests, &testCase{
		_id:       "out-of-order-records",
		_expected: &ninety,

		stream: `{"type":"assistant","message":{"content":[]},"timestamp":"2026-10-06T02:01:00Z"}` + "\n" +
			`{"type":"assistant","message":{"content":[]},"timestamp":"2026-10-06T02:00:00Z"}` + "\n" +
			`{"type":"assistant","message":{"content":[]},"timestamp":"2026-10-06T02:01:30Z"}` + "\n",
	})

	// unparsable-stamp-ignored
	tests = append(tests, &testCase{
		_id:       "unparsable-stamp-ignored",
		_expected: &ninety,

		stream: `{"type":"assistant","message":{"content":[]},"timestamp":"2026-10-06T02:00:00Z"}` + "\n" +
			`{"type":"assistant","message":{"content":[]},"timestamp":"yesterday"}` + "\n" +
			`{"type":"assistant","message":{"content":[]},"timestamp":"2026-10-06T02:01:30Z"}` + "\n",
	})

	// no-timestamp-nil
	tests = append(tests, &testCase{
		_id:       "no-timestamp-nil",
		_expected: nil,

		stream: `{"type":"result","subtype":"success","session_id":"s-1","total_cost_usd":1,"modelUsage":{},"permission_denials":[]}` + "\n",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			stream, err := ParseStream(strings.NewReader(test.stream))
			require.NoError(t, err)
			assert.Equal(t, test._expected, stream.WallSeconds)
		})
	}
}

func TestParseStream(t *testing.T) {
	type testCase struct {
		_id                   string
		_expectedCostUsd      *float64
		_expectedDenials      *int
		_expectedModel        string
		_expectedOutputTokens *int
		_expectedResult       *ResultRecord
		_expectedPycharm      string
		_expectedToolCalls    []ToolCount
		_expectedToolErrors   int
		_expectedTrace        []string
		_expectedUnparsed     int

		reader io.Reader
	}

	tests := make([]*testCase, 0)

	// real-capture
	fixture, err := os.Open("testdata/stream.jsonl")
	require.NoError(t, err)
	t.Cleanup(func() { _ = fixture.Close() })
	realCost := 150.25
	realDenials := 2
	realTokens := 1_670_000
	tests = append(tests, &testCase{
		_id:                   "real-capture",
		_expectedCostUsd:      &realCost,
		_expectedDenials:      &realDenials,
		_expectedModel:        "claude-sonnet-5[1m]",
		_expectedOutputTokens: &realTokens,
		_expectedResult: &ResultRecord{
			IsError: false,
			ModelUsage: map[string]ModelUsage{
				"claude-opus-5-5":     {OutputTokens: 20_000},
				"claude-sonnet-5[1m]": {OutputTokens: 1_650_000},
			},
			PermissionDenials: []PermissionDenial{
				{ToolUseId: "toolu_01ATYKP2Fvwidp4cJbeVtURC"},
				{ToolUseId: "toolu_02WriteDeniedOnce"},
			},
			Result:         "Review complete.",
			SessionId:      "ce98e20b-bd7e-4313-88c3-620398282a07",
			Subtype:        "success",
			TerminalReason: "completed",
			TotalCostUsd:   150.25,
		},
		_expectedPycharm: "failed",
		_expectedToolCalls: []ToolCount{
			{Calls: 1, Errors: 1, Name: "Bash"},
			{Calls: 1, Errors: 0, Name: "Read"},
		},
		_expectedToolErrors: 1,
		_expectedTrace: []string{
			`[root] Read {"file_path":"/Users`,
			`[agent m2gNDUBF] Bash {"command"`,
			`[agent m2gNDUBF] ERROR Bash: Req`,
		},
		_expectedUnparsed: 0,

		reader: fixture,
	})

	// unparsable-line-counted
	tests = append(tests, &testCase{
		_id:                   "unparsable-line-counted",
		_expectedCostUsd:      nil,
		_expectedDenials:      nil,
		_expectedModel:        "",
		_expectedOutputTokens: nil,
		_expectedResult:       nil,
		_expectedPycharm:      statusAbsent,
		_expectedToolCalls:    nil,
		_expectedToolErrors:   0,
		_expectedTrace:        []string{"[root] text: hello"},
		_expectedUnparsed:     1,

		reader: strings.NewReader("not json\n" + `{"type":"assistant","message":{"content":[{"type":"text","text":"hello"}]},"parent_tool_use_id":null}` + "\n"),
	})

	// string-content-user-message
	tests = append(tests, &testCase{
		_id:                   "string-content-user-message",
		_expectedCostUsd:      nil,
		_expectedDenials:      nil,
		_expectedModel:        "",
		_expectedOutputTokens: nil,
		_expectedResult:       nil,
		_expectedPycharm:      statusAbsent,
		_expectedToolCalls:    nil,
		_expectedToolErrors:   0,
		_expectedTrace:        []string{},
		_expectedUnparsed:     0,

		reader: strings.NewReader(`{"type":"user","message":{"role":"user","content":"review the branch"},"parent_tool_use_id":null}` + "\n"),
	})

	// model-usage-summed
	summedCost := 0.5
	summedDenials := 0
	summedTokens := 15
	tests = append(tests, &testCase{
		_id:                   "model-usage-summed",
		_expectedCostUsd:      &summedCost,
		_expectedDenials:      &summedDenials,
		_expectedModel:        "",
		_expectedOutputTokens: &summedTokens,
		_expectedResult: &ResultRecord{
			ModelUsage:        map[string]ModelUsage{"claude-opus-5-5": {OutputTokens: 10}, "claude-sonnet-5": {OutputTokens: 5}},
			PermissionDenials: []PermissionDenial{},
			SessionId:         "s-1",
			Subtype:           "success",
			TotalCostUsd:      0.5,
		},
		_expectedPycharm:    statusAbsent,
		_expectedToolCalls:  nil,
		_expectedToolErrors: 0,
		_expectedTrace:      []string{},
		_expectedUnparsed:   0,

		reader: strings.NewReader(`{"type":"result","subtype":"success","session_id":"s-1","total_cost_usd":0.5,"modelUsage":{"claude-opus-5-5":{"outputTokens":10},"claude-sonnet-5":{"outputTokens":5}},"permission_denials":[]}` + "\n"),
	})

	// denials-distinct-across-envelopes
	distinctCost := 2.0
	distinctDenials := 3
	distinctTokens := 40
	tests = append(tests, &testCase{
		_id:                   "denials-distinct-across-envelopes",
		_expectedCostUsd:      &distinctCost,
		_expectedDenials:      &distinctDenials,
		_expectedModel:        "",
		_expectedOutputTokens: &distinctTokens,
		_expectedResult: &ResultRecord{
			ModelUsage:        map[string]ModelUsage{"claude-opus-5-5": {OutputTokens: 40}},
			PermissionDenials: []PermissionDenial{{ToolUseId: "t2"}, {ToolUseId: "t3"}},
			SessionId:         "s-1",
			Subtype:           "success",
			TotalCostUsd:      2,
		},
		_expectedPycharm:    statusAbsent,
		_expectedToolCalls:  nil,
		_expectedToolErrors: 0,
		_expectedTrace:      []string{},
		_expectedUnparsed:   0,

		reader: strings.NewReader(
			`{"type":"result","subtype":"success","session_id":"s-1","total_cost_usd":1,"modelUsage":{"claude-opus-5-5":{"outputTokens":25}},"permission_denials":[{"tool_use_id":"t1"},{"tool_use_id":"t2"}]}` + "\n" +
				`{"type":"result","subtype":"success","session_id":"s-1","total_cost_usd":2,"modelUsage":{"claude-opus-5-5":{"outputTokens":40}},"permission_denials":[{"tool_use_id":"t2"},{"tool_use_id":"t3"}]}` + "\n",
		),
	})

	// resume-marker-sums-processes
	resumedCost := 3.5
	resumedDenials := 2
	resumedTokens := 15
	tests = append(tests, &testCase{
		_id:                   "resume-marker-sums-processes",
		_expectedCostUsd:      &resumedCost,
		_expectedDenials:      &resumedDenials,
		_expectedModel:        "",
		_expectedOutputTokens: &resumedTokens,
		_expectedResult: &ResultRecord{
			ModelUsage:        map[string]ModelUsage{"claude-opus-5-5": {OutputTokens: 10}},
			PermissionDenials: []PermissionDenial{{ToolUseId: "t1"}},
			SessionId:         "s-1",
			Subtype:           "success",
			TotalCostUsd:      1.5,
		},
		_expectedPycharm:    statusAbsent,
		_expectedToolCalls:  nil,
		_expectedToolErrors: 0,
		_expectedTrace:      []string{"[harness] resumed session s-1"},
		_expectedUnparsed:   0,

		reader: strings.NewReader(
			`{"type":"result","subtype":"success","is_error":true,"session_id":"s-1","total_cost_usd":2,"modelUsage":{"claude-opus-5-5":{"outputTokens":5}},"permission_denials":[{"tool_use_id":"t0"}]}` + "\n" +
				`{"type":"skilleval","subtype":"resume","session_id":"s-1","at":"2026-10-06T09:20:00+02:00"}` + "\n" +
				`{"type":"result","subtype":"success","session_id":"s-1","total_cost_usd":1.5,"modelUsage":{"claude-opus-5-5":{"outputTokens":10}},"permission_denials":[{"tool_use_id":"t1"}]}` + "\n",
		),
	})

	// string-message-not-unparsed
	tests = append(tests, &testCase{
		_id:                   "string-message-not-unparsed",
		_expectedCostUsd:      nil,
		_expectedDenials:      nil,
		_expectedModel:        "",
		_expectedOutputTokens: nil,
		_expectedResult:       nil,
		_expectedPycharm:      statusAbsent,
		_expectedToolCalls:    nil,
		_expectedToolErrors:   0,
		_expectedTrace:        []string{},
		_expectedUnparsed:     0,

		reader: strings.NewReader(`{"type":"system","subtype":"task_started","task_id":"wf-1","message":"Workflow round-1 started"}` + "\n"),
	})

	// no-envelope-nil
	tests = append(tests, &testCase{
		_id:                   "no-envelope-nil",
		_expectedCostUsd:      nil,
		_expectedDenials:      nil,
		_expectedModel:        "",
		_expectedOutputTokens: nil,
		_expectedResult:       nil,
		_expectedPycharm:      statusAbsent,
		_expectedToolCalls:    nil,
		_expectedToolErrors:   0,
		_expectedTrace:        []string{"[root] text: working"},
		_expectedUnparsed:     0,

		reader: strings.NewReader(`{"type":"assistant","message":{"content":[{"type":"text","text":"working"}]},"parent_tool_use_id":null}` + "\n"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			stream, err := ParseStream(test.reader)
			require.NoError(t, err)
			assert.Equal(t, test._expectedModel, initModel(stream))
			assert.Equal(t, test._expectedPycharm, serverStatus("pycharm", initServers(stream)))
			assert.Equal(t, test._expectedToolCalls, stream.ToolCalls)
			assert.Equal(t, test._expectedToolErrors, stream.ToolErrors)
			assert.Equal(t, test._expectedTrace, tracePrefixes(stream.Trace))
			assert.Equal(t, test._expectedResult, stream.Result)
			assert.Equal(t, test._expectedUnparsed, stream.UnparsedLines)
			assert.Equal(t, test._expectedOutputTokens, stream.OutputTokens)
			assert.Equal(t, test._expectedCostUsd, stream.CostUsd)
			assert.Equal(t, test._expectedDenials, stream.PermissionDenials)
		})
	}
}

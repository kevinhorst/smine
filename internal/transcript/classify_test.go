package transcript

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClassify pins the row shapes observed in real transcripts; every non-human kind must stay non-human.
func TestClassify(t *testing.T) {
	type testCase struct {
		_id       string
		_expected Kind

		raw string
	}

	tests := make([]*testCase, 0)

	// typed-prompt
	tests = append(tests, &testCase{
		_id:       "typed-prompt",
		_expected: KindHuman,

		raw: `{"type":"user","turnOrigin":"human","promptSource":"sdk","userType":"external","message":{"role":"user","content":"Something we need to make sure works"}}`,
	})

	// typed-command-prompt
	tests = append(tests, &testCase{
		_id:       "typed-command-prompt",
		_expected: KindHuman,

		raw: `{"type":"user","turnOrigin":"human","message":{"role":"user","content":"<command-message>fdesign</command-message>\n<command-name>/fdesign</command-name>\n<command-args>x</command-args>"}}`,
	})

	// mid-turn-typed
	tests = append(tests, &testCase{
		_id:       "mid-turn-typed",
		_expected: KindHumanQueued,

		raw: `{"type":"attachment","attachment":{"type":"queued_command","prompt":"Verdict: not bad","commandMode":"prompt","origin":{"kind":"human"},"humanTurn":true}}`,
	})

	// mid-turn-typed-older-harness
	tests = append(tests, &testCase{
		_id:       "mid-turn-typed-older-harness",
		_expected: KindHumanQueued,

		raw: `{"type":"attachment","attachment":{"type":"queued_command","prompt":"Use opus subagents","commandMode":"prompt","origin":{"kind":"human"}}}`,
	})

	// interrupted-marker
	tests = append(tests, &testCase{
		_id:       "interrupted-marker",
		_expected: KindInterrupted,

		raw: `{"type":"user","userType":"external","message":{"role":"user","content":[{"type":"text","text":"[Request interrupted by user]"}]}}`,
	})

	// interrupted-for-tool-use-marker
	tests = append(tests, &testCase{
		_id:       "interrupted-for-tool-use-marker",
		_expected: KindInterrupted,

		raw: `{"type":"user","message":{"role":"user","content":[{"type":"text","text":"[Request interrupted by user for tool use]"}]}}`,
	})

	// stop-hook-feedback
	tests = append(tests, &testCase{
		_id:       "stop-hook-feedback",
		_expected: KindHookFeedback,

		raw: `{"type":"user","isMeta":true,"userType":"external","message":{"role":"user","content":[{"type":"text","text":"Stop hook feedback:\nSession title: call mcp__ccd_session_mgmt__set_session_title with session_id \"self\" and title \"CC,FD - X - [READY]\"."}]}}`,
	})

	// skill-body
	tests = append(tests, &testCase{
		_id:       "skill-body",
		_expected: KindHarness,

		raw: `{"type":"user","isMeta":true,"message":{"role":"user","content":[{"type":"text","text":"Base directory for this skill: /Users/k/.claude/skills/fdesign\n\n# fdesign"}]}}`,
	})

	// local-command-caveat
	tests = append(tests, &testCase{
		_id:       "local-command-caveat",
		_expected: KindHarness,

		raw: `{"type":"user","isMeta":true,"message":{"role":"user","content":"<local-command-caveat>The command below was run directly in Claude Code.</local-command-caveat>"}}`,
	})

	// local-command-name
	tests = append(tests, &testCase{
		_id:       "local-command-name",
		_expected: KindLocalCommand,

		raw: `{"type":"user","message":{"role":"user","content":"<command-name>/model</command-name>\n            <command-message>model</command-message>\n            <command-args>claude-opus-5-5</command-args>"}}`,
	})

	// local-command-stdout
	tests = append(tests, &testCase{
		_id:       "local-command-stdout",
		_expected: KindLocalCommand,

		raw: "{\"type\":\"user\",\"message\":{\"role\":\"user\",\"content\":\"<local-command-stdout>Set model to `claude-opus-5-5`</local-command-stdout>\"}}",
	})

	// task-notification-stamped
	tests = append(tests, &testCase{
		_id:       "task-notification-stamped",
		_expected: KindTaskNotification,

		raw: `{"type":"user","turnOrigin":"task_notification","promptSource":"system","message":{"role":"user","content":"<task-notification>\n<task-id>a1</task-id>\n</task-notification>"}}`,
	})

	// task-notification-older-harness
	tests = append(tests, &testCase{
		_id:       "task-notification-older-harness",
		_expected: KindTaskNotification,

		raw: `{"type":"user","promptSource":"sdk","message":{"role":"user","content":"<task-notification>\n<task-id>a1</task-id>\n</task-notification>"}}`,
	})

	// peer-message
	tests = append(tests, &testCase{
		_id:       "peer-message",
		_expected: KindPeer,

		raw: `{"type":"user","turnOrigin":"peer","isMeta":true,"promptSource":"system","message":{"role":"user","content":"Another Claude session sent a message:\ngo on"}}`,
	})

	// headless-prompt
	tests = append(tests, &testCase{
		_id:       "headless-prompt",
		_expected: KindHeadless,

		raw: `{"type":"user","turnOrigin":"sdk","message":{"role":"user","content":"#allow Makefile"}}`,
	})

	// compaction-summary
	tests = append(tests, &testCase{
		_id:       "compaction-summary",
		_expected: KindCompaction,

		raw: `{"type":"user","isCompactSummary":true,"message":{"role":"user","content":"#allow Makefile"}}`,
	})

	// compaction-continuation-older-harness
	tests = append(tests, &testCase{
		_id:       "compaction-continuation-older-harness",
		_expected: KindCompaction,

		raw: `{"type":"user","message":{"role":"user","content":"This session is being continued from a previous conversation that ran out of context."}}`,
	})

	// tool-result
	tests = append(tests, &testCase{
		_id:       "tool-result",
		_expected: KindToolResult,

		raw: `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"#allow Makefile"}]}}`,
	})

	// hook-output-attachment
	tests = append(tests, &testCase{
		_id:       "hook-output-attachment",
		_expected: KindAttachment,

		raw: `{"type":"attachment","attachment":{"type":"hook_success","hookName":"UserPromptSubmit","stdout":"tool-policy #add: x"}}`,
	})

	// queued-task-notification
	tests = append(tests, &testCase{
		_id:       "queued-task-notification",
		_expected: KindAttachment,

		raw: `{"type":"attachment","attachment":{"type":"queued_command","prompt":"<task-notification>","commandMode":"task-notification","origin":{"kind":"task-notification","producer":"session-task"}}}`,
	})

	// queued-operator-steer
	tests = append(tests, &testCase{
		_id:       "queued-operator-steer",
		_expected: KindAttachment,

		raw: `{"type":"attachment","attachment":{"type":"queued_command","prompt":"OPERATOR STEER — mid-run instruction","commandMode":"prompt"}}`,
	})

	// assistant
	tests = append(tests, &testCase{
		_id:       "assistant",
		_expected: KindAssistant,

		raw: `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"#allow Makefile"}]}}`,
	})

	// unstamped-user-row
	tests = append(tests, &testCase{
		_id:       "unstamped-user-row",
		_expected: KindOther,

		raw: `{"type":"user","message":{"role":"user","content":"#allow Makefile"}}`,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			line := Line{}
			require.NoError(t, json.Unmarshal([]byte(test.raw), &line))
			assert.Equal(t, test._expected, Classify(&line))
		})
	}
}

func TestUnwrapPastedContents(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		text string
	}

	tests := make([]*testCase, 0)

	// no-wrapper
	tests = append(tests, &testCase{_id: "no-wrapper", _expected: "plain text", text: "plain text"})

	// whole-message
	tests = append(tests, &testCase{_id: "whole-message", _expected: "\n\n\n#set x\n\n", text: "\n\n<pasted_content id=\"1\">\n#set x\n</pasted_content id=\"1\">\n"})

	// beside-text
	tests = append(tests, &testCase{_id: "beside-text", _expected: "before\ninner\nafter", text: "before<pasted_content id=\"1\">inner</pasted_content id=\"1\">after"})

	// two-wrappers
	tests = append(tests, &testCase{_id: "two-wrappers", _expected: "a\none\nb\ntwo\nc", text: "a<pasted_content id=\"1\">one</pasted_content id=\"1\">b<pasted_content id=\"2\">two</pasted_content id=\"2\">c"})

	// close-without-id
	tests = append(tests, &testCase{_id: "close-without-id", _expected: "\ninner\n", text: "<pasted_content id=\"1\">inner</pasted_content>"})

	// unclosed-kept
	tests = append(tests, &testCase{_id: "unclosed-kept", _expected: "<pasted_content id=\"1\">\ninner", text: "<pasted_content id=\"1\">\ninner"})

	// open-tag-unfinished-kept
	tests = append(tests, &testCase{_id: "open-tag-unfinished-kept", _expected: "x <pasted_content id=\"1", text: "x <pasted_content id=\"1"})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, UnwrapPastedContents(test.text))
		})
	}
}

func TestStripPastedContents(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		text string
	}

	tests := make([]*testCase, 0)

	// no-wrapper
	tests = append(tests, &testCase{_id: "no-wrapper", _expected: "plain text", text: "plain text"})

	// whole-message-gone
	tests = append(tests, &testCase{_id: "whole-message-gone", _expected: "\n\n\n", text: "\n\n<pasted_content id=\"1\">\n#set x\n</pasted_content id=\"1\">\n"})

	// beside-text-keeps-typed
	tests = append(tests, &testCase{_id: "beside-text-keeps-typed", _expected: "beforeafter", text: "before<pasted_content id=\"1\">#allow -s</pasted_content id=\"1\">after"})

	// two-wrappers
	tests = append(tests, &testCase{_id: "two-wrappers", _expected: "abc", text: "a<pasted_content id=\"1\">one</pasted_content id=\"1\">b<pasted_content id=\"2\">two</pasted_content id=\"2\">c"})

	// unclosed-kept
	tests = append(tests, &testCase{_id: "unclosed-kept", _expected: "<pasted_content id=\"1\">\ninner", text: "<pasted_content id=\"1\">\ninner"})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, StripPastedContents(test.text))
		})
	}
}

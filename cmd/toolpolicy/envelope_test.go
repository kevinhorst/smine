package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeToolInput decodes one tool_input object the way run decodes the
// envelope holding it.
func decodeToolInput(raw string) (*ToolInput, error) {
	input := &ToolInput{}
	if err := json.Unmarshal([]byte(raw), input); err != nil {
		return input, fmt.Errorf("decodeToolInput: %w", err)
	}
	return input, nil
}

func TestToolArgs_UnmarshalJSON(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *ToolInput
		_shouldPass bool

		toolInput string
	}

	roundScript := "/Users/u/.claude/skills/railroad-review/workflows/round.js"

	tests := make([]*testCase, 0)

	// string-verbatim
	tests = append(tests, &testCase{
		_id:         "string-verbatim",
		_expected:   &ToolInput{Args: " plans/x.md errata: none ", Skill: "fimplement"},
		_shouldPass: true,

		toolInput: `{"skill":"fimplement","args":" plans/x.md errata: none "}`,
	})

	// object-compact
	tests = append(tests, &testCase{
		_id:         "object-compact",
		_expected:   &ToolInput{Args: `{"round":2,"station":"S1"}`, ScriptPath: roundScript},
		_shouldPass: true,

		toolInput: `{"scriptPath":"` + roundScript + `","args":{ "round": 2, "station": "S1" }}`,
	})

	// array-compact
	tests = append(tests, &testCase{
		_id:         "array-compact",
		_expected:   &ToolInput{Args: `[1,"a"]`},
		_shouldPass: true,

		toolInput: `{"args":[ 1, "a" ]}`,
	})

	// number-text
	tests = append(tests, &testCase{
		_id:         "number-text",
		_expected:   &ToolInput{Args: "42"},
		_shouldPass: true,

		toolInput: `{"args":42}`,
	})

	// bool-text
	tests = append(tests, &testCase{
		_id:         "bool-text",
		_expected:   &ToolInput{Args: "true"},
		_shouldPass: true,

		toolInput: `{"args":true}`,
	})

	// null-empty
	tests = append(tests, &testCase{
		_id:         "null-empty",
		_expected:   &ToolInput{},
		_shouldPass: true,

		toolInput: `{"args":null}`,
	})

	// absent-empty
	tests = append(tests, &testCase{
		_id:         "absent-empty",
		_expected:   &ToolInput{},
		_shouldPass: true,

		toolInput: `{}`,
	})

	// inline-script-fields
	tests = append(tests, &testCase{
		_id:         "inline-script-fields",
		_expected:   &ToolInput{Args: "{}", Script: "return 1"},
		_shouldPass: true,

		toolInput: `{"script":"return 1","args":{}}`,
	})

	// malformed-object-fails
	tests = append(tests, &testCase{
		_id:         "malformed-object-fails",
		_expected:   &ToolInput{},
		_shouldPass: false,

		toolInput: `{"args":{"round":}}`,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			input, err := decodeToolInput(test.toolInput)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, input)
		})
	}
}

func TestRun_WorkflowEnvelope(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedExit   int
		_expectedStderr string
		_expectedStdout string

		stdin string
	}

	hookConfigHome(t, renderedHookConfig(t))
	t.Setenv(headlessEnv, "")
	typedTranscript, err := filepath.Abs("testdata/transcript-relay-typed.jsonl")
	require.NoError(t, err)
	stageLaunch := func(argsMember string, transcriptPath string) string {
		return `{"session_id":"s1","hook_event_name":"PreToolUse","cwd":"/work","permission_mode":"acceptEdits",` +
			`"transcript_path":"` + transcriptPath + `","tool_name":"Workflow",` +
			`"tool_input":{"scriptPath":"/Users/u/.claude/skills/railroad-review/workflows/round.js"` + argsMember + `}}`
	}
	objectArgs := `,"args":{ "round": 2, "station": "S1" }`
	decided := `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"`
	relayDenied := `"permissionDecision":"deny","permissionDecisionReason":"tool-policy: denied Workflow — ` + causeWorkflowRelay

	tests := make([]*testCase, 0)

	// object-args-decided
	tests = append(tests, &testCase{
		_id:             "object-args-decided",
		_expectedExit:   0,
		_expectedStderr: "",
		_expectedStdout: decided,

		stdin: stageLaunch(objectArgs, ""),
	})

	// string-args-decided
	tests = append(tests, &testCase{
		_id:             "string-args-decided",
		_expectedExit:   0,
		_expectedStderr: "",
		_expectedStdout: decided,

		stdin: stageLaunch(`,"args":"round 2"`, ""),
	})

	// array-args-decided
	tests = append(tests, &testCase{
		_id:             "array-args-decided",
		_expectedExit:   0,
		_expectedStderr: "",
		_expectedStdout: decided,

		stdin: stageLaunch(`,"args":[1,2]`, ""),
	})

	// number-args-decided
	tests = append(tests, &testCase{
		_id:             "number-args-decided",
		_expectedExit:   0,
		_expectedStderr: "",
		_expectedStdout: decided,

		stdin: stageLaunch(`,"args":7`, ""),
	})

	// null-args-decided
	tests = append(tests, &testCase{
		_id:             "null-args-decided",
		_expectedExit:   0,
		_expectedStderr: "",
		_expectedStdout: decided,

		stdin: stageLaunch(`,"args":null`, ""),
	})

	// absent-args-decided
	tests = append(tests, &testCase{
		_id:             "absent-args-decided",
		_expectedExit:   0,
		_expectedStderr: "",
		_expectedStdout: decided,

		stdin: stageLaunch("", ""),
	})

	// object-args-typed-turn-judged
	tests = append(tests, &testCase{
		_id:             "object-args-typed-turn-judged",
		_expectedExit:   0,
		_expectedStderr: "",
		_expectedStdout: relayDenied,

		stdin: stageLaunch(objectArgs, typedTranscript),
	})

	// malformed-args-exit-1
	tests = append(tests, &testCase{
		_id:             "malformed-args-exit-1",
		_expectedExit:   1,
		_expectedStderr: "run: Failed to decode hook input",
		_expectedStdout: "",

		stdin: stageLaunch(`,"args":{"round":}`, ""),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			exit := run([]string{}, strings.NewReader(test.stdin), stdout, stderr)
			assert.Equal(t, test._expectedExit, exit)
			assert.Contains(t, stdout.String(), test._expectedStdout)
			assert.Contains(t, stderr.String(), test._expectedStderr)
		})
	}
}

func TestFimplementArgs_DecodedEnvelope(t *testing.T) {
	type testCase struct {
		_id                 string
		_expectedArgs       string
		_expectedInvocation bool

		input *HookInput
	}

	decode := func(toolInput string) *HookInput {
		envelope := `{"hook_event_name":"PostToolUse","tool_name":"Skill","tool_input":` + toolInput + `}`
		input := &HookInput{}
		require.NoError(t, json.Unmarshal([]byte(envelope), input))
		return input
	}

	tests := make([]*testCase, 0)

	// string-args-reach-plan-hook
	tests = append(tests, &testCase{
		_id:                 "string-args-reach-plan-hook",
		_expectedArgs:       probePlan + " errata: none",
		_expectedInvocation: true,

		input: decode(`{"skill":"fimplement","args":" ` + probePlan + ` errata: none "}`),
	})

	// object-args-compact-text
	tests = append(tests, &testCase{
		_id:                 "object-args-compact-text",
		_expectedArgs:       `{"plan":"x.md"}`,
		_expectedInvocation: true,

		input: decode(`{"skill":"fimplement","args":{ "plan": "x.md" }}`),
	})

	// absent-args-empty
	tests = append(tests, &testCase{
		_id:                 "absent-args-empty",
		_expectedArgs:       "",
		_expectedInvocation: true,

		input: decode(`{"skill":"fimplement"}`),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			args, isInvocation := fimplementArgs(test.input, testFimplementSkill)
			assert.Equal(t, test._expectedArgs, args)
			assert.Equal(t, test._expectedInvocation, isInvocation)
		})
	}
}

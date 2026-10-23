package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/transcript"
)

// relayGrants loads the grants a call of input sees under home's settings.
func relayGrants(t *testing.T, home string, input *HookInput) *Grants {
	t.Helper()
	grants, err := LoadGrants(input, defaultHookConfig(t), home, "", "", "")
	require.NoError(t, err)
	return grants
}

func TestRelayCause(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		grants *Grants
		input  *HookInput
	}

	typed := &Grants{HasHumanTurn: true, LastTurnOrigin: transcript.OriginHuman}
	notified := &Grants{HasHumanTurn: true, LastTurnOrigin: transcript.OriginTaskNotification}
	headless := &Grants{HasHumanTurn: false, LastTurnOrigin: "sdk"}
	afterTyped := func(origin string) *Grants {
		grants := &Grants{HasHumanTurn: true, LastTurnOrigin: origin}
		return grants
	}
	workflow := func(input ToolInput) *HookInput {
		call := &HookInput{ToolInput: input, ToolName: toolNameWorkflow}
		return call
	}
	stage := ToolInput{Args: `{"round":2,"station":"S1"}`, ScriptPath: "/x/workflows/round.js"}
	reformatted := "  export const meta = { name: 'relay-primer',\r\n\tdescription: 'SMINE-PRIMER — opens a notification turn' }\n\n" +
		"await new Promise(r =>  setTimeout(r, 10000))\n  return 'primed'\n"
	spacesRemoved := "export const meta={name:'relay-primer',description:'SMINE-PRIMER — opens a notification turn'}\n" +
		"await new Promise(r=>setTimeout(r,10000))\nreturn 'primed'"
	oneCharChanged := strings.Replace(relayPrimerScript, "10000", "10001", 1)
	extraStatement := relayPrimerScript + "\nawait agent('x', {})"

	tests := make([]*testCase, 0)

	// not-workflow-tool
	tests = append(tests, &testCase{
		_id:       "not-workflow-tool",
		_expected: "",

		grants: typed,
		input:  &HookInput{ToolInput: ToolInput{Command: "git status"}, ToolName: toolNameBash},
	})

	// headless-not-guarded
	tests = append(tests, &testCase{
		_id:       "headless-not-guarded",
		_expected: "",

		grants: headless,
		input:  workflow(stage),
	})

	// notification-turn-admitted
	tests = append(tests, &testCase{
		_id:       "notification-turn-admitted",
		_expected: "",

		grants: notified,
		input:  workflow(stage),
	})

	// typed-turn-stage-denied
	tests = append(tests, &testCase{
		_id:       "typed-turn-stage-denied",
		_expected: causeWorkflowRelay,

		grants: typed,
		input:  workflow(stage),
	})

	// typed-turn-inline-denied
	tests = append(tests, &testCase{
		_id:       "typed-turn-inline-denied",
		_expected: causeWorkflowRelay,

		grants: typed,
		input:  workflow(ToolInput{Script: "return 1"}),
	})

	// peer-after-typed-denied
	tests = append(tests, &testCase{
		_id:       "peer-after-typed-denied",
		_expected: causeWorkflowRelay,

		grants: afterTyped("peer"),
		input:  workflow(stage),
	})

	// sdk-after-typed-denied
	tests = append(tests, &testCase{
		_id:       "sdk-after-typed-denied",
		_expected: causeWorkflowRelay,

		grants: afterTyped("sdk"),
		input:  workflow(stage),
	})

	// scheduled-after-typed-denied
	tests = append(tests, &testCase{
		_id:       "scheduled-after-typed-denied",
		_expected: causeWorkflowRelay,

		grants: afterTyped("scheduled"),
		input:  workflow(stage),
	})

	// system-after-typed-denied
	tests = append(tests, &testCase{
		_id:       "system-after-typed-denied",
		_expected: causeWorkflowRelay,

		grants: afterTyped("system"),
		input:  workflow(stage),
	})

	// unknown-origin-denied
	tests = append(tests, &testCase{
		_id:       "unknown-origin-denied",
		_expected: causeWorkflowRelay,

		grants: afterTyped("future_origin"),
		input:  workflow(stage),
	})

	// typed-turn-without-turn-record-denied
	tests = append(tests, &testCase{
		_id:       "typed-turn-without-turn-record-denied",
		_expected: causeWorkflowRelay,

		grants: afterTyped(""),
		input:  workflow(stage),
	})

	// primer-exact-admitted
	tests = append(tests, &testCase{
		_id:       "primer-exact-admitted",
		_expected: "",

		grants: typed,
		input:  workflow(ToolInput{Script: relayPrimerScript}),
	})

	// primer-reformatted-admitted
	tests = append(tests, &testCase{
		_id:       "primer-reformatted-admitted",
		_expected: "",

		grants: typed,
		input:  workflow(ToolInput{Script: reformatted}),
	})

	// primer-spaces-removed-marker-denied
	tests = append(tests, &testCase{
		_id:       "primer-spaces-removed-marker-denied",
		_expected: causeWorkflowMarker,

		grants: typed,
		input:  workflow(ToolInput{Script: spacesRemoved}),
	})

	// primer-one-char-changed-marker-denied
	tests = append(tests, &testCase{
		_id:       "primer-one-char-changed-marker-denied",
		_expected: causeWorkflowMarker,

		grants: typed,
		input:  workflow(ToolInput{Script: oneCharChanged}),
	})

	// primer-extra-statement-marker-denied
	tests = append(tests, &testCase{
		_id:       "primer-extra-statement-marker-denied",
		_expected: causeWorkflowMarker,

		grants: typed,
		input:  workflow(ToolInput{Script: extraStatement}),
	})

	// marker-script-denied-in-notification-turn
	tests = append(tests, &testCase{
		_id:       "marker-script-denied-in-notification-turn",
		_expected: causeWorkflowMarker,

		grants: notified,
		input:  workflow(ToolInput{Script: extraStatement}),
	})

	// marker-script-denied-headless
	tests = append(tests, &testCase{
		_id:       "marker-script-denied-headless",
		_expected: causeWorkflowMarker,

		grants: headless,
		input:  workflow(ToolInput{Script: extraStatement}),
	})

	// primer-via-script-path-guarded
	tests = append(tests, &testCase{
		_id:       "primer-via-script-path-guarded",
		_expected: causeWorkflowRelay,

		grants: typed,
		input:  workflow(ToolInput{ScriptPath: "/x/relay-primer.js"}),
	})

	// primer-in-args-guarded
	tests = append(tests, &testCase{
		_id:       "primer-in-args-guarded",
		_expected: causeWorkflowRelay,

		grants: typed,
		input:  workflow(ToolInput{Args: ToolArgs(relayPrimerScript)}),
	})

	// marker-in-args-no-marker-hit
	tests = append(tests, &testCase{
		_id:       "marker-in-args-no-marker-hit",
		_expected: "",

		grants: notified,
		input:  workflow(ToolInput{Args: ToolArgs(extraStatement), ScriptPath: "/x/workflows/round.js"}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, relayCause(&defaultTexts.Causes, test.input, test.grants))
		})
	}
}

func TestRelayVerdict(t *testing.T) {
	verdict := relayVerdict(defaultToolpolicy(t), causeWorkflowRelay)

	assert.Equal(t, decisionDeny, verdict.Decision)
	assert.Equal(t, causeWorkflowRelay, verdict.Cause)
	assert.Empty(t, verdict.CandidateRule, "no allow rule lifts the relay guard")
	assert.NotContains(t, verdict.Reason, "candidate rule")
	assert.True(t, strings.HasSuffix(verdict.Reason, relayPrimerScript))
	assert.Contains(t, verdict.Reason, "end the turn")
	assert.Contains(t, verdict.Reason, "restarts at step 1")
}

func TestDecide_WorkflowRelay(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedCandidate string
		_expectedDecision  string
		_expectedReason    string

		grants *Grants
		input  *HookInput
	}

	policy := defaultToolpolicy(t)
	home := t.TempDir()
	writeAllowLayer(t, home, toolNameWorkflow)
	cwd := t.TempDir()
	call := func(fixture string, input ToolInput) *HookInput {
		envelope := &HookInput{
			Cwd:            cwd,
			HookEventName:  hookEventPreToolUse,
			PermissionMode: "default",
			SessionId:      "s1",
			ToolInput:      input,
			ToolName:       toolNameWorkflow,
			TranscriptPath: filepath.Join("testdata", fixture),
		}
		return envelope
	}
	stage := ToolInput{Args: `{"round":2,"station":"S1"}`, ScriptPath: "/x/workflows/round.js"}
	inline := ToolInput{Script: "return 1"}
	primer := ToolInput{Script: relayPrimerScript}
	reformatted := ToolInput{Script: strings.ReplaceAll(relayPrimerScript, "\n", "\r\n\t  ")}
	marked := ToolInput{Script: relayPrimerScript + "\nawait agent('x', {})"}
	allowed := settingsRuleReason + toolNameWorkflow
	relayDenied := causeWorkflowRelay + ". " + relaySteps + relayPrimerScript

	tests := make([]*testCase, 0)

	// typed-prompt-newest-denied-with-primer
	input := call("transcript-relay-typed.jsonl", inline)
	tests = append(tests, &testCase{
		_id:                "typed-prompt-newest-denied-with-primer",
		_expectedCandidate: "",
		_expectedDecision:  decisionDeny,
		_expectedReason:    relayDenied,

		grants: relayGrants(t, home, input),
		input:  input,
	})

	// script-path-guarded-like-inline
	input = call("transcript-relay-typed.jsonl", stage)
	tests = append(tests, &testCase{
		_id:                "script-path-guarded-like-inline",
		_expectedCandidate: "",
		_expectedDecision:  decisionDeny,
		_expectedReason:    relayDenied,

		grants: relayGrants(t, home, input),
		input:  input,
	})

	// notification-row-newest-allowed
	input = call("transcript-relay-notification.jsonl", stage)
	tests = append(tests, &testCase{
		_id:                "notification-row-newest-allowed",
		_expectedCandidate: "",
		_expectedDecision:  decisionAllow,
		_expectedReason:    allowed,

		grants: relayGrants(t, home, input),
		input:  input,
	})

	// typed-queued-after-notification-denied
	input = call("transcript-relay-queued-typed.jsonl", stage)
	tests = append(tests, &testCase{
		_id:                "typed-queued-after-notification-denied",
		_expectedCandidate: "",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeWorkflowRelay,

		grants: relayGrants(t, home, input),
		input:  input,
	})

	// typed-queued-before-notification-allowed
	input = call("transcript-relay-queued-before-notification.jsonl", stage)
	tests = append(tests, &testCase{
		_id:                "typed-queued-before-notification-allowed",
		_expectedCandidate: "",
		_expectedDecision:  decisionAllow,
		_expectedReason:    allowed,

		grants: relayGrants(t, home, input),
		input:  input,
	})

	// notification-attachment-after-typed-denied
	input = call("transcript-relay-notification-attachment.jsonl", stage)
	tests = append(tests, &testCase{
		_id:                "notification-attachment-after-typed-denied",
		_expectedCandidate: "",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeWorkflowRelay,

		grants: relayGrants(t, home, input),
		input:  input,
	})

	// peer-after-notification-denied
	input = call("transcript-relay-peer.jsonl", stage)
	tests = append(tests, &testCase{
		_id:                "peer-after-notification-denied",
		_expectedCandidate: "",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeWorkflowRelay,

		grants: relayGrants(t, home, input),
		input:  input,
	})

	// sdk-after-typed-denied
	input = call("transcript-relay-sdk-after-human.jsonl", stage)
	tests = append(tests, &testCase{
		_id:                "sdk-after-typed-denied",
		_expectedCandidate: "",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeWorkflowRelay,

		grants: relayGrants(t, home, input),
		input:  input,
	})

	// malformed-line-skipped
	input = call("transcript-relay-malformed.jsonl", stage)
	tests = append(tests, &testCase{
		_id:                "malformed-line-skipped",
		_expectedCandidate: "",
		_expectedDecision:  decisionAllow,
		_expectedReason:    allowed,

		grants: relayGrants(t, home, input),
		input:  input,
	})

	// headless-sdk-not-guarded
	input = call("transcript-headless.jsonl", stage)
	tests = append(tests, &testCase{
		_id:                "headless-sdk-not-guarded",
		_expectedCandidate: "",
		_expectedDecision:  decisionAllow,
		_expectedReason:    allowed,

		grants: relayGrants(t, home, input),
		input:  input,
	})

	// peer-only-not-guarded
	input = call("transcript-design-peer.jsonl", stage)
	tests = append(tests, &testCase{
		_id:                "peer-only-not-guarded",
		_expectedCandidate: "",
		_expectedDecision:  decisionAllow,
		_expectedReason:    allowed,

		grants: relayGrants(t, home, input),
		input:  input,
	})

	// missing-transcript-not-guarded
	input = call("does-not-exist.jsonl", stage)
	tests = append(tests, &testCase{
		_id:                "missing-transcript-not-guarded",
		_expectedCandidate: "",
		_expectedDecision:  decisionAllow,
		_expectedReason:    allowed,

		grants: relayGrants(t, home, input),
		input:  input,
	})

	// primer-exact-allowed
	input = call("transcript-relay-typed.jsonl", primer)
	tests = append(tests, &testCase{
		_id:                "primer-exact-allowed",
		_expectedCandidate: "",
		_expectedDecision:  decisionAllow,
		_expectedReason:    allowed,

		grants: relayGrants(t, home, input),
		input:  input,
	})

	// primer-other-whitespace-allowed
	input = call("transcript-relay-typed.jsonl", reformatted)
	tests = append(tests, &testCase{
		_id:                "primer-other-whitespace-allowed",
		_expectedCandidate: "",
		_expectedDecision:  decisionAllow,
		_expectedReason:    allowed,

		grants: relayGrants(t, home, input),
		input:  input,
	})

	// marker-script-not-primer-denied
	input = call("transcript-relay-notification.jsonl", marked)
	tests = append(tests, &testCase{
		_id:                "marker-script-not-primer-denied",
		_expectedCandidate: "",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeWorkflowMarker,

		grants: relayGrants(t, home, input),
		input:  input,
	})

	// marker-script-denied-headless
	input = call("transcript-headless.jsonl", marked)
	tests = append(tests, &testCase{
		_id:                "marker-script-denied-headless",
		_expectedCandidate: "",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeWorkflowMarker,

		grants: relayGrants(t, home, input),
		input:  input,
	})

	// deny-rule-binds-first
	input = call("transcript-relay-typed.jsonl", primer)
	denyGrants := relayGrants(t, home, input)
	denyGrants.Deny = append(denyGrants.Deny, parseRules([]string{toolNameWorkflow}, "fixture")...)
	tests = append(tests, &testCase{
		_id:                "deny-rule-binds-first",
		_expectedCandidate: toolNameWorkflow,
		_expectedDecision:  decisionDeny,
		_expectedReason:    "matches deny rule Workflow",

		grants: denyGrants,
		input:  input,
	})

	// plan-mode-enforce-binds-before-guard
	input = call("transcript-relay-typed.jsonl", inline)
	input.PermissionMode = permissionModePlan
	enforceGrants := relayGrants(t, home, input)
	planRule := &GateRules{PlanModeDenied: []string{toolNameWorkflow}, Skill: "railroad-review"}
	enforceGrants.Enforce = append(enforceGrants.Enforce, planRule)
	tests = append(tests, &testCase{
		_id:                "plan-mode-enforce-binds-before-guard",
		_expectedCandidate: "",
		_expectedDecision:  decisionDeny,
		_expectedReason:    "enforce(railroad-review): Workflow is not allowed in plan mode",

		grants: enforceGrants,
		input:  input,
	})

	// plan-mode-enforce-applies-to-primer
	input = call("transcript-relay-typed.jsonl", primer)
	input.PermissionMode = permissionModePlan
	primerEnforceGrants := relayGrants(t, home, input)
	primerEnforceGrants.Enforce = append(primerEnforceGrants.Enforce, planRule)
	tests = append(tests, &testCase{
		_id:                "plan-mode-enforce-applies-to-primer",
		_expectedCandidate: "",
		_expectedDecision:  decisionDeny,
		_expectedReason:    "enforce(railroad-review): Workflow is not allowed in plan mode",

		grants: primerEnforceGrants,
		input:  input,
	})

	// primer-without-allow-rule-denied
	input = call("transcript-relay-typed.jsonl", primer)
	bareGrants := relayGrants(t, home, input)
	bareGrants.Allow = nil
	tests = append(tests, &testCase{
		_id:                "primer-without-allow-rule-denied",
		_expectedCandidate: toolNameWorkflow,
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeNoAllowRule,

		grants: bareGrants,
		input:  input,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			verdict := Decide(test.input, policy, test.grants, home, "")
			assert.Equal(t, test._expectedDecision, verdict.Decision)
			assert.Equal(t, test._expectedCandidate, verdict.CandidateRule)
			assert.Contains(t, verdict.Reason, test._expectedReason)
		})
	}
}

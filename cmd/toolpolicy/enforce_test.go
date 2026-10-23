package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// demoGate mirrors the heartbeat-demo gate block.
func demoGate() *GateRules {
	return &GateRules{
		PlanModeDenied: []string{"Workflow"},
		RootDenied:     []string{"*/heartbeat-demo/*/closed.json"},
		Skill:          "heartbeat-demo",
		WorkflowOnly:   []string{"heartbeat-demo-worker"},
		Writers:        []Writer{{AgentTypes: []string{"heartbeat-demo-worker"}, Glob: "*/heartbeat-demo/*/artifact.json"}},
	}
}

func TestEnforceCause(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		input *HookInput
		rules []*GateRules
	}

	artifact := "/tmp/runs/heartbeat-demo/r1/artifact.json"
	tests := make([]*testCase, 0)

	// plan-mode-workflow
	tests = append(tests, &testCase{
		_id:       "plan-mode-workflow",
		_expected: "enforce(heartbeat-demo): Workflow is not allowed in plan mode",

		input: &HookInput{PermissionMode: "plan", ToolName: "Workflow"},
		rules: []*GateRules{demoGate()},
	})

	// hand-spawn
	tests = append(tests, &testCase{
		_id:       "hand-spawn",
		_expected: "enforce(heartbeat-demo): heartbeat-demo-worker agents are spawned only by the skill's Workflow",

		input: &HookInput{ToolInput: ToolInput{SubagentType: "heartbeat-demo-worker"}, ToolName: "Agent"},
		rules: []*GateRules{demoGate()},
	})

	// root-denied
	tests = append(tests, &testCase{
		_id:       "root-denied",
		_expected: "enforce(heartbeat-demo): /tmp/runs/heartbeat-demo/r1/closed.json is written only by the skill's tooling, never by the dispatcher",

		input: &HookInput{ToolInput: ToolInput{FilePath: "/tmp/runs/heartbeat-demo/r1/closed.json"}, ToolName: "Write"},
		rules: []*GateRules{demoGate()},
	})

	// owner-writes
	tests = append(tests, &testCase{
		_id:       "owner-writes",
		_expected: "",

		input: &HookInput{AgentId: "a1", AgentType: "heartbeat-demo-worker", ToolInput: ToolInput{FilePath: artifact}, ToolName: "Write"},
		rules: []*GateRules{demoGate()},
	})

	// foreign-writes
	tests = append(tests, &testCase{
		_id:       "foreign-writes",
		_expected: "enforce(heartbeat-demo): /tmp/runs/heartbeat-demo/r1/artifact.json is written only by [heartbeat-demo-worker]",

		input: &HookInput{ToolInput: ToolInput{FilePath: artifact}, ToolName: "Edit"},
		rules: []*GateRules{demoGate()},
	})

	// skill-not-invoked
	tests = append(tests, &testCase{
		_id:       "skill-not-invoked",
		_expected: "",

		input: &HookInput{ToolInput: ToolInput{FilePath: artifact}, ToolName: "Write"},
		rules: nil,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			cause := enforceCause(test.input, test.rules)

			assert.Equal(t, test._expected, cause)
		})
	}
}

func TestEnforceVerdict(t *testing.T) {
	verdict := enforceVerdict(toolNameAgent, "enforce(heartbeat-demo): x")

	assert.Equal(t, decisionDeny, verdict.Decision)
	assert.Empty(t, verdict.CandidateRule, "no allow rule lifts an enforce deny")
	assert.NotContains(t, verdict.Reason, "candidate rule")
}

func TestLoadGateRules(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedNil     bool
		_expectedDenyAll bool

		content string
	}

	tests := make([]*testCase, 0)

	// gate-block
	tests = append(tests, &testCase{
		_id:              "gate-block",
		_expectedNil:     false,
		_expectedDenyAll: false,

		content: `{"gate": {"workflowOnly": ["w"]}}`,
	})

	// no-gate-block
	tests = append(tests, &testCase{
		_id:              "no-gate-block",
		_expectedNil:     true,
		_expectedDenyAll: false,

		content: `{"anchors": {}}`,
	})

	// malformed
	tests = append(tests, &testCase{
		_id:              "malformed",
		_expectedNil:     false,
		_expectedDenyAll: true,

		content: `{"gate": `,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			home := t.TempDir()
			dir := filepath.Join(home, ".claude", "skills", "demo")
			require.NoError(t, os.MkdirAll(dir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(dir, enforceFile), []byte(test.content), 0o600))

			rules := loadGateRules(home, "demo")

			assert.Equal(t, test._expectedNil, rules == nil)
			isDenyAll := rules != nil && enforceCause(&HookInput{AgentType: "w", ToolInput: ToolInput{FilePath: "/any"}, ToolName: "Write"}, []*GateRules{rules}) != ""
			assert.Equal(t, test._expectedDenyAll, isDenyAll)
		})
	}
}

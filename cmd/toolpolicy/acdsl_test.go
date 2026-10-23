package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// acdslRepo creates a repo dir carrying acdsl/policy.json with the given
// content ("" writes none) and returns its root.
func acdslRepo(t *testing.T, policyJson string) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "acdsl"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "sub"), 0o755))
	if policyJson != "" {
		require.NoError(t, os.WriteFile(filepath.Join(root, "acdsl", "policy.json"), []byte(policyJson), 0o644))
	}
	return root
}

func acdslSession(cwd string, tool string, input ToolInput) *HookInput {
	envelope := &HookInput{
		Cwd:            cwd,
		PermissionMode: "default",
		SessionId:      "session-1",
		ToolInput:      input,
		ToolName:       tool,
	}
	return envelope
}

func TestDecideAcdslGuard(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedDecision string
		_expectedReason   string

		input *HookInput
	}

	policy := defaultToolpolicy(t)

	home := filepath.Join(t.TempDir(), "home")
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0o755))
	gated := `{"mode": "gated", "editable_scopes": ["GOLANG"]}`
	strict := `{"mode": "strict"}`
	tests := make([]*testCase, 0)

	// gated-write-policy-deny
	root := acdslRepo(t, gated)
	tests = append(tests, &testCase{
		_id:               "gated-write-policy-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeAcdslSurface,

		input: acdslSession(root, "Write", ToolInput{FilePath: filepath.Join(root, "acdsl", "policy.json")}),
	})

	// gated-edit-registry-local-deny
	root = acdslRepo(t, gated)
	tests = append(tests, &testCase{
		_id:               "gated-edit-registry-local-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeAcdslSurface,

		input: acdslSession(root, "Edit", ToolInput{FilePath: filepath.Join(root, "acdsl", "registry.local.json")}),
	})

	// gated-bash-cp-registry-deny
	root = acdslRepo(t, gated)
	tests = append(tests, &testCase{
		_id:               "gated-bash-cp-registry-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeAcdslSurface,

		input: acdslSession(root, "Bash", ToolInput{Command: "cp build/x acdsl/registry.json"}),
	})

	// gated-bash-mv-evalgen-deny
	root = acdslRepo(t, gated)
	tests = append(tests, &testCase{
		_id:               "gated-bash-mv-evalgen-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeAcdslSurface,

		input: acdslSession(root, "Bash", ToolInput{Command: "mv build/x acdsl/evalgen.json"}),
	})

	// gated-bash-redirect-policy-deny
	root = acdslRepo(t, gated)
	tests = append(tests, &testCase{
		_id:               "gated-bash-redirect-policy-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeAcdslSurface,

		input: acdslSession(root, "Bash", ToolInput{Command: "go env > acdsl/policy.json"}),
	})

	// gated-bash-go-build-bin-acdsl-deny
	root = acdslRepo(t, gated)
	tests = append(tests, &testCase{
		_id:               "gated-bash-go-build-bin-acdsl-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeAcdslSurface,

		input: acdslSession(root, "Bash", ToolInput{Command: "go build -o bin/acdsl ./cmd/acdsl"}),
	})

	// gated-edit-rules-acdsl-allow
	root = acdslRepo(t, gated)
	tests = append(tests, &testCase{
		_id:               "gated-edit-rules-acdsl-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   rootsAllowReason,

		input: acdslSession(root, "Edit", ToolInput{FilePath: filepath.Join(root, "acdsl", "rules.acdsl")}),
	})

	// gated-subdir-cwd-deny
	root = acdslRepo(t, gated)
	tests = append(tests, &testCase{
		_id:               "gated-subdir-cwd-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeAcdslSurface,

		input: acdslSession(filepath.Join(root, "sub"), "Write", ToolInput{FilePath: filepath.Join(root, "acdsl", "policy.json")}),
	})

	// strict-edit-rules-acdsl-deny
	root = acdslRepo(t, strict)
	tests = append(tests, &testCase{
		_id:               "strict-edit-rules-acdsl-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeAcdslRuleFile,

		input: acdslSession(root, "Edit", ToolInput{FilePath: filepath.Join(root, "acdsl", "rules.acdsl")}),
	})

	// strict-task-contract-plans-allow
	root = acdslRepo(t, strict)
	tests = append(tests, &testCase{
		_id:               "strict-task-contract-plans-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   rootsAllowReason,

		input: acdslSession(root, "Write", ToolInput{FilePath: filepath.Join(root, "plans", "x", "task.acdsl")}),
	})

	// strict-task-contracts-off-plans-deny
	root = acdslRepo(t, `{"mode": "strict", "allow_task_contracts": false}`)
	tests = append(tests, &testCase{
		_id:               "strict-task-contracts-off-plans-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeAcdslRuleFile,

		input: acdslSession(root, "Write", ToolInput{FilePath: filepath.Join(root, "plans", "x", "task.acdsl")}),
	})

	// free-write-policy-allow
	root = acdslRepo(t, `{"mode": "free"}`)
	tests = append(tests, &testCase{
		_id:               "free-write-policy-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   rootsAllowReason,

		input: acdslSession(root, "Write", ToolInput{FilePath: filepath.Join(root, "acdsl", "policy.json")}),
	})

	// no-policy-write-allow
	root = acdslRepo(t, "")
	tests = append(tests, &testCase{
		_id:               "no-policy-write-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   rootsAllowReason,

		input: acdslSession(root, "Write", ToolInput{FilePath: filepath.Join(root, "acdsl", "policy.json")}),
	})

	// unparseable-policy-write-allow
	root = acdslRepo(t, `{"mode": `)
	tests = append(tests, &testCase{
		_id:               "unparseable-policy-write-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   rootsAllowReason,

		input: acdslSession(root, "Write", ToolInput{FilePath: filepath.Join(root, "acdsl", "policy.json")}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			verdict := Decide(test.input, policy, fixtureGrants(t, test.input.Cwd), home, "")
			assert.Equal(t, test._expectedDecision, verdict.Decision)
			assert.Contains(t, verdict.Reason, test._expectedReason)
		})
	}
}

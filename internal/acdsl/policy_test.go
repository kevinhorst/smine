package acdsl

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kevinhorst/smine/internal/shell"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const policyBaseGolangRule = `//acdsl:ACDSL-GOLANG-100 gofmt anchor="\.go$" why="base golang rule"`

const policyBasePlanRule = `//acdsl:ACDSL-PLAN-100 gofmt anchor="^plans/" why="base plan rule"`

// gatedBasePolicy is the canonical committed base policy: GOLANG editable,
// verifiers from the base registry (gofmt only).
const gatedBasePolicy = `{"mode": "gated", "editable_scopes": ["GOLANG"]}`

// gatedPolicy is the working-copy policy handed to CheckPolicy — since the
// base policy is authoritative it only bootstraps base resolution.
func gatedPolicy() Policy {
	policy := Policy{
		AllowTaskContracts: true,
		EditableScopes:     []string{"GOLANG"},
		Mode:               PolicyModeGated,
	}
	return policy
}

// newPolicyRepo builds a git repo with a committed base on main (registry,
// the given policy document, two rules) and a work branch checked out — the
// boundary CheckPolicy diffs across.
func newPolicyRepo(t *testing.T, basePolicyJson string) string {
	t.Helper()
	root := t.TempDir()
	policyGit(t, root, "init", "-q", "-b", "main")
	policyGit(t, root, "config", "user.email", "test@example.com")
	policyGit(t, root, "config", "user.name", "test")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "acdsl"), 0o755))
	writePolicyFile(t, root, "acdsl/registry.json", `{"gofmt": {"argv": ["true"], "timeout_s": 10, "description": "base"}}`+"\n")
	writePolicyFile(t, root, "acdsl/policy.json", basePolicyJson+"\n")
	writePolicyFile(t, root, "acdsl/rules.acdsl", policyBaseGolangRule+"\n"+policyBasePlanRule+"\n")
	policyGit(t, root, "add", ".")
	policyGit(t, root, "commit", "-q", "-m", "base")
	policyGit(t, root, "checkout", "-q", "-b", "work")
	return root
}

func policyGit(t *testing.T, root string, args ...string) {
	t.Helper()
	_, err := shell.Run(context.Background(), root, "git", args...)
	require.NoError(t, err)
}

func writePolicyFile(t *testing.T, root, rel, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte(content), 0o644))
}

func policyMessages(diagnostics []Diagnostic) string {
	var lines []string
	for _, diagnostic := range diagnostics {
		lines = append(lines, diagnostic.Message)
	}
	return strings.Join(lines, "\n")
}

func TestLoadPolicy(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   PolicyMode
		_shouldPass bool
		_taskExempt bool

		absent  bool
		content string
	}

	tests := make([]*testCase, 0)

	// absent-file-free
	tests = append(tests, &testCase{
		_id:         "absent-file-free",
		_expected:   PolicyModeFree,
		_shouldPass: true,
		_taskExempt: true,

		absent: true,
	})

	// defaults-preserved
	tests = append(tests, &testCase{
		_id:         "defaults-preserved",
		_expected:   PolicyModeGated,
		_shouldPass: true,
		_taskExempt: true,

		content: `{"mode": "gated"}`,
	})

	// task-contracts-disable-honored
	tests = append(tests, &testCase{
		_id:         "task-contracts-disable-honored",
		_expected:   PolicyModeStrict,
		_shouldPass: true,

		content: `{"mode": "strict", "allow_task_contracts": false}`,
	})

	// unknown-mode
	tests = append(tests, &testCase{
		_id: "unknown-mode",

		content: `{"mode": "loose"}`,
	})

	// malformed-json
	tests = append(tests, &testCase{
		_id: "malformed-json",

		content: `{"mode": `,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			root := t.TempDir()
			if !test.absent {
				writePolicyFile(t, root, "acdsl/policy.json", test.content)
			}
			policy, err := LoadPolicy(root)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			if test._shouldPass {
				assert.Equal(t, test._expected, policy.Mode)
				assert.Equal(t, test._taskExempt, policy.AllowTaskContracts)
			}
		})
	}
}

func TestCheckPolicyFreeNeverDiffs(t *testing.T) {
	root := newPolicyRepo(t, `{"mode": "free"}`)
	writePolicyFile(t, root, "acdsl/registry.json", `{"evil": {"argv": ["true"], "timeout_s": 10, "description": "x"}}`)
	policy := Policy{AllowTaskContracts: true, Mode: PolicyModeFree}

	diagnostics, err := CheckPolicy(context.Background(), root, policy)

	require.NoError(t, err)
	assert.Empty(t, diagnostics)
}

func TestCheckPolicyOnBaseTipVacuous(t *testing.T) {
	root := newPolicyRepo(t, gatedBasePolicy)
	policyGit(t, root, "checkout", "-q", "main")
	writePolicyFile(t, root, "acdsl/rules.acdsl", policyBaseGolangRule+"\n")

	diagnostics, err := CheckPolicy(context.Background(), root, gatedPolicy())

	require.NoError(t, err)
	assert.Empty(t, diagnostics)
}

func TestCheckPolicyStrictFlagsRuleAndSurface(t *testing.T) {
	root := newPolicyRepo(t, `{"mode": "strict"}`)
	writePolicyFile(t, root, "acdsl/rules.acdsl", policyBaseGolangRule+"\n"+policyBasePlanRule+"\n"+`//acdsl:ACDSL-GOLANG-101 gofmt anchor="\.go$" why="agent addition"`+"\n")
	writePolicyFile(t, root, "acdsl/registry.json", `{"evil": {"argv": ["true"], "timeout_s": 10, "description": "x"}}`)
	policy := Policy{AllowTaskContracts: true, Mode: PolicyModeStrict}

	diagnostics, err := CheckPolicy(context.Background(), root, policy)

	require.NoError(t, err)
	messages := policyMessages(diagnostics)
	assert.Contains(t, messages, "acdsl/registry.json differs from the policy base")
	assert.Contains(t, messages, "rule ACDSL-GOLANG-101 added")
	assert.Contains(t, messages, "no rule is editable")
}

func TestCheckPolicyBaseModeSurvivesWorkingWeakening(t *testing.T) {
	root := newPolicyRepo(t, gatedBasePolicy)
	// the agent deletes the working policy and adds a protected rule — the
	// base policy still gates, and the deletion is itself a surface finding
	require.NoError(t, os.Remove(filepath.Join(root, "acdsl", "policy.json")))
	writePolicyFile(t, root, "acdsl/rules.acdsl", policyBaseGolangRule+"\n"+policyBasePlanRule+"\n"+`//acdsl:ACDSL-PLAN-101 gofmt anchor="^plans/" why="agent plan addition"`+"\n")
	free := Policy{AllowTaskContracts: true, Mode: PolicyModeFree}

	diagnostics, err := CheckPolicy(context.Background(), root, free)

	require.NoError(t, err)
	messages := policyMessages(diagnostics)
	assert.Contains(t, messages, "acdsl/policy.json differs from the policy base")
	assert.Contains(t, messages, "rule ACDSL-PLAN-101 added outside the editable scopes")
}

func TestCheckPolicyGatedScopes(t *testing.T) {
	root := newPolicyRepo(t, gatedBasePolicy)
	// editable GOLANG add + protected PLAN add, and the base PLAN rule removed
	writePolicyFile(t, root, "acdsl/rules.acdsl", policyBaseGolangRule+"\n"+`//acdsl:ACDSL-GOLANG-101 gofmt anchor="\.go$" why="agent addition"`+"\n"+`//acdsl:ACDSL-PLAN-101 gofmt anchor="^plans/" why="agent plan addition"`+"\n")

	diagnostics, err := CheckPolicy(context.Background(), root, gatedPolicy())

	require.NoError(t, err)
	messages := policyMessages(diagnostics)
	assert.NotContains(t, messages, "ACDSL-GOLANG-101")
	assert.Contains(t, messages, "rule ACDSL-PLAN-101 added outside the editable scopes")
	assert.Contains(t, messages, "rule ACDSL-PLAN-100 removed outside the editable scopes")
	assert.Len(t, diagnostics, 2)
}

func TestCheckPolicyGatedChangedMarkerLine(t *testing.T) {
	root := newPolicyRepo(t, gatedBasePolicy)
	changedPlan := `//acdsl:ACDSL-PLAN-100 gofmt anchor="^plans/" empty="ok" why="base plan rule"`
	writePolicyFile(t, root, "acdsl/rules.acdsl", policyBaseGolangRule+"\n"+changedPlan+"\n")

	diagnostics, err := CheckPolicy(context.Background(), root, gatedPolicy())

	require.NoError(t, err)
	assert.Contains(t, policyMessages(diagnostics), "rule ACDSL-PLAN-100 modified outside the editable scopes")
	assert.Len(t, diagnostics, 1)
}

func TestCheckPolicyGatedVerifierAllowlist(t *testing.T) {
	rogueRule := `//acdsl:ACDSL-GOLANG-101 rogue anchor="\.go$" why="binds unregistered verifier"`

	// default allowlist = base registry names (gofmt only)
	root := newPolicyRepo(t, gatedBasePolicy)
	writePolicyFile(t, root, "acdsl/rules.acdsl", policyBaseGolangRule+"\n"+policyBasePlanRule+"\n"+rogueRule+"\n")
	diagnostics, err := CheckPolicy(context.Background(), root, gatedPolicy())
	require.NoError(t, err)
	assert.Contains(t, policyMessages(diagnostics), `rule ACDSL-GOLANG-101 binds verifier "rogue" outside the sanctioned set`)

	// an explicit allowlist in the base policy sanctions it
	root = newPolicyRepo(t, `{"mode": "gated", "editable_scopes": ["GOLANG"], "verifier_allowlist": ["gofmt", "rogue"]}`)
	writePolicyFile(t, root, "acdsl/rules.acdsl", policyBaseGolangRule+"\n"+policyBasePlanRule+"\n"+rogueRule+"\n")
	diagnostics, err = CheckPolicy(context.Background(), root, gatedPolicy())
	require.NoError(t, err)
	assert.Empty(t, diagnostics)
}

func TestCheckPolicyOverlay(t *testing.T) {
	overlay := `{"gofmt": {"argv": ["true"], "timeout_s": 10, "description": "override"}}`

	root := newPolicyRepo(t, gatedBasePolicy)
	writePolicyFile(t, root, "acdsl/registry.local.json", overlay)
	diagnostics, err := CheckPolicy(context.Background(), root, gatedPolicy())
	require.NoError(t, err)
	assert.Contains(t, policyMessages(diagnostics), `overlay entry "gofmt" is not sanctioned in local_overrides`)

	// base-policy local_overrides sanctions the entry
	root = newPolicyRepo(t, `{"mode": "gated", "editable_scopes": ["GOLANG"], "local_overrides": ["gofmt"]}`)
	writePolicyFile(t, root, "acdsl/registry.local.json", overlay)
	diagnostics, err = CheckPolicy(context.Background(), root, gatedPolicy())
	require.NoError(t, err)
	assert.Empty(t, diagnostics)
}

func TestCheckPolicyTaskContractExempt(t *testing.T) {
	taskRule := `//acdsl:TASK-1 gofmt anchor="\.go$" lifetime="task" why="task contract"`

	root := newPolicyRepo(t, `{"mode": "strict"}`)
	writePolicyFile(t, root, "plans/feature/task.acdsl", taskRule+"\n")
	diagnostics, err := CheckPolicy(context.Background(), root, gatedPolicy())
	require.NoError(t, err)
	assert.Empty(t, diagnostics)

	// base policy disables the exemption
	root = newPolicyRepo(t, `{"mode": "strict", "allow_task_contracts": false}`)
	writePolicyFile(t, root, "plans/feature/task.acdsl", taskRule+"\n")
	diagnostics, err = CheckPolicy(context.Background(), root, gatedPolicy())
	require.NoError(t, err)
	assert.Contains(t, policyMessages(diagnostics), "rule TASK-1 added")
}

func TestCheckPolicyUnresolvableBase(t *testing.T) {
	root := newPolicyRepo(t, gatedBasePolicy)
	policy := gatedPolicy()
	policy.BaseRef = "nosuchref"

	_, err := CheckPolicy(context.Background(), root, policy)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no policy base resolves")
}

func TestCheckPolicyNewUntrackedPolicyOnBranch(t *testing.T) {
	root := t.TempDir()
	policyGit(t, root, "init", "-q", "-b", "main")
	policyGit(t, root, "config", "user.email", "test@example.com")
	policyGit(t, root, "config", "user.name", "test")
	writePolicyFile(t, root, "main.go", "package main\n")
	policyGit(t, root, "add", ".")
	policyGit(t, root, "commit", "-q", "-m", "base without policy")
	policyGit(t, root, "checkout", "-q", "-b", "work")
	writePolicyFile(t, root, "acdsl/policy.json", `{"mode": "gated"}`)

	diagnostics, err := CheckPolicy(context.Background(), root, gatedPolicy())

	require.NoError(t, err)
	assert.Contains(t, policyMessages(diagnostics), "acdsl/policy.json differs from the policy base")
}

func TestRuleDelta(t *testing.T) {
	root := newPolicyRepo(t, gatedBasePolicy)
	// reorder the GOLANG rule's fields (changed), drop the PLAN rule
	// (removed), add one in an untracked file (added)
	writePolicyFile(t, root, "acdsl/rules.acdsl", `//acdsl:ACDSL-GOLANG-100 gofmt why="base golang rule" anchor="\.go$"`+"\n")
	writePolicyFile(t, root, "extra.acdsl", `//acdsl:ACDSL-SHELL-100 gofmt anchor="\.sh$" why="untracked addition"`+"\n")
	base, isBoundary, err := policyBase(context.Background(), root, gatedPolicy())
	require.NoError(t, err)
	require.True(t, isBoundary)

	delta, err := ruleDelta(context.Background(), root, base)

	require.NoError(t, err)
	require.Len(t, delta.added, 1)
	assert.Equal(t, "ACDSL-SHELL-100", delta.added[0].Id)
	require.Len(t, delta.changed, 1)
	assert.Equal(t, "ACDSL-GOLANG-100", delta.changed[0].Id)
	require.Len(t, delta.removed, 1)
	assert.Equal(t, "ACDSL-PLAN-100", delta.removed[0].Id)
}

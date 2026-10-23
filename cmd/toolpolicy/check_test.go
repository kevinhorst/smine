package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckReport(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   string
		_shouldPass bool

		args []string
	}

	policy := defaultToolpolicy(t)
	home := fixtureHome(t)
	base := t.TempDir()
	cwd := selfServiceCwd(t, base, "wt", "")
	outside := filepath.Join(t.TempDir(), "secret.txt")
	session := fixtureGrants(t, cwd)
	session.Approvals = map[string]bool{unlockMakefile: true, unlockSettings: true}
	session.HasHumanTurn = true
	session.SkillAllow = parseRules([]string{"Bash(cargo build *)"}, "fixture")
	input := &HookInput{Cwd: cwd, PermissionMode: permissionModeAcceptEdits}

	tests := make([]*testCase, 0)

	// admitted-settings-rule
	tests = append(tests, &testCase{
		_id:         "admitted-settings-rule",
		_expected:   "1. go test ./... → admitted",
		_shouldPass: true,

		args: []string{"go test ./..."},
	})

	// admitted-read-only
	tests = append(tests, &testCase{
		_id:         "admitted-read-only",
		_expected:   "1. ls → admitted",
		_shouldPass: true,

		args: []string{"ls"},
	})

	// admitted-by-named-skill
	tests = append(tests, &testCase{
		_id:         "admitted-by-named-skill",
		_expected:   "1. jq -n 1 → admitted",
		_shouldPass: true,

		args: []string{"--skill", "jq", "jq -n 1"},
	})

	// denied-self-serviceable
	tests = append(tests, &testCase{
		_id:         "denied-self-serviceable",
		_expected:   "1. lsof -i :6101 → denied: no allow rule matches; candidate rule Bash(lsof *) — self-serviceable: list it in the plan dir's permissions.json with this target",
		_shouldPass: true,

		args: []string{"lsof -i :6101"},
	})

	// denied-wrapper-candidate
	tests = append(tests, &testCase{
		_id:         "denied-wrapper-candidate",
		_expected:   "1. timeout 5 cargo build → denied: no allow rule matches; candidate rule Bash(timeout *) — not self-serviceable (isSelfServiceRule: Program \"timeout\" is a wrapper — the rule would admit every command): reshape it, or propose it for the global template",
		_shouldPass: true,

		args: []string{"timeout 5 cargo build"},
	})

	// denied-compound-reshape
	tests = append(tests, &testCase{
		_id:         "denied-compound-reshape",
		_expected:   "1. cd sub && cargo test → denied: " + causeCompound + " — no allow rule lifts this; reshape it (split the call, use a make target, stay inside the session roots) and check again",
		_shouldPass: true,

		args: []string{"cd sub && cargo test"},
	})

	// denied-fence-reshape
	tests = append(tests, &testCase{
		_id:         "denied-fence-reshape",
		_expected:   "1. cat " + outside + " → denied: " + causeReadOutside + " — no allow rule lifts this; reshape it (split the call, use a make target, stay inside the session roots) and check again",
		_shouldPass: true,

		args: []string{"cat " + outside},
	})

	// session-skill-grant-ignored
	tests = append(tests, &testCase{
		_id:         "session-skill-grant-ignored",
		_expected:   "1. cargo build x → denied: no allow rule matches; candidate rule Bash(cargo build *) — self-serviceable: list it in the plan dir's permissions.json with this target",
		_shouldPass: true,

		args: []string{"cargo build x"},
	})

	// allow-approval-ignored
	tests = append(tests, &testCase{
		_id:         "allow-approval-ignored",
		_expected:   "1. touch Makefile → denied: " + approvableCause(unlockMakefile) + " — no allow rule lifts this; reshape it (split the call, use a make target, stay inside the session roots) and check again",
		_shouldPass: true,

		args: []string{"touch Makefile"},
	})

	// webfetch-url
	tests = append(tests, &testCase{
		_id:         "webfetch-url",
		_expected:   "1. https://docs.rs/gix → denied: no allow rule matches; candidate rule WebFetch(domain:docs.rs) — self-serviceable: list it in the plan dir's permissions.json with this target\n2. https://code.claude.com/docs → admitted",
		_shouldPass: true,

		args: []string{"https://docs.rs/gix", "https://code.claude.com/docs"},
	})

	// mcp-tool-name
	tests = append(tests, &testCase{
		_id:         "mcp-tool-name",
		_expected:   "1. mcp__goland__search_symbol → admitted\n2. mcp__peek__session_list → denied: no allow rule matches; candidate rule mcp__peek__session_list — self-serviceable: list it in the plan dir's permissions.json with this target",
		_shouldPass: true,

		args: []string{"mcp__goland__search_symbol", "mcp__peek__session_list"},
	})

	// nested-toolpolicy-not-checkable
	tests = append(tests, &testCase{
		_id:         "nested-toolpolicy-not-checkable",
		_expected:   "1. toolpolicy allow 'Bash(x *)' → not checkable: toolpolicy commands are gate commands",
		_shouldPass: true,

		args: []string{"toolpolicy allow 'Bash(x *)'"},
	})

	// no-targets-usage
	tests = append(tests, &testCase{
		_id:         "no-targets-usage",
		_expected:   "",
		_shouldPass: false,

		args: []string{"--skill", "jq"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			report, err := checkReport(input, policy, session, home, test.args)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, report)
		})
	}
}

func TestDecideCheckCommand(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedCause   string
		_expectedReason  string
		_expectedReport  bool
		_expectedApplied bool

		grants *Grants
		input  *HookInput
	}

	policy := defaultToolpolicy(t)
	home := fixtureHome(t)
	cwd := selfServiceCwd(t, t.TempDir(), "wt", "")
	human := fixtureGrants(t, cwd)
	human.HasHumanTurn = true
	headless := fixtureGrants(t, cwd)

	tests := make([]*testCase, 0)

	// report-verdict
	tests = append(tests, &testCase{
		_id:              "report-verdict",
		_expectedCause:   causeCheckReport,
		_expectedReason:  causeCheckReport + " — this call is refused only so Claude Code does not run it. Do not retry.\n1. go test ./... → admitted",
		_expectedReport:  true,
		_expectedApplied: false,

		grants: human,
		input:  &HookInput{Cwd: cwd, PermissionMode: permissionModeAcceptEdits, ToolInput: ToolInput{Command: "toolpolicy check 'go test ./...'"}, ToolName: toolNameBash},
	})

	// headless-still-answers
	tests = append(tests, &testCase{
		_id:              "headless-still-answers",
		_expectedCause:   causeCheckReport,
		_expectedReason:  causeCheckReport + " — this call is refused only so Claude Code does not run it. Do not retry.\n1. go test ./... → admitted",
		_expectedReport:  true,
		_expectedApplied: false,

		grants: headless,
		input:  &HookInput{Cwd: cwd, PermissionMode: permissionModeAcceptEdits, ToolInput: ToolInput{Command: "toolpolicy check 'go test ./...'"}, ToolName: toolNameBash},
	})

	// usage-deny
	tests = append(tests, &testCase{
		_id:              "usage-deny",
		_expectedCause:   causeSettingsUsage,
		_expectedReason:  "tool-policy: denied Bash — " + causeSettingsUsage,
		_expectedReport:  false,
		_expectedApplied: false,

		grants: human,
		input:  &HookInput{Cwd: cwd, PermissionMode: permissionModeAcceptEdits, ToolInput: ToolInput{Command: "toolpolicy check"}, ToolName: toolNameBash},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			verdict := Decide(test.input, policy, test.grants, home, "")
			assert.Equal(t, decisionDeny, verdict.Decision)
			assert.Equal(t, test._expectedCause, verdict.Cause)
			assert.Contains(t, verdict.Reason, test._expectedReason)
			assert.Equal(t, test._expectedReport, verdict.IsReport)
			assert.Equal(t, test._expectedApplied, verdict.IsApplied)
		})
	}
}

func TestSplitCheckArgs(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedSkills  []string
		_expectedTargets []string

		args []string
	}

	tests := make([]*testCase, 0)

	// no-flags
	tests = append(tests, &testCase{
		_id:              "no-flags",
		_expectedSkills:  []string{},
		_expectedTargets: []string{"ls", "go test ./..."},

		args: []string{"ls", "go test ./..."},
	})

	// two-skills
	tests = append(tests, &testCase{
		_id:              "two-skills",
		_expectedSkills:  []string{"fimplement", "package-commit"},
		_expectedTargets: []string{"ls"},

		args: []string{"--skill", "fimplement", "--skill", "package-commit", "ls"},
	})

	// dangling-flag-is-target
	tests = append(tests, &testCase{
		_id:              "dangling-flag-is-target",
		_expectedSkills:  []string{},
		_expectedTargets: []string{"--skill"},

		args: []string{"--skill"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			skills, targets := splitCheckArgs(test.args)
			assert.Equal(t, test._expectedSkills, skills)
			assert.Equal(t, test._expectedTargets, targets)
		})
	}
}

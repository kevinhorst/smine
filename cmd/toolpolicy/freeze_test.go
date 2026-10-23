package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kevinhorst/smine/internal/planrules"
)

const (
	freezeTestPlan      = "plans/2026-10-01-feature-abc1234/2026-10-01-feature-wip-abc1234"
	freezeTestOtherPlan = "plans/2026-10-01-other-def5678/2026-10-01-other-wip-def5678"
	freezeTestSession   = "session-a"
)

// freezeTestRecord is a ledger record of plan written by session, holding no
// rule.
func freezeTestRecord(plan string, session string) *planrules.Record {
	file := &planrules.File{Checked: []string{}, Rules: []*planrules.Rule{}}
	record := &planrules.Record{File: file, Plan: plan, Repo: "/repo", SessionId: session, Timestamp: "2000-01-01T00:00:00Z"}
	return record
}

// freezeTestRulesFile is a rules file of rows, each a rule and its target.
func freezeTestRulesFile(rows ...[2]string) *planrules.File {
	file := &planrules.File{Checked: []string{}, Rules: []*planrules.Rule{}}
	for _, row := range rows {
		file.Rules = append(file.Rules, &planrules.Rule{NeededFor: "test", Rule: row[0], Target: row[1]})
	}
	return file
}

func TestFreezeGuard_PlanCause(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedCause string

		guard *freezeGuard
		plan  string
	}

	tests := make([]*testCase, 0)

	// one-prompt-new-plan
	tests = append(tests, &testCase{
		_id:            "one-prompt-new-plan",
		_expectedCause: "",

		guard: &freezeGuard{designPrompts: []string{""}, records: []*planrules.Record{}, sessionId: freezeTestSession},
		plan:  freezeTestPlan,
	})

	// no-design-prompt
	tests = append(tests, &testCase{
		_id:            "no-design-prompt",
		_expectedCause: causeFreezeBudget,

		guard: &freezeGuard{designPrompts: nil, records: []*planrules.Record{}, sessionId: freezeTestSession},
		plan:  freezeTestPlan,
	})

	// budget-spent
	tests = append(tests, &testCase{
		_id:            "budget-spent",
		_expectedCause: causeFreezeBudget,

		guard: &freezeGuard{
			designPrompts: []string{""},
			records:       []*planrules.Record{freezeTestRecord(freezeTestOtherPlan, freezeTestSession)},
			sessionId:     freezeTestSession,
		},
		plan: freezeTestPlan,
	})

	// two-prompts-one-spent
	tests = append(tests, &testCase{
		_id:            "two-prompts-one-spent",
		_expectedCause: "",

		guard: &freezeGuard{
			designPrompts: []string{"", "refine"},
			records:       []*planrules.Record{freezeTestRecord(freezeTestOtherPlan, freezeTestSession)},
			sessionId:     freezeTestSession,
		},
		plan: freezeTestPlan,
	})

	// frozen-plan-not-named
	tests = append(tests, &testCase{
		_id:            "frozen-plan-not-named",
		_expectedCause: causeFreezeReopen,

		guard: &freezeGuard{
			designPrompts: []string{"refine other.md"},
			records:       []*planrules.Record{freezeTestRecord(freezeTestPlan, "session-b")},
			sessionId:     freezeTestSession,
		},
		plan: freezeTestPlan,
	})

	// frozen-plan-named-by-file-name
	tests = append(tests, &testCase{
		_id:            "frozen-plan-named-by-file-name",
		_expectedCause: "",

		guard: &freezeGuard{
			designPrompts: []string{"refine feature.md"},
			records:       []*planrules.Record{freezeTestRecord(freezeTestPlan, "session-b")},
			sessionId:     freezeTestSession,
		},
		plan: freezeTestPlan,
	})

	// frozen-plan-named-by-path
	tests = append(tests, &testCase{
		_id:            "frozen-plan-named-by-path",
		_expectedCause: "",

		guard: &freezeGuard{
			designPrompts: []string{"refine " + freezeTestPlan + "/design/feature.md"},
			records:       []*planrules.Record{freezeTestRecord(freezeTestPlan, "session-b")},
			sessionId:     freezeTestSession,
		},
		plan: freezeTestPlan,
	})

	// other-sessions-records-not-counted
	tests = append(tests, &testCase{
		_id:            "other-sessions-records-not-counted",
		_expectedCause: "",

		guard: &freezeGuard{
			designPrompts: []string{""},
			records: []*planrules.Record{
				freezeTestRecord(freezeTestOtherPlan, "session-b"),
				freezeTestRecord(freezeTestOtherPlan, "session-c"),
			},
			sessionId: freezeTestSession,
		},
		plan: freezeTestPlan,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expectedCause, test.guard.planCause(&defaultTexts.Causes, test.plan))
		})
	}
}

func TestFreezeGuard_FileCause(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedCause string

		cwd    string
		guard  *freezeGuard
		target string
	}

	root := t.TempDir()
	rulesFile := filepath.Join(root, filepath.FromSlash(freezeTestPlan), planrules.FileName)
	tests := make([]*testCase, 0)

	// rules-file-budget-left
	tests = append(tests, &testCase{
		_id:            "rules-file-budget-left",
		_expectedCause: "",

		cwd:    root,
		guard:  &freezeGuard{designPrompts: []string{""}, records: []*planrules.Record{}, root: root, sessionId: freezeTestSession},
		target: rulesFile,
	})

	// rules-file-budget-spent
	tests = append(tests, &testCase{
		_id:            "rules-file-budget-spent",
		_expectedCause: causeRulesFile + ": " + causeFreezeBudget,

		cwd:    root,
		guard:  &freezeGuard{designPrompts: nil, records: []*planrules.Record{}, root: root, sessionId: freezeTestSession},
		target: rulesFile,
	})

	// rules-file-of-frozen-plan
	tests = append(tests, &testCase{
		_id:            "rules-file-of-frozen-plan",
		_expectedCause: causeRulesFile + ": " + causeFreezeReopen,

		cwd: root,
		guard: &freezeGuard{
			designPrompts: []string{""},
			records:       []*planrules.Record{freezeTestRecord(freezeTestPlan, "session-b")},
			root:          root,
			sessionId:     freezeTestSession,
		},
		target: filepath.FromSlash(freezeTestPlan + "/" + planrules.FileName),
	})

	// other-file-in-plan-dir
	tests = append(tests, &testCase{
		_id:            "other-file-in-plan-dir",
		_expectedCause: "",

		cwd:    root,
		guard:  &freezeGuard{designPrompts: nil, records: []*planrules.Record{}, root: root, sessionId: freezeTestSession},
		target: filepath.Join(root, filepath.FromSlash(freezeTestPlan), "notes.json"),
	})

	// rules-file-in-design-dir
	tests = append(tests, &testCase{
		_id:            "rules-file-in-design-dir",
		_expectedCause: "",

		cwd:    root,
		guard:  &freezeGuard{designPrompts: nil, records: []*planrules.Record{}, root: root, sessionId: freezeTestSession},
		target: filepath.Join(root, filepath.FromSlash(freezeTestPlan), "design", planrules.FileName),
	})

	// rules-file-at-old-depth
	tests = append(tests, &testCase{
		_id:            "rules-file-at-old-depth",
		_expectedCause: "",

		cwd:    root,
		guard:  &freezeGuard{designPrompts: nil, records: []*planrules.Record{}, root: root, sessionId: freezeTestSession},
		target: filepath.Join(root, "plans", "2026-10-01-feature-wip-abc1234", planrules.FileName),
	})

	// rules-file-outside-plans
	tests = append(tests, &testCase{
		_id:            "rules-file-outside-plans",
		_expectedCause: "",

		cwd:    root,
		guard:  &freezeGuard{designPrompts: nil, records: []*planrules.Record{}, root: root, sessionId: freezeTestSession},
		target: filepath.Join(root, planrules.FileName),
	})

	// nil-guard
	tests = append(tests, &testCase{
		_id:            "nil-guard",
		_expectedCause: "",

		cwd:    root,
		guard:  nil,
		target: rulesFile,
	})

	// no-anchor
	tests = append(tests, &testCase{
		_id:            "no-anchor",
		_expectedCause: "",

		cwd:    root,
		guard:  &freezeGuard{designPrompts: nil, records: []*planrules.Record{}, root: "", sessionId: freezeTestSession},
		target: rulesFile,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expectedCause, test.guard.fileCause(&defaultTexts.Causes, test.cwd, test.target))
		})
	}
}

func TestRuleRowsCause(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool

		file  *planrules.File
		input *HookInput
	}

	input := &HookInput{Cwd: t.TempDir(), SessionId: freezeTestSession}
	policy := defaultToolpolicy(t)

	tests := make([]*testCase, 0)

	// pass-narrowest-bash-rule
	tests = append(tests, &testCase{
		_id:         "pass-narrowest-bash-rule",
		_shouldPass: true,

		file:  freezeTestRulesFile([2]string{"Bash(lsof *)", "lsof -i :6101"}),
		input: input,
	})

	// pass-fetch-domain
	tests = append(tests, &testCase{
		_id:         "pass-fetch-domain",
		_shouldPass: true,

		file:  freezeTestRulesFile([2]string{"WebFetch(domain:docs.rs)", "https://docs.rs/gix"}),
		input: input,
	})

	// pass-mcp-tool
	tests = append(tests, &testCase{
		_id:         "pass-mcp-tool",
		_shouldPass: true,

		file:  freezeTestRulesFile([2]string{"mcp__goland__search_symbol", "mcp__goland__search_symbol"}),
		input: input,
	})

	// pass-no-rules
	tests = append(tests, &testCase{
		_id:         "pass-no-rules",
		_shouldPass: true,

		file:  freezeTestRulesFile(),
		input: input,
	})

	// fail-wider-than-target
	tests = append(tests, &testCase{
		_id:         "fail-wider-than-target",
		_shouldPass: false,

		file:  freezeTestRulesFile([2]string{"Bash(go *)", "go test ./cmd/toolpolicy/..."}),
		input: input,
	})

	// fail-empty-target
	tests = append(tests, &testCase{
		_id:         "fail-empty-target",
		_shouldPass: false,

		file:  freezeTestRulesFile([2]string{"Bash(lsof *)", ""}),
		input: input,
	})

	// fail-wrapper-rule
	tests = append(tests, &testCase{
		_id:         "fail-wrapper-rule",
		_shouldPass: false,

		file:  freezeTestRulesFile([2]string{"Bash(xargs *)", "xargs ls"}),
		input: input,
	})

	// fail-repeated-rule
	tests = append(tests, &testCase{
		_id:         "fail-repeated-rule",
		_shouldPass: false,

		file:  freezeTestRulesFile([2]string{"Bash(lsof *)", "lsof -i :6101"}, [2]string{"Bash(lsof *)", "lsof -i :6101"}),
		input: input,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			cause := ruleRowsCause(test.file, test.input, policy)
			assert.Equalf(t, test._shouldPass, cause == "", "cause = %s", cause)
		})
	}
}

func TestAppendRecord(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedRecords []*planrules.Record
		_shouldPass      bool

		ledger string
		record *planrules.Record
		repo   string
	}

	record := freezeTestRecord(freezeTestPlan, freezeTestSession)
	tests := make([]*testCase, 0)

	// append-then-load
	tests = append(tests, &testCase{
		_id:              "append-then-load",
		_expectedRecords: []*planrules.Record{record},
		_shouldPass:      true,

		ledger: filepath.Join(t.TempDir(), "tool-policy", planrules.LedgerName),
		record: record,
		repo:   record.Repo,
	})

	// empty-path-fails
	tests = append(tests, &testCase{
		_id:              "empty-path-fails",
		_expectedRecords: []*planrules.Record{},
		_shouldPass:      false,

		ledger: "",
		record: record,
		repo:   record.Repo,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := appendRecord(test.ledger, test.record)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedRecords, planrules.LoadRecords(test.ledger, test.repo))
		})
	}
}

func TestDesignPrompts(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedPrompts []string

		headless string
		scan     *transcriptScan
	}

	tests := make([]*testCase, 0)

	// human-prompts-kept
	tests = append(tests, &testCase{
		_id:              "human-prompts-kept",
		_expectedPrompts: []string{"", "refine feature.md"},

		headless: "",
		scan:     &transcriptScan{designPrompts: []string{"", "refine feature.md"}, firstPrompt: "/fdesign"},
	})

	// headless-first-prompt-is-fdesign
	tests = append(tests, &testCase{
		_id:              "headless-first-prompt-is-fdesign",
		_expectedPrompts: []string{"--nightly feature.md"},

		headless: headlessEnvValue,
		scan:     &transcriptScan{designPrompts: nil, firstPrompt: "/fdesign --nightly feature.md"},
	})

	// headless-first-prompt-is-other-skill
	tests = append(tests, &testCase{
		_id:              "headless-first-prompt-is-other-skill",
		_expectedPrompts: nil,

		headless: headlessEnvValue,
		scan:     &transcriptScan{designPrompts: []string{"refine feature.md"}, firstPrompt: "/fimplement feature.md"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			t.Setenv(headlessEnv, test.headless)
			assert.Equal(t, test._expectedPrompts, designPrompts(test.scan, testDesignSkill))
		})
	}
}

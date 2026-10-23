package contextrecord

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kevinhorst/smine/internal/acdsl"
)

func TestClassifyHook(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedKind    Kind
		_expectedLabel   string
		_expectedVersion string

		hookName string
		text     string
	}

	tests := make([]*testCase, 0)

	// global-inline
	tests = append(tests, &testCase{
		_id:              "global-inline",
		_expectedKind:    KindGlobal,
		_expectedLabel:   "SessionStart:startup",
		_expectedVersion: "",

		hookName: "SessionStart:startup",
		text:     "===== GLOBAL CONTEXT (injected by hook) =====\n\n===== /Users/u/.claude/context/global/always-context.md =====\nbody\n",
	})

	// global-pointer
	tests = append(tests, &testCase{
		_id:              "global-pointer",
		_expectedKind:    KindGlobal,
		_expectedLabel:   "SubagentStart",
		_expectedVersion: "",

		hookName: "SubagentStart",
		text:     "===== GLOBAL CONTEXT (injected by hook) =====\nWritten to /Users/u/.claude/context-required/sid/global.md (15484 chars, 104 lines). Read it in full before any other tool call.",
	})

	// skill-with-version
	tests = append(tests, &testCase{
		_id:              "skill-with-version",
		_expectedKind:    KindSkill,
		_expectedLabel:   "fdesign",
		_expectedVersion: "6.3.0",

		hookName: "UserPromptSubmit",
		text:     "===== SKILL CONTEXT /fdesign v6.3.0 (injected by hook) =====\n**ACTION-CONCEPT-HOT-006** body\n",
	})

	// skill-with-lane-and-version
	tests = append(tests, &testCase{
		_id:              "skill-with-lane-and-version",
		_expectedKind:    KindSkill,
		_expectedLabel:   "railroad-review",
		_expectedVersion: "4.1.0",

		hookName: "UserPromptSubmit",
		text:     "===== SKILL CONTEXT /railroad-review lane:correctness v4.1.0 (injected by hook) =====\n**ACTION-REVIEW-DOD-001** body\n",
	})

	// skill-with-lane
	tests = append(tests, &testCase{
		_id:              "skill-with-lane",
		_expectedKind:    KindSkill,
		_expectedLabel:   "railroad-review",
		_expectedVersion: "",

		hookName: "UserPromptSubmit",
		text:     "===== SKILL CONTEXT /railroad-review lane:correctness (injected by hook) =====\n**ACTION-REVIEW-DOD-001** body\n",
	})

	// plan-context
	tests = append(tests, &testCase{
		_id:              "plan-context",
		_expectedKind:    KindPlanContext,
		_expectedLabel:   "UserPromptSubmit",
		_expectedVersion: "",

		hookName: "UserPromptSubmit",
		text:     "===== PLAN CONTEXT (injected by hook) =====\n**RULE-PLAN-FMT-001** body\n",
	})

	// acdsl-context-note
	tests = append(tests, &testCase{
		_id:              "acdsl-context-note",
		_expectedKind:    KindAcdslContext,
		_expectedLabel:   "x.json (gate-enforced…)",
		_expectedVersion: "",

		hookName: "PreToolUse:Edit",
		text:     "ACDSL rules governing x.json (gate-enforced…):\n- [ACDSL-JSON-FMT-001] body\n",
	})

	// agents-md-harness-load
	tests = append(tests, &testCase{
		_id:              "agents-md-harness-load",
		_expectedKind:    KindAgentsMd,
		_expectedLabel:   "/repo/context/AGENTS.md",
		_expectedVersion: "",

		hookName: "tool.call",
		text:     "Contents of /repo/context/AGENTS.md:\n\n# Agents\n",
	})

	// sessiontitle
	tests = append(tests, &testCase{
		_id:              "sessiontitle",
		_expectedKind:    KindHookContext,
		_expectedLabel:   "sessiontitle",
		_expectedVersion: "",

		hookName: "PostToolUse:Bash",
		text:     "Session title: \"CC,FD - Injected token count - [IMPL]\"\nCall mcp__ccd_session_mgmt__set_session_title with that title.\n",
	})

	// tool-policy
	tests = append(tests, &testCase{
		_id:              "tool-policy",
		_expectedKind:    KindHookContext,
		_expectedLabel:   "tool-policy",
		_expectedVersion: "",

		hookName: "UserPromptSubmit",
		text:     "tool-policy /fimplement hook:\n- plan: plans/x/design/x.md\n",
	})

	// worktree-notice
	tests = append(tests, &testCase{
		_id:              "worktree-notice",
		_expectedKind:    KindHookContext,
		_expectedLabel:   "worktree",
		_expectedVersion: "",

		hookName: "SessionStart:startup",
		text:     "You are operating in an isolated workspace at /repo/.claude/worktrees/x.\n",
	})

	// unmatched-with-hook-name
	tests = append(tests, &testCase{
		_id:              "unmatched-with-hook-name",
		_expectedKind:    KindHookContext,
		_expectedLabel:   "PostToolUse:Bash",
		_expectedVersion: "",

		hookName: "PostToolUse:Bash",
		text:     "Some unmarked hook output\nsecond line\n",
	})

	// unmatched-without-hook-name
	tests = append(tests, &testCase{
		_id:              "unmatched-without-hook-name",
		_expectedKind:    KindHookContext,
		_expectedLabel:   strings.Repeat("ä", 80) + "…",
		_expectedVersion: "",

		hookName: "",
		text:     strings.Repeat("ä", 90) + "\nsecond line\n",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			kind, label, version := classifyHook(test.text, test.hookName)

			assert.Equal(t, test._expectedKind, kind)
			assert.Equal(t, test._expectedLabel, label)
			assert.Equal(t, test._expectedVersion, version)
		})
	}
}

func TestHookText(t *testing.T) {
	type testCase struct {
		_id           string
		_expectedText string

		stdout string
	}

	tests := make([]*testCase, 0)

	// envelope-with-context
	tests = append(tests, &testCase{
		_id:           "envelope-with-context",
		_expectedText: "Session title: x",

		stdout: `{"hookSpecificOutput":{"hookEventName":"PostToolUse","additionalContext":"Session title: x"}}`,
	})

	// envelope-without-context
	tests = append(tests, &testCase{
		_id:           "envelope-without-context",
		_expectedText: "",

		stdout: `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow"}}`,
	})

	// empty-object
	tests = append(tests, &testCase{
		_id:           "empty-object",
		_expectedText: "",

		stdout: `{}`,
	})

	// plain-text-unchanged
	tests = append(tests, &testCase{
		_id:           "plain-text-unchanged",
		_expectedText: "===== GLOBAL CONTEXT (injected by hook) =====\nbody\n",

		stdout: "===== GLOBAL CONTEXT (injected by hook) =====\nbody\n",
	})

	// invalid-json-unchanged
	tests = append(tests, &testCase{
		_id:           "invalid-json-unchanged",
		_expectedText: `{"hookSpecificOutput":`,

		stdout: `{"hookSpecificOutput":`,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expectedText, hookText(test.stdout))
		})
	}
}

func TestSkillFromHook(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedSkill   string
		_expectedVersion string

		text string
	}

	tests := make([]*testCase, 0)

	// version
	tests = append(tests, &testCase{
		_id:              "version",
		_expectedSkill:   "fdesign",
		_expectedVersion: "6.3.0",

		text: "===== SKILL CONTEXT /fdesign v6.3.0 (injected by hook) =====\nbody\n",
	})

	// lane-and-version
	tests = append(tests, &testCase{
		_id:              "lane-and-version",
		_expectedSkill:   "railroad-review",
		_expectedVersion: "4.1.0",

		text: "===== SKILL CONTEXT /railroad-review lane:correctness v4.1.0 (injected by hook) =====\nbody\n",
	})

	// none
	tests = append(tests, &testCase{
		_id:              "none",
		_expectedSkill:   "fdesign",
		_expectedVersion: "",

		text: "===== SKILL CONTEXT /fdesign (injected by hook) =====\nbody\n",
	})

	// no-match
	tests = append(tests, &testCase{
		_id:              "no-match",
		_expectedSkill:   "",
		_expectedVersion: "",

		text: "===== PLAN CONTEXT (injected by hook) =====\nbody\n",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			skill, version := skillFromHook(test.text)

			assert.Equal(t, test._expectedSkill, skill)
			assert.Equal(t, test._expectedVersion, version)
		})
	}
}

func TestClassifyInjection(t *testing.T) {
	type testCase struct {
		_id                  string
		_expectedIsInjection bool
		_expectedKind        Kind
		_expectedLabel       string

		path string
	}

	tests := make([]*testCase, 0)

	// global-injection
	tests = append(tests, &testCase{
		_id:                  "global-injection",
		_expectedIsInjection: true,
		_expectedKind:        KindGlobal,
		_expectedLabel:       "/Users/u/.claude/context-required/sid/global.md",

		path: "/Users/u/.claude/context-required/sid/global.md",
	})

	// plan-mode-injection
	tests = append(tests, &testCase{
		_id:                  "plan-mode-injection",
		_expectedIsInjection: true,
		_expectedKind:        KindPlanContext,
		_expectedLabel:       "/Users/u/.claude/context-required/sid/planmode.md",

		path: "/Users/u/.claude/context-required/sid/planmode.md",
	})

	// skill-injection
	tests = append(tests, &testCase{
		_id:                  "skill-injection",
		_expectedIsInjection: true,
		_expectedKind:        KindSkill,
		_expectedLabel:       "fdesign",

		path: "/Users/u/.claude/context-required/sid/fdesign--main.md",
	})

	// skill-injection-with-lane
	tests = append(tests, &testCase{
		_id:                  "skill-injection-with-lane",
		_expectedIsInjection: true,
		_expectedKind:        KindSkill,
		_expectedLabel:       "railroad-review",

		path: "/Users/u/.claude/context-required/sid/railroad-review--lane-correctness--a1b2c3.md",
	})

	// non-injection-path
	tests = append(tests, &testCase{
		_id:                  "non-injection-path",
		_expectedIsInjection: false,
		_expectedKind:        "",
		_expectedLabel:       "",

		path: "/repo/context/rules/go.md",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			kind, label, isInjection := classifyInjection(test.path)

			assert.Equal(t, test._expectedKind, kind)
			assert.Equal(t, test._expectedLabel, label)
			assert.Equal(t, test._expectedIsInjection, isInjection)
		})
	}
}

func TestProjectionBlock(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedBlock string
		_expectedFound bool

		text string
	}

	numberedHeader := "     1\t// " + acdsl.ProjectionMarker + " 2 rule(s) govern this file\n" +
		"     2\t// - " + "[ACDSL-GOLANG-FMT-001] gofmt\n" +
		"     3\t// - " + "[ACDSL-GOLANG-LINT-001] lint"
	shebangHeader := "     2\t# " + acdsl.ProjectionMarker + " 1 rule(s) govern this file\n" +
		"     3\t# - " + "[ACDSL-SHELL-003] frozen set"
	frontMatterHeader := "     4\t<!-- " + acdsl.ProjectionMarker + " 1 rule(s) govern this file -->\n" +
		"     5\t<!-- - " + "[ACDSL-SKILL-001] model -->"
	plainHeader := acdsl.ProjectionMarker + " 1 rule(s) govern this file\n- [ACDSL-JSON-FMT-001] formatted"

	tests := make([]*testCase, 0)

	// numbered-read-result
	tests = append(tests, &testCase{
		_id:            "numbered-read-result",
		_expectedBlock: numberedHeader,
		_expectedFound: true,

		text: numberedHeader + "\n     4\t\n     5\tpackage x\n",
	})

	// block-to-end
	tests = append(tests, &testCase{
		_id:            "block-to-end",
		_expectedBlock: numberedHeader,
		_expectedFound: true,

		text: numberedHeader,
	})

	// after-shebang
	tests = append(tests, &testCase{
		_id:            "after-shebang",
		_expectedBlock: shebangHeader,
		_expectedFound: true,

		text: "     1\t#!/usr/bin/env bash\n" + shebangHeader + "\n     4\t\n     5\tset -e\n",
	})

	// after-front-matter
	tests = append(tests, &testCase{
		_id:            "after-front-matter",
		_expectedBlock: frontMatterHeader,
		_expectedFound: true,

		text: "     1\t---\n     2\tname: x\n     3\t---\n" + frontMatterHeader + "\n     6\t\n     7\t# X\n",
	})

	// front-matter-never-closed
	tests = append(tests, &testCase{
		_id:            "front-matter-never-closed",
		_expectedBlock: "",
		_expectedFound: false,

		text: "     1\t---\n     2\tname: x\n" + frontMatterHeader + "\n",
	})

	// marker-in-file-body
	tests = append(tests, &testCase{
		_id:            "marker-in-file-body",
		_expectedBlock: "",
		_expectedFound: false,

		text: "     1\t# Plan\n     2\t\n     3\tThe block starts with " + acdsl.ProjectionMarker + " 6 rule(s) govern this file\n",
	})

	// chunk-past-the-head
	tests = append(tests, &testCase{
		_id:            "chunk-past-the-head",
		_expectedBlock: "",
		_expectedFound: false,

		text: "    30\t// " + acdsl.ProjectionMarker + " 2 rule(s) govern this file\n    31\t// - " + "[ACDSL-GOLANG-FMT-001] gofmt\n",
	})

	// unnumbered-text
	tests = append(tests, &testCase{
		_id:            "unnumbered-text",
		_expectedBlock: "",
		_expectedFound: false,

		text: "Edited x.json\n" + plainHeader + "\n\n{}\n",
	})

	// marker-absent
	tests = append(tests, &testCase{
		_id:            "marker-absent",
		_expectedBlock: "",
		_expectedFound: false,

		text: "     1\tpackage x\n     2\t\n",
	})

	// empty
	tests = append(tests, &testCase{
		_id:            "empty",
		_expectedBlock: "",
		_expectedFound: false,

		text: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			block, found := projectionBlock(test.text)

			assert.Equal(t, test._expectedBlock, block)
			assert.Equal(t, test._expectedFound, found)
		})
	}
}

func TestLangOf(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedFound bool
		_expectedLang  string

		path string
	}

	tests := make([]*testCase, 0)

	// go-guide
	tests = append(tests, &testCase{
		_id:            "go-guide",
		_expectedFound: true,
		_expectedLang:  "go",

		path: "/repo/context/rules/go.md",
	})

	// go-part-guide
	tests = append(tests, &testCase{
		_id:            "go-part-guide",
		_expectedFound: true,
		_expectedLang:  "go",

		path: "/repo/context/rules/go_test.md",
	})

	// python-guide
	tests = append(tests, &testCase{
		_id:            "python-guide",
		_expectedFound: true,
		_expectedLang:  "python",

		path: "/repo/x/docs/rules/python.md",
	})

	// not-a-guide
	tests = append(tests, &testCase{
		_id:            "not-a-guide",
		_expectedFound: false,
		_expectedLang:  "",

		path: "/repo/context/actions/implementing.md",
	})

	// suffix-after-md
	tests = append(tests, &testCase{
		_id:            "suffix-after-md",
		_expectedFound: false,
		_expectedLang:  "",

		path: "/repo/context/rules/go.md.bak",
	})

	// part-suffix-after-md
	tests = append(tests, &testCase{
		_id:            "part-suffix-after-md",
		_expectedFound: false,
		_expectedLang:  "",

		path: "/repo/context/rules/go_test_data.md.bak",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			lang, found := langOf(test.path)

			assert.Equal(t, test._expectedLang, lang)
			assert.Equal(t, test._expectedFound, found)
		})
	}
}

func TestContextPaths(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedPaths []string

		ctxDir string
		prompt string
	}

	tests := make([]*testCase, 0)

	// shell-case
	tests = append(tests, &testCase{
		_id:            "shell-case",
		_expectedPaths: []string{"docs/actions/implementing.md"},

		ctxDir: "docs",
		prompt: "Read docs/actions/implementing.md first; honor ACTION-IMPL-001 and RULE-GOLANG-TEST-002.",
	})

	// trailing-punctuation-kept
	tests = append(tests, &testCase{
		_id:            "trailing-punctuation-kept",
		_expectedPaths: []string{"/repo/context/rules/go.md", "context/rules/go.md."},

		ctxDir: "context",
		prompt: "Read context/rules/go.md. Then `/repo/context/rules/go.md` and context/rules/go.md. again",
	})

	// none
	tests = append(tests, &testCase{
		_id:            "none",
		_expectedPaths: []string{},

		ctxDir: "docs",
		prompt: "Implement the parser; no context paths here.",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expectedPaths, contextPaths(test.prompt, test.ctxDir))
		})
	}
}

func TestIds(t *testing.T) {
	type testCase struct {
		_id          string
		_expectedIds []string

		text string
	}

	tests := make([]*testCase, 0)

	// unsorted-duplicates
	tests = append(tests, &testCase{
		_id:          "unsorted-duplicates",
		_expectedIds: []string{"ACTION-IMPL-001", "RULE-GOLANG-TEST-002"},

		text: "RULE-GOLANG-TEST-002 then ACTION-IMPL-001 and RULE-GOLANG-TEST-002 again",
	})

	// no-ids
	tests = append(tests, &testCase{
		_id:          "no-ids",
		_expectedIds: []string{},

		text: "nothing here, not even RULE-12 or lower-case-001",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expectedIds, ids(test.text))
		})
	}
}

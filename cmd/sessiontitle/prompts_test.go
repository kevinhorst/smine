package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromptSkills(t *testing.T) {
	type testCase struct {
		_id                string
		_expected          []string
		_expectedHasPrompt bool
		_shouldPass        bool

		transcriptPath string
	}

	tests := make([]*testCase, 0)

	// pass-first-prompt-skill
	tests = append(tests, &testCase{
		_id:                "pass-first-prompt-skill",
		_expected:          []string{"fdesign", "qq"},
		_expectedHasPrompt: true,
		_shouldPass:        true,

		transcriptPath: filepath.Join("testdata", "prompts-fdesign.jsonl"),
	})

	// pass-later-skill-order
	tests = append(tests, &testCase{
		_id:                "pass-later-skill-order",
		_expected:          []string{"qq", "fdesign"},
		_expectedHasPrompt: true,
		_shouldPass:        true,

		transcriptPath: filepath.Join("testdata", "prompts-later-skill.jsonl"),
	})

	// pass-no-skills
	tests = append(tests, &testCase{
		_id:                "pass-no-skills",
		_expected:          []string{},
		_expectedHasPrompt: true,
		_shouldPass:        true,

		transcriptPath: filepath.Join("testdata", "prompts-none.jsonl"),
	})

	// pass-queued-prompt-not-written-yet
	tests = append(tests, &testCase{
		_id:                "pass-queued-prompt-not-written-yet",
		_expected:          []string{},
		_expectedHasPrompt: false,
		_shouldPass:        true,

		transcriptPath: filepath.Join("testdata", "prompts-queued.jsonl"),
	})

	// pass-missing-file-not-yet
	tests = append(tests, &testCase{
		_id:                "pass-missing-file-not-yet",
		_expected:          nil,
		_expectedHasPrompt: false,
		_shouldPass:        true,

		transcriptPath: filepath.Join("testdata", "missing.jsonl"),
	})

	// fail-unreadable-directory
	tests = append(tests, &testCase{
		_id:                "fail-unreadable-directory",
		_expected:          nil,
		_expectedHasPrompt: false,
		_shouldPass:        false,

		transcriptPath: "testdata",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			skills, hasPrompt, err := promptSkills(test.transcriptPath)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, skills)
			assert.Equal(t, test._expectedHasPrompt, hasPrompt)
		})
	}
}

func TestTitleSetPattern(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		command string
	}

	tests := make([]*testCase, 0)

	// default-program
	tests = append(tests, &testCase{
		_id:       "default-program",
		_expected: "WIP",

		command: "smine-rules title set WIP",
	})

	// path-program
	tests = append(tests, &testCase{
		_id:       "path-program",
		_expected: "READY",

		command: "/opt/bin/rules title set READY",
	})

	// other-program
	tests = append(tests, &testCase{
		_id:       "other-program",
		_expected: "DESIGN-READY",

		command: "my-rules title set DESIGN-READY",
	})

	// chained-command
	tests = append(tests, &testCase{
		_id:       "chained-command",
		_expected: "WIP",

		command: "x && smine-rules title set WIP",
	})

	// bare-title-set-no-match
	tests = append(tests, &testCase{
		_id:       "bare-title-set-no-match",
		_expected: "",

		command: "title set WIP",
	})

	// lowercase-token-no-match
	tests = append(tests, &testCase{
		_id:       "lowercase-token-no-match",
		_expected: "",

		command: "smine-rules title set wip",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, titleSetToken(t, test.command))
		})
	}
}

// titleSetToken is the token the default title-set pattern captures, "" without a match.
func titleSetToken(t *testing.T, command string) string {
	t.Helper()
	triggers, err := compileTriggers(defaultHookConfig(t).Sessiontitle.TriggerPatterns)
	require.NoError(t, err)
	match := triggers.titleSet.FindStringSubmatch(command)
	if match == nil {
		return ""
	}
	return match[titleSetTokenGroup]
}

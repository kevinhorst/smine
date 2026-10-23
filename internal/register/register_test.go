package register

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSentenceViolations(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		text string
	}

	tests := make([]*testCase, 0)

	// pass-plain-sentence
	tests = append(tests, &testCase{
		_id:       "pass-plain-sentence",
		_expected: nil,

		text: "The memory stage reported no auto-memory after its only probe was denied.",
	})

	// pass-date-digits
	tests = append(tests, &testCase{
		_id:       "pass-date-digits",
		_expected: nil,

		text: "The 2026-10-08 nightly ran unattended to an accepted verdict.",
	})

	// fail-two-sentences
	tests = append(tests, &testCase{
		_id:       "fail-two-sentences",
		_expected: []string{"more than one sentence"},

		text: "The probe was denied. The stage reported success anyway.",
	})

	// fail-backtick
	tests = append(tests, &testCase{
		_id:       "fail-backtick",
		_expected: []string{"backtick"},

		text: "The `ls` probe was denied by the gate.",
	})

	// fail-hex-id
	tests = append(tests, &testCase{
		_id:       "fail-hex-id",
		_expected: []string{"hex id"},

		text: "Session 9c44f8b2 found nothing to commit.",
	})

	// fail-path-line
	tests = append(tests, &testCase{
		_id:       "fail-path-line",
		_expected: []string{"path with line number"},

		text: "The check at internal/sessions/record.go:80 returns schema violations only.",
	})

	// fail-parenthesised-count
	tests = append(tests, &testCase{
		_id:       "fail-parenthesised-count",
		_expected: []string{"parenthesised number"},

		text: "The flags were dropped silently (2 sessions, 112 turns).",
	})

	// fail-several
	tests = append(tests, &testCase{
		_id:       "fail-several",
		_expected: []string{"more than one sentence", "backtick", "hex id"},

		text: "Run `git diff` first. Session 9c44f8b2 had untracked files.",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, SentenceViolations(test.text))
		})
	}
}

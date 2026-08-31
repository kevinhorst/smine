package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBootstrapEnv(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   []string
		_shouldPass bool

		extraPrompt string
		since       string
	}

	tests := make([]*testCase, 0)

	// empty-inputs
	tests = append(tests, &testCase{
		_id:         "empty-inputs",
		_expected:   []string{},
		_shouldPass: true,

		extraPrompt: "",
		since:       "",
	})

	// valid-since
	tests = append(tests, &testCase{
		_id:         "valid-since",
		_expected:   []string{"BOOTSTRAP_SINCE=2026-08-01"},
		_shouldPass: true,

		extraPrompt: "",
		since:       "2026-08-01",
	})

	// invalid-since
	tests = append(tests, &testCase{
		_id:         "invalid-since",
		_expected:   nil,
		_shouldPass: false,

		extraPrompt: "",
		since:       "banana",
	})

	// extra-prompt-appended
	tests = append(tests, &testCase{
		_id:         "extra-prompt-appended",
		_expected:   []string{"BOOTSTRAP_SINCE=2026-08-01", "BOOTSTRAP_EXTRA_PROMPT=skip dev sessions"},
		_shouldPass: true,

		extraPrompt: "skip dev sessions",
		since:       "2026-08-01",
	})

	// extra-prompt-only
	tests = append(tests, &testCase{
		_id:         "extra-prompt-only",
		_expected:   []string{"BOOTSTRAP_EXTRA_PROMPT=focus on tvde work"},
		_shouldPass: true,

		extraPrompt: "focus on tvde work",
		since:       "",
	})

	// quote-in-extra-prompt
	tests = append(tests, &testCase{
		_id:         "quote-in-extra-prompt",
		_expected:   nil,
		_shouldPass: false,

		extraPrompt: `say "hi"`,
		since:       "",
	})

	// newline-in-extra-prompt
	tests = append(tests, &testCase{
		_id:         "newline-in-extra-prompt",
		_expected:   nil,
		_shouldPass: false,

		extraPrompt: "line one\nline two",
		since:       "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			env, err := bootstrapEnv(test.since, test.extraPrompt)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, env)
		})
	}
}

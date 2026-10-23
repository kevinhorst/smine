package evals

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckInterpretation(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []InterpretationViolation

		markdown string
	}

	fitting := strings.Repeat("y", 160)
	tooLong := strings.Repeat("y", 161)

	tests := make([]*testCase, 0)

	// pass-eight-fitting-bullets
	tests = append(tests, &testCase{
		_id:       "pass-eight-fitting-bullets",
		_expected: nil,

		markdown: "- a\n- b\n- c\n- d\n- e\n- f\n- g\n- " + fitting + "\n",
	})

	// fail-heading-and-paragraph
	tests = append(tests, &testCase{
		_id: "fail-heading-and-paragraph",
		_expected: []InterpretationViolation{
			{Line: 1, Message: "only top-level bullets allowed, no heading or paragraph"},
			{Line: 5, Message: "only top-level bullets allowed, no heading or paragraph"},
		},

		markdown: "# Interpretation\n\n- a\n\nWorker pins: none.\n",
	})

	// fail-nine-bullets
	tests = append(tests, &testCase{
		_id:       "fail-nine-bullets",
		_expected: []InterpretationViolation{{Line: 9, Message: "more than 8 bullets"}},

		markdown: "- a\n- b\n- c\n- d\n- e\n- f\n- g\n- h\n- i\n",
	})

	// fail-long-bullet
	tests = append(tests, &testCase{
		_id:       "fail-long-bullet",
		_expected: []InterpretationViolation{{Line: 2, Message: "bullet is 161 runes, at most 160"}},

		markdown: "- a\n- " + tooLong + "\n",
	})

	// fail-nested-bullet
	tests = append(tests, &testCase{
		_id:       "fail-nested-bullet",
		_expected: []InterpretationViolation{{Line: 2, Message: "only top-level bullets allowed, no heading or paragraph"}},

		markdown: "- top\n  - nested\n",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, CheckInterpretation(test.markdown))
		})
	}
}

func TestIsInterpretationChecked(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		path string
	}

	tests := make([]*testCase, 0)

	// cutoff-day
	tests = append(tests, &testCase{
		_id:       "cutoff-day",
		_expected: true,

		path: "evals/railroad-review/2026-10-06-1f725c/interpretation.md",
	})

	// before-cutoff
	tests = append(tests, &testCase{
		_id:       "before-cutoff",
		_expected: false,

		path: "evals/railroad-review/2026-10-05-1f725c-4/interpretation.md",
	})

	// undated-path
	tests = append(tests, &testCase{
		_id:       "undated-path",
		_expected: true,

		path: "acdsl/testdata/ACDSL-SKILL-018/fail/interpretation.md",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, IsInterpretationChecked(test.path))
		})
	}
}

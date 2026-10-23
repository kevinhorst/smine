package evals

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAbFile_SideOf(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *AbSide

		key string
	}

	file := &AbFile{
		Baseline: AbSide{Key: "opus-high"},
		Variant:  AbSide{Key: "sonnet-high"},
	}
	tests := make([]*testCase, 0)

	// baseline-key
	tests = append(tests, &testCase{
		_id:       "baseline-key",
		_expected: &file.Baseline,

		key: "opus-high",
	})

	// variant-key
	tests = append(tests, &testCase{
		_id:       "variant-key",
		_expected: &file.Variant,

		key: "sonnet-high",
	})

	// unknown-key
	tests = append(tests, &testCase{
		_id:       "unknown-key",
		_expected: nil,

		key: "haiku-low",
	})

	// empty-key
	tests = append(tests, &testCase{
		_id:       "empty-key",
		_expected: nil,

		key: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Same(t, test._expected, file.SideOf(test.key))
		})
	}
}

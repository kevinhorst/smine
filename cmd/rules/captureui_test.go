package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCapture(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *captureRequest
		_isValid  bool

		args []string
	}

	tests := make([]*testCase, 0)

	// two-args
	tests = append(tests, &testCase{
		_id:       "two-args",
		_expected: &captureRequest{out: "out.png", size: captureDefaultSize, source: "http://localhost:8080", waitMs: captureDefaultWaitMs},
		_isValid:  true,

		args: []string{"http://localhost:8080", "out.png"},
	})

	// size-converted
	tests = append(tests, &testCase{
		_id:       "size-converted",
		_expected: &captureRequest{out: "out.png", size: "800,600", source: "http://localhost:8080", waitMs: captureDefaultWaitMs},
		_isValid:  true,

		args: []string{"http://localhost:8080", "out.png", "--size", "800x600"},
	})

	// wait-set
	tests = append(tests, &testCase{
		_id:       "wait-set",
		_expected: &captureRequest{out: "sim.png", size: captureDefaultSize, source: captureSimulator, waitMs: "2000"},
		_isValid:  true,

		args: []string{captureSimulator, "sim.png", "--wait-ms", "2000"},
	})

	// one-arg-rejected
	tests = append(tests, &testCase{
		_id:       "one-arg-rejected",
		_expected: nil,
		_isValid:  false,

		args: []string{"http://localhost:8080"},
	})

	// odd-count-rejected
	tests = append(tests, &testCase{
		_id:       "odd-count-rejected",
		_expected: nil,
		_isValid:  false,

		args: []string{"http://localhost:8080", "out.png", "--size"},
	})

	// unknown-flag-rejected
	tests = append(tests, &testCase{
		_id:       "unknown-flag-rejected",
		_expected: nil,
		_isValid:  false,

		args: []string{"http://localhost:8080", "out.png", "--zoom", "2"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			request, isValid := parseCapture(test.args)
			assert.Equal(t, test._isValid, isValid)
			assert.Equal(t, test._expected, request)
		})
	}
}

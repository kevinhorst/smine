package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSelectCrates(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		changed []string
		hasBase bool
		scope   string
	}

	tests := make([]*testCase, 0)

	// scope-all
	tests = append(tests, &testCase{
		_id:       "scope-all",
		_expected: []string{"agentwt", "heartbeat"},

		changed: []string{"cmd/rules/main.go"},
		hasBase: true,
		scope:   auditScopeAll,
	})

	// no-base
	tests = append(tests, &testCase{
		_id:       "no-base",
		_expected: []string{"agentwt", "heartbeat"},

		changed: nil,
		hasBase: false,
		scope:   auditScopeChanged,
	})

	// empty-change-set
	tests = append(tests, &testCase{
		_id:       "empty-change-set",
		_expected: []string{"agentwt", "heartbeat"},

		changed: []string{},
		hasBase: true,
		scope:   auditScopeChanged,
	})

	// agentwt-change-runs-both
	tests = append(tests, &testCase{
		_id:       "agentwt-change-runs-both",
		_expected: []string{"agentwt", "heartbeat"},

		changed: []string{"cmd/agentwt/src/lib.rs"},
		hasBase: true,
		scope:   auditScopeChanged,
	})

	// heartbeat-change-runs-heartbeat
	tests = append(tests, &testCase{
		_id:       "heartbeat-change-runs-heartbeat",
		_expected: []string{"heartbeat"},

		changed: []string{"cmd/heartbeat/src/main.rs"},
		hasBase: true,
		scope:   auditScopeChanged,
	})

	// go-only-change-runs-none
	tests = append(tests, &testCase{
		_id:       "go-only-change-runs-none",
		_expected: []string{},

		changed: []string{"cmd/rules/main.go", "internal/acdsl/run.go"},
		hasBase: true,
		scope:   auditScopeChanged,
	})

	// both-crates-changed
	tests = append(tests, &testCase{
		_id:       "both-crates-changed",
		_expected: []string{"agentwt", "heartbeat"},

		changed: []string{"cmd/heartbeat/Cargo.toml", "cmd/agentwt/Cargo.toml"},
		hasBase: true,
		scope:   auditScopeChanged,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, selectCrates(test.scope, test.changed, test.hasBase))
		})
	}
}

func TestRunAuditCrates(t *testing.T) {
	type testCase struct {
		_id           string
		_expectedCode int
		_expectedOut  string

		args []string
		out  *bytes.Buffer
	}

	tests := make([]*testCase, 0)

	// fail-unknown-scope
	tests = append(tests, &testCase{
		_id:           "fail-unknown-scope",
		_expectedCode: exitError,
		_expectedOut:  "",

		args: []string{"-scope", "some"},
		out:  &bytes.Buffer{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expectedCode, runAuditCrates(test.args, test.out))
			assert.Equal(t, test._expectedOut, test.out.String())
		})
	}
}

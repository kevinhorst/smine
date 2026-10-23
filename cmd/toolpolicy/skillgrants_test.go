package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFenceBash_OutsideRoots(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedCause string

		command string
	}

	fixture := fixtureSessionRoots(t)
	roots := fixture.roots(fixture.grants())

	tests := make([]*testCase, 0)

	// tmp-git-init
	tests = append(tests, &testCase{
		_id:            "tmp-git-init",
		_expectedCause: causeTmpWrite,

		command: "git init -q /tmp/probe-fixture",
	})

	// tmp-file-argument
	tests = append(tests, &testCase{
		_id:            "tmp-file-argument",
		_expectedCause: causeReadOutside,

		command: "go run ./cmd/acdsl/verifiers/symbolcoverage /tmp/files.txt",
	})

	// tmp-flag-value
	tests = append(tests, &testCase{
		_id:            "tmp-flag-value",
		_expectedCause: causeReadOutside,

		command: "go tool cover -func=/tmp/cover.out",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expectedCause, fenceBash(fixture.input(fixture.anchor, test.command), fixture.policy, roots))
		})
	}
}

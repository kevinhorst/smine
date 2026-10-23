package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLintArgs(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		anchored map[string]bool
		changed  map[string]bool
		hasBase  bool
	}

	tests := make([]*testCase, 0)

	// repo-file-changed
	tests = append(tests, &testCase{
		_id:       "repo-file-changed",
		_expected: []string{"run", "--allow-serial-runners", "./internal/shell/"},

		anchored: map[string]bool{"internal/shell/run.go": true},
		changed:  map[string]bool{"internal/shell/run.go": true},
		hasBase:  true,
	})

	// repo-file-unchanged-nil
	tests = append(tests, &testCase{
		_id:       "repo-file-unchanged-nil",
		_expected: nil,

		anchored: map[string]bool{"internal/shell/run.go": true},
		changed:  map[string]bool{},
		hasBase:  true,
	})

	// changed-not-anchored-nil
	tests = append(tests, &testCase{
		_id:       "changed-not-anchored-nil",
		_expected: nil,

		anchored: map[string]bool{"internal/shell/run.go": true},
		changed:  map[string]bool{"cmd/rules/main.go": true},
		hasBase:  true,
	})

	// no-base-whole-module
	tests = append(tests, &testCase{
		_id:       "no-base-whole-module",
		_expected: []string{"run", "--allow-serial-runners", "./..."},

		anchored: map[string]bool{"internal/shell/run.go": true},
		changed:  nil,
		hasBase:  false,
	})

	// fixture-files-sorted
	tests = append(tests, &testCase{
		_id:       "fixture-files-sorted",
		_expected: []string{"run", "--allow-serial-runners", "--no-config", "--enable-only", "mnd", "./acdsl/testdata/a/fail/", "./acdsl/testdata/b/pass/"},

		anchored: map[string]bool{
			"acdsl/testdata/b/pass/x.go": true,
			"acdsl/testdata/a/fail/y.go": true,
			"acdsl/testdata/a/fail/z.go": true,
		},
		changed: nil,
		hasBase: false,
	})

	// mixed-fixture-and-repo-changed
	tests = append(tests, &testCase{
		_id:       "mixed-fixture-and-repo-changed",
		_expected: []string{"run", "--allow-serial-runners", "./internal/shell/"},

		anchored: map[string]bool{
			"acdsl/testdata/a/fail/y.go": true,
			"internal/shell/run.go":      true,
		},
		changed: map[string]bool{"internal/shell/run.go": true},
		hasBase: true,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, lintArgs(test.anchored, test.changed, test.hasBase))
		})
	}
}

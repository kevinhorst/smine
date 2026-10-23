package reviewscope

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsTestFile(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		filePath string
	}

	tests := make([]*testCase, 0)

	// go-test
	tests = append(tests, &testCase{
		_id:       "go-test",
		_expected: true,

		filePath: "internal/reviewscope/pairs_test.go",
	})

	// go-source
	tests = append(tests, &testCase{
		_id:       "go-source",
		_expected: false,

		filePath: "internal/reviewscope/pairs.go",
	})

	// python-prefix
	tests = append(tests, &testCase{
		_id:       "python-prefix",
		_expected: true,

		filePath: "app/test_views.py",
	})

	// python-suffix
	tests = append(tests, &testCase{
		_id:       "python-suffix",
		_expected: true,

		filePath: "app/views_test.py",
	})

	// ts-spec
	tests = append(tests, &testCase{
		_id:       "ts-spec",
		_expected: true,

		filePath: "web/src/button.spec.ts",
	})

	// jsx-test
	tests = append(tests, &testCase{
		_id:       "jsx-test",
		_expected: true,

		filePath: "web/src/button.test.jsx",
	})

	// testdata-fixture
	tests = append(tests, &testCase{
		_id:       "testdata-fixture",
		_expected: true,

		filePath: "skills/quality/railroad-review/testdata/review.json",
	})

	// tests-dir
	tests = append(tests, &testCase{
		_id:       "tests-dir",
		_expected: true,

		filePath: "cmd/tests/test_apply_plan.sh",
	})

	// plain-markdown
	tests = append(tests, &testCase{
		_id:       "plain-markdown",
		_expected: false,

		filePath: "docs/testing.md",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, IsTestFile(test.filePath))
		})
	}
}

func TestBuildTestPairs(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *TestPairs

		changed []string
		tracked []string
	}

	tests := make([]*testCase, 0)

	// go-stem-partner
	tests = append(tests, &testCase{
		_id: "go-stem-partner",
		_expected: &TestPairs{
			Partners: map[string][]string{"pkg/a_test.go": {"pkg/a.go"}},
			Tests:    []string{"pkg/a_test.go"},
		},

		changed: []string{"pkg/a_test.go"},
		tracked: []string{"pkg/a.go", "pkg/a_test.go", "pkg/b.go"},
	})

	// go-source-to-test
	tests = append(tests, &testCase{
		_id: "go-source-to-test",
		_expected: &TestPairs{
			Partners: map[string][]string{"pkg/a.go": {"pkg/a_test.go"}},
			Tests:    []string{},
		},

		changed: []string{"pkg/a.go"},
		tracked: []string{"pkg/a.go", "pkg/a_test.go"},
	})

	// go-package-fallback-changed
	tests = append(tests, &testCase{
		_id: "go-package-fallback-changed",
		_expected: &TestPairs{
			Partners: map[string][]string{"pkg/flow_test.go": {"pkg/b.go"}},
			Tests:    []string{"pkg/flow_test.go"},
		},

		changed: []string{"pkg/flow_test.go", "pkg/b.go", "other/c.go"},
		tracked: []string{"pkg/a.go", "pkg/b.go", "pkg/flow_test.go", "other/c.go"},
	})

	// go-package-fallback-tracked
	tests = append(tests, &testCase{
		_id: "go-package-fallback-tracked",
		_expected: &TestPairs{
			Partners: map[string][]string{"pkg/flow_test.go": {"pkg/a.go", "pkg/b.go"}},
			Tests:    []string{"pkg/flow_test.go"},
		},

		changed: []string{"pkg/flow_test.go"},
		tracked: []string{"pkg/a.go", "pkg/b.go", "pkg/c_test.go", "pkg/flow_test.go", "other/c.go"},
	})

	// go-no-package-files
	tests = append(tests, &testCase{
		_id: "go-no-package-files",
		_expected: &TestPairs{
			Partners: map[string][]string{},
			Tests:    []string{"e2e/flow_test.go"},
		},

		changed: []string{"e2e/flow_test.go"},
		tracked: []string{"e2e/flow_test.go"},
	})

	// python-tests-dir-to-parent
	tests = append(tests, &testCase{
		_id: "python-tests-dir-to-parent",
		_expected: &TestPairs{
			Partners: map[string][]string{"app/tests/test_views.py": {"app/views.py"}},
			Tests:    []string{"app/tests/test_views.py"},
		},

		changed: []string{"app/tests/test_views.py"},
		tracked: []string{"app/views.py", "app/tests/test_views.py"},
	})

	// python-root-tests-dir
	tests = append(tests, &testCase{
		_id: "python-root-tests-dir",
		_expected: &TestPairs{
			Partners: map[string][]string{"tests/test_views.py": {"views.py"}},
			Tests:    []string{"tests/test_views.py"},
		},

		changed: []string{"tests/test_views.py"},
		tracked: []string{"views.py", "tests/test_views.py"},
	})

	// ts-dunder-tests
	tests = append(tests, &testCase{
		_id: "ts-dunder-tests",
		_expected: &TestPairs{
			Partners: map[string][]string{"web/__tests__/button.test.tsx": {"web/button.tsx"}},
			Tests:    []string{"web/__tests__/button.test.tsx"},
		},

		changed: []string{"web/__tests__/button.test.tsx"},
		tracked: []string{"web/button.tsx", "web/__tests__/button.test.tsx"},
	})

	// testdata-no-partner
	tests = append(tests, &testCase{
		_id: "testdata-no-partner",
		_expected: &TestPairs{
			Partners: map[string][]string{},
			Tests:    []string{"pkg/testdata/case.json"},
		},

		changed: []string{"pkg/testdata/case.json"},
		tracked: []string{"pkg/a.go", "pkg/testdata/case.json"},
	})

	// production-without-test
	tests = append(tests, &testCase{
		_id: "production-without-test",
		_expected: &TestPairs{
			Partners: map[string][]string{},
			Tests:    []string{},
		},

		changed: []string{"pkg/a.go"},
		tracked: []string{"pkg/a.go"},
	})

	// empty-diff
	tests = append(tests, &testCase{
		_id: "empty-diff",
		_expected: &TestPairs{
			Partners: map[string][]string{},
			Tests:    []string{},
		},

		changed: []string{},
		tracked: []string{"pkg/a.go"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, BuildTestPairs(test.changed, test.tracked))
		})
	}
}

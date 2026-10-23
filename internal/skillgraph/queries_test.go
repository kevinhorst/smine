package skillgraph

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadQueries(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   []Query
		_shouldPass bool

		dir string
	}

	emptyDir := t.TempDir()

	sortedDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(sortedDir, "queries"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sortedDir, "queries", "b-second.json"), []byte(`{"assume": ["x"], "expect": "sat", "reach": "SKILL-X-RUN-001"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(sortedDir, "queries", "a-first.json"), []byte(`{"name": "first", "assume": [], "check": "x", "expect": "unsat"}`), 0o644))

	malformedDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(malformedDir, "queries"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(malformedDir, "queries", "bad.json"), []byte(`{"assume": `), 0o644))

	tests := make([]*testCase, 0)

	// no-dir
	tests = append(tests, &testCase{
		_id:         "no-dir",
		_expected:   nil,
		_shouldPass: true,

		dir: emptyDir,
	})

	// two-files-sorted
	tests = append(tests, &testCase{
		_id: "two-files-sorted",
		_expected: []Query{
			{Assume: []string{}, Check: "x", Expect: "unsat", Name: "first"},
			{Assume: []string{"x"}, Expect: "sat", Name: "b-second", Reach: "SKILL-X-RUN-001"},
		},
		_shouldPass: true,

		dir: sortedDir,
	})

	// fail-malformed
	tests = append(tests, &testCase{
		_id:         "fail-malformed",
		_expected:   nil,
		_shouldPass: false,

		dir: malformedDir,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			queries, err := LoadQueries(test.dir)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, queries)
		})
	}
}

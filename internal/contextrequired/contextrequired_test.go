package contextrequired

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLineChunks(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   []LineChunk
		_shouldPass bool

		budget int
		end    int
		path   string
		start  int
	}

	dir := t.TempDir()
	short := provideLinesFile(t, dir, "short.md", 10, 100)
	long := provideLinesFile(t, dir, "long.md", 3, ReadBudgetBytes-1)
	wide := provideLinesFile(t, dir, "wide.md", 2, ReadBudgetBytes+1)

	tests := make([]*testCase, 0)

	// pass-one-chunk
	tests = append(tests, &testCase{_id: "pass-one-chunk", _expected: []LineChunk{{End: 10, Start: 1}}, _shouldPass: true, budget: ReadBudgetBytes, end: 10, path: short, start: 1})

	// pass-split-by-budget
	tests = append(tests, &testCase{_id: "pass-split-by-budget", _expected: []LineChunk{{End: 1, Start: 1}, {End: 2, Start: 2}, {End: 3, Start: 3}}, _shouldPass: true, budget: ReadBudgetBytes, end: 3, path: long, start: 1})

	// pass-split-by-given-budget
	tests = append(tests, &testCase{_id: "pass-split-by-given-budget", _expected: []LineChunk{{End: 4, Start: 1}, {End: 8, Start: 5}, {End: 10, Start: 9}}, _shouldPass: true, budget: 400, end: 10, path: short, start: 1})

	// pass-oversized-line-own-chunk
	tests = append(tests, &testCase{_id: "pass-oversized-line-own-chunk", _expected: []LineChunk{{End: 1, Start: 1}, {End: 2, Start: 2}}, _shouldPass: true, budget: ReadBudgetBytes, end: 2, path: wide, start: 1})

	// pass-subrange
	tests = append(tests, &testCase{_id: "pass-subrange", _expected: []LineChunk{{End: 7, Start: 4}}, _shouldPass: true, budget: ReadBudgetBytes, end: 7, path: short, start: 4})

	// pass-end-past-eof-clipped
	tests = append(tests, &testCase{_id: "pass-end-past-eof-clipped", _expected: []LineChunk{{End: 10, Start: 1}}, _shouldPass: true, budget: ReadBudgetBytes, end: 40, path: short, start: 1})

	// pass-start-past-eof-unchanged
	tests = append(tests, &testCase{_id: "pass-start-past-eof-unchanged", _expected: []LineChunk{{End: 30, Start: 20}}, _shouldPass: true, budget: ReadBudgetBytes, end: 30, path: short, start: 20})

	// fail-missing-file
	tests = append(tests, &testCase{_id: "fail-missing-file", _expected: nil, _shouldPass: false, budget: ReadBudgetBytes, end: 1, path: filepath.Join(dir, "none.md"), start: 1})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			chunks, err := LineChunks(test.end, test.path, test.start, test.budget)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, chunks)
		})
	}
}

func TestReadInstructions(t *testing.T) {
	dir := t.TempDir()
	short := provideLinesFile(t, dir, "short.md", 10, 100)
	missing := filepath.Join(dir, "none.md")

	assert.Equal(t, []string{"Read " + short + " offset 1 limit 10"}, ReadInstructions(10, short, 1, ReadBudgetBytes))
	assert.Equal(t, []string{"Read " + missing + " offset 3 limit 5"}, ReadInstructions(7, missing, 3, ReadBudgetBytes))
}

func provideLinesFile(t *testing.T, dir string, name string, lines int, bytesPerLine int) string {
	t.Helper()

	line := strings.Repeat("x", bytesPerLine-1) + "\n"
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(strings.Repeat(line, lines)), 0o600)
	require.NoError(t, err)

	return path
}

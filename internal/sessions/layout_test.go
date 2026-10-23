package sessions

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWeekKey(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		moment time.Time
	}

	tests := make([]*testCase, 0)

	// mid-week-utc
	tests = append(tests, &testCase{
		_id:       "mid-week-utc",
		_expected: "2026-W39",

		moment: time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC),
	})

	// sunday-late-utc-plus-offset
	tests = append(tests, &testCase{
		_id:       "sunday-late-utc-plus-offset",
		_expected: "2026-W39",

		moment: time.Date(2026, 9, 28, 1, 0, 0, 0, time.FixedZone("CEST", 2*3600)),
	})

	// iso-year-boundary-2026-12-31
	tests = append(tests, &testCase{
		_id:       "iso-year-boundary-2026-12-31",
		_expected: "2026-W53",

		moment: time.Date(2026, 12, 31, 12, 0, 0, 0, time.UTC),
	})

	// iso-year-boundary-2027-01-04
	tests = append(tests, &testCase{
		_id:       "iso-year-boundary-2027-01-04",
		_expected: "2027-W01",

		moment: time.Date(2027, 1, 4, 12, 0, 0, 0, time.UTC),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, WeekKey(test.moment))
		})
	}
}

func TestWeekStart(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   time.Time
		_shouldPass bool

		key string
	}

	tests := make([]*testCase, 0)

	// pass-2026-W39
	tests = append(tests, &testCase{
		_id:         "pass-2026-W39",
		_expected:   time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC),
		_shouldPass: true,

		key: "2026-W39",
	})

	// pass-2026-W01
	tests = append(tests, &testCase{
		_id:         "pass-2026-W01",
		_expected:   time.Date(2025, 12, 29, 0, 0, 0, 0, time.UTC),
		_shouldPass: true,

		key: "2026-W01",
	})

	// fail-no-w
	tests = append(tests, &testCase{
		_id:         "fail-no-w",
		_expected:   time.Time{},
		_shouldPass: false,

		key: "2026-39",
	})

	// fail-garbage
	tests = append(tests, &testCase{
		_id:         "fail-garbage",
		_expected:   time.Time{},
		_shouldPass: false,

		key: "garbage",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			start, err := WeekStart(test.key)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, start)
		})
	}
}

func TestRecordFiles(t *testing.T) {
	t.Run("missing-root-empty", func(t *testing.T) {
		files, err := RecordFiles(filepath.Join(t.TempDir(), "none"))
		require.NoError(t, err)
		assert.Empty(t, files)
	})

	t.Run("nested-weeks-archived-included-md-ignored", func(t *testing.T) {
		root := t.TempDir()
		writeRecord(t, root, "go", weekNewer, "a.json", "{}")
		writeRecord(t, root, "go", weekNewer, "z.md", "x")
		writeRecord(t, root, "peek-mcp", weekOlder, "b.json", "{}")
		writeRecord(t, root, filepath.Join("archived", "old"), weekOlder, "c.json", "{}")

		files, err := RecordFiles(root)
		require.NoError(t, err)
		expected := map[string]string{
			"a": filepath.Join(root, "go", weekNewer, "a.json"),
			"b": filepath.Join(root, "peek-mcp", weekOlder, "b.json"),
			"c": filepath.Join(root, "archived", "old", weekOlder, "c.json"),
		}
		assert.Equal(t, expected, files)
	})
}

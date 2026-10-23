package heartbeat

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixtureInstance = "0199a1b2-c3d4-7e5f-8a6b-7c8d9e0f1a2b"

// writeArchive stores the fixture archive as file name, with its instance
// and dissolve time replaced.
func writeArchive(t *testing.T, dir, name, instance, dissolvedAt string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "archive_fleet.json"))
	require.NoError(t, err)
	content := strings.Replace(string(data), fixtureInstance, instance, 1)
	content = strings.Replace(content, `"dissolved_at": "2026-09-28T11:00:00Z"`, `"dissolved_at": "`+dissolvedAt+`"`, 1)
	require.NoError(t, os.WriteFile(filepath.Join(dir, name+".json"), []byte(content), 0o600))
}

func archiveInstances(fleets []ArchivedFleet) []string {
	instances := make([]string, 0, len(fleets))
	for _, fleet := range fleets {
		instances = append(instances, fleet.Instance)
	}
	return instances
}

func TestOpenArchive(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedInstances []string
		_shouldPass        bool

		dir string
	}

	older := "0199a1b2-0000-7000-8000-000000000001"
	newer := "0199a1b2-0000-7000-8000-000000000002"
	tests := make([]*testCase, 0)

	// missing-dir
	tests = append(tests, &testCase{
		_id:                "missing-dir",
		_expectedInstances: []string{},
		_shouldPass:        true,

		dir: filepath.Join(t.TempDir(), "absent"),
	})

	// two-archives-newest-first
	twoDir := t.TempDir()
	writeArchive(t, twoDir, older, older, "2026-09-28T11:00:00Z")
	writeArchive(t, twoDir, newer, newer, "2026-09-28T12:00:00Z")
	tests = append(tests, &testCase{
		_id:                "two-archives-newest-first",
		_expectedInstances: []string{newer, older},
		_shouldPass:        true,

		dir: twoDir,
	})

	// file-name-mismatch
	mismatchDir := t.TempDir()
	writeArchive(t, mismatchDir, older, newer, "2026-09-28T11:00:00Z")
	tests = append(tests, &testCase{
		_id:                "file-name-mismatch",
		_expectedInstances: []string{},
		_shouldPass:        false,

		dir: mismatchDir,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			fleets, err := OpenArchive(test.dir)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedInstances, archiveInstances(fleets))
		})
	}
}

func TestReadArchive(t *testing.T) {
	type testCase struct {
		_id                string
		_isInvalidInstance bool
		_isUnknownArchive  bool
		_shouldPass        bool

		instance string
	}

	dir := t.TempDir()
	writeArchive(t, dir, fixtureInstance, fixtureInstance, "2026-09-28T11:00:00Z")
	tests := make([]*testCase, 0)

	// invalid-id
	tests = append(tests, &testCase{
		_id:                "invalid-id",
		_isInvalidInstance: true,
		_isUnknownArchive:  false,
		_shouldPass:        false,

		instance: "../x",
	})

	// absent
	tests = append(tests, &testCase{
		_id:                "absent",
		_isInvalidInstance: false,
		_isUnknownArchive:  true,
		_shouldPass:        false,

		instance: "0199a1b2-0000-7000-8000-00000000ffff",
	})

	// present
	tests = append(tests, &testCase{
		_id:                "present",
		_isInvalidInstance: false,
		_isUnknownArchive:  false,
		_shouldPass:        true,

		instance: fixtureInstance,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			archived, err := ReadArchive(dir, test.instance)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._isInvalidInstance, errors.Is(err, ErrInvalidInstance))
			assert.Equal(t, test._isUnknownArchive, errors.Is(err, ErrUnknownArchive))
			if test._shouldPass {
				assert.Equal(t, test.instance, archived.Instance)
			}
		})
	}
}

func TestReadArchiveDecodesEveryField(t *testing.T) {
	dir := t.TempDir()
	writeArchive(t, dir, fixtureInstance, fixtureInstance, "2026-09-28T11:00:00Z")
	archived, err := ReadArchive(dir, fixtureInstance)
	require.NoError(t, err)

	assert.Equal(t, "T1", archived.Fleet)
	assert.Equal(t, "0199a1b2", archived.ShortInstance())
	assert.Equal(t, "claude-orch/T1", archived.MergeBranch)
	assert.True(t, archived.Synced)
	require.Len(t, archived.Members, 2)
	assert.Equal(t, ArchivedMember{
		Branch:    "claude/w1",
		Delivered: true,
		JoinedAt:  "2026-09-28T10:00:00Z",
		LeftAt:    "2026-09-28T10:50:00Z",
		SessionId: "w1",
		Token:     "T1-MA",
		Via:       ViaJoined,
	}, archived.Members[1])
	require.Len(t, archived.Subscriptions, 1)
	assert.Equal(t, "ended", archived.Subscriptions[0].State)
	require.Len(t, archived.Journal, 1)
	assert.Equal(t, "merged claude/w1", archived.Journal[0].Subject)
}

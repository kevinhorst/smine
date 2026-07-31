package routines

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.test.demo</string>
	<key>ProgramArguments</key>
	<array>
		<string>/bin/bash</string>
		<string>/tmp/run.sh</string>
	</array>
	<key>StartCalendarInterval</key>
	<dict>
		<key>Hour</key>
		<integer>6</integer>
		<key>Minute</key>
		<integer>0</integer>
	</dict>
</dict>
</plist>
`

func writeRoutine(t *testing.T, dir, name string) string {
	t.Helper()
	routineDir := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(routineDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, "run.sh"), []byte("#!/bin/sh\n"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, "com.test.demo.plist"), []byte(plistTemplate), 0o644))
	return routineDir
}

func intPtr(v int) *int { return &v }

func TestScanConformantRoutine(t *testing.T) {
	dir := t.TempDir()
	writeRoutine(t, dir, "demo")

	routines, err := Scan(dir)
	require.NoError(t, err)
	require.Len(t, routines, 1)

	routine := routines[0]
	assert.Empty(t, routine.LoadError)
	assert.Equal(t, "demo", routine.Name)
	assert.Equal(t, "com.test.demo", routine.Label)
	assert.True(t, routine.ScheduleSupported)
	require.Len(t, routine.Schedule, 1)
	assert.Equal(t, 6, *routine.Schedule[0].Hour)
	assert.False(t, routine.NextRun.IsZero())
}

func TestScanMissingRunShDegrades(t *testing.T) {
	dir := t.TempDir()
	routineDir := writeRoutine(t, dir, "demo")
	require.NoError(t, os.Remove(filepath.Join(routineDir, "run.sh")))

	routines, err := Scan(dir)
	require.NoError(t, err)
	require.Len(t, routines, 1)
	assert.Equal(t, "missing run.sh", routines[0].LoadError)
}

func TestScanTwoPlistsDegrades(t *testing.T) {
	dir := t.TempDir()
	routineDir := writeRoutine(t, dir, "demo")
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, "second.plist"), []byte(plistTemplate), 0o644))

	routines, err := Scan(dir)
	require.NoError(t, err)
	require.Len(t, routines, 1)
	assert.Contains(t, routines[0].LoadError, "exactly one .plist")
}

func TestScanSkipsUnderscoreDirs(t *testing.T) {
	dir := t.TempDir()
	writeRoutine(t, dir, "_templates")

	routines, err := Scan(dir)
	require.NoError(t, err)
	assert.Empty(t, routines)
}

func TestScanMissingDirIsEmpty(t *testing.T) {
	routines, err := Scan(filepath.Join(t.TempDir(), "nope"))
	require.NoError(t, err)
	assert.Empty(t, routines)
}

func TestDuplicate(t *testing.T) {
	dir := t.TempDir()
	routineDir := filepath.Join(dir, "demo")
	require.NoError(t, os.MkdirAll(routineDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, "run.sh"), []byte("#!/bin/sh\necho demo\n"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, "results.jsonl"), []byte("{}\n"), 0o644))
	content := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
	<key>Label</key><string>com.test.routine.demo</string>
	<key>ProgramArguments</key><array><string>/bin/bash</string><string>` + routineDir + `/run.sh</string></array>
	<key>StartCalendarInterval</key><dict><key>Hour</key><integer>6</integer><key>Minute</key><integer>0</integer></dict>
	<key>StandardOutPath</key><string>/tmp/claude-routine-demo.out.log</string>
	<key>StandardErrorPath</key><string>/tmp/claude-routine-demo.err.log</string>
</dict></plist>`
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, "com.test.routine.demo.plist"), []byte(content), 0o644))

	require.NoError(t, Duplicate("demo", "demo-second", dir))

	copies, err := Scan(dir)
	require.NoError(t, err)
	require.Len(t, copies, 2)
	duplicated := copies[1]
	assert.Equal(t, "demo-second", duplicated.Name)
	assert.Empty(t, duplicated.LoadError)
	assert.Equal(t, "com.test.routine.demo-second", duplicated.Label)

	wrapper, err := os.ReadFile(duplicated.WrapperPath)
	require.NoError(t, err)
	assert.Contains(t, string(wrapper), "echo demo")
	plistData, err := os.ReadFile(duplicated.PlistPath)
	require.NoError(t, err)
	assert.Contains(t, string(plistData), filepath.Join(dir, "demo-second", "run.sh"))
	assert.Contains(t, string(plistData), "/tmp/claude-routine-demo-second.out.log")
	assert.Contains(t, string(plistData), "/tmp/claude-routine-demo-second.err.log")
	assert.NotContains(t, string(plistData), "/tmp/claude-routine-demo.out.log")
	_, err = os.Stat(filepath.Join(dir, "demo-second", "results.jsonl"))
	assert.True(t, os.IsNotExist(err))

	require.Error(t, Duplicate("demo", "demo-second", dir))
	require.Error(t, Duplicate("demo", "Bad_Name", dir))
}

// A plist whose ProgramArguments does not reference the source directory would
// leave the copy running the source wrapper — both instances would then share
// .lock, results.jsonl and .cadence-stamp.
func TestDuplicateRejectsUnrewritablePlist(t *testing.T) {
	dir := t.TempDir()
	routineDir := filepath.Join(dir, "demo")
	require.NoError(t, os.MkdirAll(routineDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, "run.sh"), []byte("#!/bin/sh\n"), 0o755))
	content := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
	<key>Label</key><string>com.test.routine.demo</string>
	<key>ProgramArguments</key><array><string>/bin/bash</string><string>$HOME/elsewhere/run.sh</string></array>
</dict></plist>`
	require.NoError(t, os.WriteFile(filepath.Join(routineDir, "com.test.routine.demo.plist"), []byte(content), 0o644))

	err := Duplicate("demo", "demo-second", dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "would run the source wrapper")
	_, statErr := os.Stat(filepath.Join(dir, "demo-second"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "results.jsonl")
	lines := `{"timestamp": "2026-07-10T06:00:00Z", "exit_status": 0, "session_id": "a", "num_turns": 3, "total_cost_usd": 0.1}
not json
{"timestamp": "2026-07-11T06:00:00Z", "exit_status": 1, "session_id": "b", "num_turns": 5, "total_cost_usd": 0.2}
`
	require.NoError(t, os.WriteFile(path, []byte(lines), 0o644))

	results, err := History(path, 10)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "b", results[0].SessionId)
	assert.Equal(t, "a", results[1].SessionId)

	limited, err := History(path, 1)
	require.NoError(t, err)
	require.Len(t, limited, 1)
	assert.Equal(t, "b", limited[0].SessionId)
}

func TestHistoryMissingFileIsNil(t *testing.T) {
	results, err := History(filepath.Join(t.TempDir(), "nope.jsonl"), 10)
	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestNextRunDaily(t *testing.T) {
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.Local)
	schedule := []CalendarInterval{{Hour: intPtr(6), Minute: intPtr(30)}}

	next, ok := NextRun(schedule, now)
	require.True(t, ok)
	assert.Equal(t, time.Date(2026, 7, 15, 6, 30, 0, 0, time.Local), next)
}

func TestNextRunWeekdaySundayAliases(t *testing.T) {
	// 2026-07-14 is a Tuesday; next Sunday is 2026-07-19.
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.Local)
	for _, weekday := range []int{0, 7} {
		schedule := []CalendarInterval{{Hour: intPtr(6), Minute: intPtr(0), Weekday: intPtr(weekday)}}
		next, ok := NextRun(schedule, now)
		require.True(t, ok)
		assert.Equal(t, time.Date(2026, 7, 19, 6, 0, 0, 0, time.Local), next, "weekday %d", weekday)
	}
}

func TestNextRunMultiIntervalEarliestWins(t *testing.T) {
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.Local)
	schedule := []CalendarInterval{
		{Hour: intPtr(18), Minute: intPtr(0)},
		{Hour: intPtr(12), Minute: intPtr(0)},
	}

	next, ok := NextRun(schedule, now)
	require.True(t, ok)
	assert.Equal(t, time.Date(2026, 7, 14, 12, 0, 0, 0, time.Local), next)
}

func TestNextRunUnsatisfiable(t *testing.T) {
	now := time.Date(2026, 7, 14, 10, 0, 0, 0, time.Local)
	schedule := []CalendarInterval{{Day: intPtr(31), Month: intPtr(2), Hour: intPtr(0), Minute: intPtr(0)}}

	_, ok := NextRun(schedule, now)
	assert.False(t, ok)
}

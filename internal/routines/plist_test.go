package routines

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"howett.net/plist"
)

func writePlist(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.plist")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

const plistHeader = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
`

func TestParseScheduleSingleDict(t *testing.T) {
	path := writePlist(t, plistHeader+`<plist version="1.0"><dict>
		<key>Label</key><string>demo</string>
		<key>StartCalendarInterval</key>
		<dict><key>Hour</key><integer>6</integer><key>Minute</key><integer>15</integer></dict>
	</dict></plist>`)

	schedule, err := parseSchedule(path)
	require.NoError(t, err)
	assert.True(t, schedule.Supported)
	require.Len(t, schedule.Intervals, 1)
	assert.Equal(t, 6, *schedule.Intervals[0].Hour)
	assert.Equal(t, 15, *schedule.Intervals[0].Minute)
}

func TestParseScheduleArrayOfDicts(t *testing.T) {
	path := writePlist(t, plistHeader+`<plist version="1.0"><dict>
		<key>StartCalendarInterval</key>
		<array>
			<dict><key>Hour</key><integer>6</integer></dict>
			<dict><key>Hour</key><integer>18</integer><key>Weekday</key><integer>1</integer></dict>
		</array>
	</dict></plist>`)

	schedule, err := parseSchedule(path)
	require.NoError(t, err)
	assert.True(t, schedule.Supported)
	require.Len(t, schedule.Intervals, 2)
	assert.Equal(t, 18, *schedule.Intervals[1].Hour)
	assert.Equal(t, 1, *schedule.Intervals[1].Weekday)
}

func TestParseScheduleUnknownKeyFails(t *testing.T) {
	path := writePlist(t, plistHeader+`<plist version="1.0"><dict>
		<key>StartCalendarInterval</key>
		<dict><key>Second</key><integer>30</integer></dict>
	</dict></plist>`)

	_, err := parseSchedule(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Unknown key Second")
}

func TestParseScheduleMissingKeyIsUnscheduled(t *testing.T) {
	path := writePlist(t, plistHeader+`<plist version="1.0"><dict>
		<key>Label</key><string>demo</string>
	</dict></plist>`)

	schedule, err := parseSchedule(path)
	require.NoError(t, err)
	assert.False(t, schedule.Supported)
	assert.True(t, schedule.Unscheduled)
	assert.Nil(t, schedule.Intervals)
}

func TestParseScheduleOtherTriggerIsUnsupported(t *testing.T) {
	path := writePlist(t, plistHeader+`<plist version="1.0"><dict>
		<key>Label</key><string>demo</string>
		<key>StartInterval</key><integer>3600</integer>
	</dict></plist>`)

	schedule, err := parseSchedule(path)
	require.NoError(t, err)
	assert.False(t, schedule.Supported)
	assert.False(t, schedule.Unscheduled)
	assert.Nil(t, schedule.Intervals)
}

func TestParseEnv(t *testing.T) {
	path := writePlist(t, plistHeader+`<plist version="1.0"><dict>
		<key>Label</key><string>demo</string>
		<key>EnvironmentVariables</key>
		<dict><key>ROUTINE_TARGET_REPO</key><string>/repo</string></dict>
	</dict></plist>`)

	env, err := parseEnv(path)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"ROUTINE_TARGET_REPO": "/repo"}, env)

	bare := writePlist(t, plistHeader+`<plist version="1.0"><dict>
		<key>Label</key><string>demo</string>
	</dict></plist>`)
	env, err = parseEnv(bare)
	require.NoError(t, err)
	assert.Nil(t, env)
}

func TestSetEnvPreservesUnrelatedKeys(t *testing.T) {
	path := writePlist(t, plistHeader+`<plist version="1.0"><dict>
		<key>Label</key><string>com.test.demo</string>
		<key>ProgramArguments</key><array><string>/bin/bash</string><string>/tmp/run.sh</string></array>
		<key>RunAtLoad</key><false/>
		<key>StartCalendarInterval</key>
		<dict><key>Hour</key><integer>6</integer><key>Minute</key><integer>0</integer></dict>
	</dict></plist>`)

	require.NoError(t, SetEnv(map[string]string{"ROUTINE_CADENCE_DAYS": "2"}, path))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var content map[string]any
	_, err = plist.Unmarshal(data, &content)
	require.NoError(t, err)

	assert.Equal(t, "com.test.demo", content["Label"])
	assert.Equal(t, []any{"/bin/bash", "/tmp/run.sh"}, content["ProgramArguments"])
	assert.Equal(t, false, content["RunAtLoad"])
	schedule, ok := content["StartCalendarInterval"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, uint64(6), schedule["Hour"])
	env, ok := content["EnvironmentVariables"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "2", env["ROUTINE_CADENCE_DAYS"])

	require.NoError(t, SetEnv(nil, path))
	data, err = os.ReadFile(path)
	require.NoError(t, err)
	content = nil
	_, err = plist.Unmarshal(data, &content)
	require.NoError(t, err)
	_, hasEnv := content["EnvironmentVariables"]
	assert.False(t, hasEnv)

	_, err = os.Stat(path + ".tmp")
	assert.True(t, os.IsNotExist(err), "tmp file left behind")
}

func TestReschedulePreservesUnrelatedKeys(t *testing.T) {
	path := writePlist(t, plistHeader+`<plist version="1.0"><dict>
		<key>Label</key><string>com.test.demo</string>
		<key>ProgramArguments</key><array><string>/bin/bash</string><string>/tmp/run.sh</string></array>
		<key>RunAtLoad</key><false/>
		<key>StartCalendarInterval</key>
		<dict><key>Hour</key><integer>6</integer><key>Minute</key><integer>0</integer></dict>
	</dict></plist>`)

	interval := CalendarInterval{Hour: intPtr(9), Minute: intPtr(45), Weekday: intPtr(1)}
	require.NoError(t, Reschedule(interval, path))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var content map[string]any
	_, err = plist.Unmarshal(data, &content)
	require.NoError(t, err)

	assert.Equal(t, "com.test.demo", content["Label"])
	assert.Equal(t, []any{"/bin/bash", "/tmp/run.sh"}, content["ProgramArguments"])
	assert.Equal(t, false, content["RunAtLoad"])

	schedule, ok := content["StartCalendarInterval"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, uint64(9), schedule["Hour"])
	assert.Equal(t, uint64(45), schedule["Minute"])
	assert.Equal(t, uint64(1), schedule["Weekday"])

	_, err = os.Stat(path + ".tmp")
	assert.True(t, os.IsNotExist(err), "tmp file left behind")
}

func TestRescheduleAddsMissingKey(t *testing.T) {
	path := writePlist(t, plistHeader+`<plist version="1.0"><dict>
		<key>Label</key><string>com.test.demo</string>
		<key>ProgramArguments</key><array><string>/bin/bash</string><string>/tmp/run.sh</string></array>
	</dict></plist>`)

	interval := CalendarInterval{Hour: intPtr(7), Minute: intPtr(30)}
	require.NoError(t, Reschedule(interval, path))

	schedule, err := parseSchedule(path)
	require.NoError(t, err)
	assert.True(t, schedule.Supported)
	assert.False(t, schedule.Unscheduled)
	require.Len(t, schedule.Intervals, 1)
	assert.Equal(t, 7, *schedule.Intervals[0].Hour)
	assert.Equal(t, 30, *schedule.Intervals[0].Minute)
}

func TestPlistMetaLabelAndEnv(t *testing.T) {
	path := writePlist(t, plistHeader+`<plist version="1.0"><dict>
		<key>Label</key><string>com.test.meta</string>
		<key>EnvironmentVariables</key>
		<dict><key>ROUTINE_GROUP</key><string>demo</string></dict>
	</dict></plist>`)

	label, env, err := PlistMeta(filepath.Dir(path))
	require.NoError(t, err)
	assert.Equal(t, "com.test.meta", label)
	assert.Equal(t, map[string]string{"ROUTINE_GROUP": "demo"}, env)
}

func TestPlistMetaRequiresExactlyOnePlist(t *testing.T) {
	dir := t.TempDir()
	_, _, err := PlistMeta(dir)
	require.ErrorContains(t, err, "exactly one plist")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.plist"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.plist"), []byte("x"), 0o644))
	_, _, err = PlistMeta(dir)
	require.ErrorContains(t, err, "exactly one plist")
}

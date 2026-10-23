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

func TestParseScheduleErrors(t *testing.T) {
	type testCase struct {
		_id          string
		_expectedErr string

		path string
	}

	tests := make([]*testCase, 0)

	// missing-file
	tests = append(tests, &testCase{
		_id:          "missing-file",
		_expectedErr: "Failed to read",

		path: filepath.Join(t.TempDir(), "absent.plist"),
	})

	// malformed-plist
	tests = append(tests, &testCase{
		_id:          "malformed-plist",
		_expectedErr: "Failed to parse",

		path: writePlist(t, plistHeader+`<plist version="1.0"><dict><key>Label</key>`),
	})

	// unknown-key
	tests = append(tests, &testCase{
		_id:          "unknown-key",
		_expectedErr: "Unknown key Second",

		path: writePlist(t, plistHeader+`<plist version="1.0"><dict>
			<key>StartCalendarInterval</key>
			<dict><key>Second</key><integer>30</integer></dict>
		</dict></plist>`),
	})

	// non-integer-value
	tests = append(tests, &testCase{
		_id:          "non-integer-value",
		_expectedErr: "Unexpected value type string",

		path: writePlist(t, plistHeader+`<plist version="1.0"><dict>
			<key>StartCalendarInterval</key>
			<dict><key>Hour</key><string>six</string></dict>
		</dict></plist>`),
	})

	// interval-wrong-type
	tests = append(tests, &testCase{
		_id:          "interval-wrong-type",
		_expectedErr: "Unexpected StartCalendarInterval type string",

		path: writePlist(t, plistHeader+`<plist version="1.0"><dict>
			<key>StartCalendarInterval</key><string>daily</string>
		</dict></plist>`),
	})

	// array-item-not-dict
	tests = append(tests, &testCase{
		_id:          "array-item-not-dict",
		_expectedErr: "Unexpected interval type uint64",

		path: writePlist(t, plistHeader+`<plist version="1.0"><dict>
			<key>StartCalendarInterval</key>
			<array><integer>6</integer></array>
		</dict></plist>`),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			_, err := parseSchedule(test.path)

			assert.ErrorContains(t, err, test._expectedErr)
		})
	}
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

func TestValidateEnvKeys(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool

		env map[string]string
	}

	tests := make([]*testCase, 0)

	// pass-runner-knobs
	tests = append(tests, &testCase{
		_id:         "pass-runner-knobs",
		_shouldPass: true,

		env: map[string]string{"ROUTINE_MODEL": "sonnet", "SMINE_APPLY_CAP": "3"},
	})

	// pass-empty
	tests = append(tests, &testCase{
		_id:         "pass-empty",
		_shouldPass: true,

		env: nil,
	})

	// fail-skill-input
	tests = append(tests, &testCase{
		_id:         "fail-skill-input",
		_shouldPass: false,

		env: map[string]string{"ROUTINE_MODEL": "sonnet", "SKILLEVAL_TARGET_REPO": "/repo"},
	})

	// fail-harness-key
	tests = append(tests, &testCase{
		_id:         "fail-harness-key",
		_shouldPass: false,

		env: map[string]string{"CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS": "64"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := ValidateEnvKeys(test.env)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
		})
	}
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

func TestPlistMetaRejectsForeignEnvKey(t *testing.T) {
	path := writePlist(t, plistHeader+`<plist version="1.0"><dict>
		<key>Label</key><string>com.test.meta</string>
		<key>EnvironmentVariables</key>
		<dict><key>SKILLEVAL_TARGET_REPO</key><string>/repo</string></dict>
	</dict></plist>`)

	_, _, err := PlistMeta(filepath.Dir(path))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SKILLEVAL_TARGET_REPO")
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

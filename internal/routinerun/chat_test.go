package routinerun

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/routines"
)

// discardWriteCloser stands in for the child's stdin in pump unit tests.
type discardWriteCloser struct {
	written []string
}

func (d *discardWriteCloser) Close() error {
	return nil
}

func (d *discardWriteCloser) Write(data []byte) (int, error) {
	d.written = append(d.written, string(data))
	return len(data), nil
}

func chatSpec(t *testing.T, complete CompleteMode) *ChatSpec {
	t.Helper()
	spec := &ChatSpec{
		CellPids:  filepath.Join(t.TempDir(), routines.CellPidsFileName),
		Complete:  complete,
		Dir:       t.TempDir(),
		ExitGrace: 2 * time.Second,
		Poll:      20 * time.Millisecond,
		Prompt:    "/demo go",
		SteerDir:  t.TempDir(),
		Timeout:   20 * time.Second,
	}
	if complete == CompleteMarker {
		spec.MarkerPath = filepath.Join(spec.SteerDir, routines.RunCompleteFileName)
	}
	return spec
}

// readDockerLog returns the fake docker's argv log; "" when docker never ran.
func readDockerLog(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ""
	}
	require.NoError(t, err)
	return string(data)
}

func TestRunChat(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedDockerLog string
		_expectedResult    string
		_expectedStatus    int

		attended bool
		command  []string
		complete CompleteMode
		env      map[string]string
		timeout  time.Duration
	}

	tests := make([]*testCase, 0)

	// result-close
	tests = append(tests, &testCase{
		_id:                "result-close",
		_expectedDockerLog: "",
		_expectedResult:    "/demo go",
		_expectedStatus:    exitOk,

		attended: false,
		command:  nil,
		complete: CompleteResult,
		env:      map[string]string{},
		timeout:  20 * time.Second,
	})

	// marker-complete
	tests = append(tests, &testCase{
		_id:                "marker-complete",
		_expectedDockerLog: "",
		_expectedResult:    "/demo go",
		_expectedStatus:    exitOk,

		attended: false,
		command:  nil,
		complete: CompleteMarker,
		env:      map[string]string{fakeMarkerEnv: "MARKER"},
		timeout:  20 * time.Second,
	})

	// nudge-close
	tests = append(tests, &testCase{
		_id:                "nudge-close",
		_expectedDockerLog: "",
		_expectedResult:    "NUDGE",
		_expectedStatus:    exitOk,

		attended: false,
		command:  nil,
		complete: CompleteMarker,
		env:      map[string]string{},
		timeout:  20 * time.Second,
	})

	// child-exit-status
	tests = append(tests, &testCase{
		_id:                "child-exit-status",
		_expectedDockerLog: "",
		_expectedResult:    "",
		_expectedStatus:    3,

		attended: false,
		command:  nil,
		complete: CompleteMarker,
		env:      map[string]string{fakeExitAfterEnv: "1", fakeSilentEnv: "1"},
		timeout:  20 * time.Second,
	})

	// deadline-term
	tests = append(tests, &testCase{
		_id:                "deadline-term",
		_expectedDockerLog: "",
		_expectedResult:    "",
		_expectedStatus:    exitDeadline,

		attended: false,
		command:  nil,
		complete: CompleteResult,
		env:      map[string]string{fakeHangEnv: "1"},
		timeout:  200 * time.Millisecond,
	})

	// headless-env
	tests = append(tests, &testCase{
		_id:                "headless-env",
		_expectedDockerLog: "",
		_expectedResult:    "1",
		_expectedStatus:    exitOk,

		attended: false,
		command:  nil,
		complete: CompleteResult,
		env:      map[string]string{fakeEchoEnv: "SMINE_HEADLESS"},
		timeout:  20 * time.Second,
	})

	// attended-env
	tests = append(tests, &testCase{
		_id:                "attended-env",
		_expectedDockerLog: "",
		_expectedResult:    "",
		_expectedStatus:    exitOk,

		attended: true,
		command:  nil,
		complete: CompleteResult,
		env:      map[string]string{fakeEchoEnv: "SMINE_HEADLESS", "SMINE_HEADLESS": ""},
		timeout:  20 * time.Second,
	})

	// command-prefix
	tests = append(tests, &testCase{
		_id:                "command-prefix",
		_expectedDockerLog: "run --interactive cell-image -p --input-format stream-json --output-format stream-json --verbose\n",
		_expectedResult:    "/demo go",
		_expectedStatus:    exitOk,

		attended: false,
		command:  []string{"docker", "run", "--interactive", "cell-image"},
		complete: CompleteResult,
		env:      map[string]string{},
		timeout:  20 * time.Second,
	})

	// empty-command-runs-claude
	tests = append(tests, &testCase{
		_id:                "empty-command-runs-claude",
		_expectedDockerLog: "",
		_expectedResult:    "/demo go",
		_expectedStatus:    exitOk,

		attended: false,
		command:  []string{},
		complete: CompleteResult,
		env:      map[string]string{},
		timeout:  20 * time.Second,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			installFakeClaude(t)
			dockerLog := installFakeDocker(t)
			spec := chatSpec(t, test.complete)
			spec.Attended = test.attended
			spec.Command = test.command
			spec.Timeout = test.timeout
			for key, value := range test.env {
				if value == "MARKER" {
					value = spec.MarkerPath
				}
				t.Setenv(key, value)
			}
			expectedResult := test._expectedResult
			if expectedResult == "NUDGE" {
				expectedResult = fmt.Sprintf(nudgeMessage, spec.MarkerPath, spec.MarkerPath)
			}

			outcome, err := RunChat(context.Background(), spec)

			require.NoError(t, err)
			assert.Equal(t, test._expectedStatus, outcome.Status)
			assert.Equal(t, expectedResult, outcome.Envelope.Result)
			assert.Equal(t, test._expectedDockerLog, readDockerLog(t, dockerLog))
			assert.FileExists(t, filepath.Join(spec.SteerDir, "stream.jsonl"))
			assert.NotEmpty(t, routines.ReadCellPids(spec.CellPids))
		})
	}
}

func TestRunChatDeliversSteer(t *testing.T) {
	installFakeClaude(t)
	spec := chatSpec(t, CompleteResult)
	spec.CellId = "cell-a"
	steerPath := filepath.Join(spec.SteerDir, routines.SteerPromptFileName)
	require.NoError(t, os.WriteFile(steerPath, []byte("focus on tests\n"), 0o644))

	outcome, err := RunChat(context.Background(), spec)

	require.NoError(t, err)
	assert.Equal(t, exitOk, outcome.Status)
	assert.True(t, strings.HasPrefix(outcome.Envelope.Result, steerPreamble))
	assert.Contains(t, outcome.Envelope.Result, "focus on tests")
	assert.NoFileExists(t, steerPath)

	raw, err := os.ReadFile(filepath.Join(spec.SteerDir, routines.SteerDeliveredFileName))
	require.NoError(t, err)
	var receipt routines.SteerReceipt
	require.NoError(t, json.Unmarshal(raw, &receipt))
	assert.Equal(t, "cell-a", receipt.Cell)
	assert.Equal(t, "focus on tests\n", receipt.Prompt)
	assert.False(t, receipt.Interrupt)
	assert.FileExists(t, filepath.Join(spec.SteerDir, "cell-a", "pump.log"))
}

func idlePump(markerPath string, idleSince time.Time, backgroundTasks int, isNudged bool) *chatPump {
	pump := &chatPump{
		backgroundTasks: backgroundTasks,
		idleSince:       idleSince,
		isNudged:        isNudged,
		markerPath:      markerPath,
		spec:            &ChatSpec{Complete: CompleteMarker, MarkerPath: markerPath, SteerDir: filepath.Dir(markerPath)},
		stdin:           &discardWriteCloser{},
	}
	return pump
}

func TestChatPumpCompletion(t *testing.T) {
	type testCase struct {
		_id               string
		_expected         stopReason
		_expectedIsNudged bool

		pump *chatPump
	}

	now := time.Now()
	tests := make([]*testCase, 0)

	// mid-turn
	tests = append(tests, &testCase{
		_id:               "mid-turn",
		_expected:         reasonNone,
		_expectedIsNudged: false,

		pump: idlePump(filepath.Join(t.TempDir(), routines.RunCompleteFileName), time.Time{}, 0, false),
	})

	// turn-end-nudges
	tests = append(tests, &testCase{
		_id:               "turn-end-nudges",
		_expected:         reasonNone,
		_expectedIsNudged: true,

		pump: idlePump(filepath.Join(t.TempDir(), routines.RunCompleteFileName), now, 0, false),
	})

	// nudged-turn-end-closes
	tests = append(tests, &testCase{
		_id:               "nudged-turn-end-closes",
		_expected:         reasonResultClose,
		_expectedIsNudged: true,

		pump: idlePump(filepath.Join(t.TempDir(), routines.RunCompleteFileName), now, 0, true),
	})

	// background-running
	tests = append(tests, &testCase{
		_id:               "background-running",
		_expected:         reasonNone,
		_expectedIsNudged: false,

		pump: idlePump(filepath.Join(t.TempDir(), routines.RunCompleteFileName), now, 3, false),
	})

	// marker-written
	markerPath := filepath.Join(t.TempDir(), routines.RunCompleteFileName)
	require.NoError(t, os.WriteFile(markerPath, nil, 0o644))
	tests = append(tests, &testCase{
		_id:               "marker-written",
		_expected:         reasonComplete,
		_expectedIsNudged: false,

		pump: idlePump(markerPath, now, 0, false),
	})

	// steer-pending
	steerMarker := filepath.Join(t.TempDir(), routines.RunCompleteFileName)
	require.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(steerMarker), routines.SteerPromptFileName), []byte("go on"), 0o644))
	tests = append(tests, &testCase{
		_id:               "steer-pending",
		_expected:         reasonNone,
		_expectedIsNudged: false,

		pump: idlePump(steerMarker, now, 0, false),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.pump.completion())
			assert.Equal(t, test._expectedIsNudged, test.pump.isNudged)
		})
	}
}

// registrationFiles lists the base names of the container-session files in
// dir; nil when there are none.
func registrationFiles(t *testing.T, dir string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	require.NoError(t, err)

	var names []string
	for _, path := range paths {
		names = append(names, filepath.Base(path))
	}
	return names
}

// registeringPump is a pump at pid 101 whose spec carries registration.
func registeringPump(registration *sessionRegistration) *chatPump {
	pump := &chatPump{
		pid:   101,
		spec:  &ChatSpec{Registration: registration},
		stdin: &discardWriteCloser{},
	}
	return pump
}

// TestChatPumpRegistersSession ticks twice — the second at pid 202, after the
// transcript exists — so a registered pump proves it writes only once.
func TestChatPumpRegistersSession(t *testing.T) {
	type testCase struct {
		_id                   string
		_expectedFirst        []string
		_expectedIsRegistered bool
		_expectedSecond       []string

		event           streamEvent
		pump            *chatPump
		registrationDir string
		transcriptPath  string
	}

	sessionId := "session-1"
	initEvent := streamEvent{kind: "system", sessionId: sessionId, subtype: subtypeInit}
	tests := make([]*testCase, 0)

	// init-then-transcript
	registrationDir := filepath.Join(t.TempDir(), registrationsDir)
	projectsDir := t.TempDir()
	transcriptPath := filepath.Join(projectsDir, "project", sessionId+".jsonl")
	require.NoError(t, os.MkdirAll(filepath.Dir(transcriptPath), 0o755))
	require.NoError(t, os.WriteFile(transcriptPath, nil, 0o644))
	registration := &sessionRegistration{cwd: "/worktree", dir: registrationDir, projectsDir: projectsDir}
	tests = append(tests, &testCase{
		_id:                   "init-then-transcript",
		_expectedFirst:        []string{"101.json"},
		_expectedIsRegistered: true,
		_expectedSecond:       []string{"101.json"},

		event:           initEvent,
		pump:            registeringPump(registration),
		registrationDir: registrationDir,
		transcriptPath:  transcriptPath,
	})

	// init-without-transcript
	registrationDir = filepath.Join(t.TempDir(), registrationsDir)
	projectsDir = t.TempDir()
	registration = &sessionRegistration{cwd: "/worktree", dir: registrationDir, projectsDir: projectsDir}
	tests = append(tests, &testCase{
		_id:                   "init-without-transcript",
		_expectedFirst:        nil,
		_expectedIsRegistered: true,
		_expectedSecond:       []string{"202.json"},

		event:           initEvent,
		pump:            registeringPump(registration),
		registrationDir: registrationDir,
		transcriptPath:  filepath.Join(projectsDir, "project", sessionId+".jsonl"),
	})

	// no-registration
	projectsDir = t.TempDir()
	tests = append(tests, &testCase{
		_id:                   "no-registration",
		_expectedFirst:        nil,
		_expectedIsRegistered: false,
		_expectedSecond:       nil,

		event:           initEvent,
		pump:            registeringPump(nil),
		registrationDir: filepath.Join(t.TempDir(), registrationsDir),
		transcriptPath:  filepath.Join(projectsDir, "project", sessionId+".jsonl"),
	})

	// no-init-event
	registrationDir = filepath.Join(t.TempDir(), registrationsDir)
	projectsDir = t.TempDir()
	registration = &sessionRegistration{cwd: "/worktree", dir: registrationDir, projectsDir: projectsDir}
	tests = append(tests, &testCase{
		_id:                   "no-init-event",
		_expectedFirst:        nil,
		_expectedIsRegistered: false,
		_expectedSecond:       nil,

		event:           streamEvent{kind: "system", sessionId: sessionId, subtype: subtypeBackgroundTasks},
		pump:            registeringPump(registration),
		registrationDir: registrationDir,
		transcriptPath:  filepath.Join(projectsDir, "project", sessionId+".jsonl"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			test.pump.observe(test.event)
			test.pump.registerSession()
			assert.Equal(t, test._expectedFirst, registrationFiles(t, test.registrationDir))

			require.NoError(t, os.MkdirAll(filepath.Dir(test.transcriptPath), 0o755))
			require.NoError(t, os.WriteFile(test.transcriptPath, nil, 0o644))
			test.pump.pid = 202
			test.pump.registerSession()
			assert.Equal(t, test._expectedSecond, registrationFiles(t, test.registrationDir))
			assert.Equal(t, test._expectedIsRegistered, test.pump.isRegistered)
		})
	}
}

func TestChatPumpBackgroundHoldsNudge(t *testing.T) {
	markerPath := filepath.Join(t.TempDir(), routines.RunCompleteFileName)
	pump := idlePump(markerPath, time.Time{}, 0, false)
	pump.observe(streamEvent{kind: "result"})
	pump.observe(streamEvent{backgroundTasks: 2, kind: "system", subtype: subtypeBackgroundTasks})
	assert.Equal(t, reasonNone, pump.completion())
	assert.False(t, pump.isNudged)

	pump.observe(streamEvent{backgroundTasks: 0, kind: "system", subtype: subtypeBackgroundTasks})
	assert.Equal(t, reasonNone, pump.completion())
	assert.False(t, pump.isNudged)

	pump.observe(streamEvent{kind: "result"})
	assert.Equal(t, reasonNone, pump.completion())
	assert.True(t, pump.isNudged)
	assert.Len(t, pump.stdin.(*discardWriteCloser).written, 1)
}

func TestChatPumpWorkflowEndKeepsSession(t *testing.T) {
	markerPath := filepath.Join(t.TempDir(), routines.RunCompleteFileName)
	pump := idlePump(markerPath, time.Time{}, 1, true)
	pump.observe(streamEvent{kind: "result"})
	pump.observe(streamEvent{kind: "system", subtype: "task_progress"})
	assert.Equal(t, reasonNone, pump.completion())

	pump.observe(streamEvent{backgroundTasks: 0, kind: "system", subtype: subtypeBackgroundTasks})
	assert.Equal(t, reasonNone, pump.completion())

	pump.observe(streamEvent{kind: "system", subtype: "task_notification"})
	pump.observe(streamEvent{kind: "system", subtype: "init"})
	assert.Equal(t, reasonNone, pump.completion())

	pump.observe(streamEvent{kind: "assistant"})
	pump.observe(streamEvent{kind: "result"})
	assert.Equal(t, reasonResultClose, pump.completion())
}

func TestChatPumpOnSessionFiresOnce(t *testing.T) {
	markerPath := filepath.Join(t.TempDir(), routines.RunCompleteFileName)
	pump := idlePump(markerPath, time.Time{}, 1, true)
	var named []string
	pump.spec.OnSession = func(sessionId string) { named = append(named, sessionId) }

	pump.observe(streamEvent{kind: "system", subtype: "init"})
	pump.observe(streamEvent{kind: "system", sessionId: "s-1", subtype: "init"})
	pump.observe(streamEvent{kind: "system", sessionId: "s-1", subtype: "init"})

	assert.Equal(t, []string{"s-1"}, named)
	assert.Equal(t, "s-1", pump.sessionId)
}

func TestChatPumpSteerWaitsForTurnEnd(t *testing.T) {
	markerPath := filepath.Join(t.TempDir(), routines.RunCompleteFileName)
	steerPath := filepath.Join(filepath.Dir(markerPath), routines.SteerPromptFileName)
	require.NoError(t, os.WriteFile(steerPath, []byte("go on"), 0o644))
	pump := idlePump(markerPath, time.Time{}, 0, false)
	pump.cellDir = t.TempDir()

	pump.deliverSteer()
	assert.FileExists(t, steerPath)

	pump.idleSince = time.Now()
	pump.deliverSteer()
	assert.NoFileExists(t, steerPath)
	assert.Len(t, pump.stdin.(*discardWriteCloser).written, 1)
}

func TestNewChatPumpRequiresMarker(t *testing.T) {
	spec := &ChatSpec{Complete: CompleteMarker, SteerDir: t.TempDir()}

	_, err := newChatPump(spec)

	assert.Error(t, err)
}

func TestRunChatCancelled(t *testing.T) {
	installFakeClaude(t)
	t.Setenv(fakeHangEnv, "1")
	spec := chatSpec(t, CompleteResult)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	outcome, err := RunChat(ctx, spec)

	require.NoError(t, err)
	assert.Equal(t, exitCancelled, outcome.Status)
}

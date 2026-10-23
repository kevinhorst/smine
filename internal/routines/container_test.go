package routines

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainerLabel(t *testing.T) {
	assert.Equal(t, "smine.routine=spec-drift", ContainerLabel("spec-drift"))
}

func TestFormatContainerMounts(t *testing.T) {
	mounts := []*ContainerMount{
		{Access: MountReadWrite, Repo: "agent-plans"},
		{Access: MountReadOnly, Repo: "peek-mcp"},
	}
	assert.Equal(t, "agent-plans:rw,peek-mcp:ro", FormatContainerMounts(mounts))
}

func TestParseContainerFlag(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   bool
		_shouldPass bool

		value string
	}

	tests := make([]*testCase, 0)

	// pass-absent-is-off
	tests = append(tests, &testCase{
		_id:         "pass-absent-is-off",
		_expected:   false,
		_shouldPass: true,

		value: "",
	})

	// pass-zero-is-off
	tests = append(tests, &testCase{
		_id:         "pass-zero-is-off",
		_expected:   false,
		_shouldPass: true,

		value: "0",
	})

	// pass-one-is-on
	tests = append(tests, &testCase{
		_id:         "pass-one-is-on",
		_expected:   true,
		_shouldPass: true,

		value: "1",
	})

	// fail-word
	tests = append(tests, &testCase{
		_id:         "fail-word",
		_expected:   false,
		_shouldPass: false,

		value: "true",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			isContainerActive, err := ParseContainerFlag(test.value)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, isContainerActive)
		})
	}
}

func TestParseContainerMounts(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   []*ContainerMount
		_shouldPass bool

		value string
	}

	tests := make([]*testCase, 0)

	// pass-empty
	tests = append(tests, &testCase{
		_id:         "pass-empty",
		_expected:   nil,
		_shouldPass: true,

		value: " ",
	})

	// pass-two-mounts
	tests = append(tests, &testCase{
		_id: "pass-two-mounts",
		_expected: []*ContainerMount{
			{Access: MountReadWrite, Repo: "agent-plans"},
			{Access: MountReadOnly, Repo: "peek-mcp"},
		},
		_shouldPass: true,

		value: "agent-plans:rw, peek-mcp:ro",
	})

	// fail-missing-access
	tests = append(tests, &testCase{
		_id:         "fail-missing-access",
		_expected:   nil,
		_shouldPass: false,

		value: "peek-mcp",
	})

	// fail-missing-repo
	tests = append(tests, &testCase{
		_id:         "fail-missing-repo",
		_expected:   nil,
		_shouldPass: false,

		value: ":ro",
	})

	// fail-unknown-access
	tests = append(tests, &testCase{
		_id:         "fail-unknown-access",
		_expected:   nil,
		_shouldPass: false,

		value: "peek-mcp:write",
	})

	// fail-duplicate-repo
	tests = append(tests, &testCase{
		_id:         "fail-duplicate-repo",
		_expected:   nil,
		_shouldPass: false,

		value: "peek-mcp:ro,peek-mcp:rw",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			mounts, err := ParseContainerMounts(test.value)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, mounts)
		})
	}
}

func TestSignalContainers(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool

		routineDir string
	}

	// Any docker call fails: PATH holds no binary.
	t.Setenv("PATH", t.TempDir())

	tests := make([]*testCase, 0)

	// not-a-container-routine
	hostRoutineDir := containerTestRoutineDir(t, map[string]string{EnvContainer: "0"})
	tests = append(tests, &testCase{
		_id:         "not-a-container-routine",
		_shouldPass: true,

		routineDir: hostRoutineDir,
	})

	// container-routine-without-docker
	containerRoutineDir := containerTestRoutineDir(t, map[string]string{EnvContainer: "1"})
	tests = append(tests, &testCase{
		_id:         "container-routine-without-docker",
		_shouldPass: false,

		routineDir: containerRoutineDir,
	})

	// missing-plist
	tests = append(tests, &testCase{
		_id:         "missing-plist",
		_shouldPass: false,

		routineDir: t.TempDir(),
	})

	// invalid-container-flag
	invalidRoutineDir := containerTestRoutineDir(t, map[string]string{EnvContainer: "yes"})
	tests = append(tests, &testCase{
		_id:         "invalid-container-flag",
		_shouldPass: false,

		routineDir: invalidRoutineDir,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := SignalContainers(t.Context(), test.routineDir, ContainerPause)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
		})
	}
}

// containerTestRoutineDir is a routine dir whose one plist carries env.
func containerTestRoutineDir(t *testing.T, env map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "demo")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	content := map[string]any{"EnvironmentVariables": env, "Label": "com.smine.routine.demo"}
	require.NoError(t, writePlistAtomic(content, filepath.Join(dir, "com.smine.routine.demo.plist")))
	return dir
}

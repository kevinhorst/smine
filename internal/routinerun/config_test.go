package routinerun

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/routines"
)

func TestLoadConfig_RoutineModel(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool

		model string
	}

	tests := make([]*testCase, 0)

	// deployed-model
	tests = append(tests, &testCase{_id: "deployed-model", _shouldPass: true, model: "claude-opus-5-5[1m]"})

	// retired-model
	tests = append(tests, &testCase{_id: "retired-model", _shouldPass: false, model: "claude-opus-4-8[1m]"})

	// empty-model
	tests = append(tests, &testCase{_id: "empty-model", _shouldPass: true, model: ""})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			home := t.TempDir()
			claudeHome := filepath.Join(home, ".claude")
			require.NoError(t, os.MkdirAll(claudeHome, 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(claudeHome, "settings.json"), []byte(`{"availableModels": ["claude-opus-5-5", "claude-opus-5-5[1m]", "claude-sonnet-5"]}`), 0o600))
			t.Setenv("HOME", home)
			t.Setenv("ROUTINE_MODEL", test.model)

			_, err := loadConfig("unit-test")
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
		})
	}
}

// containerKnobs projects the container fields of a loaded config; a failed load projects to nil.
func containerKnobs(runConfig *Config) *Config {
	if runConfig == nil {
		return nil
	}

	return &Config{
		ContainerCpus:     runConfig.ContainerCpus,
		ContainerMemory:   runConfig.ContainerMemory,
		ContainerMounts:   runConfig.ContainerMounts,
		ContainerRoot:     runConfig.ContainerRoot,
		IsContainerActive: runConfig.IsContainerActive,
	}
}

func TestLoadConfig_Container(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *Config
		_shouldPass bool

		cpus   string
		flag   string
		home   string
		memory string
		mounts string
		root   string
	}

	overrideRoot := filepath.Join(t.TempDir(), "container")
	tests := make([]*testCase, 0)

	// defaults
	home := t.TempDir()
	tests = append(tests, &testCase{
		_id: "defaults",
		_expected: &Config{
			ContainerCpus:     "4",
			ContainerMemory:   "5g",
			ContainerMounts:   nil,
			ContainerRoot:     filepath.Join(home, ".cache", "claude-routine", "container"),
			IsContainerActive: false,
		},
		_shouldPass: true,

		cpus:   "",
		flag:   "",
		home:   home,
		memory: "",
		mounts: "",
		root:   "",
	})

	// all-keys-set
	tests = append(tests, &testCase{
		_id: "all-keys-set",
		_expected: &Config{
			ContainerCpus:   "2.5",
			ContainerMemory: "8g",
			ContainerMounts: []*routines.ContainerMount{
				{Access: routines.MountReadWrite, Repo: "agent-plans"},
				{Access: routines.MountReadOnly, Repo: "peek-mcp"},
			},
			ContainerRoot:     overrideRoot,
			IsContainerActive: true,
		},
		_shouldPass: true,

		cpus:   "2.5",
		flag:   "1",
		home:   t.TempDir(),
		memory: "8g",
		mounts: "agent-plans:rw,peek-mcp:ro",
		root:   overrideRoot,
	})

	// invalid-flag
	tests = append(tests, &testCase{
		_id:         "invalid-flag",
		_expected:   nil,
		_shouldPass: false,

		cpus:   "",
		flag:   "on",
		home:   t.TempDir(),
		memory: "",
		mounts: "",
		root:   "",
	})

	// invalid-mounts
	tests = append(tests, &testCase{
		_id:         "invalid-mounts",
		_expected:   nil,
		_shouldPass: false,

		cpus:   "",
		flag:   "1",
		home:   t.TempDir(),
		memory: "",
		mounts: "peek-mcp",
		root:   "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			t.Setenv("HOME", test.home)
			t.Setenv("ROUTINE_MODEL", "")
			t.Setenv(routines.EnvContainer, test.flag)
			t.Setenv(routines.EnvContainerMounts, test.mounts)
			t.Setenv("ROUTINE_CONTAINER_CPUS", test.cpus)
			t.Setenv("ROUTINE_CONTAINER_MEMORY", test.memory)
			t.Setenv("ROUTINE_CONTAINER_ROOT", test.root)

			runConfig, err := loadConfig("unit-test")

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, containerKnobs(runConfig))
		})
	}
}

func TestLoadConfig_StateDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ROUTINE_MODEL", "")

	runConfig, err := loadConfig("unit-test")

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".claude", "smine"), runConfig.StateDir)
}

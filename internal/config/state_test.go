package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateDir(t *testing.T) {
	// override-home
	stateDir, err := StateDir("/tmp/claude-home")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("/tmp/claude-home", "smine"), stateDir)

	// default-under-user-home
	userHome, err := os.UserHomeDir()
	require.NoError(t, err)

	stateDir, err = StateDir("")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(userHome, ".claude", "smine"), stateDir)
}

func TestStoreRoot(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   string
		_shouldPass bool

		mainCheckout string
		override     string
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	archive := filepath.Join(home, "GolandProjects", "agent-plans")

	tests := make([]*testCase, 0)

	// pass-checkout-dir-name
	tests = append(tests, &testCase{
		_id:         "pass-checkout-dir-name",
		_expected:   filepath.Join(archive, "claude-configs"),
		_shouldPass: true,

		mainCheckout: "/src/claude-configs",
		override:     "",
	})

	// pass-trailing-separator
	tests = append(tests, &testCase{
		_id:         "pass-trailing-separator",
		_expected:   filepath.Join(archive, "claude-configs"),
		_shouldPass: true,

		mainCheckout: "/src/claude-configs/",
		override:     "",
	})

	// pass-override-replaces-archive-root
	tests = append(tests, &testCase{
		_id:         "pass-override-replaces-archive-root",
		_expected:   "/arm/worktree/store",
		_shouldPass: true,

		mainCheckout: "/src/claude-configs",
		override:     "/arm/worktree/store/",
	})

	// pass-override-without-checkout
	tests = append(tests, &testCase{
		_id:         "pass-override-without-checkout",
		_expected:   "/arm/worktree/store",
		_shouldPass: true,

		mainCheckout: "",
		override:     "/arm/worktree/store",
	})

	// fail-override-relative
	tests = append(tests, &testCase{
		_id:         "fail-override-relative",
		_expected:   "",
		_shouldPass: false,

		mainCheckout: "/src/claude-configs",
		override:     "store",
	})

	// fail-empty
	tests = append(tests, &testCase{
		_id:         "fail-empty",
		_expected:   "",
		_shouldPass: false,

		mainCheckout: "",
		override:     "",
	})

	// fail-relative
	tests = append(tests, &testCase{
		_id:         "fail-relative",
		_expected:   "",
		_shouldPass: false,

		mainCheckout: "claude-configs",
		override:     "",
	})

	// fail-filesystem-root
	tests = append(tests, &testCase{
		_id:         "fail-filesystem-root",
		_expected:   "",
		_shouldPass: false,

		mainCheckout: "/",
		override:     "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			t.Setenv(StoreRootEnv, test.override)
			root, err := StoreRoot(test.mainCheckout)
			assert.Equal(t, test._expected, root)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
		})
	}
}

func TestStoreArchiveDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(StoreRootEnv, "/arm/worktree/store")

	archive, err := StoreArchiveDir()

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, "GolandProjects", "agent-plans"), archive, "the override never moves the archive")
}

func TestStateConstants(t *testing.T) {
	assert.Equal(t, "repos.json", StateRegistryFile)
	assert.Equal(t, "backlog", StoreBacklogDir)
	assert.Equal(t, "proposals", StoreProposalsDir)
}

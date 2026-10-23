package acdsl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeRegistry(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "registry.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestLoadRegistryValid(t *testing.T) {
	path := writeRegistry(t, `{"check": {"argv": ["go", "run", "./x"], "timeout_s": 30, "description": "d"}}`)
	registry, err := LoadRegistry(path)
	require.NoError(t, err)
	require.Contains(t, registry, "check")
	assert.Equal(t, []string{"go", "run", "./x"}, registry["check"].Argv)
	assert.Equal(t, 30, registry["check"].TimeoutSec)
}

func TestLoadRegistryMissingFileIsError(t *testing.T) {
	_, err := LoadRegistry(filepath.Join(t.TempDir(), "absent.json"))
	require.Error(t, err)
}

func TestLoadRegistryEmptyArgvIsError(t *testing.T) {
	path := writeRegistry(t, `{"check": {"argv": [], "timeout_s": 30}}`)
	_, err := LoadRegistry(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty argv")
}

func TestLoadRegistryTimeoutBounds(t *testing.T) {
	path := writeRegistry(t, `{"check": {"argv": ["x"], "timeout_s": 0}}`)
	_, err := LoadRegistry(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout_s must be 1..300")

	path = writeRegistry(t, `{"check": {"argv": ["x"], "timeout_s": 301}}`)
	_, err = LoadRegistry(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout_s must be 1..300")
}

func TestLoadRegistryLocalOverlayWinsByName(t *testing.T) {
	path := writeRegistry(t, `{"check": {"argv": ["go", "run", "./x"], "timeout_s": 30, "description": "baseline"}}`)
	local := filepath.Join(filepath.Dir(path), LocalRegistryName)
	require.NoError(t, os.WriteFile(local,
		[]byte(`{"check": {"argv": ["true"], "timeout_s": 5, "description": "local"}, "extra": {"argv": ["true"], "timeout_s": 5, "description": "added"}}`), 0o644))

	registry, err := LoadRegistry(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"true"}, registry["check"].Argv)
	assert.Equal(t, "local", registry["check"].Description)
	assert.Contains(t, registry, "extra")
}

// provideRegistryRoot is a module root whose committed registry holds one
// entry, check.
func provideRegistryRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "acdsl"), 0o755))
	committed := `{"check": {"argv": ["go", "run", "./x"], "timeout_s": 30, "description": "baseline"}}`
	require.NoError(t, os.WriteFile(filepath.Join(root, RegistryPath), []byte(committed), 0o644))
	return root
}

func TestLoadRegistryOverlay(t *testing.T) {
	type testCase struct {
		_id           string
		_expectedArgv map[string][]string
		_shouldPass   bool

		overlayPath string
		root        string
	}

	root := provideRegistryRoot(t)
	probe := `{"PROBE-X": {"argv": ["/bin/bash", "/p/probe.sh"], "timeout_s": 20, "description": "probe"}}`
	rewrite := `{"check": {"argv": ["true"], "timeout_s": 5, "description": "overlay"}}`
	tests := make([]*testCase, 0)

	// no-overlay
	tests = append(tests, &testCase{
		_id:           "no-overlay",
		_expectedArgv: map[string][]string{"check": {"go", "run", "./x"}},
		_shouldPass:   true,

		overlayPath: "",
		root:        root,
	})

	// committed-path-overlays-nothing
	tests = append(tests, &testCase{
		_id:           "committed-path-overlays-nothing",
		_expectedArgv: map[string][]string{"check": {"go", "run", "./x"}},
		_shouldPass:   true,

		overlayPath: filepath.Join(root, "acdsl", "registry.json"),
		root:        root,
	})

	// probe-entry-added
	tests = append(tests, &testCase{
		_id:           "probe-entry-added",
		_expectedArgv: map[string][]string{"PROBE-X": {"/bin/bash", "/p/probe.sh"}, "check": {"go", "run", "./x"}},
		_shouldPass:   true,

		overlayPath: writeRegistry(t, probe),
		root:        root,
	})

	// overlay-wins-by-name
	tests = append(tests, &testCase{
		_id:           "overlay-wins-by-name",
		_expectedArgv: map[string][]string{"check": {"true"}},
		_shouldPass:   true,

		overlayPath: writeRegistry(t, rewrite),
		root:        root,
	})

	// overlay-missing
	tests = append(tests, &testCase{
		_id:           "overlay-missing",
		_expectedArgv: nil,
		_shouldPass:   false,

		overlayPath: filepath.Join(t.TempDir(), "absent.json"),
		root:        root,
	})

	// overlay-invalid-entry
	tests = append(tests, &testCase{
		_id:           "overlay-invalid-entry",
		_expectedArgv: nil,
		_shouldPass:   false,

		overlayPath: writeRegistry(t, `{"PROBE-X": {"argv": [], "timeout_s": 20}}`),
		root:        root,
	})

	// committed-missing
	tests = append(tests, &testCase{
		_id:           "committed-missing",
		_expectedArgv: nil,
		_shouldPass:   false,

		overlayPath: writeRegistry(t, probe),
		root:        t.TempDir(),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			registry, err := LoadRegistryOverlay(test.overlayPath, test.root)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			argv := make(map[string][]string, len(registry))
			for name, entry := range registry {
				argv[name] = entry.Argv
			}
			assert.Equal(t, test._expectedArgv, argvOrNil(argv))
		})
	}
}

// argvOrNil is nil for an empty map, so a failed load compares equal to nil.
func argvOrNil(argv map[string][]string) map[string][]string {
	if len(argv) == 0 {
		return nil
	}
	return argv
}

func TestLoadRegistryMalformedLocalIsError(t *testing.T) {
	path := writeRegistry(t, `{"check": {"argv": ["true"], "timeout_s": 5, "description": "d"}}`)
	local := filepath.Join(filepath.Dir(path), LocalRegistryName)
	require.NoError(t, os.WriteFile(local, []byte("{nope"), 0o644))
	_, err := LoadRegistry(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), LocalRegistryName)
}

func TestLoadRegistryLocalEntriesAreValidated(t *testing.T) {
	path := writeRegistry(t, `{"check": {"argv": ["true"], "timeout_s": 5, "description": "d"}}`)
	local := filepath.Join(filepath.Dir(path), LocalRegistryName)
	require.NoError(t, os.WriteFile(local,
		[]byte(`{"check": {"argv": [], "timeout_s": 5, "description": "empty argv"}}`), 0o644))
	_, err := LoadRegistry(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty argv")
}

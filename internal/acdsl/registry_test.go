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
	assert.Contains(t, err.Error(), "timeout_s must be 1..60")

	path = writeRegistry(t, `{"check": {"argv": ["x"], "timeout_s": 61}}`)
	_, err = LoadRegistry(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout_s must be 1..60")
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

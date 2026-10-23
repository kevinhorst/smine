package routinerun

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisteredMcpConfig(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   string
		_shouldPass bool

		claudeJsonPath string
	}

	dir := t.TempDir()
	registered := filepath.Join(dir, "registered.json")
	registeredJson := `{"mcpServers":{"peek-mcp":{"type":"stdio","command":"/bin/peek-mcp","args":["start","--transport=stdio"]},"goland":{"command":"goland"}}}`
	require.NoError(t, os.WriteFile(registered, []byte(registeredJson), 0o644))
	noPeek := filepath.Join(dir, "no-peek.json")
	require.NoError(t, os.WriteFile(noPeek, []byte(`{"mcpServers":{"goland":{"command":"goland"}}}`), 0o644))

	tests := make([]*testCase, 0)

	// no-extra-args
	tests = append(tests, &testCase{
		_id:         "no-extra-args",
		_expected:   `{"mcpServers":{"peek-mcp":{"args":["start","--transport=stdio"],"command":"/bin/peek-mcp","type":"stdio"}}}`,
		_shouldPass: true,

		claudeJsonPath: registered,
	})

	// entry-missing
	tests = append(tests, &testCase{
		_id:         "entry-missing",
		_expected:   "",
		_shouldPass: false,

		claudeJsonPath: noPeek,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			mcpConfig, err := registeredMcpConfig(test.claudeJsonPath)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, mcpConfig)
		})
	}
}

func TestSmineMcpConfig(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   string
		_shouldPass bool

		claudeJsonPath string
	}

	dir := t.TempDir()
	registered := filepath.Join(dir, "registered.json")
	registeredJson := `{"oauth":{},"mcpServers":{"peek-mcp":{"type":"stdio","command":"/bin/peek-mcp","args":["start","--transport=stdio","--depth=200"],"env":{"MAX_MCP_OUTPUT_TOKENS":"125000"}},"goland":{"command":"goland"}}}`
	require.NoError(t, os.WriteFile(registered, []byte(registeredJson), 0o644))
	noPeek := filepath.Join(dir, "no-peek.json")
	require.NoError(t, os.WriteFile(noPeek, []byte(`{"mcpServers":{"goland":{"command":"goland"}}}`), 0o644))
	malformed := filepath.Join(dir, "malformed.json")
	require.NoError(t, os.WriteFile(malformed, []byte(`{"mcpServers":`), 0o644))

	tests := make([]*testCase, 0)

	// registered-peek-gets-window
	tests = append(tests, &testCase{
		_id:         "registered-peek-gets-window",
		_expected:   `{"mcpServers":{"peek-mcp":{"args":["start","--transport=stdio","--depth=200","--watch-window-days=14"],"command":"/bin/peek-mcp","type":"stdio"}}}`,
		_shouldPass: true,

		claudeJsonPath: registered,
	})

	// missing-file-fails
	tests = append(tests, &testCase{
		_id:         "missing-file-fails",
		_expected:   "",
		_shouldPass: false,

		claudeJsonPath: filepath.Join(dir, "absent.json"),
	})

	// no-peek-entry-fails
	tests = append(tests, &testCase{
		_id:         "no-peek-entry-fails",
		_expected:   "",
		_shouldPass: false,

		claudeJsonPath: noPeek,
	})

	// malformed-file-fails
	tests = append(tests, &testCase{
		_id:         "malformed-file-fails",
		_expected:   "",
		_shouldPass: false,

		claudeJsonPath: malformed,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			mcpConfig, err := SmineMcpConfig(test.claudeJsonPath)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, mcpConfig)
		})
	}
}

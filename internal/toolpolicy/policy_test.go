package toolpolicy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/install"
)

func TestPolicy_RenderedDefaults(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool

		home string
	}

	defaults, err := install.RenderDefaults(components.Gate, components.SinkHooksConfig)
	require.NoError(t, err)

	tests := make([]*testCase, 0)

	// pass-rendered-defaults
	renderedHome := t.TempDir()
	writeHookConfigBytes(t, renderedHome, defaults)
	tests = append(tests, &testCase{_id: "pass-rendered-defaults", _shouldPass: true, home: renderedHome})

	// fail-unknown-key
	unknownKeyHome := t.TempDir()
	document := map[string]map[string]any{}
	require.NoError(t, json.Unmarshal(defaults, &document))
	document["toolpolicy"]["policy_path"] = "~/.claude/hooks/tool-policy.json"
	unknownKeyBytes, err := json.Marshal(document)
	require.NoError(t, err)
	writeHookConfigBytes(t, unknownKeyHome, unknownKeyBytes)
	tests = append(tests, &testCase{_id: "fail-unknown-key", _shouldPass: false, home: unknownKeyHome})

	// fail-two-classes
	twoClassesHome := t.TempDir()
	twoClasses := &hookconfig.Config{}
	require.NoError(t, json.Unmarshal(defaults, twoClasses))
	twoClasses.Toolpolicy.ToolClasses.Read = append(twoClasses.Toolpolicy.ToolClasses.Read, "Bash")
	writeHookConfig(t, twoClassesHome, twoClasses)
	tests = append(tests, &testCase{_id: "fail-two-classes", _shouldPass: false, home: twoClassesHome})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			_, err := hookconfig.Load(test.home)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
		})
	}
}

func writeHookConfig(t *testing.T, home string, cfg *hookconfig.Config) {
	t.Helper()
	encoded, err := json.Marshal(cfg)
	require.NoError(t, err)
	writeHookConfigBytes(t, home, encoded)
}

func writeHookConfigBytes(t *testing.T, home string, encoded []byte) {
	t.Helper()
	path := hookconfig.Path(home)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, encoded, 0o644))
}

package routinerun

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	settings "github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/contextdocs"
)

func TestBootstrapSyncContextPacksReplaysRecords(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	stateDir := filepath.Join(home, ".claude", "smine")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))

	repoRoot := t.TempDir()
	contextDir := filepath.Join(repoRoot, contextdocs.SourceContextDir)
	require.NoError(t, os.MkdirAll(filepath.Join(contextDir, "actions"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(contextDir, "actions", "nav.md"),
		[]byte("**ACTION-NAV-001** `[prose]` — Keep.\n\n* Applies: always.\n"), 0o644))
	aspects := []contextdocs.RuleAspect{{Name: "NAV", Scope: "Navigation", Class: "scope"}}
	require.NoError(t, contextdocs.SaveAspects(contextDir, aspects))

	synced, skipped := t.TempDir(), t.TempDir()
	registry := `{"repos": [{"name": "synced", "path": ` + jsonQuote(t, synced) + `}, {"name": "skipped", "path": ` + jsonQuote(t, skipped) + `}]}`
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, settings.StateRegistryFile), []byte(registry), 0o644))
	record := contextdocs.ContextFile{Deploy: &contextdocs.ContextDeploy{ContextDir: "docs", Langs: nil, Role: "r", WriteRepo: true}}
	recordData, err := record.Render()
	require.NoError(t, err)
	recordPath := contextdocs.DeployRecordPath(stateDir, "synced")
	require.NoError(t, os.MkdirAll(filepath.Dir(recordPath), 0o755))
	require.NoError(t, os.WriteFile(recordPath, recordData, 0o644))

	stderr := &bytes.Buffer{}
	bootstrap := NewBootstrap(repoRoot, stderr, &bytes.Buffer{})
	bootstrap.syncContextPacks(context.Background())

	assert.Empty(t, stderr.String())
	assert.FileExists(t, filepath.Join(synced, "docs", "context.json"))
	assert.FileExists(t, filepath.Join(synced, "docs", "aspects.json"))
	assert.NoDirExists(t, filepath.Join(skipped, "docs"))
	assert.FileExists(t, filepath.Join(contextDir, "context.json"))
}

// A temp path pasted into JSON breaks on Windows backslashes; the encoder escapes it.
func jsonQuote(t *testing.T, value string) string {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return string(data)
}

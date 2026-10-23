package contextdocs

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/shell"
)

func TestRenderOverlayPatchShape(t *testing.T) {
	files := []OverlayFile{
		{Content: []byte("{\n  \"b\": 1\n}\n"), Path: "docs/context.json"},
		{Content: []byte("[]\n"), Path: "docs/aspects.json"},
	}

	patch, err := RenderOverlayPatch(files)
	require.NoError(t, err)

	expected := "diff --git a/docs/aspects.json b/docs/aspects.json\nnew file mode 100644\n--- /dev/null\n+++ b/docs/aspects.json\n@@ -0,0 +1,1 @@\n+[]\n" +
		"diff --git a/docs/context.json b/docs/context.json\nnew file mode 100644\n--- /dev/null\n+++ b/docs/context.json\n@@ -0,0 +1,3 @@\n+{\n+  \"b\": 1\n+}\n"
	assert.Equal(t, expected, string(patch))
}

func TestRenderOverlayPatchRefusesMissingNewline(t *testing.T) {
	_, err := RenderOverlayPatch([]OverlayFile{{Content: []byte("x"), Path: "docs/a.json"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "docs/a.json")
}

func TestRenderOverlayPatchEmpty(t *testing.T) {
	patch, err := RenderOverlayPatch(nil)
	require.NoError(t, err)
	assert.Empty(t, patch)
}

func TestRenderOverlayPatchApplies(t *testing.T) {
	repo := t.TempDir()
	ctx := context.Background()
	_, err := shell.Run(ctx, repo, "git", "init", "-q")
	require.NoError(t, err)
	content := "first line\n  indented second line\n\nfourth after a blank\n"
	files := []OverlayFile{
		{Content: []byte(content), Path: "docs/context.json"},
		{Content: []byte("one\n"), Path: "docs/aspects.json"},
	}
	patch, err := RenderOverlayPatch(files)
	require.NoError(t, err)
	patchPath := filepath.Join(t.TempDir(), OverlayPatchName)
	require.NoError(t, os.WriteFile(patchPath, patch, 0o644))

	_, err = shell.Run(ctx, repo, "git", "apply", patchPath)
	require.NoError(t, err)

	applied, err := os.ReadFile(filepath.Join(repo, "docs", "context.json"))
	require.NoError(t, err)
	assert.Equal(t, content, string(applied))
}

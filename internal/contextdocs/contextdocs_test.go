package contextdocs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFixtureTree(t *testing.T) string {
	t.Helper()
	src := t.TempDir()
	for dir, files := range map[string][]string{
		"rules": {"navigation.md"},
		"style": {"go.md", "plan.md"},
		"plans": {"skipped.md"},
	} {
		require.NoError(t, os.MkdirAll(filepath.Join(src, dir), 0755))
		for _, file := range files {
			require.NoError(t, os.WriteFile(filepath.Join(src, dir, file), []byte("# doc"), 0644))
		}
	}
	require.NoError(t, os.WriteFile(filepath.Join(src, "AGENTS.md"), []byte("template"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(src, "style", "notes.txt"), []byte("x"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(src, "style", "nested"), 0755))
	return src
}

func TestScan(t *testing.T) {
	t.Run("groups-md-files-name-sorted", func(t *testing.T) {
		groups, err := Scan(writeFixtureTree(t))
		require.NoError(t, err)
		require.Len(t, groups, 2)
		assert.Equal(t, "rules", groups[0].Name)
		assert.Equal(t, []string{"navigation.md"}, groups[0].Files)
		assert.Equal(t, "style", groups[1].Name)
		assert.Equal(t, []string{"go.md", "plan.md"}, groups[1].Files)
	})

	t.Run("missing-dir-errors", func(t *testing.T) {
		_, err := Scan(filepath.Join(t.TempDir(), "nope"))
		assert.Error(t, err)
	})
}

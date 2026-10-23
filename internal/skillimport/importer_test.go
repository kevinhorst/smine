package skillimport

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

// formatFields is the skill format whose typed fields the importer does not read.
const formatFields = 3

// repoSkillDirs lists the repo skill dirs the importer's markdown grammar covers:
// format 3 renders typed fields and derived links the importer never reads back.
func repoSkillDirs(t *testing.T) []string {
	t.Helper()
	dirs, err := skillgraph.Discover(filepath.Join("..", "..", "skills"))
	require.NoError(t, err)

	var importable []string
	for _, dir := range dirs {
		skill, err := skillgraph.Load(dir)
		require.NoError(t, err)
		format, _ := strconv.Atoi(skill.Field("format"))
		if format < formatFields {
			importable = append(importable, dir)
		}
	}
	return importable
}

// importForProof renders a repo skill's model to SKILL.md text, imports that
// text from a temp dir and completes the import with what markdown never
// carried: changelog, atoms, env, catalog and vocabulary of the model.
func importForProof(t *testing.T, dir string) (*skillgraph.Skill, []byte) {
	t.Helper()
	loaded, err := skillgraph.Load(dir)
	require.NoError(t, err)

	manifest, err := skillgraph.RenderManifest(loaded)
	require.NoError(t, err)

	importDir := filepath.Join(t.TempDir(), filepath.Base(dir))
	require.NoError(t, os.MkdirAll(importDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(importDir, skillgraph.FileManifest), manifest, 0o644))
	skill, err := Import(importDir)
	require.NoError(t, err)

	skill.Atoms = loaded.Atoms
	skill.Catalog = loaded.Catalog
	skill.Changelog = loaded.Changelog
	skill.Env = loaded.Env
	skill.Exclusive = loaded.Exclusive
	skill.Vocabulary = loaded.Vocabulary
	return skill, manifest
}

func TestImportProveAllSkills(t *testing.T) {
	dirs := repoSkillDirs(t)
	require.NotEmpty(t, dirs)

	// Run tests
	for _, dir := range dirs {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			skill, manifest := importForProof(t, dir)
			assert.NoError(t, Prove(manifest, skill))
		})
	}
}

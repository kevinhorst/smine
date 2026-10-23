package skills_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skillimport"
	"github.com/kevinhorst/smine/internal/skills"
)

func TestModelAllowedTools(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		dir string
	}

	tests := make([]*testCase, 0)

	// declared
	declaredDir := filepath.Join(t.TempDir(), "declared")
	writeModelSkill(t, declaredDir, "d", "allowed-tools: Read, Bash(jq *)\n")
	tests = append(tests, &testCase{
		_id:       "declared",
		_expected: "Read, Bash(jq *)",

		dir: declaredDir,
	})

	// absent
	absentDir := filepath.Join(t.TempDir(), "absent")
	writeModelSkill(t, absentDir, "d", "")
	tests = append(tests, &testCase{
		_id:       "absent",
		_expected: "",

		dir: absentDir,
	})

	// model-not-loading
	brokenDir := t.TempDir()
	require.NoError(t, os.WriteFile(skillgraph.ModelPath(brokenDir), []byte("not cue {"), 0o644))
	tests = append(tests, &testCase{
		_id:       "model-not-loading",
		_expected: "",

		dir: brokenDir,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, skills.ModelAllowedTools(test.dir))
		})
	}
}

func TestScan(t *testing.T) {
	repoRoot, homeRoot := t.TempDir(), t.TempDir()
	writeVersionedModelSkill(
		t,
		filepath.Join(repoRoot, "versioned"),
		"---\nname: versioned\ndescription: Extract JSON fields with jq.\nauthor: Kevin Horst\nversion: 1.2\nallowed-tools: Bash(jq *), Read\naliases: pk, p\n---\n\n# Versioned\n",
		`[{"version": "1.2", "date": "2026-10-02", "text": "added X"}]`,
	)
	writeHomeSkill(t, filepath.Join(homeRoot, "peek"), "---\nname: peek\ndescription: Show the latest session.\nversion: 2.0\naliases: pk\n---\n\n# Peek\n", "reference/schema.json")
	writeHomeSkill(t, filepath.Join(homeRoot, "pk"), "---\nname: pk\ndescription: Extract JSON fields with jq.\nversion: 1.2\n---\n\n# Versioned\n")
	writeHomeSkill(t, filepath.Join(homeRoot, "pkg"), "---\nname: pkg\ndescription: Unrelated skill.\nversion: 1.0\n---\n")
	writeHomeSkill(t, filepath.Join(homeRoot, "commas"), "---\nname: commas\ndescription: Stray commas.\naliases: pk, ,\n---\n")
	brokenDir := filepath.Join(repoRoot, "broken")
	require.NoError(t, os.MkdirAll(brokenDir, 0o755))
	require.NoError(t, os.WriteFile(skillgraph.ModelPath(brokenDir), []byte("not cue {"), 0o644))
	writeHomeSkill(t, filepath.Join(repoRoot, "stray"), "---\nname: stray\ndescription: Markdown only.\n---\n")
	legacyDir := filepath.Join(repoRoot, "legacy")
	require.NoError(t, os.MkdirAll(legacyDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir, skillgraph.LegacyModelFile), []byte("x: 1\n"), 0o644))

	list, err := skills.Scan(repoRoot, homeRoot)
	require.NoError(t, err)

	t.Run("repo-skill-found-by-model-file", func(t *testing.T) {
		versioned, ok := skills.Find(list, skills.OriginRepo, "versioned")
		require.True(t, ok)
		assert.Equal(t, "Extract JSON fields with jq.", versioned.Description)
		assert.Equal(t, "1.2", versioned.Version)
		assert.Equal(t, "Kevin Horst", versioned.Author)
		assert.Equal(t, "Bash(jq *), Read", versioned.AllowedTools)
		assert.Equal(t, []string{"versioned.cue"}, versioned.Files)
	})

	t.Run("legacy-skill-cue-not-a-skill", func(t *testing.T) {
		_, ok := skills.Find(list, skills.OriginRepo, "legacy")
		assert.False(t, ok)
	})

	t.Run("home-skill-from-markdown", func(t *testing.T) {
		peek, ok := skills.Find(list, skills.OriginHome, "peek")
		require.True(t, ok)
		assert.Equal(t, "Show the latest session.", peek.Description)
		assert.Equal(t, "2.0", peek.Version)
		assert.Equal(t, []string{filepath.Join("reference", "schema.json")}, peek.Files)
	})

	t.Run("repo-model-not-loading-listed-empty", func(t *testing.T) {
		broken, ok := skills.Find(list, skills.OriginRepo, "broken")
		require.True(t, ok)
		assert.Empty(t, broken.Description)
		assert.Empty(t, broken.Version)
		assert.Empty(t, broken.AllowedTools)
	})

	t.Run("stray-markdown-in-repo-ignored", func(t *testing.T) {
		_, ok := skills.Find(list, skills.OriginRepo, "stray")
		assert.False(t, ok)
		assert.Len(t, list, 5)
	})

	t.Run("repo-skill-aliases", func(t *testing.T) {
		versioned, ok := skills.Find(list, skills.OriginRepo, "versioned")
		require.True(t, ok)
		assert.Equal(t, []string{"pk", "p"}, versioned.Aliases)
	})

	t.Run("home-skill-aliases", func(t *testing.T) {
		peek, ok := skills.Find(list, skills.OriginHome, "peek")
		require.True(t, ok)
		assert.Equal(t, []string{"pk"}, peek.Aliases)
	})

	t.Run("alias-copy-not-listed", func(t *testing.T) {
		_, ok := skills.Find(list, skills.OriginHome, "pk")
		assert.False(t, ok)
	})

	t.Run("home-dir-named-like-no-alias-listed", func(t *testing.T) {
		_, ok := skills.Find(list, skills.OriginHome, "pkg")
		assert.True(t, ok)
	})

	t.Run("aliases-absent", func(t *testing.T) {
		pkg, ok := skills.Find(list, skills.OriginHome, "pkg")
		require.True(t, ok)
		assert.Nil(t, pkg.Aliases)
	})

	t.Run("aliases-stray-comma", func(t *testing.T) {
		commas, ok := skills.Find(list, skills.OriginHome, "commas")
		require.True(t, ok)
		assert.Equal(t, []string{"pk"}, commas.Aliases)
	})
}

func TestScanCacheReloadsAfterChange(t *testing.T) {
	repoRoot := t.TempDir()
	skillDir := filepath.Join(repoRoot, "changing")
	writeModelSkill(t, skillDir, "Before.", "")

	before, err := skills.Scan(repoRoot, t.TempDir())
	require.NoError(t, err)

	writeModelSkill(t, skillDir, "After the change.", "")
	modelPath := skillgraph.ModelPath(skillDir)
	later := time.Now().Add(time.Minute)
	require.NoError(t, os.Chtimes(modelPath, later, later))

	after, err := skills.Scan(repoRoot, t.TempDir())
	require.NoError(t, err)

	require.Len(t, before, 1)
	require.Len(t, after, 1)
	assert.Equal(t, "Before.", before[0].Description)
	assert.Equal(t, "After the change.", after[0].Description)
}

func TestScanGrouped(t *testing.T) {
	repoRoot, homeRoot := t.TempDir(), t.TempDir()
	writeModelSkill(t, filepath.Join(repoRoot, "util", "jq"), "Extract JSON fields with jq.", "")
	writeModelSkill(t, filepath.Join(repoRoot, "feature", "fdesign"), "Plan a feature.", "", "reference/notes.md")
	writeModelSkill(t, filepath.Join(repoRoot, "top-level"), "Ungrouped skill.", "")
	writeHomeSkill(t, filepath.Join(homeRoot, "jq"), "---\nname: jq\ndescription: Extract JSON fields with jq.\nversion: 1.0\n---\n")

	list, err := skills.Scan(repoRoot, homeRoot)
	require.NoError(t, err)
	require.Len(t, list, 4)

	jq, ok := skills.Find(list, skills.OriginRepo, "jq")
	require.True(t, ok)
	assert.Equal(t, filepath.Join(repoRoot, "util", "jq"), jq.Path)
	assert.True(t, jq.Synced, "grouped repo skill matches flat home skill by leaf name")

	homeJq, ok := skills.Find(list, skills.OriginHome, "jq")
	require.True(t, ok)
	assert.True(t, homeJq.Synced)

	fdesign, ok := skills.Find(list, skills.OriginRepo, "fdesign")
	require.True(t, ok)
	assert.Equal(t, []string{"fdesign.cue", filepath.Join("reference", "notes.md")}, fdesign.Files)
	assert.Equal(t, "feature", fdesign.Group)

	top, ok := skills.Find(list, skills.OriginRepo, "top-level")
	require.True(t, ok)
	assert.Empty(t, top.Group)
	assert.Empty(t, homeJq.Group, "home root is flat, never grouped")
}

func TestScanSyncedVersionAware(t *testing.T) {
	repoRoot, homeRoot := t.TempDir(), t.TempDir()
	writeVersionedModelSkill(t, filepath.Join(repoRoot, "bumped"), "---\nname: bumped\ndescription: d\nversion: 1.2\n---\n\n# Bumped\n", `[{"version": "1.2", "date": "2026-10-02", "text": "bumped"}]`)
	writeHomeSkill(t, filepath.Join(homeRoot, "bumped"), "---\nname: bumped\ndescription: d\nversion: 1.1\n---\n")
	writeModelSkill(t, filepath.Join(repoRoot, "even"), "d", "")
	writeHomeSkill(t, filepath.Join(homeRoot, "even"), "---\nname: even\ndescription: d\nversion: 1.0\n---\n")
	writeModelSkill(t, filepath.Join(repoRoot, "home-unversioned"), "d", "")
	writeHomeSkill(t, filepath.Join(homeRoot, "home-unversioned"), "---\nname: home-unversioned\ndescription: d\n---\n")
	writeModelSkill(t, filepath.Join(repoRoot, "repo-only"), "d", "")

	list, err := skills.Scan(repoRoot, homeRoot)
	require.NoError(t, err)

	for _, origin := range []string{skills.OriginHome, skills.OriginRepo} {
		bumped, ok := skills.Find(list, origin, "bumped")
		require.True(t, ok)
		assert.False(t, bumped.Synced, origin)
		even, ok := skills.Find(list, origin, "even")
		require.True(t, ok)
		assert.True(t, even.Synced, origin)
		homeUnversioned, ok := skills.Find(list, origin, "home-unversioned")
		require.True(t, ok)
		assert.False(t, homeUnversioned.Synced, origin)
	}
	repoOnly, ok := skills.Find(list, skills.OriginRepo, "repo-only")
	require.True(t, ok)
	assert.False(t, repoOnly.Synced)
}

// writeHomeSkill writes a deployed skill — a bare SKILL.md plus sibling
// files — into dir.
func writeHomeSkill(t *testing.T, dir, manifest string, siblings ...string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, skillgraph.FileManifest), []byte(manifest), 0o644))
	writeSiblings(t, dir, siblings)
}

// writeModelSkill writes a repo skill at version 1.0 as a saved model file;
// frontmatter holds the lines between name/description and version.
func writeModelSkill(t *testing.T, dir, description, frontmatter string, siblings ...string) {
	t.Helper()
	name := filepath.Base(dir)
	manifest := "---\nname: " + name + "\ndescription: " + description + "\n" + frontmatter + "version: 1.0\n---\n\n# " + name + "\n"
	writeVersionedModelSkill(t, dir, manifest, `[{"version": "1.0", "date": "2026-10-02", "text": "first"}]`, siblings...)
}

// writeVersionedModelSkill writes a repo skill as a saved model file: the
// markdown-era SKILL.md and changelog.json are imported, saved as a model,
// then removed.
func writeVersionedModelSkill(t *testing.T, dir, manifest, changelog string, siblings ...string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	manifestPath := filepath.Join(dir, skillgraph.FileManifest)
	changelogPath := filepath.Join(dir, "changelog.json")
	require.NoError(t, os.WriteFile(manifestPath, []byte(manifest), 0o644))
	require.NoError(t, os.WriteFile(changelogPath, []byte(changelog), 0o644))
	skill, err := skillimport.Import(dir)
	require.NoError(t, err)
	require.NoError(t, skillgraph.Save(dir, skill))
	require.NoError(t, os.Remove(manifestPath))
	require.NoError(t, os.Remove(changelogPath))
	writeSiblings(t, dir, siblings)
}

func writeSiblings(t *testing.T, dir string, siblings []string) {
	t.Helper()
	for _, sibling := range siblings {
		path := filepath.Join(dir, sibling)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte("content"), 0o644))
	}
}

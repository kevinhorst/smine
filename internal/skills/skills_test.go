package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeSkill(t *testing.T, root, name, description string, siblings ...string) {
	t.Helper()
	dir := filepath.Join(root, name)
	require.NoError(t, os.MkdirAll(dir, 0755))
	manifest := "---\nname: " + name + "\ndescription: " + description + "\n---\n\n# " + name + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(manifest), 0644))
	for _, sib := range siblings {
		path := filepath.Join(dir, sib)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		require.NoError(t, os.WriteFile(path, []byte("content"), 0644))
	}
}

func TestScanVersion(t *testing.T) {
	repoRoot := t.TempDir()
	dir := filepath.Join(repoRoot, "versioned")
	require.NoError(t, os.MkdirAll(dir, 0755))
	manifest := "---\nname: versioned\ndescription: d\nauthor: Kevin Horst\nversion: 1.2\nallowed-tools: Bash(jq *), Read\n---\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(manifest), 0644))

	list, err := Scan(repoRoot, t.TempDir())
	require.NoError(t, err)
	skill, ok := Find(list, "repo", "versioned")
	require.True(t, ok)
	assert.Equal(t, "1.2", skill.Version)
	assert.Equal(t, "Kevin Horst", skill.Author)
	assert.Equal(t, "Bash(jq *), Read", skill.AllowedTools)
}

func TestScanSyncedVersionAware(t *testing.T) {
	writeVersioned := func(t *testing.T, root, name, version string) {
		t.Helper()
		dir := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(dir, 0755))
		manifest := "---\nname: " + name + "\ndescription: d\nversion: " + version + "\n---\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(manifest), 0644))
	}

	repoRoot, homeRoot := t.TempDir(), t.TempDir()
	writeVersioned(t, repoRoot, "bumped", "1.2")
	writeVersioned(t, homeRoot, "bumped", "1.1")
	writeVersioned(t, repoRoot, "even", "1.0")
	writeVersioned(t, homeRoot, "even", "1.0")
	writeSkill(t, repoRoot, "unversioned", "d")
	writeSkill(t, homeRoot, "unversioned", "d")

	list, err := Scan(repoRoot, homeRoot)
	require.NoError(t, err)

	for _, origin := range []string{OriginHome, OriginRepo} {
		bumped, ok := Find(list, origin, "bumped")
		require.True(t, ok)
		assert.False(t, bumped.Synced, origin)
		even, ok := Find(list, origin, "even")
		require.True(t, ok)
		assert.True(t, even.Synced, origin)
		unversioned, ok := Find(list, origin, "unversioned")
		require.True(t, ok)
		assert.True(t, unversioned.Synced, origin)
	}
}

func TestScan(t *testing.T) {
	repoRoot, homeRoot := t.TempDir(), t.TempDir()
	writeSkill(t, repoRoot, "jq", "Extract JSON fields with jq.")
	writeSkill(t, repoRoot, "peek", "Show the latest session.")
	writeSkill(t, homeRoot, "peek", "Show the latest session.", "reference/schema.json")
	writeSkill(t, homeRoot, "home-only", "Only at home.")

	list, err := Scan(repoRoot, homeRoot)
	require.NoError(t, err)

	t.Run("repo-and-home-merged", func(t *testing.T) {
		require.Len(t, list, 4)
		var repoCount, homeCount int
		for _, s := range list {
			switch s.Origin {
			case "repo":
				repoCount++
			case "home":
				homeCount++
			}
		}
		assert.Equal(t, 2, repoCount)
		assert.Equal(t, 2, homeCount)
	})

	t.Run("synced-flag", func(t *testing.T) {
		peek, ok := Find(list, "home", "peek")
		require.True(t, ok)
		assert.True(t, peek.Synced)
		homeOnly, ok := Find(list, "home", "home-only")
		require.True(t, ok)
		assert.False(t, homeOnly.Synced)
		repoJq, ok := Find(list, "repo", "jq")
		require.True(t, ok)
		assert.False(t, repoJq.Synced)
		repoPeek, ok := Find(list, "repo", "peek")
		require.True(t, ok)
		assert.True(t, repoPeek.Synced)
	})

	t.Run("frontmatter-parsed", func(t *testing.T) {
		jq, ok := Find(list, "repo", "jq")
		require.True(t, ok)
		assert.Equal(t, "Extract JSON fields with jq.", jq.Description)
	})

	t.Run("absent-version-empty", func(t *testing.T) {
		jq, ok := Find(list, "repo", "jq")
		require.True(t, ok)
		assert.Empty(t, jq.Version)
		assert.Empty(t, jq.Author)
		assert.Empty(t, jq.AllowedTools)
	})

	t.Run("sibling-files-listed", func(t *testing.T) {
		peek, ok := Find(list, "home", "peek")
		require.True(t, ok)
		assert.Equal(t, []string{filepath.Join("reference", "schema.json")}, peek.Files)
		jq, ok := Find(list, "repo", "jq")
		require.True(t, ok)
		assert.Empty(t, jq.Files)
	})
}

func TestScanGrouped(t *testing.T) {
	repoRoot, homeRoot := t.TempDir(), t.TempDir()
	writeSkill(t, repoRoot, filepath.Join("util", "jq"), "Extract JSON fields with jq.")
	writeSkill(t, repoRoot, filepath.Join("feature", "fdesign"), "Plan a feature.", "reference/notes.md")
	writeSkill(t, repoRoot, "top-level", "Ungrouped skill.")
	writeSkill(t, homeRoot, "jq", "Extract JSON fields with jq.")

	list, err := Scan(repoRoot, homeRoot)
	require.NoError(t, err)
	require.Len(t, list, 4)

	jq, ok := Find(list, "repo", "jq")
	require.True(t, ok)
	assert.Equal(t, filepath.Join(repoRoot, "util", "jq"), jq.Path)
	assert.True(t, jq.Synced, "grouped repo skill matches flat home skill by leaf name")

	homeJq, ok := Find(list, "home", "jq")
	require.True(t, ok)
	assert.True(t, homeJq.Synced)

	fd, ok := Find(list, "repo", "fdesign")
	require.True(t, ok)
	assert.Equal(t, []string{filepath.Join("reference", "notes.md")}, fd.Files)
	assert.Equal(t, "feature", fd.Group)

	top, ok := Find(list, "repo", "top-level")
	require.True(t, ok)
	assert.Empty(t, top.Group)
	assert.Empty(t, homeJq.Group, "home root is flat, never grouped")
}

func TestParseFrontmatterFolded(t *testing.T) {
	folded := "---\nname: caveman\ndescription: >\n  Compress prose to the\n  bare minimum.\nauthor: X\n---\nbody\n"
	fm := parseFrontmatter(folded)
	assert.Equal(t, "caveman", fm.Name)
	assert.Equal(t, "Compress prose to the bare minimum.", fm.Description)
	assert.Equal(t, "X", fm.Author, "key after the fold still parsed")

	literal := "---\ndescription: |\n  Line one.\n  Line two.\n---\n"
	fm = parseFrontmatter(literal)
	assert.Equal(t, "Line one. Line two.", fm.Description)

	single := "---\ndescription: Plain single-line.\n---\n"
	fm = parseFrontmatter(single)
	assert.Equal(t, "Plain single-line.", fm.Description)

	terminated := "---\ndescription: >\nversion: 1.0\n---\n"
	fm = parseFrontmatter(terminated)
	assert.Empty(t, fm.Description, "next key at column 0 terminates an empty fold")
	assert.Equal(t, "1.0", fm.Version)
}

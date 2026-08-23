package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

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

func TestSplitDescription(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedArgs    []SkillArg
		_expectedSummary string

		description string
	}

	tests := make([]*testCase, 0)

	// full-convention
	tests = append(tests, &testCase{
		_id: "full-convention",
		_expectedArgs: []SkillArg{
			{Doc: "recent-turn count (default 5)", Name: "n"},
			{Doc: "mode token", Name: "list|plan|diff"},
		},
		_expectedSummary: "Show the latest session — turns, plan, diff, or list.",

		description: "Show the latest session — turns, plan, diff, or list. Trigger on /peek [n]. Args — n: recent-turn count (default 5); list|plan|diff: mode token.",
	})

	// no-args-segment
	tests = append(tests, &testCase{
		_id:              "no-args-segment",
		_expectedArgs:    nil,
		_expectedSummary: "Review code changes against project conventions.",

		description: "Review code changes against project conventions. Trigger on /railroad-review or \"review this branch\".",
	})

	// no-trigger-marker
	tests = append(tests, &testCase{
		_id:              "no-trigger-marker",
		_expectedArgs:    nil,
		_expectedSummary: "Compress prose to the bare minimum.",

		description: "Compress prose to the bare minimum.",
	})

	// trailing-whitespace-and-period
	tests = append(tests, &testCase{
		_id: "trailing-whitespace-and-period",
		_expectedArgs: []SkillArg{
			{Doc: "the one flag", Name: "flag"},
		},
		_expectedSummary: "Do the thing.",

		description: "Do the thing. Trigger on /thing. Args — flag: the one flag. ",
	})

	// arg-doc-with-colon
	tests = append(tests, &testCase{
		_id: "arg-doc-with-colon",
		_expectedArgs: []SkillArg{
			{Doc: "route selector: plan or skill", Name: "target"},
		},
		_expectedSummary: "Reformat a target.",

		description: "Reformat a target. Trigger on /fmt. Args — target: route selector: plan or skill.",
	})

	// empty-description
	tests = append(tests, &testCase{
		_id:              "empty-description",
		_expectedArgs:    nil,
		_expectedSummary: "",

		description: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			summary, args := splitDescription(test.description)
			assert.Equal(t, test._expectedSummary, summary)
			assert.Equal(t, test._expectedArgs, args)
		})
	}
}

func TestDescriptionConvention(t *testing.T) {
	const (
		maxDescriptionLen = 450
		maxSummaryLen     = 160
	)
	exempt := map[string]bool{
		"caveman": true,
		"jq":      true,
		"xlsx":    true,
	}

	repoSkillsRoot := filepath.Join("..", "..", "skills")
	require.DirExists(t, repoSkillsRoot)

	checked := 0
	err := filepath.WalkDir(repoSkillsRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "SKILL.md" {
			return err
		}
		name := filepath.Base(filepath.Dir(path))
		if exempt[name] {
			return nil
		}

		data, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		fm := parseFrontmatter(string(data))
		summary, _ := splitDescription(fm.Description)
		checked++

		assert.Containsf(t, fm.Description, ". Trigger on ", "%s: description misses the '. Trigger on' marker", name)
		assert.LessOrEqualf(t, utf8.RuneCountInString(fm.Description), maxDescriptionLen, "%s: description exceeds %d chars", name, maxDescriptionLen)
		assert.LessOrEqualf(t, utf8.RuneCountInString(summary), maxSummaryLen, "%s: first sentence exceeds %d chars", name, maxSummaryLen)
		if strings.Contains(string(data), "\n## Args") {
			assert.Containsf(t, fm.Description, "Args — ", "%s: has an Args section but no 'Args —' description segment", name)
		}
		return nil
	})
	require.NoError(t, err)
	assert.Greater(t, checked, 20, "repo skill sweep found implausibly few skills")
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

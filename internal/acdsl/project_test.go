package acdsl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func projectRules() []Rule {
	return []Rule{
		{Id: "B-002", Why: "second rule"},
		{Id: "A-001", Why: "first rule"},
	}
}

func goSyntax() projectionSyntax {
	syntax, _ := syntaxForPath("a.go")
	return syntax
}

func TestProjectionBlockSortedWithMarker(t *testing.T) {
	block := ProjectionBlock(projectRules(), goSyntax())
	require.Len(t, block, 4)
	assert.Equal(t, "// "+ProjectionMarker+" 2 rule(s) govern this file — working-copy view, stripped before commit", block[0])
	assert.Equal(t, "// - [A-001] first rule", block[1])
	assert.Equal(t, "// - [B-002] second rule", block[2])
	assert.Equal(t, "", block[3])
}

func TestProjectionBlockWrapsHtmlSyntax(t *testing.T) {
	syntax, ok := syntaxForPath("doc.md")
	require.True(t, ok)
	block := ProjectionBlock(projectRules(), syntax)
	require.Len(t, block, 4)
	assert.Equal(t, "<!-- "+ProjectionMarker+" 2 rule(s) govern this file — working-copy view, stripped before commit -->", block[0])
	assert.Equal(t, "<!-- - [A-001] first rule -->", block[1])
	assert.Equal(t, "", block[3])
}

func TestProjectionBlockEmptyForUngoverned(t *testing.T) {
	assert.Nil(t, ProjectionBlock(nil, goSyntax()))
}

func writeProjectFile(t *testing.T, content string) (string, string) {
	t.Helper()
	root := t.TempDir()
	path := "a.go"
	require.NoError(t, os.WriteFile(filepath.Join(root, path), []byte(content), 0o644))
	return root, path
}

func readBack(t *testing.T, root, path string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, path))
	require.NoError(t, err)
	return string(raw)
}

const originalContent = "package a\n\nfunc f() {}\n"

func TestProjectFileInsertsBlock(t *testing.T) {
	root, path := writeProjectFile(t, originalContent)
	changed, err := ProjectFile(root, path, projectRules())
	require.NoError(t, err)
	assert.True(t, changed)
	content := readBack(t, root, path)
	assert.True(t, strings.HasPrefix(content, "// "+ProjectionMarker))
	assert.Contains(t, content, "// - [A-001] first rule")
	assert.True(t, strings.HasSuffix(content, originalContent))
}

func TestProjectFileIdempotent(t *testing.T) {
	root, path := writeProjectFile(t, originalContent)
	_, err := ProjectFile(root, path, projectRules())
	require.NoError(t, err)
	first := readBack(t, root, path)

	changed, err := ProjectFile(root, path, projectRules())
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, first, readBack(t, root, path))
}

func TestProjectFileRefreshesStaleBlock(t *testing.T) {
	root, path := writeProjectFile(t, originalContent)
	_, err := ProjectFile(root, path, projectRules())
	require.NoError(t, err)

	changed, err := ProjectFile(root, path, []Rule{{Id: "C-003", Why: "new rule"}})
	require.NoError(t, err)
	assert.True(t, changed)
	content := readBack(t, root, path)
	assert.Contains(t, content, "C-003")
	assert.NotContains(t, content, "A-001")
	assert.True(t, strings.HasSuffix(content, originalContent))
}

func TestProjectFileStripRestoresOriginalBytes(t *testing.T) {
	root, path := writeProjectFile(t, originalContent)
	_, err := ProjectFile(root, path, projectRules())
	require.NoError(t, err)

	changed, err := ProjectFile(root, path, nil)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, originalContent, readBack(t, root, path))
}

func TestProjectFileUngovernedUntouched(t *testing.T) {
	root, path := writeProjectFile(t, originalContent)
	changed, err := ProjectFile(root, path, nil)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, originalContent, readBack(t, root, path))
}

func TestStripAll(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.go", "b.go", "c.go"} {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(originalContent), 0o644))
	}
	_, err := ProjectFile(root, "a.go", projectRules())
	require.NoError(t, err)
	_, err = ProjectFile(root, "b.go", projectRules())
	require.NoError(t, err)

	stripped, err := StripAll(root, []string{"a.go", "b.go", "c.go"})
	require.NoError(t, err)
	assert.Equal(t, 2, stripped)
	for _, name := range []string{"a.go", "b.go", "c.go"} {
		assert.Equal(t, originalContent, readBack(t, root, name))
	}
}

func TestProjectableSyntaxTable(t *testing.T) {
	assert.True(t, Projectable("cmd/acdsl/main.go"))
	assert.True(t, Projectable("A.GO"))
	assert.True(t, Projectable("cmd/hooks/guard.sh"))
	assert.True(t, Projectable("context/actions/implementing.md"))
	assert.True(t, Projectable("scripts/tool.py"))
	assert.True(t, Projectable("db/schema.sql"))
	assert.True(t, Projectable("config.yaml"))
	assert.True(t, Projectable("config.yml"))
	assert.True(t, Projectable("pyproject.toml"))
	assert.True(t, Projectable("page.html"))
	assert.True(t, Projectable("Makefile"))
	assert.False(t, Projectable("proposals/context.json"))
	assert.False(t, Projectable("logo.png"))
	assert.False(t, Projectable("acdsl/rules.acdsl"))
	assert.False(t, Projectable("LICENSE"))
}

func TestProjectFileShellShebangStaysLineOne(t *testing.T) {
	root := t.TempDir()
	original := "#!/usr/bin/env bash\necho ok\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "guard.sh"), []byte(original), 0o644))

	changed, err := ProjectFile(root, "guard.sh", projectRules())
	require.NoError(t, err)
	assert.True(t, changed)
	content := readBack(t, root, "guard.sh")
	lines := strings.Split(content, "\n")
	assert.Equal(t, "#!/usr/bin/env bash", lines[0])
	assert.Equal(t, "# "+ProjectionMarker+" 2 rule(s) govern this file — working-copy view, stripped before commit", lines[1])
	assert.Equal(t, "# - [A-001] first rule", lines[2])

	changed, err = ProjectFile(root, "guard.sh", projectRules())
	require.NoError(t, err)
	assert.False(t, changed)

	changed, err = ProjectFile(root, "guard.sh", nil)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, original, readBack(t, root, "guard.sh"))
}

func TestProjectFileMarkdownFrontmatter(t *testing.T) {
	root := t.TempDir()
	original := "---\nname: skill\n---\n# Title\nbody\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(original), 0o644))

	changed, err := ProjectFile(root, "SKILL.md", projectRules())
	require.NoError(t, err)
	assert.True(t, changed)
	lines := strings.Split(readBack(t, root, "SKILL.md"), "\n")
	assert.Equal(t, "---", lines[0])
	assert.Equal(t, "---", lines[2])
	assert.Equal(t, "<!-- "+ProjectionMarker+" 2 rule(s) govern this file — working-copy view, stripped before commit -->", lines[3])
	assert.Equal(t, "<!-- - [A-001] first rule -->", lines[4])
	assert.Equal(t, "# Title", lines[7])

	changed, err = ProjectFile(root, "SKILL.md", []Rule{{Id: "C-003", Why: "new rule"}})
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Contains(t, readBack(t, root, "SKILL.md"), "C-003")
	assert.NotContains(t, readBack(t, root, "SKILL.md"), "A-001")

	changed, err = ProjectFile(root, "SKILL.md", nil)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, original, readBack(t, root, "SKILL.md"))
}

func TestProjectFileMarkdownNoFrontmatter(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "doc.md"), []byte("# doc\n"), 0o644))
	changed, err := ProjectFile(root, "doc.md", projectRules())
	require.NoError(t, err)
	assert.True(t, changed)
	assert.True(t, strings.HasPrefix(readBack(t, root, "doc.md"), "<!-- "+ProjectionMarker))
}

func TestProjectFileMarkdownUnclosedFrontmatter(t *testing.T) {
	root := t.TempDir()
	original := "---\nname: broken\nno closing fence\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "doc.md"), []byte(original), 0o644))
	changed, err := ProjectFile(root, "doc.md", projectRules())
	require.NoError(t, err)
	assert.True(t, changed)
	content := readBack(t, root, "doc.md")
	assert.True(t, strings.HasPrefix(content, "<!-- "+ProjectionMarker))
	assert.True(t, strings.HasSuffix(content, original))
}

func TestProjectFileSqlAndMakefile(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "schema.sql"), []byte("SELECT 1;\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "Makefile"), []byte("all:\n\ttrue\n"), 0o644))

	changed, err := ProjectFile(root, "schema.sql", projectRules())
	require.NoError(t, err)
	assert.True(t, changed)
	assert.True(t, strings.HasPrefix(readBack(t, root, "schema.sql"), "-- "+ProjectionMarker))

	changed, err = ProjectFile(root, "Makefile", projectRules())
	require.NoError(t, err)
	assert.True(t, changed)
	assert.True(t, strings.HasPrefix(readBack(t, root, "Makefile"), "# "+ProjectionMarker))
}

func TestProjectFileSkipsCommentIncapable(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"data.json":  "{\"key\": 1}\n",
		"r.acdsl":    "//acdsl:X-001\n",
		"logo.png":   "\x89PNG\r\n",
		"no-ext-doc": "plain\n",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(content), 0o644))
		changed, err := ProjectFile(root, name, projectRules())
		require.NoError(t, err)
		assert.False(t, changed)
		assert.Equal(t, content, readBack(t, root, name))
	}
}

func TestStripAllMixedSyntaxes(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"a.go":     originalContent,
		"guard.sh": "#!/usr/bin/env bash\necho ok\n",
		"doc.md":   "# doc\n",
	}
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(content), 0o644))
		_, err := ProjectFile(root, name, projectRules())
		require.NoError(t, err)
	}

	stripped, err := StripAll(root, []string{"a.go", "guard.sh", "doc.md"})
	require.NoError(t, err)
	assert.Equal(t, 3, stripped)
	for name, content := range files {
		assert.Equal(t, content, readBack(t, root, name))
	}
}

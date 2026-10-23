package contextdocs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/trash"
)

func TestPackAlwaysEntriesKeepsOnlyAlwaysBlocks(t *testing.T) {
	content := "# Chapter\n\nProse paragraph.\n\n" +
		"**ACTION-IMPL-001** `[prose]` — Repo-pack entry.\n\n* Applies: always.\n\n" +
		"**ACTION-IMPL-002** `[prose]` — Always entry.\n\n* Applies: always.\n* Reach: always\n\n" +
		"**ACTION-IMPL-003** `[prose]` — List entry.\n\n* Applies: always.\n* Reach: work, peek-mcp\n\n" +
		"**ACTION-IMPL-004** `[prose]` — Disabled entry.\n\n* Applies: always.\n* Reach: none\n"
	out := PackAlwaysEntries(content)
	assert.Contains(t, out, "**ACTION-IMPL-002** `[prose]` — Always entry.")
	assert.Contains(t, out, "* Reach: always")
	assert.NotContains(t, out, "ACTION-IMPL-001")
	assert.NotContains(t, out, "ACTION-IMPL-003")
	assert.NotContains(t, out, "ACTION-IMPL-004")
	assert.NotContains(t, out, "# Chapter")
	assert.NotContains(t, out, "Prose paragraph.")
}

func TestPackAlwaysEntriesIgnoresFencesAndTombstones(t *testing.T) {
	content := "```markdown\n**ACTION-IMPL-098** `[prose]` — Example in fence.\n\n* Reach: always\n```\n\n" +
		"**ACTION-IMPL-002** `[prose]` — Always entry.\n\n* Reach: always\n\n" +
		"## Tombstones\n\n| Retired | Replacement | Date |\n|---|---|---|\n| ACTION-IMPL-009 | — | 2026-01-01 |\n"
	out := PackAlwaysEntries(content)
	assert.Contains(t, out, "ACTION-IMPL-002")
	assert.NotContains(t, out, "ACTION-IMPL-098")
	assert.NotContains(t, out, "Tombstones")
	assert.NotContains(t, out, "ACTION-IMPL-009")
}

func TestPackAlwaysEntriesKeepsFencedExample(t *testing.T) {
	content := "**ACTION-IMPL-002** `[prose]` — Always entry.\n\n* Applies: always.\n* Reach: always\n\n" +
		"```go\nexample()\n```\n\n* A late bullet.\n\n" +
		"**ACTION-IMPL-003** `[prose]` — Repo-pack entry.\n\n* Applies: always.\n"
	out := PackAlwaysEntries(content)
	assert.Contains(t, out, "example()")
	assert.Contains(t, out, "late bullet")
	assert.NotContains(t, out, "ACTION-IMPL-003")
}

func TestPackAlwaysEntriesEmptyWithoutAlwaysBlocks(t *testing.T) {
	content := "# Chapter\n\n**ACTION-IMPL-001** `[prose]` — Repo-pack entry.\n\n* Applies: always.\n"
	assert.Empty(t, PackAlwaysEntries(content))
}

func TestPackAlwaysDirConcatenatesInNameOrder(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "actions"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "facts"), 0o755))
	actionsFile := "**ACTION-IMPL-002** `[prose]` — Always action.\n\n* Reach: always\n"
	factsFile := "**FACT-CLAUDE-001** — Always fact.\n\n* Location: here\n* Reach: always\n"
	plainFile := "**FACT-CLAUDE-002** — Repo-pack fact.\n\n* Location: here\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "actions", "implementing.md"), []byte(actionsFile), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "facts", "claude.md"), []byte(factsFile), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "facts", "other.md"), []byte(plainFile), 0o644))

	out, err := PackAlwaysDir(dir)
	require.NoError(t, err)
	assert.Contains(t, out, "ACTION-IMPL-002")
	assert.Contains(t, out, "FACT-CLAUDE-001")
	assert.NotContains(t, out, "FACT-CLAUDE-002")
	// actions/ sorts before facts/, so the action block comes first
	assert.Less(t, strings.Index(out, "ACTION-IMPL-002"), strings.Index(out, "FACT-CLAUDE-001"))
}

func TestPackAlwaysDirEmptyTree(t *testing.T) {
	out, err := PackAlwaysDir(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestWriteAlwaysPackWritesHeaderAndPack(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "actions"), 0o755))
	actionsFile := "**ACTION-IMPL-002** `[prose]` — Always action.\n\n* Reach: always\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "actions", "implementing.md"), []byte(actionsFile), 0o644))
	packFile := filepath.Join(t.TempDir(), "global", "always-context.md")

	store, _ := provideTrashStore(t)

	isWritten, err := WriteAlwaysPack(dir, packFile, store)
	require.NoError(t, err)
	assert.True(t, isWritten)

	content, err := os.ReadFile(packFile)
	require.NoError(t, err)
	expected := alwaysPackHeader + "\n\n" + "**ACTION-IMPL-002** `[prose]` — Always action.\n\n* Reach: always\n"
	assert.Equal(t, expected, string(content))
}

func TestWriteAlwaysPackKeepsPrevious(t *testing.T) {
	dir := provideAlwaysContextDir(t)
	packFile := filepath.Join(t.TempDir(), "always-context.md")
	previous := AlwaysPackContent("**ACTION-IMPL-001** `[prose]` — Old action.")
	require.NoError(t, os.WriteFile(packFile, []byte(previous), 0o644))
	store, stateDir := provideTrashStore(t)

	isWritten, err := WriteAlwaysPack(dir, packFile, store)
	require.NoError(t, err)

	assert.True(t, isWritten)
	assert.Equal(t, []string{previous}, stashedContents(t, stateDir))
}

func TestWriteAlwaysPackSkipsEqualPack(t *testing.T) {
	dir := provideAlwaysContextDir(t)
	packFile := filepath.Join(t.TempDir(), "always-context.md")
	store, stateDir := provideTrashStore(t)
	_, err := WriteAlwaysPack(dir, packFile, store)
	require.NoError(t, err)

	isWritten, err := WriteAlwaysPack(dir, packFile, store)
	require.NoError(t, err)

	assert.False(t, isWritten)
	assert.Empty(t, stashedContents(t, stateDir))
}

func TestWriteAlwaysPackRefusesForeignFile(t *testing.T) {
	dir := provideAlwaysContextDir(t)
	packFile := filepath.Join(t.TempDir(), "always-context.md")
	require.NoError(t, os.WriteFile(packFile, []byte("my own notes"), 0o644))
	store, stateDir := provideTrashStore(t)

	isWritten, err := WriteAlwaysPack(dir, packFile, store)

	assert.ErrorContains(t, err, "kept: no smine header")
	assert.False(t, isWritten)
	content, readErr := os.ReadFile(packFile)
	require.NoError(t, readErr)
	assert.Equal(t, "my own notes", string(content))
	assert.Empty(t, stashedContents(t, stateDir))
}

func TestRemoveAlwaysPackRefusesForeignFile(t *testing.T) {
	packFile := filepath.Join(t.TempDir(), "always-context.md")
	require.NoError(t, os.WriteFile(packFile, []byte("my own notes"), 0o644))
	store, stateDir := provideTrashStore(t)

	err := RemoveAlwaysPack(packFile, store)

	assert.ErrorContains(t, err, "kept: no smine header")
	assert.FileExists(t, packFile)
	assert.Empty(t, stashedContents(t, stateDir))
}

func TestWriteAlwaysPackRemovesStalePack(t *testing.T) {
	packFile := filepath.Join(t.TempDir(), "always-context.md")
	stale := AlwaysPackContent("stale")
	require.NoError(t, os.WriteFile(packFile, []byte(stale), 0o644))
	store, stateDir := provideTrashStore(t)

	isWritten, err := WriteAlwaysPack(t.TempDir(), packFile, store)
	require.NoError(t, err)
	assert.False(t, isWritten)
	assert.NoFileExists(t, packFile)
	assert.Equal(t, []string{stale}, stashedContents(t, stateDir))
}

func TestWriteAlwaysPackNoPackNoEntries(t *testing.T) {
	packFile := filepath.Join(t.TempDir(), "always-context.md")
	store, _ := provideTrashStore(t)

	isWritten, err := WriteAlwaysPack(t.TempDir(), packFile, store)
	require.NoError(t, err)
	assert.False(t, isWritten)
	assert.NoFileExists(t, packFile)
}

func provideAlwaysContextDir(t *testing.T) string {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "actions"), 0o755))
	actionsFile := "**ACTION-IMPL-002** `[prose]` — Always action.\n\n* Reach: always\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "actions", "implementing.md"), []byte(actionsFile), 0o644))
	return dir
}

// provideTrashStore is a store on a fresh state dir, returned beside it for stashedContents.
func provideTrashStore(t *testing.T) (*trash.Store, string) {
	stateDir := t.TempDir()
	return trash.New(stateDir, 0), stateDir
}

// stashedContents reads every stashed file under the state dir, newest stamp first.
func stashedContents(t *testing.T, stateDir string) []string {
	stashes, err := trash.New(stateDir, 0).List()
	require.NoError(t, err)

	var contents []string
	for _, stash := range stashes {
		stampDir := filepath.Join(stateDir, trash.TrashDir, stash.Stamp)
		for _, entry := range stash.Entries {
			data, err := os.ReadFile(filepath.Join(stampDir, filepath.FromSlash(entry.Rel)))
			require.NoError(t, err)
			contents = append(contents, string(data))
		}
	}
	return contents
}

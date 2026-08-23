package contextdocs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFactsContext(t *testing.T, factsContent string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "facts"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "facts", "repo.md"), []byte(factsContent), 0644))
	return dir
}

func entryReach(t *testing.T, dir, id string) string {
	t.Helper()
	set, err := ParseContext(dir, false)
	require.NoError(t, err)
	for _, entry := range set.Entries {
		if entry.Id == id {
			return entry.Reach
		}
	}
	t.Fatalf("entry %s not found after edit", id)
	return ""
}

func TestSetEntryReachUpdatesExistingBullet(t *testing.T) {
	dir := writeFactsContext(t,
		"# Facts\n\n**FACT-REPO-STACK-001** — Go services.\n\n* Location: go.mod\n* Reach: global\n")

	require.NoError(t, SetEntryReach(dir, "FACT-REPO-STACK-001", "smine"))

	got, err := os.ReadFile(filepath.Join(dir, "facts", "repo.md"))
	require.NoError(t, err)
	assert.Equal(t,
		"# Facts\n\n**FACT-REPO-STACK-001** — Go services.\n\n* Location: go.mod\n* Reach: smine\n",
		string(got))
	assert.Equal(t, "smine", entryReach(t, dir, "FACT-REPO-STACK-001"))
}

func TestSetEntryReachInsertsAfterLastBullet(t *testing.T) {
	dir := writeFactsContext(t,
		"**FACT-REPO-STACK-001** — Go services.\n\n* Location: go.mod\n\n**FACT-REPO-ARCH-002** — Script-driven.\n\n* Location: cmd/sync\n")

	require.NoError(t, SetEntryReach(dir, "FACT-REPO-STACK-001", "none"))

	got, err := os.ReadFile(filepath.Join(dir, "facts", "repo.md"))
	require.NoError(t, err)
	assert.Equal(t,
		"**FACT-REPO-STACK-001** — Go services.\n\n* Location: go.mod\n* Reach: none\n\n**FACT-REPO-ARCH-002** — Script-driven.\n\n* Location: cmd/sync\n",
		string(got))
	assert.Equal(t, "none", entryReach(t, dir, "FACT-REPO-STACK-001"))
	assert.Equal(t, "global", entryReach(t, dir, "FACT-REPO-ARCH-002"))
}

func TestSetEntryReachInsertsIntoBulletlessBlock(t *testing.T) {
	dir := writeFactsContext(t,
		"**FACT-REPO-STACK-001** — Go services.\n\n**FACT-REPO-ARCH-002** — Script-driven.\n\n* Location: cmd/sync\n")

	require.NoError(t, SetEntryReach(dir, "FACT-REPO-STACK-001", "smine"))

	assert.Equal(t, "smine", entryReach(t, dir, "FACT-REPO-STACK-001"))
	assert.Equal(t, "global", entryReach(t, dir, "FACT-REPO-ARCH-002"))
}

func TestSetEntryReachErrors(t *testing.T) {
	dir := writeFactsContext(t, "**FACT-REPO-STACK-001** — Go services.\n\n* Location: go.mod\n")

	assert.Error(t, SetEntryReach(dir, "FACT-REPO-STACK-001", "aqms,global"), "invalid reach value")
	assert.Error(t, SetEntryReach(dir, "FACT-REPO-NOPE-999", "smine"), "unknown entry")
}

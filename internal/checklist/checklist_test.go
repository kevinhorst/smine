package checklist

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixture is a trimmed copy of the real checklist file including the &nbsp;
// separator and the legend that lacks [Blocked]/[Open].
const fixture = "# Claude Desktop — Workflow Changes\n" +
	"\n" +
	"**Status tags:** `[Done]` in place · `[Ongoing]` working pattern · `[Manual]` works but not automated · `[Untested]` speculative, unverified.\n" +
	"\n" +
	"## Overview\n" +
	"\n" +
	"| # | Change | Theme | Status |\n" +
	"| :--- | :--- | :--- | :--- |\n" +
	"| 1 | Audible cue on permission prompts | Reduce idle time | `[Done]` |\n" +
	"| 2 | Goland MCP in a worktree setup | Correctness | `[Blocked]` |\n" +
	"| 8 | Worktree awareness degradation | Correctness | `[Open]` |\n" +
	"\n" +
	"---\n" +
	"\n" +
	"## 1. Audible cue on permission prompts &nbsp; `[Done]`\n" +
	"\n" +
	"**Workflow**\n" +
	"Some body text with `[Done]` inline mention.\n" +
	"\n" +
	"## 2. Goland MCP in a worktree setup &nbsp; `[Blocked]`\n" +
	"\n" +
	"**Problem** body of entry two.\n" +
	"\n" +
	"## 8. Worktree awareness degradation &nbsp; `[Open]`\n" +
	"\n" +
	"**Solution** body of entry eight.\n"

func writeFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "checklist.md")
	require.NoError(t, os.WriteFile(path, []byte(fixture), 0644))
	return path
}

func TestParse(t *testing.T) {
	t.Run("entries-and-tags", func(t *testing.T) {
		cl, err := Parse(writeFixture(t))
		require.NoError(t, err)
		require.Len(t, cl.Entries, 3)
		assert.Equal(t, 1, cl.Entries[0].Number)
		assert.Equal(t, "Audible cue on permission prompts", cl.Entries[0].Title)
		assert.Equal(t, "Done", cl.Entries[0].Status)
		assert.Contains(t, cl.Entries[0].Body, "**Workflow**")
		assert.Equal(t, 8, cl.Entries[2].Number)
		assert.Equal(t, "Open", cl.Entries[2].Status)
	})

	t.Run("legend-union-observed", func(t *testing.T) {
		cl, err := Parse(writeFixture(t))
		require.NoError(t, err)
		// Legend: Done, Ongoing, Manual, Untested; observed adds Blocked, Open.
		for _, tag := range []string{"Done", "Ongoing", "Manual", "Untested", "Blocked", "Open"} {
			assert.Contains(t, cl.Tags, tag)
		}
		assert.Len(t, cl.Tags, 6)
	})
}

func TestSetStatus(t *testing.T) {
	t.Run("rewrites-exactly-two-lines", func(t *testing.T) {
		path := writeFixture(t)
		require.NoError(t, SetStatus(path, 8, "Untested"))

		after, err := os.ReadFile(path)
		require.NoError(t, err)
		beforeLines := strings.Split(fixture, "\n")
		afterLines := strings.Split(string(after), "\n")
		require.Equal(t, len(beforeLines), len(afterLines))

		var changed []string
		for i := range beforeLines {
			if beforeLines[i] != afterLines[i] {
				changed = append(changed, afterLines[i])
			}
		}
		require.Len(t, changed, 2)
		assert.Contains(t, changed[0], "| 8 | Worktree awareness degradation | Correctness | `[Untested]` |")
		assert.Contains(t, changed[1], "## 8. Worktree awareness degradation &nbsp; `[Untested]`")
	})

	t.Run("heading-missing-conflict", func(t *testing.T) {
		path := writeFixture(t)
		hacked := strings.Replace(fixture, "## 8. Worktree awareness degradation", "## 8. Renamed by hand", 1)
		require.NoError(t, os.WriteFile(path, []byte(hacked), 0644))

		// Overview row still matches but that's fine — the heading regex
		// still matches the renamed line. Break the heading shape instead.
		hacked = strings.Replace(fixture, " &nbsp; `[Open]`", "", 1)
		require.NoError(t, os.WriteFile(path, []byte(hacked), 0644))
		err := SetStatus(path, 8, "Done")
		assert.True(t, errors.Is(err, ErrEntryNotFound))
	})

	t.Run("tag-outside-set-rejected", func(t *testing.T) {
		path := writeFixture(t)
		err := SetStatus(path, 1, "Bogus")
		assert.True(t, errors.Is(err, ErrInvalidTag))
	})

	t.Run("body-untouched", func(t *testing.T) {
		path := writeFixture(t)
		require.NoError(t, SetStatus(path, 1, "Ongoing"))
		after, err := os.ReadFile(path)
		require.NoError(t, err)
		// The inline `[Done]` mention in the body must survive.
		assert.Contains(t, string(after), "Some body text with `[Done]` inline mention.")
	})
}

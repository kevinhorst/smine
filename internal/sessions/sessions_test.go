package sessions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validBatch is a trimmed example_batch_summary.json.
const validBatch = `{
  "batch": {
    "scope": "personal",
    "number": 1,
    "file": "sessions/personal/session-analysis-batch-01.md",
    "analyzedDate": "2026-07-03",
    "theme": "10 most recent Claude sessions"
  },
  "sessions": [
    {
      "id": "7a9b1ff1-1bf8-4c51-9869-c6f8364d05de",
      "title": "peek-mcp depth flag",
      "repo": "peek-mcp",
      "signal": "high",
      "findings": [
        {
          "dimension": "memory",
          "summary": "worktree discipline decays over long sessions",
          "quotes": ["re-anchor before acting"],
          "snippets": [
            {"kind": "violation", "lang": "go", "code": "func broken() {}", "source": "transcript"}
          ]
        }
      ],
      "frustration": [
        {"quote": "you edited the main checkout again", "trigger": "worktree path confusion"}
      ],
      "positive": [
        {"quote": "exactly what I wanted", "trigger": "clean first attempt"}
      ]
    },
    {
      "id": "109bf289-fc70-47e9-be4a-fe291e77bcff",
      "skipped": true,
      "skipReason": "trivial session"
    }
  ],
  "arcs": [
    {"sessionIds": ["7a9b1ff1-1bf8-4c51-9869-c6f8364d05de"], "summary": "fimplement meta-workflow"}
  ]
}`

func writeScope(t *testing.T, root, scope string, files map[string]string, mdReports int) {
	t.Helper()
	jsonDir := filepath.Join(root, scope, "json")
	require.NoError(t, os.MkdirAll(jsonDir, 0755))
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(jsonDir, name), []byte(content), 0644))
	}
	for i := 0; i < mdReports; i++ {
		name := filepath.Join(root, scope, "session-analysis-batch-0"+string(rune('1'+i))+".md")
		require.NoError(t, os.WriteFile(name, []byte("# report"), 0644))
	}
}

func TestBatchTitle(t *testing.T) {
	root := t.TempDir()
	withTitle := `{"batch": {"scope": "personal", "title": "The one with a title", "number": 2, "file": "b2.md"}, "sessions": []}`
	writeScope(t, root, "personal", map[string]string{"batch-02.json": withTitle}, 0)

	store := NewStore(root)
	require.NoError(t, store.Reload())

	batch, ok := store.Batch("personal", 2)
	require.True(t, ok)
	assert.Equal(t, "The one with a title", batch.Batch.Title)
}

func TestSessionRefs(t *testing.T) {
	t.Run("maps-ids-across-scopes", func(t *testing.T) {
		root := t.TempDir()
		workBatch := `{"batch": {"scope": "work", "number": 3, "file": "b3.md"}, "sessions": [{"id": "65a26e92-4c98-4879-82ce-35e644cd0ab5"}]}`
		writeScope(t, root, "personal", map[string]string{"batch-01.json": validBatch}, 0)
		writeScope(t, root, "work", map[string]string{"batch-03.json": workBatch}, 0)

		store := NewStore(root)
		require.NoError(t, store.Reload())

		refs := store.SessionRefs()
		assert.Equal(t, SessionRef{BatchNumber: 1, Scope: "personal"}, refs["7a9b1ff1-1bf8-4c51-9869-c6f8364d05de"])
		assert.Equal(t, SessionRef{BatchNumber: 1, Scope: "personal"}, refs["109bf289-fc70-47e9-be4a-fe291e77bcff"])
		assert.Equal(t, SessionRef{BatchNumber: 3, Scope: "work"}, refs["65a26e92-4c98-4879-82ce-35e644cd0ab5"])
	})

	t.Run("empty-store", func(t *testing.T) {
		store := NewStore(filepath.Join(t.TempDir(), "does-not-exist"))
		require.NoError(t, store.Reload())
		assert.Empty(t, store.SessionRefs())
	})
}

func TestStoreReload(t *testing.T) {
	t.Run("loads-all-scopes", func(t *testing.T) {
		root := t.TempDir()
		writeScope(t, root, "personal", map[string]string{"batch-01.json": validBatch}, 1)
		writeScope(t, root, "work", map[string]string{"batch-01.json": validBatch}, 0)

		store := NewStore(root)
		require.NoError(t, store.Reload())

		scopes := store.Scopes()
		require.Len(t, scopes, 2)
		assert.Equal(t, "personal", scopes[0].Name)
		assert.Equal(t, "work", scopes[1].Name)

		batch, ok := store.Batch("personal", 1)
		require.True(t, ok)
		assert.Equal(t, "personal", batch.Batch.Scope)
		require.Len(t, batch.Sessions, 2)
		assert.Equal(t, "peek-mcp depth flag", batch.Sessions[0].Title)
		assert.Equal(t, "memory", batch.Sessions[0].Findings[0].Dimension)
		require.Len(t, batch.Sessions[0].Findings[0].Snippets, 1)
		snippet := batch.Sessions[0].Findings[0].Snippets[0]
		assert.Equal(t, "violation", snippet.Kind)
		assert.Equal(t, "go", snippet.Lang)
		assert.Equal(t, "func broken() {}", snippet.Code)
		assert.Equal(t, "transcript", snippet.Source)
		require.Len(t, batch.Sessions[0].Positive, 1)
		assert.Equal(t, "exactly what I wanted", batch.Sessions[0].Positive[0].Quote)
		assert.Equal(t, "clean first attempt", batch.Sessions[0].Positive[0].Trigger)
		assert.True(t, batch.Sessions[1].Skipped)
		require.Len(t, batch.Arcs, 1)

		_, ok = store.Batch("personal", 99)
		assert.False(t, ok)
	})

	t.Run("malformed-file-skipped-and-recorded", func(t *testing.T) {
		root := t.TempDir()
		writeScope(t, root, "personal", map[string]string{
			"batch-01.json": validBatch,
			"broken.json":   "{not json",
		}, 0)

		store := NewStore(root)
		require.NoError(t, store.Reload())

		info, ok := store.Scope("personal")
		require.True(t, ok)
		assert.Len(t, info.Batches, 1)
		require.Len(t, info.LoadErrors, 1)
		assert.Contains(t, info.LoadErrors[0], "broken.json")
	})

	t.Run("empty-dir", func(t *testing.T) {
		store := NewStore(filepath.Join(t.TempDir(), "does-not-exist"))
		require.NoError(t, store.Reload())
		assert.Empty(t, store.Scopes())
	})

	t.Run("proposals-dir-skipped", func(t *testing.T) {
		root := t.TempDir()
		writeScope(t, root, "personal", map[string]string{"batch-01.json": validBatch}, 0)
		writeScope(t, root, "proposals", map[string]string{"routines.json": `{"kind": "routines"}`}, 0)

		store := NewStore(root)
		require.NoError(t, store.Reload())

		scopes := store.Scopes()
		require.Len(t, scopes, 1)
		assert.Equal(t, "personal", scopes[0].Name)
	})

	t.Run("scope-without-json-counts-md-reports", func(t *testing.T) {
		root := t.TempDir()
		scopeDir := filepath.Join(root, "personal")
		require.NoError(t, os.MkdirAll(scopeDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(scopeDir, "session-analysis-batch-01.md"), []byte("# r"), 0644))

		store := NewStore(root)
		require.NoError(t, store.Reload())

		info, ok := store.Scope("personal")
		require.True(t, ok)
		assert.Empty(t, info.Batches)
		assert.Equal(t, 1, info.MdReports)
	})
}

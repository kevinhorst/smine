package sessions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeBatch(t *testing.T, dir, scope, name, content string) {
	t.Helper()
	jsonDir := filepath.Join(dir, scope, "json")
	require.NoError(t, os.MkdirAll(jsonDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(jsonDir, name), []byte(content), 0o644))
}

func TestInvocationsBySkill(t *testing.T) {
	dir := t.TempDir()
	writeBatch(t, dir, "personal", "batch-01.json", `{
		"batch": {"scope": "personal", "file": "b1.md", "number": 1, "analyzedDate": "2026-07-10"},
		"sessions": [
			{"id": "00000000-0000-0000-0000-000000000001", "title": "s1", "invoked_skills": ["jq", "peek"]},
			{"id": "00000000-0000-0000-0000-000000000002", "title": "s2"}
		]
	}`)
	writeBatch(t, dir, "work", "batch-02.json", `{
		"batch": {"scope": "work", "file": "b2.md", "number": 2, "analyzedDate": "2026-07-12"},
		"sessions": [
			{"id": "00000000-0000-0000-0000-000000000003", "title": "s3", "invoked_skills": ["jq"]}
		]
	}`)

	store := NewStore(dir)
	require.NoError(t, store.Reload())

	invocations := store.InvocationsBySkill()
	require.Len(t, invocations["jq"], 2)
	require.Len(t, invocations["peek"], 1)

	peek := invocations["peek"][0]
	assert.Equal(t, "personal", peek.Scope)
	assert.Equal(t, 1, peek.BatchNumber)
	assert.Equal(t, "2026-07-10", peek.BatchDate)
	assert.Equal(t, "s1", peek.SessionTitle)
	assert.Equal(t, "00000000-0000-0000-0000-000000000001", peek.SessionId)
}

func TestInvocationsBySkillOldBatchContributesNothing(t *testing.T) {
	dir := t.TempDir()
	writeBatch(t, dir, "personal", "batch-01.json", `{
		"batch": {"scope": "personal", "file": "b1.md", "number": 1},
		"sessions": [{"id": "00000000-0000-0000-0000-000000000001", "title": "s1"}]
	}`)

	store := NewStore(dir)
	require.NoError(t, store.Reload())
	assert.Empty(t, store.InvocationsBySkill())
}

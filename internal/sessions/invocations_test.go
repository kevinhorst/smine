package sessions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvocationsBySkill(t *testing.T) {
	dir := t.TempDir()
	writeRecord(t, dir, "peek-mcp", weekNewer, "s1.json", `{"id": "s1", "agent": "claude", "title": "t1", "repo": "peek-mcp",
		"week": "2026-W39", "lastActive": "2026-09-22T10:00:00Z", "findings": [], "invoked_skills": ["jq", "peek"]}`)
	writeRecord(t, dir, "peek-mcp", weekNewer, "s2.json", skippedRecordJson("s2", "2026-09-23T10:00:00Z", weekNewer))
	writeRecord(t, dir, "go", weekOlder, "s3.json", `{"id": "s3", "agent": "claude", "title": "t3", "repo": "go",
		"week": "2026-W38", "lastActive": "2026-09-15T10:00:00Z", "findings": [], "invoked_skills": ["jq"]}`)

	store := NewStore(dir, nil)
	require.NoError(t, store.Reload())

	invocations := store.InvocationsBySkill()
	require.Len(t, invocations["jq"], 2)
	require.Len(t, invocations["peek"], 1)

	expected := Invocation{
		LastActive:   "2026-09-22T10:00:00Z",
		Scope:        "peek-mcp",
		SessionId:    "s1",
		SessionTitle: "t1",
		Week:         weekNewer,
	}
	assert.Equal(t, expected, invocations["peek"][0])
}

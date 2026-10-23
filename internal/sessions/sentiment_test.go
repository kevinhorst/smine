package sessions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSentimentByWeek(t *testing.T) {
	dir := t.TempDir()
	writeRecord(t, dir, "peek-mcp", weekNewer, "s1.json", `{"id": "s1", "agent": "claude", "title": "s1", "repo": "peek-mcp",
		"week": "2026-W39", "lastActive": "2026-09-22T10:00:00Z", "findings": [],
		"frustration": [{"quote": "argh", "trigger": "t"}, {"quote": "ugh", "trigger": "t"}],
		"positive": [{"quote": "nice", "trigger": "t"}]}`)
	writeRecord(t, dir, "peek-mcp", weekNewer, "s2.json", `{"id": "s2", "agent": "claude", "title": "s2", "repo": "peek-mcp",
		"week": "2026-W39", "lastActive": "2026-09-23T10:00:00Z", "findings": [], "skipped": true,
		"frustration": [{"quote": "skipped anger", "trigger": "t"}]}`)
	writeRecord(t, dir, "go", weekNewer, "s3.json", `{"id": "s3", "agent": "claude", "title": "s3", "repo": "go",
		"week": "2026-W39", "lastActive": "2026-09-24T10:00:00Z", "findings": [],
		"frustration": [{"quote": "hm", "trigger": "t"}]}`)
	writeRecord(t, dir, "go", weekOlder, "s4.json", skippedRecordJson("s4", "2026-09-15T10:00:00Z", weekOlder))

	store := NewStore(dir, nil)
	require.NoError(t, store.Reload())

	points := store.SentimentByWeek()
	require.Len(t, points, 2)

	// the older week carries only a skipped session: zero, listed first.
	assert.Equal(t, SentimentPoint{Frustration: 0, Positive: 0, Week: weekOlder}, points[0])
	// the newer week merges both scopes; the skipped session contributes nothing.
	assert.Equal(t, SentimentPoint{Frustration: 3, Positive: 1, Week: weekNewer}, points[1])
}

func TestSentimentByWeekEmpty(t *testing.T) {
	store := NewStore(t.TempDir(), nil)
	require.NoError(t, store.Reload())
	assert.Empty(t, store.SentimentByWeek())
}

package sessions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSentimentByBatch(t *testing.T) {
	dir := t.TempDir()
	writeBatch(t, dir, "personal", "batch-01.json", `{
		"batch": {"scope": "personal", "file": "b1.md", "number": 1, "analyzedDate": "2026-07-10"},
		"sessions": [
			{"id": "00000000-0000-0000-0000-000000000001", "title": "s1",
				"frustration": [{"quote": "argh", "trigger": "t"}, {"quote": "ugh", "trigger": "t"}],
				"positive": [{"quote": "nice", "trigger": "t"}]},
			{"id": "00000000-0000-0000-0000-000000000002", "title": "s2", "skipped": true,
				"frustration": [{"quote": "skipped anger", "trigger": "t"}]}
		]
	}`)
	writeBatch(t, dir, "work", "batch-01.json", `{
		"batch": {"scope": "work", "file": "b1.md", "number": 1, "analyzedDate": "2026-07-11"},
		"sessions": [
			{"id": "00000000-0000-0000-0000-000000000003", "title": "s3",
				"frustration": [{"quote": "hm", "trigger": "t"}]}
		]
	}`)
	writeBatch(t, dir, "work", "batch-02.json", `{
		"batch": {"scope": "work", "file": "b2.md", "number": 2, "analyzedDate": "2026-07-12"},
		"sessions": [
			{"id": "00000000-0000-0000-0000-000000000004", "title": "s4"}
		]
	}`)

	store := NewStore(dir)
	require.NoError(t, store.Reload())

	points := store.SentimentByBatch()
	require.Len(t, points, 2)

	// batch 1 merges both scopes; the skipped session contributes nothing.
	assert.Equal(t, SentimentPoint{BatchNumber: 1, Frustration: 3, Positive: 1}, points[0])
	// batch 2 has no sentiment fields at all.
	assert.Equal(t, SentimentPoint{BatchNumber: 2, Frustration: 0, Positive: 0}, points[1])
}

func TestSentimentByBatchEmpty(t *testing.T) {
	store := NewStore(t.TempDir())
	require.NoError(t, store.Reload())
	assert.Empty(t, store.SentimentByBatch())
}

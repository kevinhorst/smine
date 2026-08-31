package server

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kevinhorst/smine/internal/repos"
)

func TestStatusCache(t *testing.T) {
	cache := newStatusCache()

	_, ok := cache.Get("demo")
	assert.False(t, ok)

	first := []repos.WorktreeStatus{{Branch: "claude/alpha"}}
	stored := cache.Store("demo", "fp-1", first)
	assert.False(t, stored.ScannedAt.IsZero())

	entry, ok := cache.Get("demo")
	assert.True(t, ok)
	assert.Equal(t, first, entry.Statuses)
	assert.Equal(t, "fp-1", entry.Fingerprint)

	// a new scan replaces the entry wholesale
	second := []repos.WorktreeStatus{{Branch: "claude/beta"}}
	cache.Store("demo", "fp-1", second)
	entry, ok = cache.Get("demo")
	assert.True(t, ok)
	assert.Equal(t, second, entry.Statuses)
}

func TestStatusCacheDropBranches(t *testing.T) {
	cache := newStatusCache()
	statuses := []repos.WorktreeStatus{
		{Branch: "claude/alpha"},
		{Branch: "claude/beta"},
		{Branch: "claude-routines/nightly"},
	}
	stored := cache.Store("demo", "fp-1", statuses)

	cache.DropBranches("demo", []string{"claude/alpha", "claude-routines/nightly"})

	entry, ok := cache.Get("demo")
	assert.True(t, ok)
	kept := []repos.WorktreeStatus{{Branch: "claude/beta"}}
	assert.Equal(t, kept, entry.Statuses)
	assert.Equal(t, stored.ScannedAt, entry.ScannedAt)
	// the pre-removal fingerprint is kept so the next plain load re-scans
	assert.Equal(t, "fp-1", entry.Fingerprint)

	// unknown repo is a no-op
	cache.DropBranches("nope", []string{"claude/alpha"})
	_, ok = cache.Get("nope")
	assert.False(t, ok)
}

func TestStatusCacheDelete(t *testing.T) {
	cache := newStatusCache()
	cache.Store("demo", "fp-1", []repos.WorktreeStatus{{Branch: "claude/alpha"}})

	cache.Delete("demo")
	_, ok := cache.Get("demo")
	assert.False(t, ok)

	// unknown repo is a no-op
	cache.Delete("nope")
	_, ok = cache.Get("nope")
	assert.False(t, ok)
}

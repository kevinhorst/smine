package server

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kevinhorst/smine/internal/repos"
)

func TestStatusCache(t *testing.T) {
	cache := newStatusCache()
	claude := statusKey{Kind: "claude", Repo: "demo"}

	_, ok := cache.Get(claude)
	assert.False(t, ok)

	first := &repos.StatusScan{
		Kinds:   []string{"claude"},
		Rows:    []repos.WorktreeStatus{{Branch: "claude/alpha"}},
		Timings: repos.StatusTimings{AnalysisMs: 540, OrderMs: 77, WorktreesMs: 213},
	}
	stored := cache.Store(claude, "fp-1", first)
	assert.False(t, stored.ScannedAt.IsZero())

	entry, ok := cache.Get(claude)
	assert.True(t, ok)
	assert.Equal(t, first.Rows, entry.Statuses)
	assert.Equal(t, first.Kinds, entry.Kinds)
	assert.Equal(t, "fp-1", entry.Fingerprint)
	assert.Equal(t, first.Timings, entry.Timings)

	// a new scan replaces the scope wholesale
	second := &repos.StatusScan{Rows: []repos.WorktreeStatus{{Branch: "claude/beta"}}}
	cache.Store(claude, "fp-1", second)
	entry, ok = cache.Get(claude)
	assert.True(t, ok)
	assert.Equal(t, second.Rows, entry.Statuses)
	assert.Nil(t, entry.Kinds)
	assert.Equal(t, repos.StatusTimings{}, entry.Timings)
}

func TestStatusCacheScopes(t *testing.T) {
	cache := newStatusCache()
	claude := statusKey{Kind: "claude", Repo: "demo"}
	all := statusKey{Kind: statusKindAll, Repo: "demo"}

	claudeScan := &repos.StatusScan{
		Kinds: []string{"claude", "claude-routines"},
		Rows:  []repos.WorktreeStatus{{Branch: "claude/alpha"}},
	}
	allScan := &repos.StatusScan{
		Kinds: []string{"claude", "claude-routines", "codex"},
		Rows: []repos.WorktreeStatus{
			{Branch: "claude/alpha"},
			{Branch: "claude-routines/nightly"},
		},
	}
	cache.Store(claude, "fp-claude", claudeScan)
	cache.Store(all, "fp-all", allScan)

	// both scopes of one repo are held side by side, each with its own kinds
	claudeEntry, ok := cache.Get(claude)
	assert.True(t, ok)
	assert.Equal(t, claudeScan.Rows, claudeEntry.Statuses)
	assert.Equal(t, claudeScan.Kinds, claudeEntry.Kinds)
	assert.Equal(t, "fp-claude", claudeEntry.Fingerprint)

	allEntry, ok := cache.Get(all)
	assert.True(t, ok)
	assert.Equal(t, allScan.Rows, allEntry.Statuses)
	assert.Equal(t, allScan.Kinds, allEntry.Kinds)
	assert.Equal(t, "fp-all", allEntry.Fingerprint)

	// a scope never stored is a miss
	routines := statusKey{Kind: "claude-routines", Repo: "demo"}
	_, ok = cache.Get(routines)
	assert.False(t, ok)
}

func TestStatusCacheDropBranches(t *testing.T) {
	cache := newStatusCache()
	claude := statusKey{Kind: "claude", Repo: "demo"}
	all := statusKey{Kind: statusKindAll, Repo: "demo"}
	other := statusKey{Kind: "claude", Repo: "other"}

	claudeScan := &repos.StatusScan{Rows: []repos.WorktreeStatus{
		{Branch: "claude/alpha"},
		{Branch: "claude/beta"},
	}}
	allScan := &repos.StatusScan{Rows: []repos.WorktreeStatus{
		{Branch: "claude/alpha"},
		{Branch: "claude/beta"},
		{Branch: "claude-routines/nightly"},
	}}
	otherScan := &repos.StatusScan{Rows: []repos.WorktreeStatus{{Branch: "claude/alpha"}}}
	claudeStored := cache.Store(claude, "fp-claude", claudeScan)
	allStored := cache.Store(all, "fp-all", allScan)
	cache.Store(other, "fp-other", otherScan)

	dropped := []string{"claude/alpha", "claude-routines/nightly"}
	cache.DropBranches("demo", dropped)

	// rows dropped in every scope of the repo
	kept := []repos.WorktreeStatus{{Branch: "claude/beta"}}
	claudeEntry, ok := cache.Get(claude)
	assert.True(t, ok)
	assert.Equal(t, kept, claudeEntry.Statuses)
	allEntry, ok := cache.Get(all)
	assert.True(t, ok)
	assert.Equal(t, kept, allEntry.Statuses)

	// ScannedAt and the pre-removal fingerprint are kept so the next plain load re-scans
	assert.Equal(t, claudeStored.ScannedAt, claudeEntry.ScannedAt)
	assert.Equal(t, "fp-claude", claudeEntry.Fingerprint)
	assert.Equal(t, allStored.ScannedAt, allEntry.ScannedAt)
	assert.Equal(t, "fp-all", allEntry.Fingerprint)

	// another repo is untouched
	otherEntry, ok := cache.Get(other)
	assert.True(t, ok)
	assert.Equal(t, otherScan.Rows, otherEntry.Statuses)

	// unknown repo is a no-op
	cache.DropBranches("nope", dropped)
	unknown := statusKey{Kind: "claude", Repo: "nope"}
	_, ok = cache.Get(unknown)
	assert.False(t, ok)
}

func TestStatusCacheDelete(t *testing.T) {
	cache := newStatusCache()
	claude := statusKey{Kind: "claude", Repo: "demo"}
	all := statusKey{Kind: statusKindAll, Repo: "demo"}
	other := statusKey{Kind: "claude", Repo: "other"}

	scan := &repos.StatusScan{Rows: []repos.WorktreeStatus{{Branch: "claude/alpha"}}}
	cache.Store(claude, "fp-1", scan)
	cache.Store(all, "fp-1", scan)
	cache.Store(other, "fp-1", scan)

	// every scope of the repo is dropped
	cache.Delete("demo")
	_, ok := cache.Get(claude)
	assert.False(t, ok)
	_, ok = cache.Get(all)
	assert.False(t, ok)

	// another repo is kept
	otherEntry, ok := cache.Get(other)
	assert.True(t, ok)
	assert.Equal(t, scan.Rows, otherEntry.Statuses)

	// unknown repo is a no-op
	cache.Delete("nope")
	unknown := statusKey{Kind: "claude", Repo: "nope"}
	_, ok = cache.Get(unknown)
	assert.False(t, ok)
	_, ok = cache.Get(other)
	assert.True(t, ok)
}

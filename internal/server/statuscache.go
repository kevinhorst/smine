package server

import (
	"slices"
	"sync"
	"time"

	"github.com/kevinhorst/smine/internal/repos"
)

type statusEntry struct {
	Fingerprint string
	Kinds       []string
	ScannedAt   time.Time
	Statuses    []repos.WorktreeStatus
	Timings     repos.StatusTimings
}

type statusKey struct {
	Kind string
	Repo string
}

// statusCache treats stored slices as immutable: renders read, never mutate.
type statusCache struct {
	// mu guards entries (declared directly above the map it guards).
	mu      sync.RWMutex
	entries map[statusKey]statusEntry
}

func newStatusCache() *statusCache {
	cache := &statusCache{entries: make(map[statusKey]statusEntry)}
	return cache
}

// Delete drops every scope of a repo (registry removal).
func (c *statusCache) Delete(repoName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.entries {
		if key.Repo == repoName {
			delete(c.entries, key)
		}
	}
}

// DropBranches keeps the pre-removal Fingerprint on purpose, so the next plain load mismatches and re-scans.
func (c *statusCache) DropBranches(repoName string, branches []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, entry := range c.entries {
		if key.Repo != repoName {
			continue
		}
		c.entries[key] = droppedBranches(branches, entry)
	}
}

func (c *statusCache) Get(key statusKey) (statusEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[key]
	return entry, ok
}

func (c *statusCache) Store(key statusKey, fingerprint string, scan *repos.StatusScan) statusEntry {
	entry := statusEntry{
		Fingerprint: fingerprint,
		Kinds:       scan.Kinds,
		ScannedAt:   time.Now(),
		Statuses:    scan.Rows,
		Timings:     scan.Timings,
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = entry
	return entry
}

// droppedBranches builds a fresh slice because stored slices stay immutable.
func droppedBranches(branches []string, entry statusEntry) statusEntry {
	kept := make([]repos.WorktreeStatus, 0, len(entry.Statuses))
	for _, status := range entry.Statuses {
		if !slices.Contains(branches, status.Branch) {
			kept = append(kept, status)
		}
	}
	entry.Statuses = kept
	return entry
}

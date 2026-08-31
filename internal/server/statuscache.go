package server

import (
	"slices"
	"sync"
	"time"

	"github.com/kevinhorst/smine/internal/repos"
)

type statusEntry struct {
	Fingerprint string
	ScannedAt   time.Time
	Statuses    []repos.WorktreeStatus
}

// statusCache holds the last parsed worktree scan per repo so plain fragment
// loads render without a subprocess; ?refresh=1 replaces the entry wholesale.
// Stored slices are treated as immutable — renders read, never mutate.
type statusCache struct {
	// mu guards entries (declared directly above the map it guards).
	mu      sync.RWMutex
	entries map[string]statusEntry
}

func newStatusCache() *statusCache {
	cache := &statusCache{entries: make(map[string]statusEntry)}
	return cache
}

// Delete drops a repo's entry entirely (registry removal).
func (c *statusCache) Delete(repoName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, repoName)
}

// DropBranches removes the given branches' rows from the cached entry so
// plain loads agree with an optimistic removal immediately. The entry is
// replaced with a freshly built slice (stored slices stay immutable);
// ScannedAt is kept — it is still the last scan, minus removed rows. The
// pre-removal Fingerprint is kept too, deliberately: the removal changed the
// real ref/worktree set, so the next plain load mismatches and re-scans.
func (c *statusCache) DropBranches(repoName string, branches []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[repoName]
	if !ok {
		return
	}
	kept := make([]repos.WorktreeStatus, 0, len(entry.Statuses))
	for _, status := range entry.Statuses {
		if !slices.Contains(branches, status.Branch) {
			kept = append(kept, status)
		}
	}
	entry.Statuses = kept
	c.entries[repoName] = entry
}

func (c *statusCache) Get(repoName string) (statusEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[repoName]
	return entry, ok
}

func (c *statusCache) Store(repoName, fingerprint string, statuses []repos.WorktreeStatus) statusEntry {
	entry := statusEntry{Fingerprint: fingerprint, ScannedAt: time.Now(), Statuses: statuses}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[repoName] = entry
	return entry
}

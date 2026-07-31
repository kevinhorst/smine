package repos

import "sync"

// Locks serializes mutations per repo: one running operation per repo name,
// concurrent attempts get 409 instead of queueing (D20).
type Locks struct {
	// mu guards held.
	mu   sync.Mutex
	held map[string]bool
}

func NewLocks() *Locks {
	return &Locks{held: make(map[string]bool)}
}

// TryAcquire claims the repo for one mutation; false means an operation is
// already running.
func (l *Locks) TryAcquire(name string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.held[name] {
		return false
	}
	l.held[name] = true
	return true
}

func (l *Locks) Release(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.held, name)
}

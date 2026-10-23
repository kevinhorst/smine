package repos

import "sync"

// Locks rejects a concurrent mutation of one repo instead of queueing it.
type Locks struct {
	// mu guards held.
	mu   sync.Mutex
	held map[string]bool
}

func NewLocks() *Locks {
	return &Locks{held: make(map[string]bool)}
}

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

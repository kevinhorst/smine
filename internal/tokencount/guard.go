package tokencount

import (
	"errors"
	"net/http"
	"sync"
	"time"
)

const (
	breakerTrips      = 3
	maxItemRunes      = 200_000
	MaxItemsPerReport = 200
	RatePerMinute     = 30
)

// guard's breaker stays open for the process once it trips.
type guard struct {
	mu sync.Mutex

	failures int
	isOpen   bool
	last     time.Time
	now      func() time.Time
	tokens   float64
}

func newGuard(now func() time.Time) *guard {
	g := &guard{last: now(), now: now, tokens: RatePerMinute}
	return g
}

// isTripped reports an open breaker.
func (g *guard) isTripped() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.isOpen
}

func (g *guard) recordFailure(err error) bool {
	var apiErr *apiError
	if !errors.As(err, &apiErr) || !tripsBreaker(apiErr.Status) {
		return false
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	g.failures++
	if g.failures >= breakerTrips {
		g.isOpen = true
	}
	return g.isOpen
}

func (g *guard) recordSuccess() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.failures = 0
}

// take spends one call from the bucket; false when the minute is used up.
func (g *guard) take() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	g.tokens = min(RatePerMinute, g.tokens+now.Sub(g.last).Minutes()*RatePerMinute)
	g.last = now
	if g.tokens < 1 {
		return false
	}
	g.tokens--
	return true
}

func tripsBreaker(status int) bool {
	isAuth := status == http.StatusUnauthorized || status == http.StatusForbidden
	return isAuth || status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

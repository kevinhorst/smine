package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/kevinhorst/smine/internal/contextrecord"
)

const (
	// backgroundCountTimeout keeps a hung API from marking a session as counting forever.
	backgroundCountTimeout = 2 * time.Minute
	// countedParam marks the page's own reload, which starts no further run so the page never loops.
	countedParam = "counted"
	// countingNoticeFormat's numbers are the guard's.
	countingNoticeFormat = "Counting tokens in the background (pending: %d) — %d API calls per minute, %d injections per run. This page reloads once when the run finishes."
)

// countRuns allows one background count run per session at a time.
type countRuns struct {
	mu      sync.Mutex
	running map[string]bool
}

func newCountRuns() *countRuns {
	runs := &countRuns{running: map[string]bool{}}
	return runs
}

// begin claims the session's run; false when one is already in flight.
func (c *countRuns) begin(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running[id] {
		return false
	}
	c.running[id] = true
	return true
}

func (c *countRuns) end(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.running, id)
}

func (c *countRuns) isRunning(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running[id]
}

// countingStatus is the body of the counting-status endpoint.
type countingStatus struct {
	Counting bool `json:"counting"`
}

// handleSessionReportCounting is polled by the report page, which reloads once the run finishes.
func (s *Server) handleSessionReportCounting(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	status := countingStatus{Counting: s.countRuns.isRunning(id)}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		log.Printf("Server.handleSessionReportCounting: Session %s: Failed to write response: %v", id, err)
	}
}

func (s *Server) runCount(id string, lastActive time.Time, record *contextrecord.Record) {
	defer s.countRuns.end(id)

	ctx, cancel := context.WithTimeout(context.Background(), backgroundCountTimeout)
	defer cancel()

	s.tokens.Count(ctx, record)
	s.storeBreakdown(id, lastActive, record)
}

// startCounting works on a copy so the page renders the record it holds without waiting.
func (s *Server) startCounting(id string, lastActive time.Time, record *contextrecord.Record) {
	if !s.countRuns.begin(id) {
		return
	}

	go s.runCount(id, lastActive, countCopy(record))
}

// countCopy deep-copies only what a count run mutates: the agents and their items.
func countCopy(record *contextrecord.Record) *contextrecord.Record {
	clone := *record
	clone.Agents = slices.Clone(record.Agents)
	for index := range clone.Agents {
		clone.Agents[index].Items = slices.Clone(record.Agents[index].Items)
	}
	return &clone
}

// recordPending counts the record's pending items over every agent.
func recordPending(record *contextrecord.Record) int {
	pending := 0
	for agentIndex := range record.Agents {
		pending += pendingCount(record.Agents[agentIndex].Items)
	}
	return pending
}

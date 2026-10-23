package budget

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kevinhorst/smine/internal/contextrecord"
	"github.com/kevinhorst/smine/internal/peek"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/tokencount"
)

// Capture is one run of the budget job over the sessions peek serves.
type Capture struct {
	Archive    *Archive
	Ccusage    *Ccusage
	ClaudeHome string
	ContextDir string
	Counter    *tokencount.Counter
	IsCounting bool
	Peek       *peek.Client
	Roster     []repos.Repo
	Store      *Store
}

// Folds whenever the breakdown file is newer than the summary, whoever wrote it.
func (c *Capture) foldBreakdown(record *Record) (bool, error) {
	modTime := c.Store.BreakdownModTime(record.Id)
	if modTime.IsZero() {
		return false, nil
	}
	if record.Context != nil && !modTime.After(record.Context.Source) {
		return false, nil
	}

	breakdown, err := c.Store.Breakdown(record.Id)
	if err != nil {
		return false, fmt.Errorf("Capture.foldBreakdown: %w", err)
	}
	if breakdown == nil || breakdown.Record == nil {
		return false, nil
	}
	record.Context = breakdown.Summary(modTime)
	return true, nil
}

func (c *Capture) mirror(state *State) {
	archived, err := c.Archive.Mirror()
	state.Archived = archived
	if err != nil {
		state.ArchiveError = err.Error()
	}
}

// None yet, older than the session, or left uncounted by a limit while counting is on.
func (c *Capture) needsBreakdown(breakdown *Breakdown, session *peek.Session) bool {
	if breakdown == nil {
		return true
	}
	if !breakdown.IsCurrent(session.LastActive) {
		return true
	}
	return c.IsCounting && breakdown.HasRetryable()
}

// An extraction failure is stored as the breakdown's error.
func (c *Capture) saveBreakdown(ctx context.Context, conn *peek.Conn, events *peek.SessionEvents, session *peek.Session) error {
	var turns []peek.Turn
	if session.Agent == contextrecord.SessionCodex {
		served, err := conn.SessionTurns(ctx, session.Agent, session.Id)
		if err != nil {
			return fmt.Errorf("Capture.saveBreakdown: Session %s: %w", session.Id, err)
		}
		turns = served
	}

	breakdown := &Breakdown{Id: session.Id, ComputedAt: time.Now(), LastActivity: session.LastActive, Schema: RecordSchema}
	record, err := ExtractRecord(c.ClaudeHome, c.ContextDir, events, session, turns)
	if err != nil {
		breakdown.Error = err.Error()
	} else {
		c.Counter.Count(ctx, record)
		breakdown = NewBreakdown(breakdown.ComputedAt, session.Id, session.LastActive, record)
	}
	if err := c.Store.SaveBreakdown(breakdown); err != nil {
		return fmt.Errorf("Capture.saveBreakdown: %w", err)
	}
	return nil
}

// Runs from the day before the earliest session no run has covered.
func (c *Capture) takeCcusage(ctx context.Context, state *State) {
	records, err := c.Store.Records()
	if err != nil {
		state.CcusageError = err.Error()
		return
	}
	since, isNeeded := ccusageSince(records)
	if !isNeeded {
		return
	}
	if err := c.Ccusage.CheckVersion(ctx); err != nil {
		state.CcusageError = err.Error()
		return
	}

	checkedAt := time.Now()
	claude, err := c.Ccusage.ClaudeSessions(ctx, since)
	if err != nil {
		state.CcusageError = err.Error()
		return
	}
	codex, err := c.Ccusage.CodexSessions(ctx, since)
	if err != nil {
		state.CcusageError = err.Error()
		return
	}

	byId := make(map[string]*Record, len(records))
	for _, record := range records {
		byId[record.Id] = record
	}
	applyCcusage(contextrecord.SessionClaude, byId, claude)
	applyCcusage(contextrecord.SessionCodex, byId, codex)
	for _, record := range byId {
		if !record.needsCcusage() {
			continue
		}

		record.CcusageChecked = checkedAt
		if err := c.Store.SaveRecord(record); err != nil {
			state.CcusageError = err.Error()
			return
		}
	}
}

func (c *Capture) takePeek(ctx context.Context, state *State) {
	conn, err := c.Peek.Connect(ctx)
	if err != nil {
		state.PeekError = err.Error()
		return
	}
	defer conn.Close()

	sessions, err := conn.Sessions(ctx, peek.ReportAgents)
	if err != nil {
		state.PeekError = err.Error()
		return
	}
	for index := range sessions {
		if sessions[index].LastActive.IsZero() {
			continue
		}

		if err := c.takeSession(ctx, conn, &sessions[index], state); err != nil {
			log.Printf("Capture.takePeek: %v", err)
			state.PeekError = err.Error()
		}
	}
}

func (c *Capture) takeSession(ctx context.Context, conn *peek.Conn, session *peek.Session, state *State) error {
	record, err := c.Store.Record(session.Id)
	if err != nil {
		return fmt.Errorf("Capture.takeSession: %w", err)
	}
	if record == nil {
		record = &Record{Id: session.Id, Agent: session.Agent, Schema: RecordSchema}
	}
	breakdown, err := c.Store.Breakdown(session.Id)
	if err != nil {
		return fmt.Errorf("Capture.takeSession: %w", err)
	}

	isPeekTaken := record.needsPeek(session)
	isBreakdownTaken := c.needsBreakdown(breakdown, session)
	if isPeekTaken || isBreakdownTaken {
		events, err := conn.SessionEvents(ctx, session.Id)
		if err != nil {
			return fmt.Errorf("Capture.takeSession: Session %s: %w", session.Id, err)
		}
		if isPeekTaken {
			record.applyPeek(events, repos.OwnerOf(session.Cwd, c.Roster), session)
			state.Taken++
		}
		if isBreakdownTaken {
			if err := c.saveBreakdown(ctx, conn, events, session); err != nil {
				state.ContextError = err.Error()
			}
		}
	}

	isFolded, err := c.foldBreakdown(record)
	if err != nil {
		return err
	}
	if !isPeekTaken && !isFolded {
		return nil
	}

	record.CapturedAt = time.Now()
	if err := c.Store.SaveRecord(record); err != nil {
		return fmt.Errorf("Capture.takeSession: %w", err)
	}
	return nil
}

// Run executes one capture and stores its trace; a nil Peek or Ccusage client means that component is off and its side is left alone.
func (c *Capture) Run(ctx context.Context) *State {
	state := &State{StartedAt: time.Now()}
	c.mirror(state)
	if c.Peek != nil {
		c.takePeek(ctx, state)
	}
	if c.Ccusage != nil {
		c.takeCcusage(ctx, state)
	}
	state.FinishedAt = time.Now()
	if err := c.Store.SaveState(state); err != nil {
		log.Printf("Capture.Run: %v", err)
	}
	return state
}

// Creates the record of a session only ccusage knows.
func applyCcusage(agent string, byId map[string]*Record, sides map[string]*CcusageSide) {
	for id, side := range sides {
		record, ok := byId[id]
		if !ok {
			record = &Record{Id: id, Agent: agent, Schema: RecordSchema}
			byId[id] = record
		}
		record.Ccusage = side
	}
}

// Zero with isNeeded true means everything: the first run.
func ccusageSince(records []*Record) (time.Time, bool) {
	if len(records) == 0 {
		return time.Time{}, true
	}

	var earliest time.Time
	isNeeded := false
	for _, record := range records {
		if !record.needsCcusage() {
			continue
		}

		isNeeded = true
		earliest = earlier(earliest, record.LastActivity())
	}
	if !isNeeded {
		return time.Time{}, false
	}
	return earliest.Local().AddDate(0, 0, -1), true
}

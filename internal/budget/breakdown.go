package budget

import (
	"time"

	"github.com/kevinhorst/smine/internal/contextrecord"
)

// Kept so no page re-reads the transcript; Error is set and Record nil when extraction failed.
type Breakdown struct {
	Id string `json:"id"`

	ComputedAt   time.Time             `json:"computed_at"`
	Error        string                `json:"error,omitempty"`
	LastActivity time.Time             `json:"last_activity"`
	Record       *contextrecord.Record `json:"record,omitempty"`
	Schema       int                   `json:"schema"`
}

func NewBreakdown(computedAt time.Time, id string, lastActivity time.Time, record *contextrecord.Record) *Breakdown {
	record.KeepDetails()
	breakdown := &Breakdown{
		Id:           id,
		ComputedAt:   computedAt,
		LastActivity: lastActivity,
		Record:       record,
		Schema:       RecordSchema,
	}
	return breakdown
}

// An item a rate, cap or queue limit left uncounted can still be filled by a later count.
func (b *Breakdown) HasRetryable() bool {
	if b.Record == nil {
		return false
	}
	for index := range b.Record.Agents {
		if hasRetryableItem(b.Record.Agents[index].Items) {
			return true
		}
	}
	return false
}

func (b *Breakdown) IsCurrent(lastActivity time.Time) bool {
	return !b.LastActivity.Before(lastActivity)
}

func (b *Breakdown) Summary(source time.Time) *ContextSummary {
	summary := &ContextSummary{Source: source, Totals: b.Record.Totals}
	for index := range b.Record.Agents {
		agent := &b.Record.Agents[index]
		row := ContextAgent{
			Id:          agent.Id,
			Description: agent.Description,
			Kind:        agent.Kind,
			Model:       agent.Model,
			Totals:      agent.Totals,
			Type:        agent.Type,
		}
		summary.Agents = append(summary.Agents, row)
	}

	byKind := b.Record.ByKind()
	for _, kind := range contextrecord.Kinds {
		totals, ok := byKind[kind]
		if !ok {
			continue
		}

		kindRow := ContextKind{Kind: kind, Totals: totals}
		summary.Kinds = append(summary.Kinds, kindRow)
	}
	return summary
}

func hasRetryableItem(items []contextrecord.Item) bool {
	for index := range items {
		switch items[index].Uncounted {
		case contextrecord.UncountedCap, contextrecord.UncountedQueued, contextrecord.UncountedRate:
			return true
		}
	}
	return false
}

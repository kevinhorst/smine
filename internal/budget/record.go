// Package budget keeps one usage record per agent session from peek, ccusage and its injected context.
package budget

import (
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/contextrecord"
	"github.com/kevinhorst/smine/internal/peek"
)

// RecordSchema is the record and breakdown layout this build reads and writes.
const RecordSchema = 1

// Sides captured before the model split was stored lack Models.
type CcusageSide struct {
	CostUsd       float64      `json:"cost_usd"`
	FirstActivity time.Time    `json:"first_activity"`
	IsUnpriced    bool         `json:"is_unpriced,omitempty"`
	LastActivity  time.Time    `json:"last_activity"`
	Models        []ModelUsage `json:"models,omitempty"`
	Usage         TokenClasses `json:"usage"`
	WorkflowRows  int          `json:"workflow_rows,omitempty"`
}

// False without a model split or with a model whose ratios are unknown.
func (c *CcusageSide) PlanUsage() (PlanUsage, bool) {
	var usage PlanUsage
	hasTokens := false
	for index := range c.Models {
		model := &c.Models[index]
		if model.Usage.Total() == 0 {
			continue
		}

		weights, ok := weightsFor(model.Model)
		if !ok {
			var none PlanUsage
			return none, false
		}

		hasTokens = true
		equivalents := weights.equivalents(model.Usage)
		usage.All += equivalents
		if strings.HasPrefix(model.Model, fableModelPrefix) {
			usage.Fable += equivalents
		}
	}
	return usage, hasTokens
}

// heaviestWeights are the ratios of the model that drew the most tokens; false without a known model.
func (c *CcusageSide) heaviestWeights() (classWeights, bool) {
	var heaviest *ModelUsage
	for index := range c.Models {
		model := &c.Models[index]
		if heaviest == nil || model.Usage.Total() > heaviest.Usage.Total() {
			heaviest = model
		}
	}
	if heaviest == nil {
		var none classWeights
		return none, false
	}
	return weightsFor(heaviest.Model)
}

// 0 when ccusage names no first activity (Codex).
func (c *CcusageSide) WallSeconds() int {
	if c.FirstActivity.IsZero() {
		return 0
	}
	return int(c.LastActivity.Sub(c.FirstActivity).Seconds())
}

// ContextAgent is one agent row of the stored breakdown.
type ContextAgent struct {
	Id string `json:"agent_id"`

	Description string                  `json:"description,omitempty"`
	Kind        contextrecord.AgentKind `json:"kind"`
	Model       string                  `json:"model,omitempty"`
	Totals      contextrecord.Totals    `json:"totals"`
	Type        string                  `json:"type,omitempty"`
}

type ContextKind struct {
	Kind   contextrecord.Kind   `json:"kind"`
	Totals contextrecord.Totals `json:"totals"`
}

// Source is the breakdown file's modification time when it was folded in.
type ContextSummary struct {
	Agents []ContextAgent       `json:"agents"`
	Kinds  []ContextKind        `json:"kinds"`
	Source time.Time            `json:"source"`
	Totals contextrecord.Totals `json:"totals"`
}

// ModelUsage is one model's tokens within a session.
type ModelUsage struct {
	Model string       `json:"model"`
	Usage TokenClasses `json:"usage"`
}

// Root never includes the subagents summed in Subagents.
type PeekSide struct {
	ActiveSeconds int          `json:"active_seconds"`
	CostUsd       float64      `json:"cost_usd"`
	HasCost       bool         `json:"has_cost"`
	IdleSeconds   int          `json:"idle_seconds"`
	LastActive    time.Time    `json:"last_active"`
	Root          TokenClasses `json:"root"`
	SkillRuns     []SkillRun   `json:"skill_runs,omitempty"`
	StartedAt     time.Time    `json:"started_at"`
	Subagents     TokenClasses `json:"subagents"`
	WallSeconds   int          `json:"wall_seconds"`
}

// Negative seconds, which peek returns for some Codex sessions, are stored as 0.
func newPeekSide(events *peek.SessionEvents) *PeekSide {
	side := &PeekSide{
		ActiveSeconds: max(events.ActiveSeconds, 0),
		CostUsd:       events.CostUsd,
		HasCost:       events.HasTelemetry,
		IdleSeconds:   max(events.IdleSeconds, 0),
		LastActive:    events.LastActive,
		Root:          newTokenClasses(events.Usage),
		StartedAt:     events.StartedAt,
		WallSeconds:   max(events.WallSeconds, 0),
	}
	for index := range events.SkillRuns {
		run := &events.SkillRuns[index]
		skillRun := SkillRun{Seconds: run.Seconds, Skill: run.Skill, Usage: newTokenClasses(run.Usage)}
		side.SkillRuns = append(side.SkillRuns, skillRun)
	}
	for index := range events.Subagents {
		side.Subagents = side.Subagents.Add(newTokenClasses(events.Subagents[index].Usage))
	}
	return side
}

// Never below 0: a run's own end is peek's call, so the runs may not add up.
func (p *PeekSide) OutsideSkills() TokenClasses {
	var inSkills TokenClasses
	for index := range p.SkillRuns {
		inSkills = inSkills.Add(p.SkillRuns[index].Usage)
	}
	return p.Root.Minus(inSkills)
}

// SkillUsage sums one skill's runs: root-agent tokens and wall seconds.
func (p *PeekSide) SkillUsage(skill string) (TokenClasses, int) {
	var usage TokenClasses
	seconds := 0
	for index := range p.SkillRuns {
		run := &p.SkillRuns[index]
		if run.Skill != skill {
			continue
		}

		usage = usage.Add(run.Usage)
		seconds += run.Seconds
	}
	return usage, seconds
}

// Total is the whole session as ccusage counts it: root plus subagents.
func (p *PeekSide) Total() TokenClasses {
	return p.Root.Add(p.Subagents)
}

// A nil side was never obtained; CcusageChecked is the last ccusage run covering the session.
type Record struct {
	Id string `json:"id"`

	Agent          string          `json:"agent"`
	CapturedAt     time.Time       `json:"captured_at"`
	Ccusage        *CcusageSide    `json:"ccusage,omitempty"`
	CcusageChecked time.Time       `json:"ccusage_checked"`
	Context        *ContextSummary `json:"context,omitempty"`
	Cwd            string          `json:"cwd,omitempty"`
	Peek           *PeekSide       `json:"peek,omitempty"`
	Repo           string          `json:"repo,omitempty"`
	Schema         int             `json:"schema"`
	Title          string          `json:"title,omitempty"`
}

// applyPeek writes the peek side and the session identity peek knows.
func (r *Record) applyPeek(events *peek.SessionEvents, repo string, session *peek.Session) {
	r.Cwd = session.Cwd
	r.Peek = newPeekSide(events)
	r.Peek.LastActive = session.LastActive
	r.Repo = repo
	r.Title = session.Title
}

func (r *Record) needsCcusage() bool {
	if r.CcusageChecked.IsZero() {
		return true
	}
	return r.Peek != nil && r.Peek.LastActive.After(r.CcusageChecked)
}

func (r *Record) needsPeek(session *peek.Session) bool {
	if r.Peek == nil {
		return true
	}
	return session.LastActive.After(r.Peek.LastActive)
}

// Share is the part of the peek session usage takes, weighed by the heaviest ccusage model's price ratios, else by token count.
func (r *Record) Share(usage TokenClasses) float64 {
	if r.Peek == nil {
		return 0
	}

	weights := countWeights
	if r.Ccusage != nil {
		if priced, ok := r.Ccusage.heaviestWeights(); ok {
			weights = priced
		}
	}
	whole := weights.equivalents(r.Peek.Total())
	if whole == 0 {
		return 0
	}
	return weights.equivalents(usage) / whole
}

func (r *Record) LastActivity() time.Time {
	var last time.Time
	if r.Peek != nil {
		last = r.Peek.LastActive
	}
	if r.Ccusage != nil && r.Ccusage.LastActivity.After(last) {
		last = r.Ccusage.LastActivity
	}
	return last
}

// SkillRun is one skill run's root-agent tokens and seconds.
type SkillRun struct {
	Seconds int          `json:"seconds"`
	Skill   string       `json:"skill"`
	Usage   TokenClasses `json:"usage"`
}

// TokenClasses is one token count per billing class.
type TokenClasses struct {
	CacheCreation int `json:"cache_creation"`
	CacheRead     int `json:"cache_read"`
	Input         int `json:"input"`
	Output        int `json:"output"`
}

func newTokenClasses(usage peek.TokenUsage) TokenClasses {
	classes := TokenClasses{
		CacheCreation: usage.CacheCreationInputTokens,
		CacheRead:     usage.CacheReadInputTokens,
		Input:         usage.InputTokens,
		Output:        usage.OutputTokens,
	}
	return classes
}

func (t TokenClasses) Add(other TokenClasses) TokenClasses {
	sum := TokenClasses{
		CacheCreation: t.CacheCreation + other.CacheCreation,
		CacheRead:     t.CacheRead + other.CacheRead,
		Input:         t.Input + other.Input,
		Output:        t.Output + other.Output,
	}
	return sum
}

// Minus subtracts per class and never goes below 0.
func (t TokenClasses) Minus(other TokenClasses) TokenClasses {
	rest := TokenClasses{
		CacheCreation: max(t.CacheCreation-other.CacheCreation, 0),
		CacheRead:     max(t.CacheRead-other.CacheRead, 0),
		Input:         max(t.Input-other.Input, 0),
		Output:        max(t.Output-other.Output, 0),
	}
	return rest
}

func (t TokenClasses) Total() int {
	return t.CacheCreation + t.CacheRead + t.Input + t.Output
}

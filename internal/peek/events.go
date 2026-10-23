package peek

import (
	"context"
	"time"
)

const decisionReject = "reject"

// CostUsd needs HasTelemetry; PermissionRejections counts hook and user rejects and needs HasPermissions.
type SessionEvents struct {
	ActiveSeconds        int
	CostUsd              float64
	HasPermissions       bool
	HasTelemetry         bool
	IdleSeconds          int
	InvokedSkills        []SkillInvocation
	LastActive           time.Time
	PermissionRejections int
	SkillRuns            []SkillRun
	StartedAt            time.Time
	Subagents            []SubagentUsage
	TouchedFiles         []TouchedFile
	Usage                TokenUsage
	WallSeconds          int
}

// SessionEvents opens one connection for one session_events call.
func (c *Client) SessionEvents(ctx context.Context, id string) (*SessionEvents, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	return conn.SessionEvents(ctx, id)
}

type SkillInvocation struct {
	Skill     string
	Source    string
	Timestamp time.Time
}

// Where a run ends is peek's call.
type SkillRun struct {
	EndedAt   time.Time
	Seconds   int
	Skill     string
	StartedAt time.Time
	Usage     TokenUsage
}

// SubagentUsage is one subagent's tokens; session Usage never includes them.
type SubagentUsage struct {
	Id string

	Model   string
	Seconds int
	Type    string
	Usage   TokenUsage
}

type TokenUsage struct {
	CacheCreationInputTokens int
	CacheReadInputTokens     int
	InputTokens              int
	OutputTokens             int
}

type TouchedFile struct {
	Path   string
	Reads  int
	Writes int
}

// Only the skill_invoked kind carries a Skill payload the report uses.
type eventsItem struct {
	Kind      string      `json:"kind"`
	Skill     eventsSkill `json:"skill"`
	Timestamp time.Time   `json:"timestamp"`
}

type eventsPermissionRequest struct {
	Decision string `json:"decision"`
}

// peek omits it when it holds no decision for the session.
type eventsPermissions struct {
	Requests []eventsPermissionRequest `json:"requests"`
}

func (p *eventsPermissions) rejections() int {
	count := 0
	for _, request := range p.Requests {
		if request.Decision == decisionReject {
			count++
		}
	}
	return count
}

type eventsResult struct {
	Events       []eventsItem        `json:"events"`
	Permissions  *eventsPermissions  `json:"permissions"`
	Skills       []eventsSkillRun    `json:"skills"`
	Subagents    []eventsSubagent    `json:"subagents"`
	Time         eventsTime          `json:"time"`
	TouchedFiles []eventsTouchedFile `json:"touched_files"`
	Usage        eventsUsage         `json:"usage"`
}

type eventsSkill struct {
	Skill  string `json:"skill"`
	Source string `json:"source"`
}

type eventsSkillRun struct {
	EndedAt   time.Time   `json:"ended_at"`
	Seconds   int         `json:"seconds"`
	Skill     string      `json:"skill"`
	StartedAt time.Time   `json:"started_at"`
	Usage     eventsUsage `json:"usage"`
}

type eventsSubagent struct {
	AgentId   string      `json:"agent_id"`
	AgentType string      `json:"agent_type"`
	Model     string      `json:"model"`
	Seconds   int         `json:"seconds"`
	Usage     eventsUsage `json:"usage"`
}

type eventsTelemetry struct {
	CostUsd float64 `json:"cost_usd"`
}

type eventsTime struct {
	ActiveSeconds int              `json:"active_seconds"`
	IdleSeconds   int              `json:"idle_seconds"`
	LastActive    time.Time        `json:"last_active"`
	StartedAt     time.Time        `json:"started_at"`
	Telemetry     *eventsTelemetry `json:"telemetry"`
	WallSeconds   int              `json:"wall_seconds"`
}

type eventsTouchedFile struct {
	Path   string `json:"path"`
	Reads  int    `json:"reads"`
	Writes int    `json:"writes"`
}

// Codex's input includes its cached tokens; Claude reports four classes.
type eventsUsage struct {
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CachedInputTokens        int `json:"cached_input_tokens"`
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
}

// Codex's cached input counts as cache read and leaves the input class.
func (u *eventsUsage) tokens() TokenUsage {
	usage := TokenUsage{
		CacheCreationInputTokens: u.CacheCreationInputTokens,
		CacheReadInputTokens:     u.CacheReadInputTokens + u.CachedInputTokens,
		InputTokens:              u.InputTokens - u.CachedInputTokens,
		OutputTokens:             u.OutputTokens,
	}
	return usage
}

// Event kinds other than skill_invoked are dropped here, not modelled.
func newSessionEvents(parsed *eventsResult) *SessionEvents {
	events := &SessionEvents{
		ActiveSeconds: parsed.Time.ActiveSeconds,
		HasTelemetry:  parsed.Time.Telemetry != nil,
		IdleSeconds:   parsed.Time.IdleSeconds,
		LastActive:    parsed.Time.LastActive,
		StartedAt:     parsed.Time.StartedAt,
		Usage:         parsed.Usage.tokens(),
		WallSeconds:   parsed.Time.WallSeconds,
	}
	if parsed.Time.Telemetry != nil {
		events.CostUsd = parsed.Time.Telemetry.CostUsd
	}
	if parsed.Permissions != nil {
		events.HasPermissions = true
		events.PermissionRejections = parsed.Permissions.rejections()
	}
	for _, item := range parsed.Events {
		if item.Kind != "skill_invoked" {
			continue
		}

		invocation := SkillInvocation{
			Skill:     item.Skill.Skill,
			Source:    item.Skill.Source,
			Timestamp: item.Timestamp,
		}
		events.InvokedSkills = append(events.InvokedSkills, invocation)
	}
	for _, file := range parsed.TouchedFiles {
		events.TouchedFiles = append(events.TouchedFiles, TouchedFile(file))
	}
	for index := range parsed.Skills {
		run := &parsed.Skills[index]
		skillRun := SkillRun{
			EndedAt:   run.EndedAt,
			Seconds:   run.Seconds,
			Skill:     run.Skill,
			StartedAt: run.StartedAt,
			Usage:     run.Usage.tokens(),
		}
		events.SkillRuns = append(events.SkillRuns, skillRun)
	}
	for index := range parsed.Subagents {
		subagent := &parsed.Subagents[index]
		usage := SubagentUsage{
			Id:      subagent.AgentId,
			Model:   subagent.Model,
			Seconds: subagent.Seconds,
			Type:    subagent.AgentType,
			Usage:   subagent.Usage.tokens(),
		}
		events.Subagents = append(events.Subagents, usage)
	}
	return events
}

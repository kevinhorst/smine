// Package contextrecord measures a session's injected context per agent, byte-compatible with context_record.sh's id fields.
package contextrecord

import (
	"slices"
	"strings"
	"unicode/utf8"
)

// AgentKind tells the root transcript from a subagent's and a Workflow agent's.
type AgentKind string

// Counter names the mechanism that produced an item's token count.
type Counter string

// Kind names one injection mechanism — the closed set the report groups by.
type Kind string

const (
	AgentRoot     AgentKind = "root"
	AgentSubagent AgentKind = "subagent"
	AgentWorkflow AgentKind = "workflow"

	CounterAnthropic Counter = "anthropic"
	CounterO200k     Counter = "o200k_base"

	KindAcdslContext      Kind = "acdsl_context"
	KindAcdslProjection   Kind = "acdsl_projection"
	KindAgentsMd          Kind = "agents_md"
	KindCodexEnvironment  Kind = "codex_environment"
	KindCodexInstructions Kind = "codex_instructions"
	KindGlobal            Kind = "global"
	KindHookContext       Kind = "hook_context"
	KindLang              Kind = "lang"
	KindPackRead          Kind = "pack_read"
	KindPlanContext       Kind = "plan_context"
	KindPlanRules         Kind = "plan_rules"
	KindSkill             Kind = "skill"
	KindSubagentPrompt    Kind = "subagent_prompt"

	// SessionClaude and SessionCodex are the Record.Agent values.
	SessionClaude = "claude"
	SessionCodex  = "codex"

	// Sources name where in the transcript an item's text was read.
	SourceHookContext = "hook_additional_context"
	SourceHookSuccess = "hook_success"
	SourceToolResult  = "tool_result"
	SourceToolUse     = "tool_use"
	SourceTurn        = "turn"

	// Uncounted reasons an item carries instead of a token count.
	UncountedCap      = "cap"
	UncountedDisabled = "disabled"
	UncountedQueued   = "queued"
	UncountedRate     = "rate"
	UncountedSize     = "size"
)

const DetailMaxRunes = 4000

// Kinds is every kind in report order.
var Kinds = []Kind{
	KindGlobal,
	KindSkill,
	KindLang,
	KindAcdslProjection,
	KindAcdslContext,
	KindPlanRules,
	KindPlanContext,
	KindPackRead,
	KindAgentsMd,
	KindHookContext,
	KindSubagentPrompt,
	KindCodexInstructions,
	KindCodexEnvironment,
}

// Agent is one transcript's injections: the root or one subagent.
type Agent struct {
	Id string `json:"agent_id"`

	Context     *AgentContext `json:"context,omitempty"`
	Description string        `json:"description,omitempty"`
	Files       []FileTouch   `json:"files,omitempty"`
	Items       []Item        `json:"items"`
	Kind        AgentKind     `json:"kind"`
	Model       string        `json:"model,omitempty"`
	Path        string        `json:"path,omitempty"`
	SpawnDepth  int           `json:"spawn_depth,omitempty"`
	Totals      Totals        `json:"totals"`
	Type        string        `json:"type,omitempty"`
}

func (a *Agent) add(item Item) {
	a.Items = append(a.Items, item)
}

// recompute rebuilds the totals from the items.
func (a *Agent) recompute() {
	a.Totals = Totals{}
	for index := range a.Items {
		a.Totals.add(&a.Items[index])
	}
}

// Kept per agent so the report can scope every section to one agent.
type AgentContext struct {
	AcdslRules      []FileRules       `json:"acdsl_rules,omitempty"`
	Injected        InjectedContext   `json:"injected"`
	PackReads       []string          `json:"pack_reads,omitempty"`
	PlanRules       []string          `json:"plan_rules,omitempty"`
	SubagentContext []SubagentContext `json:"subagent_context,omitempty"`
}

type FileRules struct {
	File string   `json:"file"`
	Ids  []string `json:"ids"`
}

// FileTouch counts one agent's Read and write tool calls on one path.
type FileTouch struct {
	Path   string `json:"path"`
	Reads  int    `json:"reads,omitempty"`
	Writes int    `json:"writes,omitempty"`
}

// InjectedContext is the script-era id view of the hook injections.
type InjectedContext struct {
	Global []string       `json:"global,omitempty"`
	Lang   []LangContext  `json:"lang,omitempty"`
	Skill  []SkillContext `json:"skill,omitempty"`
}

// The text itself is kept for counting and never serialized.
type Item struct {
	text string

	Approx     bool    `json:"approx,omitempty"`
	Chars      int     `json:"chars"`
	Counter    Counter `json:"counter,omitempty"`
	Detail     string  `json:"detail,omitempty"`
	DetailRest int     `json:"detail_rest,omitempty"`
	EndLine    int     `json:"end_line,omitempty"`
	Error      string  `json:"error,omitempty"`
	Kind       Kind    `json:"kind"`
	Label      string  `json:"label"`
	Lines      int     `json:"lines"`
	Preview    string  `json:"preview,omitempty"`
	Range      string  `json:"range,omitempty"`
	Source     string  `json:"source"`
	StartLine  int     `json:"start_line,omitempty"`
	Tokens     int     `json:"tokens,omitempty"`
	Uncounted  string  `json:"uncounted,omitempty"`
}

// NewItem measures text: chars are runes, lines are newline-separated lines.
func NewItem(kind Kind, label, source, text string) Item {
	item := Item{
		Chars:  utf8.RuneCountInString(text),
		Kind:   kind,
		Label:  label,
		Lines:  countLines(text),
		Source: source,
		text:   text,
	}
	return item
}

// Falls back to the kept detail once the item was loaded from a store.
func (i *Item) Capped() (string, int) {
	if i.text == "" {
		return i.Detail, i.DetailRest
	}
	if utf8.RuneCountInString(i.text) <= DetailMaxRunes {
		return i.text, 0
	}

	runes := []rune(i.text)
	return string(runes[:DetailMaxRunes]), len(runes) - DetailMaxRunes
}

// SetTokens records a count and clears any earlier uncounted reason.
func (i *Item) SetTokens(counter Counter, tokens int, isApprox bool) {
	i.Approx = isApprox
	i.Counter = counter
	i.Tokens = tokens
	i.Uncounted = ""
}

// Text returns the measured text.
func (i *Item) Text() string {
	return i.text
}

type LangContext struct {
	Coverage   string   `json:"coverage,omitempty"`
	Guide      string   `json:"guide,omitempty"`
	Ids        []string `json:"ids"`
	Lang       string   `json:"lang"`
	TotalLines int      `json:"total_lines,omitempty"`
}

type Record struct {
	AcdslRules      []FileRules       `json:"acdsl_rules,omitempty"`
	Agent           string            `json:"agent"`
	Agents          []Agent           `json:"agents"`
	Cwd             string            `json:"cwd,omitempty"`
	GitBranch       string            `json:"git_branch,omitempty"`
	Injected        InjectedContext   `json:"injected"`
	PackReads       []string          `json:"pack_reads,omitempty"`
	PlanRules       []string          `json:"plan_rules,omitempty"`
	SessionId       string            `json:"session_id,omitempty"`
	SubagentContext []SubagentContext `json:"subagent_context,omitempty"`
	Totals          Totals            `json:"totals"`
}

// The population the report's filter bar counts over.
func (r *Record) AllIds() []string {
	var ids []string
	for _, skill := range r.Injected.Skill {
		ids = append(ids, skill.Ids...)
	}
	for _, lang := range r.Injected.Lang {
		ids = append(ids, lang.Ids...)
	}
	for _, file := range r.AcdslRules {
		ids = append(ids, file.Ids...)
	}
	ids = append(ids, r.PlanRules...)
	for _, subagent := range r.SubagentContext {
		ids = append(ids, subagent.Ids...)
	}
	return ids
}

// ByKind sums the items of every agent per kind.
func (r *Record) ByKind() map[Kind]Totals {
	byKind := make(map[Kind]Totals)
	for agentIndex := range r.Agents {
		for itemIndex := range r.Agents[agentIndex].Items {
			item := &r.Agents[agentIndex].Items[itemIndex]
			totals := byKind[item.Kind]
			totals.add(item)
			byKind[item.Kind] = totals
		}
	}
	return byKind
}

// The record then renders after a store round-trip without the transcript.
func (r *Record) KeepDetails() {
	for agentIndex := range r.Agents {
		items := r.Agents[agentIndex].Items
		for itemIndex := range items {
			items[itemIndex].Detail, items[itemIndex].DetailRest = items[itemIndex].Capped()
		}
	}
}

// Runs after extraction and again after counting.
func (r *Record) Recompute() {
	r.Totals = Totals{}
	for index := range r.Agents {
		r.Agents[index].recompute()
		for itemIndex := range r.Agents[index].Items {
			r.Totals.add(&r.Agents[index].Items[itemIndex])
		}
	}
}

type SkillContext struct {
	Ids     []string `json:"ids"`
	Skill   string   `json:"skill"`
	Version string   `json:"version,omitempty"`
	Via     string   `json:"via,omitempty"`
}

type SubagentContext struct {
	Ids   []string `json:"ids,omitempty"`
	Paths []string `json:"paths,omitempty"`
}

// Totals sums items; Uncounted counts items without a token count.
type Totals struct {
	Chars     int `json:"chars"`
	Items     int `json:"items"`
	Lines     int `json:"lines"`
	Tokens    int `json:"tokens"`
	Uncounted int `json:"uncounted"`
}

func (t *Totals) add(item *Item) {
	t.Chars += item.Chars
	t.Items++
	t.Lines += item.Lines
	t.Tokens += item.Tokens
	if item.Counter == "" {
		t.Uncounted++
	}
}

// A trailing newline closes the last line instead of opening an empty one.
func countLines(text string) int {
	if text == "" {
		return 0
	}
	return strings.Count(strings.TrimSuffix(text, "\n"), "\n") + 1
}

// jq's unique, never nil: an empty id list serializes as [] like the script's output.
func uniqueSorted(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	slices.Sort(values)
	return slices.Compact(values)
}

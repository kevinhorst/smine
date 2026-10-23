// Package sessiontitle holds the Desktop session title grammar, its state transitions and the skills' title.json declarations.
package sessiontitle

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// Built-in states; every other State value names a state a deployed skill declares.
const (
	StateNone    State = ""
	StateClosed  State = "CLOSED"
	StateDone    State = "DONE"
	StateReady   State = "READY"
	StateTesting State = "TESTING"
	StateWip     State = "WIP"
)

const ellipsis = "…"

// titlePattern matches the AI part greedily, so it may itself contain " - ".
var titlePattern = regexp.MustCompile(`^([^\s,]+)(?:, ?([^\s,]+))? - (.+) - ([A-Za-z0-9-]*)\[([A-Z_-]+)\]$`)

// Limits caps a rendered title and a skill's declaration; callers take them from the hooks config or the catalog defaults.
type Limits struct {
	MaxStates     int
	MaxTitleRunes int
	MaxTokenRunes int
}

type State string

func (s State) AfterClose() State {
	return StateClosed
}

// AfterDone is the state once `smine-heartbeat done` succeeded.
func (s State) AfterDone() State {
	return StateDone
}

func (s State) AfterJoin() State {
	if s == StateNone {
		return StateWip
	}
	return s
}

// AfterSet is the state a skill's `smine-rules title set` call puts into the title; the terminal states stand.
func (s State) AfterSet(target State) State {
	switch s {
	case StateTesting, StateDone, StateClosed:
		return s
	case StateNone, StateReady, StateWip:
		return target
	}
	return target
}

// IsBuiltin reports whether the state is one the hook sets by itself.
func (s State) IsBuiltin() bool {
	switch s {
	case StateNone, StateClosed, StateDone, StateReady, StateTesting, StateWip:
		return true
	}
	return false
}

type Title struct {
	Ai    string
	Fleet string
	Repo  string
	Skill string
	State State
}

// ParseTitle parses a legacy or undeclared token to StateNone, which the next transition replaces.
func ParseTitle(text string, vocabulary *Vocabulary) *Title {
	trimmed := strings.TrimSpace(text)
	match := titlePattern.FindStringSubmatch(trimmed)
	if match == nil {
		return &Title{Ai: trimmed}
	}

	title := &Title{
		Ai:    match[3],
		Fleet: match[4],
		Repo:  match[1],
		Skill: match[2],
		State: vocabulary.state(match[5]),
	}
	return title
}

// IsAwaitingAi relies on a slash-command echo starting with "/", which a generated title never does.
func (t *Title) IsAwaitingAi() bool {
	if t.Ai == "" {
		return true
	}
	isUnformatted := t.Repo == ""
	return isUnformatted && strings.HasPrefix(t.Ai, "/")
}

func (t *Title) Label() string {
	if t.Repo == "" {
		return t.Ai
	}

	label := t.prefix() + t.Ai
	if t.Fleet != "" {
		label += " - " + t.Fleet
	}
	return label
}

// prefix is "<Repo>[,<Skill>] - ".
func (t *Title) prefix() string {
	prefix := t.Repo
	if t.Skill != "" {
		prefix += "," + t.Skill
	}
	return prefix + " - "
}

// Render is the title text, the AI part shortened so the whole holds at most limits.MaxTitleRunes.
func (t *Title) Render(limits *Limits) string {
	prefix := t.prefix()
	suffix := " - " + t.Fleet + "[" + string(t.State) + "]"
	budget := limits.MaxTitleRunes - utf8.RuneCountInString(prefix) - utf8.RuneCountInString(suffix)
	return prefix + shortenAi(t.Ai, budget) + suffix
}

// shortenAi output re-parses to itself, so a shortened title stays stable.
func shortenAi(ai string, budget int) string {
	runes := []rune(ai)
	if len(runes) <= budget {
		return ai
	}

	cut := string(runes[:max(budget-1, 1)])
	if space := strings.LastIndex(cut, " "); space > 0 {
		cut = cut[:space]
	}
	return strings.TrimRight(cut, " -,") + ellipsis
}

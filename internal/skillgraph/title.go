package skillgraph

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/sessiontitle"
)

// titleSetRef is the interface a title state's set command runs through.
const titleSetRef = "title-set@1"

// catalogTitleLimits reads the title caps from the session-helpers catalog defaults, so the model check never depends on a machine's Claude home.
func catalogTitleLimits() (*sessiontitle.Limits, error) {
	maxStates, err := catalogIntDefault(components.SessionHelpers, "max_states")
	if err != nil {
		return nil, err
	}

	maxTitleRunes, err := catalogIntDefault(components.SessionHelpers, "max_title_runes")
	if err != nil {
		return nil, err
	}

	maxTokenRunes, err := catalogIntDefault(components.SessionHelpers, "max_token_runes")
	if err != nil {
		return nil, err
	}

	limits := &sessiontitle.Limits{MaxStates: maxStates, MaxTitleRunes: maxTitleRunes, MaxTokenRunes: maxTokenRunes}
	return limits, nil
}

func catalogIntDefault(id components.ID, key string) (int, error) {
	setting, isKnown := components.LookupSetting(id, key)
	if !isKnown {
		return 0, fmt.Errorf("catalogIntDefault: No catalog setting %s.%s", id, key)
	}

	value, err := components.ParseText(&setting, setting.Default)
	if err != nil {
		return 0, fmt.Errorf("catalogIntDefault: %w", err)
	}
	return value.Int, nil
}

// checkTitle binds each state to a gate step or stop whose text runs the state's set command through a title-set variable.
func checkTitle(skill *Skill) []string {
	if skill.Title == nil {
		return nil
	}

	limits, err := catalogTitleLimits()
	if err != nil {
		return []string{fmt.Sprintf("title: %v", err)}
	}

	violations := skill.Title.Violations(limits)
	if len(skill.Title.States) == 0 {
		violations = append(violations, "title: no state declared")
	}
	programs := titleSetRefs(skill)
	for _, state := range skill.Title.Tokens() {
		entryId := skill.Title.States[string(state)].Entry
		ref, isKnown := skill.Entry(entryId)
		if !isKnown {
			violations = append(violations, fmt.Sprintf("title: state %s: unknown entry %s", state, entryId))
			continue
		}

		isGateRole := ref.Entry.Role == RoleStep || ref.Entry.Role == RoleStop
		if !isGateRole {
			violations = append(violations, fmt.Sprintf("title: state %s: entry %s is a %s — a gate is presented by a step or a stop", state, entryId, ref.Entry.Role))
		}
		violations = append(violations, setCommandViolations(skill, ref.Entry, programs, state)...)
	}
	return violations
}

// renderTitle encodes the skill's title.json.
func renderTitle(skill *Skill) ([]byte, error) {
	document := &sessiontitle.Document{Declaration: skill.Title, Schema: sessiontitle.DocumentSchema, Skill: skill.Name(), Version: skill.Version()}
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("renderTitle: %w", err)
	}
	return append(encoded, '\n'), nil
}

// runsCommand reports whether the entry's statement names the command; below format 3 a param counts too.
func runsCommand(skill *Skill, entry *Entry, command string) bool {
	texts := append([]string{entry.Statement}, entry.Params()...)
	if entryFormat(skill) >= entryFormatFields {
		texts = []string{entry.Statement}
	}
	return slices.ContainsFunc(texts, func(text string) bool { return strings.Contains(text, command) })
}

// setCommandViolations requires the entry to run `<program> title set <state>` through one of the title-set references.
func setCommandViolations(skill *Skill, entry *Entry, programs []string, state sessiontitle.State) []string {
	if len(programs) == 0 {
		return []string{fmt.Sprintf("title: state %s: entry %s has no variable to run `title set %s` through — declare an env variable bound to %s", state, entry.Id, state, titleSetRef)}
	}

	commands := make([]string, 0, len(programs))
	for _, program := range programs {
		command := program + " title set " + string(state)
		if runsCommand(skill, entry, command) {
			return nil
		}
		commands = append(commands, "`"+command+"`")
	}
	return []string{fmt.Sprintf("title: state %s: entry %s never runs %s", state, entry.Id, strings.Join(commands, " or "))}
}

// titleSetIsSet reports whether one of the skill's title-set variables has a value.
func titleSetIsSet(skill *Skill, values map[string]string) bool {
	for _, ref := range titleSetRefs(skill) {
		if values[strings.TrimPrefix(ref, "$")] != "" {
			return true
		}
	}
	return false
}

// titleSetRefs returns the $-references of the skill's env keys bound to title-set@1, sorted.
func titleSetRefs(skill *Skill) []string {
	var refs []string
	keys := slices.Sorted(maps.Keys(skill.Env))
	for _, key := range keys {
		if skill.Env[key].Interface == titleSetRef {
			refs = append(refs, "$"+VariableName(skill.Name(), key))
		}
	}
	return refs
}

// Package sessionstate holds the gate's per-session record, the switch feature catalog and the hooks' reader.
package sessionstate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Feature is one switchable session effect, `<group>:<member>` or a bare word; the gate rejects every other word.
type Feature string

// Groups are the words before the colon; a bare group word names every member.
const (
	GroupInject  = "inject"
	GroupRgate   = "rgate"
	GroupSession = "session"
	GroupSkill   = "skill"
)

const (
	Heartbeat       Feature = "hb"
	InjectAcdsl     Feature = "inject:acdsl"
	InjectEnv       Feature = "inject:env"
	InjectGlobal    Feature = "inject:global"
	InjectPlan      Feature = "inject:plan"
	InjectSkill     Feature = "inject:skill"
	RgateGlobal     Feature = "rgate:global"
	RgateInvoked    Feature = "rgate:invoked"
	RgatePlan       Feature = "rgate:plan"
	SessionTitle    Feature = "session:title"
	SkillEnforce    Feature = "skill:enforce"
	SkillInvokegate Feature = "skill:invokegate"
)

// Words the parser accepts beside the catalog: every feature, and the long spelling of the heartbeat word.
const (
	All           = "all"
	globMeta      = "*?["
	groupSep      = ":"
	heartbeatLong = "heartbeat"
	hookReadgate  = "readgate (PreToolUse)"
	memberSep     = ","
)

// Entry describes one feature for the help the skillhelp hook renders; Does says what the feature does while on.
type Entry struct {
	Feature Feature
	Hook    string
	Does    string
}

// GuideEntry is the help line of the guide members, which the context index names, not this catalog.
var GuideEntry = Entry{
	Feature: Feature(GroupRgate + groupSep + "<ext|guide>"),
	Hook:    hookReadgate,
	Does:    "requires the language guides: by file extension every guide covering such files (go, rs, cue, md), by guide name that one guide (go_test, rust, skills)",
}

// catalog is the one list, in help order; Members, Names and Features derive from it.
var catalog = []Entry{
	{Feature: RgateGlobal, Hook: hookReadgate, Does: "requires the session's global context file to be read before the first gated call"},
	{Feature: RgateInvoked, Hook: hookReadgate, Does: "requires the context injected for each invoked skill to be read"},
	{Feature: RgatePlan, Hook: hookReadgate, Does: "requires the plan-mode context file to be read"},
	{Feature: SkillEnforce, Hook: "toolpolicy enforceCause (PreToolUse)", Does: "applies every active skill's enforce gate rules"},
	{Feature: SkillInvokegate, Hook: "skillinvokegate (PreToolUse)", Does: "makes a review lane invoke its required skill before any other tool call"},
	{Feature: InjectGlobal, Hook: "globalcontext (SessionStart, SubagentStart)", Does: "writes the global context file at session and subagent start"},
	{Feature: InjectSkill, Hook: "skillcontext (UserPromptSubmit, PostToolUse Skill)", Does: "injects a skill's declared context entries at its invocation"},
	{Feature: InjectAcdsl, Hook: "acdslhook (PostToolUse Read)", Does: "injects a governed file's rules as context after a Read"},
	{Feature: InjectEnv, Hook: "skillenv (SessionStart, CwdChanged, UserPromptExpansion, PreToolUse)", Does: "exports the session repo's skill values into the environment file the Bash tool sources and stops a skill whose required repo value is unset before it runs"},
	{Feature: InjectPlan, Hook: "planmodecontext", Does: "injects the plan-mode context"},
	{Feature: SessionTitle, Hook: "sessiontitle (PostToolUse, Stop)", Does: "names the session and moves its title state; the rename allow stays"},
	{Feature: Heartbeat, Hook: "heartbeat /hook", Does: "judges every hook beyond the heart attack: check denials, stop blocks, kills, runtime verdicts and the member guard"},
}

// Catalog is the feature list in help order.
func Catalog() []Entry {
	return slices.Clone(catalog)
}

// Help is the catalog in help order with the guide placeholder closing the rgate group.
func Help() []Entry {
	entries := make([]Entry, 0, len(catalog)+1)
	for index, entry := range catalog {
		entries = append(entries, entry)
		isLastRgate := entry.Feature.group() == GroupRgate && !isRgateAt(index+1)
		if isLastRgate {
			entries = append(entries, GuideEntry)
		}
	}
	return entries
}

func isRgateAt(index int) bool {
	if index >= len(catalog) {
		return false
	}
	return catalog[index].Feature.group() == GroupRgate
}

// group is the word before the colon, empty for a bare feature.
func (f Feature) group() string {
	group, _, hasMember := strings.Cut(string(f), groupSep)
	if !hasMember {
		return ""
	}
	return group
}

// Compact folds a sorted off set for a report line: a group whose every member is off reads as the group word.
func Compact(features []Feature, guides []string) []string {
	remaining := slices.Clone(features)
	words := make([]string, 0, len(features))
	for _, group := range groups() {
		members := Members(group, guides)
		isOff := func(member Feature) bool { return slices.Contains(remaining, member) }
		if !isEveryMember(members, isOff) {
			continue
		}

		words = append(words, group)
		remaining = slices.DeleteFunc(remaining, func(feature Feature) bool { return slices.Contains(members, feature) })
	}
	for _, feature := range remaining {
		words = append(words, string(feature))
	}
	return words
}

// Expand turns the parsed words into members: a group word and All expand, a guide member needs a known guide.
func Expand(features []Feature, guides []string) ([]Feature, string) {
	if len(features) == 0 {
		return nil, ""
	}

	expanded := make([]Feature, 0, len(features))
	for _, feature := range features {
		if string(feature) == All {
			expanded = append(expanded, Features(guides)...)
			continue
		}

		group, member, hasMember := strings.Cut(string(feature), groupSep)
		switch {
		case !hasMember && slices.Contains(groups(), group):
			expanded = append(expanded, Members(group, guides)...)
		case !hasMember:
			expanded = append(expanded, feature)
		case slices.Contains(Members(group, guides), feature):
			expanded = append(expanded, feature)
		default:
			return nil, member
		}
	}
	return expanded, ""
}

// Features are every member of every group plus the bare features, in help order with the guides after the rgate words.
func Features(guides []string) []Feature {
	features := make([]Feature, 0, len(catalog)+len(guides))
	for _, group := range groups() {
		features = append(features, Members(group, guides)...)
	}
	features = append(features, Heartbeat)
	return features
}

// Ext is the read-gate member of one file extension — every guide covering such files — or of one guide by its name.
func Ext(word string) Feature {
	return Feature(GroupRgate + groupSep + word)
}

// Extensions are the file extensions a guide's file globs name, sorted and unique; a glob without one adds nothing.
func Extensions(globs []string) []string {
	extensions := make([]string, 0, len(globs))
	for _, glob := range globs {
		_, extension, hasDot := strings.Cut(filepath.Base(glob), ".")
		isLiteral := hasDot && extension != "" && !strings.ContainsAny(extension, globMeta)
		if isLiteral {
			extensions = append(extensions, extension[strings.LastIndex(extension, ".")+1:])
		}
	}
	slices.Sort(extensions)
	return slices.Compact(extensions)
}

// GuideStem is a guide's name as a member word: its file name without the extension.
func GuideStem(guidePath string) string {
	return strings.TrimSuffix(filepath.Base(guidePath), ".md")
}

// GuideWords: a context index's guide member words, every covered extension and guide name, sorted unique; none without an index.
func GuideWords(indexPath string) []string {
	raw, err := os.ReadFile(indexPath)
	if err != nil {
		return nil
	}

	index := &contextIndex{}
	if err := json.Unmarshal(raw, index); err != nil {
		return nil
	}

	globs := make([]string, 0)
	words := make([]string, 0, len(index.Guides))
	for _, guide := range index.Guides {
		globs = append(globs, guide.Files...)
		words = append(words, GuideStem(guide.Path))
	}
	words = append(words, Extensions(globs)...)
	slices.Sort(words)
	return slices.Compact(words)
}

// Members are a group's features: the catalog's, plus one per guide for the read-gate group.
func Members(group string, guides []string) []Feature {
	members := make([]Feature, 0)
	for _, entry := range catalog {
		if strings.HasPrefix(string(entry.Feature), group+groupSep) {
			members = append(members, entry.Feature)
		}
	}
	if group == GroupRgate {
		for _, extension := range guides {
			members = append(members, Ext(extension))
		}
	}
	return members
}

// Names is the grammar line of a rejection: every group with its members, the guide placeholder and the bare words.
func Names() string {
	words := make([]string, 0)
	for _, group := range groups() {
		members := make([]string, 0)
		for _, member := range Members(group, nil) {
			_, name, _ := strings.Cut(string(member), groupSep)
			members = append(members, name)
		}
		if group == GroupRgate {
			members = append(members, "<ext|guide>")
		}
		words = append(words, fmt.Sprintf("%s[:%s]", group, strings.Join(members, "|")))
	}
	return strings.Join(append(words, string(Heartbeat), All), ", ")
}

// Parse answers the features one typed word names before expansion: All, a group, `group:a,b` or a bare word.
func Parse(word string) ([]Feature, bool) {
	switch word {
	case All:
		return []Feature{Feature(All)}, true
	case string(Heartbeat), heartbeatLong:
		return []Feature{Heartbeat}, true
	}

	group, members, hasMembers := strings.Cut(word, groupSep)
	if !slices.Contains(groups(), group) {
		return nil, false
	}
	if !hasMembers {
		return []Feature{Feature(group)}, true
	}

	features := make([]Feature, 0)
	for member := range strings.SplitSeq(members, memberSep) {
		if member == "" {
			return nil, false
		}
		features = append(features, Feature(group+groupSep+member))
	}
	return features, true
}

// contextIndex is the slice of the context index the guide words come from.
type contextIndex struct {
	Guides []contextGuide `json:"guides"`
}

type contextGuide struct {
	Files []string `json:"files"`
	Path  string   `json:"path"`
}

func groups() []string {
	return []string{GroupRgate, GroupSkill, GroupInject, GroupSession}
}

func isEveryMember(members []Feature, holds func(Feature) bool) bool {
	if len(members) == 0 {
		return false
	}
	for _, member := range members {
		if !holds(member) {
			return false
		}
	}
	return true
}

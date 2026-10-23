package skillgraph

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
)

// Source kinds of an atom declaration: the four closed kinds.
const (
	SourceArg         = "arg"
	SourceEnvironment = "environment"
	SourceRuntime     = "runtime"
	SourceSession     = "session"
)

// Source grammar tokens.
const (
	sourceArgPrefix     = "arg: "
	sourceMemberSep     = " => "
	sourcePartSep       = " | "
	sourceRuntimePrefix = "runtime: "
)

// runtimePattern is <anchor>[.<capture>[ (=|!=) <literal>]] after the runtime prefix.
var runtimePattern = regexp.MustCompile(`^([a-z][a-z0-9_]*)(?:\.([a-z][a-z0-9_]*))?(?: (=|!=) (\S+))?$`)

// SourcePart is one parsed part of an atom's source; Kind is one of the four kinds.
type SourcePart struct {
	Anchor  string
	Arg     string
	Capture string
	Kind    string
	Literal string
	Member  string
	Op      string
}

// checkSources: every local atom's source parses, an arg part names an Args row and a member of its enum, a runtime part names a declared anchor and capture and fits the atom's kind.
func checkSources(skill *Skill) []string {
	rows := map[string]bool{}
	for _, row := range skill.Args() {
		rows[strings.TrimLeft(row.Name, "-")] = true
	}
	var violations []string
	ids := slices.Sorted(maps.Keys(skill.Atoms))
	for _, id := range ids {
		atom := skill.Atoms[id]
		parts, err := ParseSource(atom.Source)
		if err != nil {
			violations = append(violations, fmt.Sprintf("atom %s: %v", id, err))
			continue
		}

		for index := range parts {
			violations = append(violations, sourcePartViolations(skill, id, atom, &parts[index], rows)...)
		}
	}
	return violations
}

// checkVocabularySources: a shared atom's source parses and is never runtime, which needs a skill's enforce block.
func checkVocabularySources(vocabulary *Vocabulary) []string {
	var violations []string
	ids := slices.Sorted(maps.Keys(vocabulary.Atoms))
	for _, id := range ids {
		parts, err := ParseSource(vocabulary.Atoms[id].Source)
		if err != nil {
			violations = append(violations, fmt.Sprintf("atom %s: %v", id, err))
			continue
		}

		for _, part := range parts {
			if part.Kind == SourceRuntime {
				violations = append(violations, fmt.Sprintf("atom %s: a shared atom takes no runtime source — the capture lives in one skill's enforce block", id))
			}
		}
	}
	return violations
}

func parseSourcePart(text string) (SourcePart, error) {
	switch {
	case text == SourceSession, text == SourceEnvironment:
		return SourcePart{Kind: text}, nil
	case strings.HasPrefix(text, sourceArgPrefix):
		name, member, _ := strings.Cut(strings.TrimPrefix(text, sourceArgPrefix), sourceMemberSep)
		part := SourcePart{Arg: strings.TrimLeft(strings.TrimSpace(name), "-"), Kind: SourceArg, Member: strings.TrimSpace(member)}
		return part, nil
	case strings.HasPrefix(text, sourceRuntimePrefix):
		match := runtimePattern.FindStringSubmatch(strings.TrimPrefix(text, sourceRuntimePrefix))
		if match == nil {
			return SourcePart{}, fmt.Errorf("parseSourcePart: Malformed runtime part %q — runtime: <anchor>[.<capture>[ = <literal> | != <literal>]]", text)
		}

		part := SourcePart{Anchor: match[1], Capture: match[2], Kind: SourceRuntime, Literal: match[4], Op: match[3]}
		return part, nil
	}
	return SourcePart{}, fmt.Errorf("parseSourcePart: Source part %q is none of arg: <row>[ => <member>], session, environment, runtime: <anchor>[.<capture>]", text)
}

func runtimeViolations(skill *Skill, id string, atom *Atom, part *SourcePart) []string {
	if skill.Enforce == nil {
		return []string{fmt.Sprintf("atom %s: runtime: %s — the skill declares no enforce block", id, part.Anchor)}
	}

	anchor, isKnown := skill.Enforce.Anchors[part.Anchor]
	if !isKnown {
		return []string{fmt.Sprintf("atom %s: runtime: unknown anchor %s", id, part.Anchor)}
	}
	if part.Capture != "" && !anchor.captures(part.Capture) {
		return []string{fmt.Sprintf("atom %s: runtime: anchor %s captures no %s", id, part.Anchor, part.Capture)}
	}

	kind := atom.EffectiveKind()
	isBool := kind == KindAtomBool
	isCompared := part.Op != ""
	hasCapture := part.Capture != ""
	switch {
	case !isBool && !hasCapture:
		return []string{fmt.Sprintf("atom %s: a %s atom reads a capture value — runtime: %s.<capture>", id, kind, part.Anchor)}
	case !isBool && isCompared:
		return []string{fmt.Sprintf("atom %s: a %s atom takes the capture's value, no comparison", id, kind)}
	case isCompared && !hasCapture:
		return []string{fmt.Sprintf("atom %s: a comparison needs a capture — runtime: %s.<capture> %s %s", id, part.Anchor, part.Op, part.Literal)}
	}
	return nil
}

func sourcePartViolations(skill *Skill, id string, atom *Atom, part *SourcePart, rows map[string]bool) []string {
	var violations []string
	switch part.Kind {
	case SourceArg:
		if !rows[part.Arg] {
			violations = append(violations, fmt.Sprintf("atom %s: arg %s is no Args row", id, part.Arg))
		}
		isEnumMember := atom.EffectiveKind() == KindAtomEnum && slices.Contains(atom.Members, part.Member)
		isMemberOk := part.Member == "" || isEnumMember
		if !isMemberOk {
			violations = append(violations, fmt.Sprintf("atom %s: => %s names no member of this atom", id, part.Member))
		}
	case SourceRuntime:
		violations = append(violations, runtimeViolations(skill, id, atom, part)...)
	case SourceEnvironment, SourceSession:
	}
	return violations
}

// ParseSource splits a source line on " | " and parses every part; an empty line parses to nil.
func ParseSource(source string) ([]SourcePart, error) {
	if source == "" {
		return nil, nil
	}

	var parts []SourcePart
	for text := range strings.SplitSeq(source, sourcePartSep) {
		part, err := parseSourcePart(text)
		if err != nil {
			return nil, err
		}
		parts = append(parts, part)
	}
	return parts, nil
}

// RuntimeReaders maps "<anchor>.<capture>" and "<anchor>" to the local atoms whose source reads them.
func RuntimeReaders(skill *Skill) map[string][]string {
	readers := map[string][]string{}
	ids := slices.Sorted(maps.Keys(skill.Atoms))
	for _, id := range ids {
		parts, err := ParseSource(skill.Atoms[id].Source)
		if err != nil {
			continue
		}

		for _, part := range parts {
			if part.Kind != SourceRuntime {
				continue
			}

			key := part.Anchor
			if part.Capture != "" {
				key += "." + part.Capture
			}
			readers[key] = append(readers[key], id)
		}
	}
	return readers
}

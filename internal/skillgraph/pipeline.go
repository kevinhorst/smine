package skillgraph

import (
	"fmt"
	"slices"
	"strings"
)

// Skill grant rule shape in an allowed-tools line.
const (
	allowedToolsField = "allowed-tools"
	skillGrantClose   = ")"
	skillGrantOpen    = "Skill("
)

// grantedSkills lists the leaves of the Skill(<leaf>) grants; a grant reading a variable names its leaf only once filled and is left out.
func grantedSkills(skill *Skill) []string {
	var granted []string
	for entry := range strings.SplitSeq(skill.Field(allowedToolsField), ",") {
		name, isGrant := strings.CutPrefix(strings.TrimSpace(entry), skillGrantOpen)
		isVariable := strings.HasPrefix(name, "$"+variablePrefix)
		if !isGrant || isVariable {
			continue
		}

		granted = append(granted, strings.TrimSuffix(name, skillGrantClose))
	}
	return granted
}

// hasEdge reports whether skill declares an edge of kind to name.
func hasEdge(skill *Skill, kind, name string) bool {
	routing := skill.Routing()
	if routing == nil {
		return false
	}
	return slices.ContainsFunc(routing.Pipeline.Edges, func(edge Edge) bool {
		return edge.Kind == kind && edge.Skill == name
	})
}

// isOutgoing reports an edge the skill itself follows: to, spawns, escalates.
func isOutgoing(edge Edge) bool {
	switch edge.Kind {
	case EdgeKindTo, EdgeKindSpawns, EdgeKindEscalates:
		return true
	case EdgeKindFrom, EdgeKindSpawnedBy:
		return false
	}
	return false
}

func mirrorKind(kind string) string {
	switch kind {
	case EdgeKindFrom:
		return EdgeKindTo
	case EdgeKindTo:
		return EdgeKindFrom
	case EdgeKindSpawnedBy:
		return EdgeKindSpawns
	case EdgeKindSpawns:
		return EdgeKindSpawnedBy
	case EdgeKindEscalates:
		return ""
	}
	return ""
}

func outgoingSkills(skill *Skill) []string {
	routing := skill.Routing()
	if routing == nil {
		return nil
	}

	var names []string
	for _, edge := range routing.Pipeline.Edges {
		if isOutgoing(edge) {
			names = append(names, edge.Skill)
		}
	}
	return names
}

// reachableSkills includes start itself only through a cycle.
func reachableSkills(byName map[string]*Skill, start *Skill) map[string]bool {
	reached := map[string]bool{}
	queue := []*Skill{start}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, name := range outgoingSkills(current) {
			target, isKnown := byName[name]
			if !isKnown || reached[name] {
				continue
			}

			reached[name] = true
			queue = append(queue, target)
		}
	}
	return reached
}

func skillGrantViolations(byName map[string]*Skill, skill *Skill) []string {
	reached := reachableSkills(byName, skill)
	var violations []string
	for _, granted := range grantedSkills(skill) {
		_, isKnown := byName[granted]
		isReached := granted == skill.Name() || reached[granted]
		switch {
		case !isKnown:
			violations = append(violations, fmt.Sprintf("%s: allowed-tools Skill(%s) — no such skill", skill.Name(), granted))
		case !isReached:
			violations = append(violations, fmt.Sprintf("%s: allowed-tools Skill(%s) — no outgoing edge reaches %s", skill.Name(), granted, granted))
		}
	}
	return violations
}

func CheckPipelines(skills []*Skill) []string {
	byName := map[string]*Skill{}
	for _, skill := range skills {
		byName[skill.Name()] = skill
	}
	var violations []string
	for _, skill := range skills {
		routing := skill.Routing()
		if routing == nil {
			continue
		}

		for _, edge := range routing.Pipeline.Edges {
			target, isKnown := byName[edge.Skill]
			if !isKnown {
				violations = append(violations, fmt.Sprintf("%s: pipeline edge %s %s — no such skill", skill.Name(), edge.Kind, edge.Skill))
				continue
			}

			mirror := mirrorKind(edge.Kind)
			if mirror != "" && !hasEdge(target, mirror, skill.Name()) {
				violations = append(violations, fmt.Sprintf("%s: pipeline edge %s %s — %s declares no %s %s", skill.Name(), edge.Kind, edge.Skill, edge.Skill, mirror, skill.Name()))
			}
		}
	}
	return violations
}

// CheckSkillGrants keeps a permission chain a mapped chain; an edge without a grant is a user handoff.
func CheckSkillGrants(skills []*Skill) []string {
	byName := map[string]*Skill{}
	for _, skill := range skills {
		byName[skill.Name()] = skill
	}
	var violations []string
	for _, skill := range skills {
		violations = append(violations, skillGrantViolations(byName, skill)...)
	}
	return violations
}

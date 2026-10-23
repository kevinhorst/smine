package skillgraph

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/components"
)

// fieldAllowedTools is the frontmatter key of a skill's permission manifest.
const fieldAllowedTools = "allowed-tools"

// allowedToolsView drops a rule referencing an unset variable and a Skill(<leaf>) rule of an undeployable leaf, keeps a deferred reference only for a gate to fill and grants the placeholders' awk; the result keeps the stored leading space, "" when none is left.
func allowedToolsView(deferred *deferral, deployable map[string]bool, isGated bool, values map[string]string, value string) string {
	var kept []string
	for rule := range strings.SplitSeq(value, ",") {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}

		undeferred := deferred.undeferred(rule)
		if values != nil && hasUnsetReference(undeferred, values) {
			continue
		}
		isDeferredRule := undeferred != rule
		if isDeferredRule && !isGated {
			continue
		}
		if !isSkillRuleDeployable(deployable, rule) {
			continue
		}
		kept = append(kept, rule)
	}
	if deferred != nil && !slices.Contains(kept, placeholderGrant) {
		kept = append(kept, placeholderGrant)
	}
	if len(kept) == 0 {
		return ""
	}
	return " " + strings.Join(kept, ", ")
}

// checkRequirements flags ids outside the catalog, entries needing a hard requirement (hard and soft are disjoint) and entries both needing and excluding one component.
func checkRequirements(skill *Skill) []string {
	var violations []string
	for _, id := range skill.Requires {
		violations = append(violations, componentIdViolations(id, "requires")...)
	}
	for _, ref := range skill.Entries() {
		violations = append(violations, entryRequirementViolations(ref.Entry, skill.Requires)...)
	}

	routing := skill.Routing()
	if routing == nil {
		return violations
	}
	return append(violations, routingRequirementViolations(routing, skill.Requires)...)
}

// componentDisabled returns the entries the set drops: a needed component missing or an excluding one present.
func componentDisabled(set map[components.ID]bool, skill *Skill) map[string]bool {
	disabled := map[string]bool{}
	for _, ref := range skill.Entries() {
		isDropped := !isSatisfied(ref.Entry.Needs, set) || isExcluded(set, ref.Entry.Unless)
		if isDropped {
			disabled[ref.Entry.Id] = true
		}
	}
	return disabled
}

// componentIdViolations names an id the catalog lacks.
func componentIdViolations(id components.ID, location string) []string {
	if _, ok := components.Lookup(id); ok {
		return nil
	}
	return []string{fmt.Sprintf("%s: unknown component %q", location, id)}
}

// componentSet is the render target set; nil means every catalog component, the canonical deploy.
func componentSet(ids []components.ID) map[components.ID]bool {
	if ids == nil {
		for _, component := range components.Catalog() {
			ids = append(ids, component.ID)
		}
	}

	set := make(map[components.ID]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// componentView leaves out allowed-tools rules the values or deployability cannot honor, routes and edges whose needs the set misses, edges to undeployable skills, items and anchors reading an unset variable; never changing the model.
func componentView(deferred *deferral, set map[components.ID]bool, deployable map[string]bool, values map[string]string, skill *Skill) *Skill {
	view := *skill
	view.Frontmatter = make([]Field, 0, len(skill.Frontmatter)+1)
	hasAllowedTools := false
	for _, field := range skill.Frontmatter {
		if field.Key != fieldAllowedTools {
			view.Frontmatter = append(view.Frontmatter, field)
			continue
		}

		hasAllowedTools = true
		field.Value = allowedToolsView(deferred, deployable, set[components.Gate], values, field.Value)
		if field.Value != "" {
			view.Frontmatter = append(view.Frontmatter, field)
		}
	}
	if deferred != nil && !hasAllowedTools {
		grant := Field{Key: fieldAllowedTools, Value: " " + placeholderGrant}
		view.Frontmatter = append(view.Frontmatter, grant)
	}

	view.Sections = make([]*Section, 0, len(skill.Sections))
	for _, section := range skill.Sections {
		view.Sections = append(view.Sections, sectionView(deferred, section, set, deployable, values))
	}
	if values != nil {
		view.Enforce = enforceView(skill.Enforce, values)
	}
	return &view
}

// entryRequirementViolations checks one entry's needs and unless ids.
func entryRequirementViolations(entry *Entry, requires []components.ID) []string {
	var violations []string
	for _, id := range entry.Needs {
		violations = append(violations, componentIdViolations(id, entry.Id+": needs")...)
		if slices.Contains(requires, id) {
			violations = append(violations, fmt.Sprintf("%s: needs %s, which the skill requires — a requirement is hard or soft, never both", entry.Id, id))
		}
		if slices.Contains(entry.Unless, id) {
			violations = append(violations, fmt.Sprintf("%s: needs and unless both name %s", entry.Id, id))
		}
	}
	for _, id := range entry.Unless {
		violations = append(violations, componentIdViolations(id, entry.Id+": unless")...)
	}
	return violations
}

// isExcluded reports whether the set holds one of the excluding ids.
func isExcluded(set map[components.ID]bool, unless []components.ID) bool {
	return slices.ContainsFunc(unless, func(id components.ID) bool {
		return set[id]
	})
}

// isSatisfied reports whether the set holds every needed id.
func isSatisfied(needs []components.ID, set map[components.ID]bool) bool {
	for _, id := range needs {
		if !set[id] {
			return false
		}
	}
	return true
}

// isSkillRuleDeployable reads Skill(<leaf>) rules only; a nil map means every leaf deploys.
func isSkillRuleDeployable(deployable map[string]bool, rule string) bool {
	leaf, isSkillRule := strings.CutPrefix(rule, "Skill(")
	if !isSkillRule || deployable == nil {
		return true
	}

	leaf = strings.TrimSuffix(leaf, ")")
	isDeployable, isKnown := deployable[leaf]
	return !isKnown || isDeployable
}

// routingRequirementViolations checks the needs ids of every route and edge, and that none names a required component.
func routingRequirementViolations(routing *Routing, requires []components.ID) []string {
	var violations []string
	for _, route := range routing.Routes {
		for _, id := range route.Needs {
			violations = append(violations, componentIdViolations(id, "route "+route.Use+": needs")...)
			if slices.Contains(requires, id) {
				violations = append(violations, fmt.Sprintf("route %s: needs %s, which the skill requires — a requirement is hard or soft, never both", route.Use, id))
			}
		}
	}
	for _, edge := range routing.Pipeline.Edges {
		for _, id := range edge.Needs {
			violations = append(violations, componentIdViolations(id, "edge "+edge.Kind+" "+edge.Skill+": needs")...)
			if slices.Contains(requires, id) {
				violations = append(violations, fmt.Sprintf("edge %s %s: needs %s, which the skill requires — a requirement is hard or soft, never both", edge.Kind, edge.Skill, id))
			}
		}
	}
	return violations
}

// routingView drops the routes and edges whose needs the set misses and the edges to undeployable skills.
func routingView(routing *Routing, set map[components.ID]bool, deployable map[string]bool) *Routing {
	view := *routing
	view.Routes = make([]Route, 0, len(routing.Routes))
	for _, route := range routing.Routes {
		if isSatisfied(route.Needs, set) {
			view.Routes = append(view.Routes, route)
		}
	}

	view.Pipeline.Edges = make([]Edge, 0, len(routing.Pipeline.Edges))
	for _, edge := range routing.Pipeline.Edges {
		isDeployable, isKnown := deployable[edge.Skill]
		isTargetThere := deployable == nil || !isKnown || isDeployable
		if isSatisfied(edge.Needs, set) && isTargetThere {
			view.Pipeline.Edges = append(view.Pipeline.Edges, edge)
		}
	}
	return &view
}

// sectionView is the section with every routing block replaced by its view and, with values, every entry without its unset-variable items.
func sectionView(deferred *deferral, section *Section, set map[components.ID]bool, deployable map[string]bool, values map[string]string) *Section {
	view := *section
	view.Blocks = slices.Clone(section.Blocks)
	for index := range view.Blocks {
		block := &view.Blocks[index]
		if block.Routing != nil {
			block.Routing = routingView(block.Routing, set, deployable)
		}
		if block.Entry != nil && values != nil {
			block.Entry = itemView(deferred, block.Entry, values)
		}
	}
	return &view
}

// RequiredComponents is the requires list, then Skill environment when a required repo-scoped variable needs it.
func (s *Skill) RequiredComponents() []components.ID {
	required := slices.Clone(s.Requires)
	if !slices.Contains(required, components.SkillEnvironment) && s.RequiresSessionValues() {
		required = append(required, components.SkillEnvironment)
	}
	return required
}

// MissingRequirements returns the required components installed lacks in RequiredComponents order; nil installed means every component, so none.
func MissingRequirements(installed []components.ID, skill *Skill) []components.ID {
	set := componentSet(installed)
	var missing []components.ID
	for _, id := range skill.RequiredComponents() {
		if !set[id] {
			missing = append(missing, id)
		}
	}
	return missing
}

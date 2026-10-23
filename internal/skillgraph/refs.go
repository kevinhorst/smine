package skillgraph

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// refSeparator joins the title and the short id of a rendered link.
const refSeparator = " · "

// refTokenPattern is the format-3 link: [[SECTION-NNN]] inside the skill, [[SKILL-X-SECTION-NNN]] across skills.
var refTokenPattern = regexp.MustCompile(`\[\[((?:SKILL-[A-Z0-9]{2,24}-)?[A-Z0-9]{2,12}-[0-9]{3}[a-z]?)\]\]`)

// refTarget is what a token points at: its id and the entry when it is this skill's.
type refTarget struct {
	id  string
	ref EntryRef

	isKnown bool
	isLocal bool
}

// checkRefs: every token resolves, obeys the ref discipline, and no def or payload links out.
func checkRefs(skill *Skill) []string {
	if entryFormat(skill) < entryFormatFields {
		return nil
	}

	var violations []string
	for _, ref := range skill.Entries() {
		entry := ref.Entry
		anchors := routeAnchors(skill, ref.Section)
		for _, id := range entry.Refs(skill.Name()) {
			target := resolveRef(skill, id)
			if !target.isLocal {
				continue
			}
			if !target.isKnown {
				violations = append(violations, fmt.Sprintf("%s: link %s: no such entry", entry.Id, shortId(skill.Name(), id)))
				continue
			}

			isLeaf := entry.Role == RoleDef || entry.Role == RolePayload
			if isLeaf {
				violations = append(violations, fmt.Sprintf("%s: a %s links nothing — %s", entry.Id, entry.Role, shortId(skill.Name(), id)))
				continue
			}

			targetRole := target.ref.Entry.Role
			isVocabulary := targetRole == RoleDef || targetRole == RolePayload
			isSameSection := target.ref.Section == ref.Section
			isAnchorRef := slices.Contains(anchors, id)
			isDisciplined := isVocabulary || isSameSection || isAnchorRef
			if !isDisciplined {
				violations = append(violations, fmt.Sprintf("%s: link %s: a %s in section %q — flow links stay inside the section", entry.Id, shortId(skill.Name(), id), targetRole, target.ref.Section.Title))
			}
		}
	}
	for _, id := range navigationTokens(skill) {
		if target := resolveRef(skill, id); target.isLocal && !target.isKnown {
			violations = append(violations, fmt.Sprintf("args or routing: link %s: no such entry", shortId(skill.Name(), id)))
		}
	}
	return violations
}

// checkTitles: entry titles are unique per skill, case-insensitively.
func checkTitles(skill *Skill) []string {
	if entryFormat(skill) < entryFormatFields {
		return nil
	}

	var violations []string
	owners := map[string]string{}
	for _, ref := range skill.Entries() {
		key := strings.ToLower(ref.Entry.Title)
		if owner, isTaken := owners[key]; isTaken {
			violations = append(violations, fmt.Sprintf("%s: title %q is also %s's", ref.Entry.Id, ref.Entry.Title, owner))
			continue
		}
		owners[key] = ref.Entry.Id
	}
	return violations
}

// crossRefViolations checks one entry's foreign links against the skills under the root.
func crossRefViolations(skill *Skill, entry *Entry, byName map[string]*Skill) []string {
	var violations []string
	for _, id := range entry.Refs(skill.Name()) {
		if resolveRef(skill, id).isLocal {
			continue
		}

		parts := strings.SplitN(id, "-", entryIdParts)
		target, isKnown := byName[parts[entryIdNamePart]]
		if !isKnown {
			violations = append(violations, fmt.Sprintf("%s: %s: link %s: no skill %s under the root", skill.Name(), entry.Id, id, parts[entryIdNamePart]))
			continue
		}
		if _, isEntry := target.Entry(id); !isEntry {
			violations = append(violations, fmt.Sprintf("%s: %s: link %s: no such entry in %s", skill.Name(), entry.Id, id, target.Name()))
		}
	}
	return violations
}

// displayText replaces every token by its target's title, the way the reader sees it, for the rune caps.
func displayText(skill *Skill, text string) string {
	return refTokenPattern.ReplaceAllStringFunc(text, func(token string) string {
		id := tokenId(skill.Name(), refTokenPattern.FindStringSubmatch(token)[1])
		if ref, isKnown := skill.Entry(id); isKnown {
			return ref.Entry.Title
		}
		return shortId(skill.Name(), id)
	})
}

// navigationTokens lists the full ids the Args and Routing blocks link, which resolve but obey no discipline.
func navigationTokens(skill *Skill) []string {
	var texts []string
	for _, row := range skill.Args() {
		texts = append(texts, row.Default, row.Effect)
	}
	if routing := skill.Routing(); routing != nil {
		texts = append(texts, routing.Purpose)
		texts = append(texts, routing.Preconditions...)
		for _, route := range routing.Routes {
			texts = append(texts, route.Use)
		}
	}
	var ids []string
	for _, text := range texts {
		for _, match := range refTokenPattern.FindAllStringSubmatch(text, -1) {
			ids = append(ids, tokenId(skill.Name(), match[1]))
		}
	}
	return ids
}

// resolveRef classifies a full id: this skill's and known, this skill's and missing, or another skill's.
func resolveRef(skill *Skill, id string) refTarget {
	target := refTarget{id: id}
	target.isLocal = strings.HasPrefix(id, entryIdPrefix+SkillIdName(skill.Name())+"-")
	if !target.isLocal {
		return target
	}
	target.ref, target.isKnown = skill.Entry(id)
	return target
}

// routeAnchors lists the full ids a route section's anchor clause names; a phase or context section names none.
func routeAnchors(skill *Skill, section *Section) []string {
	if section.EffectiveKind() != SectionKindRoute {
		return nil
	}

	replaces, between, err := section.Anchor(skill.Name())
	if err != nil {
		return nil
	}

	var ids []string
	items := slices.Concat(replaces, between)
	for _, item := range items {
		first, last, isRange := strings.Cut(item, anchorRangeSeparator)
		ids = append(ids, first)
		if isRange {
			ids = append(ids, last)
		}
	}
	return ids
}

// shortId drops this skill's prefix: SKILL-X-SECTION-NNN reads SECTION-NNN inside skill x, another skill's id stays whole.
func shortId(skillName, id string) string {
	return strings.TrimPrefix(id, entryIdPrefix+SkillIdName(skillName)+"-")
}

// tokenId is a token's full entry id: a short token takes the skill's prefix, a SKILL- token stays whole.
func tokenId(skillName, token string) string {
	if strings.HasPrefix(token, entryIdPrefix) {
		return token
	}
	return resolveAnchorId(token, entryIdPrefix+SkillIdName(skillName)+"-")
}

// uniqueInOrder keeps the first occurrence of every id.
func uniqueInOrder(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	var unique []string
	for _, id := range ids {
		if seen[id] {
			continue
		}

		seen[id] = true
		unique = append(unique, id)
	}
	return unique
}

// ExpandTokens replaces every token in text by its rendered link.
func ExpandTokens(skill *Skill, text string) string {
	return refTokenPattern.ReplaceAllStringFunc(text, func(token string) string {
		return RenderRef(skill, refTokenPattern.FindStringSubmatch(token)[1])
	})
}

// RenderRef renders a token's link: [<title> · <short id>](#<anchor>); an unknown or foreign target keeps its id as the title.
func RenderRef(skill *Skill, token string) string {
	id := tokenId(skill.Name(), token)
	title := shortId(skill.Name(), id)
	if ref, isKnown := skill.Entry(id); isKnown {
		title = ref.Entry.Title
	}
	return "[" + title + refSeparator + shortId(skill.Name(), id) + "](#" + strings.ToLower(id) + ")"
}

// TextRefs lists the entry ids one text links — markdown entry links, then [[ID]] tokens — each once; a short token takes the skill's prefix.
func TextRefs(skillName, text string) []string {
	var ids []string
	for _, match := range refPattern.FindAllStringSubmatch(text, -1) {
		ids = append(ids, anchorToEntryId(match[1]))
	}
	for _, match := range refTokenPattern.FindAllStringSubmatch(text, -1) {
		ids = append(ids, tokenId(skillName, match[1]))
	}
	return uniqueInOrder(ids)
}

// UnresolvedCrossRefs reports every foreign token of a format-3 skill whose target skill under the root lacks the entry.
func UnresolvedCrossRefs(skills []*Skill) []string {
	byName := map[string]*Skill{}
	for _, skill := range skills {
		byName[SkillIdName(skill.Name())] = skill
	}
	var violations []string
	for _, skill := range skills {
		if entryFormat(skill) < entryFormatFields {
			continue
		}

		for _, ref := range skill.Entries() {
			violations = append(violations, crossRefViolations(skill, ref.Entry, byName)...)
		}
	}
	return violations
}

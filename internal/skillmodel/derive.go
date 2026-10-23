package skillmodel

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

var deriveSlugPattern = regexp.MustCompile(`[^a-z0-9]+`)

// Derive involves no judgment: an entry without clauses is simply uncovered.
func Derive(skill *skillgraph.Skill) (*Model, error) {
	if violations := skillgraph.Check(skill); len(violations) > 0 {
		return nil, fmt.Errorf("Derive: %s: %s", skill.Name(), strings.Join(violations, "; "))
	}

	model := &Model{
		IsStrict:   skill.Tier == skillgraph.TierStrict,
		Skill:      skill.Name(),
		Vocabulary: VocabularyOf(skill),
	}
	refs := skill.Entries()
	sectionIds := map[string]bool{}
	previousPhase := ""
	for _, ref := range refs {
		entry := ref.Entry
		if !skillgraph.CoverableRole(entry.Role) {
			continue
		}
		sectionId := deriveSlug(ref.Section.Title)
		if sectionId != "" && !sectionIds[sectionId] {
			sectionIds[sectionId] = true
			kind := ref.Section.EffectiveKind()
			section := Section{Id: sectionId, Kind: kind, Pred: ref.Section.Pred, Title: ref.Section.Title}
			replaces, between, err := ref.Section.Anchor(skill.Name())
			if err != nil {
				return nil, fmt.Errorf(
					"Derive: %s: %w",
					skill.Name(),
					err,
				)
			}

			section.Between = between
			section.Replaces = expandReplaces(refs, replaces)
			if kind == skillgraph.SectionKindPhase {
				if previousPhase != "" {
					section.After = []string{previousPhase}
				}
				previousPhase = sectionId
			}
			model.Sections = append(model.Sections, section)
		}
		model.Coverable++
		if entry.Pred != "" || len(entry.Post) > 0 {
			model.Formalized++
		}
		if entry.Role == skillgraph.RoleRule {
			if entry.Pred != "" {
				invariant := Invariant{Id: entry.Id, Expr: entry.Pred, Section: sectionId, Text: entry.Statement}
				model.Invariants = append(model.Invariants, invariant)
			}
			continue
		}
		if entry.Role == skillgraph.RoleQuality {
			if entry.Pred != "" {
				soft := Invariant{Id: entry.Id, Expr: entry.Pred, Section: sectionId, Text: entry.Statement}
				model.Soft = append(model.Soft, soft)
			}
			continue
		}
		transition := Transition{
			Id:      entry.Id,
			Kind:    entry.Role,
			Post:    slices.Clone(entry.Post),
			Pred:    conjoinRoutePred(ref.Section.Pred, entry.Pred),
			Section: sectionId,
			Text:    entry.Statement,
		}
		model.Transitions = append(model.Transitions, transition)
	}
	return model, nil
}

// conjoinRoutePred: entering the route means the route pred holds at the firing.
func conjoinRoutePred(routePred, entryPred string) string {
	if routePred == "" {
		return entryPred
	}
	if entryPred == "" {
		return routePred
	}
	return "(" + routePred + ") and (" + entryPred + ")"
}

// deriveSlug lowercases a section title into its id form.
func deriveSlug(title string) string {
	slug := deriveSlugPattern.ReplaceAllString(strings.ToLower(title), "-")
	return strings.Trim(slug, "-")
}

// expandReplaces relies on skillgraph.Check having resolved both range ends outside the route, FIRST before LAST.
func expandReplaces(entries []skillgraph.EntryRef, items []string) []string {
	if len(items) == 0 {
		return nil
	}

	positions := map[string]int{}
	for index, ref := range entries {
		positions[ref.Entry.Id] = index
	}
	isReplaced := make([]bool, len(entries))
	for _, item := range items {
		span := splitSpan(item)
		for index := positions[span.First]; index <= positions[span.Last]; index++ {
			isReplaced[index] = true
		}
	}
	var replaced []string
	for index, ref := range entries {
		if isReplaced[index] && skillgraph.CoverableRole(ref.Entry.Role) {
			replaced = append(replaced, ref.Entry.Id)
		}
	}
	return replaced
}

package skillgraph

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
)

// Reader roles beyond an entry's own role.
const (
	ReaderQuery   = "query"
	ReaderSection = "section"
)

var (
	// checkableRoles are the readers that keep an atom alive on a format-3 skill: the facts rules and stops decide on.
	checkableRoles = map[string]bool{ReaderQuery: true, ReaderSection: true, RoleQuality: true, RoleRule: true, RoleStop: true}
	// exprKeywords are the clause grammar's word operators; every other identifier names an atom or an enum member.
	exprKeywords = map[string]bool{"and": true, "not": true, "or": true}
	// predIdentPattern also matches enum members and keywords; only atom ids are looked up.
	predIdentPattern = regexp.MustCompile(`[A-Za-z_]\w*`)
)

// Reader is one place an atom is read: an entry pred, a route section pred or a query file.
type Reader struct {
	Entry   string `json:"entry,omitempty"`
	Query   string `json:"query,omitempty"`
	Role    string `json:"role"`
	Section string `json:"section"`
}

// IsCheckable reports whether the reader is one the model can prove something over.
func (r *Reader) IsCheckable() bool {
	return checkableRoles[r.Role]
}

// checkClauseAtoms: on a format-3 skill every identifier of a pred or post is an atom or a member of an enum atom.
func checkClauseAtoms(skill *Skill) []string {
	if entryFormat(skill) < entryFormatFields {
		return nil
	}

	known := map[string]bool{}
	for id, atom := range skill.AllAtoms() {
		known[id] = true
		for _, member := range atom.Members {
			known[member] = true
		}
	}
	var violations []string
	report := func(subject, text string) {
		for _, token := range predIdentPattern.FindAllString(text, -1) {
			if !known[token] && !exprKeywords[token] {
				violations = append(violations, fmt.Sprintf("%s: %s is no atom and no enum member", subject, token))
			}
		}
	}
	for _, section := range skill.Sections {
		report(fmt.Sprintf("section %q", section.Title), section.Pred)
	}
	for _, ref := range skill.Entries() {
		report(ref.Entry.Id+": pred", ref.Entry.Pred)
		for _, atom := range ref.Entry.Post {
			report(ref.Entry.Id+": post", atom)
		}
	}
	return violations
}

func hasCheckableReader(readers []Reader) bool {
	return slices.ContainsFunc(readers, func(reader Reader) bool { return reader.IsCheckable() })
}

// AtomReaders maps every identifier read by a pred or a query to its readers, document order, queries last.
func AtomReaders(skill *Skill) map[string][]Reader {
	readers := map[string][]Reader{}
	add := func(text string, reader Reader) {
		tokens := predIdentPattern.FindAllString(text, -1)
		slices.Sort(tokens)
		for _, token := range slices.Compact(tokens) {
			readers[token] = append(readers[token], reader)
		}
	}
	for _, section := range skill.Sections {
		add(section.Pred, Reader{Role: ReaderSection, Section: section.Title})
	}
	for _, ref := range skill.Entries() {
		add(ref.Entry.Pred, Reader{Entry: ref.Entry.Id, Role: ref.Entry.Role, Section: ref.Section.Title})
	}
	for _, query := range skill.Queries {
		queryTexts := append(slices.Clone(query.Assume), query.Check)
		for _, text := range queryTexts {
			add(text, Reader{Query: query.Name, Role: ReaderQuery})
		}
	}
	return readers
}

// UnreadAtoms: a format-3 skill's local atom needs a stop, rule, quality, route or query reader; a format-2 skill's any pred; a shared atom a checkable reader in any skill.
func UnreadAtoms(skills []*Skill) []string {
	sharedRead := map[string]bool{}
	var violations []string
	for _, skill := range skills {
		readers := AtomReaders(skill)
		isStrictReaders := entryFormat(skill) >= entryFormatFields
		for id, atomReaders := range readers {
			if hasCheckableReader(atomReaders) {
				sharedRead[id] = true
			}
		}
		for _, id := range slices.Sorted(maps.Keys(skill.Atoms)) {
			isRead := len(readers[id]) > 0
			if isStrictReaders {
				isRead = hasCheckableReader(readers[id])
			}
			switch {
			case isRead:
			case isStrictReaders:
				violations = append(violations, fmt.Sprintf("%s: atom %s: no stop, rule, quality, route or query reads it — delete it and the clauses naming it", skill.Name(), id))
			default:
				violations = append(violations, fmt.Sprintf("%s: atom %s: no pred reads it — delete it and the posts granting it", skill.Name(), id))
			}
		}
	}
	if len(skills) == 0 || skills[0].Vocabulary == nil {
		return violations
	}

	for _, id := range slices.Sorted(maps.Keys(skills[0].Vocabulary.Atoms)) {
		if !sharedRead[id] {
			violations = append(violations, fmt.Sprintf("%s: atom %s: no skill's stop, rule, quality, route or query reads it — delete it and the clauses naming it", FileVocabulary, id))
		}
	}
	return violations
}

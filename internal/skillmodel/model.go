// Package skillmodel derives a skill's semantic model, a guarded transition system that is never committed.
package skillmodel

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

// Transition kinds mirror the flow-role subset of the entry ontology.
const (
	KindSpawn   = "spawn"
	KindStep    = "step"
	KindStop    = "stop"
	KindSubstep = "substep"
	KindVerify  = "verify"
)

// Atom's Source marks an input set by the world; a bool atom guarding flow must be granted or carry one.
type Atom struct {
	Id string `json:"id"`

	Aliases []string `json:"aliases,omitempty"`
	Kind    string   `json:"kind,omitempty"`
	Members []string `json:"members,omitempty"`
	Source  string   `json:"source,omitempty"`
}

// EffectiveKind returns the atom's kind with the empty default resolved.
func (a Atom) EffectiveKind() string {
	if a.Kind == "" {
		return skillgraph.KindAtomBool
	}
	return a.Kind
}

// Invariant's Section is what a route replacing the entry ranks by.
type Invariant struct {
	Id string

	Expr    string
	Section string
	Text    string
}

// Model's Vocabulary is the one vocabulary every emitter, Validate and the battery read.
type Model struct {
	Coverable   int
	Formalized  int
	Invariants  []Invariant
	IsStrict    bool
	Sections    []Section
	Skill       string
	Soft        []Invariant
	Transitions []Transition
	Vocabulary  *Vocabulary
}

// Section's Replaces lists every coverable entry id the route replaces, ranges expanded in document order.
type Section struct {
	Id string

	After    []string
	Between  []string
	Kind     string
	Pred     string
	Replaces []string
	Title    string
}

// Transition's empty Pred means true.
type Transition struct {
	Id string

	Kind    string
	Post    []string
	Pred    string
	Section string
	Text    string
}

// Vocabulary's exclusivity groups are atom sets that can never hold together.
type Vocabulary struct {
	Exclusive [][]string        `json:"exclusive,omitempty"`
	Sorts     map[string][]Atom `json:"sorts"`
}

// VocabularyOf lists the vocabulary's exclusivity groups before the skill's.
func VocabularyOf(skill *skillgraph.Skill) *Vocabulary {
	vocab := &Vocabulary{Sorts: map[string][]Atom{}}
	for id, atom := range skill.AllAtoms() {
		row := Atom{Id: id, Aliases: atom.Aliases, Kind: atom.Kind, Members: atom.Members, Source: atom.Source}
		vocab.Sorts[atom.Sort] = append(vocab.Sorts[atom.Sort], row)
	}
	for _, atoms := range vocab.Sorts {
		slices.SortFunc(atoms, func(a, b Atom) int { return strings.Compare(a.Id, b.Id) })
	}
	if skill.Vocabulary != nil {
		vocab.Exclusive = append(vocab.Exclusive, skill.Vocabulary.Exclusive...)
	}
	vocab.Exclusive = append(vocab.Exclusive, skill.Exclusive...)
	return vocab
}

// AtomIds returns every atom id the vocabulary declares.
func (v *Vocabulary) AtomIds() map[string]bool {
	ids := map[string]bool{}
	for _, atoms := range v.Sorts {
		for _, atom := range atoms {
			ids[atom.Id] = true
		}
	}
	return ids
}

// Info returns every declared atom by id.
func (v *Vocabulary) Info() map[string]Atom {
	info := map[string]Atom{}
	for _, atoms := range v.Sorts {
		for _, atom := range atoms {
			info[atom.Id] = atom
		}
	}
	return info
}

// MemberOwner maps every enum member to its owning enum atom id.
func (v *Vocabulary) MemberOwner() map[string]string {
	owners := map[string]string{}
	for _, atoms := range v.Sorts {
		for _, atom := range atoms {
			for _, member := range atom.Members {
				owners[member] = atom.Id
			}
		}
	}
	return owners
}

// checkAtoms requires globally unique member names: members become global SMT constructors.
func (v *Vocabulary) checkAtoms() []string {
	var violations []string
	seenAtom := map[string]bool{}
	seenMember := map[string]bool{}
	for _, atoms := range v.Sorts {
		for _, atom := range atoms {
			if seenAtom[atom.Id] {
				violations = append(violations, fmt.Sprintf("atom %s: duplicate id", atom.Id))
			}
			seenAtom[atom.Id] = true
			switch atom.EffectiveKind() {
			case skillgraph.KindAtomBool, skillgraph.KindAtomInt:
				if len(atom.Members) > 0 {
					violations = append(violations, fmt.Sprintf("atom %s: members on non-enum kind %q", atom.Id, atom.EffectiveKind()))
				}
			case skillgraph.KindAtomEnum:
				if len(atom.Members) == 0 {
					violations = append(violations, fmt.Sprintf("atom %s: enum without members", atom.Id))
				}
			default:
				violations = append(violations, fmt.Sprintf("atom %s: unknown kind %q", atom.Id, atom.Kind))
			}
			for _, member := range atom.Members {
				if seenMember[member] {
					violations = append(violations, fmt.Sprintf("atom %s: duplicate member %q", atom.Id, member))
				}
				seenMember[member] = true
			}
		}
	}
	for member := range seenMember {
		if seenAtom[member] {
			violations = append(violations, fmt.Sprintf("member %q collides with an atom id", member))
		}
	}
	return violations
}

// checkExclusive: the solver asserts each group over Bool constants, so other members would reach z3 mistyped.
func (v *Vocabulary) checkExclusive() []string {
	info := v.Info()
	var violations []string
	for groupIndex, group := range v.Exclusive {
		for _, member := range group {
			atom, ok := info[member]
			if ok && atom.EffectiveKind() == skillgraph.KindAtomBool {
				continue
			}

			violation := fmt.Sprintf("exclusive group %d: member %q is not a declared bool atom", groupIndex, member)
			violations = append(violations, violation)
		}
	}
	return violations
}

// Stats: coverage is reported, never enforced.
func (m *Model) Stats() (formalized, coverable int) {
	return m.Formalized, m.Coverable
}

// Validate applies the vocabulary kind rules only when IsStrict; otherwise clauses are syntax-checked annotations.
func (m *Model) Validate() []string {
	vocab := m.Vocabulary
	if vocab == nil {
		vocab = &Vocabulary{}
	}

	violations := vocab.checkAtoms()
	exclusive := vocab.checkExclusive()
	violations = append(violations, exclusive...)
	info := vocab.Info()
	sectionIds := map[string]bool{}
	for _, section := range m.Sections {
		sectionIds[section.Id] = true
	}
	for _, invariant := range m.Invariants {
		found := checkExpr(
			"invariant",
			invariant.Id,
			invariant.Expr,
			vocab,
			m.IsStrict,
		)
		violations = append(violations, found...)
	}
	for _, soft := range m.Soft {
		found := checkExpr(
			"soft",
			soft.Id,
			soft.Expr,
			vocab,
			m.IsStrict,
		)
		violations = append(violations, found...)
	}
	for _, transition := range m.Transitions {
		if transition.Pred != "" {
			found := checkExpr(
				"pred",
				transition.Id,
				transition.Pred,
				vocab,
				m.IsStrict,
			)
			violations = append(violations, found...)
		}
		for _, atom := range transition.Post {
			declared, ok := info[atom]
			if !ok {
				if m.IsStrict {
					violations = append(violations, fmt.Sprintf("post %s: unresolved atom %q", transition.Id, atom))
				}
				continue
			}
			if declared.EffectiveKind() != skillgraph.KindAtomBool {
				violations = append(violations, fmt.Sprintf("post %s: atom %q has kind %s — post grants bool atoms only", transition.Id, atom, declared.EffectiveKind()))
			}
		}
		if !sectionIds[transition.Section] {
			violations = append(violations, fmt.Sprintf("transition %s: unknown section %q", transition.Id, transition.Section))
		}
	}
	for _, section := range m.Sections {
		for _, after := range section.After {
			if !sectionIds[after] {
				violations = append(violations, fmt.Sprintf("section %s: unknown after-section %q", section.Id, after))
			}
		}
	}
	if m.IsStrict {
		violations = append(violations, m.checkProducers(vocab)...)
	}
	return violations
}

// atomIdentPattern matches candidate atom identifiers inside a pred text.
var atomIdentPattern = regexp.MustCompile(`[A-Za-z_]\w*`)

// checkProducers exempts invariant-only atoms: they are conduct vocabulary.
func (m *Model) checkProducers(vocab *Vocabulary) []string {
	info := vocab.Info()
	granted := map[string]bool{}
	for _, transition := range m.Transitions {
		for _, atom := range transition.Post {
			granted[atom] = true
		}
	}
	var violations []string
	flagged := map[string]bool{}
	for _, transition := range m.Transitions {
		for _, token := range atomIdentPattern.FindAllString(transition.Pred, -1) {
			atom, ok := info[token]
			if !ok || atom.EffectiveKind() != skillgraph.KindAtomBool || granted[token] || atom.Source != "" || flagged[token] {
				continue
			}
			flagged[token] = true
			violations = append(violations, fmt.Sprintf("pred %s: bool atom %q guards flow but nothing grants it and it declares no source", transition.Id, token))
		}
	}
	return violations
}

func checkExpr(kind, id, input string, vocab *Vocabulary, strict bool) []string {
	expr, err := ParseExpr(input)
	if err != nil {
		return []string{fmt.Sprintf("%s %s: %v", kind, id, err)}
	}
	if !strict {
		return nil
	}
	checker := &kindChecker{
		info:    vocab.Info(),
		owners:  vocab.MemberOwner(),
		subject: fmt.Sprintf("%s %s", kind, id),
	}
	checker.walk(expr)
	return checker.violations
}

// Side classifications of a comparison operand.
const (
	sideEnumAtom = "enum-atom"
	sideInt      = "int"
	sideInvalid  = "invalid"
	sideMember   = "member"
)

// kindChecker pairs comparison sides as int-with-int or enum-with-member/enum of the same sort.
type kindChecker struct {
	info       map[string]Atom
	owners     map[string]string
	subject    string
	violations []string
}

// classifyTerm: any arithmetic or literal forces every identifier in the side to be an int atom.
func (c *kindChecker) classifyTerm(term *Term) (side, sort string) {
	if term.Kind == TermIdent {
		if atom, ok := c.info[term.Ident]; ok {
			switch atom.EffectiveKind() {
			case skillgraph.KindAtomInt:
				return sideInt, ""
			case skillgraph.KindAtomEnum:
				return sideEnumAtom, atom.Id
			case skillgraph.KindAtomBool:
				c.report("atom %q used in a comparison but has kind bool", term.Ident)
				return sideInvalid, ""
			}
		}
		if owner, ok := c.owners[term.Ident]; ok {
			return sideMember, owner
		}
		c.report("unresolved atom %q", term.Ident)
		return sideInvalid, ""
	}
	for _, ident := range term.Idents() {
		atom, ok := c.info[ident]
		if !ok {
			c.report("unresolved atom %q", ident)
			return sideInvalid, ""
		}
		if atom.EffectiveKind() != skillgraph.KindAtomInt {
			c.report("atom %q used in arithmetic but has kind %s", ident, atom.EffectiveKind())
			return sideInvalid, ""
		}
	}
	return sideInt, ""
}

func (c *kindChecker) report(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	c.violations = append(c.violations, fmt.Sprintf("%s: %s", c.subject, message))
}

// walk enforces the kind rules over one expression node.
func (c *kindChecker) walk(expr *Expr) {
	switch expr.Kind {
	case ExprAtom:
		atom, ok := c.info[expr.Atom]
		if !ok {
			c.report("unresolved atom %q", expr.Atom)
			return
		}
		if atom.EffectiveKind() != skillgraph.KindAtomBool {
			c.report("atom %q used as boolean but has kind %s", expr.Atom, atom.EffectiveKind())
		}
	case ExprNot:
		c.walk(expr.Left)
	case ExprAnd, ExprOr, ExprImplies:
		c.walk(expr.Left)
		c.walk(expr.Right)
	case ExprCmp:
		c.walkCmp(expr.Cmp)
	}
}

// walkCmp checks one comparison's side pairing.
func (c *kindChecker) walkCmp(cmp *Cmp) {
	leftSide, leftSort := c.classifyTerm(cmp.Left)
	rightSide, rightSort := c.classifyTerm(cmp.Right)
	if leftSide == sideInvalid || rightSide == sideInvalid {
		return
	}
	if leftSide == sideInt && rightSide == sideInt {
		return
	}
	isEnumPair := leftSide != sideInt && rightSide != sideInt &&
		(leftSide == sideEnumAtom || rightSide == sideEnumAtom)
	if isEnumPair && leftSort == rightSort {
		return
	}
	if isEnumPair {
		c.report("comparison mixes enum sorts %q and %q", leftSort, rightSort)
		return
	}
	c.report("comparison mixes int and enum sides")
}

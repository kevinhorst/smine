package skillmodel

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

const betweenPairLength = 2

// Anchors is the one source of route anchor semantics (seam C12) the SMT encoding and the conformance replay read.
type Anchors struct {
	Replacing map[string][]Route
	Routes    map[string]*Expr
	Windows   map[string]RankWindow
}

// NewAnchors leaves context transitions without a window.
func NewAnchors(model *Model) (*Anchors, error) {
	anchors := &Anchors{
		Replacing: map[string][]Route{},
		Routes:    map[string]*Expr{},
		Windows:   map[string]RankWindow{},
	}
	for _, section := range model.Sections {
		if section.Kind != skillgraph.SectionKindRoute {
			continue
		}

		pred, err := ParseExpr(section.Pred)
		if err != nil {
			return nil, fmt.Errorf("NewAnchors: route %s: %w", section.Id, err)
		}

		anchors.Routes[section.Id] = pred
		for _, id := range section.Replaces {
			route := Route{Pred: pred, SectionId: section.Id}
			anchors.Replacing[id] = append(anchors.Replacing[id], route)
		}
	}

	windows := sectionWindows(model)
	for _, transition := range model.Transitions {
		if window, isRanked := windows[transition.Section]; isRanked {
			anchors.Windows[transition.Id] = window
		}
	}
	return anchors, nil
}

// ReplacedClause weakens I to ¬R ⇒ I: while a route runs in the entry's place, its rule is suspended.
func (a *Anchors) ReplacedClause(clause *Expr, entryId string) *Expr {
	for _, route := range a.Replacing[entryId] {
		clause = impliesExpr(notExpr(route.Pred), clause)
	}
	return clause
}

// ReplacedGuard blocks a replaced transition while any replacing route's pred holds.
func (a *Anchors) ReplacedGuard(pred *Expr, transitionId string) *Expr {
	guard := pred
	for _, route := range a.Replacing[transitionId] {
		blocked := notExpr(route.Pred)
		if guard == nil {
			guard = blocked
			continue
		}

		guard = andExpr(guard, blocked)
	}
	return guard
}

// RankWindow is the inclusive range of phase ranks a transition may fire in.
type RankWindow struct {
	Hi int
	Lo int
}

// Route is one route replacing an entry: its parsed pred and its section id.
type Route struct {
	Pred      *Expr
	SectionId string
}

// encoding's guards hold nil for an unguarded transition.
type encoding struct {
	anchors        *Anchors
	guards         map[string]*Expr
	invariants     []namedExpr
	ownConjuncts   map[string][]*Expr
	routeConjuncts map[string][]*Expr
	soft           []namedExpr
}

// newEncoding fails on an unparseable clause, never drops a guard.
func newEncoding(model *Model) (*encoding, error) {
	anchors, err := NewAnchors(model)
	if err != nil {
		return nil, err
	}

	invariants, err := parseInvariants(anchors, model.Invariants, "invariant")
	if err != nil {
		return nil, err
	}

	soft, err := parseInvariants(anchors, model.Soft, "soft")
	if err != nil {
		return nil, err
	}

	encoding := &encoding{
		anchors:        anchors,
		guards:         map[string]*Expr{},
		invariants:     invariants,
		ownConjuncts:   map[string][]*Expr{},
		routeConjuncts: map[string][]*Expr{},
		soft:           soft,
	}
	sections := map[string]*Section{}
	for index := range model.Sections {
		sections[model.Sections[index].Id] = &model.Sections[index]
	}
	for index := range model.Transitions {
		transition := &model.Transitions[index]
		pred, err := parseClause(transition.Pred)
		if err != nil {
			return nil, fmt.Errorf("newEncoding: pred %s: %w", transition.Id, err)
		}

		section, ok := sections[transition.Section]
		if !ok {
			return nil, fmt.Errorf("newEncoding: transition %s: Unknown section %q", transition.Id, transition.Section)
		}

		encoding.guards[transition.Id] = anchors.ReplacedGuard(pred, transition.Id)
		encoding.splitConjuncts(pred, anchors.Routes[section.Id], section, transition)
	}
	return encoding, nil
}

// splitConjuncts relies on Derive conjoining route and entry preds as "(R) and (E)", parsing to And(R, E).
func (e *encoding) splitConjuncts(pred *Expr, routePred *Expr, section *Section, transition *Transition) {
	if routePred == nil {
		e.ownConjuncts[transition.Id] = conjuncts(pred)
		return
	}

	e.routeConjuncts[transition.Id] = conjuncts(routePred)
	if transition.Pred != section.Pred {
		e.ownConjuncts[transition.Id] = conjuncts(pred.Right)
	}
}

// namedExpr is one effective clause with the entry id it stands for.
type namedExpr struct {
	expr *Expr
	id   string
}

// andExpr builds the conjunction node of two expressions.
func andExpr(left, right *Expr) *Expr {
	return &Expr{Kind: ExprAnd, Left: left, Right: right}
}

func conjuncts(expr *Expr) []*Expr {
	if expr == nil {
		return nil
	}

	if expr.Kind != ExprAnd {
		return []*Expr{expr}
	}

	left := conjuncts(expr.Left)
	right := conjuncts(expr.Right)
	return append(left, right...)
}

func entrySections(model *Model) map[string]string {
	owners := map[string]string{}
	for _, transition := range model.Transitions {
		owners[transition.Id] = transition.Section
	}
	for _, invariant := range slices.Concat(model.Invariants, model.Soft) {
		owners[invariant.Id] = invariant.Section
	}
	return owners
}

// impliesExpr builds the implication node antecedent ⇒ consequent.
func impliesExpr(antecedent, consequent *Expr) *Expr {
	return &Expr{Kind: ExprImplies, Left: antecedent, Right: consequent}
}

// notExpr builds the negation node of an expression.
func notExpr(operand *Expr) *Expr {
	return &Expr{Kind: ExprNot, Left: operand}
}

// parseClause parses an optional clause; blank text is no clause.
func parseClause(text string) (*Expr, error) {
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}

	return ParseExpr(text)
}

func parseInvariants(anchors *Anchors, clauses []Invariant, kind string) ([]namedExpr, error) {
	parsed := make([]namedExpr, 0, len(clauses))
	for _, clause := range clauses {
		expr, err := ParseExpr(clause.Expr)
		if err != nil {
			return nil, fmt.Errorf("parseInvariants: %s %s: %w", kind, clause.Id, err)
		}

		named := namedExpr{expr: anchors.ReplacedClause(expr, clause.Id), id: clause.Id}
		parsed = append(parsed, named)
	}
	return parsed, nil
}

// phaseRanks numbers the phase sections in document order.
func phaseRanks(sections []Section) map[string]int {
	ranks := map[string]int{}
	for _, section := range sections {
		isPhase := section.Kind == skillgraph.SectionKindPhase || section.Kind == ""
		if isPhase {
			ranks[section.Id] = len(ranks)
		}
	}
	return ranks
}

// routeWindow leaves an open side at 0 or maxRank for start, end and anchors outside every phase.
func routeWindow(maxRank int, owners map[string]string, ranks map[string]int, section *Section) RankWindow {
	if len(section.Between) == betweenPairLength {
		window := RankWindow{Hi: maxRank, Lo: 0}
		if rank, isPhase := ranks[owners[section.Between[0]]]; isPhase {
			window.Lo = rank
		}
		if rank, isPhase := ranks[owners[section.Between[1]]]; isPhase {
			window.Hi = rank
		}
		return window
	}

	var replacedRanks []int
	for _, id := range section.Replaces {
		if rank, isPhase := ranks[owners[id]]; isPhase {
			replacedRanks = append(replacedRanks, rank)
		}
	}
	if len(replacedRanks) == 0 {
		return RankWindow{Hi: maxRank, Lo: 0}
	}

	return RankWindow{Hi: slices.Max(replacedRanks), Lo: slices.Min(replacedRanks)}
}

func sectionWindows(model *Model) map[string]RankWindow {
	ranks := phaseRanks(model.Sections)
	maxRank := max(len(ranks)-1, 0)
	owners := entrySections(model)
	windows := map[string]RankWindow{}
	for index := range model.Sections {
		section := &model.Sections[index]
		switch section.Kind {
		case skillgraph.SectionKindPhase, "":
			rank := ranks[section.Id]
			windows[section.Id] = RankWindow{Hi: rank, Lo: rank}
		case skillgraph.SectionKindRoute:
			windows[section.Id] = routeWindow(maxRank, owners, ranks, section)
		case skillgraph.SectionKindContext:
		}
	}
	return windows
}

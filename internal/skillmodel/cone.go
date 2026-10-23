package skillmodel

import (
	"fmt"
	"slices"
)

// completeBound ignores granted atoms outside the cone: no guard or invariant reads them.
func (e *encoding) completeBound(cone map[string]bool, model *Model) int {
	isOutsideCone := func(atom string) bool {
		return !cone[atom]
	}
	granted := map[string]bool{}
	granting := 0
	for _, transition := range model.Transitions {
		conePost := slices.Clone(transition.Post)
		conePost = slices.DeleteFunc(conePost, isOutsideCone)
		if len(conePost) == 0 {
			continue
		}

		granting++
		for _, atom := range conePost {
			granted[atom] = true
		}
	}
	return min(granting, len(granted)) + 1
}

// cone keeps soft clauses out: reachability never asserts them.
func (e *encoding) cone(extra []*Expr, vocab *Vocabulary) map[string]bool {
	info := vocab.Info()
	cone := map[string]bool{}
	for _, guard := range e.guards {
		addAtoms(cone, guard, info)
	}
	for _, invariant := range e.invariants {
		addAtoms(cone, invariant.expr, info)
	}
	for _, expr := range extra {
		addAtoms(cone, expr, info)
	}
	isGrowing := true
	for isGrowing {
		isGrowing = addTouchedGroups(cone, vocab.Exclusive)
	}
	return cone
}

func addAtoms(cone map[string]bool, expr *Expr, info map[string]Atom) {
	for _, ident := range expr.Atoms() {
		if _, isAtom := info[ident]; isAtom {
			cone[ident] = true
		}
	}
}

func addTouchedGroups(cone map[string]bool, groups [][]string) bool {
	isInCone := func(atom string) bool {
		return cone[atom]
	}
	isGrown := false
	for _, group := range groups {
		if !slices.ContainsFunc(group, isInCone) {
			continue
		}

		for _, atom := range group {
			isGrown = isGrown || !cone[atom]
			cone[atom] = true
		}
	}
	return isGrown
}

// CompleteBound is min(P, G) + 1: G granted cone atoms, P transitions granting one; reachability is complete there.
func CompleteBound(extra []*Expr, model *Model) (int, error) {
	encoding, err := newEncoding(model)
	if err != nil {
		return 0, fmt.Errorf("CompleteBound: %w", err)
	}

	cone := encoding.cone(extra, model.Vocabulary)
	return encoding.completeBound(cone, model), nil
}

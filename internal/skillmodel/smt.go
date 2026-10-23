package skillmodel

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

// Query is the committed query file; skillgraph owns the type so the reader index can count query reads.
type Query = skillgraph.Query

const checkTimeoutMs = 10000

// SmtDoc's Checks counts its check-sat commands, one verdict line each.
type SmtDoc struct {
	Checks int
	Text   string
}

// EmitConsistency's unsat core names the conflicting invariants and exclusivity pairs.
func EmitConsistency(model *Model) (SmtDoc, error) {
	encoding, err := newEncoding(model)
	if err != nil {
		return SmtDoc{}, err
	}

	builder := &strings.Builder{}
	emitOptions(builder)
	emitAxioms(builder, encoding, model)
	builder.WriteString("(check-sat)\n(get-unsat-core)\n")
	doc := SmtDoc{Checks: 1, Text: builder.String()}
	return doc, nil
}

func EmitQuery(model *Model, query Query) (SmtDoc, error) {
	encoding, err := newEncoding(model)
	if err != nil {
		return SmtDoc{}, err
	}

	builder := &strings.Builder{}
	emitOptions(builder)
	emitAxioms(builder, encoding, model)
	render := newEmitter(model.Vocabulary)
	for _, soft := range encoding.soft {
		rendered := render.emitExpr(soft.expr)
		fmt.Fprintf(builder, "(assert-soft %s)\n", rendered)
	}
	name := smtName(query.Name)
	for index, assume := range query.Assume {
		expr, err := ParseExpr(assume)
		if err != nil {
			return SmtDoc{}, fmt.Errorf("EmitQuery: assume[%d]: %w", index, err)
		}

		rendered := render.emitExpr(expr)
		fmt.Fprintf(
			builder,
			"(assert (! %s :named query.%s.assume%d))\n",
			rendered,
			name,
			index,
		)
	}
	check, err := ParseExpr(query.Check)
	if err != nil {
		return SmtDoc{}, fmt.Errorf("EmitQuery: check: %w", err)
	}

	rendered := render.emitExpr(check)
	fmt.Fprintf(builder, "(assert (! %s :named query.%s.check))\n", rendered, name)
	builder.WriteString("(check-sat)\n(get-unsat-core)\n(get-model)\n")
	doc := SmtDoc{Checks: 1, Text: builder.String()}
	return doc, nil
}

// EmitReachability's bound is complete: posts only grant, so at most min(P, G) cone-growing firings precede a target.
func EmitReachability(model *Model, transitionId string, bound int, assume []string) (SmtDoc, error) {
	target := -1
	for index, transition := range model.Transitions {
		if transition.Id == transitionId {
			target = index
		}
	}
	if target < 0 {
		return SmtDoc{}, fmt.Errorf("EmitReachability: unknown transition %s", transitionId)
	}

	encoding, err := newEncoding(model)
	if err != nil {
		return SmtDoc{}, err
	}

	assumeExprs := make([]*Expr, 0, len(assume))
	for index, input := range assume {
		expr, err := ParseExpr(input)
		if err != nil {
			return SmtDoc{}, fmt.Errorf("EmitReachability: assume[%d]: %w", index, err)
		}

		assumeExprs = append(assumeExprs, expr)
	}
	cone := encoding.cone(assumeExprs, model.Vocabulary)
	builder := &strings.Builder{}
	emitOptions(builder)
	steps := emitUnrolling(
		bound,
		builder,
		cone,
		encoding,
		model,
	)
	for index, expr := range assumeExprs {
		rendered := steps[0].emitExpr(expr)
		fmt.Fprintf(builder, "(assert (! %s :named reach.assume%d))\n", rendered, index)
	}
	disjuncts := make([]string, 0, bound+1)
	for step := 0; step <= bound; step++ {
		disjuncts = append(disjuncts, occurrenceVar(target, step))
	}
	goal := strings.Join(disjuncts, " ")
	name := smtName(transitionId)
	fmt.Fprintf(builder, "(assert (! (or %s) :named reach.%s))\n", goal, name)
	builder.WriteString("(check-sat)\n(get-unsat-core)\n")
	doc := SmtDoc{Checks: 1, Text: builder.String()}
	return doc, nil
}

// emitter's rename maps bool atoms to their step copies for the reachability unrolling.
type emitter struct {
	info   map[string]Atom
	owners map[string]string
	rename func(string) string
}

func newEmitter(vocab *Vocabulary) *emitter {
	return &emitter{info: vocab.Info(), owners: vocab.MemberOwner()}
}

// cmpSymbols omits CmpNe: it renders as a negated equality.
var cmpSymbols = map[CmpOp]string{
	CmpEq: "=",
	CmpGe: ">=",
	CmpGt: ">",
	CmpLe: "<=",
	CmpLt: "<",
}

// emitCmp orders enums through the sort's rank function.
func (e *emitter) emitCmp(cmp *Cmp) string {
	sortId := e.enumSort(cmp.Left)
	if sortId == "" {
		sortId = e.enumSort(cmp.Right)
	}
	left := e.emitTerm(cmp.Left)
	right := e.emitTerm(cmp.Right)
	isEnumOrder := sortId != "" && cmp.Op != CmpEq && cmp.Op != CmpNe
	if isEnumOrder {
		left = "(" + sortId + "_rank " + left + ")"
		right = "(" + sortId + "_rank " + right + ")"
	}
	if cmp.Op == CmpNe {
		return "(not (= " + left + " " + right + "))"
	}
	return "(" + cmpSymbols[cmp.Op] + " " + left + " " + right + ")"
}

// emitExpr renders an expression as an SMT-LIB2 s-expression.
func (e *emitter) emitExpr(expression *Expr) string {
	switch expression.Kind {
	case ExprAtom:
		if e.rename != nil {
			return e.rename(expression.Atom)
		}
		return expression.Atom
	case ExprNot:
		return "(not " + e.emitExpr(expression.Left) + ")"
	case ExprAnd:
		return "(and " + e.emitExpr(expression.Left) + " " + e.emitExpr(expression.Right) + ")"
	case ExprOr:
		return "(or " + e.emitExpr(expression.Left) + " " + e.emitExpr(expression.Right) + ")"
	case ExprImplies:
		return "(=> " + e.emitExpr(expression.Left) + " " + e.emitExpr(expression.Right) + ")"
	case ExprCmp:
		return e.emitCmp(expression.Cmp)
	}
	return "false"
}

// emitTerm renders one comparison side.
func (e *emitter) emitTerm(term *Term) string {
	switch term.Kind {
	case TermIdent:
		return term.Ident
	case TermLit:
		return strconv.Itoa(term.Lit)
	case TermAdd:
		return "(+ " + e.emitTerm(term.Left) + " " + e.emitTerm(term.Right) + ")"
	case TermSub:
		return "(- " + e.emitTerm(term.Left) + " " + e.emitTerm(term.Right) + ")"
	}
	return "0"
}

// enumSort returns the enum atom id governing a lone-identifier side, or "".
func (e *emitter) enumSort(term *Term) string {
	if term.Kind != TermIdent {
		return ""
	}
	if atom, ok := e.info[term.Ident]; ok && atom.EffectiveKind() == skillgraph.KindAtomEnum {
		return atom.Id
	}
	if owner, ok := e.owners[term.Ident]; ok {
		return owner
	}
	return ""
}

func emitDeclarations(builder *strings.Builder, vocab *Vocabulary) {
	info := vocab.Info()
	atomIds := maps.Keys(info)
	for _, id := range slices.Sorted(atomIds) {
		atom := info[id]
		switch atom.EffectiveKind() {
		case skillgraph.KindAtomBool:
			fmt.Fprintf(builder, "(declare-const %s Bool)\n", id)
		case skillgraph.KindAtomEnum, skillgraph.KindAtomInt:
			emitRigidDeclaration(atom, builder)
		}
	}
}

func conjoinAtoms(atoms []string, scope map[string]bool) *Expr {
	var conjunction *Expr
	for _, atom := range atoms {
		if !scope[atom] {
			continue
		}

		operand := &Expr{Atom: atom, Kind: ExprAtom}
		if conjunction == nil {
			conjunction = operand
			continue
		}

		conjunction = andExpr(conjunction, operand)
	}
	return conjunction
}

func emitAxioms(builder *strings.Builder, encoding *encoding, model *Model) {
	emitDeclarations(builder, model.Vocabulary)
	render := newEmitter(model.Vocabulary)
	for _, axiom := range exclusivityAxioms(model.Vocabulary) {
		rendered := render.emitExpr(axiom.expr)
		fmt.Fprintf(builder, "(assert (! %s :named %s))\n", rendered, axiom.id)
	}
	for _, invariant := range encoding.invariants {
		rendered := render.emitExpr(invariant.expr)
		name := smtName(invariant.id)
		fmt.Fprintf(builder, "(assert (! %s :named %s))\n", rendered, name)
	}
}

// emitOptions sets a per-check timeout after which z3 answers unknown.
func emitOptions(builder *strings.Builder) {
	builder.WriteString("(set-option :produce-unsat-cores true)\n")
	fmt.Fprintf(builder, "(set-option :timeout %d)\n", checkTimeoutMs)
}

// emitRigidDeclaration: int and enum atoms are rigid for a run; enums rank by declaration order.
func emitRigidDeclaration(atom Atom, builder *strings.Builder) {
	if atom.EffectiveKind() == skillgraph.KindAtomInt {
		fmt.Fprintf(builder, "(declare-const %s Int)\n", atom.Id)
		return
	}

	constructors := "(" + strings.Join(atom.Members, ") (") + ")"
	rank := rankBody(atom.Members, 0)
	fmt.Fprintf(builder, "(declare-datatypes ((%s_sort 0)) ((%s)))\n", atom.Id, constructors)
	fmt.Fprintf(builder, "(declare-const %s %s_sort)\n", atom.Id, atom.Id)
	fmt.Fprintf(
		builder,
		"(define-fun %s_rank ((x %s_sort)) Int %s)\n",
		atom.Id,
		atom.Id,
		rank,
	)
}

// emitUnrolling lands a last-step post at state bound+1, still under the invariants.
func emitUnrolling(bound int, builder *strings.Builder, cone map[string]bool, encoding *encoding, model *Model) []*emitter {
	lastState := bound + 1
	vocab := model.Vocabulary
	info := vocab.Info()
	atomIds := maps.Keys(info)
	stepped := map[string]bool{}
	for _, id := range slices.Sorted(atomIds) {
		atom := info[id]
		switch atom.EffectiveKind() {
		case skillgraph.KindAtomBool:
			if !cone[id] {
				continue
			}

			stepped[id] = true
			for state := 0; state <= lastState; state++ {
				copyName := stepAtom(id, state)
				fmt.Fprintf(builder, "(declare-const %s Bool)\n", copyName)
			}
		case skillgraph.KindAtomEnum, skillgraph.KindAtomInt:
			emitRigidDeclaration(atom, builder)
		}
	}
	for step := 0; step <= bound; step++ {
		for index := range model.Transitions {
			occurrence := occurrenceVar(index, step)
			fmt.Fprintf(builder, "(declare-const %s Bool)\n", occurrence)
		}
		selector := selectVar(step)
		phase := phaseVar(step)
		fmt.Fprintf(builder, "(declare-const %s Int)\n(declare-const %s Int)\n", selector, phase)
	}
	steps := make([]*emitter, 0, lastState+1)
	for state := 0; state <= lastState; state++ {
		halted := haltedVar(state)
		fmt.Fprintf(builder, "(declare-const %s Bool)\n", halted)
		render := newEmitter(vocab)
		render.rename = stepRenamer(stepped, state)
		steps = append(steps, render)
	}
	var exclusive []namedExpr
	for _, axiom := range exclusivityAxioms(vocab) {
		atoms := axiom.expr.Atoms()
		isInCone := stepped[atoms[0]] && stepped[atoms[1]]
		if isInCone {
			exclusive = append(exclusive, axiom)
		}
	}
	for state := 0; state <= lastState; state++ {
		for _, axiom := range exclusive {
			rendered := steps[state].emitExpr(axiom.expr)
			fmt.Fprintf(
				builder,
				"(assert (! %s :named %s.s%d))\n",
				rendered,
				axiom.id,
				state,
			)
		}
		for _, invariant := range encoding.invariants {
			rendered := steps[state].emitExpr(invariant.expr)
			name := smtName(invariant.id)
			fmt.Fprintf(
				builder,
				"(assert (! %s :named %s.s%d))\n",
				rendered,
				name,
				state,
			)
		}
	}
	grantors := map[string][]int{}
	for index, transition := range model.Transitions {
		for _, atom := range transition.Post {
			grantors[atom] = append(grantors[atom], index)
		}
	}
	steppedIds := maps.Keys(stepped)
	for _, atom := range slices.Sorted(steppedIds) {
		initial := stepAtom(atom, 0)
		granting, isGranted := grantors[atom]
		if !isGranted {
			for state := 1; state <= lastState; state++ {
				current := stepAtom(atom, state)
				fmt.Fprintf(
					builder,
					"(assert (! (= %s %s) :named rigid.%s.s%d))\n",
					current,
					initial,
					atom,
					state,
				)
			}
			continue
		}

		fmt.Fprintf(builder, "(assert (! (not %s) :named init.%s))\n", initial, atom)
		for step := range lastState {
			current := stepAtom(atom, step)
			next := stepAtom(atom, step+1)
			causes := []string{current}
			for _, index := range granting {
				causes = append(causes, occurrenceVar(index, step))
			}
			cause := strings.Join(causes, " ")
			fmt.Fprintf(
				builder,
				"(assert (! (=> %s %s) :named frame.%s.s%d))\n",
				current,
				next,
				atom,
				step,
			)
			fmt.Fprintf(
				builder,
				"(assert (! (=> %s (or %s)) :named cause.%s.s%d))\n",
				next,
				cause,
				atom,
				step,
			)
		}
	}
	var nonStops, stops []int
	for index, transition := range model.Transitions {
		guard := encoding.guards[transition.Id]
		effect := conjoinAtoms(transition.Post, stepped)
		name := smtName(transition.Id)
		window, isRanked := encoding.anchors.Windows[transition.Id]
		for step := 0; step <= bound; step++ {
			occurrence := occurrenceVar(index, step)
			phase := phaseVar(step)
			if guard != nil {
				rendered := steps[step].emitExpr(guard)
				fmt.Fprintf(builder, "(assert (=> %s %s))\n", occurrence, rendered)
			}
			if effect != nil {
				rendered := steps[step+1].emitExpr(effect)
				fmt.Fprintf(
					builder,
					"(assert (! (=> %s %s) :named post.%s.s%d))\n",
					occurrence,
					rendered,
					name,
					step,
				)
			}
			selector := selectVar(step)
			halted := haltedVar(step)
			fmt.Fprintf(
				builder,
				"(assert (=> %s (= %s %d)))\n",
				occurrence,
				selector,
				index,
			)
			fmt.Fprintf(builder, "(assert (=> %s (not %s)))\n", occurrence, halted)
			if isRanked {
				fmt.Fprintf(
					builder,
					"(assert (=> %s (and (<= %d %s) (<= %s %d))))\n",
					occurrence,
					window.Lo,
					phase,
					phase,
					window.Hi,
				)
			}
		}
		if transition.Kind == KindStop {
			stops = append(stops, index)
			continue
		}

		nonStops = append(nonStops, index)
	}
	for step := range bound {
		phase := phaseVar(step)
		next := phaseVar(step + 1)
		fmt.Fprintf(builder, "(assert (<= %s %s))\n", phase, next)
	}
	initialHalted := haltedVar(0)
	fmt.Fprintf(builder, "(assert (not %s))\n", initialHalted)
	for step := 0; step <= bound; step++ {
		terms := []string{haltedVar(step)}
		for _, stop := range stops {
			terms = append(terms, occurrenceVar(stop, step))
		}
		next := haltedVar(step + 1)
		reasons := strings.Join(terms, " ")
		fmt.Fprintf(
			builder,
			"(assert (! (= %s (or %s)) :named halt.s%d))\n",
			next,
			reasons,
			step,
		)
	}
	for _, stop := range stops {
		stopId := model.Transitions[stop].Id
		guard := encoding.guards[stopId]
		if guard == nil || len(nonStops) == 0 {
			continue
		}

		name := smtName(stopId)
		for step := 0; step <= bound; step++ {
			blocked := make([]string, 0, len(nonStops))
			for _, other := range nonStops {
				negation := "(not " + occurrenceVar(other, step) + ")"
				blocked = append(blocked, negation)
			}
			rendered := steps[step].emitExpr(guard)
			conjunction := strings.Join(blocked, " ")
			fmt.Fprintf(
				builder,
				"(assert (! (=> %s (and %s)) :named stopblock.%s.s%d))\n",
				rendered,
				conjunction,
				name,
				step,
			)
		}
	}
	return steps
}

func exclusivityAxioms(vocab *Vocabulary) []namedExpr {
	var axioms []namedExpr
	for groupIndex, group := range vocab.Exclusive {
		for left := range len(group) {
			for right := left + 1; right < len(group); right++ {
				leftAtom := &Expr{Atom: group[left], Kind: ExprAtom}
				rightAtom := &Expr{Atom: group[right], Kind: ExprAtom}
				both := andExpr(leftAtom, rightAtom)
				name := fmt.Sprintf("vocab.exclusive%d.%s.%s", groupIndex, group[left], group[right])
				axiom := namedExpr{expr: notExpr(both), id: name}
				axioms = append(axioms, axiom)
			}
		}
	}
	return axioms
}

func rankBody(members []string, index int) string {
	if index == len(members)-1 {
		return strconv.Itoa(index)
	}
	return fmt.Sprintf("(ite ((_ is %s) x) %d %s)", members[index], index, rankBody(members, index+1))
}

// stepAtom names the step-k copy of a bool atom.
func stepAtom(atom string, step int) string {
	return fmt.Sprintf("%s__s%d", atom, step)
}

func stepRenamer(boolAtoms map[string]bool, step int) func(string) string {
	return func(atom string) string {
		if boolAtoms[atom] {
			return stepAtom(atom, step)
		}
		return atom
	}
}

// occurrenceVar names the Boolean "transition index fires at step".
func occurrenceVar(index, step int) string {
	return fmt.Sprintf("t%d_s%d", index, step)
}

// haltedVar names the Boolean "a stop fired before state".
func haltedVar(state int) string {
	return fmt.Sprintf("halted_s%d", state)
}

// phaseVar names the Int phase rank the run is in at step.
func phaseVar(step int) string {
	return fmt.Sprintf("ph_s%d", step)
}

// selectVar names the Int index of the transition firing at step.
func selectVar(step int) string {
	return fmt.Sprintf("sel_s%d", step)
}

// smtName makes an identifier safe for a :named attribute.
func smtName(id string) string {
	return strings.NewReplacer("-", "_", " ", "_", ".", "_").Replace(id)
}

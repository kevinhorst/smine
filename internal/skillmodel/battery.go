package skillmodel

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

// Battery check names; checkRunOrder fixes their run order.
const (
	CheckCompletion     = "completion"
	CheckConsistency    = "consistency"
	CheckGuard          = "guard"
	CheckHalt           = "halt"
	CheckPreservation   = "preservation"
	CheckReach          = "reach"
	CheckRedundantGuard = "redundant-guard"
	CheckValidate       = "validate"
)

// Finding severities: an error refuses a save, a warning only reports.
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
)

const checkTimeout = checkTimeoutMs * time.Millisecond

var checkRunOrder = []string{
	CheckValidate,
	CheckConsistency,
	CheckGuard,
	CheckRedundantGuard,
	CheckHalt,
	CheckPreservation,
	CheckReach,
	CheckCompletion,
}

// battery's feasibility and guards are the static obligations the reach family filters on.
type battery struct {
	bound       int
	conditions  []*Expr
	cone        map[string]bool
	encoding    *encoding
	feasibility map[string][]*obligation
	guards      map[string]*obligation
	model       *Model
	render      *emitter
}

// newBattery's conditions name only cone atoms, so the cone over them equals the cone without them.
func newBattery(model *Model) (*battery, error) {
	encoding, err := newEncoding(model)
	if err != nil {
		return nil, fmt.Errorf("newBattery: %w", err)
	}

	cone := encoding.cone(nil, model.Vocabulary)
	conditions, err := sourcedConditions(cone, model.Vocabulary)
	if err != nil {
		return nil, fmt.Errorf("newBattery: %w", err)
	}

	run := &battery{
		bound:       encoding.completeBound(cone, model),
		conditions:  conditions,
		cone:        cone,
		encoding:    encoding,
		feasibility: map[string][]*obligation{},
		guards:      map[string]*obligation{},
		model:       model,
		render:      newEmitter(model.Vocabulary),
	}
	return run, nil
}

// completionObligations: phase ranks follow document order, so the final phase is the last holding transitions.
func (b *battery) completionObligations() []*obligation {
	for sectionIndex := len(b.model.Sections) - 1; sectionIndex >= 0; sectionIndex-- {
		section := &b.model.Sections[sectionIndex]
		indexes := b.phaseTransitions(section)
		if len(indexes) == 0 {
			continue
		}

		subject := "final phase " + section.Title
		completion := &obligation{
			check:          CheckCompletion,
			findingVerdict: VerdictUnsat,
			formula:        b.firingDisjunction(indexes...),
			message:        "no run reaches the " + subject,
			subject:        subject,
		}
		return []*obligation{completion}
	}
	return nil
}

// feasibilityObligations are filter-only: a guard doing its job under a condition skips that reach check.
func (b *battery) feasibilityObligations(transition *Transition) []*obligation {
	guard := b.guardFormulas(transition.Id)
	obligations := make([]*obligation, 0, len(b.conditions))
	for _, condition := range b.conditions {
		conditionFormula := []string{b.render.emitExpr(condition)}
		premise := slices.Concat(guard, conditionFormula)
		feasible := &obligation{formula: smtConjunction(premise)}
		obligations = append(obligations, feasible)
	}
	b.feasibility[transition.Id] = obligations
	return obligations
}

func (b *battery) firingDisjunction(indexes ...int) string {
	var firings []string
	for _, index := range indexes {
		for step := 0; step <= b.bound; step++ {
			firings = append(firings, occurrenceVar(index, step))
		}
	}
	return "(or " + strings.Join(firings, " ") + ")"
}

func (b *battery) guardFormulas(transitionId string) []string {
	guard := b.encoding.guards[transitionId]
	if guard == nil {
		return nil
	}

	return []string{b.render.emitExpr(guard)}
}

// guardObligations records each guard for the checks that skip guard-dead entries.
func (b *battery) guardObligations(transition *Transition) []*obligation {
	guard := b.guardFormulas(transition.Id)
	if len(guard) == 0 {
		return nil
	}

	subject := "entry " + transition.Id
	feasible := &obligation{
		check:          CheckGuard,
		entryIds:       []string{transition.Id},
		findingVerdict: VerdictUnsat,
		formula:        guard[0],
		message:        subject + ": guard can never hold — the entry is dead",
		subject:        subject,
	}
	b.guards[transition.Id] = feasible
	return []*obligation{feasible}
}

// haltObligations: a stop whose guard cannot fail halts every run.
func (b *battery) haltObligations(transition *Transition) []*obligation {
	guard := b.guardFormulas(transition.Id)
	isGuardedStop := len(guard) > 0 && transition.Kind == KindStop
	if !isGuardedStop {
		return nil
	}

	subject := "stop " + transition.Id
	halt := &obligation{
		check:          CheckHalt,
		entryIds:       []string{transition.Id},
		findingVerdict: VerdictUnsat,
		formula:        smtNegation(guard[0]),
		message:        subject + " halts every run: its guard always holds",
		subject:        subject,
	}
	return []*obligation{halt}
}

func (b *battery) isDead(transitionId string) bool {
	guard, isGuarded := b.guards[transitionId]
	return isGuarded && guard.verdict == VerdictUnsat
}

// phaseTransitions counts an untyped section as a phase, as phaseRanks does.
func (b *battery) phaseTransitions(section *Section) []int {
	isPhase := section.Kind == skillgraph.SectionKindPhase || section.Kind == ""
	if !isPhase {
		return nil
	}

	var indexes []int
	for index, transition := range b.model.Transitions {
		if transition.Section == section.Id {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

// preservationObligations: posts only grant, so substituting true for the post atoms is the exact successor.
func (b *battery) preservationObligations(requires []*obligation, transition *Transition) []*obligation {
	if len(transition.Post) == 0 {
		return nil
	}

	successor := newEmitter(b.model.Vocabulary)
	successor.rename = postRenamer(transition.Post)
	guard := b.guardFormulas(transition.Id)
	var obligations []*obligation
	for _, invariant := range b.encoding.invariants {
		atoms := invariant.expr.Atoms()
		hasPostAtom := slices.ContainsFunc(atoms, func(atom string) bool {
			return slices.Contains(transition.Post, atom)
		})
		if !hasPostAtom {
			continue
		}

		successorInvariant := successor.emitExpr(invariant.expr)
		broken := []string{smtNegation(successorInvariant)}
		premise := slices.Concat(guard, broken)
		preservation := &obligation{
			check:          CheckPreservation,
			entryIds:       []string{transition.Id, invariant.id},
			findingVerdict: VerdictSat,
			formula:        smtConjunction(premise),
			message:        fmt.Sprintf("firing %s can break invariant %s", transition.Id, invariant.id),
			requires:       requires,
			subject:        fmt.Sprintf("entry %s, invariant %s", transition.Id, invariant.id),
		}
		obligations = append(obligations, preservation)
	}
	return obligations
}

// reachObligations skips guard-dead transitions.
func (b *battery) reachObligations() []*obligation {
	var obligations []*obligation
	for index := range b.model.Transitions {
		transition := &b.model.Transitions[index]
		if b.isDead(transition.Id) {
			continue
		}

		reach := b.transitionReach(index, transition)
		obligations = append(obligations, reach...)
	}
	completion := b.completionObligations()
	return append(obligations, completion...)
}

func (b *battery) reachPrelude() string {
	builder := &strings.Builder{}
	emitOptions(builder)
	emitUnrolling(
		b.bound,
		builder,
		b.cone,
		b.encoding,
		b.model,
	)
	return builder.String()
}

// redundancyObligations reports a lone own conjunct outside a route as the whole guard always holding.
func (b *battery) redundancyObligations(requires []*obligation, transition *Transition) []*obligation {
	own := b.encoding.ownConjuncts[transition.Id]
	renderedOwn := b.renderAll(own)
	renderedRoute := b.renderAll(b.encoding.routeConjuncts[transition.Id])
	isWholeGuard := len(own) == 1 && len(renderedRoute) == 0
	obligations := make([]*obligation, 0, len(own))
	for index, conjunct := range own {
		conjunctText := exprText(conjunct)
		message := fmt.Sprintf("entry %s: guard conjunct %q is redundant — the invariants and the rest of the guard imply it", transition.Id, conjunctText)
		if isWholeGuard {
			message = fmt.Sprintf("entry %s: guard always holds — the invariants imply it", transition.Id)
		}

		negation := []string{smtNegation(renderedOwn[index])}
		premise := slices.Concat(renderedRoute, renderedOwn[:index], renderedOwn[index+1:], negation)
		redundancy := &obligation{
			check:          CheckRedundantGuard,
			entryIds:       []string{transition.Id},
			findingVerdict: VerdictUnsat,
			formula:        smtConjunction(premise),
			message:        message,
			requires:       requires,
			subject:        fmt.Sprintf("entry %s guard conjunct %q", transition.Id, conjunctText),
		}
		obligations = append(obligations, redundancy)
	}
	return obligations
}

// renderAll renders expressions as single-state SMT formulas.
func (b *battery) renderAll(exprs []*Expr) []string {
	formulas := make([]string, 0, len(exprs))
	for _, expr := range exprs {
		formulas = append(formulas, b.render.emitExpr(expr))
	}
	return formulas
}

// staticObligations: redundancy requires a live guard and, for a stop, one that can fail.
func (b *battery) staticObligations() []*obligation {
	var obligations []*obligation
	for index := range b.model.Transitions {
		transition := &b.model.Transitions[index]
		guard := b.guardObligations(transition)
		halt := b.haltObligations(transition)
		redundancyRequires := slices.Concat(guard, halt)
		obligations = slices.Concat(
			obligations,
			guard,
			halt,
			b.redundancyObligations(redundancyRequires, transition),
			b.preservationObligations(guard, transition),
			b.feasibilityObligations(transition),
		)
	}
	return obligations
}

func (b *battery) staticPrelude() string {
	builder := &strings.Builder{}
	emitOptions(builder)
	emitAxioms(builder, b.encoding, b.model)
	return builder.String()
}

// transitionReach requires the unconditioned run, so a conditioned finding reports only deadness the condition causes.
func (b *battery) transitionReach(index int, transition *Transition) []*obligation {
	firing := b.firingDisjunction(index)
	subject := "entry " + transition.Id
	base := &obligation{
		check:          CheckReach,
		entryIds:       []string{transition.Id},
		findingVerdict: VerdictUnsat,
		formula:        firing,
		message:        fmt.Sprintf("%s is unreachable within the complete bound %d", subject, b.bound),
		subject:        subject,
	}
	obligations := []*obligation{base}
	requires := []*obligation{base}
	for conditionIndex, condition := range b.conditions {
		if b.feasibility[transition.Id][conditionIndex].verdict == VerdictUnsat {
			continue
		}

		conditionText := exprText(condition)
		premise := []string{firing, b.render.emitExpr(condition)}
		conditioned := &obligation{
			check:          CheckReach,
			entryIds:       []string{transition.Id},
			findingVerdict: VerdictUnsat,
			formula:        smtConjunction(premise),
			message:        subject + " is unreachable under " + conditionText,
			requires:       requires,
			subject:        subject + " under " + conditionText,
		}
		obligations = append(obligations, conditioned)
	}
	return obligations
}

// warnings runs the reach family after the static one, whose verdicts name the guard-dead transitions it skips.
func (b *battery) warnings(ctx context.Context) ([]Finding, error) {
	static := b.staticObligations()
	staticPrelude := b.staticPrelude()
	if err := solveObligations(ctx, static, staticPrelude); err != nil {
		return nil, fmt.Errorf("battery.warnings: Failed to solve the static family: %w", err)
	}

	reach := b.reachObligations()
	reachPrelude := b.reachPrelude()
	if err := solveObligations(ctx, reach, reachPrelude); err != nil {
		return nil, fmt.Errorf("battery.warnings: Failed to solve the reach family: %w", err)
	}

	var findings []Finding
	for _, current := range slices.Concat(static, reach) {
		if finding := current.finding(); finding != nil {
			findings = append(findings, *finding)
		}
	}
	return findings, nil
}

// Finding's EntryIds are in model order.
type Finding struct {
	Check    string   `json:"check"`
	EntryIds []string `json:"entryIds"`
	Message  string   `json:"message"`
	Severity string   `json:"severity"`
}

// obligation reports only when every required obligation came back sat; an empty findingVerdict is filter-only.
type obligation struct {
	check          string
	entryIds       []string
	findingVerdict string
	formula        string
	message        string
	requires       []*obligation
	subject        string
	verdict        string
}

// finding reports unknown as the check undecided.
func (o *obligation) finding() *Finding {
	if o.findingVerdict == "" {
		return nil
	}

	for _, required := range o.requires {
		if required.verdict != VerdictSat {
			return nil
		}
	}

	switch o.verdict {
	case VerdictSat, VerdictUnsat:
		if o.verdict != o.findingVerdict {
			return nil
		}

		return &Finding{Check: o.check, EntryIds: o.entryIds, Message: o.message, Severity: SeverityWarning}
	case VerdictUnknown:
		message := fmt.Sprintf("%s undecided within the %v check timeout: %s", o.check, checkTimeout, o.subject)
		return &Finding{Check: o.check, EntryIds: o.entryIds, Message: message, Severity: SeverityWarning}
	}
	return nil
}

// cmpText renders a comparison operator in the expression DSL.
func cmpText(operator CmpOp) string {
	for token, candidate := range cmpOps {
		if candidate == operator {
			return token
		}
	}
	return ""
}

// consistencyFindings fails closed on unknown.
func consistencyFindings(ctx context.Context, model *Model) ([]Finding, error) {
	doc, err := EmitConsistency(model)
	if err != nil {
		return nil, fmt.Errorf("consistencyFindings: %w", err)
	}

	result, err := Solve(ctx, doc, model)
	if err != nil {
		return nil, fmt.Errorf("consistencyFindings: %w", err)
	}

	switch result.Verdict {
	case VerdictSat:
		return nil, nil
	case VerdictUnsat:
		message := "the exclusivity groups contradict the invariants"
		if len(result.Core) > 0 {
			message = "invariants contradict each other: " + strings.Join(result.Core, ", ")
		}

		inconsistency := Finding{Check: CheckConsistency, EntryIds: result.Core, Message: message, Severity: SeverityError}
		findings := []Finding{inconsistency}
		return findings, nil
	case VerdictUnknown:
		return nil, errors.New("consistencyFindings: Consistency undecided within the check timeout")
	}
	return nil, fmt.Errorf("consistencyFindings: Unexpected verdict %q", result.Verdict)
}

// enumConditions builds one "atom = member" condition per enum member.
func enumConditions(atom *Atom) ([]*Expr, error) {
	conditions := make([]*Expr, 0, len(atom.Members))
	for _, member := range atom.Members {
		condition, err := ParseExpr(atom.Id + " = " + member)
		if err != nil {
			return nil, fmt.Errorf("enumConditions: %w", err)
		}

		conditions = append(conditions, condition)
	}
	return conditions, nil
}

func exprText(expr *Expr) string {
	switch expr.Kind {
	case ExprAtom:
		return expr.Atom
	case ExprNot:
		return "not " + operandText(expr.Left)
	case ExprAnd:
		return operandText(expr.Left) + " and " + operandText(expr.Right)
	case ExprOr:
		return operandText(expr.Left) + " or " + operandText(expr.Right)
	case ExprImplies:
		return operandText(expr.Left) + " => " + operandText(expr.Right)
	case ExprCmp:
		return termText(expr.Cmp.Left) + " " + cmpText(expr.Cmp.Op) + " " + termText(expr.Cmp.Right)
	}
	return ""
}

// firstPosition sorts a finding without entries after every entry.
func firstPosition(entryIds []string, positions map[string]int) int {
	if len(entryIds) == 0 {
		return len(positions)
	}

	return positions[entryIds[0]]
}

func isMessageDelimiter(character rune) bool {
	return unicode.IsSpace(character) || character == ':'
}

// operandText renders an operand: binary nodes in parentheses.
func operandText(expr *Expr) string {
	switch expr.Kind {
	case ExprAnd, ExprOr, ExprImplies:
		return "(" + exprText(expr) + ")"
	case ExprAtom, ExprCmp, ExprNot:
		return exprText(expr)
	}
	return ""
}

// postRenamer maps post atoms to true, their value once the transition fired.
func postRenamer(post []string) func(string) string {
	return func(atom string) string {
		if slices.Contains(post, atom) {
			return "true"
		}
		return atom
	}
}

// smtConjunction renders formulas as one SMT conjunction.
func smtConjunction(formulas []string) string {
	switch len(formulas) {
	case 0:
		return "true"
	case 1:
		return formulas[0]
	}
	return "(and " + strings.Join(formulas, " ") + ")"
}

// smtNegation renders the negation of one SMT formula.
func smtNegation(formula string) string {
	return "(not " + formula + ")"
}

// solveObligations asserts every selector before the first check, so a malformed formula errors ahead of any verdict.
func solveObligations(ctx context.Context, obligations []*obligation, prelude string) error {
	if len(obligations) == 0 {
		return nil
	}

	builder := &strings.Builder{}
	builder.WriteString(prelude)
	for index, current := range obligations {
		fmt.Fprintf(
			builder,
			"(declare-const ob.%d Bool)\n(assert (=> ob.%d %s))\n",
			index,
			index,
			current.formula,
		)
	}
	for index := range obligations {
		fmt.Fprintf(builder, "(check-sat-assuming (ob.%d))\n", index)
	}
	doc := SmtDoc{Checks: len(obligations), Text: builder.String()}
	verdicts, _, err := runZ3(ctx, doc)
	if err != nil {
		return fmt.Errorf("solveObligations: %w", err)
	}

	for index, verdict := range verdicts {
		obligations[index].verdict = verdict
	}
	return nil
}

func sortFindings(findings []Finding, model *Model) {
	positions := entryPositions(model)
	for index := range findings {
		findings[index].EntryIds = sortEntryIds(findings[index].EntryIds, positions)
	}
	slices.SortStableFunc(findings, func(left, right Finding) int {
		leftCheck := slices.Index(checkRunOrder, left.Check)
		rightCheck := slices.Index(checkRunOrder, right.Check)
		if leftCheck != rightCheck {
			return cmp.Compare(leftCheck, rightCheck)
		}

		leftPosition := firstPosition(left.EntryIds, positions)
		rightPosition := firstPosition(right.EntryIds, positions)
		return cmp.Compare(leftPosition, rightPosition)
	})
}

func sourcedConditions(cone map[string]bool, vocab *Vocabulary) ([]*Expr, error) {
	info := vocab.Info()
	coneIds := make([]string, 0, len(cone))
	for id := range cone {
		coneIds = append(coneIds, id)
	}
	slices.Sort(coneIds)
	var conditions []*Expr
	for _, id := range coneIds {
		atom, isAtom := info[id]
		isSourcedEnum := isAtom && atom.Source != "" && atom.EffectiveKind() == skillgraph.KindAtomEnum
		if !isSourcedEnum {
			continue
		}

		atomConditions, err := enumConditions(&atom)
		if err != nil {
			return nil, fmt.Errorf("sourcedConditions: %w", err)
		}

		conditions = append(conditions, atomConditions...)
	}
	return conditions, nil
}

// termText renders a comparison side in the DSL.
func termText(term *Term) string {
	switch term.Kind {
	case TermIdent:
		return term.Ident
	case TermLit:
		return strconv.Itoa(term.Lit)
	case TermAdd:
		return termText(term.Left) + " + " + termText(term.Right)
	case TermSub:
		return termText(term.Left) + " - " + termText(term.Right)
	}
	return ""
}

// validateFindings names every id that appears as a token of the message.
func validateFindings(model *Model, violations []string) []Finding {
	findings := make([]Finding, 0, len(violations))
	for _, violation := range violations {
		tokens := strings.FieldsFunc(violation, isMessageDelimiter)
		finding := Finding{Check: CheckValidate, EntryIds: tokens, Message: violation, Severity: SeverityError}
		findings = append(findings, finding)
	}
	sortFindings(findings, model)
	return findings
}

// RunBattery's error means the battery could not run; callers fail closed on it.
func RunBattery(ctx context.Context, model *Model) ([]Finding, error) {
	violations := model.Validate()
	if len(violations) > 0 {
		findings := validateFindings(model, violations)
		return findings, nil
	}

	if !model.IsStrict {
		return nil, nil
	}

	inconsistencies, err := consistencyFindings(ctx, model)
	if err != nil {
		return nil, fmt.Errorf("RunBattery: %w", err)
	}

	if len(inconsistencies) > 0 {
		return inconsistencies, nil
	}

	run, err := newBattery(model)
	if err != nil {
		return nil, fmt.Errorf("RunBattery: %w", err)
	}

	findings, err := run.warnings(ctx)
	if err != nil {
		return nil, fmt.Errorf("RunBattery: %w", err)
	}

	sortFindings(findings, model)
	return findings, nil
}

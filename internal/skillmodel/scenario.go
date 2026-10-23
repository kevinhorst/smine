package skillmodel

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

// Status of a section or entry under a scenario.
const (
	StatusActive   = "active"
	StatusInactive = "inactive"
	StatusUnknown  = "unknown"
)

// Input sources, as the source grammar names them.
const (
	InputSourceArg     = "arg"
	InputSourceSession = "session"
)

const inputIdSep = ":"

const (
	TriUnknown Tri = iota
	TriFalse
	TriTrue
)

// Input's Id is `arg:<row name>` or `session:<atom>[=<member>]`; Member is empty for a bool atom.
type Input struct {
	Id string

	Atom   string
	Label  string
	Member string
	Source string
}

// Scenario leaves an off enum atom at the first declared member no input forces.
type Scenario struct {
	Inputs []*Input
	On     map[string]bool
}

func NewScenario(inputs []*Input, on []string) *Scenario {
	scenario := &Scenario{Inputs: inputs, On: map[string]bool{}}
	for _, id := range on {
		scenario.Set(id, true)
	}
	return scenario
}

// Flip falsifies the first controllable literal of a holding pred, else satisfies every one.
func (s *Scenario) Flip(model *Model, pred string) bool {
	expr, err := ParseExpr(pred)
	if err != nil {
		return false
	}
	literals, isConjunction := conjunctLiterals(expr)
	if !isConjunction {
		return false
	}
	var controllable []*inputSetter
	for _, literal := range literals {
		if input := s.controller(literal); input != nil {
			controllable = append(controllable, literal.setter(input))
		}
	}
	if len(controllable) == 0 {
		return false
	}
	if evalExpr(expr, s.assignment(model.Vocabulary)) == TriTrue {
		s.Set(controllable[0].Id, !controllable[0].isOnWhenTrue())
		return true
	}
	for _, setter := range controllable {
		s.Set(setter.Id, setter.isOnWhenTrue())
	}
	return true
}

// OnIds lists the active input ids in input order.
func (s *Scenario) OnIds() []string {
	var ids []string
	for _, input := range s.Inputs {
		if s.On[input.Id] {
			ids = append(ids, input.Id)
		}
	}
	return ids
}

// Set switching an enum member on switches the atom's other inputs off.
func (s *Scenario) Set(id string, isOn bool) {
	input := s.input(id)
	if input == nil {
		return
	}
	if !isOn {
		delete(s.On, id)
		return
	}
	if input.Member != "" {
		for _, other := range s.Inputs {
			if other.Atom == input.Atom {
				delete(s.On, other.Id)
			}
		}
	}
	s.On[id] = true
}

// assignment maps every input atom to its value under the scenario.
func (s *Scenario) assignment(vocab *Vocabulary) map[string]atomValue {
	values := map[string]atomValue{}
	forced := map[string][]string{}
	for _, input := range s.Inputs {
		if input.Member == "" {
			current := values[input.Atom]
			current.IsTrue = current.IsTrue || s.On[input.Id]
			values[input.Atom] = current
			continue
		}
		forced[input.Atom] = append(forced[input.Atom], input.Member)
		if s.On[input.Id] && values[input.Atom].Member == "" {
			values[input.Atom] = atomValue{Member: input.Member}
		}
	}
	info := vocab.Info()
	for atom, members := range forced {
		if values[atom].Member != "" {
			continue
		}
		values[atom] = atomValue{Member: defaultMember(info[atom].Members, members)}
	}
	return values
}

func (s *Scenario) controller(literal *predLiteral) *Input {
	for _, input := range s.Inputs {
		if input.Atom == literal.Atom && input.Member == literal.Member {
			return input
		}
	}
	return nil
}

func (s *Scenario) input(id string) *Input {
	for _, input := range s.Inputs {
		if input.Id == id {
			return input
		}
	}
	return nil
}

// ScenarioResult's Replaced maps each replaced entry to the replacing route's section id.
type ScenarioResult struct {
	Atoms    map[string]string `json:"atoms"`
	Entries  map[string]string `json:"entries"`
	Flipped  bool              `json:"flipped"`
	On       []string          `json:"on"`
	Replaced map[string]string `json:"replaced"`
	States   map[string]string `json:"states"`
}

// Tri exists because a scenario fixes its input atoms and leaves every other atom unknown.
type Tri int

// atomValue is one assigned atom: a bool, or an enum member.
type atomValue struct {
	IsTrue bool
	Member string
}

func (v atomValue) text() string {
	if v.Member != "" {
		return v.Member
	}
	return strconv.FormatBool(v.IsTrue)
}

// inputSetter is one input switch a flip performs.
type inputSetter struct {
	Id string

	IsNegated bool
}

func (s *inputSetter) isOnWhenTrue() bool {
	return !s.IsNegated
}

type predLiteral struct {
	Atom      string
	IsNegated bool
	Member    string
}

func (l *predLiteral) setter(input *Input) *inputSetter {
	return &inputSetter{Id: input.Id, IsNegated: l.IsNegated}
}

// atomInputs reads one atom's source parts into inputs.
func atomInputs(atom *skillgraph.Atom, id string, rows map[string]string) []*Input {
	var inputs []*Input
	hasSession := false
	parts, err := skillgraph.ParseSource(atom.Source)
	if err != nil {
		return nil
	}
	for _, part := range parts {
		if part.Kind == skillgraph.SourceSession {
			hasSession = true
			continue
		}
		if part.Kind != skillgraph.SourceArg {
			continue
		}
		row, isRow := rows[part.Arg]
		if !isRow {
			continue
		}
		input := &Input{Id: InputSourceArg + inputIdSep + row, Atom: id, Label: row, Member: part.Member, Source: InputSourceArg}
		inputs = append(inputs, input)
	}
	if len(inputs) > 0 || !hasSession {
		return inputs
	}
	if atom.EffectiveKind() != skillgraph.KindAtomEnum {
		input := &Input{Id: InputSourceSession + inputIdSep + id, Atom: id, Label: id, Source: InputSourceSession}
		return []*Input{input}
	}
	for _, member := range atom.Members {
		input := &Input{Id: InputSourceSession + inputIdSep + id + "=" + member, Atom: id, Label: id + " = " + member, Member: member, Source: InputSourceSession}
		inputs = append(inputs, input)
	}
	return inputs
}

func cmpLiteral(cmp *Cmp) ([]*predLiteral, bool) {
	if cmp.Left.Kind != TermIdent || cmp.Right.Kind != TermIdent {
		return nil, false
	}
	switch cmp.Op {
	case CmpEq:
		literal := &predLiteral{Atom: cmp.Left.Ident, Member: cmp.Right.Ident}
		return []*predLiteral{literal}, true
	case CmpNe:
		literal := &predLiteral{Atom: cmp.Left.Ident, IsNegated: true, Member: cmp.Right.Ident}
		return []*predLiteral{literal}, true
	case CmpLt, CmpLe, CmpGt, CmpGe:
		return nil, false
	}
	return nil, false
}

func conjunctLiterals(expr *Expr) ([]*predLiteral, bool) {
	switch expr.Kind {
	case ExprAnd:
		left, isLeft := conjunctLiterals(expr.Left)
		right, isRight := conjunctLiterals(expr.Right)
		return append(left, right...), isLeft && isRight
	case ExprAtom:
		literal := &predLiteral{Atom: expr.Atom}
		return []*predLiteral{literal}, true
	case ExprNot:
		if expr.Left.Kind != ExprAtom {
			return nil, false
		}
		literal := &predLiteral{Atom: expr.Left.Atom, IsNegated: true}
		return []*predLiteral{literal}, true
	case ExprCmp:
		return cmpLiteral(expr.Cmp)
	case ExprOr, ExprImplies:
		return nil, false
	}
	return nil, false
}

func defaultMember(declared, forced []string) string {
	for _, member := range declared {
		if !slices.Contains(forced, member) {
			return member
		}
	}
	return ""
}

// evalCmp leaves int arithmetic unknown.
func evalCmp(cmp *Cmp, values map[string]atomValue) Tri {
	if cmp.Left.Kind != TermIdent || cmp.Right.Kind != TermIdent {
		return TriUnknown
	}
	value, isAssigned := values[cmp.Left.Ident]
	if !isAssigned || value.Member == "" {
		return TriUnknown
	}
	switch cmp.Op {
	case CmpEq:
		return triOf(value.Member == cmp.Right.Ident)
	case CmpNe:
		return triOf(value.Member != cmp.Right.Ident)
	case CmpLt, CmpLe, CmpGt, CmpGe:
		return TriUnknown
	}
	return TriUnknown
}

// evalExpr judges an expression under Kleene logic.
func evalExpr(expr *Expr, values map[string]atomValue) Tri {
	switch expr.Kind {
	case ExprAtom:
		value, isAssigned := values[expr.Atom]
		if !isAssigned || value.Member != "" {
			return TriUnknown
		}
		return triOf(value.IsTrue)
	case ExprNot:
		return triNot(evalExpr(expr.Left, values))
	case ExprAnd:
		return triAnd(evalExpr(expr.Left, values), evalExpr(expr.Right, values))
	case ExprOr:
		return triNot(triAnd(triNot(evalExpr(expr.Left, values)), triNot(evalExpr(expr.Right, values))))
	case ExprImplies:
		return triNot(triAnd(evalExpr(expr.Left, values), triNot(evalExpr(expr.Right, values))))
	case ExprCmp:
		return evalCmp(expr.Cmp, values)
	}
	return TriUnknown
}

// evalText judges a clause text; empty is true, an unparsable one unknown.
func evalText(pred string, values map[string]atomValue) Tri {
	if pred == "" {
		return TriTrue
	}
	expr, err := ParseExpr(pred)
	if err != nil {
		return TriUnknown
	}
	return evalExpr(expr, values)
}

func statusOf(truth Tri) string {
	switch truth {
	case TriTrue:
		return StatusActive
	case TriFalse:
		return StatusInactive
	case TriUnknown:
		return StatusUnknown
	}
	return StatusUnknown
}

func triAnd(left, right Tri) Tri {
	if left == TriFalse || right == TriFalse {
		return TriFalse
	}
	if left == TriTrue && right == TriTrue {
		return TriTrue
	}
	return TriUnknown
}

func triNot(truth Tri) Tri {
	switch truth {
	case TriTrue:
		return TriFalse
	case TriFalse:
		return TriTrue
	case TriUnknown:
		return TriUnknown
	}
	return TriUnknown
}

func triOf(isTrue bool) Tri {
	if isTrue {
		return TriTrue
	}
	return TriFalse
}

// Evaluate credits a replaced entry to the first active route in document order naming it.
func Evaluate(model *Model, scenario *Scenario) *ScenarioResult {
	values := scenario.assignment(model.Vocabulary)
	result := &ScenarioResult{
		Atoms:    map[string]string{},
		Entries:  map[string]string{},
		On:       scenario.OnIds(),
		Replaced: map[string]string{},
		States:   map[string]string{},
	}
	for atom, value := range values {
		result.Atoms[atom] = value.text()
	}
	for _, section := range model.Sections {
		status := StatusActive
		if section.Kind == skillgraph.SectionKindRoute {
			status = statusOf(evalText(section.Pred, values))
		}
		result.States[section.Id] = status
		if status != StatusActive {
			continue
		}
		for _, id := range section.Replaces {
			if _, isTaken := result.Replaced[id]; !isTaken {
				result.Replaced[id] = section.Id
			}
		}
	}
	for _, transition := range model.Transitions {
		result.Entries[transition.Id] = statusOf(evalText(transition.Pred, values))
	}
	for _, invariant := range slices.Concat(model.Invariants, model.Soft) {
		result.Entries[invariant.Id] = result.States[invariant.Section]
	}
	for id := range result.Replaced {
		result.Entries[id] = StatusInactive
	}
	return result
}

// Inputs matches an arg part to the Args row whose name equals it, leading dashes stripped on both sides.
func Inputs(skill *skillgraph.Skill) []*Input {
	rows := map[string]string{}
	for _, arg := range skill.Args() {
		rows[strings.TrimLeft(arg.Name, "-")] = arg.Name
	}
	atoms := skill.AllAtoms()
	var inputs []*Input
	for _, id := range slices.Sorted(maps.Keys(atoms)) {
		atom := atoms[id]
		if atom.Source == "" {
			continue
		}
		inputs = append(inputs, atomInputs(atom, id, rows)...)
	}
	return inputs
}

package skillmodel

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

// haltState is the terminal sink every stop edge points at.
const haltState = "halt"

// StartState and EndState are bracketed so they never collide with a section slug.
const (
	EndState   = "[end]"
	StartState = "[start]"
)

// anchorStart and anchorEnd are the chain-end tokens a between clause names.
const (
	anchorEnd   = "end"
	anchorStart = "start"
)

// Chain edge kinds and the otherwise bypass label.
const (
	edgeKindFlow    = "flow"
	edgeKindReentry = "reentry"
	edgeKindRoute   = "route"
	otherwiseLabel  = "otherwise"
	betweenLabel    = "between"
	replacesLabel   = "replaces"
)

// Projection resource kinds: the non-entry nodes Project derives from entry bullets.
const (
	ResourceKindSchema       = "schema"
	ResourceKindScript       = "script"
	ResourceKindSubagentType = "subagent-type"
	ResourceKindWorkflow     = "workflow"
)

const workflowPathPrefix = "workflows/"

// minEnumMembers rules out a single comparison like lanes_returned = lanes_spawned.
const minEnumMembers = 2

// boolCaseCount is how many cases a bool atom has — p and not p.
const boolCaseCount = 2

type forkCase struct {
	atom    string
	member  string
	negated bool
}

// Projection's JSON is the stable contract downstream renderers consume; the Mermaid output is a lossy view.
type Projection struct {
	Edges     []ProjectionEdge     `json:"edges"`
	Forks     []ProjectionFork     `json:"forks,omitempty"`
	Resources []ProjectionResource `json:"resources"`
	Skill     string               `json:"skill"`
	States    []ProjectionState    `json:"states"`
}

// EndsInFork: the end terminal is drawn only when the chain closes with a fork.
func (p *Projection) EndsInFork() bool {
	return slices.ContainsFunc(p.Forks, func(fork ProjectionFork) bool { return fork.Before == "" })
}

// Enums reads enums off the preds: an identifier compared against at least two distinct identifiers.
func (p *Projection) Enums() map[string][]string {
	seen := map[string]map[string]bool{}
	for _, state := range p.States {
		collectEnumMembers(state.Pred, seen)
		for _, entry := range state.Entries {
			collectEnumMembers(entry.Pred, seen)
		}
	}
	enums := map[string][]string{}
	for name, members := range seen {
		if len(members) >= minEnumMembers {
			names := maps.Keys(members)
			enums[name] = slices.Sorted(names)
		}
	}
	return enums
}

type ProjectionBranch struct {
	Pred   string   `json:"pred"`
	Routes []string `json:"routes"`
}

// ProjectionEdge kinds: flow (incl. the otherwise bypass), route (branch head), reentry (branch tail), stop.
type ProjectionEdge struct {
	Id string `json:"id,omitempty"`

	From  string `json:"from"`
	Kind  string `json:"kind"`
	Label string `json:"label,omitempty"`
	Pred  string `json:"pred,omitempty"`
	To    string `json:"to"`
}

// ProjectionEntry's detail fields are additive: absent keys keep older consumers valid.
type ProjectionEntry struct {
	Id string `json:"id"`

	Channel    string   `json:"channel,omitempty"`
	Custom     []string `json:"custom,omitempty"`
	Formal     bool     `json:"formal"`
	Gate       string   `json:"gate,omitempty"`
	Hook       string   `json:"hook,omitempty"`
	Params     []string `json:"params,omitempty"`
	Schema     string   `json:"schema,omitempty"`
	Script     string   `json:"script,omitempty"`
	Post       []string `json:"post,omitempty"`
	Pred       string   `json:"pred,omitempty"`
	RefBy      []string `json:"refBy,omitempty"`
	Refs       []string `json:"refs,omitempty"`
	Role       string   `json:"role"`
	Statement  string   `json:"statement,omitempty"`
	Subsection string   `json:"subsection,omitempty"`
	Title      string   `json:"title"`
}

// ProjectionFork's empty After/Before stand for the start/end terminals; a non-exhaustive fork gets an otherwise path.
type ProjectionFork struct {
	Id string `json:"id"`

	After      string             `json:"after,omitempty"`
	Before     string             `json:"before,omitempty"`
	Branches   []ProjectionBranch `json:"branches"`
	Exhaustive bool               `json:"exhaustive"`
}

func (f *ProjectionFork) ReentryId() string {
	return f.Id + ":reentry"
}

func (f *ProjectionFork) SplitId() string {
	return f.Id + ":split"
}

// ProjectionResource is a non-entry graph node derived from entry attributes.
type ProjectionResource struct {
	Entries []string `json:"entries"` // ids of the entries that reference it, sorted
	Id      string   `json:"id"`      // "<kind>:<name>"
	Kind    string   `json:"kind"`    // "subagent-type" | "workflow" | "schema" | "script"
	Label   string   `json:"label"`   // the name as written on the entry
}

// ProjectionSpan is a replaced run or an insertion's anchors; start and end stand in for the chain ends.
type ProjectionSpan struct {
	First string `json:"first"`
	Last  string `json:"last"`
}

type ProjectionState struct {
	Id string `json:"id"`

	Between  *ProjectionSpan   `json:"between,omitempty"`
	Entries  []ProjectionEntry `json:"entries,omitempty"`
	Kind     string            `json:"kind"`
	Pred     string            `json:"pred,omitempty"`
	Replaces []ProjectionSpan  `json:"replaces,omitempty"`
	Title    string            `json:"title"`
}

// Project puts stops of context sections on the halt state: they are global.
func Project(skill *skillgraph.Skill, model *Model) *Projection {
	projection := &Projection{Skill: skill.Name()}
	transitions := map[string]Transition{}
	for _, transition := range model.Transitions {
		transitions[transition.Id] = transition
	}
	refs := skill.Entries()
	refBy := map[string][]string{}
	for _, ref := range refs {
		if deriveSlug(ref.Section.Title) == "" {
			continue
		}
		for _, target := range ref.Entry.Refs(skill.Name()) {
			refBy[target] = append(refBy[target], ref.Entry.Id)
		}
	}
	stateIndex := map[string]int{}
	var stopEdges []ProjectionEdge
	var globalStops []ProjectionEntry
	for _, ref := range refs {
		entry := ref.Entry
		sectionId := deriveSlug(ref.Section.Title)
		if sectionId == "" {
			continue
		}
		if _, ok := stateIndex[sectionId]; !ok {
			stateIndex[sectionId] = len(projection.States)
			state := newProjectionState(ref, sectionId)
			projection.States = append(projection.States, state)
		}
		index := stateIndex[sectionId]
		transition := transitions[entry.Id]
		params := entry.Params()
		if applies := entry.Bullet(skillgraph.LabelApplies); applies != "" {
			params = append(params, applies)
		}
		params = append(params, skillgraph.FieldTexts(entry)...)
		// rows carry the authored pred; the route conjunction lives once on the state
		line := ProjectionEntry{
			Id:         entry.Id,
			Channel:    entry.EffectiveChannel(),
			Custom:     entry.Custom,
			Formal:     entry.Pred != "" || len(entry.Post) > 0,
			Gate:       entry.Bullet(skillgraph.LabelGate),
			Hook:       entry.Bullet(skillgraph.LabelHook),
			Params:     params,
			Schema:     entry.SchemaPointer(),
			Script:     entry.ScriptPointer(),
			Post:       transition.Post,
			Pred:       entry.Pred,
			RefBy:      refBy[entry.Id],
			Refs:       entry.Refs(skill.Name()),
			Role:       entry.Role,
			Statement:  entry.Statement,
			Subsection: ref.Subsection,
			Title:      entry.Title,
		}
		projection.States[index].Entries = append(projection.States[index].Entries, line)
		if entry.Role == KindStop {
			if projection.States[index].Kind == skillgraph.SectionKindContext {
				globalStops = append(globalStops, line)
				continue
			}
			edge := ProjectionEdge{
				Id:    entry.Id,
				From:  sectionId,
				Kind:  KindStop,
				Label: entry.Title,
				Pred:  transition.Pred,
				To:    haltState,
			}
			stopEdges = append(stopEdges, edge)
		}
	}
	projection.Forks = projection.deriveForks()
	projection.Edges = projection.chainEdges()
	projection.Edges = append(projection.Edges, stopEdges...)
	if len(stopEdges) > 0 || len(globalStops) > 0 {
		halt := ProjectionState{Entries: globalStops, Id: haltState, Kind: haltState, Title: "Halt"}
		projection.States = append(projection.States, halt)
	}
	projection.Resources = projectResources(refs)
	return projection
}

// flowRoles are the roles rendered as numbered execution lines.
var flowRoles = map[string]bool{
	KindSpawn:   true,
	KindStep:    true,
	KindStop:    true,
	KindSubstep: true,
	KindVerify:  true,
}

func (p *Projection) Mermaid(phasesOnly bool) string {
	builder := &strings.Builder{}
	builder.WriteString("stateDiagram-v2\n")
	for _, state := range p.States {
		if phasesOnly && state.Kind == skillgraph.SectionKindContext {
			continue
		}
		title := state.Title
		if state.Pred != "" {
			title += " {" + state.Pred + "}"
		}
		fmt.Fprintf(builder, "    state \"%s\" as %s\n", mermaidLabel(title), mermaidId(state.Id))
		sequence := 0
		for _, entry := range state.Entries {
			line := entry.Role + " " + entry.Title
			if flowRoles[entry.Role] {
				sequence++
				line = fmt.Sprintf("%d. %s", sequence, line)
			}
			if entry.Pred != "" {
				line += " {pred: " + entry.Pred + "}"
			}
			if len(entry.Post) > 0 {
				line += " {post: " + strings.Join(entry.Post, " and ") + "}"
			}
			if entry.Channel != "" && entry.Channel != "prose" {
				line += " [" + entry.Channel + "]"
			}
			fmt.Fprintf(builder, "    %s : %s\n", mermaidId(state.Id), mermaidLabel(line))
		}
	}
	for index := range p.Forks {
		fork := &p.Forks[index]
		splitId := mermaidId(fork.SplitId())
		reentryId := mermaidId(fork.ReentryId())
		fmt.Fprintf(builder, "    state %s <<choice>>\n", splitId)
		fmt.Fprintf(builder, "    state %s <<choice>>\n", reentryId)
	}
	for _, edge := range p.Edges {
		label := edge.Label
		if edge.Pred != "" {
			label = strings.TrimSpace(label + " {" + edge.Pred + "}")
		}
		if label == "" {
			fmt.Fprintf(builder, "    %s --> %s\n", mermaidId(edge.From), mermaidId(edge.To))
			continue
		}
		fmt.Fprintf(builder, "    %s --> %s : %s\n", mermaidId(edge.From), mermaidId(edge.To), mermaidLabel(label))
	}
	return builder.String()
}

// Inline: every replacing route and every between route no fork branch carries.
func (p *Projection) Inline(route *ProjectionState) bool {
	if len(route.Replaces) > 0 {
		return true
	}
	if route.Between == nil {
		return false
	}
	for _, fork := range p.Forks {
		for _, branch := range fork.Branches {
			if slices.Contains(branch.Routes, route.Id) {
				return false
			}
		}
	}
	return true
}

func (p *Projection) SectionOf(entryId string) string {
	for _, state := range p.States {
		for _, entry := range state.Entries {
			if entry.Id == entryId {
				return state.Id
			}
		}
	}
	return ""
}

func (p *Projection) chainEdges() []ProjectionEdge {
	forksAfter := map[string][]*ProjectionFork{}
	for index := range p.Forks {
		fork := &p.Forks[index]
		forksAfter[fork.After] = append(forksAfter[fork.After], fork)
	}
	var edges []ProjectionEdge
	node := ""
	if len(forksAfter[""]) > 0 {
		edges, node = forkEdges(edges, forksAfter[""], StartState)
	}
	for _, state := range p.States {
		if state.Kind != skillgraph.SectionKindPhase {
			continue
		}
		if node != "" {
			edge := ProjectionEdge{From: node, Kind: edgeKindFlow, To: state.Id}
			edges = append(edges, edge)
		}
		edges, node = forkEdges(edges, forksAfter[state.Id], state.Id)
	}
	if p.EndsInFork() {
		edge := ProjectionEdge{From: node, Kind: edgeKindFlow, To: EndState}
		edges = append(edges, edge)
	}
	for index := range p.States {
		route := &p.States[index]
		if p.Inline(route) {
			edges = append(edges, p.inlineEdges(route)...)
		}
	}
	return edges
}

// inlineEdges emits one route/reentry pair per distinct section pair.
func (p *Projection) inlineEdges(route *ProjectionState) []ProjectionEdge {
	label, spans := replacesLabel, route.Replaces
	if route.Between != nil {
		label, spans = betweenLabel, []ProjectionSpan{*route.Between}
	}
	var edges []ProjectionEdge
	seen := map[string]bool{}
	for _, span := range spans {
		splitSection := p.anchorSection(span.First, StartState)
		reentrySection := p.anchorSection(span.Last, EndState)
		if seen[splitSection+"\x00"+reentrySection] {
			continue
		}
		seen[splitSection+"\x00"+reentrySection] = true
		split := ProjectionEdge{From: splitSection, Kind: edgeKindRoute, Label: label, Pred: route.Pred, To: route.Id}
		reentry := ProjectionEdge{From: route.Id, Kind: edgeKindReentry, To: reentrySection}
		edges = append(edges, split, reentry)
	}
	return edges
}

func (p *Projection) anchorSection(entryId, terminal string) string {
	if entryId == anchorStart || entryId == anchorEnd {
		return terminal
	}
	return p.SectionOf(entryId)
}

// boundaryPhases fails for an anchor pair off the phase boundaries — such a route renders inline.
func (p *Projection) boundaryPhases(span *ProjectionSpan) (after, before string, ok bool) {
	phases := p.phaseIds()
	afterIndex := -1
	if span.First != anchorStart {
		afterIndex = slices.Index(phases, p.phaseEndingWith(span.First))
		if afterIndex < 0 {
			return "", "", false
		}
		after = phases[afterIndex]
	}
	if afterIndex+1 < len(phases) {
		before = phases[afterIndex+1]
	}
	beforeMatches := span.Last == anchorEnd && before == ""
	if span.Last != anchorEnd {
		beforeMatches = before != "" && p.phaseStartingWith(span.Last) == before
	}
	return after, before, beforeMatches
}

func (p *Projection) chainDepth(route *ProjectionState) int {
	depth := 0
	for current := route; current != nil && current.Between != nil && depth <= len(p.States); depth++ {
		current = p.routeHolding(current.Between.First)
	}
	return depth - 1
}

// deriveForks: replacing routes and inline between routes never fork.
func (p *Projection) deriveForks() []ProjectionFork {
	var routes []*ProjectionState
	for index := range p.States {
		if p.States[index].Between != nil {
			routes = append(routes, &p.States[index])
		}
	}
	slices.SortStableFunc(routes, func(a, b *ProjectionState) int {
		return p.chainDepth(a) - p.chainDepth(b)
	})
	var forks []ProjectionFork
	for _, route := range routes {
		forks = p.placeBetween(forks, route)
	}
	phases := p.phaseIds()
	slices.SortStableFunc(forks, func(a, b ProjectionFork) int {
		return slices.Index(phases, a.After) - slices.Index(phases, b.After)
	})
	enums := p.Enums()
	for index := range forks {
		forks[index].Id = fmt.Sprintf("fork-%d", index+1)
		forks[index].Exhaustive = forkExhaustive(enums, &forks[index])
	}
	return forks
}

func (p *Projection) phaseEndingWith(entryId string) string {
	for _, state := range p.States {
		flow := phaseFlow(&state)
		if len(flow) > 0 && flow[len(flow)-1] == entryId {
			return state.Id
		}
	}
	return ""
}

// phaseIds lists the phase sections in document order.
func (p *Projection) phaseIds() []string {
	var phases []string
	for _, state := range p.States {
		if state.Kind == skillgraph.SectionKindPhase {
			phases = append(phases, state.Id)
		}
	}
	return phases
}

func (p *Projection) phaseStartingWith(entryId string) string {
	for _, state := range p.States {
		flow := phaseFlow(&state)
		if len(flow) > 0 && flow[0] == entryId {
			return state.Id
		}
	}
	return ""
}

// placeBetween chains onto a branch tail first, then joins a boundary fork, else opens one.
func (p *Projection) placeBetween(forks []ProjectionFork, route *ProjectionState) []ProjectionFork {
	if tail := p.routeHolding(route.Between.First); tail != nil && isRouteTail(tail, route.Between.First) {
		extendChain(forks, route, tail.Id)
		return forks
	}
	after, before, ok := p.boundaryPhases(route.Between)
	if !ok {
		return forks
	}
	branch := ProjectionBranch{Pred: route.Pred, Routes: []string{route.Id}}
	for index := range forks {
		fork := &forks[index]
		if fork.After == after && fork.Before == before {
			fork.Branches = append(fork.Branches, branch)
			return forks
		}
	}
	fork := ProjectionFork{After: after, Before: before, Branches: []ProjectionBranch{branch}}
	return append(forks, fork)
}

func (p *Projection) routeHolding(entryId string) *ProjectionState {
	for index := range p.States {
		state := &p.States[index]
		if state.Kind != skillgraph.SectionKindRoute {
			continue
		}
		if slices.ContainsFunc(state.Entries, func(entry ProjectionEntry) bool { return entry.Id == entryId }) {
			return state
		}
	}
	return nil
}

// mermaidIdPattern matches the characters a Mermaid state id cannot carry.
var mermaidIdPattern = regexp.MustCompile(`[^A-Za-z0-9_]`)

func addResource(byId map[string]*ProjectionResource, entryId, kind, text string) {
	name := resourceName(text)
	if name == "" {
		return
	}

	id := kind + ":" + name
	resource, ok := byId[id]
	if !ok {
		resource = &ProjectionResource{Id: id, Kind: kind, Label: name}
		byId[id] = resource
	}
	resource.Entries = append(resource.Entries, entryId)
}

func branchEdges(branch *ProjectionBranch, fork *ProjectionFork) []ProjectionEdge {
	head := ProjectionEdge{From: fork.SplitId(), Kind: edgeKindRoute, Pred: branch.Pred, To: branch.Routes[0]}
	edges := []ProjectionEdge{head}
	for index := 1; index < len(branch.Routes); index++ {
		step := ProjectionEdge{From: branch.Routes[index-1], Kind: edgeKindFlow, To: branch.Routes[index]}
		edges = append(edges, step)
	}
	tail := ProjectionEdge{From: branch.Routes[len(branch.Routes)-1], Kind: edgeKindReentry, To: fork.ReentryId()}
	return append(edges, tail)
}

// collectEnumMembers skips unparsable preds.
func collectEnumMembers(pred string, seen map[string]map[string]bool) {
	if pred == "" {
		return
	}
	expr, err := ParseExpr(pred)
	if err != nil {
		return
	}
	for _, cmp := range expr.comparisons() {
		isEquality := cmp.Op == CmpEq || cmp.Op == CmpNe
		isIdentPair := cmp.Left.Kind == TermIdent && cmp.Right.Kind == TermIdent
		if !isEquality || !isIdentPair {
			continue
		}
		if seen[cmp.Left.Ident] == nil {
			seen[cmp.Left.Ident] = map[string]bool{}
		}
		seen[cmp.Left.Ident][cmp.Right.Ident] = true
	}
}

// extendChain: a chain step runs only after the step before it.
func extendChain(forks []ProjectionFork, route *ProjectionState, tailId string) {
	for forkIndex := range forks {
		branches := forks[forkIndex].Branches
		for index := range branches {
			routes := branches[index].Routes
			if routes[len(routes)-1] == tailId {
				branches[index].Routes = append(branches[index].Routes, route.Id)
				return
			}
		}
	}
}

func forkEdges(edges []ProjectionEdge, forks []*ProjectionFork, from string) ([]ProjectionEdge, string) {
	for _, fork := range forks {
		entry := ProjectionEdge{From: from, Kind: edgeKindFlow, To: fork.SplitId()}
		edges = append(edges, entry)
		for _, branch := range fork.Branches {
			branchLinks := branchEdges(&branch, fork)
			edges = append(edges, branchLinks...)
		}
		if !fork.Exhaustive {
			otherwise := ProjectionEdge{
				From:  fork.SplitId(),
				Kind:  edgeKindFlow,
				Label: otherwiseLabel,
				To:    fork.ReentryId(),
			}
			edges = append(edges, otherwise)
		}
		from = fork.ReentryId()
	}
	return edges, from
}

func forkExhaustive(enums map[string][]string, fork *ProjectionFork) bool {
	var cases []*forkCase
	for _, branch := range fork.Branches {
		branchCase := parseForkCase(branch.Pred)
		if branchCase == nil {
			return false
		}
		cases = append(cases, branchCase)
	}
	// branches are pairwise exclusive by construction: two bool branches are p and not p
	if cases[0].member == "" {
		return len(cases) == boolCaseCount
	}
	covered := map[string]bool{}
	for _, branchCase := range cases {
		covered[branchCase.member] = true
	}
	members := enums[cases[0].atom]
	if len(members) == 0 {
		return false
	}
	for _, member := range members {
		if !covered[member] {
			return false
		}
	}
	return true
}

func mermaidId(id string) string {
	if id == StartState || id == EndState {
		return "[*]"
	}
	return mermaidIdPattern.ReplaceAllString(id, "_")
}

func parseEnumCase(cmp *Cmp) *forkCase {
	isIdentPair := cmp.Left.Kind == TermIdent && cmp.Right.Kind == TermIdent
	if cmp.Op != CmpEq || !isIdentPair {
		return nil
	}
	return &forkCase{atom: cmp.Left.Ident, member: cmp.Right.Ident}
}

// parseForkCase returns nil for any other shape: such a pred is exclusive with nothing.
func parseForkCase(pred string) *forkCase {
	expr, err := ParseExpr(pred)
	if err != nil {
		return nil
	}
	switch expr.Kind {
	case ExprAtom:
		return &forkCase{atom: expr.Atom}
	case ExprNot:
		if expr.Left.Kind != ExprAtom {
			return nil
		}
		return &forkCase{atom: expr.Left.Atom, negated: true}
	case ExprCmp:
		return parseEnumCase(expr.Cmp)
	case ExprAnd, ExprOr, ExprImplies:
		return nil
	}
	return nil
}

// isRouteTail: only the route's last flow entry chains a between route onto it.
func isRouteTail(route *ProjectionState, entryId string) bool {
	last := ""
	for _, entry := range route.Entries {
		if flowRoles[entry.Role] {
			last = entry.Id
		}
	}
	return last == entryId
}

func phaseFlow(state *ProjectionState) []string {
	if state.Kind != skillgraph.SectionKindPhase {
		return nil
	}
	var flow []string
	for _, entry := range state.Entries {
		if flowRoles[entry.Role] {
			flow = append(flow, entry.Id)
		}
	}
	return flow
}

// projectResources never returns nil.
func projectResources(refs []skillgraph.EntryRef) []ProjectionResource {
	byId := map[string]*ProjectionResource{}
	for _, ref := range refs {
		if deriveSlug(ref.Section.Title) == "" {
			continue
		}

		entry := ref.Entry
		schema := entry.SchemaPointer()
		script := entry.ScriptPointer()
		scriptResourceKind := scriptKind(script)
		subagentType := entry.Bullet(skillgraph.LabelSubagentType)
		addResource(byId, entry.Id, ResourceKindSchema, schema)
		addResource(byId, entry.Id, scriptResourceKind, script)
		addResource(byId, entry.Id, ResourceKindSubagentType, subagentType)
	}

	resources := make([]ProjectionResource, 0, len(byId))
	for _, resource := range byId {
		slices.Sort(resource.Entries)
		resources = append(resources, *resource)
	}
	slices.SortFunc(resources, func(a, b ProjectionResource) int {
		return strings.Compare(a.Id, b.Id)
	})
	return resources
}

func resourceName(text string) string {
	return strings.Trim(strings.TrimSpace(text), "`")
}

func scriptKind(text string) string {
	if strings.HasPrefix(resourceName(text), workflowPathPrefix) {
		return ResourceKindWorkflow
	}
	return ResourceKindScript
}

// splitSpan reads one resolved anchor item — an id or FIRST..LAST — as a span.
func splitSpan(item string) ProjectionSpan {
	first, last, isRange := strings.Cut(item, "..")
	if !isRange {
		last = first
	}
	span := ProjectionSpan{First: first, Last: last}
	return span
}

func idName(entryId string) string {
	_, rest, _ := strings.Cut(entryId, "-")
	name, _, _ := strings.Cut(rest, "-")
	return name
}

// newProjectionState ignores the Anchor error: Derive refused any skill failing Check.
func newProjectionState(ref skillgraph.EntryRef, sectionId string) ProjectionState {
	section := ref.Section
	state := ProjectionState{Id: sectionId, Kind: section.EffectiveKind(), Pred: section.Pred, Title: section.Title}
	replaces, between, _ := section.Anchor(idName(ref.Entry.Id))
	if between != nil {
		state.Between = &ProjectionSpan{First: between[0], Last: between[1]}
	}
	for _, item := range replaces {
		state.Replaces = append(state.Replaces, splitSpan(item))
	}
	return state
}

// mermaidLabel keeps operators verbatim: Mermaid only mangles bracketed tokens.
func mermaidLabel(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

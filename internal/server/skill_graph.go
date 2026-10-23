package server

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/kevinhorst/smine/internal/server/respond"
	"github.com/kevinhorst/smine/internal/sessiontitle"
	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skillmodel"
	"github.com/kevinhorst/smine/internal/skills"
)

const tmplSkillGraph = "skill_graph.html"

// noModelDeployed is the load-error text of a skill dir without a model file.
const noModelDeployed = "no skill model — deployed copies carry only the rendered SKILL.md; open the repo skill for its graph"

// fieldAcdslContext is the frontmatter key declaring the skill's context entries.
const fieldAcdslContext = "acdsl-context"

// Edge kinds from an entry to a resource it references.
const (
	edgeKindScript     = "script"
	edgeKindSpawns     = "spawns"
	edgeKindUsesSchema = "uses-schema"
)

// Resource rail group labels.
const (
	refLabelSchemas       = "schemas"
	refLabelScripts       = "scripts"
	refLabelSubagentTypes = "subagent types"
	refLabelWorkflows     = "workflows"
)

// Gate rule labels, as the enforce block names them.
const (
	gateLabelPlanModeDenied = "planModeDenied"
	gateLabelRootDenied     = "rootDenied"
	gateLabelWorkflowOnly   = "workflowOnly"
	gateLabelWriters        = "writers"
)

// roleRoute is the role chip of a phase card's "runs here" marker row.
const roleRoute = "route"

// Card compartment slots in schema order.
const (
	slotDefs     = "defs"
	slotFlow     = "flow"
	slotPayloads = "payloads"
	slotQuality  = "quality"
	slotRules    = "rules"
)

// anchorStart and anchorEnd are the chain-end tokens a between clause names.
const (
	anchorEnd   = "end"
	anchorStart = "start"
)

// Warning kinds: what a row of the warnings panel points at.
const (
	warningKindArg     = "arg"
	warningKindAtom    = "atom"
	warningKindEntry   = "entry"
	warningKindSection = "section"
)

// warningUnreadAtom is the read-or-delete warning of a skill-local atom no pred reads.
const warningUnreadAtom = "no pred reads it — delete it and the posts granting it"

// warningNoOwner is the warning of an Args row whose Effect cell links no entry.
const warningNoOwner = "names no owning entry — link one in its Effect cell"

// Env tile states.
const (
	envStateDeferred      = "deferred"
	envStateSet           = "set"
	envStateUnsetOptional = "unset-optional"
	envStateUnsetRequired = "unset-required"
)

// envMarks is the mark an env tile shows per state.
var envMarks = map[string]string{envStateDeferred: "↯", envStateSet: "●", envStateUnsetOptional: "○", envStateUnsetRequired: "⚠"}

// structureCountRoles fixes the role order of the card header counts.
var structureCountRoles = []string{roleDef, roleRule, roleQuality, roleStep, roleSpawn, roleSubstep, roleVerify, roleStop, rolePayload}

// skillGraphPage.LoadError replaces everything when the model does not load.
type skillGraphPage struct {
	Args           []structureArgRow
	Atoms          []graphAtomRow
	Base           string
	ContextEntries []graphRefItem
	Contexts       []structureStateView
	DataJson       template.JS
	Enforce        *graphEnforceView
	EntryCount     int
	Env            []structureEnvRow
	Inputs         []structureInputRow
	LoadError      string
	Note           string
	Page           string
	Picker         []skillOption
	References     []graphRefGroup
	Skill          *skills.Skill
	Smoke          *smokeBadge
	Spine          []structureStateView
	Tab            string
	Tier           string
	Title          string
	TitleStates    *graphTitleView
	Unset          []string
}

type skillOption struct {
	IsSelected bool
	Label      string
	Value      string
}

// structureBlob carries view concerns kept out of the projection contract.
type structureBlob struct {
	Args         []structureArgRow      `json:"args,omitempty"`
	EntryNotes   map[string]string      `json:"entryNotes,omitempty"`
	Env          []structureEnvRow      `json:"env,omitempty"`
	Payloads     map[string]string      `json:"payloads"`
	Projection   *skillmodel.Projection `json:"projection"`
	Reentries    map[string]string      `json:"reentries,omitempty"`
	SectionNotes map[string]string      `json:"sectionNotes,omitempty"`
	Warnings     []graphWarning         `json:"warnings,omitempty"`
}

type graphAnchorRow struct {
	Id string

	Agent     string
	Captures  []string
	Match     []string
	Tool      string
	ToolLabel string
	UsedBy    string
}

type graphAtomRow struct {
	Id string

	GrantedBy int
	IsShared  bool
	IsUnread  bool
	Kind      string
	ReadBy    int
	Sort      string
	Source    string
}

type graphCheckRow struct {
	Id string

	Anchors []skillgraph.AnchorRef
	Kind    string
	Reason  string
}

type graphEnforceView struct {
	Anchors  []graphAnchorRow
	Checks   []graphCheckRow
	Gate     []graphGateRow
	Judgment []graphJudgmentRow
}

type graphGateRow struct {
	Label    string
	Sentence string
	Values   []string
}

type graphJudgmentRow struct {
	Id string

	Anchor   string
	Evidence string
	Question string
}

type graphRefGroup struct {
	EdgeKind string
	Items    []graphRefItem
	Label    string
}

type graphRefItem struct {
	Count    int
	EntryIds string
	Href     string
	Id       string
	Name     string
}

type graphScenario struct {
	Entries  map[string]string
	Replaced map[string]string
	States   map[string]string
}

// newGraphScenario resolves the replacing routes' titles.
func newGraphScenario(derived *skillmodel.Model, result *skillmodel.ScenarioResult) *graphScenario {
	titles := map[string]string{}
	for _, section := range derived.Sections {
		titles[section.Id] = section.Title
	}
	view := &graphScenario{Entries: result.Entries, Replaced: map[string]string{}, States: result.States}
	for id, route := range result.Replaced {
		view.Replaced[id] = titles[route]
	}
	return view
}

type graphTitleRow struct {
	Command    string
	Entry      string
	EntryTitle string
	Meaning    string
	State      string
}

type graphTitleView struct {
	States []graphTitleRow
}

// graphWarning lists server-side warnings only; battery findings join on the client.
type graphWarning struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Target  string `json:"target"`
	Title   string `json:"title"`
}

// structureStateView.Status is the default scenario's verdict.
type structureStateView struct {
	Anchor        []structureAnchorLink
	AnchorKind    string
	Compartments  []structureCompartment
	Consumes      []string
	Id            string
	Kind          string
	Pred          string
	Produces      []string
	ReplacedCount int
	RoleCounts    string
	SetBy         []structureInputLink
	Status        string
	Title         string
	Warnings      []string
}

type structureAnchorLink struct {
	FirstId    string
	FirstTitle string
	LastId     string
	LastTitle  string
}

type cardCutter struct {
	cards []structureStateView
	open  structureCompartment
	part  []structureCompartment
	parts int
	view  structureStateView
}

// closeCompartment keeps the compartment open under its label for the rows that follow.
func (c *cardCutter) closeCompartment() {
	if len(c.open.Rows) > 0 {
		c.part = append(c.part, c.open)
	}
	c.open.Rows = nil
}

// closePart names continuation cards `<id>~<n>`.
func (c *cardCutter) closePart() {
	c.closeCompartment()
	if len(c.part) == 0 {
		return
	}

	card := c.view
	if c.parts > 0 {
		card = structureStateView{
			Id:     fmt.Sprintf("%s~%d", c.view.Id, c.parts+1),
			Kind:   c.view.Kind,
			Status: c.view.Status,
			Title:  c.view.Title + " (continued)",
		}
	}
	card.Compartments = c.part
	c.cards = append(c.cards, card)
	c.part = nil
	c.parts++
}

func (c *cardCutter) split(routes []structureStateView) {
	if len(routes) == 0 {
		return
	}

	c.closePart()
	c.cards = append(c.cards, routes...)
}

// routeNester.placed guards each route against a second placement.
type routeNester struct {
	after  map[string][]string
	before map[string][]string
	placed map[string]bool
	views  map[string]structureStateView
}

func (n *routeNester) routeCards(ids []string) []structureStateView {
	var cards []structureStateView
	for _, id := range ids {
		if n.placed[id] {
			continue
		}

		n.placed[id] = true
		cards = append(cards, n.cut(n.views[id])...)
	}
	return cards
}

func (n *routeNester) cut(view structureStateView) []structureStateView {
	cutter := &cardCutter{view: view}
	for _, compartment := range view.Compartments {
		cutter.open = structureCompartment{Flow: compartment.Flow, Label: compartment.Label}
		for _, row := range compartment.Rows {
			cutter.split(n.routeCards(n.before[row.Id]))
			cutter.open.Rows = append(cutter.open.Rows, row)
			cutter.split(n.routeCards(n.after[row.Id]))
		}
		cutter.closeCompartment()
	}
	cutter.closePart()
	if cutter.parts == 0 {
		return append([]structureStateView{view}, cutter.cards...)
	}
	return cutter.cards
}

type structureArgRow struct {
	Default string           `json:"default"`
	Effect  string           `json:"effect"`
	Form    string           `json:"form"`
	Grants  []structureGrant `json:"grants,omitempty"`
	Input   string           `json:"input,omitempty"`
	Name    string           `json:"name"`
	Owners  []string         `json:"owners,omitempty"`
}

// structureEnvRow is one declared variable as the graph's env tile and card carry it.
type structureEnvRow struct {
	Cause      string            `json:"cause,omitempty"`
	Interface  string            `json:"interface"`
	IsDeferred bool              `json:"isDeferred,omitempty"`
	IsRequired bool              `json:"isRequired,omitempty"`
	Key        string            `json:"key"`
	Mark       string            `json:"mark"`
	Name       string            `json:"name"`
	Readers    []structureReader `json:"readers,omitempty"`
	Scope      string            `json:"scope"`
	Source     string            `json:"source"`
	State      string            `json:"state"`
	Summary    string            `json:"summary"`
	Value      string            `json:"value,omitempty"`
}

// structureReader is one reader of a variable: an entry row, a param of one, a grant, an anchor or a check.
type structureReader struct {
	Id string `json:"id"`

	Kind  string `json:"kind"`
	Label string `json:"label"`
}

type structureGrant struct {
	Atom  string `json:"atom"`
	Label string `json:"label"`
}

type structureInputRow struct {
	Id string

	Atom  string
	Label string
}

type structureInputLink struct {
	Id string

	Label string
}

type structureCompartment struct {
	Flow  bool
	Label string
	Rows  []structureEntryRow
}

type structureEntryRow struct {
	Channel     string
	Custom      string
	Gate        string
	Hook        string
	Id          string
	Indent      bool
	Post        string
	Pred        string
	RefByCount  int
	RefCount    int
	ReplacedBy  string
	Role        string
	Schema      string
	Script      string
	ScriptShort string
	Seq         int
	Status      string
	Stop        bool
	Subsection  string
	Title       string
	TitleState  string
	Warnings    string
}

func (s *Server) handleSkillGraph(w http.ResponseWriter, r *http.Request) {
	skill := s.findSkill(w, r)
	if skill == nil {
		return
	}

	s.renderSkillGraph(r.Context(), nil, skill, w)
}

// handleSkillsGraph falls back to the first listed skill without a picker value.
func (s *Server) handleSkillsGraph(w http.ResponseWriter, r *http.Request) {
	list, err := skills.Scan(s.skillsRepo, s.skillsHome)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	cards := mergeSkills(list)
	if len(cards) == 0 {
		http.Redirect(w, r, "/scripts/skills", http.StatusSeeOther)
		return
	}

	wanted := r.URL.Query().Get("skill")
	if wanted == "" {
		wanted = skillOptionValue(&cards[0].Skill)
	}
	var selected *skills.Skill
	picker := make([]skillOption, 0, len(cards))
	for index := range cards {
		skill := &cards[index].Skill
		option := skillOption{IsSelected: skillOptionValue(skill) == wanted, Label: skill.Name, Value: skillOptionValue(skill)}
		if option.IsSelected {
			selected = skill
		}
		picker = append(picker, option)
	}
	if selected == nil {
		http.NotFound(w, r)
		return
	}

	s.renderSkillGraph(r.Context(), picker, selected, w)
}

func (s *Server) renderSkillGraph(ctx context.Context, picker []skillOption, skill *skills.Skill, w http.ResponseWriter) {
	data := skillGraphPage{
		Page:   pageSkills,
		Picker: picker,
		Skill:  skill,
		Smoke:  s.lastSmoke(skill),
		Tab:    skillsTabGraph,
		Title:  "Skill — " + skill.Name + " — Graph",
	}
	if _, err := os.Stat(skillgraph.ModelPath(skill.Path)); err != nil {
		data.LoadError = noModelDeployed
		s.renderFragment(w, tmplSkillGraph, data)
		return
	}

	skillModel, derived, err := loadStructure(skill.Path)
	if err != nil {
		data.LoadError = err.Error()
		s.renderFragment(w, tmplSkillGraph, data)
		return
	}

	projection := skillmodel.Project(skillModel, derived)
	entries := skillModel.Entries()
	inputs := skillmodel.Inputs(skillModel)
	args, sessionInputs := argsWithInputs(structureArgs(skillModel.Args(), skillModel), inputs)
	result := skillmodel.Evaluate(derived, skillmodel.NewScenario(inputs, nil))
	scenario := newGraphScenario(derived, result)
	data.Args = args
	data.Atoms = atomRows(skillModel)
	data.Base = graphBase(skill)
	data.ContextEntries = contextEntries(skillModel.Field(fieldAcdslContext))
	data.Enforce = enforceView(skillModel.Enforce)
	data.EntryCount = len(entries)
	values, rows := s.variableValues(ctx, skillModel)
	data.Env = graphEnvRows(skillModel, entryTitles(entries), values)
	data.Unset = unsetRequired(skillModel, values, rows, s.repoRegistry.Repos())
	data.Inputs = sessionInputs
	data.Note = skillModel.Note
	data.References = referenceGroups(projection)
	data.Tier = structureTier(skill.Path, skillModel)
	data.TitleStates = titleStatesView(skillModel)
	notes := structureNotes(projection, entries)
	interfaces := phaseInterfaces(projection)
	titleStates := titleStateEntries(data.TitleStates)
	views := map[string]structureStateView{}
	for _, state := range projection.States {
		if state.Kind == "halt" {
			continue
		}
		view := buildStructureStateView(notes, scenario, state, titleStates)
		switch state.Kind {
		case skillgraph.SectionKindRoute:
			view.AnchorKind, view.Anchor, view.ReplacedCount = anchorLinks(derived, projection, &state)
			view.SetBy = routeSetBy(inputs, state.Pred)
			views[state.Id] = view
		case skillgraph.SectionKindContext:
			data.Contexts = append(data.Contexts, view)
		default:
			iface := interfaces[state.Id]
			view.Consumes = iface.consumes
			view.Produces = iface.produces
			if len(iface.produces) > 0 && !iface.consumed {
				view.Warnings = append(view.Warnings, "phase grants nothing a later phase consumes")
			}
			views[state.Id] = view
		}
	}
	data.Spine = buildSpine(newRouteNester(derived, projection, views), projection)
	blobData := structureBlob{
		Args:         args,
		EntryNotes:   entryNotes(entries),
		Env:          data.Env,
		Payloads:     payloadTexts(entries),
		Projection:   projection,
		Reentries:    routeReentries(derived, projection),
		SectionNotes: sectionNotes(projection, skillModel),
		Warnings:     graphWarnings(args, data.Atoms, notes, projection, views),
	}
	blob, err := json.Marshal(&blobData)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	data.DataJson = template.JS(blob)
	s.renderFragment(w, tmplSkillGraph, data)
}

// buildSpine appends routes no rendered entry anchors after the chain.
func buildSpine(nester *routeNester, projection *skillmodel.Projection) []structureStateView {
	var spine []structureStateView
	for _, state := range projection.States {
		if state.Kind == skillgraph.SectionKindPhase {
			spine = append(spine, nester.cut(nester.views[state.Id])...)
		}
	}
	for _, state := range projection.States {
		if state.Kind == skillgraph.SectionKindRoute {
			spine = append(spine, nester.routeCards([]string{state.Id})...)
		}
	}
	return spine
}

func routeReentries(derived *skillmodel.Model, projection *skillmodel.Projection) map[string]string {
	order := phaseEntryOrder(projection)
	reentries := map[string]string{}
	for index := range projection.States {
		route := &projection.States[index]
		if route.Kind != skillgraph.SectionKindRoute {
			continue
		}

		replaced := derivedSection(derived, route.Id).Replaces
		if len(replaced) == 0 {
			if route.Between != nil && route.Between.Last != anchorEnd {
				reentries[route.Id] = route.Between.Last
			}
			continue
		}

		position := slices.Index(order, replaced[len(replaced)-1])
		if position >= 0 && position+1 < len(order) {
			reentries[route.Id] = order[position+1]
		}
	}
	return reentries
}

func newRouteNester(derived *skillmodel.Model, projection *skillmodel.Projection, views map[string]structureStateView) *routeNester {
	nester := &routeNester{after: map[string][]string{}, before: map[string][]string{}, placed: map[string]bool{}, views: views}
	order := phaseEntryOrder(projection)
	for index := range projection.States {
		route := &projection.States[index]
		if route.Kind != skillgraph.SectionKindRoute {
			continue
		}

		replaced := derivedSection(derived, route.Id).Replaces
		switch {
		case len(replaced) > 0:
			nester.before[replaced[0]] = append(nester.before[replaced[0]], route.Id)
		case route.Between == nil:
			continue
		case route.Between.First != anchorStart:
			nester.after[route.Between.First] = append(nester.after[route.Between.First], route.Id)
		case route.Between.Last != anchorEnd:
			nester.before[route.Between.Last] = append(nester.before[route.Between.Last], route.Id)
		case len(order) > 0:
			nester.before[order[0]] = append(nester.before[order[0]], route.Id)
		}
	}
	return nester
}

func payloadTexts(entries []skillgraph.EntryRef) map[string]string {
	payloads := map[string]string{}
	for _, ref := range entries {
		if ref.Entry.Payload != nil {
			payloads[ref.Entry.Id] = strings.TrimSuffix(ref.Entry.Payload.Body, "\n")
		}
	}
	return payloads
}

func anchorLinks(derived *skillmodel.Model, projection *skillmodel.Projection, route *skillmodel.ProjectionState) (string, []structureAnchorLink, int) {
	titles := map[string]string{anchorStart: anchorStart, anchorEnd: anchorEnd}
	for _, state := range projection.States {
		for _, entry := range state.Entries {
			titles[entry.Id] = entry.Title
		}
	}
	if route.Between != nil {
		link := structureAnchorLink{FirstId: route.Between.First, FirstTitle: titles[route.Between.First], LastId: route.Between.Last, LastTitle: titles[route.Between.Last]}
		links := []structureAnchorLink{link}
		return "between", links, 0
	}
	replaced := derivedSection(derived, route.Id).Replaces
	if len(replaced) == 0 {
		return "replaces", nil, 0
	}
	first, last := replaced[0], replaced[len(replaced)-1]
	link := structureAnchorLink{FirstId: first, FirstTitle: titles[first], LastId: last, LastTitle: titles[last]}
	links := []structureAnchorLink{link}
	return "replaces", links, len(replaced)
}

// derivedSection matches on the section title's slug, which both sides carry.
func derivedSection(derived *skillmodel.Model, id string) skillmodel.Section {
	for _, section := range derived.Sections {
		if section.Id == id {
			return section
		}
	}
	return skillmodel.Section{}
}

func routeSetBy(inputs []*skillmodel.Input, pred string) []structureInputLink {
	expr, err := skillmodel.ParseExpr(pred)
	if err != nil {
		return nil
	}
	atoms := expr.Atoms()
	var links []structureInputLink
	for _, input := range inputs {
		if slices.Contains(atoms, input.Atom) {
			link := structureInputLink{Id: input.Id, Label: input.Label}
			links = append(links, link)
		}
	}
	return links
}

func phaseEntryOrder(projection *skillmodel.Projection) []string {
	var order []string
	for _, state := range projection.States {
		if state.Kind != skillgraph.SectionKindPhase && state.Kind != "" {
			continue
		}
		for _, entry := range state.Entries {
			if skillgraph.CoverableRole(entry.Role) {
				order = append(order, entry.Id)
			}
		}
	}
	return order
}

type phaseInterface struct {
	consumed bool
	consumes []string
	produces []string
}

// atomTokenPattern matches atom identifiers inside a pred expression.
var atomTokenPattern = regexp.MustCompile(`[A-Za-z_]\w*`)

// phaseInterfaces computes produces/consumes; they are never authored.
func phaseInterfaces(projection *skillmodel.Projection) map[string]phaseInterface {
	interfaces := map[string]phaseInterface{}
	var order []string
	earlier := map[string]string{}
	producedBy := map[string][]string{}
	for _, state := range projection.States {
		if state.Kind != skillgraph.SectionKindPhase && state.Kind != "" {
			continue
		}
		order = append(order, state.Id)
		iface := phaseInterface{}
		produced := map[string]bool{}
		consumed := map[string]bool{}
		for _, entry := range state.Entries {
			for _, atom := range entry.Post {
				if !produced[atom] {
					produced[atom] = true
					iface.produces = append(iface.produces, atom)
					producedBy[state.Id] = append(producedBy[state.Id], atom)
				}
				if _, ok := earlier[atom]; !ok {
					earlier[atom] = state.Id
				}
			}
			for _, token := range atomTokenPattern.FindAllString(entry.Pred, -1) {
				source, ok := earlier[token]
				if ok && source != state.Id && !consumed[token] {
					consumed[token] = true
					iface.consumes = append(iface.consumes, token)
				}
			}
		}
		sort.Strings(iface.consumes)
		sort.Strings(iface.produces)
		interfaces[state.Id] = iface
	}
	// A phase counts as consumed when any later phase consumes one of its atoms.
	for index, id := range order {
		iface := interfaces[id]
		for _, later := range order[index+1:] {
			for _, atom := range interfaces[later].consumes {
				if earlier[atom] == id {
					iface.consumed = true
				}
			}
		}
		interfaces[id] = iface
	}
	return interfaces
}

// structureArgs keeps the entries an Effect cell links — markdown links and [[ID]] tokens alike — as the row's owners; cells show the link labels.
func structureArgs(args []skillgraph.Arg, skillModel *skillgraph.Skill) []structureArgRow {
	clean := func(cell string) string {
		cell = strings.ReplaceAll(cell, `\|`, "|")
		cell = skillgraph.ExpandTokens(skillModel, cell)
		cell = structureRefLinkPattern.ReplaceAllString(cell, "$1")
		return strings.Trim(cell, "`")
	}
	rows := make([]structureArgRow, 0, len(args))
	for _, arg := range args {
		row := structureArgRow{Default: clean(arg.Default), Effect: clean(arg.Effect), Form: clean(arg.Form), Name: arg.Name}
		for _, id := range skillgraph.TextRefs(skillModel.Name(), arg.Effect) {
			if _, isEntry := skillModel.Entry(id); isEntry {
				row.Owners = append(row.Owners, id)
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// structureRefLinkPattern reduces markdown links to their label for table cells.
var structureRefLinkPattern = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)

// loadStructure passes Load's error through bare: its lines are the violations the load-error card lists.
func loadStructure(path string) (*skillgraph.Skill, *skillmodel.Model, error) {
	skillModel, err := skillgraph.Load(path)
	if err != nil {
		return nil, nil, err
	}

	derived, err := skillmodel.Derive(skillModel)
	if err != nil {
		return nil, nil, fmt.Errorf("loadStructure: %w", err)
	}

	return skillModel, derived, nil
}

func argsWithInputs(rows []structureArgRow, inputs []*skillmodel.Input) ([]structureArgRow, []structureInputRow) {
	byName := map[string]*structureArgRow{}
	for index := range rows {
		byName[rows[index].Name] = &rows[index]
	}
	var session []structureInputRow
	for _, input := range inputs {
		label := input.Atom
		if input.Member != "" {
			label = input.Atom + " = " + input.Member
		}
		if input.Source == skillmodel.InputSourceSession {
			tile := structureInputRow{Id: input.Id, Atom: input.Atom, Label: label}
			session = append(session, tile)
			continue
		}
		row, isRow := byName[input.Label]
		if !isRow {
			continue
		}
		row.Input = input.Id
		grant := structureGrant{Atom: input.Atom, Label: label}
		row.Grants = append(row.Grants, grant)
	}
	return rows, session
}

// structureTier uses the words the model file declares.
func structureTier(path string, skillModel *skillgraph.Skill) string {
	if skillModel.Tier != skillgraph.TierStrict {
		return "tier: annotations"
	}
	queries, _ := filepath.Glob(filepath.Join(path, "queries", "*.json"))
	return fmt.Sprintf("tier: strict — %d atoms, %d queries", len(skillModel.Atoms), len(queries))
}

// structureNotes flags spawns without substeps: spawned work the skill leaves opaque.
func structureNotes(projection *skillmodel.Projection, entries []skillgraph.EntryRef) map[string][]string {
	notes := map[string][]string{}
	for _, state := range projection.States {
		if state.Kind != skillgraph.SectionKindContext {
			continue
		}
		for _, entry := range state.Entries {
			if isStructureFlowRole(entry.Role) && entry.Role != roleStop {
				notes[entry.Id] = append(notes[entry.Id], fmt.Sprintf("flow entry %s sits in context section %q", entry.Role, state.Title))
			}
		}
	}
	for index, ref := range entries {
		if ref.Entry.Role != roleSpawn {
			continue
		}
		substeps := 0
		for _, follower := range entries[index+1:] {
			if follower.Section != ref.Section || follower.Entry.Role != roleSubstep {
				break
			}
			substeps++
		}
		if substeps == 0 {
			notes[ref.Entry.Id] = append(notes[ref.Entry.Id], "spawn declares no substeps — the spawned work is opaque in this skill")
		}
	}
	// Flow-order check: an atom consumed by a flow/stop pred before its earliest grant fires.
	position := 0
	firstGrant := map[string]int{}
	grantId := map[string]string{}
	type consumption struct {
		entryId  string
		atom     string
		position int
	}
	var consumptions []consumption
	for _, state := range projection.States {
		if state.Kind != skillgraph.SectionKindPhase && state.Kind != "" {
			continue
		}
		for _, entry := range state.Entries {
			if !isStructureFlowRole(entry.Role) {
				continue
			}
			position++
			for _, token := range atomTokenPattern.FindAllString(entry.Pred, -1) {
				consumptions = append(consumptions, consumption{entryId: entry.Id, atom: token, position: position})
			}
			for _, atom := range entry.Post {
				if _, seen := firstGrant[atom]; !seen {
					firstGrant[atom] = position
					grantId[atom] = entry.Id
				}
			}
		}
	}
	for _, c := range consumptions {
		if grantPos, granted := firstGrant[c.atom]; granted && grantPos > c.position {
			notes[c.entryId] = append(notes[c.entryId], fmt.Sprintf("%s consumed before its grant — %s fires later in the flow", c.atom, grantId[c.atom]))
		}
	}
	// Ref discipline: cross-section sequencing belongs to the condition system; routes are exempt toward phases.
	stateOf := map[string]skillmodel.ProjectionState{}
	entryIn := map[string]skillmodel.ProjectionState{}
	roleOf := map[string]string{}
	for _, state := range projection.States {
		for _, entry := range state.Entries {
			stateOf[strings.ToLower(entry.Id)] = state
			entryIn[entry.Id] = state
			roleOf[strings.ToLower(entry.Id)] = entry.Role
		}
	}
	for _, state := range projection.States {
		for _, entry := range state.Entries {
			for _, ref := range entry.Refs {
				key := strings.ToLower(ref)
				targetRole, known := roleOf[key]
				if !known || !isStructureFlowRole(targetRole) {
					continue
				}
				if entry.Role == roleDef || entry.Role == rolePayload {
					notes[entry.Id] = append(notes[entry.Id], fmt.Sprintf("%s %s references flow entry %s — defs and payloads are leaves", entry.Role, entry.Id, ref))
					continue
				}
				target := stateOf[key]
				if target.Id == entryIn[entry.Id].Id {
					continue
				}
				if entryIn[entry.Id].Kind == skillgraph.SectionKindRoute && (target.Kind == skillgraph.SectionKindPhase || target.Kind == "") {
					continue
				}
				notes[entry.Id] = append(notes[entry.Id], fmt.Sprintf("ref to %s crosses sections — sequencing belongs to the condition system", ref))
			}
		}
	}
	return notes
}

// buildStructureStateView places a route marker before the first entry its route replaces.
func buildStructureStateView(notes map[string][]string, scenario *graphScenario, state skillmodel.ProjectionState, titleStates map[string]string) structureStateView {
	view := structureStateView{Id: state.Id, Kind: state.Kind, Pred: state.Pred, Status: scenario.States[state.Id], Title: state.Title}
	counts := map[string]int{}
	slots := map[string][]structureEntryRow{}
	sequence := 0
	for _, entry := range state.Entries {
		counts[entry.Role]++
		slot := structureSlot(entry.Role)
		row := structureEntryRow{
			Channel:    entryChannel(entry.Channel),
			Gate:       entry.Gate,
			Hook:       entry.Hook,
			Id:         entry.Id,
			Indent:     entry.Role == roleSubstep,
			Post:       strings.Join(entry.Post, " and "),
			Pred:       entry.Pred,
			RefByCount: len(entry.RefBy),
			RefCount:   len(entry.Refs),
			ReplacedBy: scenario.Replaced[entry.Id],
			Role:       entry.Role,
			Schema:     entry.Schema,
			Script:     entry.Script,
			Status:     scenario.Entries[entry.Id],
			Stop:       entry.Role == roleStop,
			Subsection: entry.Subsection,
			Title:      entry.Title,
			TitleState: titleStates[entry.Id],
			Warnings:   strings.Join(notes[entry.Id], "; "),
		}
		row.Custom = strings.Join(routeCustomChannels(&row, entry.Custom), ", ")
		if row.Script != "" {
			row.ScriptShort = row.Script[strings.LastIndex(row.Script, "/")+1:]
		}
		if slot == slotFlow && entry.Role != roleStop {
			// Stops get no sequence number: a guarded exit is not an action in the sequence.
			sequence++
			row.Seq = sequence
		}
		slots[slot] = append(slots[slot], row)
	}
	var headerCounts []string
	for _, role := range structureCountRoles {
		if counts[role] > 0 {
			headerCounts = append(headerCounts, fmt.Sprintf("%d %s", counts[role], role))
		}
	}
	view.RoleCounts = strings.Join(headerCounts, " · ")
	for _, slot := range []string{slotDefs, slotRules, slotQuality, slotFlow, slotPayloads} {
		if len(slots[slot]) == 0 {
			continue
		}
		view.Compartments = append(view.Compartments, structureCompartment{Flow: slot == slotFlow, Label: slot, Rows: slots[slot]})
	}
	return view
}

// routeCustomChannels treats `gate:<v>`-style tags as channels: grammar-custom, semantically channels.
func routeCustomChannels(row *structureEntryRow, custom []string) []string {
	var rest []string
	for _, tag := range custom {
		switch {
		case strings.HasPrefix(tag, "gate:") && row.Gate == "":
			row.Gate = strings.TrimPrefix(tag, "gate:")
		case strings.HasPrefix(tag, "hook:") && row.Hook == "":
			row.Hook = strings.TrimPrefix(tag, "hook:")
		case strings.HasPrefix(tag, "schema:") && row.Schema == "":
			row.Schema = strings.TrimPrefix(tag, "schema:")
		case strings.HasPrefix(tag, "script:") && row.Script == "":
			row.Script = strings.TrimPrefix(tag, "script:")
		default:
			rest = append(rest, tag)
		}
	}
	return rest
}

// entryChannel hides the script channel: the JS badge already carries it.
func entryChannel(channel string) string {
	if channel == channelProse || channel == "script" {
		return ""
	}
	return channel
}

// structureSlot maps a role onto its card compartment.
func structureSlot(role string) string {
	switch role {
	case roleDef:
		return slotDefs
	case roleRule:
		return slotRules
	case roleQuality:
		return slotQuality
	case rolePayload:
		return slotPayloads
	}
	return slotFlow
}

// isStructureFlowRole mirrors the projection's flow-role set.
func isStructureFlowRole(role string) bool {
	switch role {
	case roleSpawn, roleStep, roleStop, roleSubstep, roleVerify:
		return true
	}
	return false
}

// anchorRow treats an anchor naming no agent as matching the root.
func anchorRow(anchor *skillgraph.Anchor, id string, usedBy []string) graphAnchorRow {
	matches := make([]string, 0, len(anchor.Input)+len(anchor.Exclude))
	for _, match := range anchor.Input {
		matches = append(matches, match.Field+" ~ "+match.Pattern)
	}
	for _, match := range anchor.Exclude {
		matches = append(matches, "not "+match.Field+" ~ "+match.Pattern)
	}
	captures := make([]string, 0, len(anchor.Capture))
	for _, capture := range anchor.Capture {
		captures = append(captures, capture.Name+" = "+capture.Pointer)
	}
	agent := anchor.Agent
	if agent == "" {
		agent = skillgraph.AnchorAgentRoot
	}
	row := graphAnchorRow{
		Id:        id,
		Agent:     agent,
		Captures:  captures,
		Match:     matches,
		Tool:      anchor.Tool,
		ToolLabel: enforceToolLabel(anchor.Tool),
		UsedBy:    strings.Join(usedBy, ","),
	}
	return row
}

// atomRows marks the read-or-delete violation the audit reports: a skill-local atom nothing reads.
func atomRows(skillModel *skillgraph.Skill) []graphAtomRow {
	grants, reads := atomUsage(skillModel)
	atoms := skillModel.AllAtoms()
	rows := make([]graphAtomRow, 0, len(atoms))
	for _, id := range slices.Sorted(maps.Keys(atoms)) {
		atom := atoms[id]
		_, isLocal := skillModel.Atoms[id]
		row := graphAtomRow{
			Id:        id,
			GrantedBy: grants[id],
			IsShared:  !isLocal,
			IsUnread:  isLocal && reads[id] == 0,
			Kind:      atom.EffectiveKind(),
			ReadBy:    reads[id],
			Sort:      atom.Sort,
			Source:    atom.Source,
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(a, b int) bool { return !rows[a].IsShared && rows[b].IsShared })
	return rows
}

// atomUsage counts grants from posts and reads through skillgraph.AtomReaders, the one pred scanner.
func atomUsage(skillModel *skillgraph.Skill) (map[string]int, map[string]int) {
	grants := map[string]int{}
	reads := map[string]int{}
	for _, ref := range skillModel.Entries() {
		for _, atom := range ref.Entry.Post {
			grants[atom]++
		}
	}
	for id, readers := range skillgraph.AtomReaders(skillModel) {
		reads[id] = len(readers)
	}
	return grants, reads
}

// contextEntries links exact ids only; a glob gets no link.
func contextEntries(declaration string) []graphRefItem {
	var items []graphRefItem
	for id := range strings.SplitSeq(declaration, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}

		item := graphRefItem{Name: id}
		if !strings.HasSuffix(id, "*") {
			item.Href = "/context/entry/" + url.PathEscape(id)
		}
		items = append(items, item)
	}
	return items
}

// enforceToolLabel reads ^(Edit|Write)$ as "Edit or Write"; any other pattern stays as written.
func enforceToolLabel(pattern string) string {
	match := enforceToolPattern.FindStringSubmatch(pattern)
	if match == nil {
		return pattern
	}
	return strings.Join(strings.Split(match[1], "|"), " or ")
}

// enforceToolPattern matches an anchored alternation of tool names.
var enforceToolPattern = regexp.MustCompile(`^\^\(?([A-Za-z]+(?:\|[A-Za-z]+)*)\)?\$$`)

// enforceView prefixes UsedBy ids with check: or judgment: because the two id spaces are separate.
func enforceView(enforce *skillgraph.Enforce) *graphEnforceView {
	if enforce == nil {
		return nil
	}

	usedBy := map[string][]string{}
	view := &graphEnforceView{Gate: gateRows(enforce.Gate)}
	for _, id := range slices.Sorted(maps.Keys(enforce.Checks)) {
		check := enforce.Checks[id]
		row := graphCheckRow{Id: id, Anchors: check.AnchorRefs(), Kind: check.Kind, Reason: check.Reason}
		view.Checks = append(view.Checks, row)
		user := "check:" + id
		for _, ref := range row.Anchors {
			if !slices.Contains(usedBy[ref.Anchor], user) {
				usedBy[ref.Anchor] = append(usedBy[ref.Anchor], user)
			}
		}
	}
	for _, id := range slices.Sorted(maps.Keys(enforce.Judgment)) {
		point := enforce.Judgment[id]
		row := graphJudgmentRow{Id: id, Anchor: point.Anchor, Evidence: strings.Join(point.Evidence, ", "), Question: point.Question}
		view.Judgment = append(view.Judgment, row)
		usedBy[point.Anchor] = append(usedBy[point.Anchor], "judgment:"+id)
	}
	for _, id := range slices.Sorted(maps.Keys(enforce.Anchors)) {
		row := anchorRow(enforce.Anchors[id], id, usedBy[id])
		view.Anchors = append(view.Anchors, row)
	}
	return view
}

// entryNotes maps each entry carrying a note to it.
func entryNotes(entries []skillgraph.EntryRef) map[string]string {
	notes := map[string]string{}
	for _, ref := range entries {
		if ref.Entry.Note != "" {
			notes[ref.Entry.Id] = ref.Entry.Note
		}
	}
	return notes
}

// entryTitles maps every entry id to its title for reader labels.
func entryTitles(entries []skillgraph.EntryRef) map[string]string {
	titles := make(map[string]string, len(entries))
	for _, ref := range entries {
		titles[ref.Entry.Id] = ref.Entry.Title
	}
	return titles
}

func entryWarnings(entries []skillmodel.ProjectionEntry, notes map[string][]string) []graphWarning {
	var warnings []graphWarning
	for _, entry := range entries {
		for _, message := range notes[entry.Id] {
			warning := graphWarning{Kind: warningKindEntry, Message: message, Target: entry.Id, Title: entry.Title}
			warnings = append(warnings, warning)
		}
	}
	return warnings
}

func envState(value *skills.VariableValue, isRequired bool) string {
	switch {
	case value.IsDeferred:
		return envStateDeferred
	case value.Value != "":
		return envStateSet
	case isRequired:
		return envStateUnsetRequired
	}
	return envStateUnsetOptional
}

func gateRows(gate *skillgraph.GateRules) []graphGateRow {
	if gate == nil {
		return nil
	}

	writers := make([]string, 0, len(gate.Writers))
	for _, writer := range gate.Writers {
		writers = append(writers, writer.Glob+" by "+strings.Join(writer.AgentTypes, ", "))
	}
	rows := []graphGateRow{
		{Label: gateLabelPlanModeDenied, Sentence: "denied in plan mode", Values: gate.PlanModeDenied},
		{Label: gateLabelRootDenied, Sentence: "denied at the root", Values: gate.RootDenied},
		{Label: gateLabelWorkflowOnly, Sentence: "allowed inside a Workflow only", Values: gate.WorkflowOnly},
		{Label: gateLabelWriters, Sentence: "written only by", Values: writers},
	}
	return slices.DeleteFunc(rows, func(row graphGateRow) bool { return len(row.Values) == 0 })
}

func graphBase(skill *skills.Skill) string {
	return "/scripts/skills/" + url.PathEscape(skill.Origin) + "/" + url.PathEscape(skill.Name) + "/graph"
}

// graphEnvRows is one row per declared variable in key order: the resolved value's state and every reader.
func graphEnvRows(model *skillgraph.Skill, titles map[string]string, values []skills.VariableValue) []structureEnvRow {
	byName := make(map[string]skills.VariableValue, len(values))
	for _, value := range values {
		byName[value.Name] = value
	}
	keys := slices.Sorted(maps.Keys(model.Env))
	rows := make([]structureEnvRow, 0, len(keys))
	for _, key := range keys {
		name := skillgraph.VariableName(model.Name(), key)
		value := byName[name]
		row := structureEnvRow{Cause: value.Cause, IsDeferred: value.IsDeferred, IsRequired: model.Env[key].Required, Key: key, Name: name, Source: value.Source, Value: value.Value}
		if value.Interface != nil {
			row.Interface = value.Interface.Ref()
			row.Scope = value.Interface.Scope
			row.Summary = value.Interface.Summary
		}
		row.State = envState(&value, row.IsRequired)
		row.Mark = envMarks[row.State]
		reads := skillgraph.VariableReads(model, name)
		for _, read := range reads {
			reader := structureReader{Id: read.Id, Kind: read.Kind, Label: readerLabel(read, titles)}
			row.Readers = append(row.Readers, reader)
		}
		rows = append(rows, row)
	}
	return rows
}

func graphWarnings(args []structureArgRow, atoms []graphAtomRow, notes map[string][]string, projection *skillmodel.Projection, views map[string]structureStateView) []graphWarning {
	var warnings []graphWarning
	for _, state := range projection.States {
		if state.Kind == "halt" {
			continue
		}

		for _, message := range views[state.Id].Warnings {
			warning := graphWarning{Kind: warningKindSection, Message: message, Target: state.Id, Title: state.Title}
			warnings = append(warnings, warning)
		}
		warnings = append(warnings, entryWarnings(state.Entries, notes)...)
	}
	for _, atom := range atoms {
		if !atom.IsUnread {
			continue
		}

		warning := graphWarning{Kind: warningKindAtom, Message: warningUnreadAtom, Target: atom.Id, Title: atom.Id}
		warnings = append(warnings, warning)
	}
	for _, arg := range args {
		if len(arg.Owners) > 0 {
			continue
		}

		warning := graphWarning{Kind: warningKindArg, Message: warningNoOwner, Target: arg.Name, Title: arg.Name}
		warnings = append(warnings, warning)
	}
	return warnings
}

// readerLabel is the entry's title for an entry or param reader, the id itself for a grant, anchor or check.
func readerLabel(read skillgraph.VariableRead, titles map[string]string) string {
	if title, isEntry := titles[read.Id]; isEntry {
		return title
	}
	return read.Id
}

func referenceGroups(projection *skillmodel.Projection) []graphRefGroup {
	groups := []graphRefGroup{
		resourceGroup(skillmodel.ResourceKindSubagentType, refLabelSubagentTypes, projection.Resources),
		resourceGroup(skillmodel.ResourceKindWorkflow, refLabelWorkflows, projection.Resources),
		resourceGroup(skillmodel.ResourceKindSchema, refLabelSchemas, projection.Resources),
		resourceGroup(skillmodel.ResourceKindScript, refLabelScripts, projection.Resources),
	}
	return slices.DeleteFunc(groups, func(group graphRefGroup) bool { return len(group.Items) == 0 })
}

// resourceEdgeKind names the edge from an entry to a resource of the kind.
func resourceEdgeKind(kind string) string {
	switch kind {
	case skillmodel.ResourceKindSubagentType, skillmodel.ResourceKindWorkflow:
		return edgeKindSpawns
	case skillmodel.ResourceKindSchema:
		return edgeKindUsesSchema
	case skillmodel.ResourceKindScript:
		return edgeKindScript
	}
	return ""
}

func resourceGroup(kind, label string, resources []skillmodel.ProjectionResource) graphRefGroup {
	group := graphRefGroup{EdgeKind: resourceEdgeKind(kind), Label: label}
	for _, resource := range resources {
		if resource.Kind != kind {
			continue
		}

		item := graphRefItem{Count: len(resource.Entries), EntryIds: strings.Join(resource.Entries, ","), Id: resource.Id, Name: resource.Label}
		group.Items = append(group.Items, item)
	}
	return group
}

func sectionNotes(projection *skillmodel.Projection, skillModel *skillgraph.Skill) map[string]string {
	stateIds := map[string]string{}
	for _, state := range projection.States {
		stateIds[state.Title] = state.Id
	}
	notes := map[string]string{}
	for _, section := range skillModel.Sections {
		id := stateIds[section.Title]
		if section.Note != "" && id != "" {
			notes[id] = section.Note
		}
	}
	return notes
}

// skillOptionValue is the picker value naming a skill: `<origin>/<name>`.
func skillOptionValue(skill *skills.Skill) string {
	return skill.Origin + "/" + skill.Name
}

// titleStateEntries maps each gate entry to the state it presents.
func titleStateEntries(view *graphTitleView) map[string]string {
	entries := map[string]string{}
	if view == nil {
		return entries
	}
	for _, row := range view.States {
		entries[row.Entry] = row.State
	}
	return entries
}

func titleStatesView(skillModel *skillgraph.Skill) *graphTitleView {
	if skillModel.Title == nil {
		return nil
	}

	view := &graphTitleView{}
	for _, state := range skillModel.Title.Tokens() {
		declared := skillModel.Title.States[string(state)]
		row := graphTitleRow{Command: sessiontitle.SetCommand(state), Entry: declared.Entry, Meaning: declared.Meaning, State: string(state)}
		if ref, isKnown := skillModel.Entry(declared.Entry); isKnown {
			row.EntryTitle = ref.Entry.Title
		}
		view.States = append(view.States, row)
	}
	return view
}

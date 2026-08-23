package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/kevinhorst/smine/internal/acdsl"
	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/reach"
	"github.com/kevinhorst/smine/internal/server/respond"
)

const (
	pageContext      = "context"
	tmplAspectBar    = "_aspect_bar.html"
	tmplContextDoc   = "context_doc.html"
	tmplContextIndex = "context_index.html"
	// contextSyncLockKey serializes the context sync via the repo-locks map,
	// like skillsSyncLockKey: the underscore prefix keeps it out of the
	// registry's repo-name space.
	contextSyncLockKey = "_context-sync"
)

type contextDocPage struct {
	Body  template.HTML
	Dir   string
	File  string
	Page  string
	Title string
}

type contextIndexPage struct {
	Acdsl          acdslPage
	AgentsTemplate string
	AspectBar      aspectBarFragment
	Error          string
	Filter         contextFilter
	Page           string
	RepoNames      []string
	Sections       []entryKindSection
	ShowProse      bool
	Title          string
}

// contextFilter is the one projection the Context page renders under; every
// axis comes from the query string and combines with the others.
type contextFilter struct {
	Aspect   string // scope chip
	Layer    string // "", layerAcdsl, or an entry kind (action / rule / fact)
	Lifetime string // "", "task", "doctrine" — acdsl rules only
	Reach    string // reach chip
	Topic    string
}

// layerAcdsl is the layer chip selecting the gate system instead of a prose
// entry kind.
const layerAcdsl = "acdsl"

// Active reports whether any axis narrows the page — the switch between the
// collapsed overview and the hide+uncollapse projection (plan D7).
func (f contextFilter) Active() bool {
	return f.Aspect != "" || f.Layer != "" || f.Lifetime != "" || f.Reach != "" || f.Topic != ""
}

// URL renders /context under this filter — the single builder behind every
// chip link (plan D6).
func (f contextFilter) URL() string {
	query := url.Values{}
	axes := map[string]string{
		"aspect":   f.Aspect,
		"layer":    f.Layer,
		"lifetime": f.Lifetime,
		"reach":    f.Reach,
		"topic":    f.Topic,
	}
	for axis, value := range axes {
		if value != "" {
			query.Set(axis, value)
		}
	}
	if len(query) == 0 {
		return "/context"
	}
	return "/context?" + query.Encode()
}

// with returns the filter with one axis overridden. Clearing or changing the
// scope also clears the topic — a topic only means something inside its scope.
func (f contextFilter) with(axis, value string) contextFilter {
	next := f
	switch axis {
	case "aspect":
		next.Aspect = value
		next.Topic = ""
	case "layer":
		next.Layer = value
	case "lifetime":
		next.Lifetime = value
	case "reach":
		next.Reach = value
	case "topic":
		next.Topic = value
	}
	return next
}

func parseContextFilter(r *http.Request) contextFilter {
	query := r.URL.Query()
	filter := contextFilter{
		Aspect:   query.Get("aspect"),
		Layer:    query.Get("layer"),
		Lifetime: query.Get("lifetime"),
		Reach:    query.Get("reach"),
		Topic:    query.Get("topic"),
	}
	return filter
}

// chipRowCap bounds the topic chips shown in a collapsed summary row;
// anything beyond it hides behind a client-side "+N" toggle.
const chipRowCap = 4

// topicChip is one per-scope topic filter in a group's summary row.
type topicChip struct {
	Name  string
	Count int
	URL   string
}

// entryScopeGroup is one collapsible scope section of entry cards. Without an
// active filter every group renders closed; any active filter renders only
// matching groups, open. Coverage counts the group's entries cited by at
// least one acdsl rule, over the unfiltered total.
type entryScopeGroup struct {
	Scope           string
	Open            bool
	AllURL          string
	Topics          []topicChip
	ExtraTopics     int
	Covered         int
	Total           int
	CoveragePercent string
	Entries         []registryEntryView
}

// entryKindSection is one kind heading (actions / rules / facts) with its
// scope groups.
type entryKindSection struct {
	Kind   string
	Groups []entryScopeGroup
}

// aspectBarEntry is one chip in the shared filter+manage bar: a taxonomy
// member (acdsl rule scopes and prose entries share the applies-to axis), a
// reach value, a layer, or a lifetime. Registered is false for a scope
// derived from rule IDs but missing from the taxonomy; Counts marks the rows
// that display rule/entry counts.
type aspectBarEntry struct {
	Active     bool
	Counts     bool
	EntryCount int
	Name       string
	Registered bool
	RuleCount  int
	Scope      string
	URL        string
}

// aspectBarFragment backs _aspect_bar.html, rendered once on the Context
// page. Four axes share the one filter card: reach, layer + lifetime, and
// scope. Topic filtering lives in each scope group's summary row, not here.
type aspectBarFragment struct {
	Aspects   []aspectBarEntry
	Layers    []aspectBarEntry
	Lifetimes []aspectBarEntry
	Reaches   []aspectBarEntry
	Filter    contextFilter
	// Bulk is the reach picker in the reach row — its checkboxes and submit
	// button post the listing's bulk form via the HTML form attribute.
	Bulk  reachPicker
	Error string
}

// registryEntry is one generated context.json entry in its structured form.
type registryEntry struct {
	Id          string               `json:"id"`
	Kind        string               `json:"kind"`
	Scope       string               `json:"scope"`
	Topic       string               `json:"topic"`
	Content     registryEntryContent `json:"content"`
	Enforcement string               `json:"enforcement"`
	Reach       string               `json:"reach"`
	Version     string               `json:"version"`
	Source      string               `json:"source"`
}

// registryEntryContent is the nested content object of a context.json entry;
// only the statement is read here.
type registryEntryContent struct {
	Statement string `json:"statement"`
}

// registryEntryView adds the reverse coverage map: which acdsl rules cite this
// entry in their why — the loop connection made visible.
type registryEntryView struct {
	Entry   registryEntry
	GatedBy []string
}

// aspectNamePattern bounds UI-created aspect names: short, upper-case,
// letters only — matching the entry-ID grammar's ASPECT segment.
var aspectNamePattern = regexp.MustCompile(`^[A-Z]{2,12}$`)

// contextDirPattern bounds the sync context-dir: a single path segment of
// letters, digits, underscore or dash — no slashes, so the value can never
// traverse or reach the sync script's sed as an injection primitive.
var contextDirPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// enforcementTitle explains an enforcement token in plain language — the
// tokens are contract vocabulary, the tooltip is where they get defined.
func enforcementTitle(enforcement string) string {
	switch enforcement {
	case "gate":
		return "checked mechanically by the acdsl gate — a violation blocks"
	case "hook":
		return "enforced by a Claude Code hook at tool-call time"
	case "lint":
		return "enforced by a linter or formatter"
	case "review":
		return "checked by a human or agent during review — no mechanical check"
	case "manual":
		return "followed by hand — no mechanical or review checkpoint"
	}
	return ""
}

// sourceDocHref maps an entry's recorded source (context/actions/x.md) to the
// doc-view route segment (actions/x.md) — the last two path segments.
func sourceDocHref(source string) string {
	parts := strings.Split(source, "/")
	if len(parts) < 2 {
		return source
	}
	return parts[len(parts)-2] + "/" + parts[len(parts)-1]
}

// folderPick backs the _repo_path.html picker fragment.
type folderPick struct {
	Error string
	Value string
}

func (s *Server) handleContextIndex(w http.ResponseWriter, r *http.Request) {
	filter := parseContextFilter(r)
	data := contextIndexPage{
		Error:     r.URL.Query().Get("err"),
		Filter:    filter,
		Page:      pageContext,
		ShowProse: filter.Layer != layerAcdsl,
		Title:     "Context",
	}
	data.Acdsl = s.buildAcdslPage(r)

	// A missing template renders as an empty baseline (fresh checkout, or a
	// repo without the prose pack) — mirrors loadAspectsTolerant.
	agents, err := os.ReadFile(filepath.Join(s.contextDir, "AGENTS.md"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		respond.WithInternalServerError(err, w)
		return
	}
	data.AgentsTemplate = string(agents)

	discovery, err := acdsl.DiscoverRules(r.Context(), ".")
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	data.AspectBar = s.buildAspectBar(discovery.Rules, filter)

	gatedBy := map[string][]string{}
	for _, rule := range discovery.Rules {
		for _, id := range entryIdRe.FindAllString(rule.Why, -1) {
			gatedBy[id] = append(gatedBy[id], rule.Id)
		}
	}
	var views []registryEntryView
	for _, entry := range s.loadRegistryEntries() {
		if !reachMatches(entry.Reach, filter.Reach) {
			continue
		}
		views = append(views, registryEntryView{Entry: entry, GatedBy: gatedBy[entry.Id]})
	}
	data.Sections = buildEntrySections(views, filter)

	// The sync widget's target list always loads.
	for _, repo := range s.repoRegistry.Repos() {
		data.RepoNames = append(data.RepoNames, repo.Name)
	}
	s.renderFragment(w, tmplContextIndex, data)
}

// buildEntrySections buckets entry views into the fixed kind order (actions,
// rules, facts) with sorted collapsible scope groups per kind — presentation
// grouping happens here, the template only ranges (mirrors skillGroups).
// Without an active filter every group renders closed (the overview); any
// active filter renders only matching kinds/scopes/topics, open (plan D7).
func buildEntrySections(views []registryEntryView, filter contextFilter) []entryKindSection {
	if filter.Layer == layerAcdsl {
		return nil
	}
	byKind := map[string]map[string][]registryEntryView{}
	for _, view := range views {
		if byKind[view.Entry.Kind] == nil {
			byKind[view.Entry.Kind] = map[string][]registryEntryView{}
		}
		byKind[view.Entry.Kind][view.Entry.Scope] = append(byKind[view.Entry.Kind][view.Entry.Scope], view)
	}

	var sections []entryKindSection
	for _, kind := range []string{contextdocs.RuleKindAction, contextdocs.RuleKindRule, contextdocs.RuleKindFact} {
		if filter.Layer != "" && filter.Layer != kind {
			continue
		}
		byScope := byKind[kind]
		if len(byScope) == 0 {
			continue
		}
		scopes := make([]string, 0, len(byScope))
		for scope := range byScope {
			scopes = append(scopes, scope)
		}
		sort.Strings(scopes)
		section := entryKindSection{Kind: kind}
		for _, scope := range scopes {
			if filter.Aspect != "" && filter.Aspect != scope {
				continue
			}
			group := buildEntryScopeGroup(scope, byScope[scope], filter)
			if len(group.Entries) == 0 {
				continue
			}
			section.Groups = append(section.Groups, group)
		}
		if len(section.Groups) == 0 {
			continue
		}
		sections = append(sections, section)
	}
	return sections
}

// buildEntryScopeGroup assembles one scope group under the filter: topic
// chips carry their link, coverage counts the full scope, and an active
// filter narrows the entries and opens the group.
func buildEntryScopeGroup(scope string, views []registryEntryView, filter contextFilter) entryScopeGroup {
	group := entryScopeGroup{Scope: scope, Open: filter.Active(), Topics: entryTopicChips(views)}
	scoped := filter.with("aspect", scope)
	group.AllURL = scoped.URL()
	for index := range group.Topics {
		group.Topics[index].URL = scoped.with("topic", group.Topics[index].Name).URL()
	}
	if len(group.Topics) > chipRowCap {
		group.ExtraTopics = len(group.Topics) - chipRowCap
	}
	// The topic narrows only groups that actually carry it — a same-scope
	// group in another kind keeps its full list.
	hasTopic := false
	for _, view := range views {
		if view.Entry.Topic == filter.Topic {
			hasTopic = true
		}
		if len(view.GatedBy) > 0 {
			group.Covered++
		}
	}
	group.Total = len(views)
	group.CoveragePercent = fmt.Sprintf("%.0f%%", 100*float64(group.Covered)/float64(group.Total))
	for _, view := range views {
		if filter.Topic != "" && hasTopic && view.Entry.Topic != filter.Topic {
			continue
		}
		group.Entries = append(group.Entries, view)
	}
	return group
}

// entryTopicChips counts a scope group's entries per topic, name-sorted.
func entryTopicChips(views []registryEntryView) []topicChip {
	counts := map[string]int{}
	for _, view := range views {
		if view.Entry.Topic != "" {
			counts[view.Entry.Topic]++
		}
	}
	return sortedTopicChips(counts)
}

// sortedTopicChips renders a topic count map as name-sorted chips.
func sortedTopicChips(counts map[string]int) []topicChip {
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)
	chips := make([]topicChip, 0, len(names))
	for _, name := range names {
		chips = append(chips, topicChip{Name: name, Count: counts[name]})
	}
	return chips
}

// loadRegistryEntries reads the generated context file; a missing or broken
// file renders as an empty list — the generation gates own its integrity.
func (s *Server) loadRegistryEntries() []registryEntry {
	raw, err := os.ReadFile(filepath.Join(s.contextDir, "context.json"))
	if err != nil {
		return nil
	}
	var parsed struct {
		Entries []registryEntry `json:"entries"`
	}
	if json.Unmarshal(raw, &parsed) != nil {
		return nil
	}
	return parsed.Entries
}

// reachChips maps a reach value to its filter chips: "global" is its own
// chip; a name list contributes one chip per name (exact-name semantics —
// a chip answers "what is deliberately scoped here", never coverage).
func reachChips(value string) []string {
	if value == reach.Global {
		return []string{reach.Global}
	}
	return reach.Names(value)
}

// reachMatches reports whether a rule/entry with this reach value belongs
// under the given chip. Empty chip = no reach filter.
func reachMatches(value, chip string) bool {
	if chip == "" {
		return true
	}
	for _, name := range reachChips(value) {
		if name == chip {
			return true
		}
	}
	return false
}

// reachBarEntries orders the repo-section chips: global, smine, then the
// remaining names sorted.
func reachBarEntries(ruleCounts, entryCounts map[string]int) []aspectBarEntry {
	names := map[string]bool{}
	for name := range ruleCounts {
		names[name] = true
	}
	for name := range entryCounts {
		names[name] = true
	}
	var rest []string
	for name := range names {
		if name != reach.Global && name != reach.ThisRepo {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	ordered := make([]string, 0, len(names))
	if names[reach.Global] {
		ordered = append(ordered, reach.Global)
	}
	if names[reach.ThisRepo] {
		ordered = append(ordered, reach.ThisRepo)
	}
	ordered = append(ordered, rest...)

	chips := make([]aspectBarEntry, 0, len(ordered))
	for _, name := range ordered {
		chips = append(chips, aspectBarEntry{
			Counts:     true,
			Name:       name,
			Registered: true,
			RuleCount:  ruleCounts[name],
			EntryCount: entryCounts[name],
		})
	}
	return chips
}

// allChip is the axis-clearing chip leading every filter row.
func allChip(filter contextFilter, axis, active string) aspectBarEntry {
	chip := aspectBarEntry{Active: active == "", Name: "all", Registered: true}
	chip.URL = filter.with(axis, "").URL()
	return chip
}

// axisChips renders one fixed-value filter row (layer, lifetime): all + one
// chip per value.
func axisChips(filter contextFilter, axis, active string, values []string) []aspectBarEntry {
	chips := []aspectBarEntry{allChip(filter, axis, active)}
	for _, value := range values {
		chip := aspectBarEntry{Active: active == value, Name: value, Registered: true}
		chip.URL = filter.with(axis, value).URL()
		chips = append(chips, chip)
	}
	return chips
}

// buildAspectBar merges the registered scope vocabulary with the acdsl rule
// scopes and counts both populations per member. Scope counts are taken
// under the active reach, and scopes with nothing under that reach disappear
// (plan D8); topic chips live in the scope-group summaries. Scopes derived
// from rule IDs but missing from the taxonomy surface as unregistered —
// visible, never hidden.
func (s *Server) buildAspectBar(rules []acdsl.Rule, filter contextFilter) aspectBarFragment {
	bar := aspectBarFragment{Filter: filter, Bulk: s.buildReachPicker("", "bulk-reach")}
	bar.Bulk.Label = "bulk reach"
	aspects, err := s.loadAspectsTolerant()
	if err != nil {
		bar.Error = err.Error()
		return bar
	}

	entries := s.loadRegistryEntries()

	ruleCounts := map[string]int{}
	reachRuleCounts := map[string]int{}
	for _, rule := range rules {
		for _, chip := range reachChips(rule.Reach) {
			reachRuleCounts[chip]++
		}
		if !reachMatches(rule.Reach, filter.Reach) {
			continue
		}
		if scope := ruleScope(rule.Id); scope != "" {
			ruleCounts[scope]++
		}
	}
	entryCounts := map[string]int{}
	reachEntryCounts := map[string]int{}
	for _, entry := range entries {
		for _, chip := range reachChips(entry.Reach) {
			reachEntryCounts[chip]++
		}
		if !reachMatches(entry.Reach, filter.Reach) {
			continue
		}
		entryCounts[entry.Scope]++
	}
	bar.Reaches = append([]aspectBarEntry{allChip(filter, "reach", filter.Reach)},
		reachBarEntries(reachRuleCounts, reachEntryCounts)...)
	for index := range bar.Reaches {
		if bar.Reaches[index].Name == "all" {
			continue
		}
		bar.Reaches[index].Active = bar.Reaches[index].Name == filter.Reach
		bar.Reaches[index].URL = filter.with("reach", bar.Reaches[index].Name).URL()
	}

	bar.Layers = axisChips(filter, "layer", filter.Layer,
		[]string{layerAcdsl, contextdocs.RuleKindAction, contextdocs.RuleKindRule, contextdocs.RuleKindFact})
	bar.Lifetimes = axisChips(filter, "lifetime", filter.Lifetime,
		[]string{acdsl.LifetimeTask, acdsl.LifetimeDoctrine})

	bar.Aspects = append(bar.Aspects, allChip(filter, "aspect", filter.Aspect))
	registered := map[string]bool{}
	for _, aspect := range aspects {
		registered[aspect.Name] = true
		if aspect.Class != "scope" {
			continue
		}
		if filter.Reach != "" && ruleCounts[aspect.Name] == 0 && entryCounts[aspect.Name] == 0 {
			continue
		}
		bar.Aspects = append(bar.Aspects, aspectBarEntry{
			Active:     aspect.Name == filter.Aspect,
			Counts:     true,
			Name:       aspect.Name,
			Scope:      aspect.Scope,
			Registered: true,
			RuleCount:  ruleCounts[aspect.Name],
			EntryCount: entryCounts[aspect.Name],
			URL:        filter.with("aspect", aspect.Name).URL(),
		})
	}
	var unregistered []string
	for scope := range ruleCounts {
		if !registered[scope] {
			unregistered = append(unregistered, scope)
		}
	}
	sort.Strings(unregistered)
	for _, scope := range unregistered {
		bar.Aspects = append(bar.Aspects, aspectBarEntry{
			Active:    scope == filter.Aspect,
			Counts:    true,
			Name:      scope,
			RuleCount: ruleCounts[scope],
			URL:       filter.with("aspect", scope).URL(),
		})
	}
	return bar
}

// contextEntryPage backs context_entry.html — one entry with its reverse
// coverage (mirrors skillDetailPage).
type contextEntryPage struct {
	Entry   registryEntry
	GatedBy []string
	Picker  reachPicker
	Error   string
	Page    string
	Title   string
}

// handleContextEntry renders one entry's details page; the id must match a
// generated context.json entry by string equality (mirrors handleSkillDetail).
func (s *Server) handleContextEntry(w http.ResponseWriter, r *http.Request) {
	data, found, err := s.buildContextEntryPage(r.Context(), r.PathValue("id"))
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	s.renderFragment(w, "context_entry.html", data)
}

// buildContextEntryPage assembles one entry's detail view with its reverse
// coverage; found is false when the id matches no generated entry.
func (s *Server) buildContextEntryPage(ctx context.Context, id string) (contextEntryPage, bool, error) {
	var entry *registryEntry
	for _, candidate := range s.loadRegistryEntries() {
		if candidate.Id == id {
			entry = &candidate
			break
		}
	}
	if entry == nil {
		return contextEntryPage{}, false, nil
	}

	data := contextEntryPage{Entry: *entry, Page: pageContext, Title: entry.Id,
		Picker: s.buildReachPicker(entry.Reach, "")}
	discovery, err := acdsl.DiscoverRules(ctx, ".")
	if err != nil {
		return contextEntryPage{}, false, err
	}
	for _, rule := range discovery.Rules {
		if slices.Contains(entryIdRe.FindAllString(rule.Why, -1), entry.Id) {
			data.GatedBy = append(data.GatedBy, rule.Id)
		}
	}
	return data, true, nil
}

// handleContextEntryReach sets a prose entry's reach by rewriting its source
// markdown bullet, then regenerates context.json — the WriteContextFile
// write path, same as the aspect editor. Empty input disables (reach "none").
func (s *Server) handleContextEntryReach(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	value := reachFromForm(r)
	if !reach.Valid(value) {
		data, found, err := s.buildContextEntryPage(r.Context(), id)
		if err != nil {
			respond.WithInternalServerError(err, w)
			return
		}
		if !found {
			http.NotFound(w, r)
			return
		}
		data.Error = "invalid reach: global, none, or a comma-separated repo-name list"
		s.renderFragment(w, "context_entry.html", data)
		return
	}
	if err := contextdocs.SetEntryReach(s.contextDir, id, value); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	aspects, err := s.loadAspectsTolerant()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	if err := contextdocs.WriteContextFile(s.contextDir, aspects); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	http.Redirect(w, r, "/context/entry/"+url.PathEscape(id), http.StatusSeeOther)
}

// handleContextReachBulk sets one reach value on every checked id from the
// context page: discovered acdsl rule ids through SetRuleReach, everything
// else through SetEntryReach. context.json regenerates once when any entry
// changed. Redirects back to the page with its filter state preserved.
func (s *Server) handleContextReachBulk(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	value := reachFromForm(r)
	ids := r.Form["id"]
	backFilter := contextFilter{
		Aspect:   r.FormValue("aspect"),
		Layer:    r.FormValue("layer"),
		Lifetime: r.FormValue("lifetime"),
		Reach:    r.FormValue("reach-filter"),
		Topic:    r.FormValue("topic"),
	}
	back := backFilter.URL()
	if !reach.Valid(value) || len(ids) == 0 {
		separator := "?"
		if strings.Contains(back, "?") {
			separator = "&"
		}
		http.Redirect(w, r, back+separator+"err="+url.QueryEscape("bulk reach: pick at least one item and a valid reach"), http.StatusSeeOther)
		return
	}
	discovery, err := acdsl.DiscoverRules(r.Context(), ".")
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	ruleById := map[string]acdsl.Rule{}
	for _, rule := range discovery.Rules {
		ruleById[rule.Id] = rule
	}
	entriesChanged := false
	for _, id := range ids {
		if rule, ok := ruleById[id]; ok {
			if err := acdsl.SetRuleReach(rule.File, rule.Line, value); err != nil {
				respond.WithInternalServerError(err, w)
				return
			}
			continue
		}
		if err := contextdocs.SetEntryReach(s.contextDir, id, value); err != nil {
			respond.WithInternalServerError(err, w)
			return
		}
		entriesChanged = true
	}
	if entriesChanged {
		aspects, err := s.loadAspectsTolerant()
		if err != nil {
			respond.WithInternalServerError(err, w)
			return
		}
		if err := contextdocs.WriteContextFile(s.contextDir, aspects); err != nil {
			respond.WithInternalServerError(err, w)
			return
		}
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

func (s *Server) handleContextDoc(w http.ResponseWriter, r *http.Request) {
	groups, err := contextdocs.Scan(s.contextDir)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	// The requested doc must match a scanned entry by string equality; user
	// input is never joined into a path on its own (concept security rule,
	// mirrors handleSkillFile).
	dir, file := r.PathValue("dir"), r.PathValue("file")
	found := false
	for _, group := range groups {
		if group.Name == dir && slices.Contains(group.Files, file) {
			found = true
		}
	}
	if !found {
		http.NotFound(w, r)
		return
	}

	raw, err := os.ReadFile(filepath.Join(s.contextDir, dir, file))
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	body, err := s.renderDocMarkdown(raw)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	data := contextDocPage{
		Body:  body,
		Dir:   dir,
		File:  file,
		Page:  pageContext,
		Title: "Context — " + dir + "/" + file,
	}
	s.renderFragment(w, tmplContextDoc, data)
}

// loadAspectsTolerant reads the vocabulary; a missing context.json renders
// as an empty section (fresh checkout), every other error is real.
func (s *Server) loadAspectsTolerant() ([]contextdocs.RuleAspect, error) {
	aspects, err := contextdocs.LoadAspects(s.contextDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return aspects, nil
}

// renderAspectBar re-renders the shared bar after a mutation, keeping the
// caller's active filter (hidden form fields / query params).
func (s *Server) renderAspectBar(errorMessage string, w http.ResponseWriter, r *http.Request) {
	discovery, err := acdsl.DiscoverRules(r.Context(), ".")
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	filter := contextFilter{
		Aspect:   r.FormValue("aspect"),
		Layer:    r.FormValue("layer"),
		Lifetime: r.FormValue("lifetime"),
		Reach:    r.FormValue("reach"),
		Topic:    r.FormValue("topic"),
	}
	bar := s.buildAspectBar(discovery.Rules, filter)
	if errorMessage != "" {
		bar.Error = errorMessage
	}
	s.renderFragment(w, tmplAspectBar, bar)
}

func (s *Server) handleContextAspectAdd(w http.ResponseWriter, r *http.Request) {
	name := strings.ToUpper(strings.TrimSpace(r.FormValue("name")))
	scope := strings.TrimSpace(r.FormValue("scope"))
	if !aspectNamePattern.MatchString(name) {
		s.renderAspectBar("aspect name must be 2-12 letters A-Z", w, r)
		return
	}
	if scope == "" {
		s.renderAspectBar("scope is required", w, r)
		return
	}

	aspects, err := s.loadAspectsTolerant()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	for _, aspect := range aspects {
		if aspect.Name == name {
			s.renderAspectBar("aspect "+name+" already exists", w, r)
			return
		}
	}

	aspects = append(aspects, contextdocs.RuleAspect{Name: name, Scope: scope, Class: "scope"})
	if err := contextdocs.WriteContextFile(s.contextDir, aspects); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	s.renderAspectBar("", w, r)
}

func (s *Server) handleContextAspectDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	// In-use guard: a vocabulary the entries still cite cannot lose members —
	// the next audit run would fail on every citing entry.
	set, err := contextdocs.ParseContext(s.contextDir, false)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	var blockingIds []string
	for _, entry := range set.Entries {
		if entry.Scope == name || entry.Topic == name {
			blockingIds = append(blockingIds, entry.Id)
		}
	}
	// The taxonomy is shared: acdsl rule families (ID segment) block a
	// delete the same way citing pack entries do.
	discovery, err := acdsl.DiscoverRules(r.Context(), ".")
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	for _, rule := range discovery.Rules {
		if ruleScope(rule.Id) == name || ruleTopic(rule.Id) == name {
			blockingIds = append(blockingIds, rule.Id)
		}
	}
	if len(blockingIds) > 0 {
		s.renderAspectBar("aspect "+name+" is in use: "+strings.Join(blockingIds, ", "), w, r)
		return
	}

	aspects, err := s.loadAspectsTolerant()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	kept := make([]contextdocs.RuleAspect, 0, len(aspects))
	found := false
	for _, aspect := range aspects {
		if aspect.Name == name {
			found = true
			continue
		}
		kept = append(kept, aspect)
	}
	if !found {
		respond.WithBadRequest("unknown aspect: "+name, w)
		return
	}
	if err := contextdocs.WriteContextFile(s.contextDir, kept); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	s.renderAspectBar("", w, r)
}

func (s *Server) handleContextSync(w http.ResponseWriter, r *http.Request) {
	// Concept boundary: the request selects by name, the path comes from the
	// registry (mirrors findRepo).
	repo, ok := s.repoRegistry.Find(r.FormValue("name"))
	if !ok {
		respond.WithBadRequest("unknown repo: "+r.FormValue("name"), w)
		return
	}

	// context-dir reaches the sync script's AGENTS.md placeholder expansion;
	// validate it to a single safe path segment before it leaves the server
	// (the sibling name is validated via repoRegistry.Find above).
	contextDir := r.FormValue("context-dir")
	if !contextDirPattern.MatchString(contextDir) {
		respond.WithBadRequest("invalid context-dir: letters, digits, _ or - only, no slashes", w)
		return
	}

	opts := contextdocs.SyncOptions{
		ContextDir: contextDir,
		SkipAcdsl:  r.FormValue("acdsl") != "on",
		SkipProse:  r.FormValue("prose") != "on",
		Symlink:    r.FormValue("symlink") == "on",
		Task:       r.FormValue("task") == "on",
		Target:     repo.Path,
	}

	// Flat sync: every language ships. Languages are the rules/ guides minus
	// the always-synced artifact guides — mirrors the builder's split.
	groups, err := contextdocs.Scan(s.contextDir)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	for _, group := range groups {
		if group.Name != "rules" {
			continue
		}
		for _, file := range group.Files {
			name := strings.TrimSuffix(file, ".md")
			if name == "plan" || name == "commits" {
				continue
			}
			opts.Langs = append(opts.Langs, name)
		}
	}

	op := func(ctx context.Context) (string, error) {
		return contextdocs.Sync(ctx, opts, s.syncScripts)
	}
	s.runRepoOp(contextSyncLockKey, "repo-op", op, w, r)
}

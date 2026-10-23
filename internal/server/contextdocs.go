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
	// contextSyncLockKey's underscore prefix keeps it out of the registry's repo-name space.
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
	GlobalEnabled  bool
	GlobalError    string
	GlobalFiles    []globalContextFile
	Page           string
	RepoNames      []string
	Sections       []entryKindSection
	ShowProse      bool
	Title          string
}

type contextFilter struct {
	Aspect   string // scope chip
	Layer    string // "", layerAcdsl, or an entry kind (action / rule / fact)
	Lifetime string // "", "task", "doctrine" — acdsl rules only
	Reach    string // reach chip
	Topic    string
}

// layerAcdsl is the layer chip selecting the gate system instead of a prose entry kind.
const layerAcdsl = "acdsl"

// Active switches the page between the collapsed overview and the hide+uncollapse projection.
func (f contextFilter) Active() bool {
	return f.Aspect != "" || f.Layer != "" || f.Lifetime != "" || f.Reach != "" || f.Topic != ""
}

// URL is the single builder behind every chip link on the context page and the session report.
func (f contextFilter) URL(base string) string {
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
		return base
	}
	return base + "?" + query.Encode()
}

// with clears the topic on a scope change: a topic only means something inside its scope.
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

// chipRowCap hides further topic chips behind a client-side "+N" toggle.
const chipRowCap = 4

// topicChip is one per-scope topic filter in a group's summary row.
type topicChip struct {
	Name  string
	Count int
	URL   string
}

// entryScopeGroup coverage is taken over the unfiltered total.
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

type entryKindSection struct {
	Kind   string
	Groups []entryScopeGroup
}

// aspectBarEntry.Registered is false for a scope derived from rule IDs but missing from the taxonomy.
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

// aspectBarFragment carries no topic axis: topic filtering lives in each scope group's summary row.
type aspectBarFragment struct {
	Aspects   []aspectBarEntry
	Layers    []aspectBarEntry
	Lifetimes []aspectBarEntry
	Reaches   []aspectBarEntry
	Filter    contextFilter
	// Bulk posts the listing's bulk form via the HTML form attribute.
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
	Role        string               `json:"role"`
	Reach       string               `json:"reach"`
	Version     string               `json:"version"`
	Source      string               `json:"source"`
}

type registryEntryContent struct {
	Statement string `json:"statement"`
}

type registryEntryView struct {
	Entry   registryEntry
	GatedBy []string
}

// aspectNamePattern matches the entry-ID grammar's ASPECT segment.
var aspectNamePattern = regexp.MustCompile(`^[A-Z]{2,12}$`)

// contextDirPattern forbids slashes so the value can never traverse or inject into the sync script's sed.
var contextDirPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Channel and role tags mirror internal/contextdocs's unexported vocabulary; keep the two in step.
const (
	channelGate   = "gate"
	channelHook   = "hook"
	channelLint   = "lint"
	channelProse  = "prose"
	channelSchema = "schema"
	roleDef       = "def"
	rolePayload   = "payload"
	roleQuality   = "quality"
	roleRule      = "rule"
	roleSpawn     = "spawn"
	roleStep      = "step"
	roleStop      = "stop"
	roleSubstep   = "substep"
	roleVerify    = "verify"
)

func enforcementTitle(enforcement string) string {
	switch enforcement {
	case channelGate:
		return "checked mechanically by the acdsl gate its Gate: bullet names — a violation blocks"
	case channelHook:
		return "enforced by the Claude Code hook its Hook: bullet names, at tool-call time"
	case channelLint:
		return "enforced by a linter or formatter"
	case channelSchema:
		return "produced output must conform to the JSON schema its Schema: bullet names"
	case channelProse:
		return "checked by a human or agent during review — no mechanical check"
	}
	return ""
}

func roleTitle(role string) string {
	switch role {
	case roleStep:
		return "an execution unit — removing it means some output provably does not happen"
	case roleStop:
		return "a behavioral halt — removing it means some failure provably can happen"
	case rolePayload:
		return "a template/example paired with a fenced block, not a channel"
	}
	return ""
}

// sourceDocHref maps context/actions/x.md to the doc-view segment actions/x.md.
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

	// A missing template renders as an empty baseline, as on a fresh checkout.
	agents, err := os.ReadFile(filepath.Join(s.contextDir, "AGENTS.md"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		respond.WithInternalServerError(err, w)
		return
	}
	data.AgentsTemplate = string(agents)

	globalFiles, globalErr := s.globalContextFiles()
	if globalErr != nil {
		data.GlobalError = globalErr.Error()
	}
	data.GlobalFiles = globalFiles
	data.GlobalEnabled = s.globalContextEnabled()

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

// buildEntrySections does all grouping so the template only ranges.
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

// buildEntryScopeGroup counts coverage over the full scope, not the filtered entries.
func buildEntryScopeGroup(scope string, views []registryEntryView, filter contextFilter) entryScopeGroup {
	group := entryScopeGroup{Scope: scope, Open: filter.Active(), Topics: entryTopicChips(views)}
	scoped := filter.with("aspect", scope)
	group.AllURL = scoped.URL("/context")
	for index := range group.Topics {
		group.Topics[index].URL = scoped.with("topic", group.Topics[index].Name).URL("/context")
	}
	if len(group.Topics) > chipRowCap {
		group.ExtraTopics = len(group.Topics) - chipRowCap
	}
	// The topic narrows only groups that carry it; a same-scope group in another kind keeps its full list.
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

// loadRegistryEntries renders a broken file as empty: the generation gates own its integrity.
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

// reachChips uses exact-name semantics: a chip answers "what is deliberately scoped here", never coverage.
func reachChips(value string) []string {
	if value == reach.Global || value == reach.Always {
		return []string{value}
	}
	return reach.Names(value)
}

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
		isReserved := name == reach.Global || name == reach.Always || name == reach.ThisRepo
		if !isReserved {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	ordered := make([]string, 0, len(names))
	if names[reach.Global] {
		ordered = append(ordered, reach.Global)
	}
	if names[reach.Always] {
		ordered = append(ordered, reach.Always)
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
func allChip(base string, filter contextFilter, axis, active string) aspectBarEntry {
	chip := aspectBarEntry{Active: active == "", Name: "all", Registered: true}
	chip.URL = filter.with(axis, "").URL(base)
	return chip
}

func axisChips(base string, filter contextFilter, axis, active string, values []string) []aspectBarEntry {
	chips := []aspectBarEntry{allChip(base, filter, axis, active)}
	for _, value := range values {
		chip := aspectBarEntry{Active: active == value, Name: value, Registered: true}
		chip.URL = filter.with(axis, value).URL(base)
		chips = append(chips, chip)
	}
	return chips
}

// buildAspectBar surfaces scopes missing from the taxonomy as unregistered, never hidden.
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
	bar.Reaches = append([]aspectBarEntry{allChip("/context", filter, "reach", filter.Reach)},
		reachBarEntries(reachRuleCounts, reachEntryCounts)...)
	for index := range bar.Reaches {
		if bar.Reaches[index].Name == "all" {
			continue
		}
		bar.Reaches[index].Active = bar.Reaches[index].Name == filter.Reach
		bar.Reaches[index].URL = filter.with("reach", bar.Reaches[index].Name).URL("/context")
	}

	bar.Layers = axisChips("/context", filter, "layer", filter.Layer,
		[]string{layerAcdsl, contextdocs.RuleKindAction, contextdocs.RuleKindRule, contextdocs.RuleKindFact})
	bar.Lifetimes = axisChips("/context", filter, "lifetime", filter.Lifetime,
		[]string{acdsl.LifetimeTask, acdsl.LifetimeDoctrine})

	bar.Aspects = append(bar.Aspects, allChip("/context", filter, "aspect", filter.Aspect))
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
			URL:        filter.with("aspect", aspect.Name).URL("/context"),
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
			URL:       filter.with("aspect", scope).URL("/context"),
		})
	}
	return bar
}

type contextEntryPage struct {
	Entry   registryEntry
	GatedBy []string
	Picker  reachPicker
	Error   string
	Page    string
	Title   string
}

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

// handleContextEntryReach writes through WriteContextFile, like the aspect editor; empty input means "none".
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
	if err := contextdocs.WriteContextFile(s.contextDir); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	http.Redirect(w, r, "/context/entry/"+url.PathEscape(id), http.StatusSeeOther)
}

// handleContextReachBulk regenerates context.json once, not per entry.
func (s *Server) handleContextReachBulk(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	value := reachFromForm(r)
	ids := r.Form["id"]
	isPicked := len(r.Form["reach"]) > 0
	hasSelection := isPicked && len(ids) > 0
	backFilter := contextFilter{
		Aspect:   r.FormValue("aspect"),
		Layer:    r.FormValue("layer"),
		Lifetime: r.FormValue("lifetime"),
		Reach:    r.FormValue("reach-filter"),
		Topic:    r.FormValue("topic"),
	}
	back := backFilter.URL("/context")
	if !hasSelection || !reach.Valid(value) {
		redirectWithError(w, r, back, "bulk reach: pick at least one item and a valid reach")
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
	set, err := contextdocs.ParseContext(s.contextDir, false)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	entryIds := map[string]bool{}
	for _, entry := range set.Entries {
		entryIds[entry.Id] = true
	}
	for _, id := range ids {
		_, isRule := ruleById[id]
		if !isRule && !entryIds[id] {
			redirectWithError(w, r, back, "bulk reach: unknown id "+id)
			return
		}
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
		if err := contextdocs.WriteContextFile(s.contextDir); err != nil {
			respond.WithInternalServerError(err, w)
			return
		}
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// redirectWithError sends the browser back to the listing with the message in the err query, which the page renders as a card.
func redirectWithError(w http.ResponseWriter, r *http.Request, back, message string) {
	separator := "?"
	if strings.Contains(back, "?") {
		separator = "&"
	}
	http.Redirect(w, r, back+separator+"err="+url.QueryEscape(message), http.StatusSeeOther)
}

func (s *Server) handleContextDoc(w http.ResponseWriter, r *http.Request) {
	groups, err := contextdocs.Scan(s.contextDir)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	// User input is never joined into a path; it must match a scanned entry by string equality.
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

// loadAspectsTolerant tolerates only a missing aspects.json, as on a tree without the private context surface.
func (s *Server) loadAspectsTolerant() ([]contextdocs.RuleAspect, error) {
	aspects, err := contextdocs.LoadAspects(s.contextDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return aspects, nil
}

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
	if err := contextdocs.SaveAspects(s.contextDir, aspects); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	if err := contextdocs.WriteContextFile(s.contextDir); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	s.renderAspectBar("", w, r)
}

func (s *Server) handleContextAspectDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	// In-use guard: removing a cited member would fail the next audit on every citing entry.
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
	// The taxonomy is shared: acdsl rule families block a delete like citing pack entries do.
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
	if err := contextdocs.SaveAspects(s.contextDir, kept); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	if err := contextdocs.WriteContextFile(s.contextDir); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	s.renderAspectBar("", w, r)
}

func (s *Server) handleContextSync(w http.ResponseWriter, r *http.Request) {
	// The request selects by name; the path comes from the registry.
	repo, ok := s.repoRegistry.Find(r.FormValue("name"))
	if !ok {
		respond.WithBadRequest("unknown repo: "+r.FormValue("name"), w)
		return
	}

	// context-dir becomes a path segment under the repo and the state dir, so it must be one safe segment.
	contextDir := r.FormValue("context-dir")
	if !contextDirPattern.MatchString(contextDir) {
		respond.WithBadRequest("invalid context-dir: letters, digits, _ or - only, no slashes", w)
		return
	}

	deploy := contextdocs.ContextDeploy{
		Acdsl:      r.FormValue("acdsl") == "on",
		ContextDir: contextDir,
		Markdown:   r.FormValue("markdown") == "on",
		Task:       r.FormValue("task") == "on",
		WriteRepo:  r.FormValue("write") == "on",
	}
	hasSubOption := deploy.Markdown || deploy.Acdsl || deploy.Task
	if hasSubOption && !deploy.WriteRepo {
		respond.WithBadRequest("project markdown, sync acdsl and sync task need write to repo", w)
		return
	}

	// Every language the taxonomy binds ships; the form offers no per-language choice.
	aspects, err := contextdocs.LoadAspects(s.contextDir)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	deploy.Langs = contextdocs.Langs(aspects)
	sourceRoot, err := filepath.Abs(filepath.Dir(s.contextDir))
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	options := &contextdocs.DeployOptions{ContextDeploy: deploy, Name: repo.Name, RepoPath: repo.Path, SourceRoot: sourceRoot, StateDir: s.stateDir, Trash: s.trash}
	op := func(ctx context.Context) (string, error) {
		return contextdocs.Deploy(ctx, options)
	}
	s.runRepoOp(contextSyncLockKey, "repo-op", op, w, r)
}

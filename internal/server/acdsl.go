package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/kevinhorst/smine/internal/acdsl"
	"github.com/kevinhorst/smine/internal/reach"
	"github.com/kevinhorst/smine/internal/server/respond"
)

// entryIdRe follows the coverage citation convention: why texts cite KIND-SCOPE[-TOPIC]-NNN ids.
var entryIdRe = regexp.MustCompile(`\b(ACTION|RULE|FACT)-[A-Z]{2,12}(?:-[A-Z]{2,12})?-\d{3}\b`)

// ruleScope maps ACDSL-GOLANG-ERR-001 to GOLANG, the applies-to taxonomy shared with prose entries.
func ruleScope(id string) string {
	parts := strings.Split(id, "-")
	if len(parts) < 3 {
		return ""
	}
	return parts[1]
}

// ruleTopic maps ACDSL-GOLANG-ERR-001 to ERR; three-segment IDs yield "".
func ruleTopic(id string) string {
	parts := strings.Split(id, "-")
	if len(parts) < 4 {
		return ""
	}
	return parts[2]
}

// evictionMinRuns follows the eviction policy in docs/acdsl.html: only a long, entirely clean window recommends eviction.
const evictionMinRuns = 300

// recentRunsCap bounds the "recent gate runs" list per window.
const recentRunsCap = 20

// acdslStatView precomputes the rate so internal/acdsl stays presentation-free.
type acdslStatView struct {
	Projected      bool
	Runs           int
	RedRuns        int
	Violations     int
	LastRed        string
	RedRatePercent string
}

type acdslRuleRow struct {
	Rule            acdsl.Rule
	Aspect          string
	VerifierTitle   string
	HasFixtures     bool
	Covers          []coverLink
	Stats           []acdslStatView
	Candidate       string
	CandidateDetail string
}

// acdslRunView uses the branch as the session proxy.
type acdslRunView struct {
	Ts       string
	Branch   string
	Session  string
	Outcome  string
	RedRules []string
}

type acdslScopeGroup struct {
	Scope       string
	Open        bool
	AllURL      string
	Topics      []topicChip
	ExtraTopics int
	Rows        []acdslRuleRow
}

type acdslPage struct {
	Filter          contextFilter
	Show            bool
	RuleGroups      []acdslScopeGroup
	RuleCount       int
	GateOnlyCount   int
	CoveredEntries  int
	TotalEntries    int
	CoveragePercent string
	StyleGatedCount int
	Violations      []string
	Error           string
	RecentRuns      []acdslRunView
}

func (s *Server) buildAcdslPage(r *http.Request) acdslPage {
	filter := parseContextFilter(r)
	data := acdslPage{Filter: filter, Show: filter.Layer == "" || filter.Layer == layerAcdsl}

	discovery, err := acdsl.DiscoverRules(r.Context(), ".")
	if err != nil {
		data.Error = err.Error()
		return data
	}
	data.Violations = discovery.Violations

	stats := s.acdslStats(&data)
	cited := map[string]bool{}
	styleCited := map[string]bool{}
	packIds := s.loadRegistryIds()
	verifiers, _ := acdsl.LoadRegistry("acdsl/registry.json")
	rowsByScope := map[string][]acdslRuleRow{}
	for _, rule := range discovery.Rules {
		if !reachMatches(rule.Reach, filter.Reach) {
			continue
		}
		if !lifetimeMatches(rule.Lifetime, filter.Lifetime) {
			continue
		}
		row := acdslRuleRow{Rule: rule, Aspect: ruleScope(rule.Id), Stats: stats[rule.Id],
			VerifierTitle: verifiers[rule.Verifier].Description}
		if filter.Aspect != "" && row.Aspect != filter.Aspect {
			continue
		}
		if _, statErr := os.Stat(filepath.Join(acdsl.FixturesDir, rule.Id)); statErr == nil {
			row.HasFixtures = true
		}
		row.Candidate, row.CandidateDetail = flipCandidate(rule, row.HasFixtures, row.Stats)
		for _, id := range entryIdRe.FindAllString(rule.Why, -1) {
			row.Covers = append(row.Covers, coverLink{Id: id, InRegistry: packIds[id]})
			if packIds[id] {
				cited[id] = true
			} else {
				styleCited[id] = true
			}
		}
		if !rule.Projected {
			data.GateOnlyCount++
		}
		rowsByScope[row.Aspect] = append(rowsByScope[row.Aspect], row)
	}
	scopes := make([]string, 0, len(rowsByScope))
	for scope := range rowsByScope {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)
	for _, scope := range scopes {
		topicCounts := map[string]int{}
		for _, row := range rowsByScope[scope] {
			if topic := ruleTopic(row.Rule.Id); topic != "" {
				topicCounts[topic]++
			}
		}
		scoped := filter.with("aspect", scope)
		group := acdslScopeGroup{Scope: scope, Open: filter.Active(), AllURL: scoped.URL("/context"), Topics: sortedTopicChips(topicCounts)}
		for index := range group.Topics {
			group.Topics[index].URL = scoped.with("topic", group.Topics[index].Name).URL("/context")
		}
		if len(group.Topics) > chipRowCap {
			group.ExtraTopics = len(group.Topics) - chipRowCap
		}
		for _, row := range rowsByScope[scope] {
			if filter.Topic != "" && topicCounts[filter.Topic] > 0 && ruleTopic(row.Rule.Id) != filter.Topic {
				continue
			}
			group.Rows = append(group.Rows, row)
		}
		if len(group.Rows) == 0 {
			continue
		}
		data.RuleCount += len(group.Rows)
		data.RuleGroups = append(data.RuleGroups, group)
	}
	data.CoveredEntries, data.TotalEntries = len(cited), len(packIds)
	data.StyleGatedCount = len(styleCited)
	if data.TotalEntries > 0 {
		data.CoveragePercent = fmt.Sprintf("%.0f%%", 100*float64(data.CoveredEntries)/float64(data.TotalEntries))
	}
	return data
}

// lifetimeMatches counts an unset lifetime as doctrine, the parser default.
func lifetimeMatches(lifetime, chip string) bool {
	if chip == "" {
		return true
	}
	if lifetime == "" {
		lifetime = acdsl.LifetimeDoctrine
	}
	return lifetime == chip
}

// fixtureExample is one pass/fail example file on the rule details page.
type fixtureExample struct {
	Name    string
	Content string
}

func loadFixtureExamples(ruleId, set string) []fixtureExample {
	dir := filepath.Join(acdsl.FixturesDir, ruleId, set)
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var examples []fixtureExample
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			continue
		}
		examples = append(examples, fixtureExample{Name: file.Name(), Content: string(content)})
	}
	return examples
}

// coverLink renders ids absent from the generated context file as plain text: a dead link would 404.
type coverLink struct {
	Id         string
	InRegistry bool
}

// reachOption is one choice in the reach selector on the detail pages.
type reachOption struct {
	Exclusive bool
	Label     string
	Selected  bool
	Value     string
}

// buildReachOptions keeps an unknown name as an extra checked option so the next save never drops it.
func (s *Server) buildReachOptions(current string) []reachOption {
	options := []reachOption{
		{Exclusive: true, Label: "disabled", Value: reach.None},
		{Exclusive: true, Label: "global", Value: reach.Global},
		{Exclusive: true, Label: "always (every session)", Value: reach.Always},
		{Value: reach.ThisRepo, Label: reach.ThisRepo + " (this repo)"},
	}
	for _, repo := range s.repoRegistry.Repos() {
		options = append(options, reachOption{Value: repo.Name, Label: repo.Name})
	}
	selected := map[string]bool{current: true}
	for _, name := range reach.Names(current) {
		selected[name] = true
	}
	known := map[string]bool{}
	for i := range options {
		options[i].Selected = selected[options[i].Value]
		known[options[i].Value] = true
	}
	for _, name := range reach.Names(current) {
		if !known[name] {
			options = append(options, reachOption{Value: name, Label: name, Selected: true})
		}
	}
	return options
}

// reachPicker.Form names an outer form via the HTML form attribute; empty means the picker sits inside its own.
type reachPicker struct {
	Label   string
	Form    string
	Options []reachOption
}

func (s *Server) buildReachPicker(current, form string) reachPicker {
	label := current
	if current == reach.None {
		label = "disabled"
	}
	if current == "" {
		label = "select targets"
	}
	return reachPicker{Label: label, Form: form, Options: s.buildReachOptions(current)}
}

// reachFromForm leaves validity to the caller's reach.Valid.
func reachFromForm(r *http.Request) string {
	var values []string
	for _, value := range r.Form["reach"] {
		if value = strings.TrimSpace(value); value != "" {
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return reach.None
	}
	return strings.Join(values, ",")
}

type contextRulePage struct {
	Row      acdslRuleRow
	Covers   []coverLink
	Positive []fixtureExample
	Negative []fixtureExample
	RedRuns  []acdslRunView
	Picker   reachPicker
	// VerifierFound is false when the key is absent from acdsl/registry.json.
	Verifier      acdsl.RegistryEntry
	VerifierFound bool
	Error         string
	Page          string
	Title         string
}

// handleContextRule renders one acdsl rule's details page; unknown ids 404.
func (s *Server) handleContextRule(w http.ResponseWriter, r *http.Request) {
	data, found, err := s.buildContextRulePage(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	s.renderFragment(w, "context_rule.html", data)
}

func (s *Server) buildContextRulePage(ctx context.Context, id string) (contextRulePage, bool, error) {
	discovery, err := acdsl.DiscoverRules(ctx, ".")
	if err != nil {
		return contextRulePage{}, false, err
	}
	var rule *acdsl.Rule
	for i := range discovery.Rules {
		if discovery.Rules[i].Id == id {
			rule = &discovery.Rules[i]
			break
		}
	}
	if rule == nil {
		return contextRulePage{}, false, nil
	}

	var statsSink acdslPage
	stats := s.acdslStats(&statsSink)
	data := contextRulePage{Error: statsSink.Error, Page: pageContext, Title: rule.Id,
		Picker: s.buildReachPicker(rule.Reach, "")}
	row := acdslRuleRow{Rule: *rule, Aspect: ruleScope(rule.Id), Stats: stats[rule.Id]}
	if _, statErr := os.Stat(filepath.Join(acdsl.FixturesDir, rule.Id)); statErr == nil {
		row.HasFixtures = true
	}
	row.Candidate, row.CandidateDetail = flipCandidate(*rule, row.HasFixtures, row.Stats)
	verifiers, _ := acdsl.LoadRegistry("acdsl/registry.json")
	entry, found := verifiers[rule.Verifier]
	row.VerifierTitle = entry.Description
	data.Verifier = entry
	data.VerifierFound = found
	packIds := s.loadRegistryIds()
	for _, cid := range entryIdRe.FindAllString(rule.Why, -1) {
		row.Covers = append(row.Covers, coverLink{Id: cid, InRegistry: packIds[cid]})
	}
	data.Row = row
	data.Covers = row.Covers
	data.Positive = loadFixtureExamples(rule.Id, "pass")
	data.Negative = loadFixtureExamples(rule.Id, "fail")
	for _, run := range statsSink.RecentRuns {
		if slices.Contains(run.RedRules, rule.Id) {
			data.RedRuns = append(data.RedRuns, run)
		}
	}
	return data, true, nil
}

// handleContextRuleReach treats an empty input as reach "none".
func (s *Server) handleContextRuleReach(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	value := reachFromForm(r)
	if !reach.Valid(value) {
		data, found, err := s.buildContextRulePage(r.Context(), id)
		if err != nil {
			respond.WithInternalServerError(err, w)
			return
		}
		if !found {
			http.NotFound(w, r)
			return
		}
		data.Error = "invalid reach: global, none, or a comma-separated repo-name list"
		s.renderFragment(w, "context_rule.html", data)
		return
	}
	discovery, err := acdsl.DiscoverRules(r.Context(), ".")
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	for _, rule := range discovery.Rules {
		if rule.Id == id {
			if err := acdsl.SetRuleReach(rule.File, rule.Line, value); err != nil {
				respond.WithInternalServerError(err, w)
				return
			}
			http.Redirect(w, r, "/context/rule/"+url.PathEscape(id), http.StatusSeeOther)
			return
		}
	}
	http.NotFound(w, r)
}

// flipCandidate only flags; the human flips.
func flipCandidate(rule acdsl.Rule, hasFixtures bool, stats []acdslStatView) (string, string) {
	if rule.Lifetime != acdsl.LifetimeDoctrine || rule.Reach == reach.None {
		return "", ""
	}
	var projected, gateOnly *acdslStatView
	for i := range stats {
		if stats[i].Projected {
			projected = &stats[i]
		} else {
			gateOnly = &stats[i]
		}
	}
	if rule.Projected {
		// Any red means the projection earns its tokens; only a long, entirely clean window suggests dead weight.
		if hasFixtures && projected != nil && projected.Runs >= evictionMinRuns && projected.RedRuns == 0 {
			return "eviction candidate", fmt.Sprintf(
				"projected arm: %d clean runs (threshold ≥%d runs, 0 reds — any red keeps the projection)",
				projected.Runs, evictionMinRuns)
		}
		return "", ""
	}
	if gateOnly != nil && gateOnly.RedRuns >= 1 {
		return "re-projection candidate", fmt.Sprintf(
			"gate-only arm: %d red(s) in %d runs — any red warrants projection", gateOnly.RedRuns, gateOnly.Runs)
	}
	return "", ""
}

// acdslStats shows both delivery arms for a rule flipped mid-history: the A/B pair.
func (s *Server) acdslStats(data *acdslPage) map[string][]acdslStatView {
	sink := s.acdslVerdictsPath
	if sink == "" {
		resolved, err := acdsl.DefaultVerdictsPath()
		if err != nil {
			data.Error = err.Error()
			return nil
		}
		sink = resolved
	}
	records, _, err := acdsl.ReadVerdicts(sink, "")
	if err != nil {
		data.Error = err.Error()
		return nil
	}
	for i := len(records) - 1; i >= 0 && len(data.RecentRuns) < recentRunsCap; i-- {
		record := records[i]
		run := acdslRunView{Ts: record.Ts, Branch: record.Branch, Session: record.Session, Outcome: record.Outcome}
		for _, verdict := range record.Rules {
			if verdict.Violations > 0 {
				run.RedRules = append(run.RedRules, verdict.Id)
			}
		}
		data.RecentRuns = append(data.RecentRuns, run)
	}
	grouped := map[string][]acdslStatView{}
	for _, stat := range acdsl.AggregateVerdicts(records) {
		view := acdslStatView{
			Projected:  stat.Projected,
			Runs:       stat.Runs,
			RedRuns:    stat.RedRuns,
			Violations: stat.Violations,
			LastRed:    stat.LastRed,
		}
		if stat.Runs > 0 {
			view.RedRatePercent = fmt.Sprintf("%.0f%%", 100*float64(stat.RedRuns)/float64(stat.Runs))
		}
		grouped[stat.Id] = append(grouped[stat.Id], view)
	}
	return grouped
}

// loadRegistryIds is the coverage denominator; style-guide RULE-* ids are counted separately.
func (s *Server) loadRegistryIds() map[string]bool {
	ids := map[string]bool{}
	for _, entry := range s.loadRegistryEntries() {
		ids[entry.Id] = true
	}
	return ids
}

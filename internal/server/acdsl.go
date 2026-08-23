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
	"time"

	"github.com/kevinhorst/smine/internal/acdsl"
	"github.com/kevinhorst/smine/internal/reach"
	"github.com/kevinhorst/smine/internal/server/respond"
)

const tmplAcdslSection = "_acdsl.html"

// entryIdRe matches registry and style-guide ids in why texts — the coverage
// citation convention (identity grammar KIND-SCOPE[-TOPIC]-NNN).
var entryIdRe = regexp.MustCompile(`\b(ACTION|RULE|FACT)-[A-Z]{2,12}(?:-[A-Z]{2,12})?-\d{3}\b`)

// ruleScope derives an acdsl rule's scope from its ID's second segment
// (ACDSL-GOLANG-ERR-001 → GOLANG) — the shared applies-to taxonomy with the
// prose entries. Task-contract IDs without the grammar yield "".
func ruleScope(id string) string {
	parts := strings.Split(id, "-")
	if len(parts) < 3 {
		return ""
	}
	return parts[1]
}

// ruleTopic derives an acdsl rule's topic from its ID's optional third name
// segment (ACDSL-GOLANG-ERR-001 → ERR); three-segment IDs yield "".
func ruleTopic(id string) string {
	parts := strings.Split(id, "-")
	if len(parts) < 4 {
		return ""
	}
	return parts[2]
}

// Flip-candidate threshold, from acdsl/README.md's eviction policy: any red
// run is proof the rule earns its projection — ten 1% failures are ten fully
// failed features. Eviction is recommended only when a long projected window
// stays entirely clean; any red in the gate-only arm recommends
// re-projection. UI-level recommendation only; the human flips.
const evictionMinRuns = 300

// recentRunsCap bounds the "recent gate runs" list per window.
const recentRunsCap = 20

// acdslStatView is one delivery arm's aggregated verdicts, presentation-ready
// (rate precomputed — internal/acdsl stays presentation-free).
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

// acdslRunView is one verdict record in the recent-runs list; the branch is
// the session proxy.
type acdslRunView struct {
	Ts       string
	Branch   string
	Session  string
	Outcome  string
	RedRules []string
}

// acdslScopeGroup is one collapsible scope section of rule cards. Without an
// active filter every group renders closed; any active filter renders only
// matching groups, open.
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
	FixturesURL     string
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
	FixturesResult  string
	RecentRuns      []acdslRunView
}

// buildAcdslPage assembles the whole loop view: declared rules, fixture
// presence, verdict stats over the full history, and entry coverage
// extracted from the why texts (a rule's why cites the pack rule ID it
// enforces — the coverage convention).
func (s *Server) buildAcdslPage(r *http.Request) acdslPage {
	filter := parseContextFilter(r)
	data := acdslPage{Filter: filter, Show: filter.Layer == "" || filter.Layer == layerAcdsl}
	data.FixturesURL = "/context/acdsl/fixtures/run" + strings.TrimPrefix(filter.URL(), "/context")

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
		group := acdslScopeGroup{Scope: scope, Open: filter.Active(), AllURL: scoped.URL(), Topics: sortedTopicChips(topicCounts)}
		for index := range group.Topics {
			group.Topics[index].URL = scoped.with("topic", group.Topics[index].Name).URL()
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

// lifetimeMatches reports whether a rule's lifetime passes the lifetime
// chip. Empty chip = no lifetime filter; an unset rule lifetime counts as
// doctrine (the parser default).
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

// loadFixtureExamples reads a rule's example files from one fixture set
// (pass or fail); a missing set renders as the None placeholder.
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

// coverLink is one cited id on the rule details page; ids absent from the
// generated context file render as plain text — a dead link would 404.
type coverLink struct {
	Id         string
	InRegistry bool
}

// reachOption is one choice in the reach selector on the detail pages.
type reachOption struct {
	Value    string
	Label    string
	Selected bool
}

// buildReachOptions assembles the reach checkbox group: disabled, global,
// this repo, then every registered sync target. A list value marks each of
// its names; a name outside the known options renders as an extra checked
// option so it is never silently lost on the next save.
func (s *Server) buildReachOptions(current string) []reachOption {
	options := []reachOption{
		{Value: reach.None, Label: "disabled"},
		{Value: reach.Global, Label: "global"},
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

// reachPicker backs _reach_picker.html — a dropdown-with-checkboxes
// selection over the reach options. Form ties the inputs to a form rendered
// elsewhere on the page (HTML form attribute); empty means the picker sits
// inside its own form.
type reachPicker struct {
	Label   string
	Form    string
	Options []reachOption
}

// buildReachPicker assembles the picker with a summary label reflecting the
// current value ("disabled" for none, the value itself otherwise).
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

// reachFromForm joins the reach checkboxes into the comma grammar; no
// selection (or only empty values) disables (none). Validity — including
// global/none standing alone — is reach.Valid's call, made by the caller.
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

// contextRulePage backs context_rule.html — one acdsl rule with its stats,
// covered entries, examples, and red runs.
type contextRulePage struct {
	Row      acdslRuleRow
	Covers   []coverLink
	Positive []fixtureExample
	Negative []fixtureExample
	RedRuns  []acdslRunView
	Picker   reachPicker
	// Verifier is the rule's registered definition (D4a); VerifierFound
	// is false when the key is absent from acdsl/registry.json.
	Verifier      acdsl.RegistryEntry
	VerifierFound bool
	// FixturesResult is the outcome line for a single-rule fixtures run (D4b).
	FixturesResult string
	Error          string
	Page           string
	Title          string
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

// buildContextRulePage assembles one rule's detail view: stats, fixtures,
// covered entries, red runs, and the verifier's registry definition (D4a).
// found is false when the id matches no discovered rule.
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

// handleContextRuleFixturesRun runs one rule's fixtures and re-renders the
// rule detail page; the button hx-selects #rule-fixtures from the response.
func (s *Server) handleContextRuleFixturesRun(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	data, found, err := s.buildContextRulePage(ctx, r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	registry, err := acdsl.LoadRegistry("acdsl/registry.json")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	report, err := acdsl.RunFixtures(ctx, ".", []acdsl.Rule{data.Row.Rule}, registry)
	switch {
	case err != nil:
		data.FixturesResult = "error: " + err.Error()
	case report.Checked == 0:
		data.FixturesResult = "no fixtures for this rule"
	case len(report.Failures) > 0:
		data.FixturesResult = fmt.Sprintf("%d failure(s): %s · %s", len(report.Failures), report.Failures[0], report.Meta())
	default:
		data.FixturesResult = "fixtures OK · " + report.Meta()
	}
	s.renderFragment(w, "context_rule.html", data)
}

// handleContextRuleReach sets a doctrine rule's reach from the detail page;
// an empty input disables the rule (reach "none"). Plain form POST — 303
// back to the detail page on success, the page with its error slot filled
// on an invalid value.
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

func (s *Server) handleAcdslFixturesRun(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	discovery, err := acdsl.DiscoverRules(ctx, ".")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	registry, err := acdsl.LoadRegistry("acdsl/registry.json")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	report, err := acdsl.RunFixtures(ctx, ".", discovery.Rules, registry)
	data := s.buildAcdslPage(r)
	switch {
	case err != nil:
		data.FixturesResult = "error: " + err.Error()
	case len(report.Failures) > 0:
		data.FixturesResult = fmt.Sprintf("%d failure(s): %s · %s", len(report.Failures), report.Failures[0], report.Meta())
	default:
		data.FixturesResult = fmt.Sprintf("fixtures OK (%d rule(s) with examples) · %s", report.Checked, report.Meta())
	}
	s.renderFragment(w, tmplAcdslSection, data)
}

// flipCandidate applies the computable eviction-policy thresholds to one
// rule's window stats and names the recommended flip, if any. The human
// flips; this only flags.
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
		// Any red is evidence the projection earns its tokens: a rare
		// failure is still a fully failed feature. Only a long, entirely
		// clean window suggests dead weight.
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

// acdslStats reads the full verdict history, groups the per-arm stats by
// rule id (a rule flipped mid-history shows both delivery arms — the A/B
// pair), and keeps the newest records for the recent-runs list.
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

// loadRegistryIds returns the id set of the generated pack registry — the
// coverage denominator. Style-guide RULE-* ids are not in it and are
// counted separately.
func (s *Server) loadRegistryIds() map[string]bool {
	ids := map[string]bool{}
	for _, entry := range s.loadRegistryEntries() {
		ids[entry.Id] = true
	}
	return ids
}

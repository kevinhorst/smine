package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/acdsl"
	"github.com/kevinhorst/smine/internal/checklist"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/proposals"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/sessions"
	"github.com/kevinhorst/smine/internal/skills"
)

const statusDone = "Done"

type overviewPage struct {
	Page  string
	Tiles []overviewTile
	Title string
}

type overviewTile struct {
	Id      string
	Detail  string
	Filters []tileFilter
	Href    string
	Label   string
	Split   *tileSplit
	Trend   *tileTrend
	Value   string
}

// tileFilter is one window link on the Frustration / Positive card (D3).
type tileFilter struct {
	Active bool
	Label  string
	URL    string
}

// tileSplit renders a card whose two values are the links (the card body is
// not an anchor); markup lives in the template.
type tileSplit struct {
	FrustrationHref  string
	FrustrationValue string
	PositiveHref     string
	PositiveValue    string
}

// tileTrend carries the bar-chart rects; markup lives in the template (D4).
type tileTrend struct {
	Bars []trendBar
}

// trendBar is one <rect> of the grouped bar chart, pre-formatted for the
// 100x28 viewBox.
type trendBar struct {
	Class  string
	Height string
	Width  string
	X      string
	Y      string
}

// trendWindow is the number of most recent batches the "10" window shows.
const trendWindow = 10

// Batch windows for the Frustration / Positive card (D3); windowDefault is
// what an absent or unknown ?window= resolves to (D2).
const (
	windowAll     = "all"
	windowLast    = "last"
	windowTen     = "10"
	windowDefault = windowTen
)

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	var tiles []overviewTile
	if welcome, show := s.welcomeTile(r.Context()); show {
		tiles = append(tiles, welcome)
	}
	tiles = append(tiles, s.sessionTiles(parseWindow(r))...)
	tiles = append(tiles, s.settingsTiles()...)
	tiles = append(tiles, s.skillTiles()...)
	tiles = append(tiles, s.repoTile())
	tiles = append(tiles, s.contextTile(r.Context()))
	tiles = append(tiles, s.proposalsTile())
	tiles = append(tiles, s.routineTile(r.Context()))
	tiles = append(tiles, toolsTile())
	tiles = append(tiles, s.checklistTile())

	data := overviewPage{Page: pageOverview, Tiles: tiles, Title: "Overview"}
	s.renderFragment(w, tmplOverview, data)
}

// checklistTile reports done/total progress; a parse failure degrades the
// tile, never the page (D18).
func (s *Server) checklistTile() overviewTile {
	parsedChecklist, err := checklist.Parse(s.checklistPath)
	// A fresh clone has no checklist doc (private artifact) — the tile
	// shows zero progress, same empty state as the checklist page.
	if errors.Is(err, os.ErrNotExist) {
		parsedChecklist, err = &checklist.Checklist{}, nil
	}
	if err != nil {
		return errorTile(err, "/docs/checklist", "checklist", "Checklist")
	}

	done := 0
	for _, entry := range parsedChecklist.Entries {
		if entry.Status == statusDone {
			done++
		}
	}

	tile := overviewTile{
		Id:    "checklist",
		Href:  "/docs/checklist",
		Label: "Checklist",
		Value: fmt.Sprintf("%d/%d done", done, len(parsedChecklist.Entries)),
	}
	return tile
}

// contextTile summarizes the context loop: prose entries vs acdsl rules and
// how much of the prose is gate-covered (entries cited by ≥1 rule's why).
func (s *Server) contextTile(ctx context.Context) overviewTile {
	entries := s.loadRegistryEntries()
	discovery, err := acdsl.DiscoverRules(ctx, ".")
	if err != nil {
		return errorTile(err, "/context", "context", "Context")
	}

	cited := map[string]bool{}
	for _, rule := range discovery.Rules {
		for _, id := range entryIdRe.FindAllString(rule.Why, -1) {
			cited[id] = true
		}
	}
	covered := 0
	for _, entry := range entries {
		if cited[entry.Id] {
			covered++
		}
	}
	coverage := "–"
	if len(entries) > 0 {
		coverage = fmt.Sprintf("%.0f%%", 100*float64(covered)/float64(len(entries)))
	}

	scopes := map[string]bool{}
	for _, entry := range entries {
		scopes[entry.Scope] = true
	}

	tile := overviewTile{
		Id:     "context",
		Detail: fmt.Sprintf("%d scopes · %d acdsl rules · coverage %s (%d of %d)", len(scopes), len(discovery.Rules), coverage, covered, len(entries)),
		Href:   "/context",
		Label:  "Context",
		Value:  fmt.Sprintf("%d entries", len(entries)),
	}
	return tile
}

func (s *Server) sessionTiles(window string) []overviewTile {
	analyzed, batches := 0, 0
	lastDate, lastHref := "", "/sessions"
	frustrationDate, frustrationHref := "", "/sessions"
	positiveDate, positiveHref := "", "/sessions"
	for _, scope := range s.sessions.Scopes() {
		for _, batch := range scope.Batches {
			batches++
			if batch.Batch.AnalyzedDate >= lastDate {
				lastDate = batch.Batch.AnalyzedDate
				lastHref = fmt.Sprintf("/sessions/%s/%d", scope.Name, batch.Batch.Number)
			}
			lastFrustrationId, lastPositiveId := "", ""
			for _, session := range batch.Sessions {
				if session.Skipped {
					continue
				}
				analyzed++
				if len(session.Frustration) > 0 {
					lastFrustrationId = session.Id
				}
				if len(session.Positive) > 0 {
					lastPositiveId = session.Id
				}
			}
			if lastFrustrationId != "" && batch.Batch.AnalyzedDate >= frustrationDate {
				frustrationDate = batch.Batch.AnalyzedDate
				frustrationHref = fmt.Sprintf("/sessions/%s/%d?dimension=%s&open=%s#session-%s",
					scope.Name, batch.Batch.Number, dimFrustration, lastFrustrationId, lastFrustrationId)
			}
			if lastPositiveId != "" && batch.Batch.AnalyzedDate >= positiveDate {
				positiveDate = batch.Batch.AnalyzedDate
				positiveHref = fmt.Sprintf("/sessions/%s/%d?dimension=%s&open=%s#session-%s",
					scope.Name, batch.Batch.Number, dimPositive, lastPositiveId, lastPositiveId)
			}
		}
	}

	points := windowPoints(s.sessions.SentimentByBatch(), window)
	frustration, positive := 0, 0
	for _, point := range points {
		frustration += point.Frustration
		positive += point.Positive
	}

	sessionsTile := overviewTile{
		Id:     "sessions",
		Detail: fmt.Sprintf("%d batches", batches),
		Href:   "/sessions",
		Label:  "Sessions analyzed",
		Value:  fmt.Sprintf("%d", analyzed),
	}
	analysisTile := overviewTile{
		Id:     "last-analysis",
		Detail: "newest batch",
		Href:   lastHref,
		Label:  "Last analysis",
		Value:  lastDate,
	}
	sentimentEntriesTile := overviewTile{
		Id:     "frustration-positive",
		Detail: signalDetail(max(frustrationDate, positiveDate)),
		Label:  "Frustration / Positive",
		Split: &tileSplit{
			FrustrationHref:  frustrationHref,
			FrustrationValue: fmt.Sprintf("%d", frustration),
			PositiveHref:     positiveHref,
			PositiveValue:    fmt.Sprintf("%d", positive),
		},
	}
	if trend, delta, ok := sentimentTrend(points); ok {
		sentimentEntriesTile.Filters = windowFilters(window)
		sentimentEntriesTile.Trend = trend
		sentimentEntriesTile.Detail = joinDetail(sentimentEntriesTile.Detail, delta)
	}
	return []overviewTile{sessionsTile, analysisTile, sentimentEntriesTile}
}

// sentimentTrend renders the windowed batches as a grouped bar chart for the
// merged Frustration / Positive card; ok is false when no batches are loaded
// (chart and filter row are omitted, the split card stays). delta compares
// the newest batch's frustration count to its predecessor (D5).
func sentimentTrend(points []sessions.SentimentPoint) (*tileTrend, string, bool) {
	if len(points) == 0 {
		return nil, "", false
	}

	maxValue := 1
	frustration := make([]int, len(points))
	positive := make([]int, len(points))
	for index, point := range points {
		frustration[index] = point.Frustration
		positive[index] = point.Positive
		maxValue = max(maxValue, point.Frustration, point.Positive)
	}

	trend := &tileTrend{Bars: trendBars(frustration, maxValue, positive)}
	delta := ""
	if len(points) >= 2 {
		latest, previous := points[len(points)-1], points[len(points)-2]
		delta = fmt.Sprintf("%+d vs batch %d", latest.Frustration-previous.Frustration, previous.BatchNumber)
	}
	return trend, delta, true
}

// trendBars lays out one frustration + one positive bar per batch in the
// 100x28 viewBox with 2px vertical padding; maxValue is the shared scale.
// A zero value yields a zero-height (invisible) rect (D7).
func trendBars(frustration []int, maxValue int, positive []int) []trendBar {
	const width, height, pad = 100.0, 28.0, 2.0
	slot := width / float64(len(frustration))
	barWidth := slot * 0.35
	bars := make([]trendBar, 0, 2*len(frustration))
	appendBar := func(class string, index int, offset float64, value int) {
		barHeight := float64(value) / float64(maxValue) * (height - 2*pad)
		bar := trendBar{
			Class:  class,
			Height: fmt.Sprintf("%.1f", barHeight),
			Width:  fmt.Sprintf("%.1f", barWidth),
			X:      fmt.Sprintf("%.1f", (float64(index)+offset)*slot),
			Y:      fmt.Sprintf("%.1f", height-pad-barHeight),
		}
		bars = append(bars, bar)
	}
	for index := range frustration {
		appendBar("trend-frustration", index, 0.1, frustration[index])
		appendBar("trend-positive", index, 0.55, positive[index])
	}
	return bars
}

func parseWindow(r *http.Request) string {
	switch r.URL.Query().Get("window") {
	case windowAll:
		return windowAll
	case windowLast:
		return windowLast
	case windowTen:
		return windowTen
	}
	return windowDefault
}

// windowPoints narrows the per-batch series to the selected window (D1).
func windowPoints(points []sessions.SentimentPoint, window string) []sessions.SentimentPoint {
	switch window {
	case windowAll:
		return points
	case windowLast:
		if len(points) > 1 {
			return points[len(points)-1:]
		}
	case windowTen:
		if len(points) > trendWindow {
			return points[len(points)-trendWindow:]
		}
	}
	return points
}

// windowFilters builds the card's window links; the default window links to
// the bare overview URL (D3).
func windowFilters(window string) []tileFilter {
	filters := make([]tileFilter, 0, 3)
	for _, value := range []string{windowLast, windowTen, windowAll} {
		url := "/?window=" + value
		if value == windowDefault {
			url = "/"
		}
		filter := tileFilter{
			Active: value == window,
			Label:  value,
			URL:    url,
		}
		filters = append(filters, filter)
	}
	return filters
}

// joinDetail joins non-empty detail fragments with the tile separator (D5).
func joinDetail(parts ...string) string {
	var kept []string
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, " · ")
}

// signalDetail labels a signal tile with the batch date its link jumps to
// (D6); batches without analyzedDate yield no detail line.
func signalDetail(date string) string {
	if date == "" {
		return ""
	}
	return "last signal " + date
}

// settingsTiles derives every settings.json-backed tile from one loadBoth
// call: hooks, MCP, disabled counts, model.
func (s *Server) settingsTiles() []overviewTile {
	main, disabled, err := s.loadBoth()
	if err != nil {
		return []overviewTile{errorTile(err, "/config/claude", "hooks", "Claude settings")}
	}

	var tiles []overviewTile
	tiles = append(tiles, hooksTile(main, disabled))
	tiles = append(tiles, s.mcpTile(main))
	tiles = append(tiles, modelTile(main))
	return tiles
}

func (s *Server) skillTiles() []overviewTile {
	list, err := skills.Scan(s.skillsRepo, s.skillsHome)
	if err != nil {
		return []overviewTile{errorTile(err, "/scripts/skills", "skills", "Skills")}
	}

	repoCount, localCount, outOfSync, workflowScripts := 0, 0, 0, 0
	for _, skill := range list {
		if !skill.Synced {
			outOfSync++
		}
		if skill.Origin == skills.OriginHome {
			localCount++
			continue
		}
		repoCount++
		workflowScripts += countWorkflowScripts(skill.Files)
	}

	skillsTile := overviewTile{
		Id:     "skills",
		Detail: fmt.Sprintf("active / repo · %d out of sync · %d workflows", outOfSync, workflowScripts),
		Href:   "/scripts/skills",
		Label:  "Skills",
		Value:  fmt.Sprintf("%d/%d", localCount, repoCount),
	}
	return []overviewTile{skillsTile}
}

// proposalsTile counts actual proposals — status-bearing entries only, the
// same rule the proposals filters use; rerouted/considered notes don't count.
// Detail shows the accepted/rejected split under the page's effective-state
// rule: a pending vote wins over the stored status (D1/D3).
func (s *Server) proposalsTile() overviewTile {
	files, _, err := proposals.Load(s.proposalsDir)
	if err != nil {
		return errorTile(err, "/proposals", "proposals", "Proposals")
	}
	votes, err := proposals.LoadVotes(s.votesPath())
	if err != nil {
		votes = nil
	}

	total := 0
	counts := map[string]int{}
	for _, file := range files {
		for _, group := range file.Groups {
			for index := range group.Proposals {
				if group.Proposals[index].Status == "" {
					continue
				}
				total++
				counts[effectiveState(&group.Proposals[index], file.Kind, votes)]++
			}
		}
	}
	accepted := counts["accepted"] + counts["applied"] + counts["building"]
	open := total - accepted - counts["rejected"] - counts["postponed"]

	return overviewTile{
		Id:     "proposals",
		Detail: fmt.Sprintf("%d accepted · %d rejected", accepted, counts["rejected"]),
		Href:   "/proposals",
		Label:  "Proposals",
		Value:  fmt.Sprintf("%d open", open),
	}
}

// repoTile counts registered repos and their real agent worktrees (D3/D8);
// the registry is in memory and counting is stat-cheap — no git execution.
func (s *Server) repoTile() overviewTile {
	registered := s.repoRegistry.Repos()
	worktrees := 0
	for _, repo := range registered {
		worktrees += repos.CountPoolWorktrees(repo.Path)
	}
	return overviewTile{
		Id:     "repos",
		Detail: fmt.Sprintf("%d worktrees", worktrees),
		Href:   "/repos",
		Label:  "Repos",
		Value:  fmt.Sprintf("%d", len(registered)),
	}
}

// routineTile reports active/total (active = loaded in launchd, the same
// per-request probe the routines index does); broken or unprobeable routines
// count as not active. A scan failure degrades the tile (D9).
func (s *Server) routineTile(ctx context.Context) overviewTile {
	list, err := routines.Scan(s.routinesDir)
	if err != nil {
		return errorTile(err, "/routines", "routines", "Routines")
	}

	active := 0
	for index := range list {
		if list[index].LoadError != "" {
			continue
		}
		if loaded, err := routines.IsLoaded(ctx, list[index].Label); err == nil && loaded {
			active++
		}
	}
	return overviewTile{
		Id:     "routines",
		Detail: fmt.Sprintf("%d paused", len(list)-active),
		Href:   "/routines",
		Label:  "Routines",
		Value:  fmt.Sprintf("%d/%d active", active, len(list)),
	}
}

// welcomeTile summarizes the setup checks; runSetupChecks bounds its own
// peek probe, so a down peek cannot stall the dashboard. The tile hides once
// every check is green — the Welcome surface disappears when setup is done —
// unless the install opted in with --init-welcome (debugging).
func (s *Server) welcomeTile(ctx context.Context) (overviewTile, bool) {
	checks := s.runSetupChecks(ctx)
	ok := 0
	detail := "all green"
	for _, check := range checks {
		if check.Ok {
			ok++
		} else if detail == "all green" {
			detail = "next: " + check.Name
		}
	}

	tile := overviewTile{
		Id:     "welcome",
		Detail: detail,
		Href:   "/welcome",
		Label:  "Setup",
		Value:  fmt.Sprintf("%d/%d checks", ok, len(checks)),
	}
	return tile, ok < len(checks) || s.initWelcome
}

func countWorkflowScripts(files []string) int {
	count := 0
	for _, file := range files {
		if strings.HasPrefix(file, "workflows/") && strings.HasSuffix(file, ".js") {
			count++
		}
	}
	return count
}

func errorTile(err error, href, id, label string) overviewTile {
	return overviewTile{
		Id:     id,
		Detail: err.Error(),
		Href:   href,
		Label:  label,
		Value:  "—",
	}
}

// hooksTile reports enabled/total hook groups; its detail line carries the
// disabled counts across hooks, permissions, and MCP servers (the retired
// Disabled-entries card folded in, D7).
func hooksTile(main, disabled *config.Settings) overviewTile {
	mainHooks, err := main.Hooks()
	if err != nil {
		return errorTile(err, "/config/claude", "hooks", "Hooks")
	}
	disabledHooks, err := disabled.Hooks()
	if err != nil {
		return errorTile(err, "/config/claude", "hooks", "Hooks")
	}
	disabledPerms, err := disabled.Permissions()
	if err != nil {
		return errorTile(err, "/config/claude", "hooks", "Hooks")
	}
	disabledMcp, err := main.DisabledMcpjsonServers()
	if err != nil {
		return errorTile(err, "/config/claude", "hooks", "Hooks")
	}

	enabled, total := 0, 0
	for _, groups := range mainHooks {
		enabled += len(groups)
		total += len(groups)
	}
	disabledHookCount := 0
	for _, groups := range disabledHooks {
		disabledHookCount += len(groups)
	}
	total += disabledHookCount
	permCount := len(disabledPerms.Allow) + len(disabledPerms.Ask)

	return overviewTile{
		Id:     "hooks",
		Detail: fmt.Sprintf("disabled: %d hooks · %d permissions · %d MCP", disabledHookCount, permCount, len(disabledMcp)),
		Href:   "/config/claude?open=hooks#hooks",
		Label:  "Hooks",
		Value:  fmt.Sprintf("%d/%d enabled", enabled, total),
	}
}

// mcpTile counts ~/.claude.json servers not in the live disabled list (D5).
func (s *Server) mcpTile(main *config.Settings) overviewTile {
	disabledNames, err := main.DisabledMcpjsonServers()
	if err != nil {
		return errorTile(err, "/config/claude", "mcp", "MCP servers")
	}
	known, err := config.McpServerNames(s.claudeJsonPath)
	if err != nil {
		return errorTile(err, "/config/claude", "mcp", "MCP servers")
	}

	var enabled []string
	for _, name := range known {
		if !slices.Contains(disabledNames, name) {
			enabled = append(enabled, name)
		}
	}

	tile := overviewTile{
		Id:     "mcp",
		Detail: strings.Join(enabled, " · "),
		Href:   "/config/claude?open=mcp#mcp",
		Label:  "MCP servers",
		Value:  fmt.Sprintf("%d/%d enabled", len(enabled), len(known)),
	}
	return tile
}

func modelTile(main *config.Settings) overviewTile {
	model, err := main.Model()
	if err != nil {
		return errorTile(err, "/config/claude", "model", "Model")
	}
	if model == "" {
		model = "(default)"
	}

	tile := overviewTile{
		Id:    "model",
		Href:  "/config/claude?open=model#model",
		Label: "Model",
		Value: model,
	}
	return tile
}

func toolsTile() overviewTile {
	tile := overviewTile{
		Id:     "tools",
		Detail: strings.Join(toolNames, " · "),
		Href:   "/tools",
		Label:  "Tools",
		Value:  fmt.Sprintf("%d", len(toolNames)),
	}
	return tile
}

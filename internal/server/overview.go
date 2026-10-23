package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/acdsl"
	"github.com/kevinhorst/smine/internal/backlog"
	"github.com/kevinhorst/smine/internal/budget"
	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/sessions"
	"github.com/kevinhorst/smine/internal/skills"
)

type overviewPage struct {
	Page  string
	Tiles []overviewTile
	Title string
}

type overviewTile struct {
	Id       string
	Detail   string
	Filters  []tileFilter
	Href     string
	Label    string
	LazyHref string // fragment that replaces the tile after the page renders; empty = rendered inline
	Plans    []usagePlanOption
	Split    *tileSplit
	Trend    *tileTrend
	Value    string
}

// tileFilter is one period link on the Frustration / Positive or the Usage tile.
type tileFilter struct {
	Active bool
	Label  string
	URL    string
}

// tileSplit makes the two values the links; the card body is not an anchor.
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

// trendBar is pre-formatted for the 100x28 viewBox.
type trendBar struct {
	Class  string
	Height string
	Width  string
	X      string
	Y      string
}

// trendWindow is the number of most recent batches the "10" window shows.
const trendWindow = 10

// windowDefault is what an absent or unknown ?window= resolves to.
const (
	windowAll     = "all"
	windowLast    = "last"
	windowTen     = "10"
	windowDefault = windowTen
)

// usagePresets are the Usage page's first three presets.
var usagePresets = []int{1, 7, 30}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	s.reloadSessions()
	usageDays, window := parseUsageDays(r), parseWindow(r)
	var tiles []overviewTile
	// Casual installs see only the session and proposal tiles; the rest expose engine internals.
	if !s.presentation.isDeveloperAudience() {
		tiles = append(tiles, s.sessionTiles(usageDays, window)...)
		tiles = append(tiles, s.proposalsTile())
		data := overviewPage{Page: pageOverview, Tiles: tiles, Title: translate(s.presentation.language(), "Overview")}
		s.renderFragment(w, tmplOverview, data)
		return
	}
	tiles = append(tiles, welcomePlaceholderTile())
	tiles = append(tiles, s.sessionTiles(usageDays, window)...)
	tiles = append(tiles, s.settingsTiles()...)
	if s.hasUsage() {
		tiles = append(tiles, s.usageOverviewTile(time.Now(), usageDays, window))
	}
	tiles = append(tiles, s.skillTiles()...)
	tiles = append(tiles, s.repoTile())
	tiles = append(tiles, s.contextTile(r.Context()))
	if s.hasProposals() {
		tiles = append(tiles, s.proposalsTile())
	}
	if s.records.has(s.machine, components.Routines) {
		tiles = append(tiles, s.routineTile(r.Context()))
		tiles = append(tiles, s.backlogTile())
	}

	data := overviewPage{Page: pageOverview, Tiles: tiles, Title: "Overview"}
	s.renderFragment(w, tmplOverview, data)
}

// backlogTile reads the store alone; a parse failure degrades the tile.
func (s *Server) backlogTile() overviewTile {
	store, err := s.loadBacklog()
	if err != nil {
		return errorTile(err, "/backlog", "backlog", "Backlog")
	}

	critical, pipeline := 0, 0
	for _, item := range store.Items() {
		if item.Pri == priCritical {
			critical++
		}
		if slices.Contains(backlog.PipelineStatuses, item.Status) {
			pipeline++
		}
	}

	tile := overviewTile{
		Id:     "backlog",
		Detail: fmt.Sprintf("%d critical · %d in pipeline", critical, pipeline),
		Href:   "/backlog",
		Label:  "Backlog",
		Value:  fmt.Sprintf("%d active", len(store.Items())),
	}
	return tile
}

// contextTile counts an entry as gate-covered when at least one rule's why cites it.
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

const sessionsHref = "/sessions"

// sessionTileScan accumulates the overview's session tiles over every week.
type sessionTileScan struct {
	analyzed          int
	frustrationActive string
	frustrationHref   string
	lastActive        string
	lastHref          string
	positiveActive    string
	positiveHref      string
	weeks             int
}

func newSessionTileScan() *sessionTileScan {
	scan := &sessionTileScan{frustrationHref: sessionsHref, lastHref: sessionsHref, positiveHref: sessionsHref}
	return scan
}

func (t *sessionTileScan) addSession(session *sessions.Session, weekHref string) {
	if session.Skipped {
		return
	}
	t.analyzed++
	if session.LastActive > t.lastActive {
		t.lastActive, t.lastHref = session.LastActive, weekHref
	}

	isNewerFrustration := len(session.Frustration) > 0 && session.LastActive > t.frustrationActive
	if isNewerFrustration {
		t.frustrationActive = session.LastActive
		t.frustrationHref = sessionAnchorHref(dimFrustration, session.Id, weekHref)
	}
	isNewerPositive := len(session.Positive) > 0 && session.LastActive > t.positiveActive
	if isNewerPositive {
		t.positiveActive = session.LastActive
		t.positiveHref = sessionAnchorHref(dimPositive, session.Id, weekHref)
	}
}

func (t *sessionTileScan) addWeek(week *sessions.Week) {
	t.weeks++
	weekHref := fmt.Sprintf("/sessions/%s/%s", week.Scope, week.Key)
	for index := range week.Sessions {
		t.addSession(&week.Sessions[index], weekHref)
	}
}

func dateOf(timestamp string) string {
	if len(timestamp) < len(time.DateOnly) {
		return timestamp
	}
	return timestamp[:len(time.DateOnly)]
}

func sessionAnchorHref(dimension, id, weekHref string) string {
	return fmt.Sprintf("%s?dimension=%s&open=%s#session-%s", weekHref, dimension, id, id)
}

func (s *Server) sessionTiles(usageDays int, window string) []overviewTile {
	language := s.presentation.language()
	scan := newSessionTileScan()
	for _, scope := range s.sessions.Scopes() {
		for _, week := range scope.Weeks {
			scan.addWeek(week)
		}
	}

	points := windowPoints(s.sessions.SentimentByWeek(), window)
	frustration, positive := 0, 0
	for _, point := range points {
		frustration += point.Frustration
		positive += point.Positive
	}

	sessionsTile := overviewTile{
		Id:     "sessions",
		Detail: joinDetail(fmt.Sprintf("%d %s", scan.weeks, translate(language, "weeks")), lastAnalysisDetail(dateOf(scan.lastActive), language)),
		Href:   scan.lastHref,
		Label:  "Sessions analyzed",
		Value:  strconv.Itoa(scan.analyzed),
	}
	sentimentEntriesTile := overviewTile{
		Id:     "frustration-positive",
		Detail: signalDetail(dateOf(max(scan.frustrationActive, scan.positiveActive)), language),
		Label:  "Frustration / Positive",
		Split: &tileSplit{
			FrustrationHref:  scan.frustrationHref,
			FrustrationValue: fmt.Sprintf("%d", frustration),
			PositiveHref:     scan.positiveHref,
			PositiveValue:    fmt.Sprintf("%d", positive),
		},
	}
	if trend, delta, ok := sentimentTrend(language, points); ok {
		sentimentEntriesTile.Filters = windowFilters(usageDays, window)
		sentimentEntriesTile.Trend = trend
		sentimentEntriesTile.Detail = joinDetail(sentimentEntriesTile.Detail, delta)
	}
	return []overviewTile{sessionsTile, sentimentEntriesTile}
}

// sentimentTrend's delta compares the newest batch's frustration count to its predecessor.
func sentimentTrend(language string, points []sessions.SentimentPoint) (*tileTrend, string, bool) {
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
		delta = fmt.Sprintf(translate(language, "%+d vs week %s"), latest.Frustration-previous.Frustration, previous.Week)
	}
	return trend, delta, true
}

// trendBars keeps a zero value as a zero-height rect so the bar slots stay aligned.
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

// parseUsageDays falls back to the Usage page's default for an absent or unknown ?usage=.
func parseUsageDays(r *http.Request) int {
	days, err := strconv.Atoi(r.URL.Query().Get("usage"))
	if err != nil || !slices.Contains(usagePresets, days) {
		return budget.DefaultDays
	}
	return days
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

func windowFilters(usageDays int, window string) []tileFilter {
	filters := make([]tileFilter, 0, 3)
	for _, value := range []string{windowLast, windowTen, windowAll} {
		filter := tileFilter{
			Active: value == window,
			Label:  value,
			URL:    overviewURL(usageDays, value),
		}
		filters = append(filters, filter)
	}
	return filters
}

func usageFilters(usageDays int, window string) []tileFilter {
	filters := make([]tileFilter, 0, len(usagePresets))
	for _, days := range usagePresets {
		filter := tileFilter{
			Active: days == usageDays,
			Label:  presetLabel(days),
			URL:    overviewURL(days, window),
		}
		filters = append(filters, filter)
	}
	return filters
}

// overviewURL omits defaults, so the plain Overview stays "/".
func overviewURL(usageDays int, window string) string {
	values := url.Values{}
	if usageDays != budget.DefaultDays {
		values.Set("usage", strconv.Itoa(usageDays))
	}
	if window != windowDefault {
		values.Set("window", window)
	}
	if len(values) == 0 {
		return "/"
	}
	return "/?" + values.Encode()
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

func lastAnalysisDetail(date, language string) string {
	if date == "" {
		return ""
	}
	return fmt.Sprintf(translate(language, "last %s"), date)
}

func signalDetail(date, language string) string {
	if date == "" {
		return ""
	}
	return fmt.Sprintf(translate(language, "last signal %s"), date)
}

// settingsTiles derives every settings.json-backed tile from one loadBoth call.
func (s *Server) settingsTiles() []overviewTile {
	main, disabled, err := s.loadBoth()
	if err != nil {
		return []overviewTile{errorTile(err, "/config/claude", "hooks", "Claude settings")}
	}

	parked, err := s.disabledHooks.Snapshot()
	if err != nil {
		return []overviewTile{errorTile(err, "/config/hooks", "hooks", "Hooks")}
	}

	var tiles []overviewTile
	tiles = append(tiles, hooksTile(main, disabled, parked))
	tiles = append(tiles, s.mcpTile(main))
	return tiles
}

func (s *Server) skillTiles() []overviewTile {
	list, err := skills.Scan(s.skillsRepo, s.skillsHome)
	if err != nil {
		return []overviewTile{errorTile(err, "/scripts/skills", "skills", skillsTitle)}
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
		Label:  skillsTitle,
		Value:  fmt.Sprintf("%d/%d", localCount, repoCount),
	}
	return []overviewTile{skillsTile}
}

func (s *Server) proposalsTile() overviewTile {
	open := s.openProposals()
	language := s.presentation.language()
	detail := translate(language, "open change commits — pick or reject on the Proposals page")
	tile := overviewTile{
		Id:     "proposals",
		Detail: detail,
		Href:   "/proposals",
		Label:  "Proposals",
		Value:  fmt.Sprintf("%d %s", open.Count, translate(language, "open")),
	}
	return tile
}

// repoTile runs no git: worktree counting is stat-only, the synced count reads each repo's context index.
func (s *Server) repoTile() overviewTile {
	registered := s.repoRegistry.Repos()
	sourceIndexPath := filepath.Join(s.contextDir, repos.ContextFileName)
	synced, worktrees := 0, 0
	for _, repo := range registered {
		worktrees += repos.CountPoolWorktrees(repo.Path)
		presence := repos.DetectContext(repo.Path, contextdocs.DeployRecordPath(s.stateDir, repo.Name))
		coverage, coverageErr := repos.DetectContextCoverage(sourceIndexPath, repo.Name, presence)
		if isContextSynced(presence, coverage, coverageErr) {
			synced++
		}
	}
	return overviewTile{
		Id:     "repos",
		Detail: fmt.Sprintf("%d synced · %d worktrees", synced, worktrees),
		Href:   "/repos",
		Label:  "Repos",
		Value:  fmt.Sprintf("%d", len(registered)),
	}
}

// routineTile counts broken or unprobeable routines as not active.
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

// usageOverviewTile shows the Usage page's Cost tile for one preset period over the first selected source.
func (s *Server) usageOverviewTile(now time.Time, usageDays int, window string) overviewTile {
	query := usageQuery{days: usageDays, source: s.usageSources()[0]}
	href := query.url(usagePath)
	view, err := s.loadUsage(now, query)
	if err != nil {
		return errorTile(err, href, "usage", labelUsage)
	}

	shares := view.planShares()
	cost := view.cell(pairCost, &view.total.cost, budget.CostTolerance).Values[0]
	tile := overviewTile{
		Id:      "usage",
		Detail:  joinDetail(usageCostDetail(cost), usageFableDetail(shares)),
		Filters: usageFilters(usageDays, window),
		Href:    href,
		Label:   labelUsage,
		Plans:   planOptions(view.plan),
		Value:   usagePlanValue(shares),
	}
	if view.storeErr != "" {
		tile.Detail, tile.Value = view.storeErr, usageDash
	}
	return tile
}

// usageCostDetail drops the dash of an empty period, which the tile value already shows.
func usageCostDetail(cost string) string {
	if cost == usageDash {
		return ""
	}
	return cost + " API equivalent"
}

func usageFableDetail(shares planShares) string {
	if shares.fable == 0 {
		return ""
	}
	return fmt.Sprintf("Fable %.0f%% weekly", shares.fable)
}

// usagePlanValue is "<5-hour window> / <weekly>" in percent, the window alone for a plan without a weekly share.
func usagePlanValue(shares planShares) string {
	if shares.weighed == 0 {
		return usageDash
	}
	if !shares.hasWeekly {
		return fmt.Sprintf("%.0f%%", shares.window)
	}
	return fmt.Sprintf("%.0f%% / %.0f%%", shares.window, shares.weekly)
}

// handleOverviewWelcomeTile always answers the tile, so the Overview keeps one fixed tile set.
func (s *Server) handleOverviewWelcomeTile(w http.ResponseWriter, r *http.Request) {
	s.renderFragment(w, tmplOverviewTile, s.welcomeTile(r.Context()))
}

// welcomeTile loads as a fragment because its peek probe can take the whole timeout while peek is cold.
func (s *Server) welcomeTile(ctx context.Context) overviewTile {
	selected, healthy := 0, 0
	detail := "all green"
	for _, row := range s.welcomeComponents(ctx).Rows {
		switch row.State {
		case componentOn:
			selected++
			healthy++
		case componentDown:
			selected++
			if detail == "all green" {
				detail = "next: " + row.Title
			}
		case componentOff, componentUnavailable:
		}
	}

	tile := overviewTile{
		Id:     "welcome",
		Detail: detail,
		Href:   "/welcome",
		Label:  "Setup",
		Value:  fmt.Sprintf("%d/%d components", healthy, selected),
	}
	return tile
}

// welcomePlaceholderTile keeps the tile's saved drag position until the fragment replaces it.
func welcomePlaceholderTile() overviewTile {
	tile := overviewTile{
		Id:       "welcome",
		Href:     "/welcome",
		Label:    "Setup",
		LazyHref: "/overview/welcome-tile",
		Value:    "…",
	}
	return tile
}

func countWorkflowScripts(files []string) int {
	count := 0
	for _, file := range files {
		if strings.HasPrefix(file, "workflows"+string(filepath.Separator)) && strings.HasSuffix(file, ".js") {
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

// hooksTile counts the parked groups of hooks.disabled.json; its detail line carries the disabled counts of the retired Disabled-entries card.
func hooksTile(main, disabled *config.Settings, parked map[string][]config.HookGroup) overviewTile {
	mainHooks, err := main.Hooks()
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
	for _, groups := range parked {
		disabledHookCount += len(groups)
	}
	total += disabledHookCount
	permCount := len(disabledPerms.Allow) + len(disabledPerms.Ask)

	return overviewTile{
		Id:     "hooks",
		Detail: fmt.Sprintf("disabled: %d hooks · %d permissions · %d MCP", disabledHookCount, permCount, len(disabledMcp)),
		Href:   "/config/hooks",
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

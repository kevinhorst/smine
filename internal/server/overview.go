package server

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/checklist"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/proposals"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/skills"
)

const statusDone = "Done"

type overviewPage struct {
	Page  string
	Tiles []overviewTile
	Title string
}

type overviewTile struct {
	Id     string
	Detail string
	Href   string
	Label  string
	Split  *tileSplit
	Trend  *tileTrend
	Value  string
}

// tileSplit renders a card whose two values are the links (the card body is
// not an anchor); markup lives in the template.
type tileSplit struct {
	FrustrationHref  string
	FrustrationValue string
	PositiveHref     string
	PositiveValue    string
}

// tileTrend carries sparkline polyline point strings; markup lives in the
// template (D2).
type tileTrend struct {
	FrustrationPoints string
	PositivePoints    string
}

// trendWindow is the number of most recent batches shown in the sparkline.
const trendWindow = 10

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	var tiles []overviewTile
	tiles = append(tiles, s.sessionTiles()...)
	tiles = append(tiles, s.settingsTiles()...)
	tiles = append(tiles, s.skillTiles()...)
	tiles = append(tiles, s.repoTile())
	tiles = append(tiles, s.contextTile())
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

// contextTile lists the source context languages — every context/ group
// except general (plans is already excluded by Scan).
func (s *Server) contextTile() overviewTile {
	groups, err := contextdocs.Scan(s.contextDir)
	if err != nil {
		return errorTile(err, "/context", "context", "Context")
	}

	var languages []string
	for _, group := range groups {
		if group.Name == "general" {
			continue
		}
		languages = append(languages, group.Name)
	}

	tile := overviewTile{
		Id:     "context",
		Detail: strings.Join(languages, " · "),
		Href:   "/context",
		Label:  "Context",
		Value:  fmt.Sprintf("%d", len(languages)),
	}
	return tile
}

func (s *Server) sessionTiles() []overviewTile {
	analyzed, batches, frustration, positive := 0, 0, 0, 0
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
				frustration += len(session.Frustration)
				positive += len(session.Positive)
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
	tiles := []overviewTile{sessionsTile, analysisTile, sentimentEntriesTile}
	if trend, ok := s.sentimentTile(frustrationHref); ok {
		tiles = append(tiles, trend)
	}
	return tiles
}

// sentimentTile renders the last trendWindow batches as a two-series
// sparkline; ok is false when no batches are loaded (the tile is omitted,
// matching the page's degrade convention). href reuses the frustration
// deep-link computed by sessionTiles (D5).
func (s *Server) sentimentTile(href string) (overviewTile, bool) {
	points := s.sessions.SentimentByBatch()
	if len(points) == 0 {
		return overviewTile{}, false
	}
	if len(points) > trendWindow {
		points = points[len(points)-trendWindow:]
	}

	maxValue := 1
	frustration := make([]int, len(points))
	positive := make([]int, len(points))
	for index, point := range points {
		frustration[index] = point.Frustration
		positive[index] = point.Positive
		maxValue = max(maxValue, point.Frustration, point.Positive)
	}

	latest := points[len(points)-1]
	tile := overviewTile{
		Id:    "sentiment",
		Href:  href,
		Label: "Sentiment trend",
		Trend: &tileTrend{
			FrustrationPoints: trendPoints(frustration, maxValue),
			PositivePoints:    trendPoints(positive, maxValue),
		},
		Value: fmt.Sprintf("%d", latest.Frustration),
	}
	if len(points) >= 2 {
		previous := points[len(points)-2]
		tile.Detail = fmt.Sprintf("%+d vs batch %d", latest.Frustration-previous.Frustration, previous.BatchNumber)
	}
	return tile, true
}

// trendPoints scales values into the sparkline's 100x28 viewBox with 2px
// vertical padding; maxValue is the shared scale across both series. A single
// value becomes a flat full-width line so a fresh install still shows one (D4).
func trendPoints(values []int, maxValue int) string {
	const width, height, pad = 100.0, 28.0, 2.0
	scale := func(value int) float64 {
		return height - pad - float64(value)/float64(maxValue)*(height-2*pad)
	}
	if len(values) == 1 {
		y := scale(values[0])
		return fmt.Sprintf("0,%.1f 100,%.1f", y, y)
	}

	var builder strings.Builder
	step := width / float64(len(values)-1)
	for index, value := range values {
		if index > 0 {
			builder.WriteByte(' ')
		}
		fmt.Fprintf(&builder, "%.1f,%.1f", float64(index)*step, scale(value))
	}
	return builder.String()
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
	tiles = append(tiles, disabledCountsTile(main, disabled))
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
		Detail: fmt.Sprintf("active / repo · %d out of sync", outOfSync),
		Href:   "/scripts/skills",
		Label:  "Skills",
		Value:  fmt.Sprintf("%d/%d", localCount, repoCount),
	}
	workflowsTile := overviewTile{
		Id:     "workflows",
		Detail: "skill workflow scripts",
		Href:   "/scripts/skills",
		Label:  "Workflows",
		Value:  fmt.Sprintf("%d", workflowScripts),
	}
	return []overviewTile{skillsTile, workflowsTile}
}

// proposalsTile counts actual proposals — status-bearing entries only, the
// same rule the proposals filters use; rerouted/considered notes don't count.
func (s *Server) proposalsTile() overviewTile {
	files, _, err := proposals.Load(filepath.Join(s.sessionsDir, "proposals"))
	if err != nil {
		return errorTile(err, "/proposals", "proposals", "Proposals")
	}

	total := 0
	var parts []string
	for _, file := range files {
		count := 0
		for _, group := range file.Groups {
			for index := range group.Proposals {
				if group.Proposals[index].Status != "" {
					count++
				}
			}
		}
		total += count
		parts = append(parts, fmt.Sprintf("%d %s", count, file.Kind))
	}

	return overviewTile{
		Id:     "proposals",
		Detail: strings.Join(parts, " · "),
		Href:   "/proposals",
		Label:  "Proposals",
		Value:  fmt.Sprintf("%d", total),
	}
}

// repoTile counts registered repos and their real agent worktrees (D3/D8);
// the registry is in memory and counting is stat-cheap — no git execution.
func (s *Server) repoTile() overviewTile {
	registered := s.repoRegistry.Repos()
	worktrees := 0
	for _, repo := range registered {
		worktrees += repos.CountWorktrees(repo.Path)
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

func countWorkflowScripts(files []string) int {
	count := 0
	for _, file := range files {
		if strings.HasPrefix(file, "workflows/") && strings.HasSuffix(file, ".js") {
			count++
		}
	}
	return count
}

func disabledCountsTile(main, disabled *config.Settings) overviewTile {
	disabledHooks, err := disabled.Hooks()
	if err != nil {
		return errorTile(err, "/config/claude", "disabled", "Disabled entries")
	}
	disabledPerms, err := disabled.Permissions()
	if err != nil {
		return errorTile(err, "/config/claude", "disabled", "Disabled entries")
	}
	disabledMcp, err := main.DisabledMcpjsonServers()
	if err != nil {
		return errorTile(err, "/config/claude", "disabled", "Disabled entries")
	}

	hookCount := 0
	for _, groups := range disabledHooks {
		hookCount += len(groups)
	}
	permCount := len(disabledPerms.Allow) + len(disabledPerms.Ask)

	tile := overviewTile{
		Id:     "disabled",
		Detail: fmt.Sprintf("%d hooks · %d permissions · %d MCP", hookCount, permCount, len(disabledMcp)),
		Href:   "/config/claude",
		Label:  "Disabled entries",
		Value:  fmt.Sprintf("%d", hookCount+permCount+len(disabledMcp)),
	}
	return tile
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

func hooksTile(main, disabled *config.Settings) overviewTile {
	mainHooks, err := main.Hooks()
	if err != nil {
		return errorTile(err, "/config/claude", "hooks", "Hooks")
	}
	disabledHooks, err := disabled.Hooks()
	if err != nil {
		return errorTile(err, "/config/claude", "hooks", "Hooks")
	}

	enabled, total := 0, 0
	for _, groups := range mainHooks {
		enabled += len(groups)
		total += len(groups)
	}
	for _, groups := range disabledHooks {
		total += len(groups)
	}

	tile := overviewTile{
		Id:    "hooks",
		Href:  "/config/claude?open=hooks#hooks",
		Label: "Hooks",
		Value: fmt.Sprintf("%d/%d enabled", enabled, total),
	}
	return tile
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

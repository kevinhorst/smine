package server

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"log"
	"maps"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/acdsl"
	"github.com/kevinhorst/smine/internal/budget"
	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/contextrecord"
	"github.com/kevinhorst/smine/internal/peek"
	"github.com/kevinhorst/smine/internal/skills"
	"github.com/kevinhorst/smine/internal/tokencount"
)

const (
	claudeConfigGroup       = ".claude"
	detachedHead            = "HEAD"
	sessionReportPathPrefix = "/sessions-report/"
	sizeKeyFileReads        = "file_reads"
	sizeKeyHooks            = "hooks"
	tmplSessionReport       = "session_report.html"
)

type agentRow struct {
	Id string

	Chars        string
	Description  string
	IsDimmed     bool
	Items        string
	Kind         string
	Model        string
	Name         string
	Pending      string
	PendingTitle string
	Tokens       string
	Type         string
	Url          string
}

// idChip links only ids that resolve to a live registry entry or acdsl rule.
type idChip struct {
	Id          string
	Description string
	Href        string
	Title       string
}

// idFamily groups chips sharing every id segment but the trailing number.
type idFamily struct {
	Chips []idChip
	Name  string
	Open  bool
}

// idMeta zero values mean unknown.
type idMeta struct {
	Description string
	Href        string
	Kind        string
	Lifetime    string
	Reach       string
	Scope       string
	Title       string
	Topic       string
}

type reportSections struct {
	FileReadRows   []sectionRow
	GlobalRows     []sectionRow
	HookRows       []sectionRow
	PlanFamilies   []idFamily
	ProjectionRows []sectionRow
	PromptRows     []sectionRow
	SkillRows      []sectionRow
}

// scopeLink is one agent-selection chip.
type scopeLink struct {
	IsActive bool
	Label    string
	Url      string
}

type sessionReportPage struct {
	reportSections

	Agents        []agentRow
	Branch        string
	ContextErr    string
	Counting      bool
	CountingUrl   string
	DownloadUrl   string
	Events        *peek.SessionEvents
	EventsErr     string
	Features      []featureRow
	FeaturesErr   string
	FileGroups    []touchedFileGroup
	Filter        sessionFilterBar
	HasFilter     bool
	InvokedSkills []skillRow
	IsRootInScope bool
	KindSizes     map[string]sectionSize
	Page          string
	PathBase      string
	PendingNotice string
	Plan          string
	Plans         []usagePlanOption
	Record        *contextrecord.Record
	Scope         agentScope
	ScopeLinks    []scopeLink
	SectionTags   map[string][]mechanismTag
	Session       peek.Session
	Stats         reportStats
	Title         string
	TokenNotice   string
	Totals        agentRow
}

type sessionFilterBar struct {
	Aspects   []aspectBarEntry
	Layers    []aspectBarEntry
	Lifetimes []aspectBarEntry
	Reaches   []aspectBarEntry
	Topics    []aspectBarEntry
}

// reportStats is shared by the page tiles and the markdown download; PlanPercent is "" without a weighable model split.
type reportStats struct {
	ActiveTime   string
	CacheHit     string
	CacheRead    string
	CacheWrite   string
	Cost         string
	IdleTime     string
	InputTokens  string
	OutputTokens string
	PlanPercent  string
	Runtime      string
	TotalTokens  string
}

// skillCatalog reads body token counts from the repo dir's changelog, not the home root.
type skillCatalog struct {
	deployed map[string]string
	repoDirs map[string]string
}

// skillRow.IsVersionAssumed marks the deployed version standing in for an unstamped one.
type skillRow struct {
	Ended            string
	IsVersionAssumed bool
	Skill            string
	SkillTokens      string
	Source           string
	Started          string
	Version          string
}

type touchedFile struct {
	Agents string
	Path   pathCell
	Reads  int
	Writes int
}

// touchedFileGroup buckets as peek does.
type touchedFileGroup struct {
	Files []touchedFile
	Name  string
}

// buildSessionIdIndex degrades a discovery failure to registry-only resolution.
func (s *Server) buildSessionIdIndex(ctx context.Context) map[string]idMeta {
	index := map[string]idMeta{}
	for _, entry := range s.loadRegistryEntries() {
		index[entry.Id] = idMeta{
			Description: plainText(entry.Content.Statement),
			Href:        "/context/entry/" + url.PathEscape(entry.Id),
			Kind:        entry.Kind,
			Reach:       entry.Reach,
			Scope:       entry.Scope,
			Title:       entryTitle(entry.Content.Statement),
			Topic:       entry.Topic,
		}
	}

	discovery, err := acdsl.DiscoverRules(ctx, ".")
	if err != nil {
		log.Printf("Server.buildSessionIdIndex: Acdsl discovery failed, rule ids stay unlinked: %v", err)
		return index
	}
	for _, rule := range discovery.Rules {
		index[rule.Id] = idMeta{
			Description: rule.Why,
			Href:        "/context/rule/" + url.PathEscape(rule.Id),
			Kind:        layerAcdsl,
			Lifetime:    rule.Lifetime,
			Reach:       rule.Reach,
			Scope:       ruleScope(rule.Id),
			Topic:       ruleTopic(rule.Id),
		}
	}
	return index
}

// fillRecordSections scopes every figure to the agent selection; only the agent table lists every agent.
func (s *Server) fillRecordSections(ctx context.Context, events *peek.SessionEvents, filter contextFilter, page *sessionReportPage, record *contextrecord.Record, scope agentScope) {
	base := sessionReportPathPrefix + url.PathEscape(page.Session.Id)
	scoped := scopedRecord(record, scope)
	page.Record = scoped
	page.Counting = s.countRuns.isRunning(page.Session.Id)
	page.CountingUrl = base + "/counting"
	index := s.buildSessionIdIndex(ctx)
	page.Filter = buildSessionFilterBar(base, filter, agentIds(scoped), index, scope)
	page.PathBase = record.Cwd
	page.reportSections = buildReportSections(s.claudeHome, filter, index, scoped)
	hints := warnHints(page.Features)
	warnHookRows(hints, page.HookRows)
	page.SectionTags = sectionTags(hints, page.HookRows)
	page.Agents = agentRows(base, filter, record, scope)
	page.ScopeLinks = scopeLinks(base, filter, record, scope)
	page.KindSizes = kindSizes(scoped)
	page.PendingNotice = pendingNotice(scoped, page.Counting)
	page.Totals = totalsRow(scoped)
	page.FileGroups = groupTouchedFiles(scoped, s.claudeHome)
	page.IsRootInScope = hasRootAgent(scoped)
	if page.IsRootInScope {
		page.InvokedSkills = s.skillRows(events, scoped)
	}
}

func (s *Server) handleSessionReport(w http.ResponseWriter, r *http.Request) {
	isFirstLoad := !r.URL.Query().Has(countedParam)
	page, ok := s.loadSessionReport(r.Context(), parseContextFilter(r), parseAgentScope(r), r.PathValue("id"), isFirstLoad)
	if !ok {
		http.NotFound(w, r)
		return
	}

	s.renderFragment(w, tmplSessionReport, page)
}

func (s *Server) handleSessionReportMarkdown(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	filter := parseContextFilter(r)
	page, ok := s.loadSessionReport(r.Context(), filter, parseAgentScope(r), id, true)
	if !ok {
		http.NotFound(w, r)
		return
	}

	filename := "session-" + shortId(id) + "-context-report.md"
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	if _, err := io.WriteString(w, renderSessionReportMarkdown(filter, id, page)); err != nil {
		log.Printf("Server.handleSessionReportMarkdown: Session %s: Failed to write response: %v", id, err)
	}
}

// loadRecord uses the stored breakdown only while it covers the session's last activity.
func (s *Server) loadRecord(ctx context.Context, events *peek.SessionEvents, session peek.Session) (*contextrecord.Record, bool, error) {
	breakdown, err := s.usage.Breakdown(session.Id)
	if err != nil {
		log.Printf("Server.loadRecord: Session %s: %v", session.Id, err)
	}
	if stored := currentRecord(breakdown, session.LastActive); stored != nil {
		return stored, true, nil
	}

	var turns []peek.Turn
	if session.Agent == contextrecord.SessionCodex {
		served, err := s.peekClient.SessionTurns(ctx, session.Agent, session.Id)
		if err != nil {
			return nil, false, fmt.Errorf("Server.loadRecord: %w", err)
		}
		turns = served
	}
	record, err := budget.ExtractRecord(s.claudeHome, filepath.Base(s.contextDir), events, &session, turns)
	if err != nil {
		return nil, false, fmt.Errorf("Server.loadRecord: %w", err)
	}
	s.tokens.CountCached(ctx, record)
	return record, false, nil
}

// loadSessionReport never waits on uncached token counts; a background run counts them.
func (s *Server) loadSessionReport(ctx context.Context, filter contextFilter, scope agentScope, id string, shouldCount bool) (*sessionReportPage, bool) {
	peekCtx, cancel := context.WithTimeout(ctx, peekSessionsTimeout)
	byId, err := s.peekClient.SessionsById(peekCtx)
	cancel()
	if err != nil {
		if stored, ok := s.loadStoredReport(ctx, filter, id, scope); ok {
			return stored, true
		}

		page := &sessionReportPage{
			EventsErr: fmt.Sprintf("peek-mcp unreachable at %s: %v", s.peekClient.Endpoint(), err),
			Page:      pageSessions,
			Session:   peek.Session{Id: id},
			Title:     "Session report",
		}
		s.loadSessionFeatures(page, id)
		return page, true
	}

	session, ok := byId[id]
	if !ok {
		return s.loadStoredReport(ctx, filter, id, scope)
	}

	base := sessionReportPathPrefix + url.PathEscape(id)
	page := &sessionReportPage{
		DownloadUrl: scopeURL(base+"/md", filter, scope),
		HasFilter:   filter.Active(),
		Page:        pageSessions,
		Scope:       scope,
		Session:     session,
		Title:       "Session — " + shortId(id),
	}
	s.loadSessionFeatures(page, id)
	events, err := s.peekClient.SessionEvents(ctx, id)
	if err != nil {
		page.EventsErr = err.Error()
	}

	page.Events = events
	if events != nil {
		page.Stats = newReportStats(events)
		s.fillReportPlan(id, page)
	}

	record, isStored, err := s.loadRecord(ctx, events, session)
	if err != nil {
		page.ContextErr = err.Error()
	}

	page.TokenNotice = s.tokens.Notice()
	page.Branch = sessionBranch(record, session)
	if record != nil {
		if !isStored {
			s.settleBreakdown(id, session.LastActive, record, shouldCount)
		}
		s.fillRecordSections(ctx, events, filter, page, record, scope)
	}
	return page, true
}

// fillReportPlan reads the stored plan and the session's usage record; the percent stays "" without a weighable model split.
func (s *Server) fillReportPlan(id string, page *sessionReportPage) {
	plan := budget.DefaultPlan
	if settings, err := s.usage.Settings(); err == nil {
		plan = settings.Plan
	}
	page.Plan = plan.Label()
	page.Plans = planOptions(plan)

	record, err := s.usage.Record(id)
	if err != nil || record == nil {
		return
	}

	usage, hasPlan := sessionPlanUsage(record)
	if !hasPlan {
		return
	}
	page.Stats.PlanPercent = fmt.Sprintf("%.0f%%", usage.All/plan.WindowCapacity()*percentScale)
}

// loadSkillCatalog scans both skill roots once per report.
func (s *Server) loadSkillCatalog() skillCatalog {
	found := skillCatalog{deployed: map[string]string{}, repoDirs: map[string]string{}}
	list, err := skills.Scan(s.skillsRepo, s.skillsHome)
	if err != nil {
		log.Printf("Server.loadSkillCatalog: Skill scan failed: %v", err)
		return found
	}

	for index := range list {
		skill := &list[index]
		switch skill.Origin {
		case skills.OriginHome:
			found.deployed[skill.Name] = skill.Version
			for _, alias := range skill.Aliases {
				found.deployed[alias] = skill.Version
			}
		case skills.OriginRepo:
			found.repoDirs[skill.Name] = skill.Path
			for _, alias := range skill.Aliases {
				found.repoDirs[alias] = skill.Path
			}
		}
	}
	return found
}

// loadStoredReport serves sessions peek no longer knows, or while peek is down.
func (s *Server) loadStoredReport(ctx context.Context, filter contextFilter, id string, scope agentScope) (*sessionReportPage, bool) {
	record, err := s.usage.Record(id)
	if err != nil || record == nil {
		return nil, false
	}
	breakdown, err := s.usage.Breakdown(id)
	if err != nil || breakdown == nil {
		return nil, false
	}
	if breakdown.Record == nil {
		return nil, false
	}

	session := peek.Session{
		Id:         id,
		Agent:      record.Agent,
		Cwd:        record.Cwd,
		LastActive: record.LastActivity(),
		Title:      record.Title,
	}
	base := sessionReportPathPrefix + url.PathEscape(id)
	page := &sessionReportPage{
		DownloadUrl: scopeURL(base+"/md", filter, scope),
		EventsErr:   "peek does not serve this session — the context below is the stored breakdown",
		HasFilter:   filter.Active(),
		Page:        pageSessions,
		Scope:       scope,
		Session:     session,
		Title:       "Session — " + shortId(id),
	}
	s.loadSessionFeatures(page, id)
	page.TokenNotice = s.tokens.Notice()
	page.Branch = sessionBranch(breakdown.Record, session)
	s.fillRecordSections(ctx, nil, filter, page, breakdown.Record, scope)
	return page, true
}

// skillRows pairs the n-th invocation of a skill with the n-th context of that skill.
func (s *Server) skillRows(events *peek.SessionEvents, record *contextrecord.Record) []skillRow {
	if events == nil {
		return nil
	}

	known := s.loadSkillCatalog()
	versions := map[string][]string{}
	for _, skill := range record.Injected.Skill {
		versions[skill.Skill] = append(versions[skill.Skill], skill.Version)
	}
	runs := map[string][]peek.SkillRun{}
	for _, run := range events.SkillRuns {
		runs[run.Skill] = append(runs[run.Skill], run)
	}
	seen := map[string]int{}
	rows := make([]skillRow, 0, len(events.InvokedSkills))
	for _, invocation := range events.InvokedSkills {
		row := skillRow{
			Ended:       "—",
			Skill:       invocation.Skill,
			SkillTokens: "—",
			Source:      invocation.Source,
			Started:     invocation.Timestamp.Format(time.DateTime),
			Version:     "—",
		}
		ordinal := seen[invocation.Skill]
		seen[invocation.Skill]++
		if ordinal < len(runs[invocation.Skill]) {
			run := runs[invocation.Skill][ordinal]
			row.Started = run.StartedAt.Format(time.DateTime)
			if !run.EndedAt.IsZero() {
				row.Ended = run.EndedAt.Format(time.DateTime)
			}
		}
		recorded := ""
		if ordinal < len(versions[invocation.Skill]) {
			recorded = versions[invocation.Skill][ordinal]
		}
		version := cmp.Or(recorded, known.deployed[invocation.Skill])
		if version != "" {
			row.IsVersionAssumed = recorded == ""
			row.Version = version
			if tokens, ok := skillBodyTokens(known.repoDirs[invocation.Skill], version); ok {
				row.SkillTokens = groupThousands(int64(tokens))
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// settleBreakdown leaves storing a record with pending items to the background run.
func (s *Server) settleBreakdown(id string, lastActive time.Time, record *contextrecord.Record, shouldCount bool) {
	if recordPending(record) == 0 {
		s.storeBreakdown(id, lastActive, record)
		return
	}
	if shouldCount {
		s.startCounting(id, lastActive, record)
	}
}

func (s *Server) storeBreakdown(id string, lastActive time.Time, record *contextrecord.Record) {
	breakdown := budget.NewBreakdown(time.Now(), id, lastActive, record)
	if err := s.usage.SaveBreakdown(breakdown); err != nil {
		log.Printf("Server.storeBreakdown: Session %s: %v", id, err)
	}
}

// addTotals adds one kind's totals to a section's.
func addTotals(sum, part *contextrecord.Totals) {
	sum.Chars += part.Chars
	sum.Items += part.Items
	sum.Lines += part.Lines
	sum.Tokens += part.Tokens
	sum.Uncounted += part.Uncounted
}

func agentIds(record *contextrecord.Record) []string {
	var ids []string
	for index := range record.Agents {
		view := record.Agents[index].Context
		if view == nil {
			continue
		}

		lists := contextrecord.Record{
			AcdslRules:      view.AcdslRules,
			Injected:        view.Injected,
			PlanRules:       view.PlanRules,
			SubagentContext: view.SubagentContext,
		}
		ids = append(ids, lists.AllIds()...)
	}
	return ids
}

func agentKindWord(kind contextrecord.AgentKind) string {
	switch kind {
	case contextrecord.AgentRoot:
		return "Root"
	case contextrecord.AgentSubagent:
		return "Subagent"
	case contextrecord.AgentWorkflow:
		return "Workflow"
	}
	return string(kind)
}

func agentName(agent *contextrecord.Agent) string {
	if agent.Kind == contextrecord.AgentRoot {
		return agent.Id
	}
	if agent.Type != "" {
		return agent.Type
	}
	return agent.Id
}

// agentRows covers the full record, not the selection; rows outside it are dimmed.
func agentRows(base string, filter contextFilter, record *contextrecord.Record, scope agentScope) []agentRow {
	rows := make([]agentRow, 0, len(record.Agents))
	for index := range record.Agents {
		agent := &record.Agents[index]
		pending := pendingCount(agent.Items)
		agentSelection := agentScope{id: agent.Id}
		row := agentRow{
			Id:          agent.Id,
			Chars:       groupThousands(int64(agent.Totals.Chars)),
			Description: agent.Description,
			IsDimmed:    !scope.matches(agent),
			Items:       strconv.Itoa(agent.Totals.Items),
			Kind:        agentKindWord(agent.Kind),
			Model:       agent.Model,
			Name:        agentName(agent),
			Tokens:      totalsTokenCell(&agent.Totals),
			Type:        agent.Type,
			Url:         scopeURL(base, filter, agentSelection),
		}
		if pending > 0 {
			row.Pending = strconv.Itoa(pending)
			row.PendingTitle = "not reached by the counter yet — reload to continue"
		}
		rows = append(rows, row)
	}
	return rows
}

// agentTypes lists the agent types present, sorted, for the filter chips.
func agentTypes(record *contextrecord.Record) []string {
	seen := map[string]bool{}
	for index := range record.Agents {
		agentType := record.Agents[index].Type
		if agentType != "" {
			seen[agentType] = true
		}
	}
	return slices.Sorted(maps.Keys(seen))
}

func buildReportSections(claudeHome string, filter contextFilter, index map[string]idMeta, record *contextrecord.Record) reportSections {
	sections := reportSections{
		FileReadRows:   fileReadRows(claudeHome, filter, index, record),
		GlobalRows:     globalRows(claudeHome, record),
		HookRows:       hookInjectionRows(claudeHome, record),
		ProjectionRows: projectionRows(claudeHome, filter, index, record),
		PromptRows:     promptRows(claudeHome, filter, index, record),
		SkillRows:      skillContextRows(filter, index, record),
	}
	for agentIndex := range record.Agents {
		view := record.Agents[agentIndex].Context
		if view == nil {
			continue
		}

		chips := filterIds(filter, view.PlanRules, index)
		sections.PlanFamilies = append(sections.PlanFamilies, groupIdFamilies(chips, filter.Active())...)
	}
	return sections
}

// buildSessionFilterBar counts describe the page, not the registry.
func buildSessionFilterBar(base string, filter contextFilter, ids []string, index map[string]idMeta, scope agentScope) sessionFilterBar {
	var bar sessionFilterBar
	bar.Layers = axisChips(base, filter, "layer", filter.Layer,
		[]string{layerAcdsl, contextdocs.RuleKindAction, contextdocs.RuleKindRule, contextdocs.RuleKindFact})
	bar.Lifetimes = axisChips(base, filter, "lifetime", filter.Lifetime,
		[]string{acdsl.LifetimeTask, acdsl.LifetimeDoctrine})
	scopeChipLinks("layer", base, bar.Layers, filter, scope)
	scopeChipLinks("lifetime", base, bar.Lifetimes, filter, scope)

	ruleCounts := map[string]int{}
	entryCounts := map[string]int{}
	reachRuleCounts := map[string]int{}
	reachEntryCounts := map[string]int{}
	topicCounts := map[string]int{}
	for _, id := range ids {
		meta := idMetaFor(id, index)
		isRule := meta.Kind == layerAcdsl
		for _, chip := range reachChips(meta.Reach) {
			if isRule {
				reachRuleCounts[chip]++
			} else {
				reachEntryCounts[chip]++
			}
		}
		if !reachMatches(meta.Reach, filter.Reach) {
			continue
		}
		if meta.Scope == "" {
			continue
		}
		if isRule {
			ruleCounts[meta.Scope]++
		} else {
			entryCounts[meta.Scope]++
		}
		if meta.Scope == filter.Aspect {
			topicCounts[meta.Topic]++
		}
	}

	bar.Reaches = append([]aspectBarEntry{allChip(base, filter, "reach", filter.Reach)},
		reachBarEntries(reachRuleCounts, reachEntryCounts)...)
	for index := range bar.Reaches {
		if bar.Reaches[index].Name == "all" {
			continue
		}
		bar.Reaches[index].Active = bar.Reaches[index].Name == filter.Reach
	}
	scopeChipLinks("reach", base, bar.Reaches, filter, scope)

	bar.Aspects = append(bar.Aspects, allChip(base, filter, "aspect", filter.Aspect))
	for _, aspect := range slices.Sorted(maps.Keys(mergedKeys(ruleCounts, entryCounts))) {
		chip := aspectBarEntry{
			Active:     aspect == filter.Aspect,
			Counts:     true,
			EntryCount: entryCounts[aspect],
			Name:       aspect,
			Registered: true,
			RuleCount:  ruleCounts[aspect],
		}
		bar.Aspects = append(bar.Aspects, chip)
	}
	scopeChipLinks("aspect", base, bar.Aspects, filter, scope)

	if filter.Aspect != "" {
		bar.Topics = append(bar.Topics, allChip(base, filter, "topic", filter.Topic))
		for _, topic := range slices.Sorted(maps.Keys(topicCounts)) {
			if topic == "" {
				continue
			}
			chip := aspectBarEntry{
				Active:     topic == filter.Topic,
				Counts:     true,
				EntryCount: topicCounts[topic],
				Name:       topic,
				Registered: true,
			}
			bar.Topics = append(bar.Topics, chip)
		}
		scopeChipLinks("topic", base, bar.Topics, filter, scope)
	}
	return bar
}

// currentRecord is the stored record when the breakdown covers lastActive.
func currentRecord(breakdown *budget.Breakdown, lastActive time.Time) *contextrecord.Record {
	if breakdown == nil || !breakdown.IsCurrent(lastActive) {
		return nil
	}
	return breakdown.Record
}

// filterIds projects raw id strings to chips, dropping non-matching ids.
func filterIds(filter contextFilter, ids []string, index map[string]idMeta) []idChip {
	var chips []idChip
	for _, id := range ids {
		meta := idMetaFor(id, index)
		if !matchesFilter(filter, meta) {
			continue
		}
		chip := idChip{Id: id, Description: meta.Description, Href: meta.Href, Title: meta.Title}
		chips = append(chips, chip)
	}
	return chips
}

// fileExtension matches peek's bucket key.
func fileExtension(path string) string {
	extension := strings.ToLower(filepath.Ext(path))
	if extension == "" {
		return "(none)"
	}
	return extension
}

// entryTitlePattern captures a bold lead phrase such as "**Pivotal marker**: …".
var entryTitlePattern = regexp.MustCompile(`^\*\*([^*]+)\*\*`)

func entryTitle(statement string) string {
	match := entryTitlePattern.FindStringSubmatch(statement)
	if match == nil {
		return ""
	}
	title := strings.ReplaceAll(match[1], "`", "")
	return strings.TrimRight(strings.TrimSpace(title), ".:")
}

func plainText(markdown string) string {
	return strings.NewReplacer("**", "", "`", "").Replace(markdown)
}

func groupIdFamilies(chips []idChip, open bool) []idFamily {
	var families []idFamily
	byName := map[string]int{}
	for _, chip := range chips {
		name := idFamilyName(chip.Id)
		index, ok := byName[name]
		if !ok {
			index = len(families)
			byName[name] = index
			families = append(families, idFamily{Name: name, Open: open})
		}
		families[index].Chips = append(families[index].Chips, chip)
	}
	return families
}

// groupTouchedFiles buckets the way peek's dashboard does.
func groupTouchedFiles(record *contextrecord.Record, claudeHome string) []touchedFileGroup {
	var paths []string
	byPath := map[string]*touchedFile{}
	agentsByPath := map[string][]string{}
	for agentIndex := range record.Agents {
		agent := &record.Agents[agentIndex]
		for _, touch := range agent.Files {
			file, isKnown := byPath[touch.Path]
			if !isKnown {
				file = &touchedFile{Path: relativePath(touch.Path, record.Cwd, claudeHome)}
				byPath[touch.Path] = file
				paths = append(paths, touch.Path)
			}
			file.Reads += touch.Reads
			file.Writes += touch.Writes
			agentsByPath[touch.Path] = append(agentsByPath[touch.Path], agentName(agent))
		}
	}

	var groups []touchedFileGroup
	var config []touchedFile
	byName := map[string]int{}
	for _, path := range paths {
		file := byPath[path]
		file.Agents = agentsLabel(agentsByPath[path])
		if isClaudeConfigPath(path) {
			config = append(config, *file)
			continue
		}

		name := fileExtension(path)
		index, ok := byName[name]
		if !ok {
			index = len(groups)
			byName[name] = index
			groups = append(groups, touchedFileGroup{Name: name})
		}
		groups[index].Files = append(groups[index].Files, *file)
	}

	slices.SortFunc(groups, func(first, second touchedFileGroup) int {
		if sizeOrder := len(second.Files) - len(first.Files); sizeOrder != 0 {
			return sizeOrder
		}
		return strings.Compare(first.Name, second.Name)
	})
	if len(config) > 0 {
		configGroup := touchedFileGroup{Files: config, Name: claudeConfigGroup}
		groups = append(groups, configGroup)
	}
	for index := range groups {
		slices.SortFunc(groups[index].Files, func(first, second touchedFile) int {
			return strings.Compare(first.Path.String(), second.Path.String())
		})
	}
	return groups
}

// hasRootAgent reports whether the record holds the root agent.
func hasRootAgent(record *contextrecord.Record) bool {
	return slices.ContainsFunc(record.Agents, func(agent contextrecord.Agent) bool { return agent.Kind == contextrecord.AgentRoot })
}

// idFamilyName maps RULE-PLAN-041 to RULE-PLAN.
func idFamilyName(id string) string {
	cut := strings.LastIndex(id, "-")
	if cut < 0 {
		return id
	}
	if _, err := strconv.Atoi(id[cut+1:]); err != nil {
		return id
	}
	return id[:cut]
}

// idMetaFor keeps retired or foreign ids filterable but unlinked.
func idMetaFor(id string, index map[string]idMeta) idMeta {
	if meta, ok := index[id]; ok {
		return meta
	}

	meta := idMeta{Scope: ruleScope(id), Topic: ruleTopic(id)}
	switch {
	case strings.HasPrefix(id, "ACDSL-"):
		meta.Kind = layerAcdsl
	case strings.HasPrefix(id, "ACTION-"):
		meta.Kind = contextdocs.RuleKindAction
	case strings.HasPrefix(id, "RULE-"):
		meta.Kind = contextdocs.RuleKindRule
	case strings.HasPrefix(id, "FACT-"):
		meta.Kind = contextdocs.RuleKindFact
	}
	return meta
}

// isClaudeConfigPath follows peek's config bucket rule.
func isClaudeConfigPath(path string) bool {
	segments := strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' })
	for index, segment := range segments {
		if segment != ".claude" {
			continue
		}

		isWorktree := index+1 < len(segments) && segments[index+1] == "worktrees"
		if isWorktree {
			continue
		}
		return true
	}
	return false
}

func isHookKind(kind contextrecord.Kind) bool {
	switch kind {
	case contextrecord.KindAcdslContext, contextrecord.KindAgentsMd, contextrecord.KindCodexEnvironment,
		contextrecord.KindCodexInstructions, contextrecord.KindHookContext, contextrecord.KindPlanContext:
		return true
	case contextrecord.KindAcdslProjection, contextrecord.KindGlobal, contextrecord.KindLang, contextrecord.KindPackRead,
		contextrecord.KindPlanRules, contextrecord.KindSkill, contextrecord.KindSubagentPrompt:
		return false
	}
	return false
}

func kindSizes(record *contextrecord.Record) map[string]sectionSize {
	pending := map[contextrecord.Kind]int{}
	for agentIndex := range record.Agents {
		for itemIndex := range record.Agents[agentIndex].Items {
			item := &record.Agents[agentIndex].Items[itemIndex]
			if isPending(item) {
				pending[item.Kind]++
			}
		}
	}

	byKind := record.ByKind()
	sizes := make(map[string]sectionSize)
	for kind, totals := range byKind {
		sizes[string(kind)] = sizeBadge(&totals, pending[kind])
	}

	// "Context files read" joins guide and pack reads; "Hook injections" joins every hook kind.
	var fileReads, hooks contextrecord.Totals
	var filePending, hookPending int
	for kind, totals := range byKind {
		switch {
		case kind == contextrecord.KindPackRead, kind == contextrecord.KindLang:
			addTotals(&fileReads, &totals)
			filePending += pending[kind]
		case isHookKind(kind):
			addTotals(&hooks, &totals)
			hookPending += pending[kind]
		}
	}
	if fileReads.Items > 0 {
		sizes[sizeKeyFileReads] = sizeBadge(&fileReads, filePending)
	}
	if hooks.Items > 0 {
		sizes[sizeKeyHooks] = sizeBadge(&hooks, hookPending)
	}
	return sizes
}

// matchesFilter narrows only acdsl ids by lifetime; unknown axes match only "all".
func matchesFilter(filter contextFilter, meta idMeta) bool {
	if filter.Layer != "" && meta.Kind != filter.Layer {
		return false
	}
	if filter.Aspect != "" && meta.Scope != filter.Aspect {
		return false
	}
	if filter.Topic != "" && meta.Topic != filter.Topic {
		return false
	}
	if !reachMatches(meta.Reach, filter.Reach) {
		return false
	}

	lifetimeApplies := filter.Lifetime != "" && meta.Kind == layerAcdsl
	if lifetimeApplies && meta.Lifetime != filter.Lifetime {
		return false
	}
	return true
}

// mergedKeys unions the key sets of two count maps.
func mergedKeys(first, second map[string]int) map[string]bool {
	keys := map[string]bool{}
	for key := range first {
		keys[key] = true
	}
	for key := range second {
		keys[key] = true
	}
	return keys
}

// newReportStats uses peek's cache-hit ratio: cache reads over all input-side tokens.
func newReportStats(events *peek.SessionEvents) reportStats {
	usage := events.Usage
	inputSide := usage.InputTokens + usage.CacheCreationInputTokens + usage.CacheReadInputTokens
	stats := reportStats{
		ActiveTime:   formatDuration(events.ActiveSeconds),
		CacheRead:    groupThousands(int64(usage.CacheReadInputTokens)),
		CacheWrite:   groupThousands(int64(usage.CacheCreationInputTokens)),
		Cost:         "—",
		IdleTime:     formatDuration(events.IdleSeconds),
		InputTokens:  groupThousands(int64(usage.InputTokens)),
		OutputTokens: groupThousands(int64(usage.OutputTokens)),
		Runtime:      formatDuration(events.WallSeconds),
		TotalTokens:  groupThousands(int64(inputSide + usage.OutputTokens)),
	}
	if events.HasTelemetry {
		stats.Cost = fmt.Sprintf("$%.2f", events.CostUsd)
	}
	if inputSide > 0 {
		stats.CacheHit = fmt.Sprintf("%.0f%%", float64(usage.CacheReadInputTokens)/float64(inputSide)*percentScale)
	}
	return stats
}

// pendingNotice is the throttle explanation while any item is pending.
func pendingNotice(record *contextrecord.Record, isCounting bool) string {
	pending := recordPending(record)
	if pending == 0 {
		return ""
	}
	if isCounting {
		return fmt.Sprintf(countingNoticeFormat, pending, tokencount.RatePerMinute, tokencount.MaxItemsPerReport)
	}
	return fmt.Sprintf(pendingNoticeFormat, tokencount.RatePerMinute, tokencount.MaxItemsPerReport, pending)
}

func scopeChipLinks(axis, base string, chips []aspectBarEntry, filter contextFilter, scope agentScope) {
	for index := range chips {
		value := chips[index].Name
		if value == "all" {
			value = ""
		}
		chips[index].URL = scopeURL(base, filter.with(axis, value), scope)
	}
}

func scopeLinks(base string, filter contextFilter, record *contextrecord.Record, scope agentScope) []scopeLink {
	selections := []agentScope{{}, {isRoot: true}}
	labels := []string{"all", scopeRoot}
	for _, agentType := range agentTypes(record) {
		selections = append(selections, agentScope{agentType: agentType})
		labels = append(labels, agentType)
	}

	links := make([]scopeLink, 0, len(selections))
	for index, selection := range selections {
		link := scopeLink{
			IsActive: selection == scope,
			Label:    labels[index],
			Url:      scopeURL(base, filter, selection),
		}
		links = append(links, link)
	}
	return links
}

// skillBodyTokens needs the repo skill dir; a home-only copy has no changelog.
func skillBodyTokens(skillDir, version string) (int, bool) {
	if skillDir == "" {
		return 0, false
	}

	changelog, err := loadChangelog(skillDir)
	if err != nil {
		log.Printf("skillBodyTokens: %s: %v", skillDir, err)
		return 0, false
	}

	for _, entry := range changelog {
		if entry.Version == version && entry.Tokens > 0 {
			return entry.Tokens, true
		}
	}
	return 0, false
}

// sessionBranch prefers the transcript's branch over peek's live value; a detached HEAD is no branch.
func sessionBranch(record *contextrecord.Record, session peek.Session) string {
	if record != nil && record.GitBranch != "" {
		return record.GitBranch
	}
	if session.GitBranch == detachedHead {
		return ""
	}
	return session.GitBranch
}

func totalsRow(record *contextrecord.Record) agentRow {
	pending := 0
	for agentIndex := range record.Agents {
		pending += pendingCount(record.Agents[agentIndex].Items)
	}
	row := agentRow{
		Chars:  groupThousands(int64(record.Totals.Chars)),
		Items:  strconv.Itoa(record.Totals.Items),
		Tokens: totalsTokenCell(&record.Totals),
	}
	if pending > 0 {
		row.Pending = strconv.Itoa(pending)
	}
	return row
}

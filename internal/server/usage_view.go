package server

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/budget"
	"github.com/kevinhorst/smine/internal/contextrecord"
)

const (
	// usageChartHalf places the chart's middle axis label.
	usageChartHalf = 0.5
	// usageChartLabelDays is the day count up to which every column carries its labels.
	usageChartLabelDays = 14
	usageDash           = "—"
	usageDayLabelLayout = "01-02"
	usageOutsideSkills  = "outside skills"
	// usageRemainderRows: outside skills and subagents.
	usageRemainderRows = 2
	usageSessionsLabel = "Sessions"
	// usageShownSkills is how many skills a row lists before the rest folds.
	usageShownSkills = 3
	usageSubagents   = "subagents"
	usageUnknownRepo = "unknown"
)

// usageCsvHeader names the columns of csvRow in order.
var usageCsvHeader = []string{
	"id", "agent", "title", "repo", sectionSkills, "last_activity",
	"ccusage_input", "ccusage_output", "ccusage_cache_creation", "ccusage_cache_read", "ccusage_cost_usd",
	"peek_input", "peek_output", "peek_cache_creation", "peek_cache_read", "peek_cost_usd",
	"ccusage_wall_seconds", "peek_wall_seconds", "active_seconds", "idle_seconds",
	"context_items", "context_chars", "context_tokens", "plan_percent", "agreement",
}

type usageCell struct {
	Badge   string
	Values  []string
	Verdict budget.Agreement
}

// usageChart.Plan is set under the plan unit only.
type usageChart struct {
	Average string
	Days    []usageDay
	Half    string
	Max     string
	Plan    string
	Total   string
	Units   []usageChip
}

type usageChip struct {
	Active bool
	IsAll  bool
	Label  string
	Url    string
}

// usageDay.Height is its share of the busiest day in percent.
type usageDay struct {
	Height string
	Label  string
	Title  string
	Value  string
}

// usageField is one hidden input of the range form.
type usageField struct {
	Name  string
	Value string
}

type usageGroupRow struct {
	Cost     usageCell
	Name     string
	Sessions int
	Tokens   usageCell
	Url      string
	Wall     usageCell
}

type usageKindRow struct {
	tokens int

	Chars  string
	Items  int
	Kind   contextrecord.Kind
	Tokens string
}

type usagePage struct {
	Agents         []usageChip
	Chart          *usageChart
	ContextCovered int
	ContextKinds   []usageKindRow
	CsvUrl         string
	From           string
	HasCcusage     bool
	IsBoth         bool
	Only           usageChip
	Page           string
	Plan           string
	Plans          []usagePlanOption
	Presets        []usageChip
	RangeFields    []usageField
	RepoRows       []usageGroupRow
	Repos          []usageChip
	RunErrors      []string
	RunTime        string
	Sessions       []usageSessionRow
	Skill          string
	SkillRows      []usageSkillRow
	Skills         []usageChip
	Sources        []usageChip
	StoreErr       string
	Tiles          []usageTile
	Title          string
	To             string
	Total          int
}

// usagePlanOption is one button of the page's plan switch.
type usagePlanOption struct {
	IsSelected bool
	Label      string
	Value      string
}

// usageSession is the row source of both the session table and the CSV.
type usageSession struct {
	record *budget.Record
	sums   usageSums
}

type usageSessionRow struct {
	Id string

	Agent              string
	Badge              string
	ContextChars       string
	ContextTokens      string
	ContextTokensValue int
	Cost               usageCell
	HasContext         bool
	LastActive         string
	MoreSkills         string
	MoreSkillsCount    int
	PlanPercent        string
	Repo               string
	ShortId            string
	Skills             string
	Title              string
	Tokens             usageCell
	TokensValue        int64
	Verdict            budget.Agreement
	Wall               usageCell
}

// usageSkillRow.CacheReadValue and TokensValue are the sort keys of their formatted columns.
type usageSkillRow struct {
	seconds int

	CacheRead      string
	CacheReadValue int
	Name           string
	Runs           int
	Seconds        string
	Tokens         string
	TokensValue    int
	Url            string
}

type usageSums struct {
	active          int
	cacheCreation   budget.Pair
	cacheRead       budget.Pair
	contextChars    int
	contextItems    int
	contextSessions int
	contextTokens   int
	cost            budget.Pair
	hasPlan         bool
	idle            int
	input           budget.Pair
	output          budget.Pair
	plan            budget.PlanUsage
	sessions        int
	wall            budget.Pair
}

// newSkillSums is one session's sums narrowed to one skill's runs: peek only, cost and plan by the runs' token share.
func newSkillSums(record *budget.Record, skill string) usageSums {
	sums := usageSums{sessions: 1}
	if record.Peek == nil {
		return sums
	}

	usage, seconds := record.Peek.SkillUsage(skill)
	share := record.Share(usage)
	plan, hasPlan := sessionPlanUsage(record)
	sums.active = seconds
	sums.hasPlan = hasPlan
	sums.plan = plan.Scale(share)
	sums.cacheCreation.Add(0, false, true, float64(usage.CacheCreation))
	sums.cacheRead.Add(0, false, true, float64(usage.CacheRead))
	sums.input.Add(0, false, true, float64(usage.Input))
	sums.output.Add(0, false, true, float64(usage.Output))
	sums.cost.Add(0, false, record.Peek.HasCost, record.Peek.CostUsd*share)
	sums.wall.Add(0, false, true, float64(seconds))
	return sums
}

// newUsageSums builds one session's sums from its record.
func newUsageSums(record *budget.Record) usageSums {
	sums := usageSums{sessions: 1}
	var peekTokens, ccusageTokens budget.TokenClasses
	peekCost, ccusageCost, peekWall, ccusageWall := 0.0, 0.0, 0.0, 0.0
	hasPeek, hasCcusage := record.Peek != nil, record.Ccusage != nil
	hasPeekCost, hasCcusageCost, hasCcusageWall := false, false, false
	if hasPeek {
		peekTokens = record.Peek.Total()
		peekCost, hasPeekCost = record.Peek.CostUsd, record.Peek.HasCost
		peekWall = float64(record.Peek.WallSeconds)
		sums.active, sums.idle = record.Peek.ActiveSeconds, record.Peek.IdleSeconds
	}
	if hasCcusage {
		ccusageTokens = record.Ccusage.Usage
		ccusageCost, hasCcusageCost = record.Ccusage.CostUsd, !record.Ccusage.IsUnpriced
		ccusageWall, hasCcusageWall = float64(record.Ccusage.WallSeconds()), !record.Ccusage.FirstActivity.IsZero()
	}
	sums.cacheCreation.Add(float64(ccusageTokens.CacheCreation), hasCcusage, hasPeek, float64(peekTokens.CacheCreation))
	sums.cacheRead.Add(float64(ccusageTokens.CacheRead), hasCcusage, hasPeek, float64(peekTokens.CacheRead))
	sums.input.Add(float64(ccusageTokens.Input), hasCcusage, hasPeek, float64(peekTokens.Input))
	sums.output.Add(float64(ccusageTokens.Output), hasCcusage, hasPeek, float64(peekTokens.Output))
	sums.cost.Add(ccusageCost, hasCcusageCost, hasPeekCost, peekCost)
	sums.wall.Add(ccusageWall, hasCcusageWall, hasPeek, peekWall)
	sums.plan, sums.hasPlan = sessionPlanUsage(record)
	if record.Context != nil {
		sums.contextChars = record.Context.Totals.Chars
		sums.contextItems = record.Context.Totals.Items
		sums.contextSessions = 1
		sums.contextTokens = record.Context.Totals.Tokens
	}
	return sums
}

func (s *usageSums) merge(other *usageSums) {
	s.active += other.active
	s.cacheCreation.Merge(&other.cacheCreation)
	s.cacheRead.Merge(&other.cacheRead)
	s.contextChars += other.contextChars
	s.contextItems += other.contextItems
	s.contextSessions += other.contextSessions
	s.contextTokens += other.contextTokens
	s.cost.Merge(&other.cost)
	s.hasPlan = s.hasPlan || other.hasPlan
	s.idle += other.idle
	s.input.Merge(&other.input)
	s.output.Merge(&other.output)
	s.plan = s.plan.Add(other.plan)
	s.sessions += other.sessions
	s.wall.Merge(&other.wall)
}

// tokens is the pair over all four classes.
func (s *usageSums) tokens() budget.Pair {
	var total budget.Pair
	total.Merge(&s.cacheCreation)
	total.Merge(&s.cacheRead)
	total.Merge(&s.input)
	total.Merge(&s.output)
	return total
}

// verdict is the worst verdict of the compared metrics.
func (s *usageSums) verdict() budget.Agreement {
	verdicts := []budget.Agreement{
		s.cacheCreation.Agreement(budget.TokenTolerance),
		s.cacheRead.Agreement(budget.TokenTolerance),
		s.input.Agreement(budget.TokenTolerance),
		s.output.Agreement(budget.TokenTolerance),
		s.cost.Agreement(budget.CostTolerance),
		s.wall.Agreement(budget.WallTolerance),
	}
	return budget.Worst(verdicts)
}

// planShares weighs a period's Claude sessions against the plan, in percent; weighed counts the sessions with a model split.
type planShares struct {
	claude    int
	fable     float64
	hasWeekly bool
	weekly    float64
	weighed   int
	window    float64
}

type usageTile struct {
	Cell   usageCell
	Detail string
	Label  string
}

type usageView struct {
	inPeriod []*usageSession
	location *time.Location
	period   budget.Period
	plan     budget.Plan
	query    usageQuery
	sessions []*usageSession
	sources  []usageSource
	state    *budget.State
	storeErr string
	total    usageSums
}

// activeDetail names what the active time counts: the runs' wall span under a skill filter.
func (v *usageView) activeDetail() string {
	if v.query.skill != "" {
		return "wall span of the runs · peek"
	}
	return "idle " + formatDuration(v.total.idle) + " · peek"
}

func (v *usageView) cell(format func(*budget.Pair, float64) string, pair *budget.Pair, tolerance budget.Tolerance) usageCell {
	switch v.query.source {
	case usageSourceCcusage:
		return usageCell{Values: []string{format(pair, pair.Ccusage)}}
	case usageSourcePeek:
		return usageCell{Values: []string{format(pair, pair.Peek)}}
	case usageSourceBoth:
	}

	verdict := pair.Agreement(tolerance)
	cell := usageCell{
		Badge:   verdictBadge(verdict),
		Values:  []string{format(pair, pair.Peek), format(pair, pair.Ccusage)},
		Verdict: verdict,
	}
	return cell
}

// chart counts a session on the local day of its last activity.
func (v *usageView) chart() *usageChart {
	unit := v.unit()
	days := v.period.Days()
	values := make(map[string]float64, len(days))
	highest, total := 0.0, 0.0
	for _, session := range v.sessions {
		key := session.record.LastActivity().In(v.location).Format(time.DateOnly)
		value := v.dayValue(session, unit)
		values[key] += value
		highest = max(highest, values[key])
		total += value
	}
	if highest == 0 {
		return v.emptyChart(unit)
	}

	chart := &usageChart{
		Average: chartValue(unit, total/float64(len(days))),
		Days:    make([]usageDay, 0, len(days)),
		Half:    chartValue(unit, highest*usageChartHalf),
		Max:     chartValue(unit, highest),
		Total:   chartValue(unit, total),
		Units:   v.unitChips(unit),
	}
	if unit == usageUnitPlan {
		chart.Plan = v.plan.Label()
	}
	labelStep := (len(days) + usageChartLabelDays - 1) / usageChartLabelDays
	hasValueLabels := len(days) <= usageChartLabelDays
	for index, day := range days {
		value := values[day.Format(time.DateOnly)]
		column := usageDay{
			Height: fmt.Sprintf("%.1f", value/highest*percentScale),
			Title:  day.Format(time.DateOnly) + " — " + chartValue(unit, value),
		}
		if index%labelStep == 0 {
			column.Label = day.Format(usageDayLabelLayout)
		}
		if hasValueLabels && value > 0 {
			column.Value = chartValue(unit, value)
		}
		chart.Days = append(chart.Days, column)
	}
	return chart
}

func (v *usageView) chips(active string, names []string, with func(usageQuery, string) usageQuery) []usageChip {
	names = uniqueSortedNames(names)
	chips := make([]usageChip, 0, len(names)+1)
	all := usageChip{Active: active == "", IsAll: true, Url: with(v.query, "").url("/usage")}
	chips = append(chips, all)
	for _, name := range names {
		chip := usageChip{Active: name == active, Label: name, Url: with(v.query, name).url("/usage")}
		chips = append(chips, chip)
	}
	return chips
}

func (v *usageView) contextKinds() []usageKindRow {
	byKind := make(map[contextrecord.Kind]contextrecord.Totals)
	for _, session := range v.sessions {
		if session.record.Context == nil {
			continue
		}

		addKinds(byKind, session.record.Context.Kinds)
	}

	var rows []usageKindRow
	for _, kind := range contextrecord.Kinds {
		totals, ok := byKind[kind]
		if !ok {
			continue
		}

		row := usageKindRow{
			tokens: totals.Tokens,

			Chars:  groupThousands(int64(totals.Chars)),
			Items:  totals.Items,
			Kind:   kind,
			Tokens: groupThousands(int64(totals.Tokens)),
		}
		rows = append(rows, row)
	}
	slices.SortStableFunc(rows, func(a, b usageKindRow) int { return cmp.Compare(b.tokens, a.tokens) })
	return rows
}

func (v *usageView) csvRow(session *usageSession) []string {
	record, sums := session.record, &session.sums
	return []string{
		record.Id,
		record.Agent,
		record.Title,
		repoName(record),
		strings.Join(skillNames(record), " "),
		record.LastActivity().In(v.location).Format(time.RFC3339),
		rawNumber(sums.input.Ccusage),
		rawNumber(sums.output.Ccusage),
		rawNumber(sums.cacheCreation.Ccusage),
		rawNumber(sums.cacheRead.Ccusage),
		rawCost(sums.cost.Ccusage),
		rawNumber(sums.input.Peek),
		rawNumber(sums.output.Peek),
		rawNumber(sums.cacheCreation.Peek),
		rawNumber(sums.cacheRead.Peek),
		rawCost(sums.cost.Peek),
		rawNumber(sums.wall.Ccusage),
		rawNumber(sums.wall.Peek),
		strconv.Itoa(sums.active),
		strconv.Itoa(sums.idle),
		strconv.Itoa(sums.contextItems),
		strconv.Itoa(sums.contextChars),
		strconv.Itoa(sums.contextTokens),
		v.planPercent(sums),
		string(sums.verdict()),
	}
}

// csvRows is the header and one raw row per session of the view.
func (v *usageView) csvRows() [][]string {
	rows := [][]string{usageCsvHeader}
	for _, session := range v.sessions {
		rows = append(rows, v.csvRow(session))
	}
	return rows
}

func (v *usageView) dayValue(session *usageSession, unit usageUnit) float64 {
	switch unit {
	case usageUnitPlan:
		return session.sums.plan.All / v.plan.WindowCapacity() * percentScale
	case usageUnitUsd:
	}
	return pairValue(&session.sums.cost, v.query.source)
}

// emptyChart keeps the unit chips under the plan unit so the way back to dollars stays on the page.
func (v *usageView) emptyChart(unit usageUnit) *usageChart {
	if unit != usageUnitPlan {
		return nil
	}
	chart := &usageChart{Plan: v.plan.Label(), Units: v.unitChips(unit)}
	return chart
}

// load keeps the period's records, applies the filters and sums the result.
func (v *usageView) load(records []*budget.Record) {
	for _, record := range records {
		if !v.period.Contains(record.LastActivity().In(v.location)) {
			continue
		}

		session := &usageSession{record: record, sums: v.sessionSums(record)}
		v.inPeriod = append(v.inPeriod, session)
		if v.matches(session) {
			v.sessions = append(v.sessions, session)
			v.total.merge(&session.sums)
		}
	}
	slices.SortFunc(v.sessions, func(a, b *usageSession) int {
		return b.record.LastActivity().Compare(a.record.LastActivity())
	})
}

func (v *usageView) matches(session *usageSession) bool {
	record := session.record
	if v.query.agent != "" && record.Agent != v.query.agent {
		return false
	}
	if v.query.repo != "" && repoName(record) != v.query.repo {
		return false
	}
	if v.query.skill != "" && !slices.Contains(skillNames(record), v.query.skill) {
		return false
	}
	isOnlyDiffer := v.query.only == usageOnlyDiffer
	return !isOnlyDiffer || session.sums.verdict() == budget.AgreementDiffer
}

// page hides the by-skill and injected-context breakdowns under a skill filter: neither is a skill's own.
func (v *usageView) page() *usagePage {
	var agents, repos, skills []string
	for _, session := range v.inPeriod {
		agents = append(agents, session.record.Agent)
		repos = append(repos, repoName(session.record))
		skills = append(skills, skillNames(session.record)...)
	}

	var kinds []usageKindRow
	var skillRows []usageSkillRow
	if v.query.skill == "" {
		kinds, skillRows = v.contextKinds(), v.skillRows()
	}
	onlyQuery := v.query
	onlyQuery.only = usageOnlyDiffer
	if v.query.only == usageOnlyDiffer {
		onlyQuery.only = ""
	}
	csvQuery := v.query
	csvQuery.source, csvQuery.unit = usageSourcePeek, usageUnitUsd
	page := &usagePage{
		Agents:         v.chips(v.query.agent, agents, withUsageAgent),
		Chart:          v.chart(),
		ContextCovered: v.total.contextSessions,
		ContextKinds:   kinds,
		CsvUrl:         csvQuery.url("/usage.csv"),
		From:           v.period.From.Format(time.DateOnly),
		HasCcusage:     slices.Contains(v.sources, usageSourceCcusage),
		IsBoth:         v.query.source == usageSourceBoth,
		Only:           usageChip{Active: v.query.only == usageOnlyDiffer, Label: "only disagreements", Url: onlyQuery.url("/usage")},
		Page:           pageUsage,
		Plan:           v.plan.Label(),
		Plans:          planOptions(v.plan),
		Presets:        v.presets(),
		RangeFields:    v.rangeFields(),
		RepoRows:       v.repoRows(),
		Repos:          v.chips(v.query.repo, repos, withUsageRepo),
		Sessions:       v.sessionRows(),
		Skill:          v.query.skill,
		SkillRows:      skillRows,
		Skills:         v.chips(v.query.skill, skills, withUsageSkill),
		Sources:        v.sourceChips(),
		StoreErr:       v.storeErr,
		Tiles:          v.tiles(),
		Title:          labelUsage,
		To:             v.period.LastDay().Format(time.DateOnly),
		Total:          v.total.sessions,
	}
	if v.state != nil {
		page.RunErrors = v.state.Errors()
		page.RunTime = v.state.FinishedAt.In(v.location).Format("2006-01-02 15:04")
	}
	return page
}

func (v *usageView) planDetail() string {
	shares := v.planShares()
	if shares.weighed == 0 {
		return ""
	}

	parts := []string{fmt.Sprintf("%.0f%% of a %s 5-hour window", shares.window, v.plan.Label())}
	if shares.hasWeekly {
		parts = append(parts, fmt.Sprintf("%.0f%% of the weekly limit", shares.weekly))
	}
	if shares.fable > 0 {
		parts = append(parts, fmt.Sprintf("Fable %.0f%% of its weekly half", shares.fable))
	}
	if shares.weighed < shares.claude {
		parts = append(parts, fmt.Sprintf("%d of %d sessions", shares.weighed, shares.claude))
	}
	return strings.Join(parts, " · ")
}

// planShares leaves fable at zero for a plan without a weekly share.
func (v *usageView) planShares() planShares {
	var total budget.PlanUsage
	shares := planShares{}
	for _, session := range v.sessions {
		if session.record.Agent != contextrecord.SessionClaude {
			continue
		}

		shares.claude++
		if !session.sums.hasPlan {
			continue
		}

		shares.weighed++
		total = total.Add(session.sums.plan)
	}

	capacity := v.plan.WindowCapacity()
	weeklyShare, hasWeekly := v.plan.WeeklyShare()
	shares.hasWeekly = hasWeekly
	shares.window = total.All / capacity * percentScale
	shares.weekly = total.All / capacity * weeklyShare * percentScale
	if hasWeekly {
		shares.fable = total.Fable / capacity * weeklyShare / budget.FableWeeklyShare * percentScale
	}
	return shares
}

// planPercent is "" for Codex and for a session without a weighable model split.
func (v *usageView) planPercent(sums *usageSums) string {
	if !sums.hasPlan {
		return ""
	}
	return fmt.Sprintf("%.0f%%", sums.plan.All/v.plan.WindowCapacity()*percentScale)
}

func (v *usageView) presets() []usageChip {
	chips := make([]usageChip, 0, len(budget.PresetDays))
	for _, days := range budget.PresetDays {
		query := v.query
		query.days, query.from, query.to = days, "", ""
		isActive := !v.query.hasRange() && v.query.days == days
		chip := usageChip{Active: isActive, Label: presetLabel(days), Url: query.url("/usage")}
		chips = append(chips, chip)
	}
	return chips
}

func (v *usageView) rangeFields() []usageField {
	values := v.query.filterValues()
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	slices.Sort(names)

	fields := make([]usageField, 0, len(names))
	for _, name := range names {
		field := usageField{Name: name, Value: values.Get(name)}
		fields = append(fields, field)
	}
	return fields
}

func (v *usageView) repoRows() []usageGroupRow {
	byRepo := make(map[string]*usageSums)
	for _, session := range v.sessions {
		name := repoName(session.record)
		sums, ok := byRepo[name]
		if !ok {
			sums = &usageSums{}
			byRepo[name] = sums
		}
		sums.merge(&session.sums)
	}

	names := make([]string, 0, len(byRepo))
	for name := range byRepo {
		names = append(names, name)
	}
	slices.SortFunc(names, func(a, b string) int {
		return cmp.Compare(pairValue(&byRepo[b].cost, v.query.source), pairValue(&byRepo[a].cost, v.query.source))
	})

	rows := make([]usageGroupRow, 0, len(names))
	for _, name := range names {
		sums := byRepo[name]
		tokens := sums.tokens()
		row := usageGroupRow{
			Cost:     v.cell(pairCost, &sums.cost, budget.CostTolerance),
			Name:     name,
			Sessions: sums.sessions,
			Tokens:   v.cell(pairNumber, &tokens, budget.TokenTolerance),
			Url:      withUsageRepo(v.query, name).url("/usage"),
			Wall:     v.cell(pairWall, &sums.wall, budget.WallTolerance),
		}
		rows = append(rows, row)
	}
	return rows
}

func (v *usageView) sessionRows() []usageSessionRow {
	rows := make([]usageSessionRow, 0, len(v.sessions))
	for _, session := range v.sessions {
		record, sums := session.record, &session.sums
		tokens := sums.tokens()
		skills := skillNames(record)
		shownSkills := min(len(skills), usageShownSkills)
		row := usageSessionRow{
			Id:                 record.Id,
			Agent:              record.Agent,
			ContextTokensValue: sums.contextTokens,
			Cost:               v.cell(pairCost, &sums.cost, budget.CostTolerance),
			HasContext:         record.Context != nil,
			LastActive:         record.LastActivity().In(v.location).Format("2006-01-02 15:04"),
			MoreSkills:         strings.Join(skills[shownSkills:], ", "),
			MoreSkillsCount:    len(skills) - shownSkills,
			PlanPercent:        v.planPercent(sums),
			Repo:               repoName(record),
			ShortId:            shortId(record.Id),
			Skills:             strings.Join(skills[:shownSkills], ", "),
			Title:              record.Title,
			Tokens:             v.cell(pairNumber, &tokens, budget.TokenTolerance),
			TokensValue:        int64(pairValue(&tokens, v.query.source)),
			Wall:               v.cell(pairWall, &sums.wall, budget.WallTolerance),
		}
		if v.query.source == usageSourceBoth {
			verdict := sums.verdict()
			row.Badge, row.Verdict = verdictBadge(verdict), verdict
		}
		if row.HasContext {
			row.ContextChars = groupThousands(int64(sums.contextChars))
			row.ContextTokens = groupThousands(int64(sums.contextTokens))
		}
		rows = append(rows, row)
	}
	return rows
}

// sessionSums narrows a session to the selected skill's runs when one is selected.
func (v *usageView) sessionSums(record *budget.Record) usageSums {
	if v.query.skill == "" {
		return newUsageSums(record)
	}
	return newSkillSums(record, v.query.skill)
}

// skillRows adds the uncovered root and subagent usage so the columns add up to peek's total.
func (v *usageView) skillRows() []usageSkillRow {
	bySkill := make(map[string]*usageSkillRow)
	var outside, subagents budget.TokenClasses
	for _, session := range v.sessions {
		side := session.record.Peek
		if side == nil {
			continue
		}

		addSkillRuns(bySkill, side.SkillRuns)
		outside = outside.Add(side.OutsideSkills())
		subagents = subagents.Add(side.Subagents)
	}

	rows := make([]usageSkillRow, 0, len(bySkill)+usageRemainderRows)
	for _, row := range bySkill {
		row.Url = withUsageSkill(v.query, row.Name).url("/usage")
		rows = append(rows, *row)
	}
	slices.SortFunc(rows, func(a, b usageSkillRow) int { return cmp.Compare(b.TokensValue, a.TokensValue) })
	outsideRow := usageSkillRow{CacheReadValue: outside.CacheRead, Name: usageOutsideSkills, TokensValue: freshTokens(outside)}
	subagentsRow := usageSkillRow{CacheReadValue: subagents.CacheRead, Name: usageSubagents, TokensValue: freshTokens(subagents)}
	rows = append(rows, outsideRow, subagentsRow)
	for index := range rows {
		rows[index].CacheRead = groupThousands(int64(rows[index].CacheReadValue))
		rows[index].Seconds = formatDuration(rows[index].seconds)
		rows[index].Tokens = groupThousands(int64(rows[index].TokensValue))
	}
	return rows
}

// sourceChips offers the available sources only and drops the disagreement filter when leaving both; a skill filter has peek's runs only, so no choice.
func (v *usageView) sourceChips() []usageChip {
	if v.query.skill != "" {
		return nil
	}

	chips := make([]usageChip, 0, len(v.sources))
	for _, source := range v.sources {
		query := v.query
		query.source = source
		chip := usageChip{Active: v.query.source == source, Label: string(source), Url: query.forPage().url("/usage")}
		chips = append(chips, chip)
	}
	return chips
}

// tiles leave out the injected context under a skill filter: it is the session's, not the skill's.
func (v *usageView) tiles() []usageTile {
	total := &v.total
	active := usageCell{Values: []string{formatDuration(total.active)}}
	sessions := usageCell{Values: []string{strconv.Itoa(total.sessions)}}
	tiles := []usageTile{
		{Cell: v.cell(pairCost, &total.cost, budget.CostTolerance), Detail: v.planDetail(), Label: "Cost"},
		{Cell: v.cell(pairNumber, &total.input, budget.TokenTolerance), Label: "Input tokens"},
		{Cell: v.cell(pairNumber, &total.output, budget.TokenTolerance), Label: "Output tokens"},
		{Cell: v.cell(pairNumber, &total.cacheCreation, budget.TokenTolerance), Label: "Cache write"},
		{Cell: v.cell(pairNumber, &total.cacheRead, budget.TokenTolerance), Label: "Cache read"},
		{Cell: active, Detail: v.activeDetail(), Label: "Active time"},
		{Cell: sessions, Label: usageSessionsLabel},
	}
	if v.query.skill != "" {
		return tiles
	}

	contextDetail := fmt.Sprintf("%d of %d sessions · %s tokens", total.contextSessions, total.sessions, groupThousands(int64(total.contextTokens)))
	injected := usageCell{Values: []string{groupThousands(int64(total.contextChars)) + " chars"}}
	return append(tiles, usageTile{Cell: injected, Detail: contextDetail, Label: "Injected context"})
}

// unit is the chart's unit; anything but plan percent is dollars.
func (v *usageView) unit() usageUnit {
	if v.query.unit == usageUnitPlan {
		return usageUnitPlan
	}
	return usageUnitUsd
}

// unitChips offers the chart's two units.
func (v *usageView) unitChips(active usageUnit) []usageChip {
	usdQuery, planQuery := v.query, v.query
	usdQuery.unit, planQuery.unit = usageUnitUsd, usageUnitPlan
	usd := usageChip{Active: active == usageUnitUsd, Label: "$", Url: usdQuery.url("/usage")}
	plan := usageChip{Active: active == usageUnitPlan, Label: "plan %", Url: planQuery.url("/usage")}
	return []usageChip{usd, plan}
}

func addKinds(byKind map[contextrecord.Kind]contextrecord.Totals, kinds []budget.ContextKind) {
	for _, kind := range kinds {
		totals := byKind[kind.Kind]
		totals.Chars += kind.Totals.Chars
		totals.Items += kind.Totals.Items
		totals.Tokens += kind.Totals.Tokens
		byKind[kind.Kind] = totals
	}
}

func addSkillRuns(bySkill map[string]*usageSkillRow, runs []budget.SkillRun) {
	for index := range runs {
		run := &runs[index]
		row, ok := bySkill[run.Skill]
		if !ok {
			row = &usageSkillRow{Name: run.Skill}
			bySkill[run.Skill] = row
		}
		row.Runs++
		row.CacheReadValue += run.Usage.CacheRead
		row.seconds += run.Seconds
		row.TokensValue += freshTokens(run.Usage)
	}
}

// chartValue formats a day value in the chart's unit.
func chartValue(unit usageUnit, value float64) string {
	switch unit {
	case usageUnitPlan:
		return fmt.Sprintf("%.0f%%", value)
	case usageUnitUsd:
	}
	return fmt.Sprintf("$%.2f", value)
}

// freshTokens is a usage without its cache reads: what its calls added.
func freshTokens(usage budget.TokenClasses) int {
	return usage.CacheCreation + usage.Input + usage.Output
}

// pairCost shows a dash for a side that contributed nothing.
func pairCost(pair *budget.Pair, value float64) string {
	if value == 0 && pair.Both == 0 {
		return usageDash
	}
	return fmt.Sprintf("$%.2f", value)
}

func pairNumber(pair *budget.Pair, value float64) string {
	if value == 0 && pair.Both == 0 {
		return usageDash
	}
	return groupThousands(int64(value))
}

// planOptions offers every plan, the selected one marked.
func planOptions(selected budget.Plan) []usagePlanOption {
	options := make([]usagePlanOption, 0, len(budget.Plans))
	for _, plan := range budget.Plans {
		option := usagePlanOption{IsSelected: plan == selected, Label: plan.Label(), Value: string(plan)}
		options = append(options, option)
	}
	return options
}

// pairValue prefers peek's side under both sources, else ccusage's.
func pairValue(pair *budget.Pair, source usageSource) float64 {
	switch source {
	case usageSourceCcusage:
		return pair.Ccusage
	case usageSourcePeek:
		return pair.Peek
	case usageSourceBoth:
	}
	if pair.Peek > 0 {
		return pair.Peek
	}
	return pair.Ccusage
}

// pairWall shows a dash for a side without a wall time.
func pairWall(_ *budget.Pair, value float64) string {
	if value <= 0 {
		return usageDash
	}
	return formatDuration(int(value))
}

// presetLabel names a preset period for the Usage chips and the Overview Usage tile.
func presetLabel(days int) string {
	if days == 1 {
		return "today"
	}
	return fmt.Sprintf("%d days", days)
}

func rawCost(value float64) string {
	return strconv.FormatFloat(value, 'f', 4, 64)
}

func rawNumber(value float64) string {
	return strconv.FormatInt(int64(value), 10)
}

func repoName(record *budget.Record) string {
	if record.Repo == "" {
		return usageUnknownRepo
	}
	return record.Repo
}

// sessionPlanUsage is false for Codex and for a session without a weighable model split.
func sessionPlanUsage(record *budget.Record) (budget.PlanUsage, bool) {
	isClaude := record.Agent == contextrecord.SessionClaude
	if !isClaude || record.Ccusage == nil {
		var none budget.PlanUsage
		return none, false
	}
	return record.Ccusage.PlanUsage()
}

func skillNames(record *budget.Record) []string {
	if record.Peek == nil {
		return nil
	}
	var names []string
	for index := range record.Peek.SkillRuns {
		names = append(names, record.Peek.SkillRuns[index].Skill)
	}
	return uniqueSortedNames(names)
}

func uniqueSortedNames(names []string) []string {
	slices.Sort(names)
	return slices.Compact(names)
}

func verdictBadge(verdict budget.Agreement) string {
	switch verdict {
	case budget.AgreementAgree:
		return badgeOk
	case budget.AgreementDiffer:
		return badgeError
	case budget.AgreementSingle:
		return badgeDim
	}
	return badgeDim
}

func withUsageAgent(query usageQuery, agent string) usageQuery {
	query.agent = agent
	return query
}

func withUsageRepo(query usageQuery, repo string) usageQuery {
	query.repo = repo
	return query
}

func withUsageSkill(query usageQuery, skill string) usageQuery {
	query.skill = skill
	return query
}

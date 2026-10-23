package server

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/evals"
	"github.com/kevinhorst/smine/internal/server/respond"
	"github.com/kevinhorst/smine/internal/skills"
)

const tmplSkillTests = "skill_tests.html"

// minReplicasForStats: below it the dispersion and Welch columns are all degenerate.
const minReplicasForStats = 2

// percentScale converts a 0..1 ratio to a percentage for the golden bar.
const percentScale = 100

// msPerSecond converts wall-clock seconds to the ms unit formatMetricValue takes.
const msPerSecond = 1000

const (
	badgeError                 = "badge-error"
	directionHigher            = "higher"
	metricActiveSeconds        = "active_seconds"
	metricCostUsd              = "cost_usd"
	metricEscapedMinutes       = "escaped_minutes"
	metricFalsePositiveMinutes = "false_positive_minutes"
	metricGoldenCoverage       = "golden_coverage"
	metricLostMinutes          = "lost_minutes"
	metricReviewScore          = "review_score"
	metricWallSeconds          = "wall_seconds"
	statusMixed                = "mixed"
	statusFail                 = "fail"
	statusNoVerdict            = "no verdict"
	statusPass                 = "pass"
	unitMinutes                = "min"
	unitRatio                  = "ratio"
	unitSeconds                = "s"
	unitUSD                    = "USD"
)

// allAgentTokensMajor and allAgentTokensMinor are the eval.json version from which output_tokens counts every agent.
const (
	allAgentTokensMajor = 2
	allAgentTokensMinor = 9
)

// Token-ladder provenance ranks, strongest first; worseTokenSource keeps the higher rank.
const (
	tokenSourceRankMain     = 1
	tokenSourceRankAgent    = 2
	tokenSourceRankEstimate = 3
)

// minVariantsForArmLines: a single-variant run needs no per-arm split.
const minVariantsForArmLines = 2

// The notes bounds mirror the eval schema.
const (
	notesBulletChars = 120
	notesBulletLimit = 5
)

// A skill entry id is SKILL-<SKILL>-<TOPIC>-NNN; the topic is its third dash part.
const (
	entryTopicMinParts = 4
	entryTopicPart     = 2
)

// artifactMissing names a run file the catalog does not describe.
const artifactMissing = "not in the artifact catalog"

// artifactCatalog describes the runner's output files by base name, one line per writer in internal/skilleval.
var artifactCatalog = map[string]string{
	"SKILL.md":           "the deployed skill the runs loaded, frozen at init",
	"armruns.jsonl":      "per-arm telemetry rows (retired shell fold)",
	"deltas.json":        "A/B deltas per axis and dimension",
	"eval.json":          "the folded, schema-validated eval record the page reads",
	"eval.rejected.json": "a fold output the schema refused",
	"golden.json":        "byte copy of the definition's golden at init (golden test runs)",
	"interpretation.md":  "the judge's reading of the run, human-curated",
	"overlay.patch":      "the context overlay applied to every arm worktree",
	"provision.json":     "start commit, repo and the per-label worktree paths",
	"reference.patch":    "the reference diff when the definition names a reference commit",
	"report.md":          "the rendered result table, step misses and failures",
	"run.json":           "the frozen run manifest: arms, start commit, golden kind, prompt, judge, token label",
	"run.json.sha256":    "digest the loader checks run.json against",
	"scoring.json":       "the minute price list and thresholds at init (golden test runs)",
	"seed.json":          "the golden seed at init (golden produce runs)",
	"stats.json":         "replica statistics per metric and arm",
}

// severityOrder sorts unknown severities after these, alphabetically.
var severityOrder = []string{"BLOCKER", "MAJOR", "MINOR", "NIT", "INFO"}

// artifactRow is one run file with its catalog description.
type artifactRow struct {
	File string
	What string
}

// definitionView shows the test contract whether or not any run exists yet.
type definitionView struct {
	Budgets       []string
	Recipes       []string
	SchemaVersion string
	Thresholds    []string
}

// goldenDefRow is one golden pointer with its summary (counts, weight, range).
type goldenDefRow struct {
	FindingCount  int
	Id            string
	Input         string
	OracleMinutes float64
	RangeLine     string
	SeverityLine  string
}

// evalConfigView is the run-setup widget under the heading: environment, CLI version and the manifest's config notes, one chip each.
type evalConfigView struct {
	Environment        string   // envCli | envDesktop
	Notes              []string // the notes bullets split at their " · " joints
	Version            string   // the recorded CLI version(s); the installed CLI's when VersionIsInstalled; "—" when neither is known
	VersionIsInstalled bool     // no run recorded a version, so the host's installed CLI stands in
	VersionTitle       string
}

// evalCombinedView averages ratio tiles over valid runs and sums counts, cost and tokens.
type evalCombinedView struct {
	ArmLines             []string // per-variant compact lines, only when >1 variant
	FoundCount           int      // distinct goldens behind FoundLabel
	GoldenTotal          int      // the golden's finding total; 0 without golden data
	CostTotal            string
	EscapedMinutes       string
	FalsePositives       int
	FalsePositiveMinutes string
	FoundLabel           string // "28/218" distinct goldens found of total; "" without golden total
	GoldenCoverage       string // "" when the eval carries no coverage metric
	IsFail               bool
	LostMinutes          string
	MeanPrefix           string      // "mean " over several runs, "" for one
	MinutesBar           *minutesBar // the headline run's minutes; nil without a per-severity breakdown
	NewFinds             int
	PassLine             string
	Reason               string
	ReviewScore          string
	ReviewScoreClass     string
	RunWall              string // the whole run's wall clock (runtimeS, else longest arm)
	RunWallLabel         string
	Runs                 int
	Thresholds           []string
	TokensLabel          string
	TokensTotal          string
	UnionCoverage        string // "" without touched data
}

// evalScoreGroup is one topic of the axis matrix with its followed, partial and missed counts over every run.
type evalScoreGroup struct {
	Neg     int
	NegPct  float64
	Pos     int
	PosPct  float64
	Rows    []evalRuleRow
	Topic   string
	Zero    int
	ZeroPct float64
}

func (g *evalScoreGroup) count(score int) {
	switch {
	case score > 0:
		g.Pos++
	case score < 0:
		g.Neg++
	default:
		g.Zero++
	}
}

// percentages sizes the bar segments over every scored cell of the group.
func (g *evalScoreGroup) percentages() {
	total := g.Pos + g.Zero + g.Neg
	if total == 0 {
		return
	}
	g.PosPct = percentScale * float64(g.Pos) / float64(total)
	g.ZeroPct = percentScale * float64(g.Zero) / float64(total)
	g.NegPct = percentScale * float64(g.Neg) / float64(total)
}

// goldenCardView is one definition golden with its production contributions.
type goldenCardView struct {
	Contributions  []goldenContributionRow
	Def            goldenDefRow
	ProductionLine string
	Version        int
}

// resultRow marks unavailable telemetry with dashes.
type resultRow struct {
	Id string

	ActiveTime       string
	Arm              string
	Cost             string
	FalsePositives   int
	Found            int
	Invalid          bool
	InvalidReason    string
	IsCombined       bool // the synthetic Σ row linking back to the combined hero
	Label            string
	LostMinutes      string
	Misclassified    int
	Missed           int
	MissedPct        float64
	NewFinds         int
	Pass             bool
	ReviewScore      string
	ReviewScoreClass string
	SessionId        string
	Tokens           string
	TokensSource     string // "" measured, "main" main loop only (pre-2.9), "≈" workflow agents, "est." sibling mean
	Total            int
	WallTime         string
}

// runOption is one eval dir of the run selector at the top of the Tests page.
type runOption struct {
	Dir      string
	Label    string
	Selected bool
}

// runOptionGroup is one optgroup of the run selector.
type runOptionGroup struct {
	Label   string
	Options []runOption
}

// minutesBar draws one run's minutes as escaped, false-positive and found segments with oracle and null as ticks on one axis.
type minutesBar struct {
	EscapedLabel       string
	EscapedPct         float64
	FalsePositiveLabel string
	FalsePositivePct   float64
	FoundLabel         string
	FoundPct           float64
	NullLabel          string
	NullPct            float64
	OracleLabel        string
	OraclePct          float64
	RunLabel           string
	TotalLabel         string
}

// severityBreakdown is one run's golden result per severity beside the golden's prices; prices alone when the run carries no breakdown.
type severityBreakdown struct {
	HasRun     bool
	Notes      []string // one line per severity whose minutes are not count × price
	RunLabel   string   // R1…Rn of the run whose counts show
	Rows       []severityRow
	ScoreLine  string
	Totals     severityRow
	TriageLine string
}

// severityRow counts entries by where their minutes landed: Found + Escaped = Entries; the table header carries the unit.
type severityRow struct {
	Entries              int
	Escaped              int // every entry that ships: missed, fix failed, reported below the triage line
	EscapedMinutes       string
	FalsePositiveMinutes string
	FalsePositives       int
	Found                int // found and handled, priced in FoundMinutes
	FoundMinutes         string
	IsTriageLine         bool
	LostMinutes          string
	OracleMinutes        string
	PriceFixed           string
	PriceFound           string
	PriceMissed          string
	Severity             string
}

type evalHistoryRow struct {
	BestArm      string
	Cost         string
	Date         string
	Dir          string
	GoldenLabel  string
	IsFeatured   bool
	LostMinutes  string
	MissedPct    string
	ReviewScore  string
	Status       string
	StatusClass  string
	StatusDetail string
	WallTime     string
}

type evalAxisView struct {
	Axis           string
	Groups         []evalScoreGroup
	Justifications []evalJustificationRow
}

// evalJustificationRow is one non-+1 cell with its reasoning and evidence.
type evalJustificationRow struct {
	Evidence      string
	Justification string
	RuleId        string
	RuleTitle     string
	RunLabel      string
	Score         int
	Source        string
}

// evalRankingRow is one per-axis ranking entry derived from totals.
type evalRankingRow struct {
	Axis     string
	Max      int
	Pct      float64
	Raw      int
	RunLabel string
}

// evalRuleRow cells follow legend order.
type evalRuleRow struct {
	Cells []evalScoreCell
	Rule  evals.RubricRule
}

// evalRunLabel pairs a run with its short display label (R1…Rn).
type evalRunLabel struct {
	Label string
	Run   evals.Run
}

// evalRunVerdictRow is one run's pass/fail line of the verdict card.
type evalRunVerdictRow struct {
	Failed   string
	Pass     bool
	RunLabel string
}

type evalScoreCell struct {
	Class string
	Text  string
	Title string
}

// evalVerdictView is fully composed here: the template prints, never derives.
type evalVerdictView struct {
	BestArm    string
	Checks     []string
	Headline   string
	IsFail     bool
	Reason     string
	Runs       []evalRunVerdictRow
	Status     string
	Thresholds []string
}

// evalView is one eval directory prepared for rendering.
type evalView struct {
	Artifacts             []artifactRow
	Axes                  []evalAxisView
	Combined              *evalCombinedView
	CombinedRow           *resultRow      // Σ row of the Result table; nil without a combined view
	Config                *evalConfigView // the featured view's setup widget; nil on the others
	Dir                   evals.EvalDir
	FocusRunId            string // run featured as the hero instead of the combined view; "" = combined
	InterpretationBullets []string
	NotesBullets          []string
	Rankings              []evalRankingRow
	ResultRows            []resultRow
	Runs                  []evalRunLabel
	Breakdown             *severityBreakdown // the headline run's result per severity; nil without a matching definition golden
	ShowStats             bool
	Skill                 *skills.Skill
	Verdict               *evalVerdictView
}

// goldenContributionRow renders unknown telemetry (the v1 workflow session) as dashes.
type goldenContributionRow struct {
	Cost         string
	Dir          string
	IsLoaded     bool // the dir is an eval dir the page loaded, so the Run cell links it
	NewFindings  int
	Source       string
	Status       string
	StatusClass  string
	Tokens       string
	VersionLabel string
	Wall         string
}

type skillTestsPage struct {
	Definition      *definitionView
	DefinitionError string
	EvalErrors      []string
	Evals           []evalView
	Featured        *evalView
	File            string
	FileContent     string
	Goldens         []goldenCardView
	History         []evalHistoryRow
	Page            string
	RunGroups       []runOptionGroup
	Skill           *skills.Skill
	Smoke           *smokeBadge
	SmokeCard       *smokeCardView
	Tab             string
	Title           string
	Unset           []string
}

func buildAxisViews(file evals.EvalFile, runs []evalRunLabel, cellByRuleAndRun map[string]evals.ScoreCell) []evalAxisView {
	var axes []evalAxisView
	var groupByTopic map[string]int
	for _, rule := range file.Rubric {
		if n := len(axes); n == 0 || axes[n-1].Axis != rule.Axis {
			axes = append(axes, evalAxisView{Axis: rule.Axis})
			groupByTopic = make(map[string]int)
		}
		axis := &axes[len(axes)-1]
		topic := entryTopic(rule.Id)
		groupIndex, seen := groupByTopic[topic]
		if !seen {
			groupIndex = len(axis.Groups)
			groupByTopic[topic] = groupIndex
			axis.Groups = append(axis.Groups, evalScoreGroup{Topic: topic})
		}
		group := &axis.Groups[groupIndex]

		row := evalRuleRow{Rule: rule}
		for _, runLabel := range runs {
			score, ok := cellByRuleAndRun[rule.Id+"\x00"+runLabel.Run.Id]
			row.Cells = append(row.Cells, renderScoreCell(score, ok))
			if ok {
				group.count(score.Score)
			}
			if ok && score.Score != 1 {
				justification := evalJustificationRow{
					Evidence:      strings.Join(score.Evidence, "; "),
					Justification: score.Justification,
					RuleId:        rule.Id,
					RuleTitle:     rule.Rule,
					RunLabel:      runLabel.Label,
					Score:         score.Score,
					Source:        score.Source,
				}
				axis.Justifications = append(axis.Justifications, justification)
			}
		}
		group.Rows = append(group.Rows, row)
	}
	for axisIndex := range axes {
		for groupIndex := range axes[axisIndex].Groups {
			axes[axisIndex].Groups[groupIndex].percentages()
		}
	}
	return axes
}

func buildArtifactRows(files []string) []artifactRow {
	var rows []artifactRow
	for _, file := range files {
		what, isCatalogued := artifactCatalog[filepath.Base(file)]
		if !isCatalogued {
			what = artifactMissing
		}
		row := artifactRow{File: file, What: what}
		rows = append(rows, row)
	}
	return rows
}

func buildEvalView(evalDir evals.EvalDir) evalView {
	view := evalView{Dir: evalDir}

	labelByRunId := make(map[string]string, len(evalDir.Eval.Runs))
	for index, run := range evalDir.Eval.Runs {
		label := fmt.Sprintf("R%d", index+1)
		labelByRunId[run.Id] = label
		runLabel := evalRunLabel{Label: label, Run: run}
		view.Runs = append(view.Runs, runLabel)
	}

	cellByRuleAndRun := make(map[string]evals.ScoreCell, len(evalDir.Eval.Scores))
	for _, score := range evalDir.Eval.Scores {
		cellByRuleAndRun[score.RuleId+"\x00"+score.RunId] = score
	}

	view.Axes = buildAxisViews(evalDir.Eval, view.Runs, cellByRuleAndRun)
	view.Rankings = buildRankings(evalDir.Eval.Totals, labelByRunId)
	view.Verdict = buildVerdictView(evalDir.Eval, labelByRunId)
	view.NotesBullets = clampBullets(evalDir.Eval.Eval.NotesBullets, notesBulletChars, notesBulletLimit)
	var thresholds map[string]float64
	if evalDir.Eval.Verdict != nil {
		thresholds = evalDir.Eval.Verdict.Thresholds
	}
	view.ResultRows = buildResultRows(evalDir, thresholds, labelByRunId)
	view.Combined = buildCombinedView(evalDir, view.Verdict, view.ResultRows)
	if view.Combined != nil {
		combinedRow := combinedResultRow(view.Combined, view.ResultRows)
		view.CombinedRow = &combinedRow
	}
	view.InterpretationBullets = interpretationBullets(evalDir.Interpretation)
	view.Artifacts = buildArtifactRows(evalDir.Files)
	if evalDir.Stats != nil {
		view.ShowStats = maxGroupN(evalDir.Stats) >= minReplicasForStats
	}
	return view
}

// Execution environments of the config widget: a setup block is the retired Desktop fold's, every Go-fold run is claude -p cells.
const (
	envCli     = "cli"
	envDesktop = "desktop"
)

const (
	versionUnknown      = "—"
	versionTitleKnown   = "the Claude Code CLI the arms ran under, from each run's stream init record"
	versionTitleInstall = "no run recorded its CLI version — this is the CLI installed on this host now"
	versionTitleUnknown = "no run recorded its CLI version and claude --version failed on this host"
)

func executionEnvironment(evalDir evals.EvalDir) string {
	if evalDir.Eval.Setup != nil {
		return envDesktop
	}
	return envCli
}

// buildConfigView falls back to installedVersion when no run recorded one; "" means the probe failed too.
func buildConfigView(view *evalView, installedVersion string) *evalConfigView {
	config := &evalConfigView{Environment: executionEnvironment(view.Dir), Notes: configNotes(view.NotesBullets)}
	versions := recordedClaudeVersions(view.Dir.Eval.Runs)
	switch {
	case len(versions) > 0:
		config.Version = strings.Join(versions, ", ")
		config.VersionTitle = versionTitleKnown
	case installedVersion != "":
		config.Version = installedVersion
		config.VersionIsInstalled = true
		config.VersionTitle = versionTitleInstall
	default:
		config.Version = versionUnknown
		config.VersionTitle = versionTitleUnknown
	}
	return config
}

// configNotes splits each bullet at its " · " joints so every setting is one chip.
func configNotes(bullets []string) []string {
	var notes []string
	for _, bullet := range bullets {
		for part := range strings.SplitSeq(bullet, " · ") {
			if part = strings.TrimSpace(part); part != "" {
				notes = append(notes, part)
			}
		}
	}
	return notes
}

// recordedClaudeVersions lists the distinct versions in run order.
func recordedClaudeVersions(runs []evals.Run) []string {
	var versions []string
	for _, run := range runs {
		isNew := run.ClaudeCodeVersion != "" && !slices.Contains(versions, run.ClaudeCodeVersion)
		if isNew {
			versions = append(versions, run.ClaudeCodeVersion)
		}
	}
	return versions
}

// interpretationBullets keeps the file's top-level list items, clamped to the interpretation bounds.
func interpretationBullets(markdown string) []string {
	bullets := evals.InterpretationBullets(markdown)
	return clampBullets(bullets, evals.InterpretationBulletRunes, evals.InterpretationBulletLimit)
}

func maxGroupN(stats *evals.Stats) int {
	most := 0
	for _, group := range stats.Groups {
		if group.N > most {
			most = group.N
		}
	}
	return most
}

// buildResultRows accepts nil thresholds.
func buildResultRows(evalDir evals.EvalDir, thresholds map[string]float64, labelByRunId map[string]string) []resultRow {
	if len(evalDir.Eval.Runs) == 0 {
		return nil
	}

	valueByKey := make(map[string]float64, len(evalDir.Eval.MetricValues))
	for _, value := range evalDir.Eval.MetricValues {
		if number, ok := value.Number(); ok {
			valueByKey[value.MetricId+"\x00"+value.RunId] = number
		}
	}
	goldenByRun := make(map[string]evals.GoldenRun)
	total := 0
	if evalDir.Eval.Golden != nil {
		total = evalDir.Eval.Golden.Total
		for _, perRun := range evalDir.Eval.Golden.PerRun {
			goldenByRun[perRun.RunId] = perRun
		}
	}
	passByRunId := make(map[string]bool)
	if evalDir.Eval.Verdict != nil {
		for _, perRun := range evalDir.Eval.Verdict.PerRun {
			passByRunId[perRun.RunId] = perRun.Pass
		}
	}

	cell := func(runId, metricId, unit string) string {
		if number, ok := valueByKey[metricId+"\x00"+runId]; ok {
			return formatMetricValue(unit, number)
		}
		return "—"
	}

	var rows []resultRow
	for _, run := range evalDir.Eval.Runs {
		row := resultRow{
			Id:            run.Id,
			ActiveTime:    cell(run.Id, metricActiveSeconds, unitSeconds),
			Arm:           run.Variant.Name,
			Cost:          cell(run.Id, metricCostUsd, unitUSD),
			Invalid:       run.Invalid,
			InvalidReason: run.InvalidReason,
			Label:         labelByRunId[run.Id],
			LostMinutes:   cell(run.Id, metricLostMinutes, unitMinutes),
			Pass:          passByRunId[run.Id],
			ReviewScore:   cell(run.Id, metricReviewScore, unitRatio),
			SessionId:     run.Transcript,
			WallTime:      cell(run.Id, metricWallSeconds, unitSeconds),
		}
		if row.Arm == "" {
			row.Arm = run.Id
		}
		row.ReviewScoreClass = reviewScoreClass(valueByKey, run.Id, thresholds)
		if number, source, ok := runTokens(valueByKey, evalDir.ArmRuns, run.Id, row.Arm, evalDir.Eval.SchemaVersion); ok {
			row.Tokens = formatMetricValue("tokens", number)
			row.TokensSource = source
		} else {
			row.Tokens = "—"
		}
		if golden, ok := goldenByRun[run.Id]; ok && total > 0 {
			row.FalsePositives = len(golden.FalsePositives)
			row.Found = len(golden.Matched)
			row.Misclassified = len(golden.Misclassified)
			row.Missed = len(golden.Missed)
			row.MissedPct = percentScale * float64(len(golden.Missed)) / float64(total)
			row.NewFinds = len(golden.Uncataloged)
			row.Total = total
		}
		rows = append(rows, row)
	}
	return rows
}

func isAllAgentTokens(schemaVersion string) bool {
	majorText, minorText, hasDot := strings.Cut(schemaVersion, ".")
	major, majorErr := strconv.Atoi(majorText)
	minor, minorErr := strconv.Atoi(minorText)
	isParsed := hasDot && majorErr == nil && minorErr == nil
	if !isParsed {
		return false
	}

	isLaterMajor := major > allAgentTokensMajor
	isLaterMinor := major == allAgentTokensMajor && minor >= allAgentTokensMinor
	return isLaterMajor || isLaterMinor
}

// runTokens tags output_tokens "main" before eval.json 2.9, when it counted the main loop only.
func runTokens(valueByKey map[string]float64, armRuns []evals.ArmRun, runId, arm, schemaVersion string) (float64, string, bool) {
	if number, ok := valueByKey["output_tokens\x00"+runId]; ok {
		if !isAllAgentTokens(schemaVersion) {
			return number, "main", true
		}
		return number, "", true
	}

	var ownAgentTokens *float64
	siblingSum := 0.0
	siblingCount := 0
	for _, row := range armRuns {
		if row.Id == runId && row.AgentTokens != nil {
			ownAgentTokens = row.AgentTokens
		}
		siblingValue := row.AgentTokens
		if siblingValue == nil {
			siblingValue = row.OutputTokens
		}
		if row.Arm == arm && siblingValue != nil {
			siblingSum += *siblingValue
			siblingCount++
		}
	}
	if ownAgentTokens != nil {
		return *ownAgentTokens, "≈", true
	}
	if siblingCount > 0 {
		return siblingSum / float64(siblingCount), "est.", true
	}
	return 0, "", false
}

func unionGoldenFound(armRuns []evals.ArmRun) (int, bool) {
	if len(armRuns) == 0 {
		return 0, false
	}

	seen := make(map[string]bool)
	for _, row := range armRuns {
		for _, match := range row.Judge.Matched {
			seen[match.GoldenId] = true
		}
	}
	return len(seen), true
}

func buildCombinedView(evalDir evals.EvalDir, verdict *evalVerdictView, rows []resultRow) *evalCombinedView {
	if len(rows) == 0 {
		return nil
	}

	combined := &evalCombinedView{Runs: len(rows)}
	if verdict != nil {
		passed := 0
		for _, run := range verdict.Runs {
			if run.Pass {
				passed++
			}
		}
		combined.IsFail = verdict.IsFail
		combined.PassLine = fmt.Sprintf("%s — %d/%d runs pass", strings.ToUpper(verdict.Status), passed, len(rows))
		combined.Reason = verdict.Reason
		combined.Thresholds = verdict.Thresholds
	} else {
		combined.PassLine = statusNoVerdict
	}

	valueByKey := make(map[string]float64, len(evalDir.Eval.MetricValues))
	for _, value := range evalDir.Eval.MetricValues {
		if number, ok := value.Number(); ok {
			valueByKey[value.MetricId+"\x00"+value.RunId] = number
		}
	}
	reviewSum, lostSum, escapedSum, falsePositiveSum, costSum, wallMax := 0.0, 0.0, 0.0, 0.0, 0.0, 0.0
	reviewCount, lostCount, escapedCount, falsePositiveCount, costCount := 0, 0, 0, 0, 0
	coverageSum, coverageCount := 0.0, 0
	tokensSum := 0.0
	tokensCount := 0
	tokensWorstSource := ""
	for _, row := range rows {
		if row.Invalid {
			continue
		}
		combined.FalsePositives += row.FalsePositives
		combined.NewFinds += row.NewFinds
		if number, ok := valueByKey[metricReviewScore+"\x00"+row.Id]; ok {
			reviewSum += number
			reviewCount++
		}
		if number, ok := valueByKey[metricLostMinutes+"\x00"+row.Id]; ok {
			lostSum += number
			lostCount++
		}
		if number, ok := valueByKey[metricGoldenCoverage+"\x00"+row.Id]; ok {
			coverageSum += number
			coverageCount++
		}
		if number, ok := valueByKey[metricEscapedMinutes+"\x00"+row.Id]; ok {
			escapedSum += number
			escapedCount++
		}
		if number, ok := valueByKey[metricFalsePositiveMinutes+"\x00"+row.Id]; ok {
			falsePositiveSum += number
			falsePositiveCount++
		}
		if number, ok := valueByKey[metricCostUsd+"\x00"+row.Id]; ok {
			costSum += number
			costCount++
		}
		if number, ok := valueByKey[metricWallSeconds+"\x00"+row.Id]; ok {
			wallMax = math.Max(wallMax, number)
		}
		if number, source, ok := runTokens(valueByKey, evalDir.ArmRuns, row.Id, row.Arm, evalDir.Eval.SchemaVersion); ok {
			tokensSum += number
			tokensCount++
			tokensWorstSource = worseTokenSource(tokensWorstSource, source)
		}
	}

	mean := func(sum float64, count int, unit string) string {
		if count == 0 {
			return "—"
		}
		return formatMetricValue(unit, sum/float64(count))
	}
	combined.ReviewScore = mean(reviewSum, reviewCount, unitRatio)
	combined.LostMinutes = mean(lostSum, lostCount, unitMinutes)
	combined.EscapedMinutes = mean(escapedSum, escapedCount, unitMinutes)
	combined.FalsePositiveMinutes = mean(falsePositiveSum, falsePositiveCount, unitMinutes)
	if coverageCount > 0 {
		combined.GoldenCoverage = mean(coverageSum, coverageCount, unitRatio)
	}
	combined.CostTotal = "—"
	if costCount > 0 {
		combined.CostTotal = formatMetricValue(unitUSD, costSum)
	}
	combined.TokensTotal = "—"
	if tokensCount > 0 {
		combined.TokensTotal = formatMetricValue("tokens", tokensSum)
		combined.TokensLabel = strings.TrimSpace(tokensWorstSource + " tokens Σ")
	} else {
		combined.TokensLabel = "tokens Σ"
	}
	// Without the manifest's runtimeS, the longest arm is the honest lower bound.
	combined.RunWall = "—"
	combined.RunWallLabel = "wall"
	if evalDir.Eval.Setup != nil && evalDir.Eval.Setup.RuntimeS != nil {
		combined.RunWall = formatMetricValue("ms", *evalDir.Eval.Setup.RuntimeS*msPerSecond)
	} else if wallMax > 0 {
		combined.RunWall = formatMetricValue(unitSeconds, wallMax)
		combined.RunWallLabel = "wall ≥ (longest arm)"
	}

	var thresholds map[string]float64
	if evalDir.Eval.Verdict != nil {
		thresholds = evalDir.Eval.Verdict.Thresholds
	}
	if reviewCount > 0 {
		combined.ReviewScoreClass = scoreClass(reviewSum/float64(reviewCount), thresholds)
	}
	if len(rows) > 1 {
		combined.MeanPrefix = "mean "
	}

	combinedBar(evalDir, rows, combined)
	combined.ArmLines = combinedArmLines(valueByKey, rows)
	return combined
}

func combinedBar(evalDir evals.EvalDir, rows []resultRow, combined *evalCombinedView) {
	total := 0
	if evalDir.Eval.Golden != nil {
		total = evalDir.Eval.Golden.Total
	}
	if total == 0 {
		return
	}

	if found, ok := unionGoldenFound(evalDir.ArmRuns); ok {
		combined.FoundCount, combined.GoldenTotal = found, total
		combined.FoundLabel = fmt.Sprintf("%d/%d", found, total)
		if touched := evalDir.Eval.Golden.Touched; touched != nil && touched.UnionReachableGoldens >= found {
			combined.UnionCoverage = formatMetricValue(unitRatio, float64(touched.UnionReachableGoldens)/float64(total))
		}
		return
	}

	best := -1
	for index, row := range rows {
		if row.Invalid || row.Total == 0 {
			continue
		}
		if best < 0 || row.Found > rows[best].Found {
			best = index
		}
	}
	if best < 0 {
		return
	}
	combined.FoundCount, combined.GoldenTotal = rows[best].Found, total
	combined.FoundLabel = fmt.Sprintf("%d/%d", rows[best].Found, total)
}

func combinedArmLines(valueByKey map[string]float64, rows []resultRow) []string {
	rowsByArm := make(map[string][]resultRow)
	var armOrder []string
	for _, row := range rows {
		if row.Invalid {
			continue
		}
		if _, seen := rowsByArm[row.Arm]; !seen {
			armOrder = append(armOrder, row.Arm)
		}
		rowsByArm[row.Arm] = append(rowsByArm[row.Arm], row)
	}
	if len(armOrder) < minVariantsForArmLines {
		return nil
	}

	var lines []string
	for _, arm := range armOrder {
		armRows := rowsByArm[arm]
		reviewSum, lostSum := 0.0, 0.0
		reviewCount, lostCount := 0, 0
		for _, row := range armRows {
			if number, ok := valueByKey[metricReviewScore+"\x00"+row.Id]; ok {
				reviewSum += number
				reviewCount++
			}
			if number, ok := valueByKey[metricLostMinutes+"\x00"+row.Id]; ok {
				lostSum += number
				lostCount++
			}
		}
		review := "—"
		if reviewCount > 0 {
			review = formatMetricValue(unitRatio, reviewSum/float64(reviewCount))
		}
		lost := "—"
		if lostCount > 0 {
			lost = formatMetricValue(unitMinutes, lostSum/float64(lostCount))
		}
		lines = append(lines, fmt.Sprintf("%s: review score %s · lost %s (%d runs)", arm, review, lost, len(armRows)))
	}
	return lines
}

// focusCombinedView drops Setup so wall is the run's own duration.
func focusCombinedView(evalDir evals.EvalDir, verdict *evalVerdictView, rows []resultRow, focusRunId string) *evalCombinedView {
	var focusRow *resultRow
	for index := range rows {
		if rows[index].Id == focusRunId {
			focusRow = &rows[index]
			break
		}
	}
	if focusRow == nil {
		return nil
	}

	filtered := evalDir
	filtered.Eval.Setup = nil
	filtered.ArmRuns = nil
	for _, armRun := range evalDir.ArmRuns {
		if armRun.Id == focusRunId {
			filtered.ArmRuns = append(filtered.ArmRuns, armRun)
		}
	}

	combined := buildCombinedView(filtered, verdict, []resultRow{*focusRow})
	if combined == nil {
		return nil
	}
	status := statusPass
	if !focusRow.Pass {
		status = statusFail
	}
	if focusRow.Invalid {
		status = "invalid"
	}
	if verdict == nil {
		status = statusNoVerdict
	}
	combined.PassLine = fmt.Sprintf("%s — %s — arm %s", focusRow.Label, strings.ToUpper(status), focusRow.Arm)
	combined.IsFail = !focusRow.Invalid && verdict != nil && !focusRow.Pass
	return combined
}

func combinedResultRow(combined *evalCombinedView, rows []resultRow) resultRow {
	row := resultRow{
		ActiveTime:       "—",
		Arm:              fmt.Sprintf("%d runs", combined.Runs),
		Cost:             combined.CostTotal,
		FalsePositives:   combined.FalsePositives,
		Found:            combined.FoundCount,
		IsCombined:       true,
		Label:            "Σ combined",
		LostMinutes:      combined.LostMinutes,
		NewFinds:         combined.NewFinds,
		ReviewScore:      combined.ReviewScore,
		ReviewScoreClass: combined.ReviewScoreClass,
		Tokens:           combined.TokensTotal,
		Total:            combined.GoldenTotal,
		WallTime:         combined.RunWall,
	}
	for _, perRun := range rows {
		if !perRun.Invalid {
			row.Misclassified += perRun.Misclassified
		}
	}
	if row.Total > 0 {
		row.Missed = row.Total - row.Found
		row.MissedPct = percentScale * float64(row.Missed) / float64(row.Total)
	}
	return row
}

// dashWhenEmpty renders optional hero strings as table dashes.
func dashWhenEmpty(value string) string {
	if value == "" {
		return "—"
	}
	return value
}

// clampBullets clamps defensively; the producer contracts are the bounds' source.
func clampBullets(bullets []string, chars, limit int) []string {
	var clamped []string
	for _, bullet := range bullets {
		if len(clamped) == limit {
			break
		}
		runes := []rune(bullet)
		if len(runes) > chars {
			bullet = string(runes[:chars-1]) + "…"
		}
		clamped = append(clamped, bullet)
	}
	return clamped
}

// entryTopic is the topic segment of a SKILL-<SKILL>-<TOPIC>-NNN id; "" groups every other id without a header.
func entryTopic(id string) string {
	parts := strings.Split(id, "-")
	isSkillId := len(parts) >= entryTopicMinParts && parts[0] == "SKILL"
	if !isSkillId {
		return ""
	}
	return parts[entryTopicPart]
}

// minutesLabel renders whole minutes with thousands separators; the unit lives in the label or header beside it.
func minutesLabel(minutes float64) string {
	return groupThousands(int64(math.Round(minutes)))
}

// minutesBarFor sums the run's per-severity breakdown; nil for a run the scorer wrote no breakdown for.
func minutesBarFor(golden *evals.Golden, goldenRun *evals.GoldenRun, runLabel string) *minutesBar {
	if golden == nil || goldenRun == nil || len(goldenRun.Severities) == 0 || golden.NullMinutes <= 0 {
		return nil
	}

	escaped, falsePositive, found := 0.0, 0.0, 0.0
	for _, row := range goldenRun.Severities {
		escaped += row.EscapedMinutes
		falsePositive += row.FalsePositiveMinutes
		found += row.FoundMinutes
	}
	total := escaped + falsePositive + found
	axis := math.Max(total, golden.NullMinutes)
	bar := &minutesBar{
		EscapedLabel:       minutesLabel(escaped),
		EscapedPct:         percentScale * escaped / axis,
		FalsePositiveLabel: minutesLabel(falsePositive),
		FalsePositivePct:   percentScale * falsePositive / axis,
		FoundLabel:         minutesLabel(found),
		FoundPct:           percentScale * found / axis,
		NullLabel:          minutesLabel(golden.NullMinutes),
		NullPct:            percentScale * golden.NullMinutes / axis,
		OracleLabel:        minutesLabel(golden.OracleMinutes),
		OraclePct:          percentScale * golden.OracleMinutes / axis,
		RunLabel:           runLabel,
		TotalLabel:         minutesLabel(total),
	}
	return bar
}

// buildSeverityBreakdown joins the golden's prices with the run's per-severity result; prices alone when goldenRun has none.
func buildSeverityBreakdown(golden evals.DefinitionGolden, goldenRun *evals.GoldenRun, runLabel string, scoring *evals.Scoring) *severityBreakdown {
	if scoring == nil || len(scoring.Minutes) == 0 {
		return nil
	}

	runRowBySeverity := make(map[string]evals.GoldenSeverity)
	if goldenRun != nil {
		for _, row := range goldenRun.Severities {
			runRowBySeverity[row.Severity] = row
		}
	}
	names := make([]string, 0, len(scoring.Minutes))
	for severity := range scoring.Minutes {
		names = append(names, severity)
	}
	breakdown := &severityBreakdown{HasRun: len(runRowBySeverity) > 0, RunLabel: runLabel, TriageLine: scoring.TriageLine}
	totals := severityTotals{}
	for _, severity := range sortSeverities(names) {
		minutes := scoring.Minutes[severity]
		multiplier := scoring.EscapeMultiplier[severity]
		factor := scoring.FixFactor[severity]
		runRow := runRowBySeverity[severity]
		entries := golden.SeverityCounts[severity]
		if breakdown.HasRun {
			entries = runRow.Entries
		}
		oracle := float64(entries) * minutes
		lost := runRow.FoundMinutes + runRow.EscapedMinutes + runRow.FalsePositiveMinutes - oracle
		escaped := runRow.Outcomes.Escaped()
		row := severityRow{
			Entries:              entries,
			Escaped:              escaped.Count,
			EscapedMinutes:       minutesLabel(runRow.EscapedMinutes),
			FalsePositiveMinutes: minutesLabel(runRow.FalsePositiveMinutes),
			FalsePositives:       runRow.FalsePositives,
			Found:                runRow.Entries - escaped.Count,
			FoundMinutes:         minutesLabel(runRow.FoundMinutes),
			IsTriageLine:         severity == scoring.TriageLine,
			LostMinutes:          minutesLabel(lost),
			OracleMinutes:        minutesLabel(oracle),
			PriceFixed:           plainNumber(minutes*factor) + " (×" + plainNumber(factor) + ")",
			PriceFound:           plainNumber(minutes),
			PriceMissed:          plainNumber(minutes*multiplier) + " (×" + plainNumber(multiplier) + ")",
			Severity:             severity,
		}
		breakdown.Rows = append(breakdown.Rows, row)
		if note := outcomeNote(severity, &runRow.Outcomes, minutes, minutes*factor); note != "" {
			breakdown.Notes = append(breakdown.Notes, note)
		}
		totals.add(row, runRow, oracle, oracle*multiplier, lost)
	}
	breakdown.Totals = totals.row()
	if breakdown.HasRun {
		breakdown.ScoreLine = totals.scoreLine()
	}
	return breakdown
}

// countsAs renders "1 found entry counts" / "2 found entries count".
func countsAs(count int) string {
	if count == 1 {
		return foundEntries(count) + " counts"
	}
	return foundEntries(count) + " count"
}

// foundEntries renders "1 found entry" / "2 found entries".
func foundEntries(count int) string {
	if count == 1 {
		return "1 found entry"
	}
	return fmt.Sprintf("%d found entries", count)
}

// outcomeNote names every outcome of a severity whose minutes are not count × found price; "" when the row is plain.
func outcomeNote(severity string, outcomes *evals.SeverityOutcomes, price, fixedPrice float64) string {
	var sentences []string
	if tally := outcomes.FixFailed; tally.Count > 0 {
		sentences = append(sentences, fmt.Sprintf("%s as escaped, the machine fix left the defect in the tree (%s min).", countsAs(tally.Count), minutesLabel(tally.Minutes)))
	}
	if tally := outcomes.Dismissed; tally.Count > 0 {
		sentences = append(sentences, fmt.Sprintf("%s as escaped, reported below the triage line (%s min).", countsAs(tally.Count), minutesLabel(tally.Minutes)))
	}
	if tally := outcomes.WrongSeverity; tally.Count > 0 {
		distance := tally.Minutes - float64(tally.Count)*price
		sentences = append(sentences, fmt.Sprintf("%s reported at another severity, +%s min over the price.", foundEntries(tally.Count), minutesLabel(distance)))
	}
	if tally := outcomes.Fixed; tally.Count > 0 {
		sentences = append(sentences, fmt.Sprintf("%s machine-fixed at %s instead of %s (%s min).", foundEntries(tally.Count), plainNumber(fixedPrice), plainNumber(price), minutesLabel(tally.Minutes)))
	}
	if tally := outcomes.FixUnverified; tally.Count > 0 {
		sentences = append(sentences, fmt.Sprintf("%s machine-fixed without confirmation, fix plus full price %s each (%s min).", foundEntries(tally.Count), plainNumber(fixedPrice+price), minutesLabel(tally.Minutes)))
	}
	if len(sentences) == 0 {
		return ""
	}
	return severity + ": " + strings.Join(sentences, " ")
}

// severityTotals sums the breakdown's columns; savable is null − oracle, the score's denominator.
type severityTotals struct {
	entries        int
	escaped        int
	escapedMinutes float64
	falsePositive  float64
	falsePositives int
	found          int
	foundMinutes   float64
	lost           float64
	null           float64
	oracle         float64
}

func (t *severityTotals) add(row severityRow, runRow evals.GoldenSeverity, oracle, null, lost float64) {
	t.entries += row.Entries
	t.escaped += row.Escaped
	t.escapedMinutes += runRow.EscapedMinutes
	t.falsePositive += runRow.FalsePositiveMinutes
	t.falsePositives += runRow.FalsePositives
	t.found += row.Found
	t.foundMinutes += runRow.FoundMinutes
	t.lost += lost
	t.null += null
	t.oracle += oracle
}

func (t *severityTotals) row() severityRow {
	row := severityRow{
		Entries:              t.entries,
		Escaped:              t.escaped,
		EscapedMinutes:       minutesLabel(t.escapedMinutes),
		FalsePositiveMinutes: minutesLabel(t.falsePositive),
		FalsePositives:       t.falsePositives,
		Found:                t.found,
		FoundMinutes:         minutesLabel(t.foundMinutes),
		LostMinutes:          minutesLabel(t.lost),
		OracleMinutes:        minutesLabel(t.oracle),
		Severity:             "Σ",
	}
	return row
}

// scoreLine spells the review score out of the totals: 1 − lost ÷ (null − oracle).
func (t *severityTotals) scoreLine() string {
	savable := t.null - t.oracle
	if savable <= 0 {
		return "review score undefined: null " + minutesLabel(t.null) + " − oracle " + minutesLabel(t.oracle) + " leaves no savable minutes"
	}
	return fmt.Sprintf(
		"review score %s = 1 − lost %s ÷ savable %s (null %s − oracle %s)",
		formatMetricValue(unitRatio, 1-t.lost/savable),
		minutesLabel(t.lost),
		minutesLabel(savable),
		minutesLabel(t.null),
		minutesLabel(t.oracle),
	)
}

// headlineGoldenRun is the focused run's golden row, else the verdict's best arm's, else the first run's.
func headlineGoldenRun(view *evalView) (*evals.GoldenRun, string) {
	golden := view.Dir.Eval.Golden
	if golden == nil {
		return nil, ""
	}
	runId := view.FocusRunId
	if runId == "" {
		runId = historyHeadlineRunId(*view)
	}
	for index := range golden.PerRun {
		if golden.PerRun[index].RunId != runId {
			continue
		}
		for _, runLabel := range view.Runs {
			if runLabel.Run.Id == runId {
				return &golden.PerRun[index], runLabel.Label
			}
		}
		return &golden.PerRun[index], runId
	}
	return nil, ""
}

// sortSeverities orders names by severityOrder, unknown ones after them alphabetically.
func sortSeverities(names []string) []string {
	sorted := slices.Clone(names)
	sort.SliceStable(sorted, func(i, j int) bool {
		rankI, rankJ := slices.Index(severityOrder, sorted[i]), slices.Index(severityOrder, sorted[j])
		if rankI < 0 && rankJ < 0 {
			return sorted[i] < sorted[j]
		}
		if rankI < 0 || rankJ < 0 {
			return rankJ < 0
		}
		return rankI < rankJ
	})
	return sorted
}

// plainNumber renders a price without trailing zeros.
func plainNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func reviewScoreClass(valueByKey map[string]float64, runId string, thresholds map[string]float64) string {
	score, ok := valueByKey[metricReviewScore+"\x00"+runId]
	if !ok {
		return ""
	}
	return scoreClass(score, thresholds)
}

func scoreClass(score float64, thresholds map[string]float64) string {
	bound, hasBound := thresholds[metricReviewScore]
	if hasBound && score < bound {
		return "tile-fail"
	}
	return "tile-pass"
}

// worseTokenSource ranks measured ("") over workflow-agent ("≈") over sibling-mean ("est.").
func worseTokenSource(current, candidate string) string {
	rank := func(source string) int {
		switch source {
		case "est.":
			return tokenSourceRankEstimate
		case "≈":
			return tokenSourceRankAgent
		case "main":
			return tokenSourceRankMain
		}
		return 0
	}
	if rank(candidate) > rank(current) {
		return candidate
	}
	return current
}

// runSelectorOptions lists the eval runs first and the golden-production runs in their own group.
func runSelectorOptions(views []evalView, featuredDir string, productionDirs map[string]bool) []runOptionGroup {
	evalRuns := runOptionGroup{Label: "eval runs"}
	production := runOptionGroup{Label: "golden production"}
	for _, view := range views {
		status := statusNoVerdict
		if view.Verdict != nil {
			status = view.Verdict.Status
		}
		option := runOption{
			Dir:      view.Dir.Dir,
			Label:    fmt.Sprintf("%s — %s — %s", view.Dir.Eval.Eval.Date, view.Dir.Dir, status),
			Selected: view.Dir.Dir == featuredDir,
		}
		if productionDirs[view.Dir.Dir] {
			production.Options = append(production.Options, option)
			continue
		}
		evalRuns.Options = append(evalRuns.Options, option)
	}

	var groups []runOptionGroup
	if len(evalRuns.Options) > 0 {
		groups = append(groups, evalRuns)
	}
	if len(production.Options) > 0 {
		groups = append(groups, production)
	}
	return groups
}

func buildHistoryRows(views []evalView, featuredDir string, productionDirs map[string]bool) []evalHistoryRow {
	var rows []evalHistoryRow
	for _, view := range views {
		if productionDirs[view.Dir.Dir] {
			continue
		}
		row := evalHistoryRow{
			Date:       view.Dir.Eval.Eval.Date,
			Dir:        view.Dir.Dir,
			IsFeatured: view.Dir.Dir == featuredDir,
		}
		resolvedStatus := historyStatus(view)
		row.Status, row.StatusClass, row.StatusDetail, row.BestArm = resolvedStatus.Status, resolvedStatus.Class, resolvedStatus.Detail, resolvedStatus.BestArm
		row.GoldenLabel = historyGoldenLabel(view.Dir.Eval.Golden)

		runId := historyHeadlineRunId(view)
		if runId != "" {
			valueByKey := make(map[string]float64, len(view.Dir.Eval.MetricValues))
			for _, value := range view.Dir.Eval.MetricValues {
				if number, ok := value.Number(); ok {
					valueByKey[value.MetricId+"\x00"+value.RunId] = number
				}
			}
			if number, ok := valueByKey[metricReviewScore+"\x00"+runId]; ok {
				row.ReviewScore = formatMetricValue(unitRatio, number)
			}
			if number, ok := valueByKey[metricLostMinutes+"\x00"+runId]; ok {
				row.LostMinutes = formatMetricValue(unitMinutes, number)
			}
			if number, ok := valueByKey["cost_usd\x00"+runId]; ok {
				row.Cost = formatMetricValue(unitUSD, number)
			}
			if number, ok := valueByKey[metricWallSeconds+"\x00"+runId]; ok {
				row.WallTime = formatMetricValue(unitSeconds, number)
			}
			row.MissedPct = historyMissedPct(view, runId)
		}
		rows = append(rows, row)
	}
	return rows
}

// historyGoldenLabel shows the finding count: the denominator that makes recalls comparable across rows.
func historyGoldenLabel(golden *evals.Golden) string {
	if golden == nil {
		return "—"
	}
	if golden.GoldenVersion == 0 {
		return fmt.Sprintf("— · %d", golden.Total)
	}
	return fmt.Sprintf("v%d · %d", golden.GoldenVersion, golden.Total)
}

type historyStatusView struct {
	BestArm string
	Class   string
	Detail  string
	Status  string
}

// historyStatus reads ERROR when every run is invalid.
func historyStatus(view evalView) historyStatusView {
	if view.Verdict == nil {
		noVerdict := historyStatusView{Class: badgeDim, Status: statusNoVerdict}
		return noVerdict
	}
	if reason := allInvalidReason(view); reason != "" {
		invalid := historyStatusView{BestArm: view.Verdict.BestArm, Class: badgeDim, Detail: reason, Status: "ERROR"}
		return invalid
	}
	class := badgeOk
	switch {
	case view.Verdict.IsFail:
		class = badgeError
	case view.Verdict.Status == statusMixed:
		class = badgeAction
	}
	status := strings.ToUpper(view.Verdict.Status)
	if status == "" {
		status = "—"
	}
	resolved := historyStatusView{BestArm: view.Verdict.BestArm, Class: class, Status: status}
	return resolved
}

func allInvalidReason(view evalView) string {
	if len(view.Runs) == 0 {
		return ""
	}
	reason := ""
	for _, runLabel := range view.Runs {
		if !runLabel.Run.Invalid {
			return ""
		}
		if reason == "" {
			reason = runLabel.Run.InvalidReason
		}
	}
	if reason == "" {
		reason = "invalid"
	}
	return reason
}

func historyHeadlineRunId(view evalView) string {
	if view.Verdict != nil && view.Verdict.BestArm != "" {
		for _, runLabel := range view.Runs {
			if runLabel.Run.Variant.Name == view.Verdict.BestArm {
				return runLabel.Run.Id
			}
		}
	}
	if len(view.Runs) > 0 {
		return view.Runs[0].Run.Id
	}
	return ""
}

// historyMissedPct is the golden miss rate of one run, formatted as a percent.
func historyMissedPct(view evalView, runId string) string {
	if view.Dir.Eval.Golden == nil || view.Dir.Eval.Golden.Total == 0 {
		return "—"
	}
	for _, perRun := range view.Dir.Eval.Golden.PerRun {
		if perRun.RunId == runId {
			pct := percentScale * float64(len(perRun.Missed)) / float64(view.Dir.Eval.Golden.Total)
			return fmt.Sprintf("%.0f%%", pct)
		}
	}
	return "—"
}

func buildEvalViews(evalDirs []evals.EvalDir, skill *skills.Skill) []evalView {
	var views []evalView
	for _, evalDir := range evalDirs {
		view := buildEvalView(evalDir)
		view.Skill = skill
		views = append(views, view)
	}
	return views
}

func buildDefinitionView(def *evals.Definition) *definitionView {
	if def == nil {
		return nil
	}

	metricById := make(map[string]evals.Metric, len(evals.GoldenMetrics))
	for _, metric := range evals.GoldenMetrics {
		metricById[metric.Id] = metric
	}

	scoring := def.Scoring
	if scoring == nil {
		scoring = &evals.Scoring{}
	}
	thresholdIds := make([]string, 0, len(scoring.Thresholds))
	for metricId := range scoring.Thresholds {
		thresholdIds = append(thresholdIds, metricId)
	}
	sort.Strings(thresholdIds)
	var thresholds []string
	for _, metricId := range thresholdIds {
		thresholds = append(thresholds, formatThreshold(metricById[metricId], metricId, scoring.Thresholds[metricId]))
	}

	view := &definitionView{
		Budgets:       definitionBudgets(def),
		Recipes:       []string{"/skillroutine-eval " + def.Skill},
		SchemaVersion: def.SchemaVersion,
		Thresholds:    thresholds,
	}
	return view
}

// buildGoldenCards renders one card per definition golden; a contribution's run links only when its dir is loaded.
func buildGoldenCards(def *evals.Definition, views []evalView) []goldenCardView {
	if def == nil {
		return nil
	}
	statusByDir := make(map[string]historyStatusView, len(views))
	for _, view := range views {
		statusByDir[view.Dir.Dir] = historyStatus(view)
	}

	var cards []goldenCardView
	for _, golden := range def.Goldens {
		card := goldenCardView{
			Contributions:  buildGoldenContributionRows(golden.Production, statusByDir),
			Def:            buildGoldenDefRow(golden),
			ProductionLine: goldenProductionLine(golden),
			Version:        golden.GoldenVersion,
		}
		if card.Version == 0 && golden.Production != nil {
			card.Version = lastContributionVersion(golden.Production)
		}
		cards = append(cards, card)
	}
	return cards
}

// definitionBudgets formats the preset arm's spend/time ceiling and window.
func definitionBudgets(def *evals.Definition) []string {
	lines := []string{
		fmt.Sprintf("arm: $%s / %s", strconv.FormatFloat(def.Budgets.ArmUsd, 'f', -1, 64), formatDuration(def.Budgets.ArmTimeoutS)),
		"window: $" + strconv.FormatFloat(def.Budgets.WindowUsd, 'f', -1, 64),
	}
	return lines
}

// buildGoldenDefRow collapses a skill-owned golden without the optional shape to a count-only row.
func buildGoldenDefRow(golden evals.DefinitionGolden) goldenDefRow {
	row := goldenDefRow{
		FindingCount:  golden.FindingCount,
		Id:            golden.Id,
		Input:         golden.Input,
		OracleMinutes: golden.OracleMinutes,
		SeverityLine:  severityLine(golden.SeverityCounts),
	}
	if golden.BaseLabel != "" || golden.HeadLabel != "" {
		row.RangeLine = golden.BaseLabel + " → " + golden.HeadLabel
	}
	return row
}

func buildGoldenContributionRows(production *evals.GoldenProduction, statusByDir map[string]historyStatusView) []goldenContributionRow {
	if production == nil {
		return nil
	}

	var rows []goldenContributionRow
	for _, contribution := range production.Contributions {
		status, isLoaded := statusByDir[contribution.Dir]
		row := goldenContributionRow{
			Cost:         formatOptional(unitUSD, contribution.CostUsd),
			Dir:          contribution.Dir,
			IsLoaded:     isLoaded,
			NewFindings:  contribution.NewFindings,
			Source:       contribution.Source,
			Status:       status.Status,
			StatusClass:  status.Class,
			Tokens:       contributionTokens(contribution),
			VersionLabel: fmt.Sprintf("v%d (%d)", contribution.Version, contribution.Findings),
			Wall:         formatOptionalWall(contribution.WallS),
		}
		rows = append(rows, row)
	}
	return rows
}

// contributionTokens uses subagent tokens for the interactive v1 session.
func contributionTokens(contribution evals.GoldenContribution) string {
	if contribution.OutputTokens != nil {
		return groupThousands(*contribution.OutputTokens)
	}
	if contribution.SubagentTokens != nil {
		return groupThousands(*contribution.SubagentTokens) + " (subagent)"
	}
	return "—"
}

// formatOptional renders a nullable metric value in its unit, dash when null.
func formatOptional(unit string, value *float64) string {
	if value == nil {
		return "—"
	}
	return formatMetricValue(unit, *value)
}

// formatOptionalWall renders a nullable wall-clock second count, dash when null.
func formatOptionalWall(seconds *int64) string {
	if seconds == nil {
		return "—"
	}
	return formatMetricValue("ms", float64(*seconds)*msPerSecond)
}

// goldenProductionLine names unknown portions rather than guessing them.
func goldenProductionLine(golden evals.DefinitionGolden) string {
	production := golden.Production
	if production == nil {
		return ""
	}

	parts := []string{fmt.Sprintf("Golden v%d built from %d contributions", lastContributionVersion(production), len(production.Contributions))}
	if production.Totals.CostUsd != nil {
		cost := formatMetricValue(unitUSD, *production.Totals.CostUsd)
		if hasUncostedContribution(production) {
			cost += " + the v1 workflow session"
		}
		parts = append(parts, cost)
	}
	if production.Totals.WallS != nil {
		parts = append(parts, formatOptionalWall(production.Totals.WallS)+" wall")
	}
	if production.Totals.OutputTokens != nil {
		parts = append(parts, groupThousands(*production.Totals.OutputTokens)+" output tok")
	}
	if production.Totals.SubagentTokens != nil {
		parts = append(parts, groupThousands(*production.Totals.SubagentTokens)+" subagent tok (v1)")
	}
	return strings.Join(parts, " · ")
}

func hasUncostedContribution(production *evals.GoldenProduction) bool {
	for _, contribution := range production.Contributions {
		if contribution.CostUsd == nil {
			return true
		}
	}
	return false
}

// lastContributionVersion is the newest version in the production record.
func lastContributionVersion(production *evals.GoldenProduction) int {
	version := 0
	for _, contribution := range production.Contributions {
		if contribution.Version > version {
			version = contribution.Version
		}
	}
	return version
}

// formatDuration renders a second count as a compact h/m/s string.
func formatDuration(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	if minutes%60 == 0 {
		return fmt.Sprintf("%dh", minutes/60)
	}
	return fmt.Sprintf("%dh%dm", minutes/60, minutes%60)
}

func severityLine(counts map[string]int) string {
	if len(counts) == 0 {
		return ""
	}

	var parts []string
	seen := make(map[string]bool, len(counts))
	for _, severity := range severityOrder {
		if count, ok := counts[severity]; ok {
			parts = append(parts, fmt.Sprintf("%d %s", count, severity))
			seen[severity] = true
		}
	}

	var extra []string
	for severity, count := range counts {
		if !seen[severity] {
			extra = append(extra, fmt.Sprintf("%d %s", count, severity))
		}
	}
	sort.Strings(extra)
	parts = append(parts, extra...)
	return strings.Join(parts, ", ")
}

func buildVerdictView(file evals.EvalFile, labelByRunId map[string]string) *evalVerdictView {
	verdict := file.Verdict
	if verdict == nil {
		return nil
	}

	metricById := make(map[string]evals.Metric, len(file.Metrics))
	for _, metric := range file.Metrics {
		metricById[metric.Id] = metric
	}

	passed := 0
	var runs []evalRunVerdictRow
	for _, perRun := range verdict.PerRun {
		if perRun.Pass {
			passed++
		}
		var failed []string
		for _, metricId := range perRun.FailedThresholds {
			failed = append(failed, metricLabelById(metricById, metricId))
		}
		row := evalRunVerdictRow{
			Failed:   strings.Join(failed, ", "),
			Pass:     perRun.Pass,
			RunLabel: labelByRunId[perRun.RunId],
		}
		runs = append(runs, row)
	}

	thresholdIds := make([]string, 0, len(verdict.Thresholds))
	for metricId := range verdict.Thresholds {
		thresholdIds = append(thresholdIds, metricId)
	}
	sort.Strings(thresholdIds)
	var thresholds []string
	for _, metricId := range thresholdIds {
		thresholds = append(thresholds, formatThreshold(metricById[metricId], metricId, verdict.Thresholds[metricId]))
	}

	view := &evalVerdictView{
		BestArm:    verdict.BestArm,
		Headline:   verdictHeadline(verdict.Status, passed, len(verdict.PerRun)),
		IsFail:     verdict.Status == "fail",
		Reason:     verdict.Reason,
		Runs:       runs,
		Status:     verdict.Status,
		Thresholds: thresholds,
	}
	if bestRunId := bestArmRunId(file, verdict.BestArm); bestRunId != "" {
		view.Checks = verdictChecks(file, verdict, bestRunId)
	}
	return view
}

func bestArmRunId(file evals.EvalFile, bestArm string) string {
	if bestArm == "" {
		return ""
	}
	for _, run := range file.Runs {
		if run.Variant.Name == bestArm {
			return run.Id
		}
	}
	return ""
}

func verdictChecks(file evals.EvalFile, verdict *evals.Verdict, bestRunId string) []string {
	valueByKey := make(map[string]float64, len(file.MetricValues))
	for _, value := range file.MetricValues {
		if number, ok := value.Number(); ok {
			valueByKey[value.MetricId+"\x00"+value.RunId] = number
		}
	}
	metricById := make(map[string]evals.Metric, len(file.Metrics))
	for _, metric := range file.Metrics {
		metricById[metric.Id] = metric
	}

	metricIds := make([]string, 0, len(verdict.Thresholds))
	for metricId := range verdict.Thresholds {
		metricIds = append(metricIds, metricId)
	}
	sort.Strings(metricIds)

	var checks []string
	for _, metricId := range metricIds {
		bound := verdict.Thresholds[metricId]
		metric := metricById[metricId]
		value, ok := valueByKey[metricId+"\x00"+bestRunId]
		if !ok {
			checks = append(checks, fmt.Sprintf("%s — (%s)", metricLabelById(metricById, metricId), formatThreshold(metric, metricId, bound)))
			continue
		}
		mark := "✓"
		if (metric.Direction == directionHigher && value < bound) || (metric.Direction != directionHigher && value > bound) {
			mark = "✗"
		}
		check := fmt.Sprintf("%s %s %s (%s)", metricLabelById(metricById, metricId), formatMetricValue(metric.Unit, value), mark, formatThreshold(metric, metricId, bound))
		checks = append(checks, check)
	}
	return checks
}

func formatMetricValue(unit string, value float64) string {
	switch unit {
	case unitRatio:
		percent := math.Round(value * percentScale)
		if percent == 0 {
			return "0%"
		}
		return fmt.Sprintf("%.0f%%", percent)
	case unitMinutes:
		return minutesLabel(value)
	case unitUSD:
		return fmt.Sprintf("$%.2f", value)
	case unitSeconds:
		return formatMetricValue("ms", value*msPerSecond)
	case "ms":
		seconds := value / 1000
		if seconds < 120 {
			return fmt.Sprintf("%.0f s", seconds)
		}
		return fmt.Sprintf("%.1f min", seconds/60)
	case "tokens", "count":
		if value == math.Trunc(value) {
			return groupThousands(int64(value))
		}
		return fmt.Sprintf("%.2f", value)
	default:
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
}

// formatThreshold renders "False positives ≤ 5"; an unknown metric falls back to its id.
func formatThreshold(metric evals.Metric, metricId string, bound float64) string {
	label := metricId
	unit := ""
	operator := "≤"
	if metric.Id != "" {
		label = metric.Label
		unit = metric.Unit
	}
	if metric.Direction == directionHigher {
		operator = "≥"
	}
	return fmt.Sprintf("%s %s %s", label, operator, formatMetricValue(unit, bound))
}

// groupThousands renders an integer with space thousands separators.
func groupThousands(number int64) string {
	digits := strconv.FormatInt(number, 10)
	var builder strings.Builder
	for index, digit := range digits {
		remaining := len(digits) - index
		if index > 0 && remaining%3 == 0 && digit != '-' {
			builder.WriteString(" ")
		}
		builder.WriteRune(digit)
	}
	return builder.String()
}

func metricLabel(metric evals.Metric) string {
	switch metric.Direction {
	case directionHigher:
		return metric.Label + " ↑"
	case "lower":
		return metric.Label + " ↓"
	default:
		return metric.Label
	}
}

// metricLabelById resolves a metric id to its label, falling back to the id.
func metricLabelById(metricById map[string]evals.Metric, metricId string) string {
	if metric, ok := metricById[metricId]; ok {
		return metric.Label
	}
	return metricId
}

func buildRankings(totals []evals.Total, labelByRunId map[string]string) []evalRankingRow {
	var rankings []evalRankingRow
	for _, total := range totals {
		ranking := evalRankingRow{
			Axis:     total.Axis,
			Max:      total.Max,
			Pct:      total.Pct,
			Raw:      total.Raw,
			RunLabel: labelByRunId[total.RunId],
		}
		rankings = append(rankings, ranking)
	}
	sort.SliceStable(rankings, func(i, j int) bool {
		if rankings[i].Axis != rankings[j].Axis {
			return rankings[i].Axis < rankings[j].Axis
		}
		return rankings[i].Pct > rankings[j].Pct
	})
	return rankings
}

// verdictHeadline renders an unknown status raw rather than dropping it.
func verdictHeadline(status string, passed, total int) string {
	switch status {
	case "pass":
		return "Pass — every run reached the thresholds"
	case "fail":
		return "Fail — no run reached the thresholds"
	case statusMixed:
		return fmt.Sprintf("Mixed — %d of %d runs reached the thresholds", passed, total)
	case "no-golden":
		return "No golden — nothing to judge against"
	default:
		return "Verdict: " + status
	}
}

func renderScoreCell(score evals.ScoreCell, ok bool) evalScoreCell {
	if !ok {
		absent := evalScoreCell{Class: "score-absent", Text: "—", Title: "no cell — rule not in this run's variant"}
		return absent
	}

	cell := evalScoreCell{Title: score.Justification}
	switch score.Score {
	case 1:
		cell.Class, cell.Text = "score-pos", "✓"
	case 0:
		cell.Class, cell.Text = "score-zero", "◐"
	case -1:
		cell.Class, cell.Text = "score-neg", "✗"
	default:
		cell.Class, cell.Text = "score-zero", fmt.Sprintf("%d", score.Score)
	}
	return cell
}

func (s *Server) handleSkillTests(w http.ResponseWriter, r *http.Request) {
	skill := s.findSkill(w, r)
	if skill == nil {
		return
	}

	data := s.skillTestsData(r.Context(), r.URL.Query().Get("focus"), r.URL.Query().Get("run"), skill)
	data.Title = "Skill — " + skill.Name + " — Tests"
	s.renderFragment(w, tmplSkillTests, data)
}

func (s *Server) handleSkillTestsFile(w http.ResponseWriter, r *http.Request) {
	skill := s.findSkill(w, r)
	if skill == nil {
		return
	}

	// User input is never joined into a path; both parameters must match loader-enumerated values.
	requestedDir := r.URL.Query().Get("d")
	requestedFile := r.URL.Query().Get("f")
	evalDirs, _ := evals.LoadForSkill(s.evalsDir, skill.Name)
	dirIndex := slices.IndexFunc(evalDirs, func(evalDir evals.EvalDir) bool {
		return evalDir.Dir == requestedDir
	})
	if dirIndex < 0 || !slices.Contains(evalDirs[dirIndex].Files, requestedFile) {
		http.NotFound(w, r)
		return
	}

	content, err := os.ReadFile(filepath.Join(s.evalsDir, requestedDir, requestedFile))
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	data := skillTestsPage{
		File:        filepath.Join(requestedDir, requestedFile),
		FileContent: string(content),
		Page:        pageSkills,
		Skill:       skill,
		Smoke:       s.lastSmoke(skill),
		Tab:         "tests",
		Title:       "Skill — " + skill.Name + " — " + requestedFile,
		Unset:       s.unsetVariables(r.Context(), skill),
	}
	s.renderFragment(w, tmplSkillTests, data)
}

// skillTestsData validates requestedRun against the loaded dirs before featuring it.
func (s *Server) skillTestsData(ctx context.Context, focusRunId, requestedRun string, skill *skills.Skill) skillTestsPage {
	data := skillTestsPage{Page: pageSkills, Skill: skill, Smoke: s.lastSmoke(skill), SmokeCard: s.smokeCard(skill), Tab: "tests", Unset: s.unsetVariables(ctx, skill)}

	evalDirs, evalErrors := evals.LoadForSkill(s.evalsDir, skill.Name)
	data.EvalErrors = evalErrors
	data.Evals = buildEvalViews(evalDirs, skill)
	definition, err := evals.LoadDefinition(skill.Path)
	if err != nil {
		data.DefinitionError = err.Error()
	}
	data.Definition = buildDefinitionView(definition)
	data.Goldens = buildGoldenCards(definition, data.Evals)
	production := productionDirs(definition)

	if len(data.Evals) > 0 {
		featured := &data.Evals[0]
		for index := range data.Evals {
			if data.Evals[index].Dir.Dir == requestedRun {
				featured = &data.Evals[index]
				break
			}
		}
		if focusRunId != "" {
			if focused := focusCombinedView(featured.Dir, featured.Verdict, featured.ResultRows, focusRunId); focused != nil {
				featured.Combined = focused
				featured.FocusRunId = focusRunId
			}
		}
		data.Featured = featured
		installedVersion := ""
		if len(recordedClaudeVersions(featured.Dir.Eval.Runs)) == 0 {
			installedVersion = s.claude.version(ctx)
		}
		featured.Config = buildConfigView(featured, installedVersion)
		data.History = buildHistoryRows(data.Evals, featured.Dir.Dir, production)
		data.RunGroups = runSelectorOptions(data.Evals, featured.Dir.Dir, production)
		goldenRun, runLabel := headlineGoldenRun(featured)
		if featured.Combined != nil {
			featured.Combined.MinutesBar = minutesBarFor(featured.Dir.Eval.Golden, goldenRun, runLabel)
		}
		featured.Breakdown = featuredBreakdown(definition, featured.Dir.Eval.Golden, goldenRun, runLabel)
	}
	return data
}

// featuredBreakdown joins the golden the featured run was scored against with the headline run's result; nil without one.
func featuredBreakdown(def *evals.Definition, golden *evals.Golden, goldenRun *evals.GoldenRun, runLabel string) *severityBreakdown {
	if def == nil || golden == nil {
		return nil
	}
	for _, candidate := range def.Goldens {
		if candidate.Id == golden.GoldenId {
			return buildSeverityBreakdown(candidate, goldenRun, runLabel, def.Scoring)
		}
	}
	return nil
}

// productionDirs collects every run dir a definition golden's contributions name.
func productionDirs(def *evals.Definition) map[string]bool {
	dirs := make(map[string]bool)
	if def == nil {
		return dirs
	}
	for _, golden := range def.Goldens {
		if golden.Production == nil {
			continue
		}
		for _, contribution := range golden.Production.Contributions {
			if contribution.Dir != "" {
				dirs[contribution.Dir] = true
			}
		}
	}
	return dirs
}

package skilleval

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/kevinhorst/smine/internal/evals"
	"github.com/kevinhorst/smine/internal/peek"
)

const (
	adherenceFile    = "adherence.json"
	compareFile      = "compare.json"
	evalFileName     = "eval.json"
	rejectedFileName = "eval.rejected.json"
	noEnvelopeNote   = "the arm stream carries no result envelope"
	noStreamWallNote = "the arm stream carries no timestamped record"
	reportFile       = "report.md"
	schemaVersion    = "2.13"
	scorePrefix      = "score_"

	judgeMetricCount = 4
	peekCallTimeout  = 30 * time.Second

	verdictFollowed      = "followed"
	verdictNotApplicable = "not-applicable"
	verdictNotObservable = "not-observable"
	verdictPartial       = "partial"
	verdictSkipped       = "skipped"
	verdictViolated      = "violated"

	severityBlocker = "BLOCKER"
	severityMajor   = "MAJOR"

	subtypeSuccess    = "success"
	terminalCompleted = "completed"

	axisSelf        = "self"
	directionHigher = "higher"
	directionLower  = "lower"
	maxNoteRunes    = 120
	scoreSource     = "agent"
	sourceHeartbeat = "heartbeat"
	sourceJudge     = "judge"
	sourceProbe     = "probe"
	sourceScorer    = "scorer"
	sourceTelemetry = "telemetry"
	statusFail      = "fail"
	statusMixed     = "mixed"
	statusPass      = "pass"

	metricActiveSeconds     = "active_seconds"
	metricCheckFailures     = "check_failures"
	metricCheckPass         = "check_pass"
	metricCostUsd           = "cost_usd"
	metricDefectsMajor      = "defects_major"
	metricOutputTokens      = "output_tokens"
	metricPermissionDenials = "permission_denials"
	metricQualityRank       = "quality_rank"
	metricRunsKilled        = "runs_killed"
	metricSmokePass         = "smoke_pass"
	metricStepMissPct       = "step_miss_pct"
	metricStepUnobserved    = "step_unobserved"
	metricWallSeconds       = "wall_seconds"
)

// AdherenceEntry is one entry verdict of an adherence judge's adherence.json.
type AdherenceEntry struct {
	Id       string `json:"id"`
	Evidence string `json:"evidence"`
	Note     string `json:"note"`
	Title    string `json:"title"`
	Verdict  string `json:"verdict"`
}

type AdherenceReport struct {
	Entries []AdherenceEntry `json:"entries"`
}

// CompareArm is one label's verdict in the compare judge's compare.json.
type CompareArm struct {
	Defects []CompareDefect `json:"defects"`
	Label   string          `json:"label"`
	Summary string          `json:"summary"`
}

type CompareDefect struct {
	Claim    string `json:"claim"`
	File     string `json:"file"`
	Severity string `json:"severity"`
}

// CompareReport is compare.json; Ranking lists labels, best first.
type CompareReport struct {
	Arms    []CompareArm `json:"arms"`
	Ranking []string     `json:"ranking"`
}

type armFold struct {
	adherence     []scoredEntry // nil without adherence.json
	arm           *ManifestArm
	compare       *CompareArm // nil without a compare verdict
	golden        *evals.GoldenRun
	goldenValues  []evals.MetricValue
	rank          int // 0 = unranked
	result        *ArmResult
	runtime       *ArmRuntime // nil for an arm run before runtime.json existed
	smoke         []string
	stream        *ArmStream          // the arm's parsed stream; nil when the file is missing or unreadable
	telemetry     *peek.SessionEvents // nil when peek could not serve the session
	telemetryNote string
}

// claudeCodeVersion is "" for an arm whose stream carries no init record.
func (f *armFold) claudeCodeVersion() string {
	if f.stream == nil || f.stream.Init == nil {
		return ""
	}
	return f.stream.Init.ClaudeCodeVersion
}

func (f *armFold) defectsMajor() int {
	count := 0
	for _, defect := range f.compare.Defects {
		isMajor := defect.Severity == severityBlocker || defect.Severity == severityMajor
		if isMajor {
			count++
		}
	}
	return count
}

// envelopeValues are the all-agent metrics of the arm's result envelopes.
func (f *armFold) envelopeValues() []evals.MetricValue {
	runId := f.arm.Id
	if f.result.OutputTokens == nil {
		return []evals.MetricValue{
			nullValue(metricOutputTokens, runId, noEnvelopeNote),
			nullValue(metricCostUsd, runId, noEnvelopeNote),
			nullValue(metricPermissionDenials, runId, noEnvelopeNote),
		}
	}

	return []evals.MetricValue{
		numberValue(metricOutputTokens, runId, float64(*f.result.OutputTokens)),
		numberValue(metricCostUsd, runId, *f.result.CostUsd),
		numberValue(metricPermissionDenials, runId, float64(*f.result.PermissionDenials)),
	}
}

// failedMetrics skips null golden values.
func (f *armFold) failedMetrics(thresholds map[string]float64) []string {
	failed := []string{}
	if len(f.smoke) > 0 {
		failed = append(failed, metricSmokePass)
	}
	hasFailedCheck := f.result.CheckExit != nil && *f.result.CheckExit != 0
	if hasFailedCheck {
		failed = append(failed, metricCheckPass)
	}
	for _, value := range f.goldenValues {
		bound, hasBound := thresholds[value.MetricId]
		number, isNumber := value.Number()
		isChecked := hasBound && isNumber
		if isChecked && missesBound(bound, value.MetricId, number) {
			failed = append(failed, value.MetricId)
		}
	}
	return failed
}

func (f *armFold) isValid() bool {
	return len(f.result.InvalidReasons) == 0
}

func (f *armFold) judgeValues(isCompared bool) []evals.MetricValue {
	runId := f.arm.Id
	values := make([]evals.MetricValue, 0, judgeMetricCount)
	misses, applicable := f.stepMisses()
	switch {
	case f.adherence == nil:
		values = append(values, nullValue(metricStepMissPct, runId, "no adherence.json"))
	case applicable == 0:
		values = append(values, nullValue(metricStepMissPct, runId, "no applicable entries"))
	default:
		values = append(values, numberValue(metricStepMissPct, runId, percent(misses, applicable)))
	}
	if f.adherence == nil {
		values = append(values, nullValue(metricStepUnobserved, runId, "no adherence.json"))
	} else {
		values = append(values, numberValue(metricStepUnobserved, runId, float64(f.stepUnobserved())))
	}
	if !isCompared {
		return values
	}

	if f.compare == nil {
		values = append(values, nullValue(metricDefectsMajor, runId, "no compare verdict"))
		return append(values, nullValue(metricQualityRank, runId, "no compare verdict"))
	}

	values = append(values, numberValue(metricDefectsMajor, runId, float64(f.defectsMajor())))
	return append(values, numberValue(metricQualityRank, runId, float64(f.rank)))
}

func (f *armFold) metricValues(manifest *RunManifest) []evals.MetricValue {
	runId := f.arm.Id
	values := []evals.MetricValue{boolValue(metricSmokePass, runId, len(f.smoke) == 0)}
	if f.result.CheckExit != nil {
		values = append(values, boolValue(metricCheckPass, runId, *f.result.CheckExit == 0))
	}
	values = append(values, f.telemetryValues()...)
	values = append(values, f.envelopeValues()...)
	values = append(values, f.runtimeValues()...)
	values = append(values, f.goldenValues...)
	return append(values, f.judgeValues(manifest.isCompared())...)
}

// reviewScore is the run's golden review score, ok false when unscored.
func (f *armFold) reviewScore() (float64, bool) {
	for _, value := range f.goldenValues {
		if value.MetricId == metricReviewScore {
			return value.Number()
		}
	}
	return 0, false
}

func (f *armFold) scoreCells() []evals.ScoreCell {
	cells := make([]evals.ScoreCell, 0, len(f.adherence))
	for _, scored := range f.adherence {
		if !scored.isApplicable {
			continue
		}

		cell := evals.ScoreCell{
			Evidence:      []string{scored.entry.Evidence},
			Justification: strings.TrimSpace(scored.entry.Verdict + ": " + scored.entry.Note),
			RuleId:        scored.entry.Id,
			RunId:         f.arm.Id,
			Score:         scored.score,
			Source:        scoreSource,
		}
		cells = append(cells, cell)
	}
	return cells
}

func (f *armFold) scoreValues(scoreIds []string) []evals.MetricValue {
	values := make([]evals.MetricValue, 0, len(scoreIds))
	for _, scoreId := range scoreIds {
		score, hasScore := f.result.Scores[strings.TrimPrefix(scoreId, scorePrefix)]
		if !hasScore {
			note := "no score — " + filepath.Join(runsDir, f.arm.Label, scorerLogFile)
			values = append(values, nullValue(scoreId, f.arm.Id, note))
			continue
		}
		values = append(values, numberValue(scoreId, f.arm.Id, score))
	}
	return values
}

func (f *armFold) stepMisses() (int, int) {
	misses, applicable := 0, 0
	for _, scored := range f.adherence {
		if !scored.isApplicable {
			continue
		}

		applicable++
		if scored.score < 1 {
			misses++
		}
	}
	return misses, applicable
}

func (f *armFold) stepUnobserved() int {
	count := 0
	for _, scored := range f.adherence {
		if scored.entry.Verdict == verdictNotObservable {
			count++
		}
	}
	return count
}

// telemetryValues: wall comes from the arm's own stream, so a resumed run's pause is not counted; active from peek.
func (f *armFold) telemetryValues() []evals.MetricValue {
	runId := f.arm.Id
	wall := nullValue(metricWallSeconds, runId, noStreamWallNote)
	hasWall := f.stream != nil && f.stream.WallSeconds != nil
	if hasWall {
		wall = numberValue(metricWallSeconds, runId, float64(*f.stream.WallSeconds))
	}
	active := nullValue(metricActiveSeconds, runId, f.telemetryNote)
	if f.telemetry != nil {
		active = numberValue(metricActiveSeconds, runId, float64(f.telemetry.ActiveSeconds))
	}
	return []evals.MetricValue{wall, active}
}

func (f *armFold) totals() []evals.Total {
	misses, applicable := f.stepMisses()
	if applicable == 0 {
		return nil
	}

	followed := applicable - misses
	total := evals.Total{
		Axis:  axisSelf,
		Max:   applicable,
		Pct:   percent(followed, applicable),
		Raw:   followed,
		RunId: f.arm.Id,
	}
	return []evals.Total{total}
}

// scoredEntry is scored once at ingestion.
type scoredEntry struct {
	entry        AdherenceEntry
	isApplicable bool
	score        int
}

// adherenceScore treats a verdict outside the adherence brief as a judge contract defect.
func adherenceScore(verdict string) (int, bool, error) {
	switch verdict {
	case verdictFollowed:
		return 1, true, nil
	case verdictPartial:
		return 0, true, nil
	case verdictSkipped, verdictViolated:
		return -1, true, nil
	case verdictNotApplicable, verdictNotObservable:
		return 0, false, nil
	}
	return 0, false, fmt.Errorf("adherenceScore: Unknown verdict %q", verdict)
}

func appendRubric(rubric []evals.RubricRule, entries []scoredEntry, seen map[string]bool, skillMd string) []evals.RubricRule {
	for _, scored := range entries {
		isNew := scored.isApplicable && !seen[scored.entry.Id]
		if !isNew {
			continue
		}

		seen[scored.entry.Id] = true
		rule := evals.RubricRule{
			Id:     scored.entry.Id,
			Axis:   axisSelf,
			Rule:   scored.entry.Title,
			Source: skillMd + "#" + strings.ToLower(scored.entry.Id),
		}
		rubric = append(rubric, rule)
	}
	return rubric
}

func applyCompare(folds []*armFold, runDir string) error {
	var report CompareReport
	err := readJsonFile(filepath.Join(runDir, compareFile), &report)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	if err != nil {
		return err
	}

	foldByLabel := make(map[string]*armFold, len(folds))
	for _, fold := range folds {
		foldByLabel[fold.arm.Label] = fold
	}
	for index := range report.Arms {
		fold := foldByLabel[report.Arms[index].Label]
		if fold == nil {
			return fmt.Errorf("applyCompare: Unknown label %q", report.Arms[index].Label)
		}
		fold.compare = &report.Arms[index]
	}
	for position, label := range report.Ranking {
		fold := foldByLabel[label]
		if fold == nil {
			return fmt.Errorf("applyCompare: Unknown ranked label %q", label)
		}
		fold.rank = position + 1
	}
	return nil
}

func armKeys(manifest *RunManifest) []string {
	var keys []string
	isSeen := make(map[string]bool)
	for _, arm := range manifest.Arms {
		if isSeen[arm.Key] {
			continue
		}

		isSeen[arm.Key] = true
		keys = append(keys, arm.Key)
	}
	return keys
}

func bestArm(folds []*armFold) string {
	var best *armFold
	for _, fold := range folds {
		isCandidate := fold.isValid() && len(fold.smoke) == 0
		if isCandidate && isBetter(best, fold) {
			best = fold
		}
	}
	if best == nil {
		return ""
	}
	return best.arm.Id
}

func boolValue(metricId, runId string, value bool) evals.MetricValue {
	return evals.MetricValue{MetricId: metricId, RunId: runId, Value: value}
}

func buildEvalFile(folds []*armFold, manifest *RunManifest, scorer *goldenScorer) *evals.EvalFile {
	scoreIds := scorerMetricIds(folds)
	file := &evals.EvalFile{
		Eval:          evalHeader(manifest),
		Golden:        goldenBlock(folds, scorer),
		Metrics:       metricDefs(manifest, scorer, scoreIds),
		MetricValues:  []evals.MetricValue{},
		Probes:        []evals.Probe{},
		Rubric:        []evals.RubricRule{},
		Runs:          []evals.Run{},
		SchemaVersion: schemaVersion,
		Scores:        []evals.ScoreCell{},
		SharedTotals:  []evals.Total{},
		Totals:        []evals.Total{},
		Verdict:       buildVerdict(folds, manifest, scorer),
	}
	seenRules := make(map[string]bool)
	for _, fold := range folds {
		file.MetricValues = append(file.MetricValues, fold.metricValues(manifest)...)
		file.MetricValues = append(file.MetricValues, fold.scoreValues(scoreIds)...)
		file.Probes = append(file.Probes, runProbes(fold, manifest)...)
		file.Rubric = appendRubric(file.Rubric, fold.adherence, seenRules, manifest.SkillMd)
		file.Runs = append(file.Runs, runEntry(fold, manifest))
		file.Scores = append(file.Scores, fold.scoreCells()...)
		file.Totals = append(file.Totals, fold.totals()...)
	}
	return file
}

func buildVerdict(folds []*armFold, manifest *RunManifest, scorer *goldenScorer) *evals.Verdict {
	bounds := thresholds(manifest, scorer)
	verdict := &evals.Verdict{
		BestArm:    bestArm(folds),
		PerRun:     []evals.RunVerdict{},
		Thresholds: bounds,
	}
	passes := 0
	for _, fold := range folds {
		failed := fold.failedMetrics(bounds)
		isPass := len(failed) == 0 && fold.isValid()
		if isPass {
			passes++
		}

		runVerdict := evals.RunVerdict{FailedThresholds: failed, Pass: isPass, RunId: fold.arm.Id}
		verdict.PerRun = append(verdict.PerRun, runVerdict)
	}
	failures := len(folds) - passes
	switch {
	case passes == len(folds):
		verdict.Status = statusPass
	case passes == 0:
		verdict.Status = statusFail
	default:
		verdict.Status = statusMixed
	}
	verdict.Reason = fmt.Sprintf("threshold, smoke or variant failures %d%% (%d)", failurePercent(failures, len(folds)), failures)
	return verdict
}

func cheaper(best, candidate *armFold) bool {
	hasCandidateCost := candidate.result.CostUsd != nil
	if !hasCandidateCost {
		return false
	}

	hasBestCost := best.result.CostUsd != nil
	if !hasBestCost {
		return true
	}
	return *candidate.result.CostUsd < *best.result.CostUsd
}

// configNote names what varies between runs: the definition's open args at the values the frozen prompt gave them, else the arms.
func configNote(manifest *RunManifest) string {
	var parts []string
	for _, name := range openArgNames(manifest.PresetSkillDir) {
		if value, ok := promptArgValue(manifest.Prompt, name); ok {
			parts = append(parts, name+" "+value)
		}
	}
	if len(parts) == 0 {
		return "arms: " + strings.Join(armKeys(manifest), ", ")
	}
	return strings.Join(parts, " · ")
}

// openArgNames are the args the skill's eval definition lets a run set; nil without a readable definition.
func openArgNames(skillDir string) []string {
	if skillDir == "" {
		return nil
	}

	def, err := evals.LoadDefinition(skillDir)
	if err != nil || def == nil {
		return nil
	}
	return def.Invocation.Open
}

// promptArgValue reads "<name>: <value>" from the composed prompt; open args are single tokens.
func promptArgValue(prompt, name string) (string, bool) {
	words := strings.Fields(prompt)
	for index, word := range words {
		isArg := word == name+":" && index+1 < len(words)
		if isArg {
			return words[index+1], true
		}
	}
	return "", false
}

// loadStream is nil when the stream is missing or unreadable, so wall reads null with its note and the version stays blank.
func loadStream(path string) *ArmStream {
	stream, err := parseStreamFile(path)
	if err != nil {
		return nil
	}
	return stream
}

func evalHeader(manifest *RunManifest) evals.Eval {
	notes := []string{
		configNote(manifest),
		"start commit " + shortSha(manifest.At),
	}
	for index := range notes {
		notes[index] = truncateRunes(notes[index], maxNoteRunes)
	}
	header := evals.Eval{Date: manifest.Date, NotesBullets: notes, Skill: manifest.Skill, SkillMdPath: manifest.SkillMd}
	return header
}

func fetchTelemetry(ctx context.Context, peekClient *peek.Client, sessionId string) (*peek.SessionEvents, string) {
	if sessionId == "" {
		return nil, "no session id"
	}

	callCtx, cancel := context.WithTimeout(ctx, peekCallTimeout)
	defer cancel()
	events, err := peekClient.SessionEvents(callCtx, sessionId)
	if err != nil {
		return nil, "peek: " + err.Error()
	}

	return events, ""
}

func goldenBlock(folds []*armFold, scorer *goldenScorer) *evals.Golden {
	if scorer == nil {
		return nil
	}

	block := &evals.Golden{
		GoldenFile:    scorer.path,
		GoldenId:      scorer.id,
		GoldenVersion: scorer.golden.GoldenVersion,
		NullMinutes:   scorer.nullMinutes,
		OracleMinutes: scorer.oracleMinutes,
		PerRun:        []evals.GoldenRun{},
		Scope:         scorer.scope,
		Total:         len(scorer.golden.Findings),
	}
	for _, fold := range folds {
		if fold.golden != nil {
			block.PerRun = append(block.PerRun, *fold.golden)
		}
	}
	return block
}

func hasInvoked(events *peek.SessionEvents, skill string) bool {
	for _, invocation := range events.InvokedSkills {
		if invocation.Skill == skill {
			return true
		}
	}
	return false
}

func isBetter(best, candidate *armFold) bool {
	if best == nil {
		return true
	}

	candidateScore, hasCandidateScore := candidate.reviewScore()
	bestScore, hasBestScore := best.reviewScore()
	isScoreDecided := hasCandidateScore && hasBestScore && candidateScore != bestScore
	switch {
	case isScoreDecided:
		return candidateScore > bestScore
	case candidate.rank > 0 && best.rank == 0:
		return true
	case candidate.rank > 0 && best.rank > 0:
		return candidate.rank < best.rank
	case candidate.rank == 0 && best.rank == 0:
		return cheaper(best, candidate)
	}
	return false
}

// loadAdherence returns nil for a missing file: a smoke run or a failed judge.
func loadAdherence(path string) ([]scoredEntry, error) {
	var report AdherenceReport
	err := readJsonFile(path, &report)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	scored := make([]scoredEntry, 0, len(report.Entries))
	for _, entry := range report.Entries {
		score, isApplicable, err := adherenceScore(entry.Verdict)
		if err != nil {
			return nil, fmt.Errorf("loadAdherence: %s: %s: %w", path, entry.Id, err)
		}

		scoredOne := scoredEntry{entry: entry, isApplicable: isApplicable, score: score}
		scored = append(scored, scoredOne)
	}
	return scored, nil
}

func loadArmFold(ctx context.Context, arm *ManifestArm, peekClient *peek.Client, runDir string, scorer *goldenScorer, skill string) (*armFold, error) {
	armDir := filepath.Join(runDir, runsDir, arm.Label)
	var result ArmResult
	if err := readJsonFile(filepath.Join(armDir, resultFile), &result); err != nil {
		return nil, err
	}

	adherence, err := loadAdherence(filepath.Join(armDir, adherenceFile))
	if err != nil {
		return nil, err
	}

	runtime, err := loadRuntime(filepath.Join(armDir, runtimeFile))
	if err != nil {
		return nil, err
	}

	telemetry, telemetryNote := fetchTelemetry(ctx, peekClient, result.SessionId)
	fold := &armFold{
		adherence:     adherence,
		arm:           arm,
		result:        &result,
		runtime:       runtime,
		smoke:         smokeFailures(&result, skill, telemetry),
		stream:        loadStream(filepath.Join(armDir, streamFile)),
		telemetry:     telemetry,
		telemetryNote: telemetryNote,
	}
	if scorer == nil {
		return fold, nil
	}

	var verdict judgeVerdict
	if err := readJsonFile(filepath.Join(armDir, judgeFile), &verdict); err != nil {
		return nil, err
	}

	candidates, err := loadCandidates(filepath.Join(armDir, candidatesFile))
	if err != nil {
		return nil, err
	}

	fold.golden = scorer.run(arm.Id, &verdict)
	fold.goldenValues, err = scorer.metricValues(fold.golden, candidates, fold.isValid())
	if err != nil {
		return nil, err
	}

	return fold, nil
}

func loadScorer(manifest *RunManifest, runDir string) (*goldenScorer, error) {
	if manifest.Golden.kind() != GoldenTest {
		return nil, nil
	}

	scoring, err := evals.LoadScoring(filepath.Join(runDir, scoringSnapshotFile))
	if err != nil {
		return nil, err
	}

	scorer, err := newGoldenScorer(filepath.Join(runDir, goldenSnapshotFile), scoring)
	if err != nil {
		return nil, err
	}

	scorer.id = manifest.Golden.Id
	scorer.scope = manifest.Golden.Scope
	return scorer, nil
}

func metricDefs(manifest *RunManifest, scorer *goldenScorer, scoreIds []string) []evals.Metric {
	defs := []evals.Metric{
		{Id: metricSmokePass, Direction: directionHigher, Label: "Smoke pass", Source: sourceProbe, Unit: "bool"},
		{Id: metricWallSeconds, Direction: directionLower, Label: "Wall time", Source: sourceTelemetry, Unit: "s"},
		{Id: metricActiveSeconds, Direction: directionLower, Label: "Main-loop active", Source: sourceTelemetry, Unit: "s"},
		{Id: metricOutputTokens, Direction: directionLower, Label: "Output tokens (all agents)", Source: sourceTelemetry, Unit: "tokens"},
		{Id: metricCostUsd, Direction: directionLower, Label: "Cost", Source: sourceTelemetry, Unit: "usd"},
		{Id: metricPermissionDenials, Direction: directionLower, Label: "Permission denials (all agents)", Source: sourceTelemetry, Unit: "count"},
		{Id: metricCheckFailures, Direction: directionLower, Label: "Enforce check failures", Source: sourceHeartbeat, Unit: "count"},
		{Id: metricRunsKilled, Direction: directionLower, Label: "Runs killed by heartbeat", Source: sourceHeartbeat, Unit: "count"},
	}
	if len(manifest.Check) > 0 {
		checkDef := evals.Metric{Id: metricCheckPass, Direction: directionHigher, Label: "Check pass", Source: sourceProbe, Unit: "bool"}
		defs = append(defs, checkDef)
	}
	if scorer != nil {
		defs = append(defs, evals.GoldenMetrics...)
	}
	for _, scoreId := range scoreIds {
		label := strings.TrimPrefix(scoreId, scorePrefix) + " (scorer)"
		scoreDef := evals.Metric{Id: scoreId, Direction: "", Label: label, Source: sourceScorer, Unit: "score"}
		defs = append(defs, scoreDef)
	}
	stepDefs := []evals.Metric{
		{Id: metricStepMissPct, Direction: directionLower, Label: "Step misses", Source: sourceJudge, Unit: "%"},
		{Id: metricStepUnobserved, Direction: directionLower, Label: "Unobservable entries", Source: sourceJudge, Unit: "count"},
	}
	defs = append(defs, stepDefs...)
	if !manifest.isCompared() {
		return defs
	}

	compareDefs := []evals.Metric{
		{Id: metricDefectsMajor, Direction: directionLower, Label: "Major+ defects", Source: sourceJudge, Unit: "count"},
		{Id: metricQualityRank, Direction: directionLower, Label: "Quality rank", Source: sourceJudge, Unit: "rank"},
	}
	return append(defs, compareDefs...)
}

func missesBound(bound float64, metricId string, value float64) bool {
	for _, metric := range evals.GoldenMetrics {
		if metric.Id == metricId && metric.Direction == directionHigher {
			return value < bound
		}
	}
	return value > bound
}

func nullValue(metricId, runId, note string) evals.MetricValue {
	return evals.MetricValue{MetricId: metricId, Note: note, RunId: runId, Value: nil}
}

func numberValue(metricId, runId string, value float64) evals.MetricValue {
	return evals.MetricValue{MetricId: metricId, RunId: runId, Value: value}
}

func runEntry(fold *armFold, manifest *RunManifest) evals.Run {
	model := fold.result.Model
	if model == "" {
		model = fold.arm.Model
	}
	variant := evals.Variant{Disable: []string{}, Name: fold.arm.Key}
	run := evals.Run{
		Id:                fold.arm.Id,
		ClaudeCodeVersion: fold.claudeCodeVersion(),
		Invalid:           !fold.isValid(),
		InvalidReason:     strings.Join(fold.result.InvalidReasons, "; "),
		Model:             evals.Model{Id: model, Effort: fold.arm.Effort},
		Output:            filepath.Join(runsDir, fold.arm.Label, diffFile),
		SkillMd:           manifest.SkillMd,
		Transcript:        fold.result.SessionId,
		Variant:           variant,
	}
	return run
}

func runProbes(fold *armFold, manifest *RunManifest) []evals.Probe {
	probes := make([]evals.Probe, 0, len(fold.smoke)+len(fold.result.InvalidReasons))
	for _, reason := range fold.smoke {
		probe := evals.Probe{Name: "smoke", Result: reason, RuleIds: []string{}, RunId: fold.arm.Id}
		probes = append(probes, probe)
	}
	for _, reason := range fold.result.InvalidReasons {
		probe := evals.Probe{Name: "variant", Result: reason, RuleIds: []string{}, RunId: fold.arm.Id}
		probes = append(probes, probe)
	}
	hasEmptyCandidates := fold.result.CandidateCount != nil && *fold.result.CandidateCount == 0
	if hasEmptyCandidates {
		probe := evals.Probe{Name: "candidates", Result: "zero golden candidates extracted — check the findings declaration", RuleIds: []string{}, RunId: fold.arm.Id}
		probes = append(probes, probe)
	}
	hasMissingScores := len(manifest.Scorer) > 0 && fold.result.Scores == nil
	if hasMissingScores {
		result := "no scores — see " + filepath.Join(runsDir, fold.arm.Label, scorerLogFile)
		probe := evals.Probe{Name: "scorer", Result: result, RuleIds: []string{}, RunId: fold.arm.Id}
		probes = append(probes, probe)
	}
	return probes
}

func scorerMetricIds(folds []*armFold) []string {
	isSeen := make(map[string]bool)
	for _, fold := range folds {
		for name := range fold.result.Scores {
			isSeen[scorePrefix+name] = true
		}
	}
	return slices.Sorted(maps.Keys(isSeen))
}

// smokeFailures never fails on denials, which hit every arm; an unreachable peek leaves invocation unverified.
func smokeFailures(result *ArmResult, skill string, telemetry *peek.SessionEvents) []string {
	var failures []string
	if result.SessionId == "" {
		failures = append(failures, "no session — the cell never started")
	}
	if result.ExitStatus != 0 {
		failures = append(failures, fmt.Sprintf("exit status %d", result.ExitStatus))
	}
	if result.TerminalReason != terminalCompleted {
		failures = append(failures, fmt.Sprintf("terminal reason %q", result.TerminalReason))
	}
	if result.Subtype != subtypeSuccess {
		failures = append(failures, fmt.Sprintf("result subtype %q", result.Subtype))
	}
	if result.IsError {
		failures = append(failures, "result flagged is_error")
	}
	hasFailedCheck := result.CheckExit != nil && *result.CheckExit != 0
	if hasFailedCheck {
		failures = append(failures, fmt.Sprintf("check exited %d", *result.CheckExit))
	}
	isNotInvoked := telemetry != nil && !hasInvoked(telemetry, skill)
	if isNotInvoked {
		failures = append(failures, "skill "+skill+" never invoked")
	}
	return failures
}

func thresholds(manifest *RunManifest, scorer *goldenScorer) map[string]float64 {
	bounds := map[string]float64{metricSmokePass: 1}
	if len(manifest.Check) > 0 {
		bounds[metricCheckPass] = 1
	}
	if scorer == nil {
		return bounds
	}

	maps.Copy(bounds, scorer.scoring.Thresholds)
	return bounds
}

// validateEvalFile validates the way the acdsl jsonschema verifier does.
func validateEvalFile(data []byte, schemaPath string) error {
	schema, err := jsonschema.NewCompiler().Compile(schemaPath)
	if err != nil {
		return fmt.Errorf("validateEvalFile: %s: %w", schemaPath, err)
	}

	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("validateEvalFile: %w", err)
	}

	if err := schema.Validate(instance); err != nil {
		return fmt.Errorf("validateEvalFile: %w", err)
	}

	return nil
}

// Fold writes eval.json only when it validates against schemaPath.
func Fold(ctx context.Context, peekClient *peek.Client, runDir, schemaPath string) error {
	manifest, err := loadManifest(runDir)
	if err != nil {
		return err
	}

	scorer, err := loadScorer(manifest, runDir)
	if err != nil {
		return err
	}

	folds := make([]*armFold, 0, len(manifest.Arms))
	for index := range manifest.Arms {
		fold, err := loadArmFold(
			ctx,
			&manifest.Arms[index],
			peekClient,
			runDir,
			scorer,
			manifest.Skill,
		)
		if err != nil {
			return err
		}
		folds = append(folds, fold)
	}

	if err := applyCompare(folds, runDir); err != nil {
		return err
	}

	data, err := json.MarshalIndent(buildEvalFile(folds, manifest, scorer), "", "  ")
	if err != nil {
		return fmt.Errorf("Fold: %w", err)
	}

	if err := validateEvalFile(data, schemaPath); err != nil {
		_ = os.WriteFile(filepath.Join(runDir, rejectedFileName), data, 0o644)
		return err
	}

	if err := os.WriteFile(filepath.Join(runDir, evalFileName), append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("Fold: %w", err)
	}

	report := renderReport(folds, manifest)
	if err := os.WriteFile(filepath.Join(runDir, reportFile), []byte(report), 0o644); err != nil {
		return fmt.Errorf("Fold: Failed to write report: %w", err)
	}

	return nil
}

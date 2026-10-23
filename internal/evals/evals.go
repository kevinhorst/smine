// Package evals holds the eval layer's shapes: definitions, run outputs and A/B files, legacy layouts included.
package evals

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// DefinitionFile is a skill's eval definition, relative to its skill dir.
const DefinitionFile = "eval/definition.json"

// Golden metric vocabulary shared by GoldenMetrics.
const (
	metricDirectionHigher = "higher"
	metricDirectionLower  = "lower"
	metricSourceGolden    = "golden"
	metricUnitCount       = "count"
	metricUnitMinutes     = "min"
	metricUnitRatio       = "ratio"
)

// The minute metrics price a run against the golden family's scoring.json.
var GoldenMetrics = []Metric{
	{Id: "recall", Direction: metricDirectionHigher, Label: "Golden recall", Source: metricSourceGolden, Unit: metricUnitRatio},
	{Id: "review_score", Direction: metricDirectionHigher, Label: "Review score (share of the savable minutes captured)", Source: metricSourceGolden, Unit: metricUnitRatio},
	{Id: "lost_minutes", Direction: metricDirectionLower, Label: "Lost minutes vs a perfect review", Source: metricSourceGolden, Unit: metricUnitMinutes},
	{Id: "escaped_minutes", Direction: metricDirectionLower, Label: "Escaped minutes (missed and merged)", Source: metricSourceGolden, Unit: metricUnitMinutes},
	{Id: "false_positive_minutes", Direction: metricDirectionLower, Label: "False-positive minutes", Source: metricSourceGolden, Unit: metricUnitMinutes},
	{Id: "false_positives", Direction: metricDirectionLower, Label: "False positives", Source: metricSourceGolden, Unit: metricUnitCount},
	{Id: "misclassified", Direction: metricDirectionLower, Label: "Misclassified severity", Source: metricSourceGolden, Unit: metricUnitCount},
}

// Lowest first; an unknown severity ranks below every line.
var severityOrder = []string{"INFO", "NIT", "MINOR", "MAJOR", "BLOCKER"}

// An A/B dir (ab-…) never matches.
var RunDirPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-[0-9a-f]{6,12}(-\d+)?$`)

// From the retired matrix runner; kept for historical eval dirs.
type Delta struct {
	Arm       string  `json:"arm"`
	Axis      string  `json:"axis"`
	DeltaPct  float64 `json:"delta_pct"`
	Dimension string  `json:"dimension"`
	N         int     `json:"n"`
}

// Scoring loads from the declared file, one per golden family; Scorer applies only without a golden.
type Definition struct {
	Budgets       DefinitionBudgets     `json:"budgets"`
	Check         []string              `json:"check"`
	Effort        string                `json:"effort"`
	Environment   DefinitionEnvironment `json:"environment"`
	Findings      DefinitionFindings    `json:"findings"`
	Goldens       []DefinitionGolden    `json:"goldens"`
	Invocation    DefinitionInvocation  `json:"invocation"`
	Model         string                `json:"model"`
	Prerequisites []string              `json:"prerequisites"`
	Reference     string                `json:"reference"`
	Replicas      int                   `json:"replicas"`
	SchemaVersion string                `json:"schemaVersion"`
	Scorer        DefinitionScorer      `json:"scorer"`
	Scoring       *Scoring              `json:"-"`
	ScoringFile   string                `json:"scoring"`
	Skill         string                `json:"skill"`
}

type DefinitionArg struct {
	Name  string             `json:"name"`
	Value DefinitionArgValue `json:"value,omitempty"`
}

func (a DefinitionArg) String() string {
	if a.Value == "" {
		return a.Name
	}
	return a.Name + ": " + string(a.Value)
}

// In JSON a string, or a list of strings that is joined by spaces.
type DefinitionArgValue string

func (v *DefinitionArgValue) UnmarshalJSON(data []byte) error {
	var words []string
	if err := json.Unmarshal(data, &words); err == nil {
		*v = DefinitionArgValue(strings.Join(words, " "))
		return nil
	}

	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return fmt.Errorf("DefinitionArgValue.UnmarshalJSON: %w", err)
	}

	*v = DefinitionArgValue(text)
	return nil
}

type DefinitionBudgets struct {
	ArmTimeoutS int     `json:"armTimeoutS"`
	ArmUsd      float64 `json:"armUsd"`
	WindowUsd   float64 `json:"windowUsd"`
}

// An empty registry repo is the eval repo; the checkout commit defaults to the range head.
type DefinitionEnvironment struct {
	CheckoutRef  string              `json:"checkoutRef"`
	ContextIndex string              `json:"contextIndex"`
	Overlay      string              `json:"overlay"`
	Prebuild     *DefinitionPrebuild `json:"prebuild"`
	Range        DefinitionRange     `json:"range"`
	TargetRepo   string              `json:"targetRepo"`
}

// An empty array key means the document is the array; claim keys are in fallback order.
type DefinitionFindings struct {
	ArrayKey  string   `json:"arrayKey"`
	ClaimKeys []string `json:"claimKeys"`
	Glob      string   `json:"glob"`
}

// Summary fields stay zero when the skill-owned golden file lacks the optional range/severity shape.
type DefinitionGolden struct {
	File          string              `json:"file"`
	GoldenVersion int                 `json:"goldenVersion"`
	Id            string              `json:"id"`
	Input         string              `json:"input"`
	Produce       DefinitionGoldenRun `json:"produce"`
	Scope         string              `json:"scope"`
	Test          DefinitionGoldenRun `json:"test"`

	BaseLabel      string
	FindingCount   int
	HeadLabel      string
	OracleMinutes  float64
	Production     *GoldenProduction
	SeverityCounts map[string]int
}

type DefinitionGoldenRun struct {
	Args     []DefinitionArg `json:"args"`
	SeedFile string          `json:"seedFile"`
}

type DefinitionInvocation struct {
	Args []DefinitionArg `json:"args"`
	Open []string        `json:"open"`
	Text string          `json:"text"`
}

// Outputs are the paths the arm's workers link instead of building.
type DefinitionPrebuild struct {
	Command []string `json:"command"`
	Outputs []string `json:"outputs"`
}

// Its shas fill the invocation's {base} and {head}.
type DefinitionRange struct {
	BaseLabel    string   `json:"baseLabel"`
	BaseSha      string   `json:"baseSha"`
	ExcludePaths []string `json:"excludePaths"`
	HeadLabel    string   `json:"headLabel"`
	HeadSha      string   `json:"headSha"`
}

// Runs in each arm worktree after the check; stdout {"metrics": {"<name>": <number>}}.
type DefinitionScorer struct {
	Command []string `json:"command"`
}

// Only the retired shell fold wrote armruns.jsonl; the Go fold writes none.
type ArmRun struct {
	Id string `json:"id"`

	AgentTokens  *float64    `json:"agent_tokens"`
	Arm          string      `json:"arm"`
	Judge        ArmRunJudge `json:"judge"`
	OutputTokens *float64    `json:"output_tokens"`
	TouchedFiles []string    `json:"touched_files"`
}

// ArmRunJudge is the judge block of an armruns row, reduced to the matches.
type ArmRunJudge struct {
	Matched []ArmRunJudgeMatch `json:"matched"`
}

// ArmRunJudgeMatch is one matched golden finding of an armruns judge block.
type ArmRunJudgeMatch struct {
	GoldenId string `json:"goldenId"`
}

type Eval struct {
	Date         string   `json:"date"`
	Notes        string   `json:"notes"`
	NotesBullets []string `json:"notesBullets"`
	Skill        string   `json:"skill"`
	SkillMdPath  string   `json:"skillMdPath"`
}

// Dir is the evals-root-relative path in both the flat and the nested layout.
type EvalDir struct {
	ArmRuns        []ArmRun // nil when the dir carries no armruns.jsonl
	Deltas         []Delta
	Dir            string // basename under the evals root, e.g. concept-2026-08-18
	Eval           EvalFile
	Files          []string // dir-relative artifact files: top level, plus files directly under runs/ and skills/
	Interpretation string   // interpretation.md markdown — eval-authored, human-curated; "" without the file
	Stats          *Stats   // nil when the dir carries no stats.json
}

type EvalFile struct {
	sourcePath string

	Eval          Eval          `json:"eval"`
	Golden        *Golden       `json:"golden,omitempty"`
	Metrics       []Metric      `json:"metrics"`
	MetricValues  []MetricValue `json:"metricValues"`
	Probes        []Probe       `json:"probes"`
	Rubric        []RubricRule  `json:"rubric"`
	Runs          []Run         `json:"runs"`
	SchemaVersion string        `json:"schemaVersion"`
	Scores        []ScoreCell   `json:"scores"`
	Setup         *Setup        `json:"setup"`
	SharedTotals  []Total       `json:"sharedTotals"`
	Totals        []Total       `json:"totals"`
	Verdict       *Verdict      `json:"verdict"`
}

// A pre-2.0 file is displayed with its stored axes rather than relabeled.
func (e *EvalFile) IsLegacy() bool {
	return e.SchemaVersion == ""
}

func (e *EvalFile) SourcePath() string {
	return e.sourcePath
}

type Golden struct {
	GoldenFile    string         `json:"goldenFile"`
	GoldenId      string         `json:"goldenId"`
	GoldenVersion int            `json:"goldenVersion"`
	NullMinutes   float64        `json:"nullMinutes"`
	OracleMinutes float64        `json:"oracleMinutes"`
	PerRun        []GoldenRun    `json:"perRun"`
	Scope         string         `json:"scope,omitempty"`
	Total         int            `json:"total"`
	Touched       *GoldenTouched `json:"touched,omitempty"`
}

// Present only in run dirs the retired shell fold wrote from armruns rows with touched_files.
type GoldenTouched struct {
	GoldenFiles           int `json:"goldenFiles"`
	TouchedGoldenFiles    int `json:"touchedGoldenFiles"`
	UnionReachableGoldens int `json:"unionReachableGoldens"`
}

// Telemetry fields are pointers so unknown stays distinguishable from zero.
type GoldenContribution struct {
	CostUsd        *float64 `json:"costUsd"`
	Dir            string   `json:"dir"`
	Findings       int      `json:"findings"`
	NewFindings    int      `json:"newFindings"`
	OutputTokens   *int64   `json:"outputTokens"`
	Rounds         *int     `json:"rounds"`
	Source         string   `json:"source"`
	SubagentTokens *int64   `json:"subagentTokens"`
	Version        int      `json:"version"`
	WallS          *int64   `json:"wallS"`
}

type GoldenProduction struct {
	Contributions []GoldenContribution `json:"contributions"`
	Totals        GoldenContribution   `json:"totals"`
}

type GoldenFalsePositive struct {
	FindingId string `json:"findingId"`
	Reason    string `json:"reason"`
}

type GoldenMatch struct {
	FindingId string `json:"findingId"`
	GoldenId  string `json:"goldenId"`
	Note      string `json:"note"`
}

type GoldenMisclassified struct {
	Actual    string `json:"actual"`
	Expected  string `json:"expected"`
	FindingId string `json:"findingId"`
	GoldenId  string `json:"goldenId"`
}

type GoldenRun struct {
	FalsePositives []GoldenFalsePositive `json:"falsePositives"`
	FixEscapes     []string              `json:"fixEscapes"`
	Matched        []GoldenMatch         `json:"matched"`
	Misclassified  []GoldenMisclassified `json:"misclassified"`
	Missed         []string              `json:"missed"`
	RunId          string                `json:"runId"`
	Severities     []GoldenSeverity      `json:"severities,omitempty"`
	Uncataloged    []GoldenFalsePositive `json:"uncataloged"`
}

// GoldenSeverity is one severity's counts and priced minutes of a run (v2.12, outcomes v2.13); false positives count under their claimed severity.
type GoldenSeverity struct {
	Entries              int              `json:"entries"`
	EscapedMinutes       float64          `json:"escapedMinutes"`
	FalsePositiveMinutes float64          `json:"falsePositiveMinutes"`
	FalsePositives       int              `json:"falsePositives"`
	Found                int              `json:"found"` // entries a run finding matched, the ones priced as escaped included
	FoundMinutes         float64          `json:"foundMinutes"`
	Missed               int              `json:"missed"`
	Outcomes             SeverityOutcomes `json:"outcomes"`
	Severity             string           `json:"severity"`
}

// OutcomeTally is how many golden entries ended in one outcome and the minutes they cost.
type OutcomeTally struct {
	Count   int     `json:"count"`
	Minutes float64 `json:"minutes"`
}

// SeverityOutcomes puts every golden entry of a severity in exactly one outcome; the tallies sum to the row's entries and to its found plus escaped minutes.
type SeverityOutcomes struct {
	AtPrice       OutcomeTally `json:"atPrice"`       // found, left open, reported at a severity of the same price: the price
	Dismissed     OutcomeTally `json:"dismissed"`     // found, a human-side entry reported below the triage line: the escape price
	FixFailed     OutcomeTally `json:"fixFailed"`     // found and machine-fixed, the defect still in the final tree: the escape price
	FixUnverified OutcomeTally `json:"fixUnverified"` // found, fix applied but never confirmed: the fix price plus the price
	Fixed         OutcomeTally `json:"fixed"`         // found, fix confirmed: the fix price
	Missed        OutcomeTally `json:"missed"`        // no run finding matched: the escape price
	WrongSeverity OutcomeTally `json:"wrongSeverity"` // found, left open, reported at a severity of another price: the price plus the price distance
}

// Escaped sums the outcomes priced at the escape multiplier: the entry ships, found or not.
func (o *SeverityOutcomes) Escaped() OutcomeTally {
	escaped := OutcomeTally{
		Count:   o.Dismissed.Count + o.FixFailed.Count + o.Missed.Count,
		Minutes: o.Dismissed.Minutes + o.FixFailed.Minutes + o.Missed.Minutes,
	}
	return escaped
}

// Metric is one numeric measure of the output axis; MetricValue its per-run value.
type Metric struct {
	Id        string `json:"id"`
	Direction string `json:"direction"`
	Label     string `json:"label"`
	Source    string `json:"source"`
	Unit      string `json:"unit"`
}

type MetricValue struct {
	MetricId string `json:"metricId"`
	Note     string `json:"note"`
	RunId    string `json:"runId"`
	Value    any    `json:"value"`
}

// Booleans fold to 0/1; null and strings are skipped.
func (v MetricValue) Number() (float64, bool) {
	switch typed := v.Value.(type) {
	case float64:
		return typed, true
	case bool:
		if typed {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

type Model struct {
	Id     string `json:"id"`
	Effort string `json:"effort"`
	Mode   string `json:"mode"`
}

// Probe is one mechanical pre-scoring check recorded by the eval.
type Probe struct {
	Name    string   `json:"name"`
	Result  string   `json:"result"`
	RuleIds []string `json:"ruleIds"`
	RunId   string   `json:"runId,omitempty"`
}

// RubricRule is one frozen rubric row of an axis.
type RubricRule struct {
	Id     string `json:"id"`
	Axis   string `json:"axis"`
	Phase  string `json:"phase"`
	Rule   string `json:"rule"`
	Source string `json:"source"`
}

type Run struct {
	Id                string  `json:"id"`
	ClaudeCodeVersion string  `json:"claudeCodeVersion,omitempty"` // v2.13 — the CLI the cell ran under; "" before it or for a Desktop arm
	Invalid           bool    `json:"invalid"`
	InvalidReason     string  `json:"invalidReason"`
	Model             Model   `json:"model"`
	Output            string  `json:"output"`
	SkillMd           string  `json:"skillMd"`
	Transcript        string  `json:"transcript"`
	Variant           Variant `json:"variant"`
}

// Justification and evidence are present for every non-+1 cell (schema v2 contract).
type ScoreCell struct {
	Evidence      []string `json:"evidence"`
	Justification string   `json:"justification"`
	RuleId        string   `json:"ruleId"`
	RunId         string   `json:"runId"`
	Score         int      `json:"score"`
	Source        string   `json:"source"`
}

type Scoring struct {
	EscapeMultiplier map[string]float64 `json:"escapeMultiplier"`
	FixFactor        map[string]float64 `json:"fixFactor"`
	JudgeGuide       string             `json:"judgeGuide"`
	Minutes          map[string]float64 `json:"minutes"`
	Thresholds       map[string]float64 `json:"thresholds"`
	TriageLine       string             `json:"triageLine"`
}

// A finding below the triage line is never looked at by a human.
func (s *Scoring) IsHumanSide(severity string) bool {
	return slices.Index(severityOrder, severity) >= slices.Index(severityOrder, s.TriageLine)
}

// A nil scoring or an unpriced severity is an error.
func (s *Scoring) SeverityCost(severity string) (float64, float64, error) {
	if s == nil {
		return 0, 0, errors.New("Scoring.SeverityCost: No scoring")
	}

	minutes, hasMinutes := s.Minutes[severity]
	multiplier, hasMultiplier := s.EscapeMultiplier[severity]
	isPriced := hasMinutes && hasMultiplier
	if !isPriced {
		return 0, 0, fmt.Errorf("Scoring.SeverityCost: Unpriced severity %q", severity)
	}
	return minutes, multiplier, nil
}

// RunVerdict is one run's pass/fail against the definition's thresholds.
type RunVerdict struct {
	FailedThresholds []string `json:"failedThresholds"`
	Pass             bool     `json:"pass"`
	RunId            string   `json:"runId"`
}

// Absent from Go-fold runs; only the fields the UI consumes are typed.
type Setup struct {
	Run      *SetupRun `json:"run"`
	RuntimeS *float64  `json:"runtimeS"`
}

// SetupRun is the runner-config slice of setup.run the page displays.
type SetupRun struct {
	Desktop *SetupDesktop `json:"desktop"`
}

// Wraps names the workflow script when the arms ran via the Workflow tool.
type SetupDesktop struct {
	Wraps string `json:"wraps"`
}

// Written by cmd/evalstats: per-arm statistics and Welch's t-test against a baseline arm.
type Stats struct {
	Baseline    string            `json:"baseline"`
	Comparisons []StatsComparison `json:"comparisons"`
	Groups      []StatsGroup      `json:"groups"`
	Notes       string            `json:"notes"`
}

type StatsComparison struct {
	Arm         string  `json:"arm"`
	DeltaMean   float64 `json:"deltaMean"`
	Df          float64 `json:"df"`
	MetricId    string  `json:"metricId"`
	Note        string  `json:"note"`
	Significant bool    `json:"significant"`
	WelchT      float64 `json:"welchT"`
}

type StatsGroup struct {
	Arm      string  `json:"arm"`
	Mean     float64 `json:"mean"`
	MetricId string  `json:"metricId"`
	N        int     `json:"n"`
	Note     string  `json:"note"`
	StdDev   float64 `json:"stddev"`
	Variance float64 `json:"variance"`
}

type Total struct {
	Axis  string  `json:"axis"`
	Max   int     `json:"max"`
	Pct   float64 `json:"pct"`
	Raw   int     `json:"raw"`
	RunId string  `json:"runId"`
}

// Variant names the disable list a run was rendered with; empty for the full skill.
type Variant struct {
	Disable []string `json:"disable"`
	Name    string   `json:"name"`
}

// The harness computes it against the definition's thresholds so nothing is left for the reader.
type Verdict struct {
	BestArm    string             `json:"bestArm"`
	PerRun     []RunVerdict       `json:"perRun"`
	Reason     string             `json:"reason"`
	Status     string             `json:"status"`
	Thresholds map[string]float64 `json:"thresholds"`
}

func artifactFiles(path string) []string {
	var files []string
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
			continue
		}
		if entry.Name() != "runs" && entry.Name() != "skills" {
			continue
		}
		subEntries, err := os.ReadDir(filepath.Join(path, entry.Name()))
		if err != nil {
			continue
		}
		for _, subEntry := range subEntries {
			if !subEntry.IsDir() {
				files = append(files, filepath.Join(entry.Name(), subEntry.Name()))
			}
		}
	}
	sort.Strings(files)
	return files
}

// An absent or partly malformed file degrades to what parsed; the page never fails on it.
func loadArmRuns(path string) []ArmRun {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var runs []ArmRun
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var row ArmRun
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			continue
		}
		runs = append(runs, row)
	}
	return runs
}

func loadEvalDir(path, name string) (*EvalDir, []string) {
	var loadErrors []string
	data, err := os.ReadFile(filepath.Join(path, "eval.json"))
	if err != nil {
		loadErrors = append(loadErrors, fmt.Sprintf("%s: %v", name, err))
		return nil, loadErrors
	}

	var file EvalFile
	if err := json.Unmarshal(data, &file); err != nil {
		loadErrors = append(loadErrors, fmt.Sprintf("%s/eval.json: %v", name, err))
		return nil, loadErrors
	}
	file.sourcePath = filepath.Join(path, "eval.json")

	var deltas []Delta
	deltasData, err := os.ReadFile(filepath.Join(path, "deltas.json"))
	if err == nil {
		if err := json.Unmarshal(deltasData, &deltas); err != nil {
			loadErrors = append(loadErrors, fmt.Sprintf("%s/deltas.json: %v", name, err))
		}
	}

	var stats *Stats
	statsData, err := os.ReadFile(filepath.Join(path, "stats.json"))
	if err == nil {
		stats = &Stats{}
		if err := json.Unmarshal(statsData, stats); err != nil {
			loadErrors = append(loadErrors, fmt.Sprintf("%s/stats.json: %v", name, err))
			stats = nil
		}
	}

	interpretation, _ := os.ReadFile(filepath.Join(path, "interpretation.md"))

	evalDir := &EvalDir{
		ArmRuns:        loadArmRuns(filepath.Join(path, "armruns.jsonl")),
		Deltas:         deltas,
		Dir:            name,
		Eval:           file,
		Files:          artifactFiles(path),
		Interpretation: string(interpretation),
		Stats:          stats,
	}
	return evalDir, loadErrors
}

// Malformed files land in the errors list, never fail the page (sessions LoadErrors pattern).
func LoadForSkill(dir, skill string) ([]EvalDir, []string) {
	dirPattern := regexp.MustCompile("^" + regexp.QuoteMeta(skill) + `-(\d{4}-\d{2}-\d{2}(-\d+)?|[0-9a-f]{6,12})$`)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		loadErrors := []string{fmt.Sprintf("%s: %v", dir, err)}
		return nil, loadErrors
	}

	// One run directory in either the flat or the nested layout.
	type evalCandidate struct{ path, relName string }
	var candidates []evalCandidate
	for _, entry := range entries {
		if entry.IsDir() && dirPattern.MatchString(entry.Name()) {
			candidates = append(candidates, evalCandidate{path: filepath.Join(dir, entry.Name()), relName: entry.Name()})
		}
	}
	skillRoot := filepath.Join(dir, skill)
	if nestedEntries, err := os.ReadDir(skillRoot); err == nil {
		for _, entry := range nestedEntries {
			if entry.IsDir() && RunDirPattern.MatchString(entry.Name()) {
				candidates = append(candidates, evalCandidate{path: filepath.Join(skillRoot, entry.Name()), relName: filepath.Join(skill, entry.Name())})
			}
		}
	}

	var dirs []EvalDir
	var loadErrors []string
	for _, candidate := range candidates {
		evalDir, dirErrors := loadEvalDir(candidate.path, candidate.relName)
		loadErrors = append(loadErrors, dirErrors...)
		if evalDir != nil {
			dirs = append(dirs, *evalDir)
		}
	}
	sort.Slice(dirs, func(i, j int) bool {
		if dirs[i].Eval.Eval.Date != dirs[j].Eval.Eval.Date {
			return dirs[i].Eval.Eval.Date > dirs[j].Eval.Eval.Date
		}
		return dirs[i].Dir > dirs[j].Dir
	})
	return dirs, loadErrors
}

// A missing definition is (nil, nil); an off-shape skill-owned golden yields zero summaries, never an error.
func LoadDefinition(skillDir string) (*Definition, error) {
	data, err := os.ReadFile(filepath.Join(skillDir, filepath.FromSlash(DefinitionFile)))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("LoadDefinition: read %s: %w", skillDir, err)
	}

	var def Definition
	if err := json.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("LoadDefinition: parse %s/eval/definition.json: %w", skillDir, err)
	}

	if def.ScoringFile != "" {
		scoring, err := LoadScoring(filepath.Join(skillDir, def.ScoringFile))
		if err != nil {
			return nil, err
		}
		def.Scoring = scoring
	}

	for index := range def.Goldens {
		summarizeGolden(skillDir, def.Scoring, &def.Goldens[index])
	}
	return &def, nil
}

func LoadScoring(path string) (*Scoring, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("LoadScoring: read %s: %w", path, err)
	}

	var scoring Scoring
	if err := json.Unmarshal(data, &scoring); err != nil {
		return nil, fmt.Errorf("LoadScoring: parse %s: %w", path, err)
	}

	return &scoring, nil
}

type goldenSummary struct {
	Findings []struct {
		Severity string `json:"severity"`
	} `json:"findings"`
	Production *GoldenProduction `json:"production"`
	Range      struct {
		BaseLabel string `json:"baseLabel"`
		HeadLabel string `json:"headLabel"`
	} `json:"range"`
}

// An unpriced severity adds no oracle minutes; golden content is skill-owned.
func summarizeGolden(skillDir string, scoring *Scoring, golden *DefinitionGolden) {
	data, err := os.ReadFile(filepath.Join(skillDir, golden.File))
	if err != nil {
		return
	}

	var summary goldenSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		return
	}

	golden.BaseLabel = summary.Range.BaseLabel
	golden.HeadLabel = summary.Range.HeadLabel
	golden.FindingCount = len(summary.Findings)
	golden.Production = summary.Production
	golden.SeverityCounts = make(map[string]int)
	for _, finding := range summary.Findings {
		golden.SeverityCounts[finding.Severity]++
		minutes, _, err := scoring.SeverityCost(finding.Severity)
		if err == nil {
			golden.OracleMinutes += minutes
		}
	}
}

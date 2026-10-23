package skilleval

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/kevinhorst/smine/internal/evals"
)

const (
	abDirPrefix         = "ab-"
	abSchemaVersion     = "1"
	measureAnchorsRatio = "anchors_hit_ratio"
	measureChecksRatio  = "checks_passed_ratio"
	statsFile           = "stats.json"
)

// ErrNotRunnable exists because the smoke layer keeps one record per deployed version.
var ErrNotRunnable = errors.New("variant cannot run on this layer")

// AbInitRequest's Replicas 0 takes the definition's count.
type AbInitRequest struct {
	Date       string
	EvalsRoot  string
	Layer      string
	Replicas   int
	Skill      string
	SkillsRoot string
	Token      string
	Variant    string
}

// statsKey addresses one stats.json group: a side key and a measure.
type statsKey struct {
	arm    string
	metric string
}

func abDifferences(file *evals.AbFile, stats *evals.Stats) []evals.AbDifference {
	groups := make(map[statsKey]evals.StatsGroup, len(stats.Groups))
	for _, group := range stats.Groups {
		groups[statsKey{arm: group.Arm, metric: group.MetricId}] = group
	}

	differences := make([]evals.AbDifference, 0, len(stats.Comparisons))
	for _, comparison := range stats.Comparisons {
		baseline := groups[statsKey{arm: file.Baseline.Key, metric: comparison.MetricId}]
		variant := groups[statsKey{arm: comparison.Arm, metric: comparison.MetricId}]
		difference := evals.AbDifference{
			BaselineMean:   baseline.Mean,
			BaselineN:      baseline.N,
			BaselineStdDev: baseline.StdDev,
			DeltaMean:      comparison.DeltaMean,
			Df:             comparison.Df,
			Measure:        comparison.MetricId,
			Note:           comparison.Note,
			Significant:    comparison.Significant,
			VariantMean:    variant.Mean,
			VariantN:       variant.N,
			VariantStdDev:  variant.StdDev,
			WelchT:         comparison.WelchT,
		}
		differences = append(differences, difference)
	}
	return differences
}

// collectEvalRecords relies on A/B dirs never matching RunDirPattern.
func collectEvalRecords(file *evals.AbFile, skillRoot string) error {
	entries, err := os.ReadDir(skillRoot)
	if err != nil {
		return fmt.Errorf("collectEvalRecords: %w", err)
	}

	for _, entry := range entries {
		isRunDir := entry.IsDir() && evals.RunDirPattern.MatchString(entry.Name())
		if !isRunDir {
			continue
		}

		if err := collectRunDirRecords(file, filepath.Join(skillRoot, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func collectRunDirRecords(file *evals.AbFile, runDir string) error {
	manifest, err := loadManifest(runDir)
	if errors.Is(err, fs.ErrNotExist) {
		file.Notes = append(file.Notes, "skipped "+runDir+": no run.json")
		return nil
	}
	if err != nil {
		return err
	}

	evalFile, err := loadEvalFile(runDir)
	if errors.Is(err, fs.ErrNotExist) {
		file.Notes = append(file.Notes, "skipped "+runDir+": no eval.json, not folded")
		return nil
	}
	if err != nil {
		return err
	}

	for index := range manifest.Arms {
		arm := &manifest.Arms[index]
		result, err := loadArmResult(arm.Label, runDir)
		if err != nil {
			return err
		}

		side := file.SideOf(result.SkillVersion)
		isEvalRecord := result.Layer == LayerEval && result.Skill == file.Skill
		if !isEvalRecord || side == nil {
			continue
		}

		record := evals.AbRecord{
			Measures:     runMeasures(evalFile, arm.Id),
			Path:         filepath.Join(runDir, runsDir, arm.Label, resultFile),
			SkillVersion: result.SkillVersion,
		}
		side.Records = append(side.Records, record)
	}
	return nil
}

func collectRunRecords(abDir string, file *evals.AbFile) error {
	manifest, err := loadManifest(abDir)
	if err != nil {
		return err
	}

	evalFile, err := loadEvalFile(abDir)
	if err != nil {
		return err
	}

	for index := range manifest.Arms {
		arm := &manifest.Arms[index]
		side := file.SideOf(arm.Key)
		if side == nil {
			return fmt.Errorf("collectRunRecords: %s: Arm %s key %s is neither side", abDir, arm.Label, arm.Key)
		}

		result, err := loadArmResult(arm.Label, abDir)
		if err != nil {
			return err
		}

		record := evals.AbRecord{
			Measures:     runMeasures(evalFile, arm.Id),
			Path:         filepath.Join(abDir, runsDir, arm.Label, resultFile),
			SkillVersion: result.SkillVersion,
		}
		side.Records = append(side.Records, record)
	}
	return nil
}

// collectSmokeRecords reads the cached last smoke run of each side's deployed version.
func collectSmokeRecords(file *evals.AbFile) error {
	sides := []*evals.AbSide{&file.Baseline, &file.Variant}
	for _, side := range sides {
		path, err := smokeRecordPath(file.Skill, side.Key)
		if err != nil {
			return err
		}

		var result ArmResult
		err = readJsonFile(path, &result)
		if errors.Is(err, fs.ErrNotExist) {
			file.Notes = append(file.Notes, "no smoke record of version "+side.Key+" at "+path)
			continue
		}
		if err != nil {
			return err
		}

		record := evals.AbRecord{Measures: smokeMeasures(&result), Path: path, SkillVersion: result.SkillVersion}
		side.Records = append(side.Records, record)
	}
	return nil
}

func descriptorHash(descriptor string) string {
	sum := sha256.Sum256([]byte(descriptor))
	return hex.EncodeToString(sum[:])[:runHashLength]
}

func differenceRow(difference *evals.AbDifference) string {
	significant := "no"
	if difference.Significant {
		significant = "**yes**"
	}

	cells := []string{
		difference.Measure,
		spreadCell(difference.BaselineN, difference.BaselineMean, difference.Measure, difference.BaselineStdDev),
		spreadCell(difference.VariantN, difference.VariantMean, difference.Measure, difference.VariantStdDev),
		fmt.Sprintf("%+.2f", difference.DeltaMean),
		fmt.Sprintf("%.2f", difference.WelchT),
		fmt.Sprintf("%.1f", difference.Df),
		significant,
		cellText(difference.Note),
	}
	return "| " + strings.Join(cells, " | ") + " |"
}

func labelKeys(abDir string, compare *CompareReport) (map[string]string, error) {
	if compare == nil {
		return nil, nil
	}

	manifest, err := loadManifest(abDir)
	if err != nil {
		return nil, err
	}

	keys := make(map[string]string, len(manifest.Arms))
	for _, arm := range manifest.Arms {
		keys[arm.Label] = arm.Key
	}
	return keys, nil
}

func loadAbFile(abDir string) (*evals.AbFile, error) {
	var file evals.AbFile
	if err := readJsonFile(filepath.Join(abDir, evals.AbFileName), &file); err != nil {
		return nil, err
	}

	return &file, nil
}

func loadArmResult(label, runDir string) (*ArmResult, error) {
	var result ArmResult
	if err := readJsonFile(filepath.Join(runDir, runsDir, label, resultFile), &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// loadCompare reads the compare judge's compare.json; nil when none ran.
func loadCompare(abDir string) (*CompareReport, error) {
	var report CompareReport
	err := readJsonFile(filepath.Join(abDir, compareFile), &report)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return &report, nil
}

func loadEvalFile(runDir string) (*evals.EvalFile, error) {
	var file evals.EvalFile
	if err := readJsonFile(filepath.Join(runDir, evalFileName), &file); err != nil {
		return nil, err
	}

	return &file, nil
}

func renderAbReport(compare *CompareReport, file *evals.AbFile, keys map[string]string) string {
	var out strings.Builder
	fmt.Fprintf(&out, "# %s — A/B %s\n\n", file.Skill, file.Date)
	fmt.Fprintf(&out, "- layer `%s`, variant `%s`\n", file.Layer, file.Descriptor)
	fmt.Fprintf(
		&out,
		"- baseline `%s`: %d records; variant `%s`: %d records\n\n",
		file.Baseline.Key,
		len(file.Baseline.Records),
		file.Variant.Key,
		len(file.Variant.Records),
	)
	out.WriteString("| Measure | Baseline | Variant | Δ mean | Welch t | df | Significant | Note |\n")
	out.WriteString("|---|---|---|---|---|---|---|---|\n")
	for index := range file.Differences {
		out.WriteString(differenceRow(&file.Differences[index]) + "\n")
	}
	writeAbCompare(&out, compare, keys)
	writeAbNotes(&out, file.Notes)
	return out.String()
}

// runMeasures skips null values and counts booleans as 0/1.
func runMeasures(evalFile *evals.EvalFile, runId string) map[string]float64 {
	measures := make(map[string]float64)
	for _, value := range evalFile.MetricValues {
		number, isNumber := value.Number()
		if value.RunId == runId && isNumber {
			measures[value.MetricId] = number
		}
	}
	return measures
}

func smokeMeasures(result *ArmResult) map[string]float64 {
	measures := make(map[string]float64)
	if result.CostUsd != nil {
		measures[metricCostUsd] = *result.CostUsd
	}
	if result.OutputTokens != nil {
		measures[metricOutputTokens] = float64(*result.OutputTokens)
	}
	if result.PermissionDenials != nil {
		measures[metricPermissionDenials] = float64(*result.PermissionDenials)
	}

	isTimed := !result.StartedAt.IsZero() && !result.EndedAt.IsZero()
	if isTimed {
		measures[metricWallSeconds] = result.EndedAt.Sub(result.StartedAt).Seconds()
	}
	if result.Smoke == nil {
		return measures
	}

	measures[metricSmokePass] = 0
	if result.Smoke.Verdict == statusPass {
		measures[metricSmokePass] = 1
	}

	coverage := result.Smoke.Coverage
	if coverage.AnchorsDeclared > 0 {
		measures[measureAnchorsRatio] = float64(coverage.AnchorsHit) / float64(coverage.AnchorsDeclared)
	}
	if coverage.ChecksDeclared > 0 {
		measures[measureChecksRatio] = float64(coverage.ChecksPassed) / float64(coverage.ChecksDeclared)
	}
	return measures
}

func smokeRecordPath(skill, version string) (string, error) {
	root, err := SmokeRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, skill, version, runsDir, smokeArmLabel, resultFile), nil
}

// spreadCell is mean ± standard deviation (n) in the measure's display unit.
func spreadCell(count int, mean float64, measure string, stdDev float64) string {
	return fmt.Sprintf("%s ± %s (%d)", formatMetric(measure, mean), formatMetric(measure, stdDev), count)
}

func writeAbCompare(out *strings.Builder, compare *CompareReport, keys map[string]string) {
	if compare == nil {
		return
	}

	out.WriteString("\n## Compare judge\n\n")
	fmt.Fprintf(out, "Ranking, best first: %s\n\n", strings.Join(compare.Ranking, ", "))
	for _, arm := range compare.Arms {
		fmt.Fprintf(out, "- `%s` (%s): %s\n", arm.Label, keys[arm.Label], cellText(arm.Summary))
		for _, defect := range arm.Defects {
			fmt.Fprintf(out, "  - %s `%s`: %s\n", defect.Severity, defect.File, cellText(defect.Claim))
		}
	}
}

func writeAbFile(abDir string, file *evals.AbFile) error {
	return writeJsonFile(filepath.Join(abDir, evals.AbFileName), file)
}

func writeAbNotes(out *strings.Builder, notes []string) {
	if len(notes) == 0 {
		return
	}

	out.WriteString("\n## Notes\n\n")
	for _, note := range notes {
		fmt.Fprintf(out, "- %s\n", note)
	}
}

// writeAbSides runs an eval run's definition, token and window checks before writing a side-by-side run.json.
func writeAbSides(abDir string, file *evals.AbFile, request *AbInitRequest, variant *Variant) error {
	if variant.isVersion() {
		file.Baseline.Key, file.Variant.Key = variant.versions()
		if err := os.MkdirAll(abDir, 0o755); err != nil {
			return fmt.Errorf("writeAbSides: %w", err)
		}
		return nil
	}

	def, skillDir, err := loadEvalDefinition(request.Skill, request.SkillsRoot)
	if err != nil {
		return err
	}

	golden, err := newManifestGolden(def, "", skillDir)
	if err != nil {
		return err
	}

	if err := checkToken(request.Token); err != nil {
		return err
	}

	base := newBaseConfig(def)
	varied, err := variant.apply(base)
	if err != nil {
		return err
	}

	spec := &runSpec{
		configs:  []*armConfig{base, varied},
		date:     request.Date,
		def:      def,
		golden:   golden,
		replicas: replicaCount(def, request.Replicas),
		runDir:   abDir,
		skill:    request.Skill,
		skillDir: skillDir,
		token:    request.Token,
	}
	if err := checkWindow(&def.Budgets, len(spec.configs)*spec.replicas); err != nil {
		return err
	}

	file.Baseline.Key, file.Variant.Key = base.key, varied.key
	return spec.write()
}

// AbCollect reads a version variant's stored records instead of the A/B dir's own runs.
func AbCollect(abDir string) error {
	file, err := loadAbFile(abDir)
	if err != nil {
		return err
	}

	variant, err := ParseVariant(file.Descriptor)
	if err != nil {
		return err
	}

	file.Baseline.Records = []evals.AbRecord{}
	file.Variant.Records = []evals.AbRecord{}
	file.Notes = []string{}
	switch {
	case !variant.isVersion():
		err = collectRunRecords(abDir, file)
	case file.Layer == LayerSmoke:
		err = collectSmokeRecords(file)
	default:
		err = collectEvalRecords(file, filepath.Dir(abDir))
	}
	if err != nil {
		return err
	}

	isEmptySide := len(file.Baseline.Records) == 0 || len(file.Variant.Records) == 0
	if isEmptySide {
		return fmt.Errorf(
			"AbCollect: %s: baseline %d records, variant %d — nothing to compare: %s",
			abDir,
			len(file.Baseline.Records),
			len(file.Variant.Records),
			strings.Join(file.Notes, "; "),
		)
	}
	return writeAbFile(abDir, file)
}

func AbReport(abDir string) error {
	file, err := loadAbFile(abDir)
	if err != nil {
		return err
	}

	var stats evals.Stats
	if err := readJsonFile(filepath.Join(abDir, statsFile), &stats); err != nil {
		return err
	}

	file.Differences = abDifferences(file, &stats)
	if err := writeAbFile(abDir, file); err != nil {
		return err
	}

	compare, err := loadCompare(abDir)
	if err != nil {
		return err
	}

	keys, err := labelKeys(abDir, compare)
	if err != nil {
		return err
	}

	report := renderAbReport(compare, file, keys)
	if err := os.WriteFile(filepath.Join(abDir, reportFile), []byte(report), 0o644); err != nil {
		return fmt.Errorf("AbReport: %w", err)
	}

	return nil
}

// InitAb creates evals/<skill>/ab-<date>-<hash6>[-<n>].
func InitAb(request *AbInitRequest) (string, error) {
	variant, err := ParseVariant(request.Variant)
	if err != nil {
		return "", err
	}

	isSmokeRun := !variant.isVersion() && request.Layer == LayerSmoke
	if isSmokeRun {
		return "", fmt.Errorf("InitAb: %s: the smoke layer keeps one record per deployed version, only version: variants compare: %w", request.Variant, ErrNotRunnable)
	}

	if !skillNamePattern.MatchString(request.Skill) {
		return "", fmt.Errorf("InitAb: Invalid skill name %q", request.Skill)
	}

	base := filepath.Join(request.EvalsRoot, request.Skill, abDirPrefix+request.Date+"-"+descriptorHash(request.Variant))
	abDir, err := uniqueRunDir(base)
	if err != nil {
		return "", err
	}

	file := &evals.AbFile{
		Baseline:      evals.AbSide{Records: []evals.AbRecord{}},
		Date:          request.Date,
		Descriptor:    request.Variant,
		Differences:   []evals.AbDifference{},
		Layer:         request.Layer,
		Notes:         []string{},
		SchemaVersion: abSchemaVersion,
		Skill:         request.Skill,
		Variant:       evals.AbSide{Records: []evals.AbRecord{}},
	}
	if err := writeAbSides(abDir, file, request, variant); err != nil {
		return "", err
	}

	if err := writeAbFile(abDir, file); err != nil {
		return "", err
	}
	return abDir, nil
}

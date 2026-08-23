// Package evals loads /skillroutine-eval JSON output — contract:
// skills/skillroutine/skillroutine-eval/reference/schema.json (v2.0; v1.1 files
// without schemaVersion still load and display their stored axes) — from
// evals/<skill>-<date>/ (nightly routine) or evals/<skill>-<hex>/ (matrix
// mode) directories, each holding eval.json plus an optional deltas.json.
package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

// Delta is one deltas.json row — per-axis percentage delta of an arm vs its
// counterpart, computed by routines/_lib/matrix.sh.
type Delta struct {
	Arm       string  `json:"arm"`
	Axis      string  `json:"axis"`
	DeltaPct  float64 `json:"delta_pct"`
	Dimension string  `json:"dimension"`
	N         int     `json:"n"`
}

type Eval struct {
	Date  string `json:"date"`
	Notes string `json:"notes"`
	Skill string `json:"skill"`
}

// EvalDir is one evals/<skill>-<suffix>/ results directory.
type EvalDir struct {
	Deltas []Delta
	Dir    string // basename under the evals root, e.g. concept-2026-08-18
	Eval   EvalFile
	Files  []string // dir-relative artifact files (top level, runs/, skills/)
}

type EvalFile struct {
	sourcePath string

	Eval          Eval          `json:"eval"`
	Metrics       []Metric      `json:"metrics"`
	MetricValues  []MetricValue `json:"metricValues"`
	Probes        []Probe       `json:"probes"`
	Rubric        []RubricRule  `json:"rubric"`
	Runs          []Run         `json:"runs"`
	SchemaVersion string        `json:"schemaVersion"`
	Scores        []ScoreCell   `json:"scores"`
	SharedTotals  []Total       `json:"sharedTotals"`
	Totals        []Total       `json:"totals"`
}

// IsLegacy reports a pre-2.0 file (no schemaVersion) — displayed with its
// stored axes rather than relabeled.
func (e *EvalFile) IsLegacy() bool {
	return e.SchemaVersion == ""
}

func (e *EvalFile) SourcePath() string {
	return e.sourcePath
}

type Manifest struct {
	Inputs  []string      `json:"inputs"`
	Output  string        `json:"output"`
	Runs    []ManifestRun `json:"runs"`
	Skill   string        `json:"skill"`
	SkillMd string        `json:"skillMd"`
}

type ManifestModel struct {
	Id string `json:"id"`
}

type ManifestRun struct {
	Id     string        `json:"id"`
	Model  ManifestModel `json:"model"`
	Output string        `json:"output"`
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
	Id      string  `json:"id"`
	Model   Model   `json:"model"`
	Variant Variant `json:"variant"`
}

// ScoreCell is one run × rule score; justification and evidence are present
// for every non-+1 cell (schema v2 contract).
type ScoreCell struct {
	Evidence      []string `json:"evidence"`
	Justification string   `json:"justification"`
	RuleId        string   `json:"ruleId"`
	RunId         string   `json:"runId"`
	Score         int      `json:"score"`
	Source        string   `json:"source"`
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

// artifactFiles enumerates the dir-relative files the artifact route may
// serve: top-level files plus the runs/ and skills/ artifacts.
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

	evalDir := &EvalDir{
		Deltas: deltas,
		Dir:    name,
		Eval:   file,
		Files:  artifactFiles(path),
	}
	return evalDir, loadErrors
}

// LoadForSkill reads every evals/<skill>-<date>/ (routine) or
// evals/<skill>-<hex>/ (matrix mode) directory, newest eval date first.
// eval.json is required; deltas.json is optional. Malformed files land in the
// errors list, never fail the page (sessions LoadErrors pattern).
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

	var dirs []EvalDir
	var loadErrors []string
	for _, entry := range entries {
		if !entry.IsDir() || !dirPattern.MatchString(entry.Name()) {
			continue
		}

		evalDir, dirErrors := loadEvalDir(filepath.Join(dir, entry.Name()), entry.Name())
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

// ManifestStub renders a pre-filled /skillroutine-eval manifest — the skill's
// example files as inputs, run entries as placeholders the user completes
// (D28; contract: skills/skillroutine/skillroutine-eval/reference/manifest.schema.json).
func ManifestStub(evalsDir string, examplePaths []string, skillMdPath, skillName string) (string, error) {
	run := ManifestRun{
		Id:     "run-1",
		Model:  ManifestModel{Id: "<model-id>"},
		Output: "<path-to-run-output>",
	}
	manifest := Manifest{
		Inputs:  examplePaths,
		Output:  filepath.Join(evalsDir, skillName+"-<date>", "eval.json"),
		Runs:    []ManifestRun{run},
		Skill:   skillName,
		SkillMd: skillMdPath,
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("ManifestStub: %w", err)
	}
	return string(data), nil
}

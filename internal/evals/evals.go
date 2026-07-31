// Package evals loads /skillroutine-eval JSON output — contract:
// skills/skillroutine/skillroutine-eval/reference/schema.json (v1.1) — from evals/<skill>/*.json.
package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type Eval struct {
	Date  string `json:"date"`
	Notes string `json:"notes"`
	Skill string `json:"skill"`
}

type EvalFile struct {
	sourcePath string

	Eval   Eval    `json:"eval"`
	Runs   []Run   `json:"runs"`
	Totals []Total `json:"totals"`
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

type Model struct {
	Id     string `json:"id"`
	Effort string `json:"effort"`
	Mode   string `json:"mode"`
}

type Run struct {
	Id    string `json:"id"`
	Model Model  `json:"model"`
}

type Total struct {
	Axis  string  `json:"axis"`
	Max   int     `json:"max"`
	Pct   float64 `json:"pct"`
	Raw   int     `json:"raw"`
	RunId string  `json:"runId"`
}

// LoadForSkill reads every evals/<skill>/*.json, newest eval date first.
// Malformed files land in the errors list, never fail the page (sessions
// LoadErrors pattern).
func LoadForSkill(dir, skill string) ([]EvalFile, []string) {
	pattern := filepath.Join(dir, skill, "*.json")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		loadErrors := []string{fmt.Sprintf("%s: %v", pattern, err)}
		return nil, loadErrors
	}

	var files []EvalFile
	var loadErrors []string
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			loadErrors = append(loadErrors, fmt.Sprintf("%s: %v", filepath.Base(path), err))
			continue
		}

		var file EvalFile
		if err := json.Unmarshal(data, &file); err != nil {
			loadErrors = append(loadErrors, fmt.Sprintf("%s: %v", filepath.Base(path), err))
			continue
		}
		file.sourcePath = path
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Eval.Date > files[j].Eval.Date
	})
	return files, loadErrors
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
		Output:  filepath.Join(evalsDir, skillName, "eval-<date>.json"),
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

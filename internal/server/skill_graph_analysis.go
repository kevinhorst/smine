package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/server/respond"
	"github.com/kevinhorst/smine/internal/shell"
	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skillmodel"
)

const (
	tmplSkillGraphDiffs    = "_skill_graph_diffs.html"
	tmplSkillGraphFindings = "_skill_graph_findings.html"
)

const batteryTimeout = 2 * time.Minute

const diffContextLines = 2

// graphDiffView is the model diff, and a note when no diff base exists.
type graphDiffView struct {
	ModelDiff []diffLine
	Note      string
}

// appendFile adds one model file's diff under its header line.
func (v *graphDiffView) appendFile(path string, lines []diffLine) {
	if len(lines) == 0 {
		return
	}

	header := diffLine{Kind: diffFile, Text: path}
	v.ModelDiff = append(v.ModelDiff, header)
	v.ModelDiff = append(v.ModelDiff, lines...)
}

type graphFindingsView struct {
	Error        string
	Errors       []skillmodel.Finding
	FindingsJson template.JS
	WarningCount int
}

func (s *Server) handleSkillGraphDiff(w http.ResponseWriter, r *http.Request) {
	skill := s.findSkill(w, r)
	if skill == nil {
		return
	}

	view, err := headDiffs(r.Context(), skill.Path)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	s.renderFragment(w, tmplSkillGraphDiffs, view)
}

func (s *Server) handleSkillGraphFindings(w http.ResponseWriter, r *http.Request) {
	skill := s.findSkill(w, r)
	if skill == nil {
		return
	}

	view := &graphFindingsView{}
	findings, err := loadedFindings(r.Context(), skill.Path)
	if err != nil {
		view.Error = err.Error()
		s.renderFragment(w, tmplSkillGraphFindings, view)
		return
	}

	blob, err := json.Marshal(findings)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	blocking, warnings := splitFindings(findings)
	view.Errors = blocking
	view.FindingsJson = template.JS(blob)
	view.WarningCount = len(warnings)
	s.renderFragment(w, tmplSkillGraphFindings, view)
}

// headDiffs diffs the skill's model files in the working tree against HEAD.
func headDiffs(ctx context.Context, dir string) (*graphDiffView, error) {
	view := &graphDiffView{}
	if _, err := shell.Run(ctx, dir, "git", "rev-parse", "--verify", "HEAD"); err != nil {
		view.Note = "no git HEAD for this skill dir — no diff base"
		return view, nil
	}

	paths, err := modelFilePaths(ctx, dir)
	if err != nil {
		return nil, err
	}

	for _, path := range paths {
		lines, err := headFileDiff(ctx, dir, path)
		if err != nil {
			return nil, err
		}
		view.appendFile(path, lines)
	}
	return view, nil
}

func headFileDiff(ctx context.Context, dir, path string) ([]diffLine, error) {
	before, err := shell.Run(ctx, dir, "git", "show", "HEAD:./"+path)
	if err != nil {
		before = ""
	}

	after, err := readOptional(filepath.Join(dir, path))
	if err != nil {
		return nil, err
	}

	return compactDiff(diffLines(before, after), diffContextLines), nil
}

func loadedFindings(ctx context.Context, dir string) ([]skillmodel.Finding, error) {
	graph, err := skillgraph.Load(dir)
	if err != nil {
		return nil, fmt.Errorf("loadedFindings: %w", err)
	}

	model, err := skillmodel.Derive(graph)
	if err != nil {
		return nil, fmt.Errorf("loadedFindings: %w", err)
	}

	return runGraphBattery(ctx, model)
}

func modelFilePaths(ctx context.Context, dir string) ([]string, error) {
	seen := map[string]bool{skillgraph.ModelFile(filepath.Base(dir)): true}
	current, err := filepath.Glob(filepath.Join(dir, skillgraph.DirPayloads, "*.md"))
	if err != nil {
		return nil, fmt.Errorf("modelFilePaths: %w", err)
	}

	for _, path := range current {
		seen[filepath.ToSlash(filepath.Join(skillgraph.DirPayloads, filepath.Base(path)))] = true
	}
	tracked, err := shell.Run(ctx, dir, "git", "ls-tree", "--name-only", "HEAD", skillgraph.DirPayloads+"/")
	if err != nil {
		return nil, fmt.Errorf("modelFilePaths: %w", err)
	}

	for path := range strings.FieldsSeq(tracked) {
		seen[path] = true
	}
	return slices.Sorted(maps.Keys(seen)), nil
}

// readOptional reads a file; a missing file reads as empty.
func readOptional(path string) (string, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}

	if err != nil {
		return "", fmt.Errorf("readOptional: %w", err)
	}

	return string(content), nil
}

func runGraphBattery(ctx context.Context, model *skillmodel.Model) ([]skillmodel.Finding, error) {
	batteryCtx, cancel := context.WithTimeout(ctx, batteryTimeout)
	defer cancel()
	findings, err := skillmodel.RunBattery(batteryCtx, model)
	if err != nil {
		return nil, fmt.Errorf("runGraphBattery: %w", err)
	}

	return findings, nil
}

func splitFindings(findings []skillmodel.Finding) ([]skillmodel.Finding, []skillmodel.Finding) {
	var blocking, warnings []skillmodel.Finding
	for _, finding := range findings {
		switch finding.Severity {
		case skillmodel.SeverityError:
			blocking = append(blocking, finding)
		case skillmodel.SeverityWarning:
			warnings = append(warnings, finding)
		}
	}
	return blocking, warnings
}

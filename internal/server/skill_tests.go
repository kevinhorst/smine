package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/kevinhorst/smine/internal/evals"
	"github.com/kevinhorst/smine/internal/server/respond"
	"github.com/kevinhorst/smine/internal/skills"
)

const tmplSkillTests = "skill_tests.html"

// evalAxisView is one axis of one eval: its score matrix and the non-+1
// justification rows.
type evalAxisView struct {
	Axis           string
	Justifications []evalJustificationRow
	Rows           []evalRuleRow
}

// evalJustificationRow is one non-+1 cell with its reasoning and evidence.
type evalJustificationRow struct {
	Evidence      string
	Justification string
	RuleId        string
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

// evalRuleRow is one rubric rule with its per-run score cells (run order =
// legend order; a run without a cell for this rule renders as absent).
type evalRuleRow struct {
	Cells []evalScoreCell
	Rule  evals.RubricRule
}

// evalRunLabel pairs a run with its short display label (R1…Rn).
type evalRunLabel struct {
	Label string
	Run   evals.Run
}

// evalScoreCell is one rendered matrix cell; Class picks the score color,
// Title carries the justification as tooltip.
type evalScoreCell struct {
	Class string
	Text  string
	Title string
}

// evalView is one eval directory prepared for rendering.
type evalView struct {
	Axes     []evalAxisView
	Dir      evals.EvalDir
	Rankings []evalRankingRow
	Runs     []evalRunLabel
}

type skillTestsPage struct {
	EvalErrors   []string
	Evals        []evalView
	File         string
	FileContent  string
	ManifestStub string
	Page         string
	Skill        *skills.Skill
	Tab          string
	Title        string
}

func buildAxisViews(file evals.EvalFile, runs []evalRunLabel, cellByRuleAndRun map[string]evals.ScoreCell) []evalAxisView {
	var axes []evalAxisView
	for _, rule := range file.Rubric {
		if n := len(axes); n == 0 || axes[n-1].Axis != rule.Axis {
			axes = append(axes, evalAxisView{Axis: rule.Axis})
		}
		axis := &axes[len(axes)-1]

		row := evalRuleRow{Rule: rule}
		for _, runLabel := range runs {
			score, ok := cellByRuleAndRun[rule.Id+"\x00"+runLabel.Run.Id]
			row.Cells = append(row.Cells, renderScoreCell(score, ok))
			if ok && score.Score != 1 {
				justification := evalJustificationRow{
					Evidence:      strings.Join(score.Evidence, "; "),
					Justification: score.Justification,
					RuleId:        rule.Id,
					RunLabel:      runLabel.Label,
					Score:         score.Score,
					Source:        score.Source,
				}
				axis.Justifications = append(axis.Justifications, justification)
			}
		}
		axis.Rows = append(axis.Rows, row)
	}
	return axes
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
	return view
}

func buildEvalViews(evalDirs []evals.EvalDir) []evalView {
	var views []evalView
	for _, evalDir := range evalDirs {
		views = append(views, buildEvalView(evalDir))
	}
	return views
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

func renderScoreCell(score evals.ScoreCell, ok bool) evalScoreCell {
	if !ok {
		absent := evalScoreCell{Class: "score-absent", Text: "—", Title: "no cell — rule not in this run's variant"}
		return absent
	}

	cell := evalScoreCell{Title: score.Justification}
	switch score.Score {
	case 1:
		cell.Class, cell.Text = "score-pos", "+1"
	case 0:
		cell.Class, cell.Text = "score-zero", "0"
	case -1:
		cell.Class, cell.Text = "score-neg", "−1"
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

	data := s.skillTestsData(skill)
	data.Title = "Skill — " + skill.Name + " — Tests"
	s.renderFragment(w, tmplSkillTests, data)
}

func (s *Server) handleSkillTestsFile(w http.ResponseWriter, r *http.Request) {
	skill := s.findSkill(w, r)
	if skill == nil {
		return
	}

	// Both parameters must match loader-enumerated values by string equality;
	// user input is never joined into a path on its own (concept security rule).
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
		Tab:         "tests",
		Title:       "Skill — " + skill.Name + " — " + requestedFile,
	}
	s.renderFragment(w, tmplSkillTests, data)
}

func (s *Server) skillTestsData(skill *skills.Skill) skillTestsPage {
	data := skillTestsPage{Page: pageSkills, Skill: skill, Tab: "tests"}

	evalDirs, evalErrors := evals.LoadForSkill(s.evalsDir, skill.Name)
	data.EvalErrors = evalErrors
	data.Evals = buildEvalViews(evalDirs)

	var examplePaths []string
	for _, file := range exampleFiles(s.examplesDir, skill.Name) {
		examplePaths = append(examplePaths, filepath.Join(s.examplesDir, skill.Name, file))
	}
	stub, err := evals.ManifestStub(s.evalsDir, examplePaths, filepath.Join(skill.Path, "SKILL.md"), skill.Name)
	if err != nil {
		data.EvalErrors = append(data.EvalErrors, err.Error())
	}
	data.ManifestStub = stub
	return data
}

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/evals"
	"github.com/kevinhorst/smine/internal/skillgraph"
)

// skillsTestEnv builds a repo skills root with the skill demo and the skill
// peek aliased pk, a home skills root with peek and its pk alias copy, an
// evals dir, an examples dir, and a sessions store with invoked_skills —
// peek is invoked once under its name and once, later, as pk.
func skillsTestEnv(t *testing.T) *Server {
	t.Helper()

	skillsRepo := t.TempDir()
	skillsHome := filepath.Join(t.TempDir(), "home")
	writeAliasedRepoSkill(t, skillsRepo, "1.0", demoGraphChange)
	writeAliasedHomeSkill(t, skillsHome, "1.0")
	skillDir := filepath.Join(skillsRepo, "demo")
	manifest := "---\nname: demo\ndescription: demo skill\nversion: 1.1\nallowed-tools: Bash(jq *), Read\n---\n\n# Demo\n"
	writeSkillModel(t, skillDir, manifest, `[{"version": "1.1", "date": "2026-07-10", "text": "added X"}]`)
	require.NoError(t, os.MkdirAll(filepath.Join(skillDir, "eval", "golden"), 0o755))
	definition := &evals.Definition{
		Budgets:       evals.DefinitionBudgets{ArmTimeoutS: 10800, ArmUsd: 30, WindowUsd: 150},
		Goldens:       []evals.DefinitionGolden{{File: "eval/golden/g1.json", Id: "g1", Input: "demo range"}},
		SchemaVersion: "4.0",
		ScoringFile:   "eval/golden/scoring.json",
		Skill:         "demo",
	}
	definitionContent, err := json.MarshalIndent(definition, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "eval", "definition.json"), definitionContent, 0o644))
	scoring := `{"judgeGuide": "match by root cause", "minutes": {"BLOCKER": 120, "NIT": 10}, "escapeMultiplier": {"BLOCKER": 5, "NIT": 1}, "triageLine": "NIT", "fixFactor": {},"thresholds": {"review_score": 0.6}}`
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "eval", "golden", "scoring.json"), []byte(scoring), 0o644))
	golden := `{"range": {"baseLabel": "1.1", "headLabel": "1.2"}, "findings": [{"severity": "BLOCKER"}, {"severity": "NIT"}],
	  "production": {"contributions": [{"version": 1, "findings": 2, "newFindings": 2, "dir": "demo-2026-07-11", "source": "unseeded"}], "totals": {}}}`
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "eval", "golden", "g1.json"), []byte(golden), 0o644))

	evalsDir := t.TempDir()
	evalSkillDir := filepath.Join(evalsDir, "demo-2026-07-11")
	require.NoError(t, os.MkdirAll(filepath.Join(evalSkillDir, "runs"), 0o755))
	evalContent := `{
		"schemaVersion": "2.0",
		"eval": {"skill": "demo", "date": "2026-07-11", "notes": "baseline"},
		"runs": [{"id": "run-1", "model": {"id": "claude-fable-5", "effort": "high"}}],
		"rubric": [{"id": "SKILL-DEMO-A-001", "axis": "self", "phase": "step", "rule": "Do the thing.", "source": "skills/demo/SKILL.md:3"}],
		"scores": [{"ruleId": "SKILL-DEMO-A-001", "runId": "run-1", "score": 0, "source": "agent", "justification": "not demonstrated", "evidence": ["line 4"]}],
		"probes": [{"name": "context-injection", "ruleIds": ["SKILL-DEMO-A-001"], "result": "identical sets"}],
		"totals": [{"runId": "run-1", "axis": "self", "raw": 22, "max": 25, "pct": 88}]
	}`
	require.NoError(t, os.WriteFile(filepath.Join(evalSkillDir, "eval.json"), []byte(evalContent), 0o644))
	deltasContent := `[{"axis": "self", "dimension": "context", "arm": "off", "delta_pct": 1.4, "n": 4}]`
	require.NoError(t, os.WriteFile(filepath.Join(evalSkillDir, "deltas.json"), []byte(deltasContent), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(evalSkillDir, "runs", "run-1.md"), []byte("run output\n"), 0o644))

	nestedRunDir := filepath.Join(evalsDir, "demo", "2026-07-12-abc123")
	require.NoError(t, os.MkdirAll(filepath.Join(nestedRunDir, "runs"), 0o755))
	nestedEval := `{
		"schemaVersion": "2.0",
		"eval": {"skill": "demo", "date": "2026-07-12", "notes": "nested run"},
		"runs": [{"id": "cell-1", "model": {"id": "claude-fable-5", "effort": ""}}],
		"golden": {"goldenId": "g1", "goldenFile": "eval/golden/g1.json", "goldenVersion": 1, "total": 2, "nullMinutes": 610, "oracleMinutes": 130,
		  "perRun": [{"runId": "cell-1", "matched": [{"goldenId": "G-2", "findingId": "f-2"}], "missed": ["G-1"], "falsePositives": [{"findingId": "f-9", "reason": "wrong"}], "misclassified": [],
		    "severities": [
		      {"severity": "BLOCKER", "entries": 1, "found": 0, "missed": 1, "falsePositives": 0, "foundMinutes": 0, "escapedMinutes": 600, "falsePositiveMinutes": 0, "outcomes": {"missed": {"count": 1, "minutes": 600}}},
		      {"severity": "NIT", "entries": 1, "found": 1, "missed": 0, "falsePositives": 1, "foundMinutes": 10, "escapedMinutes": 0, "falsePositiveMinutes": 10, "outcomes": {"atPrice": {"count": 1, "minutes": 10}}}
		    ]}]},
		"metricValues": [
		  {"metricId": "review_score", "runId": "cell-1", "value": 0.5},
		  {"metricId": "lost_minutes", "runId": "cell-1", "value": 240},
		  {"metricId": "escaped_minutes", "runId": "cell-1", "value": 300},
		  {"metricId": "false_positive_minutes", "runId": "cell-1", "value": 10}
		],
		"rubric": [], "scores": [], "probes": [], "totals": []
	}`
	require.NoError(t, os.WriteFile(filepath.Join(nestedRunDir, "eval.json"), []byte(nestedEval), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(nestedRunDir, "runs", "cell-1.md"), []byte("nested cell output\n"), 0o644))
	interpretation := "# Interpretation\n\n- breadth is fine\n- depth is the gap\n\nWorker pins: none.\n"
	require.NoError(t, os.WriteFile(filepath.Join(nestedRunDir, "interpretation.md"), []byte(interpretation), 0o644))

	examplesDir := t.TempDir()
	exampleSkillDir := filepath.Join(examplesDir, "demo")
	require.NoError(t, os.MkdirAll(exampleSkillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(exampleSkillDir, "input.md"), []byte("example\n"), 0o644))

	sessionsDir := t.TempDir()
	weekDir := filepath.Join(sessionsDir, "personal", "2026-W28")
	require.NoError(t, os.MkdirAll(weekDir, 0o755))
	record := `{"id": "00000000-0000-0000-0000-000000000001", "agent": "claude", "title": "s1", "repo": "personal",
		"week": "2026-W28", "lastActive": "2026-07-10T10:00:00Z", "findings": [], "invoked_skills": ["demo"]}`
	require.NoError(t, os.WriteFile(filepath.Join(weekDir, "00000000-0000-0000-0000-000000000001.json"), []byte(record), 0o644))
	ownNameRecord := `{"id": "00000000-0000-0000-0000-000000000002", "agent": "claude", "title": "s2", "repo": "personal",
		"week": "2026-W28", "lastActive": "2026-07-09T10:00:00Z", "findings": [], "invoked_skills": ["peek"]}`
	require.NoError(t, os.WriteFile(filepath.Join(weekDir, "00000000-0000-0000-0000-000000000002.json"), []byte(ownNameRecord), 0o644))
	aliasRecord := `{"id": "00000000-0000-0000-0000-000000000003", "agent": "claude", "title": "s3", "repo": "personal",
		"week": "2026-W28", "lastActive": "2026-07-12T09:00:00Z", "findings": [], "invoked_skills": ["pk"]}`
	require.NoError(t, os.WriteFile(filepath.Join(weekDir, "00000000-0000-0000-0000-000000000003.json"), []byte(aliasRecord), 0o644))

	return newTestServer(t, &Options{
		EvalsDir:    evalsDir,
		ExamplesDir: examplesDir,
		SessionsDir: sessionsDir,
		SkillsHome:  skillsHome,
		SkillsRepo:  skillsRepo,
	})
}

func TestSkillDetailRendersAllBlocks(t *testing.T) {
	server := skillsTestEnv(t)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills/repo/demo", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()

	assert.Contains(t, body, "Version history")
	assert.Contains(t, body, `<details class="section skill-tools">`)
	assert.Contains(t, body, `<span class="tool-group-key">tools</span>`)
	assert.Contains(t, body, `<span class="tool-chip">Read</span>`)
	assert.Contains(t, body, `<span class="tool-group-key">jq</span>`)
	assert.Contains(t, body, "added X")
	assert.Contains(t, body, "Examples")
	assert.Contains(t, body, "input.md")
	assert.Contains(t, body, "/scripts/skills/repo/demo/tests")
	assert.NotContains(t, body, "Eval runs")
	assert.NotContains(t, body, "eval manifest stub")
	assert.Contains(t, body, "Invocations (1)")
	assert.Contains(t, body, "/sessions/personal/2026-W28?session=00000000-0000-0000-0000-000000000001")
	assert.Contains(t, body, `id="skill-invocations"`)
	assert.Contains(t, body, `data-stamp="2026-07-10T10:00:00Z"`)
	assert.Contains(t, body, "run-band")

	aliased := httptest.NewRecorder()
	server.Handler().ServeHTTP(aliased, httptest.NewRequest(http.MethodGet, "/scripts/skills/repo/peek", nil))
	require.Equal(t, http.StatusOK, aliased.Code)
	aliasedBody := aliased.Body.String()

	t.Run("alias-badge-beside-title", func(t *testing.T) {
		assert.Contains(t, aliasedBody, "<h1>/peek</h1>\n  <span class=\"badge badge-dim\">/pk</span>")
		assert.NotContains(t, body, `<span class="badge badge-dim">/`)
	})

	t.Run("alias-invocations-listed", func(t *testing.T) {
		assert.Contains(t, aliasedBody, "Invocations (2)")
		assert.Contains(t, aliasedBody, "session=00000000-0000-0000-0000-000000000002")
		assert.Contains(t, aliasedBody, "session=00000000-0000-0000-0000-000000000003")
	})
}

func TestSkillTestsTabRendersEvaluation(t *testing.T) {
	server := skillsTestEnv(t)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills/repo/demo/tests", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()

	// Default view features the newest dir; the production run leaves the history for the golden card.
	assert.Contains(t, body, "2026-07-12 — "+filepath.Join("demo", "2026-07-12-abc123"), "newest run is featured")
	assert.Contains(t, body, "Run history")
	assert.Contains(t, body, "eval-hero", "featured run renders the combined hero card")
	assert.Contains(t, body, "<h3>Result</h3>", "per-arm values render in the Result table")
	assert.Contains(t, body, "scope-select", "run selector renders at the top")
	assert.Contains(t, body, `<optgroup label="golden production"><option value="demo-2026-07-11">`, "production run in its own selector group")
	assert.Contains(t, body, `<a href="?run=demo-2026-07-11">demo-2026-07-11</a>`, "golden card links the production run")
	assert.NotContains(t, body, "<td>2026-07-11</td>", "production run not in the history table")
	assert.Contains(t, body, "card-golden")
	assert.Contains(t, body, "Golden g1 v1")
	assert.Contains(t, body, "2 findings, oracle 130 min")
	assert.Contains(t, body, "range 1.1 → 1.2")
	assert.Contains(t, body, "eval-bullets")
	assert.Contains(t, body, "breadth is fine")
	assert.NotContains(t, body, "Worker pins", "paragraphs stay in the file")
	assert.Contains(t, body, "full interpretation")
	assert.Contains(t, body, "full notes", "the notes disclosure moved under the heading")
	assert.NotContains(t, body, "eval-notes-card")
	assert.NotContains(t, body, "eff-scatter")
	assert.Contains(t, body, "Test definition")
	assert.Contains(t, body, "Review score (share of the savable minutes captured) ≥ 60%")
	assert.NotContains(t, body, "Minutes per severity", "the price list moved into the key card")
	assert.Contains(t, body, "minutes-bar")
	assert.Contains(t, body, "<span>R1 minutes:</span>")
	assert.Contains(t, body, "escaped 600</span>")
	assert.Contains(t, body, "found 10</span>")
	assert.Contains(t, body, "<span>= 620</span>", "escaped 600 + false positives 10 + found 10")
	assert.Contains(t, body, `<span class="eval-tile-label">review score</span>`, "one run — no mean prefix")
	assert.Contains(t, body, `<span class="eval-tile-value">240 min</span><span class="eval-tile-label">lost minutes</span>`, "a tile carries its unit")
	assert.Contains(t, body, `<span class="eval-tile-value">1 · 10 min</span>`, "false positives count and minutes in one tile")
	assert.Contains(t, body, "Breakdown · R1")
	assert.Contains(t, body, `<td>BLOCKER</td><td class="meta">120 · 600 (×5) · 0 (×0)</td><td>1</td><td class="cell-found">0 (0)</td><td class="cell-missed">1 (600)</td><td class="cell-fp">0 (0)</td><td>120</td><td>480</td>`)
	assert.Contains(t, body, `<tr class="breakdown-total"><td>Σ</td><td></td><td>2</td><td class="cell-found">1 (10)</td><td class="cell-missed">1 (600)</td><td class="cell-fp">1 (10)</td><td>130</td><td>490</td></tr>`)
	assert.NotContains(t, body, "breakdown-notes", "a run priced at plain count × price has no notes")
	assert.Contains(t, body, "<th>Found (min)</th><th>Escaped (min)</th><th>False positives (min)</th>", "counts and minutes share a column")
	assert.Contains(t, body, `<span class="eval-chip eval-chip-env"`, "config widget renders under the heading")
	assert.Contains(t, body, "triage line")
	assert.Contains(t, body, "review score -2% = 1 − lost 490 ÷ savable 480 (null 610 − oracle 130)")
	assert.NotContains(t, body, "Minute prices")
	assert.Contains(t, body, "/skillroutine-eval demo")
	assert.Contains(t, body, "arm: $30")
	assert.NotContains(t, body, "cmd/skilleval/run.sh")
	assert.NotContains(t, body, "Arms:")

	// The ?run= drill-down features the older, rubric-scored dir in full.
	drill := httptest.NewRecorder()
	server.Handler().ServeHTTP(drill, httptest.NewRequest(http.MethodGet, "/scripts/skills/repo/demo/tests?run=demo-2026-07-11", nil))
	require.Equal(t, http.StatusOK, drill.Code)
	drillBody := drill.Body.String()
	assert.Contains(t, drillBody, "2026-07-11 — demo-2026-07-11")
	assert.Contains(t, drillBody, "Ranking")
	assert.Contains(t, drillBody, "22/25")
	assert.Contains(t, drillBody, "self axis")
	assert.Contains(t, drillBody, "score-matrix")
	assert.Contains(t, drillBody, `<td class="score-topic">A`, "topic header row")
	assert.Contains(t, drillBody, `<td class="score-rule" title="SKILL-DEMO-A-001">Do the thing.</td>`, "entry title with the id as hover")
	assert.Contains(t, drillBody, "justifications (1)")
	assert.Contains(t, drillBody, "not demonstrated")
	assert.Contains(t, drillBody, "context-injection")
	assert.Contains(t, drillBody, "Deltas")
	assert.Contains(t, drillBody, "&#43;1.4")
	assert.Contains(t, drillBody, "artifacts (3)")
	assert.Contains(t, drillBody, "<th>What it is</th>")
	assert.Contains(t, drillBody, "the folded, schema-validated eval record the page reads")
	assert.Contains(t, drillBody, "not in the artifact catalog", "runs/run-1.md is not a runner file")
}

func TestSkillTestsTabEmptyState(t *testing.T) {
	server := skillsTestEnv(t)
	writeRepoSkill(t, server.skillsRepo, "bare", "1.0")

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills/repo/bare/tests", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()

	assert.Contains(t, body, "No eval runs yet")
	assert.Contains(t, body, "/skillroutine-eval bare")
	assert.NotContains(t, body, "Test definition", "definition-less skill renders no card")
}

func TestSkillTestsFileServesArtifact(t *testing.T) {
	server := skillsTestEnv(t)

	response := httptest.NewRecorder()
	target := "/scripts/skills/repo/demo/tests/file?d=demo-2026-07-11&f=" + url.QueryEscape(filepath.Join("runs", "run-1.md"))
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "run output")

	denied := httptest.NewRecorder()
	target = "/scripts/skills/repo/demo/tests/file?d=demo-2026-07-11&f=" + url.QueryEscape("../../go.mod")
	server.Handler().ServeHTTP(denied, httptest.NewRequest(http.MethodGet, target, nil))
	assert.Equal(t, http.StatusNotFound, denied.Code)
}

func TestSkillTestsFileServesNestedArtifact(t *testing.T) {
	server := skillsTestEnv(t)

	response := httptest.NewRecorder()
	target := "/scripts/skills/repo/demo/tests/file?d=" + url.QueryEscape(filepath.Join("demo", "2026-07-12-abc123")) + "&f=" + url.QueryEscape(filepath.Join("runs", "cell-1.md"))
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "nested cell output")

	denied := httptest.NewRecorder()
	target = "/scripts/skills/repo/demo/tests/file?d=" + url.QueryEscape("demo/../..") + "&f=" + url.QueryEscape("go.mod")
	server.Handler().ServeHTTP(denied, httptest.NewRequest(http.MethodGet, target, nil))
	assert.Equal(t, http.StatusNotFound, denied.Code)
}

func TestSkillDetailBody(t *testing.T) {
	type testCase struct {
		_id                string
		_absentFrontmatter string
		_expectedBody      string
		_expectedCode      int
		_hasVersionHistory bool

		path     string
		response *httptest.ResponseRecorder
	}

	server := skillsTestEnv(t)
	modelManifest := "---\nname: modelled\ndescription: d\nversion: 1.0\n---\n\n# Modelled\n\nBody rendered from the model.\n"
	writeSkillModel(t, filepath.Join(server.skillsRepo, "modelled"), modelManifest, demoGraphChange)
	homeDir := filepath.Join(server.skillsHome, "deployed")
	require.NoError(t, os.MkdirAll(homeDir, 0o755))
	homeManifest := "---\nname: deployed\ndescription: d\nversion: 1.0\n---\n\n# Deployed\n\nBody read from the deployed file.\n"
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, "SKILL.md"), []byte(homeManifest), 0o644))
	writeRepoSkill(t, server.skillsRepo, "broken", "1.0")
	require.NoError(t, os.WriteFile(skillgraph.ModelPath(filepath.Join(server.skillsRepo, "broken")), []byte("not cue {"), 0o644))
	tests := make([]*testCase, 0)

	// repo-body-from-model
	tests = append(tests, &testCase{
		_id:                "repo-body-from-model",
		_absentFrontmatter: "name: modelled",
		_expectedBody:      "Body rendered from the model.",
		_expectedCode:      http.StatusOK,
		_hasVersionHistory: true,

		path:     "/scripts/skills/repo/modelled",
		response: httptest.NewRecorder(),
	})

	// home-body-from-file
	tests = append(tests, &testCase{
		_id:                "home-body-from-file",
		_absentFrontmatter: "name: deployed",
		_expectedBody:      "Body read from the deployed file.",
		_expectedCode:      http.StatusOK,
		_hasVersionHistory: false,

		path:     "/scripts/skills/home/deployed",
		response: httptest.NewRecorder(),
	})

	// repo-model-not-loading
	tests = append(tests, &testCase{
		_id:                "repo-model-not-loading",
		_absentFrontmatter: "name: broken",
		_expectedBody:      "ReadManifest",
		_expectedCode:      http.StatusInternalServerError,
		_hasVersionHistory: false,

		path:     "/scripts/skills/repo/broken",
		response: httptest.NewRecorder(),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			server.Handler().ServeHTTP(test.response, httptest.NewRequest(http.MethodGet, test.path, nil))
			assert.Equal(t, test._expectedCode, test.response.Code)
			assert.Contains(t, test.response.Body.String(), test._expectedBody)
			assert.Equal(t, test._hasVersionHistory, strings.Contains(test.response.Body.String(), "Version history"))
			assert.NotContains(t, test.response.Body.String(), test._absentFrontmatter)
		})
	}
}

func TestSkillFileServesExample(t *testing.T) {
	server := skillsTestEnv(t)

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills/repo/demo/file?f=input.md", nil))
	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "example")
}

func TestSkillsIndexInlineUsage(t *testing.T) {
	server := skillsTestEnv(t)
	writeRepoSkill(t, server.skillsRepo, "aaa-unused", "1.0")

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills", nil))
	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()

	// usage stats render inline in the one grouped view; no sort tabs exist.
	assert.Contains(t, body, "1 invocation")
	assert.Contains(t, body, "0 invocations")
	assert.Contains(t, body, "last used 2026-07-10")
	assert.NotContains(t, body, "sort=invocations")
	assert.NotContains(t, body, "sort=last-used")

	aliasedCard := skillCardOf(body, "/scripts/skills/repo/peek")
	plainCard := skillCardOf(body, "/scripts/skills/repo/demo")

	t.Run("alias-invocations-counted", func(t *testing.T) {
		assert.Contains(t, aliasedCard, "2 invocations")
		assert.Contains(t, aliasedCard, "last used 2026-07-12")
	})

	t.Run("alias-badge-rendered", func(t *testing.T) {
		assert.Contains(t, aliasedCard, `<span class="badge badge-dim">/pk</span>`)
		assert.NotContains(t, body, "/scripts/skills/home/pk")
	})

	t.Run("no-alias-no-badge", func(t *testing.T) {
		assert.Contains(t, plainCard, "/demo")
		assert.NotContains(t, plainCard, `<span class="badge badge-dim">/`)
	})
}

// skillCardOf returns the Skills-page card whose name link points at href,
// from that link to the card's end; "" when the body holds no such link.
func skillCardOf(body, href string) string {
	start := strings.Index(body, `href="`+href+`"`)
	if start < 0 {
		return ""
	}

	card, _, _ := strings.Cut(body[start:], "</div>")
	return card
}

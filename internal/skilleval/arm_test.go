package skilleval

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/heartbeat"
	"github.com/kevinhorst/smine/internal/skillgraph"
)

const fakeClaudeScript = `#!/bin/sh
[ -d "$2" ] && printf '{}\n' > "$2/review.json"
[ -n "$FAKE_CLAUDE_ARGS" ] && printf '%s\n' "$@" > "$FAKE_CLAUDE_ARGS"
echo '{"type":"result","subtype":"success","is_error":false,"session_id":"s-1","terminal_reason":"completed","result":"ok","total_cost_usd":1.5,"modelUsage":{"claude-sonnet-5":{"outputTokens":10}},"permission_denials":[{"tool_use_id":"t1"}]}'
`

// isFenced reports whether path lies inside one of the fence roots.
func isFenced(path string, roots []string) bool {
	for _, root := range roots {
		isInside := path == root || strings.HasPrefix(path, root+string(filepath.Separator))
		if isInside {
			return true
		}
	}
	return false
}

// newRunArmDir writes a one-arm run dir whose arm worktree is a fresh worktree
// of repo on branch, and returns the run dir and the arm worktree.
func newRunArmDir(t *testing.T, branch, repo string, scorer []string) (string, string) {
	armWorktree := filepath.Join(newTempDir(t), "run-2026-09-27-abc123-a")
	gitOutput(t, repo, "worktree", "add", "-q", "-b", branch, armWorktree)
	runDir := filepath.Join(newTempDir(t), "2026-09-27-abc123")
	manifest := &RunManifest{
		Arms:          []ManifestArm{{Id: "sonnet-high-r1", Key: "sonnet-high", Label: "A", Replica: 1, Worktree: armWorktree}},
		At:            gitOutput(t, repo, "rev-parse", "HEAD"),
		BudgetUsd:     1,
		CheckTimeoutS: 60,
		Layer:         LayerEval,
		Mode:          ModeNightly,
		Prompt:        "{artifactsDir}",
		Repo:          repo,
		Scorer:        scorer,
		Skill:         "railroad-review",
		SkillVersion:  "22.3.0",
		TimeoutS:      60,
		TokenLabel:    "work",
	}
	writeRunDir(t, runDir, manifest)
	return runDir, armWorktree
}

func readOrEmpty(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(content)
}

// stagedContent reads a staged MCP config; "" when nothing was staged.
func stagedContent(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(content)
}

func TestArmRunId(t *testing.T) {
	assert.Equal(t, "2026-09-27-a34281-a", armRunId("A", "/repo/evals/railroad-review/2026-09-27-a34281"))
}

func TestClaudeArgs(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		arm           *ManifestArm
		budgetUsd     float64
		mcpConfigPath string
		prompt        string
		tools         string
	}

	tests := make([]*testCase, 0)

	// defaults-only
	tests = append(tests, &testCase{
		_id: "defaults-only",
		_expected: []string{
			"claude",
			"-p", "/railroad-review",
			"--allowedTools", "Read, Skill",
			"--permission-mode", "acceptEdits",
			"--max-budget-usd", "30",
			"--output-format", "stream-json",
			"--verbose",
		},

		arm:           &ManifestArm{},
		budgetUsd:     30,
		mcpConfigPath: "",
		prompt:        "/railroad-review",
		tools:         "Read, Skill",
	})

	// model-and-effort
	tests = append(tests, &testCase{
		_id: "model-and-effort",
		_expected: []string{
			"claude",
			"-p", "/fimplement plan.md",
			"--allowedTools", "Read",
			"--permission-mode", "acceptEdits",
			"--max-budget-usd", "2.5",
			"--output-format", "stream-json",
			"--verbose",
			"--model", "opus",
			"--effort", "high",
		},

		arm:           &ManifestArm{Effort: "high", Model: "opus"},
		budgetUsd:     2.5,
		mcpConfigPath: "",
		prompt:        "/fimplement plan.md",
		tools:         "Read",
	})

	// two-disabled-servers-joined
	tests = append(tests, &testCase{
		_id: "two-disabled-servers-joined",
		_expected: []string{
			"claude",
			"-p", "/railroad-review",
			"--allowedTools", "Read",
			"--permission-mode", "acceptEdits",
			"--max-budget-usd", "45",
			"--output-format", "stream-json",
			"--verbose",
			"--model", "sonnet",
			"--disallowedTools", "mcp__goland, mcp__pycharm",
		},

		arm:           &ManifestArm{DisabledServers: []string{"goland", "pycharm"}, Model: "sonnet"},
		budgetUsd:     45,
		mcpConfigPath: "",
		prompt:        "/railroad-review",
		tools:         "Read",
	})

	// disabled-server-strict-config
	tests = append(tests, &testCase{
		_id: "disabled-server-strict-config",
		_expected: []string{
			"claude",
			"-p", "/qq",
			"--allowedTools", "Read",
			"--permission-mode", "acceptEdits",
			"--max-budget-usd", "3",
			"--output-format", "stream-json",
			"--verbose",
			"--disallowedTools", "mcp__goland",
			"--strict-mcp-config", "--mcp-config", "/tmp/skilleval-mcp-1.json",
		},

		arm:           &ManifestArm{DisabledServers: []string{"goland"}},
		budgetUsd:     3,
		mcpConfigPath: "/tmp/skilleval-mcp-1.json",
		prompt:        "/qq",
		tools:         "Read",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, claudeArgs(test.arm, test.budgetUsd, test.mcpConfigPath, test.prompt, "", test.tools))
		})
	}
}

func TestStageMcpConfig(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   string
		_shouldPass bool

		claudeJsonPath  string
		disabledServers []string
	}

	dir := newTempDir(t)
	registered := filepath.Join(dir, "claude.json")
	writeTestFile(t, registered, `{"numStartups": 3, "mcpServers": {"goland": {"type": "sse", "url": "http://127.0.0.1:64342/sse"}, "peek-mcp": {"command": "peek", "env": {"K": "v"}}}}`)
	malformed := filepath.Join(dir, "malformed.json")
	writeTestFile(t, malformed, `{"mcpServers": `)
	withoutServers := filepath.Join(dir, "plain.json")
	writeTestFile(t, withoutServers, `{"numStartups": 3}`)
	tests := make([]*testCase, 0)

	// nothing-disabled-stages-nothing
	tests = append(tests, &testCase{
		_id:         "nothing-disabled-stages-nothing",
		_expected:   "",
		_shouldPass: true,

		claudeJsonPath:  registered,
		disabledServers: nil,
	})

	// disabled-server-left-out
	tests = append(tests, &testCase{
		_id:         "disabled-server-left-out",
		_expected:   `{"mcpServers":{"peek-mcp":{"command":"peek","env":{"K":"v"}}}}`,
		_shouldPass: true,

		claudeJsonPath:  registered,
		disabledServers: []string{"goland"},
	})

	// unregistered-server-disabled-keeps-all
	tests = append(tests, &testCase{
		_id:         "unregistered-server-disabled-keeps-all",
		_expected:   `{"mcpServers":{"goland":{"type":"sse","url":"http://127.0.0.1:64342/sse"},"peek-mcp":{"command":"peek","env":{"K":"v"}}}}`,
		_shouldPass: true,

		claudeJsonPath:  registered,
		disabledServers: []string{"pycharm"},
	})

	// all-disabled-empty-config
	tests = append(tests, &testCase{
		_id:         "all-disabled-empty-config",
		_expected:   `{"mcpServers":{}}`,
		_shouldPass: true,

		claudeJsonPath:  registered,
		disabledServers: []string{"goland", "peek-mcp"},
	})

	// claude-json-missing-empty-config
	tests = append(tests, &testCase{
		_id:         "claude-json-missing-empty-config",
		_expected:   `{"mcpServers":{}}`,
		_shouldPass: true,

		claudeJsonPath:  filepath.Join(dir, "absent.json"),
		disabledServers: []string{"goland"},
	})

	// claude-json-without-servers
	tests = append(tests, &testCase{
		_id:         "claude-json-without-servers",
		_expected:   `{"mcpServers":{}}`,
		_shouldPass: true,

		claudeJsonPath:  withoutServers,
		disabledServers: []string{"goland"},
	})

	// claude-json-malformed-fails
	tests = append(tests, &testCase{
		_id:         "claude-json-malformed-fails",
		_expected:   "",
		_shouldPass: false,

		claudeJsonPath:  malformed,
		disabledServers: []string{"goland"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			staged, err := stageMcpConfig(test.claudeJsonPath, test.disabledServers)
			t.Cleanup(func() { _ = os.Remove(staged) })
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, stagedContent(staged))
		})
	}
}

func TestExportReadFence(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		path string
	}

	home := newTempDir(t)
	t.Setenv("HOME", home)
	t.Setenv(config.ReadFenceEnv, "")
	repo := newFixtureRepo(t)
	t.Chdir(repo)
	armWorktree := filepath.Join(newTempDir(t), "run-2026-09-27-abc123-a")
	for _, dir := range []string{
		filepath.Join(repo, "evals"),
		filepath.Join(repo, "skills", "leaf", "eval"),
		filepath.Join(repo, "skills", "group", "leaf", "eval"),
		filepath.Join(repo, ".claude", "worktrees", "session", "evals"),
		filepath.Join(armWorktree, "skills", "leaf", "eval"),
		filepath.Join(armWorktree+outDirSuffix, artifactsDir),
		filepath.Join(home, ".claude", "skills", "leaf", "eval"),
	} {
		require.NoError(t, os.MkdirAll(dir, 0o755))
	}
	writeTestFile(t, filepath.Join(home, ".claude", "skills", "leaf", skillgraph.FileHelpTest), "Smoke: make skill-smoke SKILL=leaf TOKEN=<label>\n")
	require.NoError(t, exportReadFence(context.Background(), armWorktree))
	roots := filepath.SplitList(os.Getenv(config.ReadFenceEnv))
	tests := make([]*testCase, 0)

	// repo-evals-and-skill-eval-dirs
	tests = append(tests, &testCase{
		_id:       "repo-evals-and-skill-eval-dirs",
		_expected: true,

		path: filepath.Join(repo, "evals", "railroad-review", "run.json"),
	})

	// repo-skill-eval-dir
	tests = append(tests, &testCase{
		_id:       "repo-skill-eval-dir",
		_expected: true,

		path: filepath.Join(repo, "skills", "leaf", "eval", "definition.json"),
	})

	// repo-grouped-skill-eval-dir
	tests = append(tests, &testCase{
		_id:       "repo-grouped-skill-eval-dir",
		_expected: true,

		path: filepath.Join(repo, "skills", "group", "leaf", "eval", "golden"),
	})

	// session-worktrees-included
	tests = append(tests, &testCase{
		_id:       "session-worktrees-included",
		_expected: true,

		path: filepath.Join(repo, ".claude", "worktrees", "session", "evals", "report.md"),
	})

	// arm-tree-included
	tests = append(tests, &testCase{
		_id:       "arm-tree-included",
		_expected: true,

		path: filepath.Join(armWorktree, "skills", "leaf", "eval", "README.md"),
	})

	// deployed-help-test-included
	tests = append(tests, &testCase{
		_id:       "deployed-help-test-included",
		_expected: true,

		path: filepath.Join(home, ".claude", "skills", "leaf", skillgraph.FileHelpTest),
	})

	// deployed-eval-dir-included
	tests = append(tests, &testCase{
		_id:       "deployed-eval-dir-included",
		_expected: true,

		path: filepath.Join(home, ".claude", "skills", "leaf", "eval", "definition.json"),
	})

	// plans-archive-store-fenced
	tests = append(tests, &testCase{
		_id:       "plans-archive-store-fenced",
		_expected: true,

		path: filepath.Join(home, "GolandProjects", "agent-plans", "repo", "proposals", "problems.json"),
	})

	// arm-store-never-fenced
	tests = append(tests, &testCase{
		_id:       "arm-store-never-fenced",
		_expected: false,

		path: filepath.Join(armWorktree, armStoreDir, "proposals", "problems.json"),
	})

	// out-dir-never-fenced
	tests = append(tests, &testCase{
		_id:       "out-dir-never-fenced",
		_expected: false,

		path: filepath.Join(armWorktree+outDirSuffix, artifactsDir, "round-1", "review.json"),
	})

	// deployed-skill-md-not-fenced
	tests = append(tests, &testCase{
		_id:       "deployed-skill-md-not-fenced",
		_expected: false,

		path: filepath.Join(home, ".claude", "skills", "leaf", "SKILL.md"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, isFenced(test.path, roots))
		})
	}
}

func TestExportContextIndex(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		contextIndex string
	}

	armWorktree := filepath.Join(newTempDir(t), "run-2026-09-28-1f725c-a")
	tests := make([]*testCase, 0)

	// index-declared
	tests = append(tests, &testCase{
		_id:       "index-declared",
		_expected: filepath.Join(armWorktree, "context", "review-index.json"),

		contextIndex: "context/review-index.json",
	})

	// index-absent
	tests = append(tests, &testCase{
		_id:       "index-absent",
		_expected: "/inherited/context.json",

		contextIndex: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			t.Setenv(contextIndexEnv, "/inherited/context.json")
			err := exportContextIndex(armWorktree, test.contextIndex)
			assert.NoError(t, err)
			assert.Equal(t, test._expected, os.Getenv(contextIndexEnv))
		})
	}
}

func TestExportStoreRoot(t *testing.T) {
	t.Setenv("HOME", newTempDir(t))
	t.Setenv(config.StoreRootEnv, "")
	armWorktree := filepath.Join(newTempDir(t), "run-2026-09-27-abc123-a")

	require.NoError(t, exportStoreRoot(armWorktree))

	root, err := config.StoreRoot("/src/claude-configs")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(armWorktree, armStoreDir), root, "every store lookup of the arm lands in its worktree")
}

func TestExportSmokeEnv(t *testing.T) {
	t.Setenv("SKILL_RAILROADREVIEW_BUILD", "smine-rules buildtest")
	t.Setenv("SKILL_FIMPLEMENT_CHECK", "make audit")
	t.Setenv(contextIndexEnv, "/arm/context.json")
	t.Setenv(config.SkillEnvOffEnv, "")
	document := &skillgraph.SmokeDocument{Skill: "railroad-review", Smoke: &skillgraph.Smoke{Env: map[string]string{"BUILD": "go build ./x/...", "TEST": "go test ./x/..."}}}

	require.NoError(t, exportSmokeEnv(document))

	assert.Equal(t, "go build ./x/...", os.Getenv("SKILL_RAILROADREVIEW_BUILD"), "the smoke's value replaces the machine's")
	assert.Equal(t, "go test ./x/...", os.Getenv("SKILL_RAILROADREVIEW_TEST"))
	_, isInherited := os.LookupEnv("SKILL_FIMPLEMENT_CHECK")
	assert.False(t, isInherited, "a machine value of another skill is cleared")
	assert.Equal(t, "/arm/context.json", os.Getenv(contextIndexEnv), "the arm's context index survives")
	assert.Equal(t, "1", os.Getenv(config.SkillEnvOffEnv))
}

func TestExportToken(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedToken string
		_shouldPass    bool

		label string
	}

	home := newTempDir(t)
	t.Setenv("HOME", home)
	writeTestFile(t, filepath.Join(home, ".config", "claude-routine", "tokens", "work"), "stored-token\n")
	tests := make([]*testCase, 0)

	// fail-empty-label
	tests = append(tests, &testCase{
		_id:            "fail-empty-label",
		_expectedToken: "ambient-token",
		_shouldPass:    false,

		label: "",
	})

	// fail-unknown-label
	tests = append(tests, &testCase{
		_id:            "fail-unknown-label",
		_expectedToken: "ambient-token",
		_shouldPass:    false,

		label: "missing",
	})

	// pass-stored-label
	tests = append(tests, &testCase{
		_id:            "pass-stored-label",
		_expectedToken: "stored-token",
		_shouldPass:    true,

		label: "work",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			t.Setenv(oauthTokenEnv, "ambient-token")
			t.Setenv("ANTHROPIC_API_KEY", "ambient-key")
			err := exportToken(test.label)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedToken, os.Getenv(oauthTokenEnv))
			assert.Equal(t, !test._shouldPass, os.Getenv("ANTHROPIC_API_KEY") != "")
		})
	}
}

func TestParseScores(t *testing.T) {
	type testCase struct {
		_id            string
		_expected      map[string]float64
		_expectedCause string

		stdout string
	}

	tests := make([]*testCase, 0)

	// valid
	tests = append(tests, &testCase{
		_id:            "valid",
		_expected:      map[string]float64{"coverage_pct": 81.5, "tests_passed": 3},
		_expectedCause: "",

		stdout: `{"metrics":{"tests_passed":3,"coverage_pct":81.5}}`,
	})

	// malformed-json
	tests = append(tests, &testCase{
		_id:            "malformed-json",
		_expected:      nil,
		_expectedCause: "malformed stdout: unexpected end of JSON input",

		stdout: `{"metrics":`,
	})

	// empty-metrics
	tests = append(tests, &testCase{
		_id:            "empty-metrics",
		_expected:      nil,
		_expectedCause: "no metrics",

		stdout: `{"metrics":{}}`,
	})

	// bad-name
	tests = append(tests, &testCase{
		_id:            "bad-name",
		_expected:      nil,
		_expectedCause: `metric name "Bad-Name" is not [a-z][a-z0-9_]*`,

		stdout: `{"metrics":{"Bad-Name":1}}`,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			scores, cause := parseScores([]byte(test.stdout))
			assert.Equal(t, test._expected, scores)
			assert.Equal(t, test._expectedCause, cause)
		})
	}
}

func TestRenderPrompt(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		overlayPatch string
	}

	tests := make([]*testCase, 0)

	// overlay-filled
	tests = append(tests, &testCase{
		_id:       "overlay-filled",
		_expected: "/railroad-review out=/runs/A/artifacts id=2026-09-26-a36dd5-A again=2026-09-26-a36dd5-A overlay-patch: /runs/overlay.patch",

		overlayPatch: "/runs/overlay.patch",
	})

	// overlay-empty
	tests = append(tests, &testCase{
		_id:       "overlay-empty",
		_expected: "/railroad-review out=/runs/A/artifacts id=2026-09-26-a36dd5-A again=2026-09-26-a36dd5-A overlay-patch: ",

		overlayPatch: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			rendered := renderPrompt(
				"/runs/A/artifacts",
				test.overlayPatch,
				"/railroad-review out={artifactsDir} id={runId} again={runId} overlay-patch: {overlayPatch}",
				"2026-09-26-a36dd5-A",
				"",
			)
			assert.Equal(t, test._expected, rendered)
		})
	}
}

func TestWriteArmSettings(t *testing.T) {
	armWorktree := filepath.Join(newTempDir(t), "run-2026-09-28-abc123-a")
	require.NoError(t, os.MkdirAll(armWorktree, 0o755))

	err := writeArmSettings(armWorktree)

	require.NoError(t, err)
	settings := armSettings{}
	require.NoError(t, readJsonFile(filepath.Join(armWorktree, ".claude", "settings.local.json"), &settings))
	assert.Equal(t, []string{armWorktree + outDirSuffix}, settings.Permissions.AdditionalDirectories)
}

func TestRunArm(t *testing.T) {
	type testCase struct {
		_id                 string
		_expectedScorerLog  string
		_expectedScores     map[string]float64
		_isScorerLogPresent bool

		armWorktree string
		runDir      string
	}

	home := newTempDir(t)
	t.Setenv("HOME", home)
	t.Setenv(config.ReadFenceEnv, "")
	t.Setenv(config.StoreRootEnv, "")
	t.Setenv(oauthTokenEnv, "")
	writeTestFile(t, filepath.Join(home, ".config", "claude-routine", "tokens", "work"), "stored-token\n")
	binDir := newTempDir(t)
	writeTestFile(t, filepath.Join(binDir, "claude"), fakeClaudeScript)
	require.NoError(t, os.Chmod(filepath.Join(binDir, "claude"), 0o755))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	repo := newFixtureRepo(t)
	t.Chdir(repo)
	readRuntime := func(context.Context) (*heartbeat.RuntimeDocument, error) {
		runs := []heartbeat.Run{{Id: 1, Session: "s-1", State: "open"}, {Id: 2, Session: "other", State: "killed"}}
		return &heartbeat.RuntimeDocument{Runs: runs}, nil
	}

	tests := make([]*testCase, 0)

	// no-scorer
	noScorerRunDir, noScorerWorktree := newRunArmDir(t, "claude-review/run-2026-09-27-abc123-a", repo, nil)
	tests = append(tests, &testCase{
		_id:                 "no-scorer",
		_expectedScorerLog:  "",
		_expectedScores:     nil,
		_isScorerLogPresent: false,

		armWorktree: noScorerWorktree,
		runDir:      noScorerRunDir,
	})

	// scorer-metrics
	metricsScorer := []string{"sh", "-c", "echo '{\"metrics\":{\"tests_passed\":3}}'"}
	metricsRunDir, metricsWorktree := newRunArmDir(t, "claude-review/run-2026-09-27-abc123-b", repo, metricsScorer)
	tests = append(tests, &testCase{
		_id:                 "scorer-metrics",
		_expectedScorerLog:  "",
		_expectedScores:     map[string]float64{"tests_passed": 3},
		_isScorerLogPresent: true,

		armWorktree: metricsWorktree,
		runDir:      metricsRunDir,
	})

	// scorer-exit-1
	failingScorer := []string{"sh", "-c", "echo '{\"metrics\":{\"tests_passed\":3}}'; exit 1"}
	failingRunDir, failingWorktree := newRunArmDir(t, "claude-review/run-2026-09-27-abc123-c", repo, failingScorer)
	tests = append(tests, &testCase{
		_id:                 "scorer-exit-1",
		_expectedScorerLog:  "scorer: exit status 1\n{\"metrics\":{\"tests_passed\":3}}\n",
		_expectedScores:     nil,
		_isScorerLogPresent: true,

		armWorktree: failingWorktree,
		runDir:      failingRunDir,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := RunArm(context.Background(), "A", "", test.runDir, readRuntime)

			require.NoError(t, err)
			var captured ArmRuntime
			require.NoError(t, readJsonFile(filepath.Join(test.runDir, runsDir, "A", runtimeFile), &captured))
			assert.Equal(t, []heartbeat.Run{{Id: 1, Session: "s-1", State: "open"}}, captured.Runs)
			var result ArmResult
			require.NoError(t, readJsonFile(filepath.Join(test.runDir, runsDir, "A", resultFile), &result))
			assert.FileExists(t, filepath.Join(test.runDir, runsDir, "A", artifactsDir, "review.json"))
			assert.NoDirExists(t, filepath.Join(test.armWorktree+outDirSuffix, artifactsDir))
			assert.FileExists(t, filepath.Join(test.armWorktree, ".claude", "settings.local.json"))
			assert.Equal(t, filepath.Join(test.armWorktree, armStoreDir), os.Getenv(config.StoreRootEnv))
			require.NotNil(t, result.OutputTokens)
			assert.Equal(t, 10, *result.OutputTokens)
			require.NotNil(t, result.CostUsd)
			assert.InDelta(t, 1.5, *result.CostUsd, 1e-9)
			require.NotNil(t, result.PermissionDenials)
			assert.Equal(t, 1, *result.PermissionDenials)
			assert.Empty(t, result.InvalidReasons)
			assert.Equal(t, LayerEval, result.Layer)
			assert.Equal(t, ModeNightly, result.Mode)
			assert.Equal(t, "railroad-review", result.Skill)
			assert.Equal(t, "22.3.0", result.SkillVersion)
			assert.Nil(t, result.Smoke)
			assert.Equal(t, test._expectedScores, result.Scores)
			_, statErr := os.Stat(filepath.Join(test.runDir, runsDir, "A", scorerLogFile))
			assert.Equal(t, test._isScorerLogPresent, statErr == nil)
			assert.Equal(t, test._expectedScorerLog, readOrEmpty(filepath.Join(test.runDir, runsDir, "A", scorerLogFile)))
		})
	}
}

func TestRunArmResume(t *testing.T) {
	home := newTempDir(t)
	t.Setenv("HOME", home)
	t.Setenv(config.ReadFenceEnv, "")
	t.Setenv(config.StoreRootEnv, "")
	t.Setenv(oauthTokenEnv, "")
	writeTestFile(t, filepath.Join(home, ".config", "claude-routine", "tokens", "work"), "stored-token\n")
	binDir := newTempDir(t)
	writeTestFile(t, filepath.Join(binDir, "claude"), fakeClaudeScript)
	require.NoError(t, os.Chmod(filepath.Join(binDir, "claude"), 0o755))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	argsPath := filepath.Join(newTempDir(t), "args.txt")
	t.Setenv("FAKE_CLAUDE_ARGS", argsPath)
	repo := newFixtureRepo(t)
	t.Chdir(repo)
	runDir, armWorktree := newRunArmDir(t, "claude-review/run-2026-09-27-abc123-d", repo, nil)
	armDir := filepath.Join(runDir, runsDir, "A")
	writeTestFile(t, filepath.Join(armDir, artifactsDir, "round-1", "review.json"), "{}\n")
	stoppedResultLine := `{"type":"result","subtype":"success","is_error":true,"session_id":"s-1","result":"limit","total_cost_usd":2,"modelUsage":{"claude-sonnet-5":{"outputTokens":5}},"permission_denials":[{"tool_use_id":"t0"}]}` + "\n"
	writeTestFile(t, filepath.Join(armDir, streamFile), stoppedResultLine)
	startedAt := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	stopped := &ArmResult{StartedAt: startedAt}
	require.NoError(t, writeJsonFile(filepath.Join(armDir, resultFile), stopped))
	readRuntime := func(context.Context) (*heartbeat.RuntimeDocument, error) {
		return &heartbeat.RuntimeDocument{}, nil
	}

	err := RunArm(context.Background(), "A", "01234567-89ab-cdef-0123-456789abcdef", runDir, readRuntime)

	require.NoError(t, err)
	args := readOrEmpty(argsPath)
	assert.Contains(t, args, "--resume\n01234567-89ab-cdef-0123-456789abcdef\n")
	assert.Contains(t, args, "-p\n"+resumePrompt+"\n")
	var result ArmResult
	require.NoError(t, readJsonFile(filepath.Join(armDir, resultFile), &result))
	assert.True(t, startedAt.Equal(result.StartedAt))
	require.NotNil(t, result.CostUsd)
	assert.InDelta(t, 3.5, *result.CostUsd, 1e-9)
	require.NotNil(t, result.OutputTokens)
	assert.Equal(t, 15, *result.OutputTokens)
	require.NotNil(t, result.PermissionDenials)
	assert.Equal(t, 2, *result.PermissionDenials)
	assert.FileExists(t, filepath.Join(armDir, artifactsDir, "round-1", "review.json"))
	assert.NoDirExists(t, filepath.Join(armWorktree+outDirSuffix, artifactsDir))
}

func TestRunArmResumeMalformedSession(t *testing.T) {
	err := RunArm(context.Background(), "A", "not-a-session", newTempDir(t), nil)

	assert.ErrorIs(t, err, ErrBadRunArg)
}

func TestSmokeCoverage_Summary(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		coverage *SmokeCoverage
	}

	tests := make([]*testCase, 0)

	// with-expectation
	tests = append(tests, &testCase{
		_id:       "with-expectation",
		_expected: "anchors 6/6 (12), checks 12/16",

		coverage: &SmokeCoverage{
			AnchorsDeclared:    12,
			AnchorsExpected:    []string{"base", "round", "station", "verdict", "fix", "close"},
			AnchorsExpectedHit: 6,
			AnchorsHit:         9,
			ChecksDeclared:     16,
			ChecksPassed:       12,
		},
	})

	// without
	tests = append(tests, &testCase{
		_id:       "without",
		_expected: "anchors 2 (3), checks 1/2",

		coverage: &SmokeCoverage{AnchorsDeclared: 3, AnchorsHit: 2, ChecksDeclared: 2, ChecksPassed: 1},
	})

	// older-record-decoded
	var older SmokeCoverage
	require.NoError(t, json.Unmarshal([]byte(`{"anchorsDeclared":12,"anchorsHit":6,"checksDeclared":16,"checksPassed":12}`), &older))
	tests = append(tests, &testCase{
		_id:       "older-record-decoded",
		_expected: "anchors 6 (12), checks 12/16",

		coverage: &older,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.coverage.Summary())
		})
	}
}

func TestStageOverlay(t *testing.T) {
	type testCase struct {
		_id              string
		_expected        string
		_expectedContent string

		armWorktree string
		overlay     string
	}

	tests := make([]*testCase, 0)

	// copied-beside-worktree
	copiedWorktree := filepath.Join(newTempDir(t), "run-2026-09-27-abc123-a")
	require.NoError(t, os.MkdirAll(copiedWorktree+outDirSuffix, 0o755))
	copiedOverlay := filepath.Join(newTempDir(t), overlayFile)
	writeTestFile(t, copiedOverlay, "diff --git a/x b/x\n")
	tests = append(tests, &testCase{
		_id:              "copied-beside-worktree",
		_expected:        filepath.Join(copiedWorktree+outDirSuffix, overlayCopyFile),
		_expectedContent: "diff --git a/x b/x\n",

		armWorktree: copiedWorktree,
		overlay:     copiedOverlay,
	})

	// no-overlay-empty
	tests = append(tests, &testCase{
		_id:              "no-overlay-empty",
		_expected:        "",
		_expectedContent: "",

		armWorktree: filepath.Join(newTempDir(t), "run-2026-09-27-abc123-b"),
		overlay:     "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			staged, err := stageOverlay(test.armWorktree, test.overlay)
			require.NoError(t, err)
			assert.Equal(t, test._expected, staged)
			assert.Equal(t, test._expectedContent, readOrEmpty(staged))
		})
	}
}

func TestVariantProblems(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		arm        *ManifestArm
		initRecord *InitRecord
		required   []string
	}

	tests := make([]*testCase, 0)

	// nil-init
	tests = append(tests, &testCase{
		_id:       "nil-init",
		_expected: nil,

		arm:        &ManifestArm{DisabledServers: []string{"pycharm"}, Model: "sonnet"},
		initRecord: nil,
		required:   []string{"goland"},
	})

	// clean
	tests = append(tests, &testCase{
		_id:       "clean",
		_expected: nil,

		arm: &ManifestArm{DisabledServers: []string{"pycharm"}, Model: "sonnet"},
		initRecord: &InitRecord{
			McpServers: []McpServer{{Name: "goland", Status: statusConnected}},
			Model:      "claude-sonnet-5",
			Tools:      []string{"Read", "mcp__goland__read_file"},
		},
		required: []string{"goland"},
	})

	// model-fallback
	tests = append(tests, &testCase{
		_id:       "model-fallback",
		_expected: []string{"requested model nosuchmodel but the cell ran claude-opus-5-5[1m]"},

		arm: &ManifestArm{Model: "nosuchmodel"},
		initRecord: &InitRecord{
			Model: "claude-opus-5-5[1m]",
			Tools: []string{"Read"},
		},
		required: nil,
	})

	// no-model-requested
	tests = append(tests, &testCase{
		_id:       "no-model-requested",
		_expected: nil,

		arm: &ManifestArm{},
		initRecord: &InitRecord{
			Model: "claude-opus-5-5[1m]",
			Tools: []string{"Read"},
		},
		required: nil,
	})

	// disabled-server-still-offered
	tests = append(tests, &testCase{
		_id:       "disabled-server-still-offered",
		_expected: []string{"server goland disabled but its tools are still offered"},

		arm: &ManifestArm{DisabledServers: []string{"goland"}},
		initRecord: &InitRecord{
			McpServers: []McpServer{{Name: "goland", Status: statusConnected}},
			Tools:      []string{"Read", "mcp__goland__search_symbol"},
		},
		required: nil,
	})

	// required-server-failed
	tests = append(tests, &testCase{
		_id:       "required-server-failed",
		_expected: []string{"server goland not connected (failed) — comparison void"},

		arm: &ManifestArm{},
		initRecord: &InitRecord{
			McpServers: []McpServer{{Name: "goland", Status: "failed"}},
			Tools:      []string{"Read"},
		},
		required: []string{"goland"},
	})

	// required-server-absent
	tests = append(tests, &testCase{
		_id:       "required-server-absent",
		_expected: []string{"server goland not connected (absent) — comparison void"},

		arm: &ManifestArm{},
		initRecord: &InitRecord{
			McpServers: []McpServer{{Name: "peek-mcp", Status: statusConnected}},
			Tools:      []string{"Read"},
		},
		required: []string{"goland"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, variantProblems(test.arm, test.initRecord, test.required))
		})
	}
}

package routinerun

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bootstrapDryLine is one stage line a dry run prints.
func bootstrapDryLine(prompt, skill string, timeoutS int, effort, tools string) string {
	return fmt.Sprintf(
		`DRY: claude -p "%s" [skill=%s timeout=%ds effort=%s tools=%s]`,
		prompt,
		skill,
		timeoutS,
		effort,
		tools,
	)
}

// bootstrapDryRunPattern matches exactly the given stdout lines.
func bootstrapDryRunPattern(lines ...string) string {
	if len(lines) == 0 {
		return "^$"
	}
	return "^" + regexp.QuoteMeta(strings.Join(lines, "\n")+"\n") + "$"
}

// newBootstrapHome is a fake HOME with a registered peek-mcp, an
// allowed-tools manifest for the smine skill only, and the given
// presentation profile ("" = none).
func newBootstrapHome(t *testing.T, profile string) string {
	t.Helper()
	home := t.TempDir()
	claudeJson := `{"mcpServers":{"peek-mcp":{"type":"stdio","command":"peek-mcp","args":["start","--transport=stdio"]}}}`
	require.NoError(t, os.WriteFile(filepath.Join(home, ".claude.json"), []byte(claudeJson), 0o600))

	skillDir := filepath.Join(home, ".claude", "skills", "smine")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	manifest := "---\nname: smine\nallowed-tools: Read, Bash(make sessions *)\n---\n\n# smine\n"
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(manifest), 0o644))
	if profile == "" {
		return home
	}

	profileDir := filepath.Join(home, ".claude", "context", "global")
	require.NoError(t, os.MkdirAll(profileDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(profileDir, "presentation-profile.md"), []byte(profile), 0o644))
	return home
}

// readBootstrapResults decodes every row of a results file.
func readBootstrapResults(t *testing.T, path string) []*bootstrapResult {
	t.Helper()
	file, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = file.Close() }()

	rows := make([]*bootstrapResult, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		row := &bootstrapResult{}
		require.NoError(t, json.Unmarshal(scanner.Bytes(), row))
		rows = append(rows, row)
	}
	require.NoError(t, scanner.Err())
	return rows
}

func TestBootstrap_Run(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedStatus int
		_expectedStderr string
		_expectedStdout string

		env      map[string]string
		repoRoot string
		stderr   *bytes.Buffer
		stdout   *bytes.Buffer
	}

	envKeys := []string{
		"BOOTSTRAP_DRY_RUN",
		"BOOTSTRAP_EXTRA_PROMPT",
		"BOOTSTRAP_N",
		"BOOTSTRAP_SINCE",
		"BOOTSTRAP_TOKEN_FILE",
		"HOME",
		"SMINE_AGENTS",
		"SMINE_DEV_MODE",
		"USERPROFILE",
	}
	runMutated := []string{
		"CLAUDE_CODE_OAUTH_TOKEN",
		"CLAUDE_CODE_PRINT_BG_WAIT_CEILING_MS",
		"DISABLE_AUTOUPDATER",
		"DISABLE_TELEMETRY",
		"PATH",
	}
	for _, key := range runMutated {
		t.Setenv(key, os.Getenv(key))
	}

	repo := newTestRepo(t)
	featureRepo := newTestRepo(t)
	gitRun(t, featureRepo, "switch", "-q", "-c", "feature")
	dirtyRepo := newTestRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dirtyRepo, "stray.txt"), []byte("stray\n"), 0o644))

	home := newBootstrapHome(t, "")
	casualHome := newBootstrapHome(t, "---\naudience: casual\ndev-mode: true\n---\n")
	noPeekHome := t.TempDir()

	styleLine := bootstrapDryLine("/smine-style 30", "smine-style", 3600, "medium", "none")
	orchestrateLine := bootstrapDryLine("/smine-orchestrate bootstrap", "smine-orchestrate", 3600, "medium", "none")
	mineTools := "Read, Bash(make sessions *)"
	minePrompt := "/smine --nightly --stages " + nightlyMiningStages
	mineLine := bootstrapDryLine(minePrompt+" --limit 30 --agents claude,codex", "smine", 10800, "medium", mineTools)
	tests := make([]*testCase, 0)

	// dry-run-three-stages
	tests = append(tests, &testCase{
		_id:             "dry-run-three-stages",
		_expectedStatus: exitOk,
		_expectedStderr: "",
		_expectedStdout: bootstrapDryRunPattern(styleLine, mineLine, orchestrateLine),

		env:      map[string]string{"BOOTSTRAP_DRY_RUN": "1", "HOME": home, "USERPROFILE": home},
		repoRoot: repo,
		stderr:   &bytes.Buffer{},
		stdout:   &bytes.Buffer{},
	})

	// since-reaches-orchestrate-only
	tests = append(tests, &testCase{
		_id:             "since-reaches-orchestrate-only",
		_expectedStatus: exitOk,
		_expectedStderr: "",
		_expectedStdout: bootstrapDryRunPattern(
			bootstrapDryLine("/smine-style 12", "smine-style", 3600, "medium", "none"),
			bootstrapDryLine(minePrompt+" --limit 12 --agents claude,codex", "smine", 10800, "medium", mineTools),
			bootstrapDryLine("/smine-orchestrate bootstrap (since: 2026-09-01)", "smine-orchestrate", 3600, "medium", "none"),
		),

		env: map[string]string{
			"BOOTSTRAP_DRY_RUN": "1",
			"BOOTSTRAP_N":       "12",
			"BOOTSTRAP_SINCE":   "2026-09-01",
			"HOME":              home,
			"USERPROFILE":       home,
		},
		repoRoot: repo,
		stderr:   &bytes.Buffer{},
		stdout:   &bytes.Buffer{},
	})

	// dev-mode-and-extra-prompt-reach-mine
	tests = append(tests, &testCase{
		_id:             "dev-mode-and-extra-prompt-reach-mine",
		_expectedStatus: exitOk,
		_expectedStderr: "",
		_expectedStdout: bootstrapDryRunPattern(
			styleLine,
			bootstrapDryLine(minePrompt+" --limit 30 --dev --agents claude --cap 2", "smine", 10800, "medium", mineTools),
			orchestrateLine,
		),

		env: map[string]string{
			"BOOTSTRAP_DRY_RUN":      "1",
			"BOOTSTRAP_EXTRA_PROMPT": "--cap 2",
			"HOME":                   home,
			"SMINE_AGENTS":           "claude",
			"SMINE_DEV_MODE":         "1",
			"USERPROFILE":            home,
		},
		repoRoot: repo,
		stderr:   &bytes.Buffer{},
		stdout:   &bytes.Buffer{},
	})

	// casual-audience-forces-dev-mode-off
	tests = append(tests, &testCase{
		_id:             "casual-audience-forces-dev-mode-off",
		_expectedStatus: exitOk,
		_expectedStderr: "",
		_expectedStdout: bootstrapDryRunPattern(styleLine, mineLine, orchestrateLine),

		env: map[string]string{
			"BOOTSTRAP_DRY_RUN": "1",
			"HOME":              casualHome,
			"SMINE_DEV_MODE":    "1",
			"USERPROFILE":       casualHome,
		},
		repoRoot: repo,
		stderr:   &bytes.Buffer{},
		stdout:   &bytes.Buffer{},
	})

	// missing-manifest-warns
	tests = append(tests, &testCase{
		_id:             "missing-manifest-warns",
		_expectedStatus: exitOk,
		_expectedStderr: "no allowed-tools manifest for smine-orchestrate; running without --allowedTools",
		_expectedStdout: bootstrapDryRunPattern(styleLine, mineLine, orchestrateLine),

		env:      map[string]string{"BOOTSTRAP_DRY_RUN": "1", "HOME": home, "USERPROFILE": home},
		repoRoot: repo,
		stderr:   &bytes.Buffer{},
		stdout:   &bytes.Buffer{},
	})

	// not-on-main-exits-64
	tests = append(tests, &testCase{
		_id:             "not-on-main-exits-64",
		_expectedStatus: exitPreflight,
		_expectedStderr: "bootstrap requires the main checkout on main (found: feature)",
		_expectedStdout: bootstrapDryRunPattern(),

		env:      map[string]string{"BOOTSTRAP_DRY_RUN": "1", "HOME": home, "USERPROFILE": home},
		repoRoot: featureRepo,
		stderr:   &bytes.Buffer{},
		stdout:   &bytes.Buffer{},
	})

	// dirty-tree-exits-64
	tests = append(tests, &testCase{
		_id:             "dirty-tree-exits-64",
		_expectedStatus: exitPreflight,
		_expectedStderr: "bootstrap requires a clean tree",
		_expectedStdout: bootstrapDryRunPattern(),

		env:      map[string]string{"BOOTSTRAP_DRY_RUN": "1", "HOME": home, "USERPROFILE": home},
		repoRoot: dirtyRepo,
		stderr:   &bytes.Buffer{},
		stdout:   &bytes.Buffer{},
	})

	// missing-token-exits-78
	tests = append(tests, &testCase{
		_id:             "missing-token-exits-78",
		_expectedStatus: exitConfig,
		_expectedStderr: "token file missing or empty: <unset>",
		_expectedStdout: bootstrapDryRunPattern(),

		env:      map[string]string{"HOME": home, "USERPROFILE": home},
		repoRoot: repo,
		stderr:   &bytes.Buffer{},
		stdout:   &bytes.Buffer{},
	})

	// missing-peek-config-exits-78
	tests = append(tests, &testCase{
		_id:             "missing-peek-config-exits-78",
		_expectedStatus: exitConfig,
		_expectedStderr: "no smine peek config — run bin/configserver -apply",
		_expectedStdout: bootstrapDryRunPattern(),

		env:      map[string]string{"BOOTSTRAP_DRY_RUN": "1", "HOME": noPeekHome, "USERPROFILE": noPeekHome},
		repoRoot: repo,
		stderr:   &bytes.Buffer{},
		stdout:   &bytes.Buffer{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			for _, key := range envKeys {
				t.Setenv(key, test.env[key])
			}

			status := NewBootstrap(test.repoRoot, test.stderr, test.stdout).Run(context.Background())

			assert.Equal(t, test._expectedStatus, status)
			assert.Contains(t, test.stderr.String(), test._expectedStderr)
			assert.Regexp(t, test._expectedStdout, test.stdout.String())
		})
	}
}

func TestBootstrap_recordResult(t *testing.T) {
	type testCase struct {
		_id           string
		_expectedRows []*bootstrapResult

		bootstrap *Bootstrap
		output    []byte
		stage     string
		status    int
	}

	syncStamp := "2000-01-01T00:00:00Z"
	tests := make([]*testCase, 0)

	// envelope-fields-copied
	tests = append(tests, &testCase{
		_id: "envelope-fields-copied",
		_expectedRows: []*bootstrapResult{
			{ExitStatus: 3, IsError: true, Result: "done", SessionId: "session-1", Stage: "smine", Subtype: "error_max_turns", Timestamp: syncStamp},
		},

		bootstrap: &Bootstrap{results: filepath.Join(t.TempDir(), bootstrapResultsFile), stderr: &bytes.Buffer{}},
		output:    []byte(`{"type":"result","subtype":"error_max_turns","is_error":true,"session_id":"session-1","num_turns":2,"result":"done"}`),
		stage:     "smine",
		status:    3,
	})

	// result-cut-at-300-runes
	tests = append(tests, &testCase{
		_id: "result-cut-at-300-runes",
		_expectedRows: []*bootstrapResult{
			{ExitStatus: 0, IsError: false, Result: strings.Repeat("é", bootstrapResultLimit), SessionId: "session-1", Stage: "smine-orchestrate", Subtype: "success", Timestamp: syncStamp},
		},

		bootstrap: &Bootstrap{results: filepath.Join(t.TempDir(), bootstrapResultsFile), stderr: &bytes.Buffer{}},
		output:    []byte(`{"subtype":"success","is_error":false,"session_id":"session-1","result":"` + strings.Repeat("é", bootstrapResultLimit+20) + `"}`),
		stage:     "smine-orchestrate",
		status:    0,
	})

	// non-json-output-leaves-fields-empty
	tests = append(tests, &testCase{
		_id: "non-json-output-leaves-fields-empty",
		_expectedRows: []*bootstrapResult{
			{ExitStatus: 124, IsError: false, Result: "", SessionId: "", Stage: "smine-style", Subtype: "", Timestamp: syncStamp},
		},

		bootstrap: &Bootstrap{results: filepath.Join(t.TempDir(), bootstrapResultsFile), stderr: &bytes.Buffer{}},
		output:    []byte("claude: command crashed\n"),
		stage:     "smine-style",
		status:    124,
	})

	// rows-append
	appendResults := filepath.Join(t.TempDir(), bootstrapResultsFile)
	existing := &bootstrapResult{ExitStatus: 0, Stage: "smine-style", Timestamp: "1999-12-31T23:00:00Z"}
	existingLine, err := json.Marshal(existing)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(appendResults, append(existingLine, '\n'), 0o644))
	tests = append(tests, &testCase{
		_id: "rows-append",
		_expectedRows: []*bootstrapResult{
			existing,
			{ExitStatus: 0, IsError: false, Result: "", SessionId: "", Stage: "smine", Subtype: "", Timestamp: syncStamp},
		},

		bootstrap: &Bootstrap{results: appendResults, stderr: &bytes.Buffer{}},
		output:    nil,
		stage:     "smine",
		status:    0,
	})

	// Run tests
	for _, test := range tests {
		synctest.Test(t, func(t *testing.T) {
			test.bootstrap.recordResult(test.output, test.stage, test.status)

			assert.Equal(t, test._expectedRows, readBootstrapResults(t, test.bootstrap.results), test._id)
		})
	}
}

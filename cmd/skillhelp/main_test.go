package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/sessionstate"
)

const fixtureSkillMd = `---
name: fixture
description: Fixture skill. Trigger on /fixture. Args — topic: slice to load.
version: 1.0
---

# Fixture

## Routing

**Purpose:** fixture purpose.

**Disambiguation:**

| For | Use |
| :--- | :--- |
| other | /other |

**Preconditions:**
- none

**Pipeline:** standalone.

## Args

- ` + "`topic`" + `: slice to load
`

const fixtureAnswer = "# Session messages\n\nType `#disable heartbeat` to mute heartbeat.\n"

const fixtureTests = "Smoke: make skill-smoke SKILL=fixture TOKEN=<label>\nEval: /skillroutine-eval fixture\n"

const fixtureMap = `{"schemaVersion": "1.0", "skill": "fixture", "lanes": {"default": ["RULE-SKILL-016"]}}`

const fixtureTitle = `{
  "schema": 2,
  "skill": "titled",
  "version": "1.0",
  "states": {
    "CODE-REVIEWABLE": {
      "entry": "SKILL-FDESIGN-CODE-005",
      "meaning": "code gate presented — approve the Changes or request a change"
    },
    "DESIGN-READY": {
      "entry": "SKILL-FDESIGN-GATE-001",
      "meaning": "design gate presented — approve or revise the design"
    }
  }
}`

func TestRun(t *testing.T) {
	type testCase struct {
		_id                   string
		_expectedBlock        bool
		_expectedExit         int
		_expectedReasonSuffix string
		_expectedSections     []string

		stdin string
	}

	dir := t.TempDir()
	skillDir := filepath.Join(dir, "fixture")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(fixtureSkillMd), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "context-map.json"), []byte(fixtureMap), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "help-test.md"), []byte(fixtureTests), 0o644))

	titledDir := filepath.Join(dir, "titled")
	require.NoError(t, os.MkdirAll(titledDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(titledDir, "SKILL.md"), []byte(fixtureSkillMd), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(titledDir, "title.json"), []byte(fixtureTitle), 0o644))

	answeredSkillMd := strings.Replace(fixtureSkillMd, "version: 1.0\n", "version: 1.0\nanswered-by: hook\n", 1)
	answeredDir := filepath.Join(dir, "answered")
	require.NoError(t, os.MkdirAll(answeredDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(answeredDir, "SKILL.md"), []byte(answeredSkillMd), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(answeredDir, "answer.md"), []byte(fixtureAnswer), 0o644))

	brokenAnsweredDir := filepath.Join(dir, "answered-broken")
	require.NoError(t, os.MkdirAll(brokenAnsweredDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(brokenAnsweredDir, "SKILL.md"), []byte(answeredSkillMd), 0o644))

	catalog := sessionstate.Help()
	require.Len(t, catalog, 13)
	featureLines := make([]string, 0, len(catalog))
	for _, entry := range catalog {
		featureLines = append(featureLines, "- `"+string(entry.Feature)+"` — "+entry.Does+" ("+entry.Hook+")")
	}
	home := t.TempDir()
	hookConfig := defaultHookConfig(t)
	writeHookConfig(t, home, hookConfig)
	featuresLead := hookConfig.Skillhelp.FeaturesLead
	answeredReason := strings.TrimRight(fixtureAnswer, "\n") + "\n\n## Features\n\n" + featuresLead + "\n\n" + strings.Join(featureLines, "\n\n") + "\n"
	answeredSections := []string{"# Session messages", "\n\n## Features\n\n" + featuresLead + "\n\n- `"}

	helpSections := []string{"Frontmatter", "Routing", "Args", "\nTests\n", "  Smoke: make skill-smoke SKILL=fixture TOKEN=<label>", "  Eval: /skillroutine-eval fixture", "Topics"}
	titledSections := []string{"Frontmatter", "Routing", "Args", "Title states (title.json)", "CODE-REVIEWABLE", "DESIGN-READY"}

	envelope := func(prompt string) string {
		raw, err := json.Marshal(map[string]string{"cwd": dir, "hook_event_name": "UserPromptSubmit", "prompt": prompt})
		require.NoError(t, err)
		return string(raw)
	}

	tests := make([]*testCase, 0)

	for _, token := range []string{"--help", "-help", "-h", "--h", "—help", "–h", "—h"} {
		// help-<token>
		tests = append(tests, &testCase{
			_id:                   "help-" + token,
			_expectedBlock:        true,
			_expectedExit:         0,
			_expectedReasonSuffix: "",
			_expectedSections:     helpSections,

			stdin: envelope("/fixture " + token),
		})
	}

	// help-first-line-of-merged-prompt
	tests = append(tests, &testCase{
		_id:                   "help-first-line-of-merged-prompt",
		_expectedBlock:        true,
		_expectedExit:         0,
		_expectedReasonSuffix: "",
		_expectedSections:     helpSections,

		stdin: envelope("/fixture -h\n\n/fixture --h"),
	})

	// skill-with-title-json
	tests = append(tests, &testCase{
		_id:                   "skill-with-title-json",
		_expectedBlock:        true,
		_expectedExit:         0,
		_expectedReasonSuffix: "",
		_expectedSections:     titledSections,

		stdin: envelope("/titled --help"),
	})

	// help-not-on-first-line-passes
	tests = append(tests, &testCase{
		_id:                   "help-not-on-first-line-passes",
		_expectedBlock:        false,
		_expectedExit:         0,
		_expectedReasonSuffix: "",
		_expectedSections:     nil,

		stdin: envelope("/fixture do the thing\n\n/fixture --help"),
	})

	// plain-invocation-passes
	tests = append(tests, &testCase{
		_id:                   "plain-invocation-passes",
		_expectedBlock:        false,
		_expectedExit:         0,
		_expectedReasonSuffix: "",
		_expectedSections:     nil,

		stdin: envelope("/fixture do the thing"),
	})

	// help-suffix-passes
	tests = append(tests, &testCase{
		_id:                   "help-suffix-passes",
		_expectedBlock:        false,
		_expectedExit:         0,
		_expectedReasonSuffix: "",
		_expectedSections:     nil,

		stdin: envelope("/fixture --helpx"),
	})

	// missing-skill-passes
	tests = append(tests, &testCase{
		_id:                   "missing-skill-passes",
		_expectedBlock:        false,
		_expectedExit:         0,
		_expectedReasonSuffix: "",
		_expectedSections:     nil,

		stdin: envelope("/missing --help"),
	})

	// non-slash-passes
	tests = append(tests, &testCase{
		_id:                   "non-slash-passes",
		_expectedBlock:        false,
		_expectedExit:         0,
		_expectedReasonSuffix: "",
		_expectedSections:     nil,

		stdin: envelope("just --help me"),
	})

	// malformed-json-fails
	tests = append(tests, &testCase{
		_id:                   "malformed-json-fails",
		_expectedBlock:        false,
		_expectedExit:         1,
		_expectedReasonSuffix: "",
		_expectedSections:     nil,

		stdin: "{not json",
	})

	// answered-bare-prompt-blocks
	tests = append(tests, &testCase{
		_id:                   "answered-bare-prompt-blocks",
		_expectedBlock:        true,
		_expectedExit:         0,
		_expectedReasonSuffix: "",
		_expectedSections:     answeredSections,

		stdin: envelope("/answered"),
	})

	// answered-with-args-blocks
	tests = append(tests, &testCase{
		_id:                   "answered-with-args-blocks",
		_expectedBlock:        true,
		_expectedExit:         0,
		_expectedReasonSuffix: "",
		_expectedSections:     answeredSections,

		stdin: envelope("/answered how do switches work"),
	})

	// answered-help-flag-renders-help
	tests = append(tests, &testCase{
		_id:                   "answered-help-flag-renders-help",
		_expectedBlock:        true,
		_expectedExit:         0,
		_expectedReasonSuffix: "",
		_expectedSections:     []string{"Frontmatter", "Routing", "Args", "answered-by"},

		stdin: envelope("/answered --help"),
	})

	// answered-missing-answer-md-exit-1
	tests = append(tests, &testCase{
		_id:                   "answered-missing-answer-md-exit-1",
		_expectedBlock:        false,
		_expectedExit:         1,
		_expectedReasonSuffix: "",
		_expectedSections:     nil,

		stdin: envelope("/answered-broken"),
	})

	// unanswered-skill-passes
	tests = append(tests, &testCase{
		_id:                   "unanswered-skill-passes",
		_expectedBlock:        false,
		_expectedExit:         0,
		_expectedReasonSuffix: "",
		_expectedSections:     nil,

		stdin: envelope("/fixture"),
	})

	// unknown-skill-passes
	tests = append(tests, &testCase{
		_id:                   "unknown-skill-passes",
		_expectedBlock:        false,
		_expectedExit:         0,
		_expectedReasonSuffix: "",
		_expectedSections:     nil,

		stdin: envelope("/missing do the thing"),
	})

	// answer-ends-with-feature-catalog
	tests = append(tests, &testCase{
		_id:                   "answer-ends-with-feature-catalog",
		_expectedBlock:        true,
		_expectedExit:         0,
		_expectedReasonSuffix: answeredReason,
		_expectedSections:     []string{answeredReason},

		stdin: envelope("/answered"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			exit := run([]string{"-claude-home", home, "-skills", dir}, strings.NewReader(test.stdin), stdout, stderr)
			assert.Equal(t, test._expectedExit, exit, stderr.String())
			assert.Equal(t, test._expectedBlock, stdout.Len() > 0)
			if test._expectedBlock {
				output := &HookOutput{}
				require.NoError(t, json.Unmarshal(stdout.Bytes(), output))
				assert.Equal(t, decisionBlock, output.Decision)
				assert.True(t, strings.HasSuffix(output.Reason, test._expectedReasonSuffix), output.Reason)
				for _, section := range test._expectedSections {
					assert.Contains(t, output.Reason, section)
				}
			}
		})
	}
}

func TestRun_HookConfig(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedExit   int
		_expectedStderr string
		_expectedStdout string

		args  []string
		stdin string
	}

	dir := t.TempDir()
	skillDir := filepath.Join(dir, "fixture")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(fixtureSkillMd), 0o644))

	customPatternHome := t.TempDir()
	customPatternConfig := defaultHookConfig(t)
	customPatternConfig.Skillhelp.HelpPattern = `^\s*/([A-Za-z0-9_-]+)[ \t]+\?[ \t]*$`
	writeHookConfig(t, customPatternHome, customPatternConfig)

	missingHome := t.TempDir()

	envelope := func(prompt string) string {
		raw, err := json.Marshal(map[string]string{"cwd": dir, "hook_event_name": "UserPromptSubmit", "prompt": prompt})
		require.NoError(t, err)
		return string(raw)
	}

	tests := make([]*testCase, 0)

	// help-pattern-from-config
	tests = append(tests, &testCase{
		_id:             "help-pattern-from-config",
		_expectedExit:   0,
		_expectedStderr: "",
		_expectedStdout: `"decision":"block"`,

		args:  []string{"-claude-home", customPatternHome, "-skills", dir},
		stdin: envelope("/fixture ?"),
	})

	// default-help-token-unknown-to-custom-pattern
	tests = append(tests, &testCase{
		_id:             "default-help-token-unknown-to-custom-pattern",
		_expectedExit:   0,
		_expectedStderr: "",
		_expectedStdout: "",

		args:  []string{"-claude-home", customPatternHome, "-skills", dir},
		stdin: envelope("/fixture --help"),
	})

	// missing-config-errors
	tests = append(tests, &testCase{
		_id:             "missing-config-errors",
		_expectedExit:   1,
		_expectedStderr: hookconfig.Path(missingHome),
		_expectedStdout: "",

		args:  []string{"-claude-home", missingHome, "-skills", dir},
		stdin: envelope("/fixture --help"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			exit := run(test.args, strings.NewReader(test.stdin), stdout, stderr)
			assert.Equal(t, test._expectedExit, exit, stderr.String())
			assert.Contains(t, stderr.String(), test._expectedStderr)
			assert.Contains(t, stdout.String(), test._expectedStdout)
			assert.Equal(t, test._expectedStdout == "", stdout.Len() == 0)
		})
	}
}

// defaultHookConfig is the hooks config the install renders from every catalog default.
func defaultHookConfig(t *testing.T) *hookconfig.Config {
	t.Helper()
	hookConfig := &hookconfig.Config{}
	for _, component := range components.Catalog() {
		rendered, err := install.RenderDefaults(component.ID, components.SinkHooksConfig)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(rendered, hookConfig))
	}
	return hookConfig
}

// writeHookConfig writes the config where hookconfig.Load reads it.
func writeHookConfig(t *testing.T, home string, hookConfig *hookconfig.Config) {
	t.Helper()
	encoded, err := json.Marshal(hookConfig)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(hookconfig.Path(home)), 0o755))
	require.NoError(t, os.WriteFile(hookconfig.Path(home), encoded, 0o644))
}

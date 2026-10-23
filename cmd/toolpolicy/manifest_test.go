package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/skillenv"
)

// writeEnvFixture deploys a skill's env.json sidecar from its document value.
func writeEnvFixture(t *testing.T, claudeHome string, document *skillenv.Document) {
	t.Helper()
	dir := filepath.Join(claudeHome, "skills", document.Skill)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	content, err := json.Marshal(document)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, skillenv.FileName), content, 0o644))
}

// writeFrontmatterFixture deploys a fixture skill whose frontmatter carries extra lines after its allowed-tools line.
func writeFrontmatterFixture(t *testing.T, home string, name string, extra string) {
	t.Helper()
	dir := filepath.Join(home, ".claude", "skills", name)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	manifest := "---\nname: " + name + "\ndescription: fixture\nallowed-tools: Read, Bash(jq *)\n" + extra + "---\n\n# fixture\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(manifest), 0o644))
}

func TestReadSkillManifest(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *skillManifest

		skill  string
		values map[string]string
	}

	home := t.TempDir()
	writeFrontmatterFixture(t, home, "placed", "docker: always\nenable-delegation: never\nenforce-permissions: acceptEdits, planMode\n")
	writeSkillFixture(t, "Read, Bash(jq *)", "", home, "plain")
	writeSkillFixture(t, "Read, Bash($SKILL_X_CHECK)", "", home, "deferred")
	jqRules := parseRules([]string{"Read", "Bash(jq *)"}, "fixture")
	checkValues := map[string]string{"SKILL_X_CHECK": "make audit"}
	tests := make([]*testCase, 0)

	// placement-lines-ignored
	tests = append(tests, &testCase{
		_id:       "placement-lines-ignored",
		_expected: &skillManifest{Rules: jqRules, Skill: "placed"},

		skill:  "placed",
		values: nil,
	})

	// missing-skill-md
	tests = append(tests, &testCase{
		_id:       "missing-skill-md",
		_expected: nil,

		skill:  "ghost",
		values: nil,
	})

	// plain
	tests = append(tests, &testCase{
		_id:       "plain",
		_expected: &skillManifest{Rules: jqRules, Skill: "plain"},

		skill:  "plain",
		values: nil,
	})

	// deferred-rule-filled
	filledRules := parseRules([]string{"Read", "Bash(make audit)"}, "fixture")
	tests = append(tests, &testCase{
		_id:       "deferred-rule-filled",
		_expected: &skillManifest{values: checkValues, Rules: filledRules, Skill: "deferred"},

		skill:  "deferred",
		values: checkValues,
	})

	// deferred-rule-multiline-one-rule-per-line
	multiLineValues := map[string]string{"SKILL_X_CHECK": "make audit\ngo clean -modcache"}
	multiLineRules := parseRules([]string{"Read", "Bash(make audit)", "Bash(go clean -modcache)"}, "fixture")
	tests = append(tests, &testCase{
		_id:       "deferred-rule-multiline-one-rule-per-line",
		_expected: &skillManifest{values: multiLineValues, Rules: multiLineRules, Skill: "deferred"},

		skill:  "deferred",
		values: multiLineValues,
	})

	// deferred-rule-unfilled-dropped
	readRules := parseRules([]string{"Read"}, "fixture")
	tests = append(tests, &testCase{
		_id:       "deferred-rule-unfilled-dropped",
		_expected: &skillManifest{Rules: readRules, Skill: "deferred"},

		skill:  "deferred",
		values: nil,
	})

	// install-rule-untouched
	tests = append(tests, &testCase{
		_id:       "install-rule-untouched",
		_expected: &skillManifest{values: checkValues, Rules: jqRules, Skill: "plain"},

		skill:  "plain",
		values: checkValues,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, readSkillManifest(home, test.skill, test.values))
		})
	}
}

func TestSessionValues(t *testing.T) {
	type testCase struct {
		_id       string
		_expected map[string]string

		anchor     string
		claudeHome string
		events     []skillEvent
		registry   *repos.Registry
	}

	base := t.TempDir()
	anchor := filepath.Join(base, "repo")
	claudeHome := filepath.Join(base, "claude")
	for _, dir := range []string{filepath.Join(anchor, ".git"), filepath.Join(claudeHome, "smine")} {
		require.NoError(t, os.MkdirAll(dir, 0o755))
	}

	installSkills := map[string]string{
		"SKILL_X_BUILD": "go build ./...",
		"SKILL_X_CHECK": "make audit",
	}
	record := install.FullRecord(runtime.GOOS)
	record.Skills.Defaults = map[string]string{"check": "go vet ./...", "test": "go test ./..."}
	record.Skills.Variables = installSkills
	require.NoError(t, install.Save(install.Env{ClaudeHome: claudeHome, GOOS: runtime.GOOS}, record))

	checkEntry := &skillenv.Entry{
		Example:   "make audit",
		Interface: "check",
		Name:      "SKILL_X_CHECK",
		Required:  true,
		Scope:     "repo",
		Summary:   "Check command",
	}
	checkDocument := &skillenv.Document{
		Schema:    skillenv.DocumentSchema,
		Skill:     "x",
		Variables: []*skillenv.Entry{checkEntry},
		Version:   "1.0.0",
	}
	writeEnvFixture(t, claudeHome, checkDocument)
	testEntry := &skillenv.Entry{
		Example:   "go test ./...",
		Interface: "test",
		Name:      "SKILL_Y_TEST",
		Required:  true,
		Scope:     "repo",
		Summary:   "Test command",
	}
	testDocument := &skillenv.Document{
		Schema:    skillenv.DocumentSchema,
		Skill:     "y",
		Variables: []*skillenv.Entry{testEntry},
		Version:   "1.0.0",
	}
	writeEnvFixture(t, claudeHome, testDocument)

	reposPath := filepath.Join(base, "repos.json")
	rowEnv := map[string]string{"SKILL_X_CHECK": "make test"}
	row := repos.Repo{Env: rowEnv, Name: "repo", Path: anchor}
	require.NoError(t, repos.NewRegistry(reposPath).Add(row))
	registry := loadRegistry(reposPath)
	require.NotNil(t, registry)

	tests := make([]*testCase, 0)

	// registry-row-over-record
	rowOverInstall := map[string]string{
		"SKILL_X_BUILD": "go build ./...",
		"SKILL_X_CHECK": "make test",
	}
	tests = append(tests, &testCase{
		_id:       "registry-row-over-record",
		_expected: rowOverInstall,

		anchor:     anchor,
		claudeHome: claudeHome,
		events:     nil,
		registry:   registry,
	})

	// no-registry-record-only
	tests = append(tests, &testCase{
		_id:       "no-registry-record-only",
		_expected: installSkills,

		anchor:     anchor,
		claudeHome: claudeHome,
		events:     nil,
		registry:   nil,
	})

	// nothing-empty
	tests = append(tests, &testCase{
		_id:       "nothing-empty",
		_expected: map[string]string{},

		anchor:     anchor,
		claudeHome: filepath.Join(base, "no-record"),
		events:     nil,
		registry:   nil,
	})

	// global-default-through-sidecar
	globalTest := map[string]string{
		"SKILL_X_BUILD": "go build ./...",
		"SKILL_X_CHECK": "make audit",
		"SKILL_Y_TEST":  "go test ./...",
	}
	testEvents := []skillEvent{{isPromptNamed: true, isToolCall: false, skill: "y"}}
	tests = append(tests, &testCase{
		_id:       "global-default-through-sidecar",
		_expected: globalTest,

		anchor:     anchor,
		claudeHome: claudeHome,
		events:     testEvents,
		registry:   nil,
	})

	// skill-default-over-global
	checkEvents := []skillEvent{{isPromptNamed: true, isToolCall: false, skill: "x"}}
	tests = append(tests, &testCase{
		_id:       "skill-default-over-global",
		_expected: installSkills,

		anchor:     anchor,
		claudeHome: claudeHome,
		events:     checkEvents,
		registry:   nil,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, sessionValues(test.anchor, test.claudeHome, test.events, test.registry))
		})
	}
}

package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/trash"
)

const (
	foreignGroup       = `{"hooks": [{"type": "command", "command": "my-own-hook"}]}`
	stopGroup          = `{"smineId": "session-helpers-stop-play-sound", "smineName": "Stop sound", "smineVersion": "1.0.0", "hooks": [{"type": "command", "command": "bash ~/.claude/hooks/play-sound.sh stop", "async": true}]}`
	stopGroupOff       = `{"smineId": "session-helpers-stop-play-sound", "smineName": "Stop sound", "smineReason": "component off", "smineVersion": "1.0.0", "hooks": [{"type": "command", "command": "bash ~/.claude/hooks/play-sound.sh stop", "async": true}]}`
	stopGroupV2        = `{"smineId": "session-helpers-stop-play-sound", "smineName": "Stop sound", "smineVersion": "1.0.1", "hooks": [{"type": "command", "command": "bash ~/.claude/hooks/play-sound.sh stop", "async": true}]}`
	testSettingsPath   = "settings.json"
	testClaudeJsonPath = ".claude.json"
)

// journalLines reads a store's journal oldest first as "<kind> <path>" lines; nil when nothing was journaled.
func journalLines(t *testing.T, store *trash.Store) []string {
	t.Helper()
	entries, err := store.Tail(100)
	require.NoError(t, err)

	var lines []string
	for _, entry := range entries {
		lines = append(lines, string(entry.Kind)+" "+entry.Path)
	}
	slices.Reverse(lines)
	return lines
}

// stashedBytes reads the stash of the newest journal entry whose path is path.
func stashedBytes(t *testing.T, env Env, path string) string {
	t.Helper()
	entries, err := env.Trash.Tail(100)
	require.NoError(t, err)

	at := slices.IndexFunc(entries, func(entry *trash.Entry) bool { return entry.Path == path && entry.Stash != "" })
	require.GreaterOrEqual(t, at, 0, "no stash of %s", path)
	data, err := os.ReadFile(filepath.Join(env.ClaudeHome, "smine", trash.TrashDir, filepath.FromSlash(entries[at].Stash)))
	require.NoError(t, err)
	return string(data)
}

// decodeFragment parses one fragment literal.
func decodeFragment(t *testing.T, content string) *config.Fragment {
	t.Helper()
	fragment := &config.Fragment{}
	require.NoError(t, json.Unmarshal([]byte(content), fragment))
	return fragment
}

// encodeDoc marshals one document for a JSONEq comparison.
func encodeDoc(doc any) string {
	data, _ := json.Marshal(doc)
	return string(data)
}

// newSettingsFiles builds in-memory live documents from literals, journaling into a store under a temp dir.
func newSettingsFiles(t *testing.T, settings, claudeJson, parked string) *settingsFiles {
	t.Helper()
	files := &settingsFiles{
		claudeJson:     config.NewSettings(),
		claudeJsonPath: testClaudeJsonPath,
		parked:         map[string][]config.HookGroup{},
		settings:       config.NewSettings(),
		settingsPath:   testSettingsPath,
		trash:          trash.New(t.TempDir(), 0),
	}
	require.NoError(t, json.Unmarshal([]byte(settings), files.settings.Doc()))
	require.NoError(t, json.Unmarshal([]byte(claudeJson), files.claudeJson.Doc()))
	require.NoError(t, json.Unmarshal([]byte(parked), &files.parked))
	return files
}

// TestApplySettings_BaseKeysAbsentOnly keeps every live base value, adds the template's missing rules and keeps live extras.
func TestApplySettings_BaseKeysAbsentOnly(t *testing.T) {
	env := newTestEnv(t)
	writeTestFile(t, env.RepoDir, "settings/claude_code/base.json", `{"model": "claude-fable-5-1", "availableModels": ["a", "b"], "theme": "dark", "permissions": {"allow": ["Bash(mine *)"], "defaultMode": "plan"}}`)
	writeTestFile(t, env.Home, ".claude/settings.json", `{"model": "fable", "availableModels": ["a"], "permissions": {"allow": ["Read"], "defaultMode": "acceptEdits"}, "mine": true}`)

	changes, err := ApplySettings(env, FullRecord(goosDarwin))
	require.NoError(t, err)
	assert.Equal(t, []string{"base theme added", `"Bash(mine *)"`}, changes)

	live, err := os.ReadFile(filepath.Join(env.ClaudeHome, "settings.json"))
	require.NoError(t, err)
	assert.JSONEq(t, `{"model": "fable", "availableModels": ["a"], "theme": "dark", "permissions": {"allow": ["Read", "Bash(mine *)"], "defaultMode": "acceptEdits"}, "mine": true}`, string(live))
}

func TestForcePreview(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *ForceReport
		_isEmpty  bool

		env Env
	}

	baseTemplate := `{"model": "m", "env": {"A": "1"}}`
	fragment := `{"hooks": {"Stop": [` + stopGroupV2 + `]}, "permissions": {"allow": ["Bash(jq *)"]}, "env": {"B": "2"}}`
	oldGroup := `{"smineId": "session-helpers-stop-old", "smineName": "Stop old", "smineVersion": "1.0.0", "hooks": [{"type": "command", "command": "old"}]}`
	tests := make([]*testCase, 0)

	// drops-listed-by-kind
	dropsEnv := newTestEnv(t)
	writeTestFile(t, dropsEnv.RepoDir, "settings/claude_code/base.json", baseTemplate)
	writeTestFile(t, dropsEnv.RepoDir, "settings/claude_code/components/session-helpers.json", fragment)
	writeTestFile(t, dropsEnv.Home, ".claude/settings.json", `{"model": "old", "stale": true, "env": {"A": "x", "MINE": "y"}, "permissions": {"allow": ["Read", "Bash(jq *)"], "additionalDirectories": ["/old"]}, "hooks": {"Stop": [`+foreignGroup+`, `+stopGroup+`, `+oldGroup+`]}}`)
	writeTestFile(t, dropsEnv.Home, ".claude/hooks.disabled.json", `{"Stop": [`+stopGroup+`]}`)
	tests = append(tests, &testCase{
		_id: "drops-listed-by-kind",
		_expected: &ForceReport{
			EnvKeys:     []string{"MINE"},
			Groups:      []string{"Stop my-own-hook", "Stop Stop old"},
			Keys:        []string{"stale"},
			Parked:      []string{"Stop Stop sound"},
			Permissions: []string{`permissions.additionalDirectories /old`, `permissions.allow Read`},
		},
		_isEmpty: false,

		env: dropsEnv,
	})

	// nothing-dropped
	cleanEnv := newTestEnv(t)
	writeTestFile(t, cleanEnv.RepoDir, "settings/claude_code/base.json", baseTemplate)
	writeTestFile(t, cleanEnv.RepoDir, "settings/claude_code/components/session-helpers.json", fragment)
	writeTestFile(t, cleanEnv.Home, ".claude/settings.json", `{"model": "mine", "env": {"A": "1", "B": "2"}, "hooks": {"Stop": [`+stopGroup+`]}, "permissions": {"allow": ["Bash(jq *)"]}}`)
	tests = append(tests, &testCase{
		_id:       "nothing-dropped",
		_expected: &ForceReport{},
		_isEmpty:  true,

		env: cleanEnv,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			report, err := ForcePreview(test.env, customRecord(components.SessionHelpers))
			assert.NoError(t, err)
			assert.Equal(t, test._expected, report)
			assert.Equal(t, test._isEmpty, report.IsEmpty())
		})
	}
}

func TestSaveParked_KeepsPrevious(t *testing.T) {
	env := newTestEnv(t)
	writeTestFile(t, env.Home, ".claude/hooks.disabled.json", `{"Stop": [`+stopGroup+`]}`)

	require.NoError(t, SaveParked(env, map[string][]config.HookGroup{}))

	parked, err := os.ReadFile(ParkedPath(env))
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(parked))
	assert.JSONEq(t, `{"Stop": [`+stopGroup+`]}`, stashedBytes(t, env, ParkedPath(env)))
	assert.Equal(t, []string{"replaced " + ParkedPath(env)}, journalLines(t, env.Trash))
}

func TestSaveParked_DropsEmptyEvents(t *testing.T) {
	env := newTestEnv(t)
	var stop config.HookGroup
	require.NoError(t, json.Unmarshal([]byte(stopGroup), &stop))
	parked := map[string][]config.HookGroup{"CwdChanged": {}, "Stop": {stop}}

	require.NoError(t, SaveParked(env, parked))

	saved, err := os.ReadFile(ParkedPath(env))
	require.NoError(t, err)
	assert.JSONEq(t, `{"Stop": [`+stopGroup+`]}`, string(saved))
	assert.Contains(t, parked, "CwdChanged", "the caller's map is left as it was")
}

// TestForceSettings rebuilds the live file from base plus the selected fragments; live extras, idless hook groups and parked hooks go, ~/.claude.json keeps its own servers.
func TestForceSettings(t *testing.T) {
	env := newTestEnv(t)
	writeTestFile(t, env.RepoDir, "settings/claude_code/base.json", `{"model": "m", "env": {"A": "1"}}`)
	writeTestFile(t, env.RepoDir, "settings/claude_code/components/session-helpers.json", `{"hooks": {"Stop": [`+stopGroupV2+`]}, "permissions": {"allow": ["Bash(jq *)"]}, "env": {"B": "2"}, "mcpServers": {"peek-mcp": {"command": "peek"}}}`)
	writeTestFile(t, env.RepoDir, "settings/claude_code/components/peek.json", `{"env": {"C": "3"}}`)
	writeTestFile(t, env.Home, ".claude/settings.json", `{"model": "old", "stale": true, "env": {"A": "x", "MINE": "y"}, "permissions": {"allow": ["Read", "Bash(jq *)"], "additionalDirectories": ["/old"]}, "hooks": {"Stop": [`+foreignGroup+`, `+stopGroup+`]}}`)
	writeTestFile(t, env.Home, ".claude/hooks.disabled.json", `{"Stop": [`+stopGroup+`]}`)
	writeTestFile(t, env.Home, ".claude.json", `{"oauthAccount": {"id": "x"}, "mcpServers": {"serena": {"command": "serena"}}}`)

	changes, err := ForceSettings(env, customRecord(components.SessionHelpers))
	require.NoError(t, err)
	assert.Equal(t, []string{"settings.json rebuilt from the template: model, env, hooks, permissions", "parked hooks dropped", "mcpServers.peek-mcp"}, changes)

	live, err := os.ReadFile(filepath.Join(env.ClaudeHome, "settings.json"))
	require.NoError(t, err)
	assert.JSONEq(t, `{"model": "m", "env": {"A": "1", "B": "2"}, "hooks": {"Stop": [`+stopGroupV2+`]}, "permissions": {"allow": ["Bash(jq *)"]}}`, string(live))

	parked, err := os.ReadFile(ParkedPath(env))
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(parked))

	claudeJson, err := os.ReadFile(claudeJsonPath(env))
	require.NoError(t, err)
	assert.JSONEq(t, `{"oauthAccount": {"id": "x"}, "mcpServers": {"serena": {"command": "serena"}, "peek-mcp": {"command": "peek"}}}`, string(claudeJson))

	expectedJournal := []string{
		"replaced " + settingsPath(env),
		"replaced " + settingsPath(env),
		"replaced " + claudeJsonPath(env),
		"replaced " + ParkedPath(env),
	}
	assert.Equal(t, expectedJournal, journalLines(t, env.Trash))

	entries, err := env.Trash.Tail(100)
	require.NoError(t, err)
	assert.Equal(t, "force sync dropped 1 groups, 1 env keys, 2 permission entries, 1 keys, 1 parked groups", entries[len(entries)-1].Reason)
	assert.JSONEq(t, `{"Stop": [`+stopGroup+`]}`, stashedBytes(t, env, ParkedPath(env)))
	assert.JSONEq(t, `{"model": "old", "stale": true, "env": {"A": "x", "MINE": "y"}, "permissions": {"allow": ["Read", "Bash(jq *)"], "additionalDirectories": ["/old"]}, "hooks": {"Stop": [`+foreignGroup+`, `+stopGroup+`]}}`, stashedBytes(t, env, settingsPath(env)))
	assert.JSONEq(t, `{"oauthAccount": {"id": "x"}, "mcpServers": {"serena": {"command": "serena"}}}`, stashedBytes(t, env, claudeJsonPath(env)))
}

// TestApplySettings_Converges applies the repo's own fragments twice: the second pass changes nothing.
func TestApplySettings_Converges(t *testing.T) {
	env := newTestEnv(t)
	env.RepoDir = filepath.Join("..", "..")

	firstChanges, err := ApplySettings(env, FullRecord(goosDarwin))
	require.NoError(t, err)
	assert.NotEmpty(t, firstChanges)

	secondChanges, err := ApplySettings(env, FullRecord(goosDarwin))
	require.NoError(t, err)
	assert.Empty(t, secondChanges)
}

func TestDeclaredHookIds(t *testing.T) {
	type testCase struct {
		_id           string
		_expectedSome bool
		_shouldPass   bool

		env Env
	}

	tests := make([]*testCase, 0)

	// catalog-fragments-unique
	catalogEnv := newTestEnv(t)
	catalogEnv.RepoDir = filepath.Join("..", "..")
	tests = append(tests, &testCase{
		_id:           "catalog-fragments-unique",
		_expectedSome: true,
		_shouldPass:   true,

		env: catalogEnv,
	})

	// duplicate-id-errors
	duplicateEnv := newTestEnv(t)
	duplicateGroup := `{"smineId": "context-stop-x", "smineName": "Stop x", "smineVersion": "1.0.0", "hooks": [{"type": "command", "command": "x"}]}`
	writeTestFile(t, duplicateEnv.RepoDir, "settings/claude_code/components/context.json", `{"hooks": {"Stop": [`+duplicateGroup+`, `+duplicateGroup+`]}}`)
	tests = append(tests, &testCase{
		_id:           "duplicate-id-errors",
		_expectedSome: false,
		_shouldPass:   false,

		env: duplicateEnv,
	})

	// missing-fragment-dir-empty-set
	tests = append(tests, &testCase{
		_id:           "missing-fragment-dir-empty-set",
		_expectedSome: false,
		_shouldPass:   true,

		env: newTestEnv(t),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			declared, err := declaredHookIds(test.env)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedSome, len(declared) > 0)
		})
	}
}

func TestApplyBase(t *testing.T) {
	type testCase struct {
		_id       string
		_changed  []string
		_expected string

		base string
		live string
	}

	tests := make([]*testCase, 0)

	// absent-key-added
	tests = append(tests, &testCase{
		_id:       "absent-key-added",
		_changed:  []string{"base theme added"},
		_expected: `{"model": "mine", "theme": "dark"}`,

		base: `{"theme": "dark"}`,
		live: `{"model": "mine"}`,
	})

	// nested-absent-child-added
	tests = append(tests, &testCase{
		_id:       "nested-absent-child-added",
		_changed:  []string{"base env.B added"},
		_expected: `{"env": {"A": "mine", "B": "2", "MINE": "x"}}`,

		base: `{"env": {"A": "1", "B": "2"}}`,
		live: `{"env": {"A": "mine", "MINE": "x"}}`,
	})

	// present-scalar-kept
	tests = append(tests, &testCase{
		_id:       "present-scalar-kept",
		_changed:  nil,
		_expected: `{"theme": "light"}`,

		base: `{"theme": "dark"}`,
		live: `{"theme": "light"}`,
	})

	// present-array-kept
	tests = append(tests, &testCase{
		_id:       "present-array-kept",
		_changed:  nil,
		_expected: `{"availableModels": ["a"]}`,

		base: `{"availableModels": ["a", "b"]}`,
		live: `{"availableModels": ["a"]}`,
	})

	// equal-value-unchanged
	tests = append(tests, &testCase{
		_id:       "equal-value-unchanged",
		_changed:  nil,
		_expected: `{"availableModels": ["a", "b"], "env": {"A": "1"}}`,

		base: `{"env": {"A": "1"}, "availableModels": ["a", "b"]}`,
		live: `{"availableModels": ["a", "b"], "env": {"A": "1"}}`,
	})

	// live-only-key-kept
	tests = append(tests, &testCase{
		_id:       "live-only-key-kept",
		_changed:  nil,
		_expected: `{"theme": "dark", "mine": true}`,

		base: `{"theme": "dark"}`,
		live: `{"theme": "dark", "mine": true}`,
	})

	// owned-permission-lists-left-alone
	tests = append(tests, &testCase{
		_id:       "owned-permission-lists-left-alone",
		_changed:  []string{"base permissions.defaultMode added"},
		_expected: `{"permissions": {"allow": ["Read"], "defaultMode": "plan"}}`,

		base: `{"permissions": {"allow": ["Bash(mine *)"], "deny": ["Bash(rm *)"], "defaultMode": "plan"}}`,
		live: `{"permissions": {"allow": ["Read"]}}`,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			live, base := config.NewDocument(), config.NewDocument()
			require.NoError(t, json.Unmarshal([]byte(test.live), live))
			require.NoError(t, json.Unmarshal([]byte(test.base), base))

			changed, err := applyBase(live, base)
			assert.NoError(t, err)
			assert.Equal(t, test._changed, changed)
			assert.JSONEq(t, test._expected, encodeDoc(live))
		})
	}
}

func TestApplyBaseLists(t *testing.T) {
	type testCase struct {
		_id       string
		_changed  []string
		_expected string

		base string
		live string
	}

	tests := make([]*testCase, 0)

	// missing-entries-added-extras-kept
	tests = append(tests, &testCase{
		_id:       "missing-entries-added-extras-kept",
		_changed:  []string{`"Bash(mine *)"`, `"Bash(rm *)"`},
		_expected: `{"permissions": {"allow": ["Read", "Bash(mine *)"], "deny": ["Bash(rm *)"]}}`,

		base: `{"permissions": {"allow": ["Read", "Bash(mine *)"], "deny": ["Bash(rm *)"]}}`,
		live: `{"permissions": {"allow": ["Read"]}}`,
	})

	// no-base-lists-no-change
	tests = append(tests, &testCase{
		_id:       "no-base-lists-no-change",
		_changed:  nil,
		_expected: `{"permissions": {"allow": ["Read"]}}`,

		base: `{"permissions": {"defaultMode": "plan"}}`,
		live: `{"permissions": {"allow": ["Read"]}}`,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			live, base := config.NewDocument(), config.NewDocument()
			require.NoError(t, json.Unmarshal([]byte(test.live), live))
			require.NoError(t, json.Unmarshal([]byte(test.base), base))

			changed, err := applyBaseLists(live, base)
			assert.NoError(t, err)
			assert.Equal(t, test._changed, changed)
			assert.JSONEq(t, test._expected, encodeDoc(live))
		})
	}
}

func TestPrune(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedChanges  []string
		_expectedJournal  []string
		_expectedParked   string
		_expectedSettings string

		files   *settingsFiles
		retired map[string]bool
	}

	retired := map[string]bool{"session-helpers-stop-old": true}
	retiredGroup := `{"smineId": "session-helpers-stop-old", "smineName": "Stop old", "smineVersion": "1.0.0", "hooks": [{"type": "command", "command": "~/.claude/hooks/bin/sessiontitle -done"}]}`
	idlessBinGroup := `{"hooks": [{"type": "command", "command": "~/.claude/hooks/bin/readgate", "timeout": 5}]}`
	heartbeatHttpGroup := `{"hooks": [{"type": "http", "url": "http://127.0.0.1:6101/hook", "timeout": 1}]}`
	parkedRetiredGroup := `{"smineId": "session-helpers-stop-old", "smineName": "Stop old", "smineVersion": "1.0.0", "hooks": [{"type": "command", "command": "x"}]}`
	parkedIdlessBinGroup := `{"hooks": [{"type": "command", "command": "~/.claude/hooks/bin/readgate"}]}`
	tests := make([]*testCase, 0)

	// retired-id-removed
	tests = append(tests, &testCase{
		_id:               "retired-id-removed",
		_expectedChanges:  []string{"hook Stop session-helpers-stop-old retired"},
		_expectedJournal:  []string{"removed settings.json#hooks/Stop/session-helpers-stop-old"},
		_expectedParked:   `{}`,
		_expectedSettings: `{"hooks": {"Stop": [` + stopGroup + `]}}`,

		files:   newSettingsFiles(t, `{"hooks": {"Stop": [`+retiredGroup+`, `+stopGroup+`]}}`, `{}`, `{}`),
		retired: retired,
	})

	// undeclared-id-kept
	undeclaredGroup := `{"smineId": "session-helpers-stop-gone", "smineName": "Stop gone", "smineVersion": "1.0.0", "hooks": [{"type": "command", "command": "gone"}]}`
	tests = append(tests, &testCase{
		_id:               "undeclared-id-kept",
		_expectedChanges:  nil,
		_expectedJournal:  nil,
		_expectedParked:   `{}`,
		_expectedSettings: `{"hooks": {"Stop": [` + undeclaredGroup + `]}}`,

		files:   newSettingsFiles(t, `{"hooks": {"Stop": [`+undeclaredGroup+`]}}`, `{}`, `{}`),
		retired: retired,
	})

	// unlisted-id-kept
	tests = append(tests, &testCase{
		_id:               "unlisted-id-kept",
		_expectedChanges:  nil,
		_expectedJournal:  nil,
		_expectedParked:   `{}`,
		_expectedSettings: `{"hooks": {"Stop": [` + stopGroup + `]}}`,

		files:   newSettingsFiles(t, `{"hooks": {"Stop": [`+stopGroup+`]}}`, `{}`, `{}`),
		retired: retired,
	})

	// idless-foreign-kept
	tests = append(tests, &testCase{
		_id:               "idless-foreign-kept",
		_expectedChanges:  nil,
		_expectedJournal:  nil,
		_expectedParked:   `{}`,
		_expectedSettings: `{"hooks": {"Stop": [` + foreignGroup + `]}}`,

		files:   newSettingsFiles(t, `{"hooks": {"Stop": [`+foreignGroup+`]}}`, `{}`, `{}`),
		retired: retired,
	})

	// idless-smine-groups-kept
	tests = append(tests, &testCase{
		_id:               "idless-smine-groups-kept",
		_expectedChanges:  nil,
		_expectedJournal:  nil,
		_expectedParked:   `{}`,
		_expectedSettings: `{"hooks": {"Stop": [` + idlessBinGroup + `, ` + heartbeatHttpGroup + `, ` + stopGroup + `]}}`,

		files:   newSettingsFiles(t, `{"hooks": {"Stop": [`+idlessBinGroup+`, `+heartbeatHttpGroup+`, `+stopGroup+`]}}`, `{}`, `{}`),
		retired: retired,
	})

	// retired-parked-removed
	tests = append(tests, &testCase{
		_id:               "retired-parked-removed",
		_expectedChanges:  []string{"parked hook Stop session-helpers-stop-old retired"},
		_expectedJournal:  []string{"removed settings.json#parked/Stop/session-helpers-stop-old"},
		_expectedParked:   `{"Stop": []}`,
		_expectedSettings: `{}`,

		files:   newSettingsFiles(t, `{}`, `{}`, `{"Stop": [`+parkedRetiredGroup+`]}`),
		retired: retired,
	})

	// idless-parked-smine-kept
	tests = append(tests, &testCase{
		_id:               "idless-parked-smine-kept",
		_expectedChanges:  nil,
		_expectedJournal:  nil,
		_expectedParked:   `{"PreToolUse": [` + parkedIdlessBinGroup + `, ` + foreignGroup + `]}`,
		_expectedSettings: `{}`,

		files:   newSettingsFiles(t, `{}`, `{}`, `{"PreToolUse": [`+parkedIdlessBinGroup+`, `+foreignGroup+`]}`),
		retired: retired,
	})

	// no-hooks-key-nothing
	tests = append(tests, &testCase{
		_id:               "no-hooks-key-nothing",
		_expectedChanges:  nil,
		_expectedJournal:  nil,
		_expectedParked:   `{}`,
		_expectedSettings: `{"model": "m"}`,

		files:   newSettingsFiles(t, `{"model": "m"}`, `{}`, `{}`),
		retired: retired,
	})

	// emptied-event-key-removed
	tests = append(tests, &testCase{
		_id:               "emptied-event-key-removed",
		_expectedChanges:  []string{"hook Stop session-helpers-stop-old retired"},
		_expectedJournal:  []string{"removed settings.json#hooks/Stop/session-helpers-stop-old"},
		_expectedParked:   `{}`,
		_expectedSettings: `{"hooks": {}}`,

		files:   newSettingsFiles(t, `{"hooks": {"Stop": [`+retiredGroup+`]}}`, `{}`, `{}`),
		retired: retired,
	})

	// empty-list-prunes-nothing
	tests = append(tests, &testCase{
		_id:               "empty-list-prunes-nothing",
		_expectedChanges:  nil,
		_expectedJournal:  nil,
		_expectedParked:   `{"Stop": [` + parkedRetiredGroup + `]}`,
		_expectedSettings: `{"hooks": {"Stop": [` + idlessBinGroup + `, ` + retiredGroup + `, ` + heartbeatHttpGroup + `]}}`,

		files:   newSettingsFiles(t, `{"hooks": {"Stop": [`+idlessBinGroup+`, `+retiredGroup+`, `+heartbeatHttpGroup+`]}}`, `{}`, `{"Stop": [`+parkedRetiredGroup+`]}`),
		retired: map[string]bool{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			changes, err := test.files.prune(test.retired)
			assert.NoError(t, err)
			assert.Equal(t, test._expectedChanges, changes)
			assert.Equal(t, test._expectedJournal, journalLines(t, test.files.trash))
			assert.JSONEq(t, test._expectedSettings, encodeDoc(test.files.settings.Doc()))
			assert.JSONEq(t, test._expectedParked, encodeDoc(test.files.parked))
		})
	}
}

// TestReconcileSettings_PrunesOnlyRetired applies one component: every declared group of the other components stays live, only the listed retired id goes.
func TestReconcileSettings_PrunesOnlyRetired(t *testing.T) {
	env := newTestEnv(t)
	peekGroup := `{"smineId": "peek-stop-status", "smineName": "Stop status", "smineVersion": "1.0.0", "hooks": [{"type": "command", "command": "peek-status"}]}`
	retiredGroup := `{"smineId": "session-helpers-stop-old", "smineName": "Stop old", "smineVersion": "1.0.0", "hooks": [{"type": "command", "command": "old"}]}`
	writeTestFile(t, env.RepoDir, "settings/claude_code/components/session-helpers.json", `{"hooks": {"Stop": [`+stopGroup+`]}}`)
	writeTestFile(t, env.RepoDir, "settings/claude_code/components/peek.json", `{"hooks": {"Stop": [`+peekGroup+`]}}`)
	writeTestFile(t, env.RepoDir, "settings/claude_code/retired-hooks.json", `{"retired": ["session-helpers-stop-old"]}`)
	writeTestFile(t, env.Home, ".claude/settings.json", `{"hooks": {"Stop": [`+stopGroup+`, `+retiredGroup+`, `+foreignGroup+`, `+peekGroup+`]}}`)
	component := &artifactComponent{id: components.Peek}

	steps, err := component.reconcileSettings(env, true, true)
	require.NoError(t, err)
	assert.Equal(t, []Step{component.step(ActionInstall, "hook Stop session-helpers-stop-old retired")}, steps)

	live, err := os.ReadFile(settingsPath(env))
	require.NoError(t, err)
	assert.JSONEq(t, `{"hooks": {"Stop": [`+stopGroup+`, `+foreignGroup+`, `+peekGroup+`]}}`, string(live))
}

func TestRetiredHookIds(t *testing.T) {
	listedEnv := newTestEnv(t)
	writeTestFile(t, listedEnv.RepoDir, "settings/claude_code/retired-hooks.json", `{"retired": ["context-stop-old"]}`)

	listed, err := retiredHookIds(listedEnv)
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"context-stop-old": true}, listed)

	missing, err := retiredHookIds(newTestEnv(t))
	require.NoError(t, err)
	assert.Empty(t, missing)
}

func TestReconcile(t *testing.T) {
	type testCase struct {
		_id                 string
		_expectedChanges    []string
		_expectedClaudeJson string
		_expectedJournal    []string
		_expectedParked     string
		_expectedSettings   string

		files      *settingsFiles
		fragment   *config.Fragment
		isSelected bool
	}

	stopFragment := decodeFragment(t, `{"hooks": {"Stop": [`+stopGroup+`]}}`)
	stopFragmentV2 := decodeFragment(t, `{"hooks": {"Stop": [`+stopGroupV2+`]}}`)
	stopGroupOther := `{"smineId": "session-helpers-stop-play-sound", "smineName": "Stop sound", "smineVersion": "1.0.0", "hooks": [{"type": "command", "command": "bash ~/.claude/hooks/play-sound.sh old"}]}`
	stopGroupOtherFields := `{"smineId": "session-helpers-stop-play-sound", "smineName": "Stop sound", "smineVersion": "1.0.0", "hooks": [{"type": "command", "command": "bash ~/.claude/hooks/play-sound.sh old", "async": true}]}`
	tests := make([]*testCase, 0)

	// selected-adds-missing-group
	tests = append(tests, &testCase{
		_id:                 "selected-adds-missing-group",
		_expectedChanges:    []string{"hook Stop session-helpers-stop-play-sound"},
		_expectedClaudeJson: `{}`,
		_expectedJournal:    nil,
		_expectedParked:     `{}`,
		_expectedSettings:   `{"hooks": {"Stop": [` + foreignGroup + `, ` + stopGroup + `]}}`,

		files:      newSettingsFiles(t, `{"hooks": {"Stop": [`+foreignGroup+`]}}`, `{}`, `{}`),
		fragment:   stopFragment,
		isSelected: true,
	})

	// selected-equal-group-no-change
	reordered := `{"hooks": [{"async": true, "command": "bash ~/.claude/hooks/play-sound.sh stop", "type": "command"}], "smineVersion": "1.0.0", "smineName": "Stop sound", "smineId": "session-helpers-stop-play-sound"}`
	tests = append(tests, &testCase{
		_id:                 "selected-equal-group-no-change",
		_expectedChanges:    nil,
		_expectedClaudeJson: `{}`,
		_expectedJournal:    nil,
		_expectedParked:     `{}`,
		_expectedSettings:   `{"hooks": {"Stop": [` + reordered + `]}}`,

		files:      newSettingsFiles(t, `{"hooks": {"Stop": [`+reordered+`]}}`, `{}`, `{}`),
		fragment:   stopFragment,
		isSelected: true,
	})

	// selected-lower-version-replaced-in-place
	tests = append(tests, &testCase{
		_id:                 "selected-lower-version-replaced-in-place",
		_expectedChanges:    []string{"hook Stop session-helpers-stop-play-sound v1.0.0→v1.0.1"},
		_expectedClaudeJson: `{}`,
		_expectedJournal:    []string{"replaced settings.json#hooks/Stop/session-helpers-stop-play-sound"},
		_expectedParked:     `{}`,
		_expectedSettings:   `{"hooks": {"Stop": [` + stopGroupV2 + `, ` + foreignGroup + `]}}`,

		files:      newSettingsFiles(t, `{"hooks": {"Stop": [`+stopGroupOther+`, `+foreignGroup+`]}}`, `{}`, `{}`),
		fragment:   stopFragmentV2,
		isSelected: true,
	})

	// legacy-integer-live-replaced
	legacyGroup := `{"smineId": "session-helpers-stop-play-sound", "smineVersion": 1, "hooks": [{"type": "command", "command": "bash ~/.claude/hooks/play-sound.sh stop", "async": true}]}`
	tests = append(tests, &testCase{
		_id:                 "legacy-integer-live-replaced",
		_expectedChanges:    []string{"hook Stop session-helpers-stop-play-sound v1.0.0→v1.0.1"},
		_expectedClaudeJson: `{}`,
		_expectedJournal:    []string{"replaced settings.json#hooks/Stop/session-helpers-stop-play-sound"},
		_expectedParked:     `{}`,
		_expectedSettings:   `{"hooks": {"Stop": [` + stopGroupV2 + `]}}`,

		files:      newSettingsFiles(t, `{"hooks": {"Stop": [`+legacyGroup+`]}}`, `{}`, `{}`),
		fragment:   stopFragmentV2,
		isSelected: true,
	})

	// selected-same-version-other-content-kept-fields-applied
	tests = append(tests, &testCase{
		_id:                 "selected-same-version-other-content-kept-fields-applied",
		_expectedChanges:    []string{"hook Stop session-helpers-stop-play-sound settings"},
		_expectedClaudeJson: `{}`,
		_expectedJournal:    nil,
		_expectedParked:     `{}`,
		_expectedSettings:   `{"hooks": {"Stop": [` + stopGroupOtherFields + `]}}`,

		files:      newSettingsFiles(t, `{"hooks": {"Stop": [`+stopGroupOther+`]}}`, `{}`, `{}`),
		fragment:   stopFragment,
		isSelected: true,
	})

	// selected-higher-version-kept
	tests = append(tests, &testCase{
		_id:                 "selected-higher-version-kept",
		_expectedChanges:    nil,
		_expectedClaudeJson: `{}`,
		_expectedJournal:    nil,
		_expectedParked:     `{}`,
		_expectedSettings:   `{"hooks": {"Stop": [` + stopGroupV2 + `]}}`,

		files:      newSettingsFiles(t, `{"hooks": {"Stop": [`+stopGroupV2+`]}}`, `{}`, `{}`),
		fragment:   stopFragment,
		isSelected: true,
	})

	// selected-parked-group-no-change
	tests = append(tests, &testCase{
		_id:                 "selected-parked-group-no-change",
		_expectedChanges:    nil,
		_expectedClaudeJson: `{}`,
		_expectedJournal:    nil,
		_expectedParked:     `{"Stop": [` + stopGroup + `]}`,
		_expectedSettings:   `{}`,

		files:      newSettingsFiles(t, `{}`, `{}`, `{"Stop": [`+stopGroup+`]}`),
		fragment:   stopFragment,
		isSelected: true,
	})

	// selected-parked-lower-version-replaced-stays-parked
	tests = append(tests, &testCase{
		_id:                 "selected-parked-lower-version-replaced-stays-parked",
		_expectedChanges:    []string{"parked hook Stop session-helpers-stop-play-sound v1.0.0→v1.0.1"},
		_expectedClaudeJson: `{}`,
		_expectedJournal:    nil,
		_expectedParked:     `{"Stop": [` + stopGroupV2 + `]}`,
		_expectedSettings:   `{}`,

		files:      newSettingsFiles(t, `{}`, `{}`, `{"Stop": [`+stopGroup+`]}`),
		fragment:   stopFragmentV2,
		isSelected: true,
	})

	// unselected-parks-with-reason
	tests = append(tests, &testCase{
		_id:                 "unselected-parks-with-reason",
		_expectedChanges:    []string{"hook Stop session-helpers-stop-play-sound parked (component off)"},
		_expectedClaudeJson: `{}`,
		_expectedJournal:    nil,
		_expectedParked:     `{"Stop": [` + stopGroupOff + `]}`,
		_expectedSettings:   `{"hooks": {"Stop": [` + foreignGroup + `]}}`,

		files:      newSettingsFiles(t, `{"hooks": {"Stop": [`+stopGroup+`, `+foreignGroup+`]}}`, `{}`, `{}`),
		fragment:   stopFragment,
		isSelected: false,
	})

	// unselected-keeps-user-park
	tests = append(tests, &testCase{
		_id:                 "unselected-keeps-user-park",
		_expectedChanges:    nil,
		_expectedClaudeJson: `{}`,
		_expectedJournal:    nil,
		_expectedParked:     `{"Stop": [` + stopGroup + `]}`,
		_expectedSettings:   `{}`,

		files:      newSettingsFiles(t, `{}`, `{}`, `{"Stop": [`+stopGroup+`]}`),
		fragment:   stopFragment,
		isSelected: false,
	})

	// reselect-unparks-component-off
	tests = append(tests, &testCase{
		_id:                 "reselect-unparks-component-off",
		_expectedChanges:    []string{"hook Stop session-helpers-stop-play-sound unparked (component on)"},
		_expectedClaudeJson: `{}`,
		_expectedJournal:    nil,
		_expectedParked:     `{"Stop": []}`,
		_expectedSettings:   `{"hooks": {"Stop": [` + foreignGroup + `, ` + stopGroup + `]}}`,

		files:      newSettingsFiles(t, `{"hooks": {"Stop": [`+foreignGroup+`]}}`, `{}`, `{"Stop": [`+stopGroupOff+`]}`),
		fragment:   stopFragment,
		isSelected: true,
	})

	// reselect-keeps-user-park
	tests = append(tests, &testCase{
		_id:                 "reselect-keeps-user-park",
		_expectedChanges:    nil,
		_expectedClaudeJson: `{}`,
		_expectedJournal:    nil,
		_expectedParked:     `{"Stop": [` + stopGroup + `]}`,
		_expectedSettings:   `{"hooks": {"Stop": [` + foreignGroup + `]}}`,

		files:      newSettingsFiles(t, `{"hooks": {"Stop": [`+foreignGroup+`]}}`, `{}`, `{"Stop": [`+stopGroup+`]}`),
		fragment:   stopFragment,
		isSelected: true,
	})

	// deselect-journals-equal-env-key
	tests = append(tests, &testCase{
		_id:                 "deselect-journals-equal-env-key",
		_expectedChanges:    []string{`"Bash(jq *)"`, "env.HEARTBEAT_ADDR"},
		_expectedClaudeJson: `{}`,
		_expectedJournal:    []string{"removed settings.json#permissions.allow", "removed settings.json#env"},
		_expectedParked:     `{}`,
		_expectedSettings:   `{"env": {}, "permissions": {"allow": ["Read"]}}`,

		files:      newSettingsFiles(t, `{"env": {"HEARTBEAT_ADDR": "127.0.0.1:6101"}, "permissions": {"allow": ["Read", "Bash(jq *)"]}}`, `{}`, `{}`),
		fragment:   decodeFragment(t, `{"env": {"HEARTBEAT_ADDR": "127.0.0.1:6101"}, "permissions": {"allow": ["Bash(jq *)"]}}`),
		isSelected: false,
	})

	// env-key-present-with-other-value-kept
	tests = append(tests, &testCase{
		_id:                 "env-key-present-with-other-value-kept",
		_expectedChanges:    nil,
		_expectedClaudeJson: `{}`,
		_expectedJournal:    nil,
		_expectedParked:     `{}`,
		_expectedSettings:   `{"env": {"HEARTBEAT_ADDR": "127.0.0.1:7000"}}`,

		files:      newSettingsFiles(t, `{"env": {"HEARTBEAT_ADDR": "127.0.0.1:7000"}}`, `{}`, `{}`),
		fragment:   decodeFragment(t, `{"env": {"HEARTBEAT_ADDR": "127.0.0.1:6101"}}`),
		isSelected: false,
	})

	// permission-and-additional-directory-lists
	tests = append(tests, &testCase{
		_id:                 "permission-and-additional-directory-lists",
		_expectedChanges:    []string{`"/h/.claude/skills"`, `"Bash(jq *)"`, `"Bash(git stash*)"`},
		_expectedClaudeJson: `{}`,
		_expectedJournal:    nil,
		_expectedParked:     `{}`,
		_expectedSettings:   `{"permissions": {"defaultMode": "acceptEdits", "allow": ["Bash(ls *)", "Bash(jq *)"], "additionalDirectories": ["/h/.claude/skills"], "deny": ["Bash(git stash*)"]}}`,

		files:      newSettingsFiles(t, `{"permissions": {"defaultMode": "acceptEdits", "allow": ["Bash(ls *)"]}}`, `{}`, `{}`),
		fragment:   decodeFragment(t, `{"permissions": {"additionalDirectories": ["/h/.claude/skills"], "allow": ["Bash(ls *)", "Bash(jq *)"], "deny": ["Bash(git stash*)"]}}`),
		isSelected: true,
	})

	// mcp-server-to-claude-json
	tests = append(tests, &testCase{
		_id:                 "mcp-server-to-claude-json",
		_expectedChanges:    []string{"mcpServers.peek-mcp"},
		_expectedClaudeJson: `{"oauthAccount": {"id": "x"}, "mcpServers": {"peek-mcp": {"command": "/h/go/bin/peek-mcp"}}}`,
		_expectedJournal:    nil,
		_expectedParked:     `{}`,
		_expectedSettings:   `{}`,

		files:      newSettingsFiles(t, `{}`, `{"oauthAccount": {"id": "x"}}`, `{}`),
		fragment:   decodeFragment(t, `{"mcpServers": {"peek-mcp": {"command": "/h/go/bin/peek-mcp"}}}`),
		isSelected: true,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			changes, err := test.files.reconcile(test.fragment, test.isSelected)
			assert.NoError(t, err)
			assert.Equal(t, test._expectedChanges, changes)
			assert.Equal(t, test._expectedJournal, journalLines(t, test.files.trash))
			assert.JSONEq(t, test._expectedSettings, encodeDoc(test.files.settings.Doc()))
			assert.JSONEq(t, test._expectedClaudeJson, encodeDoc(test.files.claudeJson.Doc()))
			assert.JSONEq(t, test._expectedParked, encodeDoc(test.files.parked))
		})
	}
}

func TestReconcileHooks(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedChanges  []string
		_expectedSettings string

		files *settingsFiles
	}

	overridden := FullRecord(goosDarwin)
	overridden.Settings = Settings{components.SessionHelpers: {"groups.session-helpers-stop-play-sound.timeout": json.RawMessage(`9`)}}
	stopGroupV2Timeout := `{"smineId": "session-helpers-stop-play-sound", "smineName": "Stop sound", "smineVersion": "1.0.1", "hooks": [{"type": "command", "command": "bash ~/.claude/hooks/play-sound.sh stop", "async": true, "timeout": 9}]}`
	tests := make([]*testCase, 0)

	// override-on-unchanged-version-replaces
	replaced := newSettingsFiles(t, `{"hooks": {"Stop": [`+stopGroupV2+`]}}`, `{}`, `{}`)
	replaced.component, replaced.record = components.SessionHelpers, overridden
	tests = append(tests, &testCase{
		_id:               "override-on-unchanged-version-replaces",
		_expectedChanges:  []string{"hook Stop session-helpers-stop-play-sound settings"},
		_expectedSettings: `{"hooks": {"Stop": [` + stopGroupV2Timeout + `]}}`,

		files: replaced,
	})

	// override-already-live-no-change
	current := newSettingsFiles(t, `{"hooks": {"Stop": [`+stopGroupV2Timeout+`]}}`, `{}`, `{}`)
	current.component, current.record = components.SessionHelpers, overridden
	tests = append(tests, &testCase{
		_id:               "override-already-live-no-change",
		_expectedChanges:  nil,
		_expectedSettings: `{"hooks": {"Stop": [` + stopGroupV2Timeout + `]}}`,

		files: current,
	})

	// cleared-override-restores-fragment
	cleared := newSettingsFiles(t, `{"hooks": {"Stop": [`+stopGroupV2Timeout+`]}}`, `{}`, `{}`)
	cleared.component, cleared.record = components.SessionHelpers, FullRecord(goosDarwin)
	tests = append(tests, &testCase{
		_id:               "cleared-override-restores-fragment",
		_expectedChanges:  []string{"hook Stop session-helpers-stop-play-sound settings"},
		_expectedSettings: `{"hooks": {"Stop": [` + stopGroupV2 + `]}}`,

		files: cleared,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			changes, err := test.files.reconcileHooks("Stop", []json.RawMessage{json.RawMessage(stopGroupV2)}, true)
			assert.NoError(t, err)
			assert.Equal(t, test._expectedChanges, changes)
			assert.JSONEq(t, test._expectedSettings, encodeDoc(test.files.settings.Doc()))
		})
	}
}

func TestReconcile_IdempotentSecondRun(t *testing.T) {
	files := newSettingsFiles(t, `{}`, `{}`, `{}`)
	fragment := decodeFragment(t, `{"hooks": {"Stop": [`+stopGroup+`]}, "permissions": {"allow": ["Bash(jq *)"]}, "env": {"A": "1"}}`)

	first, err := files.reconcile(fragment, true)
	require.NoError(t, err)
	second, err := files.reconcile(fragment, true)
	require.NoError(t, err)
	assert.Len(t, first, 3)
	assert.Empty(t, second)
}

func TestReconcile_JournalOneLinePerList(t *testing.T) {
	files := newSettingsFiles(t, `{"env": {"A": "1", "B": "2"}, "permissions": {"allow": ["Read", "Bash(jq *)", "Bash(ls *)"]}}`, `{"mcpServers": {"peek-mcp": {"command": "p"}}}`, `{}`)
	fragment := decodeFragment(t, `{"env": {"A": "1", "B": "2"}, "permissions": {"allow": ["Bash(jq *)", "Bash(ls *)"]}, "mcpServers": {"peek-mcp": {"command": "p"}}}`)

	_, err := files.reconcile(fragment, false)
	require.NoError(t, err)

	entries, err := files.trash.Tail(100)
	require.NoError(t, err)
	reasons := map[string]string{}
	for _, entry := range entries {
		reasons[entry.Path] = entry.Reason
	}
	expected := map[string]string{
		testClaudeJsonPath + "#mcpServers":      "component off, equal to the fragment: peek-mcp",
		testSettingsPath + "#env":               "component off, equal to the fragment: A, B",
		testSettingsPath + "#permissions.allow": "component off, equal to the fragment: Bash(jq *), Bash(ls *)",
	}
	assert.Equal(t, expected, reasons)
}

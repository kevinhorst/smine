package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/sessionstate"
	"github.com/kevinhorst/smine/internal/trash"
)

func TestIsSelfServiceRule(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool

		raw string
	}

	tests := make([]*testCase, 0)

	// pass-cargo
	tests = append(tests, &testCase{_id: "pass-cargo", _shouldPass: true, raw: "Bash(cargo *)"})

	// pass-make-rust
	tests = append(tests, &testCase{_id: "pass-make-rust", _shouldPass: true, raw: "Bash(make rust*)"})

	// pass-relative-binary
	tests = append(tests, &testCase{_id: "pass-relative-binary", _shouldPass: true, raw: "Bash(./bin/agentwt *)"})

	// pass-home-binary
	tests = append(tests, &testCase{_id: "pass-home-binary", _shouldPass: true, raw: "Bash(~/go/bin/smine-agentwt *)"})

	// pass-wt
	tests = append(tests, &testCase{_id: "pass-wt", _shouldPass: true, raw: "Bash(wt *)"})

	// pass-git-subcommand
	tests = append(tests, &testCase{_id: "pass-git-subcommand", _shouldPass: true, raw: "Bash(git clone *)"})

	// pass-bash-script
	tests = append(tests, &testCase{_id: "pass-bash-script", _shouldPass: true, raw: "Bash(bash skills/x.sh *)"})

	// pass-path-qualified-shell
	tests = append(tests, &testCase{_id: "pass-path-qualified-shell", _shouldPass: true, raw: "Bash(/bin/sh -c *)"})

	// pass-sudo
	tests = append(tests, &testCase{_id: "pass-sudo", _shouldPass: true, raw: "Bash(sudo *)"})

	// pass-rm
	tests = append(tests, &testCase{_id: "pass-rm", _shouldPass: true, raw: "Bash(rm *)"})

	// pass-sed
	tests = append(tests, &testCase{_id: "pass-sed", _shouldPass: true, raw: "Bash(sed *)"})

	// fail-env-wrapper
	tests = append(tests, &testCase{_id: "fail-env-wrapper", _shouldPass: false, raw: "Bash(/usr/bin/env python *)"})

	// fail-xargs-wrapper
	tests = append(tests, &testCase{_id: "fail-xargs-wrapper", _shouldPass: false, raw: "Bash(xargs *)"})

	// fail-wildcard-program
	tests = append(tests, &testCase{_id: "fail-wildcard-program", _shouldPass: false, raw: "Bash(* -rf)"})

	// fail-expansion-program
	tests = append(tests, &testCase{_id: "fail-expansion-program", _shouldPass: false, raw: "Bash($X *)"})

	// fail-bare-bash
	tests = append(tests, &testCase{_id: "fail-bare-bash", _shouldPass: false, raw: "Bash"})

	// fail-gate-binary-hooks-path
	tests = append(tests, &testCase{_id: "fail-gate-binary-hooks-path", _shouldPass: false, raw: "Bash(~/.claude/hooks/bin/toolpolicy *)"})

	// fail-gate-binary-built
	tests = append(tests, &testCase{_id: "fail-gate-binary-built", _shouldPass: false, raw: "Bash(bin/toolpolicy *)"})

	// fail-gate-binary-bare
	tests = append(tests, &testCase{_id: "fail-gate-binary-bare", _shouldPass: false, raw: "Bash(toolpolicy *)"})

	// pass-webfetch-exact
	tests = append(tests, &testCase{_id: "pass-webfetch-exact", _shouldPass: true, raw: "WebFetch(domain:crates.io)"})

	// fail-webfetch-glob
	tests = append(tests, &testCase{_id: "fail-webfetch-glob", _shouldPass: false, raw: "WebFetch(domain:*.example.com)"})

	// pass-mcp-exact
	tests = append(tests, &testCase{_id: "pass-mcp-exact", _shouldPass: true, raw: "mcp__goland__search_symbol"})

	// fail-mcp-server
	tests = append(tests, &testCase{_id: "fail-mcp-server", _shouldPass: false, raw: "mcp__goland"})

	// fail-mcp-glob
	tests = append(tests, &testCase{_id: "fail-mcp-glob", _shouldPass: false, raw: "mcp__goland__search_*"})

	// fail-skill-name
	tests = append(tests, &testCase{_id: "fail-skill-name", _shouldPass: false, raw: "Skill(smine-orchestrate)"})

	// fail-read-path
	tests = append(tests, &testCase{_id: "fail-read-path", _shouldPass: false, raw: "Read(//Users/x/**)"})

	// fail-unparseable
	tests = append(tests, &testCase{_id: "fail-unparseable", _shouldPass: false, raw: "Bash(cargo"})

	wrapperCommands := defaultToolpolicy(t).WrapperCommands

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := isSelfServiceRule(test.raw, wrapperCommands)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
		})
	}
}

func TestCheckNewRules(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   []string
		_shouldPass bool

		existing []string
		rules    []string
	}

	yesterdaysSix := []string{
		"Bash(make rust*)",
		"Bash(cargo *)",
		"Bash(bin/agentwt *)",
		"Bash(./bin/agentwt *)",
		"Bash(~/go/bin/smine-agentwt *)",
		"Bash(wt *)",
	}

	tests := make([]*testCase, 0)

	// pass-yesterdays-six
	tests = append(tests, &testCase{_id: "pass-yesterdays-six", _expected: yesterdaysSix, _shouldPass: true, existing: []string{}, rules: yesterdaysSix})

	// fail-empty
	tests = append(tests, &testCase{_id: "fail-empty", _expected: nil, _shouldPass: false, existing: []string{}, rules: []string{}})

	// fail-duplicate-in-call
	tests = append(tests, &testCase{_id: "fail-duplicate-in-call", _expected: nil, _shouldPass: false, existing: []string{}, rules: []string{"Bash(wt *)", "Bash(wt *)"}})

	// pass-already-present-skipped
	tests = append(tests, &testCase{_id: "pass-already-present-skipped", _expected: []string{}, _shouldPass: true, existing: []string{"Bash(wt *)"}, rules: []string{"Bash(wt *)"}})

	// pass-partly-present
	tests = append(tests, &testCase{
		_id:         "pass-partly-present",
		_expected:   []string{"Bash(cargo *)"},
		_shouldPass: true,

		existing: []string{"Bash(wt *)"},
		rules:    []string{"Bash(wt *)", "Bash(cargo *)"},
	})

	// pass-runner-head
	tests = append(tests, &testCase{_id: "pass-runner-head", _expected: []string{"Bash(cargo *)", "Bash(python3 *)"}, _shouldPass: true, existing: []string{}, rules: []string{"Bash(cargo *)", "Bash(python3 *)"}})

	// fail-wrapper-head
	tests = append(tests, &testCase{_id: "fail-wrapper-head", _expected: nil, _shouldPass: false, existing: []string{}, rules: []string{"Bash(cargo *)", "Bash(timeout *)"}})

	policy := defaultToolpolicy(t)

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			fresh, err := checkNewRules(test.rules, test.existing, policy)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, fresh)
		})
	}
}

func TestUncoveredRules(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		grants *Grants
		rules  []string
	}

	human := &Grants{Approvals: map[string]bool{}, HasHumanTurn: true}
	allowLine := &Grants{Approvals: map[string]bool{unlockSettings: true}, HasHumanTurn: true}

	tests := make([]*testCase, 0)

	// no-allow-line-all-uncovered
	tests = append(tests, &testCase{_id: "no-allow-line-all-uncovered", _expected: []string{"Bash(cargo *)", "Bash(uptime)"}, grants: human, rules: []string{"Bash(cargo *)", "Bash(uptime)"}})

	// allow-line-covers-all
	tests = append(tests, &testCase{_id: "allow-line-covers-all", _expected: nil, grants: allowLine, rules: []string{"Bash(python3 *)"}})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, uncoveredRules(test.rules, test.grants))
		})
	}
}

func TestWritePermissionList(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		content string
		key     string
		values  []string
	}

	tests := make([]*testCase, 0)

	// creates-missing-claude-dir
	tests = append(tests, &testCase{
		_id:       "creates-missing-claude-dir",
		_expected: "{\n  \"permissions\": {\n    \"allow\": [\n      \"Bash(cargo *)\"\n    ]\n  }\n}\n",

		content: "",
		key:     allowKey,
		values:  []string{"Bash(cargo *)"},
	})

	// keeps-other-keys-and-order
	tests = append(tests, &testCase{
		_id:       "keeps-other-keys-and-order",
		_expected: "{\n  \"skillOverrides\": {\n    \"demo\": \"on\"\n  },\n  \"permissions\": {\n    \"defaultMode\": \"acceptEdits\",\n    \"allow\": [\n      \"Bash(ls-probe *)\",\n      \"Bash(cargo *)\"\n    ]\n  }\n}\n",

		content: `{"skillOverrides": {"demo": "on"}, "permissions": {"defaultMode": "acceptEdits", "allow": ["Bash(ls-probe *)"]}}`,
		key:     allowKey,
		values:  []string{"Bash(ls-probe *)", "Bash(cargo *)"},
	})

	// replaces-allow
	tests = append(tests, &testCase{
		_id:       "replaces-allow",
		_expected: "{\n  \"permissions\": {\n    \"allow\": []\n  }\n}\n",

		content: `{"permissions": {"allow": ["Bash(cargo *)"]}}`,
		key:     allowKey,
		values:  []string{},
	})

	// writes-additional-directories
	tests = append(tests, &testCase{
		_id:       "writes-additional-directories",
		_expected: "{\n  \"permissions\": {\n    \"allow\": [\n      \"Bash(ls-probe *)\"\n    ],\n    \"additionalDirectories\": [\n      \"/tmp\"\n    ]\n  }\n}\n",

		content: `{"permissions": {"allow": ["Bash(ls-probe *)"]}}`,
		key:     additionalDirectoriesKey,
		values:  []string{"/tmp"},
	})

	// Run tests
	for index, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			base := t.TempDir()
			target := filepath.Join(base, "wt", ".claude", "settings.local.json")
			if test.content != "" {
				require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
				require.NoError(t, os.WriteFile(target, []byte(test.content), 0o644))
			}
			settings, _, err := allowedRules(target)
			require.NoError(t, err, index)
			stateDir := filepath.Join(base, "home", ".claude", "smine")

			require.NoError(t, writePermissionList(target, settings, test.key, test.values, trash.New(stateDir, 0)))
			written, err := os.ReadFile(target)
			require.NoError(t, err)
			assert.Equal(t, test._expected, string(written))
			assert.Equal(t, test.content, stashedSettings(t, stateDir))
		})
	}
}

// stashedSettings reads the stash of the newest journal entry under stateDir; "" when nothing was kept.
func stashedSettings(t *testing.T, stateDir string) string {
	t.Helper()
	entries, err := trash.New(stateDir, 0).Tail(1)
	require.NoError(t, err)
	if len(entries) == 0 {
		return ""
	}

	data, err := os.ReadFile(filepath.Join(stateDir, trash.TrashDir, filepath.FromSlash(entries[0].Stash)))
	require.NoError(t, err)
	return string(data)
}

// selfServiceCwd creates a worktree dir; content "" leaves it without .claude
// (a fresh terminal-CLI worktree).
func selfServiceCwd(t *testing.T, base string, name string, content string) string {
	t.Helper()
	cwd := filepath.Join(base, name)
	require.NoError(t, os.MkdirAll(cwd, 0o755))
	if content == "" {
		return cwd
	}
	local := filepath.Join(cwd, ".claude", "settings.local.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(local), 0o755))
	require.NoError(t, os.WriteFile(local, []byte(content), 0o644))
	return cwd
}

func TestDecideSelfServiceCommand(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedAllow   []string
		_expectedApplied bool
		_expectedReason  string

		command   string
		content   string
		grants    *Grants
		state     *sessionstate.SelfServiceState
		statePath string
	}

	policy := defaultToolpolicy(t)
	base := t.TempDir()
	home := filepath.Join(base, "home")
	stateDir := filepath.Join(base, "state")
	withProbe := `{"permissions": {"allow": ["Bash(ls-probe *)"]}}`
	cargoThenUser := `{"permissions": {"allow": ["Bash(ls-probe *)", "Bash(cargo *)", "Bash(wt *)"]}}`
	human := &Grants{Approvals: map[string]bool{}, HasHumanTurn: true}
	allowLine := &Grants{Approvals: map[string]bool{unlockSettings: true}, HasHumanTurn: true}
	denyLine := &Grants{Approvals: map[string]bool{unlockSettings: false}, HasHumanTurn: true}
	headless := &Grants{Approvals: map[string]bool{}, HasHumanTurn: false}
	removed := &Grants{Approvals: map[string]bool{unlockSettings: true}, HasHumanTurn: true, anchor: sessionAnchor{isMissing: true}}
	cargoCall := &sessionstate.SelfServiceState{LastAppended: []string{"Bash(cargo *)"}}

	tests := make([]*testCase, 0)

	// allow-line-two-applied
	tests = append(tests, &testCase{
		_id:              "allow-line-two-applied",
		_expectedAllow:   []string{"Bash(ls-probe *)", "Bash(cargo *)", "WebFetch(domain:crates.io)"},
		_expectedApplied: true,
		_expectedReason:  "APPLIED — settings.local.json appended 2 rule(s), 0 already present",

		command: "toolpolicy allow 'Bash(cargo *)' 'WebFetch(domain:crates.io)'",
		content: withProbe,
		grants:  allowLine,
	})

	// allow-line-creates-file
	tests = append(tests, &testCase{
		_id:              "allow-line-creates-file",
		_expectedAllow:   []string{"Bash(cargo *)"},
		_expectedApplied: true,
		_expectedReason:  "appended 1 rule(s), 0 already present",

		command: "toolpolicy allow 'Bash(cargo *)'",
		content: "",
		grants:  allowLine,
	})

	// allow-line-second-call-applied
	tests = append(tests, &testCase{
		_id:              "allow-line-second-call-applied",
		_expectedAllow:   []string{"Bash(ls-probe *)", "Bash(wt *)"},
		_expectedApplied: true,
		_expectedReason:  "appended 1 rule(s), 0 already present",

		command: "toolpolicy allow 'Bash(wt *)'",
		content: withProbe,
		grants:  allowLine,
		state:   cargoCall,
	})

	// already-present-applied
	tests = append(tests, &testCase{
		_id:              "already-present-applied",
		_expectedAllow:   []string{"Bash(ls-probe *)", "Bash(cargo *)"},
		_expectedApplied: true,
		_expectedReason:  "appended 1 rule(s), 1 already present",

		command: "toolpolicy allow 'Bash(ls-probe *)' 'Bash(cargo *)'",
		content: withProbe,
		grants:  allowLine,
	})

	// present-uncovered-applied
	tests = append(tests, &testCase{
		_id:              "present-uncovered-applied",
		_expectedAllow:   []string{"Bash(ls-probe *)"},
		_expectedApplied: true,
		_expectedReason:  "appended 0 rule(s), 1 already present",

		command: "toolpolicy allow 'Bash(ls-probe *)'",
		content: withProbe,
		grants:  human,
	})

	// anchor-removed-deny
	tests = append(tests, &testCase{
		_id:              "anchor-removed-deny",
		_expectedAllow:   []string{"Bash(ls-probe *)"},
		_expectedApplied: false,
		_expectedReason:  causeAnchorRemoved,

		command: "toolpolicy allow 'Bash(cargo *)'",
		content: withProbe,
		grants:  removed,
	})

	// uncovered-deny
	tests = append(tests, &testCase{
		_id:              "uncovered-deny",
		_expectedAllow:   []string{"Bash(ls-probe *)"},
		_expectedApplied: false,
		_expectedReason:  causeSettingsUncovered + "Bash(sw_vers -productVersion)",

		command: "toolpolicy allow 'Bash(sw_vers -productVersion)'",
		content: withProbe,
		grants:  human,
	})

	// uncovered-fresh-only-deny
	tests = append(tests, &testCase{
		_id:              "uncovered-fresh-only-deny",
		_expectedAllow:   []string{"Bash(ls-probe *)"},
		_expectedApplied: false,
		_expectedReason:  causeSettingsUncovered + "Bash(uptime)",

		command: "toolpolicy allow 'Bash(ls-probe *)' 'Bash(uptime)'",
		content: withProbe,
		grants:  human,
	})

	// allow-line-applied
	tests = append(tests, &testCase{
		_id:              "allow-line-applied",
		_expectedAllow:   []string{"Bash(ls-probe *)", "Bash(python3 *)"},
		_expectedApplied: true,
		_expectedReason:  "appended 1 rule(s)",

		command: "toolpolicy allow 'Bash(python3 *)'",
		content: withProbe,
		grants:  allowLine,
	})

	// deny-line-revoked
	tests = append(tests, &testCase{
		_id:              "deny-line-revoked",
		_expectedAllow:   []string{"Bash(ls-probe *)"},
		_expectedApplied: false,
		_expectedReason:  causeSettingsRevoked,

		command: "toolpolicy allow 'Bash(wt *)'",
		content: withProbe,
		grants:  denyLine,
	})

	// wrapper-rejected-deny
	tests = append(tests, &testCase{
		_id:              "wrapper-rejected-deny",
		_expectedAllow:   []string{"Bash(ls-probe *)"},
		_expectedApplied: false,
		_expectedReason:  "rules rejected",

		command: "toolpolicy allow 'Bash(xargs *)'",
		content: withProbe,
		grants:  allowLine,
	})

	// repeated-deny
	tests = append(tests, &testCase{
		_id:              "repeated-deny",
		_expectedAllow:   []string{"Bash(ls-probe *)"},
		_expectedApplied: false,
		_expectedReason:  "is repeated",

		command: "toolpolicy allow 'Bash(cargo *)' 'Bash(cargo *)'",
		content: withProbe,
		grants:  allowLine,
	})

	// empty-usage-deny
	tests = append(tests, &testCase{
		_id:              "empty-usage-deny",
		_expectedAllow:   []string{"Bash(ls-probe *)"},
		_expectedApplied: false,
		_expectedReason:  causeSettingsUsage,

		command: "toolpolicy allow",
		content: withProbe,
		grants:  human,
	})

	// redirect-usage-deny
	tests = append(tests, &testCase{
		_id:              "redirect-usage-deny",
		_expectedAllow:   []string{"Bash(ls-probe *)"},
		_expectedApplied: false,
		_expectedReason:  causeSettingsUsage,

		command: "toolpolicy allow 'Bash(cargo *)' > out.txt",
		content: withProbe,
		grants:  allowLine,
	})

	// pipe-usage-deny
	tests = append(tests, &testCase{
		_id:              "pipe-usage-deny",
		_expectedAllow:   []string{"Bash(ls-probe *)"},
		_expectedApplied: false,
		_expectedReason:  causeSettingsUsage,

		command: "toolpolicy allow 'Bash(cargo *)' | cat",
		content: withProbe,
		grants:  allowLine,
	})

	// nonliteral-usage-deny
	tests = append(tests, &testCase{
		_id:              "nonliteral-usage-deny",
		_expectedAllow:   []string{"Bash(ls-probe *)"},
		_expectedApplied: false,
		_expectedReason:  causeSettingsUsage,

		command: `toolpolicy allow "$RULE"`,
		content: withProbe,
		grants:  allowLine,
	})

	// unknown-sub-usage-deny
	tests = append(tests, &testCase{
		_id:              "unknown-sub-usage-deny",
		_expectedAllow:   []string{"Bash(ls-probe *)"},
		_expectedApplied: false,
		_expectedReason:  causeSettingsUsage,

		command: "toolpolicy grant 'Bash(cargo *)'",
		content: withProbe,
		grants:  allowLine,
	})

	// revert-applied
	tests = append(tests, &testCase{
		_id:              "revert-applied",
		_expectedAllow:   []string{"Bash(ls-probe *)", "Bash(wt *)"},
		_expectedApplied: true,
		_expectedReason:  "reverted the last toolpolicy allow",

		command: "toolpolicy revert",
		content: cargoThenUser,
		grants:  human,
		state:   cargoCall,
	})

	// second-revert-deny
	tests = append(tests, &testCase{
		_id:              "second-revert-deny",
		_expectedAllow:   []string{"Bash(ls-probe *)", "Bash(cargo *)", "Bash(wt *)"},
		_expectedApplied: false,
		_expectedReason:  "nothing to revert",

		command: "toolpolicy revert",
		content: cargoThenUser,
		grants:  human,
		state:   &sessionstate.SelfServiceState{IsReverted: true, LastAppended: []string{"Bash(cargo *)"}},
	})

	// revert-nothing-deny
	tests = append(tests, &testCase{
		_id:              "revert-nothing-deny",
		_expectedAllow:   []string{"Bash(ls-probe *)"},
		_expectedApplied: false,
		_expectedReason:  "nothing to revert",

		command: "toolpolicy revert",
		content: withProbe,
		grants:  human,
	})

	// headless-deny
	tests = append(tests, &testCase{
		_id:              "headless-deny",
		_expectedAllow:   []string{"Bash(ls-probe *)"},
		_expectedApplied: false,
		_expectedReason:  causeSettingsInteractive,

		command: "toolpolicy allow 'Bash(cargo *)'",
		content: withProbe,
		grants:  headless,
	})

	// broken-file-deny
	tests = append(tests, &testCase{
		_id:              "broken-file-deny",
		_expectedAllow:   nil,
		_expectedApplied: false,
		_expectedReason:  "current file does not load",

		command: "toolpolicy allow 'Bash(cargo *)'",
		content: "{",
		grants:  allowLine,
	})

	// no-state-path-deny
	tests = append(tests, &testCase{
		_id:              "no-state-path-deny",
		_expectedAllow:   []string{"Bash(ls-probe *)"},
		_expectedApplied: false,
		_expectedReason:  "tool-policy state unavailable",

		command:   "toolpolicy allow 'Bash(cargo *)'",
		content:   withProbe,
		grants:    allowLine,
		statePath: "-",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			cwd := selfServiceCwd(t, base, test._id, test.content)
			path := filepath.Join(stateDir, test._id+".json")
			if test.statePath == "-" {
				path = ""
			}
			if test.state != nil {
				require.NoError(t, sessionstate.Save(path, &sessionstate.SessionState{SettingsLocal: *test.state}))
			}
			input := &HookInput{
				Cwd:            cwd,
				PermissionMode: permissionModeAcceptEdits,
				SessionId:      test._id,
				ToolInput:      ToolInput{Command: test.command},
				ToolName:       toolNameBash,
			}

			verdict := Decide(input, policy, test.grants, home, path)

			assert.Equal(t, decisionDeny, verdict.Decision)
			assert.Equal(t, test._expectedApplied, verdict.IsApplied)
			assert.Contains(t, verdict.Reason, test._expectedReason)
			if test.content != "{" {
				_, allow, err := allowedRules(filepath.Join(cwd, ".claude", "settings.local.json"))
				require.NoError(t, err)
				assert.Equal(t, test._expectedAllow, allow)
			}
		})
	}
}

func TestDecideSelfServiceCommand_WritesAnchorFile(t *testing.T) {
	policy := defaultToolpolicy(t)
	base := t.TempDir()
	withProbe := `{"permissions": {"allow": ["Bash(ls-probe *)"]}}`
	anchor := selfServiceCwd(t, base, "anchor", withProbe)
	cwd := selfServiceCwd(t, base, "drifted", withProbe)
	grants := &Grants{Approvals: map[string]bool{unlockSettings: true}, HasHumanTurn: true, anchor: sessionAnchor{path: anchor}}
	input := &HookInput{
		Cwd:            cwd,
		PermissionMode: permissionModeAcceptEdits,
		SessionId:      "anchor",
		ToolInput:      ToolInput{Command: "toolpolicy allow 'Bash(cargo *)'"},
		ToolName:       toolNameBash,
	}

	verdict := Decide(input, policy, grants, filepath.Join(base, "home"), filepath.Join(base, "state", "anchor.json"))

	assert.True(t, verdict.IsApplied)
	_, anchorAllow, err := allowedRules(filepath.Join(anchor, ".claude", unlockSettings))
	require.NoError(t, err)
	assert.Equal(t, []string{"Bash(ls-probe *)", "Bash(cargo *)"}, anchorAllow)
	_, cwdAllow, err := allowedRules(filepath.Join(cwd, ".claude", unlockSettings))
	require.NoError(t, err)
	assert.Equal(t, []string{"Bash(ls-probe *)"}, cwdAllow)
}

func TestState(t *testing.T) {
	dir := t.TempDir()
	path := statePath(filepath.Join(dir, "denials.jsonl"), "session-1")
	assert.Equal(t, filepath.Join(dir, sessionstate.DirName, "session-1.json"), path)
	assert.Empty(t, statePath("", "session-1"))
	assert.Empty(t, statePath(filepath.Join(dir, "denials.jsonl"), ""))
}

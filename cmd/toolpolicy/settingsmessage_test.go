package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/sessionstate"
)

func TestParseHashtags(t *testing.T) {
	type testCase struct {
		_id       string
		_expected humanCommands

		text string
	}

	none := humanCommands{}
	tests := make([]*testCase, 0)

	// add-absolute-directory
	tests = append(tests, &testCase{
		_id:       "add-absolute-directory",
		_expected: humanCommands{messages: []settingsMessage{{directories: []string{"/tmp"}}}},

		text: "#add /tmp/",
	})

	// add-relative-is-nothing
	tests = append(tests, &testCase{
		_id:       "add-relative-is-nothing",
		_expected: humanCommands{messages: []settingsMessage{{}}},

		text: "#add relative/dir",
	})

	// add-tilde-is-nothing
	tests = append(tests, &testCase{
		_id:       "add-tilde-is-nothing",
		_expected: humanCommands{messages: []settingsMessage{{}}},

		text: "#add ~/Documents",
	})

	// add-missing-directory-is-nothing
	tests = append(tests, &testCase{
		_id:       "add-missing-directory-is-nothing",
		_expected: humanCommands{messages: []settingsMessage{{}}},

		text: "#add /tmp/does-not-exist-" + t.Name(),
	})

	// add-three-fields-is-nothing
	tests = append(tests, &testCase{
		_id:       "add-three-fields-is-nothing",
		_expected: humanCommands{messages: []settingsMessage{{}}},

		text: "#add /tmp please",
	})

	// set-one-line
	tests = append(tests, &testCase{
		_id:       "set-one-line",
		_expected: humanCommands{messages: []settingsMessage{{rules: []string{"Bash(git commit -m *)"}}}},

		text: "#set Bash(git commit -m *)",
	})

	// set-rules-per-line
	tests = append(tests, &testCase{
		_id:       "set-rules-per-line",
		_expected: humanCommands{messages: []settingsMessage{{rules: []string{"Bash(cargo *)", "WebFetch(domain:docs.rs)"}}}},

		text: "#set\nBash(cargo *)\n  WebFetch(domain:docs.rs)  \n",
	})

	// set-stops-at-blank-line
	tests = append(tests, &testCase{
		_id:       "set-stops-at-blank-line",
		_expected: humanCommands{messages: []settingsMessage{{rules: []string{"Bash(cargo *)"}}}},

		text: "#set Bash(cargo *)\n\nand then run it",
	})

	// set-stops-at-next-command
	tests = append(tests, &testCase{
		_id:       "set-stops-at-next-command",
		_expected: humanCommands{messages: []settingsMessage{{rules: []string{"Bash(cargo *)"}}, {disabled: []sessionstate.Feature{sessionstate.RgateGlobal}}}},

		text: "#set Bash(cargo *)\n#disable rgate:global",
	})

	// set-empty-is-nothing
	tests = append(tests, &testCase{
		_id:       "set-empty-is-nothing",
		_expected: humanCommands{messages: []settingsMessage{{rules: []string{}}}},

		text: "#set",
	})

	// reset
	tests = append(tests, &testCase{
		_id:       "reset",
		_expected: humanCommands{messages: []settingsMessage{{isReset: true}}},

		text: "  #reset\n",
	})

	// reset-with-words-is-nothing
	tests = append(tests, &testCase{
		_id:       "reset-with-words-is-nothing",
		_expected: humanCommands{messages: []settingsMessage{{}}},

		text: "#reset everything",
	})

	// plain-text-is-nothing
	tests = append(tests, &testCase{
		_id:       "plain-text-is-nothing",
		_expected: none,

		text: "please #add /tmp",
	})

	// unknown-hashtag-is-nothing
	tests = append(tests, &testCase{
		_id:       "unknown-hashtag-is-nothing",
		_expected: none,

		text: "#todo fix the gate",
	})

	// command-inside-prose
	tests = append(tests, &testCase{
		_id:       "command-inside-prose",
		_expected: humanCommands{messages: []settingsMessage{{disabled: []sessionstate.Feature{sessionstate.RgateGlobal}}}},

		text: "the guide is read already, so\n#disable rgate:global\nand carry on",
	})

	// skill-args-typed
	tests = append(tests, &testCase{
		_id:       "skill-args-typed",
		_expected: humanCommands{messages: []settingsMessage{{disabled: []sessionstate.Feature{sessionstate.SessionTitle}}}},

		text: "/fdesign #disable session:title",
	})

	// skill-args-tagged
	tests = append(tests, &testCase{
		_id:       "skill-args-tagged",
		_expected: humanCommands{messages: []settingsMessage{{disabled: []sessionstate.Feature{sessionstate.SessionTitle}}}},

		text: "<command-message>fdesign</command-message>\n<command-name>/fdesign</command-name>\n<command-args>#disable session:title</command-args>",
	})

	// skill-args-command-lines
	tests = append(tests, &testCase{
		_id: "skill-args-command-lines",
		_expected: humanCommands{messages: []settingsMessage{
			{disabled: []sessionstate.Feature{sessionstate.SessionTitle}},
			{disabled: []sessionstate.Feature{sessionstate.Ext("go")}},
		}},

		text: "<command-message>close</command-message>\n<command-name>/close</command-name>\n<command-args>first this\n#disable session:title\n#disable rgate:go</command-args>",
	})

	// skill-args-prose-is-nothing
	tests = append(tests, &testCase{
		_id:       "skill-args-prose-is-nothing",
		_expected: none,

		text: "<command-message>discuss</command-message>\n<command-name>/discuss</command-name>\n<command-args>turn it off in its entirety #disable session:title</command-args>",
	})

	// two-commands-one-message
	tests = append(tests, &testCase{
		_id: "two-commands-one-message",
		_expected: humanCommands{
			approvals: []approval{{isAllow: true, target: "Makefile"}},
			messages:  []settingsMessage{{disabled: []sessionstate.Feature{sessionstate.Heartbeat}}},
		},

		text: "#allow Makefile\n#disable heartbeat",
	})

	// allow-two-fields
	tests = append(tests, &testCase{
		_id:       "allow-two-fields",
		_expected: humanCommands{approvals: []approval{{isAllow: true, target: "settings.local.json"}}},

		text: "sure:\n#allow -s",
	})

	// deny-two-fields
	tests = append(tests, &testCase{
		_id:       "deny-two-fields",
		_expected: humanCommands{approvals: []approval{{isAllow: false, target: "Makefile"}}},

		text: "  #deny Makefile \n",
	})

	// allow-three-fields-is-nothing
	tests = append(tests, &testCase{
		_id:       "allow-three-fields-is-nothing",
		_expected: none,

		text: "#allow Makefile please",
	})

	// disable-one
	tests = append(tests, &testCase{
		_id:       "disable-one",
		_expected: humanCommands{messages: []settingsMessage{{disabled: []sessionstate.Feature{sessionstate.RgateGlobal}}}},

		text: "#disable rgate:global",
	})

	// disable-group-and-long-heartbeat
	tests = append(tests, &testCase{
		_id:       "disable-group-and-long-heartbeat",
		_expected: humanCommands{messages: []settingsMessage{{disabled: []sessionstate.Feature{sessionstate.Feature(sessionstate.GroupRgate), sessionstate.Heartbeat}}}},

		text: "#disable rgate  heartbeat\n",
	})

	// disable-members-list
	tests = append(tests, &testCase{
		_id:       "disable-members-list",
		_expected: humanCommands{messages: []settingsMessage{{disabled: []sessionstate.Feature{sessionstate.Ext("go"), sessionstate.Ext("rs"), sessionstate.SkillEnforce}}}},

		text: "#disable rgate:go,rs skill:enforce",
	})

	// disable-all
	tests = append(tests, &testCase{
		_id:       "disable-all",
		_expected: humanCommands{messages: []settingsMessage{{disabled: []sessionstate.Feature{sessionstate.Feature(sessionstate.All)}}}},

		text: "#disable all",
	})

	// enable-all
	tests = append(tests, &testCase{
		_id:       "enable-all",
		_expected: humanCommands{messages: []settingsMessage{{enabled: []sessionstate.Feature{sessionstate.Feature(sessionstate.All)}}}},

		text: "#enable all",
	})

	// disable-empty-member-rejects
	tests = append(tests, &testCase{
		_id:       "disable-empty-member-rejects",
		_expected: humanCommands{messages: []settingsMessage{{unknown: "rgate:"}}},

		text: "#disable rgate:",
	})

	// disable-alone-is-nothing
	tests = append(tests, &testCase{
		_id:       "disable-alone-is-nothing",
		_expected: humanCommands{messages: []settingsMessage{{}}},

		text: "#disable",
	})

	// disable-unknown-rejects
	tests = append(tests, &testCase{
		_id:       "disable-unknown-rejects",
		_expected: humanCommands{messages: []settingsMessage{{unknown: "gate"}}},

		text: "#disable gate",
	})

	// disable-known-then-unknown-rejects
	tests = append(tests, &testCase{
		_id:       "disable-known-then-unknown-rejects",
		_expected: humanCommands{messages: []settingsMessage{{unknown: "gate"}}},

		text: "#disable rgate:global gate",
	})

	// enable-unknown-rejects
	tests = append(tests, &testCase{
		_id:       "enable-unknown-rejects",
		_expected: humanCommands{messages: []settingsMessage{{unknown: "guard"}}},

		text: "#enable guard",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, parseHashtags(test.text))
		})
	}
}

func TestExpandSwitches(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []settingsMessage

		guides   []string
		messages []settingsMessage
	}

	tests := make([]*testCase, 0)

	// group-expands-with-guides
	tests = append(tests, &testCase{
		_id:       "group-expands-with-guides",
		_expected: []settingsMessage{{disabled: []sessionstate.Feature{sessionstate.RgateGlobal, sessionstate.RgateInvoked, sessionstate.RgatePlan, sessionstate.Ext("go")}}},

		guides:   []string{"go"},
		messages: []settingsMessage{{disabled: []sessionstate.Feature{sessionstate.Feature(sessionstate.GroupRgate)}}},
	})

	// all-expands-to-everything
	tests = append(tests, &testCase{
		_id:       "all-expands-to-everything",
		_expected: []settingsMessage{{enabled: sessionstate.Features(nil)}},

		guides:   nil,
		messages: []settingsMessage{{enabled: []sessionstate.Feature{sessionstate.Feature(sessionstate.All)}}},
	})

	// unknown-guide-rejects-message
	tests = append(tests, &testCase{
		_id:       "unknown-guide-rejects-message",
		_expected: []settingsMessage{{unknown: "zzz"}},

		guides:   []string{"go"},
		messages: []settingsMessage{{disabled: []sessionstate.Feature{sessionstate.Heartbeat, sessionstate.Ext("zzz")}}},
	})

	// unknown-member-rejects-message
	tests = append(tests, &testCase{
		_id:       "unknown-member-rejects-message",
		_expected: []settingsMessage{{unknown: "nope"}},

		guides:   nil,
		messages: []settingsMessage{{enabled: []sessionstate.Feature{sessionstate.Feature("skill:nope")}}},
	})

	// members-pass-through
	tests = append(tests, &testCase{
		_id:       "members-pass-through",
		_expected: []settingsMessage{{disabled: []sessionstate.Feature{sessionstate.Heartbeat, sessionstate.SkillEnforce}}, {rules: []string{"Bash(x *)"}}},

		guides:   nil,
		messages: []settingsMessage{{disabled: []sessionstate.Feature{sessionstate.Heartbeat, sessionstate.SkillEnforce}}, {rules: []string{"Bash(x *)"}}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			expandSwitches(test.messages, test.guides)
			assert.Equal(t, test._expected, test.messages)
		})
	}
}

func TestFoldSwitches(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []sessionstate.Feature

		messages []settingsMessage
	}

	tests := make([]*testCase, 0)

	// disable-then-enable-empty
	tests = append(tests, &testCase{
		_id:       "disable-then-enable-empty",
		_expected: nil,

		messages: []settingsMessage{{disabled: []sessionstate.Feature{sessionstate.RgateGlobal}}, {enabled: []sessionstate.Feature{sessionstate.RgateGlobal}}},
	})

	// disable-two-enable-one
	tests = append(tests, &testCase{
		_id:       "disable-two-enable-one",
		_expected: []sessionstate.Feature{sessionstate.Heartbeat},

		messages: []settingsMessage{{disabled: []sessionstate.Feature{sessionstate.RgateGlobal, sessionstate.Heartbeat}}, {enabled: []sessionstate.Feature{sessionstate.RgateGlobal}}},
	})

	// sorted-output
	tests = append(tests, &testCase{
		_id:       "sorted-output",
		_expected: []sessionstate.Feature{sessionstate.InjectAcdsl, sessionstate.RgateGlobal, sessionstate.SkillEnforce},

		messages: []settingsMessage{{disabled: []sessionstate.Feature{sessionstate.SkillEnforce, sessionstate.InjectAcdsl, sessionstate.RgateGlobal}}},
	})

	// duplicates-once
	tests = append(tests, &testCase{
		_id:       "duplicates-once",
		_expected: []sessionstate.Feature{sessionstate.RgateGlobal},

		messages: []settingsMessage{{disabled: []sessionstate.Feature{sessionstate.RgateGlobal, sessionstate.RgateGlobal}}, {disabled: []sessionstate.Feature{sessionstate.RgateGlobal}}},
	})

	// empty-messages
	tests = append(tests, &testCase{
		_id:       "empty-messages",
		_expected: nil,

		messages: nil,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, foldSwitches(test.messages))
		})
	}
}

func TestApplySwitches(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedContent string
		_expectedOff     []sessionstate.Feature
		_shouldPass      bool

		messages []settingsMessage
		path     string
	}

	dir := t.TempDir()
	syncNow := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	disableReadGate := []settingsMessage{{disabled: []sessionstate.Feature{sessionstate.RgateGlobal}}}
	readGateOff := `{"switches":{"off":["rgate:global"]}}`
	tests := make([]*testCase, 0)

	// no-path-no-file
	tests = append(tests, &testCase{
		_id:              "no-path-no-file",
		_expectedContent: "",
		_expectedOff:     []sessionstate.Feature{sessionstate.RgateGlobal},
		_shouldPass:      true,

		messages: disableReadGate,
		path:     "",
	})

	// new-off-set-writes
	tests = append(tests, &testCase{
		_id:              "new-off-set-writes",
		_expectedContent: "{\n  \"settings_local\": {\n    \"is_reverted\": false,\n    \"last_appended\": null\n  },\n  \"switches\": {\n    \"history\": [\n      {\n        \"action\": \"disabled\",\n        \"at\": \"2000-01-01T00:00:00Z\",\n        \"feature\": \"rgate:global\"\n      }\n    ],\n    \"off\": [\n      \"rgate:global\"\n    ]\n  }\n}\n",
		_expectedOff:     []sessionstate.Feature{sessionstate.RgateGlobal},
		_shouldPass:      true,

		messages: disableReadGate,
		path:     filepath.Join(dir, "new-off-set-writes.json"),
	})

	// unchanged-keeps-mtime
	unchangedPath := filepath.Join(dir, "unchanged-keeps-mtime.json")
	require.NoError(t, os.WriteFile(unchangedPath, []byte(readGateOff), 0o600))
	tests = append(tests, &testCase{
		_id:              "unchanged-keeps-mtime",
		_expectedContent: readGateOff,
		_expectedOff:     []sessionstate.Feature{sessionstate.RgateGlobal},
		_shouldPass:      true,

		messages: disableReadGate,
		path:     unchangedPath,
	})

	// garbage-record-error
	garbagePath := filepath.Join(dir, "garbage-record-error.json")
	require.NoError(t, os.WriteFile(garbagePath, []byte("{"), 0o600))
	tests = append(tests, &testCase{
		_id:              "garbage-record-error",
		_expectedContent: "{",
		_expectedOff:     nil,
		_shouldPass:      false,

		messages: disableReadGate,
		path:     garbagePath,
	})

	// reset-to-empty-writes
	resetPath := filepath.Join(dir, "reset-to-empty-writes.json")
	require.NoError(t, os.WriteFile(resetPath, []byte(readGateOff), 0o600))
	tests = append(tests, &testCase{
		_id:              "reset-to-empty-writes",
		_expectedContent: "{\n  \"settings_local\": {\n    \"is_reverted\": false,\n    \"last_appended\": null\n  },\n  \"switches\": {\n    \"history\": [\n      {\n        \"action\": \"enabled\",\n        \"at\": \"2000-01-01T00:00:00Z\",\n        \"feature\": \"rgate:global\"\n      }\n    ],\n    \"off\": null\n  }\n}\n",
		_expectedOff:     nil,
		_shouldPass:      true,

		messages: nil,
		path:     resetPath,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			off, err := applySwitches(test.messages, syncNow, test.path)
			written, _ := os.ReadFile(test.path)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedOff, off)
			assert.Equal(t, test._expectedContent, string(written))
		})
	}
}

func TestIsExplicitRule(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool

		rule string
	}

	tests := make([]*testCase, 0)

	// pass-wide-bash
	tests = append(tests, &testCase{_id: "pass-wide-bash", _shouldPass: true, rule: "Bash(git *)"})

	// pass-wrapper
	tests = append(tests, &testCase{_id: "pass-wrapper", _shouldPass: true, rule: "Bash(env *)"})

	// pass-bare-tool
	tests = append(tests, &testCase{_id: "pass-bare-tool", _shouldPass: true, rule: "WebSearch"})

	// pass-mcp-server
	tests = append(tests, &testCase{_id: "pass-mcp-server", _shouldPass: true, rule: "mcp__peek-mcp"})

	// pass-wildcard-domain
	tests = append(tests, &testCase{_id: "pass-wildcard-domain", _shouldPass: true, rule: "WebFetch(domain:*.rs)"})

	// fail-path-pattern
	tests = append(tests, &testCase{_id: "fail-path-pattern", _shouldPass: false, rule: "Read(~/x/**)"})

	// fail-unbalanced
	tests = append(tests, &testCase{_id: "fail-unbalanced", _shouldPass: false, rule: "Bash(git *"})

	// fail-empty
	tests = append(tests, &testCase{_id: "fail-empty", _shouldPass: false, rule: ""})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := isExplicitRule(test.rule)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
		})
	}
}

func TestApplySettingsMessages(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedContent string
		_expectedOutcome []settingsOutcome
		_shouldPass      bool

		content  string
		messages []settingsMessage
	}

	tests := make([]*testCase, 0)

	// appends-both-lists
	tests = append(tests, &testCase{
		_id:              "appends-both-lists",
		_expectedContent: "{\n  \"permissions\": {\n    \"allow\": [\n      \"Bash(ls-probe *)\",\n      \"Bash(cargo *)\"\n    ],\n    \"additionalDirectories\": [\n      \"/tmp\"\n    ]\n  }\n}\n",
		_expectedOutcome: []settingsOutcome{{directories: []string{"/tmp"}, rules: []string{"Bash(cargo *)"}}},
		_shouldPass:      true,

		content:  `{"permissions": {"allow": ["Bash(ls-probe *)"]}}`,
		messages: []settingsMessage{{directories: []string{"/tmp"}, rules: []string{"Bash(cargo *)"}}},
	})

	// present-entries-write-nothing
	tests = append(tests, &testCase{
		_id:              "present-entries-write-nothing",
		_expectedContent: `{"permissions": {"allow": ["Bash(cargo *)"], "additionalDirectories": ["/tmp/"]}}`,
		_expectedOutcome: []settingsOutcome{{directories: []string{"/tmp"}, rules: []string{"Bash(cargo *)"}}},
		_shouldPass:      true,

		content:  `{"permissions": {"allow": ["Bash(cargo *)"], "additionalDirectories": ["/tmp/"]}}`,
		messages: []settingsMessage{{directories: []string{"/tmp"}, rules: []string{"Bash(cargo *)"}}},
	})

	// rejected-rule-skipped-others-written
	tests = append(tests, &testCase{
		_id:              "rejected-rule-skipped-others-written",
		_expectedContent: "{\n  \"permissions\": {\n    \"allow\": [\n      \"Bash(cargo *)\"\n    ]\n  }\n}\n",
		_expectedOutcome: []settingsOutcome{{rejected: []string{"isExplicitRule: Rule \"Read(~/x/**)\" is a path rule, which settings never match — use #add <directory>"}, rules: []string{"Bash(cargo *)"}}},
		_shouldPass:      true,

		content:  "",
		messages: []settingsMessage{{rules: []string{"Read(~/x/**)", "Bash(cargo *)"}}},
	})

	// no-messages-no-file
	tests = append(tests, &testCase{
		_id:              "no-messages-no-file",
		_expectedContent: "",
		_expectedOutcome: []settingsOutcome{},
		_shouldPass:      true,

		content:  "",
		messages: nil,
	})

	// fail-unparseable-file
	tests = append(tests, &testCase{
		_id:              "fail-unparseable-file",
		_expectedContent: "{",
		_expectedOutcome: nil,
		_shouldPass:      false,

		content:  "{",
		messages: []settingsMessage{{directories: []string{"/tmp"}}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "wt", ".claude", "settings.local.json")
			if test.content != "" {
				require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
				require.NoError(t, os.WriteFile(target, []byte(test.content), 0o644))
			}

			outcomes, err := applySettingsMessages(target, test.messages, nil)
			written, _ := os.ReadFile(target)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedOutcome, outcomes)
			assert.Equal(t, test._expectedContent, string(written))
		})
	}
}

func TestResetPermissionLists(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedContent string
		_expectedReport  string

		content string
	}

	tests := make([]*testCase, 0)

	// clears-lists-keeps-other-keys
	tests = append(tests, &testCase{
		_id:              "clears-lists-keeps-other-keys",
		_expectedContent: "{\n  \"skillOverrides\": {\n    \"demo\": \"on\"\n  },\n  \"permissions\": {\n    \"defaultMode\": \"acceptEdits\"\n  }\n}\n",
		_expectedReport:  "tool-policy #reset: cleared permissions.allow (2), permissions.additionalDirectories (1) and switches (0) of ",

		content: `{"skillOverrides": {"demo": "on"}, "permissions": {"defaultMode": "acceptEdits", "allow": ["Bash(a *)", "Bash(b *)"], "additionalDirectories": ["/tmp"]}}`,
	})

	// missing-file-writes-nothing
	tests = append(tests, &testCase{
		_id:              "missing-file-writes-nothing",
		_expectedContent: "",
		_expectedReport:  "tool-policy #reset: cleared permissions.allow (0), permissions.additionalDirectories (0) and switches (0) of ",

		content: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "wt", ".claude", "settings.local.json")
			if test.content != "" {
				require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
				require.NoError(t, os.WriteFile(target, []byte(test.content), 0o644))
			}

			report, err := resetPermissionLists(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), "", target, nil)
			written, _ := os.ReadFile(target)

			require.NoError(t, err)
			assert.Equal(t, test._expectedReport+target, report)
			assert.Equal(t, test._expectedContent, string(written))
		})
	}
}

func TestScanTranscript_SettingsMessages(t *testing.T) {
	scan := scanTranscript("testdata/transcript-settings-messages.jsonl", testDesignSkill)

	expected := []settingsMessage{{rules: []string{"Read(~/x/**)", "mcp__peek-mcp__session_list"}}}
	assert.Equal(t, expected, scan.settingsMessages)
}

// TestScanTranscript_HashtagSources: every way a message arrives counts once, everything the harness writes never.
func TestScanTranscript_HashtagSources(t *testing.T) {
	scan := scanTranscript("testdata/transcript-hashtag-sources.jsonl", testDesignSkill)

	expectedMessages := []settingsMessage{
		{disabled: []sessionstate.Feature{sessionstate.RgateGlobal}},
		{disabled: []sessionstate.Feature{sessionstate.Heartbeat}},
		{enabled: []sessionstate.Feature{sessionstate.RgateGlobal}},
		{disabled: []sessionstate.Feature{sessionstate.InjectSkill}},
	}
	expectedApprovals := map[string]bool{"Makefile": false, "settings.local.json": true}
	assert.Equal(t, expectedMessages, scan.settingsMessages)
	assert.Equal(t, expectedApprovals, scan.approvals)
}

// TestRun_SettingsMessages runs the gate end to end over the three events that carry a command.
func TestRun_SettingsMessages(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedDirs   []string
		_expectedOff    []sessionstate.Feature
		_expectedReport string
		_expectedRules  []string
		_isReported     bool

		cwd      string
		headless string
		ledger   string
		stdin    string
	}

	base := fixtureBase(t)
	home := filepath.Join(base, "home")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	writeHookConfig(t, filepath.Join(home, ".claude"), renderedHookConfig(t))

	tests := make([]*testCase, 0)

	// prompt-add-reports-and-writes
	addCwd := settingsMessageCwd(t, base, "add", "")
	tests = append(tests, &testCase{
		_id:             "prompt-add-reports-and-writes",
		_expectedDirs:   []string{"/tmp"},
		_expectedOff:    nil,
		_expectedReport: "tool-policy #add: /tmp is in additionalDirectories of " + localSettingsOf(addCwd),
		_expectedRules:  nil,
		_isReported:     true,

		cwd:      addCwd,
		headless: "",
		ledger:   "",
		stdin:    promptEnvelope(t, addCwd, "#add /tmp/"),
	})

	// prompt-set-reports-and-writes
	setCwd := settingsMessageCwd(t, base, "set", "")
	tests = append(tests, &testCase{
		_id:             "prompt-set-reports-and-writes",
		_expectedDirs:   nil,
		_expectedOff:    nil,
		_expectedReport: "tool-policy #set: 1 rule(s) in permissions.allow of " + localSettingsOf(setCwd),
		_expectedRules:  []string{"Bash(cargo *)"},
		_isReported:     true,

		cwd:      setCwd,
		headless: "",
		ledger:   "",
		stdin:    promptEnvelope(t, setCwd, "#set Bash(cargo *)\nRead(~/x/**)"),
	})

	// prompt-reset-clears
	resetCwd := settingsMessageCwd(t, base, "reset", `{"permissions": {"allow": ["Bash(cargo *)"], "additionalDirectories": ["/tmp"]}}`)
	tests = append(tests, &testCase{
		_id:             "prompt-reset-clears",
		_expectedDirs:   nil,
		_expectedOff:    nil,
		_expectedReport: "tool-policy #reset: cleared permissions.allow (1), permissions.additionalDirectories (1) and switches (0) of " + localSettingsOf(resetCwd),
		_expectedRules:  nil,
		_isReported:     true,

		cwd:      resetCwd,
		headless: "",
		ledger:   "",
		stdin:    promptEnvelope(t, resetCwd, "#reset"),
	})

	// headless-prompt-applies-nothing
	headlessCwd := settingsMessageCwd(t, base, "headless", "")
	tests = append(tests, &testCase{
		_id:             "headless-prompt-applies-nothing",
		_expectedDirs:   nil,
		_expectedOff:    nil,
		_expectedReport: "tool-policy #",
		_expectedRules:  nil,
		_isReported:     false,

		cwd:      headlessCwd,
		headless: headlessEnvValue,
		ledger:   "",
		stdin:    promptEnvelope(t, headlessCwd, "#add /tmp"),
	})

	// wrapped-prompt-applies-nothing
	wrappedCwd := settingsMessageCwd(t, base, "wrapped", "")
	tests = append(tests, &testCase{
		_id:             "wrapped-prompt-applies-nothing",
		_expectedDirs:   nil,
		_expectedOff:    nil,
		_expectedReport: "tool-policy #",
		_expectedRules:  nil,
		_isReported:     false,

		cwd:      wrappedCwd,
		headless: "",
		ledger:   "",
		stdin:    promptEnvelope(t, wrappedCwd, "<session-message>#add /tmp</session-message>"),
	})

	// transcript-set-applies-on-tool-call
	transcriptCwd := settingsMessageCwd(t, base, "transcript", "")
	tests = append(tests, &testCase{
		_id:             "transcript-set-applies-on-tool-call",
		_expectedDirs:   nil,
		_expectedOff:    nil,
		_expectedReport: "tool-policy #",
		_expectedRules:  []string{"mcp__peek-mcp__session_list"},
		_isReported:     false,

		cwd:      transcriptCwd,
		headless: "",
		ledger:   "",
		stdin:    `{"session_id":"s1","cwd":` + jsonString(t, transcriptCwd) + `,"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"ls"},"transcript_path":"testdata/transcript-settings-messages.jsonl"}`,
	})

	// directory-added-writes-silently
	addedCwd := settingsMessageCwd(t, base, "added", "")
	tests = append(tests, &testCase{
		_id:             "directory-added-writes-silently",
		_expectedDirs:   []string{"/tmp"},
		_expectedOff:    nil,
		_expectedReport: "tool-policy",
		_expectedRules:  nil,
		_isReported:     false,

		cwd:      addedCwd,
		headless: "",
		ledger:   "",
		stdin:    `{"session_id":"s1","cwd":` + jsonString(t, addedCwd) + `,"hook_event_name":"DirectoryAdded","directory":"/tmp","source":"slash_command"}`,
	})

	// prompt-disable-reports-and-writes
	disableCwd := settingsMessageCwd(t, base, "disable", "")
	tests = append(tests, &testCase{
		_id:             "prompt-disable-reports-and-writes",
		_expectedDirs:   nil,
		_expectedOff:    []sessionstate.Feature{sessionstate.Heartbeat, sessionstate.RgateGlobal},
		_expectedReport: "tool-policy #disable: off — hb, rgate:global (from the next hook event on)\n",
		_expectedRules:  nil,
		_isReported:     true,

		cwd:      disableCwd,
		headless: "",
		ledger:   filepath.Join(base, "ledger", "prompt-disable-reports-and-writes", "denials.jsonl"),
		stdin:    promptEnvelope(t, disableCwd, "#disable rgate:global heartbeat"),
	})

	// prompt-disable-group-reports-group-word
	groupCwd := settingsMessageCwd(t, base, "group", "")
	provideContextIndex(t, groupCwd, "*.go", "*_test.go", "*.rs")
	tests = append(tests, &testCase{
		_id:             "prompt-disable-group-reports-group-word",
		_expectedDirs:   nil,
		_expectedOff:    []sessionstate.Feature{sessionstate.RgateGlobal, sessionstate.Ext("go"), sessionstate.Ext("guide-0"), sessionstate.Ext("guide-1"), sessionstate.Ext("guide-2"), sessionstate.RgateInvoked, sessionstate.RgatePlan, sessionstate.Ext("rs")},
		_expectedReport: "tool-policy #disable: off — rgate (from the next hook event on)\n",
		_expectedRules:  nil,
		_isReported:     true,

		cwd:      groupCwd,
		headless: "",
		ledger:   filepath.Join(base, "ledger", "prompt-disable-group-reports-group-word", "denials.jsonl"),
		stdin:    promptEnvelope(t, groupCwd, "#disable rgate"),
	})

	// prompt-disable-guide-name-counts
	guideNameCwd := settingsMessageCwd(t, base, "guide-name", "")
	provideContextIndex(t, guideNameCwd, "*.go", "*_test.go")
	tests = append(tests, &testCase{
		_id:             "prompt-disable-guide-name-counts",
		_expectedDirs:   nil,
		_expectedOff:    []sessionstate.Feature{sessionstate.Ext("guide-1")},
		_expectedReport: "tool-policy #disable: off — rgate:guide-1 (from the next hook event on)\n",
		_expectedRules:  nil,
		_isReported:     true,

		cwd:      guideNameCwd,
		headless: "",
		ledger:   filepath.Join(base, "ledger", "prompt-disable-guide-name-counts", "denials.jsonl"),
		stdin:    promptEnvelope(t, guideNameCwd, "#disable rgate:guide-1"),
	})

	// prompt-disable-unknown-extension-rejected
	unknownGuideCwd := settingsMessageCwd(t, base, "unknown-guide", "")
	provideContextIndex(t, unknownGuideCwd, "*.go")
	tests = append(tests, &testCase{
		_id:             "prompt-disable-unknown-extension-rejected",
		_expectedDirs:   nil,
		_expectedOff:    nil,
		_expectedReport: "tool-policy #disable: rejected — unknown feature \"zig\"; features: " + sessionstate.Names() + "\n",
		_expectedRules:  nil,
		_isReported:     true,

		cwd:      unknownGuideCwd,
		headless: "",
		ledger:   filepath.Join(base, "ledger", "prompt-disable-unknown-extension-rejected", "denials.jsonl"),
		stdin:    promptEnvelope(t, unknownGuideCwd, "#disable rgate:go,zig"),
	})

	// prompt-enable-reports-still-off
	enableCwd := settingsMessageCwd(t, base, "enable", "")
	tests = append(tests, &testCase{
		_id:             "prompt-enable-reports-still-off",
		_expectedDirs:   nil,
		_expectedOff:    []sessionstate.Feature{sessionstate.Heartbeat},
		_expectedReport: "tool-policy #enable: on — rgate:global; still off — hb\n",
		_expectedRules:  nil,
		_isReported:     true,

		cwd:      enableCwd,
		headless: "",
		ledger:   filepath.Join(base, "ledger", "prompt-enable-reports-still-off", "denials.jsonl"),
		stdin:    `{"session_id":"s1","cwd":` + jsonString(t, enableCwd) + `,"hook_event_name":"UserPromptSubmit","prompt":"#enable rgate:global","transcript_path":"testdata/transcript-switch-messages.jsonl"}`,
	})

	// prompt-disable-unknown-rejected
	unknownCwd := settingsMessageCwd(t, base, "unknown", "")
	tests = append(tests, &testCase{
		_id:             "prompt-disable-unknown-rejected",
		_expectedDirs:   nil,
		_expectedOff:    nil,
		_expectedReport: "tool-policy #disable: rejected — unknown feature \"gate\"; features: rgate[:global|invoked|plan|<ext|guide>], skill[:enforce|invokegate], inject[:global|skill|acdsl|env|plan], session[:title], hb, all\n",
		_expectedRules:  nil,
		_isReported:     true,

		cwd:      unknownCwd,
		headless: "",
		ledger:   filepath.Join(base, "ledger", "prompt-disable-unknown-rejected", "denials.jsonl"),
		stdin:    promptEnvelope(t, unknownCwd, "#disable rgate:global gate"),
	})

	// prompt-reset-clears-switches
	resetSwitchesCwd := settingsMessageCwd(t, base, "reset-switches", "")
	resetSwitchesLedger := filepath.Join(base, "ledger", "prompt-reset-clears-switches", "denials.jsonl")
	resetSwitchesRecord := &sessionstate.SessionState{Switches: sessionstate.Switches{Off: []sessionstate.Feature{sessionstate.Heartbeat, sessionstate.RgateGlobal}}}
	require.NoError(t, sessionstate.Save(statePath(resetSwitchesLedger, "s1"), resetSwitchesRecord))
	tests = append(tests, &testCase{
		_id:             "prompt-reset-clears-switches",
		_expectedDirs:   nil,
		_expectedOff:    nil,
		_expectedReport: "tool-policy #reset: cleared permissions.allow (0), permissions.additionalDirectories (0) and switches (2) of " + localSettingsOf(resetSwitchesCwd) + "\n",
		_expectedRules:  nil,
		_isReported:     true,

		cwd:      resetSwitchesCwd,
		headless: "",
		ledger:   resetSwitchesLedger,
		stdin:    promptEnvelope(t, resetSwitchesCwd, "#reset"),
	})

	// transcript-disable-applies-on-tool-call
	transcriptSwitchCwd := settingsMessageCwd(t, base, "transcript-switch", "")
	tests = append(tests, &testCase{
		_id:             "transcript-disable-applies-on-tool-call",
		_expectedDirs:   nil,
		_expectedOff:    []sessionstate.Feature{sessionstate.Heartbeat, sessionstate.RgateGlobal},
		_expectedReport: "tool-policy #",
		_expectedRules:  nil,
		_isReported:     false,

		cwd:      transcriptSwitchCwd,
		headless: "",
		ledger:   filepath.Join(base, "ledger", "transcript-disable-applies-on-tool-call", "denials.jsonl"),
		stdin:    `{"session_id":"s1","cwd":` + jsonString(t, transcriptSwitchCwd) + `,"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"ls"},"transcript_path":"testdata/transcript-switch-messages.jsonl"}`,
	})

	// prompt-set-admits-same-call
	sameCallCwd := settingsMessageCwd(t, base, "same-call", "")
	tests = append(tests, &testCase{
		_id:             "prompt-set-admits-same-call",
		_expectedDirs:   nil,
		_expectedOff:    nil,
		_expectedReport: `"permissionDecision":"allow","permissionDecisionReason":"settings rule Bash(probe-x *)"`,
		_expectedRules:  []string{"Bash(probe-x *)"},
		_isReported:     true,

		cwd:      sameCallCwd,
		headless: "",
		ledger:   filepath.Join(base, "ledger", "prompt-set-admits-same-call", "denials.jsonl"),
		stdin:    `{"session_id":"s1","cwd":` + jsonString(t, sameCallCwd) + `,"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"probe-x run"},"transcript_path":"testdata/transcript-set-same-call.jsonl"}`,
	})

	// prompt-command-inside-prose
	proseCwd := settingsMessageCwd(t, base, "prose", "")
	tests = append(tests, &testCase{
		_id:             "prompt-command-inside-prose",
		_expectedDirs:   nil,
		_expectedOff:    []sessionstate.Feature{sessionstate.RgateGlobal},
		_expectedReport: "tool-policy #disable: off — rgate:global (from the next hook event on)\n",
		_expectedRules:  nil,
		_isReported:     true,

		cwd:      proseCwd,
		headless: "",
		ledger:   filepath.Join(base, "ledger", "prompt-command-inside-prose", "denials.jsonl"),
		stdin:    promptEnvelope(t, proseCwd, "the guide is read already, so\n#disable rgate:global\nand carry on"),
	})

	// prompt-allow-reports
	allowCwd := settingsMessageCwd(t, base, "allow", "")
	tests = append(tests, &testCase{
		_id:             "prompt-allow-reports",
		_expectedDirs:   nil,
		_expectedOff:    nil,
		_expectedReport: "tool-policy #allow: Makefile unlocked for this session — the last #allow or #deny for it wins\n",
		_expectedRules:  nil,
		_isReported:     true,

		cwd:      allowCwd,
		headless: "",
		ledger:   filepath.Join(base, "ledger", "prompt-allow-reports", "denials.jsonl"),
		stdin:    promptEnvelope(t, allowCwd, "#allow Makefile"),
	})

	// prompt-two-commands-report-each
	twoCwd := settingsMessageCwd(t, base, "two", "")
	tests = append(tests, &testCase{
		_id:             "prompt-two-commands-report-each",
		_expectedDirs:   nil,
		_expectedOff:    []sessionstate.Feature{sessionstate.Heartbeat},
		_expectedReport: "tool-policy #deny: Makefile locked for this session\ntool-policy #set: 1 rule(s) in permissions.allow of " + localSettingsOf(twoCwd) + " — Bash(probe-two *)\ntool-policy #disable: off — hb (from the next hook event on)\n",
		_expectedRules:  []string{"Bash(probe-two *)"},
		_isReported:     true,

		cwd:      twoCwd,
		headless: "",
		ledger:   filepath.Join(base, "ledger", "prompt-two-commands-report-each", "denials.jsonl"),
		stdin:    promptEnvelope(t, twoCwd, "#deny Makefile\n#set Bash(probe-two *)\n#disable heartbeat"),
	})

	// prompt-paste-inert-typed-counts
	pasteCwd := settingsMessageCwd(t, base, "paste", "")
	tests = append(tests, &testCase{
		_id:             "prompt-paste-inert-typed-counts",
		_expectedDirs:   nil,
		_expectedOff:    []sessionstate.Feature{sessionstate.Heartbeat},
		_expectedReport: "tool-policy #disable: off — hb (from the next hook event on)\n",
		_expectedRules:  nil,
		_isReported:     true,

		cwd:      pasteCwd,
		headless: "",
		ledger:   filepath.Join(base, "ledger", "prompt-paste-inert-typed-counts", "denials.jsonl"),
		stdin:    promptEnvelope(t, pasteCwd, "\n\n<pasted_content id=\"77\">\n#set Bash(pasted *)\n</pasted_content id=\"77\">\n\n-> pasted this in\n#disable hb"),
	})

	// prompt-reset-then-disable
	resetThenCwd := settingsMessageCwd(t, base, "reset-then", "")
	tests = append(tests, &testCase{
		_id:             "prompt-reset-then-disable",
		_expectedDirs:   nil,
		_expectedOff:    []sessionstate.Feature{sessionstate.RgateGlobal},
		_expectedReport: "tool-policy #reset: cleared permissions.allow (0), permissions.additionalDirectories (0) and switches (0) of " + localSettingsOf(resetThenCwd) + "\ntool-policy #disable: off — rgate:global (from the next hook event on)\n",
		_expectedRules:  nil,
		_isReported:     true,

		cwd:      resetThenCwd,
		headless: "",
		ledger:   filepath.Join(base, "ledger", "prompt-reset-then-disable", "denials.jsonl"),
		stdin:    promptEnvelope(t, resetThenCwd, "#reset\n#disable rgate:global"),
	})

	// headless-prompt-disable-nothing
	headlessSwitchCwd := settingsMessageCwd(t, base, "headless-switch", "")
	tests = append(tests, &testCase{
		_id:             "headless-prompt-disable-nothing",
		_expectedDirs:   nil,
		_expectedOff:    nil,
		_expectedReport: "tool-policy #",
		_expectedRules:  nil,
		_isReported:     false,

		cwd:      headlessSwitchCwd,
		headless: headlessEnvValue,
		ledger:   filepath.Join(base, "ledger", "headless-prompt-disable-nothing", "denials.jsonl"),
		stdin:    promptEnvelope(t, headlessSwitchCwd, "#disable rgate:global"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			t.Setenv(headlessEnv, test.headless)
			stdout := &strings.Builder{}

			exit := run([]string{"-ledger", test.ledger}, strings.NewReader(test.stdin), stdout, &strings.Builder{})
			settings, err := loadLocalSettings(localSettingsOf(test.cwd))
			require.NoError(t, err)
			dirs, dirsErr := permissionList(settings, additionalDirectoriesKey)
			rules, rulesErr := permissionList(settings, allowKey)
			record, recordErr := sessionstate.Load(statePath(test.ledger, "s1"))

			assert.Equal(t, 0, exit)
			assert.NoError(t, dirsErr)
			assert.NoError(t, rulesErr)
			require.NoError(t, recordErr)
			assert.Equal(t, test._expectedDirs, dirs)
			assert.Equal(t, test._expectedOff, record.Switches.Off)
			assert.Equal(t, test._expectedRules, rules)
			assert.Equalf(t, test._isReported, strings.Contains(stdout.String(), test._expectedReport), "stdout = %q", stdout.String())
		})
	}
}

func jsonString(t *testing.T, value string) string {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	return string(raw)
}

func localSettingsOf(cwd string) string {
	return filepath.Join(cwd, ".claude", unlockSettings)
}

func promptEnvelope(t *testing.T, cwd string, prompt string) string {
	t.Helper()
	return `{"session_id":"s1","cwd":` + jsonString(t, cwd) + `,"hook_event_name":"UserPromptSubmit","prompt":` + jsonString(t, prompt) + `}`
}

// provideContextIndex writes a context index under the gate's default context dir with one guide per file glob.
func provideContextIndex(t *testing.T, cwd string, globs ...string) {
	t.Helper()
	guides := make([]map[string]any, 0, len(globs))
	for index, glob := range globs {
		guides = append(guides, map[string]any{"files": []string{glob}, "path": fmt.Sprintf("rules/guide-%d.md", index)})
	}
	raw, err := json.Marshal(map[string]any{"guides": guides})
	require.NoError(t, err)

	dir := filepath.Join(cwd, testContextDir)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "context.json"), raw, 0o644))
}

// settingsMessageCwd creates a worktree dir; content "" leaves it without .claude.
func settingsMessageCwd(t *testing.T, base string, name string, content string) string {
	t.Helper()
	cwd := filepath.Join(base, "settings-messages", name)
	require.NoError(t, os.MkdirAll(cwd, 0o755))
	if content == "" {
		return cwd
	}

	local := localSettingsOf(cwd)
	require.NoError(t, os.MkdirAll(filepath.Dir(local), 0o755))
	require.NoError(t, os.WriteFile(local, []byte(content), 0o644))
	return cwd
}

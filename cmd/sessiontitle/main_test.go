package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/desktop"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/sessionstate"
	"github.com/kevinhorst/smine/internal/shell"
)

// fdesignTitleJson is fdesign's title declaration as the skill model renders it.
const fdesignTitleJson = `{
  "schema": 2,
  "skill": "fdesign",
  "states": {
    "CODE-READY": {
      "entry": "SKILL-FDESIGN-CODE-005",
      "meaning": "code gate presented — approve the Changes or request a change"
    },
    "DESIGN-READY": {
      "entry": "SKILL-FDESIGN-GATE-001",
      "meaning": "design gate presented — approve or revise the design"
    }
  },
  "version": "1.0.0"
}
`

// railroadReviewTitleJson is railroad-review's declaration.
const railroadReviewTitleJson = `{
  "schema": 2,
  "skill": "railroad-review",
  "states": {
    "HUMAN-GATE": {
      "entry": "SKILL-RAILROADREVIEW-MODES-001",
      "meaning": "round complete — say go for the next round or accept the review"
    }
  },
  "version": "1.0.0"
}
`

func TestRun(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedExit   int
		_expectedRecord *SessionRecord
		_expectedStderr string
		_expectedStdout string

		args        []string
		environment *Environment
		events      []string
		sessionId   string
	}

	home := resolvedTempDir(t)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	configDir, err := os.UserConfigDir()
	require.NoError(t, err)
	recordsDir := filepath.Join(home, ".claude", "sessiontitle", "sessions")
	writeHookConfig(t, filepath.Join(home, ".claude"), defaultHookConfig(t))

	base := resolvedTempDir(t)
	repo := newFixtureRepo(t, filepath.Join(base, "claude-configs"))
	otherRepo := newFixtureRepo(t, filepath.Join(base, "other-repo"))
	removedWorktree := filepath.Join(base, "claude-configs", ".claude", "worktrees", "title-system-test-d73881")
	reposPath := filepath.Join(base, "repos.json")
	registryJson := `{"repos":[{"label":"CC","name":"claude-configs","path":"` + repo + `"}]}`
	require.NoError(t, os.WriteFile(reposPath, []byte(registryJson), 0o644))

	skillsDir := filepath.Join(base, "skills")
	writeSkillMd(t, skillsDir, "fdesign", "---\nname: fdesign\ntitle-code: FD\nversion: 1.0.0\n---\n")
	writeSkillMd(t, skillsDir, "qq", "---\nname: qq\nversion: 1.0.0\n---\n")
	writeSkillMd(t, skillsDir, "railroad-review", "---\nname: railroad-review\nversion: 1.0.0\n---\n")
	writeTitleJson(t, skillsDir, "fdesign", fdesignTitleJson)
	writeTitleJson(t, skillsDir, "railroad-review", railroadReviewTitleJson)
	brokenSkillsDir := filepath.Join(base, "broken-skills")
	writeTitleJson(t, brokenSkillsDir, "broken", `{"schema":1}`)
	commitArgs := []string{"-repos", reposPath, "-skills", skillsDir}
	brokenArgs := []string{"-repos", reposPath, "-skills", brokenSkillsDir}
	stateRegistryPath := filepath.Join(home, ".claude", "smine", "repos.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(stateRegistryPath), 0o755))
	require.NoError(t, os.WriteFile(stateRegistryPath, []byte(registryJson), 0o644))
	stateRegistryArgs := []string{"-skills", skillsDir}

	desktopWithSource := func(hostId string, title string, titleSource string) *Environment {
		dir := filepath.Join(configDir, "Claude", "claude-code-sessions", "org", "proj")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		data, err := json.Marshal(&desktop.Session{Title: title, TitleSource: titleSource})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, hostId+".json"), data, 0o644))
		return &Environment{EntryPoint: entryPointDesktop, HostSessionId: hostId, IsAttended: true}
	}
	// desktop shadows the package from here on: every later store fixture goes
	// through it or desktopWithSource.
	desktop := func(hostId string, title string) *Environment {
		return desktopWithSource(hostId, title, "tool")
	}
	// recorded writes the session's record through the production writer.
	recorded := func(sessionId string, record *SessionRecord) string {
		require.NoError(t, saveRecord(filepath.Join(recordsDir, sessionId+".json"), record))
		return sessionId
	}
	// titleSwitchedOff writes the gate's switch record with session-title off.
	titleSwitchedOff := func(sessionId string) string {
		state := &sessionstate.SessionState{Switches: sessionstate.Switches{Off: []sessionstate.Feature{sessionstate.SessionTitle}}}
		require.NoError(t, sessionstate.Save(sessionstate.File(sessionstate.StateDir(filepath.Join(home, ".claude")), sessionId), state))
		return sessionId
	}
	envelope := func(input *HookInput) string {
		data, err := json.Marshal(input)
		require.NoError(t, err)
		return string(data)
	}
	transcriptPath := func(name string) string {
		path, err := filepath.Abs(filepath.Join("testdata", name))
		require.NoError(t, err)
		return path
	}
	sessionStartFrom := func(sessionId string, source string) string {
		return envelope(&HookInput{Cwd: repo, HookEventName: eventSessionStart, SessionId: sessionId, Source: source})
	}
	sessionStart := func(sessionId string) string {
		return sessionStartFrom(sessionId, "startup")
	}
	toolCall := func(cwd string, sessionId string, command string, fixture string) string {
		return envelope(&HookInput{Cwd: cwd, HookEventName: eventPostToolUse, SessionId: sessionId, ToolInput: ToolInput{Command: command}, ToolName: "Bash", TranscriptPath: transcriptPath(fixture)})
	}
	turnEnd := func(cwd string, sessionId string) string {
		return envelope(&HookInput{Cwd: cwd, HookEventName: eventStop, SessionId: sessionId, TranscriptPath: transcriptPath("prompts-fdesign.jsonl")})
	}
	bashResult := func(sessionId string, command string, output string) string {
		return envelope(&HookInput{Cwd: repo, HookEventName: eventPostToolUse, SessionId: sessionId, ToolInput: ToolInput{Command: command}, ToolName: "Bash", ToolResponse: ToolResponse{Stdout: output}, TranscriptPath: transcriptPath("prompts-fdesign.jsonl")})
	}
	// titleSet is the set command's PostToolUse with the binary's confirmation line.
	titleSet := func(sessionId string, state string) string {
		return bashResult(sessionId, "smine-rules title set "+state, "title: set "+state+"\n")
	}
	otherToolCall := func(sessionId string, toolName string) string {
		return envelope(&HookInput{Cwd: repo, HookEventName: eventPostToolUse, SessionId: sessionId, ToolName: toolName, TranscriptPath: transcriptPath("prompts-fdesign.jsonl")})
	}
	promptSubmit := func(sessionId string) string {
		return envelope(&HookInput{Cwd: repo, HookEventName: "UserPromptSubmit", SessionId: sessionId})
	}
	renameCall := func(sessionId string, title string) string {
		return envelope(&HookInput{Cwd: repo, HookEventName: eventPreToolUse, ToolInput: ToolInput{SessionId: sessionId, Title: title}, ToolName: "mcp__ccd_session_mgmt__set_session_title"})
	}

	// The instruction wording is the contract ACTION-IMPL-HARNESS-001 quotes.
	instruction := func(title string) string {
		return `Session title: call mcp__ccd_session_mgmt__set_session_title with session_id "self" and title "` + title + `".` +
			` This call is allowed in plan mode: it changes only this session's sidebar title, nothing in the repository or the system.` +
			` Do it as your next action, then continue with what you were doing; if nothing is pending, end your turn without further output. Do not mention it.`
	}
	renameContext := func(title string) string {
		quoted, err := json.Marshal(instruction(title))
		require.NoError(t, err)
		return `{"hookSpecificOutput":{"additionalContext":` + string(quoted) + `,"hookEventName":"PostToolUse"}}` + "\n"
	}
	stopBlock := func(title string) string {
		quoted, err := json.Marshal(instruction(title))
		require.NoError(t, err)
		return `{"decision":"block","reason":` + string(quoted) + `}` + "\n"
	}
	renameAllow := `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow","permissionDecisionReason":"sessiontitle: hook-instructed rename of this session"}}` + "\n"
	closeCheck := "~/go/bin/smine-agentwt remove --dry-run --delete-branch claude/title-system-test-d73881"
	joinCommand := "~/go/bin/smine-heartbeat join --fleet T1"
	doneCommand := "~/go/bin/smine-heartbeat done"
	brokenDeclaration := filepath.Join(brokenSkillsDir, "broken", "title.json")
	designReady := Turn{State: "DESIGN-READY"}
	designReadyDelivered := Turn{IsDelivered: true, State: "DESIGN-READY"}

	tests := make([]*testCase, 0)

	// sessionstart-creates-record
	tests = append(tests, &testCase{
		_id:             "sessionstart-creates-record",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0001", ""),
		events:      []string{sessionStart("a0000001")},
		sessionId:   "a0000001",
	})

	// sessionstart-keeps-named-clears-turn
	tests = append(tests, &testCase{
		_id:             "sessionstart-keeps-named-clears-turn",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Fleet: "T1-MA", Named: true},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0030", "CC,FD - Session title state structure - T1-MA[WIP]"),
		events:      []string{sessionStartFrom("a0000030", "resume")},
		sessionId:   recorded("a0000030", &SessionRecord{Fleet: "T1-MA", Named: true, Turn: designReadyDelivered}),
	})

	// sessionstart-compact-keeps-turn
	tests = append(tests, &testCase{
		_id:             "sessionstart-compact-keeps-turn",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true, Turn: designReadyDelivered},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0032", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{sessionStartFrom("a0000032", "compact")},
		sessionId:   recorded("a0000032", &SessionRecord{Named: true, Turn: designReadyDelivered}),
	})

	// sessionstart-inactive-no-record
	tests = append(tests, &testCase{
		_id:             "sessionstart-inactive-no-record",
		_expectedExit:   0,
		_expectedRecord: nil,
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: &Environment{EntryPoint: "cli", HostSessionId: "local_0002", IsAttended: true},
		events:      []string{sessionStart("a0000002")},
		sessionId:   "a0000002",
	})

	// sessionstart-invalid-session-id
	tests = append(tests, &testCase{
		_id:             "sessionstart-invalid-session-id",
		_expectedExit:   1,
		_expectedRecord: nil,
		_expectedStderr: "Hook.recordPath: Invalid session id \"../escape\"\n",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0003", ""),
		events:      []string{sessionStart("../escape")},
		sessionId:   "a0000003",
	})

	// first-auto-title-names
	tests = append(tests, &testCase{
		_id:             "first-auto-title-names",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: renameContext("CC,FD - Session title state structure - [WIP]"),

		args:        commitArgs,
		environment: desktopWithSource("local_0004", "Session title state structure", "auto"),
		events:      []string{toolCall(repo, "a0000004", "ls", "prompts-fdesign.jsonl")},
		sessionId:   recorded("a0000004", &SessionRecord{}),
	})

	// first-skill-of-later-prompt
	tests = append(tests, &testCase{
		_id:             "first-skill-of-later-prompt",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: renameContext("CC,qq - Session title state structure - [WIP]"),

		args:        commitArgs,
		environment: desktopWithSource("local_0005", "Session title state structure", "auto"),
		events:      []string{toolCall(repo, "a0000005", "ls", "prompts-later-skill.jsonl")},
		sessionId:   recorded("a0000005", &SessionRecord{}),
	})

	// first-no-skill
	tests = append(tests, &testCase{
		_id:             "first-no-skill",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: renameContext("CC - Session title state structure - [WIP]"),

		args:        commitArgs,
		environment: desktopWithSource("local_0006", "Session title state structure", "auto"),
		events:      []string{toolCall(repo, "a0000006", "ls", "prompts-none.jsonl")},
		sessionId:   recorded("a0000006", &SessionRecord{}),
	})

	// first-unregistered-repo
	tests = append(tests, &testCase{
		_id:             "first-unregistered-repo",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: renameContext("other-repo - Session title state structure - [WIP]"),

		args:        commitArgs,
		environment: desktopWithSource("local_0007", "Session title state structure", "auto"),
		events:      []string{toolCall(otherRepo, "a0000007", "ls", "prompts-none.jsonl")},
		sessionId:   recorded("a0000007", &SessionRecord{}),
	})

	// first-title-missing-keeps-unnamed
	tests = append(tests, &testCase{
		_id:             "first-title-missing-keeps-unnamed",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktopWithSource("local_0008", "", "auto"),
		events:      []string{toolCall(repo, "a0000008", "ls", "prompts-fdesign.jsonl")},
		sessionId:   recorded("a0000008", &SessionRecord{}),
	})

	// first-transcript-missing-keeps-unnamed
	tests = append(tests, &testCase{
		_id:             "first-transcript-missing-keeps-unnamed",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktopWithSource("local_0009", "Session title state structure", "auto"),
		events:      []string{toolCall(repo, "a0000009", "ls", "missing.jsonl")},
		sessionId:   recorded("a0000009", &SessionRecord{}),
	})

	// first-prompt-only-queued-keeps-unnamed
	tests = append(tests, &testCase{
		_id:             "first-prompt-only-queued-keeps-unnamed",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktopWithSource("local_0010", "Session title state structure", "auto"),
		events:      []string{toolCall(repo, "a0000010", "ls", "prompts-queued.jsonl")},
		sessionId:   recorded("a0000010", &SessionRecord{}),
	})

	// first-formatted-names
	tests = append(tests, &testCase{
		_id:             "first-formatted-names",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0011", "CC,FD - Session title state structure - [READY]"),
		events:      []string{toolCall(repo, "a0000011", "ls", "prompts-fdesign.jsonl")},
		sessionId:   recorded("a0000011", &SessionRecord{}),
	})

	// first-user-title-keeps-wording
	tests = append(tests, &testCase{
		_id:             "first-user-title-keeps-wording",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: renameContext("CC,FD - My own title - [WIP]"),

		args:        commitArgs,
		environment: desktopWithSource("local_0012", "My own title", "user"),
		events:      []string{toolCall(repo, "a0000012", "ls", "prompts-fdesign.jsonl")},
		sessionId:   recorded("a0000012", &SessionRecord{}),
	})

	// first-subagent-keeps-unnamed
	tests = append(tests, &testCase{
		_id:             "first-subagent-keeps-unnamed",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktopWithSource("local_0013", "Session title state structure", "auto"),
		events:      []string{envelope(&HookInput{AgentId: "a1", Cwd: repo, HookEventName: eventPostToolUse, SessionId: "a0000013"})},
		sessionId:   recorded("a0000013", &SessionRecord{}),
	})

	// first-echo-title-keeps-unnamed
	tests = append(tests, &testCase{
		_id:             "first-echo-title-keeps-unnamed",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktopWithSource("local_0040", "/fdesign change: Session title hook /peek", "auto"),
		events:      []string{toolCall(repo, "a0000040", "ls", "prompts-fdesign.jsonl")},
		sessionId:   recorded("a0000040", &SessionRecord{}),
	})

	// first-writes-saved-token
	tests = append(tests, &testCase{
		_id:             "first-writes-saved-token",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Fleet: "T1-MA", Named: true},
		_expectedStderr: "",
		_expectedStdout: renameContext("CC,FD - Session title state structure - T1-MA[WIP]"),

		args:        commitArgs,
		environment: desktopWithSource("local_0060", "Session title state structure", "auto"),
		events:      []string{toolCall(repo, "a0000060", "ls", "prompts-fdesign.jsonl")},
		sessionId:   recorded("a0000060", &SessionRecord{Fleet: "T1-MA"}),
	})

	// first-rename-wins-over-set
	tests = append(tests, &testCase{
		_id:             "first-rename-wins-over-set",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true, Turn: designReadyDelivered},
		_expectedStderr: "",
		_expectedStdout: renameContext("CC,FD - Session title state structure - [WIP]"),

		args:        commitArgs,
		environment: desktopWithSource("local_0031", "Session title state structure", "auto"),
		events:      []string{titleSet("a0000031", "DESIGN-READY")},
		sessionId:   recorded("a0000031", &SessionRecord{}),
	})

	// set-first-renames-at-call
	tests = append(tests, &testCase{
		_id:             "set-first-renames-at-call",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true, Turn: designReadyDelivered},
		_expectedStderr: "",
		_expectedStdout: renameContext("CC,FD - Session title state structure - [DESIGN-READY]"),

		args:        commitArgs,
		environment: desktop("local_0076", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{titleSet("a0000076", "DESIGN-READY")},
		sessionId:   recorded("a0000076", &SessionRecord{Named: true}),
	})

	// set-first-matching-title-silent
	tests = append(tests, &testCase{
		_id:             "set-first-matching-title-silent",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0205", "CC,FD - Session title state structure - [DESIGN-READY]"),
		events:      []string{titleSet("a0000205", "DESIGN-READY"), turnEnd(repo, "a0000205")},
		sessionId:   recorded("a0000205", &SessionRecord{Named: true}),
	})

	// set-second-waits-for-stop
	tests = append(tests, &testCase{
		_id:             "set-second-waits-for-stop",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true, Turn: Turn{IsDelivered: true, State: "READY"}},
		_expectedStderr: "",
		_expectedStdout: renameContext("CC,FD - Session title state structure - [WIP]"),

		args:        commitArgs,
		environment: desktop("local_0077", "CC,FD - Session title state structure - [CODE-READY]"),
		events:      []string{titleSet("a0000077", "WIP"), titleSet("a0000077", "READY")},
		sessionId:   recorded("a0000077", &SessionRecord{Named: true}),
	})

	// set-delivered-turn-records-only
	tests = append(tests, &testCase{
		_id:             "set-delivered-turn-records-only",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true, Turn: Turn{IsDelivered: true, State: "WIP"}},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0078", "CC,FD - Session title state structure - [DESIGN-READY]"),
		events:      []string{titleSet("a0000078", "WIP")},
		sessionId:   recorded("a0000078", &SessionRecord{Named: true, Turn: designReadyDelivered}),
	})

	// set-unconfirmed-records-nothing
	tests = append(tests, &testCase{
		_id:             "set-unconfirmed-records-nothing",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0079", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{bashResult("a0000079", "smine-rules title set DESIGN-READY", "")},
		sessionId:   recorded("a0000079", &SessionRecord{Named: true}),
	})

	// set-undeclared-records-nothing
	tests = append(tests, &testCase{
		_id:             "set-undeclared-records-nothing",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0080", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{titleSet("a0000080", "FOO-BAR")},
		sessionId:   recorded("a0000080", &SessionRecord{Named: true}),
	})

	// set-terminal-builtin-records-nothing
	tests = append(tests, &testCase{
		_id:             "set-terminal-builtin-records-nothing",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0084", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{titleSet("a0000084", "DONE")},
		sessionId:   recorded("a0000084", &SessionRecord{Named: true}),
	})

	// set-last-call-wins-at-stop
	tests = append(tests, &testCase{
		_id:             "set-last-call-wins-at-stop",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: renameContext("CC,FD - Session title state structure - [DESIGN-READY]") + stopBlock("CC,FD - Session title state structure - [HUMAN-GATE]"),

		args:        commitArgs,
		environment: desktop("local_0087", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{titleSet("a0000087", "DESIGN-READY"), titleSet("a0000087", "HUMAN-GATE"), turnEnd(repo, "a0000087")},
		sessionId:   recorded("a0000087", &SessionRecord{Named: true}),
	})

	// set-with-path-prefix-renames
	tests = append(tests, &testCase{
		_id:             "set-with-path-prefix-renames",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true, Turn: designReadyDelivered},
		_expectedStderr: "",
		_expectedStdout: renameContext("CC,FD - Session title state structure - [DESIGN-READY]"),

		args:        commitArgs,
		environment: desktop("local_0083", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{bashResult("a0000083", "~/go/bin/smine-rules title set DESIGN-READY", "title: set DESIGN-READY\n")},
		sessionId:   recorded("a0000083", &SessionRecord{Named: true}),
	})

	// set-subagent-silent
	tests = append(tests, &testCase{
		_id:             "set-subagent-silent",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0088", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{envelope(&HookInput{AgentId: "a1", Cwd: repo, HookEventName: eventPostToolUse, SessionId: "a0000088", ToolInput: ToolInput{Command: "smine-rules title set DESIGN-READY"}, ToolName: "Bash", ToolResponse: ToolResponse{Stdout: "title: set DESIGN-READY\n"}})},
		sessionId:   recorded("a0000088", &SessionRecord{Named: true}),
	})

	// other-bash-records-nothing
	tests = append(tests, &testCase{
		_id:             "other-bash-records-nothing",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0020", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{toolCall(repo, "a0000020", `git commit -q -m "cmd.sessiontitle: Added hook"`, "prompts-fdesign.jsonl")},
		sessionId:   recorded("a0000020", &SessionRecord{Named: true}),
	})

	// other-tool-records-nothing
	tests = append(tests, &testCase{
		_id:             "other-tool-records-nothing",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true, Turn: designReady},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0014", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{otherToolCall("a0000014", "Edit"), otherToolCall("a0000014", "Workflow"), otherToolCall("a0000014", "Skill")},
		sessionId:   recorded("a0000014", &SessionRecord{Named: true, Turn: designReady}),
	})

	// prompt-keeps-pending-state
	tests = append(tests, &testCase{
		_id:             "prompt-keeps-pending-state",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true, Turn: designReady},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0039", "CC,FD - Session title state structure - [READY]"),
		events:      []string{promptSubmit("a0000039")},
		sessionId:   recorded("a0000039", &SessionRecord{Named: true, Turn: designReady}),
	})

	// stop-set-blocks-design-ready
	tests = append(tests, &testCase{
		_id:             "stop-set-blocks-design-ready",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: stopBlock("CC,FD - Session title state structure - [DESIGN-READY]"),

		args:        commitArgs,
		environment: desktop("local_0033", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{turnEnd(repo, "a0000033")},
		sessionId:   recorded("a0000033", &SessionRecord{Named: true, Turn: designReady}),
	})

	// stop-set-from-ready-keeps-fleet
	tests = append(tests, &testCase{
		_id:             "stop-set-from-ready-keeps-fleet",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: stopBlock("CC,FD - Session title state structure - T1-MA[DESIGN-READY]"),

		args:        commitArgs,
		environment: desktop("local_0081", "CC,FD - Session title state structure - T1-MA[READY]"),
		events:      []string{turnEnd(repo, "a0000081")},
		sessionId:   recorded("a0000081", &SessionRecord{Named: true, Turn: designReady}),
	})

	// stop-set-replaces-other-declared
	tests = append(tests, &testCase{
		_id:             "stop-set-replaces-other-declared",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: stopBlock("CC,FD - Session title state structure - [CODE-READY]"),

		args:        commitArgs,
		environment: desktop("local_0090", "CC,FD - Session title state structure - [DESIGN-READY]"),
		events:      []string{turnEnd(repo, "a0000090")},
		sessionId:   recorded("a0000090", &SessionRecord{Named: true, Turn: Turn{State: "CODE-READY"}}),
	})

	// stop-set-keeps-done
	tests = append(tests, &testCase{
		_id:             "stop-set-keeps-done",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0082", "CC,FD - Session title state structure - T1-MA[DONE]"),
		events:      []string{turnEnd(repo, "a0000082")},
		sessionId:   recorded("a0000082", &SessionRecord{Named: true, Turn: designReady}),
	})

	// stop-set-keeps-testing
	tests = append(tests, &testCase{
		_id:             "stop-set-keeps-testing",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0016", "CC,FD - Session title state structure - [TESTING]"),
		events:      []string{turnEnd(repo, "a0000016")},
		sessionId:   recorded("a0000016", &SessionRecord{Named: true, Turn: Turn{State: "READY"}}),
	})

	// stop-set-user-title-keeps-wording
	tests = append(tests, &testCase{
		_id:             "stop-set-user-title-keeps-wording",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: stopBlock("CC,FD - My own title - [DESIGN-READY]"),

		args:        commitArgs,
		environment: desktopWithSource("local_0085", "My own title", "user"),
		events:      []string{turnEnd(repo, "a0000085")},
		sessionId:   recorded("a0000085", &SessionRecord{Named: true, Turn: designReady}),
	})

	// stop-wip-returns-from-declared
	tests = append(tests, &testCase{
		_id:             "stop-wip-returns-from-declared",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: stopBlock("CC,FD - Session title state structure - [WIP]"),

		args:        commitArgs,
		environment: desktop("local_0034", "CC,FD - Session title state structure - [DESIGN-READY]"),
		events:      []string{turnEnd(repo, "a0000034")},
		sessionId:   recorded("a0000034", &SessionRecord{Named: true, Turn: Turn{State: "WIP"}}),
	})

	// stop-wip-on-wip-silent
	tests = append(tests, &testCase{
		_id:             "stop-wip-on-wip-silent",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0035", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{turnEnd(repo, "a0000035")},
		sessionId:   recorded("a0000035", &SessionRecord{Named: true, Turn: Turn{State: "WIP"}}),
	})

	// stop-ready-blocks
	tests = append(tests, &testCase{
		_id:             "stop-ready-blocks",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: stopBlock("CC,FD - Session title state structure - [READY]"),

		args:        commitArgs,
		environment: desktop("local_0021", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{turnEnd(repo, "a0000021")},
		sessionId:   recorded("a0000021", &SessionRecord{Named: true, Turn: Turn{State: "READY"}}),
	})

	// stop-without-set-silent
	tests = append(tests, &testCase{
		_id:             "stop-without-set-silent",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0022", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{turnEnd(repo, "a0000022")},
		sessionId:   recorded("a0000022", &SessionRecord{Named: true}),
	})

	// stop-registry-from-state-dir
	tests = append(tests, &testCase{
		_id:             "stop-registry-from-state-dir",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: stopBlock("CC,FD - Session title state structure - [READY]"),

		args:        stateRegistryArgs,
		environment: desktop("local_0094", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{turnEnd(repo, "a0000094")},
		sessionId:   recorded("a0000094", &SessionRecord{Named: true, Turn: Turn{State: "READY"}}),
	})

	// stop-unformatted-auto
	tests = append(tests, &testCase{
		_id:             "stop-unformatted-auto",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: stopBlock("CC,FD - Session title state structure - [READY]"),

		args:        commitArgs,
		environment: desktopWithSource("local_0017", "Session title state structure", "auto"),
		events:      []string{turnEnd(repo, "a0000017")},
		sessionId:   recorded("a0000017", &SessionRecord{Named: true, Turn: Turn{State: "READY"}}),
	})

	// stop-removed-worktree-keeps-repo
	tests = append(tests, &testCase{
		_id:             "stop-removed-worktree-keeps-repo",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: stopBlock("CC,FD - Title system test - [READY]"),

		args:        commitArgs,
		environment: desktop("local_0019", "CC,FD - Title system test - [WIP]"),
		events:      []string{turnEnd(removedWorktree, "a0000019")},
		sessionId:   recorded("a0000019", &SessionRecord{Named: true, Turn: Turn{State: "READY"}}),
	})

	// stop-echo-title-spends-state
	tests = append(tests, &testCase{
		_id:             "stop-echo-title-spends-state",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktopWithSource("local_0041", "/fdesign change: Session title hook /peek", "auto"),
		events:      []string{turnEnd(repo, "a0000041")},
		sessionId:   recorded("a0000041", &SessionRecord{Named: true, Turn: designReady}),
	})

	// stop-keeps-saved-token
	tests = append(tests, &testCase{
		_id:             "stop-keeps-saved-token",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Fleet: "T1-MC", Named: true},
		_expectedStderr: "",
		_expectedStdout: stopBlock("CC,FD - Session title state structure - T1-MC[READY]"),

		args:        commitArgs,
		environment: desktop("local_0059", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{turnEnd(repo, "a0000059")},
		sessionId:   recorded("a0000059", &SessionRecord{Fleet: "T1-MC", Named: true, Turn: Turn{State: "READY"}}),
	})

	// stop-second-time-silent
	tests = append(tests, &testCase{
		_id:             "stop-second-time-silent",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: stopBlock("CC,FD - Session title state structure - [DESIGN-READY]"),

		args:        commitArgs,
		environment: desktop("local_0037", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{turnEnd(repo, "a0000037"), turnEnd(repo, "a0000037")},
		sessionId:   recorded("a0000037", &SessionRecord{Named: true, Turn: designReady}),
	})

	// stop-missing-store-file-keeps-state
	tests = append(tests, &testCase{
		_id:             "stop-missing-store-file-keeps-state",
		_expectedExit:   1,
		_expectedRecord: &SessionRecord{Named: true, Turn: designReady},
		_expectedStderr: "desktop.ReadSession: Expected one store file for local_9999, found 0\n",
		_expectedStdout: "",

		args:        commitArgs,
		environment: &Environment{EntryPoint: entryPointDesktop, HostSessionId: "local_9999", IsAttended: true},
		events:      []string{turnEnd(repo, "a0009999")},
		sessionId:   recorded("a0009999", &SessionRecord{Named: true, Turn: designReady}),
	})

	// set-edit-set-stop-backstop
	tests = append(tests, &testCase{
		_id:             "set-edit-set-stop-backstop",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: renameContext("CC,FD - Session title state structure - [DESIGN-READY]") + stopBlock("CC,FD - Session title state structure - [DESIGN-READY]"),

		args:        commitArgs,
		environment: desktop("local_0038", "CC,FD - Session title state structure - [WIP]"),
		events: []string{
			titleSet("a0000038", "DESIGN-READY"),
			otherToolCall("a0000038", "Edit"),
			titleSet("a0000038", "DESIGN-READY"),
			turnEnd(repo, "a0000038"),
		},
		sessionId: recorded("a0000038", &SessionRecord{Named: true}),
	})

	// close-check-closed-at-call
	tests = append(tests, &testCase{
		_id:             "close-check-closed-at-call",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: renameContext("CC,FD - Session title state structure - [CLOSED]"),

		args:        commitArgs,
		environment: desktop("local_0042", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{toolCall(repo, "a0000042", closeCheck, "prompts-fdesign.jsonl")},
		sessionId:   recorded("a0000042", &SessionRecord{Named: true}),
	})

	// close-ready-to-closed
	tests = append(tests, &testCase{
		_id:             "close-ready-to-closed",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: renameContext("CC,FD - Session title state structure - T1-MA[CLOSED]"),

		args:        commitArgs,
		environment: desktop("local_0043", "CC,FD - Session title state structure - T1-MA[READY]"),
		events:      []string{toolCall(repo, "a0000043", closeCheck, "prompts-fdesign.jsonl")},
		sessionId:   recorded("a0000043", &SessionRecord{Named: true}),
	})

	// close-testing-to-closed
	tests = append(tests, &testCase{
		_id:             "close-testing-to-closed",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: renameContext("CC,FD - Session title state structure - [CLOSED]"),

		args:        commitArgs,
		environment: desktop("local_0044", "CC,FD - Session title state structure - [TESTING]"),
		events:      []string{toolCall(repo, "a0000044", closeCheck, "prompts-fdesign.jsonl")},
		sessionId:   recorded("a0000044", &SessionRecord{Named: true}),
	})

	// close-done-to-closed
	tests = append(tests, &testCase{
		_id:             "close-done-to-closed",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: renameContext("CC,FD - Session title state structure - [CLOSED]"),

		args:        commitArgs,
		environment: desktop("local_0045", "CC,FD - Session title state structure - [DONE]"),
		events:      []string{toolCall(repo, "a0000045", closeCheck, "prompts-fdesign.jsonl")},
		sessionId:   recorded("a0000045", &SessionRecord{Named: true}),
	})

	// close-closed-stays
	tests = append(tests, &testCase{
		_id:             "close-closed-stays",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0050", "CC,FD - Session title state structure - [CLOSED]"),
		events:      []string{toolCall(repo, "a0000050", closeCheck, "prompts-fdesign.jsonl")},
		sessionId:   recorded("a0000050", &SessionRecord{Named: true}),
	})

	// close-unformatted-auto
	tests = append(tests, &testCase{
		_id:             "close-unformatted-auto",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: renameContext("CC,FD - Session title state structure - [CLOSED]"),

		args:        commitArgs,
		environment: desktopWithSource("local_0046", "Session title state structure", "auto"),
		events:      []string{toolCall(repo, "a0000046", closeCheck, "prompts-fdesign.jsonl")},
		sessionId:   recorded("a0000046", &SessionRecord{Named: true}),
	})

	// close-user-formatted-title-moves-state
	tests = append(tests, &testCase{
		_id:             "close-user-formatted-title-moves-state",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: renameContext("CC,FD - Reworded by hand - [CLOSED]"),

		args:        commitArgs,
		environment: desktopWithSource("local_0047", "CC,FD - Reworded by hand - [READY]", "user"),
		events:      []string{toolCall(repo, "a0000047", closeCheck, "prompts-fdesign.jsonl")},
		sessionId:   recorded("a0000047", &SessionRecord{Named: true}),
	})

	// close-echo-title-waits
	tests = append(tests, &testCase{
		_id:             "close-echo-title-waits",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktopWithSource("local_0048", "/close --terminal", "auto"),
		events:      []string{toolCall(repo, "a0000048", closeCheck, "prompts-fdesign.jsonl")},
		sessionId:   recorded("a0000048", &SessionRecord{Named: true}),
	})

	// close-non-check-bash
	tests = append(tests, &testCase{
		_id:             "close-non-check-bash",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0049", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{toolCall(repo, "a0000049", "~/go/bin/smine-agentwt remove --delete-branch claude/x", "prompts-fdesign.jsonl")},
		sessionId:   recorded("a0000049", &SessionRecord{Named: true}),
	})

	// close-declared-to-closed
	tests = append(tests, &testCase{
		_id:             "close-declared-to-closed",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: renameContext("CC,FD - Session title state structure - [CLOSED]"),

		args:        commitArgs,
		environment: desktop("local_0091", "CC,FD - Session title state structure - [CODE-READY]"),
		events:      []string{toolCall(repo, "a0000091", closeCheck, "prompts-fdesign.jsonl")},
		sessionId:   recorded("a0000091", &SessionRecord{Named: true}),
	})

	// join-saves-fleet-at-call
	tests = append(tests, &testCase{
		_id:             "join-saves-fleet-at-call",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Fleet: "T1-MA", Named: true},
		_expectedStderr: "",
		_expectedStdout: renameContext("CC,FD - Session title state structure - T1-MA[WIP]"),

		args:        commitArgs,
		environment: desktop("local_0051", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{bashResult("a0000051", joinCommand, "T1-MA\n")},
		sessionId:   recorded("a0000051", &SessionRecord{Named: true}),
	})

	// join-unformatted-starts-wip
	tests = append(tests, &testCase{
		_id:             "join-unformatted-starts-wip",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Fleet: "T1-MB", Named: true},
		_expectedStderr: "",
		_expectedStdout: renameContext("CC,FD - Session title state structure - T1-MB[WIP]"),

		args:        commitArgs,
		environment: desktopWithSource("local_0052", "Session title state structure", "auto"),
		events:      []string{bashResult("a0000052", joinCommand, "T1-MB\n")},
		sessionId:   recorded("a0000052", &SessionRecord{Named: true}),
	})

	// join-ready-keeps-state
	tests = append(tests, &testCase{
		_id:             "join-ready-keeps-state",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Fleet: "T2-MC", Named: true},
		_expectedStderr: "",
		_expectedStdout: renameContext("CC,FD - Session title state structure - T2-MC[READY]"),

		args:        commitArgs,
		environment: desktop("local_0053", "CC,FD - Session title state structure - T1-MA[READY]"),
		events:      []string{bashResult("a0000053", joinCommand, "T2-MC\n")},
		sessionId:   recorded("a0000053", &SessionRecord{Named: true}),
	})

	// join-echo-title-saves-token
	tests = append(tests, &testCase{
		_id:             "join-echo-title-saves-token",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Fleet: "T1-MA", Named: true},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktopWithSource("local_0054", "/join fleet:T1", "auto"),
		events:      []string{bashResult("a0000054", joinCommand, "T1-MA\n")},
		sessionId:   recorded("a0000054", &SessionRecord{Named: true}),
	})

	// join-refused-stdout
	tests = append(tests, &testCase{
		_id:             "join-refused-stdout",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0055", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{bashResult("a0000055", joinCommand, "")},
		sessionId:   recorded("a0000055", &SessionRecord{Named: true}),
	})

	// join-non-join-bash
	tests = append(tests, &testCase{
		_id:             "join-non-join-bash",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0056", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{bashResult("a0000056", "echo T1-MA", "T1-MA\n")},
		sessionId:   recorded("a0000056", &SessionRecord{Named: true}),
	})

	// subscribe-saves-fleet-at-call
	tests = append(tests, &testCase{
		_id:             "subscribe-saves-fleet-at-call",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Fleet: "T1-O", Named: true},
		_expectedStderr: "",
		_expectedStdout: renameContext("CC,FD - Session title state structure - T1-O[WIP]"),

		args:        commitArgs,
		environment: desktop("local_0061", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{bashResult("a0000061", "~/go/bin/smine-heartbeat subscribe --fleet T1 --target improvement/x", "subscribed a0000061 to fleet T1\n")},
		sessionId:   recorded("a0000061", &SessionRecord{Named: true}),
	})

	// subscribe-equals-form
	tests = append(tests, &testCase{
		_id:             "subscribe-equals-form",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Fleet: "T2-O", Named: true},
		_expectedStderr: "",
		_expectedStdout: renameContext("CC,FD - Session title state structure - T2-O[WIP]"),

		args:        commitArgs,
		environment: desktopWithSource("local_0062", "Session title state structure", "auto"),
		events:      []string{bashResult("a0000062", "~/go/bin/smine-heartbeat subscribe --target main --fleet=T2", "")},
		sessionId:   recorded("a0000062", &SessionRecord{Named: true}),
	})

	// subscribe-without-fleet
	tests = append(tests, &testCase{
		_id:             "subscribe-without-fleet",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0063", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{bashResult("a0000063", "~/go/bin/smine-heartbeat subscribe --help", "")},
		sessionId:   recorded("a0000063", &SessionRecord{Named: true}),
	})

	// done-at-call
	tests = append(tests, &testCase{
		_id:             "done-at-call",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Fleet: "T1-MA", Named: true},
		_expectedStderr: "",
		_expectedStdout: renameContext("CC,FD - Session title state structure - T1-MA[DONE]"),

		args:        commitArgs,
		environment: desktop("local_0057", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{bashResult("a0000057", doneCommand, "delivered T1-MA\n")},
		sessionId:   recorded("a0000057", &SessionRecord{Fleet: "T1-MA", Named: true}),
	})

	// no-record-silent
	tests = append(tests, &testCase{
		_id:             "no-record-silent",
		_expectedExit:   0,
		_expectedRecord: nil,
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0066", "CC,FD - Session title state structure - [WIP]"),
		events: []string{
			titleSet("a0000066", "DESIGN-READY"),
			turnEnd(repo, "a0000066"),
		},
		sessionId: "a0000066",
	})

	// invalid-declaration-fails
	tests = append(tests, &testCase{
		_id:             "invalid-declaration-fails",
		_expectedExit:   1,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "LoadVocabulary: LoadDocument: " + brokenDeclaration + ": ParseDocument: Unsupported schema 1\n",
		_expectedStdout: "",

		args:        brokenArgs,
		environment: desktop("local_0089", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{toolCall(repo, "a0000089", "ls", "prompts-fdesign.jsonl")},
		sessionId:   recorded("a0000089", &SessionRecord{Named: true}),
	})

	// pretooluse-allow-instructed-rename
	tests = append(tests, &testCase{
		_id:             "pretooluse-allow-instructed-rename",
		_expectedExit:   0,
		_expectedRecord: nil,
		_expectedStderr: "",
		_expectedStdout: renameAllow,

		args:        commitArgs,
		environment: desktopWithSource("local_0026", "Plan-archive skill extension", "auto"),
		events:      []string{renameCall("self", "CC,FD - Plan-archive skill extension - [WIP]")},
		sessionId:   "a0000026",
	})

	// pretooluse-other-session-asks
	tests = append(tests, &testCase{
		_id:             "pretooluse-other-session-asks",
		_expectedExit:   0,
		_expectedRecord: nil,
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktopWithSource("local_0027", "Plan-archive skill extension", "auto"),
		events:      []string{renameCall("local_1234", "CC,FD - Plan-archive skill extension - [WIP]")},
		sessionId:   "a0000027",
	})

	// pretooluse-unformatted-title-asks
	tests = append(tests, &testCase{
		_id:             "pretooluse-unformatted-title-asks",
		_expectedExit:   0,
		_expectedRecord: nil,
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktopWithSource("local_0028", "Plan-archive skill extension", "auto"),
		events:      []string{renameCall("self", "A better title")},
		sessionId:   "a0000028",
	})

	// pretooluse-user-title-allowed
	tests = append(tests, &testCase{
		_id:             "pretooluse-user-title-allowed",
		_expectedExit:   0,
		_expectedRecord: nil,
		_expectedStderr: "",
		_expectedStdout: renameAllow,

		args:        commitArgs,
		environment: desktopWithSource("local_0029", "My own title", "user"),
		events:      []string{renameCall("self", "CC,FD - My own title - [WIP]")},
		sessionId:   "a0000029",
	})

	// pretooluse-allow-declared-state
	tests = append(tests, &testCase{
		_id:             "pretooluse-allow-declared-state",
		_expectedExit:   0,
		_expectedRecord: nil,
		_expectedStderr: "",
		_expectedStdout: renameAllow,

		args:        commitArgs,
		environment: desktopWithSource("local_0092", "CC,FD - Plan-archive skill extension - [WIP]", "tool"),
		events:      []string{renameCall("self", "CC,FD - Plan-archive skill extension - [DESIGN-READY]")},
		sessionId:   "a0000092",
	})

	// pretooluse-undeclared-token-asks
	tests = append(tests, &testCase{
		_id:             "pretooluse-undeclared-token-asks",
		_expectedExit:   0,
		_expectedRecord: nil,
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktopWithSource("local_0093", "CC,FD - Plan-archive skill extension - [WIP]", "tool"),
		events:      []string{renameCall("self", "CC,FD - Plan-archive skill extension - [FOO-BAR]")},
		sessionId:   "a0000093",
	})

	// switch-off-stop-no-block
	tests = append(tests, &testCase{
		_id:             "switch-off-stop-no-block",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true, Turn: Turn{State: "READY"}},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0201", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{turnEnd(repo, "b0000001")},
		sessionId:   titleSwitchedOff(recorded("b0000001", &SessionRecord{Named: true, Turn: Turn{State: "READY"}})),
	})

	// switch-off-set-not-recorded
	tests = append(tests, &testCase{
		_id:             "switch-off-set-not-recorded",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{Named: true},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0202", "CC,FD - Session title state structure - [WIP]"),
		events:      []string{titleSet("b0000002", "READY")},
		sessionId:   titleSwitchedOff(recorded("b0000002", &SessionRecord{Named: true})),
	})

	// switch-off-sessionstart-still-arms
	tests = append(tests, &testCase{
		_id:             "switch-off-sessionstart-still-arms",
		_expectedExit:   0,
		_expectedRecord: &SessionRecord{},
		_expectedStderr: "",
		_expectedStdout: "",

		args:        commitArgs,
		environment: desktop("local_0203", ""),
		events:      []string{sessionStart("b0000003")},
		sessionId:   titleSwitchedOff("b0000003"),
	})

	// switch-off-pretooluse-still-allows
	tests = append(tests, &testCase{
		_id:             "switch-off-pretooluse-still-allows",
		_expectedExit:   0,
		_expectedRecord: nil,
		_expectedStderr: "",
		_expectedStdout: renameAllow,

		args:        commitArgs,
		environment: desktopWithSource("local_0204", "Plan-archive skill extension", "auto"),
		events:      []string{envelope(&HookInput{Cwd: repo, HookEventName: eventPreToolUse, SessionId: "b0000004", ToolInput: ToolInput{SessionId: "self", Title: "CC,FD - Plan-archive skill extension - [WIP]"}, ToolName: "mcp__ccd_session_mgmt__set_session_title"})},
		sessionId:   titleSwitchedOff("b0000004"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			exit, stderr, stdout := runEvents(test.args, test.environment, test.events)
			record, err := loadRecord(filepath.Join(recordsDir, test.sessionId+".json"))
			assert.NoError(t, err)
			assert.Equal(t, test._expectedExit, exit)
			assert.Equal(t, test._expectedRecord, record)
			assert.Equal(t, test._expectedStderr, stderr)
			assert.Equal(t, test._expectedStdout, stdout)
		})
	}
}

func TestRun_HookConfig(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedExit   int
		_expectedStderr string
		_expectedStdout string

		environment *Environment
		event       string
	}

	home := resolvedTempDir(t)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	configDir, err := os.UserConfigDir()
	require.NoError(t, err)
	claudeHome := filepath.Join(home, ".claude")
	recordsDir := filepath.Join(claudeHome, "sessiontitle", "sessions")

	hookConfig := defaultHookConfig(t)
	hookConfig.Sessiontitle.MaxTitleRunes = 20
	hookConfig.Sessiontitle.TriggerPatterns[1] = `(^|\s)wrap-up-now(\s|$)`
	writeHookConfig(t, claudeHome, hookConfig)

	repo := newFixtureRepo(t, filepath.Join(resolvedTempDir(t), "claude-configs"))
	reposPath := filepath.Join(resolvedTempDir(t), "repos.json")
	require.NoError(t, os.WriteFile(reposPath, []byte(`{"repos":[{"label":"CC","name":"claude-configs","path":"`+repo+`"}]}`), 0o644))
	skillsDir := filepath.Join(resolvedTempDir(t), "skills")
	writeSkillMd(t, skillsDir, "fdesign", "---\nname: fdesign\ntitle-code: FD\nversion: 1.0.0\n---\n")
	args := []string{"-repos", reposPath, "-skills", skillsDir}

	transcript, err := filepath.Abs(filepath.Join("testdata", "prompts-fdesign.jsonl"))
	require.NoError(t, err)
	desktopSession := func(hostId string, title string, titleSource string) *Environment {
		dir := filepath.Join(configDir, "Claude", "claude-code-sessions", "org", "proj")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		data, err := json.Marshal(&desktop.Session{Title: title, TitleSource: titleSource})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, hostId+".json"), data, 0o644))
		return &Environment{EntryPoint: entryPointDesktop, HostSessionId: hostId, IsAttended: true}
	}
	recorded := func(sessionId string, record *SessionRecord) string {
		require.NoError(t, saveRecord(filepath.Join(recordsDir, sessionId+".json"), record))
		return sessionId
	}
	toolCall := func(sessionId string, command string) string {
		data, err := json.Marshal(&HookInput{Cwd: repo, HookEventName: eventPostToolUse, SessionId: sessionId, ToolInput: ToolInput{Command: command}, ToolName: "Bash", TranscriptPath: transcript})
		require.NoError(t, err)
		return string(data)
	}
	renameContext := func(title string) string {
		instruction := `Session title: call mcp__ccd_session_mgmt__set_session_title with session_id "self" and title "` + title + `".` +
			hookConfig.Sessiontitle.InstructionPlanMode + hookConfig.Sessiontitle.InstructionTail
		quoted, err := json.Marshal(instruction)
		require.NoError(t, err)
		return `{"hookSpecificOutput":{"additionalContext":` + string(quoted) + `,"hookEventName":"PostToolUse"}}` + "\n"
	}

	tests := make([]*testCase, 0)

	// max-title-runes-from-config
	tests = append(tests, &testCase{
		_id:             "max-title-runes-from-config",
		_expectedExit:   0,
		_expectedStderr: "",
		_expectedStdout: renameContext("CC,FD - Ses… - [WIP]"),

		environment: desktopSession("local_0301", "Session title state structure", "auto"),
		event:       toolCall(recorded("c0000001", &SessionRecord{}), "ls"),
	})

	// trigger-pattern-from-config
	tests = append(tests, &testCase{
		_id:             "trigger-pattern-from-config",
		_expectedExit:   0,
		_expectedStderr: "",
		_expectedStdout: renameContext("CC,FD - Se… - [DONE]"),

		environment: desktopSession("local_0302", "CC,FD - Ses… - [WIP]", "tool"),
		event:       toolCall(recorded("c0000002", &SessionRecord{Named: true}), "wrap-up-now"),
	})

	// default-done-command-ignored-under-custom-pattern
	tests = append(tests, &testCase{
		_id:             "default-done-command-ignored-under-custom-pattern",
		_expectedExit:   0,
		_expectedStderr: "",
		_expectedStdout: "",

		environment: desktopSession("local_0303", "CC,FD - Ses… - [WIP]", "tool"),
		event:       toolCall(recorded("c0000003", &SessionRecord{Named: true}), "~/go/bin/smine-heartbeat done"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			exit, stderr, stdout := runEvents(args, test.environment, []string{test.event})
			assert.Equal(t, test._expectedExit, exit)
			assert.Equal(t, test._expectedStderr, stderr)
			assert.Equal(t, test._expectedStdout, stdout)
		})
	}
}

func TestRun_MissingHookConfig(t *testing.T) {
	home := resolvedTempDir(t)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	environment := &Environment{EntryPoint: entryPointDesktop, HostSessionId: "local_0401", IsAttended: true}
	event := `{"hook_event_name":"SessionStart","session_id":"d0000001","source":"startup"}`

	exit, stderr, stdout := runEvents([]string{"-skills", t.TempDir()}, environment, []string{event})

	assert.Equal(t, 1, exit)
	assert.Contains(t, stderr, hookconfig.Path(filepath.Join(home, ".claude")))
	assert.Empty(t, stdout)
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
func writeHookConfig(t *testing.T, claudeHome string, hookConfig *hookconfig.Config) {
	t.Helper()
	encoded, err := json.Marshal(hookConfig)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(hookconfig.Path(claudeHome)), 0o755))
	require.NoError(t, os.WriteFile(hookconfig.Path(claudeHome), encoded, 0o644))
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	output, err := shell.Run(context.Background(), dir, "git", args...)
	require.NoError(t, err, output)
	return strings.TrimSpace(output)
}

// newFixtureRepo creates a repo with one commit at dir, which must lie under
// a symlink-resolved root so it matches the absolute paths git reports.
func newFixtureRepo(t *testing.T, dir string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	gitOutput(t, dir, "init", "-q", "-b", "main")
	gitOutput(t, dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "--allow-empty", "-q", "-m", "init")
	return dir
}

func resolvedTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	return dir
}

// runEvents feeds the events to run in order, one process per event as the
// harness does; output accumulates and the exit is the highest seen.
func runEvents(args []string, environment *Environment, events []string) (int, string, string) {
	exit := 0
	stderr := &bytes.Buffer{}
	stdout := &bytes.Buffer{}
	for _, event := range events {
		exit = max(exit, run(context.Background(), args, environment, stderr, strings.NewReader(event), stdout))
	}
	return exit, stderr.String(), stdout.String()
}

func writeSkillMd(t *testing.T, skillsDir string, name string, content string) {
	t.Helper()
	dir := filepath.Join(skillsDir, name)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644))
}

func writeTitleJson(t *testing.T, skillsDir string, name string, content string) {
	t.Helper()
	dir := filepath.Join(skillsDir, name)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "title.json"), []byte(content), 0o644))
}

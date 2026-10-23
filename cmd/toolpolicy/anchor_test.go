package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/transcript"
)

// cwdLine is a transcript line that records only a cwd.
func cwdLine(cwd string) transcript.Line {
	line := transcript.Line{Cwd: cwd, Type: transcript.TypeUser}
	return line
}

// toolCallLine is an assistant line calling tool with id.
func toolCallLine(id string, tool string) transcript.Line {
	block := transcript.Block{Id: id, Name: tool, Type: transcript.BlockToolUse}
	line := transcript.Line{
		Message: transcript.Message{Content: transcript.Content{Blocks: []transcript.Block{block}}},
		Type:    transcript.TypeAssistant,
	}
	return line
}

// toolResultLine is a user line answering the call id, with the structured
// worktree path the harness records on a successful EnterWorktree.
func toolResultLine(id string, isError bool, worktreePath string) transcript.Line {
	block := transcript.Block{IsError: isError, ToolUseId: id, Type: transcript.BlockToolResult}
	line := transcript.Line{
		Message:       transcript.Message{Content: transcript.Content{Blocks: []transcript.Block{block}}},
		ToolUseResult: transcript.ToolUseResult{WorktreePath: worktreePath},
		Type:          transcript.TypeUser,
	}
	return line
}

// writeSubagentTranscript records cwd as the only line of agent's own
// transcript beside the parent transcript: under <session>/subagents, or
// under <session>/subagents/workflows/<run> when run is set.
func writeSubagentTranscript(t *testing.T, agent string, cwd string, parent string, run string, session string) {
	t.Helper()
	dir := filepath.Join(filepath.Dir(parent), session, subagentsDir)
	if run != "" {
		dir = filepath.Join(dir, workflowsDir, run)
	}
	require.NoError(t, os.MkdirAll(dir, 0o755))
	writeCwdTranscript(t, filepath.Join(dir, subagentTranscriptPrefix+agent+transcriptExtension), cwd)
}

// trackAnchor feeds lines through one tracker and returns the anchor.
func trackAnchor(lines []transcript.Line) string {
	tracker := newAnchorTracker()
	for index := range lines {
		tracker.observe(&lines[index])
	}
	return tracker.current
}

func TestAnchorTracker_Observe(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		lines []transcript.Line
	}

	start := "/repo/.claude/worktrees/a"
	entered := "/repo/.claude/worktrees/b"
	second := "/repo/.claude/worktrees/c"

	tests := make([]*testCase, 0)

	// first-cwd-is-start
	tests = append(tests, &testCase{
		_id:       "first-cwd-is-start",
		_expected: start,

		lines: []transcript.Line{cwdLine(start)},
	})

	// later-cwd-ignored
	tests = append(tests, &testCase{
		_id:       "later-cwd-ignored",
		_expected: start,

		lines: []transcript.Line{cwdLine(start), cwdLine("/repo")},
	})

	// enter-worktree-success-moves
	tests = append(tests, &testCase{
		_id:       "enter-worktree-success-moves",
		_expected: entered,

		lines: []transcript.Line{cwdLine(start), toolCallLine("e1", enterWorktreeTool), toolResultLine("e1", false, entered)},
	})

	// enter-worktree-error-keeps
	tests = append(tests, &testCase{
		_id:       "enter-worktree-error-keeps",
		_expected: start,

		lines: []transcript.Line{cwdLine(start), toolCallLine("e1", enterWorktreeTool), toolResultLine("e1", true, entered)},
	})

	// enter-worktree-empty-path-keeps
	tests = append(tests, &testCase{
		_id:       "enter-worktree-empty-path-keeps",
		_expected: start,

		lines: []transcript.Line{cwdLine(start), toolCallLine("e1", enterWorktreeTool), toolResultLine("e1", false, "")},
	})

	// exit-worktree-returns-to-start
	tests = append(tests, &testCase{
		_id:       "exit-worktree-returns-to-start",
		_expected: start,

		lines: []transcript.Line{
			cwdLine(start),
			toolCallLine("e1", enterWorktreeTool),
			toolResultLine("e1", false, entered),
			toolCallLine("x1", exitWorktreeTool),
			toolResultLine("x1", false, ""),
		},
	})

	// exit-worktree-error-keeps
	tests = append(tests, &testCase{
		_id:       "exit-worktree-error-keeps",
		_expected: entered,

		lines: []transcript.Line{
			cwdLine(start),
			toolCallLine("e1", enterWorktreeTool),
			toolResultLine("e1", false, entered),
			toolCallLine("x1", exitWorktreeTool),
			toolResultLine("x1", true, ""),
		},
	})

	// enter-exit-enter-sequence
	tests = append(tests, &testCase{
		_id:       "enter-exit-enter-sequence",
		_expected: second,

		lines: []transcript.Line{
			cwdLine(start),
			toolCallLine("e1", enterWorktreeTool),
			toolResultLine("e1", false, entered),
			toolCallLine("x1", exitWorktreeTool),
			toolResultLine("x1", false, ""),
			toolCallLine("e2", enterWorktreeTool),
			toolResultLine("e2", false, second),
		},
	})

	// result-without-pending-call-ignored
	tests = append(tests, &testCase{
		_id:       "result-without-pending-call-ignored",
		_expected: start,

		lines: []transcript.Line{cwdLine(start), toolResultLine("e9", false, entered)},
	})

	// unrelated-tool-result-ignored
	tests = append(tests, &testCase{
		_id:       "unrelated-tool-result-ignored",
		_expected: start,

		lines: []transcript.Line{cwdLine(start), toolCallLine("b1", toolNameBash), toolResultLine("b1", false, entered)},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, trackAnchor(test.lines))
		})
	}
}

func TestResolveAnchor(t *testing.T) {
	type testCase struct {
		_id       string
		_expected sessionAnchor

		input   *HookInput
		tracked string
	}

	base, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	repo := filepath.Join(base, "repo")
	worktrees := filepath.Join(repo, ".claude", "worktrees")
	parent := filepath.Join(worktrees, "wt")
	agent := filepath.Join(worktrees, "agent-a1")
	nameOnly := filepath.Join(worktrees, "agent-a9")
	lane := filepath.Join(worktrees, "wf_r-1")
	outside := filepath.Join(base, "elsewhere", "agent-a1")
	otherWorktree := filepath.Join(base, "other", ".claude", "worktrees", "x")
	for _, dir := range []string{filepath.Join(repo, ".git"), filepath.Join(agent, "deep"), nameOnly, parent, lane, outside, otherWorktree} {
		require.NoError(t, os.MkdirAll(dir, 0o755))
	}
	removed := filepath.Join(worktrees, "gone")
	evil := filepath.Join(worktrees, "evil")
	require.NoError(t, os.Symlink(outside, evil))

	parentTranscript := filepath.Join(base, "projects", "parent.jsonl")
	own := func(agentId string, run string, cwd string) {
		writeSubagentTranscript(t, agentId, cwd, parentTranscript, run, "s1")
	}
	own("a1", "", agent)
	own("w1", "wf_r", lane)
	own("o1", "", outside)
	own("d1", "", filepath.Join(agent, "deep"))
	own("c1", "", worktrees)
	own("x1", "", otherWorktree)
	own("n1", "", parent)
	own("e1", "", evil)
	own("g1", "", removed)
	subagents := filepath.Join(base, "projects", "s1", subagentsDir)
	noCwdLine := `{"type":"user","message":{"content":"hi"}}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(subagents, "agent-z1.jsonl"), []byte(noCwdLine), 0o644))
	malformedFirst := "{\"type\":\n" + `{"type":"user","cwd":"` + lane + `"}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(subagents, workflowsDir, "wf_r", "agent-m1.jsonl"), []byte(malformedFirst), 0o644))
	subagent := func(agentId string) *HookInput {
		input := &HookInput{AgentId: agentId, Cwd: parent, SessionId: "s1", TranscriptPath: parentTranscript}
		return input
	}

	tests := make([]*testCase, 0)

	// agent-tool-subagent-own-worktree
	tests = append(tests, &testCase{
		_id:       "agent-tool-subagent-own-worktree",
		_expected: sessionAnchor{path: agent, source: anchorSourceSubagent},

		input:   subagent("a1"),
		tracked: parent,
	})

	// workflow-subagent-own-worktree
	tests = append(tests, &testCase{
		_id:       "workflow-subagent-own-worktree",
		_expected: sessionAnchor{path: lane, source: anchorSourceSubagent},

		input:   subagent("w1"),
		tracked: parent,
	})

	// own-transcript-missing-parent-anchor
	tests = append(tests, &testCase{
		_id:       "own-transcript-missing-parent-anchor",
		_expected: sessionAnchor{path: parent, source: anchorSourceTranscript},

		input:   subagent("zz"),
		tracked: parent,
	})

	// envelope-cwd-name-match-ignored
	tests = append(tests, &testCase{
		_id:       "envelope-cwd-name-match-ignored",
		_expected: sessionAnchor{path: parent, source: anchorSourceTranscript},

		input:   &HookInput{AgentId: "a9", Cwd: nameOnly, SessionId: "s1", TranscriptPath: parentTranscript},
		tracked: parent,
	})

	// first-cwd-outside-worktrees
	tests = append(tests, &testCase{
		_id:       "first-cwd-outside-worktrees",
		_expected: sessionAnchor{path: parent, source: anchorSourceTranscript},

		input:   subagent("o1"),
		tracked: parent,
	})

	// first-cwd-nested-deeper
	tests = append(tests, &testCase{
		_id:       "first-cwd-nested-deeper",
		_expected: sessionAnchor{path: parent, source: anchorSourceTranscript},

		input:   subagent("d1"),
		tracked: parent,
	})

	// first-cwd-is-container-itself
	tests = append(tests, &testCase{
		_id:       "first-cwd-is-container-itself",
		_expected: sessionAnchor{path: parent, source: anchorSourceTranscript},

		input:   subagent("c1"),
		tracked: parent,
	})

	// first-cwd-other-repo-worktrees
	tests = append(tests, &testCase{
		_id:       "first-cwd-other-repo-worktrees",
		_expected: sessionAnchor{path: parent, source: anchorSourceTranscript},

		input:   subagent("x1"),
		tracked: parent,
	})

	// non-isolated-subagent-in-parent-worktree
	tests = append(tests, &testCase{
		_id:       "non-isolated-subagent-in-parent-worktree",
		_expected: sessionAnchor{path: parent, source: anchorSourceSubagent},

		input:   subagent("n1"),
		tracked: parent,
	})

	// symlinked-entry-pointing-outside
	tests = append(tests, &testCase{
		_id:       "symlinked-entry-pointing-outside",
		_expected: sessionAnchor{path: parent, source: anchorSourceTranscript},

		input:   subagent("e1"),
		tracked: parent,
	})

	// subagent-worktree-removed
	tests = append(tests, &testCase{
		_id:       "subagent-worktree-removed",
		_expected: sessionAnchor{isMissing: true, path: removed, source: anchorSourceSubagent},

		input:   subagent("g1"),
		tracked: parent,
	})

	// own-transcript-without-cwd-line
	tests = append(tests, &testCase{
		_id:       "own-transcript-without-cwd-line",
		_expected: sessionAnchor{path: parent, source: anchorSourceTranscript},

		input:   subagent("z1"),
		tracked: parent,
	})

	// malformed-line-before-cwd-line
	tests = append(tests, &testCase{
		_id:       "malformed-line-before-cwd-line",
		_expected: sessionAnchor{path: lane, source: anchorSourceSubagent},

		input:   subagent("m1"),
		tracked: parent,
	})

	// no-session-id-parent-anchor
	tests = append(tests, &testCase{
		_id:       "no-session-id-parent-anchor",
		_expected: sessionAnchor{path: parent, source: anchorSourceTranscript},

		input:   &HookInput{AgentId: "a1", Cwd: parent, TranscriptPath: parentTranscript},
		tracked: parent,
	})

	// no-parent-cwd-subagent-own-worktree
	tests = append(tests, &testCase{
		_id:       "no-parent-cwd-subagent-own-worktree",
		_expected: sessionAnchor{path: agent, source: anchorSourceSubagent},

		input:   subagent("a1"),
		tracked: "",
	})

	// no-parent-cwd-no-own-transcript-envelope
	tests = append(tests, &testCase{
		_id:       "no-parent-cwd-no-own-transcript-envelope",
		_expected: sessionAnchor{path: parent, source: anchorSourceEnvelope},

		input:   subagent("zz"),
		tracked: "",
	})

	// main-session-transcript
	tests = append(tests, &testCase{
		_id:       "main-session-transcript",
		_expected: sessionAnchor{path: parent, source: anchorSourceTranscript},

		input:   &HookInput{Cwd: repo},
		tracked: parent,
	})

	// missing-transcript-envelope
	tests = append(tests, &testCase{
		_id:       "missing-transcript-envelope",
		_expected: sessionAnchor{path: parent, source: anchorSourceEnvelope},

		input:   &HookInput{Cwd: parent, TranscriptPath: filepath.Join(base, "missing.jsonl")},
		tracked: "",
	})

	// no-cwd-line-envelope
	tests = append(tests, &testCase{
		_id:       "no-cwd-line-envelope",
		_expected: sessionAnchor{path: parent, source: anchorSourceEnvelope},

		input:   &HookInput{Cwd: parent},
		tracked: "",
	})

	// deleted-worktree-missing
	tests = append(tests, &testCase{
		_id:       "deleted-worktree-missing",
		_expected: sessionAnchor{isMissing: true, path: removed, source: anchorSourceTranscript},

		input:   &HookInput{Cwd: repo},
		tracked: removed,
	})

	worktreesDir := defaultHookConfig(t).Worktreecreate.WorktreesDir

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, resolveAnchor(test.input, test.tracked, worktreesDir))
		})
	}
}

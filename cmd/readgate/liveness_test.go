package main

import (
	"bytes"
	"encoding/json"
	"io"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/contextrequired"
	"github.com/kevinhorst/smine/internal/readledger"
)

const (
	// maxDepth bounds how deep a denied instructed Read may nest before the
	// agent is stuck; each level resolves a different owed file.
	maxDepth = 5
	// maxRounds bounds how often the agent retries one denied call; the
	// largest case (a 2500-line guide read 100 lines at a time) needs 25.
	maxRounds = 50
)

// readInstruction matches one Read instruction line of a deny reason.
var readInstruction = regexp.MustCompile(`(?m)^    Read (.+) offset (\d+) limit (\d+)$`)

// simulatedAgent drives the gate the way an agent does: make a call; on a
// deny, perform every listed Read — in listed or reversed order, following
// a denied Read's own instructions first — then retry. Reads return at most
// lineCap lines, as the harness token cap does.
type simulatedAgent struct {
	claudeHome string
	isReversed bool
	lineCap    int
}

// decisionReason runs the call's PreToolUse and returns its deny reason, ""
// on allow.
func (a *simulatedAgent) decisionReason(t *testing.T, call *HookInput) string {
	stdout := runEvent(t, call, a.claudeHome)
	if stdout == "" {
		return ""
	}

	output := &HookOutput{}
	err := json.Unmarshal([]byte(stdout), output)
	require.NoError(t, err)
	return output.HookSpecificOutput.PermissionDecisionReason
}

// follow performs one instructed Read: it must unblock by obeying, then its
// PostToolUse records the lines the capped harness returns.
func (a *simulatedAgent) follow(t *testing.T, call *HookInput, depth int, instruction []string) {
	path := instruction[1]
	offset, err := strconv.Atoi(instruction[2])
	require.NoError(t, err)

	limit, err := strconv.Atoi(instruction[3])
	require.NoError(t, err)

	read := provideToolCall(call.AgentId, call.Cwd, ToolInput{FilePath: path}, toolRead)
	require.True(t, a.obey(t, read, depth+1), "instructed Read of %s never unblocked — deadlock", path)

	returned := ResponseFile{FilePath: path, NumLines: min(limit, a.lineCap), StartLine: offset}
	runEvent(t, provideReadResult(call.AgentId, returned), a.claudeHome)
}

// obey retries the call, following every deny's Read instructions, and
// reports whether it passed within maxRounds.
func (a *simulatedAgent) obey(t *testing.T, call *HookInput, depth int) bool {
	require.Less(t, depth, maxDepth, "instructed Reads nest without end — deadlock")
	for range maxRounds {
		reason := a.decisionReason(t, call)
		if reason == "" {
			return true
		}

		instructions := readInstruction.FindAllStringSubmatch(reason, -1)
		require.NotEmpty(t, instructions, "deny without Read instructions: %s", reason)
		if a.isReversed {
			slices.Reverse(instructions)
		}

		for _, instruction := range instructions {
			a.follow(t, call, depth, instruction)
		}
	}
	return false
}

// TestRun_ObeyingDenialsUnblocks is the gate's liveness property: an agent
// that follows every deny's instructions, in any order and under any read
// cap, always gets its call through. A deadlock, a livelock or an
// instruction a Read cannot satisfy fails the case.
func TestRun_ObeyingDenialsUnblocks(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedAllowed bool

		agent *simulatedAgent
		call  *HookInput
	}

	tests := make([]*testCase, 0)
	gitStatus := ToolInput{Command: "git status"}

	// root-global-and-typed-skill
	claudeHome := t.TempDir()
	repo := t.TempDir()
	provideInjection(t, claudeHome, 104, contextrequired.GlobalFile)
	provideInjection(t, claudeHome, 577, "fdesign--main.md")
	provideLedger(t, "", claudeHome, provideSkill("/fdesign change", "fdesign"))
	tests = append(tests, &testCase{
		_id:              "root-global-and-typed-skill",
		_expectedAllowed: true,

		agent: &simulatedAgent{claudeHome: claudeHome, isReversed: false, lineCap: 2000},
		call:  provideToolCall("", repo, gitStatus, toolBash),
	})

	// root-global-and-typed-skill-reversed
	claudeHome = t.TempDir()
	repo = t.TempDir()
	provideInjection(t, claudeHome, 104, contextrequired.GlobalFile)
	provideInjection(t, claudeHome, 577, "fdesign--main.md")
	provideLedger(t, "", claudeHome, provideSkill("/fdesign change", "fdesign"))
	tests = append(tests, &testCase{
		_id:              "root-global-and-typed-skill-reversed",
		_expectedAllowed: true,

		agent: &simulatedAgent{claudeHome: claudeHome, isReversed: true, lineCap: 2000},
		call:  provideToolCall("", repo, gitStatus, toolBash),
	})

	// root-three-skills-and-guide-token-capped
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	provideInjection(t, claudeHome, 104, contextrequired.GlobalFile)
	provideInjection(t, claudeHome, 577, "fdesign--main.md")
	provideInjection(t, claudeHome, 440, "fimplement--main.md")
	provideInjection(t, claudeHome, 161, "package-commit--main.md")
	invocations := []readledger.Record{
		provideSkill("/fdesign", "fdesign"),
		provideSkill("plans/x.md", "fimplement"),
		provideSkill("", "package-commit"),
	}
	provideLedger(t, "", claudeHome, invocations...)
	edit := ToolInput{FilePath: filepath.Join(repo, "a.go")}
	tests = append(tests, &testCase{
		_id:              "root-three-skills-and-guide-token-capped",
		_expectedAllowed: true,

		agent: &simulatedAgent{claudeHome: claudeHome, isReversed: false, lineCap: 448},
		call:  provideToolCall("", repo, edit, toolEdit),
	})

	// large-guide-read-in-small-chunks
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	provideInjection(t, claudeHome, 104, contextrequired.GlobalFile)
	edit = ToolInput{FilePath: filepath.Join(repo, "b.py")}
	tests = append(tests, &testCase{
		_id:              "large-guide-read-in-small-chunks",
		_expectedAllowed: true,

		agent: &simulatedAgent{claudeHome: claudeHome, isReversed: true, lineCap: 100},
		call:  provideToolCall("", repo, edit, toolEdit),
	})

	// root-chunked-guide
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	provideFile(t, filepath.Join(repo, "docs", "rules", "go.md"), provideWideLines(3, contextrequired.ReadBudgetBytes-1))
	edit = ToolInput{FilePath: filepath.Join(repo, "a.go")}
	tests = append(tests, &testCase{
		_id:              "root-chunked-guide",
		_expectedAllowed: true,

		agent: &simulatedAgent{claudeHome: claudeHome, isReversed: false, lineCap: 2000},
		call:  provideToolCall("", repo, edit, toolEdit),
	})

	// subagent-global-lane-injection-and-bash-guide
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	provideInjection(t, claudeHome, 104, contextrequired.GlobalFile)
	provideInjection(t, claudeHome, 90, "railroad-review--lane-security--agent-9.md")
	provideLedger(t, testAgentId, claudeHome, provideSkill("lane:security", "railroad-review"))
	grep := ToolInput{Command: "grep -n foo a.go"}
	tests = append(tests, &testCase{
		_id:              "subagent-global-lane-injection-and-bash-guide",
		_expectedAllowed: true,

		agent: &simulatedAgent{claudeHome: claudeHome, isReversed: false, lineCap: 2000},
		call:  provideToolCall(testAgentId, repo, grep, toolBash),
	})

	// lane-manifest-files-and-global
	claudeHome = t.TempDir()
	repo = provideNotesTree(t)
	provideInjection(t, claudeHome, 104, contextrequired.GlobalFile)
	artifact := provideArtifactPath(t)
	provideManifest(t, artifact, provideLaneManifest(nil, repo, "notes/one.txt", "notes/two.txt"))
	write := ToolInput{Content: "{}", FilePath: artifact}
	tests = append(tests, &testCase{
		_id:              "lane-manifest-files-and-global",
		_expectedAllowed: true,

		agent: &simulatedAgent{claudeHome: claudeHome, isReversed: true, lineCap: 2000},
		call:  provideToolCall(testAgentId, repo, write, toolWrite),
	})

	// after-compaction-everything-re-owed-then-unblocks
	claudeHome = t.TempDir()
	repo = t.TempDir()
	global := provideInjection(t, claudeHome, 104, contextrequired.GlobalFile)
	fdesignInjection := provideInjection(t, claudeHome, 577, "fdesign--main.md")
	provideLedger(
		t,
		"",
		claudeHome,
		provideSkill("/fdesign", "fdesign"),
		provideRead(global, 1, 104),
		provideRead(fdesignInjection, 1, 577),
	)
	runEvent(t, provideSessionStart(readledger.SourceCompact), claudeHome)
	tests = append(tests, &testCase{
		_id:              "after-compaction-everything-re-owed-then-unblocks",
		_expectedAllowed: true,

		agent: &simulatedAgent{claudeHome: claudeHome, isReversed: false, lineCap: 2000},
		call:  provideToolCall("", repo, gitStatus, toolBash),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			isAllowed := test.agent.obey(t, test.call, 0)

			assert.Equal(t, test._expectedAllowed, isAllowed)
		})
	}
}

// runEvent runs one hook event in-process and returns its stdout; a failing
// exit code fails the test.
func runEvent(t *testing.T, input *HookInput, claudeHome string) string {
	stdout := &bytes.Buffer{}
	exitCode := run(
		t.Context(),
		provideArgs(t, claudeHome),
		io.Discard,
		provideEnvelope(t, input),
		stdout,
	)
	require.Equal(t, 0, exitCode)
	return strings.TrimSpace(stdout.String())
}

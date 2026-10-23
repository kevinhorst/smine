package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/contextrequired"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/readledger"
	"github.com/kevinhorst/smine/internal/reviewscope"
	"github.com/kevinhorst/smine/internal/sessionstate"
	"github.com/kevinhorst/smine/internal/shell"
)

const (
	goGuideLines     = 300
	goTestGuideLines = 120
	noteLines        = 40
	pythonGuideLines = 2500
	skillGuideLines  = 20
	testAgentId      = "agent-9"
	testRetention    = 168 * time.Hour
	testSessionId    = "session-1"
	unreadFormat     = "\n- %s — unread:\n    Read %s offset %d limit %d"
)

func TestRun_PreToolUse(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedCode     int
		_expectedRunError string
		_expectedStderr   string
		_expectedStdout   string

		args   []string
		stdin  *strings.Reader
		stdout *bytes.Buffer
	}

	tests := make([]*testCase, 0)

	// first-touch-denied-with-range
	claudeHome := t.TempDir()
	repo := provideContextTree(t)
	goGuide := filepath.Join(repo, "docs", "rules", "go.md")
	call := provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "a.go")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "first-touch-denied-with-range",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, toolEdit, provideUnread(goGuide, 1, goGuideLines)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// read-of-guide-itself-allowed
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	goGuide = filepath.Join(repo, "docs", "rules", "go.md")
	call = provideToolCall("", repo, ToolInput{FilePath: goGuide}, toolRead)
	tests = append(tests, &testCase{
		_id:             "read-of-guide-itself-allowed",
		_expectedCode:   0,
		_expectedStdout: "",

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// read-of-owed-file-allowed
	claudeHome = t.TempDir()
	repo = t.TempDir()
	global := provideInjection(t, claudeHome, 10, contextrequired.GlobalFile)
	call = provideToolCall("", repo, ToolInput{FilePath: global}, toolRead)
	tests = append(tests, &testCase{
		_id:             "read-of-owed-file-allowed",
		_expectedCode:   0,
		_expectedStdout: "",

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// full-read-record-allows
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	goGuide = filepath.Join(repo, "docs", "rules", "go.md")
	provideLedger(t, "", claudeHome, provideRead(goGuide, 1, goGuideLines))
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "a.go")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "full-read-record-allows",
		_expectedCode:   0,
		_expectedStdout: "",

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// second-glob-of-a-guide
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	pythonGuide := filepath.Join(repo, "docs", "rules", "python.md")
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "stub.pyi")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "second-glob-of-a-guide",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, toolEdit, provideUnread(pythonGuide, 1, pythonGuideLines)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// partial-read-lists-remaining
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	pythonGuide = filepath.Join(repo, "docs", "rules", "python.md")
	provideLedger(t, "", claudeHome, provideRead(pythonGuide, 1, 2000))
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "b.py")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "partial-read-lists-remaining",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, toolEdit, provideUnread(pythonGuide, 2001, 500)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// token-capped-read-owes-rest
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	goGuide = filepath.Join(repo, "docs", "rules", "go.md")
	provideLedger(t, "", claudeHome, provideRead(goGuide, 1, 120))
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "a.go")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "token-capped-read-owes-rest",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, toolEdit, provideUnread(goGuide, 121, goGuideLines-120)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// root-owes-its-typed-skill-injection
	claudeHome = t.TempDir()
	repo = t.TempDir()
	fdesignInjection := provideInjection(t, claudeHome, 5, "fdesign--main.md")
	global = provideInjection(t, claudeHome, 10, contextrequired.GlobalFile)
	provideLedger(t, "", claudeHome, provideSkill("/fdesign change the gate", "fdesign"))
	call = provideToolCall("", repo, ToolInput{Command: "git status"}, toolBash)
	bothUnread := provideUnread(fdesignInjection, 1, 5) + provideUnread(global, 1, 10)
	tests = append(tests, &testCase{
		_id:             "root-owes-its-typed-skill-injection",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, toolBash, bothUnread),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// root-does-not-owe-subagent-injection
	claudeHome = t.TempDir()
	repo = t.TempDir()
	provideInjection(t, claudeHome, 5, "railroad-review--lane-security--agent-9.md")
	provideLedger(t, testAgentId, claudeHome, provideSkill("lane:security", "railroad-review"))
	call = provideToolCall("", repo, ToolInput{Command: "git status"}, toolBash)
	tests = append(tests, &testCase{
		_id:             "root-does-not-owe-subagent-injection",
		_expectedCode:   0,
		_expectedStdout: "",

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// root-does-not-owe-old-shared-file
	claudeHome = t.TempDir()
	repo = t.TempDir()
	provideInjection(t, claudeHome, 5, "fdesign.md")
	global = provideInjection(t, claudeHome, 10, contextrequired.GlobalFile)
	provideLedger(t, "", claudeHome, provideSkill("/fdesign", "fdesign"))
	call = provideToolCall("", repo, ToolInput{Command: "git status"}, toolBash)
	tests = append(tests, &testCase{
		_id:             "root-does-not-owe-old-shared-file",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, toolBash, provideUnread(global, 1, 10)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// read-of-one-owed-file-allowed-while-another-is-unread
	claudeHome = t.TempDir()
	repo = t.TempDir()
	provideInjection(t, claudeHome, 5, "fdesign--main.md")
	global = provideInjection(t, claudeHome, 10, contextrequired.GlobalFile)
	provideLedger(t, "", claudeHome, provideSkill("/fdesign", "fdesign"))
	call = provideToolCall("", repo, ToolInput{FilePath: global}, toolRead)
	tests = append(tests, &testCase{
		_id:             "read-of-one-owed-file-allowed-while-another-is-unread",
		_expectedCode:   0,
		_expectedStdout: "",

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// root-owes-global-injection
	claudeHome = t.TempDir()
	repo = t.TempDir()
	global = provideInjection(t, claudeHome, 10, contextrequired.GlobalFile)
	call = provideToolCall("", repo, ToolInput{Command: "git status"}, toolBash)
	tests = append(tests, &testCase{
		_id:             "root-owes-global-injection",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, toolBash, provideUnread(global, 1, 10)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// subagent-owes-global-injection
	claudeHome = t.TempDir()
	repo = t.TempDir()
	global = provideInjection(t, claudeHome, 10, contextrequired.GlobalFile)
	provideInjection(t, claudeHome, 5, "fdesign--main.md")
	call = provideToolCall(testAgentId, repo, ToolInput{Command: "git status"}, toolBash)
	tests = append(tests, &testCase{
		_id:             "subagent-owes-global-injection",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, toolBash, provideUnread(global, 1, 10)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// subagent-does-not-owe-root-injection-of-same-skill
	claudeHome = t.TempDir()
	repo = t.TempDir()
	global = provideInjection(t, claudeHome, 10, contextrequired.GlobalFile)
	provideInjection(t, claudeHome, 5, "fdesign--main.md")
	provideLedger(t, testAgentId, claudeHome, provideSkill("change the gate", "fdesign"))
	call = provideToolCall(testAgentId, repo, ToolInput{Command: "git status"}, toolBash)
	tests = append(tests, &testCase{
		_id:             "subagent-does-not-owe-root-injection-of-same-skill",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, toolBash, provideUnread(global, 1, 10)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// subagent-owes-lane-injection-by-skill-context-name
	claudeHome = t.TempDir()
	repo = t.TempDir()
	laneInjection := provideInjection(t, claudeHome, 5, "railroad-review--lane-security--agent-9.md")
	provideInjection(t, claudeHome, 5, "railroad-review--agent-9.md")
	provideLedger(t, testAgentId, claudeHome, provideSkill("lane:security", "railroad-review"))
	call = provideToolCall(testAgentId, repo, ToolInput{Command: "git status"}, toolBash)
	tests = append(tests, &testCase{
		_id:             "subagent-owes-lane-injection-by-skill-context-name",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, toolBash, provideUnread(laneInjection, 1, 5)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// subagent-owes-topic-default-injection
	claudeHome = t.TempDir()
	repo = t.TempDir()
	topicInjection := provideInjection(t, claudeHome, 5, "discuss--topic-default--agent-9.md")
	provideLedger(t, testAgentId, claudeHome, provideSkill("", "discuss"))
	call = provideToolCall(testAgentId, repo, ToolInput{Command: "git status"}, toolBash)
	tests = append(tests, &testCase{
		_id:             "subagent-owes-topic-default-injection",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, toolBash, provideUnread(topicInjection, 1, 5)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// subagent-falls-back-to-plain-injection
	claudeHome = t.TempDir()
	repo = t.TempDir()
	fdesignInjection = provideInjection(t, claudeHome, 5, "fdesign--agent-9.md")
	provideLedger(t, testAgentId, claudeHome, provideSkill("change the gate", "fdesign"))
	call = provideToolCall(testAgentId, repo, ToolInput{Command: "git status"}, toolBash)
	tests = append(tests, &testCase{
		_id:             "subagent-falls-back-to-plain-injection",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, toolBash, provideUnread(fdesignInjection, 1, 5)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// subagent-no-skill-owes-no-skill-injection
	claudeHome = t.TempDir()
	repo = t.TempDir()
	provideInjection(t, claudeHome, 5, "fdesign.md")
	call = provideToolCall(testAgentId, repo, ToolInput{Command: "git status"}, toolBash)
	tests = append(tests, &testCase{
		_id:             "subagent-no-skill-owes-no-skill-injection",
		_expectedCode:   0,
		_expectedStdout: "",

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// unmatched-file-allows
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "notes.txt")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "unmatched-file-allows",
		_expectedCode:   0,
		_expectedStdout: "",

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// guide-missing-on-disk-allows
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "q.sql")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "guide-missing-on-disk-allows",
		_expectedCode:   0,
		_expectedStdout: "",

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// no-index-allows
	claudeHome = t.TempDir()
	repo = t.TempDir()
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "a.go")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "no-index-allows",
		_expectedCode:   0,
		_expectedStdout: "",

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// empty-ledger-still-gates
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	goGuide = filepath.Join(repo, "docs", "rules", "go.md")
	provideRawLedger(t, "", claudeHome, "")
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "a.go")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "empty-ledger-still-gates",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, toolEdit, provideUnread(goGuide, 1, goGuideLines)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// missing-config-errors
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "a.go")}, toolEdit)
	tests = append(tests, &testCase{
		_id:               "missing-config-errors",
		_expectedCode:     1,
		_expectedRunError: hookconfig.Path(claudeHome),
		_expectedStdout:   "",

		args:   []string{"-claude-home", claudeHome},
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// empty-context-dir-allows
	emptyDirConfig := provideHookConfig(t)
	emptyDirConfig.Context.ContextDir = ""
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "a.go")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "empty-context-dir-allows",
		_expectedCode:   0,
		_expectedStdout: "",

		args:   provideArgsWith(t, claudeHome, emptyDirConfig),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// context-dir-from-config
	knowledgeConfig := provideHookConfig(t)
	knowledgeConfig.Context.ContextDir = "knowledge"
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	require.NoError(t, os.Rename(filepath.Join(repo, "docs"), filepath.Join(repo, "knowledge")))
	goGuide = filepath.Join(repo, "knowledge", "rules", "go.md")
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "a.go")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "context-dir-from-config",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, toolEdit, provideUnread(goGuide, 1, goGuideLines)),

		args:   provideArgsWith(t, claudeHome, knowledgeConfig),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// deny-format-from-config
	denyConfig := provideHookConfig(t)
	denyConfig.Readgate.DenyFormat = "read first (%s):%s"
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	goGuide = filepath.Join(repo, "docs", "rules", "go.md")
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "a.go")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "deny-format-from-config",
		_expectedCode:   0,
		_expectedStdout: provideVerdict(t, "read first (Edit):"+provideUnread(goGuide, 1, goGuideLines)),

		args:   provideArgsWith(t, claudeHome, denyConfig),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// subagent-ungated-from-config
	rootOnlyConfig := provideHookConfig(t)
	rootOnlyConfig.Readgate.GateSubagents = false
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	provideInjection(t, claudeHome, 10, contextrequired.GlobalFile)
	call = provideToolCall(testAgentId, repo, ToolInput{FilePath: filepath.Join(repo, "a.go")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "subagent-ungated-from-config",
		_expectedCode:   0,
		_expectedStdout: "",

		args:   provideArgsWith(t, claudeHome, rootOnlyConfig),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// root-still-gated-when-subagents-off
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	goGuide = filepath.Join(repo, "docs", "rules", "go.md")
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "a.go")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "root-still-gated-when-subagents-off",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, toolEdit, provideUnread(goGuide, 1, goGuideLines)),

		args:   provideArgsWith(t, claudeHome, rootOnlyConfig),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// disabled-guide-not-owed
	disabledGuideConfig := provideHookConfig(t)
	disabledGuideConfig.Readgate.DisabledGuides = []string{"go.md"}
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "a.go")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "disabled-guide-not-owed",
		_expectedCode:   0,
		_expectedStdout: "",

		args:   provideArgsWith(t, claudeHome, disabledGuideConfig),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// disabled-guide-leaves-sibling-owed
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	goTestGuide := filepath.Join(repo, "docs", "rules", "go_test.md")
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "a_test.go")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "disabled-guide-leaves-sibling-owed",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, toolEdit, provideUnread(goTestGuide, 1, goTestGuideLines)),

		args:   provideArgsWith(t, claudeHome, disabledGuideConfig),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// bash-token-requires-guide
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	goGuide = filepath.Join(repo, "docs", "rules", "go.md")
	call = provideToolCall("", repo, ToolInput{Command: "grep -n foo a.go"}, toolBash)
	tests = append(tests, &testCase{
		_id:             "bash-token-requires-guide",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, toolBash, provideUnread(goGuide, 1, goGuideLines)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// bash-cat-does-not-count
	claudeHome = t.TempDir()
	repo = t.TempDir()
	global = provideInjection(t, claudeHome, 10, contextrequired.GlobalFile)
	call = provideToolCall("", repo, ToolInput{Command: "cat " + global}, toolBash)
	tests = append(tests, &testCase{
		_id:             "bash-cat-does-not-count",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, toolBash, provideUnread(global, 1, 10)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// bash-without-path-allows
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	call = provideToolCall("", repo, ToolInput{Command: "go test ./internal/foo"}, toolBash)
	tests = append(tests, &testCase{
		_id:             "bash-without-path-allows",
		_expectedCode:   0,
		_expectedStdout: "",

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// pending-guide-denies-unrelated-call
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	goGuide = filepath.Join(repo, "docs", "rules", "go.md")
	provideLedger(t, "", claudeHome, provideGuide(goGuide))
	call = provideToolCall("", repo, ToolInput{Command: "go test ./internal/foo"}, toolBash)
	tests = append(tests, &testCase{
		_id:             "pending-guide-denies-unrelated-call",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, toolBash, provideUnread(goGuide, 1, goGuideLines)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// pending-guide-read-in-full-allows
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	goGuide = filepath.Join(repo, "docs", "rules", "go.md")
	provideLedger(t, "", claudeHome, provideGuide(goGuide), provideRead(goGuide, 1, goGuideLines))
	call = provideToolCall("", repo, ToolInput{Command: "go test ./internal/foo"}, toolBash)
	tests = append(tests, &testCase{
		_id:             "pending-guide-read-in-full-allows",
		_expectedCode:   0,
		_expectedStdout: "",

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// pending-guide-allows-read-of-other-owed-file
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	goGuide = filepath.Join(repo, "docs", "rules", "go.md")
	global = provideInjection(t, claudeHome, 10, contextrequired.GlobalFile)
	provideLedger(t, "", claudeHome, provideGuide(goGuide))
	call = provideToolCall("", repo, ToolInput{FilePath: global}, toolRead)
	tests = append(tests, &testCase{
		_id:             "pending-guide-allows-read-of-other-owed-file",
		_expectedCode:   0,
		_expectedStdout: "",

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// pending-guide-missing-on-disk-allows
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	provideLedger(t, "", claudeHome, provideGuide(filepath.Join(repo, "docs", "rules", "gone.md")))
	call = provideToolCall("", repo, ToolInput{Command: "go test ./internal/foo"}, toolBash)
	tests = append(tests, &testCase{
		_id:             "pending-guide-missing-on-disk-allows",
		_expectedCode:   0,
		_expectedStdout: "",

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// legacy-ledger-lines-ignored
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	goGuide = filepath.Join(repo, "docs", "rules", "go.md")
	provideRawLedger(t, "", claudeHome, fmt.Sprintf("{\"file\":%q,\"s\":1,\"e\":%d}\n", goGuide, goGuideLines))
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "a.go")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "legacy-ledger-lines-ignored",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, toolEdit, provideUnread(goGuide, 1, goGuideLines)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// malformed-ledger-lines-tolerated
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	goGuide = filepath.Join(repo, "docs", "rules", "go.md")
	provideRawLedger(t, "", claudeHome, "garbage\n")
	provideLedger(t, "", claudeHome, provideRead(goGuide, 1, goGuideLines))
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "a.go")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "malformed-ledger-lines-tolerated",
		_expectedCode:   0,
		_expectedStdout: "",

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// foreign-tree-guide-required
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	lane := provideContextTree(t)
	laneGuide := filepath.Join(lane, "docs", "rules", "go.md")
	call = provideToolCall(testAgentId, repo, ToolInput{FilePath: filepath.Join(lane, "pkg", "a.go")}, toolRead)
	tests = append(tests, &testCase{
		_id:             "foreign-tree-guide-required",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, toolRead, provideUnread(laneGuide, 1, goGuideLines)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// identical-copy-counts
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	twin := provideContextTree(t)
	provideLedger(t, "", claudeHome, provideRead(filepath.Join(twin, "docs", "rules", "go.md"), 1, goGuideLines))
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "a.go")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "identical-copy-counts",
		_expectedCode:   0,
		_expectedStdout: "",

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// divergent-copy-does-not-count
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	goGuide = filepath.Join(repo, "docs", "rules", "go.md")
	drift := provideContextTree(t)
	driftGuide := filepath.Join(drift, "docs", "rules", "go.md")
	provideFile(t, driftGuide, provideLines(goGuideLines+1))
	provideLedger(t, "", claudeHome, provideRead(driftGuide, 1, goGuideLines+1))
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "a.go")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "divergent-copy-does-not-count",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, toolEdit, provideUnread(goGuide, 1, goGuideLines)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// relative-glob-matches
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	skillGuide := filepath.Join(repo, "docs", "rules", "skills.md")
	skillMd := filepath.Join(repo, "skills", "quality", "foo", "SKILL.md")
	call = provideToolCall("", repo, ToolInput{FilePath: skillMd}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "relative-glob-matches",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, toolEdit, provideUnread(skillGuide, 1, skillGuideLines)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// missing-session-id-exits-1
	claudeHome = t.TempDir()
	tests = append(tests, &testCase{
		_id:             "missing-session-id-exits-1",
		_expectedCode:   1,
		_expectedStdout: "",

		args:   provideArgs(t, claudeHome),
		stdin:  strings.NewReader(`{"hook_event_name": "PreToolUse", "tool_name": "Bash"}`),
		stdout: &bytes.Buffer{},
	})

	// root-owes-plan-mode-injection
	claudeHome = t.TempDir()
	repo = t.TempDir()
	planInjection := provideInjection(t, claudeHome, 7, contextrequired.PlanModeFile)
	call = provideToolCall("", repo, ToolInput{Command: "git status"}, toolBash)
	tests = append(tests, &testCase{
		_id:             "root-owes-plan-mode-injection",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, toolBash, provideUnread(planInjection, 1, 7)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// root-read-of-plan-mode-injection-allowed
	claudeHome = t.TempDir()
	repo = t.TempDir()
	planInjection = provideInjection(t, claudeHome, 7, contextrequired.PlanModeFile)
	call = provideToolCall("", repo, ToolInput{FilePath: planInjection}, toolRead)
	tests = append(tests, &testCase{
		_id:             "root-read-of-plan-mode-injection-allowed",
		_expectedCode:   0,
		_expectedStdout: "",

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// subagent-does-not-owe-plan-mode-injection
	claudeHome = t.TempDir()
	repo = t.TempDir()
	provideInjection(t, claudeHome, 7, contextrequired.PlanModeFile)
	call = provideToolCall(testAgentId, repo, ToolInput{Command: "git status"}, toolBash)
	tests = append(tests, &testCase{
		_id:             "subagent-does-not-owe-plan-mode-injection",
		_expectedCode:   0,
		_expectedStdout: "",

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// session-title-rename-allowed-before-readings
	claudeHome = t.TempDir()
	repo = t.TempDir()
	provideInjection(t, claudeHome, 10, contextrequired.GlobalFile)
	call = provideToolCall("", repo, ToolInput{}, "mcp__ccd_session_mgmt__set_session_title")
	tests = append(tests, &testCase{
		_id:             "session-title-rename-allowed-before-readings",
		_expectedCode:   0,
		_expectedStdout: "",

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// tool-search-passes-before-readings
	claudeHome = t.TempDir()
	repo = t.TempDir()
	provideInjection(t, claudeHome, 10, contextrequired.GlobalFile)
	call = provideToolCall("", repo, ToolInput{}, "ToolSearch")
	tests = append(tests, &testCase{
		_id:             "tool-search-passes-before-readings",
		_expectedCode:   0,
		_expectedStdout: "",

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// peek-passes-with-pending-guide
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	goGuide = filepath.Join(repo, "docs", "rules", "go.md")
	provideLedger(t, "", claudeHome, provideGuide(goGuide))
	call = provideToolCall("", repo, ToolInput{}, "mcp__peek-mcp__session_get")
	tests = append(tests, &testCase{
		_id:             "peek-passes-with-pending-guide",
		_expectedCode:   0,
		_expectedStdout: "",

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// grep-denied-with-pending-guide
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	goGuide = filepath.Join(repo, "docs", "rules", "go.md")
	provideLedger(t, "", claudeHome, provideGuide(goGuide))
	call = provideToolCall("", repo, ToolInput{}, "Grep")
	tests = append(tests, &testCase{
		_id:             "grep-denied-with-pending-guide",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, "Grep", provideUnread(goGuide, 1, goGuideLines)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// test-file-owes-both-go-guides
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	goGuide = filepath.Join(repo, "docs", "rules", "go.md")
	goTestGuide = filepath.Join(repo, "docs", "rules", "go_test.md")
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "a_test.go")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "test-file-owes-both-go-guides",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, toolEdit, provideUnread(goGuide, 1, goGuideLines)+provideUnread(goTestGuide, 1, goTestGuideLines)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// deny-chunks-guide-over-budget
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	goGuide = filepath.Join(repo, "docs", "rules", "go.md")
	provideFile(t, goGuide, provideWideLines(3, contextrequired.ReadBudgetBytes-1))
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "a.go")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "deny-chunks-guide-over-budget",
		_expectedCode:   0,
		_expectedStdout: provideDeny(t, toolEdit, provideUnreadChunks(goGuide, []int{1, 2, 3})),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// switch-off-extension-allows
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	provideSwitchesOff(t, claudeHome, sessionstate.Ext("go"))
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "a.go")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "switch-off-extension-allows",
		_expectedCode:   0,
		_expectedStderr: "",
		_expectedStdout: "",

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// switch-off-extension-covers-every-guide
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	provideSwitchesOff(t, claudeHome, sessionstate.Ext("go"))
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "a_test.go")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "switch-off-extension-covers-every-guide",
		_expectedCode:   0,
		_expectedStderr: "",
		_expectedStdout: "",

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// switch-off-guide-name-owes-the-other
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	goGuide = filepath.Join(repo, "docs", "rules", "go.md")
	provideSwitchesOff(t, claudeHome, sessionstate.Ext("go_test"))
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "a_test.go")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "switch-off-guide-name-owes-the-other",
		_expectedCode:   0,
		_expectedStderr: "",
		_expectedStdout: provideDeny(t, toolEdit, provideUnread(goGuide, 1, goGuideLines)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// switch-off-guide-name-drops-pending-guide
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	provideLedger(t, "", claudeHome, provideGuide(filepath.Join(repo, "docs", "rules", "skills.md")))
	provideSwitchesOff(t, claudeHome, sessionstate.Ext("skills"))
	call = provideToolCall("", repo, ToolInput{Command: "ls skills"}, toolBash)
	tests = append(tests, &testCase{
		_id:             "switch-off-guide-name-drops-pending-guide",
		_expectedCode:   0,
		_expectedStderr: "",
		_expectedStdout: "",

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// switch-off-extension-drops-pending-guide
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	provideLedger(t, "", claudeHome, provideGuide(filepath.Join(repo, "docs", "rules", "go_test.md")))
	provideSwitchesOff(t, claudeHome, sessionstate.Ext("go"))
	call = provideToolCall("", repo, ToolInput{Command: "ls skills"}, toolBash)
	tests = append(tests, &testCase{
		_id:             "switch-off-extension-drops-pending-guide",
		_expectedCode:   0,
		_expectedStderr: "",
		_expectedStdout: "",

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// switch-off-other-extension-keeps-pending-guide
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	goGuide = filepath.Join(repo, "docs", "rules", "go.md")
	provideLedger(t, "", claudeHome, provideGuide(goGuide))
	provideSwitchesOff(t, claudeHome, sessionstate.Ext("py"))
	call = provideToolCall("", repo, ToolInput{Command: "ls skills"}, toolBash)
	tests = append(tests, &testCase{
		_id:             "switch-off-other-extension-keeps-pending-guide",
		_expectedCode:   0,
		_expectedStderr: "",
		_expectedStdout: provideDeny(t, toolBash, provideUnread(goGuide, 1, goGuideLines)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// switch-off-global-keeps-skill-injection
	claudeHome = t.TempDir()
	repo = t.TempDir()
	fdesignInjection = provideInjection(t, claudeHome, 5, "fdesign--main.md")
	provideInjection(t, claudeHome, 10, contextrequired.GlobalFile)
	provideLedger(t, "", claudeHome, provideSkill("/fdesign change the gate", "fdesign"))
	provideSwitchesOff(t, claudeHome, sessionstate.RgateGlobal)
	call = provideToolCall("", repo, ToolInput{Command: "git status"}, toolBash)
	tests = append(tests, &testCase{
		_id:             "switch-off-global-keeps-skill-injection",
		_expectedCode:   0,
		_expectedStderr: "",
		_expectedStdout: provideDeny(t, toolBash, provideUnread(fdesignInjection, 1, 5)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// switch-off-other-member-still-denies
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	goGuide = filepath.Join(repo, "docs", "rules", "go.md")
	provideSwitchesOff(t, claudeHome, sessionstate.RgateGlobal, sessionstate.Heartbeat)
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "a.go")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "switch-off-other-member-still-denies",
		_expectedCode:   0,
		_expectedStderr: "",
		_expectedStdout: provideDeny(t, toolEdit, provideUnread(goGuide, 1, goGuideLines)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// record-garbage-denies-and-logs
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	goGuide = filepath.Join(repo, "docs", "rules", "go.md")
	garbage := provideGarbageRecord(t, claudeHome)
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "a.go")}, toolEdit)
	tests = append(tests, &testCase{
		_id:             "record-garbage-denies-and-logs",
		_expectedCode:   0,
		_expectedStderr: fmt.Sprintf("readgate: offFeatures: Off: Load: Failed to decode %s: unexpected end of JSON input\n", garbage),
		_expectedStdout: provideDeny(t, toolEdit, provideUnread(goGuide, 1, goGuideLines)),

		args:   provideArgs(t, claudeHome),
		stdin:  provideEnvelope(t, call),
		stdout: &bytes.Buffer{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			stderr := provideStderr(t)
			runStderr := &bytes.Buffer{}
			exitCode := run(
				t.Context(),
				test.args,
				runStderr,
				test.stdin,
				test.stdout,
			)

			assert.Equal(t, test._expectedCode, exitCode)
			assert.Contains(t, runStderr.String(), test._expectedRunError)
			assert.Equal(t, test._expectedStderr, readStderr(t, stderr))
			assert.Equal(t, test._expectedStdout, test.stdout.String())
		})
	}
}

func TestRun_PreToolUseRecordsGuide(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedLedger []readledger.Record

		args       []string
		claudeHome string
		stdin      *strings.Reader
	}

	tests := make([]*testCase, 0)

	// denied-touch-records-guide
	claudeHome := t.TempDir()
	repo := provideContextTree(t)
	goGuide := filepath.Join(repo, "docs", "rules", "go.md")
	call := provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "a.go")}, toolRead)
	tests = append(tests, &testCase{
		_id:             "denied-touch-records-guide",
		_expectedLedger: []readledger.Record{provideGuide(goGuide)},

		args:       provideArgs(t, claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, call),
	})

	// second-denied-touch-records-once
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	goGuide = filepath.Join(repo, "docs", "rules", "go.md")
	provideLedger(t, "", claudeHome, provideGuide(goGuide))
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "a.go")}, toolRead)
	tests = append(tests, &testCase{
		_id:             "second-denied-touch-records-once",
		_expectedLedger: []readledger.Record{provideGuide(goGuide)},

		args:       provideArgs(t, claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, call),
	})

	// bash-token-touch-records-guide
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	goGuide = filepath.Join(repo, "docs", "rules", "go.md")
	call = provideToolCall("", repo, ToolInput{Command: "cat a.go"}, toolBash)
	tests = append(tests, &testCase{
		_id:             "bash-token-touch-records-guide",
		_expectedLedger: []readledger.Record{provideGuide(goGuide)},

		args:       provideArgs(t, claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, call),
	})

	// allowed-touch-records-nothing
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	goGuide = filepath.Join(repo, "docs", "rules", "go.md")
	provideLedger(t, "", claudeHome, provideRead(goGuide, 1, goGuideLines))
	call = provideToolCall("", repo, ToolInput{FilePath: filepath.Join(repo, "a.go")}, toolRead)
	tests = append(tests, &testCase{
		_id:             "allowed-touch-records-nothing",
		_expectedLedger: []readledger.Record{provideRead(goGuide, 1, goGuideLines)},

		args:       provideArgs(t, claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, call),
	})

	// denial-without-touch-records-nothing
	claudeHome = t.TempDir()
	repo = provideContextTree(t)
	provideInjection(t, claudeHome, 10, contextrequired.GlobalFile)
	call = provideToolCall("", repo, ToolInput{Command: "go test ./internal/foo"}, toolBash)
	tests = append(tests, &testCase{
		_id:             "denial-without-touch-records-nothing",
		_expectedLedger: nil,

		args:       provideArgs(t, claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, call),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			run(
				t.Context(),
				test.args,
				io.Discard,
				test.stdin,
				io.Discard,
			)

			assert.Equal(t, test._expectedLedger, readLedger(t, "", test.claudeHome))
		})
	}
}

func TestRun_LaneManifest(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedCoverage string
		_expectedStdout   string

		args     []string
		artifact string
		stdin    *strings.Reader
		stdout   *bytes.Buffer
	}

	tests := make([]*testCase, 0)
	fileFirstWalk := &contextdocs.Walk{
		Groups: []contextdocs.WalkGroup{{Id: "g1"}},
		Shape:  contextdocs.WalkShapeFileFirst,
	}
	groupFirstWalk := &contextdocs.Walk{
		Groups: []contextdocs.WalkGroup{{Id: "correctness/2"}},
		Shape:  contextdocs.WalkShapeGroupFirst,
	}
	completeEmptyWalk := `{"walk_complete": true, "walk": [], "findings": []}`

	// manifest-write-denied
	claudeHome := t.TempDir()
	repo := provideNotesTree(t)
	artifact := provideArtifactPath(t)
	manifestWrite := ToolInput{Content: "{}", FilePath: artifact + reviewscope.LaneManifestSuffix}
	call := provideToolCall(testAgentId, repo, manifestWrite, toolWrite)
	tests = append(tests, &testCase{
		_id:               "manifest-write-denied",
		_expectedCoverage: "",
		_expectedStdout:   provideVerdict(t, provideHookConfig(t).Readgate.ManifestWriteReason),

		args:     provideArgs(t, claudeHome),
		artifact: artifact,
		stdin:    provideEnvelope(t, call),
		stdout:   &bytes.Buffer{},
	})

	// manifest-files-required
	claudeHome = t.TempDir()
	repo = provideNotesTree(t)
	artifact = provideArtifactPath(t)
	provideManifest(t, artifact, provideLaneManifest(nil, repo, "notes/one.txt"))
	call = provideToolCall(testAgentId, repo, ToolInput{Content: "{}", FilePath: artifact}, toolWrite)
	noteOne := filepath.Join(repo, "notes", "one.txt")
	tests = append(tests, &testCase{
		_id:               "manifest-files-required",
		_expectedCoverage: "",
		_expectedStdout:   provideDeny(t, toolWrite, provideUnread(noteOne, 1, noteLines)),

		args:     provideArgs(t, claudeHome),
		artifact: artifact,
		stdin:    provideEnvelope(t, call),
		stdout:   &bytes.Buffer{},
	})

	// manifest-deny-writes-no-coverage
	claudeHome = t.TempDir()
	repo = provideNotesTree(t)
	artifact = provideArtifactPath(t)
	provideManifest(t, artifact, provideLaneManifest(nil, repo, "notes/one.txt", "notes/two.txt"))
	provideLedger(t, testAgentId, claudeHome, provideRead(filepath.Join(repo, "notes", "one.txt"), 1, noteLines))
	call = provideToolCall(testAgentId, repo, ToolInput{Content: "{}", FilePath: artifact}, toolWrite)
	noteTwo := filepath.Join(repo, "notes", "two.txt")
	tests = append(tests, &testCase{
		_id:               "manifest-deny-writes-no-coverage",
		_expectedCoverage: "",
		_expectedStdout:   provideDeny(t, toolWrite, provideUnread(noteTwo, 1, noteLines)),

		args:     provideArgs(t, claudeHome),
		artifact: artifact,
		stdin:    provideEnvelope(t, call),
		stdout:   &bytes.Buffer{},
	})

	// manifest-allow-writes-coverage
	claudeHome = t.TempDir()
	repo = provideNotesTree(t)
	artifact = provideArtifactPath(t)
	provideManifest(t, artifact, provideLaneManifest(nil, repo, "notes/one.txt", "notes/absent.txt"))
	provideLedger(t, testAgentId, claudeHome, provideRead(filepath.Join(repo, "notes", "one.txt"), 1, noteLines))
	call = provideToolCall(testAgentId, repo, ToolInput{Content: "{}", FilePath: artifact}, toolWrite)
	tests = append(tests, &testCase{
		_id:               "manifest-allow-writes-coverage",
		_expectedCoverage: provideCoverage(t, "notes/one.txt"),
		_expectedStdout:   "",

		args:     provideArgs(t, claudeHome),
		artifact: artifact,
		stdin:    provideEnvelope(t, call),
		stdout:   &bytes.Buffer{},
	})

	// manifest-malformed-fails-open
	claudeHome = t.TempDir()
	repo = provideNotesTree(t)
	artifact = provideArtifactPath(t)
	provideFile(t, artifact+reviewscope.LaneManifestSuffix, []byte("not json"))
	call = provideToolCall(testAgentId, repo, ToolInput{Content: "{}", FilePath: artifact}, toolWrite)
	tests = append(tests, &testCase{
		_id:               "manifest-malformed-fails-open",
		_expectedCoverage: "",
		_expectedStdout:   "",

		args:     provideArgs(t, claudeHome),
		artifact: artifact,
		stdin:    provideEnvelope(t, call),
		stdout:   &bytes.Buffer{},
	})

	// manifest-empty-files-allows
	claudeHome = t.TempDir()
	repo = provideNotesTree(t)
	artifact = provideArtifactPath(t)
	provideManifest(t, artifact, provideLaneManifest(nil, repo))
	call = provideToolCall(testAgentId, repo, ToolInput{Content: "{}", FilePath: artifact}, toolWrite)
	tests = append(tests, &testCase{
		_id:               "manifest-empty-files-allows",
		_expectedCoverage: provideCoverage(t),
		_expectedStdout:   "",

		args:     provideArgs(t, claudeHome),
		artifact: artifact,
		stdin:    provideEnvelope(t, call),
		stdout:   &bytes.Buffer{},
	})

	// manifest-empty-root-uses-cwd
	claudeHome = t.TempDir()
	repo = provideNotesTree(t)
	artifact = provideArtifactPath(t)
	provideManifest(t, artifact, provideLaneManifest(nil, "", "notes/one.txt"))
	call = provideToolCall(testAgentId, repo, ToolInput{Content: "{}", FilePath: artifact}, toolWrite)
	noteOne = filepath.Join(repo, "notes", "one.txt")
	tests = append(tests, &testCase{
		_id:               "manifest-empty-root-uses-cwd",
		_expectedCoverage: "",
		_expectedStdout:   provideDeny(t, toolWrite, provideUnread(noteOne, 1, noteLines)),

		args:     provideArgs(t, claudeHome),
		artifact: artifact,
		stdin:    provideEnvelope(t, call),
		stdout:   &bytes.Buffer{},
	})

	// manifest-per-artifact-no-cross-match
	claudeHome = t.TempDir()
	repo = provideNotesTree(t)
	manifested := provideArtifactPath(t)
	provideManifest(t, manifested, provideLaneManifest(nil, repo, "notes/one.txt"))
	artifact = filepath.Join(filepath.Dir(manifested), "other.json")
	call = provideToolCall(testAgentId, repo, ToolInput{Content: "{}", FilePath: artifact}, toolWrite)
	tests = append(tests, &testCase{
		_id:               "manifest-per-artifact-no-cross-match",
		_expectedCoverage: "",
		_expectedStdout:   "",

		args:     provideArgs(t, claudeHome),
		artifact: artifact,
		stdin:    provideEnvelope(t, call),
		stdout:   &bytes.Buffer{},
	})

	// head-mismatch-denied
	claudeHome = t.TempDir()
	repo = provideNotesTree(t)
	head := provideGitHead(t, repo)
	artifact = provideArtifactPath(t)
	pinned := provideLaneManifest(nil, repo, "notes/one.txt")
	pinned.Head = "0000000000000000000000000000000000000000"
	provideManifest(t, artifact, pinned)
	call = provideToolCall(testAgentId, repo, ToolInput{Content: "{}", FilePath: artifact}, toolWrite)
	headReason := fmt.Sprintf("read-gate: lane worktree is at %s, the lane manifest pins %s — review the pinned head", head, pinned.Head)
	tests = append(tests, &testCase{
		_id:               "head-mismatch-denied",
		_expectedCoverage: "",
		_expectedStdout:   provideVerdict(t, headReason),

		args:     provideArgs(t, claudeHome),
		artifact: artifact,
		stdin:    provideEnvelope(t, call),
		stdout:   &bytes.Buffer{},
	})

	// walk-complete-write-denied
	claudeHome = t.TempDir()
	repo = provideNotesTree(t)
	artifact = provideArtifactPath(t)
	provideManifest(t, artifact, provideLaneManifest(fileFirstWalk, repo, "notes/one.txt"))
	provideLedger(t, testAgentId, claudeHome, provideRead(filepath.Join(repo, "notes", "one.txt"), 1, noteLines))
	call = provideToolCall(testAgentId, repo, ToolInput{Content: completeEmptyWalk, FilePath: artifact}, toolWrite)
	tests = append(tests, &testCase{
		_id:               "walk-complete-write-denied",
		_expectedCoverage: "",
		_expectedStdout:   provideDeny(t, toolWrite, provideWalkGap(artifact, "notes/one.txt × g1")),

		args:     provideArgs(t, claudeHome),
		artifact: artifact,
		stdin:    provideEnvelope(t, call),
		stdout:   &bytes.Buffer{},
	})

	// walk-edit-reads-disk
	claudeHome = t.TempDir()
	repo = provideNotesTree(t)
	artifact = provideArtifactPath(t)
	provideManifest(t, artifact, provideLaneManifest(fileFirstWalk, repo, "notes/one.txt"))
	provideFile(t, artifact, []byte(`{"walk_complete": false, "walk": [{"file": "notes/one.txt", "group": "g1"}]}`))
	provideLedger(t, testAgentId, claudeHome, provideRead(filepath.Join(repo, "notes", "one.txt"), 1, noteLines))
	markComplete := ToolInput{FilePath: artifact, NewString: `"walk_complete": true`}
	call = provideToolCall(testAgentId, repo, markComplete, toolEdit)
	tests = append(tests, &testCase{
		_id:               "walk-edit-reads-disk",
		_expectedCoverage: provideCoverage(t, "notes/one.txt"),
		_expectedStdout:   "",

		args:     provideArgs(t, claudeHome),
		artifact: artifact,
		stdin:    provideEnvelope(t, call),
		stdout:   &bytes.Buffer{},
	})

	// walk-group-first
	claudeHome = t.TempDir()
	repo = provideNotesTree(t)
	artifact = provideArtifactPath(t)
	provideManifest(t, artifact, provideLaneManifest(groupFirstWalk, repo))
	call = provideToolCall(testAgentId, repo, ToolInput{Content: completeEmptyWalk, FilePath: artifact}, toolWrite)
	tests = append(tests, &testCase{
		_id:               "walk-group-first",
		_expectedCoverage: "",
		_expectedStdout:   provideDeny(t, toolWrite, provideWalkGap(artifact, "— × correctness/2")),

		args:     provideArgs(t, claudeHome),
		artifact: artifact,
		stdin:    provideEnvelope(t, call),
		stdout:   &bytes.Buffer{},
	})

	// walk-absent-allows
	claudeHome = t.TempDir()
	repo = provideNotesTree(t)
	artifact = provideArtifactPath(t)
	provideManifest(t, artifact, provideLaneManifest(nil, repo))
	call = provideToolCall(testAgentId, repo, ToolInput{Content: completeEmptyWalk, FilePath: artifact}, toolWrite)
	tests = append(tests, &testCase{
		_id:               "walk-absent-allows",
		_expectedCoverage: provideCoverage(t),
		_expectedStdout:   "",

		args:     provideArgs(t, claudeHome),
		artifact: artifact,
		stdin:    provideEnvelope(t, call),
		stdout:   &bytes.Buffer{},
	})

	// walk-malformed-fails-open
	claudeHome = t.TempDir()
	repo = provideNotesTree(t)
	artifact = provideArtifactPath(t)
	provideManifest(t, artifact, provideLaneManifest(fileFirstWalk, repo))
	call = provideToolCall(testAgentId, repo, ToolInput{Content: "not json at all", FilePath: artifact}, toolWrite)
	tests = append(tests, &testCase{
		_id:               "walk-malformed-fails-open",
		_expectedCoverage: provideCoverage(t),
		_expectedStdout:   "",

		args:     provideArgs(t, claudeHome),
		artifact: artifact,
		stdin:    provideEnvelope(t, call),
		stdout:   &bytes.Buffer{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			run(
				t.Context(),
				test.args,
				io.Discard,
				test.stdin,
				test.stdout,
			)

			assert.Equal(t, test._expectedStdout, test.stdout.String())
			assert.Equal(t, test._expectedCoverage, readCoverage(t, test.artifact))
		})
	}
}

func TestRun_PostToolUse(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedLedger []readledger.Record

		agentId    string
		args       []string
		claudeHome string
		stdin      *strings.Reader
	}

	tests := make([]*testCase, 0)

	// post-read-records-returned-range
	claudeHome := t.TempDir()
	returned := ResponseFile{FilePath: "/repo/docs/rules/go.md", NumLines: 448, StartLine: 1}
	tests = append(tests, &testCase{
		_id:             "post-read-records-returned-range",
		_expectedLedger: []readledger.Record{provideRead("/repo/docs/rules/go.md", 1, 448)},

		agentId:    "",
		args:       provideArgs(t, claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, provideReadResult("", returned)),
	})

	// post-read-without-file-records-nothing
	claudeHome = t.TempDir()
	image := ResponseFile{FilePath: "", NumLines: 0, StartLine: 0}
	tests = append(tests, &testCase{
		_id:             "post-read-without-file-records-nothing",
		_expectedLedger: nil,

		agentId:    "",
		args:       provideArgs(t, claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, provideReadResult("", image)),
	})

	// post-skill-records-args
	claudeHome = t.TempDir()
	skill := ToolInput{Args: "lane:security", Skill: "railroad-review"}
	tests = append(tests, &testCase{
		_id:             "post-skill-records-args",
		_expectedLedger: []readledger.Record{provideSkill("lane:security", "railroad-review")},

		agentId:    testAgentId,
		args:       provideArgs(t, claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, provideSkillResult(testAgentId, skill)),
	})

	// post-other-tool-records-nothing
	claudeHome = t.TempDir()
	bashResult := `{"session_id": "session-1", "hook_event_name": "PostToolUse", "tool_name": "Bash", "tool_response": "ok"}`
	tests = append(tests, &testCase{
		_id:             "post-other-tool-records-nothing",
		_expectedLedger: nil,

		agentId:    "",
		args:       provideArgs(t, claudeHome),
		claudeHome: claudeHome,
		stdin:      strings.NewReader(bashResult),
	})

	// prompt-slash-records-skill
	claudeHome = t.TempDir()
	tests = append(tests, &testCase{
		_id:             "prompt-slash-records-skill",
		_expectedLedger: []readledger.Record{provideSkill("/fdesign change lane:x", "fdesign")},

		agentId:    "",
		args:       provideArgs(t, claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, providePrompt("/fdesign change lane:x")),
	})

	// prompt-plain-records-nothing
	claudeHome = t.TempDir()
	tests = append(tests, &testCase{
		_id:             "prompt-plain-records-nothing",
		_expectedLedger: nil,

		agentId:    "",
		args:       provideArgs(t, claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, providePrompt("fix the gate /fdesign")),
	})

	// prompt-bare-slash-records-nothing
	claudeHome = t.TempDir()
	tests = append(tests, &testCase{
		_id:             "prompt-bare-slash-records-nothing",
		_expectedLedger: nil,

		agentId:    "",
		args:       provideArgs(t, claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, providePrompt("/ fdesign")),
	})

	// subagent-ledger-separate-from-root
	claudeHome = t.TempDir()
	returned = ResponseFile{FilePath: "/repo/a.go", NumLines: 10, StartLine: 1}
	tests = append(tests, &testCase{
		_id:             "subagent-ledger-separate-from-root",
		_expectedLedger: nil,

		agentId:    "",
		args:       provideArgs(t, claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, provideReadResult(testAgentId, returned)),
	})

	// switch-off-post-tool-still-records
	claudeHome = t.TempDir()
	provideSwitchesOff(t, claudeHome, sessionstate.Ext("go"))
	returned = ResponseFile{FilePath: "/repo/docs/rules/go.md", NumLines: 448, StartLine: 1}
	tests = append(tests, &testCase{
		_id:             "switch-off-post-tool-still-records",
		_expectedLedger: []readledger.Record{provideRead("/repo/docs/rules/go.md", 1, 448)},

		agentId:    "",
		args:       provideArgs(t, claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, provideReadResult("", returned)),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			run(
				t.Context(),
				test.args,
				io.Discard,
				test.stdin,
				io.Discard,
			)

			assert.Equal(t, test._expectedLedger, readLedger(t, test.agentId, test.claudeHome))
		})
	}
}

func TestRun_SessionStart(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedLedger    []readledger.Record
		_expectedRunStderr string

		args       []string
		claudeHome string
		stdin      *strings.Reader
	}

	tests := make([]*testCase, 0)
	earlierRead := provideRead("/repo/docs/rules/go.md", 1, 10)

	invocation := provideSkill("/fdesign", "fdesign")
	filedContext := provideContext("/home/context-required/session-1/fdesign--main.md", []string{"RULE-A-001"}, 1, 5)
	inlineContext := provideContext("", []string{"RULE-A-001"}, 0, 0)

	// session-start-compact-resets-root
	claudeHome := t.TempDir()
	provideLedger(t, "", claudeHome, earlierRead)
	tests = append(tests, &testCase{
		_id:             "session-start-compact-resets-root",
		_expectedLedger: nil,

		args:       provideArgs(t, claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, provideSessionStart(readledger.SourceCompact)),
	})

	// session-start-compact-keeps-invocations
	claudeHome = t.TempDir()
	provideLedger(t, "", claudeHome, earlierRead, invocation)
	tests = append(tests, &testCase{
		_id:             "session-start-compact-keeps-invocations",
		_expectedLedger: []readledger.Record{invocation},

		args:       provideArgs(t, claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, provideSessionStart(readledger.SourceCompact)),
	})

	// session-start-compact-keeps-filed-context
	claudeHome = t.TempDir()
	provideLedger(t, "", claudeHome, earlierRead, invocation, filedContext)
	tests = append(tests, &testCase{
		_id:             "session-start-compact-keeps-filed-context",
		_expectedLedger: []readledger.Record{invocation, filedContext},

		args:       provideArgs(t, claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, provideSessionStart(readledger.SourceCompact)),
	})

	// session-start-compact-drops-inline-context
	claudeHome = t.TempDir()
	provideLedger(t, "", claudeHome, inlineContext)
	tests = append(tests, &testCase{
		_id:             "session-start-compact-drops-inline-context",
		_expectedLedger: nil,

		args:       provideArgs(t, claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, provideSessionStart(readledger.SourceCompact)),
	})

	// session-start-compact-drops-pending-guide
	claudeHome = t.TempDir()
	provideLedger(t, "", claudeHome, invocation, provideGuide("/repo/docs/rules/go.md"))
	tests = append(tests, &testCase{
		_id:             "session-start-compact-drops-pending-guide",
		_expectedLedger: []readledger.Record{invocation},

		args:       provideArgs(t, claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, provideSessionStart(readledger.SourceCompact)),
	})

	// session-start-clear-resets-root
	claudeHome = t.TempDir()
	provideLedger(t, "", claudeHome, earlierRead)
	tests = append(tests, &testCase{
		_id:             "session-start-clear-resets-root",
		_expectedLedger: nil,

		args:       provideArgs(t, claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, provideSessionStart(readledger.SourceClear)),
	})

	// session-start-startup-keeps-ledger
	claudeHome = t.TempDir()
	provideLedger(t, "", claudeHome, earlierRead)
	tests = append(tests, &testCase{
		_id:             "session-start-startup-keeps-ledger",
		_expectedLedger: []readledger.Record{earlierRead},

		args:       provideArgs(t, claudeHome),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, provideSessionStart("startup")),
	})

	// empty-context-dir-names-setting
	emptyDirConfig := provideHookConfig(t)
	emptyDirConfig.Context.ContextDir = ""
	claudeHome = t.TempDir()
	provideLedger(t, "", claudeHome, earlierRead)
	tests = append(tests, &testCase{
		_id:                "empty-context-dir-names-setting",
		_expectedLedger:    []readledger.Record{earlierRead},
		_expectedRunStderr: noContextDirFormat + "\n",

		args:       provideArgsWith(t, claudeHome, emptyDirConfig),
		claudeHome: claudeHome,
		stdin:      provideEnvelope(t, provideSessionStart("startup")),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			runStderr := &bytes.Buffer{}
			run(
				t.Context(),
				test.args,
				runStderr,
				test.stdin,
				io.Discard,
			)

			assert.Equal(t, test._expectedLedger, readLedger(t, "", test.claudeHome))
			assert.Equal(t, test._expectedRunStderr, runStderr.String())
		})
	}
}

// provideArgs writes the default hooks config into the home and returns the hook's args.
func provideArgs(t *testing.T, claudeHome string) []string {
	return provideArgsWith(t, claudeHome, provideHookConfig(t))
}

// provideArgsWith is provideArgs with the given hooks config.
func provideArgsWith(t *testing.T, claudeHome string, hookConfig *hookconfig.Config) []string {
	writeHookConfig(t, claudeHome, hookConfig)
	return []string{"-claude-home", claudeHome}
}

// provideArtifactPath returns a findings artifact path in a fresh lane dir.
func provideArtifactPath(t *testing.T) string {
	return filepath.Join(t.TempDir(), "findings.json")
}

// provideContextTree builds a repo whose docs/context.json indexes a Go
// guide, a Go test guide, a Python guide with two globs, a skills guide matched
// through the tree-relative path and an SQL guide missing on disk; returns the root.
func provideContextTree(t *testing.T) string {
	repo := t.TempDir()
	rules := filepath.Join(repo, "docs", "rules")
	provideFile(t, filepath.Join(rules, "go.md"), provideLines(goGuideLines))
	provideFile(t, filepath.Join(rules, "go_test.md"), provideLines(goTestGuideLines))
	provideFile(t, filepath.Join(rules, "python.md"), provideLines(pythonGuideLines))
	provideFile(t, filepath.Join(rules, "skills.md"), provideLines(skillGuideLines))

	guides := []contextdocs.RuleGuide{
		{Files: []string{"*.go"}, Path: "rules/go.md"},
		{Files: []string{"*.py", "*.pyi"}, Path: "rules/python.md"},
		{Files: []string{"skills/**/SKILL.md"}, Path: "rules/skills.md"},
		{Files: []string{"*.sql"}, Path: "rules/sql.md"},
		{Files: []string{"*_test.go", "*_test_data.go"}, Path: "rules/go_test.md"},
	}
	index := &contextIndex{Guides: guides}
	data, err := json.Marshal(index)
	require.NoError(t, err)

	provideFile(t, filepath.Join(repo, "docs", contextdocs.ContextFileName), data)
	return repo
}

func provideContext(file string, ids []string, start int, end int) readledger.Record {
	return readledger.Record{End: end, File: file, Ids: ids, Kind: readledger.KindContext, Start: start}
}

func provideCoverage(t *testing.T, files ...string) string {
	coverage := &laneCoverage{Files: append([]string{}, files...)}
	data, err := json.MarshalIndent(coverage, "", "  ")
	require.NoError(t, err)
	return string(data) + "\n"
}

func provideDeny(t *testing.T, tool string, missing string) string {
	section := provideHookConfig(t).Readgate
	reason := fmt.Sprintf(section.DenyFormat, tool, missing)
	if tool == toolBash {
		reason += section.BashAddendum
	}
	return provideVerdict(t, reason)
}

func provideEnvelope(t *testing.T, input *HookInput) *strings.Reader {
	data, err := json.Marshal(input)
	require.NoError(t, err)
	return strings.NewReader(string(data))
}

func provideFile(t *testing.T, path string, content []byte) {
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	require.NoError(t, err)

	err = os.WriteFile(path, content, 0o644)
	require.NoError(t, err)
}

// provideGarbageRecord writes an undecodable switch record for the test session and returns its path.
func provideGarbageRecord(t *testing.T, claudeHome string) string {
	path := sessionstate.File(sessionstate.StateDir(claudeHome), testSessionId)
	provideFile(t, path, []byte("{"))
	return path
}

// provideGitHead commits once in a fresh repo at dir and returns HEAD.
func provideGitHead(t *testing.T, dir string) string {
	_, err := shell.Run(t.Context(), dir, "git", "init", "-q")
	require.NoError(t, err)

	_, err = shell.Run(
		t.Context(),
		dir,
		"git",
		"-c", "user.name=test",
		"-c", "user.email=test@example.com",
		"-c", "commit.gpgsign=false",
		"commit", "-q", "--allow-empty", "-m", "init",
	)
	require.NoError(t, err)

	head, err := shell.Run(t.Context(), dir, "git", "rev-parse", "HEAD")
	require.NoError(t, err)
	return strings.TrimSpace(head)
}

func provideGuide(file string) readledger.Record {
	return readledger.Record{File: file, Kind: readledger.KindGuide}
}

// provideHookConfig is the catalog's default hooks config with the context dir set to docs.
func provideHookConfig(t *testing.T) *hookconfig.Config {
	hookConfig := &hookconfig.Config{}
	hookComponents := []components.ID{components.Context, components.Gate, components.Worktrees, components.SessionHelpers}
	for _, id := range hookComponents {
		rendered, err := install.RenderDefaults(id, components.SinkHooksConfig)
		require.NoError(t, err)

		err = json.Unmarshal(rendered, hookConfig)
		require.NoError(t, err)
	}

	hookConfig.Context.ContextDir = "docs"
	return hookConfig
}

func provideLaneManifest(walk *contextdocs.Walk, root string, files ...string) *reviewscope.LaneManifest {
	manifest := &reviewscope.LaneManifest{
		Files: append([]string{}, files...),
		Root:  root,
		Walk:  walk,
	}
	return manifest
}

// provideLedger appends the records to the agent's ledger ("" is the root).
func provideLedger(t *testing.T, agentId string, claudeHome string, records ...readledger.Record) {
	ledger := readledger.New(agentId, claudeHome, testRetention, testSessionId)
	for _, record := range records {
		err := ledger.Append(&record)
		require.NoError(t, err)
	}
}

func provideLines(count int) []byte {
	return bytes.Repeat([]byte("rule\n"), count)
}

func provideManifest(t *testing.T, artifact string, manifest *reviewscope.LaneManifest) {
	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	provideFile(t, artifact+reviewscope.LaneManifestSuffix, data)
}

// provideNotesTree builds a lane root holding notes/one.txt and
// notes/two.txt, noteLines lines each.
func provideNotesTree(t *testing.T) string {
	repo := t.TempDir()
	provideFile(t, filepath.Join(repo, "notes", "one.txt"), provideLines(noteLines))
	provideFile(t, filepath.Join(repo, "notes", "two.txt"), provideLines(noteLines))
	return repo
}

// provideRawLedger appends raw lines to the agent's ledger.
func provideRawLedger(t *testing.T, agentId string, claudeHome string, content string) {
	ledger := readledger.New(agentId, claudeHome, testRetention, testSessionId)
	err := os.MkdirAll(filepath.Dir(ledger.Path()), 0o755)
	require.NoError(t, err)

	file, err := os.OpenFile(ledger.Path(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	defer file.Close()

	_, err = file.WriteString(content)
	require.NoError(t, err)
}

func providePrompt(prompt string) *HookInput {
	input := &HookInput{
		Cwd:           "/repo",
		HookEventName: eventPromptSubmit,
		Prompt:        prompt,
		SessionId:     testSessionId,
	}
	return input
}

// provideInjection writes an injection file of count lines into the session
// dir and returns its path.
func provideInjection(t *testing.T, claudeHome string, count int, name string) string {
	path := filepath.Join(contextrequired.Dir(claudeHome, testSessionId), name)
	provideFile(t, path, provideLines(count))
	return path
}

func provideRead(file string, start int, end int) readledger.Record {
	return readledger.Record{End: end, File: file, Kind: readledger.KindRead, Start: start}
}

func provideReadResult(agentId string, returned ResponseFile) *HookInput {
	input := &HookInput{
		AgentId:       agentId,
		Cwd:           "/repo",
		HookEventName: eventPostToolUse,
		SessionId:     testSessionId,
		ToolName:      toolRead,
		ToolResponse:  ToolResponse{File: returned},
	}
	return input
}

func provideSessionStart(source string) *HookInput {
	input := &HookInput{
		HookEventName: eventSessionStart,
		SessionId:     testSessionId,
		Source:        source,
	}
	return input
}

func provideSkill(args string, skill string) readledger.Record {
	return readledger.Record{Args: args, Kind: readledger.KindSkill, Skill: skill}
}

func provideSkillResult(agentId string, skill ToolInput) *HookInput {
	input := &HookInput{
		AgentId:       agentId,
		Cwd:           "/repo",
		HookEventName: eventPostToolUse,
		SessionId:     testSessionId,
		ToolInput:     skill,
		ToolName:      toolSkill,
	}
	return input
}

// provideStderr points os.Stderr, where isOff logs, at a file for the test's duration.
func provideStderr(t *testing.T) *os.File {
	stderr, err := os.Create(filepath.Join(t.TempDir(), "stderr"))
	require.NoError(t, err)

	original := os.Stderr
	os.Stderr = stderr
	t.Cleanup(func() {
		os.Stderr = original
		stderr.Close()
	})
	return stderr
}

// provideSwitchesOff writes the test session's switch record with the given features off.
func provideSwitchesOff(t *testing.T, claudeHome string, features ...sessionstate.Feature) {
	state := &sessionstate.SessionState{Switches: sessionstate.Switches{Off: features}}
	err := sessionstate.Save(sessionstate.File(sessionstate.StateDir(claudeHome), testSessionId), state)
	require.NoError(t, err)
}

func provideToolCall(agentId string, cwd string, toolInput ToolInput, toolName string) *HookInput {
	input := &HookInput{
		AgentId:       agentId,
		Cwd:           cwd,
		HookEventName: eventPreToolUse,
		SessionId:     testSessionId,
		ToolInput:     toolInput,
		ToolName:      toolName,
	}
	return input
}

func provideUnread(path string, offset int, limit int) string {
	return fmt.Sprintf(unreadFormat, path, path, offset, limit)
}

// provideUnreadChunks renders the unread line of path with one single-line Read per start.
func provideUnreadChunks(path string, starts []int) string {
	unread := &strings.Builder{}
	fmt.Fprintf(unread, "\n- %s — unread:", path)
	for _, start := range starts {
		fmt.Fprintf(unread, "\n    Read %s offset %d limit 1", path, start)
	}
	return unread.String()
}

func provideVerdict(t *testing.T, reason string) string {
	output := &HookOutput{
		HookSpecificOutput: HookDecision{
			HookEventName:            eventPreToolUse,
			PermissionDecision:       decisionDeny,
			PermissionDecisionReason: reason,
		},
	}
	data, err := json.Marshal(output)
	require.NoError(t, err)
	return string(data) + "\n"
}

func provideWalkGap(artifact string, pair string) string {
	return fmt.Sprintf("\n- %s — walk incomplete, unanswered (file × group; file empty on group-first):\n    %s", artifact, pair)
}

// provideWideLines returns count lines of bytesPerLine bytes each, newline included.
func provideWideLines(count int, bytesPerLine int) []byte {
	line := append(bytes.Repeat([]byte("x"), bytesPerLine-1), '\n')
	return bytes.Repeat(line, count)
}

// readCoverage returns the artifact's coverage record, "" when none was
// written.
func readCoverage(t *testing.T, artifact string) string {
	data, err := os.ReadFile(artifact + coverageSuffix)
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}

	require.NoError(t, err)
	return string(data)
}

// readLedger returns the agent's ledger records, nil when it has none.
func readLedger(t *testing.T, agentId string, claudeHome string) []readledger.Record {
	records, err := readledger.New(agentId, claudeHome, testRetention, testSessionId).Load()
	require.NoError(t, err)
	if len(records) == 0 {
		return nil
	}
	return records
}

func readStderr(t *testing.T, stderr *os.File) string {
	data, err := os.ReadFile(stderr.Name())
	require.NoError(t, err)
	return string(data)
}

// writeHookConfig writes a complete hooks config where the install renders it.
func writeHookConfig(t *testing.T, claudeHome string, hookConfig *hookconfig.Config) {
	encoded, err := json.Marshal(hookConfig)
	require.NoError(t, err)
	provideFile(t, hookconfig.Path(claudeHome), encoded)
}

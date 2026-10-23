package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/contextrequired"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/readledger"
	"github.com/kevinhorst/smine/internal/sessionstate"
)

const (
	testRetention = 168 * time.Hour

	expectedPointerFormat = "Written to %s lines %d-%d (%d chars). Read those lines before any other tool call — the read-gate hook enforces this."
	indexEnv              = "SKILL_CONTEXT_INDEX"
	mapEnv                = "SKILL_CONTEXT_MAP"
	testAgentId           = "agent-9"
	testSessionId         = "session-1"

	indexTest = `{
  "entries": [
    {
      "id": "ACTION-TEST-HOT-001",
      "kind": "action",
      "enforcement": "gate",
      "content": {
        "statement": "First statement.",
        "why": "because.",
        "applies": "always.",
        "bullets": ["An untagged bullet."],
        "examples": [{"lang": "go", "code": "// GOOD\nvar hookGroup int"}]
      }
    },
    {"id": "RULE-TEST-NAME-001", "kind": "rule", "enforcement": "review", "content": {"statement": "Second statement."}},
    {"id": "RULE-OTHER-001", "kind": "rule", "enforcement": "review", "content": {"statement": "Unselected statement."}}
  ]
}`
	reachEntries = `[
    {"id": "FACT-REPO-MINE-001", "kind": "fact", "reach": "smine", "content": {"statement": "Own fact."}},
    {"id": "FACT-REPO-GLOBAL-001", "kind": "fact", "reach": "global", "content": {"statement": "Global fact."}},
    {"id": "FACT-REPO-LISTED-001", "kind": "fact", "reach": "backend, smine", "content": {"statement": "Listed fact."}},
    {"id": "FACT-REPO-FOREIGN-001", "kind": "fact", "reach": "backend", "content": {"statement": "Foreign fact."}},
    {"id": "FACT-REPO-NONE-001", "kind": "fact", "reach": "none", "content": {"statement": "Disabled fact."}},
    {"id": "FACT-REPO-BARE-001", "kind": "fact", "content": {"statement": "Bare fact."}}
  ]`
	indexSelf   = `{"self": "smine", "entries": ` + reachEntries + `}`
	indexNoSelf = `{"entries": ` + reachEntries + `}`

	skillTest = `---
name: testskill
version: 1.0
acdsl-context: ACTION-TEST-HOT-001, RULE-TEST-*
---
# Test skill
`
	skillBare = `---
name: bare
version: 1.0
---
# Bare skill
`
	skillReach = `---
name: reachskill
version: 1.0
acdsl-context: FACT-*
---
# Reach skill
`
	skillSmart = `---
name: smartskill
version: 1.0
acdsl-context: FACT-*
---
# Smart skill
`
	skillNoVersion = `---
name: noversion
acdsl-context: ACTION-TEST-HOT-001, RULE-TEST-*
---
# No-version skill
`
	skillBig = `---
name: bigskill
version: 1.0
acdsl-context: RULE-BIG-*
---
# Big skill
`
	mapReach = `{"schemaVersion": "1.0", "skill": "reachskill", "lanes": {"facts": ["FACT-REPO-MINE-001"]}}`
	mapSmart = `{"schemaVersion": "1.0", "skill": "smartskill", "lanes": {
  "default": ["FACT-REPO-BARE-001"],
  "mine": ["FACT-REPO-MINE-001"],
  "listed": ["FACT-REPO-LISTED-001"]
}}`
	mapBigFacts = `{"schemaVersion": "1.0", "skill": "reachskill", "lanes": {"facts": ["FACT-BIG-*"]}}`
)

func TestRun(t *testing.T) {
	type testCase struct {
		_id                  string
		_expectedAbsent      []string
		_expectedCode        int
		_expectedLedger      []readledger.Record
		_expectedPresent     []string
		_expectedSessionDirs []string
		_expectedRunError    string
		_expectedSilent      bool
		_expectedStderr      string

		agentId    string
		args       []string
		claudeHome string
		env        map[string]string
		stdin      *strings.Reader
		stdout     *bytes.Buffer
	}

	tests := make([]*testCase, 0)
	testIds := []string{"ACTION-TEST-HOT-001", "RULE-TEST-NAME-001"}
	selfIds := []string{"FACT-REPO-BARE-001", "FACT-REPO-GLOBAL-001", "FACT-REPO-LISTED-001", "FACT-REPO-MINE-001"}
	allFactIds := []string{
		"FACT-REPO-BARE-001",
		"FACT-REPO-FOREIGN-001",
		"FACT-REPO-GLOBAL-001",
		"FACT-REPO-LISTED-001",
		"FACT-REPO-MINE-001",
		"FACT-REPO-NONE-001",
	}
	mineIds := []string{"FACT-REPO-MINE-001"}

	// slash-invocation-injects
	claudeHome := provideHome(t)
	repo := provideIndex(t, indexTest)
	tests = append(tests, &testCase{
		_id:                  "slash-invocation-injects",
		_expectedAbsent:      []string{"RULE-OTHER-001"},
		_expectedCode:        0,
		_expectedLedger:      []readledger.Record{provideContext(0, "", testIds, 0)},
		_expectedPresent:     []string{"===== SKILL CONTEXT /testskill v1.0 (injected by hook) =====", "- **ACTION-TEST-HOT-001**", "RULE-TEST-NAME-001", "First statement.", "* Applies: always."},
		_expectedSessionDirs: nil,
		_expectedSilent:      false,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/testskill some args", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// missing-version-stamps-question-mark
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexTest)
	provideFile(t, filepath.Join(claudeHome, "skills", "noversion", "SKILL.md"), skillNoVersion)
	tests = append(tests, &testCase{
		_id:                  "missing-version-stamps-question-mark",
		_expectedAbsent:      nil,
		_expectedCode:        0,
		_expectedLedger:      []readledger.Record{provideContext(0, "", testIds, 0)},
		_expectedPresent:     []string{"===== SKILL CONTEXT /noversion v? (injected by hook) =====", "- **ACTION-TEST-HOT-001**"},
		_expectedSessionDirs: nil,
		_expectedSilent:      false,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/noversion", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// typed-content-rendered-inline
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexTest)
	tests = append(tests, &testCase{
		_id:             "typed-content-rendered-inline",
		_expectedAbsent: []string{"Unselected statement."},
		_expectedCode:   0,
		_expectedLedger: []readledger.Record{provideContext(0, "", testIds, 0)},
		_expectedPresent: []string{
			"- **ACTION-TEST-HOT-001**\n- **RULE-TEST-NAME-001**\n",
			"**ACTION-TEST-HOT-001** `[gate]` — First statement.\n\n* Why: because.\n* Applies: always.\n* An untagged bullet.\n\n```go\n// GOOD\nvar hookGroup int\n```\n",
			"**RULE-TEST-NAME-001** `[review]` — Second statement.\n",
		},
		_expectedSessionDirs: nil,
		_expectedSilent:      false,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/testskill", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// plain-prompt-silent
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexTest)
	tests = append(tests, &testCase{
		_id:                  "plain-prompt-silent",
		_expectedAbsent:      nil,
		_expectedCode:        0,
		_expectedLedger:      nil,
		_expectedPresent:     nil,
		_expectedSessionDirs: nil,
		_expectedSilent:      true,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "just a question", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// undeclared-skill-silent
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexTest)
	tests = append(tests, &testCase{
		_id:                  "undeclared-skill-silent",
		_expectedAbsent:      nil,
		_expectedCode:        0,
		_expectedLedger:      nil,
		_expectedPresent:     nil,
		_expectedSessionDirs: nil,
		_expectedSilent:      true,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/bare go", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// unknown-skill-silent
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexTest)
	tests = append(tests, &testCase{
		_id:                  "unknown-skill-silent",
		_expectedAbsent:      nil,
		_expectedCode:        0,
		_expectedLedger:      nil,
		_expectedPresent:     nil,
		_expectedSessionDirs: nil,
		_expectedSilent:      true,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/nosuchskill", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// missing-index-silent
	claudeHome = provideHome(t)
	tests = append(tests, &testCase{
		_id:                  "missing-index-silent",
		_expectedAbsent:      nil,
		_expectedCode:        0,
		_expectedLedger:      nil,
		_expectedPresent:     nil,
		_expectedSessionDirs: nil,
		_expectedSilent:      true,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, t.TempDir(), "/testskill", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// missing-cwd-silent
	claudeHome = provideHome(t)
	tests = append(tests, &testCase{
		_id:                  "missing-cwd-silent",
		_expectedAbsent:      nil,
		_expectedCode:        0,
		_expectedLedger:      nil,
		_expectedPresent:     nil,
		_expectedSessionDirs: nil,
		_expectedSilent:      true,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, "", "/testskill", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// malformed-index-exits-1
	claudeHome = provideHome(t)
	repo = provideIndex(t, "{not json")
	tests = append(tests, &testCase{
		_id:                  "malformed-index-exits-1",
		_expectedAbsent:      nil,
		_expectedCode:        1,
		_expectedLedger:      nil,
		_expectedPresent:     nil,
		_expectedSessionDirs: nil,
		_expectedSilent:      true,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/testskill", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// posttooluse-wraps-json
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexTest)
	tests = append(tests, &testCase{
		_id:             "posttooluse-wraps-json",
		_expectedAbsent: nil,
		_expectedCode:   0,
		_expectedLedger: []readledger.Record{provideContext(0, "", testIds, 0)},
		_expectedPresent: []string{
			`{"hookSpecificOutput":{"additionalContext":"===== SKILL CONTEXT /testskill v1.0 (injected by hook) =====\n- **ACTION-TEST-HOT-001**`,
			`"hookEventName":"PostToolUse"}}`,
		},
		_expectedSessionDirs: nil,
		_expectedSilent:      false,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      provideSkillCall(t, "", "", repo, testSessionId, "testskill"),
		stdout:     &bytes.Buffer{},
	})

	// other-tool-silent
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexTest)
	readCall := &HookInput{Cwd: repo, HookEventName: "PostToolUse", SessionId: testSessionId, ToolName: "Read"}
	tests = append(tests, &testCase{
		_id:                  "other-tool-silent",
		_expectedAbsent:      nil,
		_expectedCode:        0,
		_expectedLedger:      nil,
		_expectedPresent:     nil,
		_expectedSessionDirs: nil,
		_expectedSilent:      true,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      provideEnvelope(t, readCall),
		stdout:     &bytes.Buffer{},
	})

	// missing-config-errors
	claudeHome = provideHome(t)
	require.NoError(t, os.Remove(hookconfig.Path(claudeHome)))
	repo = provideIndex(t, indexTest)
	tests = append(tests, &testCase{
		_id:                  "missing-config-errors",
		_expectedAbsent:      nil,
		_expectedCode:        1,
		_expectedLedger:      nil,
		_expectedPresent:     nil,
		_expectedRunError:    hookconfig.Path(claudeHome),
		_expectedSessionDirs: nil,
		_expectedSilent:      true,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/testskill", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// empty-context-dir-names-setting
	emptyDirConfig := provideHookConfig(t)
	emptyDirConfig.Context.ContextDir = ""
	claudeHome = provideHomeWith(t, emptyDirConfig)
	repo = provideIndex(t, indexTest)
	tests = append(tests, &testCase{
		_id:                  "empty-context-dir-names-setting",
		_expectedAbsent:      []string{"ACTION-TEST-HOT-001"},
		_expectedCode:        0,
		_expectedLedger:      nil,
		_expectedPresent:     []string{noContextDirFormat},
		_expectedSessionDirs: nil,
		_expectedSilent:      false,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/testskill", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// empty-context-dir-index-override-injects
	claudeHome = provideHomeWith(t, emptyDirConfig)
	repo = provideIndex(t, indexTest)
	tests = append(tests, &testCase{
		_id:                  "empty-context-dir-index-override-injects",
		_expectedAbsent:      []string{noContextDirFormat},
		_expectedCode:        0,
		_expectedLedger:      []readledger.Record{provideContext(0, "", testIds, 0)},
		_expectedPresent:     []string{"- **ACTION-TEST-HOT-001**"},
		_expectedSessionDirs: nil,
		_expectedSilent:      false,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(map[string]string{indexEnv: filepath.Join(repo, "ctx", contextdocs.ContextFileName)}),
		stdin:      providePrompt(t, repo, "/testskill", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// empty-context-dir-skill-without-declaration-silent
	claudeHome = provideHomeWith(t, emptyDirConfig)
	repo = provideIndex(t, indexTest)
	tests = append(tests, &testCase{
		_id:                  "empty-context-dir-skill-without-declaration-silent",
		_expectedAbsent:      nil,
		_expectedCode:        0,
		_expectedLedger:      nil,
		_expectedPresent:     nil,
		_expectedSessionDirs: nil,
		_expectedSilent:      true,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/bare", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// help-pattern-from-config
	helpConfig := provideHookConfig(t)
	helpConfig.Skillcontext.HelpPattern = `\?\?$`
	claudeHome = provideHomeWith(t, helpConfig)
	repo = provideIndex(t, indexTest)
	tests = append(tests, &testCase{
		_id:                  "help-pattern-from-config",
		_expectedAbsent:      nil,
		_expectedCode:        0,
		_expectedLedger:      nil,
		_expectedPresent:     nil,
		_expectedSessionDirs: nil,
		_expectedSilent:      true,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/testskill ??", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// no-context-double-dash-silent
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexTest)
	tests = append(tests, &testCase{
		_id:                  "no-context-double-dash-silent",
		_expectedAbsent:      nil,
		_expectedCode:        0,
		_expectedLedger:      nil,
		_expectedPresent:     nil,
		_expectedSessionDirs: nil,
		_expectedSilent:      true,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/testskill do the thing --no-context", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// no-context-single-dash-silent
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexTest)
	tests = append(tests, &testCase{
		_id:                  "no-context-single-dash-silent",
		_expectedAbsent:      nil,
		_expectedCode:        0,
		_expectedLedger:      nil,
		_expectedPresent:     nil,
		_expectedSessionDirs: nil,
		_expectedSilent:      true,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/testskill -no-context do the thing", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// no-context-em-dash-silent
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexTest)
	tests = append(tests, &testCase{
		_id:                  "no-context-em-dash-silent",
		_expectedAbsent:      nil,
		_expectedCode:        0,
		_expectedLedger:      nil,
		_expectedPresent:     nil,
		_expectedSessionDirs: nil,
		_expectedSilent:      true,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/testskill —no-context do the thing", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// no-context-en-dash-silent
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexTest)
	tests = append(tests, &testCase{
		_id:                  "no-context-en-dash-silent",
		_expectedAbsent:      nil,
		_expectedCode:        0,
		_expectedLedger:      nil,
		_expectedPresent:     nil,
		_expectedSessionDirs: nil,
		_expectedSilent:      true,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/testskill –no-context do the thing", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// no-context-posttooluse-silent
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexTest)
	tests = append(tests, &testCase{
		_id:                  "no-context-posttooluse-silent",
		_expectedAbsent:      nil,
		_expectedCode:        0,
		_expectedLedger:      nil,
		_expectedPresent:     nil,
		_expectedSessionDirs: nil,
		_expectedSilent:      true,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      provideSkillCall(t, "", "do the thing --no-context", repo, testSessionId, "testskill"),
		stdout:     &bytes.Buffer{},
	})

	// no-context-mention-still-injects
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexTest)
	tests = append(tests, &testCase{
		_id:                  "no-context-mention-still-injects",
		_expectedAbsent:      nil,
		_expectedCode:        0,
		_expectedLedger:      []readledger.Record{provideContext(0, "", testIds, 0)},
		_expectedPresent:     []string{"- **ACTION-TEST-HOT-001**"},
		_expectedSessionDirs: nil,
		_expectedSilent:      false,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/testskill add a --no-context, flag to x--no-contexty", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// help-token-double-dash-help-silent
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexNoSelf)
	tests = append(tests, &testCase{
		_id:                  "help-token-double-dash-help-silent",
		_expectedAbsent:      nil,
		_expectedCode:        0,
		_expectedLedger:      nil,
		_expectedPresent:     nil,
		_expectedSessionDirs: nil,
		_expectedSilent:      true,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/smartskill --help", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// help-token-single-dash-help-silent
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexNoSelf)
	tests = append(tests, &testCase{
		_id:                  "help-token-single-dash-help-silent",
		_expectedAbsent:      nil,
		_expectedCode:        0,
		_expectedLedger:      nil,
		_expectedPresent:     nil,
		_expectedSessionDirs: nil,
		_expectedSilent:      true,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/smartskill -help", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// help-token-single-dash-h-silent
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexNoSelf)
	tests = append(tests, &testCase{
		_id:                  "help-token-single-dash-h-silent",
		_expectedAbsent:      nil,
		_expectedCode:        0,
		_expectedLedger:      nil,
		_expectedPresent:     nil,
		_expectedSessionDirs: nil,
		_expectedSilent:      true,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/smartskill -h", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// help-token-double-dash-h-silent
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexNoSelf)
	tests = append(tests, &testCase{
		_id:                  "help-token-double-dash-h-silent",
		_expectedAbsent:      nil,
		_expectedCode:        0,
		_expectedLedger:      nil,
		_expectedPresent:     nil,
		_expectedSessionDirs: nil,
		_expectedSilent:      true,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/smartskill --h", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// help-token-em-dash-help-silent
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexNoSelf)
	tests = append(tests, &testCase{
		_id:                  "help-token-em-dash-help-silent",
		_expectedAbsent:      nil,
		_expectedCode:        0,
		_expectedLedger:      nil,
		_expectedPresent:     nil,
		_expectedSessionDirs: nil,
		_expectedSilent:      true,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/smartskill —help", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// help-token-en-dash-h-silent
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexNoSelf)
	tests = append(tests, &testCase{
		_id:                  "help-token-en-dash-h-silent",
		_expectedAbsent:      nil,
		_expectedCode:        0,
		_expectedLedger:      nil,
		_expectedPresent:     nil,
		_expectedSessionDirs: nil,
		_expectedSilent:      true,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/smartskill –h", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// index-override-env
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexTest)
	tests = append(tests, &testCase{
		_id:                  "index-override-env",
		_expectedAbsent:      nil,
		_expectedCode:        0,
		_expectedLedger:      []readledger.Record{provideContext(0, "", testIds, 0)},
		_expectedPresent:     []string{"- **ACTION-TEST-HOT-001**"},
		_expectedSessionDirs: nil,
		_expectedSilent:      false,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(map[string]string{indexEnv: filepath.Join(repo, "ctx", contextdocs.ContextFileName)}),
		stdin:      providePrompt(t, t.TempDir(), "/testskill", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// index-override-with-lane
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexNoSelf)
	tests = append(tests, &testCase{
		_id:                  "index-override-with-lane",
		_expectedAbsent:      []string{"FACT-REPO-GLOBAL-001"},
		_expectedCode:        0,
		_expectedLedger:      []readledger.Record{provideContext(0, "", mineIds, 0)},
		_expectedPresent:     []string{"FACT-REPO-MINE-001"},
		_expectedSessionDirs: nil,
		_expectedSilent:      false,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(map[string]string{indexEnv: filepath.Join(repo, "ctx", contextdocs.ContextFileName)}),
		stdin:      providePrompt(t, t.TempDir(), "/reachskill lane:facts", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// index-override-missing-silent
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexTest)
	tests = append(tests, &testCase{
		_id:                  "index-override-missing-silent",
		_expectedAbsent:      nil,
		_expectedCode:        0,
		_expectedLedger:      nil,
		_expectedPresent:     nil,
		_expectedSessionDirs: nil,
		_expectedSilent:      true,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(map[string]string{indexEnv: filepath.Join(t.TempDir(), "absent.json")}),
		stdin:      providePrompt(t, repo, "/testskill", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// self-present-filters-foreign-reach
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexSelf)
	tests = append(tests, &testCase{
		_id:                  "self-present-filters-foreign-reach",
		_expectedAbsent:      []string{"FACT-REPO-FOREIGN-001", "FACT-REPO-NONE-001"},
		_expectedCode:        0,
		_expectedLedger:      []readledger.Record{provideContext(0, "", selfIds, 0)},
		_expectedPresent:     selfIds,
		_expectedSessionDirs: nil,
		_expectedSilent:      false,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/reachskill", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// no-self-unfiltered
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexNoSelf)
	tests = append(tests, &testCase{
		_id:                  "no-self-unfiltered",
		_expectedAbsent:      nil,
		_expectedCode:        0,
		_expectedLedger:      []readledger.Record{provideContext(0, "", allFactIds, 0)},
		_expectedPresent:     allFactIds,
		_expectedSessionDirs: nil,
		_expectedSilent:      false,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/reachskill", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// lane-token-slices-map
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexNoSelf)
	tests = append(tests, &testCase{
		_id:                  "lane-token-slices-map",
		_expectedAbsent:      []string{"FACT-REPO-GLOBAL-001"},
		_expectedCode:        0,
		_expectedLedger:      []readledger.Record{provideContext(0, "", mineIds, 0)},
		_expectedPresent:     []string{"===== SKILL CONTEXT /reachskill lane:facts v1.0 (injected by hook) =====", "- **FACT-REPO-MINE-001**"},
		_expectedSessionDirs: nil,
		_expectedSilent:      false,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/reachskill lane:facts", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// lane-slice-reach-filtered
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexSelf)
	tests = append(tests, &testCase{
		_id:                  "lane-slice-reach-filtered",
		_expectedAbsent:      []string{"FACT-REPO-GLOBAL-001"},
		_expectedCode:        0,
		_expectedLedger:      []readledger.Record{provideContext(0, "", mineIds, 0)},
		_expectedPresent:     []string{"- **FACT-REPO-MINE-001**"},
		_expectedSessionDirs: nil,
		_expectedSilent:      false,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/reachskill lane:facts", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// unmapped-lane-full-fallback
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexNoSelf)
	tests = append(tests, &testCase{
		_id:                  "unmapped-lane-full-fallback",
		_expectedAbsent:      []string{"lane:zzz"},
		_expectedCode:        0,
		_expectedLedger:      []readledger.Record{provideContext(0, "", allFactIds, 0)},
		_expectedPresent:     append([]string{"===== SKILL CONTEXT /reachskill v1.0 (injected by hook) ====="}, allFactIds...),
		_expectedSessionDirs: nil,
		_expectedSilent:      false,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/reachskill lane:zzz", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// malformed-map-full-fallback
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexNoSelf)
	malformedMap := filepath.Join(t.TempDir(), "context-map.json")
	provideFile(t, malformedMap, "{not json")
	tests = append(tests, &testCase{
		_id:                  "malformed-map-full-fallback",
		_expectedAbsent:      []string{"lane:facts"},
		_expectedCode:        0,
		_expectedLedger:      []readledger.Record{provideContext(0, "", allFactIds, 0)},
		_expectedPresent:     append([]string{"===== SKILL CONTEXT /reachskill v1.0 (injected by hook) ====="}, allFactIds...),
		_expectedSessionDirs: nil,
		_expectedSilent:      false,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(map[string]string{mapEnv: malformedMap}),
		stdin:      providePrompt(t, repo, "/reachskill lane:facts", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// default-slice-without-token
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexNoSelf)
	tests = append(tests, &testCase{
		_id:                  "default-slice-without-token",
		_expectedAbsent:      []string{"FACT-REPO-GLOBAL-001"},
		_expectedCode:        0,
		_expectedLedger:      []readledger.Record{provideContext(0, "", []string{"FACT-REPO-BARE-001"}, 0)},
		_expectedPresent:     []string{"===== SKILL CONTEXT /smartskill topic:default v1.0 (injected by hook) =====", "- **FACT-REPO-BARE-001**"},
		_expectedSessionDirs: nil,
		_expectedSilent:      false,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/smartskill talk", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// topic-token-overrides-default
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexNoSelf)
	tests = append(tests, &testCase{
		_id:                  "topic-token-overrides-default",
		_expectedAbsent:      []string{"FACT-REPO-BARE-001"},
		_expectedCode:        0,
		_expectedLedger:      []readledger.Record{provideContext(0, "", mineIds, 0)},
		_expectedPresent:     []string{"===== SKILL CONTEXT /smartskill topic:mine v1.0 (injected by hook) =====", "- **FACT-REPO-MINE-001**"},
		_expectedSessionDirs: nil,
		_expectedSilent:      false,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/smartskill topic:mine talk", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// multi-topic-union
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexNoSelf)
	tests = append(tests, &testCase{
		_id:             "multi-topic-union",
		_expectedAbsent: []string{"FACT-REPO-GLOBAL-001"},
		_expectedCode:   0,
		_expectedLedger: []readledger.Record{provideContext(0, "", []string{"FACT-REPO-LISTED-001", "FACT-REPO-MINE-001"}, 0)},
		_expectedPresent: []string{
			"===== SKILL CONTEXT /smartskill topic:listed+mine v1.0 (injected by hook) =====",
			"- **FACT-REPO-LISTED-001**",
			"- **FACT-REPO-MINE-001**",
		},
		_expectedSessionDirs: nil,
		_expectedSilent:      false,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/smartskill topic:mine topic:listed talk", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// lane-token-on-smartskill-map
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexNoSelf)
	tests = append(tests, &testCase{
		_id:                  "lane-token-on-smartskill-map",
		_expectedAbsent:      []string{"FACT-REPO-BARE-001"},
		_expectedCode:        0,
		_expectedLedger:      []readledger.Record{provideContext(0, "", mineIds, 0)},
		_expectedPresent:     []string{"===== SKILL CONTEXT /smartskill lane:mine v1.0 (injected by hook) =====", "- **FACT-REPO-MINE-001**"},
		_expectedSessionDirs: nil,
		_expectedSilent:      false,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/smartskill lane:mine talk", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// repeat-invocation-ids-only
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexTest)
	earlier := provideContext(0, "", testIds, 0)
	provideLedger(t, "", claudeHome, earlier)
	tests = append(tests, &testCase{
		_id:             "repeat-invocation-ids-only",
		_expectedAbsent: []string{"- **", "First statement.", "Second statement."},
		_expectedCode:   0,
		_expectedLedger: []readledger.Record{earlier},
		_expectedPresent: []string{
			"===== SKILL CONTEXT /testskill v1.0 (injected by hook) =====\nInjected earlier this session (2): ACTION-TEST-HOT-001, RULE-TEST-NAME-001\n",
		},
		_expectedSessionDirs: nil,
		_expectedSilent:      false,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/testskill again", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// overlap-injects-delta-only
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexTest)
	earlier = provideContext(0, "", []string{"ACTION-TEST-HOT-001"}, 0)
	provideLedger(t, "", claudeHome, earlier)
	tests = append(tests, &testCase{
		_id:             "overlap-injects-delta-only",
		_expectedAbsent: []string{"- **ACTION-TEST-HOT-001**", "First statement."},
		_expectedCode:   0,
		_expectedLedger: []readledger.Record{earlier, provideContext(0, "", []string{"RULE-TEST-NAME-001"}, 0)},
		_expectedPresent: []string{
			"- **RULE-TEST-NAME-001**\nInjected earlier this session (1): ACTION-TEST-HOT-001\n",
			"**RULE-TEST-NAME-001** `[review]` — Second statement.",
		},
		_expectedSessionDirs: nil,
		_expectedSilent:      false,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/testskill", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// subagent-uses-own-ledger
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexTest)
	provideLedger(t, "", claudeHome, provideContext(0, "", testIds, 0))
	tests = append(tests, &testCase{
		_id:                  "subagent-uses-own-ledger",
		_expectedAbsent:      []string{"Injected earlier this session"},
		_expectedCode:        0,
		_expectedLedger:      []readledger.Record{provideContext(0, "", testIds, 0)},
		_expectedPresent:     []string{"- **ACTION-TEST-HOT-001**", "First statement.", "Second statement."},
		_expectedSessionDirs: nil,
		_expectedSilent:      false,

		agentId:    testAgentId,
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      provideSkillCall(t, testAgentId, "", repo, testSessionId, "testskill"),
		stdout:     &bytes.Buffer{},
	})

	// prune-old-session-dirs
	claudeHome = provideHome(t)
	repo = provideIndex(t, provideBigIndex(t, "RULE-BIG", "rule"))
	provideSessionDirs(t, claudeHome)
	bigFile := filepath.Join(contextrequired.Dir(claudeHome, testSessionId), "bigskill--main.md")
	tests = append(tests, &testCase{
		_id:                  "prune-old-session-dirs",
		_expectedAbsent:      []string{"Big statement 1."},
		_expectedCode:        0,
		_expectedLedger:      []readledger.Record{provideContext(279, bigFile, provideBigIds("RULE-BIG", 1, 40), 1)},
		_expectedPresent:     []string{"Written to " + bigFile + " lines 1-279"},
		_expectedSessionDirs: []string{"fresh", testSessionId},
		_expectedSilent:      false,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/bigskill", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// prune-without-file
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexTest)
	provideSessionDirs(t, claudeHome)
	tests = append(tests, &testCase{
		_id:                  "prune-without-file",
		_expectedAbsent:      []string{"Written to "},
		_expectedCode:        0,
		_expectedLedger:      []readledger.Record{provideContext(0, "", testIds, 0)},
		_expectedPresent:     []string{"First statement."},
		_expectedSessionDirs: []string{"fresh"},
		_expectedSilent:      false,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/testskill", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// retention-from-config
	longRetentionConfig := provideHookConfig(t)
	longRetentionConfig.Readgate.LedgerRetentionMs = hookconfig.DurationMs(time.Since(time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)))
	claudeHome = provideHomeWith(t, longRetentionConfig)
	repo = provideIndex(t, indexTest)
	provideSessionDirs(t, claudeHome)
	tests = append(tests, &testCase{
		_id:                  "retention-from-config",
		_expectedAbsent:      []string{"Written to "},
		_expectedCode:        0,
		_expectedLedger:      []readledger.Record{provideContext(0, "", testIds, 0)},
		_expectedPresent:     []string{"First statement."},
		_expectedSessionDirs: []string{"fresh", "old"},
		_expectedSilent:      false,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/testskill", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// switch-off-no-output
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexTest)
	provideSwitchesOff(t, claudeHome, sessionstate.InjectSkill)
	tests = append(tests, &testCase{
		_id:                  "switch-off-no-output",
		_expectedAbsent:      nil,
		_expectedCode:        0,
		_expectedLedger:      nil,
		_expectedPresent:     nil,
		_expectedSessionDirs: nil,
		_expectedSilent:      true,
		_expectedStderr:      "",

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/testskill some args", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// switch-off-skill-call-no-output
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexTest)
	provideSwitchesOff(t, claudeHome, sessionstate.InjectSkill)
	tests = append(tests, &testCase{
		_id:                  "switch-off-skill-call-no-output",
		_expectedAbsent:      nil,
		_expectedCode:        0,
		_expectedLedger:      nil,
		_expectedPresent:     nil,
		_expectedSessionDirs: nil,
		_expectedSilent:      true,
		_expectedStderr:      "",

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      provideSkillCall(t, "", "", repo, testSessionId, "testskill"),
		stdout:     &bytes.Buffer{},
	})

	// record-garbage-injects-and-logs
	claudeHome = provideHome(t)
	repo = provideIndex(t, indexTest)
	garbage := provideGarbageRecord(t, claudeHome)
	tests = append(tests, &testCase{
		_id:                  "record-garbage-injects-and-logs",
		_expectedAbsent:      []string{"RULE-OTHER-001"},
		_expectedCode:        0,
		_expectedLedger:      []readledger.Record{provideContext(0, "", testIds, 0)},
		_expectedPresent:     []string{"===== SKILL CONTEXT /testskill v1.0 (injected by hook) =====", "First statement."},
		_expectedSessionDirs: nil,
		_expectedSilent:      false,
		_expectedStderr:      fmt.Sprintf("skillcontext: isOff: IsOff: Load: Failed to decode %s: unexpected end of JSON input\n", garbage),

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		stdin:      providePrompt(t, repo, "/testskill some args", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			for key, value := range test.env {
				t.Setenv(key, value)
			}
			stderr := provideStderr(t)
			runStderr := &bytes.Buffer{}
			exitCode := run(test.args, runStderr, test.stdin, test.stdout)

			assert.Equal(t, test._expectedCode, exitCode)
			assert.Contains(t, runStderr.String(), test._expectedRunError)
			assert.Equal(t, test._expectedStderr, readStderr(t, stderr))
			assert.Equal(t, test._expectedSilent, test.stdout.Len() == 0)
			assert.Empty(t, missing(test.stdout.String(), test._expectedPresent))
			assert.Empty(t, present(test.stdout.String(), test._expectedAbsent))
			assert.Equal(t, test._expectedLedger, readLedger(t, test.agentId, test.claudeHome))
			assert.Equal(t, test._expectedSessionDirs, readSessionDirs(t, test.claudeHome))
		})
	}
}

func TestRun_InjectionFile(t *testing.T) {
	type testCase struct {
		_id                  string
		_expectedCode        int
		_expectedFile        string
		_expectedLedger      []readledger.Record
		_expectedSessionDirs []string
		_expectedStdout      string

		agentId    string
		args       []string
		claudeHome string
		env        map[string]string
		file       string
		stdin      *strings.Reader
		stdout     *bytes.Buffer
	}

	tests := make([]*testCase, 0)
	bigIds := provideBigIds("RULE-BIG", 1, 40)
	bigBodies := provideBigBodies("RULE-BIG", 1, 40)

	// over-cap-writes-file-with-pointer
	claudeHome := provideHome(t)
	repo := provideIndex(t, provideBigIndex(t, "RULE-BIG", "rule"))
	file := filepath.Join(contextrequired.Dir(claudeHome, testSessionId), "bigskill--main.md")
	head := provideHeader(bigIds, "", nil, "bigskill")
	tests = append(tests, &testCase{
		_id:                  "over-cap-writes-file-with-pointer",
		_expectedCode:        0,
		_expectedFile:        bigBodies,
		_expectedLedger:      []readledger.Record{provideContext(279, file, bigIds, 1)},
		_expectedSessionDirs: []string{testSessionId},
		_expectedStdout:      providePointer(bigBodies, file, head, []int{279}, 1) + "\n",

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		file:       file,
		stdin:      providePrompt(t, repo, "/bigskill", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// pointer-chunked-over-budget
	claudeHome = provideHome(t)
	longStatement := "Long statement %d " + strings.Repeat("y", 980)
	repo = provideIndex(t, provideIndexOf(t, "RULE-BIG", "rule", longStatement))
	file = filepath.Join(contextrequired.Dir(claudeHome, testSessionId), "bigskill--main.md")
	longBodies := provideBodiesOf("RULE-BIG", 1, 40, longStatement)
	head = provideHeader(bigIds, "", nil, "bigskill")
	tests = append(tests, &testCase{
		_id:                  "pointer-chunked-over-budget",
		_expectedCode:        0,
		_expectedFile:        longBodies,
		_expectedLedger:      []readledger.Record{provideContext(279, file, bigIds, 1)},
		_expectedSessionDirs: []string{testSessionId},
		_expectedStdout:      providePointer(longBodies, file, head, []int{266, 13}, 1) + "\n",

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		file:       file,
		stdin:      providePrompt(t, repo, "/bigskill", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// over-cap-without-session-id-inline
	claudeHome = provideHome(t)
	repo = provideIndex(t, provideBigIndex(t, "RULE-BIG", "rule"))
	tests = append(tests, &testCase{
		_id:                  "over-cap-without-session-id-inline",
		_expectedCode:        0,
		_expectedFile:        "",
		_expectedLedger:      nil,
		_expectedSessionDirs: nil,
		_expectedStdout:      provideHeader(bigIds, "", nil, "bigskill") + "\n\n" + bigBodies,

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		file:       filepath.Join(contextrequired.Root(claudeHome), "bigskill--main.md"),
		stdin:      providePrompt(t, repo, "/bigskill", ""),
		stdout:     &bytes.Buffer{},
	})

	// second-delta-appends
	claudeHome = provideHome(t)
	repo = provideIndex(t, provideBigIndex(t, "RULE-BIG", "rule"))
	file = filepath.Join(contextrequired.Dir(claudeHome, testSessionId), "bigskill--main.md")
	seed := "seed 1\nseed 2\nseed 3\n"
	provideFile(t, file, seed)
	earlier := provideContext(3, file, provideBigIds("RULE-BIG", 1, 20), 1)
	provideLedger(t, "", claudeHome, earlier)
	deltaBodies := provideBigBodies("RULE-BIG", 21, 40)
	deltaEnd := 3 + strings.Count(deltaBodies, "\n")
	head = provideHeader(provideBigIds("RULE-BIG", 21, 40), "", provideBigIds("RULE-BIG", 1, 20), "bigskill")
	tests = append(tests, &testCase{
		_id:                  "second-delta-appends",
		_expectedCode:        0,
		_expectedFile:        seed + deltaBodies,
		_expectedLedger:      []readledger.Record{earlier, provideContext(deltaEnd, file, provideBigIds("RULE-BIG", 21, 40), 4)},
		_expectedSessionDirs: []string{testSessionId},
		_expectedStdout:      providePointer(deltaBodies, file, head, []int{139}, 4) + "\n",

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		file:       file,
		stdin:      providePrompt(t, repo, "/bigskill", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// subagent-file-carries-agent
	claudeHome = provideHome(t)
	repo = provideIndex(t, provideBigIndex(t, "RULE-BIG", "rule"))
	file = filepath.Join(contextrequired.Dir(claudeHome, testSessionId), "bigskill--agent-9.md")
	head = provideHeader(bigIds, "", nil, "bigskill")
	tests = append(tests, &testCase{
		_id:                  "subagent-file-carries-agent",
		_expectedCode:        0,
		_expectedFile:        bigBodies,
		_expectedLedger:      []readledger.Record{provideContext(279, file, bigIds, 1)},
		_expectedSessionDirs: []string{testSessionId},
		_expectedStdout:      provideOutput(t, providePointer(bigBodies, file, head, []int{279}, 1)),

		agentId:    testAgentId,
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(nil),
		file:       file,
		stdin:      provideSkillCall(t, testAgentId, "", repo, testSessionId, "bigskill"),
		stdout:     &bytes.Buffer{},
	})

	// lane-file-carries-slice-and-agent
	claudeHome = provideHome(t)
	repo = provideIndex(t, provideBigIndex(t, "FACT-BIG", "fact"))
	bigFactsMap := filepath.Join(t.TempDir(), "context-map.json")
	provideFile(t, bigFactsMap, mapBigFacts)
	factIds := provideBigIds("FACT-BIG", 1, 40)
	factBodies := provideBigBodies("FACT-BIG", 1, 40)
	file = filepath.Join(contextrequired.Dir(claudeHome, testSessionId), "reachskill--lane-facts--main.md")
	head = provideHeader(factIds, "lane:facts", nil, "reachskill")
	tests = append(tests, &testCase{
		_id:                  "lane-file-carries-slice-and-agent",
		_expectedCode:        0,
		_expectedFile:        factBodies,
		_expectedLedger:      []readledger.Record{provideContext(279, file, factIds, 1)},
		_expectedSessionDirs: []string{testSessionId},
		_expectedStdout:      providePointer(factBodies, file, head, []int{279}, 1) + "\n",

		agentId:    "",
		args:       provideArgs(claudeHome),
		claudeHome: claudeHome,
		env:        provideEnv(map[string]string{mapEnv: bigFactsMap}),
		file:       file,
		stdin:      providePrompt(t, repo, "/reachskill lane:facts", testSessionId),
		stdout:     &bytes.Buffer{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			for key, value := range test.env {
				t.Setenv(key, value)
			}
			exitCode := run(test.args, io.Discard, test.stdin, test.stdout)

			assert.Equal(t, test._expectedCode, exitCode)
			assert.Equal(t, test._expectedStdout, test.stdout.String())
			assert.Equal(t, test._expectedFile, readInjectionFile(t, test.file))
			assert.Equal(t, test._expectedLedger, readLedger(t, test.agentId, test.claudeHome))
			assert.Equal(t, test._expectedSessionDirs, readSessionDirs(t, test.claudeHome))
		})
	}
}

// missing returns the substrings the output lacks.
func missing(output string, substrings []string) []string {
	absent := make([]string, 0)
	for _, substring := range substrings {
		if !strings.Contains(output, substring) {
			absent = append(absent, substring)
		}
	}
	return absent
}

// present returns the substrings the output carries.
func present(output string, substrings []string) []string {
	found := make([]string, 0)
	for _, substring := range substrings {
		if strings.Contains(output, substring) {
			found = append(found, substring)
		}
	}
	return found
}

func provideArgs(claudeHome string) []string {
	return []string{"-claude-home", claudeHome, "-skills", filepath.Join(claudeHome, "skills")}
}

// provideBigBodies is the injection file content of the big entries first to
// last: each entry's headline, a blank, four bullets, one blank line between
// entries, one trailing newline.
func provideBigBodies(prefix string, first int, last int) string {
	return provideBodiesOf(prefix, first, last, "Big statement %d.")
}

// provideBodiesOf is provideBigBodies with the statement formatted with the entry number.
func provideBodiesOf(prefix string, first int, last int, statement string) string {
	bullets := strings.Repeat("* bullet "+strings.Repeat("x", 90)+"\n", 4)
	entries := make([]string, 0)
	for number := first; number <= last; number++ {
		headline := fmt.Sprintf("**%s-%03d** `[review]` — %s\n\n", prefix, number, fmt.Sprintf(statement, number))
		entries = append(entries, headline+bullets)
	}
	return strings.Join(entries, "\n")
}

func provideBigIds(prefix string, first int, last int) []string {
	ids := make([]string, 0)
	for number := first; number <= last; number++ {
		ids = append(ids, fmt.Sprintf("%s-%03d", prefix, number))
	}
	return ids
}

// provideBigIndex is the over-cap index: 40 entries, four 90-x bullets each.
func provideBigIndex(t *testing.T, prefix string, kind string) string {
	return provideIndexOf(t, prefix, kind, "Big statement %d.")
}

// provideIndexOf is an index of 40 entries, four 90-x bullets each, the statement formatted with the entry number.
func provideIndexOf(t *testing.T, prefix string, kind string, statement string) string {
	bullet := "bullet " + strings.Repeat("x", 90)
	index := &contextdocs.ContextFile{}
	for number := 1; number <= 40; number++ {
		entry := contextdocs.RuleEntry{
			Id: fmt.Sprintf("%s-%03d", prefix, number),
			Content: contextdocs.RuleContent{
				Bullets:   []string{bullet, bullet, bullet, bullet},
				Statement: fmt.Sprintf(statement, number),
			},
			Enforcement: "review",
			Kind:        kind,
		}
		index.Entries = append(index.Entries, entry)
	}

	data, err := json.Marshal(index)
	require.NoError(t, err)
	return string(data)
}

func provideContext(end int, file string, ids []string, start int) readledger.Record {
	return readledger.Record{End: end, File: file, Ids: ids, Kind: readledger.KindContext, Start: start}
}

// provideEnv clears every override key the hook reads unless the case sets
// it, so the ambient environment never leaks into a case.
func provideEnv(overrides map[string]string) map[string]string {
	env := map[string]string{indexEnv: "", mapEnv: ""}
	maps.Copy(env, overrides)
	return env
}

func provideEnvelope(t *testing.T, input *HookInput) *strings.Reader {
	data, err := json.Marshal(input)
	require.NoError(t, err)
	return strings.NewReader(string(data))
}

func provideFile(t *testing.T, path string, content string) {
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	require.NoError(t, err)

	err = os.WriteFile(path, []byte(content), 0o644)
	require.NoError(t, err)
}

// provideGarbageRecord writes an undecodable switch record for the test session and returns its path.
func provideGarbageRecord(t *testing.T, claudeHome string) string {
	path := sessionstate.File(sessionstate.StateDir(claudeHome), testSessionId)
	provideFile(t, path, "{")
	return path
}

// provideHeader is the injection header: the banner, one id line per body,
// the repeat line naming the ids injected earlier.
func provideHeader(delta []string, lane string, repeat []string, skill string) string {
	banner := "===== SKILL CONTEXT /" + skill
	if lane != "" {
		banner += " " + lane
	}

	lines := []string{banner + " v1.0 (injected by hook) ====="}
	for _, id := range delta {
		lines = append(lines, "- **"+id+"**")
	}
	if len(repeat) > 0 {
		lines = append(lines, fmt.Sprintf("Injected earlier this session (%d): %s", len(repeat), strings.Join(repeat, ", ")))
	}
	return strings.Join(lines, "\n")
}

// provideHome is a claude home with the five test skills, their maps, and
// the default hooks config with the context dir set to ctx.
func provideHome(t *testing.T) string {
	return provideHomeWith(t, provideHookConfig(t))
}

// provideHomeWith is provideHome with the given hooks config.
func provideHomeWith(t *testing.T, hookConfig *hookconfig.Config) string {
	claudeHome := t.TempDir()
	skills := filepath.Join(claudeHome, "skills")
	writeHookConfig(t, claudeHome, hookConfig)
	provideFile(t, filepath.Join(skills, "testskill", "SKILL.md"), skillTest)
	provideFile(t, filepath.Join(skills, "bare", "SKILL.md"), skillBare)
	provideFile(t, filepath.Join(skills, "reachskill", "SKILL.md"), skillReach)
	provideFile(t, filepath.Join(skills, "reachskill", "context-map.json"), mapReach)
	provideFile(t, filepath.Join(skills, "smartskill", "SKILL.md"), skillSmart)
	provideFile(t, filepath.Join(skills, "smartskill", "context-map.json"), mapSmart)
	provideFile(t, filepath.Join(skills, "bigskill", "SKILL.md"), skillBig)
	return claudeHome
}

// provideHookConfig is the catalog's default hooks config with the context dir set to ctx.
func provideHookConfig(t *testing.T) *hookconfig.Config {
	hookConfig := &hookconfig.Config{}
	hookComponents := []components.ID{components.Context, components.Gate, components.Worktrees, components.SessionHelpers}
	for _, id := range hookComponents {
		rendered, err := install.RenderDefaults(id, components.SinkHooksConfig)
		require.NoError(t, err)

		err = json.Unmarshal(rendered, hookConfig)
		require.NoError(t, err)
	}

	hookConfig.Context.ContextDir = "ctx"
	return hookConfig
}

// provideIndex is a repo whose ctx/context.json holds the index.
func provideIndex(t *testing.T, index string) string {
	repo := t.TempDir()
	provideFile(t, filepath.Join(repo, "ctx", contextdocs.ContextFileName), index)
	return repo
}

// provideLedger appends the records to the agent's ledger ("" is the root).
func provideLedger(t *testing.T, agentId string, claudeHome string, records ...readledger.Record) {
	ledger := readledger.New(agentId, claudeHome, testRetention, testSessionId)
	for _, record := range records {
		err := ledger.Append(&record)
		require.NoError(t, err)
	}
}

func provideOutput(t *testing.T, addition string) string {
	output := &HookOutput{
		HookSpecificOutput: HookSpecificOutput{
			AdditionalContext: addition,
			HookEventName:     "PostToolUse",
		},
	}
	data, err := json.Marshal(output)
	require.NoError(t, err)
	return string(data) + "\n"
}

// providePointer is the injection of bodies appended to the file at start:
// the header, the pointer to the lines they occupy, one Read line per chunk limit.
func providePointer(bodies string, file string, head string, limits []int, start int) string {
	trimmed := strings.TrimSuffix(bodies, "\n")
	end := start + strings.Count(trimmed, "\n")
	lines := []string{head, fmt.Sprintf(expectedPointerFormat, file, start, end, utf8.RuneCountInString(trimmed))}
	offset := start
	for _, limit := range limits {
		lines = append(lines, fmt.Sprintf("Read %s offset %d limit %d", file, offset, limit))
		offset += limit
	}
	return strings.Join(lines, "\n")
}

func providePrompt(t *testing.T, cwd string, prompt string, sessionId string) *strings.Reader {
	input := &HookInput{
		Cwd:           cwd,
		HookEventName: "UserPromptSubmit",
		Prompt:        prompt,
		SessionId:     sessionId,
	}
	return provideEnvelope(t, input)
}

// provideSessionDirs creates an old (2020) and a fresh session dir under the
// context-required root.
func provideSessionDirs(t *testing.T, claudeHome string) {
	old := contextrequired.Dir(claudeHome, "old")
	err := os.MkdirAll(old, 0o755)
	require.NoError(t, err)

	err = os.MkdirAll(contextrequired.Dir(claudeHome, "fresh"), 0o755)
	require.NoError(t, err)

	longAgo := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	err = os.Chtimes(old, longAgo, longAgo)
	require.NoError(t, err)
}

func provideSkillCall(t *testing.T, agentId string, args string, cwd string, sessionId string, skill string) *strings.Reader {
	input := &HookInput{
		AgentId:       agentId,
		Cwd:           cwd,
		HookEventName: "PostToolUse",
		SessionId:     sessionId,
		ToolInput:     ToolInput{Args: args, Skill: skill},
		ToolName:      "Skill",
	}
	return provideEnvelope(t, input)
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

// readInjectionFile returns the file's content, "" when it does not exist.
func readInjectionFile(t *testing.T, path string) string {
	data, err := os.ReadFile(path)
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

// readSessionDirs returns the names under the context-required root, nil
// when it does not exist.
func readSessionDirs(t *testing.T, claudeHome string) []string {
	entries, err := os.ReadDir(contextrequired.Root(claudeHome))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
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
	provideFile(t, hookconfig.Path(claudeHome), string(encoded))
}

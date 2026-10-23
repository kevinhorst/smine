package skills

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const helpFixtureTable = `---
name: qq
description: One quick question, one short answer. Trigger on /qq or "quick question". Args — topic: context slice(s) to load; no-context: suppress injection.
author: Kevin Horst
version: 1.0.0
argument-hint: "topic:harness how many subagents run"
---

# Quick Question

## Routing

**Purpose:** answer a single question fast
and cheaply.

**Disambiguation:**

| For | Use |
| :--- | :--- |
| a subject to talk through | /discuss |
| a verdict on code | /code-verdict |

**Preconditions:**
- one question in the invocation

**Pipeline:** standalone.

## Args

| Arg | Form | Default | Effect |
| :--- | :--- | :--- | :--- |
| ` + "`topic`" + ` | ` + "`topic:<name>`" + `, repeatable | default slice | context slice(s) injected — [Topic use](#skill-qq-context-001) |
| ` + "`no-context`" + ` | ` + "`--no-context`" + ` | absent | hook flag: no injection |

## Model
`

const helpFixtureBullets = `---
name: fmt
description: Reformat a thing. Trigger on /fmt. Args — plan|concept: route selector; caveman: compress prose.
version: 2.0
---

# Fmt

## Routing

**Purpose:** reformat.

**Disambiguation:**

| For | Use |
| :--- | :--- |
| other | /other |

**Preconditions:**
- none

**Pipeline:** standalone.

## Args

- ` + "`plan|concept`" + `: route selector
- ` + "`caveman`" + `: compress prose
`

const helpFixtureNoArgs = `---
name: close
description: Remove the worktree. Trigger on /close.
version: 1.0
---

# Close

## Routing

**Purpose:** cleanup.

**Pipeline:** standalone.
`

const helpFixtureMap = `{"schemaVersion": "1.0", "skill": "qq", "lanes": {"repo": ["FACT-REPO-*"], "default": ["RULE-SKILL-016"], "cache": ["FACT-CLAUDE-CACHE-*"]}}`

const helpFixtureTitle = `{
  "schema": 2,
  "skill": "fdesign",
  "version": "1.0.0",
  "states": {
    "DESIGN-READY": {
      "entry": "SKILL-FDESIGN-GATE-001",
      "meaning": "design gate presented — approve or revise the design"
    },
    "CODE-REVIEWABLE": {
      "entry": "SKILL-FDESIGN-CODE-005",
      "meaning": "code gate presented — approve the Changes or request a change"
    }
  }
}`

// helpOutline is the part of Help TestParseHelp compares; zero without a Help.
type helpOutline struct {
	Args                []HelpArg
	DisambiguationCount int
	Purpose             string
	TitleStates         []HelpTitleState
	Topics              []string
}

func newHelpOutline(help *Help) *helpOutline {
	outline := &helpOutline{}
	if help == nil {
		return outline
	}

	outline.Args = help.Args
	outline.DisambiguationCount = len(help.Disambiguation)
	outline.Purpose = help.Purpose
	outline.TitleStates = help.TitleStates
	for _, topic := range help.Topics {
		outline.Topics = append(outline.Topics, topic.Name)
	}
	return outline
}

func TestParseHelp(t *testing.T) {
	type testCase struct {
		_id                    string
		_expectedArgs          []HelpArg
		_expectedDisambigCount int
		_expectedPurpose       string
		_expectedTitleStates   []HelpTitleState
		_expectedTopics        []string
		_shouldPass            bool

		contextMap    string
		skillMd       string
		titleDocument string
	}

	tests := make([]*testCase, 0)

	// table-args-with-map
	tests = append(tests, &testCase{
		_id: "table-args-with-map",
		_expectedArgs: []HelpArg{
			{Cells: []string{"topic:<name>, repeatable", "default slice", "context slice(s) injected"}, Name: "topic"},
			{Cells: []string{"--no-context", "absent", "hook flag: no injection"}, Name: "no-context"},
		},
		_expectedDisambigCount: 2,
		_expectedPurpose:       "answer a single question fast and cheaply.",
		_expectedTitleStates:   nil,
		_expectedTopics:        []string{"default", "cache", "repo"},
		_shouldPass:            true,

		contextMap:    helpFixtureMap,
		skillMd:       helpFixtureTable,
		titleDocument: "",
	})

	// bullet-args
	tests = append(tests, &testCase{
		_id: "bullet-args",
		_expectedArgs: []HelpArg{
			{Cells: []string{"route selector"}, Name: "plan|concept"},
			{Cells: []string{"compress prose"}, Name: "caveman"},
		},
		_expectedDisambigCount: 1,
		_expectedPurpose:       "reformat.",
		_expectedTitleStates:   nil,
		_expectedTopics:        nil,
		_shouldPass:            true,

		contextMap:    "",
		skillMd:       helpFixtureBullets,
		titleDocument: "",
	})

	// no-args
	tests = append(tests, &testCase{
		_id:                    "no-args",
		_expectedArgs:          []HelpArg{},
		_expectedDisambigCount: 0,
		_expectedPurpose:       "cleanup.",
		_expectedTitleStates:   nil,
		_expectedTopics:        nil,
		_shouldPass:            true,

		contextMap:    "",
		skillMd:       helpFixtureNoArgs,
		titleDocument: "",
	})

	// title-states
	tests = append(tests, &testCase{
		_id:                    "title-states",
		_expectedArgs:          []HelpArg{},
		_expectedDisambigCount: 0,
		_expectedPurpose:       "cleanup.",
		_expectedTitleStates: []HelpTitleState{
			{Entry: "SKILL-FDESIGN-CODE-005", Meaning: "code gate presented — approve the Changes or request a change", State: "CODE-REVIEWABLE"},
			{Entry: "SKILL-FDESIGN-GATE-001", Meaning: "design gate presented — approve or revise the design", State: "DESIGN-READY"},
		},
		_expectedTopics: nil,
		_shouldPass:     true,

		contextMap:    "",
		skillMd:       helpFixtureNoArgs,
		titleDocument: helpFixtureTitle,
	})

	// invalid-title-document
	tests = append(tests, &testCase{
		_id:                    "invalid-title-document",
		_expectedArgs:          nil,
		_expectedDisambigCount: 0,
		_expectedPurpose:       "",
		_expectedTitleStates:   nil,
		_expectedTopics:        nil,
		_shouldPass:            false,

		contextMap:    "",
		skillMd:       helpFixtureNoArgs,
		titleDocument: `{"schema": 1, "skill": "fdesign", "version": "1.0.0", "states": {}}`,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			help, err := ParseHelp(test.contextMap, "", test.skillMd, test.titleDocument)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			outline := newHelpOutline(help)
			assert.Equal(t, test._expectedArgs, outline.Args)
			assert.Equal(t, test._expectedDisambigCount, outline.DisambiguationCount)
			assert.Equal(t, test._expectedPurpose, outline.Purpose)
			assert.Equal(t, test._expectedTitleStates, outline.TitleStates)
			assert.Equal(t, test._expectedTopics, outline.Topics)
		})
	}
}

const helpFixtureWrapped = `---
name: peek
description: Show a session. Trigger on /peek.
version: 1.0
---

# Peek

## Routing

**Purpose:** show a session.

**Disambiguation:**

| For | Use |
| :--- | :--- |
| other | /other |

**Preconditions:**
- peek-mcp available and
  connected
- a session exists

**Pipeline:** standalone; /fdesign → **peek** → /fimplement
— wrapped note

| Input | Tool | Notes |
|-------|------|-------|
| n | turns | recent |
`

func TestParseHelpRoutingWrapped(t *testing.T) {
	help, err := ParseHelp("", "", helpFixtureWrapped, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"peek-mcp available and connected", "a session exists"}, help.Preconditions, "wrapped-precondition-joined")
	assert.Equal(t, "standalone; /fdesign → **peek** → /fimplement — wrapped note", help.Pipeline, "wrapped-pipeline-joined")
	assert.Equal(t, [][]string{{"other", "/other"}}, help.Disambiguation, "table-after-pipeline-ignored")
	assert.Equal(t, "show a session.", help.Purpose)
}

func TestParseHelpInvalidMap(t *testing.T) {
	_, err := ParseHelp("{not json", "", helpFixtureTable, "")
	assert.Error(t, err)
}

func TestRenderHelp(t *testing.T) {
	help, err := ParseHelp(helpFixtureMap, "", helpFixtureTable, "")
	require.NoError(t, err)

	expected := `/qq — One quick question, one short answer.

Frontmatter
  name: qq
  description: One quick question, one short answer. Trigger on /qq or "quick question". Args — topic: context slice(s) to load; no-context: suppress injection.
  author: Kevin Horst
  version: 1.0.0
  argument-hint: "topic:harness how many subagents run"

Routing
  Purpose: answer a single question fast and cheaply.
  Use instead:
    a subject to talk through  /discuss
    a verdict on code          /code-verdict
  Preconditions:
    - one question in the invocation
  Pipeline: standalone.

Args
  topic       topic:<name>, repeatable  default slice  context slice(s) injected
  no-context  --no-context              absent         hook flag: no injection
  Example: /qq topic:harness how many subagents run

Topics (context-map.json)
  default  RULE-SKILL-016
  cache    FACT-CLAUDE-CACHE-*
  repo     FACT-REPO-*
`
	assert.Equal(t, expected, RenderHelp("qq", help))
}

func TestRenderHelpNoArgs(t *testing.T) {
	help, err := ParseHelp("", "", helpFixtureNoArgs, "")
	require.NoError(t, err)
	rendered := RenderHelp("close", help)
	assert.Contains(t, rendered, "Args\n  none\n")
	assert.NotContains(t, rendered, "Topics")
	assert.NotContains(t, rendered, "Tests")
	assert.NotContains(t, rendered, "Title states")
}

func TestRenderHelpTests(t *testing.T) {
	helpTest := "Smoke: skipped — acts on a live fleet\nEval: /skillroutine-eval close\n"
	help, err := ParseHelp("", helpTest, helpFixtureNoArgs, "")
	require.NoError(t, err)

	expected := "\nTests\n  Smoke: skipped — acts on a live fleet\n  Eval: /skillroutine-eval close\n"
	assert.True(t, strings.HasSuffix(RenderHelp("close", help), expected))
}

func TestRenderHelpTitleStates(t *testing.T) {
	helpTest := "Smoke: make skill-smoke SKILL=qq TOKEN=<label>"
	help, err := ParseHelp("", helpTest, helpFixtureTable, helpFixtureTitle)
	require.NoError(t, err)

	expected := "  Example: /qq topic:harness how many subagents run\n" +
		"\nTitle states (title.json)\n" +
		"  CODE-REVIEWABLE  code gate presented — approve the Changes or request a change  SKILL-FDESIGN-CODE-005\n" +
		"  DESIGN-READY     design gate presented — approve or revise the design           SKILL-FDESIGN-GATE-001\n" +
		"\nTests\n  Smoke: make skill-smoke SKILL=qq TOKEN=<label>\n"
	assert.Contains(t, RenderHelp("qq", help), expected)
}

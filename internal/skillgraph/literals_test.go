package skillgraph

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// literalHint is the advice checkLiterals appends for the demo fixture.
const literalHint = ": call it through a $SKILL_DEMO_<NAME> variable"

// writeSkillFile writes content to rel below dir, creating its parent dirs.
func writeSkillFile(t *testing.T, dir, rel, content string) string {
	t.Helper()
	path := filepath.Join(dir, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestCheckLiterals(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		dir   string
		skill *Skill
	}

	tests := make([]*testCase, 0)

	// statement-literal
	skill := provideCompleteSkill()
	flowSection(skill).Blocks[1].Entry.Statement = "RUN `smine-rules lanes`."
	tests = append(tests, &testCase{
		_id:       "statement-literal",
		_expected: []string{"literal smine-rules" + literalHint},

		dir:   t.TempDir(),
		skill: skill,
	})

	// allowed-tools-literal
	skill = provideCompleteSkill()
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: fieldAllowedTools, Value: " Bash(smine-agentwt *)"})
	tests = append(tests, &testCase{
		_id:       "allowed-tools-literal",
		_expected: []string{"literal smine-agentwt" + literalHint},

		dir:   t.TempDir(),
		skill: skill,
	})

	// enforce-pattern-literal
	skill = provideCompleteSkill()
	skill.Enforce = &Enforce{Anchors: map[string]*Anchor{
		"status": {Input: []InputMatch{{Field: "command", Pattern: "^smine-heartbeat status"}}, Tool: "^Bash$"},
	}}
	tests = append(tests, &testCase{
		_id:       "enforce-pattern-literal",
		_expected: []string{"literal smine-heartbeat" + literalHint},

		dir:   t.TempDir(),
		skill: skill,
	})

	// routing-and-model-literal
	skill = provideCompleteSkill()
	skill.Routing().Purpose = "drives toolpolicy."
	skill.Model().Notes = []string{"calls smine-routine cell."}
	tests = append(tests, &testCase{
		_id:       "routing-and-model-literal",
		_expected: []string{"literal toolpolicy" + literalHint, "literal smine-routine" + literalHint},

		dir:   t.TempDir(),
		skill: skill,
	})

	// workflow-file-literal
	dir := t.TempDir()
	workflow := writeSkillFile(t, dir, "workflows/x.js", "const a = 1;\nrun('smine-rules lanes');\n")
	tests = append(tests, &testCase{
		_id:       "workflow-file-literal",
		_expected: []string{workflow + ":2: literal smine-rules"},

		dir:   dir,
		skill: provideCompleteSkill(),
	})

	// nested-workflow-file-literal
	dir = t.TempDir()
	common := writeSkillFile(t, dir, "workflows/common/common.src.js", "run('toolpolicy check');\n")
	tests = append(tests, &testCase{
		_id:       "nested-workflow-file-literal",
		_expected: []string{common + ":1: literal toolpolicy"},

		dir:   dir,
		skill: provideCompleteSkill(),
	})

	// agent-file-literal
	dir = t.TempDir()
	agent := writeSkillFile(t, dir, "agents/x.md", "RUN `smine-heartbeat status`.\n")
	tests = append(tests, &testCase{
		_id:       "agent-file-literal",
		_expected: []string{agent + ":1: literal smine-heartbeat"},

		dir:   dir,
		skill: provideCompleteSkill(),
	})

	// smine-routines-skill-name-passes
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[1].Entry.Statement = "INVOKE `Skill(smine-routines)`."
	tests = append(tests, &testCase{
		_id:       "smine-routines-skill-name-passes",
		_expected: nil,

		dir:   t.TempDir(),
		skill: skill,
	})

	// variable-reference-passes
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[1].Entry.Statement = "RUN `$SKILL_X_RULES lanes`."
	tests = append(tests, &testCase{
		_id:       "variable-reference-passes",
		_expected: nil,

		dir:   t.TempDir(),
		skill: skill,
	})

	// sibling-required-reference-passes
	skill = provideCompleteSkill()
	skill.Env = map[string]*Variable{"RULES": {Interface: "lanes@1", Required: true}}
	dir = t.TempDir()
	writeSkillFile(t, dir, "workflows/x.js", "run('$SKILL_DEMO_RULES lanes');\n")
	tests = append(tests, &testCase{
		_id:       "sibling-required-reference-passes",
		_expected: nil,

		dir:   dir,
		skill: skill,
	})

	// sibling-optional-reference-refused
	skill = provideCompleteSkill()
	skill.Env = map[string]*Variable{"BUILD": {Interface: "build@1"}}
	dir = t.TempDir()
	optional := writeSkillFile(t, dir, "workflows/x.js", "run('$SKILL_DEMO_BUILD');\n")
	tests = append(tests, &testCase{
		_id:       "sibling-optional-reference-refused",
		_expected: []string{optional + ": SKILL_DEMO_BUILD is optional — sibling files reference required variables only"},

		dir:   dir,
		skill: skill,
	})

	// repo-scoped-in-sibling-refused
	skill = provideCompleteSkill()
	bindEnv(skill, map[string]*Variable{"CHECK": {Interface: "check@1", Required: true}})
	dir = t.TempDir()
	repoScoped := writeSkillFile(t, dir, "agents/x.md", "RUN `$SKILL_DEMO_CHECK`.\n")
	tests = append(tests, &testCase{
		_id:       "repo-scoped-in-sibling-refused",
		_expected: []string{repoScoped + ": SKILL_DEMO_CHECK is repo-scoped — sibling files are filled at deploy and reference install-scoped required variables only"},

		dir:   dir,
		skill: skill,
	})

	// sibling-undeclared-reference-refused
	dir = t.TempDir()
	undeclared := writeSkillFile(t, dir, "agents/x.md", "RUN `$SKILL_DEMO_RULES lanes`.\n")
	tests = append(tests, &testCase{
		_id:       "sibling-undeclared-reference-refused",
		_expected: []string{undeclared + ": SKILL_DEMO_RULES is referenced but no env entry declares it"},

		dir:   dir,
		skill: provideCompleteSkill(),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, checkLiterals(test.dir, test.skill))
		})
	}
}

func TestFillVariables(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		text   string
		values map[string]string
	}

	values := map[string]string{"SKILL_DEMO_AGENTWT": "~/go/bin/smine-agentwt", "SKILL_DEMO_BUILD": "go build ./..."}
	tests := make([]*testCase, 0)

	// set-reference-replaced
	tests = append(tests, &testCase{
		_id:       "set-reference-replaced",
		_expected: "RUN `~/go/bin/smine-agentwt status`.",

		text:   "RUN `$SKILL_DEMO_AGENTWT status`.",
		values: values,
	})

	// unset-reference-kept
	tests = append(tests, &testCase{
		_id:       "unset-reference-kept",
		_expected: "RUN `$SKILL_DEMO_RULES lanes`.",

		text:   "RUN `$SKILL_DEMO_RULES lanes`.",
		values: values,
	})

	// empty-value-kept
	tests = append(tests, &testCase{
		_id:       "empty-value-kept",
		_expected: "RUN `$SKILL_DEMO_RULES lanes`.",

		text:   "RUN `$SKILL_DEMO_RULES lanes`.",
		values: map[string]string{"SKILL_DEMO_RULES": ""},
	})

	// two-references-one-line
	tests = append(tests, &testCase{
		_id:       "two-references-one-line",
		_expected: "RUN `go build ./...`, then `~/go/bin/smine-agentwt status`.",

		text:   "RUN `$SKILL_DEMO_BUILD`, then `$SKILL_DEMO_AGENTWT status`.",
		values: values,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, string(FillVariables([]byte(test.text), test.values)))
		})
	}
}

func TestFillPattern(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		pattern string
		values  map[string]string
	}

	tests := make([]*testCase, 0)

	// path-value
	tests = append(tests, &testCase{
		_id:       "path-value",
		_expected: "^~/go/bin/smine-agentwt status",

		pattern: "^$SKILL_DEMO_AGENTWT status",
		values:  map[string]string{"SKILL_DEMO_AGENTWT": "~/go/bin/smine-agentwt"},
	})

	// regex-syntax-quoted
	tests = append(tests, &testCase{
		_id:       "regex-syntax-quoted",
		_expected: `^go run \./cmd/acdsl check`,

		pattern: "^$SKILL_DEMO_ACDSL check",
		values:  map[string]string{"SKILL_DEMO_ACDSL": "go run ./cmd/acdsl"},
	})

	// unset-reference-kept
	tests = append(tests, &testCase{
		_id:       "unset-reference-kept",
		_expected: "^$SKILL_DEMO_RULES lanes",

		pattern: "^$SKILL_DEMO_RULES lanes",
		values:  map[string]string{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, FillPattern(test.pattern, test.values))
		})
	}
}

func TestVariableDisabled(t *testing.T) {
	type testCase struct {
		_id       string
		_expected map[string]bool

		skill  *Skill
		values map[string]string
	}

	tests := make([]*testCase, 0)

	// statement-reference-disables
	skill := provideCompleteSkill()
	flowSection(skill).Blocks[1].Entry.Statement = "RUN `$SKILL_DEMO_BUILD`."
	tests = append(tests, &testCase{
		_id:       "statement-reference-disables",
		_expected: map[string]bool{"SKILL-DEMO-FLOW-002": true},

		skill:  skill,
		values: map[string]string{},
	})

	// payload-reference-disables
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[3].Entry.Payload.Body = "RUN $SKILL_DEMO_BUILD\n"
	tests = append(tests, &testCase{
		_id:       "payload-reference-disables",
		_expected: map[string]bool{"SKILL-DEMO-TPL-001": true},

		skill:  skill,
		values: map[string]string{},
	})

	// item-reference-keeps-entry
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[1].Entry.Items[0].Text = "build: `$SKILL_DEMO_BUILD`."
	tests = append(tests, &testCase{
		_id:       "item-reference-keeps-entry",
		_expected: map[string]bool{},

		skill:  skill,
		values: map[string]string{},
	})

	// set-reference-keeps-entry
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[1].Entry.Statement = "RUN `$SKILL_DEMO_BUILD`."
	tests = append(tests, &testCase{
		_id:       "set-reference-keeps-entry",
		_expected: map[string]bool{},

		skill:  skill,
		values: map[string]string{"SKILL_DEMO_BUILD": "go build ./..."},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, variableDisabled(nil, test.skill, test.values))
		})
	}
}

func TestItemView(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []Item

		entry  *Entry
		values map[string]string
	}

	kept := Item{Label: LabelParam, Marker: "*", Text: "one fact."}
	textReference := Item{Label: LabelParam, Marker: "*", Text: "build: `$SKILL_DEMO_BUILD`."}
	fenceReference := Item{Fence: &Fence{Body: "$SKILL_DEMO_BUILD\n", Marker: "```"}}
	tests := make([]*testCase, 0)

	// text-reference-dropped
	tests = append(tests, &testCase{
		_id:       "text-reference-dropped",
		_expected: []Item{kept},

		entry:  &Entry{Id: "SKILL-DEMO-FLOW-002", Items: []Item{kept, textReference}},
		values: map[string]string{},
	})

	// fence-reference-dropped
	tests = append(tests, &testCase{
		_id:       "fence-reference-dropped",
		_expected: []Item{kept},

		entry:  &Entry{Id: "SKILL-DEMO-FLOW-002", Items: []Item{fenceReference, kept}},
		values: map[string]string{},
	})

	// set-reference-kept
	tests = append(tests, &testCase{
		_id:       "set-reference-kept",
		_expected: []Item{kept, textReference},

		entry:  &Entry{Id: "SKILL-DEMO-FLOW-002", Items: []Item{kept, textReference}},
		values: map[string]string{"SKILL_DEMO_BUILD": "go build ./..."},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, itemView(nil, test.entry, test.values).Items)
		})
	}
}

func TestItemView_Flags(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *Fields

		entry  *Entry
		values map[string]string
	}

	flagged := func() *Entry {
		return &Entry{Fields: &Fields{Flags: "--build '$SKILL_DEMO_BUILD'", Timeout: 600000}, Id: "SKILL-DEMO-FLOW-002"}
	}
	tests := make([]*testCase, 0)

	// unset-flags-dropped
	tests = append(tests, &testCase{
		_id:       "unset-flags-dropped",
		_expected: &Fields{Timeout: 600000},

		entry:  flagged(),
		values: map[string]string{},
	})

	// set-flags-kept
	tests = append(tests, &testCase{
		_id:       "set-flags-kept",
		_expected: &Fields{Flags: "--build '$SKILL_DEMO_BUILD'", Timeout: 600000},

		entry:  flagged(),
		values: map[string]string{"SKILL_DEMO_BUILD": "go build ./..."},
	})

	// no-fields-untouched
	tests = append(tests, &testCase{
		_id:       "no-fields-untouched",
		_expected: nil,

		entry:  &Entry{Id: "SKILL-DEMO-FLOW-002"},
		values: map[string]string{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, itemView(nil, test.entry, test.values).Fields)
		})
	}
}

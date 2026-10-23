package skillgraph

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderMarkdown_Canonical(t *testing.T) {
	rendered, err := renderMarkdown(nil, false, nil, provideCompleteSkill())
	require.NoError(t, err)
	assert.Equal(t, fixtureMarkdown, string(rendered))
}

func TestRenderMarkdown_Deployed(t *testing.T) {
	skill := provideCompleteSkill()
	flowSection(skill).Kind = SectionKindPhase
	want := strings.NewReplacer(
		"`{pred: not panic}`\\\n", "",
		"`{post: first_done}`\\\n", "",
		"`{pred: first_done}`\\\n", "",
	).Replace(fixtureMarkdown)

	rendered, err := renderMarkdown(nil, true, nil, skill)
	require.NoError(t, err)
	assert.Equal(t, want, string(rendered))
}

func TestRenderMarkdown(t *testing.T) {
	type testCase struct {
		_id         string
		_contains   string
		_shouldPass bool

		deployed bool
		disabled map[string]bool
		skill    *Skill
	}

	tests := make([]*testCase, 0)

	// deployed-disabled
	skill := provideCompleteSkill()
	flowSection(skill).Blocks = append(flowSection(skill).Blocks, Block{Heading: "Later"}, Block{Entry: &Entry{Id: "SKILL-DEMO-FLOW-004", Role: RoleStep, Statement: "RUN later.", Title: "Later step"}})
	skill.Sections = append(skill.Sections, &Section{Title: "Notes", Blocks: []Block{{Prose: "Only prose."}}})
	disabled := map[string]bool{"SKILL-DEMO-FLOW-004": true, "SKILL-DEMO-NIGHT-001": true}
	tests = append(tests, &testCase{_id: "deployed-disabled", _contains: "## Notes\n\nOnly prose.\n", _shouldPass: true, deployed: true, disabled: disabled, skill: skill})

	// args-bullets
	skill = provideCompleteSkill()
	skill.Sections[1].Blocks[0].Args = &ArgsBlock{Bullets: true, Rows: []Arg{{Name: "caveman", Effect: "compress prose"}}}
	tests = append(tests, &testCase{_id: "args-bullets", _contains: "## Args\n\n- `caveman`: compress prose\n", _shouldPass: true, skill: skill})

	// entry-without-statement
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[2].Entry.Statement = ""
	tests = append(tests, &testCase{_id: "entry-without-statement", _contains: "**SKILL-DEMO-FLOW-003** `[step]` **Second step**\n\n**SKILL-DEMO-TPL-001**", _shouldPass: true, skill: skill})

	// item-fence-and-text
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[1].Entry.Items[0].Fence = &Fence{Marker: "```"}
	tests = append(tests, &testCase{_id: "item-fence-and-text", _contains: "either a fence or a bullet", skill: skill})

	// block-without-kind
	skill = provideCompleteSkill()
	skill.Sections[0].Blocks = append(skill.Sections[0].Blocks, Block{})
	tests = append(tests, &testCase{_id: "block-without-kind", _contains: "sets exactly one", skill: skill})

	// block-with-two-kinds
	skill = provideCompleteSkill()
	skill.Sections[0].Blocks[0].Heading = "Extra"
	tests = append(tests, &testCase{_id: "block-with-two-kinds", _contains: "sets exactly one", skill: skill})

	// arg-name-with-backtick
	skill = provideCompleteSkill()
	skill.Sections[1].Blocks[0].Args.Rows[0].Name = "mo`de"
	tests = append(tests, &testCase{_id: "arg-name-with-backtick", _contains: "argLines: Name", skill: skill})

	// statement-starts-like-bullet
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[2].Entry.Statement = "* RUN the second step."
	tests = append(tests, &testCase{_id: "statement-starts-like-bullet", _contains: "starts like a bullet", skill: skill})

	// bullet-continuation-starts-like-bullet
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[1].Entry.Items[0].Text = strings.Repeat("w ", 60) + "- dangling tail"
	tests = append(tests, &testCase{_id: "bullet-continuation-starts-like-bullet", _contains: "starts like a bullet", skill: skill})

	// fence-body-closes-early
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[3].Entry.Payload.Body = "```\n"
	tests = append(tests, &testCase{_id: "fence-body-closes-early", _contains: "closes the ``` fence early", skill: skill})

	// payload-on-step-role
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[2].Entry.Payload = &Payload{Marker: "```"}
	tests = append(tests, &testCase{_id: "payload-on-step-role", _contains: "exactly the payload role", skill: skill})

	// prose-with-trailing-newline
	skill = provideCompleteSkill()
	skill.Sections[0].Blocks[0] = Block{Prose: "**Purpose:** demo.\n"}
	tests = append(tests, &testCase{_id: "prose-with-trailing-newline", _contains: "starts or ends with a blank line", skill: skill})

	// prose-owns-heading
	skill = provideCompleteSkill()
	skill.Sections[0].Blocks[0] = Block{Prose: "### Hidden"}
	tests = append(tests, &testCase{_id: "prose-owns-heading", _contains: "is a heading or entry headline", skill: skill})

	// purpose-wrapped
	skill = provideCompleteSkill()
	skill.Routing().Purpose = strings.Repeat("word ", 30) + "end."
	tests = append(tests, &testCase{_id: "purpose-wrapped", _contains: "**Purpose:** " + strings.Repeat("word ", 22) + "word\n" + strings.Repeat("word ", 7) + "end.\n", _shouldPass: true, skill: skill})

	// precondition-wrapped
	skill = provideCompleteSkill()
	skill.Routing().Preconditions = []string{strings.Repeat("word ", 30) + "end."}
	tests = append(tests, &testCase{_id: "precondition-wrapped", _contains: "- " + strings.Repeat("word ", 24) + "word\n  " + strings.Repeat("word ", 5) + "end.\n", _shouldPass: true, skill: skill})

	// pipeline-standalone-only
	skill = provideCompleteSkill()
	skill.Routing().Pipeline = Pipeline{Standalone: true}
	tests = append(tests, &testCase{_id: "pipeline-standalone-only", _contains: "**Pipeline:** standalone\n", _shouldPass: true, skill: skill})

	// pipeline-chain
	skill = provideCompleteSkill()
	skill.Routing().Pipeline = Pipeline{Edges: []Edge{
		{Kind: EdgeKindFrom, Skill: "concept"},
		{Kind: EdgeKindFrom, Optional: true, Skill: "idea"},
		{Kind: EdgeKindTo, Route: "change", Skill: "fdesign"},
		{Kind: EdgeKindSpawnedBy, Skill: "delegate"},
		{Kind: EdgeKindSpawns, Skill: "worker"},
	}}
	tests = append(tests, &testCase{_id: "pipeline-chain", _contains: "**Pipeline:** /concept, /idea *(optional)* → **demo** → /fdesign change; spawned by /delegate; spawns /worker\n", _shouldPass: true, skill: skill})

	// pipeline-note-only
	skill = provideCompleteSkill()
	skill.Routing().Pipeline = Pipeline{Note: "eval-only"}
	tests = append(tests, &testCase{_id: "pipeline-note-only", _contains: "**Pipeline:** eval-only\n", _shouldPass: true, skill: skill})

	// model-one-unviable
	skill = provideCompleteSkill()
	skill.Model().Unviable = []string{"sonnet as dispatcher"}
	tests = append(tests, &testCase{_id: "model-one-unviable", _contains: "- Tested unviable: sonnet as dispatcher\n", _shouldPass: true, skill: skill})

	// model-two-unviable
	skill = provideCompleteSkill()
	skill.Model().Unviable = []string{"sonnet as dispatcher", "haiku anywhere"}
	tests = append(tests, &testCase{_id: "model-two-unviable", _contains: "- Tested unviable:\n  - sonnet as dispatcher\n  - haiku anywhere\n", _shouldPass: true, skill: skill})

	// model-minimal
	skill = provideCompleteSkill()
	*skill.Model() = Model{Effort: "low", Reason: "cheap", Tier: "small"}
	tests = append(tests, &testCase{_id: "model-minimal", _contains: "## Model\n\n- Suggested: small / low\n- Reason: cheap\n- Tested unviable: — (none yet)\n", _shouldPass: true, skill: skill})

	// route-with-both-anchors
	skill = provideCompleteSkill()
	routeSection(skill).Replaces = "FLOW-003"
	tests = append(tests, &testCase{_id: "route-with-both-anchors", _contains: "not both", skill: skill})

	// deployed-route-replaces-line
	skill = provideCompleteSkill()
	routeSection(skill).Between = ""
	routeSection(skill).Replaces = "FLOW-003"
	tests = append(tests, &testCase{
		_id:         "deployed-route-replaces-line",
		_contains:   "## Nightly\n`{section: route, pred: mode = nightly, replaces: FLOW-003}`\n\n**SKILL-DEMO-NIGHT-001** `[step]` **Night step**\n",
		_shouldPass: true,

		deployed: true,
		skill:    skill,
	})

	// deployed-route-between-line
	skill = provideCompleteSkill()
	routeSection(skill).Between = "FLOW-001..FLOW-003"
	tests = append(tests, &testCase{
		_id:         "deployed-route-between-line",
		_contains:   "## Nightly\n`{section: route, pred: mode = nightly, between: FLOW-001..FLOW-003}`\n\n**SKILL-DEMO-NIGHT-001** `[step]` **Night step**\n",
		_shouldPass: true,

		deployed: true,
		skill:    skill,
	})

	// deployed-context-section-line
	skill = provideCompleteSkill()
	flowSection(skill).Kind = SectionKindContext
	tests = append(tests, &testCase{
		_id:         "deployed-context-section-line",
		_contains:   "## Flow\n\n**SKILL-DEMO-FLOW-001** `[rule]` **Stay calm**\n\nAgent MUST stay calm.\n",
		_shouldPass: true,

		deployed: true,
		skill:    skill,
	})

	// deployed-payload-fence-verbatim
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[3].Entry.Payload = &Payload{Body: "## TLDR\n\n  - keep `indent`\n```go\nx := 1\n```\n", Info: "markdown", Marker: "````"}
	tests = append(tests, &testCase{
		_id:         "deployed-payload-fence-verbatim",
		_contains:   "**SKILL-DEMO-TPL-001** `[payload]` **Template**\n\nThe template.\n\n````markdown\n## TLDR\n\n  - keep `indent`\n```go\nx := 1\n```\n````\n\n## Nightly\n",
		_shouldPass: true,

		deployed: true,
		skill:    skill,
	})

	// deployed-disable-glob-keeps-payload
	skill = provideCompleteSkill()
	disabled, err := resolveDisable(skill, []string{"SKILL-DEMO-FLOW-*"})
	require.NoError(t, err)
	tests = append(tests, &testCase{
		_id:         "deployed-disable-glob-keeps-payload",
		_contains:   "## Flow\n\n**SKILL-DEMO-TPL-001** `[payload]` **Template**\n\nThe template.\n\n```markdown\n## TLDR\n```\n\n## Nightly\n",
		_shouldPass: true,

		deployed: true,
		disabled: disabled,
		skill:    skill,
	})

	// deployed-args-bullets
	skill = provideCompleteSkill()
	skill.Sections[1].Blocks[0].Args = &ArgsBlock{Bullets: true, Rows: []Arg{{Name: "caveman", Effect: "compress prose"}}}
	tests = append(tests, &testCase{
		_id:         "deployed-args-bullets",
		_contains:   "## Args\n\n- `caveman`: compress prose\n\n## Flow\n",
		_shouldPass: true,

		deployed: true,
		skill:    skill,
	})

	// deployed-model-minimal
	skill = provideCompleteSkill()
	*skill.Model() = Model{Effort: "low", Reason: "cheap", Tier: "small"}
	tests = append(tests, &testCase{
		_id:         "deployed-model-minimal",
		_contains:   "RUN at night.\n\n## Model\n\n- Suggested: small / low\n- Reason: cheap\n- Tested unviable: — (none yet)\n",
		_shouldPass: true,

		deployed: true,
		skill:    skill,
	})

	// deployed-frontmatter-order
	skill = provideCompleteSkill()
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: "argument-hint", Value: " mode:a"})
	skill.Frontmatter[1], skill.Frontmatter[4] = skill.Frontmatter[4], skill.Frontmatter[1]
	tests = append(tests, &testCase{
		_id:         "deployed-frontmatter-order",
		_contains:   "---\nname: demo\nargument-hint: mode:a\nversion: 1.0\nformat: 2\ndescription: demo fixture. Trigger on nothing.\n---\n\n# Demo\n",
		_shouldPass: true,

		deployed: true,
		skill:    skill,
	})

	// deployed-fenced-item
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[1].Entry.Items = append(flowSection(skill).Blocks[1].Entry.Items, Item{Fence: &Fence{Body: "make audit\n", Info: "bash", Marker: "```"}})
	tests = append(tests, &testCase{
		_id:         "deployed-fenced-item",
		_contains:   "**SKILL-DEMO-FLOW-002** `[step]` **First step**\n\nRUN the first step.\n* param: one fact.\n\n```bash\nmake audit\n```\n\n**SKILL-DEMO-FLOW-003** `[step]` **Second step**\n",
		_shouldPass: true,

		deployed: true,
		skill:    skill,
	})

	// deployed-heading-and-prose-blocks
	skill = provideCompleteSkill()
	flowSection(skill).Blocks = append(flowSection(skill).Blocks, Block{Heading: "Wrap-up"}, Block{Prose: "Closing words.\nSecond line."})
	tests = append(tests, &testCase{
		_id:         "deployed-heading-and-prose-blocks",
		_contains:   "## TLDR\n```\n\n### Wrap-up\n\nClosing words.\nSecond line.\n\n## Nightly\n",
		_shouldPass: true,

		deployed: true,
		skill:    skill,
	})

	// runtime-lines
	skill = provideCompleteSkill()
	skill.Docker = DockerAlways
	skill.EnableDelegation = DelegationNever
	skill.EnforcePermissions = []string{PermissionAcceptEdits, PermissionPlanMode}
	tests = append(tests, &testCase{
		_id:         "runtime-lines",
		_contains:   "format: 2\ndocker: always\nenable-delegation: never\nenforce-permissions: acceptEdits, planMode\n---\n",
		_shouldPass: true,

		deployed: true,
		skill:    skill,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			rendered, err := renderMarkdown(nil, test.deployed, test.disabled, test.skill)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Contains(t, string(rendered)+errorText(err), test._contains)
		})
	}
}

func TestRenderMarkdown_Fields(t *testing.T) {
	type testCase struct {
		_id       string
		_contains string

		deferred *deferral
		skill    *Skill
	}

	tests := make([]*testCase, 0)

	// every-key-rendered
	every := &Fields{
		Flags:    "--mode <mode>",
		Forwards: []Forward{{Arg: "exclude", Flag: "--exclude", Stage: "exclude"}},
		Mints:    &Mints{Branch: "demo/{slug}", File: "plans/{slug}.md", Id: map[string]string{"run": "{date}-{n}"}},
		Reads:    []string{"repo"},
		Ref:      []string{"FLOW-003"},
		Returns:  []string{"verdict"},
		Schema:   "schemas/stage.json",
		Script:   "workflows/stage.js",
		Timeout:  60000,
	}
	skill := withEntry(provideFieldsSkill(), "SKILL-DEMO-FLOW-002", func(entry *Entry) { entry.Fields = every })
	tests = append(tests, &testCase{
		_id: "every-key-rendered",
		_contains: "**SKILL-DEMO-FLOW-002** `[step]` **First step**\n" +
			"\n" +
			"RUN the first step.\n" +
			"* script: workflows/stage.js\n" +
			"* schema: schemas/stage.json\n" +
			"* ref: [Second step · FLOW-003](#skill-demo-flow-003)\n" +
			"* flags: --mode <mode>\n" +
			"* forwards: `exclude` → `--exclude` → stage `exclude`\n" +
			"* reads: repo\n" +
			"* returns: verdict\n" +
			"* mints: branch `demo/{slug}`\n" +
			"* mints: file `plans/{slug}.md`\n" +
			"* mints: `run` = `{date}-{n}`\n" +
			"* timeout: 60000 ms\n" +
			"\n" +
			"`{pred: first_done}`\\\n",

		deferred: nil,
		skill:    skill,
	})

	// deferred-flags-wrap-in-placeholder
	skill = withEntry(provideFieldsSkill(), "SKILL-DEMO-FLOW-002", func(entry *Entry) {
		entry.Fields = &Fields{Flags: "--test `$SKILL_DEMO_TEST`", Reads: []string{"repo"}}
	})
	bindEnv(skill, map[string]*Variable{"TEST": {Interface: "test@1"}})
	block := provideBlock(testGuard, `"\052 flags: --test \140" ENVIRON["SKILL_DEMO_TEST"] "\140"`)
	tests = append(tests, &testCase{
		_id:       "deferred-flags-wrap-in-placeholder",
		_contains: "RUN the first step.\n\n" + strings.Join(block, "\n") + "\n\n* reads: repo\n\n`{pred: first_done}`\\\n",

		deferred: newDeferral("export it", map[string]bool{"SKILL_DEMO_TEST": true}, skill, nil),
		skill:    skill,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			rendered, err := renderMarkdown(test.deferred, false, nil, test.skill)
			assert.NoError(t, err)
			assert.Contains(t, string(rendered), test._contains)
		})
	}
}

func TestRenderMarkdown_Tokens(t *testing.T) {
	type testCase struct {
		_id       string
		_contains string

		skill *Skill
	}

	link := "[First step · FLOW-002](#skill-demo-flow-002)"
	tests := make([]*testCase, 0)

	// statement-token
	tests = append(tests, &testCase{
		_id:       "statement-token",
		_contains: "RUN the second step after " + link + ".\n",

		skill: withStatement(provideFieldsSkill(), "SKILL-DEMO-FLOW-003", "RUN the second step after [[FLOW-002]]."),
	})

	// args-token
	skill := provideFieldsSkill()
	skill.Sections[1].Blocks[0].Args.Rows[0].Effect = "picks a mode, see [[FLOW-002]]"
	tests = append(tests, &testCase{
		_id:       "args-token",
		_contains: "| `mode` | `a\\|b` | `a` | picks a mode, see " + link + " |\n",

		skill: skill,
	})

	// routing-token
	skill = provideFieldsSkill()
	skill.Routing().Purpose = "demo, see [[FLOW-002]]."
	skill.Routing().Preconditions = []string{"a demo request, see [[FLOW-002]]"}
	skill.Routing().Routes[0].Use = "[[FLOW-002]]"
	tests = append(tests, &testCase{
		_id: "routing-token",
		_contains: "**Purpose:** demo, see " + link + ".\n" +
			"\n" +
			"**Disambiguation:**\n" +
			"\n" +
			"| For | Use |\n" +
			"| :--- | :--- |\n" +
			"| planning | " + link + " |\n" +
			"\n" +
			"**Preconditions:**\n" +
			"- a demo request, see " + link + "\n",

		skill: skill,
	})

	// format-2-untouched
	tests = append(tests, &testCase{
		_id:       "format-2-untouched",
		_contains: "RUN the second step after [[FLOW-002]].\n",

		skill: withStatement(provideCompleteSkill(), "SKILL-DEMO-FLOW-003", "RUN the second step after [[FLOW-002]]."),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			rendered, err := renderMarkdown(nil, false, nil, test.skill)
			assert.NoError(t, err)
			assert.Contains(t, string(rendered), test._contains)
		})
	}
}

func TestRenderMarkdown_DeployedDisabledDropsEmptiedScopes(t *testing.T) {
	skill := provideCompleteSkill()
	flowSection(skill).Blocks = append(flowSection(skill).Blocks, Block{Heading: "Later"}, Block{Entry: &Entry{Id: "SKILL-DEMO-FLOW-004", Role: RoleStep, Statement: "RUN later.", Title: "Later step"}})
	disabled := map[string]bool{"SKILL-DEMO-FLOW-004": true, "SKILL-DEMO-NIGHT-001": true}

	rendered, err := renderMarkdown(nil, true, disabled, skill)
	require.NoError(t, err)
	assert.NotContains(t, string(rendered), "### Later")
	assert.NotContains(t, string(rendered), "## Nightly")
	assert.Contains(t, string(rendered), "SKILL-DEMO-FLOW-003")
}

func TestRuntimeLines(t *testing.T) {
	skill := provideAnsweredSkill()
	skill.Docker = DockerAlways
	skill.EnableDelegation = DelegationNever
	expected := []string{"answered-by: hook", "docker: always", "enable-delegation: never"}
	assert.Equal(t, expected, runtimeLines(skill), "answered-by-first")
}

func TestWrapText(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		continuation string
		first        string
		text         string
	}

	tests := make([]*testCase, 0)

	// empty-text
	tests = append(tests, &testCase{_id: "empty-text", _expected: nil, text: "  "})

	// single-spaces
	tests = append(tests, &testCase{_id: "single-spaces", _expected: []string{"a b c"}, text: "a   b\n c"})

	// exact-width
	exact := strings.Repeat("a", 64) + " " + strings.Repeat("b", 63)
	tests = append(tests, &testCase{_id: "exact-width", _expected: []string{exact}, text: exact})

	// one-rune-over
	over := strings.Repeat("a", 64) + " " + strings.Repeat("b", 64)
	tests = append(tests, &testCase{_id: "one-rune-over", _expected: []string{strings.Repeat("a", 64), strings.Repeat("b", 64)}, text: over})

	// runes-not-bytes
	wide := strings.Repeat("ä", 64) + " " + strings.Repeat("ö", 63)
	tests = append(tests, &testCase{_id: "runes-not-bytes", _expected: []string{wide}, text: wide})

	// word-wider-than-width
	long := strings.Repeat("x", 130)
	tests = append(tests, &testCase{_id: "word-wider-than-width", _expected: []string{"a", long, "b"}, text: "a " + long + " b"})

	// prefixes
	tests = append(tests, &testCase{_id: "prefixes", _expected: []string{"- " + strings.Repeat("a", 125), "  bb"}, continuation: "  ", first: "- ", text: strings.Repeat("a", 125) + " bb"})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, wrapText(test.continuation, test.first, test.text))
		})
	}
}

func TestResolveDisable(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   map[string]bool
		_shouldPass bool

		tokens []string
	}

	tests := make([]*testCase, 0)

	// exact-id
	tests = append(tests, &testCase{_id: "exact-id", _expected: map[string]bool{"SKILL-DEMO-FLOW-002": true}, _shouldPass: true, tokens: []string{"SKILL-DEMO-FLOW-002"}})

	// trailing-glob-blank-tokens
	glob := map[string]bool{"SKILL-DEMO-FLOW-001": true, "SKILL-DEMO-FLOW-002": true, "SKILL-DEMO-FLOW-003": true}
	tests = append(tests, &testCase{_id: "trailing-glob-blank-tokens", _expected: glob, _shouldPass: true, tokens: []string{" ", "SKILL-DEMO-FLOW-*", ""}})

	// no-tokens
	tests = append(tests, &testCase{_id: "no-tokens", _expected: map[string]bool{}, _shouldPass: true, tokens: nil})

	// unmatched-token
	tests = append(tests, &testCase{_id: "unmatched-token", _expected: nil, _shouldPass: false, tokens: []string{"SKILL-NOPE-*"}})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			disabled, err := resolveDisable(provideCompleteSkill(), test.tokens)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, disabled)
		})
	}
}

// errorText is an error's message, "" for nil.
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

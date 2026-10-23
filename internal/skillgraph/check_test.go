package skillgraph

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheck_FormatFieldsMarkdownLink(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "root", "group", "demo")
	skill := withStatement(provideFieldsSkill(), "SKILL-DEMO-FLOW-003", "RUN after [First step](#skill-demo-flow-002).")
	require.NoError(t, Save(dir, skill))

	_, err := Load(dir)

	assert.ErrorContains(t, err, "markdown link to an entry: a format-3 skill writes [[SECTION-NNN]]")
}

func TestCheck(t *testing.T) {
	type testCase struct {
		_id       string
		_contains string

		skill *Skill
	}

	tests := make([]*testCase, 0)

	// pass-clean
	tests = append(tests, &testCase{
		_id:       "pass-clean",
		_contains: "",

		skill: provideCompleteSkill(),
	})

	// fail-version-mismatch
	skill := provideCompleteSkill()
	skill.Changelog[0].Version = "2.0"
	tests = append(tests, &testCase{_id: "fail-version-mismatch", _contains: "differs from the newest changelog entry", skill: skill})

	// fail-empty-changelog
	skill = provideCompleteSkill()
	skill.Changelog = nil
	tests = append(tests, &testCase{_id: "fail-empty-changelog", _contains: "changelog is empty", skill: skill})

	// pass-descending
	tests = append(tests, &testCase{_id: "pass-descending", _contains: "", skill: provideVersionedSkill("1.1", "1.0")})

	// fail-duplicate-version
	tests = append(tests, &testCase{
		_id:       "fail-duplicate-version",
		_contains: `changelog versions are not strictly descending: "1.0" above "1.0"`,

		skill: provideVersionedSkill("1.0", "1.0"),
	})

	// fail-ascending-version
	tests = append(tests, &testCase{
		_id:       "fail-ascending-version",
		_contains: `changelog versions are not strictly descending: "1.0.1" above "1.1.0"`,

		skill: provideVersionedSkill("1.0.1", "1.1.0"),
	})

	// pass-two-and-three-part
	tests = append(tests, &testCase{_id: "pass-two-and-three-part", _contains: "", skill: provideVersionedSkill("1.7.1", "1.7")})

	// fail-unparseable-version
	tests = append(tests, &testCase{_id: "fail-unparseable-version", _contains: "non-numeric component", skill: provideVersionedSkill("1.1", "1.x")})

	// fail-missing-name
	skill = provideCompleteSkill()
	skill.Frontmatter = skill.Frontmatter[1:]
	tests = append(tests, &testCase{_id: "fail-missing-name", _contains: `frontmatter key "name" missing`, skill: skill})

	// fail-duplicate-frontmatter-key
	skill = provideCompleteSkill()
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: "format", Value: " 2"})
	tests = append(tests, &testCase{_id: "fail-duplicate-frontmatter-key", _contains: `"format" appears twice`, skill: skill})

	// pass-aliases
	skill = provideCompleteSkill()
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: "aliases", Value: " p, pk"})
	tests = append(tests, &testCase{_id: "pass-aliases", _contains: "", skill: skill})

	// pass-alias-with-hyphen
	skill = provideCompleteSkill()
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: "aliases", Value: " peek-short"})
	tests = append(tests, &testCase{_id: "pass-alias-with-hyphen", _contains: "", skill: skill})

	// fail-alias-uppercase
	skill = provideCompleteSkill()
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: "aliases", Value: " Pk"})
	tests = append(tests, &testCase{_id: "fail-alias-uppercase", _contains: `alias "Pk" is not kebab-case`, skill: skill})

	// fail-alias-double-hyphen
	skill = provideCompleteSkill()
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: "aliases", Value: " a--b"})
	tests = append(tests, &testCase{_id: "fail-alias-double-hyphen", _contains: `alias "a--b" is not kebab-case`, skill: skill})

	// fail-alias-trailing-hyphen
	skill = provideCompleteSkill()
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: "aliases", Value: " pk-"})
	tests = append(tests, &testCase{_id: "fail-alias-trailing-hyphen", _contains: `alias "pk-" is not kebab-case`, skill: skill})

	// fail-alias-leading-digit
	skill = provideCompleteSkill()
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: "aliases", Value: " 1pk"})
	tests = append(tests, &testCase{_id: "fail-alias-leading-digit", _contains: `alias "1pk" is not kebab-case`, skill: skill})

	// fail-alias-empty-element
	skill = provideCompleteSkill()
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: "aliases", Value: " pk,"})
	tests = append(tests, &testCase{_id: "fail-alias-empty-element", _contains: `alias "" is not kebab-case`, skill: skill})

	// fail-alias-space-inside
	skill = provideCompleteSkill()
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: "aliases", Value: " p k"})
	tests = append(tests, &testCase{_id: "fail-alias-space-inside", _contains: `alias "p k" is not kebab-case`, skill: skill})

	// fail-alias-own-name
	skill = provideCompleteSkill()
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: "aliases", Value: " demo"})
	tests = append(tests, &testCase{_id: "fail-alias-own-name", _contains: `alias "demo" repeats the skill's own name`, skill: skill})

	// fail-alias-twice
	skill = provideCompleteSkill()
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: "aliases", Value: " pk, pk"})
	tests = append(tests, &testCase{_id: "fail-alias-twice", _contains: `alias "pk" appears twice`, skill: skill})

	// fail-duplicate-id
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[2].Entry.Id = "SKILL-DEMO-FLOW-002"
	tests = append(tests, &testCase{_id: "fail-duplicate-id", _contains: "duplicate entry id", skill: skill})

	// fail-wrong-skill-segment
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[2].Entry.Id = "SKILL-OTHER-FLOW-003"
	tests = append(tests, &testCase{_id: "fail-wrong-skill-segment", _contains: "entry names skill OTHER", skill: skill})

	// fail-statement-over-200
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[2].Entry.Statement = "RUN " + strings.Repeat("x", 200)
	tests = append(tests, &testCase{_id: "fail-statement-over-200", _contains: "statement exceeds 200 runes", skill: skill})

	// pass-statement-250-format-1
	skill = provideCompleteSkill()
	skill.Frontmatter = skill.Frontmatter[:3]
	flowSection(skill).Blocks[2].Entry.Statement = "RUN " + strings.Repeat("x", 246)
	tests = append(tests, &testCase{_id: "pass-statement-250-format-1", _contains: "", skill: skill})

	// pass-statement-long-link-target
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[2].Entry.Statement = "RUN [step](" + strings.Repeat("x", 250) + ")."
	tests = append(tests, &testCase{_id: "pass-statement-long-link-target", _contains: "", skill: skill})

	// fail-bullet-over-160
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[1].Entry.Items[0].Text = strings.Repeat("y", 160)
	tests = append(tests, &testCase{_id: "fail-bullet-over-160", _contains: "bullet exceeds 160 runes", skill: skill})

	// fail-why-bullet
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[1].Entry.Items[0].Label = "Why"
	tests = append(tests, &testCase{_id: "fail-why-bullet", _contains: "Why bullet", skill: skill})

	// fail-role-order
	skill = provideCompleteSkill()
	flow := flowSection(skill)
	flow.Blocks[0], flow.Blocks[1] = flow.Blocks[1], flow.Blocks[0]
	tests = append(tests, &testCase{_id: "fail-role-order", _contains: "breaks the section order", skill: skill})

	// pass-role-order-gapped-scope
	skill = provideCompleteSkill()
	flow = flowSection(skill)
	flow.Blocks[0], flow.Blocks[1] = flow.Blocks[1], flow.Blocks[0]
	flow.Blocks = append([]Block{flow.Blocks[0], {Prose: "gap"}}, flow.Blocks[1:]...)
	tests = append(tests, &testCase{_id: "pass-role-order-gapped-scope", _contains: "", skill: skill})

	// pass-role-order-new-subsection
	skill = provideCompleteSkill()
	flow = flowSection(skill)
	rule := flow.Blocks[0]
	flow.Blocks = append(flow.Blocks[1:3], Block{Heading: "Later"}, rule)
	tests = append(tests, &testCase{_id: "pass-role-order-new-subsection", _contains: "", skill: skill})

	// fail-route-without-anchor
	skill = provideCompleteSkill()
	routeSection(skill).Between = ""
	tests = append(tests, &testCase{_id: "fail-route-without-anchor", _contains: "route section without anchor", skill: skill})

	// fail-route-both-anchors
	skill = provideCompleteSkill()
	routeSection(skill).Replaces = "FLOW-003"
	tests = append(tests, &testCase{_id: "fail-route-both-anchors", _contains: "both replaces and between", skill: skill})

	// fail-malformed-replaces
	skill = provideCompleteSkill()
	routeSection(skill).Between = ""
	routeSection(skill).Replaces = "flow three"
	tests = append(tests, &testCase{_id: "fail-malformed-replaces", _contains: "Malformed replaces item", skill: skill})

	// fail-anchor-unknown-id
	skill = provideCompleteSkill()
	routeSection(skill).Between = "FLOW-009..end"
	tests = append(tests, &testCase{_id: "fail-anchor-unknown-id", _contains: "no such entry", skill: skill})

	// fail-anchor-on-def
	skill = provideCompleteSkill()
	flowSection(skill).Blocks = append([]Block{{Entry: &Entry{Id: "SKILL-DEMO-FLOW-000", Role: RoleDef, Statement: "A term.", Title: "Term"}}}, flowSection(skill).Blocks...)
	routeSection(skill).Between = "FLOW-000..end"
	tests = append(tests, &testCase{_id: "fail-anchor-on-def", _contains: "a def is never an anchor", skill: skill})

	// fail-anchor-inside-route
	skill = provideCompleteSkill()
	routeSection(skill).Between = "NIGHT-001..end"
	tests = append(tests, &testCase{_id: "fail-anchor-inside-route", _contains: "inside the route itself", skill: skill})

	// fail-between-reentry-before-split
	skill = provideCompleteSkill()
	routeSection(skill).Between = "FLOW-003..FLOW-002"
	tests = append(tests, &testCase{_id: "fail-between-reentry-before-split", _contains: "reentry at or before the split", skill: skill})

	// pass-between-start-end
	skill = provideCompleteSkill()
	routeSection(skill).Between = "start..end"
	tests = append(tests, &testCase{_id: "pass-between-start-end", _contains: "", skill: skill})

	// fail-replaces-range-out-of-order
	skill = provideCompleteSkill()
	routeSection(skill).Between = ""
	routeSection(skill).Replaces = "FLOW-003..FLOW-002"
	tests = append(tests, &testCase{_id: "fail-replaces-range-out-of-order", _contains: "range out of order", skill: skill})

	// fail-atom-redeclares-vocabulary
	skill = provideCompleteSkill()
	skill.Vocabulary = &Vocabulary{Atoms: map[string]*Atom{"panic": {Sort: "state"}}}
	tests = append(tests, &testCase{_id: "fail-atom-redeclares-vocabulary", _contains: "declared in the skill's model file and in the shared vocabulary.cue", skill: skill})

	// fail-prose-line-129-runes
	skill = provideCompleteSkill()
	skill.Sections[0].Blocks[0] = Block{Prose: strings.Repeat("z", 129)}
	tests = append(tests, &testCase{_id: "fail-prose-line-129-runes", _contains: "line exceeds 128 runes", skill: skill})

	// pass-prose-long-line-format-1
	skill = provideCompleteSkill()
	skill.Frontmatter = skill.Frontmatter[:3]
	skill.Sections[0].Blocks[0] = Block{Prose: strings.Repeat("z", 129)}
	tests = append(tests, &testCase{_id: "pass-prose-long-line-format-1", _contains: "", skill: skill})

	// pass-prose-long-table-row
	skill = provideCompleteSkill()
	skill.Sections[0].Blocks[0] = Block{Prose: "| " + strings.Repeat("z", 140) + " |"}
	tests = append(tests, &testCase{_id: "pass-prose-long-table-row", _contains: "", skill: skill})

	// fail-routing-not-first
	skill = provideCompleteSkill()
	skill.Sections[0].Blocks = append([]Block{{Prose: "intro"}}, skill.Sections[0].Blocks...)
	tests = append(tests, &testCase{_id: "fail-routing-not-first", _contains: "a routing block is the first block of the Routing section", skill: skill})

	// fail-routing-outside-routing
	skill = provideCompleteSkill()
	skill.Sections[1].Blocks = append(skill.Sections[1].Blocks, Block{Routing: skill.Routing()})
	tests = append(tests, &testCase{_id: "fail-routing-outside-routing", _contains: `section "Args" block 2: a routing block`, skill: skill})

	// fail-two-routing-blocks
	skill = provideCompleteSkill()
	skill.Sections[1].Blocks = append(skill.Sections[1].Blocks, Block{Routing: skill.Routing()})
	tests = append(tests, &testCase{_id: "fail-two-routing-blocks", _contains: "at most one routing block", skill: skill})

	// fail-model-beside-prose
	skill = provideCompleteSkill()
	modelSection := skill.Sections[len(skill.Sections)-1]
	modelSection.Blocks = append(modelSection.Blocks, Block{Prose: "extra"})
	tests = append(tests, &testCase{_id: "fail-model-beside-prose", _contains: "a model block is the only block of the Model section", skill: skill})

	// fail-empty-pipeline
	skill = provideCompleteSkill()
	skill.Routing().Pipeline = Pipeline{}
	tests = append(tests, &testCase{_id: "fail-empty-pipeline", _contains: "pipeline: empty", skill: skill})

	// fail-self-edge
	skill = provideCompleteSkill()
	skill.Routing().Pipeline.Edges = []Edge{{Kind: EdgeKindTo, Skill: "demo"}}
	tests = append(tests, &testCase{_id: "fail-self-edge", _contains: "edge to demo names the skill itself", skill: skill})

	// fail-duplicate-edge
	skill = provideCompleteSkill()
	skill.Routing().Pipeline.Edges = []Edge{{Kind: EdgeKindTo, Skill: "fdesign"}, {Kind: EdgeKindTo, Optional: true, Skill: "fdesign"}}
	tests = append(tests, &testCase{_id: "fail-duplicate-edge", _contains: "edge to fdesign is declared twice", skill: skill})

	// pass-loop-edges
	skill = provideCompleteSkill()
	skill.Routing().Pipeline.Edges = []Edge{{Kind: EdgeKindTo, Skill: "fdesign"}, {Kind: EdgeKindFrom, Skill: "fdesign"}}
	tests = append(tests, &testCase{_id: "pass-loop-edges", _contains: "", skill: skill})

	// fail-bypass-beside-mode
	skill = provideCompleteSkill()
	skill.Docker = DockerAlways
	skill.EnforcePermissions = []string{PermissionBypass, PermissionAcceptEdits}
	tests = append(tests, &testCase{_id: "fail-bypass-beside-mode", _contains: "bypass is exclusive", skill: skill})

	// fail-bypass-without-docker
	skill = provideCompleteSkill()
	skill.EnforcePermissions = []string{PermissionBypass}
	tests = append(tests, &testCase{_id: "fail-bypass-without-docker", _contains: `bypass requires docker: "always"`, skill: skill})

	// pass-bypass-docker-always
	skill = provideCompleteSkill()
	skill.Docker = DockerAlways
	skill.EnforcePermissions = []string{PermissionBypass}
	tests = append(tests, &testCase{_id: "pass-bypass-docker-always", _contains: "", skill: skill})

	// fail-mode-twice
	skill = provideCompleteSkill()
	skill.EnforcePermissions = []string{PermissionAcceptEdits, PermissionAcceptEdits}
	tests = append(tests, &testCase{_id: "fail-mode-twice", _contains: `lists "acceptEdits" twice`, skill: skill})

	// fail-script-pointer-delegable
	skill = provideCompleteSkill()
	scripted := skill.Entries()[0].Entry
	scripted.Items = append(scripted.Items, Item{Label: LabelScript, Marker: "*", Text: "workflows/stage.js"})
	tests = append(tests, &testCase{_id: "fail-script-pointer-delegable", _contains: `declare enableDelegation: "never"`, skill: skill})

	// pass-script-pointer-root-only
	skill = provideCompleteSkill()
	skill.EnableDelegation = DelegationNever
	scripted = skill.Entries()[0].Entry
	scripted.Items = append(scripted.Items, Item{Label: LabelScript, Marker: "*", Text: "workflows/stage.js"})
	tests = append(tests, &testCase{_id: "pass-script-pointer-root-only", _contains: "", skill: skill})

	// fail-derived-key-authored
	skill = provideCompleteSkill()
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: FieldDocker, Value: " always"})
	tests = append(tests, &testCase{_id: "fail-derived-key-authored", _contains: `frontmatter key "docker" is rendered`, skill: skill})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			violations := strings.Join(Check(test.skill), "\n")
			assert.Equalf(t, test._contains == "", violations == "", "violations = %s", violations)
			assert.Contains(t, violations, test._contains)
		})
	}
}

func TestCheckRuntimeMeta_AnsweredBy(t *testing.T) {
	type testCase struct {
		_id       string
		_contains string

		skill *Skill
	}

	tests := make([]*testCase, 0)

	// authored-answered-by-key-refused
	skill := provideAnsweredSkill()
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: FieldAnsweredBy, Value: " hook"})
	tests = append(tests, &testCase{_id: "authored-answered-by-key-refused", _contains: `frontmatter key "answered-by" is rendered`, skill: skill})

	// flow-entry-refused
	skill = provideCompleteSkill()
	skill.AnsweredBy = AnsweredByHook
	tests = append(tests, &testCase{_id: "flow-entry-refused", _contains: "answeredBy: entry SKILL-DEMO-FLOW-002 is a flow entry", skill: skill})

	// no-payload-refused
	skill = provideAnsweredSkill()
	flowSection(skill).Blocks = flowSection(skill).Blocks[:1]
	tests = append(tests, &testCase{_id: "no-payload-refused", _contains: "answeredBy: no payload entry", skill: skill})

	// payload-only-passes
	tests = append(tests, &testCase{_id: "payload-only-passes", _contains: "", skill: provideAnsweredSkill()})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			violations := strings.Join(Check(test.skill), "\n")
			assert.Equalf(t, test._contains == "", violations == "", "violations = %s", violations)
			assert.Contains(t, violations, test._contains)
		})
	}
}

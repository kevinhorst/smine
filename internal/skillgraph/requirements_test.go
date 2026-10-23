package skillgraph

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/sessiontitle"
	"github.com/kevinhorst/smine/internal/skillenv"
)

// componentIdPattern extracts the #ComponentId disjunction from the schema.
var componentIdPattern = regexp.MustCompile(`#ComponentId:\s*(.+)`)

// provideRequiringSkill is the complete fixture with every component field
// set: requires peek, FLOW-002 needs gate, FLOW-003 unless mining, the route
// and the edge need mining.
func provideRequiringSkill() *Skill {
	skill := provideCompleteSkill()
	skill.Requires = []components.ID{components.Peek}
	flowSection(skill).Blocks[1].Entry.Needs = []components.ID{components.Gate}
	flowSection(skill).Blocks[2].Entry.Unless = []components.ID{components.Mining}
	skill.Routing().Routes[0].Needs = []components.ID{components.Mining}
	skill.Routing().Pipeline.Edges[0].Needs = []components.ID{components.Mining}
	return skill
}

// provideBuildSkill is the complete fixture whose FLOW-003 statement reads the optional $SKILL_DEMO_BUILD.
func provideBuildSkill() *Skill {
	skill := provideCompleteSkill()
	bindEnv(skill, map[string]*Variable{"BUILD": {Interface: "build@1"}})
	flowSection(skill).Blocks[2].Entry.Statement = "RUN `$SKILL_DEMO_BUILD`."
	return skill
}

// provideVariableEnforceSkill carries an enforce block whose status anchor reads $SKILL_DEMO_AGENTWT_STATUS and whose lanes anchor, check and judgment read $SKILL_DEMO_LANES.
func provideVariableEnforceSkill() *Skill {
	skill := provideCompleteSkill()
	bindEnv(skill, map[string]*Variable{
		"AGENTWT_STATUS": {Interface: "agentwt-status@1"},
		"LANES":          {Interface: "lanes@1"},
	})
	skill.Enforce = &Enforce{
		Anchors: map[string]*Anchor{
			"lanes":  {Input: []InputMatch{{Field: "command", Pattern: "^$SKILL_DEMO_LANES lanes"}}, Tool: "^Bash$"},
			"stage":  {Tool: "^Workflow$"},
			"status": {Input: []InputMatch{{Field: "command", Pattern: "^$SKILL_DEMO_AGENTWT_STATUS status"}}, Tool: "^Bash$"},
		},
		Checks: map[string]*EnforceCheck{
			"lanes_first":  {After: "lanes", Kind: CheckRequires, On: "stage", Reason: "r", Recovery: "x"},
			"status_first": {After: "status", Kind: CheckRequires, On: "stage", Reason: "r", Recovery: "x"},
		},
		Judgment: map[string]*Judgment{
			"lanes_judged": {Anchor: "lanes", Evidence: []string{"e"}, Question: "q", Send: "s"},
		},
	}
	return skill
}

// renderComponents renders the skill's deployed files for options.
func renderComponents(t *testing.T, options RenderOptions, skill *Skill) []RenderedFile {
	t.Helper()
	files, err := RenderDeployed(options, skill)
	require.NoError(t, err)
	return files
}

func TestCheckRequirements(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		skill *Skill
	}

	tests := make([]*testCase, 0)

	// valid-all-fields
	tests = append(tests, &testCase{
		_id:       "valid-all-fields",
		_expected: nil,

		skill: provideRequiringSkill(),
	})

	// unknown-requires
	skill := provideCompleteSkill()
	skill.Requires = []components.ID{"nope"}
	tests = append(tests, &testCase{
		_id:       "unknown-requires",
		_expected: []string{`requires: unknown component "nope"`},

		skill: skill,
	})

	// unknown-needs
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[1].Entry.Needs = []components.ID{"nope"}
	tests = append(tests, &testCase{
		_id:       "unknown-needs",
		_expected: []string{`SKILL-DEMO-FLOW-002: needs: unknown component "nope"`},

		skill: skill,
	})

	// unknown-unless
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[1].Entry.Unless = []components.ID{"nope"}
	tests = append(tests, &testCase{
		_id:       "unknown-unless",
		_expected: []string{`SKILL-DEMO-FLOW-002: unless: unknown component "nope"`},

		skill: skill,
	})

	// needs-in-requires
	skill = provideCompleteSkill()
	skill.Requires = []components.ID{components.Peek}
	flowSection(skill).Blocks[1].Entry.Needs = []components.ID{components.Peek}
	tests = append(tests, &testCase{
		_id:       "needs-in-requires",
		_expected: []string{"SKILL-DEMO-FLOW-002: needs peek, which the skill requires — a requirement is hard or soft, never both"},

		skill: skill,
	})

	// needs-and-unless-same
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[1].Entry.Needs = []components.ID{components.Gate}
	flowSection(skill).Blocks[1].Entry.Unless = []components.ID{components.Gate}
	tests = append(tests, &testCase{
		_id:       "needs-and-unless-same",
		_expected: []string{"SKILL-DEMO-FLOW-002: needs and unless both name gate"},

		skill: skill,
	})

	// unknown-route-needs
	skill = provideCompleteSkill()
	skill.Routing().Routes[0].Needs = []components.ID{"nope"}
	tests = append(tests, &testCase{
		_id:       "unknown-route-needs",
		_expected: []string{`route /fdesign: needs: unknown component "nope"`},

		skill: skill,
	})

	// unknown-edge-needs
	skill = provideCompleteSkill()
	skill.Routing().Pipeline.Edges[0].Needs = []components.ID{"nope"}
	tests = append(tests, &testCase{
		_id:       "unknown-edge-needs",
		_expected: []string{`edge to fdesign: needs: unknown component "nope"`},

		skill: skill,
	})

	// route-needs-in-requires
	skill = provideCompleteSkill()
	skill.Requires = []components.ID{components.Mining}
	skill.Routing().Routes[0].Needs = []components.ID{components.Mining}
	tests = append(tests, &testCase{
		_id:       "route-needs-in-requires",
		_expected: []string{"route /fdesign: needs mining, which the skill requires — a requirement is hard or soft, never both"},

		skill: skill,
	})

	// edge-needs-in-requires
	skill = provideCompleteSkill()
	skill.Requires = []components.ID{components.Mining}
	skill.Routing().Pipeline.Edges[0].Needs = []components.ID{components.Mining}
	tests = append(tests, &testCase{
		_id:       "edge-needs-in-requires",
		_expected: []string{"edge to fdesign: needs mining, which the skill requires — a requirement is hard or soft, never both"},

		skill: skill,
	})

	// no-routing-block
	skill = provideCompleteSkill()
	skill.Sections = skill.Sections[1:]
	skill.Requires = []components.ID{components.Agentwt}
	tests = append(tests, &testCase{
		_id:       "no-routing-block",
		_expected: nil,

		skill: skill,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, checkRequirements(test.skill))
		})
	}
}

func TestRenderDeployed_components(t *testing.T) {
	type testCase struct {
		_id             string
		_absent         []string
		_enforceAbsent  []string
		_enforcePresent []string
		_paths          []string
		_present        []string
		_shouldPass     bool

		options RenderOptions
		skill   *Skill
	}

	var canonical RenderOptions
	empty := RenderOptions{Components: []components.ID{}}
	unset := RenderOptions{Values: map[string]string{}}
	tests := make([]*testCase, 0)

	// nil-set-keeps-needs-entry
	skill := provideCompleteSkill()
	flowSection(skill).Blocks[2].Entry.Needs = []components.ID{components.Gate}
	tests = append(tests, &testCase{
		_id:         "nil-set-keeps-needs-entry",
		_paths:      []string{FileManifest},
		_present:    []string{"SKILL-DEMO-FLOW-003"},
		_shouldPass: true,

		options: canonical,
		skill:   skill,
	})

	// nil-set-drops-unless-entry
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[2].Entry.Unless = []components.ID{components.Gate}
	tests = append(tests, &testCase{
		_id:         "nil-set-drops-unless-entry",
		_absent:     []string{"SKILL-DEMO-FLOW-003"},
		_paths:      []string{FileManifest},
		_shouldPass: true,

		options: canonical,
		skill:   skill,
	})

	// empty-set-drops-needs-entry
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[2].Entry.Needs = []components.ID{components.Gate}
	tests = append(tests, &testCase{
		_id:         "empty-set-drops-needs-entry",
		_absent:     []string{"SKILL-DEMO-FLOW-003"},
		_paths:      []string{FileManifest},
		_present:    []string{"SKILL-DEMO-FLOW-002"},
		_shouldPass: true,

		options: empty,
		skill:   skill,
	})

	// empty-set-keeps-unless-entry
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[2].Entry.Unless = []components.ID{components.Gate}
	tests = append(tests, &testCase{
		_id:         "empty-set-keeps-unless-entry",
		_paths:      []string{FileManifest},
		_present:    []string{"SKILL-DEMO-FLOW-003"},
		_shouldPass: true,

		options: empty,
		skill:   skill,
	})

	// heading-emptied-by-drop
	skill = provideCompleteSkill()
	gated := routeSection(skill)
	gated.Blocks[0].Entry.Needs = []components.ID{components.Gate}
	gated.Blocks = append([]Block{{Heading: "Gated"}}, gated.Blocks...)
	tests = append(tests, &testCase{
		_id:         "heading-emptied-by-drop",
		_absent:     []string{"### Gated", "SKILL-DEMO-NIGHT-001"},
		_paths:      []string{FileManifest},
		_shouldPass: true,

		options: empty,
		skill:   skill,
	})

	// allowed-tools-unset-rule-dropped
	lanes := map[string]*Variable{"LANES": {Interface: "lanes@1"}}
	skill = provideCompleteSkill()
	bindEnv(skill, lanes)
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: fieldAllowedTools, Value: " Bash($SKILL_DEMO_LANES lanes *), Read"})
	tests = append(tests, &testCase{
		_id:         "allowed-tools-unset-rule-dropped",
		_absent:     []string{"SKILL_DEMO_LANES"},
		_paths:      []string{FileManifest},
		_present:    []string{"\nallowed-tools: Read\n"},
		_shouldPass: true,

		options: unset,
		skill:   skill,
	})

	// allowed-tools-set-rule-filled
	skill = provideCompleteSkill()
	bindEnv(skill, lanes)
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: fieldAllowedTools, Value: " Bash($SKILL_DEMO_LANES lanes *), Read"})
	tests = append(tests, &testCase{
		_id:         "allowed-tools-set-rule-filled",
		_paths:      []string{FileManifest},
		_present:    []string{"\nallowed-tools: Bash(smine-rules lanes *), Read\n"},
		_shouldPass: true,

		options: RenderOptions{Values: map[string]string{"SKILL_DEMO_LANES": "smine-rules"}},
		skill:   skill,
	})

	// allowed-tools-canonical-kept
	skill = provideCompleteSkill()
	bindEnv(skill, lanes)
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: fieldAllowedTools, Value: " Bash($SKILL_DEMO_LANES lanes *), Read"})
	tests = append(tests, &testCase{
		_id:         "allowed-tools-canonical-kept",
		_paths:      []string{FileManifest},
		_present:    []string{"\nallowed-tools: Bash($SKILL_DEMO_LANES lanes *), Read\n"},
		_shouldPass: true,

		options: canonical,
		skill:   skill,
	})

	// allowed-tools-emptied-field-dropped
	skill = provideCompleteSkill()
	bindEnv(skill, map[string]*Variable{"HEARTBEAT_STATUS": {Interface: "heartbeat-status@1"}})
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: fieldAllowedTools, Value: " Bash($SKILL_DEMO_HEARTBEAT_STATUS status *)"})
	tests = append(tests, &testCase{
		_id:         "allowed-tools-emptied-field-dropped",
		_absent:     []string{"allowed-tools"},
		_paths:      []string{FileManifest},
		_shouldPass: true,

		options: unset,
		skill:   skill,
	})

	// allowed-tools-undeployable-skill-dropped
	skill = provideCompleteSkill()
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: fieldAllowedTools, Value: " Skill(restack), Read"})
	tests = append(tests, &testCase{
		_id:         "allowed-tools-undeployable-skill-dropped",
		_absent:     []string{"Skill(restack)"},
		_paths:      []string{FileManifest},
		_present:    []string{"\nallowed-tools: Read\n"},
		_shouldPass: true,

		options: RenderOptions{Deployable: map[string]bool{"restack": false}},
		skill:   skill,
	})

	// unset-variable-entry-dropped
	tests = append(tests, &testCase{
		_id:         "unset-variable-entry-dropped",
		_absent:     []string{"SKILL-DEMO-FLOW-003", "SKILL_DEMO_BUILD"},
		_paths:      []string{FileManifest},
		_present:    []string{"SKILL-DEMO-FLOW-002"},
		_shouldPass: true,

		options: unset,
		skill:   provideBuildSkill(),
	})

	// unset-variable-payload-entry-dropped
	skill = provideCompleteSkill()
	bindEnv(skill, map[string]*Variable{"BUILD": {Interface: "build@1"}})
	flowSection(skill).Blocks[3].Entry.Payload.Body = "RUN $SKILL_DEMO_BUILD\n"
	tests = append(tests, &testCase{
		_id:         "unset-variable-payload-entry-dropped",
		_absent:     []string{"SKILL-DEMO-TPL-001", "SKILL_DEMO_BUILD"},
		_paths:      []string{FileManifest},
		_present:    []string{"SKILL-DEMO-FLOW-003"},
		_shouldPass: true,

		options: unset,
		skill:   skill,
	})

	// unset-variable-item-dropped
	skill = provideCompleteSkill()
	bindEnv(skill, map[string]*Variable{"BUILD": {Interface: "build@1"}})
	gate := flowSection(skill).Blocks[1].Entry
	gate.Items = append(gate.Items, Item{Label: LabelParam, Marker: "*", Text: "build: `$SKILL_DEMO_BUILD`."})
	tests = append(tests, &testCase{
		_id:         "unset-variable-item-dropped",
		_absent:     []string{"SKILL_DEMO_BUILD", "* param: build"},
		_paths:      []string{FileManifest},
		_present:    []string{"SKILL-DEMO-FLOW-002", "* param: one fact."},
		_shouldPass: true,

		options: unset,
		skill:   skill,
	})

	// set-variable-filled
	tests = append(tests, &testCase{
		_id:         "set-variable-filled",
		_absent:     []string{"SKILL_DEMO_BUILD"},
		_paths:      []string{FileManifest},
		_present:    []string{"SKILL-DEMO-FLOW-003", "RUN `go build ./...`."},
		_shouldPass: true,

		options: RenderOptions{Values: map[string]string{"SKILL_DEMO_BUILD": "go build ./..."}},
		skill:   provideBuildSkill(),
	})

	// nil-values-canonical-text
	tests = append(tests, &testCase{
		_id:         "nil-values-canonical-text",
		_paths:      []string{FileManifest},
		_present:    []string{"SKILL-DEMO-FLOW-003", "RUN `$SKILL_DEMO_BUILD`."},
		_shouldPass: true,

		options: canonical,
		skill:   provideBuildSkill(),
	})

	// enforce-pattern-filled-quoted
	tests = append(tests, &testCase{
		_id:             "enforce-pattern-filled-quoted",
		_enforceAbsent:  []string{"SKILL_DEMO", "lanes"},
		_enforcePresent: []string{`"^go run \\./cmd/agentwt status"`, `"status_first"`, `"stage"`},
		_paths:          []string{FileManifest, FileEnforce},
		_shouldPass:     true,

		options: RenderOptions{Values: map[string]string{"SKILL_DEMO_AGENTWT_STATUS": "go run ./cmd/agentwt"}},
		skill:   provideVariableEnforceSkill(),
	})

	// enforce-check-reading-unset-dropped
	skill = provideVariableEnforceSkill()
	skill.Enforce.Checks["status_noted"] = &EnforceCheck{After: "status", Kind: CheckRequires, On: "stage", Reason: "run `$SKILL_DEMO_LANES lanes` first", Recovery: "x"}
	tests = append(tests, &testCase{
		_id:             "enforce-check-reading-unset-dropped",
		_enforceAbsent:  []string{"status_noted", "SKILL_DEMO"},
		_enforcePresent: []string{`"status_first"`},
		_paths:          []string{FileManifest, FileEnforce},
		_shouldPass:     true,

		options: RenderOptions{Values: map[string]string{"SKILL_DEMO_AGENTWT_STATUS": "go run ./cmd/agentwt"}},
		skill:   skill,
	})

	// enforce-canonical-untouched
	tests = append(tests, &testCase{
		_id:             "enforce-canonical-untouched",
		_enforcePresent: []string{`"^$SKILL_DEMO_AGENTWT_STATUS status"`, `"lanes_first"`, `"lanes_judged"`},
		_paths:          []string{FileManifest, FileEnforce},
		_shouldPass:     true,

		options: canonical,
		skill:   provideVariableEnforceSkill(),
	})

	// edge-to-undeployable-dropped
	tests = append(tests, &testCase{
		_id:         "edge-to-undeployable-dropped",
		_absent:     []string{"→ /fdesign"},
		_paths:      []string{FileManifest},
		_present:    []string{"escalates to /fimplement"},
		_shouldPass: true,

		options: RenderOptions{Deployable: map[string]bool{"fdesign": false}},
		skill:   provideCompleteSkill(),
	})

	// route-dropped
	skill = provideCompleteSkill()
	mined := Route{For: "mining", Needs: []components.ID{components.Mining}, Use: "/smine"}
	skill.Routing().Routes = append(skill.Routing().Routes, mined)
	tests = append(tests, &testCase{
		_id:         "route-dropped",
		_absent:     []string{"/smine"},
		_paths:      []string{FileManifest},
		_present:    []string{"| planning | /fdesign |"},
		_shouldPass: true,

		options: empty,
		skill:   skill,
	})

	// edge-dropped
	skill = provideCompleteSkill()
	skill.Routing().Pipeline.Edges[0].Needs = []components.ID{components.Mining}
	tests = append(tests, &testCase{
		_id:         "edge-dropped",
		_absent:     []string{"→ /fdesign"},
		_paths:      []string{FileManifest},
		_present:    []string{"escalates to /fimplement"},
		_shouldPass: true,

		options: empty,
		skill:   skill,
	})

	// enforce-needs-gate-and-heartbeat
	skill = provideCompleteSkill()
	skill.Enforce = demoEnforce()
	tests = append(tests, &testCase{
		_id:         "enforce-needs-gate-and-heartbeat",
		_paths:      []string{FileManifest},
		_shouldPass: true,

		options: RenderOptions{Components: []components.ID{components.Gate}},
		skill:   skill,
	})

	// enforce-with-gate-and-heartbeat
	skill = provideCompleteSkill()
	skill.Enforce = demoEnforce()
	tests = append(tests, &testCase{
		_id:         "enforce-with-gate-and-heartbeat",
		_paths:      []string{FileManifest, FileEnforce},
		_shouldPass: true,

		options: RenderOptions{Components: []components.ID{components.Gate, components.Heartbeat}},
		skill:   skill,
	})

	// title-needs-session-helpers
	tests = append(tests, &testCase{
		_id:         "title-needs-session-helpers",
		_paths:      []string{FileManifest},
		_shouldPass: true,

		options: empty,
		skill:   provideTitledSkill(),
	})

	// title-with-session-helpers
	tests = append(tests, &testCase{
		_id:         "title-with-session-helpers",
		_paths:      []string{FileManifest, sessiontitle.FileName},
		_shouldPass: true,

		options: RenderOptions{Components: []components.ID{components.SessionHelpers}},
		skill:   provideTitledSkill(),
	})

	// disable-and-components-combined
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[2].Entry.Needs = []components.ID{components.Gate}
	tests = append(tests, &testCase{
		_id:         "disable-and-components-combined",
		_absent:     []string{"SKILL-DEMO-FLOW-003", "SKILL-DEMO-TPL-001"},
		_paths:      []string{FileManifest},
		_present:    []string{"SKILL-DEMO-FLOW-002"},
		_shouldPass: true,

		options: RenderOptions{Components: []components.ID{}, Disable: []string{"SKILL-DEMO-TPL-001"}},
		skill:   skill,
	})

	// deferred-statement-block
	deferredEnv := map[string]*Variable{"CHECK": {Interface: "check@1", Required: true}, "TEST": {Interface: "test@1"}}
	deferring := RenderOptions{
		Deferred:     map[string]bool{"SKILL_DEMO_CHECK": true, "SKILL_DEMO_TEST": true},
		DeferredHint: "export it",
		Values:       map[string]string{},
	}
	skill = provideCompleteSkill()
	bindEnv(skill, deferredEnv)
	flowSection(skill).Blocks[2].Entry.Statement = "RUN `$SKILL_DEMO_CHECK`."
	flowSection(skill).Blocks[1].Entry.Items = append(flowSection(skill).Blocks[1].Entry.Items, Item{Label: LabelParam, Marker: "*", Text: "tests: `$SKILL_DEMO_TEST`."})
	tests = append(tests, &testCase{
		_id:         "deferred-statement-block",
		_absent:     []string{"$SKILL_DEMO_CHECK", "**SKILL-DEMO-FLOW-003** `[step]`"},
		_paths:      []string{FileManifest, skillenv.FileName},
		_present:    []string{"\n\n```!\nawk 'BEGIN { " + checkGuard + `printf "%s", "\052\052SKILL-DEMO-FLOW-003\052\052 \140\133step\135\140 \052\052Second step\052\052\012\012RUN \140" ENVIRON["SKILL_DEMO_CHECK"] "\140." }'` + "\n```\n\n"},
		_shouldPass: true,

		options: deferring,
		skill:   skill,
	})

	// deferred-item-block
	tests = append(tests, &testCase{
		_id:         "deferred-item-block",
		_absent:     []string{"$SKILL_DEMO_TEST"},
		_paths:      []string{FileManifest, skillenv.FileName},
		_present:    []string{"**SKILL-DEMO-FLOW-002** `[step]` **First step**\n\nRUN the first step.\n* param: one fact.\n\n```!\nawk 'BEGIN { " + testGuard + `printf "%s", "\052 param: tests: \140" ENVIRON["SKILL_DEMO_TEST"] "\140." }'` + "\n```\n"},
		_shouldPass: true,

		options: deferring,
		skill:   skill,
	})

	// deferred-payload-block
	skill = provideCompleteSkill()
	bindEnv(skill, deferredEnv)
	flowSection(skill).Blocks[2].Entry.Statement = "RUN `$SKILL_DEMO_TEST`."
	flowSection(skill).Blocks[3].Entry.Payload.Body = "RUN $SKILL_DEMO_CHECK\n"
	tests = append(tests, &testCase{
		_id:         "deferred-payload-block",
		_absent:     []string{"$SKILL_DEMO_CHECK", "```markdown"},
		_paths:      []string{FileManifest, skillenv.FileName},
		_present:    []string{"```!\nawk 'BEGIN { " + checkGuard + `printf "%s", "\052\052SKILL-DEMO-TPL-001\052\052 \140\133payload\135\140 \052\052Template\052\052\012\012The template.\012\012\140\140\140markdown\012RUN " ENVIRON["SKILL_DEMO_CHECK"] "\012\140\140\140" }'` + "\n```\n"},
		_shouldPass: true,

		options: deferring,
		skill:   skill,
	})

	// deferred-grant-kept
	skill = provideBuildSkill()
	bindEnv(skill, map[string]*Variable{"BUILD": {Interface: "build@1"}, "CHECK": {Interface: "check@1", Required: true}, "LANES": {Interface: "lanes@1"}})
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: fieldAllowedTools, Value: " Bash($SKILL_DEMO_CHECK), Bash($SKILL_DEMO_LANES lanes *), Read"})
	tests = append(tests, &testCase{
		_id:         "deferred-grant-kept",
		_paths:      []string{FileManifest, skillenv.FileName},
		_present:    []string{"\nallowed-tools: Bash($SKILL_DEMO_CHECK), Bash(smine-rules lanes *), Read, Bash(awk *)\n", buildGuard},
		_shouldPass: true,

		options: RenderOptions{
			Deferred:     map[string]bool{"SKILL_DEMO_BUILD": true, "SKILL_DEMO_CHECK": true},
			DeferredHint: "export it",
			Values:       map[string]string{"SKILL_DEMO_LANES": "smine-rules"},
		},
		skill: skill,
	})

	// awk-grant-appended
	skill = provideCompleteSkill()
	bindEnv(skill, deferredEnv)
	flowSection(skill).Blocks[2].Entry.Statement = "RUN `$SKILL_DEMO_CHECK` then `$SKILL_DEMO_TEST`."
	tests = append(tests, &testCase{
		_id:         "awk-grant-appended",
		_paths:      []string{FileManifest, skillenv.FileName},
		_present:    []string{"format: 2\nallowed-tools: Bash(awk *)\n---\n"},
		_shouldPass: true,

		options: deferring,
		skill:   skill,
	})

	// deferred-env-needs-component
	tests = append(tests, &testCase{
		_id:         "deferred-env-needs-component",
		_paths:      []string{FileManifest},
		_present:    []string{"```!\nawk 'BEGIN { " + checkGuard},
		_shouldPass: true,

		options: RenderOptions{
			Components:   []components.ID{components.SessionHelpers},
			Deferred:     deferring.Deferred,
			DeferredHint: deferring.DeferredHint,
			Values:       deferring.Values,
		},
		skill: skill,
	})

	// nil-deferred-canonical
	tests = append(tests, &testCase{
		_id:         "nil-deferred-canonical",
		_absent:     []string{"```!", "awk 'BEGIN", "allowed-tools"},
		_paths:      []string{FileManifest},
		_present:    []string{"RUN `$SKILL_DEMO_CHECK` then `$SKILL_DEMO_TEST`."},
		_shouldPass: true,

		options: canonical,
		skill:   skill,
	})

	// deferred-install-value-filled
	skill = provideCompleteSkill()
	bindEnv(skill, map[string]*Variable{"LANES": {Interface: "lanes@1"}, "TEST": {Interface: "test@1"}})
	flowSection(skill).Blocks[2].Entry.Statement = "RUN `$SKILL_DEMO_TEST`, then `$SKILL_DEMO_LANES lanes --round-dir {dir}`."
	tests = append(tests, &testCase{
		_id:         "deferred-install-value-filled",
		_absent:     []string{"$SKILL_"},
		_paths:      []string{FileManifest, skillenv.FileName},
		_present:    []string{`RUN \140" ENVIRON["SKILL_DEMO_TEST"] "\140, then \140smine-rules lanes --round-dir \173dir\175\140." }'`},
		_shouldPass: true,

		options: RenderOptions{
			Deferred:     map[string]bool{"SKILL_DEMO_TEST": true},
			DeferredHint: "export it",
			Values:       map[string]string{"SKILL_DEMO_LANES": "smine-rules"},
		},
		skill: skill,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			files, err := RenderDeployed(test.options, test.skill)
			markdown := renderedContent(files, FileManifest)
			enforce := renderedContent(files, FileEnforce)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._paths, renderedPaths(files))
			for _, snippet := range test._present {
				assert.Contains(t, markdown, snippet)
			}
			for _, snippet := range test._absent {
				assert.NotContains(t, markdown, snippet)
			}
			for _, snippet := range test._enforcePresent {
				assert.Contains(t, enforce, snippet)
			}
			for _, snippet := range test._enforceAbsent {
				assert.NotContains(t, enforce, snippet)
			}
		})
	}
}

func TestRenderDeployed_modelUnchanged(t *testing.T) {
	skill := provideRequiringSkill()
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: fieldAllowedTools, Value: " Bash(smine-rules *), Read"})
	empty := RenderOptions{Components: []components.ID{}}
	renderComponents(t, empty, skill)

	want := provideRequiringSkill()
	want.Frontmatter = append(want.Frontmatter, Field{Key: fieldAllowedTools, Value: " Bash(smine-rules *), Read"})
	assert.Equal(t, want, skill, "model-unchanged-after-render")
}

func TestAllowedToolsView(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		deferred   *deferral
		deployable map[string]bool
		isGated    bool
		value      string
		values     map[string]string
	}

	set := map[string]string{"SKILL_DEMO_RULES": "smine-rules"}
	skill := provideCompleteSkill()
	bindEnv(skill, map[string]*Variable{"CHECK": {Interface: "check@1", Required: true}, "LANES": {Interface: "lanes@1"}})
	deferred := newDeferral("export it", map[string]bool{"SKILL_DEMO_CHECK": true}, skill, map[string]string{})
	tests := make([]*testCase, 0)

	// deferred-reference-kept
	tests = append(tests, &testCase{
		_id:       "deferred-reference-kept",
		_expected: " Bash($SKILL_DEMO_CHECK), Read, Bash(awk *)",

		deferred:   deferred,
		deployable: nil,
		isGated:    true,
		value:      " Bash($SKILL_DEMO_CHECK), Read",
		values:     map[string]string{},
	})

	// deferred-reference-dropped-without-gate
	tests = append(tests, &testCase{
		_id:       "deferred-reference-dropped-without-gate",
		_expected: " Read, Bash(awk *)",

		deferred:   deferred,
		deployable: nil,
		isGated:    false,
		value:      " Bash($SKILL_DEMO_CHECK), Read",
		values:     map[string]string{},
	})

	// unset-install-reference-dropped
	tests = append(tests, &testCase{
		_id:       "unset-install-reference-dropped",
		_expected: " Bash($SKILL_DEMO_CHECK), Bash(awk *)",

		deferred:   deferred,
		deployable: nil,
		isGated:    true,
		value:      " Bash($SKILL_DEMO_LANES lanes *), Bash($SKILL_DEMO_CHECK)",
		values:     map[string]string{},
	})

	// awk-grant-not-doubled
	tests = append(tests, &testCase{
		_id:       "awk-grant-not-doubled",
		_expected: " Bash(awk *), Read",

		deferred:   deferred,
		deployable: nil,
		isGated:    true,
		value:      " Bash(awk *), Read",
		values:     map[string]string{},
	})

	// unset-reference-dropped
	tests = append(tests, &testCase{
		_id:       "unset-reference-dropped",
		_expected: " Read",

		deferred:   nil,
		deployable: nil,
		value:      " Bash($SKILL_DEMO_RULES *), Read",
		values:     map[string]string{},
	})

	// set-reference-kept-verbatim
	tests = append(tests, &testCase{
		_id:       "set-reference-kept-verbatim",
		_expected: " Bash($SKILL_DEMO_RULES *), Read",

		deferred:   nil,
		deployable: nil,
		value:      " Bash($SKILL_DEMO_RULES *), Read",
		values:     set,
	})

	// nil-values-keeps-reference
	tests = append(tests, &testCase{
		_id:       "nil-values-keeps-reference",
		_expected: " Bash($SKILL_DEMO_RULES *)",

		deferred:   nil,
		deployable: nil,
		value:      " Bash($SKILL_DEMO_RULES *)",
		values:     nil,
	})

	// undeployable-skill-dropped
	tests = append(tests, &testCase{
		_id:       "undeployable-skill-dropped",
		_expected: " Read",

		deferred:   nil,
		deployable: map[string]bool{"restack": false},
		value:      " Skill(restack), Read",
		values:     nil,
	})

	// nil-deployable-keeps-skill
	tests = append(tests, &testCase{
		_id:       "nil-deployable-keeps-skill",
		_expected: " Skill(restack), Read",

		deferred:   nil,
		deployable: nil,
		value:      " Skill(restack), Read",
		values:     nil,
	})

	// unknown-leaf-kept
	tests = append(tests, &testCase{
		_id:       "unknown-leaf-kept",
		_expected: " Skill(xlsx)",

		deferred:   nil,
		deployable: map[string]bool{"restack": false},
		value:      " Skill(xlsx)",
		values:     nil,
	})

	// plain-rule-kept
	tests = append(tests, &testCase{
		_id:       "plain-rule-kept",
		_expected: " Bash(go test *)",

		deferred:   nil,
		deployable: map[string]bool{},
		value:      " Bash(go test *)",
		values:     map[string]string{},
	})

	// trailing-comma
	tests = append(tests, &testCase{
		_id:       "trailing-comma",
		_expected: " Read, Bash(go *)",

		deferred:   nil,
		deployable: nil,
		value:      " Read, Bash(go *),",
		values:     nil,
	})

	// all-dropped
	tests = append(tests, &testCase{
		_id:       "all-dropped",
		_expected: "",

		deferred:   nil,
		deployable: map[string]bool{"restack": false},
		value:      " Bash($SKILL_DEMO_RULES *), Skill(restack)",
		values:     map[string]string{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, allowedToolsView(test.deferred, test.deployable, test.isGated, test.values, test.value))
		})
	}
}

func TestMissingRequirements(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []components.ID

		env       map[string]*Variable
		installed []components.ID
		requires  []components.ID
	}

	tests := make([]*testCase, 0)

	// required-repo-scoped-needs-skill-environment
	tests = append(tests, &testCase{
		_id:       "required-repo-scoped-needs-skill-environment",
		_expected: []components.ID{components.SkillEnvironment},

		env:       map[string]*Variable{"CHECK": {Interface: "check@1", Required: true}},
		installed: []components.ID{components.Skills},
		requires:  nil,
	})

	// required-repo-scoped-met
	tests = append(tests, &testCase{
		_id:       "required-repo-scoped-met",
		_expected: nil,

		env:       map[string]*Variable{"CHECK": {Interface: "check@1", Required: true}},
		installed: []components.ID{components.Skills, components.SkillEnvironment},
		requires:  nil,
	})

	// optional-repo-scoped-needs-nothing
	tests = append(tests, &testCase{
		_id:       "optional-repo-scoped-needs-nothing",
		_expected: nil,

		env:       map[string]*Variable{"BUILD": {Interface: "build@1"}},
		installed: []components.ID{components.Skills},
		requires:  nil,
	})

	// required-install-scoped-needs-nothing
	tests = append(tests, &testCase{
		_id:       "required-install-scoped-needs-nothing",
		_expected: nil,

		env:       map[string]*Variable{"TITLE_SET": {Interface: "title-set@1", Required: true}},
		installed: []components.ID{components.Skills},
		requires:  nil,
	})

	// nil-installed-none
	tests = append(tests, &testCase{
		_id:       "nil-installed-none",
		_expected: nil,

		installed: nil,
		requires:  []components.ID{components.Peek, components.Gate},
	})

	// all-present
	tests = append(tests, &testCase{
		_id:       "all-present",
		_expected: nil,

		installed: []components.ID{components.Gate, components.Peek},
		requires:  []components.ID{components.Peek, components.Gate},
	})

	// one-missing
	tests = append(tests, &testCase{
		_id:       "one-missing",
		_expected: []components.ID{components.Peek},

		installed: []components.ID{components.Gate},
		requires:  []components.ID{components.Peek, components.Gate},
	})

	// order-kept
	tests = append(tests, &testCase{
		_id:       "order-kept",
		_expected: []components.ID{components.Peek, components.Gate, components.Mining},

		installed: []components.ID{},
		requires:  []components.ID{components.Peek, components.Gate, components.Mining},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			skill := &Skill{Catalog: provideTestCatalog(), Env: test.env, Requires: test.requires}
			assert.Equal(t, test._expected, MissingRequirements(test.installed, skill))
		})
	}
}

func TestSchema_ComponentIds(t *testing.T) {
	match := componentIdPattern.FindStringSubmatch(schemaSource)
	require.Len(t, match, 2)

	var schemaIds []components.ID
	for part := range strings.SplitSeq(match[1], "|") {
		schemaIds = append(schemaIds, components.ID(strings.Trim(strings.TrimSpace(part), `"`)))
	}

	var catalogIds []components.ID
	for _, component := range components.Catalog() {
		catalogIds = append(catalogIds, component.ID)
	}
	assert.Equal(t, catalogIds, schemaIds, "schema-list-equals-catalog")
}

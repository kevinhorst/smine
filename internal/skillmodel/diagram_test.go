package skillmodel

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

const diagramFixtureBody = "## Reference\n" +
	"`{section: context}`\n\n" +
	"**SKILL-DEMO-TERM-001** `[def]` **Round**\n\n" +
	"A round is one review pass.\n\n" +
	"`{pred: interactive}`\\\n" +
	"**SKILL-DEMO-GLOB-001** `[stop]` **Global stop**\n\n" +
	"If interactive ⇒ STOP: pause.\n\n" +
	"**SKILL-DEMO-TPL-001** `[payload]` **Template**\n\n" +
	"Template.\n\n" +
	"```markdown\n# T\n```\n\n" +
	"## Dispatch\n\n" +
	"`{post: resolve_base}`\\\n" +
	"**SKILL-DEMO-PROC-001** `[step][gate]` **Resolve base**\n\n" +
	"RESOLVE the base.\n\n" +
	"* Gate: ACDSL-DEMO-001\n\n" +
	"`{pred: nightly}`\\\n" +
	"**SKILL-DEMO-PROC-002** `[stop]` **Base stop**\n\n" +
	"If the base will not resolve ⇒ STOP: report.\n\n" +
	"**SKILL-DEMO-PROC-003** `[step]` **Unformalized step**\n\n" +
	"REPORT the result.\n\n" +
	"## Round\n\n" +
	"**SKILL-DEMO-PROC-004** `[verify]` **Tools loaded**\n\n" +
	"Before dispatch: VERIFY tools loaded; load them.\n\n" +
	"## Nightly route\n" +
	"`{section: route, pred: nightly, between: PROC-004..end}`\n\n" +
	"**SKILL-DEMO-NIGHT-001** `[step]` **Route it**\n\n" +
	"ROUTE the run.\n"

func diagramFixture(t *testing.T) *Projection {
	t.Helper()
	return projectFixture(t, diagramFixtureBody)
}

// projectFixture imports a fixture body as skill demo and projects it.
func projectFixture(t *testing.T, body string) *Projection {
	t.Helper()
	skill := importFixture(t, body, "demo")
	// Resource cases carry Script: pointers, which the model allows only on root-only skills.
	skill.EnableDelegation = skillgraph.DelegationNever
	return Project(skill, deriveFixture(t, skill))
}

func TestProject(t *testing.T) {
	projection := diagramFixture(t)

	// Reference (context), Dispatch, Round, Nightly route, halt.
	require.Len(t, projection.States, 5)
	byId := map[string]ProjectionState{}
	for _, state := range projection.States {
		byId[state.Id] = state
	}
	assert.Equal(t, skillgraph.SectionKindContext, byId["reference"].Kind)
	assert.Equal(t, skillgraph.SectionKindPhase, byId["dispatch"].Kind)
	assert.Equal(t, skillgraph.SectionKindRoute, byId["nightly-route"].Kind)
	assert.Equal(t, "nightly", byId["nightly-route"].Pred)
	assert.Equal(t, &ProjectionSpan{First: "SKILL-DEMO-PROC-004", Last: "end"}, byId["nightly-route"].Between)

	// An untagged step projects the implied prose channel; a payload none.
	assert.Equal(t, "prose", byId["dispatch"].Entries[2].Channel)
	assert.Empty(t, byId["reference"].Entries[2].Channel)

	// The fixture names no resource; the list is empty, never nil.
	assert.Equal(t, []ProjectionResource{}, projection.Resources)

	// Every entry is a line on its state, in document order, carrying the
	// additive detail fields.
	require.Len(t, byId["dispatch"].Entries, 3)
	assert.Equal(t, "gate", byId["dispatch"].Entries[0].Channel)
	assert.Equal(t, []string{"resolve_base"}, byId["dispatch"].Entries[0].Post)
	assert.True(t, byId["dispatch"].Entries[0].Formal)
	assert.False(t, byId["dispatch"].Entries[2].Formal)
	assert.Equal(t, "RESOLVE the base.", byId["dispatch"].Entries[0].Statement)
	assert.Equal(t, "ACDSL-DEMO-001", byId["dispatch"].Entries[0].Gate)
	assert.Equal(t, "A round is one review pass.", byId["reference"].Entries[0].Statement)

	// Rows carry the authored pred only — the route conjunction stays on the
	// state (and in the model), never on the row.
	assert.Empty(t, byId["nightly-route"].Entries[0].Pred)
	assert.Equal(t, "nightly", byId["dispatch"].Entries[1].Pred)

	// The context-section stop is a halt line, not an edge; the phase stop is
	// an edge. The trailing route forks after Round and re-enters at the end:
	// Dispatch→Round, Round→split, split→route (pred), route→reentry,
	// split→reentry (otherwise — a bare bool is not exhaustive), reentry→end.
	require.Len(t, byId[haltState].Entries, 1)
	assert.Equal(t, "SKILL-DEMO-GLOB-001", byId[haltState].Entries[0].Id)
	var stopEdge ProjectionEdge
	var chain []ProjectionEdge
	for _, edge := range projection.Edges {
		if edge.Kind == KindStop {
			stopEdge = edge
			continue
		}
		chain = append(chain, edge)
	}
	assert.Equal(t, "SKILL-DEMO-PROC-002", stopEdge.Id)
	assert.Equal(t, haltState, stopEdge.To)
	expectedChain := []ProjectionEdge{
		{From: "dispatch", Kind: "flow", To: "round"},
		{From: "round", Kind: "flow", To: "fork-1:split"},
		{From: "fork-1:split", Kind: "route", Pred: "nightly", To: "nightly-route"},
		{From: "nightly-route", Kind: "reentry", To: "fork-1:reentry"},
		{From: "fork-1:split", Kind: "flow", Label: "otherwise", To: "fork-1:reentry"},
		{From: "fork-1:reentry", Kind: "flow", To: EndState},
	}
	assert.Equal(t, expectedChain, chain)
	expectedForks := []ProjectionFork{{
		Id:       "fork-1",
		After:    "round",
		Branches: []ProjectionBranch{{Pred: "nightly", Routes: []string{"nightly-route"}}},
	}}
	assert.Equal(t, expectedForks, projection.Forks)

	// JSON round-trip.
	data, err := json.Marshal(projection)
	require.NoError(t, err)
	decoded := &Projection{}
	require.NoError(t, json.Unmarshal(data, decoded))
	assert.Equal(t, projection, decoded)
}

// forkSection renders one fixture section: heading, kind line with the
// optional route clause (pred plus anchor), one step entry carrying the
// optional entry pred.
func forkSection(entryId, entryPred, kind, routeClause, title string) string {
	kindLine := "`{section: " + kind + "}`"
	if routeClause != "" {
		kindLine = "`{section: " + kind + ", pred: " + routeClause + "}`"
	}
	clause := ""
	if entryPred != "" {
		clause = "`{pred: " + entryPred + "}`\\\n"
	}
	return "## " + title + "\n" + kindLine + "\n\n" + clause + "**" + entryId + "** `[step]` **Act**\n\nACT now.\n\n"
}

func TestProjectForks(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedForks []ProjectionFork

		projection *Projection
	}

	tests := make([]*testCase, 0)

	// enum-pair-exhaustive
	tests = append(tests, &testCase{
		_id: "enum-pair-exhaustive",
		_expectedForks: []ProjectionFork{{
			Id:     "fork-1",
			After:  "analysis",
			Before: "q-a",
			Branches: []ProjectionBranch{
				{Pred: "stance = critical", Routes: []string{"critical-mode"}},
				{Pred: "stance = open", Routes: []string{"open-mode"}},
			},
			Exhaustive: true,
		}},

		projection: projectFixture(t, strings.Join([]string{
			forkSection("SKILL-DEMO-ANA-001", "", "phase", "", "Analysis"),
			forkSection("SKILL-DEMO-CRIT-001", "", "route", "stance = critical, between: ANA-001..QA-001", "Critical mode"),
			forkSection("SKILL-DEMO-OPEN-001", "", "route", "stance = open, between: ANA-001..QA-001", "Open mode"),
			forkSection("SKILL-DEMO-EVAL-001", "", "context", "", "Evaluation rules"),
			forkSection("SKILL-DEMO-QA-001", "", "phase", "", "Q&A"),
		}, "")),
	})

	// negation-pair-at-start
	tests = append(tests, &testCase{
		_id: "negation-pair-at-start",
		_expectedForks: []ProjectionFork{{
			Id:     "fork-1",
			Before: "intake",
			Branches: []ProjectionBranch{
				{Pred: "not refine_mode", Routes: []string{"create-mode"}},
				{Pred: "refine_mode", Routes: []string{"refine-mode"}},
			},
			Exhaustive: true,
		}},

		projection: projectFixture(t, strings.Join([]string{
			forkSection("SKILL-DEMO-CREATE-001", "", "route", "not refine_mode, between: start..INTAKE-001", "Create mode"),
			forkSection("SKILL-DEMO-REFINE-001", "", "route", "refine_mode, between: start..INTAKE-001", "Refine mode"),
			forkSection("SKILL-DEMO-INTAKE-001", "", "phase", "", "Intake"),
		}, "")),
	})

	// chain-declared-out-of-order — the chain step sits above its predecessor
	// in the document; anchors, not order, place it.
	tests = append(tests, &testCase{
		_id: "chain-declared-out-of-order",
		_expectedForks: []ProjectionFork{{
			Id:       "fork-1",
			Before:   "identify",
			Branches: []ProjectionBranch{{Pred: "acdsl_repo", Routes: []string{"strip", "note"}}},
		}},

		projection: projectFixture(t, strings.Join([]string{
			forkSection("SKILL-DEMO-NOTE-001", "", "route", "acdsl_repo, between: STRIP-001..IDENT-001", "Note"),
			forkSection("SKILL-DEMO-STRIP-001", "", "route", "acdsl_repo, between: start..IDENT-001", "Strip"),
			forkSection("SKILL-DEMO-IDENT-001", "", "phase", "", "Identify"),
		}, "")),
	})

	// inline-between-no-fork — anchors inside one phase are no boundary.
	tests = append(tests, &testCase{
		_id:            "inline-between-no-fork",
		_expectedForks: nil,

		projection: projectFixture(t, strings.Join([]string{
			forkSection("SKILL-DEMO-ONE-001", "", "phase", "", "One") +
				"**SKILL-DEMO-ONE-002** `[step]` **More**\n\nACT again.\n\n",
			forkSection("SKILL-DEMO-TWO-001", "", "phase", "", "Two"),
			forkSection("SKILL-DEMO-MID-001", "", "route", "deep, between: ONE-001..ONE-002", "Mid"),
		}, "")),
	})

	// same-pred-chain
	tests = append(tests, &testCase{
		_id: "same-pred-chain",
		_expectedForks: []ProjectionFork{{
			Id:       "fork-1",
			After:    "judge",
			Before:   "report",
			Branches: []ProjectionBranch{{Pred: "run_mode = bootstrap", Routes: []string{"sync", "push"}}},
		}},

		projection: projectFixture(t, strings.Join([]string{
			forkSection("SKILL-DEMO-JUDGE-001", "", "phase", "", "Judge"),
			forkSection("SKILL-DEMO-SYNC-001", "", "route", "run_mode = bootstrap, between: JUDGE-001..REPORT-001", "Sync"),
			forkSection("SKILL-DEMO-PUSH-001", "", "route", "run_mode = bootstrap, between: SYNC-001..REPORT-001", "Push"),
			forkSection("SKILL-DEMO-REPORT-001", "", "phase", "", "Report"),
		}, "")),
	})

	// enum-partial-not-exhaustive
	tests = append(tests, &testCase{
		_id: "enum-partial-not-exhaustive",
		_expectedForks: []ProjectionFork{{
			Id:       "fork-1",
			After:    "approval",
			Branches: []ProjectionBranch{{Pred: "route = handoff", Routes: []string{"handoff"}}},
		}},

		projection: projectFixture(t, strings.Join([]string{
			forkSection("SKILL-DEMO-INTAKE-001", "route = change", "phase", "", "Intake"),
			forkSection("SKILL-DEMO-APPROVE-001", "", "phase", "", "Approval"),
			forkSection("SKILL-DEMO-HAND-001", "", "route", "route = handoff, between: APPROVE-001..end", "Handoff"),
		}, "")),
	})

	// no-routes
	tests = append(tests, &testCase{
		_id:            "no-routes",
		_expectedForks: nil,

		projection: projectFixture(t, strings.Join([]string{
			forkSection("SKILL-DEMO-ONE-001", "", "phase", "", "One"),
			forkSection("SKILL-DEMO-TWO-001", "", "phase", "", "Two"),
		}, "")),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expectedForks, test.projection.Forks)
		})
	}
}

func TestProjectResources(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []ProjectionResource

		projection *Projection
	}

	// resourceStep renders one step entry carrying the given bullet lines.
	resourceStep := func(id string, bullets ...string) string {
		entry := "**" + id + "** `[step]` **Act**\n\nACT now.\n"
		if len(bullets) > 0 {
			entry += "\n* " + strings.Join(bullets, "\n* ") + "\n"
		}
		return entry + "\n"
	}
	tests := make([]*testCase, 0)

	// no-bullets
	tests = append(tests, &testCase{
		_id:       "no-bullets",
		_expected: []ProjectionResource{},

		projection: projectFixture(t, "## Work\n\n"+resourceStep("SKILL-DEMO-WORK-001")),
	})

	// shared-workflow
	tests = append(tests, &testCase{
		_id: "shared-workflow",
		_expected: []ProjectionResource{{
			Entries: []string{"SKILL-DEMO-WORK-001", "SKILL-DEMO-WORK-002"},
			Id:      "workflow:workflows/lanes.js",
			Kind:    ResourceKindWorkflow,
			Label:   "workflows/lanes.js",
		}},

		projection: projectFixture(t, "## Work\n\n"+
			resourceStep("SKILL-DEMO-WORK-002", "Script: workflows/lanes.js")+
			resourceStep("SKILL-DEMO-WORK-001", "Script: workflows/lanes.js")),
	})

	// backticked-name
	tests = append(tests, &testCase{
		_id: "backticked-name",
		_expected: []ProjectionResource{{
			Entries: []string{"SKILL-DEMO-WORK-001", "SKILL-DEMO-WORK-002"},
			Id:      "workflow:workflows/mine.js",
			Kind:    ResourceKindWorkflow,
			Label:   "workflows/mine.js",
		}},

		projection: projectFixture(t, "## Work\n\n"+
			resourceStep("SKILL-DEMO-WORK-001", "Script: `workflows/mine.js`")+
			resourceStep("SKILL-DEMO-WORK-002", "Script: workflows/mine.js")),
	})

	// script-outside-workflows
	tests = append(tests, &testCase{
		_id: "script-outside-workflows",
		_expected: []ProjectionResource{{
			Entries: []string{"SKILL-DEMO-WORK-001"},
			Id:      "script:scripts/risk.sh",
			Kind:    ResourceKindScript,
			Label:   "scripts/risk.sh",
		}},

		projection: projectFixture(t, "## Work\n\n"+resourceStep("SKILL-DEMO-WORK-001", "Script: scripts/risk.sh")),
	})

	// schema
	tests = append(tests, &testCase{
		_id: "schema",
		_expected: []ProjectionResource{{
			Entries: []string{"SKILL-DEMO-WORK-001"},
			Id:      "schema:schemas/lane.json",
			Kind:    ResourceKindSchema,
			Label:   "schemas/lane.json",
		}},

		projection: projectFixture(t, "## Work\n\n"+resourceStep("SKILL-DEMO-WORK-001", "Schema: schemas/lane.json")),
	})

	// subagent-type
	tests = append(tests, &testCase{
		_id: "subagent-type",
		_expected: []ProjectionResource{{
			Entries: []string{"SKILL-DEMO-WORK-001"},
			Id:      "subagent-type:railroad-station",
			Kind:    ResourceKindSubagentType,
			Label:   "railroad-station",
		}},

		projection: projectFixture(t, "## Work\n\n"+resourceStep("SKILL-DEMO-WORK-001", "SubagentType: railroad-station")),
	})

	// one-entry-three-kinds
	tests = append(tests, &testCase{
		_id: "one-entry-three-kinds",
		_expected: []ProjectionResource{
			{Entries: []string{"SKILL-DEMO-WORK-001"}, Id: "schema:schemas/lane.json", Kind: ResourceKindSchema, Label: "schemas/lane.json"},
			{Entries: []string{"SKILL-DEMO-WORK-001"}, Id: "subagent-type:railroad-lane", Kind: ResourceKindSubagentType, Label: "railroad-lane"},
			{Entries: []string{"SKILL-DEMO-WORK-001"}, Id: "workflow:workflows/lanes.js", Kind: ResourceKindWorkflow, Label: "workflows/lanes.js"},
		},

		projection: projectFixture(t, "## Work\n\n"+resourceStep(
			"SKILL-DEMO-WORK-001",
			"Script: workflows/lanes.js",
			"Schema: schemas/lane.json",
			"SubagentType: railroad-lane",
		)),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.projection.Resources)
		})
	}
}

func TestProject_Fields(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedParams []string
		_expectedSchema string
		_expectedScript string

		entry ProjectionEntry
	}

	// fieldsEntry projects one step whose typed fields are set after import, and returns its line.
	fieldsEntry := func(fields *skillgraph.Fields) ProjectionEntry {
		skill := importFixture(t, "## Work\n\n**SKILL-DEMO-WORK-001** `[step]` **Act**\n\nACT now.\n", "demo")
		skill.EnableDelegation = skillgraph.DelegationNever
		skill.Entries()[0].Entry.Fields = fields
		projection := Project(skill, deriveFixture(t, skill))
		for _, state := range projection.States {
			if state.Id == "work" {
				return state.Entries[0]
			}
		}
		require.FailNow(t, "fieldsEntry: no work state")
		return ProjectionEntry{}
	}
	tests := make([]*testCase, 0)

	// schema-script-from-fields
	pointerFields := &skillgraph.Fields{Schema: "schemas/lane.json", Script: "workflows/lanes.js"}
	tests = append(tests, &testCase{
		_id:             "schema-script-from-fields",
		_expectedParams: []string{"script: workflows/lanes.js", "schema: schemas/lane.json"},
		_expectedSchema: "schemas/lane.json",
		_expectedScript: "workflows/lanes.js",

		entry: fieldsEntry(pointerFields),
	})

	// params-carry-field-texts
	readsFields := &skillgraph.Fields{Reads: []string{"lane", "round"}, Timeout: 600000}
	tests = append(tests, &testCase{
		_id:             "params-carry-field-texts",
		_expectedParams: []string{"reads: lane, round", "timeout: 600000 ms"},
		_expectedSchema: "",
		_expectedScript: "",

		entry: fieldsEntry(readsFields),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expectedParams, test.entry.Params)
			assert.Equal(t, test._expectedSchema, test.entry.Schema)
			assert.Equal(t, test._expectedScript, test.entry.Script)
		})
	}
}

func TestProjectReplaces(t *testing.T) {
	body := forkSection("SKILL-DEMO-WORK-001", "", "phase", "", "Work") +
		"**SKILL-DEMO-WORK-002** `[step]` **More**\n\nACT again.\n\n" +
		forkSection("SKILL-DEMO-DONE-001", "", "phase", "", "Done") +
		forkSection("SKILL-DEMO-FAST-001", "", "route", "mode = nightly, replaces: WORK-001..WORK-002 + DONE-001", "Fast")
	projection := projectFixture(t, body)

	var route ProjectionState
	for _, state := range projection.States {
		if state.Id == "fast" {
			route = state
		}
	}
	expectedSpans := []ProjectionSpan{
		{First: "SKILL-DEMO-WORK-001", Last: "SKILL-DEMO-WORK-002"},
		{First: "SKILL-DEMO-DONE-001", Last: "SKILL-DEMO-DONE-001"},
	}
	assert.Equal(t, expectedSpans, route.Replaces)
	assert.Empty(t, projection.Forks)
	assert.True(t, projection.Inline(&route))
	assert.Contains(t, projection.Edges, ProjectionEdge{From: "work", Kind: "route", Label: "replaces", Pred: "mode = nightly", To: "fast"})
	assert.Contains(t, projection.Edges, ProjectionEdge{From: "fast", Kind: "reentry", To: "work"})
	assert.Contains(t, projection.Edges, ProjectionEdge{From: "done", Kind: "route", Label: "replaces", Pred: "mode = nightly", To: "fast"})
	assert.Contains(t, projection.Edges, ProjectionEdge{From: "work", Kind: "flow", To: "done"})
}

func TestProjectionEnums(t *testing.T) {
	projection := &Projection{States: []ProjectionState{
		{Id: "route", Pred: "mode = nightly", Entries: []ProjectionEntry{{Id: "a", Pred: "mode = interactive"}}},
		{Id: "round", Entries: []ProjectionEntry{{Id: "b", Pred: "base_resolved and lanes_returned = lanes_spawned"}}},
	}}

	enums := projection.Enums()

	assert.Equal(t, map[string][]string{"mode": {"interactive", "nightly"}}, enums)
}

func TestProjectRefBy(t *testing.T) {
	body := "## Work\n\n" +
		"**SKILL-DEMO-WORK-001** `[step]` **Target**\n\n" +
		"ACT now.\n\n" +
		"**SKILL-DEMO-WORK-002** `[step]` **First caller**\n\n" +
		"ACT per [Target](#skill-demo-work-001).\n\n" +
		"**SKILL-DEMO-WORK-003** `[step]` **Second caller**\n\n" +
		"ACT per [Target](#skill-demo-work-001).\n"
	projection := projectFixture(t, body)

	lines := projection.States[0].Entries
	require.Len(t, lines, 3)
	assert.Equal(t, []string{"SKILL-DEMO-WORK-002", "SKILL-DEMO-WORK-003"}, lines[0].RefBy)
	assert.Nil(t, lines[1].RefBy)
}

func TestMermaid(t *testing.T) {
	projection := diagramFixture(t)
	projection.Edges = append(projection.Edges, ProjectionEdge{
		From: "dispatch", Id: "SKILL-DEMO-PROC-009", Kind: KindStop,
		Label: "Budget stop", Pred: "remaining_budget < round_cost", To: haltState,
	})
	text := projection.Mermaid(false)

	assert.Contains(t, text, "stateDiagram-v2")
	assert.Contains(t, text, "state \"Nightly route {nightly}\" as nightly_route")
	// Numbered flow lines with clauses and channel; defs unnumbered.
	assert.Contains(t, text, "dispatch : 1. step Resolve base {post: resolve_base} [gate]")
	assert.Contains(t, text, "dispatch : 2. stop Base stop {pred: nightly}")
	assert.Contains(t, text, "reference : def Round")
	// Raw comparison operators in labels.
	assert.Contains(t, text, "dispatch --> halt : Budget stop {remaining_budget < round_cost}")
	// Forks render as choice junctions; terminals as [*].
	assert.Contains(t, text, "state fork_1_split <<choice>>")
	assert.Contains(t, text, "state fork_1_reentry <<choice>>")
	assert.Contains(t, text, "round --> fork_1_split")
	assert.Contains(t, text, "fork_1_split --> nightly_route : {nightly}")
	assert.Contains(t, text, "nightly_route --> fork_1_reentry")
	assert.Contains(t, text, "fork_1_split --> fork_1_reentry : otherwise")
	assert.Contains(t, text, "fork_1_reentry --> [*]")
	assert.NotContains(t, text, "dispatch --> nightly_route")

	// Phases-only view drops context states and their lines.
	trimmed := projection.Mermaid(true)
	assert.NotContains(t, trimmed, "reference")
	assert.Contains(t, trimmed, "nightly_route")
	assert.Contains(t, trimmed, "halt")
}

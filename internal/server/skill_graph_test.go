package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/sessiontitle"
	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skillimport"
	"github.com/kevinhorst/smine/internal/skillmodel"
	"github.com/kevinhorst/smine/internal/skills"
)

// demoGraphManifest is the demo skill the graph tests import: args, a
// Dispatch phase with a gate step, a stop, a hook step, an opaque spawn, a
// rule and a payload, a Round phase with a consumed-before-grant pair, and a
// replacing Nightly route whose step names a schema.
const demoGraphManifest = "---\nname: demo-skill\ndescription: d\nversion: 1.0\nformat: 2\nacdsl-context: ACTION-CONCEPT-*, RULE-SKILL-012\n---\n" +
	"# Demo\n\n" +
	"## Args\n\n" +
	"| Arg | Form | Default | Effect |\n" +
	"| :-- | :--- | :------ | :----- |\n" +
	"| `paths` | pathspec list | whole diff | narrows the scope — see [Use base](#skill-demoskill-round-001) |\n" +
	"| `nightly` | flag | absent | headless mode |\n\n" +
	"## Dispatch\n\n" +
	"**SKILL-DEMOSKILL-DEF-001** `[def]` **Leaky def**\n\n" +
	"A def pointing at [Resolve base](#skill-demoskill-proc-001) flow.\n\n" +
	"`{post: base_resolved}`\\\n" +
	"**SKILL-DEMOSKILL-PROC-001** `[step][gate]` **Resolve base**\n\n" +
	"RESOLVE the base; on failure see [Base stop](#skill-demoskill-proc-002).\n\n" +
	"* Gate: ACDSL-DEMO-001\n\n" +
	"**SKILL-DEMOSKILL-PROC-002** `[stop]` **Base stop**\n\n" +
	"If the base will not resolve ⇒ STOP: run `$SKILL_DEMOSKILL_TITLE_SET title set BASE-WAIT`, report.\n\n" +
	"`{pred: mode = nightly}`\\\n" +
	"**SKILL-DEMOSKILL-PROC-004** `[step][hook:read-gate]` **Nightly branch**\n\n" +
	"BRANCH into the nightly route.\n\n" +
	"**SKILL-DEMOSKILL-PROC-005** `[spawn][script]` **Opaque fanout**\n\n" +
	"SPAWN the worker.\n\n" +
	"* Script: workflows/lanes.js\n\n" +
	"The guard below keeps its legacy place in this scope.\n\n" +
	"**SKILL-DEMOSKILL-PROC-003** `[rule]` **Guard**\n\n" +
	"Premise pin MUST hold.\n\n" +
	"**SKILL-DEMOSKILL-TPL-001** `[payload]` **Demo template**\n\n" +
	"The template.\n\n" +
	"```markdown\ntemplate body line\n```\n\n" +
	"## Round\n\n" +
	"`{pred: base_resolved and lanes_returned = lanes_spawned}`\\\n" +
	"**SKILL-DEMOSKILL-ROUND-001** `[step]` **Use base**\n\n" +
	"USE the resolved base per [Base stop](#skill-demoskill-proc-002).\n\n" +
	"`{pred: mode = interactive}`\\\n" +
	"**SKILL-DEMOSKILL-ROUND-002** `[step]` **Interactive gate**\n\n" +
	"HOLD for the human.\n\n" +
	"`{pred: late_atom}`\\\n" +
	"**SKILL-DEMOSKILL-ROUND-003** `[step]` **Early consumer**\n\n" +
	"CONSUME too early.\n\n" +
	"`{post: late_atom}`\\\n" +
	"**SKILL-DEMOSKILL-ROUND-004** `[step]` **Late producer**\n\n" +
	"GRANT late.\n\n" +
	"## Nightly route\n" +
	"`{section: route, pred: mode = nightly, replaces: ROUND-001..ROUND-002}`\n\n" +
	"**SKILL-DEMOSKILL-NIGHT-001** `[step][schema]` **Route step**\n\n" +
	"RUN the route.\n\n" +
	"* Schema: schemas/lane.json\n"

// Notes the demo fixture carries after import.
const (
	demoEntryNote   = "entry note"
	demoRouteNote   = "route note"
	demoSkillNote   = "demo note"
	demoGraphSkill  = "demo-skill"
	demoPlainSkill  = "plain-skill"
	demoTitleSetKey = "TITLE_SET"
	demoSharedAtom  = "shared_flag"
	demoGraphChange = `[{"version": "1.0", "date": "2000-01-01", "text": "fixture"}]`
	demoTitleEntry  = "SKILL-DEMOSKILL-PROC-002"
	demoTitleState  = "BASE-WAIT"
)

// provideDemoGraphSkill writes the demo skill into <skillsRepo>/demo-skill:
// imported from demoGraphManifest, then given its model-only data — the
// skill, route and entry notes, the strict tier — and saved through the
// model. The skills root carries a shared vocabulary atom.
func provideDemoGraphSkill(t *testing.T, skillsRepo string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(skillsRepo, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillsRepo, skillgraph.FileVocabulary), []byte("atoms: {\n\t"+demoSharedAtom+": {\n\t\tsort: \"event\"\n\t}\n}\n"), 0o644))
	skillDir := filepath.Join(skillsRepo, demoGraphSkill)
	require.NoError(t, os.MkdirAll(filepath.Join(skillDir, "schemas"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "schemas", "lane.json"), []byte("{}\n"), 0o644))
	atoms := `{"schemaVersion":1,"sorts":{"mode":[{"id":"mode","kind":"enum","members":["interactive","nightly"],"source":"session | arg: nightly => nightly"}],"state":[{"id":"base_resolved"},{"id":"paths_provided","source":"arg: paths"},{"id":"late_atom"},{"id":"lanes_returned","kind":"int"},{"id":"lanes_spawned","kind":"int"}]}}`
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "atoms.json"), []byte(atoms), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(demoGraphManifest), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "changelog.json"), []byte(demoGraphChange), 0o644))
	skill, err := skillimport.Import(skillDir)
	require.NoError(t, err)
	require.NoError(t, os.Remove(filepath.Join(skillDir, "atoms.json")))
	require.NoError(t, os.Remove(filepath.Join(skillDir, "changelog.json")))
	require.NoError(t, os.Remove(filepath.Join(skillDir, "SKILL.md")))

	// The demo's Script: pointer names a Workflow stage, so the model requires root-only placement.
	skill.EnableDelegation = skillgraph.DelegationNever
	skill.Note = demoSkillNote
	for _, section := range skill.Sections {
		if section.Title == "Nightly route" {
			section.Note = demoRouteNote
		}
	}
	ref, found := skill.Entry("SKILL-DEMOSKILL-PROC-001")
	require.True(t, found)
	ref.Entry.Note = demoEntryNote
	skill.Catalog = provideSkillsCatalog(t, skillsRepo)
	skill.Enforce = provideDemoEnforce()
	skill.Env = provideDemoEnv()
	skill.Title = provideDemoTitle()
	skill.Tier = skillgraph.TierStrict
	require.NoError(t, skillgraph.Save(skillDir, skill))
	return skillDir
}

// provideDemoEnv is the demo skill's env block: the title-set program the Base
// stop sets its title state through.
func provideDemoEnv() map[string]*skillgraph.Variable {
	titleSet := &skillgraph.Variable{Interface: "title-set@1"}
	env := map[string]*skillgraph.Variable{demoTitleSetKey: titleSet}
	return env
}

// provideSkillsCatalog writes testCatalogCue as the skills root's interfaces.cue and returns it loaded.
func provideSkillsCatalog(t *testing.T, skillsRepo string) *skillgraph.Catalog {
	t.Helper()
	catalogPath := filepath.Join(skillsRepo, skillgraph.FileCatalog)
	require.NoError(t, os.WriteFile(catalogPath, []byte(testCatalogCue), 0o644))
	catalog, err := skillgraph.LoadCatalog(catalogPath)
	require.NoError(t, err)
	return catalog
}

// provideDemoTitle is the demo skill's title block: one state presented by
// the Base stop.
func provideDemoTitle() *sessiontitle.Declaration {
	state := &sessiontitle.DeclaredState{Entry: demoTitleEntry, Meaning: "base awaits the human"}
	declaration := &sessiontitle.Declaration{States: map[string]*sessiontitle.DeclaredState{demoTitleState: state}}
	return declaration
}

// provideDemoEnforce is the demo skill's enforce block: two anchors, one
// requires check, one gate rule and one judgment point.
func provideDemoEnforce() *skillgraph.Enforce {
	produced := &skillgraph.Anchor{
		Capture: []skillgraph.Capture{{Name: "value", Pointer: "/value"}},
		Input:   []skillgraph.InputMatch{{Field: "command", Pattern: "^make produce"}},
		Tool:    "^Bash$",
	}
	stage := &skillgraph.Anchor{Tool: "^Workflow$"}
	check := &skillgraph.EnforceCheck{
		After:    "produced",
		Kind:     skillgraph.CheckRequires,
		On:       "stage",
		Reason:   "stage needs a produced value",
		Recovery: "produce first",
	}
	point := &skillgraph.Judgment{
		Anchor:   "produced",
		Evidence: []string{"value"},
		Question: "Is the value plausible?",
		Send:     "check the value",
	}
	enforce := &skillgraph.Enforce{
		Anchors:  map[string]*skillgraph.Anchor{"produced": produced, "stage": stage},
		Checks:   map[string]*skillgraph.EnforceCheck{"producer_first": check},
		Gate:     &skillgraph.GateRules{PlanModeDenied: []string{"Workflow"}},
		Judgment: map[string]*skillgraph.Judgment{"value_check": point},
	}
	return enforce
}

// loadDemoGraphSkill writes the demo skill into a fresh skills root and loads
// it with the shared vocabulary merged in.
func loadDemoGraphSkill(t *testing.T) *skillgraph.Skill {
	t.Helper()
	skillDir := provideDemoGraphSkill(t, filepath.Join(t.TempDir(), "repo"))
	skill, err := skillgraph.Load(skillDir)
	require.NoError(t, err)
	return skill
}

// atomRowById returns the atoms-rail row of the atom, or the zero row.
func atomRowById(rows []graphAtomRow, id string) graphAtomRow {
	for _, row := range rows {
		if row.Id == id {
			return row
		}
	}
	return graphAtomRow{}
}

// atomRowIds returns the atom ids of the rows in rail order.
func atomRowIds(rows []graphAtomRow) []string {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.Id)
	}
	return ids
}

// providePlainSkill writes an annotation-tier skill: one step, no atoms of
// its own.
func providePlainSkill(t *testing.T, skillsRepo string) {
	t.Helper()
	manifest := "---\nname: plain-skill\ndescription: d\nversion: 1.0\nformat: 2\n---\n# Plain\n\n## Run\n\n" +
		"**SKILL-PLAINSKILL-RUN-001** `[step]` **Run**\n\nRUN it.\n"
	writeSkillModel(t, filepath.Join(skillsRepo, demoPlainSkill), manifest, demoGraphChange)
}

// requireZ3 skips a case that needs the solver when z3 is not installed.
func requireZ3(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("z3"); err != nil {
		t.Skip("z3 not installed")
	}
}

func TestHandleSkillGraph(t *testing.T) {
	dir := t.TempDir()
	skillsRepo := filepath.Join(dir, "repo")
	skillDir := provideDemoGraphSkill(t, skillsRepo)
	providePlainSkill(t, skillsRepo)

	// A deployed copy carries only the rendered SKILL.md; a home copy with a
	// model is read-only.
	skillsHome := filepath.Join(dir, "home")
	deployedDir := filepath.Join(skillsHome, "deployed-skill")
	require.NoError(t, os.MkdirAll(deployedDir, 0o755))
	deployedManifest := "---\nname: deployed-skill\ndescription: d\nversion: 1.0\n---\n\n# Deployed\n"
	require.NoError(t, os.WriteFile(filepath.Join(deployedDir, "SKILL.md"), []byte(deployedManifest), 0o644))
	homeModel, err := skillgraph.Load(skillDir)
	require.NoError(t, err)
	require.NoError(t, skillgraph.Save(provideDeployedSkill(t, skillDir, skillsHome), homeModel))
	// The home copy's model binds title-set@1, so its root carries the vocabulary and the catalog like the repo's.
	vocabulary, err := os.ReadFile(filepath.Join(skillsRepo, skillgraph.FileVocabulary))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(skillsHome, skillgraph.FileVocabulary), vocabulary, 0o644))
	provideSkillsCatalog(t, skillsHome)

	// A model carrying a comment does not load.
	invalidDir := filepath.Join(skillsRepo, "broken-skill")
	writeSkillModel(t, invalidDir, "---\nname: broken-skill\ndescription: d\nversion: 1.0\n---\n\n# Broken\n", demoGraphChange)
	model, err := os.ReadFile(skillgraph.ModelPath(invalidDir))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(skillgraph.ModelPath(invalidDir), append([]byte("// note\n"), model...), 0o644))
	server := newTestServer(t, &Options{SkillsHome: skillsHome, SkillsRepo: skillsRepo})

	get := func(t *testing.T, path string) (int, string) {
		t.Helper()
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		return response.Code, response.Body.String()
	}

	t.Run("renders-graph", func(t *testing.T) {
		code, body := get(t, "/scripts/skills/repo/demo-skill/graph")
		require.Equal(t, http.StatusOK, code)
		// Projection card for the section, entry rows; the halt state is not rendered.
		assert.Contains(t, body, `data-state-id="dispatch"`)
		assert.NotContains(t, body, `data-state-id="halt"`)
		assert.NotContains(t, body, "Global stops")
		assert.Contains(t, body, `data-entry-id="SKILL-DEMOSKILL-PROC-002"`)
		// Rules compartment precedes flow in the card, per the schema order.
		assert.Less(t, strings.Index(body, `data-entry-id="SKILL-DEMOSKILL-PROC-003"`), strings.Index(body, `data-entry-id="SKILL-DEMOSKILL-PROC-001"`))
		// Tier line in the declared word, embedded data blob (projection + payloads), no Mermaid left.
		assert.Contains(t, body, "tier: strict — ")
		assert.NotContains(t, body, "tier: model")
		assert.Contains(t, body, `id="graph-data"`)
		assert.Contains(t, body, `"payloads"`)
		assert.Contains(t, body, "template body line")
		assert.NotContains(t, body, "mermaid")
		// The replacing route is a card of its own in the spine, ahead of the
		// first entry it replaces: that entry opens Round, so the route stands
		// between Dispatch and Round and cuts neither — no fork, no terminals,
		// no column of its own, no marker row, no card inside a card.
		assert.NotContains(t, body, "fork-1:split")
		assert.NotContains(t, body, `data-state-id="[end]"`)
		assert.NotContains(t, body, "st-spine-row")
		assert.NotContains(t, body, "st-inline-col")
		assert.NotContains(t, body, "data-marker-of")
		assert.NotContains(t, body, `data-state-id="round~2"`)
		dispatchCard := strings.Index(body, `data-state-id="dispatch"`)
		routeCard := strings.Index(body, `data-state-id="nightly-route"`)
		roundCard := strings.Index(body, `data-state-id="round"`)
		firstReplaced := strings.Index(body, `data-entry-id="SKILL-DEMOSKILL-ROUND-001"`)
		assert.Greater(t, dispatchCard, -1)
		assert.Greater(t, routeCard, dispatchCard)
		assert.Greater(t, roundCard, routeCard)
		assert.Greater(t, firstReplaced, roundCard)
		assert.Equal(t, 1, strings.Count(body, `data-state-id="nightly-route"`))
		// The blob names the entry the chain re-enters at behind the route.
		assert.Contains(t, body, `"reentries":{"nightly-route":"SKILL-DEMOSKILL-ROUND-003"}`)
		// No lens; the formal layer starts off and the canvas hides the clauses.
		assert.NotContains(t, body, `id="st-lens"`)
		assert.NotContains(t, body, "st-replaced")
		assert.Contains(t, body, `<input type="checkbox" id="st-formal">`)
		assert.NotContains(t, body, `id="st-formal" checked`)
		assert.Contains(t, body, `class="st-canvas st-hide-formal"`)
		// The default scenario (mode interactive) leaves the route inactive and
		// its replaced rows ungrayed.
		assert.Contains(t, body, `st-kind-route st-inactive" data-state-id="nightly-route"`)
		assert.Contains(t, body, `data-state-id="nightly-route" data-state-pred="mode = nightly" data-status="inactive"`)
		assert.Contains(t, body, `data-entry-id="SKILL-DEMOSKILL-ROUND-001"`)
		assert.NotContains(t, body, `st-inactive" data-entry-id="SKILL-DEMOSKILL-ROUND-001"`)
		assert.NotContains(t, body, `"routes":`)
		// Ref discipline: def→flow and cross-section flow refs surface as ⚠.
		assert.Contains(t, body, "references flow entry SKILL-DEMOSKILL-PROC-001 — defs and payloads are leaves")
		assert.Contains(t, body, "ref to SKILL-DEMOSKILL-PROC-002 crosses sections — sequencing belongs to the condition system")
		// The route card names its selecting tile and its replaced run with
		// the count, both ends clickable.
		assert.Contains(t, body, `set by <button type="button" class="st-subst-link" data-goto-input="arg:nightly">nightly</button>`)
		assert.Contains(t, body, `replaces 2 entries — <button type="button" class="st-subst-link" data-open-entry="SKILL-DEMOSKILL-ROUND-001">Use base</button> … <button type="button" class="st-subst-link" data-open-entry="SKILL-DEMOSKILL-ROUND-002">Interactive gate</button>`)
		// Clauses render on their own line after the badges.
		rowStart := strings.Index(body, `data-entry-id="SKILL-DEMOSKILL-ROUND-002"`)
		require.Greater(t, rowStart, -1)
		row := body[rowStart : rowStart+strings.Index(body[rowStart:], `data-entry-id="SKILL-DEMOSKILL-ROUND-003"`)]
		badges := strings.Index(row, `<span class="st-badges">`)
		assert.Greater(t, badges, -1)
		assert.Greater(t, strings.Index(row, `<span class="st-clauses st-formal-chip">`), badges)
		assert.Contains(t, row, `<span class="st-clauses st-formal-chip"><span class="st-chip st-pred" data-pred="mode = interactive">when {mode = interactive}</span></span>`)
		// Gate/hook/script render as name-carrying tags.
		assert.Contains(t, body, `class="st-tag st-tag-gate" title="gate: ACDSL-DEMO-001">ACDSL-DEMO-001</span>`)
		assert.Contains(t, body, `class="st-tag st-tag-hook" title="hook: read-gate">read-gate</span>`)
		assert.Contains(t, body, `class="st-tag st-tag-script" title="script: workflows/lanes.js">lanes.js</span>`)
		// Consumed-before-grant draws the order warning on the consumer.
		assert.Contains(t, body, "late_atom consumed before its grant — SKILL-DEMOSKILL-ROUND-004 fires later in the flow")
		assert.Contains(t, body, `<span class="st-seq st-seq-stop">`)
		assert.Contains(t, body, `class="st-badge st-badge-refby" title="referenced by 2 entry(ies) — click the row, then referenced by ▾">↙2</span>`)
		assert.Contains(t, body, "spawn declares no substeps")
		assert.Contains(t, body, `produces <button type="button" class="st-iface-atom" data-atom="base_resolved">base_resolved</button>`)
		// Arg tiles: a linked arg toggles its atom; no args card rows remain.
		assert.Contains(t, body, `data-input="arg:nightly" data-atom="mode"`)
		assert.NotContains(t, body, "st-arg-row")
		assert.NotContains(t, body, "st-input-row")
		// Graph tab and endpoints.
		assert.Contains(t, body, `href="/scripts/skills/repo/demo-skill/graph" class="active">Graph</a>`)
		assert.Contains(t, body, `hx-get="/scripts/skills/repo/demo-skill/graph/findings"`)
		assert.Contains(t, body, `hx-get="/scripts/skills/repo/demo-skill/graph/diff"`)
		// No query box.
		assert.NotContains(t, body, "graph/query")
		assert.NotContains(t, body, "st-query")
		assert.NotContains(t, body, "st-atom-list")
		assert.NotContains(t, body, "st-analysis")
	})

	t.Run("fixture-validates", func(t *testing.T) {
		skill, err := skillgraph.Load(skillDir)
		require.NoError(t, err)
		derived, err := skillmodel.Derive(skill)
		require.NoError(t, err)
		assert.Empty(t, derived.Validate())
	})

	t.Run("atoms-rail", func(t *testing.T) {
		_, body := get(t, "/scripts/skills/repo/demo-skill/graph")
		railStart := strings.Index(body, "st-kind-atoms")
		require.Greater(t, railStart, -1)
		rail := body[railStart:]
		assert.Contains(t, rail, `data-atom="late_atom">late_atom</button>`)
		assert.Contains(t, rail, `data-atom="`+demoSharedAtom+`">`+demoSharedAtom+`</button><span class="st-chip st-shared-chip"`)
		// Usage counts: a granted atom, a sourced unread local atom with the
		// warning mark, a shared unread atom without it.
		assert.Contains(t, rail, `data-atom="base_resolved">base_resolved</button><span class="st-atom-usage">granted by 1 · read by 1</span></div>`)
		assert.Contains(t, rail, `data-atom="paths_provided">paths_provided</button><span class="st-atom-usage">source: arg: paths · read by 0</span><span class="st-badge st-badge-warn" title="no pred reads it — delete it and the posts granting it">⚠</span></div>`)
		assert.Contains(t, rail, `data-atom="`+demoSharedAtom+`">`+demoSharedAtom+`</button><span class="st-chip st-shared-chip" title="shared vocabulary — skills/vocabulary.cue">shared</span><span class="st-atom-usage">granted by 0 · read by 0</span></div>`)
		assert.NotContains(t, rail, `data-edit-atom="base_resolved"`)
		assert.NotContains(t, rail, "st-obs-chip")
	})

	t.Run("resources-rail", func(t *testing.T) {
		_, body := get(t, "/scripts/skills/repo/demo-skill/graph")
		skill, err := skillgraph.Load(skillDir)
		require.NoError(t, err)
		derived, err := skillmodel.Derive(skill)
		require.NoError(t, err)
		resources := skillmodel.Project(skill, derived).Resources
		require.NotEmpty(t, resources)
		for _, resource := range resources {
			want := `data-resource-id="` + resource.Id + `" data-ref-entries="` + strings.Join(resource.Entries, ",") + `" data-edge-kind="` + resourceEdgeKind(resource.Kind) + `"`
			assert.Contains(t, body, want)
		}
		// The card is titled Resources and no longer carries the context
		// declaration.
		cardStart := strings.Index(body, "st-kind-refs")
		require.Greater(t, cardStart, -1)
		card := body[cardStart : cardStart+strings.Index(body[cardStart:], "</details>")]
		assert.Contains(t, card, `<span class="st-card-title">Resources</span>`)
		assert.NotContains(t, card, `<div class="st-compartment-label">context</div>`)
		assert.NotContains(t, card, "RULE-SKILL-012")
		assert.NotContains(t, card, "ACTION-CONCEPT-*")
	})

	t.Run("top-band", func(t *testing.T) {
		_, body := get(t, "/scripts/skills/repo/demo-skill/graph")
		// The grid holds the top band with the Context entries card alone,
		// the invocation column, then the spine.
		grid := strings.Index(body, `class="st-grid"`)
		topBand := strings.Index(body, `<div class="st-top-band">`)
		cardStart := strings.Index(body, "st-kind-context-entries")
		invocation := strings.Index(body, `id="st-invocation"`)
		spine := strings.Index(body, `class="st-spine"`)
		require.Greater(t, grid, -1)
		assert.Greater(t, topBand, grid)
		assert.Greater(t, cardStart, topBand)
		assert.Greater(t, invocation, cardStart)
		assert.Greater(t, spine, invocation)
		assert.NotContains(t, body, "st-rail-title")
		card := body[cardStart : cardStart+strings.Index(body[cardStart:], "</details>")]
		assert.Contains(t, card, `<a class="st-chip st-ref-item" href="/context/entry/RULE-SKILL-012">RULE-SKILL-012</a>`)
		assert.Contains(t, card, `<span class="st-chip st-ref-item st-ref-glob" title="glob — resolved at invocation against the context index">ACTION-CONCEPT-*</span>`)
		assert.Less(t, strings.Index(card, "ACTION-CONCEPT-*"), strings.Index(card, "RULE-SKILL-012"))
		// The rail keeps Enforcement ahead of the atoms card.
		enforceCard := strings.Index(body, "st-kind-enforce")
		assert.Greater(t, enforceCard, grid)
		assert.Less(t, enforceCard, strings.Index(body, "st-kind-atoms"))
	})

	t.Run("enforce-card", func(t *testing.T) {
		_, body := get(t, "/scripts/skills/repo/demo-skill/graph")
		cardStart := strings.Index(body, "st-kind-enforce")
		require.Greater(t, cardStart, -1)
		card := body[cardStart : cardStart+strings.Index(body[cardStart:], "</details>")]
		assert.Contains(t, card, `data-enforce-anchor="produced" data-used-by="check:producer_first,judgment:value_check"`)
		assert.Contains(t, card, `data-enforce-anchor="stage" data-used-by="check:producer_first"`)
		assert.Contains(t, card, `data-enforce-ref="stage">on stage</button>`)
		assert.Contains(t, card, `data-enforce-ref="produced">after produced</button>`)
		// Gate rules read as sentences, checks lead with their reason, the
		// anchors sit last behind a closed disclosure with a tool label.
		assert.Contains(t, card, `<span class="st-enf-sentence" title="planModeDenied">denied in plan mode</span><code class="st-enf-code">Workflow</code>`)
		assert.Contains(t, card, `<span class="st-entry-title">stage needs a produced value</span>`)
		assert.Contains(t, card, `<span class="st-entry-title">Is the value plausible?</span>`)
		assert.Contains(t, card, `title="^Bash$">Bash</span>`)
		checks := strings.Index(card, `data-enforce-id="check:producer_first"`)
		anchors := strings.Index(card, `<details class="st-compartment st-disclosure">`)
		assert.Greater(t, checks, -1)
		assert.Greater(t, anchors, checks)
		assert.Greater(t, strings.Index(card, `data-enforce-anchor="produced"`), anchors)
	})

	t.Run("enforce-absent", func(t *testing.T) {
		code, body := get(t, "/scripts/skills/repo/plain-skill/graph")
		require.Equal(t, http.StatusOK, code)
		assert.NotContains(t, body, "st-kind-enforce")
	})

	t.Run("title-card", func(t *testing.T) {
		_, body := get(t, "/scripts/skills/repo/demo-skill/graph")
		cardStart := strings.Index(body, "st-kind-title")
		require.Greater(t, cardStart, -1)
		card := body[cardStart : cardStart+strings.Index(body[cardStart:], "</details>")]
		assert.Contains(t, body, `<details class="st-card st-kind-title" data-state-id="title-states" open>`)
		assert.Contains(t, card, `<span class="st-counts">1 states</span>`)
		assert.Contains(t, card, `<span class="st-chip st-title-state">`+demoTitleState+`</span>`)
		assert.Contains(t, card, `<button type="button" class="st-subst-link" data-open-entry="`+demoTitleEntry+`">Base stop</button>`)
		assert.Contains(t, card, `<span class="st-entry-title">base awaits the human</span>`)
		assert.Contains(t, card, `<code class="st-enf-code st-enf-line">smine-rules title set `+demoTitleState+`</code>`)
		assert.NotContains(t, card, "leave")
		// The card leads the context rail, ahead of the Enforcement card.
		assert.Less(t, strings.Index(body, "st-rail-context"), cardStart)
		assert.Less(t, cardStart, strings.Index(body, "st-kind-enforce"))
	})

	t.Run("title-entry-chip", func(t *testing.T) {
		_, body := get(t, "/scripts/skills/repo/demo-skill/graph")
		rowStart := strings.Index(body, `data-entry-id="`+demoTitleEntry+`"`)
		require.Greater(t, rowStart, -1)
		row := body[rowStart : rowStart+strings.Index(body[rowStart:], `<span class="st-badges">`)]
		assert.Contains(t, row, `<span class="st-chip st-title-state" title="session title while this gate waits">[`+demoTitleState+`]</span>`)
		assert.Equal(t, 1, strings.Count(body, `title="session title while this gate waits"`))
	})

	t.Run("arg-owners-env-tiles-badge-slots", func(t *testing.T) {
		_, body := get(t, "/scripts/skills/repo/demo-skill/graph")
		// The Effect cell's entry link is the arg's owner; an arg without one warns on its tile and in the panel.
		assert.Contains(t, body, `"name":"paths","owners":["SKILL-DEMOSKILL-ROUND-001"]`)
		assert.Contains(t, body, `{"kind":"arg","message":"`+warningNoOwner+`","target":"nightly","title":"nightly"}`)
		nightlyTile := strings.Index(body, `data-arg="nightly"`)
		require.Greater(t, nightlyTile, -1)
		assert.Contains(t, body[nightlyTile:nightlyTile+strings.Index(body[nightlyTile:], `data-arg-detail`)], `st-badge-warn`)
		// The env group stands above the args in the left column, its tile and blob row carry the reader.
		envTile := strings.Index(body, `data-env="SKILL_DEMOSKILL_TITLE_SET"`)
		assert.Greater(t, envTile, strings.Index(body, `id="st-invocation"`))
		assert.Less(t, envTile, strings.Index(body, `data-arg="paths"`))
		assert.Contains(t, body, `"key":"TITLE_SET"`)
		assert.Contains(t, body, `"readers":[{"id":"SKILL-DEMOSKILL-PROC-002","kind":"entry","label":"Base stop"}]`)
		// Badges sit in three slots: counts, tags, marks.
		assert.Contains(t, body, `<span class="st-badge-counts"><span class="st-badge st-badge-refby"`)
		assert.Contains(t, body, `<span class="st-badge-tags"><span class="st-tag st-tag-hook" title="hook: read-gate">read-gate</span></span>`)
		assert.Contains(t, body, `<span class="st-badge-marks"><span class="st-badge st-badge-warn"`)
	})

	t.Run("title-absent", func(t *testing.T) {
		code, body := get(t, "/scripts/skills/repo/plain-skill/graph")
		require.Equal(t, http.StatusOK, code)
		assert.NotContains(t, body, "st-kind-title")
		assert.NotContains(t, body, "st-title-state")
	})

	t.Run("notes", func(t *testing.T) {
		_, body := get(t, "/scripts/skills/repo/demo-skill/graph")
		assert.Contains(t, body, `<span id="st-skill-note-text">`+demoSkillNote+`</span>`)
		assert.Contains(t, body, `"entryNotes":{"SKILL-DEMOSKILL-PROC-001":"`+demoEntryNote+`"}`)
		// A section's prose is no note: only the declared route note is carried.
		assert.Contains(t, body, `"sectionNotes":{"nightly-route":"`+demoRouteNote+`"}`)
	})

	t.Run("stage-and-warnings", func(t *testing.T) {
		_, body := get(t, "/scripts/skills/repo/demo-skill/graph")
		// The stage wraps the viewport and both panels; the warnings panel
		// precedes the entry panel, the button sits in the toolbar.
		stage := strings.Index(body, `<div class="st-stage">`)
		viewport := strings.Index(body, `id="st-viewport"`)
		warningsPanel := strings.Index(body, `<aside class="st-panel st-panel-warnings" id="st-warnings" hidden>`)
		entryPanel := strings.Index(body, `<aside class="st-panel" id="st-panel" hidden>`)
		require.Greater(t, stage, -1)
		assert.Greater(t, viewport, stage)
		assert.Greater(t, warningsPanel, viewport)
		assert.Greater(t, entryPanel, warningsPanel)
		assert.Less(t, strings.Index(body, `id="st-warnings-toggle"`), stage)
		// The blob carries the structural warnings with their targets.
		assert.Contains(t, body, `{"kind":"entry","message":"spawn declares no substeps — the spawned work is opaque in this skill","target":"SKILL-DEMOSKILL-PROC-005","title":"Opaque fanout"}`)
		assert.Contains(t, body, `"target":"SKILL-DEMOSKILL-DEF-001","title":"Leaky def"}`)
		assert.Contains(t, body, `"target":"SKILL-DEMOSKILL-ROUND-003","title":"Early consumer"}`)
	})

	t.Run("warnings-absent", func(t *testing.T) {
		_, body := get(t, "/scripts/skills/repo/plain-skill/graph")
		assert.NotContains(t, body, `"warnings":`)
		assert.Contains(t, body, `id="st-warnings-toggle"`)
	})

	t.Run("no-editor", func(t *testing.T) {
		for _, path := range []string{"/scripts/skills/repo/demo-skill/graph", "/scripts/skills/home/demo-skill/graph"} {
			code, body := get(t, path)
			require.Equal(t, http.StatusOK, code)
			assert.Contains(t, body, `id="graph-data"`)
			assert.NotContains(t, body, `data-st="add-entry"`)
			assert.NotContains(t, body, "st-editable")
			assert.NotContains(t, body, `"fingerprint"`)
			assert.NotContains(t, body, "data-edit-section")
			assert.NotContains(t, body, "st-preview")
		}
	})

	t.Run("no-model-deployed-copy", func(t *testing.T) {
		code, body := get(t, "/scripts/skills/home/deployed-skill/graph")
		require.Equal(t, http.StatusOK, code)
		assert.Contains(t, body, noModelDeployed)
		assert.Contains(t, body, `href="/scripts/skills/repo/deployed-skill/graph"`)
		assert.NotContains(t, body, `id="graph-data"`)
	})

	t.Run("invalid-model", func(t *testing.T) {
		code, body := get(t, "/scripts/skills/repo/broken-skill/graph")
		require.Equal(t, http.StatusOK, code)
		assert.Contains(t, body, "The skill model does not load")
		assert.Contains(t, body, "comment")
		assert.NotContains(t, body, `id="graph-data"`)
	})

	t.Run("unknown-skill-404", func(t *testing.T) {
		code, _ := get(t, "/scripts/skills/repo/nope/graph")
		assert.Equal(t, http.StatusNotFound, code)
	})

	t.Run("old-structure-route-404", func(t *testing.T) {
		code, _ := get(t, "/scripts/skills/repo/demo-skill/structure")
		assert.Equal(t, http.StatusNotFound, code)
	})

	t.Run("skills-tab-first-skill", func(t *testing.T) {
		code, body := get(t, "/scripts/skills/graph")
		require.Equal(t, http.StatusOK, code)
		// The Skills page's tabs with Graph active, the picker over every
		// listed skill, the first one selected — here the broken model.
		assert.Contains(t, body, `<a href="/scripts/skills" >Skills</a>`)
		assert.Contains(t, body, `<a href="/scripts/skills/graph" class="active">Graph</a>`)
		assert.Contains(t, body, `<option value="repo/broken-skill" selected>/broken-skill</option>`)
		assert.Contains(t, body, `<option value="repo/demo-skill">/demo-skill</option>`)
		assert.Contains(t, body, `<option value="home/deployed-skill">/deployed-skill</option>`)
		assert.Contains(t, body, `<option value="repo/plain-skill">/plain-skill</option>`)
		assert.Equal(t, 4, strings.Count(body, "<option "))
		assert.Contains(t, body, "The skill model does not load")
		// No per-skill head: its tabs belong to the skill's own pages.
		assert.NotContains(t, body, `/scripts/skills/repo/broken-skill/tests`)
	})

	t.Run("skills-tab-picked-skill", func(t *testing.T) {
		code, body := get(t, "/scripts/skills/graph?skill=repo/demo-skill")
		require.Equal(t, http.StatusOK, code)
		assert.Contains(t, body, `<option value="repo/demo-skill" selected>/demo-skill</option>`)
		assert.Contains(t, body, `<a class="meta" href="/scripts/skills/repo/demo-skill">open /demo-skill</a>`)
		assert.Contains(t, body, `data-state-id="dispatch"`)
		assert.Contains(t, body, `hx-get="/scripts/skills/repo/demo-skill/graph/findings"`)
		assert.NotContains(t, body, `/scripts/skills/repo/demo-skill/tests`)
	})

	t.Run("skills-tab-unknown-skill-404", func(t *testing.T) {
		code, _ := get(t, "/scripts/skills/graph?skill=repo/nope")
		assert.Equal(t, http.StatusNotFound, code)
	})

	t.Run("skill-tab-has-no-picker", func(t *testing.T) {
		_, body := get(t, "/scripts/skills/repo/demo-skill/graph")
		assert.NotContains(t, body, "scope-select")
		assert.Contains(t, body, `/scripts/skills/repo/demo-skill/tests`)
	})
}

// spineState is one projection state of the spine fixtures, every entry a step.
func spineState(id, kind string, entries ...string) skillmodel.ProjectionState {
	state := skillmodel.ProjectionState{Id: id, Kind: kind, Title: id}
	for _, entry := range entries {
		state.Entries = append(state.Entries, skillmodel.ProjectionEntry{Id: entry, Role: roleStep, Title: entry})
	}
	return state
}

// spineViews builds the plain card of every state: one flow compartment with
// a row per entry.
func spineViews(projection *skillmodel.Projection) map[string]structureStateView {
	views := map[string]structureStateView{}
	for _, state := range projection.States {
		rows := make([]structureEntryRow, 0, len(state.Entries))
		for _, entry := range state.Entries {
			rows = append(rows, structureEntryRow{Id: entry.Id})
		}
		compartments := []structureCompartment{{Flow: true, Label: slotFlow, Rows: rows}}
		views[state.Id] = structureStateView{Id: state.Id, Compartments: compartments, Kind: state.Kind}
	}
	return views
}

// spineOutline writes the spine's cards as `id[rows…]`, in order.
func spineOutline(cards []structureStateView) string {
	parts := make([]string, 0, len(cards))
	for _, card := range cards {
		var rows []string
		for _, compartment := range card.Compartments {
			for _, row := range compartment.Rows {
				rows = append(rows, row.Id)
			}
		}
		parts = append(parts, card.Id+"["+strings.Join(rows, " ")+"]")
	}
	return strings.Join(parts, " ")
}

func TestBuildSpine(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		derived    *skillmodel.Model
		projection *skillmodel.Projection
	}

	phase := spineState("a", skillgraph.SectionKindPhase, "E-1", "E-2")
	second := spineState("b", skillgraph.SectionKindPhase, "E-3")
	route := spineState("r", skillgraph.SectionKindRoute, "R-1")
	other := spineState("s", skillgraph.SectionKindRoute, "S-1")
	long := spineState("r", skillgraph.SectionKindRoute, "R-1", "R-2")
	triple := spineState("c", skillgraph.SectionKindPhase, "C-1", "C-2", "C-3")
	withBetween := func(state skillmodel.ProjectionState, first, last string) skillmodel.ProjectionState {
		state.Between = &skillmodel.ProjectionSpan{First: first, Last: last}
		return state
	}

	tests := make([]*testCase, 0)

	// no-routes
	tests = append(tests, &testCase{
		_id:       "no-routes",
		_expected: "a[E-1 E-2] b[E-3]",

		derived:    &skillmodel.Model{},
		projection: &skillmodel.Projection{States: []skillmodel.ProjectionState{phase, second}},
	})

	// replaces-cuts-before-first-replaced
	tests = append(tests, &testCase{
		_id:       "replaces-cuts-before-first-replaced",
		_expected: "a[E-1] r[R-1] a~2[E-2] b[E-3]",

		derived:    &skillmodel.Model{Sections: []skillmodel.Section{{Id: "r", Replaces: []string{"E-2", "E-3"}}}},
		projection: &skillmodel.Projection{States: []skillmodel.ProjectionState{phase, second, route}},
	})

	// between-cuts-after-split
	tests = append(tests, &testCase{
		_id:       "between-cuts-after-split",
		_expected: "a[E-1] r[R-1] a~2[E-2] b[E-3]",

		derived:    &skillmodel.Model{},
		projection: &skillmodel.Projection{States: []skillmodel.ProjectionState{phase, second, withBetween(route, "E-1", "E-2")}},
	})

	// between-on-phase-boundary-cuts-no-card
	tests = append(tests, &testCase{
		_id:       "between-on-phase-boundary-cuts-no-card",
		_expected: "a[E-1 E-2] r[R-1] b[E-3]",

		derived:    &skillmodel.Model{},
		projection: &skillmodel.Projection{States: []skillmodel.ProjectionState{phase, second, withBetween(route, "E-2", "E-3")}},
	})

	// between-from-start-cuts-before-reentry
	tests = append(tests, &testCase{
		_id:       "between-from-start-cuts-before-reentry",
		_expected: "a[E-1] r[R-1] a~2[E-2] b[E-3]",

		derived:    &skillmodel.Model{},
		projection: &skillmodel.Projection{States: []skillmodel.ProjectionState{phase, second, withBetween(route, "start", "E-2")}},
	})

	// between-start-to-end-precedes-the-chain
	tests = append(tests, &testCase{
		_id:       "between-start-to-end-precedes-the-chain",
		_expected: "r[R-1] a[E-1 E-2] b[E-3]",

		derived:    &skillmodel.Model{},
		projection: &skillmodel.Projection{States: []skillmodel.ProjectionState{phase, second, withBetween(route, "start", "end")}},
	})

	// alternatives-keep-section-order
	tests = append(tests, &testCase{
		_id:       "alternatives-keep-section-order",
		_expected: "a[E-1] r[R-1] s[S-1] a~2[E-2] b[E-3]",

		derived:    &skillmodel.Model{},
		projection: &skillmodel.Projection{States: []skillmodel.ProjectionState{phase, second, withBetween(route, "E-1", "E-2"), withBetween(other, "E-1", "E-2")}},
	})

	// route-anchored-in-a-route-cuts-that-route
	tests = append(tests, &testCase{
		_id:       "route-anchored-in-a-route-cuts-that-route",
		_expected: "a[E-1] r[R-1] s[S-1] r~2[R-2] a~2[E-2] b[E-3]",

		derived:    &skillmodel.Model{},
		projection: &skillmodel.Projection{States: []skillmodel.ProjectionState{phase, second, withBetween(long, "E-1", "E-2"), withBetween(other, "R-1", "R-2")}},
	})

	// three-parts-number-their-continuations
	tests = append(tests, &testCase{
		_id:       "three-parts-number-their-continuations",
		_expected: "c[C-1] r[R-1] c~2[C-2] s[S-1] c~3[C-3]",

		derived:    &skillmodel.Model{},
		projection: &skillmodel.Projection{States: []skillmodel.ProjectionState{triple, withBetween(route, "C-1", "C-2"), withBetween(other, "C-2", "C-3")}},
	})

	// unanchored-route-follows-the-phases
	tests = append(tests, &testCase{
		_id:       "unanchored-route-follows-the-phases",
		_expected: "a[E-1 E-2] b[E-3] r[R-1]",

		derived:    &skillmodel.Model{Sections: []skillmodel.Section{{Id: "r", Replaces: []string{"X-9"}}}},
		projection: &skillmodel.Projection{States: []skillmodel.ProjectionState{phase, second, route}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			nester := newRouteNester(test.derived, test.projection, spineViews(test.projection))
			assert.Equal(t, test._expected, spineOutline(buildSpine(nester, test.projection)))
		})
	}
}

func TestRouteNester_cut(t *testing.T) {
	// A route behind E-1 cuts the card where it stands: the rules and the
	// flow rows ahead of it stay, the rest of the flow and the payloads
	// continue in a card that carries the section's kind and status only.
	view := structureStateView{
		Id: "a",
		Compartments: []structureCompartment{
			{Label: slotRules, Rows: []structureEntryRow{{Id: "RU-1"}}},
			{Flow: true, Label: slotFlow, Rows: []structureEntryRow{{Id: "E-1"}, {Id: "E-2"}}},
			{Label: slotPayloads, Rows: []structureEntryRow{{Id: "P-1"}}},
		},
		Kind:       skillgraph.SectionKindPhase,
		RoleCounts: "1 rule · 2 step · 1 payload",
		Status:     "active",
		Title:      "A",
		Warnings:   []string{"phase grants nothing a later phase consumes"},
	}
	route := structureStateView{
		Id:           "r",
		Compartments: []structureCompartment{{Flow: true, Label: slotFlow, Rows: []structureEntryRow{{Id: "R-1"}}}},
		Kind:         skillgraph.SectionKindRoute,
	}
	nester := &routeNester{
		after:  map[string][]string{"E-1": {"r"}},
		before: map[string][]string{},
		placed: map[string]bool{},
		views:  map[string]structureStateView{"a": view, "r": route},
	}
	first := view
	first.Compartments = []structureCompartment{
		{Label: slotRules, Rows: []structureEntryRow{{Id: "RU-1"}}},
		{Flow: true, Label: slotFlow, Rows: []structureEntryRow{{Id: "E-1"}}},
	}
	continuation := structureStateView{
		Id: "a~2",
		Compartments: []structureCompartment{
			{Flow: true, Label: slotFlow, Rows: []structureEntryRow{{Id: "E-2"}}},
			{Label: slotPayloads, Rows: []structureEntryRow{{Id: "P-1"}}},
		},
		Kind:   skillgraph.SectionKindPhase,
		Status: "active",
		Title:  "A (continued)",
	}

	assert.Equal(t, []structureStateView{first, route, continuation}, nester.cut(view))
	assert.True(t, nester.placed["r"])
}

func TestRouteReentries(t *testing.T) {
	type testCase struct {
		_id       string
		_expected map[string]string

		derived    *skillmodel.Model
		projection *skillmodel.Projection
	}

	phase := spineState("a", skillgraph.SectionKindPhase, "E-1", "E-2")
	second := spineState("b", skillgraph.SectionKindPhase, "E-3")
	route := spineState("r", skillgraph.SectionKindRoute, "R-1")
	between := func(first, last string) skillmodel.ProjectionState {
		state := route
		state.Between = &skillmodel.ProjectionSpan{First: first, Last: last}
		return state
	}

	tests := make([]*testCase, 0)

	// substitution-reenters-behind-its-last-replaced-entry
	tests = append(tests, &testCase{
		_id:       "substitution-reenters-behind-its-last-replaced-entry",
		_expected: map[string]string{"r": "E-3"},

		derived:    &skillmodel.Model{Sections: []skillmodel.Section{{Id: "r", Replaces: []string{"E-1", "E-2"}}}},
		projection: &skillmodel.Projection{States: []skillmodel.ProjectionState{phase, second, route}},
	})

	// substitution-through-the-end-has-none
	tests = append(tests, &testCase{
		_id:       "substitution-through-the-end-has-none",
		_expected: map[string]string{},

		derived:    &skillmodel.Model{Sections: []skillmodel.Section{{Id: "r", Replaces: []string{"E-3"}}}},
		projection: &skillmodel.Projection{States: []skillmodel.ProjectionState{phase, second, route}},
	})

	// insertion-reenters-at-its-reentry-anchor
	tests = append(tests, &testCase{
		_id:       "insertion-reenters-at-its-reentry-anchor",
		_expected: map[string]string{"r": "E-2"},

		derived:    &skillmodel.Model{},
		projection: &skillmodel.Projection{States: []skillmodel.ProjectionState{phase, second, between("E-1", "E-2")}},
	})

	// insertion-to-the-end-has-none
	tests = append(tests, &testCase{
		_id:       "insertion-to-the-end-has-none",
		_expected: map[string]string{},

		derived:    &skillmodel.Model{},
		projection: &skillmodel.Projection{States: []skillmodel.ProjectionState{phase, second, between("E-3", "end")}},
	})

	// route-without-anchor-and-phases-have-none
	tests = append(tests, &testCase{
		_id:       "route-without-anchor-and-phases-have-none",
		_expected: map[string]string{},

		derived:    &skillmodel.Model{},
		projection: &skillmodel.Projection{States: []skillmodel.ProjectionState{phase, second, route}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, routeReentries(test.derived, test.projection))
		})
	}
}

func TestHandleSkillsGraphWithoutSkills(t *testing.T) {
	dir := t.TempDir()
	server := newTestServer(t, &Options{SkillsHome: filepath.Join(dir, "home"), SkillsRepo: filepath.Join(dir, "repo")})
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/scripts/skills/graph", nil))

	assert.Equal(t, http.StatusSeeOther, response.Code)
	assert.Equal(t, "/scripts/skills", response.Header().Get("Location"))
}

func TestAtomRows(t *testing.T) {
	type testCase struct {
		_id          string
		_expectedIds []string
		_expectedRow graphAtomRow

		atomId string
		skill  *skillgraph.Skill
	}

	demoSkill := loadDemoGraphSkill(t)
	demoIds := []string{"base_resolved", "lanes_returned", "lanes_spawned", "late_atom", "mode", "paths_provided", demoSharedAtom}

	tests := make([]*testCase, 0)

	// local-before-shared
	tests = append(tests, &testCase{
		_id:          "local-before-shared",
		_expectedIds: demoIds,
		_expectedRow: graphAtomRow{Id: "lanes_returned", GrantedBy: 0, IsShared: false, IsUnread: false, Kind: skillgraph.KindAtomInt, ReadBy: 1, Sort: "state", Source: ""},

		atomId: "lanes_returned",
		skill:  demoSkill,
	})

	// grant-count
	tests = append(tests, &testCase{
		_id:          "grant-count",
		_expectedIds: demoIds,
		_expectedRow: graphAtomRow{Id: "base_resolved", GrantedBy: 1, IsShared: false, IsUnread: false, Kind: skillgraph.KindAtomBool, ReadBy: 1, Sort: "state", Source: ""},

		atomId: "base_resolved",
		skill:  demoSkill,
	})

	// read-count-entry-preds
	tests = append(tests, &testCase{
		_id:          "read-count-entry-preds",
		_expectedIds: demoIds,
		_expectedRow: graphAtomRow{Id: "late_atom", GrantedBy: 1, IsShared: false, IsUnread: false, Kind: skillgraph.KindAtomBool, ReadBy: 1, Sort: "state", Source: ""},

		atomId: "late_atom",
		skill:  demoSkill,
	})

	// read-count-section-pred
	tests = append(tests, &testCase{
		_id:          "read-count-section-pred",
		_expectedIds: demoIds,
		_expectedRow: graphAtomRow{Id: "mode", GrantedBy: 0, IsShared: false, IsUnread: false, Kind: skillgraph.KindAtomEnum, ReadBy: 3, Sort: "mode", Source: "session | arg: nightly => nightly"},

		atomId: "mode",
		skill:  demoSkill,
	})

	// duplicate-token-one-read
	duplicateSkill := loadDemoGraphSkill(t)
	consumer, found := duplicateSkill.Entry("SKILL-DEMOSKILL-ROUND-003")
	require.True(t, found)
	consumer.Entry.Pred = "late_atom or late_atom"
	tests = append(tests, &testCase{
		_id:          "duplicate-token-one-read",
		_expectedIds: demoIds,
		_expectedRow: graphAtomRow{Id: "late_atom", GrantedBy: 1, IsShared: false, IsUnread: false, Kind: skillgraph.KindAtomBool, ReadBy: 1, Sort: "state", Source: ""},

		atomId: "late_atom",
		skill:  duplicateSkill,
	})

	// local-unread-flagged
	tests = append(tests, &testCase{
		_id:          "local-unread-flagged",
		_expectedIds: demoIds,
		_expectedRow: graphAtomRow{Id: "paths_provided", GrantedBy: 0, IsShared: false, IsUnread: true, Kind: skillgraph.KindAtomBool, ReadBy: 0, Sort: "state", Source: "arg: paths"},

		atomId: "paths_provided",
		skill:  demoSkill,
	})

	// shared-unread-not-flagged
	tests = append(tests, &testCase{
		_id:          "shared-unread-not-flagged",
		_expectedIds: demoIds,
		_expectedRow: graphAtomRow{Id: demoSharedAtom, GrantedBy: 0, IsShared: true, IsUnread: false, Kind: skillgraph.KindAtomBool, ReadBy: 0, Sort: "event", Source: ""},

		atomId: demoSharedAtom,
		skill:  demoSkill,
	})

	// no-atoms
	tests = append(tests, &testCase{
		_id:          "no-atoms",
		_expectedIds: []string{},
		_expectedRow: graphAtomRow{},

		atomId: "",
		skill:  &skillgraph.Skill{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			rows := atomRows(test.skill)
			assert.Equal(t, test._expectedIds, atomRowIds(rows))
			assert.Equal(t, test._expectedRow, atomRowById(rows, test.atomId))
		})
	}
}

func TestEnforceView(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *graphEnforceView

		enforce *skillgraph.Enforce
	}

	tests := make([]*testCase, 0)

	// nil-block
	tests = append(tests, &testCase{
		_id:       "nil-block",
		_expected: nil,

		enforce: nil,
	})

	// anchors-sorted-with-users
	tests = append(tests, &testCase{
		_id: "anchors-sorted-with-users",
		_expected: &graphEnforceView{
			Anchors: []graphAnchorRow{
				{Id: "produced", Agent: skillgraph.AnchorAgentRoot, Captures: []string{"value = /value"}, Match: []string{"command ~ ^make produce"}, Tool: "^Bash$", ToolLabel: "Bash", UsedBy: "check:producer_first,judgment:value_check"},
				{Id: "stage", Agent: skillgraph.AnchorAgentRoot, Captures: []string{}, Match: []string{}, Tool: "^Workflow$", ToolLabel: "Workflow", UsedBy: "check:producer_first"},
			},
			Checks: []graphCheckRow{
				{Id: "producer_first", Anchors: []skillgraph.AnchorRef{{Anchor: "stage", Role: "on"}, {Anchor: "produced", Role: "after"}}, Kind: skillgraph.CheckRequires, Reason: "stage needs a produced value"},
			},
			Gate:     []graphGateRow{{Label: gateLabelPlanModeDenied, Sentence: "denied in plan mode", Values: []string{"Workflow"}}},
			Judgment: []graphJudgmentRow{{Id: "value_check", Anchor: "produced", Evidence: "value", Question: "Is the value plausible?"}},
		},

		enforce: provideDemoEnforce(),
	})

	// anchor-default-agent
	tests = append(tests, &testCase{
		_id: "anchor-default-agent",
		_expected: &graphEnforceView{
			Anchors: []graphAnchorRow{
				{Id: "any_call", Agent: skillgraph.AnchorAgentAny, Captures: []string{}, Match: []string{}, Tool: "^Read$", ToolLabel: "Read", UsedBy: ""},
				{Id: "root_call", Agent: skillgraph.AnchorAgentRoot, Captures: []string{}, Match: []string{}, Tool: "^Bash$", ToolLabel: "Bash", UsedBy: ""},
			},
			Checks:   nil,
			Gate:     nil,
			Judgment: nil,
		},

		enforce: &skillgraph.Enforce{Anchors: map[string]*skillgraph.Anchor{
			"any_call":  {Agent: skillgraph.AnchorAgentAny, Tool: "^Read$"},
			"root_call": {Tool: "^Bash$"},
		}},
	})

	// tool-labels
	tests = append(tests, &testCase{
		_id: "tool-labels",
		_expected: &graphEnforceView{
			Anchors: []graphAnchorRow{
				{Id: "bash_call", Agent: skillgraph.AnchorAgentRoot, Captures: []string{}, Match: []string{}, Tool: "^Bash$", ToolLabel: "Bash", UsedBy: ""},
				{Id: "edit_call", Agent: skillgraph.AnchorAgentRoot, Captures: []string{}, Match: []string{}, Tool: "^(Edit|Write)$", ToolLabel: "Edit or Write", UsedBy: ""},
				{Id: "rules_call", Agent: skillgraph.AnchorAgentRoot, Captures: []string{}, Match: []string{}, Tool: "^smine-rules .*$", ToolLabel: "^smine-rules .*$", UsedBy: ""},
			},
			Checks:   nil,
			Gate:     nil,
			Judgment: nil,
		},

		enforce: &skillgraph.Enforce{Anchors: map[string]*skillgraph.Anchor{
			"bash_call":  {Tool: "^Bash$"},
			"edit_call":  {Tool: "^(Edit|Write)$"},
			"rules_call": {Tool: "^smine-rules .*$"},
		}},
	})

	// anchor-match-lines
	tests = append(tests, &testCase{
		_id: "anchor-match-lines",
		_expected: &graphEnforceView{
			Anchors: []graphAnchorRow{
				{Id: "produce", Agent: skillgraph.AnchorAgentRoot, Captures: []string{}, Match: []string{"command ~ ^make produce", "description ~ produce", "not command ~ --dry-run"}, Tool: "^Bash$", ToolLabel: "Bash", UsedBy: ""},
			},
			Checks:   nil,
			Gate:     nil,
			Judgment: nil,
		},

		enforce: &skillgraph.Enforce{Anchors: map[string]*skillgraph.Anchor{
			"produce": {
				Exclude: []skillgraph.InputMatch{{Field: "command", Pattern: "--dry-run"}},
				Input:   []skillgraph.InputMatch{{Field: "command", Pattern: "^make produce"}, {Field: "description", Pattern: "produce"}},
				Tool:    "^Bash$",
			},
		}},
	})

	// anchor-captures
	tests = append(tests, &testCase{
		_id: "anchor-captures",
		_expected: &graphEnforceView{
			Anchors: []graphAnchorRow{
				{Id: "produce", Agent: skillgraph.AnchorAgentRoot, Captures: []string{"value = /value", "path = /path"}, Match: []string{}, Tool: "^Bash$", ToolLabel: "Bash", UsedBy: ""},
			},
			Checks:   nil,
			Gate:     nil,
			Judgment: nil,
		},

		enforce: &skillgraph.Enforce{Anchors: map[string]*skillgraph.Anchor{
			"produce": {
				Capture: []skillgraph.Capture{{Name: "value", Pointer: "/value"}, {Name: "path", Pointer: "/path"}},
				Tool:    "^Bash$",
			},
		}},
	})

	// check-roles
	tests = append(tests, &testCase{
		_id: "check-roles",
		_expected: &graphEnforceView{
			Anchors: []graphAnchorRow{
				{Id: "produced", Agent: skillgraph.AnchorAgentRoot, Captures: []string{}, Match: []string{}, Tool: "^Bash$", ToolLabel: "Bash", UsedBy: "check:producer_first"},
				{Id: "stage", Agent: skillgraph.AnchorAgentRoot, Captures: []string{}, Match: []string{}, Tool: "^Workflow$", ToolLabel: "Workflow", UsedBy: "check:producer_first"},
			},
			Checks: []graphCheckRow{
				{Id: "producer_first", Anchors: []skillgraph.AnchorRef{{Anchor: "stage", Role: "on"}, {Anchor: "produced", Role: "after"}}, Kind: skillgraph.CheckRequires, Reason: "r"},
			},
			Gate:     nil,
			Judgment: nil,
		},

		enforce: &skillgraph.Enforce{
			Anchors: map[string]*skillgraph.Anchor{"produced": {Tool: "^Bash$"}, "stage": {Tool: "^Workflow$"}},
			Checks: map[string]*skillgraph.EnforceCheck{
				"producer_first": {After: "produced", Kind: skillgraph.CheckRequires, On: "stage", Reason: "r", Recovery: "p"},
			},
		},
	})

	// anchor-in-two-roles-listed-once
	tests = append(tests, &testCase{
		_id: "anchor-in-two-roles-listed-once",
		_expected: &graphEnforceView{
			Anchors: []graphAnchorRow{
				{Id: "ledger", Agent: skillgraph.AnchorAgentRoot, Captures: []string{}, Match: []string{}, Tool: "^Bash$", ToolLabel: "Bash", UsedBy: "check:next_round"},
				{Id: "round", Agent: skillgraph.AnchorAgentRoot, Captures: []string{}, Match: []string{}, Tool: "^Workflow$", ToolLabel: "Workflow", UsedBy: "check:next_round"},
			},
			Checks: []graphCheckRow{
				{Id: "next_round", Anchors: []skillgraph.AnchorRef{{Anchor: "round", Role: "on"}, {Anchor: "ledger", Role: "after"}, {Anchor: "round", Role: "since"}}, Kind: skillgraph.CheckRequires, Reason: "r"},
			},
			Gate:     nil,
			Judgment: nil,
		},

		enforce: &skillgraph.Enforce{
			Anchors: map[string]*skillgraph.Anchor{"ledger": {Tool: "^Bash$"}, "round": {Tool: "^Workflow$"}},
			Checks: map[string]*skillgraph.EnforceCheck{
				"next_round": {After: "ledger", Kind: skillgraph.CheckRequires, On: "round", Reason: "r", Recovery: "p", Since: "round"},
			},
		},
	})

	// judgment-evidence-joined
	tests = append(tests, &testCase{
		_id: "judgment-evidence-joined",
		_expected: &graphEnforceView{
			Anchors: []graphAnchorRow{
				{Id: "produced", Agent: skillgraph.AnchorAgentRoot, Captures: []string{}, Match: []string{}, Tool: "^Bash$", ToolLabel: "Bash", UsedBy: "judgment:value_check"},
			},
			Checks:   nil,
			Gate:     nil,
			Judgment: []graphJudgmentRow{{Id: "value_check", Anchor: "produced", Evidence: "value, path", Question: "q"}},
		},

		enforce: &skillgraph.Enforce{
			Anchors: map[string]*skillgraph.Anchor{"produced": {Tool: "^Bash$"}},
			Judgment: map[string]*skillgraph.Judgment{
				"value_check": {Anchor: "produced", Evidence: []string{"value", "path"}, Question: "q", Send: "s"},
			},
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, enforceView(test.enforce))
		})
	}
}

func TestGateRows(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []graphGateRow

		gate *skillgraph.GateRules
	}

	tests := make([]*testCase, 0)

	// nil-gate
	tests = append(tests, &testCase{
		_id:       "nil-gate",
		_expected: nil,

		gate: nil,
	})

	// empty-lists-dropped
	tests = append(tests, &testCase{
		_id:       "empty-lists-dropped",
		_expected: []graphGateRow{{Label: gateLabelRootDenied, Sentence: "denied at the root", Values: []string{"Agent"}}},

		gate: &skillgraph.GateRules{RootDenied: []string{"Agent"}},
	})

	// writers-line
	tests = append(tests, &testCase{
		_id:       "writers-line",
		_expected: []graphGateRow{{Label: gateLabelWriters, Sentence: "written only by", Values: []string{"plans/** by planner, fixer", "out/*.md by writer"}}},

		gate: &skillgraph.GateRules{Writers: []skillgraph.Writer{
			{AgentTypes: []string{"planner", "fixer"}, Glob: "plans/**"},
			{AgentTypes: []string{"writer"}, Glob: "out/*.md"},
		}},
	})

	// label-order
	tests = append(tests, &testCase{
		_id: "label-order",
		_expected: []graphGateRow{
			{Label: gateLabelPlanModeDenied, Sentence: "denied in plan mode", Values: []string{"Workflow"}},
			{Label: gateLabelRootDenied, Sentence: "denied at the root", Values: []string{"Agent"}},
			{Label: gateLabelWorkflowOnly, Sentence: "allowed inside a Workflow only", Values: []string{"Bash(make lanes*)"}},
			{Label: gateLabelWriters, Sentence: "written only by", Values: []string{"plans/** by planner"}},
		},

		gate: &skillgraph.GateRules{
			PlanModeDenied: []string{"Workflow"},
			RootDenied:     []string{"Agent"},
			WorkflowOnly:   []string{"Bash(make lanes*)"},
			Writers:        []skillgraph.Writer{{AgentTypes: []string{"planner"}, Glob: "plans/**"}},
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, gateRows(test.gate))
		})
	}
}

func TestContextEntries(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []graphRefItem

		declaration string
	}

	tests := make([]*testCase, 0)

	// exact-id-linked
	tests = append(tests, &testCase{
		_id:       "exact-id-linked",
		_expected: []graphRefItem{{Href: "/context/entry/RULE-SKILL-012", Name: "RULE-SKILL-012"}},

		declaration: "RULE-SKILL-012",
	})

	// glob-unlinked
	tests = append(tests, &testCase{
		_id:       "glob-unlinked",
		_expected: []graphRefItem{{Href: "", Name: "ACTION-CONCEPT-*"}},

		declaration: "ACTION-CONCEPT-*",
	})

	// declaration-order-kept
	tests = append(tests, &testCase{
		_id: "declaration-order-kept",
		_expected: []graphRefItem{
			{Href: "/context/entry/RULE-SKILL-012", Name: "RULE-SKILL-012"},
			{Href: "", Name: "ACTION-CONCEPT-*"},
			{Href: "/context/entry/FACT-REPO-ARCH-001", Name: "FACT-REPO-ARCH-001"},
		},

		declaration: "RULE-SKILL-012, ACTION-CONCEPT-*, FACT-REPO-ARCH-001",
	})

	// blank-tokens-skipped
	tests = append(tests, &testCase{
		_id: "blank-tokens-skipped",
		_expected: []graphRefItem{
			{Href: "/context/entry/RULE-SKILL-012", Name: "RULE-SKILL-012"},
			{Href: "", Name: "FACT-*"},
		},

		declaration: " , RULE-SKILL-012,, ,FACT-*,",
	})

	// empty-declaration
	tests = append(tests, &testCase{
		_id:       "empty-declaration",
		_expected: nil,

		declaration: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, contextEntries(test.declaration))
		})
	}
}

func TestSectionNotes(t *testing.T) {
	type testCase struct {
		_id       string
		_expected map[string]string

		projection *skillmodel.Projection
		skill      *skillgraph.Skill
	}

	routeProjection := &skillmodel.Projection{States: []skillmodel.ProjectionState{
		{Id: "dispatch", Kind: skillgraph.SectionKindPhase, Title: "Dispatch"},
		{Id: "nightly-route", Kind: skillgraph.SectionKindRoute, Title: "Nightly route"},
	}}

	tests := make([]*testCase, 0)

	// noted-section-keyed-by-state-id
	tests = append(tests, &testCase{
		_id:       "noted-section-keyed-by-state-id",
		_expected: map[string]string{"nightly-route": demoRouteNote},

		projection: routeProjection,
		skill: &skillgraph.Skill{Sections: []*skillgraph.Section{
			{Title: "Dispatch"},
			{Title: "Nightly route", Note: demoRouteNote},
		}},
	})

	// section-without-note-absent
	tests = append(tests, &testCase{
		_id:       "section-without-note-absent",
		_expected: map[string]string{},

		projection: routeProjection,
		skill: &skillgraph.Skill{Sections: []*skillgraph.Section{
			{Title: "Dispatch"},
			{Title: "Nightly route"},
		}},
	})

	// note-beside-prose
	tests = append(tests, &testCase{
		_id:       "note-beside-prose",
		_expected: map[string]string{"nightly-route": demoRouteNote},

		projection: routeProjection,
		skill: &skillgraph.Skill{Sections: []*skillgraph.Section{
			{Title: "Dispatch"},
			{Blocks: []skillgraph.Block{{Prose: "route prose"}}, Note: demoRouteNote, Title: "Nightly route"},
		}},
	})

	// prose-is-no-note
	tests = append(tests, &testCase{
		_id:       "prose-is-no-note",
		_expected: map[string]string{},

		projection: routeProjection,
		skill: &skillgraph.Skill{Sections: []*skillgraph.Section{
			{Blocks: []skillgraph.Block{{Heading: "Sub"}, {Prose: "\ndispatch prose\n"}, {Prose: "second prose"}}, Title: "Dispatch"},
			{Blocks: []skillgraph.Block{{Prose: "route prose"}}, Title: "Nightly route"},
		}},
	})

	// heading-only-section-absent
	tests = append(tests, &testCase{
		_id:       "heading-only-section-absent",
		_expected: map[string]string{},

		projection: routeProjection,
		skill: &skillgraph.Skill{Sections: []*skillgraph.Section{
			{Blocks: []skillgraph.Block{{Heading: "Sub"}}, Title: "Dispatch"},
			{Title: "Nightly route"},
		}},
	})

	// section-without-card-absent
	tests = append(tests, &testCase{
		_id:       "section-without-card-absent",
		_expected: map[string]string{},

		projection: routeProjection,
		skill: &skillgraph.Skill{Sections: []*skillgraph.Section{
			{Title: "Args", Note: "args note"},
			{Title: "Dispatch"},
		}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, sectionNotes(test.projection, test.skill))
		})
	}
}

func TestGraphWarnings(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []graphWarning

		args       []structureArgRow
		atoms      []graphAtomRow
		notes      map[string][]string
		projection *skillmodel.Projection
		views      map[string]structureStateView
	}

	projection := &skillmodel.Projection{States: []skillmodel.ProjectionState{
		{Id: "dispatch", Entries: []skillmodel.ProjectionEntry{{Id: "E-1", Title: "First"}, {Id: "E-2", Title: "Second"}}, Kind: skillgraph.SectionKindPhase, Title: "Dispatch"},
		{Id: "defs", Entries: []skillmodel.ProjectionEntry{{Id: "E-3", Title: "Third"}}, Kind: skillgraph.SectionKindContext, Title: "Definitions"},
		{Id: "halt", Entries: []skillmodel.ProjectionEntry{{Id: "E-4", Title: "Stop"}}, Kind: "halt", Title: "Halt"},
	}}

	tests := make([]*testCase, 0)

	// nothing-flagged
	tests = append(tests, &testCase{
		_id:       "nothing-flagged",
		_expected: nil,

		args:       nil,
		atoms:      []graphAtomRow{{Id: "read_atom"}},
		notes:      map[string][]string{},
		projection: projection,
		views:      map[string]structureStateView{},
	})

	// entry-notes-in-document-order
	tests = append(tests, &testCase{
		_id: "entry-notes-in-document-order",
		_expected: []graphWarning{
			{Kind: warningKindEntry, Message: "first note", Target: "E-1", Title: "First"},
			{Kind: warningKindEntry, Message: "second note", Target: "E-1", Title: "First"},
			{Kind: warningKindEntry, Message: "context note", Target: "E-3", Title: "Third"},
		},

		args:       nil,
		atoms:      nil,
		notes:      map[string][]string{"E-3": {"context note"}, "E-1": {"first note", "second note"}},
		projection: projection,
		views:      map[string]structureStateView{},
	})

	// section-warning-before-its-entries
	tests = append(tests, &testCase{
		_id: "section-warning-before-its-entries",
		_expected: []graphWarning{
			{Kind: warningKindSection, Message: "phase grants nothing a later phase consumes", Target: "dispatch", Title: "Dispatch"},
			{Kind: warningKindEntry, Message: "entry note", Target: "E-2", Title: "Second"},
		},

		args:       nil,
		atoms:      nil,
		notes:      map[string][]string{"E-2": {"entry note"}},
		projection: projection,
		views:      map[string]structureStateView{"dispatch": {Warnings: []string{"phase grants nothing a later phase consumes"}}},
	})

	// unread-atoms-last
	tests = append(tests, &testCase{
		_id: "unread-atoms-last",
		_expected: []graphWarning{
			{Kind: warningKindEntry, Message: "entry note", Target: "E-1", Title: "First"},
			{Kind: warningKindAtom, Message: warningUnreadAtom, Target: "dead_atom", Title: "dead_atom"},
		},

		args:       nil,
		atoms:      []graphAtomRow{{Id: "dead_atom", IsUnread: true}, {Id: "read_atom"}},
		notes:      map[string][]string{"E-1": {"entry note"}},
		projection: projection,
		views:      map[string]structureStateView{},
	})

	// halt-state-skipped
	tests = append(tests, &testCase{
		_id:       "halt-state-skipped",
		_expected: nil,

		args:       nil,
		atoms:      nil,
		notes:      map[string][]string{"E-4": {"halt note"}},
		projection: projection,
		views:      map[string]structureStateView{"halt": {Warnings: []string{"never shown"}}},
	})

	// arg-without-owner-after-atoms
	tests = append(tests, &testCase{
		_id: "arg-without-owner-after-atoms",
		_expected: []graphWarning{
			{Kind: warningKindAtom, Message: warningUnreadAtom, Target: "dead_atom", Title: "dead_atom"},
			{Kind: warningKindArg, Message: warningNoOwner, Target: "nightly", Title: "nightly"},
		},

		args:       []structureArgRow{{Name: "paths", Owners: []string{"E-1"}}, {Name: "nightly"}},
		atoms:      []graphAtomRow{{Id: "dead_atom", IsUnread: true}},
		notes:      map[string][]string{},
		projection: projection,
		views:      map[string]structureStateView{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, graphWarnings(test.args, test.atoms, test.notes, test.projection, test.views))
		})
	}
}

func TestStructureArgs(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedEffect string
		_expectedOwners []string

		args []skillgraph.Arg
	}

	model := loadDemoGraphSkill(t)
	tests := make([]*testCase, 0)

	// owners-from-markdown-links
	tests = append(tests, &testCase{
		_id:             "owners-from-markdown-links",
		_expectedEffect: "sets Use base and Base stop",
		_expectedOwners: []string{"SKILL-DEMOSKILL-ROUND-001", "SKILL-DEMOSKILL-PROC-002"},

		args: []skillgraph.Arg{{Effect: "sets [Use base](#skill-demoskill-round-001) and [Base stop](#skill-demoskill-proc-002)", Name: "mode"}},
	})

	// owner-from-short-token
	tests = append(tests, &testCase{
		_id:             "owner-from-short-token",
		_expectedEffect: "binding override — Interactive gate · ROUND-002",
		_expectedOwners: []string{"SKILL-DEMOSKILL-ROUND-002"},

		args: []skillgraph.Arg{{Effect: "binding override — [[ROUND-002]]", Name: "mode"}},
	})

	// token-and-link-to-one-entry-once
	tests = append(tests, &testCase{
		_id:             "token-and-link-to-one-entry-once",
		_expectedEffect: "Resolve base · PROC-001, see Resolve base",
		_expectedOwners: []string{"SKILL-DEMOSKILL-PROC-001"},

		args: []skillgraph.Arg{{Effect: "[[SKILL-DEMOSKILL-PROC-001]], see [Resolve base](#skill-demoskill-proc-001)", Name: "mode"}},
	})

	// unknown-token-ignored
	tests = append(tests, &testCase{
		_id:             "unknown-token-ignored",
		_expectedEffect: "see NOPE-001 · NOPE-001",
		_expectedOwners: nil,

		args: []skillgraph.Arg{{Effect: "see [[NOPE-001]]", Name: "mode"}},
	})

	// unknown-anchor-ignored
	tests = append(tests, &testCase{
		_id:             "unknown-anchor-ignored",
		_expectedEffect: "see Elsewhere",
		_expectedOwners: nil,

		args: []skillgraph.Arg{{Effect: "see [Elsewhere](#skill-other-flow-001)", Name: "mode"}},
	})

	// effect-without-link
	tests = append(tests, &testCase{
		_id:             "effect-without-link",
		_expectedEffect: "headless mode",
		_expectedOwners: nil,

		args: []skillgraph.Arg{{Effect: "headless mode", Name: "nightly"}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			rows := structureArgs(test.args, model)
			assert.Equal(t, test._expectedOwners, rows[0].Owners)
			assert.Equal(t, test._expectedEffect, rows[0].Effect)
		})
	}
}

func TestGraphEnvRows(t *testing.T) {
	type testCase struct {
		_id       string
		_expected structureEnvRow

		values []skills.VariableValue
	}

	model := loadDemoGraphSkill(t)
	name := skillgraph.VariableName(demoGraphSkill, demoTitleSetKey)
	iface, isKnown := model.Catalog.Interface("title-set@1")
	require.True(t, isKnown)
	titles := entryTitles(model.Entries())
	readers := []structureReader{{Id: demoTitleEntry, Kind: skillgraph.ReadKindEntry, Label: "Base stop"}}
	base := structureEnvRow{Interface: iface.Ref(), Key: demoTitleSetKey, Name: name, Readers: readers, Scope: iface.Scope, Summary: iface.Summary}
	tests := make([]*testCase, 0)

	// set
	set := base
	set.Mark, set.Source, set.State, set.Value = "●", "provider config-server", envStateSet, "smine-rules"
	tests = append(tests, &testCase{
		_id:       "set",
		_expected: set,

		values: []skills.VariableValue{{Interface: iface, Name: name, Source: "provider config-server", Value: "smine-rules"}},
	})

	// unset-optional
	unset := base
	unset.Cause, unset.Mark, unset.Source, unset.State = "not on PATH", "○", "unset", envStateUnsetOptional
	tests = append(tests, &testCase{
		_id:       "unset-optional",
		_expected: unset,

		values: []skills.VariableValue{{Cause: "not on PATH", Interface: iface, Name: name, Source: "unset"}},
	})

	// deferred
	deferred := base
	deferred.IsDeferred, deferred.Mark, deferred.Source, deferred.State = true, "↯", "invocation", envStateDeferred
	tests = append(tests, &testCase{
		_id:       "deferred",
		_expected: deferred,

		values: []skills.VariableValue{{Interface: iface, IsDeferred: true, Name: name, Source: "invocation"}},
	})

	// value-missing-keeps-row
	missing := base
	missing.Interface, missing.Mark, missing.Scope, missing.State, missing.Summary = "", "○", "", envStateUnsetOptional, ""
	tests = append(tests, &testCase{
		_id:       "value-missing-keeps-row",
		_expected: missing,

		values: nil,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, []structureEnvRow{test._expected}, graphEnvRows(model, titles, test.values))
		})
	}
}

func TestEnvState(t *testing.T) {
	assert.Equal(t, envStateUnsetRequired, envState(&skills.VariableValue{}, true))
	assert.Equal(t, envStateUnsetOptional, envState(&skills.VariableValue{}, false))
	assert.Equal(t, envStateSet, envState(&skills.VariableValue{Value: "x"}, true))
	assert.Equal(t, envStateDeferred, envState(&skills.VariableValue{IsDeferred: true}, true))
}

package skillimport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

// proofFixture is a small format-2 SKILL.md covering the parser's variance
// classes the importer types.
const proofFixture = "---\n" +
	"name: demo\n" +
	"description: demo fixture. Trigger on nothing.\n" +
	"argument-hint: 'mode: a'\n" +
	"version: 1.0\n" +
	"format: 2\n" +
	"---\n" +
	"# Demo\n" +
	"\n" +
	"Purpose text.\n" +
	"\n" +
	"## Routing\n" +
	"\n" +
	"| For | Use |\n" +
	"| :--- | :--- |\n" +
	"| planning | /fdesign |\n" +
	"\n" +
	"## Args\n" +
	"\n" +
	"| Arg | Form | Default | Effect |\n" +
	"| :--- | :--- | :--- | :--- |\n" +
	"| `mode` | `a\\|b` | `a` | picks a mode |\n" +
	"\n" +
	"## Flow\n" +
	"\n" +
	"`{post: first_done}`\\\n" +
	"**SKILL-DEMO-FLOW-001** `[step]` **First step**\n" +
	"\n" +
	"RUN the first\n" +
	"step with a wrapped statement.\n" +
	"* param: one fact.\n" +
	"  - nested detail.\n" +
	"\n" +
	"```text\n" +
	"woven fence\n" +
	"```\n" +
	"\n" +
	"**SKILL-DEMO-TPL-001** `[payload]` **Template**\n" +
	"\n" +
	"The template.\n" +
	"\n" +
	"````markdown\n" +
	"## TLDR\n" +
	"```go\n" +
	"x := 1\n" +
	"```\n" +
	"````\n" +
	"\n" +
	"\n" +
	"## Nightly\n" +
	"`{section: route, pred: mode = nightly, between: FLOW-001..end}`\n" +
	"\n" +
	"**SKILL-DEMO-NIGHT-001** `[step]` **Night step**\n" +
	"\n" +
	"RUN at night.\n"

// fixtureChangelog is the one-row changelog every imported fixture carries.
const fixtureChangelog = `[{"version": "1.0", "date": "2000-01-01", "text": "fixture"}]`

// writeFixtureDir writes body as <tmp>/<name>/SKILL.md with the fixture
// changelog and returns the dir.
func writeFixtureDir(t *testing.T, body, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, skillgraph.FileManifest), []byte(body), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, changelogFile), []byte(fixtureChangelog), 0o600))
	return dir
}

func TestProve(t *testing.T) {
	type testCase struct {
		_id         string
		_contains   string
		_shouldPass bool

		mutate func(skill *skillgraph.Skill)
		old    string
	}

	tests := make([]*testCase, 0)

	// pass-args-legacy-forms
	tests = append(tests, &testCase{_id: "pass-args-legacy-forms", _shouldPass: true, mutate: func(*skillgraph.Skill) {}, old: proofFixture})

	// pass-bare-bullet-names
	bare := strings.Replace(proofFixture, "| Arg | Form | Default | Effect |\n| :--- | :--- | :--- | :--- |\n| `mode` | `a\\|b` | `a` | picks a mode |\n", "- mode: picks a mode\n", 1)
	tests = append(tests, &testCase{_id: "pass-bare-bullet-names", _shouldPass: true, mutate: func(*skillgraph.Skill) {}, old: bare})

	// fail-word-changed
	tests = append(tests, &testCase{
		_id:       "fail-word-changed",
		_contains: excerptMarker,
		mutate: func(skill *skillgraph.Skill) {
			ref, _ := skill.Entry("SKILL-DEMO-NIGHT-001")
			ref.Entry.Statement = "RUN at dawn."
		},
		old: proofFixture,
	})

	// fail-parse-only
	tests = append(tests, &testCase{
		_id:       "fail-parse-only",
		_contains: "Params differs",
		mutate: func(skill *skillgraph.Skill) {
			ref, _ := skill.Entry("SKILL-DEMO-FLOW-001")
			ref.Entry.Items[0].Depth = 1
		},
		old: proofFixture,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			skill, err := Import(writeFixtureDir(t, test.old, "demo"))
			require.NoError(t, err)

			test.mutate(skill)
			err = Prove([]byte(test.old), skill)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Contains(t, errorText(err), test._contains)
		})
	}
}

func TestRewriteArgs(t *testing.T) {
	outside := "## Flow\n\n| :--- | :--- | :--- | :--- |\n- mode: effect\n"
	assert.Equal(t, outside, rewriteArgs(outside), "outside-args-untouched")

	fenced := "## Args\n\n````\n## Other\n- mode: effect\n````\n- mode: effect\n"
	want := "## Args\n\n````\n## Other\n- mode: effect\n````\n- `mode`: effect\n"
	assert.Equal(t, want, rewriteArgs(fenced), "args-heading-in-fence")
}

func TestParseSkill(t *testing.T) {
	skill, err := parseSkill([]byte(proofFixture))
	require.NoError(t, err)
	assert.Equal(t, " 'mode: a'", skill.Frontmatter[2].Value)
	assert.Equal(t, "# Demo\n\nPurpose text.", skill.Preamble)
	assert.Equal(t, "| For | Use |\n| :--- | :--- |\n| planning | /fdesign |", skill.Sections[0].Blocks[0].Prose, "routing-table-stays-prose")
	assert.Equal(t, "mode", skill.Args()[0].Name)

	flow, _ := skill.Entry("SKILL-DEMO-FLOW-001")
	assert.Equal(t, "RUN the first step with a wrapped statement.", flow.Entry.Statement)
	assert.Equal(t, []string{"first_done"}, flow.Entry.Post)
	assert.Equal(t, skillgraph.Item{Label: "param", Marker: "*", Text: "one fact."}, flow.Entry.Items[0])
	assert.Equal(t, 1, flow.Entry.Items[1].Depth)
	assert.Equal(t, &skillgraph.Fence{Body: "woven fence\n", Info: "text", Marker: "```"}, flow.Entry.Items[2].Fence)

	template, _ := skill.Entry("SKILL-DEMO-TPL-001")
	assert.Equal(t, "## TLDR\n```go\nx := 1\n```\n", template.Entry.Payload.Body)
	assert.Equal(t, "````", template.Entry.Payload.Marker)
	assert.Nil(t, template.Entry.Items)

	route := skill.Sections[3]
	assert.Equal(t, skillgraph.SectionKindRoute, route.Kind)
	assert.Equal(t, "mode = nightly", route.Pred)
	assert.Equal(t, "FLOW-001..end", route.Between)
}

func TestTakeRuntimeFields(t *testing.T) {
	rendered := []skillgraph.Field{
		{Key: "name", Value: " demo"},
		{Key: skillgraph.FieldDocker, Value: " always"},
		{Key: skillgraph.FieldEnableDelegation, Value: " never"},
		{Key: skillgraph.FieldEnforcePermissions, Value: " acceptEdits, planMode"},
		{Key: skillgraph.FieldAnsweredBy, Value: " hook"},
	}
	expectedAuthored := []skillgraph.Field{{Key: "name", Value: " demo"}}
	expectedModes := []string{skillgraph.PermissionAcceptEdits, skillgraph.PermissionPlanMode}
	skill := &skillgraph.Skill{}
	authored := takeRuntimeFields(rendered, skill)

	assert.Equal(t, expectedAuthored, authored)
	assert.Equal(t, skillgraph.DockerAlways, skill.Docker)
	assert.Equal(t, skillgraph.DelegationNever, skill.EnableDelegation)
	assert.Equal(t, expectedModes, skill.EnforcePermissions)
	assert.Equal(t, skillgraph.AnsweredByHook, skill.AnsweredBy)
}

func TestParseSkill_Failures(t *testing.T) {
	type testCase struct {
		_id       string
		_contains string

		data string
	}

	tests := make([]*testCase, 0)

	// fail-no-frontmatter
	tests = append(tests, &testCase{_id: "fail-no-frontmatter", _contains: "No frontmatter", data: "# Demo\n"})

	// fail-odd-indent
	tests = append(tests, &testCase{_id: "fail-odd-indent", _contains: "odd indent", data: strings.Replace(proofFixture, "  - nested detail.", "   - nested detail.", 1)})

	// fail-bad-clause
	tests = append(tests, &testCase{_id: "fail-bad-clause", _contains: "Section clause line", data: strings.Replace(proofFixture, "between: FLOW-001..end}", "between: FLOW-001..end, replaces: FLOW-001}", 1)})

	// fail-stray-after-fence
	tests = append(tests, &testCase{_id: "fail-stray-after-fence", _contains: "no bullet, continuation or fence", data: strings.Replace(proofFixture, "woven fence\n```\n", "woven fence\n```\n  stray line\n", 1)})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			_, err := parseSkill([]byte(test.data))
			assert.ErrorContains(t, err, test._contains)
		})
	}
}

func TestImport(t *testing.T) {
	dir := writeFixtureDir(t, proofFixture, "demo")
	atoms := `{"schemaVersion": 1, "sorts": {"state": [{"id": "first_done"}]}, "exclusive": [["first_done"]]}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, atomsFile), []byte(atoms), 0o600))

	skill, err := Import(dir)
	require.NoError(t, err)
	assert.Equal(t, &skillgraph.Atom{Sort: "state"}, skill.Atoms["first_done"], "atoms-and-changelog")
	assert.Equal(t, [][]string{{"first_done"}}, skill.Exclusive)
	assert.Equal(t, "1.0", skill.Changelog[0].Version)

	noAtoms, err := Import(writeFixtureDir(t, proofFixture, "demo"))
	require.NoError(t, err)
	assert.Nil(t, noAtoms.Atoms, "no-atoms")

	changesDir := writeFixtureDir(t, proofFixture, "demo")
	require.NoError(t, os.WriteFile(filepath.Join(changesDir, changelogFile), []byte(`[{"version": "1.0", "date": "2000-01-01", "changes": ["a", "b"]}]`), 0o600))
	changes, err := Import(changesDir)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, changes.Changelog[0].Changes, "changes-list-row kept")

	unknownDir := writeFixtureDir(t, proofFixture, "demo")
	require.NoError(t, os.WriteFile(filepath.Join(unknownDir, changelogFile), []byte(`[{"version": "1.0", "date": "2000-01-01", "text": "x", "extra": 1}]`), 0o600))
	_, err = Import(unknownDir)
	require.Error(t, err, "fail-changelog-unknown-key")
}

func TestImportVocabulary(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool

		data string
	}

	tests := make([]*testCase, 0)

	// sorts
	tests = append(tests, &testCase{_id: "sorts", _shouldPass: true, data: `{"schemaVersion": 1, "sorts": {"state": [{"id": "ready", "aliases": []}]}}`})

	// fail-duplicate-id-across-sorts
	tests = append(tests, &testCase{_id: "fail-duplicate-id-across-sorts", _shouldPass: false, data: `{"schemaVersion": 1, "sorts": {"a": [{"id": "x"}], "b": [{"id": "x"}]}}`})

	// fail-unknown-key
	tests = append(tests, &testCase{_id: "fail-unknown-key", _shouldPass: false, data: `{"schemaVersion": 1, "sorts": {}, "extra": 1}`})

	// fail-schema-version-2
	tests = append(tests, &testCase{_id: "fail-schema-version-2", _shouldPass: false, data: `{"schemaVersion": 2, "sorts": {}}`})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "vocabulary.json")
			require.NoError(t, os.WriteFile(path, []byte(test.data), 0o600))
			_, err := ImportVocabulary(path)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
		})
	}
}

func TestImportVocabulary_AliasesNil(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vocabulary.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"schemaVersion": 1, "sorts": {"state": [{"id": "ready", "aliases": []}]}}`), 0o600))
	vocabulary, err := ImportVocabulary(path)
	require.NoError(t, err)
	assert.Equal(t, &skillgraph.Atom{Sort: "state"}, vocabulary.Atoms["ready"])
}

// errorText is an error's message, "" for nil.
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

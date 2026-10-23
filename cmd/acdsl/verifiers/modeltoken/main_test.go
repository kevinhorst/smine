package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skillimport"
)

func TestValueTokens(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		value string
	}

	tests := make([]*testCase, 0)

	// bracket-and-effort
	tests = append(tests, &testCase{_id: "bracket-and-effort", _expected: []string{"opus"}, value: "2[opus:medium] station: opus:high task: x"})

	// stops-at-next-key
	tests = append(tests, &testCase{_id: "stops-at-next-key", _expected: []string{"opus"}, value: "opus:high task: claude-opus-4-8"})

	// arm-list-with-mcp-suffix
	tests = append(tests, &testCase{_id: "arm-list-with-mcp-suffix", _expected: []string{"opus", "claude-sonnet-5"}, value: "opus:high, claude-sonnet-5[1m]:low+no-goland"})

	// quoted-literal
	tests = append(tests, &testCase{_id: "quoted-literal", _expected: []string{"sonnet"}, value: "'sonnet', effort: 'low' }"})

	// identifier-skipped
	tests = append(tests, &testCase{_id: "identifier-skipped", _expected: []string{}, value: "args.laneModel || DEFAULT_SUBAGENT_MODEL }"})

	// placeholder-skipped
	tests = append(tests, &testCase{_id: "placeholder-skipped", _expected: []string{}, value: "<model>[:<effort>]"})

	// bare-flag-ends-list
	tests = append(tests, &testCase{_id: "bare-flag-ends-list", _expected: []string{"sonnet"}, value: "sonnet:low smoke repo: x"})

	// list-literal
	tests = append(tests, &testCase{_id: "list-literal", _expected: []string{"opus", "sonnet"}, value: "[opus, sonnet] efforts[]: [high]"})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, valueTokens(test.value))
		})
	}
}

// saveModel imports a demo skill whose argument-hint is hint into
// <tmp>/demo/, saves its model file and removes the markdown source, so only
// the model remains; returns the model path.
func saveModel(t *testing.T, hint string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "demo")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	manifest := "---\nname: demo\ndescription: demo fixture. Trigger on nothing.\nargument-hint: " + hint + "\nauthor: Kevin Horst\nversion: 1.0\nformat: 2\n---\n# Demo\n\n## Run\n\n**SKILL-DEMO-RUN-001** `[step]` **Act**\n\nACT now.\n"
	changelog := `[{"version": "1.0", "date": "2000-01-01", "text": "fixture"}]`
	require.NoError(t, os.WriteFile(filepath.Join(dir, skillgraph.FileManifest), []byte(manifest), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "changelog.json"), []byte(changelog), 0o644))
	skill, err := skillimport.Import(dir)
	require.NoError(t, err)
	require.NoError(t, skillgraph.Save(dir, skill))
	require.NoError(t, os.Remove(filepath.Join(dir, skillgraph.FileManifest)))
	require.NoError(t, os.Remove(filepath.Join(dir, "changelog.json")))
	return skillgraph.ModelPath(dir)
}

func TestRun_model(t *testing.T) {
	type testCase struct {
		_id           string
		_expected     string
		_expectedExit int

		file string
	}

	tests := make([]*testCase, 0)

	// model-path-rendered
	tests = append(tests, &testCase{
		_id:           "model-path-rendered",
		_expected:     "",
		_expectedExit: exitPass,

		file: saveModel(t, "'model: opus'"),
	})

	// finding-names-rendered-line
	haiku := saveModel(t, "'model: haiku'")
	tests = append(tests, &testCase{
		_id:           "finding-names-rendered-line",
		_expected:     filepath.Join(filepath.Dir(haiku), skillgraph.FileManifest) + ":4: ",
		_expectedExit: exitViolations,

		file: haiku,
	})

	t.Chdir(filepath.Join("..", "..", "..", ".."))

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			list := filepath.Join(t.TempDir(), "files.txt")
			require.NoError(t, os.WriteFile(list, []byte(test.file+"\n"), 0o600))
			var out bytes.Buffer

			exit := run([]string{list, "keys=model", "role=subagent", "line=^argument-hint:"}, &out)

			assert.Equal(t, test._expectedExit, exit, out.String())
			assert.True(t, strings.HasPrefix(out.String(), test._expected), out.String())
		})
	}
}

func TestRun(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedExit     int
		_expectedViolated string

		content  string
		fileName string
		params   []string
	}

	tests := make([]*testCase, 0)

	// subagent-key-pass
	tests = append(tests, &testCase{_id: "subagent-key-pass", _expectedExit: exitPass, content: "model: opus\n", fileName: "fixture.txt", params: []string{"keys=model", "role=subagent"}})

	// subagent-key-haiku
	tests = append(tests, &testCase{_id: "subagent-key-haiku", _expectedExit: exitViolations, _expectedViolated: ":1:", content: "model: haiku\n", fileName: "fixture.txt", params: []string{"keys=model", "role=subagent"}})

	// both-keys-on-one-line
	tests = append(tests, &testCase{_id: "both-keys-on-one-line", _expectedExit: exitViolations, _expectedViolated: "haiku", content: "argument-hint: 'lanes: 2[opus:medium] station: haiku:high task: x'\n", fileName: "fixture.txt", params: []string{"keys=lanes,station", "role=subagent"}})

	// subagent-key-fable
	tests = append(tests, &testCase{_id: "subagent-key-fable", _expectedExit: exitViolations, _expectedViolated: ":2:", content: "x\nstation: fable:high\n", fileName: "fixture.txt", params: []string{"keys=station", "role=subagent"}})

	// session-fable-pass
	tests = append(tests, &testCase{_id: "session-fable-pass", _expectedExit: exitPass, content: "arms: fable:high, opus:high\n", fileName: "fixture.txt", params: []string{"keys=arms", "role=session"}})

	// line-filter-skips-prose
	tests = append(tests, &testCase{_id: "line-filter-skips-prose", _expectedExit: exitPass, content: "`lanes: n[model:effort]`\n", fileName: "fixture.txt", params: []string{"keys=lanes", "role=subagent", "line=^argument-hint:"}})

	// ids-retired
	tests = append(tests, &testCase{_id: "ids-retired", _expectedExit: exitViolations, _expectedViolated: ":1:", content: "ROUTINE_MODEL=claude-opus-4-8[1m]\n", fileName: "fixture.txt", params: []string{"ids=true", "role=session"}})

	// ids-current
	tests = append(tests, &testCase{_id: "ids-current", _expectedExit: exitPass, content: "const m = \"claude-opus-5-5[1m]\"\n", fileName: "fixture.txt", params: []string{"ids=true", "role=session"}})

	// missing-keys
	tests = append(tests, &testCase{_id: "missing-keys", _expectedExit: exitToolError, content: "model: opus\n", fileName: "fixture.txt", params: []string{"role=subagent"}})

	// json-typed-arg-pass
	tests = append(tests, &testCase{_id: "json-typed-arg-pass", _expectedExit: exitPass, content: "{\n  \"args\": [\n    {\n      \"name\": \"station\",\n      \"value\": \"opus:medium\"\n    }\n  ]\n}\n", fileName: "fixture.json", params: []string{"keys=judge,lanes,station,models", "role=subagent"}})

	// json-typed-arg-haiku
	tests = append(tests, &testCase{_id: "json-typed-arg-haiku", _expectedExit: exitViolations, _expectedViolated: ":5:", content: "{\n  \"args\": [\n    {\n      \"name\": \"station\",\n      \"value\": \"haiku:medium\"\n    }\n  ]\n}\n", fileName: "fixture.json", params: []string{"keys=judge,lanes,station,models", "role=subagent"}})

	// json-keyed-member-haiku
	tests = append(tests, &testCase{_id: "json-keyed-member-haiku", _expectedExit: exitViolations, _expectedViolated: ":3:", content: "{\n  \"judge\": \"opus\",\n  \"models\": [\"sonnet\", \"haiku\"]\n}\n", fileName: "fixture.json", params: []string{"keys=judge,lanes,station,models", "role=subagent"}})

	t.Chdir(filepath.Join("..", "..", "..", ".."))

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, test.fileName)
			require.NoError(t, os.WriteFile(file, []byte(test.content), 0o600))
			list := filepath.Join(dir, "files.txt")
			require.NoError(t, os.WriteFile(list, []byte(file+"\n"), 0o600))
			var out bytes.Buffer

			exit := run(append([]string{list}, test.params...), &out)

			assert.Equal(t, test._expectedExit, exit, out.String())
			assert.Contains(t, out.String(), test._expectedViolated)
		})
	}
}

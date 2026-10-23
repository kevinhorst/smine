package sessions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/contextrecord"
	"github.com/kevinhorst/smine/internal/redact"
)

const (
	sessionSchemaPath = "../../skills/smine/smine-batch/reference/session.schema.json"

	// fixtureContextBlock is the canonical context object of testdata/record.json.
	fixtureContextBlock = `"context": {
    "honored": 1,
    "pack_reads": [
      "context/actions/implementing.md"
    ]
  },`

	// ghostRecordJson is the canonical form of a skipped (ghost) record.
	ghostRecordJson = `{
  "id": "85a829c6-8c10-4c9b-9dea-095f019cedc7",
  "agent": "claude",
  "findings": [],
  "format": 2,
  "lastActive": "2026-09-23T22:35:26Z",
  "repo": "claude-configs",
  "skipReason": "ghost",
  "skipped": true,
  "title": "Reload skills",
  "week": "2026-W39"
}
`

	// formatOneRecordJson is the canonical form of a record mined before format 2.
	formatOneRecordJson = `{
  "id": "c67a1a4c-ebdd-47fc-a84b-677e4eee8e41",
  "agent": "claude",
  "findings": [
    {
      "dimension": "exemplar",
      "quotes": [
        "cherry-pick or merge from there, it's not on your checkout"
      ],
      "summary": "Fix mirrored the nearest in-repo guard pattern."
    }
  ],
  "lastActive": "2026-07-24T18:02:11Z",
  "repo": "claude-configs",
  "title": "Format one record",
  "week": "2026-W30"
}
`
)

func readFixture(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", "record.json"))
	require.NoError(t, err)
	return string(content)
}

func writeTempRecord(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "c67a1a4c-ebdd-47fc-a84b-677e4eee8e41.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestCheckRecord(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedFile      string
		_expectedViolation string
		_shouldPass        bool

		path string
	}

	valid := readFixture(t)
	secretCommand := "GH_TOKEN=ghp_" + strings.Repeat("a", 36) + " gh pr list"
	tests := make([]*testCase, 0)

	// pass-valid-record
	tests = append(tests, &testCase{
		_id:                "pass-valid-record",
		_expectedFile:      valid,
		_expectedViolation: "",
		_shouldPass:        true,

		path: writeTempRecord(t, valid),
	})

	// fail-missing-required-field
	missingTitle := strings.Replace(valid, `  "title": "Config-server card label collapse sweep",`+"\n", "", 1)
	tests = append(tests, &testCase{
		_id:                "fail-missing-required-field",
		_expectedFile:      strings.Replace(valid, `"title": "Config-server card label collapse sweep"`, `"title": ""`, 1),
		_expectedViolation: "schema: ",
		_shouldPass:        false,

		path: writeTempRecord(t, missingTitle),
	})

	// pass-empty-findings-normalized
	nullFindings := strings.Replace(ghostRecordJson, `"findings": []`, `"findings": null`, 1)
	tests = append(tests, &testCase{
		_id:                "pass-empty-findings-normalized",
		_expectedFile:      ghostRecordJson,
		_expectedViolation: "",
		_shouldPass:        true,

		path: writeTempRecord(t, nullFindings),
	})

	// pass-skipped-ghost-record
	ghostWithoutFindings := `{"id": "85a829c6-8c10-4c9b-9dea-095f019cedc7", "agent": "claude", "repo": "claude-configs", ` +
		`"week": "2026-W39", "lastActive": "2026-09-23T22:35:26Z", "title": "Reload skills", "skipped": true, "skipReason": "ghost", "format": 2}`
	tests = append(tests, &testCase{
		_id:                "pass-skipped-ghost-record",
		_expectedFile:      ghostRecordJson,
		_expectedViolation: "",
		_shouldPass:        true,

		path: writeTempRecord(t, ghostWithoutFindings),
	})

	// pass-permission-redaction
	secretRecord := strings.Replace(valid, `"command": "make audit"`, `"command": "`+secretCommand+`"`, 1)
	redactedRecord := strings.Replace(valid, `"command": "make audit"`, `"command": "`+redact.RedactSecrets(secretCommand)+`"`, 1)
	tests = append(tests, &testCase{
		_id:                "pass-permission-redaction",
		_expectedFile:      redactedRecord,
		_expectedViolation: "",
		_shouldPass:        true,

		path: writeTempRecord(t, secretRecord),
	})

	// fail-malformed-json
	tests = append(tests, &testCase{
		_id:                "fail-malformed-json",
		_expectedFile:      "{not json",
		_expectedViolation: "json: ",
		_shouldPass:        false,

		path: writeTempRecord(t, "{not json"),
	})

	// fail-unknown-field
	unknownField := strings.Replace(valid, `"type": "Diagnose-Debug"`, `"colour": "red"`, 1)
	tests = append(tests, &testCase{
		_id:                "fail-unknown-field",
		_expectedFile:      unknownField,
		_expectedViolation: `json: unknown field "colour"`,
		_shouldPass:        false,

		path: writeTempRecord(t, unknownField),
	})

	// fail-bad-last-active
	badLastActive := strings.Replace(valid, "2026-07-24T18:02:11Z", "yesterday", 1)
	tests = append(tests, &testCase{
		_id:                "fail-bad-last-active",
		_expectedFile:      badLastActive,
		_expectedViolation: "schema: ",
		_shouldPass:        false,

		path: writeTempRecord(t, badLastActive),
	})

	// fail-unknown-agent
	unknownAgent := strings.Replace(valid, `"agent": "claude"`, `"agent": "gemini"`, 1)
	tests = append(tests, &testCase{
		_id:                "fail-unknown-agent",
		_expectedFile:      unknownAgent,
		_expectedViolation: "schema: ",
		_shouldPass:        false,

		path: writeTempRecord(t, unknownAgent),
	})

	// fail-format-missing
	formatMissing := strings.Replace(valid, `  "format": 2,`+"\n", "", 1)
	tests = append(tests, &testCase{
		_id:                "fail-format-missing",
		_expectedFile:      formatMissing,
		_expectedViolation: "format: 2 required",
		_shouldPass:        false,

		path: writeTempRecord(t, formatMissing),
	})

	// fail-headline-missing
	headlineMissing := strings.Replace(valid, `      "headline": "The fix mirrored the nearest guard pattern and was verified on a throwaway build.",`+"\n", "", 1)
	tests = append(tests, &testCase{
		_id:                "fail-headline-missing",
		_expectedFile:      headlineMissing,
		_expectedViolation: "schema: ",
		_shouldPass:        false,

		path: writeTempRecord(t, headlineMissing),
	})

	// fail-headline-hex-id
	headlineHexId := strings.Replace(valid, "verified on a throwaway build.", "verified on f70aa7e0.", 1)
	tests = append(tests, &testCase{
		_id:                "fail-headline-hex-id",
		_expectedFile:      headlineHexId,
		_expectedViolation: "findings[0].headline: hex id",
		_shouldPass:        false,

		path: writeTempRecord(t, headlineHexId),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			violations, err := CheckRecord(test.path, sessionSchemaPath, nil)
			content, readErr := os.ReadFile(test.path)

			assert.NoError(t, err)
			assert.NoError(t, readErr)
			assert.Equalf(t, test._shouldPass, len(violations) == 0, "violations = %v", violations)
			assert.Contains(t, strings.Join(violations, "\n"), test._expectedViolation)
			assert.Equal(t, test._expectedFile, string(content))
		})
	}
}

func TestReadRecord_ContextAgentDetail(t *testing.T) {
	contextBlock := `"context": {"agent": "claude", "git_branch": "claude/context-page", "agents": [{"agent_id": "root", ` +
		`"context": {"acdsl_rules": [{"file": "internal/a.go", "ids": ["ACDSL-GOLANG-FMT-001"]}], ` +
		`"injected": {"lang": [{"coverage": "complete", "guide": "context/rules/go.md", "ids": [], "lang": "go", "total_lines": 1327}], ` +
		`"skill": [{"ids": ["ACTION-CONCEPT-HOT-006"], "skill": "fdesign", "version": "1.4.0"}]}, ` +
		`"pack_reads": ["context/actions/implementing.md"], "plan_rules": ["RULE-PLAN-001"], ` +
		`"subagent_context": [{"ids": ["FACT-REPO-ARCH-002"], "paths": ["context/actions/implementing.md"]}]}, ` +
		`"files": [{"path": "internal/a.go", "reads": 2, "writes": 1}], ` +
		`"items": [{"chars": 12, "counter": "anthropic", "end_line": 2, "kind": "global", "label": "always-context.md", "lines": 2, ` +
		`"preview": "ACTION-IMPL-EXEC-006", "range": "1-2", "source": "hook_additional_context", "start_line": 1, "tokens": 4}], ` +
		`"kind": "root", "totals": {"chars": 12, "items": 1, "lines": 2, "tokens": 4, "uncounted": 0}}], ` +
		`"totals": {"chars": 12, "items": 1, "lines": 2, "tokens": 4, "uncounted": 0}, "honored": 1},`
	path := writeTempRecord(t, strings.Replace(readFixture(t), fixtureContextBlock, contextBlock, 1))

	session, violations, err := ReadRecord(path)
	require.NoError(t, err)
	require.Empty(t, violations)

	assert.Equal(t, "claude/context-page", session.Context.GitBranch)
	assert.Equal(t, 4, session.Context.Totals.Tokens)
	assert.Equal(t, 1, session.Context.Honored)
	require.Len(t, session.Context.Agents, 1)
	agent := session.Context.Agents[0]
	assert.Equal(t, contextrecord.AgentRoot, agent.Kind)
	require.NotNil(t, agent.Context)
	assert.Equal(t, []string{"context/actions/implementing.md"}, agent.Context.PackReads)
	assert.Equal(t, []string{"RULE-PLAN-001"}, agent.Context.PlanRules)
	assert.Equal(t, "1.4.0", agent.Context.Injected.Skill[0].Version)
	expectedFiles := []contextrecord.FileTouch{{Path: "internal/a.go", Reads: 2, Writes: 1}}
	assert.Equal(t, expectedFiles, agent.Files)
	require.Len(t, agent.Items, 1)
	assert.Equal(t, "ACTION-IMPL-EXEC-006", agent.Items[0].Preview)
	assert.Equal(t, "1-2", agent.Items[0].Range)
	assert.Equal(t, 1, agent.Items[0].StartLine)
	assert.Equal(t, 2, agent.Items[0].EndLine)
	require.Len(t, agent.Context.Injected.Lang, 1)
	assert.Equal(t, 1327, agent.Context.Injected.Lang[0].TotalLines)
}

func TestReadRecord_OlderContextRoundTrips(t *testing.T) {
	contextObject := `{"acdsl_rules":[{"file":"internal/a.go","ids":["ACDSL-GOLANG-FMT-001"]}],` +
		`"injected":{"global":["ACTION-IMPL-EXEC-006"],"lang":[{"coverage":"full","ids":["RULE-GOLANG-NAME-001"],"lang":"go"}],` +
		`"skill":[{"ids":["ACTION-CONCEPT-HOT-006"],"skill":"fdesign"}]},` +
		`"pack_reads":["context/actions/implementing.md"],"plan_rules":["RULE-PLAN-001"],` +
		`"subagent_context":[{"ids":["FACT-REPO-ARCH-002"],"paths":["context/actions/implementing.md"]}]}`
	path := writeTempRecord(t, strings.Replace(readFixture(t), fixtureContextBlock, `"context": `+contextObject+",", 1))

	session, violations, err := ReadRecord(path)
	require.NoError(t, err)
	require.Empty(t, violations)

	rendered, err := json.Marshal(session.Context)
	require.NoError(t, err)
	assert.Equal(t, contextObject, string(rendered))
}

func TestMarkSynthesized(t *testing.T) {
	path := writeTempRecord(t, readFixture(t))

	// set
	violations, err := MarkSynthesized("2026-09-27", path, sessionSchemaPath)
	require.NoError(t, err)
	assert.Empty(t, violations)

	// replace
	violations, err = MarkSynthesized("2026-09-28", path, sessionSchemaPath)
	require.NoError(t, err)
	assert.Empty(t, violations)

	session, violations, err := ReadRecord(path)
	require.NoError(t, err)
	require.Empty(t, violations)
	assert.Equal(t, "2026-09-28", session.Synthesized)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(content), `"synthesized": "2026-09-28"`))

	violations, err = MarkSynthesized("", path, sessionSchemaPath)
	require.NoError(t, err)
	require.Empty(t, violations)
	content, err = os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(content), `"synthesized"`)

	// pass-format-1-string-quote
	formatOnePath := writeTempRecord(t, formatOneRecordJson)
	violations, err = MarkSynthesized("2026-09-27", formatOnePath, sessionSchemaPath)
	require.NoError(t, err)
	assert.Empty(t, violations)
	content, err = os.ReadFile(formatOnePath)
	require.NoError(t, err)
	expected := strings.Replace(formatOneRecordJson, `  "title"`, `  "synthesized": "2026-09-27",`+"\n"+`  "title"`, 1)
	assert.Equal(t, expected, string(content))
}

func TestQuoteJson(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedEncoded string
		_expectedQuote   Quote

		encoded string
	}

	tests := make([]*testCase, 0)

	// string-quote-roundtrips
	tests = append(tests, &testCase{
		_id:              "string-quote-roundtrips",
		_expectedEncoded: `"a quote"`,
		_expectedQuote:   Quote{Text: "a quote"},

		encoded: `"a quote"`,
	})

	// object-quote-roundtrips
	tests = append(tests, &testCase{
		_id:              "object-quote-roundtrips",
		_expectedEncoded: `{"text":"a quote","turn":7}`,
		_expectedQuote:   Quote{Text: "a quote", Turn: 7},

		encoded: `{"turn": 7, "text": "a quote"}`,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			var quote Quote
			decodeErr := json.Unmarshal([]byte(test.encoded), &quote)
			encoded, encodeErr := json.Marshal(quote)

			assert.NoError(t, decodeErr)
			assert.NoError(t, encodeErr)
			assert.Equal(t, test._expectedQuote, quote)
			assert.Equal(t, test._expectedEncoded, string(encoded))
		})
	}
}

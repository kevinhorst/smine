package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const v2Table = `{
  "schemaVersion": "2.0",
  "axes": {
    "effect": ["destructive", "incorrect", "degraded", "debt", "cosmetic"],
    "exposure": ["always", "often", "rare", "eventually"]
  },
  "rows": [
    {"effect": ["destructive"], "exposure": ["always", "often", "rare"], "severity": "BLOCKER"},
    {"effect": ["destructive"], "exposure": ["eventually"], "severity": "MAJOR"},
    {"effect": ["incorrect"], "exposure": ["always", "often"], "severity": "BLOCKER"},
    {"effect": ["incorrect"], "exposure": ["rare"], "severity": "MAJOR"},
    {"effect": ["incorrect"], "exposure": ["eventually"], "severity": "MINOR"},
    {"effect": ["degraded"], "exposure": ["always", "often"], "severity": "MAJOR"},
    {"effect": ["degraded"], "exposure": ["rare"], "severity": "MINOR"},
    {"effect": ["degraded"], "exposure": ["eventually"], "severity": "NIT"},
    {"effect": ["debt"], "exposure": ["always", "often"], "severity": "MINOR"},
    {"effect": ["debt"], "exposure": ["rare", "eventually"], "severity": "NIT"},
    {"effect": ["cosmetic"], "severity": "NIT"}
  ]
}`

const smallTable = `{
  "schemaVersion": "2.0",
  "axes": {"effect": ["incorrect", "cosmetic"], "exposure": ["often", "rare"]},
  "rows": [
    {"effect": ["incorrect"], "exposure": ["often"], "severity": "MAJOR"},
    {"effect": ["incorrect"], "exposure": ["rare"], "severity": "MINOR"},
    {"effect": ["cosmetic"], "severity": "NIT"}
  ]
}`

const overlappingTable = `{
  "schemaVersion": "2.0",
  "axes": {"effect": ["incorrect", "cosmetic"], "exposure": ["often", "rare"]},
  "rows": [
    {"effect": ["incorrect"], "severity": "MAJOR"},
    {"effect": ["incorrect"], "exposure": ["rare"], "severity": "MINOR"},
    {"effect": ["cosmetic"], "severity": "NIT"}
  ]
}`

const gapTable = `{
  "schemaVersion": "2.0",
  "axes": {"effect": ["incorrect", "cosmetic"], "exposure": ["often", "rare"]},
  "rows": [
    {"effect": ["incorrect"], "exposure": ["often"], "severity": "MAJOR"},
    {"effect": ["cosmetic"], "severity": "NIT"}
  ]
}`

const unknownMemberTable = `{
  "schemaVersion": "2.0",
  "axes": {"effect": ["incorrect", "cosmetic"], "exposure": ["often", "rare"]},
  "rows": [
    {"effect": ["incorrect"], "exposure": ["often", "constantly"], "severity": "MAJOR"},
    {"effect": ["incorrect"], "exposure": ["rare"], "severity": "MINOR"},
    {"effect": ["cosmetic"], "severity": "NIT"}
  ]
}`

const smallFixtures = `{
  "schemaVersion": "2.0",
  "table": "schemas/severity-table.json",
  "fixtures": [
    {"id": "incorrect-often", "effect": "incorrect", "exposure": "often", "severity": "MAJOR", "discriminator": "exposure=often", "example": true},
    {"id": "cosmetic-rare", "effect": "cosmetic", "exposure": "rare", "severity": "NIT", "discriminator": "effect=cosmetic", "example": true}
  ]
}`

func TestCheckFixtures(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   string
		_shouldPass bool

		fixturesJson string
		tableJson    string
	}

	tests := make([]*testCase, 0)

	// pass-v2-table
	tests = append(tests, &testCase{
		_id:         "pass-v2-table",
		_expected:   "",
		_shouldPass: true,

		fixturesJson: `{"schemaVersion": "2.0", "table": "schemas/severity-table.json", "fixtures": [
			{"id": "incorrect-often", "effect": "incorrect", "exposure": "often", "severity": "BLOCKER", "discriminator": "exposure=often", "example": true},
			{"id": "destructive-eventually", "effect": "destructive", "exposure": "eventually", "severity": "MAJOR", "discriminator": "exposure=eventually", "example": true},
			{"id": "incorrect-spec-gap", "effect": "incorrect", "exposure": "rare", "severity": "MAJOR", "discriminator": "effect=incorrect", "example": true}
		]}`,
		tableJson: v2Table,
	})

	// pass-empty-fixtures
	tests = append(tests, &testCase{
		_id:         "pass-empty-fixtures",
		_expected:   "",
		_shouldPass: true,

		fixturesJson: `{"schemaVersion": "2.0", "table": "schemas/severity-table.json", "fixtures": []}`,
		tableJson:    smallTable,
	})

	// fail-overlapping-rows
	tests = append(tests, &testCase{
		_id:         "fail-overlapping-rows",
		_expected:   "Vector incorrect/rare matches 2 rows, want exactly 1",
		_shouldPass: false,

		fixturesJson: smallFixtures,
		tableJson:    overlappingTable,
	})

	// fail-gap
	tests = append(tests, &testCase{
		_id:         "fail-gap",
		_expected:   "Vector incorrect/rare matches 0 rows, want exactly 1",
		_shouldPass: false,

		fixturesJson: smallFixtures,
		tableJson:    gapTable,
	})

	// fail-fixture-mismatch
	tests = append(tests, &testCase{
		_id:         "fail-fixture-mismatch",
		_expected:   `fixture "incorrect-often" resolves to MAJOR, expects MINOR`,
		_shouldPass: false,

		fixturesJson: `{"schemaVersion": "2.0", "table": "schemas/severity-table.json", "fixtures": [
			{"id": "incorrect-often", "effect": "incorrect", "exposure": "often", "severity": "MINOR", "discriminator": "exposure=often", "example": true}
		]}`,
		tableJson: smallTable,
	})

	// fail-duplicate-id
	tests = append(tests, &testCase{
		_id:         "fail-duplicate-id",
		_expected:   `duplicate fixture id "incorrect-often"`,
		_shouldPass: false,

		fixturesJson: `{"schemaVersion": "2.0", "table": "schemas/severity-table.json", "fixtures": [
			{"id": "incorrect-often", "effect": "incorrect", "exposure": "often", "severity": "MAJOR", "discriminator": "exposure=often", "example": true},
			{"id": "incorrect-often", "effect": "incorrect", "exposure": "rare", "severity": "MINOR", "discriminator": "exposure=rare", "example": true}
		]}`,
		tableJson: smallTable,
	})

	// fail-unknown-member
	tests = append(tests, &testCase{
		_id:         "fail-unknown-member",
		_expected:   `row 0: exposure value "constantly" is not an axis member`,
		_shouldPass: false,

		fixturesJson: smallFixtures,
		tableJson:    unknownMemberTable,
	})

	// fail-invalid-json
	tests = append(tests, &testCase{
		_id:         "fail-invalid-json",
		_expected:   "Invalid JSON",
		_shouldPass: false,

		fixturesJson: `{"schemaVersion": `,
		tableJson:    smallTable,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "skill")
			require.NoError(t, os.MkdirAll(filepath.Join(dir, "schemas"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "schemas", "severity-table.json"), []byte(test.tableJson), 0o644))
			fixturesPath := filepath.Join(dir, "severity-fixtures.json")
			require.NoError(t, os.WriteFile(fixturesPath, []byte(test.fixturesJson), 0o644))
			listPath := filepath.Join(t.TempDir(), "files.txt")
			require.NoError(t, os.WriteFile(listPath, []byte(fixturesPath+"\n"), 0o644))

			var out bytes.Buffer
			rc := run([]string{listPath}, &out)

			assert.Equalf(t, test._shouldPass, rc == 0, "out = %s", out.String())
			assert.Contains(t, out.String(), test._expected)
		})
	}
}

func TestLookup(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   string
		_shouldPass bool

		effect   string
		exposure string
		rows     []row
	}

	tests := make([]*testCase, 0)

	// one-row-match
	tests = append(tests, &testCase{
		_id:         "one-row-match",
		_expected:   "MAJOR",
		_shouldPass: true,

		effect:   "incorrect",
		exposure: "often",
		rows: []row{
			{Effect: []string{"incorrect"}, Exposure: []string{"often"}, Severity: "MAJOR"},
			{Effect: []string{"incorrect"}, Exposure: []string{"rare"}, Severity: "MINOR"},
		},
	})

	// absent-axis-matches-all
	tests = append(tests, &testCase{
		_id:         "absent-axis-matches-all",
		_expected:   "NIT",
		_shouldPass: true,

		effect:   "cosmetic",
		exposure: "always",
		rows: []row{
			{Effect: []string{"cosmetic"}, Severity: "NIT"},
			{Effect: []string{"incorrect"}, Severity: "MAJOR"},
		},
	})

	// zero-rows-error
	tests = append(tests, &testCase{
		_id:         "zero-rows-error",
		_expected:   "",
		_shouldPass: false,

		effect:   "degraded",
		exposure: "rare",
		rows: []row{
			{Effect: []string{"incorrect"}, Severity: "MAJOR"},
		},
	})

	// two-rows-error
	tests = append(tests, &testCase{
		_id:         "two-rows-error",
		_expected:   "",
		_shouldPass: false,

		effect:   "incorrect",
		exposure: "rare",
		rows: []row{
			{Effect: []string{"incorrect"}, Severity: "MAJOR"},
			{Effect: []string{"incorrect"}, Exposure: []string{"rare"}, Severity: "MINOR"},
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			got, err := lookup(&table{Rows: test.rows}, test.effect, test.exposure)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, got)
		})
	}
}

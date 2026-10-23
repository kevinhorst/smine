package skillgraph

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// provideChangelogSkill is the complete fixture carrying rows, its frontmatter version following the top row.
func provideChangelogSkill(rows ...ChangelogEntry) *Skill {
	skill := provideCompleteSkill()
	skill.Changelog = rows
	for index, field := range skill.Frontmatter {
		if field.Key == "version" {
			skill.Frontmatter[index].Value = " " + rows[0].Version
		}
	}
	return skill
}

// encodedSkill is the canonical model file bytes of a fixture.
func encodedSkill(t *testing.T, skill *Skill) []byte {
	t.Helper()
	encoded, err := EncodeSkill(skill)
	require.NoError(t, err)
	return encoded
}

// versionRows is one row per version, newest first, the text naming the version.
func versionRows(versions ...string) []ChangelogEntry {
	rows := make([]ChangelogEntry, 0, len(versions))
	for _, version := range versions {
		row := ChangelogEntry{Date: "2000-01-01", Text: "row " + version, Version: version}
		rows = append(rows, row)
	}
	return rows
}

// rowVersions is the version column of rows.
func rowVersions(rows []ChangelogEntry) []string {
	versions := make([]string, 0, len(rows))
	for _, row := range rows {
		versions = append(versions, row.Version)
	}
	return versions
}

func TestBump(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		kind    int
		version string
	}

	tests := make([]*testCase, 0)

	// patch
	tests = append(tests, &testCase{_id: "patch", _expected: "1.0.1", kind: 2, version: "1.0.0"})

	// minor
	tests = append(tests, &testCase{_id: "minor", _expected: "1.1.0", kind: 1, version: "1.0.3"})

	// major
	tests = append(tests, &testCase{_id: "major", _expected: "2.0.0", kind: 0, version: "1.4.2"})

	// two-part-at-patch
	tests = append(tests, &testCase{_id: "two-part-at-patch", _expected: "1.7.1", kind: 2, version: "1.7"})

	// two-part-at-minor
	tests = append(tests, &testCase{_id: "two-part-at-minor", _expected: "1.8", kind: 1, version: "1.7"})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			bumped, err := bump(test.version, test.kind)
			assert.NoError(t, err)
			assert.Equal(t, test._expected, bumped)
		})
	}
}

func TestRowKinds(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []int

		rows []ChangelogEntry
	}

	tests := make([]*testCase, 0)

	// plain-run
	tests = append(tests, &testCase{_id: "plain-run", _expected: []int{1, 2, 2}, rows: versionRows("1.1.0", "1.0.1", "1.0.0")})

	// duplicate-inherits
	tests = append(tests, &testCase{_id: "duplicate-inherits", _expected: []int{1, 1, 2}, rows: versionRows("4.7.0", "4.7.0", "4.6.1")})

	// single-row-is-patch
	tests = append(tests, &testCase{_id: "single-row-is-patch", _expected: []int{2}, rows: versionRows("1.0")})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			kinds, err := rowKinds(test.rows)
			assert.NoError(t, err)
			assert.Equal(t, test._expected, kinds)
		})
	}
}

func TestRenumberRows(t *testing.T) {
	type testCase struct {
		_id               string
		_expected         []string
		_expectedOriginal []string
		_shouldPass       bool

		kinds []int
		rows  []ChangelogEntry
	}

	tests := make([]*testCase, 0)

	// pass-duplicate-top
	rows := versionRows("4.7.0", "4.7.0", "4.6.1")
	tests = append(tests, &testCase{
		_id:               "pass-duplicate-top",
		_expected:         []string{"4.8.0", "4.7.0", "4.6.1"},
		_expectedOriginal: []string{"4.7.0", "4.7.0", "4.6.1"},
		_shouldPass:       true,

		kinds: []int{1, 1, 2},
		rows:  rows,
	})

	// pass-cascade
	rows = versionRows("5.6.1", "5.6.0", "5.5.0", "5.4.0", "5.3.0", "5.2.0", "5.1.0", "5.1.0", "5.0.0")
	tests = append(tests, &testCase{
		_id:               "pass-cascade",
		_expected:         []string{"5.7.1", "5.7.0", "5.6.0", "5.5.0", "5.4.0", "5.3.0", "5.2.0", "5.1.0", "5.0.0"},
		_expectedOriginal: rowVersions(rows),
		_shouldPass:       true,

		kinds: []int{2, 1, 1, 1, 1, 1, 1, 1, 2},
		rows:  rows,
	})

	// pass-ascending-triple
	rows = versionRows("8.4.0", "8.3.0", "8.2.1", "8.4.0", "8.3.0", "8.2.0")
	tests = append(tests, &testCase{
		_id:               "pass-ascending-triple",
		_expected:         []string{"8.7.0", "8.6.0", "8.5.0", "8.4.0", "8.3.0", "8.2.0"},
		_expectedOriginal: rowVersions(rows),
		_shouldPass:       true,

		kinds: []int{1, 1, 1, 1, 1, 2},
		rows:  rows,
	})

	// pass-already-descending
	rows = versionRows("2.0.0", "1.1.0", "1.0.1", "1.0")
	tests = append(tests, &testCase{
		_id:               "pass-already-descending",
		_expected:         []string{"2.0.0", "1.1.0", "1.0.1", "1.0"},
		_expectedOriginal: rowVersions(rows),
		_shouldPass:       true,

		kinds: []int{0, 1, 2, 2},
		rows:  rows,
	})

	// fail-kinds-length-mismatch
	rows = versionRows("1.0", "1.0")
	tests = append(tests, &testCase{
		_id:               "fail-kinds-length-mismatch",
		_expected:         []string{},
		_expectedOriginal: []string{"1.0", "1.0"},
		_shouldPass:       false,

		kinds: []int{2},
		rows:  rows,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			renumbered, err := renumberRows(test.rows, test.kinds)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, rowVersions(renumbered))
			assert.Equal(t, test._expectedOriginal, rowVersions(test.rows), "input slice untouched")
		})
	}
}

func TestSetVersion(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []Field

		skill   *Skill
		version string
	}

	tests := make([]*testCase, 0)

	// plain
	skill := provideCompleteSkill()
	expected := provideCompleteSkill().Frontmatter
	expected[2].Value = " 1.1"
	tests = append(tests, &testCase{_id: "plain", _expected: expected, skill: skill, version: "1.1"})

	// quoted
	skill = provideCompleteSkill()
	skill.Frontmatter[2].Value = ` "1.0"`
	expected = provideCompleteSkill().Frontmatter
	expected[2].Value = ` "1.1"`
	tests = append(tests, &testCase{_id: "quoted", _expected: expected, skill: skill, version: "1.1"})

	// no-leading-space
	skill = provideCompleteSkill()
	skill.Frontmatter[2].Value = "1.0"
	expected = provideCompleteSkill().Frontmatter
	expected[2].Value = "1.1"
	tests = append(tests, &testCase{_id: "no-leading-space", _expected: expected, skill: skill, version: "1.1"})

	// no-version-field
	skill = provideCompleteSkill()
	skill.Frontmatter = skill.Frontmatter[:2]
	expected = provideCompleteSkill().Frontmatter[:2]
	tests = append(tests, &testCase{_id: "no-version-field", _expected: expected, skill: skill, version: "1.1"})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			setVersion(test.skill, test.version)
			assert.Equal(t, test._expected, test.skill.Frontmatter)
		})
	}
}

func TestRenumberChangelog(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   string
		_shouldPass bool

		data []byte
	}

	tests := make([]*testCase, 0)

	// pass-duplicate-rows-rewritten
	duplicate := provideChangelogSkill(versionRows("1.0", "1.0")...)
	renumbered := provideChangelogSkill(ChangelogEntry{Date: "2000-01-01", Text: "row 1.0", Version: "1.0.1"}, versionRows("1.0")[0])
	tests = append(tests, &testCase{
		_id:         "pass-duplicate-rows-rewritten",
		_expected:   string(encodedSkill(t, renumbered)),
		_shouldPass: true,

		data: encodedSkill(t, duplicate),
	})

	// pass-descending-same-bytes
	canonical := encodedSkill(t, provideCompleteSkill())
	tests = append(tests, &testCase{
		_id:         "pass-descending-same-bytes",
		_expected:   string(canonical),
		_shouldPass: true,

		data: canonical,
	})

	// fail-undecodable
	tests = append(tests, &testCase{
		_id:         "fail-undecodable",
		_expected:   "",
		_shouldPass: false,

		data: []byte("not cue {"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			rewritten, err := RenumberChangelog(test.data)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, string(rewritten))
		})
	}
}

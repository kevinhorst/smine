package skillgraph

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mergeRow is one changelog row of the merge fixtures.
func mergeRow(date, version, text string) ChangelogEntry {
	return ChangelogEntry{Date: date, Text: text, Version: version}
}

// parsedMerge is ParseMerge over three fixtures, aborting the test on a parse error.
func parsedMerge(t *testing.T, base, ours, theirs *Skill) *ChangelogMerge {
	t.Helper()
	merge, err := ParseMerge(encodedSkill(t, base), encodedSkill(t, ours), encodedSkill(t, theirs))
	require.NoError(t, err)
	return merge
}

func TestParseMerge(t *testing.T) {
	type testCase struct {
		_id                   string
		_expectedOursNewest   string
		_expectedTheirsNewest string
		_isUnmergeable        bool
		_shouldPass           bool

		base   []byte
		ours   []byte
		theirs []byte
	}

	baseRow := mergeRow("2000-01-01", "1.0", "fixture")
	base := encodedSkill(t, provideChangelogSkill(baseRow))
	tests := make([]*testCase, 0)

	// pass-both-sides-added-one-row
	tests = append(tests, &testCase{
		_id:                   "pass-both-sides-added-one-row",
		_expectedOursNewest:   "2000-01-02",
		_expectedTheirsNewest: "2000-01-03",
		_isUnmergeable:        false,
		_shouldPass:           true,

		base:   base,
		ours:   encodedSkill(t, provideChangelogSkill(mergeRow("2000-01-02", "1.0.1", "ours"), baseRow)),
		theirs: encodedSkill(t, provideChangelogSkill(mergeRow("2000-01-03", "1.1", "theirs"), baseRow)),
	})

	// pass-one-side-added-nothing
	tests = append(tests, &testCase{
		_id:                   "pass-one-side-added-nothing",
		_expectedOursNewest:   "2000-01-02",
		_expectedTheirsNewest: "",
		_isUnmergeable:        false,
		_shouldPass:           true,

		base:   base,
		ours:   encodedSkill(t, provideChangelogSkill(mergeRow("2000-01-02", "1.0.1", "ours"), baseRow)),
		theirs: base,
	})

	// fail-ours-edited-an-old-row
	tests = append(tests, &testCase{
		_id:                   "fail-ours-edited-an-old-row",
		_expectedOursNewest:   "",
		_expectedTheirsNewest: "",
		_isUnmergeable:        true,
		_shouldPass:           false,

		base:   base,
		ours:   encodedSkill(t, provideChangelogSkill(mergeRow("2000-01-02", "1.0.1", "ours"), mergeRow("2000-01-01", "1.0", "edited"))),
		theirs: base,
	})

	// fail-theirs-removed-a-row
	twoRowBase := provideChangelogSkill(mergeRow("2000-01-02", "1.1", "second"), baseRow)
	tests = append(tests, &testCase{
		_id:                   "fail-theirs-removed-a-row",
		_expectedOursNewest:   "",
		_expectedTheirsNewest: "",
		_isUnmergeable:        true,
		_shouldPass:           false,

		base:   encodedSkill(t, twoRowBase),
		ours:   encodedSkill(t, twoRowBase),
		theirs: base,
	})

	// pass-tokens-differ-only
	tokenRow := baseRow
	tokenRow.Tokens = 42
	tests = append(tests, &testCase{
		_id:                   "pass-tokens-differ-only",
		_expectedOursNewest:   "2000-01-02",
		_expectedTheirsNewest: "",
		_isUnmergeable:        false,
		_shouldPass:           true,

		base:   base,
		ours:   encodedSkill(t, provideChangelogSkill(mergeRow("2000-01-02", "1.0.1", "ours"), tokenRow)),
		theirs: base,
	})

	// fail-undecodable-base
	tests = append(tests, &testCase{
		_id:                   "fail-undecodable-base",
		_expectedOursNewest:   "",
		_expectedTheirsNewest: "",
		_isUnmergeable:        false,
		_shouldPass:           false,

		base:   []byte("not cue {"),
		ours:   base,
		theirs: base,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			merge, err := ParseMerge(test.base, test.ours, test.theirs)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._isUnmergeable, errors.Is(err, ErrUnmergeable))
			assert.Equal(t, test._expectedOursNewest, newestDates(merge)[0])
			assert.Equal(t, test._expectedTheirsNewest, newestDates(merge)[1])
		})
	}
}

func TestChangelogMerge_Resolve(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedBase    string
		_expectedOurs    string
		_expectedRows    []ChangelogEntry
		_expectedTheirs  string
		_expectedVersion string

		isOursFresher bool
		merge         *ChangelogMerge
	}

	baseRow := mergeRow("2000-01-01", "1.0.0", "base")
	base := provideChangelogSkill(baseRow)
	baseBytes := string(encodedSkill(t, base))
	oursPatch := mergeRow("2000-01-02", "1.0.1", "ours patch")
	theirsMinor := mergeRow("2000-01-02", "1.1.0", "theirs minor")
	ours := provideChangelogSkill(oursPatch, baseRow)
	theirs := provideChangelogSkill(theirsMinor, baseRow)
	tests := make([]*testCase, 0)

	// ours-fresher-patch-over-theirs-minor
	tests = append(tests, &testCase{
		_id:              "ours-fresher-patch-over-theirs-minor",
		_expectedBase:    baseBytes,
		_expectedOurs:    baseBytes,
		_expectedRows:    []ChangelogEntry{mergeRow("2000-01-02", "1.1.1", "ours patch"), theirsMinor, baseRow},
		_expectedTheirs:  baseBytes,
		_expectedVersion: "1.1.1",

		isOursFresher: true,
		merge:         parsedMerge(t, base, ours, theirs),
	})

	// theirs-fresher-on-top
	tests = append(tests, &testCase{
		_id:              "theirs-fresher-on-top",
		_expectedBase:    baseBytes,
		_expectedOurs:    baseBytes,
		_expectedRows:    []ChangelogEntry{theirsMinor, oursPatch, baseRow},
		_expectedTheirs:  baseBytes,
		_expectedVersion: "1.1.0",

		isOursFresher: false,
		merge:         parsedMerge(t, base, ours, theirs),
	})

	// same-row-on-both-sides-once
	shared := provideChangelogSkill(oursPatch, baseRow)
	tests = append(tests, &testCase{
		_id:              "same-row-on-both-sides-once",
		_expectedBase:    baseBytes,
		_expectedOurs:    baseBytes,
		_expectedRows:    []ChangelogEntry{oursPatch, baseRow},
		_expectedTheirs:  baseBytes,
		_expectedVersion: "1.0.1",

		isOursFresher: true,
		merge:         parsedMerge(t, base, shared, shared),
	})

	// two-rows-on-one-side-keep-order-and-kinds
	oursMinor := mergeRow("2000-01-03", "1.1.0", "ours minor")
	twoRows := provideChangelogSkill(oursMinor, oursPatch, baseRow)
	tests = append(tests, &testCase{
		_id:           "two-rows-on-one-side-keep-order-and-kinds",
		_expectedBase: baseBytes,
		_expectedOurs: baseBytes,
		_expectedRows: []ChangelogEntry{
			mergeRow("2000-01-03", "1.2.0", "ours minor"),
			mergeRow("2000-01-02", "1.1.1", "ours patch"),
			theirsMinor,
			baseRow,
		},
		_expectedTheirs:  baseBytes,
		_expectedVersion: "1.2.0",

		isOursFresher: true,
		merge:         parsedMerge(t, base, twoRows, theirs),
	})

	// two-part-base
	twoPartRow := mergeRow("2000-01-01", "1.7", "base")
	twoPartBase := provideChangelogSkill(twoPartRow)
	twoPartBytes := string(encodedSkill(t, twoPartBase))
	twoPartOurs := provideChangelogSkill(mergeRow("2000-01-02", "1.7.1", "ours patch"), twoPartRow)
	twoPartTheirs := provideChangelogSkill(mergeRow("2000-01-02", "1.8", "theirs minor"), twoPartRow)
	tests = append(tests, &testCase{
		_id:           "two-part-base",
		_expectedBase: twoPartBytes,
		_expectedOurs: twoPartBytes,
		_expectedRows: []ChangelogEntry{
			mergeRow("2000-01-02", "1.8.1", "ours patch"),
			mergeRow("2000-01-02", "1.8", "theirs minor"),
			twoPartRow,
		},
		_expectedTheirs:  twoPartBytes,
		_expectedVersion: "1.8.1",

		isOursFresher: true,
		merge:         parsedMerge(t, twoPartBase, twoPartOurs, twoPartTheirs),
	})

	// stripped-sides-carry-base-rows-and-version
	noted := provideChangelogSkill(oursPatch, baseRow)
	noted.Note = "ours note"
	notedStripped := provideChangelogSkill(baseRow)
	notedStripped.Note = "ours note"
	tests = append(tests, &testCase{
		_id:              "stripped-sides-carry-base-rows-and-version",
		_expectedBase:    baseBytes,
		_expectedOurs:    string(encodedSkill(t, notedStripped)),
		_expectedRows:    []ChangelogEntry{mergeRow("2000-01-02", "1.1.1", "ours patch"), theirsMinor, baseRow},
		_expectedTheirs:  baseBytes,
		_expectedVersion: "1.1.1",

		isOursFresher: true,
		merge:         parsedMerge(t, base, noted, theirs),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			merged, err := test.merge.Resolve(test.isOursFresher)
			assert.NoError(t, err)
			assert.Equal(t, test._expectedRows, merged.Rows)
			assert.Equal(t, test._expectedVersion, merged.Version)
			assert.Equal(t, test._expectedBase, string(merged.Base))
			assert.Equal(t, test._expectedOurs, string(merged.Ours))
			assert.Equal(t, test._expectedTheirs, string(merged.Theirs))
		})
	}
}

func TestApplyChangelog(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   string
		_shouldPass bool

		data    []byte
		rows    []ChangelogEntry
		version string
	}

	baseRow := mergeRow("2000-01-01", "1.0", "fixture")
	rows := []ChangelogEntry{mergeRow("2000-01-02", "1.1", "merged"), baseRow}
	tests := make([]*testCase, 0)

	// pass-rows-and-version-applied
	tests = append(tests, &testCase{
		_id:         "pass-rows-and-version-applied",
		_expected:   string(encodedSkill(t, provideChangelogSkill(rows...))),
		_shouldPass: true,

		data:    encodedSkill(t, provideCompleteSkill()),
		rows:    rows,
		version: "1.1",
	})

	// fail-undecodable
	tests = append(tests, &testCase{
		_id:         "fail-undecodable",
		_expected:   "",
		_shouldPass: false,

		data:    []byte("not cue {"),
		rows:    rows,
		version: "1.1",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			applied, err := ApplyChangelog(test.data, test.rows, test.version)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, string(applied))
		})
	}
}

// newestDates is the pair OursNewest, TheirsNewest, empty for a failed parse.
func newestDates(merge *ChangelogMerge) [2]string {
	if merge == nil {
		return [2]string{}
	}
	return [2]string{merge.OursNewest, merge.TheirsNewest}
}

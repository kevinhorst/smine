package contextdocs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// resolveEntry is a minimal entry with an id and a reach.
func resolveEntry(id, reachValue string) RuleEntry {
	return RuleEntry{Id: id, Content: RuleContent{Statement: id + " statement"}, Reach: reachValue}
}

// resolvedIds lists the entries' ids in order.
func resolvedIds(entries []RuleEntry) []string {
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.Id)
	}
	return ids
}

func TestResolveGlobs(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		file  *ContextFile
		globs []string
	}

	index := &ContextFile{Entries: []RuleEntry{
		resolveEntry("RULE-GO-002", ""),
		resolveEntry("ACTION-IMPL-001", ""),
		resolveEntry("RULE-GO-001", ""),
		resolveEntry("FACT-REPO-001", ""),
	}}
	tests := make([]*testCase, 0)

	// exact-id
	tests = append(tests, &testCase{
		_id:       "exact-id",
		_expected: []string{"ACTION-IMPL-001"},

		file:  index,
		globs: []string{"ACTION-IMPL-001"},
	})

	// prefix-glob
	tests = append(tests, &testCase{
		_id:       "prefix-glob",
		_expected: []string{"RULE-GO-001", "RULE-GO-002"},

		file:  index,
		globs: []string{"RULE-*"},
	})

	// two-globs-union
	tests = append(tests, &testCase{
		_id:       "two-globs-union",
		_expected: []string{"FACT-REPO-001", "RULE-GO-001", "RULE-GO-002"},

		file:  index,
		globs: []string{"RULE-*", "FACT-*"},
	})

	// duplicate-id-first-wins
	first := resolveEntry("RULE-GO-001", "")
	first.Content.Statement = "first"
	second := resolveEntry("RULE-GO-001", "")
	second.Content.Statement = "second"
	duplicates := &ContextFile{Entries: []RuleEntry{first, second}}
	tests = append(tests, &testCase{
		_id:       "duplicate-id-first-wins",
		_expected: []string{"RULE-GO-001"},

		file:  duplicates,
		globs: []string{"RULE-*"},
	})

	// unreachable-entry-dropped
	stamped := &ContextFile{
		Entries: []RuleEntry{resolveEntry("RULE-GO-001", "none"), resolveEntry("RULE-GO-002", "")},
		Self:    "repo",
	}
	tests = append(tests, &testCase{
		_id:       "unreachable-entry-dropped",
		_expected: []string{"RULE-GO-002"},

		file:  stamped,
		globs: []string{"RULE-*"},
	})

	// no-match-empty
	tests = append(tests, &testCase{
		_id:       "no-match-empty",
		_expected: []string{},

		file:  index,
		globs: []string{"SKILL-*"},
	})

	// empty-globs-empty
	tests = append(tests, &testCase{
		_id:       "empty-globs-empty",
		_expected: []string{},

		file:  index,
		globs: nil,
	})

	// sorted-by-id
	tests = append(tests, &testCase{
		_id:       "sorted-by-id",
		_expected: []string{"ACTION-IMPL-001", "FACT-REPO-001", "RULE-GO-001", "RULE-GO-002"},

		file:  index,
		globs: []string{"*"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, resolvedIds(ResolveGlobs(test.file, test.globs)))
		})
	}

	// duplicate-id-first-wins keeps the first body
	resolved := ResolveGlobs(duplicates, []string{"RULE-*"})
	assert.Equal(t, "first", resolved[0].Content.Statement)
}

func TestRenderBodies(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		entries []RuleEntry
	}

	tests := make([]*testCase, 0)

	// statement-only
	tests = append(tests, &testCase{
		_id:       "statement-only",
		_expected: "**RULE-A-001** — Do x.",

		entries: []RuleEntry{{Id: "RULE-A-001", Content: RuleContent{Statement: "Do x."}}},
	})

	// enforcement-tag
	tests = append(tests, &testCase{
		_id:       "enforcement-tag",
		_expected: "**RULE-A-001** `[gate]` — Do x.",

		entries: []RuleEntry{{Id: "RULE-A-001", Content: RuleContent{Statement: "Do x."}, Enforcement: "gate"}},
	})

	// tagged-fields-order
	tagged := RuleContent{Applies: "a", Evidence: "e", Location: "l", Statement: "Do x.", Why: "w"}
	tests = append(tests, &testCase{
		_id:       "tagged-fields-order",
		_expected: "**RULE-A-001** — Do x.\n\n* Why: w\n* Applies: a\n* Evidence: e\n* Location: l",

		entries: []RuleEntry{{Id: "RULE-A-001", Content: tagged}},
	})

	// bullets-and-notes
	noted := RuleContent{Bullets: []string{"b1", "b2"}, Notes: []string{"n1"}, Statement: "Do x."}
	tests = append(tests, &testCase{
		_id:       "bullets-and-notes",
		_expected: "**RULE-A-001** — Do x.\n\n* b1\n* b2\n\nn1",

		entries: []RuleEntry{{Id: "RULE-A-001", Content: noted}},
	})

	// example-fence
	example := RuleContent{Examples: []RuleExample{{Code: "x := 1", Lang: "go"}}, Statement: "Do x."}
	tests = append(tests, &testCase{
		_id:       "example-fence",
		_expected: "**RULE-A-001** — Do x.\n\n\n```go\nx := 1\n```",

		entries: []RuleEntry{{Id: "RULE-A-001", Content: example}},
	})

	// two-entries-blank-separated
	tests = append(tests, &testCase{
		_id:       "two-entries-blank-separated",
		_expected: "**RULE-A-001** — Do x.\n\n\n**RULE-A-002** — Do y.",

		entries: []RuleEntry{
			{Id: "RULE-A-001", Content: RuleContent{Statement: "Do x."}},
			{Id: "RULE-A-002", Content: RuleContent{Statement: "Do y."}},
		},
	})

	// empty-entries-empty-string
	tests = append(tests, &testCase{
		_id:       "empty-entries-empty-string",
		_expected: "",

		entries: nil,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, RenderBodies(test.entries))
		})
	}
}

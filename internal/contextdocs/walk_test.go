package contextdocs

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func provideWalkEntry(id, enforcement, source string) RuleEntry {
	return RuleEntry{Id: id, Enforcement: enforcement, Source: source}
}

func provideTestFamily(count int) []RuleEntry {
	entries := make([]RuleEntry, 0, count)
	for i := 1; i <= count; i++ {
		entries = append(entries, provideWalkEntry(fmt.Sprintf("RULE-GOLANG-TEST-%03d", i), channelProse, "rules/go.md"))
	}
	return entries
}

func TestBuildWalk(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *Walk
		_shouldPass bool

		request *WalkRequest
		set     RuleSet
	}

	goGuide := RuleGuide{Files: []string{"*.go"}, Name: "go", Path: "rules/go.md", Source: "rules/go.md"}
	reachEntries := []RuleEntry{
		{Id: "ACTION-REVIEW-TEST-001", Enforcement: channelProse, Reach: "global", Source: "actions/reviewing.md"},
		{Id: "FACT-REPO-TEST-001", Enforcement: "", Reach: "backend", Source: "facts/claude-configs.md"},
		{Id: "FACT-REPO-TEST-002", Enforcement: "", Reach: "smine", Source: "facts/claude-configs.md"},
		{Id: "FACT-REPO-TEST-003", Enforcement: "", Reach: "", Source: "facts/claude-configs.md"},
		{Id: "RULE-GOLANG-NAME-001", Enforcement: channelProse, Reach: "global", Source: "rules/go.md"},
		{Id: "RULE-GOLANG-NAME-002", Enforcement: channelProse, Reach: "go", Source: "rules/go.md"},
	}
	sourceSet := RuleSet{Entries: reachEntries, Guides: []RuleGuide{goGuide}, Self: "smine"}
	deployedSet := RuleSet{Entries: reachEntries, Guides: []RuleGuide{goGuide}, Self: ""}
	qualitySet := RuleSet{Entries: []RuleEntry{
		provideWalkEntry("ACTION-REVIEW-QUALITY-001", channelProse, "actions/reviewing.md"),
		provideWalkEntry("ACTION-REVIEW-QUALITY-002", channelProse, "actions/reviewing.md"),
		provideWalkEntry("ACTION-REVIEW-SPEC-001", channelProse, "actions/reviewing.md"),
	}}
	guideSet := RuleSet{
		Entries: []RuleEntry{
			provideWalkEntry("ACTION-REVIEW-QUALITY-001", channelProse, "actions/reviewing.md"),
			provideWalkEntry("RULE-GOLANG-NAME-001", channelProse, "rules/go.md"),
			provideWalkEntry("RULE-GOLANG-NAME-002", "", "rules/go.md"),
		},
		Guides: []RuleGuide{goGuide},
	}

	tests := make([]*testCase, 0)

	// families
	tests = append(tests, &testCase{
		_id: "families",
		_expected: &Walk{Shape: WalkShapeFileFirst, Groups: []WalkGroup{
			{Id: "ACTION-REVIEW-QUALITY", Items: []string{"ACTION-REVIEW-QUALITY-001", "ACTION-REVIEW-QUALITY-002"}},
		}},
		_shouldPass: true,

		request: &WalkRequest{Ceiling: 10, Direction: "code-style", Tokens: []string{"ACTION-REVIEW-QUALITY-*"}},
		set:     qualitySet,
	})

	// ceiling-split
	tests = append(tests, &testCase{
		_id: "ceiling-split",
		_expected: &Walk{Shape: WalkShapeFileFirst, Groups: []WalkGroup{
			{Id: "RULE-GOLANG-TEST/1", Items: []string{"RULE-GOLANG-TEST-001", "RULE-GOLANG-TEST-002", "RULE-GOLANG-TEST-003", "RULE-GOLANG-TEST-004", "RULE-GOLANG-TEST-005"}},
			{Id: "RULE-GOLANG-TEST/2", Items: []string{"RULE-GOLANG-TEST-006", "RULE-GOLANG-TEST-007", "RULE-GOLANG-TEST-008", "RULE-GOLANG-TEST-009", "RULE-GOLANG-TEST-010"}},
			{Id: "RULE-GOLANG-TEST/3", Items: []string{"RULE-GOLANG-TEST-011", "RULE-GOLANG-TEST-012"}},
		}},
		_shouldPass: true,

		request: &WalkRequest{Ceiling: 5, Direction: "tests", Tokens: []string{"RULE-GOLANG-TEST-*"}},
		set:     RuleSet{Entries: provideTestFamily(12)},
	})

	// enforcement-filter
	tests = append(tests, &testCase{
		_id: "enforcement-filter",
		_expected: &Walk{Shape: WalkShapeFileFirst, Groups: []WalkGroup{
			{Id: "RULE-GOLANG-FUNC", Items: []string{"RULE-GOLANG-FUNC-002", "RULE-GOLANG-FUNC-003"}},
		}},
		_shouldPass: true,

		request: &WalkRequest{Ceiling: 10, Direction: "tests", Tokens: []string{"RULE-GOLANG-FUNC-*"}},
		set: RuleSet{Entries: []RuleEntry{
			provideWalkEntry("RULE-GOLANG-FUNC-001", "lint", "rules/go.md"),
			provideWalkEntry("RULE-GOLANG-FUNC-002", channelProse, "rules/go.md"),
			provideWalkEntry("RULE-GOLANG-FUNC-003", "", "rules/go.md"),
			provideWalkEntry("RULE-GOLANG-FUNC-004", "gate", "rules/go.md"),
		}},
	})

	// guides-code-style
	tests = append(tests, &testCase{
		_id: "guides-code-style",
		_expected: &Walk{Shape: WalkShapeFileFirst, Groups: []WalkGroup{
			{Id: "ACTION-REVIEW-QUALITY", Items: []string{"ACTION-REVIEW-QUALITY-001"}},
			{Id: "RULE-GOLANG-NAME", Items: []string{"RULE-GOLANG-NAME-001", "RULE-GOLANG-NAME-002"}},
		}},
		_shouldPass: true,

		request: &WalkRequest{Ceiling: 10, Direction: "code-style", Files: []string{"internal/a.go"}, Tokens: []string{"ACTION-REVIEW-QUALITY-*"}},
		set:     guideSet,
	})

	// guides-other-direction
	tests = append(tests, &testCase{
		_id: "guides-other-direction",
		_expected: &Walk{Shape: WalkShapeFileFirst, Groups: []WalkGroup{
			{Id: "ACTION-REVIEW-QUALITY", Items: []string{"ACTION-REVIEW-QUALITY-001"}},
		}},
		_shouldPass: true,

		request: &WalkRequest{Ceiling: 10, Direction: "tests", Files: []string{"internal/a.go"}, Tokens: []string{"ACTION-REVIEW-QUALITY-*"}},
		set:     guideSet,
	})

	// guides-no-match
	tests = append(tests, &testCase{
		_id: "guides-no-match",
		_expected: &Walk{Shape: WalkShapeFileFirst, Groups: []WalkGroup{
			{Id: "ACTION-REVIEW-QUALITY", Items: []string{"ACTION-REVIEW-QUALITY-001"}},
		}},
		_shouldPass: true,

		request: &WalkRequest{Ceiling: 10, Direction: "code-style", Files: []string{"x.py"}, Tokens: []string{"ACTION-REVIEW-QUALITY-*"}},
		set:     guideSet,
	})

	// guide-token-without-governed-file
	tests = append(tests, &testCase{
		_id: "guide-token-without-governed-file",
		_expected: &Walk{Shape: WalkShapeFileFirst, Groups: []WalkGroup{
			{Id: "ACTION-REVIEW-QUALITY", Items: []string{"ACTION-REVIEW-QUALITY-001"}},
		}},
		_shouldPass: true,

		request: &WalkRequest{Ceiling: 10, Direction: "code-style", Files: []string{"x.py"}, Tokens: []string{"ACTION-REVIEW-QUALITY-*", "RULE-GOLANG-*"}},
		set:     guideSet,
	})

	// unmapped-direction
	tests = append(tests, &testCase{
		_id:         "unmapped-direction",
		_expected:   &Walk{Shape: WalkShapeFileFirst, Groups: []WalkGroup{}},
		_shouldPass: true,

		request: &WalkRequest{Ceiling: 10, Direction: "special-focus"},
		set:     qualitySet,
	})

	// ceiling-zero
	tests = append(tests, &testCase{
		_id:         "ceiling-zero",
		_expected:   nil,
		_shouldPass: false,

		request: &WalkRequest{Ceiling: 0, Direction: "tests", Tokens: []string{"ACTION-REVIEW-QUALITY-*"}},
		set:     qualitySet,
	})

	// missing-direction
	tests = append(tests, &testCase{
		_id:         "missing-direction",
		_expected:   nil,
		_shouldPass: false,

		request: &WalkRequest{Ceiling: 10, Tokens: []string{"ACTION-REVIEW-QUALITY-*"}},
		set:     qualitySet,
	})

	// reach-filtered
	tests = append(tests, &testCase{
		_id: "reach-filtered",
		_expected: &Walk{Shape: WalkShapeFileFirst, Groups: []WalkGroup{
			{Id: "ACTION-REVIEW-TEST", Items: []string{"ACTION-REVIEW-TEST-001"}},
			{Id: "FACT-REPO-TEST", Items: []string{"FACT-REPO-TEST-002", "FACT-REPO-TEST-003"}},
		}},
		_shouldPass: true,

		request: &WalkRequest{Ceiling: 10, Direction: "tests", Tokens: []string{"ACTION-REVIEW-TEST-*", "FACT-REPO-TEST-*"}},
		set:     sourceSet,
	})

	// reach-filtered-guide-rules
	tests = append(tests, &testCase{
		_id: "reach-filtered-guide-rules",
		_expected: &Walk{Shape: WalkShapeFileFirst, Groups: []WalkGroup{
			{Id: "ACTION-REVIEW-TEST", Items: []string{"ACTION-REVIEW-TEST-001"}},
			{Id: "RULE-GOLANG-NAME", Items: []string{"RULE-GOLANG-NAME-001"}},
		}},
		_shouldPass: true,

		request: &WalkRequest{Ceiling: 10, Direction: "code-style", Files: []string{"internal/a.go"}, Tokens: []string{"ACTION-REVIEW-TEST-*"}},
		set:     sourceSet,
	})

	// reach-unfiltered-without-self
	tests = append(tests, &testCase{
		_id: "reach-unfiltered-without-self",
		_expected: &Walk{Shape: WalkShapeFileFirst, Groups: []WalkGroup{
			{Id: "ACTION-REVIEW-TEST", Items: []string{"ACTION-REVIEW-TEST-001"}},
			{Id: "FACT-REPO-TEST", Items: []string{"FACT-REPO-TEST-001", "FACT-REPO-TEST-002", "FACT-REPO-TEST-003"}},
		}},
		_shouldPass: true,

		request: &WalkRequest{Ceiling: 10, Direction: "tests", Tokens: []string{"ACTION-REVIEW-TEST-*", "FACT-REPO-TEST-*"}},
		set:     deployedSet,
	})

	// reach-leaves-nothing
	tests = append(tests, &testCase{
		_id:         "reach-leaves-nothing",
		_expected:   &Walk{Shape: WalkShapeFileFirst, Groups: []WalkGroup{}},
		_shouldPass: true,

		request: &WalkRequest{Ceiling: 10, Direction: "tests", Tokens: []string{"FACT-REPO-TEST-001"}},
		set:     sourceSet,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			walk, err := BuildWalk(test.set, test.request)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, walk)
		})
	}
}

func TestGlobMatches(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		file string
		glob string
	}

	tests := make([]*testCase, 0)

	// star-spans-directories
	tests = append(tests, &testCase{
		_id:       "star-spans-directories",
		_expected: true,

		file: "a/b/c.go",
		glob: "*.go",
	})

	// double-star-skill
	tests = append(tests, &testCase{
		_id:       "double-star-skill",
		_expected: true,

		file: "skills/q/foo/SKILL.md",
		glob: "skills/**/SKILL.md",
	})

	// extension-mismatch
	tests = append(tests, &testCase{
		_id:       "extension-mismatch",
		_expected: false,

		file: "c.go",
		glob: "*.py",
	})

	// question-one-char
	tests = append(tests, &testCase{
		_id:       "question-one-char",
		_expected: true,

		file: "a.go",
		glob: "?.go",
	})

	// question-two-chars
	tests = append(tests, &testCase{
		_id:       "question-two-chars",
		_expected: false,

		file: "ab.go",
		glob: "?.go",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, GlobMatches(test.glob, test.file))
		})
	}
}

func TestLoadLaneMap(t *testing.T) {
	t.Run("reads-lanes-by-direction", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "context-map.json")
		mapJson := `{"schemaVersion": "1.0", "skill": "railroad-review", "lanes": {"tests": ["ACTION-REVIEW-TEST-*"], "spec": ["ACTION-REVIEW-SPEC-001"]}}`
		require.NoError(t, os.WriteFile(path, []byte(mapJson), 0o644))
		laneMap, err := LoadLaneMap(path)
		require.NoError(t, err)
		assert.Equal(t, []string{"ACTION-REVIEW-TEST-*"}, laneMap.Lanes["tests"])
		assert.Equal(t, []string{"ACTION-REVIEW-SPEC-001"}, laneMap.Lanes["spec"])
	})

	t.Run("missing-file-errors", func(t *testing.T) {
		_, err := LoadLaneMap(filepath.Join(t.TempDir(), "context-map.json"))
		require.Error(t, err)
		assert.ErrorIs(t, err, os.ErrNotExist)
	})
}

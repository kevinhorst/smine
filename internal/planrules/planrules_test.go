package planrules

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testFeatureFolder = "plans/2026-10-01-probe-7ea30b5"
	testPlanDir       = testFeatureFolder + "/2026-10-01-probe-wip-7ea30b5"
	testPlanFile      = testPlanDir + "/design/probe.md"
	otherPlanDir      = "plans/2026-10-01-other-1a2b3c4/2026-10-01-other-wip-1a2b3c4"
)

// testRecordLine is one ledger line of plan in repo holding one rule.
func testRecordLine(plan string, repo string, rule string) string {
	return `{"plan":"` + plan + `","repo":"` + repo + `","session_id":"s","ts":"t","checked":[],"rules":[{"needed_for":"V1","rule":"` + rule + `","target":"x"}]}`
}

func writeTestFile(t *testing.T, path string, content string) string {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestFile_Equals(t *testing.T) {
	lsofRule := &Rule{NeededFor: "V1", Rule: "Bash(lsof *)", Target: "lsof -i :6101"}
	jqRule := &Rule{NeededFor: "V2", Rule: "Bash(jq *)", Target: "jq . x.json"}
	base := &File{Checked: []string{"go test ./..."}, Rules: []*Rule{lsofRule, jqRule}}

	// same-content
	sameContent := &File{
		Checked: []string{"go test ./..."},
		Rules:   []*Rule{{NeededFor: "V1", Rule: "Bash(lsof *)", Target: "lsof -i :6101"}, {NeededFor: "V2", Rule: "Bash(jq *)", Target: "jq . x.json"}},
	}
	assert.True(t, base.Equals(sameContent))

	// rule-differs
	ruleDiffers := &File{
		Checked: []string{"go test ./..."},
		Rules:   []*Rule{lsofRule, {NeededFor: "V2", Rule: "Bash(jq -c *)", Target: "jq . x.json"}},
	}
	assert.False(t, base.Equals(ruleDiffers))

	// rule-order-differs
	ruleOrderDiffers := &File{Checked: []string{"go test ./..."}, Rules: []*Rule{jqRule, lsofRule}}
	assert.False(t, base.Equals(ruleOrderDiffers))

	// checked-differs
	checkedDiffers := &File{Checked: []string{"go vet ./..."}, Rules: []*Rule{lsofRule, jqRule}}
	assert.False(t, base.Equals(checkedDiffers))

	// nil-and-empty-lists
	nilLists := &File{}
	emptyLists := &File{Checked: []string{}, Rules: []*Rule{}}
	assert.True(t, nilLists.Equals(emptyLists))
}

func TestLoadFile(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *File
		_isNotExist bool
		_shouldPass bool

		path string
	}

	dir := t.TempDir()
	tests := make([]*testCase, 0)

	// pass-rules-and-checked
	rulesAndChecked := &File{
		Checked: []string{"go build ./..."},
		Rules:   []*Rule{{NeededFor: "V1", Rule: "Bash(lsof *)", Target: "lsof -i :6101"}},
	}
	tests = append(tests, &testCase{
		_id:         "pass-rules-and-checked",
		_expected:   rulesAndChecked,
		_isNotExist: false,
		_shouldPass: true,

		path: writeTestFile(t, filepath.Join(dir, "rules.json"), `{"checked":["go build ./..."],"rules":[{"needed_for":"V1","rule":"Bash(lsof *)","target":"lsof -i :6101"}]}`),
	})

	// pass-empty-lists
	emptyLists := &File{Checked: []string{}, Rules: []*Rule{}}
	tests = append(tests, &testCase{
		_id:         "pass-empty-lists",
		_expected:   emptyLists,
		_isNotExist: false,
		_shouldPass: true,

		path: writeTestFile(t, filepath.Join(dir, "empty.json"), `{"checked":[],"rules":[]}`),
	})

	// fail-unknown-key
	tests = append(tests, &testCase{
		_id:         "fail-unknown-key",
		_expected:   nil,
		_isNotExist: false,
		_shouldPass: false,

		path: writeTestFile(t, filepath.Join(dir, "unknown.json"), `{"checked":[],"rules":[{"rule":"Bash(lsof *)","targt":"lsof"}]}`),
	})

	// fail-malformed
	tests = append(tests, &testCase{
		_id:         "fail-malformed",
		_expected:   nil,
		_isNotExist: false,
		_shouldPass: false,

		path: writeTestFile(t, filepath.Join(dir, "malformed.json"), `{"checked":[`),
	})

	// fail-null-rule
	tests = append(tests, &testCase{
		_id:         "fail-null-rule",
		_expected:   nil,
		_isNotExist: false,
		_shouldPass: false,

		path: writeTestFile(t, filepath.Join(dir, "null.json"), `{"checked":[],"rules":[null]}`),
	})

	// fail-missing-file
	tests = append(tests, &testCase{
		_id:         "fail-missing-file",
		_expected:   nil,
		_isNotExist: true,
		_shouldPass: false,

		path: filepath.Join(dir, "missing.json"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			file, err := LoadFile(test.path)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._isNotExist, errors.Is(err, os.ErrNotExist))
			assert.Equal(t, test._expected, file)
		})
	}
}

func TestLoadRecords(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedRules []string

		ledger string
		repo   string
	}

	repo, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	otherRepo := t.TempDir()
	linkParent := t.TempDir()
	repoLink := filepath.Join(linkParent, "repo-link")
	require.NoError(t, os.Symlink(repo, repoLink))

	dir := t.TempDir()
	tests := make([]*testCase, 0)

	// two-of-repo-one-of-other
	twoOfRepo := strings.Join([]string{
		testRecordLine(testPlanDir, repo, "Bash(a *)"),
		testRecordLine(testPlanDir, otherRepo, "Bash(b *)"),
		testRecordLine(otherPlanDir, repo, "Bash(c *)"),
	}, "\n") + "\n"
	tests = append(tests, &testCase{
		_id:            "two-of-repo-one-of-other",
		_expectedRules: []string{"Bash(a *)", "Bash(c *)"},

		ledger: writeTestFile(t, filepath.Join(dir, "two.jsonl"), twoOfRepo),
		repo:   repo,
	})

	// symlinked-repo-path
	symlinked := testRecordLine(testPlanDir, repoLink, "Bash(a *)") + "\n"
	tests = append(tests, &testCase{
		_id:            "symlinked-repo-path",
		_expectedRules: []string{"Bash(a *)"},

		ledger: writeTestFile(t, filepath.Join(dir, "symlinked.jsonl"), symlinked),
		repo:   repo,
	})

	// missing-ledger
	tests = append(tests, &testCase{
		_id:            "missing-ledger",
		_expectedRules: []string{},

		ledger: filepath.Join(dir, "missing.jsonl"),
		repo:   repo,
	})

	// unparseable-line-skipped
	unparseable := "{not json\n" + testRecordLine(testPlanDir, repo, "Bash(a *)") + "\n"
	tests = append(tests, &testCase{
		_id:            "unparseable-line-skipped",
		_expectedRules: []string{"Bash(a *)"},

		ledger: writeTestFile(t, filepath.Join(dir, "unparseable.jsonl"), unparseable),
		repo:   repo,
	})

	// line-without-rules-skipped
	withoutRules := `{"plan":"` + testPlanDir + `","repo":"` + repo + `","session_id":"s","ts":"t"}` + "\n" + testRecordLine(testPlanDir, repo, "Bash(a *)") + "\n"
	tests = append(tests, &testCase{
		_id:            "line-without-rules-skipped",
		_expectedRules: []string{"Bash(a *)"},

		ledger: writeTestFile(t, filepath.Join(dir, "without.jsonl"), withoutRules),
		repo:   repo,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			records := LoadRecords(test.ledger, test.repo)
			assert.Equal(t, test._expectedRules, recordRules(records))
		})
	}
}

// recordRules are the rule texts of records in order.
func recordRules(records []*Record) []string {
	rules := make([]string, 0)
	for _, record := range records {
		rules = append(rules, record.RuleStrings()...)
	}
	return rules
}

func TestLatest(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *Record

		plan    string
		records []*Record
	}

	older := &Record{File: &File{}, Plan: testPlanDir, Timestamp: "1"}
	newer := &Record{File: &File{}, Plan: testPlanDir, Timestamp: "2"}
	other := &Record{File: &File{}, Plan: otherPlanDir, Timestamp: "3"}
	tests := make([]*testCase, 0)

	// latest-of-two
	tests = append(tests, &testCase{
		_id:       "latest-of-two",
		_expected: newer,

		plan:    testPlanDir,
		records: []*Record{older, newer, other},
	})

	// other-plan
	tests = append(tests, &testCase{
		_id:       "other-plan",
		_expected: other,

		plan:    otherPlanDir,
		records: []*Record{older, other, newer},
	})

	// never-frozen
	tests = append(tests, &testCase{
		_id:       "never-frozen",
		_expected: nil,

		plan:    "plans/2026-10-01-never-0000000/2026-10-01-never-wip-0000000",
		records: []*Record{older, newer, other},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Same(t, test._expected, Latest(test.plan, test.records))
		})
	}
}

func TestPlanDir(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		path string
	}

	tests := make([]*testCase, 0)

	// pass-feature-folder-plan-file
	tests = append(tests, &testCase{
		_id:       "pass-feature-folder-plan-file",
		_expected: testPlanDir,

		path: testPlanFile,
	})

	// fail-old-depth
	tests = append(tests, &testCase{
		_id:       "fail-old-depth",
		_expected: "",

		path: "plans/2026-10-01-probe-wip-7ea30b5/design/probe.md",
	})

	// fail-stem-differs-from-slug
	tests = append(tests, &testCase{
		_id:       "fail-stem-differs-from-slug",
		_expected: "",

		path: testPlanDir + "/design/notes.md",
	})

	// fail-done-dir
	tests = append(tests, &testCase{
		_id:       "fail-done-dir",
		_expected: "",

		path: testFeatureFolder + "/2026-10-01-probe-done-7ea30b5/design/probe.md",
	})

	// fail-folder-without-date
	tests = append(tests, &testCase{
		_id:       "fail-folder-without-date",
		_expected: "",

		path: "plans/probe-7ea30b5/2026-10-01-probe-wip-7ea30b5/design/probe.md",
	})

	// fail-three-levels-deep
	tests = append(tests, &testCase{
		_id:       "fail-three-levels-deep",
		_expected: "",

		path: testFeatureFolder + "/2026-10-01-nested-1a2b3c4/2026-10-01-probe-wip-7ea30b5/design/probe.md",
	})

	// design-companion
	tests = append(tests, &testCase{
		_id:       "design-companion",
		_expected: "",

		path: testPlanDir + "/design/probe/companion.md",
	})

	// plain-path
	tests = append(tests, &testCase{
		_id:       "plain-path",
		_expected: "",

		path: "docs/probe.md",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, PlanDir(test.path))
			assert.Equal(t, test._expected != "", IsPlanFile(test.path))
		})
	}
}

func TestPlanFileName(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		dir string
	}

	tests := make([]*testCase, 0)

	// plan-file
	tests = append(tests, &testCase{
		_id:       "plan-file",
		_expected: "probe.md",

		dir: testPlanDir,
	})

	// done-plan-dir
	tests = append(tests, &testCase{
		_id:       "done-plan-dir",
		_expected: "",

		dir: testFeatureFolder + "/2026-10-01-probe-done-7ea30b5",
	})

	// plain-path
	tests = append(tests, &testCase{
		_id:       "plain-path",
		_expected: "",

		dir: "docs",
	})

	// dir-with-trailing-segment
	tests = append(tests, &testCase{
		_id:       "dir-with-trailing-segment",
		_expected: "",

		dir: testPlanDir + "/design",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, PlanFileName(test.dir))
		})
	}
}

func TestIsPlanDir(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		dir string
	}

	tests := make([]*testCase, 0)

	// plan-file
	tests = append(tests, &testCase{
		_id:       "plan-file",
		_expected: true,

		dir: testPlanDir,
	})

	// done-plan-dir
	tests = append(tests, &testCase{
		_id:       "done-plan-dir",
		_expected: false,

		dir: testFeatureFolder + "/2026-10-01-probe-done-7ea30b5",
	})

	// plain-path
	tests = append(tests, &testCase{
		_id:       "plain-path",
		_expected: false,

		dir: "docs",
	})

	// dir-with-trailing-segment
	tests = append(tests, &testCase{
		_id:       "dir-with-trailing-segment",
		_expected: false,

		dir: testPlanDir + "/design",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, IsPlanDir(test.dir))
		})
	}
}

func TestViewLines(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		file *File
	}

	title := "**Plan rules** — from permissions.json, shown in this display copy only"
	tableHead := []string{"| Rule | Command | Needed for |", "|---|---|---|"}
	tests := make([]*testCase, 0)

	// two-rules
	twoRules := &File{
		Checked: []string{"go build ./...", "go vet ./..."},
		Rules: []*Rule{
			{NeededFor: "V1", Rule: "Bash(lsof *)", Target: "lsof -i :6101"},
			{NeededFor: "V2", Rule: "Bash(jq *)", Target: "jq . x.json"},
		},
	}
	twoRulesExpected := append([]string{title, ""}, tableHead...)
	twoRulesExpected = append(
		twoRulesExpected,
		"| `Bash(lsof *)` | `lsof -i :6101` | V1 |",
		"| `Bash(jq *)` | `jq . x.json` | V2 |",
		"",
		"- checked: `go build ./...`, `go vet ./...`",
		"",
	)
	tests = append(tests, &testCase{
		_id:       "two-rules",
		_expected: twoRulesExpected,

		file: twoRules,
	})

	// no-rules
	noRules := &File{Checked: []string{"go build ./..."}, Rules: []*Rule{}}
	tests = append(tests, &testCase{
		_id:       "no-rules",
		_expected: []string{title, "", "- no rule needed", "", "- checked: `go build ./...`", ""},

		file: noRules,
	})

	// pipe-in-target-escaped
	pipeInTarget := &File{
		Checked: []string{},
		Rules:   []*Rule{{NeededFor: "V1|V2", Rule: "Bash(go test *)", Target: "go test ./... | tail"}},
	}
	pipeExpected := append([]string{title, ""}, tableHead...)
	pipeExpected = append(pipeExpected, "| `Bash(go test *)` | `go test ./... \\| tail` | V1\\|V2 |", "", "- checked: ", "")
	tests = append(tests, &testCase{
		_id:       "pipe-in-target-escaped",
		_expected: pipeExpected,

		file: pipeInTarget,
	})

	// no-checked-targets
	noChecked := &File{Checked: []string{}, Rules: []*Rule{}}
	tests = append(tests, &testCase{
		_id:       "no-checked-targets",
		_expected: []string{title, "", "- no rule needed", "", "- checked: ", ""},

		file: noChecked,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, ViewLines(test.file))
		})
	}
}

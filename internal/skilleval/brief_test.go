package skilleval

import (
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kevinhorst/smine/internal/evals"
)

// briefPlaceholderPattern matches every placeholder Brief fills; trace.md:<n>
// in the adherence brief is the judge's own notation, not a placeholder.
var briefPlaceholderPattern = regexp.MustCompile(`<(skill|task|run dir|label|worktree|runs|reference|preset skill dir|goldens\[0\]\.file|scoring|range\.baseSha|range\.headSha|range\.pathspec)>`)

// writeBriefRunDir writes a two-arm run.json, labels out of order, with the
// given golden block.
func writeBriefRunDir(t *testing.T, runDir string, golden *ManifestGolden) {
	t.Helper()
	manifest := &RunManifest{
		Arms: []ManifestArm{
			{Id: "opus-high-r1", Key: "opus-high", Label: "B", Model: "opus", Replica: 1, Worktree: "/wt/run-B"},
			{Id: "sonnet-high-r1", Key: "sonnet-high", Label: "A", Model: "sonnet", Replica: 1, Worktree: "/wt/run-A"},
		},
		Golden: golden,
		Prompt: "/fimplement plan.md",
		Skill:  "fimplement",
	}
	writeRunDir(t, runDir, manifest)
}

// writeGoldenTestRunDir writes a golden test run dir: run.json with a test
// golden block over judged, and the golden and scoring snapshots init copies.
func writeGoldenTestRunDir(t *testing.T, runDir string, judged *evals.DefinitionRange) {
	t.Helper()
	golden := &ManifestGolden{Id: "g-1", Kind: GoldenTest, Range: *judged, Scope: "full", SeedFile: ""}
	writeBriefRunDir(t, runDir, golden)
	writeTestFile(t, filepath.Join(runDir, goldenSnapshotFile), `{"findings": []}`)
	writeTestFile(t, filepath.Join(runDir, scoringSnapshotFile), `{"judgeGuide": "same defect"}`)
}

func TestBrief(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedSnippet string
		_isExpectedEmpty bool
		_shouldPass      bool

		kind   string
		label  string
		runDir string
	}

	base := newTempDir(t)
	t.Chdir(base)
	tests := make([]*testCase, 0)

	// adherence-filled
	adherenceRunDir := filepath.Join(base, "adherence-run")
	writeBriefRunDir(t, adherenceRunDir, nil)
	tests = append(tests, &testCase{
		_id: "adherence-filled",
		_expectedSnippet: "You audit one run of the skill fimplement against its own entries.\n" +
			"Task the run was given: /fimplement plan.md\n" +
			"Skill entries: the **SKILL-…** headlines of " + adherenceRunDir + "/SKILL.md, each with its role tag and statement.\n" +
			"Run trace, one line per text, tool call or tool error; [root] is the main loop, [agent …] a subagent: " + adherenceRunDir + "/runs/A/trace.md\n" +
			"trace.md carries no Workflow agent lines: a Workflow call shows as one [root] tool call, the work of the agents it spawns is absent.\n" +
			"The run's worktree, read-only: /wt/run-A\n",
		_isExpectedEmpty: false,
		_shouldPass:      true,

		kind:   BriefAdherence,
		label:  "A",
		runDir: "adherence-run",
	})

	// adherence-not-observable
	tests = append(tests, &testCase{
		_id: "adherence-not-observable",
		_expectedSnippet: "- not-observable: the entry runs inside a workflow agent (its role is substep, or its spawn is a Workflow agent) " +
			"and neither a trace line nor an artifact shows it; never followed on absence of contrary evidence\n",
		_isExpectedEmpty: false,
		_shouldPass:      true,

		kind:   BriefAdherence,
		label:  "A",
		runDir: "adherence-run",
	})

	// golden-produce-refused
	produceRunDir := filepath.Join(base, "produce-run")
	produceGolden := &ManifestGolden{Id: "g-1", Kind: GoldenProduce, Range: evals.DefinitionRange{}, Scope: "full", SeedFile: "eval/golden/g.seed.json"}
	writeBriefRunDir(t, produceRunDir, produceGolden)
	tests = append(tests, &testCase{
		_id:              "golden-produce-refused",
		_expectedSnippet: "",
		_isExpectedEmpty: true,
		_shouldPass:      false,

		kind:   BriefGolden,
		label:  "A",
		runDir: produceRunDir,
	})

	// golden-no-excludes
	mintedRunDir := filepath.Join(base, "minted-run")
	writeGoldenTestRunDir(t, mintedRunDir, &evals.DefinitionRange{BaseSha: "base111", HeadSha: "head222"})
	tests = append(tests, &testCase{
		_id: "golden-no-excludes",
		_expectedSnippet: "Candidates: " + mintedRunDir + "/runs/B/candidates.json\n" +
			"Golden findings and binding falsePositives rulings: " + mintedRunDir + "/golden.json\n" +
			"Matching guide: the judgeGuide of " + mintedRunDir + "/scoring.json\n" +
			"Code under review: `git -C /wt/run-B diff base111..head222`; the worktree is read-only evidence.\n",
		_isExpectedEmpty: false,
		_shouldPass:      true,

		kind:   BriefGolden,
		label:  "B",
		runDir: mintedRunDir,
	})

	// golden-pathspec-with-excludes
	excludedRunDir := filepath.Join(base, "excluded-run")
	excludedRange := &evals.DefinitionRange{BaseSha: "base111", ExcludePaths: []string{"sessions/", "plans/"}, HeadSha: "head222"}
	writeGoldenTestRunDir(t, excludedRunDir, excludedRange)
	tests = append(tests, &testCase{
		_id:              "golden-pathspec-with-excludes",
		_expectedSnippet: "Code under review: `git -C /wt/run-B diff base111..head222 -- . ':(exclude)sessions/' ':(exclude)plans/'`; the worktree is read-only evidence.\n",
		_isExpectedEmpty: false,
		_shouldPass:      true,

		kind:   BriefGolden,
		label:  "B",
		runDir: excludedRunDir,
	})

	// golden-fix-state-wording
	tests = append(tests, &testCase{
		_id:              "golden-fix-state-wording",
		_expectedSnippet: "Fix check: for every matched pair whose candidate carries \"fix_state\": \"fixed\", inspect",
		_isExpectedEmpty: false,
		_shouldPass:      true,

		kind:   BriefGolden,
		label:  "B",
		runDir: mintedRunDir,
	})

	// compare-sorted-no-reference
	compareRunDir := filepath.Join(base, "compare-run")
	writeBriefRunDir(t, compareRunDir, nil)
	tests = append(tests, &testCase{
		_id: "compare-sorted-no-reference",
		_expectedSnippet: "Task: /fimplement plan.md\n" +
			"Runs:\n" +
			"- A: diff " + compareRunDir + "/runs/A/diff.patch, worktree /wt/run-A\n" +
			"- B: diff " + compareRunDir + "/runs/B/diff.patch, worktree /wt/run-B\n" +
			"\nVerify",
		_isExpectedEmpty: false,
		_shouldPass:      true,

		kind:   BriefCompare,
		label:  "",
		runDir: compareRunDir,
	})

	// compare-reference
	referenceRunDir := filepath.Join(base, "reference-run")
	writeBriefRunDir(t, referenceRunDir, nil)
	writeTestFile(t, filepath.Join(referenceRunDir, referencePatchFile), "diff --git a/x b/x\n")
	tests = append(tests, &testCase{
		_id: "compare-reference",
		_expectedSnippet: "- B: diff " + referenceRunDir + "/runs/B/diff.patch, worktree /wt/run-B\n" +
			"Reference change, known good: " + referenceRunDir + "/reference.patch\n" +
			"\nVerify",
		_isExpectedEmpty: false,
		_shouldPass:      true,

		kind:   BriefCompare,
		label:  "",
		runDir: referenceRunDir,
	})

	// unknown-kind
	tests = append(tests, &testCase{
		_id:              "unknown-kind",
		_expectedSnippet: "",
		_isExpectedEmpty: true,
		_shouldPass:      false,

		kind:   "verdict",
		label:  "A",
		runDir: compareRunDir,
	})

	// unknown-label
	tests = append(tests, &testCase{
		_id:              "unknown-label",
		_expectedSnippet: "",
		_isExpectedEmpty: true,
		_shouldPass:      false,

		kind:   BriefAdherence,
		label:  "Z",
		runDir: compareRunDir,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			brief, err := Brief(test.kind, test.label, test.runDir)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._isExpectedEmpty, brief == "")
			assert.Contains(t, brief, test._expectedSnippet)
			assert.NotRegexp(t, briefPlaceholderPattern, brief)
		})
	}
}

package skilleval

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runManifestExample is a run.json as Init writes it, one arm carrying its own
// overlay variant.
const runManifestExample = `{
  "arms": [
    {
      "id": "opus-high-r1",
      "disabledServers": [],
      "effort": "high",
      "key": "opus-high",
      "label": "B",
      "model": "opus",
      "replica": 1,
      "worktree": "/Users/devuser/.cache/claude-routine/worktrees/skilleval-2026-09-26-a36dd5-B"
    },
    {
      "id": "sonnet-high-no-goland-r1",
      "disabledServers": ["goland"],
      "effort": "high",
      "key": "sonnet-high-no-goland",
      "label": "A",
      "model": "sonnet",
      "overlayPatch": "/Users/devuser/GolandProjects/claude-configs/evals/fimplement/2026-09-26-a36dd5/overlay-variant.patch",
      "replica": 1,
      "worktree": "/Users/devuser/.cache/claude-routine/worktrees/skilleval-2026-09-26-a36dd5-A"
    }
  ],
  "at": "a36dd5b46f542542cb58401ef7cf2d7452eb28f9",
  "budgetUsd": 30,
  "check": ["make", "audit"],
  "checkTimeoutS": 1800,
  "date": "2026-09-26",
  "judge": "opus",
  "layer": "eval",
  "mode": "nightly",
  "presetSkillDir": "",
  "prompt": "/fimplement /Users/devuser/GolandProjects/claude-configs/plans/2026-09-25-flagged-cp-prompt-wip-a36dd5b/design/raw.md",
  "reference": "9e49be5e",
  "repo": "/Users/devuser/GolandProjects/claude-configs",
  "scorer": ["make", "score"],
  "skill": "fimplement",
  "skillMd": "evals/fimplement/2026-09-26-a36dd5/SKILL.md",
  "skillVersion": "1.4.0",
  "timeoutS": 7200
}
`

// runManifestSmokeFlagExample is a run.json from before the smoke flag was
// retired.
const runManifestSmokeFlagExample = `{"date": "2026-09-26", "skill": "fimplement", "smoke": true}`

func TestLoadManifest(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *RunManifest
		_shouldPass bool

		runDir string
	}

	tests := make([]*testCase, 0)

	// current-shape
	currentRun := t.TempDir()
	writeTestFile(t, filepath.Join(currentRun, manifestFile), runManifestExample)
	tests = append(tests, &testCase{
		_id: "current-shape",
		_expected: &RunManifest{
			Arms: []ManifestArm{
				{
					Id:              "opus-high-r1",
					DisabledServers: []string{},
					Effort:          "high",
					Key:             "opus-high",
					Label:           "B",
					Model:           "opus",
					OverlayPatch:    "",
					Replica:         1,
					Worktree:        "/Users/devuser/.cache/claude-routine/worktrees/skilleval-2026-09-26-a36dd5-B",
				},
				{
					Id:              "sonnet-high-no-goland-r1",
					DisabledServers: []string{"goland"},
					Effort:          "high",
					Key:             "sonnet-high-no-goland",
					Label:           "A",
					Model:           "sonnet",
					OverlayPatch:    "/Users/devuser/GolandProjects/claude-configs/evals/fimplement/2026-09-26-a36dd5/overlay-variant.patch",
					Replica:         1,
					Worktree:        "/Users/devuser/.cache/claude-routine/worktrees/skilleval-2026-09-26-a36dd5-A",
				},
			},
			At:             "a36dd5b46f542542cb58401ef7cf2d7452eb28f9",
			BudgetUsd:      30,
			Check:          []string{"make", "audit"},
			CheckTimeoutS:  1800,
			Date:           "2026-09-26",
			Judge:          "opus",
			Layer:          LayerEval,
			Mode:           ModeNightly,
			PresetSkillDir: "",
			Prompt:         "/fimplement /Users/devuser/GolandProjects/claude-configs/plans/2026-09-25-flagged-cp-prompt-wip-a36dd5b/design/raw.md",
			Reference:      "9e49be5e",
			Repo:           "/Users/devuser/GolandProjects/claude-configs",
			Scorer:         []string{"make", "score"},
			Skill:          "fimplement",
			SkillMd:        "evals/fimplement/2026-09-26-a36dd5/SKILL.md",
			SkillVersion:   "1.4.0",
			TimeoutS:       7200,
		},
		_shouldPass: true,

		runDir: currentRun,
	})

	// retired-smoke-flag
	smokeFlagRun := t.TempDir()
	writeTestFile(t, filepath.Join(smokeFlagRun, manifestFile), runManifestSmokeFlagExample)
	tests = append(tests, &testCase{
		_id: "retired-smoke-flag",
		_expected: &RunManifest{
			Date:  "2026-09-26",
			Skill: "fimplement",
		},
		_shouldPass: true,

		runDir: smokeFlagRun,
	})

	// edited-run-json-refused
	editedRun := t.TempDir()
	require.NoError(t, writeManifest(editedRun, &RunManifest{Date: "2026-09-26", Skill: "fimplement"}))
	writeTestFile(t, filepath.Join(editedRun, manifestFile), runManifestSmokeFlagExample)
	tests = append(tests, &testCase{
		_id:         "edited-run-json-refused",
		_expected:   nil,
		_shouldPass: false,

		runDir: editedRun,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			manifest, err := loadManifest(test.runDir)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, manifest)
		})
	}
}

func TestLoadManifest_recordIdentity(t *testing.T) {
	runDir := t.TempDir()
	writeTestFile(t, filepath.Join(runDir, manifestFile), `{"layer": "smoke", "mode": "interactive", "skill": "heartbeat-demo", "skillVersion": "2.1.2"}`)
	expected := &RunManifest{
		Layer:        LayerSmoke,
		Mode:         ModeInteractive,
		Skill:        "heartbeat-demo",
		SkillVersion: "2.1.2",
	}

	manifest, err := loadManifest(runDir)
	require.NoError(t, err)
	assert.Equal(t, expected, manifest)
}

func TestRunManifest_armOverlay(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		arm      *ManifestArm
		manifest *RunManifest
	}

	tests := make([]*testCase, 0)

	// arm-override-wins
	tests = append(tests, &testCase{
		_id:       "arm-override-wins",
		_expected: "/runs/x/overlay-variant.patch",

		arm:      &ManifestArm{Label: "A", OverlayPatch: "/runs/x/overlay-variant.patch"},
		manifest: &RunManifest{OverlayPatch: "/runs/x/overlay.patch"},
	})

	// no-override-run-overlay
	tests = append(tests, &testCase{
		_id:       "no-override-run-overlay",
		_expected: "/runs/x/overlay.patch",

		arm:      &ManifestArm{Label: "A", OverlayPatch: ""},
		manifest: &RunManifest{OverlayPatch: "/runs/x/overlay.patch"},
	})

	// neither-empty
	tests = append(tests, &testCase{
		_id:       "neither-empty",
		_expected: "",

		arm:      &ManifestArm{Label: "A", OverlayPatch: ""},
		manifest: &RunManifest{OverlayPatch: ""},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.manifest.armOverlay(test.arm))
		})
	}
}

func TestRunManifest_isCompared(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		manifest *RunManifest
	}

	tests := make([]*testCase, 0)

	// one-key-three-replicas
	tests = append(tests, &testCase{
		_id:       "one-key-three-replicas",
		_expected: false,

		manifest: &RunManifest{Arms: []ManifestArm{{Key: "opus-high", Label: "A"}, {Key: "opus-high", Label: "B"}, {Key: "opus-high", Label: "C"}}},
	})

	// two-keys
	tests = append(tests, &testCase{
		_id:       "two-keys",
		_expected: true,

		manifest: &RunManifest{Arms: []ManifestArm{{Key: "opus-high", Label: "A"}, {Key: "sonnet-high", Label: "B"}}},
	})

	// no-arms
	tests = append(tests, &testCase{
		_id:       "no-arms",
		_expected: false,

		manifest: &RunManifest{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.manifest.isCompared())
		})
	}
}

func TestRunManifest_requiredServers(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		arm      *ManifestArm
		manifest *RunManifest
	}

	tests := make([]*testCase, 0)

	// none-disabled
	noneDisabled := &RunManifest{Arms: []ManifestArm{{Label: "A"}, {Label: "B"}}}
	tests = append(tests, &testCase{
		_id:       "none-disabled",
		_expected: []string{},

		arm:      &noneDisabled.Arms[0],
		manifest: noneDisabled,
	})

	// other-disables-goland
	otherDisables := &RunManifest{Arms: []ManifestArm{{Label: "A"}, {DisabledServers: []string{"goland"}, Label: "B"}}}
	tests = append(tests, &testCase{
		_id:       "other-disables-goland",
		_expected: []string{"goland"},

		arm:      &otherDisables.Arms[0],
		manifest: otherDisables,
	})

	// self-disables-goland
	selfDisables := &RunManifest{Arms: []ManifestArm{{Label: "A"}, {DisabledServers: []string{"goland"}, Label: "B"}}}
	tests = append(tests, &testCase{
		_id:       "self-disables-goland",
		_expected: []string{},

		arm:      &selfDisables.Arms[1],
		manifest: selfDisables,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.manifest.requiredServers(test.arm))
		})
	}
}

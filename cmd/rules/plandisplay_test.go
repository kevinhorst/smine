package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/acdsl"
)

const (
	displayPlan       = "# Plan\n\nroute: `change`\n\n## TLDR\n- x\n"
	displayRules      = `{"checked":["go test ./..."],"rules":[{"needed_for":"A","rule":"Bash(lsof *)","target":"lsof -i :6101"}]}`
	displayRulesTable = "**Plan rules** — from permissions.json, shown in this display copy only\n\n| Rule | Command | Needed for |\n|---|---|---|\n| `Bash(lsof *)` | `lsof -i :6101` | A |\n\n- checked: `go test ./...`\n"
)

// provideDisplayPlan writes a plan file under a fresh repo's wip plan dir
// inside its feature folder and, unless rules is empty, the plan dir's rules
// file; it returns the plan file.
func provideDisplayPlan(t *testing.T, content string, name string, rules string) string {
	t.Helper()
	repo, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	plan := filepath.Join(repo, "plans", "2026-10-02-x-f1a2b3c", "2026-10-02-x-wip-abc1234")
	require.NoError(t, os.MkdirAll(filepath.Join(plan, "design"), 0o755))

	planFile := filepath.Join(plan, "design", name)
	require.NoError(t, os.WriteFile(planFile, []byte(content), 0o644))
	if rules != "" {
		require.NoError(t, os.WriteFile(filepath.Join(plan, "permissions.json"), []byte(rules), 0o644))
	}
	return planFile
}

// readDisplay is the display file's content, "" when it does not exist.
func readDisplay(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(raw)
}

func TestRunPlanDisplay(t *testing.T) {
	type testCase struct {
		_id              string
		_expected        int
		_expectedContent string

		args    []string
		display string
	}

	home, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	t.Setenv("HOME", home)
	plansDir := filepath.Join(home, ".claude", "plans")
	require.NoError(t, os.MkdirAll(plansDir, 0o755))
	tests := make([]*testCase, 0)

	// plan-with-rules
	withRules := provideDisplayPlan(t, displayPlan, "x.md", displayRules)
	tests = append(tests, &testCase{
		_id:              "plan-with-rules",
		_expected:        exitClean,
		_expectedContent: "# Plan\n\nroute: `change`\n\n" + displayRulesTable + "\n## TLDR\n- x\n",

		args:    []string{withRules, filepath.Join(plansDir, "plan-with-rules.md")},
		display: filepath.Join(plansDir, "plan-with-rules.md"),
	})

	// plan-without-rules-file
	withoutRules := provideDisplayPlan(t, displayPlan, "x.md", "")
	tests = append(tests, &testCase{
		_id:              "plan-without-rules-file",
		_expected:        exitClean,
		_expectedContent: displayPlan,

		args:    []string{withoutRules, filepath.Join(plansDir, "plan-without-rules-file.md")},
		display: filepath.Join(plansDir, "plan-without-rules-file.md"),
	})

	// empty-rule-set
	emptyRules := provideDisplayPlan(t, displayPlan, "x.md", `{"checked":[],"rules":[]}`)
	tests = append(tests, &testCase{
		_id:              "empty-rule-set",
		_expected:        exitClean,
		_expectedContent: "# Plan\n\nroute: `change`\n\n**Plan rules** — from permissions.json, shown in this display copy only\n\n- no rule needed\n\n- checked: \n\n## TLDR\n- x\n",

		args:    []string{emptyRules, filepath.Join(plansDir, "empty-rule-set.md")},
		display: filepath.Join(plansDir, "empty-rule-set.md"),
	})

	// projection-block-stripped
	projected := "<!-- " + acdsl.ProjectionMarker + " 1 rule(s) govern this file — working-copy view, stripped before commit -->\n<!-- - [X-001] y -->\n\n" + displayPlan
	withBlock := provideDisplayPlan(t, projected, "x.md", "")
	tests = append(tests, &testCase{
		_id:              "projection-block-stripped",
		_expected:        exitClean,
		_expectedContent: displayPlan,

		args:    []string{withBlock, filepath.Join(plansDir, "projection-block-stripped.md")},
		display: filepath.Join(plansDir, "projection-block-stripped.md"),
	})

	// ui-embed-rewritten
	withEmbed := provideDisplayPlan(t, "# Plan\n\n## TLDR\n![nav](ui/nav.png)\n", "x.md", "")
	tests = append(tests, &testCase{
		_id:              "ui-embed-rewritten",
		_expected:        exitClean,
		_expectedContent: "# Plan\n\n## TLDR\n![nav](" + filepath.Join(filepath.Dir(withEmbed), "ui", "nav.png") + ")\n",

		args:    []string{withEmbed, filepath.Join(plansDir, "ui-embed-rewritten.md")},
		display: filepath.Join(plansDir, "ui-embed-rewritten.md"),
	})

	// existing-display-overwritten
	overwritten := provideDisplayPlan(t, displayPlan, "x.md", "")
	require.NoError(t, os.WriteFile(filepath.Join(plansDir, "existing-display-overwritten.md"), []byte("old draft\n"), 0o644))
	tests = append(tests, &testCase{
		_id:              "existing-display-overwritten",
		_expected:        exitClean,
		_expectedContent: displayPlan,

		args:    []string{overwritten, filepath.Join(plansDir, "existing-display-overwritten.md")},
		display: filepath.Join(plansDir, "existing-display-overwritten.md"),
	})

	// plan-without-section-heading
	noHeading := provideDisplayPlan(t, "# Plan\n\nbody\n", "x.md", displayRules)
	tests = append(tests, &testCase{
		_id:              "plan-without-section-heading",
		_expected:        exitClean,
		_expectedContent: "# Plan\n\nbody\n\n" + displayRulesTable,

		args:    []string{noHeading, filepath.Join(plansDir, "plan-without-section-heading.md")},
		display: filepath.Join(plansDir, "plan-without-section-heading.md"),
	})

	// display-outside-plans-folder
	outside := filepath.Join(t.TempDir(), "x.md")
	tests = append(tests, &testCase{
		_id:              "display-outside-plans-folder",
		_expected:        exitError,
		_expectedContent: "",

		args:    []string{withRules, outside},
		display: outside,
	})

	// display-in-subfolder
	require.NoError(t, os.MkdirAll(filepath.Join(plansDir, "sub"), 0o755))
	tests = append(tests, &testCase{
		_id:              "display-in-subfolder",
		_expected:        exitError,
		_expectedContent: "",

		args:    []string{withRules, filepath.Join(plansDir, "sub", "x.md")},
		display: filepath.Join(plansDir, "sub", "x.md"),
	})

	// display-not-markdown
	tests = append(tests, &testCase{
		_id:              "display-not-markdown",
		_expected:        exitError,
		_expectedContent: "",

		args:    []string{withRules, filepath.Join(plansDir, "x.txt")},
		display: filepath.Join(plansDir, "x.txt"),
	})

	// display-is-symlink
	linkTarget := filepath.Join(t.TempDir(), "target.md")
	require.NoError(t, os.WriteFile(linkTarget, []byte("target\n"), 0o644))
	require.NoError(t, os.Symlink(linkTarget, filepath.Join(plansDir, "link.md")))
	tests = append(tests, &testCase{
		_id:              "display-is-symlink",
		_expected:        exitError,
		_expectedContent: "target\n",

		args:    []string{withRules, filepath.Join(plansDir, "link.md")},
		display: filepath.Join(plansDir, "link.md"),
	})

	// display-through-symlinked-parent
	linkedPlansDir := filepath.Join(t.TempDir(), "plans-link")
	require.NoError(t, os.Symlink(plansDir, linkedPlansDir))
	tests = append(tests, &testCase{
		_id:              "display-through-symlinked-parent",
		_expected:        exitClean,
		_expectedContent: displayPlan,

		args:    []string{withoutRules, filepath.Join(linkedPlansDir, "display-through-symlinked-parent.md")},
		display: filepath.Join(plansDir, "display-through-symlinked-parent.md"),
	})

	// source-not-a-plan-file
	notPlan := provideDisplayPlan(t, displayPlan, "notes.md", "")
	tests = append(tests, &testCase{
		_id:              "source-not-a-plan-file",
		_expected:        exitError,
		_expectedContent: "",

		args:    []string{notPlan, filepath.Join(plansDir, "source-not-a-plan-file.md")},
		display: filepath.Join(plansDir, "source-not-a-plan-file.md"),
	})

	// source-at-old-depth
	oldDepthDesign := filepath.Join(t.TempDir(), "plans", "2026-10-02-x-wip-abc1234", "design")
	require.NoError(t, os.MkdirAll(oldDepthDesign, 0o755))
	oldDepth := filepath.Join(oldDepthDesign, "x.md")
	require.NoError(t, os.WriteFile(oldDepth, []byte(displayPlan), 0o644))
	tests = append(tests, &testCase{
		_id:              "source-at-old-depth",
		_expected:        exitError,
		_expectedContent: "",

		args:    []string{oldDepth, filepath.Join(plansDir, "source-at-old-depth.md")},
		display: filepath.Join(plansDir, "source-at-old-depth.md"),
	})

	// source-missing
	tests = append(tests, &testCase{
		_id:              "source-missing",
		_expected:        exitError,
		_expectedContent: "",

		args:    []string{filepath.Join(filepath.Dir(withRules), "missing.md"), filepath.Join(plansDir, "source-missing.md")},
		display: filepath.Join(plansDir, "source-missing.md"),
	})

	// malformed-rules-file
	malformed := provideDisplayPlan(t, displayPlan, "x.md", `{"rules":[{"rule":"Bash(x *)","typo":1}]}`)
	tests = append(tests, &testCase{
		_id:              "malformed-rules-file",
		_expected:        exitError,
		_expectedContent: "",

		args:    []string{malformed, filepath.Join(plansDir, "malformed-rules-file.md")},
		display: filepath.Join(plansDir, "malformed-rules-file.md"),
	})

	// wrong-argument-count
	tests = append(tests, &testCase{
		_id:              "wrong-argument-count",
		_expected:        exitError,
		_expectedContent: "",

		args:    []string{withRules},
		display: filepath.Join(plansDir, "wrong-argument-count.md"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, runPlanDisplay(test.args))
			assert.Equal(t, test._expectedContent, readDisplay(test.display))
		})
	}
}

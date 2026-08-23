package acdsl

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// grepScript flags any listed file containing the marker VIOLATION — a
// deterministic stand-in verifier for fixture-verdict tests.
func grepScript(t *testing.T) string {
	t.Helper()
	return writeScript(t, `#!/bin/sh
found=0
while read -r f; do
  if grep -q VIOLATION "$f"; then
    echo "$f:1: seeded violation"
    found=1
  fi
done < "$1"
[ "$found" -eq 1 ] && exit 1
exit 0
`)
}

func fixtureTree(t *testing.T, ruleId string, passContent, failContent string) string {
	t.Helper()
	root := t.TempDir()
	for set, content := range map[string]string{"pass": passContent, "fail": failContent} {
		dir := filepath.Join(root, FixturesDir, ruleId, set)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		if content != "" {
			require.NoError(t, os.WriteFile(filepath.Join(dir, "example.txt"), []byte(content), 0o644))
		}
	}
	return root
}

func fixtureSetup(t *testing.T) ([]Rule, map[string]RegistryEntry) {
	t.Helper()
	rules := []Rule{{Id: "R-001", Verifier: "fake", Anchor: `\.go$`, Why: "w"}}
	registry := map[string]RegistryEntry{"fake": {Argv: []string{grepScript(t)}, TimeoutSec: 10}}
	return rules, registry
}

func TestRunFixturesGreen(t *testing.T) {
	rules, registry := fixtureSetup(t)
	root := fixtureTree(t, "R-001", "clean content", "has VIOLATION here")
	report, err := RunFixtures(context.Background(), root, rules, registry)
	require.NoError(t, err)
	assert.Empty(t, report.Failures)
	assert.Equal(t, 1, report.Checked)
	assert.Equal(t, 2, report.Files)
	assert.Greater(t, report.Elapsed, time.Duration(0))
}

func TestRunFixturesPassFixtureFlagged(t *testing.T) {
	rules, registry := fixtureSetup(t)
	root := fixtureTree(t, "R-001", "oops VIOLATION", "has VIOLATION here")
	report, err := RunFixtures(context.Background(), root, rules, registry)
	require.NoError(t, err)
	require.Len(t, report.Failures, 1)
	assert.Contains(t, report.Failures[0], "R-001: pass fixture flagged")
	assert.Equal(t, 1, report.Checked)
}

func TestRunFixturesNeedsMissingSkipsRule(t *testing.T) {
	// A rule whose needs= artifact is absent (private artifacts on a public
	// tree) skips its fixtures — the verifier would read the missing file,
	// so verdicts would be meaningless there.
	rules, registry := fixtureSetup(t)
	rules[0].Needs = []string{"context/context.json"}
	root := fixtureTree(t, "R-001", "clean", "also clean")
	report, err := RunFixtures(context.Background(), root, rules, registry)
	require.NoError(t, err)
	assert.Empty(t, report.Failures)
	assert.Equal(t, 0, report.Checked)
	assert.Equal(t, 1, report.Skipped)
	assert.Contains(t, report.Meta(), "1 skipped (needs)")
}

func TestRunFixturesNeedsPresentRuns(t *testing.T) {
	rules, registry := fixtureSetup(t)
	rules[0].Needs = []string{"context/context.json"}
	root := fixtureTree(t, "R-001", "clean content", "has VIOLATION here")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "context"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "context", "context.json"), []byte("{}"), 0o644))
	report, err := RunFixtures(context.Background(), root, rules, registry)
	require.NoError(t, err)
	assert.Empty(t, report.Failures)
	assert.Equal(t, 1, report.Checked)
}

func TestRunFixturesFailFixtureNotFlagged(t *testing.T) {
	rules, registry := fixtureSetup(t)
	root := fixtureTree(t, "R-001", "clean", "also clean")
	report, err := RunFixtures(context.Background(), root, rules, registry)
	require.NoError(t, err)
	require.Len(t, report.Failures, 1)
	assert.Contains(t, report.Failures[0], "R-001: fail fixture not flagged")
}

func TestRunFixturesEmptySetIsFailure(t *testing.T) {
	rules, registry := fixtureSetup(t)
	root := fixtureTree(t, "R-001", "", "has VIOLATION here")
	report, err := RunFixtures(context.Background(), root, rules, registry)
	require.NoError(t, err)
	require.Len(t, report.Failures, 1)
	assert.Contains(t, report.Failures[0], "fixtures/pass is empty")
}

func TestRunFixturesNoDirSkipped(t *testing.T) {
	rules, registry := fixtureSetup(t)
	report, err := RunFixtures(context.Background(), t.TempDir(), rules, registry)
	require.NoError(t, err)
	assert.Empty(t, report.Failures)
	assert.Equal(t, 0, report.Checked)
}

func TestRunFixturesUnregisteredVerifierIsError(t *testing.T) {
	rules := []Rule{{Id: "R-001", Verifier: "absent", Anchor: `\.go$`, Why: "w"}}
	root := fixtureTree(t, "R-001", "a", "b")
	_, err := RunFixtures(context.Background(), root, rules, map[string]RegistryEntry{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `verifier "absent" not registered`)
}

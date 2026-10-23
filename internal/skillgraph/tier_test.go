package skillgraph

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// savedTierFixtureDir saves the fixture with the given tier under
// <tmp>/root/group/demo and returns the skill dir.
func savedTierFixtureDir(t *testing.T, tier string) string {
	t.Helper()
	skill := provideCompleteSkill()
	skill.Tier = tier
	dir := filepath.Join(t.TempDir(), "root", "group", "demo")
	require.NoError(t, Save(dir, skill))
	return dir
}

func TestLoad_Tier(t *testing.T) {
	type testCase struct {
		_id           string
		_expectedTier string
		_shouldPass   bool

		dir string
	}

	tests := make([]*testCase, 0)

	// pass-strict
	dir := savedTierFixtureDir(t, TierStrict)
	tests = append(tests, &testCase{_id: "pass-strict", _expectedTier: TierStrict, _shouldPass: true, dir: dir})

	// pass-absent
	dir = savedTierFixtureDir(t, "")
	tests = append(tests, &testCase{_id: "pass-absent", _expectedTier: "", _shouldPass: true, dir: dir})

	// fail-unknown-tier
	dir = savedTierFixtureDir(t, TierStrict)
	modelPath := ModelPath(dir)
	data, err := os.ReadFile(modelPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(modelPath, []byte(replaceOnce(t, string(data), `tier: "strict"`, `tier: "model"`)), 0o600))
	tests = append(tests, &testCase{_id: "fail-unknown-tier", _expectedTier: "", _shouldPass: false, dir: dir})

	// pass-strict-resaved
	dir = savedTierFixtureDir(t, TierStrict)
	loaded, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, Save(dir, loaded))
	tests = append(tests, &testCase{_id: "pass-strict-resaved", _expectedTier: TierStrict, _shouldPass: true, dir: dir})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			skill, err := Load(test.dir)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedTier, tierOf(skill))
		})
	}
}

// tierOf returns a loaded skill's tier, "" for a nil skill.
func tierOf(skill *Skill) string {
	if skill == nil {
		return ""
	}
	return skill.Tier
}

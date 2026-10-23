package skillgraph

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// savedRuntimeFixtureDir saves the fixture with the given placement under <tmp>/root/group/demo and returns the skill dir.
func savedRuntimeFixtureDir(t *testing.T, docker string, delegation string, permissions []string) string {
	t.Helper()
	skill := provideCompleteSkill()
	skill.Docker = docker
	skill.EnableDelegation = delegation
	skill.EnforcePermissions = permissions
	dir := filepath.Join(t.TempDir(), "root", "group", "demo")
	require.NoError(t, Save(dir, skill))
	return dir
}

func TestLoad_RuntimeFields(t *testing.T) {
	type testCase struct {
		_id                 string
		_expectedDelegation string
		_expectedDocker     string
		_expectedModes      []string
		_shouldPass         bool

		dir string
	}

	tests := make([]*testCase, 0)

	// pass-all-set
	dir := savedRuntimeFixtureDir(t, DockerAlways, DelegationNever, []string{PermissionAcceptEdits, PermissionPlanMode})
	tests = append(tests, &testCase{_id: "pass-all-set", _expectedDelegation: DelegationNever, _expectedDocker: DockerAlways, _expectedModes: []string{PermissionAcceptEdits, PermissionPlanMode}, _shouldPass: true, dir: dir})

	// pass-absent
	dir = savedRuntimeFixtureDir(t, "", "", nil)
	tests = append(tests, &testCase{_id: "pass-absent", _expectedDelegation: "", _expectedDocker: "", _expectedModes: nil, _shouldPass: true, dir: dir})

	// fail-unknown-docker
	dir = savedRuntimeFixtureDir(t, DockerAlways, "", nil)
	modelPath := ModelPath(dir)
	data, err := os.ReadFile(modelPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(modelPath, []byte(replaceOnce(t, string(data), `docker: "always"`, `docker: "sometimes"`)), 0o600))
	tests = append(tests, &testCase{_id: "fail-unknown-docker", _expectedDelegation: "", _expectedDocker: "", _expectedModes: nil, _shouldPass: false, dir: dir})

	// fail-unknown-mode
	dir = savedRuntimeFixtureDir(t, "", "", []string{PermissionPlanMode})
	modelPath = ModelPath(dir)
	data, err = os.ReadFile(modelPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(modelPath, []byte(replaceOnce(t, string(data), `"planMode"`, `"plan"`)), 0o600))
	tests = append(tests, &testCase{_id: "fail-unknown-mode", _expectedDelegation: "", _expectedDocker: "", _expectedModes: nil, _shouldPass: false, dir: dir})

	// pass-resaved
	dir = savedRuntimeFixtureDir(t, DockerNever, DelegationAlways, nil)
	loaded, err := Load(dir)
	require.NoError(t, err)
	require.NoError(t, Save(dir, loaded))
	tests = append(tests, &testCase{_id: "pass-resaved", _expectedDelegation: DelegationAlways, _expectedDocker: DockerNever, _expectedModes: nil, _shouldPass: true, dir: dir})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			skill, err := Load(test.dir)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedDocker, runtimeOf(skill).Docker)
			assert.Equal(t, test._expectedDelegation, runtimeOf(skill).EnableDelegation)
			assert.Equal(t, test._expectedModes, runtimeOf(skill).EnforcePermissions)
		})
	}
}

// runtimeOf returns a loaded skill, or an empty one for nil.
func runtimeOf(skill *Skill) *Skill {
	if skill == nil {
		return &Skill{}
	}
	return skill
}

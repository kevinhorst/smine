package install

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
)

// artifactComponentOf is the catalog's installer for id.
func artifactComponentOf(t *testing.T, id components.ID) *artifactComponent {
	t.Helper()
	for _, component := range installers(FullRecord(goosDarwin)) {
		if component.ID() == id {
			artifact, ok := component.(*artifactComponent)
			require.True(t, ok, "%s is an artifact component", id)
			return artifact
		}
	}
	require.Failf(t, "no installer", "%s", id)
	return nil
}

func TestApply_UserFileNeverOverwritten(t *testing.T) {
	ctx := context.Background()
	env := newTestEnv(t)
	provideApplyRepo(t, env)
	instructions := filepath.Join(env.ClaudeHome, "AGENTS.md")
	selected := customRecord(components.ConfigServer, components.GlobalInstructions)

	// absent-installed-and-recorded
	_, err := Apply(ctx, env, selected)
	require.NoError(t, err)
	assert.Equal(t, "# Global\n", string(readTestFile(t, instructions)))
	isRecorded, err := env.Trash.IsUnchanged(instructions)
	require.NoError(t, err)
	assert.True(t, isRecorded)

	// unchanged-since-record-updated-with-stash
	writeTestFile(t, env.RepoDir, "settings/claude_code/AGENTS.md", "# Global v2\n")
	_, err = Apply(ctx, env, withStoredBase(t, env, selected))
	require.NoError(t, err)
	assert.Equal(t, "# Global v2\n", string(readTestFile(t, instructions)))
	assert.Equal(t, "# Global\n", stashedBytes(t, env, instructions))

	// edited-after-record-kept
	writeTestFile(t, env.ClaudeHome, "AGENTS.md", "my own instructions\n")
	writeTestFile(t, env.RepoDir, "settings/claude_code/AGENTS.md", "# Global v3\n")
	steps, err := Plan(ctx, env, selected)
	require.NoError(t, err)
	assert.Contains(t, steps, Step{Action: ActionKept, Component: components.GlobalInstructions, Detail: instructions})
	_, err = Apply(ctx, env, withStoredBase(t, env, selected))
	require.NoError(t, err)
	assert.Equal(t, "my own instructions\n", string(readTestFile(t, instructions)))
}

func TestApply_EqualUnrecordedCopyRecorded(t *testing.T) {
	ctx := context.Background()
	env := newTestEnv(t)
	provideApplyRepo(t, env)
	instructions := filepath.Join(env.ClaudeHome, "AGENTS.md")
	writeTestFile(t, env.ClaudeHome, "AGENTS.md", "# Global\n")
	selected := customRecord(components.ConfigServer, components.GlobalInstructions)

	steps, err := Plan(ctx, env, selected)
	require.NoError(t, err)
	assert.Contains(t, steps, Step{Action: ActionRecord, Component: components.GlobalInstructions, Detail: instructions})

	_, err = Apply(ctx, env, selected)
	require.NoError(t, err)
	isRecorded, err := env.Trash.IsUnchanged(instructions)
	require.NoError(t, err)
	assert.True(t, isRecorded)
	assert.Nil(t, journalLines(t, env.Trash), "a record step stashes nothing")
}

func TestRetire_MovesOwnedTargets(t *testing.T) {
	ctx := context.Background()
	component := artifactComponentOf(t, components.GlobalInstructions)

	// recorded-target-moved-and-forgotten
	env := newTestEnv(t)
	provideApplyRepo(t, env)
	instructions := filepath.Join(env.ClaudeHome, "AGENTS.md")
	_, err := component.converge(ctx, env, true)
	require.NoError(t, err)
	steps, err := component.retire(ctx, env, true)
	require.NoError(t, err)
	assert.Contains(t, steps, Step{Action: ActionRemove, Component: components.GlobalInstructions, Detail: instructions})
	assert.NoFileExists(t, instructions)
	assert.Equal(t, "# Global\n", stashedBytes(t, env, instructions))
	writeTestFile(t, env.ClaudeHome, "AGENTS.md", "# Global\n")
	isRecorded, err := env.Trash.IsUnchanged(instructions)
	require.NoError(t, err)
	assert.False(t, isRecorded, "the digest is forgotten with the move")

	// edited-target-kept-with-its-digest
	env = newTestEnv(t)
	provideApplyRepo(t, env)
	instructions = filepath.Join(env.ClaudeHome, "AGENTS.md")
	_, err = component.converge(ctx, env, true)
	require.NoError(t, err)
	writeTestFile(t, env.ClaudeHome, "AGENTS.md", "my own instructions\n")
	steps, err = component.retire(ctx, env, true)
	require.NoError(t, err)
	assert.Contains(t, steps, Step{Action: ActionKept, Component: components.GlobalInstructions, Detail: instructions})
	assert.Equal(t, "my own instructions\n", string(readTestFile(t, instructions)))
	assert.Nil(t, journalLines(t, env.Trash))
	writeTestFile(t, env.ClaudeHome, "AGENTS.md", "# Global\n")
	isRecorded, err = env.Trash.IsUnchanged(instructions)
	require.NoError(t, err)
	assert.True(t, isRecorded, "the digest stays for a kept target")
}

func TestBinaryTarget_Windows(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	env := newTestEnv(t)
	env.GOOS = goosWindows

	target := binary{kind: binaryPath, name: "rules"}.target(env)

	assert.Equal(t, filepath.Join(os.Getenv("LOCALAPPDATA"), "smine", "bin", "smine-rules.exe"), target)
	assert.Equal(t, filepath.Join(env.ClaudeHome, "hooks", "bin", "toolpolicy"), binary{kind: binaryHook, name: "toolpolicy"}.target(env))
}

func TestFileWrite_Apply(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedAction  string
		_expectedContent string
		_expectedJournal []string
		_expectedOwned   bool

		env   Env
		write fileWrite
	}

	tests := make([]*testCase, 0)

	// absent-install
	env := newTestEnv(t)
	target := filepath.Join(env.ClaudeHome, "AGENTS.md")
	tests = append(tests, &testCase{
		_id:              "absent-install",
		_expectedAction:  ActionInstall,
		_expectedContent: "new",
		_expectedJournal: nil,
		_expectedOwned:   true,

		env:   env,
		write: fileWrite{data: []byte("new"), dirPerm: 0o755, isGuarded: true, perm: 0o644, target: target},
	})

	// equal-nothing
	env = newTestEnv(t)
	target = filepath.Join(env.ClaudeHome, "settings.rendered.json")
	writeTestFile(t, env.ClaudeHome, "settings.rendered.json", "same")
	tests = append(tests, &testCase{
		_id:              "equal-nothing",
		_expectedAction:  "",
		_expectedContent: "same",
		_expectedJournal: nil,
		_expectedOwned:   false,

		env:   env,
		write: fileWrite{data: []byte("same"), dirPerm: 0o755, perm: 0o644, target: target},
	})

	// guarded-recorded-update
	env = newTestEnv(t)
	target = filepath.Join(env.ClaudeHome, "AGENTS.md")
	writeTestFile(t, env.ClaudeHome, "AGENTS.md", "old")
	require.NoError(t, env.Trash.RecordOwned(target, []byte("old")))
	tests = append(tests, &testCase{
		_id:              "guarded-recorded-update",
		_expectedAction:  ActionUpdate,
		_expectedContent: "new",
		_expectedJournal: []string{"replaced " + target},
		_expectedOwned:   true,

		env:   env,
		write: fileWrite{data: []byte("new"), dirPerm: 0o755, isGuarded: true, perm: 0o644, target: target},
	})

	// guarded-foreign-kept
	env = newTestEnv(t)
	target = filepath.Join(env.ClaudeHome, "AGENTS.md")
	writeTestFile(t, env.ClaudeHome, "AGENTS.md", "mine")
	tests = append(tests, &testCase{
		_id:              "guarded-foreign-kept",
		_expectedAction:  ActionKept,
		_expectedContent: "mine",
		_expectedJournal: nil,
		_expectedOwned:   false,

		env:   env,
		write: fileWrite{data: []byte("new"), dirPerm: 0o755, isGuarded: true, perm: 0o644, target: target},
	})

	// guarded-equal-recorded
	env = newTestEnv(t)
	target = filepath.Join(env.ClaudeHome, "AGENTS.md")
	writeTestFile(t, env.ClaudeHome, "AGENTS.md", "same")
	tests = append(tests, &testCase{
		_id:              "guarded-equal-recorded",
		_expectedAction:  ActionRecord,
		_expectedContent: "same",
		_expectedJournal: nil,
		_expectedOwned:   true,

		env:   env,
		write: fileWrite{data: []byte("same"), dirPerm: 0o755, isGuarded: true, perm: 0o644, target: target},
	})

	// binary-update-without-stash
	env = newTestEnv(t)
	target = filepath.Join(env.ClaudeHome, "hooks", "bin", "toolpolicy")
	writeTestFile(t, env.ClaudeHome, "hooks/bin/toolpolicy", "old binary")
	tests = append(tests, &testCase{
		_id:              "binary-update-without-stash",
		_expectedAction:  ActionUpdate,
		_expectedContent: "new binary",
		_expectedJournal: nil,
		_expectedOwned:   false,

		env:   env,
		write: fileWrite{data: []byte("new binary"), dirPerm: 0o755, isBinary: true, perm: 0o755, target: target},
	})

	// rendered-update-with-stash
	env = newTestEnv(t)
	target = filepath.Join(env.ClaudeHome, "settings.rendered.json")
	writeTestFile(t, env.ClaudeHome, "settings.rendered.json", "{}")
	tests = append(tests, &testCase{
		_id:              "rendered-update-with-stash",
		_expectedAction:  ActionUpdate,
		_expectedContent: `{"a": 1}`,
		_expectedJournal: []string{"replaced " + target},
		_expectedOwned:   false,

		env:   env,
		write: fileWrite{data: []byte(`{"a": 1}`), dirPerm: 0o755, perm: 0o644, target: target},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			action, err := test.write.apply(test.env, true)
			assert.NoError(t, err)
			assert.Equal(t, test._expectedAction, action)
			assert.Equal(t, test._expectedContent, string(readTestFile(t, test.write.target)))
			assert.Equal(t, test._expectedJournal, journalLines(t, test.env.Trash))
			isOwned, err := test.env.Trash.IsUnchanged(test.write.target)
			assert.NoError(t, err)
			assert.Equal(t, test._expectedOwned, isOwned)
		})
	}
}

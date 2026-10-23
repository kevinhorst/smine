package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/skillgraph"
)

// writeChangelogSkill writes a modeled demo skill whose changelog holds one row per version, newest first, and the frontmatter version of the top row.
func writeChangelogSkill(t *testing.T, versions ...string) string {
	t.Helper()
	dir := writeModeledSkill(t, t.TempDir(), "demo")
	path := skillgraph.ModelPath(dir)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	skill, err := skillgraph.DecodeSkill(data, path)
	require.NoError(t, err)

	rows := make([]skillgraph.ChangelogEntry, 0, len(versions))
	for _, version := range versions {
		rows = append(rows, skillgraph.ChangelogEntry{Date: "2000-01-01", Text: "fixture " + version, Version: version})
	}
	skill.Changelog = rows
	for index, field := range skill.Frontmatter {
		if field.Key == "version" {
			skill.Frontmatter[index].Value = " " + versions[0]
		}
	}

	encoded, err := skillgraph.EncodeSkill(skill)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, encoded, 0o644))
	return dir
}

// readModel answers a skill dir's model file bytes, nil when it is absent.
func readModel(dir string) []byte {
	data, err := os.ReadFile(skillgraph.ModelPath(dir))
	if err != nil {
		return nil
	}
	return data
}

// loadedVersions answers the loaded skill's frontmatter version then each changelog row's version, nil when Load fails.
func loadedVersions(dir string) []string {
	skill, err := skillgraph.Load(dir)
	if err != nil {
		return nil
	}

	versions := []string{skill.Version()}
	for _, row := range skill.Changelog {
		versions = append(versions, row.Version)
	}
	return versions
}

// runRenumberCaptured runs the renumber command and answers its exit code and stdout.
func runRenumberCaptured(t *testing.T, args []string) (int, string) {
	t.Helper()
	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	stdout := os.Stdout
	os.Stdout = writer
	code := runRenumber(args)
	os.Stdout = stdout
	require.NoError(t, writer.Close())

	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	return code, string(output)
}

func TestRunRenumber(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedCode     int
		_expectedOutput   string
		_expectedVersions []string
		_isUnchanged      bool

		args   []string
		before []byte
		dir    string
	}

	tests := make([]*testCase, 0)

	// duplicate-top
	duplicateDir := writeChangelogSkill(t, "1.1", "1.1", "1.0")
	tests = append(tests, &testCase{
		_id:               "duplicate-top",
		_expectedCode:     exitClean,
		_expectedOutput:   "skillmodel: renumbered " + duplicateDir + ": 1.1 → 1.2\n",
		_expectedVersions: []string{"1.2", "1.2", "1.1", "1.0"},
		_isUnchanged:      false,

		args:   []string{duplicateDir},
		before: readModel(duplicateDir),
		dir:    duplicateDir,
	})

	// already-descending
	descendingDir := writeChangelogSkill(t, "1.1", "1.0")
	tests = append(tests, &testCase{
		_id:               "already-descending",
		_expectedCode:     exitClean,
		_expectedOutput:   "skillmodel: unchanged " + descendingDir + ": 1.1\n",
		_expectedVersions: []string{"1.1", "1.1", "1.0"},
		_isUnchanged:      true,

		args:   []string{descendingDir},
		before: readModel(descendingDir),
		dir:    descendingDir,
	})

	// missing-dir
	missingDir := filepath.Join(t.TempDir(), "missing")
	tests = append(tests, &testCase{
		_id:               "missing-dir",
		_expectedCode:     exitError,
		_expectedOutput:   "",
		_expectedVersions: nil,
		_isUnchanged:      true,

		args:   []string{missingDir},
		before: nil,
		dir:    missingDir,
	})

	// usage-no-args
	emptyDir := t.TempDir()
	tests = append(tests, &testCase{
		_id:               "usage-no-args",
		_expectedCode:     exitError,
		_expectedOutput:   "",
		_expectedVersions: nil,
		_isUnchanged:      true,

		args:   nil,
		before: nil,
		dir:    emptyDir,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			code, output := runRenumberCaptured(t, test.args)
			assert.Equal(t, test._expectedCode, code)
			assert.Equal(t, test._expectedOutput, output)
			assert.Equal(t, test._isUnchanged, bytes.Equal(test.before, readModel(test.dir)))
			assert.Equal(t, test._expectedVersions, loadedVersions(test.dir))
		})
	}
}

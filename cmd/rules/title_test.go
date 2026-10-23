package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureRunTitle runs title on args and returns its exit code and everything
// it printed to stdout.
func captureRunTitle(t *testing.T, args []string) (int, string) {
	t.Helper()
	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	stdout := os.Stdout
	os.Stdout = writer
	code := runTitle(args)
	os.Stdout = stdout
	require.NoError(t, writer.Close())

	printed, err := io.ReadAll(reader)
	require.NoError(t, err)
	return code, string(printed)
}

func TestRunTitle(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedExit  int
		_expectedPrint string

		args []string
	}

	skillsDir := t.TempDir()
	fdesignDir := filepath.Join(skillsDir, "fdesign")
	require.NoError(t, os.MkdirAll(fdesignDir, 0o755))
	declaration := `{"schema": 2, "skill": "fdesign", "version": "1.0.0", "states": {"DESIGN-READY": {"entry": "SKILL-FDESIGN-GATE-001", "meaning": "design gate presented"}}}`
	require.NoError(t, os.WriteFile(filepath.Join(fdesignDir, "title.json"), []byte(declaration), 0o644))

	tests := make([]*testCase, 0)

	// declared-state
	tests = append(tests, &testCase{
		_id:            "declared-state",
		_expectedExit:  exitClean,
		_expectedPrint: "title: set DESIGN-READY\n",

		args: []string{"-skills", skillsDir, "set", "DESIGN-READY"},
	})

	// wip
	tests = append(tests, &testCase{
		_id:            "wip",
		_expectedExit:  exitClean,
		_expectedPrint: "title: set WIP\n",

		args: []string{"-skills", skillsDir, "set", "WIP"},
	})

	// ready
	tests = append(tests, &testCase{
		_id:            "ready",
		_expectedExit:  exitClean,
		_expectedPrint: "title: set READY\n",

		args: []string{"-skills", skillsDir, "set", "READY"},
	})

	// undeclared-state
	tests = append(tests, &testCase{
		_id:            "undeclared-state",
		_expectedExit:  exitError,
		_expectedPrint: "",

		args: []string{"-skills", skillsDir, "set", "HUMAN-GATE"},
	})

	// terminal-builtin
	tests = append(tests, &testCase{
		_id:            "terminal-builtin",
		_expectedExit:  exitError,
		_expectedPrint: "",

		args: []string{"-skills", skillsDir, "set", "DONE"},
	})

	// missing-verb
	tests = append(tests, &testCase{
		_id:            "missing-verb",
		_expectedExit:  exitError,
		_expectedPrint: "",

		args: []string{"-skills", skillsDir, "DESIGN-READY"},
	})

	// skills-dir-without-declarations
	tests = append(tests, &testCase{
		_id:            "skills-dir-without-declarations",
		_expectedExit:  exitError,
		_expectedPrint: "",

		args: []string{"-skills", t.TempDir(), "set", "DESIGN-READY"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			code, printed := captureRunTitle(t, test.args)
			assert.Equal(t, test._expectedExit, code)
			assert.Equal(t, test._expectedPrint, printed)
		})
	}
}

package server

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGateVerdicts(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   map[string]string
		_shouldPass bool

		home    string
		targets []string
	}

	tests := make([]*testCase, 0)

	// admitted
	home := t.TempDir()
	writeGateStub(t, home, "1. make audit → admitted")
	tests = append(tests, &testCase{
		_id:         "admitted",
		_expected:   map[string]string{"make audit": "admitted"},
		_shouldPass: true,

		home:    home,
		targets: []string{"make audit"},
	})

	// denied-with-candidate
	home = t.TempDir()
	denied := "denied: no allow rule matches; candidate rule Bash(my-build *) — self-serviceable: list it in the plan dir's permissions.json with this target"
	writeGateStub(t, home, "tool-policy check:", "1. my-build → "+denied)
	tests = append(tests, &testCase{
		_id:         "denied-with-candidate",
		_expected:   map[string]string{"my-build": denied},
		_shouldPass: true,

		home:    home,
		targets: []string{"my-build"},
	})

	// not-checkable
	home = t.TempDir()
	notCheckable := "not checkable: toolpolicy commands are gate commands"
	writeGateStub(t, home, "1. toolpolicy allow x → "+notCheckable)
	tests = append(tests, &testCase{
		_id:         "not-checkable",
		_expected:   map[string]string{"toolpolicy allow x": notCheckable},
		_shouldPass: true,

		home:    home,
		targets: []string{"toolpolicy allow x"},
	})

	// exec-error
	tests = append(tests, &testCase{
		_id:         "exec-error",
		_expected:   nil,
		_shouldPass: false,

		home:    t.TempDir(),
		targets: []string{"make audit"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			verdicts, err := gateVerdicts(context.Background(), test.home, t.TempDir(), "env-skill", test.targets)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, verdicts)
		})
	}
}

func TestExitRows(t *testing.T) {
	exit := map[string]string{"0": "clean", "1": "violations", "2": "usage", "other": "error"}
	iface := &skillgraph.Interface{Id: "plan-sections", Result: skillgraph.Result{Exit: exit}}

	rows := exitRows(iface)

	expected := []exitRow{{Code: "0", Meaning: "clean"}, {Code: "1", Meaning: "violations"}, {Code: "2", Meaning: "usage"}, {Code: "other", Meaning: "error"}}
	assert.Equal(t, expected, rows)
}

// writeGateStub installs <home>/.claude/hooks/bin/toolpolicy printing lines, whatever its args.
func writeGateStub(t *testing.T, home string, lines ...string) {
	t.Helper()
	binDir := filepath.Join(home, ".claude", "hooks", "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	name, script := "toolpolicy", "#!/bin/sh\ncat <<'EOF'\n"+strings.Join(lines, "\n")+"\nEOF\n"
	if runtime.GOOS == "windows" {
		name, script = "toolpolicy.bat", "@echo "+strings.Join(lines, "\r\n@echo ")+"\r\n"
	}
	require.NoError(t, os.WriteFile(filepath.Join(binDir, name), []byte(script), 0o755))
}

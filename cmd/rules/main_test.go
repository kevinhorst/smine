package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/contextdocs"
)

func TestDispatch(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedCode  int
		_expectedOut   string
		_expectedError string

		args []string
	}

	tests := make([]*testCase, 0)

	// help-short
	tests = append(tests, &testCase{
		_id:            "help-short",
		_expectedCode:  exitClean,
		_expectedOut:   "  task-check       validate smine change task files\n",
		_expectedError: "",

		args: []string{"-h"},
	})

	// help-long
	tests = append(tests, &testCase{
		_id:            "help-long",
		_expectedCode:  exitClean,
		_expectedOut:   "usage: rules <interfaces|validate|",
		_expectedError: "",

		args: []string{"--help"},
	})

	// help-word
	tests = append(tests, &testCase{
		_id:            "help-word",
		_expectedCode:  exitClean,
		_expectedOut:   "rules <command> -h prints the command's flags.\n",
		_expectedError: "",

		args: []string{"help"},
	})

	// no-command
	tests = append(tests, &testCase{
		_id:            "no-command",
		_expectedCode:  exitError,
		_expectedOut:   "",
		_expectedError: "usage: rules <interfaces|validate|",

		args: nil,
	})

	// unknown-command
	tests = append(tests, &testCase{
		_id:            "unknown-command",
		_expectedCode:  exitError,
		_expectedOut:   "",
		_expectedError: "rules: unknown command: nope\n",

		args: []string{"nope"},
	})

	// subcommand-help-exits-clean
	tests = append(tests, &testCase{
		_id:            "subcommand-help-exits-clean",
		_expectedCode:  exitClean,
		_expectedOut:   "",
		_expectedError: "",

		args: []string{"ledger", "-h"},
	})

	// subcommand-unknown-flag-exits-usage
	tests = append(tests, &testCase{
		_id:            "subcommand-unknown-flag-exits-usage",
		_expectedCode:  exitError,
		_expectedOut:   "",
		_expectedError: "",

		args: []string{"prune", "--nope"},
	})

	// buildtest-flags-parsed
	tests = append(tests, &testCase{
		_id:            "buildtest-flags-parsed",
		_expectedCode:  exitClean,
		_expectedOut:   "",
		_expectedError: "",

		args: []string{"buildtest", "--skip", "--build", "go build ./...", "--test", "go test ./...", "--repo", ".", "--head", "HEAD", "--root", "root", "--branch", "claude-review/x", "--salvage", "salvage"},
	})

	// interfaces
	tests = append(tests, &testCase{
		_id:            "interfaces",
		_expectedCode:  exitClean,
		_expectedOut:   "",
		_expectedError: "",

		args: []string{"interfaces"},
	})

	// interfaces-extra-arg
	tests = append(tests, &testCase{
		_id:            "interfaces-extra-arg",
		_expectedCode:  exitError,
		_expectedOut:   "",
		_expectedError: "",

		args: []string{"interfaces", "x"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			code := dispatch(test.args, stdout, stderr)
			assert.Equal(t, test._expectedCode, code)
			assert.Contains(t, stdout.String(), test._expectedOut)
			assert.Contains(t, stderr.String(), test._expectedError)
		})
	}
}

func TestRunInterfaces(t *testing.T) {
	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	stdout := os.Stdout
	os.Stdout = writer
	code := runInterfaces(nil)
	os.Stdout = stdout
	require.NoError(t, writer.Close())

	printed, err := io.ReadAll(reader)
	require.NoError(t, err)

	expected := `{"interfaces":["base@1","buildtest@1","capture-ui@1","cleanup@1","demo-close@1","demo-value@1",` +
		`"entries@1","generate@1","intake@1","lanes@1","ledger@1","merge-branch@1","pairs@1","plan-claims@1","plan-display@1",` +
		`"plan-sections@1","present@1","probe-replay@1","provision@1","prune@1","reset-round@1","snapshot@1",` +
		`"store-root@1","task-check@1","title-set@1"]}` + "\n"
	assert.Equal(t, exitClean, code)
	assert.Equal(t, expected, string(printed))
	assert.IsNonDecreasing(t, claimedInterfaces)
}

func TestParseChecklist(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   map[string]string
		_shouldPass bool

		checklists map[string]string
		value      string
	}

	tests := make([]*testCase, 0)

	// pass-correctness
	tests = append(tests, &testCase{
		_id:         "pass-correctness",
		_expected:   map[string]string{"correctness": "/a/checklist.json"},
		_shouldPass: true,

		checklists: map[string]string{},
		value:      "correctness=/a/checklist.json",
	})

	// fail-missing-separator
	tests = append(tests, &testCase{
		_id:         "fail-missing-separator",
		_expected:   map[string]string{},
		_shouldPass: false,

		checklists: map[string]string{},
		value:      "correctness",
	})

	// fail-empty-path
	tests = append(tests, &testCase{
		_id:         "fail-empty-path",
		_expected:   map[string]string{},
		_shouldPass: false,

		checklists: map[string]string{},
		value:      "contracts=",
	})

	// fail-rule-direction
	tests = append(tests, &testCase{
		_id:         "fail-rule-direction",
		_expected:   map[string]string{},
		_shouldPass: false,

		checklists: map[string]string{},
		value:      "security=/a/checklist.json",
	})

	// fail-given-twice
	tests = append(tests, &testCase{
		_id:         "fail-given-twice",
		_expected:   map[string]string{"contracts": "/a/first.json"},
		_shouldPass: false,

		checklists: map[string]string{"contracts": "/a/first.json"},
		value:      "contracts=/a/second.json",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := parseChecklist(test.value, test.checklists)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, test.checklists)
		})
	}
}

func TestContextDirOf(t *testing.T) {
	type testCase struct {
		_id                 string
		_expectedDir        string
		_expectedIsDeployed bool
		_shouldPass         bool

		cwd         string
		deployedDir string
		sourceDir   string
	}

	t.Setenv(contextdocs.ContextDirDefaultKey, "")
	writeIndex := func(root string, dir string) string {
		require.NoError(t, os.MkdirAll(filepath.Join(root, dir), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, dir, contextdocs.ContextFileName), []byte("{}"), 0o644))
		return root
	}

	tests := make([]*testCase, 0)

	// deployed-wins
	tests = append(tests, &testCase{
		_id:                 "deployed-wins",
		_expectedDir:        "/deployed",
		_expectedIsDeployed: true,
		_shouldPass:         true,

		cwd:         writeIndex(t.TempDir(), contextdocs.SourceContextDir),
		deployedDir: "/deployed",
		sourceDir:   "source",
	})

	// source-passes-through
	tests = append(tests, &testCase{
		_id:                 "source-passes-through",
		_expectedDir:        "source",
		_expectedIsDeployed: false,
		_shouldPass:         true,

		cwd:         t.TempDir(),
		deployedDir: "",
		sourceDir:   "source",
	})

	// discovery-source
	tests = append(tests, &testCase{
		_id:                 "discovery-source",
		_expectedDir:        contextdocs.SourceContextDir,
		_expectedIsDeployed: false,
		_shouldPass:         true,

		cwd:         writeIndex(t.TempDir(), contextdocs.SourceContextDir),
		deployedDir: "",
		sourceDir:   "",
	})

	// discovery-deployed
	tests = append(tests, &testCase{
		_id:                 "discovery-deployed",
		_expectedDir:        "docs",
		_expectedIsDeployed: true,
		_shouldPass:         true,

		cwd:         writeIndex(t.TempDir(), "docs"),
		deployedDir: "",
		sourceDir:   "",
	})

	// no-dir
	tests = append(tests, &testCase{
		_id:                 "no-dir",
		_expectedDir:        "",
		_expectedIsDeployed: false,
		_shouldPass:         false,

		cwd:         t.TempDir(),
		deployedDir: "",
		sourceDir:   "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			t.Chdir(test.cwd)
			dir, isDeployed, err := contextDirOf(test.deployedDir, test.sourceDir)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedDir, dir)
			assert.Equal(t, test._expectedIsDeployed, isDeployed)
		})
	}
}

// TestCommands_HelpListsEveryCommand keeps the help table and the dispatch one source.
func TestCommands_HelpListsEveryCommand(t *testing.T) {
	stdout := &bytes.Buffer{}
	printHelp(stdout)

	seen := map[string]bool{}
	for _, cmd := range commands() {
		assert.False(t, seen[cmd.name], cmd.name)
		assert.NotEmpty(t, cmd.summary, cmd.name)
		assert.Contains(t, stdout.String(), "  "+cmd.name+" ")
		seen[cmd.name] = true
	}

	settingsCommands := []string{"settings", "settings-defaults"}
	for _, name := range settingsCommands {
		assert.True(t, seen[name], name)
	}
}

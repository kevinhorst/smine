package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/reviewscope"
)

const (
	ledgerPermissiveSchema = `{"type": "object"}`
	ledgerReview           = `{"probes": [], "ranked_plan": []}`
	ledgerReviewNit        = `{"probes": [], "ranked_plan": [` +
		`{"file": "a.go", "finding_id": "nit-1", "route": "auto-fix", "severity": "NIT"}]}`
	ledgerReviewReconfirmed = `{"probes": [], "ranked_plan": [` +
		`{"file": "a.go", "finding_id": "a", "known": true, "known_id": "a", "route": "auto-fix", "severity": "MINOR"}]}`
	ledgerReviewSeverities = `{"probes": [], "ranked_plan": [` +
		`{"file": "a.go", "finding_id": "major-1", "route": "auto-fix", "severity": "MAJOR"},` +
		`{"file": "a.go", "finding_id": "minor-1", "route": "auto-fix", "severity": "MINOR"}]}`
	ledgerStrictSchema = `{"type": "object", "required": ["definition_of_done"]}`
)

// isPresent reports whether path exists.
func isPresent(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ledgerArgs is a full ledger call over the round dir's artifacts dir.
func ledgerArgs(roundDir string, extra ...string) []string {
	artifactsDir := filepath.Dir(roundDir)
	args := []string{"--round-dir", roundDir, "--known", filepath.Join(artifactsDir, "known.json"), "--coverage", filepath.Join(artifactsDir, "coverage.json")}
	return append(args, extra...)
}

// provideDeployedSchema points HOME at a fresh home holding the deployed
// railroad-review consolidation schema, none when content is empty.
func provideDeployedSchema(t *testing.T, content string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if content == "" {
		return
	}

	schemaDir := filepath.Join(home, ".claude", "skills", "railroad-review", "schemas")
	require.NoError(t, os.MkdirAll(schemaDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(schemaDir, "consolidation.json"), []byte(content), 0o644))
}

// provideKnownLedger writes a fresh known.json holding finding a of round 1
// in state and returns its path.
func provideKnownLedger(t *testing.T, state reviewscope.FixState) string {
	t.Helper()
	known := []reviewscope.KnownEntry{{FindingId: "a", FixState: state, Round: 1, Route: "auto-fix", Severity: "MINOR"}}
	data, err := json.Marshal(known)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "known.json")
	require.NoError(t, os.WriteFile(path, data, 0o644))
	return path
}

// provideLedgerFile writes content to a fresh file and returns its path.
func provideLedgerFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "schema.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

// provideLedgerRound creates a fresh round-<round> dir, with a review.json
// unless review is empty, and returns it.
func provideLedgerRound(t *testing.T, round string, review string) string {
	t.Helper()
	roundDir := filepath.Join(t.TempDir(), "round-"+round)
	require.NoError(t, os.MkdirAll(roundDir, 0o755))
	if review != "" {
		require.NoError(t, os.WriteFile(filepath.Join(roundDir, "review.json"), []byte(review), 0o644))
	}
	return roundDir
}

// readLedgerResult returns the ledger output captured at stdoutPath, the
// zero result when nothing was captured.
func readLedgerResult(t *testing.T, stdoutPath string) *reviewscope.LedgerResult {
	t.Helper()
	result := &reviewscope.LedgerResult{}
	if stdoutPath == "" {
		return result
	}

	data, err := os.ReadFile(stdoutPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, result))
	return result
}

// readFixState returns the fix state of the first row of the known ledger
// at knownPath, empty when no ledger is named.
func readFixState(t *testing.T, knownPath string) reviewscope.FixState {
	t.Helper()
	if knownPath == "" {
		return ""
	}

	data, err := os.ReadFile(knownPath)
	require.NoError(t, err)

	var known []reviewscope.KnownEntry
	require.NoError(t, json.Unmarshal(data, &known))
	require.NotEmpty(t, known)
	return known[0].FixState
}

// runLedgerTo runs the ledger subcommand with its stdout captured to
// stdoutPath, uncaptured when stdoutPath is empty.
func runLedgerTo(t *testing.T, stdoutPath string, args []string) int {
	t.Helper()
	if stdoutPath == "" {
		return runLedger(args)
	}

	capture, err := os.Create(stdoutPath)
	require.NoError(t, err)
	defer capture.Close()

	stdout := os.Stdout
	os.Stdout = capture
	defer func() { os.Stdout = stdout }()
	return runLedger(args)
}

func TestRunLedger(t *testing.T) {
	type testCase struct {
		_id                  string
		_expected            int
		_expectedAutofix     []string
		_expectedFixState    reviewscope.FixState
		_expectedIgnored     []string
		_expectedOscillating []string
		_expectedWritten     bool

		args       []string
		knownPath  string
		roundDir   string
		stdoutPath string
	}

	provideDeployedSchema(t, ledgerPermissiveSchema)
	tests := make([]*testCase, 0)

	// void-round-without-review
	roundDir := provideLedgerRound(t, "1", "")
	tests = append(tests, &testCase{
		_id:              "void-round-without-review",
		_expected:        exitClean,
		_expectedWritten: false,

		args:     []string{"--round-dir", roundDir, "--void"},
		roundDir: roundDir,
	})

	// void-review-present
	roundDir = provideLedgerRound(t, "2", ledgerReview)
	tests = append(tests, &testCase{
		_id:              "void-review-present",
		_expected:        exitError,
		_expectedWritten: false,

		args:     []string{"--round-dir", roundDir, "--void"},
		roundDir: roundDir,
	})

	// void-round-dir-missing
	missingRound := filepath.Join(t.TempDir(), "round-3")
	tests = append(tests, &testCase{
		_id:              "void-round-dir-missing",
		_expected:        exitError,
		_expectedWritten: false,

		args:     []string{"--round-dir", missingRound, "--void"},
		roundDir: missingRound,
	})

	// void-without-round-dir
	roundDir = provideLedgerRound(t, "4", "")
	tests = append(tests, &testCase{
		_id:              "void-without-round-dir",
		_expected:        exitError,
		_expectedWritten: false,

		args:     []string{"--void"},
		roundDir: roundDir,
	})

	// void-with-schema
	roundDir = provideLedgerRound(t, "5", "")
	tests = append(tests, &testCase{
		_id:              "void-with-schema",
		_expected:        exitError,
		_expectedWritten: false,

		args:     []string{"--round-dir", roundDir, "--void", "--schema", provideLedgerFile(t, ledgerPermissiveSchema)},
		roundDir: roundDir,
	})

	// void-with-known
	roundDir = provideLedgerRound(t, "6", "")
	tests = append(tests, &testCase{
		_id:              "void-with-known",
		_expected:        exitError,
		_expectedWritten: false,

		args:     []string{"--round-dir", roundDir, "--void", "--known", filepath.Join(t.TempDir(), "known.json")},
		roundDir: roundDir,
	})

	// schema-default-deployed
	roundDir = provideLedgerRound(t, "1", ledgerReview)
	tests = append(tests, &testCase{
		_id:              "schema-default-deployed",
		_expected:        exitClean,
		_expectedWritten: true,

		args:     ledgerArgs(roundDir),
		roundDir: roundDir,
	})

	// schema-explicit-violated
	roundDir = provideLedgerRound(t, "1", ledgerReview)
	tests = append(tests, &testCase{
		_id:              "schema-explicit-violated",
		_expected:        exitError,
		_expectedWritten: false,

		args:     ledgerArgs(roundDir, "--schema", provideLedgerFile(t, ledgerStrictSchema)),
		roundDir: roundDir,
	})

	// fix-state-applied
	knownPath := provideKnownLedger(t, reviewscope.FixStateOpen)
	tests = append(tests, &testCase{
		_id:               "fix-state-applied",
		_expected:         exitClean,
		_expectedFixState: reviewscope.FixStateApplied,
		_expectedWritten:  false,

		args:      []string{"--known", knownPath, "--round", "1", "--fix-state", "applied", "--ids", "a"},
		knownPath: knownPath,
		roundDir:  t.TempDir(),
	})

	// fix-state-fixed
	knownPath = provideKnownLedger(t, reviewscope.FixStateApplied)
	tests = append(tests, &testCase{
		_id:               "fix-state-fixed",
		_expected:         exitClean,
		_expectedFixState: reviewscope.FixStateFixed,
		_expectedWritten:  false,

		args:      []string{"--known", knownPath, "--round", "1", "--fix-state", "fixed", "--ids", "a"},
		knownPath: knownPath,
		roundDir:  t.TempDir(),
	})

	// fix-state-without-round
	knownPath = provideKnownLedger(t, reviewscope.FixStateOpen)
	tests = append(tests, &testCase{
		_id:               "fix-state-without-round",
		_expected:         exitError,
		_expectedFixState: reviewscope.FixStateOpen,
		_expectedWritten:  false,

		args:      []string{"--known", knownPath, "--fix-state", "applied", "--ids", "a"},
		knownPath: knownPath,
		roundDir:  t.TempDir(),
	})

	// fix-state-without-ids
	knownPath = provideKnownLedger(t, reviewscope.FixStateOpen)
	tests = append(tests, &testCase{
		_id:               "fix-state-without-ids",
		_expected:         exitError,
		_expectedFixState: reviewscope.FixStateOpen,
		_expectedWritten:  false,

		args:      []string{"--known", knownPath, "--round", "1", "--fix-state", "applied"},
		knownPath: knownPath,
		roundDir:  t.TempDir(),
	})

	// fix-state-forbidden-move
	knownPath = provideKnownLedger(t, reviewscope.FixStateOpen)
	tests = append(tests, &testCase{
		_id:               "fix-state-forbidden-move",
		_expected:         exitError,
		_expectedFixState: reviewscope.FixStateOpen,
		_expectedWritten:  false,

		args:      []string{"--known", knownPath, "--round", "1", "--fix-state", "fixed", "--ids", "a"},
		knownPath: knownPath,
		roundDir:  t.TempDir(),
	})

	// autofix-level-default-minor
	roundDir = provideLedgerRound(t, "1", ledgerReviewSeverities)
	tests = append(tests, &testCase{
		_id:                  "autofix-level-default-minor",
		_expected:            exitClean,
		_expectedAutofix:     []string{"minor-1"},
		_expectedIgnored:     []string{},
		_expectedOscillating: []string{},
		_expectedWritten:     true,

		args:       ledgerArgs(roundDir),
		roundDir:   roundDir,
		stdoutPath: filepath.Join(t.TempDir(), "stdout.json"),
	})

	// ignore-level-flag
	roundDir = provideLedgerRound(t, "1", ledgerReviewNit)
	tests = append(tests, &testCase{
		_id:                  "ignore-level-flag",
		_expected:            exitClean,
		_expectedAutofix:     []string{},
		_expectedFixState:    reviewscope.FixStateIgnored,
		_expectedIgnored:     []string{"nit-1"},
		_expectedOscillating: []string{},
		_expectedWritten:     true,

		args:       ledgerArgs(roundDir, "--ignore-level", "nit"),
		knownPath:  filepath.Join(filepath.Dir(roundDir), "known.json"),
		roundDir:   roundDir,
		stdoutPath: filepath.Join(t.TempDir(), "stdout.json"),
	})

	// ignore-level-unknown
	roundDir = provideLedgerRound(t, "1", ledgerReviewNit)
	tests = append(tests, &testCase{
		_id:              "ignore-level-unknown",
		_expected:        exitError,
		_expectedWritten: false,

		args:     ledgerArgs(roundDir, "--ignore-level", "trivial"),
		roundDir: roundDir,
	})

	// oscillating-exits-clean
	roundDir = provideLedgerRound(t, "2", ledgerReviewReconfirmed)
	knownPath = filepath.Join(filepath.Dir(roundDir), "known.json")
	reconfirmedKnown := []reviewscope.KnownEntry{{File: "a.go", FindingId: "a", FixState: reviewscope.FixStateFixed, Round: 1, Route: "auto-fix", Severity: "MINOR"}}
	reconfirmedData, err := json.Marshal(reconfirmedKnown)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(knownPath, reconfirmedData, 0o644))
	tests = append(tests, &testCase{
		_id:                  "oscillating-exits-clean",
		_expected:            exitClean,
		_expectedAutofix:     []string{},
		_expectedFixState:    reviewscope.FixStateFixed,
		_expectedIgnored:     []string{},
		_expectedOscillating: []string{"a"},
		_expectedWritten:     true,

		args:       ledgerArgs(roundDir),
		knownPath:  knownPath,
		roundDir:   roundDir,
		stdoutPath: filepath.Join(t.TempDir(), "stdout.json"),
	})

	// unfix-flag-gone
	knownPath = provideKnownLedger(t, reviewscope.FixStateFixed)
	tests = append(tests, &testCase{
		_id:               "unfix-flag-gone",
		_expected:         exitError,
		_expectedFixState: reviewscope.FixStateFixed,
		_expectedWritten:  false,

		args:      []string{"--known", knownPath, "--unfix", "a"},
		knownPath: knownPath,
		roundDir:  t.TempDir(),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, runLedgerTo(t, test.stdoutPath, test.args))
			assert.Equal(t, test._expectedWritten, isPresent(filepath.Join(filepath.Dir(test.roundDir), "known.json")))
			assert.Equal(t, test._expectedAutofix, readLedgerResult(t, test.stdoutPath).Autofix)
			assert.Equal(t, test._expectedFixState, readFixState(t, test.knownPath))
			assert.Equal(t, test._expectedIgnored, readLedgerResult(t, test.stdoutPath).Ignored)
			assert.Equal(t, test._expectedOscillating, readLedgerResult(t, test.stdoutPath).Oscillating)
		})
	}
}

func TestRunVoid_CountsVoidRounds(t *testing.T) {
	artifactsDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(artifactsDir, "round-1"), 0o755))
	roundDir := filepath.Join(artifactsDir, "round-2")
	require.NoError(t, os.MkdirAll(roundDir, 0o755))
	stdoutPath := filepath.Join(t.TempDir(), "stdout.json")

	voidArgs := []string{"--round-dir", roundDir, "--void"}
	assert.Equal(t, exitClean, runLedgerTo(t, stdoutPath, voidArgs))

	printed, err := os.ReadFile(stdoutPath)
	require.NoError(t, err)
	assert.JSONEq(t, `{"void": true, "void_rounds": 2}`, string(printed))
}

func TestRunLedger_DeployedSchemaMissing(t *testing.T) {
	type testCase struct {
		_id              string
		_expected        int
		_expectedWritten bool

		args     []string
		roundDir string
	}

	provideDeployedSchema(t, "")
	tests := make([]*testCase, 0)

	// default-schema-missing
	roundDir := provideLedgerRound(t, "1", ledgerReview)
	tests = append(tests, &testCase{
		_id:              "default-schema-missing",
		_expected:        exitError,
		_expectedWritten: false,

		args:     ledgerArgs(roundDir),
		roundDir: roundDir,
	})

	// explicit-schema-wins
	roundDir = provideLedgerRound(t, "1", ledgerReview)
	tests = append(tests, &testCase{
		_id:              "explicit-schema-wins",
		_expected:        exitClean,
		_expectedWritten: true,

		args:     ledgerArgs(roundDir, "--schema", provideLedgerFile(t, ledgerPermissiveSchema)),
		roundDir: roundDir,
	})

	// void-needs-no-schema
	roundDir = provideLedgerRound(t, "2", "")
	tests = append(tests, &testCase{
		_id:              "void-needs-no-schema",
		_expected:        exitClean,
		_expectedWritten: false,

		args:     []string{"--round-dir", roundDir, "--void"},
		roundDir: roundDir,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, runLedger(test.args))
			assert.Equal(t, test._expectedWritten, isPresent(filepath.Join(filepath.Dir(test.roundDir), "known.json")))
		})
	}
}

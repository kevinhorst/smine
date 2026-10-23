package reviewscope

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// permissiveSchema accepts every review.json object.
const permissiveSchema = `{"type": "object"}`

// ledgerFixture is one artifacts dir holding a round dir, known.json and
// coverage.json.
type ledgerFixture struct {
	artifactsDir string
	roundDir     string
	t            *testing.T
}

// newLedgerFixture creates round-<round> with a review.json of the ranked
// entries.
func newLedgerFixture(t *testing.T, round string, ranked ...rankedEntry) *ledgerFixture {
	t.Helper()
	fixture := &ledgerFixture{artifactsDir: t.TempDir(), t: t}
	fixture.roundDir = filepath.Join(fixture.artifactsDir, "round-"+round)
	require.NoError(t, os.MkdirAll(fixture.roundDir, 0o755))
	review := &reviewDocument{Probes: []reviewProbe{}, RankedPlan: ranked}
	fixture.write(filepath.Join(fixture.roundDir, reviewFile), review)
	return fixture
}

// coverage writes coverage.json.
func (f *ledgerFixture) coverage(ledger CoverageLedger) *ledgerFixture {
	f.write(f.coveragePath(), ledger)
	return f
}

func (f *ledgerFixture) coveragePath() string {
	return filepath.Join(f.artifactsDir, "coverage.json")
}

// known writes known.json.
func (f *ledgerFixture) known(rows ...KnownEntry) *ledgerFixture {
	f.write(f.knownPath(), rows)
	return f
}

func (f *ledgerFixture) knownPath() string {
	return filepath.Join(f.artifactsDir, knownFile)
}

// lane writes one lane manifest and, when finished, its walk-complete artifact.
func (f *ledgerFixture) lane(slug, direction string, files, partners []string, isFinished bool) *ledgerFixture {
	artifact := filepath.Join(f.roundDir, slug+".json")
	manifest := &LaneManifest{Artifact: artifact, Direction: direction, Files: files, Partners: partners}
	f.write(artifact+LaneManifestSuffix, manifest)
	state := &laneState{WalkComplete: isFinished}
	f.write(artifact, state)
	return f
}

// request builds the ledger request over the fixture's paths, validated
// against a permissive consolidation schema written beside the ledgers.
func (f *ledgerFixture) request(autofixLevel string, doneAfter int) *LedgerRequest {
	f.t.Helper()
	schemaPath := filepath.Join(f.artifactsDir, "consolidation.json")
	require.NoError(f.t, os.WriteFile(schemaPath, []byte(permissiveSchema), 0o644))
	return &LedgerRequest{AutofixLevel: autofixLevel, CoveragePath: f.coveragePath(), DoneAfter: doneAfter, KnownPath: f.knownPath(), RoundDir: f.roundDir, SchemaPath: schemaPath}
}

func (f *ledgerFixture) write(path string, value any) {
	f.t.Helper()
	data, err := json.Marshal(value)
	require.NoError(f.t, err)
	require.NoError(f.t, os.WriteFile(path, data, 0o644))
}

// provideKnownBytes is known.json as SetFixState writes it.
func provideKnownBytes(t *testing.T, rows ...KnownEntry) []byte {
	data, err := json.MarshalIndent(rows, "", "  ")
	require.NoError(t, err)
	return append(data, '\n')
}

// provideKnownEntry is a known row matching provideRankedEntry.
func provideKnownEntry(file, findingId, severity string, fixState FixState, round int) KnownEntry {
	return KnownEntry{
		Claim:     "claim " + findingId,
		Effect:    "incorrect",
		Exposure:  "every caller",
		File:      file,
		FindingId: findingId,
		Fix:       "fix " + findingId,
		FixState:  fixState,
		Line:      7,
		Round:     round,
		Route:     routeAutoFix,
		Severity:  severity,
	}
}

// provideRankedEntry is a complete ranked_plan entry.
func provideRankedEntry(file, findingId, severity string) rankedEntry {
	return rankedEntry{
		Claim:       "claim " + findingId,
		Effect:      "incorrect",
		Exposure:    "every caller",
		File:        file,
		FindingId:   findingId,
		Fix:         "fix " + findingId,
		Known:       false,
		KnownId:     "",
		Line:        7,
		Route:       routeAutoFix,
		RouteReason: "reason " + findingId,
		Severity:    severity,
	}
}

// readCoverageFile decodes coverage.json; nil when absent.
func readCoverageFile(t *testing.T, path string) CoverageLedger {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	ledger := CoverageLedger{}
	require.NoError(t, json.Unmarshal(data, &ledger))
	return ledger
}

// readKnownBytes is the raw content of a known.json; nil when absent.
func readKnownBytes(t *testing.T, path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return data
}

// readKnownFile decodes a known.json; nil when absent.
func readKnownFile(t *testing.T, path string) []KnownEntry {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var known []KnownEntry
	require.NoError(t, json.Unmarshal(data, &known))
	return known
}

func TestUpdateLedger(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedCoverage CoverageLedger
		_expectedKnown    []KnownEntry
		_expectedResult   *LedgerResult
		_shouldPass       bool

		fixture *ledgerFixture
		request *LedgerRequest
	}

	tests := make([]*testCase, 0)

	// autofix-split-blocker
	fixture := newLedgerFixture(t, "1", provideRankedEntry("a.go", "f1", "BLOCKER"), provideRankedEntry("b.go", "f2", "NIT"))
	tests = append(tests, &testCase{
		_id:               "autofix-split-blocker",
		_expectedCoverage: CoverageLedger{},
		_expectedKnown:    []KnownEntry{provideKnownEntry("a.go", "f1", "BLOCKER", FixStateOpen, 1), provideKnownEntry("b.go", "f2", "NIT", FixStateOpen, 1)},
		_expectedResult:   &LedgerResult{Autofix: []string{"f1", "f2"}, CoverageGaps: []string{}, Ignored: []string{}, Oscillating: []string{}, Record: []string{}},
		_shouldPass:       true,

		fixture: fixture,
		request: fixture.request("blocker", 0),
	})

	// autofix-split-minor
	fixture = newLedgerFixture(t, "1", provideRankedEntry("a.go", "f1", "MAJOR"), provideRankedEntry("b.go", "f2", "MINOR"))
	tests = append(tests, &testCase{
		_id:               "autofix-split-minor",
		_expectedCoverage: CoverageLedger{},
		_expectedKnown:    []KnownEntry{provideKnownEntry("a.go", "f1", "MAJOR", FixStateOpen, 1), provideKnownEntry("b.go", "f2", "MINOR", FixStateOpen, 1)},
		_expectedResult:   &LedgerResult{Autofix: []string{"f2"}, CoverageGaps: []string{}, Ignored: []string{}, Oscillating: []string{}, Record: []string{"f1"}},
		_shouldPass:       true,

		fixture: fixture,
		request: fixture.request("minor", 0),
	})

	// autofix-level-none
	fixture = newLedgerFixture(t, "1", provideRankedEntry("a.go", "f1", "INFO"))
	tests = append(tests, &testCase{
		_id:               "autofix-level-none",
		_expectedCoverage: CoverageLedger{},
		_expectedKnown:    []KnownEntry{provideKnownEntry("a.go", "f1", "INFO", FixStateOpen, 1)},
		_expectedResult:   &LedgerResult{Autofix: []string{}, CoverageGaps: []string{}, Ignored: []string{}, Oscillating: []string{}, Record: []string{"f1"}},
		_shouldPass:       true,

		fixture: fixture,
		request: fixture.request("none", 0),
	})

	// human-route-recorded
	humanRanked := provideRankedEntry("a.go", "f1", "NIT")
	humanRanked.Route = "human"
	humanKnown := provideKnownEntry("a.go", "f1", "NIT", FixStateOpen, 1)
	humanKnown.Route = "human"
	fixture = newLedgerFixture(t, "1", humanRanked, provideRankedEntry("b.go", "f2", "NIT"))
	tests = append(tests, &testCase{
		_id:               "human-route-recorded",
		_expectedCoverage: CoverageLedger{},
		_expectedKnown:    []KnownEntry{humanKnown, provideKnownEntry("b.go", "f2", "NIT", FixStateOpen, 1)},
		_expectedResult:   &LedgerResult{Autofix: []string{"f2"}, CoverageGaps: []string{}, Ignored: []string{}, Oscillating: []string{}, Record: []string{"f1"}},
		_shouldPass:       true,

		fixture: fixture,
		request: fixture.request("blocker", 0),
	})

	// rows-appended-open
	previous := provideKnownEntry("z.go", "f9", "MAJOR", FixStateFixed, 1)
	fixture = newLedgerFixture(t, "2", provideRankedEntry("a.go", "f1", "MINOR")).known(previous)
	tests = append(tests, &testCase{
		_id:               "rows-appended-open",
		_expectedCoverage: CoverageLedger{},
		_expectedKnown:    []KnownEntry{previous, provideKnownEntry("a.go", "f1", "MINOR", FixStateOpen, 2)},
		_expectedResult:   &LedgerResult{Autofix: []string{"f1"}, CoverageGaps: []string{}, Ignored: []string{}, Oscillating: []string{}, Record: []string{}},
		_shouldPass:       true,

		fixture: fixture,
		request: fixture.request("blocker", 0),
	})

	// known-excluded
	knownEntry := provideRankedEntry("a.go", "f1", "BLOCKER")
	knownEntry.Known = true
	fixture = newLedgerFixture(t, "1", knownEntry, provideRankedEntry("b.go", "f2", "MAJOR"))
	tests = append(tests, &testCase{
		_id:               "known-excluded",
		_expectedCoverage: CoverageLedger{},
		_expectedKnown:    []KnownEntry{provideKnownEntry("b.go", "f2", "MAJOR", FixStateOpen, 1)},
		_expectedResult:   &LedgerResult{Autofix: []string{"f2"}, CoverageGaps: []string{}, Ignored: []string{}, Oscillating: []string{}, Record: []string{}},
		_shouldPass:       true,

		fixture: fixture,
		request: fixture.request("blocker", 0),
	})

	// clean-file-credited-once-per-round
	fixture = newLedgerFixture(t, "1").
		lane("critical-lane1", directionCritical, []string{"c.go"}, []string{}, true).
		lane("critical-lane2", directionCritical, []string{"c.go"}, []string{}, true)
	tests = append(tests, &testCase{
		_id:               "clean-file-credited-once-per-round",
		_expectedCoverage: CoverageLedger{directionCritical: {"c.go": {CleanReviews: 1, Done: false, LastRound: 1}}},
		_expectedKnown:    []KnownEntry{},
		_expectedResult:   &LedgerResult{Autofix: []string{}, CoverageGaps: []string{}, Ignored: []string{}, Oscillating: []string{}, Record: []string{}},
		_shouldPass:       true,

		fixture: fixture,
		request: fixture.request("blocker", 0),
	})

	// dirty-file-reset-every-direction
	fixture = newLedgerFixture(t, "2", provideRankedEntry("a.go", "f1", "MAJOR")).
		coverage(CoverageLedger{
			directionCritical: {"a.go": {CleanReviews: 2, Done: false, LastRound: 1}},
			directionTests:    {"a.go": {CleanReviews: 1, Done: false, LastRound: 1}, "b.go": {CleanReviews: 1, Done: false, LastRound: 1}},
		}).
		lane("critical-lane1", directionCritical, []string{"a.go"}, []string{}, true)
	tests = append(tests, &testCase{
		_id: "dirty-file-reset-every-direction",
		_expectedCoverage: CoverageLedger{
			directionCritical: {"a.go": {CleanReviews: 0, Done: false, LastRound: 2}},
			directionTests:    {"a.go": {CleanReviews: 0, Done: false, LastRound: 2}, "b.go": {CleanReviews: 1, Done: false, LastRound: 1}},
		},
		_expectedKnown:  []KnownEntry{provideKnownEntry("a.go", "f1", "MAJOR", FixStateOpen, 2)},
		_expectedResult: &LedgerResult{Autofix: []string{"f1"}, CoverageGaps: []string{}, Ignored: []string{}, Oscillating: []string{}, Record: []string{}},
		_shouldPass:     true,

		fixture: fixture,
		request: fixture.request("blocker", 0),
	})

	// done-after-threshold
	fixture = newLedgerFixture(t, "3").
		coverage(CoverageLedger{directionCritical: {"c.go": {CleanReviews: 2, Done: false, LastRound: 2}}}).
		lane("critical-lane1", directionCritical, []string{"c.go"}, []string{}, true).
		lane("code-style-lane1", directionCodeStyle, []string{"c.go"}, []string{}, true).
		lane("tests-lane1", directionTests, []string{"c.go"}, []string{}, true)
	tests = append(tests, &testCase{
		_id: "done-after-threshold",
		_expectedCoverage: CoverageLedger{
			directionCodeStyle: {"c.go": {CleanReviews: 1, Done: true, LastRound: 3}},
			directionCritical:  {"c.go": {CleanReviews: 3, Done: true, LastRound: 3}},
			directionTests:     {"c.go": {CleanReviews: 1, Done: false, LastRound: 3}},
		},
		_expectedKnown:  []KnownEntry{},
		_expectedResult: &LedgerResult{Autofix: []string{}, CoverageGaps: []string{}, Ignored: []string{}, Oscillating: []string{}, Record: []string{}},
		_shouldPass:     true,

		fixture: fixture,
		request: fixture.request("blocker", 0),
	})

	// done-after-uniform
	fixture = newLedgerFixture(t, "1").
		lane("tests-lane1", directionTests, []string{"c.go"}, []string{}, true)
	tests = append(tests, &testCase{
		_id:               "done-after-uniform",
		_expectedCoverage: CoverageLedger{directionTests: {"c.go": {CleanReviews: 1, Done: true, LastRound: 1}}},
		_expectedKnown:    []KnownEntry{},
		_expectedResult:   &LedgerResult{Autofix: []string{}, CoverageGaps: []string{}, Ignored: []string{}, Oscillating: []string{}, Record: []string{}},
		_shouldPass:       true,

		fixture: fixture,
		request: fixture.request("blocker", 1),
	})

	// unfinished-lane-gap
	fixture = newLedgerFixture(t, "1").
		lane("critical-lane1", directionCritical, []string{"c.go", "d.go"}, []string{}, false).
		lane("critical-lane2", directionCritical, []string{"c.go"}, []string{}, true).
		lane("tests-lane1", directionTests, []string{"e.go", "e_test.go"}, []string{"e_test.go"}, false)
	tests = append(tests, &testCase{
		_id:               "unfinished-lane-gap",
		_expectedCoverage: CoverageLedger{directionCritical: {"c.go": {CleanReviews: 1, Done: false, LastRound: 1}}},
		_expectedKnown:    []KnownEntry{},
		_expectedResult:   &LedgerResult{Autofix: []string{}, CoverageGaps: []string{"d.go", "e.go"}, Ignored: []string{}, Oscillating: []string{}, Record: []string{}},
		_shouldPass:       true,

		fixture: fixture,
		request: fixture.request("blocker", 0),
	})

	// ignored-at-or-below-level
	fixture = newLedgerFixture(t, "1", provideRankedEntry("a.go", "f1", "MAJOR"), provideRankedEntry("b.go", "f2", "NIT"), provideRankedEntry("c.go", "f3", "INFO"))
	ignoreNitRequest := fixture.request("minor", 0)
	ignoreNitRequest.IgnoreLevel = "nit"
	tests = append(tests, &testCase{
		_id:               "ignored-at-or-below-level",
		_expectedCoverage: CoverageLedger{},
		_expectedKnown: []KnownEntry{
			provideKnownEntry("a.go", "f1", "MAJOR", FixStateOpen, 1),
			provideKnownEntry("b.go", "f2", "NIT", FixStateIgnored, 1),
			provideKnownEntry("c.go", "f3", "INFO", FixStateIgnored, 1),
		},
		_expectedResult: &LedgerResult{Autofix: []string{}, CoverageGaps: []string{}, Ignored: []string{"f2", "f3"}, Oscillating: []string{}, Record: []string{"f1"}},
		_shouldPass:     true,

		fixture: fixture,
		request: ignoreNitRequest,
	})

	// ignore-level-none-keeps-open
	fixture = newLedgerFixture(t, "1", provideRankedEntry("a.go", "f1", "INFO"))
	ignoreNoneRequest := fixture.request("none", 0)
	ignoreNoneRequest.IgnoreLevel = "none"
	tests = append(tests, &testCase{
		_id:               "ignore-level-none-keeps-open",
		_expectedCoverage: CoverageLedger{},
		_expectedKnown:    []KnownEntry{provideKnownEntry("a.go", "f1", "INFO", FixStateOpen, 1)},
		_expectedResult:   &LedgerResult{Autofix: []string{}, CoverageGaps: []string{}, Ignored: []string{}, Oscillating: []string{}, Record: []string{"f1"}},
		_shouldPass:       true,

		fixture: fixture,
		request: ignoreNoneRequest,
	})

	// unknown-autofix-level-error
	fixture = newLedgerFixture(t, "1", provideRankedEntry("a.go", "f1", "BLOCKER"))
	tests = append(tests, &testCase{
		_id:               "unknown-autofix-level-error",
		_expectedCoverage: nil,
		_expectedKnown:    nil,
		_expectedResult:   nil,
		_shouldPass:       false,

		fixture: fixture,
		request: fixture.request("severe", 0),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			result, err := UpdateLedger(test.request)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedResult, result)
			assert.Equal(t, test._expectedKnown, readKnownFile(t, test.fixture.knownPath()))
			assert.Equal(t, test._expectedCoverage, readCoverageFile(t, test.fixture.coveragePath()))
		})
	}
}

func TestUpdateLedger_SchemaRequired(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedFragment string

		fixture *ledgerFixture
		request *LedgerRequest
	}

	tests := make([]*testCase, 0)

	// schema-missing
	fixture := newLedgerFixture(t, "1", provideRankedEntry("a.go", "f1", "BLOCKER"))
	missing := filepath.Join(fixture.artifactsDir, "absent.json")
	request := fixture.request("blocker", 0)
	request.SchemaPath = missing
	tests = append(tests, &testCase{
		_id:               "schema-missing",
		_expectedFragment: missing,

		fixture: fixture,
		request: request,
	})

	// schema-path-empty
	fixture = newLedgerFixture(t, "1", provideRankedEntry("a.go", "f1", "BLOCKER"))
	request = fixture.request("blocker", 0)
	request.SchemaPath = ""
	tests = append(tests, &testCase{
		_id:               "schema-path-empty",
		_expectedFragment: "Missing field SchemaPath",

		fixture: fixture,
		request: request,
	})

	// ignore-level-unknown
	fixture = newLedgerFixture(t, "1", provideRankedEntry("a.go", "f1", "BLOCKER"))
	request = fixture.request("blocker", 0)
	request.IgnoreLevel = "loud"
	tests = append(tests, &testCase{
		_id:               "ignore-level-unknown",
		_expectedFragment: "Unknown ignore level loud",

		fixture: fixture,
		request: request,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			_, err := UpdateLedger(test.request)
			require.ErrorContains(t, err, test._expectedFragment)
			assert.NoFileExists(t, test.fixture.knownPath())
		})
	}
}

func TestSetFixState(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedBytes    []byte
		_expectedFragment string
		_shouldPass       bool

		request *FixStateRequest
	}

	open := provideKnownEntry("a.go", "f1", "MAJOR", FixStateOpen, 1)
	applied := provideKnownEntry("a.go", "f1", "MAJOR", FixStateApplied, 1)
	fixed := provideKnownEntry("a.go", "f1", "MAJOR", FixStateFixed, 1)
	ignored := provideKnownEntry("a.go", "f1", "NIT", FixStateIgnored, 1)
	other := provideKnownEntry("b.go", "f2", "MINOR", FixStateOpen, 1)
	openNextRound := provideKnownEntry("a.go", "f1", "MAJOR", FixStateOpen, 2)
	appliedNextRound := provideKnownEntry("a.go", "f1", "MAJOR", FixStateApplied, 2)
	tests := make([]*testCase, 0)

	// pass-open-to-applied
	fixture := newLedgerFixture(t, "1").known(open, other)
	tests = append(tests, &testCase{
		_id:               "pass-open-to-applied",
		_expectedBytes:    provideKnownBytes(t, applied, other),
		_expectedFragment: "",
		_shouldPass:       true,

		request: &FixStateRequest{Ids: []string{"f1"}, KnownPath: fixture.knownPath(), Round: 1, State: FixStateApplied},
	})

	// pass-applied-to-fixed
	fixture = newLedgerFixture(t, "1").known(applied, other)
	tests = append(tests, &testCase{
		_id:               "pass-applied-to-fixed",
		_expectedBytes:    provideKnownBytes(t, fixed, other),
		_expectedFragment: "",
		_shouldPass:       true,

		request: &FixStateRequest{Ids: []string{"f1"}, KnownPath: fixture.knownPath(), Round: 1, State: FixStateFixed},
	})

	// pass-fixed-to-applied
	fixture = newLedgerFixture(t, "1").known(fixed, other)
	tests = append(tests, &testCase{
		_id:               "pass-fixed-to-applied",
		_expectedBytes:    provideKnownBytes(t, applied, other),
		_expectedFragment: "",
		_shouldPass:       true,

		request: &FixStateRequest{Ids: []string{"f1"}, KnownPath: fixture.knownPath(), Round: 1, State: FixStateApplied},
	})

	// pass-same-state-kept
	fixture = newLedgerFixture(t, "1").known(applied, other)
	tests = append(tests, &testCase{
		_id:               "pass-same-state-kept",
		_expectedBytes:    provideKnownBytes(t, applied, other),
		_expectedFragment: "",
		_shouldPass:       true,

		request: &FixStateRequest{Ids: []string{"f1"}, KnownPath: fixture.knownPath(), Round: 1, State: FixStateApplied},
	})

	// pass-other-round-untouched
	fixture = newLedgerFixture(t, "2").known(open, openNextRound)
	tests = append(tests, &testCase{
		_id:               "pass-other-round-untouched",
		_expectedBytes:    provideKnownBytes(t, open, appliedNextRound),
		_expectedFragment: "",
		_shouldPass:       true,

		request: &FixStateRequest{Ids: []string{"f1"}, KnownPath: fixture.knownPath(), Round: 2, State: FixStateApplied},
	})

	// fail-open-to-fixed
	fixture = newLedgerFixture(t, "1").known(open, other)
	tests = append(tests, &testCase{
		_id:               "fail-open-to-fixed",
		_expectedBytes:    readKnownBytes(t, fixture.knownPath()),
		_expectedFragment: "SetFixState: Row f1 cannot move from open to fixed",
		_shouldPass:       false,

		request: &FixStateRequest{Ids: []string{"f1"}, KnownPath: fixture.knownPath(), Round: 1, State: FixStateFixed},
	})

	// fail-ignored-to-applied
	fixture = newLedgerFixture(t, "1").known(ignored, other)
	tests = append(tests, &testCase{
		_id:               "fail-ignored-to-applied",
		_expectedBytes:    readKnownBytes(t, fixture.knownPath()),
		_expectedFragment: "SetFixState: Row f1 cannot move from ignored to applied",
		_shouldPass:       false,

		request: &FixStateRequest{Ids: []string{"f1"}, KnownPath: fixture.knownPath(), Round: 1, State: FixStateApplied},
	})

	// fail-ignored-to-fixed
	fixture = newLedgerFixture(t, "1").known(ignored, other)
	tests = append(tests, &testCase{
		_id:               "fail-ignored-to-fixed",
		_expectedBytes:    readKnownBytes(t, fixture.knownPath()),
		_expectedFragment: "SetFixState: Row f1 cannot move from ignored to fixed",
		_shouldPass:       false,

		request: &FixStateRequest{Ids: []string{"f1"}, KnownPath: fixture.knownPath(), Round: 1, State: FixStateFixed},
	})

	// fail-target-open
	fixture = newLedgerFixture(t, "1").known(applied, other)
	tests = append(tests, &testCase{
		_id:               "fail-target-open",
		_expectedBytes:    readKnownBytes(t, fixture.knownPath()),
		_expectedFragment: "SetFixState: State open is not a target state",
		_shouldPass:       false,

		request: &FixStateRequest{Ids: []string{"f1"}, KnownPath: fixture.knownPath(), Round: 1, State: FixStateOpen},
	})

	// fail-unknown-state
	fixture = newLedgerFixture(t, "1").known(applied, other)
	tests = append(tests, &testCase{
		_id:               "fail-unknown-state",
		_expectedBytes:    readKnownBytes(t, fixture.knownPath()),
		_expectedFragment: "SetFixState: State done is not a target state",
		_shouldPass:       false,

		request: &FixStateRequest{Ids: []string{"f1"}, KnownPath: fixture.knownPath(), Round: 1, State: FixState("done")},
	})

	// fail-unknown-id
	fixture = newLedgerFixture(t, "1").known(open, other)
	tests = append(tests, &testCase{
		_id:               "fail-unknown-id",
		_expectedBytes:    readKnownBytes(t, fixture.knownPath()),
		_expectedFragment: "SetFixState: Round 1 holds no row f7",
		_shouldPass:       false,

		request: &FixStateRequest{Ids: []string{"f7"}, KnownPath: fixture.knownPath(), Round: 1, State: FixStateApplied},
	})

	// fail-id-of-other-round
	fixture = newLedgerFixture(t, "2").known(open, other)
	tests = append(tests, &testCase{
		_id:               "fail-id-of-other-round",
		_expectedBytes:    readKnownBytes(t, fixture.knownPath()),
		_expectedFragment: "SetFixState: Round 2 holds no row f1",
		_shouldPass:       false,

		request: &FixStateRequest{Ids: []string{"f1"}, KnownPath: fixture.knownPath(), Round: 2, State: FixStateApplied},
	})

	// fail-writes-nothing
	fixture = newLedgerFixture(t, "1").known(applied, other)
	tests = append(tests, &testCase{
		_id:               "fail-writes-nothing",
		_expectedBytes:    readKnownBytes(t, fixture.knownPath()),
		_expectedFragment: "SetFixState: Row f2 cannot move from open to fixed",
		_shouldPass:       false,

		request: &FixStateRequest{Ids: []string{"f1", "f2"}, KnownPath: fixture.knownPath(), Round: 1, State: FixStateFixed},
	})

	// fail-missing-ledger
	fixture = newLedgerFixture(t, "1")
	tests = append(tests, &testCase{
		_id:               "fail-missing-ledger",
		_expectedBytes:    nil,
		_expectedFragment: "readJson",
		_shouldPass:       false,

		request: &FixStateRequest{Ids: []string{"f1"}, KnownPath: fixture.knownPath(), Round: 1, State: FixStateApplied},
	})

	// fail-malformed-ledger
	fixture = newLedgerFixture(t, "1")
	require.NoError(t, os.WriteFile(fixture.knownPath(), []byte("{not json"), 0o644))
	tests = append(tests, &testCase{
		_id:               "fail-malformed-ledger",
		_expectedBytes:    []byte("{not json"),
		_expectedFragment: fixture.knownPath(),
		_shouldPass:       false,

		request: &FixStateRequest{Ids: []string{"f1"}, KnownPath: fixture.knownPath(), Round: 1, State: FixStateApplied},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := SetFixState(test.request)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Contains(t, fmt.Sprint(err), test._expectedFragment)
			assert.Equal(t, test._expectedBytes, readKnownBytes(t, test.request.KnownPath))
		})
	}
}

func TestVoidRound(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedFragment string
		_shouldPass       bool

		roundDir string
	}

	artifactsDir := t.TempDir()
	tests := make([]*testCase, 0)

	// round-without-review
	empty := filepath.Join(artifactsDir, "round-1")
	require.NoError(t, os.MkdirAll(empty, 0o755))
	tests = append(tests, &testCase{
		_id:               "round-without-review",
		_expectedFragment: "",
		_shouldPass:       true,

		roundDir: empty,
	})

	// review-present
	handoff := newLedgerFixture(t, "2").roundDir
	tests = append(tests, &testCase{
		_id:               "review-present",
		_expectedFragment: filepath.Join(handoff, reviewFile),
		_shouldPass:       false,

		roundDir: handoff,
	})

	// review-dangling-symlink
	dangling := filepath.Join(artifactsDir, "round-3")
	require.NoError(t, os.MkdirAll(dangling, 0o755))
	require.NoError(t, os.Symlink(filepath.Join(artifactsDir, "absent.json"), filepath.Join(dangling, reviewFile)))
	tests = append(tests, &testCase{
		_id:               "review-dangling-symlink",
		_expectedFragment: filepath.Join(dangling, reviewFile),
		_shouldPass:       false,

		roundDir: dangling,
	})

	// round-dir-missing
	tests = append(tests, &testCase{
		_id:               "round-dir-missing",
		_expectedFragment: "round-4",
		_shouldPass:       false,

		roundDir: filepath.Join(artifactsDir, "round-4"),
	})

	// round-dir-is-file
	plainFile := filepath.Join(artifactsDir, "round-5")
	require.NoError(t, os.WriteFile(plainFile, []byte("x"), 0o644))
	tests = append(tests, &testCase{
		_id:               "round-dir-is-file",
		_expectedFragment: "is not a directory",
		_shouldPass:       false,

		roundDir: plainFile,
	})

	// not-a-round-dir
	other := filepath.Join(artifactsDir, "lanes")
	require.NoError(t, os.MkdirAll(other, 0o755))
	tests = append(tests, &testCase{
		_id:               "not-a-round-dir",
		_expectedFragment: "is not round-<n>",
		_shouldPass:       false,

		roundDir: other,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := VoidRound(test.roundDir)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Contains(t, fmt.Sprint(err), test._expectedFragment)
			assert.NoFileExists(t, filepath.Join(test.roundDir, knownFile))
		})
	}
}

func TestVoidRounds(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   int
		_shouldPass bool

		roundDir string
	}

	tests := make([]*testCase, 0)

	// one-void-round
	single := filepath.Join(t.TempDir(), "round-1")
	require.NoError(t, os.MkdirAll(single, 0o755))
	tests = append(tests, &testCase{
		_id:         "one-void-round",
		_expected:   1,
		_shouldPass: true,

		roundDir: single,
	})

	// two-void-rounds-one-handoff
	mixedParent := t.TempDir()
	siblingDirs := []string{"round-1", "round-2", "round-3", "lanes"}
	for _, name := range siblingDirs {
		require.NoError(t, os.MkdirAll(filepath.Join(mixedParent, name), 0o755))
	}
	require.NoError(t, os.WriteFile(filepath.Join(mixedParent, "round-2", reviewFile), []byte("{}"), 0o644))
	tests = append(tests, &testCase{
		_id:         "two-void-rounds-one-handoff",
		_expected:   2,
		_shouldPass: true,

		roundDir: filepath.Join(mixedParent, "round-3"),
	})

	// no-rounds-dir-is-error
	tests = append(tests, &testCase{
		_id:         "no-rounds-dir-is-error",
		_expected:   0,
		_shouldPass: false,

		roundDir: filepath.Join(t.TempDir(), "absent", "round-1"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			count, err := VoidRounds(test.roundDir)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, count)
		})
	}
}

package reviewscope

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

const (
	knownFile      = "known.json"
	reviewFile     = "review.json"
	roundDirPrefix = "round-"
	routeAutoFix   = "auto-fix"
)

const (
	FixStateOpen    FixState = "open"
	FixStateApplied FixState = "applied"
	FixStateFixed   FixState = "fixed"
	FixStateIgnored FixState = "ignored"
)

// Clean reviews after which a direction × file is done.
const (
	doneAfterCodeStyle = 1
	doneAfterTests     = 2
	doneAfterDefault   = 3
)

// Severity ranks shared by autofix levels and finding severities.
const (
	rankNone = iota
	rankInfo
	rankNit
	rankMinor
	rankMajor
	rankBlocker
)

var (
	defaultDoneAfter = map[string]int{
		directionCodeStyle:     doneAfterCodeStyle,
		directionContracts:     doneAfterDefault,
		directionCorrectness:   doneAfterDefault,
		directionCritical:      doneAfterDefault,
		directionDataIntegrity: doneAfterDefault,
		directionSecurity:      doneAfterDefault,
		directionSpecialFocus:  doneAfterDefault,
		directionTests:         doneAfterTests,
	}
	levelRanks    = map[string]int{"blocker": rankBlocker, "major": rankMajor, "minor": rankMinor, "nit": rankNit, "info": rankInfo, "none": rankNone}
	severityRanks = map[string]int{"BLOCKER": rankBlocker, "MAJOR": rankMajor, "MINOR": rankMinor, "NIT": rankNit, "INFO": rankInfo}
)

// fixStateMoves never targets open; a later probe-replay hit demotes a fixed row.
var fixStateMoves = map[FixState][]FixState{
	FixStateApplied: {FixStateOpen, FixStateFixed},
	FixStateFixed:   {FixStateApplied},
}

// CoverageLedger is coverage.json: per direction × file, its clean-review count.
type CoverageLedger map[string]map[string]*CoverageRow

func (l CoverageLedger) DoneFiles() map[string][]string {
	done := map[string][]string{}
	for direction, rows := range l {
		for file, row := range rows {
			if row.Done {
				done[direction] = append(done[direction], file)
			}
		}
	}
	return done
}

// rowFor returns the ledger row of one direction × file, created empty.
func (l CoverageLedger) rowFor(direction, file string) *CoverageRow {
	if l[direction] == nil {
		l[direction] = map[string]*CoverageRow{}
	}

	row := l[direction][file]
	if row == nil {
		row = &CoverageRow{}
		l[direction][file] = row
	}
	return row
}

// CoverageRow is one direction × file row of coverage.json.
type CoverageRow struct {
	CleanReviews int  `json:"clean_reviews"`
	Done         bool `json:"done"`
	LastRound    int  `json:"last_round"`
}

type FixState string

// FixStateRequest names the rows of one round that move to State.
type FixStateRequest struct {
	Ids       []string
	KnownPath string
	Round     int
	State     FixState
}

// KnownEntry is one known.json row — every confirmed finding, applied or not.
type KnownEntry struct {
	Claim     string   `json:"claim"`
	Effect    string   `json:"effect"`
	Exposure  string   `json:"exposure"`
	File      string   `json:"file"`
	FindingId string   `json:"finding_id"`
	Fix       string   `json:"fix"`
	FixState  FixState `json:"fix_state"`
	Line      int      `json:"line"`
	Round     int      `json:"round"`
	Route     string   `json:"route"`
	Severity  string   `json:"severity"`
}

// LedgerRequest requires SchemaPath: review.json is validated before anything is written; an empty IgnoreLevel reads none.
type LedgerRequest struct {
	AutofixLevel string
	CoveragePath string
	DoneAfter    int // 0 = per-direction defaults
	IgnoreLevel  string
	KnownPath    string
	PlanDir      string
	RoundDir     string
	SchemaPath   string
}

type LedgerResult struct {
	Autofix      []string `json:"autofix"`
	CoverageGaps []string `json:"coverageGaps"`
	Ignored      []string `json:"ignored"`
	Oscillating  []string `json:"oscillating"`
	Record       []string `json:"record"`
}

// laneState is the completion flag of a lane artifact.
type laneState struct {
	WalkComplete bool `json:"walk_complete"`
}

// rankedEntry is one ranked_plan entry of review.json.
type rankedEntry struct {
	Claim       string `json:"claim"`
	Effect      string `json:"effect"`
	Exposure    string `json:"exposure"`
	File        string `json:"file"`
	FindingId   string `json:"finding_id"`
	Fix         string `json:"fix"`
	Known       bool   `json:"known"`
	KnownId     string `json:"known_id"`
	Line        int    `json:"line"`
	Route       string `json:"route"`
	RouteReason string `json:"route_reason"`
	Severity    string `json:"severity"`
}

// reviewDocument is the slice of review.json the ledger and replay read.
type reviewDocument struct {
	Probes     []reviewProbe `json:"probes"`
	RankedPlan []rankedEntry `json:"ranked_plan"`
}

// creditClean leaves resetting a dirty file to the caller.
func creditClean(doneAfter int, isDirty bool, round int, row *CoverageRow) {
	if isDirty || row.LastRound == round {
		return
	}

	row.CleanReviews++
	row.LastRound = round
	row.Done = row.CleanReviews >= doneAfter
}

func doneAfterFor(direction string, uniform int) int {
	if uniform > 0 {
		return uniform
	}
	return defaultDoneAfter[direction]
}

func isAutofix(entry rankedEntry, rank int) bool {
	isInLevel := severityRanks[entry.Severity] <= rank
	return isInLevel && entry.Route == routeAutoFix
}

// isIgnored holds for a severity at or below the ignore level; level none (rank 0) ignores nothing.
func isIgnored(entry rankedEntry, ignoreRank int) bool {
	return severityRanks[entry.Severity] <= ignoreRank
}

// knownIndex finds the row of one round's finding, -1 when absent.
func knownIndex(known []KnownEntry, findingId string, round int) int {
	for index := range known {
		isRow := known[index].Round == round && known[index].FindingId == findingId
		if isRow {
			return index
		}
	}
	return -1
}

func levelRankOf(level string) (int, bool) {
	if level == "" {
		return rankNone, true
	}
	rank, isKnown := levelRanks[level]
	return rank, isKnown
}

func readJson(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("readJson: %w", err)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("readJson: %s: %w", path, err)
	}

	return nil
}

// RoundNumber is the round of a `round-<n>` dir.
func RoundNumber(roundDir string) (int, error) {
	number, err := strconv.Atoi(strings.TrimPrefix(filepath.Base(filepath.Clean(roundDir)), roundDirPrefix))
	if err != nil {
		return 0, fmt.Errorf("RoundNumber: Round dir %s is not round-<n>: %w", roundDir, err)
	}
	return number, nil
}

func updateCoverage(dirty map[string]bool, ledger CoverageLedger, request *LedgerRequest, round int) ([]string, error) {
	manifests, err := filepath.Glob(filepath.Join(request.RoundDir, "*"+LaneManifestSuffix))
	if err != nil {
		return nil, fmt.Errorf("updateCoverage: %w", err)
	}

	covered, unfinished := map[string]bool{}, map[string]bool{}
	for _, path := range manifests {
		var manifest LaneManifest
		if err := readJson(path, &manifest); err != nil {
			return nil, err
		}

		var state laneState
		isFinished := readJson(manifest.Artifact, &state) == nil && state.WalkComplete
		for _, file := range manifest.Files {
			if slices.Contains(manifest.Partners, file) {
				continue
			}

			key := manifest.Direction + "\x00" + file
			if !isFinished {
				unfinished[key] = true
				continue
			}

			covered[key] = true
			doneAfter := doneAfterFor(manifest.Direction, request.DoneAfter)
			row := ledger.rowFor(manifest.Direction, file)
			creditClean(doneAfter, dirty[file], round, row)
		}
	}

	for _, rows := range ledger {
		for file, row := range rows {
			if dirty[file] {
				*row = CoverageRow{LastRound: round}
			}
		}
	}

	gaps := make([]string, 0)
	for key := range unfinished {
		file := key[strings.IndexByte(key, 0)+1:]
		if !covered[key] && !slices.Contains(gaps, file) {
			gaps = append(gaps, file)
		}
	}
	slices.Sort(gaps)
	return gaps, nil
}

func writeJson(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("writeJson: %w", err)
	}

	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("writeJson: %w", err)
	}

	return nil
}

// SetFixState is idempotent: a row already in the state is left as it is.
func SetFixState(request *FixStateRequest) error {
	sources, isTarget := fixStateMoves[request.State]
	if !isTarget {
		return fmt.Errorf("SetFixState: State %s is not a target state", request.State)
	}

	var known []KnownEntry
	if err := readJson(request.KnownPath, &known); err != nil {
		return err
	}

	for _, findingId := range request.Ids {
		index := knownIndex(known, findingId, request.Round)
		if index < 0 {
			return fmt.Errorf("SetFixState: Round %d holds no row %s", request.Round, findingId)
		}

		current := known[index].FixState
		if current == request.State {
			continue
		}

		if !slices.Contains(sources, current) {
			return fmt.Errorf("SetFixState: Row %s cannot move from %s to %s", findingId, current, request.State)
		}

		known[index].FixState = request.State
	}
	return writeJson(request.KnownPath, known)
}

// UpdateLedger puts known carry-overs in neither set; human-routed entries always record.
func UpdateLedger(request *LedgerRequest) (*LedgerResult, error) {
	rank, isKnownLevel := levelRanks[request.AutofixLevel]
	if !isKnownLevel {
		return nil, fmt.Errorf("UpdateLedger: Unknown autofix level %s", request.AutofixLevel)
	}

	ignoreRank, isKnownIgnore := levelRankOf(request.IgnoreLevel)
	if !isKnownIgnore {
		return nil, fmt.Errorf("UpdateLedger: Unknown ignore level %s", request.IgnoreLevel)
	}

	if request.SchemaPath == "" {
		return nil, errors.New("UpdateLedger: Missing field SchemaPath")
	}

	round, err := RoundNumber(request.RoundDir)
	if err != nil {
		return nil, err
	}

	reviewPath := filepath.Join(request.RoundDir, reviewFile)
	if err := validateReview(reviewPath, request.SchemaPath); err != nil {
		return nil, err
	}

	var review reviewDocument
	if err := readJson(reviewPath, &review); err != nil {
		return nil, err
	}

	known := []KnownEntry{}
	if _, err := os.Stat(request.KnownPath); err == nil {
		if err := readJson(request.KnownPath, &known); err != nil {
			return nil, err
		}
	}

	result := &LedgerResult{Autofix: []string{}, CoverageGaps: []string{}, Ignored: []string{}, Oscillating: []string{}, Record: []string{}}
	dirty := map[string]bool{}
	for _, entry := range review.RankedPlan {
		dirty[entry.File] = true
		if entry.Known {
			if isReconfirmed(known, entry) {
				result.Oscillating = append(result.Oscillating, entry.FindingId)
			}
			continue
		}

		fixState := FixStateOpen
		switch {
		case isIgnored(entry, ignoreRank):
			fixState = FixStateIgnored
			result.Ignored = append(result.Ignored, entry.FindingId)
		case isAutofix(entry, rank):
			result.Autofix = append(result.Autofix, entry.FindingId)
		default:
			result.Record = append(result.Record, entry.FindingId)
		}
		row := KnownEntry{
			Claim:     entry.Claim,
			Effect:    entry.Effect,
			Exposure:  entry.Exposure,
			File:      entry.File,
			FindingId: entry.FindingId,
			Fix:       entry.Fix,
			FixState:  fixState,
			Line:      entry.Line,
			Round:     round,
			Route:     entry.Route,
			Severity:  entry.Severity,
		}
		known = append(known, row)
	}

	if err := writeJson(request.KnownPath, known); err != nil {
		return nil, err
	}

	ledger := CoverageLedger{}
	if _, err := os.Stat(request.CoveragePath); err == nil {
		if err := readJson(request.CoveragePath, &ledger); err != nil {
			return nil, err
		}
	}

	result.CoverageGaps, err = updateCoverage(dirty, ledger, request, round)
	if err != nil {
		return nil, err
	}

	if err := writeJson(request.CoveragePath, ledger); err != nil {
		return nil, err
	}

	if request.PlanDir != "" {
		if err := copyHandoff(request.RoundDir, request.PlanDir, round); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// VoidRound refuses any review.json, even a dangling link.
func VoidRound(roundDir string) error {
	if _, err := RoundNumber(roundDir); err != nil {
		return err
	}

	info, err := os.Stat(roundDir)
	if err != nil {
		return fmt.Errorf("VoidRound: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("VoidRound: Round dir %s is not a directory", roundDir)
	}

	reviewPath := filepath.Join(roundDir, reviewFile)
	_, err = os.Lstat(reviewPath)
	if err == nil {
		return fmt.Errorf("VoidRound: %s exists — the round has a handoff", reviewPath)
	}

	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("VoidRound: %w", err)
	}
	return nil
}

// VoidRounds counts the round dirs beside roundDir, itself included, that hold no review.json — the rounds that ended without a handoff.
func VoidRounds(roundDir string) (int, error) {
	parent := filepath.Dir(roundDir)
	entries, err := os.ReadDir(parent)
	if err != nil {
		return 0, fmt.Errorf("VoidRounds: %w", err)
	}

	count := 0
	for _, entry := range entries {
		isRound := entry.IsDir() && strings.HasPrefix(entry.Name(), roundDirPrefix)
		if !isRound {
			continue
		}

		_, err := os.Lstat(filepath.Join(parent, entry.Name(), reviewFile))
		if errors.Is(err, os.ErrNotExist) {
			count++
		}
	}
	return count, nil
}

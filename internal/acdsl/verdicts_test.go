package acdsl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildVerdictRecordCleanRun(t *testing.T) {
	rules := []Rule{
		{Id: "R-A", Projected: true},
		{Id: "R-B", Projected: false},
	}
	record := BuildVerdictRecord("2026-08-07T00:00:00Z", "/repo", "claude/x", "sess-1", rules, nil)
	assert.Equal(t, "clean", record.Outcome)
	assert.Equal(t, "claude/x", record.Branch)
	assert.Equal(t, "sess-1", record.Session)
	require.Len(t, record.Rules, 2)
	assert.Equal(t, RuleVerdict{Id: "R-A", Projected: true, Violations: 0}, record.Rules[0])
	assert.Equal(t, RuleVerdict{Id: "R-B", Projected: false, Violations: 0}, record.Rules[1])
	assert.Empty(t, record.Diagnostics)
}

func TestBuildVerdictRecordRedRun(t *testing.T) {
	rules := []Rule{{Id: "R-A", Projected: true}, {Id: "R-B", Projected: false}}
	diagnostics := []Diagnostic{
		{RuleId: "R-B", Message: "b.go:1: bad"},
		{RuleId: "R-B", Message: "b.go:9: bad again"},
	}
	record := BuildVerdictRecord("2026-08-07T00:00:00Z", "/repo", "main", "", rules, diagnostics)
	assert.Equal(t, "violations", record.Outcome)
	assert.Equal(t, 0, record.Rules[0].Violations)
	assert.Equal(t, 2, record.Rules[1].Violations)
	require.Len(t, record.Diagnostics, 2)
	assert.Equal(t, DiagnosticRef{Id: "R-B", Message: "b.go:1: bad"}, record.Diagnostics[0])
}

func TestAppendReadRoundTrip(t *testing.T) {
	sink := filepath.Join(t.TempDir(), "sub", "verdicts.jsonl")
	first := BuildVerdictRecord("2026-08-06T00:00:00Z", "/repo", "main", "", []Rule{{Id: "R", Projected: true}}, nil)
	second := BuildVerdictRecord("2026-08-07T00:00:00Z", "/repo", "main", "", []Rule{{Id: "R", Projected: false}}, nil)
	require.NoError(t, AppendVerdict(sink, first))
	require.NoError(t, AppendVerdict(sink, second))

	records, skipped, err := ReadVerdicts(sink, "")
	require.NoError(t, err)
	assert.Zero(t, skipped)
	require.Len(t, records, 2)
	assert.Equal(t, first, records[0])
	assert.Equal(t, second, records[1])
}

func TestReadVerdictsMissingFileIsEmpty(t *testing.T) {
	records, skipped, err := ReadVerdicts(filepath.Join(t.TempDir(), "absent.jsonl"), "")
	require.NoError(t, err)
	assert.Zero(t, skipped)
	assert.Empty(t, records)
}

func TestReadVerdictsSkipsGarbageLines(t *testing.T) {
	sink := filepath.Join(t.TempDir(), "verdicts.jsonl")
	record := BuildVerdictRecord("2026-08-07T00:00:00Z", "/repo", "main", "", []Rule{{Id: "R", Projected: true}}, nil)
	require.NoError(t, AppendVerdict(sink, record))
	garbage, err := os.OpenFile(sink, os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	_, err = garbage.WriteString("{torn line\n")
	require.NoError(t, err)
	require.NoError(t, garbage.Close())

	records, skipped, err := ReadVerdicts(sink, "")
	require.NoError(t, err)
	assert.Equal(t, 1, skipped)
	require.Len(t, records, 1)
}

func TestReadVerdictsSinceCutoff(t *testing.T) {
	sink := filepath.Join(t.TempDir(), "verdicts.jsonl")
	old := BuildVerdictRecord("2026-08-01T00:00:00Z", "/repo", "main", "", []Rule{{Id: "R", Projected: true}}, nil)
	fresh := BuildVerdictRecord("2026-08-07T00:00:00Z", "/repo", "main", "", []Rule{{Id: "R", Projected: true}}, nil)
	require.NoError(t, AppendVerdict(sink, old))
	require.NoError(t, AppendVerdict(sink, fresh))

	records, _, err := ReadVerdicts(sink, "2026-08-05T00:00:00Z")
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "2026-08-07T00:00:00Z", records[0].Ts)
}

func TestAggregateVerdictsFlipPairAndCounts(t *testing.T) {
	records := []VerdictRecord{
		{Ts: "2026-08-01T00:00:00Z", Rules: []RuleVerdict{{Id: "R", Projected: true, Violations: 0}}},
		{Ts: "2026-08-02T00:00:00Z", Rules: []RuleVerdict{{Id: "R", Projected: true, Violations: 2}}},
		{Ts: "2026-08-03T00:00:00Z", Rules: []RuleVerdict{{Id: "R", Projected: false, Violations: 1}}},
		{Ts: "2026-08-04T00:00:00Z", Rules: []RuleVerdict{{Id: "A", Projected: true, Violations: 0}}},
	}
	stats := AggregateVerdicts(records)
	require.Len(t, stats, 3)
	assert.Equal(t, RuleStat{Id: "A", Projected: true, Runs: 1}, stats[0])
	assert.Equal(t, RuleStat{Id: "R", Projected: true, Runs: 2, RedRuns: 1, Violations: 2, LastRed: "2026-08-02T00:00:00Z"}, stats[1])
	assert.Equal(t, RuleStat{Id: "R", Projected: false, Runs: 1, RedRuns: 1, Violations: 1, LastRed: "2026-08-03T00:00:00Z"}, stats[2])
}

package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const cannedReport = `github.com/kevinhorst/smine/internal/acdsl/rule.go:74:	parseMarker	85.0%
github.com/kevinhorst/smine/internal/acdsl/rule.go:130:	FilterLifetime	62.5%
github.com/kevinhorst/smine/internal/acdsl/load.go:16:	LoadRules	100.0%
total:	(statements)	81.3%
`

func TestJudgePassesAboveMin(t *testing.T) {
	var out bytes.Buffer
	code := judge(cannedReport, "parseMarker", 80, "internal/acdsl/rule.go", &out)
	assert.Equal(t, 0, code)
	assert.Empty(t, out.String())
}

func TestJudgeFlagsBelowMinWithCoverLocation(t *testing.T) {
	var out bytes.Buffer
	code := judge(cannedReport, "FilterLifetime", 80, "internal/acdsl/rule.go", &out)
	assert.Equal(t, 1, code)
	assert.Contains(t, out.String(), "rule.go:130: coverage of FilterLifetime is 62.5% < planned 80.0%")
}

func TestJudgeFlagsAbsentSymbol(t *testing.T) {
	var out bytes.Buffer
	code := judge(cannedReport, "Absent", 80, "internal/acdsl/rule.go", &out)
	assert.Equal(t, 1, code)
	assert.Contains(t, out.String(), `symbol "Absent" absent from coverage profile`)
}

func TestJudgeUsesWorstOfSeveralMatches(t *testing.T) {
	report := "a/x.go:1:\tPing\t90.0%\nb/y.go:2:\tPing\t40.0%\n"
	var out bytes.Buffer
	code := judge(report, "Ping", 80, "a/x.go", &out)
	assert.Equal(t, 1, code)
	assert.Contains(t, out.String(), "b/y.go:2: coverage of Ping is 40.0%")
}

func TestSingleDir(t *testing.T) {
	dir, err := singleDir([]string{"internal/a/x.go", "internal/a/y.go"})
	require.NoError(t, err)
	assert.Equal(t, "internal/a", dir)

	_, err = singleDir([]string{"internal/a/x.go", "internal/b/y.go"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "anchor one package")

	_, err = singleDir(nil)
	require.Error(t, err)
}

func TestRunMissingParamsAreErrors(t *testing.T) {
	var out bytes.Buffer
	assert.Equal(t, 2, run([]string{}, &out))
}

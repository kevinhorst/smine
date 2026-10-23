package main

import (
	"testing"

	"github.com/kevinhorst/smine/internal/evals"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func abRecord(measures map[string]float64) evals.AbRecord {
	return evals.AbRecord{Measures: measures}
}

func abSide(key string, records ...evals.AbRecord) evals.AbSide {
	return evals.AbSide{Key: key, Records: records}
}

func comparedMeasures(result evals.Stats) []string {
	var measures []string
	for _, comparison := range result.Comparisons {
		measures = append(measures, comparison.MetricId)
	}
	return measures
}

func TestComputeGroups(t *testing.T) {
	file := &evals.AbFile{
		Baseline: abSide(
			"base",
			abRecord(map[string]float64{"recall": 0.8, "cost": 1}),
			abRecord(map[string]float64{"recall": 0.6}),
		),
		Variant: abSide(
			"cand",
			abRecord(map[string]float64{"recall": 0.5, "cost": 2}),
			abRecord(map[string]float64{"cost": 3}),
		),
	}

	groups := computeGroups(file)

	expected := []Group{
		{Arm: "base", MetricId: "cost", Values: []float64{1}},
		{Arm: "base", MetricId: "recall", Values: []float64{0.8, 0.6}},
		{Arm: "cand", MetricId: "cost", Values: []float64{2, 3}},
		{Arm: "cand", MetricId: "recall", Values: []float64{0.5}},
	}
	assert.Equal(t, expected, groups, "baseline side first, measures sorted per record, values appended per side and measure")
}

func TestComputeComparisons(t *testing.T) {
	type testCase struct {
		_id           string
		_expectErr    bool
		_expectDelta  float64
		_expectSignif bool
		_expectNote   bool

		baseline string
		groups   []Group
	}

	tests := make([]*testCase, 0)

	// delta-vs-baseline-single-replica
	tests = append(tests, &testCase{
		_id:           "delta-vs-baseline-single-replica",
		_expectErr:    false,
		_expectDelta:  -0.2,
		_expectSignif: false,
		_expectNote:   true,

		baseline: "base",
		groups: []Group{
			{Arm: "base", MetricId: "recall", Values: []float64{0.8}},
			{Arm: "cand", MetricId: "recall", Values: []float64{0.6}},
		},
	})

	// missing-baseline-arm
	tests = append(tests, &testCase{
		_id:        "missing-baseline-arm",
		_expectErr: true,

		baseline: "nope",
		groups: []Group{
			{Arm: "base", MetricId: "recall", Values: []float64{0.8}},
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			comparisons, err := computeComparisons(test.groups, test.baseline)
			if test._expectErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, comparisons, 1)
			assert.InDelta(t, test._expectDelta, comparisons[0].DeltaMean, 1e-9)
			assert.Equal(t, test._expectSignif, comparisons[0].Significant)
			assert.Equal(t, test._expectNote, comparisons[0].Note != "")
		})
	}
}

func TestStatsFrom(t *testing.T) {
	type testCase struct {
		_id                       string
		_expectedBaseline         string
		_expectedComparedMeasures []string
		_shouldPass               bool

		file *evals.AbFile
	}

	tests := make([]*testCase, 0)

	// baseline-is-baseline-key
	tests = append(tests, &testCase{
		_id:                       "baseline-is-baseline-key",
		_expectedBaseline:         "base",
		_expectedComparedMeasures: []string{"recall"},
		_shouldPass:               true,

		file: &evals.AbFile{
			Baseline: abSide(
				"base",
				abRecord(map[string]float64{"recall": 0.8}),
				abRecord(map[string]float64{"recall": 0.7}),
			),
			Variant: abSide(
				"cand",
				abRecord(map[string]float64{"recall": 0.6}),
				abRecord(map[string]float64{"recall": 0.5}),
			),
		},
	})

	// variant-only-measure-no-comparison
	tests = append(tests, &testCase{
		_id:                       "variant-only-measure-no-comparison",
		_expectedBaseline:         "base",
		_expectedComparedMeasures: []string{"recall"},
		_shouldPass:               true,

		file: &evals.AbFile{
			Baseline: abSide(
				"base",
				abRecord(map[string]float64{"recall": 0.8}),
			),
			Variant: abSide(
				"cand",
				abRecord(map[string]float64{"recall": 0.6, "cost": 2}),
			),
		},
	})

	// baseline-without-values-error
	tests = append(tests, &testCase{
		_id:                       "baseline-without-values-error",
		_expectedBaseline:         "",
		_expectedComparedMeasures: nil,
		_shouldPass:               false,

		file: &evals.AbFile{
			Baseline: abSide("base"),
			Variant: abSide(
				"cand",
				abRecord(map[string]float64{"recall": 0.6}),
			),
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			result, err := statsFrom(test.file)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedBaseline, result.Baseline)
			assert.Equal(t, test._expectedComparedMeasures, comparedMeasures(result))
		})
	}
}

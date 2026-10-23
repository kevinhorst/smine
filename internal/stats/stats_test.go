package stats

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMean(t *testing.T) {
	type testCase struct {
		_id       string
		_expected float64

		values []float64
	}

	tests := make([]*testCase, 0)

	// empty
	tests = append(tests, &testCase{
		_id:       "empty",
		_expected: 0,

		values: nil,
	})

	// single
	tests = append(tests, &testCase{
		_id:       "single",
		_expected: 7,

		values: []float64{7},
	})

	// known-mean
	tests = append(tests, &testCase{
		_id:       "known-mean",
		_expected: 5,

		values: []float64{2, 4, 4, 4, 5, 5, 7, 9},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.InDelta(t, test._expected, Mean(test.values), 1e-9)
		})
	}
}

func TestVariance_StdDev(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedStdDev   float64
		_expectedVariance float64

		values []float64
	}

	tests := make([]*testCase, 0)

	// fewer-than-two
	tests = append(tests, &testCase{
		_id:               "fewer-than-two",
		_expectedStdDev:   0,
		_expectedVariance: 0,

		values: []float64{5},
	})

	// known-sample-variance
	tests = append(tests, &testCase{
		_id:               "known-sample-variance",
		_expectedStdDev:   math.Sqrt(32.0 / 7.0),
		_expectedVariance: 32.0 / 7.0,

		values: []float64{2, 4, 4, 4, 5, 5, 7, 9},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.InDelta(t, test._expectedVariance, Variance(test.values), 1e-9)
			assert.InDelta(t, test._expectedStdDev, StdDev(test.values), 1e-9)
		})
	}
}

func TestWelchT(t *testing.T) {
	type testCase struct {
		_id         string
		_expectedDf float64
		_expectedT  float64

		a []float64
		b []float64
	}

	tests := make([]*testCase, 0)

	// textbook
	tests = append(tests, &testCase{
		_id:         "textbook",
		_expectedDf: 8,
		_expectedT:  -2,

		a: []float64{1, 2, 3, 4, 5},
		b: []float64{3, 4, 5, 6, 7},
	})

	// too-few-samples
	tests = append(tests, &testCase{
		_id:         "too-few-samples",
		_expectedDf: 0,
		_expectedT:  0,

		a: []float64{1},
		b: []float64{3, 4, 5},
	})

	// both-zero-variance
	tests = append(tests, &testCase{
		_id:         "both-zero-variance",
		_expectedDf: 0,
		_expectedT:  0,

		a: []float64{4, 4, 4},
		b: []float64{4, 4, 4},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			gotT, gotDf := WelchT(test.a, test.b)
			assert.InDelta(t, test._expectedT, gotT, 1e-9)
			assert.InDelta(t, test._expectedDf, gotDf, 1e-9)
		})
	}
}

func TestCriticalT95_SignificantAt05(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedCrit   float64
		_expectedSignif bool

		df float64
		tv float64
	}

	tests := make([]*testCase, 0)

	// df-1-exact
	tests = append(tests, &testCase{
		_id:             "df-1-exact",
		_expectedCrit:   12.706,
		_expectedSignif: false,

		df: 1,
		tv: 5,
	})

	// df-25-interpolated
	tests = append(tests, &testCase{
		_id:             "df-25-interpolated",
		_expectedCrit:   2.064,
		_expectedSignif: true,

		df: 25,
		tv: 3,
	})

	// df-above-table
	tests = append(tests, &testCase{
		_id:             "df-above-table",
		_expectedCrit:   1.960,
		_expectedSignif: false,

		df: 500,
		tv: 1.5,
	})

	// df-below-one-never-significant
	tests = append(tests, &testCase{
		_id:             "df-below-one-never-significant",
		_expectedCrit:   12.706,
		_expectedSignif: false,

		df: 0.5,
		tv: 99,
	})

	// negative-t-uses-absolute
	tests = append(tests, &testCase{
		_id:             "negative-t-uses-absolute",
		_expectedCrit:   2.086,
		_expectedSignif: true,

		df: 20,
		tv: -3,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.InDelta(t, test._expectedCrit, CriticalT95(test.df), 1e-9)
			assert.Equal(t, test._expectedSignif, SignificantAt05(test.tv, test.df))
		})
	}
}

package skilleval

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBoldFailure(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		failed int
		total  int
	}

	tests := make([]*testCase, 0)

	// zero
	tests = append(tests, &testCase{
		_id:       "zero",
		_expected: "**0%** (0)",

		failed: 0,
		total:  3,
	})

	// third
	tests = append(tests, &testCase{
		_id:       "third",
		_expected: "**33%** (1)",

		failed: 1,
		total:  3,
	})

	// all
	tests = append(tests, &testCase{
		_id:       "all",
		_expected: "**100%** (3)",

		failed: 3,
		total:  3,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, boldFailure(test.failed, test.total))
		})
	}
}

func TestFormatDuration(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		seconds float64
	}

	tests := make([]*testCase, 0)

	// seconds
	tests = append(tests, &testCase{
		_id:       "seconds",
		_expected: "42s",

		seconds: 41.6,
	})

	// minutes
	tests = append(tests, &testCase{
		_id:       "minutes",
		_expected: "2m 05s",

		seconds: 125,
	})

	// hours
	tests = append(tests, &testCase{
		_id:       "hours",
		_expected: "1h 02m",

		seconds: 3725,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, formatDuration(test.seconds))
		})
	}
}

func TestFormatMetric(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		metricId string
		value    float64
	}

	tests := make([]*testCase, 0)

	// review-score-percent
	tests = append(tests, &testCase{
		_id:       "review-score-percent",
		_expected: "130%",

		metricId: metricReviewScore,
		value:    1.297,
	})

	// minutes-rounded
	tests = append(tests, &testCase{
		_id:       "minutes-rounded",
		_expected: "-490 min",

		metricId: metricLostMinutes,
		value:    -489.9,
	})

	// cost-dollars-unchanged
	tests = append(tests, &testCase{
		_id:       "cost-dollars-unchanged",
		_expected: "$199.35",

		metricId: metricCostUsd,
		value:    199.35,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, formatMetric(test.metricId, test.value))
		})
	}
}

func TestFormatTokens(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		value float64
	}

	tests := make([]*testCase, 0)

	// small
	tests = append(tests, &testCase{
		_id:       "small",
		_expected: "950",

		value: 950,
	})

	// thousands
	tests = append(tests, &testCase{
		_id:       "thousands",
		_expected: "12k",

		value: 12345,
	})

	// millions
	tests = append(tests, &testCase{
		_id:       "millions",
		_expected: "2.5M",

		value: 2_500_000,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, formatTokens(test.value))
		})
	}
}

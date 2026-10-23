package budget

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPresetPeriod(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedDays   int
		_expectedFrom   time.Time
		_expectedLength time.Duration
		_expectedTo     time.Time

		days int
		now  time.Time
	}

	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	anchor := time.Date(2000, 1, 1, 15, 30, 0, 0, berlin)
	tests := make([]*testCase, 0)

	// today
	tests = append(tests, &testCase{
		_id:             "today",
		_expectedDays:   1,
		_expectedFrom:   time.Date(2000, 1, 1, 0, 0, 0, 0, berlin),
		_expectedLength: 24 * time.Hour,
		_expectedTo:     time.Date(2000, 1, 2, 0, 0, 0, 0, berlin),

		days: 1,
		now:  anchor,
	})

	// 7-days
	tests = append(tests, &testCase{
		_id:             "7-days",
		_expectedDays:   7,
		_expectedFrom:   time.Date(1999, 12, 26, 0, 0, 0, 0, berlin),
		_expectedLength: 7 * 24 * time.Hour,
		_expectedTo:     time.Date(2000, 1, 2, 0, 0, 0, 0, berlin),

		days: 7,
		now:  anchor,
	})

	// dst-change-day
	tests = append(tests, &testCase{
		_id:             "dst-change-day",
		_expectedDays:   7,
		_expectedFrom:   time.Date(2000, 3, 22, 0, 0, 0, 0, berlin),
		_expectedLength: 7*24*time.Hour - time.Hour,
		_expectedTo:     time.Date(2000, 3, 29, 0, 0, 0, 0, berlin),

		days: 7,
		now:  time.Date(2000, 3, 28, 12, 0, 0, 0, berlin),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			period := NewPresetPeriod(test.days, test.now)
			assert.Equal(t, test._expectedFrom, period.From)
			assert.Equal(t, test._expectedTo, period.To)
			assert.Equal(t, test._expectedLength, period.To.Sub(period.From))
			assert.Len(t, period.Days(), test._expectedDays)
		})
	}
}

func TestNewRangePeriod(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   Period
		_shouldPass bool

		from string
		to   string
	}

	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	tests := make([]*testCase, 0)

	// range-inclusive
	tests = append(tests, &testCase{
		_id:         "range-inclusive",
		_expected:   Period{From: time.Date(2000, 1, 1, 0, 0, 0, 0, berlin), To: time.Date(2000, 1, 4, 0, 0, 0, 0, berlin)},
		_shouldPass: true,

		from: "2000-01-01",
		to:   "2000-01-03",
	})

	// single-day
	tests = append(tests, &testCase{
		_id:         "single-day",
		_expected:   Period{From: time.Date(2000, 1, 1, 0, 0, 0, 0, berlin), To: time.Date(2000, 1, 2, 0, 0, 0, 0, berlin)},
		_shouldPass: true,

		from: "2000-01-01",
		to:   "2000-01-01",
	})

	// to-before-from-error
	tests = append(tests, &testCase{
		_id:         "to-before-from-error",
		_expected:   Period{},
		_shouldPass: false,

		from: "2000-01-03",
		to:   "2000-01-01",
	})

	// malformed-date-error
	tests = append(tests, &testCase{
		_id:         "malformed-date-error",
		_expected:   Period{},
		_shouldPass: false,

		from: "2000-01-01",
		to:   "01.03.2000",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			period, err := NewRangePeriod(test.from, berlin, test.to)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, period)
		})
	}
}

func TestPeriod_ContainsDaysLastDay(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	period := NewPresetPeriod(3, time.Date(2000, 1, 1, 15, 30, 0, 0, berlin))

	assert.True(t, period.Contains(time.Date(1999, 12, 30, 0, 0, 0, 0, berlin)))
	assert.True(t, period.Contains(time.Date(2000, 1, 1, 23, 59, 59, 0, berlin)))
	assert.False(t, period.Contains(time.Date(2000, 1, 2, 0, 0, 0, 0, berlin)))
	assert.False(t, period.Contains(time.Date(1999, 12, 29, 23, 59, 59, 0, berlin)))
	assert.Equal(t, []time.Time{
		time.Date(1999, 12, 30, 0, 0, 0, 0, berlin),
		time.Date(1999, 12, 31, 0, 0, 0, 0, berlin),
		time.Date(2000, 1, 1, 0, 0, 0, 0, berlin),
	}, period.Days())
	assert.Equal(t, time.Date(2000, 1, 1, 0, 0, 0, 0, berlin), period.LastDay())
}

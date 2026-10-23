package budget

import (
	"errors"
	"fmt"
	"time"
)

// DefaultDays is the period the Usage page opens with.
const DefaultDays = 7

// PresetDays are the offered period lengths, today alone first.
var PresetDays = []int{1, 7, 30, 90}

// From is the first day's local midnight, To the midnight after the last day.
type Period struct {
	From time.Time
	To   time.Time
}

// NewPresetPeriod is the last days local days ending with now's day.
func NewPresetPeriod(days int, now time.Time) Period {
	today := startOfDay(now)
	period := Period{From: today.AddDate(0, 0, 1-days), To: today.AddDate(0, 0, 1)}
	return period
}

// NewRangePeriod reads two local dates, both inclusive.
func NewRangePeriod(fromDate string, location *time.Location, toDate string) (Period, error) {
	first, err := time.ParseInLocation(time.DateOnly, fromDate, location)
	if err != nil {
		return Period{}, fmt.Errorf("NewRangePeriod: Invalid from date %q: %w", fromDate, err)
	}
	last, err := time.ParseInLocation(time.DateOnly, toDate, location)
	if err != nil {
		return Period{}, fmt.Errorf("NewRangePeriod: Invalid to date %q: %w", toDate, err)
	}
	if last.Before(first) {
		return Period{}, errors.New("NewRangePeriod: The to date lies before the from date")
	}

	period := Period{From: first, To: last.AddDate(0, 0, 1)}
	return period, nil
}

func (p Period) Contains(moment time.Time) bool {
	return !moment.Before(p.From) && moment.Before(p.To)
}

// Days lists every day's midnight in order.
func (p Period) Days() []time.Time {
	var days []time.Time
	for day := p.From; day.Before(p.To); day = day.AddDate(0, 0, 1) {
		days = append(days, day)
	}
	return days
}

// LastDay is the last day's midnight.
func (p Period) LastDay() time.Time {
	return p.To.AddDate(0, 0, -1)
}

func startOfDay(moment time.Time) time.Time {
	return time.Date(moment.Year(), moment.Month(), moment.Day(), 0, 0, 0, 0, moment.Location())
}

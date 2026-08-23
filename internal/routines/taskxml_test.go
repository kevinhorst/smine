package routines

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func intPointer(value int) *int { return &value }

func TestTaskXMLDailyHourMinute(t *testing.T) {
	xml, err := TaskXML("demo", []CalendarInterval{{Hour: intPointer(6), Minute: intPointer(15)}},
		`C:\repo\bin\routinewrap.exe`, `C:\repo\routines\demo`)
	require.NoError(t, err)
	assert.Contains(t, xml, "<StartBoundary>2020-01-01T06:15:00</StartBoundary>")
	assert.Contains(t, xml, "<ScheduleByDay><DaysInterval>1</DaysInterval></ScheduleByDay>")
	assert.NotContains(t, xml, "<Repetition>")
	assert.Contains(t, xml, `<Command>C:\repo\bin\routinewrap.exe</Command>`)
	assert.Contains(t, xml, `<Arguments>C:\repo\routines\demo</Arguments>`)
}

func TestTaskXMLWeeklySundaySeven(t *testing.T) {
	xml, err := TaskXML("demo", []CalendarInterval{{Hour: intPointer(3), Weekday: intPointer(7)}}, "w", "d")
	require.NoError(t, err)
	assert.Contains(t, xml, "<DaysOfWeek><Sunday /></DaysOfWeek>")
	assert.Contains(t, xml, "<WeeksInterval>1</WeeksInterval>")
}

func TestTaskXMLMonthlyDayAndMonth(t *testing.T) {
	xml, err := TaskXML("demo", []CalendarInterval{{Day: intPointer(15), Month: intPointer(3), Hour: intPointer(4)}}, "w", "d")
	require.NoError(t, err)
	assert.Contains(t, xml, "<DaysOfMonth><Day>15</Day></DaysOfMonth>")
	assert.Contains(t, xml, "<Months><March /></Months>")
}

func TestTaskXMLMonthlyDayAllMonths(t *testing.T) {
	xml, err := TaskXML("demo", []CalendarInterval{{Day: intPointer(1)}}, "w", "d")
	require.NoError(t, err)
	assert.Contains(t, xml, "<January />")
	assert.Contains(t, xml, "<December />")
}

func TestTaskXMLMinuteOnlyHourlyRepetition(t *testing.T) {
	xml, err := TaskXML("demo", []CalendarInterval{{Minute: intPointer(30)}}, "w", "d")
	require.NoError(t, err)
	assert.Contains(t, xml, "<StartBoundary>2020-01-01T00:30:00</StartBoundary>")
	assert.Contains(t, xml, "<Interval>PT1H</Interval>")
	assert.Contains(t, xml, "<Duration>P1D</Duration>")
}

func TestTaskXMLEmptyIntervalMinuteRepetition(t *testing.T) {
	xml, err := TaskXML("demo", []CalendarInterval{{}}, "w", "d")
	require.NoError(t, err)
	assert.Contains(t, xml, "<Interval>PT1M</Interval>")
}

func TestTaskXMLMultipleIntervals(t *testing.T) {
	xml, err := TaskXML("demo", []CalendarInterval{
		{Hour: intPointer(6)},
		{Hour: intPointer(18), Weekday: intPointer(1)},
	}, "w", "d")
	require.NoError(t, err)
	assert.Equal(t, 2, strings.Count(xml, "<CalendarTrigger>"))
	assert.Contains(t, xml, "<Monday />")
}

func TestTaskXMLWeekdayPlusDayUnrepresentable(t *testing.T) {
	_, err := TaskXML("demo", []CalendarInterval{{Day: intPointer(1), Weekday: intPointer(1)}}, "w", "d")
	require.ErrorContains(t, err, "unrepresentable")
}

func TestTaskXMLMonthWithoutDayUnrepresentable(t *testing.T) {
	_, err := TaskXML("demo", []CalendarInterval{{Month: intPointer(2)}}, "w", "d")
	require.ErrorContains(t, err, "unrepresentable")
}

func TestTaskXMLSettingsBlock(t *testing.T) {
	xml, err := TaskXML("demo", nil, "w", "d")
	require.NoError(t, err)
	assert.Contains(t, xml, "<MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>")
	assert.Contains(t, xml, "<StartWhenAvailable>true</StartWhenAvailable>")
	assert.Contains(t, xml, "<ExecutionTimeLimit>PT0S</ExecutionTimeLimit>")
	assert.Contains(t, xml, "<DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>")
	assert.Contains(t, xml, "<WakeToRun>true</WakeToRun>")
	assert.Contains(t, xml, "<LogonType>InteractiveToken</LogonType>")
}

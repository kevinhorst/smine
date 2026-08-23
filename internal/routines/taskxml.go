package routines

import (
	"fmt"
	"strings"
)

// taskTemplate is the Task Scheduler definition shell: triggers hole, then
// the fixed concept settings (IgnoreNew, StartWhenAvailable, no execution
// time limit, run on batteries, WakeToRun, interactive token), then the
// routinewrap action. Kept untagged so trigger translation is testable on
// any platform (plan D15).
const taskTemplate = `<?xml version="1.0"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <Triggers>
%s  </Triggers>
  <Principals>
    <Principal id="Author">
      <LogonType>InteractiveToken</LogonType>
      <RunLevel>LeastPrivilege</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <StartWhenAvailable>true</StartWhenAvailable>
    <WakeToRun>true</WakeToRun>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Enabled>true</Enabled>
  </Settings>
  <Actions Context="Author">
    <Exec>
      <Command>%s</Command>
      <Arguments>%s</Arguments>
    </Exec>
  </Actions>
</Task>
`

var weekdayNames = map[int]string{
	0: "Sunday",
	1: "Monday",
	2: "Tuesday",
	3: "Wednesday",
	4: "Thursday",
	5: "Friday",
	6: "Saturday",
	7: "Sunday", // launchd: 0 and 7 are both Sunday
}

var monthNames = [13]string{"", "January", "February", "March", "April", "May", "June",
	"July", "August", "September", "October", "November", "December"}

// TaskXML renders the Task Scheduler definition for a routine: calendar
// triggers translated from the plist schedule, the fixed concept settings,
// and routinewrap.exe as the action. An unrepresentable schedule returns an
// error — the caller surfaces it as a degraded row. An empty schedule yields
// a trigger-less task: never fired by the calendar, still runnable on demand
// (matches a plist without StartCalendarInterval under launchd).
func TaskXML(label string, schedule []CalendarInterval, wrapperExe, routineDir string) (string, error) {
	var triggers strings.Builder
	for _, interval := range schedule {
		trigger, err := triggerXML(interval)
		if err != nil {
			return "", fmt.Errorf("TaskXML: %s: %w", label, err)
		}
		triggers.WriteString(trigger)
	}
	return fmt.Sprintf(taskTemplate, triggers.String(), xmlEscape(wrapperExe), xmlEscape(routineDir)), nil
}

// triggerXML maps one CalendarInterval to a trigger element:
//
//	Hour+Minute         -> ScheduleByDay, DaysInterval 1
//	+Weekday            -> ScheduleByWeek (launchd 0 and 7 both Sunday)
//	+Day (opt. +Month)  -> ScheduleByMonth, DaysOfMonth (opt. Months)
//	Minute only         -> ScheduleByDay + Repetition PT1H over the day
//	empty interval      -> ScheduleByDay + Repetition PT1M over the day
//
// Weekday and Day together, Month without Day, and out-of-range values are
// unrepresentable in one trigger and return an error.
func triggerXML(interval CalendarInterval) (string, error) {
	if interval.Weekday != nil && interval.Day != nil {
		return "", fmt.Errorf("triggerXML: Weekday and Day set together is unrepresentable")
	}
	if interval.Month != nil && interval.Day == nil {
		return "", fmt.Errorf("triggerXML: Month without Day is unrepresentable")
	}
	boundary := fmt.Sprintf("2020-01-01T%02d:%02d:00", intOrZero(interval.Hour), intOrZero(interval.Minute))

	switch {
	case interval.Day != nil:
		months := monthNames[1:]
		if interval.Month != nil {
			if *interval.Month < 1 || *interval.Month > 12 {
				return "", fmt.Errorf("triggerXML: Month %d out of range", *interval.Month)
			}
			months = monthNames[*interval.Month : *interval.Month+1]
		}
		var monthElements strings.Builder
		for _, month := range months {
			fmt.Fprintf(&monthElements, "<%s />", month)
		}
		return fmt.Sprintf(`    <CalendarTrigger>
      <StartBoundary>%s</StartBoundary>
      <ScheduleByMonth>
        <DaysOfMonth><Day>%d</Day></DaysOfMonth>
        <Months>%s</Months>
      </ScheduleByMonth>
    </CalendarTrigger>
`, boundary, *interval.Day, monthElements.String()), nil

	case interval.Weekday != nil:
		name, ok := weekdayNames[*interval.Weekday]
		if !ok {
			return "", fmt.Errorf("triggerXML: Weekday %d out of range", *interval.Weekday)
		}
		return fmt.Sprintf(`    <CalendarTrigger>
      <StartBoundary>%s</StartBoundary>
      <ScheduleByWeek>
        <DaysOfWeek><%s /></DaysOfWeek>
        <WeeksInterval>1</WeeksInterval>
      </ScheduleByWeek>
    </CalendarTrigger>
`, boundary, name), nil

	case interval.Hour != nil:
		return dailyTrigger(boundary, ""), nil

	case interval.Minute != nil:
		return dailyTrigger(boundary, "PT1H"), nil

	default:
		return dailyTrigger(boundary, "PT1M"), nil
	}
}

// dailyTrigger renders a ScheduleByDay trigger; a non-empty repetition
// interval adds a day-long repetition block (minute-only and empty-interval
// launchd schedules).
func dailyTrigger(boundary, repetition string) string {
	repetitionBlock := ""
	if repetition != "" {
		repetitionBlock = fmt.Sprintf(`      <Repetition>
        <Interval>%s</Interval>
        <Duration>P1D</Duration>
      </Repetition>
`, repetition)
	}
	return fmt.Sprintf(`    <CalendarTrigger>
      <StartBoundary>%s</StartBoundary>
%s      <ScheduleByDay><DaysInterval>1</DaysInterval></ScheduleByDay>
    </CalendarTrigger>
`, boundary, repetitionBlock)
}

func intOrZero(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

var xmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")

func xmlEscape(value string) string {
	return xmlEscaper.Replace(value)
}

// Package routines manages local launchd routines that wrap claude -p —
// discovery from the routines directory (the directory is the registry, D4),
// plist schedules, results.jsonl histories, and launchctl control.
package routines

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"howett.net/plist"
)

// historyLimit caps the run-history view (concept limit).
const historyLimit = 50

var namePattern = regexp.MustCompile(`^[a-z0-9-]+$`)

type CalendarInterval struct {
	Day     *int
	Hour    *int
	Minute  *int
	Month   *int
	Weekday *int // launchd semantics: 0 and 7 are both Sunday
}

func (i *CalendarInterval) matches(t time.Time) bool {
	if i.Day != nil && *i.Day != t.Day() {
		return false
	}
	if i.Hour != nil && *i.Hour != t.Hour() {
		return false
	}
	if i.Minute != nil && *i.Minute != t.Minute() {
		return false
	}
	if i.Month != nil && *i.Month != int(t.Month()) {
		return false
	}
	if i.Weekday != nil && *i.Weekday%7 != int(t.Weekday()) {
		return false
	}
	return true
}

// String renders the interval for schedule display; pointer fields print
// only when set.
func (i CalendarInterval) String() string {
	var parts []string
	if i.Month != nil {
		parts = append(parts, fmt.Sprintf("Month %d", *i.Month))
	}
	if i.Day != nil {
		parts = append(parts, fmt.Sprintf("Day %d", *i.Day))
	}
	if i.Weekday != nil {
		parts = append(parts, fmt.Sprintf("Weekday %d", *i.Weekday))
	}
	if i.Hour != nil {
		parts = append(parts, fmt.Sprintf("Hour %d", *i.Hour))
	}
	if i.Minute != nil {
		parts = append(parts, fmt.Sprintf("Minute %d", *i.Minute))
	}
	if len(parts) == 0 {
		return "every minute"
	}
	return strings.Join(parts, ", ")
}

func (i *CalendarInterval) toDict() map[string]int {
	dict := make(map[string]int)
	if i.Day != nil {
		dict["Day"] = *i.Day
	}
	if i.Hour != nil {
		dict["Hour"] = *i.Hour
	}
	if i.Minute != nil {
		dict["Minute"] = *i.Minute
	}
	if i.Month != nil {
		dict["Month"] = *i.Month
	}
	if i.Weekday != nil {
		dict["Weekday"] = *i.Weekday
	}
	return dict
}

type Routine struct {
	Dir               string
	Env               map[string]string
	Label             string
	LastRun           *RunResult // nil when never run
	LoadError         string     // non-empty = contract violation, degraded row (D29)
	Name              string
	NextRun           time.Time // zero when unsupported/unsatisfiable
	PlistPath         string
	ResultsPath       string
	Schedule          []CalendarInterval
	ScheduleSupported bool
	WrapperPath       string
}

type RunResult struct {
	ExitStatus   int     `json:"exit_status"`
	NumTurns     int     `json:"num_turns"`
	SessionId    string  `json:"session_id"`
	Timestamp    string  `json:"timestamp"`
	TotalCostUsd float64 `json:"total_cost_usd"`
}

// History returns up to limit results from results.jsonl, newest first.
// Malformed lines are skipped — a broken line never hides the readable rest.
func History(resultsPath string, limit int) ([]RunResult, error) {
	data, err := os.ReadFile(resultsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("History: Failed to read %s: %w", resultsPath, err)
	}

	var results []RunResult
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var result RunResult
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			continue
		}
		results = append(results, result)
	}

	// Newest first; the file appends chronologically.
	for left, right := 0, len(results)-1; left < right; left, right = left+1, right-1 {
		results[left], results[right] = results[right], results[left]
	}
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// HistoryLimit exposes the view cap for handlers.
func HistoryLimit() int {
	return historyLimit
}

// Scan discovers contract-conformant routines: one subdir per routine with
// run.sh + exactly one <label>.plist; underscore-prefixed dirs (templates)
// are skipped. Violations yield a degraded row, never an error (D29).
func Scan(dir string) ([]Routine, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("Scan: Failed to read %s: %w", dir, err)
	}

	var routines []Routine
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), "_") {
			continue
		}
		routines = append(routines, load(filepath.Join(dir, entry.Name()), entry.Name()))
	}
	sort.Slice(routines, func(i, j int) bool { return routines[i].Name < routines[j].Name })
	return routines, nil
}

func lastResult(resultsPath string) (*RunResult, error) {
	results, err := History(resultsPath, 1)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return &results[0], nil
}

func load(routineDir, name string) Routine {
	routine := Routine{Dir: routineDir, Name: name, ResultsPath: filepath.Join(routineDir, "results.jsonl")}

	wrapper := filepath.Join(routineDir, "run.sh")
	if _, err := os.Stat(wrapper); err != nil {
		routine.LoadError = "missing run.sh"
		return routine
	}
	routine.WrapperPath = wrapper

	plists, err := filepath.Glob(filepath.Join(routineDir, "*.plist"))
	if err != nil || len(plists) != 1 {
		routine.LoadError = fmt.Sprintf("expected exactly one .plist, found %d", len(plists))
		return routine
	}
	routine.PlistPath = plists[0]
	routine.Label = strings.TrimSuffix(filepath.Base(plists[0]), ".plist")

	schedule, supported, err := parseSchedule(plists[0])
	if err != nil {
		routine.LoadError = err.Error()
		return routine
	}
	routine.Schedule = schedule
	routine.ScheduleSupported = supported

	env, err := parseEnv(plists[0])
	if err != nil {
		routine.LoadError = err.Error()
		return routine
	}
	routine.Env = env
	if supported {
		if nextRun, ok := NextRun(schedule, time.Now()); ok {
			routine.NextRun = nextRun
		}
	}

	last, err := lastResult(routine.ResultsPath)
	if err != nil {
		routine.LoadError = err.Error()
		return routine
	}
	routine.LastRun = last
	return routine
}

func Duplicate(name, newName, routinesDir string) error {
	if !namePattern.MatchString(newName) {
		return fmt.Errorf("Duplicate: Invalid name %q", newName)
	}

	source := load(filepath.Join(routinesDir, name), name)
	if source.LoadError != "" {
		return fmt.Errorf("Duplicate: Source %s: %s", name, source.LoadError)
	}

	targetDir := filepath.Join(routinesDir, newName)
	if _, err := os.Stat(targetDir); err == nil {
		return fmt.Errorf("Duplicate: Routine %s already exists", newName)
	}

	newLabel := newName
	if prefix, ok := strings.CutSuffix(source.Label, "."+name); ok {
		newLabel = prefix + "." + newName
	}

	wrapper, err := os.ReadFile(source.WrapperPath)
	if err != nil {
		return fmt.Errorf("Duplicate: Failed to read %s: %w", source.WrapperPath, err)
	}

	data, err := os.ReadFile(source.PlistPath)
	if err != nil {
		return fmt.Errorf("Duplicate: Failed to read %s: %w", source.PlistPath, err)
	}
	var content map[string]any
	if _, err := plist.Unmarshal(data, &content); err != nil {
		return fmt.Errorf("Duplicate: Failed to parse %s: %w", source.PlistPath, err)
	}

	content["Label"] = newLabel
	pointsAtCopy := false
	if args, ok := content["ProgramArguments"].([]any); ok {
		for index, arg := range args {
			text, ok := arg.(string)
			if !ok {
				continue
			}
			replaced := strings.ReplaceAll(text, source.Dir, targetDir)
			if replaced != text {
				pointsAtCopy = true
			}
			args[index] = replaced
		}
	}
	if !pointsAtCopy {
		return fmt.Errorf("Duplicate: ProgramArguments of %s does not reference %s, the copy would run the source wrapper", source.PlistPath, source.Dir)
	}

	// The log paths carry the routine name, not its directory, so the directory
	// substitution above never reaches them — without this the copy would append
	// to the source routine's log files.
	for _, key := range []string{"StandardErrorPath", "StandardOutPath"} {
		text, ok := content[key].(string)
		if !ok {
			continue
		}
		base := filepath.Base(text)
		if !strings.Contains(base, name) {
			return fmt.Errorf("Duplicate: %s of %s does not contain %q, the copy would share the source log file", key, source.PlistPath, name)
		}
		content[key] = filepath.Join(filepath.Dir(text), strings.Replace(base, name, newName, 1))
	}

	rewritten, err := plist.MarshalIndent(content, plist.XMLFormat, "\t")
	if err != nil {
		return fmt.Errorf("Duplicate: Failed to encode plist: %w", err)
	}

	if err := os.Mkdir(targetDir, 0o755); err != nil {
		return fmt.Errorf("Duplicate: Failed to create %s: %w", targetDir, err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "run.sh"), wrapper, 0o755); err != nil {
		return fmt.Errorf("Duplicate: Failed to write run.sh: %w", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, newLabel+".plist"), rewritten, 0o644); err != nil {
		return fmt.Errorf("Duplicate: Failed to write plist: %w", err)
	}
	return nil
}

func matchesAny(schedule []CalendarInterval, candidate time.Time) bool {
	for _, interval := range schedule {
		if interval.matches(candidate) {
			return true
		}
	}
	return false
}

func NextRun(schedule []CalendarInterval, now time.Time) (time.Time, bool) {
	limit := now.AddDate(0, 0, 366)
	for candidate := now.Truncate(time.Minute).Add(time.Minute); candidate.Before(limit); candidate = candidate.Add(time.Minute) {
		if matchesAny(schedule, candidate) {
			return candidate, true
		}
	}
	return time.Time{}, false
}

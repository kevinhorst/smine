package routines

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"howett.net/plist"
)

// mapInterval expects uint64 integers, as howett/plist decodes them.
func mapInterval(dict map[string]any) (CalendarInterval, error) {
	interval := CalendarInterval{}
	for key, raw := range dict {
		number, ok := raw.(uint64)
		if !ok {
			return interval, fmt.Errorf("mapInterval: Key %s: Unexpected value type %T", key, raw)
		}

		value := int(number)
		switch key {
		case "Day":
			interval.Day = &value
		case "Hour":
			interval.Hour = &value
		case "Minute":
			interval.Minute = &value
		case "Month":
			interval.Month = &value
		case "Weekday":
			interval.Weekday = &value
		default:
			return interval, fmt.Errorf("mapInterval: Unknown key %s", key)
		}
	}
	return interval, nil
}

// mapIntervals accepts launchd's both shapes: a single dict or an array of dicts.
func mapIntervals(raw any) ([]CalendarInterval, error) {
	switch value := raw.(type) {
	case map[string]any:
		interval, err := mapInterval(value)
		if err != nil {
			return nil, err
		}
		return []CalendarInterval{interval}, nil
	case []any:
		return mapIntervalArray(value)
	}
	return nil, fmt.Errorf("mapIntervals: Unexpected StartCalendarInterval type %T", raw)
}

func mapIntervalArray(items []any) ([]CalendarInterval, error) {
	var intervals []CalendarInterval
	for _, item := range items {
		dict, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("mapIntervalArray: Unexpected interval type %T", item)
		}

		interval, err := mapInterval(dict)
		if err != nil {
			return nil, err
		}
		intervals = append(intervals, interval)
	}
	return intervals, nil
}

// otherTriggerKeys tell an unsupported schedule apart from a plist with no schedule set yet.
var otherTriggerKeys = []string{
	"KeepAlive",
	"QueueDirectories",
	"RunAtLoad",
	"StartInterval",
	"StartOnMount",
	"WatchPaths",
}

// runnerKnobPrefixes exclude skill inputs: they travel as prompt args, so a run equals the interactive invocation.
var runnerKnobPrefixes = []string{"ROUTINE_", "SMINE_"}

type scheduleInfo struct {
	Intervals   []CalendarInterval
	Supported   bool
	Unscheduled bool
}

func parseSchedule(plistPath string) (*scheduleInfo, error) {
	data, err := os.ReadFile(plistPath)
	if err != nil {
		return nil, fmt.Errorf("parseSchedule: Failed to read %s: %w", plistPath, err)
	}

	var content map[string]any
	if _, err := plist.Unmarshal(data, &content); err != nil {
		return nil, fmt.Errorf("parseSchedule: Failed to parse %s: %w", plistPath, err)
	}

	raw, ok := content["StartCalendarInterval"]
	if !ok {
		info := &scheduleInfo{Unscheduled: true}
		for _, key := range otherTriggerKeys {
			if _, present := content[key]; present {
				info.Unscheduled = false
				break
			}
		}
		return info, nil
	}

	intervals, err := mapIntervals(raw)
	if err != nil {
		return nil, fmt.Errorf("parseSchedule: %s: %w", plistPath, err)
	}

	info := &scheduleInfo{Intervals: intervals, Supported: true}
	return info, nil
}

func PlistMeta(dir string) (string, map[string]string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.plist"))
	if err != nil {
		return "", nil, fmt.Errorf("PlistMeta: Failed to glob %s: %w", dir, err)
	}
	if len(matches) != 1 {
		return "", nil, fmt.Errorf("PlistMeta: Expected exactly one plist in %s, found %d", dir, len(matches))
	}
	plistPath := matches[0]

	data, err := os.ReadFile(plistPath)
	if err != nil {
		return "", nil, fmt.Errorf("PlistMeta: Failed to read %s: %w", plistPath, err)
	}
	var content map[string]any
	if _, err := plist.Unmarshal(data, &content); err != nil {
		return "", nil, fmt.Errorf("PlistMeta: Failed to parse %s: %w", plistPath, err)
	}
	label, ok := content["Label"].(string)
	if !ok || label == "" {
		return "", nil, fmt.Errorf("PlistMeta: Missing Label in %s", plistPath)
	}

	env, err := parseEnv(plistPath)
	if err != nil {
		return "", nil, err
	}
	if err := ValidateEnvKeys(env); err != nil {
		return "", nil, fmt.Errorf("PlistMeta: %s: %w", plistPath, err)
	}
	return label, env, nil
}

func parseEnv(plistPath string) (map[string]string, error) {
	data, err := os.ReadFile(plistPath)
	if err != nil {
		return nil, fmt.Errorf("parseEnv: Failed to read %s: %w", plistPath, err)
	}

	var content map[string]any
	if _, err := plist.Unmarshal(data, &content); err != nil {
		return nil, fmt.Errorf("parseEnv: Failed to parse %s: %w", plistPath, err)
	}

	raw, ok := content["EnvironmentVariables"]
	if !ok {
		return nil, nil
	}
	dict, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("parseEnv: Unexpected EnvironmentVariables type %T", raw)
	}

	env := make(map[string]string, len(dict))
	for key, value := range dict {
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("parseEnv: Key %s: Unexpected value type %T", key, value)
		}
		env[key] = text
	}
	return env, nil
}

func SetEnv(env map[string]string, plistPath string) error {
	data, err := os.ReadFile(plistPath)
	if err != nil {
		return fmt.Errorf("SetEnv: Failed to read %s: %w", plistPath, err)
	}

	var content map[string]any
	if _, err := plist.Unmarshal(data, &content); err != nil {
		return fmt.Errorf("SetEnv: Failed to parse %s: %w", plistPath, err)
	}
	if len(env) == 0 {
		delete(content, "EnvironmentVariables")
	} else {
		content["EnvironmentVariables"] = env
	}

	if err := writePlistAtomic(content, plistPath); err != nil {
		return fmt.Errorf("SetEnv: %w", err)
	}
	return nil
}

// Reschedule round-trips every key other than StartCalendarInterval untouched.
func Reschedule(interval CalendarInterval, plistPath string) error {
	data, err := os.ReadFile(plistPath)
	if err != nil {
		return fmt.Errorf("Reschedule: Failed to read %s: %w", plistPath, err)
	}

	var content map[string]any
	if _, err := plist.Unmarshal(data, &content); err != nil {
		return fmt.Errorf("Reschedule: Failed to parse %s: %w", plistPath, err)
	}
	content["StartCalendarInterval"] = interval.toDict()

	if err := writePlistAtomic(content, plistPath); err != nil {
		return fmt.Errorf("Reschedule: %w", err)
	}
	return nil
}

// ValidateEnvKeys rejects plist env keys that are no runner knob.
func ValidateEnvKeys(env map[string]string) error {
	var foreign []string
	for key := range env {
		isKnob := slices.ContainsFunc(runnerKnobPrefixes, func(prefix string) bool { return strings.HasPrefix(key, prefix) })
		if !isKnob {
			foreign = append(foreign, key)
		}
	}
	if len(foreign) == 0 {
		return nil
	}

	sort.Strings(foreign)
	return fmt.Errorf("ValidateEnvKeys: Plist env keys %s are no runner knobs (ROUTINE_*, SMINE_*) — pass skill inputs as prompt args", strings.Join(foreign, ", "))
}

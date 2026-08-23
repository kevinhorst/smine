package routines

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kevinhorst/smine/internal/fsx"
	"howett.net/plist"
)

// mapInterval converts one StartCalendarInterval dict; howett/plist decodes
// integers as uint64 (F13). Unknown keys are a hard error — never guess a
// schedule.
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

// mapIntervals accepts launchd's both shapes: a single dict or an array of
// dicts.
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

// parseSchedule decodes the plist and maps StartCalendarInterval; a plist
// without the key is valid but unsupported for next-run and reschedule
// (D24, D26) — supported reports that distinction.
func parseSchedule(plistPath string) ([]CalendarInterval, bool, error) {
	data, err := os.ReadFile(plistPath)
	if err != nil {
		return nil, false, fmt.Errorf("parseSchedule: Failed to read %s: %w", plistPath, err)
	}

	var content map[string]any
	if _, err := plist.Unmarshal(data, &content); err != nil {
		return nil, false, fmt.Errorf("parseSchedule: Failed to parse %s: %w", plistPath, err)
	}

	raw, ok := content["StartCalendarInterval"]
	if !ok {
		return nil, false, nil
	}

	intervals, err := mapIntervals(raw)
	if err != nil {
		return nil, false, fmt.Errorf("parseSchedule: %s: %w", plistPath, err)
	}
	return intervals, true, nil
}

// PlistMeta reads the routine directory's single plist and returns its label
// and environment — the routinewrap launcher's view. Parsing reuses parseEnv
// so no second plist decoder exists.
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

	rewritten, err := plist.MarshalIndent(content, plist.XMLFormat, "\t")
	if err != nil {
		return fmt.Errorf("SetEnv: Failed to encode %s: %w", plistPath, err)
	}

	tmpPath := plistPath + ".tmp"
	if err := os.WriteFile(tmpPath, rewritten, 0o644); err != nil {
		return fmt.Errorf("SetEnv: Failed to write %s: %w", tmpPath, err)
	}
	if err := fsx.ReplaceFile(tmpPath, plistPath); err != nil {
		return fmt.Errorf("SetEnv: Failed to replace %s: %w", plistPath, err)
	}

	return nil
}

// Reschedule replaces exactly StartCalendarInterval with the single form
// interval and rewrites the plist atomically (tmp+rename, F19); every other
// key round-trips untouched (D22, H3).
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

	rewritten, err := plist.MarshalIndent(content, plist.XMLFormat, "\t")
	if err != nil {
		return fmt.Errorf("Reschedule: Failed to encode %s: %w", plistPath, err)
	}

	tmpPath := plistPath + ".tmp"
	if err := os.WriteFile(tmpPath, rewritten, 0o644); err != nil {
		return fmt.Errorf("Reschedule: Failed to write %s: %w", tmpPath, err)
	}
	if err := fsx.ReplaceFile(tmpPath, plistPath); err != nil {
		return fmt.Errorf("Reschedule: Failed to replace %s: %w", plistPath, err)
	}

	return nil
}

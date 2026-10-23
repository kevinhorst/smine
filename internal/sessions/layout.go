package sessions

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	daysPerWeek   = 7
	RecordJsonExt = ".json"
	UnknownScope  = "unknown"
)

var weekKeyPattern = regexp.MustCompile(`^\d{4}-W\d{2}$`)

// IsWeekKey reports whether key is an ISO week key like 2026-W39.
func IsWeekKey(key string) bool {
	return weekKeyPattern.MatchString(key)
}

// RecordFiles includes archived folders, so an archived scope's sessions never re-mine.
func RecordFiles(root string) (map[string]string, error) {
	files := make(map[string]string)
	walk := func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != RecordJsonExt {
			return nil
		}
		files[strings.TrimSuffix(entry.Name(), RecordJsonExt)] = path
		return nil
	}

	if err := filepath.WalkDir(root, walk); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("RecordFiles: %s: %w", root, err)
	}
	return files, nil
}

// RecordPath is <root>/<scope>/<week>/<id>.json.
func RecordPath(id, root, scope, week string) string {
	return filepath.Join(root, scope, week, id+RecordJsonExt)
}

func WeekKey(moment time.Time) string {
	year, week := moment.UTC().ISOWeek()
	return fmt.Sprintf("%d-W%02d", year, week)
}

// WeekStart relies on January 4th always lying in ISO week 1.
func WeekStart(key string) (time.Time, error) {
	var year, week int
	if _, err := fmt.Sscanf(key, "%d-W%d", &year, &week); err != nil || !IsWeekKey(key) {
		return time.Time{}, fmt.Errorf("WeekStart: Invalid week key %q", key)
	}

	january4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)
	daysSinceMonday := (int(january4.Weekday()) + daysPerWeek - 1) % daysPerWeek
	return january4.AddDate(0, 0, daysPerWeek*(week-1)-daysSinceMonday), nil
}

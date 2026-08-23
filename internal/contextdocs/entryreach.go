package contextdocs

import (
	"fmt"
	"os"
	"strings"

	"github.com/kevinhorst/smine/internal/reach"
)

// SetEntryReach rewrites the Reach bullet of one entry in its source
// markdown file: replacing an existing "* Reach: " bullet inside the entry's
// bullet region, else inserting one after the region's last bullet. The
// bullet region is what parseRulesFile attaches to the entry — the
// contiguous run of bullet and blank lines directly after the headline. The
// caller regenerates context.json (WriteContextFile).
func SetEntryReach(dir string, id string, value string) error {
	if !reach.Valid(value) {
		return fmt.Errorf("SetEntryReach: invalid reach %q", value)
	}
	set, err := ParseContext(dir, false)
	if err != nil {
		return fmt.Errorf("SetEntryReach: %w", err)
	}
	// Entry.Source already carries the dir-joined path the parser read.
	var path string
	for _, entry := range set.Entries {
		if entry.Id == id {
			path = entry.Source
			break
		}
	}
	if path == "" {
		return fmt.Errorf("SetEntryReach: unknown entry %s", id)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("SetEntryReach: %w", err)
	}
	lines := strings.Split(string(data), "\n")
	start := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "**"+id+"**") {
			start = i
			break
		}
	}
	if start == -1 {
		return fmt.Errorf("SetEntryReach: %s: entry %s not found", path, id)
	}

	lastBullet := -1
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], reachBulletPrefix) {
			lines[i] = reachBulletPrefix + value
			return writeEntryLines(path, lines)
		}
		if strings.HasPrefix(lines[i], "* ") {
			lastBullet = i
			continue
		}
		if strings.TrimSpace(lines[i]) != "" {
			break
		}
	}

	insertAt := lastBullet + 1
	if lastBullet == -1 {
		// Bullet-less entry: keep the blank separator between the headline
		// and its first bullet when one is already there.
		insertAt = start + 1
		if insertAt < len(lines) && strings.TrimSpace(lines[insertAt]) == "" {
			insertAt++
		}
	}
	lines = append(lines[:insertAt], append([]string{reachBulletPrefix + value}, lines[insertAt:]...)...)
	return writeEntryLines(path, lines)
}

func writeEntryLines(path string, lines []string) error {
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return fmt.Errorf("SetEntryReach: %w", err)
	}
	return nil
}

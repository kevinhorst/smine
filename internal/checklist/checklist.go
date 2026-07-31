package checklist

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type Entry struct {
	Body   string // raw markdown of the entry section (below the heading)
	Number int
	Status string
	Title  string
}

type Checklist struct {
	Entries []Entry
	Tags    []string // legend tags ∪ tags observed in headings (D11)
}

var (
	ErrEntryNotFound = errors.New("checklist: entry not found")
	ErrInvalidTag    = errors.New("checklist: invalid status tag")

	headingRe  = regexp.MustCompile("^## (\\d+)\\. (.*?) &nbsp; `\\[([A-Za-z]+)\\]`\\s*$")
	legendRe   = regexp.MustCompile("`\\[([A-Za-z]+)\\]`")
	overviewRe = regexp.MustCompile("^\\| (\\d+) \\| (.*\\|.*) `\\[([A-Za-z]+)\\]` \\|\\s*$")
	tagRe      = regexp.MustCompile("`\\[[A-Za-z]+\\]`")
)

func Parse(path string) (*Checklist, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Parse: Failed to read %s: %w", path, err)
	}
	return parse(data)
}

func parse(data []byte) (*Checklist, error) {
	cl := &Checklist{}
	lines := strings.Split(string(data), "\n")

	var current *Entry
	var body []string
	flush := func() {
		if current != nil {
			current.Body = strings.TrimSpace(strings.Join(body, "\n"))
			cl.Entries = append(cl.Entries, *current)
		}
		current, body = nil, nil
	}

	addTag := func(tag string) {
		if !slices.Contains(cl.Tags, tag) {
			cl.Tags = append(cl.Tags, tag)
		}
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "**Status tags:**") {
			for _, m := range legendRe.FindAllStringSubmatch(line, -1) {
				addTag(m[1])
			}
			continue
		}
		if m := headingRe.FindStringSubmatch(line); m != nil {
			flush()
			number, _ := strconv.Atoi(m[1])
			current = &Entry{Number: number, Status: m[3], Title: m[2]}
			addTag(m[3])
			continue
		}
		if current != nil {
			body = append(body, line)
		}
	}
	flush()
	return cl, nil
}

// SetStatus rewrites entry number's tag in exactly two lines — its section
// heading and its overview-table row — and saves atomically. The file is
// re-read here so concurrent hand edits are never clobbered from stale state.
func SetStatus(path string, number int, status string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("SetStatus: Failed to read %s: %w", path, err)
	}
	cl, err := parse(data)
	if err != nil {
		return err
	}

	// Reject tags outside legend ∪ observed
	if !slices.Contains(cl.Tags, status) {
		return fmt.Errorf("SetStatus: Tag %s: %w", status, ErrInvalidTag)
	}

	lines := strings.Split(string(data), "\n")
	headingIdx, overviewIdx := -1, -1
	headingPrefix := fmt.Sprintf("## %d. ", number)
	overviewPrefix := fmt.Sprintf("| %d | ", number)
	for i, line := range lines {
		if headingIdx < 0 && strings.HasPrefix(line, headingPrefix) && headingRe.MatchString(line) {
			headingIdx = i
		}
		if overviewIdx < 0 && strings.HasPrefix(line, overviewPrefix) && overviewRe.MatchString(line) {
			overviewIdx = i
		}
	}
	if headingIdx < 0 || overviewIdx < 0 {
		return fmt.Errorf("SetStatus: Entry %d: %w", number, ErrEntryNotFound)
	}

	// Replace only the backticked tag at the end of each located line
	lines[headingIdx] = replaceLastTag(lines[headingIdx], status)
	lines[overviewIdx] = replaceLastTag(lines[overviewIdx], status)

	// Atomic write, same discipline as config.Save
	out := []byte(strings.Join(lines, "\n"))
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0644); err != nil {
		return fmt.Errorf("SetStatus: Failed to write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("SetStatus: Failed to rename %s to %s: %w", tmp, path, err)
	}
	return nil
}

func replaceLastTag(line, status string) string {
	matches := tagRe.FindAllStringIndex(line, -1)
	last := matches[len(matches)-1]
	return line[:last[0]] + "`[" + status + "]`" + line[last[1]:]
}

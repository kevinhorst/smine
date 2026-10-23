// Package readledger is the per-agent coverage ledger the read gate and the skill-context hook share.
package readledger

import (
	"bufio"
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/kevinhorst/smine/internal/fsx"
)

const (
	// MainAgent names the root agent's ledger; a subagent uses its agent id.
	MainAgent = "main"
	// SourceClear and SourceCompact are the SessionStart sources after which reads have left the context.
	SourceClear   = "clear"
	SourceCompact = "compact"
	stateDirName  = "read-gate-state"
)

// Kind names what a ledger record witnesses.
type Kind string

const (
	KindContext Kind = "context"
	// KindGuide marks a guide owed on every call since a governed-file touch was denied for it.
	KindGuide Kind = "guide"
	KindRead  Kind = "read"
	KindSkill Kind = "skill"
)

// Ledger is one agent's ledger file.
type Ledger struct {
	dir       string
	path      string
	retention time.Duration
}

// New opens an agent's ledger; retention is how long an untouched ledger of any agent survives.
func New(agentId string, claudeHome string, retention time.Duration, sessionId string) *Ledger {
	dir := filepath.Join(claudeHome, stateDirName)
	name := sessionId + "__" + cmp.Or(agentId, MainAgent) + ".jsonl"
	return &Ledger{dir: dir, path: filepath.Join(dir, name), retention: retention}
}

func (l *Ledger) Append(record *Record) error {
	if err := os.MkdirAll(l.dir, 0o755); err != nil {
		return fmt.Errorf("Ledger.Append: Failed to create %s: %w", l.dir, err)
	}

	l.prune()
	line, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("Ledger.Append: Failed to encode record: %w", err)
	}

	file, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("Ledger.Append: Failed to open %s: %w", l.path, err)
	}

	defer file.Close()
	if _, err := file.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("Ledger.Append: Failed to write %s: %w", l.path, err)
	}
	return nil
}

func (l *Ledger) Load() ([]Record, error) {
	file, err := os.Open(l.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("Ledger.Load: Failed to open %s: %w", l.path, err)
	}

	defer file.Close()
	records := make([]Record, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if record, isValid := parseRecord(scanner.Bytes()); isValid {
			records = append(records, record)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("Ledger.Load: Failed to scan %s: %w", l.path, err)
	}
	return records, nil
}

// Path is the ledger file.
func (l *Ledger) Path() string {
	return l.path
}

// RecordContext takes an empty file and zero range for an inline injection, which a compaction loses.
func (l *Ledger) RecordContext(end int, file string, ids []string, start int) error {
	record := &Record{End: end, File: file, Ids: ids, Kind: KindContext, Start: start}
	return l.Append(record)
}

// RecordGuide appends a guide the agent owes from now on.
func (l *Ledger) RecordGuide(file string) error {
	record := &Record{File: file, Kind: KindGuide}
	return l.Append(record)
}

// RecordPrompt records a typed /<skill> prompt as the root's counterpart of a Skill call.
func (l *Ledger) RecordPrompt(prompt string) error {
	command, isSlash := strings.CutPrefix(prompt, "/")
	if !isSlash {
		return nil
	}

	skill := command
	if end := strings.IndexFunc(command, unicode.IsSpace); end >= 0 {
		skill = command[:end]
	}
	if skill == "" {
		return nil
	}
	return l.RecordSkill(prompt, skill)
}

func (l *Ledger) RecordRead(file string, numLines int, startLine int) error {
	if file == "" || numLines < 1 {
		return nil
	}

	record := &Record{End: startLine + numLines - 1, File: file, Kind: KindRead, Start: startLine}
	return l.Append(record)
}

// RecordSkill appends a skill invocation with its args.
func (l *Ledger) RecordSkill(args string, skill string) error {
	record := &Record{Args: args, Kind: KindSkill, Skill: skill}
	return l.Append(record)
}

// Reset keeps context records with an injection file: the gate owes that file again and its bodies come back.
func (l *Ledger) Reset(source string) error {
	if source != SourceCompact && source != SourceClear {
		return nil
	}

	records, err := l.Load()
	if err != nil {
		return fmt.Errorf("Ledger.Reset: %w", err)
	}

	if err := l.rewriteRecords(slices.DeleteFunc(records, isVolatile)); err != nil {
		return fmt.Errorf("Ledger.Reset: %w", err)
	}
	return nil
}

func (l *Ledger) prune() {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-l.retention)
	for _, entry := range entries {
		if isExpired(cutoff, entry) {
			_ = os.Remove(filepath.Join(l.dir, entry.Name()))
		}
	}
}

// removeFile deletes the ledger file; a missing one is already removed.
func (l *Ledger) removeFile() error {
	err := os.Remove(l.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("Ledger.removeFile: Failed to remove %s: %w", l.path, err)
	}
	return nil
}

// rewriteRecords replaces the ledger with the records; none removes it.
func (l *Ledger) rewriteRecords(records []Record) error {
	if len(records) == 0 {
		return l.removeFile()
	}

	lines := &bytes.Buffer{}
	for _, record := range records {
		line, err := json.Marshal(&record)
		if err != nil {
			return fmt.Errorf("Ledger.rewriteRecords: Failed to encode record: %w", err)
		}

		lines.Write(append(line, '\n'))
	}

	if err := fsx.WriteFileAtomic(lines.Bytes(), l.path, 0o644); err != nil {
		return fmt.Errorf("Ledger.rewriteRecords: Failed to write %s: %w", l.path, err)
	}
	return nil
}

// LineRange is an inclusive, 1-based line range.
type LineRange struct {
	End   int
	Start int
}

// Record is one ledger line.
type Record struct {
	Args  string   `json:"args,omitempty"`
	End   int      `json:"e,omitempty"`
	File  string   `json:"file,omitempty"`
	Ids   []string `json:"ids,omitempty"`
	Kind  Kind     `json:"kind"`
	Skill string   `json:"skill,omitempty"`
	Start int      `json:"s,omitempty"`
}

// credits also accepts a worktree or symlink copy: same last two components, byte-identical content.
func (r *Record) credits(content []byte, path string) bool {
	if r.Kind != KindRead {
		return false
	}
	if r.File == path {
		return true
	}
	if lastTwo(r.File) != lastTwo(path) {
		return false
	}

	other, err := os.ReadFile(r.File)
	return err == nil && bytes.Equal(other, content)
}

func (r *Record) isKnown() bool {
	switch r.Kind {
	case KindContext, KindGuide, KindRead, KindSkill:
		return true
	}
	return false
}

func clipRanges(ranges []LineRange, total int) []LineRange {
	clipped := make([]LineRange, 0, len(ranges))
	for _, lineRange := range ranges {
		if lineRange.Start <= total {
			clipped = append(clipped, LineRange{End: min(lineRange.End, total), Start: lineRange.Start})
		}
	}
	return clipped
}

func coveredRanges(content []byte, path string, records []Record) []LineRange {
	ranges := make([]LineRange, 0)
	for _, record := range records {
		if record.credits(content, path) {
			ranges = append(ranges, LineRange{End: record.End, Start: record.Start})
		}
	}

	slices.SortFunc(ranges, func(left, right LineRange) int { return left.Start - right.Start })
	return ranges
}

func isExpired(cutoff time.Time, entry os.DirEntry) bool {
	info, err := entry.Info()
	return err == nil && info.ModTime().Before(cutoff)
}

func isVolatile(record Record) bool {
	switch record.Kind {
	case KindGuide, KindRead:
		return true
	case KindContext:
		return record.File == ""
	case KindSkill:
		return false
	}
	return false
}

func lastTwo(path string) string {
	return filepath.Join(filepath.Base(filepath.Dir(path)), filepath.Base(path))
}

func parseRecord(line []byte) (Record, bool) {
	record := Record{}
	if err := json.Unmarshal(line, &record); err != nil {
		return record, false
	}
	return record, record.isKnown()
}

func PendingGuides(records []Record) []string {
	guides := make([]string, 0)
	for _, record := range records {
		if record.Kind == KindGuide {
			guides = append(guides, record.File)
		}
	}

	slices.Sort(guides)
	return slices.Compact(guides)
}

func SeenIds(records []Record) []string {
	ids := make([]string, 0)
	for _, record := range records {
		if record.Kind == KindContext {
			ids = append(ids, record.Ids...)
		}
	}

	slices.Sort(ids)
	return slices.Compact(ids)
}

// UnreadRanges counts lines by newlines, as wc -l does.
func UnreadRanges(path string, records []Record) []LineRange {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	gaps := make([]LineRange, 0)
	next := 1
	for _, covered := range coveredRanges(content, path, records) {
		if covered.Start > next {
			gaps = append(gaps, LineRange{End: covered.Start - 1, Start: next})
		}
		next = max(next, covered.End+1)
	}

	total := bytes.Count(content, []byte("\n"))
	tail := LineRange{End: total, Start: next}
	return clipRanges(append(gaps, tail), total)
}

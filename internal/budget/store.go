package budget

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/fsx"
)

const (
	archiveDirName    = "archive"
	breakdownsDirName = "breakdowns"
	jsonSuffix        = ".json"
	recordsDirName    = "records"
	settingsFileName  = "settings.json"
	stateFileName     = "state.json"
	storeDirName      = "usage"
	storeFileMode     = 0o600
)

// Anything outside the ids both agents mint never becomes a file name.
var sessionIdRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// Server-only; Plan is the subscription the page turns tokens into plan percent for.
type Settings struct {
	Plan Plan `json:"plan"`
}

// An error text names the source that failed.
type State struct {
	ArchiveError string    `json:"archive_error,omitempty"`
	Archived     int       `json:"archived"`
	CcusageError string    `json:"ccusage_error,omitempty"`
	ContextError string    `json:"context_error,omitempty"`
	FinishedAt   time.Time `json:"finished_at"`
	PeekError    string    `json:"peek_error,omitempty"`
	StartedAt    time.Time `json:"started_at"`
	Taken        int       `json:"taken"`
}

// Errors lists the failed sources' texts.
func (s *State) Errors() []string {
	var texts []string
	for _, text := range []string{s.ArchiveError, s.PeekError, s.ContextError, s.CcusageError} {
		if text != "" {
			texts = append(texts, text)
		}
	}
	return texts
}

func (s *State) Summary() string {
	summary := fmt.Sprintf("taken %d sessions, archived %d files", s.Taken, s.Archived)
	if failed := s.Errors(); len(failed) > 0 {
		summary += "; failed: " + strings.Join(failed, "; ")
	}
	return summary
}

type Store struct {
	dir string
}

func NewStore(stateDir string) *Store {
	return &Store{dir: filepath.Join(stateDir, storeDirName)}
}

func (s *Store) path(dirName, id string) (string, error) {
	if !sessionIdRe.MatchString(id) {
		return "", fmt.Errorf("Store.path: Invalid session id %q", id) //nolint:staticcheck // ST1005: the Receiver.method prefix is the repo's error-message convention
	}
	return filepath.Join(s.dir, dirName, id+jsonSuffix), nil
}

func (s *Store) ArchiveDir() string {
	return filepath.Join(s.dir, archiveDirName)
}

func (s *Store) Breakdown(id string) (*Breakdown, error) {
	path, err := s.path(breakdownsDirName, id)
	if err != nil {
		return nil, err
	}
	data, err := readStoreFile(path)
	if data == nil {
		return nil, err
	}

	var breakdown Breakdown
	if err := json.Unmarshal(data, &breakdown); err != nil {
		return nil, fmt.Errorf("Store.Breakdown: Failed to parse %s: %w", path, err)
	}
	if breakdown.Schema != RecordSchema {
		return nil, fmt.Errorf("Store.Breakdown: %s has schema %d, want %d", path, breakdown.Schema, RecordSchema)
	}
	return &breakdown, nil
}

func (s *Store) BreakdownModTime(id string) time.Time {
	path, err := s.path(breakdownsDirName, id)
	if err != nil {
		return time.Time{}
	}
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func (s *Store) Record(id string) (*Record, error) {
	path, err := s.path(recordsDirName, id)
	if err != nil {
		return nil, err
	}
	return readRecord(path)
}

// Records loads every record; the caller picks its period.
func (s *Store) Records() ([]*Record, error) {
	dir := filepath.Join(s.dir, recordsDirName)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("Store.Records: %w", err)
	}

	records := make([]*Record, 0, len(entries))
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), jsonSuffix) {
			continue
		}

		record, err := readRecord(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

func (s *Store) SaveBreakdown(breakdown *Breakdown) error {
	path, err := s.path(breakdownsDirName, breakdown.Id)
	if err != nil {
		return err
	}
	data, err := json.Marshal(breakdown)
	if err != nil {
		return fmt.Errorf("Store.SaveBreakdown: %w", err)
	}
	return writeStoreFile(data, path)
}

func (s *Store) SaveRecord(record *Record) error {
	path, err := s.path(recordsDirName, record.Id)
	if err != nil {
		return err
	}
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("Store.SaveRecord: %w", err)
	}
	return writeStoreFile(data, path)
}

func (s *Store) SaveSettings(settings *Settings) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("Store.SaveSettings: %w", err)
	}
	return writeStoreFile(data, filepath.Join(s.dir, settingsFileName))
}

func (s *Store) SaveState(state *State) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("Store.SaveState: %w", err)
	}
	return writeStoreFile(data, filepath.Join(s.dir, stateFileName))
}

// Settings returns the default plan when none or an unknown one was saved.
func (s *Store) Settings() (*Settings, error) {
	path := filepath.Join(s.dir, settingsFileName)
	settings := &Settings{Plan: DefaultPlan}
	data, err := readStoreFile(path)
	if data == nil {
		return settings, err
	}
	if err := json.Unmarshal(data, settings); err != nil {
		return nil, fmt.Errorf("Store.Settings: Failed to parse %s: %w", path, err)
	}
	if _, isKnown := NewPlan(string(settings.Plan)); !isKnown {
		settings.Plan = DefaultPlan
	}
	return settings, nil
}

// State returns nil when no capture has run yet.
func (s *Store) State() (*State, error) {
	path := filepath.Join(s.dir, stateFileName)
	data, err := readStoreFile(path)
	if data == nil {
		return nil, err
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("Store.State: Failed to parse %s: %w", path, err)
	}
	return &state, nil
}

func readRecord(path string) (*Record, error) {
	data, err := readStoreFile(path)
	if data == nil {
		return nil, err
	}

	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("readRecord: Failed to parse %s: %w", path, err)
	}
	if record.Schema != RecordSchema {
		return nil, fmt.Errorf("readRecord: %s has schema %d, want %d", path, record.Schema, RecordSchema)
	}
	return &record, nil
}

// readStoreFile returns nil data without an error for a missing file.
func readStoreFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("readStoreFile: %w", err)
	}
	return data, nil
}

func writeStoreFile(data []byte, path string) error {
	if err := fsx.WriteFileAtomic(append(data, '\n'), path, storeFileMode); err != nil {
		return fmt.Errorf("writeStoreFile: %w", err)
	}
	return nil
}

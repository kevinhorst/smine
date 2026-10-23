package sessionstate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/planrules"
)

// The record lives under the gate's protected ledger dir with the ledger's modes.
const (
	DirName  = "state"
	dirMode  = 0o700
	fileMode = 0o600
)

// SelfServiceState is what toolpolicy allow and revert keep per session.
type SelfServiceState struct {
	IsReverted   bool     `json:"is_reverted"`
	LastAppended []string `json:"last_appended"`
}

// SessionState is the gate's per-session record; the gate is its only writer.
type SessionState struct {
	SettingsLocal SelfServiceState `json:"settings_local"`
	Switches      Switches         `json:"switches"`
}

// SwitchAction is what one history event did to its feature.
type SwitchAction string

const (
	SwitchDisabled SwitchAction = "disabled"
	SwitchEnabled  SwitchAction = "enabled"
)

// SwitchEvent is one feature turned off or on, stamped at the gate's write.
type SwitchEvent struct {
	Action  SwitchAction `json:"action"`
	At      time.Time    `json:"at"`
	Feature Feature      `json:"feature"`
}

// Switches is the session's off set, sorted, and every change to it in write order.
type Switches struct {
	History []SwitchEvent `json:"history,omitempty"`
	Off     []Feature     `json:"off"`
}

// Set replaces the off set and appends one event per feature it turns off, then per feature it turns on.
func (s *Switches) Set(off []Feature, changedAt time.Time) {
	for _, feature := range off {
		if slices.Contains(s.Off, feature) {
			continue
		}

		disabled := SwitchEvent{Action: SwitchDisabled, At: changedAt, Feature: feature}
		s.History = append(s.History, disabled)
	}
	for _, feature := range s.Off {
		if slices.Contains(off, feature) {
			continue
		}

		enabled := SwitchEvent{Action: SwitchEnabled, At: changedAt, Feature: feature}
		s.History = append(s.History, enabled)
	}
	s.Off = off
}

// File is the record of one session under a state dir; empty when either part is missing.
func File(stateDir string, sessionId string) string {
	if stateDir == "" || sessionId == "" {
		return ""
	}
	return filepath.Join(stateDir, sessionId+".json")
}

// IsOff: a missing path or file is on, an unreadable record is on with its error.
func IsOff(path string, feature Feature) (bool, error) {
	if path == "" {
		return false, nil
	}

	state, err := Load(path)
	if err != nil {
		return false, fmt.Errorf("IsOff: %w", err)
	}
	return slices.Contains(state.Switches.Off, feature), nil
}

// Off is the record's off set; a missing path or file has none, an unreadable record none with its error.
func Off(path string) ([]Feature, error) {
	state, err := Load(path)
	if err != nil {
		return nil, fmt.Errorf("Off: %w", err)
	}
	return state.Switches.Off, nil
}

// Load reads the record; an empty path or a missing file is the zero record.
func Load(path string) (*SessionState, error) {
	state := &SessionState{}
	if path == "" {
		return state, nil
	}

	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return nil, fmt.Errorf("Load: Failed to read %s: %w", path, err)
	}

	if err := json.Unmarshal(raw, state); err != nil {
		return nil, fmt.Errorf("Load: Failed to decode %s: %w", path, err)
	}
	return state, nil
}

// Save writes the record atomically with the ledger's modes.
func Save(path string, state *SessionState) error {
	if path == "" {
		return errors.New("Save: No state path (ledger or session id missing)")
	}
	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		return fmt.Errorf("Save: Failed to create state dir: %w", err)
	}

	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("Save: Failed to encode state: %w", err)
	}

	if err := fsx.WriteFileAtomic(append(raw, '\n'), path, fileMode); err != nil {
		return fmt.Errorf("Save: %w", err)
	}
	return nil
}

// StateDir is the state dir of a Claude home: <home>/tool-policy/state.
func StateDir(claudeHome string) string {
	return filepath.Join(claudeHome, planrules.LedgerDirName, DirName)
}

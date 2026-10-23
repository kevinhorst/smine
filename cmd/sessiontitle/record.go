package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/sessiontitle"
)

// SessionRecord is the hook's whole state for one session; the state itself lives in the Desktop title.
type SessionRecord struct {
	Fleet string `json:"fleet,omitempty"`
	Named bool   `json:"named"`
	Turn  Turn   `json:"turn"`
}

// Turn holds the state the turn's last `smine-rules title set` call asked for and whether its first set renamed at the call; Stop spends it.
type Turn struct {
	IsDelivered bool               `json:"delivered,omitempty"`
	State       sessiontitle.State `json:"state,omitempty"`
}

// loadRecord returns nil for a session the hook never armed.
func loadRecord(path string) (*SessionRecord, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loadRecord: Failed to read %s: %w", path, err)
	}

	record := &SessionRecord{}
	if err := json.Unmarshal(data, record); err != nil {
		return nil, fmt.Errorf("loadRecord: Failed to decode %s: %w", path, err)
	}
	return record, nil
}

func saveRecord(path string, record *SessionRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("saveRecord: Failed to encode %s: %w", path, err)
	}
	if err := fsx.WriteFileAtomic(append(data, '\n'), path, 0o644); err != nil {
		return fmt.Errorf("saveRecord: %w", err)
	}
	return nil
}

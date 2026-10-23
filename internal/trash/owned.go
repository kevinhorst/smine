package trash

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/kevinhorst/smine/internal/fsx"
)

// ForgetOwned drops the record of path; a missing record is fine.
func (s *Store) ForgetOwned(path string) error {
	if s == nil {
		return nil
	}
	record, err := s.ownedPath(path)
	if err != nil {
		return fmt.Errorf("Store.ForgetOwned: %w", err)
	}
	if err := os.Remove(record); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("Store.ForgetOwned: %w", err)
	}
	return nil
}

// IsUnchanged reports whether path holds exactly the bytes smine last recorded for it; no record, a missing file or a nil store answer false.
func (s *Store) IsUnchanged(path string) (bool, error) {
	if s == nil {
		return false, nil
	}
	record, err := s.ownedPath(path)
	if err != nil {
		return false, fmt.Errorf("Store.IsUnchanged: %w", err)
	}

	recorded, err := os.ReadFile(record)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("Store.IsUnchanged: %w", err)
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("Store.IsUnchanged: %w", err)
	}
	return strings.TrimSpace(string(recorded)) == digest(data), nil
}

// RecordOwned stores the digest of the bytes just written to path.
func (s *Store) RecordOwned(path string, data []byte) error {
	if s == nil {
		return nil
	}
	record, err := s.ownedPath(path)
	if err != nil {
		return fmt.Errorf("Store.RecordOwned: %w", err)
	}
	if err := fsx.WriteFileAtomic([]byte(digest(data)+"\n"), record, 0o644); err != nil {
		return fmt.Errorf("Store.RecordOwned: %w", err)
	}
	return nil
}

func (s *Store) ownedPath(path string) (string, error) {
	rel, err := stashRel(path)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.owned, filepath.FromSlash(rel)+digestSuffix), nil
}

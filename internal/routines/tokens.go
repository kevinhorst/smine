package routines

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

// TokenDir returns the routine token store (~/.config/claude-routine/tokens):
// one file per token, filename = label. The legacy single-token file next to
// it stays the default for routines that set no ROUTINE_TOKEN.
func TokenDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("TokenDir: %w", err)
	}
	return filepath.Join(home, ".config", "claude-routine", "tokens"), nil
}

// ListTokenLabels lists stored token labels sorted; a missing dir is an
// empty store, not an error.
func ListTokenLabels(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ListTokenLabels: %w", err)
	}

	var labels []string
	for _, entry := range entries {
		if !entry.Type().IsRegular() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		labels = append(labels, entry.Name())
	}
	slices.Sort(labels)
	return labels, nil
}

// SaveToken stores value under label in dir (file 0600, dir created 0700).
// All whitespace is stripped from the value — pasted setup-token output
// embeds line-wrap newlines. Write-only by design: nothing reads token
// values back (D8). Never overwrites an existing label (D4).
func SaveToken(label, value, dir string) error {
	value = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, value)
	if value == "" {
		return errors.New("SaveToken: token value is empty")
	}
	if err := validateTokenLabel(label); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("SaveToken: %w", err)
	}

	file, err := os.OpenFile(filepath.Join(dir, label), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("SaveToken: token label %q already exists", label)
	}
	if err != nil {
		return fmt.Errorf("SaveToken: %w", err)
	}
	if _, err := file.WriteString(value); err != nil {
		file.Close()
		return fmt.Errorf("SaveToken: %w", err)
	}
	return file.Close()
}

// validateTokenLabel allows [A-Za-z0-9._-] up to 64 bytes, rejecting a
// leading '.' or '-' — labels are filenames; the allowlist blocks path
// separators and '..' inherently.
func validateTokenLabel(label string) error {
	if label == "" || len(label) > 64 {
		return errors.New("validateTokenLabel: label must be 1-64 characters")
	}
	if label[0] == '.' || label[0] == '-' {
		return errors.New("validateTokenLabel: label must not start with '.' or '-'")
	}
	for _, r := range label {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' {
			continue
		}
		return fmt.Errorf("validateTokenLabel: invalid character %q: allowed are letters, digits, '.', '_', '-'", r)
	}
	return nil
}

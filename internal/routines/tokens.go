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

// rawTokenPrefix starts every Anthropic credential; a label carrying it is a pasted secret.
const rawTokenPrefix = "sk-ant-"

func TokenDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("TokenDir: %w", err)
	}
	return filepath.Join(home, ".config", "claude-routine", "tokens"), nil
}

// LegacyTokenPath is the fallback when a routine sets no ROUTINE_TOKEN label.
func LegacyTokenPath(dir string) string {
	return filepath.Join(filepath.Dir(dir), "token")
}

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

// ReadToken reads the legacy single file for an empty label.
func ReadToken(label string) (string, error) {
	dir, err := TokenDir()
	if err != nil {
		return "", err
	}

	path := LegacyTokenPath(dir)
	if label != "" {
		if err := validateTokenLabel(label); err != nil {
			return "", err
		}
		path = filepath.Join(dir, label)
	}
	data, err := os.ReadFile(path)
	token := strings.TrimSpace(string(data))
	if err != nil || token == "" {
		return "", fmt.Errorf("ReadToken: token file missing or empty: %s", path)
	}
	return token, nil
}

// SaveToken strips all whitespace: pasted setup-token output embeds line-wrap newlines.
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

// validateTokenLabel's allowlist blocks path separators and '..', since labels are filenames.
func validateTokenLabel(label string) error {
	if label == "" || len(label) > 64 {
		return errors.New("validateTokenLabel: label must be 1-64 characters")
	}
	if strings.HasPrefix(label, rawTokenPrefix) {
		return errors.New("validateTokenLabel: that is a raw token, not a label — pass the label of a stored token")
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

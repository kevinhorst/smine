package fixture

import (
	"errors"
	"fmt"
	"os"
)

func good(path string) error {
	if path == "" {
		return errors.New("fixture.good: Missing path")
	}
	_, err := os.Stat(path)
	return fmt.Errorf("good: reading %s: %w", path, err)
}

func bad(path string) error {
	if path == "" {
		return errors.New("missing path")
	}
	_, err := os.Stat(path)
	return fmt.Errorf("failed to read %s: %w", path, err)
}

func skipped(msg string) error {
	return errors.New(msg)
}

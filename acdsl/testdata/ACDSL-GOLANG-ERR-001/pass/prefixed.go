package fixture

import (
	"errors"
	"fmt"
	"os"
)

func load(path string) error {
	if path == "" {
		return errors.New("fixture.load: Missing path")
	}
	_, err := os.Stat(path)
	return fmt.Errorf("load: reading %s: %w", path, err)
}

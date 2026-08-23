package fixture

import "fmt"

func load(path string) error {
	return fmt.Errorf("failed to read config %s", path)
}

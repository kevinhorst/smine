package skilleval

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/kevinhorst/smine/internal/skills"
)

// ErrDeployedDiffers maps to cmd/skilleval's exit 3.
var ErrDeployedDiffers = errors.New("deployed skill differs from the checkout")

// checkDeployed refuses a deployed copy whose marker names another source than the checkout's skill dir.
func checkDeployed(skillDir string) error {
	deployedDir := filepath.Join(skills.DefaultHomePath(), filepath.Base(skillDir))
	isCurrent, err := skills.IsDeployedCurrent(skillDir, deployedDir)
	if err != nil {
		return fmt.Errorf("checkDeployed: %w", err)
	}

	if !isCurrent {
		return fmt.Errorf("checkDeployed: %s: %w", deployedDir, ErrDeployedDiffers)
	}
	return nil
}

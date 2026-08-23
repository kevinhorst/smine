package fixture

import (
	"context"
	"os/exec"
)

func usesContext(ctx context.Context) {
	_ = exec.CommandContext(ctx, "ls")
}

package fixture

import (
	"context"
	"os/exec"
)

func useContext(ctx context.Context) {
	_ = exec.CommandContext(ctx, "ls")
}

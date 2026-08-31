package skills

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/kevinhorst/smine/internal/shell"
)

const syncScript = "sync_skills.sh"

// Sync runs sync_skills.sh: copy every repo skill to the claude and codex
// home roots — all inside the script. Without prune, skills removed from the
// repo are kept in the targets; with prune they are deleted.
func Sync(ctx context.Context, prune bool, scriptsDir string) (string, error) {
	var args []string
	if prune {
		args = append(args, "--prune")
	}

	script := filepath.Join(scriptsDir, syncScript)
	output, err := shell.RunSync(ctx, "", script, args...)
	if err != nil {
		return output, fmt.Errorf("Sync: %w", err)
	}
	return output, nil
}

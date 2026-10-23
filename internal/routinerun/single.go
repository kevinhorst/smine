package routinerun

import (
	"context"
	"fmt"
	"log"
	"path/filepath"

	"github.com/kevinhorst/smine/internal/routines"
)

const markerInstruction = " When the entire task is complete, write the empty file %s (Write tool) as your final act, then end your turn."

func (r *Runner) runSingle(ctx context.Context) int {
	worktreePath, status := r.createWorktree(ctx)
	if worktreePath == "" {
		return status
	}

	marker := filepath.Join(r.config.SteerDir, routines.RunCompleteFileName)
	cell := &stageCell{
		complete:    CompleteMarker,
		grantsWrite: true,
		markerPath:  marker,
		prompt:      r.composePrompt(r.manifest.Prompt) + fmt.Sprintf(markerInstruction, marker),
		skill:       r.manifest.Skill(),
		timeout:     r.config.RunTimeout,
	}
	_, status = r.runStage(ctx, worktreePath, cell)
	if ctx.Err() != nil {
		return status
	}

	r.recordItem()
	if err := r.worktree.publish(ctx, status, exitMarker(status)); err != nil {
		log.Printf("Runner.runSingle: publish failed; worktree kept: %s: %v", worktreePath, err)
	}
	r.commitStores(ctx)
	return status
}

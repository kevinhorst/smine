package routinerun

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	settings "github.com/kevinhorst/smine/internal/config"
)

// commitStores never fails the run: uncommitted changes ride on the next run's commit.
func (r *Runner) commitStores(ctx context.Context) {
	subject := fmt.Sprintf(
		"stores: Recorded %s store changes after the %s run (%s)",
		filepath.Base(r.storeRoot),
		r.name,
		time.Now().UTC().Format(time.DateOnly),
	)
	sha, err := commitStoreDirs(ctx, r.storeRoot, subject)
	switch {
	case err != nil:
		log.Printf("Runner.commitStores: %s: Store changes left uncommitted: %v", r.storeRoot, err)
	case sha != "":
		log.Printf("Runner.commitStores: %s: Committed %s", r.storeRoot, sha)
	}
}

func commitStoreDirs(ctx context.Context, storeRoot, subject string) (string, error) {
	pathspecs := presentStoreDirs(storeRoot)
	if len(pathspecs) == 0 {
		return "", fmt.Errorf("commitStoreDirs: No store dir under %s", storeRoot)
	}

	isCommitted, err := commitChanges(ctx, "", storeRoot, pathspecs, subject)
	if err != nil {
		return "", fmt.Errorf("commitStoreDirs: %w", err)
	}
	if !isCommitted {
		return "", nil
	}

	sha, err := git(ctx, storeRoot, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("commitStoreDirs: %w", err)
	}
	return sha, nil
}

// presentStoreDirs exists because git add refuses a pathspec naming an absent, untracked dir.
func presentStoreDirs(storeRoot string) []string {
	var dirs []string
	for _, dir := range []string{settings.StoreBacklogDir} {
		if _, err := os.Stat(filepath.Join(storeRoot, dir)); err == nil {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

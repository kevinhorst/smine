package routinerun

import (
	"context"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"slices"

	"github.com/kevinhorst/smine/internal/budget"
	"github.com/kevinhorst/smine/internal/components"
	settings "github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/peek"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/tokencount"
)

const (
	budgetCcusageBinary  = "ccusage"
	budgetContextDir     = "context"
	budgetTokenCacheDir  = "context-report"
	budgetSummarySubtype = "summary"
)

func (r *Runner) newBudgetCapture() (*budget.Capture, error) {
	claudeHome, err := settings.ClaudeHomeDir("")
	if err != nil {
		return nil, fmt.Errorf("Runner.newBudgetCapture: %w", err)
	}
	env, err := install.NewEnv(r.repoRoot, io.Discard)
	if err != nil {
		return nil, fmt.Errorf("Runner.newBudgetCapture: %w", err)
	}

	// The selection record decides which sides are taken; an unreadable record takes both, as Full.
	record, _, err := install.Load(env)
	if err != nil {
		log.Printf("Runner.newBudgetCapture: %v", err)
	}

	tokenOptions := &tokencount.Options{
		CacheDir: filepath.Join(claudeHome, budgetTokenCacheDir),
		Mode:     tokencount.Mode(env.Values.CountTokens),
	}
	counter, err := tokencount.New(tokenOptions)
	if err != nil {
		return nil, fmt.Errorf("Runner.newBudgetCapture: %w", err)
	}

	registry := repos.NewRegistry(filepath.Join(r.config.StateDir, settings.StateRegistryFile))
	if err := registry.Reload(); err != nil {
		log.Printf("Runner.newBudgetCapture: %v", err)
	}

	store := budget.NewStore(r.config.StateDir)
	archive := &budget.Archive{ClaudeHome: claudeHome, Dir: store.ArchiveDir()}
	capture := &budget.Capture{
		Archive:    archive,
		ClaudeHome: claudeHome,
		ContextDir: budgetContextDirName(record),
		Counter:    counter,
		IsCounting: env.Values.CountTokens == string(tokencount.ModeKeychain),
		Roster:     registry.Repos(),
		Store:      store,
	}

	if slices.Contains(record.Components, components.Peek) {
		capture.Peek = peek.NewClient(env.Values.PeekEndpoint())
	}
	if slices.Contains(record.Components, components.Ccusage) {
		capture.Ccusage = &budget.Ccusage{ArchiveDir: store.ArchiveDir(), BinaryPath: budgetCcusageBinary}
	}
	return capture, nil
}

func (r *Runner) runBudget(ctx context.Context) int {
	capture, err := r.newBudgetCapture()
	if err != nil {
		log.Printf("Runner.runBudget: %v", err)
		r.appendFailure(exitConfig, err.Error())
		return exitConfig
	}

	state := capture.Run(ctx)
	if len(state.Errors()) > 0 {
		r.appendFailure(exitSoftware, state.Summary())
		return exitSoftware
	}
	if state.Taken > 0 {
		appendResult(r.resultsPath, summaryRow(state.Summary()))
	}
	return exitOk
}

// budgetContextDirName is the base name of the record's context.context_dir, budgetContextDir when unset or unreadable.
func budgetContextDirName(record install.Record) string {
	configured, err := install.Resolve(record, components.Context, "context_dir")
	if err != nil {
		log.Printf("budgetContextDirName: %v", err)
		return budgetContextDir
	}
	if configured.Text == "" {
		return budgetContextDir
	}
	return filepath.Base(configured.Text)
}

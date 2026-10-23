package install

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/gitroot"
	"github.com/kevinhorst/smine/internal/trash"
)

// Step and Result actions.
const (
	ActionInstall         = "install"
	ActionRemove          = "remove"
	ActionRestartRequired = "restart-required"
	ActionUnchanged       = "unchanged"
	ActionUpdate          = "update"
)

// fixReapply is the fix hint of a component whose artifacts are missing.
const fixReapply = "run bin/configserver -apply from the checkout"

// stateDirName is the state dir under the Claude home: the record, the trash, the journal.
const stateDirName = "smine"

// Owner-only modes for files holding per-user secrets or state.
const (
	privateDirPerm  = 0o700
	privateFilePerm = 0o600
)

// Component is one catalog entry's installer, one implementation per components.Catalog entry.
type Component interface {
	ID() components.ID
	Plan(ctx context.Context, env Env, selected bool) ([]Step, error)
	Install(ctx context.Context, env Env) error
	InstallHooks(ctx context.Context, env Env) ([]Step, error)
	Remove(ctx context.Context, env Env) error
	Health(ctx context.Context, env Env) Health
}

// ComponentStatus is one row of Status.
type ComponentStatus struct {
	Component components.Component
	Selected  bool
	Available bool   // on this GOOS
	Health    Health // zero value when not selected
}

// Env is everything a step needs to know about the machine.
type Env struct {
	ClaudeHome string
	Home       string
	// IsPayload: RepoDir is an extracted release archive (bin/PAYLOAD present), every binary prebuilt under bin/.
	IsPayload bool
	// MainRepoDir is the main checkout RepoDir belongs to (RepoDir itself outside a worktree): what plists and the gate marker embed.
	MainRepoDir string
	RepoDir     string
	GOOS        string
	Trash       *trash.Store // the stash every removal and replacement of this env goes through
	Values      Values       // the install and marker rows of the record, defaults where unset
	Stdout      io.Writer
}

// NewEnv resolves the machine for the checkout at repoDir and reads its install values from the record.
func NewEnv(repoDir string, stdout io.Writer) (Env, error) {
	return NewEnvAt(repoDir, "", stdout)
}

// NewEnvAt is NewEnv with an explicit Claude home, empty for the default one.
func NewEnvAt(repoDir, claudeHome string, stdout io.Writer) (Env, error) {
	absolute, err := filepath.Abs(repoDir)
	if err != nil {
		return Env{}, fmt.Errorf("NewEnvAt: %w", err)
	}

	home, err := config.NativeHome()
	if err != nil {
		return Env{}, fmt.Errorf("NewEnvAt: %w", err)
	}

	resolvedClaudeHome, err := config.ClaudeHomeDir(claudeHome)
	if err != nil {
		return Env{}, fmt.Errorf("NewEnvAt: %w", err)
	}

	env := Env{
		ClaudeHome:  resolvedClaudeHome,
		GOOS:        runtime.GOOS,
		Home:        home,
		IsPayload:   HasPayload(absolute),
		MainRepoDir: gitroot.MainRoot(absolute),
		RepoDir:     absolute,
		Stdout:      stdout,
	}
	record, err := LoadCurrent(env)
	if err != nil {
		return Env{}, fmt.Errorf("NewEnvAt: %w", err)
	}

	env.Values, err = InstallValues(record)
	if err != nil {
		return Env{}, fmt.Errorf("NewEnvAt: %w", err)
	}
	env.Trash = trash.New(filepath.Join(resolvedClaudeHome, stateDirName), env.Values.TrashRetentionDays)
	return env, nil
}

// markers is the env's marker table: ports, homes and the session-helpers sounds.
func (e Env) markers() config.Markers {
	return e.Values.Markers(e.Home, e.MainRepoDir)
}

// Health is one component's state on Welcome and Profile.
type Health struct {
	Ok     bool
	Detail string
	Fix    string
}

// Result is what Apply did for one component; Steps are the writes its plan held, the restart of a daemon among them.
type Result struct {
	Component components.ID
	Action    string
	Err       error
	Steps     []Step
}

// Step is one planned write, shown in previews and reports.
type Step struct {
	Component components.ID
	Action    string // "install" | "update" | "remove" | "kept"
	Detail    string // "~/.claude/hooks/bin/toolpolicy"
}

// applyOne installs or removes a component only when its plan holds steps.
func applyOne(ctx context.Context, env Env, component Component, isSelected bool) Result {
	result := Result{Action: ActionUnchanged, Component: component.ID()}
	steps, err := component.Plan(ctx, env, isSelected)
	result.Steps = steps
	if err != nil {
		result.Err = err
		return result
	}
	if !slices.ContainsFunc(steps, isDueStep) {
		return result
	}

	if !isSelected {
		result.Action = ActionRemove
		result.Err = component.Remove(ctx, env)
		return result
	}

	result.Action = ActionInstall
	result.Err = component.Install(ctx, env)
	isRestartDue := result.Err == nil && slices.ContainsFunc(steps, isServerAgentStep)
	if isRestartDue {
		result.Action = ActionRestartRequired
	}
	return result
}

// installOrder is catalog order with Skills last: its sync probes the provider binaries the other components install, so a skill needing one deploys in the same apply.
func installOrder(all []Component) []Component {
	ordered := slices.Clone(all)
	index := slices.IndexFunc(ordered, func(component Component) bool { return component.ID() == components.Skills })
	if index < 0 {
		return ordered
	}
	skills := ordered[index]
	return append(slices.Delete(ordered, index, index+1), skills)
}

// isDueStep is a step that writes or removes; a kept step only reports a file left alone.
func isDueStep(step Step) bool {
	return step.Action != ActionKept
}

// LoadCurrent is the saved record every apply entry point and render reads, FullRecord when none is saved.
func LoadCurrent(env Env) (Record, error) {
	record, _, err := Load(env)
	if err != nil {
		return Record{}, err
	}
	return record, nil
}

// markApplied re-reads the saved record and saves it with applied true; the loop's writers (the skills sync) may have saved in between, so the base is re-read.
func markApplied(env Env) error {
	record, err := LoadCurrent(env)
	if err != nil {
		return fmt.Errorf("markApplied: %w", err)
	}
	record.Applied = true
	if err := Save(env, record); err != nil {
		return fmt.Errorf("markApplied: %w", err)
	}
	return nil
}

// Apply saves the record with applied false before the first step, installs selected components in install order, removes the rest in reverse catalog order, then saves applied true (master A5).
func Apply(ctx context.Context, env Env, r Record) ([]Result, error) {
	r.Applied = false
	if err := Save(env, r); err != nil {
		return nil, fmt.Errorf("Apply: %w", err)
	}

	record := expandPreset(r, env.GOOS)
	all := installers(record)
	results := make([]Result, 0, len(all))
	installs := installOrder(all)
	for _, component := range installs {
		if slices.Contains(record.Components, component.ID()) {
			results = append(results, applyOne(ctx, env, component, true))
		}
	}
	for _, component := range slices.Backward(all) {
		if !slices.Contains(record.Components, component.ID()) {
			results = append(results, applyOne(ctx, env, component, false))
		}
	}
	return results, markApplied(env)
}

// ApplyComponent runs Apply's install step for one selected component, plus Session hooks when its rows render into the hooks config: the CLI's and the Hooks tab's Save.
func ApplyComponent(ctx context.Context, env Env, r Record, id components.ID) ([]Result, error) {
	record := expandPreset(r, env.GOOS)
	if !slices.Contains(record.Components, id) {
		if err := Save(env, r); err != nil {
			return nil, fmt.Errorf("ApplyComponent: %w", err)
		}
		return nil, fmt.Errorf("ApplyComponent: Component %s is not selected", id)
	}

	values, err := InstallValues(record)
	if err != nil {
		return nil, fmt.Errorf("ApplyComponent: %w", err)
	}
	env.Values = values

	r.Applied = false
	if err := Save(env, r); err != nil {
		return nil, fmt.Errorf("ApplyComponent: %w", err)
	}

	applied := []components.ID{id}
	if id != components.SessionHooks && rendersHooksConfig(id) {
		applied = append(applied, components.SessionHooks)
	}
	var results []Result
	for _, component := range installers(record) {
		if slices.Contains(applied, component.ID()) {
			results = append(results, applyOne(ctx, env, component, true))
		}
	}
	return results, markApplied(env)
}

// Failed reports whether any result carries an error.
func Failed(results []Result) bool {
	return slices.ContainsFunc(results, func(result Result) bool { return result.Err != nil })
}

// Plan lists the steps Apply would run to move the machine to r.
func Plan(ctx context.Context, env Env, r Record) ([]Step, error) {
	record := expandPreset(r, env.GOOS)
	var steps []Step
	for _, component := range installers(record) {
		componentSteps, err := component.Plan(ctx, env, slices.Contains(record.Components, component.ID()))
		if err != nil {
			return steps, fmt.Errorf("Plan: %s: %w", component.ID(), err)
		}
		steps = append(steps, componentSteps...)
	}
	return steps, nil
}

// Status returns one row per catalog entry in catalog order.
func Status(ctx context.Context, env Env, r Record) []ComponentStatus {
	var rows []ComponentStatus
	for _, component := range installers(r) {
		catalogRow, _ := components.Lookup(component.ID())
		row := ComponentStatus{
			Available: components.Available(component.ID(), env.GOOS),
			Component: catalogRow,
			Selected:  slices.Contains(r.Components, component.ID()),
		}
		if row.Selected && row.Available {
			row.Health = component.Health(ctx, env)
		}
		rows = append(rows, row)
	}
	return rows
}

// Summary renders one line per result, failures with their error.
func Summary(results []Result) string {
	var summary strings.Builder
	for _, result := range results {
		fmt.Fprintf(&summary, "%s: %s", result.Component, result.Action)
		if result.Err != nil {
			fmt.Fprintf(&summary, " — FAILED: %v", result.Err)
		}
		summary.WriteString("\n")
	}
	return summary.String()
}

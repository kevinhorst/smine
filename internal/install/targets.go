package install

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/skills"
)

// Sync targets of the Overview's Sync widget; all is the full Apply of the record.
const (
	SyncTargetAll      = "all"
	SyncTargetHooks    = "hooks"
	SyncTargetSettings = "settings"
	SyncTargetSkills   = "skills"
)

// SyncTargetOrder is the order a picked subset runs in: settings, hooks, then the skills deploy.
var SyncTargetOrder = []string{SyncTargetSettings, SyncTargetHooks, SyncTargetSkills}

// ApplyHooks refreshes the hook binaries and scripts of the selected components only.
func ApplyHooks(ctx context.Context, env Env, r Record) ([]Step, error) {
	record := expandPreset(r, env.GOOS)
	var steps []Step
	for _, component := range installers(record) {
		if !slices.Contains(record.Components, component.ID()) {
			continue
		}

		componentSteps, err := component.InstallHooks(ctx, env)
		steps = append(steps, componentSteps...)
		if err != nil {
			return steps, fmt.Errorf("ApplyHooks: %s: %w", component.ID(), err)
		}
	}
	return steps, nil
}

// IsSyncTarget accepts all and every SyncTargetOrder entry.
func IsSyncTarget(target string) bool {
	return target == SyncTargetAll || slices.Contains(SyncTargetOrder, target)
}

// SyncTargets writes its report to env.Stdout; all runs Apply alone, the rest run in SyncTargetOrder and stop at the first failure.
func SyncTargets(ctx context.Context, env Env, r Record, targets []string) error {
	if slices.Contains(targets, SyncTargetAll) {
		fmt.Fprintln(env.Stdout, "== "+SyncTargetAll)
		results, err := Apply(ctx, env, r)
		fmt.Fprint(env.Stdout, Summary(results))
		if err == nil && Failed(results) {
			err = errors.New("SyncTargets: A component failed — see the report")
		}
		return err
	}

	for _, target := range SyncTargetOrder {
		if !slices.Contains(targets, target) {
			continue
		}

		fmt.Fprintln(env.Stdout, "== "+target)
		if err := syncTarget(ctx, env, r, target); err != nil {
			return fmt.Errorf("SyncTargets: %s: %w", target, err)
		}
	}
	return nil
}

func printChanges(env Env, changes []string) {
	if len(changes) == 0 {
		fmt.Fprintln(env.Stdout, "(no changes)")
		return
	}
	for _, change := range changes {
		fmt.Fprintln(env.Stdout, change)
	}
}

func syncTarget(ctx context.Context, env Env, r Record, target string) error {
	switch target {
	case SyncTargetSettings:
		changes, err := ApplySettings(env, r)
		printChanges(env, changes)
		return err
	case SyncTargetHooks:
		steps, err := ApplyHooks(ctx, env, r)
		changes := make([]string, 0, len(steps))
		for _, step := range steps {
			changes = append(changes, step.Action+" "+step.Detail)
		}
		printChanges(env, changes)
		return err
	case SyncTargetSkills:
		if !slices.Contains(expandPreset(r, env.GOOS).Components, components.Skills) {
			fmt.Fprintln(env.Stdout, "skills: component not selected — nothing deployed")
			return nil
		}
		selection := &skills.Selection{Chosen: r.Skills.Chosen, Mode: r.Skills.Mode}
		return syncSkills(env, selection, expandPreset(r, env.GOOS).Components)
	}
	return fmt.Errorf("syncTarget: Unknown sync target %q", target)
}

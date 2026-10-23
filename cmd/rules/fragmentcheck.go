package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/reviewscope"
	"github.com/kevinhorst/smine/internal/shell"
)

const (
	fragmentCheckUsage  = "usage: rules fragment-check [--base <ref>] [--mainline main] [--settings settings/claude_code]"
	defaultSettingsDir  = "settings/claude_code"
	shortCommitLength   = 12
	fragmentCheckPrefix = "fragment-check: "
)

// fragmentDrift is one group whose hooks changed against the base copy without a higher smineVersion: the apply keeps an equal-version live group's command, so the change never reaches a machine.
type fragmentDrift struct {
	BaseVersion config.HookVersion
	Id          string
	Path        string
	Version     config.HookVersion
}

func (d *fragmentDrift) String() string {
	return fmt.Sprintf("%s: %s: hooks changed against the base but smineVersion stays %s (base %s) — bump it, the apply keeps an equal-version live group's command", d.Path, d.Id, d.Version, d.BaseVersion)
}

// hookIdentity is the slice of a group a machine must receive: its name and its hooks without the render-owned timeout and async fields.
type hookIdentity struct {
	Hooks []config.Hook
	Name  string
}

// checkFragments compares every component fragment of settingsDir against its copy at baseCommit; a fragment or group absent there is new and passes.
func checkFragments(ctx context.Context, baseCommit, repoDir, settingsDir string) ([]*fragmentDrift, error) {
	componentsDir := filepath.ToSlash(filepath.Join(settingsDir, config.ComponentFragmentDir))
	atBase, err := trackedAt(ctx, baseCommit, componentsDir, repoDir)
	if err != nil {
		return nil, fmt.Errorf("checkFragments: %w", err)
	}

	paths, err := filepath.Glob(filepath.Join(repoDir, componentsDir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("checkFragments: %w", err)
	}
	slices.Sort(paths)

	var drifts []*fragmentDrift
	for _, path := range paths {
		relative, err := filepath.Rel(repoDir, path)
		if err != nil {
			return nil, fmt.Errorf("checkFragments: %w", err)
		}
		relative = filepath.ToSlash(relative)
		if !atBase[relative] {
			continue
		}

		current, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("checkFragments: %w", err)
		}
		base, err := shell.Run(ctx, repoDir, "git", "show", baseCommit+":"+relative)
		if err != nil {
			return nil, fmt.Errorf("checkFragments: %w", err)
		}

		currentGroups, err := fragmentGroups(current, relative)
		if err != nil {
			return nil, fmt.Errorf("checkFragments: %w", err)
		}
		baseGroups, err := fragmentGroups([]byte(base), relative)
		if err != nil {
			return nil, fmt.Errorf("checkFragments: %s at %s: %w", relative, baseCommit, err)
		}
		found, err := fragmentDrifts(baseGroups, currentGroups, relative)
		if err != nil {
			return nil, fmt.Errorf("checkFragments: %w", err)
		}
		drifts = append(drifts, found...)
	}
	return drifts, nil
}

// fragmentDrifts lists the groups of current whose identity differs from base's at an equal or lower version.
func fragmentDrifts(base, current map[string]config.HookGroup, path string) ([]*fragmentDrift, error) {
	var drifts []*fragmentDrift
	ids := slices.Sorted(maps.Keys(current))
	for _, id := range ids {
		baseGroup, isKnown := base[id]
		if !isKnown {
			continue
		}

		group := current[id]
		if identityOf(&group) == identityOf(&baseGroup) {
			continue
		}
		order, err := group.Version.Compare(baseGroup.Version)
		if err != nil {
			return nil, fmt.Errorf("fragmentDrifts: %s: %w", id, err)
		}
		if order > 0 {
			continue
		}
		drifts = append(drifts, &fragmentDrift{BaseVersion: baseGroup.Version, Id: id, Path: path, Version: group.Version})
	}
	return drifts, nil
}

// fragmentGroups decodes a fragment's groups by id; the component is the file's stem, the markers stay literal on both sides.
func fragmentGroups(data []byte, relative string) (map[string]config.HookGroup, error) {
	component := strings.TrimSuffix(filepath.Base(relative), ".json")
	fragment, err := config.ParseFragment(data, component, config.Markers{})
	if err != nil {
		return nil, fmt.Errorf("fragmentGroups: %s: %w", relative, err)
	}

	groups, err := fragment.HookGroups()
	if err != nil {
		return nil, fmt.Errorf("fragmentGroups: %s: %w", relative, err)
	}
	byId := make(map[string]config.HookGroup, len(groups))
	for _, group := range groups {
		byId[group.Id] = group
	}
	return byId, nil
}

// identityOf is the JSON of the group's hook identity, the comparison key of the check.
func identityOf(group *config.HookGroup) string {
	identity := hookIdentity{Hooks: make([]config.Hook, 0, len(group.Hooks)), Name: group.Name}
	for _, hook := range group.Hooks {
		hook.Async = false
		hook.Timeout = 0
		identity.Hooks = append(identity.Hooks, hook)
	}

	encoded, err := json.Marshal(identity)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func runFragmentCheck(args []string) int {
	flags := flag.NewFlagSet("fragment-check", flag.ContinueOnError)
	bracket := flags.String("base", "", "explicit base ref; absent = the upstream fork point, else the merge-base with the mainline")
	mainline := flags.String("mainline", "main", "mainline branch")
	settingsDir := flags.String("settings", defaultSettingsDir, "the settings template dir holding the component fragments")
	if err := flags.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, fragmentCheckUsage)
		return parseExit(err)
	}

	ctx := context.Background()
	base, err := reviewscope.DeriveBase(ctx, ".", "HEAD", *mainline, *bracket)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	drifts, err := checkFragments(ctx, base.Commit, ".", *settingsDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	for _, drift := range drifts {
		fmt.Println(drift)
	}
	if len(drifts) > 0 {
		fmt.Printf("%s%d group(s) changed without a smineVersion bump against %s\n", fragmentCheckPrefix, len(drifts), shortCommit(base.Commit))
		return exitError
	}
	fmt.Printf("%severy changed group against %s carries a bumped smineVersion\n", fragmentCheckPrefix, shortCommit(base.Commit))
	return exitClean
}

// shortCommit is the abbreviated commit the report names.
func shortCommit(commit string) string {
	if len(commit) <= shortCommitLength {
		return commit
	}
	return commit[:shortCommitLength]
}

// trackedAt is the set of files under dir at commit, repo-relative with forward slashes.
func trackedAt(ctx context.Context, commit, dir, repoDir string) (map[string]bool, error) {
	listed, err := shell.Run(ctx, repoDir, "git", "ls-tree", "-r", "--name-only", commit, "--", dir)
	if err != nil {
		return nil, fmt.Errorf("trackedAt: %w", err)
	}

	tracked := map[string]bool{}
	for line := range strings.SplitSeq(strings.TrimSpace(listed), "\n") {
		if line != "" {
			tracked[filepath.ToSlash(line)] = true
		}
	}
	return tracked, nil
}

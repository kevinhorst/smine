package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/trash"
)

// Skill modes of a Selection (master C1).
const (
	SelectionModeAll    = "all"
	SelectionModeChosen = "chosen"
	SelectionModeNone   = "none"
)

// selectionModes are the modes a Selection accepts.
var selectionModes = []string{SelectionModeAll, SelectionModeChosen, SelectionModeNone}

// Selection is the skill part of the selection record (master C1): every available skill, the chosen leaves, or none.
type Selection struct {
	Mode   string
	Chosen []string
}

// isChosen: a nil selection or mode all deploys every leaf, mode none no leaf — a prune then removes every marked dir.
func (s *Selection) isChosen(leaf string) bool {
	if s == nil || s.Mode == SelectionModeAll {
		return true
	}
	return slices.Contains(s.Chosen, leaf)
}

// validate refuses a mode outside the three; a nil selection is valid.
func (s *Selection) validate() error {
	if s == nil || slices.Contains(selectionModes, s.Mode) {
		return nil
	}
	return fmt.Errorf("Selection.validate: Unknown skills mode %q", s.Mode) //nolint:staticcheck // ST1005: the Receiver.method prefix is the repo's error-message convention
}

// SyncOptions is one deploy of the repo skills into the skill homes.
type SyncOptions struct {
	Home       string          // user home holding .claude, .codex and .config
	Into       string          // variant target root, empty for the claude skills root
	Prune      bool            // remove marked installed skills and agents outside the deploy
	RepoDir    string          // checkout to deploy from
	Trash      *trash.Store    // the stash every removed dir, pruned file and replaced profile goes to
	Variant    string          // <leaf>=<name>:<disable>, empty for the full sync
	Rows       *Rows           // the record's SKILL_* and PROVIDER_* rows; nil = none
	ContextDir string          // the context.context_dir setting the deploy's context links resolve against
	Selection  *Selection      // nil = all
	Components []components.ID // nil = every component
}

func RepoSyncOptions(scriptsDir string) (*SyncOptions, error) {
	absolute, err := filepath.Abs(scriptsDir)
	if err != nil {
		return nil, fmt.Errorf("RepoSyncOptions: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("RepoSyncOptions: %w", err)
	}

	stateDir, err := config.StateDir("")
	if err != nil {
		return nil, fmt.Errorf("RepoSyncOptions: %w", err)
	}

	options := &SyncOptions{
		Home:    home,
		RepoDir: filepath.Dir(filepath.Dir(absolute)),
		Trash:   trash.New(stateDir, 0),
	}
	return options, nil
}

func newDeployRun(options *SyncOptions) *deployRun {
	contextDir := options.ContextDir
	if contextDir == "" {
		contextDir = contextDirDefault
	}
	run := &deployRun{
		contextDir:   contextDir,
		deferredHint: deferredHintSync,
		isProbing:    true,
		options:      options,
		report:       &strings.Builder{},
	}
	return run
}

// DeployLeaf refuses a skill the component set cannot run; casual-profile overrides wait for the next full sync.
func DeployLeaf(leaf string, options *SyncOptions) (string, error) {
	run := newDeployRun(options)
	source, err := run.loadLeaf(leaf)
	if err != nil {
		return run.report.String(), fmt.Errorf("DeployLeaf: %w", err)
	}

	missing := skillgraph.MissingRequirements(options.Components, source.skill)
	if len(missing) > 0 {
		return run.report.String(), fmt.Errorf("DeployLeaf: Skill %s unavailable (%s)", leaf, joinIds(missing))
	}

	unconfigured := Unconfigured(run.values(true, source))
	if len(unconfigured) > 0 {
		return run.report.String(), fmt.Errorf("DeployLeaf: Skill %s unconfigured (%s)", leaf, strings.Join(unconfigured, ", "))
	}

	sources, err := run.repoLeaves()
	if err != nil {
		return run.report.String(), fmt.Errorf("DeployLeaf: %w", err)
	}

	run.deployable = run.deployableLeaves(sources)
	for _, target := range run.targets() {
		isDeferring := target == run.claudePath(skillsDirName)
		unconfigured := Unconfigured(run.values(isDeferring, source))
		if len(unconfigured) > 0 {
			run.logf("  skip %s unconfigured (%s) in %s", leaf, strings.Join(unconfigured, ", "), target)
			continue
		}

		targetDir := filepath.Join(target, leaf)
		previous := deployedVersion(targetDir)
		files, err := run.deployLeaf(nil, isDeferring, source, "", targetDir)
		if err != nil {
			return run.report.String(), fmt.Errorf("DeployLeaf: %w", err)
		}

		run.logf("  %s -> %s%s", leaf, targetDir, versionNote(source.skill.Version(), previous))
		if err := run.deployAliases(files, source.skill, target); err != nil {
			return run.report.String(), fmt.Errorf("DeployLeaf: %w", err)
		}
	}
	if err := run.deployAgents(source); err != nil {
		return run.report.String(), fmt.Errorf("DeployLeaf: %w", err)
	}
	return run.report.String(), nil
}

// RemoveLeaf removes the skill and its alias copies from both homes; an unmarked dir is refused, a missing one skipped.
func RemoveLeaf(leaf string, options *SyncOptions) (string, error) {
	run := newDeployRun(options)
	source, err := run.loadLeaf(leaf)
	if err != nil {
		return run.report.String(), fmt.Errorf("RemoveLeaf: %w", err)
	}

	names := append([]string{leaf}, source.skill.Aliases()...)
	for _, target := range run.targets() {
		for _, name := range names {
			if err := run.removeInstalled(filepath.Join(target, name)); err != nil {
				return run.report.String(), fmt.Errorf("RemoveLeaf: %w", err)
			}
		}
	}
	if err := run.removeAgents(source); err != nil {
		return run.report.String(), fmt.Errorf("RemoveLeaf: %w", err)
	}
	return run.report.String(), nil
}

// Sync deploys one variant when options.Variant is set, else the selection (mode none deploys nothing, so a prune removes every marked dir); the report returns also on failure.
func Sync(options *SyncOptions) (string, error) {
	if err := options.Selection.validate(); err != nil {
		return "", fmt.Errorf("Sync: %w", err)
	}

	run := newDeployRun(options)
	run.logf("using context dir: %s", run.contextDir)
	deploy := run.deployAll
	if options.Variant != "" {
		deploy = run.deployVariant
	}
	if err := deploy(); err != nil {
		return run.report.String(), fmt.Errorf("Sync: %w", err)
	}
	return run.report.String(), nil
}

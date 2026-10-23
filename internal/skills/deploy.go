package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/kevinhorst/smine/internal/acdsl"
	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/presentation"
	"github.com/kevinhorst/smine/internal/shell"
	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/trash"
)

const (
	// agentMarkerLine is the line of the marker inside an agent file's front matter: after the opening ---.
	agentMarkerLine   = 2
	actionSkillsSync  = "skills sync"
	agentsDirName     = "agents"
	claudeDirName     = ".claude"
	codexDirName      = ".codex"
	contextDirDefault = "docs"
	contextDirKey     = "AGENT_CONTEXT_DIR_DEFAULT"
	contextDirToken   = "$" + contextDirKey
	deferredHintShell = "export it in your shell"
	deferredHintSync  = "set it for this repo on the skill page, or export it"
	envKeepFormal     = "KEEP_FORMAL"
	evalDirName       = "eval"
	fieldLicense      = "license"
	markerFileName    = ".smine-deployed"
	maxVariantNameLen = 24
	overrideOn        = "on"
	pruneSkipName     = "codex-primary-runtime"
	renderedFileMode  = fs.FileMode(0o644)
	skillsDirName     = "skills"
	titleCodeField    = "title-code"
	variantSeparator  = "--"
)

var (
	// legacyAudiencePattern is the profile line of the renamed audience value.
	legacyAudiencePattern = regexp.MustCompile(`(?m)^audience:[ \t]*non-developer`)
	// skippedSiblingDirs are the skill subdirs deploy never copies into the skill dir.
	skippedSiblingDirs = []string{agentsDirName, evalDirName, skillgraph.DirPayloads, skillgraph.DirSmoke}
	// variantNamePattern is a kebab-case variant name.
	variantNamePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
)

// deployedFile's path is relative to the skill dir and slash-separated.
type deployedFile struct {
	content []byte
	mode    fs.FileMode
	path    string
}

type deployRun struct {
	contextDir   string
	deferredHint string
	deployable   map[string]bool
	isProbing    bool
	options      *SyncOptions
	probes       map[string]*probeResult
	report       *strings.Builder
}

// claudePath is a path below the Claude home.
func (r *deployRun) claudePath(parts ...string) string {
	return filepath.Join(append([]string{r.options.Home, claudeDirName}, parts...)...)
}

// deployAgents copies <leaf dir>/agents/*.md into the Claude home; Codex has no Agent-tool agents.
func (r *deployRun) deployAgents(leaf *sourceLeaf) error {
	sources, err := filepath.Glob(filepath.Join(leaf.dir, agentsDirName, "*.md"))
	if err != nil {
		return fmt.Errorf("deployRun.deployAgents: %w", err)
	}

	target := r.claudePath(agentsDirName)
	values := ValueMap(r.values(true, leaf))
	for _, source := range sources {
		content, err := os.ReadFile(source)
		if err != nil {
			return fmt.Errorf("deployRun.deployAgents: %w", err)
		}

		name := filepath.Base(source)
		filled := skillgraph.FillVariables(acdsl.StripProjection(source, content), values)
		installed := filepath.Join(target, name)
		current, readErr := os.ReadFile(installed)
		isPresent := readErr == nil
		isOurs := isPresent && hasAgentMarker(current)
		isAdopted := isPresent && !isOurs && bytes.Equal(current, filled)
		if isPresent && !isOurs && !isAdopted {
			r.logf("  agent %s kept: no smine marker", installed)
			continue
		}
		if err := fsx.WriteFileAtomic(markedAgent(filled), installed, renderedFileMode); err != nil {
			return fmt.Errorf("deployRun.deployAgents: %w", err)
		}

		note := ""
		if isAdopted {
			note = " (adopted, marker added)"
		}
		r.logf("  agent %s -> %s%s", name, target, note)
	}
	return nil
}

// deployAliases writes one copy of the deployed files, deploy marker included, per declared alias.
func (r *deployRun) deployAliases(files []*deployedFile, skill *skillgraph.Skill, target string) error {
	for _, alias := range skill.Aliases() {
		aliasDir := filepath.Join(target, alias)
		if err := r.writeFiles(aliasFiles(alias, files, skill), aliasDir); err != nil {
			return fmt.Errorf("deployRun.deployAliases: %w", err)
		}

		r.logf("  %s -> %s (alias of %s)", alias, aliasDir, skill.Name())
	}
	return nil
}

func (r *deployRun) deployAll() error {
	sources, err := r.repoLeaves()
	if err != nil {
		return err
	}

	r.deployable = r.deployableLeaves(sources)
	leaves := r.selectedLeaves(sources)
	for _, target := range r.targets() {
		if err := r.deployTarget(leaves, target); err != nil {
			return err
		}
	}

	names := make([]string, 0, len(leaves))
	deployedNames := make([]string, 0, len(leaves))
	for _, leaf := range leaves {
		names = append(names, leaf.name())
		deployedNames = append(deployedNames, leaf.name())
		deployedNames = append(deployedNames, leaf.skill.Aliases()...)
	}
	if err := r.syncAgents(names, sources); err != nil {
		return err
	}
	if err := r.removeRetiredTools(); err != nil {
		return err
	}
	return r.syncOverrides(deployedNames)
}

// deployLeaf writes nothing when the model does not render; the returned files include the deploy marker.
func (r *deployRun) deployLeaf(disable []string, isDeferring bool, leaf *sourceLeaf, renamed, targetDir string) ([]*deployedFile, error) {
	files, err := r.leafFiles(disable, isDeferring, leaf, renamed)
	if err != nil {
		return nil, fmt.Errorf("deployRun.deployLeaf: %w", err)
	}

	stamp, err := markerContent(leaf.dir, files)
	if err != nil {
		return nil, fmt.Errorf("deployRun.deployLeaf: %w", err)
	}
	marker := &deployedFile{content: stamp, mode: renderedFileMode, path: markerFileName}
	files = append(files, marker)
	if err := r.writeFiles(files, targetDir); err != nil {
		return nil, fmt.Errorf("deployRun.deployLeaf: %w", err)
	}
	return files, nil
}

// deployTarget renders the Claude home with repo-scoped values deferred to invocation, the Codex home without, skipping a leaf unconfigured for that home.
func (r *deployRun) deployTarget(leaves []*sourceLeaf, target string) error {
	isDeferring := target == r.claudePath(skillsDirName)
	names := map[string]bool{}
	for _, leaf := range leaves {
		unconfigured := Unconfigured(r.values(isDeferring, leaf))
		if len(unconfigured) > 0 {
			r.logf("  skip %s unconfigured (%s) in %s", leaf.name(), strings.Join(unconfigured, ", "), target)
			continue
		}

		targetDir := filepath.Join(target, leaf.name())
		previous := deployedVersion(targetDir)
		files, err := r.deployLeaf(nil, isDeferring, leaf, "", targetDir)
		if err != nil {
			return err
		}

		r.logf("  %s -> %s%s", leaf.name(), targetDir, versionNote(leaf.skill.Version(), previous))
		if err := r.deployAliases(files, leaf.skill, target); err != nil {
			return err
		}

		names[leaf.name()] = true
		for _, alias := range leaf.skill.Aliases() {
			names[alias] = true
		}
	}
	if err := r.pruneInstalled(names, target); err != nil {
		return err
	}

	r.logf("synced to %s", target)
	return nil
}

// deployVariant's <leaf>--<name> dir never shadows the real skill.
func (r *deployRun) deployVariant() error {
	leaf, rest, hasName := strings.Cut(r.options.Variant, "=")
	name, disable, hasDisable := strings.Cut(rest, ":")
	if !hasName || !hasDisable {
		return fmt.Errorf("deployRun.deployVariant: Variant %q is not <leaf>=<name>:<disable>", r.options.Variant)
	}
	if !variantNamePattern.MatchString(name) {
		return fmt.Errorf("deployRun.deployVariant: Variant name must be kebab-case: %q", name)
	}
	if len(name) > maxVariantNameLen {
		return fmt.Errorf("deployRun.deployVariant: Variant name longer than %d chars: %s", maxVariantNameLen, name)
	}

	source, err := r.loadLeaf(leaf)
	if err != nil {
		return err
	}

	unconfigured := Unconfigured(r.values(true, source))
	if len(unconfigured) > 0 {
		return fmt.Errorf("deployRun.deployVariant: Skill %s unconfigured (%s)", leaf, strings.Join(unconfigured, ", "))
	}

	root := r.options.Into
	if root == "" {
		root = r.claudePath(skillsDirName)
	}
	renamed := leaf + variantSeparator + name
	targetDir := filepath.Join(root, renamed)
	if _, err := r.deployLeaf(strings.Split(disable, ","), true, source, renamed, targetDir); err != nil {
		return err
	}

	r.logf("rendered variant %s (disabled: %s) -> %s", renamed, disable, targetDir)
	return nil
}

// deployableLeaves marks every repo leaf the run can deploy: requirements met and every required variable set or deferred to invocation.
func (r *deployRun) deployableLeaves(sources []*sourceLeaf) map[string]bool {
	deployable := make(map[string]bool, len(sources))
	for _, source := range sources {
		values := r.values(true, source)
		isAvailable := len(skillgraph.MissingRequirements(r.options.Components, source.skill)) == 0
		isConfigured := len(skillgraph.UnconfiguredVariables(DeferredNames(values), source.skill, ValueMap(values))) == 0
		deployable[source.name()] = isAvailable && isConfigured
	}
	return deployable
}

// leafFiles lists nothing when the model does not render; sibling text files arrive filled, binary ones as they are.
func (r *deployRun) leafFiles(disable []string, isDeferring bool, leaf *sourceLeaf, renamed string) ([]*deployedFile, error) {
	rendered, err := r.renderedFiles(disable, isDeferring, leaf, renamed)
	if err != nil {
		return nil, err
	}

	files, err := siblingFiles(leaf.dir)
	if err != nil {
		return nil, err
	}

	values := ValueMap(r.values(isDeferring, leaf))
	for _, file := range files {
		if utf8.Valid(file.content) {
			file.content = skillgraph.FillVariables(file.content, values)
		}
	}
	return append(files, rendered...), nil
}

// loadLeaf resolves a leaf name to its repo skill dir and loads its model.
func (r *deployRun) loadLeaf(leaf string) (*sourceLeaf, error) {
	leafDir, err := findLeaf(leaf, filepath.Join(r.options.RepoDir, skillsDirName))
	if err != nil {
		return nil, err
	}

	skill, err := skillgraph.Load(leafDir)
	if err != nil {
		return nil, fmt.Errorf("deployRun.loadLeaf: %w", err)
	}
	return &sourceLeaf{dir: leafDir, skill: skill}, nil
}

// logf appends one line to the report.
func (r *deployRun) logf(format string, args ...any) {
	fmt.Fprintf(r.report, format+"\n", args...)
}

func (r *deployRun) manifest(deployed []byte, renamed string, skill *skillgraph.Skill) ([]byte, error) {
	text := string(deployed)
	if os.Getenv(envKeepFormal) != "" {
		canonical, err := skillgraph.RenderManifest(skill)
		if err != nil {
			return nil, fmt.Errorf("deployRun.manifest: %w", err)
		}
		text = string(canonical)
	}

	text = strings.ReplaceAll(text, contextDirToken, r.contextDir)
	if renamed != "" {
		text = strings.Replace(text, "\nname: "+skill.Name()+"\n", "\nname: "+renamed+"\n", 1)
	}
	return []byte(text), nil
}

// pruneInstalled never counts a variant leftover (<leaf>--<name>) as deployed and never touches an unmarked dir.
func (r *deployRun) pruneInstalled(names map[string]bool, target string) error {
	info, err := os.Lstat(target)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("deployRun.pruneInstalled: %w", err)
	}

	isLinkedRoot := info.Mode()&fs.ModeSymlink != 0
	entries, err := os.ReadDir(target)
	if err != nil {
		return fmt.Errorf("deployRun.pruneInstalled: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		installed := filepath.Join(target, name)
		isVariant := strings.Contains(name, variantSeparator)
		isRepoSkill := names[name] && !isVariant
		if !entry.IsDir() || isRepoSkill {
			continue
		}
		if name == pruneSkipName {
			r.logf("  skipped (prune skip-list): %s", installed)
			continue
		}
		if !hasMarker(installed) {
			r.logf("  kept (not deployed by smine): %s", installed)
			continue
		}
		if !r.options.Prune {
			r.logf("  kept (not deployed, prune removes it): %s", installed)
			continue
		}
		if isLinkedRoot {
			return fmt.Errorf("deployRun.pruneInstalled: Refusing to prune under the symlinked root %s", target)
		}
		entry := &trash.Entry{Action: actionSkillsSync, Reason: "marked dir outside the selection"}
		stash, err := r.options.Trash.Move(installed, entry)
		if err != nil {
			return fmt.Errorf("deployRun.pruneInstalled: %w", err)
		}
		r.logf("  pruned: %s -> trash %s", installed, stash)
	}
	return nil
}

// pruneUnwanted moves every file the render did not produce into the trash, then removes the dirs left empty (master D3).
func (r *deployRun) pruneUnwanted(targetDir string, wanted map[string]bool) error {
	var dirs []string
	err := filepath.WalkDir(targetDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			dirs = append(dirs, path)
			return nil
		}

		rel, err := filepath.Rel(targetDir, path)
		if err != nil {
			return fmt.Errorf("deployRun.pruneUnwanted: %w", err)
		}
		if wanted[filepath.ToSlash(rel)] {
			return nil
		}
		stashed := &trash.Entry{Action: actionSkillsSync, Reason: "not produced by the render"}
		_, err = r.options.Trash.Move(path, stashed)
		return err
	})
	if err != nil {
		return fmt.Errorf("deployRun.pruneUnwanted: %w", err)
	}

	slices.Reverse(dirs)
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("deployRun.pruneUnwanted: %w", err)
		}
		if len(entries) > 0 {
			continue
		}
		if err := os.Remove(dir); err != nil {
			return fmt.Errorf("deployRun.pruneUnwanted: %w", err)
		}
	}
	return nil
}

// removeAgents skips a definition not installed and keeps one without the smine marker.
func (r *deployRun) removeAgents(leaf *sourceLeaf) error {
	sources, err := filepath.Glob(filepath.Join(leaf.dir, agentsDirName, "*.md"))
	if err != nil {
		return fmt.Errorf("deployRun.removeAgents: %w", err)
	}

	for _, source := range sources {
		installed := r.claudePath(agentsDirName, filepath.Base(source))
		current, err := os.ReadFile(installed)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("deployRun.removeAgents: %w", err)
		}
		if !hasAgentMarker(current) {
			r.logf("  agent %s kept: no smine marker", installed)
			continue
		}

		entry := &trash.Entry{Action: actionSkillsSync, Reason: "agent of a skill outside the selection"}
		if _, err := r.options.Trash.Move(installed, entry); err != nil {
			return fmt.Errorf("deployRun.removeAgents: %w", err)
		}
		r.logf("  agent removed: %s -> trash", installed)
	}
	return nil
}

// removeInstalled skips a missing dir and refuses one without the deploy marker.
func (r *deployRun) removeInstalled(dir string) error {
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if !hasMarker(dir) {
		return fmt.Errorf("deployRun.removeInstalled: Refusing to remove %s — not deployed by smine", dir)
	}
	entry := &trash.Entry{Action: "skills remove", Reason: "removed from the Skills page"}
	stash, err := r.options.Trash.Move(dir, entry)
	if err != nil {
		return fmt.Errorf("deployRun.removeInstalled: %w", err)
	}
	r.logf("  removed: %s -> trash %s", dir, stash)
	return nil
}

// removeRetiredTools stashes the agent tool scripts the agentwt binary replaced.
func (r *deployRun) removeRetiredTools() error {
	for _, name := range []string{"remove_agent_worktrees.sh", filepath.Join("_lib", "verdict.sh")} {
		entry := &trash.Entry{Action: actionSkillsSync, Reason: "retired tool script"}
		if _, err := r.options.Trash.Move(r.claudePath(agentsDirName, "tools", name), entry); err != nil {
			return fmt.Errorf("deployRun.removeRetiredTools: %w", err)
		}
	}
	return nil
}

// renderedFiles renders for the run's component set, deployability and values, deferred ones as invocation placeholders; help-test.md names the eval when the skill dir holds one.
func (r *deployRun) renderedFiles(disable []string, isDeferring bool, leaf *sourceLeaf, renamed string) ([]*deployedFile, error) {
	_, statErr := os.Stat(filepath.Join(leaf.dir, skillgraph.DirEval, skillgraph.FileEvalDefinition))
	values := r.values(isDeferring, leaf)
	options := skillgraph.RenderOptions{
		Components:   r.options.Components,
		Deferred:     DeferredNames(values),
		DeferredHint: r.deferredHint,
		Deployable:   r.deployable,
		Disable:      disable,
		HasEval:      statErr == nil,
		Values:       ValueMap(values),
	}
	files, err := skillgraph.RenderDeployed(options, leaf.skill)
	if err != nil {
		return nil, fmt.Errorf("deployRun.renderedFiles: %w", err)
	}

	rendered := make([]*deployedFile, 0, len(files))
	for _, file := range files {
		content := file.Content
		if file.Path == skillgraph.FileManifest {
			content, err = r.manifest(file.Content, renamed, leaf.skill)
		}
		if err != nil {
			return nil, err
		}

		deployed := &deployedFile{content: content, mode: renderedFileMode, path: file.Path}
		rendered = append(rendered, deployed)
	}
	return rendered, nil
}

// repoLeaves loads every modeled skill dir of the checkout once, sorted by leaf name.
func (r *deployRun) repoLeaves() ([]*sourceLeaf, error) {
	root := filepath.Join(r.options.RepoDir, skillsDirName)
	dirs, err := skillgraph.Discover(root)
	if err != nil {
		return nil, fmt.Errorf("deployRun.repoLeaves: %w", err)
	}
	if len(dirs) == 0 {
		return nil, fmt.Errorf("deployRun.repoLeaves: No skill model file found under %s", root)
	}

	slices.SortFunc(dirs, func(a, b string) int {
		return strings.Compare(filepath.Base(a), filepath.Base(b))
	})
	leaves := make([]*sourceLeaf, 0, len(dirs))
	for _, dir := range dirs {
		skill, err := skillgraph.Load(dir)
		if err != nil {
			return nil, fmt.Errorf("deployRun.repoLeaves: %w", err)
		}

		leaf := &sourceLeaf{dir: dir, skill: skill}
		leaves = append(leaves, leaf)
	}
	return leaves, nil
}

// selectedLeaves keeps chosen sources whose requirements the component set meets, logging every skip.
func (r *deployRun) selectedLeaves(sources []*sourceLeaf) []*sourceLeaf {
	var leaves []*sourceLeaf
	for _, source := range sources {
		if !r.options.Selection.isChosen(source.name()) {
			r.logf("  skip (not chosen): %s", source.name())
			continue
		}

		missing := skillgraph.MissingRequirements(r.options.Components, source.skill)
		if len(missing) > 0 {
			r.logf("  skip %s unavailable (%s)", source.name(), joinIds(missing))
			continue
		}

		unconfigured := Unconfigured(r.values(true, source))
		if len(unconfigured) > 0 {
			r.logf("  skip %s unconfigured (%s)", source.name(), strings.Join(unconfigured, ", "))
			continue
		}
		leaves = append(leaves, source)
	}
	return leaves
}

// stashForeign moves a present target dir aside before a deploy writes into it: an unmarked dir is not ours, a marked dir whose files changed is the user's edit (master D3, D4).
func (r *deployRun) stashForeign(targetDir string, files []*deployedFile) error {
	if _, err := os.Lstat(targetDir); errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	reason := "unmarked dir under a deployed name"
	if hasMarker(targetDir) {
		isModified, err := isLocallyModified(targetDir, renderedHash(files))
		if err != nil {
			return fmt.Errorf("deployRun.stashForeign: %w", err)
		}
		if !isModified {
			return nil
		}
		reason = "locally modified"
	}

	entry := &trash.Entry{Action: actionSkillsSync, Reason: reason}
	stash, err := r.options.Trash.Move(targetDir, entry)
	if err != nil {
		return fmt.Errorf("deployRun.stashForeign: %w", err)
	}
	r.logf("  %s: %s, previous copy stashed at %s", targetDir, reason, stash)
	return nil
}

// syncAgents removes the agents of every other repo leaf only under Prune.
func (r *deployRun) syncAgents(names []string, sources []*sourceLeaf) error {
	for _, source := range sources {
		var err error
		switch {
		case slices.Contains(names, source.name()):
			err = r.deployAgents(source)
		case r.options.Prune:
			err = r.removeAgents(source)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// syncOverrides writes the routine overlay for a casual profile and removes it for a developer one; the user's settings files are never touched (master B16).
func (r *deployRun) syncOverrides(leaves []string) error {
	profilePath := r.claudePath("context", "global", "presentation-profile.md")
	if err := migrateAudience(profilePath, r.options.Trash); err != nil {
		return err
	}

	overlayPath := filepath.Join(r.options.Home, ".config", "claude-routine", "skill-overrides.json")
	if presentation.Load(profilePath).IsDeveloperAudience() {
		err := os.Remove(overlayPath)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("deployRun.syncOverrides: %w", err)
		}
		return nil
	}

	enabled := map[string]string{}
	for _, leaf := range leaves {
		enabled[leaf] = overrideOn
	}
	overlay := config.NewSettings()
	if err := overlay.SetSkillOverrides(enabled); err != nil {
		return fmt.Errorf("deployRun.syncOverrides: %w", err)
	}
	if err := config.Save(overlayPath, overlay, nil); err != nil {
		return fmt.Errorf("deployRun.syncOverrides: %w", err)
	}

	r.logf("  routine overlay -> %s (%d leaves; hide them in your sessions from the Skills page)", overlayPath, len(leaves))
	return nil
}

// targets are the skill homes a deploy writes: Claude's, then Codex's.
func (r *deployRun) targets() []string {
	return []string{r.claudePath(skillsDirName), filepath.Join(r.options.Home, codexDirName, skillsDirName)}
}

// values resolves the leaf's variables for this run's record rows and component set against the checkout; a probing run is the prober, the bundle run probes nothing.
func (r *deployRun) values(isDeferring bool, leaf *sourceLeaf) []VariableValue {
	var prober Prober
	if r.isProbing {
		prober = r
	}
	return Values(
		context.Background(),
		r.options.RepoDir,
		r.options.Rows,
		isDeferring,
		prober,
		r.options.Components,
		leaf.skill,
	)
}

// writeFiles stashes a foreign or edited target dir, writes every file, then stashes files the render did not produce.
func (r *deployRun) writeFiles(files []*deployedFile, targetDir string) error {
	if err := r.stashForeign(targetDir, files); err != nil {
		return err
	}

	wanted := map[string]bool{}
	for _, file := range files {
		wanted[file.path] = true
		path := filepath.Join(targetDir, filepath.FromSlash(file.path))
		if err := fsx.WriteFileAtomic(file.content, path, file.mode); err != nil {
			return fmt.Errorf("deployRun.writeFiles: %w", err)
		}
	}
	return r.pruneUnwanted(targetDir, wanted)
}

// Claims runs `<executable> interfaces` once per executable of the run and caches the answer or the failure.
func (r *deployRun) Claims(ctx context.Context, provider *skillgraph.Provider) ([]string, error) {
	executable := ExpandHome(provider.Executable())
	if cached, isCached := r.probes[executable]; isCached {
		return cached.claims, cached.err
	}

	result := probeProvider(ctx, executable)
	if r.probes == nil {
		r.probes = map[string]*probeResult{}
	}
	r.probes[executable] = result
	return result.claims, result.err
}

// probeResult is one handshake answer: the claimed interface versions, or why the provider did not answer.
type probeResult struct {
	claims []string
	err    error
}

// interfacesAnswer is the handshake document every provider prints.
type interfacesAnswer struct {
	Interfaces []string `json:"interfaces"`
}

// sourceLeaf is one repo skill: its dir and its loaded model.
type sourceLeaf struct {
	dir   string
	skill *skillgraph.Skill
}

// name is the leaf name the skill deploys under.
func (l *sourceLeaf) name() string {
	return filepath.Base(l.dir)
}

// aliasFiles re-stamps the marker's rendered line over the alias copies, whose manifest differs from the skill's.
func aliasFiles(alias string, files []*deployedFile, skill *skillgraph.Skill) []*deployedFile {
	copies := make([]*deployedFile, 0, len(files))
	var marker *deployedFile
	for _, file := range files {
		switch file.path {
		case markerFileName:
			marker = file
		case skillgraph.FileManifest:
			manifest := &deployedFile{content: aliasManifest(alias, file.content, skill), mode: file.mode, path: file.path}
			copies = append(copies, manifest)
		default:
			copies = append(copies, file)
		}
	}
	if marker == nil {
		return copies
	}

	sourceLine, _, _ := strings.Cut(string(marker.content), "\n")
	stamp := sourceLine + "\n" + markerRenderedPrefix + renderedHash(copies) + "\n"
	aliasMarker := &deployedFile{content: []byte(stamp), mode: marker.mode, path: markerFileName}
	return append(copies, aliasMarker)
}

// aliasManifest sets title-code to the skill name so session titles read the same under both names.
func aliasManifest(alias string, manifest []byte, skill *skillgraph.Skill) []byte {
	lines := "\nname: " + alias + "\n"
	if skill.Field(titleCodeField) == "" {
		lines += titleCodeField + ": " + skill.Name() + "\n"
	}

	text := strings.Replace(string(manifest), "\nname: "+skill.Name()+"\n", lines, 1)
	return []byte(text)
}

func deployedVersion(targetDir string) string {
	content, err := os.ReadFile(filepath.Join(targetDir, skillgraph.FileManifest))
	if err != nil {
		return ""
	}
	return parseFrontmatter(string(content)).Version
}

func findLeaf(leaf, root string) (string, error) {
	grouped, err := filepath.Glob(filepath.Join(root, "*", leaf))
	if err != nil {
		return "", fmt.Errorf("findLeaf: %w", err)
	}

	for _, dir := range append([]string{filepath.Join(root, leaf)}, grouped...) {
		if _, err := os.Stat(skillgraph.ModelPath(dir)); err == nil {
			return dir, nil
		}
	}
	return "", fmt.Errorf("findLeaf: No skill leaf named %s under %s", leaf, root)
}

// agentMarkerAt is the line of an agent file's marker: inside the front matter after the opening ---, else line 1.
func agentMarkerAt(content []byte) int {
	if bytes.HasPrefix(content, []byte(frontmatterFence+"\n")) {
		return agentMarkerLine
	}
	return 1
}

// hasAgentMarker reports whether an installed agent file carries the smine marker where markedAgent puts it.
func hasAgentMarker(content []byte) bool {
	return trash.HasMarker(content, agentMarkerAt(content))
}

// hasMarker reports whether a skill dir carries the deploy marker.
func hasMarker(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, markerFileName))
	return err == nil
}

// joinIds joins component ids for a report line.
func joinIds(ids []components.ID) string {
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		names = append(names, string(id))
	}
	return strings.Join(names, ", ")
}

// markedAgent inserts the marker as a YAML comment after the opening front-matter line; a file without front matter gets it as line 1.
func markedAgent(filled []byte) []byte {
	marker := []byte(trash.MarkerLine(filled) + "\n")
	if agentMarkerAt(filled) != agentMarkerLine {
		return append(marker, filled...)
	}
	first, rest, _ := bytes.Cut(filled, []byte{'\n'})
	return bytes.Join([][]byte{first, marker[:len(marker)-1], rest}, []byte{'\n'})
}

// migrateAudience: the audience value non-developer became casual.
func migrateAudience(profilePath string, keep *trash.Store) error {
	content, err := os.ReadFile(profilePath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("migrateAudience: %w", err)
	}
	if !legacyAudiencePattern.Match(content) {
		return nil
	}

	migrated := legacyAudiencePattern.ReplaceAll(content, []byte("audience: casual"))
	entry := &trash.Entry{Action: actionSkillsSync, Reason: "audience value migrated"}
	if _, err := keep.Keep(profilePath, entry); err != nil {
		return fmt.Errorf("migrateAudience: %w", err)
	}
	if err := fsx.WriteFileAtomic(migrated, profilePath, renderedFileMode); err != nil {
		return fmt.Errorf("migrateAudience: %w", err)
	}
	return nil
}

// probeProvider runs the handshake under probeTimeout and decodes its answer.
func probeProvider(ctx context.Context, executable string) *probeResult {
	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	output, err := shell.Run(probeCtx, "", executable, "interfaces")
	if err != nil {
		failed := &probeResult{err: fmt.Errorf("probeProvider: %w", err)}
		return failed
	}

	answer := &interfacesAnswer{}
	if err := json.Unmarshal([]byte(output), answer); err != nil {
		failed := &probeResult{err: fmt.Errorf("probeProvider: %s printed no interfaces document: %w", executable, err)}
		return failed
	}
	return &probeResult{claims: answer.Interfaces}
}

func readSibling(path, rel string) (*deployedFile, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("readSibling: %w", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("readSibling: %w", err)
	}

	file := &deployedFile{content: acdsl.StripProjection(path, content), mode: info.Mode().Perm(), path: rel}
	return file, nil
}

// siblingFiles skips the model file and the dirs of skippedSiblingDirs, the agents dir included.
func siblingFiles(leafDir string) ([]*deployedFile, error) {
	modelFile := skillgraph.ModelFile(filepath.Base(leafDir))
	var files []*deployedFile
	err := filepath.WalkDir(leafDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(leafDir, path)
		if err != nil {
			return fmt.Errorf("siblingFiles: %w", err)
		}

		isSourceOnly := slices.Contains(skippedSiblingDirs, rel)
		if entry.IsDir() && isSourceOnly {
			return fs.SkipDir
		}
		if rel == modelFile || !entry.Type().IsRegular() {
			return nil
		}

		file, err := readSibling(path, filepath.ToSlash(rel))
		if err != nil {
			return err
		}

		files = append(files, file)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("siblingFiles: %w", err)
	}
	return files, nil
}

// versionNote is the report suffix naming a deploy's version change.
func versionNote(newVersion, oldVersion string) string {
	switch {
	case newVersion == "":
		return ""
	case oldVersion == "":
		return " (new v" + newVersion + ")"
	case oldVersion == newVersion:
		return " (unchanged v" + newVersion + ")"
	}
	return " (bumped v" + oldVersion + " -> v" + newVersion + ")"
}

// SetOverrides sets or clears (empty value) the skillOverrides of leaves in a user's settings file; the Skills page's visibility action is its caller (master B16).
func SetOverrides(leaves []string, path, value string, keep *trash.Store) error {
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	settings, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("SetOverrides: %w", err)
	}

	overrides, err := settings.SkillOverrides()
	if err != nil {
		return fmt.Errorf("SetOverrides: %w", err)
	}
	if overrides == nil {
		overrides = map[string]string{}
	}
	for _, leaf := range leaves {
		overrides[leaf] = value
		if value == "" {
			delete(overrides, leaf)
		}
	}
	if err := settings.SetSkillOverrides(overrides); err != nil {
		return fmt.Errorf("SetOverrides: %w", err)
	}

	if err := config.Save(path, settings, keep); err != nil {
		return fmt.Errorf("SetOverrides: %w", err)
	}
	return nil
}

// Package repos manages the repo registry and runs the worktree lifecycle
// scripts and git operations against registered repositories. Repo paths come
// exclusively from the registry file — request input only ever selects by
// name (concept security boundary).
package repos

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/reach"
	"github.com/kevinhorst/smine/internal/shell"
)

var namePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

const labelMaxLength = 80

type Repo struct {
	Label string `json:"label,omitempty"`
	Name  string `json:"name"`
	Path  string `json:"path"`
}

func (r *Repo) Validate() error {
	// Label (display-only, never the URL segment — Name stays the key)
	if len(r.Label) > labelMaxLength {
		return fmt.Errorf("Repo.Validate: Invalid field Label: Longer than %d chars", labelMaxLength)
	}

	// Name (also the URL segment; add/delete/choose-folder/prune-jetbrains are
	// shadowed for POST by the literal /repos/* routes)
	if !namePattern.MatchString(r.Name) {
		return fmt.Errorf("Repo.Validate: Invalid field Name: %q", r.Name)
	}
	// The name is also the repo's mining folder under sessions/ — the two
	// structural folder names cannot be repo identities.
	if r.Name == "default" || r.Name == "archived" {
		return fmt.Errorf("Repo.Validate: Invalid field Name: %q is reserved", r.Name)
	}

	// Path
	if !filepath.IsAbs(r.Path) {
		return fmt.Errorf("Repo.Validate: Invalid field Path: Must be absolute: %q", r.Path)
	}

	return nil
}

type Registry struct {
	path string

	// mu guards the loaded state below for concurrent request reads vs. reload.
	mu            sync.RWMutex
	loadedModTime time.Time
	loadedSize    int64
	repos         []Repo
}

func NewRegistry(path string) *Registry {
	return &Registry{path: path}
}

// apply swaps in a snapshot's content and file identity. Callers hold r.mu.
func (r *Registry) apply(snapshot *registrySnapshot) {
	r.loadedModTime = snapshot.modTime
	r.loadedSize = snapshot.size
	r.repos = snapshot.repos
}

// refreshIfChanged reloads when the file identity (mtime+size) differs from
// the loaded state — reads track external edits and other writers without a
// restart. A failed re-read keeps the last good state and logs; only startup
// fails loudly.
func (r *Registry) refreshIfChanged() {
	r.mu.RLock()
	modTime, size := r.loadedModTime, r.loadedSize
	r.mu.RUnlock()

	info, err := os.Stat(r.path)
	isMissingAndUnloaded := os.IsNotExist(err) && size == 0 && modTime.IsZero()
	if isMissingAndUnloaded {
		return
	}
	if err == nil && info.ModTime().Equal(modTime) && info.Size() == size {
		return
	}

	if err := r.Reload(); err != nil {
		log.Printf("Registry.refreshIfChanged: Keeping last good state: %v", err)
	}
}

// save writes the registry file atomically (tmp + rename, config.Save
// pattern) and records the written state + file identity. Callers hold r.mu.
func (r *Registry) save(repos []Repo) error {
	data, err := json.MarshalIndent(registryFile{Repos: repos}, "", "  ")
	if err != nil {
		return fmt.Errorf("save: %w", err)
	}
	data = append(data, '\n')

	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	if err := fsx.ReplaceFile(tmp, r.path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("save: %w", err)
	}

	info, err := os.Stat(r.path)
	if err != nil {
		return fmt.Errorf("save: Failed to stat after write: %w", err)
	}
	r.apply(&registrySnapshot{modTime: info.ModTime(), repos: repos, size: info.Size()})
	return nil
}

// Add validates the repo, rejects duplicate names, and persists — against a
// fresh read of the file, so a stale in-memory slice is never written back.
func (r *Registry) Add(repo Repo) error {
	if err := repo.Validate(); err != nil {
		return fmt.Errorf("Registry.Add: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	snapshot, err := readRegistrySnapshot(r.path)
	if err != nil {
		return fmt.Errorf("Registry.Add: %w", err)
	}
	for i := range snapshot.repos {
		if snapshot.repos[i].Name == repo.Name {
			return fmt.Errorf("Registry.Add: Duplicate repo name %s", repo.Name)
		}
	}

	repos := append(snapshot.repos, repo)
	if err := r.save(repos); err != nil {
		return fmt.Errorf("Registry.Add: %w", err)
	}
	return nil
}

func (r *Registry) Find(name string) (*Repo, bool) {
	r.refreshIfChanged()
	r.mu.RLock()
	defer r.mu.RUnlock()
	for i := range r.repos {
		if r.repos[i].Name == name {
			repo := r.repos[i]
			return &repo, true
		}
	}
	return nil, false
}

// Reload re-reads the registry file and swaps it in.
func (r *Registry) Reload() error {
	snapshot, err := readRegistrySnapshot(r.path)
	if err != nil {
		return fmt.Errorf("Registry.Reload: %w", err)
	}

	r.mu.Lock()
	r.apply(snapshot)
	r.mu.Unlock()
	return nil
}

// Remove deletes the named repo and persists — like Add, against a fresh
// read of the file.
func (r *Registry) Remove(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	snapshot, err := readRegistrySnapshot(r.path)
	if err != nil {
		return fmt.Errorf("Registry.Remove: %w", err)
	}
	index := slices.IndexFunc(snapshot.repos, func(repo Repo) bool { return repo.Name == name })
	if index < 0 {
		return fmt.Errorf("Registry.Remove: Unknown repo name %s", name)
	}

	repos := slices.Delete(snapshot.repos, index, index+1)
	if err := r.save(repos); err != nil {
		return fmt.Errorf("Registry.Remove: %w", err)
	}
	return nil
}

func (r *Registry) Repos() []Repo {
	r.refreshIfChanged()
	r.mu.RLock()
	defer r.mu.RUnlock()
	repos := make([]Repo, len(r.repos))
	copy(repos, r.repos)
	return repos
}

// registrySnapshot is one parsed read of the registry file plus the file
// identity (mtime+size) it was read at; the zero value is a missing file.
type registrySnapshot struct {
	modTime time.Time
	repos   []Repo
	size    int64
}

// readRegistrySnapshot reads and validates the registry file. A missing file
// is an empty registry (bootstrapping, sessions-store pattern); an invalid
// entry fails the read loudly — the registry is a small user-authored file.
// Stat runs BEFORE the read, so a write racing the read is re-detected on
// the next stat rather than missed.
func readRegistrySnapshot(path string) (*registrySnapshot, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &registrySnapshot{}, nil
		}
		return nil, fmt.Errorf("readRegistrySnapshot: Failed to stat %s: %w", path, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("readRegistrySnapshot: Failed to read %s: %w", path, err)
	}

	var file registryFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("readRegistrySnapshot: Failed to parse %s: %w", path, err)
	}

	seen := make(map[string]bool)
	for _, repo := range file.Repos {
		if err := repo.Validate(); err != nil {
			return nil, fmt.Errorf("readRegistrySnapshot: %w", err)
		}
		if seen[repo.Name] {
			return nil, fmt.Errorf("readRegistrySnapshot: Duplicate repo name %s", repo.Name)
		}
		seen[repo.Name] = true
	}

	snapshot := &registrySnapshot{modTime: info.ModTime(), repos: file.Repos, size: info.Size()}
	return snapshot, nil
}

type registryFile struct {
	Repos []Repo `json:"repos"`
}

// Context states — a repo's relationship to the context system: the source
// repo authors it, a deployed target consumes a synced copy.
const (
	ContextNone     = "none"
	ContextSource   = "source"
	ContextDeployed = "deployed"
)

// ContextPresence describes what DetectContext found: the state, the
// context folder, the deployed/authored language guides, and whether the
// acdsl gate slice is active.
type ContextPresence struct {
	Acdsl      bool     // deployed: deploy.acdsl; source: acdsl/registry.json exists
	ContextDir string   // deployed: deploy.contextDir; source: "context"
	Langs      []string // deployed: deploy.langs; source: rules/<lang>.md guides minus plan, commits
	State      string   // ContextNone | ContextSource | ContextDeployed
}

// contextArtifactGuides are the always-synced non-language guides.
var contextArtifactGuides = map[string]bool{"plan": true, "commits": true}

// DetectContext probes <repo>/context/context.json (source: generated
// entries, no deploy key) then <repo>/docs/context.json (deployed: carries
// the deploy section — docs is sync's own probe default; other contextDirs
// are undetectable without it). Anything else is ContextNone.
func DetectContext(repoPath string) ContextPresence {
	if deploy, exists := readContextDeploy(filepath.Join(repoPath, "context", ContextFileName)); exists && deploy == nil {
		presence := ContextPresence{State: ContextSource, ContextDir: "context"}
		if info, err := os.Stat(filepath.Join(repoPath, "acdsl", "registry.json")); err == nil && !info.IsDir() {
			presence.Acdsl = true
		}
		guides, err := os.ReadDir(filepath.Join(repoPath, "context", "rules"))
		if err != nil {
			return presence
		}
		for _, guide := range guides {
			name := strings.TrimSuffix(guide.Name(), ".md")
			if guide.IsDir() || name == guide.Name() || contextArtifactGuides[name] {
				continue
			}
			presence.Langs = append(presence.Langs, name)
		}
		return presence
	}
	if deploy, exists := readContextDeploy(filepath.Join(repoPath, "docs", ContextFileName)); exists && deploy != nil {
		contextDir := deploy.ContextDir
		if contextDir == "" {
			contextDir = "docs"
		}
		return ContextPresence{Acdsl: deploy.Acdsl, ContextDir: contextDir, Langs: deploy.Langs, State: ContextDeployed}
	}
	return ContextPresence{State: ContextNone}
}

// ContextFileName is the generated context file probed for detection.
const ContextFileName = "context.json"

// contextDeploy is the deploy section of a deployed target's context.json.
type contextDeploy struct {
	Acdsl      bool     `json:"acdsl"`
	ContextDir string   `json:"contextDir"`
	Langs      []string `json:"langs"`
}

// readContextDeploy reads a context.json and reports whether the file
// exists and, when it does, its deploy section (nil for the source form).
func readContextDeploy(path string) (deploy *contextDeploy, exists bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var parsed struct {
		Deploy *contextDeploy `json:"deploy"`
	}
	if json.Unmarshal(raw, &parsed) != nil {
		return nil, false
	}
	return parsed.Deploy, true
}

// contextIndex is the slice of a generated context.json read for coverage
// checks: deployable entries plus the scope→language aspects.
type contextIndex struct {
	Aspects []contextAspect `json:"aspects"`
	Entries []contextEntry  `json:"entries"`
}

type contextAspect struct {
	Class string `json:"class"`
	Lang  string `json:"lang"`
	Name  string `json:"name"`
}

type contextEntry struct {
	Id    string `json:"id"`
	Reach string `json:"reach"`
	Scope string `json:"scope"`
}

// ContextCoverage reports how much of the source context reaches a deployed
// target: Expected counts the source entries whose reach covers the repo
// (language-bound scopes only when the target synced that language), Missing
// lists the expected ids absent from the target's deployed index.
type ContextCoverage struct {
	Expected int
	Missing  []string
}

// DetectContextCoverage compares the source context index against the
// target's deployed one. Non-deployed states carry no coverage and return
// the zero value.
func DetectContextCoverage(sourceIndexPath, repoName, repoPath string, presence ContextPresence) (ContextCoverage, error) {
	var coverage ContextCoverage
	if presence.State != ContextDeployed {
		return coverage, nil
	}
	source, err := readContextIndex(sourceIndexPath)
	if err != nil {
		return coverage, fmt.Errorf("DetectContextCoverage: %w", err)
	}
	deployed, err := readContextIndex(filepath.Join(repoPath, presence.ContextDir, ContextFileName))
	if err != nil {
		return coverage, fmt.Errorf("DetectContextCoverage: %w", err)
	}

	langByScope := map[string]string{}
	for _, aspect := range source.Aspects {
		if aspect.Class == "scope" && aspect.Lang != "" {
			langByScope[aspect.Name] = aspect.Lang
		}
	}
	syncedLangs := map[string]bool{}
	for _, lang := range presence.Langs {
		syncedLangs[lang] = true
	}
	deployedIds := map[string]bool{}
	for _, entry := range deployed.Entries {
		deployedIds[entry.Id] = true
	}

	for _, entry := range source.Entries {
		if !reach.DeploysTo(entry.Reach, repoName) {
			continue
		}
		if lang, isLangBound := langByScope[entry.Scope]; isLangBound && !syncedLangs[lang] {
			continue
		}
		coverage.Expected++
		if !deployedIds[entry.Id] {
			coverage.Missing = append(coverage.Missing, entry.Id)
		}
	}
	return coverage, nil
}

func readContextIndex(path string) (*contextIndex, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("readContextIndex: %w", err)
	}
	index := &contextIndex{}
	if err := json.Unmarshal(raw, index); err != nil {
		return nil, fmt.Errorf("readContextIndex: %s: %w", path, err)
	}
	return index, nil
}

// CountPoolWorktrees counts real agent worktrees under <repo>/.claude/worktrees:
// subdirs owning a .git entry. Hollow pool dirs (no .git) and the pool-guard
// decoy (.git file at the pool root) never count. Stat-cheap — the overview
// tile relies on that (no git execution there, D3/D8); the repos page uses
// WorktreeCounts for branch-kind counts instead.
func CountPoolWorktrees(repoPath string) int {
	pool := filepath.Join(repoPath, ".claude", "worktrees")
	entries, err := os.ReadDir(pool)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := os.Lstat(filepath.Join(pool, entry.Name(), ".git")); err == nil {
			count++
		}
	}
	return count
}

// Fingerprint captures the state the worktree status table derives from:
// agent branch tips plus the checked-out worktree set. Two fast git reads —
// the status cache serves its entry only while this matches; dirty counts
// are invisible to it (accepted staleness, Reload covers them).
func Fingerprint(ctx context.Context, repoPath string) (string, error) {
	refs, err := shell.Run(ctx, repoPath, "git", "for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/claude/", "refs/heads/claude-routines/")
	if err != nil {
		return "", fmt.Errorf("Fingerprint: %s: %w", strings.TrimSpace(refs), err)
	}

	worktrees, err := shell.Run(ctx, repoPath, "git", "worktree", "list", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("Fingerprint: %s: %w", strings.TrimSpace(worktrees), err)
	}
	return refs + "\n" + worktrees, nil
}

// WorktreeKindCount is one branch-prefix bucket of a repo's agent worktrees
// (label = prefix minus the trailing slash: "claude", "claude-routines").
type WorktreeKindCount struct {
	Count int
	Label string
}

// WorktreeCounts counts the repo's checked-out agent worktrees per branch
// prefix (claude/, claude-routines/), read from git worktree list — routine
// worktrees live outside the repo, so a pool-directory scan cannot see them.
// Zero-count prefixes are omitted; errors degrade to nil.
func WorktreeCounts(ctx context.Context, repoPath string, branchPrefixes []string) []WorktreeKindCount {
	output, err := shell.Run(ctx, repoPath, "git", "worktree", "list", "--porcelain")
	if err != nil {
		return nil
	}
	counts := make([]int, len(branchPrefixes))
	for _, line := range strings.Split(output, "\n") {
		branch, isBranchLine := strings.CutPrefix(line, "branch refs/heads/")
		if !isBranchLine {
			continue
		}
		for index, prefix := range branchPrefixes {
			if strings.HasPrefix(branch, prefix) {
				counts[index]++
				break
			}
		}
	}
	var result []WorktreeKindCount
	for index, prefix := range branchPrefixes {
		if counts[index] == 0 {
			continue
		}
		bucket := WorktreeKindCount{Count: counts[index], Label: strings.TrimSuffix(prefix, "/")}
		result = append(result, bucket)
	}
	return result
}

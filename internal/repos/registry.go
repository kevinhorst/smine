// Package repos manages the repo registry and runs the worktree lifecycle
// scripts and git operations against registered repositories. Repo paths come
// exclusively from the registry file — request input only ever selects by
// name (concept security boundary).
package repos

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/reach"
	"github.com/kevinhorst/smine/internal/shell"
)

var namePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type Repo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (r *Repo) Validate() error {
	// Name (also the URL segment; add/delete/choose-folder/prune-jetbrains are
	// shadowed for POST by the literal /repos/* routes)
	if !namePattern.MatchString(r.Name) {
		return fmt.Errorf("Repo.Validate: Invalid field Name: %q", r.Name)
	}

	// Path
	if !filepath.IsAbs(r.Path) {
		return fmt.Errorf("Repo.Validate: Invalid field Path: Must be absolute: %q", r.Path)
	}

	return nil
}

type Registry struct {
	path string

	// mu guards repos for concurrent request reads vs. Reload.
	mu    sync.RWMutex
	repos []Repo
}

func NewRegistry(path string) *Registry {
	return &Registry{path: path}
}

func (r *Registry) swap(repos []Repo) {
	r.mu.Lock()
	r.repos = repos
	r.mu.Unlock()
}

func (r *Registry) Find(name string) (*Repo, bool) {
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

// Reload re-reads the registry file. A missing file is an empty registry
// (bootstrapping, sessions-store pattern); an invalid entry fails the reload
// loudly — the registry is a small user-authored file.
func (r *Registry) Reload() error {
	data, err := os.ReadFile(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			r.swap(nil)
			return nil
		}
		return fmt.Errorf("Registry.Reload: Failed to read %s: %w", r.path, err)
	}

	var file registryFile
	if err := json.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("Registry.Reload: Failed to parse %s: %w", r.path, err)
	}

	seen := make(map[string]bool)
	for _, repo := range file.Repos {
		if err := repo.Validate(); err != nil {
			return fmt.Errorf("Registry.Reload: %w", err)
		}
		if seen[repo.Name] {
			return fmt.Errorf("Registry.Reload: Duplicate repo name %s", repo.Name)
		}
		seen[repo.Name] = true
	}

	r.swap(file.Repos)
	return nil
}

// Add validates the repo, rejects duplicate names, persists the extended
// registry, and only then swaps it in — a failed save changes nothing.
func (r *Registry) Add(repo Repo) error {
	if err := repo.Validate(); err != nil {
		return fmt.Errorf("Registry.Add: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.repos {
		if r.repos[i].Name == repo.Name {
			return fmt.Errorf("Registry.Add: Duplicate repo name %s", repo.Name)
		}
	}

	repos := append(slices.Clone(r.repos), repo)
	if err := r.save(repos); err != nil {
		return fmt.Errorf("Registry.Add: %w", err)
	}
	r.repos = repos
	return nil
}

// Remove deletes the named repo, persists, and swaps in on success.
func (r *Registry) Remove(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	index := slices.IndexFunc(r.repos, func(repo Repo) bool { return repo.Name == name })
	if index < 0 {
		return fmt.Errorf("Registry.Remove: Unknown repo name %s", name)
	}

	repos := slices.Delete(slices.Clone(r.repos), index, index+1)
	if err := r.save(repos); err != nil {
		return fmt.Errorf("Registry.Remove: %w", err)
	}
	r.repos = repos
	return nil
}

// save writes the registry file atomically (tmp + rename, config.Save
// pattern). Callers hold r.mu.
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
	return nil
}

func (r *Registry) Repos() []Repo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	repos := make([]Repo, len(r.repos))
	copy(repos, r.repos)
	return repos
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
// CountWorktrees for branch-kind counts instead.
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

// CountWorktrees counts the repo's checked-out agent worktrees whose branch
// sits under one of the given prefixes (claude/, claude-routines/), read from
// git worktree list — routine worktrees live outside the repo, so a
// pool-directory scan cannot see them. Errors degrade to 0.
func CountWorktrees(ctx context.Context, repoPath string, branchPrefixes []string) int {
	output, err := shell.Run(ctx, repoPath, "git", "worktree", "list", "--porcelain")
	if err != nil {
		return 0
	}
	count := 0
	for _, line := range strings.Split(output, "\n") {
		branch, isBranchLine := strings.CutPrefix(line, "branch refs/heads/")
		if !isBranchLine {
			continue
		}
		for _, prefix := range branchPrefixes {
			if strings.HasPrefix(branch, prefix) {
				count++
				break
			}
		}
	}
	return count
}

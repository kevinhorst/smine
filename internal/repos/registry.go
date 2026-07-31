// Package repos manages the repo registry and runs the worktree lifecycle
// scripts and git operations against registered repositories. Repo paths come
// exclusively from the registry file — request input only ever selects by
// name (concept security boundary).
package repos

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sync"
)

var namePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type Presence struct {
	HasDir     bool
	HasRootDoc bool
}

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
	if err := os.Rename(tmp, r.path); err != nil {
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

func detectPresence(markerDir, markerDoc, repoPath string) Presence {
	presence := Presence{}
	if info, err := os.Stat(filepath.Join(repoPath, markerDoc)); err == nil && !info.IsDir() {
		presence.HasRootDoc = true
	}
	if info, err := os.Stat(filepath.Join(repoPath, markerDir)); err == nil && info.IsDir() {
		presence.HasDir = true
	}
	return presence
}

// DetectClaude reports CLAUDE.md / .claude/ presence for a repo path.
func DetectClaude(repoPath string) Presence {
	return detectPresence(".claude", "CLAUDE.md", repoPath)
}

// DetectCodex reports AGENTS.md / .codex/ presence for a repo path.
func DetectCodex(repoPath string) Presence {
	return detectPresence(".codex", "AGENTS.md", repoPath)
}

// ContextPresence reports the context/ dir: general guidance presence plus
// the language-specific subdirs (every subdir except general).
type ContextPresence struct {
	HasGeneral bool
	Languages  []string
}

// DetectContext scans <repo>/context; a missing dir is an empty presence.
// ReadDir order is sorted, so Languages arrives sorted.
func DetectContext(repoPath string) ContextPresence {
	presence := ContextPresence{}
	entries, err := os.ReadDir(filepath.Join(repoPath, "context"))
	if err != nil {
		return presence
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if entry.Name() == "general" {
			presence.HasGeneral = true
			continue
		}
		presence.Languages = append(presence.Languages, entry.Name())
	}
	return presence
}

// CountWorktrees counts real agent worktrees under <repo>/.claude/worktrees:
// subdirs owning a .git entry. Hollow pool dirs (no .git) and the pool-guard
// decoy (.git file at the pool root) never count.
func CountWorktrees(repoPath string) int {
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

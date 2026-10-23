// Package repos manages the repo registry and runs git and worktree operations against registered repos.
package repos

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/reach"
	"github.com/kevinhorst/smine/internal/shell"
	"github.com/kevinhorst/smine/internal/trash"
)

var namePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// envKeyPattern is a rendered skill variable name, the only key an Env row may hold.
var envKeyPattern = regexp.MustCompile(`^SKILL_[A-Z0-9]+_[A-Z][A-Z0-9_]*$`)

const labelMaxLength = 80

type Repo struct {
	Env   map[string]string `json:"env,omitempty"`
	Label string            `json:"label,omitempty"`
	Name  string            `json:"name"`
	Path  string            `json:"path"`
}

func (r *Repo) Validate() error {
	// Label (display-only, never the URL segment — Name stays the key)
	if len(r.Label) > labelMaxLength {
		return fmt.Errorf("Repo.Validate: Invalid field Label: Longer than %d chars", labelMaxLength)
	}

	// Name (also the URL segment; literal /repos/* POST routes shadow a few names)
	if !namePattern.MatchString(r.Name) {
		return fmt.Errorf("Repo.Validate: Invalid field Name: %q", r.Name)
	}
	// The name is also the mining folder under sessions/, so its structural folder names are reserved.
	if r.Name == "unknown" || r.Name == "archived" {
		return fmt.Errorf("Repo.Validate: Invalid field Name: %q is reserved", r.Name)
	}
	// A repo named like a reach token would shadow the reach grammar at sync time.
	if r.Name == reach.Always || r.Name == reach.Global || r.Name == reach.None {
		return fmt.Errorf("Repo.Validate: Invalid field Name: %q is reserved", r.Name)
	}

	// Path
	if !filepath.IsAbs(r.Path) {
		return fmt.Errorf("Repo.Validate: Invalid field Path: Must be absolute: %q", r.Path)
	}

	// Env (the repo's skill values; the kind rules run where the catalog is known)
	envKeys := slices.Sorted(maps.Keys(r.Env))
	for _, key := range envKeys {
		if !envKeyPattern.MatchString(key) {
			return fmt.Errorf("Repo.Validate: Invalid field Env: Key %q is no skill variable name", key)
		}
		if strings.TrimSpace(r.Env[key]) == "" {
			return fmt.Errorf("Repo.Validate: Invalid field Env: %s is empty — delete the key instead", key)
		}
	}

	return nil
}

type Registry struct {
	path string
	// trash keeps every previous repos.json and journals a removed row; built from the file's own dir, the state dir.
	trash *trash.Store

	// mu guards the loaded state below for concurrent request reads vs. reload.
	mu            sync.RWMutex
	loadedModTime time.Time
	loadedSize    int64
	repos         []Repo
}

func NewRegistry(path string) *Registry {
	return &Registry{path: path, trash: trash.New(filepath.Dir(path), 0)}
}

// apply swaps in a snapshot's content and file identity. Callers hold r.mu.
func (r *Registry) apply(snapshot *registrySnapshot) {
	r.loadedModTime = snapshot.modTime
	r.loadedSize = snapshot.size
	r.repos = snapshot.repos
}

// refreshIfChanged keeps the last good state on a failed re-read; only startup fails loudly.
func (r *Registry) refreshIfChanged() {
	r.mu.RLock()
	modTime, size := r.loadedModTime, r.loadedSize
	r.mu.RUnlock()

	info, statErr := os.Stat(r.path)
	isMissingAndUnloaded := os.IsNotExist(statErr) && size == 0 && modTime.IsZero()
	if isMissingAndUnloaded {
		return
	}
	if statErr == nil && info.ModTime().Equal(modTime) && info.Size() == size {
		return
	}

	if err := r.Reload(); err != nil {
		log.Printf("Registry.refreshIfChanged: Keeping last good state: %v", err)
		return
	}
	// A file removed after a successful load is a real transition, but never a silent one.
	if os.IsNotExist(statErr) {
		log.Printf("Registry.refreshIfChanged: %s removed — registry is now empty", r.path)
	}
}

// save expects the caller to hold r.mu.
func (r *Registry) save(repos []Repo) error {
	data, err := json.MarshalIndent(registryFile{Repos: repos}, "", "  ")
	if err != nil {
		return fmt.Errorf("save: %w", err)
	}
	data = append(data, '\n')

	entry := &trash.Entry{Action: "repos save", Reason: "repos.json rewritten"}
	if _, err := r.trash.Keep(r.path, entry); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	if err := fsx.WriteFileAtomic(data, r.path, 0o644); err != nil {
		return fmt.Errorf("save: %w", err)
	}

	info, err := os.Stat(r.path)
	if err != nil {
		return fmt.Errorf("save: Failed to stat after write: %w", err)
	}
	r.apply(&registrySnapshot{modTime: info.ModTime(), repos: repos, size: info.Size()})
	return nil
}

// Add persists against a fresh read of the file, so a stale in-memory slice is never written back.
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

// FindByPath returns the row whose Path equals the cleaned absolute path.
func (r *Registry) FindByPath(path string) (*Repo, bool) {
	r.refreshIfChanged()
	r.mu.RLock()
	defer r.mu.RUnlock()
	cleaned := filepath.Clean(path)
	for i := range r.repos {
		if filepath.Clean(r.repos[i].Path) == cleaned {
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

// Remove persists against a fresh read of the file, like Add.
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

	removed := snapshot.repos[index]
	envKeys := slices.Sorted(maps.Keys(removed.Env))
	entry := &trash.Entry{
		Action: "repos delete",
		Kind:   trash.KindRemoved,
		Path:   r.path + "#" + name,
		Reason: "row removed; env keys " + strings.Join(envKeys, ", "),
	}
	if err := r.trash.Note(entry); err != nil {
		return fmt.Errorf("Registry.Remove: %w", err)
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

// Update replaces the row named repo.Name against a fresh read of the file, like Add.
func (r *Registry) Update(repo Repo) error {
	if err := repo.Validate(); err != nil {
		return fmt.Errorf("Registry.Update: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	snapshot, err := readRegistrySnapshot(r.path)
	if err != nil {
		return fmt.Errorf("Registry.Update: %w", err)
	}
	index := slices.IndexFunc(snapshot.repos, func(row Repo) bool { return row.Name == repo.Name })
	if index < 0 {
		return fmt.Errorf("Registry.Update: Unknown repo name %s", repo.Name)
	}

	repos := slices.Clone(snapshot.repos)
	repos[index] = repo
	if err := r.save(repos); err != nil {
		return fmt.Errorf("Registry.Update: %w", err)
	}
	return nil
}

// registrySnapshot's zero value is a missing file.
type registrySnapshot struct {
	modTime time.Time
	repos   []Repo
	size    int64
}

// readRegistrySnapshot stats before reading, so a write racing the read is re-detected on the next stat.
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

const (
	ContextNone     = "none"
	ContextSource   = "source"
	ContextDeployed = "deployed"
)

type ContextPresence struct {
	Acdsl      bool     // deployed: deploy.acdsl; source: acdsl/registry.json exists
	ContextDir string   // deployed: deploy.contextDir; source: "context"
	IndexPath  string   // deployed: the repo copy when written, else the build record; source: ""
	IsWritten  bool     // deployed: the record says write to repo and the repo copy exists
	Langs      []string // deployed: deploy.langs; source: rules/<lang>.md guides minus plan, commits
	State      string   // ContextNone | ContextSource | ContextDeployed
}

// contextArtifactGuides are the always-synced non-language guides.
var contextArtifactGuides = map[string]bool{"plan": true, "commits": true}

// DetectContext classifies a repo: source when it authors the context tree, deployed when its build record exists, else none.
func DetectContext(repoPath string, recordPath string) ContextPresence {
	sourceDir := filepath.Join(repoPath, contextdocs.SourceContextDir)
	if isSourceContext(sourceDir) {
		return detectSourceContext(repoPath, sourceDir)
	}
	record, err := contextdocs.LoadContextFile(recordPath)
	if err != nil || record.Deploy == nil {
		return ContextPresence{State: ContextNone}
	}

	deploy := record.Deploy
	presence := ContextPresence{Acdsl: deploy.Acdsl, ContextDir: deploy.ContextDir, IndexPath: recordPath, Langs: deploy.Langs, State: ContextDeployed}
	repoIndex := filepath.Join(repoPath, deploy.ContextDir, contextdocs.ContextFileName)
	if deploy.WriteRepo && isFile(repoIndex) {
		presence.IndexPath = repoIndex
		presence.IsWritten = true
	}
	return presence
}

func detectSourceContext(repoPath string, sourceDir string) ContextPresence {
	presence := ContextPresence{ContextDir: contextdocs.SourceContextDir, State: ContextSource}
	if isFile(filepath.Join(repoPath, "acdsl", "registry.json")) {
		presence.Acdsl = true
	}
	guides, err := os.ReadDir(filepath.Join(sourceDir, "rules"))
	if err != nil {
		return presence
	}
	for _, guide := range guides {
		name, isLang := langGuideName(guide)
		if !isLang {
			continue
		}
		presence.Langs = append(presence.Langs, name)
	}
	return presence
}

// A shipped aspects.json beside a deployed index never makes a target a source repo.
func isSourceContext(sourceDir string) bool {
	if !isFile(filepath.Join(sourceDir, contextdocs.AspectsFileName)) {
		return false
	}
	return !contextdocs.IsDeployedIndex(filepath.Join(sourceDir, contextdocs.ContextFileName))
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// ContextFileName is the generated context file probed for deployed-target detection.
const ContextFileName = "context.json"

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

// ContextCoverage counts language-bound entries only when the target synced that language.
type ContextCoverage struct {
	Expected int
	Missing  []string
}

func DetectContextCoverage(sourceIndexPath string, repoName string, presence ContextPresence) (ContextCoverage, error) {
	var coverage ContextCoverage
	if presence.State != ContextDeployed {
		return coverage, nil
	}
	source, err := readContextIndex(sourceIndexPath)
	if err != nil {
		return coverage, fmt.Errorf("DetectContextCoverage: %w", err)
	}
	deployed, err := readContextIndex(presence.IndexPath)
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

// CountPoolWorktrees stays stat-only: the overview tile runs no git.
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

// Fingerprint ignores dirty counts on purpose; Reload covers that staleness.
func Fingerprint(ctx context.Context, repoPath string) (string, error) {
	output, err := shell.Run(ctx, repoPath, "git", "for-each-ref", "--format=%(refname) %(objectname)", "refs/heads/")
	if err != nil {
		return "", fmt.Errorf("Fingerprint: %s: %w", strings.TrimSpace(output), err)
	}

	var branchRefs []string
	for line := range strings.Lines(output) {
		_, isHead := strings.CutPrefix(strings.TrimSpace(line), "refs/heads/")
		if isHead {
			branchRefs = append(branchRefs, strings.TrimSpace(line))
		}
	}
	refs := strings.Join(branchRefs, "\n")

	worktrees, err := shell.Run(ctx, repoPath, "git", "worktree", "list", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("Fingerprint: %s: %w", strings.TrimSpace(worktrees), err)
	}
	return refs + "\n" + worktrees, nil
}

type WorktreeKindCount struct {
	Count int
	Label string
}

// WorktreeCounts reads git worktree list: routine worktrees live outside the repo's pool directory.
func WorktreeCounts(ctx context.Context, repoPath string) []WorktreeKindCount {
	output, err := shell.Run(ctx, repoPath, "git", "worktree", "list", "--porcelain")
	if err != nil {
		return nil
	}
	countsByKind := make(map[string]int)
	var currentPath string
	for _, line := range strings.Split(output, "\n") {
		if path, isPathLine := strings.CutPrefix(line, "worktree "); isPathLine {
			currentPath = path
			continue
		}
		if line == "detached" {
			countsByKind[detachedKind(currentPath)]++
			continue
		}
		branch, isBranchLine := strings.CutPrefix(line, "branch refs/heads/")
		if !isBranchLine {
			continue
		}
		kind := AgentBranchKind(branch)
		if kind == "" {
			continue
		}
		countsByKind[kind]++
	}
	var counts []WorktreeKindCount
	for _, kind := range slices.Sorted(maps.Keys(countsByKind)) {
		bucket := WorktreeKindCount{Count: countsByKind[kind], Label: kind}
		counts = append(counts, bucket)
	}
	return counts
}

func detachedKind(path string) string {
	if strings.Contains(path, "/.codex/worktrees/") {
		return "codex"
	}
	return "detached"
}

// langGuideName returns a rules dir entry's lang; false for dirs, non-guides, artifact guides and `<lang>_<part>` parts.
func langGuideName(guide fs.DirEntry) (string, bool) {
	name := strings.TrimSuffix(guide.Name(), ".md")
	switch {
	case guide.IsDir(), name == guide.Name():
		return "", false
	case contextArtifactGuides[name], strings.Contains(name, "_"):
		return "", false
	}
	return name, true
}

package sessions

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kevinhorst/smine/internal/contextrecord"
	"github.com/kevinhorst/smine/internal/shell"
	"github.com/kevinhorst/smine/internal/trash"
)

const archivedDirName = "archived"

type ArchivedFolder struct {
	Name     string
	Sessions int
}

// ContextRecord keeps every field optional so an older record re-checks unchanged.
type ContextRecord struct {
	AcdslRules      []contextrecord.FileRules       `json:"acdsl_rules,omitempty"`
	Agent           string                          `json:"agent,omitempty"`
	Agents          []contextrecord.Agent           `json:"agents,omitempty"`
	Cwd             string                          `json:"cwd,omitempty"`
	GitBranch       string                          `json:"git_branch,omitempty"`
	Honored         int                             `json:"honored,omitempty"`
	Injected        *contextrecord.InjectedContext  `json:"injected,omitempty"`
	PackReads       []string                        `json:"pack_reads,omitempty"`
	PlanRules       []string                        `json:"plan_rules,omitempty"`
	SessionId       string                          `json:"session_id,omitempty"`
	SubagentContext []contextrecord.SubagentContext `json:"subagent_context,omitempty"`
	Totals          *contextrecord.Totals           `json:"totals,omitempty"`
	TouchedFiles    *TouchedFiles                   `json:"touched_files,omitempty"`
}

// NewContextRecord carries an extracted used-context record into a session record's context key.
func NewContextRecord(record *contextrecord.Record) *ContextRecord {
	injected := record.Injected
	totals := record.Totals
	contextRecord := &ContextRecord{
		AcdslRules:      record.AcdslRules,
		Agent:           record.Agent,
		Agents:          record.Agents,
		Cwd:             record.Cwd,
		GitBranch:       record.GitBranch,
		Injected:        &injected,
		PackReads:       record.PackReads,
		PlanRules:       record.PlanRules,
		SessionId:       record.SessionId,
		SubagentContext: record.SubagentContext,
		Totals:          &totals,
	}
	return contextRecord
}

type ContextUse struct {
	Channel string `json:"channel,omitempty"`
	Kind    string `json:"kind"`
	RuleId  string `json:"ruleId"`
}

type Cost struct {
	ActiveSeconds int `json:"activeSeconds"`
	Denials       int `json:"denials"`
	InputTokens   int `json:"inputTokens"`
	OutputTokens  int `json:"outputTokens"`
	Prompts       int `json:"prompts"`
	Turns         int `json:"turns"`
	WallSeconds   int `json:"wallSeconds"`
}

type Finding struct {
	ContextUse *ContextUse `json:"contextUse,omitempty"`
	Dimension  string      `json:"dimension"`
	Headline   string      `json:"headline,omitempty"`
	Quotes     []Quote     `json:"quotes,omitempty"`
	Snippets   []Snippet   `json:"snippets,omitempty"`
	Summary    string      `json:"summary"`
}

type Frustration struct {
	Quote   string `json:"quote"`
	Trigger string `json:"trigger,omitempty"`
}

type Invocation struct {
	LastActive   string
	Scope        string
	SessionId    string
	SessionTitle string
	Week         string
}

type Permission struct {
	Command  string `json:"command"`
	Decision string `json:"decision,omitempty"`
	Tool     string `json:"tool"`
}

type Permissions struct {
	Denied           []Permission `json:"denied,omitempty"`
	Granted          []Permission `json:"granted,omitempty"`
	TelemetryPresent bool         `json:"telemetry_present"`
}

type Positive struct {
	Quote   string `json:"quote"`
	Trigger string `json:"trigger,omitempty"`
}

type Quote struct {
	Text string `json:"text"`
	Turn int    `json:"turn"`
}

// MarshalJSON keeps a quote without a turn a bare string, so a format-1 record re-saves byte for byte.
func (q Quote) MarshalJSON() ([]byte, error) {
	if q.Turn == 0 {
		data, err := json.Marshal(q.Text)
		if err != nil {
			return nil, fmt.Errorf("Quote.MarshalJSON: %w", err)
		}
		return data, nil
	}

	type quoteObject Quote
	data, err := json.Marshal(quoteObject(q))
	if err != nil {
		return nil, fmt.Errorf("Quote.MarshalJSON: %w", err)
	}
	return data, nil
}

func (q *Quote) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &q.Text); err != nil {
			return fmt.Errorf("Quote.UnmarshalJSON: %w", err)
		}
		return nil
	}

	type quoteObject Quote
	var object quoteObject
	if err := json.Unmarshal(data, &object); err != nil {
		return fmt.Errorf("Quote.UnmarshalJSON: %w", err)
	}
	*q = Quote(object)
	return nil
}

type ScopeInfo struct {
	LoadErrors []string
	Name       string
	Weeks      []*Week
}

type SentimentPoint struct {
	Frustration int
	Positive    int
	Week        string
}

type Session struct {
	Id string `json:"id"`

	Agent         string         `json:"agent"`
	Context       *ContextRecord `json:"context,omitempty"`
	Cost          *Cost          `json:"cost,omitempty"`
	DataGaps      string         `json:"dataGaps,omitempty"`
	Findings      []Finding      `json:"findings"`
	Format        int            `json:"format,omitempty"`
	Frustration   []Frustration  `json:"frustration,omitempty"`
	InvokedSkills []string       `json:"invoked_skills,omitempty"`
	LastActive    string         `json:"lastActive"`
	Permissions   *Permissions   `json:"permissions,omitempty"`
	Positive      []Positive     `json:"positive,omitempty"`
	Repo          string         `json:"repo"`
	Signal        string         `json:"signal,omitempty"`
	SkipReason    string         `json:"skipReason,omitempty"`
	Skipped       bool           `json:"skipped,omitempty"`
	Synthesized   string         `json:"synthesized,omitempty"`
	Title         string         `json:"title"`
	Type          string         `json:"type,omitempty"`
	Week          string         `json:"week"`
}

type SessionRef struct {
	LastActive string
	Scope      string
	Week       string
}

type Snippet struct {
	Code   string `json:"code"`
	Kind   string `json:"kind"`
	Lang   string `json:"lang"`
	Source string `json:"source"`
}

type Store struct {
	dir string

	// fingerprint is every record file's path, size and mtime at the last load; Reload returns early while it holds.
	fingerprint string
	// mu guards byScope for concurrent request reads vs. Reload.
	mu      sync.RWMutex
	byScope map[string]*ScopeInfo
	trash   *trash.Store
}

func NewStore(dir string, store *trash.Store) *Store {
	return &Store{
		byScope: make(map[string]*ScopeInfo),
		dir:     dir,
		trash:   store,
	}
}

// commitPaths commits only the named paths, so a dirty checkout commits nothing else.
func (s *Store) commitPaths(ctx context.Context, subject string, paths ...string) error {
	args := append([]string{"commit", "-q", "-m", subject, "--"}, paths...)
	if _, err := s.git(ctx, args...); err != nil {
		return err
	}
	return nil
}

// git runs one git command in the store's checkout.
func (s *Store) git(ctx context.Context, args ...string) (string, error) {
	output, err := shell.Run(ctx, s.dir, "git", args...)
	if err != nil {
		return "", fmt.Errorf("Store.git: %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(output), err)
	}
	return output, nil
}

// isTracked reports whether git tracks any file under path; a folder archived before the git operations is untracked.
func (s *Store) isTracked(ctx context.Context, path string) (bool, error) {
	output, err := s.git(ctx, "ls-files", "--", path)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(output) != "", nil
}

func (s *Store) loadedFingerprint() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fingerprint
}

// moveScope moves a folder between paths relative to the store: git mv plus a commit of both paths when tracked, a rename when not.
func (s *Store) moveScope(ctx context.Context, subject, source, target string) error {
	isTracked, err := s.isTracked(ctx, source)
	if err != nil {
		return fmt.Errorf("Store.moveScope: %w", err)
	}
	if !isTracked {
		if err := os.Rename(filepath.Join(s.dir, source), filepath.Join(s.dir, target)); err != nil {
			return fmt.Errorf("Store.moveScope: Failed to move %s: %w", source, err)
		}
		return nil
	}

	if _, err := s.git(ctx, "mv", source, target); err != nil {
		return fmt.Errorf("Store.moveScope: %w", err)
	}
	if err := s.commitPaths(ctx, subject, source, target); err != nil {
		return fmt.Errorf("Store.moveScope: %w", err)
	}
	return nil
}

// recordFingerprint joins the path, size and mtime of every file and dir under the store, in walk order; any add, move, removal or rewrite changes it.
func (s *Store) recordFingerprint() (string, error) {
	var lines []string
	walk := func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		lines = append(lines, fmt.Sprintf("%s %d %d", path, info.Size(), info.ModTime().UnixNano()))
		return nil
	}

	if err := filepath.WalkDir(s.dir, walk); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("Store.recordFingerprint: %s: %w", s.dir, err)
	}
	return strings.Join(lines, "\n"), nil
}

func (s *Store) requireCheckout(ctx context.Context) error {
	if _, err := shell.Run(ctx, s.dir, "git", "rev-parse", "--is-inside-work-tree"); err != nil {
		return fmt.Errorf("Store.requireCheckout: sessions dir is not a git checkout: %s: %w", s.dir, err)
	}
	return nil
}

func (s *Store) swap(fingerprint string, fresh map[string]*ScopeInfo) {
	s.mu.Lock()
	s.byScope = fresh
	s.fingerprint = fingerprint
	s.mu.Unlock()
}

// ArchiveScope moves a scope's tracked records with git mv and commits the move, so the archive stays tracked (master E2).
func (s *Store) ArchiveScope(ctx context.Context, name string) error {
	if err := validateScopeName(name); err != nil {
		return fmt.Errorf("Store.ArchiveScope: %w", err)
	}
	if err := s.requireCheckout(ctx); err != nil {
		return fmt.Errorf("Store.ArchiveScope: %w", err)
	}

	archivedDir := filepath.Join(s.dir, archivedDirName)
	if err := os.MkdirAll(archivedDir, 0o755); err != nil {
		return fmt.Errorf("Store.ArchiveScope: Failed to create %s: %w", archivedDir, err)
	}

	target := filepath.Join(archivedDir, name)
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("Store.ArchiveScope: %s already archived", name)
	}
	if err := s.moveScope(ctx, "sessions: Archived the "+name+" records", name, filepath.Join(archivedDirName, name)); err != nil {
		return fmt.Errorf("Store.ArchiveScope: %w", err)
	}
	return nil
}

func (s *Store) ArchivedFolders() []ArchivedFolder {
	entries, err := os.ReadDir(filepath.Join(s.dir, archivedDirName))
	if err != nil {
		return nil
	}

	folders := make([]ArchivedFolder, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		folder := ArchivedFolder{Name: entry.Name()}
		if records, err := RecordFiles(filepath.Join(s.dir, archivedDirName, entry.Name())); err == nil {
			folder.Sessions = len(records)
		}
		folders = append(folders, folder)
	}
	sort.Slice(folders, func(i, j int) bool { return folders[i].Name < folders[j].Name })
	return folders
}

// DeleteScope moves an archived folder into the trash and commits its removal; still-indexed sessions may re-mine afterwards (master E1).
func (s *Store) DeleteScope(ctx context.Context, name string) error {
	if err := validateScopeName(name); err != nil {
		return fmt.Errorf("Store.DeleteScope: %w", err)
	}
	if err := s.requireCheckout(ctx); err != nil {
		return fmt.Errorf("Store.DeleteScope: %w", err)
	}

	target := filepath.Join(s.dir, archivedDirName, name)
	if _, err := os.Stat(target); err != nil {
		return fmt.Errorf("Store.DeleteScope: %s is not archived: %w", name, err)
	}

	archivedPath := filepath.Join(archivedDirName, name)
	isTracked, err := s.isTracked(ctx, archivedPath)
	if err != nil {
		return fmt.Errorf("Store.DeleteScope: %w", err)
	}

	entry := &trash.Entry{Action: "sessions delete", Reason: "archived records deleted from the Sessions page"}
	if _, err := s.trash.Move(target, entry); err != nil {
		return fmt.Errorf("Store.DeleteScope: %w", err)
	}
	if !isTracked {
		return nil
	}

	if _, err := s.git(ctx, "rm", "-r", "-q", "--cached", archivedPath); err != nil {
		return fmt.Errorf("Store.DeleteScope: %w", err)
	}
	if err := s.commitPaths(ctx, "sessions: Deleted the archived "+name+" records", archivedPath); err != nil {
		return fmt.Errorf("Store.DeleteScope: %w", err)
	}
	return nil
}

func (s *Store) InvocationsBySkill() map[string][]Invocation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string][]Invocation)
	for _, info := range s.byScope {
		for _, week := range info.Weeks {
			collectInvocations(result, week)
		}
	}
	return result
}

// Reload records malformed files as load errors, never fatal.
func (s *Store) Reload() error {
	info, err := os.Stat(s.dir)
	if err == nil && !info.IsDir() {
		return fmt.Errorf("Store.Reload: Not a directory: %s", s.dir)
	}

	fingerprint, err := s.recordFingerprint()
	if err != nil {
		return fmt.Errorf("Store.Reload: %w", err)
	}
	if fingerprint == s.loadedFingerprint() {
		return nil
	}

	fresh := make(map[string]*ScopeInfo)

	scopeDirs, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			s.swap(fingerprint, fresh)
			return nil
		}
		return fmt.Errorf("Store.Reload: Failed to read %s: %w", s.dir, err)
	}

	for _, scopeDir := range scopeDirs {
		// archived folders leave the store but stay counted as mined by the pending selection
		if !scopeDir.IsDir() || scopeDir.Name() == archivedDirName {
			continue
		}
		fresh[scopeDir.Name()] = loadScope(scopeDir.Name(), filepath.Join(s.dir, scopeDir.Name()))
	}

	s.swap(fingerprint, fresh)
	return nil
}

func (s *Store) Scope(scope string) (*ScopeInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	info, ok := s.byScope[scope]
	return info, ok
}

func (s *Store) Scopes() []*ScopeInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	scopes := make([]*ScopeInfo, 0, len(s.byScope))
	for _, info := range s.byScope {
		scopes = append(scopes, info)
	}
	sort.Slice(scopes, func(i, j int) bool { return scopes[i].Name < scopes[j].Name })
	return scopes
}

func (s *Store) SentimentByWeek() []SentimentPoint {
	s.mu.RLock()
	defer s.mu.RUnlock()

	byWeek := make(map[string]*SentimentPoint)
	for _, info := range s.byScope {
		for _, week := range info.Weeks {
			addSentiment(byWeek, week)
		}
	}

	points := make([]SentimentPoint, 0, len(byWeek))
	for _, point := range byWeek {
		points = append(points, *point)
	}
	slices.SortFunc(points, func(a, b SentimentPoint) int { return strings.Compare(a.Week, b.Week) })
	return points
}

func (s *Store) SessionRefs() map[string]SessionRef {
	s.mu.RLock()
	defer s.mu.RUnlock()

	refs := make(map[string]SessionRef)
	for _, info := range s.byScope {
		for _, week := range info.Weeks {
			for index := range week.Sessions {
				refs[week.Sessions[index].Id] = SessionRef{LastActive: week.Sessions[index].LastActive, Scope: week.Scope, Week: week.Key}
			}
		}
	}
	return refs
}

// UnarchiveScope refuses, never merges, when a live folder of that name was mined meanwhile.
func (s *Store) UnarchiveScope(ctx context.Context, name string) error {
	if err := validateScopeName(name); err != nil {
		return fmt.Errorf("Store.UnarchiveScope: %w", err)
	}
	if err := s.requireCheckout(ctx); err != nil {
		return fmt.Errorf("Store.UnarchiveScope: %w", err)
	}

	target := filepath.Join(s.dir, name)
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("Store.UnarchiveScope: %s already exists as a live folder", name)
	}
	if err := s.moveScope(ctx, "sessions: Unarchived the "+name+" records", filepath.Join(archivedDirName, name), name); err != nil {
		return fmt.Errorf("Store.UnarchiveScope: %w", err)
	}
	return nil
}

// Week returns one scope's week by key.
func (s *Store) Week(scope, key string) (*Week, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	info, ok := s.byScope[scope]
	if !ok {
		return nil, false
	}
	for _, week := range info.Weeks {
		if week.Key == key {
			return week, true
		}
	}
	return nil, false
}

type TouchedFiles struct {
	Reads  int `json:"reads"`
	Writes int `json:"writes"`
}

// Week is one repo's session records of one ISO week, oldest first.
type Week struct {
	Key      string
	Scope    string
	Sessions []Session
}

// Label renders the week as "<Mon> – <Sun>", the run-band convention.
func (w *Week) Label() string {
	start, err := WeekStart(w.Key)
	if err != nil {
		return w.Key
	}
	return start.Format(time.DateOnly) + " – " + start.AddDate(0, 0, daysPerWeek-1).Format(time.DateOnly)
}

// load reads the week dir's record JSONs; a malformed JSON is a load error.
func (w *Week) load(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{fmt.Sprintf("%s: %v", dir, err)}
	}

	loadErrors := []string{}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != RecordJsonExt {
			continue
		}
		if loadError := w.loadSession(filepath.Join(dir, entry.Name())); loadError != "" {
			loadErrors = append(loadErrors, loadError)
		}
	}
	slices.SortFunc(w.Sessions, func(a, b Session) int { return strings.Compare(a.LastActive, b.LastActive) })
	return loadErrors
}

func (w *Week) loadSession(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("%s: %v", path, err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return fmt.Sprintf("%s: %v", path, err)
	}
	w.Sessions = append(w.Sessions, session)
	return ""
}

func addSentiment(byWeek map[string]*SentimentPoint, week *Week) {
	point, ok := byWeek[week.Key]
	if !ok {
		point = &SentimentPoint{Week: week.Key}
		byWeek[week.Key] = point
	}
	for index := range week.Sessions {
		if week.Sessions[index].Skipped {
			continue
		}
		point.Frustration += len(week.Sessions[index].Frustration)
		point.Positive += len(week.Sessions[index].Positive)
	}
}

func collectInvocations(result map[string][]Invocation, week *Week) {
	for index := range week.Sessions {
		session := &week.Sessions[index]
		for _, skillName := range session.InvokedSkills {
			invocation := Invocation{
				LastActive:   session.LastActive,
				Scope:        week.Scope,
				SessionId:    session.Id,
				SessionTitle: session.Title,
				Week:         week.Key,
			}
			result[skillName] = append(result[skillName], invocation)
		}
	}
}

func loadScope(name, scopePath string) *ScopeInfo {
	info := &ScopeInfo{Name: name}
	weekDirs, err := os.ReadDir(scopePath)
	if err != nil {
		info.LoadErrors = append(info.LoadErrors, fmt.Sprintf("%s: %v", scopePath, err))
		return info
	}

	for _, weekDir := range weekDirs {
		if !weekDir.IsDir() || !IsWeekKey(weekDir.Name()) {
			continue
		}
		week := &Week{Key: weekDir.Name(), Scope: name}
		info.LoadErrors = append(info.LoadErrors, week.load(filepath.Join(scopePath, weekDir.Name()))...)
		if len(week.Sessions) > 0 {
			info.Weeks = append(info.Weeks, week)
		}
	}
	slices.SortFunc(info.Weeks, func(a, b *Week) int { return strings.Compare(b.Key, a.Key) })
	return info
}

// validateScopeName keeps "unknown" archivable — it is an ordinary mining folder.
func validateScopeName(name string) error {
	// name (also a path segment: never traversal, never the archive container)
	isUnsafe := name == "" || name == archivedDirName || name == "." || name == ".." || name != filepath.Base(name)
	if isUnsafe || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("validateScopeName: Invalid scope name %q", name)
	}
	return nil
}

// ResolveRef finds a session by its id or by a prefix only one id carries; an ambiguous prefix resolves nothing.
func ResolveRef(idOrPrefix string, refs map[string]SessionRef) (string, SessionRef, bool) {
	var none SessionRef
	if ref, ok := refs[idOrPrefix]; ok {
		return idOrPrefix, ref, true
	}

	matched := ""
	for id := range refs {
		if !strings.HasPrefix(id, idOrPrefix) {
			continue
		}
		if matched != "" {
			return "", none, false
		}
		matched = id
	}
	if matched == "" {
		return "", none, false
	}
	return matched, refs[matched], true
}

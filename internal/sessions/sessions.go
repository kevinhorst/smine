package sessions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type Arc struct {
	SessionIds []string `json:"sessionIds"`
	Summary    string   `json:"summary"`
}

type Batch struct {
	AnalyzedDate string `json:"analyzedDate"`
	DateRange    string `json:"dateRange"`
	File         string `json:"file"`
	Number       int    `json:"number"`
	Scope        string `json:"scope"`
	SourceNote   string `json:"sourceNote"`
	Theme        string `json:"theme"`
	Title        string `json:"title"`
}

type BatchSummary struct {
	sourcePath string

	Arcs     []Arc     `json:"arcs"`
	Batch    Batch     `json:"batch"`
	Sessions []Session `json:"sessions"`
}

func (b *BatchSummary) SourcePath() string {
	return b.sourcePath
}

type Finding struct {
	Dimension string    `json:"dimension"`
	Quotes    []string  `json:"quotes"`
	Snippets  []Snippet `json:"snippets"`
	Summary   string    `json:"summary"`
}

type Snippet struct {
	Code   string `json:"code"`
	Kind   string `json:"kind"`
	Lang   string `json:"lang"`
	Source string `json:"source"`
}

type SentimentPoint struct {
	BatchNumber int
	Frustration int
	Positive    int
}

type SessionRef struct {
	BatchNumber int
	Scope       string
}

type Frustration struct {
	Quote   string `json:"quote"`
	Trigger string `json:"trigger"`
}

type Invocation struct {
	BatchDate    string
	BatchNumber  int
	Scope        string
	SessionId    string
	SessionTitle string
}

type Positive struct {
	Quote   string `json:"quote"`
	Trigger string `json:"trigger"`
}

type Session struct {
	Id string `json:"id"`

	DataGaps      string        `json:"dataGaps"`
	Findings      []Finding     `json:"findings"`
	Frustration   []Frustration `json:"frustration"`
	InvokedSkills []string      `json:"invoked_skills"`
	Positive      []Positive    `json:"positive"`
	Repo          string        `json:"repo"`
	Signal        string        `json:"signal"`
	SkipReason    string        `json:"skipReason"`
	Skipped       bool          `json:"skipped"`
	Title         string        `json:"title"`
	Type          string        `json:"type"`
}

type ScopeInfo struct {
	Batches    []*BatchSummary
	LoadErrors []string
	MdReports  int
	Name       string
}

type Store struct {
	dir string

	// mu guards byScope for concurrent request reads vs. Reload.
	mu      sync.RWMutex
	byScope map[string]*ScopeInfo
}

func NewStore(dir string) *Store {
	return &Store{
		byScope: make(map[string]*ScopeInfo),
		dir:     dir,
	}
}

// Reload scans dir/*/json/*.json, building fresh state before swapping it in
// under the write lock. Malformed files are recorded, never fatal.
func (s *Store) Reload() error {
	fresh := make(map[string]*ScopeInfo)

	scopeDirs, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			s.swap(fresh)
			return nil
		}
		return fmt.Errorf("Store.Reload: Failed to read %s: %w", s.dir, err)
	}

	for _, scopeDir := range scopeDirs {
		if !scopeDir.IsDir() {
			continue
		}
		info := &ScopeInfo{Name: scopeDir.Name()}
		scopePath := filepath.Join(s.dir, scopeDir.Name())

		if files, err := os.ReadDir(scopePath); err == nil {
			for _, f := range files {
				if !f.IsDir() && strings.Contains(f.Name(), "batch") && strings.HasSuffix(f.Name(), ".md") {
					info.MdReports++
				}
			}
		}

		jsonDir := filepath.Join(scopePath, "json")
		files, err := os.ReadDir(jsonDir)
		if err != nil && !os.IsNotExist(err) {
			info.LoadErrors = append(info.LoadErrors, fmt.Sprintf("%s: %v", jsonDir, err))
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
				continue
			}
			path := filepath.Join(jsonDir, f.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				info.LoadErrors = append(info.LoadErrors, fmt.Sprintf("%s: %v", f.Name(), err))
				continue
			}
			var batch BatchSummary
			if err := json.Unmarshal(data, &batch); err != nil {
				info.LoadErrors = append(info.LoadErrors, fmt.Sprintf("%s: %v", f.Name(), err))
				continue
			}
			// The directory is the scope's identity; older work-scope batches
			// carry the organisation's own name (e.g. "aqms") in the JSON.
			batch.Batch.Scope = scopeDir.Name()
			batch.sourcePath = path
			info.Batches = append(info.Batches, &batch)
		}
		sort.Slice(info.Batches, func(i, j int) bool {
			return info.Batches[i].Batch.Number < info.Batches[j].Batch.Number
		})
		fresh[scopeDir.Name()] = info
	}

	s.swap(fresh)
	return nil
}

func (s *Store) swap(fresh map[string]*ScopeInfo) {
	s.mu.Lock()
	s.byScope = fresh
	s.mu.Unlock()
}

// InvocationsBySkill aggregates invoked_skills across every loaded batch,
// keyed by skill name. Computed per call — the store is already in memory
// (D12); batches predating the schema extension simply contribute nothing.
func (s *Store) InvocationsBySkill() map[string][]Invocation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string][]Invocation)
	for _, info := range s.byScope {
		for _, batch := range info.Batches {
			collectInvocations(batch, result)
		}
	}
	return result
}

func collectInvocations(batch *BatchSummary, result map[string][]Invocation) {
	for _, session := range batch.Sessions {
		for _, skillName := range session.InvokedSkills {
			invocation := Invocation{
				BatchDate:    batch.Batch.AnalyzedDate,
				BatchNumber:  batch.Batch.Number,
				Scope:        batch.Batch.Scope,
				SessionId:    session.Id,
				SessionTitle: session.Title,
			}
			result[skillName] = append(result[skillName], invocation)
		}
	}
}

// SentimentByBatch sums frustration/positive entries per batch number, both
// scopes combined, sorted by batch number. Computed per call — the store is
// already in memory, mirroring InvocationsBySkill. Skipped sessions and
// batches predating the positive field simply contribute zero.
func (s *Store) SentimentByBatch() []SentimentPoint {
	s.mu.RLock()
	defer s.mu.RUnlock()

	byNumber := make(map[int]*SentimentPoint)
	for _, info := range s.byScope {
		for _, batch := range info.Batches {
			point, ok := byNumber[batch.Batch.Number]
			if !ok {
				point = &SentimentPoint{BatchNumber: batch.Batch.Number}
				byNumber[batch.Batch.Number] = point
			}
			for index := range batch.Sessions {
				if batch.Sessions[index].Skipped {
					continue
				}
				point.Frustration += len(batch.Sessions[index].Frustration)
				point.Positive += len(batch.Sessions[index].Positive)
			}
		}
	}

	points := make([]SentimentPoint, 0, len(byNumber))
	for _, point := range byNumber {
		points = append(points, *point)
	}
	sort.Slice(points, func(i, j int) bool { return points[i].BatchNumber < points[j].BatchNumber })
	return points
}

// SessionRefs maps every loaded session id to its batch. Computed per call —
// the store is already in memory, mirroring InvocationsBySkill.
func (s *Store) SessionRefs() map[string]SessionRef {
	s.mu.RLock()
	defer s.mu.RUnlock()

	refs := make(map[string]SessionRef)
	for _, info := range s.byScope {
		for _, batch := range info.Batches {
			for index := range batch.Sessions {
				refs[batch.Sessions[index].Id] = SessionRef{BatchNumber: batch.Batch.Number, Scope: batch.Batch.Scope}
			}
		}
	}
	return refs
}

func (s *Store) Batch(scope string, number int) (*BatchSummary, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	info, ok := s.byScope[scope]
	if !ok {
		return nil, false
	}
	for _, b := range info.Batches {
		if b.Batch.Number == number {
			return b, true
		}
	}
	return nil, false
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

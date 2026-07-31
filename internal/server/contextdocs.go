package server

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/server/respond"
)

const (
	pageContext        = "context"
	tmplContextAspects = "_aspects.html"
	tmplContextDoc     = "context_doc.html"
	tmplContextIndex   = "context_index.html"
	// contextSyncLockKey serializes the context sync via the repo-locks map,
	// like skillsSyncLockKey: the underscore prefix keeps it out of the
	// registry's repo-name space.
	contextSyncLockKey = "_context-sync"
)

type contextDocPage struct {
	Body  template.HTML
	Dir   string
	File  string
	Page  string
	Title string
}

type contextIndexPage struct {
	AgentsTemplate string
	AspectsSection aspectsFragment
	Groups         []contextdocs.Group
	Page           string
	RepoNames      []string
	Title          string
}

// aspectsFragment backs the _aspects.html fragment.
type aspectsFragment struct {
	Aspects []contextdocs.RuleAspect
	Error   string
}

// aspectNamePattern bounds UI-created aspect names: short, upper-case,
// letters only — matching the entry-ID grammar's ASPECT segment.
var aspectNamePattern = regexp.MustCompile(`^[A-Z]{2,12}$`)

// folderPick backs the _repo_path.html picker fragment.
type folderPick struct {
	Error string
	Value string
}

func (s *Server) handleContextIndex(w http.ResponseWriter, r *http.Request) {
	groups, err := contextdocs.Scan(s.contextDir)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	agents, err := os.ReadFile(filepath.Join(s.contextDir, "AGENTS.md"))
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	aspects, err := s.loadAspectsTolerant()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	data := contextIndexPage{
		AgentsTemplate: string(agents),
		AspectsSection: aspectsFragment{Aspects: aspects},
		Groups:         groups,
		Page:           pageContext,
		Title:          "Context",
	}
	for _, repo := range s.repoRegistry.Repos() {
		data.RepoNames = append(data.RepoNames, repo.Name)
	}
	s.renderFragment(w, tmplContextIndex, data)
}

func (s *Server) handleContextDoc(w http.ResponseWriter, r *http.Request) {
	groups, err := contextdocs.Scan(s.contextDir)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	// The requested doc must match a scanned entry by string equality; user
	// input is never joined into a path on its own (concept security rule,
	// mirrors handleSkillFile).
	dir, file := r.PathValue("dir"), r.PathValue("file")
	found := false
	for _, group := range groups {
		if group.Name == dir && slices.Contains(group.Files, file) {
			found = true
		}
	}
	if !found {
		http.NotFound(w, r)
		return
	}

	raw, err := os.ReadFile(filepath.Join(s.contextDir, dir, file))
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	body, err := renderMarkdown(raw)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	data := contextDocPage{
		Body:  body,
		Dir:   dir,
		File:  file,
		Page:  pageContext,
		Title: "Context — " + dir + "/" + file,
	}
	s.renderFragment(w, tmplContextDoc, data)
}

// loadAspectsTolerant reads the vocabulary; a missing aspects.json renders
// as an empty section (fresh checkout), every other error is real.
func (s *Server) loadAspectsTolerant() ([]contextdocs.RuleAspect, error) {
	aspects, err := contextdocs.LoadAspects(filepath.Join(s.contextDir, "rules"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return aspects, nil
}

func (s *Server) renderAspects(errorMessage string, w http.ResponseWriter) {
	aspects, err := s.loadAspectsTolerant()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	s.renderFragment(w, tmplContextAspects, aspectsFragment{Aspects: aspects, Error: errorMessage})
}

func (s *Server) handleContextAspectAdd(w http.ResponseWriter, r *http.Request) {
	name := strings.ToUpper(strings.TrimSpace(r.FormValue("name")))
	scope := strings.TrimSpace(r.FormValue("scope"))
	if !aspectNamePattern.MatchString(name) {
		s.renderAspects("aspect name must be 2-12 letters A-Z", w)
		return
	}
	if scope == "" {
		s.renderAspects("scope is required", w)
		return
	}

	aspects, err := s.loadAspectsTolerant()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	for _, aspect := range aspects {
		if aspect.Name == name {
			s.renderAspects("aspect "+name+" already exists", w)
			return
		}
	}

	aspects = append(aspects, contextdocs.RuleAspect{Name: name, Scope: scope})
	if err := contextdocs.SaveAspects(filepath.Join(s.contextDir, "rules"), aspects); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	s.renderAspects("", w)
}

func (s *Server) handleContextAspectDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	rulesDir := filepath.Join(s.contextDir, "rules")

	// In-use guard: a vocabulary the entries still cite cannot lose members —
	// the next audit run would fail on every citing entry.
	set, err := contextdocs.ParseRulesDir(rulesDir, false)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	var blockingIds []string
	for _, entry := range set.Entries {
		if entry.Aspect == name {
			blockingIds = append(blockingIds, entry.Id)
		}
	}
	if len(blockingIds) > 0 {
		s.renderAspects("aspect "+name+" is in use: "+strings.Join(blockingIds, ", "), w)
		return
	}

	aspects, err := s.loadAspectsTolerant()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	kept := make([]contextdocs.RuleAspect, 0, len(aspects))
	found := false
	for _, aspect := range aspects {
		if aspect.Name == name {
			found = true
			continue
		}
		kept = append(kept, aspect)
	}
	if !found {
		respond.WithBadRequest("unknown aspect: "+name, w)
		return
	}
	if err := contextdocs.SaveAspects(rulesDir, kept); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	s.renderAspects("", w)
}

func (s *Server) handleContextSync(w http.ResponseWriter, r *http.Request) {
	// Concept boundary: the request selects by name, the path comes from the
	// registry (mirrors findRepo).
	repo, ok := s.repoRegistry.Find(r.FormValue("name"))
	if !ok {
		respond.WithBadRequest("unknown repo: "+r.FormValue("name"), w)
		return
	}

	opts := contextdocs.SyncOptions{
		ContextDir: r.FormValue("context-dir"),
		Symlink:    r.FormValue("symlink") == "on",
		Target:     repo.Path,
	}

	// Flat sync: every language ships. Languages are the style/ files minus
	// the always-synced artifact guides — mirrors the builder's split.
	groups, err := contextdocs.Scan(s.contextDir)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	for _, group := range groups {
		if group.Name != "style" {
			continue
		}
		for _, file := range group.Files {
			name := strings.TrimSuffix(file, ".md")
			if name == "plan" || name == "commits" {
				continue
			}
			opts.Langs = append(opts.Langs, name)
		}
	}

	op := func(ctx context.Context) (string, error) {
		return contextdocs.Sync(ctx, opts, s.syncScripts)
	}
	s.runRepoOp(contextSyncLockKey, "repo-op", op, w, r)
}

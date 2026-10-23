package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/server/catalog"
	"github.com/kevinhorst/smine/internal/server/respond"
)

var errEntryMissing = errors.New("entry not found")

func (s *Server) handleConfigEntryRemove(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")
	key := r.PathValue("key")
	name := r.PathValue("name")
	if s.entryEntry(target, key, w) == nil {
		return
	}

	if !s.mutateClaudeEntries(key, w, func(doc *config.Document, path []string) error {
		if !doc.Unset(append(path, name)) {
			return errEntryMissing
		}
		return nil
	}) {
		return
	}

	s.renderConfigRow(w, target, key)
}

func (s *Server) handleConfigEntrySet(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")
	key := r.PathValue("key")
	name := strings.TrimSpace(r.FormValue("name"))
	value := r.FormValue("value")
	entry := s.entryEntry(target, key, w)
	if entry == nil {
		return
	}
	if name == "" {
		respond.WithBadRequest("empty entry name", w)
		return
	}
	if err := catalog.ValidateEntry(entry, value); err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}

	if !s.mutateClaudeEntries(key, w, func(doc *config.Document, path []string) error {
		quoted, _ := json.Marshal(value)
		return doc.Set(append(path, name), quoted)
	}) {
		return
	}

	s.renderConfigRow(w, target, key)
}

// entryEntry gates the entry endpoints to documented map keys of the claude target.
func (s *Server) entryEntry(target, key string, w http.ResponseWriter) *catalog.Entry {
	if target != catalog.TargetClaude {
		respond.WithNotFound("no entry editor for target: "+target, w)
		return nil
	}

	entry := s.catalogEntry(target, key)
	if entry == nil || entry.Type != catalog.TypeMap {
		respond.WithBadRequest("not a documented map key: "+key, w)
		return nil
	}
	return entry
}

// mutateClaudeEntries answers 404 for a missing entry and 400 for a parent that is no object.
func (s *Server) mutateClaudeEntries(key string, w http.ResponseWriter, mutate func(doc *config.Document, path []string) error) bool {
	settings, err := config.Load(s.settingsPath)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return false
	}

	err = mutate(settings.Doc(), strings.Split(key, "."))
	switch {
	case errors.Is(err, errEntryMissing):
		respond.WithNotFound(err.Error(), w)
		return false
	case err != nil:
		respond.WithBadRequest(err.Error(), w)
		return false
	}

	if err := config.Save(s.settingsPath, settings, s.trash); err != nil {
		respond.WithInternalServerError(err, w)
		return false
	}
	return true
}

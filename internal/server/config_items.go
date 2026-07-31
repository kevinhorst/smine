package server

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/creachadair/tomledit/parser"
	"github.com/kevinhorst/smine/internal/codex"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/server/catalog"
	"github.com/kevinhorst/smine/internal/server/respond"
)

func (s *Server) handleConfigItemAdd(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")
	key := r.PathValue("key")
	value := strings.TrimSpace(r.FormValue("value"))
	entry := s.itemEntry(target, key, w)
	if entry == nil {
		return
	}
	if value == "" {
		respond.WithBadRequest("empty item", w)
		return
	}

	if target == catalog.TargetClaude && !s.mutateClaudeItems(key, w, func(items []json.RawMessage) ([]json.RawMessage, bool) {
		quoted, _ := json.Marshal(value)
		return append(items, quoted), true
	}) {
		return
	}
	if target == catalog.TargetCodex && !s.mutateCodexItems(key, w, func(items []string) ([]string, bool) {
		return append(items, strconv.Quote(value)), true
	}) {
		return
	}

	s.renderConfigRow(w, target, key)
}

func (s *Server) handleConfigItemRemove(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")
	key := r.PathValue("key")
	index, err := strconv.Atoi(r.PathValue("index"))
	if err != nil || index < 0 {
		respond.WithBadRequest("invalid index", w)
		return
	}
	if s.itemEntry(target, key, w) == nil {
		return
	}

	removeAt := func(length int) bool { return index < length }
	if target == catalog.TargetClaude && !s.mutateClaudeItems(key, w, func(items []json.RawMessage) ([]json.RawMessage, bool) {
		if !removeAt(len(items)) {
			return items, false
		}
		return slices.Delete(items, index, index+1), true
	}) {
		return
	}
	if target == catalog.TargetCodex && !s.mutateCodexItems(key, w, func(items []string) ([]string, bool) {
		if !removeAt(len(items)) {
			return items, false
		}
		return slices.Delete(items, index, index+1), true
	}) {
		return
	}

	s.renderConfigRow(w, target, key)
}

// itemEntry gates the item endpoints to documented array keys (D5).
func (s *Server) itemEntry(target, key string, w http.ResponseWriter) *catalog.Entry {
	if !catalog.IsTarget(target) {
		respond.WithNotFound("unknown target: "+target, w)
		return nil
	}

	entry := s.catalogEntry(target, key)
	if entry == nil || entry.Type != catalog.TypeArray {
		respond.WithBadRequest("not a documented array key: "+key, w)
		return nil
	}
	return entry
}

// mutateClaudeItems loads the array (missing key = empty), applies mutate,
// and saves. A false second return from mutate means "index gone" → 404.
func (s *Server) mutateClaudeItems(key string, w http.ResponseWriter, mutate func([]json.RawMessage) ([]json.RawMessage, bool)) bool {
	settings, err := config.Load(s.settingsPath)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return false
	}

	path := strings.Split(key, ".")
	var items []json.RawMessage
	if raw, ok := settings.Doc().Get(path); ok {
		if err := json.Unmarshal(raw, &items); err != nil {
			respond.WithBadRequest("existing value is not an array", w)
			return false
		}
	}

	items, ok := mutate(items)
	if !ok {
		respond.WithNotFound("item index out of range", w)
		return false
	}

	encoded, err := json.Marshal(items)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return false
	}
	if items == nil {
		encoded = json.RawMessage("[]")
	}
	if err := settings.Doc().Set(path, encoded); err != nil {
		respond.WithBadRequest(err.Error(), w)
		return false
	}

	if err := config.Save(s.settingsPath, settings); err != nil {
		respond.WithInternalServerError(err, w)
		return false
	}
	return true
}

// mutateCodexItems rebuilds the TOML array literal from item literals (F16).
func (s *Server) mutateCodexItems(key string, w http.ResponseWriter, mutate func([]string) ([]string, bool)) bool {
	cfg, err := codex.Load(s.codexPath)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return false
	}

	var items []string
	if literal, ok := cfg.Get(key); ok {
		value, err := parser.ParseValue(literal)
		if err != nil {
			respond.WithBadRequest("existing value is not valid TOML", w)
			return false
		}
		array, isArray := value.X.(parser.Array)
		if !isArray {
			respond.WithBadRequest("existing value is not an array", w)
			return false
		}
		for _, element := range array {
			if item, isValue := element.(parser.Value); isValue {
				items = append(items, item.String())
			}
		}
	}

	items, ok := mutate(items)
	if !ok {
		respond.WithNotFound("item index out of range", w)
		return false
	}

	literal := "[" + strings.Join(items, ", ") + "]"
	if err := cfg.Set(key, literal); err != nil {
		respond.WithBadRequest(err.Error(), w)
		return false
	}

	if err := codex.Save(s.codexPath, cfg); err != nil {
		respond.WithInternalServerError(err, w)
		return false
	}
	return true
}

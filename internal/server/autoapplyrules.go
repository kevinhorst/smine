package server

import (
	"fmt"
	"net/http"
	"os"

	"github.com/kevinhorst/smine/internal/server/respond"
)

const maxAutoApplyRulesBytes = 65536

// handleAutoApplyRulesSave writes atomically — same discipline as
// checklist.SetStatus; the full-page reload mirrors handleChecklistStatus and
// re-renders the proposals auto-apply tab the form lives on.
func (s *Server) handleAutoApplyRulesSave(w http.ResponseWriter, r *http.Request) {
	content := r.FormValue("content")
	if len(content) > maxAutoApplyRulesBytes {
		respond.WithBadRequest("rules file exceeds 64 KiB", w)
		return
	}

	tmp := s.autoApplyRulesPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		respond.WithInternalServerError(fmt.Errorf("handleAutoApplyRulesSave: Failed to write %s: %w", tmp, err), w)
		return
	}

	if err := os.Rename(tmp, s.autoApplyRulesPath); err != nil {
		os.Remove(tmp)
		respond.WithInternalServerError(fmt.Errorf("handleAutoApplyRulesSave: Failed to rename %s to %s: %w", tmp, s.autoApplyRulesPath, err), w)
		return
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

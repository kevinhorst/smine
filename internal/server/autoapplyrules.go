package server

import (
	"fmt"
	"os"

	"github.com/kevinhorst/smine/internal/fsx"
)

const maxAutoApplyRulesBytes = 65536

// saveAutoApplyRules writes atomically — same discipline as
// checklist.SetStatus. Called from the configure-panel save (the rules
// textarea rides the params form).
func (s *Server) saveAutoApplyRules(content string) error {
	tmp := s.autoApplyRulesPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		return fmt.Errorf("saveAutoApplyRules: Failed to write %s: %w", tmp, err)
	}

	if err := fsx.ReplaceFile(tmp, s.autoApplyRulesPath); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("saveAutoApplyRules: Failed to rename %s to %s: %w", tmp, s.autoApplyRulesPath, err)
	}
	return nil
}

package server

import (
	"net/http"
	"strings"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/server/respond"
)

const tmplComponentOff = "component_off.html"

// componentOffPage replaces a page whose components are off: the nav hides it, a typed URL explains it.
type componentOffPage struct {
	Needs string
	Page  string
	Title string
}

// hasProposals: the Proposals page reads the change commits the Mining pipeline produces, through agentwt; Mining brings peek for the session records.
func (s *Server) hasProposals() bool {
	return s.records.has(s.machine, components.Mining) && s.records.has(s.machine, components.Agentwt)
}

// hasUsage: the Usage page reads the store only smine-budget fills, so it needs Routines and at least one cost source.
func (s *Server) hasUsage() bool {
	hasSource := s.records.has(s.machine, components.Peek) || s.records.has(s.machine, components.Ccusage)
	return hasSource && s.records.has(s.machine, components.Routines)
}

// withGate refuses a permission-list route while the Permission gate is off: its lists are the gate's, and editing them without it drifts from what a later install syncs.
func (s *Server) withGate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.records.has(s.machine, components.Gate) {
			respond.WithConflict("the Permission gate is off: install it on the Profile page to edit its permission lists", w)
			return
		}
		next(w, r)
	}
}

func (s *Server) renderComponentOff(w http.ResponseWriter, page, title string, needs ...components.ID) {
	titles := make([]string, 0, len(needs))
	for _, id := range needs {
		if component, ok := components.Lookup(id); ok {
			titles = append(titles, component.Title)
		}
	}
	data := componentOffPage{Needs: strings.Join(titles, ", "), Page: page, Title: translate(s.presentation.language(), title)}
	s.renderFragment(w, tmplComponentOff, data)
}

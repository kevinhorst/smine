package server

import (
	"net/http"
	"slices"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/server/respond"
)

const tmplMcp = "_mcp.html"

type mcpData struct {
	Error      string
	Overridden bool
	Servers    []mcpRow
}

type mcpRow struct {
	Enabled bool
	Name    string
}

func (s *Server) handleGetMCP(w http.ResponseWriter, _ *http.Request) {
	main, _, err := s.loadBoth()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	s.renderMCP(w, main)
}

// handleToggleMCP flips one name in the live disabledMcpjsonServers list —
// the only MCP state (D5). Any leftover key in the disabled sidecar (the
// deleted shuttle mechanism) is cleared on the way.
func (s *Server) handleToggleMCP(w http.ResponseWriter, r *http.Request) {
	serverName := r.PathValue("server")

	main, disabled, err := s.loadBoth()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	list, err := main.DisabledMcpjsonServers()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	if index := slices.Index(list, serverName); index >= 0 {
		list = slices.Delete(list, index, index+1)
	} else {
		list = append(list, serverName)
	}

	if err := main.SetDisabledMcpjsonServers(list); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	if err := disabled.SetDisabledMcpjsonServers(nil); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	if err := s.saveBoth(main, disabled); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	w.Header().Set("HX-Trigger", "config-op")
	s.renderMCP(w, main)
}

// renderMCP lists every server in ~/.claude.json plus any disabled-list
// names not registered there; enabled = absent from the live disabled list
// (D5). An unreadable ~/.claude.json degrades to the disabled list (D11).
func (s *Server) renderMCP(w http.ResponseWriter, main *config.Settings) {
	disabledNames, err := main.DisabledMcpjsonServers()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	var data mcpData
	known, err := config.McpServerNames(s.claudeJsonPath)
	if err != nil {
		data.Error = err.Error()
	}

	names := known
	for _, name := range disabledNames {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	slices.Sort(names)

	for _, name := range names {
		row := mcpRow{Enabled: !slices.Contains(disabledNames, name), Name: name}
		data.Servers = append(data.Servers, row)
	}
	data.Overridden = s.sectionOverridden(main, "disabledMcpjsonServers", "enabledMcpjsonServers")

	s.renderFragment(w, tmplMcp, data)
}

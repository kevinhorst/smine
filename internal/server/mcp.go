package server

import (
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/peek"
	"github.com/kevinhorst/smine/internal/server/respond"
)

const tmplMcp = "_mcp.html"

const peekServerName = "peek-mcp"

const peekUpdateTimeout = 3 * time.Minute

// mcpData.OverriddenCount is the server count whose enabled state differs from the fragment's disabled list.
type mcpData struct {
	Error           string
	Overridden      bool
	OverriddenCount int
	Peek            *peekStatus
	Servers         []mcpRow
}

// mcpRow.IsOverride marks a server the fragment's disabled list puts on the other side.
type mcpRow struct {
	Command    string
	Enabled    bool
	IsOverride bool
	Name       string
}

type peekStatus struct {
	Expected        string
	Installed       string
	InstalledErr    string
	Running         string // empty = server-owned peek not reachable
	UpdateAvailable bool
}

func (s *Server) handleGetMCP(w http.ResponseWriter, r *http.Request) {
	main, _, err := s.loadBoth()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	s.renderMCP(r.Context(), w, main)
}

// handleToggleMCP treats disabledMcpjsonServers as the only MCP state and clears leftover sidecar keys.
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

	s.renderMCP(r.Context(), w, main)
}

// handleUpdatePeek leaves running sessions on their old stdio peek: per-agent stdio is the contract.
func (s *Server) handleUpdatePeek(w http.ResponseWriter, r *http.Request) {
	command, _, err := config.McpServer(s.claudeJsonPath, peekServerName)
	if err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), peekUpdateTimeout)
	defer cancel()
	if err := peek.UpdateBinary(ctx, command, s.peekExpectedVersion); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	if err := peek.Restart(ctx, s.peekSpawn); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	main, _, err := s.loadBoth()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	s.renderMCP(r.Context(), w, main)
}

func (s *Server) peekStatusFor(ctx context.Context, command string) *peekStatus {
	status := &peekStatus{Expected: s.peekExpectedVersion}

	installed, err := peek.InstalledVersion(ctx, command)
	if err != nil {
		status.InstalledErr = err.Error()
	}
	status.Installed = installed
	status.UpdateAvailable = err == nil && installed != status.Expected

	if id, err := peek.IdentityAt(s.peekSpawn.Addr()); err == nil {
		status.Running = id.Version
	}
	return status
}

// renderMCP degrades to the disabled list when ~/.claude.json is unreadable.
func (s *Server) renderMCP(ctx context.Context, w http.ResponseWriter, main *config.Settings) {
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

	fragmentDisabled, isFragmentKnown := s.fragmentDisabledMcp()
	for _, name := range names {
		command, _, _ := config.McpServer(s.claudeJsonPath, name)
		row := mcpRow{Command: command, Enabled: !slices.Contains(disabledNames, name), Name: name}
		row.IsOverride = isFragmentKnown && row.Enabled == slices.Contains(fragmentDisabled, name)
		if row.IsOverride {
			data.OverriddenCount++
		}
		data.Servers = append(data.Servers, row)
		if name == peekServerName && s.peekExpectedVersion != "" {
			data.Peek = s.peekStatusFor(ctx, command)
		}
	}
	data.Overridden = s.sectionOverridden(main, "disabledMcpjsonServers", "enabledMcpjsonServers")

	s.renderFragment(w, tmplMcp, data)
}

// fragmentDisabledMcp degrades a fragment load error to an unknown fragment: no row is then marked.
func (s *Server) fragmentDisabledMcp() ([]string, bool) {
	fragment, err := config.LoadFragment(s.claudeFragmentPath, s.markers())
	if err != nil {
		return nil, false
	}

	disabled, err := fragment.DisabledMcpjsonServers()
	if err != nil {
		return nil, false
	}
	return disabled, true
}

package server

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/server/respond"
)

const tmplPermissions = "_permissions.html"

const (
	listAllow         = "allow"
	listAsk           = "ask"
	listDisabledAllow = "disabledAllow"
	listDisabledAsk   = "disabledAsk"
)

type permissionRow struct {
	Enabled bool
	Index   int
	List    string
	Value   string
}

type permissionsData struct {
	Allow      []permissionRow
	Ask        []permissionRow
	Overridden bool
}

func (s *Server) handleGetPermissions(w http.ResponseWriter, _ *http.Request) {
	main, disabled, err := s.loadBoth()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	s.renderPermissions(w, main, disabled)
}

func (s *Server) handleTogglePermission(w http.ResponseWriter, r *http.Request) {
	list := r.PathValue("list")
	index, err := strconv.Atoi(r.PathValue("index"))
	if err != nil {
		respond.WithBadRequest("invalid index", w)
		return
	}

	main, disabled, err := s.loadBoth()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	mainPerms, err := main.Permissions()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	disabledPerms, err := disabled.Permissions()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	var source, dest *[]string
	switch list {
	case listAllow:
		source, dest = &mainPerms.Allow, &disabledPerms.Allow
	case listAsk:
		source, dest = &mainPerms.Ask, &disabledPerms.Ask
	case listDisabledAllow:
		source, dest = &disabledPerms.Allow, &mainPerms.Allow
	case listDisabledAsk:
		source, dest = &disabledPerms.Ask, &mainPerms.Ask
	default:
		respond.WithBadRequest("invalid list: "+list, w)
		return
	}

	if index >= len(*source) {
		respond.WithBadRequest("index out of range", w)
		return
	}

	entry := (*source)[index]
	*source = slices.Delete(*source, index, index+1)
	*dest = append(*dest, entry)

	if err := main.SetPermissions(mainPerms); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	if err := disabled.SetPermissions(disabledPerms); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	if err := s.saveBoth(main, disabled); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	w.Header().Set("HX-Trigger", "config-op")
	s.renderPermissions(w, main, disabled)
}

func (s *Server) renderPermissions(w http.ResponseWriter, main, disabled *config.Settings) {
	mainPerms, err := main.Permissions()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	disabledPerms, err := disabled.Permissions()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	var data permissionsData
	for index, value := range mainPerms.Allow {
		row := permissionRow{Enabled: true, Index: index, List: listAllow, Value: value}
		data.Allow = append(data.Allow, row)
	}
	for index, value := range disabledPerms.Allow {
		row := permissionRow{Index: index, List: listDisabledAllow, Value: value}
		data.Allow = append(data.Allow, row)
	}
	for index, value := range mainPerms.Ask {
		row := permissionRow{Enabled: true, Index: index, List: listAsk, Value: value}
		data.Ask = append(data.Ask, row)
	}
	for index, value := range disabledPerms.Ask {
		row := permissionRow{Index: index, List: listDisabledAsk, Value: value}
		data.Ask = append(data.Ask, row)
	}
	sortPermissionRows(data.Allow)
	sortPermissionRows(data.Ask)
	data.Overridden = s.sectionOverridden(main, "permissions")
	s.renderFragment(w, tmplPermissions, data)
}

// sortPermissionRows orders rows alphabetically so a toggle never moves a
// row; each row keeps its own storage List+Index for the toggle URL (D7).
func sortPermissionRows(rows []permissionRow) {
	slices.SortFunc(rows, func(left, right permissionRow) int {
		return strings.Compare(left.Value, right.Value)
	})
}

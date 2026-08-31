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

// A permission row's source relative to the repo fragment: present in both,
// present only live (added locally), or present only in the fragment.
const (
	permSourceShared   = "shared"
	permSourceLocal    = "local"
	permSourceRepoOnly = "repoOnly"
)

type permissionRow struct {
	Enabled bool
	Index   int
	List    string
	Value   string
	Source  string
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

// handleAddPermission puts a doc-surfaced rule into live permissions.allow: a
// rule parked in settings.disabled.json is moved back instead of duplicated,
// a rule already live is a no-op (raw.md D4). Responds with the chip in its
// new state so the doc page swaps in place.
func (s *Server) handleAddPermission(w http.ResponseWriter, r *http.Request) {
	rule := strings.TrimSpace(r.FormValue("rule"))
	if rule == "" || !isPermRule(rule) {
		respond.WithBadRequest("invalid permission rule", w)
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

	if !slices.Contains(mainPerms.Allow, rule) && !slices.Contains(mainPerms.Ask, rule) {
		if index := slices.Index(disabledPerms.Allow, rule); index >= 0 {
			disabledPerms.Allow = slices.Delete(disabledPerms.Allow, index, index+1)
		}
		if index := slices.Index(disabledPerms.Ask, rule); index >= 0 {
			disabledPerms.Ask = slices.Delete(disabledPerms.Ask, index, index+1)
		}
		mainPerms.Allow = append(mainPerms.Allow, rule)

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
	}

	state := permRuleState{
		Allow:         mainPerms.Allow,
		Ask:           mainPerms.Ask,
		DisabledAllow: disabledPerms.Allow,
		DisabledAsk:   disabledPerms.Ask,
	}
	s.renderFragment(w, tmplPermChip, permChip(rule, state))
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

	// One fragment load feeds both the per-rule marking and the section
	// badge; a load error degrades to no marking (never 500 over the badge).
	fragment, fragErr := config.LoadFragment(s.claudeFragmentPath)
	var fragPerms config.Permissions
	if fragErr == nil {
		fragPerms, _ = fragment.Permissions()
	}

	var data permissionsData
	data.Allow = permissionRows(mainPerms.Allow, disabledPerms.Allow, fragPerms.Allow, listAllow, listDisabledAllow, fragErr == nil)
	data.Ask = permissionRows(mainPerms.Ask, disabledPerms.Ask, fragPerms.Ask, listAsk, listDisabledAsk, fragErr == nil)
	sortPermissionRows(data.Allow)
	sortPermissionRows(data.Ask)
	if fragErr == nil {
		data.Overridden = claudeOverridden(main.Doc(), fragment.Doc(), []string{"permissions"})
	}
	s.renderFragment(w, tmplPermissions, data)
}

// permissionRows builds one list's rows: live rows (enabled), disabled rows
// (parked), then repo-only rows (in the fragment, absent from both). When the
// fragment is unknown, marking is skipped and no repo-only rows are added.
func permissionRows(mainList, disabledList, fragList []string, enabledList, disabledName string, known bool) []permissionRow {
	source := func(value string) string {
		if !known {
			return ""
		}
		if slices.Contains(fragList, value) {
			return permSourceShared
		}
		return permSourceLocal
	}
	var rows []permissionRow
	for index, value := range mainList {
		rows = append(rows, permissionRow{Enabled: true, Index: index, List: enabledList, Value: value, Source: source(value)})
	}
	for index, value := range disabledList {
		rows = append(rows, permissionRow{Index: index, List: disabledName, Value: value, Source: source(value)})
	}
	if known {
		for _, value := range fragList {
			if !slices.Contains(mainList, value) && !slices.Contains(disabledList, value) {
				rows = append(rows, permissionRow{Value: value, Source: permSourceRepoOnly})
			}
		}
	}
	return rows
}

// sortPermissionRows orders rows alphabetically so a toggle never moves a
// row; each row keeps its own storage List+Index for the toggle URL (D7).
func sortPermissionRows(rows []permissionRow) {
	slices.SortFunc(rows, func(left, right permissionRow) int {
		return strings.Compare(left.Value, right.Value)
	})
}

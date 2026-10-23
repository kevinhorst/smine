package server

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/server/catalog"
	"github.com/kevinhorst/smine/internal/server/respond"
)

const (
	pagePermissions     = "config-permissions"
	tmplPermissions     = "_permissions.html"
	tmplPermissionsPage = "permissions.html"
)

const (
	listAllow         = "allow"
	listAsk           = "ask"
	listDeny          = "deny"
	listDisabledAllow = "disabledAllow"
	listDisabledAsk   = "disabledAsk"
	listDisabledDeny  = "disabledDeny"
)

const (
	keyPermissions = "permissions"
	headMcp        = "mcp"
	mcpRulePrefix  = "mcp__"
)

// A row's source against the repo fragment: byte-equal, live only, same id but different content, or fragment only.
const (
	sourceShared   = "shared"
	sourceLocal    = "local"
	sourceChanged  = "changed"
	sourceRepoOnly = "repoOnly"
)

// permissionHead is one tool head's rows inside a list, rows sorted by value; the counts feed the collapsed summary's badges.
type permissionHead struct {
	Head     string
	Local    int
	Parked   int
	RepoOnly int
	Rows     []permissionRow
}

// permissionList is one of allow, ask and deny: Key names the live list for the add form, Name is the heading.
type permissionList struct {
	Heads []permissionHead
	Key   string
	Name  string
}

type permissionRow struct {
	Enabled bool
	Index   int
	List    string
	Value   string
	Source  string
}

// permissionsData.OverriddenCount is the local and repo-only row count, the section badge's only source: keys outside the three lists (defaultMode) are the Claude tab's.
type permissionsData struct {
	IsFragment      bool // a mutation answer, which carries the overridden badge out of band
	Lists           []permissionList
	OverriddenCount int
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
	if err != nil || index < 0 {
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
	case listDeny:
		source, dest = &mainPerms.Deny, &disabledPerms.Deny
	case listDisabledAllow:
		source, dest = &disabledPerms.Allow, &mainPerms.Allow
	case listDisabledAsk:
		source, dest = &disabledPerms.Ask, &mainPerms.Ask
	case listDisabledDeny:
		source, dest = &disabledPerms.Deny, &mainPerms.Deny
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

	s.renderPermissions(w, main, disabled)
}

// handleAddPermission moves a parked rule back instead of duplicating it.
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
	}

	state := permRuleState{
		Allow:         mainPerms.Allow,
		Ask:           mainPerms.Ask,
		DisabledAllow: disabledPerms.Allow,
		DisabledAsk:   disabledPerms.Ask,
	}
	s.renderFragment(w, tmplPermChip, permChip(rule, state))
}

func (s *Server) handleAddPermissionItem(w http.ResponseWriter, r *http.Request) {
	list := r.PathValue("list")
	value := strings.TrimSpace(r.FormValue("value"))
	isDisabled, key, ok := listStorage(list)
	if !ok || isDisabled {
		respond.WithBadRequest("invalid list: "+list, w)
		return
	}
	if value == "" {
		respond.WithBadRequest("empty rule", w)
		return
	}

	main, disabled, err := s.loadBoth()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	path := []string{keyPermissions, key}
	rules, err := main.Doc().List(path)
	if err != nil {
		respond.WithBadRequest("existing value is not an array", w)
		return
	}
	if containsRule(rules, value) {
		respond.WithBadRequest("rule already in "+key, w)
		return
	}

	quoted, _ := json.Marshal(value)
	if err := main.Doc().SetList(path, append(rules, quoted)); err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}
	if err := config.Save(s.settingsPath, main, s.trash); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	s.renderPermissions(w, main, disabled)
}

func (s *Server) handleRemovePermission(w http.ResponseWriter, r *http.Request) {
	list := r.PathValue("list")
	index, err := strconv.Atoi(r.PathValue("index"))
	if err != nil || index < 0 {
		respond.WithBadRequest("invalid index", w)
		return
	}

	isDisabled, key, ok := listStorage(list)
	if !ok {
		respond.WithBadRequest("invalid list: "+list, w)
		return
	}

	main, disabled, err := s.loadBoth()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	target := main
	if isDisabled {
		target = disabled
	}
	path := []string{keyPermissions, key}
	rules, err := target.Doc().List(path)
	if err != nil {
		respond.WithBadRequest("existing value is not an array", w)
		return
	}
	if index >= len(rules) {
		respond.WithNotFound("rule not found", w)
		return
	}

	if err := target.Doc().SetList(path, slices.Delete(rules, index, index+1)); err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}
	if err := s.saveBoth(main, disabled); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	s.renderPermissions(w, main, disabled)
}

func (s *Server) renderPermissions(w http.ResponseWriter, main, disabled *config.Settings) {
	data, err := s.permissionsView(main, disabled)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	data.IsFragment = true
	s.renderFragment(w, tmplPermissions, data)
}

// permissionsPage is the Permissions tab: the lists plus the Claude target's sync widget.
type permissionsPage struct {
	permissionsData
	FragmentMissing bool
	FragmentPath    string
	Page            string
	Target          string
	Title           string
}

// handlePermissionsPage renders the Permissions tab.
func (s *Server) handlePermissionsPage(w http.ResponseWriter, _ *http.Request) {
	if !s.records.has(s.machine, components.Gate) {
		s.renderComponentOff(w, pagePermissions, "Permissions", components.Gate)
		return
	}

	main, disabled, err := s.loadBoth()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	data, err := s.permissionsView(main, disabled)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	_, statErr := os.Stat(s.claudeFragmentPath)
	page := permissionsPage{
		permissionsData: data,
		FragmentMissing: statErr != nil,
		FragmentPath:    s.claudeFragmentPath,
		Page:            pagePermissions,
		Target:          catalog.TargetClaude,
		Title:           "Permissions",
	}
	s.renderFragment(w, tmplPermissionsPage, page)
}

// permissionsView classifies the live and parked rules of the three lists against the fragment.
func (s *Server) permissionsView(main, disabled *config.Settings) (permissionsData, error) {
	var data permissionsData
	mainPerms, err := main.Permissions()
	if err != nil {
		return data, fmt.Errorf("Server.permissionsView: %w", err)
	}

	disabledPerms, err := disabled.Permissions()
	if err != nil {
		return data, fmt.Errorf("Server.permissionsView: %w", err)
	}

	// A fragment load error degrades to no marking, never a 500 over the badge.
	fragment, fragErr := config.LoadFragment(s.claudeFragmentPath, s.markers())
	var fragPerms config.Permissions
	if fragErr == nil {
		fragPerms, _ = fragment.Permissions()
	}

	allowRows := permissionRows(mainPerms.Allow, disabledPerms.Allow, fragPerms.Allow, listAllow, listDisabledAllow, fragErr == nil)
	askRows := permissionRows(mainPerms.Ask, disabledPerms.Ask, fragPerms.Ask, listAsk, listDisabledAsk, fragErr == nil)
	denyRows := permissionRows(mainPerms.Deny, disabledPerms.Deny, fragPerms.Deny, listDeny, listDisabledDeny, fragErr == nil)
	data.Lists = []permissionList{
		{Heads: headGroups(allowRows), Key: listAllow, Name: "Allow"},
		{Heads: headGroups(askRows), Key: listAsk, Name: "Ask"},
		{Heads: headGroups(denyRows), Key: listDeny, Name: "Deny"},
	}
	for _, list := range data.Lists {
		for _, head := range list.Heads {
			data.OverriddenCount += head.Local + head.RepoOnly
		}
	}
	return data, nil
}

// containsRule reports whether a raw string list holds value.
func containsRule(rules []json.RawMessage, value string) bool {
	for _, rule := range rules {
		var text string
		if err := json.Unmarshal(rule, &text); err != nil {
			continue
		}
		if text == value {
			return true
		}
	}
	return false
}

// headGroups sorts rows by value and folds them into one group per tool head, heads alphabetical.
func headGroups(rows []permissionRow) []permissionHead {
	sortPermissionRows(rows)
	byHead := make(map[string][]permissionRow)
	for _, row := range rows {
		head := ruleHead(row.Value)
		byHead[head] = append(byHead[head], row)
	}

	heads := make([]permissionHead, 0, len(byHead))
	for _, head := range slices.Sorted(maps.Keys(byHead)) {
		heads = append(heads, newPermissionHead(head, byHead[head]))
	}
	return heads
}

func newPermissionHead(head string, rows []permissionRow) permissionHead {
	group := permissionHead{Head: head, Rows: rows}
	for index := range rows {
		row := &rows[index]
		switch row.Source {
		case sourceLocal:
			group.Local++
		case sourceRepoOnly:
			group.RepoOnly++
		case sourceShared, sourceChanged, "":
		}
		if !row.Enabled && row.Source != sourceRepoOnly {
			group.Parked++
		}
	}
	return group
}

// listStorage resolves a list name to the file it lives in and the settings key under permissions.
func listStorage(list string) (isDisabled bool, key string, ok bool) {
	switch list {
	case listAllow, listAsk, listDeny:
		return false, list, true
	case listDisabledAllow:
		return true, listAllow, true
	case listDisabledAsk:
		return true, listAsk, true
	case listDisabledDeny:
		return true, listDeny, true
	}
	return false, "", false
}

func permissionRows(mainList, disabledList, fragList []string, enabledList, disabledName string, known bool) []permissionRow {
	source := func(value string) string {
		if !known {
			return ""
		}
		if slices.Contains(fragList, value) {
			return sourceShared
		}
		return sourceLocal
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
				rows = append(rows, permissionRow{Value: value, Source: sourceRepoOnly})
			}
		}
	}
	return rows
}

// ruleHead is the tool a rule names: the text before its first "(", "mcp" for an MCP rule, the whole rule otherwise.
func ruleHead(value string) string {
	if strings.HasPrefix(value, mcpRulePrefix) {
		return headMcp
	}

	head, _, _ := strings.Cut(value, "(")
	return head
}

// sortPermissionRows sorts so a toggle never moves a row; rows keep their storage index for the URL.
func sortPermissionRows(rows []permissionRow) {
	slices.SortFunc(rows, func(left, right permissionRow) int {
		return strings.Compare(left.Value, right.Value)
	})
}

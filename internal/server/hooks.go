package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/server/respond"
	"github.com/kevinhorst/smine/internal/trash"
)

const (
	// hookDiffContext is the unchanged lines kept around each change of a hooks diff.
	hookDiffContext = 2
	// otherHooksCard is the card key of the groups no component fragment declares, idless ones included.
	otherHooksCard = "other"
	pageHooks      = "config-hooks"
	tmplHooks      = "hooks.html"
	tmplHooksCard  = "_hooks_card.html"
)

// The ?state= filter values of the Hooks page.
const (
	hookStateAll      = "all"
	hookStateChanged  = "changed"
	hookStateDisabled = "disabled"
	hookStateLocal    = "local"
)

var hookStates = []string{hookStateAll, hookStateDisabled, hookStateLocal, hookStateChanged}

type hookView struct {
	Command    string
	Content    string
	Name       string
	ScriptPath string
}

// hookFragment is the repo fragment's hook groups; Known is false when the fragment could not be loaded, which marks nothing.
type hookFragment struct {
	groups  map[string][]config.HookGroup
	known   bool
	matched map[string]bool
}

func newHookFragment(groups map[string][]config.HookGroup, known bool) *hookFragment {
	return &hookFragment{groups: groups, known: known, matched: map[string]bool{}}
}

// classify is shared for a fragment-equal group (its park reason ignored), changed for one sharing the id, local otherwise; a match is remembered for repoOnlyRows.
func (f *hookFragment) classify(event string, group *config.HookGroup) (string, []diffLine) {
	if !f.known {
		return "", nil
	}

	unreasoned := *group
	unreasoned.Reason = ""
	live := hookGroupJSON(&unreasoned)
	var sameId *config.HookGroup
	for index := range f.groups[event] {
		candidate := &f.groups[event][index]
		if hookGroupJSON(candidate) == live {
			f.matched[hookMatchKey(event, candidate)] = true
			return sourceShared, nil
		}
		if group.Id != "" && candidate.Id == group.Id {
			sameId = candidate
		}
	}
	if sameId == nil {
		return sourceLocal, nil
	}

	f.matched[hookMatchKey(event, sameId)] = true
	return sourceChanged, compactDiff(diffLines(hookGroupJSON(sameId), live), hookDiffContext)
}

// repoOnlyRows lists the fragment groups no live or parked group matched, after every classify call.
func (f *hookFragment) repoOnlyRows(owners map[string]string) []hookGroupInfo {
	var rows []hookGroupInfo
	for _, event := range slices.Sorted(maps.Keys(f.groups)) {
		for index := range f.groups[event] {
			group := &f.groups[event][index]
			if f.matched[hookMatchKey(event, group)] {
				continue
			}

			row := newHookGroupInfo(false, event, index, group, owners)
			row.Source = sourceRepoOnly
			rows = append(rows, row)
		}
	}
	return rows
}

type hookGroupInfo struct {
	Command string
	Diff    []diffLine
	Enabled bool
	Event   string
	Index   int
	Matcher string
	Name    string
	Owner   string // title of the component whose fragment declares the group's id, empty for idless groups
	Source  string
	Views   []hookView
}

func newHookGroupInfo(enabled bool, event string, index int, group *config.HookGroup, owners map[string]string) hookGroupInfo {
	matcher := ""
	if group.Matcher != nil {
		matcher = *group.Matcher
	}
	row := hookGroupInfo{
		Command: summarizeHookGroup(group),
		Enabled: enabled,
		Event:   event,
		Index:   index,
		Matcher: matcher,
		Name:    hookLabel(group),
		Owner:   owners[group.Id],
		Views:   hookViews(group),
	}
	return row
}

// hooksCard is one component: its selection state and its hook groups with their toggles; IsOther marks the Other hooks card.
type hooksCard struct {
	Component  components.Component
	Groups     []hookGroupInfo
	IsOpen     bool // rendered expanded: the first card, or every card under a filter
	IsOther    bool
	IsSelected bool
}

// newOtherHooksCard collects the live and parked groups no component fragment declares.
func newOtherHooksCard(groups []hookGroupInfo) hooksCard {
	card := hooksCard{IsOther: true}
	for _, group := range groups {
		isUnowned := group.Owner == ""
		if isUnowned && group.Source != sourceRepoOnly {
			card.Groups = append(card.Groups, group)
		}
	}
	return card
}

// ParkedCount is the number of parked groups; with TogglableCount it is the header toggle's state.
func (c *hooksCard) ParkedCount() int {
	count := 0
	for _, group := range c.Groups {
		if group.matchesState(hookStateDisabled) {
			count++
		}
	}
	return count
}

// TogglableCount is the number of live and parked groups, the repo-only ones excluded.
func (c *hooksCard) TogglableCount() int {
	count := 0
	for _, group := range c.Groups {
		if group.Source != sourceRepoOnly {
			count++
		}
	}
	return count
}

// matchGroups keeps the groups whose name, event, matcher or command holds query and whose side matches state.
func (c *hooksCard) matchGroups(query, state string) {
	needle := strings.ToLower(query)
	c.Groups = slices.DeleteFunc(c.Groups, func(group hookGroupInfo) bool {
		if !group.matchesState(state) {
			return true
		}
		haystack := strings.ToLower(group.Name + "\n" + group.Event + "\n" + group.Matcher + "\n" + group.Command)
		return !strings.Contains(haystack, needle)
	})
}

// matchesState reports the group under one of the state filters.
func (g *hookGroupInfo) matchesState(state string) bool {
	switch state {
	case hookStateDisabled:
		return !g.Enabled && g.Source != sourceRepoOnly
	case hookStateLocal:
		return g.Source == sourceLocal
	case hookStateChanged:
		return g.Source == sourceChanged
	}
	return true
}

// IsUninstalled marks a component card whose component the record lacks: its groups stay parked until the component is installed.
func (c *hooksCard) IsUninstalled() bool {
	return !c.IsOther && !c.IsSelected
}

// Key is the card's route segment and DOM id suffix.
func (c *hooksCard) Key() string {
	if c.IsOther {
		return otherHooksCard
	}
	return string(c.Component.ID)
}

// Title is the card's heading.
func (c *hooksCard) Title() string {
	if c.IsOther {
		return "Other hooks"
	}
	return c.Component.Title
}

// hooksPage is the page: one card per component with hook groups, filtered by q and state.
type hooksPage struct {
	Cards  []hooksCard
	Page   string
	Query  string
	State  string
	States []string
	Title  string
}

// isFiltered reports a query or a state other than all.
func (p *hooksPage) isFiltered() bool {
	return p.Query != "" || p.State != hookStateAll
}

// handleHooksPage renders every card; ?q= filters groups by name, event, matcher and command, ?state= by side.
func (s *Server) handleHooksPage(w http.ResponseWriter, r *http.Request) {
	record, err := s.records.load(s.machine)
	if err != nil {
		log.Printf("Server.handleHooksPage: %v", err)
	}

	page := hooksPage{
		Page:   pageHooks,
		Query:  strings.TrimSpace(r.URL.Query().Get("q")),
		State:  r.URL.Query().Get("state"),
		States: hookStates,
		Title:  "Hooks",
	}
	if !slices.Contains(hookStates, page.State) {
		page.State = hookStateAll
	}
	page.Cards, err = s.hooksCards(record, &page)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	s.renderFragment(w, tmplHooks, page)
}

// handleToggleComponentHooks parks every live group a component's fragment owns (enabled=true) or pops every parked one (enabled=false), in one write.
func (s *Server) handleToggleComponentHooks(w http.ResponseWriter, r *http.Request) {
	id, ok := hookCardComponent(w, r)
	if !ok {
		return
	}
	component, _ := components.Lookup(id)

	enabled, err := strconv.ParseBool(r.URL.Query().Get("enabled"))
	if err != nil {
		respond.WithBadRequest("invalid enabled state", w)
		return
	}
	if !enabled && s.isUninstalledCard(w, string(id)) {
		return
	}

	if !s.repoLocks.TryAcquire(syncLockKey) {
		respond.WithConflict("an operation is already running", w)
		return
	}
	defer s.repoLocks.Release(syncLockKey)

	main, err := config.Load(s.settingsPath)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	mainHooks, err := main.Hooks()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	if mainHooks == nil {
		mainHooks = make(map[string][]config.HookGroup)
	}

	ids := s.componentHookIds(component.Title)
	var peeked []parkedSlot
	if enabled {
		err = s.parkGroups(ids, mainHooks)
	} else {
		peeked, err = s.unparkGroups(ids, mainHooks)
	}
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	if !s.writeHooks(main, mainHooks, w) {
		return
	}
	if err := s.dropUnparked(peeked); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	s.renderHooksCard(w, string(id))
}

// componentHookIds lists the fragment hook ids the component titled title owns.
func (s *Server) componentHookIds(title string) []string {
	var ids []string
	for id, owner := range s.hookOwners() {
		if owner == title {
			ids = append(ids, id)
		}
	}
	return ids
}

func (s *Server) handleRemoveHook(w http.ResponseWriter, r *http.Request) {
	card, ok := hookCardKey(w, r)
	if !ok {
		return
	}

	event := r.PathValue("event")
	index, err := strconv.Atoi(r.PathValue("index"))
	if err != nil || index < 0 {
		respond.WithBadRequest("invalid index", w)
		return
	}

	// enabled: required — it names the row's side, settings.json or the sidecar
	enabled, err := strconv.ParseBool(r.URL.Query().Get("enabled"))
	if err != nil {
		respond.WithBadRequest("invalid enabled state", w)
		return
	}

	if !s.repoLocks.TryAcquire(syncLockKey) {
		respond.WithConflict("an operation is already running", w)
		return
	}
	defer s.repoLocks.Release(syncLockKey)

	main, err := config.Load(s.settingsPath)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	if !enabled {
		group, ok, err := s.disabledHooks.Peek(event, index)
		if err != nil {
			respond.WithInternalServerError(err, w)
			return
		}
		if !ok {
			respond.WithNotFound("hook not found", w)
			return
		}

		entry := &trash.Entry{Action: "hooks remove", Kind: trash.KindRemoved, Path: install.ParkedPath(s.machine) + "#" + event + "/" + groupName(&group), Reason: "removed from the Hooks tab (parked row)"}
		if err := s.trash.Note(entry); err != nil {
			respond.WithInternalServerError(err, w)
			return
		}
		if err := s.disabledHooks.Drop(event, index); err != nil {
			respond.WithInternalServerError(err, w)
			return
		}
		s.renderHooksCard(w, card)
		return
	}

	mainHooks, err := main.Hooks()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	group, ok := takeHookGroup(mainHooks, event, index)
	if !ok {
		respond.WithNotFound("hook not found", w)
		return
	}
	entry := &trash.Entry{Action: "hooks remove", Kind: trash.KindRemoved, Path: s.settingsPath + "#hooks/" + event + "/" + groupName(&group), Reason: "removed from the Hooks tab (live row)"}
	if err := s.trash.Note(entry); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	s.saveHooks(card, main, mainHooks, w)
}

func (s *Server) handleToggleHook(w http.ResponseWriter, r *http.Request) {
	card, ok := hookCardKey(w, r)
	if !ok {
		return
	}

	event := r.PathValue("event")
	index, err := strconv.Atoi(r.PathValue("index"))
	if err != nil || index < 0 {
		respond.WithBadRequest("invalid index", w)
		return
	}

	// enabled: required — it names the row's current side; the toggle moves the group to the other side (D33)
	enabled, err := strconv.ParseBool(r.URL.Query().Get("enabled"))
	if err != nil {
		respond.WithBadRequest("invalid enabled state", w)
		return
	}
	if !enabled && s.isUninstalledCard(w, card) {
		return
	}

	if !s.repoLocks.TryAcquire(syncLockKey) {
		respond.WithConflict("an operation is already running", w)
		return
	}
	defer s.repoLocks.Release(syncLockKey)

	main, err := config.Load(s.settingsPath)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	mainHooks, err := main.Hooks()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	if mainHooks == nil {
		mainHooks = make(map[string][]config.HookGroup)
	}

	if !enabled {
		// disabled row → re-enable: peek the sidecar, write settings.json, then drop from the sidecar
		group, ok, err := s.disabledHooks.Peek(event, index)
		if err != nil {
			respond.WithInternalServerError(err, w)
			return
		}
		if !ok {
			respond.WithNotFound("hook not found", w)
			return
		}
		mainHooks[event] = append(mainHooks[event], group)
		if !s.writeHooks(main, mainHooks, w) {
			return
		}
		if err := s.disabledHooks.Drop(event, index); err != nil {
			respond.WithInternalServerError(err, w)
			return
		}
		s.renderHooksCard(w, card)
		return
	}

	// enabled row → disable: remove from settings.json, park in the sidecar
	group, ok := takeHookGroup(mainHooks, event, index)
	if !ok {
		respond.WithNotFound("hook not found", w)
		return
	}
	if err := s.disabledHooks.Add(event, group); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	s.saveHooks(card, main, mainHooks, w)
}

// hookGroupRows classifies every live and parked group against the fragment, owners attached.
func (s *Server) hookGroupRows() ([]hookGroupInfo, error) {
	main, err := config.Load(s.settingsPath)
	if err != nil {
		return nil, fmt.Errorf("Server.hookGroupRows: %w", err)
	}

	mainHooks, err := main.Hooks()
	if err != nil {
		return nil, fmt.Errorf("Server.hookGroupRows: %w", err)
	}

	parked, err := s.disabledHooks.Snapshot()
	if err != nil {
		return nil, fmt.Errorf("Server.hookGroupRows: %w", err)
	}
	return hookRows(mainHooks, parked, s.loadHookFragment(), s.hookOwners()), nil
}

// hookOwners maps every fragment hook id to its component's title; an unloadable fragment owns nothing.
func (s *Server) hookOwners() map[string]string {
	owners := map[string]string{}
	for _, component := range components.Catalog() {
		fragment, err := config.LoadComponentFragment(s.claudeFragmentPath, string(component.ID), s.markers())
		if err != nil {
			continue
		}

		ids, err := fragment.HookIds()
		if err != nil {
			continue
		}
		for _, id := range ids {
			owners[id] = component.Title
		}
	}
	return owners
}

// newHooksCard builds one component's card from the group rows its fragment owns.
func newHooksCard(record install.Record, component *components.Component, groups []hookGroupInfo) hooksCard {
	card := hooksCard{
		Component:  *component,
		IsSelected: slices.Contains(record.Components, component.ID),
	}
	for _, group := range groups {
		if group.Owner == component.Title {
			card.Groups = append(card.Groups, group)
		}
	}
	return card
}

// hooksCards lists one card per catalog component with hook groups, then the Other hooks card; a filter keeps the matching groups and opens every card left, else only the first is open.
func (s *Server) hooksCards(record install.Record, page *hooksPage) ([]hooksCard, error) {
	groups, err := s.hookGroupRows()
	if err != nil {
		return nil, fmt.Errorf("Server.hooksCards: %w", err)
	}

	var cards []hooksCard
	for _, component := range components.Catalog() {
		cards = append(cards, newHooksCard(record, &component, groups))
	}
	cards = append(cards, newOtherHooksCard(groups))

	if page.isFiltered() {
		for index := range cards {
			cards[index].matchGroups(page.Query, page.State)
		}
	}
	cards = slices.DeleteFunc(cards, func(card hooksCard) bool { return len(card.Groups) == 0 })
	for index := range cards {
		cards[index].IsOpen = page.isFiltered() || index == 0
	}
	return cards, nil
}

// loadHookFragment degrades a fragment load error to an unknown fragment: the rows then carry no source.
func (s *Server) loadHookFragment() *hookFragment {
	fragment, err := config.LoadFragment(s.claudeFragmentPath, s.markers())
	if err != nil {
		return newHookFragment(nil, false)
	}

	groups, err := fragment.Hooks()
	if err != nil {
		return newHookFragment(nil, false)
	}
	return newHookFragment(groups, true)
}

// renderHooksCard re-renders one card, a component's or the Other hooks card, open as a mutation leaves it.
func (s *Server) renderHooksCard(w http.ResponseWriter, key string) {
	groups, err := s.hookGroupRows()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	if key == otherHooksCard {
		card := newOtherHooksCard(groups)
		card.IsOpen = true
		s.renderFragment(w, tmplHooksCard, &card)
		return
	}

	component, ok := components.Lookup(components.ID(key))
	if !ok {
		respond.WithNotFound("unknown component", w)
		return
	}

	record, err := s.records.load(s.machine)
	if err != nil {
		log.Printf("Server.renderHooksCard: %v", err)
	}

	card := newHooksCard(record, &component, groups)
	card.IsOpen = true
	s.renderFragment(w, tmplHooksCard, &card)
}

// saveHooks persists settings.json and re-renders the card named by key.
func (s *Server) saveHooks(key string, main *config.Settings, mainHooks map[string][]config.HookGroup, w http.ResponseWriter) {
	if !s.writeHooks(main, mainHooks, w) {
		return
	}
	s.renderHooksCard(w, key)
}

// writeHooks persists settings.json with mainHooks and answers the error; the caller renders on true.
func (s *Server) writeHooks(main *config.Settings, mainHooks map[string][]config.HookGroup, w http.ResponseWriter) bool {
	if err := main.SetHooks(mainHooks); err != nil {
		respond.WithInternalServerError(err, w)
		return false
	}
	if err := config.Save(s.settingsPath, main, s.trash); err != nil {
		respond.WithInternalServerError(err, w)
		return false
	}
	return true
}

func eventHookRows(enabled bool, event string, fragment *hookFragment, groups []config.HookGroup, owners map[string]string) []hookGroupInfo {
	rows := make([]hookGroupInfo, 0, len(groups))
	for index := range groups {
		group := &groups[index]
		row := newHookGroupInfo(enabled, event, index, group, owners)
		row.Source, row.Diff = fragment.classify(event, group)
		rows = append(rows, row)
	}
	return rows
}

// groupName names a group for a journal line: its smineName, else its id, else its first command.
func groupName(group *config.HookGroup) string {
	switch {
	case group.Name != "":
		return group.Name
	case group.Id != "":
		return group.Id
	case len(group.Hooks) > 0:
		return group.Hooks[0].Command
	}
	return "(unnamed group)"
}

// hookCardComponent reads the {component} path value; an unknown id answers 404.
func hookCardComponent(w http.ResponseWriter, r *http.Request) (components.ID, bool) {
	id := components.ID(r.PathValue("component"))
	if _, ok := components.Lookup(id); !ok {
		respond.WithNotFound("unknown component", w)
		return "", false
	}
	return id, true
}

// isUninstalledCard answers 409 when key is a component the record lacks: its hook binaries are not on the machine, so a group of it must not go live.
func (s *Server) isUninstalledCard(w http.ResponseWriter, key string) bool {
	if key == otherHooksCard {
		return false
	}
	record, err := s.records.load(s.machine)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return true
	}
	if slices.Contains(record.Components, components.ID(key)) {
		return false
	}
	component, _ := components.Lookup(components.ID(key))
	respond.WithConflict("install "+component.Title+" first: its hook binaries are not on this machine", w)
	return true
}

// hookCardKey reads the {component} path value of a group route: the Other hooks card or a known component, else 404.
func hookCardKey(w http.ResponseWriter, r *http.Request) (string, bool) {
	key := r.PathValue("component")
	if key == otherHooksCard {
		return key, true
	}
	if _, ok := components.Lookup(components.ID(key)); !ok {
		respond.WithNotFound("unknown component", w)
		return "", false
	}
	return key, true
}

// hookGroupJSON is a group's canonical pretty JSON: the identity rows are compared by and the text their diff shows.
func hookGroupJSON(group *config.HookGroup) string {
	raw, err := json.Marshal(group)
	if err != nil {
		return ""
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return string(raw)
	}
	pretty, err := json.MarshalIndent(canonicalJSON(value), "", "  ")
	if err != nil {
		return string(raw)
	}
	return string(pretty)
}

// hookMatchKey identifies a fragment group by its id, else by its canonical JSON.
func hookMatchKey(event string, group *config.HookGroup) string {
	if group.Id != "" {
		return event + "\x00" + group.Id
	}
	return event + "\x00" + hookGroupJSON(group)
}

// hookRows classifies the live and parked rows against the fragment, then appends the fragment groups nothing matched.
func hookRows(mainHooks, disabledHooks map[string][]config.HookGroup, fragment *hookFragment, owners map[string]string) []hookGroupInfo {
	rows := sideHookRows(true, fragment, mainHooks, owners)
	rows = append(rows, sideHookRows(false, fragment, disabledHooks, owners)...)
	rows = append(rows, fragment.repoOnlyRows(owners)...)
	slices.SortStableFunc(rows, func(left, right hookGroupInfo) int {
		if c := strings.Compare(left.Event, right.Event); c != 0 {
			return c
		}
		if left.Enabled != right.Enabled {
			if left.Enabled {
				return -1
			}
			return 1
		}
		return strings.Compare(left.Name, right.Name)
	})
	return rows
}

// hookLabel is the group's smine name, else its hook names cut to the name cap.
func hookLabel(hookGroup *config.HookGroup) string {
	if hookGroup.Name != "" {
		return hookGroup.Name
	}
	return cutRunes(summarizeHookNames(hookGroup), config.MaxHookNameRunes)
}

// hookText is what a hook runs: its command, or its url for an http hook.
func hookText(hook *config.Hook) string {
	if hook.Command != "" {
		return hook.Command
	}
	return hook.Url
}

// cutRunes shortens text to at most limit runes, the last one an ellipsis when something was cut.
func cutRunes(text string, limit int) string {
	if utf8.RuneCountInString(text) <= limit {
		return text
	}

	runes := []rune(text)
	return string(runes[:limit-1]) + "…"
}

// takeHookGroup removes and returns the group at event/index, dropping an emptied event key.
func takeHookGroup(hooks map[string][]config.HookGroup, event string, index int) (config.HookGroup, bool) {
	groups, ok := hooks[event]
	if !ok || index >= len(groups) {
		return config.HookGroup{}, false
	}

	group := groups[index]
	hooks[event] = slices.Delete(groups, index, index+1)
	if len(hooks[event]) == 0 {
		delete(hooks, event)
	}
	return group, true
}

func sideHookRows(enabled bool, fragment *hookFragment, hooks map[string][]config.HookGroup, owners map[string]string) []hookGroupInfo {
	var rows []hookGroupInfo
	for _, event := range slices.Sorted(maps.Keys(hooks)) {
		rows = append(rows, eventHookRows(enabled, event, fragment, hooks[event], owners)...)
	}
	return rows
}

func summarizeHookGroup(hookGroup *config.HookGroup) string {
	if len(hookGroup.Hooks) == 0 {
		return "(empty)"
	}

	commands := make([]string, 0, len(hookGroup.Hooks))
	for _, hook := range hookGroup.Hooks {
		commands = append(commands, hookText(&hook))
	}
	return strings.Join(commands, " && ")
}

func hookViews(hookGroup *config.HookGroup) []hookView {
	views := make([]hookView, 0, len(hookGroup.Hooks))
	for _, hook := range hookGroup.Hooks {
		view := hookView{
			Command: hookText(&hook),
			Name:    hookDisplayName(&hook),
		}
		view.ScriptPath, view.Content = resolveScript(hook.Command)
		views = append(views, view)
	}
	return views
}

func hookDisplayName(hook *config.Hook) string {
	if hook.Name != "" {
		return hook.Name
	}
	return hookText(hook)
}

// summarizeHookNames mirrors summarizeHookGroup but prefers each hook's name.
func summarizeHookNames(hookGroup *config.HookGroup) string {
	if len(hookGroup.Hooks) == 0 {
		return "(empty)"
	}

	names := make([]string, 0, len(hookGroup.Hooks))
	for _, hook := range hookGroup.Hooks {
		names = append(names, hookDisplayName(&hook))
	}
	return strings.Join(names, " && ")
}

// maxHookScriptBytes separates a few-KiB hook script from a multi-MiB compiled hook binary.
const maxHookScriptBytes = 256 << 10

// resolveScript resolves binaries to nothing: the page shows a script's source, never a binary's bytes.
func resolveScript(command string) (path, content string) {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return "", ""
	}

	token := fields[0]
	if strings.HasPrefix(token, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", ""
		}

		token = filepath.Join(home, token[2:])
	}
	if !filepath.IsAbs(token) {
		return "", ""
	}

	info, err := os.Stat(token)
	if err != nil || info.IsDir() {
		return "", ""
	}
	if info.Size() > maxHookScriptBytes {
		return "", ""
	}

	data, err := os.ReadFile(token)
	if err != nil {
		return "", ""
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return "", ""
	}

	return token, string(data)
}

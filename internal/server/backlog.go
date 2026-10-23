package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/kevinhorst/smine/internal/backlog"
	"github.com/kevinhorst/smine/internal/peek"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/server/respond"
	"github.com/kevinhorst/smine/internal/sessions"
	"github.com/kevinhorst/smine/internal/skills"
)

const (
	pageBacklog         = "backlog"
	tmplBacklog         = "backlog.html"
	tmplBacklogItem     = "backlog_item.html"
	tmplBacklogActivity = "_backlog_activity"
	tmplBacklogCellsOOB = "_backlog_cells_oob.html"
	tmplBacklogRow      = "_backlog_row.html"
	tmplBacklogRuns     = "_backlog_runs.html"

	backlogFamily     = "backlog"
	backlogStoreFile  = "backlog.json"
	backlogTabArchive = "archive"
	historyLimitItem  = 50
	maxBoardRuns      = 2
	maxItemSessions   = 20
	priCritical       = "critical"
	priHigh           = "high"
	priLow            = "low"
	priMedium         = "medium"
)

// sessionPointerRe accepts a session id or an eight-plus hex prefix, with or without the "session " word the skill writes, and an optional "#<finding index>".
var sessionPointerRe = regexp.MustCompile(`^(?:session )?([0-9a-f][0-9a-f-]{7,})(?:#(\d+))?$`)

type backlogDayView struct {
	IsOpen bool
	Items  []backlogItemView
	Key    string
	Label  string
}

type backlogFilterCount struct {
	Count int
	Value string
}

type backlogItemPage struct {
	Epics    []string
	Item     backlogItemView
	Page     string
	Row      *routineView
	Stage    string // the stage the run records on its branch; "" = unchanged
	StoreRel string
	Title    string
}

type backlogItemView struct {
	Deps           []depView
	Description    string // the description without its Done when line
	DoneWhen       string // the Done when line, empty when the description has none
	Evidence       []evidenceLink
	IsActivityOOB  bool // the Activity cell answers a dispatch or a route write and swaps out of band
	IsLive         bool // the item's newest run row is the routine's live run
	IsRouteListed  bool // the route's skill is one a backlog routine runs, so the route select shows it
	IsRouteOOB     bool // the route cell answers a route write and swaps out of band
	IsStatusOOB    bool // the status cell answers a status write and swaps out of band
	IsTitleOOB     bool // the title cell, holding the priority select, answers a priority write and swaps out of band
	Item           backlog.Item
	LastRun        *routines.RunResult // newest run row carrying the item id
	MinedByURL     string              // the peek page of the run session that mined the row
	PollSince      int64               // unix time of a dispatch whose run has not started yet; the row shows "starting" and polls
	Priorities     []string
	RouteRest      string            // Route after the skill token, e.g. " change"
	Routine        *routines.Routine // the routine of the item's newest run, else the first of Routines; the dispatch select's default
	RoutineMissing bool
	Routines       []routines.Routine // the backlog routines running the route skill, by name; the dispatch select's options
	RouteSkill     string
	RouteSkills    []string             // the skills a backlog routine runs, the route select's options
	Runs           []routines.RunResult // run rows carrying the item id, newest first
	Sessions       []peek.Session       // item page only: run sessions merged with titles carrying the id
	SkillHref      string               // skill page URL, empty when the skill is not installed
	Statuses       []string
}

type backlogPageData struct {
	Active         int
	Archive        *backlog.Archive
	DataJson       template.JS
	Epics          []string
	Error          string
	Live           int
	Mined          int
	NextId         string
	Page           string
	PriCounts      []backlogFilterCount
	RoutineMissing int
	Sections       []backlogSectionView
	StatusCounts   []backlogFilterCount
	StoreRel       string
	Tab            string
	Title          string
	Updated        string
}

type backlogPanelBlob struct {
	Epics      []string                    `json:"epics"`
	Items      map[string]backlogPanelItem `json:"items"`
	Priorities []string                    `json:"priorities"`
	Statuses   []string                    `json:"statuses"`
}

// backlogPanelItem is what the board's panel card needs; the item page keeps the full view.
type backlogPanelItem struct {
	Id string `json:"id"`

	Deps        []depView             `json:"deps"`
	Description string                `json:"description"`
	DoneWhen    string                `json:"doneWhen"`
	Epic        string                `json:"epic"`
	Evidence    []evidenceLink        `json:"evidence"`
	Mined       *backlog.Mined        `json:"mined"`
	MinedByURL  string                `json:"minedByUrl"`
	Pri         string                `json:"pri"`
	PriClass    string                `json:"priClass"`
	Route       string                `json:"route"`
	RouteSkill  string                `json:"routeSkill"`
	Routine     string                `json:"routine"` // the dispatch select's default
	Routines    []backlogPanelRoutine `json:"routines"`
	Size        string                `json:"size"`
	Stage       string                `json:"stage"`
	Status      string                `json:"status"`
	Theme       string                `json:"theme"`
	Title       string                `json:"title"`
}

type backlogPanelRoutine struct {
	Live   bool   `json:"live"`
	Loaded bool   `json:"loaded"`
	Name   string `json:"name"`
}

type backlogSectionView struct {
	Days     []backlogDayView // Icebox only: the items grouped per day, newest first
	IsIcebox bool
	Items    []backlogItemView
	Name     string
}

type depView struct {
	Id string `json:"id"`

	IsActive bool `json:"active"` // the dep is on the board; an archived dep renders as text
}

type dispatchForm struct {
	IsBoard bool // posted from the board's card: the answer swaps the item's Activity cell
	Prompt  string
	Routine string // the chosen routine's name; the handler admits only a backlog routine running the item's route skill
}

func (f *dispatchForm) Validate(r *http.Request) error {
	// board
	f.IsBoard = r.FormValue("board") == "1"

	// prompt
	f.Prompt = strings.TrimSpace(r.FormValue("prompt"))

	// routine
	f.Routine = strings.TrimSpace(r.FormValue("routine"))
	if f.Routine == "" {
		return errors.New("dispatchForm.Validate: Missing field routine")
	}
	return nil
}

type evidenceLink struct {
	Text    string `json:"text"`
	WeekURL string `json:"weekUrl"` // the session's card on its week page, opened and flashed on arrival
}

func (s *Server) handleBacklogPage(w http.ResponseWriter, r *http.Request) {
	s.reloadSessions()
	data := backlogPageData{Page: pageBacklog, StoreRel: s.backlogStorePath(), Tab: "active", Title: "Backlog"}
	if r.URL.Query().Get("tab") == backlogTabArchive {
		data.Tab = backlogTabArchive
	}
	store, err := s.loadBacklog()
	if err != nil {
		data.Error = err.Error()
		s.renderFragment(w, tmplBacklog, data)
		return
	}
	data.Active, data.NextId, data.Updated = store.Active(), store.NextId(), store.Updated

	if data.Tab == backlogTabArchive {
		data.Archive = &store.Archive
	}

	scanned := s.scanRoutines()
	list := loadableRoutines(scanned)
	routeSkills := backlogRouteSkills(scanned)
	installed := s.installedSkills()
	histories := routineHistories(list)
	loaded := loadedRoutines(r.Context(), list)
	active := activeIds(store)
	statusCounts := map[string]int{}
	priCounts := map[string]int{}
	refs := s.sessions.SessionRefs()
	resolve := func(entry string) evidenceLink { return resolveSessionPointer(entry, refs) }
	data.Epics = store.SectionNames()
	blob := backlogPanelBlob{Epics: data.Epics, Items: map[string]backlogPanelItem{}, Priorities: backlog.Priorities, Statuses: backlog.Statuses}
	for _, section := range store.Sections {
		view := backlogSectionView{IsIcebox: section.Name == backlog.IceboxSection, Name: section.Name}
		for _, item := range section.Items {
			itemView := backlogItemViewFor(active, installed, item, list, resolve, routeSkills)
			itemView.MinedByURL = minedByURL(s.peekDashboardURL, item.Mined)
			itemView = withRuns(itemView, histories, maxBoardRuns)
			statusCounts[item.Status]++
			priCounts[item.Pri]++
			if itemView.RoutineMissing {
				data.RoutineMissing++
			}
			if itemView.IsLive {
				data.Live++
			}
			if item.Mined != nil {
				data.Mined++
			}
			blob.Items[item.Id] = panelItem(loaded, &itemView)
			view.Items = append(view.Items, itemView)
		}
		if view.IsIcebox {
			view.Days = iceboxDays(view.Items)
		}
		data.Sections = append(data.Sections, view)
	}
	data.StatusCounts = filterCounts(backlog.Statuses, statusCounts)
	data.PriCounts = filterCounts(backlog.Priorities, priCounts)
	encoded, err := json.Marshal(blob)
	if err != nil {
		data.Error = err.Error()
	}

	data.DataJson = template.JS(encoded)
	s.renderFragment(w, tmplBacklog, data)
}

func (s *Server) handleBacklogItem(w http.ResponseWriter, r *http.Request) {
	item, store, ok := s.findBacklogItem(w, r)
	if !ok {
		return
	}
	s.reloadSessions()
	sessions, _ := s.peekRoutineSessions(r.Context())
	view := withSessions(s.itemView(item, historyLimitItem, store), historyLimitItem, sessions)
	data := backlogItemPage{
		Epics:    store.SectionNames(),
		Item:     view,
		Page:     pageBacklog,
		Stage:    backlog.StageForSkill(view.RouteSkill),
		StoreRel: s.backlogStorePath(),
		Title:    item.Id + " " + item.Title,
	}
	if view.Routine != nil {
		row := s.routineView(r.Context(), view.Routine)
		data.Row = &row
	}
	s.renderFragment(w, tmplBacklogItem, data)
}

// handleBacklogActivity re-renders one row's Activity cell, polled while a dispatch is pending or its run is live; the selects outside it are never swapped.
func (s *Server) handleBacklogActivity(w http.ResponseWriter, r *http.Request) {
	item, store, ok := s.findBacklogItem(w, r)
	if !ok {
		return
	}
	since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
	view := s.itemView(item, maxBoardRuns, store)
	isDispatchPending := since > 0 && !hasRunSince(view.LastRun, since)
	// The cell swaps itself: a cell rendered without hx-get, neither pending nor live, ends the polling.
	if isDispatchPending {
		view.PollSince = since
	}

	s.renderFragment(w, tmplBacklogActivity, view)
}

// handleBacklogRuns is polled while the newest run is live.
func (s *Server) handleBacklogRuns(w http.ResponseWriter, r *http.Request) {
	item, store, ok := s.findBacklogItem(w, r)
	if !ok {
		return
	}
	sessions, _ := s.peekRoutineSessions(r.Context())
	view := withSessions(s.itemView(item, historyLimitItem, store), historyLimitItem, sessions)
	s.renderFragment(w, tmplBacklogRuns, view)
}

func (s *Server) handleBacklogEpicAdd(w http.ResponseWriter, r *http.Request) {
	answerBacklogWrite(backlog.AddSection(r.FormValue("name"), s.backlogStorePath()), w)
}

func (s *Server) handleBacklogEpicMove(w http.ResponseWriter, r *http.Request) {
	answerBacklogWrite(backlog.MoveItem(r.PathValue("id"), s.backlogStorePath(), r.FormValue("epic")), w)
}

func (s *Server) handleBacklogEpicRemove(w http.ResponseWriter, r *http.Request) {
	answerBacklogWrite(backlog.RemoveSection(r.PathValue("name"), s.backlogStorePath()), w)
}

func (s *Server) handleBacklogPri(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	markTitle := func(view *backlogItemView) { view.IsTitleOOB = true }
	s.answerItemWrite(r.Context(), backlog.SetPri(id, s.backlogStorePath(), r.FormValue("pri")), id, markTitle, w)
}

// handleBacklogRoute admits only a skill a backlog routine runs, the choice the select offers.
func (s *Server) handleBacklogRoute(w http.ResponseWriter, r *http.Request) {
	skill := r.FormValue("skill")
	if !slices.Contains(backlogRouteSkills(s.scanRoutines()), skill) {
		respond.WithBadRequest("no backlog routine runs /"+skill, w)
		return
	}

	id := r.PathValue("id")
	// The Activity cell follows the route: its routine, missing state and last run change with it.
	markRoute := func(view *backlogItemView) { view.IsActivityOOB, view.IsRouteOOB = true, true }
	s.answerItemWrite(r.Context(), backlog.SetRouteSkill(id, s.backlogStorePath(), skill), id, markRoute, w)
}

// handleBacklogStatus reloads only when the item leaves the board; any other status is answered in place.
func (s *Server) handleBacklogStatus(w http.ResponseWriter, r *http.Request) {
	id, status := r.PathValue("id"), r.FormValue("status")
	if slices.Contains(backlog.ArchiveStatuses, status) {
		answerBacklogWrite(backlog.MoveToArchive(id, s.backlogStorePath(), status+" (board)", status), w)
		return
	}

	markStatus := func(view *backlogItemView) { view.IsStatusOOB = true }
	s.answerItemWrite(r.Context(), backlog.SetStatus(id, s.backlogStorePath(), status), id, markStatus, w)
}

// handleBacklogDispatch returns an OOB cell or row that polls until the run ends: the item's Activity cell from the card, the routine row from the item page.
func (s *Server) handleBacklogDispatch(w http.ResponseWriter, r *http.Request) {
	item, store, ok := s.findBacklogItem(w, r)
	if !ok {
		return
	}
	form := &dispatchForm{}
	if err := form.Validate(r); err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}
	candidates := backlogRoutinesFor(loadableRoutines(s.scanRoutines()), item.RouteSkill())
	if len(candidates) == 0 {
		respond.WithConflict("routine missing: no backlog routine runs "+item.Route, w)
		return
	}
	position := slices.IndexFunc(candidates, func(candidate routines.Routine) bool { return candidate.Name == form.Routine })
	if position < 0 {
		respond.WithBadRequest(form.Routine+" is no backlog routine running /"+item.RouteSkill(), w)
		return
	}
	routine := &candidates[position]
	if routine.IsLive {
		respond.WithConflict(routine.Name+" has a run active", w)
		return
	}
	loaded, err := routines.IsLoaded(r.Context(), routine.Label)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	if !loaded {
		respond.WithConflict(routine.Name+" is stopped — Start it on the Routines page", w)
		return
	}

	since := time.Now().Unix()
	output, err := s.runNow(r.Context(), dispatchPrompt(&item, form.Prompt), routine)
	if !form.IsBoard {
		s.renderRoutineResult(r.Context(), output, since, err, routine, w)
		return
	}

	// The board swaps the item's Activity cell out of band; it polls from "starting" through "running" until the run ends.
	result := opResult{Output: output, Page: pageBacklog, Subject: "dispatch " + item.Id + " to " + routine.Name}
	if err != nil {
		result.Error = err.Error()
	}
	s.renderFragment(w, tmplOpResult, result)

	view := s.itemView(item, maxBoardRuns, store)
	view.IsActivityOOB = true
	if err == nil {
		view.PollSince = since
	}
	s.renderFragment(w, tmplBacklogCellsOOB, view)
}

// answerItemWrite answers an in-place write with the cells mark flags out of band and the card data as the backlog:item event; nothing reloads.
func (s *Server) answerItemWrite(ctx context.Context, err error, id string, mark func(view *backlogItemView), w http.ResponseWriter) {
	if answerWriteError(err, w) {
		return
	}

	store, err := s.loadBacklog()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	item, found := store.Find(id)
	if !found {
		respond.WithConflict("file changed underneath, reload", w)
		return
	}

	view := s.itemView(item, maxBoardRuns, store)
	mark(&view)
	events := map[string]backlogPanelItem{"backlog:item": panelItem(loadedRoutines(ctx, view.Routines), &view)}
	encoded, err := json.Marshal(events)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	w.Header().Set("HX-Trigger-After-Swap", asciiJson(encoded))
	s.renderFragment(w, tmplBacklogCellsOOB, view)
}

func (s *Server) backlogStorePath() string {
	return filepath.Join(s.backlogDir, backlogStoreFile)
}

// findBacklogItem resolves {id} to an item and its store or writes the 404/500 itself.
func (s *Server) findBacklogItem(w http.ResponseWriter, r *http.Request) (backlog.Item, *backlog.Store, bool) {
	store, err := s.loadBacklog()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return backlog.Item{}, nil, false
	}
	item, found := store.Find(r.PathValue("id"))
	if !found {
		respond.WithNotFound("unknown backlog item", w)
		return backlog.Item{}, nil, false
	}
	return item, store, true
}

// installedSkills yields none on a scan failure; route skills then render as plain text.
func (s *Server) installedSkills() []skills.Skill {
	list, err := skills.Scan(s.skillsRepo, s.skillsHome)
	if err != nil {
		return nil
	}
	return list
}

// itemView builds one item's view from the store, the routines and their run rows; peek sessions are the item page's addition.
func (s *Server) itemView(item backlog.Item, max int, store *backlog.Store) backlogItemView {
	scanned := s.scanRoutines()
	list := loadableRoutines(scanned)
	refs := s.sessions.SessionRefs()
	resolve := func(entry string) evidenceLink { return resolveSessionPointer(entry, refs) }
	view := backlogItemViewFor(activeIds(store), s.installedSkills(), item, list, resolve, backlogRouteSkills(scanned))
	view.MinedByURL = minedByURL(s.peekDashboardURL, item.Mined)
	return withRuns(view, routineHistories(list), max)
}

// loadBacklog parses the store; a missing file is an empty store (D19).
func (s *Server) loadBacklog() (*backlog.Store, error) {
	store, err := backlog.Load(s.backlogStorePath())
	if errors.Is(err, os.ErrNotExist) {
		return &backlog.Store{}, nil
	}
	if err != nil {
		return nil, err
	}
	return store, nil
}

// scanRoutines yields none on a scan failure; the board degrades to store-only rows.
func (s *Server) scanRoutines() []routines.Routine {
	list, err := routines.Scan(s.routinesDir)
	if err != nil {
		return nil
	}
	return list
}

// activeIds is the set of ids on the board; a dep outside it is archived.
func activeIds(store *backlog.Store) map[string]bool {
	active := map[string]bool{}
	for _, item := range store.Items() {
		active[item.Id] = true
	}
	return active
}

// answerBacklogWrite answers a structural write — an archive move, an epic change — with a full reload: headings, tables and the archive tab change.
func answerBacklogWrite(err error, w http.ResponseWriter) {
	if answerWriteError(err, w) {
		return
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// answerWriteError answers a failed board write — 400 on a refused value, 409 on a vanished item, else 500 — and reports whether it answered.
func answerWriteError(err error, w http.ResponseWriter) bool {
	refused := []error{backlog.ErrInvalidPri, backlog.ErrInvalidRoute, backlog.ErrInvalidSection, backlog.ErrInvalidStatus}
	isRefused := slices.ContainsFunc(refused, func(target error) bool { return errors.Is(err, target) })
	switch {
	case isRefused:
		respond.WithBadRequest(err.Error(), w)
		return true
	case errors.Is(err, backlog.ErrItemNotFound):
		respond.WithConflict("file changed underneath, reload", w)
		return true
	case err != nil:
		respond.WithInternalServerError(err, w)
		return true
	}
	return false
}

// asciiJson escapes every non-ASCII rune as \uXXXX: a response header reaches the browser as Latin-1, so raw UTF-8 would garble titles.
func asciiJson(data []byte) string {
	var escaped strings.Builder
	for _, char := range string(data) {
		if char < utf8.RuneSelf {
			escaped.WriteRune(char)
			continue
		}
		for _, unit := range utf16.Encode([]rune{char}) {
			fmt.Fprintf(&escaped, `\u%04x`, unit)
		}
	}
	return escaped.String()
}

// backlogRouteSkills lists, sorted, the skills the backlog family's routines run — a routine without a plist still declares its skill.
func backlogRouteSkills(list []routines.Routine) []string {
	var skillNames []string
	for index := range list {
		routine := &list[index]
		isBacklogRoutine := routineFamily(routine) == backlogFamily && routine.Skill != ""
		if isBacklogRoutine && !slices.Contains(skillNames, routine.Skill) {
			skillNames = append(skillNames, routine.Skill)
		}
	}
	slices.Sort(skillNames)
	return skillNames
}

// backlogRoutinesFor lists, by name, the backlog family's routines that run the skill — the routines an item on that route can dispatch to.
func backlogRoutinesFor(list []routines.Routine, skill string) []routines.Routine {
	var matched []routines.Routine
	if skill == "" {
		return matched
	}

	for index := range list {
		isCandidate := routineFamily(&list[index]) == backlogFamily && list[index].Skill == skill
		if isCandidate {
			matched = append(matched, list[index])
		}
	}
	slices.SortFunc(matched, func(left, right routines.Routine) int { return strings.Compare(left.Name, right.Name) })
	return matched
}

func backlogItemViewFor(active map[string]bool, installed []skills.Skill, item backlog.Item, list []routines.Routine, resolve func(string) evidenceLink, routeSkills []string) backlogItemView {
	description, doneWhen := splitDoneWhen(item.Description)
	view := backlogItemView{
		Description: description,
		DoneWhen:    doneWhen,
		Evidence:    evidenceLinks(item.Evidence, resolve),
		Item:        item,
		Priorities:  backlog.Priorities,
		RouteSkill:  item.RouteSkill(),
		RouteSkills: routeSkills,
		Statuses:    backlog.Statuses,
	}
	view.IsRouteListed = slices.Contains(routeSkills, view.RouteSkill)
	for _, dep := range item.Deps {
		view.Deps = append(view.Deps, depView{Id: dep, IsActive: active[dep]})
	}
	if view.RouteSkill != "" {
		view.Routines = backlogRoutinesFor(list, view.RouteSkill)
		view.RoutineMissing = len(view.Routines) == 0
		if !view.RoutineMissing {
			view.Routine = &view.Routines[0]
		}
		view.RouteRest = strings.TrimPrefix(item.Route, "/"+view.RouteSkill)
		view.SkillHref = skillHref(installed, view.RouteSkill)
	}
	return view
}

// dayLabel renders a store date the way the sessions page labels a day; an unparseable date stays as written.
func dayLabel(date string) string {
	if stamp, err := time.Parse(time.DateOnly, date); err == nil {
		return stamp.Format("Mon 2006-01-02")
	}
	return date
}

// dispatchPrompt puts id and title on the first line; the runner reads the id from it (ItemIdOf).
func dispatchPrompt(item *backlog.Item, extra string) string {
	prompt := item.Id + ": " + item.Title
	if extra != "" {
		prompt += " " + extra
	}
	if args := item.RouteArgs(); args != "" {
		prompt = args + " " + prompt
	}
	return prompt + "\n\n" + item.Description
}

// evidenceLinks resolves every entry through the session resolver; a non-session entry comes back as text.
func evidenceLinks(entries []string, resolve func(string) evidenceLink) []evidenceLink {
	links := make([]evidenceLink, 0, len(entries))
	for _, entry := range entries {
		links = append(links, resolve(entry))
	}
	return links
}

func filterCounts(order []string, counts map[string]int) []backlogFilterCount {
	result := make([]backlogFilterCount, 0, len(order))
	for _, value := range order {
		result = append(result, backlogFilterCount{Count: counts[value], Value: value})
	}
	return result
}

// hasRunSince reports whether the newest run row — started or finished — was written at or after the dispatch time.
func hasRunSince(last *routines.RunResult, sinceUnix int64) bool {
	if last == nil {
		return false
	}
	stamp := runTime(last)
	return !stamp.IsZero() && stamp.Unix() >= sinceUnix
}

// iceboxDays groups Icebox rows by the day they were mined, else added, newest first; the newest day starts open.
func iceboxDays(items []backlogItemView) []backlogDayView {
	byKey := map[string]*backlogDayView{}
	for _, item := range items {
		key := item.Item.Added
		if item.Item.Mined != nil {
			key = item.Item.Mined.Date
		}
		day, ok := byKey[key]
		if !ok {
			day = &backlogDayView{Key: key, Label: dayLabel(key)}
			byKey[key] = day
		}
		day.Items = append(day.Items, item)
	}

	days := make([]backlogDayView, 0, len(byKey))
	for _, key := range slices.Sorted(maps.Keys(byKey)) {
		days = append(days, *byKey[key])
	}
	slices.Reverse(days)
	if len(days) > 0 {
		days[0].IsOpen = true
	}
	return days
}

func itemRuns(history []routines.RunResult, id string, max int) []routines.RunResult {
	var kept []routines.RunResult
	for _, result := range history {
		if result.Item == id {
			kept = append(kept, result)
		}
	}
	if len(kept) > max {
		kept = kept[:max]
	}
	return kept
}

// loadableRoutines keeps the routines that loaded; dispatch and run rows need their plist.
func loadableRoutines(list []routines.Routine) []routines.Routine {
	var kept []routines.Routine
	for _, routine := range list {
		if routine.LoadError == "" {
			kept = append(kept, routine)
		}
	}
	return kept
}

// loadedRoutines probes launchd once per loadable routine; a probe error counts as not loaded, as the panel did per item.
func loadedRoutines(ctx context.Context, list []routines.Routine) map[string]bool {
	loaded := map[string]bool{}
	for _, routine := range list {
		isLoaded, err := routines.IsLoaded(ctx, routine.Label)
		loaded[routine.Label] = err == nil && isLoaded
	}
	return loaded
}

func mergeSessions(first, second []peek.Session, max int) []peek.Session {
	seen := map[string]bool{}
	var merged []peek.Session
	for _, session := range append(slices.Clone(first), second...) {
		if seen[session.Id] {
			continue
		}
		seen[session.Id] = true
		merged = append(merged, session)
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].LastActive.After(merged[j].LastActive)
	})
	if len(merged) > max {
		merged = merged[:max]
	}
	return merged
}

// minedByURL is the peek page of the run session that mined the row; empty when the row names none or no dashboard is configured.
func minedByURL(base string, mined *backlog.Mined) string {
	if mined == nil {
		return ""
	}
	return peekSessionURL(base, mined.By)
}

// panelItem flattens one board row into the panel card's data; loaded is the per-routine launchd probe.
func panelItem(loaded map[string]bool, view *backlogItemView) backlogPanelItem {
	item := backlogPanelItem{
		Id:          view.Item.Id,
		Deps:        view.Deps,
		Description: view.Description,
		DoneWhen:    view.DoneWhen,
		Epic:        view.Item.Section,
		Evidence:    view.Evidence,
		Mined:       view.Item.Mined,
		MinedByURL:  view.MinedByURL,
		Pri:         view.Item.Pri,
		PriClass:    priBadgeClass(view.Item.Pri),
		Route:       view.Item.Route,
		RouteSkill:  view.RouteSkill,
		Size:        view.Item.Size,
		Stage:       backlog.StageForSkill(view.RouteSkill),
		Status:      view.Item.Status,
		Theme:       view.Item.Theme,
		Title:       view.Item.Title,
	}
	if view.Routine != nil {
		item.Routine = view.Routine.Name
	}
	for _, routine := range view.Routines {
		panelRoutine := backlogPanelRoutine{Live: routine.IsLive, Loaded: loaded[routine.Label], Name: routine.Name}
		item.Routines = append(item.Routines, panelRoutine)
	}
	return item
}

// priBadgeClass is the Go twin of the priBadge template func; the panel JSON carries the class.
func priBadgeClass(pri string) string {
	switch pri {
	case priCritical:
		return badgeError
	case priHigh:
		return badgeAction
	}
	return badgeDim
}

// priLabel is the short label a priority select shows so the long words never run under its arrow; the stored value stays.
func priLabel(pri string) string {
	switch pri {
	case priCritical:
		return "crit"
	case priMedium:
		return "med"
	case priHigh, priLow:
		return pri
	}
	return pri
}

// resolveSessionPointer links a session pointer to its card on the week page, or with "#<n>" to that finding, as the Proposals evidence does; a session no mined record holds stays text.
func resolveSessionPointer(entry string, refs map[string]sessions.SessionRef) evidenceLink {
	link := evidenceLink{Text: entry}
	match := sessionPointerRe.FindStringSubmatch(entry)
	if match == nil {
		return link
	}

	id, ref, found := sessions.ResolveRef(match[1], refs)
	if !found {
		return link
	}

	query := url.Values{"session": {id}}
	link.WeekURL = fmt.Sprintf("/sessions/%s/%s?%s", url.PathEscape(ref.Scope), url.PathEscape(ref.Week), query.Encode())
	if finding := match[2]; finding != "" {
		link.WeekURL += fmt.Sprintf("#f-%s-%s", id, finding)
	}
	return link
}

// routineFamily is the routine's manifest group, else its name up to the first '-', else its whole name.
func routineFamily(routine *routines.Routine) string {
	if routine.Group != "" {
		return routine.Group
	}
	prefix, _, _ := strings.Cut(routine.Name, "-")
	return prefix
}

func routineHistories(list []routines.Routine) map[string][]routines.RunResult {
	histories := map[string][]routines.RunResult{}
	for _, routine := range list {
		history, err := routines.History(routine.ResultsPath, routines.HistoryLimit())
		if err != nil {
			continue
		}
		histories[routine.Name] = history
	}
	return histories
}

// runSessions resolves the sessions the run rows name, in row order.
func runSessions(runs []routines.RunResult, sessions []peek.Session) []peek.Session {
	byId := map[string]peek.Session{}
	for _, session := range sessions {
		byId[session.Id] = session
	}
	var matched []peek.Session
	for _, run := range runs {
		if session, found := byId[run.SessionId]; found {
			matched = append(matched, session)
		}
	}
	return matched
}

func skillHref(installed []skills.Skill, name string) string {
	for _, origin := range []string{skills.OriginRepo, skills.OriginHome} {
		if skill, found := skills.Find(installed, origin, name); found {
			return "/scripts/skills/" + skill.Origin + "/" + skill.Name
		}
	}
	return ""
}

// splitDoneWhen cuts the description at its "Done when:" line; a description without one keeps everything.
func splitDoneWhen(description string) (string, string) {
	index := strings.Index(description, "\nDone when:")
	if index < 0 {
		if doneWhen, isDoneWhenOnly := strings.CutPrefix(description, "Done when:"); isDoneWhenOnly {
			return "", strings.TrimSpace(doneWhen)
		}
		return description, ""
	}
	rest := strings.TrimSpace(description[index+1:])
	return strings.TrimSpace(description[:index]), strings.TrimSpace(strings.TrimPrefix(rest, "Done when:"))
}

// titleSessions matches exact tokens: `B-005` never matches `B-0051` or `SUB-005`.
func titleSessions(id string, max int, sessions []peek.Session) []peek.Session {
	token := regexp.MustCompile(`(^|[^A-Za-z0-9])` + regexp.QuoteMeta(id) + `($|[^0-9])`)
	var matched []peek.Session
	for _, session := range sessions {
		if token.MatchString(session.Title) {
			matched = append(matched, session)
		}
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].LastActive.After(matched[j].LastActive)
	})
	if len(matched) > max {
		matched = matched[:max]
	}
	return matched
}

// runTime is a run row's start or end time; an unparseable stamp is the zero time and sorts last.
func runTime(run *routines.RunResult) time.Time {
	stamp, err := time.Parse(time.RFC3339, run.Timestamp)
	if err != nil {
		return time.Time{}
	}
	return stamp
}

// withRuns merges the item's run rows of every candidate routine, newest first; the routine of the newest row becomes the item's routine.
func withRuns(view backlogItemView, histories map[string][]routines.RunResult, max int) backlogItemView {
	for index := range view.Routines {
		routine := &view.Routines[index]
		runs := itemRuns(histories[routine.Name], view.Item.Id, max)
		isNewer := len(runs) > 0 && (view.LastRun == nil || runTime(&runs[0]).After(runTime(view.LastRun)))
		if isNewer {
			view.LastRun = &runs[0]
			view.Routine = routine
		}
		view.Runs = append(view.Runs, runs...)
	}
	if view.LastRun == nil {
		return view
	}

	slices.SortStableFunc(view.Runs, func(left, right routines.RunResult) int { return runTime(&right).Compare(runTime(&left)) })
	view.Runs = view.Runs[:min(len(view.Runs), max)]
	view.LastRun = &view.Runs[0]
	view.IsLive = view.LastRun.IsRunning() && view.Routine.IsLive
	return view
}

// withSessions adds the peek sessions the item page shows: the run rows' sessions merged with titles carrying the id.
func withSessions(view backlogItemView, max int, peekSessions []peek.Session) backlogItemView {
	view.Sessions = mergeSessions(titleSessions(view.Item.Id, max, peekSessions), runSessions(view.Runs, peekSessions), max)
	return view
}

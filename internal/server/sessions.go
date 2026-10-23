package server

import (
	"context"
	"fmt"
	"log"
	"maps"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kevinhorst/smine/internal/peek"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/server/respond"
	"github.com/kevinhorst/smine/internal/sessions"
)

const (
	pageSessions        = "sessions"
	tmplSessionsArchive = "sessions_archive.html"
	tmplSessionsContext = "_sessions_context.html"
	tmplSessionsIndex   = "sessions_index.html"
	tmplSessionsWeek    = "sessions_week.html"
	tmplWeekBody        = "_week_body.html"

	// dimFrustration/dimPositive are backed by the per-session quote arrays, not by findings.
	dimFrustration = "frustration"
	dimPositive    = "positive"

	headlineFallbackRunes = 120

	// unknownDayKey sorts below every date, so sessions without a parseable lastActive close the page.
	unknownDayKey   = "0000-00-00"
	unknownDayLabel = "unknown day"
)

type weekPage struct {
	Days          []sessionDayView
	Dimension     string
	Dimensions    []string
	Open          string
	Page          string
	SessionIds    []string
	SessionsParam string
	Sessions      []sessionView
	Title         string
	Week          *sessions.Week
}

type sessionsArchivePage struct {
	ActiveScopes []string
	Archived     []sessions.ArchivedFolder
	Page         string
	Title        string
}

// sessionsContextFragment's RetryUrl reloads the fragment while peek does not answer.
type sessionsContextFragment struct {
	ContextWeeks []contextWeekGroup
	PeekErr      string
	RetryUrl     string
}

type sessionsIndexPage struct {
	IsAll      bool
	Page       string
	Scope      *sessions.ScopeInfo
	ScopeNames []string
	Title      string
}

type contextSessionView struct {
	RecordUrl string
	Repo      string
	Session   peek.Session
	ShortId   string
}

type contextDayGroup struct {
	Label    string
	Sessions []contextSessionView
}

type contextWeekGroup struct {
	Count int
	Days  []contextDayGroup
	Key   string
	Label string
}

// add expects newest-first input, so a new day always follows the previous one.
func (g *contextWeekGroup) add(day time.Time, view contextSessionView) {
	g.Count++
	label := day.Format("Mon 2006-01-02")
	if len(g.Days) == 0 || g.Days[len(g.Days)-1].Label != label {
		g.Days = append(g.Days, contextDayGroup{Label: label})
	}
	last := &g.Days[len(g.Days)-1]
	last.Sessions = append(last.Sessions, view)
}

type findingRowView struct {
	Anchor     string
	Dimension  string
	Headline   string
	QuoteCount int
	Quotes     []sessions.Quote
	Snippets   []sessions.Snippet
	Summary    string
}

// sessionDayView is one local calendar day of a week page; Key sorts, Label shows.
type sessionDayView struct {
	IsOpen   bool
	Key      string
	Label    string
	Sessions []sessionView
}

type sessionView struct {
	HasReport   bool
	Live        bool
	Rows        []findingRowView
	Selected    bool
	Session     *sessions.Session
	ShortId     string
	SignalLabel string
}

func (s *Server) handleSessionsWeek(w http.ResponseWriter, r *http.Request) {
	s.reloadSessions()
	scope, key := r.PathValue("scope"), r.PathValue("week")
	if !sessions.IsWeekKey(key) {
		respond.WithBadRequest("invalid week", w)
		return
	}

	week, ok := s.sessions.Week(scope, key)
	if !ok {
		http.NotFound(w, r)
		return
	}

	sessionIds := parseSessionIds(r.URL.Query())
	language := s.presentation.language()
	data := weekPage{
		Dimension:     r.URL.Query().Get("dimension"),
		Dimensions:    weekDimensions(week),
		Open:          r.URL.Query().Get("open"),
		Page:          pageSessions,
		SessionIds:    sessionIds,
		SessionsParam: strings.Join(sessionIds, ","),
		Title:         fmt.Sprintf("%s · %s %s", scope, translate(language, "Week"), week.Label()),
		Week:          week,
	}
	peekSessions, err := s.peekSessions(r.Context())
	if err != nil {
		log.Printf("Server.handleSessionsWeek: Week %s: Session badges off: %v", key, err)
	}

	data.Sessions = visibleSessions(data.Dimension, peekSessions, sessionIds, week)
	data.Days = sessionDays(time.Local, data.Open, data.Sessions)
	if r.Header.Get("HX-Request") != "" {
		s.renderFragment(w, tmplWeekBody, data)
		return
	}

	s.renderFragment(w, tmplSessionsWeek, data)
}

func (s *Server) handleSessionsArchiveTab(w http.ResponseWriter, r *http.Request) {
	s.reloadSessions()
	data := sessionsArchivePage{
		ActiveScopes: s.activeScopeNames(),
		Archived:     s.sessions.ArchivedFolders(),
		Page:         pageSessions,
		Title:        "Sessions — archived records",
	}
	s.renderFragment(w, tmplSessionsArchive, data)
}

func (s *Server) handleSessionsIndex(w http.ResponseWriter, r *http.Request) {
	s.reloadSessions()
	names := s.sessionScopeNames()
	if len(names) == 0 {
		data := sessionsIndexPage{Page: pageSessions, Title: "Sessions"}
		s.renderFragment(w, tmplSessionsIndex, data)
		return
	}

	data := sessionsIndexPage{
		IsAll:      true,
		Page:       pageSessions,
		Scope:      s.mergedScope(),
		ScopeNames: names,
		Title:      translate(s.presentation.language(), "Sessions"),
	}
	s.renderFragment(w, tmplSessionsIndex, data)
}

// handleSessionsContext loads after the page has rendered, so a cold or slow peek only delays this list.
func (s *Server) handleSessionsContext(w http.ResponseWriter, r *http.Request) {
	data := sessionsContextFragment{RetryUrl: r.URL.RequestURI()}
	peekSessions, err := s.peekSessions(r.Context())
	if err != nil {
		data.PeekErr = err.Error()
		s.renderFragment(w, tmplSessionsContext, data)
		return
	}

	views := s.contextSessions(peekSessions, r.URL.Query().Get("scope"))
	s.linkContextSessions(views)
	data.ContextWeeks = groupContextSessionsByWeek(time.Local, views)
	s.renderFragment(w, tmplSessionsContext, data)
}

func (s *Server) handleSessionsScope(w http.ResponseWriter, r *http.Request) {
	s.reloadSessions()
	scope := r.PathValue("scope")
	info, ok := s.sessions.Scope(scope)
	if !ok {
		if _, isRegistered := s.repoRegistry.Find(scope); !isRegistered {
			http.NotFound(w, r)
			return
		}
		info = &sessions.ScopeInfo{Name: scope}
	}

	data := sessionsIndexPage{
		Page:       pageSessions,
		Scope:      info,
		ScopeNames: s.sessionScopeNames(),
		Title:      translate(s.presentation.language(), "Sessions") + " — " + scope,
	}
	s.renderFragment(w, tmplSessionsIndex, data)
}

// activeScopeNames are the mined scope folders, archived ones excluded.
func (s *Server) activeScopeNames() []string {
	names := make([]string, 0)
	for _, scopeInfo := range s.sessions.Scopes() {
		names = append(names, scopeInfo.Name)
	}
	return names
}

// contextSessions spans every registered repo for an empty scope, else the scope's repo; an unregistered scope has none.
func (s *Server) contextSessions(peekSessions map[string]peek.Session, scope string) []contextSessionView {
	if scope == "" {
		return contextSessionsAll(peekSessions, s.repoRegistry.Repos())
	}

	repo, ok := s.repoRegistry.Find(scope)
	if !ok {
		return nil
	}
	return contextSessionsUnder(peekSessions, repo.Path)
}

// linkContextSessions points each mined session at its record on the week page.
func (s *Server) linkContextSessions(views []contextSessionView) {
	refs := s.sessions.SessionRefs()
	for index := range views {
		ref, isMined := refs[views[index].Session.Id]
		if isMined {
			views[index].RecordUrl = sessionRecordURL(views[index].Session.Id, ref)
		}
	}
}

// mergedScope holds every scope's weeks, newest week first, scope name breaking ties.
func (s *Server) mergedScope() *sessions.ScopeInfo {
	merged := &sessions.ScopeInfo{}
	for _, scopeInfo := range s.sessions.Scopes() {
		merged.LoadErrors = append(merged.LoadErrors, scopeInfo.LoadErrors...)
		merged.Weeks = append(merged.Weeks, scopeInfo.Weeks...)
	}
	slices.SortStableFunc(merged.Weeks, func(a, b *sessions.Week) int {
		return strings.Compare(b.Key, a.Key)
	})
	return merged
}

// sessionScopeNames includes repos without mined batches: a live session may exist for them.
func (s *Server) sessionScopeNames() []string {
	names := s.activeScopeNames()
	for _, repo := range s.repoRegistry.Repos() {
		if !slices.Contains(names, repo.Name) {
			names = append(names, repo.Name)
		}
	}
	slices.Sort(names)
	return names
}

// peekSessions waits at most peekSessionsTimeout, so a cold peek never holds a page; a disabled peek yields none and no error.
func (s *Server) peekSessions(ctx context.Context) (map[string]peek.Session, error) {
	if s.peekClient.Endpoint() == "" {
		return nil, nil
	}

	peekCtx, cancel := context.WithTimeout(ctx, peekSessionsTimeout)
	defer cancel()
	byId, err := s.peekClient.SessionsById(peekCtx)
	if err != nil {
		return nil, fmt.Errorf("Server.peekSessions: peek-mcp at %s did not answer: %w", s.peekClient.Endpoint(), err)
	}
	return byId, nil
}

func (s *Server) handleSessionsScopeArchive(w http.ResponseWriter, r *http.Request) {
	scope := r.FormValue("scope")
	result := opResult{Page: pageSessions, Subject: "archive " + scope}
	if err := s.sessions.ArchiveScope(r.Context(), scope); err != nil {
		result.Error = err.Error()
		s.renderFragment(w, tmplOpResult, result)
		return
	}

	w.Header().Set("HX-Redirect", "/sessions-archive")
	s.renderFragment(w, tmplOpResult, result)
}

func (s *Server) handleSessionsScopeDelete(w http.ResponseWriter, r *http.Request) {
	result := opResult{Page: pageSessions, Subject: "delete " + r.PathValue("name")}
	if err := s.sessions.DeleteScope(r.Context(), r.PathValue("name")); err != nil {
		result.Error = err.Error()
		s.renderFragment(w, tmplOpResult, result)
		return
	}

	w.Header().Set("HX-Redirect", "/sessions-archive")
	s.renderFragment(w, tmplOpResult, result)
}

func (s *Server) handleSessionsScopeUnarchive(w http.ResponseWriter, r *http.Request) {
	result := opResult{Page: pageSessions, Subject: "unarchive " + r.PathValue("name")}
	if err := s.sessions.UnarchiveScope(r.Context(), r.PathValue("name")); err != nil {
		result.Error = err.Error()
		s.renderFragment(w, tmplOpResult, result)
		return
	}

	w.Header().Set("HX-Redirect", "/sessions-archive")
	s.renderFragment(w, tmplOpResult, result)
}

// reloadSessions rescans per request because the store has no watcher.
func (s *Server) reloadSessions() {
	if err := s.sessions.Reload(); err != nil {
		log.Printf("sessions: reload failed, serving previous state: %v", err)
	}
}

func parseSessionIds(query url.Values) []string {
	var ids []string
	for _, value := range query["session"] {
		for _, id := range strings.Split(value, ",") {
			if id = strings.TrimSpace(id); id != "" {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

// shortId keeps ids that are already short (hand-edited fixtures) intact.
func shortId(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func signalLabel(signal string) string {
	if strings.HasPrefix(strings.ToLower(signal), "yes") {
		return "analyzed"
	}
	return signal
}

// contextSessionsAll attributes each session to the deepest registered repo holding its cwd.
func contextSessionsAll(peekSessions map[string]peek.Session, registered []repos.Repo) []contextSessionView {
	byId := make(map[string]contextSessionView)
	depth := make(map[string]int)
	for _, repo := range registered {
		for _, view := range contextSessionsUnder(peekSessions, repo.Path) {
			if len(repo.Path) <= depth[view.Session.Id] {
				continue
			}
			view.Repo = repo.Name
			byId[view.Session.Id] = view
			depth[view.Session.Id] = len(repo.Path)
		}
	}

	views := slices.Collect(maps.Values(byId))
	sortNewestFirst(views)
	return views
}

// contextSessionsUnder assumes every peek-known session has a context report.
func contextSessionsUnder(peekSessions map[string]peek.Session, root string) []contextSessionView {
	root = filepath.Clean(root)
	var views []contextSessionView
	for _, session := range peekSessions {
		isUnderRoot := session.Cwd == root || strings.HasPrefix(session.Cwd, root+string(filepath.Separator))
		if !isUnderRoot {
			continue
		}
		views = append(views, contextSessionView{Session: session, ShortId: shortId(session.Id)})
	}
	sortNewestFirst(views)
	return views
}

// findingRows keeps the record's own index in every anchor, so a dimension filter never renumbers a row.
func findingRows(dimension string, session *sessions.Session) []findingRowView {
	var rows []findingRowView
	for index, finding := range session.Findings {
		if dimension != "" && finding.Dimension != dimension {
			continue
		}

		headline := finding.Headline
		if headline == "" {
			headline = firstSentence(finding.Summary)
		}
		row := findingRowView{
			Anchor:     fmt.Sprintf("f-%s-%d", session.Id, index),
			Dimension:  finding.Dimension,
			Headline:   headline,
			QuoteCount: len(finding.Quotes),
			Quotes:     finding.Quotes,
			Snippets:   finding.Snippets,
			Summary:    finding.Summary,
		}
		rows = append(rows, row)
	}
	return rows
}

// firstSentence stands in for the headline of a record mined before format 2.
func firstSentence(summary string) string {
	cut := len(summary)
	for _, mark := range []string{". ", ": "} {
		if index := strings.Index(summary, mark); index >= 0 && index < cut {
			cut = index
		}
	}

	sentence := summary[:cut]
	if utf8.RuneCountInString(sentence) <= headlineFallbackRunes {
		return sentence
	}
	return string([]rune(sentence)[:headlineFallbackRunes]) + "…"
}

// groupContextSessionsByWeek expects newest-first input; days and weeks follow the local calendar.
func groupContextSessionsByWeek(location *time.Location, views []contextSessionView) []contextWeekGroup {
	var weeks []contextWeekGroup
	for _, view := range views {
		local := view.Session.LastActive.In(location)
		day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
		key := sessions.WeekKey(day)
		if len(weeks) == 0 || weeks[len(weeks)-1].Key != key {
			week := sessions.Week{Key: key}
			weeks = append(weeks, contextWeekGroup{Key: key, Label: week.Label()})
		}
		weeks[len(weeks)-1].add(day, view)
	}
	return weeks
}

// sessionDays groups the visible sessions by local day, newest first; the newest day and any day holding a selected or opened session start open.
func sessionDays(location *time.Location, open string, views []sessionView) []sessionDayView {
	byKey := make(map[string]*sessionDayView)
	for _, view := range views {
		key, label := unknownDayKey, unknownDayLabel
		if stamp, err := time.Parse(time.RFC3339, view.Session.LastActive); err == nil {
			local := stamp.In(location)
			key, label = local.Format(time.DateOnly), local.Format("Mon 2006-01-02")
		}

		day, ok := byKey[key]
		if !ok {
			day = &sessionDayView{Key: key, Label: label}
			byKey[key] = day
		}
		day.Sessions = append(day.Sessions, view)
		isTargeted := view.Selected || view.Session.Id == open
		day.IsOpen = day.IsOpen || isTargeted
	}

	days := make([]sessionDayView, 0, len(byKey))
	for _, key := range slices.Sorted(maps.Keys(byKey)) {
		days = append(days, *byKey[key])
	}
	slices.Reverse(days)
	if len(days) > 0 {
		days[0].IsOpen = true
	}
	return days
}

func sessionRecordURL(id string, ref sessions.SessionRef) string {
	return fmt.Sprintf("/sessions/%s/%s?open=%s#session-%s", ref.Scope, ref.Week, id, id)
}

// sortNewestFirst breaks equal times by id so a map-built slice renders stably.
func sortNewestFirst(views []contextSessionView) {
	slices.SortFunc(views, func(a, b contextSessionView) int {
		if byTime := b.Session.LastActive.Compare(a.Session.LastActive); byTime != 0 {
			return byTime
		}
		return strings.Compare(a.Session.Id, b.Session.Id)
	})
}

func visibleSessions(dimension string, peekSessions map[string]peek.Session, sessionIds []string, week *sessions.Week) []sessionView {
	var views []sessionView
	for index := range week.Sessions {
		session := &week.Sessions[index]
		if len(sessionIds) > 0 && !slices.Contains(sessionIds, session.Id) {
			continue
		}

		rows := findingRows(dimension, session)
		if dimension != "" && len(rows) == 0 && !hasQuoteEntries(session, dimension) {
			continue
		}

		// A non-empty selection hides non-members, so every visible session is selected.
		peekSession, hasReport := peekSessions[session.Id]
		view := sessionView{
			HasReport:   hasReport,
			Live:        peekSession.Live,
			Rows:        rows,
			Selected:    len(sessionIds) > 0,
			Session:     session,
			ShortId:     shortId(session.Id),
			SignalLabel: signalLabel(session.Signal),
		}
		views = append(views, view)
	}
	return views
}

func weekDimensions(week *sessions.Week) []string {
	dimensions := make(map[string]bool)
	for index := range week.Sessions {
		session := &week.Sessions[index]
		for _, finding := range session.Findings {
			dimensions[finding.Dimension] = true
		}
		dimensions[dimFrustration] = dimensions[dimFrustration] || len(session.Frustration) > 0
		dimensions[dimPositive] = dimensions[dimPositive] || len(session.Positive) > 0
	}

	var present []string
	for _, name := range slices.Sorted(maps.Keys(dimensions)) {
		if dimensions[name] {
			present = append(present, name)
		}
	}
	return present
}

func hasQuoteEntries(session *sessions.Session, dimension string) bool {
	switch dimension {
	case dimFrustration:
		return len(session.Frustration) > 0
	case dimPositive:
		return len(session.Positive) > 0
	}
	return false
}

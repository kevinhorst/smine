package server

import (
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/server/respond"
	"github.com/kevinhorst/smine/internal/sessions"
)

const (
	pageSessions      = "sessions"
	tmplBatchBody     = "_batch_body.html"
	tmplSessionsBatch = "sessions_batch.html"
	tmplSessionsIndex = "sessions_index.html"

	// dimFrustration/dimPositive are pseudo-dimensions (D10): backed by the
	// per-session quote arrays, not by findings.
	dimFrustration = "frustration"
	dimPositive    = "positive"
)

type batchPage struct {
	Batch         *sessions.BatchSummary
	Dimension     string
	Dimensions    []string
	Open          string
	Page          string
	SessionIds    []string
	SessionsParam string
	Sessions      []sessionView
	ThemeBullets  []string
	ThemeLabel    string
	Title         string
	ToggleAllURL  string
}

type sessionsIndexPage struct {
	BatchBands []batchBand
	Page       string
	Scope      *sessions.ScopeInfo
	ScopeNames []string
	Title      string
}

type batchBand struct {
	Batches   []*sessions.BatchSummary
	DateRange string
	Label     string
}

type sessionView struct {
	Findings    []sessions.Finding
	Selected    bool
	Session     *sessions.Session
	ShortId     string
	SignalLabel string
}

func (s *Server) handleSessionsBatch(w http.ResponseWriter, r *http.Request) {
	scope := r.PathValue("scope")
	number, err := strconv.Atoi(r.PathValue("batch"))
	if err != nil {
		respond.WithBadRequest("invalid batch number", w)
		return
	}

	batch, ok := s.sessions.Batch(scope, number)
	if !ok {
		http.NotFound(w, r)
		return
	}

	dimensions := make(map[string]bool)
	for _, session := range batch.Sessions {
		for _, finding := range session.Findings {
			dimensions[finding.Dimension] = true
		}
		if len(session.Frustration) > 0 {
			dimensions[dimFrustration] = true
		}
		if len(session.Positive) > 0 {
			dimensions[dimPositive] = true
		}
	}

	themeLabel, themeBulletList := themeBullets(batch.Batch.Theme)
	sessionIds := parseSessionIds(r.URL.Query())
	data := batchPage{
		Batch:         batch,
		Dimension:     r.URL.Query().Get("dimension"),
		Dimensions:    slices.Sorted(maps.Keys(dimensions)),
		Open:          r.URL.Query().Get("open"),
		Page:          pageSessions,
		SessionIds:    sessionIds,
		SessionsParam: strings.Join(sessionIds, ","),
		ThemeBullets:  themeBulletList,
		ThemeLabel:    themeLabel,
		Title:         fmt.Sprintf("%s · Batch %d", scope, number),
	}
	data.Sessions = visibleSessions(batch, data.Dimension, sessionIds)
	data.ToggleAllURL = toggleAllURL(batch, data.Dimension, sessionIds)
	if r.Header.Get("HX-Request") != "" {
		s.renderFragment(w, tmplBatchBody, data)
		return
	}

	s.renderFragment(w, tmplSessionsBatch, data)
}

func (s *Server) handleSessionsIndex(w http.ResponseWriter, r *http.Request) {
	scopes := s.sessions.Scopes()
	if len(scopes) == 0 {
		data := sessionsIndexPage{Page: pageSessions, Title: "Sessions"}
		s.renderFragment(w, tmplSessionsIndex, data)
		return
	}

	http.Redirect(w, r, "/sessions/"+scopes[0].Name, http.StatusFound)
}

func (s *Server) handleSessionsReload(w http.ResponseWriter, r *http.Request) {
	result := opResult{Page: pageSessions, Subject: "reload sessions"}
	if err := s.sessions.Reload(); err != nil {
		result.Error = err.Error()
	}

	// The batch list re-pulls itself on this event so the current scope
	// refreshes in place.
	w.Header().Set("HX-Trigger", "sessions-reload")
	s.renderFragment(w, tmplOpResult, result)
}

func (s *Server) handleSessionsScope(w http.ResponseWriter, r *http.Request) {
	scope := r.PathValue("scope")
	info, ok := s.sessions.Scope(scope)
	if !ok {
		http.NotFound(w, r)
		return
	}

	names := make([]string, 0)
	for _, scopeInfo := range s.sessions.Scopes() {
		names = append(names, scopeInfo.Name)
	}
	data := sessionsIndexPage{
		BatchBands: batchBands(info.Batches),
		Page:       pageSessions,
		Scope:      info,
		ScopeNames: names,
		Title:      "Sessions — " + scope,
	}
	s.renderFragment(w, tmplSessionsIndex, data)
}

// batchBands groups batches into decades (Batch 1-10, 11-20, …) by number.
// Batches arrive sorted by number, so each decade is contiguous — one pass.
func batchBands(batches []*sessions.BatchSummary) []batchBand {
	var bands []batchBand
	for _, batch := range batches {
		decade := (batch.Batch.Number - 1) / 10
		label := fmt.Sprintf("Batch %d-%d", decade*10+1, decade*10+10)
		if n := len(bands); n > 0 && bands[n-1].Label == label {
			bands[n-1].Batches = append(bands[n-1].Batches, batch)
			continue
		}
		bands = append(bands, batchBand{Batches: []*sessions.BatchSummary{batch}, Label: label})
	}
	for index := range bands {
		bands[index].DateRange = bandDateRange(bands[index].Batches)
	}
	return bands
}

// isoDatePattern extracts full ISO dates from the free-form batch date
// strings ("2026-06-07/10", "2026-05-27 → 2026-05-29", prose suffixes).
var isoDatePattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

func bandDateRange(batches []*sessions.BatchSummary) string {
	first, last := "", ""
	for _, batch := range batches {
		source := batch.Batch.DateRange
		if source == "" {
			source = batch.Batch.AnalyzedDate
		}
		for _, date := range isoDatePattern.FindAllString(source, -1) {
			if first == "" || date < first {
				first = date
			}
			if date > last {
				last = date
			}
		}
	}
	if first == "" {
		return ""
	}
	if first == last {
		return first
	}
	return first + " → " + last
}

func themeBullets(theme string) (string, []string) {
	if theme == "" {
		return "", nil
	}

	label, rest := "", theme
	if index := strings.Index(theme, ":"); index >= 0 {
		label = strings.TrimSpace(theme[:index+1])
		rest = theme[index+1:]
	}

	separator := ","
	if strings.Contains(rest, ";") {
		separator = ";"
	}

	var bullets []string
	for _, segment := range strings.Split(rest, separator) {
		if trimmed := strings.TrimSpace(segment); trimmed != "" {
			bullets = append(bullets, trimmed)
		}
	}
	return label, bullets
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

func toggleAllURL(batch *sessions.BatchSummary, dimension string, selected []string) string {
	query := url.Values{}
	if dimension != "" {
		query.Set("dimension", dimension)
	}
	if len(selected) == 0 {
		ids := make([]string, 0, len(batch.Sessions))
		for index := range batch.Sessions {
			ids = append(ids, batch.Sessions[index].Id)
		}
		query.Set("session", strings.Join(ids, ","))
	}
	base := fmt.Sprintf("/sessions/%s/%d", batch.Batch.Scope, batch.Batch.Number)
	if len(query) == 0 {
		return base
	}
	return base + "?" + query.Encode()
}

func visibleSessions(batch *sessions.BatchSummary, dimension string, sessionIds []string) []sessionView {
	var views []sessionView
	for index := range batch.Sessions {
		session := &batch.Sessions[index]
		if len(sessionIds) > 0 && !slices.Contains(sessionIds, session.Id) {
			continue
		}

		findings := matchingFindings(session, dimension)
		if dimension != "" && len(findings) == 0 && !hasQuoteEntries(session, dimension) {
			continue
		}

		// A non-empty selection hides non-members, so every visible
		// session is selected.
		view := sessionView{
			Findings:    findings,
			Selected:    len(sessionIds) > 0,
			Session:     session,
			ShortId:     shortId(session.Id),
			SignalLabel: signalLabel(session.Signal),
		}
		views = append(views, view)
	}
	return views
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

func matchingFindings(session *sessions.Session, dimension string) []sessions.Finding {
	if dimension == "" {
		return session.Findings
	}

	var findings []sessions.Finding
	for _, finding := range session.Findings {
		if finding.Dimension == dimension {
			findings = append(findings, finding)
		}
	}
	return findings
}

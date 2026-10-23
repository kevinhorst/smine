package server

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/budget"
	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/server/respond"
)

const (
	pageUsage           = "usage"
	tmplUsage           = "usage.html"
	usageCcusageBinary  = "ccusage"
	usageCsvContentType = "text/csv; charset=utf-8"
	usageOff            = "the Usage page needs Routines and peek or ccusage"
	usageOnlyDiffer     = "differ"
	usagePath           = "/usage"
	// usageRecalculateTimeout bounds one ccusage pass over the whole archive.
	usageRecalculateTimeout = 10 * time.Minute
)

// usageCcusageDirs cover the launchd PATH, which usually lacks ccusage.
var usageCcusageDirs = []string{"/opt/homebrew/bin", "/usr/local/bin"}

// usageQuery is the page's whole state, all carried in the URL.
type usageQuery struct {
	agent  string
	days   int
	from   string
	only   string
	repo   string
	skill  string
	source usageSource
	to     string
	unit   usageUnit
}

func parseUsageQuery(r *http.Request) usageQuery {
	values := r.URL.Query()
	query := usageQuery{
		agent:  values.Get("agent"),
		days:   budget.DefaultDays,
		from:   values.Get("from"),
		only:   values.Get("only"),
		repo:   values.Get("repo"),
		skill:  values.Get("skill"),
		source: newUsageSource(values.Get("source")),
		to:     values.Get("to"),
		unit:   newUsageUnit(values.Get("unit")),
	}
	if days, err := strconv.Atoi(values.Get("days")); err == nil && slices.Contains(budget.PresetDays, days) {
		query.days = days
	}
	return query
}

// filterValues is everything but the period, defaults omitted.
func (q usageQuery) filterValues() url.Values {
	values := url.Values{}
	setIfPresent(values, "agent", q.agent)
	setIfPresent(values, "only", q.only)
	setIfPresent(values, "repo", q.repo)
	setIfPresent(values, "skill", q.skill)
	switch q.source {
	case usageSourceBoth, usageSourceCcusage:
		values.Set("source", string(q.source))
	case usageSourcePeek:
	}
	if q.unit == usageUnitPlan {
		values.Set("unit", string(q.unit))
	}
	return values
}

// forCsv is the export's query: always both sources, in dollars.
func (q usageQuery) forCsv() usageQuery {
	q.source, q.unit = usageSourceBoth, usageUnitUsd
	return q
}

// forPage drops the disagreement filter unless both sources are shown; a skill filter reads peek's runs only.
func (q usageQuery) forPage() usageQuery {
	if q.skill != "" {
		q.source = usageSourcePeek
	}
	if q.source != usageSourceBoth {
		q.only = ""
	}
	return q
}

func (q usageQuery) hasRange() bool {
	return q.from != "" || q.to != ""
}

// period is the custom range when either date is given, else the preset.
func (q usageQuery) period(now time.Time) (budget.Period, error) {
	if !q.hasRange() {
		return budget.NewPresetPeriod(q.days, now), nil
	}
	if q.from == "" || q.to == "" {
		return budget.Period{}, errors.New("usageQuery.period: A custom range needs both from and to")
	}
	return budget.NewRangePeriod(q.from, now.Location(), q.to)
}

// url renders the query on path, defaults omitted.
func (q usageQuery) url(path string) string {
	values := q.filterValues()
	switch {
	case q.hasRange():
		values.Set("from", q.from)
		values.Set("to", q.to)
	case q.days != budget.DefaultDays:
		values.Set("days", strconv.Itoa(q.days))
	}
	if len(values) == 0 {
		return path
	}
	return path + "?" + values.Encode()
}

// usageSource is which side of every pair the page shows.
type usageSource string

const (
	usageSourceBoth    usageSource = "both"
	usageSourceCcusage usageSource = "ccusage"
	usageSourcePeek    usageSource = "peek"
)

// usageSourceOrder is the chip order of the sources.
var usageSourceOrder = []usageSource{usageSourcePeek, usageSourceCcusage, usageSourceBoth}

// newUsageSource reads a URL's source value; anything unknown is peek.
func newUsageSource(value string) usageSource {
	switch source := usageSource(value); source {
	case usageSourceBoth, usageSourceCcusage:
		return source
	case usageSourcePeek:
	}
	return usageSourcePeek
}

// fit keeps the source when available, else takes the first available one.
func (q usageQuery) fit(available []usageSource) usageQuery {
	if len(available) > 0 && !slices.Contains(available, q.source) {
		q.source = available[0]
	}
	return q
}

// usageUnit is what the cost-per-day chart counts in.
type usageUnit string

const (
	usageUnitPlan usageUnit = "plan"
	usageUnitUsd  usageUnit = "usd"
)

// newUsageUnit reads a URL's unit value; anything unknown is dollars.
func newUsageUnit(value string) usageUnit {
	if usageUnit(value) == usageUnitPlan {
		return usageUnitPlan
	}
	return usageUnitUsd
}

func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	if !s.hasUsage() {
		s.renderComponentOff(w, pageUsage, labelUsage, components.Routines, components.Peek, components.Ccusage)
		return
	}

	view, err := s.loadUsage(time.Now(), parseUsageQuery(r).forPage())
	if err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}
	s.renderFragment(w, tmplUsage, view.page())
}

func (s *Server) handleUsageCsv(w http.ResponseWriter, r *http.Request) {
	if !s.hasUsage() {
		respond.WithBadRequest(usageOff, w)
		return
	}

	view, err := s.loadUsage(time.Now(), parseUsageQuery(r).forCsv())
	if err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}

	filename := fmt.Sprintf(
		"usage-%s-%s.csv",
		view.period.From.Format(time.DateOnly),
		view.period.LastDay().Format(time.DateOnly),
	)
	w.Header().Set("Content-Type", usageCsvContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	if err := csv.NewWriter(w).WriteAll(view.csvRows()); err != nil {
		log.Printf("Server.handleUsageCsv: Failed to write response: %v", err)
	}
}

// handleUsagePlan stores the plan in the usage store, never in the profile every session is shown.
func (s *Server) handleUsagePlan(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		respond.WithBadRequest("invalid form", w)
		return
	}

	// plan
	raw := r.PostForm.Get("plan")
	plan, isKnown := budget.NewPlan(raw)
	if !isKnown {
		respond.WithBadRequest(fmt.Sprintf("plan %q is not a known plan", raw), w)
		return
	}

	settings := &budget.Settings{Plan: plan}
	if err := s.usage.SaveSettings(settings); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	http.Redirect(w, r, usageReturnUrl(r.Referer()), http.StatusSeeOther)
}

// handleUsageRecalculate backfills new ccusage fields into records captured before the change.
func (s *Server) handleUsageRecalculate(w http.ResponseWriter, r *http.Request) {
	if !s.records.has(s.machine, components.Ccusage) {
		respond.WithBadRequest("the ccusage component is off", w)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), usageRecalculateTimeout)
	defer cancel()

	ccusage := &budget.Ccusage{ArchiveDir: s.usage.ArchiveDir(), BinaryPath: s.ccusagePath()}
	saved, err := ccusage.Recalculate(ctx, s.usage)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	log.Printf("Server.handleUsageRecalculate: Recalculated %d records", saved)
	http.Redirect(w, r, usageReturnUrl(r.Referer()), http.StatusSeeOther)
}

// usageSources lists the sources the selected components provide, both only with peek and ccusage on.
func (s *Server) usageSources() []usageSource {
	var sources []usageSource
	if s.records.has(s.machine, components.Peek) {
		sources = append(sources, usageSourcePeek)
	}
	if s.records.has(s.machine, components.Ccusage) {
		sources = append(sources, usageSourceCcusage)
	}
	if len(sources) == len(usageSourceOrder)-1 {
		sources = append(sources, usageSourceBoth)
	}
	return sources
}

func (s *Server) ccusagePath() string {
	if s.usageCcusagePath != "" {
		return s.usageCcusagePath
	}
	if path, err := exec.LookPath(usageCcusageBinary); err == nil {
		return path
	}
	for _, dir := range usageCcusageDirs {
		candidate := filepath.Join(dir, usageCcusageBinary)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return usageCcusageBinary
}

// loadUsage reads the usage store once and builds the view of one query, fitted to the selected sources.
func (s *Server) loadUsage(now time.Time, query usageQuery) (*usageView, error) {
	period, err := query.period(now)
	if err != nil {
		return nil, err
	}

	sources := s.usageSources()
	view := &usageView{location: now.Location(), period: period, plan: budget.DefaultPlan, query: query.fit(sources), sources: sources}
	records, err := s.usage.Records()
	if err != nil {
		view.storeErr = err.Error()
	}
	settings, err := s.usage.Settings()
	if err == nil {
		view.plan = settings.Plan
	}
	state, err := s.usage.State()
	if err == nil {
		view.state = state
	}
	view.load(records)
	return view, nil
}

// usageReturnUrl sends a plan change back to the Usage page, the Overview or the session report it came from, query kept.
func usageReturnUrl(referer string) string {
	parsed, err := url.Parse(referer)
	if err != nil {
		return usagePath
	}

	isPlanPage := parsed.Path == usagePath || parsed.Path == "/"
	if !isPlanPage && !strings.HasPrefix(parsed.Path, sessionReportPathPrefix) {
		return usagePath
	}
	if parsed.RawQuery == "" {
		return parsed.Path
	}
	return parsed.Path + "?" + parsed.RawQuery
}

func setIfPresent(values url.Values, key, value string) {
	if value != "" {
		values.Set(key, value)
	}
}

package server

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/server/respond"
)

// standardSetting is a cross-routine setting every routine supports via its
// plist env; the wrapper default applies when the key is absent (D2, D3).
type standardSetting struct {
	Key     string
	Label   string
	Default string
}

var standardSettings = []standardSetting{
	{Key: "ROUTINE_MODEL", Label: "Model", Default: "claude-opus-4-8[1m]"},
	{Key: "ROUTINE_MAX_BUDGET_USD", Label: "Max budget (USD)", Default: "15"},
	{Key: "ROUTINE_PERMISSION_MODE", Label: "Permission mode", Default: "acceptEdits"},
}

// routineSettings declares routine-specific settings rendered on that
// routine's configure widget; absent key = wrapper default, like
// standardSettings (D3). Values are validated per key (validateSmineSetting).
var routineSettings = map[string][]standardSetting{
	"smine-nightly": {
		{Key: "SMINE_AUTO_APPLY", Label: "Auto-apply mode", Default: "never"},
		{Key: "SMINE_AUTO_APPLY_DIMENSIONS", Label: "Auto-apply dimensions (always mode)", Default: "all"},
		{Key: "SMINE_APPLY_CAP", Label: "Max proposals applied", Default: "3"},
		{Key: "SMINE_MAX_PROPOSALS_MINED", Label: "Max proposals mined", Default: "unlimited"},
		{Key: "SMINE_MAX_PROPOSALS_PER_DIMENSION", Label: "Max proposals mined per dimension", Default: "unlimited"},
	},
}

var autoApplyModes = []string{"never", "decide", "always"}

var autoApplyDimensions = []string{"context", "routines", "skills", "style", "workflows"}

// declaredExamples holds placeholder examples for known routine-specific keys (D7).
var declaredExamples = map[string]string{
	"ROUTINE_CADENCE_DAYS": "1",
}

type standardSettingView struct {
	standardSetting
	Value string
}

func settingsFor(routineName string) []standardSetting {
	return append(slices.Clone(standardSettings), routineSettings[routineName]...)
}

func isSettingKey(key, routineName string) bool {
	for _, setting := range settingsFor(routineName) {
		if setting.Key == key {
			return true
		}
	}
	return false
}

// validateSmineSetting rejects unparseable values for the smine-nightly
// settings; unknown keys pass through untouched (generic env stays free-form).
func validateSmineSetting(key, value string) error {
	switch key {
	case "SMINE_AUTO_APPLY":
		if !slices.Contains(autoApplyModes, value) {
			return fmt.Errorf("validateSmineSetting: Invalid mode %s: must be never, decide or always", value)
		}
	case "SMINE_AUTO_APPLY_DIMENSIONS":
		for _, dimension := range strings.Split(value, ",") {
			if !slices.Contains(autoApplyDimensions, strings.TrimSpace(dimension)) {
				return fmt.Errorf("validateSmineSetting: Invalid dimension %s: must be context, routines, skills, style or workflows", strings.TrimSpace(dimension))
			}
		}
	case "SMINE_APPLY_CAP", "SMINE_MAX_PROPOSALS_MINED", "SMINE_MAX_PROPOSALS_PER_DIMENSION":
		number, err := strconv.Atoi(value)
		if err != nil || number < 1 {
			return fmt.Errorf("validateSmineSetting: Invalid value for %s: must be a positive integer", key)
		}
	}
	return nil
}

const (
	pageRoutines         = "routines"
	tmplRoutineConfigure = "_routine_configure.html"
	tmplRoutineDetail    = "routine_detail.html"
	tmplRoutineRow       = "_routine_row.html"
	tmplRoutineRowOOB    = "_routine_row_oob.html"
	tmplRoutinesIndex    = "routines_index.html"

	// statusStopPolling is htmx's stop-polling response code (D25).
	statusStopPolling = 286
)

type routineDetailPage struct {
	History    []routines.RunResult
	HistoryErr string
	Page       string
	Routine    routines.Routine
	Row        routineView
	Title      string
}

type routineView struct {
	EnvPairs  []envPair
	Loaded    bool
	LoadedErr string
	OOB       bool
	PollSince int64
	Routine   routines.Routine
}

type envPair struct {
	Example string
	Key     string
	Value   string
}

type routineConfigureView struct {
	EnvPairs []envPair
	Name     string
	Repos    []repos.Repo
	Standard []standardSettingView
}

type routinesIndexPage struct {
	Page  string
	Rows  []routineView
	Title string
}

func (s *Server) findRoutine(w http.ResponseWriter, r *http.Request) *routines.Routine {
	list, err := routines.Scan(s.routinesDir)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return nil
	}

	for index := range list {
		if list[index].Name == r.PathValue("name") {
			return &list[index]
		}
	}
	http.NotFound(w, r)
	return nil
}

func (s *Server) rescanRoutine(routine *routines.Routine) *routines.Routine {
	list, err := routines.Scan(s.routinesDir)
	if err != nil {
		return routine
	}

	for index := range list {
		if list[index].Name == routine.Name {
			return &list[index]
		}
	}

	return routine
}

func (s *Server) routineView(ctx context.Context, routine *routines.Routine) routineView {
	view := routineView{EnvPairs: declaredPairs(routine.Env, routine.Name), Routine: *routine}
	if routine.LoadError != "" {
		return view
	}

	loaded, err := routines.IsLoaded(ctx, routine.Label)
	if err != nil {
		view.LoadedErr = err.Error()
		return view
	}

	view.Loaded = loaded
	return view
}

// declaredPairs lists routine-specific env sorted by key; the standard
// settings are excluded — they render as settings, not env (D4).
func declaredPairs(env map[string]string, routineName string) []envPair {
	var pairs []envPair
	for _, key := range slices.Sorted(maps.Keys(env)) {
		if isSettingKey(key, routineName) {
			continue
		}
		pairs = append(pairs, envPair{Key: key, Value: env[key], Example: declaredExamples[key]})
	}

	return pairs
}

func (s *Server) renderRoutineResult(ctx context.Context, output string, pollSince int64, resultErr error, routine *routines.Routine, w http.ResponseWriter) {
	result := opResult{Output: output, Page: pageRoutines, Subject: routine.Name}
	if resultErr != nil {
		result.Error = resultErr.Error()
	}
	s.renderFragment(w, tmplOpResult, result)

	view := s.routineView(ctx, routine)
	view.OOB = true
	if resultErr == nil {
		view.PollSince = pollSince
	}
	s.renderFragment(w, tmplRoutineRowOOB, view)
}

func (s *Server) handleRoutineDetail(w http.ResponseWriter, r *http.Request) {
	routine := s.findRoutine(w, r)
	if routine == nil {
		return
	}

	data := routineDetailPage{Page: pageRoutines, Routine: *routine, Title: "Routine — " + routine.Name}
	history, err := routines.History(routine.ResultsPath, routines.HistoryLimit())
	if err != nil {
		data.HistoryErr = err.Error()
	}

	data.History = history
	data.Row = s.routineView(r.Context(), routine)

	s.renderFragment(w, tmplRoutineDetail, data)
}

func (s *Server) handleRoutineReschedule(w http.ResponseWriter, r *http.Request) {
	routine := s.findRoutine(w, r)
	if routine == nil {
		return
	}

	interval, err := intervalFromForm(r)
	if err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}

	if err := routines.Reschedule(interval, routine.PlistPath); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	output, err := s.reloadRoutine(r.Context(), routine)
	s.renderRoutineResult(r.Context(), output, 0, err, s.rescanRoutine(routine), w)
}

func (s *Server) handleRoutineConfigure(w http.ResponseWriter, r *http.Request) {
	routine := s.findRoutine(w, r)
	if routine == nil {
		return
	}

	view := routineConfigureView{
		EnvPairs: declaredPairs(routine.Env, routine.Name),
		Name:     routine.Name,
		Repos:    s.repoRegistry.Repos(),
	}

	for _, setting := range settingsFor(routine.Name) {
		view.Standard = append(view.Standard, standardSettingView{standardSetting: setting, Value: routine.Env[setting.Key]})
	}

	s.renderFragment(w, tmplRoutineConfigure, view)
}

func (s *Server) handleRoutineParams(w http.ResponseWriter, r *http.Request) {
	routine := s.findRoutine(w, r)
	if routine == nil {
		return
	}

	if err := r.ParseForm(); err != nil {
		respond.WithBadRequest("invalid form", w)
		return
	}

	keys, values := r.PostForm["key"], r.PostForm["value"]
	if len(keys) != len(values) {
		respond.WithBadRequest("key/value fields must pair up", w)
		return
	}

	env := make(map[string]string)
	for index, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			respond.WithBadRequest("empty param key", w)
			return
		}
		value := strings.TrimSpace(values[index])
		if value == "" && isSettingKey(key, routine.Name) {
			continue // absent key = wrapper default (D3)
		}
		if value != "" {
			if err := validateSmineSetting(key, value); err != nil {
				respond.WithBadRequest(err.Error(), w)
				return
			}
		}
		env[key] = value
	}

	if err := routines.SetEnv(env, routine.PlistPath); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	output, err := s.reloadRoutine(r.Context(), routine)
	s.renderRoutineResult(r.Context(), output, 0, err, s.rescanRoutine(routine), w)
}

func (s *Server) handleRoutineDuplicate(w http.ResponseWriter, r *http.Request) {
	routine := s.findRoutine(w, r)
	if routine == nil {
		return
	}

	if err := routines.Duplicate(routine.Name, r.FormValue("newname"), s.routinesDir); err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}

	w.Header().Set("HX-Trigger", "repo-op")
	result := opResult{Page: pageRoutines, Subject: "duplicate " + routine.Name}

	s.renderFragment(w, tmplOpResult, result)
}

func (s *Server) handleRoutineRow(w http.ResponseWriter, r *http.Request) {
	routine := s.findRoutine(w, r)
	if routine == nil {
		return
	}

	since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
	view := s.routineView(r.Context(), routine)

	if hasResultAfter(routine.LastRun, since) {
		w.WriteHeader(statusStopPolling)
	} else {
		view.PollSince = since
	}

	s.renderFragment(w, tmplRoutineRow, view)
}

func (s *Server) handleRoutineRun(w http.ResponseWriter, r *http.Request) {
	routine := s.findRoutine(w, r)
	if routine == nil {
		return
	}

	output, err := routines.RunNow(r.Context(), routine.Label)
	// The row polls /routines/{name}/row?since=<now> until a newer results
	// line appears
	s.renderRoutineResult(r.Context(), output, time.Now().Unix(), err, routine, w)
}

func (s *Server) handleRoutineStart(w http.ResponseWriter, r *http.Request) {
	routine := s.findRoutine(w, r)
	if routine == nil {
		return
	}

	output, err := routines.Start(r.Context(), routine.Label, routine.PlistPath)
	s.renderRoutineResult(r.Context(), output, 0, err, routine, w)
}

func (s *Server) handleRoutineStop(w http.ResponseWriter, r *http.Request) {
	routine := s.findRoutine(w, r)
	if routine == nil {
		return
	}

	output, err := routines.Stop(r.Context(), routine.Label)
	s.renderRoutineResult(r.Context(), output, 0, err, routine, w)
}

func (s *Server) handleRoutinesIndex(w http.ResponseWriter, r *http.Request) {
	list, err := routines.Scan(s.routinesDir)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	data := routinesIndexPage{Page: pageRoutines, Title: "Routines"}
	for index := range list {
		data.Rows = append(data.Rows, s.routineView(r.Context(), &list[index]))
	}
	s.renderFragment(w, tmplRoutinesIndex, data)
}

func hasResultAfter(last *routines.RunResult, sinceUnix int64) bool {
	if last == nil {
		return false
	}

	finished, err := time.Parse(time.RFC3339, last.Timestamp)
	if err != nil {
		return false
	}
	return finished.Unix() > sinceUnix
}

func intervalFromForm(r *http.Request) (routines.CalendarInterval, error) {
	interval := routines.CalendarInterval{}

	// hour
	hour, err := strconv.Atoi(r.FormValue("hour"))
	if err != nil {
		return interval, errors.New("intervalFromForm: Invalid parameter hour")
	}
	if hour < 0 || hour > 23 {
		return interval, errors.New("intervalFromForm: Invalid parameter hour")
	}
	interval.Hour = &hour

	// minute
	minute, err := strconv.Atoi(r.FormValue("minute"))
	if err != nil {
		return interval, errors.New("intervalFromForm: Invalid parameter minute")
	}
	if minute < 0 || minute > 59 {
		return interval, errors.New("intervalFromForm: Invalid parameter minute")
	}
	interval.Minute = &minute

	// weekday (optional; launchd: 0 and 7 are both Sunday)
	if raw := r.FormValue("weekday"); raw != "" {
		weekday, err := strconv.Atoi(raw)
		if err != nil {
			return interval, errors.New("intervalFromForm: Invalid parameter weekday")
		}
		if weekday < 0 || weekday > 7 {
			return interval, errors.New("intervalFromForm: Invalid parameter weekday")
		}
		interval.Weekday = &weekday
	}

	if raw := r.FormValue("day"); raw != "" {
		if interval.Weekday != nil {
			return interval, errors.New("intervalFromForm: Parameters day and weekday are mutually exclusive")
		}
		day, err := strconv.Atoi(raw)
		if err != nil {
			return interval, errors.New("intervalFromForm: Invalid parameter day")
		}
		if day < 1 || day > 31 {
			return interval, errors.New("intervalFromForm: Invalid parameter day")
		}
		interval.Day = &day
	}
	return interval, nil
}

// reloadRoutine bounces a loaded job so launchd picks up the rewritten
// schedule; an unloaded routine needs nothing.
func (s *Server) reloadRoutine(ctx context.Context, routine *routines.Routine) (string, error) {
	loaded, err := routines.IsLoaded(ctx, routine.Label)
	if err != nil {
		return "", err
	}
	if !loaded {
		return "", nil
	}

	stopOutput, err := routines.Stop(ctx, routine.Label)
	if err != nil {
		return stopOutput, err
	}

	startOutput, err := routines.Start(ctx, routine.Label, routine.PlistPath)
	return stopOutput + startOutput, err
}

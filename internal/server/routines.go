package server

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/peek"
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

// routineModelDefault mirrors run.sh's ROUTINE_MODEL fallback; the profile
// style test runs on it too, so the test answer previews the model the
// pipeline actually writes with.
const routineModelDefault = "claude-opus-4-8[1m]"

var standardSettings = []standardSetting{
	{Key: "ROUTINE_EXTRA_PROMPT", Label: "Extra prompt (appended to the routine prompt)", Default: ""},
	{Key: "ROUTINE_MODEL", Label: "Model", Default: routineModelDefault},
	{Key: "ROUTINE_MAX_BUDGET_USD", Label: "Max budget (USD)", Default: "15"},
	{Key: "ROUTINE_MAX_OPEN_BRANCHES", Label: "Max open branches (un-merged)", Default: "unlimited"},
	{Key: "ROUTINE_PERMISSION_MODE", Label: "Permission mode", Default: "acceptEdits"},
	{Key: "ROUTINE_TOKEN", Label: "Token (account)", Default: "default token file"},
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
		{Key: "SMINE_AGENTS", Label: "Agents mined", Default: "claude,codex"},
	},
}

var autoApplyModes = []string{"never", "decide", "always"}

var autoApplyDimensions = []string{"context", "routines", "skills", "style"}

var mineAgents = []string{"claude", "codex"}

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
				return fmt.Errorf("validateSmineSetting: Invalid dimension %s: must be context, routines, skills or style", strings.TrimSpace(dimension))
			}
		}
	case "SMINE_AGENTS":
		for _, agent := range strings.Split(value, ",") {
			if !slices.Contains(mineAgents, strings.TrimSpace(agent)) {
				return fmt.Errorf("validateSmineSetting: Invalid agent %s: must be claude or codex", strings.TrimSpace(agent))
			}
		}
	case "SMINE_APPLY_CAP", "SMINE_MAX_PROPOSALS_MINED", "SMINE_MAX_PROPOSALS_PER_DIMENSION", "ROUTINE_MAX_OPEN_BRANCHES":
		number, err := strconv.Atoi(value)
		if err != nil || number < 1 {
			return fmt.Errorf("validateSmineSetting: Invalid value for %s: must be a positive integer", key)
		}
	}
	return nil
}

// pinnedRoutine always sorts first on the routines index — the routine the
// page exists for.
const pinnedRoutine = "smine-nightly"

// runNowPromptFileName is the one-shot Run-Now prompt extension in the
// routine dir; run.sh appends its content to the prompt and deletes it.
const runNowPromptFileName = ".run-now-prompt"

const (
	pageRoutines            = "routines"
	tmplRoutineConfigure    = "_routine_configure.html"
	tmplRoutineConfigureOOB = "_routine_configure_oob.html"
	tmplRoutineDetail       = "routine_detail.html"
	tmplRoutineRow          = "_routine_row.html"
	tmplRoutineRowOOB       = "_routine_row_oob.html"
	tmplRoutinesIndex       = "routines_index.html"

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
	Loaded    bool
	LoadedErr string
	OOB       bool
	Pinned    bool
	PollSince int64
	Routine   routines.Routine
	Session   *peek.Session // peek session at the routine's worktree cwd; nil = none/peek down
}

type envPair struct {
	Example string
	Key     string
	Value   string
}

type routineConfigureView struct {
	AutoApplyContent string
	AutoApplyError   string
	AutoApplyPath    string // empty = this routine has no rules editor
	EnvPairs         []envPair
	Name             string
	Repos            []repos.Repo
	Standard         []standardSettingView
	TokenErr         string
	TokenLabels      []string
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

func (s *Server) routineView(ctx context.Context, routine *routines.Routine, sessions map[string]peek.Session) routineView {
	view := routineView{Pinned: routine.Name == pinnedRoutine, Routine: *routine}
	if routine.LoadError != "" {
		return view
	}
	view.Session = routineSession(routine, sessions)

	loaded, err := routines.IsLoaded(ctx, routine.Label)
	if err != nil {
		view.LoadedErr = err.Error()
		return view
	}

	view.Loaded = loaded
	return view
}

// routineSession picks the peek session running in the routine's worktrees —
// per the worktree.sh contract the group worktree is
// $ROUTINE_WT_ROOT/$ROUTINE_GROUP (default
// ~/.cache/claude-routine/worktrees/<name>), and matrix.sh runs eval cells in
// detached worktrees under the sibling <group>-cells/. Sessions may sit in
// subdirectories of either, so matching is by prefix; the most recently
// active match wins.
func routineSession(routine *routines.Routine, sessions map[string]peek.Session) *peek.Session {
	if len(sessions) == 0 {
		return nil
	}
	root := routine.Env["ROUTINE_WT_ROOT"]
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		root = filepath.Join(home, ".cache", "claude-routine", "worktrees")
	}
	group := routine.Env["ROUTINE_GROUP"]
	if group == "" {
		group = routine.Name
	}

	groupWorktree := filepath.Clean(filepath.Join(root, group))
	cellsRoot := groupWorktree + "-cells"
	var latest *peek.Session
	for cwd, session := range sessions {
		isGroupCwd := cwd == groupWorktree || strings.HasPrefix(cwd, groupWorktree+string(filepath.Separator))
		isCellCwd := strings.HasPrefix(cwd, cellsRoot+string(filepath.Separator))
		if !isGroupCwd && !isCellCwd {
			continue
		}
		if latest == nil || session.LastActive.After(latest.LastActive) {
			latest = &session
		}
	}
	return latest
}

// peekRoutineSessions is the routines-page peek lookup; peek down → nil map,
// the pages render with dashed session cells (mirrors repos D3).
func (s *Server) peekRoutineSessions(ctx context.Context) map[string]peek.Session {
	index, err := s.peekClient.SessionIndex(ctx)
	if err != nil {
		return nil
	}
	return index.ByCwd
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

	view := s.routineView(ctx, routine, s.peekRoutineSessions(ctx))
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
	data.Row = s.routineView(r.Context(), routine, s.peekRoutineSessions(r.Context()))

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

	s.renderFragment(w, tmplRoutineConfigure, s.routineConfigureViewFor(routine))
}

// routineConfigureViewFor builds the configure-panel view; shared by the
// configure GET and the params POST, which re-renders the panel (D7).
func (s *Server) routineConfigureViewFor(routine *routines.Routine) routineConfigureView {
	view := routineConfigureView{
		EnvPairs: declaredPairs(routine.Env, routine.Name),
		Name:     routine.Name,
		Repos:    s.repoRegistry.Repos(),
	}

	for _, setting := range settingsFor(routine.Name) {
		view.Standard = append(view.Standard, standardSettingView{standardSetting: setting, Value: routine.Env[setting.Key]})
	}

	labels, err := routines.ListTokenLabels(s.tokenDir)
	if err != nil {
		view.TokenErr = err.Error()
	}
	view.TokenLabels = labels

	// The auto-apply decide-rules editor lives with the routine that
	// declares the mode setting (smine-nightly).
	if isSettingKey("SMINE_AUTO_APPLY", routine.Name) {
		view.AutoApplyPath = s.autoApplyRulesPath
		content, err := os.ReadFile(s.autoApplyRulesPath)
		if err != nil {
			view.AutoApplyError = err.Error()
		}
		view.AutoApplyContent = string(content)
	}

	return view
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

	tokenLabels, err := routines.ListTokenLabels(s.tokenDir)
	if err != nil {
		respond.WithInternalServerError(err, w)
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
			// Stricter than the ROUTINE_TARGET_REPO precedent by design:
			// an unknown label guarantees exit 78 every night (D9).
			if key == "ROUTINE_TOKEN" && !slices.Contains(tokenLabels, value) {
				respond.WithBadRequest(fmt.Sprintf("unknown token label %q", value), w)
				return
			}
		}
		env[key] = value
	}

	// The decide-rules textarea rides the same form for the routine that
	// hosts the editor; absent field (other routines) writes nothing.
	if content, ok := r.PostForm["auto_apply_content"]; ok && isSettingKey("SMINE_AUTO_APPLY", routine.Name) {
		if len(content[0]) > maxAutoApplyRulesBytes {
			respond.WithBadRequest("rules file exceeds 64 KiB", w)
			return
		}
		if err := s.saveAutoApplyRules(content[0]); err != nil {
			respond.WithInternalServerError(err, w)
			return
		}
	}

	if err := routines.SetEnv(env, routine.PlistPath); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	output, err := s.reloadRoutine(r.Context(), routine)
	rescanned := s.rescanRoutine(routine)
	s.renderRoutineResult(r.Context(), output, 0, err, rescanned, w)
	// Re-render the open panel out-of-band so the persisted values show up
	// without reopening Configure.
	s.renderFragment(w, tmplRoutineConfigureOOB, s.routineConfigureViewFor(rescanned))
}

// handleRoutineTokenAdd stores a new token from the standalone add-token
// widget. The value is write-only: never rendered or logged (D8).
func (s *Server) handleRoutineTokenAdd(w http.ResponseWriter, r *http.Request) {
	label, value := strings.TrimSpace(r.FormValue("token_label")), strings.TrimSpace(r.FormValue("token_value"))
	if value == "" {
		respond.WithBadRequest("token value required", w)
		return
	}

	labels, err := routines.ListTokenLabels(s.tokenDir)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	if label == "" {
		label = defaultTokenLabel(labels)
	}
	if err := routines.SaveToken(label, value, s.tokenDir); err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}

	s.renderFragment(w, tmplOpResult, opResult{Page: pageRoutines, Subject: "add token " + label})
}

// defaultTokenLabel names an unlabeled token by creation date, suffixed on
// same-day collision (token-2026-08-12, token-2026-08-12-2, …) (D5).
func defaultTokenLabel(labels []string) string {
	base := time.Now().Format("token-2006-01-02")
	label := base
	for suffix := 2; slices.Contains(labels, label); suffix++ {
		label = fmt.Sprintf("%s-%d", base, suffix)
	}
	return label
}

func (s *Server) handleRoutineDuplicate(w http.ResponseWriter, r *http.Request) {
	routine := s.findRoutine(w, r)
	if routine == nil {
		return
	}

	suffix := r.FormValue("suffix")
	if suffix == "" {
		respond.WithBadRequest("suffix required", w)
		return
	}
	if err := routines.Duplicate(routine.Name, routine.Name+"-"+suffix, s.routinesDir); err != nil {
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
	view := s.routineView(r.Context(), routine, s.peekRoutineSessions(r.Context()))

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

	// One-shot Run-Now extension: written before the kickstart, consumed and
	// deleted by the wrapper at run start — it never leaks into later runs.
	prompt := strings.TrimSpace(r.FormValue("prompt"))
	if prompt != "" {
		promptPath := filepath.Join(routine.Dir, runNowPromptFileName)
		if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
			respond.WithInternalServerError(fmt.Errorf("handleRoutineRun: Failed to write %s: %w", promptPath, err), w)
			return
		}
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
	sessions := s.peekRoutineSessions(r.Context())
	for index := range list {
		data.Rows = append(data.Rows, s.routineView(r.Context(), &list[index], sessions))
	}
	// The pinned routine leads; Scan's alphabetical order holds behind it.
	sort.SliceStable(data.Rows, func(i, j int) bool {
		return data.Rows[i].Pinned && !data.Rows[j].Pinned
	})
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
// schedule; an unloaded routine needs nothing. A job with a live instance is
// never bounced — the bounce is a bootout, which kills the running process
// tree mid-run; the saved file keeps its changes and the reload happens on
// the next Reschedule/Save/Stop once the run has ended.
func (s *Server) reloadRoutine(ctx context.Context, routine *routines.Routine) (string, error) {
	loaded, err := routines.IsLoaded(ctx, routine.Label)
	if err != nil {
		return "", err
	}
	if !loaded {
		return "", nil
	}

	running, err := routines.IsRunning(ctx, routine.Label)
	if err != nil {
		return "", err
	}
	if running {
		return "saved, but a run is active — reload skipped so the run survives; save again (or Stop/Start) after it finishes to apply the change", nil
	}

	stopOutput, err := routines.Stop(ctx, routine.Label)
	if err != nil {
		return stopOutput, err
	}

	startOutput, err := routines.Start(ctx, routine.Label, routine.PlistPath)
	return stopOutput + startOutput, err
}

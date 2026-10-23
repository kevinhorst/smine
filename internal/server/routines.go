package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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

	"github.com/kevinhorst/smine/internal/budget"
	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/models"
	"github.com/kevinhorst/smine/internal/peek"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/server/respond"
)

// standardSetting falls back to the wrapper default when the key is absent.
type standardSetting struct {
	Default           string
	Help              string
	IsContainerDetail bool // shown only while the Docker container setting is on
	Key               string
	Label             string
}

// smineNightlyModel mirrors the smine-nightly template's ROUTINE_MODEL; keep the two in step.
const smineNightlyModel = "claude-opus-5-5[1m]"

var standardSettings = []standardSetting{
	{Key: "ROUTINE_EXTRA_PROMPT", Label: "Extra prompt", Help: "Appended to every run's prompt.", Default: ""},
	{Key: "ROUTINE_MODEL", Label: "Model", Help: "Model for the routine's claude calls; empty = the settings.json model, like an interactive session.", Default: "settings.json model"},
	{Key: "ROUTINE_MAX_BUDGET_USD", Label: "Max budget (USD)", Help: "Optional spend cap per claude call; empty = no cap, like an interactive session.", Default: "none"},
	{Key: "ROUTINE_MAX_OPEN_BRANCHES", Label: "Max open branches", Help: "Un-merged dated branches the group may keep; at the cap a run exits 0 without a branch.", Default: "unlimited"},
	{Key: "ROUTINE_PERMISSION_MODE", Label: "Permission mode", Help: "claude --permission-mode for every call; empty = settings.json defaultMode, like an interactive session.", Default: "settings.json defaultMode"},
	{Key: "ROUTINE_TOKEN", Label: "Token (account)", Help: "Which stored setup-token the run authenticates with.", Default: "default token file"},
	{Key: routines.EnvContainer, Label: "Docker container", Help: "Runs every claude cell of a run in a fenced, bounded Docker container.", Default: "off"},
	{Key: routines.EnvContainerMounts, Label: "Container mounts", Help: "Registered repos a container run sees at their host path; the tool-policy gate still decides what the agent may touch.", Default: "none", IsContainerDetail: true},
	{Key: routines.EnvContainerCpus, Label: "Container CPUs", Help: "docker --cpus of each container cell.", Default: routines.DefaultContainerCpus, IsContainerDetail: true},
	{Key: routines.EnvContainerMemory, Label: "Container memory", Help: "docker --memory of each container cell, e.g. 8g.", Default: routines.DefaultContainerMemory, IsContainerDetail: true},
}

var dockerMemoryRe = regexp.MustCompile(`^[0-9]+[bkmg]?$`)

// mountAccessField prefixes the access select of each picked repo: mount_access:<repo>.
const mountAccessField = "mount_access:"

const mountsPlaceholder = "select repos"

// routineSettings entries replace a standard setting of the same key for that routine.
var routineSettings = map[string][]standardSetting{
	"smine-nightly": {
		{Key: "ROUTINE_MAX_BUDGET_USD", Label: "Run budget (USD)", Help: "Spend cap of the whole nightly pipeline session, workflow agents included.", Default: "100"},
		{Key: "SMINE_MAX_PROPOSALS_MINED", Label: "Max proposals per night", Help: "Change tasks each mining stage may write; the night's proposals are at most this many per stage.", Default: "5"},
		{Key: "SMINE_PROPOSAL_WINDOW_NIGHTS", Label: "Nights kept", Help: "Proposals of the newest N mined nights stay; older ones are dropped and listed under History.", Default: "2"},
		{Key: "SMINE_MAX_SESSIONS_MINED", Label: "Max sessions mined per run", Help: "Newest first within peek's 14-day window; each session gets its own miner agent.", Default: "30"},
		{Key: "SMINE_AGENTS", Label: "Agents mined", Default: "claude,codex"},
		{Key: "SMINE_SUBAGENTS", Label: "Mine subagent transcripts", Help: "1 = also mine each session's subagent transcripts (one extra peek pull per subagent); 0 = top-level only.", Default: "0"},
		{Key: "ROUTINE_ORCHESTRATE", Label: "Judge stage", Help: "1 = smine-orchestrate judges the run before the runner publishes it; 0 = publish without the judge.", Default: "1"},
	},
}

var mineAgents = []string{"claude", "codex"}

// retiredKeys are hidden from the form, so the next Save drops them from the plist.
var retiredKeys = map[string][]string{
	"smine-nightly": {
		"SMINE_APPLY_CAP",
		"SMINE_AUTO_APPLY",
		"SMINE_AUTO_APPLY_DIMENSIONS",
		"SMINE_MAX_PROPOSALS_PER_DIMENSION",
	},
}

// declaredSettings labels cross-routine plist keys that stay declared in the plist rather than as settings.
var declaredSettings = map[string]standardSetting{
	"ROUTINE_CADENCE_DAYS": {Key: "ROUTINE_CADENCE_DAYS", Label: "Run every N days", Help: "Skips scheduled runs until N days passed; empty or 1 = every run.", Default: "1"},
	"ROUTINE_TARGET_REPO":  {Key: "ROUTINE_TARGET_REPO", Label: "Target repo", Help: "The clone the run works on.", Default: ""},
}

// excludedKeys are standard settings a routine kind never reads; the form hides them.
var excludedKeys = map[string][]string{
	"smine-nightly": {"ROUTINE_MAX_OPEN_BRANCHES"},
}

type standardSettingView struct {
	standardSetting
	Value string
}

func settingsFor(routineName string) []standardSetting {
	merged := slices.Clone(standardSettings)
	merged = slices.DeleteFunc(merged, func(setting standardSetting) bool { return slices.Contains(excludedKeys[routineName], setting.Key) })
	for _, override := range routineSettings[routineName] {
		index := slices.IndexFunc(merged, func(setting standardSetting) bool { return setting.Key == override.Key })
		if index >= 0 {
			merged[index] = override
			continue
		}
		merged = append(merged, override)
	}
	return merged
}

func isSettingKey(key, routineName string) bool {
	for _, setting := range settingsFor(routineName) {
		if setting.Key == key {
			return true
		}
	}
	return false
}

// validateSmineSetting passes unknown keys through: generic env stays free-form.
func validateSmineSetting(key string, vocabulary *models.Vocabulary, value string) error {
	switch key {
	case "SMINE_AGENTS":
		for _, agent := range strings.Split(value, ",") {
			if !slices.Contains(mineAgents, strings.TrimSpace(agent)) {
				return fmt.Errorf("validateSmineSetting: Invalid agent %s: must be claude or codex", strings.TrimSpace(agent))
			}
		}
	case "SMINE_SUBAGENTS", "ROUTINE_ORCHESTRATE":
		if value != "0" && value != "1" {
			return fmt.Errorf("validateSmineSetting: Invalid value %s for %s: must be 0 or 1", value, key)
		}
	case "SMINE_MAX_PROPOSALS_MINED", "SMINE_MAX_SESSIONS_MINED", "SMINE_PROPOSAL_WINDOW_NIGHTS", "ROUTINE_MAX_OPEN_BRANCHES":
		number, err := strconv.Atoi(value)
		if err != nil || number < 1 {
			return fmt.Errorf("validateSmineSetting: Invalid value for %s: must be a positive integer", key)
		}
	case "ROUTINE_MODEL":
		if err := vocabulary.Check(value); err != nil {
			return fmt.Errorf("validateSmineSetting: Invalid model: %w", err)
		}
	case routines.EnvContainer:
		if value != "" && !routines.ContainerSupported {
			return errors.New("validateSmineSetting: Docker container routines are not supported on windows")
		}
		if _, err := routines.ParseContainerFlag(value); err != nil {
			return fmt.Errorf("validateSmineSetting: %w", err)
		}
	case routines.EnvContainerCpus:
		cpus, err := strconv.ParseFloat(value, 64)
		if err != nil || cpus <= 0 {
			return fmt.Errorf("validateSmineSetting: Invalid value for %s: must be a positive number", key)
		}
	case routines.EnvContainerMemory:
		if !dockerMemoryRe.MatchString(value) {
			return fmt.Errorf("validateSmineSetting: Invalid value for %s: must be a size like 8g", key)
		}
	}
	return nil
}

// pinnedRoutine always sorts first: it is the routine the page exists for.
const pinnedRoutine = "smine-nightly"

// steerLogFileName is server-owned; the delivery pump never touches it.
const steerLogFileName = "steer-log.jsonl"

// Steer state dropped means the prompt slot emptied without a matching receipt.
const (
	steerStateDelivered = "delivered"
	steerStateDropped   = "dropped"
	steerStateQueued    = "queued"
	steerStateReplaced  = "replaced"
)

// pausedFileName marks a SIGSTOPped run as "<rootPid> <RFC3339>".
const pausedFileName = ".paused"

const (
	pageRoutines          = "routines"
	tmplRoutineConfigure  = "_routine_configure.html"
	tmplRoutineDetail     = "routine_detail.html"
	tmplRoutineRun        = "routine_run.html"
	tmplRoutineRow        = "_routine_row.html"
	tmplRoutineRowOOB     = "_routine_row_oob.html"
	tmplRoutineSessions   = "_routine_sessions.html"
	tmplRoutineSteerPanel = "_routine_steer_panel.html"
	tmplRoutinesIndex     = "routines_index.html"

	// statusStopPolling is htmx's stop-polling response code (D25).
	statusStopPolling = 286
	// recentReportDays bounds the run reports shown open; older ones hide behind the show-all toggle.
	recentReportDays = 2
)

type routineDetailPage struct {
	History       []routines.RunResult
	HistoryErr    string
	Page          string
	RecentReports int
	Reports       []routines.RunReport
	ReportsErr    string
	Routine       routines.Routine
	Row           routineView
	Title         string
}

// routineGroup is one collapsible block of the routines list; the pinned routine's group opens by default.
type routineGroup struct {
	Family   string
	IsPinned bool
	Loaded   int
	Rows     []routineView
}

type routineRunPage struct {
	Page    string
	RawJson string
	Report  routines.RunReport
	Routine routines.Routine
	Title   string
}

type routineView struct {
	Loaded         bool
	LoadedErr      string
	OOB            bool
	Paused         bool // .paused marker present and the run is still live
	PauseSupported bool
	PendingReload  bool // .pending-reload marker present — a deferred reload awaits the run's end
	Pinned         bool
	PollSince      int64
	Routine        routines.Routine
	SteerEntries   []steerEntry // steer history from the log, newest first
	SteerState     string       // newest entry's state when queued|delivered, else ""
}

// routineSessionsView loads after the row renders so a slow or cold peek never holds the page.
type routineSessionsView struct {
	Sessions    []peek.Session // matching peek sessions, newest first, capped; empty = none
	Unavailable string         // peek error root cause; empty when peek answered
}

type envPair struct {
	Example string
	Help    string
	Key     string
	Label   string
	Value   string
}

// containerMountView is one repo of the Container mounts picker and, when selected, its row in the access table.
type containerMountView struct {
	Access      string
	IsMountable bool // false for a stored mount the registry no longer offers
	IsSelected  bool
	Repo        string
}

// routineConfigureView is the Configure card; a save answers with it, the reload's outcome beside the Save button.
type routineConfigureView struct {
	EnvPairs    []envPair
	HasMounts   bool
	IsError     bool
	Models      []string
	ModelsErr   string
	Mounts      []containerMountView
	MountsErr   string
	MountsLabel string
	Name        string
	Notice      string
	Output      string
	Repos       []repos.Repo
	Standard    []standardSettingView
	TokenErr    string
	TokenLabels []string
}

// globalRunRow is one run in the cross-routine history (D8).
type globalRunRow struct {
	IsLive  bool // a started row whose run the scheduler still runs
	Result  routines.RunResult
	Routine string
}

type routinesIndexPage struct {
	GlobalHistory []globalRunRow
	Groups        []routineGroup
	Page          string
	Title         string
}

// globalHistory relies on RFC3339 stamps sorting lexically in chronological order.
func globalHistory(list []routines.Routine) []globalRunRow {
	var rows []globalRunRow
	for index := range list {
		history, err := routines.History(list[index].ResultsPath, routines.HistoryLimit())
		if err != nil {
			continue
		}
		for _, result := range history {
			row := globalRunRow{IsLive: result.IsRunning() && list[index].IsLive, Result: result, Routine: list[index].Name}
			rows = append(rows, row)
		}
	}

	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].Result.Timestamp > rows[j].Result.Timestamp
	})
	if len(rows) > routines.HistoryLimit() {
		rows = rows[:routines.HistoryLimit()]
	}
	return rows
}

// leadsWhen orders a flagged element before an unflagged one; equals otherwise.
func leadsWhen(left, right bool) int {
	switch {
	case left && !right:
		return -1
	case right && !left:
		return 1
	}
	return 0
}

// recentReportCount counts the leading reports (newest first) started within the last recentReportDays local days.
func recentReportCount(reports []routines.RunReport, now time.Time) int {
	period := budget.NewPresetPeriod(recentReportDays, now)
	count := 0
	for _, report := range reports {
		started, err := time.Parse(time.RFC3339, report.Started)
		if err != nil || !period.Contains(started) {
			break
		}
		count++
	}
	return count
}

// routineGroups keys a row by its manifest group, else its name; the pinned routine's group leads and it leads its group.
func routineGroups(rows []routineView) []routineGroup {
	familyOf := func(row routineView) string {
		if row.Routine.Group != "" {
			return row.Routine.Group
		}
		return row.Routine.Name
	}
	slices.SortStableFunc(rows, func(left, right routineView) int {
		if byFamily := strings.Compare(familyOf(left), familyOf(right)); byFamily != 0 {
			return byFamily
		}
		return strings.Compare(left.Routine.Name, right.Routine.Name)
	})

	var groups []routineGroup
	for _, row := range rows {
		family := familyOf(row)
		if n := len(groups); n == 0 || groups[n-1].Family != family {
			groups = append(groups, routineGroup{Family: family})
		}
		group := &groups[len(groups)-1]
		group.Rows = append(group.Rows, row)
		group.IsPinned = group.IsPinned || row.Pinned
		if row.Loaded {
			group.Loaded++
		}
	}
	for index := range groups {
		slices.SortStableFunc(groups[index].Rows, func(left, right routineView) int { return leadsWhen(left.Pinned, right.Pinned) })
	}
	slices.SortStableFunc(groups, func(left, right routineGroup) int { return leadsWhen(left.IsPinned, right.IsPinned) })
	return groups
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
	view := routineView{Pinned: routine.Name == pinnedRoutine, Routine: *routine}
	if routine.LoadError != "" {
		return view
	}
	view.PauseSupported = routines.PauseSupported
	if _, err := os.Stat(filepath.Join(routine.Dir, routines.PendingReloadMarker)); err == nil {
		view.PendingReload = true
	}
	if steerDir, err := routines.SteerDir(routine.Env, routine.Name); err == nil {
		view.SteerEntries = liveRunSteers(routineSteerEntries(steerDir), routine)
		if len(view.SteerEntries) > 0 {
			newest := view.SteerEntries[0].State
			if newest == steerStateQueued || newest == steerStateDelivered {
				view.SteerState = newest
			}
		}
	}

	pausedPath := filepath.Join(routine.Dir, pausedFileName)
	if _, err := os.Stat(pausedPath); err == nil {
		running, err := routines.IsRunning(ctx, routine.Label)
		if err == nil && !running {
			// The run died or was killed while paused — stale marker.
			_ = os.Remove(pausedPath)
		} else {
			view.Paused = true
		}
	}

	loaded, err := routines.IsLoaded(ctx, routine.Label)
	if err != nil {
		view.LoadedErr = err.Error()
		return view
	}

	view.Loaded = loaded
	return view
}

// maxRoutineSessions caps the sessions listed per routine row.
const maxRoutineSessions = 3

// routineMatcher also matches the branch prefix, since branch identity survives the recycled worktree path.
type routineMatcher struct {
	branchPrefix  string
	groupWorktree string
}

func newRoutineMatcher(routine *routines.Routine) (routineMatcher, error) {
	groupWorktree, err := routines.GroupWorktree(routine.Env, routine.Name)
	if err != nil {
		return routineMatcher{}, fmt.Errorf("newRoutineMatcher: %w", err)
	}

	group := filepath.Base(groupWorktree)
	matcher := routineMatcher{
		branchPrefix:  "claude-routines/" + group + "-",
		groupWorktree: groupWorktree,
	}
	return matcher, nil
}

func (m routineMatcher) matches(session peek.Session) bool {
	isGroupCwd := session.Cwd == m.groupWorktree || strings.HasPrefix(session.Cwd, m.groupWorktree+string(filepath.Separator))
	if isGroupCwd {
		return true
	}
	return strings.HasPrefix(session.GitBranch, m.branchPrefix)
}

// routineSessions lists only live sessions when any is live: old runs' sessions are clutter then.
func routineSessions(routine *routines.Routine, sessions []peek.Session, max int) []peek.Session {
	matcher, err := newRoutineMatcher(routine)
	if err != nil {
		return nil
	}

	var matched []peek.Session
	hasLive := false
	for _, session := range sessions {
		if matcher.matches(session) {
			matched = append(matched, session)
			hasLive = hasLive || session.Live
		}
	}
	if hasLive {
		live := matched[:0]
		for _, session := range matched {
			if session.Live {
				live = append(live, session)
			}
		}
		matched = live
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].LastActive.After(matched[j].LastActive)
	})
	if len(matched) > max {
		matched = matched[:max]
	}
	return matched
}

// peekRoutineSessions is bounded: a cold peek blocks session_list for its whole initial scan.
func (s *Server) peekRoutineSessions(ctx context.Context) ([]peek.Session, error) {
	peekCtx, cancel := context.WithTimeout(ctx, peekSessionsTimeout)
	defer cancel()

	sessions, err := s.peekClient.Sessions(peekCtx, []string{"claude"})
	if err != nil {
		return nil, fmt.Errorf("Server.peekRoutineSessions: %w", err)
	}
	return sessions, nil
}

// mountsLabel mirrors the picker summary the configure fragment's script rewrites on change.
func mountsLabel(repoNames []string) string {
	if len(repoNames) == 0 {
		return mountsPlaceholder
	}
	return strings.Join(repoNames, ", ")
}

// routineRepoRoot is the checkout holding routines/<name>, the runner's own repo.
func routineRepoRoot(routine *routines.Routine) string {
	return filepath.Dir(filepath.Dir(routine.Dir))
}

// declaredPairs excludes the standard settings: they render as settings, not env.
func declaredPairs(env map[string]string, routineName string) []envPair {
	var pairs []envPair
	for _, key := range slices.Sorted(maps.Keys(env)) {
		if isSettingKey(key, routineName) || slices.Contains(retiredKeys[routineName], key) {
			continue
		}
		declared := declaredSettings[key]
		pairs = append(pairs, envPair{Example: declared.Default, Help: declared.Help, Key: key, Label: declared.Label, Value: env[key]})
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

	reports, err := routines.Reports(routine.Dir, routines.HistoryLimit())
	if err != nil {
		data.ReportsErr = err.Error()
	}

	data.Reports = reports
	data.RecentReports = recentReportCount(reports, time.Now())

	s.renderFragment(w, tmplRoutineDetail, data)
}

func (s *Server) handleRoutineRunReport(w http.ResponseWriter, r *http.Request) {
	routine := s.findRoutine(w, r)
	if routine == nil {
		return
	}

	// id — one path segment, no traversal
	id := r.PathValue("id")
	if id == "" || strings.ContainsAny(id, "/\\") || strings.Contains(id, "..") {
		http.NotFound(w, r)
		return
	}

	report, err := routines.LoadReport(filepath.Join(routine.Dir, "runs", id+".json"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	page := routineRunPage{
		Page:    pageRoutines,
		RawJson: string(raw),
		Report:  *report,
		Routine: *routine,
		Title:   "Run — " + id,
	}
	s.renderFragment(w, tmplRoutineRun, page)
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

func (s *Server) routineConfigureViewFor(routine *routines.Routine) routineConfigureView {
	view := routineConfigureView{
		EnvPairs: declaredPairs(routine.Env, routine.Name),
		Name:     routine.Name,
		Repos:    s.repoRegistry.Repos(),
	}

	for _, setting := range settingsFor(routine.Name) {
		isContainerRow := setting.Key == routines.EnvContainer || setting.IsContainerDetail
		if isContainerRow && !routines.ContainerSupported {
			continue
		}
		view.Standard = append(view.Standard, standardSettingView{standardSetting: setting, Value: routine.Env[setting.Key]})
	}

	labels, err := routines.ListTokenLabels(s.tokenDir)
	if err != nil {
		view.TokenErr = err.Error()
	}
	view.TokenLabels = labels

	vocabulary, err := s.vocabulary()
	if err != nil {
		view.ModelsErr = err.Error()
		vocabulary = &models.Vocabulary{}
	}
	view.Models = vocabulary.Available

	stored, err := routines.ParseContainerMounts(routine.Env[routines.EnvContainerMounts])
	if err != nil {
		view.MountsErr = err.Error()
	}
	view.Mounts = s.containerMountViews(routine, stored)

	var selected []string
	for _, mount := range stored {
		selected = append(selected, mount.Repo)
	}
	view.HasMounts = len(selected) > 0
	view.MountsLabel = mountsLabel(selected)

	return view
}

// containerMountViews offers every registered repo but the routine's own; stored mounts the registry lost stay visible.
func (s *Server) containerMountViews(routine *routines.Routine, stored []*routines.ContainerMount) []containerMountView {
	access := make(map[string]string, len(stored))
	for _, mount := range stored {
		access[mount.Repo] = string(mount.Access)
	}

	var views []containerMountView
	offered := make(map[string]bool)
	for _, repo := range s.repoRegistry.Repos() {
		if repo.Path == routineRepoRoot(routine) {
			continue
		}
		offered[repo.Name] = true

		storedAccess, isSelected := access[repo.Name]
		if !isSelected {
			storedAccess = string(routines.MountReadOnly)
		}
		view := containerMountView{Access: storedAccess, IsMountable: true, IsSelected: isSelected, Repo: repo.Name}
		views = append(views, view)
	}
	for _, mount := range stored {
		if !offered[mount.Repo] {
			view := containerMountView{Access: string(mount.Access), IsMountable: false, IsSelected: true, Repo: mount.Repo}
			views = append(views, view)
		}
	}
	return views
}

// containerMountsFromForm reads the picked repos and each one's access select; "" means none.
func (s *Server) containerMountsFromForm(form url.Values, routine *routines.Routine) (string, error) {
	var mounts []*routines.ContainerMount
	for _, repoName := range form["mount_repo"] {
		access := form.Get(mountAccessField + repoName)
		if access == "" {
			access = string(routines.MountReadOnly)
		}
		mount := &routines.ContainerMount{Access: routines.MountAccess(access), Repo: repoName}
		mounts = append(mounts, mount)
	}

	value := routines.FormatContainerMounts(mounts)
	parsed, err := routines.ParseContainerMounts(value)
	if err != nil {
		return "", fmt.Errorf("containerMountsFromForm: %w", err)
	}

	for _, mount := range parsed {
		repo, isRegistered := s.repoRegistry.Find(mount.Repo)
		if !isRegistered {
			return "", fmt.Errorf("containerMountsFromForm: Repo %s is not registered", mount.Repo)
		}
		if repo.Path == routineRepoRoot(routine) {
			return "", fmt.Errorf("containerMountsFromForm: Repo %s is the routine's own repo", mount.Repo)
		}
	}
	return value, nil
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

	vocabulary, err := s.vocabulary()
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
			if err := validateSmineSetting(key, vocabulary, value); err != nil {
				respond.WithBadRequest(err.Error(), w)
				return
			}
			// Strict by design: an unknown label guarantees exit 78 every night.
			if key == "ROUTINE_TOKEN" && !slices.Contains(tokenLabels, value) {
				respond.WithBadRequest(fmt.Sprintf("unknown token label %q", value), w)
				return
			}
		}
		env[key] = value
	}

	mounts, err := s.containerMountsFromForm(r.PostForm, routine)
	if err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}
	delete(env, routines.EnvContainerMounts)
	if mounts != "" {
		env[routines.EnvContainerMounts] = mounts
	}

	if err := routines.SetEnv(env, routine.PlistPath); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	output, err := s.reloadRoutine(r.Context(), routine)
	rescanned := s.rescanRoutine(routine)
	// The card re-renders from the just-written plist, so the persisted values show without reopening Configure.
	view := s.routineConfigureViewFor(rescanned)
	view.Notice, view.Output = "saved", output
	if err != nil {
		view.IsError, view.Notice = true, err.Error()
	}
	s.renderFragment(w, tmplRoutineConfigure, view)

	row := s.routineView(r.Context(), rescanned)
	row.OOB = true
	s.renderFragment(w, tmplRoutineRowOOB, row)
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

func (s *Server) handleRoutineDelete(w http.ResponseWriter, r *http.Request) {
	routine := s.findRoutine(w, r)
	if routine == nil {
		return
	}

	if err := routines.Delete(r.Context(), routine.Name, s.routinesDir, s.trash); err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}

	w.Header().Set("HX-Trigger", "repo-op")
	s.renderFragment(w, tmplOpResult, opResult{Page: pageRoutines, Subject: "delete " + routine.Name})
}

func (s *Server) handleRoutineRow(w http.ResponseWriter, r *http.Request) {
	routine := s.findRoutine(w, r)
	if routine == nil {
		return
	}

	if _, err := s.applyPendingReload(r.Context(), routine); err != nil {
		log.Printf("handleRoutineRow: %v", err)
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

	output, err := s.runNow(r.Context(), strings.TrimSpace(r.FormValue("prompt")), routine)
	// The row polls /routines/{name}/row?since=<now> until a newer results line appears
	s.renderRoutineResult(r.Context(), output, time.Now().Unix(), err, routine, w)
}

// runNow's extension is deleted by the wrapper at run start, so it never leaks into later runs.
func (s *Server) runNow(ctx context.Context, prompt string, routine *routines.Routine) (string, error) {
	if prompt != "" {
		promptPath := filepath.Join(routine.Dir, routines.RunNowPromptFileName)
		if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
			return "", fmt.Errorf("Server.runNow: Failed to write %s: %w", promptPath, err)
		}
	}
	output, err := routines.RunNow(ctx, routine.Label)
	if err != nil {
		return output, fmt.Errorf("Server.runNow: %w", err)
	}
	return output, nil
}

// steerEntry delivery is read from the pump's receipt and persisted on the next log write.
type steerEntry struct {
	DeliveredAt time.Time `json:"delivered_at,omitzero"`
	Prompt      string    `json:"prompt"`
	QueuedAt    time.Time `json:"queued_at"`
	State       string    `json:"state"` // queued | delivered | replaced
}

// appendSteerEntry marks an undelivered prior entry replaced: the prompt file holds one slot.
func appendSteerEntry(steerDir, prompt string) error {
	logPath := filepath.Join(steerDir, steerLogFileName)
	entries, err := readSteerLog(logPath)
	if err != nil {
		return fmt.Errorf("appendSteerEntry: %w", err)
	}

	entries = reconcileSteerEntries(entries, steerDir)
	for index := range entries {
		if entries[index].State == steerStateQueued {
			entries[index].State = steerStateReplaced
		}
	}

	newEntry := steerEntry{Prompt: prompt, QueuedAt: time.Now().UTC(), State: steerStateQueued}
	entries = append(entries, newEntry)
	return writeSteerLog(logPath, entries)
}

// dropUndelivered: a steer the run never read is not still queued.
func dropUndelivered(entries []steerEntry, isPromptQueued bool) []steerEntry {
	if isPromptQueued {
		return entries
	}
	for index := range entries {
		if entries[index].State == steerStateQueued {
			entries[index].State = steerStateDropped
		}
	}
	return entries
}

// liveRunSteers hides older runs' steers, which stay in the log on disk.
func liveRunSteers(entries []steerEntry, routine *routines.Routine) []steerEntry {
	if !routine.IsLive {
		return nil
	}

	started, err := time.Parse(time.RFC3339, routine.LastRun.Timestamp)
	if err != nil {
		return nil
	}
	var kept []steerEntry
	for _, entry := range entries {
		if !entry.QueuedAt.Before(started) {
			kept = append(kept, entry)
		}
	}
	return kept
}

// readSteerLog parses the jsonl steer log; a missing file is an empty history.
func readSteerLog(logPath string) ([]steerEntry, error) {
	raw, err := os.ReadFile(logPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("readSteerLog: %w", err)
	}

	entries := make([]steerEntry, 0)
	for line := range strings.SplitSeq(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry steerEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("readSteerLog: Bad line %q: %w", line, err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// reconcileSteerEntries falls back to the mtime rule for a legacy plain-text receipt.
func reconcileSteerEntries(entries []steerEntry, steerDir string) []steerEntry {
	isPromptQueued := fsx.HasContent(filepath.Join(steerDir, routines.SteerPromptFileName))
	receiptPath := filepath.Join(steerDir, routines.SteerDeliveredFileName)
	info, err := os.Stat(receiptPath)
	if err != nil {
		return dropUndelivered(entries, isPromptQueued)
	}

	raw, err := os.ReadFile(receiptPath)
	if err != nil {
		return dropUndelivered(entries, isPromptQueued)
	}

	var receipt routines.SteerReceipt
	if json.Unmarshal(raw, &receipt) == nil && receipt.Prompt != "" {
		deliveredAt := receipt.DeliveredAt
		if deliveredAt.IsZero() {
			deliveredAt = info.ModTime().UTC()
		}
		for index := range slices.Backward(entries) {
			if entries[index].State == steerStateQueued && entries[index].Prompt == receipt.Prompt {
				entries[index].State = steerStateDelivered
				entries[index].DeliveredAt = deliveredAt
				break
			}
		}
		return dropUndelivered(entries, isPromptQueued)
	}

	// Legacy receipt (pre-JSON pump): delivery provable only by mtime.
	for index := range entries {
		if entries[index].State == steerStateQueued && info.ModTime().After(entries[index].QueuedAt) {
			entries[index].State = steerStateDelivered
			entries[index].DeliveredAt = info.ModTime().UTC()
		}
	}
	return dropUndelivered(entries, isPromptQueued)
}

// routineSteerEntries returns the reconciled steer history, newest first.
func routineSteerEntries(steerDir string) []steerEntry {
	entries, err := readSteerLog(filepath.Join(steerDir, steerLogFileName))
	if err != nil {
		log.Printf("routineSteerEntries: %v", err)
		return nil
	}

	entries = reconcileSteerEntries(entries, steerDir)
	slices.Reverse(entries)
	return entries
}

func writeSteerLog(logPath string, entries []steerEntry) error {
	var buf strings.Builder
	for _, entry := range entries {
		line, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("writeSteerLog: %w", err)
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	if err := os.WriteFile(logPath, []byte(buf.String()), 0o644); err != nil {
		return fmt.Errorf("writeSteerLog: %w", err)
	}
	return nil
}

func (s *Server) handleRoutineSteerPanel(w http.ResponseWriter, r *http.Request) {
	routine := s.findRoutine(w, r)
	if routine == nil {
		return
	}

	view := s.routineView(r.Context(), routine)
	s.renderFragment(w, tmplRoutineSteerPanel, view)
}

// handleRoutineSessions is requested by the row on load, so the page never waits on peek.
func (s *Server) handleRoutineSessions(w http.ResponseWriter, r *http.Request) {
	routine := s.findRoutine(w, r)
	if routine == nil {
		return
	}

	view := routineSessionsView{}
	sessions, err := s.peekRoutineSessions(r.Context())
	if err != nil {
		view.Unavailable = fmt.Sprintf("%s at %s", rootCause(err), s.peekClient.Endpoint())
	}

	view.Sessions = routineSessions(routine, sessions, maxRoutineSessions)
	s.renderFragment(w, tmplRoutineSessions, view)
}

// handleRoutineSteer writes atomically so the pump never reads a partial file.
func (s *Server) handleRoutineSteer(w http.ResponseWriter, r *http.Request) {
	routine := s.findRoutine(w, r)
	if routine == nil {
		return
	}

	prompt := strings.TrimSpace(r.FormValue("prompt"))
	if prompt == "" {
		respond.WithBadRequest("handleRoutineSteer: Missing parameter prompt", w)
		return
	}

	running, err := routines.IsRunning(r.Context(), routine.Label)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	if !running {
		s.renderRoutineResult(r.Context(), "no active run — use Run now with a prompt instead", 0, nil, routine, w)
		return
	}

	steerDir, err := routines.SteerDir(routine.Env, routine.Name)
	if err != nil {
		respond.WithInternalServerError(fmt.Errorf("handleRoutineSteer: %w", err), w)
		return
	}
	if err := os.MkdirAll(steerDir, 0o755); err != nil {
		respond.WithInternalServerError(fmt.Errorf("handleRoutineSteer: Failed to create %s: %w", steerDir, err), w)
		return
	}

	// Order matters: the pump may deliver the instant the prompt file exists, and its receipt must survive.
	if err := appendSteerEntry(steerDir, prompt); err != nil {
		log.Printf("handleRoutineSteer: %v", err)
	}
	_ = os.Remove(filepath.Join(steerDir, routines.SteerDeliveredFileName))

	steerPath := filepath.Join(steerDir, routines.SteerPromptFileName)
	if err := fsx.WriteFileAtomic([]byte(prompt), steerPath, 0o644); err != nil {
		respond.WithInternalServerError(fmt.Errorf("handleRoutineSteer: %w", err), w)
		return
	}
	s.renderRoutineResult(r.Context(), "steer queued — the run reads it as a user message at its next turn boundary", 0, nil, routine, w)
}

func (s *Server) handleRoutinePause(w http.ResponseWriter, r *http.Request) {
	routine := s.findRoutine(w, r)
	if routine == nil {
		return
	}

	rootPid, err := routines.Pause(r.Context(), routine.Label)
	if err != nil {
		s.renderRoutineResult(r.Context(), "", 0, err, routine, w)
		return
	}
	if err := routines.SignalContainers(r.Context(), routine.Dir, routines.ContainerPause); err != nil {
		log.Printf("handleRoutinePause: %v", err)
	}

	marker := fmt.Sprintf("%d %s\n", rootPid, time.Now().UTC().Format(time.RFC3339))
	markerPath := filepath.Join(routine.Dir, pausedFileName)
	if err := os.WriteFile(markerPath, []byte(marker), 0o644); err != nil {
		s.renderRoutineResult(r.Context(), "", 0, fmt.Errorf("handleRoutinePause: Failed to write %s: %w", markerPath, err), routine, w)
		return
	}
	s.renderRoutineResult(r.Context(), "paused — the dispatcher's deadline excludes paused time; in-flight worker cells' deadlines keep ticking", 0, nil, routine, w)
}

func (s *Server) handleRoutineResume(w http.ResponseWriter, r *http.Request) {
	routine := s.findRoutine(w, r)
	if routine == nil {
		return
	}

	if err := routines.SignalContainers(r.Context(), routine.Dir, routines.ContainerUnpause); err != nil {
		log.Printf("handleRoutineResume: %v", err)
	}

	err := routines.Resume(r.Context(), routine.Label)
	if err == nil {
		_ = os.Remove(filepath.Join(routine.Dir, pausedFileName))
	}
	s.renderRoutineResult(r.Context(), "resumed", 0, err, routine, w)
}

// handleRoutineKill writes the killed row itself: the wrapper's TERM trap was outrun by bootout's KILL escalation.
func (s *Server) handleRoutineKill(w http.ResponseWriter, r *http.Request) {
	routine := s.findRoutine(w, r)
	if routine == nil {
		return
	}

	// A SIGSTOPped tree would hold every TERM pending forever.
	if err := routines.Resume(r.Context(), routine.Label); err != nil {
		log.Printf("handleRoutineKill: Resume: %v", err)
	}
	if err := routines.SignalContainers(r.Context(), routine.Dir, routines.ContainerUnpause); err != nil {
		log.Printf("handleRoutineKill: %v", err)
	}

	cellPidsPath := ""
	if wtDir, err := routines.GroupWorktree(routine.Env, routine.Name); err == nil {
		cellPidsPath = filepath.Join(wtDir, routines.CellPidsFileName)
	}
	killed, err := routines.KillRun(r.Context(), cellPidsPath, routine.Label, routine.PlistPath)
	if err != nil {
		s.renderRoutineResult(r.Context(), "", 0, err, routine, w)
		return
	}
	if killed == 0 {
		s.renderRoutineResult(r.Context(), "no active run", 0, nil, routine, w)
		return
	}

	_ = os.Remove(filepath.Join(routine.Dir, pausedFileName))
	if err := appendKilledResult(routine, killed); err != nil {
		log.Printf("handleRoutineKill: %v", err)
	}
	result := fmt.Sprintf("run killed (%d processes/groups signaled) — schedule stays loaded", killed)
	s.renderRoutineResult(r.Context(), result, 0, nil, routine, w)
}

// killedExitStatus matches the SIGTERM exit (128+15) the wrapper's own trap records.
const killedExitStatus = 143

func appendKilledResult(routine *routines.Routine, killed int) error {
	row := routines.RunResult{
		ExitStatus: killedExitStatus,
		IsError:    true,
		Result:     fmt.Sprintf("run killed via UI (%d processes)", killed),
		Subtype:    "error",
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}
	line, err := json.Marshal(row)
	if err != nil {
		return fmt.Errorf("appendKilledResult: %w", err)
	}

	resultsPath := filepath.Join(routine.Dir, "results.jsonl")
	file, err := os.OpenFile(resultsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("appendKilledResult: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("appendKilledResult: Failed to write %s: %w", resultsPath, err)
	}
	return nil
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
	var rows []routineView
	for index := range list {
		if list[index].LoadError == "" {
			if _, err := s.applyPendingReload(r.Context(), &list[index]); err != nil {
				log.Printf("handleRoutinesIndex: %v", err)
			}
		}
		rows = append(rows, s.routineView(r.Context(), &list[index]))
	}
	data.Groups = routineGroups(rows)
	data.GlobalHistory = globalHistory(list)
	s.renderFragment(w, tmplRoutinesIndex, data)
}

// hasResultAfter ignores start markers, so the row keeps polling until the summary lands.
func hasResultAfter(last *routines.RunResult, sinceUnix int64) bool {
	if last == nil || last.IsRunning() {
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

// reloadRoutine never bounces a live job: the bootout would kill the running process tree.
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
		markerPath := filepath.Join(routine.Dir, routines.PendingReloadMarker)
		marker := time.Now().UTC().Format(time.RFC3339) + " reload-deferred\n"
		if err := os.WriteFile(markerPath, []byte(marker), 0o644); err != nil {
			return "", fmt.Errorf("reloadRoutine: Failed to write %s: %w", markerPath, err)
		}
		return "saved; a run is active — reload deferred, applies automatically once the run ends (Kill ends it now)", nil
	}

	stopOutput, err := routines.Stop(ctx, routine.Label)
	if err != nil {
		return stopOutput, err
	}

	startOutput, err := routines.Start(ctx, routine.Label, routine.PlistPath)
	if err == nil {
		// A completed bounce subsumes any deferred reload.
		_ = os.Remove(filepath.Join(routine.Dir, routines.PendingReloadMarker))
	}
	return stopOutput + startOutput, err
}

// applyPendingReload degrades a lost race to a no-op; bouncing an idle job that raced is harmless.
func (s *Server) applyPendingReload(ctx context.Context, routine *routines.Routine) (bool, error) {
	markerPath := filepath.Join(routine.Dir, routines.PendingReloadMarker)
	if _, err := os.Stat(markerPath); err != nil {
		return false, nil
	}

	loaded, err := routines.IsLoaded(ctx, routine.Label)
	if err != nil {
		return false, fmt.Errorf("applyPendingReload: %s: %w", routine.Name, err)
	}
	if !loaded {
		// Stopped meanwhile: the saved plist loads on the next Start, so the marker is moot.
		_ = os.Remove(markerPath)
		return false, nil
	}

	running, err := routines.IsRunning(ctx, routine.Label)
	if err != nil {
		return false, fmt.Errorf("applyPendingReload: %s: %w", routine.Name, err)
	}
	if running {
		return false, nil
	}

	if _, err := routines.Stop(ctx, routine.Label); err != nil {
		return false, fmt.Errorf("applyPendingReload: %s: %w", routine.Name, err)
	}
	if _, err := routines.Start(ctx, routine.Label, routine.PlistPath); err != nil {
		return false, fmt.Errorf("applyPendingReload: %s: %w", routine.Name, err)
	}

	if err := os.Remove(markerPath); err != nil {
		return true, fmt.Errorf("applyPendingReload: Failed to remove %s: %w", markerPath, err)
	}
	return true, nil
}

// ApplyPendingReloads must run after routines.SyncAll at startup.
func (s *Server) ApplyPendingReloads(ctx context.Context) {
	list, err := routines.Scan(s.routinesDir)
	if err != nil {
		log.Printf("ApplyPendingReloads: %v", err)
		return
	}
	for index := range list {
		if list[index].LoadError != "" {
			continue
		}
		if _, err := s.applyPendingReload(ctx, &list[index]); err != nil {
			log.Printf("ApplyPendingReloads: %v", err)
		}
	}
}

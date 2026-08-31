package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/proposals"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/sessions"
	"github.com/kevinhorst/smine/internal/shell"
)

const (
	pageWelcome       = "welcome"
	tmplWelcome       = "welcome.html"
	tmplWelcomeChecks = "_welcome_checks.html"
	tmplWelcomeVerify = "_welcome_verify.html"
)

const (
	nightlyRoutineName = "smine-nightly"
	// probeTimeout bounds the peek round trip so a down peek degrades a row
	// instead of stalling the page or the overview tile (D4).
	probeTimeout = 2 * time.Second
	fixRoster    = "add your project repos to permissions.additionalDirectories on the Config page — each git repo listed there is mined"
)

// setupCheck is one probe result. A probe never returns error — failures
// fold into Ok=false with Detail carrying the reason (D4).
type setupCheck struct {
	Id     string
	Detail string
	Fix    string
	Group  string
	Name   string
	Ok     bool
}

type welcomeChecksData struct {
	CanVerify bool
	CountOk   int
	Groups    []welcomeGroup
	Total     int
}

type welcomeGroup struct {
	Checks []setupCheck
	Name   string
}

// claudeVerifyResult is the slice of the claude -p --output-format json
// envelope the verification reads to confirm the run and locate its session.
type claudeVerifyResult struct {
	IsError   bool   `json:"is_error"`
	Result    string `json:"result"`
	SessionId string `json:"session_id"`
}

type welcomeVerifyData struct {
	Check       setupCheck
	PeekIndexed bool
	SessionId   string
	ShortId     string
}

type welcomePage struct {
	BootstrapSinceDefault string
	BootstrapToday        string
	Checks                welcomeChecksData
	Page                  string
	Proposal              proposalView
	Tab                   string
	Title                 string
}

func (s *Server) checkClaudeCli() setupCheck {
	check := setupCheck{
		Id:    "claude-cli",
		Fix:   fixInstallClaude,
		Group: "claude runtime",
		Name:  "claude CLI",
	}
	if path, err := exec.LookPath("claude"); err == nil {
		check.Detail = "resolved " + path + " (as seen by the config server)"
		check.Ok = true
		return check
	}
	for _, candidate := range claudeCandidates() {
		if _, err := os.Stat(candidate); err == nil {
			check.Detail = "found " + candidate + " (not on the config server's PATH, but the routine wrapper resolves it)"
			check.Ok = true
			return check
		}
	}

	check.Detail = "not found on the config server's PATH"
	return check
}

func (s *Server) checkNightlyRoutine(ctx context.Context) setupCheck {
	check := setupCheck{
		Id:    "nightly-routine",
		Fix:   fixLoadRoutine,
		Group: "nightly routine",
		Name:  nightlyRoutineName + " loaded",
	}
	list, err := routines.Scan(s.routinesDir)
	if err != nil {
		check.Detail = err.Error()
		return check
	}

	index := slices.IndexFunc(list, func(routine routines.Routine) bool {
		return routine.Name == nightlyRoutineName
	})
	if index < 0 {
		check.Detail = "routines/" + nightlyRoutineName + " not found under " + s.routinesDir
		return check
	}

	routine := list[index]
	if routine.LoadError != "" {
		check.Detail = routine.LoadError
		return check
	}
	loaded, err := routines.IsLoaded(ctx, routine.Label)
	if err != nil {
		check.Detail = err.Error()
		return check
	}
	if !loaded {
		check.Detail = routine.Label + " is not loaded by the scheduler"
		return check
	}

	check.Ok = true
	check.Detail = routine.Label + " — " + lastRunSummary(routine.ResultsPath)
	if history, err := routines.History(routine.ResultsPath, 1); err == nil && len(history) > 0 && history[0].ExitStatus == 78 {
		check.Ok = false
		check.Fix = fixInstallToken
	}
	return check
}

func (s *Server) checkPeekFragment() setupCheck {
	check := setupCheck{
		Id:    "peek-fragment",
		Fix:   fixPeekFragment,
		Group: "peek",
		Name:  "peek config fragment",
	}
	command, args, err := config.McpServer(s.claudeJsonPath, "peek-mcp")
	if err != nil {
		check.Detail = err.Error()
		return check
	}
	if _, err := os.Stat(command); err != nil {
		check.Detail = "configured binary " + command + " does not exist"
		return check
	}
	if !slices.Contains(args, "--control-port=0") {
		check.Detail = "fragment lacks --control-port=0 — per-session peeks would contend for the control port"
		return check
	}

	check.Detail = command + ", --control-port=0"
	check.Ok = true
	return check
}

func (s *Server) checkPeekReachable(ctx context.Context) setupCheck {
	check := setupCheck{
		Id:    "peek-reachable",
		Fix:   fixPeekReachable,
		Group: "peek",
		Name:  "peek reachable",
	}
	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	byId, err := s.peekClient.SessionsById(probeCtx)
	if err != nil {
		check.Detail = err.Error()
		return check
	}

	check.Detail = fmt.Sprintf("%s — %d sessions indexed", s.peekClient.Endpoint(), len(byId))
	check.Ok = true
	return check
}

func (s *Server) checkRoster() setupCheck {
	check := setupCheck{
		Id:    "mining-roster",
		Fix:   fixRoster,
		Group: "nightly routine",
		Name:  "mining roster",
	}
	settings, err := config.Load(s.settingsPath)
	if err != nil {
		check.Detail = err.Error()
		return check
	}
	permissions, err := settings.Permissions()
	if err != nil {
		check.Detail = err.Error()
		return check
	}

	repoCount := 0
	for _, dir := range permissions.AdditionalDirectories {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			repoCount++
		}
	}
	if repoCount == 0 {
		check.Detail = "no git repos in permissions.additionalDirectories — the nightly run has nothing to mine"
		return check
	}

	check.Detail = fmt.Sprintf("%d git repos on the roster", repoCount)
	check.Ok = true
	return check
}

func (s *Server) checkSyncedAssets() setupCheck {
	check := setupCheck{
		Id:    "synced-assets",
		Fix:   fixSyncAssets,
		Group: "deployed home state",
		Name:  "settings, hooks and skills synced",
	}
	if _, err := os.Stat(s.settingsPath); err != nil {
		check.Detail = s.settingsPath + " does not exist"
		return check
	}
	hookCount := countDirEntries(filepath.Join(filepath.Dir(s.settingsPath), "hooks"))
	skillCount := countDirEntries(s.skillsHome)
	if hookCount == 0 || skillCount == 0 {
		check.Detail = fmt.Sprintf("%d hooks, %d skills deployed — sync incomplete", hookCount, skillCount)
		return check
	}

	check.Detail = fmt.Sprintf("settings.json, %d hooks, %d skills deployed", hookCount, skillCount)
	check.Ok = true
	return check
}

func (s *Server) checkToken() setupCheck {
	check := setupCheck{
		Id:    "routine-token",
		Fix:   fixInstallToken,
		Group: "claude runtime",
		Name:  "routine token",
	}
	labels, err := routines.ListTokenLabels(s.tokenDir)
	if err == nil && len(labels) > 0 {
		check.Detail = fmt.Sprintf("%d labeled token(s) stored", len(labels))
		check.Ok = true
		return check
	}
	legacyPath := routines.LegacyTokenPath(s.tokenDir)
	if info, err := os.Stat(legacyPath); err == nil && info.Size() > 0 {
		check.Detail = "token file present at " + legacyPath
		check.Ok = true
		return check
	}

	check.Detail = "no token — the nightly run exits 78 and does nothing"
	return check
}

func (s *Server) handleWelcome(w http.ResponseWriter, r *http.Request) {
	tab := r.URL.Query().Get("tab")
	if tab != "tutorial" {
		tab = "setup"
	}

	data := welcomePage{Page: pageWelcome, Tab: tab, Title: "Welcome"}
	if tab == "setup" {
		data.Checks = s.welcomeChecks(r.Context())
		now := time.Now()
		data.BootstrapSinceDefault = now.AddDate(0, 0, -30).Format("2006-01-02")
		data.BootstrapToday = now.Format("2006-01-02")
	} else {
		data.Proposal = demoProposalView()
	}
	s.renderFragment(w, tmplWelcome, data)
}

func (s *Server) handleWelcomeChecks(w http.ResponseWriter, r *http.Request) {
	s.renderFragment(w, tmplWelcomeChecks, s.welcomeChecks(r.Context()))
}

// handleWelcomeVerifyToken proves the routine token with one 1-turn headless
// claude call. Go never reads the token: bash sources the file, exactly as
// run.sh does. Costs one paid API call — the button carries hx-confirm.
func (s *Server) handleWelcomeVerifyToken(w http.ResponseWriter, r *http.Request) {
	data := welcomeVerifyData{
		Check: setupCheck{
			Id:    "token-verify",
			Fix:   fixInstallToken,
			Group: "claude runtime",
			Name:  "token verified",
		},
	}
	tokenPath, detail := s.verifyTokenPath()
	bashPath := verifyBashPath()
	switch {
	case tokenPath == "":
		data.Check.Detail = detail
	case bashPath == "":
		data.Check.Detail = "no bash available to run the verification"
	default:
		script := verifyPathPrefix + `CLAUDE_CODE_OAUTH_TOKEN="$(tr -d '[:space:]' < "$1")" claude -p 'reply with exactly: ok' --max-turns 1 --output-format json`
		output, err := shell.Run(r.Context(), "", bashPath, "-c", script, "bash", tokenPath)
		if err != nil {
			data.Check.Detail = tailLines(output, 3)
			break
		}
		data = s.verifiedTokenData(r.Context(), output)
	}
	s.renderFragment(w, tmplWelcomeVerify, data)
}

func (s *Server) runSetupChecks(ctx context.Context) []setupCheck {
	checks := make([]setupCheck, 0, 8)
	checks = append(checks, s.checkClaudeCli())
	checks = append(checks, s.checkToken())
	checks = append(checks, platformDepChecks()...)
	checks = append(checks, s.checkPeekFragment())
	checks = append(checks, s.checkPeekReachable(ctx))
	checks = append(checks, s.checkSyncedAssets())
	checks = append(checks, s.checkNightlyRoutine(ctx))
	checks = append(checks, s.checkRoster())
	return checks
}

// verifiedTokenData interprets a successful verification run: the claude
// JSON envelope confirms the answer and names the session, and a peek lookup
// shows the run is visible to the mining loop.
func (s *Server) verifiedTokenData(ctx context.Context, output string) welcomeVerifyData {
	check := setupCheck{
		Id:     "token-verify",
		Detail: "claude answered — token works",
		Fix:    fixInstallToken,
		Group:  "claude runtime",
		Name:   "token verified",
		Ok:     true,
	}
	data := welcomeVerifyData{Check: check}
	start := strings.Index(output, "{")
	if start < 0 {
		return data
	}
	var result claudeVerifyResult
	if err := json.Unmarshal([]byte(output[start:]), &result); err != nil {
		return data
	}
	if result.IsError {
		data.Check.Detail = "claude reported an error: " + result.Result
		data.Check.Ok = false
		return data
	}

	data.Check.Detail = fmt.Sprintf("claude answered %q", result.Result)
	data.SessionId = result.SessionId
	data.ShortId = result.SessionId
	if len(data.ShortId) > 8 {
		data.ShortId = data.ShortId[:8]
	}
	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	if byId, err := s.peekClient.SessionsById(probeCtx); err == nil {
		_, data.PeekIndexed = byId[result.SessionId]
	}
	return data
}

// verifyTokenPath picks the token file the verification runs against: the
// legacy single file, else the first stored label. The path is derived from
// server state, never from request input.
func (s *Server) verifyTokenPath() (string, string) {
	legacyPath := routines.LegacyTokenPath(s.tokenDir)
	if info, err := os.Stat(legacyPath); err == nil && info.Size() > 0 {
		return legacyPath, ""
	}
	labels, err := routines.ListTokenLabels(s.tokenDir)
	if err == nil && len(labels) > 0 {
		return filepath.Join(s.tokenDir, labels[0]), ""
	}
	return "", "no token file to verify — save one first"
}

func (s *Server) welcomeChecks(ctx context.Context) welcomeChecksData {
	checks := s.runSetupChecks(ctx)
	data := welcomeChecksData{
		CanVerify: verifyBashPath() != "",
		Groups:    groupChecks(checks),
		Total:     len(checks),
	}
	for _, check := range checks {
		if check.Ok {
			data.CountOk++
		}
	}
	return data
}

func countDirEntries(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	return len(entries)
}

// demoProposalView builds the tutorial's synthetic proposal and runs it
// through the real card builder, so the example can never drift from the
// live card markup (D7). Vote controls render inert via Disabled.
func demoProposalView() proposalView {
	demo := &proposals.Proposal{
		Id:       "welcome-demo-001",
		Change:   "Add a context rule: resolve merge conflicts by cache → reset → reapply, never by editing conflict hunks in place",
		Evidence: demoEvidence(),
		Fields:   demoFields(),
		Proposed: "2026-08-24",
		Status:   "proposed",
		Tags:     []string{"repo:example"},
		Target:   "context/actions/implementing.md",
		Title:    "Example proposal — reconcile by reapply",
	}
	refs := map[string]sessions.SessionRef{}
	votes := map[string]proposals.Vote{}
	view := proposalCardView("context", demo, refs, votes)
	view.Disabled = true
	return view
}

func demoEvidence() []proposals.Evidence {
	demoSession := proposals.Session{
		Id:   "00000000000000000000000000000000",
		Note: "example session where in-place conflict edits re-broke on the next sync",
	}
	item := proposals.Evidence{
		Sessions: []proposals.Session{demoSession},
		Title:    "merge went wrong twice in one week",
	}
	return []proposals.Evidence{item}
}

func demoFields() []proposals.Field {
	field := proposals.Field{
		Label: "why",
		Text:  "hand-resolved hunks are re-derived work; replaying the delta on a clean base is repeatable",
	}
	return []proposals.Field{field}
}

func groupChecks(checks []setupCheck) []welcomeGroup {
	var groups []welcomeGroup
	for _, check := range checks {
		if len(groups) == 0 || groups[len(groups)-1].Name != check.Group {
			groups = append(groups, welcomeGroup{Name: check.Group})
		}
		groups[len(groups)-1].Checks = append(groups[len(groups)-1].Checks, check)
	}
	return groups
}

func lastRunSummary(resultsPath string) string {
	history, err := routines.History(resultsPath, 1)
	if err != nil || len(history) == 0 {
		return "never ran yet"
	}
	if history[0].ExitStatus == 78 {
		return "last run exited 78 — missing token (see the routine token check)"
	}
	if history[0].ExitStatus != 0 {
		return fmt.Sprintf("last run exited %d", history[0].ExitStatus)
	}
	return "last run ok (" + history[0].Timestamp + ")"
}

func tailLines(output string, n int) string {
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	if len(lines) == 0 {
		return "verification failed with no output"
	}
	return strings.Join(lines, " · ")
}

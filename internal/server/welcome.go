package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/repos"
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
	// probeTimeout lets a down peek degrade a row instead of stalling the page.
	probeTimeout = 2 * time.Second
	// bootstrapSinceDays is how far back the bootstrap date picker starts.
	bootstrapSinceDays = 30
	// verifyDetailLines is how much of a failed token verification's output the card shows.
	verifyDetailLines = 3
)

// welcomeComponentsData is the setup tab: one row per catalog component.
type welcomeComponentsData struct {
	CanVerify bool
	CountDown int
	LoadError string
	Rows      []componentRow
}

type claudeVerifyResult struct {
	IsError   bool   `json:"is_error"`
	Result    string `json:"result"`
	SessionId string `json:"session_id"`
}

type welcomeVerifyData struct {
	Detail      string
	Fix         string
	Ok          bool
	PeekIndexed bool
	SessionId   string
	ShortId     string
}

type welcomePage struct {
	BootstrapSinceDefault string
	BootstrapToday        string
	Change                changeCardView
	Components            welcomeComponentsData
	IsBootstrapShown      bool
	Page                  string
	Tab                   string
	Title                 string
}

func (s *Server) handleWelcome(w http.ResponseWriter, r *http.Request) {
	tab := r.URL.Query().Get("tab")
	if tab != "tutorial" {
		tab = "setup"
	}

	data := welcomePage{Page: pageWelcome, Tab: tab, Title: "Welcome"}
	if tab == "setup" {
		data.Components = s.welcomeComponents(r.Context())
		data.IsBootstrapShown = s.records.has(s.machine, components.Mining)
		data.BootstrapSinceDefault, data.BootstrapToday = bootstrapDates(time.Now())
	} else {
		data.Change = demoChangeView()
	}
	s.renderFragment(w, tmplWelcome, data)
}

func (s *Server) handleWelcomeChecks(w http.ResponseWriter, r *http.Request) {
	s.renderFragment(w, tmplWelcomeChecks, s.welcomeComponents(r.Context()))
}

// handleWelcomeVerifyToken never reads the token in Go: bash sources the file, exactly as run.sh does.
func (s *Server) handleWelcomeVerifyToken(w http.ResponseWriter, r *http.Request) {
	data := welcomeVerifyData{Fix: fixInstallToken}
	tokenPath, detail := s.verifyTokenPath()
	bashPath := verifyBashPath()
	switch {
	case tokenPath == "":
		data.Detail = detail
	case bashPath == "":
		data.Detail = "no bash available to run the verification"
	default:
		script := verifyPathPrefix + `SMINE_HEADLESS=1 CLAUDE_CODE_OAUTH_TOKEN="$(tr -d '[:space:]' < "$1")" claude -p 'reply with exactly: ok' --max-turns 1 --output-format json`
		output, err := shell.Run(r.Context(), "", bashPath, "-c", script, "bash", tokenPath)
		if err != nil {
			data.Detail = tailLines(output, verifyDetailLines)
			break
		}
		data = s.verifiedTokenData(r.Context(), output)
	}
	s.renderFragment(w, tmplWelcomeVerify, data)
}

// verifiedTokenData checks via peek that the run is visible to the mining loop.
func (s *Server) verifiedTokenData(ctx context.Context, output string) welcomeVerifyData {
	data := welcomeVerifyData{
		Detail: "claude answered — token works",
		Fix:    fixInstallToken,
		Ok:     true,
	}
	start := strings.Index(output, "{")
	if start < 0 {
		return data
	}
	var result claudeVerifyResult
	if err := json.Unmarshal([]byte(output[start:]), &result); err != nil {
		return data
	}
	if result.IsError {
		data.Detail = "claude reported an error: " + result.Result
		data.Ok = false
		return data
	}

	data.Detail = fmt.Sprintf("claude answered %q", result.Result)
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

// verifyTokenPath derives the path from server state, never from request input.
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

// welcomeComponents runs health only for selected, available components (master C7).
func (s *Server) welcomeComponents(ctx context.Context) welcomeComponentsData {
	record, err := s.records.load(s.machine)
	data := welcomeComponentsData{
		CanVerify: verifyBashPath() != "",
		Rows:      componentRows(install.Status(ctx, s.machine, record), s.machine.GOOS),
	}
	if err != nil {
		data.LoadError = err.Error()
	}
	for _, row := range data.Rows {
		if row.State == componentDown {
			data.CountDown++
		}
	}
	return data
}

func bootstrapDates(now time.Time) (string, string) {
	sinceDefault := now.AddDate(0, 0, -bootstrapSinceDays).Format(time.DateOnly)
	today := now.Format(time.DateOnly)
	return sinceDefault, today
}

// demoChangeView renders through the live card template so the example never drifts from the real markup.
func demoChangeView() changeCardView {
	demoFile := repos.ChangeFile{
		Added:  1,
		Diff:   "diff --git a/context/actions/implementing.md b/context/actions/implementing.md\n--- a/context/actions/implementing.md\n+++ b/context/actions/implementing.md\n@@ -1 +1,2 @@\n+Reconcile by cache → reset → reapply, never by editing conflict hunks in place.",
		Path:   "context/actions/implementing.md",
		Status: "M",
	}
	commit := repos.ChangeCommit{
		Added:     1,
		Body:      "Two sessions in one week re-broke a merge after resolving conflict hunks in place; replaying the delta on a clean base is repeatable.",
		ChangeIds: []string{"context/reconcile-by-reapply"},
		Files:     []repos.ChangeFile{demoFile},
		Kind:      "context",
		Sessions:  []string{"00000000-0000-0000-0000-000000000000"},
		Sha:       "0000000000000000000000000000000000000000",
		Subject:   "context.actions: Added the reconcile-by-reapply rule (example)",
	}
	evidence := []routines.ChangeEvidence{{Finding: 0, Quote: "the merge broke again after I fixed the hunks by hand", Session: commit.Sessions[0]}}
	task := routines.ChangeTask{
		Id:       commit.ChangeIds[0],
		Evidence: evidence,
		Fix:      "Replaying the delta on a clean base is repeatable; editing hunks in place is not.",
		Problem:  "Two sessions in one week re-broke a merge after resolving conflict hunks in place.",
	}
	refs := map[string]sessions.SessionRef{}
	links := sessionLinks(commit.Sessions, refs)
	branch := repos.NightBranchPrefix + "2026-08-24" + repos.ProposalsSuffix
	view := changeCardView{
		Branch:     branch,
		Commit:     commit,
		Files:      cardFiles(&commit),
		IsDisabled: true,
		Night:      nightLabel(branch),
		SessionAge: sessionAge(links, time.Now()),
		Sessions:   links,
		Problem:    task.Problem,
		ShortSha:   "00000000",
		Tasks:      []changeTaskView{taskView(&task, refs)},
		Verdict:    changeVerdictView{Class: badgeOk, Label: verdictReady},
	}
	return view
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

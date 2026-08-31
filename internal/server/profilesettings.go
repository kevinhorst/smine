package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/server/respond"
	"github.com/kevinhorst/smine/internal/shell"
	"github.com/kevinhorst/smine/internal/skills"
)

const (
	pageProfile     = "profile"
	tmplProfile     = "profile.html"
	tmplProfileTest = "_profile_test.html"
)

// styleTestPrompt is the fixed sample the test button sends: it forces the
// answer through every surface the profile governs — language, register,
// jargon handling (plan D9). It must read as a writing sample, never a task:
// phrased as a task, the model started investigating the repo with tools and
// died on --max-turns 1 (observed error_max_turns).
const styleTestPrompt = "This is a style check, not a task: do not use any tools or read any files — reply directly with prose only. Write me 3-4 sentences conveying exactly this: last night 12 of my work sessions were reviewed and 3 improvement suggestions came out."

type profilePage struct {
	Audience     string
	DevMode      bool
	Language     string
	Page         string
	ProfilePath  string
	StyleContent string
	StyleError   string
	StylePath    string
	Title        string
}

type profileTestView struct {
	Answer string
	Detail string
	Model  string
	Ok     bool
	Token  string
}

// styleTestTarget is the resolved model/token pair a test call runs with;
// TokenName is the label (or "token file"), never the token value.
type styleTestTarget struct {
	Detail    string
	Model     string
	TokenName string
	TokenPath string
}

func (s *Server) handleProfilePage(w http.ResponseWriter, r *http.Request) {
	s.renderFragment(w, tmplProfile, s.profilePageData())
}

func (s *Server) handleProfileSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		respond.WithBadRequest("invalid form", w)
		return
	}

	// audience
	audience := r.PostForm.Get("audience")
	if audience != "" && audience != audienceCasual {
		respond.WithBadRequest(fmt.Sprintf("unknown audience %q", audience), w)
		return
	}

	// language
	language := r.PostForm.Get("language")
	if language != languageEnglish && language != languageGerman {
		respond.WithBadRequest(fmt.Sprintf("unsupported language %q", language), w)
		return
	}

	// dev-mode — casual audience forces it off (self-mining stays locked)
	devMode := r.PostForm.Get("dev_mode") == "on" && audience != audienceCasual

	audienceChanged := (audience == "") != s.presentation.isDeveloperAudience()
	if err := s.presentation.saveProfileSelection(audience, language, devMode); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	// The audience drives skill visibility (skillOverrides) — the sync
	// script owns that flip, so an audience change re-runs it (plan D7).
	if audienceChanged {
		if !s.repoLocks.TryAcquire(skillsSyncLockKey) {
			respond.WithConflict("a skills sync is already running — retry in a moment", w)
			return
		}
		_, err := skills.Sync(r.Context(), false, s.syncScripts)
		s.repoLocks.Release(skillsSyncLockKey)
		if err != nil {
			log.Printf("Server.handleProfileSave: Skills sync after audience change failed: %v", err)
			respond.WithInternalServerError(fmt.Errorf("profile saved, but the skills sync failed: %w", err), w)
			return
		}
	}

	http.Redirect(w, r, "/profile", http.StatusSeeOther)
}

func (s *Server) handleProfileStyleSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		respond.WithBadRequest("invalid form", w)
		return
	}

	content := r.PostForm.Get("style_content")
	if len(content) > maxStyleProfileBytes {
		respond.WithBadRequest("style profile exceeds 64 KiB", w)
		return
	}

	if err := s.presentation.saveStyle(content); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	http.Redirect(w, r, "/profile", http.StatusSeeOther)
}

// handleProfileTest proves the live profile with one 1-turn headless claude
// call — the run's SessionStart hook injects the just-saved profile files, so
// the answer shows the style the machine actually produces (plan D8). Costs
// one paid API call — the button carries hx-confirm.
func (s *Server) handleProfileTest(w http.ResponseWriter, r *http.Request) {
	target := s.styleTestTarget()
	view := profileTestView{Model: target.Model, Token: target.TokenName}
	bashPath := verifyBashPath()
	switch {
	case target.TokenPath == "":
		view.Detail = target.Detail
	case bashPath == "":
		view.Detail = "no bash available to run the test"
	default:
		script := verifyPathPrefix + `CLAUDE_CODE_OAUTH_TOKEN="$(tr -d '[:space:]' < "$1")" claude -p "$2" --model "$3" --max-turns 1 --output-format json`
		output, err := shell.Run(r.Context(), "", bashPath, "-c", script, "bash", target.TokenPath, styleTestPrompt, target.Model)
		if err != nil {
			view.Detail = tailLines(output, 3)
			break
		}
		answer := profileTestAnswer(output)
		view.Answer, view.Detail, view.Ok = answer.Answer, answer.Detail, answer.Ok
	}
	s.renderFragment(w, tmplProfileTest, view)
}

func (s *Server) profilePageData() profilePage {
	styleContent, err := s.presentation.styleContent()
	data := profilePage{
		Audience:     s.presentation.audience(),
		DevMode:      s.presentation.isDevMode(),
		Language:     s.presentation.language(),
		Page:         pageProfile,
		ProfilePath:  s.presentation.path,
		StyleContent: styleContent,
		StylePath:    s.presentation.stylePath,
		Title:        "Profile",
	}
	if err != nil {
		data.StyleError = err.Error()
	}
	return data
}

// styleTestTarget resolves what the test call runs with: the smine-nightly
// routine's ROUTINE_MODEL / ROUTINE_TOKEN when configured, else the routine
// default model and the token-verify pick order. Without the explicit model,
// headless claude falls back to the settings.json model, which the routine
// token's account may have no credits for (observed 429).
func (s *Server) styleTestTarget() styleTestTarget {
	tokenPath, detail := s.verifyTokenPath()
	target := styleTestTarget{
		Detail:    detail,
		Model:     routineModelDefault,
		TokenName: "token file",
		TokenPath: tokenPath,
	}
	list, err := routines.Scan(s.routinesDir)
	if err != nil {
		return target
	}
	index := slices.IndexFunc(list, func(routine routines.Routine) bool {
		return routine.Name == nightlyRoutineName
	})
	if index < 0 {
		return target
	}

	env := list[index].Env
	if env["ROUTINE_MODEL"] != "" {
		target.Model = env["ROUTINE_MODEL"]
	}

	label := env["ROUTINE_TOKEN"]
	if label == "" {
		return target
	}
	labeledPath := filepath.Join(s.tokenDir, label)
	info, err := os.Stat(labeledPath)
	if err != nil || info.Size() == 0 {
		return target
	}

	target.Detail = ""
	target.TokenName = label
	target.TokenPath = labeledPath
	return target
}

// profileTestAnswer interprets the claude JSON envelope: the answer text is
// the test result the user judges.
func profileTestAnswer(output string) profileTestView {
	view := profileTestView{Detail: "claude answered, but the response envelope was unreadable"}
	start := strings.Index(output, "{")
	if start < 0 {
		return view
	}

	var result claudeVerifyResult
	if err := json.Unmarshal([]byte(output[start:]), &result); err != nil {
		return view
	}
	if result.IsError {
		view.Detail = "claude reported an error: " + result.Result
		return view
	}

	view.Answer = result.Result
	view.Detail = ""
	view.Ok = true
	return view
}

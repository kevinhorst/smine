package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/presentation"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/server/respond"
	"github.com/kevinhorst/smine/internal/shell"
)

const (
	noticeSaved             = "Saved"
	pageProfile             = "profile"
	tmplProfile             = "profile.html"
	tmplProfilePresentation = "_profile_presentation.html"
	tmplProfileSettings     = "_profile_settings.html"
	tmplProfileStyle        = "_profile_style.html"
	tmplProfileTest         = "_profile_test.html"
)

// styleTestPrompt must read as a writing sample: phrased as a task, the model used tools and hit --max-turns 1.
const styleTestPrompt = "This is a style check, not a task: do not use any tools or read any files — reply directly with prose only. Write me 3-4 sentences conveying exactly this: last night 12 of my work sessions were reviewed and 3 improvement suggestions came out."

type profilePage struct {
	Audience            string
	AutoMerge           bool
	DevMode             bool
	Installation        installationView
	Language            string
	Notice              string // a save's answer beside the saved form's button, translated at render
	Page                string
	PresentationContent string
	ProfilePath         string
	StyleContent        string
	StyleError          string
	StylePath           string
	Title               string
	TokenDir            string
}

type profileTestView struct {
	Answer string
	Detail string
	Model  string
	Ok     bool
	Token  string
}

// styleTestTarget.TokenName is the label, never the token value.
type styleTestTarget struct {
	Detail    string
	Model     string
	TokenName string
	TokenPath string
}

func (s *Server) handleProfilePage(w http.ResponseWriter, r *http.Request) {
	data := s.profilePageData()
	data.Installation = s.installationView(r.Context())
	s.renderFragment(w, tmplProfile, data)
}

func (s *Server) handleProfileSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		respond.WithBadRequest("invalid form", w)
		return
	}

	// audience — fixed at install; a posted value is ignored
	audience := s.presentation.audience()

	// language
	language := r.PostForm.Get("language")
	if language != presentation.LanguageEnglish && language != languageGerman {
		respond.WithBadRequest(fmt.Sprintf("unsupported language %q", language), w)
		return
	}

	// dev-mode — casual audience forces it off (self-mining stays locked)
	devMode := r.PostForm.Get("dev_mode") == "on" && audience != presentation.AudienceCasual

	// auto-merge — developers may turn the orchestrate merge off; casual always merges
	autoMerge := true
	if audience != presentation.AudienceCasual {
		autoMerge = r.PostForm.Get("auto_merge") == "on"
	}

	previous := s.presentation.language()
	if err := s.presentation.saveProfileSelection(audience, language, devMode, autoMerge); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	// A language change re-renders the whole page: the nav and every label switch, not only this form.
	if language != previous {
		w.Header().Set("HX-Refresh", "true")
	}
	s.renderProfileForm(w, tmplProfileSettings)
}

func (s *Server) handleProfilePresentationSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		respond.WithBadRequest("invalid form", w)
		return
	}

	content := r.PostForm.Get("presentation_content")
	if len(content) > maxStyleProfileBytes {
		respond.WithBadRequest("presentation profile exceeds 64 KiB", w)
		return
	}

	if err := s.presentation.savePresentationBody(content); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	s.renderProfileForm(w, tmplProfilePresentation)
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
	s.renderProfileForm(w, tmplProfileStyle)
}

// renderProfileForm answers a save with the saved form, its notice beside the Save button.
func (s *Server) renderProfileForm(w http.ResponseWriter, name string) {
	data := s.profilePageData()
	data.Notice = noticeSaved
	s.renderFragment(w, name, data)
}

// handleProfileTest costs one paid API call, so the button carries hx-confirm.
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
		script := verifyPathPrefix + `SMINE_HEADLESS=1 CLAUDE_CODE_OAUTH_TOKEN="$(tr -d '[:space:]' < "$1")" claude -p "$2" --model "$3" --max-turns 1 --disallowedTools "*" --output-format json`
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

// handleProfileTokenAdd treats the value as write-only: never rendered or logged.
func (s *Server) handleProfileTokenAdd(w http.ResponseWriter, r *http.Request) {
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

	s.renderFragment(w, tmplOpResult, opResult{Page: pageProfile, Subject: "add token " + label})
}

func (s *Server) profilePageData() profilePage {
	styleContent, err := s.presentation.styleContent()
	data := profilePage{
		Audience:            s.presentation.audience(),
		AutoMerge:           s.presentation.autoMerge(),
		DevMode:             s.presentation.isDevMode(),
		Language:            s.presentation.language(),
		Page:                pageProfile,
		PresentationContent: s.presentation.presentationContent(),
		ProfilePath:         s.presentation.path,
		StyleContent:        styleContent,
		StylePath:           s.presentation.stylePath,
		Title:               "Profile",
		TokenDir:            s.tokenDir,
	}
	if err != nil {
		data.StyleError = err.Error()
	}
	return data
}

// styleTestTarget always names a model: the settings.json fallback may lack credits on the routine token's account.
func (s *Server) styleTestTarget() styleTestTarget {
	tokenPath, detail := s.verifyTokenPath()
	target := styleTestTarget{
		Detail:    detail,
		Model:     smineNightlyModel,
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

// defaultTokenLabel yields token-2026-08-12, then token-2026-08-12-2 on a same-day collision.
func defaultTokenLabel(labels []string) string {
	base := time.Now().Format("token-2006-01-02")
	label := base
	for suffix := 2; slices.Contains(labels, label); suffix++ {
		label = fmt.Sprintf("%s-%d", base, suffix)
	}
	return label
}

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

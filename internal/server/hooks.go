package server

import (
	"encoding/json"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/server/respond"
)

const tmplHooks = "_hooks.html"

type hookView struct {
	Command    string
	Content    string
	Name       string
	ScriptPath string
}

type hookGroupInfo struct {
	Command string
	Enabled bool
	Event   string
	Index   int
	Matcher string
	Name    string
	Views   []hookView
}

type hooksData struct {
	Diff       []diffLine
	Hooks      []hookGroupInfo
	Overridden bool
}

func (s *Server) handleGetHooks(w http.ResponseWriter, _ *http.Request) {
	main, err := config.Load(s.settingsPath)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	s.renderHooks(w, main)
}

func (s *Server) handleToggleHook(w http.ResponseWriter, r *http.Request) {
	event := r.PathValue("event")
	index, err := strconv.Atoi(r.PathValue("index"))
	if err != nil || index < 0 {
		respond.WithBadRequest("invalid index", w)
		return
	}

	// enabled: required — it names the side the index addresses, i.e. the
	// row's current state; the toggle moves the group to the other side (D33)
	enabled, err := strconv.ParseBool(r.URL.Query().Get("enabled"))
	if err != nil {
		respond.WithBadRequest("invalid enabled state", w)
		return
	}

	main, err := config.Load(s.settingsPath)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	mainHooks, err := main.Hooks()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	if mainHooks == nil {
		mainHooks = make(map[string][]config.HookGroup)
	}

	if !enabled {
		// disabled row → re-enable: pop from the sidecar, write back to settings.json
		group, ok, err := s.disabledHooks.Pop(event, index)
		if err != nil {
			respond.WithInternalServerError(err, w)
			return
		}
		if !ok {
			respond.WithNotFound("hook not found", w)
			return
		}
		mainHooks[event] = append(mainHooks[event], group)
		s.saveHooks(main, mainHooks, w)
		return
	}

	// enabled row → disable: remove from settings.json, park in the sidecar
	group, ok := takeHookGroup(mainHooks, event, index)
	if !ok {
		respond.WithNotFound("hook not found", w)
		return
	}
	if err := s.disabledHooks.Add(event, group); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	s.saveHooks(main, mainHooks, w)
}

func (s *Server) handleDeleteHook(w http.ResponseWriter, r *http.Request) {
	event := r.PathValue("event")
	index, err := strconv.Atoi(r.PathValue("index"))
	if err != nil || index < 0 {
		respond.WithBadRequest("invalid index", w)
		return
	}

	main, err := config.Load(s.settingsPath)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	// disabled row → drop from the sidecar
	if r.URL.Query().Get("enabled") == "false" {
		_, ok, err := s.disabledHooks.Pop(event, index)
		if err != nil {
			respond.WithInternalServerError(err, w)
			return
		}
		if !ok {
			respond.WithNotFound("hook not found", w)
			return
		}
		s.renderHooks(w, main)
		return
	}

	// enabled row → delete the entry from settings.json (the docs' removal
	// semantics, F26)
	mainHooks, err := main.Hooks()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	if _, ok := takeHookGroup(mainHooks, event, index); !ok {
		respond.WithNotFound("hook not found", w)
		return
	}
	s.saveHooks(main, mainHooks, w)
}

// saveHooks persists settings.json and re-renders the hooks fragment.
func (s *Server) saveHooks(main *config.Settings, mainHooks map[string][]config.HookGroup, w http.ResponseWriter) {
	if err := main.SetHooks(mainHooks); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	if err := config.Save(s.settingsPath, main); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	// Every hook mutation re-pulls #config-body (D3).
	w.Header().Set("HX-Trigger", "config-op")
	s.renderHooks(w, main)
}

func (s *Server) renderHooks(w http.ResponseWriter, main *config.Settings) {
	mainHooks, err := main.Hooks()
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	data := hooksData{
		Hooks:      hookRows(mainHooks, s.disabledHooks.Snapshot()),
		Overridden: s.sectionOverridden(main, "hooks"),
	}
	if data.Overridden {
		data.Diff = s.hooksFragmentDiff(main)
	}
	s.renderFragment(w, tmplHooks, data)
}

// hooksFragmentDiff diffs the repo fragment's hooks key against the live
// settings' — canonical pretty-printed JSON, so key order and toggle-induced
// array order are not noise. A fragment load error renders no diff, matching
// sectionOverridden's degrade-to-quiet.
func (s *Server) hooksFragmentDiff(main *config.Settings) []diffLine {
	fragment, err := config.LoadFragment(s.claudeFragmentPath)
	if err != nil {
		return nil
	}
	return compactDiff(diffLines(canonicalPretty(fragment.Doc(), "hooks"), canonicalPretty(main.Doc(), "hooks")), 2)
}

// canonicalPretty pretty-prints a top-level key after canonicalJSON
// normalization; a missing key renders empty, an unparseable one verbatim.
func canonicalPretty(doc *config.Document, key string) string {
	raw, ok := doc.Get([]string{key})
	if !ok {
		return ""
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return string(raw)
	}
	pretty, err := json.MarshalIndent(canonicalJSON(value), "", "  ")
	if err != nil {
		return string(raw)
	}
	return string(pretty)
}

func eventHookRows(enabled bool, event string, groups []config.HookGroup) []hookGroupInfo {
	rows := make([]hookGroupInfo, 0, len(groups))
	for index, hookGroup := range groups {
		matcher := ""
		if hookGroup.Matcher != nil {
			matcher = *hookGroup.Matcher
		}
		row := hookGroupInfo{
			Command: summarizeHookGroup(&hookGroup),
			Enabled: enabled,
			Event:   event,
			Index:   index,
			Matcher: matcher,
			Name:    summarizeHookNames(&hookGroup),
			Views:   hookViews(&hookGroup),
		}
		rows = append(rows, row)
	}
	return rows
}

func hookRows(mainHooks, disabledHooks map[string][]config.HookGroup) []hookGroupInfo {
	rows := sideHookRows(true, mainHooks)
	rows = append(rows, sideHookRows(false, disabledHooks)...)
	slices.SortStableFunc(rows, func(left, right hookGroupInfo) int {
		if c := strings.Compare(left.Event, right.Event); c != 0 {
			return c
		}
		return strings.Compare(left.Name, right.Name)
	})
	return rows
}

// takeHookGroup removes and returns the group at event/index — the removal
// half of the deleted moveHookGroup.
func takeHookGroup(hooks map[string][]config.HookGroup, event string, index int) (config.HookGroup, bool) {
	groups, ok := hooks[event]
	if !ok || index >= len(groups) {
		return config.HookGroup{}, false
	}

	group := groups[index]
	hooks[event] = slices.Delete(groups, index, index+1)
	if len(hooks[event]) == 0 {
		delete(hooks, event)
	}
	return group, true
}

func sideHookRows(enabled bool, hooks map[string][]config.HookGroup) []hookGroupInfo {
	var rows []hookGroupInfo
	for _, event := range slices.Sorted(maps.Keys(hooks)) {
		rows = append(rows, eventHookRows(enabled, event, hooks[event])...)
	}
	return rows
}

func summarizeHookGroup(hookGroup *config.HookGroup) string {
	if len(hookGroup.Hooks) == 0 {
		return "(empty)"
	}

	commands := make([]string, 0, len(hookGroup.Hooks))
	for _, hook := range hookGroup.Hooks {
		commands = append(commands, hook.Command)
	}
	return strings.Join(commands, " && ")
}

func hookViews(hookGroup *config.HookGroup) []hookView {
	views := make([]hookView, 0, len(hookGroup.Hooks))
	for _, hook := range hookGroup.Hooks {
		view := hookView{
			Command: hook.Command,
			Name:    hookDisplayName(&hook),
		}
		view.ScriptPath, view.Content = resolveScript(hook.Command)
		views = append(views, view)
	}
	return views
}

func hookDisplayName(hook *config.Hook) string {
	if hook.Name != "" {
		return hook.Name
	}
	return hook.Command
}

// summarizeHookNames mirrors summarizeHookGroup but prefers each hook's name.
func summarizeHookNames(hookGroup *config.HookGroup) string {
	if len(hookGroup.Hooks) == 0 {
		return "(empty)"
	}

	names := make([]string, 0, len(hookGroup.Hooks))
	for _, hook := range hookGroup.Hooks {
		names = append(names, hookDisplayName(&hook))
	}
	return strings.Join(names, " && ")
}

// resolveScript maps a hook command's first token to a readable file,
// expanding a leading ~. Inline commands resolve to nothing.
func resolveScript(command string) (path, content string) {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return "", ""
	}

	token := fields[0]
	if strings.HasPrefix(token, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", ""
		}

		token = filepath.Join(home, token[2:])
	}
	if !filepath.IsAbs(token) {
		return "", ""
	}

	info, err := os.Stat(token)
	if err != nil || info.IsDir() {
		return "", ""
	}

	data, err := os.ReadFile(token)
	if err != nil {
		return "", ""
	}

	return token, string(data)
}

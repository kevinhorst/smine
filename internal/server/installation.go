package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/server/respond"
	"github.com/kevinhorst/smine/internal/skills"
)

// Component states as Profile and Welcome render them (concept D9).
const (
	componentDown        = "down"
	componentOff         = "off"
	componentOn          = "on"
	componentUnavailable = "unavailable"
)

const (
	readOnlyInstallation    = "the development instance shows the installation read-only"
	tmplInstallationPreview = "_installation_preview.html"
	tmplRecordHistory       = "_record_history.html"
)

// componentRow is one catalog component as Profile and Welcome show it.
type componentRow struct {
	Id         string
	Brings     []string
	Detail     string
	Fix        string
	RequireIds []string // the ids behind Requires, which the form's tick cascade reads
	Requires   []string
	State      string
	Summary    string
	Title      string
	Without    []string
}

// installationPreview is the Apply confirmation: the steps plus the record as hidden inputs, so Apply runs what was shown.
type installationPreview struct {
	ChosenSkills      []string
	Components        []string
	Error             string
	IsSkillsConfirmed bool
	Notes             []string
	SkillsMode        string
	Steps             []install.Step
}

// installationView lists every repo skill as a picker option, the record's chosen ones selected.
type installationView struct {
	History           []recordHistoryRow
	HistoryError      string
	IsApplyPending    bool
	IsReadOnly        bool
	IsSkillsConfirmed bool
	LoadError         string
	Notes             []string
	Rows              []componentRow
	Skills            []skillPickOption
	SkillsError       string
	SkillsMode        string
	Suspended         []string
}

// recordHistoryRow is one previous install.json beside the record, newest first (master A4).
type recordHistoryRow struct {
	Size  int64
	Stamp string
	Time  time.Time
}

// recordHistoryView is the diff of one previous record against the current file.
type recordHistoryView struct {
	Diff  []diffLine
	Error string
	Stamp string
}

// skillPickOption is one picker row: the leaf and the component ids it needs, so the page can disable it while they are unticked.
type skillPickOption struct {
	IsSelected bool
	Name       string
	Requires   []string
}

// recordCache holds install.json by modification time: the nav asks on every page and the terminal installer may rewrite the file while the server runs.
type recordCache struct {
	modTime time.Time
	mu      sync.Mutex
	record  install.Record
}

// has reports whether the stored record selects id; an unreadable record counts as Full for page gating only, so a broken file never hides a page.
func (c *recordCache) has(env install.Env, id components.ID) bool {
	record, err := c.load(env)
	if err != nil {
		record = install.FullRecord(env.GOOS)
	}
	return slices.Contains(record.Components, id)
}

// load re-reads the record only when install.json's modification time moved; a missing file is Full (concept D18).
func (c *recordCache) load(env install.Env) (install.Record, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	info, statErr := os.Stat(install.RecordPath(env))
	if statErr == nil && info.ModTime().Equal(c.modTime) {
		return c.record, nil
	}

	record, _, err := install.Load(env)
	if err != nil {
		return install.Record{}, fmt.Errorf("recordCache.load: %w", err)
	}

	c.modTime = time.Time{}
	if statErr == nil {
		c.modTime = info.ModTime()
	}
	c.record = record
	return record, nil
}

// applyInstallation runs install.Apply under the sync lock so Profile, Skills and Sync never write the record at once, then restarts detached when the LaunchAgent changed.
func (s *Server) applyInstallation(record install.Record, w http.ResponseWriter, r *http.Request) {
	isRestartDue := false
	applyRecord := func(ctx context.Context) (string, error) {
		var report strings.Builder
		env := s.machine
		env.Stdout = &report
		results, err := install.Apply(ctx, env, record)
		report.WriteString(install.Summary(results))
		s.titles.reload()

		isRestartDue = slices.ContainsFunc(results, isRestartResult)
		if isRestartDue {
			report.WriteString("the config server restarts in a moment — reload the page\n")
		}
		if err == nil && install.Failed(results) {
			err = errors.New("applyInstallation: A component failed — see the report")
		}
		return report.String(), err
	}
	s.runRepoOp(syncLockKey, "repo-op", applyRecord, w, r)
	if !isRestartDue {
		return
	}

	if err := restartServerAgent(s.repoDir); err != nil {
		log.Printf("Server.applyInstallation: %v", err)
	}
}

func (s *Server) handleInstallationApply(w http.ResponseWriter, r *http.Request) {
	if s.isDevInstance {
		respond.WithBadRequest(readOnlyInstallation, w)
		return
	}
	if err := r.ParseForm(); err != nil {
		respond.WithBadRequest("invalid form", w)
		return
	}

	stored, err := s.records.load(s.machine)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	record, _ := s.wantedRecord(r.PostForm, stored)
	s.applyInstallation(record, w, r)
}

func (s *Server) handleInstallationPreview(w http.ResponseWriter, r *http.Request) {
	if s.isDevInstance {
		respond.WithBadRequest(readOnlyInstallation, w)
		return
	}
	if err := r.ParseForm(); err != nil {
		respond.WithBadRequest("invalid form", w)
		return
	}

	stored, err := s.records.load(s.machine)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	record, notes := s.wantedRecord(r.PostForm, stored)
	preview := installationPreview{
		ChosenSkills:      record.Skills.Chosen,
		IsSkillsConfirmed: record.Skills.IsConfirmed,
		Notes:             notes,
		SkillsMode:        record.Skills.Mode,
	}
	for _, id := range record.Components {
		preview.Components = append(preview.Components, string(id))
	}
	if err := install.Validate(record, s.machine.GOOS); err != nil {
		preview.Error = err.Error()
		s.renderFragment(w, tmplInstallationPreview, preview)
		return
	}

	preview.Steps, err = install.Plan(r.Context(), s.machine, record)
	if err != nil {
		preview.Error = err.Error()
	}
	s.renderFragment(w, tmplInstallationPreview, preview)
}

func (s *Server) installationView(ctx context.Context) installationView {
	record, err := s.records.load(s.machine)
	view := installationView{
		IsReadOnly:        s.isDevInstance,
		IsSkillsConfirmed: record.Skills.IsConfirmed,
		Rows:              componentRows(install.Status(ctx, s.machine, record), s.machine.GOOS),
		SkillsMode:        record.Skills.Mode,
	}
	switch {
	case err != nil:
		view.LoadError = err.Error()
	default:
		view.IsApplyPending = !record.Applied
		view.Notes = install.Notes(record)
		view.Suspended = suspendedNames(record)
	}

	view.Skills, err = skillPickOptions(s.skillsRepo, record.Skills.Chosen)
	if err != nil {
		view.SkillsError = err.Error()
	}

	view.History, err = recordHistoryRows(install.RecordPath(s.machine))
	if err != nil {
		view.HistoryError = err.Error()
	}
	return view
}

// handleRecordHistory renders one previous install.json, named by its stamp, as a diff against the current file; a stamp matches a listed history file by string equality.
func (s *Server) handleRecordHistory(w http.ResponseWriter, r *http.Request) {
	path := install.RecordPath(s.machine)
	history, err := install.History(path)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	stamp := r.PathValue("stamp")
	if !slices.Contains(history, path+"."+stamp) {
		respond.WithNotFound("no record history "+stamp, w)
		return
	}

	previous, err := os.ReadFile(path + "." + stamp)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	view := recordHistoryView{Stamp: stamp}
	view.Diff, err = fileDiff(previous, path)
	if err != nil {
		view.Error = err.Error()
	}
	s.renderFragment(w, tmplRecordHistory, view)
}

// wantedRecord is the form's record with chosen skills the component set cannot deploy dropped, each a note; an unreadable skill root drops nothing.
func (s *Server) wantedRecord(form url.Values, stored install.Record) (install.Record, []string) {
	record, notes := formRecord(form, stored, s.machine.GOOS)
	list, err := skills.Scan(s.skillsRepo, "")
	if err != nil {
		log.Printf("Server.wantedRecord: %v", err)
		return record, notes
	}
	return record, append(notes, dropUnavailableSkills(&record, list)...)
}

func componentRows(statuses []install.ComponentStatus, goos string) []componentRow {
	rows := make([]componentRow, 0, len(statuses))
	for _, status := range statuses {
		rows = append(rows, newComponentRow(status, goos))
	}
	return rows
}

// formRecord builds the wanted record from the form over the stored one: unticking suspends dependents, ticking brings requirements and restores suspended dependents; every other field is the stored record's (master A1, A7).
func formRecord(form url.Values, stored install.Record, goos string) (install.Record, []string) {
	submitted := []components.ID{components.ConfigServer}
	for _, value := range form["component"] {
		submitted = append(submitted, components.ID(value))
	}

	record := stored
	record.Suspended.Components = slices.Clone(stored.Suspended.Components)
	record.Skills.Suspended = slices.Clone(stored.Skills.Suspended)
	var notes []string
	for _, id := range stored.Components {
		if slices.Contains(submitted, id) {
			continue
		}
		leaving := components.Dependents(id, submitted)
		if len(leaving) == 0 {
			continue
		}

		notes = append(notes, fmt.Sprintf("turning %s off also suspends %s", id, joinComponentIds(leaving)))
		for _, dependent := range leaving {
			if !slices.Contains(record.Suspended.Components, dependent) {
				record.Suspended.Components = append(record.Suspended.Components, dependent)
			}
		}
		submitted = slices.DeleteFunc(submitted, func(other components.ID) bool {
			return slices.Contains(leaving, other)
		})
	}
	submitted, record.Suspended.Components = restoreSuspended(submitted, record.Suspended.Components)

	// Close drops ids outside the catalog; keep them so Validate refuses the form instead of the typo vanishing silently.
	selected := components.Close(submitted)
	for _, id := range submitted {
		if _, ok := components.Lookup(id); !ok {
			selected = append(selected, id)
		}
	}
	record.Components = selected
	record.Preset = presetOf(selected, goos)
	selection := formSelection(form)
	record.Skills.Chosen, record.Skills.IsConfirmed, record.Skills.Mode = selection.Chosen, selection.IsConfirmed, selection.Mode
	return record, notes
}

// formSelection reads the skill mode, the confirmation tick and, for mode chosen only, the picked leaves.
func formSelection(form url.Values) install.Selection {
	selection := install.Selection{IsConfirmed: form.Get("skills_confirmed") != "", Mode: form.Get("skills_mode")}
	if selection.Mode != skills.SelectionModeChosen {
		return selection
	}

	for _, leaf := range form["skill"] {
		if trimmed := strings.TrimSpace(leaf); trimmed != "" {
			selection.Chosen = append(selection.Chosen, trimmed)
		}
	}
	return selection
}

func isRestartResult(result install.Result) bool {
	return result.Action == install.ActionRestartRequired
}

func joinComponentIds(ids []components.ID) string {
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		names = append(names, string(id))
	}
	return strings.Join(names, ", ")
}

// newComponentRow maps one status: unavailable before off before the health verdict — off is a choice, down a fault (concept D9).
func newComponentRow(status install.ComponentStatus, goos string) componentRow {
	row := componentRow{
		Id:      string(status.Component.ID),
		Brings:  status.Component.Brings,
		Summary: status.Component.Summary,
		Title:   status.Component.Title,
		Without: status.Component.Without,
	}
	for _, required := range status.Component.Requires {
		if component, ok := components.Lookup(required); ok {
			row.RequireIds = append(row.RequireIds, string(required))
			row.Requires = append(row.Requires, component.Title)
		}
	}

	switch {
	case !status.Available:
		row.State = componentUnavailable
		row.Detail = "not available on " + goos
	case !status.Selected:
		row.State = componentOff
	case status.Health.Ok:
		row.State = componentOn
		row.Detail = status.Health.Detail
	default:
		row.State = componentDown
		row.Detail = status.Health.Detail
		row.Fix = status.Health.Fix
	}
	return row
}

// skillPickOptions lists the repo's skills in scan order as picker options, the chosen ones selected.
func skillPickOptions(skillsRepo string, chosen []string) ([]skillPickOption, error) {
	list, err := skills.Scan(skillsRepo, "")
	if err != nil {
		return nil, fmt.Errorf("skillPickOptions: %w", err)
	}

	options := make([]skillPickOption, 0, len(list))
	for _, skill := range list {
		option := skillPickOption{IsSelected: slices.Contains(chosen, skill.Name), Name: skill.Name}
		for _, required := range skill.Requires {
			option.Requires = append(option.Requires, string(required))
		}
		options = append(options, option)
	}
	return options, nil
}

// dropUnavailableSkills suspends chosen leaves whose required components the record lacks and restores suspended ones whose requirements returned, one note each (master A7).
func dropUnavailableSkills(record *install.Record, list []skills.Skill) []string {
	isAllMet := func(leaf string) (bool, bool) {
		index := slices.IndexFunc(list, func(skill skills.Skill) bool { return skill.Name == leaf })
		if index < 0 {
			return true, false
		}
		isPresent := func(required components.ID) bool { return slices.Contains(record.Components, required) }
		missing := slices.DeleteFunc(slices.Clone(list[index].Requires), isPresent)
		return len(missing) == 0, true
	}

	var notes []string
	// A leaf comes back only into a chosen selection: mode all deploys it anyway, mode none lists no leaf.
	isChosenMode := record.Skills.Mode == skills.SelectionModeChosen
	suspended := slices.Clone(record.Skills.Suspended)
	for _, leaf := range suspended {
		isMet, isKnown := isAllMet(leaf)
		isRestorable := isKnown && isMet
		if !isRestorable || !isChosenMode {
			continue
		}
		if !slices.Contains(record.Skills.Chosen, leaf) {
			record.Skills.Chosen = append(record.Skills.Chosen, leaf)
		}
		record.Skills.Suspended = slices.DeleteFunc(record.Skills.Suspended, func(other string) bool { return other == leaf })
		notes = append(notes, fmt.Sprintf("skill %s restored — its components are back", leaf))
	}

	record.Skills.Chosen = slices.DeleteFunc(record.Skills.Chosen, func(leaf string) bool {
		isMet, isKnown := isAllMet(leaf)
		if isMet || !isKnown {
			return false
		}
		if !slices.Contains(record.Skills.Suspended, leaf) {
			record.Skills.Suspended = append(record.Skills.Suspended, leaf)
		}
		notes = append(notes, fmt.Sprintf("skill %s needs a component that is off — suspended until it returns", leaf))
		return true
	})
	return notes
}

// presetOf names the preset a closed component set equals, else custom.
func presetOf(selected []components.ID, goos string) string {
	switch {
	case slices.Equal(selected, components.Full(goos)):
		return install.PresetFull
	case slices.Equal(selected, components.Minimal()):
		return install.PresetMinimal
	default:
		return install.PresetCustom
	}
}

// recordHistoryRows lists the history files beside the record, newest first, with the time each was kept.
func recordHistoryRows(path string) ([]recordHistoryRow, error) {
	history, err := install.History(path)
	if err != nil {
		return nil, fmt.Errorf("recordHistoryRows: %w", err)
	}

	rows := make([]recordHistoryRow, 0, len(history))
	for _, file := range history {
		info, err := os.Stat(file)
		if err != nil {
			return nil, fmt.Errorf("recordHistoryRows: %w", err)
		}
		row := recordHistoryRow{Size: info.Size(), Stamp: strings.TrimPrefix(file, path+"."), Time: info.ModTime()}
		rows = append(rows, row)
	}
	return rows, nil
}

// restoreSuspended brings back, until none is left, every suspended component whose requirements the submitted set holds — a restored one can meet another's — and returns the rest; one ticked again leaves the list.
func restoreSuspended(submitted, suspended []components.ID) ([]components.ID, []components.ID) {
	isMissing := func(required components.ID) bool { return !slices.Contains(submitted, required) }
	isBack := func(id components.ID) bool {
		component, ok := components.Lookup(id)
		isAllMet := ok && !slices.ContainsFunc(component.Requires, isMissing)
		return isAllMet || slices.Contains(submitted, id)
	}

	remaining := slices.Clone(suspended)
	for {
		index := slices.IndexFunc(remaining, isBack)
		if len(remaining) == 0 {
			return submitted, nil
		}
		if index < 0 {
			return submitted, remaining
		}
		if !slices.Contains(submitted, remaining[index]) {
			submitted = append(submitted, remaining[index])
		}
		remaining = slices.Delete(remaining, index, index+1)
	}
}

// suspendedNames lists the record's suspended components, then its suspended routines and skills, for the Installation card.
func suspendedNames(record install.Record) []string {
	var names []string
	for _, id := range record.Suspended.Components {
		names = append(names, string(id))
	}
	for _, label := range record.Suspended.Routines {
		names = append(names, "routine "+label)
	}
	for _, leaf := range record.Skills.Suspended {
		names = append(names, "skill "+leaf)
	}
	return names
}

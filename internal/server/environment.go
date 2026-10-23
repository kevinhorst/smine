package server

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"os/exec"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/server/respond"
	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skills"
	"github.com/kevinhorst/smine/internal/trash"
)

const (
	skillsTabEnvironment = "environment"
	tmplEnvironment      = "environment.html"
	tmplGlobalDefaults   = "_global_defaults.html"
)

// Provider states: the handshake answered every bound version, lacks one, or the program is not there.
const (
	stateCurrent = "current"
	stateMissing = "missing"
	stateStale   = "stale"
)

// declaredVariable is one repo-scoped variable of a selected skill with its record.
type declaredVariable struct {
	Interface *skillgraph.Interface
	Required  bool
	Skill     string
}

// environmentPage is the Skills Environment tab: the state of every tool the selected skills run through, then one skill's Environment widget behind a select.
type environmentPage struct {
	Defaults  *globalDefaultsCard
	Env       skillEnvView
	Page      string
	Providers []providerState
	Skill     string
	Skills    []string
	Tab       string
	Title     string
}

// globalDefaultRow is one repo-scoped kind on the Environment tab: its record, the global default and the selected skills reading it.
type globalDefaultRow struct {
	Interface *skillgraph.Interface
	Skills    []string
	Value     string
}

// globalDefaultsCard is the Environment tab's card of global defaults; a save answers with the card and its notice.
type globalDefaultsCard struct {
	Notice string
	Rows   []globalDefaultRow
}

// providerBinding is one provider the skills bind install-scoped variables through: the refs bound.
type providerBinding struct {
	provider *skillgraph.Provider
	refs     []string
}

// providerState is one provider's health: current, stale with the missing versions, or missing with the component to install.
type providerState struct {
	Id string

	Fix      string
	Override string // the record PROVIDER_ row, the advanced input's content
	Program  string
	State    string // stateCurrent | stateStale | stateMissing
}

// handleEnvironment renders the Environment tab: one state per provider the selected skills bind.
func (s *Server) handleEnvironment(w http.ResponseWriter, r *http.Request) {
	record, err := s.records.load(s.machine)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	models, err := s.selectedModels(record)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	rows := install.SkillRows(record)
	data := environmentPage{Page: pageSkills, Tab: skillsTabEnvironment, Title: skillsTitle}
	data.Defaults = newGlobalDefaultsCard(models, rows)
	data.Providers = s.providerStates(r.Context(), rows, models, skills.NewProber(), record)
	data.Skills = declaringSkills(models)
	if len(data.Skills) == 0 {
		s.renderFragment(w, tmplEnvironment, data)
		return
	}

	data.Skill = data.Skills[0]
	if requested := r.URL.Query().Get("skill"); slices.Contains(data.Skills, requested) {
		data.Skill = requested
	}
	skill, err := s.repoSkillNamed(data.Skill)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}
	data.Env = s.skillEnv(r.Context(), r.URL.Query().Get("repo"), skill)
	s.renderFragment(w, tmplEnvironment, data)
}

// handleEnvironmentWidget renders the Environment widget of the repo skill ?skill= names, on ?repo= — the tab's select swaps it in place.
func (s *Server) handleEnvironmentWidget(w http.ResponseWriter, r *http.Request) {
	skill, err := s.repoSkillNamed(r.URL.Query().Get("skill"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.renderFragment(w, tmplSkillEnv, s.skillEnv(r.Context(), r.URL.Query().Get("repo"), skill))
}

// handleEnvironmentForget removes one SKILL_ or PROVIDER_ row of the record on the user's click, journaling its key and value, and answers with the ?skill= widget (master A6).
func (s *Server) handleEnvironmentForget(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}

	skill, err := s.repoSkillNamed(r.PostForm.Get("skill"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if !s.repoLocks.TryAcquire(syncLockKey) {
		respond.WithConflict("an operation is already running", w)
		return
	}
	defer s.repoLocks.Release(syncLockKey)

	record, err := s.records.load(s.machine)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	key := r.PostForm.Get("key")
	rows := install.SkillRows(record)
	value, isVariable := rows.Skills[key]
	providerValue, isProvider := rows.Providers[key]
	switch {
	case isVariable:
		delete(rows.Skills, key)
	case isProvider:
		value = providerValue
		delete(rows.Providers, key)
	default:
		respond.WithNotFound("no record row "+key, w)
		return
	}

	entry := &trash.Entry{Action: "environment forget", Kind: trash.KindForgotten, Path: "install.json#" + key, Reason: value}
	if err := s.trash.Note(entry); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	install.SetSkillRows(&record, rows)
	if err := install.Save(s.machine, record); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	view := s.skillEnv(r.Context(), r.PostForm.Get("repo"), skill)
	view.Notice = "forgot " + key
	s.renderFragment(w, tmplSkillEnv, view)
}

// handleEnvironmentSave writes the posted PROVIDER_ overrides of the selected skills' providers into the record, an empty field clearing the row, then syncs the skills so every deployed copy runs the program in effect; the form is checked first, the record re-read and saved under the lock.
func (s *Server) handleEnvironmentSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}

	checked, err := s.records.load(s.machine)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	models, err := s.selectedModels(checked)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	if err := applyProviderForm(install.SkillRows(checked), r.PostForm, models); err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}

	save := func(ctx context.Context) (string, error) {
		record, err := s.records.load(s.machine)
		if err != nil {
			return "", fmt.Errorf("Server.handleEnvironmentSave: %w", err)
		}

		rows := install.SkillRows(record)
		if err := applyProviderForm(rows, r.PostForm, models); err != nil {
			return "", fmt.Errorf("Server.handleEnvironmentSave: %w", err)
		}

		install.SetSkillRows(&record, rows)
		if err := install.Save(s.machine, record); err != nil {
			return "", fmt.Errorf("Server.handleEnvironmentSave: %w", err)
		}

		output, err := skills.Sync(s.skillOptions(record))
		s.titles.reload()
		return output, err
	}
	s.runRepoOp(syncLockKey, "repo-op", save, w, r)
}

// handleGlobalDefaultsSave writes the posted global default of every kind the card lists into the record, an empty field clearing it; nothing deploys, the next invocation reads it.
func (s *Server) handleGlobalDefaultsSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}

	if !s.repoLocks.TryAcquire(syncLockKey) {
		respond.WithConflict("an operation is already running", w)
		return
	}
	defer s.repoLocks.Release(syncLockKey)

	record, err := s.records.load(s.machine)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	models, err := s.selectedModels(record)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	rows := install.SkillRows(record)
	listed := newGlobalDefaultsCard(models, rows)
	if err := applyGlobalDefaults(listed, r.PostForm, rows); err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}

	install.SetSkillRows(&record, rows)
	if err := install.Save(s.machine, record); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	saved := newGlobalDefaultsCard(models, rows)
	saved.Notice = "saved — the next invocation reads it"
	s.renderFragment(w, tmplGlobalDefaults, saved)
}

// handleSkillRepoEnv writes the posted repo-scoped values of one skill into the registry row's env, keeping every other skill's keys; an empty field clears the key.
func (s *Server) handleSkillRepoEnv(w http.ResponseWriter, r *http.Request) {
	skill := s.findSkill(w, r)
	if skill == nil {
		return
	}
	if skill.Origin != skills.OriginRepo {
		http.NotFound(w, r)
		return
	}

	repo, ok := s.repoRegistry.Find(r.PathValue("repo"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}

	model, err := skillgraph.Load(skill.Path)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	models := []*skillgraph.Skill{model}
	env, err := environmentForm(models, r.PostForm, repo.Path)
	if err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}

	updated := *repo
	updated.Env = map[string]string{}
	maps.Copy(updated.Env, repo.Env)
	for name := range repoScopedVariables(models) {
		delete(updated.Env, name)
	}
	maps.Copy(updated.Env, env)
	if len(updated.Env) == 0 {
		updated.Env = nil
	}

	if !s.repoLocks.TryAcquire(registryLockKey) {
		respond.WithConflict("an operation is already running", w)
		return
	}
	defer s.repoLocks.Release(registryLockKey)

	if err := s.repoRegistry.Update(updated); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	view := s.skillEnv(r.Context(), repo.Name, skill)
	view.Notice = "saved for " + repo.Name + " — its next session reads the new values"
	s.renderFragment(w, tmplSkillEnv, view)
}

// repoSkillNamed is the repo skill called name.
func (s *Server) repoSkillNamed(name string) (*skills.Skill, error) {
	list, err := skills.Scan(s.skillsRepo, s.skillsHome)
	if err != nil {
		return nil, fmt.Errorf("Server.repoSkillNamed: %w", err)
	}

	skill, ok := skills.Find(list, skills.OriginRepo, name)
	if !ok {
		return nil, fmt.Errorf("Server.repoSkillNamed: No repo skill %q", name)
	}
	return skill, nil
}

// providerStates runs providerStateOf once per provider the models bind an install-scoped variable through, sorted by id.
func (s *Server) providerStates(ctx context.Context, rows *skills.Rows, models []*skillgraph.Skill, prober skills.Prober, record install.Record) []providerState {
	set := install.ExpandedComponents(record, s.machine.GOOS)
	bindings := providerBindings(models)
	ids := slices.Sorted(maps.Keys(bindings))
	states := make([]providerState, 0, len(ids))
	for _, id := range ids {
		binding := bindings[id]
		isInstalled := slices.Contains(set, binding.provider.Component)
		state := providerStateOf(ctx, binding.refs, isInstalled, prober, binding.provider)
		state.Override = rows.Providers[skills.ProviderKey(id)]
		states = append(states, state)
	}
	return states
}

// selectedModels loads the model of every selected repo skill whose components are installed.
func (s *Server) selectedModels(record install.Record) ([]*skillgraph.Skill, error) {
	list, err := skills.Scan(s.skillsRepo, s.skillsHome)
	if err != nil {
		return nil, fmt.Errorf("Server.selectedModels: %w", err)
	}

	var models []*skillgraph.Skill
	for _, skill := range list {
		isAvailable := skill.Origin == skills.OriginRepo && missingComponents(skill.Requires, record.Components) == ""
		if !isAvailable || !isSelected(record.Skills, skill.Name) {
			continue
		}

		model, err := skillgraph.Load(skill.Path)
		if err != nil {
			return nil, fmt.Errorf("Server.selectedModels: Skill %s: %w", skill.Name, err)
		}
		models = append(models, model)
	}
	return models, nil
}

// applyGlobalDefaults writes each posted field of a kind the card lists into the rows' defaults, an empty field deleting it; a field the card lacks or a value its kind refuses fails the form.
func applyGlobalDefaults(card *globalDefaultsCard, form url.Values, rows *skills.Rows) error {
	listed := make(map[string]*skillgraph.Interface, len(card.Rows))
	for _, row := range card.Rows {
		listed[row.Interface.Id] = row.Interface
	}

	keys := slices.Sorted(maps.Keys(form))
	for _, key := range keys {
		iface, isListed := listed[key]
		if !isListed {
			return fmt.Errorf("%s: no selected skill reads this kind", key)
		}

		value := formValue(form, key)
		if value == "" {
			delete(rows.Defaults, key)
			continue
		}
		if err := skillgraph.CheckValue(iface, value); err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		rows.Defaults[key] = value
	}
	return nil
}

// applyProviderForm writes every posted PROVIDER_ field of a provider the models bind; the first unknown key or invalid program refuses the form.
func applyProviderForm(rows *skills.Rows, form url.Values, models []*skillgraph.Skill) error {
	bound := map[string]bool{}
	for id := range providerBindings(models) {
		bound[skills.ProviderKey(id)] = true
	}

	keys := slices.Sorted(maps.Keys(form))
	for _, key := range keys {
		if !strings.HasPrefix(key, providerPrefix) {
			continue
		}
		if err := applyProviderField(rows, bound[key], key, formValue(form, key)); err != nil {
			return err
		}
	}
	return nil
}

// declaringSkills names the models declaring at least one variable, sorted — the Environment tab's select.
func declaringSkills(models []*skillgraph.Skill) []string {
	var names []string
	for _, model := range models {
		if len(model.VariableNames()) > 0 {
			names = append(names, model.Name())
		}
	}
	slices.Sort(names)
	return names
}

// environmentForm keeps every posted field that names a declared repo-scoped variable of the models, validated against its record and resolved under repoPath.
func environmentForm(models []*skillgraph.Skill, form url.Values, repoPath string) (map[string]string, error) {
	declared := repoScopedVariables(models)
	env := map[string]string{}
	keys := slices.Sorted(maps.Keys(form))
	for _, key := range keys {
		variable, isDeclared := declared[key]
		if !isDeclared {
			continue
		}

		value := formValue(form, key)
		if value == "" {
			continue
		}
		if err := skillgraph.CheckValue(variable.Interface, value); err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		if cause := skills.ValidateLocation(repoPath, variable.Interface, value); cause != "" {
			return nil, fmt.Errorf("%s: %s", key, cause)
		}
		env[key] = value
	}
	return env, nil
}

// formValue is the trimmed field with a browser's CRLF line breaks as LF, the one line break a command value takes.
func formValue(form url.Values, key string) string {
	return strings.TrimSpace(strings.ReplaceAll(form.Get(key), "\r\n", "\n"))
}

// newGlobalDefaultsCard is one row per repo-scoped interface id the models bind, sorted, with the skills reading it and the record's global default.
func newGlobalDefaultsCard(models []*skillgraph.Skill, rows *skills.Rows) *globalDefaultsCard {
	byId := map[string]*globalDefaultRow{}
	declared := repoScopedVariables(models)
	for _, variable := range declared {
		if variable.Interface == nil {
			continue
		}

		id := variable.Interface.Id
		if byId[id] == nil {
			byId[id] = &globalDefaultRow{Interface: variable.Interface, Value: rows.Defaults[id]}
		}
		if !slices.Contains(byId[id].Skills, variable.Skill) {
			byId[id].Skills = append(byId[id].Skills, variable.Skill)
		}
	}

	card := &globalDefaultsCard{}
	ids := slices.Sorted(maps.Keys(byId))
	for _, id := range ids {
		slices.Sort(byId[id].Skills)
		card.Rows = append(card.Rows, *byId[id])
	}
	return card
}

// providerBindings maps each provider id to the install-scoped refs the models bind through it, sorted; repo-scoped capabilities have no install-wide program to report.
func providerBindings(models []*skillgraph.Skill) map[string]*providerBinding {
	bindings := map[string]*providerBinding{}
	for _, model := range models {
		repoScoped := model.RepoScoped()
		variables := model.VariableNames()
		names := slices.Sorted(maps.Keys(variables))
		for _, name := range names {
			if repoScoped[name] || model.Catalog == nil {
				continue
			}
			addBinding(bindings, model.Catalog.ProviderFor(variables[name].Interface), variables[name].Interface)
		}
	}
	for _, binding := range bindings {
		slices.Sort(binding.refs)
	}
	return bindings
}

// addBinding records ref under provider; a nil provider is skipped.
func addBinding(bindings map[string]*providerBinding, provider *skillgraph.Provider, ref string) {
	if provider == nil {
		return
	}
	if bindings[provider.Id] == nil {
		bindings[provider.Id] = &providerBinding{provider: provider}
	}

	binding := bindings[provider.Id]
	if !slices.Contains(binding.refs, ref) {
		binding.refs = append(binding.refs, ref)
	}
}

// providerStateOf: missing when the program is unset or not on PATH, stale when a bound version is absent from the handshake, current otherwise.
func providerStateOf(ctx context.Context, bound []string, isInstalled bool, prober skills.Prober, provider *skillgraph.Provider) providerState {
	state := providerState{Id: provider.Id, Program: provider.Program}
	componentTitle := components.Title(provider.Component)
	isProvided := isInstalled && provider.IsProvided()
	if !isProvided {
		state.State, state.Fix = stateMissing, "install the "+componentTitle+" component"
		return state
	}
	if _, err := exec.LookPath(skills.ExpandHome(provider.Executable())); err != nil {
		state.State, state.Fix = stateMissing, "install the "+componentTitle+" component — "+provider.Program+" is not on PATH"
		return state
	}

	claims, err := prober.Claims(ctx, provider)
	if err != nil {
		state.State, state.Fix = stateStale, "rebuild and install the "+componentTitle+" component — "+err.Error()
		return state
	}

	var lacking []string
	for _, ref := range bound {
		if !slices.Contains(claims, ref) {
			lacking = append(lacking, ref)
		}
	}
	if len(lacking) > 0 {
		state.State, state.Fix = stateStale, "lacks "+strings.Join(lacking, ", ")+" — rebuild and install the "+componentTitle+" component"
		return state
	}
	state.State = stateCurrent
	return state
}

// repoNames lists the registered repos by name, the skill page's selector.
func repoNames(registered []repos.Repo) []string {
	names := make([]string, 0, len(registered))
	for _, repo := range registered {
		names = append(names, repo.Name)
	}
	return names
}

// repoScopedVariables is every repo-scoped variable of the models by name, with its record, required flag and skill.
func repoScopedVariables(models []*skillgraph.Skill) map[string]declaredVariable {
	declared := map[string]declaredVariable{}
	for _, model := range models {
		variables := model.VariableNames()
		for name := range model.RepoScoped() {
			iface, _ := model.Catalog.Interface(variables[name].Interface)
			declared[name] = declaredVariable{Interface: iface, Required: variables[name].Required, Skill: model.Name()}
		}
	}
	return declared
}

package server

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/server/respond"
	"github.com/kevinhorst/smine/internal/sessions"
	"github.com/kevinhorst/smine/internal/sessiontitle"
	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skills"
)

const (
	pageSkills      = "skills"
	tmplSkillDetail = "skill_detail.html"
	tmplSkillsIndex = "skills_index.html"
)

const (
	skillStateActive = "active" // only in ~/.claude/skills
	skillStateRepo   = "repo"   // only in the repo
	skillStateSynced = "synced" // present in both roots
)

// Tabs of the Skills page: the skill list and the graph of one picked skill.
const (
	skillsTabGraph  = "graph"
	skillsTabSkills = "skills"
	skillsTitle     = "Skills"
	tmplSkillEnv    = "_skill_env.html"
)

// recentMajorCount is how many major versions show before "Older versions" collapses the rest.
const recentMajorCount = 2

// The Skills page's visibility action: off hides every deployed leaf in the user's sessions, on clears the overrides.
const (
	overrideOff     = "off"
	overrideOn      = "on"
	skillMarkerFile = ".smine-deployed"
)

// skillCard's HasSmokeColumn and HasUnsetColumn are page-wide, so every card's status grid keeps the same columns.
type skillCard struct {
	HasSmokeColumn bool
	HasUnsetColumn bool
	HomeVersion    string // set when the home variant's version differs (D7)
	Invocations    int
	LastUsed       string
	Missing        string // components the record lacks for this repo skill, comma-joined
	Skill          skills.Skill
	Smoke          *smokeBadge // nil without a smoke record
	State          string      // skillStateSynced | skillStateActive | skillStateRepo
	Unset          []string    // required variables without a value
}

type skillDetailPage struct {
	Body            template.HTML
	Changelog       []skillgraph.ChangelogEntry
	ChangelogError  string
	ChangelogOlder  []skillgraph.ChangelogEntry
	Env             skillEnvView
	EnvCount        int
	Examples        []string
	File            string
	FileContent     string
	InvocationLines []string
	Invocations     []sessions.Invocation
	Page            string
	Skill           *skills.Skill
	Smoke           *smokeBadge
	Tab             string
	Title           string
	TitleError      string
	TitleStates     []skillTitleState
	Toc             []skillTocEntry
	ToolCount       int
	Tools           []skills.ToolGroup
	Unset           []string
}

type skillGroup struct {
	Family        string
	Invocations   int // sum over Skills (D3)
	Skills        []skillCard
	Uninstalled   int      // members only the repo holds
	Unset         []string // "<skill>: <variables>" per member with a required variable unset, so a collapsed group still shows it
	UnsetCount    int      // the unset required variables over all members
	UpdatesNeeded int      // members with a version drift (D3)
}

// skillTitleState is one row of the Skill tab's Title states table.
type skillTitleState struct {
	Entry   string
	Meaning string
	State   string
}

type skillsIndexPage struct {
	CountActive     int
	CountAll        int
	CountRepo       int
	CountSynced     int
	Groups          []skillGroup
	IsCasual        bool
	Page            string
	SkillsMode      string
	State           string // "" | skillStateSynced | skillStateActive | skillStateRepo
	Tab             string
	Title           string
	VisibilityCount int // the deployed leaves and aliases the visibility action writes
}

func (s *Server) findSkill(w http.ResponseWriter, r *http.Request) *skills.Skill {
	list, err := skills.Scan(s.skillsRepo, s.skillsHome)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return nil
	}

	skill, ok := skills.Find(list, r.PathValue("origin"), r.PathValue("name"))
	if !ok {
		http.NotFound(w, r)
		return nil
	}

	return skill
}

func (s *Server) handleSkillDetail(w http.ResponseWriter, r *http.Request) {
	skill := s.findSkill(w, r)
	if skill == nil {
		return
	}

	manifest, err := skillManifest(skill)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	rendered, err := s.renderSkillMarkdown(stripSkillFrontmatter(manifest))
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	argNames := make([]string, 0, len(skill.Args))
	for _, arg := range skill.Args {
		argNames = append(argNames, arg.Name)
	}

	data := s.skillDetailData(r.Context(), r.URL.Query().Get("repo"), skill)
	data.Body = rendered.Body
	data.Toc = rendered.Toc
	data.InvocationLines = skills.FormatInvocationLines(skill.ArgumentHint, argNames)
	data.Tools = skills.GroupTools(skill.AllowedTools)
	for _, group := range data.Tools {
		data.ToolCount += len(group.Tools)
	}
	data.Title = "Skill — " + skill.Name
	s.renderFragment(w, tmplSkillDetail, data)
}

func (s *Server) skillDetailData(ctx context.Context, repo string, skill *skills.Skill) skillDetailPage {
	data := skillDetailPage{Page: pageSkills, Skill: skill, Smoke: s.lastSmoke(skill), Tab: "skill"}
	data.Env = s.skillEnv(ctx, repo, skill)
	data.EnvCount = data.Env.count()
	data.Unset = data.Env.Unset

	changelog, err := loadChangelog(skill.Path)
	if err != nil {
		data.ChangelogError = err.Error()
	}
	data.Changelog, data.ChangelogOlder = splitChangelogByMajor(changelog)

	data.TitleStates, err = skillTitleStates(skill)
	if err != nil {
		data.TitleError = err.Error()
	}

	data.Examples = exampleFiles(s.examplesDir, skill.Name)
	// Newest first so the client-side week bands expand the current week.
	invocations := skillInvocations(s.sessions.InvocationsBySkill(), skill)
	slices.SortStableFunc(invocations, func(left, right sessions.Invocation) int {
		return strings.Compare(right.LastActive, left.LastActive)
	})
	data.Invocations = invocations
	return data
}

// changelogMajor maps an unparseable version to -1, which sorts with its neighbors.
func changelogMajor(version string) int {
	head, _, _ := strings.Cut(version, ".")
	major, err := strconv.Atoi(head)
	if err != nil {
		return -1
	}
	return major
}

// loadChangelog yields nil for deployed copies: they carry only the rendered SKILL.md.
func loadChangelog(skillDir string) ([]skillgraph.ChangelogEntry, error) {
	if _, err := os.Stat(skillgraph.ModelPath(skillDir)); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("loadChangelog: Failed to stat %s: %w", skillDir, err)
	}

	skill, err := skillgraph.Load(skillDir)
	if err != nil {
		return nil, fmt.Errorf("loadChangelog: Failed to load %s: %w", skillDir, err)
	}
	return skill.Changelog, nil
}

// skillManifest renders a repo skill's model in memory; no checkout holds a SKILL.md.
func skillManifest(skill *skills.Skill) ([]byte, error) {
	if skill.Origin == skills.OriginRepo {
		return skillgraph.ReadManifest(skillgraph.ModelPath(skill.Path))
	}

	manifest, err := os.ReadFile(filepath.Join(skill.Path, skillgraph.FileManifest))
	if err != nil {
		return nil, fmt.Errorf("skillManifest: %w", err)
	}
	return manifest, nil
}

func skillTitleStates(skill *skills.Skill) ([]skillTitleState, error) {
	declaration, err := titleDeclaration(skill)
	if err != nil {
		return nil, fmt.Errorf("skillTitleStates: %w", err)
	}
	if declaration == nil {
		return nil, nil
	}

	states := make([]skillTitleState, 0, len(declaration.States))
	for _, token := range declaration.Tokens() {
		declared := declaration.States[string(token)]
		state := skillTitleState{Entry: declared.Entry, Meaning: declared.Meaning, State: string(token)}
		states = append(states, state)
	}
	return states, nil
}

// splitChangelogByMajor expects entries newest first.
func splitChangelogByMajor(entries []skillgraph.ChangelogEntry) ([]skillgraph.ChangelogEntry, []skillgraph.ChangelogEntry) {
	majors := make([]int, 0, recentMajorCount)
	var recent, older []skillgraph.ChangelogEntry
	for _, entry := range entries {
		major := changelogMajor(entry.Version)
		if !slices.Contains(majors, major) {
			majors = append(majors, major)
		}
		if slices.Index(majors, major) < recentMajorCount {
			recent = append(recent, entry)
			continue
		}
		older = append(older, entry)
	}
	return recent, older
}

func titleDeclaration(skill *skills.Skill) (*sessiontitle.Declaration, error) {
	if skill.Origin == skills.OriginRepo {
		model, err := skillgraph.Load(skill.Path)
		if err != nil {
			return nil, fmt.Errorf("titleDeclaration: %w", err)
		}
		return model.Title, nil
	}

	document, err := sessiontitle.LoadDocument(filepath.Join(skill.Path, sessiontitle.FileName))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("titleDeclaration: %w", err)
	}
	return document.Declaration, nil
}

func exampleFiles(examplesDir, skillName string) []string {
	entries, err := os.ReadDir(filepath.Join(examplesDir, skillName))
	if err != nil {
		return nil
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		files = append(files, entry.Name())
	}
	return files
}

func (s *Server) handleSkillFile(w http.ResponseWriter, r *http.Request) {
	skill := s.findSkill(w, r)
	if skill == nil {
		return
	}

	// User input is never joined into a path; it must match a scanned sibling by string equality.
	requested := r.URL.Query().Get("f")
	baseDir := skill.Path
	if !slices.Contains(skill.Files, requested) {
		if !slices.Contains(exampleFiles(s.examplesDir, skill.Name), requested) {
			http.NotFound(w, r)
			return
		}
		baseDir = filepath.Join(s.examplesDir, skill.Name)
	}

	content, err := os.ReadFile(filepath.Join(baseDir, requested))
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	data := skillDetailPage{
		File:        requested,
		FileContent: string(content),
		Page:        pageSkills,
		Skill:       skill,
		Smoke:       s.lastSmoke(skill),
		Tab:         "skill",
		Title:       "Skill — " + skill.Name + " — " + requested,
		Unset:       s.unsetVariables(r.Context(), skill),
	}
	s.renderFragment(w, tmplSkillDetail, data)
}

// handleSkillsSync syncs the record's selection for its component set; prune removes only dirs the sync marked (skills row).
func (s *Server) handleSkillsSync(w http.ResponseWriter, r *http.Request) {
	op := func(ctx context.Context) (string, error) {
		record, err := s.records.load(s.machine)
		if err != nil {
			return "", fmt.Errorf("handleSkillsSync: %w", err)
		}

		output, err := skills.Sync(s.skillOptions(record))
		s.titles.reload()
		return output, err
	}
	s.runRepoOp(syncLockKey, "repo-op", op, w, r)
}

// handleSkillDeploy records the skill in the selection, then deploys it: mode none becomes chosen with it, chosen gains it, all stays (concept Flow 4).
func (s *Server) handleSkillDeploy(w http.ResponseWriter, r *http.Request) {
	_, skill, ok := s.repoSkill(w, r)
	if !ok {
		return
	}

	deploySkill := func(ctx context.Context) (string, error) {
		record, err := s.records.load(s.machine)
		if err != nil {
			return "", fmt.Errorf("handleSkillDeploy: %w", err)
		}
		if missing := missingComponents(skill.Requires, record.Components); missing != "" {
			return "", fmt.Errorf("handleSkillDeploy: Skill %s unavailable (%s)", skill.Name, missing)
		}

		record.Skills = selectionWith(record.Skills, skill.Name)
		if err := install.Save(s.machine, record); err != nil {
			return "", fmt.Errorf("handleSkillDeploy: %w", err)
		}

		output, err := skills.DeployLeaf(skill.Name, s.skillOptions(record))
		s.titles.reload()
		return output, err
	}
	s.runRepoOp(syncLockKey, "repo-op", deploySkill, w, r)
}

// handleSkillEnv validates the posted SKILL_ and PROVIDER_ fields, writes them into the record (an empty field removes the row), keeps orphan rows for the widget's Forget button and redeploys a selected skill when an install-scoped row changed; a default-only save, an unselected skill or a refused deploy shows the op result.
func (s *Server) handleSkillEnv(w http.ResponseWriter, r *http.Request) {
	skill := s.findSkill(w, r)
	if skill == nil {
		return
	}
	if skill.Origin != skills.OriginRepo {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}

	if !s.repoLocks.TryAcquire(syncLockKey) {
		respond.WithConflict("an operation is already running", w)
		return
	}
	defer s.repoLocks.Release(syncLockKey)

	model, err := skillgraph.Load(skill.Path)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	record, err := s.records.load(s.machine)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	previous, rows := install.SkillRows(record), install.SkillRows(record)
	if err := applyEnvForm(rows, r.PostForm, model); err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}

	install.SetSkillRows(&record, rows)
	if err := install.Save(s.machine, record); err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	view := s.skillEnv(r.Context(), "", skill)
	switch {
	case !isInstallChange(previous, rows, r.PostForm, model.RepoScoped()):
		view.Notice = "saved — the next invocation reads the new default"
	case !isSelected(record.Skills, skill.Name):
		view.Notice = "saved — " + skill.Name + " is not selected, nothing deployed"
	default:
		output, err := skills.DeployLeaf(skill.Name, s.skillOptions(record))
		s.titles.reload()
		view.Notice, view.Output = "saved and deployed", output
		if err != nil {
			view.IsError, view.Notice = true, err.Error()
		}
	}
	s.renderFragment(w, tmplSkillEnv, view)
}

// handleSkillEnvWidget renders the Environment widget of a repo skill on ?repo=<name>, the defaults without one — the repo chips swap it in place.
func (s *Server) handleSkillEnvWidget(w http.ResponseWriter, r *http.Request) {
	skill := s.findSkill(w, r)
	if skill == nil {
		return
	}
	if skill.Origin != skills.OriginRepo {
		http.NotFound(w, r)
		return
	}
	s.renderFragment(w, tmplSkillEnv, s.skillEnv(r.Context(), r.URL.Query().Get("repo"), skill))
}

// handleSkillRemove drops the skill from the selection, then from both skill homes; mode all becomes chosen with every other available skill.
func (s *Server) handleSkillRemove(w http.ResponseWriter, r *http.Request) {
	list, skill, ok := s.repoSkill(w, r)
	if !ok {
		return
	}

	removeSkill := func(ctx context.Context) (string, error) {
		record, err := s.records.load(s.machine)
		if err != nil {
			return "", fmt.Errorf("handleSkillRemove: %w", err)
		}

		record.Skills = selectionWithout(record.Skills, skill.Name, availableLeaves(list, record.Components))
		if err := install.Save(s.machine, record); err != nil {
			return "", fmt.Errorf("handleSkillRemove: %w", err)
		}

		output, err := skills.RemoveLeaf(skill.Name, s.skillOptions(record))
		s.titles.reload()
		return output, err
	}
	s.runRepoOp(syncLockKey, "repo-op", removeSkill, w, r)
}

func (s *Server) handleSkillsIndex(w http.ResponseWriter, r *http.Request) {
	list, err := skills.Scan(s.skillsRepo, s.skillsHome)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	state := r.URL.Query().Get("state")
	if state != skillStateSynced && state != skillStateActive && state != skillStateRepo {
		state = ""
	}

	data := skillsIndexPage{Page: pageSkills, State: state, Tab: skillsTabSkills, Title: skillsTitle}
	var filtered []skillCard
	for _, merged := range mergeSkills(list) {
		data.CountAll++
		switch merged.State {
		case skillStateSynced:
			data.CountSynced++
		case skillStateActive:
			data.CountActive++
		case skillStateRepo:
			data.CountRepo++
		}
		if state == "" || merged.State == state {
			filtered = append(filtered, merged)
		}
	}
	cards := withUsage(filtered, s.sessions.InvocationsBySkill())
	record, err := s.records.load(s.machine)
	if err != nil {
		log.Printf("Server.handleSkillsIndex: %v", err)
	}

	data.SkillsMode = record.Skills.Mode
	data.IsCasual = !s.presentation.isDeveloperAudience()
	if data.IsCasual {
		leaves, err := s.deployedLeaves()
		if err != nil {
			log.Printf("Server.handleSkillsIndex: %v", err)
		}
		data.VisibilityCount = len(leaves)
	}
	for index := range cards {
		cards[index].Smoke = s.lastSmoke(&cards[index].Skill)
		cards[index].Missing = missingComponents(cards[index].Skill.Requires, record.Components)
		cards[index].Unset = s.unsetVariables(r.Context(), &cards[index].Skill)
	}
	markStatusColumns(cards)
	data.Groups = skillGroups(cards)
	s.renderFragment(w, tmplSkillsIndex, data)
}

// handleSkillVisibility sets (off) or clears (on) the skillOverrides of every deployed leaf in the user's settings.json — the casual profile's own click (master B16).
func (s *Server) handleSkillVisibility(w http.ResponseWriter, r *http.Request) {
	visibility := r.FormValue("visibility")
	if visibility != overrideOff && visibility != overrideOn {
		respond.WithBadRequest("visibility is off or on", w)
		return
	}

	value := ""
	if visibility == overrideOff {
		value = overrideOff
	}
	setVisibility := func(ctx context.Context) (string, error) {
		leaves, err := s.deployedLeaves()
		if err != nil {
			return "", err
		}
		if err := skills.SetOverrides(leaves, s.settingsPath, value, s.trash); err != nil {
			return "", fmt.Errorf("handleSkillVisibility: %w", err)
		}
		return fmt.Sprintf("skillOverrides: %d leaves %s", len(leaves), cmp.Or(value, "cleared")), nil
	}
	s.runRepoOp(syncLockKey, "repo-op", setVisibility, w, r)
}

// deployedLeaves names every dir under the Claude skill home the deploy marked — leaves and alias copies alike.
func (s *Server) deployedLeaves() ([]string, error) {
	markers, err := filepath.Glob(filepath.Join(s.skillsHome, "*", skillMarkerFile))
	if err != nil {
		return nil, fmt.Errorf("Server.deployedLeaves: %w", err)
	}

	leaves := make([]string, 0, len(markers))
	for _, marker := range markers {
		leaves = append(leaves, filepath.Base(filepath.Dir(marker)))
	}
	return leaves, nil
}

// repoSkill resolves {name} to a repo skill by string equality against the scan — request input never becomes a path on its own.
func (s *Server) repoSkill(w http.ResponseWriter, r *http.Request) ([]skills.Skill, *skills.Skill, bool) {
	list, err := skills.Scan(s.skillsRepo, s.skillsHome)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return nil, nil, false
	}

	skill, ok := skills.Find(list, skills.OriginRepo, r.PathValue("name"))
	if !ok {
		http.NotFound(w, r)
		return nil, nil, false
	}
	return list, skill, true
}

// skillOptions builds the sync options for the record's selection and component set from the server's checkout.
func (s *Server) skillOptions(record install.Record) *skills.SyncOptions {
	selection := &skills.Selection{Chosen: record.Skills.Chosen, Mode: record.Skills.Mode}
	contextDir, err := install.Resolve(record, components.Context, "context_dir")
	if err != nil {
		log.Printf("Server.skillOptions: %v", err)
	}

	options := &skills.SyncOptions{
		Components: record.Components,
		ContextDir: contextDir.Text,
		Home:       s.machine.Home,
		Prune:      true,
		RepoDir:    s.repoDir,
		Selection:  selection,
		Rows:       install.SkillRows(record),
		Trash:      s.trash,
	}
	return options
}

func mergeSkills(list []skills.Skill) []skillCard {
	var cards []skillCard
	for _, skill := range list {
		if n := len(cards); n > 0 && cards[n-1].Skill.Name == skill.Name {
			if cards[n-1].Skill.Version != skill.Version {
				cards[n-1].HomeVersion = cards[n-1].Skill.Version
			}
			cards[n-1].Skill = skill // repo entry replaces the home one
			cards[n-1].State = skillStateSynced
			continue
		}

		state := skillStateRepo
		if skill.Origin == skills.OriginHome {
			state = skillStateActive
		}
		cards = append(cards, skillCard{Skill: skill, State: state})
	}
	return cards
}

func withUsage(cards []skillCard, invocations map[string][]sessions.Invocation) []skillCard {
	for index := range cards {
		invoked := skillInvocations(invocations, &cards[index].Skill)
		cards[index].Invocations = len(invoked)
		for _, invocation := range invoked {
			if usedOn := dateOf(invocation.LastActive); usedOn > cards[index].LastUsed {
				cards[index].LastUsed = usedOn
			}
		}
	}
	return cards
}

// markStatusColumns opens the unset and smoke columns on every card once any card fills them, and drops them from a page that has none.
func markStatusColumns(cards []skillCard) {
	hasSmoke := slices.ContainsFunc(cards, func(card skillCard) bool { return card.Smoke != nil })
	hasUnset := slices.ContainsFunc(cards, func(card skillCard) bool { return len(card.Unset) > 0 })
	for index := range cards {
		cards[index].HasSmokeColumn = hasSmoke
		cards[index].HasUnsetColumn = hasUnset
	}
}

func skillGroups(list []skillCard) []skillGroup {
	familyOf := func(card skillCard) string {
		if card.Skill.Group != "" {
			return card.Skill.Group
		}
		return card.Skill.Name
	}
	slices.SortStableFunc(list, func(left, right skillCard) int {
		if byFamily := strings.Compare(familyOf(left), familyOf(right)); byFamily != 0 {
			return byFamily
		}
		return strings.Compare(left.Skill.Name, right.Skill.Name)
	})

	var groups []skillGroup
	for _, card := range list {
		family := familyOf(card)
		if n := len(groups); n == 0 || groups[n-1].Family != family {
			groups = append(groups, skillGroup{Family: family})
		}
		group := &groups[len(groups)-1]
		group.Skills = append(group.Skills, card)
		group.Invocations += card.Invocations
		if card.HomeVersion != "" {
			group.UpdatesNeeded++
		}
		if card.State == skillStateRepo {
			group.Uninstalled++
		}
		if len(card.Unset) > 0 {
			group.Unset = append(group.Unset, card.Skill.Name+": "+strings.Join(card.Unset, ", "))
			group.UnsetCount += len(card.Unset)
		}
	}
	return groups
}

// availableLeaves lists the repo skills whose requirements the set meets.
func availableLeaves(list []skills.Skill, installed []components.ID) []string {
	var leaves []string
	for _, skill := range list {
		if skill.Origin != skills.OriginRepo {
			continue
		}
		if missingComponents(skill.Requires, installed) == "" {
			leaves = append(leaves, skill.Name)
		}
	}
	return leaves
}

// isSelected mirrors the deploy's choice: mode all deploys every leaf, chosen its list, none no leaf.
func isSelected(selection install.Selection, leaf string) bool {
	if selection.Mode == skills.SelectionModeAll {
		return true
	}
	return slices.Contains(selection.Chosen, leaf)
}

// missingComponents is empty when the skill is available.
func missingComponents(requires, installed []components.ID) string {
	var missing []components.ID
	for _, id := range requires {
		if !slices.Contains(installed, id) {
			missing = append(missing, id)
		}
	}
	return joinComponentIds(missing)
}

func selectionWith(selection install.Selection, leaf string) install.Selection {
	switch selection.Mode {
	case skills.SelectionModeAll:
		return selection
	case skills.SelectionModeChosen:
		if slices.Contains(selection.Chosen, leaf) {
			return selection
		}
		selection.Chosen = append(slices.Clone(selection.Chosen), leaf)
	case skills.SelectionModeNone:
		selection.Chosen = []string{leaf}
		selection.Mode = skills.SelectionModeChosen
	}
	return selection
}

// selectionWithout turns mode all into chosen with every other available skill; an empty chosen list becomes none.
func selectionWithout(selection install.Selection, leaf string, available []string) install.Selection {
	chosen := selection.Chosen
	if selection.Mode == skills.SelectionModeAll {
		chosen = available
	}

	remaining := slices.DeleteFunc(slices.Clone(chosen), func(other string) bool {
		return other == leaf
	})
	selection.Chosen = remaining
	selection.Mode = skills.SelectionModeChosen
	if len(remaining) == 0 {
		selection.Chosen = nil
		selection.Mode = skills.SelectionModeNone
	}
	return selection
}

// skillInvocations includes invocations recorded under each declared alias.
func skillInvocations(invocations map[string][]sessions.Invocation, skill *skills.Skill) []sessions.Invocation {
	invoked := slices.Clone(invocations[skill.Name])
	for _, alias := range skill.Aliases {
		invoked = append(invoked, invocations[alias]...)
	}
	return invoked
}

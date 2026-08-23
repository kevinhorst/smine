package server

import (
	"context"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/server/respond"
	"github.com/kevinhorst/smine/internal/sessions"
	"github.com/kevinhorst/smine/internal/skills"
)

const (
	pageSkills      = "skills"
	tmplSkillDetail = "skill_detail.html"
	tmplSkillsIndex = "skills_index.html"

	// skillsSyncLockKey serializes the global skill sync via the repo-locks
	// map, like jetbrainsLockKey (D30): the underscore prefix keeps it out of
	// the registry's repo-name space.
	skillsSyncLockKey = "_skills-sync"
)

const (
	skillStateActive = "active" // only in ~/.claude/skills
	skillStateRepo   = "repo"   // only in the repo
	skillStateSynced = "synced" // present in both roots
)

type skillCard struct {
	HomeVersion string // set when the home variant's version differs (D7)
	Invocations int
	LastUsed    string
	Skill       skills.Skill
	State       string // skillStateSynced | skillStateActive | skillStateRepo
}

type skillDetailPage struct {
	Body           template.HTML
	Changelog      []skills.ChangelogEntry
	ChangelogError string
	Examples       []string
	File           string
	FileContent    string
	Invocations    []sessions.Invocation
	Page           string
	Skill          *skills.Skill
	Tab            string
	Title          string
}

type skillGroup struct {
	Family        string
	Invocations   int // sum over Skills (D3)
	Skills        []skillCard
	UpdatesNeeded int // members with a version drift (D3)
}

type skillsIndexPage struct {
	CountActive int
	CountAll    int
	CountRepo   int
	CountSynced int
	Groups      []skillGroup
	Page        string
	State       string // "" | skillStateSynced | skillStateActive | skillStateRepo
	Title       string
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

	manifest, err := os.ReadFile(filepath.Join(skill.Path, "SKILL.md"))
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	body, err := s.renderDocMarkdown(manifest)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	data := s.skillDetailData(skill)
	data.Body = body
	data.Title = "Skill — " + skill.Name
	s.renderFragment(w, tmplSkillDetail, data)
}

func (s *Server) skillDetailData(skill *skills.Skill) skillDetailPage {
	data := skillDetailPage{Page: pageSkills, Skill: skill, Tab: "skill"}

	changelog, err := skills.LoadChangelog(skill.Path)
	if err != nil {
		data.ChangelogError = err.Error()
	}
	data.Changelog = changelog

	data.Examples = exampleFiles(s.examplesDir, skill.Name)
	data.Invocations = s.sessions.InvocationsBySkill()[skill.Name]
	return data
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

	// The requested file must match a scanned sibling — or an enumerated
	// examples/<skill>/ entry — by string equality; user input is never
	// joined into a path on its own (concept security rule).
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
		Tab:         "skill",
		Title:       "Skill — " + skill.Name + " — " + requested,
	}
	s.renderFragment(w, tmplSkillDetail, data)
}

func (s *Server) handleSkillsSync(w http.ResponseWriter, r *http.Request) {
	prune := r.FormValue("prune") == "on"
	op := func(ctx context.Context) (string, error) {
		return skills.Sync(ctx, prune, s.syncScripts)
	}
	s.runRepoOp(skillsSyncLockKey, "repo-op", op, w, r)
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

	data := skillsIndexPage{Page: pageSkills, State: state, Title: "Skills"}
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
	data.Groups = skillGroups(withUsage(filtered, s.sessions.InvocationsBySkill()))
	s.renderFragment(w, tmplSkillsIndex, data)
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
		invoked := invocations[cards[index].Skill.Name]
		cards[index].Invocations = len(invoked)
		for _, invocation := range invoked {
			if invocation.BatchDate > cards[index].LastUsed {
				cards[index].LastUsed = invocation.BatchDate
			}
		}
	}
	return cards
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
	}
	slices.SortStableFunc(groups, func(a, b skillGroup) int {
		multiA, multiB := len(a.Skills) > 1, len(b.Skills) > 1
		switch {
		case multiA == multiB:
			return 0
		case multiA:
			return -1
		default:
			return 1
		}
	})
	return groups
}

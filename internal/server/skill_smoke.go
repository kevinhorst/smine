package server

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/kevinhorst/smine/internal/skilleval"
	"github.com/kevinhorst/smine/internal/skills"
)

// smokeAnchorView is one expected anchor of the record, hit or never hit in its runtime.
type smokeAnchorView struct {
	Id string

	IsHit bool
}

type smokeBadge struct {
	Cause    string
	Coverage string // empty for a terminal-verdict smoke
	Current  string // the skill's version, shown when the record is stale
	Date     string
	IsPass   bool
	IsStale  bool
	Verdict  string
	Version  string
}

func newSmokeBadge(currentVersion string, record *skilleval.ArmResult) *smokeBadge {
	if record == nil {
		return nil
	}

	smoke := record.Smoke
	if smoke == nil {
		return nil
	}

	badge := &smokeBadge{
		Cause:   smoke.Cause,
		Current: currentVersion,
		Date:    record.EndedAt.Format(time.DateOnly),
		IsPass:  smoke.Verdict == statusPass,
		IsStale: record.SkillVersion != currentVersion,
		Verdict: smoke.Verdict,
		Version: record.SkillVersion,
	}
	if smoke.Coverage.AnchorsDeclared > 0 {
		badge.Coverage = smoke.Coverage.Summary()
	}
	return badge
}

// smokeCardView is the Tests tab's smoke card: the newest record judged against its own expectation.
type smokeCardView struct {
	Anchors []smokeAnchorView
	Badge   *smokeBadge
	Checks  string
	Command string
	Cost    string
	Dir     string
	Session string
	Tokens  string
}

// lastSmoke logs an unreadable record instead of failing the page; the skill's version decides staleness.
func (s *Server) lastSmoke(skill *skills.Skill) *smokeBadge {
	if s.smokeDir == "" {
		return nil
	}

	record, err := skilleval.LastSmoke(s.smokeDir, skill.Name)
	if err != nil {
		log.Printf("Server.lastSmoke: Skill %s: %v", skill.Name, err)
		return nil
	}
	return newSmokeBadge(skill.Version, record)
}

// smokeCard reads the newest record once more by its version; an unreadable record leaves the tab without a card.
func (s *Server) smokeCard(skill *skills.Skill) *smokeCardView {
	badge := s.lastSmoke(skill)
	if badge == nil {
		return nil
	}

	record, err := skilleval.LoadSmokeRecord(s.smokeDir, skill.Name, badge.Version)
	if err != nil {
		log.Printf("Server.smokeCard: Skill %s: %v", skill.Name, err)
		return nil
	}

	coverage := record.Result.Smoke.Coverage
	view := &smokeCardView{
		Badge:   badge,
		Checks:  fmt.Sprintf("%d/%d", coverage.ChecksPassed, coverage.ChecksDeclared),
		Command: "make skill-smoke SKILL=" + skill.Name + " TOKEN=" + record.Manifest.TokenLabel,
		Dir:     record.Dir,
		Session: record.Result.SessionId,
	}
	if record.Result.CostUsd != nil {
		view.Cost = fmt.Sprintf("%.2f USD", *record.Result.CostUsd)
	}
	if record.Result.OutputTokens != nil {
		view.Tokens = strconv.Itoa(*record.Result.OutputTokens)
	}
	hits := smokeHits(record.Runtime)
	for _, id := range coverage.AnchorsExpected {
		anchor := smokeAnchorView{Id: id, IsHit: hits[id]}
		view.Anchors = append(view.Anchors, anchor)
	}
	return view
}

// smokeHits is the set of anchor ids the record's runs hit; no runtime capture hits nothing.
func smokeHits(captured *skilleval.ArmRuntime) map[string]bool {
	hits := map[string]bool{}
	if captured == nil {
		return hits
	}

	for _, run := range captured.Runs {
		for _, anchor := range run.Anchors {
			hits[anchor.Id] = true
		}
	}
	return hits
}

package skillgraph

import (
	"cmp"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Smoke run modes; an unset mode runs nightly.
const (
	SmokeModeInteractive = "interactive"
	SmokeModeNightly     = "nightly"
)

// Defaults a resolved smoke document fills in for an unset run field.
const (
	smokeDefaultEffort  = "low"
	smokeDefaultMinutes = 15
	smokeDefaultModel   = "sonnet"
	smokeDefaultUsd     = 3.0
)

// smokeSchema is the schema number of a resolved smoke document.
const smokeSchema = 1

// Smoke's {token} in Invoke is the label the smoke runs under, never written into the model file; Env holds the repo-scoped values the run sets by env key, the machine's never reach it.
type Smoke struct {
	Effort  string            `json:"effort,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Expect  *SmokeExpect      `json:"expect,omitempty"`
	Fixture *SmokeFixture     `json:"fixture,omitempty"`
	Invoke  *string           `json:"invoke,omitempty"`
	Limits  *SmokeLimits      `json:"limits,omitempty"`
	Mode    string            `json:"mode,omitempty"`
	Model   string            `json:"model,omitempty"`
	Skip    string            `json:"skip,omitempty"`
}

// SmokeDocument sets MinAnchors and MinChecks exactly when Expect names anchors.
type SmokeDocument struct {
	*Smoke

	Schema  int    `json:"schema"`
	Skill   string `json:"skill"`
	Version string `json:"version"`
}

type SmokeExpect struct {
	Anchors    []string `json:"anchors,omitempty"`
	Check      []string `json:"check,omitempty"`
	MinAnchors *float64 `json:"minAnchors,omitempty"`
	MinChecks  *float64 `json:"minChecks,omitempty"`
}

type SmokeFixture struct {
	Patch string `json:"patch,omitempty"`
	Tree  string `json:"tree,omitempty"`
}

// SmokeLimits bounds a run; a run over a limit fails.
type SmokeLimits struct {
	Minutes int     `json:"minutes,omitempty"`
	Usd     float64 `json:"usd,omitempty"`
}

// checkSmoke leaves fixture presence on disk to Load.
func checkSmoke(skill *Skill) []string {
	smoke := skill.Smoke
	if smoke == nil {
		return nil
	}

	hasSkip := smoke.Skip != ""
	isRun := smoke.Invoke != nil
	switch {
	case hasSkip && isRun:
		return []string{"smoke: skip and invoke both declared — a smoke is a skip or a run"}
	case hasSkip:
		return nil
	case !isRun:
		return []string{"smoke: neither skip nor invoke declared — declare a run, or a skip with its reason"}
	}

	var anchors []string
	if smoke.Expect != nil {
		anchors = smoke.Expect.Anchors
	}
	var declared map[string]*Anchor
	if skill.Enforce != nil {
		declared = skill.Enforce.Anchors
	}
	var violations []string
	for _, anchor := range anchors {
		if _, isDeclared := declared[anchor]; !isDeclared {
			violations = append(violations, fmt.Sprintf("smoke: expect anchor %s: no such anchor in the enforce block", anchor))
		}
	}
	isAnchorJudged := len(anchors) > 0
	if smoke.Mode == SmokeModeInteractive && !isAnchorJudged {
		violations = append(violations, "smoke: an interactive run declares expect.anchors — its verdict comes from the session's heartbeat rows")
	}
	if smoke.Fixture != nil {
		violations = append(violations, smokeFixturePathViolations("patch", smoke.Fixture.Patch)...)
		violations = append(violations, smokeFixturePathViolations("tree", smoke.Fixture.Tree)...)
	}
	return append(violations, smokeEnvViolations(skill)...)
}

// smokeEnvViolations: a run sets only declared repo-scoped variables and every required one; scope is judged only against a known catalog record.
func smokeEnvViolations(skill *Skill) []string {
	var violations []string
	keys := slices.Sorted(maps.Keys(skill.Smoke.Env))
	for _, key := range keys {
		variable, isDeclared := skill.Env[key]
		if !isDeclared {
			violations = append(violations, fmt.Sprintf("smoke: env %s: no env entry declares it", key))
			continue
		}

		iface, isKnown := skill.catalog().Interface(variable.Interface)
		if isKnown && !iface.IsRepoScoped() {
			violations = append(violations, fmt.Sprintf("smoke: env %s: %s is install-scoped — a smoke sets repo-scoped values only", key, iface.Ref()))
		}
	}

	envKeys := slices.Sorted(maps.Keys(skill.Env))
	for _, key := range envKeys {
		variable := skill.Env[key]
		iface, isKnown := skill.catalog().Interface(variable.Interface)
		isRequiredRepoScoped := variable.Required && isKnown && iface.IsRepoScoped()
		if _, isSet := skill.Smoke.Env[key]; isRequiredRepoScoped && !isSet {
			violations = append(violations, fmt.Sprintf("smoke: env %s: required and repo-scoped — set it in the smoke's env, a smoke never reads the machine's value", key))
		}
	}
	return violations
}

// ratioOrZero is a declared coverage minimum, 0 when unset.
func ratioOrZero(ratio *float64) float64 {
	if ratio == nil {
		return 0
	}
	return *ratio
}

func renderHelpTest(hasEval bool, skill *Skill) []byte {
	leaf := skill.Name()
	smoke := skill.Smoke
	var lines []string
	switch {
	case smoke == nil:
	case smoke.Invoke == nil:
		lines = append(lines, "Smoke: skipped — "+smoke.Skip)
	case smoke.Mode == SmokeModeInteractive:
		command := strings.TrimSpace("/" + leaf + " " + *smoke.Invoke)
		lines = append(lines, fmt.Sprintf("Smoke (interactive): run %s in a session, then make skill-smoke-verdict SKILL=%s SESSION=<id>", command, leaf))
	default:
		lines = append(lines, "Smoke: make skill-smoke SKILL="+leaf+" TOKEN=<label>")
	}
	if hasEval {
		lines = append(lines, "Eval: /skillroutine-eval "+leaf)
	}
	if len(lines) == 0 {
		return nil
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

// resolvedSmoke relies on the schema guaranteeing a run's Expect.
func resolvedSmoke(smoke *Smoke) *Smoke {
	if smoke.Invoke == nil {
		return &Smoke{Skip: smoke.Skip}
	}

	resolved := &Smoke{
		Effort:  cmp.Or(smoke.Effort, smokeDefaultEffort),
		Env:     smoke.Env,
		Expect:  resolvedSmokeExpect(smoke.Expect),
		Fixture: smoke.Fixture,
		Invoke:  smoke.Invoke,
		Limits:  resolvedSmokeLimits(smoke.Limits),
		Mode:    cmp.Or(smoke.Mode, SmokeModeNightly),
		Model:   cmp.Or(smoke.Model, smokeDefaultModel),
	}
	return resolved
}

func resolvedSmokeExpect(expect *SmokeExpect) *SmokeExpect {
	resolved := &SmokeExpect{Anchors: expect.Anchors, Check: expect.Check}
	if len(expect.Anchors) == 0 {
		return resolved
	}

	minAnchors := ratioOrZero(expect.MinAnchors)
	minChecks := ratioOrZero(expect.MinChecks)
	resolved.MinAnchors = &minAnchors
	resolved.MinChecks = &minChecks
	return resolved
}

// resolvedSmokeLimits fills each unset limit with its default.
func resolvedSmokeLimits(limits *SmokeLimits) *SmokeLimits {
	if limits == nil {
		limits = &SmokeLimits{}
	}
	return &SmokeLimits{Minutes: cmp.Or(limits.Minutes, smokeDefaultMinutes), Usd: cmp.Or(limits.Usd, smokeDefaultUsd)}
}

// smokeFixtureFileViolations leaves a path checkSmoke refuses to it.
func smokeFixtureFileViolations(dir string, field string, fixturePath string, isTree bool) []string {
	isCheckable := fixturePath != "" && len(smokeFixturePathViolations(field, fixturePath)) == 0
	if !isCheckable {
		return nil
	}

	info, err := os.Stat(filepath.Join(dir, DirSmoke, fixturePath))
	if err != nil {
		return []string{fmt.Sprintf("smoke: fixture %s %s: %v", field, fixturePath, err)}
	}

	if info.IsDir() == isTree {
		return nil
	}
	return []string{fmt.Sprintf("smoke: fixture %s %s: wrong kind — tree names a directory, patch a file", field, fixturePath)}
}

func smokeFixturePathViolations(field string, fixturePath string) []string {
	if fixturePath == "" {
		return nil
	}

	hasParent := slices.Contains(strings.Split(filepath.ToSlash(fixturePath), "/"), "..")
	if filepath.IsLocal(fixturePath) && !hasParent {
		return nil
	}
	return []string{fmt.Sprintf("smoke: fixture %s %q leaves %s/ — name a path inside it", field, fixturePath, DirSmoke)}
}

func smokeFixtureViolations(dir string, smoke *Smoke) []string {
	if smoke == nil || smoke.Fixture == nil {
		return nil
	}

	violations := smokeFixtureFileViolations(dir, "patch", smoke.Fixture.Patch, false)
	return append(violations, smokeFixtureFileViolations(dir, "tree", smoke.Fixture.Tree, true)...)
}

// LoadSmoke resolves in memory; no file holds the document.
func LoadSmoke(skillDir string) (*SmokeDocument, error) {
	skill, err := Load(skillDir)
	if err != nil {
		return nil, fmt.Errorf("LoadSmoke: %w", err)
	}
	return SmokeOf(skill), nil
}

func SmokeOf(skill *Skill) *SmokeDocument {
	if skill.Smoke == nil {
		return nil
	}

	document := &SmokeDocument{Schema: smokeSchema, Skill: skill.Name(), Smoke: resolvedSmoke(skill.Smoke), Version: skill.Version()}
	return document
}

func MissingSmokes(rootSkills []*Skill) []string {
	var violations []string
	for _, skill := range rootSkills {
		if skill.Smoke == nil {
			violations = append(violations, skill.Name()+": no smoke block — declare a run, or a skip with its reason, in "+ModelFile(skill.Name()))
		}
	}
	return violations
}

package skilleval

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/shell"
	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skills"
	"github.com/kevinhorst/smine/internal/trash"
)

const (
	smokeArmLabel    = "A"
	smokeRootDir     = ".cache/claude-routine/artifacts/smoke"
	smokeSkillsDir   = "skills"
	smokeStagingDir  = ".staging"
	smokeStampLayout = "2006-01-02-150405"

	// smokeTokenPlaceholder exists because no token label is ever checked in.
	smokeTokenPlaceholder = "{token}"
	// smokeVariablePrefix opens every rendered skill variable a machine may have exported.
	smokeVariablePrefix = "SKILL_"
)

type SmokeOutcome struct {
	IsPass bool
	Lines  []string
}

// SmokeRecord is one stored smoke run: its result, its heartbeat rows and its run manifest, all under Dir.
type SmokeRecord struct {
	Dir      string
	Manifest *RunManifest
	Result   *ArmResult
	Runtime  *ArmRuntime
}

func fixturePath(entry, skillDir string) string {
	if entry == "" {
		return ""
	}
	return filepath.Join(skillDir, skillgraph.DirSmoke, entry)
}

func fixturePaths(document *skillgraph.SmokeDocument, skillDir string) (string, string) {
	if document.Fixture == nil {
		return "", ""
	}
	return fixturePath(document.Fixture.Patch, skillDir), fixturePath(document.Fixture.Tree, skillDir)
}

func interactiveOutcome(document *skillgraph.SmokeDocument, tokenLabel string) *SmokeOutcome {
	lines := []string{
		fmt.Sprintf("smoke %s %s: interactive — run %s in a session", document.Skill, document.Version, smokePrompt(document, tokenLabel)),
		"then: make skill-smoke-verdict SKILL=" + document.Skill + " SESSION=<session id>",
	}
	return &SmokeOutcome{IsPass: true, Lines: lines}
}

// loadDeployedEnforce reads the deployed declaration: the one heartbeat enforced and coverage counts against.
func loadDeployedEnforce(document *skillgraph.SmokeDocument) (*skillgraph.Enforce, error) {
	if !hasAnchorExpectation(document) {
		return nil, nil
	}

	path := filepath.Join(skills.DefaultHomePath(), document.Skill, skillgraph.FileEnforce)
	var deployed skillgraph.EnforceDocument
	if err := readJsonFile(path, &deployed); err != nil {
		return nil, err
	}

	if deployed.Enforce == nil {
		return nil, fmt.Errorf("loadDeployedEnforce: %s: No anchors or checks", path)
	}
	return deployed.Enforce, nil
}

func loadSmoke(ctx context.Context, leaf string) (string, *skillgraph.SmokeDocument, error) {
	checkout, err := shell.Run(ctx, "", "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", nil, fmt.Errorf("loadSmoke: %w", err)
	}

	skillsRoot := filepath.Join(strings.TrimSpace(checkout), smokeSkillsDir)
	dirs, err := skillgraph.Discover(skillsRoot)
	if err != nil {
		return "", nil, fmt.Errorf("loadSmoke: %w", err)
	}

	index := slices.IndexFunc(dirs, func(dir string) bool { return filepath.Base(dir) == leaf })
	if index < 0 {
		return "", nil, fmt.Errorf("loadSmoke: No skill %s under %s", leaf, skillsRoot)
	}

	document, err := skillgraph.LoadSmoke(dirs[index])
	if err != nil {
		return "", nil, fmt.Errorf("loadSmoke: %w", err)
	}

	if document == nil {
		return "", nil, fmt.Errorf("loadSmoke: Skill %s declares no smoke: %s has no smoke block", leaf, skillgraph.ModelPath(dirs[index]))
	}
	return dirs[index], document, nil
}

func newSmokeManifest(atCommit string, document *skillgraph.SmokeDocument, mode, overlay, tokenLabel string) *RunManifest {
	arm := ManifestArm{
		Id:      smokeArmLabel,
		Effort:  document.Effort,
		Key:     smokeArmLabel,
		Label:   smokeArmLabel,
		Model:   document.Model,
		Replica: 1,
	}
	timeoutS := document.Limits.Minutes * secondsPerMinute
	manifest := &RunManifest{
		Arms:          []ManifestArm{arm},
		At:            atCommit,
		BudgetUsd:     document.Limits.Usd,
		Check:         document.Expect.Check,
		CheckTimeoutS: timeoutS,
		Date:          time.Now().Format(time.DateOnly),
		Layer:         LayerSmoke,
		Mode:          mode,
		OverlayPatch:  overlay,
		Prompt:        smokePrompt(document, tokenLabel),
		Skill:         document.Skill,
		SkillVersion:  document.Version,
		TimeoutS:      timeoutS,
		TokenLabel:    tokenLabel,
	}
	return manifest
}

// newStagingDir's name feeds the arm's worktree and branch names, so no two smokes share one.
func newStagingDir(leaf string) (string, error) {
	root, err := SmokeRoot()
	if err != nil {
		return "", err
	}

	runDir := filepath.Join(root, smokeStagingDir, time.Now().Format(smokeStampLayout)+"-"+leaf)
	if err := os.MkdirAll(filepath.Join(runDir, runsDir, smokeArmLabel), 0o755); err != nil {
		return "", fmt.Errorf("newStagingDir: %w", err)
	}
	return runDir, nil
}

// placeFixtureTree fails on an existing fixture file, never overwrites.
func placeFixtureTree(tree, worktree string) error {
	if tree == "" {
		return nil
	}

	if err := os.CopyFS(worktree, os.DirFS(tree)); err != nil {
		return fmt.Errorf("placeFixtureTree: %w", err)
	}
	return nil
}

func publishRecord(leaf, stagingDir, version string) error {
	root, err := SmokeRoot()
	if err != nil {
		return err
	}

	versionDir := filepath.Join(root, leaf, version)
	if err := versionAside(versionDir); err != nil {
		return fmt.Errorf("publishRecord: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(versionDir), 0o755); err != nil {
		return fmt.Errorf("publishRecord: %w", err)
	}

	if err := os.Rename(stagingDir, versionDir); err != nil {
		return fmt.Errorf("publishRecord: %w", err)
	}
	return nil
}

func recordRun(document *skillgraph.SmokeDocument, enforce *skillgraph.Enforce, runDir string) (*SmokeResult, error) {
	armDir := filepath.Join(runDir, runsDir, smokeArmLabel)
	var result ArmResult
	if err := readJsonFile(filepath.Join(armDir, resultFile), &result); err != nil {
		return nil, err
	}

	captured, err := loadRuntime(filepath.Join(armDir, runtimeFile))
	if err != nil {
		return nil, err
	}

	if captured == nil {
		return nil, fmt.Errorf("recordRun: %s: No runtime.json", armDir)
	}

	result.Smoke = judgeRun(document, enforce, &result, captured)
	if err := writeJsonFile(filepath.Join(armDir, resultFile), &result); err != nil {
		return nil, err
	}
	return result.Smoke, nil
}

func runFixtureArm(ctx context.Context, document *skillgraph.SmokeDocument, readRuntime RuntimeReader, skillDir, tokenLabel string) (string, error) {
	atCommit, err := resolveRef(ctx, "HEAD", skillDir)
	if err != nil {
		return "", err
	}

	runDir, err := newStagingDir(document.Skill)
	if err != nil {
		return "", err
	}

	patch, tree := fixturePaths(document, skillDir)
	manifest := newSmokeManifest(atCommit, document, ModeNightly, patch, tokenLabel)
	if err := writeManifest(runDir, manifest); err != nil {
		return "", err
	}

	if err := Provision(ctx, runDir); err != nil {
		return "", err
	}

	provisioned, err := loadManifest(runDir)
	if err != nil {
		return "", err
	}

	if err := placeFixtureTree(tree, provisioned.Arms[0].Worktree); err != nil {
		return "", err
	}

	if err := RunArm(ctx, smokeArmLabel, "", runDir, readRuntime); err != nil {
		return "", err
	}
	return runDir, nil
}

// exportSmokeEnv leaves the session no machine skill value: every inherited SKILL_ variable but the arm's context index goes, the smoke's own values are set and the skillenv hook is switched off.
func exportSmokeEnv(document *skillgraph.SmokeDocument) error {
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		isMachineValue := strings.HasPrefix(name, smokeVariablePrefix) && name != contextIndexEnv
		if !isMachineValue {
			continue
		}

		if err := os.Unsetenv(name); err != nil {
			return fmt.Errorf("exportSmokeEnv: %w", err)
		}
	}

	keys := slices.Sorted(maps.Keys(document.Env))
	for _, key := range keys {
		if err := os.Setenv(skillgraph.VariableName(document.Skill, key), document.Env[key]); err != nil {
			return fmt.Errorf("exportSmokeEnv: %w", err)
		}
	}

	if err := os.Setenv(config.SkillEnvOffEnv, "1"); err != nil {
		return fmt.Errorf("exportSmokeEnv: %w", err)
	}
	return nil
}

// runNightly leaves the staging dir and worktree of a harness failure for inspection.
func runNightly(ctx context.Context, document *skillgraph.SmokeDocument, readRuntime RuntimeReader, skillDir, tokenLabel string) (*SmokeOutcome, error) {
	if err := exportToken(tokenLabel); err != nil {
		return nil, err
	}

	if err := exportSmokeEnv(document); err != nil {
		return nil, err
	}

	enforce, err := loadDeployedEnforce(document)
	if err != nil {
		return nil, err
	}

	runDir, err := runFixtureArm(ctx, document, readRuntime, skillDir, tokenLabel)
	if err != nil {
		return nil, err
	}

	smoke, err := recordRun(document, enforce, runDir)
	if err != nil {
		return nil, err
	}

	stateDir, err := config.StateDir("")
	if err != nil {
		return nil, fmt.Errorf("runNightly: %w", err)
	}

	report, err := CleanupArm(ctx, smokeArmLabel, runDir, trash.New(stateDir, 0))
	if err != nil {
		return nil, err
	}

	if !report.IsAllRemoved() {
		return nil, fmt.Errorf("runNightly: %s: Cleanup left a worktree or branch behind", runDir)
	}

	if err := publishRecord(document.Skill, runDir, document.Version); err != nil {
		return nil, err
	}
	return verdictOutcome(document, smoke), nil
}

// smokePrompt leaves {token} for the person to fill in when the label is empty.
func smokePrompt(document *skillgraph.SmokeDocument, tokenLabel string) string {
	invoke := *document.Invoke
	if tokenLabel != "" {
		invoke = strings.ReplaceAll(invoke, smokeTokenPlaceholder, tokenLabel)
	}
	return strings.TrimSpace("/" + document.Skill + " " + invoke)
}

// versionAside renames a present record dir to <dir>.<n>, n the first free number from 1, so every smoke of a version is kept.
func versionAside(versionDir string) error {
	if _, err := os.Stat(versionDir); errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	for n := 1; ; n++ {
		aside := versionDir + "." + strconv.Itoa(n)
		if _, err := os.Stat(aside); err == nil {
			continue
		}

		if err := os.Rename(versionDir, aside); err != nil {
			return fmt.Errorf("versionAside: %w", err)
		}
		return nil
	}
}

func verdictOutcome(document *skillgraph.SmokeDocument, smoke *SmokeResult) *SmokeOutcome {
	head := fmt.Sprintf("smoke %s %s: %s", document.Skill, document.Version, smoke.Verdict)
	if smoke.Cause != "" {
		head += " — " + smoke.Cause
	}

	outcome := &SmokeOutcome{IsPass: smoke.Verdict == statusPass, Lines: []string{head}}
	if hasAnchorExpectation(document) {
		outcome.Lines = append(outcome.Lines, "coverage: "+smoke.Coverage.Summary())
	}
	return outcome
}

func writeSessionRecord(captured *ArmRuntime, document *skillgraph.SmokeDocument, sessionId string, smoke *SmokeResult) error {
	runDir, err := newStagingDir(document.Skill)
	if err != nil {
		return err
	}

	manifest := newSmokeManifest("", document, ModeInteractive, "", "")
	if err := writeManifest(runDir, manifest); err != nil {
		return err
	}

	armDir := filepath.Join(runDir, runsDir, smokeArmLabel)
	if err := writeJsonFile(filepath.Join(armDir, runtimeFile), captured); err != nil {
		return err
	}

	result := &ArmResult{
		EndedAt:      time.Now(),
		Layer:        manifest.Layer,
		Mode:         manifest.Mode,
		SessionId:    sessionId,
		Skill:        manifest.Skill,
		SkillVersion: manifest.SkillVersion,
		Smoke:        smoke,
	}
	if err := writeJsonFile(filepath.Join(armDir, resultFile), result); err != nil {
		return err
	}
	return publishRecord(document.Skill, runDir, document.Version)
}

// JudgeSmokeSession lets the run row alone decide, since an interactive smoke has no stream.
func JudgeSmokeSession(ctx context.Context, leaf string, readRuntime RuntimeReader, sessionId string) (*SmokeOutcome, error) {
	_, document, err := loadSmoke(ctx, leaf)
	if err != nil {
		return nil, err
	}

	if !hasAnchorExpectation(document) {
		return nil, fmt.Errorf("JudgeSmokeSession: Skill %s expects no anchors — only a heartbeat smoke is judged from a session", leaf)
	}

	enforce, err := loadDeployedEnforce(document)
	if err != nil {
		return nil, err
	}

	captured := captureRuntime(ctx, readRuntime, sessionId)
	run, missing := skillRun(captured, leaf)
	smoke := judgeSession(document, enforce, missing, run)
	if err := writeSessionRecord(captured, document, sessionId, smoke); err != nil {
		return nil, err
	}
	return verdictOutcome(document, smoke), nil
}

func LastSmoke(root, skill string) (*ArmResult, error) {
	paths, err := filepath.Glob(filepath.Join(root, skill, "*", runsDir, smokeArmLabel, resultFile))
	if err != nil {
		return nil, fmt.Errorf("LastSmoke: %w", err)
	}

	var newest *ArmResult
	for _, path := range paths {
		var record ArmResult
		if err := readJsonFile(path, &record); err != nil {
			return nil, err
		}
		if newest == nil || record.EndedAt.After(newest.EndedAt) {
			newest = &record
		}
	}
	return newest, nil
}

// LoadSmokeRecord reads the record of one skill version; a record without a runtime capture leaves Runtime nil.
func LoadSmokeRecord(root, skill, version string) (*SmokeRecord, error) {
	dir := filepath.Join(root, skill, version)
	var result ArmResult
	if err := readJsonFile(filepath.Join(dir, runsDir, smokeArmLabel, resultFile), &result); err != nil {
		return nil, fmt.Errorf("LoadSmokeRecord: %w", err)
	}

	var manifest RunManifest
	if err := readJsonFile(filepath.Join(dir, manifestFile), &manifest); err != nil {
		return nil, fmt.Errorf("LoadSmokeRecord: %w", err)
	}

	captured, err := loadRuntime(filepath.Join(dir, runsDir, smokeArmLabel, runtimeFile))
	if err != nil {
		return nil, fmt.Errorf("LoadSmokeRecord: %w", err)
	}

	record := &SmokeRecord{Dir: dir, Manifest: &manifest, Result: &result, Runtime: captured}
	return record, nil
}

// RunSmoke refuses a deployed copy that differs from the checkout's render.
func RunSmoke(ctx context.Context, leaf string, readRuntime RuntimeReader, tokenLabel string) (*SmokeOutcome, error) {
	skillDir, document, err := loadSmoke(ctx, leaf)
	if err != nil {
		return nil, err
	}

	if document.Invoke == nil {
		line := fmt.Sprintf("smoke %s %s: skipped — %s", leaf, document.Version, document.Skip)
		outcome := &SmokeOutcome{IsPass: true, Lines: []string{line}}
		return outcome, nil
	}

	if err := checkDeployed(skillDir); err != nil {
		return nil, err
	}

	switch document.Mode {
	case skillgraph.SmokeModeInteractive:
		return interactiveOutcome(document, tokenLabel), nil
	case skillgraph.SmokeModeNightly:
		return runNightly(ctx, document, readRuntime, skillDir, tokenLabel)
	}
	return nil, fmt.Errorf("RunSmoke: Skill %s: Unknown smoke mode %q", leaf, document.Mode)
}

// SmokeRoot is the smoke record cache, ~/.cache/claude-routine/artifacts/smoke.
func SmokeRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("SmokeRoot: %w", err)
	}
	return filepath.Join(home, smokeRootDir), nil
}

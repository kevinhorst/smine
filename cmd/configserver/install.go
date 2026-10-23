package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/skills"
)

// explicitFlags names the flags the caller set, so a plain re-run keeps the stored ports.
type installRequest struct {
	addr                  string
	componentsArg         string
	credentialPath        string
	explicitFlags         map[string]bool
	heartbeatPort         int
	initWelcome           bool
	isClaudeFound         bool
	isPreview             bool
	isSkillsConfirmed     bool
	peekControlPort       int
	peekPort              int
	presentationProfileId string
	skillsArg             string
}

// buildRecord starts from the stored record, Full when absent.
func buildRecord(env install.Env, request *installRequest) (install.Record, error) {
	record, err := install.LoadCurrent(env)
	if err != nil {
		return record, fmt.Errorf("buildRecord: %w", err)
	}

	if request.componentsArg != "" {
		preset, ids, err := parseComponents(request.componentsArg, env.GOOS)
		if err != nil {
			return record, fmt.Errorf("buildRecord: %w", err)
		}
		record.Preset = preset
		record.Components = ids
	}

	if request.skillsArg != "" {
		selection, err := parseSkills(request.skillsArg)
		if err != nil {
			return record, fmt.Errorf("buildRecord: %w", err)
		}
		// mode and chosen only: the record's variables, providers and confirmation stay
		record.Skills.Chosen = selection.Chosen
		record.Skills.Mode = selection.Mode
	}
	if request.explicitFlags["skills-confirmed"] {
		record.Skills.IsConfirmed = request.isSkillsConfirmed
	}
	return record, nil
}

// installPresentationProfile copies the selected repo template to the env's profile path only when none exists, recording the digest; an empty id installs nothing (plan D11, master A33).
func installPresentationProfile(env install.Env, profileId string) error {
	if profileId == "" {
		return nil
	}

	target := filepath.Join(env.ClaudeHome, "context", "global", "presentation-profile.md")
	_, err := os.Stat(target)
	if err == nil {
		fmt.Fprintf(env.Stdout, "kept: %s (exists)\n", target)
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("installPresentationProfile: %w", err)
	}

	source := filepath.Join(env.RepoDir, "settings", "claude_code", "presentation-profile."+profileId+".md")
	content, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("installPresentationProfile: template %q: %w", profileId, err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("installPresentationProfile: %w", err)
	}
	if err := fsx.WriteFileAtomic(content, target, 0o644); err != nil {
		return fmt.Errorf("installPresentationProfile: %w", err)
	}
	if err := env.Trash.RecordOwned(target, content); err != nil {
		return fmt.Errorf("installPresentationProfile: %w", err)
	}
	fmt.Fprintf(env.Stdout, "-> Presentation profile installed: %s (%s)\n", target, profileId)
	return nil
}

// parseComponents reads "minimal", "full" or an id list; a list gains the fixed config server and every requirement (master C6).
func parseComponents(value, goos string) (string, []components.ID, error) {
	switch value {
	case install.PresetMinimal:
		return value, components.Minimal(), nil
	case install.PresetFull:
		return value, components.Full(goos), nil
	}

	ids := []components.ID{components.ConfigServer}
	for name := range strings.SplitSeq(value, ",") {
		id := components.ID(strings.TrimSpace(name))
		if _, ok := components.Lookup(id); !ok {
			return "", nil, fmt.Errorf("parseComponents: Unknown component %q", id)
		}
		ids = append(ids, id)
	}
	return install.PresetCustom, components.Close(ids), nil
}

// parseSkills reads all | none | chosen:<leaf>,<leaf>.
func parseSkills(value string) (install.Selection, error) {
	mode, leaves, _ := strings.Cut(value, ":")
	selection := install.Selection{Mode: mode}
	if leaves != "" {
		selection.Chosen = strings.Split(leaves, ",")
	}

	switch mode {
	case skills.SelectionModeAll, skills.SelectionModeChosen, skills.SelectionModeNone:
		return selection, nil
	default:
		return selection, fmt.Errorf("parseSkills: Unknown skills mode %q", mode)
	}
}

// printPlan is -preview: the steps, or "no steps"; nothing is written.
func printPlan(ctx context.Context, env install.Env, record install.Record) int {
	steps, err := install.Plan(ctx, env, record)
	if err != nil {
		fmt.Fprintf(os.Stderr, "install: %v\n", err)
		return 1
	}
	if len(steps) == 0 {
		fmt.Println("no steps")
		return 0
	}
	for _, step := range steps {
		fmt.Printf("%s %s %s\n", step.Component, step.Action, step.Detail)
	}
	return 0
}

// runApply applies the stored record as is, replacing the retired sync scripts.
func runApply(ctx context.Context) int {
	env, record, err := storedRecord()
	if err != nil {
		fmt.Fprintf(os.Stderr, "apply: %v\n", err)
		return 1
	}

	results, err := install.Apply(ctx, env, record)
	fmt.Print(install.Summary(results))
	if err == nil {
		err = restartServer(ctx, env, results)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "apply: %v\n", err)
		return 1
	}
	if install.Failed(results) {
		return 1
	}
	return 0
}

// runInstall is the one install flow of every OS, so platform differences stay in prepare and finish.
func runInstall(ctx context.Context, request *installRequest) int {
	repoDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "install: getwd: %v\n", err)
		return 1
	}

	env, err := install.NewEnv(repoDir, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "install: %v\n", err)
		return 1
	}

	record, err := buildRecord(env, request)
	if err == nil {
		err = install.Validate(record, env.GOOS)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "install: %v\n", err)
		return 1
	}
	if request.isPreview {
		return printPlan(ctx, env, record)
	}

	if err := ensureCredential(request.credentialPath); err != nil {
		fmt.Fprintf(os.Stderr, "install: %v\n", err)
		return 1
	}
	// before prepareInstall, which on Windows relaunches elevated: a repository the elevated process creates is owned by Administrators, and the user's git then refuses it as dubious
	if err := setupGit(ctx, env); err != nil {
		fmt.Fprintf(os.Stderr, "install: %v\n", err)
		return 1
	}
	if code, done := prepareInstall(ctx, repoDir, record, request); done {
		return code
	}
	if err := runMigrateState(ctx, repoDir, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "install: %v\n", err)
		return 1
	}

	env, err = saveInstallValues(env, &record, request)
	if err != nil {
		fmt.Fprintf(os.Stderr, "install: %v\n", err)
		return 1
	}

	results, err := install.Apply(ctx, env, record)
	fmt.Print(install.Summary(results))
	if err == nil {
		err = installPresentationProfile(env, request.presentationProfileId)
	}
	if err == nil {
		err = finishInstall(ctx, env, record, request, results)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "install: %v\n", err)
		return 1
	}
	if install.Failed(results) {
		return 1
	}
	return 0
}

// saveInstallValues writes only the explicitly passed flags into the record's install rows.
func saveInstallValues(env install.Env, record *install.Record, request *installRequest) (install.Env, error) {
	set := func(id components.ID, key, text string) error {
		setting, _ := components.LookupSetting(id, key)
		value, err := components.ParseText(&setting, text)
		if err != nil {
			return fmt.Errorf("saveInstallValues: %w", err)
		}
		return install.SetOverride(record, id, key, value)
	}
	if request.explicitFlags["addr"] {
		_, port, err := net.SplitHostPort(request.addr)
		if err != nil {
			return env, fmt.Errorf("saveInstallValues: Invalid addr %q: %w", request.addr, err)
		}
		if err := set(components.ConfigServer, "port", port); err != nil {
			return env, err
		}
	}
	if request.explicitFlags["init-welcome"] {
		if err := set(components.ConfigServer, "init_welcome", strconv.FormatBool(request.initWelcome)); err != nil {
			return env, err
		}
	}
	if request.explicitFlags["peek-port"] {
		if err := set(components.Peek, "port", strconv.Itoa(request.peekPort)); err != nil {
			return env, err
		}
	}
	if request.explicitFlags["peek-control-port"] {
		if err := set(components.Peek, "control_port", strconv.Itoa(request.peekControlPort)); err != nil {
			return env, err
		}
	}
	if request.explicitFlags["heartbeat-port"] {
		if err := set(components.Heartbeat, "port", strconv.Itoa(request.heartbeatPort)); err != nil {
			return env, err
		}
	}

	rows := install.SkillRows(*record)
	orphans, err := skills.OrphanRows(env.RepoDir, rows)
	if err != nil {
		return env, fmt.Errorf("saveInstallValues: %w", err)
	}
	for _, key := range orphans {
		fmt.Fprintf(env.Stdout, "orphan row kept: %s\n", key)
	}
	install.SetSkillRows(record, rows)

	values, err := install.InstallValues(*record)
	if err != nil {
		return env, fmt.Errorf("saveInstallValues: %w", err)
	}
	env.Values = values
	return env, nil
}

// storedRecord is the checkout's env and its stored record.
func storedRecord() (install.Env, install.Record, error) {
	repoDir, err := os.Getwd()
	if err != nil {
		return install.Env{}, install.Record{}, fmt.Errorf("storedRecord: %w", err)
	}

	env, err := install.NewEnv(repoDir, os.Stdout)
	if err != nil {
		return env, install.Record{}, fmt.Errorf("storedRecord: %w", err)
	}

	record, err := install.LoadCurrent(env)
	if err != nil {
		return env, record, fmt.Errorf("storedRecord: %w", err)
	}
	return env, record, nil
}

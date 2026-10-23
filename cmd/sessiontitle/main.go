// Command sessiontitle keeps the Desktop session title in the shape "<RepoId>,<SkillId> - <AI title> - <FleetId>[STATE]".
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/desktop"
	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/sessionstate"
	"github.com/kevinhorst/smine/internal/sessiontitle"
	"github.com/kevinhorst/smine/internal/shell"
	"github.com/kevinhorst/smine/internal/skills"
)

// Environment is the Desktop identity the harness exports to hooks.
type Environment struct {
	EntryPoint    string
	HostSessionId string
	IsAttended    bool
}

func (e *Environment) isActive() bool {
	isDesktop := e.EntryPoint == entryPointDesktop && desktop.IsHostSessionId(e.HostSessionId)
	return isDesktop && e.IsAttended
}

// Hook holds what every event decision reads.
type Hook struct {
	desktopSessionsDir string
	hostSessionId      string
	limits             *sessiontitle.Limits
	record             *SessionRecord
	recordsDir         string
	registry           *repos.Registry
	section            *hookconfig.Sessiontitle
	skillsDir          string
	triggers           *Triggers
	vocabulary         *sessiontitle.Vocabulary
}

// Arm creates the record once and clears the pending state on every later start but a compaction, which keeps the turn open; named survives a resume.
func (h *Hook) Arm(sessionId string, source string) error {
	path, err := h.recordPath(sessionId)
	if err != nil {
		return err
	}

	record, err := loadRecord(path)
	if err != nil {
		return fmt.Errorf("Hook.Arm: %w", err)
	}
	if record == nil {
		record = &SessionRecord{}
	}
	if source != sourceCompact {
		record.Turn = Turn{}
	}
	return saveRecord(path, record)
}

func (h *Hook) DecideFirst(ctx context.Context, input *HookInput) (*HookOutput, error) {
	target, isPending, err := h.firstTitle(ctx, input)
	if err != nil {
		return nil, err
	}
	if isPending {
		return nil, nil
	}
	h.record.Named = true
	if target == "" {
		return nil, nil
	}
	return h.instruct(eventPostToolUse, target)
}

func (h *Hook) DecideJoin(ctx context.Context, input *HookInput) (*HookOutput, error) {
	token := strings.TrimSpace(input.ToolResponse.Stdout)
	if !h.triggers.memberToken.MatchString(token) {
		return nil, nil
	}
	h.record.Fleet = token
	return h.decide(ctx, input, sessiontitle.State.AfterJoin)
}

// DecideSet records a confirmed `smine-rules title set` call; the turn's first set renames at the call, every later one waits for the Stop, where the last wins.
func (h *Hook) DecideSet(ctx context.Context, input *HookInput, state sessiontitle.State) (*HookOutput, error) {
	h.record.Turn.State = state
	if h.record.Turn.IsDelivered {
		return nil, nil
	}
	h.record.Turn.IsDelivered = true
	return h.decide(ctx, input, func(current sessiontitle.State) sessiontitle.State { return current.AfterSet(state) })
}

// DecideStop spends the pending state before deciding, so the re-Stop after the rename finds nothing.
func (h *Hook) DecideStop(ctx context.Context, input *HookInput) (*HookOutput, error) {
	target := h.record.Turn.State
	h.record.Turn = Turn{}
	if target == sessiontitle.StateNone {
		return nil, nil
	}
	return h.decide(ctx, input, func(state sessiontitle.State) sessiontitle.State { return state.AfterSet(target) })
}

func (h *Hook) DecideSubscribe(ctx context.Context, input *HookInput) (*HookOutput, error) {
	match := h.triggers.subscribeFleet.FindStringSubmatch(input.ToolInput.Command)
	if len(match) <= 1 {
		return nil, nil
	}
	h.record.Fleet = match[1] + "-O"
	return h.decide(ctx, input, sessiontitle.State.AfterJoin)
}

// DecideTool runs the first rename while it is owed, then the Bash call's own handling; the first rename's instruction wins the output.
func (h *Hook) DecideTool(ctx context.Context, input *HookInput) (*HookOutput, error) {
	var first *HookOutput
	if !h.record.Named {
		output, err := h.DecideFirst(ctx, input)
		if err != nil {
			return nil, err
		}
		first = output
	}
	if input.ToolName != toolBash {
		return first, nil
	}

	output, err := h.decideBashCall(ctx, input)
	if err != nil {
		return nil, err
	}
	if first != nil {
		return first, nil
	}
	return output, nil
}

func (h *Hook) applySavedFleet(title *sessiontitle.Title) {
	if h.record.Fleet != "" {
		title.Fleet = h.record.Fleet
	}
}

// confirmedSet is the state of a `smine-rules title set` call whose printed line confirms a settable token.
func (h *Hook) confirmedSet(input *HookInput) (sessiontitle.State, bool) {
	match := h.triggers.titleSet.FindStringSubmatch(input.ToolInput.Command)
	if len(match) <= titleSetTokenGroup {
		return sessiontitle.StateNone, false
	}

	state := sessiontitle.State(match[titleSetTokenGroup])
	isConfirmed := strings.TrimSpace(input.ToolResponse.Stdout) == sessiontitle.SetOutput(state)
	if !isConfirmed || !h.vocabulary.IsSettable(state) {
		return sessiontitle.StateNone, false
	}
	return state, true
}

func (h *Hook) decide(ctx context.Context, input *HookInput, transition func(sessiontitle.State) sessiontitle.State) (*HookOutput, error) {
	session, err := desktop.ReadSession(h.hostSessionId, h.desktopSessionsDir)
	if err != nil {
		return nil, err
	}
	title := sessiontitle.ParseTitle(session.Title, h.vocabulary)
	if title.IsAwaitingAi() {
		return nil, nil
	}
	h.applySavedFleet(title)

	candidates, _, err := promptSkills(input.TranscriptPath)
	if err != nil {
		return nil, err
	}
	if err := h.fillIds(ctx, candidates, input.Cwd, title); err != nil {
		return nil, err
	}
	title.State = transition(title.State)
	target := title.Render(h.limits)
	if target == session.Title {
		return nil, nil
	}
	return h.instruct(input.HookEventName, target)
}

func (h *Hook) decideBashCall(ctx context.Context, input *HookInput) (*HookOutput, error) {
	command := input.ToolInput.Command
	switch {
	case h.triggers.closeCheck.MatchString(command):
		return h.decide(ctx, input, sessiontitle.State.AfterClose)
	case h.triggers.done.MatchString(command):
		return h.decide(ctx, input, sessiontitle.State.AfterDone)
	case h.triggers.join.MatchString(command):
		return h.DecideJoin(ctx, input)
	case h.triggers.subscribe.MatchString(command):
		return h.DecideSubscribe(ctx, input)
	}

	state, isSet := h.confirmedSet(input)
	if !isSet {
		return nil, nil
	}
	return h.DecideSet(ctx, input, state)
}

func (h *Hook) decideRecorded(ctx context.Context, input *HookInput) (*HookOutput, error) {
	switch input.HookEventName {
	case eventPostToolUse:
		return h.DecideTool(ctx, input)
	case eventStop:
		return h.DecideStop(ctx, input)
	}
	return nil, nil
}

func (h *Hook) decideRename(input *HookInput) *HookOutput {
	isSelf := input.ToolInput.SessionId == renameSelf
	isFormatted := sessiontitle.ParseTitle(input.ToolInput.Title, h.vocabulary).State != sessiontitle.StateNone
	if !isSelf || !isFormatted {
		return nil
	}

	output := &HookOutput{
		HookSpecificOutput: &HookSpecificOutput{
			HookEventName:            eventPreToolUse,
			PermissionDecision:       permissionAllow,
			PermissionDecisionReason: h.section.RenameAllowReason,
		},
	}
	return output
}

// Built-in commands and plugin skills never name a title.
func (h *Hook) deployedSkill(candidates []string) string {
	for _, name := range candidates {
		if _, err := os.Stat(filepath.Join(h.skillsDir, name, "SKILL.md")); err == nil {
			return name
		}
	}
	return ""
}

func (h *Hook) fillIds(ctx context.Context, candidates []string, cwd string, title *sessiontitle.Title) error {
	if title.Repo == "" {
		title.Repo = h.repoId(ctx, cwd)
	}
	if title.Skill != "" {
		return nil
	}

	skill := h.deployedSkill(candidates)
	if skill == "" {
		return nil
	}
	var err error
	title.Skill, err = h.skillId(skill)
	return err
}

func (h *Hook) firstTitle(ctx context.Context, input *HookInput) (string, bool, error) {
	session, err := desktop.ReadSession(h.hostSessionId, h.desktopSessionsDir)
	if err != nil {
		return "", false, err
	}
	title := sessiontitle.ParseTitle(session.Title, h.vocabulary)
	if title.IsAwaitingAi() {
		return "", true, nil
	}
	if title.State != sessiontitle.StateNone {
		return "", false, nil
	}
	h.applySavedFleet(title)

	candidates, hasPrompt, err := promptSkills(input.TranscriptPath)
	if err != nil {
		return "", false, err
	}
	if !hasPrompt {
		return "", true, nil
	}
	if err := h.fillIds(ctx, candidates, input.Cwd, title); err != nil {
		return "", false, err
	}
	title.State = sessiontitle.StateWip
	return title.Render(h.limits), false, nil
}

func (h *Hook) instruct(event string, target string) (*HookOutput, error) {
	if event == eventStop {
		block := &HookOutput{Decision: decisionBlock, Reason: h.renameInstruction(target)}
		return block, nil
	}

	output := &HookOutput{
		HookSpecificOutput: &HookSpecificOutput{
			AdditionalContext: h.renameInstruction(target),
			HookEventName:     event,
		},
	}
	return output, nil
}

func (h *Hook) recordPath(sessionId string) (string, error) {
	if !sessionIdPattern.MatchString(sessionId) {
		return "", fmt.Errorf("Hook.recordPath: Invalid session id %q", sessionId)
	}
	return filepath.Join(h.recordsDir, sessionId+".json"), nil
}

func (h *Hook) renameInstruction(target string) string {
	quoted, _ := json.Marshal(target)
	return fmt.Sprintf(renameInstructionFormat, quoted) + h.section.InstructionPlanMode + h.section.InstructionTail
}

func (h *Hook) repoId(ctx context.Context, cwd string) string {
	checkout := cwd
	output, err := shell.Run(ctx, cwd, "git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err == nil {
		checkout = filepath.Dir(strings.TrimSpace(output))
	}

	for _, repo := range h.registry.Repos() {
		if filepath.Clean(repo.Path) != checkout {
			continue
		}
		if repo.Label != "" {
			return repo.Label
		}
		return repo.Name
	}
	return filepath.Base(checkout)
}

// runRecorded loads, decides and writes the record back once under the record's lock; no record means a session the hook never armed.
func (h *Hook) runRecorded(ctx context.Context, input *HookInput, stderr io.Writer, stdout io.Writer) int {
	path, err := h.recordPath(input.SessionId)
	if err != nil {
		return writeOutput(nil, err, stderr, stdout)
	}

	var output *HookOutput
	err = fsx.WithFileLock(path, func() error {
		record, err := loadRecord(path)
		if err != nil || record == nil {
			return err
		}

		h.record = record
		output, err = h.decideRecorded(ctx, input)
		if err != nil {
			return err
		}
		return saveRecord(path, h.record)
	})
	return writeOutput(output, err, stderr, stdout)
}

// skillId is the skill's title-code, else its name.
func (h *Hook) skillId(name string) (string, error) {
	skillMd, err := os.ReadFile(filepath.Join(h.skillsDir, name, "SKILL.md"))
	if err != nil {
		return "", fmt.Errorf("Hook.skillId: Failed to read SKILL.md for %s: %w", name, err)
	}
	if code := skills.TitleCode(string(skillMd)); code != "" {
		return code, nil
	}
	return name, nil
}

// HookInput is the union of every event's envelope; fields an event lacks stay zero.
type HookInput struct {
	AgentId        string       `json:"agent_id"`
	Cwd            string       `json:"cwd"`
	HookEventName  string       `json:"hook_event_name"`
	SessionId      string       `json:"session_id"`
	Source         string       `json:"source"`
	ToolInput      ToolInput    `json:"tool_input"`
	ToolName       string       `json:"tool_name"`
	ToolResponse   ToolResponse `json:"tool_response"`
	TranscriptPath string       `json:"transcript_path"`
}

type HookOutput struct {
	Decision           string              `json:"decision,omitempty"`
	HookSpecificOutput *HookSpecificOutput `json:"hookSpecificOutput,omitempty"`
	Reason             string              `json:"reason,omitempty"`
}

type HookSpecificOutput struct {
	AdditionalContext        string `json:"additionalContext,omitempty"`
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision,omitempty"`
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
}

type ToolInput struct {
	Command   string `json:"command"`
	SessionId string `json:"session_id"`
	Title     string `json:"title"`
}

// ToolResponse is the slice of a Bash result the join and set triggers read.
type ToolResponse struct {
	Stdout string `json:"stdout"`
}

// isOff reads the session's switch record; an unreadable record is logged and counts as on.
func isOff(claudeHome string, sessionId string, feature sessionstate.Feature) bool {
	isOff, err := sessionstate.IsOff(sessionstate.File(sessionstate.StateDir(claudeHome), sessionId), feature)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sessiontitle: isOff: %v\n", err)
	}
	return isOff
}

func main() {
	environment := &Environment{
		EntryPoint:    os.Getenv(envEntryPoint),
		HostSessionId: os.Getenv(envHostSessionId),
		IsAttended:    os.Getenv(envSessionAttended) == attendedValue,
	}
	os.Exit(run(context.Background(), os.Args[1:], environment, os.Stderr, os.Stdin, os.Stdout))
}

func run(ctx context.Context, args []string, environment *Environment, stderr io.Writer, stdin io.Reader, stdout io.Writer) int {
	stateDir, err := config.StateDir("")
	if err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return 1
	}

	flags := flag.NewFlagSet("sessiontitle", flag.ContinueOnError)
	flags.SetOutput(stderr)
	reposPath := flags.String("repos", filepath.Join(stateDir, config.StateRegistryFile), "repo registry")
	skillsDir := flags.String("skills", "", "deployed skills directory")
	if err := flags.Parse(args); err != nil {
		return 1
	}
	if !environment.isActive() {
		return 0
	}

	input := &HookInput{}
	if err := json.NewDecoder(stdin).Decode(input); err != nil {
		fmt.Fprintf(stderr, "run: Failed to decode hook input: %v\n", err)
		return 1
	}
	if input.AgentId != "" {
		return 0
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		fmt.Fprintf(stderr, "run: Failed to resolve user config dir: %v\n", err)
		return 1
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(stderr, "run: Failed to resolve home: %v\n", err)
		return 1
	}
	claudeHome := filepath.Join(home, ".claude")
	cfg, err := hookconfig.Load(claudeHome)
	if err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return 1
	}
	triggers, err := compileTriggers(cfg.Sessiontitle.TriggerPatterns)
	if err != nil {
		fmt.Fprintf(stderr, "run: %s: %v\n", hookconfig.Path(claudeHome), err)
		return 1
	}
	registry := repos.NewRegistry(*reposPath)
	if err := registry.Reload(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	vocabulary, err := sessiontitle.LoadVocabulary(*skillsDir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	limits := &sessiontitle.Limits{
		MaxStates:     cfg.Sessiontitle.MaxStates,
		MaxTitleRunes: cfg.Sessiontitle.MaxTitleRunes,
		MaxTokenRunes: cfg.Sessiontitle.MaxTokenRunes,
	}
	hook := &Hook{
		desktopSessionsDir: desktop.SessionStoreDir(configDir),
		hostSessionId:      environment.HostSessionId,
		limits:             limits,
		recordsDir:         filepath.Join(claudeHome, "sessiontitle", "sessions"),
		registry:           registry,
		section:            &cfg.Sessiontitle,
		skillsDir:          *skillsDir,
		triggers:           triggers,
		vocabulary:         vocabulary,
	}

	switch input.HookEventName {
	case eventSessionStart:
		if err := hook.Arm(input.SessionId, input.Source); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	case eventPreToolUse:
		return writeOutput(hook.decideRename(input), nil, stderr, stdout)
	case eventPostToolUse, eventStop:
		if isOff(claudeHome, input.SessionId, sessionstate.SessionTitle) {
			return 0
		}
		return hook.runRecorded(ctx, input, stderr, stdout)
	}
	return 0
}

func writeOutput(output *HookOutput, err error, stderr io.Writer, stdout io.Writer) int {
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if output == nil {
		return 0
	}
	if err := json.NewEncoder(stdout).Encode(output); err != nil {
		fmt.Fprintf(stderr, "writeOutput: Failed to encode output: %v\n", err)
		return 1
	}
	return 0
}

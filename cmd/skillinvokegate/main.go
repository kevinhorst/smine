// Command skillinvokegate is the PreToolUse skill gate of review lanes: a REQUIRE-SKILL marker forces Skill(<name>) first.
package main

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/contextrequired"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/reviewscope"
	"github.com/kevinhorst/smine/internal/sessionstate"
	"github.com/kevinhorst/smine/internal/transcript"
)

// HookDecision is the PreToolUse permission verdict.
type HookDecision struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason"`
}

type HookInput struct {
	AgentId        string    `json:"agent_id"`
	Cwd            string    `json:"cwd"`
	SessionId      string    `json:"session_id"`
	ToolInput      ToolInput `json:"tool_input"`
	ToolName       string    `json:"tool_name"`
	TranscriptPath string    `json:"transcript_path"`
}

func (i *HookInput) isContextRequiredRead(claudeHome string) bool {
	if i.ToolName != toolRead || i.SessionId == "" {
		return false
	}

	path := i.ToolInput.FilePath
	if hasDotSegment(path) {
		return false
	}
	return strings.HasPrefix(path, contextrequired.Root(claudeHome)+"/"+i.SessionId+"/")
}

func (i *HookInput) readTarget(shellMetaChars string) string {
	switch i.ToolName {
	case toolRead:
		return i.ToolInput.FilePath
	case toolBash:
		return catTarget(i.ToolInput.Command, shellMetaChars)
	}
	return ""
}

func (i *HookInput) transcriptPath() string {
	isSubagent := i.AgentId != "" && i.SessionId != ""
	if !isSubagent || i.TranscriptPath == "" {
		return i.TranscriptPath
	}

	dir := filepath.Dir(i.TranscriptPath) + "/" + i.SessionId + "/subagents"
	return subagentTranscript(i.AgentId, dir)
}

// HookOutput wraps the deny verdict; an allow writes nothing.
type HookOutput struct {
	HookSpecificOutput HookDecision `json:"hookSpecificOutput"`
}

type Session struct {
	hasAssistant  bool
	invokedSkills []string
	promptLines   []string
}

func (s *Session) hasInvoked(skill string) bool {
	if strings.HasPrefix(s.prompt(), "/"+skill) {
		return true
	}
	return slices.Contains(s.invokedSkills, skill)
}

// manifests lists the lane manifests the prompt names.
func (s *Session) manifests() []string {
	var paths []string
	for _, match := range manifestPattern.FindAllStringSubmatch(s.prompt(), -1) {
		paths = append(paths, match[1])
	}
	return paths
}

func (s *Session) prompt() string {
	return strings.Join(s.promptLines, "\n")
}

// record folds one transcript line into the session.
func (s *Session) record(line *transcript.Line) {
	switch line.Type {
	case transcript.TypeAssistant:
		s.hasAssistant = true
		s.invokedSkills = append(s.invokedSkills, skillCalls(&line.Message.Content)...)
	case transcript.TypeUser:
		if !s.hasAssistant {
			s.promptLines = append(s.promptLines, promptText(&line.Message.Content))
		}
	}
}

// requiredSkill is the skill the prompt's marker names, empty without one.
func (s *Session) requiredSkill() string {
	match := requiredPattern.FindStringSubmatch(s.prompt())
	if match == nil {
		return ""
	}
	return match[1]
}

// ToolInput holds the tool arguments the gate reads.
type ToolInput struct {
	Command  string `json:"command"`
	FilePath string `json:"file_path"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stderr, os.Stdin, os.Stdout))
}

func catTarget(command string, shellMetaChars string) string {
	if strings.ContainsAny(command, shellMetaChars) {
		return ""
	}

	words := strings.FieldsFunc(command, isWordBreak)
	if len(words) != 2 || words[0] != catCommand {
		return ""
	}
	return words[1]
}

func contextRel(contextDir string, contextName string, target string) string {
	if rel, isInside := strings.CutPrefix(target, contextDir+"/"); isInside {
		return rel
	}

	segment := "/" + contextName + "/"
	index := strings.LastIndex(target, segment)
	if index < 0 {
		return ""
	}
	return target[index+len(segment):]
}

// decide returns the deny reason for a call of a session whose prompt requires a skill, empty when it passes.
func decide(cfg *hookconfig.Config, claudeHome string, input *HookInput, session *Session) string {
	required := session.requiredSkill()
	if !session.hasInvoked(required) {
		return reasonBeforeInvocation(cfg, claudeHome, input, required)
	}
	return reasonAfterInvocation(cfg, input, session.manifests())
}

// gatedSession is the call's session when its prompt requires a skill; nil leaves the call to the other hooks.
func gatedSession(input *HookInput) *Session {
	path := input.transcriptPath()
	if path == "" {
		return nil
	}

	session, err := readSession(path)
	if err != nil {
		return nil
	}
	if session.requiredSkill() == "" {
		return nil
	}
	return session
}

func hasDotSegment(path string) bool {
	for _, segment := range dotSegments {
		if strings.Contains(path, segment) {
			return true
		}
	}
	return strings.HasSuffix(path, "/.") || strings.HasSuffix(path, "/..")
}

// isEntryPath reports a context entry file: an .md file of an entry chapter.
func isEntryPath(rel string, entryChapters []string) bool {
	if !strings.HasSuffix(rel, entrySuffix) {
		return false
	}

	chapter, _, hasChapter := strings.Cut(rel, "/")
	return hasChapter && slices.Contains(entryChapters, chapter)
}

func isGatedPath(contextDir string, entryChapters []string, rel string, target string) bool {
	if isEntryPath(rel, entryChapters) {
		return true
	}

	info, err := os.Stat(contextDir)
	if err != nil || !info.IsDir() {
		return false
	}
	return strings.HasPrefix(target, contextDir+"/")
}

func isGuide(indexPath string, rel string) bool {
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return false
	}

	index := &contextdocs.RuleSet{}
	if err := json.Unmarshal(data, index); err != nil {
		return false
	}

	for _, guide := range index.Guides {
		if guide.Path == rel {
			return true
		}
	}
	return false
}

func isListed(cwd string, manifestPath string, target string) bool {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return false
	}

	manifest := &reviewscope.LaneManifest{}
	if err := json.Unmarshal(data, manifest); err != nil {
		return false
	}

	base := strings.TrimSuffix(cmp.Or(manifest.Root, cwd), "/")
	if base == "" {
		return false
	}

	for _, file := range manifest.Files {
		if base+"/"+file == target {
			return true
		}
	}
	return false
}

func isManifestRead(input *HookInput, manifests []string, target string) bool {
	if input.ToolName != toolRead || !strings.HasPrefix(target, "/") {
		return false
	}

	if hasDotSegment(target) {
		return false
	}

	for _, manifestPath := range manifests {
		if isListed(input.Cwd, manifestPath, target) {
			return true
		}
	}
	return false
}

// isOff reads the session's switch record; an unreadable record is logged and counts as on.
func isOff(claudeHome string, sessionId string, feature sessionstate.Feature) bool {
	isOff, err := sessionstate.IsOff(sessionstate.File(sessionstate.StateDir(claudeHome), sessionId), feature)
	if err != nil {
		fmt.Fprintf(os.Stderr, "skillinvokegate: isOff: %v\n", err)
	}
	return isOff
}

func isWordBreak(char rune) bool {
	return strings.ContainsRune(shellWordBreaks, char)
}

func promptText(content *transcript.Content) string {
	if len(content.Blocks) == 0 {
		return content.Text
	}

	texts := make([]string, 0, len(content.Blocks))
	for _, block := range content.Blocks {
		texts = append(texts, block.Text)
	}
	return strings.Join(texts, "\n")
}

func readSession(path string) (*Session, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("readSession: Failed to open transcript: %w", err)
	}
	defer func() { _ = file.Close() }()

	session := &Session{}
	scanner := transcript.NewScanner(file)
	for scanner.Scan() {
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}

		line := &transcript.Line{}
		var mismatch *json.UnmarshalTypeError
		if err := json.Unmarshal(raw, line); err != nil && !errors.As(err, &mismatch) {
			return nil, fmt.Errorf("readSession: Failed to decode transcript line: %w", err)
		}
		session.record(line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("readSession: Failed to scan transcript: %w", err)
	}
	return session, nil
}

// reasonAfterInvocation denies a direct read of an injected context entry.
func reasonAfterInvocation(cfg *hookconfig.Config, input *HookInput, manifests []string) string {
	section := &cfg.Skillinvokegate
	target := input.readTarget(section.ShellMetaChars)
	if target == "" {
		return ""
	}

	contextName := cfg.Context.ContextDir
	if contextName == "" {
		return ""
	}
	contextDir := input.Cwd + "/" + contextName
	rel := contextRel(contextDir, contextName, target)
	if rel == "" || !isGatedPath(contextDir, section.EntryChapters, rel, target) {
		return ""
	}

	defaultIndex := contextDir + "/" + contextdocs.ContextFileName
	indexPath := cmp.Or(os.Getenv(contextIndexKey), defaultIndex)
	if isGuide(indexPath, rel) || isManifestRead(input, manifests, target) {
		return ""
	}
	return fmt.Sprintf(section.DenyTexts[0], target)
}

func reasonBeforeInvocation(cfg *hookconfig.Config, claudeHome string, input *HookInput, required string) string {
	if input.ToolName == toolSkill || input.isContextRequiredRead(claudeHome) {
		return ""
	}
	return fmt.Sprintf(cfg.Skillinvokegate.DenyTexts[1], required)
}

func run(args []string, stderr io.Writer, stdin io.Reader, stdout io.Writer) int {
	flags := flag.NewFlagSet("skillinvokegate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	claudeHome := flags.String("claude-home", "", "Claude Code home directory (default ~/.claude)")
	if err := flags.Parse(args); err != nil {
		return usageExitCode
	}

	home, err := config.ClaudeHomeDir(*claudeHome)
	if err != nil {
		fmt.Fprintf(stderr, "run: %v\n", err)
		return 1
	}

	input := &HookInput{}
	if err := json.NewDecoder(stdin).Decode(input); err != nil {
		fmt.Fprintf(stderr, "run: Failed to decode hook input: %v\n", err)
		return 1
	}
	if isOff(home, input.SessionId, sessionstate.SkillInvokegate) {
		return 0
	}

	session := gatedSession(input)
	if session == nil {
		return 0
	}

	cfg, err := loadConfig(home)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return writeDeny(err.Error(), stderr, stdout)
	}

	reason := decide(cfg, home, input, session)
	if reason == "" {
		return 0
	}
	return writeDeny(reason, stderr, stdout)
}

// loadConfig reads the hooks config and checks the section's two deny texts the gate indexes.
func loadConfig(claudeHome string) (*hookconfig.Config, error) {
	cfg, err := hookconfig.Load(claudeHome)
	if err != nil {
		return nil, fmt.Errorf("loadConfig: %w", err)
	}
	if len(cfg.Skillinvokegate.DenyTexts) != denyTextCount {
		return nil, fmt.Errorf(
			"loadConfig: %s: skillinvokegate.deny_texts holds %d texts, the gate needs %d (setting gate.invoke_deny_texts)",
			hookconfig.Path(claudeHome),
			len(cfg.Skillinvokegate.DenyTexts),
			denyTextCount,
		)
	}
	return cfg, nil
}

func writeDeny(reason string, stderr io.Writer, stdout io.Writer) int {
	output := &HookOutput{
		HookSpecificOutput: HookDecision{
			HookEventName:            eventPreToolUse,
			PermissionDecision:       decisionDeny,
			PermissionDecisionReason: reason,
		},
	}
	if err := json.NewEncoder(stdout).Encode(output); err != nil {
		fmt.Fprintf(stderr, "run: Failed to encode verdict: %v\n", err)
		return 1
	}
	return 0
}

func skillCalls(content *transcript.Content) []string {
	var skills []string
	for _, block := range content.Blocks {
		isSkillCall := block.Type == transcript.BlockToolUse && block.Name == toolSkill
		if isSkillCall {
			skills = append(skills, block.Input.Skill)
		}
	}
	return skills
}

func subagentTranscript(agentId string, dir string) string {
	name := "agent-" + agentId + ".jsonl"
	found := ""
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if entry.Name() == name {
			found = path
			return fs.SkipAll
		}
		return nil
	})
	return found
}

// Package transcript is the Claude Code session transcript model shared by hooks.
package transcript

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

const (
	AttachmentHookAdditionalContext = "hook_additional_context"
	AttachmentHookSuccess           = "hook_success"
	AttachmentQueuedCommand         = "queued_command"
	BlockText                       = "text"
	BlockToolResult                 = "tool_result"
	BlockToolUse                    = "tool_use"
	CommandModePrompt               = "prompt"
	OriginHuman                     = "human"
	OriginPeer                      = "peer"
	OriginScheduled                 = "scheduled"
	OriginSdk                       = "sdk"
	OriginSystem                    = "system"
	OriginTaskNotification          = "task_notification"
	TypeAssistant                   = "assistant"
	TypeAttachment                  = "attachment"
	TypeUser                        = "user"
	commandArgsClose                = "</command-args>"
	commandArgsOpen                 = "<command-args>"
	commandNameClose                = "</command-name>"
	commandNameOpen                 = "<command-name>/"
	compactionOpen                  = "This session is being continued"
	interruptedOpen                 = "[Request interrupted by user"
	lineBufferInitial               = 1 << 20
	lineBufferMax                   = 64 << 20
	localCommandOpen                = "<local-command-"
	localCommandNameOpen            = "<command-name>"
	pastedContentClose              = "</pasted_content"
	pastedContentOpen               = "<pasted_content id=\""
	slashWordWrapping               = "`\"'()[]{}*.,;:!?"
	systemReminderClose             = "</system-reminder>"
	systemReminderOpen              = "<system-reminder>"
	taskNotificationOpen            = "<task-notification>"
)

// Kind is who wrote a transcript line; the hooks trust the two human kinds and nothing else.
type Kind string

const (
	KindAssistant        Kind = "assistant"
	KindAttachment       Kind = "attachment"
	KindCompaction       Kind = "compaction"
	KindHarness          Kind = "harness"
	KindHeadless         Kind = "headless"
	KindHookFeedback     Kind = "hook-feedback"
	KindHuman            Kind = "human"
	KindHumanQueued      Kind = "human-queued"
	KindInterrupted      Kind = "interrupted"
	KindLocalCommand     Kind = "local-command"
	KindOther            Kind = "other"
	KindPeer             Kind = "peer"
	KindTaskNotification Kind = "task-notification"
	KindToolResult       Kind = "tool-result"
)

// hookFeedbackPattern opens the user row Claude Code writes from a hook's block reason ("Stop hook feedback:").
var hookFeedbackPattern = regexp.MustCompile(`^[A-Za-z]+ hook feedback:`)

// skillLeafPattern never matches a word with a second slash: that is a path.
var skillLeafPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// sliceTokenPattern is the skill-context hook's lane:/topic: token grammar.
var sliceTokenPattern = regexp.MustCompile(`(?:^|\s)((?:lane|topic):[a-z][a-z0-9-]*)`)

// Attachment is a queued command (sent while a turn ran) or a hook record of a harness attachment line.
type Attachment struct {
	CommandMode string  `json:"commandMode"`
	Content     Texts   `json:"content"`
	HookName    string  `json:"hookName"`
	HumanTurn   *bool   `json:"humanTurn"`
	IsMeta      bool    `json:"isMeta"`
	Origin      Origin  `json:"origin"`
	Prompt      Content `json:"prompt"`
	Stdout      string  `json:"stdout"`
	Type        string  `json:"type"`
}

// isHumanQueuedPrompt: a human origin and no meta mark; humanTurn, absent on older transcripts, must not read false.
func (a *Attachment) isHumanQueuedPrompt() bool {
	isQueuedPrompt := a.Type == AttachmentQueuedCommand && a.CommandMode == CommandModePrompt
	isHumanStamped := a.Origin.Kind == OriginHuman && !a.IsMeta
	isTurnDenied := a.HumanTurn != nil && !*a.HumanTurn
	return isQueuedPrompt && isHumanStamped && !isTurnDenied
}

// Block is one content block of a transcript message.
type Block struct {
	Content   Content   `json:"content"`
	Id        string    `json:"id"`
	Input     ToolInput `json:"input"`
	IsError   bool      `json:"is_error"`
	Name      string    `json:"name"`
	Text      string    `json:"text"`
	ToolUseId string    `json:"tool_use_id"`
	Type      string    `json:"type"`
}

// Content is written by the harness as a plain string or an array of blocks.
type Content struct {
	Blocks []Block
	Text   string
}

// Joined returns a tool result's text as the model received it.
func (c *Content) Joined() string {
	if c.Text != "" {
		return c.Text
	}
	var texts []string
	for _, block := range c.Blocks {
		if block.Type == BlockText {
			texts = append(texts, block.Text)
		}
	}
	return strings.Join(texts, "\n")
}

// hasToolResult: a user row carrying a tool result is the tool's, whatever text sits beside it.
func (c *Content) hasToolResult() bool {
	return slices.ContainsFunc(c.Blocks, func(block Block) bool { return block.Type == BlockToolResult })
}

// humanTexts: each text block, preamble stripped, pastes removed; empty and unclosed-preamble blocks are dropped.
func (c *Content) humanTexts() []string {
	texts := make([]string, 0)
	for _, raw := range c.texts() {
		stripped := stripHarnessPreamble(raw)
		if stripped == "" || strings.HasPrefix(stripped, systemReminderOpen) {
			continue
		}
		typed := strings.TrimSpace(StripPastedContents(stripped))
		if typed == "" {
			continue
		}
		texts = append(texts, typed)
	}
	return texts
}

// texts lists the string form or every text block, raw.
func (c *Content) texts() []string {
	if c.Text != "" {
		return []string{c.Text}
	}
	texts := make([]string, 0, len(c.Blocks))
	for _, block := range c.Blocks {
		if block.Type == BlockText {
			texts = append(texts, block.Text)
		}
	}
	return texts
}

// LeadingText strips harness preamble blocks before taking the first non-empty text.
func (c *Content) LeadingText() string {
	if c.Text != "" {
		return stripHarnessPreamble(c.Text)
	}
	for _, block := range c.Blocks {
		if block.Type != BlockText {
			continue
		}
		if text := stripHarnessPreamble(block.Text); text != "" {
			return text
		}
	}
	return ""
}

// SlashSkills reads every text block, a pasted one included, with its harness preamble stripped.
func (c *Content) SlashSkills() []string {
	texts := []string{c.Text}
	for _, block := range c.Blocks {
		if block.Type == BlockText {
			texts = append(texts, block.Text)
		}
	}

	skills := make([]string, 0)
	for _, text := range texts {
		skills = append(skills, slashWords(stripHarnessPreamble(text))...)
	}
	return skills
}

func (c *Content) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &c.Text); err != nil {
			return fmt.Errorf("Content.UnmarshalJSON: Failed to decode text content: %w", err)
		}
		return nil
	}
	if err := json.Unmarshal(data, &c.Blocks); err != nil {
		return fmt.Errorf("Content.UnmarshalJSON: Failed to decode block content: %w", err)
	}
	return nil
}

type Line struct {
	Attachment       Attachment    `json:"attachment"`
	Cwd              string        `json:"cwd"`
	GitBranch        string        `json:"gitBranch"`
	IsCompactSummary bool          `json:"isCompactSummary"`
	IsMeta           bool          `json:"isMeta"`
	Message          Message       `json:"message"`
	SessionId        string        `json:"sessionId"`
	ToolUseResult    ToolUseResult `json:"toolUseResult"`
	TurnOrigin       string        `json:"turnOrigin"`
	Type             string        `json:"type"`
}

// kindOfUser reads the harness stamps first, the text only for the stampless rows the harness writes itself.
func (l *Line) kindOfUser() Kind {
	if l.Message.Content.hasToolResult() {
		return KindToolResult
	}
	switch l.TurnOrigin {
	case OriginHuman:
		return KindHuman
	case OriginTaskNotification:
		return KindTaskNotification
	case OriginPeer:
		return KindPeer
	case OriginSdk, OriginScheduled, OriginSystem:
		return KindHeadless
	}
	if l.IsCompactSummary {
		return KindCompaction
	}

	text := strings.TrimSpace(l.Message.Content.Joined())
	if l.IsMeta {
		if hookFeedbackPattern.MatchString(text) {
			return KindHookFeedback
		}
		return KindHarness
	}
	switch {
	case strings.HasPrefix(text, interruptedOpen):
		return KindInterrupted
	case strings.HasPrefix(text, taskNotificationOpen):
		return KindTaskNotification
	case strings.HasPrefix(text, localCommandOpen), strings.HasPrefix(text, localCommandNameOpen):
		return KindLocalCommand
	case strings.HasPrefix(text, compactionOpen):
		return KindCompaction
	}
	return KindOther
}

type Message struct {
	Content Content `json:"content"`
	Model   string  `json:"model"`
}

// Origin is the harness's sender stamp on a queued command.
type Origin struct {
	Kind string `json:"kind"`
}

// Texts decodes other attachment types' objects to empty, so the line still decodes for every reader.
type Texts []string

func (t *Texts) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	switch data[0] {
	case '"':
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return fmt.Errorf("Texts.UnmarshalJSON: Failed to decode text content: %w", err)
		}
		*t = Texts{text}
	case '[':
		var texts []string
		if err := json.Unmarshal(data, &texts); err == nil {
			*t = texts
		}
	}
	return nil
}

type ToolInput struct {
	Command      string `json:"command"`
	Description  string `json:"description"`
	FilePath     string `json:"file_path"`
	Limit        int    `json:"limit"`
	NotebookPath string `json:"notebook_path"`
	Offset       int    `json:"offset"`
	Prompt       string `json:"prompt"`
	Skill        string `json:"skill"`
}

// ToolUseResult models only the worktree path; string or array results decode to the zero value.
type ToolUseResult struct {
	WorktreePath string `json:"worktreePath"`
}

func (r *ToolUseResult) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || data[0] != '{' {
		return nil
	}

	object := toolUseResultObject{}
	if err := json.Unmarshal(data, &object); err != nil {
		return fmt.Errorf("ToolUseResult.UnmarshalJSON: Failed to decode result object: %w", err)
	}

	r.WorktreePath = object.WorktreePath
	return nil
}

// toolUseResultObject decodes without recursing into the custom unmarshaler.
type toolUseResultObject struct {
	WorktreePath string `json:"worktreePath"`
}

func slashLeaf(word string) (string, bool) {
	leaf, hasSlash := strings.CutPrefix(strings.Trim(word, slashWordWrapping), "/")
	return leaf, hasSlash && skillLeafPattern.MatchString(leaf)
}

func slashWords(text string) []string {
	skills := make([]string, 0)
	for word := range strings.FieldsSeq(text) {
		if leaf, isSlashWord := slashLeaf(word); isSlashWord {
			skills = append(skills, leaf)
		}
	}
	return skills
}

// stripHarnessPreamble keeps an unclosed block, so the message fails the exact-text checks.
func stripHarnessPreamble(text string) string {
	rest := strings.TrimSpace(text)
	for strings.HasPrefix(rest, systemReminderOpen) {
		_, after, isClosed := strings.Cut(rest, systemReminderClose)
		if !isClosed {
			return rest
		}
		rest = strings.TrimSpace(after)
	}
	return rest
}

// Classify names a line's author; every hook reads human rows through it and nothing else counts as the user.
func Classify(line *Line) Kind {
	switch line.Type {
	case TypeAssistant:
		return KindAssistant
	case TypeAttachment:
		if line.Attachment.isHumanQueuedPrompt() {
			return KindHumanQueued
		}
		return KindAttachment
	case TypeUser:
		return line.kindOfUser()
	}
	return KindOther
}

// CommandNameSkill reads the Desktop app's "<command-name>/<skill></command-name>" tag.
func CommandNameSkill(text string) []string {
	_, after, isFound := strings.Cut(text, commandNameOpen)
	if !isFound {
		return nil
	}
	name, _, isClosed := strings.Cut(after, commandNameClose)
	if !isClosed {
		return nil
	}
	fields := strings.Fields(name)
	if len(fields) == 0 {
		return nil
	}
	return []string{fields[0]}
}

// HumanSlashSkills counts typed and pasted "/<skill>" words at any position.
func HumanSlashSkills(line *Line) ([]string, bool) {
	content := humanContent(line)
	if content == nil {
		return nil, false
	}
	return content.SlashSkills(), true
}

// HumanTexts: what the user typed on a line, pastes removed — a prompt's text blocks or a mid-turn message — nothing for other kinds.
func HumanTexts(line *Line) []string {
	content := humanContent(line)
	if content == nil {
		return nil
	}
	return content.humanTexts()
}

// humanContent is the content the user wrote on a line, nil for every kind the user did not write.
func humanContent(line *Line) *Content {
	switch Classify(line) {
	case KindHuman:
		return &line.Message.Content
	case KindHumanQueued:
		return &line.Attachment.Prompt
	case KindAssistant, KindAttachment, KindCompaction, KindHarness, KindHeadless, KindHookFeedback, KindInterrupted,
		KindLocalCommand, KindOther, KindPeer, KindTaskNotification, KindToolResult:
		return nil
	}
	return nil
}

// IsHuman: a typed prompt row; hook feedback, tool results, compaction summaries, relays and headless prompts are not.
func IsHuman(line *Line) bool {
	return Classify(line) == KindHuman
}

// IsHumanQueuedPrompt: a message the user typed while a turn ran.
func IsHumanQueuedPrompt(line *Line) bool {
	return Classify(line) == KindHumanQueued
}

// NewScanner is sized for entries of tens of megabytes when a tool result embeds a large file.
func NewScanner(reader io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, lineBufferInitial), lineBufferMax)
	return scanner
}

// PromptStartSkill reads "/<skill> ..." at the start of a prompt.
func PromptStartSkill(text string) []string {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "/") {
		return nil
	}
	fields := strings.Fields(trimmed[1:])
	if len(fields) == 0 || fields[0] == "" {
		return nil
	}
	return []string{fields[0]}
}

// SlashCommand reads the Desktop command-name/command-args tags or "/<skill> <args>" on the first line.
func SlashCommand(text string) (string, string, bool) {
	if skills := CommandNameSkill(text); len(skills) > 0 {
		_, after, _ := strings.Cut(text, commandArgsOpen)
		args, _, _ := strings.Cut(after, commandArgsClose)
		return skills[0], strings.TrimSpace(args), true
	}

	skills := PromptStartSkill(text)
	if len(skills) == 0 {
		return "", "", false
	}

	firstLine, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	args := strings.TrimPrefix(firstLine, "/"+skills[0])
	return skills[0], strings.TrimSpace(args), true
}

// SlashWordArgs keeps line breaks in the arguments.
func SlashWordArgs(skill string, text string) (string, bool) {
	rest := strings.TrimLeftFunc(text, unicode.IsSpace)
	for rest != "" {
		word, after := rest, ""
		if end := strings.IndexFunc(rest, unicode.IsSpace); end >= 0 {
			word, after = rest[:end], rest[end:]
		}
		if leaf, isSlashWord := slashLeaf(word); isSlashWord && leaf == skill {
			return strings.TrimSpace(after), true
		}
		rest = strings.TrimLeftFunc(after, unicode.IsSpace)
	}
	return "", false
}

// SliceTokens returns the args' lane:/topic: context-slice tokens, sorted and deduplicated.
func SliceTokens(args string) []string {
	tokens := make([]string, 0)
	for _, match := range sliceTokenPattern.FindAllStringSubmatch(args, -1) {
		tokens = append(tokens, match[1])
	}

	slices.Sort(tokens)
	return slices.Compact(tokens)
}

// StripPastedContents removes every complete Desktop paste wrapper with its body; an unclosed wrapper stays as typed.
func StripPastedContents(text string) string {
	return rewritePastedContents(text, func(string) string { return "" })
}

// UnwrapPastedContents replaces every complete Desktop paste wrapper by its inner text; an unclosed wrapper stays as typed.
func UnwrapPastedContents(text string) string {
	return rewritePastedContents(text, func(body string) string { return "\n" + strings.TrimSpace(body) + "\n" })
}

// rewritePastedContents replaces each complete paste wrapper by what replace makes of its body; the rest is kept as typed.
func rewritePastedContents(text string, replace func(body string) string) string {
	var rewritten strings.Builder
	rest := text
	for {
		before, afterOpen, hasOpen := strings.Cut(rest, pastedContentOpen)
		if !hasOpen {
			break
		}
		_, inner, isOpenClosed := strings.Cut(afterOpen, "\">")
		if !isOpenClosed {
			break
		}
		body, afterClose, isClosed := strings.Cut(inner, pastedContentClose)
		if !isClosed {
			break
		}
		_, afterTag, isTagClosed := strings.Cut(afterClose, ">")
		if !isTagClosed {
			break
		}

		rewritten.WriteString(before)
		rewritten.WriteString(replace(body))
		rest = afterTag
	}
	rewritten.WriteString(rest)
	return rewritten.String()
}

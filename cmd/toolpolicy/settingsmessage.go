package main

import (
	"cmp"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/sessionstate"
	"github.com/kevinhorst/smine/internal/transcript"
	"github.com/kevinhorst/smine/internal/trash"
)

// A human message that edits the anchor worktree's settings.local.json or the session's switches.
type settingsMessage struct {
	directories []string
	disabled    []sessionstate.Feature
	enabled     []sessionstate.Feature
	isReset     bool
	rules       []string
	unknown     string
}

// hasEffect: the message changes a list or a switch; a rejected switch message changes nothing.
func (m *settingsMessage) hasEffect() bool {
	return len(m.directories)+len(m.rules)+len(m.disabled)+len(m.enabled) > 0
}

// isCommand: the message is answered on the typed prompt, a rejection included.
func (m *settingsMessage) isCommand() bool {
	return m.isReset || m.unknown != "" || m.hasEffect()
}

type settingsOutcome struct {
	directories []string
	disabled    []sessionstate.Feature
	enabled     []sessionstate.Feature
	guides      []string
	off         []sessionstate.Feature
	rejected    []string
	rules       []string
	unknown     string
}

func applySettingsMessages(target string, messages []settingsMessage, keep *trash.Store) ([]settingsOutcome, error) {
	outcomes := make([]settingsOutcome, len(messages))
	if len(messages) == 0 {
		return outcomes, nil
	}

	settings, err := loadLocalSettings(target)
	if err != nil {
		return nil, fmt.Errorf("applySettingsMessages: %w", err)
	}
	directories, err := permissionList(settings, additionalDirectoriesKey)
	if err != nil {
		return nil, fmt.Errorf("applySettingsMessages: %w", err)
	}
	allow, err := permissionList(settings, allowKey)
	if err != nil {
		return nil, fmt.Errorf("applySettingsMessages: %w", err)
	}

	isDirectoriesChanged, isAllowChanged := false, false
	for index, message := range messages {
		outcome := &outcomes[index]
		outcome.disabled = message.disabled
		outcome.enabled = message.enabled
		outcome.unknown = message.unknown
		for _, directory := range message.directories {
			outcome.directories = append(outcome.directories, directory)
			if !containsDirectory(directories, directory) {
				directories = append(directories, directory)
				isDirectoriesChanged = true
			}
		}
		for _, rule := range message.rules {
			if err := isExplicitRule(rule); err != nil {
				outcome.rejected = append(outcome.rejected, err.Error())
				continue
			}

			outcome.rules = append(outcome.rules, rule)
			if !slices.Contains(allow, rule) {
				allow = append(allow, rule)
				isAllowChanged = true
			}
		}
	}
	if !isDirectoriesChanged && !isAllowChanged {
		return outcomes, nil
	}

	if isDirectoriesChanged {
		if err := setPermissionList(settings, additionalDirectoriesKey, directories); err != nil {
			return nil, fmt.Errorf("applySettingsMessages: %w", err)
		}
	}
	if isAllowChanged {
		if err := setPermissionList(settings, allowKey, allow); err != nil {
			return nil, fmt.Errorf("applySettingsMessages: %w", err)
		}
	}
	if err := saveLocalSettings(target, settings, keep); err != nil {
		return nil, fmt.Errorf("applySettingsMessages: %w", err)
	}
	return outcomes, nil
}

func containsDirectory(directories []string, directory string) bool {
	return slices.ContainsFunc(directories, func(entry string) bool {
		return filepath.Clean(entry) == directory
	})
}

// directoryOf keeps only an absolute path naming an existing directory; anything else is silently nothing.
func directoryOf(path string) []string {
	if !filepath.IsAbs(path) {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return nil
	}
	return []string{filepath.Clean(path)}
}

func isExplicitRule(raw string) error {
	rule, err := ParseRule(raw)
	if err != nil {
		return err
	}
	if rule.Kind == ruleKindPathPattern {
		return fmt.Errorf("isExplicitRule: Rule %q is a path rule, which settings never match — use %s <directory>", raw, wordAdd)
	}
	return nil
}

// foldMessages keeps the messages the predicate admits; a #reset drops everything before it.
func foldMessages(kept []settingsMessage, messages []settingsMessage, keep func(*settingsMessage) bool) []settingsMessage {
	for index := range messages {
		message := &messages[index]
		if message.isReset {
			kept = kept[:0]
			continue
		}
		if keep(message) {
			kept = append(kept, *message)
		}
	}
	return kept
}

// promptHashtags parses the prompt being submitted, pastes removed; wrapped and headless prompts carry nothing.
func promptHashtags(input *HookInput) (humanCommands, bool) {
	isTyped := input.HookEventName == hookEventUserPromptSubmit && !isWrappedPrompt(input.Prompt)
	if !isTyped || os.Getenv(headlessEnv) == headlessEnvValue {
		return humanCommands{}, false
	}

	commands := parseHashtags(transcript.StripPastedContents(input.Prompt))
	isAnswered := slices.ContainsFunc(commands.messages, func(message settingsMessage) bool { return message.isCommand() })
	return commands, isAnswered || len(commands.approvals) > 0
}

// approvalReport names the lock state a typed #allow or #deny leaves.
func approvalReport(grant *approval) string {
	if grant.isAllow {
		return fmt.Sprintf("%s%s: %s unlocked for this session — the last %s or %s for it wins", reportPrefix, wordAllow, grant.target, wordAllow, wordDeny)
	}
	return fmt.Sprintf("%s%s: %s locked for this session", reportPrefix, wordDeny, grant.target)
}

// afterReset: the index after the prompt's last #reset, or 0 without one.
func afterReset(messages []settingsMessage) (int, bool) {
	for index, message := range slices.Backward(messages) {
		if message.isReset {
			return index + 1, true
		}
	}
	return 0, false
}

func reconcileSettingsMessages(input *HookInput, grants *Grants, contextDir string, statePath string, stdout io.Writer) {
	anchor := grants.anchorRoot(input)
	if anchor == "" {
		return
	}

	now := time.Now().UTC()
	target := filepath.Join(anchor, ".claude", unlockSettings)
	guides := guideWords(anchor, contextDir)
	prompt, hasPrompt := promptHashtags(input)
	expandSwitches(prompt.messages, guides)
	report := make([]string, 0)
	for index := range prompt.approvals {
		grant := &prompt.approvals[index]
		grants.Approvals[grant.target] = grant.isAllow
		report = append(report, approvalReport(grant))
	}

	messages := slices.Clone(grants.SettingsMessages)
	expandSwitches(messages, guides)
	promptStart, isReset := afterReset(prompt.messages)
	if isReset {
		line, err := resetPermissionLists(now, statePath, target, grants.trash)
		if err != nil {
			fmt.Fprintf(os.Stderr, "toolpolicy: reconcileSettingsMessages: %v\n", err)
			return
		}
		report = append(report, line)
		messages = messages[:0]
	}
	promptMessages := foldMessages(nil, prompt.messages[promptStart:], func(message *settingsMessage) bool { return message.isCommand() })
	messages = append(messages, promptMessages...)
	if input.HookEventName == hookEventDirectoryAdded {
		added := settingsMessage{directories: directoryOf(input.Directory)}
		messages = append(messages, added)
	}

	outcomes, err := applySettingsMessages(target, messages, grants.trash)
	if err != nil {
		fmt.Fprintf(os.Stderr, "toolpolicy: reconcileSettingsMessages: %v\n", err)
		return
	}
	off, err := applySwitches(messages, now, statePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "toolpolicy: reconcileSettingsMessages: %v\n", err)
		return
	}
	for _, outcome := range outcomes {
		grants.AdditionalDirectories = append(grants.AdditionalDirectories, outcome.directories...)
		grants.Allow = append(grants.Allow, parseRules(outcome.rules, target)...)
	}

	answered := outcomes[len(outcomes)-len(promptMessages)-directoryAddedCount(input):]
	for index := range answered[:len(promptMessages)] {
		answered[index].guides = guides
		answered[index].off = off
		report = append(report, strings.TrimSuffix(settingsReport(&answered[index], target), "\n"))
	}
	if hasPrompt && len(report) > 0 {
		fmt.Fprintln(stdout, strings.Join(report, "\n"))
	}
}

// directoryAddedCount: the DirectoryAdded event appends one message after the prompt's.
func directoryAddedCount(input *HookInput) int {
	if input.HookEventName == hookEventDirectoryAdded {
		return 1
	}
	return 0
}

// applySwitches writes the folded off set and its history into the session record when it differs; no path means no record.
func applySwitches(messages []settingsMessage, now time.Time, statePath string) ([]sessionstate.Feature, error) {
	off := foldSwitches(messages)
	if statePath == "" {
		return off, nil
	}

	state, err := sessionstate.Load(statePath)
	if err != nil {
		return nil, fmt.Errorf("applySwitches: %w", err)
	}
	if slices.Equal(state.Switches.Off, off) {
		return off, nil
	}

	state.Switches.Set(off, now)
	if err := sessionstate.Save(statePath, state); err != nil {
		return nil, fmt.Errorf("applySwitches: %w", err)
	}
	return off, nil
}

// expandSwitches turns each message's typed words into members; a word outside the catalog rejects its message.
func expandSwitches(messages []settingsMessage, guides []string) {
	for index := range messages {
		message := &messages[index]
		disabled, unknownDisabled := sessionstate.Expand(message.disabled, guides)
		enabled, unknownEnabled := sessionstate.Expand(message.enabled, guides)
		unknown := cmp.Or(message.unknown, unknownDisabled, unknownEnabled)
		if unknown != "" {
			*message = settingsMessage{unknown: unknown}
			continue
		}
		message.disabled, message.enabled = disabled, enabled
	}
}

// featureWords joins features for a report line, whole groups by their word; an empty set reads "none".
func featureWords(features []sessionstate.Feature, guides []string) string {
	if len(features) == 0 {
		return "none"
	}
	return strings.Join(sessionstate.Compact(slices.Sorted(slices.Values(features)), guides), ", ")
}

// guideWords: the read-gate members the anchor's context index adds, by the configured context dir; none without one.
func guideWords(anchor string, contextDir string) []string {
	if contextDir == "" {
		return nil
	}
	return sessionstate.GuideWords(filepath.Join(anchor, contextDir, contextdocs.ContextFileName))
}

// foldSwitches: per feature the last message wins; the result is sorted.
func foldSwitches(messages []settingsMessage) []sessionstate.Feature {
	off := make(map[sessionstate.Feature]bool)
	for _, message := range messages {
		for _, feature := range message.disabled {
			off[feature] = true
		}
		for _, feature := range message.enabled {
			delete(off, feature)
		}
	}
	return slices.Sorted(maps.Keys(off))
}

func resetPermissionLists(now time.Time, statePath string, target string, keep *trash.Store) (string, error) {
	settings, err := loadLocalSettings(target)
	if err != nil {
		return "", fmt.Errorf("resetPermissionLists: %w", err)
	}
	directories, err := permissionList(settings, additionalDirectoriesKey)
	if err != nil {
		return "", fmt.Errorf("resetPermissionLists: %w", err)
	}
	allow, err := permissionList(settings, allowKey)
	if err != nil {
		return "", fmt.Errorf("resetPermissionLists: %w", err)
	}

	state, err := sessionstate.Load(statePath)
	if err != nil {
		return "", fmt.Errorf("resetPermissionLists: %w", err)
	}
	switches := len(state.Switches.Off)
	report := fmt.Sprintf(
		"%s%s: cleared permissions.allow (%d), permissions.%s (%d) and switches (%d) of %s",
		reportPrefix,
		wordReset,
		len(allow),
		additionalDirectoriesKey,
		len(directories),
		switches,
		target,
	)
	if switches > 0 {
		state.Switches.Set(nil, now)
		if err := sessionstate.Save(statePath, state); err != nil {
			return "", fmt.Errorf("resetPermissionLists: %w", err)
		}
	}
	if len(allow)+len(directories) == 0 {
		return report, nil
	}

	settings.Doc().Unset([]string{permissionsKey, allowKey})
	settings.Doc().Unset([]string{permissionsKey, additionalDirectoriesKey})
	if err := saveLocalSettings(target, settings, keep); err != nil {
		return "", fmt.Errorf("resetPermissionLists: %w", err)
	}
	return report, nil
}

func settingsReport(outcome *settingsOutcome, target string) string {
	lines := make([]string, 0)
	for _, directory := range outcome.directories {
		lines = append(lines, fmt.Sprintf("%s%s: %s is in %s of %s — a root from this call on", reportPrefix, wordAdd, directory, additionalDirectoriesKey, target))
	}
	if len(outcome.rules) > 0 {
		lines = append(lines, fmt.Sprintf("%s%s: %d rule(s) in permissions.allow of %s — %s", reportPrefix, wordSet, len(outcome.rules), target, strings.Join(outcome.rules, " ")))
	}
	for _, rejected := range outcome.rejected {
		lines = append(lines, fmt.Sprintf("%s%s: rejected — %s", reportPrefix, wordSet, rejected))
	}
	lines = append(lines, switchReport(outcome)...)
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// switchMessage parses the words after #disable or #enable; a word outside the grammar rejects the whole message.
func switchMessage(fields []string) settingsMessage {
	if len(fields) < switchFieldCount {
		return settingsMessage{}
	}

	features := make([]sessionstate.Feature, 0, len(fields)-1)
	for _, word := range fields[1:] {
		parsed, isKnown := sessionstate.Parse(word)
		if !isKnown {
			return settingsMessage{unknown: word}
		}
		features = append(features, parsed...)
	}
	if fields[0] == wordDisable {
		return settingsMessage{disabled: features}
	}
	return settingsMessage{enabled: features}
}

// switchReport: the typed switch message's outcome and the off set it leaves.
func switchReport(outcome *settingsOutcome) []string {
	lines := make([]string, 0)
	if outcome.unknown != "" {
		lines = append(lines, fmt.Sprintf("%s%s: rejected — unknown feature %q; features: %s", reportPrefix, wordDisable, outcome.unknown, sessionstate.Names()))
	}
	off := featureWords(outcome.off, outcome.guides)
	if len(outcome.disabled) > 0 {
		lines = append(lines, fmt.Sprintf("%s%s: off — %s (from the next hook event on)", reportPrefix, wordDisable, off))
	}
	if len(outcome.enabled) > 0 {
		lines = append(lines, fmt.Sprintf("%s%s: on — %s; still off — %s", reportPrefix, wordEnable, featureWords(outcome.enabled, outcome.guides), off))
	}
	return lines
}

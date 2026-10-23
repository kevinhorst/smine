package main

import (
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/transcript"
)

// hashtagWords are the first fields that make a line of a human message a command; any other line is prose.
var hashtagWords = []string{wordAllow, wordDeny, wordAdd, wordDisable, wordEnable, wordReset, wordSet}

// An approval line: #allow or #deny plus its target.
type approval struct {
	isAllow bool
	target  string
}

// A command line of a human message, with the rule lines a #set takes after it.
type hashtagLine struct {
	fields    []string
	following []string
	raw       string
}

// The commands one human text carries, in line order.
type humanCommands struct {
	approvals []approval
	messages  []settingsMessage
}

// commandText is the text a prompt carries its command lines in: a /<skill> prompt's args, the whole text otherwise.
func commandText(text string) string {
	if len(transcript.CommandNameSkill(text)) > 0 {
		_, args, _ := transcript.SlashCommand(text)
		return args
	}

	skills := transcript.PromptStartSkill(text)
	if len(skills) == 0 {
		return text
	}
	return strings.TrimPrefix(strings.TrimSpace(text), "/"+skills[0])
}

// hashtagLines scans a text line by line; a #set also takes the lines after it up to a blank line or the next command.
func hashtagLines(text string) []hashtagLine {
	lines := strings.Split(text, "\n")
	commands := make([]hashtagLine, 0)
	for index := 0; index < len(lines); index++ {
		fields := strings.Fields(lines[index])
		if len(fields) == 0 || !slices.Contains(hashtagWords, fields[0]) {
			continue
		}

		command := hashtagLine{fields: fields, raw: strings.TrimSpace(lines[index])}
		if fields[0] == wordSet {
			command.following, index = setContinuation(lines, index)
		}
		commands = append(commands, command)
	}
	return commands
}

// isCommandLine: the line's first field is a hashtag word.
func isCommandLine(line string) bool {
	fields := strings.Fields(line)
	return len(fields) > 0 && slices.Contains(hashtagWords, fields[0])
}

// parseHashtags folds every command line of one human text: approvals and settings messages, each in line order.
func parseHashtags(text string) humanCommands {
	commands := humanCommands{}
	for _, line := range hashtagLines(commandText(text)) {
		switch line.fields[0] {
		case wordAllow, wordDeny:
			if len(line.fields) != approvalFieldCount {
				continue
			}
			grant := approval{isAllow: line.fields[0] == wordAllow, target: approvalTarget(line.fields[1])}
			commands.approvals = append(commands.approvals, grant)
		default:
			commands.messages = append(commands.messages, parseSettingsLine(&line))
		}
	}
	return commands
}

// parseSettingsLine applies each command word's own line grammar; a line that misses it is nothing.
func parseSettingsLine(line *hashtagLine) settingsMessage {
	fields := line.fields
	switch fields[0] {
	case wordAdd:
		if len(fields) != addFieldCount {
			return settingsMessage{}
		}
		return settingsMessage{directories: directoryOf(fields[1])}
	case wordDisable, wordEnable:
		return switchMessage(fields)
	case wordReset:
		return settingsMessage{isReset: len(fields) == 1}
	case wordSet:
		return settingsMessage{rules: setRules(line)}
	}
	return settingsMessage{}
}

// setContinuation collects the non-blank lines after a #set that are not commands themselves.
func setContinuation(lines []string, start int) ([]string, int) {
	following := make([]string, 0)
	index := start
	for index+1 < len(lines) {
		next := strings.TrimSpace(lines[index+1])
		if next == "" || isCommandLine(next) {
			break
		}
		following = append(following, next)
		index++
	}
	return following, index
}

// setRules: the rest of the #set line, verbatim, then one rule per following line.
func setRules(line *hashtagLine) []string {
	rules := make([]string, 0, len(line.following)+1)
	first := strings.TrimSpace(strings.TrimPrefix(line.raw, wordSet))
	if first != "" {
		rules = append(rules, first)
	}
	return append(rules, line.following...)
}

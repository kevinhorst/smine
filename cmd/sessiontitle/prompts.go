package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"

	"github.com/kevinhorst/smine/internal/transcript"
)

// Triggers are sessiontitle.trigger_patterns compiled; memberToken matches the join command's stdout, titleSet holds the token in group titleSetTokenGroup.
type Triggers struct {
	closeCheck     *regexp.Regexp
	done           *regexp.Regexp
	join           *regexp.Regexp
	memberToken    *regexp.Regexp
	subscribe      *regexp.Regexp
	subscribeFleet *regexp.Regexp
	titleSet       *regexp.Regexp
}

// compileTriggers compiles the patterns in their configured order: close check, done, join, subscribe, the fleet flag, the member token, title set.
func compileTriggers(patterns []string) (*Triggers, error) {
	triggers := &Triggers{}
	slots := []**regexp.Regexp{
		&triggers.closeCheck,
		&triggers.done,
		&triggers.join,
		&triggers.subscribe,
		&triggers.subscribeFleet,
		&triggers.memberToken,
		&triggers.titleSet,
	}
	if len(patterns) != len(slots) {
		return nil, fmt.Errorf("compileTriggers: sessiontitle.trigger_patterns holds %d patterns, not %d", len(patterns), len(slots))
	}

	for i, pattern := range patterns {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("compileTriggers: sessiontitle.trigger_patterns[%d]: %w", i, err)
		}
		*slots[i] = compiled
	}
	return triggers, nil
}

func promptSkills(transcriptPath string) ([]string, bool, error) {
	file, err := os.Open(transcriptPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("promptSkills: Failed to open %s: %w", transcriptPath, err)
	}
	defer file.Close()

	skills := make([]string, 0)
	hasPrompt := false
	scanner := transcript.NewScanner(file)
	for scanner.Scan() {
		line := transcript.Line{}
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}
		if !transcript.IsHuman(&line) {
			continue
		}
		hasPrompt = true
		text := line.Message.Content.LeadingText()
		skills = append(skills, transcript.CommandNameSkill(text)...)
		skills = append(skills, transcript.PromptStartSkill(text)...)
	}
	if err := scanner.Err(); err != nil {
		return nil, false, fmt.Errorf("promptSkills: Failed to read %s: %w", transcriptPath, err)
	}
	return skills, hasPrompt, nil
}

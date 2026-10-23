package budget

import (
	"errors"
	"fmt"

	"github.com/kevinhorst/smine/internal/contextrecord"
	"github.com/kevinhorst/smine/internal/peek"
)

const turnRoleUser = "user"

// events may be nil; turns is read for Codex only.
func ExtractRecord(claudeHome, contextDir string, events *peek.SessionEvents, session *peek.Session, turns []peek.Turn) (*contextrecord.Record, error) {
	switch session.Agent {
	case contextrecord.SessionClaude:
		transcript := TranscriptPath(claudeHome, session.Id)
		if transcript == "" {
			return nil, errors.New("ExtractRecord: No transcript on disk")
		}
		record, err := contextrecord.Extract(transcript, contextDir)
		if err != nil {
			return nil, fmt.Errorf("ExtractRecord: %w", err)
		}
		return record, nil
	case contextrecord.SessionCodex:
		return contextrecord.ExtractCodex(session.Id, session.Model, userTurnTexts(turns), fileTouches(events)), nil
	}
	return nil, fmt.Errorf("ExtractRecord: Unknown session agent %q", session.Agent)
}

func fileTouches(events *peek.SessionEvents) []contextrecord.FileTouch {
	if events == nil {
		return nil
	}

	touches := make([]contextrecord.FileTouch, 0, len(events.TouchedFiles))
	for _, file := range events.TouchedFiles {
		touch := contextrecord.FileTouch{Path: file.Path, Reads: file.Reads, Writes: file.Writes}
		touches = append(touches, touch)
	}
	return touches
}

func userTurnTexts(turns []peek.Turn) []string {
	var texts []string
	for _, turn := range turns {
		if turn.Role == turnRoleUser {
			texts = append(texts, turn.Text)
		}
	}
	return texts
}

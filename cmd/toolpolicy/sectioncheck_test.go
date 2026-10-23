package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kevinhorst/smine/internal/transcript"
)

// bashCallLine is an assistant line running command through Bash.
func bashCallLine(command string) transcript.Line {
	block := transcript.Block{Id: "bash", Input: transcript.ToolInput{Command: command}, Name: toolNameBash, Type: transcript.BlockToolUse}
	line := transcript.Line{
		Message: transcript.Message{Content: transcript.Content{Blocks: []transcript.Block{block}}},
		Type:    transcript.TypeAssistant,
	}
	return line
}

func TestTranscriptScan_ObserveSectionChecks(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		lines []transcript.Line
	}

	planFile := "plans/2026-09-28-probe-7ea30b5/2026-09-28-probe-wip-7ea30b5/design/probe.md"
	tests := make([]*testCase, 0)

	// section-check-kept
	tests = append(tests, &testCase{
		_id:       "section-check-kept",
		_expected: []string{planFile},

		lines: []transcript.Line{bashCallLine("smine-rules plan-sections " + planFile)},
	})

	// surrounding-whitespace-trimmed
	tests = append(tests, &testCase{
		_id:       "surrounding-whitespace-trimmed",
		_expected: []string{planFile},

		lines: []transcript.Line{bashCallLine("  smine-rules plan-sections " + planFile + "\n")},
	})

	// every-check-kept-in-order
	tests = append(tests, &testCase{
		_id:       "every-check-kept-in-order",
		_expected: []string{"a.md", "b.md"},

		lines: []transcript.Line{bashCallLine("smine-rules plan-sections a.md"), bashCallLine("smine-rules plan-sections b.md")},
	})

	// other-commands-ignored
	tests = append(tests, &testCase{
		_id:       "other-commands-ignored",
		_expected: nil,

		lines: []transcript.Line{
			bashCallLine("smine-rules plan-display " + planFile + " ~/.claude/plans/x.md"),
			bashCallLine("smine-rules plan-sections " + planFile + " | tail"),
			bashCallLine("cp " + planFile + " elsewhere.md"),
			toolCallLine("write", toolNameWrite),
		},
	})

	// user-lines-ignored
	userLine := bashCallLine("smine-rules plan-sections " + planFile)
	userLine.Type = transcript.TypeUser
	tests = append(tests, &testCase{
		_id:       "user-lines-ignored",
		_expected: nil,

		lines: []transcript.Line{userLine},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			scan := transcriptScan{}
			for _, line := range test.lines {
				scan.observeSectionChecks(&line)
			}
			assert.Equal(t, test._expected, scan.sectionedPlans)
		})
	}
}

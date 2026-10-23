package plans

import "strings"

// minFenceRun is the shortest backtick or tilde run that opens a fence.
const minFenceRun = 3

// Only a same-character run at least as long closes a fence, so quoted inner fences stay inside.
func NextFence(open string, line string) string {
	trimmed := strings.TrimLeft(line, " ")
	marker := ""
	for _, char := range []string{"`", "~"} {
		run := len(trimmed) - len(strings.TrimLeft(trimmed, char))
		if run >= minFenceRun {
			marker = strings.Repeat(char, run)
		}
	}
	if marker == "" {
		return open
	}
	if open == "" {
		return marker
	}
	isClosing := marker[0] == open[0] && len(marker) >= len(open) && strings.TrimSpace(trimmed[len(marker):]) == ""
	if isClosing {
		return ""
	}
	return open
}

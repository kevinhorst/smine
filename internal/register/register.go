package register

import (
	"regexp"
)

var (
	backtick    = regexp.MustCompile("`")
	hexId       = regexp.MustCompile(`[0-9a-f]{8,}`)
	parenNumber = regexp.MustCompile(`\(\d`)
	pathLine    = regexp.MustCompile(`/\S*:\d+`)
	sentenceEnd = regexp.MustCompile(`[.!?]\s`)
)

// SentenceViolations names what breaks a one-sentence headline at a glance: a second sentence, code, an id, a path or a count.
func SentenceViolations(text string) []string {
	var violations []string
	if sentenceEnd.MatchString(text) {
		violations = append(violations, "more than one sentence")
	}
	if backtick.MatchString(text) {
		violations = append(violations, "backtick")
	}
	if hexId.MatchString(text) {
		violations = append(violations, "hex id")
	}
	if pathLine.MatchString(text) {
		violations = append(violations, "path with line number")
	}
	if parenNumber.MatchString(text) {
		violations = append(violations, "parenthesised number")
	}
	return violations
}

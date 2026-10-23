package evals

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

// The Tests tab shows every interpretation bullet whole.
const (
	InterpretationBulletLimit = 8
	InterpretationBulletRunes = 160
)

// A run written earlier keeps the interpretation it was written with.
const InterpretationCutoff = "2026-10-06"

// The run dir name carries the run date.
var interpretationRunDate = regexp.MustCompile(`(?:^|/)evals/[^/]+/(\d{4}-\d{2}-\d{2})-[^/]+/interpretation\.md$`)

// InterpretationViolation is one line of an interpretation.md outside the interpretation bounds.
type InterpretationViolation struct {
	Line    int
	Message string
}

func interpretationBullet(line string) (string, bool) {
	isBullet := strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ")
	if !isBullet {
		return "", false
	}
	return strings.TrimSpace(line[2:]), true
}

// CheckInterpretation reports every heading, paragraph, surplus bullet and over-long bullet.
func CheckInterpretation(markdown string) []InterpretationViolation {
	var violations []InterpretationViolation
	bulletCount := 0
	for index, line := range strings.Split(markdown, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		bullet, isBullet := interpretationBullet(line)
		if !isBullet {
			violation := InterpretationViolation{Line: index + 1, Message: "only top-level bullets allowed, no heading or paragraph"}
			violations = append(violations, violation)
			continue
		}
		bulletCount++
		if bulletCount == InterpretationBulletLimit+1 {
			violation := InterpretationViolation{Line: index + 1, Message: fmt.Sprintf("more than %d bullets", InterpretationBulletLimit)}
			violations = append(violations, violation)
		}
		runeCount := utf8.RuneCountInString(bullet)
		if runeCount > InterpretationBulletRunes {
			message := fmt.Sprintf("bullet is %d runes, at most %d", runeCount, InterpretationBulletRunes)
			violation := InterpretationViolation{Line: index + 1, Message: message}
			violations = append(violations, violation)
		}
	}
	return violations
}

// InterpretationBullets keeps the top-level list items; headings and paragraphs are dropped.
func InterpretationBullets(markdown string) []string {
	var bullets []string
	for line := range strings.SplitSeq(markdown, "\n") {
		bullet, isBullet := interpretationBullet(line)
		if isBullet {
			bullets = append(bullets, bullet)
		}
	}
	return bullets
}

// A path outside a dated run dir is checked.
func IsInterpretationChecked(path string) bool {
	match := interpretationRunDate.FindStringSubmatch(filepath.ToSlash(path))
	if match == nil {
		return true
	}
	return match[1] >= InterpretationCutoff
}

package secretscan

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

type compiledPattern struct {
	confidence string
	id         string
	regex      *regexp.Regexp
}

type span struct {
	end   int
	start int
}

func compilePatterns() ([]compiledPattern, error) {
	patterns := make([]compiledPattern, 0, len(patternSpecs))
	for _, spec := range patternSpecs {
		regex, err := regexp.Compile(spec.regex)
		if err != nil {
			return nil, fmt.Errorf("compilePatterns: Invalid pattern %s: %w", spec.id, err)
		}

		patterns = append(patterns, compiledPattern{confidence: spec.confidence, id: spec.id, regex: regex})
	}

	return patterns, nil
}

func excerpt(value string) string {
	value = strings.ReplaceAll(value, "\n", `\n`)
	if utf8.RuneCountInString(value) <= excerptMaxLength {
		return value
	}
	runes := []rune(value)

	return string(runes[:excerptMaxLength]) + "…"
}

func lineOf(content []byte, offset int) int {
	return 1 + bytes.Count(content[:offset], []byte{'\n'})
}

// scanContent runs all pattern detectors and the entropy detector over one
// file's content; entropy hits overlapping a pattern hit are suppressed.
func scanContent(patterns []compiledPattern, path string, content []byte) []Finding {
	findings := make([]Finding, 0)
	patternSpans := make([]span, 0)
	for _, pattern := range patterns {
		for _, match := range pattern.regex.FindAllIndex(content, -1) {
			value := string(content[match[0]:match[1]])
			findings = append(findings, Finding{
				value:      value,
				Confidence: pattern.confidence,
				Detector:   pattern.id,
				Excerpt:    excerpt(value),
				Line:       lineOf(content, match[0]),
				Path:       path,
			})
			patternSpans = append(patternSpans, span{end: match[1], start: match[0]})
		}
	}

	findings = append(findings, entropyFindings(path, content, patternSpans)...)
	sortFindings(findings)
	return findings
}

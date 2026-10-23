// Package cmdline splits a command value on spaces with double-quote grouping; the grammar lives in the README.
package cmdline

import (
	"errors"
	"fmt"
	"strings"
)

// operators are the shell constructs a command value never carries; "||" precedes "|" so the longer one is named.
var operators = []string{"&&", "||", "|", ";", "$("}

// Split splits line into words; a value with a shell operator, a backtick, a newline or an unbalanced quote is refused.
func Split(line string) ([]string, error) {
	if strings.ContainsAny(line, "\n\r") {
		return nil, errors.New("cmdline.Split: Newline in a command value")
	}
	if strings.Contains(line, "`") {
		return nil, errors.New("cmdline.Split: Backtick in a command value")
	}
	for _, operator := range operators {
		if strings.Contains(line, operator) {
			return nil, fmt.Errorf("cmdline.Split: Shell operator %q in %q", operator, line)
		}
	}
	return splitWords(line)
}

// SplitLines splits a multi-line command value into one argv per non-blank line; a value without a command line is refused.
func SplitLines(value string) ([][]string, error) {
	var lines [][]string
	for line := range strings.SplitSeq(value, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}

		words, err := Split(line)
		if err != nil {
			return nil, fmt.Errorf("cmdline.SplitLines: %w", err)
		}
		if len(words) == 0 {
			continue
		}
		lines = append(lines, words)
	}
	if len(lines) == 0 {
		return nil, errors.New("cmdline.SplitLines: No command line in the value")
	}
	return lines, nil
}

// splitWords walks the runes once: a quote toggles grouping, a backslash before a quote inside a group escapes it.
func splitWords(line string) ([]string, error) {
	var words []string
	var word strings.Builder
	isQuoted := false
	hasWord := false
	runes := []rune(line)
	for index := 0; index < len(runes); index++ {
		current := runes[index]
		isEscapedQuote := isQuoted && current == '\\' && index+1 < len(runes) && runes[index+1] == '"'
		switch {
		case isEscapedQuote:
			word.WriteRune('"')
			index++
		case current == '"':
			isQuoted = !isQuoted
			hasWord = true
		case current == ' ' && !isQuoted:
			if hasWord {
				words = append(words, word.String())
				word.Reset()
				hasWord = false
			}
		default:
			word.WriteRune(current)
			hasWord = true
		}
	}
	if isQuoted {
		return nil, fmt.Errorf("cmdline.Split: Unbalanced quote in %q", line)
	}
	if hasWord {
		words = append(words, word.String())
	}
	return words, nil
}

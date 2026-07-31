package secretscan

import (
	"bytes"
	"fmt"
	"math"
	"regexp"
	"strings"
)

// Thresholds and minimum lengths are calibrated against the fixture corpus in
// entropy_test.go — change them only together with the corpus.
const (
	base64EntropyThreshold = 4.5
	base64MinTokenLength   = 24
	hexEntropyThreshold    = 3.0
	hexMinTokenLength      = 32
)

const (
	detectorEntropyBase64 = "entropy-base64"
	detectorEntropyHex    = "entropy-hex"
)

var (
	base64TokenRegex = regexp.MustCompile(fmt.Sprintf(`[A-Za-z0-9+/=_\-]{%d,}`, base64MinTokenLength))
	hexTokenRegex    = regexp.MustCompile(fmt.Sprintf(`\b[0-9a-fA-F]{%d,}\b`, hexMinTokenLength))
)

func entropyFindings(path string, content []byte, patternSpans []span) []Finding {
	findings := make([]Finding, 0)
	findings = append(findings, tokenFindings(detectorEntropyBase64, base64TokenRegex, base64EntropyThreshold, path, content, patternSpans)...)
	findings = append(findings, tokenFindings(detectorEntropyHex, hexTokenRegex, hexEntropyThreshold, path, content, patternSpans)...)

	return findings
}

// lineContextWindow bounds the context inspected around a token so minified
// single-line files stay O(tokens), not O(tokens x line length).
const lineContextWindow = 512

func hasLineContext(content []byte, offset int, keywords []string) bool {
	lineStart := bytes.LastIndexByte(content[:offset], '\n') + 1
	lineStart = max(lineStart, offset-lineContextWindow)
	lineEnd := len(content)
	if newlineIndex := bytes.IndexByte(content[offset:], '\n'); newlineIndex >= 0 {
		lineEnd = offset + newlineIndex
	}
	lineEnd = min(lineEnd, offset+lineContextWindow)

	lineText := strings.ToLower(string(content[lineStart:lineEnd]))
	for _, keyword := range keywords {
		if strings.Contains(lineText, keyword) {
			return true
		}
	}

	return false
}

func overlapsAny(start, end int, spans []span) bool {
	for _, candidate := range spans {
		if start < candidate.end && candidate.start < end {
			return true
		}
	}

	return false
}

// shannonEntropy sums over a fixed-order byte array so float accumulation
// order is deterministic (map iteration order would not be).
func shannonEntropy(token string) float64 {
	var counts [256]int
	for _, tokenByte := range []byte(token) {
		counts[tokenByte]++
	}

	entropy := 0.0
	for _, count := range &counts {
		if count == 0 {
			continue
		}
		probability := float64(count) / float64(len(token))
		entropy -= probability * math.Log2(probability)
	}

	return entropy
}

func tokenFindings(detector string, tokenRegex *regexp.Regexp, threshold float64, path string, content []byte, patternSpans []span) []Finding {
	findings := make([]Finding, 0)
	for _, match := range tokenRegex.FindAllIndex(content, -1) {
		if overlapsAny(match[0], match[1], patternSpans) {
			continue
		}
		token := string(content[match[0]:match[1]])
		if shannonEntropy(token) < threshold {
			continue
		}
		if detector == detectorEntropyHex && hasLineContext(content, match[0], hashContextKeywords) {
			continue
		}
		if detector == detectorEntropyBase64 && hasLineContext(content, match[0], mediaContextKeywords) {
			continue
		}

		findings = append(findings, Finding{
			value:      token,
			Confidence: ConfidenceMedium,
			Detector:   detector,
			Excerpt:    excerpt(token),
			Line:       lineOf(content, match[0]),
			Path:       path,
		})
	}

	return findings
}

// Package redact is the one deterministic secret and PII detector.
package redact

import (
	"cmp"
	"regexp"
	"slices"
	"strings"
)

const (
	// secretGroup marks the secret when a pattern keeps its surrounding name.
	secretGroup = "secret"

	// secretValue excludes a leading "[" so an already-redacted marker never re-matches.
	secretValue = `(?P<secret>"[^"]*"|'[^']*'|[^\s'"\[][^\s'"]*)`
)

var piiPatterns = []*pattern{
	{kind: "email", regex: regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,}\b`)},
	{kind: "phone", regex: regexp.MustCompile(`\+\d{1,3}[\d ()/-]{7,}\d`)},
	{kind: "iban", regex: regexp.MustCompile(`\b[A-Z]{2}\d{2}(?: ?[A-Z0-9]{4}){3,7}(?: ?[A-Z0-9]{1,3})?\b`)},
	{kind: "home-path", regex: regexp.MustCompile(`(?:/Users/|/home/|[A-Za-z]:\\Users\\)[^/\\\s]+`)},
}

var secretPatterns = []*pattern{
	{kind: "private-key", regex: regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?(?:-----END [A-Z ]*PRIVATE KEY-----|\z)`)},
	{kind: "anthropic-key", regex: regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{8,}`)},
	{kind: "api-key", regex: regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}`)},
	{kind: "github-token", regex: regexp.MustCompile(`\b(?:gh[pousr]_[A-Za-z0-9]{36,}|github_pat_[A-Za-z0-9_]{22,})`)},
	{kind: "aws-key", regex: regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`)},
	{kind: "slack-token", regex: regexp.MustCompile(`\bxox[abposr]-[A-Za-z0-9-]{10,}`)},
	{kind: "google-key", regex: regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}`)},
	{kind: "jwt", regex: regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`)},
	{kind: "url-credentials", regex: regexp.MustCompile(`://[^/\s:@]+:(?P<secret>[^/\s@\[][^/\s@]*)@`)},
	{kind: "auth-header", regex: regexp.MustCompile(`(?i)authorization:\s*(?:bearer|basic|token)\s+` + secretValue)},
	{kind: "secret-flag", regex: regexp.MustCompile(`(?i)--(?:token|password|passwd|api-key|secret|access-key)[= ]` + secretValue)},
	{kind: "secret-assignment", regex: regexp.MustCompile(`(?i)\b[a-z0-9_]*(?:token|secret|password|passwd|api_?key|access_?key|private_?key)[a-z0-9_]*=` + secretValue)},
}

// Hit never carries the matched text, so a report cannot re-leak it.
type Hit struct {
	Kind string
	Line int
}

type pattern struct {
	kind  string
	regex *regexp.Regexp
}

func (p *pattern) redact(text string) string {
	group := p.regex.SubexpIndex(secretGroup)
	var out strings.Builder
	last := 0
	for _, match := range p.regex.FindAllStringSubmatchIndex(text, -1) {
		start, end := match[0], match[1]
		hasSecretGroup := group > 0 && match[2*group] >= 0
		if hasSecretGroup {
			start, end = match[2*group], match[2*group+1]
		}
		out.WriteString(text[last:start])
		out.WriteString("[REDACTED:" + p.kind + "]")
		last = end
	}
	out.WriteString(text[last:])
	return out.String()
}

// RedactSecrets keeps PII classes: paths and addresses in commands are allowlist evidence.
func RedactSecrets(text string) string {
	for _, secret := range secretPatterns {
		text = secret.redact(text)
	}
	return text
}

// Scan reports every secret and PII match in text, ordered by line then kind.
func Scan(text string) []Hit {
	var hits []Hit
	for _, detector := range slices.Concat(secretPatterns, piiPatterns) {
		for _, match := range detector.regex.FindAllStringIndex(text, -1) {
			hit := Hit{Kind: detector.kind, Line: strings.Count(text[:match[0]], "\n") + 1}
			hits = append(hits, hit)
		}
	}
	slices.SortFunc(hits, func(a, b Hit) int {
		return cmp.Or(cmp.Compare(a.Line, b.Line), strings.Compare(a.Kind, b.Kind))
	})
	return hits
}

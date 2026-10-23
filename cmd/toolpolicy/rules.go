package main

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

type ruleKind int

const (
	// ruleKindTool matches a tool by bare name.
	ruleKindTool ruleKind = iota
	// ruleKindBashPrefix matches a shell command against a wildcard pattern.
	ruleKindBashPrefix
	// ruleKindName matches a Skill or Agent name, with an optional ":*" prefix form.
	ruleKindName
	// ruleKindDomain matches a WebFetch hostname pattern.
	ruleKindDomain
	// ruleKindMcp matches a whole MCP server, one tool, or a tool glob.
	ruleKindMcp
	// ruleKindPathPattern is a Read/Edit path rule — parsed, never matched.
	ruleKindPathPattern
	// ruleKindParam is a Tool(param:value) rule — deny/ask only, never matched.
	ruleKindParam
)

type Rule struct {
	Kind ruleKind
	Raw  string
	Spec string
	Tool string
}

func ParseRule(raw string) (*Rule, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("ParseRule: Empty rule")
	}

	open := strings.Index(trimmed, "(")
	if open < 0 {
		if !isToolName(trimmed) && !isMcpRuleName(trimmed) {
			return nil, fmt.Errorf("ParseRule: Invalid tool name in rule %q", trimmed)
		}
		return newBareRule(trimmed), nil
	}

	if !strings.HasSuffix(trimmed, ")") {
		return nil, fmt.Errorf("ParseRule: Unbalanced parentheses in rule %q", trimmed)
	}
	tool := trimmed[:open]
	spec := trimmed[open+1 : len(trimmed)-1]
	if !isToolName(tool) {
		return nil, fmt.Errorf("ParseRule: Invalid tool name in rule %q", trimmed)
	}
	if strings.HasPrefix(tool, mcpPrefix) {
		return nil, fmt.Errorf("ParseRule: MCP rule with parentheses is skipped by the harness: %q", trimmed)
	}

	kind := specKind(tool, spec)
	if kind == ruleKindDomain {
		spec = strings.TrimPrefix(spec, domainSpecPrefix)
	}
	rule := &Rule{
		Kind: kind,
		Raw:  trimmed,
		Spec: spec,
		Tool: tool,
	}
	return rule, nil
}

// Matches reports whether the rule covers this call.
func (r *Rule) Matches(toolName string, input *ToolInput) bool {
	switch r.Kind {
	case ruleKindTool:
		return toolName == r.Tool
	case ruleKindBashPrefix:
		return toolName == r.Tool && matchBashPattern(r.Spec, input.Command)
	case ruleKindName:
		return toolName == r.Tool && matchName(r.Spec, nameValue(toolName, input))
	case ruleKindDomain:
		return toolName == r.Tool && matchDomain(r.Spec, urlHost(input.Url))
	case ruleKindMcp:
		return matchMcp(r.Raw, toolName)
	case ruleKindPathPattern:
		return false
	case ruleKindParam:
		return false
	}
	return false
}

func NarrowestRule(toolName string, input *ToolInput, gitAdmittedFlags []string) string {
	switch toolName {
	case toolNameBash:
		return narrowestBashRule(input.Command, gitAdmittedFlags)
	case toolNameWebFetch:
		return toolNameWebFetch + "(" + domainSpecPrefix + urlHost(input.Url) + ")"
	case toolNameSkill:
		return toolNameSkill + "(" + input.Skill + ")"
	case toolNameAgent:
		return toolNameAgent + "(" + input.SubagentType + ")"
	}
	return toolName
}

func globMatch(pattern string, text string) bool {
	patternIndex, textIndex := 0, 0
	starIndex, starText := -1, 0
	for textIndex < len(text) {
		switch {
		case patternIndex < len(pattern) && pattern[patternIndex] == '*':
			starIndex = patternIndex
			starText = textIndex
			patternIndex++
		case patternIndex < len(pattern) && pattern[patternIndex] == text[textIndex]:
			patternIndex++
			textIndex++
		case starIndex >= 0:
			patternIndex = starIndex + 1
			starText++
			textIndex = starText
		default:
			return false
		}
	}
	for patternIndex < len(pattern) && pattern[patternIndex] == '*' {
		patternIndex++
	}
	return patternIndex == len(pattern)
}

// The harness skips an unanchored glob, so the server segment after "mcp__" never carries one.
func isMcpRuleName(name string) bool {
	if !strings.HasPrefix(name, mcpPrefix) {
		return false
	}
	rest := name[len(mcpPrefix):]
	server, toolPattern, hasTool := strings.Cut(rest, "__")
	if !isToolName(server) {
		return false
	}
	if !hasTool {
		return true
	}
	return isToolName(strings.ReplaceAll(toolPattern, "*", "x"))
}

func isToolName(name string) bool {
	if name == "" {
		return false
	}
	for index, char := range name {
		isLetter := (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
		isDigit := char >= '0' && char <= '9'
		isJoiner := char == '_' || char == '-'
		if index == 0 && !isLetter {
			return false
		}
		if !isLetter && !isDigit && !isJoiner {
			return false
		}
	}
	return true
}

func matchBashPattern(pattern string, command string) bool {
	if colonPrefix, hasColonForm := strings.CutSuffix(pattern, colonWildcard); hasColonForm {
		pattern = colonPrefix + trailingWildcard
	}
	if prefix, hasTrailingStar := strings.CutSuffix(pattern, trailingWildcard); hasTrailingStar {
		if command == prefix {
			return true
		}
	}
	return globMatch(pattern, command)
}

func matchDomain(pattern string, host string) bool {
	pattern = strings.TrimSuffix(strings.ToLower(pattern), ".")
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "" {
		return false
	}
	if pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		return strings.HasSuffix(host, pattern[1:])
	}

	patternLabels := strings.Split(pattern, ".")
	hostLabels := strings.Split(host, ".")
	if len(patternLabels) != len(hostLabels) {
		return false
	}
	for index, label := range patternLabels {
		if label == "*" {
			continue
		}
		if label != hostLabels[index] {
			return false
		}
	}
	return true
}

func matchMcp(rule string, toolName string) bool {
	if !strings.HasPrefix(toolName, mcpPrefix) {
		return false
	}
	if strings.Contains(rule, "*") {
		return globMatch(rule, toolName)
	}
	if toolName == rule {
		return true
	}
	isBareServer := !strings.Contains(rule[len(mcpPrefix):], "__")
	return isBareServer && strings.HasPrefix(toolName, rule+"__")
}

func matchName(spec string, value string) bool {
	if value == "" {
		return false
	}
	if prefix, hasColonForm := strings.CutSuffix(spec, colonWildcard); hasColonForm {
		return strings.HasPrefix(value, prefix)
	}
	return spec == value
}

func nameValue(toolName string, input *ToolInput) string {
	if toolName == toolNameAgent {
		return input.SubagentType
	}
	return input.Skill
}

func narrowestBashRule(command string, gitAdmittedFlags []string) string {
	fields := strings.Fields(gitRuleCommand(command, gitAdmittedFlags))
	if len(fields) == 0 {
		return toolNameBash
	}

	program := fields[0]
	for _, token := range fields[1:] {
		if strings.HasPrefix(token, "-") {
			continue
		}
		if isToolName(token) {
			return toolNameBash + "(" + program + " " + token + trailingWildcard + ")"
		}
		break
	}
	return toolNameBash + "(" + program + trailingWildcard + ")"
}

func newBareRule(name string) *Rule {
	kind := ruleKindTool
	if strings.HasPrefix(name, mcpPrefix) {
		kind = ruleKindMcp
	}
	rule := &Rule{
		Kind: kind,
		Raw:  name,
		Tool: name,
	}
	return rule
}

// specKind classifies a parenthesized spec by the tool that owns it.
func specKind(tool string, spec string) ruleKind {
	switch tool {
	case toolNameBash, "PowerShell":
		return ruleKindBashPrefix
	case toolNameWebFetch:
		if strings.HasPrefix(spec, domainSpecPrefix) {
			return ruleKindDomain
		}
		return ruleKindParam
	case toolNameSkill, toolNameAgent:
		if isParamSpec(spec) {
			return ruleKindParam
		}
		return ruleKindName
	case "Read", "Edit", "Write", "NotebookEdit", "Glob", "Grep", "MultiEdit":
		return ruleKindPathPattern
	}
	return ruleKindParam
}

func isParamSpec(spec string) bool {
	colon := strings.Index(spec, ":")
	if colon < 0 {
		return false
	}
	return spec[colon:] != colonWildcard
}

// urlHost extracts the lowercase hostname of a URL, empty when unparseable.
func urlHost(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}

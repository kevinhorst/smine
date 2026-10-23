package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRule(t *testing.T) {
	type testCase struct {
		_id           string
		_expectedKind ruleKind
		_shouldPass   bool

		raw string
	}

	tests := make([]*testCase, 0)

	// bare-tool
	tests = append(tests, &testCase{
		_id:           "bare-tool",
		_expectedKind: ruleKindTool,
		_shouldPass:   true,

		raw: "Bash",
	})

	// bash-prefix-space
	tests = append(tests, &testCase{
		_id:           "bash-prefix-space",
		_expectedKind: ruleKindBashPrefix,
		_shouldPass:   true,

		raw: "Bash(git * log*)",
	})

	// bash-prefix-colon
	tests = append(tests, &testCase{
		_id:           "bash-prefix-colon",
		_expectedKind: ruleKindBashPrefix,
		_shouldPass:   true,

		raw: "Bash(ls:*)",
	})

	// skill-colon-prefix
	tests = append(tests, &testCase{
		_id:           "skill-colon-prefix",
		_expectedKind: ruleKindName,
		_shouldPass:   true,

		raw: "Skill(update-config:*)",
	})

	// webfetch-domain-wildcard
	tests = append(tests, &testCase{
		_id:           "webfetch-domain-wildcard",
		_expectedKind: ruleKindDomain,
		_shouldPass:   true,

		raw: "WebFetch(domain:*.example.com)",
	})

	// mcp-bare-server
	tests = append(tests, &testCase{
		_id:           "mcp-bare-server",
		_expectedKind: ruleKindMcp,
		_shouldPass:   true,

		raw: "mcp__peek-mcp",
	})

	// mcp-exact-tool
	tests = append(tests, &testCase{
		_id:           "mcp-exact-tool",
		_expectedKind: ruleKindMcp,
		_shouldPass:   true,

		raw: "mcp__goland__search_symbol",
	})

	// mcp-tool-glob
	tests = append(tests, &testCase{
		_id:           "mcp-tool-glob",
		_expectedKind: ruleKindMcp,
		_shouldPass:   true,

		raw: "mcp__github__get_*",
	})

	// read-path-pattern
	tests = append(tests, &testCase{
		_id:           "read-path-pattern",
		_expectedKind: ruleKindPathPattern,
		_shouldPass:   true,

		raw: "Read(~/x/**)",
	})

	// agent-param-rule
	tests = append(tests, &testCase{
		_id:           "agent-param-rule",
		_expectedKind: ruleKindParam,
		_shouldPass:   true,

		raw: "Agent(model:opus)",
	})

	// fail-unbalanced
	tests = append(tests, &testCase{
		_id:         "fail-unbalanced",
		_shouldPass: false,

		raw: "Bash(",
	})

	// fail-mcp-parens
	tests = append(tests, &testCase{
		_id:         "fail-mcp-parens",
		_shouldPass: false,

		raw: "mcp__goland__search_symbol(x)",
	})

	// fail-empty
	tests = append(tests, &testCase{
		_id:         "fail-empty",
		_shouldPass: false,

		raw: "  ",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			rule, err := ParseRule(test.raw)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			if rule != nil {
				assert.Equal(t, test._expectedKind, rule.Kind)
			}
		})
	}
}

func TestRule_Matches(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		input    *ToolInput
		rule     string
		toolName string
	}

	tests := make([]*testCase, 0)

	// bash-trailing-star-bare-prefix
	tests = append(tests, &testCase{
		_id:       "bash-trailing-star-bare-prefix",
		_expected: true,

		input:    &ToolInput{Command: "npm run"},
		rule:     "Bash(npm run *)",
		toolName: "Bash",
	})

	// bash-trailing-star-args
	tests = append(tests, &testCase{
		_id:       "bash-trailing-star-args",
		_expected: true,

		input:    &ToolInput{Command: "npm run build"},
		rule:     "Bash(npm run *)",
		toolName: "Bash",
	})

	// bash-trailing-star-other-program
	tests = append(tests, &testCase{
		_id:       "bash-trailing-star-other-program",
		_expected: false,

		input:    &ToolInput{Command: "npm install"},
		rule:     "Bash(npm run *)",
		toolName: "Bash",
	})

	// bash-inner-star
	tests = append(tests, &testCase{
		_id:       "bash-inner-star",
		_expected: true,

		input:    &ToolInput{Command: "git -C x log --oneline"},
		rule:     "Bash(git * log*)",
		toolName: "Bash",
	})

	// bash-colon-star
	tests = append(tests, &testCase{
		_id:       "bash-colon-star",
		_expected: true,

		input:    &ToolInput{Command: "ls -la"},
		rule:     "Bash(ls:*)",
		toolName: "Bash",
	})

	// bash-merge-ff-only-matches
	tests = append(tests, &testCase{
		_id:       "bash-merge-ff-only-matches",
		_expected: true,

		input:    &ToolInput{Command: "git merge --ff-only login-simplification-pr-split/1-worker-move"},
		rule:     "Bash(git merge --ff-only *)",
		toolName: "Bash",
	})

	// bash-merge-ff-only-plain-merge
	tests = append(tests, &testCase{
		_id:       "bash-merge-ff-only-plain-merge",
		_expected: false,

		input:    &ToolInput{Command: "git merge login-simplification-pr-split/1-worker-move"},
		rule:     "Bash(git merge --ff-only *)",
		toolName: "Bash",
	})

	// bash-go-mod-vendor
	tests = append(tests, &testCase{
		_id:       "bash-go-mod-vendor",
		_expected: true,

		input:    &ToolInput{Command: "go mod vendor"},
		rule:     "Bash(go mod *)",
		toolName: "Bash",
	})

	// bash-rule-other-tool
	tests = append(tests, &testCase{
		_id:       "bash-rule-other-tool",
		_expected: false,

		input:    &ToolInput{Command: "ls -la"},
		rule:     "Bash(ls *)",
		toolName: "PowerShell",
	})

	// domain-subdomain
	tests = append(tests, &testCase{
		_id:       "domain-subdomain",
		_expected: true,

		input:    &ToolInput{Url: "https://api.example.com/x"},
		rule:     "WebFetch(domain:*.example.com)",
		toolName: "WebFetch",
	})

	// domain-subdomain-not-apex
	tests = append(tests, &testCase{
		_id:       "domain-subdomain-not-apex",
		_expected: false,

		input:    &ToolInput{Url: "https://example.com/x"},
		rule:     "WebFetch(domain:*.example.com)",
		toolName: "WebFetch",
	})

	// domain-inner-star-one-label
	tests = append(tests, &testCase{
		_id:       "domain-inner-star-one-label",
		_expected: true,

		input:    &ToolInput{Url: "https://example.org/"},
		rule:     "WebFetch(domain:example.*)",
		toolName: "WebFetch",
	})

	// domain-inner-star-no-dot-crossing
	tests = append(tests, &testCase{
		_id:       "domain-inner-star-no-dot-crossing",
		_expected: false,

		input:    &ToolInput{Url: "https://example.evil.com/"},
		rule:     "WebFetch(domain:example.*)",
		toolName: "WebFetch",
	})

	// domain-exact-case-insensitive
	tests = append(tests, &testCase{
		_id:       "domain-exact-case-insensitive",
		_expected: true,

		input:    &ToolInput{Url: "https://Code.Claude.com/docs"},
		rule:     "WebFetch(domain:code.claude.com)",
		toolName: "WebFetch",
	})

	// mcp-bare-server-any-tool
	tests = append(tests, &testCase{
		_id:       "mcp-bare-server-any-tool",
		_expected: true,

		input:    &ToolInput{},
		rule:     "mcp__goland",
		toolName: "mcp__goland__anything",
	})

	// mcp-bare-server-prefix-collision
	tests = append(tests, &testCase{
		_id:       "mcp-bare-server-prefix-collision",
		_expected: false,

		input:    &ToolInput{},
		rule:     "mcp__ccd_session",
		toolName: "mcp__ccd_session_mgmt__get_session",
	})

	// mcp-glob-match
	tests = append(tests, &testCase{
		_id:       "mcp-glob-match",
		_expected: true,

		input:    &ToolInput{},
		rule:     "mcp__goland__get_*",
		toolName: "mcp__goland__get_symbol_info",
	})

	// mcp-glob-miss
	tests = append(tests, &testCase{
		_id:       "mcp-glob-miss",
		_expected: false,

		input:    &ToolInput{},
		rule:     "mcp__goland__get_*",
		toolName: "mcp__goland__search_symbol",
	})

	// skill-exact
	tests = append(tests, &testCase{
		_id:       "skill-exact",
		_expected: true,

		input:    &ToolInput{Skill: "jq"},
		rule:     "Skill(jq)",
		toolName: "Skill",
	})

	// skill-exact-miss
	tests = append(tests, &testCase{
		_id:       "skill-exact-miss",
		_expected: false,

		input:    &ToolInput{Skill: "jqx"},
		rule:     "Skill(jq)",
		toolName: "Skill",
	})

	// skill-colon-prefix
	tests = append(tests, &testCase{
		_id:       "skill-colon-prefix",
		_expected: true,

		input:    &ToolInput{Skill: "update-config:add"},
		rule:     "Skill(update-config:*)",
		toolName: "Skill",
	})

	// agent-name
	tests = append(tests, &testCase{
		_id:       "agent-name",
		_expected: true,

		input:    &ToolInput{SubagentType: "Explore"},
		rule:     "Agent(Explore)",
		toolName: "Agent",
	})

	// path-pattern-never
	tests = append(tests, &testCase{
		_id:       "path-pattern-never",
		_expected: false,

		input:    &ToolInput{FilePath: "/Users/x/x/file"},
		rule:     "Read(~/x/**)",
		toolName: "Read",
	})

	// bare-tool
	tests = append(tests, &testCase{
		_id:       "bare-tool",
		_expected: true,

		input:    &ToolInput{},
		rule:     "WebSearch",
		toolName: "WebSearch",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			rule, err := ParseRule(test.rule)
			require.NoError(t, err)
			assert.Equal(t, test._expected, rule.Matches(test.toolName, test.input))
		})
	}
}

func TestNarrowestRule(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		input    *ToolInput
		toolName string
	}

	tests := make([]*testCase, 0)

	// bash-program-subcommand
	tests = append(tests, &testCase{
		_id:       "bash-program-subcommand",
		_expected: "Bash(brew list *)",

		input:    &ToolInput{Command: "brew list"},
		toolName: "Bash",
	})

	// bash-flag-before-subcommand
	tests = append(tests, &testCase{
		_id:       "bash-flag-before-subcommand",
		_expected: "Bash(gh pr *)",

		input:    &ToolInput{Command: "gh --version pr view 1"},
		toolName: "Bash",
	})

	// bash-git-C-candidate
	tests = append(tests, &testCase{
		_id:       "bash-git-C-candidate",
		_expected: "Bash(git add *)",

		input:    &ToolInput{Command: "git -C sub add -A"},
		toolName: "Bash",
	})

	// bash-git-plain-candidate
	tests = append(tests, &testCase{
		_id:       "bash-git-plain-candidate",
		_expected: "Bash(git fetch *)",

		input:    &ToolInput{Command: "git fetch origin"},
		toolName: "Bash",
	})

	// bash-program-only
	tests = append(tests, &testCase{
		_id:       "bash-program-only",
		_expected: "Bash(brew *)",

		input:    &ToolInput{Command: "brew"},
		toolName: "Bash",
	})

	// bash-path-argument-not-subcommand
	tests = append(tests, &testCase{
		_id:       "bash-path-argument-not-subcommand",
		_expected: "Bash(touch *)",

		input:    &ToolInput{Command: "touch /Users/x/Documents/cache/.probe"},
		toolName: "Bash",
	})

	// webfetch-host
	tests = append(tests, &testCase{
		_id:       "webfetch-host",
		_expected: "WebFetch(domain:example.org)",

		input:    &ToolInput{Url: "https://example.org/a"},
		toolName: "WebFetch",
	})

	// skill-name
	tests = append(tests, &testCase{
		_id:       "skill-name",
		_expected: "Skill(peek)",

		input:    &ToolInput{Skill: "peek"},
		toolName: "Skill",
	})

	// mcp-tool-string
	tests = append(tests, &testCase{
		_id:       "mcp-tool-string",
		_expected: "mcp__goland__replace_text_in_file",

		input:    &ToolInput{},
		toolName: "mcp__goland__replace_text_in_file",
	})

	admittedFlags := defaultToolpolicy(t).GitAdmittedFlags

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, NarrowestRule(test.toolName, test.input, admittedFlags))
		})
	}
}

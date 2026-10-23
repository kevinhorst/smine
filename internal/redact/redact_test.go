package redact

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedactSecrets(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		text string
	}

	tests := make([]*testCase, 0)

	// anthropic-key-redacted
	tests = append(tests, &testCase{
		_id:       "anthropic-key-redacted",
		_expected: "claude --key [REDACTED:anthropic-key]",

		text: "claude --key sk-ant-" + strings.Repeat("a", 20),
	})

	// api-key-redacted
	tests = append(tests, &testCase{
		_id:       "api-key-redacted",
		_expected: "openai [REDACTED:api-key]",

		text: "openai sk-" + strings.Repeat("b", 24),
	})

	// github-token-redacted
	tests = append(tests, &testCase{
		_id:       "github-token-redacted",
		_expected: "gh auth login [REDACTED:github-token]",

		text: "gh auth login ghp_" + strings.Repeat("a", 36),
	})

	// github-pat-redacted
	tests = append(tests, &testCase{
		_id:       "github-pat-redacted",
		_expected: "echo [REDACTED:github-token]",

		text: "echo github_pat_" + strings.Repeat("a", 22),
	})

	// aws-key-redacted
	tests = append(tests, &testCase{
		_id:       "aws-key-redacted",
		_expected: "aws configure [REDACTED:aws-key]",

		text: "aws configure AKIA" + "ABCDEFGHIJKLMNOP",
	})

	// slack-token-redacted
	tests = append(tests, &testCase{
		_id:       "slack-token-redacted",
		_expected: "post [REDACTED:slack-token]",

		text: "post xoxb-" + strings.Repeat("1", 12),
	})

	// google-key-redacted
	tests = append(tests, &testCase{
		_id:       "google-key-redacted",
		_expected: "maps [REDACTED:google-key]",

		text: "maps AIza" + strings.Repeat("a", 35),
	})

	// jwt-redacted
	tests = append(tests, &testCase{
		_id:       "jwt-redacted",
		_expected: "curl [REDACTED:jwt]",

		text: "curl eyJ" + strings.Repeat("a", 10) + ".eyJ" + strings.Repeat("b", 10) + "." + strings.Repeat("c", 10),
	})

	// private-key-block-redacted
	tests = append(tests, &testCase{
		_id:       "private-key-block-redacted",
		_expected: "key: [REDACTED:private-key] end",

		text: "key: -----BEGIN RSA PRIVATE KEY-----\nabc\n-----END RSA PRIVATE KEY----- end",
	})

	// url-credentials-keeps-user-and-host
	tests = append(tests, &testCase{
		_id:       "url-credentials-keeps-user-and-host",
		_expected: "git clone https://devuser:[REDACTED:url-credentials]@example.com/x.git",

		text: "git clone https://devuser:hunter2pass@example.com/x.git",
	})

	// auth-header-keeps-scheme
	tests = append(tests, &testCase{
		_id:       "auth-header-keeps-scheme",
		_expected: `curl -H "Authorization: Bearer [REDACTED:auth-header]" https://api.example.com`,

		text: `curl -H "Authorization: Bearer abc123def" https://api.example.com`,
	})

	// secret-flag-keeps-flag
	tests = append(tests, &testCase{
		_id:       "secret-flag-keeps-flag",
		_expected: "tool --token [REDACTED:secret-flag] run",

		text: "tool --token abc123 run",
	})

	// secret-assignment-keeps-name
	tests = append(tests, &testCase{
		_id:       "secret-assignment-keeps-name",
		_expected: "GH_TOKEN=[REDACTED:secret-assignment] gh pr list",

		text: "GH_TOKEN=abc123 gh pr list",
	})

	// quoted-assignment-redacted-whole
	tests = append(tests, &testCase{
		_id:       "quoted-assignment-redacted-whole",
		_expected: "PASSWORD=[REDACTED:secret-assignment] ./run",

		text: `PASSWORD="two words" ./run`,
	})

	// plain-command-unchanged
	tests = append(tests, &testCase{
		_id:       "plain-command-unchanged",
		_expected: "make audit",

		text: "make audit",
	})

	// home-path-unchanged
	tests = append(tests, &testCase{
		_id:       "home-path-unchanged",
		_expected: "ls /Users/devuser/GolandProjects",

		text: "ls /Users/devuser/GolandProjects",
	})

	// email-unchanged
	tests = append(tests, &testCase{
		_id:       "email-unchanged",
		_expected: "git log --author devuser@example.com",

		text: "git log --author devuser@example.com",
	})

	// already-redacted-idempotent
	tests = append(tests, &testCase{
		_id:       "already-redacted-idempotent",
		_expected: "GH_TOKEN=[REDACTED:secret-assignment] gh pr list",

		text: "GH_TOKEN=[REDACTED:secret-assignment] gh pr list",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, RedactSecrets(test.text))
		})
	}
}

func TestScan(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []Hit

		text string
	}

	tests := make([]*testCase, 0)

	// clean-directives-no-hits
	tests = append(tests, &testCase{
		_id:       "clean-directives-no-hits",
		_expected: nil,

		text: "- Prefer short sentences.\n- Use bullets for lists.",
	})

	// email-hit-line-3
	tests = append(tests, &testCase{
		_id:       "email-hit-line-3",
		_expected: []Hit{{Kind: "email", Line: 3}},

		text: "a\nb\nmail devuser@example.com",
	})

	// home-path-hit
	tests = append(tests, &testCase{
		_id:       "home-path-hit",
		_expected: []Hit{{Kind: "home-path", Line: 1}},

		text: "see /Users/devuser/notes",
	})

	// secret-hit-in-profile
	tests = append(tests, &testCase{
		_id:       "secret-hit-in-profile",
		_expected: []Hit{{Kind: "github-token", Line: 1}},

		text: "token ghp_" + strings.Repeat("a", 36),
	})

	// multiple-hits-sorted-by-line-then-kind
	tests = append(tests, &testCase{
		_id:       "multiple-hits-sorted-by-line-then-kind",
		_expected: []Hit{{Kind: "home-path", Line: 1}, {Kind: "email", Line: 2}, {Kind: "home-path", Line: 2}},

		text: "x /home/bob\ndevuser@example.com and /Users/al",
	})

	// withheld-stub-no-hits
	tests = append(tests, &testCase{
		_id:       "withheld-stub-no-hits",
		_expected: nil,

		text: "<!-- generated by /smine-style — a mining re-run rewrites this file; edit freely until then (Profile page or any editor) -->\n\n" +
			"No style profile is active: the last mining run produced a profile that matched the secret/PII check, so it was withheld. The next /smine-style run retries.\n",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, Scan(test.text))
		})
	}
}

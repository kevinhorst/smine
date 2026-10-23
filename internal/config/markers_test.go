package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMarkers_Expand(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		input string
	}

	markers := Markers{
		HeartbeatPort:     "6102",
		Home:              "/home/a",
		NotificationSound: "/System/Library/Sounds/Glass.aiff",
		PeekControlPort:   "42542",
		PeekMcp:           "/home/a/go/bin/peek-mcp",
		RepoDir:           "/home/a/src/smine",
		StopSound:         "/System/Library/Sounds/Ping.aiff",
	}
	tests := make([]*testCase, 0)

	// expands-repo-dir
	tests = append(tests, &testCase{
		_id:       "expands-repo-dir",
		_expected: `"toolpolicy -gate-repo /home/a/src/smine"`,

		input: `"toolpolicy -gate-repo {{REPO_DIR}}"`,
	})

	// expands-port
	tests = append(tests, &testCase{
		_id:       "expands-port",
		_expected: `"http://127.0.0.1:42542/otlp"`,

		input: `"http://127.0.0.1:{{PEEK_CONTROL_PORT}}/otlp"`,
	})

	// expands-home
	tests = append(tests, &testCase{
		_id:       "expands-home",
		_expected: `"/home/a/GolandProjects/smine"`,

		input: `"{{HOME}}/GolandProjects/smine"`,
	})

	// expands-peek-mcp
	tests = append(tests, &testCase{
		_id:       "expands-peek-mcp",
		_expected: `"/home/a/go/bin/peek-mcp"`,

		input: `"{{PEEK_MCP}}"`,
	})

	// unknown-markers-untouched
	tests = append(tests, &testCase{
		_id:       "unknown-markers-untouched",
		_expected: `{{UNKNOWN}}/x on 42542`,

		input: `{{UNKNOWN}}/x on {{PEEK_CONTROL_PORT}}`,
	})

	// expands-heartbeat-port
	tests = append(tests, &testCase{
		_id:       "expands-heartbeat-port",
		_expected: `"http://127.0.0.1:6102/hook"`,

		input: `"http://127.0.0.1:{{HEARTBEAT_PORT}}/hook"`,
	})

	// sound-markers
	tests = append(tests, &testCase{
		_id:       "sound-markers",
		_expected: `"play-sound.sh /System/Library/Sounds/Glass.aiff" "play-sound.sh /System/Library/Sounds/Ping.aiff"`,

		input: `"play-sound.sh {{NOTIFICATION_SOUND}}" "play-sound.sh {{STOP_SOUND}}"`,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			expanded := markers.Expand([]byte(test.input))
			assert.Equal(t, test._expected, string(expanded))
		})
	}
}

func TestMarkers_Collapse(t *testing.T) {
	type testCase struct {
		_id          string
		_expected    string
		_expectedErr bool

		live     string
		markers  Markers
		template string
	}

	markers := Markers{Home: "/home/a", PeekControlPort: "42542", PeekMcp: "/home/a/go/bin/peek-mcp", StopSound: "/System/Library/Sounds/Ping.aiff"}
	tests := make([]*testCase, 0)

	// port-literal-restored
	tests = append(tests, &testCase{
		_id:          "port-literal-restored",
		_expected:    `{"url": "http://127.0.0.1:{{PEEK_CONTROL_PORT}}/otlp"}`,
		_expectedErr: false,

		live:     `{"url": "http://127.0.0.1:42542/otlp"}`,
		markers:  markers,
		template: `{"url": "http://127.0.0.1:{{PEEK_CONTROL_PORT}}/otlp"}`,
	})

	// changed-port-stays-literal
	tests = append(tests, &testCase{
		_id:          "changed-port-stays-literal",
		_expected:    `{"url": "http://127.0.0.1:9999/otlp", "n": 42542}`,
		_expectedErr: false,

		live:     `{"url": "http://127.0.0.1:9999/otlp", "n": 42542}`,
		markers:  markers,
		template: `{"url": "http://127.0.0.1:{{PEEK_CONTROL_PORT}}/otlp"}`,
	})

	// home-prefix-restored
	tests = append(tests, &testCase{
		_id:          "home-prefix-restored",
		_expected:    `["{{HOME}}/GolandProjects/smine", "{{HOME}}"]`,
		_expectedErr: false,

		live:     `["/home/a/GolandProjects/smine", "/home/a"]`,
		markers:  markers,
		template: `["{{HOME}}/GolandProjects/smine"]`,
	})

	// extra-home-dir-collapsed
	tests = append(tests, &testCase{
		_id:          "extra-home-dir-collapsed",
		_expected:    `["{{HOME}}/new/dir"]`,
		_expectedErr: false,

		live:     `["/home/a/new/dir"]`,
		markers:  markers,
		template: `[]`,
	})

	// peek-mcp-restored
	tests = append(tests, &testCase{
		_id:          "peek-mcp-restored",
		_expected:    `command = "{{PEEK_MCP}}"`,
		_expectedErr: false,

		live:     `command = "/home/a/go/bin/peek-mcp"`,
		markers:  markers,
		template: ``,
	})

	// home-lookalike-untouched
	tests = append(tests, &testCase{
		_id:          "home-lookalike-untouched",
		_expected:    `["/home/a2/x"]`,
		_expectedErr: false,

		live:     `["/home/a2/x"]`,
		markers:  markers,
		template: `[]`,
	})

	// literal-marker-in-live-refused
	tests = append(tests, &testCase{
		_id:          "literal-marker-in-live-refused",
		_expected:    "",
		_expectedErr: true,

		live:     `["{{HOME}}/x"]`,
		markers:  markers,
		template: `[]`,
	})

	// empty-home-refused
	tests = append(tests, &testCase{
		_id:          "empty-home-refused",
		_expected:    "",
		_expectedErr: true,

		live:     `{}`,
		markers:  Markers{PeekControlPort: "42542"},
		template: `{}`,
	})

	// no-markers-identity
	tests = append(tests, &testCase{
		_id:          "no-markers-identity",
		_expected:    `{"model": "live"}`,
		_expectedErr: false,

		live:     `{"model": "live"}`,
		markers:  markers,
		template: `{"model": "fragment"}`,
	})

	// sound-marker-restored
	tests = append(tests, &testCase{
		_id:          "sound-marker-restored",
		_expected:    `{"command": "play-sound.sh {{STOP_SOUND}}"}`,
		_expectedErr: false,

		live:     `{"command": "play-sound.sh /System/Library/Sounds/Ping.aiff"}`,
		markers:  markers,
		template: `{"command": "play-sound.sh {{STOP_SOUND}}"}`,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			collapsed, err := test.markers.Collapse([]byte(test.live), []byte(test.template))
			assert.Equal(t, test._expectedErr, err != nil, err)
			assert.Equal(t, test._expected, string(collapsed))
		})
	}
}

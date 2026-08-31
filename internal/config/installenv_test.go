package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandInstallMarkers(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		installEnv string
		input      string
	}

	tests := make([]*testCase, 0)

	// expands-from-install-env
	tests = append(tests, &testCase{
		_id:       "expands-from-install-env",
		_expected: `"http://127.0.0.1:42542/otlp"`,

		installEnv: "CONFIGSERVER_PORT=6002\nPEEK_CONTROL_PORT=42542\n",
		input:      `"http://127.0.0.1:{{PEEK_CONTROL_PORT}}/otlp"`,
	})

	// missing-file-uses-default
	tests = append(tests, &testCase{
		_id:       "missing-file-uses-default",
		_expected: `"http://127.0.0.1:42442/otlp"`,

		installEnv: "",
		input:      `"http://127.0.0.1:{{PEEK_CONTROL_PORT}}/otlp"`,
	})

	// unknown-markers-untouched
	tests = append(tests, &testCase{
		_id:       "unknown-markers-untouched",
		_expected: `{{HOME}}/x on 42442`,

		installEnv: "",
		input:      `{{HOME}}/x on {{PEEK_CONTROL_PORT}}`,
	})

	// quoted-value-unquoted
	tests = append(tests, &testCase{
		_id:       "quoted-value-unquoted",
		_expected: "42542",

		installEnv: "PEEK_CONTROL_PORT=\"42542\"\n",
		input:      "{{PEEK_CONTROL_PORT}}",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if test.installEnv != "" {
				require.NoError(t, os.WriteFile("install.env", []byte(test.installEnv), 0o644))
			}

			expanded := ExpandInstallMarkers([]byte(test.input))
			assert.Equal(t, test._expected, string(expanded))
		})
	}
}

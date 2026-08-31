package routines

import (
	"maps"
	"testing"
)

// Fixture captured from `launchctl print-disabled gui/502` on this machine
// (2026-07-30) — the shape overrideLabels is written against (D8).
const printDisabledFixture = `	disabled services = {
		"com.docker.helper" => enabled
		"io.tailscale.ipn.macsys.login-item-helper" => enabled
		"com.apple.Siri.agent" => disabled
		"com.smine.routine.smine-nightly" => disabled
		"2BUA8C4S2C.com.1password.browser-helper" => enabled
	}
`

func TestOverrideLabels(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   map[string]string
	}{
		{
			name:   "probe fixture: enabled and disabled labels both captured",
			output: printDisabledFixture,
			want: map[string]string{
				"com.docker.helper":                         "enabled",
				"io.tailscale.ipn.macsys.login-item-helper": "enabled",
				"com.apple.Siri.agent":                      "disabled",
				"com.smine.routine.smine-nightly":           "disabled",
				"2BUA8C4S2C.com.1password.browser-helper":   "enabled",
			},
		},
		{
			name:   "empty output",
			output: "",
			want:   map[string]string{},
		},
		{
			name:   "unquoted line ignored",
			output: "com.example.btm => disabled\n",
			want:   map[string]string{},
		},
		{
			name:   "unknown state ignored",
			output: `"com.example.btm" => pending` + "\n",
			want:   map[string]string{},
		},
		{
			name:   "arrow inside quotes not falsely matched",
			output: `"com.example.\"=> disabled\".x" => enabled` + "\n",
			want:   map[string]string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := overrideLabels(tc.output)
			if !maps.Equal(got, tc.want) {
				t.Errorf("overrideLabels() = %v, want %v", got, tc.want)
			}
		})
	}
}

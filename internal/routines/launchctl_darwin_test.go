package routines

import (
	"maps"
	"testing"
)

// Fixture captured from `launchctl print-disabled gui/502` on this machine
// (2026-07-30) — the shape disabledLabels is written against (D8).
const printDisabledFixture = `	disabled services = {
		"com.docker.helper" => enabled
		"io.tailscale.ipn.macsys.login-item-helper" => enabled
		"com.apple.Siri.agent" => disabled
		"com.smine.routine.smine-nightly" => disabled
		"2BUA8C4S2C.com.1password.browser-helper" => enabled
	}
`

func TestDisabledLabels(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   map[string]bool
	}{
		{
			name:   "probe fixture: only disabled labels, enabled ignored",
			output: printDisabledFixture,
			want: map[string]bool{
				"com.apple.Siri.agent":            true,
				"com.smine.routine.smine-nightly": true,
			},
		},
		{
			name:   "empty output",
			output: "",
			want:   map[string]bool{},
		},
		{
			name:   "unquoted line ignored",
			output: "com.example.btm => disabled\n",
			want:   map[string]bool{},
		},
		{
			name:   "arrow inside quotes not falsely matched",
			output: `"com.example.\"=> disabled\".x" => enabled` + "\n",
			want:   map[string]bool{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := disabledLabels(tc.output)
			if !maps.Equal(got, tc.want) {
				t.Errorf("disabledLabels() = %v, want %v", got, tc.want)
			}
		})
	}
}

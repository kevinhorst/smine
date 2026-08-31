package config

import (
	"os"
	"strings"
)

const (
	DefaultPeekControlPort = "42442"

	installEnvPath = "install.env"
)

// ExpandInstallMarkers replaces per-install markers ({{PEEK_CONTROL_PORT}})
// in a repo fragment with the values from install.env in the working
// directory — the same expansion sync_settings.sh applies, so fragment↔live
// comparison and revert see the installed values, not the markers.
func ExpandInstallMarkers(data []byte) []byte {
	values := installEnvValues(installEnvPath)
	expanded := strings.ReplaceAll(string(data), "{{PEEK_CONTROL_PORT}}", values["PEEK_CONTROL_PORT"])
	return []byte(expanded)
}

func installEnvValues(path string) map[string]string {
	values := map[string]string{"PEEK_CONTROL_PORT": DefaultPeekControlPort}
	data, err := os.ReadFile(path)
	if err != nil {
		return values
	}

	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		key, value, ok := strings.Cut(trimmed, "=")
		if !ok || strings.HasPrefix(key, "#") {
			continue
		}
		values[key] = strings.Trim(value, `"`)
	}
	return values
}

package peek

import (
	"path/filepath"
	"strconv"
)

// A stdio peek watches 3 days unless told otherwise; mining looks two weeks back.
const MiningWindowDays = 14

// No telemetry port: a private stdio peek over both agent homes.
func StdioArgs(home string, windowDays int) []string {
	return []string{
		"start",
		"--transport=stdio",
		"--depth=100",
		"--control-port=0",
		"--claude-home=" + filepath.Join(home, ".claude"),
		"--codex-home=" + filepath.Join(home, ".codex"),
		WatchWindowArg(windowDays),
	}
}

// WatchWindowArg is the peek start flag that sets an instance's watch window.
func WatchWindowArg(days int) string {
	return "--watch-window-days=" + strconv.Itoa(days)
}

package install

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
)

// Values is the typed view of the install settings and marker values the record carries or defaults: what config.InstallEnv was.
type Values struct {
	ConfigServerPort   string
	CountTokens        string
	HeartbeatPort      string
	InitWelcome        bool
	NotificationSound  string
	PeekClaudeHome     string
	PeekCodexHome      string
	PeekControlPort    string
	PeekPort           string
	ServerPath         string
	StopSound          string
	TrashRetentionDays int
}

// Markers is the deploy-time marker table the fragments and copied files expand with; the repo dir takes forward slashes like the home, since the values land in JSON strings and bash hook commands.
func (v Values) Markers(home, repoDir string) config.Markers {
	return config.Markers{
		HeartbeatPort:     v.HeartbeatPort,
		Home:              home,
		NotificationSound: v.NotificationSound,
		PeekControlPort:   v.PeekControlPort,
		PeekMcp:           config.PeekMcpPath(home),
		RepoDir:           filepath.ToSlash(repoDir),
		StopSound:         v.StopSound,
	}
}

// PeekEndpoint is the MCP endpoint of this install's central peek.
func (v Values) PeekEndpoint() string {
	return "http://127.0.0.1:" + v.PeekPort + "/mcp"
}

// ClearOverride removes component.key from the record, dropping an emptied component map.
func ClearOverride(record *Record, id components.ID, key string) {
	delete(record.Settings[id], key)
	if len(record.Settings[id]) == 0 {
		delete(record.Settings, id)
	}
	if len(record.Settings) == 0 {
		record.Settings = nil
	}
}

// InstallValues resolves the install sink rows of the record into the typed view.
func InstallValues(record Record) (Values, error) {
	values := Values{}
	read := func(id components.ID, key string) (components.Value, error) {
		return Resolve(record, id, key)
	}
	port, err := read(components.ConfigServer, components.PortKey)
	if err != nil {
		return values, fmt.Errorf("InstallValues: %w", err)
	}
	values.ConfigServerPort = strconv.Itoa(port.Int)

	welcome, err := read(components.ConfigServer, "init_welcome")
	if err != nil {
		return values, fmt.Errorf("InstallValues: %w", err)
	}
	values.InitWelcome = welcome.Bool

	serverPath, err := read(components.ConfigServer, "server_path")
	if err != nil {
		return values, fmt.Errorf("InstallValues: %w", err)
	}
	values.ServerPath = cmp.Or(serverPath.Text, os.Getenv("PATH"))

	peekPort, err := read(components.Peek, components.PortKey)
	if err != nil {
		return values, fmt.Errorf("InstallValues: %w", err)
	}
	values.PeekPort = strconv.Itoa(peekPort.Int)

	controlPort, err := read(components.Peek, "control_port")
	if err != nil {
		return values, fmt.Errorf("InstallValues: %w", err)
	}
	values.PeekControlPort = strconv.Itoa(controlPort.Int)

	peekClaudeHome, err := read(components.Peek, "claude_home")
	if err != nil {
		return values, fmt.Errorf("InstallValues: %w", err)
	}
	values.PeekClaudeHome = peekClaudeHome.Text

	peekCodexHome, err := read(components.Peek, "codex_home")
	if err != nil {
		return values, fmt.Errorf("InstallValues: %w", err)
	}
	values.PeekCodexHome = peekCodexHome.Text

	heartbeatPort, err := read(components.Heartbeat, components.PortKey)
	if err != nil {
		return values, fmt.Errorf("InstallValues: %w", err)
	}
	values.HeartbeatPort = strconv.Itoa(heartbeatPort.Int)

	countTokens, err := read(components.Routines, "count_tokens")
	if err != nil {
		return values, fmt.Errorf("InstallValues: %w", err)
	}
	values.CountTokens = countTokens.Text

	notificationSound, err := read(components.SessionHelpers, "notification_sound")
	if err != nil {
		return values, fmt.Errorf("InstallValues: %w", err)
	}
	values.NotificationSound = notificationSound.Text

	stopSound, err := read(components.SessionHelpers, "stop_sound")
	if err != nil {
		return values, fmt.Errorf("InstallValues: %w", err)
	}
	values.StopSound = stopSound.Text

	retention, err := read(components.ConfigServer, "trash_retention_days")
	if err != nil {
		return values, fmt.Errorf("InstallValues: %w", err)
	}
	values.TrashRetentionDays = retention.Int
	return values, nil
}

// IsOverridden reports whether the record sets component.key.
func IsOverridden(record Record, id components.ID, key string) bool {
	_, isSet := record.Settings[id][key]
	return isSet
}

// Resolve is one setting's effective value: the record's override, else the catalog default; a group row's default lives in the fragment (LookupRow).
func Resolve(record Record, id components.ID, key string) (components.Value, error) {
	setting, isStatic := components.LookupSetting(id, key)
	if !isStatic {
		row, isGroup := groupRow(key)
		if !isGroup {
			return components.Value{}, fmt.Errorf("Resolve: Unknown setting %s.%s", id, key)
		}
		setting = row
	}

	if raw, isSet := record.Settings[id][key]; isSet {
		value, err := components.ParseJson(&setting, raw)
		if err != nil {
			return components.Value{}, fmt.Errorf("Resolve: %w", err)
		}
		return value, nil
	}

	if !isStatic {
		return components.Value{}, fmt.Errorf("Resolve: %s.%s takes its default from the fragment — read it through LookupRow", id, key)
	}
	value, err := components.ParseText(&setting, setting.Default)
	if err != nil {
		return components.Value{}, fmt.Errorf("Resolve: %w", err)
	}
	return value, nil
}

// SetOverride stores a parsed value in the record; a static row's value equal to the default clears the override.
func SetOverride(record *Record, id components.ID, key string, value components.Value) error {
	setting, isStatic := components.LookupSetting(id, key)
	_, isGroup := groupRow(key)
	if !isStatic && !isGroup {
		return fmt.Errorf("SetOverride: Unknown setting %s.%s", id, key)
	}

	if isStatic {
		defaultValue, err := components.ParseText(&setting, setting.Default)
		if err != nil {
			return fmt.Errorf("SetOverride: %w", err)
		}
		if value.String() == defaultValue.String() {
			ClearOverride(record, id, key)
			return nil
		}
	}

	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("SetOverride: %w", err)
	}
	if record.Settings == nil {
		record.Settings = Settings{}
	}
	if record.Settings[id] == nil {
		record.Settings[id] = map[string]json.RawMessage{}
	}
	record.Settings[id][key] = raw
	return nil
}

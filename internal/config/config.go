package config

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/kevinhorst/smine/internal/fsx"
)

type Hook struct {
	Async   bool   `json:"async,omitempty"`
	Command string `json:"command"`
	Name    string `json:"name,omitempty"`
	Type    string `json:"type"`
}

type HookGroup struct {
	Hooks   []Hook  `json:"hooks"`
	Matcher *string `json:"matcher,omitempty"` // plain tool-name pattern, e.g. "Read" or "Write|Edit"
}

type Permissions struct {
	AdditionalDirectories []string `json:"additionalDirectories,omitempty"`
	Allow                 []string `json:"allow,omitempty"`
	Ask                   []string `json:"ask,omitempty"`
}

// Settings wraps the settings.json document. Typed views decode from and
// encode into the document per call; keys the views do not model survive
// untouched with their order preserved.
type Settings struct {
	doc *Document
}

func NewSettings() *Settings {
	return &Settings{doc: NewDocument()}
}

func (s *Settings) Doc() *Document {
	return s.doc
}

func (s *Settings) get(key string, out any) error {
	raw, ok := s.doc.Get([]string{key})
	if !ok {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("Settings.get: Failed to decode key %s: %w", key, err)
	}
	return nil
}

func (s *Settings) set(key string, value any, isZero bool) error {
	if isZero {
		s.doc.Unset([]string{key})
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("Settings.set: Failed to encode key %s: %w", key, err)
	}
	return s.doc.Set([]string{key}, raw)
}

func (s *Settings) Hooks() (map[string][]HookGroup, error) {
	var hooks map[string][]HookGroup
	if err := s.get("hooks", &hooks); err != nil {
		return nil, err
	}
	return hooks, nil
}

func (s *Settings) SetHooks(hooks map[string][]HookGroup) error {
	return s.set("hooks", hooks, len(hooks) == 0)
}

func (s *Settings) Env() (map[string]string, error) {
	var env map[string]string
	if err := s.get("env", &env); err != nil {
		return nil, err
	}
	return env, nil
}

func (s *Settings) SetEnv(env map[string]string) error {
	return s.set("env", env, len(env) == 0)
}

func (s *Settings) Model() (string, error) {
	var model string
	if err := s.get("model", &model); err != nil {
		return "", err
	}
	return model, nil
}

func (s *Settings) SetModel(model string) error {
	return s.set("model", model, model == "")
}

func (s *Settings) Permissions() (Permissions, error) {
	var perms Permissions
	if err := s.get("permissions", &perms); err != nil {
		return Permissions{}, err
	}
	return perms, nil
}

func (s *Settings) SetPermissions(perms Permissions) error {
	return s.set("permissions", perms, len(perms.Allow) == 0 && len(perms.Ask) == 0)
}

func (s *Settings) DisabledMcpjsonServers() ([]string, error) {
	var servers []string
	if err := s.get("disabledMcpjsonServers", &servers); err != nil {
		return nil, err
	}
	return servers, nil
}

func (s *Settings) SetDisabledMcpjsonServers(servers []string) error {
	return s.set("disabledMcpjsonServers", servers, len(servers) == 0)
}

// McpServerNames lists the MCP server names registered in Claude Code's
// ~/.claude.json (top-level mcpServers keys). A missing file yields nil.
// MCP bundles, plugins, and claude_desktop_settings.json are not consulted.
func McpServerNames(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("McpServerNames: Failed to read %s: %w", path, err)
	}

	var file struct {
		McpServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("McpServerNames: Failed to parse %s: %w", path, err)
	}
	return slices.Sorted(maps.Keys(file.McpServers)), nil
}

// McpServer returns one registered server's command and args from Claude
// Code's ~/.claude.json. A missing file or unregistered name is an error —
// callers probing config health want the reason, not an empty value.
func McpServer(path, name string) (string, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("McpServer: Failed to read %s: %w", path, err)
	}

	var file mcpServerFile
	if err := json.Unmarshal(data, &file); err != nil {
		return "", nil, fmt.Errorf("McpServer: Failed to parse %s: %w", path, err)
	}
	server, ok := file.McpServers[name]
	if !ok {
		return "", nil, fmt.Errorf("McpServer: No %q entry in %s", name, path)
	}
	return server.Command, server.Args, nil
}

type mcpServerEntry struct {
	Args    []string `json:"args"`
	Command string   `json:"command"`
}

type mcpServerFile struct {
	McpServers map[string]mcpServerEntry `json:"mcpServers"`
}

func DefaultClaudeJsonPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude.json")
}

func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

func DisabledPath(settingsPath string) string {
	dir := filepath.Dir(settingsPath)
	return filepath.Join(dir, "settings.disabled.json")
}

func Load(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewSettings(), nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	s := NewSettings()
	if err := json.Unmarshal(data, s.doc); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return s, nil
}

// LoadFragment loads a repo settings fragment, expanding the per-install
// markers a fragment may carry (live settings files never do — use Load).
func LoadFragment(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewSettings(), nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	s := NewSettings()
	if err := json.Unmarshal(ExpandInstallMarkers(data), s.doc); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return s, nil
}

func Save(path string, s *Settings) error {
	data, err := json.MarshalIndent(s.doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}
	data = append(data, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", tmp, err)
	}
	if err := fsx.ReplaceFile(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("renaming %s to %s: %w", tmp, path, err)
	}
	return nil
}

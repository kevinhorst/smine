package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/kevinhorst/smine/internal/semver"
)

const (
	BaseSettingsFile     = "base.json"
	ComponentFragmentDir = "components"
	permissionsKey       = "permissions"
	zeroHookVersion      = "0.0.0"
)

// Fragment is one component's marker-expanded settings share; hook groups and MCP servers stay raw so Claude Code's unknown keys survive a round trip.
type Fragment struct {
	Env         map[string]string            `json:"env,omitempty"`
	Hooks       map[string][]json.RawMessage `json:"hooks,omitempty"`
	McpServers  map[string]json.RawMessage   `json:"mcpServers,omitempty"`
	Permissions FragmentPermissions          `json:"permissions"`
}

// mergeInto leaves MCP servers out since they belong to ~/.claude.json, not settings.json.
func (f *Fragment) mergeInto(doc *Document) error {
	for _, event := range slices.Sorted(maps.Keys(f.Hooks)) {
		if err := appendList(doc, []string{"hooks", event}, f.Hooks[event]); err != nil {
			return err
		}
	}

	for _, list := range f.Permissions.Lists() {
		if err := appendList(doc, list.path, RawStrings(list.values)); err != nil {
			return err
		}
	}

	for _, key := range slices.Sorted(maps.Keys(f.Env)) {
		value, err := json.Marshal(f.Env[key])
		if err != nil {
			return fmt.Errorf("Fragment.mergeInto: %w", err)
		}
		if err := doc.Set([]string{"env", key}, value); err != nil {
			return fmt.Errorf("Fragment.mergeInto: %w", err)
		}
	}
	return nil
}

// validateHooks requires every group to carry an id prefixed by its component, a three-part version above 0.0.0 and a name within the cap.
func (f *Fragment) validateHooks(component string) error {
	for _, event := range slices.Sorted(maps.Keys(f.Hooks)) {
		for index, raw := range f.Hooks[event] {
			if err := validateHookGroup(component, event, index, raw); err != nil {
				return err
			}
		}
	}
	return nil
}

// HookGroups returns every hook group decoded, in event order.
func (f *Fragment) HookGroups() ([]HookGroup, error) {
	var groups []HookGroup
	for _, event := range slices.Sorted(maps.Keys(f.Hooks)) {
		for _, raw := range f.Hooks[event] {
			var group HookGroup
			if err := json.Unmarshal(raw, &group); err != nil {
				return nil, fmt.Errorf("Fragment.HookGroups: hooks.%s: %w", event, err)
			}
			groups = append(groups, group)
		}
	}
	return groups, nil
}

// HookIds returns every hook group's id in event order.
func (f *Fragment) HookIds() ([]string, error) {
	var ids []string
	for _, event := range slices.Sorted(maps.Keys(f.Hooks)) {
		for _, raw := range f.Hooks[event] {
			var group HookGroup
			if err := json.Unmarshal(raw, &group); err != nil {
				return nil, fmt.Errorf("Fragment.HookIds: hooks.%s: %w", event, err)
			}
			ids = append(ids, group.Id)
		}
	}
	return ids, nil
}

// FragmentPermissions are the permission entries a fragment owns one by one.
type FragmentPermissions struct {
	AdditionalDirectories []string `json:"additionalDirectories,omitempty"`
	Allow                 []string `json:"allow,omitempty"`
	Deny                  []string `json:"deny,omitempty"`
}

// Lists returns the three owned lists in a fixed order.
func (p FragmentPermissions) Lists() []PermissionList {
	lists := []PermissionList{
		{path: []string{permissionsKey, "additionalDirectories"}, values: p.AdditionalDirectories},
		{path: []string{permissionsKey, "allow"}, values: p.Allow},
		{path: []string{permissionsKey, "deny"}, values: p.Deny},
	}
	return lists
}

// PermissionList is one owned list and where it sits in settings.json.
type PermissionList struct {
	path   []string
	values []string
}

// Path is the settings.json path of the list.
func (l PermissionList) Path() []string {
	return l.path
}

// Values are the entries the fragment owns in the list.
func (l PermissionList) Values() []string {
	return l.values
}

// appendList appends values to the array at path.
func appendList(doc *Document, path []string, values []json.RawMessage) error {
	if len(values) == 0 {
		return nil
	}

	list, err := doc.List(path)
	if err != nil {
		return fmt.Errorf("appendList: %w", err)
	}
	return doc.SetList(path, append(list, values...))
}

// readFragment reads the component fragment at path; the component is the file's stem.
func readFragment(path string, markers Markers) (*Fragment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("readFragment: %w", err)
	}

	component := strings.TrimSuffix(filepath.Base(path), ".json")
	fragment, err := ParseFragment(data, component, markers)
	if err != nil {
		return nil, fmt.Errorf("readFragment: %s: %w", path, err)
	}
	return fragment, nil
}

// ParseFragment decodes one component's fragment, markers expanded; an unknown top-level key is refused so a typo never ships silently.
func ParseFragment(data []byte, component string, markers Markers) (*Fragment, error) {
	decoder := json.NewDecoder(bytes.NewReader(markers.Expand(data)))
	decoder.DisallowUnknownFields()
	fragment := &Fragment{}
	if err := decoder.Decode(fragment); err != nil {
		return nil, fmt.Errorf("ParseFragment: Failed to parse: %w", err)
	}

	if err := fragment.validateHooks(component); err != nil {
		return nil, fmt.Errorf("ParseFragment: %w", err)
	}
	return fragment, nil
}

func validateHookGroup(component, event string, index int, raw json.RawMessage) error {
	var group HookGroup
	if err := json.Unmarshal(raw, &group); err != nil {
		return fmt.Errorf("Fragment.validateHooks: hooks.%s[%d]: %w", event, index, err)
	}

	// smineId
	prefix := component + "-"
	if !strings.HasPrefix(group.Id, prefix) {
		return fmt.Errorf("Fragment.validateHooks: hooks.%s[%d]: smineId %q must start with %q", event, index, group.Id, prefix)
	}

	// smineVersion
	components, err := semver.Parse(string(group.Version))
	if err != nil {
		return fmt.Errorf("Fragment.validateHooks: hooks.%s[%d]: smineVersion: %w", event, index, err)
	}
	if len(components) != semver.Parts {
		return fmt.Errorf("Fragment.validateHooks: hooks.%s[%d]: smineVersion %q must have three components", event, index, group.Version)
	}
	order, err := group.Version.Compare(zeroHookVersion)
	if err != nil {
		return fmt.Errorf("Fragment.validateHooks: hooks.%s[%d]: %w", event, index, err)
	}
	if order <= 0 {
		return fmt.Errorf("Fragment.validateHooks: hooks.%s[%d]: smineVersion must be above %s", event, index, zeroHookVersion)
	}

	// smineName
	nameRunes := utf8.RuneCountInString(group.Name)
	if nameRunes == 0 || nameRunes > MaxHookNameRunes {
		return fmt.Errorf("Fragment.validateHooks: hooks.%s[%d]: smineName must be 1 to %d runes, got %d", event, index, MaxHookNameRunes, nameRunes)
	}
	return nil
}

// LoadBaseSettings reads <settingsDir>/base.json expanded.
func LoadBaseSettings(settingsDir string, markers Markers) (*Settings, error) {
	data, err := os.ReadFile(filepath.Join(settingsDir, BaseSettingsFile))
	if errors.Is(err, fs.ErrNotExist) {
		return NewSettings(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("LoadBaseSettings: %w", err)
	}

	settings := NewSettings()
	if err := json.Unmarshal(markers.Expand(data), settings.doc); err != nil {
		return nil, fmt.Errorf("LoadBaseSettings: Failed to parse %s: %w", BaseSettingsFile, err)
	}
	return settings, nil
}

// LoadComponentFragment returns an empty fragment for a component without a file, since it owns no settings entry.
func LoadComponentFragment(settingsDir, id string, markers Markers) (*Fragment, error) {
	path := filepath.Join(settingsDir, ComponentFragmentDir, id+".json")
	fragment, err := readFragment(path, markers)
	if errors.Is(err, fs.ErrNotExist) {
		return &Fragment{}, nil
	}
	return fragment, err
}

// RawStrings encodes each string as one JSON value.
func RawStrings(values []string) []json.RawMessage {
	raws := make([]json.RawMessage, 0, len(values))
	for _, value := range values {
		encoded, _ := json.Marshal(value)
		raws = append(raws, encoded)
	}
	return raws
}

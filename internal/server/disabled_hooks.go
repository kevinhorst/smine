package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/kevinhorst/smine/internal/config"
)

// disabledHookStore parks toggled-off hook groups in memory — settings.json
// stays the only persistent hook store (D33). A hard death loses the content;
// a graceful shutdown round-trips it through the tmp file.
type disabledHookStore struct {
	// mu guards groups.
	mu     sync.Mutex
	groups map[string][]config.HookGroup
}

func newDisabledHookStore() *disabledHookStore {
	return &disabledHookStore{groups: make(map[string][]config.HookGroup)}
}

func (d *disabledHookStore) Add(event string, group config.HookGroup) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.groups[event] = append(d.groups[event], group)
}

// Pop removes and returns the group at event/index; false when absent.
func (d *disabledHookStore) Pop(event string, index int) (config.HookGroup, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	groups, ok := d.groups[event]
	if !ok || index >= len(groups) {
		return config.HookGroup{}, false
	}

	group := groups[index]
	d.groups[event] = slices.Delete(groups, index, index+1)
	if len(d.groups[event]) == 0 {
		delete(d.groups, event)
	}
	return group, true
}

// Clear drops every parked group — a claude Revert makes the repo fragment
// the whole truth, so parked drift must not survive it.
func (d *disabledHookStore) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.groups = make(map[string][]config.HookGroup)
}

// Snapshot returns a rendering copy.
func (d *disabledHookStore) Snapshot() map[string][]config.HookGroup {
	d.mu.Lock()
	defer d.mu.Unlock()

	snapshot := make(map[string][]config.HookGroup, len(d.groups))
	for event, groups := range d.groups {
		snapshot[event] = slices.Clone(groups)
	}
	return snapshot
}

// disabledHooksTmpPath sits next to settings.json; the file exists only
// between a graceful shutdown and the next boot (D33).
func disabledHooksTmpPath(settingsPath string) string {
	return filepath.Join(filepath.Dir(settingsPath), ".disabled-hooks.tmp.json")
}

// restoreDisabledHooks builds the boot store: the shutdown tmp file (consumed
// and deleted) plus a one-time absorption of the sidecar's hooks key — after
// this boot, settings.disabled.json never holds hooks again (D33, F25).
func restoreDisabledHooks(settingsPath string) (*disabledHookStore, error) {
	store := newDisabledHookStore()
	if err := absorbTmpFile(store, disabledHooksTmpPath(settingsPath)); err != nil {
		return store, err
	}

	if err := absorbSidecarHooks(store, settingsPath); err != nil {
		return store, err
	}
	return store, nil
}

func absorbTmpFile(store *disabledHookStore, tmpPath string) error {
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("absorbTmpFile: Failed to read %s: %w", tmpPath, err)
	}

	var groups map[string][]config.HookGroup
	if err := json.Unmarshal(data, &groups); err != nil {
		return fmt.Errorf("absorbTmpFile: Failed to parse %s: %w", tmpPath, err)
	}
	for event, eventGroups := range groups {
		for _, group := range eventGroups {
			store.Add(event, group)
		}
	}

	if err := os.Remove(tmpPath); err != nil {
		return fmt.Errorf("absorbTmpFile: Failed to remove %s: %w", tmpPath, err)
	}
	return nil
}

func absorbSidecarHooks(store *disabledHookStore, settingsPath string) error {
	sidecar, err := config.Load(config.DisabledPath(settingsPath))
	if err != nil {
		return fmt.Errorf("absorbSidecarHooks: %w", err)
	}

	hooks, err := sidecar.Hooks()
	if err != nil {
		return fmt.Errorf("absorbSidecarHooks: %w", err)
	}
	if len(hooks) == 0 {
		return nil
	}

	for event, groups := range hooks {
		for _, group := range groups {
			store.Add(event, group)
		}
	}

	if err := sidecar.SetHooks(nil); err != nil {
		return fmt.Errorf("absorbSidecarHooks: %w", err)
	}
	if err := config.Save(config.DisabledPath(settingsPath), sidecar); err != nil {
		return fmt.Errorf("absorbSidecarHooks: %w", err)
	}
	return nil
}

// FlushDisabledHooks parks the in-memory disabled hooks next to settings.json
// for the next boot; an empty store leaves no file (D33).
func (s *Server) FlushDisabledHooks() error {
	groups := s.disabledHooks.Snapshot()
	if len(groups) == 0 {
		return nil
	}

	data, err := json.MarshalIndent(groups, "", "  ")
	if err != nil {
		return fmt.Errorf("Server.FlushDisabledHooks: %w", err)
	}
	return os.WriteFile(disabledHooksTmpPath(s.settingsPath), data, 0o600)
}

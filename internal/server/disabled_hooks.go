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

// disabledHookStore parks toggled-off hook groups in a write-through sidecar
// (hooks.disabled.json next to settings.json) — disk-visible and restart-proof;
// settings.json stays the only store for enabled hooks (revises D33).
type disabledHookStore struct {
	// mu guards groups and serializes persists.
	mu     sync.Mutex
	path   string
	groups map[string][]config.HookGroup
}

func newDisabledHookStore(settingsPath string) *disabledHookStore {
	return &disabledHookStore{
		path:   filepath.Join(filepath.Dir(settingsPath), "hooks.disabled.json"),
		groups: make(map[string][]config.HookGroup),
	}
}

func (d *disabledHookStore) Add(event string, group config.HookGroup) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.add(event, group)
	return d.persistLocked()
}

// add appends without persisting — the boot absorbs batch their persist.
func (d *disabledHookStore) add(event string, group config.HookGroup) {
	d.groups[event] = append(d.groups[event], group)
}

// Pop removes and returns the group at event/index; ok=false when absent.
func (d *disabledHookStore) Pop(event string, index int) (config.HookGroup, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	groups, ok := d.groups[event]
	if !ok || index >= len(groups) {
		return config.HookGroup{}, false, nil
	}

	group := groups[index]
	d.groups[event] = slices.Delete(groups, index, index+1)
	if len(d.groups[event]) == 0 {
		delete(d.groups, event)
	}
	return group, true, d.persistLocked()
}

// Clear drops every parked group — a claude Revert makes the repo fragment
// the whole truth, so parked drift must not survive it.
func (d *disabledHookStore) Clear() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.groups = make(map[string][]config.HookGroup)
	return d.persistLocked()
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

// persist is the lock-taking wrapper for the boot path.
func (d *disabledHookStore) persist() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.persistLocked()
}

// persistLocked mirrors the map to the sidecar; an empty map removes the file.
func (d *disabledHookStore) persistLocked() error {
	if len(d.groups) == 0 {
		if err := os.Remove(d.path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("disabledHookStore.persistLocked: %w", err)
		}
		return nil
	}

	data, err := json.MarshalIndent(d.groups, "", "  ")
	if err != nil {
		return fmt.Errorf("disabledHookStore.persistLocked: %w", err)
	}
	if err := os.WriteFile(d.path, data, 0o600); err != nil {
		return fmt.Errorf("disabledHookStore.persistLocked: %w", err)
	}
	return nil
}

// load reads an existing sidecar into the store; a missing file is empty.
func (d *disabledHookStore) load() error {
	data, err := os.ReadFile(d.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("disabledHookStore.load: %w", err)
	}
	if err := json.Unmarshal(data, &d.groups); err != nil {
		return fmt.Errorf("disabledHookStore.load: Failed to parse %s: %w", d.path, err)
	}
	return nil
}

// disabledHooksTmpPath sits next to settings.json; legacy shutdown handoff of
// the retired in-memory store — read (and consumed) at boot as migration only.
func disabledHooksTmpPath(settingsPath string) string {
	return filepath.Join(filepath.Dir(settingsPath), ".disabled-hooks.tmp.json")
}

// restoreDisabledHooks builds the boot store from hooks.disabled.json, then
// absorbs the two legacy stores (shutdown tmp file, settings.disabled.json
// hooks key) and persists the merge — legacy state migrates on first boot.
func restoreDisabledHooks(settingsPath string) (*disabledHookStore, error) {
	store := newDisabledHookStore(settingsPath)
	if err := store.load(); err != nil {
		return store, err
	}
	if err := absorbTmpFile(store, disabledHooksTmpPath(settingsPath)); err != nil {
		return store, err
	}

	if err := absorbSidecarHooks(store, settingsPath); err != nil {
		return store, err
	}
	return store, store.persist()
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
			store.add(event, group)
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
			store.add(event, group)
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

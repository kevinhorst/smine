package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/install"
)

// disabledHookStore reads hooks.disabled.json on every call and writes through the install engine's helper; settings.json stays the only store for enabled hooks.
type disabledHookStore struct {
	// mu serializes a drop-then-persist against a concurrent add.
	mu  sync.Mutex
	env install.Env
}

func newDisabledHookStore(env install.Env) *disabledHookStore {
	return &disabledHookStore{env: env}
}

// addAll appends every group per event in one load and one save.
func (d *disabledHookStore) addAll(groups map[string][]config.HookGroup) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	parked, err := install.LoadParked(d.env)
	if err != nil {
		return fmt.Errorf("disabledHookStore.addAll: %w", err)
	}

	for event, eventGroups := range groups {
		parked[event] = append(parked[event], eventGroups...)
	}
	if err := install.SaveParked(d.env, parked); err != nil {
		return fmt.Errorf("disabledHookStore.addAll: %w", err)
	}
	return nil
}

func (d *disabledHookStore) Add(event string, group config.HookGroup) error {
	groups := map[string][]config.HookGroup{event: {group}}
	if err := d.addAll(groups); err != nil {
		return fmt.Errorf("disabledHookStore.Add: %w", err)
	}
	return nil
}

// Drop removes the group at event/index and persists; a missing group is fine, the settings write before it already holds the group.
func (d *disabledHookStore) Drop(event string, index int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	parked, err := install.LoadParked(d.env)
	if err != nil {
		return fmt.Errorf("disabledHookStore.Drop: %w", err)
	}

	groups, ok := parked[event]
	if !ok || index >= len(groups) {
		return nil
	}

	parked[event] = slices.Delete(groups, index, index+1)
	if len(parked[event]) == 0 {
		delete(parked, event)
	}
	if err := install.SaveParked(d.env, parked); err != nil {
		return fmt.Errorf("disabledHookStore.Drop: %w", err)
	}
	return nil
}

// Peek returns the group at event/index without writing; ok=false when absent.
func (d *disabledHookStore) Peek(event string, index int) (config.HookGroup, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	parked, err := install.LoadParked(d.env)
	if err != nil {
		return config.HookGroup{}, false, fmt.Errorf("disabledHookStore.Peek: %w", err)
	}

	groups, ok := parked[event]
	if !ok || index >= len(groups) {
		return config.HookGroup{}, false, nil
	}
	return groups[index], true, nil
}

// Snapshot is the sidecar as it is on disk.
func (d *disabledHookStore) Snapshot() (map[string][]config.HookGroup, error) {
	parked, err := install.LoadParked(d.env)
	if err != nil {
		return nil, fmt.Errorf("disabledHookStore.Snapshot: %w", err)
	}
	return parked, nil
}

// disabledHooksTmpPath is the retired store's shutdown handoff, read at boot as migration only.
func disabledHooksTmpPath(settingsPath string) string {
	return filepath.Join(filepath.Dir(settingsPath), ".disabled-hooks.tmp.json")
}

// restoreDisabledHooks absorbs the two legacy stores into the sidecar, then retires them.
func restoreDisabledHooks(env install.Env, settingsPath string) (*disabledHookStore, error) {
	store := newDisabledHookStore(env)
	tmpPath := disabledHooksTmpPath(settingsPath)
	absorbed, err := readTmpFile(tmpPath)
	if err != nil {
		return store, err
	}

	sidecar, sidecarHooks, err := readSidecarHooks(settingsPath)
	if err != nil {
		return store, err
	}
	for event, groups := range sidecarHooks {
		absorbed[event] = append(absorbed[event], groups...)
	}
	if len(absorbed) == 0 {
		return store, nil
	}

	if err := store.addAll(absorbed); err != nil {
		return store, fmt.Errorf("restoreDisabledHooks: %w", err)
	}
	if err := os.Remove(tmpPath); err != nil && !os.IsNotExist(err) {
		return store, fmt.Errorf("restoreDisabledHooks: Failed to remove %s: %w", tmpPath, err)
	}
	if len(sidecarHooks) == 0 {
		return store, nil
	}

	if err := sidecar.SetHooks(nil); err != nil {
		return store, fmt.Errorf("restoreDisabledHooks: %w", err)
	}
	if err := config.Save(config.DisabledPath(settingsPath), sidecar, env.Trash); err != nil {
		return store, fmt.Errorf("restoreDisabledHooks: %w", err)
	}
	return store, nil
}

// readSidecarHooks reads the hooks key of settings.disabled.json, the older legacy store.
func readSidecarHooks(settingsPath string) (*config.Settings, map[string][]config.HookGroup, error) {
	sidecar, err := config.Load(config.DisabledPath(settingsPath))
	if err != nil {
		return nil, nil, fmt.Errorf("readSidecarHooks: %w", err)
	}

	hooks, err := sidecar.Hooks()
	if err != nil {
		return nil, nil, fmt.Errorf("readSidecarHooks: %w", err)
	}
	return sidecar, hooks, nil
}

// readTmpFile reads the legacy tmp handoff; a missing file is empty.
func readTmpFile(tmpPath string) (map[string][]config.HookGroup, error) {
	groups := map[string][]config.HookGroup{}
	data, err := os.ReadFile(tmpPath)
	if os.IsNotExist(err) {
		return groups, nil
	}
	if err != nil {
		return nil, fmt.Errorf("readTmpFile: Failed to read %s: %w", tmpPath, err)
	}

	if err := json.Unmarshal(data, &groups); err != nil {
		return nil, fmt.Errorf("readTmpFile: Failed to parse %s: %w", tmpPath, err)
	}
	if groups == nil {
		groups = map[string][]config.HookGroup{}
	}
	return groups, nil
}

package install

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/fsx"
)

// Settings keys the persist walk treats specially.
const (
	envKey         = "env"
	permissionsKey = "permissions"
)

// templateFile is one template file of the settings dir: its marker-bearing bytes and the document with the markers expanded.
type templateFile struct {
	data    []byte
	doc     *config.Document
	isDirty bool
	path    string
}

func loadTemplateFile(path string, markers config.Markers) (*templateFile, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		data = []byte("{}\n")
	} else if err != nil {
		return nil, fmt.Errorf("loadTemplateFile: %w", err)
	}

	doc := config.NewDocument()
	if err := json.Unmarshal(markers.Expand(data), doc); err != nil {
		return nil, fmt.Errorf("loadTemplateFile: Failed to parse %s: %w", path, err)
	}
	return &templateFile{data: data, doc: doc, path: path}, nil
}

// isMarkerValue reports a value the file's unexpanded bytes hold as a {{…}} marker: a rendered value, never persisted back.
func (f *templateFile) isMarkerValue(path []string) bool {
	raw := config.NewDocument()
	if err := json.Unmarshal(f.data, raw); err != nil {
		return false
	}

	value, isPresent := raw.Get(path)
	return isPresent && strings.Contains(string(value), "{{")
}

// set writes value at path when it differs from what the file holds and reports whether it did.
func (f *templateFile) set(path []string, value json.RawMessage) (bool, error) {
	current, isPresent := f.doc.Get(path)
	if isPresent && isSameJson(current, value) {
		return false, nil
	}
	if err := f.doc.Set(path, value); err != nil {
		return false, fmt.Errorf("templateFile.set: %w", err)
	}
	f.isDirty = true
	return true, nil
}

func (f *templateFile) unset(path []string) {
	if f.doc.Unset(path) {
		f.isDirty = true
	}
}

// save collapses the install markers against the file's previous bytes and refuses a result that does not expand back.
func (f *templateFile) save(markers config.Markers) error {
	if !f.isDirty {
		return nil
	}

	data, err := json.MarshalIndent(f.doc, "", "  ")
	if err != nil {
		return fmt.Errorf("templateFile.save: %w", err)
	}
	data = append(data, '\n')
	collapsed, err := markers.Collapse(data, f.data)
	if err != nil {
		return fmt.Errorf("templateFile.save: %s: %w", f.path, err)
	}
	if err := fsx.WriteFileAtomic(collapsed, f.path, 0o644); err != nil {
		return fmt.Errorf("templateFile.save: %w", err)
	}
	return nil
}

// ownership is which component fragment declares each env key and each permission list entry.
type ownership struct {
	envOwner        map[string]string
	permissionOwner map[string]map[string]bool
}

func loadOwnership(env Env) (*ownership, error) {
	owned := &ownership{envOwner: map[string]string{}, permissionOwner: map[string]map[string]bool{}}
	for _, row := range components.Catalog() {
		fragment, err := config.LoadComponentFragment(settingsDir(env), string(row.ID), env.markers())
		if err != nil {
			return nil, fmt.Errorf("loadOwnership: %w", err)
		}
		for key := range fragment.Env {
			owned.envOwner[key] = string(row.ID)
		}
		for _, list := range fragment.Permissions.Lists() {
			listKey := list.Path()[1]
			if owned.permissionOwner[listKey] == nil {
				owned.permissionOwner[listKey] = map[string]bool{}
			}
			for _, value := range list.Values() {
				owned.permissionOwner[listKey][value] = true
			}
		}
	}
	return owned, nil
}

// PersistSettings writes the live settings.json back into the template by ownership (README § Settings reconciliation).
func PersistSettings(env Env) ([]string, error) {
	live, err := config.Load(settingsPath(env))
	if err != nil {
		return nil, fmt.Errorf("PersistSettings: %w", err)
	}

	owned, err := loadOwnership(env)
	if err != nil {
		return nil, fmt.Errorf("PersistSettings: %w", err)
	}

	base, err := loadTemplateFile(filepath.Join(settingsDir(env), config.BaseSettingsFile), env.markers())
	if err != nil {
		return nil, fmt.Errorf("PersistSettings: %w", err)
	}

	fragments := map[string]*templateFile{}
	var changes []string
	for _, key := range live.Doc().Keys() {
		value, _ := live.Doc().Get([]string{key})
		switch key {
		case hooksKey:
		case envKey:
			envChanges, err := persistEnv(env, value, owned, base, fragments)
			if err != nil {
				return changes, err
			}
			changes = append(changes, envChanges...)
		case permissionsKey:
			permissionChanges, err := persistPermissions(value, owned, base)
			if err != nil {
				return changes, err
			}
			changes = append(changes, permissionChanges...)
		default:
			isSet, err := base.set([]string{key}, value)
			if err != nil {
				return changes, fmt.Errorf("PersistSettings: %w", err)
			}
			if isSet {
				changes = append(changes, "base "+key)
			}
		}
	}

	for _, key := range base.doc.Keys() {
		if _, isLive := live.Doc().Get([]string{key}); !isLive && key != hooksKey {
			base.unset([]string{key})
			changes = append(changes, "base "+key+" removed")
		}
	}

	if err := base.save(env.markers()); err != nil {
		return changes, fmt.Errorf("PersistSettings: %w", err)
	}
	for _, id := range slices.Sorted(maps.Keys(fragments)) {
		if err := fragments[id].save(env.markers()); err != nil {
			return changes, fmt.Errorf("PersistSettings: %w", err)
		}
	}
	return dedupeChanges(changes), nil
}

// persistEnv routes each live env key to the fragment declaring it, else to base, and drops base env keys no longer live.
func persistEnv(env Env, raw json.RawMessage, owned *ownership, base *templateFile, fragments map[string]*templateFile) ([]string, error) {
	liveEnv := config.NewDocument()
	if err := json.Unmarshal(raw, liveEnv); err != nil {
		return nil, fmt.Errorf("persistEnv: env is not an object: %w", err)
	}

	var changes []string
	for _, key := range liveEnv.Keys() {
		value, _ := liveEnv.Get([]string{key})
		owner, isOwned := owned.envOwner[key]
		target, label := base, "base"
		if isOwned {
			fragment, err := fragmentFile(env, owner, fragments)
			if err != nil {
				return changes, err
			}
			target, label = fragment, owner
		}
		if target.isMarkerValue([]string{envKey, key}) {
			continue
		}

		isSet, err := target.set([]string{envKey, key}, value)
		if err != nil {
			return changes, fmt.Errorf("persistEnv: %w", err)
		}
		if isSet {
			changes = append(changes, label+" env."+key)
		}
	}

	baseEnv, _ := base.doc.Get([]string{envKey})
	baseKeys := config.NewDocument()
	_ = json.Unmarshal(baseEnv, baseKeys)
	for _, key := range baseKeys.Keys() {
		if _, isLive := liveEnv.Get([]string{key}); !isLive {
			base.unset([]string{envKey, key})
			changes = append(changes, "base env."+key+" removed")
		}
	}
	return changes, nil
}

// persistPermissions writes the unowned entries of each owned list and every other permissions key to base.
func persistPermissions(raw json.RawMessage, owned *ownership, base *templateFile) ([]string, error) {
	livePermissions := config.NewDocument()
	if err := json.Unmarshal(raw, livePermissions); err != nil {
		return nil, fmt.Errorf("persistPermissions: permissions is not an object: %w", err)
	}

	var changes []string
	for _, key := range livePermissions.Keys() {
		value, _ := livePermissions.Get([]string{key})
		ownedValues, isOwnedList := owned.permissionOwner[key]
		if !isOwnedList {
			isSet, err := base.set([]string{permissionsKey, key}, value)
			if err != nil {
				return changes, fmt.Errorf("persistPermissions: %w", err)
			}
			if isSet {
				changes = append(changes, "base permissions."+key)
			}
			continue
		}

		var entries []string
		if err := json.Unmarshal(value, &entries); err != nil {
			return changes, fmt.Errorf("persistPermissions: permissions.%s is not a string list: %w", key, err)
		}
		unowned := slices.DeleteFunc(entries, func(entry string) bool { return ownedValues[entry] })
		current, _ := base.doc.List([]string{permissionsKey, key})
		if isSameJson(mustMarshal(current), mustMarshal(config.RawStrings(unowned))) {
			continue
		}
		if err := base.doc.SetList([]string{permissionsKey, key}, config.RawStrings(unowned)); err != nil {
			return changes, fmt.Errorf("persistPermissions: %w", err)
		}
		base.isDirty = true
		changes = append(changes, "base permissions."+key)
	}

	basePermissions, _ := base.doc.Get([]string{permissionsKey})
	baseKeys := config.NewDocument()
	_ = json.Unmarshal(basePermissions, baseKeys)
	for _, key := range baseKeys.Keys() {
		if _, isLive := livePermissions.Get([]string{key}); !isLive {
			base.unset([]string{permissionsKey, key})
			changes = append(changes, "base permissions."+key+" removed")
		}
	}
	return changes, nil
}

func fragmentFile(env Env, id string, fragments map[string]*templateFile) (*templateFile, error) {
	if fragment, ok := fragments[id]; ok {
		return fragment, nil
	}

	fragment, err := loadTemplateFile(filepath.Join(settingsDir(env), config.ComponentFragmentDir, id+".json"), env.markers())
	if err != nil {
		return nil, fmt.Errorf("fragmentFile: %w", err)
	}
	fragments[id] = fragment
	return fragment, nil
}

func mustMarshal(list []json.RawMessage) json.RawMessage {
	if len(list) == 0 {
		return json.RawMessage("[]")
	}
	encoded, _ := json.Marshal(list)
	return encoded
}

func dedupeChanges(changes []string) []string {
	seen := map[string]bool{}
	kept := make([]string, 0, len(changes))
	for _, change := range changes {
		if seen[change] {
			continue
		}
		seen[change] = true
		kept = append(kept, change)
	}
	return kept
}

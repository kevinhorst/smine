package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/trash"
)

const (
	// hooksKey is the settings.json key holding the hook groups per event.
	hooksKey = "hooks"

	// mcpServersKey is the ~/.claude.json key holding the MCP servers fragments declare.
	mcpServersKey = "mcpServers"

	// Reasons a smine group is parked for; a user's own park carries none.
	reasonComponentOff = "component off"

	// retiredHookFile names the ids the repo retired: the only ids the prune may remove (master B1).
	retiredHookFile = "retired-hooks.json"
)

var (
	// forceSectionKeys are the top-level keys ForcePreview diffs per entry rather than as a whole.
	forceSectionKeys = map[string]bool{envKey: true, hooksKey: true, permissionsKey: true}

	// ownedListLabels are the dotted paths of the permission lists fragments own, which applyBase leaves to the list reconcile.
	ownedListLabels = ownedListLabelSet()
)

// ForceReport lists what ForceSettings would remove, for the confirm the config page shows first (master B8).
type ForceReport struct {
	EnvKeys     []string
	Groups      []string
	Keys        []string
	Parked      []string
	Permissions []string
}

// IsEmpty reports a rebuild that removes nothing.
func (r *ForceReport) IsEmpty() bool {
	return len(r.EnvKeys)+len(r.Groups)+len(r.Keys)+len(r.Parked)+len(r.Permissions) == 0
}

type retiredHooks struct {
	Retired []string `json:"retired"`
}

// settingsFiles are the live documents fragments land in, saved only where something changed.
type settingsFiles struct {
	claudeJson        *config.Settings
	claudeJsonPath    string
	component         components.ID // the component whose fragment reconciles, for its group overrides
	isClaudeJsonDirty bool
	isParkedDirty     bool
	isSettingsDirty   bool
	parked            map[string][]config.HookGroup
	record            Record // the overrides the hookGroup sink applies
	settings          *config.Settings
	settingsPath      string
	trash             *trash.Store
}

func loadSettingsFiles(env Env) (*settingsFiles, error) {
	settings, err := config.Load(settingsPath(env))
	if err != nil {
		return nil, fmt.Errorf("loadSettingsFiles: %w", err)
	}

	claudeJson, err := config.Load(claudeJsonPath(env))
	if err != nil {
		return nil, fmt.Errorf("loadSettingsFiles: %w", err)
	}

	parked, err := LoadParked(env)
	if err != nil {
		return nil, fmt.Errorf("loadSettingsFiles: %w", err)
	}

	files := &settingsFiles{
		claudeJson:     claudeJson,
		claudeJsonPath: claudeJsonPath(env),
		parked:         parked,
		settings:       settings,
		settingsPath:   settingsPath(env),
		trash:          env.Trash,
	}
	return files, nil
}

// journal writes one line for a removal inside settings.json; the file's own previous bytes are kept by config.Save.
func (f *settingsFiles) journal(action string, kind trash.Kind, where, reason string) {
	path := f.settingsPath
	if where != "" {
		path += "#" + where
	}

	entry := &trash.Entry{Action: action, Kind: kind, Path: path, Reason: reason}
	if err := f.trash.Note(entry); err != nil {
		log.Printf("settingsFiles.journal: %v", err)
	}
}

// journalRemovals journals the equality-guarded deletes of a deselected component as one line per list or key set, the entries named in the reason (master B5, B6); on select the changes are additions.
func (f *settingsFiles) journalRemovals(isSelected bool, path, where string, labels []string) {
	if isSelected || len(labels) == 0 {
		return
	}

	entry := &trash.Entry{Action: "settings reconcile", Kind: trash.KindRemoved, Path: path + "#" + where, Reason: "component off, equal to the fragment: " + strings.Join(labels, ", ")}
	if err := f.trash.Note(entry); err != nil {
		log.Printf("settingsFiles.journalRemovals: %v", err)
	}
}

// listLabels unquotes reconcileList's changes for the journal.
func listLabels(changes []string) []string {
	labels := make([]string, 0, len(changes))
	for _, change := range changes {
		labels = append(labels, entryLabel(json.RawMessage(change)))
	}
	return labels
}

// keyLabels strips reconcileKeys' "<key>." prefix for the journal.
func keyLabels(key string, changes []string) []string {
	labels := make([]string, 0, len(changes))
	for _, change := range changes {
		labels = append(labels, strings.TrimPrefix(change, key+"."))
	}
	return labels
}

// parkedIndex finds a parked group by id, -1 when absent.
func (f *settingsFiles) parkedIndex(event, id string) int {
	return slices.IndexFunc(f.parked[event], func(parked config.HookGroup) bool { return parked.Id == id })
}

// prune removes every group whose id the repo's retire list names, live and parked, journaling each; every other group stays (master B1).
func (f *settingsFiles) prune(retired map[string]bool) ([]string, error) {
	if len(retired) == 0 {
		return nil, nil
	}

	hooks := config.NewDocument()
	if raw, ok := f.settings.Doc().Get([]string{hooksKey}); ok {
		if err := json.Unmarshal(raw, hooks); err != nil {
			return nil, fmt.Errorf("settingsFiles.prune: hooks is not an object: %w", err)
		}
	}

	var changes []string
	for _, event := range hooks.Keys() {
		eventChanges, err := f.pruneEvent(event, retired)
		if err != nil {
			return changes, err
		}
		changes = append(changes, eventChanges...)
	}

	parkedEvents := slices.Sorted(maps.Keys(f.parked))
	for _, event := range parkedEvents {
		changes = append(changes, f.pruneParked(event, retired)...)
	}
	return changes, nil
}

// pruneEvent rewrites one live event list without its retired groups.
func (f *settingsFiles) pruneEvent(event string, retired map[string]bool) ([]string, error) {
	path := []string{hooksKey, event}
	live, err := f.settings.Doc().List(path)
	if err != nil {
		return nil, fmt.Errorf("settingsFiles.pruneEvent: %w", err)
	}

	var changes []string
	kept := make([]json.RawMessage, 0, len(live))
	for _, raw := range live {
		group, err := decodeGroup(raw)
		if err != nil {
			return changes, fmt.Errorf("settingsFiles.pruneEvent: %w", err)
		}

		if !isRetired(&group, retired) {
			kept = append(kept, raw)
			continue
		}
		changes = append(changes, "hook "+event+" "+group.Id+" retired")
		f.journal("settings prune", trash.KindRemoved, "hooks/"+event+"/"+group.Id, "retired by the repo")
	}

	if len(changes) == 0 {
		return nil, nil
	}
	f.isSettingsDirty = true
	return changes, f.settings.Doc().SetList(path, kept)
}

// pruneParked drops retired groups from one parked event list.
func (f *settingsFiles) pruneParked(event string, retired map[string]bool) []string {
	var changes []string
	kept := make([]config.HookGroup, 0, len(f.parked[event]))
	for _, group := range f.parked[event] {
		if !isRetired(&group, retired) {
			kept = append(kept, group)
			continue
		}
		changes = append(changes, "parked hook "+event+" "+group.Id+" retired")
		f.journal("settings prune", trash.KindRemoved, "parked/"+event+"/"+group.Id, "retired by the repo")
	}

	if len(changes) == 0 {
		return nil
	}
	f.parked[event] = kept
	f.isParkedDirty = true
	return changes
}

// reconcile counts a parked group as present and never touches entries matching no fragment.
func (f *settingsFiles) reconcile(fragment *config.Fragment, isSelected bool) ([]string, error) {
	var changes []string
	for _, event := range slices.Sorted(maps.Keys(fragment.Hooks)) {
		eventChanges, err := f.reconcileHooks(event, fragment.Hooks[event], isSelected)
		if err != nil {
			return changes, err
		}
		changes = append(changes, eventChanges...)
	}

	for _, list := range fragment.Permissions.Lists() {
		listChanges, err := reconcileList(f.settings.Doc(), list.Path(), config.RawStrings(list.Values()), isSelected)
		if err != nil {
			return changes, err
		}
		f.journalRemovals(isSelected, f.settingsPath, strings.Join(list.Path(), "."), listLabels(listChanges))
		changes = append(changes, listChanges...)
	}

	envChanges, err := reconcileKeys(f.settings.Doc(), envKey, rawEnv(fragment.Env), isSelected)
	if err != nil {
		return changes, err
	}
	f.journalRemovals(isSelected, f.settingsPath, envKey, keyLabels(envKey, envChanges))
	f.isSettingsDirty = f.isSettingsDirty || len(changes)+len(envChanges) > 0
	changes = append(changes, envChanges...)

	mcpChanges, err := reconcileKeys(f.claudeJson.Doc(), mcpServersKey, fragment.McpServers, isSelected)
	f.journalRemovals(isSelected, f.claudeJsonPath, mcpServersKey, keyLabels(mcpServersKey, mcpChanges))
	f.isClaudeJsonDirty = f.isClaudeJsonDirty || len(mcpChanges) > 0
	return append(changes, mcpChanges...), err
}

// reconcileHooks matches one event's groups by id: a lower live version is replaced in place, the parked sidecar included.
func (f *settingsFiles) reconcileHooks(event string, groups []json.RawMessage, isSelected bool) ([]string, error) {
	path := []string{hooksKey, event}
	live, err := f.settings.Doc().List(path)
	if err != nil {
		return nil, fmt.Errorf("settingsFiles.reconcileHooks: %w", err)
	}

	var changes []string
	for _, fragmentGroup := range groups {
		group, err := applyGroupFields(f.record, f.component, fragmentGroup)
		if err != nil {
			return changes, fmt.Errorf("settingsFiles.reconcileHooks: %w", err)
		}

		wanted, err := decodeGroup(group)
		if err != nil {
			return changes, fmt.Errorf("settingsFiles.reconcileHooks: %w", err)
		}

		liveAt, liveVersion, err := indexOfId(live, wanted.Id)
		if err != nil {
			return changes, fmt.Errorf("settingsFiles.reconcileHooks: %w", err)
		}

		parkedAt := f.parkedIndex(event, wanted.Id)
		isLive := liveAt >= 0
		isMissing := !isLive && parkedAt < 0
		isOutdated := false
		isFieldStale := false
		var patched json.RawMessage
		if isLive {
			order, err := liveVersion.Compare(wanted.Version)
			if err != nil {
				return changes, fmt.Errorf("settingsFiles.reconcileHooks: %s: %w", wanted.Id, err)
			}

			isOutdated = order < 0
			patched, err = copyGroupFields(live[liveAt], group)
			if err != nil {
				return changes, fmt.Errorf("settingsFiles.reconcileHooks: %s: %w", wanted.Id, err)
			}
			isFieldStale = order == 0 && !isSameJson(patched, live[liveAt])
		}
		switch {
		case isSelected && isMissing:
			live = append(live, group)
			changes = append(changes, "hook "+event+" "+wanted.Id)
		case isSelected && isOutdated:
			live[liveAt] = group
			changes = append(changes, fmt.Sprintf("hook %s %s v%s→v%s", event, wanted.Id, liveVersion, wanted.Version))
			f.journal("settings reconcile", trash.KindReplaced, "hooks/"+event+"/"+wanted.Id, fmt.Sprintf("fragment v%s over live v%s", wanted.Version, liveVersion))
		case isSelected && isFieldStale:
			live[liveAt] = patched
			changes = append(changes, "hook "+event+" "+wanted.Id+" settings")
		case !isSelected && isLive:
			parked, err := decodeGroup(live[liveAt])
			if err != nil {
				return changes, fmt.Errorf("settingsFiles.reconcileHooks: %w", err)
			}
			parked.Reason = reasonComponentOff
			f.parked[event] = append(f.parked[event], parked)
			f.isParkedDirty = true
			live = slices.Delete(live, liveAt, liveAt+1)
			changes = append(changes, "hook "+event+" "+wanted.Id+" parked (component off)")
		}

		// reconcileParked unparks through the document, so the local list goes there first and is re-read after.
		if len(changes) > 0 {
			if err := f.settings.Doc().SetList(path, live); err != nil {
				return changes, fmt.Errorf("settingsFiles.reconcileHooks: %w", err)
			}
		}
		parkedChanges, err := f.reconcileParked(event, parkedAt, &wanted, isSelected)
		if err != nil {
			return changes, fmt.Errorf("settingsFiles.reconcileHooks: %w", err)
		}
		if len(parkedChanges) > 0 {
			live, _ = f.settings.Doc().List(path)
		}
		changes = append(changes, parkedChanges...)
	}

	if len(changes) == 0 {
		return nil, nil
	}
	if err := f.settings.Doc().SetList(path, live); err != nil {
		return changes, fmt.Errorf("settingsFiles.reconcileHooks: %w", err)
	}
	return changes, nil
}

// reconcileParked replaces an outdated parked group in place, still disabled, and brings a component-off park back live on reselect; a user's park stays (master B2, B3).
func (f *settingsFiles) reconcileParked(event string, parkedAt int, wanted *config.HookGroup, isSelected bool) ([]string, error) {
	if parkedAt < 0 {
		return nil, nil
	}

	parked := f.parked[event][parkedAt]
	order, err := parked.Version.Compare(wanted.Version)
	if err != nil {
		return nil, fmt.Errorf("settingsFiles.reconcileParked: %s: %w", wanted.Id, err)
	}
	isOutdated := order < 0
	isComponentOff := parked.Reason == reasonComponentOff
	switch {
	case isSelected && isComponentOff:
		parked.Reason = ""
		raw, err := json.Marshal(parked)
		if err != nil {
			return nil, fmt.Errorf("settingsFiles.reconcileParked: %s: %w", wanted.Id, err)
		}

		path := []string{hooksKey, event}
		live, err := f.settings.Doc().List(path)
		if err != nil {
			return nil, fmt.Errorf("settingsFiles.reconcileParked: %w", err)
		}
		if err := f.settings.Doc().SetList(path, append(live, raw)); err != nil {
			return nil, fmt.Errorf("settingsFiles.reconcileParked: %w", err)
		}

		f.parked[event] = slices.Delete(f.parked[event], parkedAt, parkedAt+1)
		f.isParkedDirty, f.isSettingsDirty = true, true
		return []string{"hook " + event + " " + wanted.Id + " unparked (component on)"}, nil
	case isSelected && isOutdated:
		f.parked[event][parkedAt] = *wanted
		f.isParkedDirty = true
		return []string{fmt.Sprintf("parked hook %s %s v%s→v%s", event, wanted.Id, parked.Version, wanted.Version)}, nil
	}
	return nil, nil
}

// save writes ~/.claude.json 0600 since it holds harness credentials.
func (f *settingsFiles) save(env Env) error {
	if f.isSettingsDirty {
		if err := config.Save(settingsPath(env), f.settings, env.Trash); err != nil {
			return fmt.Errorf("settingsFiles.save: %w", err)
		}
	}

	if f.isClaudeJsonDirty {
		data, err := json.MarshalIndent(f.claudeJson.Doc(), "", "  ")
		if err != nil {
			return fmt.Errorf("settingsFiles.save: %w", err)
		}

		entry := &trash.Entry{Action: "settings reconcile", Reason: "mcpServers reconciled"}
		if _, err := env.Trash.Keep(claudeJsonPath(env), entry); err != nil {
			return fmt.Errorf("settingsFiles.save: %w", err)
		}
		if err := fsx.WriteFileAtomic(append(data, '\n'), claudeJsonPath(env), privateFilePerm); err != nil {
			return fmt.Errorf("settingsFiles.save: %w", err)
		}
	}

	if !f.isParkedDirty {
		return nil
	}
	if err := SaveParked(env, f.parked); err != nil {
		return fmt.Errorf("settingsFiles.save: %w", err)
	}
	return nil
}

// addHookIds adds a component's ids to the set and refuses one already present.
func addHookIds(declared map[string]bool, ids []string, component string) error {
	for _, id := range ids {
		if declared[id] {
			return fmt.Errorf("declaredHookIds: %s: Duplicate hook id %s", component, id)
		}
		declared[id] = true
	}
	return nil
}

func claudeJsonPath(env Env) string {
	return filepath.Join(env.Home, ".claude.json")
}

// composeFor is the template document ForceSettings installs for r.
func composeFor(env Env, r Record) (*config.Settings, error) {
	ids := make([]string, 0, len(r.Components))
	for _, id := range r.Components {
		ids = append(ids, string(id))
	}
	composed, err := config.ComposeSettings(settingsDir(env), env.markers(), ids)
	if err != nil {
		return nil, fmt.Errorf("composeFor: %w", err)
	}
	return composed, nil
}

// decodeGroup reads a raw group's typed view, id and version included.
func decodeGroup(raw json.RawMessage) (config.HookGroup, error) {
	var group config.HookGroup
	if err := json.Unmarshal(raw, &group); err != nil {
		return config.HookGroup{}, fmt.Errorf("decodeGroup: %w", err)
	}
	return group, nil
}

// declaredHookIds is the id set over every catalog component's fragment, unselected components included; a duplicate is an error.
func declaredHookIds(env Env) (map[string]bool, error) {
	declared := map[string]bool{}
	for _, row := range components.Catalog() {
		fragment, err := config.LoadComponentFragment(settingsDir(env), string(row.ID), env.markers())
		if err != nil {
			return nil, fmt.Errorf("declaredHookIds: %w", err)
		}

		ids, err := fragment.HookIds()
		if err != nil {
			return nil, fmt.Errorf("declaredHookIds: %s: %w", row.ID, err)
		}
		if err := addHookIds(declared, ids, string(row.ID)); err != nil {
			return nil, err
		}
	}
	return declared, nil
}

// droppedEntries lists the entries of one string list present live and absent composed.
func droppedEntries(live, composed *config.Document, path []string) []string {
	liveEntries, _ := live.List(path)
	composedEntries, _ := composed.List(path)
	var dropped []string
	for _, entry := range liveEntries {
		isComposed := slices.ContainsFunc(composedEntries, func(other json.RawMessage) bool { return isSameJson(other, entry) })
		if !isComposed {
			dropped = append(dropped, entryLabel(entry))
		}
	}
	return dropped
}

// entryLabel is a list entry as the preview shows it: a JSON string unquoted, any other value as its JSON.
func entryLabel(entry json.RawMessage) string {
	var text string
	if err := json.Unmarshal(entry, &text); err != nil {
		return string(entry)
	}
	return text
}

// droppedGroups lists live hook groups the composed document lacks, by smineId, else by event and first command.
func droppedGroups(live, composed *config.Document) ([]string, error) {
	hooks := config.NewDocument()
	if raw, ok := live.Get([]string{hooksKey}); ok {
		if err := json.Unmarshal(raw, hooks); err != nil {
			return nil, fmt.Errorf("droppedGroups: hooks is not an object: %w", err)
		}
	}

	var dropped []string
	for _, event := range hooks.Keys() {
		liveGroups, err := live.List([]string{hooksKey, event})
		if err != nil {
			return nil, fmt.Errorf("droppedGroups: %w", err)
		}

		composedGroups, _ := composed.List([]string{hooksKey, event})
		eventDropped, err := droppedEventGroups(event, liveGroups, composedGroups)
		if err != nil {
			return nil, err
		}
		dropped = append(dropped, eventDropped...)
	}
	return dropped, nil
}

// droppedEventGroups lists one event's live groups without an id, or whose id the composed list lacks.
func droppedEventGroups(event string, liveGroups, composedGroups []json.RawMessage) ([]string, error) {
	var dropped []string
	for _, raw := range liveGroups {
		group, err := decodeGroup(raw)
		if err != nil {
			return nil, fmt.Errorf("droppedEventGroups: %w", err)
		}

		at, _, err := indexOfId(composedGroups, group.Id)
		if err != nil {
			return nil, fmt.Errorf("droppedEventGroups: %w", err)
		}
		if group.Id == "" || at < 0 {
			dropped = append(dropped, event+" "+groupLabel(&group))
		}
	}
	return dropped, nil
}

// droppedKeys lists the keys of one object present live and absent composed.
func droppedKeys(live, composed *config.Document, key string) []string {
	liveObject, composedObject := config.NewDocument(), config.NewDocument()
	if raw, ok := live.Get([]string{key}); ok {
		_ = json.Unmarshal(raw, liveObject)
	}
	if raw, ok := composed.Get([]string{key}); ok {
		_ = json.Unmarshal(raw, composedObject)
	}

	var dropped []string
	for _, name := range liveObject.Keys() {
		if _, isComposed := composedObject.Get([]string{name}); !isComposed {
			dropped = append(dropped, name)
		}
	}
	return dropped
}

// groupLabel names a group for a report: its smineName, else its id, else its first command.
func groupLabel(group *config.HookGroup) string {
	switch {
	case group.Name != "":
		return group.Name
	case group.Id != "":
		return group.Id
	case len(group.Hooks) > 0:
		return group.Hooks[0].Command
	}
	return "(unnamed group)"
}

// applyBase adds every base key absent live, recursing into objects both documents hold; the owned permission lists are reconciled as lists, never replaced.
func applyBase(live, base *config.Document) ([]string, error) {
	return applyBaseLevel(live, base, "")
}

// applyBaseLevel adds the base keys absent at one object level and recurses into objects; a present value is the user's (master B7).
func applyBaseLevel(live, base *config.Document, prefix string) ([]string, error) {
	var changed []string
	for _, key := range base.Keys() {
		label := prefix + key
		if ownedListLabels[label] {
			continue
		}

		baseValue, _ := base.Get([]string{key})
		liveValue, isPresent := live.Get([]string{key})

		childChanged, isObjects, err := applyBaseChild(live, key, baseValue, liveValue, label)
		if err != nil {
			return changed, err
		}
		if isObjects {
			changed = append(changed, childChanged...)
			continue
		}
		if isPresent {
			continue
		}

		if err := live.Set([]string{key}, baseValue); err != nil {
			return changed, fmt.Errorf("applyBaseLevel: %w", err)
		}
		changed = append(changed, "base "+label+" added")
	}
	return changed, nil
}

// applyBaseChild recurses when both values are objects and reports whether they were.
func applyBaseChild(live *config.Document, key string, baseValue, liveValue json.RawMessage, label string) ([]string, bool, error) {
	baseChild, liveChild := config.NewDocument(), config.NewDocument()
	isObjects := json.Unmarshal(baseValue, baseChild) == nil && json.Unmarshal(liveValue, liveChild) == nil
	if !isObjects {
		return nil, false, nil
	}

	changed, err := applyBaseLevel(liveChild, baseChild, label+".")
	if err != nil || len(changed) == 0 {
		return nil, true, err
	}

	encoded, err := json.Marshal(liveChild)
	if err != nil {
		return nil, true, fmt.Errorf("applyBaseChild: %w", err)
	}
	if err := live.Set([]string{key}, encoded); err != nil {
		return nil, true, fmt.Errorf("applyBaseChild: %w", err)
	}
	return changed, true, nil
}

// applyBaseLists adds the base template's permission entries that are missing live; live extras stay.
func applyBaseLists(live, base *config.Document) ([]string, error) {
	var changes []string
	for _, list := range ownedLists() {
		values, err := base.List(list.Path())
		if err != nil {
			return changes, fmt.Errorf("applyBaseLists: %w", err)
		}

		listChanges, err := reconcileList(live, list.Path(), values, true)
		if err != nil {
			return changes, err
		}
		changes = append(changes, listChanges...)
	}
	return changes, nil
}

// indexOfId finds the live group carrying id and returns its index and version, -1 when absent.
func indexOfId(live []json.RawMessage, id string) (int, config.HookVersion, error) {
	for index, raw := range live {
		group, err := decodeGroup(raw)
		if err != nil {
			return -1, "", fmt.Errorf("indexOfId: %w", err)
		}
		if group.Id == id {
			return index, group.Version, nil
		}
	}
	return -1, "", nil
}

// ownedLists are the permission lists a template file may carry, in the fragment's fixed order.
func ownedLists() []config.PermissionList {
	return config.FragmentPermissions{}.Lists()
}

func ownedListLabelSet() map[string]bool {
	labels := map[string]bool{}
	for _, list := range ownedLists() {
		labels[strings.Join(list.Path(), ".")] = true
	}
	return labels
}

// isSameJson compares canonically, ignoring key order and spacing.
func isSameJson(left, right json.RawMessage) bool {
	var leftValue, rightValue any
	if json.Unmarshal(left, &leftValue) != nil || json.Unmarshal(right, &rightValue) != nil {
		return false
	}
	return reflect.DeepEqual(leftValue, rightValue)
}

// isRetired reports a group carrying an id the retire list names; an idless group is never retired.
func isRetired(group *config.HookGroup, retired map[string]bool) bool {
	return group.Id != "" && retired[group.Id]
}

func rawEnv(env map[string]string) map[string]json.RawMessage {
	raws := make(map[string]json.RawMessage, len(env))
	for key, value := range env {
		encoded, _ := json.Marshal(value)
		raws[key] = encoded
	}
	return raws
}

// reconcileKeys adds an absent key when selected and deletes it when removed only while it still equals the fragment's value.
func reconcileKeys(doc *config.Document, key string, entries map[string]json.RawMessage, isSelected bool) ([]string, error) {
	var changes []string
	for _, name := range slices.Sorted(maps.Keys(entries)) {
		path := []string{key, name}
		live, isPresent := doc.Get(path)
		isOwned := isPresent && isSameJson(live, entries[name])
		switch {
		case isSelected && !isPresent:
			if err := doc.Set(path, entries[name]); err != nil {
				return changes, fmt.Errorf("reconcileKeys: %w", err)
			}
			changes = append(changes, key+"."+name)
		case !isSelected && isOwned:
			doc.Unset(path)
			changes = append(changes, key+"."+name)
		}
	}
	return changes, nil
}

// reconcileList handles one owned string list (permissions).
func reconcileList(doc *config.Document, path []string, values []json.RawMessage, isSelected bool) ([]string, error) {
	live, err := doc.List(path)
	if err != nil {
		return nil, fmt.Errorf("reconcileList: %w", err)
	}

	var changes []string
	for _, value := range values {
		isLive := slices.ContainsFunc(live, func(entry json.RawMessage) bool { return isSameJson(entry, value) })
		switch {
		case isSelected && !isLive:
			live = append(live, value)
			changes = append(changes, string(value))
		case !isSelected && isLive:
			live = slices.DeleteFunc(live, func(entry json.RawMessage) bool { return isSameJson(entry, value) })
			changes = append(changes, string(value))
		}
	}
	if len(changes) == 0 {
		return nil, nil
	}
	return changes, doc.SetList(path, live)
}

// retiredHookIds reads settings/claude_code/retired-hooks.json; a missing file retires nothing.
func retiredHookIds(env Env) (map[string]bool, error) {
	data, err := os.ReadFile(filepath.Join(settingsDir(env), retiredHookFile))
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("retiredHookIds: %w", err)
	}

	var file retiredHooks
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("retiredHookIds: %w", err)
	}

	retired := make(map[string]bool, len(file.Retired))
	for _, id := range file.Retired {
		retired[id] = true
	}
	return retired, nil
}

func settingsDir(env Env) string {
	return filepath.Join(env.RepoDir, "settings", "claude_code")
}

func settingsPath(env Env) string {
	return filepath.Join(env.ClaudeHome, "settings.json")
}

// LoadParked reads hooks.disabled.json, the parked hook groups per event; an absent file is an empty map.
func LoadParked(env Env) (map[string][]config.HookGroup, error) {
	parked := map[string][]config.HookGroup{}
	data, err := os.ReadFile(ParkedPath(env))
	if errors.Is(err, fs.ErrNotExist) {
		return parked, nil
	}
	if err != nil {
		return nil, fmt.Errorf("LoadParked: %w", err)
	}

	if err := json.Unmarshal(data, &parked); err != nil {
		return nil, fmt.Errorf("LoadParked: Failed to parse %s: %w", ParkedPath(env), err)
	}
	return parked, nil
}

// ParkedPath is hooks.disabled.json, the parked hook groups beside settings.json.
func ParkedPath(env Env) string {
	return filepath.Join(env.ClaudeHome, "hooks.disabled.json")
}

// SaveParked keeps the previous parked file in the trash and writes hooks.disabled.json atomically.
func SaveParked(env Env, parked map[string][]config.HookGroup) error {
	kept := maps.Clone(parked)
	maps.DeleteFunc(kept, func(_ string, groups []config.HookGroup) bool { return len(groups) == 0 })
	data, err := json.MarshalIndent(kept, "", "  ")
	if err != nil {
		return fmt.Errorf("SaveParked: %w", err)
	}
	if current, err := os.ReadFile(ParkedPath(env)); err == nil && bytes.Equal(current, data) {
		return nil
	}

	entry := &trash.Entry{Action: "parked hooks save", Reason: "hooks.disabled.json rewritten"}
	if _, err := env.Trash.Keep(ParkedPath(env), entry); err != nil {
		return fmt.Errorf("SaveParked: %w", err)
	}
	if err := fsx.WriteFileAtomic(data, ParkedPath(env), 0o644); err != nil {
		return fmt.Errorf("SaveParked: %w", err)
	}
	return nil
}

// ForcePreview diffs the live settings against the composed template: every live key, group, env key, permission entry and parked group the rebuild drops.
func ForcePreview(env Env, r Record) (*ForceReport, error) {
	files, err := loadSettingsFiles(env)
	if err != nil {
		return nil, fmt.Errorf("ForcePreview: %w", err)
	}

	composed, err := composeFor(env, r)
	if err != nil {
		return nil, fmt.Errorf("ForcePreview: %w", err)
	}

	report := &ForceReport{}
	live := files.settings.Doc()
	for _, key := range live.Keys() {
		_, isComposed := composed.Doc().Get([]string{key})
		if !isComposed && !forceSectionKeys[key] {
			report.Keys = append(report.Keys, key)
		}
	}

	report.Groups, err = droppedGroups(live, composed.Doc())
	if err != nil {
		return nil, fmt.Errorf("ForcePreview: %w", err)
	}

	report.EnvKeys = droppedKeys(live, composed.Doc(), envKey)
	for _, list := range ownedLists() {
		for _, entry := range droppedEntries(live, composed.Doc(), list.Path()) {
			report.Permissions = append(report.Permissions, strings.Join(list.Path(), ".")+" "+entry)
		}
	}

	parkedEvents := slices.Sorted(maps.Keys(files.parked))
	for _, event := range parkedEvents {
		for _, group := range files.parked[event] {
			report.Parked = append(report.Parked, event+" "+groupLabel(&group))
		}
	}
	return report, nil
}

// ForceSettings rebuilds settings.json from the template alone after the preview was confirmed; the previous file and parked file go to the trash and the journal names what went (master B8).
func ForceSettings(env Env, r Record) ([]string, error) {
	files, err := loadSettingsFiles(env)
	if err != nil {
		return nil, fmt.Errorf("ForceSettings: %w", err)
	}

	report, err := ForcePreview(env, r)
	if err != nil {
		return nil, fmt.Errorf("ForceSettings: %w", err)
	}

	composed, err := composeFor(env, r)
	if err != nil {
		return nil, fmt.Errorf("ForceSettings: %w", err)
	}

	changes := []string{"settings.json rebuilt from the template: " + strings.Join(composed.Doc().Keys(), ", ")}
	reason := fmt.Sprintf(
		"force sync dropped %d groups, %d env keys, %d permission entries, %d keys, %d parked groups",
		len(report.Groups),
		len(report.EnvKeys),
		len(report.Permissions),
		len(report.Keys),
		len(report.Parked),
	)
	files.journal("force sync", trash.KindReplaced, "", reason)
	files.settings = composed
	files.isSettingsDirty = true
	if len(files.parked) > 0 {
		files.parked = map[string][]config.HookGroup{}
		files.isParkedDirty = true
		changes = append(changes, "parked hooks dropped")
	}

	for _, row := range components.Catalog() {
		fragment, err := config.LoadComponentFragment(settingsDir(env), string(row.ID), env.markers())
		if err != nil {
			return changes, fmt.Errorf("ForceSettings: %w", err)
		}

		mcpChanges, err := reconcileKeys(files.claudeJson.Doc(), mcpServersKey, fragment.McpServers, slices.Contains(r.Components, row.ID))
		if err != nil {
			return changes, fmt.Errorf("ForceSettings: %w", err)
		}
		files.isClaudeJsonDirty = files.isClaudeJsonDirty || len(mcpChanges) > 0
		changes = append(changes, mcpChanges...)
	}
	return changes, files.save(env)
}

// ApplySettings brings settings.json and ~/.claude.json to r, as the config page's Sync runs it.
func ApplySettings(env Env, r Record) ([]string, error) {
	files, err := loadSettingsFiles(env)
	if err != nil {
		return nil, fmt.Errorf("ApplySettings: %w", err)
	}

	base, err := config.LoadBaseSettings(settingsDir(env), env.markers())
	if err != nil {
		return nil, fmt.Errorf("ApplySettings: %w", err)
	}

	changes, err := applyBase(files.settings.Doc(), base.Doc())
	if err != nil {
		return changes, fmt.Errorf("ApplySettings: %w", err)
	}

	listChanges, err := applyBaseLists(files.settings.Doc(), base.Doc())
	if err != nil {
		return changes, fmt.Errorf("ApplySettings: %w", err)
	}
	changes = append(changes, listChanges...)
	files.isSettingsDirty = len(changes) > 0

	files.record = r
	for _, row := range components.Catalog() {
		fragment, err := config.LoadComponentFragment(settingsDir(env), string(row.ID), env.markers())
		if err != nil {
			return changes, fmt.Errorf("ApplySettings: %w", err)
		}

		files.component = row.ID
		componentChanges, err := files.reconcile(fragment, slices.Contains(r.Components, row.ID))
		if err != nil {
			return changes, fmt.Errorf("ApplySettings: %w", err)
		}
		changes = append(changes, componentChanges...)
	}

	retired, err := retiredHookIds(env)
	if err != nil {
		return changes, fmt.Errorf("ApplySettings: %w", err)
	}
	pruneChanges, err := files.prune(retired)
	changes = append(changes, pruneChanges...)
	if err != nil {
		return changes, fmt.Errorf("ApplySettings: %w", err)
	}
	return changes, files.save(env)
}

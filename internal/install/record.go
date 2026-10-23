// Package install is the installation engine behind Plan, Apply and Status over the selection record.
package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/skills"
)

// Presets of a Record (master C1).
const (
	PresetCustom  = "custom"
	PresetFull    = "full"
	PresetMinimal = "minimal"
)

// recordSchema is 2 since the settings and skill variables moved in.
const recordSchema = 2

// Record history and compare-and-swap (master A4, A8).
const (
	historyKeep = 50
	// historyLayout is the trash stamp layout: RFC 3339 nanoseconds, time colons replaced for NTFS.
	historyLayout = "2006-01-02T15-04-05.000000000Z"
)

var presets = []string{PresetCustom, PresetFull, PresetMinimal}

// ErrRecordChanged is Save's refusal of a record whose base stamp is older than the file's.
var ErrRecordChanged = errors.New("install.json changed since it was read — reload and retry")

// pendingFlag reads only the applied key, telling an explicit false from a record written before the key existed.
type pendingFlag struct {
	Applied *bool `json:"applied"`
}

// Record is the selection record (master C1) of the components and skills the machine should have.
type Record struct {
	Applied    bool            `json:"applied"`
	Schema     int             `json:"schema"`
	Preset     string          `json:"preset"`
	Components []components.ID `json:"components"`
	Retired    []components.ID `json:"retired,omitempty"`
	Settings   Settings        `json:"settings,omitempty"`
	Skills     Selection       `json:"skills"`
	Suspended  Suspended       `json:"suspended,omitzero"`
	Updated    time.Time       `json:"updated"`
}

// Selection is the record's skill part: mode all, chosen or none; a confirmed selection makes the terminal installer skip its skill question.
type Selection struct {
	Mode        string            `json:"mode"`
	Chosen      []string          `json:"chosen,omitempty"`
	Defaults    map[string]string `json:"defaults,omitempty"` // global default per repo-scoped interface id
	IsConfirmed bool              `json:"confirmed,omitempty"`
	Providers   map[string]string `json:"providers,omitempty"` // PROVIDER_<ID> program overrides
	Suspended   []string          `json:"suspended,omitempty"` // chosen leaves whose required component is off, restored when it returns
	Variables   map[string]string `json:"variables,omitempty"` // SKILL_<SKILL>_<NAME> rows the deploy fills
}

// Suspended holds what a Profile apply set aside so a later apply can bring it back.
type Suspended struct {
	Components []components.ID `json:"components,omitempty"`
	Routines   []string        `json:"routines,omitempty"` // labels of the routines a Routines removal stopped, started again when it returns
}

// SkillRows is a copy of the record's skill rows as the skills sync reads them, maps never nil; SetSkillRows stores one back.
func SkillRows(r Record) *skills.Rows {
	rows := &skills.Rows{
		Defaults:  maps.Clone(r.Skills.Defaults),
		Providers: maps.Clone(r.Skills.Providers),
		Skills:    maps.Clone(r.Skills.Variables),
	}
	if rows.Defaults == nil {
		rows.Defaults = map[string]string{}
	}
	if rows.Providers == nil {
		rows.Providers = map[string]string{}
	}
	if rows.Skills == nil {
		rows.Skills = map[string]string{}
	}
	return rows
}

// SetSkillRows stores the rows on the record; an empty map is dropped so the record stays minimal.
func SetSkillRows(r *Record, rows *skills.Rows) {
	r.Skills.Defaults, r.Skills.Providers, r.Skills.Variables = nil, nil, nil
	if len(rows.Defaults) > 0 {
		r.Skills.Defaults = rows.Defaults
	}
	if len(rows.Providers) > 0 {
		r.Skills.Providers = rows.Providers
	}
	if len(rows.Skills) > 0 {
		r.Skills.Variables = rows.Skills
	}
}

// LoadRecordAt reads the record of the Claude home for a reader with no checkout, the hook binaries.
func LoadRecordAt(claudeHome string) (Record, error) {
	record, _, err := Load(Env{ClaudeHome: claudeHome, GOOS: runtime.GOOS})
	return record, err
}

// Settings holds the overrides only: component id → setting key → the value in its kind's JSON form.
type Settings map[components.ID]map[string]json.RawMessage

// partitionRetired keeps ids the catalog no longer lists in Retired instead of dropping them (master A3).
func partitionRetired(r *Record) {
	for _, id := range r.Components {
		if _, ok := components.Lookup(id); ok {
			continue
		}
		if !slices.Contains(r.Retired, id) {
			r.Retired = append(r.Retired, id)
		}
	}
	r.Components = slices.DeleteFunc(slices.Clone(r.Components), func(id components.ID) bool {
		_, ok := components.Lookup(id)
		return !ok
	})
}

// Selects reports whether the record, its preset expanded, holds the component.
func Selects(r Record, goos string, id components.ID) bool {
	return slices.Contains(expandPreset(r, goos).Components, id)
}

// expandPreset fills Components for the full and minimal presets.
func expandPreset(r Record, goos string) Record {
	switch r.Preset {
	case PresetFull:
		r.Components = components.Full(goos)
	case PresetMinimal:
		r.Components = components.Minimal()
	case PresetCustom:
	}
	return r
}

// validateComponent checks one selected id is known, available, unique and has its requirements selected.
func validateComponent(id components.ID, selected []components.ID, goos string) error {
	if _, ok := components.Lookup(id); !ok {
		return fmt.Errorf("validateComponent: Unknown component %q", id)
	}
	if !components.Available(id, goos) {
		return fmt.Errorf("validateComponent: Component %s is not available on %s", id, goos)
	}

	occurrences := slices.DeleteFunc(slices.Clone(selected), func(other components.ID) bool {
		return other != id
	})
	if len(occurrences) > 1 {
		return fmt.Errorf("validateComponent: Component %s listed twice", id)
	}

	missing := slices.DeleteFunc(components.Close([]components.ID{id}), func(required components.ID) bool {
		return slices.Contains(selected, required)
	})
	if len(missing) > 0 {
		return fmt.Errorf("validateComponent: Component %s requires %v", id, missing)
	}
	return nil
}

// validateSettings refuses a value its kind refuses; an unknown component or key is kept inert and returned as a note (master A3).
func validateSettings(settings Settings) ([]string, error) {
	var notes []string
	for id, overrides := range settings {
		if _, ok := components.Lookup(id); !ok {
			notes = append(notes, fmt.Sprintf("settings %s: unknown component, kept", id))
			continue
		}

		for key, raw := range overrides {
			setting, isStatic := components.LookupSetting(id, key)
			if !isStatic {
				row, isGroup := groupRow(key)
				if !isGroup {
					notes = append(notes, fmt.Sprintf("settings %s.%s: unknown key, kept", id, key))
					continue
				}
				setting = row
			}
			if _, err := components.ParseJson(&setting, raw); err != nil {
				return notes, fmt.Errorf("validateSettings: %w", err)
			}
		}
	}
	return notes, nil
}

// validateSkills checks the mode and that only mode chosen lists leaves.
func validateSkills(selection Selection) error {
	switch selection.Mode {
	case skills.SelectionModeAll, skills.SelectionModeNone:
		if len(selection.Chosen) > 0 {
			return fmt.Errorf("validateSkills: Mode %s lists chosen skills", selection.Mode)
		}
	case skills.SelectionModeChosen:
		if len(selection.Chosen) == 0 {
			return errors.New("validateSkills: Mode chosen lists no skill")
		}
	default:
		return fmt.Errorf("validateSkills: Unknown skills mode %q", selection.Mode)
	}
	return nil
}

// checkBase refuses a save whose base stamp differs from the stored record's; a zero base means the writer never loaded (master A8).
func checkBase(current []byte, base time.Time) error {
	var stored Record
	if err := json.Unmarshal(current, &stored); err != nil {
		return fmt.Errorf("checkBase: Failed to parse the stored record: %w", err)
	}
	if !stored.Updated.Equal(base) {
		return fmt.Errorf(
			"checkBase: Stored %s, base %s: %w",
			stored.Updated.Format(time.RFC3339Nano),
			base.Format(time.RFC3339Nano),
			ErrRecordChanged,
		)
	}
	return nil
}

// isPendingApply reports a stored record an apply saved before its loop: its before-state was copied when that save ran, so its own copy would only double the history. A record without the key predates the flag.
func isPendingApply(current []byte) bool {
	var stored pendingFlag
	if err := json.Unmarshal(current, &stored); err != nil || stored.Applied == nil {
		return false
	}
	return !*stored.Applied
}

// keepHistory copies the current bytes to <path>.<stamp> and keeps the newest historyKeep copies (master A4).
func keepHistory(path string, current []byte) error {
	stampedPath := path + "." + time.Now().UTC().Format(historyLayout)
	if err := fsx.WriteFileAtomic(current, stampedPath, 0o644); err != nil {
		return fmt.Errorf("keepHistory: %w", err)
	}

	history, err := History(path)
	if err != nil {
		return fmt.Errorf("keepHistory: %w", err)
	}
	for _, old := range history[min(len(history), historyKeep):] {
		if err := os.Remove(old); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("keepHistory: %w", err)
		}
	}
	return nil
}

// ExpandedComponents is the record's component set with its preset expanded for goos.
func ExpandedComponents(record Record, goos string) []components.ID {
	return expandPreset(record, goos).Components
}

// FullRecord is what a machine without install.json is treated as (concept D18).
func FullRecord(goos string) Record {
	record := Record{
		Applied:    true,
		Components: components.Full(goos),
		Preset:     PresetFull,
		Schema:     recordSchema,
		Skills:     Selection{Mode: skills.SelectionModeAll},
	}
	return record
}

// History lists the record's history files newest first.
func History(path string) ([]string, error) {
	matches, err := filepath.Glob(path + ".*")
	if err != nil {
		return nil, fmt.Errorf("History: %w", err)
	}

	var history []string
	for _, match := range matches {
		if _, err := time.Parse(historyLayout, strings.TrimPrefix(match, path+".")); err == nil {
			history = append(history, match)
		}
	}
	slices.Sort(history)
	slices.Reverse(history)
	return history, nil
}

// Load returns the record and whether the file existed, FullRecord for env.GOOS when it is absent.
func Load(env Env) (Record, bool, error) {
	data, err := os.ReadFile(RecordPath(env))
	if errors.Is(err, fs.ErrNotExist) {
		return FullRecord(env.GOOS), false, nil
	}
	if err != nil {
		return Record{}, false, fmt.Errorf("Load: %w", err)
	}

	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return Record{}, true, fmt.Errorf("Load: Failed to parse %s: %w", RecordPath(env), err)
	}
	partitionRetired(&record)
	if err := Validate(record, env.GOOS); err != nil {
		return record, true, fmt.Errorf("Load: %w", err)
	}
	return record, true, nil
}

// Notes lists what the record keeps without being able to name it: retired ids and unknown settings entries, for the Profile page.
func Notes(r Record) []string {
	var notes []string
	for _, id := range r.Retired {
		notes = append(notes, fmt.Sprintf("component %s: no longer in the catalog, kept", id))
	}
	settingNotes, _ := validateSettings(r.Settings)
	return append(notes, settingNotes...)
}

// RecordPath is install.json in the state dir of env's Claude home.
func RecordPath(env Env) string {
	return filepath.Join(env.ClaudeHome, stateDirName, config.StateInstallFile)
}

// Save keeps the previous bytes beside the file, refuses a stale base stamp and writes atomically, all under the record's file lock (master A4, A8).
func Save(env Env, r Record) error {
	record := expandPreset(r, env.GOOS)
	if err := Validate(record, env.GOOS); err != nil {
		return fmt.Errorf("Save: %w", err)
	}

	path := RecordPath(env)
	err := fsx.WithFileLock(path, func() error {
		current, err := os.ReadFile(path)
		isPresent := err == nil
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("Failed to read %s: %w", path, err)
		}

		switch {
		case isPresent:
			if err := checkBase(current, record.Updated); err != nil {
				return err
			}
			if isPendingApply(current) {
				break
			}
			if err := keepHistory(path, current); err != nil {
				return err
			}
		case !record.Updated.IsZero():
			return fmt.Errorf("%w (the file is gone)", ErrRecordChanged)
		}

		record.Updated = time.Now().UTC()
		data, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			return fmt.Errorf("Failed to encode the record: %w", err)
		}
		return fsx.WriteFileAtomic(append(data, '\n'), path, 0o644)
	})
	if err != nil {
		return fmt.Errorf("Save: %w", err)
	}
	return nil
}

// Validate checks requirements, platform availability, mode and ids.
func Validate(r Record, goos string) error {
	// Schema
	if r.Schema != recordSchema {
		return fmt.Errorf("Validate: Unsupported schema %d", r.Schema)
	}

	// Preset
	if !slices.Contains(presets, r.Preset) {
		return fmt.Errorf("Validate: Unknown preset %q", r.Preset)
	}

	// Components
	if !slices.Contains(r.Components, components.ConfigServer) {
		return errors.New("Validate: Missing the fixed component config-server")
	}
	for _, id := range r.Components {
		if err := validateComponent(id, r.Components, goos); err != nil {
			return fmt.Errorf("Validate: %w", err)
		}
	}

	// Skills
	if err := validateSkills(r.Skills); err != nil {
		return fmt.Errorf("Validate: %w", err)
	}

	// Settings
	if err := components.CheckSettings(); err != nil {
		return fmt.Errorf("Validate: %w", err)
	}
	if _, err := validateSettings(r.Settings); err != nil {
		return fmt.Errorf("Validate: %w", err)
	}
	return nil
}

package install

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
)

const (
	groupFieldAsync   = "async"
	groupFieldMatcher = "matcher"
	groupFieldTimeout = "timeout"
	// groupKeyPrefix starts every derived hook-group row key: groups.<smineId>.<field>.
	groupKeyPrefix = "groups."
)

// groupFields are the hook-group fields every group gets a derived row for; matcher and async stay off the Smine page.
var groupFields = []groupField{
	{help: "Seconds Claude Code waits for the hook before it gives up.", kind: components.KindInt, name: groupFieldTimeout},
	{help: "Tool-name pattern the group fires on; empty fires on every tool.", isHidden: true, kind: components.KindRegex, name: groupFieldMatcher},
	{help: "Run the hook without waiting for its answer.", isHidden: true, kind: components.KindBool, name: groupFieldAsync},
}

type groupField struct {
	help     string
	isHidden bool
	kind     components.Kind
	name     string
}

// groupDefault is the fragment's value of the field, the kind's zero text when the fragment sets none.
func groupDefault(group *config.HookGroup, field string) string {
	switch field {
	case groupFieldTimeout:
		timeout := 0
		if len(group.Hooks) > 0 {
			timeout = group.Hooks[0].Timeout
		}
		return strconv.Itoa(timeout)
	case groupFieldMatcher:
		if group.Matcher == nil {
			return ""
		}
		return *group.Matcher
	case groupFieldAsync:
		isAsync := len(group.Hooks) > 0 && group.Hooks[0].Async
		return strconv.FormatBool(isAsync)
	}
	return ""
}

// groupRow is the derived row a groups.<smineId>.<field> key names, without its fragment default; false for any other key.
func groupRow(key string) (components.Setting, bool) {
	rest, isGroupKey := strings.CutPrefix(key, groupKeyPrefix)
	separator := strings.LastIndex(rest, ".")
	if !isGroupKey || separator <= 0 {
		return components.Setting{}, false
	}

	smineId, name := rest[:separator], rest[separator+1:]
	index := slices.IndexFunc(groupFields, func(field groupField) bool { return field.name == name })
	if index < 0 {
		return components.Setting{}, false
	}
	return newGroupRow(smineId, &groupFields[index], ""), true
}

func newGroupRow(smineId string, field *groupField, defaultText string) components.Setting {
	row := components.Setting{
		Applies:  components.AppliesLive,
		Default:  defaultText,
		Help:     field.help,
		IsHidden: field.isHidden,
		Key:      groupKeyPrefix + smineId + "." + field.name,
		Kind:     field.kind,
		Sink:     components.Sink{Kind: components.SinkHookGroup, Target: smineId + "." + field.name},
	}
	return row
}

// LookupRow finds a static or derived row of the component.
func LookupRow(env Env, id components.ID, key string) (components.Setting, error) {
	rows, err := Schema(env, id)
	if err != nil {
		return components.Setting{}, err
	}
	for _, row := range rows {
		if row.Key == key {
			return row, nil
		}
	}
	return components.Setting{}, fmt.Errorf("LookupRow: Unknown setting %s.%s", id, key)
}

// Schema is a component's rows: the catalog's static rows, then one timeout, matcher and async row per hook group of its fragment.
func Schema(env Env, id components.ID) ([]components.Setting, error) {
	component, ok := components.Lookup(id)
	if !ok {
		return nil, fmt.Errorf("Schema: Unknown component %q", id)
	}
	rows := slices.Clone(component.Settings)

	fragment, err := config.LoadComponentFragment(settingsDir(env), string(id), env.markers())
	if err != nil {
		return nil, fmt.Errorf("Schema: %w", err)
	}
	groups, err := fragment.HookGroups()
	if err != nil {
		return nil, fmt.Errorf("Schema: %w", err)
	}
	for _, group := range groups {
		for _, field := range groupFields {
			rows = append(rows, newGroupRow(group.Id, &field, groupDefault(&group, field.name)))
		}
	}
	return rows, nil
}

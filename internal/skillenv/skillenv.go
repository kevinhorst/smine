// Package skillenv resolves the skill values of a session's repo and renders them as shell exports.
package skillenv

import (
	"maps"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/gitroot"
	"github.com/kevinhorst/smine/internal/repos"
)

const (
	exportPrefix   = "export "
	quoteEscape    = `'\''`
	variablePrefix = "SKILL_"
)

// Sources of a resolved value, strongest first.
const (
	SourceRepo          Source = "repo"
	SourceSkillDefault  Source = "skill default"
	SourceGlobalDefault Source = "global default"
)

// Rows are the install record's rows the resolution reads: the global default per interface id and the per-skill defaults.
type Rows struct {
	Defaults  map[string]string
	Variables map[string]string
}

// Source names where a resolved value came from.
type Source string

// Value is one resolved value with its source; Repo names the registry row a repo value came from.
type Value struct {
	Repo   string
	Source Source
	Value  string
}

// ExportLines renders values as `export KEY='value'` lines, keys sorted; a value's single quote is closed, escaped and reopened.
func ExportLines(values map[string]Value) []string {
	lines := make([]string, 0, len(values))
	for _, key := range slices.Sorted(maps.Keys(values)) {
		quoted := strings.ReplaceAll(values[key].Value, "'", quoteEscape)
		lines = append(lines, exportPrefix+key+"='"+quoted+"'")
	}
	return lines
}

// Resolve is the effective value map of cwd's repo, the stronger source written last: the global default of each bound interface, the record's SKILL_ rows, the registry row's env; nil rows or registry contribute nothing.
func Resolve(bindings map[string]string, cwd string, registry *repos.Registry, rows *Rows) map[string]Value {
	values := map[string]Value{}
	if rows == nil {
		rows = &Rows{}
	}
	for name, interfaceId := range bindings {
		if value := rows.Defaults[interfaceId]; value != "" {
			values[name] = Value{Source: SourceGlobalDefault, Value: value}
		}
	}
	for key, value := range rows.Variables {
		isSkillDefault := strings.HasPrefix(key, variablePrefix) && value != ""
		if isSkillDefault {
			values[key] = Value{Source: SourceSkillDefault, Value: value}
		}
	}
	if registry == nil || cwd == "" {
		return values
	}

	row, isRegistered := registry.FindByPath(gitroot.MainRoot(cwd))
	if !isRegistered {
		return values
	}
	for key, value := range row.Env {
		if value != "" {
			values[key] = Value{Repo: row.Name, Source: SourceRepo, Value: value}
		}
	}
	return values
}

// ValueMap is the values without their sources, the name → value view FillVariables reads.
func ValueMap(values map[string]Value) map[string]string {
	plain := make(map[string]string, len(values))
	for name, value := range values {
		plain[name] = value.Value
	}
	return plain
}

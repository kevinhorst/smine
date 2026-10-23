package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/install"
)

const (
	settingsUsage         = "usage: rules settings [--claude-home <dir>] [--repo <dir>] list [--json] [<component>] | get <component>.<key> | set <component>.<key> <value>"
	settingsDefaultsUsage = "usage: rules settings-defaults [--check] <path>"
	settingsSetArgCount   = 2
)

// settingRow is one listed setting with its effective value.
type settingRow struct {
	Applies      components.Applies `json:"applies"`
	Component    components.ID      `json:"component"`
	Current      string             `json:"current"`
	Default      string             `json:"default"`
	Help         string             `json:"help"`
	IsOverridden bool               `json:"overridden"`
	Key          string             `json:"key"`
	Kind         components.Kind    `json:"kind"`
	Sink         string             `json:"sink"`
}

// effectiveValue is a row's override when the record sets one, else its default — the fragment's for a group row.
func effectiveValue(record install.Record, id components.ID, row *components.Setting) (components.Value, error) {
	if install.IsOverridden(record, id, row.Key) {
		return install.Resolve(record, id, row.Key)
	}
	return components.ParseText(row, row.Default)
}

// findRow looks a static or derived row up; false for an unknown component or key.
func findRow(env install.Env, id components.ID, key string) (components.Setting, bool, error) {
	if _, ok := components.Lookup(id); !ok {
		return components.Setting{}, false, nil
	}

	rows, err := install.Schema(env, id)
	if err != nil {
		return components.Setting{}, false, fmt.Errorf("findRow: %w", err)
	}
	index := slices.IndexFunc(rows, func(row components.Setting) bool { return row.Key == key })
	if index < 0 {
		return components.Setting{}, false, nil
	}
	return rows[index], true, nil
}

func runSettings(args []string) int {
	flags := flag.NewFlagSet("settings", flag.ContinueOnError)
	claudeHome := flags.String("claude-home", "", "Claude home whose record is read and written (default ~/.claude)")
	repoDir := flags.String("repo", ".", "the smine checkout whose fragments the apply renders from")
	if err := flags.Parse(args); err != nil {
		return parseExit(err)
	}
	if flags.NArg() == 0 {
		fmt.Fprintln(os.Stderr, settingsUsage)
		return exitError
	}
	args = flags.Args()

	env, err := install.NewEnvAt(*repoDir, *claudeHome, os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	record, _, err := install.Load(env)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	switch args[0] {
	case "list":
		return settingsList(record, args[1:], env, os.Stdout)
	case "get":
		return settingsGet(record, args[1:], env, os.Stdout)
	case "set":
		return settingsSet(context.Background(), record, args[1:], env, os.Stdout)
	}
	fmt.Fprintln(os.Stderr, settingsUsage)
	return exitError
}

// runSettingsDefaults writes or checks the heartbeat crate's defaults fixture: every heartbeatConfig row's default in its rendered JSON form.
func runSettingsDefaults(args []string) int {
	flags := flag.NewFlagSet("settings-defaults", flag.ExitOnError)
	isCheck := flags.Bool("check", false, "compare instead of writing: fail on a stale file")
	flags.Parse(args)
	if flags.NArg() != 1 {
		fmt.Fprintln(os.Stderr, settingsDefaultsUsage)
		return exitError
	}

	rendered, err := install.RenderDefaults(components.Heartbeat, components.SinkHeartbeatConfig)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	return writeOrCheck(flags.Arg(0), rendered, *isCheck, "settings-defaults")
}

// settingRows lists every row of every catalog component, static and derived, with its effective value.
func settingRows(record install.Record, env install.Env) ([]settingRow, error) {
	var rows []settingRow
	for _, component := range components.Catalog() {
		schema, err := install.Schema(env, component.ID)
		if err != nil {
			return nil, fmt.Errorf("settingRows: %w", err)
		}

		for _, setting := range schema {
			value, err := effectiveValue(record, component.ID, &setting)
			if err != nil {
				return nil, fmt.Errorf("settingRows: %w", err)
			}

			row := settingRow{
				Applies:      setting.Applies,
				Component:    component.ID,
				Current:      value.String(),
				Default:      setting.Default,
				Help:         setting.Help,
				IsOverridden: install.IsOverridden(record, component.ID, setting.Key),
				Key:          setting.Key,
				Kind:         setting.Kind,
				Sink:         string(setting.Sink.Kind) + "(" + setting.Sink.Target + ")",
			}
			rows = append(rows, row)
		}
	}
	return rows, nil
}

// settingsGet prints the value, its default and its source.
func settingsGet(record install.Record, args []string, env install.Env, out io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, settingsUsage)
		return exitError
	}

	id, key, ok := splitSettingName(args[0])
	if !ok {
		fmt.Fprintln(os.Stderr, "rules: settings: a setting is <component>.<key>")
		return exitError
	}

	setting, isKnown, err := findRow(env, id, key)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	if !isKnown {
		fmt.Fprintf(os.Stderr, "rules: settings: unknown setting %s\n", args[0])
		return exitError
	}

	value, err := effectiveValue(record, id, &setting)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	source := "default"
	if install.IsOverridden(record, id, key) {
		source = "record"
	}
	fmt.Fprintln(out, value.String())
	fmt.Fprintf(out, "default: %s\nsource: %s\nkind: %s\napplies: %s\nhelp: %s\n", setting.Default, source, setting.Kind, setting.Applies, setting.Help)
	return exitClean
}

// settingsList prints one line per row: key, kind, current (an asterisk marks an override), default, help; --json prints the rows.
func settingsList(record install.Record, args []string, env install.Env, out io.Writer) int {
	flags := flag.NewFlagSet("settings list", flag.ContinueOnError)
	isJson := flags.Bool("json", false, "print the rows with their current values as JSON")
	if err := flags.Parse(args); err != nil {
		return parseExit(err)
	}
	if flags.NArg() > 1 {
		fmt.Fprintln(os.Stderr, settingsUsage)
		return exitError
	}

	filter := components.ID(flags.Arg(0))
	if _, ok := components.Lookup(filter); filter != "" && !ok {
		fmt.Fprintf(os.Stderr, "rules: settings: unknown component %s\n", filter)
		return exitError
	}

	rows, err := settingRows(record, env)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	if filter != "" {
		rows = slices.DeleteFunc(rows, func(row settingRow) bool { return row.Component != filter })
	}
	if *isJson {
		data, err := json.MarshalIndent(rows, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "rules:", err)
			return exitError
		}

		fmt.Fprintln(out, string(data))
		return exitClean
	}

	for _, row := range rows {
		marker := " "
		if row.IsOverridden {
			marker = "*"
		}
		fmt.Fprintf(out, "%s%s.%s\t%s\t%s\t(default %s)\t%s\n", marker, row.Component, row.Key, row.Kind, row.Current, row.Default, row.Help)
	}
	return exitClean
}

// settingsSet validates, stores the override, applies the component and prints what the apply wrote; a value equal to the row's default clears the override.
func settingsSet(ctx context.Context, record install.Record, args []string, env install.Env, out io.Writer) int {
	if len(args) != settingsSetArgCount {
		fmt.Fprintln(os.Stderr, settingsUsage)
		return exitError
	}

	id, key, ok := splitSettingName(args[0])
	if !ok {
		fmt.Fprintln(os.Stderr, "rules: settings: a setting is <component>.<key>")
		return exitError
	}

	setting, isKnown, err := findRow(env, id, key)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	if !isKnown {
		fmt.Fprintf(os.Stderr, "rules: settings: unknown setting %s\n", args[0])
		return exitError
	}

	value, err := components.ParseText(&setting, args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "rules: settings: %v\n  help: %s\n  valid: %s\n", err, setting.Help, validForm(&setting))
		return exitError
	}

	defaultValue, err := components.ParseText(&setting, setting.Default)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	install.ClearOverride(&record, id, key)
	isDefault := value.String() == defaultValue.String()
	if !isDefault {
		if err := install.SetOverride(&record, id, key, value); err != nil {
			fmt.Fprintln(os.Stderr, "rules:", err)
			return exitError
		}
	}

	results, err := install.ApplyComponent(ctx, env, record, id)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitViolations
	}
	fmt.Fprint(out, install.Summary(results))
	if install.Failed(results) {
		return exitViolations
	}

	fmt.Fprintf(out, "%s.%s = %s\n", id, key, value.String())
	if setting.Applies == components.AppliesRestart {
		fmt.Fprintln(out, "restart: the consumer was restarted")
		return exitClean
	}
	fmt.Fprintln(out, "reload: live")
	return exitClean
}

func splitSettingName(name string) (components.ID, string, bool) {
	id, key, ok := strings.Cut(name, ".")
	if !ok || id == "" || key == "" {
		return "", "", false
	}
	return components.ID(id), key, true
}

// validForm names what ParseText accepts for the kind.
func validForm(setting *components.Setting) string {
	switch setting.Kind {
	case components.KindBool:
		return "true or false"
	case components.KindInt:
		return "a non-negative integer"
	case components.KindDuration:
		return "a Go duration such as 5s or 250ms"
	case components.KindEnum:
		return "one of " + strings.Join(setting.Values, ", ")
	case components.KindRegex:
		return "a Go regular expression"
	case components.KindString:
		return "text"
	case components.KindStrings, components.KindGlobs, components.KindPaths, components.KindModels:
		return `a JSON string array such as ["a","b"]`
	}
	return ""
}

// writeOrCheck writes rendered to path, or with isCheck compares and fails on a difference.
func writeOrCheck(path string, rendered []byte, isCheck bool, command string) int {
	if !isCheck {
		if err := fsx.WriteFileAtomic(rendered, path, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "rules:", err)
			return exitError
		}
		return exitClean
	}

	current, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	if bytes.Equal(current, rendered) {
		fmt.Printf("%s: %s is current\n", command, path)
		return exitClean
	}
	fmt.Fprintf(os.Stderr, "%s: %s is stale — regenerate with make %s\n", command, path, command)
	return exitViolations
}

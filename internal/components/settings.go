package components

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	KindBool     Kind = "bool"
	KindDuration Kind = "duration"
	KindEnum     Kind = "enum"
	KindGlobs    Kind = "globs"
	KindInt      Kind = "int"
	KindModels   Kind = "models"
	KindPaths    Kind = "paths"
	KindRegex    Kind = "regex"
	KindString   Kind = "string"
	KindStrings  Kind = "strings"
)

const (
	SinkHeartbeatConfig SinkKind = "heartbeatConfig"
	SinkHookGroup       SinkKind = "hookGroup"
	SinkHooksConfig     SinkKind = "hooksConfig"
	SinkInstall         SinkKind = "install"
	SinkMarker          SinkKind = "marker"
)

const (
	AppliesLive    Applies = "live"
	AppliesRestart Applies = "restart"
)

var (
	kinds     = []Kind{KindBool, KindDuration, KindEnum, KindGlobs, KindInt, KindModels, KindPaths, KindRegex, KindString, KindStrings}
	listKinds = []Kind{KindGlobs, KindModels, KindPaths, KindStrings}
	sinkKinds = []SinkKind{SinkHeartbeatConfig, SinkHookGroup, SinkHooksConfig, SinkInstall, SinkMarker}
	// sinkTargetPattern: hooksConfig <section>.<field>, heartbeatConfig <key>, hookGroup <smineId>.<field>, marker {{NAME}}, install <Field>.
	sinkTargetPattern = regexp.MustCompile(`^([a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*|[a-z][a-z0-9-]*\.(timeout|matcher|async)|\{\{[A-Z_]+\}\}|[A-Z][A-Za-z]+)$`)
)

// Applies says what a changed value costs: live, or a restart of the consumer.
type Applies string

// Kind is a setting's value type; it decides the page widget, the CLI parser and the JSON rendering.
type Kind string

// Setting is one catalog row: the schema of one configurable value.
type Setting struct {
	Applies  Applies
	Default  string // the kind's text form
	Help     string // one sentence, shown on every surface
	IsHidden bool   // not on the Smine page; the CLI and the settings doc keep the row
	Key      string // unique within the component, snake_case segments joined by dots
	Kind     Kind
	Sink     Sink
	Values   []string // enum members, KindEnum only
}

// Sink is the consumer a value renders into.
type Sink struct {
	Kind   SinkKind
	Target string
}

// SinkKind names where a rendered value lands.
type SinkKind string

// Value is one parsed setting value; exactly the field its Kind names is set.
type Value struct {
	Bool     bool
	Duration time.Duration
	Int      int
	Kind     Kind
	List     []string
	Text     string
}

// MarshalJSON renders the consumer-file form: ms integers for durations, arrays for list kinds, scalars else.
func (v Value) MarshalJSON() ([]byte, error) {
	var payload any
	switch v.Kind {
	case KindBool:
		payload = v.Bool
	case KindInt:
		payload = v.Int
	case KindDuration:
		payload = v.Duration.Milliseconds()
	case KindString, KindEnum, KindRegex:
		payload = v.Text
	case KindStrings, KindGlobs, KindPaths, KindModels:
		payload = []string{}
		if v.List != nil {
			payload = v.List
		}
	}
	if payload == nil {
		return nil, fmt.Errorf("Value.MarshalJSON: Unknown kind %q", v.Kind)
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("Value.MarshalJSON: %w", err)
	}
	return data, nil
}

// String is the CLI and page form: the inverse of ParseText.
func (v Value) String() string {
	switch v.Kind {
	case KindBool:
		return strconv.FormatBool(v.Bool)
	case KindInt:
		return strconv.Itoa(v.Int)
	case KindDuration:
		return v.Duration.String()
	case KindString, KindEnum, KindRegex:
		return v.Text
	case KindStrings, KindGlobs, KindPaths, KindModels:
		list := v.List
		if list == nil {
			list = []string{}
		}
		data, _ := json.Marshal(list)
		return string(data)
	}
	return ""
}

// checkSetting is the schema check of one row: kind, help, sink shape, enum values, a default that parses.
func checkSetting(setting *Setting) error {
	// Kind
	if !slices.Contains(kinds, setting.Kind) {
		return fmt.Errorf("checkSetting: Unknown kind %q", setting.Kind)
	}

	// Help
	if strings.TrimSpace(setting.Help) == "" {
		return errors.New("checkSetting: Missing help")
	}

	// Sink
	if !slices.Contains(sinkKinds, setting.Sink.Kind) {
		return fmt.Errorf("checkSetting: Unknown sink %q", setting.Sink.Kind)
	}

	if !sinkTargetPattern.MatchString(setting.Sink.Target) {
		return fmt.Errorf("checkSetting: Sink target %q has the wrong shape", setting.Sink.Target)
	}

	// Applies
	if setting.Applies != AppliesLive && setting.Applies != AppliesRestart {
		return fmt.Errorf("checkSetting: Unknown applies %q", setting.Applies)
	}

	// Values
	hasValues := len(setting.Values) > 0
	if hasValues != (setting.Kind == KindEnum) {
		return errors.New("checkSetting: Values belong to enum rows only, and every enum row lists them")
	}

	// Default
	if _, err := ParseText(setting, setting.Default); err != nil {
		return fmt.Errorf("checkSetting: Default: %w", err)
	}
	return nil
}

// checkValue applies the per-kind constraints a parsed value still has to meet.
func checkValue(setting *Setting, value *Value) error {
	switch setting.Kind {
	case KindEnum:
		if !slices.Contains(setting.Values, value.Text) {
			return fmt.Errorf("checkValue: %s takes one of %s, not %q", setting.Key, strings.Join(setting.Values, ", "), value.Text)
		}
	case KindRegex:
		if _, err := regexp.Compile(value.Text); err != nil {
			return fmt.Errorf("checkValue: %s is not a regular expression: %w", setting.Key, err)
		}
	case KindGlobs, KindModels:
		for _, pattern := range value.List {
			if _, err := path.Match(pattern, ""); err != nil {
				return fmt.Errorf("checkValue: %s holds the bad pattern %q: %w", setting.Key, pattern, err)
			}
		}
	case KindInt:
		if value.Int < 0 {
			return fmt.Errorf("checkValue: %s takes no negative value", setting.Key)
		}
	case KindDuration:
		if value.Duration < 0 {
			return fmt.Errorf("checkValue: %s takes no negative duration", setting.Key)
		}
	case KindBool, KindString, KindStrings, KindPaths:
	}
	return nil
}

// CheckSettings refuses a catalog whose settings rows are not a valid schema; TestCatalog and install.Validate call it.
func CheckSettings() error {
	for _, component := range catalog {
		seen := map[string]bool{}
		for _, setting := range component.Settings {
			if err := checkSetting(&setting); err != nil {
				return fmt.Errorf("CheckSettings: %s.%s: %w", component.ID, setting.Key, err)
			}

			if seen[setting.Key] {
				return fmt.Errorf("CheckSettings: %s.%s: Duplicate key", component.ID, setting.Key)
			}

			seen[setting.Key] = true
		}
	}
	return nil
}

// IsListKind reports a kind whose value is a string list.
func IsListKind(kind Kind) bool {
	return slices.Contains(listKinds, kind)
}

// LookupSetting returns the static row for component.key.
func LookupSetting(id ID, key string) (Setting, bool) {
	component, ok := Lookup(id)
	if !ok {
		return Setting{}, false
	}

	index := slices.IndexFunc(component.Settings, func(setting Setting) bool { return setting.Key == key })
	if index < 0 {
		return Setting{}, false
	}
	return component.Settings[index], true
}

// ParseJson parses a consumer-form JSON value by kind — the record's override form.
func ParseJson(setting *Setting, raw json.RawMessage) (Value, error) {
	value := Value{Kind: setting.Kind}
	var err error
	switch setting.Kind {
	case KindBool:
		err = json.Unmarshal(raw, &value.Bool)
	case KindInt:
		err = json.Unmarshal(raw, &value.Int)
	case KindDuration:
		var millis int64
		err = json.Unmarshal(raw, &millis)
		value.Duration = time.Duration(millis) * time.Millisecond
	case KindString, KindEnum, KindRegex:
		err = json.Unmarshal(raw, &value.Text)
	case KindStrings, KindGlobs, KindPaths, KindModels:
		err = json.Unmarshal(raw, &value.List)
	}
	if err != nil {
		return Value{}, fmt.Errorf("ParseJson: %s is not a %s: %w", setting.Key, setting.Kind, err)
	}

	if err := checkValue(setting, &value); err != nil {
		return Value{}, fmt.Errorf("ParseJson: %w", err)
	}
	return value, nil
}

// ParseText parses the CLI and page form: true|false, a decimal, a Go duration, a JSON string array for list kinds, plain text else.
func ParseText(setting *Setting, text string) (Value, error) {
	value := Value{Kind: setting.Kind}
	var err error
	switch setting.Kind {
	case KindBool:
		value.Bool, err = strconv.ParseBool(text)
	case KindInt:
		value.Int, err = strconv.Atoi(text)
	case KindDuration:
		value.Duration, err = time.ParseDuration(text)
	case KindString, KindEnum, KindRegex:
		value.Text = text
	case KindStrings, KindGlobs, KindPaths, KindModels:
		err = json.Unmarshal([]byte(text), &value.List)
	}
	if err != nil {
		return Value{}, fmt.Errorf("ParseText: %s takes a %s, not %q: %w", setting.Key, setting.Kind, text, err)
	}

	if err := checkValue(setting, &value); err != nil {
		return Value{}, fmt.Errorf("ParseText: %w", err)
	}
	return value, nil
}

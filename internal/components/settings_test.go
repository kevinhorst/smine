package components

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// catalogWithContextRow is the catalog with the Context component's first row replaced by setting.
func catalogWithContextRow(setting Setting) []Component {
	components := Catalog()
	for i := range components {
		if components[i].ID != Context {
			continue
		}

		components[i].Settings = slices.Clone(components[i].Settings)
		components[i].Settings[0] = setting
	}
	return components
}

// checkSettingsOf runs CheckSettings over components in place of the catalog.
func checkSettingsOf(components []Component) error {
	saved := catalog
	catalog = components
	defer func() { catalog = saved }()
	return CheckSettings()
}

// provideSetting is a valid hooksConfig row of kind; an enum row lists a, b, c.
func provideSetting(kind Kind) *Setting {
	setting := &Setting{
		Applies: AppliesLive,
		Default: "",
		Help:    "A test row.",
		Key:     "test_key",
		Kind:    kind,
		Sink:    Sink{Kind: SinkHooksConfig, Target: "section.test_key"},
		Values:  nil,
	}
	switch kind {
	case KindBool:
		setting.Default = "false"
	case KindInt:
		setting.Default = "0"
	case KindDuration:
		setting.Default = "1s"
	case KindEnum:
		setting.Default = "a"
		setting.Values = []string{"a", "b", "c"}
	case KindStrings, KindGlobs, KindPaths, KindModels:
		setting.Default = "[]"
	case KindString, KindRegex:
	}
	return setting
}

func TestCheckSettings(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool

		components []Component
	}

	tests := make([]*testCase, 0)

	// pass-catalog
	tests = append(tests, &testCase{
		_id:         "pass-catalog",
		_shouldPass: true,

		components: Catalog(),
	})

	// fail-unknown-kind
	setting := *provideSetting(KindString)
	setting.Kind = "float"
	tests = append(tests, &testCase{
		_id:         "fail-unknown-kind",
		_shouldPass: false,

		components: catalogWithContextRow(setting),
	})

	// fail-empty-help
	setting = *provideSetting(KindString)
	setting.Help = "  "
	tests = append(tests, &testCase{
		_id:         "fail-empty-help",
		_shouldPass: false,

		components: catalogWithContextRow(setting),
	})

	// fail-enum-without-values
	setting = *provideSetting(KindEnum)
	setting.Values = nil
	tests = append(tests, &testCase{
		_id:         "fail-enum-without-values",
		_shouldPass: false,

		components: catalogWithContextRow(setting),
	})

	// fail-values-on-string
	setting = *provideSetting(KindString)
	setting.Values = []string{"a"}
	tests = append(tests, &testCase{
		_id:         "fail-values-on-string",
		_shouldPass: false,

		components: catalogWithContextRow(setting),
	})

	// fail-bad-sink-target
	setting = *provideSetting(KindString)
	setting.Sink.Target = "Section.Field"
	tests = append(tests, &testCase{
		_id:         "fail-bad-sink-target",
		_shouldPass: false,

		components: catalogWithContextRow(setting),
	})

	// fail-default-not-parsing
	setting = *provideSetting(KindInt)
	setting.Default = "many"
	tests = append(tests, &testCase{
		_id:         "fail-default-not-parsing",
		_shouldPass: false,

		components: catalogWithContextRow(setting),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := checkSettingsOf(test.components)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
		})
	}
}

func TestParseText(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   Value
		_shouldPass bool

		setting *Setting
		text    string
	}

	tests := make([]*testCase, 0)

	// pass-bool
	tests = append(tests, &testCase{
		_id:         "pass-bool",
		_expected:   Value{Bool: true, Kind: KindBool},
		_shouldPass: true,

		setting: provideSetting(KindBool),
		text:    "true",
	})

	// pass-int
	tests = append(tests, &testCase{
		_id:         "pass-int",
		_expected:   Value{Int: 9000, Kind: KindInt},
		_shouldPass: true,

		setting: provideSetting(KindInt),
		text:    "9000",
	})

	// fail-negative-int
	tests = append(tests, &testCase{
		_id:         "fail-negative-int",
		_expected:   Value{},
		_shouldPass: false,

		setting: provideSetting(KindInt),
		text:    "-1",
	})

	// pass-duration
	tests = append(tests, &testCase{
		_id:         "pass-duration",
		_expected:   Value{Duration: 200 * time.Millisecond, Kind: KindDuration},
		_shouldPass: true,

		setting: provideSetting(KindDuration),
		text:    "200ms",
	})

	// fail-negative-duration
	tests = append(tests, &testCase{
		_id:         "fail-negative-duration",
		_expected:   Value{},
		_shouldPass: false,

		setting: provideSetting(KindDuration),
		text:    "-5s",
	})

	// pass-enum
	tests = append(tests, &testCase{
		_id:         "pass-enum",
		_expected:   Value{Kind: KindEnum, Text: "b"},
		_shouldPass: true,

		setting: provideSetting(KindEnum),
		text:    "b",
	})

	// fail-enum-outside-values
	tests = append(tests, &testCase{
		_id:         "fail-enum-outside-values",
		_expected:   Value{},
		_shouldPass: false,

		setting: provideSetting(KindEnum),
		text:    "d",
	})

	// pass-regex
	tests = append(tests, &testCase{
		_id:         "pass-regex",
		_expected:   Value{Kind: KindRegex, Text: `^[a-z]+$`},
		_shouldPass: true,

		setting: provideSetting(KindRegex),
		text:    `^[a-z]+$`,
	})

	// fail-bad-regex
	tests = append(tests, &testCase{
		_id:         "fail-bad-regex",
		_expected:   Value{},
		_shouldPass: false,

		setting: provideSetting(KindRegex),
		text:    `^[a-z+$`,
	})

	// pass-strings-json
	tests = append(tests, &testCase{
		_id:         "pass-strings-json",
		_expected:   Value{Kind: KindStrings, List: []string{"main", "master"}},
		_shouldPass: true,

		setting: provideSetting(KindStrings),
		text:    `["main","master"]`,
	})

	// fail-strings-not-array
	tests = append(tests, &testCase{
		_id:         "fail-strings-not-array",
		_expected:   Value{},
		_shouldPass: false,

		setting: provideSetting(KindStrings),
		text:    "main,master",
	})

	// fail-bad-glob
	tests = append(tests, &testCase{
		_id:         "fail-bad-glob",
		_expected:   Value{},
		_shouldPass: false,

		setting: provideSetting(KindGlobs),
		text:    `["[a-"]`,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			value, err := ParseText(test.setting, test.text)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, value)
		})
	}
}

func TestParseJson(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   Value
		_shouldPass bool

		raw     json.RawMessage
		setting *Setting
	}

	tests := make([]*testCase, 0)

	// pass-bool
	tests = append(tests, &testCase{
		_id:         "pass-bool",
		_expected:   Value{Bool: true, Kind: KindBool},
		_shouldPass: true,

		raw:     json.RawMessage(`true`),
		setting: provideSetting(KindBool),
	})

	// pass-int
	tests = append(tests, &testCase{
		_id:         "pass-int",
		_expected:   Value{Int: 9000, Kind: KindInt},
		_shouldPass: true,

		raw:     json.RawMessage(`9000`),
		setting: provideSetting(KindInt),
	})

	// fail-negative-int
	tests = append(tests, &testCase{
		_id:         "fail-negative-int",
		_expected:   Value{},
		_shouldPass: false,

		raw:     json.RawMessage(`-1`),
		setting: provideSetting(KindInt),
	})

	// pass-duration
	tests = append(tests, &testCase{
		_id:         "pass-duration",
		_expected:   Value{Duration: 200 * time.Millisecond, Kind: KindDuration},
		_shouldPass: true,

		raw:     json.RawMessage(`200`),
		setting: provideSetting(KindDuration),
	})

	// fail-negative-duration
	tests = append(tests, &testCase{
		_id:         "fail-negative-duration",
		_expected:   Value{},
		_shouldPass: false,

		raw:     json.RawMessage(`-5000`),
		setting: provideSetting(KindDuration),
	})

	// pass-enum
	tests = append(tests, &testCase{
		_id:         "pass-enum",
		_expected:   Value{Kind: KindEnum, Text: "b"},
		_shouldPass: true,

		raw:     json.RawMessage(`"b"`),
		setting: provideSetting(KindEnum),
	})

	// fail-enum-outside-values
	tests = append(tests, &testCase{
		_id:         "fail-enum-outside-values",
		_expected:   Value{},
		_shouldPass: false,

		raw:     json.RawMessage(`"d"`),
		setting: provideSetting(KindEnum),
	})

	// pass-regex
	tests = append(tests, &testCase{
		_id:         "pass-regex",
		_expected:   Value{Kind: KindRegex, Text: `^[a-z]+$`},
		_shouldPass: true,

		raw:     json.RawMessage(`"^[a-z]+$"`),
		setting: provideSetting(KindRegex),
	})

	// fail-bad-regex
	tests = append(tests, &testCase{
		_id:         "fail-bad-regex",
		_expected:   Value{},
		_shouldPass: false,

		raw:     json.RawMessage(`"^[a-z+$"`),
		setting: provideSetting(KindRegex),
	})

	// pass-strings-json
	tests = append(tests, &testCase{
		_id:         "pass-strings-json",
		_expected:   Value{Kind: KindStrings, List: []string{"main", "master"}},
		_shouldPass: true,

		raw:     json.RawMessage(`["main","master"]`),
		setting: provideSetting(KindStrings),
	})

	// fail-strings-not-array
	tests = append(tests, &testCase{
		_id:         "fail-strings-not-array",
		_expected:   Value{},
		_shouldPass: false,

		raw:     json.RawMessage(`"main"`),
		setting: provideSetting(KindStrings),
	})

	// fail-bad-glob
	tests = append(tests, &testCase{
		_id:         "fail-bad-glob",
		_expected:   Value{},
		_shouldPass: false,

		raw:     json.RawMessage(`["[a-"]`),
		setting: provideSetting(KindGlobs),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			value, err := ParseJson(test.setting, test.raw)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, value)
		})
	}
}

func TestValue_String_RoundTrip(t *testing.T) {
	type testCase struct {
		_id string

		setting *Setting
		text    string
	}

	tests := make([]*testCase, 0)

	// bool
	tests = append(tests, &testCase{
		_id: "bool",

		setting: provideSetting(KindBool),
		text:    "true",
	})

	// int
	tests = append(tests, &testCase{
		_id: "int",

		setting: provideSetting(KindInt),
		text:    "16384",
	})

	// duration
	tests = append(tests, &testCase{
		_id: "duration",

		setting: provideSetting(KindDuration),
		text:    "1m30s",
	})

	// string
	tests = append(tests, &testCase{
		_id: "string",

		setting: provideSetting(KindString),
		text:    "claude/",
	})

	// enum
	tests = append(tests, &testCase{
		_id: "enum",

		setting: provideSetting(KindEnum),
		text:    "c",
	})

	// regex
	tests = append(tests, &testCase{
		_id: "regex",

		setting: provideSetting(KindRegex),
		text:    `^[A-Za-z0-9][A-Za-z0-9._-]*$`,
	})

	// strings
	tests = append(tests, &testCase{
		_id: "strings",

		setting: provideSetting(KindStrings),
		text:    `["acdsl","acdsl.exe"]`,
	})

	// globs
	tests = append(tests, &testCase{
		_id: "globs",

		setting: provideSetting(KindGlobs),
		text:    `["plans/**","*.go"]`,
	})

	// paths
	tests = append(tests, &testCase{
		_id: "paths",

		setting: provideSetting(KindPaths),
		text:    `["~/.claude/plans"]`,
	})

	// models
	tests = append(tests, &testCase{
		_id: "models",

		setting: provideSetting(KindModels),
		text:    `["*fable*"]`,
	})

	// empty-list
	tests = append(tests, &testCase{
		_id: "empty-list",

		setting: provideSetting(KindStrings),
		text:    "[]",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			value, err := ParseText(test.setting, test.text)
			assert.NoError(t, err)
			assert.Equal(t, test.text, value.String())
		})
	}
}

func TestValue_MarshalJSON(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		value Value
	}

	tests := make([]*testCase, 0)

	// bool
	tests = append(tests, &testCase{
		_id:       "bool",
		_expected: `true`,

		value: Value{Bool: true, Kind: KindBool},
	})

	// int
	tests = append(tests, &testCase{
		_id:       "int",
		_expected: `55000`,

		value: Value{Int: 55000, Kind: KindInt},
	})

	// duration-as-ms
	tests = append(tests, &testCase{
		_id:       "duration-as-ms",
		_expected: `604800000`,

		value: Value{Duration: 168 * time.Hour, Kind: KindDuration},
	})

	// string
	tests = append(tests, &testCase{
		_id:       "string",
		_expected: `"claude/"`,

		value: Value{Kind: KindString, Text: "claude/"},
	})

	// enum
	tests = append(tests, &testCase{
		_id:       "enum",
		_expected: `"keychain"`,

		value: Value{Kind: KindEnum, Text: "keychain"},
	})

	// regex
	tests = append(tests, &testCase{
		_id:       "regex",
		_expected: `"^a+$"`,

		value: Value{Kind: KindRegex, Text: "^a+$"},
	})

	// strings
	tests = append(tests, &testCase{
		_id:       "strings",
		_expected: `["main","master"]`,

		value: Value{Kind: KindStrings, List: []string{"main", "master"}},
	})

	// empty-list-as-array
	tests = append(tests, &testCase{
		_id:       "empty-list-as-array",
		_expected: `[]`,

		value: Value{Kind: KindGlobs, List: nil},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			data, err := json.Marshal(test.value)
			assert.NoError(t, err)
			assert.JSONEq(t, test._expected, string(data))
		})
	}
}

package install

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kevinhorst/smine/internal/components"
)

func TestResolve(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   components.Value
		_shouldPass bool

		id     components.ID
		key    string
		record Record
	}

	tests := make([]*testCase, 0)

	// default
	tests = append(tests, &testCase{
		_id:         "default",
		_expected:   components.Value{Int: 6001, Kind: components.KindInt},
		_shouldPass: true,

		id:     components.ConfigServer,
		key:    "port",
		record: FullRecord(goosDarwin),
	})

	// override
	overridden := FullRecord(goosDarwin)
	overridden.Settings = Settings{components.ConfigServer: {"port": json.RawMessage(`6002`)}}
	tests = append(tests, &testCase{
		_id:         "override",
		_expected:   components.Value{Int: 6002, Kind: components.KindInt},
		_shouldPass: true,

		id:     components.ConfigServer,
		key:    "port",
		record: overridden,
	})

	// fail-unknown-key
	tests = append(tests, &testCase{
		_id:         "fail-unknown-key",
		_expected:   components.Value{},
		_shouldPass: false,

		id:     components.ConfigServer,
		key:    "nope",
		record: FullRecord(goosDarwin),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			value, err := Resolve(test.record, test.id, test.key)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, value)
		})
	}
}

func TestSetOverride(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   Settings
		_shouldPass bool

		key    string
		record Record
		value  components.Value
	}

	tests := make([]*testCase, 0)

	// override
	tests = append(tests, &testCase{
		_id:         "override",
		_expected:   Settings{components.ConfigServer: {"port": json.RawMessage(`6002`)}},
		_shouldPass: true,

		key:    "port",
		record: FullRecord(goosDarwin),
		value:  components.Value{Int: 6002, Kind: components.KindInt},
	})

	// default-equal-clears
	overridden := FullRecord(goosDarwin)
	overridden.Settings = Settings{components.ConfigServer: {"port": json.RawMessage(`6002`)}}
	tests = append(tests, &testCase{
		_id:         "default-equal-clears",
		_expected:   nil,
		_shouldPass: true,

		key:    "port",
		record: overridden,
		value:  components.Value{Int: 6001, Kind: components.KindInt},
	})

	// fail-unknown-key
	tests = append(tests, &testCase{
		_id:         "fail-unknown-key",
		_expected:   nil,
		_shouldPass: false,

		key:    "nope",
		record: FullRecord(goosDarwin),
		value:  components.Value{Int: 6002, Kind: components.KindInt},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := SetOverride(&test.record, components.ConfigServer, test.key, test.value)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, test.record.Settings)
		})
	}
}

func TestInstallValues(t *testing.T) {
	type testCase struct {
		_id       string
		_expected Values

		record Record
	}

	notificationSound, _ := components.LookupSetting(components.SessionHelpers, "notification_sound")
	stopSound, _ := components.LookupSetting(components.SessionHelpers, "stop_sound")
	defaultValues := Values{
		ConfigServerPort:   "6001",
		CountTokens:        "off",
		HeartbeatPort:      "6101",
		InitWelcome:        false,
		NotificationSound:  notificationSound.Default,
		PeekClaudeHome:     "",
		PeekCodexHome:      "",
		PeekControlPort:    "42442",
		PeekPort:           "4242",
		ServerPath:         os.Getenv("PATH"),
		StopSound:          stopSound.Default,
		TrashRetentionDays: 0,
	}

	tests := make([]*testCase, 0)

	// trash-retention-default-zero
	tests = append(tests, &testCase{
		_id:       "trash-retention-default-zero",
		_expected: defaultValues,

		record: FullRecord(goosDarwin),
	})

	// trash-retention-override
	overridden := FullRecord(goosDarwin)
	overridden.Settings = Settings{components.ConfigServer: {"trash_retention_days": json.RawMessage(`30`)}}
	overriddenValues := defaultValues
	overriddenValues.TrashRetentionDays = 30
	tests = append(tests, &testCase{
		_id:       "trash-retention-override",
		_expected: overriddenValues,

		record: overridden,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			values, err := InstallValues(test.record)
			assert.NoError(t, err)
			assert.Equal(t, test._expected, values)
		})
	}
}

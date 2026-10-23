package install

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
)

func TestSchema(t *testing.T) {
	env := newTestEnv(t)
	env.RepoDir = filepath.Join("..", "..")
	component, _ := components.Lookup(components.SessionHelpers)
	fragment, err := config.LoadComponentFragment(settingsDir(env), string(components.SessionHelpers), env.markers())
	require.NoError(t, err)
	groups, err := fragment.HookGroups()
	require.NoError(t, err)

	rows, err := Schema(env, components.SessionHelpers)
	require.NoError(t, err)

	// static-rows-first
	assert.Equal(t, component.Settings, rows[:len(component.Settings)])

	// three-rows-per-group
	derived := rows[len(component.Settings):]
	assert.Len(t, derived, 3*len(groups))

	// group-defaults-from-fragment
	byKey := map[string]components.Setting{}
	for _, row := range derived {
		byKey[row.Key] = row
	}
	assert.Equal(t, "5", byKey["groups.session-helpers-pretooluse-sessiontitle.timeout"].Default)
	assert.Equal(t, "mcp__ccd_session_mgmt__set_session_title", byKey["groups.session-helpers-pretooluse-sessiontitle.matcher"].Default)
	assert.Equal(t, "true", byKey["groups.session-helpers-stop-play-sound.async"].Default)
	assert.Equal(t, "0", byKey["groups.session-helpers-stop-play-sound.timeout"].Default)
	assert.Equal(t, components.Sink{Kind: components.SinkHookGroup, Target: "session-helpers-stop-play-sound.async"}, byKey["groups.session-helpers-stop-play-sound.async"].Sink)
}

func TestLookupRow(t *testing.T) {
	type testCase struct {
		_id          string
		_expectedKey string
		_shouldPass  bool

		key string
	}

	env := newTestEnv(t)
	env.RepoDir = filepath.Join("..", "..")
	tests := make([]*testCase, 0)

	// static-row
	tests = append(tests, &testCase{_id: "static-row", _expectedKey: "stop_sound", _shouldPass: true, key: "stop_sound"})

	// derived-row
	tests = append(tests, &testCase{_id: "derived-row", _expectedKey: "groups.session-helpers-stop-play-sound.async", _shouldPass: true, key: "groups.session-helpers-stop-play-sound.async"})

	// fail-unknown-group
	tests = append(tests, &testCase{_id: "fail-unknown-group", _expectedKey: "", _shouldPass: false, key: "groups.nope.async"})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			row, err := LookupRow(env, components.SessionHelpers, test.key)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedKey, row.Key)
		})
	}
}

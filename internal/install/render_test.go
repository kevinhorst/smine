package install

import (
	"encoding/json"
	"maps"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
)

// heartbeatRawConfigKeys are the keys cmd/heartbeat/src/config.rs RawConfig accepts under deny_unknown_fields.
var heartbeatRawConfigKeys = []string{
	"attack_deny_reason", "attack_inherited_reason", "attack_stop_reason", "attack_substituted_reason",
	"claim_poll_ms", "claim_polls", "client_timeout_ms", "collision_prefixes",
	"digest_line_paths", "digest_lines", "exempt_paths", "exits_batch", "exits_poll_ms",
	"forbidden_subagent_models", "guarded_tools", "hook_max_body", "hook_max_detail",
	"hook_max_subject", "inbox_capacity", "kill_at", "kill_reason",
	"ledger_second_ms", "ledger_tail_window", "max_ended_runs", "max_field",
	"max_id_len", "max_refused_unknown", "max_terminal", "member_deny_reason",
	"member_role_text", "merge_branch_prefix", "peek_addr", "peek_call_timeout_ms",
	"peek_healthz_timeout_ms", "peek_retry_ms", "runtime_digest_lines", "runtime_stop_reason",
	"scan_settle_ms", "shutdown_timeout_ms", "socket_write_timeout_ms", "staging_target",
	"subscriber_deny_reason", "transcript_max_lines", "version_keys", "waiting_notifications",
	"waiting_tools", "worker_branch_prefix", "workers",
}

// decodeRendered parses a rendered document into its top-level keys.
func decodeRendered(t *testing.T, data []byte) map[string]json.RawMessage {
	t.Helper()
	var document map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &document))
	return document
}

func TestRenderDocument(t *testing.T) {
	env := newTestEnv(t)
	record := FullRecord(goosDarwin)
	require.NoError(t, setText(&record, components.Context, "context_dir", "docs"))
	require.NoError(t, setText(&record, components.Peek, "port", "4300"))

	// nested-sections
	hooks, err := RenderDocument(env, record, components.SinkHooksConfig)
	require.NoError(t, err)
	sections := decodeRendered(t, hooks)
	assert.JSONEq(t, `{"context_dir": "docs"}`, string(sections["context"]))
	assert.Contains(t, sections, "toolpolicy")
	assert.Contains(t, sections, "sounds")

	// duration-as-ms
	var readgate map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(sections["readgate"], &readgate))
	assert.JSONEq(t, `604800000`, string(readgate["ledger_retention_ms"]))

	// heartbeat-keys-match-raw-config
	heartbeat, err := RenderDocument(env, record, components.SinkHeartbeatConfig)
	require.NoError(t, err)
	keys := decodeRendered(t, heartbeat)
	assert.Equal(t, heartbeatRawConfigKeys, slices.Sorted(maps.Keys(keys)))
	assert.JSONEq(t, `"127.0.0.1:4300"`, string(keys["peek_addr"]))
	assert.JSONEq(t, `""`, string(keys["staging_target"]))

	// unselected-component-renders-nothing
	minimal := customRecord(components.ConfigServer)
	empty, err := RenderDocument(env, minimal, components.SinkHeartbeatConfig)
	require.NoError(t, err)
	assert.Equal(t, "{}\n", string(empty))
}

func TestRenderDefaults(t *testing.T) {
	data, err := RenderDefaults(components.Heartbeat, components.SinkHeartbeatConfig)
	require.NoError(t, err)

	keys := decodeRendered(t, data)
	withoutPeekAddr := slices.DeleteFunc(slices.Clone(heartbeatRawConfigKeys), func(key string) bool { return key == heartbeatPeekAddrKey })
	assert.Equal(t, withoutPeekAddr, slices.Sorted(maps.Keys(keys)))
	assert.JSONEq(t, `2`, string(keys["workers"]))
	assert.JSONEq(t, `30000`, string(keys["peek_retry_ms"]))
	assert.JSONEq(t, `[]`, string(keys["forbidden_subagent_models"]))
}

func TestRestartKeysChanged(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		deployed string
		record   Record
	}

	env := newTestEnv(t)
	base := FullRecord(goosDarwin)
	deployed, err := RenderDocument(env, base, components.SinkHeartbeatConfig)
	require.NoError(t, err)
	liveChange := FullRecord(goosDarwin)
	require.NoError(t, setText(&liveChange, components.Heartbeat, "kill_at", "3"))
	restartChange := FullRecord(goosDarwin)
	require.NoError(t, setText(&restartChange, components.Heartbeat, "workers", "4"))
	peekChange := FullRecord(goosDarwin)
	require.NoError(t, setText(&peekChange, components.Peek, "port", "4300"))
	tests := make([]*testCase, 0)

	// unchanged-no-restart
	tests = append(tests, &testCase{_id: "unchanged-no-restart", _expected: false, deployed: string(deployed), record: base})

	// live-change-no-restart
	tests = append(tests, &testCase{_id: "live-change-no-restart", _expected: false, deployed: string(deployed), record: liveChange})

	// restart-key-restarts
	tests = append(tests, &testCase{_id: "restart-key-restarts", _expected: true, deployed: string(deployed), record: restartChange})

	// peek-addr-restarts
	tests = append(tests, &testCase{_id: "peek-addr-restarts", _expected: true, deployed: string(deployed), record: peekChange})

	// missing-file-restarts
	tests = append(tests, &testCase{_id: "missing-file-restarts", _expected: true, deployed: "", record: base})

	// unparseable-file-restarts
	tests = append(tests, &testCase{_id: "unparseable-file-restarts", _expected: true, deployed: "{", record: base})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			testEnv := newTestEnv(t)
			if test.deployed != "" {
				writeTestFile(t, testEnv.ClaudeHome, heartbeatConfigTarget, test.deployed)
			}
			rendered, err := RenderDocument(testEnv, test.record, components.SinkHeartbeatConfig)
			require.NoError(t, err)

			isChanged, err := restartKeysChanged(testEnv, rendered)
			assert.NoError(t, err)
			assert.Equal(t, test._expected, isChanged)
		})
	}
}

func TestApplyGroupFields(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		group  string
		record Record
	}

	group := `{"smineId": "session-helpers-pretooluse-sessiontitle", "smineName": "Session title", "smineVersion": "1.0.1", "matcher": "x", "hooks": [{"type": "command", "command": "c", "timeout": 5}]}`
	timeoutRecord := FullRecord(goosDarwin)
	timeoutRecord.Settings = Settings{components.SessionHelpers: {"groups.session-helpers-pretooluse-sessiontitle.timeout": json.RawMessage(`9`)}}
	matcherRecord := FullRecord(goosDarwin)
	matcherRecord.Settings = Settings{components.SessionHelpers: {"groups.session-helpers-pretooluse-sessiontitle.matcher": json.RawMessage(`"Bash"`)}}
	tests := make([]*testCase, 0)

	// timeout-on-hook
	tests = append(tests, &testCase{
		_id:       "timeout-on-hook",
		_expected: `{"smineId": "session-helpers-pretooluse-sessiontitle", "smineName": "Session title", "smineVersion": "1.0.1", "matcher": "x", "hooks": [{"type": "command", "command": "c", "timeout": 9}]}`,

		group:  group,
		record: timeoutRecord,
	})

	// matcher-on-group
	tests = append(tests, &testCase{
		_id:       "matcher-on-group",
		_expected: `{"smineId": "session-helpers-pretooluse-sessiontitle", "smineName": "Session title", "smineVersion": "1.0.1", "matcher": "Bash", "hooks": [{"type": "command", "command": "c", "timeout": 5}]}`,

		group:  group,
		record: matcherRecord,
	})

	// no-override-unchanged
	tests = append(tests, &testCase{
		_id:       "no-override-unchanged",
		_expected: group,

		group:  group,
		record: FullRecord(goosDarwin),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			rendered, err := applyGroupFields(test.record, components.SessionHelpers, json.RawMessage(test.group))
			assert.NoError(t, err)
			assert.JSONEq(t, test._expected, string(rendered))
		})
	}
}

func TestRenderedFile_Write(t *testing.T) {
	env := newTestEnv(t)
	file := renderedFile{dirPerm: privateDirPerm, sink: components.SinkHeartbeatConfig, target: heartbeatConfigTarget}

	write, err := file.write(env, FullRecord(goosDarwin))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(env.ClaudeHome, "heartbeat", "config.json"), write.target)
	assert.Contains(t, decodeRendered(t, write.data), "workers")
}

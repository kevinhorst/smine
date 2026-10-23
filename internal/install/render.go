package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/hookconfig"
)

// hooksConfigTarget and heartbeatConfigTarget are the rendered files, Claude-home-relative.
const (
	heartbeatConfigTarget = "heartbeat/config.json"
	hooksConfigTarget     = "hooks/" + hookconfig.File
)

// heartbeatPeekAddrKey is the daemon's peek address, derived from peek.port rather than a row of its own.
const heartbeatPeekAddrKey = "peek_addr"

// loopbackHost is the host every local smine endpoint binds.
const loopbackHost = "127.0.0.1"

// renderedDocument is one consumer file as the sink keys of every selected component resolve it.
type renderedDocument map[string]any

// marshal renders the document two-space indented, keys sorted, newline-terminated.
func (d renderedDocument) marshal() ([]byte, error) {
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("renderedDocument.marshal: %w", err)
	}
	return append(data, '\n'), nil
}

// set places value at the dotted target, creating the section objects on the way.
func (d renderedDocument) set(target string, value any) {
	segments := strings.Split(target, ".")
	node := map[string]any(d)
	for _, segment := range segments[:len(segments)-1] {
		child, ok := node[segment].(map[string]any)
		if !ok {
			child = map[string]any{}
			node[segment] = child
		}
		node = child
	}
	node[segments[len(segments)-1]] = value
}

// renderedFile is a file artifact the apply renders from the schema instead of copying from the repo.
type renderedFile struct {
	dirPerm os.FileMode
	sink    components.SinkKind
	target  string // Claude-home-relative
}

// write resolves the file's bytes for the record.
func (f renderedFile) write(env Env, record Record) (fileWrite, error) {
	data, err := RenderDocument(env, record, f.sink)
	if err != nil {
		return fileWrite{}, err
	}
	write := fileWrite{data: data, dirPerm: f.dirPerm, perm: 0o644, target: filepath.Join(env.ClaudeHome, f.target)}
	return write, nil
}

// applyGroupFields sets the hookGroup sink's overridden values on a fragment's raw group before the smineId reconcile.
func applyGroupFields(record Record, id components.ID, group json.RawMessage) (json.RawMessage, error) {
	decoded, err := decodeGroup(group)
	if err != nil {
		return nil, fmt.Errorf("applyGroupFields: %w", err)
	}

	document := config.NewDocument()
	if err := json.Unmarshal(group, document); err != nil {
		return nil, fmt.Errorf("applyGroupFields: %w", err)
	}
	isChanged := false
	for _, field := range groupFields {
		key := groupKeyPrefix + decoded.Id + "." + field.name
		if !IsOverridden(record, id, key) {
			continue
		}

		value, err := Resolve(record, id, key)
		if err != nil {
			return nil, fmt.Errorf("applyGroupFields: %w", err)
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("applyGroupFields: %w", err)
		}
		if err := setGroupField(document, field.name, raw); err != nil {
			return nil, fmt.Errorf("applyGroupFields: %s: %w", key, err)
		}
		isChanged = true
	}
	if !isChanged {
		return group, nil
	}

	rendered, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("applyGroupFields: %w", err)
	}
	return rendered, nil
}

// heartbeatPeekAddr is the peek address the daemon dials: the loopback host and peek.port.
func heartbeatPeekAddr(record Record) (string, error) {
	port, err := Resolve(record, components.Peek, "port")
	if err != nil {
		return "", fmt.Errorf("heartbeatPeekAddr: %w", err)
	}
	return loopbackHost + ":" + strconv.Itoa(port.Int), nil
}

// rendersHooksConfig reports whether a component owns rows of the hooks config, which the Session hooks component renders.
func rendersHooksConfig(id components.ID) bool {
	component, _ := components.Lookup(id)
	return slices.ContainsFunc(component.Settings, func(setting components.Setting) bool {
		return setting.Sink.Kind == components.SinkHooksConfig
	})
}

// restartKeysChanged reports whether a restart-class heartbeatConfig value differs between the deployed file and the rendered one.
func restartKeysChanged(env Env, rendered []byte) (bool, error) {
	current, err := os.ReadFile(filepath.Join(env.ClaudeHome, heartbeatConfigTarget))
	if errors.Is(err, fs.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("restartKeysChanged: %w", err)
	}

	var before, after map[string]json.RawMessage
	if err := json.Unmarshal(current, &before); err != nil {
		return true, nil // an unparseable deployed file is replaced and the daemon restarted
	}
	if err := json.Unmarshal(rendered, &after); err != nil {
		return false, fmt.Errorf("restartKeysChanged: %w", err)
	}

	restartKeys := []string{heartbeatPeekAddrKey}
	component, _ := components.Lookup(components.Heartbeat)
	for _, row := range component.Settings {
		isRestartRow := row.Sink.Kind == components.SinkHeartbeatConfig && row.Applies == components.AppliesRestart
		if isRestartRow {
			restartKeys = append(restartKeys, row.Sink.Target)
		}
	}
	isChanged := slices.ContainsFunc(restartKeys, func(key string) bool {
		return !isSameJson(before[key], after[key])
	})
	return isChanged, nil
}

// copyGroupFields carries the render-owned fields of wanted onto live — a field wanted lacks is removed — and keeps the rest of live.
func copyGroupFields(live, wanted json.RawMessage) (json.RawMessage, error) {
	liveGroup := config.NewDocument()
	if err := json.Unmarshal(live, liveGroup); err != nil {
		return nil, fmt.Errorf("copyGroupFields: %w", err)
	}
	wantedGroup := config.NewDocument()
	if err := json.Unmarshal(wanted, wantedGroup); err != nil {
		return nil, fmt.Errorf("copyGroupFields: %w", err)
	}

	for _, field := range groupFields {
		raw, err := readGroupField(wantedGroup, field.name)
		if err != nil {
			return nil, fmt.Errorf("copyGroupFields: %w", err)
		}
		if err := setGroupField(liveGroup, field.name, raw); err != nil {
			return nil, fmt.Errorf("copyGroupFields: %w", err)
		}
	}

	patched, err := json.Marshal(liveGroup)
	if err != nil {
		return nil, fmt.Errorf("copyGroupFields: %w", err)
	}
	return patched, nil
}

// readGroupField reads one group field: the matcher on the group, timeout and async on its first hook; nil when absent.
func readGroupField(group *config.Document, name string) (json.RawMessage, error) {
	if name == groupFieldMatcher {
		raw, _ := group.Get([]string{name})
		return raw, nil
	}

	hooks, err := group.List([]string{hooksKey})
	if err != nil {
		return nil, fmt.Errorf("readGroupField: %w", err)
	}
	if len(hooks) == 0 {
		return nil, nil
	}

	hook := config.NewDocument()
	if err := json.Unmarshal(hooks[0], hook); err != nil {
		return nil, fmt.Errorf("readGroupField: %w", err)
	}
	raw, _ := hook.Get([]string{name})
	return raw, nil
}

// setGroupField writes one group field — the matcher on the group, timeout and async on its first hook — and removes it for a nil raw.
func setGroupField(group *config.Document, name string, raw json.RawMessage) error {
	if name == groupFieldMatcher {
		if raw == nil {
			group.Unset([]string{name})
			return nil
		}
		return group.Set([]string{name}, raw)
	}

	hooks, err := group.List([]string{hooksKey})
	if err != nil {
		return fmt.Errorf("setGroupField: %w", err)
	}
	if len(hooks) == 0 {
		return errors.New("setGroupField: Group has no hook")
	}

	hook := config.NewDocument()
	if err := json.Unmarshal(hooks[0], hook); err != nil {
		return fmt.Errorf("setGroupField: %w", err)
	}
	hook.Unset([]string{name})
	if raw != nil {
		if err := hook.Set([]string{name}, raw); err != nil {
			return fmt.Errorf("setGroupField: %w", err)
		}
	}
	encoded, err := json.Marshal(hook)
	if err != nil {
		return fmt.Errorf("setGroupField: %w", err)
	}
	hooks[0] = encoded
	return group.SetList([]string{hooksKey}, hooks)
}

// RenderDefaults renders the sink's rows of one component with every default — the heartbeat crate's fixture.
func RenderDefaults(id components.ID, sink components.SinkKind) ([]byte, error) {
	component, ok := components.Lookup(id)
	if !ok {
		return nil, fmt.Errorf("RenderDefaults: Unknown component %q", id)
	}

	document := renderedDocument{}
	for _, row := range component.Settings {
		if row.Sink.Kind != sink {
			continue
		}

		value, err := components.ParseText(&row, row.Default)
		if err != nil {
			return nil, fmt.Errorf("RenderDefaults: %w", err)
		}
		document.set(row.Sink.Target, value)
	}
	return document.marshal()
}

// RenderDocument renders every row of the sink across the record's selected components into one JSON document, keys sorted, two-space indented.
func RenderDocument(env Env, record Record, sink components.SinkKind) ([]byte, error) {
	selected := ExpandedComponents(record, env.GOOS)
	document := renderedDocument{}
	for _, component := range components.Catalog() {
		if !slices.Contains(selected, component.ID) {
			continue
		}

		rows, err := Schema(env, component.ID)
		if err != nil {
			return nil, fmt.Errorf("RenderDocument: %w", err)
		}
		for _, row := range rows {
			if row.Sink.Kind != sink {
				continue
			}

			value, err := Resolve(record, component.ID, row.Key)
			if err != nil {
				return nil, fmt.Errorf("RenderDocument: %w", err)
			}
			document.set(row.Sink.Target, value)
		}
	}

	isHeartbeatDocument := sink == components.SinkHeartbeatConfig && slices.Contains(selected, components.Heartbeat)
	if isHeartbeatDocument {
		peekAddr, err := heartbeatPeekAddr(record)
		if err != nil {
			return nil, fmt.Errorf("RenderDocument: %w", err)
		}
		document.set(heartbeatPeekAddrKey, peekAddr)
	}
	return document.marshal()
}

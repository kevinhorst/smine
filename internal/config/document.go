package config

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Document is a JSON object whose key order is preserved across load and
// save. Values stay raw so keys the code does not model survive untouched.
type Document struct {
	keys   []string
	values map[string]json.RawMessage
}

func NewDocument() *Document {
	return &Document{values: make(map[string]json.RawMessage)}
}

func (d *Document) Get(path []string) (json.RawMessage, bool) {
	if len(path) == 0 {
		return nil, false
	}
	raw, ok := d.values[path[0]]
	if !ok {
		return nil, false
	}
	if len(path) == 1 {
		return raw, true
	}
	child := NewDocument()
	if err := json.Unmarshal(raw, child); err != nil {
		return nil, false
	}
	return child.Get(path[1:])
}

func (d *Document) Keys() []string {
	keys := make([]string, len(d.keys))
	copy(keys, d.keys)
	return keys
}

func (d *Document) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, key := range d.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		name, err := json.Marshal(key)
		if err != nil {
			return nil, fmt.Errorf("Document.MarshalJSON: Failed to encode key %s: %w", key, err)
		}
		buf.Write(name)
		buf.WriteByte(':')
		buf.Write(d.values[key])
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func (d *Document) Set(path []string, value json.RawMessage) error {
	if len(path) == 0 {
		return fmt.Errorf("Document.Set: Empty path")
	}
	key := path[0]
	if len(path) == 1 {
		if _, ok := d.values[key]; !ok {
			d.keys = append(d.keys, key)
		}
		d.values[key] = value
		return nil
	}
	child := NewDocument()
	if raw, ok := d.values[key]; ok {
		if err := json.Unmarshal(raw, child); err != nil {
			return fmt.Errorf("Document.Set: Key %s is not an object: %w", key, err)
		}
	}
	if err := child.Set(path[1:], value); err != nil {
		return err
	}
	encoded, err := json.Marshal(child)
	if err != nil {
		return fmt.Errorf("Document.Set: Failed to encode child %s: %w", key, err)
	}
	return d.Set([]string{key}, encoded)
}

func (d *Document) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("Document.UnmarshalJSON: Failed to read opening token: %w", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return fmt.Errorf("Document.UnmarshalJSON: Not a JSON object")
	}
	d.keys = nil
	d.values = make(map[string]json.RawMessage)
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("Document.UnmarshalJSON: Failed to read key: %w", err)
		}
		key := tok.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return fmt.Errorf("Document.UnmarshalJSON: Failed to read value of %s: %w", key, err)
		}
		if _, ok := d.values[key]; !ok {
			d.keys = append(d.keys, key)
		}
		d.values[key] = raw
	}
	if _, err := dec.Token(); err != nil {
		return fmt.Errorf("Document.UnmarshalJSON: Failed to read closing token: %w", err)
	}
	return nil
}

func (d *Document) Unset(path []string) bool {
	if len(path) == 0 {
		return false
	}
	key := path[0]
	raw, ok := d.values[key]
	if !ok {
		return false
	}
	if len(path) == 1 {
		delete(d.values, key)
		for i, k := range d.keys {
			if k == key {
				d.keys = append(d.keys[:i], d.keys[i+1:]...)
				break
			}
		}
		return true
	}
	child := NewDocument()
	if err := json.Unmarshal(raw, child); err != nil {
		return false
	}
	if !child.Unset(path[1:]) {
		return false
	}
	encoded, err := json.Marshal(child)
	if err != nil {
		return false
	}
	return d.Set([]string{key}, encoded) == nil
}

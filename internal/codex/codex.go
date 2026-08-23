package codex

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/creachadair/tomledit"
	"github.com/creachadair/tomledit/parser"
	"github.com/creachadair/tomledit/transform"
	"github.com/kevinhorst/smine/internal/fsx"
)

var ErrNotFound = errors.New("codex: entry not found")

// Config wraps ~/.codex/config.toml as a lossless TOML document.
type Config struct {
	doc *tomledit.Document
}

// Entry is one toggleable unit for the UI: a global key-value or a table.
// Subtables of a listed table are grouped under it and not toggleable alone.
type Entry struct {
	Key       string
	Subtables []string
	Value     string // formatted value for globals, empty for tables
}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			doc, _ := tomledit.Parse(strings.NewReader(""))
			return &Config{doc: doc}, nil
		}
		return nil, fmt.Errorf("Load: Failed to open %s: %w", path, err)
	}
	defer f.Close()

	doc, err := tomledit.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("Load: Failed to parse %s: %w", path, err)
	}
	return &Config{doc: doc}, nil
}

// Entries lists global key-values in document order, then sections in
// document order; a section prefixed by an earlier section is grouped under
// it as a subtable.
func (c *Config) Entries() []Entry {
	var entries []Entry
	for _, item := range c.doc.Global.Items {
		kv, ok := item.(*parser.KeyValue)
		if !ok {
			continue
		}
		entries = append(entries, Entry{Key: kv.Name.String(), Value: kv.Value.String()})
	}
	for _, section := range c.doc.Sections {
		name := section.TableName()
		grouped := false
		for i := range entries {
			if entries[i].Value != "" {
				continue
			}
			parent, err := parser.ParseKey(entries[i].Key)
			if err == nil && parent.IsPrefixOf(name) && !parent.Equals(name) {
				entries[i].Subtables = append(entries[i].Subtables, name.String())
				grouped = true
				break
			}
		}
		if !grouped {
			entries = append(entries, Entry{Key: name.String()})
		}
	}
	return entries
}

// takeUnit removes and returns the toggle unit for key: a global key-value,
// or a table together with every section it prefixes (subtables).
func (c *Config) takeUnit(key parser.Key) (*parser.KeyValue, []*tomledit.Section) {
	for i, item := range c.doc.Global.Items {
		kv, ok := item.(*parser.KeyValue)
		if !ok || !kv.Name.Equals(key) {
			continue
		}
		c.doc.Global.Items = append(c.doc.Global.Items[:i], c.doc.Global.Items[i+1:]...)
		return kv, nil
	}

	var taken []*tomledit.Section
	var kept []*tomledit.Section
	for _, section := range c.doc.Sections {
		name := section.TableName()
		if name.Equals(key) || key.IsPrefixOf(name) {
			taken = append(taken, section)
		} else {
			kept = append(kept, section)
		}
	}
	if len(taken) == 0 {
		return nil, nil
	}
	c.doc.Sections = kept
	return nil, taken
}

func (c *Config) Get(key string) (string, bool) {
	k, err := parser.ParseKey(key)
	if err != nil {
		return "", false
	}
	entry := c.doc.First(k...)
	if entry == nil || !entry.IsMapping() {
		return "", false
	}
	return entry.KeyValue.Value.String(), true
}

func (c *Config) Set(key, value string) error {
	k, err := parser.ParseKey(key)
	if err != nil {
		return fmt.Errorf("Config.Set: Invalid key %s: %w", key, err)
	}
	val, err := parser.ParseValue(value)
	if err != nil {
		return fmt.Errorf("Config.Set: Invalid TOML value for %s: %w", key, err)
	}

	if entry := c.doc.First(k...); entry != nil && entry.IsMapping() {
		entry.KeyValue.Value = val
		return nil
	}

	// New key: global for a single-segment key, otherwise into its table.
	kv := &parser.KeyValue{Name: k[len(k)-1:], Value: val}
	if len(k) == 1 {
		kv.Name = k
		transform.InsertMapping(c.doc.Global, kv, true)
		return nil
	}
	tableEntry := transform.FindTable(c.doc, k[:len(k)-1]...)
	if tableEntry == nil {
		section := &tomledit.Section{
			Heading: &parser.Heading{Name: k[:len(k)-1]},
		}
		c.doc.Sections = append(c.doc.Sections, section)
		transform.InsertMapping(section, kv, true)
		return nil
	}
	transform.InsertMapping(tableEntry.Section, kv, true)
	return nil
}

func (c *Config) Unset(key string) bool {
	k, err := parser.ParseKey(key)
	if err != nil {
		return false
	}
	entry := c.doc.First(k...)
	if entry == nil {
		return false
	}
	return entry.Remove()
}

func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex", "config.toml")
}

func DisabledPath(configPath string) string {
	dir := filepath.Dir(configPath)
	return filepath.Join(dir, "config.disabled.toml")
}

func Save(path string, c *Config) error {
	var buf bytes.Buffer
	if err := tomledit.Format(&buf, c.doc); err != nil {
		return fmt.Errorf("Save: Failed to format %s: %w", path, err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("Save: Failed to write %s: %w", tmp, err)
	}
	if err := fsx.ReplaceFile(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("Save: Failed to rename %s to %s: %w", tmp, path, err)
	}
	return nil
}

// Toggle shuttles the unit at key between config.toml and its .disabled
// sibling. The destination file is saved first (D13).
func Toggle(configPath, key string) error {
	k, err := parser.ParseKey(key)
	if err != nil {
		return fmt.Errorf("Toggle: Invalid key %s: %w", key, err)
	}
	main, err := Load(configPath)
	if err != nil {
		return err
	}
	disabled, err := Load(DisabledPath(configPath))
	if err != nil {
		return err
	}

	src, dst := main, disabled
	srcPath, dstPath := configPath, DisabledPath(configPath)
	kv, sections := src.takeUnit(k)
	if kv == nil && sections == nil {
		src, dst = disabled, main
		srcPath, dstPath = DisabledPath(configPath), configPath
		kv, sections = src.takeUnit(k)
	}
	if kv == nil && sections == nil {
		return fmt.Errorf("Toggle: Entry not found %s: %w", key, ErrNotFound)
	}

	// Re-add the unit in the destination document
	if kv != nil {
		transform.InsertMapping(dst.doc.Global, kv, true)
	} else {
		dst.doc.Sections = append(dst.doc.Sections, sections...)
	}

	if err := Save(dstPath, dst); err != nil {
		return err
	}
	return Save(srcPath, src)
}

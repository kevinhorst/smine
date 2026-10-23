package tokencount

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"maps"
	"os"
	"path/filepath"
	"sync"

	"github.com/kevinhorst/smine/internal/contextrecord"
)

const (
	cacheDirMode  = 0o700
	cacheFile     = "token-cache.json"
	cacheFileMode = 0o600
	cacheTempGlob = "token-cache-*.tmp"
)

// cache is the content-addressed token store: one JSON object, key → tokens.
type cache struct {
	mu sync.Mutex

	entries map[string]int
	isDirty bool
	path    string
}

// loadCache treats a missing or unreadable file as an empty cache.
func loadCache(path string) *cache {
	loaded := &cache{entries: make(map[string]int), path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return loaded
	}
	if err != nil {
		log.Printf("loadCache: %s unreadable, starting empty: %v", path, err)
		return loaded
	}
	if err := json.Unmarshal(data, &loaded.entries); err != nil {
		log.Printf("loadCache: %s invalid, starting empty: %v", path, err)
		loaded.entries = make(map[string]int)
	}
	return loaded
}

func (c *cache) get(key string) (int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	tokens, ok := c.entries[key]
	return tokens, ok
}

func (c *cache) put(key string, tokens int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = tokens
	c.isDirty = true
}

// save merges on-disk entries first: another process may have counted.
func (c *cache) save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.isDirty {
		return nil
	}

	merged := make(map[string]int)
	if data, err := os.ReadFile(c.path); err == nil {
		_ = json.Unmarshal(data, &merged)
	}
	maps.Copy(merged, c.entries)
	c.entries = merged

	data, err := json.Marshal(merged)
	if err != nil {
		return fmt.Errorf("cache.save: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(c.path), cacheDirMode); err != nil {
		return fmt.Errorf("cache.save: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(c.path), cacheTempGlob)
	if err != nil {
		return fmt.Errorf("cache.save: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		os.Remove(temp.Name())
		return fmt.Errorf("cache.save: %w", err)
	}
	if err := temp.Close(); err != nil {
		os.Remove(temp.Name())
		return fmt.Errorf("cache.save: %w", err)
	}
	if err := os.Rename(temp.Name(), c.path); err != nil {
		os.Remove(temp.Name())
		return fmt.Errorf("cache.save: %w", err)
	}
	c.isDirty = false
	return nil
}

// cacheKey relies on a text's count under one tokenizer never changing.
func cacheKey(counter contextrecord.Counter, model, text string) string {
	sum := sha256.Sum256([]byte(string(counter) + "\x00" + model + "\x00" + text))
	return hex.EncodeToString(sum[:])
}

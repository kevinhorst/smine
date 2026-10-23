package acdsl

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Cache scopes a registry entry may declare; an absent scope means CacheTree.
const (
	CacheFiles = "files"
	CacheNone  = "none"
	CacheTree  = "tree"
)

const (
	cacheDir      = "bin/acdsl-cache"
	cacheDirMode  = 0o755
	cacheFileMode = 0o644
	cacheTempGlob = "verdict-*.tmp"
	missingHash   = "missing"
)

// A hit replays exactly what the verifier printed: exit 0 with no diagnostics, or exit 1 with them.
type cachedVerdict struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
	Exit        int          `json:"exit"`
}

// contentHashes is computed once per Check before the pool starts; workers only read it.
type contentHashes struct {
	byPath map[string]string
	root   string
	tree   string
}

func hashUniverse(root string, universe []string) *contentHashes {
	hashes := &contentHashes{byPath: make(map[string]string, len(universe)), root: root}
	tree := sha256.New()
	for _, path := range universe {
		sum := hashFile(filepath.Join(root, path))
		hashes.byPath[path] = sum
		fmt.Fprintf(tree, "%s %s\n", path, sum)
	}
	hashes.tree = hex.EncodeToString(tree.Sum(nil))
	return hashes
}

// hashOf serves universe paths from the map and anything else (needs, param files) from disk.
func (h *contentHashes) hashOf(path string) string {
	if sum, ok := h.byPath[path]; ok {
		return sum
	}
	if filepath.IsAbs(path) {
		return hashFile(path)
	}
	return hashFile(filepath.Join(h.root, path))
}

// underPrefixes lists the universe paths below any of the declared directory prefixes, sorted.
func (h *contentHashes) underPrefixes(prefixes []string) []string {
	var matched []string
	for path := range h.byPath {
		for _, prefix := range prefixes {
			if strings.HasPrefix(path, prefix) {
				matched = append(matched, path)
				break
			}
		}
	}
	sort.Strings(matched)
	return matched
}

// An unreadable input hashes as a constant, so its key differs from every readable state.
func hashFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return missingHash
	}

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// cacheKey returns "" when the run must not be cached: scope none, or a verifier that is not a file on disk.
func cacheKey(entry RegistryEntry, rule Rule, files []string, hashes *contentHashes) string {
	if entry.Cache == CacheNone || hashes == nil {
		return ""
	}
	binary := hashFile(entry.Argv[0])
	if binary == missingHash {
		return ""
	}

	key := sha256.New()
	fmt.Fprintf(key, "binary %s\n", binary)
	fmt.Fprintf(key, "argv %s\n", strings.Join(entry.Argv[1:], " "))
	fmt.Fprintf(key, "rule %s\n", rule.Id)
	fmt.Fprintf(key, "why %s\n", rule.Why)
	for _, name := range sortedKeys(rule.Params) {
		fmt.Fprintf(key, "param %s=%s\n", name, rule.Params[name])
	}
	if entry.Cache == CacheFiles {
		for _, path := range cacheInputs(entry, rule, files, hashes) {
			fmt.Fprintf(key, "file %s %s\n", path, hashes.hashOf(path))
		}
	} else {
		fmt.Fprintf(key, "tree %s\n", hashes.tree)
	}
	return hex.EncodeToString(key.Sum(nil))
}

// cacheInputs is the files scope: matched files, needs, param values naming a file, declared read prefixes.
func cacheInputs(entry RegistryEntry, rule Rule, files []string, hashes *contentHashes) []string {
	seen := map[string]bool{}
	var inputs []string
	add := func(path string) {
		if !seen[path] {
			seen[path] = true
			inputs = append(inputs, path)
		}
	}
	for _, path := range files {
		add(path)
	}
	for _, path := range rule.Needs {
		add(path)
	}
	for _, name := range sortedKeys(rule.Params) {
		value := rule.Params[name]
		if info, err := os.Stat(filepath.Join(hashes.root, value)); err == nil && !info.IsDir() {
			add(value)
		}
	}
	for _, path := range hashes.underPrefixes(entry.Reads) {
		add(path)
	}
	sort.Strings(inputs)
	return inputs
}

func cachePath(root, key string) string {
	return filepath.Join(root, cacheDir, key+".json")
}

func loadVerdict(root, key string) (*cachedVerdict, bool) {
	data, err := os.ReadFile(cachePath(root, key))
	if err != nil {
		return nil, false
	}

	var verdict cachedVerdict
	if err := json.Unmarshal(data, &verdict); err != nil {
		return nil, false
	}

	return &verdict, true
}

// storeVerdict writes through a temp file and a rename; a failed store is a miss next time, never an error.
func storeVerdict(root, key string, verdict *cachedVerdict) {
	data, err := json.Marshal(verdict)
	if err != nil {
		return
	}

	dir := filepath.Join(root, cacheDir)
	if err := os.MkdirAll(dir, cacheDirMode); err != nil {
		return
	}

	temp, err := os.CreateTemp(dir, cacheTempGlob)
	if err != nil {
		return
	}

	if _, err := temp.Write(data); err != nil {
		temp.Close()
		os.Remove(temp.Name())
		return
	}

	if err := temp.Close(); err != nil {
		os.Remove(temp.Name())
		return
	}

	if err := os.Chmod(temp.Name(), cacheFileMode); err != nil {
		os.Remove(temp.Name())
		return
	}

	if err := os.Rename(temp.Name(), cachePath(root, key)); err != nil {
		os.Remove(temp.Name())
	}
}

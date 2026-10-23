package acdsl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeCacheFile writes one file below root, creating its parent dirs.
func writeCacheFile(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, path)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}

// countRuns counts the lines the counter-file stand-in verifier appended; no file means no run.
func countRuns(t *testing.T, counter string) int {
	t.Helper()
	data, err := os.ReadFile(counter)
	if os.IsNotExist(err) {
		return 0
	}
	require.NoError(t, err)
	return strings.Count(string(data), "\n")
}

// provideCacheRoot writes a fake verifier binary, the universe files and two out-of-universe inputs under a temp root.
func provideCacheRoot(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	writeCacheFile(t, root, "bin/verifier", "binary")
	writeCacheFile(t, root, "a.go", "package a\n")
	writeCacheFile(t, root, "b.go", "package b\n")
	writeCacheFile(t, root, "notes.md", "notes\n")
	writeCacheFile(t, root, "skills/x/skill.cue", "skill\n")
	writeCacheFile(t, root, "needs.json", "{}\n")
	writeCacheFile(t, root, "config.json", "{}\n")
	return root, filepath.Join(root, "bin", "verifier")
}

// provideCachedCheck writes a root holding a.go and notes.md and a stand-in verifier that counts its runs, then ends in tail.
func provideCachedCheck(t *testing.T, cache, tail string) (string, string, map[string]RegistryEntry) {
	t.Helper()
	root := t.TempDir()
	writeCacheFile(t, root, "a.go", "package a\n")
	writeCacheFile(t, root, "notes.md", "notes\n")
	counter := filepath.Join(t.TempDir(), "runs.txt")
	script := writeScript(t, fmt.Sprintf("#!/bin/sh\necho x >> '%s'\n%s", counter, tail))
	registry := map[string]RegistryEntry{"fake": {Argv: []string{"/bin/sh", script}, Cache: cache, TimeoutSec: 10}}
	return root, counter, registry
}

func TestCacheKey(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedEqual bool

		entry     RegistryEntry
		files     []string
		hashes    *contentHashes
		reference string
		rule      Rule
	}

	universe := []string{"a.go", "b.go", "notes.md", "skills/x/skill.cue"}
	matched := []string{"a.go"}
	rule := Rule{Id: "X-001", Verifier: "fake", Why: "w"}
	tests := make([]*testCase, 0)

	// none-scope-empty-key
	root, binary := provideCacheRoot(t)
	noneEntry := RegistryEntry{Argv: []string{binary}, Cache: CacheNone, TimeoutSec: 10}
	tests = append(tests, &testCase{
		_id:            "none-scope-empty-key",
		_expectedEqual: true,

		entry:     noneEntry,
		files:     matched,
		hashes:    hashUniverse(root, universe),
		reference: "",
		rule:      rule,
	})

	// binary-missing-empty-key
	root, _ = provideCacheRoot(t)
	absentEntry := RegistryEntry{Argv: []string{filepath.Join(root, "bin", "absent")}, Cache: CacheFiles, TimeoutSec: 10}
	tests = append(tests, &testCase{
		_id:            "binary-missing-empty-key",
		_expectedEqual: true,

		entry:     absentEntry,
		files:     matched,
		hashes:    hashUniverse(root, universe),
		reference: "",
		rule:      rule,
	})

	// files-scope-matched-files-only
	root, binary = provideCacheRoot(t)
	filesEntry := RegistryEntry{Argv: []string{binary}, Cache: CacheFiles, TimeoutSec: 10}
	reference := cacheKey(filesEntry, rule, matched, hashUniverse(root, universe))
	writeCacheFile(t, root, "b.go", "package b // changed\n")
	tests = append(tests, &testCase{
		_id:            "files-scope-matched-files-only",
		_expectedEqual: true,

		entry:     filesEntry,
		files:     matched,
		hashes:    hashUniverse(root, universe),
		reference: reference,
		rule:      rule,
	})

	// files-scope-needs-and-param-file
	root, binary = provideCacheRoot(t)
	filesEntry = RegistryEntry{Argv: []string{binary}, Cache: CacheFiles, TimeoutSec: 10}
	inputsRule := Rule{Id: "X-001", Verifier: "fake", Why: "w", Needs: []string{"needs.json"}, Params: map[string]string{"config": "config.json"}}
	hashes := hashUniverse(root, universe)
	reference = cacheKey(filesEntry, inputsRule, matched, hashes)
	writeCacheFile(t, root, "needs.json", "{\"changed\": true}\n")
	writeCacheFile(t, root, "config.json", "{\"changed\": true}\n")
	tests = append(tests, &testCase{
		_id:            "files-scope-needs-and-param-file",
		_expectedEqual: false,

		entry:     filesEntry,
		files:     matched,
		hashes:    hashes,
		reference: reference,
		rule:      inputsRule,
	})

	// files-scope-reads-prefix
	root, binary = provideCacheRoot(t)
	readsEntry := RegistryEntry{Argv: []string{binary}, Cache: CacheFiles, Reads: []string{"skills/"}, TimeoutSec: 10}
	reference = cacheKey(readsEntry, rule, matched, hashUniverse(root, universe))
	writeCacheFile(t, root, "skills/x/skill.cue", "skill changed\n")
	tests = append(tests, &testCase{
		_id:            "files-scope-reads-prefix",
		_expectedEqual: false,

		entry:     readsEntry,
		files:     matched,
		hashes:    hashUniverse(root, universe),
		reference: reference,
		rule:      rule,
	})

	// tree-scope-whole-universe
	root, binary = provideCacheRoot(t)
	treeEntry := RegistryEntry{Argv: []string{binary}, Cache: CacheTree, TimeoutSec: 10}
	reference = cacheKey(treeEntry, rule, matched, hashUniverse(root, universe))
	writeCacheFile(t, root, "notes.md", "notes changed\n")
	tests = append(tests, &testCase{
		_id:            "tree-scope-whole-universe",
		_expectedEqual: false,

		entry:     treeEntry,
		files:     matched,
		hashes:    hashUniverse(root, universe),
		reference: reference,
		rule:      rule,
	})

	// params-order-independent
	root, binary = provideCacheRoot(t)
	filesEntry = RegistryEntry{Argv: []string{binary}, Cache: CacheFiles, TimeoutSec: 10}
	hashes = hashUniverse(root, universe)
	forwardRule := Rule{Id: "X-001", Verifier: "fake", Why: "w", Params: map[string]string{"a": "1", "b": "2"}}
	backwardRule := Rule{Id: "X-001", Verifier: "fake", Why: "w", Params: map[string]string{"b": "2", "a": "1"}}
	tests = append(tests, &testCase{
		_id:            "params-order-independent",
		_expectedEqual: true,

		entry:     filesEntry,
		files:     matched,
		hashes:    hashes,
		reference: cacheKey(filesEntry, forwardRule, matched, hashes),
		rule:      backwardRule,
	})

	// unreadable-input-hashes-missing
	root, binary = provideCacheRoot(t)
	filesEntry = RegistryEntry{Argv: []string{binary}, Cache: CacheFiles, TimeoutSec: 10}
	reference = cacheKey(filesEntry, rule, matched, hashUniverse(root, universe))
	require.NoError(t, os.Remove(filepath.Join(root, "a.go")))
	tests = append(tests, &testCase{
		_id:            "unreadable-input-hashes-missing",
		_expectedEqual: false,

		entry:     filesEntry,
		files:     matched,
		hashes:    hashUniverse(root, universe),
		reference: reference,
		rule:      rule,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expectedEqual, cacheKey(test.entry, test.rule, test.files, test.hashes) == test.reference)
		})
	}
}

func TestCheckReplaysCachedVerdict(t *testing.T) {
	type testCase struct {
		_id                  string
		_expectedDiagnostics []Diagnostic
		_expectedRuns        int
		_shouldPass          bool

		counter  string
		registry map[string]RegistryEntry
		root     string
		rules    []Rule
		universe []string
	}

	ctx := context.Background()
	rules := []Rule{{Id: "X-001", Verifier: "fake", Anchor: `\.go$`, Why: "the why"}}
	universe := []string{"a.go", "notes.md"}
	tests := make([]*testCase, 0)

	// hit-skips-run-and-replays-pass
	root, counter, registry := provideCachedCheck(t, CacheFiles, "exit 0\n")
	_, err := Check(ctx, root, rules, registry, universe)
	require.NoError(t, err)
	tests = append(tests, &testCase{
		_id:                  "hit-skips-run-and-replays-pass",
		_expectedDiagnostics: nil,
		_expectedRuns:        1,
		_shouldPass:          true,

		counter:  counter,
		registry: registry,
		root:     root,
		rules:    rules,
		universe: universe,
	})

	// hit-replays-diagnostics
	root, counter, registry = provideCachedCheck(t, CacheFiles, "echo 'a.go:1: finding'\nexit 1\n")
	_, err = Check(ctx, root, rules, registry, universe)
	require.NoError(t, err)
	finding := Diagnostic{RuleId: "X-001", Verifier: "fake", Message: "a.go:1: finding", Why: "the why"}
	tests = append(tests, &testCase{
		_id:                  "hit-replays-diagnostics",
		_expectedDiagnostics: []Diagnostic{finding},
		_expectedRuns:        1,
		_shouldPass:          true,

		counter:  counter,
		registry: registry,
		root:     root,
		rules:    rules,
		universe: universe,
	})

	// files-scope-unrelated-change-still-hits
	root, counter, registry = provideCachedCheck(t, CacheFiles, "exit 0\n")
	_, err = Check(ctx, root, rules, registry, universe)
	require.NoError(t, err)
	writeCacheFile(t, root, "notes.md", "notes changed\n")
	tests = append(tests, &testCase{
		_id:                  "files-scope-unrelated-change-still-hits",
		_expectedDiagnostics: nil,
		_expectedRuns:        1,
		_shouldPass:          true,

		counter:  counter,
		registry: registry,
		root:     root,
		rules:    rules,
		universe: universe,
	})

	// files-scope-matched-change-misses
	root, counter, registry = provideCachedCheck(t, CacheFiles, "exit 0\n")
	_, err = Check(ctx, root, rules, registry, universe)
	require.NoError(t, err)
	writeCacheFile(t, root, "a.go", "package a // changed\n")
	tests = append(tests, &testCase{
		_id:                  "files-scope-matched-change-misses",
		_expectedDiagnostics: nil,
		_expectedRuns:        2,
		_shouldPass:          true,

		counter:  counter,
		registry: registry,
		root:     root,
		rules:    rules,
		universe: universe,
	})

	// tree-scope-any-change-misses
	root, counter, registry = provideCachedCheck(t, CacheTree, "exit 0\n")
	_, err = Check(ctx, root, rules, registry, universe)
	require.NoError(t, err)
	writeCacheFile(t, root, "notes.md", "notes changed\n")
	tests = append(tests, &testCase{
		_id:                  "tree-scope-any-change-misses",
		_expectedDiagnostics: nil,
		_expectedRuns:        2,
		_shouldPass:          true,

		counter:  counter,
		registry: registry,
		root:     root,
		rules:    rules,
		universe: universe,
	})

	// exit-two-never-stored
	root, counter, registry = provideCachedCheck(t, CacheFiles, "exit 2\n")
	_, err = Check(ctx, root, rules, registry, universe)
	require.Error(t, err)
	tests = append(tests, &testCase{
		_id:                  "exit-two-never-stored",
		_expectedDiagnostics: nil,
		_expectedRuns:        2,
		_shouldPass:          false,

		counter:  counter,
		registry: registry,
		root:     root,
		rules:    rules,
		universe: universe,
	})

	// corrupt-cache-file-is-miss
	root, counter, registry = provideCachedCheck(t, CacheFiles, "exit 0\n")
	_, err = Check(ctx, root, rules, registry, universe)
	require.NoError(t, err)
	stored, err := filepath.Glob(filepath.Join(root, cacheDir, "*.json"))
	require.NoError(t, err)
	require.Len(t, stored, 1)
	require.NoError(t, os.WriteFile(stored[0], []byte("{"), 0o644))
	tests = append(tests, &testCase{
		_id:                  "corrupt-cache-file-is-miss",
		_expectedDiagnostics: nil,
		_expectedRuns:        2,
		_shouldPass:          true,

		counter:  counter,
		registry: registry,
		root:     root,
		rules:    rules,
		universe: universe,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			diagnostics, err := Check(ctx, test.root, test.rules, test.registry, test.universe)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedDiagnostics, diagnostics)
			assert.Equal(t, test._expectedRuns, countRuns(t, test.counter))
		})
	}
}

func TestLoadRegistryCacheScope(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool

		path string
	}

	tests := make([]*testCase, 0)

	// pass-files-with-reads
	tests = append(tests, &testCase{
		_id:         "pass-files-with-reads",
		_shouldPass: true,

		path: writeRegistry(t, `{"x": {"argv": ["go", "run", "./x"], "cache": "files", "reads": ["skills/"], "timeout_s": 30, "description": "d"}}`),
	})

	// pass-absent-scope
	tests = append(tests, &testCase{
		_id:         "pass-absent-scope",
		_shouldPass: true,

		path: writeRegistry(t, `{"x": {"argv": ["go", "run", "./x"], "timeout_s": 30, "description": "d"}}`),
	})

	// fail-unknown-scope
	tests = append(tests, &testCase{
		_id:         "fail-unknown-scope",
		_shouldPass: false,

		path: writeRegistry(t, `{"x": {"argv": ["go", "run", "./x"], "cache": "always", "timeout_s": 30, "description": "d"}}`),
	})

	// fail-reads-without-files
	tests = append(tests, &testCase{
		_id:         "fail-reads-without-files",
		_shouldPass: false,

		path: writeRegistry(t, `{"x": {"argv": ["go", "run", "./x"], "cache": "tree", "reads": ["skills/"], "timeout_s": 30, "description": "d"}}`),
	})

	// fail-reads-not-prefix
	tests = append(tests, &testCase{
		_id:         "fail-reads-not-prefix",
		_shouldPass: false,

		path: writeRegistry(t, `{"x": {"argv": ["go", "run", "./x"], "cache": "files", "reads": ["skills"], "timeout_s": 30, "description": "d"}}`),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			_, err := LoadRegistry(test.path)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
		})
	}
}

package trash

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func provideFile(t *testing.T, path, content string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func provideStore(t *testing.T, retentionDays int) (*Store, string) {
	t.Helper()
	stateDir := filepath.Join(t.TempDir(), "smine")
	return New(stateDir, retentionDays), stateDir
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func TestStashRelRoundTrip(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedTarget string
		_shouldPass     bool

		path string
	}

	tests := make([]*testCase, 0)

	// pass-absolute-path
	absolute := filepath.Join(t.TempDir(), "a", "b.json")
	tests = append(tests, &testCase{
		_id:             "pass-absolute-path",
		_expectedTarget: absolute,
		_shouldPass:     true,

		path: absolute,
	})

	// pass-relative-path-made-absolute
	relativeAbsolute, err := filepath.Abs(filepath.Join("x", "y.txt"))
	require.NoError(t, err)
	tests = append(tests, &testCase{
		_id:             "pass-relative-path-made-absolute",
		_expectedTarget: relativeAbsolute,
		_shouldPass:     true,

		path: filepath.Join("x", "y.txt"),
	})

	// fail-root-path
	rootPath := string(filepath.Separator)
	if runtime.GOOS == goosWindows {
		rootPath = `C:\`
	}
	tests = append(tests, &testCase{
		_id:             "fail-root-path",
		_expectedTarget: "",
		_shouldPass:     false,

		path: rootPath,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			rel, err := stashRel(test.path)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedTarget, roundTrip(rel))
			assert.NotContains(t, rel, ":")
		})
	}
}

func roundTrip(rel string) string {
	if rel == "" {
		return ""
	}
	return targetOf(rel)
}

func TestStashRelWindowsVolume(t *testing.T) {
	if runtime.GOOS != goosWindows {
		t.Skip("volume names exist on Windows only")
	}

	rel, err := stashRel(`C:\Users\k\.claude\settings.json`)

	require.NoError(t, err)
	assert.Equal(t, "C/Users/k/.claude/settings.json", rel)
	assert.Equal(t, `C:\Users\k\.claude\settings.json`, targetOf(rel))
}

func TestStore_Move(t *testing.T) {
	store, _ := provideStore(t, 0)

	// file moved, journal line removed with stash
	file := provideFile(t, filepath.Join(t.TempDir(), "settings.json"), "old")
	stashPath, err := store.Move(file, &Entry{Action: "test move"})
	require.NoError(t, err)
	assert.NoFileExists(t, file)
	assert.Equal(t, "old", readFile(t, filepath.Join(store.root, stashPath)))
	entries, err := store.Tail(10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, KindRemoved, entries[0].Kind)
	assert.Equal(t, file, entries[0].Path)
	assert.Equal(t, filepath.ToSlash(stashPath), entries[0].Stash)

	// relative path journaled absolute
	relativeDir := t.TempDir()
	t.Chdir(relativeDir)
	provideFile(t, filepath.Join(relativeDir, "relative.json"), "rel")
	_, err = store.Move("relative.json", &Entry{Action: "test move"})
	require.NoError(t, err)
	entries, err = store.Tail(1)
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(entries[0].Path))

	// dir moved with its files
	dir := filepath.Join(t.TempDir(), "skill")
	provideFile(t, filepath.Join(dir, "SKILL.md"), "body")
	stashPath, err = store.Move(dir, &Entry{Action: "test move"})
	require.NoError(t, err)
	assert.NoDirExists(t, dir)
	assert.Equal(t, "body", readFile(t, filepath.Join(store.root, stashPath, "SKILL.md")))

	// missing path keeps nothing, no journal line
	stashPath, err = store.Move(filepath.Join(t.TempDir(), "absent"), &Entry{Action: "test move"})
	require.NoError(t, err)
	assert.Empty(t, stashPath)
	entries, err = store.Tail(10)
	require.NoError(t, err)
	assert.Len(t, entries, 3)

	// nil store keeps nothing
	var nilStore *Store
	kept := provideFile(t, filepath.Join(t.TempDir(), "kept"), "x")
	stashPath, err = nilStore.Move(kept, &Entry{Action: "test move"})
	require.NoError(t, err)
	assert.Empty(t, stashPath)
	assert.FileExists(t, kept)
}

func TestStore_MoveCrossDeviceFallback(t *testing.T) {
	store, _ := provideStore(t, 0)
	original := rename
	rename = func(string, string) error { return errors.New("cross-device link") }
	t.Cleanup(func() { rename = original })

	dir := filepath.Join(t.TempDir(), "routine")
	provideFile(t, filepath.Join(dir, "results.jsonl"), "line")

	stashPath, err := store.Move(dir, &Entry{Action: "test move"})

	require.NoError(t, err)
	assert.NoDirExists(t, dir)
	assert.Equal(t, "line", readFile(t, filepath.Join(store.root, stashPath, "results.jsonl")))
}

func TestStore_Keep(t *testing.T) {
	store, _ := provideStore(t, 0)

	// present file copied, original untouched, journal replaced
	file := provideFile(t, filepath.Join(t.TempDir(), "install.json"), "first")
	firstStash, err := store.Keep(file, &Entry{Action: "test keep"})
	require.NoError(t, err)
	assert.Equal(t, "first", readFile(t, file))
	assert.Equal(t, "first", readFile(t, filepath.Join(store.root, firstStash)))
	entries, err := store.Tail(10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, KindReplaced, entries[0].Kind)

	// two keeps of one path land in two stamps
	require.NoError(t, os.WriteFile(file, []byte("second"), 0o644))
	secondStash, err := store.Keep(file, &Entry{Action: "test keep"})
	require.NoError(t, err)
	assert.NotEqual(t, firstStash, secondStash)
	assert.Equal(t, "second", readFile(t, filepath.Join(store.root, secondStash)))

	// missing file keeps nothing
	stashPath, err := store.Keep(filepath.Join(t.TempDir(), "absent"), &Entry{Action: "test keep"})
	require.NoError(t, err)
	assert.Empty(t, stashPath)
}

func TestStore_Note(t *testing.T) {
	store, _ := provideStore(t, 0)

	require.NoError(t, store.Note(&Entry{Action: "environment forget", Kind: KindForgotten, Path: "install.json#SKILL_X"}))

	entries, err := store.Tail(10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, KindForgotten, entries[0].Kind)
	assert.False(t, entries[0].Time.IsZero())
}

func TestStore_Restore(t *testing.T) {
	store, _ := provideStore(t, 0)

	// file restored, stamp dir removed when empty
	file := provideFile(t, filepath.Join(t.TempDir(), "settings.json"), "old")
	stashPath, err := store.Move(file, &Entry{Action: "test"})
	require.NoError(t, err)
	stampName, rel := splitStash(stashPath)
	require.NoError(t, store.Restore(stampName, rel, false))
	assert.Equal(t, "old", readFile(t, file))
	assert.NoDirExists(t, filepath.Join(store.root, stampName))

	// dir restored
	dir := filepath.Join(t.TempDir(), "skill")
	provideFile(t, filepath.Join(dir, "SKILL.md"), "body")
	stashPath, err = store.Move(dir, &Entry{Action: "test"})
	require.NoError(t, err)
	stampName, rel = splitStash(stashPath)
	require.NoError(t, store.Restore(stampName, rel, false))
	assert.Equal(t, "body", readFile(t, filepath.Join(dir, "SKILL.md")))

	// present target refused with ErrTargetExists
	stashPath, err = store.Keep(file, &Entry{Action: "test"})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(file, []byte("new"), 0o644))
	stampName, rel = splitStash(stashPath)
	err = store.Restore(stampName, rel, false)
	assert.ErrorIs(t, err, ErrTargetExists)
	assert.Equal(t, "new", readFile(t, file))

	// present target replaced with the tick, old target stashed, journal restored plus removed
	require.NoError(t, store.Restore(stampName, rel, true))
	assert.Equal(t, "old", readFile(t, file))
	entries, err := store.Tail(2)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, KindRestored, entries[0].Kind)
	assert.Equal(t, KindRemoved, entries[1].Kind)
	assert.Equal(t, "new", readFile(t, filepath.Join(store.root, filepath.FromSlash(entries[1].Stash))))

	// unknown stamp or rel errors
	assert.Error(t, store.Restore("2000-01-01T00-00-00.000000000Z", "nope", false))
}

func TestStore_List(t *testing.T) {
	store, _ := provideStore(t, 0)

	// empty root → nil
	stashes, err := store.List()
	require.NoError(t, err)
	assert.Nil(t, stashes)

	// two stamps newest first with entries and sizes
	older := filepath.Join(store.root, stamp(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)))
	provideFile(t, filepath.Join(older, "Users", "k", "a.json"), "12")
	newer := filepath.Join(store.root, stamp(time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)))
	provideFile(t, filepath.Join(newer, "Users", "k", "b.json"), "1234")

	// a non-stamp dir skipped
	require.NoError(t, os.MkdirAll(filepath.Join(store.root, "not-a-stamp"), 0o755))

	stashes, err = store.List()
	require.NoError(t, err)
	require.Len(t, stashes, 2)
	assert.Equal(t, filepath.Base(newer), stashes[0].Stamp)
	assert.Equal(t, []StashEntry{{Rel: "Users/k/b.json", Size: 4}}, stashes[0].Entries)
	assert.Equal(t, []StashEntry{{Rel: "Users/k/a.json", Size: 2}}, stashes[1].Entries)
}

func TestStore_ListMovedDirIsOneEntry(t *testing.T) {
	store, _ := provideStore(t, 0)
	dir := filepath.Join(t.TempDir(), "skill")
	provideFile(t, filepath.Join(dir, "SKILL.md"), "body")
	provideFile(t, filepath.Join(dir, "payloads", "a.md"), "abc")
	stashPath, err := store.Move(dir, &Entry{Action: "test move"})
	require.NoError(t, err)

	stashes, err := store.List()

	require.NoError(t, err)
	require.Len(t, stashes, 1)
	_, rel := splitStash(stashPath)
	assert.Equal(t, []StashEntry{{IsDir: true, Rel: rel, Size: 7}}, stashes[0].Entries)

	// the dir entry restores whole
	require.NoError(t, store.Restore(stashes[0].Stamp, rel, false))
	assert.Equal(t, "abc", readFile(t, filepath.Join(dir, "payloads", "a.md")))
}

func TestStore_Sweep(t *testing.T) {
	// retention 0 keeps
	keeping, _ := provideStore(t, 0)
	old := filepath.Join(keeping.root, stamp(time.Now().UTC().Add(-48*time.Hour)))
	provideFile(t, filepath.Join(old, "f"), "x")
	removed, err := keeping.Sweep()
	require.NoError(t, err)
	assert.Zero(t, removed)
	assert.DirExists(t, old)

	// older than retention removed, newer kept
	sweeping, _ := provideStore(t, 1)
	old = filepath.Join(sweeping.root, stamp(time.Now().UTC().Add(-48*time.Hour)))
	provideFile(t, filepath.Join(old, "f"), "x")
	fresh := filepath.Join(sweeping.root, stamp(time.Now().UTC().Add(-time.Hour)))
	provideFile(t, filepath.Join(fresh, "f"), "x")
	removed, err = sweeping.Sweep()
	require.NoError(t, err)
	assert.Equal(t, 1, removed)
	assert.NoDirExists(t, old)
	assert.DirExists(t, fresh)

	// a retention set at runtime sweeps
	keeping.SetRetention(1)
	removed, err = keeping.Sweep()
	require.NoError(t, err)
	assert.Equal(t, 1, removed)

	// sweep runs after a Move (an old stamp disappears)
	provideFile(t, filepath.Join(old, "f"), "x")
	file := provideFile(t, filepath.Join(t.TempDir(), "g"), "y")
	_, err = sweeping.Move(file, &Entry{Action: "test"})
	require.NoError(t, err)
	assert.NoDirExists(t, old)
}

func TestStore_Tail(t *testing.T) {
	store, stateDir := provideStore(t, 0)

	// missing journal → nil
	entries, err := store.Tail(5)
	require.NoError(t, err)
	assert.Nil(t, entries)

	// limit honoured, newest first
	for _, action := range []string{"one", "two", "three"} {
		require.NoError(t, store.Note(&Entry{Action: action, Kind: KindForgotten}))
	}
	entries, err = store.Tail(2)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "three", entries[0].Action)
	assert.Equal(t, "two", entries[1].Action)

	// a torn last line skipped
	journal, err := os.OpenFile(filepath.Join(stateDir, JournalFile), os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	_, err = journal.WriteString(`{"action":"torn`)
	require.NoError(t, err)
	require.NoError(t, journal.Close())
	entries, err = store.Tail(10)
	require.NoError(t, err)
	assert.Len(t, entries, 3)
}

func TestOwnedRoundTrip(t *testing.T) {
	store, _ := provideStore(t, 0)
	file := provideFile(t, filepath.Join(t.TempDir(), "AGENTS.md"), "ours")

	// no record → false
	isUnchanged, err := store.IsUnchanged(file)
	require.NoError(t, err)
	assert.False(t, isUnchanged)

	// record then unchanged true
	require.NoError(t, store.RecordOwned(file, []byte("ours")))
	isUnchanged, err = store.IsUnchanged(file)
	require.NoError(t, err)
	assert.True(t, isUnchanged)

	// edited file → false
	require.NoError(t, os.WriteFile(file, []byte("theirs"), 0o644))
	isUnchanged, err = store.IsUnchanged(file)
	require.NoError(t, err)
	assert.False(t, isUnchanged)

	// forget then false
	require.NoError(t, os.WriteFile(file, []byte("ours"), 0o644))
	require.NoError(t, store.ForgetOwned(file))
	isUnchanged, err = store.IsUnchanged(file)
	require.NoError(t, err)
	assert.False(t, isUnchanged)

	// nil store → false
	var nilStore *Store
	isUnchanged, err = nilStore.IsUnchanged(file)
	require.NoError(t, err)
	assert.False(t, isUnchanged)
}

func TestHasMarkerAndStrip(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedHasLine1 bool
		_expectedHasLine2 bool

		data string
	}

	body := "#!/bin/sh\necho hi\n"
	marked := "#!/bin/sh\n" + MarkerLine([]byte(body)) + "\necho hi\n"

	tests := make([]*testCase, 0)

	// marker-on-line-1
	tests = append(tests, &testCase{
		_id:               "marker-on-line-1",
		_expectedHasLine1: true,
		_expectedHasLine2: false,

		data: MarkerPrefix + "abc\nrest\n",
	})

	// marker-on-line-2
	tests = append(tests, &testCase{
		_id:               "marker-on-line-2",
		_expectedHasLine1: false,
		_expectedHasLine2: true,

		data: marked,
	})

	// no-marker
	tests = append(tests, &testCase{
		_id:               "no-marker",
		_expectedHasLine1: false,
		_expectedHasLine2: false,

		data: body,
	})

	// short-file
	tests = append(tests, &testCase{
		_id:               "short-file",
		_expectedHasLine1: false,
		_expectedHasLine2: false,

		data: "x",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expectedHasLine1, HasMarker([]byte(test.data), 1))
			assert.Equal(t, test._expectedHasLine2, HasMarker([]byte(test.data), 2))
		})
	}

	stripped := StripMarker([]byte(marked), 2)
	assert.Equal(t, body, string(stripped))
	assert.Equal(t, MarkerLine(stripped), MarkerLine([]byte(body)))
	assert.Equal(t, body, string(StripMarker([]byte(body), 2)))
}

func splitStash(stashPath string) (string, string) {
	stampName, rel, _ := strings.Cut(filepath.ToSlash(stashPath), "/")
	return stampName, rel
}

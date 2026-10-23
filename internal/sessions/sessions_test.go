package sessions

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/shell"
	"github.com/kevinhorst/smine/internal/trash"
)

const (
	idFirst   = "7a9b1ff1-1bf8-4c51-9869-c6f8364d05de"
	idSecond  = "109bf289-fc70-47e9-be4a-fe291e77bcff"
	idOther   = "65a26e92-4c98-4879-82ce-35e644cd0ab5"
	weekOlder = "2026-W38"
	weekNewer = "2026-W39"
)

// validRecordJson is one rendered session record.
const validRecordJson = `{
  "id": "7a9b1ff1-1bf8-4c51-9869-c6f8364d05de",
  "agent": "claude",
  "title": "peek-mcp depth flag",
  "repo": "peek-mcp",
  "week": "2026-W39",
  "lastActive": "2026-09-24T10:00:00Z",
  "signal": "high",
  "findings": [
    {
      "dimension": "memory",
      "summary": "worktree discipline decays over long sessions",
      "quotes": ["re-anchor before acting"],
      "snippets": [
        {"kind": "violation", "lang": "go", "code": "func broken() {}", "source": "transcript"}
      ]
    }
  ],
  "frustration": [
    {"quote": "you edited the main checkout again", "trigger": "worktree path confusion"}
  ],
  "positive": [
    {"quote": "exactly what I wanted", "trigger": "clean first attempt"}
  ]
}`

func skippedRecordJson(id, lastActive, week string) string {
	return `{"id": "` + id + `", "agent": "claude", "title": "t", "repo": "r", "week": "` + week +
		`", "lastActive": "` + lastActive + `", "findings": [], "skipped": true, "skipReason": "trivial"}`
}

func writeRecord(t *testing.T, root, scope, week, name, content string) {
	t.Helper()
	dir := filepath.Join(root, scope, week)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
}

func TestSessionRefs(t *testing.T) {
	t.Run("maps-ids-across-scopes", func(t *testing.T) {
		root := t.TempDir()
		writeRecord(t, root, "peek-mcp", weekNewer, idFirst+".json", validRecordJson)
		writeRecord(t, root, "go", weekOlder, idOther+".json", skippedRecordJson(idOther, "2026-09-15T10:00:00Z", weekOlder))

		store := NewStore(root, nil)
		require.NoError(t, store.Reload())

		refs := store.SessionRefs()
		assert.Equal(t, SessionRef{LastActive: "2026-09-24T10:00:00Z", Scope: "peek-mcp", Week: weekNewer}, refs[idFirst])
		assert.Equal(t, SessionRef{LastActive: "2026-09-15T10:00:00Z", Scope: "go", Week: weekOlder}, refs[idOther])
	})

	t.Run("empty-store", func(t *testing.T) {
		store := NewStore(filepath.Join(t.TempDir(), "does-not-exist"), nil)
		require.NoError(t, store.Reload())
		assert.Empty(t, store.SessionRefs())
	})
}

func TestStoreReload(t *testing.T) {
	t.Run("week-grouping-newest-first", func(t *testing.T) {
		root := t.TempDir()
		writeRecord(t, root, "peek-mcp", weekNewer, idFirst+".json", validRecordJson)
		writeRecord(t, root, "peek-mcp", weekOlder, idSecond+".json", skippedRecordJson(idSecond, "2026-09-15T10:00:00Z", weekOlder))

		store := NewStore(root, nil)
		require.NoError(t, store.Reload())

		info, ok := store.Scope("peek-mcp")
		require.True(t, ok)
		require.Len(t, info.Weeks, 2)
		assert.Equal(t, weekNewer, info.Weeks[0].Key)
		assert.Equal(t, weekOlder, info.Weeks[1].Key)

		week, ok := store.Week("peek-mcp", weekNewer)
		require.True(t, ok)
		require.Len(t, week.Sessions, 1)
		session := week.Sessions[0]
		assert.Equal(t, "peek-mcp depth flag", session.Title)
		require.Len(t, session.Findings[0].Snippets, 1)
		assert.Equal(t, "func broken() {}", session.Findings[0].Snippets[0].Code)
		assert.Equal(t, "clean first attempt", session.Positive[0].Trigger)

		_, ok = store.Week("peek-mcp", "2026-W01")
		assert.False(t, ok)
	})

	t.Run("sessions-oldest-first", func(t *testing.T) {
		root := t.TempDir()
		writeRecord(t, root, "go", weekNewer, "b.json", skippedRecordJson("b", "2026-09-25T10:00:00Z", weekNewer))
		writeRecord(t, root, "go", weekNewer, "a.json", skippedRecordJson("a", "2026-09-22T10:00:00Z", weekNewer))

		store := NewStore(root, nil)
		require.NoError(t, store.Reload())

		week, ok := store.Week("go", weekNewer)
		require.True(t, ok)
		assert.Equal(t, "a", week.Sessions[0].Id)
		assert.Equal(t, "b", week.Sessions[1].Id)
	})

	t.Run("non-json-files-ignored", func(t *testing.T) {
		root := t.TempDir()
		writeRecord(t, root, "go", weekNewer, idFirst+".md", "---\n---\n")
		writeRecord(t, root, "go", weekNewer, idSecond+".json", skippedRecordJson(idSecond, "2026-09-22T10:00:00Z", weekNewer))

		store := NewStore(root, nil)
		require.NoError(t, store.Reload())

		info, ok := store.Scope("go")
		require.True(t, ok)
		assert.Empty(t, info.LoadErrors)
		require.Len(t, info.Weeks, 1)
		require.Len(t, info.Weeks[0].Sessions, 1)
		assert.Equal(t, idSecond, info.Weeks[0].Sessions[0].Id)
	})

	t.Run("malformed-json-load-error", func(t *testing.T) {
		root := t.TempDir()
		writeRecord(t, root, "go", weekNewer, idFirst+".json", validRecordJson)
		writeRecord(t, root, "go", weekNewer, "broken.json", "{not json")

		store := NewStore(root, nil)
		require.NoError(t, store.Reload())

		info, ok := store.Scope("go")
		require.True(t, ok)
		require.Len(t, info.Weeks, 1)
		require.Len(t, info.LoadErrors, 1)
		assert.Contains(t, info.LoadErrors[0], "broken.json")
	})

	t.Run("archived-and-non-week-skipped", func(t *testing.T) {
		root := t.TempDir()
		writeRecord(t, root, "go", weekNewer, idFirst+".json", validRecordJson)
		writeRecord(t, root, "go", "json", idOther+".json", validRecordJson)
		writeRecord(t, root, filepath.Join("archived", "old"), weekNewer, idSecond+".json", validRecordJson)

		store := NewStore(root, nil)
		require.NoError(t, store.Reload())

		scopes := store.Scopes()
		require.Len(t, scopes, 1)
		assert.Equal(t, "go", scopes[0].Name)
		assert.Len(t, scopes[0].Weeks, 1)
	})

	t.Run("empty-dir", func(t *testing.T) {
		store := NewStore(filepath.Join(t.TempDir(), "does-not-exist"), nil)
		require.NoError(t, store.Reload())
		assert.Empty(t, store.Scopes())
	})

	t.Run("unchanged-files-skip-read", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "go", weekNewer, idFirst+".json")
		modTime := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
		writeRecord(t, root, "go", weekNewer, idFirst+".json", validRecordJson)
		require.NoError(t, os.Chtimes(path, modTime, modTime))

		store := NewStore(root, nil)
		require.NoError(t, store.Reload())

		sameSizeRecord := strings.Replace(validRecordJson, "peek-mcp depth flag", "peek-mcp depth flop", 1)
		require.NoError(t, os.WriteFile(path, []byte(sameSizeRecord), 0o644))
		require.NoError(t, os.Chtimes(path, modTime, modTime))
		require.NoError(t, store.Reload())

		week, ok := store.Week("go", weekNewer)
		require.True(t, ok)
		require.Len(t, week.Sessions, 1)
		assert.Equal(t, "peek-mcp depth flag", week.Sessions[0].Title)
	})

	t.Run("changed-record-reloads", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "go", weekNewer, idFirst+".json")
		modTime := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
		writeRecord(t, root, "go", weekNewer, idFirst+".json", validRecordJson)
		require.NoError(t, os.Chtimes(path, modTime, modTime))

		store := NewStore(root, nil)
		require.NoError(t, store.Reload())

		changedRecord := strings.Replace(validRecordJson, "peek-mcp depth flag", "peek-mcp depth flop", 1)
		require.NoError(t, os.WriteFile(path, []byte(changedRecord), 0o644))
		require.NoError(t, os.Chtimes(path, modTime.Add(time.Hour), modTime.Add(time.Hour)))
		require.NoError(t, store.Reload())

		week, ok := store.Week("go", weekNewer)
		require.True(t, ok)
		require.Len(t, week.Sessions, 1)
		assert.Equal(t, "peek-mcp depth flop", week.Sessions[0].Title)
	})

	t.Run("added-record-reloads", func(t *testing.T) {
		root := t.TempDir()
		writeRecord(t, root, "go", weekNewer, idFirst+".json", validRecordJson)

		store := NewStore(root, nil)
		require.NoError(t, store.Reload())

		writeRecord(t, root, "go", weekNewer, idSecond+".json", skippedRecordJson(idSecond, "2026-09-25T10:00:00Z", weekNewer))
		require.NoError(t, store.Reload())

		week, ok := store.Week("go", weekNewer)
		require.True(t, ok)
		require.Len(t, week.Sessions, 2)
		assert.Equal(t, idSecond, week.Sessions[1].Id)
	})
}

func TestResolveRef(t *testing.T) {
	type testCase struct {
		_id          string
		_expectedId  string
		_expectedOk  bool
		_expectedRef SessionRef

		idOrPrefix string
		refs       map[string]SessionRef
	}

	idSibling := "7a9b0000-0000-4000-8000-000000000000"
	refFirst := SessionRef{LastActive: "2026-09-24T10:00:00Z", Scope: "go", Week: weekNewer}
	refSecond := SessionRef{LastActive: "2026-09-15T10:00:00Z", Scope: "peek-mcp", Week: weekOlder}
	refSibling := SessionRef{LastActive: "2026-09-16T10:00:00Z", Scope: "go", Week: weekOlder}
	refs := map[string]SessionRef{idFirst: refFirst, idSecond: refSecond, idSibling: refSibling}
	twoRefs := map[string]SessionRef{idFirst: refFirst, idSecond: refSecond}

	tests := make([]*testCase, 0)

	// exact-id
	tests = append(tests, &testCase{
		_id:          "exact-id",
		_expectedId:  idFirst,
		_expectedOk:  true,
		_expectedRef: refFirst,

		idOrPrefix: idFirst,
		refs:       refs,
	})

	// unique-prefix
	tests = append(tests, &testCase{
		_id:          "unique-prefix",
		_expectedId:  idFirst,
		_expectedOk:  true,
		_expectedRef: refFirst,

		idOrPrefix: "7a9b1ff1",
		refs:       refs,
	})

	// ambiguous-prefix
	tests = append(tests, &testCase{
		_id:          "ambiguous-prefix",
		_expectedId:  "",
		_expectedOk:  false,
		_expectedRef: SessionRef{},

		idOrPrefix: "7a9b",
		refs:       refs,
	})

	// unknown
	tests = append(tests, &testCase{
		_id:          "unknown",
		_expectedId:  "",
		_expectedOk:  false,
		_expectedRef: SessionRef{},

		idOrPrefix: "deadbeef",
		refs:       refs,
	})

	// empty-prefix
	tests = append(tests, &testCase{
		_id:          "empty-prefix",
		_expectedId:  "",
		_expectedOk:  false,
		_expectedRef: SessionRef{},

		idOrPrefix: "",
		refs:       twoRefs,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			id, ref, ok := ResolveRef(test.idOrPrefix, test.refs)
			assert.Equal(t, test._expectedId, id)
			assert.Equal(t, test._expectedRef, ref)
			assert.Equal(t, test._expectedOk, ok)
		})
	}
}

func TestWeek_Label(t *testing.T) {
	week := &Week{Key: weekNewer}
	assert.Equal(t, "2026-09-21 – 2026-09-27", week.Label())

	week = &Week{Key: "garbage"}
	assert.Equal(t, "garbage", week.Label())
}

func TestValidateScopeName(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool

		name string
	}

	tests := make([]*testCase, 0)

	// fail-empty
	tests = append(tests, &testCase{
		_id:         "fail-empty",
		_shouldPass: false,

		name: "",
	})

	// fail-dot
	tests = append(tests, &testCase{
		_id:         "fail-dot",
		_shouldPass: false,

		name: ".",
	})

	// fail-dot-dot
	tests = append(tests, &testCase{
		_id:         "fail-dot-dot",
		_shouldPass: false,

		name: "..",
	})

	// fail-archive-container
	tests = append(tests, &testCase{
		_id:         "fail-archive-container",
		_shouldPass: false,

		name: "archived",
	})

	// fail-parent-traversal
	tests = append(tests, &testCase{
		_id:         "fail-parent-traversal",
		_shouldPass: false,

		name: "../evil",
	})

	// fail-nested-path
	tests = append(tests, &testCase{
		_id:         "fail-nested-path",
		_shouldPass: false,

		name: "a/b",
	})

	// fail-backslash
	tests = append(tests, &testCase{
		_id:         "fail-backslash",
		_shouldPass: false,

		name: `a\b`,
	})

	// pass-plain
	tests = append(tests, &testCase{
		_id:         "pass-plain",
		_shouldPass: true,

		name: "oldscope",
	})

	// pass-unknown
	tests = append(tests, &testCase{
		_id:         "pass-unknown",
		_shouldPass: true,

		name: "unknown",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := validateScopeName(test.name)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
		})
	}
}

func TestStoreDotScopeNames(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool

		name      string
		operation func(ctx context.Context, name string) error
		root      string
	}

	newGuardedRoot := func() (string, *Store) {
		root := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(root, "livescope"), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(root, archivedDirName, "oldscope"), 0o755))
		return root, NewStore(root, nil)
	}
	tests := make([]*testCase, 0)

	// fail-archive-dot-dot
	root, store := newGuardedRoot()
	tests = append(tests, &testCase{
		_id:         "fail-archive-dot-dot",
		_shouldPass: false,

		name:      "..",
		operation: store.ArchiveScope,
		root:      root,
	})

	// fail-archive-dot
	root, store = newGuardedRoot()
	tests = append(tests, &testCase{
		_id:         "fail-archive-dot",
		_shouldPass: false,

		name:      ".",
		operation: store.ArchiveScope,
		root:      root,
	})

	// fail-delete-dot-dot
	root, store = newGuardedRoot()
	tests = append(tests, &testCase{
		_id:         "fail-delete-dot-dot",
		_shouldPass: false,

		name:      "..",
		operation: store.DeleteScope,
		root:      root,
	})

	// fail-delete-dot
	root, store = newGuardedRoot()
	tests = append(tests, &testCase{
		_id:         "fail-delete-dot",
		_shouldPass: false,

		name:      ".",
		operation: store.DeleteScope,
		root:      root,
	})

	// fail-unarchive-dot-dot
	root, store = newGuardedRoot()
	tests = append(tests, &testCase{
		_id:         "fail-unarchive-dot-dot",
		_shouldPass: false,

		name:      "..",
		operation: store.UnarchiveScope,
		root:      root,
	})

	// fail-unarchive-dot
	root, store = newGuardedRoot()
	tests = append(tests, &testCase{
		_id:         "fail-unarchive-dot",
		_shouldPass: false,

		name:      ".",
		operation: store.UnarchiveScope,
		root:      root,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := test.operation(t.Context(), test.name)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.DirExists(t, filepath.Join(test.root, "livescope"))
			assert.DirExists(t, filepath.Join(test.root, archivedDirName, "oldscope"))
		})
	}
}

func TestStoreArchivedFolders(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []ArchivedFolder

		store *Store
	}

	tests := make([]*testCase, 0)

	// missing-archived-dir
	tests = append(tests, &testCase{
		_id:       "missing-archived-dir",
		_expected: nil,

		store: NewStore(t.TempDir(), nil),
	})

	// empty-archived-dir
	emptyRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(emptyRoot, archivedDirName), 0o755))
	tests = append(tests, &testCase{
		_id:       "empty-archived-dir",
		_expected: []ArchivedFolder{},

		store: NewStore(emptyRoot, nil),
	})

	// sorted-with-counts-stray-file-skipped
	populatedRoot := t.TempDir()
	writeRecord(t, populatedRoot, filepath.Join(archivedDirName, "zeta"), weekNewer, idFirst+".json", validRecordJson)
	writeRecord(t, populatedRoot, filepath.Join(archivedDirName, "alpha"), weekNewer, idSecond+".json", validRecordJson)
	require.NoError(t, os.WriteFile(filepath.Join(populatedRoot, archivedDirName, "stray.txt"), nil, 0o644))
	sorted := []ArchivedFolder{{Name: "alpha", Sessions: 1}, {Name: "zeta", Sessions: 1}}
	tests = append(tests, &testCase{
		_id:       "sorted-with-counts-stray-file-skipped",
		_expected: sorted,

		store: NewStore(populatedRoot, nil),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.store.ArchivedFolders())
		})
	}
}

func TestStoreArchiveScope(t *testing.T) {
	root := newSessionsCheckout(t, "oldscope")
	store := NewStore(root, trash.New(t.TempDir(), 0))

	require.NoError(t, store.ArchiveScope(t.Context(), "oldscope"))
	assert.DirExists(t, filepath.Join(root, "archived", "oldscope"))
	assert.NoDirExists(t, filepath.Join(root, "oldscope"))
	assert.Equal(t, "sessions: Archived the oldscope records", gitOutput(t, root, "log", "-1", "--format=%s"))
	assert.Empty(t, gitOutput(t, root, "status", "--porcelain"))
	assert.NotEmpty(t, gitOutput(t, root, "ls-files", "archived/oldscope"))
	assert.Empty(t, gitOutput(t, root, "ls-files", "oldscope"))

	// already archived
	require.NoError(t, os.MkdirAll(filepath.Join(root, "oldscope"), 0o755))
	assert.Error(t, store.ArchiveScope(t.Context(), "oldscope"))

	// invalid names
	assert.Error(t, store.ArchiveScope(t.Context(), "archived"))
	assert.Error(t, store.ArchiveScope(t.Context(), "../evil"))
	assert.Error(t, store.ArchiveScope(t.Context(), ""))

	folders := store.ArchivedFolders()
	require.Len(t, folders, 1)
	assert.Equal(t, "oldscope", folders[0].Name)
	assert.Equal(t, 1, folders[0].Sessions)
}

func TestStoreArchiveScope_NotACheckoutRefused(t *testing.T) {
	root := t.TempDir()
	writeRecord(t, root, "oldscope", weekNewer, idFirst+".json", validRecordJson)
	store := NewStore(root, trash.New(t.TempDir(), 0))

	err := store.ArchiveScope(t.Context(), "oldscope")

	assert.ErrorContains(t, err, "sessions dir is not a git checkout")
	assert.DirExists(t, filepath.Join(root, "oldscope"))
	assert.NoDirExists(t, filepath.Join(root, "archived"))
}

func TestStoreDeleteScope(t *testing.T) {
	root := newSessionsCheckout(t, "livescope")
	trashStore := trash.New(t.TempDir(), 0)
	store := NewStore(root, trashStore)

	// only archived folders are deletable
	assert.Error(t, store.DeleteScope(t.Context(), "livescope"))
	assert.DirExists(t, filepath.Join(root, "livescope"))
	assert.Error(t, store.DeleteScope(t.Context(), "archived"))
	assert.Error(t, store.DeleteScope(t.Context(), "../evil"))

	require.NoError(t, store.ArchiveScope(t.Context(), "livescope"))
	require.NoError(t, store.DeleteScope(t.Context(), "livescope"))
	assert.NoDirExists(t, filepath.Join(root, "archived", "livescope"))
	assert.Equal(t, "sessions: Deleted the archived livescope records", gitOutput(t, root, "log", "-1", "--format=%s"))
	assert.Empty(t, gitOutput(t, root, "status", "--porcelain"))
	assert.Empty(t, gitOutput(t, root, "ls-files", "archived"))

	stashes, err := trashStore.List()
	require.NoError(t, err)
	require.Len(t, stashes, 1)
	require.Len(t, stashes[0].Entries, 1)
	assert.True(t, stashes[0].Entries[0].IsDir)
	assert.True(t, strings.HasSuffix(stashes[0].Entries[0].Rel, "archived/livescope"))

	journal, err := trashStore.Tail(1)
	require.NoError(t, err)
	require.Len(t, journal, 1)
	assert.Equal(t, "sessions delete", journal[0].Action)
	assert.Equal(t, trash.KindRemoved, journal[0].Kind)
}

func TestStoreDeleteScope_UntrackedFolderCommitsNothing(t *testing.T) {
	root := newSessionsCheckout(t, "livescope")
	trashStore := trash.New(t.TempDir(), 0)
	store := NewStore(root, trashStore)
	writeRecord(t, root, filepath.Join(archivedDirName, "legacy"), weekNewer, idSecond+".json", validRecordJson)
	head := gitOutput(t, root, "rev-parse", "HEAD")

	require.NoError(t, store.DeleteScope(t.Context(), "legacy"))

	assert.NoDirExists(t, filepath.Join(root, archivedDirName, "legacy"))
	assert.Equal(t, head, gitOutput(t, root, "rev-parse", "HEAD"))
	assert.Empty(t, gitOutput(t, root, "status", "--porcelain"))
	stashes, err := trashStore.List()
	require.NoError(t, err)
	assert.Len(t, stashes, 1)
}

func TestStoreUnarchiveScope(t *testing.T) {
	root := newSessionsCheckout(t, "oldscope")
	store := NewStore(root, trash.New(t.TempDir(), 0))
	require.NoError(t, store.ArchiveScope(t.Context(), "oldscope"))

	require.NoError(t, store.UnarchiveScope(t.Context(), "oldscope"))
	assert.DirExists(t, filepath.Join(root, "oldscope"))
	assert.NoDirExists(t, filepath.Join(root, "archived", "oldscope"))
	assert.Equal(t, "sessions: Unarchived the oldscope records", gitOutput(t, root, "log", "-1", "--format=%s"))
	assert.Empty(t, gitOutput(t, root, "status", "--porcelain"))
	assert.NotEmpty(t, gitOutput(t, root, "ls-files", "oldscope"))

	// live folder of the same name blocks unarchive
	require.NoError(t, store.ArchiveScope(t.Context(), "oldscope"))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "oldscope"), 0o755))
	assert.Error(t, store.UnarchiveScope(t.Context(), "oldscope"))
	assert.DirExists(t, filepath.Join(root, "archived", "oldscope"))
}

// gitOutput runs one git command in dir and returns its trimmed output.
func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	output, err := shell.Run(t.Context(), dir, "git", args...)
	require.NoError(t, err, output)
	return strings.TrimSpace(output)
}

// newSessionsCheckout commits one record per scope under <repo>/sessions and returns that sessions dir.
func newSessionsCheckout(t *testing.T, scopes ...string) string {
	t.Helper()
	repo := t.TempDir()
	root := filepath.Join(repo, "sessions")
	for _, scope := range scopes {
		writeRecord(t, root, scope, weekNewer, idFirst+".json", validRecordJson)
	}

	gitOutput(t, repo, "init", "-q")
	gitOutput(t, repo, "config", "user.name", "t")
	gitOutput(t, repo, "config", "user.email", "t@t")
	gitOutput(t, repo, "config", "commit.gpgsign", "false")
	gitOutput(t, repo, "config", "core.hooksPath", t.TempDir())
	gitOutput(t, repo, "add", "-A")
	gitOutput(t, repo, "commit", "-q", "-m", "records")
	return root
}

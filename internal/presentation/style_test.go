package presentation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/trash"
)

func TestGuardStyleProfile(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedContent string
		_expectedHits    int
		_expectedStashed []string

		path     string
		stateDir string
		store    *trash.Store
	}

	cleanProfile := "- Prefer short sentences.\n- Use bullets for lists.\n"
	emailProfile := cleanProfile + "- Ask devuser@example.com before merging.\n"
	secretProfile := cleanProfile + "- Token ghp_" + strings.Repeat("a", 36) + "\n"
	writeProfile := func(content string) string {
		path := filepath.Join(t.TempDir(), "style-profile.md")
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
		return path
	}
	tests := make([]*testCase, 0)

	// clean-profile-untouched
	cleanStore, cleanStateDir := provideTrashStore(t)
	tests = append(tests, &testCase{
		_id:              "clean-profile-untouched",
		_expectedContent: cleanProfile,
		_expectedHits:    0,
		_expectedStashed: nil,

		path:     writeProfile(cleanProfile),
		stateDir: cleanStateDir,
		store:    cleanStore,
	})

	// email-hit-writes-stub
	emailStore, emailStateDir := provideTrashStore(t)
	tests = append(tests, &testCase{
		_id:              "email-hit-writes-stub",
		_expectedContent: styleWithheldStub,
		_expectedHits:    1,
		_expectedStashed: []string{emailProfile},

		path:     writeProfile(emailProfile),
		stateDir: emailStateDir,
		store:    emailStore,
	})

	// secret-hit-writes-stub
	secretStore, secretStateDir := provideTrashStore(t)
	tests = append(tests, &testCase{
		_id:              "secret-hit-writes-stub",
		_expectedContent: styleWithheldStub,
		_expectedHits:    1,
		_expectedStashed: []string{secretProfile},

		path:     writeProfile(secretProfile),
		stateDir: secretStateDir,
		store:    secretStore,
	})

	// missing-file-no-hits
	missingStore, missingStateDir := provideTrashStore(t)
	tests = append(tests, &testCase{
		_id:              "missing-file-no-hits",
		_expectedContent: "",
		_expectedHits:    0,
		_expectedStashed: nil,

		path:     filepath.Join(t.TempDir(), "absent.md"),
		stateDir: missingStateDir,
		store:    missingStore,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			hits, err := GuardStyleProfile(test.path, test.store)
			content, _ := os.ReadFile(test.path)

			require.NoError(t, err)
			assert.Len(t, hits, test._expectedHits)
			assert.Equal(t, test._expectedContent, string(content))
			assert.Equal(t, test._expectedStashed, stashedContents(t, test.stateDir))
		})
	}
}

// provideTrashStore is a store on a fresh state dir, returned beside it for stashedContents.
func provideTrashStore(t *testing.T) (*trash.Store, string) {
	stateDir := t.TempDir()
	return trash.New(stateDir, 0), stateDir
}

// stashedContents reads every stashed file under the state dir, newest stamp first.
func stashedContents(t *testing.T, stateDir string) []string {
	stashes, err := trash.New(stateDir, 0).List()
	require.NoError(t, err)

	var contents []string
	for _, stash := range stashes {
		stampDir := filepath.Join(stateDir, trash.TrashDir, stash.Stamp)
		for _, entry := range stash.Entries {
			data, err := os.ReadFile(filepath.Join(stampDir, filepath.FromSlash(entry.Rel)))
			require.NoError(t, err)
			contents = append(contents, string(data))
		}
	}
	return contents
}

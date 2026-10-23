package skilleval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/evals"
	"github.com/kevinhorst/smine/internal/shell"
)

func commitAll(t *testing.T, dir, message string) {
	t.Helper()
	gitOutput(t, dir, "add", "-A")
	gitOutput(t, dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", message)
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	output, err := shell.Run(context.Background(), dir, "git", args...)
	require.NoError(t, err, output)
	return strings.TrimSpace(output)
}

// listFiles returns every regular file under root, root-relative and sorted;
// empty when root is absent.
func listFiles(t *testing.T, root string) []string {
	t.Helper()
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			return nil
		}

		relative, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("listFiles: %w", err)
		}

		files = append(files, relative)
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return files
	}

	require.NoError(t, err)
	sort.Strings(files)
	return files
}

func localBranches(t *testing.T, repo string) []string {
	t.Helper()
	return strings.Fields(gitOutput(t, repo, "for-each-ref", "--format=%(refname:short)", "refs/heads"))
}

// newFixtureRepo returns the symlink-resolved root of a repo with one commit
// holding file.txt and a .gitignore that ignores bin/.
func newFixtureRepo(t *testing.T) string {
	t.Helper()
	root := newTempDir(t)
	gitOutput(t, root, "init", "-q", "-b", "main")
	writeTestFile(t, filepath.Join(root, "file.txt"), "one\n")
	writeTestFile(t, filepath.Join(root, ".gitignore"), "bin/\n")
	commitAll(t, root, "init")
	return root
}

// newTempDir returns a symlink-resolved temp dir, matching the absolute paths
// git reports.
func newTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	return dir
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func writeDefinitionFile(t *testing.T, skillDir string, definition *evals.Definition) {
	t.Helper()
	content, err := json.MarshalIndent(definition, "", "  ")
	require.NoError(t, err)
	writeTestFile(t, filepath.Join(skillDir, "eval", "definition.json"), string(content))
}

func writeRunDir(t *testing.T, runDir string, manifest *RunManifest) {
	t.Helper()
	require.NoError(t, os.MkdirAll(runDir, 0o755))
	require.NoError(t, writeManifest(runDir, manifest))
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

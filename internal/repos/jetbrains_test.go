package repos

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	golandRecentProjects = `<application>
  <component name="RecentProjectsManager">
    <option name="additionalInfo">
      <map>
        <entry key="$USER_HOME$/GolandProjects/project-alpha">
          <value>
            <RecentProjectMetaInfo frameTitle="project-alpha">
              <option name="activationTimestamp" value="1" />
            </RecentProjectMetaInfo>
          </value>
        </entry>
        <entry key="$USER_HOME$/GolandProjects/project-alpha/.claude/worktrees/foo-123">
          <value>
            <RecentProjectMetaInfo frameTitle="foo-123">
              <option name="activationTimestamp" value="2" />
            </RecentProjectMetaInfo>
          </value>
        </entry>
        <entry key="$USER_HOME$/GolandProjects/project-beta/.claude/worktrees/bar-456">
          <value>
            <RecentProjectMetaInfo frameTitle="bar-456">
              <option name="activationTimestamp" value="3" />
            </RecentProjectMetaInfo>
          </value>
        </entry>
        <entry key="$USER_HOME$/GolandProjects/other-project">
          <value>
            <RecentProjectMetaInfo frameTitle="other-project">
              <option name="activationTimestamp" value="4" />
            </RecentProjectMetaInfo>
          </value>
        </entry>
      </map>
    </option>
  </component>
</application>
`
	golandRecentProjectsPruned = `<application>
  <component name="RecentProjectsManager">
    <option name="additionalInfo">
      <map>
        <entry key="$USER_HOME$/GolandProjects/project-alpha">
          <value>
            <RecentProjectMetaInfo frameTitle="project-alpha">
              <option name="activationTimestamp" value="1" />
            </RecentProjectMetaInfo>
          </value>
        </entry>
        <entry key="$USER_HOME$/GolandProjects/other-project">
          <value>
            <RecentProjectMetaInfo frameTitle="other-project">
              <option name="activationTimestamp" value="4" />
            </RecentProjectMetaInfo>
          </value>
        </entry>
      </map>
    </option>
  </component>
</application>
`
	pycharmRecentProjects = `<application>
  <component name="RecentProjectsManager">
    <option name="additionalInfo">
      <map>
        <entry key="$USER_HOME$/PycharmProjects/scraper">
          <value><RecentProjectMetaInfo frameTitle="scraper" /></value>
        </entry>
        <entry key="$USER_HOME$/PycharmProjects/scraper/.claude/worktrees/baz-789">
          <value><RecentProjectMetaInfo frameTitle="baz-789" /></value>
        </entry>
      </map>
    </option>
  </component>
</application>
`
	pycharmRecentProjectsPruned = `<application>
  <component name="RecentProjectsManager">
    <option name="additionalInfo">
      <map>
        <entry key="$USER_HOME$/PycharmProjects/scraper">
          <value><RecentProjectMetaInfo frameTitle="scraper" /></value>
        </entry>
      </map>
    </option>
  </component>
</application>
`
	cleanRecentProjects = `<application>
  <component name="RecentProjectsManager">
    <option name="additionalInfo">
      <map>
        <entry key="$USER_HOME$/GolandProjects/clean-project">
          <value><RecentProjectMetaInfo frameTitle="clean-project" /></value>
        </entry>
      </map>
    </option>
  </component>
</application>
`
	singleLineRecentProjects = `<map>
  <entry key="$USER_HOME$/GolandProjects/kept-before"><value /></entry>
  <entry key="$USER_HOME$/GolandProjects/kept-before/.claude/worktrees/one-line"><value /></entry>
  <entry key="$USER_HOME$/GolandProjects/kept-after"><value /></entry>
</map>
`
	singleLineRecentProjectsPruned = `<map>
  <entry key="$USER_HOME$/GolandProjects/kept-before"><value /></entry>
  <entry key="$USER_HOME$/GolandProjects/kept-after"><value /></entry>
</map>
`
	golandFile        = "GoLand2026.1/options/recentProjects.xml"
	golandBackupFile  = golandFile + ".bak.20000101-000000"
	pycharmFile       = "PyCharm2026.1/options/recentProjects.xml"
	pycharmBackupFile = pycharmFile + ".bak.20000101-000000"
)

// readTree maps every file under dir (slash-separated relative path) to its
// content; a missing dir is an empty tree.
func readTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	tree := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("readTree: %w", err)
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return fmt.Errorf("readTree: %w", err)
		}

		tree[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return tree
	}

	require.NoError(t, err)
	return tree
}

// runningProducts fakes the running-IDE probe: the named products run.
func runningProducts(products ...string) func(ctx context.Context, product string) bool {
	return func(_ context.Context, product string) bool {
		return slices.Contains(products, product)
	}
}

// writeRecentProjects writes one IDE's Recent Projects file under configDir.
func writeRecentProjects(t *testing.T, configDir, rel, content string) {
	t.Helper()
	path := filepath.Join(configDir, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestJetbrainsPrune_run(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedFiles  map[string]string
		_expectedOutput string

		prune *jetbrainsPrune
	}

	now := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	var configDir string

	tests := make([]*testCase, 0)

	// no-config-dir
	configDir = filepath.Join(t.TempDir(), "JetBrains")
	tests = append(tests, &testCase{
		_id:             "no-config-dir",
		_expectedFiles:  map[string]string{},
		_expectedOutput: "no JetBrains config dir at " + configDir + "; nothing to do\n",

		prune: &jetbrainsPrune{configDir: configDir, isRunning: runningProducts(), now: now},
	})

	// no-entries
	configDir = t.TempDir()
	writeRecentProjects(t, configDir, golandFile, cleanRecentProjects)
	tests = append(tests, &testCase{
		_id:             "no-entries",
		_expectedFiles:  map[string]string{golandFile: cleanRecentProjects},
		_expectedOutput: "no worktree entries found; nothing to prune\n",

		prune: &jetbrainsPrune{configDir: configDir, isRunning: runningProducts(), now: now},
	})

	// dry-run-counts-and-changes-nothing
	configDir = t.TempDir()
	writeRecentProjects(t, configDir, golandFile, golandRecentProjects)
	writeRecentProjects(t, configDir, pycharmFile, pycharmRecentProjects)
	tests = append(tests, &testCase{
		_id: "dry-run-counts-and-changes-nothing",
		_expectedFiles: map[string]string{
			golandFile:  golandRecentProjects,
			pycharmFile: pycharmRecentProjects,
		},
		_expectedOutput: "would remove 2 worktree entries: " + golandFile + "\n" +
			"would remove 1 worktree entry: " + pycharmFile + "\n" +
			"dry run: 3 worktree entries would be removed\n",

		prune: &jetbrainsPrune{configDir: configDir, isDryRun: true, isRunning: runningProducts(), now: now},
	})

	// prune-removes-blocks-and-writes-backup
	configDir = t.TempDir()
	writeRecentProjects(t, configDir, golandFile, golandRecentProjects)
	writeRecentProjects(t, configDir, pycharmFile, pycharmRecentProjects)
	tests = append(tests, &testCase{
		_id: "prune-removes-blocks-and-writes-backup",
		_expectedFiles: map[string]string{
			golandBackupFile:  golandRecentProjects,
			golandFile:        golandRecentProjectsPruned,
			pycharmBackupFile: pycharmRecentProjects,
			pycharmFile:       pycharmRecentProjectsPruned,
		},
		_expectedOutput: "removed 2 worktree entries: " + golandFile + " (backup: recentProjects.xml.bak.20000101-000000)\n" +
			"removed 1 worktree entry: " + pycharmFile + " (backup: recentProjects.xml.bak.20000101-000000)\n" +
			"done: removed 3 worktree entries across 2 file(s)\n",

		prune: &jetbrainsPrune{configDir: configDir, isRunning: runningProducts(), now: now},
	})

	// single-line-entry
	configDir = t.TempDir()
	writeRecentProjects(t, configDir, golandFile, singleLineRecentProjects)
	tests = append(tests, &testCase{
		_id: "single-line-entry",
		_expectedFiles: map[string]string{
			golandBackupFile: singleLineRecentProjects,
			golandFile:       singleLineRecentProjectsPruned,
		},
		_expectedOutput: "removed 1 worktree entry: " + golandFile + " (backup: recentProjects.xml.bak.20000101-000000)\n" +
			"done: removed 1 worktree entry across 1 file(s)\n",

		prune: &jetbrainsPrune{configDir: configDir, isRunning: runningProducts(), now: now},
	})

	// running-ide-skipped
	configDir = t.TempDir()
	writeRecentProjects(t, configDir, golandFile, golandRecentProjects)
	tests = append(tests, &testCase{
		_id:            "running-ide-skipped",
		_expectedFiles: map[string]string{golandFile: golandRecentProjects},
		_expectedOutput: "skipping " + golandFile + ": GoLand is running (quit it or pass --force)\n" +
			"no worktree entries found; nothing to prune, skipped 1 running IDE(s)\n",

		prune: &jetbrainsPrune{configDir: configDir, isRunning: runningProducts("GoLand"), now: now},
	})

	// running-ide-forced-with-warning
	configDir = t.TempDir()
	writeRecentProjects(t, configDir, golandFile, golandRecentProjects)
	tests = append(tests, &testCase{
		_id: "running-ide-forced-with-warning",
		_expectedFiles: map[string]string{
			golandBackupFile: golandRecentProjects,
			golandFile:       golandRecentProjectsPruned,
		},
		_expectedOutput: "warning: GoLand is running; --force given, pruning anyway (edit may be clobbered on exit)\n" +
			"removed 2 worktree entries: " + golandFile + " (backup: recentProjects.xml.bak.20000101-000000)\n" +
			"done: removed 2 worktree entries across 1 file(s)\n",

		prune: &jetbrainsPrune{configDir: configDir, isForce: true, isRunning: runningProducts("GoLand"), now: now},
	})

	// running-ide-in-dry-run-reported
	configDir = t.TempDir()
	writeRecentProjects(t, configDir, golandFile, golandRecentProjects)
	tests = append(tests, &testCase{
		_id:            "running-ide-in-dry-run-reported",
		_expectedFiles: map[string]string{golandFile: golandRecentProjects},
		_expectedOutput: "would remove 2 worktree entries: " + golandFile + "\n" +
			"dry run: 2 worktree entries would be removed\n",

		prune: &jetbrainsPrune{configDir: configDir, isDryRun: true, isRunning: runningProducts("GoLand"), now: now},
	})

	// running-ide-forced-in-dry-run-warns
	configDir = t.TempDir()
	writeRecentProjects(t, configDir, golandFile, golandRecentProjects)
	tests = append(tests, &testCase{
		_id:            "running-ide-forced-in-dry-run-warns",
		_expectedFiles: map[string]string{golandFile: golandRecentProjects},
		_expectedOutput: "warning: GoLand is running; --force given, pruning anyway (edit may be clobbered on exit)\n" +
			"would remove 2 worktree entries: " + golandFile + "\n" +
			"dry run: 2 worktree entries would be removed\n",

		prune: &jetbrainsPrune{configDir: configDir, isDryRun: true, isForce: true, isRunning: runningProducts("GoLand"), now: now},
	})

	// two-ides-one-running
	configDir = t.TempDir()
	writeRecentProjects(t, configDir, golandFile, golandRecentProjects)
	writeRecentProjects(t, configDir, pycharmFile, pycharmRecentProjects)
	tests = append(tests, &testCase{
		_id: "two-ides-one-running",
		_expectedFiles: map[string]string{
			golandFile:        golandRecentProjects,
			pycharmBackupFile: pycharmRecentProjects,
			pycharmFile:       pycharmRecentProjectsPruned,
		},
		_expectedOutput: "skipping " + golandFile + ": GoLand is running (quit it or pass --force)\n" +
			"removed 1 worktree entry: " + pycharmFile + " (backup: recentProjects.xml.bak.20000101-000000)\n" +
			"done: removed 1 worktree entry across 1 file(s), skipped 1 running IDE(s)\n",

		prune: &jetbrainsPrune{configDir: configDir, isRunning: runningProducts("GoLand"), now: now},
	})

	// singular-dry-run-wording
	configDir = t.TempDir()
	writeRecentProjects(t, configDir, pycharmFile, pycharmRecentProjects)
	tests = append(tests, &testCase{
		_id:            "singular-dry-run-wording",
		_expectedFiles: map[string]string{pycharmFile: pycharmRecentProjects},
		_expectedOutput: "would remove 1 worktree entry: " + pycharmFile + "\n" +
			"dry run: 1 worktree entry would be removed\n",

		prune: &jetbrainsPrune{configDir: configDir, isDryRun: true, isRunning: runningProducts(), now: now},
	})

	// symlinked-file-left-alone
	configDir = t.TempDir()
	linkTarget := filepath.Join(t.TempDir(), "recentProjects.xml")
	require.NoError(t, os.WriteFile(linkTarget, []byte(golandRecentProjects), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(configDir, "GoLand2026.1", "options"), 0o755))
	require.NoError(t, os.Symlink(linkTarget, filepath.Join(configDir, filepath.FromSlash(golandFile))))
	tests = append(tests, &testCase{
		_id:             "symlinked-file-left-alone",
		_expectedFiles:  map[string]string{golandFile: golandRecentProjects},
		_expectedOutput: "no worktree entries found; nothing to prune\n",

		prune: &jetbrainsPrune{configDir: configDir, isRunning: runningProducts(), now: now},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			output, err := test.prune.run(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, test._expectedOutput, output)
			assert.Equal(t, test._expectedFiles, readTree(t, test.prune.configDir))
		})
	}
}

func TestProductName(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedPattern string
		_expectedProduct string

		rel string
	}

	tests := make([]*testCase, 0)

	// versioned-dir
	tests = append(tests, &testCase{
		_id:              "versioned-dir",
		_expectedPattern: `GoLand[^/]*\.app/Contents/MacOS`,
		_expectedProduct: "GoLand",

		rel: filepath.Join("GoLand2026.1", "options", "recentProjects.xml"),
	})

	// mapped-app-name
	tests = append(tests, &testCase{
		_id:              "mapped-app-name",
		_expectedPattern: `IntelliJ IDEA CE[^/]*\.app/Contents/MacOS`,
		_expectedProduct: "IdeaIC",

		rel: filepath.Join("IdeaIC2026.1", "options", "recentProjects.xml"),
	})

	// dir-without-version
	tests = append(tests, &testCase{
		_id:              "dir-without-version",
		_expectedPattern: `Fleet[^/]*\.app/Contents/MacOS`,
		_expectedProduct: "Fleet",

		rel: filepath.Join("Fleet", "options", "recentProjects.xml"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expectedProduct, productName(test.rel))
			assert.Equal(t, test._expectedPattern, appPattern(productName(test.rel)))
		})
	}
}

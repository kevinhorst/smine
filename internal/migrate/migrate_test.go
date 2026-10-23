package migrate

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/shell"
)

const (
	// absent marks a file that does not exist in a contents map.
	absent = "<absent>"
	// directory marks a path that is a directory in a contents map.
	directory = "<dir>"
)

func commitAll(t *testing.T, dir, message string) {
	t.Helper()
	gitOutput(t, dir, "add", "-A")
	gitOutput(t, dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", message)
}

// deleteStore removes every store file from the checkout in one commit.
func deleteStore(t *testing.T, checkout string) {
	t.Helper()
	for _, gitPath := range storeFiles {
		gitOutput(t, checkout, "rm", "-q", gitPath)
	}

	gitOutput(t, checkout, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "drop stores")
}

// everyOutcome maps every base-relative target of a fixture request to one
// outcome; cases override single keys.
func everyOutcome(request *Request, base string, outcome Outcome) map[string]Outcome {
	outcomes := make(map[string]Outcome)
	for _, step := range append(request.moveSteps(), request.storeSteps()...) {
		outcomes[relative(base, step.Target)] = outcome
	}
	outcomes[relative(base, filepath.Dir(request.StoreRoot))] = outcome
	return outcomes
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	output, err := shell.Run(context.Background(), dir, "git", args...)
	require.NoError(t, err, output)
	return strings.TrimSpace(output)
}

// newFixture returns a request below a fresh symlink-resolved base: the
// checkout <base>/checkout is a repo on main whose one commit holds every
// store file as "v1 <path>\n"; <base>/home/smine, the archive
// <base>/agent-plans and the store root <base>/agent-plans/checkout do not
// exist.
func newFixture(t *testing.T) (*Request, string) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	checkout := filepath.Join(base, "checkout")
	require.NoError(t, os.MkdirAll(checkout, 0o755))
	gitOutput(t, checkout, "init", "-q", "-b", "main")
	for _, gitPath := range storeFiles {
		writeFile(t, filepath.Join(checkout, filepath.FromSlash(gitPath)), "v1 "+gitPath+"\n")
	}

	writeFile(t, filepath.Join(checkout, ".gitignore"), "/repos.json\n")
	commitAll(t, checkout, "init")
	request := &Request{
		MainCheckout: checkout,
		StateDir:     filepath.Join(base, "home", "smine"),
		StoreRoot:    filepath.Join(base, "agent-plans", "checkout"),
	}
	return request, base
}

// outcomes maps each step's base-relative target to its outcome.
func outcomes(base string, steps []*Step) map[string]Outcome {
	result := make(map[string]Outcome)
	for _, step := range steps {
		result[relative(base, step.Target)] = step.Outcome
	}
	return result
}

// readContents reads every key of expected below base, absent marking a
// missing file and directory a directory.
func readContents(base string, expected map[string]string) map[string]string {
	contents := make(map[string]string)
	for key := range expected {
		fullPath := filepath.Join(base, filepath.FromSlash(key))
		info, err := os.Stat(fullPath)
		if errors.Is(err, fs.ErrNotExist) {
			contents[key] = absent
			continue
		}

		if err == nil && info.IsDir() {
			contents[key] = directory
			continue
		}

		data, err := os.ReadFile(fullPath)
		if err != nil {
			contents[key] = err.Error()
			continue
		}

		contents[key] = string(data)
	}
	return contents
}

func relative(base, target string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return filepath.ToSlash(rel)
}

// storeContents maps every store file below dir (base-relative, slash) to
// "<version> <path>\n", or to absent when version is absent.
func storeContents(contents map[string]string, dir, version string) map[string]string {
	for _, gitPath := range storeFiles {
		content := version + " " + gitPath + "\n"
		if version == absent {
			content = absent
		}

		contents[path.Join(dir, gitPath)] = content
	}
	return contents
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// writeMoveSources writes the untracked checkout file that moves into the
// state dir.
func writeMoveSources(t *testing.T, checkout string) {
	t.Helper()
	writeFile(t, filepath.Join(checkout, "repos.json"), "registry\n")
}

func TestRequest_Run(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedContents map[string]string
		_expectedOutcomes map[string]Outcome
		_shouldPass       bool

		base    string
		request *Request
	}

	tests := make([]*testCase, 0)

	// checkout-files-moved
	request, base := newFixture(t)
	writeMoveSources(t, request.MainCheckout)
	expectedOutcomes := everyOutcome(request, base, OutcomeCopied)
	expectedOutcomes["home/smine/repos.json"] = OutcomeMoved
	expectedOutcomes["agent-plans"] = OutcomeInitialized
	expectedContents := map[string]string{
		"checkout/repos.json":   absent,
		"home/smine/repos.json": "registry\n",
	}
	tests = append(tests, &testCase{
		_id:               "checkout-files-moved",
		_expectedContents: expectedContents,
		_expectedOutcomes: expectedOutcomes,
		_shouldPass:       true,

		base:    base,
		request: request,
	})

	// store-files-copied-from-working-tree
	request, base = newFixture(t)
	expectedOutcomes = everyOutcome(request, base, OutcomeCopied)
	expectedOutcomes["home/smine/repos.json"] = OutcomeAbsent
	expectedOutcomes["agent-plans"] = OutcomeInitialized
	expectedContents = make(map[string]string)
	expectedContents = storeContents(expectedContents, "agent-plans/checkout", "v1")
	expectedContents = storeContents(expectedContents, "checkout", "v1")
	tests = append(tests, &testCase{
		_id:               "store-files-copied-from-working-tree",
		_expectedContents: expectedContents,
		_expectedOutcomes: expectedOutcomes,
		_shouldPass:       true,

		base:    base,
		request: request,
	})

	// store-files-restored
	request, base = newFixture(t)
	deleteStore(t, request.MainCheckout)
	expectedOutcomes = everyOutcome(request, base, OutcomeRestored)
	expectedOutcomes["home/smine/repos.json"] = OutcomeAbsent
	expectedOutcomes["agent-plans"] = OutcomeInitialized
	expectedContents = make(map[string]string)
	expectedContents = storeContents(expectedContents, "agent-plans/checkout", "v1")
	expectedContents = storeContents(expectedContents, "checkout", absent)
	tests = append(tests, &testCase{
		_id:               "store-files-restored",
		_expectedContents: expectedContents,
		_expectedOutcomes: expectedOutcomes,
		_shouldPass:       true,

		base:    base,
		request: request,
	})

	// restore-picks-pre-deletion-content
	request, base = newFixture(t)
	for _, gitPath := range storeFiles {
		writeFile(t, filepath.Join(request.MainCheckout, filepath.FromSlash(gitPath)), "v2 "+gitPath+"\n")
	}

	commitAll(t, request.MainCheckout, "edit stores")
	deleteStore(t, request.MainCheckout)
	expectedOutcomes = everyOutcome(request, base, OutcomeRestored)
	expectedOutcomes["home/smine/repos.json"] = OutcomeAbsent
	expectedOutcomes["agent-plans"] = OutcomeInitialized
	expectedContents = make(map[string]string)
	expectedContents = storeContents(expectedContents, "agent-plans/checkout", "v2")
	tests = append(tests, &testCase{
		_id:               "restore-picks-pre-deletion-content",
		_expectedContents: expectedContents,
		_expectedOutcomes: expectedOutcomes,
		_shouldPass:       true,

		base:    base,
		request: request,
	})

	// restore-follows-first-parent
	request, base = newFixture(t)
	gitOutput(t, request.MainCheckout, "switch", "-q", "-c", "side")
	writeFile(t, filepath.Join(request.MainCheckout, "backlog", "backlog.json"), "side backlog/backlog.json\n")
	commitAll(t, request.MainCheckout, "side edit")
	gitOutput(t, request.MainCheckout, "rm", "-q", "backlog/backlog.json")
	commitAll(t, request.MainCheckout, "side delete")
	gitOutput(t, request.MainCheckout, "switch", "-q", "main")
	gitOutput(t, request.MainCheckout, "-c", "user.name=t", "-c", "user.email=t@t", "merge", "-q", "--no-ff", "-m", "merge side", "side")
	expectedOutcomes = everyOutcome(request, base, OutcomeCopied)
	expectedOutcomes["home/smine/repos.json"] = OutcomeAbsent
	expectedOutcomes["agent-plans"] = OutcomeInitialized
	expectedOutcomes["agent-plans/checkout/backlog/backlog.json"] = OutcomeRestored
	expectedContents = map[string]string{
		"agent-plans/checkout/backlog/backlog.json": "v1 backlog/backlog.json\n",
		"checkout/backlog/backlog.json":             absent,
	}
	tests = append(tests, &testCase{
		_id:               "restore-follows-first-parent",
		_expectedContents: expectedContents,
		_expectedOutcomes: expectedOutcomes,
		_shouldPass:       true,

		base:    base,
		request: request,
	})

	// target-exists-kept
	request, base = newFixture(t)
	writeMoveSources(t, request.MainCheckout)
	writeFile(t, filepath.Join(base, "home", "smine", "repos.json"), "existing registry\n")
	writeFile(t, filepath.Join(base, "agent-plans", "checkout", "backlog", "backlog.json"), "existing backlog\n")
	expectedOutcomes = everyOutcome(request, base, OutcomeCopied)
	expectedOutcomes["home/smine/repos.json"] = OutcomeKept
	expectedOutcomes["agent-plans"] = OutcomeInitialized
	expectedOutcomes["agent-plans/checkout/backlog/backlog.json"] = OutcomeKept
	expectedContents = map[string]string{
		"agent-plans/checkout/backlog/backlog.json": "existing backlog\n",
		"checkout/backlog/backlog.json":             "v1 backlog/backlog.json\n",
		"checkout/repos.json":                       "registry\n",
		"home/smine/repos.json":                     "existing registry\n",
	}
	tests = append(tests, &testCase{
		_id:               "target-exists-kept",
		_expectedContents: expectedContents,
		_expectedOutcomes: expectedOutcomes,
		_shouldPass:       true,

		base:    base,
		request: request,
	})

	// archive-repo-initialized
	request, base = newFixture(t)
	expectedOutcomes = everyOutcome(request, base, OutcomeCopied)
	expectedOutcomes["home/smine/repos.json"] = OutcomeAbsent
	expectedOutcomes["agent-plans"] = OutcomeInitialized
	expectedContents = map[string]string{
		"agent-plans/.git": directory,
	}
	tests = append(tests, &testCase{
		_id:               "archive-repo-initialized",
		_expectedContents: expectedContents,
		_expectedOutcomes: expectedOutcomes,
		_shouldPass:       true,

		base:    base,
		request: request,
	})

	// archive-repo-kept
	request, base = newFixture(t)
	require.NoError(t, os.MkdirAll(filepath.Join(base, "agent-plans"), 0o755))
	gitOutput(t, filepath.Join(base, "agent-plans"), "init", "-q")
	expectedOutcomes = everyOutcome(request, base, OutcomeCopied)
	expectedOutcomes["home/smine/repos.json"] = OutcomeAbsent
	expectedOutcomes["agent-plans"] = OutcomeKept
	expectedContents = map[string]string{
		"agent-plans/.git": directory,
	}
	tests = append(tests, &testCase{
		_id:               "archive-repo-kept",
		_expectedContents: expectedContents,
		_expectedOutcomes: expectedOutcomes,
		_shouldPass:       true,

		base:    base,
		request: request,
	})

	// second-run-no-op
	request, base = newFixture(t)
	writeMoveSources(t, request.MainCheckout)
	_, err := request.Run(context.Background())
	require.NoError(t, err)
	expectedOutcomes = everyOutcome(request, base, OutcomeKept)
	expectedContents = make(map[string]string)
	expectedContents = storeContents(expectedContents, "agent-plans/checkout", "v1")
	expectedContents["home/smine/repos.json"] = "registry\n"
	tests = append(tests, &testCase{
		_id:               "second-run-no-op",
		_expectedContents: expectedContents,
		_expectedOutcomes: expectedOutcomes,
		_shouldPass:       true,

		base:    base,
		request: request,
	})

	// checkout-not-a-repo-fails
	request, base = newFixture(t)
	request.MainCheckout = filepath.Join(base, "plain")
	require.NoError(t, os.MkdirAll(request.MainCheckout, 0o755))
	expectedOutcomes = map[string]Outcome{
		"agent-plans":           OutcomeInitialized,
		"home/smine/repos.json": OutcomeAbsent,
	}
	expectedContents = map[string]string{
		"agent-plans/checkout/backlog/backlog.json": absent,
	}
	tests = append(tests, &testCase{
		_id:               "checkout-not-a-repo-fails",
		_expectedContents: expectedContents,
		_expectedOutcomes: expectedOutcomes,
		_shouldPass:       false,

		base:    base,
		request: request,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			steps, err := test.request.Run(context.Background())
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedOutcomes, outcomes(test.base, steps))
			assert.Equal(t, test._expectedContents, readContents(test.base, test._expectedContents))
		})
	}
}

func TestStep_Line(t *testing.T) {
	step := &Step{Outcome: OutcomeMoved, Source: "/c/repos.json", Target: "/h/smine/repos.json"}
	assert.Equal(t, "moved       /h/smine/repos.json <- /c/repos.json", step.Line())
}

func TestCopyThenRemove(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "checkout", "repos.json")
	target := filepath.Join(base, "home", "smine", "repos.json")
	writeFile(t, source, "registry\n")

	err := copyThenRemove(source, target)

	require.NoError(t, err)
	content, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "registry\n", string(content))
	info, err := os.Stat(target)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
	assert.NoFileExists(t, source)
}

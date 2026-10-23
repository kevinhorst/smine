package main

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/shell"
)

const (
	cleanupPrefix = "claude-review/rr-a-"
	cleanupRunId  = "wf_071e500f-edc"
)

// cleanupFixture is one repo, the head its review pinned and a salvage dir
// beside the round dirs.
type cleanupFixture struct {
	head    string
	repo    string
	salvage string
	t       *testing.T
}

// newCleanupFixture returns a symlink-resolved repo — the paths git reports —
// with one commit holding file.txt, and a fresh salvage dir path.
func newCleanupFixture(t *testing.T) *cleanupFixture {
	t.Helper()
	fixture := &cleanupFixture{t: t}
	fixture.repo = fixture.tempDir()
	fixture.salvage = filepath.Join(fixture.tempDir(), "salvage")
	gitOutput(t, fixture.repo, "init", "-q", "-b", "main")
	writeCleanupFile(t, filepath.Join(fixture.repo, "file.txt"), "one\n")
	gitOutput(t, fixture.repo, "add", "-A")
	gitOutput(t, fixture.repo, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "init")
	fixture.head = gitOutput(t, fixture.repo, "rev-parse", "HEAD")
	return fixture
}

// addWorktree adds path on a new branch and returns path.
func (f *cleanupFixture) addWorktree(branch, path string) string {
	f.t.Helper()
	gitOutput(f.t, f.repo, "worktree", "add", "-q", "-b", branch, path)
	return path
}

// args is a cleanup or reset-round call over the fixture, extra appended.
func (f *cleanupFixture) args(extra ...string) []string {
	args := []string{"--repo", f.repo, "--prefix", cleanupPrefix, "--head", f.head, "--salvage", f.salvage}
	return append(args, extra...)
}

// breakWorktree points the worktree's .git file at a missing gitdir: every
// git call inside it fails, git worktree list still lists it.
func (f *cleanupFixture) breakWorktree(path string) {
	f.t.Helper()
	writeCleanupFile(f.t, filepath.Join(path, ".git"), "gitdir: /nonexistent\n")
}

// harnessWorktree adds <repo>/.claude/worktrees/<name> on worktree-<name>,
// the Workflow harness's isolation layout, and returns its path.
func (f *cleanupFixture) harnessWorktree(name string) string {
	f.t.Helper()
	return f.addWorktree("worktree-"+name, filepath.Join(f.repo, ".claude", "worktrees", name))
}

// reviewWorktree adds a worktree on branch under a fresh dir, named after the
// branch's last segment, and returns its path.
func (f *cleanupFixture) reviewWorktree(branch string) string {
	f.t.Helper()
	return f.addWorktree(branch, filepath.Join(f.tempDir(), filepath.Base(branch)))
}

// roundDir creates round-<round> beside the salvage dir holding the named
// files, each with its name as content, and returns it.
func (f *cleanupFixture) roundDir(round string, files ...string) string {
	f.t.Helper()
	dir := filepath.Join(filepath.Dir(f.salvage), "round-"+round)
	require.NoError(f.t, os.MkdirAll(dir, 0o755))
	for _, file := range files {
		writeCleanupFile(f.t, filepath.Join(dir, file), file+"\n")
	}
	return dir
}

// scope is a cleanup scope over the fixture.
func (f *cleanupFixture) scope() *cleanupScope {
	return &cleanupScope{Head: f.head, Prefix: cleanupPrefix, Repo: f.repo, Salvage: f.salvage}
}

// tempDir is a symlink-resolved temp dir.
func (f *cleanupFixture) tempDir() string {
	f.t.Helper()
	dir, err := filepath.EvalSymlinks(f.t.TempDir())
	require.NoError(f.t, err)
	return dir
}

// contentsOf reads each file, "" for a missing one.
func contentsOf(files []string) []string {
	contents := make([]string, 0, len(files))
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			contents = append(contents, "")
			continue
		}

		contents = append(contents, string(raw))
	}
	return contents
}

// dirEntries lists a dir's entry names, sorted; empty when it is absent.
func dirEntries(dir string) []string {
	names := make([]string, 0)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return names
	}

	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	output, err := shell.Run(context.Background(), dir, "git", args...)
	require.NoError(t, err, output)
	return strings.TrimSpace(output)
}

// localBranches lists the repo's local branches, sorted.
func localBranches(t *testing.T, repo string) []string {
	t.Helper()
	return strings.Fields(gitOutput(t, repo, "for-each-ref", "--format=%(refname:short)", "refs/heads"))
}

// onDisk keeps the paths that exist.
func onDisk(paths []string) []string {
	present := make([]string, 0, len(paths))
	for _, path := range paths {
		if isPresent(path) {
			present = append(present, path)
		}
	}
	return present
}

// skippedNames cuts each skipped row to its path or branch, dropping git's
// refusal or the salvage error.
func skippedNames(rows []string) []string {
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		name, _, _ := strings.Cut(row, ": ")
		names = append(names, name)
	}
	return names
}

func writeCleanupFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestCleanupScope_Clean(t *testing.T) {
	type testCase struct {
		_id                   string
		_expectedBranches     []string
		_expectedContents     []string
		_expectedRemoved      []string
		_expectedSkipped      []string
		_expectedUnattributed []string

		files []string
		scope *cleanupScope
	}

	tests := make([]*testCase, 0)

	// salvage-failure-skipped-next-removed
	failing := newCleanupFixture(t)
	failingBroken := failing.reviewWorktree(cleanupPrefix + "lane1")
	failingClean := failing.reviewWorktree(cleanupPrefix + "lane2")
	failing.breakWorktree(failingBroken)
	tests = append(tests, &testCase{
		_id:                   "salvage-failure-skipped-next-removed",
		_expectedBranches:     []string{cleanupPrefix + "lane2"},
		_expectedContents:     []string{},
		_expectedRemoved:      []string{failingClean},
		_expectedSkipped:      []string{failingBroken, cleanupPrefix + "lane1"},
		_expectedUnattributed: []string{},

		files: nil,
		scope: failing.scope(),
	})

	// existing-dest-suffixed
	existing := newCleanupFixture(t)
	existingLane := existing.reviewWorktree(cleanupPrefix + "lane1")
	writeCleanupFile(t, filepath.Join(existingLane, "new.txt"), "fresh\n")
	writeCleanupFile(t, filepath.Join(existing.salvage, "rr-a-lane1", "new.txt"), "earlier\n")
	tests = append(tests, &testCase{
		_id:                   "existing-dest-suffixed",
		_expectedBranches:     []string{cleanupPrefix + "lane1"},
		_expectedContents:     []string{"earlier\n", "fresh\n"},
		_expectedRemoved:      []string{existingLane},
		_expectedSkipped:      []string{},
		_expectedUnattributed: []string{},

		files: []string{filepath.Join(existing.salvage, "rr-a-lane1", "new.txt"), filepath.Join(existing.salvage, "rr-a-lane1-2", "new.txt")},
		scope: existing.scope(),
	})

	// run-harness-worktree-selected
	harness := newCleanupFixture(t)
	harnessPath := harness.harnessWorktree(cleanupRunId + "-1")
	harnessScope := harness.scope()
	harnessScope.RunIds = []string{cleanupRunId}
	tests = append(tests, &testCase{
		_id:                   "run-harness-worktree-selected",
		_expectedBranches:     []string{"worktree-" + cleanupRunId + "-1"},
		_expectedContents:     []string{},
		_expectedRemoved:      []string{harnessPath},
		_expectedSkipped:      []string{},
		_expectedUnattributed: []string{},

		files: nil,
		scope: harnessScope,
	})

	// foreign-review-unattributed
	foreign := newCleanupFixture(t)
	foreignPath := foreign.reviewWorktree("claude-review/rr-b-lane1")
	tests = append(tests, &testCase{
		_id:                   "foreign-review-unattributed",
		_expectedBranches:     []string{},
		_expectedContents:     []string{},
		_expectedRemoved:      []string{},
		_expectedSkipped:      []string{},
		_expectedUnattributed: []string{foreignPath},

		files: nil,
		scope: foreign.scope(),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			report, err := test.scope.clean(context.Background())
			require.NoError(t, err)
			assert.Equal(t, test._expectedBranches, report.Branches)
			assert.Equal(t, test._expectedRemoved, report.Removed)
			assert.Equal(t, test._expectedSkipped, skippedNames(report.Skipped))
			assert.Equal(t, test._expectedUnattributed, report.Unattributed)
			assert.Equal(t, test._expectedContents, contentsOf(test.files))
		})
	}
}

func TestRunCleanup(t *testing.T) {
	type testCase struct {
		_id               string
		_expected         int
		_expectedBranches []string
		_expectedOnDisk   []string

		args       []string
		candidates []string
		fixture    *cleanupFixture
	}

	tests := make([]*testCase, 0)

	// run-selects-unrenamed-harness-worktree
	selected := newCleanupFixture(t)
	selectedRun := selected.harnessWorktree(cleanupRunId + "-1")
	selectedNear := selected.harnessWorktree("wf_071e500f-edc2-1")
	selectedOther := selected.harnessWorktree("wf_0f7f2d73-f7b-1")
	selectedNested := selected.addWorktree("nested", filepath.Join(selected.repo, ".claude", "worktrees", "sub", cleanupRunId+"-2"))
	tests = append(tests, &testCase{
		_id:               "run-selects-unrenamed-harness-worktree",
		_expected:         exitClean,
		_expectedBranches: []string{"main", "nested", "worktree-wf_071e500f-edc2-1", "worktree-wf_0f7f2d73-f7b-1"},
		_expectedOnDisk:   []string{selectedNear, selectedOther, selectedNested},

		args:       selected.args("--run", cleanupRunId),
		candidates: []string{selectedRun, selectedNear, selectedOther, selectedNested},
		fixture:    selected,
	})

	// run-worktree-renamed-to-prefix-branch
	renamed := newCleanupFixture(t)
	renamedPath := renamed.addWorktree(cleanupPrefix+"correctness-lane1-r1-abc", filepath.Join(renamed.repo, ".claude", "worktrees", cleanupRunId+"-2"))
	tests = append(tests, &testCase{
		_id:               "run-worktree-renamed-to-prefix-branch",
		_expected:         exitClean,
		_expectedBranches: []string{"main"},
		_expectedOnDisk:   []string{},

		args:       renamed.args("--run", cleanupRunId),
		candidates: []string{renamedPath},
		fixture:    renamed,
	})

	// run-worktree-foreign-branch-kept
	foreign := newCleanupFixture(t)
	foreignPath := foreign.addWorktree("claude/session-x", filepath.Join(foreign.repo, ".claude", "worktrees", cleanupRunId+"-3"))
	tests = append(tests, &testCase{
		_id:               "run-worktree-foreign-branch-kept",
		_expected:         exitClean,
		_expectedBranches: []string{"claude/session-x", "main"},
		_expectedOnDisk:   []string{},

		args:       foreign.args("--run", cleanupRunId),
		candidates: []string{foreignPath},
		fixture:    foreign,
	})

	// without-run-harness-worktree-untouched
	untouched := newCleanupFixture(t)
	untouchedPath := untouched.harnessWorktree(cleanupRunId + "-1")
	tests = append(tests, &testCase{
		_id:               "without-run-harness-worktree-untouched",
		_expected:         exitClean,
		_expectedBranches: []string{"main", "worktree-" + cleanupRunId + "-1"},
		_expectedOnDisk:   []string{untouchedPath},

		args:       untouched.args(),
		candidates: []string{untouchedPath},
		fixture:    untouched,
	})

	// malformed-run-id-uppercase
	upper := newCleanupFixture(t)
	upperPath := upper.harnessWorktree("wf_ABC-1")
	tests = append(tests, &testCase{
		_id:               "malformed-run-id-uppercase",
		_expected:         exitError,
		_expectedBranches: []string{"main", "worktree-wf_ABC-1"},
		_expectedOnDisk:   []string{upperPath},

		args:       upper.args("--run", "wf_ABC"),
		candidates: []string{upperPath},
		fixture:    upper,
	})

	// malformed-run-id-without-wf
	bare := newCleanupFixture(t)
	barePath := bare.harnessWorktree(cleanupRunId + "-1")
	tests = append(tests, &testCase{
		_id:               "malformed-run-id-without-wf",
		_expected:         exitError,
		_expectedBranches: []string{"main", "worktree-" + cleanupRunId + "-1"},
		_expectedOnDisk:   []string{barePath},

		args:       bare.args("--run", cleanupRunId, "--run", "071e500f-edc"),
		candidates: []string{barePath},
		fixture:    bare,
	})

	// prefix-outside-claude-review
	outside := newCleanupFixture(t)
	outsidePath := outside.reviewWorktree("review/rr-a-lane1")
	tests = append(tests, &testCase{
		_id:               "prefix-outside-claude-review",
		_expected:         exitError,
		_expectedBranches: []string{"main", "review/rr-a-lane1"},
		_expectedOnDisk:   []string{outsidePath},

		args:       []string{"--repo", outside.repo, "--prefix", "review/rr-a-", "--head", outside.head, "--salvage", outside.salvage},
		candidates: []string{outsidePath},
		fixture:    outside,
	})

	// salvage-failure-exit-violations
	failing := newCleanupFixture(t)
	failingBroken := failing.reviewWorktree(cleanupPrefix + "lane1")
	failingClean := failing.reviewWorktree(cleanupPrefix + "lane2")
	failing.breakWorktree(failingBroken)
	tests = append(tests, &testCase{
		_id:               "salvage-failure-exit-violations",
		_expected:         exitViolations,
		_expectedBranches: []string{cleanupPrefix + "lane1", "main"},
		_expectedOnDisk:   []string{failingBroken},

		args:       failing.args(),
		candidates: []string{failingBroken, failingClean},
		fixture:    failing,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, runCleanup(test.args))
			assert.Equal(t, test._expectedBranches, localBranches(t, test.fixture.repo))
			assert.Equal(t, test._expectedOnDisk, onDisk(test.candidates))
		})
	}
}

func TestRunResetRound(t *testing.T) {
	type testCase struct {
		_id                     string
		_expected               int
		_expectedBranches       []string
		_expectedContents       []string
		_expectedOnDisk         []string
		_expectedRoundEntries   []string
		_expectedSalvageEntries []string

		args       []string
		candidates []string
		files      []string
		fixture    *cleanupFixture
		roundDir   string
	}

	tests := make([]*testCase, 0)

	// round-reset
	reset := newCleanupFixture(t)
	resetOne := reset.reviewWorktree(cleanupPrefix + "correctness-lane1-r1-abc")
	resetTwo := reset.reviewWorktree(cleanupPrefix + "correctness-lane1-r2-abc")
	resetHarness := reset.harnessWorktree(cleanupRunId + "-1")
	gitOutput(t, reset.repo, "branch", cleanupPrefix+"station-r1-abc")
	gitOutput(t, reset.repo, "branch", cleanupPrefix+"correctness-lane1-r10-abc")
	resetDir := reset.roundDir("1", "lane.json", filepath.Join("probes", "p.json"))
	tests = append(tests, &testCase{
		_id:                     "round-reset",
		_expected:               exitClean,
		_expectedBranches:       []string{cleanupPrefix + "correctness-lane1-r10-abc", cleanupPrefix + "correctness-lane1-r2-abc", "main"},
		_expectedContents:       []string{"lane.json\n", filepath.Join("probes", "p.json") + "\n"},
		_expectedOnDisk:         []string{resetTwo},
		_expectedRoundEntries:   []string{},
		_expectedSalvageEntries: []string{"branches", "round-1", "rr-a-correctness-lane1-r1-abc", cleanupRunId + "-1"},

		args:       reset.args("--round-dir", resetDir, "--round", "1", "--run", cleanupRunId),
		candidates: []string{resetOne, resetTwo, resetHarness},
		files:      []string{filepath.Join(reset.salvage, "round-1", "lane.json"), filepath.Join(reset.salvage, "round-1", "probes", "p.json")},
		fixture:    reset,
		roundDir:   resetDir,
	})

	// round-files-into-existing-dest
	again := newCleanupFixture(t)
	againDir := again.roundDir("1", "lane.json")
	writeCleanupFile(t, filepath.Join(again.salvage, "round-1", "lane.json"), "earlier\n")
	tests = append(tests, &testCase{
		_id:                     "round-files-into-existing-dest",
		_expected:               exitClean,
		_expectedBranches:       []string{"main"},
		_expectedContents:       []string{"earlier\n", "lane.json\n"},
		_expectedOnDisk:         []string{},
		_expectedRoundEntries:   []string{},
		_expectedSalvageEntries: []string{"round-1", "round-1-2"},

		args:       again.args("--round-dir", againDir, "--round", "1"),
		candidates: nil,
		files:      []string{filepath.Join(again.salvage, "round-1", "lane.json"), filepath.Join(again.salvage, "round-1-2", "lane.json")},
		fixture:    again,
		roundDir:   againDir,
	})

	// already-clean-round-idempotent
	clean := newCleanupFixture(t)
	cleanOne := clean.reviewWorktree(cleanupPrefix + "correctness-lane1-r1-abc")
	cleanDir := clean.roundDir("1", "lane.json")
	cleanArgs := clean.args("--round-dir", cleanDir, "--round", "1")
	require.Equal(t, exitClean, runResetRound(cleanArgs))
	tests = append(tests, &testCase{
		_id:                     "already-clean-round-idempotent",
		_expected:               exitClean,
		_expectedBranches:       []string{"main"},
		_expectedContents:       []string{"lane.json\n"},
		_expectedOnDisk:         []string{},
		_expectedRoundEntries:   []string{},
		_expectedSalvageEntries: []string{"branches", "round-1", "rr-a-correctness-lane1-r1-abc"},

		args:       cleanArgs,
		candidates: []string{cleanOne},
		files:      []string{filepath.Join(clean.salvage, "round-1", "lane.json")},
		fixture:    clean,
		roundDir:   cleanDir,
	})

	// round-dir-mismatch
	mismatch := newCleanupFixture(t)
	mismatchOne := mismatch.reviewWorktree(cleanupPrefix + "correctness-lane1-r1-abc")
	mismatchDir := mismatch.roundDir("1", "lane.json")
	tests = append(tests, &testCase{
		_id:                     "round-dir-mismatch",
		_expected:               exitError,
		_expectedBranches:       []string{cleanupPrefix + "correctness-lane1-r1-abc", "main"},
		_expectedContents:       []string{},
		_expectedOnDisk:         []string{mismatchOne},
		_expectedRoundEntries:   []string{"lane.json"},
		_expectedSalvageEntries: []string{},

		args:       mismatch.args("--round-dir", mismatchDir, "--round", "2"),
		candidates: []string{mismatchOne},
		files:      nil,
		fixture:    mismatch,
		roundDir:   mismatchDir,
	})

	// salvage-inside-round-dir
	inside := newCleanupFixture(t)
	insideOne := inside.reviewWorktree(cleanupPrefix + "correctness-lane1-r1-abc")
	insideDir := inside.roundDir("1", "lane.json")
	tests = append(tests, &testCase{
		_id:                     "salvage-inside-round-dir",
		_expected:               exitError,
		_expectedBranches:       []string{cleanupPrefix + "correctness-lane1-r1-abc", "main"},
		_expectedContents:       []string{},
		_expectedOnDisk:         []string{insideOne},
		_expectedRoundEntries:   []string{"lane.json"},
		_expectedSalvageEntries: []string{},

		args:       []string{"--repo", inside.repo, "--prefix", cleanupPrefix, "--head", inside.head, "--salvage", filepath.Join(insideDir, "salvage"), "--round-dir", insideDir, "--round", "1"},
		candidates: []string{insideOne},
		files:      nil,
		fixture:    inside,
		roundDir:   insideDir,
	})

	// round-missing
	noRound := newCleanupFixture(t)
	noRoundOne := noRound.reviewWorktree(cleanupPrefix + "correctness-lane1-r1-abc")
	noRoundDir := noRound.roundDir("1", "lane.json")
	tests = append(tests, &testCase{
		_id:                     "round-missing",
		_expected:               exitError,
		_expectedBranches:       []string{cleanupPrefix + "correctness-lane1-r1-abc", "main"},
		_expectedContents:       []string{},
		_expectedOnDisk:         []string{noRoundOne},
		_expectedRoundEntries:   []string{"lane.json"},
		_expectedSalvageEntries: []string{},

		args:       noRound.args("--round-dir", noRoundDir),
		candidates: []string{noRoundOne},
		files:      nil,
		fixture:    noRound,
		roundDir:   noRoundDir,
	})

	// round-dir-absent
	absent := newCleanupFixture(t)
	absentOne := absent.reviewWorktree(cleanupPrefix + "correctness-lane1-r1-abc")
	absentDir := filepath.Join(filepath.Dir(absent.salvage), "round-1")
	tests = append(tests, &testCase{
		_id:                     "round-dir-absent",
		_expected:               exitError,
		_expectedBranches:       []string{cleanupPrefix + "correctness-lane1-r1-abc", "main"},
		_expectedContents:       []string{},
		_expectedOnDisk:         []string{absentOne},
		_expectedRoundEntries:   []string{},
		_expectedSalvageEntries: []string{},

		args:       absent.args("--round-dir", absentDir, "--round", "1"),
		candidates: []string{absentOne},
		files:      nil,
		fixture:    absent,
		roundDir:   absentDir,
	})

	// malformed-run-id
	malformed := newCleanupFixture(t)
	malformedOne := malformed.reviewWorktree(cleanupPrefix + "correctness-lane1-r1-abc")
	malformedDir := malformed.roundDir("1", "lane.json")
	tests = append(tests, &testCase{
		_id:                     "malformed-run-id",
		_expected:               exitError,
		_expectedBranches:       []string{cleanupPrefix + "correctness-lane1-r1-abc", "main"},
		_expectedContents:       []string{},
		_expectedOnDisk:         []string{malformedOne},
		_expectedRoundEntries:   []string{"lane.json"},
		_expectedSalvageEntries: []string{},

		args:       malformed.args("--round-dir", malformedDir, "--round", "1", "--run", "wf_x/../y"),
		candidates: []string{malformedOne},
		files:      nil,
		fixture:    malformed,
		roundDir:   malformedDir,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, runResetRound(test.args))
			assert.Equal(t, test._expectedBranches, localBranches(t, test.fixture.repo))
			assert.Equal(t, test._expectedOnDisk, onDisk(test.candidates))
			assert.Equal(t, test._expectedRoundEntries, dirEntries(test.roundDir))
			assert.Equal(t, test._expectedSalvageEntries, dirEntries(test.fixture.salvage))
			assert.Equal(t, test._expectedContents, contentsOf(test.files))
		})
	}
}

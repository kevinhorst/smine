package reviewscope

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	probeReplayBranch   = "claude-review/test-probe-replay"
	probeReplayMakefile = "build:\n\tmkdir -p bin\n\tcp acdsl.sh bin/acdsl\n\tchmod +x bin/acdsl\n\ntest:\n\ttrue\n"
)

// errorText is the error's message, empty for nil.
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// provideFakeAcdslTree is a tree whose bin/acdsl runs the given sh script.
func provideFakeAcdslTree(t *testing.T, script string) string {
	t.Helper()
	tree := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tree, "bin"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tree, "bin", "acdsl"), []byte("#!/bin/sh\n"+script+"\n"), 0o755))
	return tree
}

// provideProbeRound creates round-<n> under parent with a review.json holding probe and its registry file.
func provideProbeRound(t *testing.T, parent, round string, probe reviewProbe) string {
	t.Helper()
	roundDir := filepath.Join(parent, "round-"+round)
	require.NoError(t, os.MkdirAll(filepath.Join(roundDir, "probes"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(roundDir, "probes", probe.Registry), []byte("{}\n"), 0o644))
	writeProbeReview(t, roundDir, probe)
	return roundDir
}

// provideProbeRepo commits the Makefile and the script the build copies to bin/acdsl, and returns the tip.
func provideProbeRepo(fixture *gitFixture, script string) string {
	commitFixtureFile(fixture, "#!/bin/sh\n"+script+"\n", "acdsl.sh")
	return commitFixtureFile(fixture, probeReplayMakefile, "Makefile")
}

// provideRoundWithMirrors creates round-<n> under parent with one probe
// mirror per name.
func provideRoundWithMirrors(t *testing.T, parent, round string, names ...string) string {
	t.Helper()
	roundDir := filepath.Join(parent, "round-"+round)
	require.NoError(t, os.MkdirAll(filepath.Join(roundDir, "probes"), 0o755))
	for _, name := range names {
		require.NoError(t, os.WriteFile(filepath.Join(roundDir, "probes", name), []byte("rule "+round+" "+name+"\n"), 0o644))
	}
	return roundDir
}

// readTree maps every file name in dir to its content.
func readTree(t *testing.T, dir string) map[string]string {
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	files := map[string]string{}
	for _, entry := range entries {
		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		require.NoError(t, err)
		files[entry.Name()] = string(content)
	}
	return files
}

// writeProbeReview writes roundDir's review.json indexing probes.
func writeProbeReview(t *testing.T, roundDir string, probes ...reviewProbe) {
	t.Helper()
	review := &reviewDocument{Probes: probes, RankedPlan: []rankedEntry{}}
	data, err := json.Marshal(review)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(roundDir, reviewFile), data, 0o644))
}

func TestCopyMirrors(t *testing.T) {
	type testCase struct {
		_id       string
		_expected map[string]string

		roundDirs []string
		tree      string
	}

	tests := make([]*testCase, 0)

	// per-round-prefix
	parent := t.TempDir()
	firstRound := provideRoundWithMirrors(t, parent, "1", "f1.acdsl", "notes.txt")
	secondRound := provideRoundWithMirrors(t, parent, "2", "f1.acdsl", "f4.acdsl")
	tests = append(tests, &testCase{
		_id: "per-round-prefix",
		_expected: map[string]string{
			"railroad-probes-round-1-f1.acdsl": "rule 1 f1.acdsl\n",
			"railroad-probes-round-2-f1.acdsl": "rule 2 f1.acdsl\n",
			"railroad-probes-round-2-f4.acdsl": "rule 2 f4.acdsl\n",
		},

		roundDirs: []string{firstRound, secondRound},
		tree:      t.TempDir(),
	})

	// no-mirrors
	tests = append(tests, &testCase{
		_id:       "no-mirrors",
		_expected: map[string]string{},

		roundDirs: []string{filepath.Join(t.TempDir(), "round-1")},
		tree:      t.TempDir(),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := copyMirrors(test.roundDirs, test.tree)
			require.NoError(t, err)
			assert.Equal(t, test._expected, readTree(t, test.tree))
		})
	}
}

func TestBuildForReplay(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedErrorText string
		_expectedLog       string
		_shouldPass        bool

		build  [][]string
		logOut *bytes.Buffer
		tree   string
	}

	flagMakefile := "build:\n\t@exit 1\n\nflag-build:\n\ttouch built\n\nsecond-build:\n\ttest -f built\n\nbroken:\n\t@exit 1\n"
	tests := make([]*testCase, 0)

	// configured-build-runs
	configuredTree := provideTree(t, map[string]string{"Makefile": flagMakefile})
	tests = append(tests, &testCase{
		_id:                "configured-build-runs",
		_expectedErrorText: "",
		_expectedLog:       "build (make flag-build): ok",
		_shouldPass:        true,

		build:  [][]string{{"make", "flag-build"}},
		logOut: &bytes.Buffer{},
		tree:   configuredTree,
	})

	// two-lines-logged-joined
	tests = append(tests, &testCase{
		_id:                "two-lines-logged-joined",
		_expectedErrorText: "",
		_expectedLog:       "build (make flag-build; make second-build): ok",
		_shouldPass:        true,

		build:  [][]string{{"make", "flag-build"}, {"make", "second-build"}},
		logOut: &bytes.Buffer{},
		tree:   provideTree(t, map[string]string{"Makefile": flagMakefile}),
	})

	// failing-build-names-command
	tests = append(tests, &testCase{
		_id:                "failing-build-names-command",
		_expectedErrorText: "buildForReplay: Build make broken failed:",
		_expectedLog:       "build (make broken): failed:",
		_shouldPass:        false,

		build:  [][]string{{"make", "broken"}},
		logOut: &bytes.Buffer{},
		tree:   provideTree(t, map[string]string{"Makefile": flagMakefile}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := buildForReplay(context.Background(), test.logOut, test.tree, test.build)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Contains(t, errorText(err), test._expectedErrorText)
			assert.Contains(t, test.logOut.String(), test._expectedLog)
		})
	}

	assert.True(t, pathExists(filepath.Join(configuredTree, "built")), "the configured build ran in the tree")
}

func TestCheckProbe(t *testing.T) {
	type testCase struct {
		_id       string
		_expected ProbeResult

		program string
		tree    string
	}

	roundDir := filepath.Join(t.TempDir(), "round-2")
	probe := reviewProbe{FindingId: "f3-r2", Id: "ACDSL-PROBE-003", Registry: "registry-f3.json"}
	tests := make([]*testCase, 0)

	// exit-0-green
	greenTree := provideFakeAcdslTree(t, "exit 0")
	tests = append(tests, &testCase{
		_id:       "exit-0-green",
		_expected: ProbeResult{FindingId: "f3-r2", Id: "ACDSL-PROBE-003", Result: resultGreen, Round: "round-2"},

		program: filepath.Join(greenTree, "bin", "acdsl"),
		tree:    greenTree,
	})

	// exit-1-red
	redTree := provideFakeAcdslTree(t, "echo violation\nexit 1")
	tests = append(tests, &testCase{
		_id:       "exit-1-red",
		_expected: ProbeResult{FindingId: "f3-r2", Id: "ACDSL-PROBE-003", Result: resultRed, Round: "round-2"},

		program: filepath.Join(redTree, "bin", "acdsl"),
		tree:    redTree,
	})

	// exit-2-error
	errorTree := provideFakeAcdslTree(t, "echo registry unreadable\nexit 2")
	tests = append(tests, &testCase{
		_id:       "exit-2-error",
		_expected: ProbeResult{FindingId: "f3-r2", Id: "ACDSL-PROBE-003", Result: "error: registry unreadable", Round: "round-2"},

		program: filepath.Join(errorTree, "bin", "acdsl"),
		tree:    errorTree,
	})

	// program-split-into-argv
	tests = append(tests, &testCase{
		_id:       "program-split-into-argv",
		_expected: ProbeResult{FindingId: "f3-r2", Id: "ACDSL-PROBE-003", Result: "error: argv: --flavor x check -rule ACDSL-PROBE-003", Round: "round-2"},

		program: "sh bin/acdsl --flavor x",
		tree:    provideFakeAcdslTree(t, "echo argv: $1 $2 $3 $4 $5\nexit 2"),
	})

	// unparsable-program-errors
	tests = append(tests, &testCase{
		_id:       "unparsable-program-errors",
		_expected: ProbeResult{FindingId: "f3-r2", Id: "ACDSL-PROBE-003", Result: `error: cmdline.Split: Unbalanced quote in "sh \"bin/acdsl"`, Round: "round-2"},

		program: `sh "bin/acdsl`,
		tree:    provideFakeAcdslTree(t, "exit 0"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, checkProbe(context.Background(), probe, test.program, roundDir, test.tree))
		})
	}
}

func TestReplayProbes(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedErrorText string
		_expectedHits      []ProbeResult
		_expectedProbes    []ProbeResult
		_shouldPass        bool

		fixture *gitFixture
		request *ProbeRequest
	}

	probe := reviewProbe{FindingId: "f1-r1", Id: "ACDSL-PROBE-001", Registry: "registry-f1.json"}
	green := ProbeResult{FindingId: "f1-r1", Id: "ACDSL-PROBE-001", Result: resultGreen, Round: "round-1"}
	red := ProbeResult{FindingId: "f1-r1", Id: "ACDSL-PROBE-001", Result: resultRed, Round: "round-1"}
	tests := make([]*testCase, 0)

	// green-flag-build
	greenRepo := newGitFixture(t)
	greenHead := provideProbeRepo(greenRepo, "exit 0")
	tests = append(tests, &testCase{
		_id:                "green-flag-build",
		_expectedErrorText: "",
		_expectedHits:      []ProbeResult{},
		_expectedProbes:    []ProbeResult{green},
		_shouldPass:        true,

		fixture: greenRepo,
		request: &ProbeRequest{Branch: probeReplayBranch, Build: makeBuild, Head: greenHead, Repo: greenRepo.repo, Root: t.TempDir(), RoundDirs: []string{provideProbeRound(t, t.TempDir(), "1", probe)}},
	})

	// two-build-lines
	twoLinesRepo := newGitFixture(t)
	commitFixtureFile(twoLinesRepo, "#!/bin/sh\nexit 0\n", "acdsl.sh")
	twoLinesHead := commitFixtureFile(twoLinesRepo, "bin-dir:\n\tmkdir -p bin\n\nacdsl:\n\tcp acdsl.sh bin/acdsl\n\tchmod +x bin/acdsl\n", "Makefile")
	tests = append(tests, &testCase{
		_id:                "two-build-lines",
		_expectedErrorText: "",
		_expectedHits:      []ProbeResult{},
		_expectedProbes:    []ProbeResult{green},
		_shouldPass:        true,

		fixture: twoLinesRepo,
		request: &ProbeRequest{Branch: probeReplayBranch, Build: "make bin-dir\nmake acdsl", Head: twoLinesHead, Repo: twoLinesRepo.repo, Root: t.TempDir(), RoundDirs: []string{provideProbeRound(t, t.TempDir(), "1", probe)}},
	})

	// red-probe
	redRepo := newGitFixture(t)
	redHead := provideProbeRepo(redRepo, "echo violation\nexit 1")
	tests = append(tests, &testCase{
		_id:                "red-probe",
		_expectedErrorText: "",
		_expectedHits:      []ProbeResult{red},
		_expectedProbes:    []ProbeResult{red},
		_shouldPass:        true,

		fixture: redRepo,
		request: &ProbeRequest{Branch: probeReplayBranch, Build: makeBuild, Head: redHead, Repo: redRepo.repo, Root: t.TempDir(), RoundDirs: []string{provideProbeRound(t, t.TempDir(), "1", probe)}},
	})

	// build-failed
	failedRepo := newGitFixture(t)
	failedHead := commitFixtureFile(failedRepo, "build:\n\t@echo build broke\n\t@exit 1\n\ntest:\n\ttrue\n", "Makefile")
	tests = append(tests, &testCase{
		_id:                "build-failed",
		_expectedErrorText: "buildForReplay: Build make build failed:",
		_expectedHits:      []ProbeResult{},
		_expectedProbes:    []ProbeResult{},
		_shouldPass:        false,

		fixture: failedRepo,
		request: &ProbeRequest{Branch: probeReplayBranch, Build: makeBuild, Head: failedHead, Repo: failedRepo.repo, Root: t.TempDir(), RoundDirs: []string{provideProbeRound(t, t.TempDir(), "1", probe)}},
	})

	// configured-build-and-acdsl
	configuredRepo := newGitFixture(t)
	configuredHead := commitFixtureFile(configuredRepo, "#!/bin/sh\nexit 0\n", "acdsl.sh")
	tests = append(tests, &testCase{
		_id:                "configured-build-and-acdsl",
		_expectedErrorText: "",
		_expectedHits:      []ProbeResult{},
		_expectedProbes:    []ProbeResult{green},
		_shouldPass:        true,

		fixture: configuredRepo,
		request: &ProbeRequest{Acdsl: "sh acdsl.sh", Branch: probeReplayBranch, Build: "true", Head: configuredHead, Repo: configuredRepo.repo, Root: t.TempDir(), RoundDirs: []string{provideProbeRound(t, t.TempDir(), "1", probe)}},
	})

	// no-acdsl-after-build
	goRepo := newGitFixture(t)
	goHead := commitFixtureFile(goRepo, "module example.com/probe\n", "go.mod")
	tests = append(tests, &testCase{
		_id:                "no-acdsl-after-build",
		_expectedErrorText: "No bin/acdsl in",
		_expectedHits:      []ProbeResult{},
		_expectedProbes:    []ProbeResult{},
		_shouldPass:        false,

		fixture: goRepo,
		request: &ProbeRequest{Branch: probeReplayBranch, Build: "true", Head: goHead, Repo: goRepo.repo, Root: t.TempDir(), RoundDirs: []string{provideProbeRound(t, t.TempDir(), "1", probe)}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			replay, err := ReplayProbes(context.Background(), test.request)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Contains(t, errorText(err), test._expectedErrorText)
			assert.Equal(t, test._expectedHits, replay.Hits)
			assert.Equal(t, test._expectedProbes, replay.Probes)
			assert.True(t, replay.Salvage.Removed, replay.Salvage.Reason)
			assert.False(t, pathExists(replay.Salvage.Path))
			assert.Empty(t, test.fixture.run("branch", "--list", probeReplayBranch))
		})
	}
}

func TestReplayProbes_Unconfigured(t *testing.T) {
	type testCase struct {
		_id                string
		_expected          *ProbeReplay
		_expectedErrorText string
		_shouldPass        bool

		request *ProbeRequest
	}

	probe := reviewProbe{FindingId: "f1-r1", Id: "ACDSL-PROBE-001", Registry: "registry-f1.json"}
	absentRepo := filepath.Join(t.TempDir(), "absent-repo")
	tests := make([]*testCase, 0)

	// missing-build-exits-with-fix
	tests = append(tests, &testCase{
		_id:                "missing-build-exits-with-fix",
		_expected:          nil,
		_expectedErrorText: "ReplayProbes: commandLines: No build command — the repo's build and test commands are set on the skill page",
		_shouldPass:        false,

		request: &ProbeRequest{Branch: probeReplayBranch, Build: "", Head: "deadbeef", Repo: absentRepo, Root: t.TempDir(), RoundDirs: []string{provideProbeRound(t, t.TempDir(), "1", probe)}},
	})

	// no-probes-needs-no-build
	parent := t.TempDir()
	firstRound := filepath.Join(parent, "round-1")
	secondRound := filepath.Join(parent, "round-2")
	require.NoError(t, os.MkdirAll(firstRound, 0o755))
	require.NoError(t, os.MkdirAll(secondRound, 0o755))
	writeProbeReview(t, firstRound)
	writeProbeReview(t, secondRound)
	tests = append(tests, &testCase{
		_id:                "no-probes-needs-no-build",
		_expected:          &ProbeReplay{Hits: []ProbeResult{}, Probes: []ProbeResult{}, Salvage: nil},
		_expectedErrorText: "",
		_shouldPass:        true,

		request: &ProbeRequest{Branch: probeReplayBranch, Build: "", Head: "deadbeef", Repo: absentRepo, Root: t.TempDir(), RoundDirs: []string{firstRound, secondRound}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			replay, err := ReplayProbes(context.Background(), test.request)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedErrorText, errorText(err))
			assert.Equal(t, test._expected, replay)
			assert.Empty(t, readTree(t, test.request.Root), "a run that builds nothing provisions no worktree")
			assert.False(t, pathExists(filepath.Join(test.request.RoundDirs[len(test.request.RoundDirs)-1], "probe-replay.log")))
		})
	}
}

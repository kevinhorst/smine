package skilleval

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/evals"
)

func TestResolveRepo(t *testing.T) {
	type testCase struct {
		_id            string
		_expected      string
		_expectedIsErr bool

		name string
	}

	mainCheckout := newTempDir(t)
	registered := newTempDir(t)
	registryPath := filepath.Join(newTempDir(t), config.StateRegistryFile)
	writeTestFile(t, registryPath, `{"repos":[{"name":"peek-mcp","path":"`+registered+`"}]}`)

	tests := make([]*testCase, 0)

	// empty-name-main-checkout
	tests = append(tests, &testCase{
		_id:            "empty-name-main-checkout",
		_expected:      mainCheckout,
		_expectedIsErr: false,

		name: "",
	})

	// registered-name
	tests = append(tests, &testCase{
		_id:            "registered-name",
		_expected:      registered,
		_expectedIsErr: false,

		name: "peek-mcp",
	})

	// unregistered-name-err
	tests = append(tests, &testCase{
		_id:            "unregistered-name-err",
		_expected:      "",
		_expectedIsErr: true,

		name: "nosuchrepo",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			repo, err := resolveRepo(mainCheckout, test.name, registryPath)
			assert.Equal(t, test._expected, repo)
			assert.Equal(t, test._expectedIsErr, errors.Is(err, ErrUnregisteredRepo))
		})
	}
}

func TestResolveRef(t *testing.T) {
	type testCase struct {
		_id            string
		_expected      string
		_expectedIsErr bool

		ref string
	}

	repo := newFixtureRepo(t)
	head := gitOutput(t, repo, "rev-parse", "HEAD")

	tests := make([]*testCase, 0)

	// empty-ref
	tests = append(tests, &testCase{
		_id:            "empty-ref",
		_expected:      "",
		_expectedIsErr: false,

		ref: "",
	})

	// short-sha-resolves
	tests = append(tests, &testCase{
		_id:            "short-sha-resolves",
		_expected:      head,
		_expectedIsErr: false,

		ref: head[:8],
	})

	// branch-name-resolves
	tests = append(tests, &testCase{
		_id:            "branch-name-resolves",
		_expected:      head,
		_expectedIsErr: false,

		ref: "main",
	})

	// unknown-ref-err
	tests = append(tests, &testCase{
		_id:            "unknown-ref-err",
		_expected:      "",
		_expectedIsErr: true,

		ref: "deadbeef",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			sha, err := resolveRef(context.Background(), test.ref, repo)
			assert.Equal(t, test._expected, sha)
			assert.Equal(t, test._expectedIsErr, errors.Is(err, ErrUnresolvableRef))
		})
	}
}

func TestProvision(t *testing.T) {
	type testCase struct {
		_id                     string
		_expectedArmFiles       []string
		_expectedArmWorktrees   []string
		_expectedAt             string
		_expectedHasPrebuildLog bool
		_expectedPrebuildLog    string
		_expectedReferencePath  bool
		_shouldPass             bool

		prebuildLog string
		runDir      string
	}

	repo := newFixtureRepo(t)
	first := gitOutput(t, repo, "rev-parse", "HEAD")
	writeTestFile(t, filepath.Join(repo, "file.txt"), "two\n")
	commitAll(t, repo, "second")
	second := gitOutput(t, repo, "rev-parse", "HEAD")
	wtRoot := newTempDir(t)
	t.Setenv(wtRootEnv, wtRoot)
	t.Chdir(repo)

	brokenOverlay := filepath.Join(newTempDir(t), "overlay.patch")
	writeTestFile(t, brokenOverlay, "diff --git a/missing.txt b/missing.txt\n--- a/missing.txt\n+++ b/missing.txt\n@@ -1 +1 @@\n-x\n+y\n")
	runOverlay := filepath.Join(newTempDir(t), "overlay.patch")
	writeTestFile(t, runOverlay, "diff --git a/file.txt b/file.txt\n--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-two\n+run\n")
	variantOverlay := filepath.Join(newTempDir(t), "overlay-variant.patch")
	writeTestFile(t, variantOverlay, "diff --git a/file.txt b/file.txt\n--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-two\n+variant\n")
	twoArms := []ManifestArm{{Label: "a"}, {Label: "b"}}

	tests := make([]*testCase, 0)

	// two-arms-worktrees-and-manifest-rewrite
	armsRun := filepath.Join(newTempDir(t), "case-arms")
	writeRunDir(t, armsRun, &RunManifest{Arms: twoArms, At: first[:8]})
	tests = append(tests, &testCase{
		_id:               "two-arms-worktrees-and-manifest-rewrite",
		_expectedArmFiles: []string{"one\n", "one\n"},
		_expectedArmWorktrees: []string{
			filepath.Join(wtRoot, armWorktreePrefix+"case-arms-a"),
			filepath.Join(wtRoot, armWorktreePrefix+"case-arms-b"),
		},
		_expectedAt:             first,
		_expectedHasPrebuildLog: false,
		_expectedPrebuildLog:    "",
		_expectedReferencePath:  false,
		_shouldPass:             true,

		prebuildLog: armPrebuildLog("case-arms", wtRoot),
		runDir:      armsRun,
	})

	// reference-patch-written
	referenceRun := filepath.Join(newTempDir(t), "case-reference")
	writeRunDir(t, referenceRun, &RunManifest{Arms: []ManifestArm{{Label: "a"}}, Reference: "main"})
	tests = append(tests, &testCase{
		_id:                     "reference-patch-written",
		_expectedArmFiles:       []string{"two\n"},
		_expectedArmWorktrees:   []string{filepath.Join(wtRoot, armWorktreePrefix+"case-reference-a")},
		_expectedAt:             second,
		_expectedHasPrebuildLog: false,
		_expectedPrebuildLog:    "",
		_expectedReferencePath:  true,
		_shouldPass:             true,

		prebuildLog: armPrebuildLog("case-reference", wtRoot),
		runDir:      referenceRun,
	})

	// no-reference-no-patch
	plainRun := filepath.Join(newTempDir(t), "case-plain")
	writeRunDir(t, plainRun, &RunManifest{Arms: []ManifestArm{{Label: "a"}}})
	tests = append(tests, &testCase{
		_id:                     "no-reference-no-patch",
		_expectedArmFiles:       []string{"two\n"},
		_expectedArmWorktrees:   []string{filepath.Join(wtRoot, armWorktreePrefix+"case-plain-a")},
		_expectedAt:             second,
		_expectedHasPrebuildLog: false,
		_expectedPrebuildLog:    "",
		_expectedReferencePath:  false,
		_shouldPass:             true,

		prebuildLog: armPrebuildLog("case-plain", wtRoot),
		runDir:      plainRun,
	})

	// overlay-fails-whole-run
	overlayRun := filepath.Join(newTempDir(t), "case-overlay")
	writeRunDir(t, overlayRun, &RunManifest{Arms: []ManifestArm{{Label: "a"}}, OverlayPatch: brokenOverlay})
	tests = append(tests, &testCase{
		_id:                     "overlay-fails-whole-run",
		_expectedArmFiles:       []string{""},
		_expectedArmWorktrees:   []string{""},
		_expectedAt:             "",
		_expectedHasPrebuildLog: false,
		_expectedPrebuildLog:    "",
		_expectedReferencePath:  false,
		_shouldPass:             false,

		prebuildLog: armPrebuildLog("case-overlay", wtRoot),
		runDir:      overlayRun,
	})

	// arm-overlay-override
	overrideArms := []ManifestArm{{Label: "a", OverlayPatch: variantOverlay}, {Label: "b"}}
	overrideRun := filepath.Join(newTempDir(t), "case-override")
	writeRunDir(t, overrideRun, &RunManifest{Arms: overrideArms, OverlayPatch: runOverlay})
	tests = append(tests, &testCase{
		_id:               "arm-overlay-override",
		_expectedArmFiles: []string{"variant\n", "run\n"},
		_expectedArmWorktrees: []string{
			filepath.Join(wtRoot, armWorktreePrefix+"case-override-a"),
			filepath.Join(wtRoot, armWorktreePrefix+"case-override-b"),
		},
		_expectedAt:             second,
		_expectedHasPrebuildLog: false,
		_expectedPrebuildLog:    "",
		_expectedReferencePath:  false,
		_shouldPass:             true,

		prebuildLog: armPrebuildLog("case-override", wtRoot),
		runDir:      overrideRun,
	})

	// prebuild-absent
	prebuildAbsentRun := filepath.Join(newTempDir(t), "case-prebuild-absent")
	writeRunDir(t, prebuildAbsentRun, &RunManifest{Arms: []ManifestArm{{Label: "a"}}, Prebuild: nil})
	tests = append(tests, &testCase{
		_id:                     "prebuild-absent",
		_expectedArmFiles:       []string{"two\n"},
		_expectedArmWorktrees:   []string{filepath.Join(wtRoot, armWorktreePrefix+"case-prebuild-absent-a")},
		_expectedAt:             second,
		_expectedHasPrebuildLog: false,
		_expectedPrebuildLog:    "",
		_expectedReferencePath:  false,
		_shouldPass:             true,

		prebuildLog: armPrebuildLog("case-prebuild-absent", wtRoot),
		runDir:      prebuildAbsentRun,
	})

	// prebuild-ok
	okPrebuild := &evals.DefinitionPrebuild{Command: []string{"go", "version"}, Outputs: []string{"file.txt"}}
	prebuildOkRun := filepath.Join(newTempDir(t), "case-prebuild-ok")
	writeRunDir(t, prebuildOkRun, &RunManifest{Arms: []ManifestArm{{Label: "a"}}, Prebuild: okPrebuild})
	tests = append(tests, &testCase{
		_id:                     "prebuild-ok",
		_expectedArmFiles:       []string{"two\n"},
		_expectedArmWorktrees:   []string{filepath.Join(wtRoot, armWorktreePrefix+"case-prebuild-ok-a")},
		_expectedAt:             second,
		_expectedHasPrebuildLog: true,
		_expectedPrebuildLog:    "",
		_expectedReferencePath:  false,
		_shouldPass:             true,

		prebuildLog: armPrebuildLog("case-prebuild-ok", wtRoot),
		runDir:      prebuildOkRun,
	})

	// prebuild-exit-nonzero
	failingPrebuild := &evals.DefinitionPrebuild{Command: []string{"false"}, Outputs: []string{}}
	prebuildFailRun := filepath.Join(newTempDir(t), "case-prebuild-fail")
	writeRunDir(t, prebuildFailRun, &RunManifest{Arms: []ManifestArm{{Label: "a"}}, Prebuild: failingPrebuild})
	tests = append(tests, &testCase{
		_id:                     "prebuild-exit-nonzero",
		_expectedArmFiles:       []string{""},
		_expectedArmWorktrees:   []string{""},
		_expectedAt:             "",
		_expectedHasPrebuildLog: true,
		_expectedPrebuildLog:    "",
		_expectedReferencePath:  false,
		_shouldPass:             false,

		prebuildLog: armPrebuildLog("case-prebuild-fail", wtRoot),
		runDir:      prebuildFailRun,
	})

	// prebuild-output-missing
	missingPrebuild := &evals.DefinitionPrebuild{Command: []string{"go", "version"}, Outputs: []string{"file.txt", "bin/acdsl"}}
	prebuildMissingRun := filepath.Join(newTempDir(t), "case-prebuild-missing")
	writeRunDir(t, prebuildMissingRun, &RunManifest{Arms: []ManifestArm{{Label: "a"}}, Prebuild: missingPrebuild})
	tests = append(tests, &testCase{
		_id:                     "prebuild-output-missing",
		_expectedArmFiles:       []string{""},
		_expectedArmWorktrees:   []string{""},
		_expectedAt:             "",
		_expectedHasPrebuildLog: true,
		_expectedPrebuildLog:    "",
		_expectedReferencePath:  false,
		_shouldPass:             false,

		prebuildLog: armPrebuildLog("case-prebuild-missing", wtRoot),
		runDir:      prebuildMissingRun,
	})

	// prebuild-log-written
	loggedPrebuild := &evals.DefinitionPrebuild{Command: []string{"go", "version"}, Outputs: []string{}}
	prebuildLogRun := filepath.Join(newTempDir(t), "case-prebuild-log")
	writeRunDir(t, prebuildLogRun, &RunManifest{Arms: []ManifestArm{{Label: "a"}}, Prebuild: loggedPrebuild})
	tests = append(tests, &testCase{
		_id:                     "prebuild-log-written",
		_expectedArmFiles:       []string{"two\n"},
		_expectedArmWorktrees:   []string{filepath.Join(wtRoot, armWorktreePrefix+"case-prebuild-log-a")},
		_expectedAt:             second,
		_expectedHasPrebuildLog: true,
		_expectedPrebuildLog:    "go version go",
		_expectedReferencePath:  false,
		_shouldPass:             true,

		prebuildLog: armPrebuildLog("case-prebuild-log", wtRoot),
		runDir:      prebuildLogRun,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := Provision(context.Background(), test.runDir)
			manifest, loadErr := loadManifest(test.runDir)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			require.NoError(t, loadErr)
			assert.Equal(t, test._expectedArmFiles, armFiles(manifest))
			assert.Equal(t, test._expectedArmWorktrees, armWorktrees(manifest))
			assert.Equal(t, test._expectedAt, manifest.At)
			assert.Equal(t, test._expectedHasPrebuildLog, pathExists(test.prebuildLog))
			assert.Contains(t, readOrEmpty(test.prebuildLog), test._expectedPrebuildLog)
			assert.NoFileExists(t, filepath.Join(test.runDir, runsDir, "a", prebuildLogFile))
			assert.Equal(t, test._expectedReferencePath, pathExists(filepath.Join(test.runDir, referencePatchFile)))
		})
	}
}

func TestProvision_LeavesOtherRuns(t *testing.T) {
	repo := newFixtureRepo(t)
	wtRoot := newTempDir(t)
	t.Setenv(wtRootEnv, wtRoot)
	t.Chdir(repo)
	hostName := armWorktreePrefix + "2026-10-02-210648-skillroutine-eval-a"
	hostWorktree := filepath.Join(wtRoot, hostName)
	hostBranch := armBranchPrefix + "2026-10-02-210648-skillroutine-eval-a"
	gitOutput(t, repo, "worktree", "add", "-q", "-b", hostBranch, hostWorktree)
	writeTestFile(t, filepath.Join(hostWorktree, "new.txt"), "uncommitted\n")
	hostArtifact := filepath.Join(hostWorktree+outDirSuffix, artifactsDir, "review.json")
	writeTestFile(t, hostArtifact, "{}\n")
	workerBranch := workerBranchPrefix + "2026-10-02-210648-skillroutine-eval-a-lane1"
	gitOutput(t, repo, "branch", workerBranch)
	runDir := filepath.Join(newTempDir(t), "2026-10-02-f1be0a")
	writeRunDir(t, runDir, &RunManifest{Arms: []ManifestArm{{Label: "a"}}})

	require.NoError(t, Provision(context.Background(), runDir))

	assert.Equal(t, "uncommitted\n", readTestFile(t, filepath.Join(hostWorktree, "new.txt")))
	assert.Equal(t, "{}\n", readTestFile(t, hostArtifact))
	assert.Equal(t, hostBranch, gitOutput(t, hostWorktree, "rev-parse", "--abbrev-ref", "HEAD"))
	assert.Contains(t, localBranches(t, repo), workerBranch)
	assert.Empty(t, listFiles(t, filepath.Join(wtRoot, ".stale-archive")))
}

// context.json is gitignored, so only the regeneration puts it into an arm worktree; the context dir is the test's own
// (one entry, one aspect), never the checkout's, whose aspects file the public tree lacks.
func TestProvision_RegeneratesContextIndex(t *testing.T) {
	repo := newFixtureRepo(t)
	contextDir := filepath.Join(repo, contextdocs.SourceContextDir)
	require.NoError(t, os.MkdirAll(filepath.Join(contextDir, "actions"), 0o755))
	entry := []byte("**ACTION-NAV-001** `[prose]` — Never scan.\n\n* Applies: everywhere.\n")
	require.NoError(t, os.WriteFile(filepath.Join(contextDir, "actions", "nav.md"), entry, 0o644))
	aspects := []contextdocs.RuleAspect{{Name: "NAV", Scope: "Navigation", Class: "scope"}}
	require.NoError(t, contextdocs.SaveAspects(contextDir, aspects))
	commitAll(t, repo, "context")
	wtRoot := newTempDir(t)
	t.Setenv(wtRootEnv, wtRoot)
	t.Chdir(repo)
	runDir := filepath.Join(newTempDir(t), "case-context")
	writeRunDir(t, runDir, &RunManifest{Arms: []ManifestArm{{Label: "a"}}})

	require.NoError(t, Provision(context.Background(), runDir))

	armContext := filepath.Join(wtRoot, armWorktreePrefix+"case-context-a", contextdocs.SourceContextDir)
	assert.FileExists(t, filepath.Join(armContext, contextdocs.ContextFileName))
}

func TestCheckModels(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool

		manifest *RunManifest
	}

	tests := make([]*testCase, 0)

	// resolving-arms-and-judge
	tests = append(tests, &testCase{
		_id:         "resolving-arms-and-judge",
		_shouldPass: true,

		manifest: &RunManifest{Arms: []ManifestArm{{Label: "A", Model: "opus"}, {Label: "B", Model: "claude-opus-5-5[1m]"}}, Judge: "opus"},
	})

	// legacy-arm-token
	tests = append(tests, &testCase{
		_id:         "legacy-arm-token",
		_shouldPass: false,

		manifest: &RunManifest{Arms: []ManifestArm{{Label: "A", Model: "opus-high"}}},
	})

	// forbidden-judge
	tests = append(tests, &testCase{
		_id:         "forbidden-judge",
		_shouldPass: false,

		manifest: &RunManifest{Arms: []ManifestArm{{Label: "A", Model: "opus"}}, Judge: "fable"},
	})

	// no-judge
	tests = append(tests, &testCase{
		_id:         "no-judge",
		_shouldPass: true,

		manifest: &RunManifest{Arms: []ManifestArm{{Label: "A", Model: ""}}},
	})

	home := t.TempDir()
	claudeHome := filepath.Join(home, ".claude")
	require.NoError(t, os.MkdirAll(filepath.Join(claudeHome, "heartbeat"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(claudeHome, "settings.json"), []byte(`{"availableModels": ["claude-opus-5-5", "claude-opus-5-5[1m]", "claude-fable-5", "claude-sonnet-5"]}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(claudeHome, "heartbeat", "config.json"), []byte(`{"forbidden_subagent_models": ["*fable*"]}`), 0o600))
	t.Setenv("HOME", home)

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := checkModels(test.manifest)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
		})
	}
}

// armFiles lists each arm worktree's file.txt content in arm order; empty for
// an arm without a worktree.
func armFiles(manifest *RunManifest) []string {
	contents := make([]string, 0, len(manifest.Arms))
	for _, arm := range manifest.Arms {
		content, err := os.ReadFile(filepath.Join(arm.Worktree, "file.txt"))
		if arm.Worktree == "" || err != nil {
			content = nil
		}
		contents = append(contents, string(content))
	}
	return contents
}

// armPrebuildLog is where provision logs arm a's prebuild for the run named
// runName under wtRoot: beside the arm worktree, in its -out dir.
func armPrebuildLog(runName, wtRoot string) string {
	return filepath.Join(wtRoot, armWorktreePrefix+runName+"-a"+outDirSuffix, prebuildLogFile)
}

// armWorktrees lists the manifest's arm worktree paths in arm order.
func armWorktrees(manifest *RunManifest) []string {
	paths := make([]string, 0, len(manifest.Arms))
	for _, arm := range manifest.Arms {
		paths = append(paths, arm.Worktree)
	}
	return paths
}

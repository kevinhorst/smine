package skilleval

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/heartbeat"
	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skills"
)

const (
	fakeSmokeClaudeScript = `#!/bin/sh
echo '{"type":"system","subtype":"init","session_id":"s-smoke","model":"claude-sonnet-5","tools":["Read"],"mcp_servers":[]}'
echo '{"type":"result","subtype":"success","is_error":false,"session_id":"s-smoke","terminal_reason":"completed","result":"ok","total_cost_usd":'"${SMOKE_FAKE_COST:-0.5}"',"modelUsage":{"claude-sonnet-5":{"outputTokens":10}},"permission_denials":[]}'
`
	fakeSmokeCostEnv = "SMOKE_FAKE_COST"
	smokeNewPatch    = "diff --git a/notes/new.md b/notes/new.md\nnew file mode 100644\n--- /dev/null\n+++ b/notes/new.md\n@@ -0,0 +1 @@\n+new\n"
	smokeTestLeaf    = "heartbeat-demo"
	smokeTestVersion = "2.1.2"
	smokeTokenLabel  = "personal"
	untrackedCheck   = `test -f notes/plan.md && test -f notes/new.md && test -z "$(git ls-files notes/plan.md notes/new.md)"`
)

// loadedRecordView is the part of a loaded smoke record a test compares: HasRuntime tells a missing runtime.json from an empty one.
type loadedRecordView struct {
	Coverage   SmokeCoverage
	Dir        string
	HasRuntime bool
	Prompt     string
	RunAnchors []string
	TokenLabel string
	Verdict    string
}

// smokeFixture is one smoke case's world: a HOME holding the model settings,
// the heartbeat config, the token and the deployed heartbeat-demo, a checkout
// holding heartbeat-demo with its smoke fixtures, its own worktree root and
// the fake claude's cost ("" for the script's default).
type smokeFixture struct {
	costUsd string
	home    string
	repo    string
	wtRoot  string
}

// newSmokeFixture builds a case's world from the repo's skills root: the
// checkout gets the vocabulary and heartbeat-demo pinned at the fixture
// version, document's smoke as the smoke block of its model file (none when
// nil) and the tree and new-file patch fixtures; HOME gets the deployed copy the deploy renders from that
// checkout.
func newSmokeFixture(t *testing.T, document *skillgraph.SmokeDocument, skillsSource string) *smokeFixture {
	t.Helper()
	fixture := &smokeFixture{home: newTempDir(t), repo: newFixtureRepo(t), wtRoot: newTempDir(t)}
	claudeHome := filepath.Join(fixture.home, ".claude")
	writeTestFile(t, filepath.Join(claudeHome, "settings.json"), `{"availableModels": ["claude-sonnet-5"]}`)
	writeTestFile(t, filepath.Join(claudeHome, "heartbeat", "config.json"), `{"forbidden_subagent_models": []}`)
	writeTestFile(t, filepath.Join(fixture.home, ".config", "claude-routine", "tokens", smokeTokenLabel), "stored-token\n")

	skillDir := fixture.skillDir()
	require.NoError(t, os.CopyFS(skillDir, os.DirFS(filepath.Join(skillsSource, "orchestration", smokeTestLeaf))))
	require.NoError(t, copyFile(filepath.Join(skillsSource, skillgraph.FileVocabulary), filepath.Join(fixture.repo, smokeSkillsDir, skillgraph.FileVocabulary)))
	require.NoError(t, copyFile(filepath.Join(skillsSource, skillgraph.FileCatalog), filepath.Join(fixture.repo, smokeSkillsDir, skillgraph.FileCatalog)))
	writeTestFile(t, filepath.Join(skillDir, skillgraph.DirSmoke, "tree", "notes", "plan.md"), "plan\n")
	writeTestFile(t, filepath.Join(skillDir, skillgraph.DirSmoke, "new.patch"), smokeNewPatch)
	skill, err := skillgraph.Load(skillDir)
	require.NoError(t, err)
	pinned := slices.IndexFunc(skill.Changelog, func(row skillgraph.ChangelogEntry) bool { return row.Version == smokeTestVersion })
	require.GreaterOrEqual(t, pinned, 0, "heartbeat-demo's changelog holds the fixture version")
	skill.Changelog = skill.Changelog[pinned:]
	for i := range skill.Frontmatter {
		if skill.Frontmatter[i].Key == "version" {
			skill.Frontmatter[i].Value = " " + smokeTestVersion
		}
	}
	skill.Smoke = nil
	if document != nil {
		skill.Smoke = document.Smoke
	}
	require.NoError(t, skillgraph.Save(skillDir, skill))
	commitAll(t, fixture.repo, "skills")

	// The smine-rules provider fills nothing on Windows, so the environment fills heartbeat-demo's two required capabilities on every OS; the stub makes the value resolve and answers the handshake.
	installRulesStub(t)
	t.Setenv("SKILL_HEARTBEATDEMO_DEMO_CLOSE", "smine-rules")
	t.Setenv("SKILL_HEARTBEATDEMO_DEMO_VALUE", "smine-rules")
	options := &skills.SyncOptions{Home: fixture.home, RepoDir: fixture.repo}
	_, err = skills.DeployLeaf(smokeTestLeaf, options)
	require.NoError(t, err)
	return fixture
}

// installRulesStub puts a smine-rules ahead of PATH that answers the handshake for heartbeat-demo's two capabilities.
func installRulesStub(t *testing.T) {
	t.Helper()
	dir := newTempDir(t)
	answer := `{"interfaces":["demo-close@1","demo-value@1"]}`
	name, script := "smine-rules", "#!/bin/sh\necho '"+answer+"'\n"
	if runtime.GOOS == "windows" {
		name, script = "smine-rules.bat", "@echo "+answer+"\r\n"
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// deployedDir is the deployed heartbeat-demo under the fixture's HOME.
func (f *smokeFixture) deployedDir() string {
	return filepath.Join(f.home, ".claude", "skills", smokeTestLeaf)
}

// enter makes the fixture the process world for one subtest: HOME, the
// worktree root, the fake claude's cost and the cwd.
func (f *smokeFixture) enter(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", f.home)
	t.Setenv(wtRootEnv, f.wtRoot)
	t.Setenv(fakeSmokeCostEnv, f.costUsd)
	t.Chdir(f.repo)
}

// skillDir is heartbeat-demo in the fixture's checkout.
func (f *smokeFixture) skillDir() string {
	return filepath.Join(f.repo, smokeSkillsDir, "orchestration", smokeTestLeaf)
}

// smokeRecordView is the part of a published smoke record a test compares:
// result.json's identity and verdict, run.json's prompt.
type smokeRecordView struct {
	Layer        string
	Mode         string
	Prompt       string
	SkillVersion string
	Verdict      string
}

// armLeftovers counts the arm worktrees and branches left in repo beside the
// main checkout and branch.
func armLeftovers(t *testing.T, repo string) int {
	t.Helper()
	worktrees := strings.Count(gitOutput(t, repo, "worktree", "list", "--porcelain"), "worktree ")
	return worktrees - 1 + len(localBranches(t, repo)) - 1
}

// errorText is err's message, "" for nil.
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// failingRuntimeReader is a heartbeat that does not answer.
func failingRuntimeReader(context.Context) (*heartbeat.RuntimeDocument, error) {
	return nil, errors.New("connection refused")
}

// interactiveSmokeDocument is heartbeat-demo's interactive smoke judged by
// the given anchors.
func interactiveSmokeDocument(anchors []string) *skillgraph.SmokeDocument {
	document := provideSmokeDocument(anchors, nil)
	document.Mode = skillgraph.SmokeModeInteractive
	return document
}

// nightlySmokeDocument is heartbeat-demo's nightly smoke on the tree and
// new-file patch fixtures.
func nightlySmokeDocument(anchors, check []string) *skillgraph.SmokeDocument {
	document := provideSmokeDocument(anchors, check)
	document.Fixture = &skillgraph.SmokeFixture{Patch: "new.patch", Tree: "tree"}
	return document
}

// provideSmokeManifest is run.json of heartbeat-demo's default nightly smoke
// at abc123 billing the personal token, dated on the synctest epoch.
func provideSmokeManifest() *RunManifest {
	arm := ManifestArm{Id: "A", Effort: "low", Key: "A", Label: "A", Model: "sonnet", Replica: 1}
	syncNow := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	manifest := &RunManifest{
		Arms:          []ManifestArm{arm},
		At:            "abc123",
		BudgetUsd:     3,
		CheckTimeoutS: 900,
		Date:          syncNow.In(time.Local).Format(time.DateOnly),
		Layer:         LayerSmoke,
		Mode:          ModeNightly,
		Prompt:        "/heartbeat-demo pass-clean-run",
		Skill:         smokeTestLeaf,
		SkillVersion:  smokeTestVersion,
		TimeoutS:      900,
		TokenLabel:    smokeTokenLabel,
	}
	return manifest
}

// readSmokeRecord reads heartbeat-demo's published 2.1.2 record under home;
// nil without one.
func readSmokeRecord(t *testing.T, home string) *smokeRecordView {
	t.Helper()
	versionDir := filepath.Join(home, smokeRootDir, smokeTestLeaf, smokeTestVersion)
	resultPath := filepath.Join(versionDir, runsDir, smokeArmLabel, resultFile)
	if !pathExists(resultPath) {
		return nil
	}

	var result ArmResult
	require.NoError(t, readJsonFile(resultPath, &result))
	manifest, err := loadManifest(versionDir)
	require.NoError(t, err)
	require.NotNil(t, result.Smoke)
	view := &smokeRecordView{
		Layer:        result.Layer,
		Mode:         result.Mode,
		Prompt:       manifest.Prompt,
		SkillVersion: result.SkillVersion,
		Verdict:      result.Smoke.Verdict,
	}
	return view
}

// smokeRuntimeReader is a heartbeat answering with runs.
func smokeRuntimeReader(runs ...heartbeat.Run) RuntimeReader {
	return func(context.Context) (*heartbeat.RuntimeDocument, error) {
		return &heartbeat.RuntimeDocument{Runs: runs}, nil
	}
}

// stagingEntries counts the run dirs left in the smoke root's staging dir
// under home.
func stagingEntries(t *testing.T, home string) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(home, smokeRootDir, smokeStagingDir))
	if errors.Is(err, fs.ErrNotExist) {
		return 0
	}

	require.NoError(t, err)
	return len(entries)
}

// viewLoadedRecord is record's compared part; nil without one.
func viewLoadedRecord(record *SmokeRecord) *loadedRecordView {
	if record == nil {
		return nil
	}

	view := &loadedRecordView{
		Coverage:   record.Result.Smoke.Coverage,
		Dir:        record.Dir,
		HasRuntime: record.Runtime != nil,
		Prompt:     record.Manifest.Prompt,
		TokenLabel: record.Manifest.TokenLabel,
		Verdict:    record.Result.Smoke.Verdict,
	}
	if record.Runtime == nil {
		return view
	}

	for _, run := range record.Runtime.Runs {
		for _, anchor := range run.Anchors {
			view.RunAnchors = append(view.RunAnchors, anchor.Id)
		}
	}
	return view
}

// writeSmokeRecord writes record as a result.json at path, dirs included.
func writeSmokeRecord(t *testing.T, path string, record *ArmResult) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, writeJsonFile(path, record))
}

func TestRunSmoke(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedAside    []string
		_expectedErrPart  string
		_expectedIsDiffer bool
		_expectedLeftover int
		_expectedOutcome  *SmokeOutcome
		_expectedRecord   *smokeRecordView
		_expectedStaging  int
		_shouldPass       bool

		fixture     *smokeFixture
		leaf        string
		readRuntime RuntimeReader
		tokenLabel  string
	}

	skillsSource, err := filepath.Abs(filepath.Join("..", "..", smokeSkillsDir))
	require.NoError(t, err)
	binDir := newTempDir(t)
	writeTestFile(t, filepath.Join(binDir, "claude"), fakeSmokeClaudeScript)
	require.NoError(t, os.Chmod(filepath.Join(binDir, "claude"), 0o755))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv(config.ReadFenceEnv, "")
	t.Setenv(config.StoreRootEnv, "")
	t.Setenv(oauthTokenEnv, "")
	heartbeatRun := provideSmokeRun()
	heartbeatRun.Session = "s-smoke"
	otherSkillRun := provideSmokeRun()
	otherSkillRun.Session = "s-smoke"
	otherSkillRun.Skill = "fdesign"
	answering := smokeRuntimeReader(heartbeatRun)
	tests := make([]*testCase, 0)

	// fail-unknown-leaf
	tests = append(tests, &testCase{
		_id:               "fail-unknown-leaf",
		_expectedAside:    []string{},
		_expectedErrPart:  "No skill no-such-skill under",
		_expectedIsDiffer: false,
		_expectedLeftover: 0,
		_expectedOutcome:  nil,
		_expectedRecord:   nil,
		_expectedStaging:  0,
		_shouldPass:       false,

		fixture:     newSmokeFixture(t, nightlySmokeDocument(nil, nil), skillsSource),
		leaf:        "no-such-skill",
		readRuntime: answering,
		tokenLabel:  smokeTokenLabel,
	})

	// fail-no-smoke-json
	tests = append(tests, &testCase{
		_id:               "fail-no-smoke-json",
		_expectedAside:    []string{},
		_expectedErrPart:  "declares no smoke",
		_expectedIsDiffer: false,
		_expectedLeftover: 0,
		_expectedOutcome:  nil,
		_expectedRecord:   nil,
		_expectedStaging:  0,
		_shouldPass:       false,

		fixture:     newSmokeFixture(t, nil, skillsSource),
		leaf:        smokeTestLeaf,
		readRuntime: answering,
		tokenLabel:  smokeTokenLabel,
	})

	// pass-skip
	skip := &skillgraph.SmokeDocument{Smoke: &skillgraph.Smoke{Skip: "no dummy input"}, Schema: 1, Skill: smokeTestLeaf, Version: smokeTestVersion}
	tests = append(tests, &testCase{
		_id:               "pass-skip",
		_expectedAside:    []string{},
		_expectedErrPart:  "",
		_expectedIsDiffer: false,
		_expectedLeftover: 0,
		_expectedOutcome:  &SmokeOutcome{IsPass: true, Lines: []string{"smoke heartbeat-demo 2.1.2: skipped — no dummy input"}},
		_expectedRecord:   nil,
		_expectedStaging:  0,
		_shouldPass:       true,

		fixture:     newSmokeFixture(t, skip, skillsSource),
		leaf:        smokeTestLeaf,
		readRuntime: answering,
		tokenLabel:  smokeTokenLabel,
	})

	// fail-deployed-stale-source
	staleSource := newSmokeFixture(t, nightlySmokeDocument(nil, nil), skillsSource)
	writeTestFile(t, filepath.Join(staleSource.skillDir(), "README.md"), "edited after the deploy\n")
	tests = append(tests, &testCase{
		_id:               "fail-deployed-stale-source",
		_expectedAside:    []string{},
		_expectedErrPart:  staleSource.deployedDir(),
		_expectedIsDiffer: true,
		_expectedLeftover: 0,
		_expectedOutcome:  nil,
		_expectedRecord:   nil,
		_expectedStaging:  0,
		_shouldPass:       false,

		fixture:     staleSource,
		leaf:        smokeTestLeaf,
		readRuntime: answering,
		tokenLabel:  smokeTokenLabel,
	})

	// fail-deployed-marker-missing
	markerMissing := newSmokeFixture(t, nightlySmokeDocument(nil, nil), skillsSource)
	require.NoError(t, os.Remove(filepath.Join(markerMissing.deployedDir(), ".smine-deployed")))
	tests = append(tests, &testCase{
		_id:               "fail-deployed-marker-missing",
		_expectedAside:    []string{},
		_expectedErrPart:  markerMissing.deployedDir(),
		_expectedIsDiffer: true,
		_expectedLeftover: 0,
		_expectedOutcome:  nil,
		_expectedRecord:   nil,
		_expectedStaging:  0,
		_shouldPass:       false,

		fixture:     markerMissing,
		leaf:        smokeTestLeaf,
		readRuntime: answering,
		tokenLabel:  smokeTokenLabel,
	})

	// pass-interactive
	interactiveLines := []string{
		"smoke heartbeat-demo 2.1.2: interactive — run /heartbeat-demo pass-clean-run in a session",
		"then: make skill-smoke-verdict SKILL=heartbeat-demo SESSION=<session id>",
	}
	tests = append(tests, &testCase{
		_id:               "pass-interactive",
		_expectedAside:    []string{},
		_expectedErrPart:  "",
		_expectedIsDiffer: false,
		_expectedLeftover: 0,
		_expectedOutcome:  &SmokeOutcome{IsPass: true, Lines: interactiveLines},
		_expectedRecord:   nil,
		_expectedStaging:  0,
		_shouldPass:       true,

		fixture:     newSmokeFixture(t, interactiveSmokeDocument([]string{"produced", "closed"}), skillsSource),
		leaf:        smokeTestLeaf,
		readRuntime: answering,
		tokenLabel:  smokeTokenLabel,
	})

	// pass-empty-invoke
	emptyInvoke := nightlySmokeDocument(nil, nil)
	empty := ""
	emptyInvoke.Invoke = &empty
	tests = append(tests, &testCase{
		_id:               "pass-empty-invoke",
		_expectedAside:    []string{},
		_expectedErrPart:  "",
		_expectedIsDiffer: false,
		_expectedLeftover: 0,
		_expectedOutcome:  &SmokeOutcome{IsPass: true, Lines: []string{"smoke heartbeat-demo 2.1.2: pass"}},
		_expectedRecord:   &smokeRecordView{Layer: LayerSmoke, Mode: ModeNightly, Prompt: "/heartbeat-demo", SkillVersion: smokeTestVersion, Verdict: statusPass},
		_expectedStaging:  0,
		_shouldPass:       true,

		fixture:     newSmokeFixture(t, emptyInvoke, skillsSource),
		leaf:        smokeTestLeaf,
		readRuntime: answering,
		tokenLabel:  smokeTokenLabel,
	})

	// pass-nightly-terminal
	tests = append(tests, &testCase{
		_id:               "pass-nightly-terminal",
		_expectedAside:    []string{},
		_expectedErrPart:  "",
		_expectedIsDiffer: false,
		_expectedLeftover: 0,
		_expectedOutcome:  &SmokeOutcome{IsPass: true, Lines: []string{"smoke heartbeat-demo 2.1.2: pass"}},
		_expectedRecord:   &smokeRecordView{Layer: LayerSmoke, Mode: ModeNightly, Prompt: "/heartbeat-demo pass-clean-run", SkillVersion: smokeTestVersion, Verdict: statusPass},
		_expectedStaging:  0,
		_shouldPass:       true,

		fixture:     newSmokeFixture(t, nightlySmokeDocument(nil, []string{"sh", "-c", untrackedCheck}), skillsSource),
		leaf:        smokeTestLeaf,
		readRuntime: answering,
		tokenLabel:  smokeTokenLabel,
	})

	// fail-nightly-check
	tests = append(tests, &testCase{
		_id:               "fail-nightly-check",
		_expectedAside:    []string{},
		_expectedErrPart:  "",
		_expectedIsDiffer: false,
		_expectedLeftover: 0,
		_expectedOutcome:  &SmokeOutcome{IsPass: false, Lines: []string{"smoke heartbeat-demo 2.1.2: fail — check exited 3"}},
		_expectedRecord:   &smokeRecordView{Layer: LayerSmoke, Mode: ModeNightly, Prompt: "/heartbeat-demo pass-clean-run", SkillVersion: smokeTestVersion, Verdict: statusFail},
		_expectedStaging:  0,
		_shouldPass:       true,

		fixture:     newSmokeFixture(t, nightlySmokeDocument(nil, []string{"sh", "-c", "exit 3"}), skillsSource),
		leaf:        smokeTestLeaf,
		readRuntime: answering,
		tokenLabel:  smokeTokenLabel,
	})

	// fail-nightly-over-budget
	overBudget := newSmokeFixture(t, nightlySmokeDocument(nil, nil), skillsSource)
	overBudget.costUsd = "5"
	tests = append(tests, &testCase{
		_id:               "fail-nightly-over-budget",
		_expectedAside:    []string{},
		_expectedErrPart:  "",
		_expectedIsDiffer: false,
		_expectedLeftover: 0,
		_expectedOutcome:  &SmokeOutcome{IsPass: false, Lines: []string{"smoke heartbeat-demo 2.1.2: fail — cost 5.00 USD reached the 3 USD limit"}},
		_expectedRecord:   &smokeRecordView{Layer: LayerSmoke, Mode: ModeNightly, Prompt: "/heartbeat-demo pass-clean-run", SkillVersion: smokeTestVersion, Verdict: statusFail},
		_expectedStaging:  0,
		_shouldPass:       true,

		fixture:     overBudget,
		leaf:        smokeTestLeaf,
		readRuntime: answering,
		tokenLabel:  smokeTokenLabel,
	})

	// fail-nightly-heartbeat-down
	heartbeatDownLines := []string{
		"smoke heartbeat-demo 2.1.2: fail — no heartbeat run row — heartbeat did not answer: connection refused",
		"coverage: anchors 0/2 (4), checks 0/4",
	}
	tests = append(tests, &testCase{
		_id:               "fail-nightly-heartbeat-down",
		_expectedAside:    []string{},
		_expectedErrPart:  "",
		_expectedIsDiffer: false,
		_expectedLeftover: 0,
		_expectedOutcome:  &SmokeOutcome{IsPass: false, Lines: heartbeatDownLines},
		_expectedRecord:   &smokeRecordView{Layer: LayerSmoke, Mode: ModeNightly, Prompt: "/heartbeat-demo pass-clean-run", SkillVersion: smokeTestVersion, Verdict: statusFail},
		_expectedStaging:  0,
		_shouldPass:       true,

		fixture:     newSmokeFixture(t, nightlySmokeDocument([]string{"produced", "closed"}, nil), skillsSource),
		leaf:        smokeTestLeaf,
		readRuntime: failingRuntimeReader,
		tokenLabel:  smokeTokenLabel,
	})

	// fail-nightly-no-run-row
	noRunRowLines := []string{
		"smoke heartbeat-demo 2.1.2: fail — no heartbeat run row — no enforced run of heartbeat-demo in the session",
		"coverage: anchors 0/2 (4), checks 0/4",
	}
	tests = append(tests, &testCase{
		_id:               "fail-nightly-no-run-row",
		_expectedAside:    []string{},
		_expectedErrPart:  "",
		_expectedIsDiffer: false,
		_expectedLeftover: 0,
		_expectedOutcome:  &SmokeOutcome{IsPass: false, Lines: noRunRowLines},
		_expectedRecord:   &smokeRecordView{Layer: LayerSmoke, Mode: ModeNightly, Prompt: "/heartbeat-demo pass-clean-run", SkillVersion: smokeTestVersion, Verdict: statusFail},
		_expectedStaging:  0,
		_shouldPass:       true,

		fixture:     newSmokeFixture(t, nightlySmokeDocument([]string{"produced", "closed"}, nil), skillsSource),
		leaf:        smokeTestLeaf,
		readRuntime: smokeRuntimeReader(otherSkillRun),
		tokenLabel:  smokeTokenLabel,
	})

	// pass-nightly-heartbeat
	heartbeatLines := []string{"smoke heartbeat-demo 2.1.2: pass", "coverage: anchors 2/2 (4), checks 2/4"}
	tests = append(tests, &testCase{
		_id:               "pass-nightly-heartbeat",
		_expectedAside:    []string{},
		_expectedErrPart:  "",
		_expectedIsDiffer: false,
		_expectedLeftover: 0,
		_expectedOutcome:  &SmokeOutcome{IsPass: true, Lines: heartbeatLines},
		_expectedRecord:   &smokeRecordView{Layer: LayerSmoke, Mode: ModeNightly, Prompt: "/heartbeat-demo pass-clean-run", SkillVersion: smokeTestVersion, Verdict: statusPass},
		_expectedStaging:  0,
		_shouldPass:       true,

		fixture:     newSmokeFixture(t, nightlySmokeDocument([]string{"produced", "closed"}, nil), skillsSource),
		leaf:        smokeTestLeaf,
		readRuntime: answering,
		tokenLabel:  smokeTokenLabel,
	})

	// second-smoke-keeps-first-as-dot-one
	secondSmoke := newSmokeFixture(t, nightlySmokeDocument([]string{"produced", "closed"}, nil), skillsSource)
	writeTestFile(t, filepath.Join(secondSmoke.home, smokeRootDir, smokeTestLeaf, smokeTestVersion, "first.txt"), "first\n")
	tests = append(tests, &testCase{
		_id:               "second-smoke-keeps-first-as-dot-one",
		_expectedAside:    []string{"first.txt"},
		_expectedErrPart:  "",
		_expectedIsDiffer: false,
		_expectedLeftover: 0,
		_expectedOutcome:  &SmokeOutcome{IsPass: true, Lines: heartbeatLines},
		_expectedRecord:   &smokeRecordView{Layer: LayerSmoke, Mode: ModeNightly, Prompt: "/heartbeat-demo pass-clean-run", SkillVersion: smokeTestVersion, Verdict: statusPass},
		_expectedStaging:  0,
		_shouldPass:       true,

		fixture:     secondSmoke,
		leaf:        smokeTestLeaf,
		readRuntime: answering,
		tokenLabel:  smokeTokenLabel,
	})

	// fail-nightly-no-token
	tests = append(tests, &testCase{
		_id:               "fail-nightly-no-token",
		_expectedAside:    []string{},
		_expectedErrPart:  "Missing token label",
		_expectedIsDiffer: false,
		_expectedLeftover: 0,
		_expectedOutcome:  nil,
		_expectedRecord:   nil,
		_expectedStaging:  0,
		_shouldPass:       false,

		fixture:     newSmokeFixture(t, nightlySmokeDocument(nil, nil), skillsSource),
		leaf:        smokeTestLeaf,
		readRuntime: answering,
		tokenLabel:  "",
	})

	// fail-fixture-tree-overlaps
	overlaps := newSmokeFixture(t, nightlySmokeDocument(nil, nil), skillsSource)
	writeTestFile(t, filepath.Join(overlaps.skillDir(), skillgraph.DirSmoke, "tree", "file.txt"), "fixture\n")
	tests = append(tests, &testCase{
		_id:               "fail-fixture-tree-overlaps",
		_expectedAside:    []string{},
		_expectedErrPart:  "placeFixtureTree",
		_expectedIsDiffer: false,
		_expectedLeftover: 2,
		_expectedOutcome:  nil,
		_expectedRecord:   nil,
		_expectedStaging:  1,
		_shouldPass:       false,

		fixture:     overlaps,
		leaf:        smokeTestLeaf,
		readRuntime: answering,
		tokenLabel:  smokeTokenLabel,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			test.fixture.enter(t)
			outcome, err := RunSmoke(context.Background(), test.leaf, test.readRuntime, test.tokenLabel)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedIsDiffer, errors.Is(err, ErrDeployedDiffers))
			assert.Contains(t, errorText(err), test._expectedErrPart)
			assert.Equal(t, test._expectedOutcome, outcome)
			assert.Equal(t, test._expectedRecord, readSmokeRecord(t, test.fixture.home))
			assert.Equal(t, test._expectedLeftover, armLeftovers(t, test.fixture.repo))
			assert.Equal(t, test._expectedStaging, stagingEntries(t, test.fixture.home))
			assert.Equal(t, test._expectedAside, listFiles(t, filepath.Join(test.fixture.home, smokeRootDir, smokeTestLeaf, smokeTestVersion+".1")))
		})
	}
}

func TestJudgeSmokeSession(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedErrPart string
		_expectedOutcome *SmokeOutcome
		_expectedRecord  *smokeRecordView
		_shouldPass      bool

		fixture     *smokeFixture
		readRuntime RuntimeReader
	}

	skillsSource, err := filepath.Abs(filepath.Join("..", "..", smokeSkillsDir))
	require.NoError(t, err)
	anchored := interactiveSmokeDocument([]string{"produced", "closed"})
	open := provideSmokeRun()
	open.State = "open"
	tests := make([]*testCase, 0)

	// pass-ended
	endedLines := []string{"smoke heartbeat-demo 2.1.2: pass", "coverage: anchors 2/2 (4), checks 2/4"}
	tests = append(tests, &testCase{
		_id:              "pass-ended",
		_expectedErrPart: "",
		_expectedOutcome: &SmokeOutcome{IsPass: true, Lines: endedLines},
		_expectedRecord:  &smokeRecordView{Layer: LayerSmoke, Mode: ModeInteractive, Prompt: "/heartbeat-demo pass-clean-run", SkillVersion: smokeTestVersion, Verdict: statusPass},
		_shouldPass:      true,

		fixture:     newSmokeFixture(t, anchored, skillsSource),
		readRuntime: smokeRuntimeReader(provideSmokeRun()),
	})

	// fail-still-open
	openLines := []string{"smoke heartbeat-demo 2.1.2: fail — run 7 still open — the session has not ended", "coverage: anchors 2/2 (4), checks 2/4"}
	tests = append(tests, &testCase{
		_id:              "fail-still-open",
		_expectedErrPart: "",
		_expectedOutcome: &SmokeOutcome{IsPass: false, Lines: openLines},
		_expectedRecord:  &smokeRecordView{Layer: LayerSmoke, Mode: ModeInteractive, Prompt: "/heartbeat-demo pass-clean-run", SkillVersion: smokeTestVersion, Verdict: statusFail},
		_shouldPass:      true,

		fixture:     newSmokeFixture(t, anchored, skillsSource),
		readRuntime: smokeRuntimeReader(open),
	})

	// fail-heartbeat-down
	downLines := []string{
		"smoke heartbeat-demo 2.1.2: fail — no heartbeat run row — heartbeat did not answer: connection refused",
		"coverage: anchors 0/2 (4), checks 0/4",
	}
	tests = append(tests, &testCase{
		_id:              "fail-heartbeat-down",
		_expectedErrPart: "",
		_expectedOutcome: &SmokeOutcome{IsPass: false, Lines: downLines},
		_expectedRecord:  &smokeRecordView{Layer: LayerSmoke, Mode: ModeInteractive, Prompt: "/heartbeat-demo pass-clean-run", SkillVersion: smokeTestVersion, Verdict: statusFail},
		_shouldPass:      true,

		fixture:     newSmokeFixture(t, anchored, skillsSource),
		readRuntime: failingRuntimeReader,
	})

	// fail-check-form
	tests = append(tests, &testCase{
		_id:              "fail-check-form",
		_expectedErrPart: "expects no anchors",
		_expectedOutcome: nil,
		_expectedRecord:  nil,
		_shouldPass:      false,

		fixture:     newSmokeFixture(t, nightlySmokeDocument(nil, []string{"make", "audit"}), skillsSource),
		readRuntime: smokeRuntimeReader(provideSmokeRun()),
	})

	// fail-skip-form
	skip := &skillgraph.SmokeDocument{Smoke: &skillgraph.Smoke{Skip: "no dummy input"}, Schema: 1, Skill: smokeTestLeaf, Version: smokeTestVersion}
	tests = append(tests, &testCase{
		_id:              "fail-skip-form",
		_expectedErrPart: "expects no anchors",
		_expectedOutcome: nil,
		_expectedRecord:  nil,
		_shouldPass:      false,

		fixture:     newSmokeFixture(t, skip, skillsSource),
		readRuntime: smokeRuntimeReader(provideSmokeRun()),
	})

	// fail-no-smoke-json
	tests = append(tests, &testCase{
		_id:              "fail-no-smoke-json",
		_expectedErrPart: "declares no smoke",
		_expectedOutcome: nil,
		_expectedRecord:  nil,
		_shouldPass:      false,

		fixture:     newSmokeFixture(t, nil, skillsSource),
		readRuntime: smokeRuntimeReader(provideSmokeRun()),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			test.fixture.enter(t)
			outcome, err := JudgeSmokeSession(context.Background(), smokeTestLeaf, test.readRuntime, "s-1")
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Contains(t, errorText(err), test._expectedErrPart)
			assert.Equal(t, test._expectedOutcome, outcome)
			assert.Equal(t, test._expectedRecord, readSmokeRecord(t, test.fixture.home))
		})
	}
}

func TestLastSmoke(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *ArmResult
		_shouldPass bool

		root string
	}

	older := &ArmResult{EndedAt: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), Layer: LayerSmoke, Skill: smokeTestLeaf, SkillVersion: "2.1.2"}
	newer := &ArmResult{EndedAt: time.Date(2000, 1, 2, 0, 0, 0, 0, time.UTC), Layer: LayerSmoke, Skill: smokeTestLeaf, SkillVersion: "2.1.1"}
	newest := &ArmResult{EndedAt: time.Date(2000, 1, 3, 0, 0, 0, 0, time.UTC), Layer: LayerSmoke, Skill: "fdesign", SkillVersion: "9.0.0"}
	tests := make([]*testCase, 0)

	// none-nil
	tests = append(tests, &testCase{
		_id:         "none-nil",
		_expected:   nil,
		_shouldPass: true,

		root: newTempDir(t),
	})

	// newest-ended-at-wins
	versionsRoot := newTempDir(t)
	writeSmokeRecord(t, filepath.Join(versionsRoot, smokeTestLeaf, "2.1.2", runsDir, smokeArmLabel, resultFile), older)
	writeSmokeRecord(t, filepath.Join(versionsRoot, smokeTestLeaf, "2.1.1", runsDir, smokeArmLabel, resultFile), newer)
	tests = append(tests, &testCase{
		_id:         "newest-ended-at-wins",
		_expected:   newer,
		_shouldPass: true,

		root: versionsRoot,
	})

	// other-skill-ignored
	otherRoot := newTempDir(t)
	writeSmokeRecord(t, filepath.Join(otherRoot, smokeTestLeaf, "2.1.2", runsDir, smokeArmLabel, resultFile), older)
	writeSmokeRecord(t, filepath.Join(otherRoot, "fdesign", "9.0.0", runsDir, smokeArmLabel, resultFile), newest)
	tests = append(tests, &testCase{
		_id:         "other-skill-ignored",
		_expected:   older,
		_shouldPass: true,

		root: otherRoot,
	})

	// staging-never-read
	stagingRoot := newTempDir(t)
	writeSmokeRecord(t, filepath.Join(stagingRoot, smokeTestLeaf, "2.1.2", runsDir, smokeArmLabel, resultFile), older)
	writeTestFile(t, filepath.Join(stagingRoot, smokeStagingDir, "2000-01-03-000000-heartbeat-demo", runsDir, smokeArmLabel, resultFile), "{not json")
	tests = append(tests, &testCase{
		_id:         "staging-never-read",
		_expected:   older,
		_shouldPass: true,

		root: stagingRoot,
	})

	// fail-malformed-json
	malformedRoot := newTempDir(t)
	writeTestFile(t, filepath.Join(malformedRoot, smokeTestLeaf, "2.1.2", runsDir, smokeArmLabel, resultFile), "{not json")
	tests = append(tests, &testCase{
		_id:         "fail-malformed-json",
		_expected:   nil,
		_shouldPass: false,

		root: malformedRoot,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			record, err := LastSmoke(test.root, smokeTestLeaf)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, record)
		})
	}
}

func TestLoadSmokeRecord(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *loadedRecordView
		_shouldPass bool

		version string
	}

	home := newTempDir(t)
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	root := filepath.Join(home, smokeRootDir)
	expectedAnchors := []string{"produced", "closed"}
	run := provideSmokeRun()
	captured := &ArmRuntime{Findings: []heartbeat.Finding{}, Runs: []heartbeat.Run{run}}
	coverage := SmokeCoverage{AnchorsDeclared: 4, AnchorsExpected: expectedAnchors, AnchorsExpectedHit: 2, AnchorsHit: 4, ChecksDeclared: 4, ChecksPassed: 2}
	tests := make([]*testCase, 0)

	// pass-full-record
	full := interactiveSmokeDocument(expectedAnchors)
	require.NoError(t, writeSessionRecord(captured, full, "s-1", judgeSession(full, provideSmokeEnforce(), "", &run)))
	tests = append(tests, &testCase{
		_id: "pass-full-record",
		_expected: &loadedRecordView{
			Coverage:   coverage,
			Dir:        filepath.Join(root, smokeTestLeaf, smokeTestVersion),
			HasRuntime: true,
			Prompt:     "/heartbeat-demo pass-clean-run",
			RunAnchors: []string{"produced", "stage", "edit", "closed", "closed"},
			TokenLabel: "",
			Verdict:    statusPass,
		},
		_shouldPass: true,

		version: smokeTestVersion,
	})

	// pass-no-runtime
	bare := interactiveSmokeDocument(expectedAnchors)
	bare.Version = "2.1.1"
	require.NoError(t, writeSessionRecord(captured, bare, "s-1", judgeSession(bare, provideSmokeEnforce(), "", &run)))
	require.NoError(t, os.Remove(filepath.Join(root, smokeTestLeaf, "2.1.1", runsDir, smokeArmLabel, runtimeFile)))
	tests = append(tests, &testCase{
		_id: "pass-no-runtime",
		_expected: &loadedRecordView{
			Coverage:   coverage,
			Dir:        filepath.Join(root, smokeTestLeaf, "2.1.1"),
			HasRuntime: false,
			Prompt:     "/heartbeat-demo pass-clean-run",
			RunAnchors: nil,
			TokenLabel: "",
			Verdict:    statusPass,
		},
		_shouldPass: true,

		version: "2.1.1",
	})

	// fail-missing-result
	tests = append(tests, &testCase{
		_id:         "fail-missing-result",
		_expected:   nil,
		_shouldPass: false,

		version: "9.9.9",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			record, err := LoadSmokeRecord(root, smokeTestLeaf, test.version)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, viewLoadedRecord(record))
		})
	}
}

func TestPlaceFixtureTree(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedContent string
		_expectedFiles   []string
		_shouldPass      bool

		probe    string
		tree     string
		worktree string
	}

	tests := make([]*testCase, 0)

	// nested-tree-copied-with-paths
	nestedTree := newTempDir(t)
	writeTestFile(t, filepath.Join(nestedTree, "notes", "plan.md"), "plan\n")
	writeTestFile(t, filepath.Join(nestedTree, "a", "b", "deep.txt"), "deep\n")
	nestedWorktree := newTempDir(t)
	writeTestFile(t, filepath.Join(nestedWorktree, "file.txt"), "one\n")
	tests = append(tests, &testCase{
		_id:              "nested-tree-copied-with-paths",
		_expectedContent: "plan\n",
		_expectedFiles:   []string{filepath.Join("a", "b", "deep.txt"), "file.txt", filepath.Join("notes", "plan.md")},
		_shouldPass:      true,

		probe:    filepath.Join(nestedWorktree, "notes", "plan.md"),
		tree:     nestedTree,
		worktree: nestedWorktree,
	})

	// fail-existing-file-unchanged
	overlapTree := newTempDir(t)
	writeTestFile(t, filepath.Join(overlapTree, "file.txt"), "fixture\n")
	overlapWorktree := newTempDir(t)
	writeTestFile(t, filepath.Join(overlapWorktree, "file.txt"), "one\n")
	tests = append(tests, &testCase{
		_id:              "fail-existing-file-unchanged",
		_expectedContent: "one\n",
		_expectedFiles:   []string{"file.txt"},
		_shouldPass:      false,

		probe:    filepath.Join(overlapWorktree, "file.txt"),
		tree:     overlapTree,
		worktree: overlapWorktree,
	})

	// empty-tree-no-op
	plainWorktree := newTempDir(t)
	writeTestFile(t, filepath.Join(plainWorktree, "file.txt"), "one\n")
	tests = append(tests, &testCase{
		_id:              "empty-tree-no-op",
		_expectedContent: "one\n",
		_expectedFiles:   []string{"file.txt"},
		_shouldPass:      true,

		probe:    filepath.Join(plainWorktree, "file.txt"),
		tree:     "",
		worktree: plainWorktree,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := placeFixtureTree(test.tree, test.worktree)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedFiles, listFiles(t, test.worktree))
			assert.Equal(t, test._expectedContent, readOrEmpty(test.probe))
		})
	}
}

func TestNewSmokeManifest(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *RunManifest

		document *skillgraph.SmokeDocument
		mode     string
		overlay  string
	}

	tests := make([]*testCase, 0)

	// default-limits-and-identity
	tests = append(tests, &testCase{
		_id:       "default-limits-and-identity",
		_expected: provideSmokeManifest(),

		document: provideSmokeDocument(nil, nil),
		mode:     ModeNightly,
		overlay:  "",
	})

	// given-mode
	interactive := provideSmokeManifest()
	interactive.Mode = ModeInteractive
	tests = append(tests, &testCase{
		_id:       "given-mode",
		_expected: interactive,

		document: provideSmokeDocument(nil, nil),
		mode:     ModeInteractive,
		overlay:  "",
	})

	// empty-invoke-prompt
	emptyInvoke := provideSmokeDocument(nil, nil)
	empty := ""
	emptyInvoke.Invoke = &empty
	emptyPrompt := provideSmokeManifest()
	emptyPrompt.Prompt = "/heartbeat-demo"
	tests = append(tests, &testCase{
		_id:       "empty-invoke-prompt",
		_expected: emptyPrompt,

		document: emptyInvoke,
		mode:     ModeNightly,
		overlay:  "",
	})

	// token-placeholder-replaced
	tokenInvoke := provideSmokeDocument(nil, nil)
	withToken := "qq token: {token} again {token}"
	tokenInvoke.Invoke = &withToken
	tokenPrompt := provideSmokeManifest()
	tokenPrompt.Prompt = "/heartbeat-demo qq token: " + smokeTokenLabel + " again " + smokeTokenLabel
	tests = append(tests, &testCase{
		_id:       "token-placeholder-replaced",
		_expected: tokenPrompt,

		document: tokenInvoke,
		mode:     ModeNightly,
		overlay:  "",
	})

	// check-copied
	checked := provideSmokeManifest()
	checked.Check = []string{"make", "audit"}
	tests = append(tests, &testCase{
		_id:       "check-copied",
		_expected: checked,

		document: provideSmokeDocument(nil, []string{"make", "audit"}),
		mode:     ModeNightly,
		overlay:  "",
	})

	// overlay-passed-through
	overlaid := provideSmokeManifest()
	overlaid.OverlayPatch = "/skills/heartbeat-demo/smoke/new.patch"
	tests = append(tests, &testCase{
		_id:       "overlay-passed-through",
		_expected: overlaid,

		document: provideSmokeDocument(nil, nil),
		mode:     ModeNightly,
		overlay:  "/skills/heartbeat-demo/smoke/new.patch",
	})

	// Run tests
	for _, test := range tests {
		synctest.Test(t, func(t *testing.T) {
			manifest := newSmokeManifest("abc123", test.document, test.mode, test.overlay, smokeTokenLabel)
			assert.Equal(t, test._expected, manifest, test._id)
		})
	}
}

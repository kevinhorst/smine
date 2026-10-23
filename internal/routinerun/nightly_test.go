package routinerun

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/backlog"
	settings "github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/presentation"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/trash"
)

const acceptedSummary = `{"stages":[` +
	`{"stage":"style","status":"ok","summary":"profile refreshed"},` +
	`{"stage":"batch","status":"ok","summary":"1 record written"},` +
	`{"stage":"memory","status":"ok","summary":"no auto-memory"},` +
	`{"stage":"context","status":"ok","summary":"1 task"},` +
	`{"stage":"permissions","status":"ok","summary":"0 tasks"},` +
	`{"stage":"acdsl","status":"ok","summary":"0 tasks"},` +
	`{"stage":"problems","status":"ok","summary":"0 tasks"},` +
	`{"stage":"implement","status":"ok","summary":"1 change committed"},` +
	`{"stage":"consolidate","status":"ok","summary":"1 change picks alone"},` +
	`{"stage":"judge","status":"ok","summary":"accepted"}],` +
	`"changes":[{"id":"context/fact-x","sha":"abc1234","verdict":"ready","reason":"gates green"}],` +
	`"verdict":{"accepted":true,"reason":"audit green"}}`

// nightlyStages is the stage list a run with a due style stage passes.
const nightlyStages = "style," + nightlyMiningStages + ",judge"

// newNightlySandbox is a smine-nightly sandbox whose pending probe answers pending instead of asking peek.
func newNightlySandbox(t *testing.T, pending int) *sandbox {
	t.Helper()
	box := newSandbox(t, "smine-nightly", `{"kind":"smine-nightly","prompt":"/smine --nightly"}`)
	probe := countPending
	t.Cleanup(func() { countPending = probe })
	countPending = func(*nightlyRun, context.Context) (int, error) { return pending, nil }
	return box
}

// nightlyMarker is the run-summary path the fake /smine dispatcher writes.
func (s *sandbox) nightlyMarker() string {
	return filepath.Join(s.steerDir, runSummaryFileName)
}

func TestLoadNightlyOptions(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedAutoMerge bool
		_expectedDevMode   bool
		_expectedSubagents bool

		env     map[string]string
		profile *presentation.Profile
	}

	developer := &presentation.Profile{AutoMerge: true, Language: "en"}
	tests := make([]*testCase, 0)

	// defaults
	tests = append(tests, &testCase{
		_id:                "defaults",
		_expectedAutoMerge: true,
		_expectedDevMode:   false,
		_expectedSubagents: false,

		env:     map[string]string{},
		profile: developer,
	})

	// casual-forces-dev-off-merge-on
	tests = append(tests, &testCase{
		_id:                "casual-forces-dev-off-merge-on",
		_expectedAutoMerge: true,
		_expectedDevMode:   false,
		_expectedSubagents: false,

		env:     map[string]string{"SMINE_DEV_MODE": "1"},
		profile: &presentation.Profile{Audience: "casual", AutoMerge: false, DevMode: true, Language: "de"},
	})

	// env-dev-mode-wins
	tests = append(tests, &testCase{
		_id:                "env-dev-mode-wins",
		_expectedAutoMerge: true,
		_expectedDevMode:   true,
		_expectedSubagents: false,

		env:     map[string]string{"SMINE_DEV_MODE": "1"},
		profile: developer,
	})

	// developer-auto-merge-off
	tests = append(tests, &testCase{
		_id:                "developer-auto-merge-off",
		_expectedAutoMerge: false,
		_expectedDevMode:   false,
		_expectedSubagents: false,

		env:     map[string]string{},
		profile: &presentation.Profile{AutoMerge: false, Language: "en"},
	})

	// auto-merge-env-zero
	tests = append(tests, &testCase{
		_id:                "auto-merge-env-zero",
		_expectedAutoMerge: false,
		_expectedDevMode:   false,
		_expectedSubagents: false,

		env:     map[string]string{"ROUTINE_AUTO_MERGE": "0"},
		profile: developer,
	})

	// subagents-one-on
	tests = append(tests, &testCase{
		_id:                "subagents-one-on",
		_expectedAutoMerge: true,
		_expectedDevMode:   false,
		_expectedSubagents: true,

		env:     map[string]string{"SMINE_SUBAGENTS": "1"},
		profile: developer,
	})

	// subagents-eight-off
	tests = append(tests, &testCase{
		_id:                "subagents-eight-off",
		_expectedAutoMerge: true,
		_expectedDevMode:   false,
		_expectedSubagents: false,

		env:     map[string]string{"SMINE_SUBAGENTS": "8"},
		profile: developer,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			for _, key := range []string{"SMINE_DEV_MODE", "ROUTINE_AUTO_MERGE", "SMINE_SUBAGENTS"} {
				t.Setenv(key, test.env[key])
			}

			options := loadNightlyOptions(test.profile)

			assert.Equal(t, test._expectedAutoMerge, options.isAutoMerge)
			assert.Equal(t, test._expectedDevMode, options.isDevMode)
			assert.Equal(t, test._expectedSubagents, options.isSubagents)
		})
	}
}

func TestPipelinePrompt(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		isStyleDue bool
		options    *nightlyOptions
		roster     []routines.RunReportRepo
	}

	base := func() *nightlyOptions {
		return &nightlyOptions{agents: "claude", changesCap: 5, isOrchestrated: true, language: "en", sessionsCap: 30}
	}
	tests := make([]*testCase, 0)

	// all-stages
	tests = append(tests, &testCase{
		_id:       "all-stages",
		_expected: "/smine --nightly --stages " + nightlyStages + " --agents claude --cap 5 --limit 30 --language en --marker /wt/.smine-run/run-complete.json",

		isStyleDue: true,
		options:    base(),
		roster:     []routines.RunReportRepo{},
	})

	// style-not-due
	tests = append(tests, &testCase{
		_id:       "style-not-due",
		_expected: "/smine --nightly --stages " + nightlyMiningStages + ",judge --agents claude --cap 5 --limit 30 --language en --marker /wt/.smine-run/run-complete.json",

		isStyleDue: false,
		options:    base(),
		roster:     []routines.RunReportRepo{},
	})

	// judge-off
	judgeOff := base()
	judgeOff.isOrchestrated = false
	tests = append(tests, &testCase{
		_id:       "judge-off",
		_expected: "/smine --nightly --stages " + nightlyMiningStages + " --agents claude --cap 5 --limit 30 --language en --marker /wt/.smine-run/run-complete.json",

		isStyleDue: false,
		options:    judgeOff,
		roster:     []routines.RunReportRepo{},
	})

	// dev-subagents-no-repos-arg
	devSubagents := base()
	devSubagents.isDevMode = true
	devSubagents.isSubagents = true
	tests = append(tests, &testCase{
		_id:       "dev-subagents-no-repos-arg",
		_expected: "/smine --nightly --stages " + nightlyMiningStages + ",judge --dev --subagents --agents claude --cap 5 --limit 30 --language en --marker /wt/.smine-run/run-complete.json",

		isStyleDue: false,
		options:    devSubagents,
		roster:     []routines.RunReportRepo{{Name: "smine", Path: "/r/smine"}},
	})

	// language-empty
	languageEmpty := base()
	languageEmpty.language = ""
	tests = append(tests, &testCase{
		_id:       "language-empty",
		_expected: "/smine --nightly --stages " + nightlyMiningStages + ",judge --agents claude --cap 5 --limit 30 --marker /wt/.smine-run/run-complete.json",

		isStyleDue: false,
		options:    languageEmpty,
		roster:     []routines.RunReportRepo{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			runner := &Runner{
				config:   &Config{},
				dir:      t.TempDir(),
				manifest: &routines.Manifest{Kind: routines.KindSmineNightly, Prompt: "/smine --nightly"},
			}
			nightly := &nightlyRun{options: test.options, roster: test.roster, runner: runner}

			assert.Equal(t, test._expected, nightly.pipelinePrompt(test.isStyleDue, "/wt/.smine-run/run-complete.json"))
		})
	}
}

func TestNightlyRun(t *testing.T) {
	box := newNightlySandbox(t, 1)
	t.Setenv("ROUTINE_AUTO_MERGE", "0")
	storeWrite := filepath.Join(box.store, "backlog", "backlog.json")
	task := `{"id": "context/fact-x", "kind": "context", "title": "Fact x", "problem": "Sessions lacked fact x.", "evidence": [{"session": "s1", "quote": "where is x", "finding": 0}], "fix": "The fact is recorded.", "sessions": ["s1"], "target": "context/facts/x.md", "change": "Record fact x.", "notes": ""}`
	t.Setenv(fakeScriptEnv, "/smine=mined.txt=mined;/smine="+storeWrite+"=mined row;/smine=.smine-run/changes/context-fact-x.json="+task)
	t.Setenv(fakeMarkerEnv, box.nightlyMarker())
	t.Setenv(fakeMarkerBodyEnv, acceptedSummary)

	status := box.run(t)

	require.Equal(t, exitOk, status)
	rows := box.results(t)
	require.Len(t, rows, 2)
	assert.True(t, rows[0].IsRunning())
	assert.Equal(t, "pipeline", rows[1].Stage)
	assert.Equal(t, nightBranch(today(), repos.ProposalsSuffix), rows[1].Branch)

	reports, err := routines.Reports(box.routineDir, 5)
	require.NoError(t, err)
	require.Len(t, reports, 1)
	report := reports[0]
	var reportStages []string
	for _, stage := range report.Stages {
		reportStages = append(reportStages, stage.Stage)
	}
	assert.Equal(t, "full", report.Mode)
	assert.Equal(t, append([]string{"pipeline"}, strings.Split(nightlyStages, ",")...), reportStages)
	assert.Contains(t, report.Stages[0].Result, "/smine --nightly --stages "+nightlyStages)
	expectedChanges := []routines.RunReportChange{{Id: "context/fact-x", Reason: "gates green", Sha: "abc1234", Verdict: "ready"}}
	assert.Equal(t, expectedChanges, report.Changes)
	expectedEvidence := []routines.ChangeEvidence{{Quote: "where is x", Session: "s1"}}
	expectedTasks := []routines.ChangeTask{{Id: "context/fact-x", Change: "Record fact x.", Evidence: expectedEvidence, Fix: "The fact is recorded.", Kind: "context", Problem: "Sessions lacked fact x.", Sessions: []string{"s1"}, Target: "context/facts/x.md", Title: "Fact x"}}
	assert.Equal(t, expectedTasks, report.Tasks)
	assert.Equal(t, 0, report.Publish.Status)
	assert.Equal(t, "nothing to land", report.Publish.BatchLanding)
	assert.Empty(t, report.BatchBranch)
	require.NotNil(t, report.Orchestrate)
	assert.False(t, report.Orchestrate.Merged)
	assert.Contains(t, report.Orchestrate.Report, "kept: auto-merge off")

	branch := nightBranch(today(), repos.ProposalsSuffix)
	files := gitRun(t, box.repo, "show", "--name-only", "--format=", branch)
	assert.Contains(t, files, "mined.txt")
	assert.NotContains(t, files, runSummaryFileName)
	assert.NotContains(t, files, "backlog.json")
	archive := filepath.Dir(box.store)
	assert.Equal(t, filepath.Base(box.store)+"/backlog/backlog.json", gitRun(t, archive, "show", "--name-only", "--format=", "HEAD"))
	assert.FileExists(t, filepath.Join(box.routineDir, styleStampFileName))
}

func TestNightlyRun_IsStyleDue(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		nightly *nightlyRun
	}

	tests := make([]*testCase, 0)

	// no-stamp
	tests = append(tests, &testCase{
		_id:       "no-stamp",
		_expected: true,

		nightly: newStyleDueRun(t, false),
	})

	// plan-commit-after-stamp
	planRun := newStyleDueRun(t, true)
	commitFileAt(t, planRun.runner.repoRoot, "plans/x.md", "plan\n", styleAfterStamp)
	tests = append(tests, &testCase{
		_id:       "plan-commit-after-stamp",
		_expected: true,

		nightly: planRun,
	})

	// archive-proposal-archive-after-stamp
	proposalArchiveRun := newStyleDueRun(t, true)
	commitFileAt(t, filepath.Dir(proposalArchiveRun.runner.storeRoot), "code/proposals/archive/done.md", "done\n", styleAfterStamp)
	tests = append(tests, &testCase{
		_id:       "archive-proposal-archive-after-stamp",
		_expected: false,

		nightly: proposalArchiveRun,
	})

	// archive-backlog-after-stamp
	archiveBacklogRun := newStyleDueRun(t, true)
	commitFileAt(t, filepath.Dir(archiveBacklogRun.runner.storeRoot), "code/backlog/backlog.json", "{}\n", styleAfterStamp)
	tests = append(tests, &testCase{
		_id:       "archive-backlog-after-stamp",
		_expected: true,

		nightly: archiveBacklogRun,
	})

	// code-repo-backlog-after-stamp
	codeBacklogRun := newStyleDueRun(t, true)
	commitFileAt(t, codeBacklogRun.runner.repoRoot, "backlog/backlog.json", "{}\n", styleAfterStamp)
	tests = append(tests, &testCase{
		_id:       "code-repo-backlog-after-stamp",
		_expected: false,

		nightly: codeBacklogRun,
	})

	// archive-other-path-after-stamp
	otherPathRun := newStyleDueRun(t, true)
	commitFileAt(t, filepath.Dir(otherPathRun.runner.storeRoot), "code/proposals/skills.json", "{}\n", styleAfterStamp)
	tests = append(tests, &testCase{
		_id:       "archive-other-path-after-stamp",
		_expected: false,

		nightly: otherPathRun,
	})

	// nothing-after-stamp
	tests = append(tests, &testCase{
		_id:       "nothing-after-stamp",
		_expected: false,

		nightly: newStyleDueRun(t, true),
	})

	// store-root-not-a-repo
	plainStoreRun := newStyleDueRun(t, true)
	plainStoreRun.runner.storeRoot = t.TempDir()
	tests = append(tests, &testCase{
		_id:       "store-root-not-a-repo",
		_expected: false,

		nightly: plainStoreRun,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.nightly.isStyleDue(context.Background()))
		})
	}
}

// Committer dates around the style stamp 946771200 (2000-01-02 UTC).
const (
	styleAfterStamp  = "946857600 +0000"
	styleBeforeStamp = "946684800 +0000"
	styleStamp       = "946771200"
)

// newStyleDueRun is a nightly run over a code repo and a store archive whose
// commits all predate the style stamp; isStamped writes the stamp.
func newStyleDueRun(t *testing.T, isStamped bool) *nightlyRun {
	t.Helper()
	isolateHome(t)
	t.Setenv("GIT_COMMITTER_DATE", styleBeforeStamp)
	repoRoot := newTestRepo(t)
	storeRoot := filepath.Join(t.TempDir(), "archive", "code")
	newStoreArchive(t, storeRoot, sandboxStore)

	dir := t.TempDir()
	if isStamped {
		require.NoError(t, os.WriteFile(filepath.Join(dir, styleStampFileName), []byte(styleStamp+"\n"), 0o644))
	}
	runner := &Runner{dir: dir, repoRoot: repoRoot, storeRoot: storeRoot}
	return &nightlyRun{runner: runner}
}

func TestNightlyRunStyleOkPublishes(t *testing.T) {
	box := newNightlySandbox(t, 1)
	t.Setenv("ROUTINE_AUTO_MERGE", "0")
	t.Setenv(fakeScriptEnv, "/smine=.smine-run/style-profile.md="+styleProfileMarker+" — test -->")
	t.Setenv(fakeMarkerEnv, box.nightlyMarker())
	t.Setenv(fakeMarkerBodyEnv, acceptedSummary)

	status := box.run(t)

	require.Equal(t, exitOk, status)
	published, err := os.ReadFile(presentation.DefaultStylePath())
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(published), styleProfileMarker))
}

func TestNightlyRunStyleFailedKeepsOldProfile(t *testing.T) {
	box := newNightlySandbox(t, 1)
	t.Setenv("ROUTINE_AUTO_MERGE", "0")
	t.Setenv(fakeScriptEnv, "/smine=.smine-run/style-profile.md="+styleProfileMarker+" — new -->")
	t.Setenv(fakeMarkerEnv, box.nightlyMarker())
	failedStyle := strings.Replace(acceptedSummary, `{"stage":"style","status":"ok"`, `{"stage":"style","status":"failed"`, 1)
	t.Setenv(fakeMarkerBodyEnv, failedStyle)
	oldProfile := styleProfileMarker + " — old -->\n"
	require.NoError(t, os.MkdirAll(filepath.Dir(presentation.DefaultStylePath()), 0o755))
	require.NoError(t, os.WriteFile(presentation.DefaultStylePath(), []byte(oldProfile), 0o644))

	box.run(t)

	kept, err := os.ReadFile(presentation.DefaultStylePath())
	require.NoError(t, err)
	assert.Equal(t, oldProfile, string(kept))
}

// isolateHome points the home at a temp dir: a landing deploys through install.Apply, which resolves ~/.claude from it.
func isolateHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

// landingScratchRef is the journal path of the scratch branch the night's landing deletes.
func landingScratchRef(nightly *nightlyRun) string {
	scratch := strings.TrimSuffix(nightly.branch, repos.ProposalsSuffix) + landingSuffix
	return nightly.runner.repoRoot + "#" + scratch
}

// newLandingRun is a judged night over a fresh repo: two ready change commits
// on the proposals branch, the second reported by an abbreviated sha.
func newLandingRun(t *testing.T, isAutoMerge bool) (*nightlyRun, []string) {
	t.Helper()
	isolateHome(t)
	repo := newTestRepo(t)
	group := newTestGroup(t, repo, nightGroup, 0)
	night := nightBranch(today(), repos.ProposalsSuffix)
	first := commitOnBranch(t, repo, night, "a.txt", "skills.a: Fixed a\n\nSmine-Change: skills/a")
	second := commitOnBranch(t, repo, night, "b.txt", "skills.b: Fixed b\n\nSmine-Change: skills/b")

	changes := []routines.RunReportChange{
		{Id: "skills/a", Sha: first, Verdict: string(changeReady)},
		{Id: "skills/b", Sha: second[:8], Verdict: string(changeReady)},
	}
	report := &reportWriter{path: filepath.Join(t.TempDir(), "run.json"), report: &routines.RunReport{Changes: changes}}
	runner := &Runner{config: &Config{AgentwtPath: group.agentwtPath}, repoRoot: repo, worktree: group}
	nightly := &nightlyRun{branch: night, options: &nightlyOptions{isAutoMerge: isAutoMerge}, report: report, runner: runner}
	return nightly, []string{first, second}
}

func TestNightlyRunLandsAcceptedRun(t *testing.T) {
	type testCase struct {
		_id                     string
		_expectedLandingJournal []string
		_expectedMerged         bool
		_expectedMoved          bool
		_expectedReports        []string

		mainBefore string
		nightly    *nightlyRun
	}

	accepted := &runVerdict{Accepted: true, Reason: "audit green"}
	tests := make([]*testCase, 0)

	// every-ready-change-landed
	landed, _ := newLandingRun(t, true)
	tests = append(tests, &testCase{
		_id:                     "every-ready-change-landed",
		_expectedLandingJournal: []string{landingScratchRef(landed)},
		_expectedMerged:         true,
		_expectedMoved:          true,
		_expectedReports:        []string{"accepted=true: audit green", "deploy failed"},

		mainBefore: gitRun(t, landed.runner.repoRoot, "rev-parse", "main"),
		nightly:    landed,
	})

	// one-conflict-moves-nothing
	conflicting, conflictShas := newLandingRun(t, true)
	commitFile(t, conflicting.runner.repoRoot, "b.txt", "main side\n")
	tests = append(tests, &testCase{
		_id:                     "one-conflict-moves-nothing",
		_expectedLandingJournal: []string{landingScratchRef(conflicting)},
		_expectedMerged:         false,
		_expectedMoved:          false,
		_expectedReports:        []string{"kept: " + conflictShas[1] + " conflicts with main — kept on its night"},

		mainBefore: gitRun(t, conflicting.runner.repoRoot, "rev-parse", "main"),
		nightly:    conflicting,
	})

	// forward-skipped-on-dirty-main
	dirty, _ := newLandingRun(t, true)
	require.NoError(t, os.WriteFile(filepath.Join(dirty.runner.repoRoot, "dirty.txt"), []byte("wip\n"), 0o644))
	tests = append(tests, &testCase{
		_id:                     "forward-skipped-on-dirty-main",
		_expectedLandingJournal: []string{landingScratchRef(dirty)},
		_expectedMerged:         false,
		_expectedMoved:          false,
		_expectedReports:        []string{"kept: skipped: main is checked out with uncommitted changes"},

		mainBefore: gitRun(t, dirty.runner.repoRoot, "rev-parse", "main"),
		nightly:    dirty,
	})

	// auto-merge-off
	off, _ := newLandingRun(t, false)
	tests = append(tests, &testCase{
		_id:                     "auto-merge-off",
		_expectedLandingJournal: []string{},
		_expectedMerged:         false,
		_expectedMoved:          false,
		_expectedReports:        []string{"kept: auto-merge off"},

		mainBefore: gitRun(t, off.runner.repoRoot, "rev-parse", "main"),
		nightly:    off,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			outcome := test.nightly.land(context.Background(), accepted)

			assert.Equal(t, test._expectedMerged, outcome.Merged)
			assert.Equal(t, test._expectedMoved, outcome.MainSha != test.mainBefore)
			for _, line := range test._expectedReports {
				assert.Contains(t, outcome.Report, line)
			}
			assert.Empty(t, gitRun(t, test.nightly.runner.repoRoot, "branch", "--list", "*"+landingSuffix))
			assert.Equal(t, test._expectedLandingJournal, journalPaths(t, test.nightly.runner.worktree.trash, "nightly landing"))
		})
	}
}

func TestNightlyRunLandsRecordsAndReadyChange(t *testing.T) {
	box := newNightlySandbox(t, 1)
	t.Setenv("ROUTINE_AUTO_MERGE", "1")
	t.Setenv(fakeScriptEnv, "/smine=sessions/third.md=third;/smine=change.txt=change")
	t.Setenv(fakeCommitEnv, "skills.x: Changed x\n\nSmine-Change: skills/x")
	t.Setenv(fakeMarkerEnv, box.nightlyMarker())
	readySummary := strings.Replace(acceptedSummary, `"sha":"abc1234"`, `"sha":"`+fakeHeadPlaceholder+`"`, 1)
	t.Setenv(fakeMarkerBodyEnv, readySummary)

	status := box.run(t)

	require.Equal(t, exitOk, status)
	reports, err := routines.Reports(box.routineDir, 5)
	require.NoError(t, err)
	require.Len(t, reports, 1)
	report := reports[0]
	batch := nightBranch(today(), repos.BatchSuffix)
	assert.Equal(t, batch, report.BatchBranch)
	assert.True(t, strings.HasPrefix(report.Publish.BatchLanding, "landed as "), report.Publish.BatchLanding)
	assert.Equal(t, []routines.RunReportFile{{Path: "sessions/third.md", Status: "A"}}, report.Publish.Files)
	require.NotNil(t, report.Orchestrate)
	assert.True(t, report.Orchestrate.Merged, report.Orchestrate.Report)
	assert.Equal(t, gitRun(t, box.repo, "rev-parse", "main"), report.Orchestrate.MainSha)
	tree := gitRun(t, box.repo, "ls-tree", "-r", "--name-only", "main")
	assert.Contains(t, tree, "sessions/third.md")
	assert.Contains(t, tree, "change.txt")
	open := gitRun(t, box.repo, "branch", "--list", "--format=%(refname:short)", "claude-routines/*")
	assert.Equal(t, []string{nightBranch(today(), repos.ProposalsSuffix)}, strings.Fields(open))
}

func TestNightlyRunNothingToMine(t *testing.T) {
	box := newNightlySandbox(t, 0)
	t.Setenv("ROUTINE_AUTO_MERGE", "1")
	t.Setenv(fakeMarkerEnv, box.nightlyMarker())
	t.Setenv(fakeMarkerBodyEnv, acceptedSummary)
	older := nightBranch("2026-01-01", repos.ProposalsSuffix)
	commitOnBranch(t, box.repo, older, "a.txt", "skills.a: Fixed a\n\nSmine-Change: skills/a")
	commitOnBranch(t, box.repo, nightBranch("2026-01-02", repos.ProposalsSuffix), "b.txt", "skills.b: Fixed b\n\nSmine-Change: skills/b")
	commitOnBranch(t, box.repo, nightBranch("2026-01-03", repos.ProposalsSuffix), "c.txt", "skills.c: Fixed c\n\nSmine-Change: skills/c")
	mainBefore := gitRun(t, box.repo, "rev-parse", "main")

	status := box.run(t)

	require.Equal(t, exitOk, status)
	reports, err := routines.Reports(box.routineDir, 5)
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.NotNil(t, reports[0].Orchestrate)
	assert.Equal(t, "nothing to mine: no unmined session in peek's window", reports[0].Orchestrate.Report)
	assert.Empty(t, reports[0].Stages)
	assert.Equal(t, mainBefore, gitRun(t, box.repo, "rev-parse", "main"))
	assert.False(t, box.branchExists(t, nightBranch(today(), repos.ProposalsSuffix)))
	assert.True(t, box.branchExists(t, older))
	assert.Empty(t, droppedShas(t, box.repo))
}

func TestNightlyRunDropsOldNights(t *testing.T) {
	box := newNightlySandbox(t, 1)
	t.Setenv("ROUTINE_AUTO_MERGE", "0")
	t.Setenv(fakeMarkerEnv, box.nightlyMarker())
	t.Setenv(fakeMarkerBodyEnv, acceptedSummary)
	oldest := nightBranch("2026-01-01", repos.ProposalsSuffix)
	oldestSha := commitOnBranch(t, box.repo, oldest, "a.txt", "skills.a: Fixed a\n\nSmine-Change: skills/a")
	middle := nightBranch("2026-01-02", repos.ProposalsSuffix)
	middleSha := commitOnBranch(t, box.repo, middle, "b.txt", "skills.b: Fixed b\n\nSmine-Change: skills/b")
	newest := nightBranch("2026-01-03", repos.ProposalsSuffix)
	commitOnBranch(t, box.repo, newest, "c.txt", "skills.c: Fixed c\n\nSmine-Change: skills/c")

	status := box.run(t)

	require.Equal(t, exitOk, status)
	assert.ElementsMatch(t, []string{oldestSha, middleSha}, droppedShas(t, box.repo))
	remaining := gitRun(t, box.repo, "branch", "--list", "--format=%(refname:short)", "claude-routines/*")
	assert.ElementsMatch(t, []string{newest, nightBranch(today(), repos.ProposalsSuffix)}, strings.Fields(remaining))
}

func TestNightlyRunSweepsOversizedChange(t *testing.T) {
	box := newNightlySandbox(t, 1)
	t.Setenv("ROUTINE_AUTO_MERGE", "0")
	t.Setenv(fakeMarkerEnv, box.nightlyMarker())
	t.Setenv(fakeMarkerBodyEnv, acceptedSummary)
	tonightFiles := make([]string, 0, repos.MaxChangeFiles+1)
	olderFiles := make([]string, 0, repos.MaxChangeFiles+1)
	for i := range repos.MaxChangeFiles + 1 {
		tonightFiles = append(tonightFiles, fmt.Sprintf("/smine=big/file-%d.txt=x", i))
		olderFiles = append(olderFiles, fmt.Sprintf("old/file-%d.txt", i))
	}
	t.Setenv(fakeScriptEnv, strings.Join(tonightFiles, ";"))
	t.Setenv(fakeCommitEnv, "skills.big: Changed eight files\n\nThe big change.\n\nSmine-Change: skills/big")
	older := nightBranch("2026-01-01", repos.ProposalsSuffix)
	olderSha := commitFilesOnBranch(t, box.repo, older, olderFiles, "skills.old: Changed eight old files\n\nSmine-Change: skills/old")

	status := box.run(t)

	require.Equal(t, exitOk, status)
	tonight := nightBranch(today(), repos.ProposalsSuffix)
	sha := gitRun(t, box.repo, "log", "-1", "--format=%H", "--grep=Smine-Change: skills/big", tonight)
	assert.Equal(t, sha, gitRun(t, box.repo, "for-each-ref", "--format=%(objectname)", repos.RejectedRefPrefix))
	assert.True(t, box.branchExists(t, older))
	assert.NotContains(t, droppedShas(t, box.repo), olderSha)
	store, err := backlog.Load(filepath.Join(box.store, "backlog", "backlog.json"))
	require.NoError(t, err)
	icebox := make([]backlog.Item, 0)
	for _, item := range store.Items() {
		if item.Section == backlog.IceboxSection {
			icebox = append(icebox, item)
		}
	}
	require.Len(t, icebox, 1)
	assert.Equal(t, "commit "+sha, icebox[0].Evidence[0])
	assert.Contains(t, icebox[0].Description, "8 files, 8 changed lines — over the proposal cap (7 files / 420 lines)")
	assert.True(t, box.branchExists(t, nightBranch(today(), repos.ProposalsSuffix)))
}

func TestApplySettled(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []routines.RunReportChange

		changes []routines.RunReportChange
		settled map[string]string
	}

	fullSha := "abc1234def5678abc1234def5678abc1234def56"
	reason := "over the proposal cap: 8 files, 8 lines → B-002"
	settled := map[string]string{fullSha: reason}
	tests := make([]*testCase, 0)

	// short-sha-row-matches
	tests = append(tests, &testCase{
		_id:       "short-sha-row-matches",
		_expected: []routines.RunReportChange{{Id: "skills/big", Reason: reason, Sha: "abc1234", Verdict: string(changeRejected)}},

		changes: []routines.RunReportChange{{Id: "skills/big", Reason: "gates green", Sha: "abc1234", Verdict: string(changeReady)}},
		settled: settled,
	})

	// full-sha-row-matches
	tests = append(tests, &testCase{
		_id:       "full-sha-row-matches",
		_expected: []routines.RunReportChange{{Id: "skills/big", Reason: reason, Sha: fullSha, Verdict: string(changeRejected)}},

		changes: []routines.RunReportChange{{Id: "skills/big", Reason: "gates green", Sha: fullSha, Verdict: string(changeReady)}},
		settled: settled,
	})

	// other-rows-untouched
	tests = append(tests, &testCase{
		_id:       "other-rows-untouched",
		_expected: []routines.RunReportChange{{Id: "skills/small", Reason: "gates green", Sha: "fff0000", Verdict: string(changeReady)}},

		changes: []routines.RunReportChange{{Id: "skills/small", Reason: "gates green", Sha: "fff0000", Verdict: string(changeReady)}},
		settled: settled,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, applySettled(test.changes, test.settled))
		})
	}
}

// expectedOversizedItem is the Icebox row TestOversizedItem's change yields with description and title.
func expectedOversizedItem(description, title string) *backlog.Item {
	item := &backlog.Item{
		Added:       "2026-10-08",
		Deps:        []string{},
		Description: description,
		Evidence:    []string{"commit abc1234def", "s1", "s2"},
		Link:        "",
		Mined:       &backlog.Mined{By: "night-session", Date: "2026-10-08", Sessions: []string{"s1", "s2"}},
		Pri:         "high",
		Route:       "/fdesign",
		Size:        "L",
		Status:      backlog.StatusShaped,
		Theme:       "smine",
		Title:       title,
	}
	return item
}

func TestOversizedItem(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *backlog.Item

		change *repos.OpenChange
		task   *routines.ChangeTask
	}

	now := time.Date(2026, 10, 8, 3, 0, 0, 0, time.UTC)
	branch := nightBranch("2026-10-07", repos.ProposalsSuffix)
	message := &repos.CommitMessage{
		Body:      "The commit body.",
		ChangeIds: []string{"skills/big"},
		Sessions:  []string{"s1", "s2"},
		Subject:   "skills.big: Changed eight files",
	}
	change := &repos.OpenChange{Branch: branch, Commit: repos.UnplacedCommit{Sha: "abc1234def", ShortSha: "abc1234"}, Message: message}
	stat := &repos.ChangeStat{Files: 8, Lines: 30}
	size := "8 files, 30 changed lines — over the proposal cap (7 files / 420 lines); commit abc1234def on " + branch
	tests := make([]*testCase, 0)

	// with-task-record
	tests = append(tests, &testCase{
		_id:       "with-task-record",
		_expected: expectedOversizedItem("The problem.\nThe fix.\n"+size+"\nDone when: Split the change.", message.Subject),

		change: change,
		task:   &routines.ChangeTask{Id: "skills/big", Change: "Split the change.", Fix: "The fix.", Problem: "The problem."},
	})

	// without-record
	tests = append(tests, &testCase{
		_id:       "without-record",
		_expected: expectedOversizedItem("The commit body.\n"+size+"\nDone when: skills.big: Changed eight files", message.Subject),

		change: change,
		task:   nil,
	})

	// long-subject
	longSubject := "skills.big: " + strings.Repeat("x", 118)
	longMessage := &repos.CommitMessage{
		Body:      "The commit body.",
		ChangeIds: []string{"skills/big"},
		Sessions:  []string{"s1", "s2"},
		Subject:   longSubject,
	}
	longChange := &repos.OpenChange{Branch: branch, Commit: repos.UnplacedCommit{Sha: "abc1234def", ShortSha: "abc1234"}, Message: longMessage}
	tests = append(tests, &testCase{
		_id:       "long-subject",
		_expected: expectedOversizedItem(longSubject+"\nThe commit body.\n"+size+"\nDone when: "+longSubject, longSubject[:119]+"…"),

		change: longChange,
		task:   nil,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, oversizedItem(test.change, "night-session", test.task, stat, now))
		})
	}
}

func (s *sandbox) branchExists(t *testing.T, branch string) bool {
	t.Helper()
	_, err := git(context.Background(), s.repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

func TestNightlyRunMissingSummary(t *testing.T) {
	box := newNightlySandbox(t, 1)
	t.Setenv("ROUTINE_AUTO_MERGE", "1")
	t.Setenv(fakeExitAfterEnv, "1")

	status := box.run(t)

	assert.NotEqual(t, exitOk, status)
	reports, err := routines.Reports(box.routineDir, 5)
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.NotNil(t, reports[0].Orchestrate)
	assert.False(t, reports[0].Orchestrate.Merged)
	assert.Contains(t, reports[0].Orchestrate.Report, "accepted=false: no valid run summary")
}

func TestNightlyRunSubjectMarker(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedSubject string

		markerBody string
	}

	failedStyleAndImplement := strings.Replace(acceptedSummary, `{"stage":"style","status":"ok"`, `{"stage":"style","status":"failed"`, 1)
	failedStyleAndImplement = strings.Replace(failedStyleAndImplement, `{"stage":"implement","status":"ok"`, `{"stage":"implement","status":"failed"`, 1)
	tests := make([]*testCase, 0)

	// accepted-run
	tests = append(tests, &testCase{
		_id:              "accepted-run",
		_expectedSubject: "sessions: Recorded nightly smine-nightly output",

		markerBody: acceptedSummary,
	})

	// failed-stages-named
	tests = append(tests, &testCase{
		_id:              "failed-stages-named",
		_expectedSubject: "sessions: [stage-failed: style, implement] Recorded nightly smine-nightly output",

		markerBody: failedStyleAndImplement,
	})

	// empty-marker-is-no-summary
	tests = append(tests, &testCase{
		_id:              "empty-marker-is-no-summary",
		_expectedSubject: "sessions: [failed] Recorded nightly smine-nightly output",

		markerBody: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			box := newNightlySandbox(t, 1)
			t.Setenv("ROUTINE_AUTO_MERGE", "0")
			t.Setenv(fakeScriptEnv, "/smine=sessions/mined.md=mined")
			t.Setenv(fakeMarkerEnv, box.nightlyMarker())
			t.Setenv(fakeMarkerBodyEnv, test.markerBody)

			box.run(t)

			subject := gitRun(t, box.repo, "log", "-1", "--format=%s", "main")
			assert.True(t, strings.HasPrefix(subject, test._expectedSubject), "subject: %s", subject)
		})
	}
}

func TestNightlyRunDeletesSettledProposals(t *testing.T) {
	box := newNightlySandbox(t, 1)
	t.Setenv("ROUTINE_AUTO_MERGE", "0")
	settled := nightBranch("2026-01-01", repos.ProposalsSuffix)
	rejected := commitOnBranch(t, box.repo, settled, "a.txt", "skills.a: Fixed a\n\nSmine-Change: skills/a")
	require.NoError(t, repos.Reject(context.Background(), box.repo, rejected))
	open := nightBranch("2026-01-02", repos.ProposalsSuffix)
	commitOnBranch(t, box.repo, open, "b.txt", "skills.b: Fixed b\n\nSmine-Change: skills/b")
	t.Setenv(fakeScriptEnv, "/smine=sessions/third.md=third")
	t.Setenv(fakeMarkerEnv, box.nightlyMarker())
	t.Setenv(fakeMarkerBodyEnv, acceptedSummary)

	status := box.run(t)

	require.Equal(t, exitOk, status)
	stateDir, err := settings.StateDir("")
	require.NoError(t, err)
	settledPaths := journalPaths(t, trash.New(stateDir, 0), "nightly settled proposals")
	assert.Equal(t, []string{box.repo + "#" + settled}, settledPaths)
	remaining := gitRun(t, box.repo, "branch", "--list", "--format=%(refname:short)", "claude-routines/*")
	assert.ElementsMatch(t, []string{open, nightBranch(today(), repos.ProposalsSuffix)}, strings.Fields(remaining))
	assert.Contains(t, gitRun(t, box.repo, "ls-tree", "-r", "--name-only", "main"), "sessions/third.md")
}

func TestGroupWorktree_SeedLedgerLeavesIndexClean(t *testing.T) {
	repo := newTestRepo(t)
	night := "claude-routines/smine-nightly-2026-01-01"
	gitRun(t, repo, "checkout", "-q", "-b", night)
	commitFile(t, repo, "sessions/a.md", "a\n")
	gitRun(t, repo, "checkout", "-q", "main")
	worktree := filepath.Join(t.TempDir(), "wt")
	gitRun(t, repo, "worktree", "add", "-q", "-b", "claude-routines/smine-nightly-2026-01-02", worktree, "main")
	group := &groupWorktree{path: worktree, repoRoot: repo}

	err := group.seedLedger(context.Background(), night)

	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(worktree, "sessions", "a.md"))
	assert.Empty(t, gitRun(t, worktree, "diff", "--cached", "--name-only"))
}

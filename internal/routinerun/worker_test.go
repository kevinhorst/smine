package routinerun

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/routines"
)

const (
	workerTestFailureEnvelope = `{"type":"result","subtype":"error_during_execution","is_error":true,"session_id":"session-1","result":"broke"}`
	workerTestSessionId       = "0123abcd-0123-4567-89ab-0123456789ab"
	workerTestTimeout         = 30 * time.Second
)

// workerObservation is what a RunWorker run leaves behind: the result line's
// fields and the worker branch and worktree in the repo.
type workerObservation struct {
	branchBody    string
	branchSubject string
	exitStatus    int
	hasWorktree   bool
	isCommitTip   bool
	result        string
	sessionId     string
	status        string
}

// keepWorktree checks out branch at the worker path of name, as a stopped
// cell launched with KeepStopped leaves it.
func keepWorktree(t *testing.T, repo, name, branch string) string {
	t.Helper()
	path := filepath.Join(repo, workerWorktreesPath, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	gitRun(t, repo, "worktree", "add", "--quiet", "-b", branch, path, mainBranch)
	return path
}

// observeWorker reads the run's result and the worker branch and worktree;
// a refused run yields the zero result fields.
func observeWorker(t *testing.T, spec *WorkerSpec, result *WorkerResult) *workerObservation {
	t.Helper()
	branch := workerBranchPrefix + spec.Name
	_, err := os.Stat(filepath.Join(spec.Repo, workerWorktreesPath, spec.Name))
	observation := &workerObservation{hasWorktree: err == nil}
	if tip, err := git(context.Background(), spec.Repo, "rev-parse", "refs/heads/"+branch); err == nil {
		observation.branchBody = gitRun(t, spec.Repo, "log", "-1", "--format=%b", tip)
		observation.branchSubject = gitRun(t, spec.Repo, "log", "-1", "--format=%s", tip)
		observation.isCommitTip = result != nil && result.Branch == branch && result.Commit == tip
	}
	if result == nil {
		return observation
	}

	observation.exitStatus = result.ExitStatus
	observation.result = result.Result
	observation.sessionId = result.SessionId
	observation.status = result.Status
	return observation
}

// setTestEnv sets each variable for the running subtest only.
func setTestEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for key, value := range env {
		t.Setenv(key, value)
	}
}

// writeTestSkill deploys a SKILL.md with the given frontmatter lines under
// home's skills dir.
func writeTestSkill(t *testing.T, home, name, frontmatter string) {
	t.Helper()
	dir := filepath.Join(home, ".claude", "skills", name)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	content := "---\nname: " + name + "\n" + frontmatter + "---\nbody\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644))
}

func TestRunWorker(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *workerObservation
		_shouldPass bool

		env  map[string]string
		spec *WorkerSpec
	}

	ctx := context.Background()
	installFakeClaude(t)
	home := t.TempDir()
	writeTestSkill(t, home, "demo", "allowed-tools: Read\n")
	steerRoot := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ROUTINE_STEER_ROOT", steerRoot)
	t.Setenv("DISABLE_AUTOUPDATER", "")
	t.Setenv("ROUTINE_PROTECTED_PATHS", "")
	t.Setenv(headlessEnvKey, "")
	markerOf := func(name string) string {
		return filepath.Join(steerRoot, workerSteerPrefix+name, routines.RunCompleteFileName)
	}
	nudgeOf := func(name string) string {
		return fmt.Sprintf(nudgeMessage, markerOf(name), markerOf(name))
	}
	tests := make([]*testCase, 0)

	// done-with-leftovers
	tests = append(tests, &testCase{
		_id: "done-with-leftovers",
		_expected: &workerObservation{
			branchSubject: "fleet: Recorded worker done-with-leftovers output",
			isCommitTip:   true,
			result:        "/demo go" + fmt.Sprintf(markerInstruction, markerOf("done-with-leftovers")),
			sessionId:     "session-1",
			status:        workerStatusDone,
		},
		_shouldPass: true,

		env:  map[string]string{fakeMarkerEnv: markerOf("done-with-leftovers"), fakeScriptEnv: "/demo=out.txt=done"},
		spec: &WorkerSpec{Base: mainBranch, Name: "done-with-leftovers", Repo: newTestRepo(t), Skill: "demo", SkillArgs: "go", Timeout: workerTestTimeout},
	})

	// done-clean-tree
	tests = append(tests, &testCase{
		_id: "done-clean-tree",
		_expected: &workerObservation{
			branchSubject: "init",
			isCommitTip:   true,
			result:        "/demo go" + fmt.Sprintf(markerInstruction, markerOf("done-clean-tree")),
			sessionId:     "session-1",
			status:        workerStatusDone,
		},
		_shouldPass: true,

		env:  map[string]string{fakeMarkerEnv: markerOf("done-clean-tree")},
		spec: &WorkerSpec{Base: mainBranch, Name: "done-clean-tree", Repo: newTestRepo(t), Skill: "demo", SkillArgs: "go", Timeout: workerTestTimeout},
	})

	// stopped-without-marker
	tests = append(tests, &testCase{
		_id: "stopped-without-marker",
		_expected: &workerObservation{
			branchSubject: "fleet: Recorded worker stopped-without-marker output",
			isCommitTip:   true,
			result:        nudgeOf("stopped-without-marker"),
			sessionId:     "session-1",
			status:        workerStatusStopped,
		},
		_shouldPass: true,

		env:  map[string]string{fakeScriptEnv: "/demo=out.txt=half"},
		spec: &WorkerSpec{Base: mainBranch, Name: "stopped-without-marker", Repo: newTestRepo(t), Skill: "demo", SkillArgs: "go", Timeout: workerTestTimeout},
	})

	// stopped-kept-worktree
	tests = append(tests, &testCase{
		_id: "stopped-kept-worktree",
		_expected: &workerObservation{
			branchSubject: "fleet: Recorded worker stopped-kept-worktree output",
			hasWorktree:   true,
			isCommitTip:   true,
			result:        nudgeOf("stopped-kept-worktree"),
			sessionId:     "session-1",
			status:        workerStatusStopped,
		},
		_shouldPass: true,

		env:  map[string]string{fakeScriptEnv: "/demo=out.txt=half"},
		spec: &WorkerSpec{Base: mainBranch, KeepStopped: true, Name: "stopped-kept-worktree", Repo: newTestRepo(t), Skill: "demo", SkillArgs: "go", Timeout: workerTestTimeout},
	})

	// resume-in-kept-worktree
	resumeRepo := newTestRepo(t)
	keepWorktree(t, resumeRepo, "resume-in-kept-worktree", "claude/resume-in-kept-worktree")
	tests = append(tests, &testCase{
		_id: "resume-in-kept-worktree",
		_expected: &workerObservation{
			branchSubject: "fleet: Recorded worker resume-in-kept-worktree output",
			isCommitTip:   true,
			result:        "go on" + fmt.Sprintf(markerInstruction, markerOf("resume-in-kept-worktree")),
			sessionId:     "session-1",
			status:        workerStatusDone,
		},
		_shouldPass: true,

		env:  map[string]string{fakeMarkerEnv: markerOf("resume-in-kept-worktree"), fakeScriptEnv: "go on=resumed.txt=yes"},
		spec: &WorkerSpec{KeepStopped: true, Name: "resume-in-kept-worktree", Repo: resumeRepo, Resume: workerTestSessionId, Skill: "demo", SkillArgs: "go on", Timeout: workerTestTimeout},
	})

	// resume-prompt-without-slash
	promptRepo := newTestRepo(t)
	keepWorktree(t, promptRepo, "resume-prompt-without-slash", "claude/resume-prompt-without-slash")
	tests = append(tests, &testCase{
		_id: "resume-prompt-without-slash",
		_expected: &workerObservation{
			branchSubject: "init",
			isCommitTip:   true,
			result:        "plan refined, continue" + fmt.Sprintf(markerInstruction, markerOf("resume-prompt-without-slash")),
			sessionId:     "session-1",
			status:        workerStatusDone,
		},
		_shouldPass: true,

		env:  map[string]string{fakeMarkerEnv: markerOf("resume-prompt-without-slash")},
		spec: &WorkerSpec{Name: "resume-prompt-without-slash", Repo: promptRepo, Resume: workerTestSessionId, Skill: "demo", SkillArgs: "plan refined, continue", Timeout: workerTestTimeout},
	})

	// resume-without-worktree
	tests = append(tests, &testCase{
		_id:         "resume-without-worktree",
		_expected:   &workerObservation{},
		_shouldPass: false,

		env:  map[string]string{},
		spec: &WorkerSpec{Name: "resume-without-worktree", Repo: newTestRepo(t), Resume: workerTestSessionId, Skill: "demo", SkillArgs: "go on", Timeout: workerTestTimeout},
	})

	// resume-worktree-on-other-branch
	otherRepo := newTestRepo(t)
	keepWorktree(t, otherRepo, "resume-worktree-on-other-branch", "claude/other")
	tests = append(tests, &testCase{
		_id:         "resume-worktree-on-other-branch",
		_expected:   &workerObservation{hasWorktree: true},
		_shouldPass: false,

		env:  map[string]string{},
		spec: &WorkerSpec{Name: "resume-worktree-on-other-branch", Repo: otherRepo, Resume: workerTestSessionId, Skill: "demo", SkillArgs: "go on", Timeout: workerTestTimeout},
	})

	// resume-done-releases-worktree
	releaseRepo := newTestRepo(t)
	keepWorktree(t, releaseRepo, "resume-done-releases-worktree", "claude/resume-done-releases-worktree")
	tests = append(tests, &testCase{
		_id: "resume-done-releases-worktree",
		_expected: &workerObservation{
			branchSubject: "init",
			isCommitTip:   true,
			result:        "go on" + fmt.Sprintf(markerInstruction, markerOf("resume-done-releases-worktree")),
			sessionId:     "session-1",
			status:        workerStatusDone,
		},
		_shouldPass: true,

		env:  map[string]string{fakeMarkerEnv: markerOf("resume-done-releases-worktree")},
		spec: &WorkerSpec{KeepStopped: true, Name: "resume-done-releases-worktree", Repo: releaseRepo, Resume: workerTestSessionId, Skill: "demo", SkillArgs: "go on", Timeout: workerTestTimeout},
	})

	// failed-exit
	tests = append(tests, &testCase{
		_id: "failed-exit",
		_expected: &workerObservation{
			branchSubject: "fleet: [failed] Recorded worker failed-exit output",
			exitStatus:    3,
			isCommitTip:   true,
			result:        "/demo go" + fmt.Sprintf(markerInstruction, markerOf("failed-exit")),
			sessionId:     "session-1",
			status:        workerStatusFailed,
		},
		_shouldPass: true,

		env:  map[string]string{fakeExitAfterEnv: "1", fakeScriptEnv: "/demo=out.txt=partial"},
		spec: &WorkerSpec{Base: mainBranch, Name: "failed-exit", Repo: newTestRepo(t), Skill: "demo", SkillArgs: "go", Timeout: workerTestTimeout},
	})

	// failed-envelope
	tests = append(tests, &testCase{
		_id: "failed-envelope",
		_expected: &workerObservation{
			branchSubject: "fleet: [failed] Recorded worker failed-envelope output",
			isCommitTip:   true,
			result:        "broke",
			sessionId:     "session-1",
			status:        workerStatusFailed,
		},
		_shouldPass: true,

		env:  map[string]string{fakeEnvelopeEnv: workerTestFailureEnvelope, fakeMarkerEnv: markerOf("failed-envelope"), fakeScriptEnv: "/demo=out.txt=partial"},
		spec: &WorkerSpec{Base: mainBranch, Name: "failed-envelope", Repo: newTestRepo(t), Skill: "demo", SkillArgs: "go", Timeout: workerTestTimeout},
	})

	// commit-body-taken
	tests = append(tests, &testCase{
		_id: "commit-body-taken",
		_expected: &workerObservation{
			branchBody:    "worker body",
			branchSubject: "fleet: Recorded worker commit-body-taken output",
			isCommitTip:   true,
			result:        "/demo go" + fmt.Sprintf(markerInstruction, markerOf("commit-body-taken")),
			sessionId:     "session-1",
			status:        workerStatusDone,
		},
		_shouldPass: true,

		env: map[string]string{
			fakeMarkerEnv: markerOf("commit-body-taken"),
			fakeScriptEnv: "/demo=out.txt=done;/demo=" + commitBodyFileName + "=worker body",
		},
		spec: &WorkerSpec{Base: mainBranch, Name: "commit-body-taken", Repo: newTestRepo(t), Skill: "demo", SkillArgs: "go", Timeout: workerTestTimeout},
	})

	// locked-worktree-removed
	lockedRepo := newTestRepo(t)
	lockedPath := keepWorktree(t, lockedRepo, "locked-worktree-removed", "claude/locked-worktree-removed")
	gitRun(t, lockedRepo, "worktree", "lock", lockedPath)
	tests = append(tests, &testCase{
		_id: "locked-worktree-removed",
		_expected: &workerObservation{
			branchSubject: "init",
			isCommitTip:   true,
			result:        "go on" + fmt.Sprintf(markerInstruction, markerOf("locked-worktree-removed")),
			sessionId:     "session-1",
			status:        workerStatusDone,
		},
		_shouldPass: true,

		env:  map[string]string{fakeMarkerEnv: markerOf("locked-worktree-removed")},
		spec: &WorkerSpec{Name: "locked-worktree-removed", Repo: lockedRepo, Resume: workerTestSessionId, Skill: "demo", SkillArgs: "go on", Timeout: workerTestTimeout},
	})

	// base-unknown
	tests = append(tests, &testCase{
		_id:         "base-unknown",
		_expected:   &workerObservation{},
		_shouldPass: false,

		env:  map[string]string{},
		spec: &WorkerSpec{Base: "no-such-ref", Name: "base-unknown", Repo: newTestRepo(t), Skill: "demo", SkillArgs: "go", Timeout: workerTestTimeout},
	})

	// name-taken
	takenRepo := newTestRepo(t)
	gitRun(t, takenRepo, "branch", "claude/name-taken", mainBranch)
	tests = append(tests, &testCase{
		_id:         "name-taken",
		_expected:   &workerObservation{branchSubject: "init"},
		_shouldPass: false,

		env:  map[string]string{},
		spec: &WorkerSpec{Base: mainBranch, Name: "name-taken", Repo: takenRepo, Skill: "demo", SkillArgs: "go", Timeout: workerTestTimeout},
	})

	// stale-marker-ignored
	staleMarker := markerOf("stale-marker-ignored")
	require.NoError(t, os.MkdirAll(filepath.Dir(staleMarker), 0o755))
	require.NoError(t, os.WriteFile(staleMarker, nil, 0o644))
	tests = append(tests, &testCase{
		_id: "stale-marker-ignored",
		_expected: &workerObservation{
			branchSubject: "init",
			isCommitTip:   true,
			result:        nudgeOf("stale-marker-ignored"),
			sessionId:     "session-1",
			status:        workerStatusStopped,
		},
		_shouldPass: true,

		env:  map[string]string{},
		spec: &WorkerSpec{Base: mainBranch, Name: "stale-marker-ignored", Repo: newTestRepo(t), Skill: "demo", SkillArgs: "go", Timeout: workerTestTimeout},
	})

	// prompt-shape
	tests = append(tests, &testCase{
		_id: "prompt-shape",
		_expected: &workerObservation{
			branchSubject: "init",
			isCommitTip:   true,
			result:        "/demo plans/x/design/x.md --nightly" + fmt.Sprintf(markerInstruction, markerOf("prompt-shape")),
			sessionId:     "session-1",
			status:        workerStatusDone,
		},
		_shouldPass: true,

		env:  map[string]string{fakeMarkerEnv: markerOf("prompt-shape")},
		spec: &WorkerSpec{Base: mainBranch, Name: "prompt-shape", Repo: newTestRepo(t), Skill: "demo", SkillArgs: "plans/x/design/x.md --nightly", Timeout: workerTestTimeout},
	})

	// marker-env-default
	tests = append(tests, &testCase{
		_id: "marker-env-default",
		_expected: &workerObservation{
			branchSubject: "init",
			isCommitTip:   true,
			result:        "1",
			sessionId:     "session-1",
			status:        workerStatusDone,
		},
		_shouldPass: true,

		env:  map[string]string{fakeEchoEnv: headlessEnvKey, fakeMarkerEnv: markerOf("marker-env-default")},
		spec: &WorkerSpec{Base: mainBranch, Name: "marker-env-default", Repo: newTestRepo(t), Skill: "demo", SkillArgs: "go", Timeout: workerTestTimeout},
	})

	// marker-env-attended
	tests = append(tests, &testCase{
		_id: "marker-env-attended",
		_expected: &workerObservation{
			branchSubject: "init",
			isCommitTip:   true,
			result:        "",
			sessionId:     "session-1",
			status:        workerStatusDone,
		},
		_shouldPass: true,

		env:  map[string]string{fakeEchoEnv: headlessEnvKey, fakeMarkerEnv: markerOf("marker-env-attended")},
		spec: &WorkerSpec{Attended: true, Base: mainBranch, Name: "marker-env-attended", Repo: newTestRepo(t), Skill: "demo", SkillArgs: "go", Timeout: workerTestTimeout},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			setTestEnv(t, test.env)

			result, err := RunWorker(ctx, test.spec)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, observeWorker(t, test.spec, result))
		})
	}
}

func TestWorkerSpec_claudeArgs(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		spec *WorkerSpec
	}

	home := t.TempDir()
	writeTestSkill(t, home, "demo", "allowed-tools: Read, Bash(jq *)\n")
	writeTestSkill(t, home, "bare", "")
	t.Setenv("HOME", home)
	tests := make([]*testCase, 0)

	// skill-grants-plus-extras
	tests = append(tests, &testCase{
		_id:       "skill-grants-plus-extras",
		_expected: []string{"--allowedTools", "Read, Bash(jq *), Workflow, Skill, Write"},

		spec: &WorkerSpec{Skill: "demo"},
	})

	// skill-without-grants
	tests = append(tests, &testCase{
		_id:       "skill-without-grants",
		_expected: []string{"--allowedTools", "Workflow, Skill, Write"},

		spec: &WorkerSpec{Skill: "bare"},
	})

	// resume-flag
	tests = append(tests, &testCase{
		_id:       "resume-flag",
		_expected: []string{"--allowedTools", "Read, Bash(jq *), Workflow, Skill, Write", "--resume", workerTestSessionId},

		spec: &WorkerSpec{Resume: workerTestSessionId, Skill: "demo"},
	})

	// knobs-set
	tests = append(tests, &testCase{
		_id: "knobs-set",
		_expected: []string{
			"--allowedTools", "Read, Bash(jq *), Workflow, Skill, Write",
			"--model", "sonnet",
			"--effort", "high",
			"--max-budget-usd", "5",
		},

		spec: &WorkerSpec{BudgetUsd: "5", Effort: "high", Model: "sonnet", Skill: "demo"},
	})

	// knobs-unset
	tests = append(tests, &testCase{
		_id:       "knobs-unset",
		_expected: []string{"--allowedTools", "Read, Bash(jq *), Workflow, Skill, Write"},

		spec: &WorkerSpec{BudgetUsd: "", Effort: "", Model: "", Resume: "", Skill: "demo"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.spec.claudeArgs())
		})
	}
}

func TestWorkerSpec_validate(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool

		headless string
		spec     *WorkerSpec
	}

	tests := make([]*testCase, 0)

	// pass-minimal
	tests = append(tests, &testCase{
		_id:         "pass-minimal",
		_shouldPass: true,

		headless: "",
		spec:     &WorkerSpec{Base: mainBranch, Name: "t1-plan", Skill: "fimplement"},
	})

	// pass-attended-unmarked
	tests = append(tests, &testCase{
		_id:         "pass-attended-unmarked",
		_shouldPass: true,

		headless: "",
		spec:     &WorkerSpec{Attended: true, Base: mainBranch, Name: "t1-plan", Skill: "fimplement"},
	})

	// pass-resume-without-base
	tests = append(tests, &testCase{
		_id:         "pass-resume-without-base",
		_shouldPass: true,

		headless: "",
		spec:     &WorkerSpec{Base: "", Name: "t1-plan", Resume: workerTestSessionId, Skill: "fimplement"},
	})

	// fail-empty-name
	tests = append(tests, &testCase{
		_id:         "fail-empty-name",
		_shouldPass: false,

		headless: "",
		spec:     &WorkerSpec{Base: mainBranch, Name: "", Skill: "fimplement"},
	})

	// fail-uppercase-name
	tests = append(tests, &testCase{
		_id:         "fail-uppercase-name",
		_shouldPass: false,

		headless: "",
		spec:     &WorkerSpec{Base: mainBranch, Name: "T1-plan", Skill: "fimplement"},
	})

	// fail-name-with-slash
	tests = append(tests, &testCase{
		_id:         "fail-name-with-slash",
		_shouldPass: false,

		headless: "",
		spec:     &WorkerSpec{Base: mainBranch, Name: "t1/plan", Skill: "fimplement"},
	})

	// fail-empty-base
	tests = append(tests, &testCase{
		_id:         "fail-empty-base",
		_shouldPass: false,

		headless: "",
		spec:     &WorkerSpec{Base: "", Name: "t1-plan", Skill: "fimplement"},
	})

	// fail-resume-not-a-session-id
	tests = append(tests, &testCase{
		_id:         "fail-resume-not-a-session-id",
		_shouldPass: false,

		headless: "",
		spec:     &WorkerSpec{Name: "t1-plan", Resume: "--dangerously-skip-permissions", Skill: "fimplement"},
	})

	// fail-empty-skill
	tests = append(tests, &testCase{
		_id:         "fail-empty-skill",
		_shouldPass: false,

		headless: "",
		spec:     &WorkerSpec{Base: mainBranch, Name: "t1-plan", Skill: ""},
	})

	// fail-skill-with-slash
	tests = append(tests, &testCase{
		_id:         "fail-skill-with-slash",
		_shouldPass: false,

		headless: "",
		spec:     &WorkerSpec{Base: mainBranch, Name: "t1-plan", Skill: "/fimplement"},
	})

	// fail-budget-not-a-number
	tests = append(tests, &testCase{
		_id:         "fail-budget-not-a-number",
		_shouldPass: false,

		headless: "",
		spec:     &WorkerSpec{Base: mainBranch, BudgetUsd: "five", Name: "t1-plan", Skill: "fimplement"},
	})

	// fail-attended-under-marker
	tests = append(tests, &testCase{
		_id:         "fail-attended-under-marker",
		_shouldPass: false,

		headless: "1",
		spec:     &WorkerSpec{Attended: true, Base: mainBranch, Name: "t1-plan", Skill: "fimplement"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			t.Setenv(headlessEnvKey, test.headless)

			err := test.spec.validate()

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
		})
	}
}

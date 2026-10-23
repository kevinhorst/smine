package heartbeat

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// documentFixture is a real `bin/heartbeat status --fleet T1 --format json`
// capture with P1–P3 running and T1 subscribed by one session: four spawn-tree
// workers (the first stopped, with an isolation worktree row carrying `agents`),
// work items, collisions and the one train of the target `main`, the default
// branch.
func documentFixture(t *testing.T) string {
	t.Helper()
	fixture, err := os.ReadFile(filepath.Join("testdata", "status_document.json"))
	require.NoError(t, err)
	return string(fixture)
}

// writeHeartbeatStub creates a stub heartbeat binary that records its argv in
// `argv` beside itself, prints output and exits with exitCode.
func writeHeartbeatStub(t *testing.T, exitCode int, output string) string {
	t.Helper()
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "output")
	require.NoError(t, os.WriteFile(outputPath, []byte(output), 0o644))
	script := "#!/bin/sh\n" +
		`printf '%s' "$*" > "` + filepath.Join(dir, "argv") + "\"\n" +
		`cat "` + outputPath + "\"\n" +
		"exit " + strconv.Itoa(exitCode) + "\n"
	stub := filepath.Join(dir, "heartbeat")
	require.NoError(t, os.WriteFile(stub, []byte(script), 0o755))
	return stub
}

func readArgv(t *testing.T, stub string) string {
	t.Helper()
	argv, err := os.ReadFile(filepath.Join(filepath.Dir(stub), "argv"))
	require.NoError(t, err)
	return string(argv)
}

func TestDecodeDocument(t *testing.T) {
	document, err := decodeDocument(documentFixture(t))
	require.NoError(t, err)

	assert.Equal(t, "/Users/devuser/.claude", document.Heartbeat.ClaudeHome)
	assert.Equal(t, "127.0.0.1:6101", document.Heartbeat.Addr)
	assert.NotEmpty(t, document.Sensors)
	for _, sensor := range document.Sensors {
		assert.Contains(t, []SensorState{SensorStateOk, SensorStateDegraded}, sensor.State, sensor.Name)
	}

	require.Len(t, document.Sessions, 1)
	session := document.Sessions[0]
	assert.Equal(t, "bd080288-be15-4ddb-b88f-993e49c1ffd9", session.Id)
	assert.Equal(t, SessionStateRunning, session.State)
	assert.Equal(t, "CC,FImpl - P3 orchestrator loop acceptance tests - [READY]", session.Title)
	assert.Nil(t, session.Blocked)
	require.Len(t, session.Subagents, 4)
	assert.Equal(t, SubagentStateStopped, session.Subagents[0].State)
	assert.Equal(t, SubagentStateRunning, session.Subagents[1].State)

	require.Len(t, document.Repos, 1)
	assert.Equal(t, "claude-configs", document.Repos[0].Name)
	assert.Empty(t, document.Repos[0].ScanError)

	require.Len(t, document.Fleets, 1)
	fleet := document.Fleets[0]
	assert.Equal(t, "T1", fleet.Id)
	assert.Equal(t, "/Users/devuser/GolandProjects/claude-configs", fleet.Repo)
	assert.Equal(t, "main", fleet.Target)
	assert.Equal(t, TargetStateOpen, fleet.TargetState)
	assert.Equal(t, 5, fleet.MemberCount)

	require.Len(t, document.Members, 5)
	assert.Equal(t, Via("subscriber"), document.Members[0].Via)
	assert.Equal(t, "claude/agent-a3ede1d975161fb7f", document.Members[1].Branch)
	assert.Equal(t, Via("worker"), document.Members[1].Via)

	require.NotEmpty(t, document.Branches)
	var agentBranch *Branch
	targets := make(map[string]string, len(document.Branches))
	for i := range document.Branches {
		assert.Contains(t, branchStates(), document.Branches[i].State, document.Branches[i].Branch)
		targets[document.Branches[i].Branch] = document.Branches[i].Target
		if len(document.Branches[i].Agents) > 0 {
			agentBranch = &document.Branches[i]
		}
	}
	require.NotNil(t, agentBranch)
	assert.Equal(t, "claude/agent-a3ede1d975161fb7f", agentBranch.Branch)
	assert.Equal(t, BranchStateUncommitted, agentBranch.State)
	assert.Equal(t, BranchAgent{AgentId: "a3ede1d975161fb7f", SessionId: session.Id}, agentBranch.Agents[0])
	assert.Empty(t, agentBranch.Target, "a row without target decodes empty")
	assert.Equal(t, "feature/x", targets["claude/heartbeat-test-session-9a6ef8"])

	require.NotEmpty(t, document.WorkItems)
	assert.NotEmpty(t, document.WorkItems[0].PlannedFiles)
	require.NotEmpty(t, document.Collisions)
	for _, collision := range document.Collisions {
		assert.Contains(t, collisionLevels(), collision.Level)
	}

	require.Len(t, document.Trains, 1, "one train per base, the default branch included")
	assert.Equal(t, "main", document.Trains[0].Base)
	require.NotEmpty(t, document.Trains[0].Steps)
	assert.Equal(t, 1, document.Trains[0].Steps[0].Position)

	assert.Equal(t, "develop", document.StagingTarget)

	require.Len(t, document.Landings, 3, "one staging row per registered repo plus each held target")
	absent := document.Landings[0]
	assert.Equal(t, "/Users/devuser/GolandProjects/agent-plans", absent.Repo)
	assert.Nil(t, absent.Holder, "a free turn has no holder")
	assert.Empty(t, absent.Queue)
	require.NotNil(t, absent.Release, "the staging row carries release facts")
	assert.Equal(t, ReleaseStateAbsent, absent.Release.State())

	staging := document.Landings[1]
	assert.Equal(t, "/Users/devuser/GolandProjects/claude-configs", staging.Repo)
	assert.Equal(t, document.StagingTarget, staging.Branch)
	require.NotNil(t, staging.Release)
	assert.Equal(t, 3, staging.Release.Unreleased)
	assert.Equal(t, ReleaseStateReleasable, staging.Release.State())
	require.NotNil(t, staging.Holder)
	assert.Equal(t, "83985202-8be5-450e-8d92-586ced8742d5", staging.Holder.SessionId)
	assert.Equal(t, "CC,FD - Permission jailbreak fix continuation - [DONE]", staging.Holder.Title)
	assert.Equal(t, "2026-10-01T14:00:00Z", staging.Holder.Since)
	require.Len(t, staging.Queue, 1)
	assert.Empty(t, staging.Queue[0].Title, "absent until the registry named the session")

	feature := document.Landings[2]
	assert.Equal(t, "feature/x", feature.Branch)
	assert.Nil(t, feature.Release, "only the staging row carries release facts")
	require.NotNil(t, feature.Holder)
	assert.Equal(t, "7c1e9a40-2b3d-4f5e-8a6b-9c0d1e2f3a4b", feature.Holder.SessionId)
}

func TestDecodeDocumentRejectsUnknownSchema(t *testing.T) {
	_, err := decodeDocument(`{"schema":2}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Unsupported schema 2")
}

func TestDecodeDocumentRejectsMalformed(t *testing.T) {
	_, err := decodeDocument("not json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Decode")
}

func TestDecodeDocumentP1Only(t *testing.T) {
	output := `{"schema":1,` +
		`"heartbeat":{"version":"0.1.0","claude_home":"/h","addr":"127.0.0.1:6101","started_at":"2026-09-27T10:00:00Z","events_applied":3},` +
		`"sensors":[{"name":"hooks","state":"ok","since":"2026-09-27T10:00:00Z"}],` +
		`"sessions":[{"session_id":"s1","state":"blocked","state_since":"2026-09-27T10:01:00Z","since_restart":false,` +
		`"blocked":{"program":"Bash:ps","candidate_rule":"Bash(ps *)","denied_at":"2026-09-27T10:00:59Z"},"subagents":[]}]}`
	document, err := decodeDocument(output)
	require.NoError(t, err)
	assert.Empty(t, document.Fleets)
	assert.Empty(t, document.Members)
	assert.Empty(t, document.Repos)
	assert.Empty(t, document.Branches)
	assert.Empty(t, document.Collisions)
	assert.Empty(t, document.Landings)
	require.Len(t, document.Sessions, 1)
	require.NotNil(t, document.Sessions[0].Blocked)
	assert.Equal(t, "Bash(ps *)", document.Sessions[0].Blocked.CandidateRule)
}

func TestDecodeDocumentOpenEnums(t *testing.T) {
	output := `{"schema":1,"future_section":[1],` +
		`"fleets":[{"id":"T1","members":1,"target_state":"paused","foreign":[],"subscriptions":[]}],` +
		`"members":[{"fleet":"T1","session_id":"s1","via":"adopted"}],` +
		`"branches":[{"repo":"/r","branch":"b","state":"quarantined","since":"x","target":"main","sessions":[],"work_items":[]}]}`
	document, err := decodeDocument(output)
	require.NoError(t, err)
	assert.Equal(t, TargetState("paused"), document.Fleets[0].TargetState)
	assert.Equal(t, ToneDim, document.Fleets[0].TargetState.Tone())
	assert.Equal(t, Via("adopted"), document.Members[0].Via)
	assert.Equal(t, ToneDim, document.Branches[0].State.Tone())
	assert.Equal(t, ToneDim, ReleaseState("paused").Tone())
}

func TestRelease_State(t *testing.T) {
	type testCase struct {
		_id           string
		_expected     ReleaseState
		_expectedTone Tone

		release *Release
	}

	tests := make([]*testCase, 0)

	// absent
	tests = append(tests, &testCase{
		_id:           "absent",
		_expected:     ReleaseStateAbsent,
		_expectedTone: ToneDim,

		release: &Release{Exists: false, Releasable: false},
	})

	// absent-wins-over-releasable
	tests = append(tests, &testCase{
		_id:           "absent-wins-over-releasable",
		_expected:     ReleaseStateAbsent,
		_expectedTone: ToneDim,

		release: &Release{Exists: false, Releasable: true},
	})

	// releasable
	tests = append(tests, &testCase{
		_id:           "releasable",
		_expected:     ReleaseStateReleasable,
		_expectedTone: ToneOk,

		release: &Release{Exists: true, Releasable: true},
	})

	// diverged
	tests = append(tests, &testCase{
		_id:           "diverged",
		_expected:     ReleaseStateDiverged,
		_expectedTone: ToneAction,

		release: &Release{Exists: true, Releasable: false},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			state := test.release.State()
			assert.Equal(t, test._expected, state)
			assert.Equal(t, test._expectedTone, state.Tone())
		})
	}
}

func TestCli_Status(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedArgv   string
		_expectedDetail string
		_isForeignHome  bool
		_isUnreachable  bool
		_shouldPass     bool

		cli   *Cli
		fleet string
	}

	const baseArgv = "--addr 127.0.0.1:6101 --claude-home /h status --format json"
	cliFor := func(stub string) *Cli {
		return &Cli{Addr: "127.0.0.1:6101", BinaryPath: stub, ClaudeHome: "/h"}
	}

	tests := make([]*testCase, 0)

	// ok-exit-0
	tests = append(tests, &testCase{
		_id:             "ok-exit-0",
		_expectedArgv:   baseArgv,
		_expectedDetail: "<nil>",
		_isForeignHome:  false,
		_isUnreachable:  false,
		_shouldPass:     true,

		cli:   cliFor(writeHeartbeatStub(t, 0, documentFixture(t))),
		fleet: "",
	})

	// unreachable-exit-3
	tests = append(tests, &testCase{
		_id:             "unreachable-exit-3",
		_expectedArgv:   baseArgv,
		_expectedDetail: "heartbeat: no daemon on 127.0.0.1:6101",
		_isForeignHome:  false,
		_isUnreachable:  true,
		_shouldPass:     false,

		cli:   cliFor(writeHeartbeatStub(t, 3, "heartbeat: no daemon on 127.0.0.1:6101\n")),
		fleet: "",
	})

	// foreign-exit-4
	tests = append(tests, &testCase{
		_id:             "foreign-exit-4",
		_expectedArgv:   baseArgv,
		_expectedDetail: "serves /other",
		_isForeignHome:  true,
		_isUnreachable:  false,
		_shouldPass:     false,

		cli:   cliFor(writeHeartbeatStub(t, 4, "heartbeat: 127.0.0.1:6101 serves /other\n")),
		fleet: "",
	})

	// invalid-fleet-exit-2
	tests = append(tests, &testCase{
		_id:             "invalid-fleet-exit-2",
		_expectedArgv:   baseArgv + " --fleet T 1",
		_expectedDetail: "must be 1-62 characters",
		_isForeignHome:  false,
		_isUnreachable:  false,
		_shouldPass:     false,

		cli:   cliFor(writeHeartbeatStub(t, 2, "error: invalid value 'T 1' for '--fleet': must be 1-62 characters\n")),
		fleet: "T 1",
	})

	// refused-exit-1
	tests = append(tests, &testCase{
		_id:             "refused-exit-1",
		_expectedArgv:   baseArgv,
		_expectedDetail: "409 Conflict",
		_isForeignHome:  false,
		_isUnreachable:  false,
		_shouldPass:     false,

		cli:   cliFor(writeHeartbeatStub(t, 1, "heartbeat: 409 Conflict: refused\n")),
		fleet: "",
	})

	// fleet-flag
	tests = append(tests, &testCase{
		_id:             "fleet-flag",
		_expectedArgv:   baseArgv + " --fleet T1",
		_expectedDetail: "<nil>",
		_isForeignHome:  false,
		_isUnreachable:  false,
		_shouldPass:     true,

		cli:   cliFor(writeHeartbeatStub(t, 0, documentFixture(t))),
		fleet: "T1",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			document, err := test.cli.Status(context.Background(), test.fleet)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._shouldPass, document != nil)
			assert.Equal(t, test._isUnreachable, errors.Is(err, ErrUnreachable))
			assert.Equal(t, test._isForeignHome, errors.Is(err, ErrForeignHome))
			assert.Contains(t, fmt.Sprint(err), test._expectedDetail)
			assert.Equal(t, test._expectedArgv, readArgv(t, test.cli.BinaryPath))
		})
	}
}

func branchStates() []BranchState {
	return []BranchState{
		BranchStateConflicting,
		BranchStateMergeReady,
		BranchStateMerged,
		BranchStateNoWork,
		BranchStateRemovable,
		BranchStateUncommitted,
		BranchStateUnmerged,
	}
}

func collisionLevels() []CollisionLevel {
	return []CollisionLevel{
		CollisionLevelConfirmed,
		CollisionLevelPlannedFile,
		CollisionLevelPlannedSymbol,
		CollisionLevelTouched,
	}
}

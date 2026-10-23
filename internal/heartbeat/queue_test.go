package heartbeat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQueue(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []QueueItem

		document *Document
	}

	fleet := func(state TargetState) Fleet {
		return Fleet{Id: "T1", MemberCount: 1, MergeBranch: "claude-orch/T1", Repo: "/r", Target: "feature/f3", TargetState: state}
	}
	sessionMember := Member{Fleet: "T1", SessionId: "s1", Via: ViaJoined}
	waiting := Session{
		Id:         "s1",
		State:      SessionStateWaiting,
		StateSince: "2026-09-27T10:00:00Z",
		Title:      "CC,FImpl - Worker - T1[WIP]",
		WaitingFor: "permission prompt",
	}
	blocked := Session{
		Id:         "s1",
		Blocked:    &Blocked{CandidateRule: "Bash(ps *)", DeniedAt: "2026-09-27T10:00:59Z", Program: "Bash:ps"},
		State:      SessionStateBlocked,
		StateSince: "2026-09-27T10:01:00Z",
		Title:      "CC,FImpl - Worker - T1[WIP]",
	}
	idle := Session{Id: "s1", State: SessionStateIdle, Title: "CC,FImpl - Worker - T1[WIP]"}
	idleSince := Session{Id: "s1", State: SessionStateIdle, StateSince: "2026-09-27T10:02:00Z", Title: "CC,FD - Design - T1-MA[READY]"}
	running := Session{Id: "s1", State: SessionStateRunning, StateSince: "2026-09-27T10:02:00Z", Title: "CC,FD - Design - T1-MA[WIP]"}
	joinedMember := Member{Branch: "claude/w", Fleet: "T1", MemberId: "A", SessionId: "s1", Token: "T1-MA", Via: ViaJoined}
	deliveredMember := Member{Branch: "claude/w", Delivered: true, Fleet: "T1", MemberId: "A", SessionId: "s1", Token: "T1-MA", Via: ViaJoined}
	mergeReady := Branch{Branch: "claude/w", Repo: "/r", State: BranchStateMergeReady, Target: "feature/f3"}
	waitingItem := QueueItem{
		Id:      "T1/waiting/s1",
		Detail:  "permission prompt",
		Fleet:   "T1",
		Kind:    QueueKindWaiting,
		Since:   "2026-09-27T10:00:00Z",
		Subject: "CC,FImpl - Worker - T1[WIP]",
	}

	tests := make([]*testCase, 0)

	// no-fleets
	tests = append(tests, &testCase{
		_id:       "no-fleets",
		_expected: []QueueItem{},

		document: &Document{Sessions: []Session{waiting}},
	})

	// waiting-member
	tests = append(tests, &testCase{
		_id:       "waiting-member",
		_expected: []QueueItem{waitingItem},

		document: &Document{Fleets: []Fleet{fleet(TargetStateOpen)}, Members: []Member{sessionMember}, Sessions: []Session{waiting}},
	})

	// blocked-member
	tests = append(tests, &testCase{
		_id: "blocked-member",
		_expected: []QueueItem{{
			Id:      "T1/permission/s1",
			Detail:  "Bash(ps *)",
			Fleet:   "T1",
			Kind:    QueueKindPermission,
			Since:   "2026-09-27T10:00:59Z",
			Subject: "CC,FImpl - Worker - T1[WIP]",
		}},

		document: &Document{Fleets: []Fleet{fleet(TargetStateOpen)}, Members: []Member{sessionMember}, Sessions: []Session{blocked}},
	})

	// idle-member
	tests = append(tests, &testCase{
		_id:       "idle-member",
		_expected: []QueueItem{},

		document: &Document{Fleets: []Fleet{fleet(TargetStateOpen)}, Members: []Member{sessionMember}, Sessions: []Session{idle}},
	})

	// non-member-waiting
	tests = append(tests, &testCase{
		_id:       "non-member-waiting",
		_expected: []QueueItem{},

		document: &Document{Fleets: []Fleet{fleet(TargetStateOpen)}, Sessions: []Session{waiting}},
	})

	// other-fleet-member-waiting
	tests = append(tests, &testCase{
		_id:       "other-fleet-member-waiting",
		_expected: []QueueItem{},

		document: &Document{
			Fleets:   []Fleet{fleet(TargetStateOpen)},
			Members:  []Member{{Fleet: "T2", SessionId: "s1", Via: ViaJoined}},
			Sessions: []Session{waiting},
		},
	})

	// worker-member
	tests = append(tests, &testCase{
		_id:       "worker-member",
		_expected: []QueueItem{},

		document: &Document{
			Fleets:   []Fleet{fleet(TargetStateOpen)},
			Members:  []Member{{Branch: "claude/t1-alpha", Fleet: "T1", MemberId: "B", SessionId: "s1", Via: ViaWorker}},
			Sessions: []Session{waiting},
		},
	})

	// integrated-ready-to-sync
	tests = append(tests, &testCase{
		_id: "integrated-ready-to-sync",
		_expected: []QueueItem{{
			Id:      "T1/ready_to_sync",
			Detail:  "all member work merged into claude-orch/T1 — the orchestrator syncs the target next",
			Fleet:   "T1",
			Kind:    QueueKindReadyToSync,
			Subject: "feature/f3",
		}},

		document: &Document{Fleets: []Fleet{fleet(TargetStateIntegrated)}},
	})

	// integrated-sync-blocked
	blockedFleet := fleet(TargetStateIntegrated)
	blockedFleet.Journal = []JournalEntry{
		{At: "2026-09-28T10:00:00Z", By: "o", Kind: "merged", Members: []string{"A"}, Subject: "merged claude/w"},
		{At: "2026-09-28T11:00:00Z", By: "o", Kind: "note", Subject: "sync blocked: feature/f3 checked out at /r"},
	}
	tests = append(tests, &testCase{
		_id: "integrated-sync-blocked",
		_expected: []QueueItem{{
			Id:      "T1/sync_blocked",
			Detail:  "sync blocked: feature/f3 checked out at /r",
			Fleet:   "T1",
			Kind:    QueueKindSyncBlocked,
			Since:   "2026-09-28T11:00:00Z",
			Subject: "feature/f3",
		}},

		document: &Document{Fleets: []Fleet{blockedFleet}},
	})

	// sync-blocked-superseded-by-merge
	supersededFleet := fleet(TargetStateIntegrated)
	supersededFleet.Journal = []JournalEntry{
		{At: "2026-09-28T11:00:00Z", By: "o", Kind: "note", Subject: "sync blocked: feature/f3 checked out at /r"},
		{At: "2026-09-28T12:00:00Z", By: "o", Kind: "merged", Members: []string{"B"}, Subject: "merged claude/x"},
	}
	tests = append(tests, &testCase{
		_id:       "sync-blocked-superseded-by-merge",
		_expected: []QueueItem{{Id: "T1/ready_to_sync", Detail: "all member work merged into claude-orch/T1 — the orchestrator syncs the target next", Fleet: "T1", Kind: QueueKindReadyToSync, Subject: "feature/f3"}},

		document: &Document{Fleets: []Fleet{supersededFleet}},
	})

	// integrated-synced
	syncedFleet := fleet(TargetStateIntegrated)
	syncedFleet.Synced = true
	tests = append(tests, &testCase{
		_id: "integrated-synced",
		_expected: []QueueItem{{
			Id:      "T1/ready_for_main",
			Detail:  "all fleet work synced to the target — landing on main is human",
			Fleet:   "T1",
			Kind:    QueueKindReadyForMain,
			Subject: "feature/f3",
		}},

		document: &Document{Fleets: []Fleet{syncedFleet}},
	})

	// missing
	tests = append(tests, &testCase{
		_id: "missing",
		_expected: []QueueItem{{
			Id:      "T1/target_missing",
			Detail:  "integration branch is no local branch in the repo's last scan",
			Fleet:   "T1",
			Kind:    QueueKindTargetMissing,
			Subject: "feature/f3",
		}},

		document: &Document{Fleets: []Fleet{fleet(TargetStateMissing)}},
	})

	// open
	tests = append(tests, &testCase{
		_id:       "open",
		_expected: []QueueItem{},

		document: &Document{Fleets: []Fleet{fleet(TargetStateOpen)}},
	})

	// awaiting-done
	tests = append(tests, &testCase{
		_id: "awaiting-done",
		_expected: []QueueItem{{
			Id:      "T1/awaiting_done/s1",
			Detail:  "branch claude/w is merge-ready; the member has not run /done",
			Fleet:   "T1",
			Kind:    QueueKindAwaitingDone,
			Since:   "2026-09-27T10:02:00Z",
			Subject: "T1-MA",
		}},

		document: &Document{
			Branches: []Branch{mergeReady},
			Fleets:   []Fleet{fleet(TargetStateOpen)},
			Members:  []Member{joinedMember},
			Sessions: []Session{idleSince},
		},
	})

	// awaiting-done-delivered
	tests = append(tests, &testCase{
		_id:       "awaiting-done-delivered",
		_expected: []QueueItem{},

		document: &Document{
			Branches: []Branch{mergeReady},
			Fleets:   []Fleet{fleet(TargetStateOpen)},
			Members:  []Member{deliveredMember},
			Sessions: []Session{idleSince},
		},
	})

	// awaiting-done-running
	tests = append(tests, &testCase{
		_id:       "awaiting-done-running",
		_expected: []QueueItem{},

		document: &Document{
			Branches: []Branch{mergeReady},
			Fleets:   []Fleet{fleet(TargetStateOpen)},
			Members:  []Member{joinedMember},
			Sessions: []Session{running},
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, Queue(test.document))
		})
	}
}

func TestDocument_MemberRows(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []MemberRow

		document *Document
		fleet    *Fleet
	}

	fleet := &Fleet{Id: "T1", MemberCount: 2, Repo: "/r1", Target: "main", TargetState: TargetStateOpen}
	worker := Session{
		Id:          "s1",
		ActiveSkill: "fimplement",
		State:       SessionStateRunning,
		StateSince:  "2026-09-27T10:00:00Z",
		Title:       "CC,FImpl - Worker - T1[WIP]",
	}
	branch := func(repo, name string, state BranchState) Branch {
		return Branch{Branch: name, Repo: repo, State: state, Target: "main", WorkItems: []string{"2026-09-27-x-wip-abc1234"}}
	}
	workerMember := Member{Branch: "claude/t1-alpha", Fleet: "T1", MemberId: "B", SessionId: "s1", Via: ViaWorker}
	deliveredWorker := Member{Branch: "claude/t1-alpha", Delivered: true, Fleet: "T1", MemberId: "B", SessionId: "s1", Via: ViaWorker}

	tests := make([]*testCase, 0)

	// session-member
	tests = append(tests, &testCase{
		_id: "session-member",
		_expected: []MemberRow{{
			Id:        "s1",
			Name:      "CC,FImpl - Worker - T1[WIP]",
			Role:      "member",
			SessionId: "s1",
			Since:     "2026-09-27T10:00:00Z",
			Skill:     "fimplement",
			State:     "running",
			Title:     "CC,FImpl - Worker - T1[WIP]",
			Tone:      ToneAction,
			Via:       ViaJoined,
		}},

		document: &Document{Members: []Member{{Fleet: "T1", SessionId: "s1", Via: ViaJoined}}, Sessions: []Session{worker}},
		fleet:    fleet,
	})

	// worker-running
	tests = append(tests, &testCase{
		_id: "worker-running",
		_expected: []MemberRow{{
			Id:        "claude/t1-alpha",
			Branch:    "claude/t1-alpha",
			MemberId:  "B",
			Name:      "t1-alpha",
			Role:      "worker",
			SessionId: "s1",
			State:     "running",
			Tone:      ToneAction,
			Via:       ViaWorker,
		}},

		document: &Document{Members: []Member{workerMember}, Sessions: []Session{worker}},
		fleet:    fleet,
	})

	// worker-delivered
	tests = append(tests, &testCase{
		_id: "worker-delivered",
		_expected: []MemberRow{{
			Id:        "claude/t1-alpha",
			Branch:    "claude/t1-alpha",
			Delivered: true,
			MemberId:  "B",
			Name:      "t1-alpha",
			Role:      "worker",
			SessionId: "s1",
			State:     "ended",
			Tone:      ToneDim,
			Via:       ViaWorker,
		}},

		document: &Document{Members: []Member{deliveredWorker}, Sessions: []Session{worker}},
		fleet:    fleet,
	})

	// worker-branch-state
	tests = append(tests, &testCase{
		_id: "worker-branch-state",
		_expected: []MemberRow{{
			Id:          "claude/t1-alpha",
			Branch:      "claude/t1-alpha",
			BranchState: BranchStateMergeReady,
			MemberId:    "B",
			Name:        "t1-alpha",
			Role:        "worker",
			SessionId:   "s1",
			State:       "running",
			Tone:        ToneAction,
			Via:         ViaWorker,
			WorkItem:    "2026-09-27-x-wip-abc1234",
		}},

		document: &Document{
			Branches: []Branch{branch("/r1", "claude/t1-alpha", BranchStateMergeReady)},
			Members:  []Member{workerMember},
			Sessions: []Session{worker},
		},
		fleet: fleet,
	})

	// same-branch-other-repo
	tests = append(tests, &testCase{
		_id: "same-branch-other-repo",
		_expected: []MemberRow{{
			Id:        "claude/t1-alpha",
			Branch:    "claude/t1-alpha",
			MemberId:  "B",
			Name:      "t1-alpha",
			Role:      "worker",
			SessionId: "s1",
			State:     "running",
			Tone:      ToneAction,
			Via:       ViaWorker,
		}},

		document: &Document{
			Branches: []Branch{branch("/r2", "claude/t1-alpha", BranchStateMergeReady)},
			Members:  []Member{workerMember},
			Sessions: []Session{worker},
		},
		fleet: fleet,
	})

	// unknown-via
	tests = append(tests, &testCase{
		_id: "unknown-via",
		_expected: []MemberRow{{
			Id:        "s1",
			Branch:    "claude/w",
			MemberId:  "C",
			Name:      "T1-MC",
			Role:      "adopted",
			SessionId: "s1",
			Token:     "T1-MC",
			Via:       "adopted",
		}},

		document: &Document{
			Members:  []Member{{Branch: "claude/w", Fleet: "T1", MemberId: "C", SessionId: "s1", Token: "T1-MC", Via: "adopted"}},
			Sessions: []Session{worker},
		},
		fleet: fleet,
	})

	// no-branch
	tests = append(tests, &testCase{
		_id: "no-branch",
		_expected: []MemberRow{{
			Id:        "s1",
			Name:      "CC,FImpl - Worker - T1[WIP]",
			Role:      "member",
			SessionId: "s1",
			Since:     "2026-09-27T10:00:00Z",
			Skill:     "fimplement",
			State:     "running",
			Title:     "CC,FImpl - Worker - T1[WIP]",
			Tone:      ToneAction,
			Via:       ViaJoined,
		}},

		document: &Document{
			Branches: []Branch{branch("/r1", "", BranchStateRemovable)},
			Members:  []Member{{Fleet: "T1", SessionId: "s1", Via: ViaJoined}},
			Sessions: []Session{worker},
		},
		fleet: fleet,
	})

	// other-fleet-member
	tests = append(tests, &testCase{
		_id:       "other-fleet-member",
		_expected: []MemberRow{},

		document: &Document{Members: []Member{{Fleet: "T2", SessionId: "s1", Via: ViaJoined}}, Sessions: []Session{worker}},
		fleet:    fleet,
	})

	// unknown-session
	tests = append(tests, &testCase{
		_id:       "unknown-session",
		_expected: []MemberRow{{Id: "s9", Role: "member", SessionId: "s9", Via: ViaJoined}},

		document: &Document{Members: []Member{{Fleet: "T1", SessionId: "s9", Via: ViaJoined}}},
		fleet:    fleet,
	})

	// token-and-delivered
	tests = append(tests, &testCase{
		_id: "token-and-delivered",
		_expected: []MemberRow{{
			Id:        "s9",
			Delivered: true,
			MemberId:  "B",
			Name:      "T1-MB",
			Role:      "member",
			SessionId: "s9",
			Token:     "T1-MB",
			Via:       ViaJoined,
		}},

		document: &Document{Members: []Member{{Delivered: true, Fleet: "T1", MemberId: "B", SessionId: "s9", Token: "T1-MB", Via: ViaJoined}}},
		fleet:    fleet,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.document.MemberRows(test.fleet))
		})
	}
}

func TestStateTone(t *testing.T) {
	type testCase struct {
		_id       string
		_expected Tone

		tone func() Tone
	}

	tests := make([]*testCase, 0)
	sessionStates := map[SessionState]Tone{
		SessionStateBlocked: ToneError,
		SessionStateDied:    ToneError,
		SessionStateEnded:   ToneDim,
		SessionStateIdle:    ToneDim,
		SessionStateRunning: ToneAction,
		SessionStateWaiting: ToneError,
		"unknown":           ToneDim,
	}
	for state, tone := range sessionStates {
		tests = append(tests, &testCase{_id: "session-" + string(state), _expected: tone, tone: state.Tone})
	}

	subagentStates := map[SubagentState]Tone{
		SubagentStateRunning: ToneAction,
		SubagentStateStopped: ToneDim,
		"unknown":            ToneDim,
	}
	for state, tone := range subagentStates {
		tests = append(tests, &testCase{_id: "subagent-" + string(state), _expected: tone, tone: state.Tone})
	}

	branchTones := map[BranchState]Tone{
		BranchStateConflicting: ToneError,
		BranchStateMergeReady:  ToneOk,
		BranchStateMerged:      ToneOk,
		BranchStateNoWork:      ToneDim,
		BranchStateRemovable:   ToneDim,
		BranchStateUncommitted: ToneAction,
		BranchStateUnmerged:    ToneAction,
		"unknown":              ToneDim,
	}
	for state, tone := range branchTones {
		tests = append(tests, &testCase{_id: "branch-" + string(state), _expected: tone, tone: state.Tone})
	}

	targetTones := map[TargetState]Tone{
		TargetStateIntegrated: ToneOk,
		TargetStateMissing:    ToneError,
		TargetStateOpen:       ToneAction,
		"unknown":             ToneDim,
	}
	for state, tone := range targetTones {
		tests = append(tests, &testCase{_id: "target-" + string(state), _expected: tone, tone: state.Tone})
	}

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.tone())
		})
	}
}

package heartbeat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithoutToken(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		title string
		token string
	}

	tests := make([]*testCase, 0)

	// exact-suffix-cut
	tests = append(tests, &testCase{
		_id:       "exact-suffix-cut",
		_expected: "CC,FD - Fdesign/Fimplement permission guard",

		title: "CC,FD - Fdesign/Fimplement permission guard - T1-MC[DONE]",
		token: "T1-MC",
	})

	// token-absent
	tests = append(tests, &testCase{
		_id:       "token-absent",
		_expected: "CC,FD - guard - T1-MB[DONE]",

		title: "CC,FD - guard - T1-MB[DONE]",
		token: "T1-MC",
	})

	// no-trailing-bracket
	tests = append(tests, &testCase{
		_id:       "no-trailing-bracket",
		_expected: "CC,FD - guard - T1-MC[DONE] later",

		title: "CC,FD - guard - T1-MC[DONE] later",
		token: "T1-MC",
	})

	// empty-token
	tests = append(tests, &testCase{
		_id:       "empty-token",
		_expected: "CC,FD - guard - [DONE]",

		title: "CC,FD - guard - [DONE]",
		token: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, withoutToken(test.title, test.token))
		})
	}
}

func TestVia_Role(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		via Via
	}

	tests := make([]*testCase, 0)

	// joined
	tests = append(tests, &testCase{
		_id:       "joined",
		_expected: "member",

		via: ViaJoined,
	})

	// subscriber
	tests = append(tests, &testCase{
		_id:       "subscriber",
		_expected: "operator",

		via: ViaSubscriber,
	})

	// worker
	tests = append(tests, &testCase{
		_id:       "worker",
		_expected: "worker",

		via: ViaWorker,
	})

	// unknown
	tests = append(tests, &testCase{
		_id:       "unknown",
		_expected: "adopted",

		via: "adopted",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.via.Role())
		})
	}
}

func TestMemberRows_LastActive(t *testing.T) {
	document, err := decodeDocument(documentFixture(t))
	require.NoError(t, err)

	rows := document.MemberRows(&document.Fleets[0])

	require.Len(t, rows, 5)
	assert.Equal(t, "2026-09-28T01:17:55.938452Z", rows[0].LastActive, "the subscriber session's last hook")
	assert.Equal(t, "operator", rows[0].Role)
	assert.Empty(t, rows[1].LastActive, "a worker has no hook time of its own")
	assert.Equal(t, "worker", rows[1].Role)
	assert.Equal(t, "agent-a3ede1d975161fb7f", rows[1].Name, "a worker is named by its branch without claude/")
	assert.Equal(t, "CC,FImpl - P3 orchestrator loop acceptance tests - [READY]", rows[0].Title, "the session member keeps its raw title")
	assert.Empty(t, rows[1].Title, "a worker has no session title")
}

func TestDocument_FleetSessions(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []Session

		document *Document
	}

	fleet := Fleet{Id: "T1", Repo: "/r"}
	operator := Session{Id: "s-op", Title: "CC,Fleet - Orchestrator - T1-O[WIP]"}
	joined := Session{Id: "s-member", Title: "CC,FD - Design - T1-MA[WIP]"}
	cell := Session{Id: "s-cell", Title: "CC,FImpl - Worker"}
	outsider := Session{Id: "s-out", Title: "CC,FD - Unrelated"}
	subscriberMember := Member{Fleet: "T1", SessionId: "s-op", Token: "T1-O", Via: ViaSubscriber}
	joinedMember := Member{Branch: "claude/m", Fleet: "T1", MemberId: "A", SessionId: "s-member", Token: "T1-MA", Via: ViaJoined}
	workerMember := Member{Branch: "claude/w", Fleet: "T1", MemberId: "B", SessionId: "s-op", Token: "T1-MB", Via: ViaWorker}
	workerBranch := Branch{Branch: "claude/w", Repo: "/r", Sessions: []string{"s-cell"}}

	tests := make([]*testCase, 0)

	// no-fleets
	tests = append(tests, &testCase{
		_id:       "no-fleets",
		_expected: []Session{},

		document: &Document{Sessions: []Session{outsider, joined}},
	})

	// joined-and-subscriber
	tests = append(tests, &testCase{
		_id:       "joined-and-subscriber",
		_expected: []Session{joined, operator},

		document: &Document{
			Fleets:   []Fleet{fleet},
			Members:  []Member{subscriberMember, joinedMember},
			Sessions: []Session{outsider, joined, operator},
		},
	})

	// worker-cell
	tests = append(tests, &testCase{
		_id:       "worker-cell",
		_expected: []Session{cell, operator},

		document: &Document{
			Branches: []Branch{workerBranch},
			Fleets:   []Fleet{fleet},
			Members:  []Member{subscriberMember, workerMember},
			Sessions: []Session{cell, outsider, operator},
		},
	})

	// worker-branch-in-other-repo
	tests = append(tests, &testCase{
		_id:       "worker-branch-in-other-repo",
		_expected: []Session{operator},

		document: &Document{
			Branches: []Branch{{Branch: "claude/w", Repo: "/other", Sessions: []string{"s-cell"}}},
			Fleets:   []Fleet{fleet},
			Members:  []Member{subscriberMember, workerMember},
			Sessions: []Session{cell, operator},
		},
	})

	// worker-without-branch-row
	tests = append(tests, &testCase{
		_id:       "worker-without-branch-row",
		_expected: []Session{operator},

		document: &Document{
			Fleets:   []Fleet{fleet},
			Members:  []Member{subscriberMember, workerMember},
			Sessions: []Session{cell, operator},
		},
	})

	// member-session-missing
	tests = append(tests, &testCase{
		_id:       "member-session-missing",
		_expected: []Session{},

		document: &Document{
			Fleets:   []Fleet{fleet},
			Members:  []Member{joinedMember},
			Sessions: []Session{outsider},
		},
	})

	// matched-twice-listed-once
	tests = append(tests, &testCase{
		_id:       "matched-twice-listed-once",
		_expected: []Session{joined},

		document: &Document{
			Branches: []Branch{{Branch: "claude/w", Repo: "/r", Sessions: []string{"s-member"}}},
			Fleets:   []Fleet{fleet},
			Members:  []Member{joinedMember, workerMember},
			Sessions: []Session{joined},
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.document.FleetSessions())
		})
	}
}

package heartbeat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runtimeFixture is a runtime-v1 document: an act-mode subscriber, one
// open and one killed run, one handed-over and one open finding.
func runtimeFixture(t *testing.T) string {
	t.Helper()
	fixture, err := os.ReadFile(filepath.Join("testdata", "runtime_document.json"))
	require.NoError(t, err)
	return string(fixture)
}

func TestCli_Runtime(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedDetail string
		_isForeignHome  bool
		_isUnreachable  bool
		_shouldPass     bool

		cli *Cli
	}

	cliFor := func(stub string) *Cli {
		return &Cli{Addr: "127.0.0.1:6101", BinaryPath: stub, ClaudeHome: "/h"}
	}

	tests := make([]*testCase, 0)

	// fixture-decodes
	tests = append(tests, &testCase{
		_id:             "fixture-decodes",
		_expectedDetail: "<nil>",
		_isForeignHome:  false,
		_isUnreachable:  false,
		_shouldPass:     true,

		cli: cliFor(writeHeartbeatStub(t, 0, runtimeFixture(t))),
	})

	// schema-2
	tests = append(tests, &testCase{
		_id:             "schema-2",
		_expectedDetail: "Cli.Runtime: Unsupported schema 2",
		_isForeignHome:  false,
		_isUnreachable:  false,
		_shouldPass:     false,

		cli: cliFor(writeHeartbeatStub(t, 0, `{"schema":2,"runs":[],"findings":[]}`)),
	})

	// malformed
	tests = append(tests, &testCase{
		_id:             "malformed",
		_expectedDetail: "Cli.Runtime: Failed to decode",
		_isForeignHome:  false,
		_isUnreachable:  false,
		_shouldPass:     false,

		cli: cliFor(writeHeartbeatStub(t, 0, "not json")),
	})

	// unreachable-exit-3
	tests = append(tests, &testCase{
		_id:             "unreachable-exit-3",
		_expectedDetail: "heartbeat: no daemon on 127.0.0.1:6101",
		_isForeignHome:  false,
		_isUnreachable:  true,
		_shouldPass:     false,

		cli: cliFor(writeHeartbeatStub(t, 3, "heartbeat: no daemon on 127.0.0.1:6101\n")),
	})

	// foreign-exit-4
	tests = append(tests, &testCase{
		_id:             "foreign-exit-4",
		_expectedDetail: "serves /other",
		_isForeignHome:  true,
		_isUnreachable:  false,
		_shouldPass:     false,

		cli: cliFor(writeHeartbeatStub(t, 4, "heartbeat: 127.0.0.1:6101 serves /other\n")),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			document, err := test.cli.Runtime(context.Background())
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._shouldPass, document != nil)
			assert.Equal(t, test._isUnreachable, errors.Is(err, ErrUnreachable))
			assert.Equal(t, test._isForeignHome, errors.Is(err, ErrForeignHome))
			assert.Contains(t, fmt.Sprint(err), test._expectedDetail)
			assert.Equal(t, "--addr 127.0.0.1:6101 --claude-home /h runtime status --format json", readArgv(t, test.cli.BinaryPath))
		})
	}
}

func TestCli_RuntimeFields(t *testing.T) {
	cli := &Cli{Addr: "127.0.0.1:6101", BinaryPath: writeHeartbeatStub(t, 0, runtimeFixture(t)), ClaudeHome: "/h"}

	document, err := cli.Runtime(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "127.0.0.1:6101", document.Heartbeat.Addr)
	require.Len(t, document.Subscriptions, 1)
	watch := RuntimeSubscription{
		LastDigest:  &RuntimeLastDigest{Id: "d3", At: "2026-09-29T09:14:02Z", Outcome: "sent"},
		Mode:        "act",
		Session:     "5d1c2a90-3b7e-4f2a-9c11-0e6f1d2b3a44",
		Since:       "2026-09-29T09:00:00Z",
		Target:      "8a2f7c31-6d4e-4b9a-8e22-1f3c5a7b9d01",
		TargetTitle: "CC,Rr - Review runtime checker",
		Title:       "CC,Rt - Runtime watch",
	}
	assert.Equal(t, watch, document.Subscriptions[0])

	require.Len(t, document.Runs, 2)
	lanesCaptures := map[string]json.RawMessage{"lanes": json.RawMessage(`[{"direction":"tests"}]`)}
	anchors := []AnchorHit{
		{Id: "stack", Captures: map[string]json.RawMessage{}, Seq: 4},
		{Id: "lanes", Captures: lanesCaptures, Seq: 9},
	}
	open := Run{
		Id:       1,
		Anchors:  anchors,
		Failures: []CheckFailure{{Check: "round_buildtest_build", Count: 1}},
		Findings: 2,
		Passes:   []CheckPass{{Check: "round_after_lanes", Count: 2}},
		Reason:   "",
		Session:  "8a2f7c31-6d4e-4b9a-8e22-1f3c5a7b9d01",
		Skill:    "railroad-review",
		State:    "open",
		Title:    "CC,Rr - Review runtime checker",
		Version:  "7.4.0",
	}
	assert.Equal(t, open, document.Runs[0])
	assert.Equal(t, "killed", document.Runs[1].State)
	assert.Equal(t, "value_provenance ×8", document.Runs[1].Reason)
	killedPasses := []CheckPass{{Check: "producer_first", Count: 2}}
	assert.Equal(t, killedPasses, document.Runs[1].Passes)

	require.Len(t, document.Findings, 2)
	answerable := Finding{
		Id:       "0199a1b2-c3d4-7e5f-8a6b-7c8d9e0f1a2c",
		Acted:    "open",
		Entry:    "SKILL-RAILROADREVIEW-STATION-009",
		Evidence: []string{"red base reported at 09:13:55Z"},
		Kind:     "check_failed",
		Moves:    []string{"send", "stop"},
		Reason:   "base red and expect-base-red absent",
		Run:      1,
		Session:  "8a2f7c31-6d4e-4b9a-8e22-1f3c5a7b9d01",
		Skill:    "railroad-review",
		Title:    "CC,Rr - Review runtime checker",
		Version:  "7.4.0",
	}
	assert.Equal(t, answerable, document.Findings[1])
}

func TestCli_RuntimeWithoutSubscriber(t *testing.T) {
	output := `{"schema":1,"heartbeat":{"addr":"127.0.0.1:6101"},"subscriptions":[],"runs":[],"findings":[]}`
	cli := &Cli{Addr: "127.0.0.1:6101", BinaryPath: writeHeartbeatStub(t, 0, output), ClaudeHome: "/h"}

	document, err := cli.Runtime(context.Background())
	require.NoError(t, err)

	assert.Empty(t, document.Subscriptions)
	assert.Empty(t, document.Runs)
	assert.Empty(t, document.Findings)
}

func TestCli_RuntimeRunWithoutPasses(t *testing.T) {
	output := `{"schema":1,"heartbeat":{"addr":"127.0.0.1:6101"},"subscriptions":[],"runs":[{"id":1,"anchors":[],"failures":[],"findings":0,"reason":"","session":"s-1","skill":"heartbeat-demo","state":"open","title":"","version":"2.1.2"}],"findings":[]}`
	cli := &Cli{Addr: "127.0.0.1:6101", BinaryPath: writeHeartbeatStub(t, 0, output), ClaudeHome: "/h"}

	document, err := cli.Runtime(context.Background())
	require.NoError(t, err)

	require.Len(t, document.Runs, 1)
	assert.Empty(t, document.Runs[0].Passes)
}

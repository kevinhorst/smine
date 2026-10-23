package heartbeat

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCli_Remove(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedDetail string
		_shouldPass     bool

		cli *Cli
	}

	cliFor := func(stub string) *Cli {
		return &Cli{Addr: "127.0.0.1:6101", BinaryPath: stub, ClaudeHome: "/h"}
	}

	tests := make([]*testCase, 0)

	// removed-exit-0
	tests = append(tests, &testCase{
		_id:             "removed-exit-0",
		_expectedDetail: "<nil>",
		_shouldPass:     true,

		cli: cliFor(writeHeartbeatStub(t, 0, "T1-MB joined\n")),
	})

	// refused-exit-1
	tests = append(tests, &testCase{
		_id:             "refused-exit-1",
		_expectedDetail: "Fleet T1 member B: heartbeat: 409 no live member with that letter",
		_shouldPass:     false,

		cli: cliFor(writeHeartbeatStub(t, 1, "heartbeat: 409 no live member with that letter\n")),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := test.cli.Remove(context.Background(), "T1", "B")
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Contains(t, fmt.Sprint(err), test._expectedDetail)
			assert.Equal(t, "--addr 127.0.0.1:6101 --claude-home /h remove --fleet T1 --member B", readArgv(t, test.cli.BinaryPath))
		})
	}
}

func TestFleet_OutsideCollisions(t *testing.T) {
	ended := Subscription{OutsideCollisions: true, State: "ended"}
	active := Subscription{OutsideCollisions: true, State: subscriptionActive}

	assert.False(t, (&Fleet{Subscriptions: []Subscription{ended}}).OutsideCollisions(), "an ended subscription's flag is stale")
	assert.True(t, (&Fleet{Subscriptions: []Subscription{ended, active}}).OutsideCollisions())
	assert.False(t, (&Fleet{}).OutsideCollisions())
}

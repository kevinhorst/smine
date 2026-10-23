//go:build darwin

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/trash"
)

// fakeHost stands in for lsof, kill and launchctl: it records every signal and stop instead of sending one.
type fakeHost struct {
	command string
	pid     int
	signals []syscall.Signal
	stops   []string
}

func (h *fakeHost) listener(ctx context.Context, port string) (int, string) {
	return h.pid, h.command
}

func (h *fakeHost) signal(pid int, signal syscall.Signal) error {
	h.signals = append(h.signals, signal)
	if signal == 0 {
		return errors.New("fakeHost.signal: Process gone")
	}
	return nil
}

func (h *fakeHost) stop(ctx context.Context, label string) (string, error) {
	h.stops = append(h.stops, label)
	return "", nil
}

func installFakeHost(t *testing.T, host *fakeHost) {
	t.Helper()
	previousLookup, previousSignal, previousStop := lookupListener, signalProcess, stopAgent
	lookupListener, signalProcess, stopAgent = host.listener, host.signal, host.stop
	t.Cleanup(func() {
		lookupListener, signalProcess, stopAgent = previousLookup, previousSignal, previousStop
	})
}

func writePlist(t *testing.T, dir, label string) string {
	t.Helper()
	plist := filepath.Join(dir, label+".plist")
	require.NoError(t, os.WriteFile(plist, []byte("<string>/repo/bin/configserver</string>"), 0o644))
	return plist
}

func TestRemoveLegacyAgent(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedOutput string
		_expectedStash  int
		_expectedStops  []string
		_isKept         bool

		host  *fakeHost
		plist string
		store *trash.Store
	}

	tests := make([]*testCase, 0)

	// smine-label-stashed
	dir := t.TempDir()
	tests = append(tests, &testCase{
		_id:             "smine-label-stashed",
		_expectedOutput: "-> Stashing legacy configserver agent com.smine.configserver-legacy ...\n",
		_expectedStash:  1,
		_expectedStops:  []string{"com.smine.configserver-legacy"},
		_isKept:         false,

		host:  &fakeHost{},
		plist: writePlist(t, dir, "com.smine.configserver-legacy"),
		store: trash.New(filepath.Join(dir, "state"), 0),
	})

	// foreign-label-left
	dir = t.TempDir()
	tests = append(tests, &testCase{
		_id:             "foreign-label-left",
		_expectedOutput: "-> Leaving com.example.server: it runs a configserver but is not a smine agent (no com.smine. prefix)\n",
		_expectedStash:  0,
		_expectedStops:  nil,
		_isKept:         true,

		host:  &fakeHost{},
		plist: writePlist(t, dir, "com.example.server"),
		store: trash.New(filepath.Join(dir, "state"), 0),
	})

	// server-label-skipped
	dir = t.TempDir()
	tests = append(tests, &testCase{
		_id:             "server-label-skipped",
		_expectedOutput: "",
		_expectedStash:  0,
		_expectedStops:  nil,
		_isKept:         true,

		host:  &fakeHost{},
		plist: writePlist(t, dir, install.ServerAgentLabel),
		store: trash.New(filepath.Join(dir, "state"), 0),
	})

	// heartbeat-label-skipped
	dir = t.TempDir()
	tests = append(tests, &testCase{
		_id:             "heartbeat-label-skipped",
		_expectedOutput: "",
		_expectedStash:  0,
		_expectedStops:  nil,
		_isKept:         true,

		host:  &fakeHost{},
		plist: writePlist(t, dir, install.HeartbeatAgentLabel),
		store: trash.New(filepath.Join(dir, "state"), 0),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			installFakeHost(t, test.host)
			out := &bytes.Buffer{}

			err := removeLegacyAgent(context.Background(), out, test.plist, test.store)
			_, statErr := os.Stat(test.plist)
			stashes, listErr := test.store.List()

			assert.NoError(t, err)
			assert.NoError(t, listErr)
			assert.Equal(t, test._expectedOutput, out.String())
			assert.Equal(t, test._isKept, statErr == nil)
			assert.Len(t, stashes, test._expectedStash)
			assert.Equal(t, test._expectedStops, test.host.stops)
		})
	}
}

func TestStopForeignListener(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedAsks    int
		_expectedSignals []syscall.Signal
		_shouldPass      bool

		answer bool
		host   *fakeHost
	}

	repoDir := t.TempDir()

	tests := make([]*testCase, 0)

	// no-listener
	tests = append(tests, &testCase{
		_id:              "no-listener",
		_expectedAsks:    0,
		_expectedSignals: nil,
		_shouldPass:      true,

		answer: false,
		host:   &fakeHost{},
	})

	// own-checkout-left
	tests = append(tests, &testCase{
		_id:              "own-checkout-left",
		_expectedAsks:    0,
		_expectedSignals: nil,
		_shouldPass:      true,

		answer: false,
		host:   &fakeHost{command: filepath.Join(repoDir, "bin", "configserver"), pid: 4242},
	})

	// not-a-configserver
	tests = append(tests, &testCase{
		_id:              "not-a-configserver",
		_expectedAsks:    0,
		_expectedSignals: nil,
		_shouldPass:      false,

		answer: true,
		host:   &fakeHost{command: "/usr/bin/python3", pid: 4242},
	})

	// configserver-declined
	tests = append(tests, &testCase{
		_id:              "configserver-declined",
		_expectedAsks:    1,
		_expectedSignals: nil,
		_shouldPass:      false,

		answer: false,
		host:   &fakeHost{command: "/other/checkout/bin/configserver", pid: 4242},
	})

	// configserver-confirmed
	tests = append(tests, &testCase{
		_id:              "configserver-confirmed",
		_expectedAsks:    1,
		_expectedSignals: []syscall.Signal{syscall.SIGTERM, 0},
		_shouldPass:      true,

		answer: true,
		host:   &fakeHost{command: "/other/checkout/bin/configserver", pid: 4242},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			installFakeHost(t, test.host)
			asks := 0
			ask := func(question string) bool {
				asks++
				return test.answer
			}

			err := stopForeignListener(context.Background(), &bytes.Buffer{}, "6001", repoDir, ask)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedAsks, asks)
			assert.Equal(t, test._expectedSignals, test.host.signals)
		})
	}
}

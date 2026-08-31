package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartPeek(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		bin        string
		claudeHome string
		codexHome  string
		port       int
	}

	healthzPort := func(claudeHome, codexHome string) int {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, `{"version":"1.2.2","claudeHome":%q,"codexHome":%q,"controlPort":0}`, claudeHome, codexHome)
		})
		testServer := httptest.NewServer(handler)
		t.Cleanup(testServer.Close)
		return testServer.Listener.Addr().(*net.TCPAddr).Port
	}
	notFoundPort := func() int {
		testServer := httptest.NewServer(http.NotFoundHandler())
		t.Cleanup(testServer.Close)
		return testServer.Listener.Addr().(*net.TCPAddr).Port
	}
	freePort := func() int {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		port := listener.Addr().(*net.TCPAddr).Port
		listener.Close()
		return port
	}

	tests := make([]*testCase, 0)

	// identity-match-reuses
	tests = append(tests, &testCase{
		_id:       "identity-match-reuses",
		_expected: true,

		bin:        "unused",
		claudeHome: "/home/a/.claude",
		codexHome:  "/home/a/.codex",
		port:       healthzPort("/home/a/.claude", "/home/a/.codex"),
	})

	// home-mismatch-refuses
	tests = append(tests, &testCase{
		_id:       "home-mismatch-refuses",
		_expected: false,

		bin:        "unused",
		claudeHome: "/home/b/.claude",
		codexHome:  "/home/b/.codex",
		port:       healthzPort("/home/a/.claude", "/home/a/.codex"),
	})

	// non-peek-listener-refuses
	tests = append(tests, &testCase{
		_id:       "non-peek-listener-refuses",
		_expected: false,

		bin:        "unused",
		claudeHome: "/home/a/.claude",
		codexHome:  "/home/a/.codex",
		port:       notFoundPort(),
	})

	// no-listener-spawns
	tests = append(tests, &testCase{
		_id:       "no-listener-spawns",
		_expected: true,

		bin:        "true",
		claudeHome: "/home/a/.claude",
		codexHome:  "/home/a/.codex",
		port:       freePort(),
	})

	// spawn-failure-degrades
	tests = append(tests, &testCase{
		_id:       "spawn-failure-degrades",
		_expected: false,

		bin:        "definitely-missing-peek-binary",
		claudeHome: "/home/a/.claude",
		codexHome:  "/home/a/.codex",
		port:       freePort(),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			ok := startPeek(context.Background(), test.bin, test.port, 0, "", test.claudeHome, test.codexHome)
			assert.Equal(t, test._expected, ok)
		})
	}
}

package peek

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// healthzPort serves a peek /healthz identity for the given homes.
func healthzPort(t *testing.T, claudeHome, codexHome string) int {
	t.Helper()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"version":"1.2.2","claudeHome":%q,"codexHome":%q,"controlPort":0}`, claudeHome, codexHome)
	})
	testServer := httptest.NewServer(handler)
	t.Cleanup(testServer.Close)
	return testServer.Listener.Addr().(*net.TCPAddr).Port
}

// notFoundPort serves a listener that is not an identifiable peek.
func notFoundPort(t *testing.T) int {
	t.Helper()
	testServer := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(testServer.Close)
	return testServer.Listener.Addr().(*net.TCPAddr).Port
}

// nonJsonPort serves 200 with an HTML body — a listener that answers
// /healthz but is not a peek.
func nonJsonPort(t *testing.T) int {
	t.Helper()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<html>not peek</html>")
	})
	testServer := httptest.NewServer(handler)
	t.Cleanup(testServer.Close)
	return testServer.Listener.Addr().(*net.TCPAddr).Port
}

// freePort returns a loopback port nothing listens on.
func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port
}

func TestProbe(t *testing.T) {
	type testCase struct {
		_id       string
		_expected Holder

		claudeHome string
		codexHome  string
		port       int
	}

	tests := make([]*testCase, 0)

	// own-identity
	tests = append(tests, &testCase{
		_id:       "own-identity",
		_expected: HolderOwn,

		claudeHome: "/home/a/.claude",
		codexHome:  "/home/a/.codex",
		port:       healthzPort(t, "/home/a/.claude", "/home/a/.codex"),
	})

	// foreign-identity
	tests = append(tests, &testCase{
		_id:       "foreign-identity",
		_expected: HolderForeign,

		claudeHome: "/home/b/.claude",
		codexHome:  "/home/b/.codex",
		port:       healthzPort(t, "/home/a/.claude", "/home/a/.codex"),
	})

	// non-peek-listener-foreign
	tests = append(tests, &testCase{
		_id:       "non-peek-listener-foreign",
		_expected: HolderForeign,

		claudeHome: "/home/a/.claude",
		codexHome:  "/home/a/.codex",
		port:       notFoundPort(t),
	})

	// non-json-healthz-foreign
	tests = append(tests, &testCase{
		_id:       "non-json-healthz-foreign",
		_expected: HolderForeign,

		claudeHome: "/home/a/.claude",
		codexHome:  "/home/a/.codex",
		port:       nonJsonPort(t),
	})

	// no-listener-none
	tests = append(tests, &testCase{
		_id:       "no-listener-none",
		_expected: HolderNone,

		claudeHome: "/home/a/.claude",
		codexHome:  "/home/a/.codex",
		port:       freePort(t),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			opts := SpawnOptions{ClaudeHome: test.claudeHome, CodexHome: test.codexHome, Port: test.port}
			assert.Equal(t, test._expected, Probe(opts))
		})
	}
}

func TestStart(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		bin        string
		claudeHome string
		codexHome  string
		port       int
	}

	tests := make([]*testCase, 0)

	// identity-match-reuses
	tests = append(tests, &testCase{
		_id:       "identity-match-reuses",
		_expected: true,

		bin:        "unused",
		claudeHome: "/home/a/.claude",
		codexHome:  "/home/a/.codex",
		port:       healthzPort(t, "/home/a/.claude", "/home/a/.codex"),
	})

	// home-mismatch-refuses
	tests = append(tests, &testCase{
		_id:       "home-mismatch-refuses",
		_expected: false,

		bin:        "unused",
		claudeHome: "/home/b/.claude",
		codexHome:  "/home/b/.codex",
		port:       healthzPort(t, "/home/a/.claude", "/home/a/.codex"),
	})

	// non-peek-listener-refuses
	tests = append(tests, &testCase{
		_id:       "non-peek-listener-refuses",
		_expected: false,

		bin:        "unused",
		claudeHome: "/home/a/.claude",
		codexHome:  "/home/a/.codex",
		port:       notFoundPort(t),
	})

	// no-listener-spawns
	tests = append(tests, &testCase{
		_id:       "no-listener-spawns",
		_expected: true,

		bin:        "true",
		claudeHome: "/home/a/.claude",
		codexHome:  "/home/a/.codex",
		port:       freePort(t),
	})

	// spawn-failure-degrades
	tests = append(tests, &testCase{
		_id:       "spawn-failure-degrades",
		_expected: false,

		bin:        "definitely-missing-peek-binary",
		claudeHome: "/home/a/.claude",
		codexHome:  "/home/a/.codex",
		port:       freePort(t),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			opts := SpawnOptions{
				Bin:        test.bin,
				ClaudeHome: test.claudeHome,
				CodexHome:  test.codexHome,
				Port:       test.port,
			}
			ok := Start(context.Background(), opts)
			assert.Equal(t, test._expected, ok)
		})
	}
}

func TestReleaseAssetName(t *testing.T) {
	asset, err := releaseAssetName()
	require.NoError(t, err)

	switch runtime.GOOS {
	case "darwin":
		assert.Equal(t, "peek-mcp", asset)
	case "linux":
		assert.Equal(t, "peek-mcp-linux-"+runtime.GOARCH, asset)
	case "windows":
		assert.Equal(t, "peek-mcp-windows-"+runtime.GOARCH+".exe", asset)
	default:
		t.Fatalf("unexpected platform %s", runtime.GOOS)
	}
}

func TestInstalledVersion(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "peek-mcp")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\necho '  1.2.2  '\n"), 0o755))

	version, err := InstalledVersion(context.Background(), script)
	require.NoError(t, err)
	assert.Equal(t, "1.2.2", version)

	_, err = InstalledVersion(context.Background(), filepath.Join(dir, "missing-binary"))
	assert.Error(t, err)
}

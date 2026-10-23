//go:build windows

package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNoBashRefusals needs a process whose bash lookup finds nothing: CI runs
// it in its own go test step with SMINE_BASH naming a missing file (the lookup
// is resolved once per process).
func TestNoBashRefusals(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedBody   string
		_expectedStatus int

		path string
	}

	override := os.Getenv("SMINE_BASH")
	if _, err := os.Stat(override); override == "" || err == nil {
		t.Skip("needs SMINE_BASH naming a missing file — run in the no-bash CI step")
	}
	options := &Options{TokenDir: writeBootstrapToken(t)}
	server := newTestServer(t, options)
	tests := make([]*testCase, 0)

	// bootstrap-refused
	tests = append(tests, &testCase{
		_id:             "bootstrap-refused",
		_expectedBody:   "no bash available to run the bootstrap",
		_expectedStatus: http.StatusConflict,

		path: "/welcome/bootstrap",
	})

	// style-test-refused
	tests = append(tests, &testCase{
		_id:             "style-test-refused",
		_expectedBody:   "no bash available to run the test",
		_expectedStatus: http.StatusOK,

		path: "/profile/test",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			server.Handler().ServeHTTP(recorder, formPost(test.path, url.Values{"since": {"2026-08-01"}}))

			require.Equal(t, test._expectedStatus, recorder.Code)
			assert.Contains(t, recorder.Body.String(), test._expectedBody)
			assert.Zero(t, server.bootstrap.pid)
		})
	}
}

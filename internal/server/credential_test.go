package server

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testToken = "secret-token"

// basicRequest builds a request carrying user and password as HTTP Basic.
func basicRequest(method, path, user, password string) *http.Request {
	request := httptest.NewRequest(method, path, nil)
	request.SetBasicAuth(user, password)
	return request
}

// headerRequest builds a request with a raw Authorization header.
func headerRequest(method, path, authorization string) *http.Request {
	request := httptest.NewRequest(method, path, nil)
	request.Header.Set("Authorization", authorization)
	return request
}

// writeDigestFile writes content as a credential file and returns its path.
func writeDigestFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "credential")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestCredential_Require(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedCode      int
		_expectedChallenge string

		request *http.Request
	}

	challenge := `Basic realm="smine config server", charset="UTF-8"`
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := NewCredential(testToken).Require(next)
	tests := make([]*testCase, 0)

	// no-header
	tests = append(tests, &testCase{
		_id:                "no-header",
		_expectedCode:      http.StatusUnauthorized,
		_expectedChallenge: challenge,

		request: httptest.NewRequest(http.MethodGet, "/", nil),
	})

	// wrong-password
	tests = append(tests, &testCase{
		_id:                "wrong-password",
		_expectedCode:      http.StatusUnauthorized,
		_expectedChallenge: challenge,

		request: basicRequest(http.MethodGet, "/", "dev", "wrong"),
	})

	// right-password-any-user
	tests = append(tests, &testCase{
		_id:                "right-password-any-user",
		_expectedCode:      http.StatusNoContent,
		_expectedChallenge: "",

		request: basicRequest(http.MethodGet, "/", "anyone", testToken),
	})

	// right-password-empty-user
	tests = append(tests, &testCase{
		_id:                "right-password-empty-user",
		_expectedCode:      http.StatusNoContent,
		_expectedChallenge: "",

		request: basicRequest(http.MethodGet, "/", "", testToken),
	})

	// token-as-user-name
	tests = append(tests, &testCase{
		_id:                "token-as-user-name",
		_expectedCode:      http.StatusUnauthorized,
		_expectedChallenge: challenge,

		request: basicRequest(http.MethodGet, "/", testToken, ""),
	})

	// empty-password
	tests = append(tests, &testCase{
		_id:                "empty-password",
		_expectedCode:      http.StatusUnauthorized,
		_expectedChallenge: challenge,

		request: basicRequest(http.MethodGet, "/", "dev", ""),
	})

	// malformed-base64
	tests = append(tests, &testCase{
		_id:                "malformed-base64",
		_expectedCode:      http.StatusUnauthorized,
		_expectedChallenge: challenge,

		request: headerRequest(http.MethodGet, "/", "Basic !!!not-base64"),
	})

	// bearer-scheme
	tests = append(tests, &testCase{
		_id:                "bearer-scheme",
		_expectedCode:      http.StatusUnauthorized,
		_expectedChallenge: challenge,

		request: headerRequest(http.MethodGet, "/", "Bearer "+testToken),
	})

	// lowercase-scheme
	lowercase := basicRequest(http.MethodGet, "/", "dev", testToken)
	lowercase.Header.Set("Authorization", "basic "+strings.TrimPrefix(lowercase.Header.Get("Authorization"), "Basic "))
	tests = append(tests, &testCase{
		_id:                "lowercase-scheme",
		_expectedCode:      http.StatusNoContent,
		_expectedChallenge: "",

		request: lowercase,
	})

	// asset-path-no-header
	tests = append(tests, &testCase{
		_id:                "asset-path-no-header",
		_expectedCode:      http.StatusUnauthorized,
		_expectedChallenge: challenge,

		request: httptest.NewRequest(http.MethodGet, "/assets/htmx.min.js", nil),
	})

	// head-method-no-header
	tests = append(tests, &testCase{
		_id:                "head-method-no-header",
		_expectedCode:      http.StatusUnauthorized,
		_expectedChallenge: challenge,

		request: httptest.NewRequest(http.MethodHead, "/", nil),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, test.request)
			assert.Equal(t, test._expectedCode, recorder.Code)
			assert.Equal(t, test._expectedChallenge, recorder.Header().Get("WWW-Authenticate"))
		})
	}
}

func TestCredential_RequireWrapsHandler(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedCode     int
		_expectedSettings bool

		request      *http.Request
		settingsPath string
	}

	credential := NewCredential(testToken)
	envPost := func() *http.Request {
		values := url.Values{"key": {"AUTH_PROBE"}, "value": {"set"}}
		return formPost("/api/env", values)
	}
	serve := func(server *Server, request *http.Request) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		credential.Require(server.Handler()).ServeHTTP(recorder, request)
		return recorder
	}
	readSettings := func(path string) bool {
		content, _ := os.ReadFile(path)
		return strings.Contains(string(content), "AUTH_PROBE")
	}

	tests := make([]*testCase, 0)

	// post-without-credential
	tests = append(tests, &testCase{
		_id:               "post-without-credential",
		_expectedCode:     http.StatusUnauthorized,
		_expectedSettings: false,

		request:      envPost(),
		settingsPath: filepath.Join(t.TempDir(), "settings.json"),
	})

	// post-with-credential-no-origin
	withCredential := envPost()
	withCredential.SetBasicAuth("dev", testToken)
	tests = append(tests, &testCase{
		_id:               "post-with-credential-no-origin",
		_expectedCode:     http.StatusOK,
		_expectedSettings: true,

		request:      withCredential,
		settingsPath: filepath.Join(t.TempDir(), "settings.json"),
	})

	// post-with-credential-cross-site
	crossSite := envPost()
	crossSite.SetBasicAuth("dev", testToken)
	crossSite.Header.Set("Sec-Fetch-Site", "cross-site")
	tests = append(tests, &testCase{
		_id:               "post-with-credential-cross-site",
		_expectedCode:     http.StatusForbidden,
		_expectedSettings: false,

		request:      crossSite,
		settingsPath: filepath.Join(t.TempDir(), "settings.json"),
	})

	// get-with-credential
	tests = append(tests, &testCase{
		_id:               "get-with-credential",
		_expectedCode:     http.StatusOK,
		_expectedSettings: false,

		request:      basicRequest(http.MethodGet, "/", "dev", testToken),
		settingsPath: filepath.Join(t.TempDir(), "settings.json"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			response := serve(newTestServer(t, &Options{SettingsPath: test.settingsPath}), test.request)
			assert.Equal(t, test._expectedCode, response.Code, response.Body.String())
			assert.Equal(t, test._expectedSettings, readSettings(test.settingsPath))
		})
	}
}

func TestLoadCredential(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool

		path string
	}

	tokenDigest := NewCredential(testToken).digest
	digestHex := hex.EncodeToString(tokenDigest[:])
	tests := make([]*testCase, 0)

	// fail-missing-file
	tests = append(tests, &testCase{
		_id:         "fail-missing-file",
		_shouldPass: false,

		path: filepath.Join(t.TempDir(), "absent"),
	})

	// fail-empty-file
	tests = append(tests, &testCase{
		_id:         "fail-empty-file",
		_shouldPass: false,

		path: writeDigestFile(t, ""),
	})

	// fail-not-hex
	tests = append(tests, &testCase{
		_id:         "fail-not-hex",
		_shouldPass: false,

		path: writeDigestFile(t, strings.Repeat("z", 64)),
	})

	// fail-short-digest
	tests = append(tests, &testCase{
		_id:         "fail-short-digest",
		_shouldPass: false,

		path: writeDigestFile(t, digestHex[:62]),
	})

	// fail-long-digest
	tests = append(tests, &testCase{
		_id:         "fail-long-digest",
		_shouldPass: false,

		path: writeDigestFile(t, digestHex+"00"),
	})

	// pass-valid
	tests = append(tests, &testCase{
		_id:         "pass-valid",
		_shouldPass: true,

		path: writeDigestFile(t, digestHex),
	})

	// pass-valid-trailing-newline
	tests = append(tests, &testCase{
		_id:         "pass-valid-trailing-newline",
		_shouldPass: true,

		path: writeDigestFile(t, digestHex+"\n"),
	})

	// pass-valid-crlf
	tests = append(tests, &testCase{
		_id:         "pass-valid-crlf",
		_shouldPass: true,

		path: writeDigestFile(t, digestHex+"\r\n"),
	})

	// pass-uppercase-hex
	tests = append(tests, &testCase{
		_id:         "pass-uppercase-hex",
		_shouldPass: true,

		path: writeDigestFile(t, strings.ToUpper(digestHex)),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			credential, err := LoadCredential(test.path)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._shouldPass, credential != nil && credential.isValidRequest(basicRequest(http.MethodGet, "/", "dev", testToken)))
		})
	}
}

func TestNewCredential(t *testing.T) {
	credential := NewCredential(testToken)

	assert.True(t, credential.isValidRequest(basicRequest(http.MethodGet, "/", "dev", testToken)))
	assert.False(t, credential.isValidRequest(basicRequest(http.MethodGet, "/", "dev", "other-token")))
	assert.False(t, credential.isValidRequest(basicRequest(http.MethodGet, "/", "dev", "")))
}

func TestGenerateCredential(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configserver", "credential")

	token, err := GenerateCredential(path)
	require.NoError(t, err)
	assert.Len(t, token, 26)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(content), token)

	credential, err := LoadCredential(path)
	require.NoError(t, err)
	assert.True(t, credential.isValidRequest(basicRequest(http.MethodGet, "/", "dev", token)))

	rotated, err := GenerateCredential(path)
	require.NoError(t, err)
	assert.NotEqual(t, token, rotated)

	reloaded, err := LoadCredential(path)
	require.NoError(t, err)
	assert.False(t, reloaded.isValidRequest(basicRequest(http.MethodGet, "/", "dev", token)))
	assert.True(t, reloaded.isValidRequest(basicRequest(http.MethodGet, "/", "dev", rotated)))
}

func TestCredential_FollowsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "configserver", "credential")
	token, err := GenerateCredential(path)
	require.NoError(t, err)
	credential, err := LoadCredential(path)
	require.NoError(t, err)
	loadedAt := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

	// rotated-token-applies-without-reload
	rotated, err := GenerateCredential(path)
	require.NoError(t, err)
	require.NoError(t, os.Chtimes(path, loadedAt, loadedAt))
	assert.False(t, credential.isValidRequest(basicRequest(http.MethodGet, "/", "dev", token)))
	assert.True(t, credential.isValidRequest(basicRequest(http.MethodGet, "/", "dev", rotated)))

	// invalid-file-keeps-previous
	require.NoError(t, os.WriteFile(path, []byte("not hex"), 0o600))
	require.NoError(t, os.Chtimes(path, loadedAt.Add(time.Hour), loadedAt.Add(time.Hour)))
	assert.True(t, credential.isValidRequest(basicRequest(http.MethodGet, "/", "dev", rotated)))

	// removed-file-keeps-previous
	require.NoError(t, os.Remove(path))
	assert.True(t, credential.isValidRequest(basicRequest(http.MethodGet, "/", "dev", rotated)))
	assert.False(t, credential.isValidRequest(basicRequest(http.MethodGet, "/", "dev", "other-token")))
}

func TestGenerateCredential_Modes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes")
	}

	dir := filepath.Join(t.TempDir(), "configserver")
	_, err := GenerateCredential(filepath.Join(dir, "credential"))
	require.NoError(t, err)

	dirInfo, err := os.Stat(dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())

	fileInfo, err := os.Stat(filepath.Join(dir, "credential"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), fileInfo.Mode().Perm())
}

func TestGenerateCredential_ParentIsFile(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(parent, []byte("x"), 0o600))

	_, err := GenerateCredential(filepath.Join(parent, "credential"))
	assert.Error(t, err)
}

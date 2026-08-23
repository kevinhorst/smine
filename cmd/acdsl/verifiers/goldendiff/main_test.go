package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFilesList(t *testing.T, files ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "files.txt")
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(files, "\n")+"\n"), 0o644))
	return path
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func serve(t *testing.T, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestRunGoldenMatchWritesObservation(t *testing.T) {
	dir := t.TempDir()
	golden := writeFile(t, dir, "resp.golden", "line one\nline two\n")
	observed := filepath.Join(dir, "resp.observed")
	server := serve(t, "line one\nline two\n")

	var out bytes.Buffer
	code := run([]string{writeFilesList(t, golden), "url=" + server.URL, "observed=" + observed}, &out)
	assert.Equal(t, 0, code, out.String())
	written, err := os.ReadFile(observed)
	require.NoError(t, err)
	assert.Equal(t, "line one\nline two\n", string(written))
}

func TestRunMismatchAnchorsGoldenLines(t *testing.T) {
	dir := t.TempDir()
	golden := writeFile(t, dir, "resp.golden", "line one\nline two\n")
	observed := filepath.Join(dir, "resp.observed")
	server := serve(t, "line one\nline CHANGED\n")

	var out bytes.Buffer
	code := run([]string{writeFilesList(t, golden), "url=" + server.URL, "observed=" + observed}, &out)
	assert.Equal(t, 1, code)
	assert.Contains(t, out.String(), golden+`:2: expected "line two", observed "line CHANGED"`)
}

func TestRunReportCapAtTen(t *testing.T) {
	dir := t.TempDir()
	golden := writeFile(t, dir, "resp.golden", strings.Repeat("same\n", 12))
	observed := filepath.Join(dir, "resp.observed")
	server := serve(t, strings.Repeat("diff\n", 12))

	var out bytes.Buffer
	code := run([]string{writeFilesList(t, golden), "url=" + server.URL, "observed=" + observed}, &out)
	assert.Equal(t, 1, code)
	assert.Equal(t, 10, strings.Count(out.String(), "expected"))
	assert.Contains(t, out.String(), "further differing line(s)")
}

func TestRunUnreachableTargetIsViolationNotError(t *testing.T) {
	dir := t.TempDir()
	golden := writeFile(t, dir, "resp.golden", "x\n")
	observed := filepath.Join(dir, "resp.observed")

	var out bytes.Buffer
	code := run([]string{writeFilesList(t, golden), "url=http://127.0.0.1:1/nope", "observed=" + observed}, &out)
	assert.Equal(t, 1, code)
	assert.Contains(t, out.String(), "request failed")
}

func TestRunNormalizeSpecApplied(t *testing.T) {
	dir := t.TempDir()
	golden := writeFile(t, dir, "resp.golden", "path <HOME>/x\n")
	observed := filepath.Join(dir, "resp.observed")
	spec := writeFile(t, dir, "normalize.spec", "# mask home\ns|/Users/[a-z]+|<HOME>|\n")
	server := serve(t, "path /Users/kevin/x\n")

	var out bytes.Buffer
	code := run([]string{writeFilesList(t, golden), "url=" + server.URL, "observed=" + observed, "normalize=" + spec}, &out)
	assert.Equal(t, 0, code, out.String())
}

func TestRunBadSpecLineIsError(t *testing.T) {
	dir := t.TempDir()
	golden := writeFile(t, dir, "resp.golden", "x\n")
	observed := filepath.Join(dir, "resp.observed")
	spec := writeFile(t, dir, "normalize.spec", "not-a-rule\n")
	server := serve(t, "x\n")

	var out bytes.Buffer
	code := run([]string{writeFilesList(t, golden), "url=" + server.URL, "observed=" + observed, "normalize=" + spec}, &out)
	assert.Equal(t, 2, code)
}

func TestRunMissingParamsAreErrors(t *testing.T) {
	var out bytes.Buffer
	assert.Equal(t, 2, run([]string{}, &out))
	assert.Equal(t, 2, run([]string{writeFilesList(t, "a", "b"), "url=http://x", "observed=o"}, &out))
}

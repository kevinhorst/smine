package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeList writes one Go file named name with content under a temp root and returns a files-list naming it.
func writeList(t *testing.T, name, content string) string {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "cmd", "hook", name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	listPath := filepath.Join(root, "files.txt")
	require.NoError(t, os.WriteFile(listPath, []byte(path+"\n"), 0o644))
	return listPath
}

func TestRun(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedCode  int
		_expectedLines int

		listPath string
		params   []string
	}

	tests := make([]*testCase, 0)

	// pass-clean
	tests = append(tests, &testCase{
		_id:            "pass-clean",
		_expectedCode:  0,
		_expectedLines: 0,

		listPath: writeList(t, "clean.go", "package hook\n\ntype settings struct{ Cap int }\n\nfunc over(s settings, n int) bool {\n\treturn n > s.Cap\n}\n"),
		params:   []string{},
	})

	// pass-protocol-exempt
	tests = append(tests, &testCase{
		_id:            "pass-protocol-exempt",
		_expectedCode:  0,
		_expectedLines: 0,

		listPath: writeList(t, "protocol.go", "package hook\n\nconst x = 2\n"),
		params:   []string{},
	})

	// fail-const-literal
	tests = append(tests, &testCase{
		_id:            "fail-const-literal",
		_expectedCode:  1,
		_expectedLines: 1,

		listPath: writeList(t, "literal.go", "package hook\n\nconst inlineCap = 9000\n"),
		params:   []string{},
	})

	// fail-body-literal
	tests = append(tests, &testCase{
		_id:            "fail-body-literal",
		_expectedCode:  1,
		_expectedLines: 1,

		listPath: writeList(t, "body.go", "package hook\n\nfunc over(n int) bool {\n\treturn n > 50\n}\n"),
		params:   []string{},
	})

	// pass-trivial-literals
	tests = append(tests, &testCase{
		_id:            "pass-trivial-literals",
		_expectedCode:  0,
		_expectedLines: 0,

		listPath: writeList(t, "trivial.go", "package hook\n\nfunc edge(n int) int {\n\tif n > 0 {\n\t\treturn n - 1\n\t}\n\treturn -1\n}\n"),
		params:   []string{},
	})

	// fail-composite-literal
	tests = append(tests, &testCase{
		_id:            "fail-composite-literal",
		_expectedCode:  1,
		_expectedLines: 1,

		listPath: writeList(t, "composite.go", "package hook\n\nvar names = []string{\"a\"}\n"),
		params:   []string{},
	})

	// pass-ignored-param
	tests = append(tests, &testCase{
		_id:            "pass-ignored-param",
		_expectedCode:  0,
		_expectedLines: 0,

		listPath: writeList(t, "ignored.go", "package hook\n\nfunc code() int {\n\treturn 2\n}\n"),
		params:   []string{"ignored=0o644,0o755,2"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			var out bytes.Buffer
			code := run(append([]string{test.listPath}, test.params...), &out)
			assert.Equal(t, test._expectedCode, code)
			assert.Equal(t, test._expectedLines, bytes.Count(out.Bytes(), []byte("\n")))
		})
	}
}

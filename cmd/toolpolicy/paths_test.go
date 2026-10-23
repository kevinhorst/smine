package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsInsideRoots(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		cwd   string
		path  string
		roots []string
	}

	base := t.TempDir()
	cwd := filepath.Join(base, "work")
	require.NoError(t, os.MkdirAll(filepath.Join(cwd, "sub"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(base, "other"), 0o755))
	linkRoot := filepath.Join(base, "link")
	require.NoError(t, os.Symlink(cwd, linkRoot))

	tests := make([]*testCase, 0)

	// absolute-inside
	tests = append(tests, &testCase{
		_id:       "absolute-inside",
		_expected: true,

		cwd:   cwd,
		path:  filepath.Join(cwd, "sub", "file.go"),
		roots: []string{cwd},
	})

	// relative-inside
	tests = append(tests, &testCase{
		_id:       "relative-inside",
		_expected: true,

		cwd:   cwd,
		path:  "sub/file.go",
		roots: []string{cwd},
	})

	// dotdot-escape
	tests = append(tests, &testCase{
		_id:       "dotdot-escape",
		_expected: false,

		cwd:   cwd,
		path:  "../other/file.go",
		roots: []string{cwd},
	})

	// symlinked-root
	tests = append(tests, &testCase{
		_id:       "symlinked-root",
		_expected: true,

		cwd:   cwd,
		path:  filepath.Join(linkRoot, "sub", "new.go"),
		roots: []string{cwd},
	})

	// empty-path-is-cwd
	tests = append(tests, &testCase{
		_id:       "empty-path-is-cwd",
		_expected: true,

		cwd:   cwd,
		path:  "",
		roots: []string{cwd},
	})

	// outside-every-root
	tests = append(tests, &testCase{
		_id:       "outside-every-root",
		_expected: false,

		cwd:   cwd,
		path:  filepath.Join(base, "other", "x"),
		roots: []string{cwd, filepath.Join(base, "third")},
	})

	// prefix-name-collision
	tests = append(tests, &testCase{
		_id:       "prefix-name-collision",
		_expected: false,

		cwd:   cwd,
		path:  cwd + "-sibling/file",
		roots: []string{cwd},
	})

	globRoot := filepath.Join(base, "projects", "*", "memory")

	// glob-root-inside
	tests = append(tests, &testCase{
		_id:       "glob-root-inside",
		_expected: true,

		cwd:   cwd,
		path:  filepath.Join(base, "projects", "slug", "memory", "fact.md"),
		roots: []string{globRoot},
	})

	// glob-root-is-the-directory
	tests = append(tests, &testCase{
		_id:       "glob-root-is-the-directory",
		_expected: true,

		cwd:   cwd,
		path:  filepath.Join(base, "projects", "slug", "memory"),
		roots: []string{globRoot},
	})

	// glob-root-one-segment-only
	tests = append(tests, &testCase{
		_id:       "glob-root-one-segment-only",
		_expected: false,

		cwd:   cwd,
		path:  filepath.Join(base, "projects", "a", "b", "memory", "fact.md"),
		roots: []string{globRoot},
	})

	// glob-root-target-too-short
	tests = append(tests, &testCase{
		_id:       "glob-root-target-too-short",
		_expected: false,

		cwd:   cwd,
		path:  filepath.Join(base, "projects", "slug"),
		roots: []string{globRoot},
	})

	// glob-root-sibling-file
	tests = append(tests, &testCase{
		_id:       "glob-root-sibling-file",
		_expected: false,

		cwd:   cwd,
		path:  filepath.Join(base, "projects", "slug", "session.jsonl"),
		roots: []string{globRoot},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, IsInsideRoots(test.cwd, test.path, test.roots))
		})
	}
}

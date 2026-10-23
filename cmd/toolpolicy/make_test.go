package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMakeArgs(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *makeInvocation
		_shouldPass bool

		args []string
	}

	cwd := "/repo/work"
	home := "/home/u"
	invocation := func(dir string, targets []string, assignments map[string]string) *makeInvocation {
		expected := &makeInvocation{assignments: assignments, dir: dir, targets: targets}
		return expected
	}

	tests := make([]*testCase, 0)

	// pass-bare-targets
	tests = append(tests, &testCase{_id: "pass-bare-targets", _expected: invocation(cwd, []string{"audit", "build"}, map[string]string{}), _shouldPass: true, args: []string{"audit", "build"}})

	// pass-dry-run
	tests = append(tests, &testCase{_id: "pass-dry-run", _expected: invocation(cwd, []string{"audit"}, map[string]string{}), _shouldPass: true, args: []string{"-n", "audit"}})

	// pass-jobs-cluster
	tests = append(tests, &testCase{_id: "pass-jobs-cluster", _expected: invocation(cwd, []string{"test"}, map[string]string{}), _shouldPass: true, args: []string{"-kj4", "test"}})

	// pass-version
	tests = append(tests, &testCase{_id: "pass-version", _expected: invocation(cwd, []string{}, map[string]string{}), _shouldPass: true, args: []string{"--version"}})

	// pass-directory-short
	tests = append(tests, &testCase{_id: "pass-directory-short", _expected: invocation("/repo/work/sub", []string{"audit"}, map[string]string{}), _shouldPass: true, args: []string{"-C", "sub", "audit"}})

	// pass-directory-attached
	tests = append(tests, &testCase{_id: "pass-directory-attached", _expected: invocation("/repo/work/sub", []string{"audit"}, map[string]string{}), _shouldPass: true, args: []string{"-Csub", "audit"}})

	// pass-directory-cumulative
	tests = append(tests, &testCase{_id: "pass-directory-cumulative", _expected: invocation("/repo/work/a/b", []string{"audit"}, map[string]string{}), _shouldPass: true, args: []string{"-C", "a", "-C", "b", "audit"}})

	// pass-directory-long
	tests = append(tests, &testCase{_id: "pass-directory-long", _expected: invocation("/repo/work/sub", []string{"audit"}, map[string]string{}), _shouldPass: true, args: []string{"--directory=sub", "audit"}})

	// pass-directory-prefix
	tests = append(tests, &testCase{_id: "pass-directory-prefix", _expected: invocation("/repo/work/sub", []string{"audit"}, map[string]string{}), _shouldPass: true, args: []string{"--dir", "sub", "audit"}})

	// pass-directory-home
	tests = append(tests, &testCase{_id: "pass-directory-home", _expected: invocation("/home/u/x", []string{"audit"}, map[string]string{}), _shouldPass: true, args: []string{"-C", "~/x", "audit"}})

	// pass-directory-absolute
	tests = append(tests, &testCase{_id: "pass-directory-absolute", _expected: invocation("/abs", []string{"audit"}, map[string]string{}), _shouldPass: true, args: []string{"-C", "/abs", "audit"}})

	// pass-assignment
	tests = append(tests, &testCase{_id: "pass-assignment", _expected: invocation(cwd, []string{"acdsl"}, map[string]string{"ARGS": "check -rule x"}), _shouldPass: true, args: []string{"acdsl", "ARGS=check -rule x"}})

	// fail-file-flag
	tests = append(tests, &testCase{_id: "fail-file-flag", _expected: nil, _shouldPass: false, args: []string{"-f", "x.mk", "t"}})

	// fail-file-in-cluster
	tests = append(tests, &testCase{_id: "fail-file-in-cluster", _expected: nil, _shouldPass: false, args: []string{"-sf", "x", "t"}})

	// fail-eval
	tests = append(tests, &testCase{_id: "fail-eval", _expected: nil, _shouldPass: false, args: []string{"--eval=x"}})

	// fail-eval-prefix
	tests = append(tests, &testCase{_id: "fail-eval-prefix", _expected: nil, _shouldPass: false, args: []string{"--ev", "x"}})

	// fail-makefile-long
	tests = append(tests, &testCase{_id: "fail-makefile-long", _expected: nil, _shouldPass: false, args: []string{"--makefile=x", "t"}})

	// fail-include
	tests = append(tests, &testCase{_id: "fail-include", _expected: nil, _shouldPass: false, args: []string{"-I", "dir", "t"}})

	// fail-env-overrides
	tests = append(tests, &testCase{_id: "fail-env-overrides", _expected: nil, _shouldPass: false, args: []string{"-e", "t"}})

	// fail-directory-missing-value
	tests = append(tests, &testCase{_id: "fail-directory-missing-value", _expected: nil, _shouldPass: false, args: []string{"-C"}})

	// fail-value-metachar
	tests = append(tests, &testCase{_id: "fail-value-metachar", _expected: nil, _shouldPass: false, args: []string{"acdsl", "ARGS=a;b"}})

	// fail-bad-variable-name
	tests = append(tests, &testCase{_id: "fail-bad-variable-name", _expected: nil, _shouldPass: false, args: []string{"1X=a"}})

	// fail-path-target
	tests = append(tests, &testCase{_id: "fail-path-target", _expected: nil, _shouldPass: false, args: []string{"../x"}})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			parsed, isParsed := parseMakeArgs(test.args, cwd, home)
			assert.Equal(t, test._shouldPass, isParsed)
			assert.Equal(t, test._expected, parsed)
		})
	}
}

func TestMakeParams(t *testing.T) {
	type testCase struct {
		_id       string
		_expected map[string][]string

		makefile string
	}

	tests := make([]*testCase, 0)

	// help-line-param
	tests = append(tests, &testCase{
		_id:       "help-line-param",
		_expected: map[string][]string{"acdsl": {"ARGS"}},

		makefile: "## acdsl: run the acdsl runner — ARGS=\"check\" | \"fixtures\"\nacdsl:\n\tgo run ./cmd/acdsl $(ARGS)\n",
	})

	// two-params
	tests = append(tests, &testCase{
		_id:       "two-params",
		_expected: map[string][]string{"rust-load": {"REPO", "OLD"}},

		makefile: "## rust-load: load evidence, plus REPO=<clone> and OLD=<checkout>\n",
	})

	// no-param-help-line
	tests = append(tests, &testCase{
		_id:       "no-param-help-line",
		_expected: map[string][]string{},

		makefile: "## build: build all binaries\nbuild:\n\tgo build ./...\n",
	})

	// plain-comment-ignored
	tests = append(tests, &testCase{
		_id:       "plain-comment-ignored",
		_expected: map[string][]string{},

		makefile: "# acdsl: ARGS=\"check\"\n",
	})

	// empty-makefile
	tests = append(tests, &testCase{
		_id:       "empty-makefile",
		_expected: map[string][]string{},

		makefile: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, makeParams(test.makefile))
		})
	}
}

func TestMakeParams_RepoMakefile(t *testing.T) {
	raw, err := os.ReadFile("../../Makefile")
	require.NoError(t, err)

	params := makeParams(string(raw))

	assert.Equal(t, []string{"ARGS"}, params["acdsl"])
	assert.Equal(t, []string{"ARGS"}, params["context-record"])
	assert.Equal(t, []string{"ARGS"}, params["rules"])
	assert.Equal(t, []string{"REPO", "OLD"}, params["rust-load"])
	assert.Equal(t, []string{"SKILL", "JSON", "PHASES"}, params["model-diagram"])
	assert.Equal(t, []string{"build-install", "install-full", "docs-deploy", "serve"}, makeOperatorTargets(string(raw)))
}

func TestIsSafeMakeCall(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedIsMake bool
		_expectedSafe   bool

		cmd string
	}

	policy := defaultToolpolicy(t)

	home := t.TempDir()
	cwd := t.TempDir()
	makefile := "## acdsl: run acdsl — ARGS=\"check\"\n## test-shell: run shell tests; T=<name> filters\n## test: run tests\n"
	require.NoError(t, os.WriteFile(filepath.Join(cwd, "Makefile"), []byte(makefile), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(cwd, "other"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cwd, "other", "GNUmakefile"), []byte("## deploy: ship — VERSION=x.y.z\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(cwd, "empty"), 0o755))

	tests := make([]*testCase, 0)

	// make-bare-target
	tests = append(tests, &testCase{_id: "make-bare-target", _expectedIsMake: true, _expectedSafe: true, cmd: `make test`})

	// make-declared-param
	tests = append(tests, &testCase{_id: "make-declared-param", _expectedIsMake: true, _expectedSafe: true, cmd: `make test-shell T=sync_skills_variant`})

	// make-declared-args
	tests = append(tests, &testCase{_id: "make-declared-args", _expectedIsMake: true, _expectedSafe: true, cmd: `make acdsl ARGS="project -plan Makefile"`})

	// make-dry-run
	tests = append(tests, &testCase{_id: "make-dry-run", _expectedIsMake: true, _expectedSafe: true, cmd: `make -n test`})

	// make-undeclared-variable
	tests = append(tests, &testCase{_id: "make-undeclared-variable", _expectedIsMake: true, _expectedSafe: false, cmd: `make test GO=evil`})

	// make-shell-override
	tests = append(tests, &testCase{_id: "make-shell-override", _expectedIsMake: true, _expectedSafe: false, cmd: `make test-shell SHELL=/tmp/x T=a`})

	// make-param-of-other-target
	tests = append(tests, &testCase{_id: "make-param-of-other-target", _expectedIsMake: true, _expectedSafe: false, cmd: `make acdsl T=x`})

	// make-flag-file
	tests = append(tests, &testCase{_id: "make-flag-file", _expectedIsMake: true, _expectedSafe: false, cmd: `make test -f evil.mk`})

	// make-flag-eval
	tests = append(tests, &testCase{_id: "make-flag-eval", _expectedIsMake: true, _expectedSafe: false, cmd: `make test --eval=x`})

	// make-directory-reads-that-makefile
	tests = append(tests, &testCase{_id: "make-directory-reads-that-makefile", _expectedIsMake: true, _expectedSafe: true, cmd: `make -C other deploy VERSION=1.2.8`})

	// make-directory-param-not-declared-there
	tests = append(tests, &testCase{_id: "make-directory-param-not-declared-there", _expectedIsMake: true, _expectedSafe: false, cmd: `make -C other test-shell T=x`})

	// make-no-makefile-bare-target
	tests = append(tests, &testCase{_id: "make-no-makefile-bare-target", _expectedIsMake: true, _expectedSafe: true, cmd: `make -C empty build`})

	// make-no-makefile-assignment
	tests = append(tests, &testCase{_id: "make-no-makefile-assignment", _expectedIsMake: true, _expectedSafe: false, cmd: `make -C empty build X=1`})

	// make-value-injection
	tests = append(tests, &testCase{_id: "make-value-injection", _expectedIsMake: true, _expectedSafe: false, cmd: `make acdsl ARGS='check; rm -rf /'`})

	// make-value-subst
	tests = append(tests, &testCase{_id: "make-value-subst", _expectedIsMake: true, _expectedSafe: false, cmd: `make acdsl ARGS="$(cat x)"`})

	// make-nonliteral-target
	tests = append(tests, &testCase{_id: "make-nonliteral-target", _expectedIsMake: true, _expectedSafe: false, cmd: `make $T`})

	// make-wrapped-timeout
	tests = append(tests, &testCase{_id: "make-wrapped-timeout", _expectedIsMake: true, _expectedSafe: true, cmd: `timeout 60 make test`})

	// not-make
	tests = append(tests, &testCase{_id: "not-make", _expectedIsMake: false, _expectedSafe: false, cmd: `go test ./...`})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			call, isMake := singleMakeCall(test.cmd, policy)
			assert.Equal(t, test._expectedIsMake, isMake)
			assert.Equal(t, test._expectedSafe, isMake && isSafeMakeCall(call, cwd, home, policy.MakefileNames))
		})
	}
}

func TestMakeOperatorTargets(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		makefile string
	}

	tests := make([]*testCase, 0)

	// tagged-target
	tests = append(tests, &testCase{_id: "tagged-target", _expected: []string{"deploy"}, makefile: "## deploy: [operator] ship the build\ndeploy:\n"})

	// untagged-target
	tests = append(tests, &testCase{_id: "untagged-target", _expected: []string{}, makefile: "## test: run tests\ntest:\n"})

	// tag-outside-help-line
	tests = append(tests, &testCase{_id: "tag-outside-help-line", _expected: []string{}, makefile: "# deploy: [operator] ship\ndeploy: # [operator]\n"})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, makeOperatorTargets(test.makefile))
		})
	}
}

func TestIsOperatorMakeCall(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		cmd string
	}

	policy := defaultToolpolicy(t)

	home := t.TempDir()
	cwd := t.TempDir()
	makefile := "## deploy: [operator] ship the build\n## test: run tests\n"
	require.NoError(t, os.WriteFile(filepath.Join(cwd, "Makefile"), []byte(makefile), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(cwd, "other"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cwd, "other", "GNUmakefile"), []byte("## test: [operator] run tests\n"), 0o644))

	tests := make([]*testCase, 0)

	// tagged-target
	tests = append(tests, &testCase{_id: "tagged-target", _expected: true, cmd: `make deploy`})

	// untagged-target
	tests = append(tests, &testCase{_id: "untagged-target", _expected: false, cmd: `make test`})

	// two-targets-one-tagged
	tests = append(tests, &testCase{_id: "two-targets-one-tagged", _expected: true, cmd: `make test deploy`})

	// directory-flag-reads-that-makefile
	tests = append(tests, &testCase{_id: "directory-flag-reads-that-makefile", _expected: true, cmd: `make -C other test`})

	// unparsable-call
	tests = append(tests, &testCase{_id: "unparsable-call", _expected: false, cmd: `make -f x deploy`})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			call, isMake := singleMakeCall(test.cmd, policy)
			assert.True(t, isMake)
			assert.Equal(t, test._expected, isOperatorMakeCall(call, cwd, home, policy.MakefileNames))
		})
	}
}

func TestIsMakeSource(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		path string
	}

	makefileNames := defaultToolpolicy(t).MakefileNames

	tests := make([]*testCase, 0)

	// makefile
	tests = append(tests, &testCase{_id: "makefile", _expected: true, path: "/repo/Makefile"})

	// lowercase-makefile
	tests = append(tests, &testCase{_id: "lowercase-makefile", _expected: true, path: "makefile"})

	// gnumakefile
	tests = append(tests, &testCase{_id: "gnumakefile", _expected: true, path: "sub/GNUmakefile"})

	// include-file
	tests = append(tests, &testCase{_id: "include-file", _expected: true, path: "build/rules.mk"})

	// nested-makefile
	tests = append(tests, &testCase{_id: "nested-makefile", _expected: true, path: "cmd/x/Makefile"})

	// backup-file
	tests = append(tests, &testCase{_id: "backup-file", _expected: false, path: "Makefile.bak"})

	// go-file
	tests = append(tests, &testCase{_id: "go-file", _expected: false, path: "cmd/mk.go"})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, isMakeSource(test.path, makefileNames))
		})
	}
}

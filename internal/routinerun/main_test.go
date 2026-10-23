package routinerun

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	fakeClaudeVersion    = "9.9.9 (Claude Code)"
	fakeDockerFailEnv    = "ROUTINERUN_FAKE_DOCKER_FAIL"
	fakeDockerGatewayEnv = "ROUTINERUN_FAKE_DOCKER_GATEWAY"
	fakeDockerLogEnv     = "ROUTINERUN_FAKE_DOCKER_LOG"
	fakeGatewayAnswer    = "192.0.2.254 STREAM host.docker.internal\n192.0.2.254 DGRAM\n"
)

// The test binary doubles as fake children: invoked through a `claude`
// symlink it plays a stream-json claude session; with ROUTINERUN_FAKE_CHILD
// set it plays a plain cell child.
const (
	fakeChildEnv        = "ROUTINERUN_FAKE_CHILD"
	fakeCommitEnv       = "ROUTINERUN_FAKE_COMMIT"
	fakeEchoEnv         = "ROUTINERUN_FAKE_ECHO_ENV"
	fakeEnvelopeEnv     = "ROUTINERUN_FAKE_ENVELOPE"
	fakeExitAfterEnv    = "ROUTINERUN_FAKE_EXIT_AFTER"
	fakeHeadPlaceholder = "{HEAD}"
	fakeHangEnv         = "ROUTINERUN_FAKE_HANG"
	fakeHangMarkerEnv   = "ROUTINERUN_FAKE_HANG_MARKER" // file the hanging fake writes, so a test cancels a cell that really runs
	fakeMarkerBodyEnv   = "ROUTINERUN_FAKE_MARKER_BODY"
	fakeMarkerEnv       = "ROUTINERUN_FAKE_MARKER"
	fakeScriptEnv       = "ROUTINERUN_FAKE_SCRIPT"
	fakeSilentEnv       = "ROUTINERUN_FAKE_SILENT"
	fakeResultEnvelope  = `{"type":"result","subtype":"success","is_error":false,"session_id":"session-1","num_turns":2,"total_cost_usd":0.5,"result":%s}`
)

type fakeMessage struct {
	Message fakeMessageBody `json:"message"`
}

type fakeMessageBody struct {
	Content string `json:"content"`
}

func TestMain(m *testing.M) {
	switch filepath.Base(os.Args[0]) {
	case "claude":
		os.Exit(fakeClaude())
	case "docker":
		os.Exit(fakeDocker())
	case fakeAgentwtName:
		os.Exit(fakeAgentwt(os.Args[1:]))
	}
	if mode := os.Getenv(fakeChildEnv); mode != "" {
		os.Exit(fakeChild(mode))
	}
	os.Exit(m.Run())
}

// fakeChild: "exit:N" exits N, "env:NAME" prints $NAME, "sleep" ignores TERM-free waiting until killed.
func fakeChild(mode string) int {
	if name, ok := strings.CutPrefix(mode, "env:"); ok {
		fmt.Fprintln(os.Stdout, os.Getenv(name))
		return 0
	}
	if code, ok := strings.CutPrefix(mode, "exit:"); ok {
		status, _ := strconv.Atoi(code)
		fmt.Fprintln(os.Stdout, "child output")
		return status
	}
	time.Sleep(time.Hour)
	return 0
}

// fakeClaude answers every stream-json user message with an assistant event
// and a result echoing the message; optional per-message side effects come
// from ROUTINERUN_FAKE_SCRIPT (prompt prefix → file to write in cwd).
func fakeClaude() int {
	isVersionCall := len(os.Args) > 1 && os.Args[1] == "--version"
	if isVersionCall {
		fmt.Fprintln(os.Stdout, fakeClaudeVersion)
		return 0
	}
	if os.Getenv(fakeHangEnv) != "" {
		if marker := os.Getenv(fakeHangMarkerEnv); marker != "" {
			_ = os.WriteFile(marker, nil, 0o644)
		}
		time.Sleep(time.Hour)
		return 0
	}

	exitAfter, _ := strconv.Atoi(os.Getenv(fakeExitAfterEnv))
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	answered := 0
	for scanner.Scan() {
		var message fakeMessage
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			continue
		}
		applyFakeScript(message.Message.Content)
		head := commitFakeChange()
		// The marker lands before the result event, as a real cell's Write tool call does inside the turn.
		if marker := os.Getenv(fakeMarkerEnv); marker != "" {
			body := strings.ReplaceAll(os.Getenv(fakeMarkerBodyEnv), fakeHeadPlaceholder, head)
			_ = os.MkdirAll(filepath.Dir(marker), 0o755)
			_ = os.WriteFile(marker, []byte(body), 0o644)
		}
		if os.Getenv(fakeSilentEnv) == "" {
			writeFakeTurn(message.Message.Content)
		}
		answered++
		if exitAfter > 0 && answered >= exitAfter {
			return 3
		}
	}
	return 0
}

// commitFakeChange plays the implement stage: with ROUTINERUN_FAKE_COMMIT set
// it commits everything outside sessions/ under that message and returns HEAD.
func commitFakeChange() string {
	message := os.Getenv(fakeCommitEnv)
	if message == "" {
		return ""
	}

	ctx := context.Background()
	if _, err := git(ctx, ".", "add", "-A", "--", ".", ":!"+ledgerDir); err != nil {
		return ""
	}
	if _, err := git(ctx, ".", "commit", "--quiet", "-m", message); err != nil {
		return ""
	}
	head, _ := git(ctx, ".", "rev-parse", "HEAD")
	return head
}

// applyFakeScript: ROUTINERUN_FAKE_SCRIPT is "prefix=relative/path=content;…".
func applyFakeScript(prompt string) {
	for rule := range strings.SplitSeq(os.Getenv(fakeScriptEnv), ";") {
		parts := strings.SplitN(rule, "=", 3)
		if len(parts) != 3 {
			continue
		}
		if !strings.HasPrefix(prompt, parts[0]) {
			continue
		}
		_ = os.MkdirAll(filepath.Dir(parts[1]), 0o755)
		_ = os.WriteFile(parts[1], []byte(parts[2]+"\n"), 0o644)
	}
}

// fakeDocker appends its argv as one line to ROUTINERUN_FAKE_DOCKER_LOG,
// fails the subcommand named by ROUTINERUN_FAKE_DOCKER_FAIL with exit 1,
// answers getent with ROUTINERUN_FAKE_DOCKER_GATEWAY and plays the fake
// claude for an interactive run — the cell.
func fakeDocker() int {
	line := strings.Join(os.Args[1:], " ") + "\n"
	logFile, err := os.OpenFile(os.Getenv(fakeDockerLogEnv), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err == nil {
		_, _ = logFile.WriteString(line)
		logFile.Close()
	}
	if len(os.Args) < 2 || os.Args[1] == os.Getenv(fakeDockerFailEnv) {
		fmt.Fprintln(os.Stderr, "fake docker: failed")
		return 1
	}

	switch {
	case slices.Contains(os.Args, "getent"):
		fmt.Fprint(os.Stdout, os.Getenv(fakeDockerGatewayEnv))
		return 0
	case slices.Contains(os.Args, "--interactive"):
		return fakeClaude()
	}
	return 0
}

// writeFakeTurn prints one assistant turn: ROUTINERUN_FAKE_ENVELOPE replaces
// the result line verbatim, else the result echoes the prompt — or the env
// var ROUTINERUN_FAKE_ECHO_ENV names.
func writeFakeTurn(prompt string) {
	fmt.Fprintln(os.Stdout, `{"type":"assistant","message":{}}`)
	if name := os.Getenv(fakeEchoEnv); name != "" {
		prompt = os.Getenv(name)
	}
	if envelope := os.Getenv(fakeEnvelopeEnv); envelope != "" {
		fmt.Fprintln(os.Stdout, envelope)
	} else {
		quoted, _ := json.Marshal(prompt)
		fmt.Fprintf(os.Stdout, fakeResultEnvelope+"\n", quoted)
	}
	fmt.Fprintln(os.Stdout, `{"type":"system","subtype":"task_summary"}`)
}

// installFakeClaude puts a `claude` symlink to the test binary first on PATH.
func installFakeClaude(t *testing.T) {
	t.Helper()
	binDir := t.TempDir()
	executable, err := os.Executable()
	require.NoError(t, err)
	require.NoError(t, os.Symlink(executable, filepath.Join(binDir, "claude")))
	// The dirs prepareProcessEnv prepends stay behind the fake so they never shadow it.
	t.Setenv("PATH", strings.Join([]string{binDir, smineBinDir(), dockerDesktopBin, homebrewRustupBin, homebrewBin, os.Getenv("PATH")}, string(os.PathListSeparator)))
}

// installFakeDocker puts a `docker` symlink to the test binary first on PATH
// and returns the argv log path.
func installFakeDocker(t *testing.T) string {
	t.Helper()
	binDir := t.TempDir()
	executable, err := os.Executable()
	require.NoError(t, err)
	require.NoError(t, os.Symlink(executable, filepath.Join(binDir, "docker")))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	logPath := filepath.Join(t.TempDir(), "docker.log")
	t.Setenv(fakeDockerLogEnv, logPath)
	t.Setenv(fakeDockerGatewayEnv, fakeGatewayAnswer)
	return logPath
}

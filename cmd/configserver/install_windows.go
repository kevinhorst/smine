//go:build windows

package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/shell"
)

const serverTaskName = "configserver"

// serverTaskTemplate is the logon task for the config server — the
// LaunchAgent analog: logon trigger, restart-on-crash (bounded; next logon is
// the backstop), no execution time limit, single instance. Holes: command,
// arguments.
const serverTaskTemplate = `<?xml version="1.0"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <Triggers>
    <LogonTrigger><Enabled>true</Enabled></LogonTrigger>
  </Triggers>
  <Principals>
    <Principal id="Author">
      <LogonType>InteractiveToken</LogonType>
      <RunLevel>LeastPrivilege</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <StartWhenAvailable>true</StartWhenAvailable>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Enabled>true</Enabled>
    <RestartOnFailure>
      <Interval>PT1M</Interval>
      <Count>3</Count>
    </RestartOnFailure>
  </Settings>
  <Actions Context="Author">
    <Exec>
      <Command>%s</Command>
      <Arguments>%s</Arguments>
      <WorkingDirectory>%s</WorkingDirectory>
    </Exec>
  </Actions>
</Task>
`

// runInstall is the Windows single-source install (addendum M3): bootstrap
// (bash locate, claude/peek deploy, PATH, syncs) plus the scheduling half.
// install.ps1 (from-source) and smine-setup.exe both delegate here.
// Routine sync is NOT done here: the
// server task started by registerServerTask runs SyncAll at boot (main.go),
// and a second concurrent SyncAll from this process races it on
// Register-ScheduledTask (observed: all three routines "sync failed" in the
// server log while the installer's copies won).
func runInstall(ctx context.Context, addr string, initWelcome bool, peekPort, peekControlPort int) int {
	repoDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "install: getwd: %v\n", err)
		return 1
	}
	if code, done := relaunchElevated(); done { // A11 — Register-ScheduledTask needs an elevated token
		return code
	}
	bash := shell.BashPath()
	if bash == "" {
		fmt.Fprintln(os.Stderr, "install: Git Bash not found (set SMINE_BASH)")
		return 1
	}
	if output, err := shell.Run(ctx, repoDir, filepath.Join(repoDir, "cmd", "sync", "ensure_git_repo.sh")); err != nil {
		fmt.Fprintf(os.Stderr, "install: ensure_git_repo.sh: %v\n%s\n", err, output)
		return 1
	}
	shimDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "smine", "bin")
	if err := ensureUserPath(shimDir); err != nil {
		fmt.Fprintf(os.Stderr, "install: %v\n", err)
		return 1
	}
	claudeFound := deployClaude(ctx, repoDir, shimDir, bash)
	deployPeek(repoDir, shimDir)
	if err := runSyncs(ctx, repoDir, bash); err != nil {
		fmt.Fprintf(os.Stderr, "install: %v\n", err)
		return 1
	}

	if err := materializePlists(repoDir); err != nil {
		fmt.Fprintf(os.Stderr, "install: %v\n", err)
		return 1
	}
	if err := takeOverPort(ctx, addr); err != nil {
		fmt.Fprintf(os.Stderr, "install: %v\n", err)
		return 1
	}
	if err := registerServerTask(ctx, repoDir, addr, initWelcome, peekPort, peekControlPort); err != nil {
		fmt.Fprintf(os.Stderr, "install: %v\n", err)
		return 1
	}

	if err := verifyServing(ctx, addr); err != nil {
		fmt.Fprintf(os.Stderr, "install: %v\n", err)
		return 1
	}
	if claudeFound {
		tokenCheck()
	} else {
		fmt.Println("NOTE: no claude CLI found (PATH or Claude Desktop) - routines will not run until one is installed")
	}
	fmt.Printf("configserver logon task registered; serving on %s\n", addr)
	return 0
}

// materializePlists mirrors install.sh: routines/*/*.plist.template ->
// .plist, skip-if-exists (edited copies belong to the config server),
// forward-slash paths so Duplicate's string rewrite stays separator-stable
// (plan D1).
func materializePlists(repoDir string) error {
	templates, err := filepath.Glob(filepath.Join(repoDir, "routines", "*", "*.plist.template"))
	if err != nil {
		return fmt.Errorf("materializePlists: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("materializePlists: %w", err)
	}
	for _, template := range templates {
		target := strings.TrimSuffix(template, ".template")
		if _, err := os.Stat(target); err == nil {
			fmt.Printf("skip: %s exists (edit via the config server)\n", filepath.Base(target))
			continue
		}
		data, err := os.ReadFile(template)
		if err != nil {
			return fmt.Errorf("materializePlists: %w", err)
		}
		rendered := strings.ReplaceAll(string(data), "__REPO_DIR__", filepath.ToSlash(repoDir))
		rendered = strings.ReplaceAll(rendered, "__HOME__", filepath.ToSlash(home))
		if err := os.WriteFile(target, []byte(rendered), 0o644); err != nil {
			return fmt.Errorf("materializePlists: %w", err)
		}
		fmt.Printf("   %s\n", filepath.Base(target))
	}
	return nil
}

// takeOverPort mirrors install.sh's lsof takeover with a try-bind: a free
// port passes; a held port gets one stop of the smine server task and a
// retry; a still-held port is a foreign holder and fatal.
func takeOverPort(ctx context.Context, addr string) error {
	bindable := func() bool {
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			return false
		}
		listener.Close()
		return true
	}
	if bindable() {
		return nil
	}
	fmt.Printf("-> Stopping task holding %s ...\n", addr)
	command := fmt.Sprintf(`Stop-ScheduledTask -TaskPath '\smine\' -TaskName '%s' -ErrorAction SilentlyContinue`, serverTaskName)
	if _, err := shell.Run(ctx, "", "powershell", "-NoProfile", "-NonInteractive", "-Command", command); err != nil {
		return fmt.Errorf("takeOverPort: %w", err)
	}
	for attempt := 0; attempt < 10; attempt++ {
		time.Sleep(500 * time.Millisecond)
		if bindable() {
			return nil
		}
	}
	return fmt.Errorf("takeOverPort: %s is held by a foreign process — stop it and re-run", addr)
}

// registerServerTask registers \smine\configserver from serverTaskTemplate
// with the port flags in the task arguments (plan D11), then fires the
// immediate first run.
func registerServerTask(ctx context.Context, repoDir, addr string, initWelcome bool, peekPort, peekControlPort int) error {
	command := filepath.Join(repoDir, "bin", "configserver.exe")
	logPath := filepath.Join(os.Getenv("LOCALAPPDATA"), "claude-routine", "logs", "configserver.log")
	arguments := fmt.Sprintf(`-addr %s -init-welcome=%t -peek-port %d -peek-control-port %d -logfile "%s"`,
		addr, initWelcome, peekPort, peekControlPort, logPath)
	taskXML := fmt.Sprintf(serverTaskTemplate, xmlEscapeInstall(command), xmlEscapeInstall(arguments), xmlEscapeInstall(repoDir))

	xmlFile, err := os.CreateTemp("", "smine-configserver-*.xml")
	if err != nil {
		return fmt.Errorf("registerServerTask: %w", err)
	}
	defer os.Remove(xmlFile.Name())
	if _, err := xmlFile.WriteString(taskXML); err != nil {
		xmlFile.Close()
		return fmt.Errorf("registerServerTask: %w", err)
	}
	xmlFile.Close()

	// -User binds the principal to the current user: the template's Principal
	// has no <UserId>, and an unbound InteractiveToken principal is a
	// machine-wide task whose registration needs admin (0x80070005 from a
	// non-elevated shell). A self-bound task registers without elevation.
	register := fmt.Sprintf(
		`Register-ScheduledTask -TaskName '%s' -TaskPath '\smine\' -User $env:USERNAME -Xml (Get-Content -Raw '%s') -Force | Out-Null; Start-ScheduledTask -TaskPath '\smine\' -TaskName '%s'`,
		serverTaskName, xmlFile.Name(), serverTaskName)
	if output, err := shell.Run(ctx, "", "powershell", "-NoProfile", "-NonInteractive", "-Command", register); err != nil {
		return fmt.Errorf("registerServerTask: %s: %w (if access is still denied, re-run once from an elevated PowerShell)", output, err)
	}
	return nil
}

// verifyServing mirrors install.sh's served-by loop: the port must answer
// HTTP within the window, else the install failed loudly. Generous budgets:
// the freshly started server runs SyncAll before it listens, and the overview
// render probes every routine's task state — each a powershell spawn, seconds
// apiece on a cold Windows box. A 2s client timeout aborted every poll
// mid-render (the "superfluous WriteHeader" log noise) and failed the install
// against a healthy server.
func verifyServing(ctx context.Context, addr string) error {
	url := "http://127.0.0.1" + addr
	if !strings.HasPrefix(addr, ":") {
		url = "http://" + addr
	}
	client := &http.Client{Timeout: 15 * time.Second}
	for attempt := 0; attempt < 60; attempt++ {
		time.Sleep(500 * time.Millisecond)
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("verifyServing: %w", err)
		}
		response, err := client.Do(request)
		if err != nil {
			continue
		}
		response.Body.Close()
		if response.StatusCode == http.StatusOK {
			return nil
		}
	}
	return fmt.Errorf("verifyServing: %s did not answer — check %%LOCALAPPDATA%%\\claude-routine\\logs and Task Scheduler history", addr)
}

// tokenCheck prints the routine-token guidance when the file is missing —
// routines exit 78 without it (plan D8; path shared with macOS).
func tokenCheck() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	tokenPath := filepath.Join(home, ".config", "claude-routine", "token")
	if info, err := os.Stat(tokenPath); err == nil && info.Size() > 0 {
		return
	}
	if labels, err := routines.ListTokenLabels(filepath.Join(home, ".config", "claude-routine", "tokens")); err == nil && len(labels) > 0 {
		return
	}
	fmt.Printf("NOTE: routine OAuth token missing: %s\n", tokenPath)
	fmt.Println("      run `claude setup-token` and save the token there, or add a labeled token via the config server Configure widget — routines exit 78 without it")
}

var installEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")

func xmlEscapeInstall(value string) string {
	return installEscaper.Replace(value)
}

// deployClaude makes `claude` callable for routines: a CLI already visible
// from Git Bash wins (native installer or npm); otherwise Claude Desktop's
// bundled claude.exe gets the repo's bash shim deployed onto the shim dir.
// Neither found is a warning, not an error (DR4): install completes, the
// token prompt is skipped, routines exit 127 until a runtime appears.
func deployClaude(ctx context.Context, repoDir, shimDir, bash string) bool {
	if _, err := shell.Run(ctx, "", bash, "-lc", "command -v claude"); err == nil {
		fmt.Println("-> claude: found on PATH")
		return true
	}
	glob := filepath.Join(os.Getenv("LOCALAPPDATA"),
		`Packages\Claude_*\LocalCache\Roaming\Claude\claude-code\*\claude.exe`)
	matches, _ := filepath.Glob(glob)
	if len(matches) == 0 {
		fmt.Println("-> claude: not found (PATH or Claude Desktop) - install the claude CLI or Claude Desktop for routines")
		return false
	}
	// The shim re-resolves the newest Desktop version per call; deploying it
	// is all that's needed (claude-shim.sh contract unchanged).
	if err := fsx.CopyFile(filepath.Join(repoDir, "claude-shim.sh"), filepath.Join(shimDir, "claude")); err != nil {
		fmt.Printf("-> claude: shim deploy failed: %v\n", err)
		return false
	}
	fmt.Println("-> claude: Claude Desktop shim deployed")
	return true
}

// deployPeek refreshes shimDir\peek-mcp.exe: SMINE_PEEK_BIN override, then a
// sibling peek-mcp checkout, then an already-present copy (installer-bundled).
// Missing everywhere is a warning — the session column degrades. A locked
// destination keeps the old exe (a running peek-mcp holds the file).
func deployPeek(repoDir, shimDir string) {
	dest := filepath.Join(shimDir, "peek-mcp.exe")
	source := os.Getenv("SMINE_PEEK_BIN")
	if source == "" {
		source = filepath.Join(filepath.Dir(repoDir), "peek-mcp", "dist", "peek-mcp-windows-amd64.exe")
	}
	if _, err := os.Stat(source); err != nil {
		if _, err := os.Stat(dest); err == nil {
			fmt.Printf("-> peek-mcp: keeping %s (no fresher source)\n", dest)
		} else {
			fmt.Printf("-> peek-mcp not found (%s, set SMINE_PEEK_BIN) - session column degrades\n", source)
		}
		return
	}
	if err := fsx.CopyFile(source, dest); err != nil {
		fmt.Println("-> peek-mcp.exe in use, not refreshed (stop it and re-run to update)")
		return
	}
	fmt.Printf("-> peek-mcp: %s\n", source)
}

// runSyncs applies settings/hooks/skills through Git Bash — shell.Run's .sh
// rewrite resolves the same bash. ensureUserPath already prepended the shim
// dir to this process's PATH, so the scripts' jq resolves the shipped jq.exe.
func runSyncs(ctx context.Context, repoDir, bash string) error {
	fmt.Printf("-> Syncing settings/hooks/skills (bash: %s) ...\n", bash)
	for _, script := range []string{"sync_settings.sh", "sync_hooks.sh", "sync_skills.sh"} {
		if output, err := shell.Run(ctx, repoDir, filepath.Join(repoDir, "cmd", "sync", script)); err != nil {
			return fmt.Errorf("runSyncs: %s: %w\n%s", script, err, output)
		}
	}
	return nil
}

// ensureUserPath appends dir to HKCU\Environment Path (expandsz) when absent
// — case-insensitive segment match, never a substring check — and broadcasts
// WM_SETTINGCHANGE so new shells see it. Also prepends dir to this process's
// PATH for the steps that follow.
func ensureUserPath(dir string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("ensureUserPath: %w", err)
	}
	defer key.Close()

	current, valtype, err := key.GetStringValue("Path")
	if err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("ensureUserPath: %w", err)
	}
	present := false
	for _, segment := range strings.Split(current, ";") {
		if strings.EqualFold(strings.TrimSpace(segment), dir) {
			present = true
			break
		}
	}
	if !present {
		updated := dir
		if current != "" {
			updated = current + ";" + dir
		}
		// Preserve the value's type; a fresh value is expandsz like the
		// system-provisioned Path.
		if valtype == registry.SZ {
			err = key.SetStringValue("Path", updated)
		} else {
			err = key.SetExpandStringValue("Path", updated)
		}
		if err != nil {
			return fmt.Errorf("ensureUserPath: %w", err)
		}
		broadcastEnvironmentChange()
		fmt.Printf("-> PATH: %s registered (user)\n", dir)
	}
	return os.Setenv("PATH", dir+";"+os.Getenv("PATH"))
}

var (
	modUser32               = windows.NewLazySystemDLL("user32.dll")
	procSendMessageTimeoutW = modUser32.NewProc("SendMessageTimeoutW")
	modShell32              = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteExW     = modShell32.NewProc("ShellExecuteExW")
)

// broadcastEnvironmentChange tells running shells the user environment
// changed (best-effort; x/sys exports no SendMessageTimeout wrapper).
func broadcastEnvironmentChange() {
	const hwndBroadcast = 0xffff
	const wmSettingChange = 0x001a
	const smtoAbortIfHung = 0x0002
	env, err := windows.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}
	procSendMessageTimeoutW.Call(hwndBroadcast, wmSettingChange, 0,
		uintptr(unsafe.Pointer(env)), smtoAbortIfHung, 5000, 0)
}

// shellExecuteInfo is SHELLEXECUTEINFOW — x/sys has plain ShellExecute only,
// which returns no process handle to wait on.
type shellExecuteInfo struct {
	cbSize         uint32
	fMask          uint32
	hwnd           windows.Handle
	lpVerb         *uint16
	lpFile         *uint16
	lpParameters   *uint16
	lpDirectory    *uint16
	nShow          int32
	hInstApp       windows.Handle
	lpIDList       uintptr
	lpClass        *uint16
	hkeyClass      windows.Handle
	dwHotKey       uint32
	hIconOrMonitor windows.Handle
	hProcess       windows.Handle
}

// relaunchElevated re-runs this binary elevated when the current token is
// not (A11): ShellExecuteW verb "runas" with the original args plus
// -logfile %TEMP%\smine-install.log, waits for exit, returns (exitCode,
// true). An already-elevated process returns (0, false) and continues
// inline. A declined UAC prompt is a fatal install error.
func relaunchElevated() (int, bool) {
	if windows.GetCurrentProcessToken().IsElevated() {
		return 0, false
	}

	logPath := filepath.Join(os.Getenv("TEMP"), "smine-install.log")
	os.Remove(logPath)
	args := os.Args[1:]
	hasLogfile := false
	for _, arg := range args {
		if arg == "-logfile" || arg == "--logfile" {
			hasLogfile = true
			break
		}
	}
	if !hasLogfile {
		args = append(args, "-logfile", logPath)
	}
	escaped := make([]string, 0, len(args))
	for _, arg := range args {
		escaped = append(escaped, syscall.EscapeArg(arg))
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "install: executable path: %v\n", err)
		return 1, true
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "install: getwd: %v\n", err)
		return 1, true
	}
	code, err := shellExecuteWait("runas", exe, strings.Join(escaped, " "), cwd)
	if err != nil {
		// ERROR_CANCELLED: the user declined the UAC prompt.
		fmt.Fprintf(os.Stderr, "install: elevation failed: %v\n", err)
		return 1, true
	}
	// The elevated child logs to a file (windowsgui, no shared console);
	// replay it so the console/dev path still shows the output.
	if content, err := os.ReadFile(logPath); err == nil {
		fmt.Print(string(content))
	}
	return code, true
}

// shellExecuteWait runs file via ShellExecuteExW with the given verb, waits
// for the child to exit and returns its exit code.
func shellExecuteWait(verb, file, params, cwd string) (int, error) {
	const seeMaskNoCloseProcess = 0x00000040
	verbPtr, err := windows.UTF16PtrFromString(verb)
	if err != nil {
		return 0, fmt.Errorf("shellExecuteWait: %w", err)
	}
	filePtr, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return 0, fmt.Errorf("shellExecuteWait: %w", err)
	}
	paramsPtr, err := windows.UTF16PtrFromString(params)
	if err != nil {
		return 0, fmt.Errorf("shellExecuteWait: %w", err)
	}
	cwdPtr, err := windows.UTF16PtrFromString(cwd)
	if err != nil {
		return 0, fmt.Errorf("shellExecuteWait: %w", err)
	}

	info := shellExecuteInfo{
		fMask:        seeMaskNoCloseProcess,
		lpVerb:       verbPtr,
		lpFile:       filePtr,
		lpParameters: paramsPtr,
		lpDirectory:  cwdPtr,
		nShow:        windows.SW_HIDE, // the child is windowsgui; nothing to show
	}
	info.cbSize = uint32(unsafe.Sizeof(info))
	ok, _, callErr := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		return 0, fmt.Errorf("shellExecuteWait: %w", callErr)
	}
	if info.hProcess == 0 {
		return 0, fmt.Errorf("shellExecuteWait: no process handle")
	}
	defer windows.CloseHandle(info.hProcess)
	if _, err := windows.WaitForSingleObject(info.hProcess, windows.INFINITE); err != nil {
		return 0, fmt.Errorf("shellExecuteWait: %w", err)
	}
	var exitCode uint32
	if err := windows.GetExitCodeProcess(info.hProcess, &exitCode); err != nil {
		return 0, fmt.Errorf("shellExecuteWait: %w", err)
	}
	return int(exitCode), nil
}

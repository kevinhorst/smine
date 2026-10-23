//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/server"
	"github.com/kevinhorst/smine/internal/shell"
)

const serverTaskName = "configserver"

func prepareInstall(ctx context.Context, repoDir string, record install.Record, request *installRequest) (int, bool) {
	if code, done := relaunchElevated(); done { // A11 — Register-ScheduledTask needs an elevated token
		return code, true
	}

	bash := shell.BashPath()
	if bash == "" {
		fmt.Fprintln(os.Stderr, "install: Git Bash not found (set SMINE_BASH)")
		return 1, true
	}
	shimDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "smine", "bin")
	if err := ensureUserPath(shimDir); err != nil {
		fmt.Fprintf(os.Stderr, "install: %v\n", err)
		return 1, true
	}
	request.isClaudeFound = deployClaude(ctx, repoDir, shimDir, bash)
	if slices.Contains(record.Components, components.Peek) {
		deployPeek(repoDir, shimDir)
	}
	return 0, false
}

// Routine sync stays the server task's boot sync — a second concurrent SyncAll races it on Register-ScheduledTask.
func finishInstall(ctx context.Context, env install.Env, record install.Record, request *installRequest, results []install.Result) error {
	peekPort := request.peekPort
	if !slices.Contains(record.Components, components.Peek) {
		peekPort = 0
	}
	if err := takeOverPort(ctx, request.addr); err != nil {
		return err
	}
	if err := registerServerTask(ctx, env.RepoDir, request.addr, request.initWelcome, peekPort, request.peekControlPort); err != nil {
		return err
	}
	if err := verifyServing(ctx, request.addr); err != nil {
		return err
	}
	if slices.Contains(record.Components, components.Heartbeat) {
		if err := install.RestartHeartbeatService(ctx, env); err != nil {
			return err
		}
	}

	isTokenWanted := slices.Contains(record.Components, components.Routines) || slices.Contains(record.Components, components.Mining)
	switch {
	case !request.isClaudeFound:
		fmt.Println("NOTE: no claude CLI found (PATH or Claude Desktop) - routines will not run until one is installed")
	case isTokenWanted:
		tokenCheck()
	}
	fmt.Printf("configserver logon task registered; serving on %s\n", request.addr)
	return nil
}

// The logon task owns the server on Windows.
func restartServer(ctx context.Context, env install.Env, results []install.Result) error {
	return nil
}

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
	if _, err := routines.StopTask(ctx, serverTaskName); err != nil {
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

func registerServerTask(ctx context.Context, repoDir, addr string, initWelcome bool, peekPort, peekControlPort int) error {
	command := filepath.Join(repoDir, "bin", "configserver.exe")
	logPath := filepath.Join(os.Getenv("LOCALAPPDATA"), "claude-routine", "logs", "configserver.log")
	arguments := fmt.Sprintf(`-addr %s -init-welcome=%t -peek-port %d -peek-control-port %d -logfile "%s"`,
		addr, initWelcome, peekPort, peekControlPort, logPath)
	taskXML := routines.LogonTaskXML(arguments, command, repoDir)
	if output, err := routines.RegisterLogonTask(ctx, serverTaskName, taskXML); err != nil {
		return fmt.Errorf("registerServerTask: %s: %w (if access is still denied, re-run once from an elevated PowerShell)", output, err)
	}
	return nil
}

// A 2s client timeout aborted every poll mid-render and failed the install against a healthy server.
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
		isChallenge := response.StatusCode == http.StatusUnauthorized
		if isChallenge && strings.Contains(response.Header.Get("WWW-Authenticate"), server.CredentialRealm) {
			return nil
		}
	}
	return fmt.Errorf("verifyServing: %s did not answer — check %%LOCALAPPDATA%%\\claude-routine\\logs and Task Scheduler history", addr)
}

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
	fmt.Println("      run `claude setup-token` and save the token there, or add a labeled token on the config server's Profile page — routines exit 78 without it")
}

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
	if err := fsx.CopyFile(filepath.Join(repoDir, "claude-shim.sh"), filepath.Join(shimDir, "claude")); err != nil {
		fmt.Printf("-> claude: shim deploy failed: %v\n", err)
		return false
	}
	fmt.Println("-> claude: Claude Desktop shim deployed")
	return true
}

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

func ensureCredential(credentialPath string) error {
	_, err := os.Stat(credentialPath)
	if err == nil {
		return nil
	}

	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("ensureCredential: %w", err)
	}

	token, err := server.GenerateCredential(credentialPath)
	if err != nil {
		return err
	}

	caption, err := windows.UTF16PtrFromString("smine config server token")
	if err != nil {
		return fmt.Errorf("ensureCredential: %w", err)
	}

	text, err := windows.UTF16PtrFromString("Token (shown once):\n\n" + token + "\n\nCtrl+C copies this box. Save the token in your password manager — the browser asks for it as the password; any user name works.")
	if err != nil {
		return fmt.Errorf("ensureCredential: %w", err)
	}

	if _, err := windows.MessageBox(0, text, caption, windows.MB_OK|windows.MB_ICONINFORMATION); err != nil {
		return fmt.Errorf("ensureCredential: %w", err)
	}

	return nil
}

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
		// A fresh value is expandsz like the system-provisioned Path.
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

// Best-effort; x/sys exports no SendMessageTimeout wrapper.
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

// SHELLEXECUTEINFOW: x/sys has plain ShellExecute only, which returns no process handle to wait on.
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

func relaunchElevated() (int, bool) {
	if windows.GetCurrentProcessToken().IsElevated() {
		return 0, false
	}

	logPath := filepath.Join(os.Getenv("TEMP"), "smine-install.log")
	if err := os.Rename(logPath, logPath+".1"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return 0, false
	}

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
	if content, err := os.ReadFile(logPath); err == nil {
		fmt.Print(string(content))
	}
	return code, true
}

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

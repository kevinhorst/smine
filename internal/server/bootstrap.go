package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/kevinhorst/smine/internal/server/respond"
	"github.com/kevinhorst/smine/internal/shell"
)

const bootstrapDeadline = 6 * time.Hour

const tmplWelcomeBootstrap = "_welcome_bootstrap.html"

type bootstrapRun struct {
	mu        sync.Mutex
	exitCode  int
	exited    bool
	pid       int
	running   bool
	startedAt time.Time
}

type bootstrapView struct {
	Detail    string
	Running   bool
	SessionId string
}

// handleWelcomeBootstrap resolves claude through the login shell: the server's PATH lacks the claude shim.
func (s *Server) handleWelcomeBootstrap(w http.ResponseWriter, r *http.Request) {
	s.bootstrap.mu.Lock()
	defer s.bootstrap.mu.Unlock()
	if s.bootstrap.running {
		respond.WithConflict("a bootstrap run is already in progress", w)
		return
	}

	env, err := bootstrapEnv(r.FormValue("since"), r.FormValue("extra-prompt"))
	if err != nil {
		respond.WithBadRequest(err.Error(), w)
		return
	}

	// Token and model resolve like the profile style test.
	target := s.styleTestTarget()
	if target.TokenPath == "" {
		respond.WithConflict("bootstrap needs the routine token: "+target.Detail, w)
		return
	}
	bashPath := verifyBashPath()
	if bashPath == "" {
		respond.WithConflict("no bash available to run the bootstrap", w)
		return
	}

	// Seek to the end, not O_APPEND: the Git Bash child on Windows writes nothing through an inherited O_APPEND handle.
	logPath := filepath.Join(os.TempDir(), "smine-bootstrap.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	if _, err := logFile.Seek(0, io.SeekEnd); err != nil {
		logFile.Close()
		respond.WithInternalServerError(err, w)
		return
	}

	// Multi-hour child under its own backstop deadline, far above any real bootstrap run.
	script := verifyPathPrefix + `exec "$1" bootstrap`
	runCtx, cancel := context.WithTimeout(context.Background(), bootstrapDeadline)
	// The server's cwd is the main checkout; bash needs forward slashes on every platform.
	routineBin := filepath.Join("bin", "routine")
	if runtime.GOOS == "windows" {
		routineBin += ".exe"
	}
	cmd := exec.CommandContext(runCtx, bashPath, "-c", script, "bash", filepath.ToSlash(routineBin))
	cmd.Env = append(os.Environ(), env...)
	cmd.Env = append(cmd.Env, "BOOTSTRAP_TOKEN_FILE="+target.TokenPath, "ROUTINE_MODEL="+target.Model)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	shell.HideWindow(cmd)
	if err := cmd.Start(); err != nil {
		cancel()
		logFile.Close()
		respond.WithInternalServerError(fmt.Errorf("handleWelcomeBootstrap: Failed to start claude: %w", err), w)
		return
	}

	s.bootstrap.pid = cmd.Process.Pid
	s.bootstrap.running = true
	s.bootstrap.exited = false
	s.bootstrap.exitCode = 0
	s.bootstrap.startedAt = time.Now()
	go func() {
		waitErr := cmd.Wait()
		cancel()
		logFile.Close()
		exitCode := 0
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		s.bootstrap.mu.Lock()
		s.bootstrap.exited = true
		s.bootstrap.exitCode = exitCode
		s.bootstrap.running = false
		s.bootstrap.mu.Unlock()
	}()

	view := bootstrapView{
		Detail:  fmt.Sprintf("bootstrap started (pid %d, log %s) — waiting for the session id…", cmd.Process.Pid, logPath),
		Running: true,
	}
	s.renderFragment(w, tmplWelcomeBootstrap, view)
}

func (s *Server) handleWelcomeBootstrapStatus(w http.ResponseWriter, r *http.Request) {
	s.bootstrap.mu.Lock()
	pid := s.bootstrap.pid
	running := s.bootstrap.running
	exited := s.bootstrap.exited
	exitCode := s.bootstrap.exitCode
	startedAt := s.bootstrap.startedAt
	s.bootstrap.mu.Unlock()

	view := bootstrapView{Running: running}
	if pid == 0 {
		view.Detail = "no bootstrap run yet"
		s.renderFragment(w, tmplWelcomeBootstrap, view)
		return
	}

	view.SessionId = s.resolveBootstrapSession(r.Context(), startedAt)
	logPath := filepath.Join(os.TempDir(), "smine-bootstrap.log")
	switch {
	case running && view.SessionId == "":
		view.Detail = fmt.Sprintf("bootstrap running (pid %d) — session not indexed yet…", pid)
	case running:
		view.Detail = fmt.Sprintf("bootstrap running (pid %d) — session", pid)
	case exited && exitCode != 0:
		view.Detail = fmt.Sprintf("bootstrap failed (exit %d) — see the log %s", exitCode, logPath)
	case view.SessionId != "":
		view.Detail = "bootstrap finished — session"
	default:
		view.Detail = fmt.Sprintf("bootstrap finished (pid %d) — see the peek dashboard or the log", pid)
	}
	s.renderFragment(w, tmplWelcomeBootstrap, view)
}

// resolveBootstrapSession degrades to "" instead of erroring when peek is unreachable.
func (s *Server) resolveBootstrapSession(ctx context.Context, startedAt time.Time) string {
	repoRoot, err := os.Getwd()
	if err != nil {
		return ""
	}

	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	index, err := s.peekClient.SessionIndex(probeCtx)
	if err != nil {
		return ""
	}

	session, ok := index.ByCwd[filepath.Clean(repoRoot)]
	if !ok || session.LastActive.Before(startedAt.Add(-time.Minute)) {
		return ""
	}
	return session.Id
}

// bootstrapEnv passes inputs as env, which the wrapper turns into deterministic stage flags.
func bootstrapEnv(since string, extraPrompt string) ([]string, error) {
	env := make([]string, 0)

	// since
	since = strings.TrimSpace(since)
	if since != "" {
		if _, err := time.Parse("2006-01-02", since); err != nil {
			return nil, fmt.Errorf("bootstrapEnv: Invalid field since: %q", since)
		}
		env = append(env, "BOOTSTRAP_SINCE="+since)
	}

	// extra-prompt
	extraPrompt = strings.TrimSpace(extraPrompt)
	if strings.ContainsAny(extraPrompt, "\"'\n") {
		return nil, errors.New("bootstrapEnv: Invalid field extra-prompt: Quotes and newlines are not allowed")
	}
	if extraPrompt != "" {
		env = append(env, "BOOTSTRAP_EXTRA_PROMPT="+extraPrompt)
	}
	return env, nil
}

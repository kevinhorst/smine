package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	pid       int
	running   bool
	startedAt time.Time
}

type bootstrapView struct {
	Detail    string
	Running   bool
	SessionId string
}

// handleWelcomeBootstrap spawns the one-shot bootstrap wrapper
// (cmd/bootstrap/run.sh) detached (the startPeek precedent: a deliberately
// multi-hour child outside shell.Run's deadline — the run is watched via
// peek, not this process). The wrapper resolves claude through the login
// shell exactly like the token verification: the server process PATH lacks
// the claude shim (observed on the Windows logon task), the login-shell
// PATH has it; auth comes from the routine token the wrapper reads from
// BOOTSTRAP_TOKEN_FILE.
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

	// Token and model resolve like the profile style test: the routine's
	// labeled token and ROUTINE_MODEL when configured, else the defaults.
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

	logPath := filepath.Join(os.TempDir(), "smine-bootstrap.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		respond.WithInternalServerError(err, w)
		return
	}

	// Multi-hour child under its own backstop deadline (the routinewrap
	// pattern): bounded at 6h, far above any real bootstrap run.
	script := verifyPathPrefix + `exec bash "$1"`
	runCtx, cancel := context.WithTimeout(context.Background(), bootstrapDeadline)
	cmd := exec.CommandContext(runCtx, bashPath, "-c", script, "bash", filepath.Join("cmd", "bootstrap", "run.sh"))
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
	s.bootstrap.startedAt = time.Now()
	go func() {
		cmd.Wait()
		cancel()
		logFile.Close()
		s.bootstrap.mu.Lock()
		s.bootstrap.running = false
		s.bootstrap.mu.Unlock()
	}()

	view := bootstrapView{
		Detail:  fmt.Sprintf("bootstrap started (pid %d, log %s) — waiting for the session id…", cmd.Process.Pid, logPath),
		Running: true,
	}
	s.renderFragment(w, tmplWelcomeBootstrap, view)
}

// handleWelcomeBootstrapStatus reports the run and resolves its peek session
// id by matching the repo-root cwd against sessions newer than the start.
func (s *Server) handleWelcomeBootstrapStatus(w http.ResponseWriter, r *http.Request) {
	s.bootstrap.mu.Lock()
	pid := s.bootstrap.pid
	running := s.bootstrap.running
	startedAt := s.bootstrap.startedAt
	s.bootstrap.mu.Unlock()

	view := bootstrapView{Running: running}
	if pid == 0 {
		view.Detail = "no bootstrap run yet"
		s.renderFragment(w, tmplWelcomeBootstrap, view)
		return
	}

	view.SessionId = s.resolveBootstrapSession(r.Context(), startedAt)
	switch {
	case running && view.SessionId == "":
		view.Detail = fmt.Sprintf("bootstrap running (pid %d) — session not indexed yet…", pid)
	case running:
		view.Detail = fmt.Sprintf("bootstrap running (pid %d) — session", pid)
	case view.SessionId != "":
		view.Detail = "bootstrap finished — session"
	default:
		view.Detail = fmt.Sprintf("bootstrap finished (pid %d) — see the peek dashboard or the log", pid)
	}
	s.renderFragment(w, tmplWelcomeBootstrap, view)
}

// resolveBootstrapSession finds the newest peek session running in the repo
// root that started around or after the bootstrap spawn; "" when none is
// indexed yet or peek is unreachable (the status line degrades, never errors).
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

// bootstrapEnv turns the form inputs into the wrapper's environment — the
// wrapper translates them into deterministic stage flags (env→flag, the
// nightly pattern; relayed prose args are retired).
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

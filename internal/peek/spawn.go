package peek

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/shell"
)

// Field names are the wire contract of peek-mcp's /healthz payload.
type Identity struct {
	ClaudeHome  string `json:"claudeHome"`
	CodexHome   string `json:"codexHome"`
	ControlPort int    `json:"controlPort"`
	Version     string `json:"version"`
}

// cmd/configserver and the MCP update handler share one implementation.
type SpawnOptions struct {
	BackLink    string
	Bin         string
	ClaudeHome  string
	CodexHome   string
	ControlPort int
	Port        int
}

func (o SpawnOptions) Addr() string {
	return fmt.Sprintf("127.0.0.1:%d", o.Port)
}

// releaseAssetName maps the running platform to its peek-mcp release asset.
func releaseAssetName() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return "peek-mcp", nil // universal binary, signed
	case "linux":
		return "peek-mcp-linux-" + runtime.GOARCH, nil
	case "windows":
		return "peek-mcp-windows-" + runtime.GOARCH + ".exe", nil
	}
	return "", fmt.Errorf("releaseAssetName: Unsupported platform %s", runtime.GOOS)
}

// IdentityAt reads peek-mcp's /healthz identity at addr (host:port).
func IdentityAt(addr string) (*Identity, error) {
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("healthz status %d", resp.StatusCode)
	}
	var id Identity
	if err := json.NewDecoder(resp.Body).Decode(&id); err != nil {
		return nil, err
	}
	return &id, nil
}

// The `version` subcommand is peek-mcp's only version interface (no --version flag).
func InstalledVersion(ctx context.Context, bin string) (string, error) {
	output, err := shell.Run(ctx, "", bin, "version")
	if err != nil {
		return "", fmt.Errorf("InstalledVersion: %s: %w", bin, err)
	}
	return strings.TrimSpace(output), nil
}

// lsof without sudo lists only this user's processes, so another profile's peek is never killed.
func Restart(ctx context.Context, opts SpawnOptions) error {
	for _, port := range []int{opts.Port, opts.ControlPort} {
		if port == 0 {
			continue
		}
		output, err := shell.Run(ctx, "", "lsof", fmt.Sprintf("-tiTCP:%d", port), "-sTCP:LISTEN")
		if err != nil {
			continue // no listener on this port
		}
		for _, pid := range strings.Fields(output) {
			if _, err := shell.Run(ctx, "", "kill", pid); err != nil {
				return fmt.Errorf("Restart: Failed to kill pid %s on port %d: %w", pid, port, err)
			}
		}
	}

	if !Start(ctx, opts) {
		return errors.New("Restart: Peek did not come up after respawn")
	}
	return nil
}

// Holder is who listens on a profile's peek port.
type Holder int

const (
	HolderNone    Holder = iota // nothing listens
	HolderOwn                   // peek-mcp serving this profile's homes
	HolderForeign               // another profile's peek, or a process that is not an identifiable peek
)

// Another macOS profile's peek is reachable on the shared loopback; serving it would show that user's sessions.
func Probe(opts SpawnOptions) Holder {
	addr := opts.Addr()
	id, err := IdentityAt(addr)
	if err == nil {
		if id.ClaudeHome == opts.ClaudeHome && id.CodexHome == opts.CodexHome {
			log.Printf("peek-mcp %s serving on %s for this profile", id.Version, addr)
			return HolderOwn
		}
		log.Printf("ERROR: peek-mcp on %s serves claude-home=%s codex-home=%s, want %s / %s — another profile's peek holds this port; set peek.port and peek.control_port (smine-rules settings set) and reinstall (session column disabled)", addr, id.ClaudeHome, id.CodexHome, opts.ClaudeHome, opts.CodexHome)
		return HolderForeign
	}
	conn, dialErr := net.DialTimeout("tcp", addr, time.Second)
	if dialErr != nil {
		return HolderNone
	}
	conn.Close()
	log.Printf("ERROR: %s is in use but not an identifiable peek-mcp (%v) — stale pre-1.2.2 peek or foreign process; reinstall to replace it (session column disabled)", addr, err)
	return HolderForeign
}

// A foreign holder disables the peek integration for this run.
func Start(ctx context.Context, opts SpawnOptions) bool {
	switch Probe(opts) {
	case HolderOwn:
		return true
	case HolderForeign:
		return false
	case HolderNone:
	}

	addr := opts.Addr()
	args := []string{"start", "--transport", "http", "--port", strconv.Itoa(opts.Port),
		"--claude-home", opts.ClaudeHome, "--codex-home", opts.CodexHome}
	if opts.ControlPort != 0 {
		args = append(args, "--control-port", strconv.Itoa(opts.ControlPort))
		args = append(args, "--back-link", opts.BackLink)
	}
	cmd := exec.CommandContext(ctx, opts.Bin, args...)
	cmd.Stderr = os.Stderr
	shell.HideWindow(cmd)
	if err := cmd.Start(); err != nil {
		log.Printf("peek-mcp spawn failed (session column degraded): %v", err)
		return false
	}

	log.Printf("peek-mcp spawned (pid %d) on http://%s/mcp", cmd.Process.Pid, addr)
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("peek-mcp exited: %v", err)
		}
	}()
	return true
}

// A running process keeps its old inode, so the rename never corrupts a live peek.
func UpdateBinary(ctx context.Context, dest, version string) error {
	asset, err := releaseAssetName()
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://github.com/kevinhorst/peek-mcp/releases/download/v%s/%s", version, asset)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("UpdateBinary: %s: %w", url, err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("UpdateBinary: %s: %w", url, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("UpdateBinary: %s: Status %d", url, response.StatusCode)
	}

	tmp := dest + ".download"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("UpdateBinary: %s: %w", tmp, err)
	}
	if _, err := io.Copy(file, response.Body); err != nil {
		file.Close()
		os.Remove(tmp)
		return fmt.Errorf("UpdateBinary: %s: %w", tmp, err)
	}
	if err := file.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("UpdateBinary: %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("UpdateBinary: %s: %w", dest, err)
	}
	return nil
}

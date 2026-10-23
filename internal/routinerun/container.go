package routinerun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	settings "github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/shell"
)

const (
	cellImage          = "smine-routine-cell"
	cellNamePrefix     = "smine-cell-"
	containerEntry     = "container"
	containerPidsLimit = 4096
	containerSlots     = 4
	dockerfilePath     = "cmd/routine/container/Dockerfile"
	gatewayHost        = "host.docker.internal"
	holderSuffix       = "-firewall"
	imageBuildLogName  = "image-build.log"
	imageBuildTimeout  = 45 * time.Minute
	peekVersionFile    = "PEEK_MCP_VERSION"
	registrationsDir   = "container-sessions"
	stopTimeout        = 2 * time.Minute
)

var errSlotTimeout = errors.New("container slot timeout")

// The OAuth token travels in the CLAUDE_CODE_ family.
var passThroughPrefixes = []string{"CLAUDE_CODE_", "ROUTINE_", "SMINE_"}

// bindMount mounts a host path at the identical path inside the cell.
type bindMount struct {
	isReadOnly bool
	path       string
}

func (b *bindMount) arg() string {
	spec := "type=bind,source=" + b.path + ",target=" + b.path
	if b.isReadOnly {
		spec += ",readonly"
	}
	return spec
}

// A group runs one cell at a time, so container names are fixed per group.
type containerCell struct {
	backlogDir string
	config     *Config
	home       string
	ports      []string
	repoMounts []*bindMount
	repoRoot   string
	routine    string
	slot       *os.File
	worktree   string
}

func newContainerCell(r *Runner, worktree string) *containerCell {
	claudeHome := filepath.Dir(settings.DefaultPath())
	return &containerCell{
		backlogDir: filepath.Join(r.storeRoot, settings.StoreBacklogDir),
		config:     r.config,
		home:       filepath.Dir(claudeHome),
		repoMounts: r.repoMounts,
		repoRoot:   r.repoRoot,
		routine:    r.name,
		worktree:   worktree,
	}
}

// The kernel frees a slot when its runner dies.
func (c *containerCell) acquireSlot(ctx context.Context, wait time.Duration) (bool, error) {
	deadline := time.Now().Add(wait)
	for {
		slot, err := c.trySlots()
		if err != nil {
			return false, err
		}
		if slot != nil {
			c.slot = slot
			return true, nil
		}
		if !time.Now().Before(deadline) {
			return false, nil
		}

		select {
		case <-ctx.Done():
			return false, fmt.Errorf("containerCell.acquireSlot: %w", ctx.Err())
		case <-time.After(lockPoll):
		}
	}
}

// A missing read-only source is skipped: the gate and the hooks treat each as optional.
func (c *containerCell) binds() ([]*bindMount, error) {
	claudeHome := filepath.Join(c.home, ".claude")
	writable := []string{
		filepath.Join(c.repoRoot, ".git"),
		filepath.Join(c.repoRoot, ".claude", "worktrees"),
		c.worktree,
		filepath.Join(claudeHome, "projects"),
		filepath.Join(claudeHome, "plans"),
		filepath.Join(claudeHome, "context-required"),
		filepath.Join(claudeHome, "read-gate-state"),
		filepath.Join(claudeHome, "tool-policy"),
		filepath.Join(claudeHome, "acdsl"),
		filepath.Join(c.home, ".cache", "claude-routine", "artifacts"),
		c.config.SteerDir,
		filepath.Join(c.home, ".peek", "state"),
		c.backlogDir,
	}

	readOnly := []string{
		filepath.Join(claudeHome, "settings.json"),
		filepath.Join(claudeHome, "skills"),
		filepath.Join(claudeHome, "agents"),
		filepath.Join(claudeHome, "context"),
		filepath.Join(claudeHome, "AGENTS.md"),
		filepath.Join(claudeHome, "settings.local.json"),
		filepath.Join(c.config.StateDir, settings.StateRegistryFile),
		filepath.Join(claudeHome, "heartbeat", "config.json"),
	}
	readOnly = append(readOnly, hookFiles(filepath.Join(claudeHome, "hooks"))...)

	var mounts []*bindMount
	for _, path := range writable {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return nil, fmt.Errorf("containerCell.binds: %w", err)
		}

		mount := &bindMount{path: path}
		mounts = append(mounts, mount)
	}
	for _, path := range readOnly {
		if _, err := os.Stat(path); err != nil {
			continue
		}

		mount := &bindMount{isReadOnly: true, path: path}
		mounts = append(mounts, mount)
	}
	return append(mounts, c.repoMounts...), nil
}

func (c *containerCell) cellName() string {
	return cellNamePrefix + c.config.Group
}

// The relays bind before the fence's execs run, so one connect per port proves them; a dead relay fails the cell.
func (c *containerCell) checkRelays(ctx context.Context) error {
	for _, port := range c.ports {
		probe := "exec 3<>/dev/tcp/127.0.0.1/" + port
		if output, err := shell.Run(ctx, "", "docker", "exec", c.holderName(), "bash", "-c", probe); err != nil {
			return fmt.Errorf("containerCell.checkRelays: Port %s: %s: %w", port, firstLine(output), err)
		}
	}
	return nil
}

// command is the docker run prefix RunChat execs in place of claude.
func (c *containerCell) command(ctx context.Context) ([]string, error) {
	mounts, err := c.binds()
	if err != nil {
		return nil, err
	}

	args := []string{
		"docker", "run", "--rm", "--interactive", "--init",
		"--name", c.cellName(),
		"--label", routines.ContainerLabel(c.routine),
		"--network", "container:" + c.holderName(),
		"--user", strconv.Itoa(os.Getuid()) + ":" + strconv.Itoa(os.Getgid()),
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--security-opt", "seccomp=builtin",
		"--cpus", c.config.ContainerCpus,
		"--memory", c.config.ContainerMemory,
		"--pids-limit", strconv.Itoa(containerPidsLimit),
		"--workdir", c.worktree,
	}
	args = append(args, c.envArgs(ctx)...)
	for _, mount := range mounts {
		args = append(args, "--mount", mount.arg())
	}
	for _, volume := range cacheVolumes(c.home) {
		args = append(args, "--mount", volume)
	}
	return append(args, cellImage, "claude"), nil
}

// Pass-through by name keeps the token off the command line; the identity goes by value: no gitconfig inside.
func (c *containerCell) envArgs(ctx context.Context) []string {
	args := []string{"--env", "HOME=" + c.home, "--env", headlessEnv}
	for _, name := range passThroughEnv(os.Environ()) {
		args = append(args, "--env", name)
	}

	name := gitConfigValue(ctx, c.repoRoot, "user.name")
	email := gitConfigValue(ctx, c.repoRoot, "user.email")
	identity := []string{
		"GIT_AUTHOR_NAME=" + name,
		"GIT_AUTHOR_EMAIL=" + email,
		"GIT_COMMITTER_NAME=" + name,
		"GIT_COMMITTER_EMAIL=" + email,
	}
	for _, entry := range identity {
		args = append(args, "--env", entry)
	}
	return args
}

// The cell joins the holder's namespace without the capability to change these rules.
func (c *containerCell) fence(ctx context.Context) error {
	output, err := shell.Run(ctx, "", "docker", "exec", c.holderName(), "getent", "ahosts", gatewayHost)
	if err != nil {
		return fmt.Errorf("containerCell.fence: %w", err)
	}

	addrs := gatewayAddrs(output)
	if len(addrs) == 0 {
		return errors.New("containerCell.fence: No address for the host gateway")
	}

	for _, rule := range fenceRules(addrs, c.ports) {
		args := append([]string{"exec", c.holderName()}, rule...)
		if _, err := shell.Run(ctx, "", "docker", args...); err != nil {
			return fmt.Errorf("containerCell.fence: %w", err)
		}
	}
	return nil
}

func (c *containerCell) holderName() string {
	return c.cellName() + holderSuffix
}

func (c *containerCell) registration() *sessionRegistration {
	return &sessionRegistration{
		cwd:         c.worktree,
		dir:         filepath.Join(c.config.StateDir, registrationsDir),
		projectsDir: filepath.Join(c.home, ".claude", "projects"),
	}
}

func (c *containerCell) start(ctx context.Context) ([]string, error) {
	env, err := install.NewEnv(c.repoRoot, io.Discard)
	if err != nil {
		return nil, fmt.Errorf("containerCell.start: %w", err)
	}
	c.ports = []string{env.Values.HeartbeatPort, env.Values.PeekControlPort}
	for _, port := range c.ports {
		if _, err := strconv.Atoi(port); err != nil {
			return nil, fmt.Errorf("containerCell.start: Invalid port %q: %w", port, err)
		}
	}

	isHeld, err := c.acquireSlot(ctx, groupLockWait)
	if err != nil {
		return nil, err
	}
	if !isHeld {
		return nil, errSlotTimeout
	}

	c.sweep(ctx)
	if err := c.startHolder(ctx); err != nil {
		return nil, err
	}
	if err := c.fence(ctx); err != nil {
		return nil, err
	}
	if err := c.checkRelays(ctx); err != nil {
		return nil, err
	}
	return c.command(ctx)
}

// The holder owns the cell's network namespace, alone holds the capabilities the fence needs, and relays the hook ports from loopback.
func (c *containerCell) startHolder(ctx context.Context) error {
	args := []string{
		"run", "--detach", "--rm",
		"--name", c.holderName(),
		"--label", routines.ContainerLabel(c.routine),
		"--user", "0",
		"--cap-drop", "ALL",
		"--cap-add", "NET_ADMIN",
		"--cap-add", "NET_RAW",
		"--add-host", gatewayHost + ":host-gateway",
		cellImage, "sh", "-c", relayScript(c.ports),
	}
	if output, err := shell.Run(ctx, "", "docker", args...); err != nil {
		return fmt.Errorf("containerCell.startHolder: %s: %w", firstLine(output), err)
	}
	return nil
}

// stop takes its own deadline: it runs on cancelled runs too.
func (c *containerCell) stop() {
	ctx, cancel := context.WithTimeout(context.Background(), stopTimeout)
	defer cancel()

	c.sweep(ctx)
	if c.slot != nil {
		c.slot.Close()
		c.slot = nil
	}
}

// Before a start, sweep clears a killed runner's leftovers; an absent container is not an error.
func (c *containerCell) sweep(ctx context.Context) {
	if output, err := shell.Run(ctx, "", "docker", "rm", "--force", c.cellName(), c.holderName()); err != nil {
		log.Printf("containerCell.sweep: %s", firstLine(output))
	}
	c.registration().clear()
}

func (c *containerCell) trySlots() (*os.File, error) {
	for index := range containerSlots {
		path := filepath.Join(c.config.ContainerRoot, "slots", strconv.Itoa(index)+".lock")
		slot, isHeld, err := acquireLock(path, 0)
		if err != nil {
			return nil, fmt.Errorf("containerCell.trySlots: %w", err)
		}
		if isHeld {
			return slot, nil
		}
	}
	return nil, nil
}

// Field names are the ones heartbeat's registry edge and agentwt read.
type containerSession struct {
	Cwd        string `json:"cwd"`
	Entrypoint string `json:"entrypoint"`
	Pid        int    `json:"pid"`
	SessionId  string `json:"sessionId"`
}

// The harness registry stays inside the container, where its pid means nothing to the host.
type sessionRegistration struct {
	cwd         string
	dir         string
	projectsDir string
}

// clear removes the registrations of this cell's worktree.
func (s *sessionRegistration) clear() {
	paths, err := filepath.Glob(filepath.Join(s.dir, "*.json"))
	if err != nil {
		return
	}

	for _, path := range paths {
		if s.owns(path) {
			_ = os.Remove(path)
		}
	}
}

func (s *sessionRegistration) owns(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	var session containerSession
	if err := json.Unmarshal(data, &session); err != nil {
		return false
	}
	return session.Cwd == s.cwd
}

// pid is the docker client's; heartbeat forwards a registration only once its transcript exists.
func (s *sessionRegistration) write(pid int, sessionId string) (bool, error) {
	transcripts, err := filepath.Glob(filepath.Join(s.projectsDir, "*", sessionId+".jsonl"))
	if err != nil {
		return false, fmt.Errorf("sessionRegistration.write: %w", err)
	}
	if len(transcripts) == 0 {
		return false, nil
	}

	session := &containerSession{
		Cwd:        s.cwd,
		Entrypoint: containerEntry,
		Pid:        pid,
		SessionId:  sessionId,
	}
	data, err := json.Marshal(session)
	if err != nil {
		return false, fmt.Errorf("sessionRegistration.write: %w", err)
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return false, fmt.Errorf("sessionRegistration.write: %w", err)
	}

	path := filepath.Join(s.dir, strconv.Itoa(pid)+".json")
	if err := fsx.WriteFileAtomic(data, path, 0o644); err != nil {
		return false, fmt.Errorf("sessionRegistration.write: %w", err)
	}
	return true, nil
}

// Docker's layer cache is the freshness check; the daemon is probed, never started.
func buildImage(ctx context.Context, logPath, repoRoot string) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("buildImage: docker CLI not found on PATH %s: %w", os.Getenv("PATH"), err)
	}
	if output, err := shell.Run(ctx, "", "docker", "version", "--format", "{{.Server.Version}}"); err != nil {
		return fmt.Errorf("buildImage: Docker daemon unreachable: %s: %w", firstLine(output), err)
	}

	version, err := install.ClaudeVersion(ctx)
	if err != nil {
		return fmt.Errorf("buildImage: %w", err)
	}

	peekVersion, err := os.ReadFile(filepath.Join(repoRoot, peekVersionFile))
	if err != nil {
		return fmt.Errorf("buildImage: %w", err)
	}

	claudeHome := filepath.Dir(settings.DefaultPath())
	hooks := hookNames(filepath.Join(claudeHome, "hooks", "bin"), repoRoot)
	buildArgs := []string{
		"CLAUDE_VERSION=" + version,
		"HOOKS=" + strings.Join(hooks, " "),
		"HOST_GID=" + strconv.Itoa(os.Getgid()),
		"HOST_HOME=" + filepath.Dir(claudeHome),
		"HOST_UID=" + strconv.Itoa(os.Getuid()),
		"PEEK_MCP_VERSION=" + strings.TrimSpace(string(peekVersion)),
	}
	args := []string{
		"build",
		"--file", filepath.Join(repoRoot, dockerfilePath),
		"--tag", cellImage,
	}
	for _, buildArg := range buildArgs {
		args = append(args, "--build-arg", buildArg)
	}
	args = append(args, repoRoot)

	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("buildImage: %w", err)
	}
	defer logFile.Close()

	buildCtx, cancel := context.WithTimeout(ctx, imageBuildTimeout)
	defer cancel()

	cmd := exec.CommandContext(buildCtx, "docker", args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.WaitDelay = time.Second
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("buildImage: docker build failed, see %s: %w", logPath, err)
	}
	return nil
}

// The host's darwin build caches are unusable inside the Linux cell.
func cacheVolumes(home string) []string {
	return []string{
		"type=volume,source=smine-cell-gomod,target=" + filepath.Join(home, "go", "pkg", "mod"),
		"type=volume,source=smine-cell-gocache,target=" + filepath.Join(home, ".cache", "go-build"),
		"type=volume,source=smine-cell-cargo,target=/usr/local/cargo/registry",
	}
}

func fenceRules(addrs, ports []string) [][]string {
	var rules [][]string
	for _, addr := range addrs {
		binary := "iptables"
		if strings.Contains(addr, ":") {
			binary = "ip6tables"
		}

		for _, port := range ports {
			accept := []string{binary, "-A", "OUTPUT", "-d", addr, "-p", "tcp", "--dport", port, "-j", "ACCEPT"}
			rules = append(rules, accept)
		}
		reject := []string{binary, "-A", "OUTPUT", "-d", addr, "-j", "REJECT"}
		rules = append(rules, reject)
	}
	return rules
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	return line
}

func gatewayAddrs(output string) []string {
	var addrs []string
	for line := range strings.SplitSeq(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || slices.Contains(addrs, fields[0]) {
			continue
		}
		addrs = append(addrs, fields[0])
	}
	return addrs
}

func gitConfigValue(ctx context.Context, dir, key string) string {
	output, err := git(ctx, dir, "config", "--get", key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(output)
}

// The hooks bin dir comes from the image.
func hookFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var files []string
	for _, entry := range entries {
		if entry.Type().IsRegular() {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	return files
}

// The image builds for Linux only the deployed hooks that still have a cmd dir.
func hookNames(hooksBin, repoRoot string) []string {
	entries, err := os.ReadDir(hooksBin)
	if err != nil {
		return nil
	}

	var names []string
	for _, entry := range entries {
		info, err := os.Stat(filepath.Join(repoRoot, "cmd", entry.Name()))
		isCommand := err == nil && info.IsDir()
		if isCommand {
			names = append(names, entry.Name())
		}
	}
	return names
}

func isPassThroughName(name string) bool {
	if name == "DISABLE_AUTOUPDATER" {
		return true
	}
	return slices.ContainsFunc(passThroughPrefixes, func(prefix string) bool {
		return strings.HasPrefix(name, prefix)
	})
}

func passThroughEnv(environ []string) []string {
	var names []string
	for _, entry := range environ {
		name, _, _ := strings.Cut(entry, "=")
		if isPassThroughName(name) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

// Claude Code refuses HTTP hooks to private addresses, so the settings keep 127.0.0.1 and the holder forwards it to the host.
func relayScript(ports []string) string {
	var script strings.Builder
	for _, port := range ports {
		fmt.Fprintf(&script, "socat TCP-LISTEN:%s,bind=127.0.0.1,reuseaddr,fork TCP:%s:%s & ", port, gatewayHost, port)
	}
	script.WriteString("exec sleep infinity")
	return script.String()
}

// The run's own checkout is never a repo mount: its .git and worktree pool are mounted already.
func resolveRepoMounts(mounts []*routines.ContainerMount, registryPath, repoRoot string) ([]*bindMount, error) {
	if len(mounts) == 0 {
		return nil, nil
	}

	registry := repos.NewRegistry(registryPath)
	if err := registry.Reload(); err != nil {
		return nil, fmt.Errorf("resolveRepoMounts: %w", err)
	}

	binds := make([]*bindMount, 0, len(mounts))
	for _, mount := range mounts {
		repo, isRegistered := registry.Find(mount.Repo)
		if !isRegistered {
			return nil, fmt.Errorf("resolveRepoMounts: Repo %s is not registered", mount.Repo)
		}
		if repo.Path == repoRoot {
			return nil, fmt.Errorf("resolveRepoMounts: Repo %s is the run's own repo", mount.Repo)
		}
		if _, err := os.Stat(repo.Path); err != nil {
			return nil, fmt.Errorf("resolveRepoMounts: Repo %s: %w", mount.Repo, err)
		}

		bind := &bindMount{isReadOnly: mount.Access == routines.MountReadOnly, path: repo.Path}
		binds = append(binds, bind)
	}
	return binds, nil
}

package routinerun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	settings "github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/routines"
)

const (
	cellTestDockerSocket = "/var/run/docker.sock"
	cellTestGroup        = "group"
	cellTestManifest     = `{"kind":"single","prompt":"/spec-drift"}`
	cellTestPid          = 4242
	cellTestRoutine      = "boxed"
	cellTestSessionId    = "session-1"
	cellTestSettings     = `{"env":{"HEARTBEAT_ADDR":"127.0.0.1:6101"}}`
	cellTestWorktree     = "/worktrees/group"
)

func TestBindMount_arg(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		mount *bindMount
	}

	tests := make([]*testCase, 0)

	// read-write-same-path
	tests = append(tests, &testCase{
		_id:       "read-write-same-path",
		_expected: "type=bind,source=/home/a/.claude/projects,target=/home/a/.claude/projects",

		mount: &bindMount{path: "/home/a/.claude/projects"},
	})

	// read-only
	tests = append(tests, &testCase{
		_id:       "read-only",
		_expected: "type=bind,source=/home/a/.claude/skills,target=/home/a/.claude/skills,readonly",

		mount: &bindMount{isReadOnly: true, path: "/home/a/.claude/skills"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.mount.arg())
		})
	}
}

func TestBuildImage(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedError string
		_expectedLog   string
		_shouldPass    bool

		buildLog   string
		dockerFail string
		dockerLog  string
		home       string
		path       string
		repoRoot   string
	}

	installFakeClaude(t)
	claudePath, err := exec.LookPath("claude")
	require.NoError(t, err)
	installFakeDocker(t)
	dockerPath, err := exec.LookPath("docker")
	require.NoError(t, err)
	dockerOnly := filepath.Dir(dockerPath)
	withClaude := dockerOnly + string(os.PathListSeparator) + filepath.Dir(claudePath)
	versionLine := "version --format {{.Server.Version}}\n"

	tests := make([]*testCase, 0)

	// docker-cli-missing
	tests = append(tests, &testCase{
		_id:            "docker-cli-missing",
		_expectedError: "docker CLI not found on PATH",
		_expectedLog:   "",
		_shouldPass:    false,

		buildLog:   filepath.Join(t.TempDir(), imageBuildLogName),
		dockerFail: "",
		dockerLog:  filepath.Join(t.TempDir(), "docker.log"),
		home:       cellTestBuildHome(t, "toolpolicy"),
		path:       filepath.Dir(claudePath),
		repoRoot:   cellTestBuildRepo(t, "toolpolicy"),
	})

	// daemon-down
	tests = append(tests, &testCase{
		_id:            "daemon-down",
		_expectedError: "Docker daemon unreachable",
		_expectedLog:   versionLine,
		_shouldPass:    false,

		buildLog:   filepath.Join(t.TempDir(), imageBuildLogName),
		dockerFail: "version",
		dockerLog:  filepath.Join(t.TempDir(), "docker.log"),
		home:       cellTestBuildHome(t, "toolpolicy"),
		path:       withClaude,
		repoRoot:   cellTestBuildRepo(t, "toolpolicy"),
	})

	// claude-version-unreadable
	tests = append(tests, &testCase{
		_id:            "claude-version-unreadable",
		_expectedError: "ClaudeVersion",
		_expectedLog:   versionLine,
		_shouldPass:    false,

		buildLog:   filepath.Join(t.TempDir(), imageBuildLogName),
		dockerFail: "",
		dockerLog:  filepath.Join(t.TempDir(), "docker.log"),
		home:       cellTestBuildHome(t, "toolpolicy"),
		path:       dockerOnly,
		repoRoot:   cellTestBuildRepo(t, "toolpolicy"),
	})

	// build-args
	home := cellTestBuildHome(t, "readgate", "toolpolicy")
	repoRoot := cellTestBuildRepo(t, "readgate", "toolpolicy")
	tests = append(tests, &testCase{
		_id:            "build-args",
		_expectedError: "",
		_expectedLog:   versionLine + cellTestBuildLine(home, repoRoot, "readgate toolpolicy"),
		_shouldPass:    true,

		buildLog:   filepath.Join(t.TempDir(), imageBuildLogName),
		dockerFail: "",
		dockerLog:  filepath.Join(t.TempDir(), "docker.log"),
		home:       home,
		path:       withClaude,
		repoRoot:   repoRoot,
	})

	// build-fails
	home = cellTestBuildHome(t, "toolpolicy")
	repoRoot = cellTestBuildRepo(t, "toolpolicy")
	buildLog := filepath.Join(t.TempDir(), imageBuildLogName)
	tests = append(tests, &testCase{
		_id:            "build-fails",
		_expectedError: buildLog,
		_expectedLog:   versionLine + cellTestBuildLine(home, repoRoot, "toolpolicy"),
		_shouldPass:    false,

		buildLog:   buildLog,
		dockerFail: "build",
		dockerLog:  filepath.Join(t.TempDir(), "docker.log"),
		home:       home,
		path:       withClaude,
		repoRoot:   repoRoot,
	})

	// hook-without-cmd-dir-dropped
	home = cellTestBuildHome(t, "ghost", "toolpolicy")
	repoRoot = cellTestBuildRepo(t, "toolpolicy")
	tests = append(tests, &testCase{
		_id:            "hook-without-cmd-dir-dropped",
		_expectedError: "",
		_expectedLog:   versionLine + cellTestBuildLine(home, repoRoot, "toolpolicy"),
		_shouldPass:    true,

		buildLog:   filepath.Join(t.TempDir(), imageBuildLogName),
		dockerFail: "",
		dockerLog:  filepath.Join(t.TempDir(), "docker.log"),
		home:       home,
		path:       withClaude,
		repoRoot:   repoRoot,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			t.Setenv("HOME", test.home)
			t.Setenv("PATH", test.path)
			t.Setenv(fakeDockerFailEnv, test.dockerFail)
			t.Setenv(fakeDockerLogEnv, test.dockerLog)
			err := buildImage(context.Background(), test.buildLog, test.repoRoot)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Contains(t, fmt.Sprint(err), test._expectedError)
			assert.Equal(t, test._expectedLog, readDockerLog(t, test.dockerLog))
		})
	}
}

func TestContainerCell_acquireSlot(t *testing.T) {
	type testCase struct {
		_id           string
		_expectedSlot string
		_isHeld       bool
		_shouldPass   bool

		cell *containerCell
		ctx  context.Context
		wait time.Duration
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := make([]*testCase, 0)

	// first-slot-free
	tests = append(tests, &testCase{
		_id:           "first-slot-free",
		_expectedSlot: "0.lock",
		_isHeld:       true,
		_shouldPass:   true,

		cell: cellTestCell(t, t.TempDir()),
		ctx:  context.Background(),
		wait: 0,
	})

	// first-taken-second-free
	cell := cellTestCell(t, t.TempDir())
	cellTestHoldSlot(t, cell, 0)
	tests = append(tests, &testCase{
		_id:           "first-taken-second-free",
		_expectedSlot: "1.lock",
		_isHeld:       true,
		_shouldPass:   true,

		cell: cell,
		ctx:  context.Background(),
		wait: 0,
	})

	// all-taken-wait-zero
	cell = cellTestCell(t, t.TempDir())
	cellTestHoldAllSlots(t, cell)
	tests = append(tests, &testCase{
		_id:           "all-taken-wait-zero",
		_expectedSlot: "",
		_isHeld:       false,
		_shouldPass:   true,

		cell: cell,
		ctx:  context.Background(),
		wait: 0,
	})

	// cancelled-context
	cell = cellTestCell(t, t.TempDir())
	cellTestHoldAllSlots(t, cell)
	tests = append(tests, &testCase{
		_id:           "cancelled-context",
		_expectedSlot: "",
		_isHeld:       false,
		_shouldPass:   false,

		cell: cell,
		ctx:  cancelled,
		wait: time.Hour,
	})

	// released-slot-reusable
	cell = cellTestCell(t, t.TempDir())
	cellTestHoldSlot(t, cell, 0).Close()
	tests = append(tests, &testCase{
		_id:           "released-slot-reusable",
		_expectedSlot: "0.lock",
		_isHeld:       true,
		_shouldPass:   true,

		cell: cell,
		ctx:  context.Background(),
		wait: 0,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			isHeld, err := test.cell.acquireSlot(test.ctx, test.wait)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._isHeld, isHeld)
			assert.Equal(t, test._expectedSlot, cellTestSlotName(test.cell))
		})
	}
}

func TestContainerCell_binds(t *testing.T) {
	type testCase struct {
		_id          string
		_expected    []string
		_expectedDir string

		cell *containerCell
	}

	tests := make([]*testCase, 0)

	// all-sources-present
	cell := cellTestCell(t, t.TempDir())
	cellTestReadOnlySources(t, cell)
	tests = append(tests, &testCase{
		_id:          "all-sources-present",
		_expected:    cellTestBindArgs(cell, nil, cellTestReadOnlyPaths(cell)),
		_expectedDir: filepath.Join(cell.home, ".peek", "state"),

		cell: cell,
	})

	// missing-read-only-skipped
	cell = cellTestCell(t, t.TempDir())
	tests = append(tests, &testCase{
		_id:          "missing-read-only-skipped",
		_expected:    cellTestBindArgs(cell, nil, nil),
		_expectedDir: filepath.Join(cell.home, ".peek", "state"),

		cell: cell,
	})

	// missing-writable-created
	cell = cellTestCell(t, t.TempDir())
	tests = append(tests, &testCase{
		_id:          "missing-writable-created",
		_expected:    cellTestBindArgs(cell, nil, nil),
		_expectedDir: filepath.Join(cell.repoRoot, ".claude", "worktrees"),

		cell: cell,
	})

	// backlog-store-writable
	cell = cellTestCell(t, t.TempDir())
	tests = append(tests, &testCase{
		_id:          "backlog-store-writable",
		_expected:    cellTestBindArgs(cell, nil, nil),
		_expectedDir: cell.backlogDir,

		cell: cell,
	})

	// repo-mounts-last
	cell = cellTestCell(t, t.TempDir())
	cellTestReadOnlySources(t, cell)
	plansRepo, peekRepo := t.TempDir(), t.TempDir()
	cell.repoMounts = []*bindMount{{path: plansRepo}, {isReadOnly: true, path: peekRepo}}
	expected := cellTestBindArgs(cell, nil, cellTestReadOnlyPaths(cell))
	expected = append(expected, "type=bind,source="+plansRepo+",target="+plansRepo, "type=bind,source="+peekRepo+",target="+peekRepo+",readonly")
	tests = append(tests, &testCase{
		_id:          "repo-mounts-last",
		_expected:    expected,
		_expectedDir: filepath.Join(cell.home, ".peek", "state"),

		cell: cell,
	})

	// hooks-dir-top-level-files-only
	cell = cellTestCell(t, t.TempDir())
	cellTestReadOnlySources(t, cell)
	hooksDir := filepath.Join(cell.home, ".claude", "hooks")
	cellTestWriteFile(t, filepath.Join(hooksDir, "notes.txt"), "notes\n")
	cellTestWriteFile(t, filepath.Join(hooksDir, "bin", "toolpolicy"), "binary")
	readOnly := cellTestReadOnlyPaths(cell)
	readOnly = append(readOnly[:len(readOnly)-1], filepath.Join(hooksDir, "config.json"), filepath.Join(hooksDir, "notes.txt"))
	tests = append(tests, &testCase{
		_id:          "hooks-dir-top-level-files-only",
		_expected:    cellTestBindArgs(cell, nil, readOnly),
		_expectedDir: filepath.Join(cell.home, ".peek", "state"),

		cell: cell,
	})

	// no-socket-no-home-root
	cell = cellTestCell(t, t.TempDir())
	cellTestReadOnlySources(t, cell)
	tests = append(tests, &testCase{
		_id:          "no-socket-no-home-root",
		_expected:    cellTestBindArgs(cell, nil, cellTestReadOnlyPaths(cell)),
		_expectedDir: filepath.Join(cell.home, ".peek", "state"),

		cell: cell,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			mounts, err := test.cell.binds()
			assert.NoError(t, err)
			assert.Equal(t, test._expected, cellTestMountArgs(mounts))
			assert.DirExists(t, test._expectedDir)
			assert.NotContains(t, cellTestMountSources(mounts), test.cell.home)
			assert.NotContains(t, cellTestMountSources(mounts), cellTestDockerSocket)
		})
	}
}

func TestContainerCell_command(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		cell *containerCell
		view func(argv []string) []string
	}

	tokenValue := "secret-token-value"
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", tokenValue)
	cell := cellTestCell(t, newTestRepo(t))
	t.Setenv("HOME", cell.home)
	cellTestWriteFile(t, filepath.Join(cell.home, ".claude", "settings.json"), cellTestSettings)
	user := strconv.Itoa(os.Getuid()) + ":" + strconv.Itoa(os.Getgid())

	tests := make([]*testCase, 0)

	// flag-set
	tests = append(tests, &testCase{
		_id: "flag-set",
		_expected: []string{
			"docker", "run", "--rm", "--interactive", "--init",
			"--name", "smine-cell-group",
			"--label", "smine.routine=" + cellTestRoutine,
			"--network", "container:smine-cell-group-firewall",
			"--user", user,
			"--cap-drop", "ALL",
			"--security-opt", "no-new-privileges",
			"--security-opt", "seccomp=builtin",
		},

		cell: cell,
		view: func(argv []string) []string { return cellTestArgsFrom(argv, "docker", 19) },
	})

	// limits-from-config
	tests = append(tests, &testCase{
		_id:       "limits-from-config",
		_expected: []string{"--cpus", "2", "--memory", "3g", "--pids-limit", "4096"},

		cell: cell,
		view: func(argv []string) []string { return cellTestArgsFrom(argv, "--cpus", 6) },
	})

	// network-is-holder
	tests = append(tests, &testCase{
		_id:       "network-is-holder",
		_expected: []string{"--network", "container:smine-cell-group-firewall"},

		cell: cell,
		view: func(argv []string) []string { return cellTestArgsFrom(argv, "--network", 2) },
	})

	// workdir-is-worktree
	tests = append(tests, &testCase{
		_id:       "workdir-is-worktree",
		_expected: []string{"--workdir", cell.worktree},

		cell: cell,
		view: func(argv []string) []string { return cellTestArgsFrom(argv, "--workdir", 2) },
	})

	// ends-with-image-and-claude
	tests = append(tests, &testCase{
		_id:       "ends-with-image-and-claude",
		_expected: []string{cellImage, "claude"},

		cell: cell,
		view: func(argv []string) []string { return argv[len(argv)-2:] },
	})

	// token-by-name-only
	tests = append(tests, &testCase{
		_id:       "token-by-name-only",
		_expected: []string{"CLAUDE_CODE_OAUTH_TOKEN"},

		cell: cell,
		view: func(argv []string) []string {
			return append(cellTestArgsContaining(argv, "CLAUDE_CODE_OAUTH_TOKEN"), cellTestArgsContaining(argv, tokenValue)...)
		},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			command, err := test.cell.command(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, test._expected, test.view(command))
		})
	}
}

func TestContainerCell_start(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedLog   string
		_isSlotTimeout bool
		_shouldPass    bool

		cell       *containerCell
		dockerFail string
		dockerLog  string
		gateway    string
	}

	box := newSandbox(t, cellTestRoutine, cellTestManifest)
	installFakeDocker(t)
	cellTestWriteFile(t, filepath.Join(box.home, ".claude", "settings.json"), cellTestSettings)
	holder := "smine-cell-" + cellTestRoutine + "-firewall"
	sweepLine := "rm --force smine-cell-" + cellTestRoutine + " " + holder + "\n"
	holderLine := "run --detach --rm --name " + holder + " --label smine.routine=" + cellTestRoutine +
		" --user 0 --cap-drop ALL --cap-add NET_ADMIN --cap-add NET_RAW --add-host host.docker.internal:host-gateway smine-routine-cell sh -c " +
		relayScript([]string{"6101", "42442"}) + "\n"
	getentLine := "exec " + holder + " getent ahosts host.docker.internal\n"
	fenceLines := "exec " + holder + " iptables -A OUTPUT -d 192.0.2.254 -p tcp --dport 6101 -j ACCEPT\n" +
		"exec " + holder + " iptables -A OUTPUT -d 192.0.2.254 -p tcp --dport 42442 -j ACCEPT\n" +
		"exec " + holder + " iptables -A OUTPUT -d 192.0.2.254 -j REJECT\n"
	relayLines := "exec " + holder + " bash -c exec 3<>/dev/tcp/127.0.0.1/6101\n" +
		"exec " + holder + " bash -c exec 3<>/dev/tcp/127.0.0.1/42442\n"

	tests := make([]*testCase, 0)

	// happy-path
	tests = append(tests, &testCase{
		_id:            "happy-path",
		_expectedLog:   sweepLine + holderLine + getentLine + fenceLines + relayLines,
		_isSlotTimeout: false,
		_shouldPass:    true,

		cell:       cellTestSandboxCell(t, box),
		dockerFail: "",
		dockerLog:  filepath.Join(t.TempDir(), "docker.log"),
		gateway:    fakeGatewayAnswer,
	})

	// holder-fails
	tests = append(tests, &testCase{
		_id:            "holder-fails",
		_expectedLog:   sweepLine + holderLine,
		_isSlotTimeout: false,
		_shouldPass:    false,

		cell:       cellTestSandboxCell(t, box),
		dockerFail: "run",
		dockerLog:  filepath.Join(t.TempDir(), "docker.log"),
		gateway:    fakeGatewayAnswer,
	})

	// fence-fails
	tests = append(tests, &testCase{
		_id:            "fence-fails",
		_expectedLog:   sweepLine + holderLine + getentLine,
		_isSlotTimeout: false,
		_shouldPass:    false,

		cell:       cellTestSandboxCell(t, box),
		dockerFail: "exec",
		dockerLog:  filepath.Join(t.TempDir(), "docker.log"),
		gateway:    fakeGatewayAnswer,
	})

	// no-gateway-address
	tests = append(tests, &testCase{
		_id:            "no-gateway-address",
		_expectedLog:   sweepLine + holderLine + getentLine,
		_isSlotTimeout: false,
		_shouldPass:    false,

		cell:       cellTestSandboxCell(t, box),
		dockerFail: "",
		dockerLog:  filepath.Join(t.TempDir(), "docker.log"),
		gateway:    "",
	})

	// slot-timeout
	cell := cellTestSandboxCell(t, box)
	cellTestHoldAllSlots(t, cell)
	tests = append(tests, &testCase{
		_id:            "slot-timeout",
		_expectedLog:   "",
		_isSlotTimeout: true,
		_shouldPass:    false,

		cell:       cell,
		dockerFail: "",
		dockerLog:  filepath.Join(t.TempDir(), "docker.log"),
		gateway:    fakeGatewayAnswer,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			t.Setenv(fakeDockerFailEnv, test.dockerFail)
			t.Setenv(fakeDockerGatewayEnv, test.gateway)
			t.Setenv(fakeDockerLogEnv, test.dockerLog)
			synctest.Test(t, func(t *testing.T) {
				command, err := test.cell.start(context.Background())
				assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
				assert.Equal(t, test._shouldPass, command != nil)
				assert.Equal(t, test._isSlotTimeout, errors.Is(err, errSlotTimeout))
				assert.Equal(t, test._expectedLog, readDockerLog(t, test.dockerLog))
			})
		})
	}
}

func TestContainerCell_stop(t *testing.T) {
	type testCase struct {
		_id          string
		_expectedLog string

		cell             *containerCell
		dockerLog        string
		registrationPath string
	}

	installFakeDocker(t)
	sweepLine := "rm --force smine-cell-group smine-cell-group-firewall\n"

	tests := make([]*testCase, 0)

	// removes-both-containers
	cell := cellTestCell(t, t.TempDir())
	tests = append(tests, &testCase{
		_id:          "removes-both-containers",
		_expectedLog: sweepLine,

		cell:             cell,
		dockerLog:        filepath.Join(t.TempDir(), "docker.log"),
		registrationPath: filepath.Join(cell.registration().dir, "1.json"),
	})

	// clears-registration
	cell = cellTestCell(t, t.TempDir())
	tests = append(tests, &testCase{
		_id:          "clears-registration",
		_expectedLog: sweepLine,

		cell:             cell,
		dockerLog:        filepath.Join(t.TempDir(), "docker.log"),
		registrationPath: cellTestRegistrationFile(t, cell.registration(), cell.worktree),
	})

	// releases-slot
	cell = cellTestCell(t, t.TempDir())
	_, err := cell.acquireSlot(context.Background(), 0)
	require.NoError(t, err)
	tests = append(tests, &testCase{
		_id:          "releases-slot",
		_expectedLog: sweepLine,

		cell:             cell,
		dockerLog:        filepath.Join(t.TempDir(), "docker.log"),
		registrationPath: filepath.Join(cell.registration().dir, "1.json"),
	})

	// idempotent-second-call
	cell = cellTestCell(t, t.TempDir())
	_, err = cell.acquireSlot(context.Background(), 0)
	require.NoError(t, err)
	dockerLog := filepath.Join(t.TempDir(), "docker.log")
	registrationPath := cellTestRegistrationFile(t, cell.registration(), cell.worktree)
	t.Setenv(fakeDockerLogEnv, dockerLog)
	cell.stop()
	tests = append(tests, &testCase{
		_id:          "idempotent-second-call",
		_expectedLog: sweepLine + sweepLine,

		cell:             cell,
		dockerLog:        dockerLog,
		registrationPath: registrationPath,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			t.Setenv(fakeDockerLogEnv, test.dockerLog)
			test.cell.stop()
			assert.Equal(t, test._expectedLog, readDockerLog(t, test.dockerLog))
			assert.NoFileExists(t, test.registrationPath)
			assert.Nil(t, test.cell.slot)
			assert.True(t, cellTestSlotFree(t, test.cell, 0))
		})
	}
}

func TestFenceRules(t *testing.T) {
	type testCase struct {
		_id       string
		_expected [][]string

		addrs []string
		ports []string
	}

	tests := make([]*testCase, 0)

	// one-v4-address-two-ports
	tests = append(tests, &testCase{
		_id: "one-v4-address-two-ports",
		_expected: [][]string{
			{"iptables", "-A", "OUTPUT", "-d", "192.0.2.254", "-p", "tcp", "--dport", "6101", "-j", "ACCEPT"},
			{"iptables", "-A", "OUTPUT", "-d", "192.0.2.254", "-p", "tcp", "--dport", "42442", "-j", "ACCEPT"},
			{"iptables", "-A", "OUTPUT", "-d", "192.0.2.254", "-j", "REJECT"},
		},

		addrs: []string{"192.0.2.254"},
		ports: []string{"6101", "42442"},
	})

	// v4-and-v6
	tests = append(tests, &testCase{
		_id: "v4-and-v6",
		_expected: [][]string{
			{"iptables", "-A", "OUTPUT", "-d", "192.0.2.254", "-p", "tcp", "--dport", "6101", "-j", "ACCEPT"},
			{"iptables", "-A", "OUTPUT", "-d", "192.0.2.254", "-j", "REJECT"},
			{"ip6tables", "-A", "OUTPUT", "-d", "fdc4:f303:9324::254", "-p", "tcp", "--dport", "6101", "-j", "ACCEPT"},
			{"ip6tables", "-A", "OUTPUT", "-d", "fdc4:f303:9324::254", "-j", "REJECT"},
		},

		addrs: []string{"192.0.2.254", "fdc4:f303:9324::254"},
		ports: []string{"6101"},
	})

	// no-address
	tests = append(tests, &testCase{
		_id:       "no-address",
		_expected: nil,

		addrs: nil,
		ports: []string{"6101", "42442"},
	})

	// no-ports
	tests = append(tests, &testCase{
		_id: "no-ports",
		_expected: [][]string{
			{"iptables", "-A", "OUTPUT", "-d", "192.0.2.254", "-j", "REJECT"},
		},

		addrs: []string{"192.0.2.254"},
		ports: nil,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, fenceRules(test.addrs, test.ports))
		})
	}
}

func TestGatewayAddrs(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		output string
	}

	tests := make([]*testCase, 0)

	// duplicate-lines
	tests = append(tests, &testCase{
		_id:       "duplicate-lines",
		_expected: []string{"192.0.2.254"},

		output: "192.0.2.254 STREAM host.docker.internal\n192.0.2.254 DGRAM\n192.0.2.254 RAW\n",
	})

	// v4-and-v6
	tests = append(tests, &testCase{
		_id:       "v4-and-v6",
		_expected: []string{"fdc4:f303:9324::254", "192.0.2.254"},

		output: "fdc4:f303:9324::254 STREAM host.docker.internal\nfdc4:f303:9324::254 DGRAM\n192.0.2.254 STREAM\n",
	})

	// blank-lines
	tests = append(tests, &testCase{
		_id:       "blank-lines",
		_expected: []string{"192.0.2.254"},

		output: "\n\n192.0.2.254 STREAM host.docker.internal\n\n",
	})

	// empty
	tests = append(tests, &testCase{
		_id:       "empty",
		_expected: nil,

		output: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, gatewayAddrs(test.output))
		})
	}
}

func TestPassThroughEnv(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		environ []string
	}

	tests := make([]*testCase, 0)

	// token-and-prefixes-kept
	tests = append(tests, &testCase{
		_id:       "token-and-prefixes-kept",
		_expected: []string{"CLAUDE_CODE_OAUTH_TOKEN", "ROUTINE_GROUP", "SMINE_HEADLESS"},

		environ: []string{"CLAUDE_CODE_OAUTH_TOKEN=token", "ROUTINE_GROUP=group", "SMINE_HEADLESS=1"},
	})

	// path-home-dropped
	tests = append(tests, &testCase{
		_id:       "path-home-dropped",
		_expected: []string{"ROUTINE_TIMEOUT_S"},

		environ: []string{"PATH=/usr/bin", "HOME=/home/a", "ROUTINE_TIMEOUT_S=60"},
	})

	// disable-autoupdater-kept
	tests = append(tests, &testCase{
		_id:       "disable-autoupdater-kept",
		_expected: []string{"DISABLE_AUTOUPDATER"},

		environ: []string{"DISABLE_AUTOUPDATER=1", "DISABLE_TELEMETRY=1"},
	})

	// empty-environ
	tests = append(tests, &testCase{
		_id:       "empty-environ",
		_expected: nil,

		environ: nil,
	})

	// entry-without-equals
	tests = append(tests, &testCase{
		_id:       "entry-without-equals",
		_expected: []string{"SMINE_FLAG"},

		environ: []string{"SMINE_FLAG", "OTHER"},
	})

	// sorted-output
	tests = append(tests, &testCase{
		_id:       "sorted-output",
		_expected: []string{"CLAUDE_CODE_EFFORT", "ROUTINE_MODEL", "SMINE_DEV"},

		environ: []string{"SMINE_DEV=1", "CLAUDE_CODE_EFFORT=high", "ROUTINE_MODEL=opus"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, passThroughEnv(test.environ))
		})
	}
}

func TestRelayScript(t *testing.T) {
	expected := "socat TCP-LISTEN:6101,bind=127.0.0.1,reuseaddr,fork TCP:host.docker.internal:6101 & " +
		"socat TCP-LISTEN:42442,bind=127.0.0.1,reuseaddr,fork TCP:host.docker.internal:42442 & " +
		"exec sleep infinity"
	assert.Equal(t, expected, relayScript([]string{"6101", "42442"}))
}

func TestResolveRepoMounts(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   []*bindMount
		_shouldPass bool

		mounts       []*routines.ContainerMount
		registryPath string
	}

	ownRepo, plansRepo, peekRepo := t.TempDir(), t.TempDir(), t.TempDir()
	missingRepo := filepath.Join(t.TempDir(), "gone")
	registryPath := filepath.Join(t.TempDir(), settings.StateRegistryFile)
	registry := fmt.Sprintf(
		`{"repos":[{"name":"own","path":%q},{"name":"agent-plans","path":%q},{"name":"peek-mcp","path":%q},{"name":"gone","path":%q}]}`,
		ownRepo,
		plansRepo,
		peekRepo,
		missingRepo,
	)
	cellTestWriteFile(t, registryPath, registry)

	tests := make([]*testCase, 0)

	// pass-none
	tests = append(tests, &testCase{
		_id:         "pass-none",
		_expected:   nil,
		_shouldPass: true,

		mounts:       nil,
		registryPath: filepath.Join(t.TempDir(), "absent.json"),
	})

	// pass-read-write-and-read-only
	tests = append(tests, &testCase{
		_id:         "pass-read-write-and-read-only",
		_expected:   []*bindMount{{path: plansRepo}, {isReadOnly: true, path: peekRepo}},
		_shouldPass: true,

		mounts: []*routines.ContainerMount{
			{Access: routines.MountReadWrite, Repo: "agent-plans"},
			{Access: routines.MountReadOnly, Repo: "peek-mcp"},
		},
		registryPath: registryPath,
	})

	// fail-unregistered
	tests = append(tests, &testCase{
		_id:         "fail-unregistered",
		_expected:   nil,
		_shouldPass: false,

		mounts:       []*routines.ContainerMount{{Access: routines.MountReadOnly, Repo: "winds"}},
		registryPath: registryPath,
	})

	// fail-own-repo
	tests = append(tests, &testCase{
		_id:         "fail-own-repo",
		_expected:   nil,
		_shouldPass: false,

		mounts:       []*routines.ContainerMount{{Access: routines.MountReadWrite, Repo: "own"}},
		registryPath: registryPath,
	})

	// fail-missing-path
	tests = append(tests, &testCase{
		_id:         "fail-missing-path",
		_expected:   nil,
		_shouldPass: false,

		mounts:       []*routines.ContainerMount{{Access: routines.MountReadOnly, Repo: "gone"}},
		registryPath: registryPath,
	})

	// fail-unreadable-registry
	brokenRegistryPath := filepath.Join(t.TempDir(), settings.StateRegistryFile)
	cellTestWriteFile(t, brokenRegistryPath, "{")
	tests = append(tests, &testCase{
		_id:         "fail-unreadable-registry",
		_expected:   nil,
		_shouldPass: false,

		mounts:       []*routines.ContainerMount{{Access: routines.MountReadOnly, Repo: "peek-mcp"}},
		registryPath: brokenRegistryPath,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			binds, err := resolveRepoMounts(test.mounts, test.registryPath, ownRepo)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, binds)
		})
	}
}

func TestSessionRegistration_clear(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedExists bool

		path         string
		registration *sessionRegistration
	}

	tests := make([]*testCase, 0)

	// own-cwd-removed
	registration := cellTestRegistration(t)
	tests = append(tests, &testCase{
		_id:             "own-cwd-removed",
		_expectedExists: false,

		path:         cellTestRegistrationFile(t, registration, cellTestWorktree),
		registration: registration,
	})

	// other-cwd-kept
	registration = cellTestRegistration(t)
	tests = append(tests, &testCase{
		_id:             "other-cwd-kept",
		_expectedExists: true,

		path:         cellTestRegistrationFile(t, registration, "/worktrees/other"),
		registration: registration,
	})

	// malformed-kept
	registration = cellTestRegistration(t)
	malformedPath := filepath.Join(registration.dir, "7.json")
	cellTestWriteFile(t, malformedPath, "{")
	tests = append(tests, &testCase{
		_id:             "malformed-kept",
		_expectedExists: true,

		path:         malformedPath,
		registration: registration,
	})

	// dir-missing
	registration = cellTestRegistration(t)
	tests = append(tests, &testCase{
		_id:             "dir-missing",
		_expectedExists: false,

		path:         filepath.Join(registration.dir, "1.json"),
		registration: registration,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			test.registration.clear()
			assert.Equal(t, test._expectedExists, cellTestFileExists(test.path))
		})
	}
}

func TestSessionRegistration_write(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *containerSession
		_isWritten  bool
		_shouldPass bool

		registration *sessionRegistration
	}

	written := &containerSession{
		Cwd:        cellTestWorktree,
		Entrypoint: containerEntry,
		Pid:        cellTestPid,
		SessionId:  cellTestSessionId,
	}

	tests := make([]*testCase, 0)

	// transcript-missing
	tests = append(tests, &testCase{
		_id:         "transcript-missing",
		_expected:   nil,
		_isWritten:  false,
		_shouldPass: true,

		registration: cellTestRegistration(t),
	})

	// transcript-present
	registration := cellTestRegistration(t)
	cellTestTranscript(t, registration, cellTestSessionId)
	require.NoError(t, os.MkdirAll(registration.dir, 0o755))
	tests = append(tests, &testCase{
		_id:         "transcript-present",
		_expected:   written,
		_isWritten:  true,
		_shouldPass: true,

		registration: registration,
	})

	// dir-missing-created
	registration = cellTestRegistration(t)
	cellTestTranscript(t, registration, cellTestSessionId)
	tests = append(tests, &testCase{
		_id:         "dir-missing-created",
		_expected:   written,
		_isWritten:  true,
		_shouldPass: true,

		registration: registration,
	})

	// second-write-overwrites
	registration = cellTestRegistration(t)
	cellTestTranscript(t, registration, "session-old")
	cellTestTranscript(t, registration, cellTestSessionId)
	isWritten, err := registration.write(cellTestPid, "session-old")
	require.NoError(t, err)
	require.True(t, isWritten)
	tests = append(tests, &testCase{
		_id:         "second-write-overwrites",
		_expected:   written,
		_isWritten:  true,
		_shouldPass: true,

		registration: registration,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			isWritten, err := test.registration.write(cellTestPid, cellTestSessionId)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._isWritten, isWritten)
			assert.Equal(t, test._expected, cellTestReadRegistration(filepath.Join(test.registration.dir, strconv.Itoa(cellTestPid)+".json")))
		})
	}
}

// cellTestArgsContaining returns the args that contain text.
func cellTestArgsContaining(argv []string, text string) []string {
	var matches []string
	for _, arg := range argv {
		if strings.Contains(arg, text) {
			matches = append(matches, arg)
		}
	}
	return matches
}

// cellTestArgsFrom returns count args starting at the first occurrence of
// first, nil when first is absent or the window runs past the end.
func cellTestArgsFrom(argv []string, first string, count int) []string {
	for index, arg := range argv {
		if arg == first && index+count <= len(argv) {
			return argv[index : index+count]
		}
	}
	return nil
}

// cellTestBindArgs is the expected mount list of cell: the rendered
// settings, the fixed writable set, extraWritable, then readOnly.
func cellTestBindArgs(cell *containerCell, extraWritable, readOnly []string) []string {
	claudeHome := filepath.Join(cell.home, ".claude")
	writable := []string{
		filepath.Join(cell.repoRoot, ".git"),
		filepath.Join(cell.repoRoot, ".claude", "worktrees"),
		cell.worktree,
		filepath.Join(claudeHome, "projects"),
		filepath.Join(claudeHome, "plans"),
		filepath.Join(claudeHome, "context-required"),
		filepath.Join(claudeHome, "read-gate-state"),
		filepath.Join(claudeHome, "tool-policy"),
		filepath.Join(claudeHome, "acdsl"),
		filepath.Join(cell.home, ".cache", "claude-routine", "artifacts"),
		cell.config.SteerDir,
		filepath.Join(cell.home, ".peek", "state"),
		cell.backlogDir,
	}
	writable = append(writable, extraWritable...)

	var args []string
	for _, path := range writable {
		args = append(args, "type=bind,source="+path+",target="+path)
	}
	for _, path := range readOnly {
		args = append(args, "type=bind,source="+path+",target="+path+",readonly")
	}
	return args
}

// cellTestBuildHome is a HOME whose deployed hooks bin holds hooks.
func cellTestBuildHome(t *testing.T, hooks ...string) string {
	t.Helper()
	home := t.TempDir()
	for _, hook := range hooks {
		cellTestWriteFile(t, filepath.Join(home, ".claude", "hooks", "bin", hook), "binary")
	}
	return home
}

// cellTestBuildLine is the fake docker log line of the image build.
func cellTestBuildLine(home, repoRoot, hooks string) string {
	args := []string{
		"build",
		"--file", filepath.Join(repoRoot, dockerfilePath),
		"--tag", cellImage,
		"--build-arg", "CLAUDE_VERSION=9.9.9",
		"--build-arg", "HOOKS=" + hooks,
		"--build-arg", "HOST_GID=" + strconv.Itoa(os.Getgid()),
		"--build-arg", "HOST_HOME=" + home,
		"--build-arg", "HOST_UID=" + strconv.Itoa(os.Getuid()),
		"--build-arg", "PEEK_MCP_VERSION=v1.2.14",
		repoRoot,
	}
	return strings.Join(args, " ") + "\n"
}

// cellTestBuildRepo is a checkout pinning the peek version and holding a
// command dir per name.
func cellTestBuildRepo(t *testing.T, commands ...string) string {
	t.Helper()
	repoRoot := t.TempDir()
	cellTestWriteFile(t, filepath.Join(repoRoot, peekVersionFile), "v1.2.14\n")
	for _, command := range commands {
		require.NoError(t, os.MkdirAll(filepath.Join(repoRoot, "cmd", command), 0o755))
	}
	return repoRoot
}

// cellTestCell is a container cell over its own temp home; nothing in it
// reads HOME.
func cellTestCell(t *testing.T, repoRoot string) *containerCell {
	t.Helper()
	home := t.TempDir()
	defaults, err := install.InstallValues(install.Record{})
	require.NoError(t, err)
	runConfig := &Config{
		ContainerCpus:   "2",
		ContainerMemory: "3g",
		ContainerRoot:   filepath.Join(home, ".cache", "claude-routine", "container"),
		Group:           cellTestGroup,
		StateDir:        filepath.Join(home, ".claude", "smine"),
		SteerDir:        filepath.Join(home, ".cache", "claude-routine", "steer", cellTestGroup),
	}
	cell := &containerCell{
		backlogDir: filepath.Join(home, "GolandProjects", "agent-plans", "repo", settings.StoreBacklogDir),
		config:     runConfig,
		home:       home,
		ports:      []string{defaults.HeartbeatPort, defaults.PeekControlPort},
		repoRoot:   repoRoot,
		routine:    cellTestRoutine,
		worktree:   filepath.Join(home, "worktrees", cellTestGroup),
	}
	t.Cleanup(func() { cellTestReleaseSlot(cell) })
	return cell
}

func cellTestFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func cellTestHoldAllSlots(t *testing.T, cell *containerCell) {
	t.Helper()
	for index := range containerSlots {
		cellTestHoldSlot(t, cell, index)
	}
}

// cellTestHoldSlot takes slot index of cell's slot dir for the rest of the
// test.
func cellTestHoldSlot(t *testing.T, cell *containerCell, index int) *os.File {
	t.Helper()
	path := filepath.Join(cell.config.ContainerRoot, "slots", strconv.Itoa(index)+".lock")
	slot, isHeld, err := acquireLock(path, 0)
	require.NoError(t, err)
	require.True(t, isHeld)
	t.Cleanup(func() { slot.Close() })
	return slot
}

func cellTestMountArgs(mounts []*bindMount) []string {
	args := make([]string, 0, len(mounts))
	for _, mount := range mounts {
		args = append(args, mount.arg())
	}
	return args
}

func cellTestMountSources(mounts []*bindMount) []string {
	sources := make([]string, 0, len(mounts))
	for _, mount := range mounts {
		sources = append(sources, mount.path)
	}
	return sources
}

// cellTestReadOnlyPaths are the read-only sources cellTestReadOnlySources
// creates, in mount order.
func cellTestReadOnlyPaths(cell *containerCell) []string {
	claudeHome := filepath.Join(cell.home, ".claude")
	return []string{
		filepath.Join(claudeHome, "settings.json"),
		filepath.Join(claudeHome, "skills"),
		filepath.Join(claudeHome, "agents"),
		filepath.Join(claudeHome, "context"),
		filepath.Join(claudeHome, "AGENTS.md"),
		filepath.Join(claudeHome, "settings.local.json"),
		filepath.Join(cell.config.StateDir, settings.StateRegistryFile),
		filepath.Join(claudeHome, "heartbeat", "config.json"),
		filepath.Join(claudeHome, "hooks", "config.json"),
	}
}

func cellTestReadOnlySources(t *testing.T, cell *containerCell) {
	t.Helper()
	claudeHome := filepath.Join(cell.home, ".claude")
	for _, dir := range []string{"skills", "agents", "context"} {
		require.NoError(t, os.MkdirAll(filepath.Join(claudeHome, dir), 0o755))
	}
	cellTestWriteFile(t, filepath.Join(claudeHome, "settings.json"), cellTestSettings)
	cellTestWriteFile(t, filepath.Join(claudeHome, "AGENTS.md"), "# Global\n")
	cellTestWriteFile(t, filepath.Join(claudeHome, "settings.local.json"), "{}")
	cellTestWriteFile(t, filepath.Join(cell.config.StateDir, settings.StateRegistryFile), `{"repos":[]}`)
	cellTestWriteFile(t, filepath.Join(claudeHome, "heartbeat", "config.json"), "{}")
	cellTestWriteFile(t, filepath.Join(claudeHome, "hooks", "config.json"), "{}")
}

func cellTestReadRegistration(path string) *containerSession {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var session containerSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil
	}
	return &session
}

func cellTestRegistration(t *testing.T) *sessionRegistration {
	t.Helper()
	root := t.TempDir()
	registration := &sessionRegistration{
		cwd:         cellTestWorktree,
		dir:         filepath.Join(root, ".claude", "smine", registrationsDir),
		projectsDir: filepath.Join(root, ".claude", "projects"),
	}
	return registration
}

// cellTestRegistrationFile writes a registration of cwd into registration's
// dir and returns its path.
func cellTestRegistrationFile(t *testing.T, registration *sessionRegistration, cwd string) string {
	t.Helper()
	session := &containerSession{Cwd: cwd, Entrypoint: containerEntry, Pid: cellTestPid, SessionId: cellTestSessionId}
	data, err := json.Marshal(session)
	require.NoError(t, err)

	path := filepath.Join(registration.dir, strconv.Itoa(cellTestPid)+".json")
	cellTestWriteFile(t, path, string(data))
	return path
}

func cellTestReleaseSlot(cell *containerCell) {
	if cell.slot != nil {
		cell.slot.Close()
		cell.slot = nil
	}
}

// cellTestSandboxCell is a container cell of the sandbox's routine with its
// own slot dir.
func cellTestSandboxCell(t *testing.T, box *sandbox) *containerCell {
	t.Helper()
	runner, err := NewRunner(box.routineDir)
	require.NoError(t, err)

	runner.config.ContainerRoot = t.TempDir()
	cell := newContainerCell(runner, filepath.Join(box.wtRoot, runner.config.Group))
	t.Cleanup(func() { cellTestReleaseSlot(cell) })
	return cell
}

func cellTestSlotFree(t *testing.T, cell *containerCell, index int) bool {
	t.Helper()
	path := filepath.Join(cell.config.ContainerRoot, "slots", strconv.Itoa(index)+".lock")
	slot, isHeld, err := acquireLock(path, 0)
	require.NoError(t, err)
	if isHeld {
		slot.Close()
	}
	return isHeld
}

func cellTestSlotName(cell *containerCell) string {
	if cell.slot == nil {
		return ""
	}
	return filepath.Base(cell.slot.Name())
}

// cellTestTranscript writes the transcript of sessionId under
// registration's projects dir.
func cellTestTranscript(t *testing.T, registration *sessionRegistration, sessionId string) {
	t.Helper()
	cellTestWriteFile(t, filepath.Join(registration.projectsDir, "-worktrees-group", sessionId+".jsonl"), "{}\n")
}

func cellTestWriteFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

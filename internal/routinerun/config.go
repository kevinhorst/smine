// Package routinerun executes routine runs for cmd/routine.
package routinerun

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	settings "github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/models"
	"github.com/kevinhorst/smine/internal/routines"
)

const (
	defaultExitGraceS    = 60
	defaultPollS         = 2
	defaultRunTimeoutS   = 46800
	defaultStageTimeoutS = 7200
)

// Exit statuses shared with the README's exit-code table.
const (
	exitCancelled   = 143
	exitConfig      = 78
	exitDeadline    = 124
	exitLockTimeout = 75
	exitOk          = 0
	exitSoftware    = 70
	exitUnavailable = 69
)

// Config is the run's environment, read once at start.
type Config struct {
	AgentwtPath       string // the installed binary the cells also call
	BranchPrefix      string
	BudgetUsd         string // "" = flag omitted, no cap
	CadenceDays       int
	ContainerCpus     string // docker --cpus
	ContainerMemory   string // docker --memory
	ContainerMounts   []*routines.ContainerMount
	ContainerRoot     string // slot locks; never mounted
	ExitGrace         time.Duration
	ExtraPrompt       string
	Group             string
	IsContainerActive bool
	MaxOpenBranches   int    // 0 = unlimited
	Model             string // "" = flag omitted, settings.json model
	PermissionMode    string // "" = flag omitted, settings.json defaultMode
	Poll              time.Duration
	RunTimeout        time.Duration
	StageTimeout      time.Duration
	StateDir          string // <claude home>/smine: registry and machine state
	SteerDir          string
	TokenLabel        string
	WorktreeRoot      string
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		log.Printf("envInt: %s: Invalid value %q, using %d", key, value, fallback)
		return fallback
	}
	return parsed
}

// envSeconds reads a positive second count; zero or invalid falls back.
func envSeconds(key string, fallback int) time.Duration {
	seconds := envInt(key, fallback)
	if seconds == 0 {
		seconds = fallback
	}
	return time.Duration(seconds) * time.Second
}

func envString(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func loadConfig(routineName string) (*Config, error) {
	env := processEnv("ROUTINE_CONTAINER_ROOT", "ROUTINE_GROUP", "ROUTINE_STEER_ROOT", "ROUTINE_WT_ROOT")
	worktreeRoot, err := routines.WorktreeRoot(env)
	if err != nil {
		return nil, fmt.Errorf("loadConfig: %w", err)
	}

	steerDir, err := routines.SteerDir(env, routineName)
	if err != nil {
		return nil, fmt.Errorf("loadConfig: %w", err)
	}

	stateDir, err := settings.StateDir("")
	if err != nil {
		return nil, fmt.Errorf("loadConfig: %w", err)
	}

	containerRoot, err := routines.ContainerRoot(env)
	if err != nil {
		return nil, fmt.Errorf("loadConfig: %w", err)
	}

	isContainerActive, err := routines.ParseContainerFlag(os.Getenv(routines.EnvContainer))
	if err != nil {
		return nil, fmt.Errorf("loadConfig: %w", err)
	}

	containerMounts, err := routines.ParseContainerMounts(os.Getenv(routines.EnvContainerMounts))
	if err != nil {
		return nil, fmt.Errorf("loadConfig: %w", err)
	}

	group := routines.GroupName(env, routineName)
	config := &Config{
		AgentwtPath:       filepath.Join(smineBinDir(), "smine-agentwt"),
		BranchPrefix:      envString("ROUTINE_BRANCH_PREFIX", "claude-routines/"+group),
		BudgetUsd:         os.Getenv("ROUTINE_MAX_BUDGET_USD"),
		CadenceDays:       envInt("ROUTINE_CADENCE_DAYS", 1),
		ContainerCpus:     envString(routines.EnvContainerCpus, routines.DefaultContainerCpus),
		ContainerMemory:   envString(routines.EnvContainerMemory, routines.DefaultContainerMemory),
		ContainerMounts:   containerMounts,
		ContainerRoot:     containerRoot,
		ExitGrace:         envSeconds("ROUTINE_CHAT_EXIT_GRACE_S", defaultExitGraceS),
		ExtraPrompt:       os.Getenv("ROUTINE_EXTRA_PROMPT"),
		Group:             group,
		IsContainerActive: isContainerActive,
		MaxOpenBranches:   envInt("ROUTINE_MAX_OPEN_BRANCHES", 0),
		Model:             os.Getenv("ROUTINE_MODEL"),
		PermissionMode:    os.Getenv("ROUTINE_PERMISSION_MODE"),
		Poll:              envSeconds("ROUTINE_CHAT_POLL_S", defaultPollS),
		RunTimeout:        envSeconds("ROUTINE_TIMEOUT_S", defaultRunTimeoutS),
		StageTimeout:      envSeconds("ROUTINE_STAGE_TIMEOUT_S", defaultStageTimeoutS),
		StateDir:          stateDir,
		SteerDir:          steerDir,
		TokenLabel:        os.Getenv("ROUTINE_TOKEN"),
		WorktreeRoot:      worktreeRoot,
	}

	if config.Model != "" {
		vocabulary, err := models.LoadHome(filepath.Dir(settings.DefaultPath()))
		if err != nil {
			return nil, fmt.Errorf("loadConfig: %w", err)
		}
		if err := vocabulary.Check(config.Model); err != nil {
			return nil, fmt.Errorf("loadConfig: ROUTINE_MODEL: %w", err)
		}
	}
	return config, nil
}

func processEnv(keys ...string) map[string]string {
	env := make(map[string]string, len(keys))
	for _, key := range keys {
		env[key] = os.Getenv(key)
	}
	return env
}

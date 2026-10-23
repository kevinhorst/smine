// Command skilleval runs the eval, A/B and smoke layers of skill testing step by step.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/heartbeat"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/peek"
	"github.com/kevinhorst/smine/internal/routinerun"
	"github.com/kevinhorst/smine/internal/skilleval"
	"github.com/kevinhorst/smine/internal/trash"
)

const (
	abDirArgCount        = 1
	armArgCount          = 2
	bgWaitCeilingEnv     = "CLAUDE_CODE_PRINT_BG_WAIT_CEILING_MS"
	briefArgCountCompare = 2
	briefArgCountLabel   = 3
	cleanupArgCount      = 2
	genArgCount          = 2
	initArgCount         = 1
	promoteMaxArgCount   = 2
	provisionArgCount    = 1
	smokeArgCount        = 1
	smokeVerdictArgCount = 2
	defaultEvalsRoot     = "evals"
	defaultSchemaPath    = "skills/skillroutine/skillroutine-eval/reference/schema.json"
	defaultSkillsRoot    = "skills"
	exitBadMaster        = 65
	exitBadVariant       = 72
	exitClosedArg        = 75
	exitDeployedDiffers  = 3
	exitFailure          = 1
	exitManifestChanged  = 76
	exitMissingMaster    = 66
	exitNoDefinition     = 69
	exitNoGolden         = 74
	exitNotRunnable      = 73
	exitOverWindow       = 71
	exitUnknownToken     = 70
	exitUnregisteredRepo = 67
	exitUnresolvableRef  = 68
	exitUsage            = 2
	exitUsageGoldens     = 64
	usage                = "usage: skilleval ab-collect <ab-dir> | ab-init [-layer eval|smoke] [-replicas <n>] [-token <label>] -variant <descriptor> <skill> | ab-report <ab-dir> | arm [-heartbeat <bin>] [-resume <session id>] <run-dir> <label> | brief adherence|golden <run-dir> <label> | brief compare <run-dir> | cleanup <run-dir> <label> | compose [-golden produce|test] [-set <name>[=<value>]]… <skill> | fold [-peek <url>] [-schema <path>] <run-dir> | goldens gen <master> | goldens check | init [-golden produce|test] [-replicas <n>] [-set <name>[=<value>]]… -token <label> <skill> | promote <run-dir> [<label>] | provision <run-dir> | smoke [-heartbeat <bin>] [-token <label>] <leaf> | smoke-verdict [-heartbeat <bin>] <leaf> <session-id>"
)

var goldenKinds = []string{"", skilleval.GoldenProduce, skilleval.GoldenTest}

type repeatedFlag []string

func (f *repeatedFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func (f *repeatedFlag) String() string {
	return strings.Join(*f, " ")
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	code := dispatch(ctx, os.Args[1:])
	stop()
	os.Exit(code)
}

func checkGoldens() int {
	stale, err := skilleval.CheckGoldens(defaultSkillsRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitFailure
	}

	if len(stale) > 0 {
		fmt.Fprintln(os.Stderr, "goldens: stale derived files — run go run ./cmd/skilleval goldens gen <master>:\n  "+strings.Join(stale, "\n  "))
		return exitFailure
	}

	return 0
}

func dispatch(ctx context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, usage)
		return exitUsage
	}

	switch args[0] {
	case "ab-collect":
		return runAbCollect(args[1:])
	case "ab-init":
		return runAbInit(args[1:])
	case "ab-report":
		return runAbReport(args[1:])
	case "arm":
		return runArm(ctx, args[1:])
	case "brief":
		return runBrief(args[1:])
	case "cleanup":
		return runCleanup(ctx, args[1:])
	case "compose":
		return runCompose(args[1:])
	case "fold":
		return runFold(ctx, args[1:])
	case "goldens":
		return runGoldens(args[1:])
	case "init":
		return runInit(args[1:])
	case "promote":
		return runPromote(args[1:])
	case "provision":
		return runProvision(ctx, args[1:])
	case "smoke":
		return runSmoke(ctx, args[1:])
	case "smoke-verdict":
		return runSmokeVerdict(ctx, args[1:])
	default:
		fmt.Fprintln(os.Stderr, usage)
		return exitUsage
	}
}

func failureCode(err error) int {
	if errors.Is(err, skilleval.ErrManifestChanged) {
		return exitManifestChanged
	}
	return exitFailure
}

func generateGoldens(masterPath string) int {
	err := skilleval.GenerateGoldens(masterPath)
	switch {
	case err == nil:
		return 0
	case errors.Is(err, os.ErrNotExist):
		fmt.Fprintln(os.Stderr, err)
		return exitMissingMaster
	case errors.Is(err, skilleval.ErrBadMaster):
		fmt.Fprintln(os.Stderr, err)
		return exitBadMaster
	}
	fmt.Fprintln(os.Stderr, err)
	return exitFailure
}

func initExitCode(err error) int {
	switch {
	case errors.Is(err, skilleval.ErrBadRunArg):
		return exitUsage
	case errors.Is(err, skilleval.ErrClosedArg):
		return exitClosedArg
	case errors.Is(err, skilleval.ErrNoDefinition):
		return exitNoDefinition
	case errors.Is(err, skilleval.ErrNoGolden):
		return exitNoGolden
	case errors.Is(err, skilleval.ErrUnknownToken):
		return exitUnknownToken
	case errors.Is(err, skilleval.ErrOverWindow):
		return exitOverWindow
	case errors.Is(err, skilleval.ErrBadVariant):
		return exitBadVariant
	case errors.Is(err, skilleval.ErrNotRunnable):
		return exitNotRunnable
	}
	return exitFailure
}

// installValues is the install record's values for this checkout, the catalog defaults when the record is unreadable.
func installValues(fallback string) install.Values {
	env, err := install.NewEnv(".", io.Discard)
	if err == nil {
		return env.Values
	}

	fmt.Fprintf(os.Stderr, "skilleval: %v — %s\n", err, fallback)
	defaults, err := install.InstallValues(install.Record{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "skilleval: %v\n", err)
	}
	return defaults
}

func newHeartbeatCli(heartbeatBin, home string) *heartbeat.Cli {
	values := installValues("default heartbeat port")
	markers := values.Markers(home, "")
	return &heartbeat.Cli{
		Addr:       string(markers.Expand([]byte(heartbeat.AddrTemplate))),
		BinaryPath: heartbeatBin,
		ClaudeHome: filepath.Join(home, ".claude"),
	}
}

func reportSmoke(err error, outcome *skilleval.SmokeOutcome) int {
	switch {
	case errors.Is(err, skilleval.ErrDeployedDiffers):
		fmt.Fprintln(os.Stderr, err)
		return exitDeployedDiffers
	case err != nil:
		fmt.Fprintln(os.Stderr, err)
		return exitFailure
	}

	for _, line := range outcome.Lines {
		fmt.Println(line)
	}
	if !outcome.IsPass {
		return exitFailure
	}
	return 0
}

func runAbCollect(args []string) int {
	if len(args) != abDirArgCount {
		fmt.Fprintln(os.Stderr, usage)
		return exitUsage
	}

	if err := skilleval.AbCollect(args[0]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitFailure
	}

	return 0
}

func runAbInit(args []string) int {
	flags := flag.NewFlagSet("ab-init", flag.ExitOnError)
	layer := flags.String("layer", skilleval.LayerEval, "lower layer whose records are compared: eval or smoke")
	replicas := flags.Int("replicas", 0, "runs per side (0 = the definition's, else 1)")
	token := flags.String("token", "", "stored token label the runs bill; unused by a version variant")
	variant := flags.String("variant", "", "one changed factor: model:<name>, effort:<level>, no-<mcp>, overlay:<patch>, version:<a>..<b>")
	_ = flags.Parse(args)
	isLayer := *layer == skilleval.LayerEval || *layer == skilleval.LayerSmoke
	isUsage := flags.NArg() != initArgCount || *replicas < 0
	if isUsage || !isLayer {
		fmt.Fprintln(os.Stderr, usage)
		return exitUsage
	}

	request := &skilleval.AbInitRequest{
		Date:       time.Now().Format(time.DateOnly),
		EvalsRoot:  defaultEvalsRoot,
		Layer:      *layer,
		Replicas:   *replicas,
		Skill:      flags.Arg(0),
		SkillsRoot: defaultSkillsRoot,
		Token:      *token,
		Variant:    *variant,
	}
	abDir, err := skilleval.InitAb(request)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return initExitCode(err)
	}

	fmt.Println(abDir)
	return 0
}

func runAbReport(args []string) int {
	if len(args) != abDirArgCount {
		fmt.Fprintln(os.Stderr, usage)
		return exitUsage
	}

	if err := skilleval.AbReport(args[0]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitFailure
	}

	return 0
}

func runArm(ctx context.Context, args []string) int {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitFailure
	}

	flags := flag.NewFlagSet("arm", flag.ExitOnError)
	heartbeatBin := flags.String("heartbeat", filepath.Join(config.SmineBinDir(runtime.GOOS, home), "smine-heartbeat"), "heartbeat binary the arm reads its session's enforced runs from")
	resume := flags.String("resume", "", "session id of the arm's stopped cell: resume it in its worktree instead of starting a fresh cell")
	_ = flags.Parse(args)
	if flags.NArg() != armArgCount {
		fmt.Fprintln(os.Stderr, usage)
		return exitUsage
	}

	if err := os.Setenv(bgWaitCeilingEnv, "0"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitFailure
	}

	cli := newHeartbeatCli(*heartbeatBin, home)
	release := routinerun.PreventSleep()
	defer release()
	if err := skilleval.RunArm(ctx, flags.Arg(1), *resume, flags.Arg(0), cli.Runtime); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return failureCode(err)
	}

	return 0
}

func runBrief(args []string) int {
	isLabelBrief := len(args) == briefArgCountLabel && args[0] != skilleval.BriefCompare
	isCompareBrief := len(args) == briefArgCountCompare && args[0] == skilleval.BriefCompare
	if !isLabelBrief && !isCompareBrief {
		fmt.Fprintln(os.Stderr, usage)
		return exitUsage
	}

	label := ""
	if isLabelBrief {
		label = args[2]
	}

	text, err := skilleval.Brief(args[0], label, args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return failureCode(err)
	}

	fmt.Print(text)
	return 0
}

func runCleanup(ctx context.Context, args []string) int {
	if len(args) != cleanupArgCount {
		fmt.Fprintln(os.Stderr, usage)
		return exitUsage
	}

	stateDir, err := config.StateDir("")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return failureCode(err)
	}

	report, err := skilleval.CleanupArm(ctx, args[1], args[0], trash.New(stateDir, 0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return failureCode(err)
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitFailure
	}

	fmt.Println(string(data))
	if !report.IsAllRemoved() {
		return exitFailure
	}

	return 0
}

func runFold(ctx context.Context, args []string) int {
	values := installValues("default peek port")
	flags := flag.NewFlagSet("fold", flag.ExitOnError)
	peekEndpoint := flags.String("peek", values.PeekEndpoint(), "peek-mcp MCP endpoint")
	schemaPath := flags.String("schema", defaultSchemaPath, "eval.json schema")
	_ = flags.Parse(args)
	if flags.NArg() != 1 {
		fmt.Fprintln(os.Stderr, usage)
		return exitUsage
	}

	if err := skilleval.Fold(ctx, peek.NewClient(*peekEndpoint), flags.Arg(0), *schemaPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return failureCode(err)
	}

	return 0
}

func runGoldens(args []string) int {
	isGen := len(args) == genArgCount && args[0] == "gen"
	isCheck := len(args) == 1 && args[0] == "check"
	switch {
	case isGen:
		return generateGoldens(args[1])
	case isCheck:
		return checkGoldens()
	}
	fmt.Fprintln(os.Stderr, usage)
	return exitUsageGoldens
}

func runCompose(args []string) int {
	flags := flag.NewFlagSet("compose", flag.ExitOnError)
	golden := flags.String("golden", "", "golden run kind: produce or test (default test when the definition has a golden)")
	var sets repeatedFlag
	flags.Var(&sets, "set", "name=value, or a bare flag name: sets an arg the definition opens; repeatable")
	_ = flags.Parse(args)
	isUsage := flags.NArg() != initArgCount || !slices.Contains(goldenKinds, *golden)
	if isUsage {
		fmt.Fprintln(os.Stderr, usage)
		return exitUsage
	}

	request := &skilleval.InitRequest{
		Golden:     *golden,
		Overrides:  skilleval.ParseSets(sets),
		Skill:      flags.Arg(0),
		SkillsRoot: defaultSkillsRoot,
	}
	prompt, err := skilleval.Compose(request)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return initExitCode(err)
	}

	fmt.Println(prompt)
	return 0
}

func runInit(args []string) int {
	flags := flag.NewFlagSet("init", flag.ExitOnError)
	replicas := flags.Int("replicas", 0, "runs of the configuration (0 = the definition's, else 1)")
	token := flags.String("token", "", "stored token label the runs bill")
	golden := flags.String("golden", "", "golden run kind: produce or test (default test when the definition has a golden)")
	var sets repeatedFlag
	flags.Var(&sets, "set", "name=value, or a bare flag name: sets an arg the definition opens; repeatable")
	_ = flags.Parse(args)
	isUsage := flags.NArg() != initArgCount || *replicas < 0
	if isUsage || !slices.Contains(goldenKinds, *golden) {
		fmt.Fprintln(os.Stderr, usage)
		return exitUsage
	}

	request := &skilleval.InitRequest{
		Date:       time.Now().Format(time.DateOnly),
		EvalsRoot:  defaultEvalsRoot,
		Golden:     *golden,
		Overrides:  skilleval.ParseSets(sets),
		Replicas:   *replicas,
		Skill:      flags.Arg(0),
		SkillsRoot: defaultSkillsRoot,
		Token:      *token,
	}
	runDir, err := skilleval.Init(request)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return initExitCode(err)
	}

	fmt.Println(runDir)
	return 0
}

func runPromote(args []string) int {
	isUsage := len(args) == 0 || len(args) > promoteMaxArgCount
	if isUsage {
		fmt.Fprintln(os.Stderr, usage)
		return exitUsage
	}

	label := ""
	if len(args) == promoteMaxArgCount {
		label = args[1]
	}

	seedFile, err := skilleval.Promote(label, args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return failureCode(err)
	}

	fmt.Println(seedFile)
	return 0
}

func runProvision(ctx context.Context, args []string) int {
	if len(args) != provisionArgCount {
		fmt.Fprintln(os.Stderr, usage)
		return exitUsage
	}

	err := skilleval.Provision(ctx, args[0])
	switch {
	case err == nil:
		return 0
	case errors.Is(err, skilleval.ErrUnregisteredRepo):
		fmt.Fprintln(os.Stderr, err)
		return exitUnregisteredRepo
	case errors.Is(err, skilleval.ErrUnresolvableRef):
		fmt.Fprintln(os.Stderr, err)
		return exitUnresolvableRef
	}
	fmt.Fprintln(os.Stderr, err)
	return failureCode(err)
}

func runSmoke(ctx context.Context, args []string) int {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitFailure
	}

	flags := flag.NewFlagSet("smoke", flag.ExitOnError)
	heartbeatBin := flags.String("heartbeat", filepath.Join(config.SmineBinDir(runtime.GOOS, home), "smine-heartbeat"), "heartbeat binary the smoke reads its session's enforced runs from")
	token := flags.String("token", "", "stored token label a headless smoke bills")
	_ = flags.Parse(args)
	if flags.NArg() != smokeArgCount {
		fmt.Fprintln(os.Stderr, usage)
		return exitUsage
	}

	if err := os.Setenv(bgWaitCeilingEnv, "0"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitFailure
	}

	cli := newHeartbeatCli(*heartbeatBin, home)
	leaf := flags.Arg(0)
	release := routinerun.PreventSleep()
	defer release()
	outcome, err := skilleval.RunSmoke(ctx, leaf, cli.Runtime, *token)
	return reportSmoke(err, outcome)
}

func runSmokeVerdict(ctx context.Context, args []string) int {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitFailure
	}

	flags := flag.NewFlagSet("smoke-verdict", flag.ExitOnError)
	heartbeatBin := flags.String("heartbeat", filepath.Join(config.SmineBinDir(runtime.GOOS, home), "smine-heartbeat"), "heartbeat binary the verdict reads the session's enforced runs from")
	_ = flags.Parse(args)
	if flags.NArg() != smokeVerdictArgCount {
		fmt.Fprintln(os.Stderr, usage)
		return exitUsage
	}

	cli := newHeartbeatCli(*heartbeatBin, home)
	leaf := flags.Arg(0)
	sessionId := flags.Arg(1)
	outcome, err := skilleval.JudgeSmokeSession(ctx, leaf, cli.Runtime, sessionId)
	return reportSmoke(err, outcome)
}

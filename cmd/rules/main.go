// Command rules validates the ACTION/RULE/FACT entry files and generates the untracked context file (context/context.json).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/gitwt"
	"github.com/kevinhorst/smine/internal/reach"
	"github.com/kevinhorst/smine/internal/reviewscope"
	"github.com/kevinhorst/smine/internal/shell"
	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skills"
	"github.com/kevinhorst/smine/internal/trash"
)

const (
	exitClean      = 0
	exitViolations = 1
	exitError      = 2

	defaultLanes       = 2
	defaultWalkCeiling = 10
	voidFlagCount      = 2 // --round-dir and --void, nothing else

	ledgerUsage = "usage: rules ledger --known <file> (--round-dir <dir> --coverage <file> [--autofix-level minor] [--ignore-level none] [--done-after n] [--schema <file>] [--plan-dir <dir>] | --round n --fix-state applied|fixed --ids a,b) | rules ledger --reject <id> --reason <text> --rejected <file> --round-dir <dir> | rules ledger --round-dir <dir> --void"
)

// checklistDocument is checklist-<direction>.json as the checklist agent writes it.
type checklistDocument struct {
	Groups []contextdocs.WalkGroup `json:"groups"`
	Head   string                  `json:"head"`
	Shape  string                  `json:"shape"`
}

// laneFlags are the lanes subcommand's parsed flags.
type laneFlags struct {
	Base         string
	Ceiling      int
	Checklists   map[string]string
	ChunksPath   string
	CoveragePath string
	DeployedDir  string
	Directions   []string
	Excludes     []string
	Head         string
	Lanes        int
	MapPath      string
	Paths        []string
	RoundDir     string
	SourceDir    string
	WorktreeRoot string
}

type voidReport struct {
	Void       bool `json:"void"`
	VoidRounds int  `json:"void_rounds"`
}

// command is one subcommand of the binary; the table is the dispatch and the help text.
type command struct {
	name    string
	run     func(args []string) int
	summary string
}

func main() {
	os.Exit(dispatch(os.Args[1:], os.Stdout, os.Stderr))
}

// commands lists every subcommand in help order; each prints its own flag usage on -h.
func commands() []*command {
	return []*command{
		{name: "interfaces", run: runInterfaces, summary: "print the interface versions this build implements"},
		{name: "validate", run: runValidate, summary: "check a context tree, one line per contract violation"},
		{name: "generate", run: runGenerate, summary: "render context.json from the context tree and aspects.json"},
		{name: "pack", run: runPack, summary: "write the always-reach entries to the global always-context pack"},
		{name: "entries", run: runEntries, summary: "list the entries carrying a marker tag"},
		{name: "render-skill", run: runRenderSkill, summary: "print a skill dir's SKILL.md, deployed form or help-test body"},
		{name: "walk", run: runWalk, summary: "print a lane's checklist for one review direction"},
		{name: "pairs", run: runPairs, summary: "print a review range's test pairing"},
		{name: "base", run: runBase, summary: "print the review base commit and its source"},
		{name: "audit-crates", run: runAuditCratesStdout, summary: "print the Rust crates whose files changed on this branch"},
		{name: "lanes", run: runLanes, summary: "write one round's lane manifests and print the lane plan"},
		{name: "intake", run: runIntake, summary: "print the review-config analysis of a range"},
		{name: "provision", run: runProvision, summary: "create one review worktree for a railroad-review worker"},
		{name: "cleanup", run: runCleanup, summary: "salvage and remove every worktree and branch of one review run"},
		{name: "reset-round", run: runResetRound, summary: "salvage one failed review round before its re-dispatch"},
		{name: "ledger", run: runLedger, summary: "update the known and coverage ledgers from a round's review.json"},
		{name: "probe-replay", run: runProbeReplay, summary: "replay the persisted probes of rounds against the post-fix head"},
		{name: "buildtest", run: runBuildTest, summary: "build and test one commit in a disposable worktree"},
		{name: "prune", run: runPrune, summary: "reduce a finished review's artifact dir to its keep list"},
		{name: "snapshot", run: runSnapshot, summary: "commit the working tree on top of HEAD through a temporary index"},
		{name: "present", run: runPresent, summary: "print the gate text or a plan's review section from review.json"},
		{name: "demo-value", run: runDemoValue, summary: "print the heartbeat-demo producer value"},
		{name: "demo-close", run: runDemoClose, summary: "write the heartbeat-demo closing marker"},
		{name: "workflow-gen", run: runWorkflowGen, summary: "regenerate or check the stage workflow files"},
		{name: "plan-sections", run: runPlanSections, summary: "check a plan file's sections"},
		{name: "plan-claims", run: runPlanClaims, summary: "print a plan's refute ledger from its Assumptions table"},
		{name: "plan-display", run: runPlanDisplay, summary: "write a plan's display copy"},
		{name: "capture-ui", run: runCaptureUi, summary: "write a PNG of a running UI"},
		{name: "merge-branch", run: runMergeBranch, summary: "create or clean up a merge-resolve work branch"},
		{name: "merge-skillcue", run: runMergeSkillcue, summary: "git merge driver for skill model files (<leaf>.cue)"},
		{name: "store-check", run: runStoreCheck, summary: "validate the plans-archive backlog store"},
		{name: "store-root", run: runStoreRoot, summary: "print the store root of the repo"},
		{name: "settings", run: runSettings, summary: "list, get or set a hook or heartbeat setting of this machine's install record"},
		{name: "settings-defaults", run: runSettingsDefaults, summary: "regenerate or check the heartbeat crate's config defaults fixture"},
		{name: "fragment-check", run: runFragmentCheck, summary: "fail a settings fragment hook group changed against the branch base without a smineVersion bump"},
		{name: "task-check", run: runTaskCheck, summary: "validate smine change task files"},
		{name: "title", run: runTitle, summary: "confirm the session-title state a gate step sets for the turn end"},
	}
}

// dispatch runs the named subcommand; -h, --help and help print the command table and exit clean.
func dispatch(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return exitError
	}

	name := args[0]
	if isHelpWord(name) {
		printHelp(stdout)
		return exitClean
	}
	for _, cmd := range commands() {
		if cmd.name == name {
			return cmd.run(args[1:])
		}
	}

	fmt.Fprintf(stderr, "rules: unknown command: %s\n", name)
	printUsage(stderr)
	return exitError
}

func isHelpWord(word string) bool {
	return word == "-h" || word == "--help" || word == "help"
}

// parseExit maps a ContinueOnError flag set's parse error: -h is a clean exit, anything else a usage error.
func parseExit(err error) int {
	if errors.Is(err, flag.ErrHelp) {
		return exitClean
	}
	fmt.Fprintln(os.Stderr, "rules:", err)
	return exitError
}

func printHelp(out io.Writer) {
	printUsage(out)
	fmt.Fprintln(out)
	for _, cmd := range commands() {
		fmt.Fprintf(out, "  %-16s %s\n", cmd.name, cmd.summary)
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "rules <command> -h prints the command's flags.")
}

func printUsage(out io.Writer) {
	names := make([]string, 0)
	for _, cmd := range commands() {
		names = append(names, cmd.name)
	}
	fmt.Fprintf(out, "usage: rules <%s> [flags] | rules -h\n", strings.Join(names, "|"))
}

func runAuditCratesStdout(args []string) int {
	return runAuditCrates(args, os.Stdout)
}

func runValidate(args []string) int {
	flags := flag.NewFlagSet("validate", flag.ExitOnError)
	deployedDir := flags.String("deployed", "", "deployed context root; baseline files recognized by their synced header")
	sourceDir := flags.String("dir", "context", "source context directory (ignored with --deployed)")
	flags.Parse(args)

	contextDir, detectOrigin := *sourceDir, false
	if *deployedDir != "" {
		contextDir, detectOrigin = *deployedDir, true
	}

	set, err := contextdocs.ParseContext(contextDir, detectOrigin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	aspects, err := contextdocs.LoadAspects(*sourceDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	violations := contextdocs.ValidateRules(set, aspects, ".")
	violations = append(violations, contextdocs.ValidateBudgets(contextDir, set)...)
	for _, violation := range violations {
		fmt.Println(violation)
	}
	if len(violations) > 0 {
		fmt.Fprintf(os.Stderr, "rules: %d violation(s) in %s\n", len(violations), contextDir)
		return exitViolations
	}

	fmt.Printf("rules: %d entries, %d tombstones OK in %s\n", len(set.Entries), len(set.Tombstones), contextDir)
	return exitClean
}

func runPack(args []string) int {
	flags := flag.NewFlagSet("pack", flag.ExitOnError)
	sourceDir := flags.String("dir", "context", "context source directory")
	flags.Parse(args)

	packFile, err := contextdocs.AlwaysPackFile()
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	stateDir, err := config.StateDir("")
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	isWritten, err := contextdocs.WriteAlwaysPack(*sourceDir, packFile, trash.New(stateDir, 0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	if isWritten {
		fmt.Println("always pack ->", packFile)
		return exitClean
	}
	fmt.Println("always pack: no always entries,", packFile, "absent")
	return exitClean
}

func runEntries(args []string) int {
	flags := flag.NewFlagSet("entries", flag.ExitOnError)
	marker := flags.String("marker", "", "marker tag to select (e.g. DoD); required")
	deployedDir := flags.String("deployed", "", "deployed context root; baseline files recognized by their synced header")
	sourceDir := flags.String("dir", "", "source context directory; absent = context/ or the deployed dir holding context.json")
	flags.Parse(args)
	if *marker == "" {
		fmt.Fprintln(os.Stderr, "usage: rules entries --marker <tag> [--deployed <dir>] [--dir <context-dir>]")
		return exitError
	}

	contextDir, detectOrigin, err := contextDirOf(*deployedDir, *sourceDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	set, err := contextdocs.ParseContext(contextDir, detectOrigin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	count := 0
	for _, entry := range set.Entries {
		for _, tag := range entry.Markers {
			if tag == *marker {
				fmt.Printf("%s — %s\n", entry.Id, entry.Content.Statement)
				count++
				break
			}
		}
	}
	if count == 0 {
		fmt.Fprintf(os.Stderr, "rules: no entries with marker [%s] in %s\n", *marker, contextDir)
		return exitViolations
	}
	return exitClean
}

func runWalk(args []string) int {
	flags := flag.NewFlagSet("walk", flag.ExitOnError)
	ceiling := flags.Int("ceiling", defaultWalkCeiling, "group ceiling — most entries per checklist group")
	deployedDir := flags.String("deployed", "", "deployed context root holding context.json (overrides --dir)")
	direction := flags.String("direction", "", "railroad-review direction (e.g. code-style); required")
	filesCsv := flags.String("files", "", "comma-separated lane files, repo-relative — select the language guides")
	mapPath := flags.String("map", "", "the skill's context-map.json; required")
	sourceDir := flags.String("dir", "", "source context directory; absent = context/ or the deployed dir holding context.json")
	flags.Parse(args)
	if *direction == "" || *mapPath == "" {
		fmt.Fprintln(os.Stderr, "usage: rules walk --direction <d> --map <context-map.json> [--files a.go,b.go] [--ceiling 10] [--deployed <dir>] [--dir <context-dir>]")
		return exitError
	}

	contextDir, _, err := contextDirOf(*deployedDir, *sourceDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	set, err := loadIndexSet(contextDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	laneMap, err := contextdocs.LoadLaneMap(*mapPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	request := &contextdocs.WalkRequest{
		Ceiling:   *ceiling,
		Direction: *direction,
		Files:     splitCsv(*filesCsv),
		Tokens:    laneMap.Lanes[*direction],
	}
	walk, err := contextdocs.BuildWalk(set, request)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	if code := printJson(walk); code != exitClean {
		return code
	}
	if len(walk.Groups) == 0 {
		fmt.Fprintf(os.Stderr, "rules: direction %s selects no entries in %s\n", *direction, contextDir)
		return exitViolations
	}
	return exitClean
}

func runPairs(args []string) int {
	flags := flag.NewFlagSet("pairs", flag.ExitOnError)
	base := flags.String("base", "", "review base commit; required")
	excludeCsv := flags.String("exclude", "", "comma-separated pathspecs excluded from the diff")
	head := flags.String("head", "", "review head commit; required")
	flags.Parse(args)
	if *base == "" || *head == "" {
		fmt.Fprintln(os.Stderr, "usage: rules pairs --base <commit> --head <commit> [--exclude x,y]")
		return exitError
	}

	ctx := context.Background()
	diffArgs := []string{"diff", "--name-only", *base, *head, "--", "."}
	for _, exclude := range splitCsv(*excludeCsv) {
		diffArgs = append(diffArgs, ":(exclude)"+exclude)
	}

	changed, err := shell.Run(ctx, ".", "git", diffArgs...)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	tracked, err := shell.Run(ctx, ".", "git", "ls-tree", "-r", "--name-only", *head)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	pairs := reviewscope.BuildTestPairs(splitLines(changed), splitLines(tracked))
	return printJson(pairs)
}

func runBase(args []string) int {
	flags := flag.NewFlagSet("base", flag.ExitOnError)
	base := flags.String("base", "", "the invocation's scope bracket base; absent = derive")
	head := flags.String("head", "", "review head commit; required")
	mainline := flags.String("mainline", "", "mainline branch (e.g. main); required")
	flags.Parse(args)
	if *head == "" || *mainline == "" {
		fmt.Fprintln(os.Stderr, "usage: rules base --head <commit> --mainline <branch> [--base <ref>]")
		return exitError
	}

	derived, err := reviewscope.DeriveBase(context.Background(), ".", *head, *mainline, *base)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	return printJson(derived)
}

func runLanes(args []string) int {
	flags := flag.NewFlagSet("lanes", flag.ExitOnError)
	base := flags.String("base", "", "review base commit; required")
	ceiling := flags.Int("ceiling", defaultWalkCeiling, "walk group ceiling")
	checklists := map[string]string{}
	flags.Func("checklist", "judgment-direction checklist, <direction>=<path>; repeatable", func(value string) error {
		return parseChecklist(value, checklists)
	})
	chunksPath := flags.String("chunks", "", "dispatcher partition, [{name, files}] JSON; absent = unchunked")
	coveragePath := flags.String("coverage", "", "coverage ledger JSON; absent = round 1")
	deployedDir := flags.String("deployed", "", "deployed context root")
	directionsCsv := flags.String("directions", "", "comma-separated directions; required")
	excludeCsv := flags.String("exclude", "", "comma-separated pathspecs excluded from the diff")
	head := flags.String("head", "", "review head commit; required")
	knownPath := flags.String("known", "", "known findings the caller hands in; first in the round's known list")
	lanes := flags.Int("lanes", defaultLanes, "lanes per direction")
	defaultMap := filepath.Join(skills.DefaultHomePath(), "railroad-review", "context-map.json")
	mapPath := flags.String("map", defaultMap, "the skill's context-map.json; default the deployed railroad-review copy")
	pathsCsv := flags.String("paths", "", "comma-separated pathspecs narrowing the diff")
	roundDir := flags.String("round-dir", "", "the round's artifact directory; required")
	sourceDir := flags.String("dir", "", "source context directory; absent = context/ or the deployed dir holding context.json")
	worktreeRoot := flags.String("worktree-root", "", "explicit worktree root; empty = harness isolation")
	flags.Parse(args)
	isMissingRequired := *base == "" || *head == "" || *directionsCsv == "" || *roundDir == ""
	if isMissingRequired {
		fmt.Fprintln(os.Stderr, "usage: rules lanes --round-dir <dir> --directions a,b --base <commit> --head <commit> [--map <context-map.json>] [--lanes 2] [--paths p,q] [--exclude x,y] [--chunks <file>] [--coverage <file>] [--known <file>] [--checklist <direction>=<path>]... [--ceiling 10] [--deployed <dir>] [--worktree-root <dir>]")
		return exitError
	}
	if round, err := reviewscope.RoundNumber(*roundDir); err == nil && round > 1 && *coveragePath == "" {
		fmt.Fprintf(os.Stderr, "rules: %s is a later round: --coverage <artifactsDir>/coverage.json is required\n", *roundDir)
		return exitError
	}

	parsed := &laneFlags{
		Base:         *base,
		Ceiling:      *ceiling,
		Checklists:   checklists,
		ChunksPath:   *chunksPath,
		CoveragePath: *coveragePath,
		DeployedDir:  *deployedDir,
		Directions:   splitCsv(*directionsCsv),
		Excludes:     splitCsv(*excludeCsv),
		Head:         *head,
		Lanes:        *lanes,
		MapPath:      *mapPath,
		Paths:        splitCsv(*pathsCsv),
		RoundDir:     *roundDir,
		SourceDir:    *sourceDir,
		WorktreeRoot: *worktreeRoot,
	}
	request, err := laneRequest(parsed)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	plan, err := reviewscope.PlanLanes(request)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	if err := plan.WriteKnownView(*knownPath, *roundDir); err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	if code := printJson(plan); code != exitClean {
		return code
	}
	if len(plan.MissingContext) > 0 {
		fmt.Fprintf(os.Stderr, "rules: directions %s select no walk entries\n", strings.Join(plan.MissingContext, ", "))
		return exitViolations
	}
	return exitClean
}

func runProvision(args []string) int {
	flags := flag.NewFlagSet("provision", flag.ExitOnError)
	branch := flags.String("branch", "", "the worker's claude-review/* branch; required")
	head := flags.String("head", "", "review head commit; required")
	overlay := flags.String("overlay", "", "review-context overlay patch; absent = none")
	prebuiltCsv := flags.String("prebuilt", "", "comma-separated repo-relative paths linked from --repo into the worktree")
	repo := flags.String("repo", "", "target repository; required")
	root := flags.String("root", "", "worktree root; required")
	flags.Parse(args)
	isMissingRequired := *branch == "" || *head == "" || *repo == "" || *root == "" || flags.NArg() != 1
	if isMissingRequired {
		fmt.Fprintln(os.Stderr, "usage: rules provision --repo <repo> --root <dir> --head <commit> --branch <branch> [--overlay <patch>] [--prebuilt p,q] <slug>")
		return exitError
	}

	request := &gitwt.ProvisionRequest{
		Branch:   *branch,
		Head:     *head,
		Overlay:  *overlay,
		Prebuilt: splitCsv(*prebuiltCsv),
		Repo:     *repo,
		Root:     *root,
		Slug:     flags.Arg(0),
	}
	provisioned, err := gitwt.Provision(context.Background(), request)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitViolations
	}
	return printJson(provisioned)
}

func runLedger(args []string) int {
	flags := flag.NewFlagSet("ledger", flag.ContinueOnError)
	autofixLevel := flags.String("autofix-level", "minor", "highest severity the fix workflow fixes")
	coveragePath := flags.String("coverage", "", "artifactsDir/coverage.json; required")
	doneAfter := flags.Int("done-after", 0, "uniform done-after count; 0 = per-direction defaults")
	fixState := flags.String("fix-state", "", "state the --ids rows of --round move to: applied | fixed")
	idsCsv := flags.String("ids", "", "comma-separated finding ids (with --fix-state)")
	ignoreLevel := flags.String("ignore-level", "none", "severity at or below which a new row is ledgered ignored: never fixed, hidden at the gate")
	knownPath := flags.String("known", "", "artifactsDir/known.json; required")
	planDir := flags.String("plan-dir", "", "plan dir receiving reviews/round-<n>/review.json and probes/")
	reason := flags.String("reason", "", "the human's rejection reason (with --reject)")
	reject := flags.String("reject", "", "finding id the human rejected: appended to --rejected")
	rejectedPath := flags.String("rejected", "", "the plan dir's reviews/rejected.json (with --reject)")
	round := flags.Int("round", 0, "round of the --ids rows (with --fix-state)")
	roundDir := flags.String("round-dir", "", "the round's artifact directory; required unless --fix-state")
	defaultSchema := filepath.Join(skills.DefaultHomePath(), "railroad-review", "schemas", "consolidation.json")
	schemaPath := flags.String("schema", defaultSchema, "consolidation.json review.json is validated against first; default the deployed railroad-review copy")
	isVoid := flags.Bool("void", false, "record a round without a handoff: review.json must be absent; writes and validates nothing")
	if err := flags.Parse(args); err != nil {
		return parseExit(err)
	}

	if *isVoid {
		return runVoid(flags.NFlag(), *roundDir)
	}
	if *reject != "" {
		return runReject(*reject, *reason, *rejectedPath, *roundDir)
	}
	if *knownPath == "" {
		fmt.Fprintln(os.Stderr, ledgerUsage)
		return exitError
	}

	if *fixState != "" {
		request := &reviewscope.FixStateRequest{
			Ids:       splitCsv(*idsCsv),
			KnownPath: *knownPath,
			Round:     *round,
			State:     reviewscope.FixState(*fixState),
		}
		return runFixState(request)
	}

	request := &reviewscope.LedgerRequest{AutofixLevel: *autofixLevel, CoveragePath: *coveragePath, DoneAfter: *doneAfter, IgnoreLevel: *ignoreLevel, KnownPath: *knownPath, PlanDir: *planDir, RoundDir: *roundDir, SchemaPath: *schemaPath}
	result, err := reviewscope.UpdateLedger(request)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	if code := printJson(result); code != exitClean {
		return code
	}
	if len(result.Oscillating) > 0 {
		fmt.Fprintf(os.Stderr, "rules: findings confirmed again after their fix landed: %s\n", strings.Join(result.Oscillating, ", "))
	}
	return exitClean
}

func runFixState(request *reviewscope.FixStateRequest) int {
	isIncomplete := request.Round <= 0 || len(request.Ids) == 0
	if isIncomplete {
		fmt.Fprintln(os.Stderr, ledgerUsage)
		return exitError
	}

	if err := reviewscope.SetFixState(request); err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	return exitClean
}

// runReject appends one human-rejected finding to the rejection ledger.
func runReject(findingId, reason, rejectedPath, roundDir string) int {
	if reason == "" || rejectedPath == "" || roundDir == "" {
		fmt.Fprintln(os.Stderr, "usage: rules ledger --reject <id> --reason <text> --rejected <file> --round-dir <dir>")
		return exitError
	}

	round, err := reviewscope.RoundNumber(roundDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	rejection := reviewscope.Rejection{FindingId: findingId, Reason: reason, Round: round}
	if err := reviewscope.AppendRejection(rejectedPath, rejection); err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	return exitClean
}

func runVoid(flagCount int, roundDir string) int {
	if flagCount != voidFlagCount || roundDir == "" {
		fmt.Fprintln(os.Stderr, "usage: rules ledger --round-dir <dir> --void")
		return exitError
	}

	if err := reviewscope.VoidRound(roundDir); err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	voidRounds, err := reviewscope.VoidRounds(roundDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	report := &voidReport{Void: true, VoidRounds: voidRounds}
	return printJson(report)
}

func runProbeReplay(args []string) int {
	flags := flag.NewFlagSet("probe-replay", flag.ExitOnError)
	acdsl := flags.String("acdsl", "", "acdsl program, one command line (spaces split words, double quotes group them); absent = <tree>/bin/acdsl")
	branch := flags.String("branch", "", "the replay worktree's claude-review/* branch; required")
	build := flags.String("build", "", "build command, one command per line; required when a round holds a probe")
	head := flags.String("head", "", "post-fix head commit; required")
	repo := flags.String("repo", "", "target repository; required")
	root := flags.String("root", "", "worktree root; required")
	roundsCsv := flags.String("rounds", "", "comma-separated round dirs 1..r; required")
	flags.Parse(args)
	rounds := splitCsv(*roundsCsv)
	isMissingRequired := *branch == "" || *head == "" || *repo == "" || *root == "" || len(rounds) == 0
	if isMissingRequired {
		fmt.Fprintln(os.Stderr, "usage: rules probe-replay --repo <repo> --head <commit> --root <dir> --branch <branch> --rounds <dir,dir> [--acdsl <program>] [--build <cmd>]")
		return exitError
	}

	request := &reviewscope.ProbeRequest{Acdsl: *acdsl, Branch: *branch, Build: *build, Head: *head, Repo: *repo, Root: *root, RoundDirs: rounds}
	replay, err := reviewscope.ReplayProbes(context.Background(), request)
	if replay == nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	if code := printJson(replay); code != exitClean {
		return code
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	return exitClean
}

func runSnapshot(args []string) int {
	flags := flag.NewFlagSet("snapshot", flag.ExitOnError)
	scope := flags.String("scope", "", "wip or all — both snapshot staged, unstaged and untracked work; the caller's review base differs (HEAD vs fork point); required")
	flags.Parse(args)
	if *scope == "" {
		fmt.Fprintln(os.Stderr, "usage: rules snapshot --scope wip|all")
		return exitError
	}

	result, err := reviewscope.Snapshot(context.Background(), ".", *scope)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	return printJson(result)
}

func runPresent(args []string) int {
	flags := flag.NewFlagSet("present", flag.ExitOnError)
	ignoreLevel := flags.String("ignore-level", "none", "severity at or below which rows leave the findings table into the ignored line")
	review := flags.String("review", "", "the round's review.json; required")
	section := flags.String("section", "", "plan-section heading (<date> — <base>...<head> — <directions, round>); absent = gate text")
	flags.Parse(args)
	if *review == "" {
		fmt.Fprintln(os.Stderr, "usage: rules present --review <review.json> [--ignore-level none] [--section <heading>]")
		return exitError
	}

	text, err := reviewscope.Present(reviewscope.PresentRequest{Heading: *section, IgnoreLevel: *ignoreLevel, ReviewPath: *review})
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	fmt.Print(text)
	return exitClean
}

func runBuildTest(args []string) int {
	flags := flag.NewFlagSet("buildtest", flag.ExitOnError)
	branch := flags.String("branch", "", "the worktree's claude-review/* branch; required")
	build := flags.String("build", "", "build command, one command per line; required unless --skip")
	head := flags.String("head", "", "commit to build and test; required")
	overlay := flags.String("overlay", "", "review-context overlay patch; absent = none")
	repo := flags.String("repo", "", "target repository; required")
	root := flags.String("root", "", "worktree root; required")
	salvage := flags.String("salvage", "", "salvage directory; required")
	isWaived := flags.Bool("skip", false, "run nothing: the review waived this health check, both steps answer skipped")
	test := flags.String("test", "", "test command, one command per line; required unless --skip")
	flags.Parse(args)
	isMissingRequired := *branch == "" || *head == "" || *repo == "" || *root == "" || *salvage == ""
	if isMissingRequired {
		fmt.Fprintln(os.Stderr, "usage: rules buildtest --repo <repo> --head <commit> --root <dir> --branch <branch> --salvage <dir> [--overlay <patch>] [--build <cmd>] [--test <cmd>] [--skip]")
		return exitError
	}

	request := &reviewscope.BuildTestRequest{Branch: *branch, Build: *build, Head: *head, IsWaived: *isWaived, Overlay: *overlay, Repo: *repo, Root: *root, Salvage: *salvage, Test: *test}
	result, err := reviewscope.RunBuildTest(context.Background(), request)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	return printJson(result)
}

func runPrune(args []string) int {
	flags := flag.NewFlagSet("prune", flag.ContinueOnError)
	artifactsDir := flags.String("artifacts", "", "the review's artifact directory; required")
	isVerbose := flags.Bool("verbose", false, "also keep each round's lane reports and verdicts/*.json")
	if err := flags.Parse(args); err != nil {
		return parseExit(err)
	}

	if *artifactsDir == "" {
		fmt.Fprintln(os.Stderr, "usage: rules prune --artifacts <dir> [--verbose]")
		return exitError
	}

	stateDir, err := config.StateDir("")
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	request := &reviewscope.PruneRequest{ArtifactsDir: *artifactsDir, IsVerbose: *isVerbose, Trash: trash.New(stateDir, 0)}
	result, err := reviewscope.Prune(request)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	return printJson(result)
}

// printJson prints value as indented JSON on stdout.
func printJson(value any) int {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	fmt.Println(string(data))
	return exitClean
}

// splitCsv splits a comma-separated flag value, dropping empty items.
func splitCsv(csv string) []string {
	var items []string
	for item := range strings.SplitSeq(csv, ",") {
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

// splitLines splits command output into lines, dropping empty ones.
func splitLines(output string) []string {
	lines := make([]string, 0)
	for line := range strings.SplitSeq(output, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// contextDirOf honors --deployed first, then the shared resolution over --dir, context/ and the deployed dir.
func contextDirOf(deployedDir, sourceDir string) (string, bool, error) {
	if deployedDir != "" {
		return deployedDir, true, nil
	}
	return contextdocs.ResolveContextDir(sourceDir, os.Getenv(contextdocs.ContextDirDefaultKey))
}

func laneRequest(flags *laneFlags) (*reviewscope.LaneRequest, error) {
	contextDir, _, err := contextDirOf(flags.DeployedDir, flags.SourceDir)
	if err != nil {
		return nil, fmt.Errorf("laneRequest: %w", err)
	}

	ctx := context.Background()
	diffArgs := []string{"diff", "--name-only", flags.Base, flags.Head, "--"}
	diffArgs = append(diffArgs, flags.Paths...)
	if len(flags.Paths) == 0 {
		diffArgs = append(diffArgs, ".")
	}
	for _, exclude := range flags.Excludes {
		diffArgs = append(diffArgs, ":(exclude)"+exclude)
	}

	changed, err := shell.Run(ctx, ".", "git", diffArgs...)
	if err != nil {
		return nil, fmt.Errorf("laneRequest: %w", err)
	}

	tracked, err := shell.Run(ctx, ".", "git", "ls-tree", "-r", "--name-only", flags.Head)
	if err != nil {
		return nil, fmt.Errorf("laneRequest: %w", err)
	}

	set, err := loadIndexSet(contextDir)
	if err != nil {
		return nil, fmt.Errorf("laneRequest: %w", err)
	}

	laneMap, err := contextdocs.LoadLaneMap(flags.MapPath)
	if err != nil {
		return nil, fmt.Errorf("laneRequest: %w", err)
	}

	diffFiles := splitLines(changed)
	if len(diffFiles) == 0 {
		return nil, fmt.Errorf("laneRequest: Empty diff between base %s and head %s: nothing to review", flags.Base, flags.Head)
	}

	headCommit, err := shell.Run(ctx, ".", "git", "rev-parse", "--verify", flags.Head+"^{commit}")
	if err != nil {
		return nil, fmt.Errorf("laneRequest: %w", err)
	}

	request := &reviewscope.LaneRequest{
		Ceiling:      flags.Ceiling,
		Checklists:   map[string]*contextdocs.Walk{},
		DiffFiles:    diffFiles,
		Directions:   flags.Directions,
		Done:         map[string][]string{},
		Head:         strings.TrimSpace(headCommit),
		Lanes:        flags.Lanes,
		Pairs:        reviewscope.BuildTestPairs(diffFiles, splitLines(tracked)),
		RoundDir:     flags.RoundDir,
		RuleSet:      set,
		Tokens:       laneMap.Lanes,
		WorktreeRoot: flags.WorktreeRoot,
	}
	if err := loadLaneExtras(flags, request); err != nil {
		return nil, err
	}
	return request, nil
}

// loadIndexSet reads the walk's rule set from the context dir's index — the only context a deployed repo or an overlay carries.
func loadIndexSet(contextDir string) (contextdocs.RuleSet, error) {
	index, err := contextdocs.LoadContextFile(filepath.Join(contextDir, contextdocs.ContextFileName))
	if err != nil {
		return contextdocs.RuleSet{}, fmt.Errorf("loadIndexSet: %w", err)
	}
	return index.RuleSet(), nil
}

func loadLaneExtras(flags *laneFlags, request *reviewscope.LaneRequest) error {
	if flags.ChunksPath != "" {
		if err := readJsonFile(flags.ChunksPath, &request.Chunks); err != nil {
			return err
		}
	}

	if flags.CoveragePath != "" {
		ledger := reviewscope.CoverageLedger{}
		if err := readJsonFile(flags.CoveragePath, &ledger); err != nil {
			return err
		}
		request.Done = ledger.DoneFiles()
	}

	for direction, checklistPath := range flags.Checklists {
		walk, err := readChecklist(direction, request.Head, checklistPath)
		if err != nil {
			return err
		}
		request.Checklists[direction] = walk
	}
	return nil
}

func parseChecklist(value string, checklists map[string]string) error {
	direction, path, isSplit := strings.Cut(value, "=")
	if !isSplit || path == "" {
		return fmt.Errorf("parseChecklist: %q is not <direction>=<path>", value)
	}

	isJudgmentDirection := direction == "contracts" || direction == "correctness"
	if !isJudgmentDirection {
		return fmt.Errorf("parseChecklist: %q is no judgment direction (contracts, correctness)", direction)
	}

	if _, isSet := checklists[direction]; isSet {
		return fmt.Errorf("parseChecklist: Checklist for %s given twice", direction)
	}

	checklists[direction] = path
	return nil
}

// readChecklist refuses a contracts checklist built for another head: the diff it was extracted from moved.
func readChecklist(direction, head, path string) (*contextdocs.Walk, error) {
	var document checklistDocument
	if err := readJsonFile(path, &document); err != nil {
		return nil, err
	}

	isStale := direction == "contracts" && document.Head != head
	if isStale {
		return nil, fmt.Errorf("readChecklist: %s is for head %q, not %s — the head moved, re-extract the contracts checklist", path, document.Head, head)
	}
	walk := &contextdocs.Walk{Groups: document.Groups, Shape: document.Shape}
	return walk, nil
}

// readJsonFile decodes one JSON file into target.
func readJsonFile(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("readJsonFile: %w", err)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("readJsonFile: %s: %w", path, err)
	}

	return nil
}

func runGenerate(args []string) int {
	flags := flag.NewFlagSet("generate", flag.ExitOnError)
	sourceDir := flags.String("dir", "context", "source context directory")
	outFile := flags.String("out", "context/context.json", "context file to write")
	flags.Parse(args)

	set, err := contextdocs.ParseContext(*sourceDir, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	aspects, err := contextdocs.LoadAspects(*sourceDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	violations := contextdocs.ValidateRules(set, aspects, ".")
	violations = append(violations, contextdocs.ValidateBudgets(*sourceDir, set)...)
	if len(violations) > 0 {
		for _, violation := range violations {
			fmt.Println(violation)
		}
		fmt.Fprintf(os.Stderr, "rules: refusing to generate from %d violation(s)\n", len(violations))
		return exitViolations
	}

	rendered, err := contextdocs.RenderContextJson(set, aspects, reach.ThisRepo)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	if err := os.WriteFile(*outFile, rendered, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}
	fmt.Printf("rules: wrote %s (%d entries, %d tombstones)\n", *outFile, len(set.Entries), len(set.Tombstones))
	return exitClean
}

func runRenderSkill(args []string) int {
	flags := flag.NewFlagSet("render-skill", flag.ExitOnError)
	disable := flags.String("disable", "", "comma-separated entry ids or trailing-* globs to strip (with --strip-formal)")
	helpTest := flags.Bool("help-test", false, "print the help-test.md body instead: how to run the skill's smoke and eval")
	stripFormal := flags.Bool("strip-formal", false, "print the deployed SKILL.md: no formal clauses")
	flags.Parse(args)
	hasStrayDisable := *disable != "" && !*stripFormal
	isMisused := flags.NArg() != 1 || hasStrayDisable
	if isMisused {
		fmt.Fprintln(os.Stderr, "usage: rules render-skill [--strip-formal [--disable id,glob*] | --help-test] <skill dir>")
		return exitError
	}
	dir := flags.Arg(0)
	skill, err := skillgraph.Load(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	isCanonical := !*helpTest && !*stripFormal
	if isCanonical {
		manifest, err := skillgraph.RenderManifest(skill)
		if err != nil {
			fmt.Fprintln(os.Stderr, "rules:", err)
			return exitError
		}

		fmt.Print(string(manifest))
		return exitClean
	}

	_, statErr := os.Stat(filepath.Join(dir, skillgraph.DirEval, skillgraph.FileEvalDefinition))
	hasEval := statErr == nil
	wanted := skillgraph.FileManifest
	if *helpTest {
		wanted = skillgraph.FileHelpTest
	}
	renderOptions := skillgraph.RenderOptions{Disable: splitCsv(*disable), HasEval: hasEval}
	files, err := skillgraph.RenderDeployed(renderOptions, skill)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	for _, file := range files {
		if file.Path == wanted {
			fmt.Print(string(file.Content))
		}
	}
	return exitClean
}

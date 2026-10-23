package routinerun

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/backlog"
	settings "github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/install"
	"github.com/kevinhorst/smine/internal/peek"
	"github.com/kevinhorst/smine/internal/presentation"
	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/sessions"
	"github.com/kevinhorst/smine/internal/skills"
)

// smine-nightly SMINE_* defaults.
const (
	defaultChangesCap   = 5
	defaultSessionsCap  = 30
	defaultWindowNights = 2
	publishFailedStatus = 1
)

const (
	pipelineStage      = "pipeline"
	runSummaryFileName = "run-complete.json"
	styleStampFileName = ".style-stamp"
)

// nightlyMiningStages are the stages between style and judge, in run order.
const nightlyMiningStages = "batch,memory,context,permissions,acdsl,problems,implement,consolidate"

// countPending is the run's pending-session probe; tests swap it, peek is absent there.
var countPending = (*nightlyRun).pendingCount

// nightlyOptions lets SMINE_*/ROUTINE_* env win over the presentation profile.
type nightlyOptions struct {
	agents         string
	changesCap     int
	isAutoMerge    bool
	isDevMode      bool
	isOrchestrated bool
	isSubagents    bool
	language       string
	sessionsCap    int
	windowNights   int
}

type nightlyRun struct {
	batchBranch string
	branch      string
	options     *nightlyOptions
	report      *reportWriter
	roster      []routines.RunReportRepo
	runner      *Runner
	sessionId   string // the dispatcher cell's session; an oversized change's Icebox row names it as its miner
}

func newNightlyRun(runner *Runner) *nightlyRun {
	options := loadNightlyOptions(presentation.Load(presentation.DefaultPath()))
	roster := runner.nightlyRoster()
	nightly := &nightlyRun{
		options: options,
		report:  newNightlyReport(runner, options, roster),
		roster:  roster,
		runner:  runner,
	}
	return nightly
}

// deploy applies the stored selection record and never changes its selection (concept D10).
func (n *nightlyRun) deploy(ctx context.Context, repoRoot string) string {
	var output strings.Builder
	env, err := install.NewEnv(repoRoot, &output)
	if err != nil {
		return "\ndeploy failed: " + err.Error()
	}

	record, _, err := install.Load(env)
	if err != nil {
		return "\ndeploy failed: " + err.Error()
	}

	var report strings.Builder
	if record.Skills.Mode == skills.SelectionModeNone {
		report.WriteString("\nskills: none")
	}

	results, err := install.Apply(ctx, env, record)
	if err == nil && !install.Failed(results) {
		return report.String()
	}
	log.Printf("nightlyRun.deploy: Deploy failed: %v\n%s%s", err, install.Summary(results), output.String())
	for _, result := range results {
		if result.Err != nil {
			fmt.Fprintf(&report, "\ndeploy failed: %s: %v", result.Component, result.Err)
		}
	}
	if err != nil {
		report.WriteString("\ndeploy failed: " + err.Error())
	}
	return report.String()
}

// dispatch fails the run with a rejected verdict on a missing or invalid summary.
func (n *nightlyRun) dispatch(ctx context.Context, worktreePath string) (*runSummary, int) {
	mcpConfig, err := SmineMcpConfig(settings.DefaultClaudeJsonPath())
	if err != nil {
		log.Printf("nightlyRun.dispatch: %v", err)
		return newRejectedSummary("no smine peek config: " + err.Error()), exitConfig
	}

	marker := filepath.Join(n.runner.config.SteerDir, runSummaryFileName)
	cell := &stageCell{
		complete:   CompleteMarker,
		effort:     effortHigh,
		markerPath: marker,
		mcpConfig:  mcpConfig,
		prompt:     n.pipelinePrompt(n.isStyleDue(ctx), marker),
		skill:      n.runner.manifest.Skill(),
		stage:      pipelineStage,
		timeout:    n.runner.config.RunTimeout,
	}
	envelope, status := n.runner.runStage(ctx, worktreePath, cell)
	n.report.addStage(envelope, cell.stage, status)
	n.sessionId = envelope.SessionId

	summary, err := loadRunSummary(marker)
	if err != nil {
		log.Printf("nightlyRun.dispatch: %v", err)
		return newRejectedSummary("no valid run summary: " + err.Error()), firstFailure(status, exitSoftware)
	}

	n.report.addSummaryStages(summary, envelope.SessionId)
	n.report.setChanges(summary.reportChanges())
	if summary.isStageOk(styleStageName) {
		n.writeStyleStamp()
		if err := PublishStyleProfile(StyleStagingPath(worktreePath), n.runner.worktree.trash); err != nil {
			log.Printf("nightlyRun.dispatch: Style publish failed: %v", err)
		}
	}
	if len(summary.failedStages()) > 0 {
		status = firstFailure(status, exitSoftware)
	}
	return summary, status
}

func (n *nightlyRun) isStyleDue(ctx context.Context) bool {
	stamp := readTrimmed(filepath.Join(n.runner.dir, styleStampFileName))
	if stamp == "" {
		return true
	}

	since := "--since=@" + stamp
	planChanges, err := git(
		ctx,
		n.runner.repoRoot,
		"log",
		since,
		"--format=%h",
		"--",
		"plans",
	)
	if err == nil && planChanges != "" {
		return true
	}

	storeChanges, err := git(
		ctx,
		n.runner.storeRoot,
		"log",
		since,
		"--format=%h",
		"--",
		settings.StoreBacklogDir+"/backlog.json",
	)
	return err == nil && storeChanges != ""
}

// land never switches the main checkout's branch; agentwt refuses a dirty checked-out main itself.
func (n *nightlyRun) land(ctx context.Context, verdict *runVerdict) *routines.RunReportOrchestrate {
	outcome := &routines.RunReportOrchestrate{Report: fmt.Sprintf("accepted=%t: %s", verdict.Accepted, verdict.Reason)}
	switch {
	case !verdict.Accepted:
		outcome.Report += "\nkept: rejected by the judge"
	case !n.options.isAutoMerge:
		outcome.Report += "\nkept: auto-merge off"
	default:
		n.landReady(ctx, outcome)
	}
	outcome.MainSha, _ = git(ctx, n.runner.repoRoot, "rev-parse", mainBranch)
	return outcome
}

// landBatch picks the night's records commit onto main; a refused pick leaves the batch branch for the next night.
func (n *nightlyRun) landBatch(ctx context.Context, publish *routines.RunReportPublish) {
	if n.batchBranch == "" {
		publish.BatchLanding = "nothing to land"
		return
	}
	repoRoot := n.runner.repoRoot
	sha, err := git(ctx, repoRoot, "rev-parse", "refs/heads/"+n.batchBranch)
	if err != nil {
		publish.BatchLanding = "kept: " + err.Error()
		return
	}

	pick, err := repos.Pick(ctx, n.runner.config.AgentwtPath, mainBranch, repoRoot, sha)
	if err != nil {
		publish.BatchLanding = "kept: " + err.Error()
		return
	}
	if pick.Outcome != repos.PickOutcomePicked {
		publish.BatchLanding = "kept: " + pick.Outcome + ": " + pick.Reason
		return
	}

	publish.BatchLanding = "landed as " + pick.NewCommit
	n.runner.worktree.forceDeleteBranch(ctx, n.batchBranch)
}

// landReady picks every ready change onto a scratch branch and moves main only when all of them applied.
func (n *nightlyRun) landReady(ctx context.Context, outcome *routines.RunReportOrchestrate) {
	ready := n.readyShas(ctx)
	if len(ready) == 0 {
		outcome.Report += "\nkept: no ready change"
		return
	}

	repoRoot := n.runner.repoRoot
	scratch := strings.TrimSuffix(n.branch, repos.ProposalsSuffix) + landingSuffix
	if _, err := git(ctx, repoRoot, "branch", scratch, mainBranch); err != nil {
		outcome.Report += "\nkept: " + err.Error()
		return
	}
	defer n.runner.worktree.forceDeleteBranch(ctx, scratch)

	for _, sha := range ready {
		pick, err := repos.Pick(ctx, n.runner.config.AgentwtPath, scratch, repoRoot, sha)
		if err != nil {
			outcome.Report += "\nkept: " + err.Error()
			return
		}
		if pick.Outcome != repos.PickOutcomePicked {
			outcome.Report += fmt.Sprintf("\nkept: %s conflicts with main — kept on its night (%s)", sha, pick.Reason)
			return
		}
	}

	scratchSha, err := git(ctx, repoRoot, "rev-parse", "refs/heads/"+scratch)
	if err != nil {
		outcome.Report += "\nkept: " + err.Error()
		return
	}
	forward, err := repos.Forward(ctx, n.runner.config.AgentwtPath, mainBranch, repoRoot, scratchSha)
	if err != nil {
		outcome.Report += "\nkept: " + err.Error()
		return
	}
	if forward.Outcome != repos.ForwardOutcomeForwarded {
		outcome.Report += "\nkept: " + forward.Outcome + ": " + forward.Reason
		return
	}

	outcome.Merged = true
	outcome.Report += n.deploy(ctx, repoRoot)
}

// pendingCount asks peek for unmined sessions exactly as `sessions pending` does; a night with none is not cut.
func (n *nightlyRun) pendingCount(ctx context.Context) (int, error) {
	stateDir, err := settings.StateDir("")
	if err != nil {
		return 0, fmt.Errorf("nightlyRun.pendingCount: %w", err)
	}

	registryPath := filepath.Join(stateDir, settings.StateRegistryFile)
	root := filepath.Join(n.runner.repoRoot, ledgerDir)
	request, err := sessions.NewPendingRequest(n.options.isDevMode, n.options.sessionsCap, registryPath, n.runner.repoRoot, root)
	if err != nil {
		return 0, fmt.Errorf("nightlyRun.pendingCount: %w", err)
	}
	request.Since = time.Now().AddDate(0, 0, -peek.MiningWindowDays)

	command, args, err := settings.McpServer(settings.DefaultClaudeJsonPath(), peekServerName)
	if err != nil {
		return 0, fmt.Errorf("nightlyRun.pendingCount: %w", err)
	}
	args = append(slices.Clone(args), peek.WatchWindowArg(peek.MiningWindowDays))
	candidates, err := peek.NewStdioClient(args, command).Sessions(ctx, strings.Split(n.options.agents, ","))
	if err != nil {
		return 0, fmt.Errorf("nightlyRun.pendingCount: %w", err)
	}
	return len(sessions.SelectPending(candidates, request)), nil
}

func (n *nightlyRun) pipelinePrompt(isStyleDue bool, marker string) string {
	stages := []string{}
	if isStyleDue {
		stages = append(stages, styleStageName)
	}
	stages = append(stages, nightlyMiningStages)
	if n.options.isOrchestrated {
		stages = append(stages, judgeStageName)
	}

	parts := []string{n.runner.manifest.Prompt, "--stages " + strings.Join(stages, ",")}
	if n.options.isDevMode {
		parts = append(parts, "--dev")
	}
	if n.options.isSubagents {
		parts = append(parts, "--subagents")
	}
	parts = append(
		parts,
		"--agents "+n.options.agents,
		fmt.Sprintf("--cap %d", n.options.changesCap),
		fmt.Sprintf("--limit %d", n.options.sessionsCap),
	)
	if n.options.language != "" {
		parts = append(parts, "--language "+n.options.language)
	}
	parts = append(parts, "--marker "+marker)
	return n.runner.composePrompt(strings.Join(parts, " "))
}

// publish commits the night's two branches and records the publish block; returns 0 or 1.
func (n *nightlyRun) publish(ctx context.Context, worktreePath string, summary *runSummary, status int) int {
	tasks, err := routines.ReadChangeTasks(worktreePath)
	if err != nil {
		log.Printf("nightlyRun.publish: Task files: %v", err)
	}
	n.report.setTasks(tasks)
	n.report.setChanges(applySettled(n.report.report.Changes, n.sweepOversized(ctx)))

	publishStatus := exitOk
	batchBranch, err := n.runner.worktree.publishNight(ctx, status, nightlyMarker(summary, status), n.branch)
	if err != nil {
		log.Printf("publish failed; worktree kept: %s: %v", worktreePath, err)
		publishStatus = publishFailedStatus
	}
	n.batchBranch = batchBranch

	block := &routines.RunReportPublish{Files: []routines.RunReportFile{}, Status: publishStatus}
	if batchBranch != "" {
		block.Commit, _ = git(ctx, n.runner.repoRoot, "rev-parse", batchBranch)
		block.Files = n.publishedFiles(ctx, batchBranch)
	}
	if publishStatus == exitOk {
		n.landBatch(ctx, block)
	}
	n.report.setPublish(n.branch, batchBranch, block)
	return publishStatus
}

func (n *nightlyRun) publishedFiles(ctx context.Context, branch string) []routines.RunReportFile {
	output, err := git(ctx, n.runner.repoRoot, "show", "--name-status", "--format=", branch)
	if err != nil {
		return []routines.RunReportFile{}
	}

	files := []routines.RunReportFile{}
	for _, line := range nonEmptyLines(output) {
		status, path, _ := strings.Cut(line, "\t")
		files = append(files, routines.RunReportFile{Path: path, Status: status})
	}
	return files
}

// readyShas orders the judge's ready changes as the branch holds them, parent before child.
func (n *nightlyRun) readyShas(ctx context.Context) []string {
	ready := make([]string, 0)
	for _, change := range n.report.report.Changes {
		if change.Verdict == string(changeReady) {
			ready = append(ready, change.Sha)
		}
	}

	output, err := git(ctx, n.runner.repoRoot, "rev-list", "--reverse", mainBranch+".."+n.branch)
	if err != nil {
		return ready
	}
	ordered := make([]string, 0, len(ready))
	for _, sha := range nonEmptyLines(output) {
		if slices.ContainsFunc(ready, func(candidate string) bool { return strings.HasPrefix(sha, candidate) }) {
			ordered = append(ordered, sha)
		}
	}
	return ordered
}

func (n *nightlyRun) run(ctx context.Context) int {
	if err := n.runner.worktree.prepareNight(ctx); err != nil {
		log.Printf("worktree prepare failed: %v", err)
		n.runner.appendFailure(exitSoftware, "worktree prepare failed: "+firstLine(err.Error()))
		return exitSoftware
	}

	pending, err := countPending(n, ctx)
	if err != nil {
		log.Printf("pending sessions: %v", err)
		n.runner.appendFailure(exitSoftware, "pending sessions: "+firstLine(err.Error()))
		return exitSoftware
	}
	if pending == 0 {
		reason := "nothing to mine: no unmined session in peek's window"
		log.Print(reason)
		n.report.setOrchestrate(&routines.RunReportOrchestrate{Report: reason})
		n.report.finalize(exitOk)
		return exitOk
	}

	n.runner.worktree.ageOut(ctx, n.options.windowNights)
	branch, err := n.runner.worktree.createNight(ctx)
	if err != nil {
		log.Printf("worktree create failed: %v", err)
		n.runner.appendFailure(exitSoftware, "worktree create failed: "+firstLine(err.Error()))
		return exitSoftware
	}
	n.branch = branch
	n.runner.worktree.regenerateContext()
	n.runner.markStarted(branch)

	worktreePath := n.runner.worktree.path
	overlaySkillVisibility(worktreePath)
	summary, status := n.dispatch(ctx, worktreePath)
	if ctx.Err() != nil {
		return status
	}

	publishStatus := n.publish(ctx, worktreePath, summary, status)
	n.runner.commitStores(ctx)

	isLandDue := n.options.isOrchestrated && publishStatus == exitOk
	if isLandDue {
		n.report.setOrchestrate(n.land(ctx, summary.Verdict))
	}
	n.runner.worktree.deleteSettledProposals(ctx, branch)
	n.report.finalize(status)
	return status
}

// sweepOversized parks tonight's change commits over the proposal cap: an Icebox row plus the rejected ref; returns each settled sha with its reason.
func (n *nightlyRun) sweepOversized(ctx context.Context) map[string]string {
	settled := map[string]string{}
	open, err := repos.OpenChanges(ctx, n.runner.worktree.agentwtPath, n.runner.repoRoot)
	if err != nil {
		log.Printf("nightlyRun.sweepOversized: %v", err)
		return settled
	}

	reports, err := routines.Reports(n.runner.dir, routines.HistoryLimit())
	if err != nil {
		log.Printf("nightlyRun.sweepOversized: Reports: %v", err)
	}

	storePath := filepath.Join(n.runner.storeRoot, settings.StoreBacklogDir, "backlog.json")
	for index := range open {
		change := &open[index]
		if change.Branch != n.branch {
			continue
		}
		stat, err := repos.ReadChangeStat(ctx, n.runner.repoRoot, change.Commit.Sha)
		if err != nil {
			log.Printf("nightlyRun.sweepOversized: %v", err)
			continue
		}
		if !stat.IsOverCap() {
			continue
		}

		item := oversizedItem(change, n.sessionId, routines.FindTask(reports, change.Message.ChangeIds[0]), stat, time.Now())
		id, err := backlog.AppendIcebox(item, storePath)
		if err != nil {
			log.Printf("nightlyRun.sweepOversized: Change %s: %v", change.Commit.ShortSha, err)
			continue
		}
		if err := repos.Reject(ctx, n.runner.repoRoot, change.Commit.Sha); err != nil {
			log.Printf("nightlyRun.sweepOversized: Change %s: %v", change.Commit.ShortSha, err)
			continue
		}
		settled[change.Commit.Sha] = fmt.Sprintf("over the proposal cap: %d files, %d lines → %s", stat.Files, stat.Lines, id)
		log.Printf("nightlyRun.sweepOversized: Change %s: %s", change.Commit.ShortSha, settled[change.Commit.Sha])
	}
	return settled
}

func (n *nightlyRun) writeStyleStamp() {
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	if err := os.WriteFile(filepath.Join(n.runner.dir, styleStampFileName), []byte(stamp+"\n"), 0o644); err != nil {
		log.Printf("nightlyRun.writeStyleStamp: %v", err)
	}
}

// applySettled rewrites the night's rows for the shas the sweep settled; a report may record a short sha.
func applySettled(changes []routines.RunReportChange, settled map[string]string) []routines.RunReportChange {
	for index := range changes {
		for sha, reason := range settled {
			if strings.HasPrefix(sha, changes[index].Sha) || strings.HasPrefix(changes[index].Sha, sha) {
				changes[index].Reason = reason
				changes[index].Verdict = string(changeRejected)
			}
		}
	}
	return changes
}

func compactStamp(moment time.Time) string {
	return moment.UTC().Format("20060102T150405Z")
}

func firstFailure(current, next int) int {
	if current != exitOk {
		return current
	}
	return next
}

func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func loadNightlyOptions(profile *presentation.Profile) *nightlyOptions {
	isCasual := profile.Audience == presentation.AudienceCasual
	subagents := envString("SMINE_SUBAGENTS", "0")
	if subagents != "0" && subagents != "1" {
		log.Printf("loadNightlyOptions: SMINE_SUBAGENTS=%s is neither 0 nor 1; subagent mining off", subagents)
	}
	options := &nightlyOptions{
		agents:         envString("SMINE_AGENTS", "claude,codex"),
		changesCap:     envInt("SMINE_MAX_PROPOSALS_MINED", defaultChangesCap),
		isAutoMerge:    nightlyAutoMerge(profile, isCasual),
		isDevMode:      nightlyDevMode(profile, isCasual),
		isOrchestrated: envString("ROUTINE_ORCHESTRATE", "1") == "1",
		isSubagents:    subagents == "1",
		language:       profile.Language,
		sessionsCap:    envInt("SMINE_MAX_SESSIONS_MINED", defaultSessionsCap),
		windowNights:   envInt("SMINE_PROPOSAL_WINDOW_NIGHTS", defaultWindowNights),
	}
	return options
}

// nightlyMarker: a summary naming failed stages means the pipeline ran to its end.
func nightlyMarker(summary *runSummary, status int) string {
	failed := summary.failedStages()
	if len(failed) == 0 {
		return exitMarker(status)
	}
	return fmt.Sprintf(stageFailedMarker, strings.Join(failed, ", "))
}

// nightlyAutoMerge: ROUTINE_AUTO_MERGE wins, casual always merges, developers may opt out.
func nightlyAutoMerge(profile *presentation.Profile, isCasual bool) bool {
	if value := os.Getenv("ROUTINE_AUTO_MERGE"); value != "" {
		return value != "0"
	}
	if isCasual {
		return true
	}
	return profile.AutoMerge
}

// nightlyDevMode: casual forces it off; a non-empty SMINE_DEV_MODE wins over the profile.
func nightlyDevMode(profile *presentation.Profile, isCasual bool) bool {
	if isCasual {
		return false
	}
	if os.Getenv("SMINE_DEV_MODE") != "" {
		return true
	}
	return profile.DevMode
}

// overlaySkillVisibility writes gitignored local settings, never committed.
func overlaySkillVisibility(worktreePath string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	overlay := filepath.Join(home, ".config", "claude-routine", "skill-overrides.json")
	if _, err := os.Stat(overlay); err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Join(worktreePath, ".claude"), 0o755); err != nil {
		log.Printf("overlaySkillVisibility: %v", err)
		return
	}
	if err := fsx.CopyFile(overlay, filepath.Join(worktreePath, ".claude", "settings.local.json")); err != nil {
		log.Printf("overlaySkillVisibility: %v", err)
	}
}

// oversizedItem is the Icebox row for a change over the cap: the task's problem and fix when a report holds it, else the commit's own text; minedBy is the run's session.
func oversizedItem(change *repos.OpenChange, minedBy string, task *routines.ChangeTask, stat *repos.ChangeStat, now time.Time) *backlog.Item {
	today := now.Format(time.DateOnly)
	problem, fix, doneWhen := change.Message.Body, "", change.Message.Subject
	if task != nil {
		problem, fix, doneWhen = task.Problem, task.Fix, task.Change
	}

	size := fmt.Sprintf(
		"%d files, %d changed lines — over the proposal cap (%d files / %d lines); commit %s on %s",
		stat.Files,
		stat.Lines,
		repos.MaxChangeFiles,
		repos.MaxChangeLines,
		change.Commit.Sha,
		change.Branch,
	)
	lines := []string{problem}
	title := backlog.ClampTitle(change.Message.Subject)
	if title != change.Message.Subject {
		lines = slices.Insert(lines, 0, change.Message.Subject)
	}
	if fix != "" {
		lines = append(lines, fix)
	}
	lines = append(lines, size, "Done when: "+doneWhen)
	evidence := append([]string{"commit " + change.Commit.Sha}, change.Message.Sessions...)
	item := &backlog.Item{
		Added:       today,
		Deps:        []string{},
		Description: strings.Join(lines, "\n"),
		Evidence:    evidence,
		Link:        "",
		Mined:       &backlog.Mined{By: minedBy, Date: today, Sessions: change.Message.Sessions},
		Pri:         "high",
		Route:       "/fdesign",
		Size:        "L",
		Status:      backlog.StatusShaped,
		Theme:       "smine",
		Title:       title,
	}
	return item
}

func readTrimmed(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

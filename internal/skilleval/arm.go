package skilleval

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/routinerun"
	"github.com/kevinhorst/smine/internal/routines"
	"github.com/kevinhorst/smine/internal/shell"
	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skills"
)

const (
	artifactsDir    = "artifacts"
	checkLogFile    = "check.log"
	diffFile        = "diff.patch"
	inputsDir       = "inputs"
	outDirSuffix    = "-out"
	overlayCopyFile = "review-context.patch"
	resultFile      = "result.json"
	scorerLogFile   = "scorer.log"
	streamFile      = "stream.jsonl"
	traceFile       = "trace.md"

	armExtraTools = ", Workflow, Skill"
	// armStoreDir is where a smoke fixture tree places the stores the skill works on.
	armStoreDir      = "store"
	contextIndexEnv  = "SKILL_CONTEXT_INDEX"
	fallbackTools    = "Read, Write, Edit, Agent, Glob, Grep, Workflow, Skill, Bash(git diff *), Bash(git log *), Bash(git rev-parse *), Bash(jq *), Bash(ls *)"
	floatBits        = 64
	mcpConfigPattern = "skilleval-mcp-*.json"
	mcpToolPrefix    = "mcp__"
	oauthTokenEnv    = "CLAUDE_CODE_OAUTH_TOKEN"
	permissionMode   = "acceptEdits"
	resultTextRunes  = 300
	resumePrompt     = "The usage window has reset. Resume exactly where you stopped — every instruction of the original prompt stands."
	shortestFloat    = -1
	statusAbsent     = "absent"
	statusConnected  = "connected"
)

// Run-record identity values: whoever writes run.json sets them, RunArm copies them into result.json.
const (
	LayerEval       = "eval"
	LayerSmoke      = "smoke"
	ModeInteractive = "interactive"
	ModeNightly     = "nightly"
)

// competingAuthEnv are the credentials claude prefers over CLAUDE_CODE_OAUTH_TOKEN.
var competingAuthEnv = []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN"}

// scoreNamePattern is eval.json's metric-id grammar a score name must fit once prefixed with score_.
var scoreNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// sessionIdPattern is the UUID shape of a claude session id a resume names.
var sessionIdPattern = regexp.MustCompile(`^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$`)

// ArmResult is the run record of every test layer; session time is peek's and lives only in eval.json.
type ArmResult struct {
	AllowedTools      string             `json:"allowedTools"`
	Auth              string             `json:"auth"`
	CandidateCount    *int               `json:"candidateCount"`
	CheckExit         *int               `json:"checkExit"`
	CostUsd           *float64           `json:"costUsd"`
	EndedAt           time.Time          `json:"endedAt"`
	ExitStatus        int                `json:"exitStatus"`
	Golden            string             `json:"golden,omitempty"`
	InvalidReasons    []string           `json:"invalidReasons"`
	IsError           bool               `json:"isError"`
	Layer             string             `json:"layer"`
	McpServers        []McpServer        `json:"mcpServers"`
	Mode              string             `json:"mode"`
	Model             string             `json:"model"`
	OutputTokens      *int               `json:"outputTokens"`
	PermissionDenials *int               `json:"permissionDenials"`
	ResultText        string             `json:"resultText"`
	Scores            map[string]float64 `json:"scores,omitempty"`
	SessionId         string             `json:"sessionId"`
	Skill             string             `json:"skill"`
	SkillVersion      string             `json:"skillVersion"`
	Smoke             *SmokeResult       `json:"smoke,omitempty"`
	StartedAt         time.Time          `json:"startedAt"`
	Subtype           string             `json:"subtype"`
	TerminalReason    string             `json:"terminalReason"`
	ToolCalls         []ToolCount        `json:"toolCalls"`
	ToolErrors        int                `json:"toolErrors"`
	UnparsedLines     int                `json:"unparsedLines"`
}

// mcpConfigFile mirrors the mcpServers object of .claude.json and of a --mcp-config file.
type mcpConfigFile struct {
	McpServers map[string]json.RawMessage `json:"mcpServers"`
}

func newArmResult(checkExit *int, status int, stream *ArmStream, tools string) *ArmResult {
	result := &ArmResult{
		AllowedTools:      tools,
		CheckExit:         checkExit,
		CostUsd:           stream.CostUsd,
		ExitStatus:        status,
		OutputTokens:      stream.OutputTokens,
		PermissionDenials: stream.PermissionDenials,
		ToolCalls:         stream.ToolCalls,
		ToolErrors:        stream.ToolErrors,
		UnparsedLines:     stream.UnparsedLines,
	}
	if stream.Init != nil {
		result.McpServers = stream.Init.McpServers
		result.Model = stream.Init.Model
		result.SessionId = stream.Init.SessionId
	}
	if stream.Result != nil {
		result.IsError = stream.Result.IsError
		result.ResultText = truncateRunes(stream.Result.Result, resultTextRunes)
		result.SessionId = stream.Result.SessionId
		result.Subtype = stream.Result.Subtype
		result.TerminalReason = stream.Result.TerminalReason
	}
	return result
}

type armPermissions struct {
	AdditionalDirectories []string `json:"additionalDirectories"`
}

// armSettings admits the arm's -out dir to the engine and the tool-policy gate.
type armSettings struct {
	Permissions armPermissions `json:"permissions"`
}

// resumeMarker is the stream line the runner writes where a resumed cell's records begin.
type resumeMarker struct {
	At        time.Time `json:"at"`
	SessionId string    `json:"session_id"`
	Subtype   string    `json:"subtype"`
	Type      string    `json:"type"`
}

// scorerOutput is a scorer's stdout: named metric values of one run.
type scorerOutput struct {
	Metrics map[string]float64 `json:"metrics"`
}

// SmokeCoverage is all zero for a terminal-verdict smoke.
type SmokeCoverage struct {
	AnchorsDeclared    int      `json:"anchorsDeclared"`
	AnchorsExpected    []string `json:"anchorsExpected,omitempty"`
	AnchorsExpectedHit int      `json:"anchorsExpectedHit,omitempty"`
	AnchorsHit         int      `json:"anchorsHit"`
	ChecksDeclared     int      `json:"checksDeclared"`
	ChecksPassed       int      `json:"checksPassed"`
}

// Summary is the coverage as the smoke output and the skill pages print it: expected hit over expected, declared in brackets.
func (c *SmokeCoverage) Summary() string {
	if len(c.AnchorsExpected) == 0 {
		return fmt.Sprintf("anchors %d (%d), checks %d/%d", c.AnchorsHit, c.AnchorsDeclared, c.ChecksPassed, c.ChecksDeclared)
	}
	return fmt.Sprintf("anchors %d/%d (%d), checks %d/%d", c.AnchorsExpectedHit, len(c.AnchorsExpected), c.AnchorsDeclared, c.ChecksPassed, c.ChecksDeclared)
}

type SmokeResult struct {
	Cause    string        `json:"cause"`
	Coverage SmokeCoverage `json:"coverage"`
	Verdict  string        `json:"verdict"`
}

// armRunId is the arm's run id, lowercased: run dir basename plus label.
func armRunId(label, runDir string) string {
	return strings.ToLower(filepath.Base(runDir) + "-" + label)
}

func armTools(skill string) string {
	tools := skills.AllowedTools(filepath.Join(skills.DefaultHomePath(), skill))
	if tools == "" {
		return fallbackTools
	}
	return tools + armExtraTools
}

// appendResumeMarker separates the stopped process's records from the resumed one's, so the parser banks the first totals.
func appendResumeMarker(resumeSession, streamPath string) error {
	marker := &resumeMarker{At: time.Now(), SessionId: resumeSession, Subtype: subtypeResume, Type: recordSkilleval}
	encoded, err := json.Marshal(marker)
	if err != nil {
		return fmt.Errorf("appendResumeMarker: %w", err)
	}

	stream, err := openStream(true, streamPath)
	if err != nil {
		return err
	}
	defer stream.Close()

	if _, err := stream.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("appendResumeMarker: %w", err)
	}
	return nil
}

// captureDiff stages everything so new files enter the diff, then unstages again.
func captureDiff(ctx context.Context, base, diffPath, worktree string) error {
	if _, err := shell.Run(ctx, worktree, "git", "add", "-A"); err != nil {
		return fmt.Errorf("captureDiff: Failed to stage: %w", err)
	}

	diff, diffErr := shell.Run(ctx, worktree, "git", "diff", "--cached", "--binary", base)
	if _, err := shell.Run(ctx, worktree, "git", "reset", "-q"); err != nil {
		return fmt.Errorf("captureDiff: Failed to unstage: %w", err)
	}

	if diffErr != nil {
		return fmt.Errorf("captureDiff: Failed to diff: %w", diffErr)
	}

	if err := os.WriteFile(diffPath, []byte(diff), 0o644); err != nil {
		return fmt.Errorf("captureDiff: %w", err)
	}

	return nil
}

// checkResumeSession refuses a resume target that is no session id before any file moves.
func checkResumeSession(resumeSession string) error {
	isMalformed := resumeSession != "" && !sessionIdPattern.MatchString(resumeSession)
	if isMalformed {
		return fmt.Errorf("checkResumeSession: %q is no session id: %w", resumeSession, ErrBadRunArg)
	}
	return nil
}

// claudeArgs keeps the tool deny as a second line behind the strict MCP config.
func claudeArgs(arm *ManifestArm, budgetUsd float64, mcpConfigPath, prompt, resumeSession, tools string) []string {
	args := []string{
		"claude",
		"-p", prompt,
		"--allowedTools", tools,
		"--permission-mode", permissionMode,
		"--max-budget-usd", strconv.FormatFloat(budgetUsd, 'f', shortestFloat, floatBits),
		"--output-format", "stream-json",
		"--verbose",
	}
	if resumeSession != "" {
		args = append(args, "--resume", resumeSession)
	}
	if arm.Model != "" {
		args = append(args, "--model", arm.Model)
	}
	if arm.Effort != "" {
		args = append(args, "--effort", arm.Effort)
	}
	if len(arm.DisabledServers) > 0 {
		args = append(args, "--disallowedTools", disallowedRules(arm.DisabledServers))
	}
	if mcpConfigPath != "" {
		args = append(args, "--strict-mcp-config", "--mcp-config", mcpConfigPath)
	}
	return args
}

func disallowedRules(servers []string) string {
	rules := make([]string, 0, len(servers))
	for _, server := range servers {
		rules = append(rules, mcpToolPrefix+server)
	}
	return strings.Join(rules, ", ")
}

// exportContextIndex points the hooks at the overlay's index, a name the reviewed tree does not know.
func exportContextIndex(armWorktree, contextIndex string) error {
	if contextIndex == "" {
		return nil
	}

	if err := os.Setenv(contextIndexEnv, filepath.Join(armWorktree, contextIndex)); err != nil {
		return fmt.Errorf("exportContextIndex: %w", err)
	}

	return nil
}

// exportReadFence fences the plans archive too: an arm never touches a live store, it has its own.
func exportReadFence(ctx context.Context, armWorktree string) error {
	evalRepo, err := mainCheckoutRoot(ctx)
	if err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("exportReadFence: %w", err)
	}

	storeArchive, err := config.StoreArchiveDir()
	if err != nil {
		return fmt.Errorf("exportReadFence: %w", err)
	}

	bases := []string{evalRepo, filepath.Join(evalRepo, ".claude", "worktrees", "*"), armWorktree}
	patterns := []string{
		filepath.Join(home, ".claude", "skills", "*", "eval"),
		filepath.Join(home, ".claude", "skills", "*", skillgraph.FileHelpTest),
	}
	for _, base := range bases {
		patterns = append(patterns, filepath.Join(base, "evals"), filepath.Join(base, "skills", "*", "eval"), filepath.Join(base, "skills", "*", "*", "eval"))
	}

	roots := []string{storeArchive}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("exportReadFence: %w", err)
		}
		roots = append(roots, matches...)
	}

	if err := os.Setenv(config.ReadFenceEnv, strings.Join(roots, string(os.PathListSeparator))); err != nil {
		return fmt.Errorf("exportReadFence: %w", err)
	}

	return nil
}

// exportStoreRoot keeps everything an arm writes out of the plans archive.
func exportStoreRoot(armWorktree string) error {
	if err := os.Setenv(config.StoreRootEnv, filepath.Join(armWorktree, armStoreDir)); err != nil {
		return fmt.Errorf("exportStoreRoot: %w", err)
	}

	return nil
}

// exportToken: an arm never bills an inherited env token, an API key or the CLI login.
func exportToken(label string) error {
	if label == "" {
		return errors.New("exportToken: Missing token label: run.json token must name a stored token")
	}

	token, err := routines.ReadToken(label)
	if err != nil {
		return fmt.Errorf("exportToken: %w", err)
	}

	for _, name := range competingAuthEnv {
		if err := os.Unsetenv(name); err != nil {
			return fmt.Errorf("exportToken: %w", err)
		}
	}

	if err := os.Setenv(oauthTokenEnv, token); err != nil {
		return fmt.Errorf("exportToken: %w", err)
	}

	return nil
}

// openStream appends a resumed cell's records behind the stopped cell's, so one file holds the whole run.
func openStream(isResume bool, path string) (*os.File, error) {
	if !isResume {
		created, err := os.Create(path)
		if err != nil {
			return nil, fmt.Errorf("openStream: %w", err)
		}
		return created, nil
	}

	appended, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("openStream: %w", err)
	}
	return appended, nil
}

func offersServerTools(server string, tools []string) bool {
	prefix := mcpToolPrefix + server + "__"
	for _, tool := range tools {
		if strings.HasPrefix(tool, prefix) {
			return true
		}
	}
	return false
}

func parseScores(stdout []byte) (map[string]float64, string) {
	var output scorerOutput
	if err := json.Unmarshal(stdout, &output); err != nil {
		return nil, "malformed stdout: " + err.Error()
	}

	if len(output.Metrics) == 0 {
		return nil, "no metrics"
	}

	for name := range output.Metrics {
		if !scoreNamePattern.MatchString(name) {
			return nil, fmt.Sprintf("metric name %q is not [a-z][a-z0-9_]*", name)
		}
	}
	return output.Metrics, ""
}

func placeSeed(manifest *RunManifest, outDir, runDir string) (string, error) {
	hasSeed := manifest.Golden.kind() == GoldenProduce && manifest.Golden.SeedFile != ""
	if !hasSeed {
		return "", nil
	}

	seedPath := filepath.Join(outDir, inputsDir, seedSnapshotFile)
	if err := os.MkdirAll(filepath.Dir(seedPath), 0o755); err != nil {
		return "", fmt.Errorf("placeSeed: %w", err)
	}

	if err := copyFile(filepath.Join(runDir, seedSnapshotFile), seedPath); err != nil {
		return "", fmt.Errorf("placeSeed: %w", err)
	}
	return seedPath, nil
}

// renderPrompt fills the per-arm placeholders a preset prompt carries.
func renderPrompt(artifactsPath, overlayPatch, prompt, runId, seedPath string) string {
	replacer := strings.NewReplacer(
		"{artifactsDir}", artifactsPath,
		"{overlayPatch}", overlayPatch,
		"{runId}", runId,
		"{seed}", seedPath,
	)
	return replacer.Replace(prompt)
}

func runCheck(ctx context.Context, arm *ManifestArm, logPath string, manifest *RunManifest) (int, error) {
	logOut, err := os.Create(logPath)
	if err != nil {
		return 0, fmt.Errorf("runCheck: %w", err)
	}
	defer logOut.Close()

	spec := &routinerun.CellSpec{
		Args:    manifest.Check,
		Dir:     arm.Worktree,
		Stderr:  logOut,
		Stdout:  logOut,
		Timeout: time.Duration(manifest.CheckTimeoutS) * time.Second,
	}
	status, err := routinerun.RunCell(ctx, spec)
	if err != nil {
		return 0, fmt.Errorf("runCheck: %w", err)
	}

	return status, nil
}

// resumedStart keeps the stopped cell's start so the record spans both halves of the run.
func resumedStart(armDir string, fallback time.Time) time.Time {
	var stopped ArmResult
	err := readJsonFile(filepath.Join(armDir, resultFile), &stopped)
	if err != nil || stopped.StartedAt.IsZero() {
		return fallback
	}
	return stopped.StartedAt
}

func runClaude(ctx context.Context, args []string, guard *initGuard, isResume bool, streamPath string, timeout time.Duration, worktree string) (int, error) {
	streamOut, err := openStream(isResume, streamPath)
	if err != nil {
		return 0, fmt.Errorf("runClaude: %w", err)
	}
	defer streamOut.Close()

	guard.out = streamOut
	spec := &routinerun.CellSpec{
		Args:    args,
		Dir:     worktree,
		Stdout:  guard,
		Timeout: timeout,
	}
	status, err := routinerun.RunCell(ctx, spec)
	if err != nil {
		return 0, fmt.Errorf("runClaude: %w", err)
	}

	return status, nil
}

// runScorer treats a failing or malformed scorer as a result, never a harness error.
func runScorer(ctx context.Context, arm *ManifestArm, logPath string, manifest *RunManifest) (map[string]float64, error) {
	if len(manifest.Scorer) == 0 {
		return nil, nil
	}

	logOut, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("runScorer: %w", err)
	}
	defer logOut.Close()

	var stdout bytes.Buffer
	spec := &routinerun.CellSpec{
		Args:    manifest.Scorer,
		Dir:     arm.Worktree,
		Stderr:  logOut,
		Stdout:  &stdout,
		Timeout: time.Duration(manifest.CheckTimeoutS) * time.Second,
	}
	status, err := routinerun.RunCell(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("runScorer: %w", err)
	}

	scores, cause := parseScores(stdout.Bytes())
	if status != 0 {
		cause = fmt.Sprintf("exit status %d", status)
	}
	if cause != "" {
		fmt.Fprintf(logOut, "scorer: %s\n%s", cause, stdout.String())
		return nil, nil
	}
	return scores, nil
}

func serverStatus(name string, servers []McpServer) string {
	for _, server := range servers {
		if server.Name == name {
			return server.Status
		}
	}
	return statusAbsent
}

// stageMcpConfig writes a temp file, never into a run dir, because it can hold server credentials.
func stageMcpConfig(claudeJsonPath string, disabledServers []string) (string, error) {
	if len(disabledServers) == 0 {
		return "", nil
	}

	var claudeJson mcpConfigFile
	data, err := os.ReadFile(claudeJsonPath)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return "", fmt.Errorf("stageMcpConfig: %w", err)
	default:
		if err := json.Unmarshal(data, &claudeJson); err != nil {
			return "", fmt.Errorf("stageMcpConfig: Failed to parse %s: %w", claudeJsonPath, err)
		}
	}

	staged := &mcpConfigFile{McpServers: map[string]json.RawMessage{}}
	for name, server := range claudeJson.McpServers {
		if !slices.Contains(disabledServers, name) {
			staged.McpServers[name] = server
		}
	}

	encoded, err := json.Marshal(staged)
	if err != nil {
		return "", fmt.Errorf("stageMcpConfig: %w", err)
	}

	file, err := os.CreateTemp("", mcpConfigPattern)
	if err != nil {
		return "", fmt.Errorf("stageMcpConfig: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(encoded); err != nil {
		return "", fmt.Errorf("stageMcpConfig: %w", err)
	}
	return file.Name(), nil
}

// stageOverlay keeps the path the arm receives out of the run dir.
func stageOverlay(armWorktree, overlay string) (string, error) {
	if overlay == "" {
		return "", nil
	}

	staged := filepath.Join(armWorktree+outDirSuffix, overlayCopyFile)
	if err := copyFile(overlay, staged); err != nil {
		return "", err
	}

	return staged, nil
}

// variantProblems checks the model because the CLI silently falls back on an unknown --model.
func variantProblems(arm *ManifestArm, initRecord *InitRecord, required []string) []string {
	if initRecord == nil {
		return nil
	}

	var problems []string
	isOtherModel := arm.Model != "" && !strings.Contains(initRecord.Model, arm.Model)
	if isOtherModel {
		problems = append(problems, fmt.Sprintf("requested model %s but the cell ran %s", arm.Model, initRecord.Model))
	}
	for _, server := range arm.DisabledServers {
		if offersServerTools(server, initRecord.Tools) {
			problems = append(problems, fmt.Sprintf("server %s disabled but its tools are still offered", server))
		}
	}
	for _, server := range required {
		status := serverStatus(server, initRecord.McpServers)
		if status != statusConnected {
			problems = append(problems, fmt.Sprintf("server %s not connected (%s) — comparison void", server, status))
		}
	}
	return problems
}

// writeArmSettings never makes the repository itself an additional directory.
func writeArmSettings(armWorktree string) error {
	settings := &armSettings{
		Permissions: armPermissions{AdditionalDirectories: []string{armWorktree + outDirSuffix}},
	}
	encoded, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("writeArmSettings: %w", err)
	}

	settingsDir := filepath.Join(armWorktree, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		return fmt.Errorf("writeArmSettings: %w", err)
	}
	if err := os.WriteFile(filepath.Join(settingsDir, "settings.local.json"), encoded, 0o644); err != nil {
		return fmt.Errorf("writeArmSettings: %w", err)
	}
	return nil
}

func writeCandidates(armDir, artifactsPath string, manifest *RunManifest) (*int, error) {
	if manifest.Findings.Glob == "" {
		return nil, nil
	}

	candidates, err := extractCandidates(artifactsPath, &manifest.Findings)
	if err != nil {
		return nil, err
	}

	if err := writeJsonFile(filepath.Join(armDir, candidatesFile), candidates); err != nil {
		return nil, err
	}

	count := len(candidates)
	return &count, nil
}

func writeTrace(path string, trace []string) error {
	content := strings.Join(trace, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writeTrace: %w", err)
	}

	return nil
}

// RunArm returns an error for harness failures only; a failing cell is a result.
func RunArm(ctx context.Context, label, resumeSession, runDir string, readRuntime RuntimeReader) error {
	if err := checkResumeSession(resumeSession); err != nil {
		return err
	}

	manifest, err := loadManifest(runDir)
	if err != nil {
		return err
	}

	arm, err := manifest.armByLabel(label)
	if err != nil {
		return err
	}

	armDir, err := filepath.Abs(filepath.Join(runDir, runsDir, arm.Label))
	if err != nil {
		return fmt.Errorf("RunArm: %w", err)
	}

	if err := os.MkdirAll(armDir, 0o755); err != nil {
		return fmt.Errorf("RunArm: %w", err)
	}

	armArtifacts := filepath.Join(armDir, artifactsDir)
	artifactsPath := filepath.Join(arm.Worktree+outDirSuffix, artifactsDir)
	if err := os.MkdirAll(arm.Worktree+outDirSuffix, 0o755); err != nil {
		return fmt.Errorf("RunArm: %w", err)
	}

	if resumeSession != "" {
		if err := os.Rename(armArtifacts, artifactsPath); err != nil {
			return fmt.Errorf("RunArm: Failed to hand the stopped cell its artifacts back: %w", err)
		}
	}

	if err := os.MkdirAll(artifactsPath, 0o755); err != nil {
		return fmt.Errorf("RunArm: %w", err)
	}

	seedPath, err := placeSeed(manifest, arm.Worktree+outDirSuffix, runDir)
	if err != nil {
		return err
	}

	if err := writeArmSettings(arm.Worktree); err != nil {
		return fmt.Errorf("RunArm: %w", err)
	}

	if err := exportToken(manifest.TokenLabel); err != nil {
		return err
	}

	overlayPath, err := stageOverlay(arm.Worktree, manifest.armOverlay(arm))
	if err != nil {
		return err
	}

	if err := exportReadFence(ctx, arm.Worktree); err != nil {
		return err
	}

	if err := exportStoreRoot(arm.Worktree); err != nil {
		return err
	}

	if err := exportContextIndex(arm.Worktree, manifest.ContextIndex); err != nil {
		return err
	}

	mcpConfigPath, err := stageMcpConfig(config.DefaultClaudeJsonPath(), arm.DisabledServers)
	if err != nil {
		return err
	}
	if mcpConfigPath != "" {
		defer os.Remove(mcpConfigPath)
	}

	tools := armTools(manifest.Skill)
	prompt := renderPrompt(artifactsPath, overlayPath, manifest.Prompt, armRunId(arm.Label, runDir), seedPath)
	startedAt := time.Now()
	isResume := resumeSession != ""
	if isResume {
		prompt = resumePrompt
		startedAt = resumedStart(armDir, startedAt)
		if err := appendResumeMarker(resumeSession, filepath.Join(armDir, streamFile)); err != nil {
			return err
		}
	}

	cellCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	guard := &initGuard{arm: arm, cancel: cancel, required: manifest.requiredServers(arm)}
	status, err := runClaude(
		cellCtx,
		claudeArgs(arm, manifest.BudgetUsd, mcpConfigPath, prompt, resumeSession, tools),
		guard,
		isResume,
		filepath.Join(armDir, streamFile),
		time.Duration(manifest.TimeoutS)*time.Second,
		arm.Worktree,
	)
	if err != nil {
		return err
	}

	endedAt := time.Now()
	if err := os.Rename(artifactsPath, armArtifacts); err != nil {
		return fmt.Errorf("RunArm: Failed to collect artifacts: %w", err)
	}

	stream, err := parseStreamFile(filepath.Join(armDir, streamFile))
	if err != nil {
		return err
	}

	if err := writeTrace(filepath.Join(armDir, traceFile), stream.Trace); err != nil {
		return err
	}

	var checkExit *int
	if len(manifest.Check) > 0 {
		checkStatus, err := runCheck(ctx, arm, filepath.Join(armDir, checkLogFile), manifest)
		if err != nil {
			return err
		}
		checkExit = &checkStatus
	}

	scores, err := runScorer(ctx, arm, filepath.Join(armDir, scorerLogFile), manifest)
	if err != nil {
		return err
	}

	candidateCount, err := writeCandidates(armDir, armArtifacts, manifest)
	if err != nil {
		return err
	}

	if err := captureDiff(ctx, manifest.At, filepath.Join(armDir, diffFile), arm.Worktree); err != nil {
		return err
	}

	result := newArmResult(checkExit, status, stream, tools)
	result.Auth = manifest.authLabel()
	result.CandidateCount = candidateCount
	result.EndedAt = endedAt
	result.Golden = manifest.Golden.kind()
	result.InvalidReasons = variantProblems(arm, stream.Init, manifest.requiredServers(arm))
	result.Layer = manifest.Layer
	result.Mode = manifest.Mode
	result.Scores = scores
	result.Skill = manifest.Skill
	result.SkillVersion = manifest.SkillVersion
	result.StartedAt = startedAt
	captured := captureRuntime(ctx, readRuntime, result.SessionId)
	if err := writeJsonFile(filepath.Join(armDir, runtimeFile), captured); err != nil {
		return err
	}
	return writeJsonFile(filepath.Join(armDir, resultFile), result)
}

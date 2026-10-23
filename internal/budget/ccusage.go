package budget

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/contextrecord"
)

const (
	ccusageConfigDirEnv   = "CLAUDE_CONFIG_DIR"
	ccusageDateLayout     = "20060102"
	ccusageVersionParts   = 3
	ccusageWorkflowPrefix = "wf_"
	ccusageWorkflowSuffix = "/subagents/workflows"
	uuidLength            = 36
)

// The oldest ccusage whose session report keys rows by transcript id.
var ccusageMinVersion = [ccusageVersionParts]int{20, 0, 20}

// The Claude report runs over the transcript archive alone, the Codex report over Codex's own store.
type Ccusage struct {
	ArchiveDir string
	BinaryPath string
}

func (c *Ccusage) run(ctx context.Context, env []string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, c.BinaryPath, args...)
	command.Env = append(os.Environ(), env...)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("Ccusage.run: %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String())) //nolint:staticcheck // ST1005: the Receiver.method prefix is the repo's error-message convention
	}
	return output, nil
}

// CheckVersion fails for a missing binary or one older than ccusageMinVersion.
func (c *Ccusage) CheckVersion(ctx context.Context) error {
	output, err := c.run(ctx, nil, "--version")
	if err != nil {
		return fmt.Errorf("Ccusage.CheckVersion: %w", err)
	}

	version, ok := parseCcusageVersion(string(output))
	if !ok {
		return fmt.Errorf("Ccusage.CheckVersion: Unreadable version %q", strings.TrimSpace(string(output)))
	}
	if isOlderVersion(version, ccusageMinVersion) {
		return fmt.Errorf("Ccusage.CheckVersion: Version %s is older than %s", joinVersion(version), joinVersion(ccusageMinVersion))
	}
	return nil
}

// A workflow row is added to the session its project path names.
func (c *Ccusage) ClaudeSessions(ctx context.Context, since time.Time) (map[string]*CcusageSide, error) {
	env := []string{ccusageConfigDirEnv + "=" + c.ArchiveDir}
	output, err := c.run(ctx, env, sessionArgs("claude", since)...)
	if err != nil {
		return nil, fmt.Errorf("Ccusage.ClaudeSessions: %w", err)
	}

	sides, err := decodeClaudeSessions(output)
	if err != nil {
		return nil, fmt.Errorf("Ccusage.ClaudeSessions: %w", err)
	}
	return sides, nil
}

// Keyed by the session uuid its rollout file name ends with.
func (c *Ccusage) CodexSessions(ctx context.Context, since time.Time) (map[string]*CcusageSide, error) {
	output, err := c.run(ctx, nil, sessionArgs("codex", since)...)
	if err != nil {
		return nil, fmt.Errorf("Ccusage.CodexSessions: %w", err)
	}

	sides, err := decodeCodexSessions(output)
	if err != nil {
		return nil, fmt.Errorf("Ccusage.CodexSessions: %w", err)
	}
	return sides, nil
}

// Overwrites ccusage sides whatever a capture run already covered.
func (c *Ccusage) Recalculate(ctx context.Context, store *Store) (int, error) {
	if err := c.CheckVersion(ctx); err != nil {
		return 0, fmt.Errorf("Ccusage.Recalculate: %w", err)
	}
	records, err := store.Records()
	if err != nil {
		return 0, fmt.Errorf("Ccusage.Recalculate: %w", err)
	}

	checkedAt := time.Now()
	claude, err := c.ClaudeSessions(ctx, time.Time{})
	if err != nil {
		return 0, fmt.Errorf("Ccusage.Recalculate: %w", err)
	}
	codex, err := c.CodexSessions(ctx, time.Time{})
	if err != nil {
		return 0, fmt.Errorf("Ccusage.Recalculate: %w", err)
	}
	return saveSides(checkedAt, claude, codex, records, store)
}

type ccusageClaudeReport struct {
	Sessions []ccusageClaudeRow `json:"sessions"`
}

type ccusageClaudeRow struct {
	CacheCreationTokens int               `json:"cacheCreationTokens"`
	CacheReadTokens     int               `json:"cacheReadTokens"`
	FirstActivity       time.Time         `json:"firstActivity"`
	InputTokens         int               `json:"inputTokens"`
	LastActivity        time.Time         `json:"lastActivity"`
	ModelBreakdowns     []ccusageModelRow `json:"modelBreakdowns"`
	OutputTokens        int               `json:"outputTokens"`
	ProjectPath         string            `json:"projectPath"`
	SessionId           string            `json:"sessionId"`
	TotalCost           float64           `json:"totalCost"`
}

func (r *ccusageClaudeRow) isWorkflow() bool {
	return strings.HasPrefix(r.SessionId, ccusageWorkflowPrefix)
}

// A workflow row's project path ends in <session id>/subagents/workflows.
func (r *ccusageClaudeRow) parentId() string {
	if !r.isWorkflow() {
		return r.SessionId
	}
	parent, isNested := strings.CutSuffix(filepath.ToSlash(r.ProjectPath), ccusageWorkflowSuffix)
	if !isNested {
		return r.SessionId
	}
	return filepath.Base(parent)
}

func (r *ccusageClaudeRow) usage() TokenClasses {
	classes := TokenClasses{
		CacheCreation: r.CacheCreationTokens,
		CacheRead:     r.CacheReadTokens,
		Input:         r.InputTokens,
		Output:        r.OutputTokens,
	}
	return classes
}

type ccusageCodexReport struct {
	Sessions []ccusageCodexRow `json:"sessions"`
}

type ccusageCodexRow struct {
	CacheCreationTokens int       `json:"cacheCreationTokens"`
	CacheReadTokens     int       `json:"cacheReadTokens"`
	CostUsd             float64   `json:"costUSD"`
	InputTokens         int       `json:"inputTokens"`
	LastActivity        time.Time `json:"lastActivity"`
	OutputTokens        int       `json:"outputTokens"`
	SessionFile         string    `json:"sessionFile"`
	SessionId           string    `json:"sessionId"`
}

func (r *ccusageCodexRow) usage() TokenClasses {
	classes := TokenClasses{
		CacheCreation: r.CacheCreationTokens,
		CacheRead:     r.CacheReadTokens,
		Input:         r.InputTokens,
		Output:        r.OutputTokens,
	}
	return classes
}

// uuid is the trailing session uuid of rollout-<timestamp>-<uuid>.
func (r *ccusageCodexRow) uuid() string {
	if len(r.SessionFile) < uuidLength {
		return r.SessionId
	}
	return r.SessionFile[len(r.SessionFile)-uuidLength:]
}

// ccusageModelRow is one model's part of a Claude session row.
type ccusageModelRow struct {
	CacheCreationTokens int    `json:"cacheCreationTokens"`
	CacheReadTokens     int    `json:"cacheReadTokens"`
	InputTokens         int    `json:"inputTokens"`
	ModelName           string `json:"modelName"`
	OutputTokens        int    `json:"outputTokens"`
}

func (r *ccusageModelRow) usage() TokenClasses {
	classes := TokenClasses{
		CacheCreation: r.CacheCreationTokens,
		CacheRead:     r.CacheReadTokens,
		Input:         r.InputTokens,
		Output:        r.OutputTokens,
	}
	return classes
}

func addModelUsage(known []ModelUsage, rows []ccusageModelRow) []ModelUsage {
	for index := range rows {
		row := &rows[index]
		position := slices.IndexFunc(known, func(usage ModelUsage) bool { return usage.Model == row.ModelName })
		if position < 0 {
			entry := ModelUsage{Model: row.ModelName}
			known = append(known, entry)
			position = len(known) - 1
		}
		known[position].Usage = known[position].Usage.Add(row.usage())
	}
	slices.SortFunc(known, func(a, b ModelUsage) int { return strings.Compare(a.Model, b.Model) })
	return known
}

func decodeClaudeSessions(output []byte) (map[string]*CcusageSide, error) {
	var report ccusageClaudeReport
	if err := json.Unmarshal(output, &report); err != nil {
		return nil, fmt.Errorf("decodeClaudeSessions: %w", err)
	}

	sides := make(map[string]*CcusageSide)
	for index := range report.Sessions {
		row := &report.Sessions[index]
		side := sideFor(row.parentId(), sides)
		side.CostUsd += row.TotalCost
		side.Models = addModelUsage(side.Models, row.ModelBreakdowns)
		side.Usage = side.Usage.Add(row.usage())
		side.FirstActivity = earlier(side.FirstActivity, row.FirstActivity)
		side.LastActivity = later(side.LastActivity, row.LastActivity)
		if row.isWorkflow() {
			side.WorkflowRows++
		}
	}
	markUnpriced(sides)
	return sides, nil
}

func decodeCodexSessions(output []byte) (map[string]*CcusageSide, error) {
	var report ccusageCodexReport
	if err := json.Unmarshal(output, &report); err != nil {
		return nil, fmt.Errorf("decodeCodexSessions: %w", err)
	}

	sides := make(map[string]*CcusageSide)
	for index := range report.Sessions {
		row := &report.Sessions[index]
		side := sideFor(row.uuid(), sides)
		side.CostUsd += row.CostUsd
		side.Usage = side.Usage.Add(row.usage())
		side.LastActivity = later(side.LastActivity, row.LastActivity)
	}
	markUnpriced(sides)
	return sides, nil
}

func earlier(known, candidate time.Time) time.Time {
	if known.IsZero() || candidate.Before(known) {
		return candidate
	}
	return known
}

func isOlderVersion(version, minimum [ccusageVersionParts]int) bool {
	for index := range version {
		if version[index] != minimum[index] {
			return version[index] < minimum[index]
		}
	}
	return false
}

func joinVersion(version [ccusageVersionParts]int) string {
	return fmt.Sprintf("%d.%d.%d", version[0], version[1], version[2])
}

func later(known, candidate time.Time) time.Time {
	if candidate.After(known) {
		return candidate
	}
	return known
}

// ccusage 20.0.20 prices an unknown model at 0 without a marker.
func markUnpriced(sides map[string]*CcusageSide) {
	for _, side := range sides {
		side.IsUnpriced = side.CostUsd == 0 && side.Usage.Total() > 0
	}
}

// parseCcusageVersion reads "ccusage 20.0.20".
func parseCcusageVersion(output string) ([ccusageVersionParts]int, bool) {
	var version [ccusageVersionParts]int
	fields := strings.Fields(output)
	if len(fields) == 0 {
		return version, false
	}
	parts := strings.Split(fields[len(fields)-1], ".")
	if len(parts) != ccusageVersionParts {
		return version, false
	}
	for index, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil {
			return version, false
		}
		version[index] = number
	}
	return version, true
}

// A record ccusage did not return keeps the side it has.
func saveSides(checkedAt time.Time, claude, codex map[string]*CcusageSide, records []*Record, store *Store) (int, error) {
	byId := make(map[string]*Record, len(records))
	for _, record := range records {
		byId[record.Id] = record
	}
	applyCcusage(contextrecord.SessionClaude, byId, claude)
	applyCcusage(contextrecord.SessionCodex, byId, codex)

	saved := 0
	for id, record := range byId {
		_, isClaude := claude[id]
		_, isCodex := codex[id]
		if !isClaude && !isCodex {
			continue
		}

		record.CcusageChecked = checkedAt
		if err := store.SaveRecord(record); err != nil {
			return saved, fmt.Errorf("saveSides: %w", err)
		}
		saved++
	}
	return saved, nil
}

func sessionArgs(agent string, since time.Time) []string {
	args := []string{agent, "session", "--json"}
	if !since.IsZero() {
		args = append(args, "--since", since.Format(ccusageDateLayout))
	}
	return args
}

func sideFor(id string, sides map[string]*CcusageSide) *CcusageSide {
	side, ok := sides[id]
	if !ok {
		side = &CcusageSide{}
		sides[id] = side
	}
	return side
}

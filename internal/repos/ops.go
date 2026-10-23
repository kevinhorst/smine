package repos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

// agentwt pick contract: conflict and skipped exit pickRefusalExit with the document still on stdout.
const (
	PickOutcomeConflict = "conflict"
	PickOutcomePicked   = "picked"
	PickOutcomeSkipped  = "skipped"

	pickRefusalExit = 3
	pickSchema      = 1
)

// agentwt probe contract: every outcome is a row of the document, the exit is 0.
const (
	ProbeOutcomeApplies  = "applies"
	ProbeOutcomeConflict = "conflict"
	ProbeOutcomeSkipped  = "skipped"

	probeSchema = 1
)

// agentwt forward contract: skipped exits pickRefusalExit with the document still on stdout.
const (
	ForwardOutcomeForwarded = "forwarded"
	ForwardOutcomeSkipped   = "skipped"
	ForwardOutcomeUnchanged = "unchanged"

	forwardSchema = 1
)

type ForwardResult struct {
	From    string `json:"from"`
	Onto    string `json:"onto"`
	Outcome string `json:"outcome"`
	Reason  string `json:"reason"`
	Schema  int    `json:"schema"`
	To      string `json:"to"`
}

type OpOutput struct {
	Lines []OpLine
	Text  string
}

// OpLine is one outcome line of an operation (wire.rs OpLine).
type OpLine struct {
	Outcome string `json:"outcome"` // removed | branch_deleted | skipped
	Subject string `json:"subject"`
	Text    string `json:"text"`
}

// opDocument is the binary's operation result on the wire (wire.rs OpResult).
type opDocument struct {
	Lines     []OpLine `json:"lines"`
	Operation string   `json:"operation"`
}

func (o *OpOutput) HasSkips() bool {
	for _, line := range o.Lines {
		if line.Outcome == "skipped" {
			return true
		}
	}
	return false
}

type PickResult struct {
	Commit    string `json:"commit"`
	NewCommit string `json:"new_commit"`
	Onto      string `json:"onto"`
	Outcome   string `json:"outcome"`
	Reason    string `json:"reason"`
	Schema    int    `json:"schema"`
}

// IsRefusal reports whether the pick left the target untouched.
func (p *PickResult) IsRefusal() bool {
	switch p.Outcome {
	case PickOutcomeConflict, PickOutcomeSkipped:
		return true
	case PickOutcomePicked:
		return false
	}
	return false
}

// ProbeEntry is one probed commit: what a pick onto the target would do.
type ProbeEntry struct {
	Commit  string `json:"commit"`
	Outcome string `json:"outcome"`
	Reason  string `json:"reason"`
}

func (e *ProbeEntry) IsApplicable() bool {
	return e.Outcome == ProbeOutcomeApplies
}

// ProbeResult is the agentwt probe document: one entry per asked commit, in argument order.
type ProbeResult struct {
	Commits []ProbeEntry `json:"commits"`
	Onto    string       `json:"onto"`
	OntoSha string       `json:"onto_sha"`
	Schema  int          `json:"schema"`
}

// ByCommit indexes the entries by their full sha.
func (p *ProbeResult) ByCommit() map[string]ProbeEntry {
	entries := make(map[string]ProbeEntry, len(p.Commits))
	for _, entry := range p.Commits {
		entries[entry.Commit] = entry
	}
	return entries
}

// AgentBranchKind is the Go twin of agentwt's agent_kind; keep the two in step.
func AgentBranchKind(branch string) string {
	kind, _, hasSlash := strings.Cut(branch, "/")
	if !hasSlash {
		return ""
	}
	if kind != "claude" && !strings.HasPrefix(kind, "claude-") {
		return ""
	}
	return kind
}

// Forward moves a local branch to a commit that holds its tip; the caller verified both.
func Forward(ctx context.Context, agentwtPath, onto, repoPath, sha string) (*ForwardResult, error) {
	output, runErr := shell.Run(ctx, repoPath, agentwtPath, agentwtArgs(repoPath, "forward", "--onto", onto, sha)...)
	result, err := decodeForward(output)
	if err != nil {
		if runErr != nil {
			return nil, fmt.Errorf("Forward: %s: %w", strings.TrimSpace(output), runErr)
		}
		return nil, fmt.Errorf("Forward: %w", err)
	}

	var exitErr *exec.ExitError
	isRefusalExit := errors.As(runErr, &exitErr) && exitErr.ExitCode() == pickRefusalExit
	switch {
	case runErr == nil && result.Outcome != ForwardOutcomeSkipped:
		return result, nil
	case isRefusalExit && result.Outcome == ForwardOutcomeSkipped:
		return result, nil
	case runErr != nil:
		return nil, fmt.Errorf("Forward: Outcome %q on a failed exit: %w", result.Outcome, runErr)
	}
	return nil, fmt.Errorf("Forward: Outcome %q on exit 0", result.Outcome)
}

func IsAgentBranch(branch string) bool {
	return AgentBranchKind(branch) != ""
}

// Pick assumes the caller verified sha against a listed change; request input never reaches argv unverified.
func Pick(ctx context.Context, agentwtPath, onto, repoPath, sha string) (*PickResult, error) {
	output, runErr := shell.Run(ctx, repoPath, agentwtPath, agentwtArgs(repoPath, "pick", "--onto", onto, sha)...)
	return pickOutcome(output, runErr)
}

// Probe asks agentwt once what a pick of each sha onto the target's tip would do; nothing moves, an empty list asks nothing.
func Probe(ctx context.Context, agentwtPath, onto, repoPath string, shas []string) (*ProbeResult, error) {
	if len(shas) == 0 {
		return &ProbeResult{Commits: []ProbeEntry{}, Onto: onto, Schema: probeSchema}, nil
	}

	argv := agentwtArgs(repoPath, "probe", append([]string{"--onto", onto}, shas...)...)
	output, err := shell.Run(ctx, repoPath, agentwtPath, argv...)
	if err != nil {
		return nil, fmt.Errorf("Probe: %s: %w", strings.TrimSpace(output), err)
	}
	return decodeProbe(output)
}

// Remove leaves removed vs. skipped to the binary's safety predicate.
func Remove(ctx context.Context, branch string, force, deleteBranch bool, repoPath, agentwtPath string) (*OpOutput, error) {
	if err := ValidateBranch(ctx, branch, repoPath); err != nil {
		return nil, err
	}

	args := []string{"remove"}
	if deleteBranch {
		args = append(args, "--delete-branch")
	}
	if force {
		args = append(args, "--force")
	}
	args = append(args, branch)
	return runOp(ctx, repoPath, agentwtPath, args...)
}

// agentwtArgs is the argv of one agentwt command: the JSON format and the repo come first, then the command and its arguments.
func agentwtArgs(repoPath, command string, args ...string) []string {
	return append([]string{"--format", "json", "-C", repoPath, command}, args...)
}

func decodeForward(output string) (*ForwardResult, error) {
	start := strings.Index(output, "{")
	if start < 0 {
		return nil, errors.New("decodeForward: No document in output")
	}

	var result ForwardResult
	if err := json.NewDecoder(strings.NewReader(output[start:])).Decode(&result); err != nil {
		return nil, fmt.Errorf("decodeForward: Decode: %w", err)
	}
	if result.Schema != forwardSchema {
		return nil, fmt.Errorf("decodeForward: Unsupported schema %d", result.Schema)
	}

	switch result.Outcome {
	case ForwardOutcomeForwarded, ForwardOutcomeSkipped, ForwardOutcomeUnchanged:
		return &result, nil
	}
	return nil, fmt.Errorf("decodeForward: Unknown outcome %q", result.Outcome)
}

// decodePick starts at the first brace and stops after one object: stderr narration may surround it.
func decodePick(output string) (*PickResult, error) {
	start := strings.Index(output, "{")
	if start < 0 {
		return nil, errors.New("decodePick: No document in output")
	}

	var result PickResult
	if err := json.NewDecoder(strings.NewReader(output[start:])).Decode(&result); err != nil {
		return nil, fmt.Errorf("decodePick: Decode: %w", err)
	}
	if result.Schema != pickSchema {
		return nil, fmt.Errorf("decodePick: Unsupported schema %d", result.Schema)
	}

	switch result.Outcome {
	case PickOutcomeConflict, PickOutcomePicked, PickOutcomeSkipped:
		return &result, nil
	}
	return nil, fmt.Errorf("decodePick: Unknown outcome %q", result.Outcome)
}

// decodeProbe starts at the first brace and refuses an unknown schema or outcome rather than guess.
func decodeProbe(output string) (*ProbeResult, error) {
	start := strings.Index(output, "{")
	if start < 0 {
		return nil, errors.New("decodeProbe: No document in output")
	}

	var result ProbeResult
	if err := json.NewDecoder(strings.NewReader(output[start:])).Decode(&result); err != nil {
		return nil, fmt.Errorf("decodeProbe: Decode: %w", err)
	}
	if result.Schema != probeSchema {
		return nil, fmt.Errorf("decodeProbe: Unsupported schema %d", result.Schema)
	}

	for _, entry := range result.Commits {
		switch entry.Outcome {
		case ProbeOutcomeApplies, ProbeOutcomeConflict, ProbeOutcomeSkipped:
			continue
		}
		return nil, fmt.Errorf("decodeProbe: Commit %s: Unknown outcome %q", entry.Commit, entry.Outcome)
	}
	return &result, nil
}

// pickOutcome decides between a refusal document (exit 3) and a real failure.
func pickOutcome(output string, runErr error) (*PickResult, error) {
	result, err := decodePick(output)
	if err != nil {
		if runErr != nil {
			return nil, fmt.Errorf("pickOutcome: %s: %w", strings.TrimSpace(output), runErr)
		}
		return nil, fmt.Errorf("pickOutcome: %w", err)
	}

	var exitErr *exec.ExitError
	isRefusalExit := errors.As(runErr, &exitErr) && exitErr.ExitCode() == pickRefusalExit
	switch {
	case runErr == nil && !result.IsRefusal():
		return result, nil
	case isRefusalExit && result.IsRefusal():
		return result, nil
	case runErr != nil:
		return nil, fmt.Errorf("pickOutcome: Outcome %q on a failed exit: %w", result.Outcome, runErr)
	}
	return nil, fmt.Errorf("pickOutcome: Outcome %q on exit 0", result.Outcome)
}

// runOp returns the combined output on a non-zero exit: refusals print their reason on stderr.
func runOp(ctx context.Context, repoPath, agentwtPath string, args ...string) (*OpOutput, error) {
	argv := agentwtArgs(repoPath, args[0], args[1:]...)
	output, err := shell.Run(ctx, repoPath, agentwtPath, argv...)
	if err != nil {
		result := &OpOutput{Text: strings.TrimSpace(output)}
		return result, fmt.Errorf("runOp: %s: %w", args[0], err)
	}

	var decoded opDocument
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		return nil, fmt.Errorf("runOp: %s: Decode: %w", args[0], err)
	}
	texts := make([]string, 0, len(decoded.Lines))
	for _, line := range decoded.Lines {
		texts = append(texts, line.Text)
	}
	result := &OpOutput{Lines: decoded.Lines, Text: strings.Join(texts, "\n")}
	return result, nil
}

// ValidateBranch guards every branch operation: request input never reaches an argv unverified.
func ValidateBranch(ctx context.Context, branch, repoPath string) error {
	// Branch: agent family only (claude/<x>, claude-<kind>/<x>)
	if !IsAgentBranch(branch) {
		return fmt.Errorf("ValidateBranch: Not an agent branch: %q", branch)
	}

	// Branch: must exist as a local ref in this repo
	if _, err := shell.Run(ctx, repoPath, "git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err != nil {
		return fmt.Errorf("ValidateBranch: Unknown branch %q: %w", branch, err)
	}

	return nil
}

package acdsl

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// VerdictRecord archives one gate run — the raw material of the standing
// ablation loop: per-rule first-pass violation rates from real sessions,
// not probe harnesses.
type VerdictRecord struct {
	Ts          string          `json:"ts"` // RFC3339 UTC — lexicographic order is time order
	Root        string          `json:"root"`
	Branch      string          `json:"branch"`
	Session     string          `json:"session,omitempty"`
	Outcome     string          `json:"outcome"` // clean | violations
	Rules       []RuleVerdict   `json:"rules"`
	Diagnostics []DiagnosticRef `json:"diagnostics,omitempty"`
}

// RuleVerdict is one rule's slice of a run: its delivery flag at run time
// and how many violations its verifier reported. Zero-violation rows are
// deliberate — they are the rate denominator.
type RuleVerdict struct {
	Id         string `json:"id"`
	Projected  bool   `json:"projected"`
	Violations int    `json:"violations"`
}

// DiagnosticRef preserves the finding text for later analysis without
// duplicating the rule's why.
type DiagnosticRef struct {
	Id      string `json:"id"`
	Message string `json:"message"`
}

// DefaultVerdictsPath resolves the log sink: $ACDSL_VERDICTS_PATH, else
// ~/.claude/acdsl/verdicts.jsonl — home-anchored because pool worktrees are
// destroyed with their sessions and the loop's data must outlive them.
func DefaultVerdictsPath() (string, error) {
	if fromEnv := os.Getenv("ACDSL_VERDICTS_PATH"); fromEnv != "" {
		return fromEnv, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("DefaultVerdictsPath: %w", err)
	}
	return filepath.Join(home, ".claude", "acdsl", "verdicts.jsonl"), nil
}

// BuildVerdictRecord rolls a check run into its archive record.
func BuildVerdictRecord(ts, root, branch, session string, rules []Rule, diagnostics []Diagnostic) VerdictRecord {
	perRule := map[string]int{}
	for _, diagnostic := range diagnostics {
		perRule[diagnostic.RuleId]++
	}
	record := VerdictRecord{Ts: ts, Root: root, Branch: branch, Session: session, Outcome: "clean"}
	for _, rule := range rules {
		record.Rules = append(record.Rules, RuleVerdict{Id: rule.Id, Projected: rule.Projected, Violations: perRule[rule.Id]})
	}
	for _, diagnostic := range diagnostics {
		record.Diagnostics = append(record.Diagnostics, DiagnosticRef{Id: diagnostic.RuleId, Message: diagnostic.Message})
	}
	if len(diagnostics) > 0 {
		record.Outcome = "violations"
	}
	return record
}

// AppendVerdict appends one JSONL record to path, creating the directory on
// first use. The caller treats failure as a warning: the gate's exit code
// never depends on logging.
func AppendVerdict(path string, record VerdictRecord) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("AppendVerdict: %w", err)
	}
	line, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("AppendVerdict: %w", err)
	}
	sink, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("AppendVerdict: %w", err)
	}
	defer sink.Close()
	if _, err := sink.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("AppendVerdict: %w", err)
	}
	return nil
}

// ReadVerdicts loads the JSONL sink. A missing file is an empty log, not an
// error; unparseable lines are skipped and counted so drift stays visible
// without ever being fatal. since (RFC3339, empty = all) filters by Ts.
func ReadVerdicts(path, since string) ([]VerdictRecord, int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, 0, fmt.Errorf("ReadVerdicts: %w", err)
	}
	var records []VerdictRecord
	skipped := 0
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		var record VerdictRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			skipped++
			continue
		}
		if since != "" && record.Ts < since {
			continue
		}
		records = append(records, record)
	}
	return records, skipped, nil
}

// RuleStat aggregates one (rule, projected) slice of the verdict log — the
// eviction decision reads exactly this table. A rule flipped mid-window
// yields two rows: the A/B comparison the loop exists for.
type RuleStat struct {
	Id         string
	Projected  bool
	Runs       int
	RedRuns    int
	Violations int
	LastRed    string
}

// verdictKey identifies one (rule, projected) aggregation bucket.
type verdictKey struct {
	id        string
	projected bool
}

// AggregateVerdicts folds records into per-(rule, projected) stats, sorted
// by id, projected=true first.
func AggregateVerdicts(records []VerdictRecord) []RuleStat {
	stats := map[verdictKey]*RuleStat{}
	for _, record := range records {
		for _, verdict := range record.Rules {
			k := verdictKey{verdict.Id, verdict.Projected}
			stat, ok := stats[k]
			if !ok {
				stat = &RuleStat{Id: verdict.Id, Projected: verdict.Projected}
				stats[k] = stat
			}
			stat.Runs++
			if verdict.Violations > 0 {
				stat.RedRuns++
				stat.Violations += verdict.Violations
				if record.Ts > stat.LastRed {
					stat.LastRed = record.Ts
				}
			}
		}
	}
	out := make([]RuleStat, 0, len(stats))
	for _, stat := range stats {
		out = append(out, *stat)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Id != out[j].Id {
			return out[i].Id < out[j].Id
		}
		return out[i].Projected && !out[j].Projected
	})
	return out
}

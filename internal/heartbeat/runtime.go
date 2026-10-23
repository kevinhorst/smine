package heartbeat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kevinhorst/smine/internal/shell"
)

// runtimeSchema is the runtime document version this decoder reads.
const runtimeSchema = 1

// Mirrors runtime-v1 FindingRow.
type Finding struct {
	Id       string   `json:"id"`
	Acted    string   `json:"acted"`
	Entry    string   `json:"entry"`
	Evidence []string `json:"evidence"`
	Kind     string   `json:"kind"`
	Moves    []string `json:"moves"`
	Reason   string   `json:"reason"`
	Run      uint64   `json:"run"`
	Session  string   `json:"session"`
	Skill    string   `json:"skill"`
	Title    string   `json:"title"`
	Version  string   `json:"version"`
}

// AnchorHit is one runtime-v1 AnchorHitRow: a successful anchored call.
type AnchorHit struct {
	Id       string                     `json:"id"`
	Captures map[string]json.RawMessage `json:"captures"`
	Seq      uint64                     `json:"seq"`
}

// Mirrors runtime-v1 CheckFailureRow.
type CheckFailure struct {
	Check string `json:"check"`
	Count int    `json:"count"`
}

// Mirrors runtime-v1 CheckPassRow: a check that held with its guard matched.
type CheckPass struct {
	Check string `json:"check"`
	Count int    `json:"count"`
}

// Run is one runtime-v1 RunRow: an open, ended or killed enforced skill run.
type Run struct {
	Id       uint64         `json:"id"`
	Anchors  []AnchorHit    `json:"anchors"`
	Failures []CheckFailure `json:"failures"`
	Findings int            `json:"findings"`
	Passes   []CheckPass    `json:"passes"`
	Reason   string         `json:"reason"`
	Session  string         `json:"session"`
	Skill    string         `json:"skill"`
	State    string         `json:"state"`
	Title    string         `json:"title"`
	Version  string         `json:"version"`
}

// Mirrors cmd/heartbeat/schema/runtime-v1.json.
type RuntimeDocument struct {
	Findings      []Finding             `json:"findings"`
	Heartbeat     Heartbeat             `json:"heartbeat"`
	Runs          []Run                 `json:"runs"`
	Schema        int                   `json:"schema"`
	Subscriptions []RuntimeSubscription `json:"subscriptions"`
}

// RuntimeLastDigest is the runtime subscriber's latest digest and its fate.
type RuntimeLastDigest struct {
	Id      string `json:"id"`
	At      string `json:"at"`
	Outcome string `json:"outcome"`
}

// A subscriber session bound to exactly one observed session.
type RuntimeSubscription struct {
	LastDigest  *RuntimeLastDigest `json:"last_digest"`
	Mode        string             `json:"mode"`
	Session     string             `json:"session"`
	Since       string             `json:"since"`
	Target      string             `json:"target"`
	TargetTitle string             `json:"target_title"`
	Title       string             `json:"title"`
}

// A schema other than runtimeSchema is a hard error.
func (c *Cli) Runtime(ctx context.Context) (*RuntimeDocument, error) {
	args := c.args("runtime", "status", "--format", "json")
	output, err := shell.Run(ctx, "", c.BinaryPath, args...)
	if err != nil {
		return nil, classifyRunError(output, err)
	}

	document := &RuntimeDocument{}
	if err := json.Unmarshal([]byte(output), document); err != nil {
		return nil, fmt.Errorf("Cli.Runtime: Failed to decode: %w", err)
	}

	if document.Schema != runtimeSchema {
		return nil, fmt.Errorf("Cli.Runtime: Unsupported schema %d", document.Schema)
	}
	return document, nil
}

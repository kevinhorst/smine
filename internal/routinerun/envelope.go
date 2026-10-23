package routinerun

import (
	"bytes"
	"encoding/json"
)

// Envelope carries the same fields --output-format json prints.
type Envelope struct {
	IsError      bool    `json:"is_error"`
	NumTurns     int     `json:"num_turns"`
	Result       string  `json:"result"`
	SessionId    string  `json:"session_id"`
	Subtype      string  `json:"subtype"`
	TotalCostUsd float64 `json:"total_cost_usd"`
}

// isFailure catches an agent that failed while the CLI exited 0.
func (e *Envelope) isFailure() bool {
	hasFailedSubtype := e.Subtype != "" && e.Subtype != "success"
	return e.IsError || hasFailedSubtype
}

// parseEnvelope yields the zero envelope for a killed cell's empty or malformed output.
func parseEnvelope(raw []byte) *Envelope {
	envelope := &Envelope{}
	if err := json.Unmarshal(bytes.TrimSpace(raw), envelope); err != nil {
		return &Envelope{}
	}
	return envelope
}

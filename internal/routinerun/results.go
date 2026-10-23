package routinerun

import (
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/kevinhorst/smine/internal/routines"
)

const resultPreviewRunes = 300

// appendResult never fails the run: the log is an observer.
func appendResult(path string, row *routines.RunResult) {
	line, err := json.Marshal(row)
	if err != nil {
		log.Printf("appendResult: %s: Failed to encode row: %v", path, err)
		return
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("appendResult: %s: Failed to open: %v", path, err)
		return
	}
	defer file.Close()

	if _, err := file.Write(append(line, '\n')); err != nil {
		log.Printf("appendResult: %s: Failed to write: %v", path, err)
	}
}

func failureRow(status int, reason string) *routines.RunResult {
	return &routines.RunResult{
		ExitStatus: status,
		IsError:    true,
		Result:     truncateRunes(reason, resultPreviewRunes),
		Subtype:    "error",
		Timestamp:  utcStamp(time.Now()),
	}
}

func resultRow(envelope *Envelope, item, branch, stage string, status int) *routines.RunResult {
	return &routines.RunResult{
		Branch:       branch,
		ExitStatus:   status,
		IsError:      envelope.IsError,
		Item:         item,
		NumTurns:     envelope.NumTurns,
		Result:       truncateRunes(envelope.Result, resultPreviewRunes),
		SessionId:    envelope.SessionId,
		Stage:        stage,
		Subtype:      envelope.Subtype,
		Timestamp:    utcStamp(time.Now()),
		TotalCostUsd: envelope.TotalCostUsd,
	}
}

func startedRow(item, branch string) *routines.RunResult {
	return &routines.RunResult{
		Branch:    branch,
		Item:      item,
		Result:    "run started",
		Subtype:   routines.SubtypeStarted,
		Timestamp: utcStamp(time.Now()),
	}
}

// summaryRow is the row of an agent-free run that did work.
func summaryRow(summary string) *routines.RunResult {
	return &routines.RunResult{
		Result:    truncateRunes(summary, resultPreviewRunes),
		Subtype:   budgetSummarySubtype,
		Timestamp: utcStamp(time.Now()),
	}
}

func truncateRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}

func utcStamp(moment time.Time) string {
	return moment.UTC().Format("2006-01-02T15:04:05Z")
}

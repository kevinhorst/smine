package main

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/contextrequired"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/readledger"
)

type Injection struct {
	agentId    string
	claudeHome string
	entries    []contextdocs.RuleEntry
	hookConfig *hookconfig.Config
	lane       string
	sessionId  string
	skill      string
	version    string
}

func (i *Injection) Render() (string, error) {
	section := &i.hookConfig.Skillcontext
	ledger := i.ledger()
	seen, err := seenIds(ledger)
	if err != nil {
		return "", fmt.Errorf("Injection.Render: %w", err)
	}

	delta, repeat := split(i.entries, seen)
	head := header(delta, i.lane, repeat, section, i.skill, i.version)
	if len(delta) == 0 {
		return head, nil
	}

	bodies := contextdocs.RenderBodies(delta)
	text := head + "\n\n" + bodies
	isInline := ledger == nil || utf8.RuneCountInString(text) <= section.InlineCap
	if isInline {
		return text, i.record(delta, "", ledger, readledger.LineRange{})
	}

	path := i.filePath()
	lines, err := appendLines(bodies, path)
	if err != nil {
		return "", fmt.Errorf("Injection.Render: %w", err)
	}

	instructions := contextrequired.ReadInstructions(lines.End, path, lines.Start, i.hookConfig.Globalcontext.ReadBudget)
	pointer := head + "\n" + fmt.Sprintf(section.PointerFormat, path, lines.Start, lines.End, utf8.RuneCountInString(bodies)) +
		"\n" + strings.Join(instructions, "\n")
	return pointer, i.record(delta, path, ledger, lines)
}

func (i *Injection) filePath() string {
	name := i.skill
	if i.lane != "" {
		name += "--" + strings.ReplaceAll(i.lane, ":", "-")
	}

	name += "--" + cmp.Or(i.agentId, readledger.MainAgent) + ".md"
	return filepath.Join(contextrequired.Dir(i.claudeHome, i.sessionId), name)
}

func (i *Injection) ledger() *readledger.Ledger {
	if i.sessionId == "" {
		return nil
	}
	return readledger.New(i.agentId, i.claudeHome, i.hookConfig.Readgate.LedgerRetentionMs.Duration(), i.sessionId)
}

// record appends the delivered ids to the ledger; nil ledger records nothing.
func (i *Injection) record(delta []contextdocs.RuleEntry, file string, ledger *readledger.Ledger, lines readledger.LineRange) error {
	if ledger == nil {
		return nil
	}
	return ledger.RecordContext(lines.End, file, entryIds(delta), lines.Start)
}

func appendLines(bodies string, path string) (readledger.LineRange, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return readledger.LineRange{}, fmt.Errorf("appendLines: Failed to create %s: %w", filepath.Dir(path), err)
	}

	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return readledger.LineRange{}, fmt.Errorf("appendLines: Failed to read %s: %w", path, err)
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return readledger.LineRange{}, fmt.Errorf("appendLines: Failed to open %s: %w", path, err)
	}

	defer file.Close()
	if _, err := file.WriteString(bodies + "\n"); err != nil {
		return readledger.LineRange{}, fmt.Errorf("appendLines: Failed to write %s: %w", path, err)
	}

	start := bytes.Count(existing, []byte("\n")) + 1
	lines := readledger.LineRange{End: start + strings.Count(bodies, "\n"), Start: start}
	return lines, nil
}

func seenIds(ledger *readledger.Ledger) ([]string, error) {
	if ledger == nil {
		return nil, nil
	}

	records, err := ledger.Load()
	if err != nil {
		return nil, fmt.Errorf("seenIds: %w", err)
	}
	return readledger.SeenIds(records), nil
}

// split assumes seen is sorted.
func split(entries []contextdocs.RuleEntry, seen []string) ([]contextdocs.RuleEntry, []contextdocs.RuleEntry) {
	delta := make([]contextdocs.RuleEntry, 0, len(entries))
	repeat := make([]contextdocs.RuleEntry, 0)
	for _, entry := range entries {
		if _, isSeen := slices.BinarySearch(seen, entry.Id); isSeen {
			repeat = append(repeat, entry)
			continue
		}
		delta = append(delta, entry)
	}
	return delta, repeat
}

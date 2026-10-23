package server

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kevinhorst/smine/internal/contextrecord"
)

const (
	// cellBadgeDim is a token cell's dim badge classes.
	cellBadgeDim = "badge " + badgeDim
	// coverageComplete and coveragePartial are the extractor's guide verdicts.
	coverageComplete = "complete"
	coveragePartial  = "partial"
	// pendingNoticeFormat's first two numbers are the guard's.
	pendingNoticeFormat = "Token counting is throttled — %d API calls per minute, %d injections per page load. %d injections pending: reload to continue."
	tagKindGate         = "gate"
	tagKindHook         = "hook"
	tagKindScript       = "script"
	tagKindTool         = "tool"
	tagTextAgent        = "Agent"
	wordCountFailed     = "count failed"
	wordOff             = "off"
	wordPending         = "pending"
	wordSkipped         = "skipped · too large"
)

// coverageBadge is the language-guide coverage rendered as words.
type coverageBadge struct {
	Class string
	Text  string
	Title string
}

// mechanismTag names the injector of a kind in the Graph tab's tag classes; Warn is the hint of a feature that was off.
type mechanismTag struct {
	Class string
	Text  string
	Warn  string
}

type pathCell struct {
	Prefix string
	Rel    string
}

// String is the full path, for titles and the markdown.
func (p pathCell) String() string {
	return p.Prefix + p.Rel
}

type sectionSize struct {
	Chars   string
	Pending string
	Tokens  string
}

// String is the one-line form the markdown headings carry.
func (s sectionSize) String() string {
	if s.Chars == "" {
		return ""
	}
	if s.Pending == "" {
		return s.Chars + " · " + s.Tokens
	}
	return fmt.Sprintf("%s · %s so far · %s", s.Chars, s.Tokens, s.Pending)
}

type tokenCell struct {
	Class string
	Text  string
	Title string
}

// isPending reports an item the counter has not reached yet.
func isPending(item *contextrecord.Item) bool {
	if item.Counter != "" {
		return false
	}

	switch item.Uncounted {
	case contextrecord.UncountedCap, contextrecord.UncountedQueued, contextrecord.UncountedRate:
		return true
	}
	return false
}

// itemTokenCell renders one item's count or its state in words.
func itemTokenCell(item *contextrecord.Item) tokenCell {
	switch {
	case item.Counter != "" && item.Approx:
		return tokenCell{Text: "≈" + groupThousands(int64(item.Tokens))}
	case item.Counter != "":
		return tokenCell{Text: groupThousands(int64(item.Tokens))}
	case item.Error != "":
		return tokenCell{Class: cellBadgeDim, Text: wordCountFailed, Title: item.Error}
	}
	return pendingCell(item.Uncounted)
}

// kindTag uses the hook's own name as the tag text for hook context.
func kindTag(kind contextrecord.Kind, label string) mechanismTag {
	switch kind {
	case contextrecord.KindGlobal:
		return mechanismTag{Class: tagKindHook, Text: "global-context"}
	case contextrecord.KindSkill:
		return mechanismTag{Class: tagKindHook, Text: "skill-context"}
	case contextrecord.KindLang:
		return mechanismTag{Class: tagKindGate, Text: "read-gate"}
	case contextrecord.KindAcdslProjection:
		return mechanismTag{Class: tagKindGate, Text: layerAcdsl}
	case contextrecord.KindAcdslContext:
		return mechanismTag{Class: tagKindHook, Text: "acdsl-context"}
	case contextrecord.KindPlanRules:
		return mechanismTag{Class: tagKindScript, Text: "acdsl project -plan"}
	case contextrecord.KindPlanContext:
		return mechanismTag{Class: tagKindHook, Text: "planmode-context"}
	case contextrecord.KindPackRead:
		return mechanismTag{Class: tagKindTool, Text: "Read"}
	case contextrecord.KindAgentsMd:
		return mechanismTag{Class: tagKindHook, Text: "harness"}
	case contextrecord.KindHookContext:
		return mechanismTag{Class: tagKindHook, Text: label}
	case contextrecord.KindSubagentPrompt:
		return mechanismTag{Class: tagKindTool, Text: tagTextAgent}
	case contextrecord.KindCodexInstructions, contextrecord.KindCodexEnvironment:
		return mechanismTag{Class: tagKindHook, Text: "codex"}
	}
	return mechanismTag{Class: tagKindHook, Text: string(kind)}
}

func pendingCell(reason string) tokenCell {
	switch reason {
	case contextrecord.UncountedRate:
		return tokenCell{Class: cellBadgeDim, Text: wordPending, Title: "rate limit — reload to count"}
	case contextrecord.UncountedCap:
		return tokenCell{Class: cellBadgeDim, Text: wordPending, Title: "load cap — reload to count"}
	case contextrecord.UncountedQueued:
		return tokenCell{Class: cellBadgeDim, Text: wordPending, Title: "not counted yet — a background run counts it"}
	case contextrecord.UncountedSize:
		return tokenCell{Class: cellBadgeDim, Text: wordSkipped, Title: "over 200 000 characters"}
	case contextrecord.UncountedDisabled:
		return tokenCell{Class: cellBadgeDim, Text: wordOff, Title: "token counting is off"}
	}
	return tokenCell{Text: "—"}
}

// pendingCount counts the pending items of a slice.
func pendingCount(items []contextrecord.Item) int {
	count := 0
	for index := range items {
		if isPending(&items[index]) {
			count++
		}
	}
	return count
}

func relativePath(path, cwd, claudeHome string) pathCell {
	for _, base := range []string{cwd, claudeHome} {
		if base == "" {
			continue
		}
		prefix := filepath.Clean(base) + string(filepath.Separator)
		if rel, found := strings.CutPrefix(path, prefix); found {
			return pathCell{Prefix: prefix, Rel: rel}
		}
	}
	return pathCell{Rel: path}
}

func sizeBadge(totals *contextrecord.Totals, pending int) sectionSize {
	size := sectionSize{
		Chars:  groupThousands(int64(totals.Chars)) + " chars",
		Tokens: groupThousands(int64(totals.Tokens)) + " tokens",
	}
	if pending > 0 {
		size.Pending = fmt.Sprintf("%d pending", pending)
	}
	return size
}

func totalsTokenCell(totals *contextrecord.Totals) string {
	if totals.Tokens == 0 && totals.Uncounted == totals.Items {
		return "—"
	}
	return groupThousands(int64(totals.Tokens))
}

package contextrecord

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/kevinhorst/smine/internal/acdsl"
)

const (
	agentsMdName       = "AGENTS.md"
	contextRequiredDir = "context-required"
	frontMatterFence   = "---"
	frontMatterYamlEnd = "..."
	globalBanner       = "GLOBAL CONTEXT (injected by hook)"
	globalInjection    = "global.md"
	labelMaxRunes      = 80
	laneSeparator      = "--"
	markdownSuffix     = ".md"
	planModeInjection  = "planmode.md"
	prefixAcdslContext = "ACDSL rules governing "
	prefixAgentsMd     = "Contents of "
	prefixGlobal       = "===== GLOBAL CONTEXT"
	prefixPlanContext  = "===== PLAN CONTEXT"
	prefixSkill        = "===== SKILL CONTEXT /"
	previewMaxRunes    = 40
	rulesDir           = "rules"
)

var (
	globalMarkerPattern = regexp.MustCompile(`===== ([^=\n]+) =====`)
	langGuidePattern    = regexp.MustCompile(`/rules/([a-z]+)(?:_[a-z]+)?\.md$`)
	skillNamePattern    = regexp.MustCompile(`SKILL CONTEXT /([^ \n]+)(?: (?:lane|topic):[^ \n]+)?(?: v([^ \n]+))?`)

	// Hooks whose additionalContext carries no marker, by first line; the order is the match order.
	hookLabels = []hookLabelRule{
		{label: "sessiontitle", prefix: "Session title:"},
		{label: "tool-policy", prefix: "tool-policy"},
		{label: "worktree", prefix: "You are operating in an isolated workspace"},
		{label: "heartbeat", prefix: "heartbeat"},
	}
)

// hookEnvelope is the JSON form a hook writes on PostToolUse.
type hookEnvelope struct {
	HookSpecificOutput hookOutput `json:"hookSpecificOutput"`
}

// hookLabelRule maps a hook's lead text to the hook's name.
type hookLabelRule struct {
	label  string
	prefix string
}

type hookOutput struct {
	AdditionalContext string `json:"additionalContext"`
}

// Anything unrecognized is hook context labeled by the hook it came from.
func classifyHook(text, hookName string) (Kind, string, string) {
	first := firstLine(text)
	switch {
	case strings.HasPrefix(text, prefixGlobal):
		return KindGlobal, hookName, ""
	case strings.HasPrefix(text, prefixSkill):
		skill, version := skillFromHook(text)
		return KindSkill, skill, version
	case strings.HasPrefix(text, prefixPlanContext):
		return KindPlanContext, hookName, ""
	case strings.HasPrefix(text, prefixAcdslContext):
		return KindAcdslContext, strings.TrimSuffix(strings.TrimPrefix(first, prefixAcdslContext), ":"), ""
	case strings.HasPrefix(text, prefixAgentsMd) && strings.Contains(first, agentsMdName):
		return KindAgentsMd, strings.TrimSuffix(strings.TrimPrefix(first, prefixAgentsMd), ":"), ""
	}
	return KindHookContext, hookLabel(first, hookName), ""
}

// global.md is the global file, planmode.md the plan-mode one, any other a skill's.
func classifyInjection(path string) (Kind, string, bool) {
	directory := filepath.Base(filepath.Dir(filepath.Dir(path)))
	if directory != contextRequiredDir {
		return "", "", false
	}
	base := filepath.Base(path)
	switch base {
	case globalInjection:
		return KindGlobal, path, true
	case planModeInjection:
		return KindPlanContext, path, true
	}
	return KindSkill, skillFromInjection(base), true
}

// The script's subagent path scan.
func contextPaths(prompt, ctxDir string) []string {
	pattern := regexp.MustCompile(`[^ ` + "`" + `"\n]*` + regexp.QuoteMeta(ctxDir) + `/[^ ` + "`" + `"\n]+`)
	return uniqueSorted(pattern.FindAllString(prompt, -1))
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return line
}

// globalMarkers lists the per-file headers of an inline global context.
func globalMarkers(text string) []string {
	var files []string
	for _, marker := range submatches(globalMarkerPattern, text) {
		if marker != globalBanner {
			files = append(files, marker)
		}
	}
	return files
}

func hookLabel(first, hookName string) string {
	for _, rule := range hookLabels {
		if strings.HasPrefix(first, rule.prefix) {
			return rule.label
		}
	}
	if hookName != "" {
		return hookName
	}
	return truncateLabel(first, labelMaxRunes)
}

// ids extracts every context entry or rule id, sorted and unique.
func ids(text string) []string {
	return uniqueSorted(idPattern.FindAllString(text, -1))
}

// langOf reads the language of a guide path (…/rules/go.md → go).
func langOf(path string) (string, bool) {
	match := langGuidePattern.FindStringSubmatch(path)
	if match == nil {
		return "", false
	}
	return match[1], true
}

// cat -n numbering is kept since the model read it; only a head-of-file marker is an injection.
func projectionBlock(text string) (string, bool) {
	lines := strings.Split(text, "\n")
	start, isHead := projectionHead(lines)
	if !isHead {
		return "", false
	}
	end := len(lines)
	for index := start + 1; index < len(lines); index++ {
		if isBlankResultLine(lines[index]) {
			end = index
			break
		}
	}
	return strings.Join(lines[start:end], "\n"), true
}

// A result that does not start at the file's first line cannot hold the block.
func projectionHead(lines []string) (int, bool) {
	if len(lines) == 0 || resultLineNumber(lines[0]) != 1 {
		return 0, false
	}

	head := 0
	first := resultLineContent(lines[0])
	switch {
	case strings.HasPrefix(first, "#!"):
		head = 1
	case first == frontMatterFence:
		head = frontMatterEnd(lines)
	}
	if head >= len(lines) {
		return 0, false
	}
	return head, strings.Contains(lines[head], acdsl.ProjectionMarker)
}

// 0 when the front-matter block never closes.
func frontMatterEnd(lines []string) int {
	for index := 1; index < len(lines); index++ {
		content := resultLineContent(lines[index])
		if content == frontMatterFence || content == frontMatterYamlEnd {
			return index + 1
		}
	}
	return 0
}

// resultLineContent is a cat -n line's content after its number and tab.
func resultLineContent(line string) string {
	_, content, _ := strings.Cut(line, "\t")
	return content
}

// resultLineNumber is a cat -n line's number, 0 for a line without one.
func resultLineNumber(line string) int {
	match := numberedLinePattern.FindStringSubmatch(line)
	if match == nil {
		return 0
	}

	number, err := strconv.Atoi(match[1])
	if err != nil {
		return 0
	}
	return number
}

func isBlankResultLine(line string) bool {
	_, content, hasNumber := strings.Cut(line, "\t")
	if hasNumber {
		return strings.TrimSpace(content) == ""
	}
	return strings.TrimSpace(line) == ""
}

func skillFromHook(text string) (string, string) {
	match := skillNamePattern.FindStringSubmatch(text)
	if match == nil {
		return "", ""
	}
	return match[1], match[2]
}

// fdesign--main.md → fdesign, railroad-review--lane-correctness--a1.md → railroad-review.
func skillFromInjection(base string) string {
	name := strings.TrimSuffix(base, markdownSuffix)
	name, _, _ = strings.Cut(name, laneSeparator)
	return name
}

func truncateLabel(text string, maxRunes int) string {
	if utf8.RuneCountInString(text) <= maxRunes {
		return text
	}
	runes := []rune(text)
	return string(runes[:maxRunes]) + "…"
}

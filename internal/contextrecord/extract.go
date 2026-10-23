package contextrecord

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/transcript"
)

const (
	agentPrefix      = "agent-"
	defaultReadLimit = 2000
	coverageComplete = "complete"
	coveragePartial  = "partial"
	metaSuffix       = ".meta.json"
	planUseMarker    = "acdsl project -plan"
	rootAgentId      = "main"
	subagentsDir     = "subagents"
	syntheticModel   = "<synthetic>"
	toolAgent        = "Agent"
	toolBash         = "Bash"
	toolEdit         = "Edit"
	toolMultiEdit    = "MultiEdit"
	toolNotebookEdit = "NotebookEdit"
	toolRead         = "Read"
	toolWrite        = "Write"
	transcriptSuffix = ".jsonl"
	workflowsDir     = "workflows"
)

var (
	idPattern             = regexp.MustCompile(`\b[A-Z][A-Z0-9]*(?:-[A-Z][A-Z0-9]*)+-[0-9]{3}\b`)
	numberedLinePattern   = regexp.MustCompile(`^\s*([0-9]+)\t(.*)$`)
	planRulePattern       = regexp.MustCompile(`- \[([A-Z0-9-]+)\]`)
	projectionRulePattern = regexp.MustCompile(`- \[(ACDSL-[A-Z0-9-]+)\]`)
)

// agentMeta is the harness's agent-<id>.meta.json beside a subagent transcript.
type agentMeta struct {
	AgentType   string `json:"agentType"`
	Description string `json:"description"`
	Model       string `json:"model"`
	SpawnDepth  int    `json:"spawnDepth"`
}

type readNumbers struct {
	count       int
	isLastEmpty bool
	lastLine    int
	startLine   int
}

// A Read ending at the file's end numbers the empty line after the final newline; it is no file line.
func (r readNumbers) fileEnd(limit int) int {
	isAtFileEnd := r.count < limit
	if r.isLastEmpty && isAtFileEnd && r.lastLine > r.startLine {
		return r.lastLine - 1
	}
	return r.lastLine
}

// langReads accumulates one guide's chunked Reads into its lang row.
type langReads struct {
	guide string
	ids   []string
	lang  string
}

type transcriptIdentity struct {
	cwd       string
	gitBranch string
	sessionId string
}

type walk struct {
	agent           *Agent
	context         *AgentContext
	ctxDir          string
	cwd             string
	files           map[string]*FileTouch
	hookTexts       map[string]bool
	langs           map[string]*langReads
	transcriptModel string
	uses            map[string]transcript.Block
}

func newWalk(agent *Agent, ctxDir string) *walk {
	agent.Context = &AgentContext{}
	agent.Items = []Item{}
	w := &walk{
		agent:     agent,
		context:   agent.Context,
		ctxDir:    ctxDir,
		files:     make(map[string]*FileTouch),
		hookTexts: make(map[string]bool),
		langs:     make(map[string]*langReads),
		uses:      make(map[string]transcript.Block),
	}
	return w
}

// addHook classifies one hook text, deduplicated by exact text (D3).
func (w *walk) addHook(hookName, source, text string) {
	if text == "" || w.hookTexts[text] {
		return
	}
	w.hookTexts[text] = true

	kind, label, version := classifyHook(text, hookName)
	item := NewItem(kind, label, source, text)
	if kind == KindHookContext {
		item.Preview = truncateLabel(firstLine(text), previewMaxRunes)
	}
	w.agent.add(item)
	switch kind {
	case KindGlobal:
		w.context.Injected.Global = append(w.context.Injected.Global, globalMarkers(text)...)
	case KindSkill:
		skill := SkillContext{Ids: ids(text), Skill: label, Version: version, Via: "marker"}
		w.context.Injected.Skill = append(w.context.Injected.Skill, skill)
	case KindAcdslContext, KindAcdslProjection, KindAgentsMd, KindCodexEnvironment, KindCodexInstructions,
		KindHookContext, KindLang, KindPackRead, KindPlanContext, KindPlanRules, KindSubagentPrompt:
	}
}

// Classified by the path the tool was asked for.
func (w *walk) addRead(use *transcript.Block, text string) {
	path := use.Input.FilePath
	if injectionKind, label, isInjection := classifyInjection(path); isInjection {
		w.agent.add(newReadItem(injectionKind, label, use, text))
		if injectionKind == KindGlobal {
			w.context.Injected.Global = append(w.context.Injected.Global, path)
		}
		return
	}

	isPackRead := w.cwd != "" && strings.HasPrefix(path, filepath.Join(w.cwd, w.ctxDir)+string(filepath.Separator))
	if lang, isGuide := langOf(path); isGuide {
		w.agent.add(newReadItem(KindLang, path, use, text))
		w.recordGuideRead(lang, path, text)
	} else if isPackRead {
		w.agent.add(newReadItem(KindPackRead, path, use, text))
	}
	if isPackRead {
		w.context.PackReads = append(w.context.PackReads, path)
	}
}

// A projection block is measured only in a successful Read result.
func (w *walk) addResult(block *transcript.Block) {
	use, isKnown := w.uses[block.ToolUseId]
	text := block.Content.Joined()
	if isKnown && use.Name == toolBash && strings.Contains(use.Input.Command, planUseMarker) {
		w.agent.add(NewItem(KindPlanRules, firstLine(use.Input.Command), SourceToolResult, text))
		w.context.PlanRules = append(w.context.PlanRules, submatches(planRulePattern, text)...)
	}

	isFileRead := isKnown && use.Name == toolRead && !block.IsError && use.Input.FilePath != ""
	if !isFileRead {
		return
	}
	w.addRead(&use, text)
	w.addProjection(use.Input.FilePath, text)
}

// Only the block at the head of the file counts; any other marker in the text is not one.
func (w *walk) addProjection(file, text string) {
	projection, hasProjection := projectionBlock(text)
	if !hasProjection {
		return
	}

	w.agent.add(NewItem(KindAcdslProjection, file, SourceToolResult, projection))
	rules := FileRules{File: file, Ids: uniqueSorted(submatches(projectionRulePattern, projection))}
	w.context.AcdslRules = append(w.context.AcdslRules, rules)
}

// addUse remembers a tool_use for its result and measures an Agent prompt.
func (w *walk) addUse(block *transcript.Block) {
	w.uses[block.Id] = *block
	w.touchFile(block)
	if block.Name != toolAgent {
		return
	}

	label := block.Input.Description
	if label == "" {
		label = toolAgent
	}
	w.agent.add(NewItem(KindSubagentPrompt, label, SourceToolUse, block.Input.Prompt))
	entry := SubagentContext{Ids: ids(block.Input.Prompt), Paths: contextPaths(block.Input.Prompt, w.ctxDir)}
	w.context.SubagentContext = append(w.context.SubagentContext, entry)
}

// The assistant lines' model wins over meta.json's, which is the spawn alias, not a model id.
func (w *walk) finish() {
	if w.transcriptModel != "" {
		w.agent.Model = w.transcriptModel
	}
	for _, guide := range slices.Sorted(maps.Keys(w.langs)) {
		reads := w.langs[guide]
		row := LangContext{
			Guide: guide,
			Ids:   uniqueSorted(reads.ids),
			Lang:  reads.lang,
		}
		w.context.Injected.Lang = append(w.context.Injected.Lang, row)
	}
	w.context.PackReads = uniqueSorted(w.context.PackReads)
	w.context.PlanRules = uniqueSorted(w.context.PlanRules)
	for _, path := range slices.Sorted(maps.Keys(w.files)) {
		w.agent.Files = append(w.agent.Files, *w.files[path])
	}
}

// line dispatches one transcript line.
func (w *walk) line(entry *transcript.Line) {
	switch entry.Type {
	case transcript.TypeAttachment:
		w.attachment(&entry.Attachment)
	case transcript.TypeAssistant:
		if w.transcriptModel == "" && entry.Message.Model != "" && entry.Message.Model != syntheticModel {
			w.transcriptModel = entry.Message.Model
		}
		for index := range entry.Message.Content.Blocks {
			if entry.Message.Content.Blocks[index].Type == transcript.BlockToolUse {
				w.addUse(&entry.Message.Content.Blocks[index])
			}
		}
	case transcript.TypeUser:
		for index := range entry.Message.Content.Blocks {
			if entry.Message.Content.Blocks[index].Type == transcript.BlockToolResult {
				w.addResult(&entry.Message.Content.Blocks[index])
			}
		}
	}
}

func (w *walk) attachment(attachment *transcript.Attachment) {
	switch attachment.Type {
	case transcript.AttachmentHookSuccess:
		w.addHook(attachment.HookName, SourceHookSuccess, hookText(attachment.Stdout))
	case transcript.AttachmentHookAdditionalContext:
		w.addHook(attachment.HookName, SourceHookContext, strings.Join(attachment.Content, "\n"))
	}
}

func (w *walk) recordGuideRead(lang, path, text string) {
	reads, ok := w.langs[path]
	if !ok {
		reads = &langReads{guide: path, lang: lang}
		w.langs[path] = reads
	}
	reads.ids = append(reads.ids, ids(text)...)
}

func (w *walk) touchFile(block *transcript.Block) {
	path := block.Input.FilePath
	if block.Name == toolNotebookEdit {
		path = block.Input.NotebookPath
	}
	if path == "" {
		return
	}
	var isRead, isWrite bool
	switch block.Name {
	case toolRead:
		isRead = true
	case toolEdit, toolMultiEdit, toolNotebookEdit, toolWrite:
		isWrite = true
	}
	if !isRead && !isWrite {
		return
	}

	touch, ok := w.files[path]
	if !ok {
		touch = &FileTouch{Path: path}
		w.files[path] = touch
	}
	if isRead {
		touch.Reads++
	}
	if isWrite {
		touch.Writes++
	}
}

// ctxDir is the context-pack directory name under the session cwd.
func Extract(path, ctxDir string) (*Record, error) {
	record := &Record{Agent: SessionClaude}
	root := &Agent{Id: rootAgentId, Kind: AgentRoot, Path: path}
	identity, err := scanIdentity(path)
	if err != nil {
		return nil, fmt.Errorf("Extract: %w", err)
	}
	record.Cwd = identity.cwd
	record.GitBranch = identity.gitBranch
	record.SessionId = identity.sessionId

	if err := walkTranscript(path, ctxDir, identity.cwd, root); err != nil {
		return nil, fmt.Errorf("Extract: %w", err)
	}
	record.Agents = append(record.Agents, *root)

	for _, subagent := range subagentTranscripts(strings.TrimSuffix(path, transcriptSuffix)) {
		agent := subagent
		if err := walkTranscript(agent.Path, ctxDir, identity.cwd, &agent); err != nil {
			return nil, fmt.Errorf("Extract: %w", err)
		}
		record.Agents = append(record.Agents, agent)
	}
	settleCoverage(record.Agents)

	rootContext := record.Agents[0].Context
	record.AcdslRules = rootContext.AcdslRules
	record.Injected = rootContext.Injected
	record.PackReads = rootContext.PackReads
	record.PlanRules = rootContext.PlanRules
	record.SubagentContext = rootContext.SubagentContext
	record.Recompute()
	return record, nil
}

func coverage(spans [][2]int, total int) string {
	if len(spans) == 0 || total == 0 {
		return coveragePartial
	}
	slices.SortFunc(spans, func(first, second [2]int) int { return first[0] - second[0] })
	next := 1
	isContiguous := true
	for _, span := range spans {
		if span[0] > next {
			isContiguous = false
		}
		next = max(span[1]+1, next)
	}
	if isContiguous && next > total {
		return coverageComplete
	}
	return coveragePartial
}

// guideSpans collects the line spans of the agent's Reads of one guide.
func guideSpans(agent *Agent, guide string) [][2]int {
	var spans [][2]int
	for _, item := range agent.Items {
		if item.Kind == KindLang && item.Label == guide {
			spans = append(spans, [2]int{item.StartLine, item.EndLine})
		}
	}
	return spans
}

// hookText unwraps the JSON hook envelope, else returns the stdout itself.
func hookText(stdout string) string {
	var envelope hookEnvelope
	if err := json.Unmarshal([]byte(stdout), &envelope); err == nil {
		return envelope.HookSpecificOutput.AdditionalContext
	}
	return stdout
}

// newReadItem measures a Read result and records the line span it returned.
func newReadItem(kind Kind, label string, use *transcript.Block, text string) Item {
	item := NewItem(kind, label, SourceToolResult, text)
	item.StartLine, item.EndLine = returnedSpan(use, text)
	item.Range = readSpan(item.StartLine, item.EndLine)
	return item
}

func readSpan(startLine, endLine int) string {
	if endLine == 0 {
		return ""
	}
	return fmt.Sprintf("lines %d–%d", startLine, endLine)
}

// A missing meta file leaves the identity fields empty.
func readSubagentMeta(agent *Agent) {
	data, err := os.ReadFile(strings.TrimSuffix(agent.Path, transcriptSuffix) + metaSuffix)
	if err != nil {
		return
	}
	var meta agentMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return
	}
	agent.Description = meta.Description
	agent.Model = meta.Model
	agent.SpawnDepth = meta.SpawnDepth
	agent.Type = meta.AgentType
}

// 1-based inclusive, offset 1 when unset; 0, 0 for an empty result.
func returnedSpan(use *transcript.Block, text string) (int, int) {
	if span, isNumbered := numberedSpan(text); isNumbered {
		return span.startLine, span.fileEnd(readLimit(use))
	}

	lines := countLines(text)
	if lines == 0 {
		return 0, 0
	}
	startLine := max(use.Input.Offset, 1)
	return startLine, startLine + lines - 1
}

func numberedSpan(text string) (readNumbers, bool) {
	var span readNumbers
	for line := range strings.SplitSeq(text, "\n") {
		match := numberedLinePattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		number, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		if span.count == 0 {
			span.startLine = number
		}
		span.count++
		span.lastLine = number
		span.isLastEmpty = strings.TrimSpace(match[2]) == ""
	}
	return span, span.count > 0
}

func readLimit(use *transcript.Block) int {
	if use.Input.Limit > 0 {
		return use.Input.Limit
	}
	return defaultReadLimit
}

func scanIdentity(path string) (transcriptIdentity, error) {
	var identity transcriptIdentity
	file, err := os.Open(path)
	if err != nil {
		return identity, fmt.Errorf("scanIdentity: %w", err)
	}
	defer file.Close()

	scanner := transcript.NewScanner(file)
	for scanner.Scan() {
		var entry transcript.Line
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if identity.sessionId == "" && entry.SessionId != "" {
			identity.sessionId = entry.SessionId
		}
		if identity.gitBranch == "" && entry.GitBranch != "" {
			identity.gitBranch = entry.GitBranch
		}
		if identity.cwd == "" && entry.Type == transcript.TypeUser && entry.Cwd != "" {
			identity.cwd = entry.Cwd
		}
		if identity.cwd != "" && identity.sessionId != "" && identity.gitBranch != "" {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return identity, fmt.Errorf("scanIdentity: %w", err)
	}
	return identity, nil
}

// The read gate forces each agent to a guide's end, so the furthest line any agent read is the last.
func settleCoverage(agents []Agent) {
	totals := map[string]int{}
	for agentIndex := range agents {
		for _, item := range agents[agentIndex].Items {
			if item.Kind == KindLang {
				totals[item.Label] = max(totals[item.Label], item.EndLine)
			}
		}
	}
	for agentIndex := range agents {
		agent := &agents[agentIndex]
		for langIndex := range agent.Context.Injected.Lang {
			lang := &agent.Context.Injected.Lang[langIndex]
			lang.TotalLines = totals[lang.Guide]
			lang.Coverage = coverage(guideSpans(agent, lang.Guide), lang.TotalLines)
		}
	}
}

func subagentTranscripts(stem string) []Agent {
	var agents []Agent
	plain, _ := filepath.Glob(filepath.Join(stem, subagentsDir, agentPrefix+"*"+transcriptSuffix))
	for _, path := range plain {
		agents = append(agents, subagentFor(path, AgentSubagent))
	}
	workflow, _ := filepath.Glob(filepath.Join(stem, subagentsDir, workflowsDir, "*", agentPrefix+"*"+transcriptSuffix))
	for _, path := range workflow {
		agents = append(agents, subagentFor(path, AgentWorkflow))
	}
	return agents
}

func subagentFor(path string, kind AgentKind) Agent {
	id := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(path), agentPrefix), transcriptSuffix)
	agent := Agent{Id: id, Kind: kind, Path: path}
	readSubagentMeta(&agent)
	return agent
}

// submatches returns every first capture of pattern in text, in order.
func submatches(pattern *regexp.Regexp, text string) []string {
	var values []string
	for _, match := range pattern.FindAllStringSubmatch(text, -1) {
		values = append(values, match[1])
	}
	return values
}

// A missing subagent transcript is an error, a malformed line is skipped.
func walkTranscript(path, ctxDir, cwd string, agent *Agent) error {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("walkTranscript: Transcript missing: %s", path)
		}
		return fmt.Errorf("walkTranscript: %w", err)
	}
	defer file.Close()

	w := newWalk(agent, ctxDir)
	w.cwd = cwd
	scanner := transcript.NewScanner(file)
	for scanner.Scan() {
		var entry transcript.Line
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		w.line(&entry)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("walkTranscript: %s: %w", path, err)
	}
	w.finish()
	return nil
}

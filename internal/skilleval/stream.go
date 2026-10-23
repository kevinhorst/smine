package skilleval

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

const (
	blockText       = "text"
	blockToolResult = "tool_result"
	blockToolUse    = "tool_use"

	recordAssistant = "assistant"
	recordResult    = "result"
	recordSkilleval = "skilleval"
	recordSystem    = "system"
	recordUser      = "user"

	subtypeInit   = "init"
	subtypeResume = "resume"

	agentIdRunes      = 8
	initialLineBytes  = 1024 * 1024
	maxLineBytes      = 64 * 1024 * 1024
	traceSnippetRunes = 240
)

// workflowRunPattern also prefixes every isolation worktree the harness creates for that run's agents.
var workflowRunPattern = regexp.MustCompile(`^wf_[0-9a-z-]+$`)

type ArmStream struct {
	CostUsd           *float64
	Init              *InitRecord
	OutputTokens      *int
	PermissionDenials *int
	Result            *ResultRecord
	ToolCalls         []ToolCount
	ToolErrors        int
	Trace             []string
	UnparsedLines     int
	WallSeconds       *int // Σ per process of its first-to-last record span; a resumed run's pause between processes is not counted; nil without a timestamped record
	WorkflowRuns      []string
}

// InitRecord holds the tools and MCP servers the cell actually got.
type InitRecord struct {
	ClaudeCodeVersion string      `json:"claude_code_version"`
	McpServers        []McpServer `json:"mcp_servers"`
	Model             string      `json:"model"`
	SessionId         string      `json:"session_id"`
	Tools             []string    `json:"tools"`
}

type McpServer struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// ModelUsage covers every agent of the session, subagents and workflow agents included.
type ModelUsage struct {
	OutputTokens int `json:"outputTokens"`
}

// PermissionDenial is one denied tool call of a result envelope's turn.
type PermissionDenial struct {
	ToolUseId string `json:"tool_use_id"`
}

type ResultRecord struct {
	IsError           bool                  `json:"is_error"`
	ModelUsage        map[string]ModelUsage `json:"modelUsage"`
	PermissionDenials []PermissionDenial    `json:"permission_denials"`
	Result            string                `json:"result"`
	SessionId         string                `json:"session_id"`
	Subtype           string                `json:"subtype"`
	TerminalReason    string                `json:"terminal_reason"`
	TotalCostUsd      float64               `json:"total_cost_usd"`
}

type ToolCount struct {
	Calls  int    `json:"calls"`
	Errors int    `json:"errors"`
	Name   string `json:"name"`
}

type contentBlock struct {
	Id        string          `json:"id"`
	Content   flexibleText    `json:"content"`
	Input     json.RawMessage `json:"input"`
	IsError   bool            `json:"is_error"`
	Name      string          `json:"name"`
	Text      string          `json:"text"`
	ToolUseId string          `json:"tool_use_id"`
	Type      string          `json:"type"`
}

// contentBlocks: prompt messages carry a plain string, all others a block array.
type contentBlocks []contentBlock

func (b *contentBlocks) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*b = contentBlocks{{Text: text, Type: blockText}}
		return nil
	}

	var blocks []contentBlock
	if err := json.Unmarshal(data, &blocks); err != nil {
		return fmt.Errorf("contentBlocks.UnmarshalJSON: %w", err)
	}

	*b = blocks
	return nil
}

// flexibleText decodes tool_result content: a string or an array of text parts.
type flexibleText string

func (t *flexibleText) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*t = flexibleText(text)
		return nil
	}

	var parts []textPart
	if err := json.Unmarshal(data, &parts); err != nil {
		return fmt.Errorf("flexibleText.UnmarshalJSON: %w", err)
	}

	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		texts = append(texts, part.Text)
	}
	*t = flexibleText(strings.Join(texts, "\n"))
	return nil
}

type streamMessage struct {
	Content contentBlocks `json:"content"`
}

// UnmarshalJSON decodes a system task record's plain-string message to an empty message.
func (m *streamMessage) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		return nil
	}

	var object streamMessageObject
	if err := json.Unmarshal(data, &object); err != nil {
		return fmt.Errorf("streamMessage.UnmarshalJSON: %w", err)
	}

	*m = streamMessage(object)
	return nil
}

// streamMessageObject drops the decoder so UnmarshalJSON does not recurse.
type streamMessageObject streamMessage

// Result envelopes are cumulative within one process; the runner's resume marker starts a new process's count.
type streamParser struct {
	bankedCostUsd      float64
	bankedOutputTokens int
	bankedWall         time.Duration
	counts             map[string]*ToolCount
	denied             map[string]bool
	hasTimestamp       bool
	namesById          map[string]string
	segmentFirst       time.Time
	segmentLast        time.Time
	segmentResult      *ResultRecord
	stream             *ArmStream
}

func newStreamParser() *streamParser {
	return &streamParser{
		counts:    make(map[string]*ToolCount),
		denied:    make(map[string]bool),
		namesById: make(map[string]string),
		stream:    &ArmStream{},
	}
}

func (p *streamParser) addAssistant(record *streamRecord) {
	agent := agentLabel(record.ParentToolUseId)
	for index := range record.Message.Content {
		p.addAssistantBlock(agent, &record.Message.Content[index])
	}
}

func (p *streamParser) addAssistantBlock(agent string, block *contentBlock) {
	switch block.Type {
	case blockText:
		p.trace("[%s] text: %s", agent, snippet(block.Text))
	case blockToolUse:
		p.namesById[block.Id] = block.Name
		p.count(block.Name).Calls++
		p.trace("[%s] %s %s", agent, block.Name, snippet(string(block.Input)))
	}
}

func (p *streamParser) addLine(line []byte) {
	var record streamRecord
	if err := json.Unmarshal(line, &record); err != nil {
		p.stream.UnparsedLines++
		return
	}

	p.addTimestamp(record.Timestamp)
	switch record.Type {
	case recordAssistant:
		p.addAssistant(&record)
	case recordResult:
		p.addResult(line)
	case recordSkilleval:
		p.addSkilleval(&record)
	case recordSystem:
		p.addSystem(line, record.Subtype)
	case recordUser:
		p.addUser(&record)
	}
}

func (p *streamParser) addResult(line []byte) {
	var result ResultRecord
	if err := json.Unmarshal(line, &result); err != nil {
		p.stream.UnparsedLines++
		return
	}

	for _, denial := range result.PermissionDenials {
		p.denied[denial.ToolUseId] = true
	}
	p.segmentResult = &result
	p.stream.Result = &result
}

func (p *streamParser) addSkilleval(record *streamRecord) {
	if record.Subtype != subtypeResume {
		return
	}

	p.bankSegment()
	p.bankSpan()
	p.trace("[harness] resumed session %s", record.SessionId)
}

// addTimestamp widens the current process's span; an absent or unparsable stamp changes nothing.
func (p *streamParser) addTimestamp(stamp string) {
	if stamp == "" {
		return
	}

	recordedAt, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		return
	}

	p.hasTimestamp = true
	if p.segmentFirst.IsZero() || recordedAt.Before(p.segmentFirst) {
		p.segmentFirst = recordedAt
	}
	if recordedAt.After(p.segmentLast) {
		p.segmentLast = recordedAt
	}
}

// bankSpan closes one process's span so the pause before the next process is not counted.
func (p *streamParser) bankSpan() {
	if !p.segmentFirst.IsZero() {
		p.bankedWall += p.segmentLast.Sub(p.segmentFirst)
	}
	p.segmentFirst = time.Time{}
	p.segmentLast = time.Time{}
}

func (p *streamParser) addSystem(line []byte, subtype string) {
	isFirstInit := subtype == subtypeInit && p.stream.Init == nil
	if !isFirstInit {
		return
	}

	var initRecord InitRecord
	if err := json.Unmarshal(line, &initRecord); err != nil {
		p.stream.UnparsedLines++
		return
	}

	p.stream.Init = &initRecord
}

func (p *streamParser) addUser(record *streamRecord) {
	p.addWorkflowRun(record.ToolUseResult)
	agent := agentLabel(record.ParentToolUseId)
	for index := range record.Message.Content {
		block := &record.Message.Content[index]
		isFailedResult := block.Type == blockToolResult && block.IsError
		if !isFailedResult {
			continue
		}

		name := p.namesById[block.ToolUseId]
		p.count(name).Errors++
		p.stream.ToolErrors++
		p.trace("[%s] ERROR %s: %s", agent, name, snippet(string(block.Content)))
	}
}

func (p *streamParser) addWorkflowRun(toolUseResult json.RawMessage) {
	var launched workflowLaunch
	if err := json.Unmarshal(toolUseResult, &launched); err != nil {
		return
	}

	isNewRun := workflowRunPattern.MatchString(launched.RunId) && !slices.Contains(p.stream.WorkflowRuns, launched.RunId)
	if isNewRun {
		p.stream.WorkflowRuns = append(p.stream.WorkflowRuns, launched.RunId)
	}
}

// bankSegment closes one process's cumulative envelope before the next process counts from zero.
func (p *streamParser) bankSegment() {
	if p.segmentResult == nil {
		return
	}

	for _, usage := range p.segmentResult.ModelUsage {
		p.bankedOutputTokens += usage.OutputTokens
	}
	p.bankedCostUsd += p.segmentResult.TotalCostUsd
	p.segmentResult = nil
}

func (p *streamParser) count(name string) *ToolCount {
	counted, ok := p.counts[name]
	if !ok {
		counted = &ToolCount{Name: name}
		p.counts[name] = counted
	}
	return counted
}

func (p *streamParser) finish() *ArmStream {
	for _, counted := range p.counts {
		p.stream.ToolCalls = append(p.stream.ToolCalls, *counted)
	}
	sort.Slice(p.stream.ToolCalls, func(i, j int) bool {
		return p.stream.ToolCalls[i].Name < p.stream.ToolCalls[j].Name
	})
	p.bankSpan()
	if p.hasTimestamp {
		wallSeconds := int(math.Round(p.bankedWall.Seconds()))
		p.stream.WallSeconds = &wallSeconds
	}
	if p.stream.Result == nil {
		return p.stream
	}

	p.bankSegment()
	denials := len(p.denied)
	p.stream.CostUsd = &p.bankedCostUsd
	p.stream.OutputTokens = &p.bankedOutputTokens
	p.stream.PermissionDenials = &denials
	return p.stream
}

func (p *streamParser) trace(format string, args ...any) {
	p.stream.Trace = append(p.stream.Trace, fmt.Sprintf(format, args...))
}

type streamRecord struct {
	Message         streamMessage   `json:"message"`
	ParentToolUseId *string         `json:"parent_tool_use_id"`
	SessionId       string          `json:"session_id"`
	Subtype         string          `json:"subtype"`
	Timestamp       string          `json:"timestamp"`
	ToolUseResult   json.RawMessage `json:"tool_use_result"`
	Type            string          `json:"type"`
}

type textPart struct {
	Text string `json:"text"`
}

// workflowLaunch is the part of a Workflow tool result that names its run.
type workflowLaunch struct {
	RunId string `json:"runId"`
}

func agentLabel(parentToolUseId *string) string {
	if parentToolUseId == nil {
		return "root"
	}

	runes := []rune(*parentToolUseId)
	if len(runes) > agentIdRunes {
		runes = runes[len(runes)-agentIdRunes:]
	}
	return "agent " + string(runes)
}

func parseStreamFile(path string) (*ArmStream, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("parseStreamFile: %w", err)
	}
	defer file.Close()

	return ParseStream(file)
}

// snippet flattens whitespace and caps the text so one trace line is one event.
func snippet(text string) string {
	return truncateRunes(strings.Join(strings.Fields(text), " "), traceSnippetRunes)
}

func truncateRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "…"
}

// ParseStream counts undecodable lines, never fails on them.
func ParseStream(reader io.Reader) (*ArmStream, error) {
	parser := newStreamParser()
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, initialLineBytes), maxLineBytes)
	for scanner.Scan() {
		parser.addLine(scanner.Bytes())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("ParseStream: %w", err)
	}

	return parser.finish(), nil
}

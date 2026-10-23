package routinerun

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/routines"
)

const (
	eventBuffer            = 256
	nudgeMessage           = "The run is not marked complete: no file at %s. Nobody answers questions in this session. Finish any remaining work, then write %s with the Write tool, with exactly the content your instructions specify for it, as your final act."
	roleUser               = "user"
	steerPreamble          = "OPERATOR STEER — mid-run instruction from the routines UI, act on it now: "
	subtypeBackgroundTasks = "background_tasks_changed"
	subtypeInit            = "init"
)

type CompleteMode string

// Marker mode exists for dispatchers whose turns end while lanes still run.
const (
	CompleteMarker CompleteMode = "marker"
	CompleteResult CompleteMode = "result"
)

type stopReason string

const (
	reasonCancelled   stopReason = "cancelled"
	reasonChildExit   stopReason = "child-exit"
	reasonComplete    stopReason = "complete"
	reasonDeadline    stopReason = "deadline"
	reasonNone        stopReason = ""
	reasonResultClose stopReason = "result-close"
)

type ChatOutcome struct {
	Envelope   *Envelope
	ResultLine []byte
	Status     int
}

// ChatSpec is one steerable stream-json claude session.
type ChatSpec struct {
	Args         []string // claude args after the stream-json flags
	Attended     bool     // true = started without the headless marker: an attended session launched it
	CellId       string   // "" = cell-less: files live in SteerDir itself
	CellPids     string
	Command      []string // launch argv ahead of the stream-json flags; empty = claude
	Complete     CompleteMode
	Dir          string
	ExitGrace    time.Duration
	MarkerPath   string                 // required for CompleteMarker
	OnSession    func(sessionId string) // called once when the stream's init event names the session; nil = no call
	Poll         time.Duration
	Prompt       string
	Registration *sessionRegistration // container cells: host registration of the session
	SteerDir     string
	Timeout      time.Duration // active seconds, pause gaps excluded
}

type chatPump struct {
	activeTime      time.Duration
	backgroundTasks int // running set of the latest background_tasks_changed snapshot
	cellDir         string
	idleSince       time.Time // zero = mid-turn
	isNudged        bool
	isRegistered    bool
	lastResult      []byte
	lastTick        time.Time
	log             *os.File
	markerPath      string
	pid             int
	sessionId       string
	spec            *ChatSpec
	stdin           io.WriteCloser
}

func newChatPump(spec *ChatSpec) (*chatPump, error) {
	cellDir := spec.SteerDir
	if spec.CellId != "" {
		cellDir = filepath.Join(spec.SteerDir, spec.CellId)
	}
	if err := os.MkdirAll(cellDir, 0o755); err != nil {
		return nil, fmt.Errorf("newChatPump: %w", err)
	}
	if spec.Complete == CompleteMarker && spec.MarkerPath == "" {
		return nil, errors.New("newChatPump: Marker cell without MarkerPath")
	}
	if spec.MarkerPath != "" {
		_ = os.Remove(spec.MarkerPath)
	}

	logFile, err := os.Create(filepath.Join(cellDir, "pump.log"))
	if err != nil {
		return nil, fmt.Errorf("newChatPump: %w", err)
	}

	pump := &chatPump{
		cellDir:    cellDir,
		lastTick:   time.Now(),
		log:        logFile,
		markerPath: spec.MarkerPath,
		spec:       spec,
	}
	return pump, nil
}

func (p *chatPump) completion() stopReason {
	if p.idleSince.IsZero() || p.backgroundTasks > 0 {
		return reasonNone
	}
	if _, err := os.Stat(p.markerPath); err == nil {
		return reasonComplete
	}

	isSteerPending := fsx.HasContent(filepath.Join(p.spec.SteerDir, routines.SteerPromptFileName))
	if isSteerPending {
		return reasonNone
	}
	if p.spec.Complete == CompleteResult || p.isNudged {
		return reasonResultClose
	}
	p.nudge()
	return reasonNone
}

// deliverSteer never sends an interrupt: it would abort in-flight subagents.
func (p *chatPump) deliverSteer() {
	if p.idleSince.IsZero() {
		return
	}
	steerFile := filepath.Join(p.spec.SteerDir, routines.SteerPromptFileName)
	if !fsx.HasContent(steerFile) {
		return
	}

	claimed := filepath.Join(p.cellDir, "steer-delivered.tmp")
	if err := os.Rename(steerFile, claimed); err != nil {
		return
	}
	defer os.Remove(claimed)

	prompt, err := os.ReadFile(claimed)
	if err != nil {
		p.logf("steer read failed: %v", err)
		return
	}
	if err := p.send(steerPreamble + strings.TrimRight(string(prompt), "\n")); err != nil {
		p.logf("steer send failed: %v", err)
		return
	}

	cell := p.spec.CellId
	if cell == "" {
		cell = "cell"
	}
	receipt := &routines.SteerReceipt{
		Cell:        cell,
		DeliveredAt: time.Now().UTC().Truncate(time.Second),
		Interrupt:   false,
		Prompt:      string(prompt),
	}
	data, err := json.Marshal(receipt)
	if err == nil {
		err = fsx.WriteFileAtomic(data, filepath.Join(p.spec.SteerDir, routines.SteerDeliveredFileName), 0o644)
	}
	if err != nil {
		p.logf("receipt write failed: %v", err)
	}
	p.logf("delivered ts=%s interrupt=0", utcStamp(time.Now()))
	p.idleSince = time.Time{}
}

// drain consumes events until the stream closes (true) or wait elapses.
func (p *chatPump) drain(events <-chan streamEvent, wait time.Duration) bool {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	for {
		select {
		case event, isOpen := <-events:
			if !isOpen {
				return true
			}
			p.observe(event)
		case <-timer.C:
			return false
		}
	}
}

func (p *chatPump) logf(format string, args ...any) {
	fmt.Fprintf(p.log, format+"\n", args...)
}

func (p *chatPump) loop(ctx context.Context, pid int, events <-chan streamEvent) stopReason {
	ticker := time.NewTicker(p.spec.Poll)
	defer ticker.Stop()
	for {
		select {
		case event, isOpen := <-events:
			if !isOpen {
				return reasonChildExit
			}
			p.observe(event)
		case now := <-ticker.C:
			reason := p.tick(now)
			if reason == reasonDeadline {
				terminateGroup(pid)
			}
			if reason != reasonNone {
				return reason
			}
		case <-ctx.Done():
			return reasonCancelled
		}
	}
}

func (p *chatPump) nudge() {
	p.isNudged = true
	p.idleSince = time.Time{}
	if err := p.send(fmt.Sprintf(nudgeMessage, p.markerPath, p.markerPath)); err != nil {
		p.logf("nudge send failed: %v", err)
		return
	}
	p.logf("nudged ts=%s", utcStamp(time.Now()))
}

// observe ignores system events after a result (task_summary): they do not reopen the turn.
func (p *chatPump) observe(event streamEvent) {
	switch event.kind {
	case "result":
		p.lastResult = event.line
		if p.idleSince.IsZero() {
			p.idleSince = time.Now()
		}
	case roleUser, "assistant":
		p.idleSince = time.Time{}
	case "system":
		if event.subtype == subtypeBackgroundTasks {
			p.observeBackgroundTasks(event.backgroundTasks)
		}
		isNamingInit := event.subtype == subtypeInit && event.sessionId != ""
		isFirstInit := isNamingInit && p.sessionId == ""
		if event.subtype == subtypeInit {
			p.sessionId = event.sessionId
		}
		if isFirstInit && p.spec.OnSession != nil {
			p.spec.OnSession(event.sessionId)
		}
	}
}

// observeBackgroundTasks reopens the turn: the harness opens a task-notification turn when the last task ends.
func (p *chatPump) observeBackgroundTasks(count int) {
	hasLastTaskEnded := count == 0 && p.backgroundTasks > 0
	p.backgroundTasks = count
	if hasLastTaskEnded {
		p.idleSince = time.Time{}
	}
}

// registerSession is a no-op for a host cell: only a container cell carries a Registration.
func (p *chatPump) registerSession() {
	registration := p.spec.Registration
	isWritable := registration != nil && p.sessionId != ""
	if !isWritable || p.isRegistered {
		return
	}

	isWritten, err := registration.write(p.pid, p.sessionId)
	if err != nil {
		p.logf("registration failed: %v", err)
		return
	}
	p.isRegistered = isWritten
}

func (p *chatPump) send(text string) error {
	message := &userMessage{
		Message: messageBody{Content: text, Role: roleUser},
		Type:    roleUser,
	}
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("chatPump.send: %w", err)
	}
	if _, err := p.stdin.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("chatPump.send: %w", err)
	}
	return nil
}

func (p *chatPump) shutdown(pid int, events <-chan streamEvent, reason stopReason) {
	_ = p.stdin.Close()
	isTerminated := reason == reasonDeadline || reason == reasonCancelled
	if !isTerminated {
		if p.drain(events, p.spec.ExitGrace) {
			return
		}
		p.logf("child alive %ds after EOF — TERM", int(p.spec.ExitGrace.Seconds()))
		terminateGroup(pid)
	}
	if p.drain(events, killGrace) {
		return
	}
	killGroup(pid)
	p.drain(events, killGrace)
}

// tick counts a gap far beyond the poll interval as SIGSTOP time: only the nominal interval counts.
func (p *chatPump) tick(now time.Time) stopReason {
	gap := now.Sub(p.lastTick)
	p.lastTick = now
	if gap > 2*p.spec.Poll+time.Second {
		paused := gap - p.spec.Poll
		p.logf("pause gap: %ds excluded from deadline", int(paused.Seconds()))
		if !p.idleSince.IsZero() {
			p.idleSince = p.idleSince.Add(paused)
		}
		gap = p.spec.Poll
	}
	p.activeTime += gap

	p.registerSession()
	p.deliverSteer()
	if reason := p.completion(); reason != reasonNone {
		return reason
	}
	if p.activeTime >= p.spec.Timeout {
		p.logf("deadline after %ds active", int(p.spec.Timeout.Seconds()))
		return reasonDeadline
	}
	return reasonNone
}

type messageBody struct {
	Content string `json:"content"`
	Role    string `json:"role"`
}

type backgroundTask struct {
	TaskId string `json:"task_id"`
}

type streamEvent struct {
	backgroundTasks int // len(tasks) of a background_tasks_changed snapshot
	kind            string
	line            []byte
	sessionId       string
	subtype         string
}

type streamHeader struct {
	SessionId string           `json:"session_id"`
	Subtype   string           `json:"subtype"`
	Tasks     []backgroundTask `json:"tasks"`
	Type      string           `json:"type"`
}

type userMessage struct {
	Message         messageBody `json:"message"`
	ParentToolUseId *string     `json:"parent_tool_use_id"` // always null — a top-level user turn
	Type            string      `json:"type"`
}

func chatStatus(reason stopReason, waitErr error) int {
	switch reason {
	case reasonComplete, reasonResultClose:
		return exitOk
	case reasonDeadline:
		return exitDeadline
	case reasonCancelled:
		return exitCancelled
	case reasonChildExit, reasonNone:
		if waitErr == nil {
			return exitOk
		}
		return exitCode(waitErr)
	}
	return exitSoftware
}

// readStream closes events at EOF, which is the pump's child-exit signal.
func readStream(stdout io.Reader, stream io.Writer, events chan<- streamEvent, done <-chan struct{}) {
	defer close(events)
	reader := bufio.NewReader(stdout)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			_, _ = stream.Write(line)
			header := parseStreamHeader(line)
			event := streamEvent{
				backgroundTasks: len(header.Tasks),
				kind:            header.Type,
				line:            []byte(strings.TrimRight(string(line), "\n")),
				sessionId:       header.SessionId,
				subtype:         header.Subtype,
			}
			select {
			case events <- event:
			case <-done:
				return
			}
		}
		if err != nil {
			return
		}
	}
}

func parseStreamHeader(line []byte) *streamHeader {
	var header streamHeader
	if err := json.Unmarshal(line, &header); err != nil {
		return &streamHeader{}
	}
	return &header
}

// RunChat holds stdin open so the session's lifetime belongs to the pump, not to the model's tool use.
func RunChat(ctx context.Context, spec *ChatSpec) (*ChatOutcome, error) {
	pump, err := newChatPump(spec)
	if err != nil {
		return nil, err
	}
	defer pump.log.Close()

	stream, err := os.Create(filepath.Join(pump.cellDir, "stream.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("RunChat: %w", err)
	}
	defer stream.Close()

	command := spec.Command
	if len(command) == 0 {
		command = []string{"claude"}
	}
	args := append(slices.Clone(command[1:]), "-p", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose")
	args = append(args, spec.Args...)
	cmd := exec.CommandContext(ctx, command[0], args...)
	cmd.Dir = spec.Dir
	cmd.Env = os.Environ()
	if !spec.Attended {
		cmd.Env = append(cmd.Env, headlessEnv)
	}
	cmd.Stderr = pump.log
	isolateGroup(cmd)
	cmd.Cancel = func() error {
		terminateGroup(cmd.Process.Pid)
		return nil
	}
	cmd.WaitDelay = killGrace

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("RunChat: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("RunChat: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("RunChat: Failed to start %s: %w", command[0], err)
	}
	registerCell(spec.CellPids, cmd.Process.Pid)
	pump.pid = cmd.Process.Pid
	pump.stdin = stdin

	events := make(chan streamEvent, eventBuffer)
	done := make(chan struct{})
	defer close(done)
	go readStream(stdout, stream, events, done)

	if err := pump.send(spec.Prompt); err != nil {
		pump.logf("first message failed: %v", err)
	}
	reason := pump.loop(ctx, cmd.Process.Pid, events)
	pump.shutdown(cmd.Process.Pid, events, reason)
	waitErr := cmd.Wait()

	status := chatStatus(reason, waitErr)
	if ctx.Err() != nil {
		status = exitCancelled
	}
	logReason := reason
	if logReason == reasonNone {
		logReason = reasonChildExit
	}
	pump.logf("end reason=%s status=%d active=%ds", logReason, status, int(pump.activeTime.Seconds()))

	outcome := &ChatOutcome{
		Envelope:   parseEnvelope(pump.lastResult),
		ResultLine: pump.lastResult,
		Status:     status,
	}
	return outcome, nil
}

package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/kevinhorst/smine/internal/gitroot"
	"github.com/kevinhorst/smine/internal/transcript"
)

type anchorTracker struct {
	current string
	pending map[string]string
	start   string
}

func newAnchorTracker() *anchorTracker {
	tracker := &anchorTracker{pending: map[string]string{}}
	return tracker
}

func (a *anchorTracker) applyResult(block *transcript.Block, result *transcript.ToolUseResult) {
	tool, isPending := a.pending[block.ToolUseId]
	if !isPending || block.IsError {
		return
	}

	delete(a.pending, block.ToolUseId)
	switch tool {
	case enterWorktreeTool:
		if result.WorktreePath != "" {
			a.current = result.WorktreePath
		}
	case exitWorktreeTool:
		a.current = a.start
	}
}

// observe advances the tracker by one transcript line.
func (a *anchorTracker) observe(line *transcript.Line) {
	if a.start == "" && line.Cwd != "" {
		a.start = line.Cwd
		a.current = line.Cwd
	}
	for index := range line.Message.Content.Blocks {
		block := &line.Message.Content.Blocks[index]
		switch block.Type {
		case transcript.BlockToolUse:
			a.recordCall(block)
		case transcript.BlockToolResult:
			a.applyResult(block, &line.ToolUseResult)
		}
	}
}

func (a *anchorTracker) recordCall(block *transcript.Block) {
	isWorktreeCall := block.Name == enterWorktreeTool || block.Name == exitWorktreeTool
	if isWorktreeCall {
		a.pending[block.Id] = block.Name
	}
}

type sessionAnchor struct {
	isMissing bool
	path      string
	source    string
}

func firstCwd(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := transcript.NewScanner(file)
	for scanner.Scan() {
		line := transcript.Line{}
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}
		if line.Cwd != "" {
			return line.Cwd
		}
	}
	return ""
}

func resolveAnchor(input *HookInput, tracked string, worktreesDir string) sessionAnchor {
	anchor := sessionAnchor{path: input.Cwd, source: anchorSourceEnvelope}
	if tracked != "" {
		anchor = sessionAnchor{path: tracked, source: anchorSourceTranscript}
	}
	if worktree := subagentWorktree(input, anchor.path, worktreesDir); worktree != "" {
		anchor = sessionAnchor{path: worktree, source: anchorSourceSubagent}
	}
	if anchor.source == anchorSourceEnvelope {
		return anchor
	}

	if _, err := os.Stat(anchor.path); err != nil {
		anchor.isMissing = true
	}
	return anchor
}

// cmd/skillinvokegate searches the same dir at any depth; the envelope carries the parent's transcript path.
func subagentTranscript(input *HookInput) string {
	if input.SessionId == "" || input.TranscriptPath == "" {
		return ""
	}

	name := subagentTranscriptPrefix + input.AgentId + transcriptExtension
	subagents := filepath.Join(filepath.Dir(input.TranscriptPath), input.SessionId, subagentsDir)
	direct := filepath.Join(subagents, name)
	if isFile(direct) {
		return direct
	}

	runDirs, err := os.ReadDir(filepath.Join(subagents, workflowsDir))
	if err != nil {
		return ""
	}
	for _, runDir := range runDirs {
		candidate := filepath.Join(subagents, workflowsDir, runDir.Name(), name)
		if isFile(candidate) {
			return candidate
		}
	}
	return ""
}

func subagentWorktree(input *HookInput, parent string, worktreesDir string) string {
	if input.AgentId == "" {
		return ""
	}

	cwd := firstCwd(subagentTranscript(input))
	if cwd == "" {
		return ""
	}

	worktree := resolvePath(parent, cwd)
	container := resolvePath(parent, filepath.Join(gitroot.MainRoot(parent), worktreesDir))
	if filepath.Dir(worktree) != container {
		return ""
	}
	return worktree
}

// Package contextrequired names the per-session required-reading directory the read gate enforces.
package contextrequired

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const GlobalFile = "global.md"

// The plan-mode context hook writes and removes it.
const PlanModeFile = "planmode.md"

// ReadBudgetBytes is the authoring-time bound of one required-reading file; the hooks chunk by the hooks config's read budget.
const ReadBudgetBytes = 55000

// LineChunk is one Read instruction's inclusive line range.
type LineChunk struct {
	End   int
	Start int
}

// chunkLines cuts a new chunk before every line that would push the current one over the budget.
func chunkLines(budget int, last int, lines []string, start int) []LineChunk {
	chunks := make([]LineChunk, 0, 1)
	chunk := LineChunk{End: start, Start: start}
	chunkBytes := 0
	for number := start; number <= last; number++ {
		size := len(lines[number-1])
		if isOverBudget(budget, chunkBytes, size) {
			chunks = append(chunks, chunk)
			chunk = LineChunk{End: number, Start: number}
			chunkBytes = 0
		}

		chunk.End = number
		chunkBytes += size
	}
	return append(chunks, chunk)
}

// isOverBudget is true when size no longer fits the current chunk; a chunk always takes its first line.
func isOverBudget(budget int, chunkBytes int, size int) bool {
	if chunkBytes == 0 {
		return false
	}
	return chunkBytes+size > budget
}

// readLines splits a file into its lines, newline included, without the trailing empty element.
func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("readLines: Failed to read %s: %w", path, err)
	}

	lines := strings.SplitAfter(string(data), "\n")
	hasTrailingEmpty := lines[len(lines)-1] == ""
	if hasTrailingEmpty {
		lines = lines[:len(lines)-1]
	}
	return lines, nil
}

// Dir is the session's required-reading directory.
func Dir(claudeHome string, sessionId string) string {
	return filepath.Join(Root(claudeHome), sessionId)
}

// LineChunks splits lines start..end of path into ranges of at most budget bytes; a longer single line is its own chunk.
func LineChunks(end int, path string, start int, budget int) ([]LineChunk, error) {
	lines, err := readLines(path)
	if err != nil {
		return nil, fmt.Errorf("LineChunks: %w", err)
	}

	isBeyondFile := start > len(lines)
	if isBeyondFile {
		whole := []LineChunk{{End: end, Start: start}}
		return whole, nil
	}

	last := min(end, len(lines))
	return chunkLines(budget, last, lines, start), nil
}

// ReadInstructions renders one Read line per chunk of lines start..end; an unreadable file yields the one unchunked line.
func ReadInstructions(end int, path string, start int, budget int) []string {
	chunks, err := LineChunks(end, path, start, budget)
	if err != nil {
		chunks = []LineChunk{{End: end, Start: start}}
	}

	instructions := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		instructions = append(instructions, fmt.Sprintf("Read %s offset %d limit %d", path, chunk.Start, chunk.End-chunk.Start+1))
	}
	return instructions
}

// Root holds every session's required-reading directory.
func Root(claudeHome string) string {
	return filepath.Join(claudeHome, "context-required")
}

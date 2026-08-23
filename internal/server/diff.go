package server

import (
	"os"
	"strings"
)

const (
	diffSame = "same"
	diffAdd  = "add"
	diffDel  = "del"
	diffSkip = "skip"
)

// diffLine is one rendered diff line; Text carries the "+ "/"- "/"  " prefix
// so the text op-result and the HTML partial render identically.
type diffLine struct {
	Kind string
	Text string
}

// diffLines computes a line-level LCS diff from oldText to newText.
func diffLines(oldText, newText string) []diffLine {
	oldLines := strings.Split(oldText, "\n")
	newLines := strings.Split(newText, "\n")
	rows, cols := len(oldLines), len(newLines)
	lcs := make([][]int, rows+1)
	for i := range lcs {
		lcs[i] = make([]int, cols+1)
	}
	for i := rows - 1; i >= 0; i-- {
		for j := cols - 1; j >= 0; j-- {
			if oldLines[i] == newLines[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}

	var lines []diffLine
	i, j := 0, 0
	for i < rows && j < cols {
		switch {
		case oldLines[i] == newLines[j]:
			lines = append(lines, diffLine{Kind: diffSame, Text: "  " + oldLines[i]})
			i, j = i+1, j+1
		case lcs[i+1][j] >= lcs[i][j+1]:
			lines = append(lines, diffLine{Kind: diffDel, Text: "- " + oldLines[i]})
			i++
		default:
			lines = append(lines, diffLine{Kind: diffAdd, Text: "+ " + newLines[j]})
			j++
		}
	}
	for ; i < rows; i++ {
		lines = append(lines, diffLine{Kind: diffDel, Text: "- " + oldLines[i]})
	}
	for ; j < cols; j++ {
		lines = append(lines, diffLine{Kind: diffAdd, Text: "+ " + newLines[j]})
	}
	return lines
}

// compactDiff keeps context unchanged lines around each change and collapses
// longer same-runs into one skip line; a diff with no changes returns nil.
func compactDiff(lines []diffLine, context int) []diffLine {
	keep := make([]bool, len(lines))
	changed := false
	for i, line := range lines {
		if line.Kind == diffSame {
			continue
		}
		changed = true
		for j := max(0, i-context); j <= min(len(lines)-1, i+context); j++ {
			keep[j] = true
		}
	}
	if !changed {
		return nil
	}

	var out []diffLine
	skipping := false
	for i, line := range lines {
		if keep[i] {
			out = append(out, line)
			skipping = false
			continue
		}
		if !skipping {
			out = append(out, diffLine{Kind: diffSkip, Text: "  ⋯"})
			skipping = true
		}
	}
	return out
}

// formatDiff renders diff lines as plain text for op results.
func formatDiff(lines []diffLine) string {
	texts := make([]string, 0, len(lines))
	for _, line := range lines {
		texts = append(texts, line.Text)
	}
	return strings.Join(texts, "\n")
}

// fileDiff diffs a file's captured before-state against its current on-disk
// content; a missing before-state diffs from empty.
func fileDiff(before []byte, path string) ([]diffLine, error) {
	after, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return compactDiff(diffLines(string(before), string(after)), 2), nil
}

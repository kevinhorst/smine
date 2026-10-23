package main

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/reviewscope"
	"github.com/kevinhorst/smine/internal/shell"
)

type findingsArtifact struct {
	Walk         []walkEntry `json:"walk"`
	WalkComplete bool        `json:"walk_complete"`
}

type laneCoverage struct {
	Files []string `json:"files"`
}

// Manifest methods are nil-safe: a call without a manifest owes nothing through it.
type Manifest struct {
	root string
	spec *reviewscope.LaneManifest
}

func (m *Manifest) existingFiles() []string {
	if m == nil {
		return nil
	}

	files := m.presentFiles()
	for i, file := range files {
		files[i] = filepath.Join(m.root, file)
	}
	return files
}

func (m *Manifest) headMismatch(ctx context.Context, format string) string {
	if m == nil || m.spec.Head == "" {
		return ""
	}

	output, err := shell.Run(ctx, m.root, "git", "rev-parse", "HEAD")
	if err != nil {
		return ""
	}

	head := strings.TrimSpace(output)
	if head == m.spec.Head {
		return ""
	}
	return fmt.Sprintf(format, head, m.spec.Head)
}

func (m *Manifest) presentFiles() []string {
	isAbsent := func(file string) bool { return isMissing(filepath.Join(m.root, file)) }
	return slices.DeleteFunc(append([]string{}, m.spec.Files...), isAbsent)
}

func (m *Manifest) unansweredPairs(input *HookInput) []walkEntry {
	if m == nil || m.spec.Walk == nil {
		return nil
	}

	answered, isMarking := walkAnswers(input)
	if !isMarking {
		return nil
	}

	isAnswered := func(pair walkEntry) bool { return slices.Contains(answered, pair) }
	return slices.DeleteFunc(m.walkPairs(), isAnswered)
}

func (m *Manifest) walkPairs() []walkEntry {
	groups := m.spec.Walk.Groups
	if m.spec.Walk.Shape == contextdocs.WalkShapeGroupFirst {
		return groupPairs("", groups)
	}

	pairs := make([]walkEntry, 0, len(m.spec.Files)*len(groups))
	for _, file := range m.spec.Files {
		pairs = append(pairs, groupPairs(file, groups)...)
	}
	return pairs
}

func (m *Manifest) writeCoverage(artifact string) error {
	if m == nil {
		return nil
	}

	coverage := &laneCoverage{Files: m.presentFiles()}
	data, err := json.MarshalIndent(coverage, "", "  ")
	if err != nil {
		return fmt.Errorf("Manifest.writeCoverage: Failed to encode: %w", err)
	}

	path := artifact + coverageSuffix
	if err := fsx.WriteFileAtomic(append(data, '\n'), path, 0o644); err != nil {
		return fmt.Errorf("Manifest.writeCoverage: Failed to write %s: %w", path, err)
	}
	return nil
}

// walkEntry is one checklist pair; the file is empty on group-first.
type walkEntry struct {
	File  string `json:"file"`
	Group string `json:"group"`
}

func (e walkEntry) displayFile() string {
	return cmp.Or(e.File, walkNoFile)
}

func groupPairs(file string, groups []contextdocs.WalkGroup) []walkEntry {
	pairs := make([]walkEntry, 0, len(groups))
	for _, group := range groups {
		pairs = append(pairs, walkEntry{File: file, Group: group.Id})
	}
	return pairs
}

func loadManifest(input *HookInput) *Manifest {
	target := input.ToolInput.FilePath
	if !input.isWrite() || target == "" {
		return nil
	}

	spec, err := readLaneManifest(target + reviewscope.LaneManifestSuffix)
	if err != nil {
		return nil
	}

	manifest := &Manifest{
		root: cmp.Or(spec.Root, input.Cwd),
		spec: spec,
	}
	return manifest
}

func readLaneManifest(path string) (*reviewscope.LaneManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("readLaneManifest: Failed to read %s: %w", path, err)
	}

	spec := &reviewscope.LaneManifest{}
	if err := json.Unmarshal(data, spec); err != nil {
		return nil, fmt.Errorf("readLaneManifest: Failed to decode %s: %w", path, err)
	}
	return spec, nil
}

func readWalk(path string) []walkEntry {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	artifact := &findingsArtifact{}
	if err := json.Unmarshal(data, artifact); err != nil {
		return nil
	}
	return artifact.Walk
}

func walkAnswers(input *HookInput) ([]walkEntry, bool) {
	tool := input.ToolInput
	answered := readWalk(tool.FilePath)
	switch input.ToolName {
	case toolWrite:
		written := &findingsArtifact{}
		if err := json.Unmarshal([]byte(tool.Content), written); err != nil {
			return nil, false
		}
		return append(answered, written.Walk...), written.WalkComplete
	case toolEdit:
		isMarking := strings.Contains(tool.NewString, walkMarkSpaced) || strings.Contains(tool.NewString, walkMarkTight)
		return answered, isMarking
	}
	return nil, false
}

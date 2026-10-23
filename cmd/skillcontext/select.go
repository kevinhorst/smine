package main

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/contextdocs"
	"github.com/kevinhorst/smine/internal/contextrequired"
	"github.com/kevinhorst/smine/internal/hookconfig"
	"github.com/kevinhorst/smine/internal/transcript"
)

func applyMap(args string, declared []string, defaultSlice string, mapPath string) ([]string, string) {
	laneMap, err := contextdocs.LoadLaneMap(mapPath)
	if err != nil {
		return declared, ""
	}

	kind, names := sliceTokens(args)
	if len(names) == 0 {
		return defaultSliceOf(declared, defaultSlice, laneMap)
	}

	tokens := make([]string, 0)
	for _, name := range names {
		tokens = append(tokens, laneMap.Lanes[name]...)
	}
	if len(tokens) == 0 {
		return declared, ""
	}
	return uniqueSorted(tokens), kind + ":" + strings.Join(names, "+")
}

func declaration(skillMd string) ([]string, string, error) {
	data, err := os.ReadFile(skillMd)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("declaration: Failed to read %s: %w", skillMd, err)
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 || lines[0] != frontmatterFence {
		return nil, "", nil
	}

	var declared []string
	version := ""
	for _, line := range lines[1:] {
		if line == frontmatterFence {
			break
		}
		if value, isDeclaration := strings.CutPrefix(line, declarationKey); isDeclaration && declared == nil {
			declared = splitTokens(value)
		}
		if value, isVersion := strings.CutPrefix(line, versionKey); isVersion && version == "" {
			version = strings.TrimSpace(value)
		}
	}
	return declared, version, nil
}

// defaultSliceOf returns a smartskill's default slice, else the declaration.
func defaultSliceOf(declared []string, defaultSlice string, laneMap *contextdocs.LaneMap) ([]string, string) {
	tokens := laneMap.Lanes[defaultSlice]
	if len(tokens) == 0 {
		return declared, ""
	}
	return uniqueSorted(tokens), "topic:" + defaultSlice
}

// indexPath is the process-env override, else the index under the configured context dir; "" when neither is set.
func indexPath(contextDir string, cwd string) string {
	if override := os.Getenv(indexKey); override != "" {
		return override
	}
	if contextDir == "" {
		return ""
	}
	return filepath.Join(cwd, contextDir, contextdocs.ContextFileName)
}

func inject(claudeHome string, cfg *hookconfig.Config, input *HookInput, call *Invocation, skillsDir string) (string, error) {
	skillDir := filepath.Join(skillsDir, call.Skill)
	declared, version, err := declaration(filepath.Join(skillDir, skillFileName))
	if err != nil {
		return "", fmt.Errorf("inject: %w", err)
	}
	if len(declared) == 0 {
		return "", nil
	}

	path := indexPath(cfg.Context.ContextDir, input.Cwd)
	if path == "" {
		return noContextDirFormat, nil
	}

	mapPath := cmp.Or(os.Getenv(mapKey), filepath.Join(skillDir, mapFileName))
	tokens, lane := applyMap(call.Args, declared, cfg.Skillcontext.DefaultSlice, mapPath)
	index, err := loadIndex(path)
	if err != nil {
		return "", fmt.Errorf("inject: %w", err)
	}
	if index == nil {
		return "", nil
	}

	entries := contextdocs.ResolveGlobs(index, tokens)
	if len(entries) == 0 {
		return "", nil
	}

	pruneSessions(claudeHome, cfg.Readgate.LedgerRetentionMs.Duration())
	injection := &Injection{
		agentId:    input.AgentId,
		claudeHome: claudeHome,
		entries:    entries,
		hookConfig: cfg,
		lane:       lane,
		sessionId:  input.SessionId,
		version:    version,
		skill:      call.Skill,
	}
	addition, err := injection.Render()
	if err != nil {
		return "", fmt.Errorf("inject: %w", err)
	}
	return addition, nil
}

// loadIndex reads a context index; a missing file is nil.
func loadIndex(path string) (*contextdocs.ContextFile, error) {
	index, err := contextdocs.LoadContextFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loadIndex: %w", err)
	}
	return index, nil
}

func pruneSessions(claudeHome string, retention time.Duration) {
	root := contextrequired.Root(claudeHome)
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-retention)
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || !entry.IsDir() {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.RemoveAll(filepath.Join(root, entry.Name()))
		}
	}
}

// sliceTokens returns the args' slice kind and names; the kind is the first sorted token's.
func sliceTokens(args string) (string, []string) {
	tokens := transcript.SliceTokens(args)
	if len(tokens) == 0 {
		return "", nil
	}

	names := make([]string, 0, len(tokens))
	for _, token := range tokens {
		_, name, _ := strings.Cut(token, ":")
		names = append(names, name)
	}

	kind, _, _ := strings.Cut(tokens[0], ":")
	return kind, uniqueSorted(names)
}

// splitTokens splits a comma list, trimmed, empties dropped.
func splitTokens(value string) []string {
	tokens := make([]string, 0)
	for token := range strings.SplitSeq(value, ",") {
		if token = strings.TrimSpace(token); token != "" {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func uniqueSorted(values []string) []string {
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	return slices.Compact(sorted)
}

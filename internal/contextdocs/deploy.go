package contextdocs

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kevinhorst/smine/internal/acdsl"
	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/trash"
)

// baselineHeader heads every projected markdown file; its marker is what origin detection reads.
const baselineHeader = "<!-- synced from smine — do not edit; repo-owned files in this dir are overlays (see README.md) -->"

// The context tree's entry groups; only these project, global/ and plans/ never do.
const (
	groupActions = "actions"
	groupFacts   = "facts"
	groupRules   = "rules"
)

var projectedGroups = map[string]bool{groupActions: true, groupFacts: true, groupRules: true}

var contextDirPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// DeployOptions names one registry repo and the source checkout the engine builds from.
type DeployOptions struct {
	ContextDeploy

	Name       string
	RepoPath   string
	SourceRoot string
	StateDir   string
	Trash      *trash.Store // the stash every overwrite in the target repo and the always pack keeps its previous bytes in
}

// A sub-option is validated by the caller (the form, the record); the engine checks only what it cannot run without.
func (o *DeployOptions) validate() error {
	if o.Name == "" {
		return errors.New("Deploy: Missing registry name")
	}
	if !contextDirPattern.MatchString(o.ContextDir) {
		return fmt.Errorf("Deploy: Invalid context folder %q", o.ContextDir)
	}
	if o.StateDir == "" {
		return errors.New("Deploy: Missing state dir")
	}
	info, err := os.Stat(o.RepoPath)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("Deploy: Repo path %s is not a directory", o.RepoPath)
	}
	return nil
}

// Deploy builds a repo's overlay in the state dir and, with WriteRepo, writes the context files into its working tree; git never runs.
func Deploy(ctx context.Context, options *DeployOptions) (string, error) {
	if err := options.validate(); err != nil {
		return "", err
	}
	report, err := refreshAlwaysPack(options)
	if err != nil {
		return "", err
	}

	index, err := buildIndex(options)
	if err != nil {
		return "", err
	}
	files, err := renderOverlayFiles(index, options.ContextDir)
	if err != nil {
		return "", err
	}
	recordLines, err := writeRecord(files, options)
	if err != nil {
		return "", err
	}
	report = append(report, recordLines...)
	if !options.WriteRepo {
		report = append(report, fmt.Sprintf("  %s not written (write to repo off)", options.RepoPath))
		return strings.Join(report, "\n"), nil
	}

	repoLines, err := writeRepo(ctx, files, index, options)
	if err != nil {
		return strings.Join(append(report, repoLines...), "\n"), err
	}
	return strings.Join(append(report, repoLines...), "\n"), nil
}

func buildIndex(options *DeployOptions) (*ContextFile, error) {
	sourceIndex := filepath.Join(options.SourceRoot, SourceContextDir, ContextFileName)
	source, err := LoadContextFile(sourceIndex)
	if err != nil {
		return nil, fmt.Errorf("Deploy: Source index: %w", err)
	}
	index := FilterIndex(source, options.Name, options.Langs, options.ContextDir)
	if err := MergeRepoOwned(index, filepath.Join(options.RepoPath, options.ContextDir), options.SourceRoot); err != nil {
		return nil, fmt.Errorf("Deploy: %w", err)
	}

	record := options.ContextDeploy
	record.Role = cmp.Or(record.Role, DefaultRole)
	index.Deploy = &record
	return index, nil
}

func refreshAlwaysPack(options *DeployOptions) ([]string, error) {
	packFile, err := AlwaysPackFile()
	if err != nil {
		return nil, fmt.Errorf("Deploy: %w", err)
	}
	isWritten, err := WriteAlwaysPack(filepath.Join(options.SourceRoot, SourceContextDir), packFile, options.Trash)
	if err != nil {
		return nil, fmt.Errorf("Deploy: %w", err)
	}
	if !isWritten {
		return nil, nil
	}
	return []string{"  always pack -> " + packFile}, nil
}

func renderOverlayFiles(index *ContextFile, contextDir string) ([]OverlayFile, error) {
	indexData, err := index.Render()
	if err != nil {
		return nil, fmt.Errorf("Deploy: %w", err)
	}
	aspectsData, err := RenderAspects(index.Aspects)
	if err != nil {
		return nil, fmt.Errorf("Deploy: %w", err)
	}
	files := []OverlayFile{
		{Content: indexData, Path: path.Join(contextDir, ContextFileName)},
		{Content: aspectsData, Path: path.Join(contextDir, AspectsFileName)},
	}
	return files, nil
}

// The record dir holds the two files under their bare names and the patch that adds them under the context folder.
func writeRecord(files []OverlayFile, options *DeployOptions) ([]string, error) {
	recordDir := filepath.Dir(DeployRecordPath(options.StateDir, options.Name))
	var report []string
	for _, file := range files {
		destination := filepath.Join(recordDir, path.Base(file.Path))
		if err := writeAtomic(destination, file.Content); err != nil {
			return nil, err
		}
		report = append(report, fmt.Sprintf("  %s -> %s", path.Base(file.Path), destination))
	}
	patch, err := RenderOverlayPatch(files)
	if err != nil {
		return nil, fmt.Errorf("Deploy: %w", err)
	}
	patchPath := filepath.Join(recordDir, OverlayPatchName)
	if err := writeAtomic(patchPath, patch); err != nil {
		return nil, err
	}
	report = append(report, fmt.Sprintf("  %s -> %s (adds %d file(s) under %s/)", OverlayPatchName, patchPath, len(files), options.ContextDir))
	return report, nil
}

func writeRepo(ctx context.Context, files []OverlayFile, index *ContextFile, options *DeployOptions) ([]string, error) {
	var report []string
	for _, file := range files {
		destination := filepath.Join(options.RepoPath, filepath.FromSlash(file.Path))
		if err := writeAtomic(destination, file.Content); err != nil {
			return report, err
		}
		report = append(report, fmt.Sprintf("  %s -> %s", file.Path, destination))
	}
	if options.Markdown {
		lines, err := projectMarkdown(index, options)
		report = append(report, lines...)
		if err != nil {
			return report, err
		}
	}
	if options.Acdsl {
		lines, err := acdsl.Dist(ctx, options.SourceRoot, options.Name, options.RepoPath, options.Task)
		report = append(report, lines...)
		if err != nil {
			return report, fmt.Errorf("Deploy: %w", err)
		}
	}
	return report, nil
}

// projectMarkdown writes AGENTS.md and the filtered markdown tree; a repo-owned file under a baseline name aborts the run.
func projectMarkdown(index *ContextFile, options *DeployOptions) ([]string, error) {
	sourceDir := filepath.Join(options.SourceRoot, SourceContextDir)
	template, err := os.ReadFile(filepath.Join(sourceDir, "AGENTS.md"))
	if err != nil {
		return nil, fmt.Errorf("projectMarkdown: %w", err)
	}
	agents := RenderAgentsTemplate(string(template), index.Deploy.Role, options.ContextDir, options.Langs)
	agentsPath := filepath.Join(options.RepoPath, "AGENTS.md")
	if err := writeAtomic(agentsPath, []byte(agents)); err != nil {
		return nil, err
	}
	if err := removeAgentsSymlink(options.RepoPath); err != nil {
		return nil, err
	}
	report := []string{"  AGENTS.md -> " + agentsPath}

	groups, err := Scan(sourceDir)
	if err != nil {
		return report, fmt.Errorf("projectMarkdown: %w", err)
	}
	for _, group := range groups {
		lines, err := projectGroup(group, sourceDir, index, options)
		report = append(report, lines...)
		if err != nil {
			return report, err
		}
	}
	return report, nil
}

func projectGroup(group Group, sourceDir string, index *ContextFile, options *DeployOptions) ([]string, error) {
	if !projectedGroups[group.Name] {
		return nil, nil
	}
	contextRoot := filepath.Join(options.RepoPath, options.ContextDir)
	var report []string
	for _, file := range group.Files {
		content, err := os.ReadFile(filepath.Join(sourceDir, group.Name, file))
		if err != nil {
			return report, fmt.Errorf("projectGroup: %w", err)
		}
		filtered := FilterRulesFile(string(content), options.Name, index.Aspects, options.Langs)
		if !isProjected(group.Name, strings.TrimSuffix(file, ".md"), filtered, index.Aspects, options.Langs) {
			continue
		}
		destination := filepath.Join(contextRoot, group.Name, file)
		if err := writeBaselineFile(destination, filtered, options.Trash); err != nil {
			return report, err
		}
		report = append(report, fmt.Sprintf("  %s/%s -> %s", group.Name, file, destination))
	}
	return report, nil
}

// Actions always project, a guide with its language, a facts file only when a FACT entry survives the filter.
func isProjected(group string, name string, filtered string, aspects []RuleAspect, langs []string) bool {
	switch group {
	case groupActions:
		return true
	case groupRules:
		return isGuideSelected(name, Langs(aspects), toSet(langs))
	case groupFacts:
		return strings.HasPrefix(filtered, "**FACT-") || strings.Contains(filtered, "\n**FACT-")
	}
	return false
}

// A destination whose first line lacks the synced marker is the repo's own file and is never overwritten.
func writeBaselineFile(destination string, filtered string, keep *trash.Store) error {
	existing, err := os.ReadFile(destination)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("writeBaselineFile: %w", err)
	}
	headline, _, _ := strings.Cut(string(existing), "\n")
	isRepoOwned := err == nil && !strings.Contains(headline, BaselineHeaderMarker)
	if isRepoOwned {
		return fmt.Errorf("writeBaselineFile: %s is repo-owned but collides with a baseline file name", destination)
	}
	wanted := baselineHeader + "\n\n" + filtered
	if string(existing) == wanted {
		return nil
	}
	entry := &trash.Entry{Action: actionContextSync, Reason: "baseline file rewritten in the target repo"}
	if _, err := keep.Keep(destination, entry); err != nil {
		return fmt.Errorf("writeBaselineFile: %w", err)
	}
	return writeAtomic(destination, []byte(wanted))
}

// Claude Code reads AGENTS.md directly and skips it behind a CLAUDE.md; only our own symlink is removed.
func removeAgentsSymlink(repoPath string) error {
	link := filepath.Join(repoPath, "CLAUDE.md")
	target, err := os.Readlink(link)
	if err != nil || target != "AGENTS.md" {
		return nil
	}
	if err := os.Remove(link); err != nil {
		return fmt.Errorf("removeAgentsSymlink: %w", err)
	}
	return nil
}

func writeAtomic(destination string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("Deploy: %w", err)
	}
	if err := fsx.WriteFileAtomic(content, destination, 0o644); err != nil {
		return fmt.Errorf("Deploy: %w", err)
	}
	return nil
}

package repos

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/shell"
)

const (
	backupStampLayout  = "20060102-150405"
	entryEnd           = "</entry>"
	recentProjectsFile = "recentProjects.xml"
	versionChars       = "0123456789."
)

var (
	// jetbrainsApps lists only products whose app-bundle name differs from the config dir.
	jetbrainsApps = map[string]string{
		"AndroidStudio": "Android Studio",
		"IdeaIC":        "IntelliJ IDEA CE",
		"IntelliJIdea":  "IntelliJ IDEA",
		"PyCharmCE":     "PyCharm CE",
	}
	// worktreeEntryPattern opens a Recent Projects entry of a Claude worktree.
	worktreeEntryPattern = regexp.MustCompile(`<entry key="[^"]*/\.claude/worktrees/`)
)

// jetbrainsPrune skips a running IDE's file unless forced: the IDE rewrites it on exit.
type jetbrainsPrune struct {
	changed   int
	configDir string
	isDryRun  bool
	isForce   bool
	isRunning func(ctx context.Context, product string) bool
	lines     []string
	now       time.Time
	removed   int
	skipped   int
}

// pruneFile leaves a symlinked file alone, so the atomic replace never turns a link into a copy.
func (p *jetbrainsPrune) pruneFile(ctx context.Context, path string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}

	rel := strings.TrimPrefix(path, p.configDir+string(filepath.Separator))
	data, err := os.ReadFile(path)
	if err != nil {
		p.report("skipping %s: %v", rel, err)
		return nil
	}

	kept, count := dropWorktreeEntries(string(data))
	if count == 0 {
		return nil
	}

	product := productName(rel)
	if p.isRunning(ctx, product) {
		switch {
		case p.isForce:
			p.report("warning: %s is running; --force given, pruning anyway (edit may be clobbered on exit)", product)
		case !p.isDryRun:
			p.report("skipping %s: %s is running (quit it or pass --force)", rel, product)
			p.skipped++
			return nil
		}
	}

	p.removed += count
	if p.isDryRun {
		p.report("would remove %s: %s", entryCount(count), rel)
		return nil
	}

	backup := path + ".bak." + p.now.Format(backupStampLayout)
	if err := os.WriteFile(backup, data, info.Mode().Perm()); err != nil {
		return fmt.Errorf("jetbrainsPrune.pruneFile: Failed to write backup: %w", err)
	}

	if err := fsx.WriteFileAtomic([]byte(kept), path, info.Mode().Perm()); err != nil {
		return fmt.Errorf("jetbrainsPrune.pruneFile: %w", err)
	}

	p.changed++
	p.report("removed %s: %s (backup: %s)", entryCount(count), rel, filepath.Base(backup))
	return nil
}

func (p *jetbrainsPrune) report(format string, args ...any) {
	p.lines = append(p.lines, fmt.Sprintf(format, args...))
}

// run prunes every IDE's file and returns the report.
func (p *jetbrainsPrune) run(ctx context.Context) (string, error) {
	info, err := os.Stat(p.configDir)
	if err != nil || !info.IsDir() {
		return fmt.Sprintf("no JetBrains config dir at %s; nothing to do\n", p.configDir), nil
	}

	files, err := filepath.Glob(filepath.Join(p.configDir, "*", "options", recentProjectsFile))
	if err != nil {
		return "", fmt.Errorf("jetbrainsPrune.run: %w", err)
	}

	for _, path := range files {
		if err := p.pruneFile(ctx, path); err != nil {
			return p.text(), err
		}
	}

	p.report("%s", p.summary())
	return p.text(), nil
}

func (p *jetbrainsPrune) summary() string {
	skippedNote := ""
	if p.skipped > 0 {
		skippedNote = fmt.Sprintf(", skipped %d running IDE(s)", p.skipped)
	}

	switch {
	case p.removed == 0:
		return "no worktree entries found; nothing to prune" + skippedNote
	case p.isDryRun:
		return fmt.Sprintf("dry run: %s would be removed", entryCount(p.removed))
	}
	return fmt.Sprintf("done: removed %s across %d file(s)%s", entryCount(p.removed), p.changed, skippedNote)
}

func (p *jetbrainsPrune) text() string {
	return strings.Join(p.lines, "\n") + "\n"
}

// appPattern is the pgrep -f pattern matching the product's app bundle.
func appPattern(product string) string {
	return cmp.Or(jetbrainsApps[product], product) + `[^/]*\.app/Contents/MacOS`
}

func dropWorktreeEntries(content string) (string, int) {
	var kept strings.Builder
	count := 0
	isSkipping := false
	for line := range strings.SplitAfterSeq(content, "\n") {
		if worktreeEntryPattern.MatchString(line) {
			count++
			isSkipping = true
		}

		if !isSkipping {
			kept.WriteString(line)
			continue
		}

		if strings.Contains(line, entryEnd) {
			isSkipping = false
		}
	}
	return kept.String(), count
}

func entryCount(count int) string {
	if count == 1 {
		return "1 worktree entry"
	}
	return fmt.Sprintf("%d worktree entries", count)
}

// isIdeRunning reports whether the product's app bundle has a live process.
func isIdeRunning(ctx context.Context, product string) bool {
	_, err := shell.Run(ctx, "", "pgrep", "-f", appPattern(product))
	return err == nil
}

// productName maps GoLand2026.1/options/recentProjects.xml to GoLand.
func productName(rel string) string {
	dir, _, _ := strings.Cut(rel, string(filepath.Separator))
	index := strings.IndexAny(dir, versionChars)
	if index < 0 {
		return dir
	}
	return dir[:index]
}

func PruneJetbrains(ctx context.Context, dryRun, force bool) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("PruneJetbrains: %w", err)
	}

	prune := &jetbrainsPrune{
		configDir: filepath.Join(home, "Library", "Application Support", "JetBrains"),
		isDryRun:  dryRun,
		isForce:   force,
		isRunning: isIdeRunning,
		now:       time.Now(),
	}
	output, err := prune.run(ctx)
	if err != nil {
		return output, fmt.Errorf("PruneJetbrains: %w", err)
	}
	return output, nil
}

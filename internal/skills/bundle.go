package skills

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/fsx"
	"github.com/kevinhorst/smine/internal/skillgraph"
)

const (
	// bundleIndexName is the index file Bundle writes at the bundle root.
	bundleIndexName = "README.md"
	// unfilledReferenceNeedle is a $SKILL_ reference that survived the render; a placeholder reads its names through awk's ENVIRON, so none is in a block.
	unfilledReferenceNeedle = `\$SKILL_[A-Z0-9]+_[A-Z][A-Z0-9_]*`
)

// ErrBundleReferences refuses a bundle whose written file names a smine program, a this-repo make target or an unfilled variable.
var ErrBundleReferences = errors.New("bundle carries smine references")

// makeTargetPattern reads the Makefile's documented targets: the `## <target>:` help lines.
var makeTargetPattern = regexp.MustCompile(`(?m)^## ([a-z][a-z0-9-]*):`)

// bundleIndex is the README.md of dist/skills: per shipped skill what the full install adds, per skipped skill what it needs.
func (r *deployRun) bundleIndex(sources []*sourceLeaf) string {
	var index strings.Builder
	index.WriteString("# Skills\n\nEvery skill here runs without smine; a repo-scoped value is read from the shell (`export SKILL_…=…`).\n\n")
	for _, source := range sources {
		values := r.values(true, source)
		switch {
		case bundleSkip(source, values) == "":
			fmt.Fprintf(&index, "- %s%s\n", source.name(), fullInstallAdds(values))
		default:
			fmt.Fprintf(&index, "- %s (skipped): needs %s\n", source.name(), skippedNeeds(source, values))
		}
	}
	return index.String()
}

// bundleLeaf writes a skill for the empty component set only when isBundled admits it.
func (r *deployRun) bundleLeaf(outDir string, source *sourceLeaf) error {
	if !r.isBundled(source) {
		return nil
	}

	files, err := r.leafFiles(nil, true, source, "")
	if err != nil {
		return err
	}

	targetDir := filepath.Join(outDir, source.name())
	for _, file := range files {
		path := filepath.Join(targetDir, filepath.FromSlash(file.path))
		if err := fsx.WriteFileAtomic(file.content, path, file.mode); err != nil {
			return fmt.Errorf("deployRun.bundleLeaf: %w", err)
		}
	}

	r.logf("  %s -> %s", source.name(), targetDir)
	return nil
}

// isBundled admits a skill that requires no component, carries no license key and lacks no required install-scoped variable, logging each skip.
func (r *deployRun) isBundled(source *sourceLeaf) bool {
	reason := bundleSkip(source, r.values(true, source))
	if reason != "" {
		r.logf("  skip %s %s", source.name(), reason)
	}
	return reason == ""
}

// bundleScanPattern is the smine program needles, an unfilled variable reference and `make <target>` for every documented target of repoDir's Makefile.
func bundleScanPattern(repoDir string) (*regexp.Regexp, error) {
	makefile, err := os.ReadFile(filepath.Join(repoDir, "Makefile"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("bundleScanPattern: %w", err)
	}

	alternatives := slices.Clone(skillgraph.LiteralNeedles)
	alternatives = append(alternatives, unfilledReferenceNeedle)
	for _, match := range makeTargetPattern.FindAllStringSubmatch(string(makefile), -1) {
		alternatives = append(alternatives, `make `+regexp.QuoteMeta(match[1])+`\b`)
	}
	return regexp.MustCompile(strings.Join(alternatives, "|")), nil
}

// bundleSkip is why the bundle leaves a skill out, empty when it ships; values come from a deferring resolution, so repo-scoped ones never skip it.
func bundleSkip(source *sourceLeaf, values []VariableValue) string {
	unconfigured := Unconfigured(values)
	switch {
	case len(source.skill.Requires) > 0:
		return "(requires " + joinIds(source.skill.Requires) + ")"
	case source.skill.Field(fieldLicense) != "":
		return "(license)"
	case len(unconfigured) > 0:
		return "unconfigured (" + strings.Join(unconfigured, ", ") + ")"
	}
	return ""
}

// fullInstallAdds is the index tail naming what the optional install-scoped variables left unset would add; empty when none.
func fullInstallAdds(values []VariableValue) string {
	var summaries []string
	for index := range values {
		value := &values[index]
		isUnsetOptional := !value.Required && value.Value == ""
		if !isUnsetOptional || value.IsDeferred {
			continue
		}
		summaries = append(summaries, summaryClause(value))
	}
	if len(summaries) == 0 {
		return ""
	}
	return ": with the full install also " + strings.Join(summaries, ", ")
}

// newBundleRun is the empty-set run over repoDir's skills with deployability computed, plus the loaded leaves.
func newBundleRun(repoDir string) (*deployRun, []*sourceLeaf, error) {
	options := &SyncOptions{Components: []components.ID{}, RepoDir: repoDir}
	run := &deployRun{contextDir: contextDirDefault, deferredHint: deferredHintShell, options: options, report: &strings.Builder{}}
	sources, err := run.repoLeaves()
	if err != nil {
		return run, nil, err
	}

	run.deployable = run.deployableLeaves(sources)
	return run, sources, nil
}

// requireEmptyDir refuses a dir holding anything; a missing dir passes.
func requireEmptyDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("requireEmptyDir: %w", err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("requireEmptyDir: %s is not empty — remove it first", dir)
	}
	return nil
}

// scanBundle returns one "path:line: term" hit per match, paths relative to outDir.
func scanBundle(outDir string, pattern *regexp.Regexp) ([]string, error) {
	var hits []string
	err := filepath.WalkDir(outDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.Type().IsRegular() {
			return nil
		}

		rel, err := filepath.Rel(outDir, path)
		if err != nil {
			return fmt.Errorf("scanBundle: %w", err)
		}

		fileHits, err := scanFile(path, pattern, filepath.ToSlash(rel))
		if err != nil {
			return err
		}

		hits = append(hits, fileHits...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scanBundle: %w", err)
	}
	return hits, nil
}

// scanContent returns one "rel:line: term" hit per match of pattern in content.
func scanContent(content []byte, pattern *regexp.Regexp, rel string) []string {
	var hits []string
	lines := strings.Split(string(content), "\n")
	for index, line := range lines {
		for _, term := range pattern.FindAllString(line, -1) {
			hits = append(hits, fmt.Sprintf("%s:%d: %s", rel, index+1, term))
		}
	}
	return hits
}

// scanFile returns the bundle scan hits of one file, labeled with rel.
func scanFile(path string, pattern *regexp.Regexp, rel string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("scanFile: %w", err)
	}
	return scanContent(content, pattern, rel), nil
}

// skippedNeeds names what a skipped skill needs: its required components, a license, or each unset required variable's interface and provider component.
func skippedNeeds(source *sourceLeaf, values []VariableValue) string {
	switch {
	case len(source.skill.Requires) == 1:
		return "the " + joinIds(source.skill.Requires) + " component"
	case len(source.skill.Requires) > 1:
		return "the " + joinIds(source.skill.Requires) + " components"
	case source.skill.Field(fieldLicense) != "":
		return "a license"
	}

	catalog := source.skill.Catalog
	if catalog == nil {
		catalog = &skillgraph.Catalog{}
	}
	var needs []string
	for index := range values {
		value := &values[index]
		isUnsetRequired := value.Required && value.Value == ""
		if !isUnsetRequired || value.IsDeferred {
			continue
		}
		needs = append(needs, variableNeed(catalog, value))
	}
	return strings.Join(needs, ", ")
}

// summaryClause is the interface summary of value as a lower-case clause without the closing period; the variable name when the interface is unknown.
func summaryClause(value *VariableValue) string {
	if value.Interface == nil {
		return value.Name
	}

	summary := strings.TrimSuffix(value.Interface.Summary, ".")
	first, size := utf8.DecodeRuneInString(summary)
	if size == 0 {
		return value.Name
	}
	return string(unicode.ToLower(first)) + summary[size:]
}

// variableNeed is "<interface> from the <component> component", the interface alone without a providing component, the variable name without an interface.
func variableNeed(catalog *skillgraph.Catalog, value *VariableValue) string {
	if value.Interface == nil {
		return value.Name
	}

	ref := value.Interface.Ref()
	provider := catalog.ProviderFor(ref)
	if provider == nil || provider.Component == "" {
		return ref
	}
	return ref + " from the " + string(provider.Component) + " component"
}

// Bundle renders component-free skills without a deploy marker plus the README.md index, refuses a non-empty outDir, and returns the report also on failure (ErrBundleReferences, one line per hit).
func Bundle(outDir, repoDir string) (string, error) {
	if err := requireEmptyDir(outDir); err != nil {
		return "", fmt.Errorf("Bundle: %w", err)
	}

	pattern, err := bundleScanPattern(repoDir)
	if err != nil {
		return "", fmt.Errorf("Bundle: %w", err)
	}

	run, sources, err := newBundleRun(repoDir)
	if err != nil {
		return run.report.String(), fmt.Errorf("Bundle: %w", err)
	}

	for _, source := range sources {
		if err := run.bundleLeaf(outDir, source); err != nil {
			return run.report.String(), fmt.Errorf("Bundle: %w", err)
		}
	}

	index := []byte(run.bundleIndex(sources))
	indexPath := filepath.Join(outDir, bundleIndexName)
	if err := fsx.WriteFileAtomic(index, indexPath, renderedFileMode); err != nil {
		return run.report.String(), fmt.Errorf("Bundle: %w", err)
	}

	run.logf("  index -> %s", indexPath)
	hits, err := scanBundle(outDir, pattern)
	if err != nil {
		return run.report.String(), fmt.Errorf("Bundle: %w", err)
	}
	if len(hits) > 0 {
		return run.report.String(), fmt.Errorf("Bundle: %d hit(s): %w\n%s", len(hits), ErrBundleReferences, strings.Join(hits, "\n"))
	}
	return run.report.String(), nil
}

// CheckBundle renders every bundled skill for the empty set — the smineless form — in memory and returns one "leaf/path:line: term" hit per match.
func CheckBundle(repoDir string) ([]string, error) {
	pattern, err := bundleScanPattern(repoDir)
	if err != nil {
		return nil, fmt.Errorf("CheckBundle: %w", err)
	}

	run, sources, err := newBundleRun(repoDir)
	if err != nil {
		return nil, fmt.Errorf("CheckBundle: %w", err)
	}

	var hits []string
	for _, source := range sources {
		if !run.isBundled(source) {
			continue
		}

		files, err := run.leafFiles(nil, true, source, "")
		if err != nil {
			return nil, fmt.Errorf("CheckBundle: %w", err)
		}
		for _, file := range files {
			hits = append(hits, scanContent(file.content, pattern, source.name()+"/"+file.path)...)
		}
	}
	return hits, nil
}

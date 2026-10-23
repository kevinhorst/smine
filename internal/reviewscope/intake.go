package reviewscope

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

const (
	intakeLaneCount     = 2
	intakeLaneEffort    = "medium"
	intakeMarkerLines   = 5
	intakeNumstatFields = 3
	intakeMaxFiles      = 50
	intakeMaxLines      = 4000
	intakeStationEffort = "high"

	classBinary    = "binary"
	classGenerated = "generated"
	classLockfile  = "lockfile"
	classMixed     = "mixed"
	classProse     = "prose"
	classRecords   = "records"
	classVendored  = "vendored"

	defaultExcluded = "excluded"
	defaultKept     = "kept"
)

var (
	generatedMarkerPattern = regexp.MustCompile(`Code generated|DO NOT EDIT|@generated`)
	generatedPathPattern   = regexp.MustCompile(`\.pb\.go$|_gen\.go$|(^|/)[^/]+\.gen\.[^/]+$`)
	lockfileNames          = []string{"Cargo.lock", "Gemfile.lock", "composer.lock", "go.sum", "package-lock.json", "pnpm-lock.yaml", "poetry.lock", "yarn.lock"}
	prosePattern           = regexp.MustCompile(`\.(md|rst|txt)$`)
	recordsPattern         = regexp.MustCompile(`(^|/)(evals|plans|proposals|sessions)/|\.jsonl$`)
	vendoredPattern        = regexp.MustCompile(`(^|/)(node_modules|third_party|vendor)/`)
)

// ChangedFile is one numstat row with its exclusion class, empty when the file is reviewable.
type ChangedFile struct {
	Added     int
	Class     string
	Deleted   int
	IsBinary  bool
	IsRemoved bool
	Path      string
}

func (f *ChangedFile) lines() int {
	return f.Added + f.Deleted
}

// Exclusion is one candidate row of the report, a file or a collapsed directory.
type Exclusion struct {
	Class    string
	Default  string
	Files    int
	Lines    int
	Pathspec string
}

type IntakeReport struct {
	Base       string
	Exclusions []Exclusion
	Files      []ChangedFile
	Head       string
	Model      string
}

// chunkCount is the number of chunks the default bounds need over the reviewed files.
func (r *IntakeReport) chunkCount() int {
	files, lines := r.reviewedTotals()
	return max(ceilDiv(files, intakeMaxFiles), ceilDiv(lines, intakeMaxLines))
}

// chunkSuggestion is the chunks bracket the bounds imply over the reviewed files, empty within bounds.
func (r *IntakeReport) chunkSuggestion() string {
	chunks := r.chunkCount()
	if chunks <= 1 {
		return ""
	}

	files, lines := r.reviewedTotals()
	return fmt.Sprintf("layers[%d:%d]", ceilDiv(files, chunks), ceilDiv(lines, chunks))
}

func (r *IntakeReport) excludedPathspecs() []string {
	pathspecs := make([]string, 0, len(r.Exclusions))
	for _, exclusion := range r.Exclusions {
		if exclusion.Default != defaultExcluded {
			continue
		}
		pathspecs = append(pathspecs, exclusion.Pathspec)
	}
	return pathspecs
}

// reviewedTotals counts the files and lines that stay after the excluded rows, the set the Suggested line reviews.
func (r *IntakeReport) reviewedTotals() (int, int) {
	files, lines := 0, 0
	for _, file := range r.Files {
		if defaultOf(file.Class) == defaultExcluded {
			continue
		}
		files++
		lines += file.lines()
	}
	return files, lines
}

func (r *IntakeReport) totals() (int, int, int) {
	added, deleted := 0, 0
	for _, file := range r.Files {
		added += file.Added
		deleted += file.Deleted
	}
	return len(r.Files), added, deleted
}

func (r *IntakeReport) writeChunks(out *strings.Builder) {
	suggestion := r.chunkSuggestion()
	if suggestion == "" {
		fmt.Fprintf(out, "Chunks: none needed (within %d files / %d lines)\n", intakeMaxFiles, intakeMaxLines)
		return
	}
	fmt.Fprintf(out, "Chunks: %d under the default bound (%d files / %d lines) — suggested chunks: %s\n", r.chunkCount(), intakeMaxFiles, intakeMaxLines, suggestion)
}

func (r *IntakeReport) writeExclusions(out *strings.Builder) {
	if len(r.Exclusions) == 0 {
		out.WriteString("Exclusions: none\n")
		return
	}

	out.WriteString("Exclusions (candidates):\n\n| Pathspec | Class | Files | Lines | Default |\n| --- | --- | --- | --- | --- |\n")
	for _, exclusion := range r.Exclusions {
		fmt.Fprintf(out, "| %s | %s | %d | %d | %s |\n", exclusion.Pathspec, exclusion.Class, exclusion.Files, exclusion.Lines, exclusion.Default)
	}
}

func (r *IntakeReport) writeModels(out *strings.Builder) {
	if r.Model == "" {
		out.WriteString("Models: no subagent model admitted — see the skill's Model section\n")
		return
	}
	fmt.Fprintf(
		out,
		"Models: lanes %d[%s:%s], station %s:%s — strongest admitted subagent model; weaker classes are tested unviable (skill Model section)\n",
		intakeLaneCount,
		r.Model,
		intakeLaneEffort,
		r.Model,
		intakeStationEffort,
	)
}

// Render prints the gate text; isGate appends the reply line a bare invocation waits on.
func (r *IntakeReport) Render(isGate bool) string {
	var out strings.Builder
	files, added, deleted := r.totals()
	fmt.Fprintf(&out, "Diff: %s..%s — %d files, %d lines changed (%d added, %d deleted)\n\n", r.Base, r.Head, files, added+deleted, added, deleted)
	r.writeExclusions(&out)
	reviewedFiles, reviewedLines := r.reviewedTotals()
	fmt.Fprintf(&out, "\nReviewed after exclusions: %d files, %d lines\n", reviewedFiles, reviewedLines)
	r.writeChunks(&out)
	r.writeModels(&out)
	fmt.Fprintf(&out, "\nSuggested: /railroad-review%s\n", r.Suggested())
	if isGate {
		out.WriteString("\nReply go (the suggested line), defaults (no args), or /railroad-review <args>.\n")
	}
	return out.String()
}

// Suggested is the args text of the suggested invocation, empty when nothing is suggested.
func (r *IntakeReport) Suggested() string {
	var args strings.Builder
	if pathspecs := r.excludedPathspecs(); len(pathspecs) > 0 {
		args.WriteString(" exclude: " + strings.Join(pathspecs, " "))
	}
	if suggestion := r.chunkSuggestion(); suggestion != "" {
		args.WriteString(" chunks: " + suggestion)
	}
	if r.Model != "" {
		fmt.Fprintf(&args, " lanes: %d[%s:%s] station: %s:%s", intakeLaneCount, r.Model, intakeLaneEffort, r.Model, intakeStationEffort)
	}
	return args.String()
}

type IntakeRequest struct {
	Base     string
	Excludes []string
	Head     string
	Model    string
	Paths    []string
	Repo     string
}

// diffArgs is the git diff command over the request's range and pathspecs, renames split into delete and add.
func (r *IntakeRequest) diffArgs(mode ...string) []string {
	args := []string{"diff", "--no-renames"}
	args = append(args, mode...)
	args = append(args, r.Base, r.Head, "--")
	args = append(args, r.Paths...)
	if len(r.Paths) == 0 {
		args = append(args, ".")
	}
	for _, exclude := range r.Excludes {
		args = append(args, ":(exclude)"+exclude)
	}
	return args
}

// Intake computes the review-config analysis of one range from its numstat, never from hunks.
func Intake(ctx context.Context, request *IntakeRequest) (*IntakeReport, error) {
	numstat, err := shell.Run(ctx, request.Repo, "git", request.diffArgs("--numstat")...)
	if err != nil {
		return nil, fmt.Errorf("Intake: %w", err)
	}

	files, err := parseNumstat(numstat)
	if err != nil {
		return nil, fmt.Errorf("Intake: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("Intake: Empty diff between base %s and head %s: nothing to review", request.Base, request.Head)
	}

	removed, err := shell.Run(ctx, request.Repo, "git", request.diffArgs("--name-only", "--diff-filter=D")...)
	if err != nil {
		return nil, fmt.Errorf("Intake: %w", err)
	}

	removedPaths := strings.Split(strings.TrimSpace(removed), "\n")
	for index := range files {
		files[index].IsRemoved = slices.Contains(removedPaths, files[index].Path)
		class, err := classify(ctx, request, &files[index])
		if err != nil {
			return nil, fmt.Errorf("Intake: %w", err)
		}
		files[index].Class = class
	}

	report := &IntakeReport{Base: request.Base, Exclusions: collapseExclusions(files), Files: files, Head: request.Head, Model: request.Model}
	return report, nil
}

// ancestors lists a path's directories from the root down, the file's own directory last.
func ancestors(filePath string) []string {
	dirs := make([]string, 0)
	for index, char := range filePath {
		if char == '/' {
			dirs = append(dirs, filePath[:index])
		}
	}
	return dirs
}

func ceilDiv(value int, divisor int) int {
	return (value + divisor - 1) / divisor
}

// classify names a file's exclusion class, empty for a reviewable file; the generated marker needs the head blob.
func classify(ctx context.Context, request *IntakeRequest, file *ChangedFile) (string, error) {
	class := classifyPath(file)
	if class != "" {
		return class, nil
	}
	if file.IsRemoved {
		return "", nil
	}

	isMarked, err := hasGeneratedMarker(ctx, request.Repo, request.Head, file.Path)
	if err != nil {
		return "", err
	}
	if isMarked {
		return classGenerated, nil
	}
	return "", nil
}

// classifyPath is the path-only part of classify: hard classes first, then the soft ones, first match wins.
func classifyPath(file *ChangedFile) string {
	switch {
	case file.IsBinary:
		return classBinary
	case generatedPathPattern.MatchString(file.Path):
		return classGenerated
	case slices.Contains(lockfileNames, path.Base(file.Path)):
		return classLockfile
	case vendoredPattern.MatchString(file.Path):
		return classVendored
	case prosePattern.MatchString(file.Path):
		return classProse
	case recordsPattern.MatchString(file.Path):
		return classRecords
	}
	return ""
}

// collapseExclusions groups candidate files under the highest directory whose changed files are all candidates.
func collapseExclusions(files []ChangedFile) []Exclusion {
	candidatesByDir := map[string]int{}
	filesByDir := map[string]int{}
	for _, file := range files {
		for _, dir := range ancestors(file.Path) {
			filesByDir[dir]++
			if file.Class != "" {
				candidatesByDir[dir]++
			}
		}
	}

	rows := map[string]*Exclusion{}
	order := make([]string, 0)
	for _, file := range files {
		if file.Class == "" {
			continue
		}

		pathspec := collapsedPathspec(file.Path, candidatesByDir, filesByDir)
		row, isKnown := rows[pathspec]
		if !isKnown {
			row = &Exclusion{Class: file.Class, Default: defaultOf(file.Class), Pathspec: pathspec}
			rows[pathspec] = row
			order = append(order, pathspec)
		}
		if row.Class != file.Class {
			row.Class = classMixed
		}
		if defaultOf(file.Class) == defaultKept {
			row.Default = defaultKept
		}
		row.Files++
		row.Lines += file.lines()
	}

	sort.Strings(order)
	exclusions := make([]Exclusion, 0, len(order))
	for _, pathspec := range order {
		exclusions = append(exclusions, *rows[pathspec])
	}
	return exclusions
}

// collapsedPathspec is the highest ancestor whose changed files are all candidates, else the file itself.
func collapsedPathspec(filePath string, candidatesByDir map[string]int, filesByDir map[string]int) string {
	for _, dir := range ancestors(filePath) {
		if candidatesByDir[dir] == filesByDir[dir] {
			return dir + "/"
		}
	}
	return filePath
}

// defaultOf is excluded for a hard class, kept for a soft one.
func defaultOf(class string) string {
	switch class {
	case classBinary, classGenerated, classLockfile, classVendored:
		return defaultExcluded
	case classMixed, classProse, classRecords:
		return defaultKept
	}
	return defaultKept
}

func hasGeneratedMarker(ctx context.Context, repo string, head string, filePath string) (bool, error) {
	content, err := shell.Run(ctx, repo, "git", "show", head+":"+filePath)
	if err != nil {
		return false, fmt.Errorf("hasGeneratedMarker: %w", err)
	}

	lines := strings.SplitN(content, "\n", intakeMarkerLines+1)
	if len(lines) > intakeMarkerLines {
		lines = lines[:intakeMarkerLines]
	}
	return generatedMarkerPattern.MatchString(strings.Join(lines, "\n")), nil
}

// parseNumstat decodes git diff --numstat rows; a binary row carries - for both counts.
func parseNumstat(numstat string) ([]ChangedFile, error) {
	files := make([]ChangedFile, 0)
	for line := range strings.SplitSeq(strings.TrimSpace(numstat), "\n") {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", intakeNumstatFields)
		if len(fields) != intakeNumstatFields {
			return nil, fmt.Errorf("parseNumstat: Unexpected numstat row %q", line)
		}

		file := ChangedFile{Path: fields[2]}
		if fields[0] == "-" {
			file.IsBinary = true
			files = append(files, file)
			continue
		}

		added, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, fmt.Errorf("parseNumstat: Unexpected numstat row %q: %w", line, err)
		}
		deleted, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, fmt.Errorf("parseNumstat: Unexpected numstat row %q: %w", line, err)
		}
		file.Added, file.Deleted = added, deleted
		files = append(files, file)
	}
	return files, nil
}

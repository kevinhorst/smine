package repos

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

const (
	ChangeTrailer   = "Smine-Change"
	SessionsTrailer = "Smine-Sessions"

	// NightBranchPrefix is followed by <date>[-n] and one of the two suffixes.
	NightBranchPrefix = "claude-routines/smine-nightly-"
	BatchSuffix       = "-batch"
	ProposalsSuffix   = "-proposals"

	// nameStatusMinFields: a --name-status line is a status and at least one path, tab-separated.
	nameStatusMinFields = 2
	// numstatFields: a --numstat line is added, deleted and the path, tab-separated.
	numstatFields = 3
	// pickedFields: a PickedChanges record is sha, committer date and the message, unit-separated.
	pickedFields = 3
)

// trailerLine matches one git trailer line ("Key: value").
var trailerLine = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9-]*):\s*(.*)$`)

// cherryPickLine is the provenance line `git cherry-pick -x` appends inside the trailer block; git reads it as part of the block.
var cherryPickLine = regexp.MustCompile(`^\(cherry picked from commit ([0-9a-f]{7,64})\)$`)

// diffHeader opens one file's section of a `git show --patch` text.
var diffHeader = regexp.MustCompile(`^diff --git a/(.+) b/(.+)$`)

type ChangeCommit struct {
	Added     int
	Body      string
	ChangeIds []string // every <kind>/<slug> Smine-Change trailer, in order
	Deleted   int
	Files     []ChangeFile
	Kind      string // kind of the first id
	Sessions  []string
	Sha       string
	Subject   string
}

// ChangeFile is one --name-status line with that file's capped share of the patch and its numstat counts.
type ChangeFile struct {
	Added     int
	Deleted   int
	Diff      string
	IsDiffCut bool
	Path      string
	Status    string
}

// ChangeStat is a commit's size as the proposal cap measures it: files touched, lines added plus deleted.
type ChangeStat struct {
	Files int
	Lines int
}

func (s *ChangeStat) IsOverCap() bool {
	return s.Files > MaxChangeFiles || s.Lines > MaxChangeLines
}

type CommitMessage struct {
	Body      string
	ChangeIds []string
	Sessions  []string
	Subject   string
}

// PickedChange is a change commit placed on a target by a pick: Sha is the placed commit, SourceSha the proposal it came from.
type PickedChange struct {
	Date      string // committer date as YYYY-MM-DD
	Message   *CommitMessage
	Sha       string
	SourceSha string
}

// lineCount is one --numstat line's counts; a binary file's "-" counts 0.
type lineCount struct {
	Added   int
	Deleted int
}

// BranchMessages parses every commit message of target..branch in one git call, keyed by sha.
func BranchMessages(ctx context.Context, branch, repoPath, target string) (map[string]*CommitMessage, error) {
	output, err := shell.Run(ctx, repoPath, "git", "log", "--format=%H%x1f%B%x1e", branch, "^"+target)
	if err != nil {
		return nil, fmt.Errorf("BranchMessages: %s: %w", strings.TrimSpace(output), err)
	}

	messages := make(map[string]*CommitMessage)
	for record := range strings.SplitSeq(output, "\x1e") {
		sha, message, ok := strings.Cut(strings.TrimSpace(record), "\x1f")
		if !ok {
			continue
		}
		messages[sha] = ParseCommitMessage(message)
	}
	return messages, nil
}

// ListNightBranches lists one suffix's branches, newest date first; the date-stamped names sort chronologically.
func ListNightBranches(ctx context.Context, repoPath, suffix string) ([]string, error) {
	output, err := shell.Run(
		ctx,
		repoPath,
		"git",
		"for-each-ref",
		"--sort=-refname",
		"--format=%(refname:short)",
		"refs/heads/"+NightBranchPrefix+"*"+suffix,
	)
	if err != nil {
		return nil, fmt.Errorf("ListNightBranches: %s: %w", strings.TrimSpace(output), err)
	}

	branches := make([]string, 0)
	for line := range strings.Lines(output) {
		branch := strings.TrimSpace(line)
		if branch != "" {
			branches = append(branches, branch)
		}
	}
	return branches, nil
}

// PickedChanges lists the newest limit commits on target that carry a Smine-Change trailer and a cherry-pick provenance line, newest first.
func PickedChanges(ctx context.Context, limit int, repoPath, target string) ([]PickedChange, error) {
	output, err := shell.Run(
		ctx,
		repoPath,
		"git",
		"log",
		target,
		"--max-count="+strconv.Itoa(limit),
		"--grep=^"+ChangeTrailer+":",
		"--format=%H%x1f%cs%x1f%B%x1e",
	)
	if err != nil {
		return nil, fmt.Errorf("PickedChanges: %s: %w", strings.TrimSpace(output), err)
	}

	picked := make([]PickedChange, 0)
	for record := range strings.SplitSeq(output, "\x1e") {
		fields := strings.SplitN(strings.TrimSpace(record), "\x1f", pickedFields)
		if len(fields) < pickedFields {
			continue
		}

		source := pickSource(fields[2])
		parsed := ParseCommitMessage(fields[2])
		if source == "" || len(parsed.ChangeIds) == 0 {
			continue
		}
		picked = append(picked, PickedChange{Date: fields[1], Message: parsed, Sha: fields[0], SourceSha: source})
	}
	return picked, nil
}

func ParseCommitMessage(message string) *CommitMessage {
	message = strings.TrimSpace(message)
	subject, rest, _ := strings.Cut(message, "\n")
	parsed := &CommitMessage{ChangeIds: []string{}, Sessions: []string{}, Subject: strings.TrimSpace(subject)}

	rest = strings.TrimSpace(rest)
	paragraphs := strings.Split(rest, "\n\n")
	last := paragraphs[len(paragraphs)-1]
	trailers, isTrailerBlock := parseTrailers(last)
	if isTrailerBlock {
		parsed.ChangeIds = append(parsed.ChangeIds, trailers[ChangeTrailer]...)
		for _, value := range trailers[SessionsTrailer] {
			parsed.Sessions = appendMissing(parsed.Sessions, splitSessions(value))
		}
		paragraphs = paragraphs[:len(paragraphs)-1]
	}

	parsed.Body = strings.TrimSpace(strings.Join(paragraphs, "\n\n"))
	return parsed
}

func ReadChangeCommit(ctx context.Context, maxDiffLines int, message *CommitMessage, repoPath, sha string) (*ChangeCommit, error) {
	nameStatus, err := shell.Run(ctx, repoPath, "git", "show", "--name-status", "--format=", sha)
	if err != nil {
		return nil, fmt.Errorf("ReadChangeCommit: Commit %s: %s: %w", sha, strings.TrimSpace(nameStatus), err)
	}

	patch, err := shell.Run(ctx, repoPath, "git", "show", "--patch", "--format=", sha)
	if err != nil {
		return nil, fmt.Errorf("ReadChangeCommit: Commit %s: %s: %w", sha, strings.TrimSpace(patch), err)
	}

	numstat, err := shell.Run(ctx, repoPath, "git", "show", "--numstat", "--no-renames", "--format=", sha)
	if err != nil {
		return nil, fmt.Errorf("ReadChangeCommit: Commit %s: %s: %w", sha, strings.TrimSpace(numstat), err)
	}

	kind := ""
	if len(message.ChangeIds) > 0 {
		kind, _, _ = strings.Cut(message.ChangeIds[0], "/")
	}
	files := parseNameStatus(nameStatus)
	attachFileDiffs(files, strings.TrimRight(patch, "\n"), maxDiffLines)
	added, deleted := attachLineCounts(files, parseNumstat(numstat))
	commit := &ChangeCommit{
		Added:     added,
		Body:      message.Body,
		ChangeIds: message.ChangeIds,
		Deleted:   deleted,
		Files:     files,
		Kind:      kind,
		Sessions:  message.Sessions,
		Sha:       sha,
		Subject:   message.Subject,
	}
	return commit, nil
}

// ReadChangeStat measures one commit as the proposal cap does, from git's numstat.
func ReadChangeStat(ctx context.Context, repoPath, sha string) (*ChangeStat, error) {
	numstat, err := shell.Run(ctx, repoPath, "git", "show", "--numstat", "--no-renames", "--format=", sha)
	if err != nil {
		return nil, fmt.Errorf("ReadChangeStat: Commit %s: %s: %w", sha, strings.TrimSpace(numstat), err)
	}

	stat := &ChangeStat{}
	for _, count := range parseNumstat(numstat) {
		stat.Files++
		stat.Lines += count.Added + count.Deleted
	}
	return stat, nil
}

func appendMissing(list, values []string) []string {
	for _, value := range values {
		if !slices.Contains(list, value) {
			list = append(list, value)
		}
	}
	return list
}

// attachFileDiffs gives every file its own capped section of the patch; a file without a section (binary) keeps an empty Diff.
func attachFileDiffs(files []ChangeFile, patch string, maxLines int) {
	sections := splitPatch(patch)
	for index := range files {
		section, ok := sections[files[index].Path]
		if !ok {
			continue
		}
		files[index].Diff, files[index].IsDiffCut = capLines(section, maxLines)
	}
}

// attachLineCounts gives every file its numstat counts and returns the commit's totals.
func attachLineCounts(files []ChangeFile, counts map[string]lineCount) (int, int) {
	added, deleted := 0, 0
	for index := range files {
		count := counts[files[index].Path]
		files[index].Added, files[index].Deleted = count.Added, count.Deleted
		added += count.Added
		deleted += count.Deleted
	}
	return added, deleted
}

// capLines keeps the first limit lines of text; isCut reports a cut.
func capLines(text string, limit int) (string, bool) {
	lines := strings.Split(text, "\n")
	if len(lines) <= limit {
		return text, false
	}
	return strings.Join(lines[:limit], "\n"), true
}

// diffHeaderPaths returns the b/ path first, then the a/ path when it differs (a rename); nil for any other line.
func diffHeaderPaths(line string) []string {
	match := diffHeader.FindStringSubmatch(line)
	if match == nil {
		return nil
	}
	return appendMissing([]string{match[2]}, []string{match[1]})
}

func parseNameStatus(output string) []ChangeFile {
	files := make([]ChangeFile, 0)
	for line := range strings.Lines(output) {
		fields := strings.Split(strings.TrimRight(line, "\n"), "\t")
		if len(fields) < nameStatusMinFields {
			continue
		}
		file := ChangeFile{Path: fields[len(fields)-1], Status: fields[0]}
		files = append(files, file)
	}
	return files
}

// parseNumstat keys each --numstat line's counts by its path; a binary file's "-" parses as 0.
func parseNumstat(output string) map[string]lineCount {
	counts := make(map[string]lineCount)
	for line := range strings.Lines(output) {
		fields := strings.Split(strings.TrimRight(line, "\n"), "\t")
		if len(fields) < numstatFields {
			continue
		}
		added, _ := strconv.Atoi(fields[0])
		deleted, _ := strconv.Atoi(fields[1])
		counts[fields[len(fields)-1]] = lineCount{Added: added, Deleted: deleted}
	}
	return counts
}

// pickSource is the sha of the last cherry-pick provenance line; empty when the commit was not picked.
func pickSource(message string) string {
	source := ""
	for line := range strings.Lines(message) {
		if match := cherryPickLine.FindStringSubmatch(strings.TrimSpace(line)); match != nil {
			source = match[1]
		}
	}
	return source
}

// parseTrailers keeps every value of a repeated key: a consolidated commit carries one Smine-Change per merged change.
func parseTrailers(paragraph string) (map[string][]string, bool) {
	trailers := make(map[string][]string)
	for line := range strings.Lines(paragraph) {
		line = strings.TrimSpace(line)
		if line == "" || cherryPickLine.MatchString(line) {
			continue
		}
		match := trailerLine.FindStringSubmatch(line)
		if match == nil {
			return nil, false
		}
		trailers[match[1]] = append(trailers[match[1]], strings.TrimSpace(match[2]))
	}
	return trailers, len(trailers) > 0
}

// splitPatch keys each file's section of a `git show --patch` text by the paths its header names.
func splitPatch(patch string) map[string]string {
	sections := make(map[string]string)
	var keys, current []string
	flush := func() {
		for _, key := range keys {
			sections[key] = strings.Join(current, "\n")
		}
	}
	for line := range strings.Lines(patch) {
		text := strings.TrimRight(line, "\n")
		if paths := diffHeaderPaths(text); paths != nil {
			flush()
			keys, current = paths, nil
		}
		current = append(current, text)
	}
	flush()
	return sections
}

// splitSessions splits the Smine-Sessions value ("<id>, <id>").
func splitSessions(value string) []string {
	sessions := make([]string, 0)
	for part := range strings.SplitSeq(value, ",") {
		if id := strings.TrimSpace(part); id != "" {
			sessions = append(sessions, id)
		}
	}
	return sessions
}

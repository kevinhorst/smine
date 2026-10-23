package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/kevinhorst/smine/internal/shell"
	"github.com/kevinhorst/smine/internal/skillgraph"
)

const (
	mergeSkillcueArgs     = 6
	mergeSkillcueConflict = 1
	mergeSkillcueTimeout  = time.Minute
	mergeSkillcueUsage    = "usage: rules merge-skillcue <base> <ours> <theirs> <path> <ours label> <theirs label>"
)

// mergeSide is one side of a merge: git's conflict label for it and the name the log line prints.
type mergeSide struct {
	label string
	name  string
}

// mergeFiles is the three files git hands a merge driver plus the path it reports.
type mergeFiles struct {
	base   string
	ours   string
	path   string
	theirs string
}

func runMergeSkillcue(args []string) int {
	// git hands a driver's stderr to the user, so the shell helper's per-call timing lines stay out of it.
	log.SetOutput(io.Discard)
	if len(args) != mergeSkillcueArgs {
		fmt.Fprintln(os.Stderr, mergeSkillcueUsage)
		return exitError
	}

	ctx, cancel := context.WithTimeout(context.Background(), mergeSkillcueTimeout)
	defer cancel()
	files := mergeFiles{base: args[0], ours: args[1], path: args[3], theirs: args[2]}
	ours := mergeSide{label: args[4], name: "ours"}
	theirs := mergeSide{label: args[5], name: "theirs"}
	merged, err := mergeSkillcue(ctx, files, ours, theirs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "merge-skillcue: %s: %v — left to git\n", files.path, err)
		_, _ = shell.Run(ctx, "", "git", "merge-file", files.ours, files.base, files.theirs)
		return mergeSkillcueConflict
	}

	if err := os.WriteFile(files.ours, merged, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "merge-skillcue: %s: %v\n", files.path, err)
		return exitError
	}
	return exitClean
}

// mergeSkillcue merges the changelog through the model, the rest as text; any error means git's own merge.
func mergeSkillcue(ctx context.Context, files mergeFiles, ours, theirs mergeSide) ([]byte, error) {
	base, err := os.ReadFile(files.base)
	if err != nil {
		return nil, fmt.Errorf("mergeSkillcue: %w", err)
	}

	oursData, err := os.ReadFile(files.ours)
	if err != nil {
		return nil, fmt.Errorf("mergeSkillcue: %w", err)
	}

	theirsData, err := os.ReadFile(files.theirs)
	if err != nil {
		return nil, fmt.Errorf("mergeSkillcue: %w", err)
	}

	merge, err := skillgraph.ParseMerge(base, oursData, theirsData)
	if err != nil {
		return nil, fmt.Errorf("mergeSkillcue: %w", err)
	}

	isOursFresher, basis := fresherSide(ctx, ours, theirs, merge.OursNewest, merge.TheirsNewest)
	fresher := theirs.name
	if isOursFresher {
		fresher = ours.name
	}
	fmt.Fprintf(os.Stderr, "merge-skillcue: %s: fresher %s (%s)\n", files.path, fresher, basis)

	merged, err := merge.Resolve(isOursFresher)
	if err != nil {
		return nil, fmt.Errorf("mergeSkillcue: %w", err)
	}

	remainder, err := mergeRemainder(ctx, merged)
	if err != nil {
		return nil, fmt.Errorf("mergeSkillcue: %w", err)
	}

	applied, err := skillgraph.ApplyChangelog(remainder, merged.Rows, merged.Version)
	if err != nil {
		return nil, fmt.Errorf("mergeSkillcue: %w", err)
	}
	return applied, nil
}

// fresherSide answers whether ours is fresher, by commit time or else newest row date, a tie to theirs, and the basis.
func fresherSide(ctx context.Context, ours, theirs mergeSide, oursNewest, theirsNewest string) (bool, string) {
	oursTime, isOursResolved := commitTime(ctx, ours.label)
	theirsTime, isTheirsResolved := commitTime(ctx, theirs.label)
	if isOursResolved && isTheirsResolved {
		return oursTime > theirsTime, "commit time"
	}
	return oursNewest > theirsNewest, "row date"
}

// commitTime resolves a conflict label's first token to a commit and reads its committer time.
func commitTime(ctx context.Context, label string) (int64, bool) {
	token, _, _ := strings.Cut(strings.TrimSpace(label), " ")
	if token == "" {
		return 0, false
	}

	sha, err := shell.Run(ctx, "", "git", "rev-parse", "--verify", "--quiet", "--end-of-options", token+"^{commit}")
	if err != nil {
		return 0, false
	}

	seconds, err := shell.Run(ctx, "", "git", "show", "-s", "--format=%ct", strings.TrimSpace(sha))
	if err != nil {
		return 0, false
	}

	parsed, err := strconv.ParseInt(strings.TrimSpace(seconds), 10, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

// mergeRemainder merges the stripped sides as text: the other side when one equals the base, else git merge-file.
func mergeRemainder(ctx context.Context, merged *skillgraph.Merged) ([]byte, error) {
	if bytes.Equal(merged.Ours, merged.Base) {
		return merged.Theirs, nil
	}

	if bytes.Equal(merged.Theirs, merged.Base) {
		return merged.Ours, nil
	}

	dir, err := os.MkdirTemp("", "merge-skillcue-")
	if err != nil {
		return nil, fmt.Errorf("mergeRemainder: %w", err)
	}
	defer os.RemoveAll(dir)

	paths := map[string][]byte{"base": merged.Base, "ours": merged.Ours, "theirs": merged.Theirs}
	for name, data := range paths {
		if err := os.WriteFile(dir+"/"+name, data, 0o644); err != nil {
			return nil, fmt.Errorf("mergeRemainder: %w", err)
		}
	}

	command := exec.CommandContext(ctx, "git", "merge-file", "-p", dir+"/ours", dir+"/base", dir+"/theirs")
	output, err := command.Output()
	exitErr, isExitErr := errors.AsType[*exec.ExitError](err)
	if isExitErr {
		return nil, fmt.Errorf("mergeRemainder: Remainder conflicts in %d hunk(s)", exitErr.ExitCode())
	}

	if err != nil {
		return nil, fmt.Errorf("mergeRemainder: %w", err)
	}
	return output, nil
}

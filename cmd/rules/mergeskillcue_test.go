package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	skillcueBaseSeconds    = 1700000000
	skillcueFallbackMarker = "— left to git"
	skillcuePath           = "done.cue"
)

var skillcueFresherPattern = regexp.MustCompile(`fresher (ours|theirs) \((commit time|row date)\)`)

// skillcueOutcome is what a driver run left in %A: conflict markers, or the decoded preamble, added rows and version.
type skillcueOutcome struct {
	hasMarkers bool
	preamble   string
	rows       []string
	version    string
}

// skillcueRepo is a fixture repo with a base commit on main, one commit on each side branch, and the three sides as
// driver input files outside the repo.
type skillcueRepo struct {
	basePath     string
	dir          string
	oursCommit   string
	oursPath     string
	theirsCommit string
	theirsPath   string
}

// driverArgs is the driver's argument list in git's %O %A %B %P %X %Y order.
func (r *skillcueRepo) driverArgs(oursLabel, theirsLabel string) []string {
	return []string{r.basePath, r.oursPath, r.theirsPath, skillcuePath, oursLabel, theirsLabel}
}

// captureRunMergeSkillcue runs merge-skillcue in repo and returns its exit code, its stderr and the logger's output.
func captureRunMergeSkillcue(t *testing.T, repo string, args []string) (int, string, string) {
	t.Helper()
	t.Chdir(repo)
	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	logged := &bytes.Buffer{}
	log.SetOutput(logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	stderr := os.Stderr
	os.Stderr = writer
	code := runMergeSkillcue(args)
	os.Stderr = stderr
	require.NoError(t, writer.Close())

	printed, err := io.ReadAll(reader)
	require.NoError(t, err)
	return code, string(printed), logged.String()
}

// commitSkillcue writes content to the model file in repo and commits it with author and committer time fixed at seconds.
func commitSkillcue(t *testing.T, repo string, content []byte, message string, seconds int64) string {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(repo, skillcuePath), content, 0o644))
	gitOutput(t, repo, "add", skillcuePath)

	date := fmt.Sprintf("@%d +0000", seconds)
	command := exec.CommandContext(t.Context(), "git", "commit", "-q", "-m", message)
	command.Dir = repo
	command.Env = append(
		os.Environ(),
		"GIT_AUTHOR_NAME=t",
		"GIT_AUTHOR_EMAIL=t@t",
		"GIT_AUTHOR_DATE="+date,
		"GIT_COMMITTER_NAME=t",
		"GIT_COMMITTER_EMAIL=t@t",
		"GIT_COMMITTER_DATE="+date,
	)
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
	return gitOutput(t, repo, "rev-parse", "HEAD")
}

// encodeSkillcue writes a skill through the production encoder.
func encodeSkillcue(t *testing.T, skill *skillgraph.Skill) []byte {
	t.Helper()
	encoded, err := skillgraph.EncodeSkill(skill)
	require.NoError(t, err)
	return encoded
}

// provideSkillcueBase is the done skill read through the production decoder with one fixture row on top, so the
// expected versions do not depend on done's own history.
func provideSkillcueBase(t *testing.T) *skillgraph.Skill {
	t.Helper()
	data, err := os.ReadFile(skillgraph.ModelPath(filepath.Join("..", "..", "skills", "orchestration", "done")))
	require.NoError(t, err)

	done, err := skillgraph.DecodeSkill(data, skillcuePath)
	require.NoError(t, err)

	row := skillgraph.ChangelogEntry{Date: "2026-10-09", Text: "fixture base", Version: "9.0.0"}
	return provideSkillcueSide(done, done.Preamble, row)
}

// provideSkillcueRepo builds the fixture repo: base on main, ours and theirs each one commit on their own branch.
func provideSkillcueRepo(t *testing.T, base, ours []byte, oursSeconds int64, theirs []byte, theirsSeconds int64) *skillcueRepo {
	t.Helper()
	dir := t.TempDir()
	gitOutput(t, dir, "init", "-q", "-b", "main")
	commitSkillcue(t, dir, base, "base", skillcueBaseSeconds)

	gitOutput(t, dir, "checkout", "-q", "-b", "ours")
	oursCommit := commitSkillcue(t, dir, ours, "ours side", oursSeconds)

	gitOutput(t, dir, "checkout", "-q", "-b", "theirs", "main")
	theirsCommit := commitSkillcue(t, dir, theirs, "theirs side", theirsSeconds)

	sides := t.TempDir()
	repo := &skillcueRepo{
		basePath:     filepath.Join(sides, "base"),
		dir:          dir,
		oursCommit:   oursCommit,
		oursPath:     filepath.Join(sides, "ours"),
		theirsCommit: theirsCommit,
		theirsPath:   filepath.Join(sides, "theirs"),
	}
	require.NoError(t, os.WriteFile(repo.basePath, base, 0o644))
	require.NoError(t, os.WriteFile(repo.oursPath, ours, 0o644))
	require.NoError(t, os.WriteFile(repo.theirsPath, theirs, 0o644))
	return repo
}

// provideSkillcueSide is skill with row prepended, the frontmatter version following it, and preamble in place.
func provideSkillcueSide(skill *skillgraph.Skill, preamble string, row skillgraph.ChangelogEntry) *skillgraph.Skill {
	side := *skill
	added := []skillgraph.ChangelogEntry{row}
	side.Changelog = slices.Concat(added, skill.Changelog)
	side.Frontmatter = slices.Clone(skill.Frontmatter)
	for index, field := range side.Frontmatter {
		if field.Key == "version" {
			side.Frontmatter[index].Value = " " + row.Version
		}
	}
	side.Preamble = preamble
	return &side
}

// readSkillcueOutcome reads what the driver left at path; the rows are the ones above the base's baseCount rows.
func readSkillcueOutcome(t *testing.T, path string, baseCount int) *skillcueOutcome {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	outcome := &skillcueOutcome{hasMarkers: bytes.Contains(data, []byte("<<<<<<<"))}
	skill, err := skillgraph.DecodeSkill(data, skillcuePath)
	if err != nil {
		return outcome
	}

	rows := make([]string, 0)
	for _, row := range skill.Changelog[:len(skill.Changelog)-baseCount] {
		rows = append(rows, row.Version+": "+row.Text)
	}
	outcome.preamble = skill.Preamble
	outcome.rows = rows
	outcome.version = skill.Version()
	return outcome
}

func TestRunMergeSkillcue(t *testing.T) {
	type testCase struct {
		_id              string
		_expected        int
		_expectedBasis   string
		_expectedOutcome *skillcueOutcome
		_shouldFallBack  bool

		args []string
		repo string
	}

	base := provideSkillcueBase(t)
	baseBytes := encodeSkillcue(t, base)
	baseCount := len(base.Changelog)
	oursPatch := skillgraph.ChangelogEntry{Date: "2026-10-10", Text: "ours patch", Version: "9.0.1"}
	theirsMinor := skillgraph.ChangelogEntry{Date: "2026-10-10", Text: "theirs minor", Version: "9.1.0"}
	oursBytes := encodeSkillcue(t, provideSkillcueSide(base, base.Preamble, oursPatch))
	theirsBytes := encodeSkillcue(t, provideSkillcueSide(base, base.Preamble, theirsMinor))
	oursOnTop := &skillcueOutcome{
		hasMarkers: false,
		preamble:   base.Preamble,
		rows:       []string{"9.1.1: ours patch", "9.1.0: theirs minor"},
		version:    "9.1.1",
	}
	theirsOnTop := &skillcueOutcome{
		hasMarkers: false,
		preamble:   base.Preamble,
		rows:       []string{"9.1.0: theirs minor", "9.0.1: ours patch"},
		version:    "9.1.0",
	}
	conflicted := &skillcueOutcome{hasMarkers: true, preamble: "", rows: nil, version: ""}

	tests := make([]*testCase, 0)

	// ours-fresher-by-commit-time
	oursFresher := provideSkillcueRepo(t, baseBytes, oursBytes, skillcueBaseSeconds+200, theirsBytes, skillcueBaseSeconds+100)
	tests = append(tests, &testCase{
		_id:              "ours-fresher-by-commit-time",
		_expected:        exitClean,
		_expectedBasis:   "fresher ours (commit time)",
		_expectedOutcome: oursOnTop,
		_shouldFallBack:  false,

		args: oursFresher.driverArgs(oursFresher.oursCommit, oursFresher.theirsCommit),
		repo: oursFresher.dir,
	})

	// theirs-fresher-by-commit-time
	theirsFresher := provideSkillcueRepo(t, baseBytes, oursBytes, skillcueBaseSeconds+100, theirsBytes, skillcueBaseSeconds+200)
	tests = append(tests, &testCase{
		_id:              "theirs-fresher-by-commit-time",
		_expected:        exitClean,
		_expectedBasis:   "fresher theirs (commit time)",
		_expectedOutcome: theirsOnTop,
		_shouldFallBack:  false,

		args: theirsFresher.driverArgs(theirsFresher.oursCommit, theirsFresher.theirsCommit),
		repo: theirsFresher.dir,
	})

	// tie-goes-to-theirs
	tie := provideSkillcueRepo(t, baseBytes, oursBytes, skillcueBaseSeconds+100, theirsBytes, skillcueBaseSeconds+100)
	tests = append(tests, &testCase{
		_id:              "tie-goes-to-theirs",
		_expected:        exitClean,
		_expectedBasis:   "fresher theirs (commit time)",
		_expectedOutcome: theirsOnTop,
		_shouldFallBack:  false,

		args: tie.driverArgs(tie.oursCommit, tie.theirsCommit),
		repo: tie.dir,
	})

	// labels-unresolvable-row-date
	laterPatch := skillgraph.ChangelogEntry{Date: "2026-10-11", Text: "ours patch", Version: "9.0.1"}
	laterBytes := encodeSkillcue(t, provideSkillcueSide(base, base.Preamble, laterPatch))
	unresolvable := provideSkillcueRepo(t, baseBytes, laterBytes, skillcueBaseSeconds+100, theirsBytes, skillcueBaseSeconds+200)
	tests = append(tests, &testCase{
		_id:              "labels-unresolvable-row-date",
		_expected:        exitClean,
		_expectedBasis:   "fresher ours (row date)",
		_expectedOutcome: oursOnTop,
		_shouldFallBack:  false,

		args: unresolvable.driverArgs("HEAD", "nowhere (gone)"),
		repo: unresolvable.dir,
	})

	// shared-row-once
	sharedPatch := skillgraph.ChangelogEntry{Date: "2026-10-10", Text: "shared patch", Version: "9.0.1"}
	sharedBytes := encodeSkillcue(t, provideSkillcueSide(base, base.Preamble, sharedPatch))
	shared := provideSkillcueRepo(t, baseBytes, sharedBytes, skillcueBaseSeconds+100, sharedBytes, skillcueBaseSeconds+200)
	sharedOnce := &skillcueOutcome{
		hasMarkers: false,
		preamble:   base.Preamble,
		rows:       []string{"9.0.1: shared patch"},
		version:    "9.0.1",
	}
	tests = append(tests, &testCase{
		_id:              "shared-row-once",
		_expected:        exitClean,
		_expectedBasis:   "fresher theirs (commit time)",
		_expectedOutcome: sharedOnce,
		_shouldFallBack:  false,

		args: shared.driverArgs(shared.oursCommit, shared.theirsCommit),
		repo: shared.dir,
	})

	// remainder-clean
	theirsPreamble := strings.Replace(base.Preamble, "# Done", "# Done theirs", 1)
	editedBytes := encodeSkillcue(t, provideSkillcueSide(base, theirsPreamble, theirsMinor))
	clean := provideSkillcueRepo(t, baseBytes, oursBytes, skillcueBaseSeconds+100, editedBytes, skillcueBaseSeconds+200)
	preambleMerged := &skillcueOutcome{
		hasMarkers: false,
		preamble:   theirsPreamble,
		rows:       []string{"9.1.0: theirs minor", "9.0.1: ours patch"},
		version:    "9.1.0",
	}
	tests = append(tests, &testCase{
		_id:              "remainder-clean",
		_expected:        exitClean,
		_expectedBasis:   "fresher theirs (commit time)",
		_expectedOutcome: preambleMerged,
		_shouldFallBack:  false,

		args: clean.driverArgs(clean.oursCommit, clean.theirsCommit),
		repo: clean.dir,
	})

	// remainder-conflict-falls-back
	oursPreamble := strings.Replace(base.Preamble, "# Done", "# Done ours", 1)
	oursEditedBytes := encodeSkillcue(t, provideSkillcueSide(base, oursPreamble, oursPatch))
	conflict := provideSkillcueRepo(t, baseBytes, oursEditedBytes, skillcueBaseSeconds+100, editedBytes, skillcueBaseSeconds+200)
	tests = append(tests, &testCase{
		_id:              "remainder-conflict-falls-back",
		_expected:        mergeSkillcueConflict,
		_expectedBasis:   "fresher theirs (commit time)",
		_expectedOutcome: conflicted,
		_shouldFallBack:  true,

		args: conflict.driverArgs(conflict.oursCommit, conflict.theirsCommit),
		repo: conflict.dir,
	})

	// undecodable-side-falls-back
	undecodable := provideSkillcueRepo(t, baseBytes, []byte("this is not cue\n"), skillcueBaseSeconds+100, theirsBytes, skillcueBaseSeconds+200)
	tests = append(tests, &testCase{
		_id:              "undecodable-side-falls-back",
		_expected:        mergeSkillcueConflict,
		_expectedBasis:   "",
		_expectedOutcome: conflicted,
		_shouldFallBack:  true,

		args: undecodable.driverArgs(undecodable.oursCommit, undecodable.theirsCommit),
		repo: undecodable.dir,
	})

	// usage-five-args
	usage := provideSkillcueRepo(t, baseBytes, oursBytes, skillcueBaseSeconds+100, theirsBytes, skillcueBaseSeconds+200)
	untouched := &skillcueOutcome{
		hasMarkers: false,
		preamble:   base.Preamble,
		rows:       []string{"9.0.1: ours patch"},
		version:    "9.0.1",
	}
	tests = append(tests, &testCase{
		_id:              "usage-five-args",
		_expected:        exitError,
		_expectedBasis:   "",
		_expectedOutcome: untouched,
		_shouldFallBack:  false,

		args: usage.driverArgs(usage.oursCommit, usage.theirsCommit)[:5],
		repo: usage.dir,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			code, stderr, logged := captureRunMergeSkillcue(t, test.repo, test.args)
			assert.Equal(t, test._expected, code)
			assert.Empty(t, logged)
			assert.Equal(t, test._expectedBasis, skillcueFresherPattern.FindString(stderr))
			assert.Equal(t, test._shouldFallBack, strings.Contains(stderr, skillcueFallbackMarker))
			assert.Equal(t, test._expectedOutcome, readSkillcueOutcome(t, test.args[1], baseCount))
		})
	}
}

func TestCommitTime(t *testing.T) {
	type testCase struct {
		_id            string
		_expected      int64
		_shouldResolve bool

		label string
	}

	repo := t.TempDir()
	gitOutput(t, repo, "init", "-q", "-b", "main")
	commit := commitSkillcue(t, repo, []byte("fixture\n"), "fixture", skillcueBaseSeconds)
	t.Chdir(repo)

	tests := make([]*testCase, 0)

	// full-sha
	tests = append(tests, &testCase{
		_id:            "full-sha",
		_expected:      skillcueBaseSeconds,
		_shouldResolve: true,

		label: commit,
	})

	// abbreviated-sha-with-subject
	tests = append(tests, &testCase{
		_id:            "abbreviated-sha-with-subject",
		_expected:      skillcueBaseSeconds,
		_shouldResolve: true,

		label: commit[:7] + " (fixture)",
	})

	// head
	tests = append(tests, &testCase{
		_id:            "head",
		_expected:      skillcueBaseSeconds,
		_shouldResolve: true,

		label: "HEAD",
	})

	// unknown-ref
	tests = append(tests, &testCase{
		_id:            "unknown-ref",
		_expected:      0,
		_shouldResolve: false,

		label: "nowhere (gone)",
	})

	// empty-label
	tests = append(tests, &testCase{
		_id:            "empty-label",
		_expected:      0,
		_shouldResolve: false,

		label: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			seconds, isResolved := commitTime(t.Context(), test.label)
			assert.Equal(t, test._expected, seconds)
			assert.Equal(t, test._shouldResolve, isResolved)
		})
	}
}

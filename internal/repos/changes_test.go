package repos

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initNightRepo builds a repo with two nights' branch pairs, a landing scratch branch and one change commit on the
// newest proposals branch; it returns the repo plus the commit sha.
func initNightRepo(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t"}, args...)...)
		cmd.Dir = repo
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, string(output))
		return strings.TrimSpace(string(output))
	}
	run("init", "-q", "-b", "main")
	require.NoError(t, os.WriteFile(filepath.Join(repo, "gone.md"), []byte("gone\n"), 0o644))
	run("add", "gone.md")
	run("commit", "-q", "-m", "base")
	run("branch", NightBranchPrefix+"2026-10-03"+BatchSuffix)
	run("branch", NightBranchPrefix+"2026-10-04"+BatchSuffix)
	run("branch", NightBranchPrefix+"2026-10-04-landing")
	run("switch", "-q", "-c", NightBranchPrefix+"2026-10-03"+ProposalsSuffix)
	run("switch", "-q", "-c", NightBranchPrefix+"2026-10-04"+ProposalsSuffix)
	lines := make([]string, 0, 10)
	for index := range 10 {
		lines = append(lines, "line "+strings.Repeat("x", index))
	}
	require.NoError(t, os.WriteFile(filepath.Join(repo, "rule.md"), []byte(strings.Join(lines, "\n")+"\n"), 0o644))
	run("rm", "-q", "gone.md")
	run("add", "rule.md")
	run("commit", "-q", "-m", "context.rules: Added a rule\n\nThe finding.\n\nSmine-Change: context/rule\nSmine-Sessions: s1, s2")
	run("switch", "-q", "main")
	run("branch", "claude-routines/other-2026-10-04")
	return repo, run("rev-parse", NightBranchPrefix+"2026-10-04"+ProposalsSuffix)
}

func TestBranchMessages(t *testing.T) {
	repo := initSettleRepo(t, settleNight04)
	first := commitOnBranch(t, repo, settleNight04, "skills.a: Fixed a\n\nThe finding.\n\nSmine-Change: skills/a\nSmine-Sessions: s1")
	second := commitOnBranch(t, repo, settleNight04, "sessions: Recorded output")

	messages, err := BranchMessages(context.Background(), settleNight04, repo, "main")
	require.NoError(t, err)

	expected := map[string]*CommitMessage{
		first:  {Body: "The finding.", ChangeIds: []string{"skills/a"}, Sessions: []string{"s1"}, Subject: "skills.a: Fixed a"},
		second: {Body: "", ChangeIds: []string{}, Sessions: []string{}, Subject: "sessions: Recorded output"},
	}
	assert.Equal(t, expected, messages)
}

func TestPickedChanges(t *testing.T) {
	repo := initSettleRepo(t)
	source := strings.Repeat("a", 40)
	picked := commitOnBranch(t, repo, "main", "skills.x: Fixed x\n\nSmine-Change: skills/x\n(cherry picked from commit "+source+")")
	commitOnBranch(t, repo, "main", "skills.y: Hand-made change\n\nSmine-Change: skills/y")
	commitOnBranch(t, repo, "main", "sessions: Recorded batch\n\n(cherry picked from commit "+strings.Repeat("b", 40)+")")
	newest := commitOnBranch(t, repo, "main", "context.facts: Added z\n\nSmine-Change: context/z\nSmine-Change: context/w\n(cherry picked from commit "+strings.Repeat("c", 40)+")")
	date := gitIn(t, repo, "show", "--no-patch", "--format=%cs", newest)

	changes, err := PickedChanges(context.Background(), 10, repo, "main")
	require.NoError(t, err)

	require.Len(t, changes, 2)
	assert.Equal(t, newest, changes[0].Sha)
	assert.Equal(t, date, changes[0].Date)
	assert.Equal(t, strings.Repeat("c", 40), changes[0].SourceSha)
	assert.Equal(t, []string{"context/z", "context/w"}, changes[0].Message.ChangeIds)
	assert.Equal(t, picked, changes[1].Sha)
	assert.Equal(t, source, changes[1].SourceSha)
	assert.Equal(t, "skills.x: Fixed x", changes[1].Message.Subject)
}

func TestPickedChangesLimit(t *testing.T) {
	repo := initSettleRepo(t)
	commitOnBranch(t, repo, "main", "skills.x: Fixed x\n\nSmine-Change: skills/x\n(cherry picked from commit "+strings.Repeat("a", 40)+")")
	newest := commitOnBranch(t, repo, "main", "skills.y: Fixed y\n\nSmine-Change: skills/y\n(cherry picked from commit "+strings.Repeat("b", 40)+")")

	changes, err := PickedChanges(context.Background(), 1, repo, "main")
	require.NoError(t, err)

	require.Len(t, changes, 1)
	assert.Equal(t, newest, changes[0].Sha)
}

func TestListNightBranches(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		suffix string
	}

	repo, _ := initNightRepo(t)
	tests := make([]*testCase, 0)

	// proposals-newest-first
	tests = append(tests, &testCase{
		_id:       "proposals-newest-first",
		_expected: []string{NightBranchPrefix + "2026-10-04" + ProposalsSuffix, NightBranchPrefix + "2026-10-03" + ProposalsSuffix},

		suffix: ProposalsSuffix,
	})

	// batch-newest-first
	tests = append(tests, &testCase{
		_id:       "batch-newest-first",
		_expected: []string{NightBranchPrefix + "2026-10-04" + BatchSuffix, NightBranchPrefix + "2026-10-03" + BatchSuffix},

		suffix: BatchSuffix,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			branches, err := ListNightBranches(context.Background(), repo, test.suffix)
			require.NoError(t, err)
			assert.Equal(t, test._expected, branches)
		})
	}
}

// fileSummaries renders each file as "<status> <path> <diff lines> <cut> +<added> −<deleted>" so a table asserts the per-file split in one value.
func fileSummaries(files []ChangeFile) []string {
	summaries := make([]string, 0, len(files))
	for _, file := range files {
		summary := fmt.Sprintf(
			"%s %s %d %t +%d −%d",
			file.Status,
			file.Path,
			len(strings.Split(file.Diff, "\n")),
			file.IsDiffCut,
			file.Added,
			file.Deleted,
		)
		summaries = append(summaries, summary)
	}
	return summaries
}

// sectionLineCounts maps each splitPatch key to its section's line count.
func sectionLineCounts(sections map[string]string) map[string]int {
	counts := make(map[string]int, len(sections))
	for key, section := range sections {
		counts[key] = len(strings.Split(section, "\n"))
	}
	return counts
}

func TestReadChangeCommit(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		maxDiffLines int
	}

	repo, sha := initNightRepo(t)
	messages, err := BranchMessages(context.Background(), NightBranchPrefix+"2026-10-04"+ProposalsSuffix, repo, "main")
	require.NoError(t, err)
	message := messages[sha]
	require.NotNil(t, message)
	tests := make([]*testCase, 0)

	// full-diff
	tests = append(tests, &testCase{
		_id:       "full-diff",
		_expected: []string{"D gone.md 7 false +0 −1", "A rule.md 16 false +10 −0"},

		maxDiffLines: 400,
	})

	// diff-truncated-per-file
	tests = append(tests, &testCase{
		_id:       "diff-truncated-per-file",
		_expected: []string{"D gone.md 5 true +0 −1", "A rule.md 5 true +10 −0"},

		maxDiffLines: 5,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			commit, err := ReadChangeCommit(context.Background(), test.maxDiffLines, message, repo, sha)
			require.NoError(t, err)
			assert.Equal(t, "context.rules: Added a rule", commit.Subject)
			assert.Equal(t, "The finding.", commit.Body)
			assert.Equal(t, []string{"context/rule"}, commit.ChangeIds)
			assert.Equal(t, "context", commit.Kind)
			assert.Equal(t, []string{"s1", "s2"}, commit.Sessions)
			assert.Equal(t, test._expected, fileSummaries(commit.Files))
			assert.Equal(t, 10, commit.Added)
			assert.Equal(t, 1, commit.Deleted)
		})
	}
}

func TestParseNumstat(t *testing.T) {
	type testCase struct {
		_id       string
		_expected map[string]lineCount

		output string
	}

	tests := make([]*testCase, 0)

	// two-files
	tests = append(tests, &testCase{
		_id:       "two-files",
		_expected: map[string]lineCount{"gone.md": {Added: 0, Deleted: 1}, "rule.md": {Added: 10, Deleted: 0}},

		output: "0\t1\tgone.md\n10\t0\trule.md\n",
	})

	// binary-dash-counts-zero
	tests = append(tests, &testCase{
		_id:       "binary-dash-counts-zero",
		_expected: map[string]lineCount{"x.png": {Added: 0, Deleted: 0}},

		output: "-\t-\tx.png\n",
	})

	// short-line-skipped
	tests = append(tests, &testCase{
		_id:       "short-line-skipped",
		_expected: map[string]lineCount{"a.md": {Added: 2, Deleted: 3}},

		output: "2\t3\ta.md\n5\tb.md\n\n",
	})

	// empty
	tests = append(tests, &testCase{
		_id:       "empty",
		_expected: map[string]lineCount{},

		output: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, parseNumstat(test.output))
		})
	}
}

func TestReadChangeStat(t *testing.T) {
	repo, sha := initNightRepo(t)

	stat, err := ReadChangeStat(context.Background(), repo, sha)
	require.NoError(t, err)

	assert.Equal(t, &ChangeStat{Files: 2, Lines: 11}, stat)
	assert.False(t, stat.IsOverCap())
}

func TestChangeStat_IsOverCap(t *testing.T) {
	type testCase struct {
		_id       string
		_expected bool

		stat *ChangeStat
	}

	tests := make([]*testCase, 0)

	// under-both
	tests = append(tests, &testCase{
		_id:       "under-both",
		_expected: false,

		stat: &ChangeStat{Files: 3, Lines: 100},
	})

	// files-at-cap
	tests = append(tests, &testCase{
		_id:       "files-at-cap",
		_expected: false,

		stat: &ChangeStat{Files: 7, Lines: 420},
	})

	// files-over
	tests = append(tests, &testCase{
		_id:       "files-over",
		_expected: true,

		stat: &ChangeStat{Files: 8, Lines: 8},
	})

	// lines-over
	tests = append(tests, &testCase{
		_id:       "lines-over",
		_expected: true,

		stat: &ChangeStat{Files: 1, Lines: 421},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.stat.IsOverCap())
		})
	}
}

func TestSplitPatch(t *testing.T) {
	type testCase struct {
		_id       string
		_expected map[string]int

		patch string
	}

	deletion := "diff --git a/gone.md b/gone.md\ndeleted file mode 100644\nindex 1111111..0000000\n--- a/gone.md\n+++ /dev/null\n@@ -1 +0,0 @@\n-gone"
	addedLines := make([]string, 0, 10)
	for index := range 10 {
		addedLines = append(addedLines, "+line "+strings.Repeat("x", index))
	}
	addition := "diff --git a/rule.md b/rule.md\nnew file mode 100644\nindex 0000000..2222222\n--- /dev/null\n+++ b/rule.md\n@@ -0,0 +1,10 @@\n" + strings.Join(addedLines, "\n")
	rename := "diff --git a/old.md b/new.md\nsimilarity index 90%\nrename from old.md\nrename to new.md\n--- a/old.md\n+++ b/new.md\n@@ -1 +1 @@\n-a\n+b"
	tests := make([]*testCase, 0)

	// two-files
	tests = append(tests, &testCase{
		_id:       "two-files",
		_expected: map[string]int{"gone.md": 7, "rule.md": 16},

		patch: deletion + "\n" + addition,
	})

	// rename-keys-both-paths
	tests = append(tests, &testCase{
		_id:       "rename-keys-both-paths",
		_expected: map[string]int{"new.md": 9, "old.md": 9},

		patch: rename,
	})

	// empty-patch
	tests = append(tests, &testCase{
		_id:       "empty-patch",
		_expected: map[string]int{},

		patch: "",
	})

	// text-without-header
	tests = append(tests, &testCase{
		_id:       "text-without-header",
		_expected: map[string]int{},

		patch: "Binary files a/x.png and b/x.png differ",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, sectionLineCounts(splitPatch(test.patch)))
		})
	}
}

func TestParseCommitMessage(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *CommitMessage

		message string
	}

	tests := make([]*testCase, 0)

	// change-commit
	tests = append(tests, &testCase{
		_id:       "change-commit",
		_expected: &CommitMessage{Body: "First paragraph.\n\nSecond paragraph.", ChangeIds: []string{"skills/fix-x"}, Sessions: []string{"a", "b"}, Subject: "skills.x: Fixed x"},

		message: "skills.x: Fixed x\n\nFirst paragraph.\n\nSecond paragraph.\n\nSmine-Change: skills/fix-x\nSmine-Sessions: a, b\n",
	})

	// two-change-trailers
	tests = append(tests, &testCase{
		_id:       "two-change-trailers",
		_expected: &CommitMessage{Body: "Merged.", ChangeIds: []string{"skills/b", "skills/a"}, Sessions: []string{"a"}, Subject: "skills.x: Merged two changes"},

		message: "skills.x: Merged two changes\n\nMerged.\n\nSmine-Change: skills/b\nSmine-Change: skills/a\nSmine-Sessions: a\n",
	})

	// two-sessions-trailers-union
	tests = append(tests, &testCase{
		_id:       "two-sessions-trailers-union",
		_expected: &CommitMessage{Body: "Merged.", ChangeIds: []string{"skills/a", "skills/b"}, Sessions: []string{"a", "b", "c"}, Subject: "skills.x: Merged two changes"},

		message: "skills.x: Merged two changes\n\nMerged.\n\nSmine-Change: skills/a\nSmine-Sessions: a, b\nSmine-Change: skills/b\nSmine-Sessions: b, c\n",
	})

	// no-trailers
	tests = append(tests, &testCase{
		_id:       "no-trailers",
		_expected: &CommitMessage{Body: "Just a body: with a colon later.", ChangeIds: []string{}, Sessions: []string{}, Subject: "sessions: Recorded output"},

		message: "sessions: Recorded output\n\nJust a body: with a colon later.\n",
	})

	// subject-only
	tests = append(tests, &testCase{
		_id:       "subject-only",
		_expected: &CommitMessage{ChangeIds: []string{}, Sessions: []string{}, Subject: "base"},

		message: "base\n",
	})

	// trailers-without-body
	tests = append(tests, &testCase{
		_id:       "trailers-without-body",
		_expected: &CommitMessage{ChangeIds: []string{"memory/m"}, Sessions: []string{}, Subject: "memory: Drained m"},

		message: "memory: Drained m\n\nSmine-Change: memory/m\n",
	})

	// trailer-block-without-change
	tests = append(tests, &testCase{
		_id:       "trailer-block-without-change",
		_expected: &CommitMessage{Body: "Records.", ChangeIds: []string{}, Sessions: []string{"a"}, Subject: "sessions: Recorded output"},

		message: "sessions: Recorded output\n\nRecords.\n\nSmine-Sessions: a\n",
	})

	// mixed-last-paragraph-is-body
	tests = append(tests, &testCase{
		_id:       "mixed-last-paragraph-is-body",
		_expected: &CommitMessage{Body: "Smine-Change: context/x\nnot a trailer line", ChangeIds: []string{}, Sessions: []string{}, Subject: "context: Edited"},

		message: "context: Edited\n\nSmine-Change: context/x\nnot a trailer line\n",
	})

	// trailer-line-in-earlier-paragraph
	tests = append(tests, &testCase{
		_id:       "trailer-line-in-earlier-paragraph",
		_expected: &CommitMessage{Body: "Smine-Change: context/x\n\nThe finding.", ChangeIds: []string{}, Sessions: []string{}, Subject: "context: Edited"},

		message: "context: Edited\n\nSmine-Change: context/x\n\nThe finding.\n",
	})

	// cherry-pick-provenance-in-trailers
	tests = append(tests, &testCase{
		_id:       "cherry-pick-provenance-in-trailers",
		_expected: &CommitMessage{Body: "The finding.", ChangeIds: []string{"context/x"}, Sessions: []string{"a", "b"}, Subject: "context: Edited"},

		message: "context: Edited\n\nThe finding.\n\nSmine-Change: context/x\nSmine-Sessions: a, b\n(cherry picked from commit f70aa7e0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6)\n",
	})

	// provenance-line-only
	tests = append(tests, &testCase{
		_id:       "provenance-line-only",
		_expected: &CommitMessage{Body: "The finding.\n\n(cherry picked from commit f70aa7e0)", ChangeIds: []string{}, Sessions: []string{}, Subject: "context: Edited"},

		message: "context: Edited\n\nThe finding.\n\n(cherry picked from commit f70aa7e0)\n",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, ParseCommitMessage(test.message))
		})
	}
}

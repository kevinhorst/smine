package gitwt

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/shell"
)

const (
	appendTwoPatch = "diff --git a/file.txt b/file.txt\n--- a/file.txt\n+++ b/file.txt\n@@ -1 +1,2 @@\n one\n+two\n"
	stalePatch     = "diff --git a/file.txt b/file.txt\n--- a/file.txt\n+++ b/file.txt\n@@ -1 +1,2 @@\n nope\n+two\n"
)

// addWorktree adds path to repo on a new branch; an empty branch detaches.
func addWorktree(t *testing.T, branch, path, repo string) {
	t.Helper()
	if branch == "" {
		gitOutput(t, repo, "worktree", "add", "-q", "--detach", path)
		return
	}

	gitOutput(t, repo, "worktree", "add", "-q", "-b", branch, path)
}

func commitAll(t *testing.T, dir, message string) {
	t.Helper()
	gitOutput(t, dir, "add", "-A")
	gitOutput(t, dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", message)
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	output, err := shell.Run(context.Background(), dir, "git", args...)
	require.NoError(t, err, output)
	return strings.TrimSpace(output)
}

// listFiles returns every regular file under root, root-relative and sorted;
// empty when root is absent.
func listFiles(t *testing.T, root string) []string {
	t.Helper()
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			return nil
		}

		relative, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("listFiles: %w", err)
		}

		files = append(files, relative)
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return files
	}

	require.NoError(t, err)
	sort.Strings(files)
	return files
}

// listLinks returns every symlink under root as "<root-relative> -> <target>",
// sorted; empty when root is absent.
func listLinks(t *testing.T, root string) []string {
	t.Helper()
	links := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.Type()&fs.ModeSymlink == 0 {
			return nil
		}

		relative, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("listLinks: %w", err)
		}

		target, err := os.Readlink(path)
		if err != nil {
			return fmt.Errorf("listLinks: %w", err)
		}

		links = append(links, relative+" -> "+target)
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return links
	}

	require.NoError(t, err)
	sort.Strings(links)
	return links
}

func localBranches(t *testing.T, repo string) []string {
	t.Helper()
	return strings.Fields(gitOutput(t, repo, "for-each-ref", "--format=%(refname:short)", "refs/heads"))
}

// newFixtureRepo returns the symlink-resolved root of a repo with one commit
// holding file.txt and a .gitignore that ignores bin/.
func newFixtureRepo(t *testing.T) string {
	t.Helper()
	root := newTempDir(t)
	gitOutput(t, root, "init", "-q", "-b", "main")
	writeFile(t, filepath.Join(root, "file.txt"), "one\n")
	writeFile(t, filepath.Join(root, ".gitignore"), "bin/\n")
	commitAll(t, root, "init")
	return root
}

// newPrebuiltRepo returns a fixture repo whose ignored bin/ holds the built
// outputs bin/acdsl and bin/verifiers/gofmt.
func newPrebuiltRepo(t *testing.T) string {
	t.Helper()
	root := newFixtureRepo(t)
	writeFile(t, filepath.Join(root, "bin", "acdsl"), "acdsl\n")
	writeFile(t, filepath.Join(root, "bin", "verifiers", "gofmt"), "gofmt\n")
	return root
}

// newTempDir returns a symlink-resolved temp dir, matching the absolute paths
// git reports.
func newTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	return dir
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readOrEmpty(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(content)
}

func reasonOf(result *SalvageResult) string {
	if result == nil {
		return ""
	}
	return result.Reason
}

// skippedBranches cuts each skipped row to its branch name, dropping git's
// refusal text.
func skippedBranches(result *BranchResult) []string {
	if result == nil {
		return nil
	}

	branches := make([]string, 0, len(result.Skipped))
	for _, row := range result.Skipped {
		branch, _, _ := strings.Cut(row, ":")
		branches = append(branches, branch)
	}
	return branches
}

// withoutReason copies result with Reason cleared, so git's wording stays out
// of the struct comparison.
func withoutReason(result *SalvageResult) *SalvageResult {
	if result == nil {
		return nil
	}

	copied := *result
	copied.Reason = ""
	return &copied
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestArchiveBranch(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedFiles []string
		_expectedSha   string
		_shouldPass    bool

		branch string
		dest   string
		repo   string
	}

	tests := make([]*testCase, 0)

	// diff-written-sha-returned
	diffRepo := newFixtureRepo(t)
	diffWorktree := filepath.Join(newTempDir(t), "wt")
	addWorktree(t, "claude/feature", diffWorktree, diffRepo)
	writeFile(t, filepath.Join(diffWorktree, "committed.txt"), "change\n")
	commitAll(t, diffWorktree, "change")
	tests = append(tests, &testCase{
		_id:            "diff-written-sha-returned",
		_expectedFiles: []string{"claude_feature.diff"},
		_expectedSha:   gitOutput(t, diffRepo, "rev-parse", "claude/feature"),
		_shouldPass:    true,

		branch: "claude/feature",
		dest:   filepath.Join(newTempDir(t), "branches"),
		repo:   diffRepo,
	})

	// empty-diff-writes-nothing
	emptyRepo := newFixtureRepo(t)
	gitOutput(t, emptyRepo, "branch", "claude/empty")
	tests = append(tests, &testCase{
		_id:            "empty-diff-writes-nothing",
		_expectedFiles: []string{},
		_expectedSha:   gitOutput(t, emptyRepo, "rev-parse", "claude/empty"),
		_shouldPass:    true,

		branch: "claude/empty",
		dest:   filepath.Join(newTempDir(t), "branches"),
		repo:   emptyRepo,
	})

	// unknown-branch-errors
	unknownRepo := newFixtureRepo(t)
	tests = append(tests, &testCase{
		_id:            "unknown-branch-errors",
		_expectedFiles: []string{},
		_expectedSha:   "",
		_shouldPass:    false,

		branch: "claude/missing",
		dest:   filepath.Join(newTempDir(t), "branches"),
		repo:   unknownRepo,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			sha, err := ArchiveBranch(context.Background(), "main", test.branch, test.dest, test.repo)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedSha, sha)
			assert.Equal(t, test._expectedFiles, listFiles(t, test.dest))
		})
	}
}

func TestBranches(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		prefix string
		repo   string
	}

	repo := newFixtureRepo(t)
	gitOutput(t, repo, "branch", "claude-review/rr-a-1")
	gitOutput(t, repo, "branch", "claude-review/rr-a-2")
	gitOutput(t, repo, "branch", "claude-review/rr-b-1")
	tests := make([]*testCase, 0)

	// under-prefix
	tests = append(tests, &testCase{
		_id:       "under-prefix",
		_expected: []string{"claude-review/rr-a-1", "claude-review/rr-a-2"},

		prefix: "claude-review/rr-a-",
		repo:   repo,
	})

	// no-match
	tests = append(tests, &testCase{
		_id:       "no-match",
		_expected: []string{},

		prefix: "claude-review/rr-z-",
		repo:   repo,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			branches, err := Branches(context.Background(), test.prefix, test.repo)
			require.NoError(t, err)
			assert.Equal(t, test._expected, branches)
		})
	}
}

func TestList(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []Worktree

		repo string
	}

	tests := make([]*testCase, 0)

	// main-and-linked
	linkedRepo := newFixtureRepo(t)
	linkedHead := gitOutput(t, linkedRepo, "rev-parse", "HEAD")
	linkedPath := filepath.Join(newTempDir(t), "feature")
	addWorktree(t, "feature", linkedPath, linkedRepo)
	tests = append(tests, &testCase{
		_id: "main-and-linked",
		_expected: []Worktree{
			{Branch: "main", Head: linkedHead, Path: linkedRepo},
			{Branch: "feature", Head: linkedHead, Path: linkedPath},
		},

		repo: linkedRepo,
	})

	// detached-worktree
	detachedRepo := newFixtureRepo(t)
	detachedHead := gitOutput(t, detachedRepo, "rev-parse", "HEAD")
	detachedPath := filepath.Join(newTempDir(t), "detached")
	addWorktree(t, "", detachedPath, detachedRepo)
	tests = append(tests, &testCase{
		_id: "detached-worktree",
		_expected: []Worktree{
			{Branch: "main", Head: detachedHead, Path: detachedRepo},
			{Branch: "", Head: detachedHead, Path: detachedPath},
		},

		repo: detachedRepo,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			worktrees, err := List(context.Background(), test.repo)
			require.NoError(t, err)
			assert.Equal(t, test._expected, worktrees)
		})
	}
}

func TestProvision(t *testing.T) {
	type testCase struct {
		_id                 string
		_expected           *Provisioned
		_expectedBranches   []string
		_expectedContent    string
		_expectedLinks      []string
		_expectedPathExists bool
		_shouldPass         bool

		request *ProvisionRequest
	}

	tests := make([]*testCase, 0)

	// fresh-worktree
	freshRequest := &ProvisionRequest{Branch: "claude-review/rr-x-lane1", Head: "HEAD", Repo: newFixtureRepo(t), Root: newTempDir(t), Slug: "lane1"}
	tests = append(tests, &testCase{
		_id:                 "fresh-worktree",
		_expected:           &Provisioned{Branch: "claude-review/rr-x-lane1", Path: filepath.Join(freshRequest.Root, "lane1")},
		_expectedBranches:   []string{"claude-review/rr-x-lane1", "main"},
		_expectedContent:    "one\n",
		_expectedLinks:      []string{},
		_expectedPathExists: true,
		_shouldPass:         true,

		request: freshRequest,
	})

	// path-exists
	existsRequest := &ProvisionRequest{Branch: "claude-review/rr-x-lane1", Head: "HEAD", Repo: newFixtureRepo(t), Root: newTempDir(t), Slug: "lane1"}
	require.NoError(t, os.MkdirAll(filepath.Join(existsRequest.Root, "lane1"), 0o755))
	tests = append(tests, &testCase{
		_id:                 "path-exists",
		_expected:           nil,
		_expectedBranches:   []string{"main"},
		_expectedContent:    "",
		_expectedLinks:      []string{},
		_expectedPathExists: true,
		_shouldPass:         false,

		request: existsRequest,
	})

	// overlay-applies
	appliesOverlay := filepath.Join(newTempDir(t), "overlay.patch")
	writeFile(t, appliesOverlay, appendTwoPatch)
	appliesRequest := &ProvisionRequest{Branch: "claude-review/rr-x-lane1", Head: "HEAD", Overlay: appliesOverlay, Repo: newFixtureRepo(t), Root: newTempDir(t), Slug: "lane1"}
	tests = append(tests, &testCase{
		_id:                 "overlay-applies",
		_expected:           &Provisioned{Branch: "claude-review/rr-x-lane1", Path: filepath.Join(appliesRequest.Root, "lane1")},
		_expectedBranches:   []string{"claude-review/rr-x-lane1", "main"},
		_expectedContent:    "one\ntwo\n",
		_expectedLinks:      []string{},
		_expectedPathExists: true,
		_shouldPass:         true,

		request: appliesRequest,
	})

	// overlay-fails-rolls-back
	failsOverlay := filepath.Join(newTempDir(t), "overlay.patch")
	writeFile(t, failsOverlay, stalePatch)
	failsRequest := &ProvisionRequest{Branch: "claude-review/rr-x-lane1", Head: "HEAD", Overlay: failsOverlay, Repo: newFixtureRepo(t), Root: newTempDir(t), Slug: "lane1"}
	tests = append(tests, &testCase{
		_id:                 "overlay-fails-rolls-back",
		_expected:           nil,
		_expectedBranches:   []string{"main"},
		_expectedContent:    "",
		_expectedLinks:      []string{},
		_expectedPathExists: false,
		_shouldPass:         false,

		request: failsRequest,
	})

	// existing-worktree-reused
	reusedRequest := &ProvisionRequest{Branch: "claude-review/rr-x-lane1", Head: "HEAD", Repo: newFixtureRepo(t), Root: newTempDir(t), Slug: "lane1"}
	_, err := Provision(context.Background(), reusedRequest)
	require.NoError(t, err)
	tests = append(tests, &testCase{
		_id:                 "existing-worktree-reused",
		_expected:           &Provisioned{Branch: "claude-review/rr-x-lane1", Path: filepath.Join(reusedRequest.Root, "lane1")},
		_expectedBranches:   []string{"claude-review/rr-x-lane1", "main"},
		_expectedContent:    "one\n",
		_expectedLinks:      []string{},
		_expectedPathExists: true,
		_shouldPass:         true,

		request: reusedRequest,
	})

	// existing-overlay-worktree-reused
	reusedOverlay := filepath.Join(newTempDir(t), "overlay.patch")
	writeFile(t, reusedOverlay, appendTwoPatch)
	reusedOverlayRequest := &ProvisionRequest{Branch: "claude-review/rr-x-lane1", Head: "HEAD", Overlay: reusedOverlay, Repo: newFixtureRepo(t), Root: newTempDir(t), Slug: "lane1"}
	_, err = Provision(context.Background(), reusedOverlayRequest)
	require.NoError(t, err)
	tests = append(tests, &testCase{
		_id:                 "existing-overlay-worktree-reused",
		_expected:           &Provisioned{Branch: "claude-review/rr-x-lane1", Path: filepath.Join(reusedOverlayRequest.Root, "lane1")},
		_expectedBranches:   []string{"claude-review/rr-x-lane1", "main"},
		_expectedContent:    "one\ntwo\n",
		_expectedLinks:      []string{},
		_expectedPathExists: true,
		_shouldPass:         true,

		request: reusedOverlayRequest,
	})

	// existing-worktree-other-branch
	otherBranchRequest := &ProvisionRequest{Branch: "claude-review/rr-x-lane1", Head: "HEAD", Repo: newFixtureRepo(t), Root: newTempDir(t), Slug: "lane1"}
	_, err = Provision(context.Background(), otherBranchRequest)
	require.NoError(t, err)
	tests = append(tests, &testCase{
		_id:                 "existing-worktree-other-branch",
		_expected:           nil,
		_expectedBranches:   []string{"claude-review/rr-x-lane1", "main"},
		_expectedContent:    "one\n",
		_expectedLinks:      []string{},
		_expectedPathExists: true,
		_shouldPass:         false,

		request: &ProvisionRequest{Branch: "claude-review/rr-x-lane2", Head: "HEAD", Repo: otherBranchRequest.Repo, Root: otherBranchRequest.Root, Slug: "lane1"},
	})

	// existing-worktree-other-head
	otherHeadRequest := &ProvisionRequest{Branch: "claude-review/rr-x-lane1", Head: "HEAD", Repo: newFixtureRepo(t), Root: newTempDir(t), Slug: "lane1"}
	_, err = Provision(context.Background(), otherHeadRequest)
	require.NoError(t, err)
	writeFile(t, filepath.Join(otherHeadRequest.Repo, "moved.txt"), "moved\n")
	commitAll(t, otherHeadRequest.Repo, "moved")
	tests = append(tests, &testCase{
		_id:                 "existing-worktree-other-head",
		_expected:           nil,
		_expectedBranches:   []string{"claude-review/rr-x-lane1", "main"},
		_expectedContent:    "one\n",
		_expectedLinks:      []string{},
		_expectedPathExists: true,
		_shouldPass:         false,

		request: otherHeadRequest,
	})

	// prebuilt-file-linked
	fileRequest := &ProvisionRequest{Branch: "claude-review/rr-x-lane1", Head: "HEAD", Prebuilt: []string{"bin/acdsl"}, Repo: newPrebuiltRepo(t), Root: newTempDir(t), Slug: "lane1"}
	tests = append(tests, &testCase{
		_id:                 "prebuilt-file-linked",
		_expected:           &Provisioned{Branch: "claude-review/rr-x-lane1", Path: filepath.Join(fileRequest.Root, "lane1")},
		_expectedBranches:   []string{"claude-review/rr-x-lane1", "main"},
		_expectedContent:    "one\n",
		_expectedLinks:      []string{filepath.Join("bin", "acdsl") + " -> " + filepath.Join(fileRequest.Repo, "bin", "acdsl")},
		_expectedPathExists: true,
		_shouldPass:         true,

		request: fileRequest,
	})

	// prebuilt-dir-linked
	dirRequest := &ProvisionRequest{Branch: "claude-review/rr-x-lane1", Head: "HEAD", Prebuilt: []string{"bin/verifiers"}, Repo: newPrebuiltRepo(t), Root: newTempDir(t), Slug: "lane1"}
	tests = append(tests, &testCase{
		_id:                 "prebuilt-dir-linked",
		_expected:           &Provisioned{Branch: "claude-review/rr-x-lane1", Path: filepath.Join(dirRequest.Root, "lane1")},
		_expectedBranches:   []string{"claude-review/rr-x-lane1", "main"},
		_expectedContent:    "one\n",
		_expectedLinks:      []string{filepath.Join("bin", "verifiers") + " -> " + filepath.Join(dirRequest.Repo, "bin", "verifiers")},
		_expectedPathExists: true,
		_shouldPass:         true,

		request: dirRequest,
	})

	// prebuilt-with-overlay
	prebuiltOverlay := filepath.Join(newTempDir(t), "overlay.patch")
	writeFile(t, prebuiltOverlay, appendTwoPatch)
	withOverlayRequest := &ProvisionRequest{Branch: "claude-review/rr-x-lane1", Head: "HEAD", Overlay: prebuiltOverlay, Prebuilt: []string{"bin/acdsl", "bin/verifiers"}, Repo: newPrebuiltRepo(t), Root: newTempDir(t), Slug: "lane1"}
	tests = append(tests, &testCase{
		_id:               "prebuilt-with-overlay",
		_expected:         &Provisioned{Branch: "claude-review/rr-x-lane1", Path: filepath.Join(withOverlayRequest.Root, "lane1")},
		_expectedBranches: []string{"claude-review/rr-x-lane1", "main"},
		_expectedContent:  "one\ntwo\n",
		_expectedLinks: []string{
			filepath.Join("bin", "acdsl") + " -> " + filepath.Join(withOverlayRequest.Repo, "bin", "acdsl"),
			filepath.Join("bin", "verifiers") + " -> " + filepath.Join(withOverlayRequest.Repo, "bin", "verifiers"),
		},
		_expectedPathExists: true,
		_shouldPass:         true,

		request: withOverlayRequest,
	})

	// prebuilt-missing-refused
	missingRequest := &ProvisionRequest{Branch: "claude-review/rr-x-lane1", Head: "HEAD", Prebuilt: []string{"bin/acdsl"}, Repo: newFixtureRepo(t), Root: newTempDir(t), Slug: "lane1"}
	tests = append(tests, &testCase{
		_id:                 "prebuilt-missing-refused",
		_expected:           nil,
		_expectedBranches:   []string{"main"},
		_expectedContent:    "",
		_expectedLinks:      []string{},
		_expectedPathExists: false,
		_shouldPass:         false,

		request: missingRequest,
	})

	// prebuilt-outside-repo-refused
	outsideRequest := &ProvisionRequest{Branch: "claude-review/rr-x-lane1", Head: "HEAD", Prebuilt: []string{"../bin/acdsl"}, Repo: newPrebuiltRepo(t), Root: newTempDir(t), Slug: "lane1"}
	tests = append(tests, &testCase{
		_id:                 "prebuilt-outside-repo-refused",
		_expected:           nil,
		_expectedBranches:   []string{"main"},
		_expectedContent:    "",
		_expectedLinks:      []string{},
		_expectedPathExists: false,
		_shouldPass:         false,

		request: outsideRequest,
	})

	// prebuilt-relative-repo
	workingDir, err := os.Getwd()
	require.NoError(t, err)
	relativeRepo := newPrebuiltRepo(t)
	relativeRepoPath, err := filepath.Rel(workingDir, relativeRepo)
	require.NoError(t, err)
	relativeRequest := &ProvisionRequest{Branch: "claude-review/rr-x-lane1", Head: "HEAD", Prebuilt: []string{"bin/acdsl"}, Repo: relativeRepoPath, Root: newTempDir(t), Slug: "lane1"}
	tests = append(tests, &testCase{
		_id:                 "prebuilt-relative-repo",
		_expected:           &Provisioned{Branch: "claude-review/rr-x-lane1", Path: filepath.Join(relativeRequest.Root, "lane1")},
		_expectedBranches:   []string{"claude-review/rr-x-lane1", "main"},
		_expectedContent:    "one\n",
		_expectedLinks:      []string{filepath.Join("bin", "acdsl") + " -> " + filepath.Join(relativeRepo, "bin", "acdsl")},
		_expectedPathExists: true,
		_shouldPass:         true,

		request: relativeRequest,
	})

	// no-prebuilt-unchanged
	unchangedRequest := &ProvisionRequest{Branch: "claude-review/rr-x-lane1", Head: "HEAD", Repo: newPrebuiltRepo(t), Root: newTempDir(t), Slug: "lane1"}
	tests = append(tests, &testCase{
		_id:                 "no-prebuilt-unchanged",
		_expected:           &Provisioned{Branch: "claude-review/rr-x-lane1", Path: filepath.Join(unchangedRequest.Root, "lane1")},
		_expectedBranches:   []string{"claude-review/rr-x-lane1", "main"},
		_expectedContent:    "one\n",
		_expectedLinks:      []string{},
		_expectedPathExists: true,
		_shouldPass:         true,

		request: unchangedRequest,
	})

	// reuse-keeps-links
	reuseLinksRequest := &ProvisionRequest{Branch: "claude-review/rr-x-lane1", Head: "HEAD", Prebuilt: []string{"bin/acdsl"}, Repo: newPrebuiltRepo(t), Root: newTempDir(t), Slug: "lane1"}
	_, err = Provision(context.Background(), reuseLinksRequest)
	require.NoError(t, err)
	tests = append(tests, &testCase{
		_id:                 "reuse-keeps-links",
		_expected:           &Provisioned{Branch: "claude-review/rr-x-lane1", Path: filepath.Join(reuseLinksRequest.Root, "lane1")},
		_expectedBranches:   []string{"claude-review/rr-x-lane1", "main"},
		_expectedContent:    "one\n",
		_expectedLinks:      []string{filepath.Join("bin", "acdsl") + " -> " + filepath.Join(reuseLinksRequest.Repo, "bin", "acdsl")},
		_expectedPathExists: true,
		_shouldPass:         true,

		request: reuseLinksRequest,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			provisioned, err := Provision(context.Background(), test.request)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, provisioned)
			assert.Equal(t, test._expectedPathExists, pathExists(filepath.Join(test.request.Root, test.request.Slug)))
			assert.Equal(t, test._expectedBranches, localBranches(t, test.request.Repo))
			assert.Equal(t, test._expectedContent, readOrEmpty(filepath.Join(test.request.Root, test.request.Slug, "file.txt")))
			assert.Equal(t, test._expectedLinks, listLinks(t, filepath.Join(test.request.Root, test.request.Slug)))
		})
	}
}

func TestRemoveBranches(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedBranches []string
		_expectedDeleted  []string
		_expectedFiles    []string
		_expectedSkipped  []string

		request *BranchRequest
	}

	tests := make([]*testCase, 0)

	// prefix-only
	prefixRepo := newFixtureRepo(t)
	gitOutput(t, prefixRepo, "branch", "claude-review/rr-a-1")
	gitOutput(t, prefixRepo, "branch", "claude-review/rr-a-2")
	gitOutput(t, prefixRepo, "branch", "other/keep")
	tests = append(tests, &testCase{
		_id:               "prefix-only",
		_expectedBranches: []string{"main", "other/keep"},
		_expectedDeleted:  []string{"claude-review/rr-a-1", "claude-review/rr-a-2"},
		_expectedFiles:    []string{},
		_expectedSkipped:  []string{},

		request: &BranchRequest{Base: "HEAD", Dest: filepath.Join(newTempDir(t), "branches"), Prefix: "claude-review/rr-a-", Repo: prefixRepo},
	})

	// diff-saved-before-delete
	diffRepo := newFixtureRepo(t)
	diffWorktree := filepath.Join(newTempDir(t), "wt")
	addWorktree(t, "claude-review/rr-a-1", diffWorktree, diffRepo)
	writeFile(t, filepath.Join(diffWorktree, "committed.txt"), "change\n")
	commitAll(t, diffWorktree, "change")
	gitOutput(t, diffRepo, "worktree", "remove", diffWorktree)
	gitOutput(t, diffRepo, "branch", "claude-review/rr-a-2")
	tests = append(tests, &testCase{
		_id:               "diff-saved-before-delete",
		_expectedBranches: []string{"main"},
		_expectedDeleted:  []string{"claude-review/rr-a-1", "claude-review/rr-a-2"},
		_expectedFiles:    []string{"claude-review_rr-a-1.diff"},
		_expectedSkipped:  []string{},

		request: &BranchRequest{Base: "HEAD", Dest: filepath.Join(newTempDir(t), "branches"), Prefix: "claude-review/rr-a-", Repo: diffRepo},
	})

	// checked-out-branch-skipped
	checkedOutRepo := newFixtureRepo(t)
	addWorktree(t, "claude-review/rr-a-1", filepath.Join(newTempDir(t), "wt"), checkedOutRepo)
	tests = append(tests, &testCase{
		_id:               "checked-out-branch-skipped",
		_expectedBranches: []string{"claude-review/rr-a-1", "main"},
		_expectedDeleted:  []string{},
		_expectedFiles:    []string{},
		_expectedSkipped:  []string{"claude-review/rr-a-1"},

		request: &BranchRequest{Base: "HEAD", Dest: filepath.Join(newTempDir(t), "branches"), Prefix: "claude-review/rr-a-", Repo: checkedOutRepo},
	})

	// names-exact
	exactRepo := newFixtureRepo(t)
	gitOutput(t, exactRepo, "branch", "claude-review/rr-a-buildtest-r1")
	gitOutput(t, exactRepo, "branch", "claude-review/rr-a-buildtest-r10")
	tests = append(tests, &testCase{
		_id:               "names-exact",
		_expectedBranches: []string{"claude-review/rr-a-buildtest-r10", "main"},
		_expectedDeleted:  []string{"claude-review/rr-a-buildtest-r1"},
		_expectedFiles:    []string{},
		_expectedSkipped:  []string{},

		request: &BranchRequest{Base: "HEAD", Dest: filepath.Join(newTempDir(t), "branches"), Names: []string{"claude-review/rr-a-buildtest-r1"}, Repo: exactRepo},
	})

	// names-beside-prefix
	besideRepo := newFixtureRepo(t)
	gitOutput(t, besideRepo, "branch", "claude-review/rr-a-1")
	gitOutput(t, besideRepo, "branch", "worktree-wf_071e500f-edc-1")
	gitOutput(t, besideRepo, "branch", "other/keep")
	tests = append(tests, &testCase{
		_id:               "names-beside-prefix",
		_expectedBranches: []string{"main", "other/keep"},
		_expectedDeleted:  []string{"claude-review/rr-a-1", "worktree-wf_071e500f-edc-1"},
		_expectedFiles:    []string{},
		_expectedSkipped:  []string{},

		request: &BranchRequest{Base: "HEAD", Dest: filepath.Join(newTempDir(t), "branches"), Names: []string{"worktree-wf_071e500f-edc-1", "claude-review/rr-a-1"}, Prefix: "claude-review/rr-a-", Repo: besideRepo},
	})

	// empty-prefix-without-names
	emptyRepo := newFixtureRepo(t)
	gitOutput(t, emptyRepo, "branch", "claude-review/rr-a-1")
	tests = append(tests, &testCase{
		_id:               "empty-prefix-without-names",
		_expectedBranches: []string{"claude-review/rr-a-1", "main"},
		_expectedDeleted:  []string{},
		_expectedFiles:    []string{},
		_expectedSkipped:  []string{},

		request: &BranchRequest{Base: "HEAD", Dest: filepath.Join(newTempDir(t), "branches"), Repo: emptyRepo},
	})

	// name-checked-out-skipped
	namedRepo := newFixtureRepo(t)
	addWorktree(t, "worktree-wf_071e500f-edc-1", filepath.Join(newTempDir(t), "wt"), namedRepo)
	tests = append(tests, &testCase{
		_id:               "name-checked-out-skipped",
		_expectedBranches: []string{"main", "worktree-wf_071e500f-edc-1"},
		_expectedDeleted:  []string{},
		_expectedFiles:    []string{},
		_expectedSkipped:  []string{"worktree-wf_071e500f-edc-1"},

		request: &BranchRequest{Base: "HEAD", Dest: filepath.Join(newTempDir(t), "branches"), Names: []string{"worktree-wf_071e500f-edc-1"}, Repo: namedRepo},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			result, err := RemoveBranches(context.Background(), test.request)
			require.NoError(t, err)
			assert.Equal(t, test._expectedDeleted, result.Deleted)
			assert.Equal(t, test._expectedSkipped, skippedBranches(result))
			assert.Equal(t, test._expectedBranches, localBranches(t, test.request.Repo))
			assert.Equal(t, test._expectedFiles, listFiles(t, test.request.Dest))
		})
	}
}

func TestSalvage(t *testing.T) {
	type testCase struct {
		_id                    string
		_expected              *SalvageResult
		_expectedFiles         []string
		_expectedLinks         []string
		_expectedPathExists    bool
		_expectedReasonPart    string
		_expectedSourceContent string

		request *SalvageRequest
		source  string
	}

	tests := make([]*testCase, 0)

	// clean-tree-removed
	cleanRepo := newFixtureRepo(t)
	cleanWorktree := filepath.Join(newTempDir(t), "wt")
	addWorktree(t, "claude-review/rr-a-1", cleanWorktree, cleanRepo)
	tests = append(tests, &testCase{
		_id:                    "clean-tree-removed",
		_expected:              &SalvageResult{OverlayReversed: true, Path: cleanWorktree, Removed: true},
		_expectedFiles:         []string{},
		_expectedLinks:         []string{},
		_expectedPathExists:    false,
		_expectedReasonPart:    "",
		_expectedSourceContent: "",

		request: &SalvageRequest{Dest: filepath.Join(newTempDir(t), "salvage"), Repo: cleanRepo, Worktree: cleanWorktree},
	})

	// tracked-edit-archived
	trackedRepo := newFixtureRepo(t)
	trackedWorktree := filepath.Join(newTempDir(t), "wt")
	addWorktree(t, "claude-review/rr-a-1", trackedWorktree, trackedRepo)
	writeFile(t, filepath.Join(trackedWorktree, "file.txt"), "one\nedit\n")
	tests = append(tests, &testCase{
		_id:                    "tracked-edit-archived",
		_expected:              &SalvageResult{OverlayReversed: true, Path: trackedWorktree, Removed: true, TrackedDiff: true},
		_expectedFiles:         []string{trackedDiffFile},
		_expectedLinks:         []string{},
		_expectedPathExists:    false,
		_expectedReasonPart:    "",
		_expectedSourceContent: "",

		request: &SalvageRequest{Dest: filepath.Join(newTempDir(t), "salvage"), Repo: trackedRepo, Worktree: trackedWorktree},
	})

	// untracked-and-ignored-moved
	untrackedRepo := newFixtureRepo(t)
	untrackedWorktree := filepath.Join(newTempDir(t), "wt")
	addWorktree(t, "claude-review/rr-a-1", untrackedWorktree, untrackedRepo)
	writeFile(t, filepath.Join(untrackedWorktree, "new.txt"), "new\n")
	writeFile(t, filepath.Join(untrackedWorktree, "bin", "tool"), "binary\n")
	tests = append(tests, &testCase{
		_id:                    "untracked-and-ignored-moved",
		_expected:              &SalvageResult{Moved: 2, OverlayReversed: true, Path: untrackedWorktree, Removed: true},
		_expectedFiles:         []string{filepath.Join("bin", "tool"), "new.txt"},
		_expectedLinks:         []string{},
		_expectedPathExists:    false,
		_expectedReasonPart:    "",
		_expectedSourceContent: "",

		request: &SalvageRequest{Dest: filepath.Join(newTempDir(t), "salvage"), Repo: untrackedRepo, Worktree: untrackedWorktree},
	})

	// commits-since-base-archived
	commitsRepo := newFixtureRepo(t)
	commitsBase := gitOutput(t, commitsRepo, "rev-parse", "HEAD")
	commitsWorktree := filepath.Join(newTempDir(t), "wt")
	addWorktree(t, "claude-review/rr-a-1", commitsWorktree, commitsRepo)
	writeFile(t, filepath.Join(commitsWorktree, "committed.txt"), "change\n")
	commitAll(t, commitsWorktree, "change")
	tests = append(tests, &testCase{
		_id:                    "commits-since-base-archived",
		_expected:              &SalvageResult{CommitsDiff: true, OverlayReversed: true, Path: commitsWorktree, Removed: true},
		_expectedFiles:         []string{commitsDiffFile},
		_expectedLinks:         []string{},
		_expectedPathExists:    false,
		_expectedReasonPart:    "",
		_expectedSourceContent: "",

		request: &SalvageRequest{Base: commitsBase, Dest: filepath.Join(newTempDir(t), "salvage"), Repo: commitsRepo, Worktree: commitsWorktree},
	})

	// overlay-reversed
	overlayRepo := newFixtureRepo(t)
	overlayPatch := filepath.Join(newTempDir(t), "overlay.patch")
	writeFile(t, overlayPatch, appendTwoPatch)
	overlayWorktree := filepath.Join(newTempDir(t), "wt")
	addWorktree(t, "claude-review/rr-a-1", overlayWorktree, overlayRepo)
	gitOutput(t, overlayWorktree, "apply", overlayPatch)
	tests = append(tests, &testCase{
		_id:                    "overlay-reversed",
		_expected:              &SalvageResult{OverlayReversed: true, Path: overlayWorktree, Removed: true},
		_expectedFiles:         []string{},
		_expectedLinks:         []string{},
		_expectedPathExists:    false,
		_expectedReasonPart:    "",
		_expectedSourceContent: "",

		request: &SalvageRequest{Dest: filepath.Join(newTempDir(t), "salvage"), Overlay: overlayPatch, Repo: overlayRepo, Worktree: overlayWorktree},
	})

	// locked-worktree-refused
	lockedRepo := newFixtureRepo(t)
	lockedWorktree := filepath.Join(newTempDir(t), "wt")
	addWorktree(t, "claude-review/rr-a-1", lockedWorktree, lockedRepo)
	gitOutput(t, lockedRepo, "worktree", "lock", lockedWorktree)
	tests = append(tests, &testCase{
		_id:                    "locked-worktree-refused",
		_expected:              &SalvageResult{OverlayReversed: true, Path: lockedWorktree, Removed: false},
		_expectedFiles:         []string{},
		_expectedLinks:         []string{},
		_expectedPathExists:    true,
		_expectedReasonPart:    "locked",
		_expectedSourceContent: "",

		request: &SalvageRequest{Dest: filepath.Join(newTempDir(t), "salvage"), Repo: lockedRepo, Worktree: lockedWorktree},
	})

	// prebuilt-link-moved-source-intact
	linkedRepo := newPrebuiltRepo(t)
	linkedRoot := newTempDir(t)
	linkedRequest := &ProvisionRequest{Branch: "claude-review/rr-a-1", Head: "HEAD", Prebuilt: []string{"bin/acdsl"}, Repo: linkedRepo, Root: linkedRoot, Slug: "wt"}
	_, err := Provision(context.Background(), linkedRequest)
	require.NoError(t, err)
	linkedWorktree := filepath.Join(linkedRoot, "wt")
	tests = append(tests, &testCase{
		_id:                    "prebuilt-link-moved-source-intact",
		_expected:              &SalvageResult{Moved: 1, OverlayReversed: true, Path: linkedWorktree, Removed: true},
		_expectedFiles:         []string{filepath.Join("bin", "acdsl")},
		_expectedLinks:         []string{filepath.Join("bin", "acdsl") + " -> " + filepath.Join(linkedRepo, "bin", "acdsl")},
		_expectedPathExists:    false,
		_expectedReasonPart:    "",
		_expectedSourceContent: "acdsl\n",

		request: &SalvageRequest{Dest: filepath.Join(newTempDir(t), "salvage"), Repo: linkedRepo, Worktree: linkedWorktree},
		source:  filepath.Join(linkedRepo, "bin", "acdsl"),
	})

	// archive-only-keeps-worktree
	archiveRepo := newFixtureRepo(t)
	archiveBase := gitOutput(t, archiveRepo, "rev-parse", "HEAD")
	archiveWorktree := filepath.Join(newTempDir(t), "wt")
	addWorktree(t, "claude-review/rr-a-1", archiveWorktree, archiveRepo)
	writeFile(t, filepath.Join(archiveWorktree, "committed.txt"), "change\n")
	commitAll(t, archiveWorktree, "change")
	writeFile(t, filepath.Join(archiveWorktree, "file.txt"), "one\nedit\n")
	writeFile(t, filepath.Join(archiveWorktree, "new.txt"), "new\n")
	tests = append(tests, &testCase{
		_id:                    "archive-only-keeps-worktree",
		_expected:              &SalvageResult{CommitsDiff: true, Moved: 1, OverlayReversed: true, Path: archiveWorktree, Removed: false, TrackedDiff: true},
		_expectedFiles:         []string{commitsDiffFile, "new.txt", trackedDiffFile},
		_expectedLinks:         []string{},
		_expectedPathExists:    true,
		_expectedReasonPart:    "",
		_expectedSourceContent: "one\nedit\n",

		request: &SalvageRequest{Base: archiveBase, Dest: filepath.Join(newTempDir(t), "salvage"), IsArchiveOnly: true, Repo: archiveRepo, Worktree: archiveWorktree},
		source:  filepath.Join(archiveWorktree, "file.txt"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			salvaged, err := Salvage(context.Background(), test.request)
			require.NoError(t, err)
			assert.Equal(t, test._expected, withoutReason(salvaged))
			assert.Contains(t, reasonOf(salvaged), test._expectedReasonPart)
			assert.Equal(t, test._expectedPathExists, pathExists(test.request.Worktree))
			assert.Equal(t, test._expectedFiles, listFiles(t, test.request.Dest))
			assert.Equal(t, test._expectedLinks, listLinks(t, test.request.Dest))
			assert.Equal(t, test._expectedSourceContent, readOrEmpty(test.source))
		})
	}
}

package reviewscope

import (
	"path"
	"regexp"
	"strings"
)

// goExt is the Go source extension — the one language with a package fallback.
const goExt = ".go"

var (
	jsSource        = regexp.MustCompile(`^\.[cm]?[jt]sx?$`)
	jsTestSuffix    = regexp.MustCompile(`\.(test|spec)(\.[cm]?[jt]sx?)$`)
	testFilePattern = regexp.MustCompile(`(^|/)(tests?|__tests__|testdata)/|_test\.(go|py)$|(^|/)test_[^/]*\.py$|\.(test|spec)\.[cm]?[jt]sx?$`)
)

type TestPairs struct {
	Partners map[string][]string `json:"partners"`
	Tests    []string            `json:"tests"`
}

// counterparts keeps the naming-convention candidates that are tracked.
func counterparts(filePath string, isTest bool, tracked map[string]bool) []string {
	candidates := testCandidates(filePath)
	if isTest {
		candidates = productionCandidates(filePath)
	}

	found := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if tracked[candidate] && candidate != filePath {
			found = append(found, candidate)
		}
	}
	return found
}

// goFilesIn lists the production Go files of one directory among paths.
func goFilesIn(dir string, paths []string) []string {
	files := make([]string, 0)
	for _, candidate := range paths {
		isGoSource := path.Ext(candidate) == goExt && !IsTestFile(candidate)
		if isGoSource && path.Dir(candidate) == dir {
			files = append(files, candidate)
		}
	}
	return files
}

// packageFiles is a Go test's partner set when no same-stem file exists.
func packageFiles(testPath string, changed, tracked []string) []string {
	if path.Ext(testPath) != goExt {
		return nil
	}

	dir := path.Dir(testPath)
	if files := goFilesIn(dir, changed); len(files) > 0 {
		return files
	}
	return goFilesIn(dir, tracked)
}

// productionCandidates names the files a test file covers by convention.
func productionCandidates(testPath string) []string {
	dir, base := path.Split(testPath)
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	switch {
	case ext == goExt:
		return []string{dir + strings.TrimSuffix(stem, "_test") + ext}
	case ext == ".py":
		name := strings.TrimSuffix(strings.TrimPrefix(stem, "test_"), "_test") + ext
		return withParent(dir, "tests", name)
	case jsTestSuffix.MatchString(base):
		name := jsTestSuffix.ReplaceAllString(base, "$2")
		return withParent(dir, "__tests__", name)
	default:
		return nil
	}
}

// testCandidates names the test files covering a production file by convention.
func testCandidates(productionPath string) []string {
	dir, base := path.Split(productionPath)
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	switch {
	case ext == goExt:
		return []string{dir + stem + "_test" + goExt}
	case ext == ".py":
		return []string{dir + "test_" + base, dir + stem + "_test.py", dir + "tests/test_" + base}
	case jsSource.MatchString(ext):
		return []string{dir + stem + ".test" + ext, dir + stem + ".spec" + ext, dir + "__tests__/" + stem + ".test" + ext}
	default:
		return nil
	}
}

// withParent names a file in dir, plus in dir's parent when dir is a test directory.
func withParent(dir, testDir, name string) []string {
	candidates := []string{dir + name}
	if path.Base(dir) == testDir {
		parent := path.Dir(path.Clean(dir))
		candidates = append(candidates, path.Join(parent, name))
	}
	return candidates
}

// BuildTestPairs pairs every changed file with its tracked counterparts.
func BuildTestPairs(changed, tracked []string) *TestPairs {
	trackedSet := make(map[string]bool, len(tracked))
	for _, trackedPath := range tracked {
		trackedSet[trackedPath] = true
	}

	pairs := &TestPairs{Partners: map[string][]string{}, Tests: []string{}}
	for _, changedPath := range changed {
		isTest := IsTestFile(changedPath)
		if isTest {
			pairs.Tests = append(pairs.Tests, changedPath)
		}

		partners := counterparts(changedPath, isTest, trackedSet)
		if isTest && len(partners) == 0 {
			partners = packageFiles(changedPath, changed, tracked)
		}

		if len(partners) > 0 {
			pairs.Partners[changedPath] = partners
		}
	}
	return pairs
}

// IsTestFile reports whether a repo-relative path is a test file or test fixture.
func IsTestFile(filePath string) bool {
	return testFilePattern.MatchString(filePath)
}

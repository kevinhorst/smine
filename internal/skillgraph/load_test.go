package skillgraph

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// savedFixtureDir saves the fixture under <tmp>/root/group/demo — nested so
// no vocabulary.cue above the temp dir can leak into the lookup — and
// returns the skill dir.
func savedFixtureDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "root", "group", "demo")
	require.NoError(t, Save(dir, provideCompleteSkill()))
	return dir
}

// writeVocabulary saves a one-atom vocabulary at path.
func writeVocabulary(t *testing.T, id, path string) {
	t.Helper()
	vocabulary := &Vocabulary{Atoms: map[string]*Atom{id: {Sort: "state"}}}
	require.NoError(t, SaveVocabulary(path, vocabulary))
}

func TestLoad(t *testing.T) {
	type testCase struct {
		_id            string
		_errorContains string
		_sharedAtom    string
		_shouldPass    bool

		dir string
	}

	tests := make([]*testCase, 0)

	// pass-model-named-after-dir
	dir := savedFixtureDir(t)
	tests = append(tests, &testCase{_id: "pass-model-named-after-dir", _errorContains: "", _sharedAtom: "", _shouldPass: true, dir: dir})

	// pass-vocabulary-parent-wins
	dir = savedFixtureDir(t)
	writeVocabulary(t, "near", filepath.Join(dir, "..", FileVocabulary))
	writeVocabulary(t, "far", filepath.Join(dir, "..", "..", FileVocabulary))
	tests = append(tests, &testCase{_id: "pass-vocabulary-parent-wins", _errorContains: "", _sharedAtom: "near", _shouldPass: true, dir: dir})

	// pass-no-vocabulary
	dir = savedFixtureDir(t)
	tests = append(tests, &testCase{_id: "pass-no-vocabulary", _errorContains: "", _sharedAtom: "", _shouldPass: true, dir: dir})

	// fail-missing-model-file
	dir = savedFixtureDir(t)
	require.NoError(t, os.Remove(ModelPath(dir)))
	tests = append(tests, &testCase{_id: "fail-missing-model-file", _errorContains: "demo.cue", _sharedAtom: "", _shouldPass: false, dir: dir})

	// fail-legacy-model-file
	dir = savedFixtureDir(t)
	require.NoError(t, os.Rename(ModelPath(dir), filepath.Join(dir, LegacyModelFile)))
	tests = append(tests, &testCase{_id: "fail-legacy-model-file", _errorContains: "demo.cue", _sharedAtom: "", _shouldPass: false, dir: dir})

	// fail-name-differs-from-dir
	dir = filepath.Join(t.TempDir(), "root", "group", "other")
	require.NoError(t, Save(dir, provideCompleteSkill()))
	require.NoError(t, os.Rename(filepath.Join(dir, ModelFile("demo")), ModelPath(dir)))
	tests = append(tests, &testCase{_id: "fail-name-differs-from-dir", _errorContains: `Frontmatter name "demo" differs from the skill dir`, _sharedAtom: "", _shouldPass: false, dir: dir})

	// fail-missing-payload-file
	dir = savedFixtureDir(t)
	require.NoError(t, os.Remove(filepath.Join(dir, "payloads", "SKILL-DEMO-TPL-001.md")))
	tests = append(tests, &testCase{_id: "fail-missing-payload-file", _errorContains: "SKILL-DEMO-TPL-001.md", _sharedAtom: "", _shouldPass: false, dir: dir})

	// fail-version-mismatch
	dir = savedFixtureDir(t)
	modelPath := ModelPath(dir)
	data, err := os.ReadFile(modelPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(modelPath, []byte(replaceOnce(t, string(data), `version: "1.0"`, `version: "2.0"`)), 0o600))
	tests = append(tests, &testCase{_id: "fail-version-mismatch", _errorContains: "", _sharedAtom: "", _shouldPass: false, dir: dir})

	// fail-comment-in-model-file
	dir = savedFixtureDir(t)
	modelPath = ModelPath(dir)
	data, err = os.ReadFile(modelPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(modelPath, append([]byte("// note\n"), data...), 0o600))
	tests = append(tests, &testCase{_id: "fail-comment-in-model-file", _errorContains: "", _sharedAtom: "", _shouldPass: false, dir: dir})

	// fail-atom-without-sort
	dir = savedFixtureDir(t)
	modelPath = ModelPath(dir)
	data, err = os.ReadFile(modelPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(modelPath, []byte(replaceOnce(t, string(data), "first_done: {\n\t\tsort: \"state\"\n\t}", "first_done: {\n\t\tkind: \"bool\"\n\t}")), 0o600))
	tests = append(tests, &testCase{_id: "fail-atom-without-sort", _errorContains: "", _sharedAtom: "", _shouldPass: false, dir: dir})

	// pass-changelog-tokens
	dir = savedFixtureDir(t)
	modelPath = ModelPath(dir)
	data, err = os.ReadFile(modelPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(modelPath, []byte(replaceOnce(t, string(data), `version: "1.0"`, "tokens: 3210\n\t\tversion: \"1.0\"")), 0o600))
	tests = append(tests, &testCase{_id: "pass-changelog-tokens", _errorContains: "", _sharedAtom: "", _shouldPass: true, dir: dir})

	// fail-changelog-tokens-zero
	dir = savedFixtureDir(t)
	modelPath = ModelPath(dir)
	data, err = os.ReadFile(modelPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(modelPath, []byte(replaceOnce(t, string(data), `version: "1.0"`, "tokens: 0\n\t\tversion: \"1.0\"")), 0o600))
	tests = append(tests, &testCase{_id: "fail-changelog-tokens-zero", _errorContains: "", _sharedAtom: "", _shouldPass: false, dir: dir})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			skill, err := Load(test.dir)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Contains(t, fmt.Sprint(err), test._errorContains)
			assert.Equal(t, test._sharedAtom, sharedAtomId(skill))
		})
	}
}

func TestLoad_ChangelogTokensRoundTrip(t *testing.T) {
	skill := provideCompleteSkill()
	skill.Changelog[0].Tokens = 3210
	dir := filepath.Join(t.TempDir(), "root", "group", "demo")
	require.NoError(t, Save(dir, skill))

	loaded, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, skill.Changelog, loaded.Changelog, "load")

	require.NoError(t, Save(dir, loaded))
	reloaded, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, loaded.Changelog, reloaded.Changelog, "save and reload")
	assert.Equal(t, 3210, reloaded.Changelog[0].Tokens)
}

func TestLoad_ReadsPayloadBody(t *testing.T) {
	skill, err := Load(savedFixtureDir(t))
	require.NoError(t, err)

	ref, found := skill.Entry("SKILL-DEMO-TPL-001")
	require.True(t, found)
	assert.Equal(t, "## TLDR\n", ref.Entry.Payload.Body)
}

func TestDiscover(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, Save(filepath.Join(root, "group-b", "demo"), provideCompleteSkill()))
	require.NoError(t, Save(filepath.Join(root, "group-a", "demo"), provideCompleteSkill()))
	require.NoError(t, Save(filepath.Join(root, "demo"), provideCompleteSkill()))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("root file\n"), 0o600))

	dirs, err := Discover(root)
	require.NoError(t, err)
	want := []string{filepath.Join(root, "demo"), filepath.Join(root, "group-a", "demo"), filepath.Join(root, "group-b", "demo")}
	assert.Equal(t, want, dirs, "two-level-sorted, one-level, root-files-ignored")

	require.NoError(t, os.MkdirAll(filepath.Join(root, "group-a", "bare"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "group-a", "bare", FileManifest), []byte("---\n"), 0o600))
	_, err = Discover(root)
	require.ErrorContains(t, err, "has SKILL.md but no bare.cue", "fail-skill-md-without-model")
	require.NoError(t, os.RemoveAll(filepath.Join(root, "group-a", "bare")))

	legacyDir := filepath.Join(root, "group-b", "old")
	require.NoError(t, os.MkdirAll(legacyDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir, LegacyModelFile), []byte("x: 1\n"), 0o600))
	_, err = Discover(root)
	require.ErrorContains(t, err, "holds the legacy model file — run: git mv "+filepath.Join(legacyDir, LegacyModelFile)+" "+filepath.Join(legacyDir, "old.cue"), "fail-legacy-model-file")

	_, err = Discover(filepath.Join(root, "missing"))
	require.Error(t, err, "fail-missing-root")
}

func TestModelPaths(t *testing.T) {
	assert.Equal(t, "railroad-review.cue", ModelFile("railroad-review"))
	assert.Equal(t, filepath.Join("skills", "x", "y", "y.cue"), ModelPath(filepath.Join("skills", "x", "y")))
	assert.True(t, IsModelFile(filepath.Join("skills", "x", "y", "y.cue")), "model-file")
	assert.False(t, IsModelFile(filepath.Join("skills", "x", "y", LegacyModelFile)), "legacy-model-file")
	assert.False(t, IsModelFile(filepath.Join("skills", "interfaces.cue")), "catalog")
}

func TestFindRoot(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		dir        string
		vocabulary string
	}

	base := filepath.Join(t.TempDir(), "outer")
	tests := make([]*testCase, 0)

	// one-level
	tests = append(tests, &testCase{_id: "one-level", _expected: filepath.Join(base, "a", "skills"), dir: filepath.Join(base, "a", "skills", "leaf"), vocabulary: filepath.Join(base, "a", "skills")})

	// two-levels
	tests = append(tests, &testCase{_id: "two-levels", _expected: filepath.Join(base, "b", "skills"), dir: filepath.Join(base, "b", "skills", "group", "leaf"), vocabulary: filepath.Join(base, "b", "skills")})

	// none
	tests = append(tests, &testCase{_id: "none", _expected: "", dir: filepath.Join(base, "c", "skills", "group", "leaf")})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			require.NoError(t, os.MkdirAll(test.dir, 0o755))
			if test.vocabulary != "" {
				writeVocabulary(t, "shared", filepath.Join(test.vocabulary, FileVocabulary))
			}

			root, err := FindRoot(test.dir)
			require.NoError(t, err)
			assert.Equal(t, test._expected, root)
		})
	}
}

// replaceOnce replaces the one required occurrence of old in text.
func replaceOnce(t *testing.T, text, old, replacement string) string {
	t.Helper()
	require.Contains(t, text, old)
	return replaceFirst(text, old, replacement)
}

// replaceFirst replaces the first occurrence of old in text.
func replaceFirst(text, old, replacement string) string {
	index := len(text)
	for position := range len(text) - len(old) + 1 {
		if text[position:position+len(old)] == old {
			index = position
			break
		}
	}
	if index == len(text) {
		return text
	}
	return text[:index] + replacement + text[index+len(old):]
}

// sharedAtomId returns the single shared vocabulary atom id of a loaded
// skill, "" for none or a nil skill.
func sharedAtomId(skill *Skill) string {
	if skill == nil || skill.Vocabulary == nil {
		return ""
	}
	for id := range skill.Vocabulary.Atoms {
		return id
	}
	return ""
}

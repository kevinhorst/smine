package skillgraph

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/sessiontitle"
	"github.com/kevinhorst/smine/internal/skillenv"
)

func TestSave_RoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skills", "demo")
	writeVocabulary(t, "shared_ready", filepath.Join(dir, "..", FileVocabulary))
	require.NoError(t, Save(dir, provideCompleteSkill()))

	first, err := os.ReadFile(ModelPath(dir))
	require.NoError(t, err)

	loaded, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "shared_ready", sharedAtomId(loaded), "vocabulary merged")

	loaded.Catalog = nil
	loaded.Vocabulary = nil
	loaded.Dir = ""
	assert.Equal(t, provideCompleteSkill(), loaded, "Load == model")

	require.NoError(t, Save(dir, loaded))
	second, err := os.ReadFile(ModelPath(dir))
	require.NoError(t, err)
	assert.Equal(t, string(first), string(second), "two saves byte-identical, re-save idempotent")

	manifest, err := RenderManifest(loaded)
	require.NoError(t, err)
	assert.Equal(t, fixtureMarkdown, string(manifest))
}

func TestSave(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "demo")
	require.NoError(t, Save(dir, provideCompleteSkill()))

	payloadDir := filepath.Join(dir, DirPayloads)
	orphan := filepath.Join(payloadDir, "SKILL-DEMO-TPL-009.md")
	keptFile := filepath.Join(payloadDir, "notes.txt")
	keptDir := filepath.Join(payloadDir, "sub")
	require.NoError(t, os.WriteFile(orphan, []byte("orphan\n"), 0o600))
	require.NoError(t, os.WriteFile(keptFile, []byte("kept\n"), 0o600))
	require.NoError(t, os.MkdirAll(keptDir, 0o755))

	modelPath := ModelPath(dir)
	past := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, os.Chtimes(modelPath, past, past))
	require.NoError(t, Save(dir, provideCompleteSkill()))

	assert.NoFileExists(t, orphan, "orphan-payload-deleted")
	assert.FileExists(t, keptFile, "non-.md kept")
	assert.DirExists(t, keptDir, "subdirs kept")
	info, err := os.Stat(modelPath)
	require.NoError(t, err)
	assert.Equal(t, past, info.ModTime().UTC(), "pass-unchanged-file-keeps-mtime")

	invalid := provideCompleteSkill()
	invalid.Changelog[0].Version = "9.9"
	fresh := filepath.Join(t.TempDir(), "fresh")
	require.Error(t, Save(fresh, invalid))
	assert.NoDirExists(t, fresh, "fail-check-writes-nothing")

	declared := provideTitledSkill()
	declared.Enforce = demoEnforce()
	declaredDir := filepath.Join(provideCatalogRoot(t), "demo")
	require.NoError(t, Save(declaredDir, declared))
	leftovers, err := LeftoverRenders(declaredDir)
	require.NoError(t, err)
	assert.Empty(t, leftovers, "no-generated-file-written")

	loaded, err := Load(declaredDir)
	require.NoError(t, err)
	_, hasTitleSet := loaded.Catalog.Interface("title-set@1")
	assert.True(t, hasTitleSet, "catalog-attached-by-load")
	loaded.Catalog = declared.Catalog
	loaded.Vocabulary = nil
	loaded.Dir = ""
	assert.Equal(t, declared, loaded, "declarations-round-trip-through-the-model")

	smoked := filepath.Join(t.TempDir(), "demo")
	require.NoError(t, Save(smoked, provideSmokedSkill()))
	assert.NoFileExists(t, filepath.Join(smoked, FileSmoke), "no-smoke-file-written")
	loadedSmoked, err := Load(smoked)
	require.NoError(t, err)
	loadedSmoked.Catalog = nil
	loadedSmoked.Vocabulary = nil
	loadedSmoked.Dir = ""
	assert.Equal(t, provideSmokedSkill(), loadedSmoked, "smoke-block-round-trips-through-the-model")
}

func TestLeftoverRenders(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		dir string
	}

	tests := make([]*testCase, 0)

	// none
	tests = append(tests, &testCase{
		_id:       "none",
		_expected: nil,

		dir: t.TempDir(),
	})

	// manifest-only
	manifestDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(manifestDir, FileManifest), []byte("---\n"), 0o600))
	tests = append(tests, &testCase{
		_id:       "manifest-only",
		_expected: []string{filepath.Join(manifestDir, FileManifest)},

		dir: manifestDir,
	})

	// answer-md-reported
	answerDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(answerDir, FileAnswer), []byte("## Help\n"), 0o600))
	tests = append(tests, &testCase{
		_id:       "answer-md-reported",
		_expected: []string{filepath.Join(answerDir, FileAnswer)},

		dir: answerDir,
	})

	// env-json-reported
	envDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(envDir, skillenv.FileName), []byte("{}\n"), 0o600))
	tests = append(tests, &testCase{
		_id:       "env-json-reported",
		_expected: []string{filepath.Join(envDir, skillenv.FileName)},

		dir: envDir,
	})

	// all-four
	allDir := t.TempDir()
	for _, name := range []string{FileEnforce, FileManifest, FileSmoke, sessiontitle.FileName} {
		require.NoError(t, os.WriteFile(filepath.Join(allDir, name), []byte("{}\n"), 0o600))
	}
	tests = append(tests, &testCase{
		_id:       "all-four",
		_expected: []string{filepath.Join(allDir, FileEnforce), filepath.Join(allDir, FileManifest), filepath.Join(allDir, FileSmoke), filepath.Join(allDir, sessiontitle.FileName)},

		dir: allDir,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			leftovers, err := LeftoverRenders(test.dir)
			require.NoError(t, err)
			assert.Equal(t, test._expected, leftovers)
		})
	}
}

func TestSaveVocabulary(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileVocabulary)
	vocabulary := &Vocabulary{Atoms: map[string]*Atom{"ready": {Sort: "state"}}}
	require.NoError(t, SaveVocabulary(path, vocabulary))

	loaded, err := LoadVocabulary(path)
	require.NoError(t, err)
	assert.Equal(t, vocabulary, loaded, "round trip")

	invalidPath := filepath.Join(t.TempDir(), FileVocabulary)
	invalid := &Vocabulary{Atoms: map[string]*Atom{"bad id": {Sort: "state"}}}
	require.Error(t, SaveVocabulary(invalidPath, invalid))
	assert.NoFileExists(t, invalidPath, "fail-invalid-writes-nothing")
}

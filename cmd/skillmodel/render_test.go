package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/sessiontitle"
	"github.com/kevinhorst/smine/internal/skillenv"
	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skillimport"
	"github.com/kevinhorst/smine/internal/tokencount"
)

// renderFixtureBody holds one step and one payload entry, so the rendering
// carries a payload file.
const renderFixtureBody = "## Run\n\n" +
	"**SKILL-DEMO-RUN-001** `[step]` **Act**\n\nACT now.\n\n" +
	"**SKILL-DEMO-TPL-001** `[payload]` **Template**\n\nThe template.\n\n" +
	"```markdown\n# T\n```\n"

// writeMarkdownSkill writes a markdown-era demo skill — SKILL.md and
// changelog.json — into root/leaf and returns the dir.
func writeMarkdownSkill(t *testing.T, root, leaf string) string {
	t.Helper()
	dir := filepath.Join(root, leaf)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	manifest := "---\nname: demo\ndescription: demo fixture. Trigger on nothing.\nauthor: Kevin Horst\nversion: 1.0\nformat: 2\n---\n# Demo\n\n" + renderFixtureBody
	changelog := `[{"version": "1.0", "date": "2000-01-01", "text": "fixture"}]`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(manifest), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "changelog.json"), []byte(changelog), 0o644))
	return dir
}

// writeModeledSkill writes a demo skill and saves it as a model; the
// markdown-era SKILL.md and changelog.json are removed, as the cutover does.
func writeModeledSkill(t *testing.T, root, leaf string) string {
	t.Helper()
	dir := writeMarkdownSkill(t, root, leaf)
	skill, err := skillimport.Import(dir)
	require.NoError(t, err)
	require.NoError(t, skillgraph.Save(dir, skill))
	require.NoError(t, os.Remove(filepath.Join(dir, skillgraph.FileManifest)))
	require.NoError(t, os.Remove(filepath.Join(dir, legacyChangelogFile)))
	return dir
}

func TestCheckRendered(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		dir string
	}

	tests := make([]*testCase, 0)

	// canonical-clean
	tests = append(tests, &testCase{
		_id:       "canonical-clean",
		_expected: nil,

		dir: writeModeledSkill(t, t.TempDir(), "demo"),
	})

	// non-canonical-model
	nonCanonicalDir := writeModeledSkill(t, t.TempDir(), "demo")
	model, err := os.ReadFile(skillgraph.ModelPath(nonCanonicalDir))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(skillgraph.ModelPath(nonCanonicalDir), append(model, '\n'), 0o644))
	tests = append(tests, &testCase{
		_id:       "non-canonical-model",
		_expected: []string{"differs from its canonical form — run: make skill-render SKILL=demo"},

		dir: nonCanonicalDir,
	})

	// missing-payload
	payloadDir := writeModeledSkill(t, t.TempDir(), "demo")
	require.NoError(t, os.Remove(filepath.Join(payloadDir, skillgraph.DirPayloads, "SKILL-DEMO-TPL-001.md")))
	tests = append(tests, &testCase{
		_id:       "missing-payload",
		_expected: []string{"invalid model"},

		dir: payloadDir,
	})

	// orphan-payload
	orphanDir := writeModeledSkill(t, t.TempDir(), "demo")
	require.NoError(t, os.WriteFile(filepath.Join(orphanDir, skillgraph.DirPayloads, "X.md"), []byte("x\n"), 0o644))
	tests = append(tests, &testCase{
		_id:       "orphan-payload",
		_expected: []string{"orphaned payload"},

		dir: orphanDir,
	})

	// invalid-model
	invalidDir := writeModeledSkill(t, t.TempDir(), "demo")
	invalidModel, err := os.ReadFile(skillgraph.ModelPath(invalidDir))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(skillgraph.ModelPath(invalidDir), append([]byte("// note\n"), invalidModel...), 0o644))
	tests = append(tests, &testCase{
		_id:       "invalid-model",
		_expected: []string{"invalid model"},

		dir: invalidDir,
	})

	// leftover-changelog
	leftoverChangelogDir := writeModeledSkill(t, t.TempDir(), "demo")
	require.NoError(t, os.WriteFile(filepath.Join(leftoverChangelogDir, legacyChangelogFile), []byte("[]"), 0o644))
	tests = append(tests, &testCase{
		_id:       "leftover-changelog",
		_expected: []string{"second source next to the model file"},

		dir: leftoverChangelogDir,
	})

	// legacy-model-file
	legacyModelDir := writeModeledSkill(t, t.TempDir(), "demo")
	legacyModel, err := os.ReadFile(skillgraph.ModelPath(legacyModelDir))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(legacyModelDir, skillgraph.LegacyModelFile), legacyModel, 0o644))
	tests = append(tests, &testCase{
		_id:       "legacy-model-file",
		_expected: []string{skillgraph.LegacyModelFile + ": legacy model file — run: git mv " + filepath.Join(legacyModelDir, skillgraph.LegacyModelFile) + " " + skillgraph.ModelPath(legacyModelDir)},

		dir: legacyModelDir,
	})

	// leftover-skill-md
	leftoverManifestDir := writeModeledSkill(t, t.TempDir(), "demo")
	require.NoError(t, os.WriteFile(filepath.Join(leftoverManifestDir, skillgraph.FileManifest), []byte("# Demo\n"), 0o644))
	tests = append(tests, &testCase{
		_id:       "leftover-skill-md",
		_expected: []string{"SKILL.md: generated file in a skill dir — delete it, deploy renders it into the skill homes"},

		dir: leftoverManifestDir,
	})

	// leftover-title-json
	leftoverTitleDir := writeModeledSkill(t, t.TempDir(), "demo")
	require.NoError(t, os.WriteFile(filepath.Join(leftoverTitleDir, sessiontitle.FileName), []byte("{}\n"), 0o644))
	tests = append(tests, &testCase{
		_id:       "leftover-title-json",
		_expected: []string{sessiontitle.FileName + ": generated file in a skill dir"},

		dir: leftoverTitleDir,
	})

	// leftover-env-json
	leftoverEnvDir := writeModeledSkill(t, t.TempDir(), "demo")
	require.NoError(t, os.WriteFile(filepath.Join(leftoverEnvDir, skillenv.FileName), []byte("{}\n"), 0o644))
	tests = append(tests, &testCase{
		_id:       "leftover-env-json",
		_expected: []string{skillenv.FileName + ": generated file in a skill dir"},

		dir: leftoverEnvDir,
	})

	// leftover-enforce-json
	leftoverEnforceDir := writeModeledSkill(t, t.TempDir(), "demo")
	require.NoError(t, os.WriteFile(filepath.Join(leftoverEnforceDir, skillgraph.FileEnforce), []byte("{}\n"), 0o644))
	tests = append(tests, &testCase{
		_id:       "leftover-enforce-json",
		_expected: []string{skillgraph.FileEnforce + ": generated file in a skill dir"},

		dir: leftoverEnforceDir,
	})

	// legacy-atoms
	legacyDir := writeModeledSkill(t, t.TempDir(), "demo")
	require.NoError(t, os.WriteFile(filepath.Join(legacyDir, legacyAtomsFile), []byte("{}"), 0o644))
	tests = append(tests, &testCase{
		_id:       "legacy-atoms",
		_expected: []string{legacyAtomsFile + ": second source next to the model file — delete it"},

		dir: legacyDir,
	})

	// smoke-block-no-leftover
	smokedDir := writeModeledSkill(t, t.TempDir(), "demo")
	smoked, err := skillgraph.Load(smokedDir)
	require.NoError(t, err)
	smoked.Smoke = &skillgraph.Smoke{Skip: "fixture"}
	require.NoError(t, skillgraph.Save(smokedDir, smoked))
	require.NoFileExists(t, filepath.Join(smokedDir, skillgraph.FileSmoke))
	tests = append(tests, &testCase{
		_id:       "smoke-block-no-leftover",
		_expected: nil,

		dir: smokedDir,
	})

	// leftover-smoke-json
	leftoverSmokeDir := writeModeledSkill(t, t.TempDir(), "demo")
	require.NoError(t, os.WriteFile(filepath.Join(leftoverSmokeDir, skillgraph.FileSmoke), []byte("{}\n"), 0o644))
	tests = append(tests, &testCase{
		_id:       "leftover-smoke-json",
		_expected: []string{skillgraph.FileSmoke + ": generated file in a skill dir"},

		dir: leftoverSmokeDir,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, driftKinds(checkRendered(test.dir), test._expected))
		})
	}
}

// driftKinds maps each drift line to the expected fragment it contains, so a
// case asserts the line count and each line's kind in one comparison; a line
// matching no fragment stays verbatim and fails the comparison.
func driftKinds(drift, fragments []string) []string {
	var kinds []string
	for _, line := range drift {
		kind := line
		for _, fragment := range fragments {
			if strings.Contains(line, fragment) {
				kind = fragment
			}
		}
		kinds = append(kinds, kind)
	}
	return kinds
}

func TestRenderTargets(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedDirs      []string
		_expectedUnmodeled []string
		_shouldPass        bool

		args []string
	}

	skillRoot := t.TempDir()
	skillDir := writeModeledSkill(t, skillRoot, "demo")
	unmodeledDir := writeMarkdownSkill(t, t.TempDir(), "legacy")
	nestedRoot := t.TempDir()
	oneLevel := writeModeledSkill(t, nestedRoot, "demo")
	twoLevel := writeModeledSkill(t, filepath.Join(nestedRoot, "group"), "demo")
	brokenRoot := t.TempDir()
	writeModeledSkill(t, brokenRoot, "demo")
	writeMarkdownSkill(t, brokenRoot, "legacy")
	tests := make([]*testCase, 0)

	// skill-dir-arg
	tests = append(tests, &testCase{
		_id:                "skill-dir-arg",
		_expectedDirs:      []string{skillDir},
		_expectedUnmodeled: nil,
		_shouldPass:        true,

		args: []string{skillDir},
	})

	// unmodeled-arg
	tests = append(tests, &testCase{
		_id:                "unmodeled-arg",
		_expectedDirs:      nil,
		_expectedUnmodeled: []string{unmodeledDir},
		_shouldPass:        true,

		args: []string{unmodeledDir},
	})

	// root-arg
	tests = append(tests, &testCase{
		_id:                "root-arg",
		_expectedDirs:      []string{oneLevel, twoLevel},
		_expectedUnmodeled: nil,
		_shouldPass:        true,

		args: []string{nestedRoot},
	})

	// root-with-unmodeled-dir
	tests = append(tests, &testCase{
		_id:                "root-with-unmodeled-dir",
		_expectedDirs:      nil,
		_expectedUnmodeled: nil,
		_shouldPass:        false,

		args: []string{brokenRoot},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			dirs, unmodeled, err := renderTargets(test.args)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedDirs, dirs)
			assert.Equal(t, test._expectedUnmodeled, unmodeled)
		})
	}
}

func TestImportPath(t *testing.T) {
	t.Run("vocabulary-json", func(t *testing.T) {
		dir := t.TempDir()
		vocabularyPath := filepath.Join(dir, legacyVocabularyFile)
		vocabulary := `{"schemaVersion": 1, "sorts": {"mode": [{"id": "mode", "kind": "enum", "members": ["interactive", "nightly"], "source": "session"}]}}`
		require.NoError(t, os.WriteFile(vocabularyPath, []byte(vocabulary), 0o644))

		require.NoError(t, importPath(vocabularyPath))

		loaded, err := skillgraph.LoadVocabulary(filepath.Join(dir, skillgraph.FileVocabulary))
		require.NoError(t, err)
		assert.Equal(t, []string{"interactive", "nightly"}, loaded.Atoms["mode"].Members)
		assert.FileExists(t, vocabularyPath)
	})

	t.Run("skill-dir", func(t *testing.T) {
		dir := writeMarkdownSkill(t, t.TempDir(), "demo")

		require.NoError(t, importPath(dir))

		assert.FileExists(t, skillgraph.ModelPath(dir))
		assert.FileExists(t, filepath.Join(dir, skillgraph.DirPayloads, "SKILL-DEMO-TPL-001.md"))
		assert.FileExists(t, filepath.Join(dir, legacyChangelogFile))
		// The kept sources are the only drift: the model is canonical.
		leftover := []string{"generated file in a skill dir", "second source next to the model file"}
		assert.Equal(t, leftover, driftKinds(checkRendered(dir), leftover))
	})
}

// saveRoutedSkill saves a minimal skill named name with a routing block
// carrying edges into root/name and returns the dir.
func saveRoutedSkill(t *testing.T, root, name string, edges ...skillgraph.Edge) string {
	t.Helper()
	dir := filepath.Join(root, name)
	skill := &skillgraph.Skill{
		Changelog:   []skillgraph.ChangelogEntry{{Date: "2000-01-01", Text: "fixture", Version: "1.0"}},
		Frontmatter: []skillgraph.Field{{Key: "name", Value: " " + name}, {Key: "version", Value: " 1.0"}},
		Sections: []*skillgraph.Section{{
			Title: skillgraph.SectionRouting,
			Blocks: []skillgraph.Block{{Routing: &skillgraph.Routing{
				Pipeline:      skillgraph.Pipeline{Edges: edges, Standalone: true},
				Preconditions: []string{"none"},
				Purpose:       name + ".",
				Routes:        []skillgraph.Route{{For: "other", Use: "/other"}},
			}}},
		}},
		Smoke: &skillgraph.Smoke{Skip: "fixture"},
	}
	require.NoError(t, skillgraph.Save(dir, skill))
	return dir
}

// provideSkillsRoot writes a skills root — vocabulary.cue with the given
// shared atoms plus skills a → b with agreeing edges — and returns it.
func provideSkillsRoot(t *testing.T, shared ...string) string {
	t.Helper()
	root := t.TempDir()
	vocabulary := &skillgraph.Vocabulary{Atoms: map[string]*skillgraph.Atom{}}
	for _, id := range shared {
		vocabulary.Atoms[id] = &skillgraph.Atom{Sort: "state"}
	}
	require.NoError(t, skillgraph.SaveVocabulary(filepath.Join(root, skillgraph.FileVocabulary), vocabulary))
	saveRoutedSkill(t, root, "a", skillgraph.Edge{Kind: skillgraph.EdgeKindTo, Skill: "b"})
	saveRoutedSkill(t, root, "b", skillgraph.Edge{Kind: skillgraph.EdgeKindFrom, Skill: "a"})
	return root
}

func TestRootDrift(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		root string
	}

	tests := make([]*testCase, 0)

	// edges-agree
	tests = append(tests, &testCase{_id: "edges-agree", _expected: nil, root: provideSkillsRoot(t)})

	// one-sided-edge
	oneSidedRoot := provideSkillsRoot(t)
	saveRoutedSkill(t, oneSidedRoot, "c", skillgraph.Edge{Kind: skillgraph.EdgeKindSpawns, Skill: "a"})
	tests = append(tests, &testCase{_id: "one-sided-edge", _expected: []string{"c: pipeline edge spawns a — a declares no spawnedBy c"}, root: oneSidedRoot})

	// grant-without-edge
	grantRoot := provideSkillsRoot(t)
	grantDir := saveRoutedSkill(t, grantRoot, "c")
	granting, err := skillgraph.Load(grantDir)
	require.NoError(t, err)
	granting.Frontmatter = append(granting.Frontmatter, skillgraph.Field{Key: "allowed-tools", Value: " Skill(a)"})
	require.NoError(t, skillgraph.Save(grantDir, granting))
	tests = append(tests, &testCase{_id: "grant-without-edge", _expected: []string{"c: allowed-tools Skill(a) — no outgoing edge reaches a"}, root: grantRoot})

	// unread-atom
	tests = append(tests, &testCase{_id: "unread-atom", _expected: []string{"vocabulary.cue: atom shared: no skill's stop, rule, quality, route or query reads it"}, root: provideSkillsRoot(t, "shared")})

	// missing-smoke
	missingRoot := provideSkillsRoot(t)
	unsmoked, err := skillgraph.Load(filepath.Join(missingRoot, "b"))
	require.NoError(t, err)
	unsmoked.Smoke = nil
	require.NoError(t, skillgraph.Save(filepath.Join(missingRoot, "b"), unsmoked))
	tests = append(tests, &testCase{_id: "missing-smoke", _expected: []string{"b: no smoke block"}, root: missingRoot})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, driftKinds(rootDrift(test.root), test._expected))
		})
	}
}

func TestRenderRoots(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		args []string
	}

	root := provideSkillsRoot(t)
	outside := writeModeledSkill(t, t.TempDir(), "demo")
	tests := make([]*testCase, 0)

	// root-arg
	tests = append(tests, &testCase{_id: "root-arg", _expected: []string{root}, args: []string{root}})

	// skill-dirs-share-root
	tests = append(tests, &testCase{_id: "skill-dirs-share-root", _expected: []string{root}, args: []string{filepath.Join(root, "a"), filepath.Join(root, "b"), root}})

	// dir-outside-root
	tests = append(tests, &testCase{_id: "dir-outside-root", _expected: nil, args: []string{outside}})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			roots, err := renderRoots(test.args)
			require.NoError(t, err)
			assert.Equal(t, test._expected, roots)
		})
	}
}

func TestDeployedBody(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   string
		_expectedOk bool

		files []skillgraph.RenderedFile
	}

	tests := make([]*testCase, 0)

	// frontmatter-stripped
	tests = append(tests, &testCase{
		_id:         "frontmatter-stripped",
		_expected:   "# Demo\n\nBody.\n",
		_expectedOk: true,

		files: []skillgraph.RenderedFile{
			{Content: []byte("payload\n"), Path: "payloads/X.md"},
			{Content: []byte("---\nname: demo\nversion: 1.0\n---\n\n# Demo\n\nBody.\n"), Path: skillgraph.FileManifest},
		},
	})

	// no-frontmatter
	tests = append(tests, &testCase{
		_id:         "no-frontmatter",
		_expected:   "# Demo\n\nBody.\n",
		_expectedOk: true,

		files: []skillgraph.RenderedFile{{Content: []byte("# Demo\n\nBody.\n"), Path: skillgraph.FileManifest}},
	})

	// unterminated-frontmatter
	tests = append(tests, &testCase{
		_id:         "unterminated-frontmatter",
		_expected:   "---\nname: demo\n# Demo\n",
		_expectedOk: true,

		files: []skillgraph.RenderedFile{{Content: []byte("---\nname: demo\n# Demo\n"), Path: skillgraph.FileManifest}},
	})

	// no-manifest
	tests = append(tests, &testCase{
		_id:         "no-manifest",
		_expected:   "",
		_expectedOk: false,

		files: []skillgraph.RenderedFile{{Content: []byte("payload\n"), Path: "payloads/X.md"}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			body, ok := deployedBody(test.files)
			assert.Equal(t, test._expected, body)
			assert.Equal(t, test._expectedOk, ok)
		})
	}
}

// The success and endpoint-failure cases need a Counter on an httptest
// endpoint; tokencount exports no such seam, so only the off path runs here.
func TestCountSkillTokens(t *testing.T) {
	options := &tokencount.Options{CacheDir: t.TempDir(), Mode: tokencount.ModeOff}
	counter, err := tokencount.New(options)
	require.NoError(t, err)
	skill, err := skillgraph.Load(writeModeledSkill(t, t.TempDir(), "demo"))
	require.NoError(t, err)
	skill.Changelog[0].Tokens = 42

	err = countSkillTokens(context.Background(), counter, skill)

	require.ErrorContains(t, err, "countSkillTokens: Counter.CountText: Claude token counting is off")
	assert.Equal(t, 42, skill.Changelog[0].Tokens)
}

func TestExportedSkill_Json(t *testing.T) {
	skill := &skillgraph.Skill{
		Atoms:       map[string]*skillgraph.Atom{"done": {Sort: "state"}},
		Changelog:   []skillgraph.ChangelogEntry{{Date: "2000-01-01", Text: "fixture", Version: "1.0"}},
		Frontmatter: []skillgraph.Field{{Key: "name", Value: "demo"}},
		Vocabulary:  &skillgraph.Vocabulary{Atoms: map[string]*skillgraph.Atom{"mode": {Sort: "mode"}}},
	}
	exported := &exportedSkill{AllAtoms: skill.AllAtoms(), Skill: skill}

	data, err := json.Marshal(exported)
	require.NoError(t, err)

	var document map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &document))
	assert.JSONEq(t, `{"done": {"sort": "state"}, "mode": {"sort": "mode"}}`, string(document["allAtoms"]))
	assert.JSONEq(t, `[{"key": "name", "value": "demo"}]`, string(document["frontmatter"]))
	assert.Contains(t, document, "changelog")
}

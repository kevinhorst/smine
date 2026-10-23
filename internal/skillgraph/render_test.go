package skillgraph

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/sessiontitle"
)

// provideAnsweredSkill is the complete fixture answered by the hook: its rule and its payload, no flow entry.
func provideAnsweredSkill() *Skill {
	skill := provideCompleteSkill()
	skill.AnsweredBy = AnsweredByHook
	flow := flowSection(skill)
	flow.Blocks = []Block{flow.Blocks[0], flow.Blocks[3]}
	skill.Sections = slices.Delete(skill.Sections, 3, 4)
	delete(skill.Atoms, "first_done")
	return skill
}

// renderedPaths lists the paths of rendered files in order.
func renderedPaths(files []RenderedFile) []string {
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.Path)
	}
	return paths
}

// renderedContent returns the content rendered at path, "" when absent.
func renderedContent(files []RenderedFile, path string) string {
	for _, file := range files {
		if file.Path == path {
			return string(file.Content)
		}
	}
	return ""
}

func TestRender(t *testing.T) {
	files, err := Render(provideCompleteSkill())
	require.NoError(t, err)
	assert.Equal(t, []string{"demo.cue", "payloads/SKILL-DEMO-TPL-001.md"}, renderedPaths(files), "files-are-model-and-payloads")
	assert.Equal(t, "## TLDR\n", renderedContent(files, "payloads/SKILL-DEMO-TPL-001.md"), "payload-file-verbatim")
	assert.NotContains(t, renderedContent(files, "demo.cue"), "## TLDR", "payload-body-never-encoded")
}

func TestRenderManifest(t *testing.T) {
	manifest, err := RenderManifest(provideCompleteSkill())
	require.NoError(t, err)
	assert.Equal(t, fixtureMarkdown, string(manifest))

	broken := provideCompleteSkill()
	broken.Changelog[0].Version = "9.9"
	_, err = RenderManifest(broken)
	assert.ErrorContains(t, err, "differs from the newest changelog entry")
}

func TestRenderDeployed_Declarations(t *testing.T) {
	var canonical RenderOptions
	skill := provideTitledSkill()
	skill.Enforce = demoEnforce()
	files, err := RenderDeployed(canonical, skill)
	require.NoError(t, err)
	assert.Equal(t, []string{FileManifest, FileEnforce, sessiontitle.FileName}, renderedPaths(files), "declarations-after-the-manifest")

	smoked, err := RenderDeployed(canonical, provideSmokedSkill())
	require.NoError(t, err)
	assert.Equal(t, []string{FileManifest, FileHelpTest, FileEnforce}, renderedPaths(smoked), "help-test-before-the-declarations")

	plain, err := RenderDeployed(canonical, provideCompleteSkill())
	require.NoError(t, err)
	assert.Equal(t, []string{FileManifest}, renderedPaths(plain), "no-block-no-declaration")

	answered, err := RenderDeployed(canonical, provideAnsweredSkill())
	require.NoError(t, err)
	assert.Equal(t, []string{FileManifest, FileAnswer}, renderedPaths(answered), "answer-md-after-the-declarations")

	twoPayloads := provideAnsweredSkill()
	secondPayload := Block{Entry: &Entry{
		Id:        "SKILL-DEMO-TPL-002",
		Payload:   &Payload{Body: "## Second\n", Info: "markdown", Marker: "```"},
		Role:      RolePayload,
		Statement: "The second template.",
		Title:     "Second template",
	}}
	flowSection(twoPayloads).Blocks = append(flowSection(twoPayloads).Blocks, secondPayload)
	joined, err := RenderDeployed(canonical, twoPayloads)
	require.NoError(t, err)
	assert.Equal(t, "## TLDR\n\n## Second\n", renderedContent(joined, FileAnswer), "answer-joins-payload-bodies")
}

func TestReadManifest(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   string
		_shouldPass bool

		path string
	}

	tests := make([]*testCase, 0)

	// model-path-renders
	modelDir := filepath.Join(t.TempDir(), "skills", "demo")
	require.NoError(t, Save(modelDir, provideCompleteSkill()))
	tests = append(tests, &testCase{
		_id:         "model-path-renders",
		_expected:   fixtureMarkdown,
		_shouldPass: true,

		path: ModelPath(modelDir),
	})

	// markdown-path-passes-through
	markdownPath := filepath.Join(t.TempDir(), FileManifest)
	require.NoError(t, os.WriteFile(markdownPath, []byte("---\nname: fixture\n---\n"), 0o600))
	tests = append(tests, &testCase{
		_id:         "markdown-path-passes-through",
		_expected:   "---\nname: fixture\n---\n",
		_shouldPass: true,

		path: markdownPath,
	})

	// model-not-loading
	brokenDir := filepath.Join(t.TempDir(), "broken")
	require.NoError(t, os.MkdirAll(brokenDir, 0o755))
	require.NoError(t, os.WriteFile(ModelPath(brokenDir), []byte("not cue {\n"), 0o600))
	tests = append(tests, &testCase{
		_id:         "model-not-loading",
		_expected:   "",
		_shouldPass: false,

		path: ModelPath(brokenDir),
	})

	// missing-file
	tests = append(tests, &testCase{
		_id:         "missing-file",
		_expected:   "",
		_shouldPass: false,

		path: filepath.Join(t.TempDir(), FileManifest),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			manifest, err := ReadManifest(test.path)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, string(manifest))
		})
	}
}

func TestManifestLabel(t *testing.T) {
	assert.Equal(t, filepath.Join("skills", "demo", FileManifest), ManifestLabel(filepath.Join("skills", "demo", "demo.cue")), "manifest-label")
	assert.Equal(t, filepath.Join("fixtures", "pass", "x.md"), ManifestLabel(filepath.Join("fixtures", "pass", "x.md")), "other-path")
	assert.Equal(t, filepath.Join("skills", "demo", LegacyModelFile), ManifestLabel(filepath.Join("skills", "demo", LegacyModelFile)), "legacy-path")
}

func TestRender_Refusals(t *testing.T) {
	type testCase struct {
		_id       string
		_contains string

		skill *Skill
	}

	tests := make([]*testCase, 0)

	// check-violation-refused
	skill := provideCompleteSkill()
	skill.Changelog[0].Version = "9.9"
	tests = append(tests, &testCase{_id: "check-violation-refused", _contains: "differs from the newest changelog entry", skill: skill})

	// layout-error-refused
	skill = provideCompleteSkill()
	skill.Sections[0].Blocks[0] = Block{Prose: "### Hidden"}
	tests = append(tests, &testCase{_id: "layout-error-refused", _contains: "is a heading or entry headline", skill: skill})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			files, err := Render(test.skill)
			assert.Nil(t, files)
			assert.ErrorContains(t, err, test._contains)
		})
	}
}

func TestRender_NotesNotRendered(t *testing.T) {
	plain, err := RenderManifest(provideCompleteSkill())
	require.NoError(t, err)

	noted := provideCompleteSkill()
	noted.Note = "why this skill exists"
	flowSection(noted).Note = "a section note\nover two lines"
	flowSection(noted).Blocks[1].Entry.Note = "an entry note"
	notedManifest, err := RenderManifest(noted)
	require.NoError(t, err)
	assert.Equal(t, string(plain), string(notedManifest))

	files, err := Render(noted)
	require.NoError(t, err)
	assert.Equal(t, 4, strings.Count(renderedContent(files, "demo.cue"), "note:"), "three notes plus the pipeline note")

	var canonical RenderOptions
	plainDeployed, err := RenderDeployed(canonical, provideCompleteSkill())
	require.NoError(t, err)

	notedDeployed, err := RenderDeployed(canonical, noted)
	require.NoError(t, err)
	assert.Equal(t, renderedContent(plainDeployed, FileManifest), renderedContent(notedDeployed, FileManifest))
}

func TestRenderDeployed(t *testing.T) {
	type testCase struct {
		_id         string
		_paths      []string
		_shouldPass bool

		options RenderOptions
		skill   *Skill
	}

	tests := make([]*testCase, 0)

	// skill-md-only
	options := RenderOptions{HasEval: false}
	tests = append(tests, &testCase{_id: "skill-md-only", _paths: []string{FileManifest}, _shouldPass: true, options: options, skill: provideCompleteSkill()})

	// help-test-from-smoke
	options = RenderOptions{HasEval: false}
	tests = append(tests, &testCase{_id: "help-test-from-smoke", _paths: []string{FileManifest, FileHelpTest, FileEnforce}, _shouldPass: true, options: options, skill: provideSmokedSkill()})

	// help-test-from-eval
	options = RenderOptions{HasEval: true}
	tests = append(tests, &testCase{_id: "help-test-from-eval", _paths: []string{FileManifest, FileHelpTest}, _shouldPass: true, options: options, skill: provideCompleteSkill()})

	// unmatched-disable-token
	options = RenderOptions{Disable: []string{"SKILL-NOPE-001"}}
	tests = append(tests, &testCase{_id: "unmatched-disable-token", _paths: []string{}, _shouldPass: false, options: options, skill: provideCompleteSkill()})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			files, err := RenderDeployed(test.options, test.skill)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._paths, renderedPaths(files))
		})
	}
}

func TestRender_SmokeBlock(t *testing.T) {
	files, err := Render(provideSmokedSkill())

	require.NoError(t, err)
	assert.Equal(t, []string{"demo.cue", "payloads/SKILL-DEMO-TPL-001.md"}, renderedPaths(files), "no-smoke-file-rendered")
	assert.Contains(t, renderedContent(files, "demo.cue"), "smoke: {", "block-encoded")
}

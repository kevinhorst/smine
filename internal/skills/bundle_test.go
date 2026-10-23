package skills

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/components"
	"github.com/kevinhorst/smine/internal/skillgraph"
)

// provideBundleRoot is a deploy root with two leaves: base declares
// acdsl-context, extra declares nothing; neither sets a title state, whose
// set command would be a smine reference.
func provideBundleRoot(t *testing.T) string {
	t.Helper()
	root := provideAcdslContextRoot(t)
	base := provideBundleSkill("base")
	acdslContext := skillgraph.Field{Key: "acdsl-context", Value: " RULE-*, FACT-*"}
	base.Frontmatter = append(base.Frontmatter, acdslContext)
	require.NoError(t, skillgraph.Save(filepath.Join(root, "repo", "skills", "group", "base"), base))
	require.NoError(t, skillgraph.Save(filepath.Join(root, "repo", "skills", "group", "extra"), provideBundleSkill("extra")))
	return root
}

// provideBundleSkill is the deploy fixture without its title declaration, the
// rules variable and the `title set` step text the declaration requires.
func provideBundleSkill(name string) *skillgraph.Skill {
	skill := provideDeploySkill(name)
	skill.Env = nil
	skill.Title = nil
	for _, section := range skill.Sections {
		for _, block := range section.Blocks {
			if block.Entry != nil && block.Entry.Title == "Second step" {
				block.Entry.Statement = "RUN the second step."
			}
		}
	}
	return skill
}

// provideBundleCheckSkill is the bundle fixture whose first step runs the required repo-scoped check command.
func provideBundleCheckSkill(name string) *skillgraph.Skill {
	skill := provideBundleSkill(name)
	skill.Env = map[string]*skillgraph.Variable{"CHECK": {Interface: "check@1", Required: true}}
	skill.Sections[1].Blocks[0].Entry.Statement = "RUN `$" + skillgraph.VariableName(name, "CHECK") + "` first."
	return skill
}

// saveBundleSkill overwrites one leaf of the root with skill.
func saveBundleSkill(t *testing.T, root string, skill *skillgraph.Skill) {
	t.Helper()
	require.NoError(t, skillgraph.Save(filepath.Join(root, "repo", "skills", "group", skill.Name()), skill))
}

func TestCheckBundle(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   []string
		_shouldPass bool

		root string
	}

	tests := make([]*testCase, 0)

	// clean-tree-no-hits
	root := provideBundleRoot(t)
	tests = append(tests, &testCase{
		_id:         "clean-tree-no-hits",
		_expected:   nil,
		_shouldPass: true,

		root: root,
	})

	// documented-make-target-hit
	root = provideBundleRoot(t)
	writeDeployFile(t, root, "repo/Makefile", "## audit: the gate\naudit:\n\tgo test ./...\n", 0o644)
	referencing := provideBundleSkill("extra")
	referencing.Sections[1].Blocks[0].Entry.Statement = "RUN make audit, then make up."
	saveBundleSkill(t, root, referencing)
	tests = append(tests, &testCase{
		_id:         "documented-make-target-hit",
		_expected:   []string{"extra/SKILL.md:31: make audit"},
		_shouldPass: true,

		root: root,
	})

	// smine-literal-hit
	root = provideBundleRoot(t)
	writeDeployFile(t, root, "repo/skills/group/extra/notes.md", "intro\nrun smine-rules check\n", 0o644)
	tests = append(tests, &testCase{
		_id:         "smine-literal-hit",
		_expected:   []string{"extra/notes.md:2: smine-rules"},
		_shouldPass: true,

		root: root,
	})

	// unfilled-reference-hit
	root = provideBundleRoot(t)
	writeDeployFile(t, root, "repo/skills/group/extra/notes.md", "run $SKILL_EXTRA_RULES\n", 0o644)
	tests = append(tests, &testCase{
		_id:         "unfilled-reference-hit",
		_expected:   []string{"extra/notes.md:1: $SKILL_EXTRA_RULES"},
		_shouldPass: true,

		root: root,
	})

	// unconfigured-and-requiring-not-scanned
	root = provideBundleRoot(t)
	saveRequiredSkill(t, root, "needy")
	writeDeployFile(t, root, "repo/skills/group/needy/notes.md", "make audit and smine-heartbeat\n", 0o644)
	writeDeployFile(t, root, "repo/Makefile", "## audit: the gate\n", 0o644)
	requiring := provideBundleSkill("extra")
	requiring.Requires = []components.ID{components.Peek}
	saveBundleSkill(t, root, requiring)
	writeDeployFile(t, root, "repo/skills/group/extra/notes.md", "smine-heartbeat\n", 0o644)
	tests = append(tests, &testCase{
		_id:         "unconfigured-and-requiring-not-scanned",
		_expected:   nil,
		_shouldPass: true,

		root: root,
	})

	// placeholder-passes-scan
	root = provideBundleRoot(t)
	saveBundleSkill(t, root, provideBundleCheckSkill("checky"))
	tests = append(tests, &testCase{
		_id:         "placeholder-passes-scan",
		_expected:   nil,
		_shouldPass: true,

		root: root,
	})

	// repo-scoped-required-bundled
	root = provideBundleRoot(t)
	saveBundleSkill(t, root, provideBundleCheckSkill("checky"))
	writeDeployFile(t, root, "repo/skills/group/checky/notes.md", "run smine-rules check\n", 0o644)
	tests = append(tests, &testCase{
		_id:         "repo-scoped-required-bundled",
		_expected:   []string{"checky/notes.md:1: smine-rules"},
		_shouldPass: true,

		root: root,
	})

	// install-scoped-required-skipped
	root = provideBundleRoot(t)
	saveRequiredSkill(t, root, "needy")
	writeDeployFile(t, root, "repo/skills/group/needy/notes.md", "run smine-rules check\n", 0o644)
	tests = append(tests, &testCase{
		_id:         "install-scoped-required-skipped",
		_expected:   nil,
		_shouldPass: true,

		root: root,
	})

	// broken-model-fails
	root = provideBundleRoot(t)
	writeDeployFile(t, root, "repo/skills/group/broken/broken.cue", "not cue {\n", 0o644)
	tests = append(tests, &testCase{
		_id:         "broken-model-fails",
		_expected:   nil,
		_shouldPass: false,

		root: root,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			hits, err := CheckBundle(filepath.Join(test.root, "repo"))

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, hits)
		})
	}
}

func TestBundle(t *testing.T) {
	type testCase struct {
		_id               string
		_expectedError    string
		_expectedFiles    map[string]bool
		_expectedReport   string
		_expectedSnippets map[string][]string
		_isReferenceError bool
		_shouldPass       bool

		outDir string
		root   string
	}

	tests := make([]*testCase, 0)

	// writes-bundleable-leaves
	root := provideBundleRoot(t)
	tests = append(tests, &testCase{
		_id: "writes-bundleable-leaves",
		_expectedFiles: map[string]bool{
			"dist/base/SKILL.md":  true,
			"dist/base/notes.md":  true,
			"dist/extra/SKILL.md": true,
		},
		_expectedReport: "  base -> " + filepath.Join(root, "dist", "base"),
		_shouldPass:     true,

		outDir: filepath.Join(root, "dist"),
		root:   root,
	})

	// skips-requires
	root = provideBundleRoot(t)
	requiring := provideBundleSkill("extra")
	requiring.Requires = []components.ID{components.Peek}
	saveBundleSkill(t, root, requiring)
	tests = append(tests, &testCase{
		_id:             "skips-requires",
		_expectedFiles:  map[string]bool{"dist/extra": false, "dist/base": true},
		_expectedReport: "  skip extra (requires peek)",
		_shouldPass:     true,

		outDir: filepath.Join(root, "dist"),
		root:   root,
	})

	// skips-license
	root = provideBundleRoot(t)
	licensed := provideBundleSkill("extra")
	license := skillgraph.Field{Key: fieldLicense, Value: " Proprietary"}
	licensed.Frontmatter = append(licensed.Frontmatter, license)
	saveBundleSkill(t, root, licensed)
	tests = append(tests, &testCase{
		_id:             "skips-license",
		_expectedFiles:  map[string]bool{"dist/extra": false},
		_expectedReport: "  skip extra (license)",
		_shouldPass:     true,

		outDir: filepath.Join(root, "dist"),
		root:   root,
	})

	// acdsl-context-kept
	root = provideBundleRoot(t)
	tests = append(tests, &testCase{
		_id: "acdsl-context-kept",
		_expectedSnippets: map[string][]string{
			"dist/base/SKILL.md": {"\nacdsl-context: RULE-*, FACT-*\n"},
		},
		_shouldPass: true,

		outDir: filepath.Join(root, "dist"),
		root:   root,
	})

	// no-marker
	root = provideBundleRoot(t)
	tests = append(tests, &testCase{
		_id:            "no-marker",
		_expectedFiles: map[string]bool{"dist/base/" + markerFileName: false},
		_shouldPass:    true,

		outDir: filepath.Join(root, "dist"),
		root:   root,
	})

	// agents-not-included
	root = provideBundleRoot(t)
	writeDeployFile(t, root, "repo/skills/group/base/agents/helper.md", "helper agent\n", 0o644)
	tests = append(tests, &testCase{
		_id:            "agents-not-included",
		_expectedFiles: map[string]bool{"dist/base/agents": false, "dist/agents": false},
		_shouldPass:    true,

		outDir: filepath.Join(root, "dist"),
		root:   root,
	})

	// non-empty-dir-refused
	root = provideBundleRoot(t)
	writeDeployFile(t, root, "dist/leftover.md", "old\n", 0o644)
	tests = append(tests, &testCase{
		_id:            "non-empty-dir-refused",
		_expectedError: "is not empty — remove it first",
		_expectedFiles: map[string]bool{"dist/base": false},
		_shouldPass:    false,

		outDir: filepath.Join(root, "dist"),
		root:   root,
	})

	// missing-dir-created
	root = provideBundleRoot(t)
	tests = append(tests, &testCase{
		_id:            "missing-dir-created",
		_expectedFiles: map[string]bool{"out/nested/base/SKILL.md": true},
		_shouldPass:    true,

		outDir: filepath.Join(root, "out", "nested"),
		root:   root,
	})

	// reference-in-entry-fails
	root = provideBundleRoot(t)
	writeDeployFile(t, root, "repo/Makefile", "## audit: the gate\n", 0o644)
	referencing := provideBundleSkill("extra")
	referencing.Sections[1].Blocks[0].Entry.Statement = "RUN make audit."
	saveBundleSkill(t, root, referencing)
	tests = append(tests, &testCase{
		_id:               "reference-in-entry-fails",
		_expectedError:    "extra/SKILL.md:",
		_isReferenceError: true,
		_shouldPass:       false,

		outDir: filepath.Join(root, "dist"),
		root:   root,
	})

	// reference-in-sibling-file-fails
	root = provideBundleRoot(t)
	writeDeployFile(t, root, "repo/skills/group/extra/README.md", "intro\nrun toolpolicy allow\n", 0o644)
	tests = append(tests, &testCase{
		_id:               "reference-in-sibling-file-fails",
		_expectedError:    "extra/README.md:2: toolpolicy",
		_isReferenceError: true,
		_shouldPass:       false,

		outDir: filepath.Join(root, "dist"),
		root:   root,
	})

	// hit-names-path-line-term
	root = provideBundleRoot(t)
	writeDeployFile(t, root, "repo/Makefile", "## audit: the gate\naudit:\n", 0o644)
	writeDeployFile(t, root, "repo/skills/group/extra/notes.md", "a\nb\nrun smine-heartbeat status, then make audit\n", 0o644)
	tests = append(tests, &testCase{
		_id:               "hit-names-path-line-term",
		_expectedError:    "2 hit(s): bundle carries smine references\nextra/notes.md:3: smine-heartbeat\nextra/notes.md:3: make audit",
		_isReferenceError: true,
		_shouldPass:       false,

		outDir: filepath.Join(root, "dist"),
		root:   root,
	})

	// author-path-and-peek-pass
	root = provideBundleRoot(t)
	writeDeployFile(t, root, "repo/skills/group/extra/notes.md", "see ~/GolandProjects/x and mcp__peek-mcp__session_get\n", 0o644)
	tests = append(tests, &testCase{
		_id:            "author-path-and-peek-pass",
		_expectedFiles: map[string]bool{"dist/extra/notes.md": true},
		_shouldPass:    true,

		outDir: filepath.Join(root, "dist"),
		root:   root,
	})

	// unconfigured-skipped
	root = provideBundleRoot(t)
	saveRequiredSkill(t, root, "needy")
	tests = append(tests, &testCase{
		_id:             "unconfigured-skipped",
		_expectedFiles:  map[string]bool{"dist/needy": false, "dist/base": true},
		_expectedReport: "  skip needy unconfigured (SKILL_NEEDY_TITLE_SET)",
		_shouldPass:     true,

		outDir: filepath.Join(root, "dist"),
		root:   root,
	})

	// clean-bundle-passes
	root = provideBundleRoot(t)
	tests = append(tests, &testCase{
		_id:            "clean-bundle-passes",
		_expectedFiles: map[string]bool{"dist/base/SKILL.md": true},
		_shouldPass:    true,

		outDir: filepath.Join(root, "dist"),
		root:   root,
	})

	// repo-scoped-required-bundled
	root = provideBundleRoot(t)
	saveBundleSkill(t, root, provideBundleCheckSkill("checky"))
	tests = append(tests, &testCase{
		_id: "repo-scoped-required-bundled",
		_expectedSnippets: map[string][]string{
			"dist/checky/SKILL.md": {`if (ENVIRON["SKILL_CHECKY_CHECK"] == "") { print "SKILL_CHECKY_CHECK: not set - runs everything before a commit, e.g. go vet ./... - export it in your shell"; exit 2 }; `},
		},
		_shouldPass: true,

		outDir: filepath.Join(root, "dist"),
		root:   root,
	})

	// index-written
	root = provideBundleRoot(t)
	tests = append(tests, &testCase{
		_id:             "index-written",
		_expectedReport: "  index -> " + filepath.Join(root, "dist", "README.md"),
		_expectedSnippets: map[string][]string{
			"dist/README.md": {"# Skills\n", "\n- base\n", "\n- extra\n"},
		},
		_shouldPass: true,

		outDir: filepath.Join(root, "dist"),
		root:   root,
	})

	// index-names-skipped-capability
	root = provideBundleRoot(t)
	saveRequiredSkill(t, root, "needy")
	requiring = provideBundleSkill("extra")
	requiring.Requires = []components.ID{components.Peek}
	saveBundleSkill(t, root, requiring)
	tests = append(tests, &testCase{
		_id: "index-names-skipped-capability",
		_expectedSnippets: map[string][]string{
			"dist/README.md": {
				"\n- extra (skipped): needs the peek component\n",
				"\n- needy (skipped): needs title-set@1 from the config-server component\n",
			},
		},
		_shouldPass: true,

		outDir: filepath.Join(root, "dist"),
		root:   root,
	})

	// index-names-full-install-adds
	root = provideBundleRoot(t)
	saveBundleSkill(t, root, provideDeploySkill("extra"))
	tests = append(tests, &testCase{
		_id: "index-names-full-install-adds",
		_expectedSnippets: map[string][]string{
			"dist/README.md": {"\n- extra: with the full install also sets the title state\n"},
		},
		_shouldPass: true,

		outDir: filepath.Join(root, "dist"),
		root:   root,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			report, err := Bundle(test.outDir, filepath.Join(test.root, "repo"))

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._isReferenceError, errors.Is(err, ErrBundleReferences))
			assert.Contains(t, fmt.Sprint(err), test._expectedError)
			assert.Contains(t, report, test._expectedReport)
			assert.Equal(t, test._expectedFiles, existingFiles(test.root, test._expectedFiles))
			assert.Empty(t, missingSnippets(test.root, test._expectedSnippets))
		})
	}
}

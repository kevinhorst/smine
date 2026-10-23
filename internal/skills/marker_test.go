package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarkerContent(t *testing.T) {
	root := provideDeployRoot(t, "base")
	leafDir := filepath.Join(root, "repo", "skills", "group", "base")
	first := &deployedFile{content: []byte("a\n"), path: "a.md"}
	second := &deployedFile{content: []byte("b\n"), path: "sub/b.md"}
	edited := &deployedFile{content: []byte("edited\n"), path: "a.md"}
	inOrder := []*deployedFile{first, second}
	reversed := []*deployedFile{second, first}
	withEdit := []*deployedFile{edited, second}

	marker, err := markerContent(leafDir, inOrder)
	require.NoError(t, err)
	source, err := sourceHash(leafDir)
	require.NoError(t, err)

	assert.Equal(t, markerSourcePrefix+source+"\n"+markerRenderedPrefix+renderedHash(inOrder)+"\n", string(marker), "two-lines")
	assert.Equal(t, 2, strings.Count(string(marker), "\n"), "two-lines-only")
	assert.Equal(t, renderedHash(inOrder), renderedHash(reversed), "order-independent")
	assert.NotEqual(t, renderedHash(inOrder), renderedHash(withEdit), "content-dependent")
}

func TestIsLocallyModified(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   bool
		_shouldPass bool

		dir    string
		wanted string
	}

	tests := make([]*testCase, 0)

	// fresh-deploy-unmodified
	root := provideDeployRoot(t, "base")
	syncFixture(t, deployOptions(root))
	tests = append(tests, &testCase{
		_id:         "fresh-deploy-unmodified",
		_expected:   false,
		_shouldPass: true,

		dir: filepath.Join(root, "home", ".claude", "skills", "base"),
	})

	// edited-file-modified
	root = provideDeployRoot(t, "base")
	syncFixture(t, deployOptions(root))
	writeDeployFile(t, root, "home/.claude/skills/base/notes.md", "edited\n", 0o644)
	tests = append(tests, &testCase{
		_id:         "edited-file-modified",
		_expected:   true,
		_shouldPass: true,

		dir: filepath.Join(root, "home", ".claude", "skills", "base"),
	})

	// added-file-modified
	root = provideDeployRoot(t, "base")
	syncFixture(t, deployOptions(root))
	writeDeployFile(t, root, "home/.claude/skills/base/extra.md", "extra\n", 0o644)
	tests = append(tests, &testCase{
		_id:         "added-file-modified",
		_expected:   true,
		_shouldPass: true,

		dir: filepath.Join(root, "home", ".claude", "skills", "base"),
	})

	// marker-without-rendered-line-equal-render-unmodified
	root = provideDeployRoot(t, "base")
	syncFixture(t, deployOptions(root))
	markerPath := filepath.Join(root, "home", ".claude", "skills", "base", markerFileName)
	marker, err := os.ReadFile(markerPath)
	require.NoError(t, err)
	sourceLine, _, _ := strings.Cut(string(marker), "\n")
	writeDeployFile(t, root, "home/.claude/skills/base/"+markerFileName, sourceLine+"\n", 0o644)
	legacyDir := filepath.Join(root, "home", ".claude", "skills", "base")
	legacyHash, err := deployedHash(legacyDir)
	require.NoError(t, err)
	tests = append(tests, &testCase{
		_id:         "marker-without-rendered-line-equal-render-unmodified",
		_expected:   false,
		_shouldPass: true,

		dir:    legacyDir,
		wanted: legacyHash,
	})

	// marker-without-rendered-line-differing-render-modified
	tests = append(tests, &testCase{
		_id:         "marker-without-rendered-line-differing-render-modified",
		_expected:   true,
		_shouldPass: true,

		dir:    legacyDir,
		wanted: "another render",
	})

	// marker-missing-fails
	root = provideDeployRoot(t, "base")
	writeDeployFile(t, root, "home/.claude/skills/base/notes.md", "mine\n", 0o644)
	tests = append(tests, &testCase{
		_id:         "marker-missing-fails",
		_expected:   false,
		_shouldPass: false,

		dir: filepath.Join(root, "home", ".claude", "skills", "base"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			isModified, err := isLocallyModified(test.dir, test.wanted)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, isModified)
		})
	}
}

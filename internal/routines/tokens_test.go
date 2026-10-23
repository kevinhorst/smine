package routines

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadToken(t *testing.T) {
	type testCase struct {
		_id            string
		_expectedToken string
		_shouldPass    bool

		label string
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	tokenDir := filepath.Join(home, ".config", "claude-routine", "tokens")
	require.NoError(t, os.MkdirAll(tokenDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(tokenDir, "work"), []byte(" labeled-value\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tokenDir, "empty"), []byte("\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".config", "claude-routine", "token"), []byte("legacy-value"), 0o600))

	tests := make([]*testCase, 0)

	// labeled
	tests = append(tests, &testCase{
		_id:            "labeled",
		_expectedToken: "labeled-value",
		_shouldPass:    true,

		label: "work",
	})

	// empty-label-legacy
	tests = append(tests, &testCase{
		_id:            "empty-label-legacy",
		_expectedToken: "legacy-value",
		_shouldPass:    true,

		label: "",
	})

	// unknown-label
	tests = append(tests, &testCase{
		_id:            "unknown-label",
		_expectedToken: "",
		_shouldPass:    false,

		label: "nope",
	})

	// empty-file
	tests = append(tests, &testCase{
		_id:            "empty-file",
		_expectedToken: "",
		_shouldPass:    false,

		label: "empty",
	})

	// raw-token-rejected
	tests = append(tests, &testCase{
		_id:            "raw-token-rejected",
		_expectedToken: "",
		_shouldPass:    false,

		label: "sk-ant-oat01-abc",
	})

	// path-traversal-rejected
	tests = append(tests, &testCase{
		_id:            "path-traversal-rejected",
		_expectedToken: "",
		_shouldPass:    false,

		label: "../token",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			token, err := ReadToken(test.label)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedToken, token)
		})
	}
}

func TestListTokenLabelsMissingDir(t *testing.T) {
	labels, err := ListTokenLabels(filepath.Join(t.TempDir(), "absent"))
	require.NoError(t, err)
	assert.Nil(t, labels)
}

func TestListTokenLabelsEmptyDir(t *testing.T) {
	labels, err := ListTokenLabels(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, labels)
}

func TestListTokenLabelsSortedSkipsNonFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "work"), []byte("t1"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "personal"), []byte("t2"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte("junk"), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), 0o755))

	labels, err := ListTokenLabels(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"personal", "work"}, labels)
}

func TestSaveToken(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool
		label       string
		value       string
	}

	expectedFileMode := os.FileMode(0o600)
	expectedDirMode := os.FileMode(0o700)
	if runtime.GOOS == "windows" {
		expectedFileMode = 0o666
		expectedDirMode = 0o777
	}

	tests := make([]*testCase, 0)

	// happy-path
	tests = append(tests, &testCase{
		_id:         "happy-path",
		_shouldPass: true,
		label:       "work",
		value:       "sk-ant-oat01-abc",
	})

	// whitespace-stripped-value
	tests = append(tests, &testCase{
		_id:         "whitespace-stripped-value",
		_shouldPass: true,
		label:       "wrapped",
		value:       " tok\nen\t",
	})

	// whitespace-only-value
	tests = append(tests, &testCase{
		_id:         "whitespace-only-value",
		_shouldPass: false,
		label:       "blank",
		value:       " \n\t ",
	})

	// empty-label
	tests = append(tests, &testCase{
		_id:         "empty-label",
		_shouldPass: false,
		label:       "",
		value:       "token",
	})

	// label-with-slash
	tests = append(tests, &testCase{
		_id:         "label-with-slash",
		_shouldPass: false,
		label:       "a/b",
		value:       "token",
	})

	// label-dot-dot
	tests = append(tests, &testCase{
		_id:         "label-dot-dot",
		_shouldPass: false,
		label:       "..",
		value:       "token",
	})

	// label-leading-dot
	tests = append(tests, &testCase{
		_id:         "label-leading-dot",
		_shouldPass: false,
		label:       ".hidden",
		value:       "token",
	})

	// label-leading-dash
	tests = append(tests, &testCase{
		_id:         "label-leading-dash",
		_shouldPass: false,
		label:       "-flag",
		value:       "token",
	})

	// label-too-long
	tests = append(tests, &testCase{
		_id:         "label-too-long",
		_shouldPass: false,
		label:       strings65(),
		value:       "token",
	})

	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "tokens")
			err := SaveToken(test.label, test.value, dir)
			if !test._shouldPass {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			info, err := os.Stat(filepath.Join(dir, test.label))
			require.NoError(t, err)
			assert.Equal(t, expectedFileMode, info.Mode().Perm())

			dirInfo, err := os.Stat(dir)
			require.NoError(t, err)
			assert.Equal(t, expectedDirMode, dirInfo.Mode().Perm())

			content, err := os.ReadFile(filepath.Join(dir, test.label))
			require.NoError(t, err)
			assert.NotContains(t, string(content), " ")
			assert.NotContains(t, string(content), "\n")
			assert.NotEmpty(t, content)
		})
	}
}

func TestSaveTokenDuplicateLabelKeepsOriginal(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, SaveToken("work", "original", dir))

	err := SaveToken("work", "replacement", dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	content, err := os.ReadFile(filepath.Join(dir, "work"))
	require.NoError(t, err)
	assert.Equal(t, "original", string(content))
}

func strings65() string {
	label := make([]byte, 65)
	for index := range label {
		label[index] = 'a'
	}
	return string(label)
}

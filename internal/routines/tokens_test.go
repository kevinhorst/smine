package routines

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

			dirInfo, err := os.Stat(dir)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())

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

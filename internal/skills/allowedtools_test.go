package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllowedTools(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		content string
	}

	tests := make([]*testCase, 0)

	// present
	tests = append(tests, &testCase{
		_id:       "present",
		_expected: "Read, Bash(jq *)",

		content: "---\nname: demo\nallowed-tools: Read, Bash(jq *)\n---\nbody\n",
	})

	// absent-field
	tests = append(tests, &testCase{
		_id:       "absent-field",
		_expected: "",

		content: "---\nname: demo\n---\nallowed-tools: not frontmatter\n",
	})

	// missing-skill
	tests = append(tests, &testCase{
		_id:       "missing-skill",
		_expected: "",

		content: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			dir := t.TempDir()
			if test.content != "" {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(test.content), 0o644))
			}

			assert.Equal(t, test._expected, AllowedTools(dir))
		})
	}
}

package skills

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrphanRows(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedEnv     *Rows
		_expectedOrphans []string

		env *Rows
	}

	repoDir := filepath.Join(provideDeployRoot(t, "base"), "repo")
	tests := make([]*testCase, 0)

	// orphan-skill-row-listed
	tests = append(tests, &testCase{
		_id:              "orphan-skill-row-listed",
		_expectedEnv:     &Rows{Providers: map[string]string{}, Skills: map[string]string{"SKILL_RAILROADREVIEW_BUILD": "go build ./..."}},
		_expectedOrphans: []string{"SKILL_RAILROADREVIEW_BUILD"},

		env: &Rows{Providers: map[string]string{}, Skills: map[string]string{"SKILL_RAILROADREVIEW_BUILD": "go build ./..."}},
	})

	// orphan-provider-row-listed
	tests = append(tests, &testCase{
		_id:              "orphan-provider-row-listed",
		_expectedEnv:     &Rows{Providers: map[string]string{"PROVIDER_GONE": "gone"}, Skills: map[string]string{}},
		_expectedOrphans: []string{"PROVIDER_GONE"},

		env: &Rows{Providers: map[string]string{"PROVIDER_GONE": "gone"}, Skills: map[string]string{}},
	})

	// declared-rows-not-listed
	tests = append(tests, &testCase{
		_id: "declared-rows-not-listed",
		_expectedEnv: &Rows{
			Providers: map[string]string{"PROVIDER_SMINE_RULES": "my-rules"},
			Skills:    map[string]string{"SKILL_BASE_TITLE_SET": "my-rules"},
		},
		_expectedOrphans: nil,

		env: &Rows{
			Providers: map[string]string{"PROVIDER_SMINE_RULES": "my-rules"},
			Skills:    map[string]string{"SKILL_BASE_TITLE_SET": "my-rules"},
		},
	})

	// empty-env
	tests = append(tests, &testCase{
		_id:              "empty-env",
		_expectedEnv:     &Rows{},
		_expectedOrphans: nil,

		env: &Rows{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			orphans, err := OrphanRows(repoDir, test.env)

			require.NoError(t, err)
			assert.Equal(t, test._expectedOrphans, orphans)
			assert.Equal(t, test._expectedEnv, test.env)
		})
	}
}

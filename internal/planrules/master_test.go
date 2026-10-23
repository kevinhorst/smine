package planrules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeMasterFixture(t *testing.T, root string, relative string, argsLine string) string {
	t.Helper()
	path := filepath.Join(root, relative)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("# Title\n\n"+argsLine+"\n\n## TLDR\n"), 0o644))
	return path
}

func TestDerivedPlans(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		masterFile string
	}

	root := t.TempDir()
	feature := "plans/2026-10-01-whole-c627d20"
	masterLink := "master: [whole](../../2026-10-01-whole-wip-c627d20/design/whole.md)"
	master := writeMasterFixture(t, root, feature+"/2026-10-01-whole-wip-c627d20/design/whole.md", "role: `master`, target: `opus`")
	git := writeMasterFixture(t, root, feature+"/2026-10-02-whole-git-wip-f18e3b9/design/whole-git.md", "role: `derived`, "+masterLink+", target: `opus`")
	docs := writeMasterFixture(t, root, feature+"/2026-10-02-whole-docs-wip-617c74d/design/whole-docs.md", "role: `derived`, "+masterLink)
	writeMasterFixture(t, root, feature+"/2026-10-02-whole-loose-wip-617c74d/design/whole-loose.md", "target: `opus`")
	writeMasterFixture(t, root, feature+"/2026-10-02-whole-foreign-wip-617c74d/design/whole-foreign.md", "role: `derived`, master: [other](../../2026-10-01-other-wip-c627d20/design/other.md)")
	plain := writeMasterFixture(t, root, "plans/2026-10-01-plain-c627d20/2026-10-01-plain-wip-c627d20/design/plain.md", "target: `opus`")

	tests := make([]*testCase, 0)

	// master-lists-linked-derived
	tests = append(tests, &testCase{
		_id:       "master-lists-linked-derived",
		_expected: []string{docs, git},

		masterFile: master,
	})

	// derived-is-no-master
	tests = append(tests, &testCase{
		_id:       "derived-is-no-master",
		_expected: nil,

		masterFile: git,
	})

	// plain-plan-is-no-master
	tests = append(tests, &testCase{
		_id:       "plain-plan-is-no-master",
		_expected: nil,

		masterFile: plain,
	})

	// missing-file-is-no-master
	tests = append(tests, &testCase{
		_id:       "missing-file-is-no-master",
		_expected: nil,

		masterFile: filepath.Join(root, "missing.md"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, DerivedPlans(test.masterFile))
		})
	}
}

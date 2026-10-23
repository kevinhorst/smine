package skillenv

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kevinhorst/smine/internal/repos"
)

func provideRegistry(t *testing.T, rows ...repos.Repo) *repos.Registry {
	t.Helper()
	registry := repos.NewRegistry(filepath.Join(t.TempDir(), "repos.json"))
	for _, row := range rows {
		require.NoError(t, registry.Add(row))
	}
	require.NoError(t, registry.Reload())
	return registry
}

func TestResolve(t *testing.T) {
	type testCase struct {
		_id       string
		_expected map[string]Value

		bindings map[string]string
		cwd      string
		registry *repos.Registry
		rows     *Rows
	}

	base := t.TempDir()
	main := filepath.Join(base, "main")
	bare := filepath.Join(base, "bare")
	unregistered := filepath.Join(base, "unregistered")
	worktree := filepath.Join(main, ".claude", "worktrees", "x")
	worktreeGitDir := filepath.Join(main, ".git", "worktrees", "x")
	for _, dir := range []string{worktreeGitDir, filepath.Join(bare, ".git"), filepath.Join(worktree, "cmd"), unregistered} {
		require.NoError(t, os.MkdirAll(dir, 0o755))
	}
	pointer := "gitdir: " + worktreeGitDir + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(worktree, ".git"), []byte(pointer), 0o644))

	mainEnv := map[string]string{"SKILL_FIMPLEMENT_CHECK": "make test"}
	mainRow := repos.Repo{Env: mainEnv, Name: "main", Path: main}
	bareRow := repos.Repo{Name: "bare", Path: bare}
	registry := provideRegistry(t, mainRow, bareRow)

	variables := map[string]string{
		"SKILL_FIMPLEMENT_CHECK":     "make audit",
		"SKILL_RAILROADREVIEW_BUILD": "go build ./...",
	}
	rows := &Rows{Variables: variables}
	defaults := map[string]Value{
		"SKILL_FIMPLEMENT_CHECK":     {Source: SourceSkillDefault, Value: "make audit"},
		"SKILL_RAILROADREVIEW_BUILD": {Source: SourceSkillDefault, Value: "go build ./..."},
	}
	rowOverDefault := map[string]Value{
		"SKILL_FIMPLEMENT_CHECK":     {Repo: "main", Source: SourceRepo, Value: "make test"},
		"SKILL_RAILROADREVIEW_BUILD": {Source: SourceSkillDefault, Value: "go build ./..."},
	}
	checkBinding := map[string]string{"SKILL_FIMPLEMENT_CHECK": "check"}
	globalCheck := &Rows{Defaults: map[string]string{"check": "go vet ./..."}}

	tests := make([]*testCase, 0)

	// row-over-default
	tests = append(tests, &testCase{
		_id:       "row-over-default",
		_expected: rowOverDefault,

		bindings: nil,
		cwd:      main,
		registry: registry,
		rows:     rows,
	})

	// default-without-row
	tests = append(tests, &testCase{
		_id:       "default-without-row",
		_expected: defaults,

		bindings: nil,
		cwd:      bare,
		registry: registry,
		rows:     rows,
	})

	// unregistered-cwd-defaults-only
	tests = append(tests, &testCase{
		_id:       "unregistered-cwd-defaults-only",
		_expected: defaults,

		bindings: nil,
		cwd:      unregistered,
		registry: registry,
		rows:     rows,
	})

	// worktree-cwd-resolves-main
	tests = append(tests, &testCase{
		_id:       "worktree-cwd-resolves-main",
		_expected: rowOverDefault,

		bindings: nil,
		cwd:      filepath.Join(worktree, "cmd"),
		registry: registry,
		rows:     rows,
	})

	// nil-inputs-empty
	tests = append(tests, &testCase{
		_id:       "nil-inputs-empty",
		_expected: map[string]Value{},

		bindings: nil,
		cwd:      main,
		registry: nil,
		rows:     nil,
	})

	// empty-row-value-ignored
	emptyRowEnv := &Rows{Variables: map[string]string{
		"SKILL_FIMPLEMENT_CHECK":     "",
		"SKILL_RAILROADREVIEW_BUILD": "go build ./...",
	}}
	tests = append(tests, &testCase{
		_id:       "empty-row-value-ignored",
		_expected: map[string]Value{"SKILL_RAILROADREVIEW_BUILD": {Source: SourceSkillDefault, Value: "go build ./..."}},

		bindings: nil,
		cwd:      unregistered,
		registry: registry,
		rows:     emptyRowEnv,
	})

	// global-default-through-binding
	tests = append(tests, &testCase{
		_id:       "global-default-through-binding",
		_expected: map[string]Value{"SKILL_FIMPLEMENT_CHECK": {Source: SourceGlobalDefault, Value: "go vet ./..."}},

		bindings: checkBinding,
		cwd:      unregistered,
		registry: registry,
		rows:     globalCheck,
	})

	// skill-default-over-global
	skillOverGlobal := &Rows{
		Defaults:  map[string]string{"check": "go vet ./..."},
		Variables: map[string]string{"SKILL_FIMPLEMENT_CHECK": "make audit"},
	}
	tests = append(tests, &testCase{
		_id:       "skill-default-over-global",
		_expected: map[string]Value{"SKILL_FIMPLEMENT_CHECK": {Source: SourceSkillDefault, Value: "make audit"}},

		bindings: checkBinding,
		cwd:      unregistered,
		registry: registry,
		rows:     skillOverGlobal,
	})

	// repo-value-names-repo
	tests = append(tests, &testCase{
		_id:       "repo-value-names-repo",
		_expected: map[string]Value{"SKILL_FIMPLEMENT_CHECK": {Repo: "main", Source: SourceRepo, Value: "make test"}},

		bindings: checkBinding,
		cwd:      main,
		registry: registry,
		rows:     globalCheck,
	})

	// unbound-global-ignored
	tests = append(tests, &testCase{
		_id:       "unbound-global-ignored",
		_expected: map[string]Value{},

		bindings: nil,
		cwd:      unregistered,
		registry: registry,
		rows:     globalCheck,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, Resolve(test.bindings, test.cwd, test.registry, test.rows))
		})
	}
}

func TestExportLines(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		values map[string]Value
	}

	tests := make([]*testCase, 0)

	// sorted
	tests = append(tests, &testCase{
		_id: "sorted",
		_expected: []string{
			"export SKILL_A_X='first'",
			"export SKILL_B_X='second'",
		},

		values: map[string]Value{"SKILL_B_X": {Value: "second"}, "SKILL_A_X": {Value: "first"}},
	})

	// single-quote-escaped
	tests = append(tests, &testCase{
		_id:       "single-quote-escaped",
		_expected: []string{`export SKILL_A_X='it'\''s'`},

		values: map[string]Value{"SKILL_A_X": {Value: "it's"}},
	})

	// newline-kept-inside-quotes
	tests = append(tests, &testCase{
		_id:       "newline-kept-inside-quotes",
		_expected: []string{"export SKILL_A_X='line one\nline two'"},

		values: map[string]Value{"SKILL_A_X": {Value: "line one\nline two"}},
	})

	// empty-map
	tests = append(tests, &testCase{
		_id:       "empty-map",
		_expected: []string{},

		values: map[string]Value{},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, ExportLines(test.values))
		})
	}
}

func TestValueMap(t *testing.T) {
	values := map[string]Value{
		"SKILL_A_X": {Repo: "main", Source: SourceRepo, Value: "make test"},
		"SKILL_B_X": {Source: SourceGlobalDefault, Value: "go vet ./..."},
	}

	expected := map[string]string{"SKILL_A_X": "make test", "SKILL_B_X": "go vet ./..."}
	assert.Equal(t, expected, ValueMap(values))
}

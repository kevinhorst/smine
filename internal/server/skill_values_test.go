package server

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kevinhorst/smine/internal/repos"
	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/kevinhorst/smine/internal/skills"
)

func TestUnsetRequired(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		registered []repos.Repo
		rows       *skills.Rows
		values     []skills.VariableValue
	}

	model := &skillgraph.Skill{
		Env: map[string]*skillgraph.Variable{
			"BUILD": {Interface: "build@1", Required: true},
			"LANES": {Interface: "lanes@1", Required: true},
			"LINT":  {Interface: "lint@1"},
		},
		Frontmatter: []skillgraph.Field{{Key: "name", Value: " demo"}},
	}
	buildDeferred := skills.VariableValue{IsDeferred: true, Name: "SKILL_DEMO_BUILD", Required: true}
	lanesSet := skills.VariableValue{Name: "SKILL_DEMO_LANES", Required: true, Value: "smine-rules"}
	lintUnset := skills.VariableValue{Name: "SKILL_DEMO_LINT"}
	buildDefault := &skills.Rows{Skills: map[string]string{"SKILL_DEMO_BUILD": "make build"}}
	noRows := &skills.Rows{Skills: map[string]string{}}
	tests := make([]*testCase, 0)

	// install-required-unset
	tests = append(tests, &testCase{
		_id:       "install-required-unset",
		_expected: []string{"SKILL_DEMO_LANES"},

		registered: nil,
		rows:       buildDefault,
		values:     []skills.VariableValue{buildDeferred, {Name: "SKILL_DEMO_LANES", Required: true}, lintUnset},
	})

	// optional-unset-ignored
	tests = append(tests, &testCase{
		_id:       "optional-unset-ignored",
		_expected: nil,

		registered: nil,
		rows:       buildDefault,
		values:     []skills.VariableValue{buildDeferred, lanesSet, lintUnset},
	})

	// repo-required-without-default-or-repo-value
	tests = append(tests, &testCase{
		_id:       "repo-required-without-default-or-repo-value",
		_expected: []string{"SKILL_DEMO_BUILD (missing in 1 of 1 repos: other)"},

		registered: []repos.Repo{{Env: map[string]string{"SKILL_DEMO_LINT": "bin/acdsl"}, Name: "other"}},
		rows:       noRows,
		values:     []skills.VariableValue{buildDeferred, lanesSet, lintUnset},
	})

	// repo-required-with-default
	tests = append(tests, &testCase{
		_id:       "repo-required-with-default",
		_expected: nil,

		registered: nil,
		rows:       buildDefault,
		values:     []skills.VariableValue{buildDeferred, lanesSet, lintUnset},
	})

	// repo-required-with-every-repo-value
	tests = append(tests, &testCase{
		_id:       "repo-required-with-every-repo-value",
		_expected: nil,

		registered: []repos.Repo{{Env: map[string]string{"SKILL_DEMO_BUILD": "go build ./..."}, Name: "demo"}},
		rows:       noRows,
		values:     []skills.VariableValue{buildDeferred, lanesSet, lintUnset},
	})

	// repo-required-lacking-in-one-repo
	tests = append(tests, &testCase{
		_id:       "repo-required-lacking-in-one-repo",
		_expected: []string{"SKILL_DEMO_BUILD (missing in 1 of 2 repos: other)"},

		registered: []repos.Repo{{Env: map[string]string{"SKILL_DEMO_BUILD": "go build ./..."}, Name: "demo"}, {Name: "other"}},
		rows:       noRows,
		values:     []skills.VariableValue{buildDeferred, lanesSet, lintUnset},
	})

	// repo-required-lacking-in-many-repos-truncated
	tests = append(tests, &testCase{
		_id:       "repo-required-lacking-in-many-repos-truncated",
		_expected: []string{"SKILL_DEMO_BUILD (missing in 5 of 6 repos: a, b, c, +2 more)"},

		registered: []repos.Repo{{Name: "a"}, {Name: "b"}, {Name: "c"}, {Env: map[string]string{"SKILL_DEMO_BUILD": "make build"}, Name: "demo"}, {Name: "d"}, {Name: "e"}},
		rows:       noRows,
		values:     []skills.VariableValue{buildDeferred, lanesSet, lintUnset},
	})

	// repo-required-with-global-default
	buildInterface := &skillgraph.Interface{Id: "build", Scope: skillgraph.ScopeRepo}
	buildGlobal := skills.VariableValue{Interface: buildInterface, IsDeferred: true, Name: "SKILL_DEMO_BUILD", Required: true}
	globalDefault := &skills.Rows{Defaults: map[string]string{"build": "make audit"}, Skills: map[string]string{}}
	tests = append(tests, &testCase{
		_id:       "repo-required-with-global-default",
		_expected: nil,

		registered: nil,
		rows:       globalDefault,
		values:     []skills.VariableValue{buildGlobal, lanesSet, lintUnset},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, unsetRequired(model, test.values, test.rows, test.registered))
		})
	}
}

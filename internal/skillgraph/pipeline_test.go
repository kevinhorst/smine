package skillgraph

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// provideGrantingSkill is provideRoutedSkill plus an allowed-tools line.
func provideGrantingSkill(allowedTools string, name string, edges ...Edge) *Skill {
	skill := provideRoutedSkill(name, edges...)
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: "allowed-tools", Value: " " + allowedTools})
	return skill
}

// provideRoutedSkill is a minimal skill named name whose routing block
// carries edges.
func provideRoutedSkill(name string, edges ...Edge) *Skill {
	return &Skill{
		Frontmatter: []Field{{Key: "name", Value: " " + name}},
		Sections: []*Section{{
			Title: SectionRouting,
			Blocks: []Block{{Routing: &Routing{
				Pipeline:      Pipeline{Edges: edges, Standalone: len(edges) == 0},
				Preconditions: []string{"none"},
				Purpose:       name + ".",
				Routes:        []Route{{For: "other", Use: "/other"}},
			}}},
		}},
	}
}

func TestCheckPipelines(t *testing.T) {
	type testCase struct {
		_id       string
		_contains string

		skills []*Skill
	}

	tests := make([]*testCase, 0)

	// pass-agreeing-to-from
	tests = append(tests, &testCase{_id: "pass-agreeing-to-from", _contains: "", skills: []*Skill{
		provideRoutedSkill("a", Edge{Kind: EdgeKindTo, Skill: "b"}),
		provideRoutedSkill("b", Edge{Kind: EdgeKindFrom, Skill: "a"}),
	}})

	// fail-missing-from-twin
	tests = append(tests, &testCase{_id: "fail-missing-from-twin", _contains: "a: pipeline edge to b — b declares no from a", skills: []*Skill{
		provideRoutedSkill("a", Edge{Kind: EdgeKindTo, Skill: "b"}),
		provideRoutedSkill("b"),
	}})

	// fail-missing-spawned-by-twin
	tests = append(tests, &testCase{_id: "fail-missing-spawned-by-twin", _contains: "a: pipeline edge spawns b — b declares no spawnedBy a", skills: []*Skill{
		provideRoutedSkill("a", Edge{Kind: EdgeKindSpawns, Skill: "b"}),
		provideRoutedSkill("b"),
	}})

	// pass-escalates-one-sided
	tests = append(tests, &testCase{_id: "pass-escalates-one-sided", _contains: "", skills: []*Skill{
		provideRoutedSkill("a", Edge{Kind: EdgeKindEscalates, Skill: "b"}),
		provideRoutedSkill("b"),
	}})

	// fail-unknown-skill
	tests = append(tests, &testCase{_id: "fail-unknown-skill", _contains: "a: pipeline edge to ghost — no such skill", skills: []*Skill{
		provideRoutedSkill("a", Edge{Kind: EdgeKindTo, Skill: "ghost"}),
	}})

	// fail-target-without-routing
	tests = append(tests, &testCase{_id: "fail-target-without-routing", _contains: "b declares no from a", skills: []*Skill{
		provideRoutedSkill("a", Edge{Kind: EdgeKindTo, Skill: "b"}),
		{Frontmatter: []Field{{Key: "name", Value: " b"}}},
	}})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			violations := strings.Join(CheckPipelines(test.skills), "\n")
			assert.Equalf(t, test._contains == "", violations == "", "violations = %s", violations)
			assert.Contains(t, violations, test._contains)
		})
	}
}

func TestCheckSkillGrants(t *testing.T) {
	type testCase struct {
		_id       string
		_contains string

		skills []*Skill
	}

	tests := make([]*testCase, 0)

	// pass-no-manifest
	tests = append(tests, &testCase{_id: "pass-no-manifest", _contains: "", skills: []*Skill{
		provideRoutedSkill("a"),
	}})

	// pass-non-skill-rules-ignored
	tests = append(tests, &testCase{_id: "pass-non-skill-rules-ignored", _contains: "", skills: []*Skill{
		provideGrantingSkill("Read, Bash(jq *), Workflow", "a"),
	}})

	// pass-variable-grant-left-out
	tests = append(tests, &testCase{_id: "pass-variable-grant-left-out", _contains: "", skills: []*Skill{
		provideGrantingSkill("Read, Skill($SKILL_A_COMMIT_SKILL)", "a"),
	}})

	// pass-grant-over-to-edge
	tests = append(tests, &testCase{_id: "pass-grant-over-to-edge", _contains: "", skills: []*Skill{
		provideGrantingSkill("Read, Skill(b)", "a", Edge{Kind: EdgeKindTo, Skill: "b"}),
		provideRoutedSkill("b", Edge{Kind: EdgeKindFrom, Skill: "a"}),
	}})

	// pass-grant-over-spawns-edge
	tests = append(tests, &testCase{_id: "pass-grant-over-spawns-edge", _contains: "", skills: []*Skill{
		provideGrantingSkill("Skill(b)", "a", Edge{Kind: EdgeKindSpawns, Skill: "b"}),
		provideRoutedSkill("b", Edge{Kind: EdgeKindSpawnedBy, Skill: "a"}),
	}})

	// pass-grant-over-escalates-edge
	tests = append(tests, &testCase{_id: "pass-grant-over-escalates-edge", _contains: "", skills: []*Skill{
		provideGrantingSkill("Skill(b)", "a", Edge{Kind: EdgeKindEscalates, Skill: "b"}),
		provideRoutedSkill("b"),
	}})

	// pass-grant-transitive
	tests = append(tests, &testCase{_id: "pass-grant-transitive", _contains: "", skills: []*Skill{
		provideGrantingSkill("Skill(c)", "a", Edge{Kind: EdgeKindTo, Skill: "b"}),
		provideRoutedSkill("b", Edge{Kind: EdgeKindFrom, Skill: "a"}, Edge{Kind: EdgeKindSpawns, Skill: "c"}),
		provideRoutedSkill("c", Edge{Kind: EdgeKindSpawnedBy, Skill: "b"}),
	}})

	// pass-self-grant
	tests = append(tests, &testCase{_id: "pass-self-grant", _contains: "", skills: []*Skill{
		provideGrantingSkill("Skill(a)", "a"),
	}})

	// pass-self-grant-without-routing
	tests = append(tests, &testCase{_id: "pass-self-grant-without-routing", _contains: "", skills: []*Skill{
		{Frontmatter: []Field{{Key: "name", Value: " a"}, {Key: "allowed-tools", Value: " Skill(a)"}}},
	}})

	// pass-cycle-terminates
	tests = append(tests, &testCase{_id: "pass-cycle-terminates", _contains: "", skills: []*Skill{
		provideGrantingSkill("Skill(b)", "a", Edge{Kind: EdgeKindTo, Skill: "b"}, Edge{Kind: EdgeKindFrom, Skill: "b"}),
		provideRoutedSkill("b", Edge{Kind: EdgeKindFrom, Skill: "a"}, Edge{Kind: EdgeKindTo, Skill: "a"}),
	}})

	// fail-grant-without-edge
	tests = append(tests, &testCase{_id: "fail-grant-without-edge", _contains: "a: allowed-tools Skill(b) — no outgoing edge reaches b", skills: []*Skill{
		provideGrantingSkill("Skill(b)", "a"),
		provideRoutedSkill("b"),
	}})

	// fail-grant-incoming-edge-only
	tests = append(tests, &testCase{_id: "fail-grant-incoming-edge-only", _contains: "a: allowed-tools Skill(b) — no outgoing edge reaches b", skills: []*Skill{
		provideGrantingSkill("Skill(b)", "a", Edge{Kind: EdgeKindFrom, Skill: "b"}),
		provideRoutedSkill("b", Edge{Kind: EdgeKindTo, Skill: "a"}),
	}})

	// fail-grant-without-routing
	tests = append(tests, &testCase{_id: "fail-grant-without-routing", _contains: "a: allowed-tools Skill(b) — no outgoing edge reaches b", skills: []*Skill{
		{Frontmatter: []Field{{Key: "name", Value: " a"}, {Key: "allowed-tools", Value: " Skill(b)"}}},
		provideRoutedSkill("b"),
	}})

	// fail-second-grant-unreached
	tests = append(tests, &testCase{_id: "fail-second-grant-unreached", _contains: "a: allowed-tools Skill(c) — no outgoing edge reaches c", skills: []*Skill{
		provideGrantingSkill("Skill(b), Skill(c)", "a", Edge{Kind: EdgeKindTo, Skill: "b"}),
		provideRoutedSkill("b", Edge{Kind: EdgeKindFrom, Skill: "a"}),
		provideRoutedSkill("c"),
	}})

	// fail-unknown-skill
	tests = append(tests, &testCase{_id: "fail-unknown-skill", _contains: "a: allowed-tools Skill(ghost) — no such skill", skills: []*Skill{
		provideGrantingSkill("Skill(ghost)", "a"),
	}})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			violations := strings.Join(CheckSkillGrants(test.skills), "\n")
			assert.Equalf(t, test._contains == "", violations == "", "violations = %s", violations)
			assert.Contains(t, violations, test._contains)
		})
	}
}

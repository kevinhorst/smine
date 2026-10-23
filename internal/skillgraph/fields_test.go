package skillgraph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// provideFieldsSkill is the complete fixture at format 3: no labelled item, an exclude Args row, a def opening the Flow section.
func provideFieldsSkill() *Skill {
	skill := provideCompleteSkill()
	setFrontmatter(skill, "format", " 3")
	args := skill.Sections[1].Blocks[0].Args
	args.Rows = append(args.Rows, Arg{Default: "none", Effect: "skips files", Form: "csv", Name: "exclude"})
	flow := flowSection(skill)
	step, _ := skill.Entry("SKILL-DEMO-FLOW-002")
	step.Entry.Items = nil
	def := &Entry{Id: "SKILL-DEMO-FLOW-010", Role: RoleDef, Statement: "The `repo` path the run reads.", Title: "Repo dir"}
	flow.Blocks = append([]Block{{Entry: def}}, flow.Blocks...)
	skill.Atoms["mode"] = &Atom{Kind: KindAtomEnum, Members: []string{"interactive", "nightly"}, Sort: "mode"}
	return skill
}

// setFrontmatter overwrites or appends one frontmatter key.
func setFrontmatter(skill *Skill, key, value string) {
	for index := range skill.Frontmatter {
		if skill.Frontmatter[index].Key == key {
			skill.Frontmatter[index].Value = value
			return
		}
	}
	skill.Frontmatter = append(skill.Frontmatter, Field{Key: key, Value: value})
}

// withEntry applies change to the entry id of skill and returns the skill.
func withEntry(skill *Skill, id string, change func(entry *Entry)) *Skill {
	ref, _ := skill.Entry(id)
	change(ref.Entry)
	return skill
}

// violationHeads cuts each violation to the length of the expected head at the same index, so OS error text and temp paths drop out.
func violationHeads(violations, heads []string) []string {
	var cut []string
	for index, violation := range violations {
		if index < len(heads) && strings.HasPrefix(violation, heads[index]) {
			violation = heads[index]
		}
		cut = append(cut, violation)
	}
	return cut
}

func TestCheckFields(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		skill *Skill
	}

	tests := make([]*testCase, 0)

	// pass-flags-resolve
	tests = append(tests, &testCase{
		_id:       "pass-flags-resolve",
		_expected: nil,

		skill: withEntry(provideFieldsSkill(), "SKILL-DEMO-FLOW-002", func(entry *Entry) {
			entry.Fields = &Fields{Flags: "--repo <repo> --exclude <exclude csv>"}
		}),
	})

	// fail-flags-unknown-placeholder
	tests = append(tests, &testCase{
		_id:       "fail-flags-unknown-placeholder",
		_expected: []string{"SKILL-DEMO-FLOW-002: flags: placeholder <ghost> is no Args row, def, read, returned or minted name"},

		skill: withEntry(provideFieldsSkill(), "SKILL-DEMO-FLOW-002", func(entry *Entry) {
			entry.Fields = &Fields{Flags: "--x <ghost>"}
		}),
	})

	// fail-forwards-no-row
	tests = append(tests, &testCase{
		_id:       "fail-forwards-no-row",
		_expected: []string{"SKILL-DEMO-FLOW-002: forwards: ghost is no Args row"},

		skill: withEntry(provideFieldsSkill(), "SKILL-DEMO-FLOW-002", func(entry *Entry) {
			entry.Fields = &Fields{Forwards: []Forward{{Arg: "ghost", Flag: "--ghost"}}}
		}),
	})

	// fail-forwards-stage-without-schema
	tests = append(tests, &testCase{
		_id:       "fail-forwards-stage-without-schema",
		_expected: []string{"SKILL-DEMO-FLOW-002: forwards: stage mode needs a schema on the entry"},

		skill: withEntry(provideFieldsSkill(), "SKILL-DEMO-FLOW-002", func(entry *Entry) {
			entry.Fields = &Fields{Forwards: []Forward{{Arg: "mode", Stage: "mode"}}}
		}),
	})

	// fail-reads-without-schema
	tests = append(tests, &testCase{
		_id:       "fail-reads-without-schema",
		_expected: []string{"SKILL-DEMO-FLOW-002: reads and returns need a schema on the entry"},

		skill: withEntry(provideFieldsSkill(), "SKILL-DEMO-FLOW-002", func(entry *Entry) {
			entry.Fields = &Fields{Reads: []string{"applied"}}
		}),
	})

	// fail-script-on-step
	tests = append(tests, &testCase{
		_id:       "fail-script-on-step",
		_expected: []string{"SKILL-DEMO-FLOW-002: script: not on a step — roles spawn, substep"},

		skill: withEntry(provideFieldsSkill(), "SKILL-DEMO-FLOW-002", func(entry *Entry) {
			entry.Fields = &Fields{Script: "workflows/a.js"}
		}),
	})

	// fail-returns-on-step
	tests = append(tests, &testCase{
		_id:       "fail-returns-on-step",
		_expected: []string{"SKILL-DEMO-FLOW-002: returns: not on a step — roles spawn, substep"},

		skill: withEntry(provideFieldsSkill(), "SKILL-DEMO-FLOW-002", func(entry *Entry) {
			entry.Fields = &Fields{Returns: []string{"applied"}, Schema: "schemas/a.json"}
		}),
	})

	// fail-timeout-without-run
	tests = append(tests, &testCase{
		_id:       "fail-timeout-without-run",
		_expected: []string{"SKILL-DEMO-FLOW-002: timeout: only a RUN step takes a timeout"},

		skill: withEntry(provideFieldsSkill(), "SKILL-DEMO-FLOW-002", func(entry *Entry) {
			entry.Statement = "WAIT for the first step."
			entry.Fields = &Fields{Timeout: 600000}
		}),
	})

	// fail-mints-empty
	tests = append(tests, &testCase{
		_id:       "fail-mints-empty",
		_expected: []string{"SKILL-DEMO-FLOW-002: mints: names no branch, file or id"},

		skill: withEntry(provideFieldsSkill(), "SKILL-DEMO-FLOW-002", func(entry *Entry) {
			entry.Fields = &Fields{Mints: &Mints{}}
		}),
	})

	// fail-mints-branch-prefix
	prefixed := provideFieldsSkill()
	setFrontmatter(prefixed, "branch-prefix", " claude-demo/")
	tests = append(tests, &testCase{
		_id:       "fail-mints-branch-prefix",
		_expected: []string{"SKILL-DEMO-FLOW-002: mints: branch other/<n> does not start with claude-demo/"},

		skill: withEntry(prefixed, "SKILL-DEMO-FLOW-002", func(entry *Entry) {
			entry.Fields = &Fields{Mints: &Mints{Branch: "other/<n>"}}
		}),
	})

	// fail-mints-id-no-placeholder
	tests = append(tests, &testCase{
		_id:       "fail-mints-id-no-placeholder",
		_expected: []string{`SKILL-DEMO-FLOW-002: mints: id run = "fixed" holds no {…} or <…> placeholder`},

		skill: withEntry(provideFieldsSkill(), "SKILL-DEMO-FLOW-002", func(entry *Entry) {
			entry.Fields = &Fields{Mints: &Mints{Id: map[string]string{"run": "fixed"}}}
		}),
	})

	// fail-def-with-fields
	tests = append(tests, &testCase{
		_id:       "fail-def-with-fields",
		_expected: []string{"SKILL-DEMO-FLOW-010: a def carries no typed field"},

		skill: withEntry(provideFieldsSkill(), "SKILL-DEMO-FLOW-010", func(entry *Entry) {
			entry.Fields = &Fields{Ref: []string{"FLOW-002"}}
		}),
	})

	// fail-script-channel-without-field
	tests = append(tests, &testCase{
		_id:       "fail-script-channel-without-field",
		_expected: []string{"SKILL-DEMO-FLOW-003: [script] names its stage file in the script field"},

		skill: withEntry(provideFieldsSkill(), "SKILL-DEMO-FLOW-003", func(entry *Entry) {
			entry.Channel = ChannelScript
		}),
	})

	// pass-format-2-ignored
	tests = append(tests, &testCase{
		_id:       "pass-format-2-ignored",
		_expected: nil,

		skill: withEntry(provideCompleteSkill(), "SKILL-DEMO-FLOW-002", func(entry *Entry) {
			entry.Fields = &Fields{Script: "workflows/a.js"}
		}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, checkFields(test.skill))
		})
	}
}

func TestCheckFieldFiles(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		dir   string
		skill *Skill
	}

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "workflows"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "schemas"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "workflows", "a.js"), []byte("export default 1\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "schemas", "a.json"), []byte(`{"properties": {"applied": {}, "stage": {}}}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "schemas", "empty.json"), []byte(`{"type": "object"}`), 0o644))

	tests := make([]*testCase, 0)

	// pass-pointers-exist
	tests = append(tests, &testCase{
		_id:       "pass-pointers-exist",
		_expected: nil,

		dir: dir,
		skill: withEntry(provideFieldsSkill(), "SKILL-DEMO-FLOW-002", func(entry *Entry) {
			entry.Fields = &Fields{Forwards: []Forward{{Arg: "mode", Stage: "stage"}}, Reads: []string{"applied.count"}, Schema: "schemas/a.json", Script: "workflows/a.js"}
		}),
	})

	// fail-script-outside-workflows
	tests = append(tests, &testCase{
		_id:       "fail-script-outside-workflows",
		_expected: []string{"SKILL-DEMO-FLOW-002: script: stages/a.js is not under workflows/"},

		dir: dir,
		skill: withEntry(provideFieldsSkill(), "SKILL-DEMO-FLOW-002", func(entry *Entry) {
			entry.Fields = &Fields{Script: "stages/a.js"}
		}),
	})

	// fail-schema-missing
	tests = append(tests, &testCase{
		_id:       "fail-schema-missing",
		_expected: []string{"SKILL-DEMO-FLOW-002: schema: schemas/missing.json: ", "SKILL-DEMO-FLOW-002: schema: schemaProperties: "},

		dir: dir,
		skill: withEntry(provideFieldsSkill(), "SKILL-DEMO-FLOW-002", func(entry *Entry) {
			entry.Fields = &Fields{Schema: "schemas/missing.json"}
		}),
	})

	// fail-schema-without-properties
	tests = append(tests, &testCase{
		_id:       "fail-schema-without-properties",
		_expected: []string{"SKILL-DEMO-FLOW-002: schema: schemaProperties: "},

		dir: dir,
		skill: withEntry(provideFieldsSkill(), "SKILL-DEMO-FLOW-002", func(entry *Entry) {
			entry.Fields = &Fields{Schema: "schemas/empty.json"}
		}),
	})

	// fail-reads-unknown-property
	tests = append(tests, &testCase{
		_id:       "fail-reads-unknown-property",
		_expected: []string{"SKILL-DEMO-FLOW-002: reads: ghost is no property of the entry's schema"},

		dir: dir,
		skill: withEntry(provideFieldsSkill(), "SKILL-DEMO-FLOW-002", func(entry *Entry) {
			entry.Fields = &Fields{Reads: []string{"ghost"}, Schema: "schemas/a.json"}
		}),
	})

	// fail-forwards-stage-unknown-property
	tests = append(tests, &testCase{
		_id:       "fail-forwards-stage-unknown-property",
		_expected: []string{"SKILL-DEMO-FLOW-002: forwards: stage ghost is no property of schemas/a.json"},

		dir: dir,
		skill: withEntry(provideFieldsSkill(), "SKILL-DEMO-FLOW-002", func(entry *Entry) {
			entry.Fields = &Fields{Forwards: []Forward{{Arg: "mode", Stage: "ghost"}}, Schema: "schemas/a.json"}
		}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			violations := checkFieldFiles(test.dir, test.skill)

			assert.Equal(t, test._expected, violationHeads(violations, test._expected))
		})
	}
}

func TestCheckLabels(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		skill *Skill
	}

	tests := make([]*testCase, 0)

	// fail-param-label-on-format-3
	tests = append(tests, &testCase{
		_id:       "fail-param-label-on-format-3",
		_expected: []string{"SKILL-DEMO-FLOW-002: labelled item param: format 3 carries typed fields, no labelled bullets"},

		skill: withEntry(provideFieldsSkill(), "SKILL-DEMO-FLOW-002", func(entry *Entry) {
			entry.Items = []Item{{Label: LabelParam, Marker: "*", Text: "one fact."}}
		}),
	})

	// pass-label-on-format-2
	tests = append(tests, &testCase{
		_id:       "pass-label-on-format-2",
		_expected: nil,

		skill: provideCompleteSkill(),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, checkLabels(test.skill))
		})
	}
}

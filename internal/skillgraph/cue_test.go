package skillgraph

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// encodedFixture is the canonical model file of the fixture.
func encodedFixture(t *testing.T) string {
	t.Helper()
	encoded, err := EncodeSkill(provideCompleteSkill())
	require.NoError(t, err)
	return string(encoded)
}

func TestDecodeSkill(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool

		data string
	}

	canonical := encodedFixture(t)
	replace := func(old, replacement string) string {
		require.Contains(t, canonical, old)
		return strings.Replace(canonical, old, replacement, 1)
	}
	tests := make([]*testCase, 0)

	// pass-canonical
	tests = append(tests, &testCase{_id: "pass-canonical", _shouldPass: true, data: canonical})

	// fail-comment
	tests = append(tests, &testCase{_id: "fail-comment", _shouldPass: false, data: "// a note\n" + canonical})

	// fail-unknown-field
	tests = append(tests, &testCase{_id: "fail-unknown-field", _shouldPass: false, data: canonical + "\nextra: 1\n"})

	// fail-not-concrete
	tests = append(tests, &testCase{_id: "fail-not-concrete", _shouldPass: false, data: replace(`preamble:`, "note: string\npreamble:")})

	// fail-payload-body-inline
	tests = append(tests, &testCase{_id: "fail-payload-body-inline", _shouldPass: false, data: replace(`marker: "`+"```"+`"`, `body: "x\n", marker: "`+"```"+`"`)})

	// fail-post-on-rule
	tests = append(tests, &testCase{_id: "fail-post-on-rule", _shouldPass: false, data: replace(`pred:      "not panic"`, `pred:      "not panic", post: ["panic"]`)})

	// fail-pred-on-def
	tests = append(tests, &testCase{_id: "fail-pred-on-def", _shouldPass: false, data: replace(`role:      "rule"`, `role:      "def"`)})

	// fail-payload-on-step
	tests = append(tests, &testCase{_id: "fail-payload-on-step", _shouldPass: false, data: replace(`role:      "payload"`, `role:      "step"`)})

	// fail-payload-missing
	tests = append(tests, &testCase{_id: "fail-payload-missing", _shouldPass: false, data: replace(`role:      "step"
			statement: "RUN at night."`, `role:      "payload"
			statement: "RUN at night."`)})

	// fail-channel-on-payload
	tests = append(tests, &testCase{_id: "fail-channel-on-payload", _shouldPass: false, data: replace(`role:      "payload"`, `role:      "payload", channel: "prose"`)})

	// fail-bad-id
	tests = append(tests, &testCase{_id: "fail-bad-id", _shouldPass: false, data: replace(`"SKILL-DEMO-FLOW-003"`, `"SKILL-DEMO-FLOW-3"`)})

	// fail-bad-role
	tests = append(tests, &testCase{_id: "fail-bad-role", _shouldPass: false, data: replace(`role:      "rule"`, `role:      "gate"`)})

	// fail-title-four-words
	tests = append(tests, &testCase{_id: "fail-title-four-words", _shouldPass: false, data: replace(`"Night step"`, `"Night step four words"`)})

	// fail-route-without-pred
	tests = append(tests, &testCase{_id: "fail-route-without-pred", _shouldPass: false, data: replace(`pred: "mode = nightly"`, ``)})

	// fail-pred-on-phase
	tests = append(tests, &testCase{_id: "fail-pred-on-phase", _shouldPass: false, data: replace(`kind: "route"`, `kind: "phase"`)})

	// fail-anchor-on-phase
	tests = append(tests, &testCase{_id: "fail-anchor-on-phase", _shouldPass: false, data: replace(`title: "Flow"`, `title: "Flow", replaces: "FLOW-001"`)})

	// fail-block-two-kinds
	tests = append(tests, &testCase{_id: "fail-block-two-kinds", _shouldPass: false, data: replace("\t\trouting: {\n", "\t\theading: \"Extra\"\n\t\trouting: {\n")})

	// fail-members-on-bool
	tests = append(tests, &testCase{_id: "fail-members-on-bool", _shouldPass: false, data: replace(`sort: "state"`, `sort: "state", members: ["a"]`)})

	// fail-item-label-pred
	tests = append(tests, &testCase{_id: "fail-item-label-pred", _shouldPass: false, data: replace(`label:  "param"`, `label:  "pred"`)})

	// fail-changelog-text-and-changes
	tests = append(tests, &testCase{_id: "fail-changelog-text-and-changes", _shouldPass: false, data: replace(`text:    "fixture"`, `text:    "fixture", changes: ["x"]`)})

	// pass-changelog-changes-list
	tests = append(tests, &testCase{_id: "pass-changelog-changes-list", _shouldPass: true, data: replace(`text:    "fixture"`, `changes: ["x"]`)})

	// fail-edge-kind
	tests = append(tests, &testCase{_id: "fail-edge-kind", _shouldPass: false, data: replace(`kind:  "to"`, `kind:  "before"`)})

	// fail-edge-skill-name
	tests = append(tests, &testCase{_id: "fail-edge-skill-name", _shouldPass: false, data: replace(`skill: "fdesign"`, `skill: "/fdesign"`)})

	// fail-empty-preconditions
	tests = append(tests, &testCase{_id: "fail-empty-preconditions", _shouldPass: false, data: replace(`preconditions: ["a demo request"]`, `preconditions: []`)})

	// fail-model-tier-size-word
	tests = append(tests, &testCase{_id: "fail-model-tier-size-word", _shouldPass: false, data: replace(`tier:   "frontier"`, `tier:   "large"`)})

	// fail-model-effort-missing
	tests = append(tests, &testCase{_id: "fail-model-effort-missing", _shouldPass: false, data: replace(`effort: "high"`, ``)})

	// fail-model-runner-field
	tests = append(tests, &testCase{_id: "fail-model-runner-field", _shouldPass: false, data: replace(`effort: "high"`, `effort: "high", runner: "demo-runner"`)})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			_, err := DecodeSkill([]byte(test.data), modelDecodeLabel)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
		})
	}
}

func TestEncodeSkill(t *testing.T) {
	first, err := EncodeSkill(provideCompleteSkill())
	require.NoError(t, err)

	second, err := EncodeSkill(provideCompleteSkill())
	require.NoError(t, err)
	assert.Equal(t, string(first), string(second), "pass-deterministic-across-contexts")
	assert.Less(t, strings.Index(string(first), "first_done"), strings.Index(string(first), "panic:"), "pass-atoms-sorted-by-key")
	assert.NotContains(t, string(first), "## TLDR", "pass-payload-body-never-encoded")
}

func TestGuardRoundTrip(t *testing.T) {
	type testCase struct {
		_id         string
		_contains   string
		_shouldPass bool

		encoded func(t *testing.T) []byte
		skill   *Skill
	}

	encodeOf := func(skill *Skill) func(t *testing.T) []byte {
		return func(t *testing.T) []byte {
			encoded, err := EncodeSkill(skill)
			require.NoError(t, err)
			return encoded
		}
	}
	tests := make([]*testCase, 0)

	// guard-pass-decoded-equals-model
	skill := provideCompleteSkill()
	tests = append(tests, &testCase{_id: "guard-pass-decoded-equals-model", _shouldPass: true, encoded: encodeOf(skill), skill: skill})

	// pass-interpolation-escaped
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[2].Entry.Statement = `RUN \(literal) and "quotes" and ⇒ and $x.`
	tests = append(tests, &testCase{_id: "pass-interpolation-escaped", _shouldPass: true, encoded: encodeOf(skill), skill: skill})

	// pass-multi-line-note-round-trips
	skill = provideCompleteSkill()
	skill.Note = "line one\nline two\n"
	tests = append(tests, &testCase{_id: "pass-multi-line-note-round-trips", _shouldPass: true, encoded: encodeOf(skill), skill: skill})

	// pass-empty-list-round-trips
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[2].Entry.Post = []string{}
	tests = append(tests, &testCase{_id: "pass-empty-list-round-trips", _shouldPass: true, encoded: encodeOf(skill), skill: skill})

	// model-with-requires-needs-unless-route-and-edge-needs
	skill = provideRequiringSkill()
	tests = append(tests, &testCase{_id: "model-with-requires-needs-unless-route-and-edge-needs", _shouldPass: true, encoded: encodeOf(skill), skill: skill})

	// entry-with-every-field
	skill = provideCompleteSkill()
	flowSection(skill).Blocks[2].Entry.Fields = &Fields{
		Flags:    "--round-dir <round_dir>",
		Forwards: []Forward{{Arg: "lanes", Flag: "--lanes", Stage: "lanes"}},
		Mints:    &Mints{Branch: "claude/review-{token}", File: "round-{n}/review.json", Id: map[string]string{"round_id": "R{n}"}},
		Reads:    []string{"lanes"},
		Ref:      []string{"FLOW-001", "SKILL-OTHER-FLOW-002"},
		Returns:  []string{"review.path"},
		Schema:   "schemas/round.json",
		Script:   "workflows/round.js",
		Timeout:  600000,
	}
	tests = append(tests, &testCase{_id: "entry-with-every-field", _shouldPass: true, encoded: encodeOf(skill), skill: skill})

	// guard-fail-encoded-model-invalid
	skill = provideCompleteSkill()
	invalid := provideCompleteSkill()
	flowSection(invalid).Blocks[0].Entry.Role = "gate"
	tests = append(tests, &testCase{_id: "guard-fail-encoded-model-invalid", _contains: "does not load", encoded: encodeOf(invalid), skill: skill})

	// guard-fail-bytes-of-another-model
	skill = provideCompleteSkill()
	other := provideCompleteSkill()
	flowSection(other).Blocks[2].Entry.Statement = "RUN something else."
	tests = append(tests, &testCase{_id: "guard-fail-bytes-of-another-model", _contains: `section "Flow"`, encoded: encodeOf(other), skill: skill})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			err := guardRoundTrip(test.encoded(t), test.skill)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Contains(t, errorText(err), test._contains)
		})
	}
}

func TestVocabularyRoundTrip(t *testing.T) {
	vocabulary := &Vocabulary{Atoms: map[string]*Atom{
		"mode":  {Aliases: []string{"headless"}, Kind: KindAtomEnum, Members: []string{"interactive", "nightly"}, Sort: "mode", Source: "session"},
		"ready": {Sort: "state"},
	}}
	encoded, err := encodeVocabulary(vocabulary)
	require.NoError(t, err)
	require.NoError(t, guardVocabularyRoundTrip(encoded, FileVocabulary), "pass-round-trip")

	_, err = decodeVocabulary(append([]byte("// comment\n"), encoded...), FileVocabulary)
	require.Error(t, err, "fail-comment")

	withObserver := strings.Replace(string(encoded), `sort: "state"`, `sort: "state", observer: {kind: "none"}`, 1)
	_, err = decodeVocabulary([]byte(withObserver), FileVocabulary)
	require.Error(t, err, "fail-observer-field")

	withoutMembers := strings.Replace(string(encoded), `kind: "enum"`, `kind: "enum", members: []`, 1)
	_, err = decodeVocabulary([]byte(withoutMembers), FileVocabulary)
	require.Error(t, err, "fail-enum-without-members")
}

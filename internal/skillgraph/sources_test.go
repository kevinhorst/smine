package skillgraph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// withAtom declares atom a on the fields fixture, with demoEnforce's anchors unless enforce is nil.
func withAtom(atom *Atom, enforce *Enforce) *Skill {
	skill := provideFieldsSkill()
	skill.Atoms["a"] = atom
	skill.Enforce = enforce
	return skill
}

func TestParseSource(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   []SourcePart
		_shouldPass bool

		source string
	}

	tests := make([]*testCase, 0)

	// session
	tests = append(tests, &testCase{
		_id:         "session",
		_expected:   []SourcePart{{Kind: SourceSession}},
		_shouldPass: true,

		source: "session",
	})

	// environment
	tests = append(tests, &testCase{
		_id:         "environment",
		_expected:   []SourcePart{{Kind: SourceEnvironment}},
		_shouldPass: true,

		source: "environment",
	})

	// arg-with-member
	tests = append(tests, &testCase{
		_id:         "arg-with-member",
		_expected:   []SourcePart{{Kind: SourceSession}, {Arg: "nightly", Kind: SourceArg, Member: "nightly"}},
		_shouldPass: true,

		source: "session | arg: --nightly => nightly",
	})

	// runtime-anchor
	tests = append(tests, &testCase{
		_id:         "runtime-anchor",
		_expected:   []SourcePart{{Anchor: "ledger", Kind: SourceRuntime}},
		_shouldPass: true,

		source: "runtime: ledger",
	})

	// runtime-capture
	tests = append(tests, &testCase{
		_id:         "runtime-capture",
		_expected:   []SourcePart{{Anchor: "ledger", Capture: "void_rounds", Kind: SourceRuntime}},
		_shouldPass: true,

		source: "runtime: ledger.void_rounds",
	})

	// runtime-equal
	tests = append(tests, &testCase{
		_id:         "runtime-equal",
		_expected:   []SourcePart{{Anchor: "base_health", Capture: "base_build", Kind: SourceRuntime, Literal: "ok", Op: "="}},
		_shouldPass: true,

		source: "runtime: base_health.base_build = ok",
	})

	// runtime-not-equal
	tests = append(tests, &testCase{
		_id:         "runtime-not-equal",
		_expected:   []SourcePart{{Anchor: "ledger", Capture: "oscillating", Kind: SourceRuntime, Literal: "[]", Op: "!="}},
		_shouldPass: true,

		source: "runtime: ledger.oscillating != []",
	})

	// fail-free-text
	tests = append(tests, &testCase{
		_id:         "fail-free-text",
		_expected:   nil,
		_shouldPass: false,

		source: "check: the bracket base does not resolve",
	})

	// fail-runtime-malformed
	tests = append(tests, &testCase{
		_id:         "fail-runtime-malformed",
		_expected:   nil,
		_shouldPass: false,

		source: "runtime: Ledger.void rounds",
	})

	// empty-is-nil
	tests = append(tests, &testCase{
		_id:         "empty-is-nil",
		_expected:   nil,
		_shouldPass: true,

		source: "",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			parts, err := ParseSource(test.source)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, parts)
		})
	}
}

func TestCheckSources(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		skill *Skill
	}

	enum := func(source string) *Atom {
		return &Atom{Kind: KindAtomEnum, Members: []string{"x", "y"}, Sort: "mode", Source: source}
	}

	tests := make([]*testCase, 0)

	// pass-arg-row
	tests = append(tests, &testCase{
		_id:       "pass-arg-row",
		_expected: nil,

		skill: withAtom(&Atom{Sort: "state", Source: "session | arg: --mode"}, demoEnforce()),
	})

	// fail-arg-no-row
	tests = append(tests, &testCase{
		_id:       "fail-arg-no-row",
		_expected: []string{"atom a: arg ghost is no Args row"},

		skill: withAtom(&Atom{Sort: "state", Source: "arg: ghost"}, demoEnforce()),
	})

	// fail-member-not-of-enum
	tests = append(tests, &testCase{
		_id:       "fail-member-not-of-enum",
		_expected: []string{"atom a: => z names no member of this atom"},

		skill: withAtom(enum("arg: mode => z"), demoEnforce()),
	})

	// pass-runtime-bool-capture
	tests = append(tests, &testCase{
		_id:       "pass-runtime-bool-capture",
		_expected: nil,

		skill: withAtom(&Atom{Sort: "state", Source: "runtime: produced.value != ok"}, demoEnforce()),
	})

	// fail-runtime-unknown-anchor
	tests = append(tests, &testCase{
		_id:       "fail-runtime-unknown-anchor",
		_expected: []string{"atom a: runtime: unknown anchor ghost"},

		skill: withAtom(&Atom{Sort: "state", Source: "runtime: ghost"}, demoEnforce()),
	})

	// fail-runtime-unknown-capture
	tests = append(tests, &testCase{
		_id:       "fail-runtime-unknown-capture",
		_expected: []string{"atom a: runtime: anchor produced captures no ghost"},

		skill: withAtom(&Atom{Sort: "state", Source: "runtime: produced.ghost"}, demoEnforce()),
	})

	// fail-int-without-capture
	tests = append(tests, &testCase{
		_id:       "fail-int-without-capture",
		_expected: []string{"atom a: a int atom reads a capture value — runtime: produced.<capture>"},

		skill: withAtom(&Atom{Kind: KindAtomInt, Sort: "count", Source: "runtime: produced"}, demoEnforce()),
	})

	// fail-enum-with-op
	tests = append(tests, &testCase{
		_id:       "fail-enum-with-op",
		_expected: []string{"atom a: a enum atom takes the capture's value, no comparison"},

		skill: withAtom(enum("runtime: produced.value = x"), demoEnforce()),
	})

	// fail-bool-op-without-capture
	tests = append(tests, &testCase{
		_id:       "fail-bool-op-without-capture",
		_expected: []string{"atom a: a comparison needs a capture — runtime: produced.<capture> = ok"},

		skill: withAtom(&Atom{Sort: "state", Source: "runtime: produced = ok"}, demoEnforce()),
	})

	// fail-no-enforce-block
	tests = append(tests, &testCase{
		_id:       "fail-no-enforce-block",
		_expected: []string{"atom a: runtime: produced — the skill declares no enforce block"},

		skill: withAtom(&Atom{Sort: "state", Source: "runtime: produced"}, nil),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, checkSources(test.skill))
		})
	}
}

func TestCheckVocabularySources(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		vocabulary *Vocabulary
	}

	tests := make([]*testCase, 0)

	// fail-runtime-shared
	tests = append(tests, &testCase{
		_id:       "fail-runtime-shared",
		_expected: []string{"atom shared: a shared atom takes no runtime source — the capture lives in one skill's enforce block"},

		vocabulary: &Vocabulary{Atoms: map[string]*Atom{"shared": {Sort: "state", Source: "runtime: ledger.autofix"}}},
	})

	// pass-session
	tests = append(tests, &testCase{
		_id:       "pass-session",
		_expected: nil,

		vocabulary: &Vocabulary{Atoms: map[string]*Atom{"shared": {Sort: "state", Source: "session"}}},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, checkVocabularySources(test.vocabulary))
		})
	}
}

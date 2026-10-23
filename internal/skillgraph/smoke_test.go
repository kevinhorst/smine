package skillgraph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// demoSmoke is a valid run declaration judged by demoEnforce's stage
// anchor.
func demoSmoke() *Smoke {
	invoke := "task: x"
	return &Smoke{Expect: &SmokeExpect{Anchors: []string{"stage"}}, Invoke: &invoke}
}

// provideSmokedSkill is the complete fixture carrying demoEnforce and
// demoSmoke.
func provideSmokedSkill() *Skill {
	skill := provideCompleteSkill()
	skill.Enforce = demoEnforce()
	skill.Smoke = demoSmoke()
	return skill
}

// withSmoke appends a smoke block to encoded model file bytes.
func withSmoke(block string, encoded []byte) []byte {
	return append(slices.Clone(encoded), []byte("smoke: "+block+"\n")...)
}

func TestCheckSmoke(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		skill *Skill
	}

	empty := ""
	tests := make([]*testCase, 0)

	// pass-no-block
	tests = append(tests, &testCase{
		_id:       "pass-no-block",
		_expected: nil,

		skill: provideCompleteSkill(),
	})

	// pass-skip
	skill := provideCompleteSkill()
	skill.Smoke = &Smoke{Skip: "acts on a live fleet"}
	tests = append(tests, &testCase{
		_id:       "pass-skip",
		_expected: nil,

		skill: skill,
	})

	// pass-run-anchors
	tests = append(tests, &testCase{
		_id:       "pass-run-anchors",
		_expected: nil,

		skill: provideSmokedSkill(),
	})

	// pass-run-check-empty-invoke
	skill = provideCompleteSkill()
	skill.Smoke = &Smoke{Expect: &SmokeExpect{Check: []string{"make", "audit"}}, Invoke: &empty}
	tests = append(tests, &testCase{
		_id:       "pass-run-check-empty-invoke",
		_expected: nil,

		skill: skill,
	})

	// pass-interactive-anchors
	skill = provideSmokedSkill()
	skill.Smoke.Mode = SmokeModeInteractive
	tests = append(tests, &testCase{
		_id:       "pass-interactive-anchors",
		_expected: nil,

		skill: skill,
	})

	// pass-fixture-local
	skill = provideSmokedSkill()
	skill.Smoke.Fixture = &SmokeFixture{Patch: "change.patch", Tree: "tree"}
	tests = append(tests, &testCase{
		_id:       "pass-fixture-local",
		_expected: nil,

		skill: skill,
	})

	// fail-skip-and-invoke
	skill = provideSmokedSkill()
	skill.Smoke.Skip = "x"
	tests = append(tests, &testCase{
		_id:       "fail-skip-and-invoke",
		_expected: []string{"smoke: skip and invoke both declared — a smoke is a skip or a run"},

		skill: skill,
	})

	// fail-neither
	skill = provideCompleteSkill()
	skill.Smoke = &Smoke{}
	tests = append(tests, &testCase{
		_id:       "fail-neither",
		_expected: []string{"smoke: neither skip nor invoke declared — declare a run, or a skip with its reason"},

		skill: skill,
	})

	// fail-unknown-anchor
	skill = provideSmokedSkill()
	skill.Smoke.Expect.Anchors = []string{"stage", "nope"}
	tests = append(tests, &testCase{
		_id:       "fail-unknown-anchor",
		_expected: []string{"smoke: expect anchor nope: no such anchor in the enforce block"},

		skill: skill,
	})

	// fail-anchor-without-enforce
	skill = provideSmokedSkill()
	skill.Enforce = nil
	tests = append(tests, &testCase{
		_id:       "fail-anchor-without-enforce",
		_expected: []string{"smoke: expect anchor stage: no such anchor in the enforce block"},

		skill: skill,
	})

	// fail-interactive-check
	skill = provideCompleteSkill()
	skill.Smoke = &Smoke{Expect: &SmokeExpect{Check: []string{"make", "audit"}}, Invoke: &empty, Mode: SmokeModeInteractive}
	tests = append(tests, &testCase{
		_id:       "fail-interactive-check",
		_expected: []string{"smoke: an interactive run declares expect.anchors — its verdict comes from the session's heartbeat rows"},

		skill: skill,
	})

	// fail-fixture-parent
	skill = provideSmokedSkill()
	skill.Smoke.Fixture = &SmokeFixture{Patch: "../change.patch", Tree: "a/../../b"}
	tests = append(tests, &testCase{
		_id: "fail-fixture-parent",
		_expected: []string{
			`smoke: fixture patch "../change.patch" leaves smoke/ — name a path inside it`,
			`smoke: fixture tree "a/../../b" leaves smoke/ — name a path inside it`,
		},

		skill: skill,
	})

	// fail-fixture-inner-parent
	skill = provideSmokedSkill()
	skill.Smoke.Fixture = &SmokeFixture{Tree: "a/../b"}
	tests = append(tests, &testCase{
		_id:       "fail-fixture-inner-parent",
		_expected: []string{`smoke: fixture tree "a/../b" leaves smoke/ — name a path inside it`},

		skill: skill,
	})

	// fail-fixture-absolute
	skill = provideSmokedSkill()
	skill.Smoke.Fixture = &SmokeFixture{Tree: "/tmp/tree"}
	tests = append(tests, &testCase{
		_id:       "fail-fixture-absolute",
		_expected: []string{`smoke: fixture tree "/tmp/tree" leaves smoke/ — name a path inside it`},

		skill: skill,
	})

	envSkill := func(smokeEnv map[string]string) *Skill {
		skill := provideSmokedSkill()
		bindEnv(skill, map[string]*Variable{
			"BUILD": {Interface: "build@1", Required: true},
			"LANES": {Interface: "lanes@1", Required: true},
			"TEST":  {Interface: "test@1"},
		})
		skill.Smoke.Env = smokeEnv
		return skill
	}

	// pass-env-required-set
	tests = append(tests, &testCase{
		_id:       "pass-env-required-set",
		_expected: nil,

		skill: envSkill(map[string]string{"BUILD": "go build ./x/..."}),
	})

	// pass-env-optional-set
	tests = append(tests, &testCase{
		_id:       "pass-env-optional-set",
		_expected: nil,

		skill: envSkill(map[string]string{"BUILD": "go build ./x/...", "TEST": "go test ./x/..."}),
	})

	// fail-env-required-missing
	tests = append(tests, &testCase{
		_id:       "fail-env-required-missing",
		_expected: []string{"smoke: env BUILD: required and repo-scoped — set it in the smoke's env, a smoke never reads the machine's value"},

		skill: envSkill(nil),
	})

	// fail-env-undeclared
	tests = append(tests, &testCase{
		_id:       "fail-env-undeclared",
		_expected: []string{"smoke: env NOPE: no env entry declares it"},

		skill: envSkill(map[string]string{"BUILD": "go build ./x/...", "NOPE": "x"}),
	})

	// fail-env-install-scoped
	tests = append(tests, &testCase{
		_id:       "fail-env-install-scoped",
		_expected: []string{"smoke: env LANES: lanes@1 is install-scoped — a smoke sets repo-scoped values only"},

		skill: envSkill(map[string]string{"BUILD": "go build ./x/...", "LANES": "smine-rules"}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			violations := checkSmoke(test.skill)

			assert.Equal(t, test._expected, violations)
		})
	}
}

func TestDecodeSmoke(t *testing.T) {
	type testCase struct {
		_id         string
		_shouldPass bool

		data []byte
	}

	encoded, err := EncodeSkill(provideCompleteSkill())
	require.NoError(t, err)
	tests := make([]*testCase, 0)

	// pass-skip
	tests = append(tests, &testCase{_id: "pass-skip", _shouldPass: true, data: withSmoke(`{skip: "acts on a live fleet"}`, encoded)})

	// pass-run-minimal
	tests = append(tests, &testCase{_id: "pass-run-minimal", _shouldPass: true, data: withSmoke(`{invoke: "", expect: {}}`, encoded)})

	// pass-run-check
	tests = append(tests, &testCase{_id: "pass-run-check", _shouldPass: true, data: withSmoke(`{invoke: "x", expect: {check: ["make", "audit"]}}`, encoded)})

	// pass-run-full
	full := `{effort: "low", expect: {anchors: ["stage"], minAnchors: 0.5, minChecks: 0}, fixture: {patch: "change.patch", tree: "tree"}, invoke: "task: x", limits: {minutes: 10, usd: 1.5}, mode: "interactive", model: "opus"}`
	tests = append(tests, &testCase{_id: "pass-run-full", _shouldPass: true, data: withSmoke(full, encoded)})

	// fail-empty-block
	tests = append(tests, &testCase{_id: "fail-empty-block", _shouldPass: false, data: withSmoke(`{}`, encoded)})

	// fail-skip-and-invoke
	tests = append(tests, &testCase{_id: "fail-skip-and-invoke", _shouldPass: false, data: withSmoke(`{skip: "x", invoke: "", expect: {}}`, encoded)})

	// fail-empty-skip
	tests = append(tests, &testCase{_id: "fail-empty-skip", _shouldPass: false, data: withSmoke(`{skip: ""}`, encoded)})

	// fail-multiline-invoke
	tests = append(tests, &testCase{_id: "fail-multiline-invoke", _shouldPass: false, data: withSmoke(`{invoke: "a\nb", expect: {}}`, encoded)})

	// fail-no-expect
	tests = append(tests, &testCase{_id: "fail-no-expect", _shouldPass: false, data: withSmoke(`{invoke: "x"}`, encoded)})

	// fail-empty-anchors
	tests = append(tests, &testCase{_id: "fail-empty-anchors", _shouldPass: false, data: withSmoke(`{invoke: "x", expect: {anchors: []}}`, encoded)})

	// fail-anchors-and-check
	tests = append(tests, &testCase{_id: "fail-anchors-and-check", _shouldPass: false, data: withSmoke(`{invoke: "x", expect: {anchors: ["stage"], check: ["make"]}}`, encoded)})

	// fail-ratio-above-one
	tests = append(tests, &testCase{_id: "fail-ratio-above-one", _shouldPass: false, data: withSmoke(`{invoke: "x", expect: {anchors: ["stage"], minAnchors: 1.5}}`, encoded)})

	// fail-minimum-on-check
	tests = append(tests, &testCase{_id: "fail-minimum-on-check", _shouldPass: false, data: withSmoke(`{invoke: "x", expect: {minChecks: 0.5}}`, encoded)})

	// fail-empty-check
	tests = append(tests, &testCase{_id: "fail-empty-check", _shouldPass: false, data: withSmoke(`{invoke: "x", expect: {check: []}}`, encoded)})

	// fail-zero-minutes
	tests = append(tests, &testCase{_id: "fail-zero-minutes", _shouldPass: false, data: withSmoke(`{invoke: "x", expect: {}, limits: {minutes: 0}}`, encoded)})

	// fail-negative-usd
	tests = append(tests, &testCase{_id: "fail-negative-usd", _shouldPass: false, data: withSmoke(`{invoke: "x", expect: {}, limits: {usd: -1}}`, encoded)})

	// fail-unknown-mode
	tests = append(tests, &testCase{_id: "fail-unknown-mode", _shouldPass: false, data: withSmoke(`{invoke: "x", expect: {}, mode: "daily"}`, encoded)})

	// fail-unknown-effort
	tests = append(tests, &testCase{_id: "fail-unknown-effort", _shouldPass: false, data: withSmoke(`{invoke: "x", expect: {}, effort: "huge"}`, encoded)})

	// fail-unknown-field
	tests = append(tests, &testCase{_id: "fail-unknown-field", _shouldPass: false, data: withSmoke(`{invoke: "x", expect: {}, prompt: "y"}`, encoded)})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			_, err := DecodeSkill(test.data, modelDecodeLabel)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
		})
	}
}

func TestSmokeOf(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		skill *Skill
	}

	tests := make([]*testCase, 0)

	// run-defaults-resolved
	skill := provideSmokedSkill()
	skill.Smoke.Fixture = &SmokeFixture{Patch: "change.patch", Tree: "tree"}
	tests = append(tests, &testCase{
		_id: "run-defaults-resolved",
		_expected: `{
  "effort": "low",
  "expect": {
    "anchors": [
      "stage"
    ],
    "minAnchors": 0,
    "minChecks": 0
  },
  "fixture": {
    "patch": "change.patch",
    "tree": "tree"
  },
  "invoke": "task: x",
  "limits": {
    "minutes": 15,
    "usd": 3
  },
  "mode": "nightly",
  "model": "sonnet",
  "schema": 1,
  "skill": "demo",
  "version": "1.0"
}
`,

		skill: skill,
	})

	// run-check-declared-values-kept
	skill = provideCompleteSkill()
	invoke := ""
	skill.Smoke = &Smoke{
		Effort: "high",
		Env:    map[string]string{"TEST": "go test ./x/..."},
		Expect: &SmokeExpect{Check: []string{"make", "audit"}},
		Invoke: &invoke,
		Limits: &SmokeLimits{Minutes: 5},
		Model:  "opus",
	}
	tests = append(tests, &testCase{
		_id: "run-check-declared-values-kept",
		_expected: `{
  "effort": "high",
  "env": {
    "TEST": "go test ./x/..."
  },
  "expect": {
    "check": [
      "make",
      "audit"
    ]
  },
  "invoke": "",
  "limits": {
    "minutes": 5,
    "usd": 3
  },
  "mode": "nightly",
  "model": "opus",
  "schema": 1,
  "skill": "demo",
  "version": "1.0"
}
`,

		skill: skill,
	})

	// skip-reason-only
	skill = provideCompleteSkill()
	skill.Smoke = &Smoke{Skip: "acts on a live fleet"}
	tests = append(tests, &testCase{
		_id: "skip-reason-only",
		_expected: `{
  "skip": "acts on a live fleet",
  "schema": 1,
  "skill": "demo",
  "version": "1.0"
}
`,

		skill: skill,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			encoded, err := json.MarshalIndent(SmokeOf(test.skill), "", "  ")

			require.NoError(t, err)
			assert.Equal(t, test._expected, string(encoded)+"\n")
		})
	}

	assert.Nil(t, SmokeOf(provideCompleteSkill()), "no-smoke-block-nil")
}

func TestLoadSmoke(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   *SmokeDocument
		_shouldPass bool

		dir string
	}

	skill := provideSmokedSkill()
	smokedDir := filepath.Join(t.TempDir(), "demo")
	require.NoError(t, Save(smokedDir, skill))
	plainDir := filepath.Join(t.TempDir(), "demo")
	require.NoError(t, Save(plainDir, provideCompleteSkill()))
	expected := &SmokeDocument{Schema: smokeSchema, Skill: "demo", Smoke: resolvedSmoke(skill.Smoke), Version: "1.0"}
	tests := make([]*testCase, 0)

	// no-smoke-block-nil
	tests = append(tests, &testCase{
		_id:         "no-smoke-block-nil",
		_expected:   nil,
		_shouldPass: true,

		dir: plainDir,
	})

	// smoke-block-resolved
	tests = append(tests, &testCase{
		_id:         "smoke-block-resolved",
		_expected:   expected,
		_shouldPass: true,

		dir: smokedDir,
	})

	// no-model-fails
	tests = append(tests, &testCase{
		_id:         "no-model-fails",
		_expected:   nil,
		_shouldPass: false,

		dir: t.TempDir(),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			document, err := LoadSmoke(test.dir)

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, document)
		})
	}
}

func TestSmokeFixtureViolations(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		dir   string
		smoke *Smoke
	}

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, DirSmoke, "tree"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, DirSmoke, "change.patch"), []byte("diff\n"), 0o600))
	withFixture := func(fixture *SmokeFixture) *Smoke {
		smoke := demoSmoke()
		smoke.Fixture = fixture
		return smoke
	}
	tests := make([]*testCase, 0)

	// pass-no-block
	tests = append(tests, &testCase{_id: "pass-no-block", _expected: nil, dir: dir, smoke: nil})

	// pass-no-fixture
	tests = append(tests, &testCase{_id: "pass-no-fixture", _expected: nil, dir: dir, smoke: demoSmoke()})

	// pass-present
	tests = append(tests, &testCase{_id: "pass-present", _expected: nil, dir: dir, smoke: withFixture(&SmokeFixture{Patch: "change.patch", Tree: "tree"})})

	// pass-non-local-left-to-check
	tests = append(tests, &testCase{_id: "pass-non-local-left-to-check", _expected: nil, dir: dir, smoke: withFixture(&SmokeFixture{Tree: "../tree"})})

	// fail-tree-missing
	missing := fmt.Sprintf("smoke: fixture tree absent: stat %s: no such file or directory", filepath.Join(dir, DirSmoke, "absent"))
	tests = append(tests, &testCase{_id: "fail-tree-missing", _expected: []string{missing}, dir: dir, smoke: withFixture(&SmokeFixture{Tree: "absent"})})

	// fail-tree-is-file
	tests = append(tests, &testCase{_id: "fail-tree-is-file", _expected: []string{"smoke: fixture tree change.patch: wrong kind — tree names a directory, patch a file"}, dir: dir, smoke: withFixture(&SmokeFixture{Tree: "change.patch"})})

	// fail-patch-is-dir
	tests = append(tests, &testCase{_id: "fail-patch-is-dir", _expected: []string{"smoke: fixture patch tree: wrong kind — tree names a directory, patch a file"}, dir: dir, smoke: withFixture(&SmokeFixture{Patch: "tree"})})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			violations := smokeFixtureViolations(test.dir, test.smoke)

			assert.Equal(t, test._expected, violations)
		})
	}
}

func TestLoad_SmokeFixture(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "demo")
	skill := provideSmokedSkill()
	skill.Smoke.Fixture = &SmokeFixture{Tree: "tree"}
	require.NoError(t, Save(dir, skill))

	_, err := Load(dir)
	require.ErrorContains(t, err, "smoke: fixture tree tree", "fail-tree-missing")

	require.NoError(t, os.MkdirAll(filepath.Join(dir, DirSmoke, "tree"), 0o755))
	loaded, err := Load(dir)
	require.NoError(t, err)
	loaded.Catalog = nil
	loaded.Vocabulary = nil
	loaded.Dir = ""
	assert.Equal(t, skill, loaded, "pass-tree-present: Load == model")
}

func TestMissingSmokes(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		rootSkills []*Skill
	}

	tests := make([]*testCase, 0)

	// all-declared
	tests = append(tests, &testCase{_id: "all-declared", _expected: nil, rootSkills: []*Skill{provideSmokedSkill()}})

	// one-missing
	tests = append(tests, &testCase{
		_id:       "one-missing",
		_expected: []string{"demo: no smoke block — declare a run, or a skip with its reason, in demo.cue"},

		rootSkills: []*Skill{provideSmokedSkill(), provideCompleteSkill()},
	})

	// empty-root
	tests = append(tests, &testCase{_id: "empty-root", _expected: nil, rootSkills: nil})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			violations := MissingSmokes(test.rootSkills)

			assert.Equal(t, test._expected, violations)
		})
	}
}

func TestRenderHelpTest(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		hasEval bool
		skill   *Skill
	}

	empty := ""
	tests := make([]*testCase, 0)

	// nothing-declared
	tests = append(tests, &testCase{_id: "nothing-declared", _expected: "", hasEval: false, skill: provideCompleteSkill()})

	// eval-only
	tests = append(tests, &testCase{_id: "eval-only", _expected: "Eval: /skillroutine-eval demo\n", hasEval: true, skill: provideCompleteSkill()})

	// skip
	skipped := provideCompleteSkill()
	skipped.Smoke = &Smoke{Skip: "acts on a live fleet"}
	tests = append(tests, &testCase{_id: "skip", _expected: "Smoke: skipped — acts on a live fleet\n", hasEval: false, skill: skipped})

	// nightly-run-and-eval
	tests = append(tests, &testCase{_id: "nightly-run-and-eval", _expected: "Smoke: make skill-smoke SKILL=demo TOKEN=<label>\nEval: /skillroutine-eval demo\n", hasEval: true, skill: provideSmokedSkill()})

	// interactive-run
	interactive := provideSmokedSkill()
	interactive.Smoke.Mode = SmokeModeInteractive
	tests = append(tests, &testCase{_id: "interactive-run", _expected: "Smoke (interactive): run /demo task: x in a session, then make skill-smoke-verdict SKILL=demo SESSION=<id>\n", hasEval: false, skill: interactive})

	// interactive-empty-invoke
	bare := provideSmokedSkill()
	bare.Smoke.Invoke = &empty
	bare.Smoke.Mode = SmokeModeInteractive
	tests = append(tests, &testCase{_id: "interactive-empty-invoke", _expected: "Smoke (interactive): run /demo in a session, then make skill-smoke-verdict SKILL=demo SESSION=<id>\n", hasEval: false, skill: bare})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			rendered := renderHelpTest(test.hasEval, test.skill)

			assert.Equal(t, test._expected, string(rendered))
		})
	}
}

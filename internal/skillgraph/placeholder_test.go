package skillgraph

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kevinhorst/smine/internal/skillenv"
)

const (
	checkGuard = `if (ENVIRON["SKILL_DEMO_CHECK"] == "") { print "SKILL_DEMO_CHECK: not set - runs everything before a commit, e.g. go vet ./... - export it"; exit 2 }; `
	buildGuard = `if (ENVIRON["SKILL_DEMO_BUILD"] == "") exit 0; `
	testGuard  = `if (ENVIRON["SKILL_DEMO_TEST"] == "") exit 0; `
)

// provideDeferral defers the demo skill's CHECK (required), BUILD and TEST (optional) with LANES filled at deploy.
func provideDeferral() *deferral {
	skill := provideCompleteSkill()
	bindEnv(skill, map[string]*Variable{
		"BUILD": {Interface: "build@1"},
		"CHECK": {Interface: "check@1", Required: true},
		"LANES": {Interface: "lanes@1"},
		"TEST":  {Interface: "test@1"},
	})
	names := map[string]bool{"SKILL_DEMO_BUILD": true, "SKILL_DEMO_CHECK": true, "SKILL_DEMO_TEST": true}
	values := map[string]string{"SKILL_DEMO_LANES": "smine-rules"}
	return newDeferral("export it", names, skill, values)
}

// provideBlock is the fenced block of one awk program: the guards, then the printf over the pieces.
func provideBlock(guards, pieces string) []string {
	return []string{"```!", "awk 'BEGIN { " + guards + `printf "%s", ` + pieces + " }'", "```"}
}

// provideBlockCommand runs the rendered line of lines through sh with env as the whole environment beside PATH.
func provideBlockCommand(t *testing.T, env []string, lines []string) *exec.Cmd {
	t.Helper()
	line := provideDeferral().block(lines)[1]
	command := exec.CommandContext(t.Context(), "sh", "-c", line)
	command.Env = append([]string{"PATH=" + os.Getenv("PATH")}, env...)
	return command
}

// commandExit is the exit code behind err: 0 for nil, the child's code for an exit error, -1 for any other failure.
func commandExit(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, isExit := errors.AsType[*exec.ExitError](err); isExit {
		return exitErr.ExitCode()
	}
	return -1
}

func TestDeferralBlock(t *testing.T) {
	type testCase struct {
		_id       string
		_expected []string

		lines []string
	}

	deferred := provideDeferral()
	tests := make([]*testCase, 0)

	// required-entry-wrapped
	tests = append(tests, &testCase{
		_id:       "required-entry-wrapped",
		_expected: provideBlock(checkGuard, `"\052\052X\052\052 \140\133step\135\140 \052\052Y\052\052\012\012RUN \140" ENVIRON["SKILL_DEMO_CHECK"] "\140."`),

		lines: []string{"**X** `[step]` **Y**", "", "RUN `$SKILL_DEMO_CHECK`."},
	})

	// optional-item-guarded
	tests = append(tests, &testCase{
		_id:       "optional-item-guarded",
		_expected: provideBlock(testGuard, `"\052 tests: \140" ENVIRON["SKILL_DEMO_TEST"] "\140."`),

		lines: []string{"* tests: `$SKILL_DEMO_TEST`."},
	})

	// two-optionals-guarded-in-order
	tests = append(tests, &testCase{
		_id:       "two-optionals-guarded-in-order",
		_expected: provideBlock(buildGuard+testGuard, `"\052 \140" ENVIRON["SKILL_DEMO_BUILD"] "\140 then \140" ENVIRON["SKILL_DEMO_TEST"] "\140."`),

		lines: []string{"* `$SKILL_DEMO_BUILD` then `$SKILL_DEMO_TEST`."},
	})

	// required-guard-first
	tests = append(tests, &testCase{
		_id:       "required-guard-first",
		_expected: provideBlock(checkGuard+testGuard, `"\052 \140" ENVIRON["SKILL_DEMO_TEST"] "\140 after \140" ENVIRON["SKILL_DEMO_CHECK"] "\140."`),

		lines: []string{"* `$SKILL_DEMO_TEST` after `$SKILL_DEMO_CHECK`."},
	})

	// punctuation-octal-escaped
	tests = append(tests, &testCase{
		_id:       "punctuation-octal-escaped",
		_expected: provideBlock(checkGuard, `"RUN \140" ENVIRON["SKILL_DEMO_CHECK"] "\140 with \042x\042 and \044HOME\073 a \134 backslash \173as is\175 \050it\047s\051."`),

		lines: []string{"RUN `$SKILL_DEMO_CHECK` with \"x\" and $HOME; a \\ backslash {as is} (it's)."},
	})

	// filled-reference-escaped
	tests = append(tests, &testCase{
		_id:       "filled-reference-escaped",
		_expected: provideBlock(testGuard, `"\052 \140" ENVIRON["SKILL_DEMO_TEST"] "\140 over \173a\175 and \140smine-rules lanes\140."`),

		lines: []string{"* `$SKILL_DEMO_TEST` over {a} and `$SKILL_DEMO_LANES lanes`."},
	})

	// line-break-as-escape
	tests = append(tests, &testCase{
		_id:       "line-break-as-escape",
		_expected: provideBlock(checkGuard, `"a \140" ENVIRON["SKILL_DEMO_CHECK"] "\140\012b"`),

		lines: []string{"a `$SKILL_DEMO_CHECK`", "b"},
	})

	// multibyte-runes-octal-escaped
	tests = append(tests, &testCase{
		_id:       "multibyte-runes-octal-escaped",
		_expected: provideBlock(checkGuard, `"RUN \140" ENVIRON["SKILL_DEMO_CHECK"] "\140 \342\200\224 \303\266lt."`),

		lines: []string{"RUN `$SKILL_DEMO_CHECK` — ölt."},
	})

	// unset-install-reference-escaped
	tests = append(tests, &testCase{
		_id:       "unset-install-reference-escaped",
		_expected: provideBlock(checkGuard, `"RUN \140" ENVIRON["SKILL_DEMO_CHECK"] "\140 and \140\044SKILL_DEMO_PLAN_SECTIONS plan-sections\140."`),

		lines: []string{"RUN `$SKILL_DEMO_CHECK` and `$SKILL_DEMO_PLAN_SECTIONS plan-sections`."},
	})

	// reference-only-no-text-piece
	tests = append(tests, &testCase{
		_id:       "reference-only-no-text-piece",
		_expected: provideBlock(checkGuard, `ENVIRON["SKILL_DEMO_CHECK"]`),

		lines: []string{"$SKILL_DEMO_CHECK"},
	})

	// no-deferred-untouched
	tests = append(tests, &testCase{
		_id:       "no-deferred-untouched",
		_expected: []string{"RUN `$SKILL_DEMO_LANES lanes`."},

		lines: []string{"RUN `$SKILL_DEMO_LANES lanes`."},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, deferred.block(test.lines))
		})
	}
}

func TestDeferralBlock_runs(t *testing.T) {
	type testCase struct {
		_id           string
		_expectedExit int
		_expectedOut  string

		command *exec.Cmd
	}

	if runtime.GOOS == "windows" {
		t.Skip("the block runs through sh and awk, which CI's Windows job lacks")
	}
	tests := make([]*testCase, 0)

	// required-set-prints-text
	tests = append(tests, &testCase{
		_id:           "required-set-prints-text",
		_expectedExit: 0,
		_expectedOut:  "RUN `go vet ./...` — ölt \"x\" {a} (it's).\nb",

		command: provideBlockCommand(t, []string{"SKILL_DEMO_CHECK=go vet ./..."}, []string{"RUN `$SKILL_DEMO_CHECK` — ölt \"x\" {a} (it's).", "b"}),
	})

	// required-unset-aborts
	tests = append(tests, &testCase{
		_id:           "required-unset-aborts",
		_expectedExit: placeholderAbortExit,
		_expectedOut:  "SKILL_DEMO_CHECK: not set - runs everything before a commit, e.g. go vet ./... - export it\n",

		command: provideBlockCommand(t, nil, []string{"RUN `$SKILL_DEMO_CHECK`."}),
	})

	// optional-unset-prints-nothing
	tests = append(tests, &testCase{
		_id:           "optional-unset-prints-nothing",
		_expectedExit: 0,
		_expectedOut:  "",

		command: provideBlockCommand(t, []string{"SKILL_DEMO_CHECK=go vet ./..."}, []string{"* `$SKILL_DEMO_TEST` after `$SKILL_DEMO_CHECK`."}),
	})

	// optional-set-prints-both
	tests = append(tests, &testCase{
		_id:           "optional-set-prints-both",
		_expectedExit: 0,
		_expectedOut:  "* `go test ./...` after `go vet ./...`.",

		command: provideBlockCommand(t, []string{"SKILL_DEMO_CHECK=go vet ./...", "SKILL_DEMO_TEST=go test ./..."}, []string{"* `$SKILL_DEMO_TEST` after `$SKILL_DEMO_CHECK`."}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			out, err := test.command.Output()

			assert.Equal(t, test._expectedExit, commandExit(err))
			assert.Equal(t, test._expectedOut, string(out))
		})
	}
}

// provideSummaryDeferral defers the demo skill's required CHECK, its catalog record carrying summary.
func provideSummaryDeferral(summary string) *deferral {
	skill := provideCompleteSkill()
	bindEnv(skill, map[string]*Variable{"CHECK": {Interface: "check@1", Required: true}})
	iface, _ := skill.Catalog.Interface("check@1")
	iface.Summary = summary
	return newDeferral("export it", map[string]bool{"SKILL_DEMO_CHECK": true}, skill, nil)
}

func TestDeferralAbortSentence(t *testing.T) {
	type testCase struct {
		_id       string
		_expected string

		deferred *deferral
	}

	tests := make([]*testCase, 0)

	// abort-sentence-shape
	tests = append(tests, &testCase{
		_id:       "abort-sentence-shape",
		_expected: "not set - runs everything before a commit, e.g. go vet ./... - export it",

		deferred: provideSummaryDeferral("Runs everything before a commit."),
	})

	// quotes-and-brackets-dropped
	tests = append(tests, &testCase{
		_id:       "quotes-and-brackets-dropped",
		_expected: "not set - runs the repo s gates see docs , e.g. go vet ./... - export it",

		deferred: provideSummaryDeferral("Runs the repo's gates; see (docs)."),
	})

	// non-ascii-dropped
	tests = append(tests, &testCase{
		_id:       "non-ascii-dropped",
		_expected: "not set - lt die Kette, e.g. go vet ./... - export it",

		deferred: provideSummaryDeferral("Ölt die Kette."),
	})

	// empty-summary
	tests = append(tests, &testCase{
		_id:       "empty-summary",
		_expected: "not set - , e.g. go vet ./... - export it",

		deferred: provideSummaryDeferral(""),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			assert.Equal(t, test._expected, test.deferred.abortSentence("SKILL_DEMO_CHECK"))
		})
	}
}

// provideEmittedDeferral runs block over lines first, so the deferral has recorded the names they read.
func provideEmittedDeferral(deferred *deferral, lines []string) *deferral {
	deferred.block(lines)
	return deferred
}

func TestDeferralRenderEnv(t *testing.T) {
	type testCase struct {
		_id       string
		_expected *skillenv.Document

		deferred *deferral
	}

	unknownSkill := provideCompleteSkill()
	bindEnv(unknownSkill, map[string]*Variable{"CHECK": {Interface: "unknown@1", Required: true}})
	unknownDeferral := newDeferral("export it", map[string]bool{"SKILL_DEMO_CHECK": true}, unknownSkill, nil)
	tests := make([]*testCase, 0)

	// emitted-names-only
	tests = append(tests, &testCase{
		_id: "emitted-names-only",
		_expected: &skillenv.Document{
			Schema:    skillenv.DocumentSchema,
			Skill:     "demo",
			Variables: []*skillenv.Entry{{Example: "go test ./...", Interface: "test", Name: "SKILL_DEMO_TEST", Required: false, Scope: ScopeRepo, Summary: "Runs the suite."}},
			Version:   "1.0",
		},

		deferred: provideEmittedDeferral(provideDeferral(), []string{"* tests: `$SKILL_DEMO_TEST`."}),
	})

	// required-summary-example-copied
	tests = append(tests, &testCase{
		_id: "required-summary-example-copied",
		_expected: &skillenv.Document{
			Schema:    skillenv.DocumentSchema,
			Skill:     "demo",
			Variables: []*skillenv.Entry{{Example: "go vet ./...", Interface: "check", Name: "SKILL_DEMO_CHECK", Required: true, Scope: ScopeRepo, Summary: "Runs everything before a commit."}},
			Version:   "1.0",
		},

		deferred: provideEmittedDeferral(provideDeferral(), []string{"RUN `$SKILL_DEMO_CHECK`."}),
	})

	// unknown-interface-empty-fields
	tests = append(tests, &testCase{
		_id: "unknown-interface-empty-fields",
		_expected: &skillenv.Document{
			Schema:    skillenv.DocumentSchema,
			Skill:     "demo",
			Variables: []*skillenv.Entry{{Example: "", Interface: "", Name: "SKILL_DEMO_CHECK", Required: true, Scope: "", Summary: ""}},
			Version:   "1.0",
		},

		deferred: provideEmittedDeferral(unknownDeferral, []string{"RUN `$SKILL_DEMO_CHECK`."}),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			content, err := test.deferred.renderEnv()
			document, parseErr := skillenv.ParseDocument(content)

			assert.NoError(t, err)
			assert.NoError(t, parseErr)
			assert.Equal(t, test._expected, document)
		})
	}
}

func TestNewDeferral(t *testing.T) {
	skill := provideCompleteSkill()
	bindEnv(skill, map[string]*Variable{"CHECK": {Interface: "check@1", Required: true}})

	assert.Nil(t, newDeferral("export it", nil, skill, nil), "nil-names-nil")
	assert.Nil(t, newDeferral("export it", map[string]bool{"SKILL_OTHER_CHECK": true}, skill, nil), "undeclared-names-nil")
	assert.NotNil(t, newDeferral("export it", map[string]bool{"SKILL_DEMO_CHECK": true}, skill, nil), "declared-name-defers")
}

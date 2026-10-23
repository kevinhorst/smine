package cmdline

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplit(t *testing.T) {
	type testCase struct {
		_id            string
		_expected      []string
		_expectedError string
		_shouldPass    bool

		line string
	}

	tests := make([]*testCase, 0)

	// pass-plain-words
	tests = append(tests, &testCase{
		_id:            "pass-plain-words",
		_expected:      []string{"make", "audit"},
		_expectedError: "",
		_shouldPass:    true,

		line: "make audit",
	})

	// pass-quoted-group
	tests = append(tests, &testCase{
		_id:            "pass-quoted-group",
		_expected:      []string{"go", "test", "-run", "A B", "./..."},
		_expectedError: "",
		_shouldPass:    true,

		line: `go test -run "A B" ./...`,
	})

	// pass-quoted-path
	tests = append(tests, &testCase{
		_id:            "pass-quoted-path",
		_expected:      []string{"spaced path/bin", "check"},
		_expectedError: "",
		_shouldPass:    true,

		line: `"spaced path/bin" check`,
	})

	// pass-escaped-quote
	tests = append(tests, &testCase{
		_id:            "pass-escaped-quote",
		_expected:      []string{"echo", `say "hi"`},
		_expectedError: "",
		_shouldPass:    true,

		line: `echo "say \"hi\""`,
	})

	// pass-extra-spaces
	tests = append(tests, &testCase{
		_id:            "pass-extra-spaces",
		_expected:      []string{"make", "test"},
		_expectedError: "",
		_shouldPass:    true,

		line: "  make   test  ",
	})

	// pass-empty-quotes-are-a-word
	tests = append(tests, &testCase{
		_id:            "pass-empty-quotes-are-a-word",
		_expected:      []string{"run", ""},
		_expectedError: "",
		_shouldPass:    true,

		line: `run ""`,
	})

	// pass-empty-line
	tests = append(tests, &testCase{
		_id:            "pass-empty-line",
		_expected:      nil,
		_expectedError: "",
		_shouldPass:    true,

		line: "",
	})

	// fail-and
	tests = append(tests, &testCase{
		_id:            "fail-and",
		_expected:      nil,
		_expectedError: `cmdline.Split: Shell operator "&&" in "make build && make test"`,
		_shouldPass:    false,

		line: "make build && make test",
	})

	// fail-or-named-before-pipe
	tests = append(tests, &testCase{
		_id:            "fail-or-named-before-pipe",
		_expected:      nil,
		_expectedError: `cmdline.Split: Shell operator "||" in "a || b"`,
		_shouldPass:    false,

		line: "a || b",
	})

	// fail-pipe
	tests = append(tests, &testCase{
		_id:            "fail-pipe",
		_expected:      nil,
		_expectedError: `cmdline.Split: Shell operator "|" in "a | b"`,
		_shouldPass:    false,

		line: "a | b",
	})

	// fail-semicolon
	tests = append(tests, &testCase{
		_id:            "fail-semicolon",
		_expected:      nil,
		_expectedError: `cmdline.Split: Shell operator ";" in "a; b"`,
		_shouldPass:    false,

		line: "a; b",
	})

	// fail-substitution
	tests = append(tests, &testCase{
		_id:            "fail-substitution",
		_expected:      nil,
		_expectedError: `cmdline.Split: Shell operator "$(" in "echo $(x)"`,
		_shouldPass:    false,

		line: "echo $(x)",
	})

	// fail-backtick
	tests = append(tests, &testCase{
		_id:            "fail-backtick",
		_expected:      nil,
		_expectedError: "cmdline.Split: Backtick in a command value",
		_shouldPass:    false,

		line: "echo `x`",
	})

	// fail-newline
	tests = append(tests, &testCase{
		_id:            "fail-newline",
		_expected:      nil,
		_expectedError: "cmdline.Split: Newline in a command value",
		_shouldPass:    false,

		line: "make\naudit",
	})

	// fail-unbalanced-quote
	tests = append(tests, &testCase{
		_id:            "fail-unbalanced-quote",
		_expected:      nil,
		_expectedError: `cmdline.Split: Unbalanced quote in "go test -run \"A"`,
		_shouldPass:    false,

		line: `go test -run "A`,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			words, err := Split(test.line)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, words)
			assert.Equal(t, test._expectedError, errorText(err))
		})
	}
}

func TestSplitLines(t *testing.T) {
	type testCase struct {
		_id            string
		_expected      [][]string
		_expectedError string
		_shouldPass    bool

		value string
	}

	tests := make([]*testCase, 0)

	// one-line
	tests = append(tests, &testCase{
		_id:            "one-line",
		_expected:      [][]string{{"make", "audit"}},
		_expectedError: "",
		_shouldPass:    true,

		value: "make audit",
	})

	// two-lines
	tests = append(tests, &testCase{
		_id:            "two-lines",
		_expected:      [][]string{{"python", "manage.py", "check"}, {"pytest"}},
		_expectedError: "",
		_shouldPass:    true,

		value: "python manage.py check\npytest",
	})

	// blank-lines-skipped
	tests = append(tests, &testCase{
		_id:            "blank-lines-skipped",
		_expected:      [][]string{{"make", "build"}, {"make", "test"}},
		_expectedError: "",
		_shouldPass:    true,

		value: "\nmake build\n\n   \nmake test",
	})

	// trailing-newline
	tests = append(tests, &testCase{
		_id:            "trailing-newline",
		_expected:      [][]string{{"make", "audit"}},
		_expectedError: "",
		_shouldPass:    true,

		value: "make audit\n",
	})

	// operator-in-second-line-refused
	tests = append(tests, &testCase{
		_id:            "operator-in-second-line-refused",
		_expected:      nil,
		_expectedError: `cmdline.SplitLines: cmdline.Split: Shell operator "|" in "a | b"`,
		_shouldPass:    false,

		value: "make build\na | b",
	})

	// empty-refused
	tests = append(tests, &testCase{
		_id:            "empty-refused",
		_expected:      nil,
		_expectedError: "cmdline.SplitLines: No command line in the value",
		_shouldPass:    false,

		value: "",
	})

	// whitespace-only-refused
	tests = append(tests, &testCase{
		_id:            "whitespace-only-refused",
		_expected:      nil,
		_expectedError: "cmdline.SplitLines: No command line in the value",
		_shouldPass:    false,

		value: "  \n\t\n ",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			lines, err := SplitLines(test.value)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, lines)
			assert.Equal(t, test._expectedError, errorText(err))
		})
	}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

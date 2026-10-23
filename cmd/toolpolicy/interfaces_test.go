package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunInterfaces(t *testing.T) {
	type testCase struct {
		_id             string
		_expectedCode   int
		_expectedError  string
		_expectedStdout string

		args []string
	}

	tests := make([]*testCase, 0)

	// interfaces-without-policy
	tests = append(tests, &testCase{
		_id:             "interfaces-without-policy",
		_expectedCode:   0,
		_expectedError:  "",
		_expectedStdout: `{"interfaces":["permission-check@1","permission-freeze@1"]}` + "\n",

		args: []string{"interfaces"},
	})

	// interfaces-extra-arg
	tests = append(tests, &testCase{
		_id:             "interfaces-extra-arg",
		_expectedCode:   usageExitCode,
		_expectedError:  "usage: toolpolicy interfaces\n",
		_expectedStdout: "",

		args: []string{"interfaces", "x"},
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			stdout, stderr := &strings.Builder{}, &strings.Builder{}
			code := run(test.args, strings.NewReader(""), stdout, stderr)
			assert.Equal(t, test._expectedCode, code)
			assert.Equal(t, test._expectedStdout, stdout.String())
			assert.Contains(t, stderr.String(), test._expectedError)
		})
	}
}

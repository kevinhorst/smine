package install

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasPayload(t *testing.T) {
	repo := t.TempDir()
	assert.False(t, HasPayload(repo))

	writeTestFile(t, repo, "bin/"+PayloadMarker, "2.0.0\n")
	assert.True(t, HasPayload(repo))
}

func TestBinary_Source(t *testing.T) {
	type testCase struct {
		_id              string
		_expectedContent string
		_shouldPass      bool

		binary binary
		env    Env
	}

	t.Setenv("PATH", "")
	tests := make([]*testCase, 0)

	// payload-prebuilt
	env := newTestEnv(t)
	writeTestFile(t, env.RepoDir, "bin/toolpolicy", "toolpolicy binary")
	tests = append(tests, &testCase{
		_id:              "payload-prebuilt",
		_expectedContent: "toolpolicy binary",
		_shouldPass:      true,

		binary: binary{kind: binaryHook, name: "toolpolicy"},
		env:    env,
	})

	// payload-missing-binary
	env = newTestEnv(t)
	tests = append(tests, &testCase{
		_id:              "payload-missing-binary",
		_expectedContent: "",
		_shouldPass:      false,

		binary: binary{kind: binaryHook, name: "toolpolicy"},
		env:    env,
	})

	// rust-prebuilt-without-payload
	env = newTestEnv(t)
	env.IsPayload = false
	writeTestFile(t, env.RepoDir, "bin/agentwt", "agentwt binary")
	tests = append(tests, &testCase{
		_id:              "rust-prebuilt-without-payload",
		_expectedContent: "agentwt binary",
		_shouldPass:      true,

		binary: binary{isRust: true, kind: binaryPath, name: "agentwt"},
		env:    env,
	})

	// development-checkout-needs-go
	env = newTestEnv(t)
	env.IsPayload = false
	writeTestFile(t, env.RepoDir, "bin/toolpolicy", "toolpolicy binary")
	tests = append(tests, &testCase{
		_id:              "development-checkout-needs-go",
		_expectedContent: "",
		_shouldPass:      false,

		binary: binary{kind: binaryHook, name: "toolpolicy"},
		env:    env,
	})

	// windows-payload-exe
	env = newTestEnv(t)
	env.GOOS = goosWindows
	writeTestFile(t, env.RepoDir, "bin/toolpolicy.exe", "toolpolicy exe")
	tests = append(tests, &testCase{
		_id:              "windows-payload-exe",
		_expectedContent: "toolpolicy exe",
		_shouldPass:      true,

		binary: binary{kind: binaryHook, name: "toolpolicy"},
		env:    env,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			data, err := test.binary.source(context.Background(), test.env, t.TempDir())

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expectedContent, string(data))
		})
	}
}

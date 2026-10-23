package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDemoValue(t *testing.T) {
	type testCase struct {
		_id        string
		_isSameAs  bool
		_otherSeed string
		_otherSalt string

		salt string
		seed string
	}

	tests := make([]*testCase, 0)

	// same-seed-and-salt
	tests = append(tests, &testCase{
		_id:        "same-seed-and-salt",
		_isSameAs:  true,
		_otherSeed: "s1",
		_otherSalt: "run",

		salt: "run",
		seed: "s1",
	})

	// other-salt
	tests = append(tests, &testCase{
		_id:        "other-salt",
		_isSameAs:  false,
		_otherSeed: "s1",
		_otherSalt: "other",

		salt: "run",
		seed: "s1",
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			value := demoValue(test.seed, test.salt)

			assert.Len(t, value, demoValueLength)
			assert.Equal(t, test._isSameAs, value == demoValue(test._otherSeed, test._otherSalt))
		})
	}
}

func TestRunDemo_Usage(t *testing.T) {
	assert.Equal(t, exitError, runDemoValue(nil))
	assert.Equal(t, exitError, runDemoClose(nil))
}

func TestRunDemoClose(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "never-staged")

	code := runDemoClose([]string{"--dir", dir})

	assert.Equal(t, exitClean, code)
	assert.FileExists(t, filepath.Join(dir, demoClosedFile))
	content, err := os.ReadFile(filepath.Join(dir, demoClosedFile))
	require.NoError(t, err)
	assert.Contains(t, string(content), "closed")
}

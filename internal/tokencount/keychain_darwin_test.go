//go:build darwin

package tokencount

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const stubSecretOutput = "SECRET-OUTPUT"

// useSecurityBinary points securityBinary at path for the running test and
// restores the previous value when it ends.
func useSecurityBinary(t *testing.T, path string) {
	t.Helper()

	previous := securityBinary
	securityBinary = path
	t.Cleanup(func() { securityBinary = previous })
}

// writeSecurityStub writes an executable stand-in for /usr/bin/security that
// exits 99 unless invoked with exactly the find-generic-password arguments,
// then runs body.
func writeSecurityStub(t *testing.T, body string) string {
	t.Helper()

	script := "#!/bin/sh\n" +
		"if [ \"$#\" -ne 6 ] || [ \"$*\" != \"find-generic-password -s com.smine.configserver -a anthropic-api-key -w\" ]; then\n" +
		"  exit 99\n" +
		"fi\n" +
		body + "\n"
	path := filepath.Join(t.TempDir(), "security")
	require.NoError(t, os.WriteFile(path, []byte(script), 0o700))
	return path
}

func TestReadKeychainKey(t *testing.T) {
	type testCase struct {
		_id                string
		_expectedErrorText string
		_expectedKey       string
		_isMissing         bool
		_shouldPass        bool

		securityBinary string
	}

	tests := make([]*testCase, 0)

	// value-trimmed
	tests = append(tests, &testCase{
		_id:                "value-trimmed",
		_expectedErrorText: "",
		_expectedKey:       "sk-test-value",
		_isMissing:         false,
		_shouldPass:        true,

		securityBinary: writeSecurityStub(t, "printf '  sk-test-value\\n'\nexit 0"),
	})

	// item-missing
	tests = append(tests, &testCase{
		_id:                "item-missing",
		_expectedErrorText: "No keychain item",
		_expectedKey:       "",
		_isMissing:         true,
		_shouldPass:        false,

		securityBinary: writeSecurityStub(t, "exit 44"),
	})

	// exit-failure-hides-output
	tests = append(tests, &testCase{
		_id:                "exit-failure-hides-output",
		_expectedErrorText: "security find-generic-password failed",
		_expectedKey:       "",
		_isMissing:         false,
		_shouldPass:        false,

		securityBinary: writeSecurityStub(t, "printf '"+stubSecretOutput+"'\nexit 1"),
	})

	// empty-output
	tests = append(tests, &testCase{
		_id:                "empty-output",
		_expectedErrorText: "empty",
		_expectedKey:       "",
		_isMissing:         false,
		_shouldPass:        false,

		securityBinary: writeSecurityStub(t, "exit 0"),
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			useSecurityBinary(t, test.securityBinary)
			key, err := readKeychainKey(context.Background())
			assert.Equal(t, test._expectedKey, key)
			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._isMissing, errors.Is(err, errKeychainMissing))
			assert.Contains(t, fmt.Sprint(err), test._expectedErrorText)
			assert.NotContains(t, fmt.Sprint(err), stubSecretOutput)
		})
	}
}

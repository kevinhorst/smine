//go:build windows

package tokencount

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadKeychainKey_Windows(t *testing.T) {
	type testCase struct {
		_id         string
		_expected   string
		_isMissing  bool
		_shouldPass bool

		blob []byte
		err  error
	}

	tests := make([]*testCase, 0)

	// utf16-value-trimmed
	tests = append(tests, &testCase{
		_id:         "utf16-value-trimmed",
		_expected:   "sk-test-value",
		_isMissing:  false,
		_shouldPass: true,

		blob: []byte{' ', 0, 's', 0, 'k', 0, '-', 0, 't', 0, 'e', 0, 's', 0, 't', 0, '-', 0, 'v', 0, 'a', 0, 'l', 0, 'u', 0, 'e', 0, ' ', 0},
		err:  nil,
	})

	// entry-missing
	tests = append(tests, &testCase{
		_id:         "entry-missing",
		_expected:   "",
		_isMissing:  true,
		_shouldPass: false,

		blob: nil,
		err:  errKeychainMissing,
	})

	// entry-empty
	tests = append(tests, &testCase{
		_id:         "entry-empty",
		_expected:   "",
		_isMissing:  false,
		_shouldPass: false,

		blob: []byte{' ', 0},
		err:  nil,
	})

	// Run tests
	for _, test := range tests {
		t.Run(test._id, func(t *testing.T) {
			previous := readCredentialBlob
			readCredentialBlob = func(string) ([]byte, error) { return test.blob, test.err }
			t.Cleanup(func() { readCredentialBlob = previous })

			key, err := readKeychainKey(context.Background())

			assert.Equalf(t, test._shouldPass, err == nil, "err = %v", err)
			assert.Equal(t, test._expected, key)
			assert.Equal(t, test._isMissing, errors.Is(err, errKeychainMissing))
		})
	}
}

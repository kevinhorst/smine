//go:build !darwin && !windows

package tokencount

import (
	"context"
	"errors"
)

const KeychainSupported = false

func readKeychainKey(ctx context.Context) (string, error) {
	return "", errors.New("readKeychainKey: no credential store on this OS (macOS Keychain, Windows Credential Manager)")
}

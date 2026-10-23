//go:build darwin

package tokencount

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	keychainAccount     = "anthropic-api-key"
	keychainMissingExit = 44
	keychainService     = "com.smine.configserver"
	keychainTimeout     = 10 * time.Second
)

const KeychainSupported = true

// securityBinary is Apple's keychain CLI; a var so tests can point it at a stub.
var securityBinary = "/usr/bin/security"

var errKeychainMissing = errors.New("readKeychainKey: No keychain item " + keychainService + " / " + keychainAccount +
	" — add it with: security add-generic-password -s " + keychainService + " -a " + keychainAccount + " -w")

// readKeychainKey never logs the key or puts it in an error.
func readKeychainKey(ctx context.Context) (string, error) {
	runCtx, cancel := context.WithTimeout(ctx, keychainTimeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, securityBinary, "find-generic-password", "-s", keychainService, "-a", keychainAccount, "-w")
	output, err := cmd.Output()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == keychainMissingExit {
		return "", errKeychainMissing
	}
	if err != nil {
		return "", fmt.Errorf("readKeychainKey: security find-generic-password failed: %w", err)
	}

	key := strings.TrimSpace(string(output))
	if key == "" {
		return "", errors.New("readKeychainKey: Keychain item is empty")
	}
	return key, nil
}

//go:build windows

package tokencount

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	credTarget      = "com.smine.configserver/anthropic-api-key"
	credTypeGeneric = 1
	credNotFound    = windows.Errno(1168)
)

const KeychainSupported = true

var (
	advapi32 = windows.NewLazySystemDLL("advapi32.dll")
	credRead = advapi32.NewProc("CredReadW")
	credFree = advapi32.NewProc("CredFree")
)

// readCredentialBlob is the Credential Manager call; a var so tests can point it at a stub.
var readCredentialBlob = credReadBlob

var errKeychainMissing = errors.New("readKeychainKey: No Credential Manager entry " + credTarget +
	" — add it with: cmdkey /generic:" + credTarget + " /user:anthropic-api-key /pass:<key>")

// credential mirrors CREDENTIALW up to the fields read here; the trailing attribute and name fields are never touched.
type credential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

// readKeychainKey never logs the key or puts it in an error.
func readKeychainKey(ctx context.Context) (string, error) {
	blob, err := readCredentialBlob(credTarget)
	if err != nil {
		return "", err
	}

	key := strings.TrimSpace(decodeUtf16(blob))
	if key == "" {
		return "", errors.New("readKeychainKey: Credential Manager entry is empty")
	}
	return key, nil
}

// credReadBlob copies the blob out before CredFree; ERROR_NOT_FOUND is the missing-entry case.
func credReadBlob(target string) ([]byte, error) {
	name, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return nil, fmt.Errorf("credReadBlob: %w", err)
	}

	var cred *credential
	status, _, callErr := credRead.Call(uintptr(unsafe.Pointer(name)), credTypeGeneric, 0, uintptr(unsafe.Pointer(&cred)))
	if status == 0 {
		if errors.Is(callErr, credNotFound) {
			return nil, errKeychainMissing
		}
		return nil, fmt.Errorf("credReadBlob: CredRead failed: %w", callErr)
	}
	defer credFree.Call(uintptr(unsafe.Pointer(cred)))

	return append([]byte(nil), unsafe.Slice(cred.CredentialBlob, cred.CredentialBlobSize)...), nil
}

func decodeUtf16(blob []byte) string {
	units := make([]uint16, len(blob)/2)
	for i := range units {
		units[i] = uint16(blob[2*i]) | uint16(blob[2*i+1])<<8
	}
	return windows.UTF16ToString(units)
}

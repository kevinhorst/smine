package server

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CredentialRealm is how the Windows installer recognizes a serving instance.
const CredentialRealm = "smine config server"

const (
	credentialDirMode  = 0o700
	credentialFileMode = 0o600
)

// Credential holds only the token's digest and follows its file, so -new-credential applies on the next request.
type Credential struct {
	mu      sync.Mutex
	digest  [sha256.Size]byte
	modTime time.Time
	path    string // empty for an in-memory credential
}

// LoadCredential reads the hex digest GenerateCredential stored at path.
func LoadCredential(path string) (*Credential, error) {
	digest, modTime, err := readDigest(path)
	if err != nil {
		return nil, fmt.Errorf("LoadCredential: %w", err)
	}

	credential := &Credential{digest: digest, modTime: modTime, path: path}
	return credential, nil
}

// NewCredential builds the credential that accepts token.
func NewCredential(token string) *Credential {
	return &Credential{digest: sha256.Sum256([]byte(token))}
}

// current re-reads the digest when the file's mtime moved; an unreadable or invalid file keeps the last valid digest.
func (c *Credential) current() [sha256.Size]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.path == "" {
		return c.digest
	}

	info, err := os.Stat(c.path)
	if err != nil || info.ModTime().Equal(c.modTime) {
		return c.digest
	}

	digest, modTime, err := readDigest(c.path)
	c.modTime = info.ModTime()
	if err != nil {
		log.Printf("Credential.current: Keeping the previous credential: %v", err)
		return c.digest
	}

	c.digest, c.modTime = digest, modTime
	return c.digest
}

func readDigest(path string) ([sha256.Size]byte, time.Time, error) {
	var digest [sha256.Size]byte
	info, err := os.Stat(path)
	if err != nil {
		return digest, time.Time{}, fmt.Errorf("readDigest: %w", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return digest, time.Time{}, fmt.Errorf("readDigest: %w", err)
	}

	decoded, err := hex.DecodeString(strings.TrimSpace(string(content)))
	if err != nil {
		return digest, time.Time{}, fmt.Errorf("readDigest: %s: Digest is not hex: %w", path, err)
	}

	if len(decoded) != sha256.Size {
		return digest, time.Time{}, fmt.Errorf("readDigest: %s: Digest has %d bytes, want %d", path, len(decoded), sha256.Size)
	}

	copy(digest[:], decoded)
	return digest, info.ModTime(), nil
}

// Require ignores the Basic user name; only the password is checked.
func (c *Credential) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !c.isValidRequest(r) {
			w.Header().Set("WWW-Authenticate", `Basic realm="`+CredentialRealm+`", charset="UTF-8"`)
			http.Error(w, "credential required", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (c *Credential) isValidRequest(r *http.Request) bool {
	_, password, ok := r.BasicAuth()
	if !ok {
		return false
	}

	presented := sha256.Sum256([]byte(password))
	expected := c.current()
	return subtle.ConstantTimeCompare(presented[:], expected[:]) == 1
}

// GenerateCredential never writes the token itself, only its digest.
func GenerateCredential(path string) (string, error) {
	token := rand.Text()
	credential := NewCredential(token)

	if err := os.MkdirAll(filepath.Dir(path), credentialDirMode); err != nil {
		return "", fmt.Errorf("GenerateCredential: %w", err)
	}

	if err := os.WriteFile(path, []byte(hex.EncodeToString(credential.digest[:])+"\n"), credentialFileMode); err != nil {
		return "", fmt.Errorf("GenerateCredential: %w", err)
	}

	return token, nil
}

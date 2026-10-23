package desktop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

const (
	// The Desktop exports to a session's hooks the id its session record is named after.
	HostSessionEnv = "CLAUDE_CODE_HOST_SESSION_ID"

	destinationSession   = "session"
	sessionStoreName     = "Claude/claude-code-sessions"
	updateAddDirectories = "addDirectories"
)

var hostSessionPattern = regexp.MustCompile(`^local_[0-9a-f-]+$`)

// A folder the user added, or a grant given through a prompt.
type PermissionUpdate struct {
	Destination string   `json:"destination"`
	Directories []string `json:"directories"`
	Type        string   `json:"type"`
}

type Session struct {
	PermissionUpdates []PermissionUpdate `json:"sessionPermissionUpdates"`
	Title             string             `json:"title"`
	TitleSource       string             `json:"titleSource"`
}

func (s *Session) AddedDirectories() []string {
	directories := make([]string, 0)
	for _, update := range s.PermissionUpdates {
		isSessionDirectory := update.Type == updateAddDirectories && update.Destination == destinationSession
		if isSessionDirectory {
			directories = append(directories, update.Directories...)
		}
	}
	return directories
}

// The shape check keeps an id from addressing a path outside the session store.
func IsHostSessionId(id string) bool {
	return hostSessionPattern.MatchString(id)
}

func ReadSession(hostSessionId string, sessionsDir string) (*Session, error) {
	if !IsHostSessionId(hostSessionId) {
		return nil, fmt.Errorf("desktop.ReadSession: Invalid host session id %q", hostSessionId)
	}

	pattern := filepath.Join(sessionsDir, "*", "*", hostSessionId+".json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("desktop.ReadSession: Invalid pattern %s: %w", pattern, err)
	}
	if len(matches) != 1 {
		return nil, fmt.Errorf("desktop.ReadSession: Expected one store file for %s, found %d", hostSessionId, len(matches))
	}

	raw, err := os.ReadFile(matches[0])
	if err != nil {
		return nil, fmt.Errorf("desktop.ReadSession: Failed to read %s: %w", matches[0], err)
	}

	session := &Session{}
	if err := json.Unmarshal(raw, session); err != nil {
		return nil, fmt.Errorf("desktop.ReadSession: Failed to decode %s: %w", matches[0], err)
	}
	return session, nil
}

// SessionStoreDir is the session store under the user config dir.
func SessionStoreDir(configDir string) string {
	return filepath.Join(configDir, filepath.FromSlash(sessionStoreName))
}

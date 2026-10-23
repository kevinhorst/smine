package main

import (
	"github.com/kevinhorst/smine/internal/desktop"
)

func sessionDirectories(hostSessionId string, sessionsDir string) []string {
	if hostSessionId == "" {
		return nil
	}

	session, err := desktop.ReadSession(hostSessionId, sessionsDir)
	if err != nil {
		return nil
	}
	return session.AddedDirectories()
}

package main

import (
	"path/filepath"

	"github.com/kevinhorst/smine/internal/sessionstate"
)

func statePath(ledgerPath string, sessionId string) string {
	if ledgerPath == "" || sessionId == "" {
		return ""
	}
	return sessionstate.File(filepath.Join(filepath.Dir(ledgerPath), sessionstate.DirName), sessionId)
}

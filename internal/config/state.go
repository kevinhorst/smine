package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	StateInstallFile  = "install.json"
	StateRegistryFile = "repos.json"
	StoreBacklogDir   = "backlog"
	StoreProposalsDir = "proposals"

	// Set by the skilleval arm runner so a confined headless session never resolves the live stores.
	StoreRootEnv = "SMINE_STORE_ROOT"
	// Set by the smoke runner so the skillenv hook exports nothing: a smoke session carries the smoke's own values only.
	SkillEnvOffEnv = "SMINE_SKILLENV_OFF"

	stateDirName       = "smine"
	storeArchiveName   = "agent-plans"
	storeArchiveParent = "GolandProjects"
)

// The same path from every checkout, writable only by the operator's processes.
func StateDir(claudeHomeOverride string) (string, error) {
	claudeHome, err := ClaudeHomeDir(claudeHomeOverride)
	if err != nil {
		return "", fmt.Errorf("StateDir: %w", err)
	}
	return filepath.Join(claudeHome, stateDirName), nil
}

// StoreArchiveDir returns the plans-archive repo holding every repo's stores.
func StoreArchiveDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("StoreArchiveDir: Failed to resolve home: %w", err)
	}
	return filepath.Join(home, storeArchiveParent, storeArchiveName), nil
}

// The plans-archive dir is named after the main checkout's directory.
func StoreRoot(mainCheckout string) (string, error) {
	if override := os.Getenv(StoreRootEnv); override != "" {
		if !filepath.IsAbs(override) {
			return "", fmt.Errorf("StoreRoot: Invalid %s %q: Must be absolute", StoreRootEnv, override)
		}
		return filepath.Clean(override), nil
	}

	if !filepath.IsAbs(mainCheckout) {
		return "", fmt.Errorf("StoreRoot: Invalid main checkout %q: Must be absolute", mainCheckout)
	}

	repoName := filepath.Base(mainCheckout)
	if repoName == string(filepath.Separator) {
		return "", fmt.Errorf("StoreRoot: Invalid main checkout %q: Names no repo dir", mainCheckout)
	}

	archive, err := StoreArchiveDir()
	if err != nil {
		return "", fmt.Errorf("StoreRoot: %w", err)
	}
	return filepath.Join(archive, repoName), nil
}

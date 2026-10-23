package config

import (
	"os"
	"path/filepath"
)

// SmineBinDir is where the install puts the smine-* PATH binaries: ~/go/bin, or on windows the shim dir the install registers on the user PATH (LOCALAPPDATA, else the home's AppData\Local).
func SmineBinDir(goos, home string) string {
	if goos != "windows" {
		return filepath.Join(home, "go", "bin")
	}

	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		local = filepath.Join(home, "AppData", "Local")
	}
	return filepath.Join(local, "smine", "bin")
}

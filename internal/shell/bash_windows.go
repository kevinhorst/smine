//go:build windows

package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"golang.org/x/sys/windows/registry"
)

var bashOnce sync.Once
var bashResolved string

// BashPath locates Git for Windows bash.exe: SMINE_BASH override, the
// GitForWindows registry InstallPath (HKLM, HKCU, WOW6432Node), known install
// dirs, then git.exe's sibling bin\bash.exe. System32's WSL bash is never
// eligible. Empty result means not found — the exec then fails loudly with
// the original error.
func BashPath() string {
	bashOnce.Do(func() { bashResolved = discoverBash() })
	return bashResolved
}

func discoverBash() string {
	if p := os.Getenv("SMINE_BASH"); p != "" {
		return p
	}
	for _, root := range []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER} {
		for _, path := range []string{`SOFTWARE\GitForWindows`, `SOFTWARE\WOW6432Node\GitForWindows`} {
			if p := registryBash(root, path); p != "" {
				return p
			}
		}
	}
	for _, p := range []string{
		`C:\Program Files\Git\bin\bash.exe`,
		filepath.Join(os.Getenv("LOCALAPPDATA"), `Programs\Git\bin\bash.exe`),
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if git, err := exec.LookPath("git.exe"); err == nil {
		p := filepath.Join(filepath.Dir(filepath.Dir(git)), `bin\bash.exe`)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func registryBash(root registry.Key, path string) string {
	key, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()
	install, _, err := key.GetStringValue("InstallPath")
	if err != nil {
		return ""
	}
	p := filepath.Join(install, `bin\bash.exe`)
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}

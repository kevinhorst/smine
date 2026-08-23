package shell

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScriptArgvWindowsScript(t *testing.T) {
	name, args := scriptArgv("windows", `C:\Program Files\Git\bin\bash.exe`,
		`C:\repo\cmd\sync\sync_skills.sh`, []string{"--flag", `C:\Users\kevin\repo`})

	assert.Equal(t, `C:\Program Files\Git\bin\bash.exe`, name)
	assert.Equal(t, []string{"C:/repo/cmd/sync/sync_skills.sh", "--flag", "C:/Users/kevin/repo"}, args)
}

func TestScriptArgvWindowsNonScriptPassthrough(t *testing.T) {
	name, args := scriptArgv("windows", `C:\Git\bin\bash.exe`, "git", []string{"status"})

	assert.Equal(t, "git", name)
	assert.Equal(t, []string{"status"}, args)
}

func TestScriptArgvDarwinPassthrough(t *testing.T) {
	name, args := scriptArgv("darwin", "", "/repo/cmd/sync/sync_skills.sh", []string{`C:\odd`})

	assert.Equal(t, "/repo/cmd/sync/sync_skills.sh", name)
	assert.Equal(t, []string{`C:\odd`}, args)
}

func TestSlashDrivePathLeavesOpaqueArgs(t *testing.T) {
	assert.Equal(t, "--max-budget-usd", slashDrivePath("--max-budget-usd"))
	assert.Equal(t, "a:b", slashDrivePath("a:b"))
	assert.Equal(t, "C:/x", slashDrivePath(`C:\x`))
}

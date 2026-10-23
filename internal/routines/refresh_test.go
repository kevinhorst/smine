package routines

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"howett.net/plist"
)

const refreshTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key><string>com.test.routine.demo</string>
	<key>ProgramArguments</key><array><string>__REPO_DIR__/bin/routine</string><string>run</string><string>__REPO_DIR__/routines/demo</string></array>
	<key>StandardOutPath</key><string>__HOME__/Library/Logs/demo.out.log</string>
	<key>EnvironmentVariables</key><dict><key>ROUTINE_MODEL</key><string>template-model</string></dict>
	<key>StartCalendarInterval</key><dict><key>Hour</key><integer>3</integer></dict>
</dict>
</plist>
`

// writeRefreshFixture lays out repo/routines/demo with the template and
// returns (repoDir, templatePath, installedPath).
func writeRefreshFixture(t *testing.T) (string, string, string) {
	t.Helper()
	repoDir := t.TempDir()
	routineDir := filepath.Join(repoDir, "routines", "demo")
	require.NoError(t, os.MkdirAll(routineDir, 0o755))
	templatePath := filepath.Join(routineDir, "com.test.routine.demo.plist.template")
	require.NoError(t, os.WriteFile(templatePath, []byte(refreshTemplate), 0o644))
	return repoDir, templatePath, filepath.Join(routineDir, "com.test.routine.demo.plist")
}

func decodePlist(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var content map[string]any
	_, err = plist.Unmarshal(data, &content)
	require.NoError(t, err)
	return content
}

func TestRefreshPlistsMaterializesMissing(t *testing.T) {
	repoDir, _, installedPath := writeRefreshFixture(t)

	results, err := RefreshPlists("/home/demo", repoDir)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].Changed)

	content := decodePlist(t, installedPath)
	args := content["ProgramArguments"].([]any)
	assert.Equal(t, []any{repoDir + "/bin/routine", "run", repoDir + "/routines/demo"}, args)
	assert.Equal(t, "/home/demo/Library/Logs/demo.out.log", content["StandardOutPath"])
	assert.NoFileExists(t, filepath.Join(filepath.Dir(installedPath), PendingReloadMarker), "fresh materialization needs no reload")
}

func TestRefreshPlistsIdempotent(t *testing.T) {
	repoDir, _, _ := writeRefreshFixture(t)

	_, err := RefreshPlists("/home/demo", repoDir)
	require.NoError(t, err)

	results, err := RefreshPlists("/home/demo", repoDir)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.False(t, results[0].Changed, "identical content is not rewritten")
}

func TestRefreshPlistsCarriesOverOperatorKeys(t *testing.T) {
	repoDir, templatePath, installedPath := writeRefreshFixture(t)

	_, err := RefreshPlists("/home/demo", repoDir)
	require.NoError(t, err)

	// Operator edits: env + schedule diverge from the template.
	operatorEnv := map[string]string{"ROUTINE_MODEL": "operator-model", "ROUTINE_TOKEN": "alt"}
	require.NoError(t, SetEnv(operatorEnv, installedPath))
	hour, minute := 5, 30
	interval := CalendarInterval{Hour: &hour, Minute: &minute}
	require.NoError(t, Reschedule(interval, installedPath))

	// Template body changes (a new program argument).
	updatedText := strings.Replace(refreshTemplate, "routines/demo</string>", "routines/demo</string><string>--flag</string>", 1)
	require.NotEqual(t, refreshTemplate, updatedText)
	require.NoError(t, os.WriteFile(templatePath, []byte(updatedText), 0o644))

	results, err := RefreshPlists("/home/demo", repoDir)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].Changed)

	content := decodePlist(t, installedPath)
	args := content["ProgramArguments"].([]any)
	require.Len(t, args, 4, "template body wins")
	env := content["EnvironmentVariables"].(map[string]any)
	assert.Equal(t, "operator-model", env["ROUTINE_MODEL"], "operator env carried over")
	assert.Equal(t, "alt", env["ROUTINE_TOKEN"])
	schedule := content["StartCalendarInterval"].(map[string]any)
	assert.Equal(t, uint64(5), schedule["Hour"], "operator schedule carried over")
	assert.FileExists(t, filepath.Join(filepath.Dir(installedPath), PendingReloadMarker), "changed plist marks a pending reload")
}

func TestRefreshPlistsDeploysNewTemplateEnvKey(t *testing.T) {
	repoDir, templatePath, installedPath := writeRefreshFixture(t)

	_, err := RefreshPlists("/home/demo", repoDir)
	require.NoError(t, err)

	// Operator overrides the existing knob and adds an own key.
	operatorEnv := map[string]string{"ROUTINE_MODEL": "operator-model", "ROUTINE_EXTRA_PROMPT": "extra"}
	require.NoError(t, SetEnv(operatorEnv, installedPath))

	// The template gains a new deployment knob.
	updatedText := strings.Replace(
		refreshTemplate,
		"<key>ROUTINE_MODEL</key><string>template-model</string>",
		"<key>ROUTINE_MODEL</key><string>template-model</string><key>CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS</key><string>40</string>",
		1,
	)
	require.NotEqual(t, refreshTemplate, updatedText)
	require.NoError(t, os.WriteFile(templatePath, []byte(updatedText), 0o644))

	results, err := RefreshPlists("/home/demo", repoDir)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].Changed)

	content := decodePlist(t, installedPath)
	env := content["EnvironmentVariables"].(map[string]any)
	assert.Equal(t, "40", env["CLAUDE_CODE_MAX_CONCURRENT_SUBAGENTS"], "new template knob deploys")
	assert.Equal(t, "operator-model", env["ROUTINE_MODEL"], "operator value wins its key")
	assert.Equal(t, "extra", env["ROUTINE_EXTRA_PROMPT"], "operator-added key survives")
}

func TestRefreshPlistsRestoresTemplateEnvAfterClear(t *testing.T) {
	repoDir, _, installedPath := writeRefreshFixture(t)

	_, err := RefreshPlists("/home/demo", repoDir)
	require.NoError(t, err)

	// SetEnv with an empty map deletes the key; the template base layer
	// re-deploys its defaults on the next refresh (merge semantics).
	require.NoError(t, SetEnv(map[string]string{}, installedPath))

	results, err := RefreshPlists("/home/demo", repoDir)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].Changed)

	content := decodePlist(t, installedPath)
	env := content["EnvironmentVariables"].(map[string]any)
	assert.Equal(t, "template-model", env["ROUTINE_MODEL"], "template default returns after a full clear")
}

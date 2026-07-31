package shell

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeScript(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "script.sh")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o755))
	return path
}

func TestRunSuccessReturnsOutput(t *testing.T) {
	script := writeScript(t, "#!/bin/sh\necho hello\n")
	output, err := Run(context.Background(), "", script)
	require.NoError(t, err)
	assert.Equal(t, "hello\n", output)
}

func TestRunFailureReturnsOutputAndError(t *testing.T) {
	script := writeScript(t, "#!/bin/sh\necho broken >&2\nexit 3\n")
	output, err := Run(context.Background(), "", script)
	require.Error(t, err)
	assert.Contains(t, output, "broken")
}

func TestRunTimeoutKills(t *testing.T) {
	script := writeScript(t, "#!/bin/sh\nsleep 30\n")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := Run(ctx, "", script)
	require.Error(t, err)
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestRunAppliesDirAsCwd(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, "#!/bin/sh\npwd\n")
	output, err := Run(context.Background(), dir, script)
	require.NoError(t, err)

	// macOS: TMPDIR /var/... is a symlink to /private/var/... — compare resolved.
	want, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	got, err := filepath.EvalSymlinks(strings.TrimSpace(output))
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

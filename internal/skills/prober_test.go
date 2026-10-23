package skills

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kevinhorst/smine/internal/skillgraph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProber(t *testing.T) {
	dir := t.TempDir()
	countPath := filepath.Join(dir, "count")
	name, script := "counting-rules", "#!/bin/sh\necho x >> '"+countPath+"'\necho '{\"interfaces\":[\"title-set@1\"]}'\n"
	if runtime.GOOS == "windows" {
		name, script = "counting-rules.bat", "@echo x>>\""+countPath+"\"\r\n@echo {\"interfaces\":[\"title-set@1\"]}\r\n"
	}
	writeDeployFile(t, dir, name, script, 0o755)
	provider := &skillgraph.Provider{Id: "smine-rules", Implements: []string{"title-set@1"}, Program: filepath.Join(dir, name)}
	prober := NewProber()

	first, firstErr := prober.Claims(context.Background(), provider)
	second, secondErr := prober.Claims(context.Background(), provider)
	count, err := os.ReadFile(countPath)
	require.NoError(t, err)

	assert.NoError(t, firstErr)
	assert.NoError(t, secondErr)
	assert.Equal(t, []string{"title-set@1"}, first)
	assert.Equal(t, first, second)
	assert.Equal(t, 1, strings.Count(string(count), "x"), "probe-runs-once-per-executable")
}

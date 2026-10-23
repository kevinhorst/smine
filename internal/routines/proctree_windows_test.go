//go:build windows

package routines

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDescendantsOf(t *testing.T) {
	children := map[pid][]pid{1: {2, 3}, 3: {4}, 9: {10}}

	assert.Equal(t, []pid{1, 2, 3, 4}, descendantsOf(children, 1))
	assert.Equal(t, []pid{3, 4}, descendantsOf(children, 3))
	assert.Equal(t, []pid{7}, descendantsOf(children, 7), "a pid without children is its own tree")

	looping := map[pid][]pid{5: {6}, 6: {5}, 0: {0}}
	assert.Equal(t, []pid{5, 6}, descendantsOf(looping, 5), "a reused pid that loops back is walked once")
	assert.Equal(t, []pid{0}, descendantsOf(looping, 0), "the idle process lists itself as its parent")
}

func TestTerminateTree_EndsChildTree(t *testing.T) {
	child := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", "Start-Sleep -Seconds 60")
	require.NoError(t, child.Start())

	ended, err := TerminateTree(child.Process.Pid)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, ended, 1)
	assert.Error(t, child.Wait(), "the child exits 143, not 0")
}

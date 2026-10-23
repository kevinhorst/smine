//go:build darwin

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestClearQuarantine(t *testing.T) {
	root := t.TempDir()
	flagged := []string{filepath.Join(root, "bin", "installer"), filepath.Join(root, "bin", "PAYLOAD")}
	clean := filepath.Join(root, "README.md")
	for _, path := range append(flagged, clean) {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte("x"), 0o755))
	}
	for _, path := range flagged {
		require.NoError(t, unix.Setxattr(path, quarantineAttr, []byte("0083;00000000;Safari;"), 0))
	}
	out := &bytes.Buffer{}

	err := clearQuarantine(out, root)
	_, attrErr := unix.Getxattr(flagged[0], quarantineAttr, nil)
	again := &bytes.Buffer{}
	againErr := clearQuarantine(again, root)

	assert.NoError(t, err)
	assert.ErrorIs(t, attrErr, unix.ENOATTR, "the flag is gone")
	assert.Equal(t, "-> Cleared the macOS quarantine flag from 2 files under "+root+"\n", out.String())
	assert.NoError(t, againErr)
	assert.Empty(t, again.String(), "a clean tree prints nothing")
}

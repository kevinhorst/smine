package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunMissingArgsIsError(t *testing.T) {
	var out bytes.Buffer
	assert.Equal(t, 2, run([]string{}, &out))
	assert.Empty(t, out.String())
}

func TestRunNoModeIsError(t *testing.T) {
	var out bytes.Buffer
	assert.Equal(t, 2, run([]string{"files.txt"}, &out))
	assert.Empty(t, out.String())
}

func TestRunUnknownModeIsError(t *testing.T) {
	var out bytes.Buffer
	// An unrecognized mode is a tool error (exit 2) before the wrapped verb runs;
	// the files-list path is never read, so a non-existent path is fine here.
	assert.Equal(t, 2, run([]string{"files.txt", "mode=bogus"}, &out))
	assert.Empty(t, out.String())
}

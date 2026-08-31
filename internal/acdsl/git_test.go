package acdsl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitShowFile(t *testing.T) {
	root := newPolicyRepo(t, gatedBasePolicy)
	base, err := GitRevParse(context.Background(), root, "main")
	require.NoError(t, err)

	content, exists, err := GitShowFile(context.Background(), root, base, "acdsl/rules.acdsl")
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Contains(t, content, "ACDSL-GOLANG-100")

	_, exists, err = GitShowFile(context.Background(), root, base, "acdsl/nosuchfile.json")
	require.NoError(t, err)
	assert.False(t, exists)
}

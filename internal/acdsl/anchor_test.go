package acdsl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveAnchorIncludeOnly(t *testing.T) {
	tracked := []string{"cmd/a/main.go", "internal/b/b.go", "internal/b/b_test.go"}
	matched, err := ResolveAnchor(tracked, Rule{Id: "X", Anchor: `^internal/`})
	require.NoError(t, err)
	assert.Equal(t, []string{"internal/b/b.go", "internal/b/b_test.go"}, matched)
}

func TestResolveAnchorWithExclude(t *testing.T) {
	tracked := []string{"internal/b/b.go", "internal/b/b_test.go"}
	matched, err := ResolveAnchor(tracked, Rule{Id: "X", Anchor: `^internal/`, AnchorNot: `_test\.go$`})
	require.NoError(t, err)
	assert.Equal(t, []string{"internal/b/b.go"}, matched)
}

func TestResolveAnchorNoMatchIsError(t *testing.T) {
	_, err := ResolveAnchor([]string{"a.go"}, Rule{Id: "X-001", Anchor: `^nothing/`})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "X-001")
	assert.ErrorIs(t, err, ErrAnchorEmpty)
}

func TestResolveAnchorInvalidRegexpIsError(t *testing.T) {
	_, err := ResolveAnchor([]string{"a.go"}, Rule{Id: "X", Anchor: `(`})
	require.Error(t, err)

	_, err = ResolveAnchor([]string{"a.go"}, Rule{Id: "X", Anchor: `a`, AnchorNot: `(`})
	require.Error(t, err)
}

func TestResolveAnchorDeterministicOrder(t *testing.T) {
	tracked := []string{"a.go", "b.go", "c.go"} // caller-sorted
	first, err := ResolveAnchor(tracked, Rule{Id: "X", Anchor: `\.go$`})
	require.NoError(t, err)
	second, err := ResolveAnchor(tracked, Rule{Id: "X", Anchor: `\.go$`})
	require.NoError(t, err)
	assert.Equal(t, first, second)
	assert.Equal(t, tracked, first)
}

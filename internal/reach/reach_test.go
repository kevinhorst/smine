package reach

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidAcceptsGlobalAndNameLists(t *testing.T) {
	assert.True(t, Valid(Global))
	assert.True(t, Valid(ThisRepo))
	assert.True(t, Valid("aqms"))
	assert.True(t, Valid("aqms,peek-mcp"))
	assert.True(t, Valid("aqms, peek-mcp"))
	assert.True(t, Valid("repo,aqms")) // "repo" is an ordinary name now
	assert.True(t, Valid("a.b_c-d,x9"))
}

func TestValidRejectsReservedEmptyAndMalformedTokens(t *testing.T) {
	assert.False(t, Valid(""))
	assert.False(t, Valid("aqms,global"))
	assert.False(t, Valid("aqms,none"))
	assert.False(t, Valid("aqms,,peek-mcp"))
	assert.False(t, Valid("aqms peek-mcp"))
	assert.False(t, Valid("aqms/peek"))
}

func TestValidAcceptsNone(t *testing.T) {
	assert.True(t, Valid(None))
}

func TestNamesSplitsListsAndEmptiesGlobal(t *testing.T) {
	assert.Nil(t, Names(Global))
	assert.Nil(t, Names(None))
	assert.Nil(t, Names(""))
	assert.Equal(t, []string{"smine"}, Names(ThisRepo))
	assert.Equal(t, []string{"aqms", "peek-mcp"}, Names("aqms, peek-mcp"))
}

func TestDeploysToGlobalCoversAnyTarget(t *testing.T) {
	assert.True(t, DeploysTo(Global, "aqms"))
	assert.True(t, DeploysTo(Global, "anything"))
}

func TestDeploysToEmptyCoversNothing(t *testing.T) {
	assert.False(t, DeploysTo("", "aqms"))
}

func TestDeploysToNoneCoversNothing(t *testing.T) {
	assert.False(t, DeploysTo(None, "aqms"))
	assert.False(t, DeploysTo(None, Global))
	assert.False(t, DeploysTo(None, None))
	assert.False(t, DeploysTo(None, ThisRepo))
}

func TestDeploysToListCoversOnlyNamedTargets(t *testing.T) {
	assert.True(t, DeploysTo("aqms,peek-mcp", "aqms"))
	assert.True(t, DeploysTo("aqms, peek-mcp", "peek-mcp"))
	assert.False(t, DeploysTo("aqms,peek-mcp", "smine"))
	// smine is a plain name: a list of one, matched only by itself —
	// and no sync target is ever named smine, so it never deploys.
	assert.True(t, DeploysTo(ThisRepo, ThisRepo))
	assert.False(t, DeploysTo(ThisRepo, "aqms"))
}

package repos

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocks(t *testing.T) {
	locks := NewLocks()
	assert.True(t, locks.TryAcquire("repo"))
	assert.False(t, locks.TryAcquire("repo"))
	locks.Release("repo")
	assert.True(t, locks.TryAcquire("repo"))
}

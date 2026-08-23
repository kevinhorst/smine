package shell

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunDialog(t *testing.T) {
	t.Run("echo-passthrough-returns-output", func(t *testing.T) {
		output, err := RunDialog(context.Background(), "echo", "picked")
		require.NoError(t, err)
		assert.Contains(t, output, "picked")
	})

	t.Run("failing-command-wraps-error-with-output", func(t *testing.T) {
		output, err := RunDialog(context.Background(), "sh", "-c", "echo dialog dead >&2; exit 3")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "RunDialog")
		assert.Contains(t, output, "dialog dead")
	})
}

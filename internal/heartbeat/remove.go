package heartbeat

import (
	"context"
	"fmt"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

// A refusal keeps the CLI's message.
func (c *Cli) Remove(ctx context.Context, fleet string, member string) error {
	args := c.args("remove", "--fleet", fleet, "--member", member)
	output, err := shell.Run(ctx, "", c.BinaryPath, args...)
	if err != nil {
		return fmt.Errorf("Cli.Remove: Fleet %s member %s: %s: %w", fleet, member, strings.TrimSpace(output), err)
	}
	return nil
}

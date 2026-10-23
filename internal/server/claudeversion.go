package server

import (
	"context"
	"log"
	"sync"

	"github.com/kevinhorst/smine/internal/install"
)

// installedClaude probes `claude --version` once per server; a failed probe reads "" for the server's lifetime.
type installedClaude struct {
	once  sync.Once
	value string
}

func (c *installedClaude) version(ctx context.Context) string {
	c.once.Do(func() {
		version, err := install.ClaudeVersion(context.WithoutCancel(ctx))
		if err != nil {
			log.Printf("installedClaude.version: %v", err)
			return
		}
		c.value = version
	})
	return c.value
}

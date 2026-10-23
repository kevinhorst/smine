package install

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kevinhorst/smine/internal/shell"
)

// ClaudeVersion is the installed Claude Code CLI's version, the first word of `claude --version`.
func ClaudeVersion(ctx context.Context) (string, error) {
	output, err := shell.Run(ctx, "", "claude", "--version")
	if err != nil {
		return "", fmt.Errorf("ClaudeVersion: %w", err)
	}

	fields := strings.Fields(output)
	if len(fields) == 0 {
		return "", errors.New("ClaudeVersion: Empty version output")
	}
	return fields[0], nil
}

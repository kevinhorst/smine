package routinerun

import (
	"encoding/json"
	"fmt"
	"slices"

	settings "github.com/kevinhorst/smine/internal/config"
	"github.com/kevinhorst/smine/internal/peek"
)

const peekServerName = "peek-mcp"

type mcpConfigFile struct {
	McpServers map[string]*mcpConfigServer `json:"mcpServers"`
}

type mcpConfigServer struct {
	Args    []string `json:"args"`
	Command string   `json:"command"`
	Type    string   `json:"type"`
}

// registeredMcpConfig appends extraArgs to the registered peek-mcp entry.
func registeredMcpConfig(claudeJsonPath string, extraArgs ...string) (string, error) {
	command, args, err := settings.McpServer(claudeJsonPath, peekServerName)
	if err != nil {
		return "", fmt.Errorf("registeredMcpConfig: %w", err)
	}

	server := &mcpConfigServer{
		Args:    append(slices.Clone(args), extraArgs...),
		Command: command,
		Type:    "stdio",
	}
	servers := map[string]*mcpConfigServer{peekServerName: server}
	file := &mcpConfigFile{McpServers: servers}
	data, err := json.Marshal(file)
	if err != nil {
		return "", fmt.Errorf("registeredMcpConfig: %w", err)
	}
	return string(data), nil
}

func SmineMcpConfig(claudeJsonPath string) (string, error) {
	mcpConfig, err := registeredMcpConfig(claudeJsonPath, peek.WatchWindowArg(peek.MiningWindowDays))
	if err != nil {
		return "", fmt.Errorf("SmineMcpConfig: %w", err)
	}
	return mcpConfig, nil
}

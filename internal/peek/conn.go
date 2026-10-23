package peek

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

const toolErrorMessage = "Tool returned an error"

// Calls share the transport, so a stdio peek is spawned once for all of them.
type Conn struct {
	mcpClient *mcpclient.Client
}

func (c *Conn) listSessions(ctx context.Context, agent string) ([]listItem, error) {
	request := mcp.CallToolRequest{}
	request.Params.Name = "session_list"
	request.Params.Arguments = map[string]any{"agent": agent}
	result, err := c.mcpClient.CallTool(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("Conn.listSessions: %w", err)
	}
	if result.IsError {
		return nil, errors.New("Conn.listSessions: " + toolErrorText(result))
	}

	// session_list always responds with structured content (F2).
	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return nil, fmt.Errorf("Conn.listSessions: %w", err)
	}

	var listResult sessionListResult
	if err := json.Unmarshal(data, &listResult); err != nil {
		return nil, fmt.Errorf("Conn.listSessions: %w", err)
	}
	return listResult.Sessions, nil
}

func (c *Conn) Close() error {
	if err := c.mcpClient.Close(); err != nil {
		return fmt.Errorf("Conn.Close: %w", err)
	}
	return nil
}

func (c *Conn) SessionEvents(ctx context.Context, id string) (*SessionEvents, error) {
	request := mcp.CallToolRequest{}
	request.Params.Name = "session_events"
	request.Params.Arguments = map[string]any{"breakdown": true, "id": id, "json": true}
	result, err := c.mcpClient.CallTool(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("Conn.SessionEvents: %w", err)
	}
	if result.IsError {
		return nil, errors.New("Conn.SessionEvents: " + toolErrorText(result))
	}

	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return nil, fmt.Errorf("Conn.SessionEvents: %w", err)
	}

	var parsed eventsResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("Conn.SessionEvents: %w", err)
	}
	return newSessionEvents(&parsed), nil
}

// Sessions returns every session of the given agents, uncollapsed.
func (c *Conn) Sessions(ctx context.Context, agents []string) ([]Session, error) {
	var sessions []Session
	for _, agent := range agents {
		items, err := c.listSessions(ctx, agent)
		if err != nil {
			return nil, err
		}
		sessions = appendAgentSessions(agent, items, sessions)
	}
	return sessions, nil
}

// Only the turns section is requested (json:true).
func (c *Conn) SessionTurns(ctx context.Context, agent, id string) ([]Turn, error) {
	request := mcp.CallToolRequest{}
	request.Params.Name = "session_get"
	request.Params.Arguments = map[string]any{
		"agent":  agent,
		"diff":   false,
		"events": false,
		"id":     id,
		"json":   true,
		"n":      turnsPageSize,
		"plan":   false,
		"turns":  true,
	}
	result, err := c.mcpClient.CallTool(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("Conn.SessionTurns: %w", err)
	}
	if result.IsError {
		return nil, errors.New("Conn.SessionTurns: " + toolErrorText(result))
	}

	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return nil, fmt.Errorf("Conn.SessionTurns: %w", err)
	}

	var parsed turnsResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("Conn.SessionTurns: %w", err)
	}

	turns := make([]Turn, 0, len(parsed.Turns))
	for _, item := range parsed.Turns {
		turn := Turn{Model: item.Meta.Model, Role: item.Role, Text: item.Text}
		turns = append(turns, turn)
	}
	return turns, nil
}

// peek states a failed call's reason in the result's text content.
func toolErrorText(result *mcp.CallToolResult) string {
	reasons := make([]string, 0, len(result.Content))
	for _, content := range result.Content {
		if text, ok := content.(mcp.TextContent); ok {
			reasons = append(reasons, text.Text)
		}
	}
	if len(reasons) == 0 {
		return toolErrorMessage
	}
	return toolErrorMessage + ": " + strings.Join(reasons, " ")
}

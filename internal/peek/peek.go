// Package peek is an MCP client for peek-mcp (Streamable HTTP) — the single
// source of session liveness; the config server never imports peek-mcp Go
// code, the tool interface is the whole contract (D3).
package peek

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	// LiveWindow marks a session live when its last activity is at most this
	// old — peek-mcp exposes no live flag (D10).
	LiveWindow = 15 * time.Minute

	clientName    = "config-server"
	clientVersion = "1.0"
)

type Client struct {
	endpoint string
}

func NewClient(endpoint string) *Client {
	return &Client{endpoint: endpoint}
}

func (c *Client) connect(ctx context.Context) (*mcpclient.Client, error) {
	mcpClient, err := mcpclient.NewStreamableHttpClient(c.endpoint)
	if err != nil {
		return nil, fmt.Errorf("Client.connect: %s: %w", c.endpoint, err)
	}
	if err := mcpClient.Start(ctx); err != nil {
		return nil, fmt.Errorf("Client.connect: Failed to start transport: %w", err)
	}

	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{Name: clientName, Version: clientVersion}
	if _, err := mcpClient.Initialize(ctx, initRequest); err != nil {
		mcpClient.Close()
		return nil, fmt.Errorf("Client.connect: Initialize failed: %w", err)
	}

	return mcpClient, nil
}

func (c *Client) listSessions(ctx context.Context, mcpClient *mcpclient.Client) ([]listItem, error) {
	request := mcp.CallToolRequest{}
	request.Params.Name = "session_list"
	request.Params.Arguments = map[string]any{"agent": "claude"}
	result, err := mcpClient.CallTool(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("Client.listSessions: %w", err)
	}
	if result.IsError {
		return nil, errors.New("Client.listSessions: Tool returned an error")
	}

	// session_list always responds with structured content (F2).
	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return nil, fmt.Errorf("Client.listSessions: %w", err)
	}

	var listResult sessionListResult
	if err := json.Unmarshal(data, &listResult); err != nil {
		return nil, fmt.Errorf("Client.listSessions: %w", err)
	}
	return listResult.Sessions, nil
}

// SessionsByCwd returns the most recently active claude session per working
// directory — one session_list call, cwd nested in each item's meta since
// peek-mcp v1.0.5. Sessions without a cwd (no turn meta yet) are skipped.
func (c *Client) SessionsByCwd(ctx context.Context) (map[string]Session, error) {
	mcpClient, err := c.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer mcpClient.Close()

	items, err := c.listSessions(ctx, mcpClient)
	if err != nil {
		return nil, err
	}

	byCwd := make(map[string]Session)
	for _, item := range items {
		if item.Meta.Cwd == "" {
			continue
		}

		cwd := filepath.Clean(item.Meta.Cwd)
		candidate := Session{
			Id:         item.Id,
			Cwd:        cwd,
			LastActive: item.LastActive,
			Live:       time.Since(item.LastActive) <= LiveWindow,
			Title:      item.Title,
		}
		known, ok := byCwd[cwd]
		if !ok || candidate.LastActive.After(known.LastActive) {
			byCwd[cwd] = candidate
		}
	}
	return byCwd, nil
}

func (c *Client) SessionsById(ctx context.Context) (map[string]Session, error) {
	mcpClient, err := c.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer mcpClient.Close()

	items, err := c.listSessions(ctx, mcpClient)
	if err != nil {
		return nil, err
	}

	byId := make(map[string]Session)
	for _, item := range items {
		byId[item.Id] = Session{
			Id:         item.Id,
			Cwd:        filepath.Clean(item.Meta.Cwd),
			LastActive: item.LastActive,
			Live:       time.Since(item.LastActive) <= LiveWindow,
			Title:      item.Title,
		}
	}
	return byId, nil
}

type listItem struct {
	Id         string    `json:"id"`
	Meta       listMeta  `json:"meta"`
	LastActive time.Time `json:"last_active"`
	Title      string    `json:"title"`
}

// listMeta models the one session-meta field the client consumes; the server
// sends more (session_id, git_branch, model, origin), all ignored.
type listMeta struct {
	Cwd string `json:"cwd"`
}

type Session struct {
	Id string

	Cwd        string
	LastActive time.Time
	Live       bool
	Title      string
}

type sessionListResult struct {
	Sessions []listItem `json:"sessions"`
}

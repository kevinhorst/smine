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

// Endpoint reports the configured peek-mcp URL — degraded-mode banners name
// it because the root-cause error alone often lacks the address.
func (c *Client) Endpoint() string {
	return c.endpoint
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

// SessionIndex returns the most recently active claude session per working
// directory and per git branch — one session_list call, cwd and git_branch
// nested in each item's meta since peek-mcp v1.0.5. Sessions without a cwd
// (no turn meta yet) are skipped from ByCwd; detached-HEAD sessions are
// skipped from ByBranch. Both maps exist so pool-recycled worktree dirs can
// be attributed by branch identity instead of the dir's previous occupant.
func (c *Client) SessionIndex(ctx context.Context) (*SessionIndex, error) {
	mcpClient, err := c.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer mcpClient.Close()

	items, err := c.listSessions(ctx, mcpClient)
	if err != nil {
		return nil, err
	}

	index := &SessionIndex{
		ByBranch: make(map[string]Session),
		ByCwd:    make(map[string]Session),
	}
	for _, item := range items {
		candidate := Session{
			Id:         item.Id,
			Cwd:        item.Meta.Cwd,
			GitBranch:  item.Meta.GitBranch,
			LastActive: item.LastActive,
			Live:       time.Since(item.LastActive) <= LiveWindow,
			Title:      item.Title,
		}
		if candidate.Cwd != "" {
			candidate.Cwd = filepath.Clean(candidate.Cwd)
			keepNewest(index.ByCwd, candidate.Cwd, candidate)
		}

		isRealBranch := candidate.GitBranch != "" && candidate.GitBranch != "HEAD"
		if isRealBranch {
			keepNewest(index.ByBranch, candidate.GitBranch, candidate)
		}
	}
	return index, nil
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

// listMeta models the session-meta fields the client consumes; the server
// sends more (session_id, model, origin), all ignored.
type listMeta struct {
	Cwd       string `json:"cwd"`
	GitBranch string `json:"git_branch"`
}

type Session struct {
	Id string

	Cwd        string
	GitBranch  string
	LastActive time.Time
	Live       bool
	Title      string
}

// SessionIndex holds the newest-session projections one session_list call
// yields: per working directory and per checked-out git branch.
type SessionIndex struct {
	ByBranch map[string]Session
	ByCwd    map[string]Session
}

type sessionListResult struct {
	Sessions []listItem `json:"sessions"`
}

// keepNewest stores candidate under key unless a more recently active
// session already occupies it.
func keepNewest(sessions map[string]Session, key string, candidate Session) {
	known, ok := sessions[key]
	if !ok || candidate.LastActive.After(known.LastActive) {
		sessions[key] = candidate
	}
}

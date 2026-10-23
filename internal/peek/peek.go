// Package peek is the MCP client for peek-mcp, whose tool interface is the whole contract (D3).
package peek

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

const (
	// peek-mcp exposes no live flag (D10).
	LiveWindow = 15 * time.Minute

	clientName    = "config-server"
	clientVersion = "1.0"
)

// ReportAgents are the agents whose sessions get a context report.
var ReportAgents = []string{"claude", "codex"}

type Client struct {
	endpoint  string
	stdioArgs []string
	stdioBin  string
}

func NewClient(endpoint string) *Client {
	return &Client{endpoint: endpoint}
}

// A private peek per connection, gone when the connection closes.
func NewStdioClient(args []string, bin string) *Client {
	return &Client{stdioArgs: args, stdioBin: bin}
}

// Degraded-mode banners name it because the root-cause error often lacks the address.
func (c *Client) Endpoint() string {
	return c.endpoint
}

// Connect opens one connection; the caller closes it.
func (c *Client) Connect(ctx context.Context) (*Conn, error) {
	mcpClient, err := c.newTransportClient()
	if err != nil {
		return nil, err
	}
	if err := mcpClient.Start(ctx); err != nil {
		return nil, fmt.Errorf("Client.Connect: Failed to start transport: %w", err)
	}

	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{Name: clientName, Version: clientVersion}
	if _, err := mcpClient.Initialize(ctx, initRequest); err != nil {
		mcpClient.Close()
		return nil, fmt.Errorf("Client.Connect: Initialize failed: %w", err)
	}

	conn := &Conn{mcpClient: mcpClient}
	return conn, nil
}

// ByBranch lets pool-recycled worktree dirs be attributed by branch, not the dir's previous occupant.
func (c *Client) SessionIndex(ctx context.Context) (*SessionIndex, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	items, err := conn.listSessions(ctx, "claude")
	if err != nil {
		return nil, err
	}

	index := &SessionIndex{
		ByBranch: make(map[string]Session),
		ByCwd:    make(map[string]Session),
	}
	for _, item := range items {
		candidate := newSession("claude", &item)
		if candidate.Cwd != "" {
			keepNewest(index.ByCwd, candidate.Cwd, candidate)
		}

		isRealBranch := candidate.GitBranch != "" && candidate.GitBranch != "HEAD"
		if isRealBranch {
			keepNewest(index.ByBranch, candidate.GitBranch, candidate)
		}
	}
	return index, nil
}

// Uncollapsed: callers that need all matches filter and sort themselves.
func (c *Client) Sessions(ctx context.Context, agents []string) ([]Session, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	return conn.Sessions(ctx, agents)
}

func (c *Client) SessionsById(ctx context.Context) (map[string]Session, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	byId := make(map[string]Session)
	for _, agent := range ReportAgents {
		items, err := conn.listSessions(ctx, agent)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			byId[item.Id] = newSession(agent, &item)
		}
	}
	return byId, nil
}

// Start(ctx) spawns the stdio child under ctx.
func (c *Client) newTransportClient() (*mcpclient.Client, error) {
	if c.stdioBin != "" {
		stdioTransport := transport.NewStdio(c.stdioBin, nil, c.stdioArgs...)
		return mcpclient.NewClient(stdioTransport), nil
	}

	mcpClient, err := mcpclient.NewStreamableHttpClient(c.endpoint)
	if err != nil {
		return nil, fmt.Errorf("Client.newTransportClient: %s: %w", c.endpoint, err)
	}
	return mcpClient, nil
}

type listItem struct {
	Id         string    `json:"id"`
	Meta       listMeta  `json:"meta"`
	LastActive time.Time `json:"last_active"`
	Title      string    `json:"title"`
}

// The server sends more fields (session_id, origin), all ignored.
type listMeta struct {
	Cwd       string `json:"cwd"`
	GitBranch string `json:"git_branch"`
	Model     string `json:"model"`
}

type Session struct {
	Id string

	Agent      string
	Cwd        string
	GitBranch  string
	LastActive time.Time
	Live       bool
	Model      string
	Title      string
}

type SessionIndex struct {
	ByBranch map[string]Session
	ByCwd    map[string]Session
}

type sessionListResult struct {
	Sessions []listItem `json:"sessions"`
}

func appendAgentSessions(agent string, items []listItem, sessions []Session) []Session {
	for _, item := range items {
		sessions = append(sessions, newSession(agent, &item))
	}
	return sessions
}

// filepath.Clean("") is ".", which would read as a path.
func cleanCwd(cwd string) string {
	if cwd == "" {
		return ""
	}
	return filepath.Clean(cwd)
}

func keepNewest(sessions map[string]Session, key string, candidate Session) {
	known, ok := sessions[key]
	if !ok || candidate.LastActive.After(known.LastActive) {
		sessions[key] = candidate
	}
}

// Live is judged against LiveWindow at call time.
func newSession(agent string, item *listItem) Session {
	return Session{
		Id:         item.Id,
		Agent:      agent,
		Cwd:        cleanCwd(item.Meta.Cwd),
		GitBranch:  item.Meta.GitBranch,
		LastActive: item.LastActive,
		Live:       time.Since(item.LastActive) <= LiveWindow,
		Model:      item.Meta.Model,
		Title:      item.Title,
	}
}

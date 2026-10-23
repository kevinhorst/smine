package peek

import "context"

// One page holds every turn; the count stays far below this.
const turnsPageSize = 10000

// SessionTurns opens one connection for one session_get call.
func (c *Client) SessionTurns(ctx context.Context, agent, id string) ([]Turn, error) {
	conn, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	return conn.SessionTurns(ctx, agent, id)
}

// Turn is one conversation turn as session_get serves it.
type Turn struct {
	Model string
	Role  string
	Text  string
}

type turnsItem struct {
	Meta turnsMeta `json:"meta"`
	Role string    `json:"role"`
	Text string    `json:"text"`
}

type turnsMeta struct {
	Model string `json:"model"`
}

type turnsResult struct {
	Turns []turnsItem `json:"turns"`
}

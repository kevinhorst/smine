package server

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/kevinhorst/smine/internal/contextrecord"
)

const (
	agentParam      = "agent"
	scopeIdPrefix   = "id:"
	scopeRoot       = "root"
	scopeTypePrefix = "type:"
)

// agentScope's zero value selects every agent.
type agentScope struct {
	agentType string
	id        string
	isRoot    bool
}

// parseAgentScope selects every agent for an unknown value.
func parseAgentScope(r *http.Request) agentScope {
	value := r.URL.Query().Get(agentParam)
	if value == scopeRoot {
		return agentScope{isRoot: true}
	}
	if agentType, isType := strings.CutPrefix(value, scopeTypePrefix); isType {
		return agentScope{agentType: agentType}
	}
	if id, isId := strings.CutPrefix(value, scopeIdPrefix); isId {
		return agentScope{id: id}
	}
	return agentScope{}
}

// IsAll reports the zero selection: every agent.
func (a agentScope) IsAll() bool {
	return a.Param() == ""
}

// Param is the scope's query value, "" for every agent.
func (a agentScope) Param() string {
	switch {
	case a.isRoot:
		return scopeRoot
	case a.agentType != "":
		return scopeTypePrefix + a.agentType
	case a.id != "":
		return scopeIdPrefix + a.id
	}
	return ""
}

// matches reports whether the agent is in the selection.
func (a agentScope) matches(agent *contextrecord.Agent) bool {
	switch {
	case a.isRoot:
		return agent.Kind == contextrecord.AgentRoot
	case a.agentType != "":
		return agent.Type == a.agentType
	case a.id != "":
		return agent.Id == a.id
	}
	return true
}

func scopeURL(base string, filter contextFilter, scope agentScope) string {
	link := filter.URL(base)
	param := scope.Param()
	if param == "" {
		return link
	}

	separator := "?"
	if strings.Contains(link, "?") {
		separator = "&"
	}
	return link + separator + agentParam + "=" + url.QueryEscape(param)
}

// scopedRecord keeps the record-level id views only when the root is selected.
func scopedRecord(record *contextrecord.Record, scope agentScope) *contextrecord.Record {
	scoped := &contextrecord.Record{
		Agent:     record.Agent,
		Cwd:       record.Cwd,
		GitBranch: record.GitBranch,
		SessionId: record.SessionId,
	}
	for index := range record.Agents {
		if scope.matches(&record.Agents[index]) {
			scoped.Agents = append(scoped.Agents, record.Agents[index])
		}
	}

	if hasRootAgent(scoped) {
		scoped.AcdslRules = record.AcdslRules
		scoped.Injected = record.Injected
		scoped.PackReads = record.PackReads
		scoped.PlanRules = record.PlanRules
		scoped.SubagentContext = record.SubagentContext
	}
	scoped.Recompute()
	return scoped
}

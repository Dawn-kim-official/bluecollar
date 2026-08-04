package agentcontract

import "strings"

const unnamedAgentName = "the assistant"

type AgentIdentity struct {
	Name   string
	Handle string
}

func (identity AgentIdentity) DisplayName() string {
	if trimmedName := strings.TrimSpace(identity.Name); trimmedName != "" {
		return trimmedName
	}
	return unnamedAgentName
}

func (identity AgentIdentity) MentionExample() string {
	trimmedHandle := strings.TrimSpace(identity.Handle)
	if trimmedHandle == "" {
		return "your bot handle"
	}
	if strings.HasPrefix(trimmedHandle, "@") {
		return trimmedHandle
	}
	return "@" + trimmedHandle
}

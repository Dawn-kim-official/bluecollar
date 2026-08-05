package main

import (
	"context"
	"encoding/json"
	"strings"

	acp "github.com/coder/acp-go-sdk"
	"github.com/yeomyeonggeori/bluecollar/taskstate"
)

const ledgerMetaKey = "bluecollar.dev/ledger"

type ledgerRecord struct {
	Name string          `json:"name"`
	Body json.RawMessage `json:"body,omitempty"`
}

type sessionUpdateSender interface {
	SessionUpdate(context.Context, acp.SessionNotification) error
}

func sendLedgerEvent(ctx context.Context, sender sessionUpdateSender, sessionID acp.SessionId, rawTurnEvent taskstate.RawTurnEvent) {
	if sender == nil {
		return
	}
	sender.SessionUpdate(ctx, acp.SessionNotification{
		SessionId: sessionID,
		Update:    sessionUpdateForEvent(rawTurnEvent),
	})
}

func sessionUpdateForEvent(rawTurnEvent taskstate.RawTurnEvent) acp.SessionUpdate {
	meta := ledgerMeta(rawTurnEvent)
	if toolName, isRequest := toolNameOfEvent(rawTurnEvent.Name, ".requested"); isRequest {
		return acp.SessionUpdate{ToolCall: &acp.SessionUpdateToolCall{
			ToolCallId: acp.ToolCallId(observationIDOfEvent(rawTurnEvent.Body)),
			Title:      toolName,
			Status:     acp.ToolCallStatusPending,
			RawInput:   rawInputOfEvent(rawTurnEvent.Body),
			Meta:       meta,
		}}
	}
	if _, isResult := toolNameOfEvent(rawTurnEvent.Name, ".result"); isResult {
		status := acp.ToolCallStatusCompleted
		if isFailureEvent(rawTurnEvent.Body) {
			status = acp.ToolCallStatusFailed
		}
		return acp.SessionUpdate{ToolCallUpdate: &acp.SessionToolCallUpdate{
			ToolCallId: acp.ToolCallId(observationIDOfEvent(rawTurnEvent.Body)),
			Status:     &status,
			RawOutput:  json.RawMessage(rawTurnEvent.Body),
			Meta:       meta,
		}}
	}
	return acp.SessionUpdate{AgentThoughtChunk: &acp.SessionUpdateAgentThoughtChunk{
		Content: acp.TextBlock(rawTurnEvent.Name),
		Meta:    meta,
	}}
}

func ledgerMeta(rawTurnEvent taskstate.RawTurnEvent) map[string]any {
	record := ledgerRecord{Name: rawTurnEvent.Name}
	if json.Valid([]byte(rawTurnEvent.Body)) {
		record.Body = json.RawMessage(rawTurnEvent.Body)
	} else {
		quoted, _ := json.Marshal(rawTurnEvent.Body)
		record.Body = quoted
	}
	return map[string]any{ledgerMetaKey: record}
}

func toolNameOfEvent(eventName string, suffix string) (string, bool) {
	if !strings.HasPrefix(eventName, "tool.") || !strings.HasSuffix(eventName, suffix) {
		return "", false
	}
	return strings.TrimSuffix(strings.TrimPrefix(eventName, "tool."), suffix), true
}

func observationIDOfEvent(body string) string {
	decoded := struct {
		ObservationID string `json:"observationID"`
	}{}
	json.Unmarshal([]byte(body), &decoded)
	return decoded.ObservationID
}

func rawInputOfEvent(body string) any {
	decoded := struct {
		Input json.RawMessage `json:"input"`
	}{}
	if json.Unmarshal([]byte(body), &decoded) != nil || len(decoded.Input) == 0 {
		return nil
	}
	return decoded.Input
}

func isFailureEvent(body string) bool {
	decoded := struct {
		Failure *json.RawMessage `json:"failure"`
	}{}
	return json.Unmarshal([]byte(body), &decoded) == nil && decoded.Failure != nil
}

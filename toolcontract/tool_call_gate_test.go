package toolcontract

import (
	"context"
	"encoding/json"
	"testing"
)

type recordingToolCallGate struct {
	reviewedToolNames []string
	withheldToolName  string
	errorValue        error
}

func (gate *recordingToolCallGate) ReviewToolCall(_ context.Context, toolInvocation ToolInvocation, _ ToolDefinition) (ToolCallReview, error) {
	gate.reviewedToolNames = append(gate.reviewedToolNames, toolInvocation.ToolName)
	if gate.errorValue != nil {
		return ToolCallReview{}, gate.errorValue
	}
	if toolInvocation.ToolName == gate.withheldToolName {
		return ToolCallReview{Result: ToolFailureResult(FailureUnknown, FailureCodes.InteractionRequired, "approval", "waiting for the requester")}, nil
	}
	return ToolCallReview{MayProceed: true}, nil
}

func gatedToolSet(t *testing.T, gate ToolCallGate) (*ToolSet, *int) {
	t.Helper()
	handlerCallCount := 0
	toolSet := NewToolSet([]string{"message_send"})
	errorValue := toolSet.RegisterTool(ToolDefinition{
		ID:              "test:message_send",
		Name:            "message_send",
		Description:     "send a message",
		Visibility:      ToolVisibilityModel,
		InputSchema:     json.RawMessage(`{"type":"object","properties":{"message":{"type":"string"}}}`),
		SideEffectClass: ToolSideEffectExternalSend,
		ResultContract:  &ToolResultContract{Schema: json.RawMessage(`{"type":"object","properties":{},"additionalProperties":true}`)},
	}, func(context.Context, ToolInvocation) (ToolResult, error) {
		handlerCallCount++
		return ToolSuccessData("sent", json.RawMessage(`{}`)), nil
	})
	if errorValue != nil {
		t.Fatalf("expected the tool to register: %v", errorValue)
	}
	toolSet.UseToolCallGate(gate)
	return toolSet, &handlerCallCount
}

func TestAWithheldCallNeverReachesItsHandler(t *testing.T) {
	toolSet, handlerCallCount := gatedToolSet(t, &recordingToolCallGate{withheldToolName: "message_send"})

	result, errorValue := toolSet.Invoke(context.Background(), ToolInvocation{ToolName: "message_send", Input: json.RawMessage(`{"message":"보냅니다"}`)})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if *handlerCallCount != 0 {
		t.Fatalf("a gate that runs after the handler has already had the effect it was gating, calls=%d", *handlerCallCount)
	}
	if !result.Failed() || result.FailureCode() != FailureCodes.InteractionRequired.String() {
		t.Fatalf("expected the gate's result to be returned, got %+v", result)
	}
}

func TestAPermittedCallRunsUntouched(t *testing.T) {
	toolSet, handlerCallCount := gatedToolSet(t, &recordingToolCallGate{})

	result, errorValue := toolSet.Invoke(context.Background(), ToolInvocation{ToolName: "message_send", Input: json.RawMessage(`{"message":"보냅니다"}`)})

	if errorValue != nil || result.Failed() || *handlerCallCount != 1 {
		t.Fatalf("expected one ordinary invocation, calls=%d result=%+v", *handlerCallCount, result)
	}
}

func TestANarrowedToolSetKeepsTheGateItWasBuiltWith(t *testing.T) {
	gate := &recordingToolCallGate{withheldToolName: "message_send"}
	toolSet, handlerCallCount := gatedToolSet(t, gate)

	narrowedToolSet := toolSet.WithAllowedToolNames([]string{"message_send"})
	narrowedToolSet.Invoke(context.Background(), ToolInvocation{ToolName: "message_send", Input: json.RawMessage(`{"message":"보냅니다"}`)})

	if *handlerCallCount != 0 || len(gate.reviewedToolNames) != 1 {
		t.Fatalf("a narrowed tool set that drops the gate is how an approved-only call runs unapproved, calls=%d reviewed=%+v", *handlerCallCount, gate.reviewedToolNames)
	}
}

func TestAGateThatCannotDecideWithholdsTheCall(t *testing.T) {
	toolSet, handlerCallCount := gatedToolSet(t, &recordingToolCallGate{errorValue: context.DeadlineExceeded})

	result, errorValue := toolSet.Invoke(context.Background(), ToolInvocation{ToolName: "message_send", Input: json.RawMessage(`{"message":"보냅니다"}`)})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if *handlerCallCount != 0 || !result.Failed() {
		t.Fatalf("a gate that fails open lets an unreviewed call through, calls=%d result=%+v", *handlerCallCount, result)
	}
}

func TestAToolSetWithNoGateInvokesDirectly(t *testing.T) {
	toolSet, handlerCallCount := gatedToolSet(t, nil)

	result, errorValue := toolSet.Invoke(context.Background(), ToolInvocation{ToolName: "message_send", Input: json.RawMessage(`{"message":"보냅니다"}`)})

	if errorValue != nil || result.Failed() || *handlerCallCount != 1 {
		t.Fatalf("expected an ungated tool set to invoke directly, calls=%d result=%+v", *handlerCallCount, result)
	}
}

package toolcontract

import "context"

type ToolCallReview struct {
	MayProceed bool
	Result     ToolResult
}

type ToolCallGate interface {
	ReviewToolCall(context.Context, ToolInvocation, ToolDefinition) (ToolCallReview, error)
}

func (toolSet *ToolSet) UseToolCallGate(toolCallGate ToolCallGate) {
	if toolSet == nil {
		return
	}
	toolSet.toolCallGate = toolCallGate
}

func (toolSet *ToolSet) reviewToolCall(ctx context.Context, toolInvocation ToolInvocation, toolDefinition ToolDefinition) (ToolResult, bool) {
	if toolSet.toolCallGate == nil {
		return ToolResult{}, false
	}
	review, errorValue := toolSet.toolCallGate.ReviewToolCall(ctx, toolInvocation, toolDefinition)
	if errorValue != nil {
		return ToolFailureResult(FailureUnknown, FailureCodes.OperationFailed, "tool_call_gate", errorValue.Error()), true
	}
	if review.MayProceed {
		return ToolResult{}, false
	}
	return review.Result, true
}

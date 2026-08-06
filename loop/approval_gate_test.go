package loop

import (
	"context"
	"encoding/json"
	"github.com/yeomyeonggeori/bluecollar/toolcontract"
	"testing"

	"github.com/yeomyeonggeori/bluecollar/taskstate"
)

func TestTerminalRunModelApprovalPausesBeforeExecution(t *testing.T) {
	terminalInput := `{"command":"publish-release","approvalRequired":true,"approvalReason":"This command publishes the release."}`
	languageModel := &sequenceLanguageModel{contents: []string{
		`{"action":"continue","toolName":"terminal_run","toolInput":` + terminalInput + `}`,
		`{"question":"릴리스를 게시할까요?"}`,
		finishMessageDocument("릴리스를 게시했습니다."),
	}}
	services := newTurnRunnerTestServices(languageModel, TurnOptions{MaxIterationCount: 4})
	toolSet := newTestToolSet([]string{toolcontract.TerminalRunToolName})
	invokedInputs := []string{}
	registerTestTool(toolSet, toolcontract.ToolDefinition{Name: toolcontract.TerminalRunToolName}, func(_ context.Context, invocation toolcontract.ToolInvocation) (toolcontract.ToolResult, error) {
		invokedInputs = append(invokedInputs, string(invocation.Input))
		return testToolSuccess(`{"status":"published"}`), nil
	})

	firstResult, errorValue := services.runner.RunTurn(context.Background(), AgentTurnRequest{
		RequesterPersonID: "person-1",
		ConversationID:    "conversation-1",
		Prompt:            "릴리스를 게시해줘",
		ResponseLanguage:  ResponseLanguageKorean,
		ToolSet:           toolSet,
		WorkspaceRootPath: t.TempDir(),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if firstResult.TaskRun.Status != taskstate.TaskStatusWaitingApproval || len(invokedInputs) != 0 {
		t.Fatalf("expected approval before terminal execution, calls=%d result=%+v", len(invokedInputs), firstResult)
	}
}

func TestApprovalContinuationKeepsPrePauseObservations(t *testing.T) {
	languageModel := &sequenceLanguageModel{contents: []string{
		`{"action":"continue","toolName":"message_search","toolInput":{"queries":["고객지원 월간회의"]}}`,
		`{"action":"continue","toolName":"message_delete","toolInput":{"messageIDs":["message-1"]}}`,
		`{"question":"메모를 삭제할까요?"}`,
		finishMessageDocument("메모를 삭제했습니다."),
	}}
	services := newTurnRunnerTestServices(languageModel, TurnOptions{MaxIterationCount: 6})
	toolRegistry := newTestCapabilityToolSet([]string{"message_search", "message_delete"})
	searchCallCount := 0
	registerTestTool(toolRegistry, toolcontract.ToolDefinition{Name: "message_search"}, func(context.Context, toolcontract.ToolInvocation) (toolcontract.ToolResult, error) {
		searchCallCount++
		return testToolSuccess(`{"messageIDs":["message-1"]}`), nil
	})
	deleteCallCount := 0
	registerTestTool(toolRegistry, toolcontract.ToolDefinition{Name: "message_delete", RequiresApproval: true}, func(context.Context, toolcontract.ToolInvocation) (toolcontract.ToolResult, error) {
		deleteCallCount++
		return testToolSuccess(`{"deletedMessageIDs":["message-1"]}`), nil
	})

	firstResult, errorValue := services.runner.RunTurn(context.Background(), AgentTurnRequest{
		RequesterPersonID:     "person-1",
		ConversationID:        "conversation-1",
		Prompt:                "고객지원 월간회의 메모를 찾아서 삭제해줘",
		ResponseLanguage:      ResponseLanguageKorean,
		ToolSet:               toolRegistry,
		PinnedToolNames:       toolRegistry.ListToolNames(),
		RequiredEvidenceTools: []string{"message_search", "message_delete"},
		WorkspaceRootPath:     t.TempDir(),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if firstResult.TaskRun.Status != taskstate.TaskStatusWaitingApproval || searchCallCount != 1 || deleteCallCount != 0 {
		t.Fatalf("expected search then held delete, got search=%d delete=%d result=%+v", searchCallCount, deleteCallCount, firstResult)
	}

	if _, errorValue := services.taskRunService.AdvanceTaskRun(firstResult.TaskRun.TaskRunID, "assistant"); errorValue != nil {
		t.Fatal(errorValue)
	}
	secondResult, errorValue := services.runner.RunTurn(context.Background(), AgentTurnRequest{
		RequesterPersonID:      "person-1",
		ExistingTaskRunID:      firstResult.TaskRun.TaskRunID,
		IsApprovalContinuation: true,
		ConversationID:         "conversation-1",
		Prompt:                 "승인",
		ResponseLanguage:       ResponseLanguageKorean,
		ToolSet:                toolRegistry,
		PinnedToolNames:        toolRegistry.ListToolNames(),
		RequiredEvidenceTools:  []string{"message_search", "message_delete"},
		WorkspaceRootPath:      t.TempDir(),
		CarriedOutCalls: []CarriedOutCall{{
			ToolName:  "message_delete",
			ToolInput: json.RawMessage(`{"messageIDs":["message-1"]}`),
			Result:    testToolSuccess(`{"deletedMessageIDs":["message-1"]}`),
		}},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if secondResult.TaskRun.Status != taskstate.TaskStatusCompleted {
		t.Fatalf("expected the continuation to finish from restored evidence, got %+v", secondResult)
	}
	if searchCallCount != 1 || deleteCallCount != 0 {
		t.Fatalf("expected the pre-pause search to survive the continuation without either call running again, got search=%d delete=%d", searchCallCount, deleteCallCount)
	}
}

func TestCurrentThreadSendSkipsRuntimeApproval(t *testing.T) {
	sendDefinition := testToolDescriptor("message_send")
	sendDefinition.RequiresApproval = true
	sendDefinition.SideEffectClass = toolcontract.ToolSideEffectExternalSend
	toolSet := newTestToolSetWithDefinitions([]toolcontract.ToolDefinition{sendDefinition})

	currentThreadCall := turnActionDocument{ToolName: "message_send", ToolInput: json.RawMessage(`{"targetType":"currentThread","message":"요약"}`)}
	if toolCallRequiresRuntimeApproval(toolSet, currentThreadCall) {
		t.Fatal("expected a current-thread send to run without approval, like a reply")
	}
	currentChannelCall := turnActionDocument{ToolName: "message_send", ToolInput: json.RawMessage(`{"targetType":"currentChannel","message":"메모"}`)}
	if toolCallRequiresRuntimeApproval(toolSet, currentChannelCall) {
		t.Fatal("expected a current-channel send to run without approval, like a reply")
	}
	directMessageCall := turnActionDocument{ToolName: "message_send", ToolInput: json.RawMessage(`{"targetType":"directMessage","personHint":"테스트","message":"안내"}`)}
	if !toolCallRequiresRuntimeApproval(toolSet, directMessageCall) {
		t.Fatal("expected an external send to keep requiring approval")
	}
}

func TestASecondCallOnAnApprovalContinuationIsStillHeld(t *testing.T) {
	languageModel := &sequenceLanguageModel{contents: []string{
		directToolAction("continue", "", "task_delete", `{"taskID":"task-A"}`),
		`{"question":"task-A를 삭제할까요?"}`,
		directToolAction("continue", "", "task_delete", `{"taskID":"task-B"}`),
		`{"question":"task-B도 삭제할까요?"}`,
	}}
	services := newTurnRunnerTestServices(languageModel, TurnOptions{MaxIterationCount: 4})
	toolRegistry := newTestCapabilityToolSet([]string{"task_delete"})
	invokedInputs := []string{}
	registerTestTool(toolRegistry, toolcontract.ToolDefinition{Name: "task_delete", RequiresApproval: true}, func(_ context.Context, invocation toolcontract.ToolInvocation) (toolcontract.ToolResult, error) {
		invokedInputs = append(invokedInputs, string(invocation.Input))
		return testToolSuccess(`{"status":"deleted"}`), nil
	})

	firstResult, errorValue := services.runner.RunTurn(context.Background(), AgentTurnRequest{
		RequesterPersonID: "person-1",
		ConversationID:    "conversation-1",
		Prompt:            "task-A 삭제해줘",
		ResponseLanguage:  ResponseLanguageKorean,
		ToolSet:           toolRegistry,
		PinnedToolNames:   []string{"task_delete"},
		WorkspaceRootPath: t.TempDir(),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if firstResult.TaskRun.Status != taskstate.TaskStatusWaitingApproval {
		t.Fatalf("expected the first call to be held, result=%+v", firstResult)
	}
	services.taskRunService.AppendTaskEvent(firstResult.TaskRun.TaskRunID, "approval.executed", `{"toolName":"task_delete","toolInput":{"taskID":"task-A"}}`)
	if _, errorValue := services.taskRunService.AdvanceTaskRun(firstResult.TaskRun.TaskRunID, "assistant"); errorValue != nil {
		t.Fatal(errorValue)
	}

	continuationResult, errorValue := services.runner.RunTurn(context.Background(), AgentTurnRequest{
		RequesterPersonID:      "person-1",
		ExistingTaskRunID:      firstResult.TaskRun.TaskRunID,
		IsApprovalContinuation: true,
		ConversationID:         "conversation-1",
		ResponseLanguage:       ResponseLanguageKorean,
		ToolSet:                toolRegistry,
		PinnedToolNames:        []string{"task_delete"},
		WorkspaceRootPath:      t.TempDir(),
		CarriedOutCalls: []CarriedOutCall{{
			ToolName:  "task_delete",
			ToolInput: json.RawMessage(`{"taskID":"task-A"}`),
			Result:    testToolSuccess(`{"status":"deleted"}`),
		}},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(invokedInputs) != 0 {
		t.Fatalf("a call the requester never saw ran on the strength of someone else's approval, invoked %+v", invokedInputs)
	}
	if continuationResult.TaskRun.Status != taskstate.TaskStatusWaitingApproval {
		t.Fatalf("expected the second call to be held for its own approval, result=%+v", continuationResult)
	}
}

package loop

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yeomyeonggeori/bluecollar/model"
	"github.com/yeomyeonggeori/bluecollar/toolcontract"
)

type sharedValueLanguageModel struct {
	inner *sequenceLanguageModel
}

func (languageModel sharedValueLanguageModel) GenerateResponse(ctx context.Context, prompt string) (string, error) {
	return languageModel.inner.GenerateResponse(ctx, prompt)
}

func (languageModel sharedValueLanguageModel) GenerateStructuredResponse(ctx context.Context, request model.StructuredResponseRequest) (model.StructuredResponse, error) {
	return languageModel.inner.GenerateStructuredResponse(ctx, request)
}

func TestEscalationFiresWhenTwoTiersAreConfiguredWithTheSameModel(t *testing.T) {
	startingTier := &sequenceLanguageModel{
		modelTier: "xlow",
		contents: []string{
			`{"action":"continue","toolName":"file_write","toolInput":{"path":"tmp/app/index.html","content":"one"}}`,
			`{"action":"continue","toolName":"terminal_run","toolInput":{"command":"npm run build"}}`,
		},
	}
	tierProvider := sharedValueLanguageModel{inner: startingTier}
	startingTier.contents = append(startingTier.contents, finishMessageDocument("continued after the escalation"))
	services := newTurnRunnerTestServices(tierProvider, TurnOptions{
		TaskLevel:         TaskLevelXLow,
		MaxIterationCount: 2,
		MaxToolCallCount:  10,
	})
	services.runner.UseTaskLevelLanguageModelResolver(func(TaskLevel) model.LanguageModelProvider {
		return sharedValueLanguageModel{inner: startingTier}
	})
	toolRegistry := newTestToolSet([]string{"file_write", "terminal_run"})
	registerTestTool(toolRegistry, toolcontract.ToolDefinition{Name: "file_write"}, func(context.Context, toolcontract.ToolInvocation) (toolcontract.ToolResult, error) {
		return testToolSuccess(`{"path":"tmp/app/index.html"}`), nil
	})
	registerTestTool(toolRegistry, terminalRunTestToolDefinition(), func(context.Context, toolcontract.ToolInvocation) (toolcontract.ToolResult, error) {
		data := json.RawMessage(`{"mode":"command","completed":true,"exitCode":0,"stdout":"built","stderr":"","timedOut":false,"outputTrimmed":false}`)
		return toolcontract.ToolSuccessData(string(data), data), nil
	})

	result, errorValue := services.runner.RunTurn(context.Background(), AgentTurnRequest{
		RequesterPersonID: "person-1",
		ConversationID:    "conversation-1",
		Prompt:            "build the site",
		ToolSet:           toolRegistry,
		PinnedToolNames:   []string{"file_write", "terminal_run"},
	})
	if errorValue != nil {
		t.Fatalf("expected the escalated run to finish: %v", errorValue)
	}

	if !taskEventsContain(services.taskEventService.ListTaskEvent(result.TaskRun.TaskRunID), "agent.model_escalated", `"newTaskLevel":"low"`) {
		t.Fatal("two tiers that resolve to equal provider values still escalate, and an escalation the ledger does not record is one nobody can audit")
	}
}

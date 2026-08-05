package loop

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/bluecollar/toolcontract"
)

func TestATurnStartsKnowingWhatTheHostAlreadyCarriedOut(t *testing.T) {
	languageModel := &sequenceLanguageModel{
		modelTier: "xlow",
		contents:  []string{finishMessageDocument("이미 보냈습니다")},
	}
	services := newTurnRunnerTestServices(languageModel, TurnOptions{TaskLevel: TaskLevelXLow, MaxIterationCount: 2, MaxToolCallCount: 5})

	result, errorValue := services.runner.RunTurn(context.Background(), AgentTurnRequest{
		RequesterPersonID: "person-1",
		ConversationID:    "conversation-1",
		Prompt:            "회의록 보내줘",
		ToolSet:           newTestToolSet([]string{"message_send"}),
		CarriedOutCalls: []CarriedOutCall{{
			ToolName:  "message_send",
			ToolInput: json.RawMessage(`{"to":["alice"],"message":"회의록"}`),
			Result:    toolcontract.ToolSuccessData("sent to alice", json.RawMessage(`{"messageID":"m-1"}`)),
		}},
	})
	if errorValue != nil {
		t.Fatalf("expected the turn to run: %v", errorValue)
	}

	taskEvents := services.taskEventService.ListTaskEvent(result.TaskRun.TaskRunID)
	if !taskEventsContain(taskEvents, "tool.message_send.result", "sent to alice") {
		t.Fatalf("a call the host carried out has to reach the ledger as the loop's own observation, got %d events", len(taskEvents))
	}
	if !strings.Contains(strings.Join(promptsOf(languageModel), "\n"), "sent to alice") {
		t.Fatal("the model has to see what the host already did, or it will ask for it again")
	}
}

func promptsOf(languageModel *sequenceLanguageModel) []string {
	prompts := []string{}
	for _, request := range languageModel.requests {
		for _, message := range request.Messages {
			prompts = append(prompts, message.Content)
		}
	}
	return prompts
}

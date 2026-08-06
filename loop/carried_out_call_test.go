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

func TestACarriedOutCallDoesNotReuseAnObservationIDTheLedgerAlreadyHolds(t *testing.T) {
	languageModel := &sequenceLanguageModel{contents: []string{
		finishMessageDocument("삭제했습니다."),
	}}
	services := newTurnRunnerTestServices(languageModel, TurnOptions{MaxIterationCount: 4})
	taskRun := services.taskRunService.CreateTaskRun("person-1", "conversation-1", "그 일정 삭제해줘")
	services.taskRunService.AppendTaskEvent(taskRun.TaskRunID, "tool.calendar_delete.result", `{"observationID":"obs-001","tool":"calendar_delete"}`)

	services.runner.RunTurn(context.Background(), AgentTurnRequest{
		RequesterPersonID: "person-1",
		ExistingTaskRunID: taskRun.TaskRunID,
		ConversationID:    "conversation-1",
		Prompt:            "확인",
		WorkspaceRootPath: t.TempDir(),
		CarriedOutCalls: []CarriedOutCall{{
			ToolName:  "calendar_delete",
			ToolInput: json.RawMessage(`{"eventHint":"calendar-event-001"}`),
			Result:    testToolSuccess(`{"status":"deleted"}`),
		}},
	})

	recordedResults := []string{}
	for _, taskEvent := range services.taskEventService.ListTaskEvent(taskRun.TaskRunID) {
		if taskEvent.Name == "tool.calendar_delete.result" {
			recordedResults = append(recordedResults, taskEvent.Body)
		}
	}
	if len(recordedResults) != 2 {
		t.Fatalf("expected the seeded observation and the carried out one, got %+v", recordedResults)
	}
	if strings.Contains(recordedResults[1], `"observationID":"obs-001"`) {
		t.Fatalf("the carried out call took an observation ID the ledger already holds, body=%s", recordedResults[1])
	}
}

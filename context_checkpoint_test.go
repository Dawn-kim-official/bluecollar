package bluecollar

import (
	"strings"
	"testing"

	"github.com/Dawn-kim-official/bluecollar/taskstate"
	"github.com/Dawn-kim-official/bluecollar/toolcontract"
)

func toolResultTaskEvent(taskEventID string, observationID string) taskstate.TaskEvent {
	return taskstate.TaskEvent{
		TaskEventID: taskEventID,
		Name:        "tool.note_write.result",
		Body: marshalEventBody(turnObservation{
			ObservationID: observationID,
			Action:        "continue",
			Tool:          "note_write",
			Output:        toolcontract.ToolOutput{Content: "wrote " + observationID},
		}),
	}
}

func unreadableTaskEvent(taskEventID string) taskstate.TaskEvent {
	return taskstate.TaskEvent{TaskEventID: taskEventID, Name: "tool.note_write.result", Body: "{this is not decodable"}
}

func checkpointTaskEvent(summary TaskContextSummary) taskstate.TaskEvent {
	return taskstate.TaskEvent{
		TaskEventID: "event-checkpoint",
		Name:        taskContextSummaryEventName,
		Body:        marshalEventBody(summary),
	}
}

func retainedContextCheckpoint() TaskContextSummary {
	return TaskContextSummary{
		ObservationID:                 "context-summary-observation-3",
		CompactedThroughObservationID: "observation-3",
		CompactedObservationIDs:       []string{"observation-1", "observation-2", "observation-3"},
		AccountedTaskEventIDs:         []string{"event-1", "event-2", "event-3", "event-4"},
		RetainedObservations:          []turnObservation{{ObservationID: "observation-4", Action: "continue", Tool: "note_write"}},
		CompactedObservationCount:     3,
		CompactedToolCallCount:        3,
		Goal:                          "ship",
	}
}

func TestAResumeRebuildsContextFromTheCheckpointAloneWithoutTheEventsItAccountsFor(t *testing.T) {
	events := []taskstate.TaskEvent{
		unreadableTaskEvent("event-1"),
		unreadableTaskEvent("event-2"),
		unreadableTaskEvent("event-3"),
		unreadableTaskEvent("event-4"),
		checkpointTaskEvent(retainedContextCheckpoint()),
		toolResultTaskEvent("event-5", "observation-5"),
	}

	state, errorValue := restoreAgentTaskState(AgentTurnRequest{Prompt: "ship"}, TurnOptions{}, taskstate.TaskRun{TaskRunID: "task-1", Status: taskstate.TaskStatusRunning}, events)

	if errorValue != nil {
		t.Fatalf("expected the resume to rebuild state: %v", errorValue)
	}
	if len(state.Observations) != 2 {
		t.Fatalf("expected the retained observation and the one after the checkpoint, got %d", len(state.Observations))
	}
	if state.Observations[0].ObservationID != "observation-4" || state.Observations[1].ObservationID != "observation-5" {
		t.Fatalf("expected observation-4 then observation-5, got %s and %s", state.Observations[0].ObservationID, state.Observations[1].ObservationID)
	}
}

func TestACheckpointDoesNotLoseTheWorkItAbsorbed(t *testing.T) {
	events := []taskstate.TaskEvent{
		toolResultTaskEvent("event-1", "observation-1"),
		toolResultTaskEvent("event-2", "observation-2"),
		toolResultTaskEvent("event-3", "observation-3"),
		toolResultTaskEvent("event-4", "observation-4"),
		checkpointTaskEvent(retainedContextCheckpoint()),
		toolResultTaskEvent("event-5", "observation-5"),
	}

	state, errorValue := restoreAgentTaskState(AgentTurnRequest{Prompt: "ship"}, TurnOptions{}, taskstate.TaskRun{TaskRunID: "task-1", Status: taskstate.TaskStatusRunning}, events)

	if errorValue != nil {
		t.Fatalf("expected the resume to rebuild state: %v", errorValue)
	}
	if state.IterationCount != 5 {
		t.Fatalf("compaction must not give the task iterations back; expected 5, got %d", state.IterationCount)
	}
	if state.ToolCallCount != 5 {
		t.Fatalf("compaction must not hide completed tool calls; expected 5, got %d", state.ToolCallCount)
	}
}

func TestATaskWithNoCheckpointStillResumesFromItsWholeHistory(t *testing.T) {
	events := []taskstate.TaskEvent{
		toolResultTaskEvent("event-1", "observation-1"),
		toolResultTaskEvent("event-2", "observation-2"),
	}

	state, errorValue := restoreAgentTaskState(AgentTurnRequest{Prompt: "ship"}, TurnOptions{}, taskstate.TaskRun{TaskRunID: "task-1", Status: taskstate.TaskStatusRunning}, events)

	if errorValue != nil {
		t.Fatalf("expected the resume to rebuild state: %v", errorValue)
	}
	if len(state.Observations) != 2 || state.IterationCount != 2 {
		t.Fatalf("expected the full history, got %d observations and iteration count %d", len(state.Observations), state.IterationCount)
	}
}

func TestTheCheckpointBookkeepingNeverReachesTheModel(t *testing.T) {
	renderedSummary := summaryObservation(retainedContextCheckpoint()).ContentText()

	if strings.Contains(renderedSummary, "accountedTaskEventIDs") || strings.Contains(renderedSummary, "retainedObservations") {
		t.Fatalf("the checkpoint's restore bookkeeping is not context for the model, got %s", renderedSummary)
	}
	if !strings.Contains(renderedSummary, "ship") {
		t.Fatalf("expected the summary the model reads to survive, got %s", renderedSummary)
	}
}


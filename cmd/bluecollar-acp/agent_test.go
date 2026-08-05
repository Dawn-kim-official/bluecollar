package main

import (
	"context"
	"io"
	"strings"
	"testing"

	acp "github.com/coder/acp-go-sdk"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yeomyeonggeori/bluecollar/model"
	"github.com/yeomyeonggeori/bluecollar/taskstate"
)

type scriptedLanguageModel struct {
	contents  []string
	callCount int
}

func (languageModel *scriptedLanguageModel) GenerateResponse(context.Context, string) (string, error) {
	return "", nil
}

func (languageModel *scriptedLanguageModel) GenerateStructuredResponse(_ context.Context, request model.StructuredResponseRequest) (model.StructuredResponse, error) {
	if request.StructuredOutputSchema.Name != "bluecollar_agent_turn_action" {
		return model.StructuredResponse{Content: `{"route":"start_task","classification":"bounded_task","taskShape":"maintenance_task","level":"xlow","estimatedMinutes":1,"responseLanguage":"en","reason":"test"}`}, nil
	}
	if languageModel.callCount >= len(languageModel.contents) {
		return model.StructuredResponse{Content: `{"action":"finish","message":"done","goalSatisfied":true}`}, nil
	}
	content := languageModel.contents[languageModel.callCount]
	languageModel.callCount++
	return model.StructuredResponse{Content: content}, nil
}

type hostToolCall struct {
	toolName string
}

func publishedCatalog(t *testing.T, calls *[]hostToolCall) *mcp.Server {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "host", Version: "test"}, nil)
	server.AddTool(&mcp.Tool{
		Name:        "note_write",
		Description: "write a note",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{"text": map[string]any{"type": "string"}}},
		Meta:        mcp.Meta{"blueclaw/sideEffectClass": "state_change"},
	}, func(_ context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		*calls = append(*calls, hostToolCall{toolName: "note_write"})
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "note written"}}}, nil
	})
	return server
}

type hostClient struct {
	agentMessage string
}

func (client *hostClient) SessionUpdate(_ context.Context, notification acp.SessionNotification) error {
	if chunk := notification.Update.AgentMessageChunk; chunk != nil && chunk.Content.Text != nil {
		client.agentMessage += chunk.Content.Text.Text
	}
	return nil
}

func (client *hostClient) RequestPermission(context.Context, acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	return acp.RequestPermissionResponse{Outcome: acp.RequestPermissionOutcome{Cancelled: &acp.RequestPermissionOutcomeCancelled{Outcome: "cancelled"}}}, nil
}

func (client *hostClient) ReadTextFile(context.Context, acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	return acp.ReadTextFileResponse{}, io.ErrUnexpectedEOF
}
func (client *hostClient) WriteTextFile(context.Context, acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	return acp.WriteTextFileResponse{}, io.ErrUnexpectedEOF
}
func (client *hostClient) CreateTerminal(context.Context, acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{}, io.ErrUnexpectedEOF
}
func (client *hostClient) KillTerminal(context.Context, acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	return acp.KillTerminalResponse{}, io.ErrUnexpectedEOF
}
func (client *hostClient) TerminalOutput(context.Context, acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, io.ErrUnexpectedEOF
}
func (client *hostClient) ReleaseTerminal(context.Context, acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, io.ErrUnexpectedEOF
}
func (client *hostClient) WaitForTerminalExit(context.Context, acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, io.ErrUnexpectedEOF
}

func TestAHostDrivesTheLoopOverACPAndItsToolsComeFromTheCatalog(t *testing.T) {
	hostCalls := []hostToolCall{}
	catalogServer := publishedCatalog(t, &hostCalls)
	catalogClientTransport, catalogServerTransport := mcp.NewInMemoryTransports()
	go catalogServer.Run(t.Context(), catalogServerTransport)

	languageModel := &scriptedLanguageModel{contents: []string{
		`{"action":"continue","toolName":"note_write","toolInput":{"text":"회의록"}}`,
		`{"action":"finish","message":"노트를 남겼습니다","goalSatisfied":true,"completionEvidenceIDs":["obs-001"]}`,
	}}

	agentInputReader, agentInputWriter := io.Pipe()
	agentOutputReader, agentOutputWriter := io.Pipe()
	runningAgent := newAgent(languageModel, "bluecollar")
	runningAgent.resolveTransport = func(acp.McpServer) (mcp.Transport, error) { return catalogClientTransport, nil }
	go func() {
		connection := acp.NewAgentSideConnection(runningAgent, agentOutputWriter, agentInputReader)
		<-connection.Done()
	}()

	host := &hostClient{}
	connection := acp.NewClientSideConnection(host, agentInputWriter, agentOutputReader)
	if _, errorValue := connection.Initialize(t.Context(), acp.InitializeRequest{ProtocolVersion: acp.ProtocolVersionNumber}); errorValue != nil {
		t.Fatalf("initialize: %v", errorValue)
	}
	newSession, errorValue := connection.NewSession(t.Context(), acp.NewSessionRequest{Cwd: t.TempDir(), McpServers: []acp.McpServer{{Stdio: &acp.McpServerStdio{Name: "host"}}}})
	if errorValue != nil {
		t.Fatalf("session/new: %v", errorValue)
	}
	promptResponse, errorValue := connection.Prompt(t.Context(), acp.PromptRequest{
		SessionId: newSession.SessionId,
		Prompt:    []acp.ContentBlock{acp.TextBlock("회의록 정리해줘")},
	})
	if errorValue != nil {
		t.Fatalf("session/prompt: %v", errorValue)
	}

	if promptResponse.StopReason == "" {
		t.Fatal("the host has to learn how the turn ended")
	}
	if len(hostCalls) != 1 || hostCalls[0].toolName != "note_write" {
		t.Fatalf("the loop owns no tools, so the work has to land on the host's catalog, got %+v\n%s", hostCalls, ledgerOf(runningAgent, newSession.SessionId))
	}
}

func ledgerOf(runningAgent *agent, sessionID acp.SessionId) string {
	openSession, isKnown := runningAgent.session(sessionID)
	if !isKnown {
		return "(no session)"
	}
	lines := []string{}
	for _, taskEvent := range openSession.taskRuns.ListTaskEvent(openSession.taskRunID) {
		lines = append(lines, "  "+taskEvent.Name+"  "+truncate(taskEvent.Body, 200))
	}
	return strings.Join(lines, "\n")
}

func truncate(text string, limit int) string {
	collapsed := strings.Join(strings.Fields(text), " ")
	if len(collapsed) <= limit {
		return collapsed
	}
	return collapsed[:limit] + "…"
}

func TestTheLoopsVerdictReachesTheHostAsAStopReason(t *testing.T) {
	for status, expectedStopReason := range map[taskstate.TaskStatus]acp.StopReason{
		taskstate.TaskStatusCompleted: acp.StopReasonEndTurn,
		taskstate.TaskStatusCancelled: acp.StopReasonCancelled,
		taskstate.TaskStatusBlocked:   acp.StopReasonRefusal,
	} {
		if stopReason := stopReasonForStatus(status); stopReason != expectedStopReason {
			t.Fatalf("a task the loop left %q reaches the host as %q, expected %q", status, stopReason, expectedStopReason)
		}
	}
}

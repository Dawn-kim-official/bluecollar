package loop

import (
	"github.com/yeomyeonggeori/bluecollar/toolcontract"
	"strings"
	"testing"
)

func TestSystemInstructionAddsApprovalContinuationDirective(t *testing.T) {
	withContinuation := buildAgentSystemInstruction(AgentTurnRequest{IsApprovalContinuation: true, ConversationID: "conversation-1", ToolSet: newTestToolSet([]string{toolcontract.AskInputToolName})})
	if !strings.Contains(withContinuation, "runtime has already performed") {
		t.Fatalf("expected approval-continuation directive in the instruction")
	}

	withoutContinuation := buildAgentSystemInstruction(AgentTurnRequest{IsApprovalContinuation: false, ConversationID: "conversation-1", ToolSet: newTestToolSet([]string{toolcontract.AskInputToolName})})
	if strings.Contains(withoutContinuation, "runtime has already performed") {
		t.Fatal("did not expect the approval-continuation directive without a continuation")
	}
}

func TestSystemInstructionGuidesBareMentionAndPlayfulReplies(t *testing.T) {
	instruction := buildAgentSystemInstruction(AgentTurnRequest{AgentIdentity: AgentIdentity{Name: "Ada", Handle: "ada"}, ConversationID: "conversation-1"})

	for _, expected := range []string{"only mentions you", "@ada", "recent visible conversation context", "Do not silently ignore", "good-humored coworker"} {
		if !strings.Contains(instruction, expected) {
			t.Fatalf("expected system instruction to contain %q, got %s", expected, instruction)
		}
	}
}

func TestSystemInstructionRestrictsCheckpointsAndRequiresRecovery(t *testing.T) {
	instruction := buildAgentSystemInstruction(AgentTurnRequest{ConversationID: "conversation-1", ToolSet: newTestToolSet([]string{toolcontract.AskInputToolName})})

	for _, expected := range []string{
		"Do not use continue.message as a pre-tool repeat-back",
		"meaningful intermediate progress",
		"finish is the permanent final messenger reply",
		"Do not give up after one failed attempt",
		"checking which stage failed",
		"delivery is genuinely blocked",
	} {
		if !strings.Contains(instruction, expected) {
			t.Fatalf("expected system instruction to contain %q, got %s", expected, instruction)
		}
	}
}

func TestSystemInstructionRequiresConcreteReadResults(t *testing.T) {
	instruction := buildAgentSystemInstruction(AgentTurnRequest{ConversationID: "conversation-1", ToolSet: newTestToolSet([]string{toolcontract.AskInputToolName})})
	for _, expected := range []string{"final reply must state the concrete result facts", "status-only reply"} {
		if !strings.Contains(instruction, expected) {
			t.Fatalf("expected system instruction to contain %q, got %s", expected, instruction)
		}
	}
}

func TestSystemInstructionGuidesApprovalMessageAsNaturalQuestion(t *testing.T) {
	instruction := buildAgentSystemInstruction(AgentTurnRequest{ConversationID: "conversation-1", ToolSet: newTestToolSet([]string{toolcontract.AskInputToolName})})

	for _, expected := range []string{
		"natural user-facing question",
		"테스트에게 다음 내용을 보낼까요?",
		"instead of naming internal tools or operations",
	} {
		if !strings.Contains(instruction, expected) {
			t.Fatalf("expected system instruction to contain %q, got %s", expected, instruction)
		}
	}
}

func TestAWorkspaceTaskIsNotToldAboutMessengersItHasNone(t *testing.T) {
	workspaceOnly := buildAgentSystemInstruction(AgentTurnRequest{
		ToolSet: newTestToolSet([]string{toolcontract.TerminalRunToolName}),
	})

	for _, absent := range []string{"Bare mentions and banter", "Recipients:", "Delivery and artifacts", "Approvals and user input", "Skills:"} {
		if strings.Contains(workspaceOnly, absent) {
			t.Fatalf("a container with a shell and no conversation was carrying %q: the instruction ran to 12,753 bytes against a 136 byte task, and every byte of it competes with the work", absent)
		}
	}
	if !strings.Contains(workspaceOnly, "Failure recovery:") {
		t.Fatal("what applies to every task stays")
	}
}

func TestATaskWithAConversationKeepsItsMessengerGuidance(t *testing.T) {
	messenger := buildAgentSystemInstruction(AgentTurnRequest{
		ConversationID: "conversation-1",
		ToolSet:        newTestToolSet([]string{toolcontract.AskInputToolName}),
	})

	for _, present := range []string{"Bare mentions and banter", "Recipients:", "Approvals and user input"} {
		if !strings.Contains(messenger, present) {
			t.Fatalf("a task that does speak to people still needs %q", present)
		}
	}
}

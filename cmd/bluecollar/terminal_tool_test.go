package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/bluecollar/toolcontract"
)

func invokeTerminalRun(t *testing.T, workspacePath string, input string) toolcontract.ToolResult {
	t.Helper()
	toolSet := newWorkspaceToolSet(workspacePath)
	result, errorValue := toolSet.Invoke(context.Background(), toolcontract.ToolInvocation{
		ToolName: toolcontract.TerminalRunToolName,
		Input:    json.RawMessage(input),
	})
	if errorValue != nil {
		t.Fatalf("expected the invocation to return a result: %v", errorValue)
	}
	return result
}

func decodedOutput(t *testing.T, result toolcontract.ToolResult) terminalRunOutput {
	t.Helper()
	output := terminalRunOutput{}
	if errorValue := json.Unmarshal(result.Output.Data, &output); errorValue != nil {
		t.Fatalf("expected structured output, got %q: %v", result.Output.Data, errorValue)
	}
	return output
}

func TestTheAgentSeesWhatTheCommandPrinted(t *testing.T) {
	result := invokeTerminalRun(t, t.TempDir(), `{"command":"echo hello from the shell"}`)

	if result.Failed() {
		t.Fatalf("expected a successful command, got %+v", result)
	}
	output := decodedOutput(t, result)
	if !strings.Contains(output.Output, "hello from the shell") || output.ExitCode != 0 {
		t.Fatalf("expected the printed line and a zero exit, got %+v", output)
	}
}

func TestACommandRunsWhereTheWorkIs(t *testing.T) {
	workspacePath := t.TempDir()
	if errorValue := os.WriteFile(filepath.Join(workspacePath, "marker.txt"), []byte("found"), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}

	result := invokeTerminalRun(t, workspacePath, `{"command":"cat marker.txt"}`)

	if !strings.Contains(decodedOutput(t, result).Output, "found") {
		t.Fatalf("a shell that starts somewhere else cannot do the task it was given, got %+v", result)
	}
}

func TestAFailingCommandReportsItsExitCodeRatherThanVanishing(t *testing.T) {
	result := invokeTerminalRun(t, t.TempDir(), `{"command":"echo to stderr 1>&2; exit 3"}`)

	output := decodedOutput(t, result)
	if output.ExitCode != 3 {
		t.Fatalf("expected the exit code the agent has to react to, got %+v", output)
	}
	if !strings.Contains(output.Output, "to stderr") {
		t.Fatalf("expected stderr captured alongside stdout, got %+v", output)
	}
}

func TestACommandThatNeverEndsIsStoppedAndSaidSo(t *testing.T) {
	result := invokeTerminalRun(t, t.TempDir(), `{"command":"sleep 30","timeoutSecond":1}`)

	if !result.Failed() {
		t.Fatalf("expected the hung command to come back as a failure, got %+v", result)
	}
	if !strings.Contains(result.UserSafeFailureSummary(), "still running") {
		t.Fatalf("expected the agent to learn why it stopped, got %q", result.UserSafeFailureSummary())
	}
}

func TestAnEmptyCommandIsRefusedBeforeAShellIsStarted(t *testing.T) {
	result := invokeTerminalRun(t, t.TempDir(), `{"command":"   "}`)

	if !result.Failed() || result.FailureCode() != toolcontract.FailureCodes.InvalidInput.String() {
		t.Fatalf("expected an invalid input failure, got %+v", result)
	}
}

func TestALongOutputKeepsItsEndBecauseThatIsWhereTheErrorIs(t *testing.T) {
	result := invokeTerminalRun(t, t.TempDir(), `{"command":"head -c 40000 /dev/zero | tr '\\0' 'a'; echo THEEND"}`)

	output := decodedOutput(t, result)
	if !output.Truncated {
		t.Fatalf("expected the capture to report that it dropped the head, got %+v", output.Truncated)
	}
	if !strings.Contains(output.Output, "THEEND") {
		t.Fatal("truncating the tail throws away the failure the agent has to read")
	}
	if len(output.Output) > maximumCapturedOutput {
		t.Fatalf("expected the capture bounded, got %d bytes", len(output.Output))
	}
}

func TestARunnerToldToBringNoShellBringsNone(t *testing.T) {
	if turnToolSet(runOptions{withoutTools: true}) != nil {
		t.Fatal("expected no tool set when the runner is asked to answer from reasoning alone")
	}
	if turnToolSet(runOptions{workspacePath: t.TempDir()}) == nil {
		t.Fatal("expected a shell by default, because a runner with no tools cannot be benchmarked")
	}
}

func TestACommandTheModelMarkedForApprovalIsRefusedRatherThanRunUnasked(t *testing.T) {
	result := invokeTerminalRun(t, t.TempDir(), `{"command":"rm -rf /tmp/whatever","approvalRequired":true,"approvalReason":"destructive"}`)

	if !result.Failed() || result.FailureCode() != toolcontract.FailureCodes.InteractionRequired.String() {
		t.Fatalf("a runner with no requester must not answer for one, got %+v", result)
	}
	if !strings.Contains(result.UserSafeFailureSummary(), "nobody to ask") {
		t.Fatalf("expected the agent to learn why, got %q", result.UserSafeFailureSummary())
	}
}

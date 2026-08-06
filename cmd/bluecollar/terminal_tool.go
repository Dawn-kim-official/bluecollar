package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/yeomyeonggeori/bluecollar/toolcontract"
)

const (
	defaultCommandTimeout = 2 * time.Minute
	maximumCapturedOutput = 32 * 1024
)

type terminalRunInput struct {
	Command          string `json:"command"`
	TimeoutSecond    int    `json:"timeoutSecond"`
	ApprovalRequired bool   `json:"approvalRequired"`
	ApprovalReason   string `json:"approvalReason"`
}

type terminalRunOutput struct {
	ExitCode  int    `json:"exitCode"`
	Output    string `json:"output"`
	Truncated bool   `json:"truncated"`
}

var terminalRunInputSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "command": {"type": "string", "minLength": 1},
    "timeoutSecond": {"type": "integer"},
    "approvalRequired": {"type": "boolean"},
    "approvalReason": {"type": "string"}
  },
  "required": ["command"],
  "additionalProperties": false
}`)

var terminalRunOutputSchema = json.RawMessage(`{
  "type": "object",
  "properties": {
    "exitCode": {"type": "integer"},
    "output": {"type": "string"},
    "truncated": {"type": "boolean"}
  },
  "required": ["exitCode", "output"],
  "additionalProperties": false
}`)

func newWorkspaceToolSet(workspacePath string) *toolcontract.ToolSet {
	toolSet := toolcontract.NewToolSet([]string{toolcontract.TerminalRunToolName})
	toolcontract.RegisterToolFunction(toolSet, toolcontract.ToolFunction[terminalRunInput, toolcontract.ToolResult]{
		Definition: toolcontract.ToolDefinition{
			ID:              "bluecollar/terminal_run",
			Name:            toolcontract.TerminalRunToolName,
			Description:     "Run one shell command in the working directory and read back its combined output and exit code.",
			Visibility:      toolcontract.ToolVisibilityModel,
			InputSchema:     terminalRunInputSchema,
			OutputSchema:    terminalRunOutputSchema,
			ResultContract:  &toolcontract.ToolResultContract{Schema: terminalRunOutputSchema},
			SideEffectClass: toolcontract.ToolSideEffectStateChange,
		},
		Handler: func(toolContext context.Context, input terminalRunInput) (toolcontract.ToolResult, error) {
			return runShellCommand(toolContext, workspacePath, input), nil
		},
		Result: toolcontract.IdentityToolResult,
	})
	return toolSet
}

func runShellCommand(ctx context.Context, workingDirectoryPath string, input terminalRunInput) toolcontract.ToolResult {
	command := strings.TrimSpace(input.Command)
	if command == "" {
		return toolcontract.ToolFailureResult(toolcontract.FailureInvalidInput, toolcontract.FailureCodes.InvalidInput, "terminal_run", "a command is required")
	}
	if input.ApprovalRequired {
		return toolcontract.ToolFailureResult(toolcontract.FailureInteractionRequired, toolcontract.FailureCodes.InteractionRequired, "terminal_run",
			"this runner has nobody to ask, so a command needing approval will not run here; decide whether to run it unapproved or stop and say why")
	}
	commandContext, cancel := context.WithTimeout(ctx, commandTimeout(input.TimeoutSecond))
	defer cancel()

	capturedOutput := &bytes.Buffer{}
	shellCommand := exec.CommandContext(commandContext, "sh", "-c", command)
	shellCommand.Dir = workingDirectoryPath
	shellCommand.Stdout = capturedOutput
	shellCommand.Stderr = capturedOutput
	runError := shellCommand.Run()

	if errors.Is(commandContext.Err(), context.DeadlineExceeded) {
		return toolcontract.ToolFailureResult(toolcontract.FailureDependencyUnavailable, toolcontract.FailureCodes.Unavailable, "terminal_run",
			"the command was still running after "+commandTimeout(input.TimeoutSecond).String()+" and was stopped")
	}
	return terminalRunResult(shellCommand.ProcessState.ExitCode(), capturedOutput.String(), runError)
}

func terminalRunResult(exitCode int, output string, runError error) toolcontract.ToolResult {
	truncatedOutput, wasTruncated := truncateOutput(output)
	document, marshalError := json.Marshal(terminalRunOutput{ExitCode: exitCode, Output: truncatedOutput, Truncated: wasTruncated})
	if marshalError != nil {
		return toolcontract.ToolFailureResult(toolcontract.FailureUnknown, toolcontract.FailureCodes.OperationFailed, "terminal_run", marshalError.Error())
	}
	if runError != nil && exitCode == 0 {
		return toolcontract.ToolFailureResult(toolcontract.FailureUnknown, toolcontract.FailureCodes.OperationFailed, "terminal_run", runError.Error())
	}
	return toolcontract.ToolSuccessData(truncatedOutput, document)
}

func truncateOutput(output string) (string, bool) {
	if len(output) <= maximumCapturedOutput {
		return output, false
	}
	return output[len(output)-maximumCapturedOutput:], true
}

func commandTimeout(timeoutSecond int) time.Duration {
	if timeoutSecond <= 0 {
		return defaultCommandTimeout
	}
	return time.Duration(timeoutSecond) * time.Second
}
